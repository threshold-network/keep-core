---
title: FROST signer — split `SessionState` into 6 named substructures
date: 2026-08-18
status: draft
tags: [frost-signer, architecture, deepening, state]
---

# FROST signer — split `SessionState` into 6 named substructures

## Problem Statement

`pkg/tbtc/signer/src/engine/state.rs` defines `SessionState` as a 31-field flat struct holding 6 unrelated concerns:

- **DKG material**: 5 fields (request fingerprint, key packages, public key package, result, policy snapshot version).
- **Legacy non-interactive signing**: 12 fields (sign request fingerprint, message bytes, round state, active attempt context, finalize fingerprint, signature result, build-tx fingerprint, transaction result, 4 consumed-marker registries).
- **Audit trail**: 1 field (`attempt_transition_records`).
- **Lifecycle**: 5 fields (refresh request fingerprint, refresh result, refresh history, refresh count, emergency rekey event).
- **Interactive FROST (Phase 7.1)**: 5 fields (interactive signing map, bound key group, 3 marker registries).
- **Operational capacity pins**: 3 fields (heartbeat rate limiter, retired interactive timestamp, aggregate eviction pin).

Reading any one concern (Round2, Aggregate, Refresh, Audit) requires the entire struct in scope. A change to any field ripples through every test that constructs a fixture. The 21 hand-written test fixtures in `engine::tests` plus the 1 `SessionState { ... }` literal inside the `TryFrom<PersistedSessionState>` constructor in `persistence.rs` explicitly build `SessionState { ... }` literals and break on every shape change.

The persistence conversion already projects per-field — the structural split is already evidenced in the codec. The grouping is grounded in call-site co-location, not in abstract design preference.

## Solution

Split `SessionState` into 6 named substructures, each holding one concern. The grouping is co-located by call sites; the existing `TryFrom` projections become the migration path.

```
SessionState {
    dkg:           DkgSessionState,            // 5 fields
    signing:       LegacySigningSessionState,  // 12 fields
    interactive:   InteractiveSessionState,    // 5 fields
    audit:         AuditTrail,                 // 1 field
    lifecycle:     LifecycleState,             // 5 fields
    capacity_pins: OperationalState,           // 3 fields
}
```

The persisted schema (`PersistedSessionState`) is unchanged. The wire format is byte-for-byte stable. The `TryFrom` impls project into the new substructures and flatten back to the same 31 persisted fields.

Migration cost: roughly 147 production `.field` read/write sites (`interactive.rs` ~89, `dkg.rs` ~17, `state.rs` ~13 — mostly the `per_message_interactive_session` exhaustive destructuring, `lifecycle.rs` ~13, `transaction.rs` ~9, `verify_share.rs` ~3, `audit.rs` ~2, `telemetry.rs` ~1) plus the `persistence.rs` `TryFrom` internals (not cleanly separable from this count: the field names are shared verbatim between `SessionState` and `PersistedSessionState`, so a plain grep over `persistence.rs` cannot distinguish a `session.dkg_result` access from a `persisted.dkg_result` access). Struct literals: 21 in `tests.rs` + 1 inside the `TryFrom<PersistedSessionState>` impl in `persistence.rs` = 22 total. The `PersistedSessionState { ... }` literal at tests.rs:623 is the wire schema and intentionally unchanged.

## User Stories

1. As a maintainer, I want the signing path to read only `session.interactive` so that a future signing refactor does not touch the DKG fields.
2. As a maintainer, I want the DKG path to read only `session.dkg` so that a future DKG refactor does not touch the signing fields.
3. As a maintainer, I want the lifecycle path to read only `session.lifecycle` so that refresh and rekey logic is independent of the signing path.
4. As a test author, I want to construct a `SessionState` fixture with only the fields my test cares about so that the fixture is smaller and the test's intent is clearer.
5. As a maintainer, I want the persistence codec to flatten the substructures into the same 31 persisted fields so that the on-disk file format is unchanged.
6. As a Go host integrator, I want the wire schema unchanged so that the bridge does not need to bump its ABI version.
7. As a tooling author, I want the `engine::tests::persisted_session_state_*` fixtures to construct a `PersistedSessionState` (the wire form) without touching the new `SessionState` grouping so that the wire-schema tests stay readable.
8. As a security reviewer, I want the cross-field retirement invariant (at `persistence.rs:2222–2230`) to be preserved at the TryFrom site so that the security boundary is unchanged.

