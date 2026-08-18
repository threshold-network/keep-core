---
title: FROST signer — funnel the policy `reject_*` family
date: 2026-08-18
status: draft
tags: [frost-signer, architecture, deepening, policy]
---

# FROST signer — funnel the policy `reject_*` family

## Problem Statement

`pkg/tbtc/signer/src/engine/policy.rs` exposes 7 `reject_*` helpers across 44 production call sites. The signing family (3 helpers) already collapses into a single `reject_signing_policy_with_metric` funnel with 3 thin wrappers. The remaining duplication is concentrated in 3 uncompressed helpers:

- `reject_admission_policy` (policy.rs:236) — 17 lines, own metric (`run_dkg_admission_reject_total`).
- `reject_quarantine_policy` (policy.rs:510) — 13 lines.
- `reject_lifecycle_policy<T>` (policy.rs:524) — 13 lines, generic over `T`.

Each of the three carries its own copy of `let detail = ...; log_policy_decision(STAGE, session_id, "reject", reason_code); Err(EngineError::Variant {...})`. The shape is identical; only the stage label and the variant name differ.

A second tier of leakage: two hand-constructed `EngineError::LifecyclePolicyRejected` sites bypass the log hook entirely:

- `interactive.rs:1916` — emergency-rekey kill switch.
- `transaction.rs:62` — `build_taproot_tx` entry.

The `lifecycle_policy` log stage is NOT emitted for either path today. The invariant "all rejections go through `log_policy_decision`" is enforced only by convention.

## Solution

Extract a single private core `reject_with(stage, session_id, reason_code, detail) -> Result<(), EngineError>` inside `policy.rs`. The 7 existing helpers stay as `pub(crate)` 1-line wrappers. The 2 hand-constructed sites route through the new helper so the `lifecycle_policy` log stage fires.

The change is mechanical: the 3 uncompressed helpers lose ~25 lines, the 2 hand-constructed sites lose their inline `EngineError::LifecyclePolicyRejected` construction, and the new core is the single place to change metric/log/Err mapping.

## User Stories

1. As a maintainer, I want to add a new policy stage (e.g. a future consensus-policy gate) so that the change is one new variant in the `PolicyRejectStage` enum, not a new helper.
2. As a maintainer, I want the metric/log mapping in one place so that a future logging change is a single diff.
3. As a maintainer, I want the two hand-constructed sites to fire the `lifecycle_policy` log stage so that the audit log is uniform across all rejection paths.
4. As a security reviewer, I want the `lifecycle_policy` stage to be emitted for every rejection so that the audit trail is complete.
5. As a test author, I want the 7 helper names preserved so that the existing test assertions (`metrics.build_taproot_tx_policy_reject_total`, etc.) continue to work.
6. As a maintainer, I want the `T` generic on `reject_lifecycle_policy<T>` to survive so that `promote_canary` (which returns `Result<PromoteCanaryResult, EngineError>`) does not need to wrap.
7. As a Go host integrator, I want the public `EngineError` variants unchanged so that the bridge does not need to bump its ABI version.

## Implementation Decisions

### Refactor shape

```rust
#[derive(Clone, Copy)]
pub(crate) enum PolicyRejectStage {
    Admission,
    AutoQuarantine,
    Lifecycle,
    SigningPolicyFirewall { metric: PolicyRejectMetricKind },
    Provenance,  // no log, no session_id
}

fn reject_with(
    stage: PolicyRejectStage,
    session_id: Option<&str>,
    reason_code: &str,
    detail: impl Into<String>,
) -> EngineError {
    // 1. metric update per stage variant
    // 2. log_policy_decision per stage (skip for Provenance)
    // 3. build EngineError::Variant { ... } per stage and return it
}
```

`reject_with` returns the constructed `EngineError` itself, not a `Result`. Every stage always rejects — there is no success path — so returning the bare error and letting each wrapper decide how to wrap it (`Err(reject_with(...))` for a `T = ()` wrapper, or unified against any `T` for the generic `reject_lifecycle_policy<T>`) avoids a `Result<(), EngineError>` core whose `Ok` variant is never constructed. That shape is what motivated the original hand-written duplication in the first place: each of the 3 uncompressed helpers already returns the error directly rather than routing through a fictitious success case.

The 7 existing helpers become 1-line wrappers:

