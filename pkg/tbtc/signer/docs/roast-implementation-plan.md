# ROAST Implementation Plan (FROST Migration)

Date: 2026-02-27
Status: Draft implementation roadmap
Owner: Threshold Labs
Scope: native FROST signing robustness via ROAST-style coordinator semantics

## 1. Why This Plan Exists

ROAST coordinator semantics are currently deferred in the Rust signer migration.
This document provides a concrete implementation path.

## 2. Current Baseline

Implemented today:

- Native signer supports cohort-aware `StartSignRound.signing_participants`.
- Finalize is strict to the declared round cohort.
- Retry/cohort attempt metadata is propagated in keep-core runtime.
- Deterministic ROAST-style coordinator selection exists in keep-core
  (`pkg/frost/roast.SelectCoordinator`).

Not implemented today:

- Full ROAST coordinator state machine and policy enforcement in native path.
- Protocol-level coordinator authorization checks.
- Complete malicious/aborting participant robustness flow.

## 3. Goal And Non-Goals

Goal:

- Implement ROAST-style coordinator semantics end-to-end for native FROST
  signing so liveness under aborting/failing participants is materially stronger
  than current restart/re-cohort-only behavior.

Non-goals (for this plan):

- Distributed DKG redesign.
- Full protocol replacement beyond current FROST migration architecture.
- Mandatory true late t-of-n finalize in the same increment set.

## 4. Design Principles

1. Fail closed on ambiguity or transcript mismatch.
2. Keep attempt identity explicit and stable across Rust + keep-core boundaries.
3. Bind every signing step to a deterministic transcript (message, session,
   attempt, cohort, coordinator context).
4. Prefer incremental rollout with strict feature gating and clear fallback
   behavior.
5. Prefer a stateless coordinator model where possible; coordinator authority
   and active attempt context should be derivable from transcript + static
   session configuration. Stateful transition authorization and replay
   registries remain mandatory for fail-closed restart safety.

## 5. Threat Model Snapshot

- Malicious coordinator:
  attempts unauthorized advancement, malformed cohort context, or replay.
- Malicious participant:
  strategic aborts/invalid shares to force repeated retries or unfair exclusion.
- Network adversary:
  replay/reorder/delay/duplication of attempt-context-bearing messages.
- Corrupt persistent state:
  tampered state payloads attempting stale-attempt acceptance after restart.

This is a scoped threat model for implementation sequencing; deeper adversarial
analysis is a Phase 5 gate artifact.

## 6. Target Semantics

- Every attempt has a deterministic `attempt_id`.
- Coordinator for attempt `k` is deterministic from attempt seed + included
  members + attempt number.
- Participants reject requests whose attempt/coordinator context does not match
  local transcript expectations.
- Retry policy is explicit: rotate coordinator first when possible; then exclude
  members only when timeout/blame evidence policy allows it.
- Attempt-number advancement is authenticated by defined policy (quorum timeout
  evidence, blame evidence, or an equivalent signed transition rule).
- Replay/nonce-safety invariants are preserved across attempt transitions.
- Observability can explain why an attempt failed and why next attempt was
  selected.

## 7. Phased Implementation Plan

### Phase 0: Protocol Spec Freeze

Deliverables:

- Short RFC/decision brief for ROAST semantics in current architecture.
- Phase 0 artifact:
  `docs/frost-migration/roast-phase-0-spec-freeze.md`.
- Nonce-safety argument for attempt transitions (cohort/coordinator changes)
  under current deterministic nonce model.
- Threat-model-to-control mapping for coordinator, participant, network, and
  persisted-state adversaries.
- Canonical transcript fields and domain-separation tags.
- Error taxonomy for coordinator/attempt mismatch and stale attempt reuse.
- Resolve known preconditions from prior reviews before coordinator enforcement:
  - response validation bypass risk in cohort response handling,
  - double-derivation ambiguity for included members,
  - consumed-registry capacity-check ordering in sign path.

Acceptance criteria:

- Spec approved by signer and keep-core owners.
- No unresolved ambiguity on attempt identity or coordinator authority.
- Cross-language test vectors pass for canonical attempt-context hashing.

### Phase 1: API/Contract Extensions

Deliverables:

- Extend native signer request envelope(s) with explicit attempt context:
  `attempt_number`, `attempt_id`, `coordinator_identifier`,
  `included_participants_fingerprint`.
- Canonical serialization/hash rules for attempt context.
- Backward-compatible contract gating (`feature`/env/runtime flag).
- Explicit migration behavior for pre-ROAST persisted sessions when strict mode
  is enabled (recommended: fail closed with clear error and require session
  restart).
- Shared cross-repo test vectors for attempt-context serialization/hash
  round-trip.

Acceptance criteria:

- FFI tests cover encode/decode, mismatch rejection, idempotent retry behavior.
- Strict mode rejects missing/invalid attempt context.
- Migration-path tests cover pre-ROAST session-state behavior on strict-mode
  enablement.

### Phase 1.5: Consumed-Registry Integration

Deliverables:

- Define relationship between `attempt_id`, existing `round_id`, and consumed
  registries (`consumed_sign_round_ids`, `consumed_finalize_round_ids`).
- Decide whether attempt tracking is additive (new registry) or replacement key
  strategy, with explicit replay implications.
- Define cap policy for attempt-related registries and interaction with
  `TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION`.
- Ensure capacity-check ordering is fail-closed before state mutation.

Acceptance criteria:

- Registry model is documented and implemented consistently in Rust + keep-core
  retry handling.
