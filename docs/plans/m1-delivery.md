# M1 Delivery Plan — UTXO Reservations

Versioned, durable counterpart to the working tracker `agent-docs/m1/STATUS.md`
(which is gitignored — see that file for full handoff log). This file is the
single canonical progress document for milestone 1 (UTXO reservations). Update
this when STATUS.md gets a major change; STATUS.md updates more freely as the
working scratchpad.

**Source of truth for the PR decomposition and order:** `docs/spec/reservations/pr-strategy.md` §4.1 + §9.
**Build order** (corrected 2026-08-24): **A → C → D → E → B → F → G** (then keep-core PR #H post-epic).

## Current status — 2026-09-04

**tbtc-v2 (`reservations-upgrade`, tip `09b3d2c7d`, 2026-09-04T10:58:46Z) — A–G build order fully merged:**

| # | PR | tbtc-v2 branch | Status | Notes |
|---|----|---------------|--------|-------|
| 1 | A `storage-layout` | `m1/storage-layout` | **MERGED** [tbtc-v2#1106](https://github.com/threshold-network/tbtc-v2/pull/1106) (2026-09-01) | — |
| 2 | C `acceptance-core` | `m1/acceptance-core` | **MERGED** [tbtc-v2#1107](https://github.com/threshold-network/tbtc-v2/pull/1107) (2026-09-01) | — |
| 3 | D `reanchor-core` | `m1/reanchor-core` | **MERGED** [tbtc-v2#1108](https://github.com/threshold-network/tbtc-v2/pull/1108) (2026-09-01) | — |
| 4 | E `timeout-and-stranding` | `m1/timeout-and-stranding` | **MERGED** [tbtc-v2#1109](https://github.com/threshold-network/tbtc-v2/pull/1109) (2026-09-02) | Review-fix [#1119](https://github.com/threshold-network/tbtc-v2/pull/1119) (11 confirmed findings) merged 2026-09-03, folded in |
| 5 | B `router-minimal` | `m1/router-minimal` | **MERGED** [tbtc-v2#1110](https://github.com/threshold-network/tbtc-v2/pull/1110) (2026-09-03) | Review-fix [#1118](https://github.com/threshold-network/tbtc-v2/pull/1118) (2 confirmed findings) merged 2026-09-02, folded in |
| 6 | F `vault-pause-flags` | `m1/vault-pause-flags` | **MERGED** [tbtc-v2#1111](https://github.com/threshold-network/tbtc-v2/pull/1111) (2026-09-03T15:35:06Z) | Prior conflict blocker (9-file stale-snapshot vs B/#1118) resolved and merged |
| 7 | G `bridge-integration-seams` | `m1/bridge-integration-seams` | **MERGED** [tbtc-v2#1112](https://github.com/threshold-network/tbtc-v2/pull/1112) (2026-09-03T17:09:41Z) | Prior conflict blocker (10-file, same root cause as F) resolved and merged |

**Also landed on `reservations-upgrade` since G, spec'd as out-of-stack items:**

| PR | Branch | Status | Notes |
|----|--------|--------|-------|
| [#1120](https://github.com/threshold-network/tbtc-v2/pull/1120) `m1/reanchor-dissolution-gate-fix` | fix(bridge): remove dissolution-eligibility gate from `requestReservationReanchor` | **MERGED** (2026-09-03T11:25:12Z) | Spec'd as PR I in `pr-strategy.md`; independent of A–G |
| 5 direct commits by the maintainer (no PR) | `reservations-upgrade` directly | **LANDED** 2026-09-04T08:38–10:58 | `test: cover reservations end-to-end and consolidate harnesses`; `style: prettier-format reservation contracts and deployment docs`; `fix(reservations): router occupancy parity, closeReservation completeness, snapshot pin, drop dead strand overload`; `fix(reservations): adapt closing-period gate to packed counters; restore re-anchor amount floor`; `style(reservations): plain-text comment in re-anchor request gate`. Pushed directly, not through PR review — flagged here for visibility, not a queue item. |

**Open, not yet merged (tbtc-v2):**

| PR | Branch → base | Status | Notes |
|----|---------------|--------|-------|
| [#1121](https://github.com/threshold-network/tbtc-v2/pull/1121) `port/reserved-redemption-veto` → `reservations-upgrade` | feat(bridge): port reserved-redemption and veto surface from the settlement branch | OPEN, `mergeable: CONFLICTING`/`mergeState: DIRTY` (flipped from `UNKNOWN` within minutes of opening, once GitHub finished recomputing) | Touches `Reservation.sol`, `ReservationRouter.sol`, `ReservationProofs.sol`, `ReservationVault.sol` — the same files the 5 direct maintainer commits (row above) modified on `reservations-upgrade` after this PR's branch point. New work, not part of the original A–H decomposition; not yet reviewed. |
| [#1116](https://github.com/threshold-network/tbtc-v2/pull/1116) `reservations-upgrade` → `dev` | chore(bridge): milestone-1 UTXO reservations stack tracker | OPEN, `mergeable: MERGEABLE`/`mergeState: UNSTABLE` | Diff now `+20394/-45` across 50 files (reflects full A–G payload). UNSTABLE = 5 checks still `IN_PROGRESS` (`contracts-build-and-test`, `contracts-format`, `contracts-slither`, `contracts-deployment-dry-run`, docs preview), no failures seen. `dev` still ahead by commits `reservations-upgrade` lacks — reconcile before this can go green and merge. |

**keep-core (`reservations-epic`, tip `356d35bae`, 2026-09-03T16:42:49Z) — H plus full downstream chain merged:**

| PR | keep-core branch | Status | Notes |
|----|------------------|--------|-------|
| [#4274](https://github.com/threshold-network/keep-core/pull/4274) `m1/keep-core-client` (H) | **MERGED** into `reservations-epic` (2026-09-03T05:45:08Z) | — |
| [#4276](https://github.com/threshold-network/keep-core/pull/4276) `m1/reservation-readiness-fixes` | **MERGED** (2026-09-03T05:49:28Z) | — |
| [#4277](https://github.com/threshold-network/keep-core/pull/4277) `m1/reservation-protobuf-marshaling` | **MERGED** (2026-09-03T05:49:37Z) | — |
| [#4283](https://github.com/threshold-network/keep-core/pull/4283) `m1/reservation-review-fixes` | fix(tbtc): reservation review remediation (37 findings from multi-agent review of #4282) | **MERGED** (2026-09-03T15:04:40Z) | Not previously tracked in this doc — found live this session |
| [#4278](https://github.com/threshold-network/keep-core/pull/4278) `m1/reservation-coordination-checklist` | **MERGED** (2026-09-03T13:02:32Z) | Prior rebase/CI blocker resolved and merged |
| [#4279](https://github.com/threshold-network/keep-core/pull/4279) `m1/reservation-multisigner-integration-test` | **MERGED** (2026-09-03T12:34:15Z) | Prior conflict blocker resolved and merged |
| [#4280](https://github.com/threshold-network/keep-core/pull/4280) `m1/reservation-test-coverage-backfill` | **MERGED** (2026-09-03T16:42:50Z) | M2 test-coverage backfill (7/8 items); current epic tip |

**Open, not yet merged (keep-core):**

| PR | Branch → base | Status | Notes |
|----|---------------|--------|-------|
| [#4238](https://github.com/threshold-network/keep-core/pull/4238) `feat/utxo-reservation-wallet-support` → `reservations-epic` | OPEN, `mergeable: CONFLICTING`/`mergeState: DIRTY` | 22 commits, `+3779/-37` across 15 files. Base ref still pinned to the pre-H epic base (`a7ac8989`) — never rebased across 6 merged PRs since. Parallel branch, not part of the H chain. **Reconcile-or-supersede decision is a human call** (blocked, see todo). |
| [#4282](https://github.com/threshold-network/keep-core/pull/4282) `reservations-epic` → `dev` | OPEN, `mergeable: CONFLICTING`/`mergeState: DIRTY` | Diff now `+34034/-307` across 110 files (full epic payload). Real conflicts against `dev` this time (not just a stale label) — touches `.github/workflows/client.yml`, `cmd/start.go`, `config/config_test.go`, `pkg/chain/ethereum/tbtc.go`, and generated `pkg/chain/ethereum/tbtc/gen/**` files, all likely touched independently on `dev`. `reservations-epic` was 63 ahead / 68 behind `dev` as of last count; needs an explicit merge/rebase decision, not a trivial fast-forward. |

**Incidentally on the epic chain, not reservation content (keep-core):** [#4284](https://github.com/threshold-network/keep-core/pull/4284) `fix(net/local): bound release-boundary settle drain in TestReleaseBroadcastChannel` — base `reservations-epic` (not `main`), 1-file/+3-1, **MERGED** 2026-09-03T15:37:48Z (before #4280, does not change current tip). Flaky-test fix that happened to branch off the epic's working tip; unrelated to reservations logic.

**Fully unrelated keep-core PRs seen in the same query window (base `main`, no action needed):** [#4285](https://github.com/threshold-network/keep-core/pull/4285)/[#4286](https://github.com/threshold-network/keep-core/pull/4286) (dependabot version bumps, OPEN); [#4199](https://github.com/threshold-network/keep-core/pull/4199)/[#4226](https://github.com/threshold-network/keep-core/pull/4226) (frost-schnorr signer work, separate feature).

**Superseded pre-M1 chain (tbtc-v2, not part of the active A–H queue):** the original monolithic reservation-feature stack, superseded by the milestone-1 decomposition per `docs/spec/reservations/pr-strategy.md` §4.1. Kept as frozen-spec reference only.

| PR | Branch → base | Status | Title |
|----|---------------|--------|-------|
| [#1088](https://github.com/threshold-network/tbtc-v2/pull/1088) | `feat/utxo-reservation-core` → `reservations-upgrade` | OPEN, parked | draft: UTXO reservations — segregated custody with in-kind redemption |
| [#1090](https://github.com/threshold-network/tbtc-v2/pull/1090) | `feat/utxo-reservation-router` → `feat/utxo-reservation-core` | OPEN, parked | feat(bridge): delegatecall reservation router (EIP-170) + RFC 13 |
| [#1091](https://github.com/threshold-network/tbtc-v2/pull/1091) | `feat/utxo-reservation-settlement` → `feat/utxo-reservation-router` | OPEN, parked | feat(bridge): two-phase authorize-then-prove reservation settlement |
| [#1092](https://github.com/threshold-network/tbtc-v2/pull/1092) | `feat/utxo-reservation-renewal` → `feat/utxo-reservation-settlement` | OPEN, parked | feat(bridge): bounded permissionless renewal and strict expiry semantics |
| [#1093](https://github.com/threshold-network/tbtc-v2/pull/1093) | `feat/utxo-reservation-backing` → `feat/utxo-reservation-renewal` | OPEN, parked | feat(bridge): claim-equals-anchor backing model with financed in-kind fees |
| [#1094](https://github.com/threshold-network/tbtc-v2/pull/1094) | `feat/utxo-reservation-guards` → `feat/utxo-reservation-backing` | OPEN, parked | feat(bridge): reveal-side wallet binding, pending-deposit guard, stranding and monitoring |
| [#1095](https://github.com/threshold-network/tbtc-v2/pull/1095) | `docs/utxo-reservation-release` → `feat/utxo-reservation-guards` | OPEN, parked | docs+test: reservation release completeness (M-09) |
| [#1096](https://github.com/threshold-network/tbtc-v2/pull/1096) | `feat/utxo-reservation-partial-redemption` → `docs/utxo-reservation-release` | OPEN, parked | feat(reservation): partial reserved redemption (1-in-2-out split) |
| [#1102](https://github.com/threshold-network/tbtc-v2/pull/1102) | `fix/utxo-reservation-review-followups` → `feat/utxo-reservation-core` | **MERGED** | fix(reservation): address multi-agent review findings on #1088 |
| [#1104](https://github.com/threshold-network/tbtc-v2/pull/1104) | `test/reanchor-fee-exposure` → `feat/utxo-reservation-backing` | **MERGED** | test(reservation): characterize the accepted cumulative re-anchor fee exposure |

**Worktrees present** (`git worktree list`): `/tmp/m1-{a,b,c,d,e,f,g,h}` exist. `/tmp/m1-h-{a,r,w}` were transient parallel-builder worktrees for PR H's acceptance/re-anchor/watchers branches, merged into `m1-h` and safe to prune. `/tmp/src-{1091,1093,1094,1096,1102}` are read-only reference copies. (`m1-g2` worktree and `m1/bridge-integration-seams-g2` branch retired 2026-08-26 — fast-forward-merged into `m1/bridge-integration-seams`; single branch now carries all of PR #G.) All F/G rebase-blocker worktrees are now moot since F and G merged; safe to prune `/tmp/m1-f` and `/tmp/m1-g` next session if untouched.

## Resolved this session (2026-08-25)

1. **PR #B "runtime BLOCKED" was self-inflicted, not a Hardhat bug.** `FORKING_URL` was set in the shell; `hardhat.config.ts:104-108` enables forking on that env var, and the fork provider path is what throws on `config.chains`. Unset with `env -u FORKING_URL` before running m1 stub-only tests. Going forward: a `RUNBOOK.md` captures the gotcha so it doesn't get re-discovered.
2. **Bootstrap-ordering test fails for a real reason, not flakiness.** Traced through `Bridge.ReservationCaps.test.ts` fixture numbers (`slotCapacity = 4 * 1_000_000 = 4_000_000`): the test sets an oversized `reservationMaxTotalAmount` while caps are zero-disabled (succeeds, as intended), then calls `updateReservationCaps` with real caps — and that call reverts, one step earlier than the test expects, because the two-sided invariant correctly rejects configuring caps that don't dominate the already-oversized stored total. The check works as designed; the test scenario is itself the hazard. **Fixed:** bootstrap-ordering describe block now has two tests — hazard and safe-order.
3. **PR #B test fix landed and verified.** Tip `b21429dd` → `3156ed50`. Both new bootstrap-ordering tests pass; full suite (`Bridge.ReservationCaps.test.ts` + `Bridge.StorageLayout.test.ts`) re-verified by the manager 9/9 green with `FORKING_URL` unset. Storage layout unchanged; `assertStorageUpgradeSafe` still passing against TIP-109. Yarn-format failures characterized: 50 pre-existing Solhint warnings distributed across `contracts/test/Mock*.sol` and unrelated files, plus `gasReporterOutput.json` prettier issue. None on PR #B or PR #C files. Right standard for m1 PRs is "no new failures on PR files," not "clean exit code."
4. **PR #C duplicate-guard policy pick landed (option b).** Tip `f1ede944` → `3419e475`. Net `-8/+2` on `Reservation.sol:391-398`: replaced 8-line verbose NOTE block with 2-line `// keep: see :424 for rationale ...` directive. Both require calls at `:401` and `:424` preserved; rationale comment at `:417-421` unchanged. Verbatim byte-fidelity to `#1094` maintained. See `manager-to-implementor-2026-08-25-pr-b-c-accepted.md` §2.
5. **PR #D reanchor-fee fix confirmed landed.** `reservation.cumulativeReanchorFee += minerFee;` at `ReservationProofs.sol:692`.
6. **PR #F's "Hardhat/Ethers version mismatch" was the same `FORKING_URL` self-inflicted cause as PR #B's item 1, not a real environment defect.** The manager's original test attempt trusted the implementor's "20/20 passing" claim without re-running it because of a misdiagnosed failure. Re-ran with `env -u FORKING_URL`: 19 passing in `ReservationVault.test.ts` + 1 passing in `Bridge.StorageLayout.test.ts` = 20/20, exactly as claimed. See `manager-to-implementor-2026-08-25-pr-f-accepted.md` (corrected section).
7. **PR #G-part-2 built by manager-orchestrated subagents, not the implementor agent (retired 2026-08-25).** Governance/Bridge/Wallets/WalletProposalValidator/Deposit seams landed on `m1/bridge-integration-seams-g2` @ `9362cda1`. Two subagent attempts failed before the fix landed: (1) a naive port of the four missing Bridge wrapper functions compiled 26,529 B, 1,953 B over EIP-170 even at `runs:200`; (2) an optimizer-only `runs:200` override (landed separately as `45cf8196`, kept) could not recover the missing symbols on its own. Root cause: `ReservationRouter.sol:66-69` invariant 2 forbids Bridge from declaring `updateReservationParameters`/`updateReservationCaps`/`reservationParameters` because the Router already does. Fix: `fallback() external payable` on Bridge delegatecalls to `self.reservationRouter`; `BridgeGovernance.sol`/`WalletProposalValidator.sol` call sites cast through `IReservationBridge(address(bridge))`. Final size 22,870 B. `m1/bridge-integration-seams-g2` was later fast-forward-merged back into `m1/bridge-integration-seams` and retired (2026-08-26). See row 7 above for full current-state detail.

## Resolved this session (2026-08-26)

1. **Two stale-hash corrections.** G-part-1's tip was recorded as `da9c0fda` in this doc, `STATUS.md`, and prior implementor memos — the actual branch ref was always `89528a45`; `da9c0fda` never matched anything reachable. Corrected here and in `STATUS.md`; prior implementor/manager memos left as historical record, not rewritten.
2. **PR #G branch consolidated.** `m1/bridge-integration-seams-g2` fast-forward-merged into `m1/bridge-integration-seams` (identical commit `9362cda1`, g2 was linearly stacked on g's tip) and deleted. One branch now carries all of PR #G; row 7/7b collapsed into a single row 7.
3. **`Bridge.Deposit.test.ts`'s `ReimbursementPool` fixture gap investigated and closed as out-of-scope.** Confirmed pre-existing and repo-wide by reproducing the identical failure on `fix/utxo-reservation-review-followups` (current `main`-tracking tip) with PR #G's diff stashed out entirely. Root cause: `deploy/00_resolve_reimbursement_pool.ts` throws because `hardhat.config.ts:198-240`'s `external.deployments` never configures a `ReimbursementPool` source for the `hardhat` network. Not a blocker for PR #G — none of the three reservation regression suites go through `deployments.fixture()`. No m1 action item.
4. **Epic-merge dry run: all seven m1 branches (A, C, D, E, B, F, G, in the corrected `pr-strategy.md` §9 build order) merge cleanly with zero manual conflict resolution**, run on a scratch throwaway branch cut from `milestone/utxo-reservation-m1` and deleted afterward (no state left behind). Only A and C were fast-forwards (they sit directly on each other); D, E, B, F, G all three-way auto-merged clean because their file-level changes are disjoint or non-overlapping regions of `Reservation.sol`/`Bridge.sol`/vault files. Final `hardhat compile` on the merged tree passes; Bridge runtime bytecode 22,870 B, 1,706 B under the 24,576 B EIP-170 limit — same figure as PR #G's own branch, confirming the merge doesn't add any further bytecode growth from the other six PRs. This substantially de-risks the eventual human-driven epic integration (step map row 12): the actual merge should be at least as clean as this dry run, modulo whatever changes come out of GitHub PR review.
5. **PR H (`m1/keep-core-client`) research complete; build brief written and build started.** Full research (branch surface already landed by PR #4238 vs. net-new PR H scope, exact Go files/interfaces/line estimates, parallelization plan, open questions, blockers) — see `agent-docs/m1/manager-to-implementor-2026-08-26-pr-h-build-brief.md`. Headline finding: PR #4238 landed only the proposal/marshaling/assembler layer (no executor, no submission methods, no chain bindings); PR H's net-new bottom-up estimate is ~2,300-2,500 production Go + ~2,300 test Go (vs. the previously published ~1,100-1,400/~1,200 estimate, revised up given the work is greenfield executor-plus-plumbing, not client rework). Real blocker found and worked around: PR H needs Go ABI bindings regenerated against the m1 tbtc-v2 surface, normally gated on a published `@keep-network/tbtc-v2` npm package (which requires PRs A-G to merge first, a human-gated step) — unblocked by generating bindings locally against `/tmp/m1-g`'s compiled Solidity artifacts instead (byte-identical source, since that's the actual surface that will ship). Branch `m1/keep-core-client` created on top of PR #4238 (`b4f63944`), worktree `/tmp/m1-h`. Bindings-regeneration subagent dispatched; build in progress.
6. **PR H (`m1/keep-core-client`) build completed.** Chain-interface writes/reads/events, acceptance
   + re-anchor executors, three watchers, and operator wiring all landed; tip `48985451d`; `go build
   ./...` clean; 552 tests passing across `pkg/tbtc`, `pkg/tbtcpg`, `pkg/maintainer/spv`,
   `pkg/clientinfo`. Full narrative: `agent-docs/m1/STATUS.md` row 11 and its "PR H build completed
   2026-08-26" session note (two real defects found and fixed mid-build: a `pkg/tbtcpg` test-double
   keying its reserved-deposits map on `*big.Int` pointer identity instead of value, and a stub
   funding transaction with zero outputs that the real acceptance-proposal assembler needs a valid
   locking script from).
7. **PR #G fallback swallowed the reservation router's revert reason.** `Bridge.sol`'s `fallback()`
   (added for `ReservationRouter` invariant 3, "no standalone authority") captured the router
   `delegatecall`'s `result` but never used it — every failure collapsed to the single string
   `"Reservation router delegatecall failed"`, discarding `onlyGovernance`/cap-validation/custom-error
   reasons and any specific `revertedWith` assertion m2 tests would need. No test in the current PR
   set exercises the fallback path with a `revertedWith` assertion (the reservation-cap tests call
   `updateReservationParameters` directly on the governance-parameters stub, not through the Bridge
   fallback), so nothing broke today — but it would have silently broken every such assertion once
   m2 test authors started writing them through the Bridge address. Fixed on `m1/bridge-integration-
   seams` (tip `9362cda1` → `cf457613`): failure path now re-reverts with the router's raw return
   data (`revert(add(result, 0x20), mload(result))`) instead of a canned string. Recompiled and
   re-ran all three PR #G regression suites (`Bridge.StorageLayout`, `ReservationVault`,
   `Bridge.ReservationCaps`) — 28/28 still passing; Bridge bytecode size unaffected. (A stray
   `solidity/node_modules` dev symlink got picked up by `git add -A` while committing this and was
   un-staged before commit — local machine artifact, not repo content.)
8. **PR #G and PR #H description files written.** `agent-docs/m1/pr-{G,H}-description.md` were
   missing (A-F had them; G and H did not, since both were built after the description-writing
   convention started with the earlier PRs). Written to the same standard as A-F: exact diffstat
   re-derived from `git diff --stat` against each PR's actual base (not the doc's recollection),
   full commit list, the two defects found and fixed during each build, and verification commands
   with results. All eight PRs now have a ready-to-paste GitHub PR body.
9. **Full branch/PR review against `pr-strategy.md` §4.1/§4.2 expectations.** All 8 branches
   independently re-verified (content, tip hash, git parentage, build/test results) against the
   spec doc and the claim docs. Results:
   - **Headline finding: zero automated behavioral coverage of the reservation
     lifecycle across all eight PRs combined.** PR C (acceptance), PR D (re-anchor)
     and PR E (timeout/stranding) ship no test files at all; PR B's own new
     `Bridge.ReservationCaps.test.ts` only exercises the governance cap/parameter
     setters, never `requestReservationAcceptance`/`submitReservationProof`/
     `requestReservationReanchor`/any `notifyReservation*` function. PR D's own
     description explicitly flagged this risk ("the characterization test that
     pins this exposure [`#1104`] ... must be carried forward when PR #B makes the
     surface reachable; if it is dropped, the accepted regression stops being
     executable and degrades to prose") — PR B has been built since 2026-08-25 and
     the carry-forward never happened, confirmed absent from every branch. The 28
     tests passing on the merged epic tree (storage-layout parity, cap/parameter
     governance, vault-level pause/fee mechanics) never drive a reservation through
     accept/prove/settle/re-anchor/timeout/stranding. Full detail and the two
     remediation options: `pr-strategy.md` §4.1's justification section.
   - **A, C, E, F, G otherwise clean match** against their own per-PR claims (tip
     hash, base, required
     functions present, forbidden functions absent where the row-141 correction applies,
     build/test results reproduced).
   - **PR H's claimed "552 tests passing" re-verified correct** (`go test ./pkg/tbtc/...
     ./pkg/tbtcpg/... ./pkg/maintainer/spv/... ./pkg/clientinfo/... -count=1` → "552 passed in 7
     packages", independently reproduced twice).
   - **`pr-{A,C,D,E}-description.md` had stale self-reported tip/base hashes** — each was written
     before a later fix landed on that same PR (or its cited predecessor) and never refreshed.
     Fixed: A's Head `78e6b607`→`e175092a`; C's Head `54124d8c`→`1f87f8d8`, Base(A)
     `78e6b607`→`e175092a`, Diff `+930`→`+932`; E's Head `aa91cd3d`→`2e63515f`, Base(D)
     `e2bdb3a5`→`08536cd4`. B, F, G, H's description files were already current.
   - **PR D's git fork point from PR C, and a retracted finding.** PR D forked from PR C at
     `f1ede944`, two commits before PR C's current tip `1f87f8d8` (the `3419e475`/`1f87f8d8`
     comment-annotation fix). This session initially flagged that as a hazard — opening PR D
     stacked on PR C's live branch would show a "spurious revert" of PR C's fix in the GitHub
     diff — and recorded it as an open decision. **Retracted: tested and refuted.** GitHub PR
     diffs use three-dot semantics (`base...head`, from the merge-base forward), not two-dot.
     `git diff --stat m1/acceptance-core...m1/reanchor-core` shows exactly PR D's own 144
     insertions / 0 deletions, with zero trace of PR C's fix region. The original claim used
     two-dot semantics (`base..head`), which isn't what GitHub renders, and manufactured an
     artifact that doesn't exist under the diff GitHub actually shows. No action needed; PR D
     can be opened stacked on PR C's live branch as originally planned. `pr-D-description.md`
     corrected to retract the note.
   - **Two PR-strategy.md size-table figures are confirmed stale, both undershoots (not code
     defects — content is correct, verified complete against spec):** PR B measures 860
     production Solidity lines (1,160 with tests) against the doc's own corrected ~690 estimate;
     PR H measures ~4,849 production Go lines (excluding generated ABI bindings and test code)
     against the doc's revised ~2,300–2,500 estimate — roughly double, the third estimate in a row
     for PR H's size (~1,100-1,400 → ~2,300-2,500 → ~4,849 actual). Corrected in `pr-strategy.md`
     §4.1/§4.2 (see that doc's own correction there).
   - **Epic-merge dry run re-run at current tips** (fresh scratch worktree, not the stale
     2026-08-25 snapshot): all seven tbtc-v2 branches (A, C, D, E, B, F, G) still merge with zero
     conflicts despite PR D's fork-point drift above (confirms the drift is harmless to the actual
     merge, only to a stacked-PR diff view). Compiles clean; Bridge runtime bytecode measured
     **22,791 B**, not the previously recorded 22,870 B (the earlier dry run predates PR G's
     `cf457613` fallback-revert-bubbling fix, which shaved ~79 B). Still 1,785 B under the 24,576 B
     EIP-170 limit. All 28 reservation regression tests pass on the merged tree.

## Resolved open items (carry-over from prior session, closed 2026-08-25)

- ~~**Deploy-script gap.**~~ **RESOLVED by PR #G.** `solidity/deploy/97_set_reservation_parameters.ts` exists on `m1/bridge-integration-seams` and runs exactly the sequence this item specified: `beginReservationCapsUpdate`/`finalizeReservationCapsUpdate` first (passes trivially since `reservationMaxTotalAmount` defaults to `0`), then `beginReservationParametersUpdate`/`finalizeReservationParametersUpdate` (wires the vault address into the Bridge), then `setVaultStatus(vault, true)` via `BridgeGovernance`. See row 7 above.

## Open items (agent-actionable, awaiting a decision — found in the 2026-08-26 review)

~~**Reservation lifecycle has zero behavioral test coverage across PRs C/D/E.**~~ **PARTIALLY RESOLVED 2026-08-26, see per-branch accounting below.** Per the human operator's call, attempted to split the test files so PRs C/D/E reviewers see the test in the same diff as the code. Final state after multiple rounds:

- PR G (`m1/bridge-integration-seams` @ `649b1062`): **13 new passing tests across 3 new self-contained files** (`Bridge.ReservationAcceptanceAuthorization.test.ts` 2/2 + `Bridge.ReservationSourceAnchorBinding.test.ts` 1/1 + `Bridge.ReservationStranding.test.ts` 10/10), added at `9a24212d`. Plus 2 pre-existing passing tests in `Bridge.ReservationSettlement.test.ts` (the PR #1104 characterization block, landed earlier at `93eccfb5`). Total 15/15 passing across 4 reservation test files. Verified at tip `649b1062` after the `Bridge`-dependency-declaration fix that landed in `07_deploy_tbtc_vault.ts` after the test files.
- PR E (`m1/timeout-and-stranding` @ `a5c7ef61`): **15 new passing tests in `Bridge.ReservationStrandingLibrary.test.ts` (renamed 2026-08-26 from `Bridge.ReservationStranding.test.ts` to avoid stack-merge add/add conflict with PR G's file of the same name) plus a new `contracts/test/ReservationStrandingExecutor.sol` TestExecutor contract.** Real PR-E-local coverage of `notifyReservationStranded`, `notifyStaleReservedDeposit`, and (transitively) `strandReservation` - all three exercised via a TestExecutor that holds its own `BridgeState.Storage` and forwards calls using `using Reservation for BridgeState.Storage`. Bypasses the missing router entirely. Verified 15/15 passing, 0 skipped.
- PR C (`m1/acceptance-core` @ `1f87f8d8`): **no test file shipped.** Cherry-pick attempt at `610609df` and stub-redo attempt at `ff68539d` both reverted; both produced placeholders with stubbed router entry points, providing zero functional coverage. Final state: no test file. The acceptance flow (`submitReservationAcceptanceProof`, `prepareReservationForSettlement`, `consumeAcceptedDeposit`) is exercised only by PR G's `Bridge.ReservationAcceptanceAuthorization.test.ts`.
- PR D (`m1/reanchor-core` @ `08536cd4`): **no test file shipped.** Cherry-pick attempt at `e0c0dd71` and stub-redo attempt at `f5221e47` both reverted for the same reason. The re-anchor flow (`submitReservationReanchorProof`, `requireCurrentSourceAnchor`) is exercised only by PR G's `Bridge.ReservationSourceAnchorBinding.test.ts`.

**Coverage net per branch:**
| Branch | Real passing tests | Functional coverage of branch code |
|---|---|---|
| PR C | 0 | none — coverage lives only on PR G |
| PR D | 0 | none — coverage lives only on PR G |
| PR E | 15 | real — `notifyReservationStranded`, `notifyStaleReservedDeposit`, `strandReservation` exercised end-to-end via TestExecutor |
| PR G | 13 new + 2 pre-existing | real - 13 new tests from `9a24212d` (acceptance + source-anchor + stranding) + 2 from `93eccfb5` (Settlement characterization). Both verified at tip `649b1062` |



## Open items (manager-not-actionable)

- ~~**Human pushes/opens all eight PR branches on GitHub**~~ **DONE 2026-08-27.** All 8 PRs pushed
  and opened as drafts, flat-based per `pr-strategy.md` §5/§9 (not stacked — corrected from an
  earlier stacked plan this session):
  - tbtc-v2, base `reservations-upgrade`: [#1106](https://github.com/threshold-network/tbtc-v2/pull/1106) (A), [#1107](https://github.com/threshold-network/tbtc-v2/pull/1107) (C), [#1108](https://github.com/threshold-network/tbtc-v2/pull/1108) (D), [#1109](https://github.com/threshold-network/tbtc-v2/pull/1109) (E), [#1110](https://github.com/threshold-network/tbtc-v2/pull/1110) (B), [#1111](https://github.com/threshold-network/tbtc-v2/pull/1111) (F), [#1112](https://github.com/threshold-network/tbtc-v2/pull/1112) (G).
  - keep-core, base `reservations-epic`: [#4274](https://github.com/threshold-network/keep-core/pull/4274) (H).
  - Correction: the main session **does** have `gh` API/push access (contrary to an earlier note
    that agent lacks it) — verified by these 8 successful `git push` + `gh pr create` calls.
  - Because the branches are git-stacked (built serially A→C→D→E→B→F→G / #4238→H) but based flat
    on the epic branch, downstream PRs render cumulative diffs today (e.g. G shows 30 files/+7549
    vs its own ~665 lines) — expected per §5's flat + sequential-merge design, not a defect. Each
    PR's diff shrinks automatically as its dependency merges. Review/merge in order A→C→D→E→B→F→G→H.
  - Full descriptions (now reflecting flat bases): `agent-docs/m1/pr-*-description.md`.
- **Epic integration (step map row 12-14: review/merge A-H onto `reservations-upgrade` /
  `reservations-epic`, full suite, final PR to `main`)** — all 8 PRs now open (see above);
  human review and merge in order A→C→D→E→B→F→G→H is the remaining step. De-risked by the
  2026-08-26 dry-run re-verification (all seven tbtc-v2 branches merge clean at current tips, no
  manual conflicts, 28/28 passing, Bridge bytecode 22,791 B — see item 9 above).
- **Post-m1: structural bound on re-anchor fee ratio** — `roadmap.md` §7 item 5, committed for post-m1 work.
- **Should fee revenue pay down `inKindFeeDebtSat` first?** — `roadmap.md` §7 item 6, independent of `vault.md`'s sweep-safety decision, not blocking m1.

## Key documents (read these before doing anything)

| Document | Purpose |
|---|---|
| `docs/spec/reservations/pr-strategy.md` | Versioned PR decomposition + build order. **Authoritative for what each PR contains.** |
| `docs/spec/reservations/feature-spec.md` | Reverse-engineered reservations feature spec. Authoritative for behavior. |
| `docs/spec/reservations/roadmap.md` | m1 scope decisions, including deferred items. |
| `agent-docs/m1/STATUS.md` | Working tracker — full handoff log, resolved items, current open items. |
| `agent-docs/m1/RUNBOOK.md` | Operational gotchas — environment quirks, code conventions, deploy-script conventions. |
| `agent-docs/m1/step-05-f-g-build-brief.md` | Build brief for PR #F and PR #G implementor (includes new deploy-script spec). |
| `agent-docs/m1/manager-to-implementor-2026-08-25-decision1-testrun.md` | Manager memo: PR #B test fix + deploy-script gap analysis. |
| `agent-docs/m1/manager-to-implementor-2026-08-25-next-steps.md` | Handover note: scope of next steps for the implementor agent. |
| `agent-docs/m1/manager-to-implementor-2026-08-25-pr-b-c-accepted.md` | Manager memo: PR #B and PR #C followup accepted, ready for human to push. |

## Correction policy

When a step reveals the plan got something wrong (branch already existed, conflict resolved differently than predicted, scope larger or smaller than estimated), update `pr-strategy.md` factually in the same way as the existing corrections in §4.1 and §9. Mirror the correction here in the "Resolved this session" or "New open items" section so the durable plan and the gitignored working tracker stay aligned.

— Coordinator
