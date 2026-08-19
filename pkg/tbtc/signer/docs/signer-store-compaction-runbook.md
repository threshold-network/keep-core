# Sign-Store Witness Journal Compaction Runbook

## Audience

Operators handling a signer node whose local `.state-witness` journal has been
compacted by the new minimum-viable compaction path that ships with the
follow-up to PR #4198. This runbook assumes no Rust knowledge; every step
uses shell commands an operator can paste into a maintenance session.

## Background

The signer's durable store protects an anti-rollback chain by anchoring every
state commitment to a *store fingerprint* and by recording every state
write as a fixed-width PREPARE/COMMIT pair in a `.state-witness` journal.
In v3 the journal also carries a per-record `chain_hash` field that links
every record to the previous one through a domain-separated SHA-256 link
(see `signer-store-v2-to-v3-migration-runbook.md` for the v3 record layout).

The per-record chain is tamper-evident: a same-uid attacker who can rewrite
a historical record and recompute a self-consistent downstream chain still
diverges from any independently-observed prior head, so the chain detects
rewrites against the last signed segment-header checkpoint (or, for an
unanchored signer, against the last local compaction). The chain is not
tamper-RESISTANT by itself; see the *Security Model / Limitations*
subsection of the v2-to-v3 runbook.

A long-lived signer with no signed anchor checkpoint (unanchored topology)
would otherwise grow its `.state-witness` journal indefinitely. The
follow-up branch implements a minimum-viable compaction path that:

- appends a single *compaction record* to the live `.state-witness` journal
  committing to a fresh genesis header,
- renames the live `.state-witness` to `.state-witness.previous`, and
- starts a fresh `.state-witness` containing only the new genesis header
  and zero records.

The compaction record is itself a witness record (with its own `chain_hash`
link into the prior chain), so the live journal's last entry on disk is
still the verifiable tip of the pre-compaction chain, and the new genesis
header is rooted in it. The previous journal is retained indefinitely; see
*Inspection and recovery* below.

## When this activates

The compaction path activates automatically when the unanchored record
ceiling is reached AND no signed anchor checkpoint is configured. The
ceiling is the same `state_witness_rotation_threshold` knob that already
governs segment rotation, exposed under a new ceiling name for the
compaction path. In a topology that has a signed anchor checkpoint, the
existing segment-rotation path is preferred over compaction: rotation
produces a signed header that external observers can verify, while
compaction produces only a local record.

If a signer is configured to anchor, the operator should not see this
runbook's behavior on a healthy node. If a compacted `.state-witness.previous`
appears on an anchored signer, the anchor wiring should be inspected before
the next state write.

## Pre-flight

1. Locate the durable store directory (the directory containing the
   `.state-witness` journal for the affected signer).
2. Confirm a compaction has happened by listing the journal slot:

       ls -la .state-witness .state-witness.previous 2>/dev/null

   On a healthy pre-compaction node, only `.state-witness` exists. After a
   compaction, both exist and `.state-witness.previous` is the older
   journal preserved byte-for-byte on disk under the new name. The rename
   is operator-visible: `stat .state-witness.previous` shows the
   mtime/ctime of the original write, and `stat .state-witness` shows a
   later mtime/ctime from the fresh start.
3. Confirm `.store-id` exists in the same directory and is exactly 32 bytes
   long. Compaction does NOT change the store fingerprint; the new genesis
   header chains to the same `.store-id` that the pre-compaction chain
   anchored against.
4. Stop the signer process before any further action. The store's
   exclusive lock must be released before any operator touches the journal
   files directly.

## Procedure

The four steps below mirror the procedure the v2-to-v3 runbook embeds;
they are repeated here so the operator does not have to cross-reference
two runbooks.

1. **Stop the signer process.** Ensure the process is fully exited and
   the store's exclusive lock is released before touching any file in the
   store directory.

