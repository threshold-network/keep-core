# Signer API Contract Decision Brief

Date: February 23, 2026

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

### Rewrite plan and `tbtc-signer` use coarse session operations

The rewrite plan defines:

- `RunDKG(session_id, participants, threshold)`
- `StartSignRound(session_id, message, key_group)`
- `FinalizeSignRound(session_id, round_contributions)`
- `BuildTaprootTx(...)`
- `RefreshShares(...)`

(plan tracked in `pkg/tbtc/signer/docs/rust-rewrite-bootstrap.md`)

The bootstrap Rust crate already exposes this coarse C ABI surface:

(file: `pkg/tbtc/signer/src/lib.rs`)

## Design Alternatives

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