```rust
pub(crate) fn reject_admission_policy(...) -> Result<(), EngineError> {
    Err(reject_with(PolicyRejectStage::Admission, Some(session_id), reason_code, detail))
}
pub(crate) fn reject_quarantine_policy(...) -> Result<(), EngineError> {
    Err(reject_with(PolicyRejectStage::AutoQuarantine, Some(session_id), reason_code, detail))
}
pub(crate) fn reject_lifecycle_policy<T>(...) -> Result<T, EngineError> {
    Err(reject_with(PolicyRejectStage::Lifecycle, Some(session_id), reason_code, detail))
}
fn reject_signing_policy_with_metric(...) -> Result<(), EngineError> {
    Err(reject_with(PolicyRejectStage::SigningPolicyFirewall { metric: kind }, Some(session_id), reason_code, detail))
}
pub(crate) fn reject_signing_policy(...) -> Result<(), EngineError> {
    reject_signing_policy_with_metric(..., PolicyRejectMetricKind::BuildTaprootTx)
}
fn reject_heartbeat_signing_policy(...) -> Result<(), EngineError> {
    reject_signing_policy_with_metric(..., PolicyRejectMetricKind::Heartbeat)
}
fn reject_interactive_rate_limit_signing_policy(...) -> Result<(), EngineError> {
    reject_signing_policy_with_metric(..., PolicyRejectMetricKind::InteractiveRateLimit)
}
```

`Err(reject_with(...))` type-checks for every wrapper regardless of `T`, including `reject_lifecycle_policy<T>` where `T` varies by caller (`()` for most call sites, `TransactionResult` for `build_taproot_tx`'s hand-constructed site below) — there is no `?` and no dead-code branch to reconcile.

The `reject_provenance_gate` helper in `provenance.rs` is intentionally NOT folded into `reject_with` — it has a different protocol (no log, no session_id, no metric). It stays as-is.

### Stage-to-variant mapping

The `reject_with` core maps `PolicyRejectStage` to the `EngineError` variant:

| Stage | Variant |
|---|---|
| `Admission` | `EngineError::AdmissionPolicyRejected` |
| `AutoQuarantine` | `EngineError::QuarantinePolicyRejected` |
| `Lifecycle` | `EngineError::LifecyclePolicyRejected` |
| `SigningPolicyFirewall { metric }` | `EngineError::SigningPolicyRejected` |
| `Provenance` | (not folded; `reject_provenance_gate` stays separate) |

The `transaction.rs:253` matches-arm `matches!(error, EngineError::SigningPolicyRejected { .. })` continues to work because the `SigningPolicyRejected` variant is unchanged.

### Hand-constructed site routing

The two sites `interactive.rs:1916` and `transaction.rs:62` currently construct `EngineError::LifecyclePolicyRejected` directly, each with `reason_code: "emergency_rekey_required"` and its own `format!` detail carrying `emergency_rekey_event.triggered_at_unix` and `.reason`. They become:

```rust
// interactive.rs:1916 (enforce_interactive_signing_gates, returns Result<(), EngineError>)
return reject_lifecycle_policy(
    session_id,
    "emergency_rekey_required",
    format!(
        "emergency rekey required for session [{}] since [{}]: {}",
        session_id, emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
    ),
);

// transaction.rs:62 (build_taproot_tx, returns Result<TransactionResult, EngineError>)
return reject_lifecycle_policy(
    &request.session_id,
    "emergency_rekey_required",
    format!(
        "build_taproot_tx blocked: emergency rekey required since [{}]: {}",
        emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
    ),
);
```

`reject_lifecycle_policy<T>` always constructs `Err`, never `Ok`, so `return reject_lifecycle_policy(...)` type-checks directly against each caller's own `Result<T, EngineError>` — `T = ()` for `enforce_interactive_signing_gates`, `T = TransactionResult` for `build_taproot_tx`. There is no `?` here: wrapping the call in `Err(reject_lifecycle_policy(...)?)` would be a type error (the `?` already returns early on the helper's `Err`, so the outer `Err(...)` would need a success value that never exists).

The `reason_code` and `detail` are preserved byte-for-byte from the current inline construction — the refactor changes only how the `Err` is built, not its field values.

The `reject_lifecycle_policy` helper now fires the `lifecycle_policy` log stage. The `Err(EngineError::LifecyclePolicyRejected { ... })` variant is preserved; the log fires before the Err is returned.

### Out-of-scope changes

- The `enforce_signing_policy_firewall_inner` `charge_rate_limit: bool` flag is unchanged. The flag is load-bearing for the idempotent cache recheck, but it does NOT collapse the heartbeat / build-taproot split (heartbeat has its own dedicated enforcement path). The spec does not propose unifying the two enforcement paths.
- The `PolicyRejectMetricKind` enum is unchanged.
- The `record_hardening_telemetry` calls inside `reject_signing_policy_with_metric` are preserved (the new core forwards to the same metric updates).
- The `log_policy_decision` function is unchanged.

## Testing Decisions

### Test surface

The 7 helper names are preserved. The existing `engine::tests` assertions that name the helpers (e.g. tests inspecting `metrics.build_taproot_tx_policy_reject_total`) continue to work without change.