2. **Do NOT delete `.state-witness.previous`.** The previous journal is
   retained indefinitely. It is the only on-disk evidence of the
   pre-compaction chain, and an operator can recover pre-compaction
   history from it. Treat the file as a forensic record: rename it
   aside only if your incident response workflow requires a non-default
   name, and never modify it in place.

       # The default retain name is .state-witness.previous; do not delete it.
       # If your environment requires a timestamped name instead, rename it
       # rather than copying or moving its contents:
       mv .state-witness.previous .state-witness.previous-$(date -u +%Y%m%dT%H%M%SZ)

   The file is preserved byte-for-byte under either name.

3. **Verify the live journal's first 16 bytes are the v3 magic.** The new
   `.state-witness` must begin with `TBTCWITNESSv3\0\0\0`:

       head -c 16 .state-witness | od -An -tx1
       # 54 42 54 43 57 49 54 4e 45 53 53 76 33 00 00 00

   A non-v3 magic on a freshly-compacted live journal indicates that the
   compaction path did not run as expected; halt and investigate.

4. **Restart the signer with the unchanged ABI.** The new build will open
   the fresh `.state-witness` and start a new chain at generation 1,
   preserving the store fingerprint. State writes resume against the new
   chain. `.store-id` and any state image are preserved.

## Inspection and recovery

### Verify a `.state-witness.previous` file is intact

The previous journal's magic, length, and trailing chain hash can be read
with shell commands only. Treat the read-only commands as a fingerprint
check before you trust the file for forensic recovery.

1. Magic check — the previous journal must also begin with the v3 magic
   (compaction is only meaningful on v3 journals):

       head -c 16 .state-witness.previous | od -An -tx1
       # 54 42 54 43 57 49 54 4e 45 53 53 76 33 00 00 00
       # ^ the first four bytes spell TBTC; the trailing bytes must include v3\0\0\0

2. Length check — the previous journal is 472 bytes (signed segment header)
   plus 137 bytes per record. The total length modulo 137 must equal 472
   once the header is subtracted:

       prev_len=$(stat -c %s .state-witness.previous)
       body=$((prev_len - 472))
       if [ $((body % 137)) -ne 0 ]; then
         echo "previous journal length is not 472 + N*137: corrupt"
       fi

3. Trailing chain hash check — the last 32 bytes of the previous journal
   are the chain hash of its final record. Print them and compare against
   the on-the-fly recomputation if you have tooling that knows the
   per-record chain domain. A mismatch is evidence of in-place tampering.

       tail -c 32 .state-witness.previous | od -An -tx1 -v

4. Compaction record check — the last record of `.state-witness.previous`
   is the compaction record that committed to the new genesis header.
   Confirm the new `.state-witness`'s 472-byte segment header's
   `header_commitment` field matches the last record's commitment; this
   is the link the live journal inherits. (The exact byte offsets for the
   commitment field are the same as the v2-to-v3 runbook's segment
   header layout.)

If any of these checks fail, treat the previous journal as suspect. Do
NOT delete it; rename it aside (for example
`.state-witness.previous.suspect-<timestamp>`) and escalate.

### Recovering pre-compaction history

The previous journal is the canonical pre-compaction record. To recover
historical state-commitment data from it, restart the signer with an
operator-side tool that reads `.state-witness.previous` in read-only mode
and walks the records via their 137-byte fixed width. Do not modify the
file in place; copy it to a working directory and operate on the copy.

If a future build supports replaying the previous journal into a fresh
state image, the on-disk file under `.state-witness.previous` is the input
the replay tool expects. The replay tool must verify the per-record
`chain_hash` link from the genesis header forward, and the segment header
signature over the `header_commitment`, before accepting any record.

## Verification

After the signer restarts, confirm the post-compaction chain is healthy:

- The signer starts cleanly without a rotation or anchor error.
- The new `.state-witness` journal exists and its first 16 bytes are
  `TBTCWITNESSv3\0\0\0`:

      head -c 16 .state-witness | od -An -tx1
      # 54 42 54 43 57 49 54 4e 45 53 53 76 33 00 00 00

