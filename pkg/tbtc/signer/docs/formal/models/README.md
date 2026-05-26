# Formal Verification Models

This directory contains executable TLA+ models used for signer hardening
formal-verification checks.

Model coverage:

- `RoastAttemptStateMachine.tla`:
  ROAST attempt-transition invariants (attempt monotonicity, coordinator and
  cohort safety, replay rejection shape). This model does not cover the full
  Aborted/Completed/Nonce lifecycle.
- `StateKeyProviderPolicy.tla`:
  encrypted-state key/provider policy invariants aligned with PR #82 surfaces
  (provider policy gating + key-id binding fail-closed).
- `TeeEnforcementModes.tla`:
  TEE enforcement mode transition and admission invariants aligned with PR #88
  policy surfaces.
- `RoastRolloutPolicy.tla`:
  Phase 5 staged rollout and rollback transition guards (canary progression,
  rollback preconditions, and halted-mode terminal behavior).

Model bounds:

- `RoastAttemptStateMachine.tla` is currently bounded to participants `{1,2,3,4}`
  and max attempt `6` for exhaustive TLC search in CI.
- `StateKeyProviderPolicy.tla` uses profile/provider/key-id finite domains that
  represent policy transitions, not arbitrary unbounded inputs.
- `TeeEnforcementModes.tla` uses finite mode and attestation domains.
- `RoastRolloutPolicy.tla` uses finite rollout stages and trigger booleans to
  exhaustively check stage/rollback constraints.

Traceability matrix:

- `RoastAttemptStateMachine.tla`:
  `MonotonicAttemptNumber`, `ReplaySafe` ->
  `validate_attempt_context`, replay guards in start/finalize flow in
  `tools/tbtc-signer/src/engine.rs`.
- `StateKeyProviderPolicy.tla`:
  `LoadSuccessImpliesExactBinding`, `FailClosedDisallowedProvider` ->
  `decode_encrypted_state_envelope`, `encode_encrypted_state_envelope` in
  `tools/tbtc-signer/src/engine.rs`.
- `TeeEnforcementModes.tla`:
  `EnforceModeRequiresValidAttestationWithoutOverride`,
  `NoDirectDisabledToEnforceTransition` -> policy design in
  `docs/frost-migration/tee-whitelisted-signer-enforcement-plan.md`.
- `RoastRolloutPolicy.tla`:
  `BroadRequiresCanaryHistory`, `RollbackTransitionRequiresTrigger`,
  `CanaryHoldBlocksPromotion`, `BootstrapCannotJumpToBroad`,
  `EmergencyStopBlocksForwardProgress`, `HaltedModeIsTerminal` ->
  `docs/frost-migration/roast-phase-5-security-rollout-gates.md`.

Run all models with:

```bash
scripts/formal/run_tla_models.sh
```
