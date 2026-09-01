# Sign-Store Follow-up Work (deferred from PR #4198)

PR #4198 (`codex/signer-store-identity-abi`) implements the v2 store-identity
fix: binding state commitments to the stable `.store-id` only, atomic genesis
creation, and bounded O(1) journal verification.

This branch (`codex/signer-store-identity-abi-followup`) then landed D1 (drop
the ABI bump, land durability only), D2 (rename `durable` -> `descriptor_bound`),
the v1->v2 re-anchor gap (P0 #3), the `validate_state_image` 3x/persist
optimization, and hardening (`cfg(not(unix))` guards, `validate_entry_name`
rejecting `.` / `..`, dropping PID from `unique_temp_name`) in commit
`b976a46a4`, plus 16 new tests + 2 strengthened tests — not in a separate prior
PR as an earlier revision of this document implied.

The four items below were originally deferred to this branch. Status as of the
second multi-agent-review pass (`agent-docs/reviews/codex-signer-store-identity-followup/`):

## P0 — Critical

### Per-record hash chain for witness journal (P0 #2) — **DONE**
- **Landed in:** `90d99f487`, `pkg/tbtc/signer/src/engine/store.rs`
  (`encode_state_witness_record` / `apply_state_witness_record`).
- Every witness record now carries a 32-byte `chain_hash` field:
  `chain_hash[i+1] = sha256(domain || chain_hash[i] || record[i+1])`. Records
  grew from 105 to 137 bytes; the 472-byte segment header layout is unchanged
  (frozen cross-language contract, see `signed_segment_header_matches_frozen_472_byte_vector`
  in store.rs) and is pinned by
  `docs/signer-store-v2-to-v3-migration-runbook.md`.
- **Design decision (resolved):** chain continuity is intentionally
  **segment-scoped**. A new segment's chain seeds from its own signed
  `StateAnchorAcknowledgement`/genesis header, not from the retiring segment's
  `last_chain_hash` — cross-segment continuity is delegated entirely to the
  externally-signed checkpoint (or, for unanchored signers, to the new local
  compaction below), not to the per-record chain. This was evaluated against
  threading the prior segment's chain hash through the segment header and
  rejected as unnecessary complexity for the actual threat model; see the
  "Security model / limitations" section of
  `docs/signer-store-v2-to-v3-migration-runbook.md` for the residual-tamper
  analysis this implies (the chain is CT-log-like — tamper-evident against a
  previously-observed head, not a tamper-resistance guarantee absent an
  external anchor).
- **Test coverage:** `mid_journal_chain_hash_tamper`,
  `wrong_domain_separator`, `retired_v2_journals_are_recognized_by_magic_alone`,
  `rotated_segment_chain_hash_is_segment_scoped_to_header_commitment`, and a
  frozen cross-language vector (`record_chain_hash_matches_frozen_go_v3_vector`)
  in `store.rs`.

### Compaction implementation for witness journal (P0 #4) — **DONE**
- **Landed in this pass:** `pkg/tbtc/signer/src/engine/store.rs`
  (`StateFileLock::compact_witness_journal_local`,
  `recover_state_witness_compaction`, `synthetic_compaction_acknowledgement`).
- **Original issue:** the rotation-threshold ceiling
  (`ensure_witness_record_capacity`) hard-failed every write once
  `witness_max_records` was reached, and the only rotation path
  (`rotate_state_witness_segment*`) was gated on an externally-signed
  `StateAnchorAcknowledgement`. For an **unanchored** signer (no anchor
  service configured — a fully supported production topology), that signed
  checkpoint never arrives, so `witness_rotation_threshold` stays permanently
  `None` and the ceiling was a permanent, unrecoverable write-lockout.
- **What shipped:** `compact_witness_journal_local` runs automatically inside
  `ensure_witness_record_capacity` when the ceiling is reached and no anchor
  is configured. It appends a self-referential compaction record committing
  to a new genesis header (`synthetic_compaction_acknowledgement` — a
  zero-signature in-band marker `parse_state_witness_segment_header`
  recognizes and treats as the un-signed local-compaction case, distinct from
  an externally-signed checkpoint), atomically renames the current
  `.state-witness` to `.state-witness.previous` (retained indefinitely, never
  auto-deleted — see the recovery runbook below), and creates a fresh
  `.state-witness` seeded from the compaction record so the chain stays
  provable across the compaction boundary. Crash recovery
  (`recover_state_witness_compaction`) follows the same create-then-verify-then-rename
  pattern as `rotate_state_witness_segment_inner` and runs unconditionally at
  store-open, mirroring how rotation recovery is wired in.
  The old ceiling error message, which pointed operators at a checkpoint ABI
  with zero FFI exports in this build, is gone — local compaction now
  succeeds instead of failing in the unanchored case; the anchored case is
  unaffected and continues to use the signed-checkpoint rotation path.
- **Operator runbook:** `docs/signer-store-compaction-runbook.md` (new) —
  covers when local compaction activates, how to inspect/verify a retained
  `.state-witness.previous`, and the recovery procedure.

## P1 — High

### Trust journal rotation/compaction — **DONE (partial)**
- **Landed in this pass:** `pkg/tbtc/signer/src/engine/anchor_trust.rs`
  (`STATE_ANCHOR_TRUST_MAX_RECORDS = 1_024`).
- Enforced alongside the existing `STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH`
  (256 MB) byte cap in `parse_state_anchor_trust_journal`. Trust-journal
  records vary in size (unlike the witness journal's fixed-size records), so
  this is an upper-bound check derived from byte length rather than an exact
  per-record count — it is a loose bound for realistic journal usage and only
  fires when many small records approach the byte cap. Test:
  `trust_journal_exceeding_records_ceiling_is_rejected_under_byte_cap`.
- No trust-symbol FFI exports currently exist (`frost_tbtc_*anchor*`/
  `*checkpoint*`/`*trust*` are all off the wire contract pending a future PR
  that re-exposes them with Go-side coordination), so the ceiling is
  defense-in-depth against a future re-exposure rather than a reachable
  concern in the current build.

### `witness_history` unbounded growth — **already mitigated, not a standalone item**
- The pre-existing segment-rotation mechanism (`rotate_state_witness_segment_inner`,
  landed before this followup in `7bfa0e535`) already resets `witness_history`
  to a single entry (`[tip]`) on every acknowledged rotation. The new local
  compaction (P0 #4 above) does the same for the unanchored case. The
  "1M persists / 122 MB resident" scenario this item originally described only
  applies if neither rotation nor compaction ever runs — the same root cause
  as the P0 #4 gap, not an independent defect. No separate code change is
  needed beyond the P0 #4 fix above.

## Implementation notes

Findings tracked above come from
`agent-docs/reviews/codex-signer-store-identity-followup/findings.json`
(the second multi-agent-review pass on this branch). All four originally
deferred items are now resolved (three by code changes, one determined to be
already mitigated). Remaining lower-severity findings from that review
(simplicity, documentation, and test-coverage items) were fixed alongside
these four; see the review's `findings.json` for the full list.

## References

- Multi-agent-review (PR #4198, first pass): `agent-docs/reviews/pr-4198/findings.json`
- Multi-agent-review (this followup, second pass): `agent-docs/reviews/codex-signer-store-identity-followup/findings.json`
- Compaction runbook: `docs/signer-store-compaction-runbook.md`
- v2-to-v3 migration runbook (hash chain + security model + filesystem dependency): `docs/signer-store-v2-to-v3-migration-runbook.md`
- Implementation PR: https://github.com/threshold-network/keep-core/pull/4198
