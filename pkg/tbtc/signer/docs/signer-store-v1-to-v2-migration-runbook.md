# Sign-Store v1 to v2 Migration Runbook

## Audience

Operators handling a signer node that fails to start because its durable store
journal carries the retired v1 transcript. This runbook assumes no Rust
knowledge; every step uses shell commands an operator can paste into a
maintenance session.

## Background

The signer's durable store protects an anti-rollback chain by anchoring every
state commitment to a *store fingerprint*. There are two transcript versions:

- **v1 (retired)** — the fingerprint mixed the 32-byte `.store-id` with four
  volatile inputs: the canonical path fingerprint, the filesystem fingerprint,
  the lock-file fingerprint, and the `.store-id` itself. Any silent change to
  path, device, lock, or rename invalidated every committed record and left
  the signer unstartable.
- **v2 (current)** — the fingerprint binds ONLY the stable, fsynced `.store-id`
  bytes. Path, device, inode, and lock-file descriptors are still validated on
  every access for substitution defense, but they are NOT part of the
  committed transcript. A v2 commitment stays valid across path, device,
  inode, and lock changes as long as `.store-id` is preserved.

The on-disk `.state-witness` journal header carries a 16-byte magic that names
the transcript version it was written under: `TBTCWITNESSv2\0\0\0` for v2,
`TBTCWITNESSv1\0\0\0` for v1. The current build never writes v1 and never
repairs a v1 journal in place; it rejects any v1 journal it encounters so the
store fails closed with an actionable migration error instead of a generic
"invalid commitment".

## Symptoms

The signer refuses to start with an `EngineError::Internal` whose message
contains:

> signer state witness journal uses the retired v1 state-commitment transcript
> (magic [TBTCWITNESSv1]); this build commits under v2, whose store
> fingerprint binds only the stable .store-id bytes. ...

The message embeds the full 4-step recovery procedure (see below) and ends
with an explicit warning that the journal must not be deleted.

## Pre-flight

1. Locate the durable store directory (the directory containing the
   `.state-witness` journal for the affected signer).
2. Confirm the store is on v1 by reading the first 16 bytes of `.state-witness`:

       head -c 16 .state-witness | od -An -tx1

   The first 16 bytes must be exactly:

       54 42 54 43 57 49 54 4e 45 53 53 76 31 00 00 00

   which spells `TBTCWITNESSv1\0\0\0`. If the bytes do not begin with the
   `TBTCWITNESSv1` literal, the store is not on v1 and this runbook does not
   apply; investigate other journal damage instead.
3. Confirm `.store-id` exists in the same directory and is exactly 32 bytes
   long.
4. Stop the signer process before any further action.

## Procedure

The four steps below are the same procedure the error message embeds; they
are repeated here so the operator does not have to copy prose out of a log
line.

1. **Stop the signer process.** Ensure the process is fully exited and the
   store's exclusive lock is released before touching any file in the store
   directory.

2. **Rename the existing `.state-witness` journal aside. Do NOT delete it.**
   Pick a non-conflicting name; a timestamp makes a recoverable choice:

       mv .state-witness .state-witness.v1-retired-$(date -u +%Y%m%dT%H%M%SZ)

   The v1 journal is preserved byte-for-byte on disk under the new name and
   remains available for forensic analysis or rollback.

3. **Restart the signer with the new ABI.** The new build will find the
   `.state-witness` slot empty and regenerate the journal from scratch at
   generation 1, accepting the v1→v2 break as a one-time migration event.
   `.store-id` and any state image are preserved; only the anti-rollback
   chain is reset.

4. **Verify the migration.** After restart, confirm the new journal's genesis
   fingerprint matches the v2 fingerprint derived from the existing `.store-id`
   bytes. The signer logs the store fingerprint on startup; it MUST equal the
   v2 fingerprint for the existing `.store-id`. If the printed fingerprint
   does not match, halt and roll back from backup — do NOT continue signing.

## Verification

After the signer restarts, confirm the migration succeeded:

- The signer starts cleanly without the v1 rejection error.
- The new `.state-witness` journal exists and its first 16 bytes are
  `TBTCWITNESSv2\0\0\0`:

      head -c 16 .state-witness | od -An -tx1
      # 54 42 54 43 57 49 54 4e 45 53 53 76 32 00 00 00

- The startup log includes the v2 store fingerprint derived from the existing
  `.store-id`. The fingerprint MUST match the v2 transcript computation over
  the unchanged `.store-id` bytes; if the operator has tooling to recompute
  the v2 fingerprint, compare it directly. Any mismatch means the migration
  is incomplete.
- The first committed record is at generation 1 with a `PREPARE` and `COMMIT`
  pair, not a continuation of the v1 chain.

If verification fails, the migration is incomplete. Do not bring the signer
into a threshold set until the failure is diagnosed; see Rollback.

## Rollback

A v1 journal cannot be re-anchored under v2 — the v1 fingerprint domain is
deliberately incompatible with v2's transcript, and the v2 build will reject
any v1 journal on disk. Once v2 state advances (the journal is regenerated
or a single record is committed), rollback to v1 code is no longer possible.

The only supported recovery from a failed migration is to restore the v1
journal from the renamed backup (`.state-witness.v1-retired-<timestamp>`)
back to `.state-witness` and re-deploy the v1 build:

    mv .state-witness.v1-retired-<timestamp> .state-witness

If the rename in step 2 above was not performed, recovery requires the
operator's own snapshot of the store directory; the renamed file is the
documented source of truth.

After restore, the signer is back on v1 and the migration can be re-attempted
from the top of this runbook.

## Network coordination

The anti-rollback chain is local to each signer; the on-chain threshold set
does not enforce a coordinated migration. However:

- **Every signer in a threshold set MUST complete this migration before any
  signer runs the new v2 build under load.** A partial-upgrade state is not
  safe for signing: a signer still on v1 cannot verify a v2 commitment, and
  a signer already on v2 will reject the v1 commitments still being produced
  by its peers. Cross-version signing attempts will fail closed.
- **Migrate during a planned maintenance window** so the threshold set is
  either fully on v1 or fully on v2 at every instant.
- **Coordinate the rollout order** so an operator of any single signer can
  roll back to v1 (via backup restore) without leaving the threshold set in
  a cross-version state.

## References

- The recovery procedure is implemented and discoverable via the Rust
  function `retired_v1_state_witness_journal_recovery_steps` in
  `pkg/tbtc/signer/src/engine/store.rs`.
- The rejection error path is `retired_v1_state_witness_journal_error` in the
  same file, triggered when `is_retired_v1_state_witness_journal` recognizes
  the v1 magic in `.state-witness`.