The `transaction.rs:253` matches-arm (`matches!(error, EngineError::SigningPolicyRejected { .. })`) continues to work because the `SigningPolicyRejected` variant is unchanged.

The new `lifecycle_policy` log firing at the two hand-constructed sites is exercised by existing tests that observe the metrics. A new test (`engine::tests::lifecycle_rejection_fires_log_for_kill_switch`) explicitly checks that the `lifecycle_policy` stage appears in the audit log when the kill switch fires.

### What makes a good test here

- A good test of `reject_with` verifies that the stage-specific metric and the stage-specific log line are emitted.
- A good test of the 7 wrappers verifies that the wrapper-level signature and the variant it returns are unchanged.
- A good test of the two hand-constructed sites verifies that the `lifecycle_policy` log stage fires.

### Prior art

The existing `engine::tests::hardening_metrics_count_calls_before_provenance_gate_rejection` (around line 2270) exercises the metric side. The new spec reuses the same test scaffolding.

### Definition of Done

- The 3 uncompressed helpers (`reject_admission_policy`, `reject_quarantine_policy`, `reject_lifecycle_policy<T>`) become 1-line wrappers around `reject_with`.
- The 4 signing helpers (`reject_signing_policy_with_metric` + 3 thin wrappers) continue to work with the same call signatures.
- The 2 hand-constructed `EngineError::LifecyclePolicyRejected` sites route through `reject_lifecycle_policy` (or its replacement).
- The `lifecycle_policy` log stage fires for the kill switch and the `build_taproot_tx` entry.
- `cargo test --lib --package frost-signer` passes (no behavioral change for the existing assertions).
- The chaos suite (`scripts/run_phase5_chaos_suite.sh`) passes.
- The `TBTC_SIGNER_ABI_*` constants are not bumped.

## Out of Scope

- The `enforce_signing_policy_firewall_inner` parameterization. The `charge_rate_limit: bool` flag is preserved; the heartbeat / build-taproot split is NOT collapsed.
- The `interactive_rate_limit_reject_total` field. Confirmed declared and live; not a pre-existing bug (see Risks).
- The `record_canary_policy_outcome` call at `transaction.rs:254`. The matches-arm decision continues to work because the variant is unchanged.
- The `log_policy_decision` function itself. The new core forwards to the same function.

## Further Notes

### Open questions

- (Resolved) The `reject_lifecycle_policy<T>` generic shape fits cleanly because `reject_with` returns a bare `EngineError`, not a `Result<(), EngineError>`. Every wrapper — including the generic one — does `Err(reject_with(...))`, which unifies against any `T`. No restated `Err(EngineError::LifecyclePolicyRejected { ... })` literal and no `?`-then-dead-code pattern are needed.
- The `PolicyRejectStage::SigningPolicyFirewall { metric }` carries the metric kind. The metric is recorded inside `reject_with`, not in the wrapper. The split is intentional: the wrapper is a thin pass-through; the metric is a property of the stage, not of the wrapper.

### Risks

- The `reject_with` core is a single dispatch point. A future bug in the metric/log/Err mapping affects all 7 helpers. The risk is mitigated by the small interface (a single function with 4 parameters) and the existing test surface (the 7 helpers preserve their return types).
- The `T` generic on `reject_lifecycle_policy<T>` is unconstrained: nothing stops a future caller from instantiating it at a type that can never actually be produced (since the function only ever returns `Err`). The risk is small — a wrong `T` only shows up as a caller-side type mismatch at the `return reject_lifecycle_policy(...)` call site, caught by the compiler, not a runtime bug.
- Investigation initially suspected `interactive_rate_limit_reject_total` was referenced but undeclared in `HardeningTelemetryState`. Verification against current code found the field IS declared and live (telemetry.rs:435-436, policy.rs:563-565) — no pre-existing bug, no risk here.

### Alternatives considered

- **Drop the 7 helpers; call sites use `policy::reject_with(...)` directly**: renames 23 call sites. The user picked the wrapper-preserving approach (Candidate 4 seam = option 0).
- **`PolicyGate` struct with stage instances**: heaviest refactor. The user picked the in-place helper approach.

### Related specs

- **Candidate 1** (interactive-session collapse): independent, can run in parallel. `interactive.rs:1916` sits inside `enforce_interactive_signing_gates` (interactive.rs:1895), a standalone helper C1 does not touch — C1's restructuring is scoped to the lock-prologue and marker-durability blocks at the top of each of the 5 `pub fn` phase handlers, not to internal business-logic helpers they call.
- **Candidate 2** (persistence split): independent, can run in parallel.
- **Candidate 5** (SessionState grouping): independent, can run in parallel.
- **Candidate 6** (FFI macro): independent, can run in parallel.
