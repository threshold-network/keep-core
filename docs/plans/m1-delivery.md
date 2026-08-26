# M1 Delivery Plan — UTXO Reservations

Versioned, durable counterpart to the working tracker `agent-docs/m1/STATUS.md`
(which is gitignored — see that file for full handoff log). This file is the
single canonical progress document for milestone 1 (UTXO reservations). Update
this when STATUS.md gets a major change; STATUS.md updates more freely as the
working scratchpad.

**Source of truth for the PR decomposition and order:** `docs/spec/reservations/pr-strategy.md` §4.1 + §9.
**Build order** (corrected 2026-08-24): **A → C → D → E → B → F → G** (then keep-core PR #H post-epic).

## Current status — 2026-08-25

| # | PR | tbtc-v2 branch | Status | What blocks |
|---|----|---------------|--------|-------------|
| 1 | A `storage-layout` | `m1/storage-layout` @ `e175092a` | Built, awaiting human to open | — |
| 2 | C `acceptance-core` | `m1/acceptance-core` @ `1f87f8d8` | **Clean 2026-08-25, awaiting human to open** (line-numbers fix landed) | **Duplicate wallet-binding guard annotated per option (b).** Net `-8/+2` on `Reservation.sol:391-398`, no behavior change. Directive comment line-number refs corrected at `1f87f8d8`. |
| 3 | D `reanchor-core` | `m1/reanchor-core` @ `08536cd4` | Built, awaiting human to open | — (`cumulativeReanchorFee += minerFee` confirmed at `ReservationProofs.sol:692`) |
| 4 | E `timeout-and-stranding` | `m1/timeout-and-stranding` @ `2e63515f` | Built, awaiting human to open | — |
| 5 | B `router-minimal` | `m1/router-minimal` @ `3156ed50` | **Clean 2026-08-25, awaiting human to open** | Bootstrap-ordering test split into two `it` blocks (hazard + safe order). 9/9 green with `FORKING_URL` unset. |
| 6 | F `m1/vault-pause-flags` | `m1/vault-pause-flags` @ `941d79e9` | **Built 2026-08-25, awaiting human to push** (Option B scope) | Implementor may proceed to PR #G build brief |
| 7 | G `m1/bridge-integration-seams` (deploy scripts + contract seams + tests) | `m1/bridge-integration-seams` @ `cf457613` | **Built 2026-08-25 (deploy scripts by implementor; contract seams by manager-orchestrated subagents after implementor agent retired), awaiting human to push** | 95/96/97 deploy scripts (97 references functions that land in the contract-seam layer; TODO already replaced with reference to `agent-docs/inventory/reservation-parameters.md`; `reservationTermSeconds` bumped to the on-chain 90-day floor — full comparison table in the inventory file's "Comparison against `feature-spec.md`" section). Governance begin/finalize wrappers in `BridgeGovernanceParameters.sol`/`BridgeGovernance.sol`; `isReservedDeposit` view, `setReservationRouter` s…
| 8 | H `m1/keep-core-client` (keep-core repo) | `m1/keep-core-client` @ `48985451d` | **Built 2026-08-26, awaiting human to push** | Base PR #4238 (`b4f63944`), 62 files vs `main` merge-base, +23040/-50. Chain-interface writes/reads/events on `pkg/tbtc.Chain`, then `pkg/tbtcpg.Chain`/`pkg/maintainer/spv.Chain`; acceptance + re-anchor proposal/proof tasks; three watchers (stranding/stale-deposit/action-timeout); operator wiring in `pkg/tbtcpg/tbtcpg.go`, `pkg/maintainer/spv/spv.go`, `pkg/tbtc/tbtc.go` gated on `config.Reservations.Enabled`. `go build ./...` clean, 552 tests passing. Full detail: `agent-docs/m1/STATUS.md` row 11 and "PR H build completed 2026-08-26" session note. |

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
   - **PR D's real git fork point from PR C is `f1ede944`, not C's current tip `1f87f8d8`** — D
     was forked two commits before C's later duplicate-guard-comment fix (`3419e475`,
     `1f87f8d8`) landed, and was never rebased. D's own diff never touches that region (confirmed:
     D's hunks land at lines ~281/~334/~536 relative to its base), so the eventual epic merge is
     unaffected — verified below. The one real consequence: opening PR D on GitHub with base set
     to `m1/acceptance-core` *before* PR C merges will show a spurious 2-line "revert" of PR C's
     comment fix in D's diff. `pr-D-description.md` corrected to state the true fork point and
     flag this; left as an open human decision (rebase D..G, or open D directly against the epic
     branch instead of stacking it on C) rather than auto-rebased — a rebase cascades through
     E/B/F/G and would need all five re-verified plus a fresh dry run.
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

- **Reservation lifecycle has zero behavioral test coverage across PRs C/D/E.**
  See item 9's headline finding above. Two remediation paths, neither started:
  (a) write a behavioral suite exercising accept/prove/settle, re-anchor, timeout,
  and stranding through the router (PR B) before human push — this is genuinely
  new test-writing work, not a quick fix, likely several hundred lines; or (b)
  accept the gap for m1 and disclose it explicitly in the PR descriptions/review
  request so reviewers aren't relying on a false "9,908 lines of reservation
  tests" justification. Decision needed before opening PRs C, D, E, or B.
- **PR D's stacked-base drift vs PR C** (item 9 above). Decision needed before
  opening PR D: rebase D→G onto C's current tip (cascades, needs re-verification
  of D/E/B/F/G plus a fresh dry run), or open D directly against the epic branch
  rather than stacking it on C.

## Open items (manager-not-actionable)

- **Human pushes/opens all eight PR branches on GitHub** — agent has no `gh` API access for
  pushing or opening. All branches are local-only, built and green: `/tmp/m1-{a,b,c,d,e,f,g,h}`
  (A `m1/storage-layout`, B `m1/router-minimal`, C `m1/acceptance-core`, D `m1/reanchor-core`,
  E `m1/timeout-and-stranding`, F `m1/vault-pause-flags`, G `m1/bridge-integration-seams` — all
  five tbtc-v2 branches stack in build order A→C→D→E→B→F→G onto `milestone/utxo-reservation-m1`;
  H `m1/keep-core-client` in the keep-core repo, stacked on PR #4238). Full descriptions in
  `agent-docs/m1/pr-*-description.md`.
- **Epic integration (step map row 12-14: review/merge A-H onto `milestone/utxo-reservation-m1`,
  full suite, final PR to `main`)** — gated on A-H being human-opened first; de-risked by the
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
