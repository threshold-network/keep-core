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

## New open items (carry-over from prior session)

- **Deploy-script gap is real, not hypothetical.** `solidity/deploy/95_deploy_reservation_vault.ts` ends with an explicit NOTE block delegating all wiring to governance. `96_transfer_reservation_vault_ownership.ts` only transfers `Ownable`. Repo-wide grep across `solidity/` (excluding `node_modules`/`test/`/`artifacts/`/`cache/`) finds zero callers of `updateReservationParameters` or `updateReservationCaps` outside the contract definitions, the `BridgeGovernance` wrapper at `:1862`, and `typechain/` bindings. On a fresh deploy today the vault ships orphaned. The new `97_set_reservation_parameters.ts` belongs in **PR #G** (it calls `BridgeGovernance` setters, which is PR #G's stated scope) and must call `updateReservationCaps` first (passes trivially because `reservationMaxTotalAmount` defaults to `0`), then `updateReservationParameters`. Without this script, reservations are unreachable on a clean deploy.

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
  2026-08-25 dry-run merge (all seven tbtc-v2 branches merge clean, no manual conflicts).
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
