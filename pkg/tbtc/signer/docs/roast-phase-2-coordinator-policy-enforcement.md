# ROAST Phase 2: Coordinator Policy Enforcement

Date: 2026-02-27
Status: Complete
Owner: Threshold Labs
Scope: enforce active attempt/coordinator policy in signer flows with authenticated attempt advancement checks

## Objective

Enforce attempt-context consistency across signer `StartSignRound` and
`FinalizeSignRound` calls so stale/future/mismatched attempt payloads fail
closed under ROAST strict mode.

## Decisions Implemented In This Increment

1. Added per-session `active_attempt_context` state in signer runtime and
   persisted session state.
2. Enforced active-attempt matching when an attempt context is active:
   - missing `attempt_context` rejects in strict mode and remains accepted in
     non-strict compatibility mode,
   - stale attempt number (`< active`) rejects,
   - future attempt number (`> active`) requires valid transition evidence and
     otherwise rejects fail-closed,
   - coordinator mismatch rejects,
   - participants/fingerprint/attempt-id mismatch rejects.
3. Bound finalize attempt context to the active start attempt context to prevent
   coordinator/attempt drift between phases.
4. Added cleanup semantics: active attempt context is cleared with other signing
   material on finalize lifecycle teardown.
5. Added explicit `attempt_transition_evidence` contract validation for
   `attempt_number` advancement:
   - only `N -> N+1` is accepted,
   - previous attempt/coordinator fields must match active context,
   - `previous_round_id` and `previous_sign_request_fingerprint` must match
     active signer session state,
   - new attempt ID must differ from active attempt ID.
6. Added deterministic coordinator authorization parity with keep-core
   `pkg/frost/roast.SelectCoordinator` policy:
   - signer recomputes coordinator from canonical included participants,
     message-derived attempt seed, and attempt number,
   - request `coordinator_identifier` must match the deterministic selection
     result or the request is rejected fail-closed.

## Evidence (Code + Tests)

- State model updates:
  `pkg/tbtc/signer/src/engine` (`SessionState.active_attempt_context`,
  `PersistedSessionState.active_attempt_context`).
- Policy enforcement helper:
  `pkg/tbtc/signer/src/engine` (`enforce_active_attempt_context_match`).
- Deterministic coordinator selector parity helper:
  `pkg/tbtc/signer/src/go_math_rand.rs`
  (`select_coordinator_identifier`).
- Start stale-attempt rejection:
  `engine::tests::start_sign_round_rejects_stale_attempt_number_against_active_attempt_context`.
- Start future-attempt rejection:
  `engine::tests::start_sign_round_rejects_future_attempt_number_without_transition_authorization`.
- Start next-attempt acceptance with valid evidence:
  `engine::tests::start_sign_round_allows_next_attempt_with_valid_transition_evidence`.
- Start next-attempt acceptance with valid evidence after restart/reload:
  `engine::tests::start_sign_round_allows_next_attempt_with_valid_transition_evidence_after_reload`.
- Start next-attempt rejection with invalid evidence:
  `engine::tests::start_sign_round_rejects_next_attempt_with_invalid_transition_evidence`.
- Start far-future attempt rejection even with evidence:
  `engine::tests::start_sign_round_rejects_far_future_attempt_even_with_transition_evidence`.
- Start stale-attempt rejection remains enforced after authorized transition and
  restart/reload:
  `engine::tests::start_sign_round_rejects_stale_attempt_after_authorized_transition_across_reload`.
- Start non-deterministic coordinator rejection (strict mode):
  `engine::tests::start_sign_round_rejects_nondeterministic_coordinator_identifier_in_roast_strict_mode`.
- Finalize coordinator mismatch rejection:
  `engine::tests::finalize_sign_round_rejects_coordinator_mismatch_against_active_attempt_context`.
- Finalize stale-attempt rejection:
  `engine::tests::finalize_sign_round_rejects_stale_attempt_number_against_active_attempt_context`.
- Non-strict finalize compatibility with active attempt context:
  `engine::tests::finalize_sign_round_accepts_missing_attempt_context_when_not_strict_with_active_attempt_context`.
- Non-strict finalize compatibility after restart/reload:
  `engine::tests::finalize_sign_round_accepts_missing_attempt_context_after_reload_when_not_strict`.
- Non-strict payload mismatch remains conflict-classified:
  `engine::tests::start_sign_round_returns_session_conflict_for_attempt_context_presence_mismatch_in_non_strict_mode`.

## Remaining Work

1. No open blocking items for Phase 2 coordinator-policy scope. Next protocol
   increment is Phase 3 transcript/replay hardening.