## Implementation Decisions

### Substructure definitions

```rust
#[derive(Default)]
pub(crate) struct DkgSessionState {
    pub(crate) request_fingerprint: Option<String>,
    pub(crate) key_packages: Option<BTreeMap<u16, frost::keys::KeyPackage>>,
    pub(crate) public_key_package: Option<frost::keys::PublicKeyPackage>,
    pub(crate) result: Option<DkgResult>,
    /// DKG signing-policy firewall compatibility check (state.rs:160-161).
    pub(crate) policy_snapshot_version: u32,
}

#[derive(Default)]
pub(crate) struct LegacySigningSessionState {
    pub(crate) request_fingerprint: Option<String>,
    pub(crate) message_bytes: Option<SecretBytes>,
    pub(crate) round_state: Option<RoundState>,
    pub(crate) active_attempt_context: Option<AttemptContext>,
    pub(crate) finalize_request_fingerprint: Option<String>,
    pub(crate) signature_result: Option<SignatureResult>,
    pub(crate) build_tx_request_fingerprint: Option<String>,
    pub(crate) tx_result: Option<TransactionResult>,
    pub(crate) consumed_attempt_ids: HashSet<String>,
    pub(crate) consumed_sign_round_ids: HashSet<String>,
    pub(crate) consumed_finalize_round_ids: HashSet<String>,
    pub(crate) consumed_finalize_request_fingerprints: HashSet<String>,
}

#[derive(Default)]
pub(crate) struct InteractiveSessionState {
    pub(crate) interactive_signing: BTreeMap<u16, InteractiveSigningState>,
    pub(crate) bound_key_group: Option<String>,
    pub(crate) consumed_attempt_markers: HashSet<String>,
    pub(crate) authorized_aggregate_markers: HashSet<String>,
    pub(crate) aggregated_attempt_markers: HashSet<String>,
}

#[derive(Default)]
pub(crate) struct AuditTrail(pub(crate) Vec<TranscriptAuditRecord>);

#[derive(Default)]
pub(crate) struct LifecycleState {
    pub(crate) refresh_request_fingerprint: Option<String>,
    pub(crate) refresh_result: Option<RefreshSharesResult>,
    pub(crate) refresh_history: Vec<RefreshHistoryRecord>,
    pub(crate) refresh_count: u64,
    pub(crate) emergency_rekey_event: Option<EmergencyRekeyEvent>,
}

#[derive(Default)]
pub(crate) struct OperationalState {
    pub(crate) heartbeat_rate_limiter: PolicyRateLimiterState,
    pub(crate) retired_interactive_at_unix: Option<u64>,
    pub(crate) aggregate_eviction_pin: Arc<()>,
}

#[derive(Default)]
pub(crate) struct SessionState {
    pub(crate) dkg: DkgSessionState,
    pub(crate) signing: LegacySigningSessionState,
    pub(crate) interactive: InteractiveSessionState,
    pub(crate) audit: AuditTrail,
    pub(crate) lifecycle: LifecycleState,
    pub(crate) capacity_pins: OperationalState,
}
```

### Field naming

The substructure fields drop the redundant prefix. `dkg_request_fingerprint` becomes `dkg.request_fingerprint`; `consumed_interactive_attempt_markers` becomes `interactive.consumed_attempt_markers`. The wire-schema field names (`dkg_request_fingerprint`, `consumed_interactive_attempt_markers`) are preserved on `PersistedSessionState`.

### Call-site co-location evidence

The grouping is grounded in the call sites:

