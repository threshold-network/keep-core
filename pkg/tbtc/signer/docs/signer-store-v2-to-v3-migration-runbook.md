# Sign-Store v2 to v3 Migration Runbook

## Audience

Operators handling a signer node that fails to start because its durable
store journal carries the retired v2 record layout. This runbook assumes no
Rust knowledge; every step uses shell commands an operator can paste into a
maintenance session.

## Background

The signer's durable store protects an anti-rollback chain by anchoring every
state commitment to a *store fingerprint* and by recording every state
write as a fixed-width PREPARE/COMMIT pair in a `.state-witness` journal.
There are three record-layout versions:

- **v1 (retired)** — superseded by v2. Recognized by the `TBTCWITNESSv1`
  magic.
- **v2 (retired)** — pre-hash-chain layout. Every record is exactly 105 bytes:
  one-byte record type, eight-byte generation, and three 32-byte fields
  (previous commitment, state-image digest, commitment). The record is
  self-verifying through its commitment but the journal itself carries no
  per-record chaining: an attacker who can rewrite a historical record and
  recompute its commitment can do so without invalidating any later record's
  commitment.
- **v3 (current)** — adds a 32-byte `chain_hash` field to every record,
  making the record exactly 137 bytes. The chain_hash commits to all
  preceding records via a domain-separated SHA-256 link, so any historical
  tamper with the journal is detectable on reload even when the
  state-commitment chain itself is unchanged.

The on-disk `.state-witness` journal header carries a 16-byte magic that
names the record-layout version it was written under: `TBTCWITNESSv3\0\0\0`
for v3, `TBTCWITNESSv2\0\0\0` for v2, `TBTCWITNESSv1\0\0\0` for v1. The current
build never writes v1 or v2 and never repairs a v2 journal in place; it
rejects any v2 journal it encounters so the store fails closed with an
actionable migration error instead of a generic "missing or partial record"
from the 105 vs 137 byte record-length mismatch.

The 472-byte signed segment header is unchanged between v2 and v3, so a v3
signer can still parse and verify a v2 segment header followed by v2
records - but the records themselves fail closed under the new layout. A
v2 journal on disk therefore fails closed at its very first record, not
after silent parsing.

## Symptoms

The signer refuses to start with an `EngineError::Internal` whose message
contains:

> signer state witness journal uses the retired v2 record layout (magic
> [TBTCWITNESSv2]); this build commits under v3, which adds a 32-byte
> per-record hash chain and grows every record from 105 to 137 bytes. ...

The message embeds the full 4-step recovery procedure (see below) and ends
with an explicit warning that the journal must not be deleted.

## Pre-flight

1. Locate the durable store directory (the directory containing the
   `.state-witness` journal for the affected signer).
2. Confirm the store is on v2 by reading the first 16 bytes of
   `.state-witness`:

       head -c 16 .state-witness | od -An -tx1

   The first 16 bytes must be exactly:

       54 42 54 43 57 49 54 4e 45 53 53 76 32 00 00 00

   which spells `TBTCWITNESSv2\0\0\0`. If the bytes do not begin with the
   `TBTCWITNESSv2` literal, the store is not on v2 and this runbook does not
   apply; if they begin with `TBTCWITNESSv1`, use the v1-to-v2 runbook
   instead; otherwise investigate other journal damage.
3. Confirm `.store-id` exists in the same directory and is exactly 32 bytes
   long. The v2-to-v3 migration preserves the store fingerprint (it depends
   on `.store-id` only); any later restart that cannot find the unchanged
   `.store-id` will regenerate the chain against a brand-new fingerprint,
   which is the wrong outcome.
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

       mv .state-witness .state-witness.v2-retired-$(date -u +%Y%m%dT%H%M%SZ)

   The v2 journal is preserved byte-for-byte on disk under the new name and
   remains available for forensic analysis or rollback. Do NOT modify it
   in place - the migration error is built on the assumption that the bytes
   on disk are exactly the bytes that were originally written under v2.

3. **Restart the signer with the new ABI.** The new build will open the
   fresh `.state-witness` and regenerate the journal from scratch at
   generation 1, accepting the v2-to-v3 break as a one-time migration event.
   `.store-id` and any state image are preserved; only the anti-rollback
   chain is reset and the new chain is anchored on a per-record hash chain
   from the new genesis onwards. Confirm the new journal's magic-byte header
   reads `TBTCWITNESSv3\0\0\0` (see *Verification* below) and that `.store-id`
   is byte-for-byte unchanged as evidence the migration completed.

## Verification

After the signer restarts, confirm the migration succeeded:

- The signer starts cleanly without the v2 rejection error.
- The new `.state-witness` journal exists and its first 16 bytes are
  `TBTCWITNESSv3\0\0\0`:

      head -c 16 .state-witness | od -An -tx1
      # 54 42 54 43 57 49 54 4e 45 53 53 76 33 00 00 00

