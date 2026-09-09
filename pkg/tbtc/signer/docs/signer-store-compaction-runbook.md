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
header is rooted in it. **The previous journal (`.state-witness.previous`)
is unlinked immediately after the rename pair completes — it is NOT retained
on disk. No forensic recovery of the pre-compaction journal is possible
without an external operator-taken directory snapshot taken before compaction.**

## When this activates

The compaction path activates automatically when the unanchored record
ceiling is reached AND no signed anchor checkpoint is configured. The
ceiling is governed by `TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS` (the
`witness_max_records` setting, always present). The separate knob
`state_witness_rotation_threshold` (env `TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS`)
is an anchor-only setting that **must be unset** for compaction to trigger at
all — it is not a control over compaction timing.

**Important:** In this build, the checkpoint-delivery FFI exports
(`frost_tbtc_acknowledge_state_witness_checkpoint`,
`frost_tbtc_recover_state_witness_checkpoint`) have been removed — no host
can currently deliver a signed checkpoint. Anchored topology configuration
remains settable via env/config, but the signed-checkpoint rotation path is
unreachable. Local compaction is therefore the only reachable journal-lifecycle
path in this build. The "preferred path" framing should be revisited once
checkpoint symbols are re-exposed.

If a signer is configured to anchor, the operator should not see this
runbook's behavior on a healthy node. If a compacted `.state-witness.previous`
appears on an anchored signer, the anchor wiring should be inspected before
the next state write.

## Pre-flight

1. Locate the durable store directory (the directory containing the
   `.state-witness` journal for the affected signer).
2. Confirm `.store-id` exists in the same directory and is exactly 32 bytes
   long. Compaction does NOT change the store fingerprint; the new genesis
   header chains to the same `.store-id` that the pre-compaction chain
   anchored against.
3. Optionally, list the journal slot to confirm compaction state:

       ls -la .state-witness .state-witness.previous 2>/dev/null

   **Note:** A transient `ENOENT` on `.state-witness` is possible during the
   nanosecond-to-microsecond window between the two rename operations of an
   in-progress compaction. If the signer process is confirmed still running,
   retry the `ls` once — this is not evidence of corruption.

   After compaction completes, `.state-witness.previous` is immediately
   unlinked and will not appear.
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

2. **Verify the live journal's first 16 bytes are the v3 magic.** The new
   `.state-witness` must begin with `TBTCWITNESSv3\0\0\0`:

       head -c 16 .state-witness | od -An -tx1
       # 54 42 54 43 57 49 54 4e 45 53 53 76 33 00 00 00

   A non-v3 magic on a freshly-compacted live journal indicates that the
   compaction path did not run as expected; halt and investigate.

3. **Restart the signer with the unchanged ABI.** The new build will open
   the fresh `.state-witness` and start a new chain at generation 1,
   preserving the store fingerprint. State writes resume against the new
   chain. `.store-id` and any state image are preserved.

## Inspection and recovery

**The previous journal (`.state-witness.previous`) is unlinked immediately
after compaction completes. It is NOT retained on disk and cannot be
inspected or recovered after the fact. Operators who need forensic recovery
capability MUST take a directory snapshot BEFORE triggering compaction
(i.e., before the signer reaches the `witness_max_records` ceiling).**

There is no rollback procedure for compaction. Once `compact_witness_journal_local`
returns successfully, the pre-compaction journal is gone. The only recovery
is from an external directory snapshot taken prior to compaction.

### Verifying a snapshotted copy of `.state-witness.previous`

If an operator preserved a copy of `.state-witness.previous` in a directory
snapshot taken before compaction, its magic, length, and trailing chain hash
can be checked with shell commands only:

1. Magic check — the previous journal begins with either the v3 signed
   segment magic (`TBTCWITNESSSEG1\0`, if it has ever rotated/compacted
   before) or the plain v3 magic (`TBTCWITNESSv3\0\0\0`, if it is a
   never-rotated genesis journal):

       head -c 16 .state-witness.previous | od -An -tx1

2. Length check — **branch on which magic matched above** before applying
   the modulo-137 check, since the header length differs:

       prev_len=$(stat -c %s .state-witness.previous)
       magic=$(head -c 16 .state-witness.previous)
       if [ "$magic" = "$(printf 'TBTCWITNESSSEG1\0')" ]; then
         header=472  # signed segment header
       else
         header=48   # plain magic header (16-byte magic + 32-byte store-id);
                     # this is the case for a never-rotated genesis journal,
                     # exactly the journal a FIRST compaction renames aside
       fi
       body=$((prev_len - header))
       if [ $((body % 137)) -ne 0 ]; then
         echo "previous journal length is not header + N*137: corrupt"
       fi

   Applying the 472-byte-header formula unconditionally reports a healthy
   first-compaction file (48-byte header) as corrupt.

3. Trailing chain hash check — the last 32 bytes of the previous journal
   are the chain hash of its final record:

       tail -c 32 .state-witness.previous | od -An -tx1 -v

## Verification

After the signer restarts, confirm the post-compaction chain is healthy:

- The signer starts cleanly without a rotation or anchor error.
- The new `.state-witness` journal exists and its first 16 bytes are
  `TBTCWITNESSv3\0\0\0`:

      head -c 16 .state-witness | od -An -tx1
      # 54 42 54 43 57 49 54 4e 45 53 53 76 33 00 00 00

- The `.store-id` file is byte-for-byte unchanged from before compaction
  (compare against a pre-compaction snapshot if one was taken).
- The first committed record on the new chain is at generation 1 with a
  PREPARE and COMMIT pair, anchored on the new genesis header that the
  compaction record committed to.

If verification fails, the compaction is incomplete. Do not bring the
signer into a threshold set until the failure is diagnosed.

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
`.state-witness` (the live journal) can rewrite the post-compaction chain
up to the next compaction or anchor. The chain is tamper-evident against an
independently-observed prior head (e.g. an external anchor checkpoint or
a snapshot taken before the compaction), not tamper-RESISTANT by itself.

Operators who need a stronger guarantee than the local chain must ensure
a signed anchor checkpoint is configured, so the rotation path takes
precedence over compaction; see *When this activates* above.

## References

- The compaction implementation and the prior-fingerprint limitation are
  described above (*When this activates* and *Security model /
  limitations*).
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