- `DkgSessionState` (5 fields) — co-read by `persist_distributed_dkg_key_package` (dkg.rs:215–311), `interactive_session_open` (interactive.rs:373–382), `verify_share` (verify_share.rs:117). `policy_snapshot_version` is round-tripped through persistence and destructured in `per_message_interactive_session`, but has no other production reader today (`current_policy_snapshot_version()` in policy.rs is currently unreferenced elsewhere) — it groups with `dkg` by name and doc comment, not by an established multi-site co-read pattern.
- `LegacySigningSessionState` (12 fields) — co-read by `transaction.rs:72–75, 290–300, 315–316` and the legacy signing path's consumed-marker registries.
- `InteractiveSessionState` (5 fields) — co-read by every `interactive_round1/2/aggregate/session_open/session_abort` path; the `bound_key_group.is_some() && dkg_*.is_none()` discriminator in `per_message_interactive_session` at `state.rs:553–621` separates per-message from DKG-bearing sessions.
- `AuditTrail` (1 field) — reads at `audit.rs:335` and `audit.rs:397`.
- `LifecycleState` (5 fields) — co-read by `legacy_synthetic_refresh_artifacts_present` (lifecycle.rs:140–145), `refresh_cadence_status` (lifecycle.rs:174, fields read at 197–212), `trigger_emergency_rekey` (lifecycle.rs:286–318), `telemetry.rs:521`.
- `OperationalState` (3 fields) — co-read by `compact_retired_per_message_sessions`, `aggregate_eviction_pin`, `heartbeat_rate_limiter`. The three share the property "not a content field" — they are capacity-management state with mixed persistence.

### Migration path

The access sites rewrite from `session.<field>` to `session.<group>.<field>`. The 22 struct literals rewrite to the nested form.

**Per-file migration cost** (counted as `.field_name` occurrences per file, per the 31 field names; approximate — a plain-text count cannot distinguish a shadowed local of the same name, though none were found in a spot check):

| File | `.field` sites | Notes |
|---|---|---|
| `interactive.rs` | ~89 | All five content groups touched; heaviest consumer |
| `persistence.rs` | not separable | Field names are shared verbatim between `SessionState` and `PersistedSessionState`; the `TryFrom` impls read/write both types under the same field names, so a text count conflates the two. Both `TryFrom` bodies are rewritten regardless (see "Persistence codec shape" below). |
| `dkg.rs` | ~17 | `dkg.*` + `interactive.bound_key_group` |
| `state.rs` | ~13 | Mostly the exhaustive destructuring in `per_message_interactive_session` (state.rs:553–621) |
| `lifecycle.rs` | ~13 | `lifecycle.*` + `dkg.result` + `interactive.bound_key_group` |
| `transaction.rs` | ~9 | `signing.*` + `lifecycle.emergency_rekey_event` |
| `verify_share.rs` | ~3 | `interactive.bound_key_group` + `dkg.public_key_package` |
| `audit.rs` | ~2 | `audit.attempt_transition_records` |
| `telemetry.rs` | ~1 | `lifecycle.emergency_rekey_event` |
| **Total (excl. persistence.rs, tests.rs)** | **~147** | |

The 21 `SessionState { ... }` literals in `tests.rs` and the 1 inside the `TryFrom<PersistedSessionState>` impl in `persistence.rs` (22 total) rewrite to the nested form. The `PersistedSessionState { ... }` literal at tests.rs:623 is the wire schema and intentionally unchanged.

### Persistence codec shape

The `TryFrom<PersistedSessionState> for SessionState` impl (persistence.rs:1917-2233 today) validates the consumed-marker registries and hex-decodes the DKG/sign-message fields exactly as it does now, then projects into the new substructures in a single literal:

