# ROAST Phase 4: Liveness Policy And Recovery Behavior

Date: 2026-02-27
Status: In progress
Owner: Threshold Labs
Scope: establish explicit recoverable-vs-terminal semantics for signer failures

## Objective

Begin Phase 4 by making retry/abort intent explicit in signer error responses,
so keep-core and operators can distinguish transient/retryable failures from
terminal/session-ending failures using machine-readable fields.

## Decisions Implemented In This Increment

1. Added `EngineError::recovery_class()` classification in
   `pkg/tbtc/signer/src/errors.rs` with values:
   - `recoverable`
   - `terminal`
2. Extended signer FFI error payloads with `ErrorResponse.recovery_class` in
   `pkg/tbtc/signer/src/api.rs` and `pkg/tbtc/signer/src/ffi.rs`.
3. Preserved existing `code`/`message` error contract while adding explicit
   recovery intent for policy/telemetry consumers.
4. Added `frost_tbtc_roast_liveness_policy` FFI endpoint and
   `engine::roast_liveness_policy()` with an env-configurable coordinator
   timeout (`TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS`, default `30000ms`).
5. Extended `AttemptTransitionEvidence` with structured
   `exclusion_evidence` (`reason`, `excluded_member_identifiers`,
   `invalid_share_proof_fingerprint`) and validated it on attempt advancement.
6. Added structured transition telemetry on successful attempt advancement via
   `RoundState.attempt_transition_telemetry` (from/to attempt, from/to
   coordinator, reason, excluded members, `coordinator_rotated`).

## Rationale

- Phase 4 requires a clear distinction between retryable and terminal failures.
- Keep-core retry loops and future liveness policies should not infer retry
  semantics from error text.
- This change is additive: existing error code handling remains intact.

## Evidence (Code + Tests)

- Recovery classification method + unit test:
  - `pkg/tbtc/signer/src/errors.rs`
  - `errors::tests::recovery_class_maps_retryable_and_terminal_errors`
- FFI payload extension:
  - `pkg/tbtc/signer/src/api.rs` (`ErrorResponse`)
  - `pkg/tbtc/signer/src/ffi.rs` (`error_result`)
- API-level assertions:
  - `pkg/tbtc/signer/src/lib.rs`
  - `run_dkg_rejects_conflicting_repeat_request_for_same_session`
  - `roast_liveness_policy_reports_default_contract`
  - `start_and_finalize_sign_round_rejects_synthetic_contributions_when_bootstrap_disabled`
  - `start_sign_round_returns_session_finalized_after_finalize`
  - `start_sign_round_returns_session_not_found_for_unknown_session`
  - `build_taproot_tx_rejects_invalid_input_txid_hex`
- Timeout policy parser validation:
  - `pkg/tbtc/signer/src/engine`
  - `roast_coordinator_timeout_ms_env_parser_is_strict_bounds`
- Exclusion/blame evidence validation:
  - `pkg/tbtc/signer/src/api.rs` (`AttemptExclusionEvidence`, `AttemptTransitionEvidence`)
  - `pkg/tbtc/signer/src/engine` (`validate_transition_exclusion_evidence`)
  - `start_sign_round_rejects_next_attempt_without_exclusion_evidence`
  - `start_sign_round_rejects_timeout_reason_with_invalid_share_fingerprint`
  - `start_sign_round_accepts_invalid_share_proof_exclusion_evidence`
  - `start_sign_round_rejects_invalid_share_proof_without_fingerprint`
  - `start_sign_round_rejects_invalid_share_proof_with_empty_fingerprint`
- Transition telemetry assertions:
  - `pkg/tbtc/signer/src/api.rs` (`AttemptTransitionTelemetry`)
  - `start_sign_round_allows_next_attempt_with_valid_transition_evidence`
  - `start_sign_round_accepts_invalid_share_proof_exclusion_evidence`
- FFI header contract update:
  - `pkg/tbtc/signer/include/frost_tbtc.h`
- Phase 4 liveness, exclusion-evidence, and transition-telemetry evidence is
  summarized in this document.
- Contract documentation alignment:
  - `pkg/tbtc/signer/README.md` (`FFI contract` section now includes `recovery_class`)

## Remaining Phase 4 Work

1. Wire keep-core runtime to consume and enforce the signer-exported
   coordinator-timeout policy contract.
2. Wire keep-core attempt-transition flow to emit/consume
   `exclusion_evidence` (`coordinator_timeout` vs `invalid_share_proof`) and
   map runtime faults into the schema.
3. Wire keep-core consumers to ingest `attempt_transition_telemetry` and
   propagate it into operator-facing logs/metrics.
4. Build end-to-end liveness tests with injected coordinator/member failures
   and traceable recovery outcomes.
