# Sign-Store Follow-up Work (deferred from PR #4198)

PR #4198 (`codex/signer-store-identity-abi`) implements D1 (drop ABI bump, land
durability only), D2 (rename `durable` -> `descriptor_bound`), the v1->v2 re-anchor
gap (P0 #3), the `validate_state_image` 3x/persist optimization, hardening
(`cfg(not(unix))` guards, `validate_entry_name` rejecting `.` / `..`, dropping
PID from `unique_temp_name`), and 16 new tests + 2 strengthened tests.

The following items are **deferred** to this follow-up branch (per user
directive: "bigger changes go to a follow-up PR"):

## P0 — Critical

### Per-record hash chain for witness journal (P0 #2)
- **Where:** `pkg/tbtc/signer/src/engine/store.rs:3622-3763` (append_witness_record
  + extend_witness_prefix), `store.rs:1909-1992` (verify_state_witness_journal +
  witness_anchor_matches), `store.rs:3269-3503` (rotate_state_witness_segment_inner).
- **Issue:** Journal anchors verification on the trailing record only; no
  per-record hash chain. A same-uid writer who can `pwrite` into the middle of
  `.state-witness` and `utimensat(2)` to restore original size+mtime+ctime is not
  detected until next fresh open (months away for a custody signer).
- **Plan:** Implement `chain_hash[i+1] = sha256(domain || chain_hash[i] ||
  record[i+1])`. Segment header's `header_commitment` becomes the final chain
  root. Alternative: switch to a merkle log of records inside the journal
  (CT/Trillian style) which also gives the per-record proofs the
  `state_witness_proof` ABI requires but the implementation does not produce.

### Compaction implementation for witness journal (P0 #4)
- **Where:** `pkg/tbtc/signer/src/engine/store.rs:3269-3503`,
  `anchor.rs:228-237`, `store.rs:3683-3689`.
- **Issue:** Rotation-threshold error references a checkpoint ABI that doesn't
  exist. Default `rotation_threshold_records` test values are 8/2; production
  traffic could park the journal in minutes.
- **Plan:** Implement minimum-viable compaction: append a "compaction record"
  committing to a new genesis header, rename `.state-witness` ->
  `.state-witness.previous`, create a new `.state-witness` containing only the
  new header + zero records. The "compaction record" is itself a witness
  record so the chain is provable.

## P1 — High

### Trust journal rotation/compaction
- **Where:** `pkg/tbtc/signer/src/engine/anchor_trust.rs:103-130`
  (`STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH = 256 MB`).
- **Issue:** 256 MB ~= 2.3M records at 116 B each. Key compromise event or
  config-pinned transition retry can flood.
- **Plan:** Add `state_anchor_trust_max_records` ceiling analogous to
  `state_witness_rotation_threshold`.

### `witness_history` unbounded growth
- **Where:** `pkg/tbtc/signer/src/engine/store.rs:476-486` (WitnessJournalPrefix)
  + `store.rs:478-580`.
- **Issue:** ~104 B/persist growth; at 1M persists with 1-record retention
  window, ~122 MB resident heap.
- **Plan:** After each rotation, prune `witness_history` to current segment
  base + required retention window. Or cap at fixed generations / ring buffer
  of anchored checkpoints.

## Implementation notes

The follow-up branch is currently at the original branch HEAD
(`08b6d6f40`). Apply each item on top of that base. Each item should:

1. Land as a separate commit with a focused message naming the multi-agent-review
   finding it addresses.
2. Pass `cargo clippy --all-targets --all-features -- -D warnings`.
3. Pass `cargo test --lib -- --test-threads=1`.
4. Add tests that would fail on regression.

## References

- Multi-agent-review source: `agent-docs/reviews/pr-4198/findings.json`
- Multi-agent-review report: `agent-docs/reviews/pr-4198/report.md`
- Gap inventory (decisions): `agent-docs/gap-inventory.md`
- Implementation PR: https://github.com/threshold-network/keep-core/pull/4198