- `.state-witness.previous` still exists in the same directory and is
  byte-for-byte unchanged from before the restart.
- The startup log includes the v3 store fingerprint derived from the
  existing `.store-id`. The fingerprint MUST match the v3 transcript
  computation over the unchanged `.store-id` bytes (the same transcript
  the v2-to-v3 runbook documents). A post-compaction restart against a
  different `.store-id` would change the fingerprint, which is the wrong
  outcome.
- The first committed record on the new chain is at generation 1 with a
  PREPARE and COMMIT pair, anchored on the new genesis header that the
  compaction record committed to.

If verification fails, the compaction is incomplete. Do not bring the
signer into a threshold set until the failure is diagnosed; see
*Rollback* below.

## Rollback

Compaction is local and additive. Rolling back is straightforward:

- Stop the signer process.
- Delete the new `.state-witness` (it contains only a fresh genesis +
  zero or a handful of records, none of which have anchored external
  commitments because the unanchored topology has no signed anchor
  checkpoint).
- Rename `.state-witness.previous` back to `.state-witness`:

      mv .state-witness.previous .state-witness

- Restart the signer. The pre-compaction chain resumes from its last
  record.

If the previous journal itself is corrupt, the only recovery is from a
prior snapshot of the store directory that contains an intact pre-rotation
journal. The compaction path does not modify the previous journal's bytes,
so any snapshot taken before the compaction (or a snapshot taken of the
directory as it stands after the compaction) is sufficient.

## Network coordination

The anti-rollback chain is local to each signer; the on-chain threshold
set does not enforce a coordinated compaction. However:

- **Compaction is per-signer.** A compaction on one signer does not
  trigger a compaction on its peers. Each signer in a threshold set
  compacts independently when its own record ceiling is reached, and the
  compacted-on-this-side / not-compacted-on-that-side state is normal
  and safe.
- **A compacted signer still produces chain-hash-linked records against
  its new genesis header.** Other signers do not need to know which
  generation another signer is on; the cross-signer protocol surface
  is unchanged.
- **Anchor signers are unaffected.** If the signer is configured to
  anchor, the existing segment-rotation path takes precedence over
  compaction, and this runbook's automatic-compaction behavior does
  not apply. Operators of an anchored signer who see
  `.state-witness.previous` should check the anchor wiring.

## Security model / limitations

Compaction produces a local record; it is not a signed commitment. An
attacker with same-uid access to the store directory who can rewrite
`.state-witness.previous` can rewrite the pre-compaction chain without
detection, and can also rewrite the post-compaction chain up to the next
compaction or anchor. The chain is tamper-evident against an
independently-observed prior head (e.g. an external anchor checkpoint or
a snapshot taken before the compaction), not tamper-RESISTANT by itself.

Operators who need a stronger guarantee than the local chain must ensure
a signed anchor checkpoint is configured, so the rotation path takes
precedence over compaction; see *When this activates* above.

## References

- The compaction plan and the prior-fingerprint limitation are tracked in
  `pkg/tbtc/signer/docs/FOLLOWUP.md` under P0 #4 (Compaction implementation
  for witness journal).
- The v3 record layout, segment header layout, and the per-record
  `chain_hash` domain are documented in
  `signer-store-v2-to-v3-migration-runbook.md` and in
  `pkg/tbtc/signer/src/engine/store.rs`
  (`TBTC_SIGNER_STATE_WITNESS_MAGIC`,
  `TBTC_SIGNER_STATE_WITNESS_RECORD_CHAIN_DOMAIN`,
  `TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_VERSION`).
- The store's file-locking semantics rely on POSIX `flock()` and may be
  weak or absent on shared or network filesystems (NFS, some container
  overlay filesystems); see the v2-to-v3 runbook's *Filesystem
  dependency* section for the operator-facing guidance.
