# Rust Rewrite Bootstrap (tbtc-signer)

Date: 2026-02-23

This document tracks the initial code bootstrap for the `tbtc-signer` Rust
rewrite architecture.

## Implemented in this branch

> Scope note: this section records the broader `tbtc-signer` rust-rewrite
> bootstrap effort across keep-core, not the diff of a single PR. Bullets that
> cite a `threshold-network/keep-core` PR or commit (e.g. the `BuildTaprootTx`
> CGO bridge wiring, the transitional bootstrap-signing orchestration) live in
> those **separate keep-core changes** and are **not part of this crate's PR
> diff**. Within this PR the crate is standalone: it builds the `cdylib` and C
> header, but nothing in keep-core's Go build links it yet (no `cgo`/`libfrost`
> consumer references the crate). See the production gate below before treating
> any of this as wired.

- Added `pkg/tbtc/signer` Rust crate that builds a `cdylib` named
  `libfrost_tbtc`.
- Added a C ABI contract in `pkg/tbtc/signer/include/frost_tbtc.h`.
- Implemented coarse request/response operations keyed by `session_id`:
  - `frost_tbtc_run_dkg`
  - `frost_tbtc_start_sign_round`
  - `frost_tbtc_finalize_sign_round`
  - `frost_tbtc_build_taproot_tx`
  - `frost_tbtc_refresh_shares`
- Implemented idempotency and conflict checks for retried operations under the
  same session ID.
- Added file-backed persistent session-state adapter with atomic writes and
  schema-validated reload for crash/restart recovery scaffolding.
  - Storage path: `TBTC_SIGNER_STATE_PATH` when set, otherwise temp-dir default
    `frost_tbtc_engine_state.json` for non-production bootstrap runs. The
    production profile rejects the implicit temp-dir state path. Operators must
    settle `TBTC_SIGNER_PROFILE`, `TBTC_SIGNER_STATE_PATH`, and key-provider
    environment before the first signer FFI call because the engine state handle
    is initialized once per process.
  - Durability semantics: temp-state file is `sync_all`'d before rename, then
    parent directory is synced after rename to close power-loss persistence gaps.
- Added persistence hardening guardrails for state storage:
  - process-level state lock file (`<state-file>.lock`) with non-blocking
    exclusive lock acquisition to prevent concurrent writer processes,
  - load/persist operations are bound to the active lock path in-process (do
    not follow later env-var path changes),
  - corruption policy defaults to fail-closed and can be set to
    `quarantine_and_reset` via `TBTC_SIGNER_STATE_CORRUPTION_POLICY`,
  - existing empty state-file loads emit warning diagnostics instead of silent
    reset behavior,
  - corrupted backup retention cap via
    `TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT` (default `5` backups).
  - added regression coverage proving in-process state-path switching is
    rejected after lock acquisition.
  - added crash-matrix coverage for truncated (partial-write-like) state-file
    payload handling under both fail-closed and quarantine-and-reset policies.
  - added crash-matrix coverage for schema-version mismatch recovery policy
    behavior under both fail-closed and quarantine-and-reset modes.
  - added fault-injection crash-matrix coverage for persist-path failures
    before rename and after rename:
    - pre-rename failure preserves prior durable state after restart and
      cleans up temp state artifacts,
    - post-rename failure (before directory sync) still yields a loadable
      persisted snapshot after restart.
  - added regression coverage for true multi-process state-lock contention.
  - added integration restart/reload coverage proving persisted multi-session
    state recovers after simulated process restart, idempotent retries remain
    stable after reload, and new sessions can progress post-recovery.
- Wired `frost-secp256k1-tr` primitives for coarse signing sessions:
  - `RunDkg` uses deterministic dealer key generation and derives `key_group`
    from the FROST verifying key,
  - `StartSignRound` emits member-scoped real signature-share contributions and
    supports optional `signing_participants` for explicit signing cohorts
    (`None` defaults to all DKG participants),
  - deterministic round nonce derivation now binds directly to message bytes
    (in addition to `round_id`) as defense-in-depth against future
    round-ID-schema drift,
  - deterministic seed framing is now length-prefixed per input component to
    avoid delimiter ambiguity when binary fields contain embedded `0x00` bytes,
  - `FinalizeSignRound` enforces real-contribution membership against the
    resolved signing cohort and aggregates Schnorr signatures over that cohort,
    while preserving bootstrap synthetic-contribution compatibility.
  - `FinalizeSignRound` now returns an explicit validation error when real
    contribution identifiers do not exactly match the round signing cohort,
    avoiding opaque aggregate-signature failures for contributor-set mismatch.
  - Added regression coverage proving real finalize rejects contribution
    identifiers outside the resolved signing cohort before aggregation.