- The `.store-id` file is byte-for-byte unchanged from before migration
  (compare against a pre-migration snapshot if one was taken).
- The first committed record is at generation 1 with a `PREPARE` and
  `COMMIT` pair, not a continuation of the v2 chain. The new `COMMIT`
  record carries a 32-byte `chain_hash` field at offset 105..137; verify
  it matches the domain-separated SHA-256 link from the genesis chain hash.
- The journal record count advances by one PREPARE/COMMIT pair per state
  write, as before.

If verification fails, the migration is incomplete. Do not bring the signer
into a threshold set until the failure is diagnosed; see Rollback.

## Rollback

A v2 journal cannot be re-anchored under v3 - the v3 build will reject any
v2 journal on disk, and v2 builds lack the per-record chain_hash
verification that v3 uses to detect historical tamper. Once v3 state
advances (the journal is regenerated or a single record is committed),
rollback to v2 code is no longer possible.

The only supported recovery from a failed migration is to restore the v2
journal from the renamed backup (`.state-witness.v2-retired-<timestamp>`)
back to `.state-witness` and re-deploy the v2 build:

    mv .state-witness.v2-retired-<timestamp> .state-witness

If the rename in step 2 above was not performed, recovery requires the
operator's own snapshot of the store directory; the renamed file is the
documented source of truth.

After restore, the signer is back on v2 and the migration can be
re-attempted from the top of this runbook.

## Network coordination

The `.state-witness` journal is local to each signer and is never
exchanged over the signing protocol. The journal format (v2 or v3) does
not appear in any cross-signer message or FFI response. Therefore,
signers may migrate independently; there is no requirement for lockstep
timing across the threshold set. Coordinate only the downtime needed to
preserve threshold availability (i.e., ensure enough signers remain online
to meet the threshold during the migration window).

## Security model / limitations

The v3 per-record `chain_hash` is an UNKEYED SHA-256 accumulator over
the journal records. It is tamper-evident, not tamper-RESISTANT: a
same-uid attacker with equal compute can rewrite a historical record
and recompute a self-consistent downstream chain (including the
segment header's `header_commitment`) without invalidating the
`chain_hash` link, up to the next signed segment-header checkpoint or,
on an unanchored signer, up to the next local compaction.

The chain is CT-log-like: it detects rewrites against an
independently-observed prior head (a previously-seen segment header
commitment, a signed anchor checkpoint, a snapshot of the journal
taken before the rewrite). It does NOT provide a cryptographic
tamper-resistance guarantee absent one of those external anchors.

**Note:** The compaction path does NOT retain `.state-witness.previous`
on disk — it is unlinked immediately after compaction completes. A
snapshot taken before compaction is the only way to preserve the
pre-compaction journal for forensic analysis; see
`signer-store-compaction-runbook.md` for details.

Operators who need tamper-resistance rather than tamper-evidence
must ensure the signer is configured to anchor: a signed anchor
checkpoint makes segment rotation the preferred path and gives
external observers a signed prior head to detect against. Unanchored
signers rely on the local compaction path to bound the length of any
unanchored rewrite window; see `signer-store-compaction-runbook.md`
for that path.

## Filesystem dependency

The store's exclusive writer lock relies on POSIX `flock(2)` advisory
locking semantics. On some shared or network filesystems - notably
NFS, and some container overlay filesystems - `flock(2)` may be weak
or absent, and a second process on the same store directory can
acquire the lock at the same time as the first. The store will
appear to be running cleanly with two writers, and the resulting
journal corruption will only surface on reload.

Deployments MUST verify their target filesystem supports standard
POSIX advisory locking before bringing the signer into a threshold
set. Local filesystems (ext4, xfs, btrfs) and container-native
overlay filesystems with proven `flock(2)` support are acceptable.
Filesystems that do not support `flock(2)` or that map it to a
no-op MUST be replaced with a local filesystem; the signer must
not be deployed against NFS-mounted state, container volumes that
do not preserve `flock(2)` semantics, or any filesystem documented
to weaken advisory locks.

## References

- The rejection error path is `retired_v2_state_witness_journal_error` in
  `pkg/tbtc/signer/src/engine/store.rs`, triggered when
  `is_retired_v2_state_witness_journal` recognizes the v2 magic in
  `.state-witness`.
- The v2 magic is `TBTC_SIGNER_STATE_WITNESS_MAGIC_V2` in the same file;
  the v3 magic is `TBTC_SIGNER_STATE_WITNESS_MAGIC`.
- The per-record hash-chain scheme is documented at
  `TBTC_SIGNER_STATE_WITNESS_RECORD_CHAIN_DOMAIN` in the same file.
- The 472-byte segment header layout is unchanged between v2 and v3; the
  cross-language byte vector still pins
  `TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_VERSION = 1`.
- The compaction path that bounds an unanchored rewrite window is
  documented in `signer-store-compaction-runbook.md`.