- Capacity-limit tests cover deterministic fail-closed behavior (no eviction
  weakening).

### Phase 2: Coordinator Policy Enforcement

Deliverables:

- keep-core coordinator runtime enforces selected coordinator semantics for each
  attempt (not informational-only).
- Reject non-authorized coordinator actions in native flow.
- Implement authenticated attempt-transition policy (who can advance attempt and
  with what evidence).

Acceptance criteria:

- Integration matrix covers:

  | Scenario | Expected |
  | --- | --- |
  | Correct coordinator + correct attempt context | Accept |
  | Wrong coordinator + correct attempt context | Reject |
  | Correct coordinator + wrong attempt number | Reject |
  | Correct coordinator + wrong participants fingerprint | Reject |
  | Coordinator valid for attempt N but request carries N+1 | Reject |
  | Valid payload for stale attempt `< current` | Reject |

- Deterministic attempt transitions are reproducible across retries/restarts.

### Phase 3: Attempt Transcript And Replay Hardening

Deliverables:

- Bind signer-side state to `(session_id, attempt_id, message, cohort)` and
  reject cross-attempt replay.
- Define state-machine behavior for concurrent/future attempt payloads (for
  example receiving attempt `N+1` while local state is at `N`).
- Persist attempt lifecycle artifacts needed for restart-safe enforcement.
- Add bounded retention policy for attempt registries (fail closed, no silent
  eviction that weakens replay protection).

Acceptance criteria:

- Restart tests prove stale attempt replay rejection.
- Concurrency tests prove deterministic handling of future-attempt payloads.
- Capacity-limit tests prove deterministic fail-closed behavior.

### Phase 4: Liveness Policy And Recovery Behavior

Deliverables:

- Explicit policy for excluding failed members and advancing to next attempt.
- Coordinator-failure detection semantics (timeout source, default timeout,
  configurability, and who triggers advance).
- Evidence requirements for exclusion/blame (timeout-only vs cryptographic proof
  for invalid-share faults).
- Clear distinction between recoverable (retry) and terminal (abort) errors.
- Optional policy hook for adaptive backoff/coordinator rotation.

Acceptance criteria:

- End-to-end tests with injected signer/coordinator failures succeed when
  threshold is still available.
- Failure reasons are surfaced in structured telemetry.
- Exclusion decisions are traceable to policy-defined evidence in logs/telemetry.

### Phase 5: Security/Review Gates And Rollout

Deliverables:

- Phase 5 gate artifact:
  `docs/frost-migration/roast-phase-5-security-rollout-gates.md`.
- Adversarial review packet focused on coordinator authority, transcript
  binding, replay resistance, and restart safety.
- Rollout plan with feature flags, canary stages, and rollback conditions.
- Operational readiness metrics and rollback thresholds:
  - attempt success rate,
  - coordinator rotations per signing request,
  - p95/p99 signing latency deltas vs baseline.
- Provisional threshold bands are documented in the Phase 5 gate artifact and
  must be calibrated against baseline before production cutover.
- Benchmarks for happy-path, single-member failure, and coordinator-failure
  conditions, validated against tBTC protocol timeout budgets.
- Chaos test suite for network partition/delay/duplication and process crash
  during active signing rounds.

Acceptance criteria:

- All blocking review findings resolved.
- Human sign-off recorded for ROAST gate.
- Performance and chaos criteria meet documented rollout thresholds.

## 8. Proposed Chunking

Recommended chunk order (docs+code):

1. Resolve precondition findings from prior cohort/consumed-registry reviews.
2. Phase 0 spec freeze + shared test vectors.
3. Phase 1 FFI/API contract scaffolding + strict-mode migration semantics.
4. Phase 1.5 consumed-registry integration and cap policy.
5. Phase 2 coordinator enforcement + authenticated attempt advancement.
6. Phase 3 transcript/replay/restart/concurrency hardening.
7. Phase 4 liveness policy + coordinator timeout/blame evidence.
8. Phase 5 performance benchmarks + chaos/failure-matrix testing.
9. Adversarial review packet + rollout runbook + human sign-off.

## 9. Dependencies And Ownership

- `tbtc` repo:
  - `tools/tbtc-signer` request/state model updates and tests.
- `threshold-network/keep-core` repo:
  - coordinator policy enforcement, runtime retry transitions, integration tests.
- Canonical attempt-context struct source of truth:
  `tools/tbtc-signer/src/api.rs`.
- keep-core must implement byte-for-byte compatible encode/decode + hashing.
- Shared attempt-context vectors should be maintained and validated in both
  repos (proposed path:
  `docs/frost-migration/test-vectors/roast-attempt-context-v1.json`).

## 10. Relation To True Late t-of-n Finalize

ROAST and true late t-of-n are different roadmap items.

- ROAST should be implemented first for robustness/liveness guarantees.
- True late t-of-n can be reconsidered after ROAST based on production evidence.
- Attempt/transcript identifiers should be versioned so late t-of-n (if adopted
  later) can reuse model foundations without breaking compatibility.

## 11. Definition Of Done

ROAST is considered implemented for this migration when:

- coordinator authority is enforced in native flow,
- attempt transcript binding is enforced end-to-end,
- replay/restart safety is proven by tests,
- liveness under partial failures is demonstrated by e2e and chaos failure
  matrix tests,
- benchmarked latency/rotation metrics remain within documented rollout
  thresholds,
- backward-compatible upgrade behavior from pre-ROAST sessions is tested, and
- external human review sign-off is complete.
