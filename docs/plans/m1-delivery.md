# M1 Delivery Plan — UTXO Reservations

Versioned, durable counterpart to the working tracker `agent-docs/m1/STATUS.md`
(which is gitignored — see that file for full handoff log). This file is the
single canonical progress document for milestone 1 (UTXO reservations). Update
this when STATUS.md gets a major change; STATUS.md updates more freely as the
working scratchpad.

**Source of truth for the PR decomposition and order:** `docs/spec/reservations/pr-strategy.md` §4.1 + §9.
**Build order** (corrected 2026-08-24): **A → C → D → E → B → F → G** (then keep-core PR #H post-epic).

## Current status — 2026-09-03

| # | PR | tbtc-v2 branch | Status | What blocks |
|---|----|---------------|--------|-------------|
| 1 | A `storage-layout` | `m1/storage-layout` @ `e175092a` | **MERGED** [tbtc-v2#1106](https://github.com/threshold-network/tbtc-v2/pull/1106) @ `edfbe0c2` (2026-09-01) into `reservations-upgrade` | — |
| 2 | C `acceptance-core` | `m1/acceptance-core` @ `1f87f8d8` | **MERGED** [tbtc-v2#1107](https://github.com/threshold-network/tbtc-v2/pull/1107) @ `e30e3233` (2026-09-01) into `reservations-upgrade` | — |
| 3 | D `reanchor-core` | `m1/reanchor-core` @ `08536cd4` | **MERGED** [tbtc-v2#1108](https://github.com/threshold-network/tbtc-v2/pull/1108) @ `a3ea888e` (2026-09-01) into `reservations-upgrade` | — |
| 4 | E `timeout-and-stranding` | `m1/timeout-and-stranding` @ `a5c7ef61` | **MERGED** [tbtc-v2#1109](https://github.com/threshold-network/tbtc-v2/pull/1109) @ `76eab846` (2026-09-02) into `reservations-upgrade` | Review-fix [#1119](https://github.com/threshold-network/tbtc-v2/pull/1119) (11 confirmed findings) merged 2026-09-03 @ `24386f1f`, already folded in |
| 5 | B `router-minimal` | `m1/router-minimal` @ `3156ed50` | **MERGED** [tbtc-v2#1110](https://github.com/threshold-network/tbtc-v2/pull/1110) @ `8f30da66` (2026-09-03) into `reservations-upgrade` | Review-fix [#1118](https://github.com/threshold-network/tbtc-v2/pull/1118) (2 confirmed findings) merged 2026-09-02 @ `6568b8a6`, already folded in |
| 6 | F `m1/vault-pause-flags` | `m1/vault-pause-flags` @ `324eab4d` | **OPEN, CONFLICTING**: [tbtc-v2#1111](https://github.com/threshold-network/tbtc-v2/pull/1111) | Stale-snapshot conflict against the current `reservations-upgrade` tip (`8f30da66`): F's branch carries B's *pre-`#1118`* content, which now collides with B's actual merged content. 9 conflicting files (verified via `git merge-tree`, not just the GitHub label). Needs a fresh rebase onto `8f30da66`; next in build order. |
| 7 | G `m1/bridge-integration-seams` | `m1/bridge-integration-seams` @ `81bd85c4` | **OPEN, CONFLICTING**: [tbtc-v2#1112](https://github.com/threshold-network/tbtc-v2/pull/1112) | Same root cause as F (row 6), plus a 10th conflicting file (`WalletProposalValidator.test.ts`) since G is built on top of F. Was `MERGEABLE`/`CLEAN` earlier 2026-09-03 before B merged. Blocked on F's rebase landing first. |
| 8 | H `m1/keep-core-client` (keep-core repo) | `m1/keep-core-client` @ `48985451d` | **MERGED** [keep-core#4274](https://github.com/threshold-network/keep-core/pull/4274) (2026-09-03) into `reservations-epic` | — |

**Downstream keep-core chain past H (stacked #4278→#4279→#4280, per keep-core), verified live 2026-09-03:**

| PR | keep-core branch | Status | What blocks |
|----|------------------|--------|-------------|
| [#4276](https://github.com/threshold-network/keep-core/pull/4276) `m1/reservation-readiness-fixes` | fix(spv): re-verify reservation action generation before SPV proof submission | **MERGED** into `reservations-epic` | — |
| [#4277](https://github.com/threshold-network/keep-core/pull/4277) `m1/reservation-protobuf-marshaling` | reservation proposal marshaling test coverage | **MERGED** into `reservations-epic` | — |
| [#4278](https://github.com/threshold-network/keep-core/pull/4278) `m1/reservation-coordination-checklist` | fix(tbtc): remove frequency gate on reservation checklist actions | OPEN, draft, `mergeable: MERGEABLE`/`mergeState: CLEAN` against `reservations-epic` | Per its own tracking PR #4282: CI (`client-build-test-publish`) still failing on current head `b852948` — base auto-retargeted to `reservations-epic` after #4277 merged, but not yet rebased to pick up #4276/#4277's fix content. Needs a rebase + fresh CI run. |
| [#4279](https://github.com/threshold-network/keep-core/pull/4279) `m1/reservation-multisigner-integration-test` | multi-signer simulated integration test | OPEN, draft, `mergeable: CONFLICTING`/`mergeState: DIRTY`, base `m1/reservation-coordination-checklist` | Stacked on #4278; blocked on #4278 landing and rebasing first. |
| [#4280](https://github.com/threshold-network/keep-core/pull/4280) `m1/reservation-test-coverage-backfill` | M2 test-coverage backfill (7/8 items) | OPEN, draft, `mergeable: MERGEABLE`/`mergeState: UNSTABLE`, base `m1/reservation-multisigner-integration-test` | Stacked on #4279; blocked on #4278→#4279 landing first. |

**Tracking PRs:**
- [tbtc-v2#1116](https://github.com/threshold-network/tbtc-v2/pull/1116) (`reservations-upgrade` → `dev`, no diff) watches the tbtc-v2 stack against the fixed build order. Checklist text is stale (last updated 2026-09-01, before B merged) — refresh once F/G land. Flags a `dev`-divergence risk: `reservations-upgrade` forked from `main`, not `dev`; `dev` has 28 commits it lacks (confirmed zero overlap with `solidity/contracts/bridge/*`) — needs reconciling before the final PR to `dev`/`main` can land, independent of the F/G rebase.
- [keep-core#4282](https://github.com/threshold-network/keep-core/pull/4282) (`reservations-epic` → `dev`, no diff), opened 2026-09-03, watches the keep-core stack. Same `dev`-divergence pattern, worse: `reservations-epic` is 63 commits ahead / **68 commits behind** current `dev` (its own stated diff caveat — the PR's GitHub diff is computed against a stale merge-base and does not yet reflect a real merge onto current `dev`). Separately confirms `keep-core#4238` (`feat/utxo-reservation-wallet-support`, 12 commits, draft) remains a parallel, unreconciled branch targeting `reservations-epic` — independent of the H-chain, not merged, no reconcile-or-supersede decision made yet.

**Worktrees present** (`git worktree list`): `/tmp/m1-{a,b,c,d,e,f,g,h}` exist. `/tmp/m1-h-{a,r,w}` were transient parallel-builder worktrees for PR H's acceptance/re-anchor/watchers branches, merged into `m1-h` and safe to prune. `/tmp/src-{1091,1093,1094,1096,1102}` are read-only reference copies. (`m1-g2` worktree and `m1/bridge-integration-seams-g2` branch retired 2026-08-26 — fast-forward-merged into `m1/bridge-integration-seams`; single branch now carries all of PR #G.)

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