```rust
fn try_from(persisted: PersistedSessionState) -> Result<Self, Self::Error> {
    // ... unchanged: consumed-marker validation, hex-decode of dkg_key_packages /
    // dkg_public_key_package_hex / sign_message_hex, refresh_history monotonicity
    // check, emergency_rekey_event non-empty-reason check ...

    let session = SessionState {
        dkg: DkgSessionState {
            request_fingerprint: persisted.dkg_request_fingerprint,
            key_packages: dkg_key_packages,
            public_key_package: dkg_public_key_package,
            result: persisted.dkg_result,
            policy_snapshot_version: persisted.policy_snapshot_version,  // persisted
        },
        signing: LegacySigningSessionState {
            request_fingerprint: persisted.sign_request_fingerprint,
            message_bytes: sign_message_bytes,
            // ... etc ...
        },
        audit: AuditTrail(persisted.attempt_transition_records),
        lifecycle: LifecycleState {
            refresh_request_fingerprint: persisted.refresh_request_fingerprint,
            // ... etc ...
            refresh_count: persisted.refresh_count.max(persisted.refresh_history.len() as u64),
        },
        interactive: InteractiveSessionState {
            interactive_signing: BTreeMap::new(),  // never restored; nonces gone after restart
            bound_key_group: persisted.bound_key_group,
            // ... etc ...
        },
        capacity_pins: OperationalState {
            heartbeat_rate_limiter: PolicyRateLimiterState::default(),  // transient, not persisted
            retired_interactive_at_unix: persisted.retired_interactive_at_unix,  // persisted
            aggregate_eviction_pin: Arc::new(()),  // transient, not persisted
        },
    };

    // Cross-field retirement invariant (persistence.rs:2222-2230 today)
    if session.capacity_pins.retired_interactive_at_unix.is_some()
        && !per_message_interactive_session(&session)
    {
        return Err(EngineError::Internal(
            "persisted retired interactive session must have the per-message role".to_string(),
        ));
    }

    Ok(session)
}
```

`OperationalState` is NOT wholesale-defaulted: `retired_interactive_at_unix` is a persisted field (round-tripped through both `TryFrom` impls) while `heartbeat_rate_limiter` and `aggregate_eviction_pin` are transient (never serialized) — the group's own doc comment already calls this out as "capacity-management state with mixed persistence."

The inverse `TryFrom<&SessionState> for PersistedSessionState` reads the substructures and flattens back to the same 31 fields. The wire schema is unchanged.

### Out-of-scope changes

- The wire schema (`PersistedSessionState`) is unchanged. Field names, ordering, and serde tags are preserved byte-for-byte.
- The `TransactionResult`, `SignatureResult`, `DkgResult`, `RoundState`, `AttemptContext`, `TranscriptAuditRecord`, `RefreshSharesResult`, `RefreshHistoryRecord`, `EmergencyRekeyEvent`, `CanaryRolloutState` are unchanged in shape.
- The `Drop for InteractiveSigningState` (state.rs:105–109) is unchanged. The type itself does not move; only its containing field path changes.
- The `InteractiveSigningState` struct definition is unchanged.

## Testing Decisions

### Test surface

The chaos suite (`scripts/run_phase5_chaos_suite.sh`) pins `engine::tests::<name>` paths. The test paths are preserved. The 21 test fixtures in `tests.rs` rewrite to the nested form (e.g. `dkg_result: Some(...)` becomes `dkg: DkgSessionState { result: Some(...), ..Default::default() }`).

The single `PersistedSessionState { ... }` literal at tests.rs:623 is the wire schema and unchanged. The test that constructs it is the wire-schema test; it does not exercise the new grouping.

### What makes a good test here

- A good test of the migrated `SessionState` literal verifies that the previously-set fields are still set after the migration (e.g. `session.dkg.result` is `Some(...)`).
- A good test of the cross-field retirement invariant (`persistence.rs:2222–2230`) verifies that a `SessionState` with `capacity_pins.retired_interactive_at_unix` set AND `interactive.bound_key_group` not set is rejected. No dedicated unit test exercises this exact TryFrom-level check today — add one as part of this spec (see Definition of Done).
- A good test of the `PerMessage` discriminant (`per_message_interactive_session`) verifies that the discrimination logic still works after the destructuring changes.

### Prior art

The existing `engine::tests::persisted_session_state_round_trip_preserves_bound_key_group` (tests.rs:4615) exercises the `bound_key_group` round-trip. The migration preserves the test path; the fixture changes from the flat form to the nested form.

### Definition of Done

- The 6 substructures are defined in `state.rs`.
- `SessionState` is a struct-of-substructures with field names that drop the redundant prefix.
- All production `.field` access sites rewrite to `session.<group>.<field>` (see the per-file migration cost table).
- The 22 struct literals rewrite to the nested form.
- The single `PersistedSessionState` literal at tests.rs:623 is unchanged.
- The `TryFrom<PersistedSessionState> for SessionState` impl projects into the new substructures.
- The `TryFrom<&SessionState> for PersistedSessionState` impl flattens back to the same 31 fields.
- The cross-field retirement invariant is preserved at the TryFrom site, and a new `engine::tests::` unit test exercises it directly (none exists today).
- The `refresh_count = persisted.refresh_count.max(history.len() as u64)` legacy semantics are preserved.
- `cargo test --lib --package frost-signer` passes (no behavioral change).
- The chaos suite (`scripts/run_phase5_chaos_suite.sh`) passes.
- The `TBTC_SIGNER_ABI_*` constants are not bumped.
- The persisted state schema version is not bumped.