- Replaced version-suffix bootstrap gating with explicit fail-closed runtime
  control for synthetic finalize payloads:
  - `FinalizeSignRound` synthetic-contribution acceptance now requires
    `TBTC_SIGNER_ALLOW_BOOTSTRAP=true` in a non-production profile,
  - default behavior is fail-closed (synthetic finalize payloads are rejected),
  - `TBTC_SIGNER_PROFILE=production` forces bootstrap synthetic finalize
    rejection even if the bootstrap env flag is set,
  - `TBTC_SIGNER_PROFILE=production` requires an explicit
    `TBTC_SIGNER_STATE_PATH` and rejects the implicit temp-dir state path,
  - `TBTC_SIGNER_PROFILE=production` rejects bootstrap dealer DKG before session
    state mutation so the dealer-only path cannot be used as production DKG,
  - `TBTC_SIGNER_PROFILE=production` forces ROAST strict attempt-context
    enforcement even if `TBTC_SIGNER_ENABLE_ROAST_STRICT` is unset or false,
  - added FFI coverage for enabled/disabled bootstrap finalize behavior and
    strict env-flag parsing.
- Replaced placeholder `BuildTaprootTx` behavior with `rust-bitcoin` transaction
  assembly for unsigned version-1 transactions:
  - validates non-empty input/output sets and parses input txids, P2TR prevout
    scripts, and output scripts,
  - validates `value_sats` accounting to reject overspend payloads
    (`output_total > input_total`),
  - validates input/output value-sum arithmetic for `u64` overflow safety,
  - validates per-input/per-output `value_sats` against Bitcoin max-money
    bounds and rejects duplicate input outpoints (`txid:vout`),
  - returns serialized transaction hex plus the ordered BIP-341 key-spend
    `SIGHASH_DEFAULT` messages derived from the transaction and all prevouts,
  - adds session-keyed idempotency/conflict semantics for repeated build calls,
  - binds interactive Open/Round2 to the Build artifact stored on the fresh
    per-signing session while resolving DKG/lifecycle state from the unique
    wallet session, and rechecks active non-rate policy before share release,
  - explicitly rejects `script_tree_hex` until full script-tree semantics are
    implemented (no silent ignore behavior).
- Wired keep-core wallet orchestration to route unsigned transaction shape data
  through the native tbtc-signer `BuildTaprootTx` CGO bridge path:
  - added canonical unsigned transaction I/O extraction on
    `bitcoin.TransactionBuilder`,
  - extended keep-core native tbtc-signer engine registration/CGO contract
    with `BuildTaprootTx` request/response handling,
  - invoked `BuildTaprootTx` from wallet transaction signing before sig-hash
    computation and surfaced returned `tx_hex` at coordinator runtime,
  - added focused unit coverage for request/response encoding, bridge
    unavailability handling, and transaction I/O extraction.
- Wired keep-core transitional bootstrap signing orchestration to pass explicit
  threshold cohorts via `StartSignRound.signing_participants`, validate
  round-state cohort consistency, and add non-full cohort coverage
  (`threshold-network/keep-core` PR:
  `https://github.com/threshold-network/keep-core/pull/3868`).
- Added keep-core bootstrap attempt-variation coverage for same-session cohort
  changes, asserting `StartSignRound` cohort inputs across retries and
  `session_conflict` fallback propagation for mismatched retry cohorts
  (`threshold-network/keep-core` commit `69e844216`).
- Added keep-core non-bootstrap native FROST cohort-attempt variation coverage:
  two signing rounds with different attempt cohorts (`[1,2,3]` then `[1,3]`)
  now validate commitment/signature-share participant sets in
  `NewSigningPackage` and `Aggregate`
  (`threshold-network/keep-core` commit `9ff880422`).
