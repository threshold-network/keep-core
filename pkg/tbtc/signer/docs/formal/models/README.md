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
  `validate_attempt_context` in `src/engine/roast.rs`; replay guards in
  start/finalize flow in `src/engine/signing.rs`.
- `StateKeyProviderPolicy.tla`:
  `LoadSuccessImpliesExactBinding`, `FailClosedDisallowedProvider` ->
  `decode_encrypted_state_envelope`, `encode_encrypted_state_envelope` in
  `src/engine/persistence.rs`.
- `TeeEnforcementModes.tla`:
  `EnforceModeRequiresValidAttestationWithoutOverride`,
  `NoDirectDisabledToEnforceTransition` -> policy design in
  `pkg/tbtc/signer/docs/tee-whitelisted-signer-enforcement-plan.md`.
- `RoastRolloutPolicy.tla`:
  `BroadRequiresCanaryHistory`, `RollbackTransitionRequiresTrigger`,
  `CanaryHoldBlocksPromotion`, `BootstrapCannotJumpToBroad`,
  `EmergencyStopBlocksForwardProgress`, `HaltedModeIsTerminal` ->
  `pkg/tbtc/signer/docs/roast-phase-5-security-rollout-gates.md`.

Implementation status (read before trusting a green check):

- **Implemented** — invariants trace to shipped code:
  - `RoastAttemptStateMachine.tla` -> `src/engine/roast.rs`, `src/engine/signing.rs`.
  - `StateKeyProviderPolicy.tla` -> `src/engine/persistence.rs`.
- **Planned / not yet implemented** — invariants trace to design docs, not code:
  - `TeeEnforcementModes.tla` and `RoastRolloutPolicy.tla` model the three-mode
    (`disabled`/`audit`/`enforce`) + break-glass enforcement profile and the
    staged rollout/rollback policy. The shipped signer implements only a binary
    provenance enforce gate (`src/engine/provenance.rs`) and has no audit-mode
    ramp, break-glass path, or rollout state machine. Both trace to plans that
    are explicitly "not active" future hardening profiles
    (`tee-whitelisted-signer-enforcement-plan.md`,
    `roast-phase-5-security-rollout-gates.md`).

A passing TLC run proves each model is internally consistent; for the two
planned models it does **not** prove the shipped signer enforces that behavior.

Run all models with:

```bash
scripts/formal/run_tla_models.sh
```