## Out of Scope

- The wire schema (`PersistedSessionState`) shape. The schema-version, the field names, and the serde tags are preserved.
- The `Drop for InteractiveSigningState` implementation. The type itself does not move.
- The `per_message_interactive_session` discriminator logic. The destructuring pattern changes; the boolean logic is unchanged.
- The `compact_retired_per_message_sessions` and `restore_compacted_retired_sessions` internals. The call sites rewrite to `session.capacity_pins.retired_interactive_at_unix` etc.
- The audit trail (`attempt_transition_records`) consumer code. The audit.rs:335 and audit.rs:397 read sites change to `session.audit.attempt_transition_records`.
- The legacy `refresh_count` semantics. The `max(persisted.refresh_count, persisted.refresh_history.len() as u64)` projection is preserved.

## Further Notes

### Open questions

- The `AuditTrail(pub(crate) Vec<TranscriptAuditRecord>)` newtype is a single-field wrapper. A future tightening could inline it as `session.audit: Vec<TranscriptAuditRecord>` and drop the newtype. The newtype is preferred for the call-site clarity (`session.audit.0` vs `session.audit`).
- The `OperationalState` carries a transient `aggregate_eviction_pin: Arc<()>` that is never persisted. The newtype is a convenient place to clamp the persistence surface, but the inline invariant "this field is never serialized" is not enforced by the type system. A future tightening could use a `#[serde(skip)]` on the field.

### Risks

- The cross-field retirement invariant at `persistence.rs:2222–2230` spans 3 groups (`capacity_pins` for `retired_interactive_at_unix`, `interactive` and `dkg` via `per_message_interactive_session`'s discriminator). After the migration, the check is `if session.capacity_pins.retired_interactive_at_unix.is_some() && !per_message_interactive_session(&session) { return Err(...) }`. The risk is that a future maintainer forgets one of the three groups. The mitigation is the inline check at the TryFrom site (per the spec decision) and the new dedicated unit test this spec adds (see Definition of Done) — no existing test exercises this exact check today.
- The ~147 production access sites (per the per-file migration cost table) are a mechanical migration. The risk is that one site is missed or that the field path is wrong. The mitigation is the existing test suite (every consumer is exercised by tests) and the per-file migration cost table in the spec.
- The 22 struct literals in `tests.rs` and `persistence.rs` are a mechanical migration. The risk is similar. The mitigation is the test suite passing.

### Alternatives considered

- **6 new types in `engine::session` submodule**: the substructures live under a separate module. The user picked the in-place approach (Candidate 5 seam = option 0).
- **Phased migration: groups first, then field accesses**: split the migration into two PRs. The user picked the single-pass mechanical rewrite.
- **Coexist: keep flat names as accessors, deprecate later**: the ~147 production access sites do NOT change. The user picked the mechanical rewrite.

### Related specs

- **Candidate 1** (interactive-session collapse): independent, can run in parallel. Both touch `interactive.rs`, but on disjoint lines: C1 rewrites only the lock-prologue and marker-durability blocks at the top/end of each phase handler; this spec rewrites the ~89 field-access sites in the phase-specific business logic between them, which C1 explicitly leaves untouched. A rebase is mechanical, not a semantic conflict.
- **Candidate 2** (persistence split): NOT independent — both specs rewrite the bodies of `TryFrom<PersistedSessionState> for SessionState` and `TryFrom<&SessionState> for PersistedSessionState` (persistence.rs:1917-2233 and the inverse). See Candidate 2's "Related specs" for the sequencing note; do not apply both to a worktree in parallel and expect a clean merge.
- **Candidate 4** (policy `reject_*` funnel): independent, can run in parallel.
- **Candidate 6** (FFI macro): independent, can run in parallel; the FFI entry points remain unchanged.