- Added keep-core signer-executor runtime retry integration coverage for
  strict native FFI mode, proving cohort changes across attempts propagate
  through runtime `Attempt` fields passed to native signing execution
  (`threshold-network/keep-core` commit `d63d08bdd`).
- Added keep-core transitional `frost-tbtc-signer-v1` runtime retry/cohort
  integration coverage under strict native FFI mode:
  one signer is forced to miss legacy fallback share material, attempt-1
  includes that signer and fails, attempt-2 excludes it and succeeds, and
  `StartSignRound.signing_participants` cohorts are asserted across attempts
  (`threshold-network/keep-core` commit `7814f81a9`).
- Added post-finalize signing-material cleanup in `pkg/tbtc/signer` session
  state: on successful finalize, bootstrap DKG key packages, DKG public key
  package cache, sign-request fingerprint, sign message bytes, and round state
  are removed while preserving finalize idempotency cache.
- Added finalized-session guardrails in `pkg/tbtc/signer`: subsequent
  `StartSignRound` calls for an already-finalized session return
  `session_finalized`, preventing round restart and nonce/key-material reuse on
  the same session ID.
- Added best-effort zeroization during post-finalize material purge for
  directly owned signing buffers/strings (`sign_request_fingerprint`,
  `sign_message_bytes`, `round_state.session_id`, `round_state.round_id`,
  `round_state.message_digest_hex`, `round_state.signing_participants`,
  `own_contribution.identifier`, `own_contribution.signature_share_hex`) before
  dropping session references.
- Added nonce-lifecycle hardening for in-round ephemeral data:
  - zeroized deterministic round nonces for non-own participants immediately
    after deriving commitments,
  - zeroized own signing nonces immediately after round-2 signing (on both
    success and error paths),
  - zeroized temporary decoded signature-share byte buffers during finalize
    aggregation,
  - zeroized temporary DKG key-package byte buffers during persisted-state
    decode/encode transitions,
  - zeroized temporary serialized signature-share bytes after outbound
    contribution encoding,
  - zeroized deterministic DKG keygen seed bytes after seeding RNG,
  - zeroized transient serialized request/state buffers used for request
    fingerprinting, bootstrap synthetic finalize hashing, and persisted-state
    load/persist I/O.
- Added durable nonce single-use enforcement:
  - tracked consumed sign-round IDs and reject regenerated own contributions
    for previously-consumed `(session_id, round_id)` pairs,
  - tracked consumed finalize round IDs and reject repeated aggregate signature
    production for previously-consumed `(session_id, round_id)` pairs when
    finalize idempotency cache is unavailable.
- Added durable finalize replay safeguards:
  - tracked consumed finalize request fingerprints and reject replayed finalize
    payloads when finalize idempotency cache is unavailable,
  - enforced replay rejection before `round_state` access so consumed-request
    replays still fail closed after post-finalize signing-material purge.
- Added consolidated nonce-lifecycle replay coverage:
  - mapped nonce/replay invariants to enforcement sites and tests,
  - added explicit restart-aware replay guard coverage for consumed sign-round
    IDs and consumed finalize request fingerprints when idempotency caches are
    unavailable after simulated process restart.
- Added fail-closed retention bounds for session-scoped consumed registries:
  - bounded consumed sign/finalize round-ID and finalize-request-fingerprint
    registries per session to prevent unbounded growth,
  - reject over-limit runtime insertions and over-limit persisted payloads
    instead of evicting entries (no silent replay-protection weakening).
- Added fail-closed global session-registry bounds:
  - bounded total persisted session count via `TBTC_SIGNER_MAX_SESSIONS`
    (default `1024`),
  - reject over-limit persisted state payloads during decode/encode, and reject
    new runtime session creation at capacity while preserving idempotent retries
    for existing `session_id` values.
- Bootstrap dealer-model constraint: the current engine holds all generated key
  packages for a session in one process. This is temporary bootstrap behavior
  and does not provide production threshold key isolation.
- Added unit tests for retry/idempotency and sequencing behavior.

## Deferred to follow-up increments

