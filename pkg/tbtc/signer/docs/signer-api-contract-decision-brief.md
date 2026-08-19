# Signer API Contract Decision Brief

Date: February 23, 2026
Status: Partially adopted — see corrected FFI-surface description above

Purpose: capture the API-contract direction before further implementation work.

## Decision to make

Choose one primary integration contract between keep-core and `tbtc-signer`:

1. Round-level crypto API (current keep-core shape).
2. Coarse session API (rewrite plan shape).

## Current mismatch

### keep-core currently expects round-level calls

In `threshold-network/keep-core` on
`feat/frost-schnorr-migration-scaffold`, the native FROST engine interface is:

- `GenerateNoncesAndCommitments(...)`
- `NewSigningPackage(...)`
- `Sign(...)`
- `Aggregate(...)`

(file: `pkg/frost/signing/native_frost_engine_frost_native.go`)

`executeNativeFROSTSigning(...)` orchestrates round 1/round 2 around that
interface.

(file: `pkg/frost/signing/native_frost_protocol_frost_native.go`)

### Rewrite plan and `tbtc-signer` actual FFI surface (corrected)

The rewrite plan defines:

- `RunDKG(session_id, participants, threshold)`
- `StartSignRound(session_id, message, key_group)`
- `FinalizeSignRound(session_id, round_contributions)`
- `BuildTaprootTx(...)`
- `RefreshShares(...)`

(plan tracked in `pkg/tbtc/signer/docs/rust-rewrite-bootstrap.md`)

The actual FFI surface in `pkg/tbtc/signer/src/lib.rs` is HYBRID and does not
uniformly key on `session_id`:

- DKG stays round-level:
  - `frost_tbtc_dkg_part1` takes `DkgPart1Request { participant_identifier,
    max_signers, min_signers }` (no `session_id`).
  - `frost_tbtc_dkg_part2` takes `DkgPart2Request { secret_package_hex,
    round1_packages }` (no `session_id`).
  - `frost_tbtc_dkg_part3` takes `DkgPart3Request { secret_package_hex,
    round1_packages, round2_packages }` (no `session_id`).
- Signing-package construction stays round-level:
  - `frost_tbtc_new_signing_package` takes
    `NewSigningPackageRequest { message_hex, commitments }` (no `session_id`).
- Session-keyed (`session_id` is part of the request):
  - `frost_tbtc_build_taproot_tx` (`BuildTaprootTxRequest`).
  - `frost_tbtc_refresh_shares` (`RefreshSharesRequest`; symbol retained but
    fail-closed with `cryptographic_refresh_not_supported` until a
    multi-round FROST refresh protocol is implemented).
  - `frost_tbtc_verify_signature_share` (`VerifySignatureShareRequest`).
  - The hardened interactive signing session ops
    (`frost_tbtc_interactive_session_open`, `frost_tbtc_interactive_round1`,
    `frost_tbtc_interactive_round2`, `frost_tbtc_interactive_session_abort`,
    `frost_tbtc_interactive_aggregate`) - all keyed by
    `(session_id, attempt_id, member_identifier)` per the frozen Phase 7
    interactive-session spec.
- Wire-contract version: the `frost_tbtc_abi_version` export reports
  `abi_major = 3, abi_minor = 0` (per `TBTC_SIGNER_ABI_MAJOR` /
  `TBTC_SIGNER_ABI_MINOR` in `lib.rs`). Earlier references in this crate's
  docs to an `ABI 4.0` value for the same major are stale.

The "already exposes RunDKG / StartSignRound / FinalizeSignRound" claim in
the earlier draft of this brief is therefore an oversimplification: the
round-level DKG and signing-package construction paths are still
round-level, and only the build-tx / refresh-shares / verify-share /
interactive-session subset is session-keyed. The recommendation in the
Recommendation section below still favors the coarse session shape as the
end-state, but the current surface is a hybrid that needs to be made
explicit before any further keep-core wiring.

### Round-Level API Compatibility

Pros:

- Fastest bridge enablement.
- Minimal keep-core refactor.
- Reuses existing round orchestration and tests.

Cons:

- Diverges from rewrite-plan contract.
- Keeps nonce/round details crossing the FFI boundary.
- Harder future transport swap (CGO -> sidecar).
- Higher chance of long-term rework.

### Coarse Session API

Pros:

- Aligns with the agreed architecture.
- Better idempotency/retry semantics keyed by `session_id`.
- Smaller and more stable FFI surface for audits.
- Cleaner future sidecar extraction.

Cons:

- Requires keep-core orchestration refactor now.
- Higher short-term implementation cost.
- Existing test flows need migration.

### Temporary Compatibility Layer

Pros:

- Unblocks integration while retaining coarse API as the end-state.

Cons:

- Temporary adapter debt.
- Risk that temporary path becomes permanent.

## Recommendation

Recommend the **coarse session API** as the production direction, with the
temporary compatibility layer only as a tightly scoped bridge if needed for
delivery pace.

Justification:

- The rewrite plan explicitly selected coarse, idempotent session operations as
  a safety and operability decision, not just an API style preference.
- Custody-critical failure modes (retries, restart boundaries, nonce lifecycle)
  are easier to reason about with the session contract.
- Implementing the round-level compatibility path now likely causes a second
  refactor later.

## Immediate implications

1. Define keep-core adapter contract against coarse calls first (before wiring
   full cryptographic paths).
2. Decide where round-state ownership lives during transition.
3. Keep the new `frost_tbtc_signer` keep-core registration scaffold as-is until
   the contract choice is finalized.

## Review Questions

1. Do you agree Option B should be the production contract?
2. If yes, do you prefer:
   - direct keep-core orchestration refactor now, or
   - temporary compatibility layer first (Option C)?
3. Any blockers with auditability or operational risk assumptions in this
   recommendation?

## Review Response Summary (2026-02-23)

- Agrees strongly with Option B as production contract.
- Recommends direct refactor, with Go-side temporary shim only if test
  migration blocks timeline.
- Flags three gates:
  - persistent state backend before production,
  - explicit nonce lifecycle/reuse prevention and audit coverage,
  - monotonic refresh epoch counter (not wall-clock derived).
