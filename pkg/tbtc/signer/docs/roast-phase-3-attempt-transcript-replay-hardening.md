# ROAST Phase 3: Attempt Transcript And Replay Hardening

Date: 2026-02-27
Status: Complete
Owner: Threshold Labs
Scope: explicit attempt replay registry hardening for signer-side transcript lifecycle

## Objective

Harden signer replay behavior by persisting a dedicated consumed-attempt registry
so attempt replay safety does not depend only on `round_id` composition.

## Decisions Implemented In This Increment

1. Added per-session `consumed_attempt_ids` tracking in signer runtime state.
2. Persisted `consumed_attempt_ids` in durable session state with:
   - empty-entry rejection,
   - duplicate-entry rejection,
   - bounded-size fail-closed validation using existing consumed-registry cap.
3. Enforced `attempt_id` replay rejection in `start_sign_round` before round
   signing material generation:
   - if `attempt_context.attempt_id` is already consumed for the session,
     signer rejects fail-closed.
4. Enforced `consumed_attempt_ids` capacity checks before mutation and without
   eviction behavior.
5. Extended restart/reload replay tests to prove consumed-attempt protection
   remains active after cache loss and process restart.

## Evidence (Code + Tests)

- Runtime and persistence model updates:
  `pkg/tbtc/signer/src/engine` (`SessionState.consumed_attempt_ids`,
  `PersistedSessionState.consumed_attempt_ids`,
  `SessionState::try_from`, `PersistedSessionState::try_from`).
- Start-path attempt replay enforcement:
  `pkg/tbtc/signer/src/engine` (`start_sign_round` consumed-attempt checks).
- Persisted-state validation tests:
  - `engine::tests::persisted_session_state_rejects_empty_consumed_attempt_id`
  - `engine::tests::persisted_session_state_rejects_duplicate_consumed_attempt_id`
  - `engine::tests::persisted_session_state_rejects_consumed_attempt_registry_over_limit`
- Replay enforcement tests:
  - `engine::tests::start_sign_round_rejects_consumed_attempt_id_when_sign_cache_is_missing`
  - `engine::tests::start_sign_round_attempt_replay_guard_survives_process_restart_with_sign_cache_loss`
- Capacity fail-closed test:
  - `engine::tests::start_sign_round_rejects_when_consumed_attempt_registry_is_at_capacity_with_attempt_context`

## Remaining Phase 3 Work

1. No open blocking items for Phase 3 transcript/replay scope. Next protocol
   increment is Phase 4 liveness policy and recovery behavior:
   `pkg/tbtc/signer/docs/roast-phase-4-liveness-policy-recovery.md`.