- Harden DKG/signing for production invariants (distributed DKG flow,
  cryptographic RNG policy, crash-safe recovery).
- Implement ROAST coordinator semantics.
  Implementation roadmap:
  `pkg/tbtc/signer/docs/roast-implementation-plan.md`.
- Extend `BuildTaprootTx` with full Taproot script-tree construction/signing
  policy semantics (current bootstrap path assembles validated unsigned txs).
- Define canonical serialization rules and compatibility tests beyond JSON.
- Expand persistence crash-matrix coverage beyond current truncated-state,
  multi-process lock-contention, and integration restart/reload cases,
  including broader filesystem fault-injection and keep-core wiring scenarios.
- Consolidate and document cohort-retry coverage evidence across protocol-level
  and runtime-level tests for external review packet updates.

### Future consideration only (non-committed): true late t-of-n finalize

- This is a potential future direction, not a committed delivery item, and it
  may not be implemented.
- Detailed discussion draft:
  `pkg/tbtc/signer/docs/true-late-t-of-n-finalize-considerations.md`.

- Current posture: we support early subset selection (`signing_participants` at
  `StartSignRound`), but not late subset selection after shares are already
  produced for a larger cohort.
- Candidate behavior: allow finalize-time selection of any responding subset
  `S` where `|S| >= threshold`, with signing packages and commitments bound to
  that exact subset.
- Potential benefits:
  - improved liveness under mid-round signer drop-off,
  - fewer full-round restarts/cohort reselections,
  - lower tail latency for retry-heavy signing conditions.
- Tradeoffs:
  - requires API/flow redesign (round state and contribution exchange),
  - increases nonce lifecycle and persistence complexity,
  - expands coordinator policy and test/review surface across Rust + keep-core
    integration.

## Production gates (must close before rollout)

- Consumer-activation re-review (mandatory): the crate currently has no Go
  consumer in keep-core, so its custody-critical surface has only been reviewed
  as inert code. Before any PR wires a Go consumer (links the `cdylib` / enables
  the `BuildTaprootTx` CGO bridge), the crate must get a dedicated security
  re-review as a now-load-bearing dependency. Treat this as a hard, mechanical
  gate, not a cultural assumption. The re-review MUST validate at least:
  - Exclusion-evidence trust: the signer applies auto-quarantine penalties from
    caller-supplied `attempt_transition_evidence` using a fault-*count* threshold
    and a hex-only `invalid_share_proof_fingerprint` check (`engine::roast`), with
    no accuser-corroboration of its own. It therefore assumes the Go ROAST layer
    has already established exclusions via `VerifyBundle` + the f+1 accuser-quorum
    `NextAttempt` policy (RFC-21 Layer B) before feeding them in. Confirm the wired
    consumer only feeds quorum-established exclusions, or add signer-side
    verification. (Coordinator-*selection* grindability is benign: RFC-21 makes a
    byzantine coordinator unable to fabricate exclusions.)
- Durable session state: complete production hardening around the persistent
  backend (crash-safe fsync semantics, path configuration, process lock model,
  corruption handling policy, and broader retention/cleanup lifecycle
  management across sessions; session-scoped consumed-registry and global
  session-registry bounds are implemented) and prove behavior across
  integration crash matrix scenarios
  (including power-loss during persist and multi-process lock contention).
- Nonce lifecycle controls: complete external security review sign-off for
  replay guarantees across retries/restarts and track dependency-level
  zeroization limitations (single-use enforcement, finalize replay safeguards,
  restart-aware replay audit coverage, and transient-buffer zeroization
  hardening are implemented).
- Distributed DKG: bootstrap dealer DKG is fail-closed in the production
  profile; replace it with production distributed DKG wiring and evidence before
  enabling production activation.
- Threshold signing semantics: replace bootstrap n-of-n finalize strictness with
  t-of-n contribution handling and filter signing-package commitments to actual
  contributing participants; complete keep-core cohort-selection wiring and
  non-full-cohort integration coverage.
- Refresh epoch policy: keep `refresh_epoch` monotonic via internal counter
  semantics (do not use wall-clock values for refresh ordering).

## Validation command

```bash
cd pkg/tbtc/signer
cargo test
```
