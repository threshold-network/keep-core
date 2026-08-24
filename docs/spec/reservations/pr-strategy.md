# PR Strategy for Milestone 1 Delivery

Status: LIVE — the milestone 1 delivery plan (2026-08-21). Supersedes
`epic-merge-plan.md` as the delivery vehicle.
Scope: milestone 1.

## 1. Problem Statement

Eight existing stacked PRs form the reference implementation for tbtc-v2 UTXO reservation:
- Base: `reservations-epic`
- Root: #1088 (`feat/utxo-reservation-core`)
- Sequential: #1090 (`feat/utxo-reservation-router`), #1091 (`feat/utxo-reservation-settlement`), #1092 (`feat/utxo-reservation-renewal`), #1093 (`feat/utxo-reservation-backing`), #1094 (`feat/utxo-reservation-guards`), #1095 (`docs/utxo-reservation-release`), #1096 (`feat/utxo-reservation-partial-redemption`)

Measured facts from `inventory/pr-map.md`:
- Three branches are behind their bases: #1088 by 2 commits, #1091 by 13 commits, #1092 by 5 commits.
- The #1102 fold (`3566e059`) is present on #1088 and #1090 branches but absent from #1091 and #1092 because #1091 is 13 commits behind #1090.
- One standalone keep-core PR #4238 provides a reusable two-phase type layer but no executor (`milestone-inventory.md` C-8).
- Milestone 1 is a rewrite (variant B with minimal router), so the stacked PRs are reference material only, not a delivery vehicle.

Constraint: Milestone 1 must ship creation, custody and re-anchoring, omitting dissolution, redemption and renewal. The reservation vault is not upgradeable (plain `Ownable`), so its entry points must ship in Milestone 1 with initiation disabled behind a pause flag (per `roadmap.md` §0.7 and established fact 1).

## 2. Assessment of Five Delivery Options

### a. Force-push over existing PRs — rejected: orphans every review comment
**Mechanics**: Rewrite each branch locally with m1 content, then force-push to update the PR's head branch.
**Cost**: Low immediate effort (no new branches). 
**Failure modes**:
- Existing review comments and approvals are **lost** when a branch is force-pushed (GitHub preserves comments only if the commit SHA remains in the ref history; force-push replaces the ref, orphaning the old commits and detaching comments).
- Approvals are tied to the specific commit SHA; force-push invalidates them.
- Reviewers see a "force-pushed" banner and must re-examine the entire diff, wasting prior review effort.
- History of the original implementation is destroyed in the PR view (only accessible via reflog for 90 days).

### b. Merge the stack, then delete — rejected: m1 would ship dissolution, redemption and renewal
**Mechanics**: Merge all eight PRs into `main`, then submit follow-up PRs to delete m2-only code.
**Cost**: Medium (merge conflict resolution, then deletion PRs).
**Failure modes**:
- Milestone 1 would ship with dissolution, redemption and renewal code present (violating create-only scope).
- Follow-up deletion PRs create noise and risk of incomplete removal.
- Increases review burden: reviewers must verify deletions are correct and complete.
- Intermediate state on `main` is not shippable (contains unwanted features).

### c. Archive-branch copies — rejected: PR numbers and titles stop matching their content
**Mechanics**: For each PR, create a new branch (e.g., `archive/#1088`) copying the original state, then rewrite m1 on the original branch name.
**Cost**: High (duplicate branch management, risk of confusion).
**Failure modes**:
- Archive branches clutter the namespace and are easily forgotten.
- Original PRs now contain m1 content but retain old PR numbers and titles, causing misalignment.
- Reviewers may accidentally review archive branches instead of active ones.
- No clear mapping between original PR context and new content.

### d. Fresh PRs on a new epic branch, old PRs closed — **chosen** (§4)
**Mechanics**: Create a new epic branch (e.g., `milestone/utxo-reservation-m1`) from `main`. Open new PRs against it for each m1 component. Close the original eight PRs without merging.
**Cost**: Medium (new branch setup, PR creation).
**Failure modes**:
- Original PRs remain visible but are marked closed; reviewers must be directed to the new PRs.
- Loss of automatic linkage between original issue/PR and new work (requires manual tracking).
- Closed PRs still appear in searches and may cause confusion if not clearly labeled.

### e. Tag or draft preservation — rejected: a tag does not keep a PR's review threads attached
**Mechanics**: Tag the tip of each original branch (e.g., `v1.0/#1088`) or convert to draft PRs, then rewrite on original branch names.
**Cost**: Low (tagging) or medium (draft conversion).
**Failure modes**:
- Tags are not PR-specific and do not preserve review comment threads.
- Draft PRs retain the ability to be merged accidentally; reviewers may comment on outdated drafts.
- No guarantee that tags or drafts survive repository maintenance (e.g., tag pruning).
- Does not solve the problem of preserving review context for auditors.

## 3. A merged or closed PR stays readable after its branch is deleted

**Answer**: deleting the head branch does not take the pull request's commits,
diff, or review threads with it. Closing or merging a PR and deleting its branch
leaves the PR page and its full review history intact.

**Evidence — measured against this repository, 2026-08-21.** GitHub keeps a
`refs/pull/N/head` ref per pull request, independent of the head branch.
Reproduce:

```
# 1. #1102 was merged and its branch deleted: absent from the branch list
git ls-remote --heads origin | grep -c 'utxo-reservation-review-followups'   # -> 0

# 2. its PR ref survives and still resolves
git ls-remote origin 'refs/pull/1102/head'
# -> 1706ddf5db9e9cc5836e1912920af1978445a493  refs/pull/1102/head

# 3. and the commits are fetchable
git fetch origin 'refs/pull/1102/head:refs/tmp/pr1102'
git log --oneline -1 refs/tmp/pr1102
# -> 1706ddf5 test(reservation): characterize worst-case underbacking

# 4. retention is not recent or partial: 875 PR refs are present, and PR #1
#    (the repository's first) still fetches
git ls-remote origin 'refs/pull/*/head' | wc -l                             # -> 875
git fetch origin 'refs/pull/1/head:refs/tmp/pr1' && git log --oneline -1 refs/tmp/pr1
# -> fcf60513 Initial development setup
```

**What that measurement covers, and what it does not.** It proves commit and
diff retention for this repository across its whole history, which is the half a
git tag could have substituted for. It does not prove review-thread retention,
because threads are not git objects: they are records on the PR attached to
`(pull_request_id, commit_sha, path, line)`, which is why they keep rendering
once the branch is gone, and why a tag could never have preserved them in the
first place. Diff retention is measured; thread retention is observable on any
closed PR in this repo but is not something this document has tested
systematically.

**Confidence**: high for commit and diff retention (measured above). The
practical conclusion is unchanged and now rests on evidence: **no archive branch
or belt-and-braces tag is needed** — the PR is the preservation mechanism, and a
tag would have preserved strictly less than the PR ref already does.

**One caveat worth stating plainly**, since it is the obvious objection: GitHub
ships a *restore branch* button for closed PRs, which exists precisely because
people delete branches they still want. That feature restores a *branch* for
further work. It is not evidence that a deleted branch costs you the PR's
readability, which is what the measurement above settles.

**Corrected 2026-08-21.** This section previously quoted two passages as GitHub
documentation — one attributed to the "Restore Tidied Pull Requests" blog post,
one to the "Deleting and restoring branches in a pull request" docs page — and
claimed "Verified via GitHub's documented behavior". Both pages were read
directly and **neither contains the sentence attributed to it**: the blog post
is three sentences announcing the restore-branch feature, and the docs page is a
click-path how-to. The quotations were fabricated, and a branch-deletion
decision rested on them. The conclusion survives, but only because the
measurement above replaces them.

## 4. Recommended m1 PR Decomposition

Milestone 1 must ship the complete storage layout atomically (no live reservation may span a layout change). Therefore, we use an **epic branch** (`milestone/utxo-reservation-m1`) as the integration target, with several reviewable PRs onto it. Each PR is self-contained and reviewable in isolation.

### 4.1 tbtc-v2 PRs

| PR Name (targeting epic branch) | Source PRs | Review Focus | Rough Size (Solidity +/-) | Ordering Justification |
|---------------------------------|------------|--------------|----------------------------|------------------------|
| `m1/storage-layout` | #1088, #1090, #1091, #1092, #1093, #1094, **#1096**, **#1102** | Full storage layout declaration (all fields, enums, structs) and governance parameters. Must be written exactly as in m2 (no omissions). **#1096 is required** for three declare-only fields that exist only on the partial branch — `ReservationAction.isPartial`, `retryCreditSourceNonce`, `BridgeState.Storage.reservationRetryCreditActionNonce` (`milestone-inventory.md` §4). **Also required, ported from `#1102`** (not extracted from `#1090`'s fold branch — §7): `ReservationRequest.cumulativeReanchorFee`, `maxCumulativeReanchorFee`, `reservationDissolutionTxMaxFee`, and a re-derived `BridgeState.Storage.__gap` (do not copy `#1102`'s own `__gap` number — it was computed against a different field set). All three ship **declare-only**: no m1 path writes or reads them, because the 2026-08-23 lever-4 decision rejected the flat ceiling. They are declared anyway per Rule 5 and `m1-b-implementation.md` §4.5 — a field a later milestone reads cannot be added while reservations are live — which `milestone-inventory.md` §1.3 calls "the load-bearing one" for `cumulativeReanchorFee` specifically, since re-anchor settlement writes the claim down each hop and the cumulative total is unrecoverable from surviving state. **History: an edit on 2026-08-24 struck these three fields from this row, reading the lever-4 decision's cost sentence ("adds a `ReservationRequest` storage field ... plus a parameter validated only `> 0`") as rejecting the declarations too. That edit was wrong and is reverted. Row 140 below already drew the line correctly on 2026-08-23 — lever 4 drops the *check*, not the declaration — and the 2026-08-24 edit was made without reading it.**  **Also required, and missed until 2026-08-24: `activeReservationsCount` and `maxActiveReservations`.** `milestone-inventory.md` §2.1 lists both as m1 fields that m1 **writes and reads**, with no extraction source, and §4.1 of `m1-b-implementation.md` makes them a launch gate — they convert variant B's saturation cliff into a revert. Declared as `uint32`s, which pack into the free tail of the fee-parameter slot, so `__gap` stays 32. PR #A declares them only; the acceptance-time cap check, the counter's increment and decrement sites, and the paired router view are new work owned by PR #B. Assert the layout field-for-field against §2.1 and §4 — `m1-b-implementation.md` §4.5 is a launch gate. | +538 -1 measured | First: foundation. No behavior depends on it yet, but all later PRs require these declarations. |
| `m1/router-minimal` | #1090 (post-#1102) through **#1094** — verified 2026-08-21: only `submitReservationProof` and `updateReservationParameters` exist at #1090; `requestReservationAcceptance`/`requestReservationReanchor`/`notifyReservationActionTimeout` first appear at #1091, `updateReservationCaps` at #1093, `notifyStaleReservedDeposit`/`notifyReservationStranded` at #1094 (#1102 does not touch `ReservationRouter.sol`, so no fold gap here) | Minimal router: 8 retained entry points (acceptance, re-anchor, submitProof, timeout, stale deposit, stranding, update parameters, update caps), 11 views, 1 new view (`activeReservationsCount` — genuinely new, not present at any source tip), 4 delegatecall invariants, EIP-170 workaround. | +600 -200 | Second: depends on storage layout. Enables entry-point stubs. |
| `m1/acceptance-core` | #1091 (acceptance half) + **#1093** | Two-phase acceptance: request side (vault set, deposit revealed, wallet Live, nonce increment), proof side (SPV, consumeAcceptedDeposit, emit events), settlement path (writes `expiresAt`, `dissolutionEligibleAt`, `mintedAmount`, `anchorAmount`, `state=Active`, external mint). **Extract `#1093` unmerged (§7, corrected 2026-08-22)**: `#1091`-`#1096` is the canonical design (every spec doc in this set is written against it), not one side of a conflict with `#1090`+`#1102` to reconcile — attempting that merge fails 56/121 tests and overflows EIP-170 (`agent-docs/m1/step-01-execution-report.md`). `#1093` already reworks the backing-accounting model #1091 established (H-04, `inventory/pr-map.md` §5.3); it needs no fold content beyond §9 step 3's ported items. | +1800 -400 | Third: uses router and storage. First reachable path. |
| `m1/reanchor-core` | #1091 (re-anchor half) + **#1093** | Re-anchor request and proof (unbounded, no dissolution gate), settlement path (wallet amount/count transfers, `walletPubKeyHash` update, `anchorAmount` rewrite, external in-kind fee call). **Extract `#1093` unmerged (§7, corrected 2026-08-22)**. **Corrected 2026-08-23: do not port enforcement, only the field declarations move (and those belong to PR #A, not here).** Verified 2026-08-21 that raw `#1093` has the H-04 `mintedAmount = newAnchorAmount` rewrite but zero occurrences of `cumulativeReanchorFee` anywhere in `Reservation.sol`/`ReservationProofs.sol` — it lacks `#1102`'s `maxCumulativeReanchorFee` grinding-cap *check* inside `submitReservationReanchorProof` entirely. Step 3 investigated porting that check; the 2026-08-23 decision accepted the resulting unbounded-ratio exposure for m1 instead (`pr-review-followups.md` items 5/7, lever 4) and deferred a structural bound to post-m1 work (`roadmap.md` §7 item 5). `ReservationRequest.cumulativeReanchorFee` and `BridgeState.Storage.maxCumulativeReanchorFee` still ship as **declare-only** fields per storage completeness (`milestone-inventory.md` §1.3, "the load-bearing one" — a position field m2 will read cannot be added later without the live-state migration §4.5 forbids); that declaration is PR #A's job (§4.1 row above), not this PR's. This PR only drops the *check*. The acceptance is pinned by a characterization test, now live at tbtc-v2 `#1104` (staged directly on `#1093`, not the epic branch — carry it forward verbatim per §6's extraction-provenance convention when this PR is opened). | +1000 -300 | Fourth: builds on acceptance-core; shares settlement helpers. |
| `m1/timeout-and-stranding` | ~~#1091 (timeout)~~ **#1093 (timeout, corrected 2026-08-24)**, #1094 (stale deposit cleanup + stranding) | Timeout slashing path (`notifyReservationActionTimeout`), stranding group from #1094 — stale deposit cleanup (`notifyStaleReservedDeposit`) and stranding (`notifyReservationStranded`, `strandReservation`, `strandLateSettlementIfTargetWalletClosed`); `notifyStaleReservedDeposit` does not exist at #1091, only from #1094 on. **Do not extract the timeout path from `#1091`.** `#1091` predates a field rename: it calls the storage field `reservationGracePeriod` (4 occurrences in its `Reservation.sol`, plus `BridgeState.sol`, `IReservationBridge.sol`, `ReservationProofs.sol`, `ReservationRouter.sol`), which `#1093` renamed to `reservationDissolutionDelay`. It is the only `#1091` storage name absent from `#1096`, and PR #A declares only the new name, so `#1091`'s text will not compile. `#1093` carries the same `notifyReservationActionTimeout` (91 lines) post-rename — extract from there. | +800 -150 | Fourth in the corrected build order; see the "Build order" note below this table. |
| `m1/vault-pause-flags` | tbtc-v2 #1088 (base vault) + **#1093** (in-kind fee financing) | ReservationVault with pause flags on initiation-only entry points (`redemptionsPaused` constructor-default `true` for redemptions, `renewalsPaused` constructor-default `true` for renewals), The fee-financing surface (`financeInKindFee` `:529`, `repayInKindFeeDebt` `:568`, `inKindFeeDebtSat`, `updateFeeReserveTarget` `:599`) must be **present and ungated** — re-anchor is B's only unpin and calls it on the settlement path (`m1-b-implementation.md` §3). Note `pauseRenewals`/`unpauseRenewals` already ship (`:409`, `:415`); what is new is the `redemptionsPaused` trio. Plus restrictive `pauseRenewals` and `unpauseRenewals` functions. | +662 −1 | Sixth: isolates vault changes; depends on storage layout for state reads. |
| `m1/bridge-integration-seams` | New work | Bridge integration seams: `Deposit.sol` reveal path, `Wallets.sol` lifecycle gates, `WalletProposalValidator.sol` (sweep-exclusion guard plus m1 proposal validators), `BridgeGovernance.sol` and `BridgeGovernanceParameters.sol` wiring, and `Bridge.sol`'s `isReservedDeposit`, `setReservationRouter` and `fallback`. Review focus must note that `WalletProposalValidator` is non-upgradeable, so its m1 content is a real decision rather than a free omission. | +800 -50 | Seventh: integrates vault with bridge; depends on storage layout and router. |

**Build order - CORRECTED 2026-08-24. The `Ordering Justification` column
above is superseded by this paragraph.** The column ordered PR #B second, but
`ReservationRouter.sol` is a thin dispatch surface: all 12 of its entry points
call `Reservation.<fn>` library functions (via `using Reservation for
BridgeState.Storage`) plus `ReservationProofs.submitReservationProof`. None of
that logic exists after PR #A, which is declarations only, so #B could not
compile in second position and would have violated §4's "self-contained and
reviewable in isolation" rule. Compounding it, `Reservation.sol` and
`ReservationProofs.sol` import each other, so they form one indivisible
compilation unit that #C, #D and #E all add functions to.

The buildable order is **A -> C -> D -> E -> B -> F -> G**, decided by the
roadmap owner on 2026-08-24. Every PR keeps exactly the content its row
specifies; only the sequence changes, so the router still lands once as a
single reviewable unit with its 8 entry points and 4 delegatecall invariants
intact. §9's step *numbers* are deliberately left alone because other
documents cite them ("§9 step 3", "step 4/PR #A"); read §9 as a list of work
items, not as an execution sequence.

**Extraction tip - CORRECTED 2026-08-24. Rows 139, 140 and 141 say "extract
`#1093`"; for m1 *code* the correct tip is `#1094`.** The 2026-08-22
correction that pointed those rows at `#1093` was answering a different
question - whether to merge `#1090`+`#1102` into the lineage (no) - and was
read afterwards as naming `#1093` the extraction tip. It is not the furthest
m1-scoped tip. Measured 2026-08-24: `#1094` is **not** purely additive over
`#1093`. It rewrites five members that PRs #C and #D had already extracted:

| Member | `#1093` | `#1094` | What `#1094` adds |
|---|---|---|---|
| `requestReservationAcceptance` | 153 L | 163 L | wallet-binding guard: only the wallet a deposit was revealed for may be authorized as custodian |
| `consumeAcceptedDeposit` | 26 L | 33 L | double-consume guard on the pending marker, plus the `pendingReservedDeposits` decrement |
| `settleAcceptance` | 88 L | 135 L | unwind of a newer pending generation's reserved capacity (described in-source as a permanent leak otherwise); late-settlement routing through the deposit's immutable `vault` rather than the live `reservationVault`; the stranding hook |
| `submitReservationReanchorProof` | 137 L | 158 L | — |
| `unwindPendingAction` | 57 L | 69 L | — |

Extracting from `#1093` therefore ships m1 with a security guard and two
correctness fixes missing. PRs #C and #D were built from `#1093` first, then
rebuilt from `#1094` on the roadmap owner's decision. Same rule PR #A already
followed when it took the layout from `#1096`: use the furthest tip whose
changes are still in scope.

`#1096` is **not** that tip for code. It edits `requestReservationAcceptance`,
`submitReservationReanchorProof` and `unwindPendingAction` again and deletes
`requireCurrentSourceAnchor` outright, all partial-redemption (m2) work.
`#1096` contributes declare-only fields to PR #A and nothing else.

**Consequence for row 141.** `strandReservation` and
`strandLateSettlementIfTargetWalletClosed` move out of PR #E and into PR #C:
`#1094`'s `settleAcceptance` calls the stranding hook directly, so the helpers
cannot land later than the function that calls them. PR #E reduces to
`notifyReservationActionTimeout`, `notifyReservationStranded` and
`notifyStaleReservedDeposit`. `addWalletReservationKey` and
`removeWalletReservationKey`, which `#1093` did not have at all, also land
with PR #C for the same reason.

**Total lines — RESOLVED 2026-08-23 by measurement. The `Rough Size` column
above is superseded by the table below; the two figures it was reconciled
against were both wrong and were never in the same unit.**

`m1 = ~4,500 net contract Solidity lines` (range 4,400-4,600), measured against
`origin/feat/utxo-reservation-guards` by classifying every symbol in all 20
changed contract files as m1 or m2. Full method and evidence:
`agent-docs/m1/step-04-line-count-reconciliation.md`.

Why the old comparison was void:
- **5,261 is an additions-sum.** Its chain starts at `roadmap.md` §5.1's 9,206,
  measured "additions only" from PR diffs, so a line rewritten by a later PR in
  the stack counts twice. 9,206 is reproducible and correct in that unit.
- **6,862 was a net-diff estimate**, pricing fresh extraction PRs onto an epic
  branch that holds no reservation code and so has nothing to rewrite.
- Subtracting one from the other produced a meaningless 1,601.

The number that settles it: `git diff origin/main
origin/feat/utxo-reservation-guards -- solidity/contracts` is **5,958
additions**, and that includes every m2-only function — dissolution, in-kind
redemption, renewal, watchtower veto, retry credit. m1 alone therefore cannot
cost 6,862. Measured m2-only content is 1,440 lines, leaving ~4,500 for m1 plus
~25 lines of genuinely new work.

Corrected per-PR sizes, replacing the column above:

| PR | old estimate | measured | note |
|---|---|---|---|
| #A `m1/storage-layout` | +1,200 | **~380** | declarations only |
| #B `m1/router-minimal` | +600 | **~690** | slightly under-estimated |
| #C `m1/acceptance-core` | +1,800 | **~470** | plus shared helpers below |
| #D `m1/reanchor-core` | +1,000 | **~294** | |
| #E `m1/timeout-and-stranding` | +800 | **~287** | |
| #F `m1/vault-pause-flags` | +662 | **~642** | old figure was the file's line count, not an estimate |
| #G `m1/bridge-integration-seams` | +800 | **~665** | extractable, not "New work" |
| shared proof/lifecycle helpers | — | **~480** | needed by #C/#D/#E jointly; count once |

The `-` column's ~1,150 is also overstated: the net diff shows only 48
deletions across the whole stack.

**Justification against one atomic PR**:
- An atomic PR would bundle the whole ~4,500-line m1 surface, making review impractical and increasing the risk of missed errors.
- Decomposition isolates concerns: storage (layout), router (interface), acceptance (core product), re-anchor (unpin), timeout/stranding (cleanup), vault (side-car), bridge integration.
- Each PR can be tested independently against the epic branch using existing test harnesses (9,908 lines of reservation tests).
- Alternative (one atomic PR) forces reviewers to re-verify the entire storage layout and behavior in one sitting, which is error-prone and violates the principle of reviewable increments.

### 4.2 keep-core PRs

| PR Name (keep-core repo, targeting the keep-core epic branch) | Source PRs | Review Focus | Rough Size (Go +/-) | Ordering Justification |
|---------------------------------|------------|--------------|----------------------------|------------------------|
| `m1/keep-core-client` | **#4238** (build on, not supersede — §8) | Keep-core client rework for acceptance and re-anchor with nonce-carrying proposals, regenerated ABI bindings, and executor duties. | ~1,100-1,400 +1,200 | First: client work; must wait for stable tbtc-v2 ABI surface. |

**Total estimated lines**: ~1,100-1,400 production Go (`m1-variant-comparison.md`
§3 and `timeline-estimate.md` §7), ~1,200 test Go. (Corrected 2026-08-21: this
previously said ~1,650, which is 18% above the top of the range every other doc
gives and was unsourced. `milestone-inventory.md` D-26 asks for a bottom-up
rebuild of this figure.)

**Cross-repo ordering constraint**: keep-core binds against the tbtc-v2 ABI, so the Solidity entry-point surface must be stable before the Go client is finalised.

## 5. Why the flat set, not a stack

Stacking's costs are structural, not tooling bugs. A stacked PR targets its
parent's branch rather than the integration branch, and three consequences
follow from that base choice alone:

- **Restacking.** When a parent merges or is rewritten, each child must be
  retargeted and rebased. `gh pr edit --base <branch>` does the retarget
  ([`gh pr edit` manual](https://cli.github.com/manual/gh_pr_edit): `-B,
  --base <branch>`, "Change the base branch for this pull request"), but the
  rebase is manual and repeats per child.
- **Reviewers see the diff against the immediate parent**, not against the
  integration branch, so no single PR shows the reviewable whole.
- **A merge queue on `main` never sees the children.** A queue is configured on
  the target branch and enqueues PRs whose base *is* that branch; a PR based on
  a sibling feature branch is not a candidate until it is retargeted.

**Corrected 2026-08-21.** This section previously asserted that `gh pr edit
--base` "is blocked once GitHub recognises a stack, which is why the prior plan
needed the `gh-stack` extension", and that stacked PRs "do not support
auto-merge or merge queues". The first is false: `gh pr edit` documents `--base`
with no stack condition, and GitHub has no native stacked-PR concept that could
trigger one. The second overstated the auto-merge half — auto-merge is a
per-PR setting and does not depend on what the base branch is. The merge-queue
half is restated above as the base-branch consequence it actually is.
**UNVERIFIED**: the auto-merge and merge-queue behaviours above are reasoned
from how the features are configured, not tested — testing them would mutate
real pull requests.

**The recommendation does not use stacking.** A flat set of PRs onto one epic
branch per repo avoids restacking entirely (each merges independently), allows
parallel review with no dependency chain, and gives every PR a stable base, so
its diff is the reviewable unit. The epic branch is the single integration
point.

## 6. Salvaging Review Effort Already Spent

**Concrete mechanism**: For each m1 PR, include a section in the PR description titled "Extraction Provenance" that lists:
- The source PR(s) and commit range used, for example "Extracted from #1091 commit range `<base>..<head>`".
- Specific files and line ranges copied verbatim, for example "`ReservationProofs.sol:343-390` (acceptance proof dispatcher)".
- A link to the original review thread on those lines. The eight reservation PRs live in `threshold-network/tbtc-v2`, so the shape is `https://github.com/threshold-network/tbtc-v2/pull/<N>#discussion_r<id>`; take the real `<id>` from the thread's own permalink rather than constructing it.
- A note on any modifications made for m1, for example "Removed `Redemption` and `Dissolution` arms from the dispatcher; otherwise verbatim".

**Example shape for `m1/acceptance-core`** (placeholders, not real links):
> Extraction Provenance:
> - Source: #1091 (`feat/utxo-reservation-settlement`), `ReservationProofs.sol:343-390`
> - Verbatim copy of the `submitReservationProof` dispatcher with the `Redemption` and `Dissolution` arms removed.
> - Original review: `https://github.com/threshold-network/tbtc-v2/pull/1091#discussion_r<id>`
> - This component was reviewed in #1091 as part of the two-phase settlement entry point.

This allows reviewers to focus only on the m1-specific changes (removal of m2 arms) and trust the verbatim extraction, drastically reducing re-review effort.

## 7. Handling the #1102 Fold Problem

The #1102 fold (30 review fixes) is absent from `#1091` **and everything above it** — `#1092` through `#1096`, including the guards and partial tips. `#1091` is 13 commits behind `#1090` and `#1092` is 5 behind `#1091`; `#1093`-`#1096` are 0 behind their own parents but inherit `#1091`'s staleness, so being current with your base does not mean carrying the fold. Extracting from the guards tip (`#1094`) silently drops these fixes.

**Recommendation**: port #1102's m1-relevant fixes onto `#1091`-`#1096` individually, extracting those branches unmerged. Do not rebase or merge the two trees — verified 2026-08-22 that neither produces a usable extraction source (see below). `#1091`-`#1096` is the canonical codebase: every spec doc in this set (`feature-spec.md`, `inventory/data-model.md`, `inventory/proofs.md`, `roadmap.md`, `inventory/vault.md`) is written exclusively against its action-record model (`ReservationProofs.sol` split out, `ReservationAction`/`ActionType`/`ActionState`, `retryCredit`/`requestNonce`); `#1090`'s `isRetry`/`lastTimeoutWasWalletFault`/redemption-timestamp-keyed-veto model appears nowhere in the doc set — it is superseded, not a parallel branch to reconcile with. `inventory/pr-map.md`'s own PR-disposition table already said this: extract `#1088` "post-#1102, post-#1091 form" — the storage foundation plus #1091's mechanics replacement, not a blend of both mechanics.

**Why not rebase or merge**:
- **Rebase fails on the git history itself, not on code conflicts.** `#1090`'s commits were rewritten (new SHAs) when `#1102`'s fold landed, so `merge-base(#1090, #1091)` sits at the pre-fold `#1088` tip (`3d4335d7`), not at `#1090`'s current tip. `router..settlement` is 37 commits containing stale duplicates of `#1090`'s own rewritten commits interleaved with `#1091`'s actual work — no `--onto` boundary can separate them. Verified 2026-08-22 (`agent-docs/m1/step-01-execution-report.md` §3.1).
- **A full-tree merge compiles but does not pass.** Merging produces a tree holding both proof surfaces (`#1091`'s split `ReservationProofs.sol` plus `#1090`+fold's mechanics kept inside `Reservation.sol`) — 65/121 reservation tests pass (111/121 on the untouched control), and the merged `Reservation` library is 29,768 B, 5,192 B over the EIP-170 limit, because holding both surfaces is what causes the overflow — neither parent alone exceeds it. Verified 2026-08-22 (same report, §3.3, §5).
- **The two trees are not old/new of one design.** Only 6 of 23 `Reservation.sol` functions are shared; retry model, veto keying, and the wallet-gate on redemption request all differ structurally, not incrementally (same report, §3.3 table). Reconciling that would be design work with no defined stopping point, not conflict resolution — and it is moot besides, because the design question is already answered: #1091's model is what every downstream spec doc describes.
- Porting individual fixes needs #1102's 30 findings enumerated once (from its PR body — `gh pr view 1102 --json body`, itemized in full) and checked against m1's own scope. Done 2026-08-22: roughly 5-6 of the 30 are m1-relevant (dust floor + cumulative re-anchor fee cap, the two governance-parameter exposure/validation fixes, the `ReservationParametersUpdated` event fix, the `BridgeState.Storage.__gap` correction, optionally the `_outpointKey` helper). The remaining ~20+ are redemption/dissolution/veto mechanics `roadmap.md` §0.7 already puts out of m1 scope. See `agent-docs/m1/step-02-port-1102-fixes.md` for the itemized list and disposition.
- **Independent corroboration, 2026-08-23** (reached by a different route than
  the fold-history-rewrite argument above): `#1091`'s current GitHub head
  (`f32c8cf3`) is `mergeable: CONFLICTING` against its own base and has **0
  check-runs** — the branch has 5 commits / 8 files / +713 -27 of untested
  delta on top of its last-green SHA (`a114ed7a`, 2026-08-11), including two
  real contract-behavior changes. Not itself a reason for the recommendation
  above (m1 already extracts `#1093` unmerged, not `#1091` raw), but it is
  further evidence against ever proposing a rebase or merge of this stack.

**Corrected 2026-08-22.** This section previously recommended sequentially rebasing `#1091` onto `#1090`, then `#1092` onto the rebased `#1091`, calling the rebase "mechanical (conflict resolution)" and validatable "by running the existing test suite... on the rebased branches". Executed and found false on both counts: the rebase does not run at all (history rewrite, not conflicts — first bullet above), and the merge fallback that does run fails 56 of 121 reservation tests and overflows EIP-170 (second and third bullets). The recommendation above replaces it.

## 8. The keep-core Side (#4238)

PR #4238 is a types-and-assembly foundation that already carries the two-phase
nonce constructs; what it lacks is the executor (`milestone-inventory.md` C-8).

**Recommendation**: **build on #4238**, do not supersede it (`D-25`).

**Why**:
- Its four proposal structs and the chain interface already carry `RequestNonce`,
  and action records are read by generation (`GetReservationAction(key, nonce)`).
  Those are the two-phase constructs, so there is no superseded design to
  replace — only a missing executor to add (`inventory/keep-core.md` §1).
- The pause flags m1 needs are Solidity in `ReservationVault`, which lives in
  tbtc-v2. They were never #4238's to carry, so their absence is not a reason to
  supersede a Go PR.

**Scope note**: variant B drops the dissolution executor from the Go work, worth
roughly 300-500 production lines.

**Corrected 2026-08-21.** This section previously read "PR #4238 implements the
superseded single-phase design (write-once custody term)" and recommended
**Supersede**, on the reasoning that "m1 needs nothing from #4238 beyond
historical reference". Correction C-8 refutes the premise, and D-25 records the
opposite disposition; the two documents asserted contradictory recommendations
until this rewrite. Three of the five original "Why" bullets restated the
same repo-location argument.

## 9. Ordered Action List

1. **Decision**: Adopt the epic-branch flat-PR strategy (reject force-push, merge-then-delete, archive copy, tag/draft preservation).
2. **Create one epic branch per repo** (a keep-core PR cannot target a branch in tbtc-v2):
   - tbtc-v2: `git checkout -b milestone/utxo-reservation-m1 main`
   - keep-core: `git checkout -b milestone/utxo-reservation-m1 main` (or reuse the existing `reservations-epic`, to which `#4238` is already retargeted)
3. **Port `#1102`'s m1-relevant fixes onto `#1091`-`#1096` — do not rebase or merge the trees** (§7; verified 2026-08-22 that both mechanics fail, see `agent-docs/m1/step-01-execution-report.md`):
   - `#1091`-`#1096` extract **unmerged, at their own tips** — this is the canonical design (§7) and needs no rebase.
   - Read `#1102`'s full PR body (`gh pr view 1102 --json body`) and check each of its ~30 findings against m1 scope (create/custody/re-anchor only, `roadmap.md` §0.7). Roughly 5-6 are m1-relevant: the dust floor + cumulative re-anchor fee cap (`maxCumulativeReanchorFee`/`cumulativeReanchorFee`, already the subject of `pr-review-followups.md` items 5/7), the `reservationDissolutionTxMaxFee`/`maxCumulativeReanchorFee` exposure in `reservationParameters()` plus their `updateReservationParameters` nonzero-validation, the `ReservationParametersUpdated` event-redeclaration fix, and the `BridgeState.Storage.__gap` correction (re-derive independently against m1's actual final field set — do not copy either tree's literal number; git's own auto-merge silently dropped a storage member during the abandoned merge attempt with no conflict marker, so verify by counting, not by trusting either side).
   - Port each confirmed-applicable fix directly onto the relevant `#1091`-`#1096` branch/file, verified against the reservation test suite. Do not import `#1090`'s superseded retry/veto/proof-surface mechanics along with it.
   - Full itemized disposition of all ~30 findings: `agent-docs/m1/step-02-port-1102-fixes.md`.
4. **Open PR #A**: `m1/storage-layout` -> epic branch  
   - Extract storage layout, enums, structs, governance parameters from #1088, #1090, #1091, #1092, #1093, #1094, and **#1096** (required for `ReservationAction.isPartial`, `retryCreditSourceNonce`, `BridgeState.Storage.reservationRetryCreditActionNonce` — §4.1), plus the m1-relevant `#1102` declarations (`ReservationRequest.cumulativeReanchorFee`, `maxCumulativeReanchorFee`, `reservationDissolutionTxMaxFee`) as **declare-only** fields and a re-derived `__gap`. **Built 2026-08-24**: 26 `Storage` fields consume 16 slots, `__gap` 48 → **32**, confirmed by the compiler — `Bridge`'s total storage footprint measured 4,128 bytes / 129 slots, byte-for-byte equal to the deployed TIP-109 artifact's, which is the check `assertStorageUpgradeSafe` alone does not make. Note `cumulativeReanchorFee` sits inside a mapping value and therefore costs no `Storage` slot; only the two governance parameters do, packing into one. An earlier 2026-08-24 edit wrongly struck all three fields; see §4.1 row for that history. Assert field-for-field against `milestone-inventory.md` §2.1 and §4.
5. **Open PR #B**: `m1/router-minimal` -> epic branch  
   - Extract minimal router from #1090 through #1094 (tip), not #1090 alone: 6 of the 8 named entry points do not exist at #1090 (see §4.1 evidence). `ReservationRouter.sol` is not touched by #1102, so no rebase/fold gap applies here regardless of which of #1091-#1094 you read from.
6. **Open PR #C**: `m1/acceptance-core` -> epic branch  
   - Extract acceptance request/proof/settlement from **`#1093`** (unmerged — §7), verified via extraction provenance. `#1093` already carries the H-04 `mintedAmount` backing-model rework; it needs no fold content beyond step 3's ported items.
7. **Open PR #D**: `m1/reanchor-core` -> epic branch
   - Extract re-anchor request/proof/settlement from **`#1093`** (unmerged — §7). **Corrected 2026-08-23: do not port the `maxCumulativeReanchorFee` check.** Step 3 confirmed raw `#1093` has zero occurrences of `cumulativeReanchorFee` in `Reservation.sol`/`ReservationProofs.sol` (no `#1102`-equivalent check inside `submitReservationReanchorProof`); the follow-up decision accepted that as m1's exposure rather than porting the check (`pr-review-followups.md` items 5/7, lever 4), pinned by a characterization test already live at tbtc-v2 `#1104` (staged on `#1093` — carry it forward verbatim per §6, rather than re-deriving it). This does not touch step 4/PR #A's field declarations (`ReservationRequest.cumulativeReanchorFee`, `BridgeState.Storage.maxCumulativeReanchorFee` still ship, declare-only — storage completeness, `milestone-inventory.md` §1.3): PR #D only omits the check, not the fields.
8. **Open PR #E**: `m1/timeout-and-stranding` -> epic branch  
   - Extract timeout (`notifyReservationActionTimeout`) from `#1091` and stranding (`notifyStaleReservedDeposit`, `notifyReservationStranded`, `strandReservation`, `strandLateSettlementIfTargetWalletClosed`) from `#1094`. Neither function is in step 3's m1-relevant `#1102` list, so no fold content applies here.
9. **Open PR #F**: `m1/vault-pause-flags` -> epic branch  
   - Use tbtc-v2 #1088 as the base vault plus #1093's in-kind fee financing; add the pause-flag logic (initiation only — never the fee/settlement path).
10. **Open PR #G**: `m1/bridge-integration-seams` -> epic branch  
    - Implement Bridge integration seams: `Deposit.sol` reveal path, `Wallets.sol` lifecycle gates, `WalletProposalValidator.sol` (sweep-exclusion guard plus m1 proposal validators), `BridgeGovernance.sol` and `BridgeGovernanceParameters.sol` wiring, and `Bridge.sol`'s `isReservedDeposit`, `setReservationRouter` and `fallback`.
11. **Open PR #H** in `threshold-network/keep-core`: `m1/keep-core-client` -> the **keep-core** epic branch  
    - Extend `#4238` (build on, not supersede — §8: its four proposal structs and chain interface already carry the two-phase nonce constructs) with the missing executor for acceptance and re-anchor, plus regenerated ABI bindings.
12. **Review and merge** each PR in order A-H onto the epic branch.
13. **Final verification**: Run full reservation test suite on the epic branch to confirm m1 behavior.
14. **Ready for milestone**: The epic branch now contains the complete m1 implementation; open a final PR from `milestone/utxo-reservation-m1` to `main` when desired.

**What is given up**:
- The ability to deliver m1 by simply merging the existing stacked PRs (they are superseded by the rewrite).
- The perception of continuity in PR numbers (new PRs replace the old ones for active development).
- Some historical git metadata (the original commits remain accessible via tags or the old branches, but are not part of the m1 history).

## 10. Open Questions

**Resolved by this document (kept here as a closure note, not re-litigated elsewhere):**
- Epic branch source: `main`, not `reservations-epic` — decided at step 2 (line 267), avoiding the unrelated security fix #1098.
- m1 PR naming convention: `m1/...`, used throughout §4.1/§4.2's tables and §9's action list; `milestone/...` is reserved for the epic branch name itself, a different thing.

**Still open:**
- How should the extraction provenance links (§6) remain stable if the original PR is later closed or a comment is resolved? §3 shows closed/merged PRs and their diffs survive branch deletion (which substantially covers "closed"), but does not test comment-resolution behavior specifically — resolved threads are a separate GitHub UI state, not something §3's git-based measurement observed.

## 11. Provenance

**Verified**:
- Stacked PR structure and branch bases: `inventory/pr-map.md` (§2)
- #1102 fold reach: `inventory/pr-map.md` (§3)
- Per-PR diffstat and attribution: `inventory/pr-map.md` (§1)
- Data model fields, enums, governance: `inventory/data-model.md`
- Action lifecycle and proof details: `inventory/proofs.md`
- Router entry points and delegatecall invariants: `inventory/router.md`
- Milestone 1 scope and decisions: `roadmap.md` (§0.1-0.8, §1.1-1.5)
- Reservation test inventory: `inventory/router.md` (§5)
- Pre-established facts about vault upgradeability and pause gates: `roadmap.md` (§0.7)

**Taken from another document**:
- Extraction verdict per PR: `inventory/pr-map.md` (§6)
- Extraction hazards: `inventory/pr-map.md` (§5)
- Decision on #1092 being structural: `roadmap.md` (§0.1)
- Vault-side upgradeability argument: `roadmap.md` (§0.7)
- Four delegatecall invariants being test-asserted: `inventory/router.md` (§2)
- Storage-completeness rule sharpening: `inventory/data-model.md` (§1)
- Timeout path reading redemption fields: `inventory/data-model.md` (§2)
- Enum positional immutability: `inventory/data-model.md` (§3)
- `unwindPendingAction` handling: `inventory/data-model.md` (§2)
- Global invariant on exit: `roadmap.md` (§0.3)
- Dissolution as permissionless slashing vector: `roadmap.md` (§0.6)
- Minimal router cost argument: `inventory/router.md` (§3)
- Keep-core #4238's actual shape: `milestone-inventory.md` C-8 and `inventory/keep-core.md` §1 (two-phase type layer, no executor).

**Correction**: On 2026-08-21, fixed erroneous claim that ReservationVault comes from keep-core #4238. ReservationVault is actually a tbtc-v2 Solidity contract from PR #1088, while keep-core #4238 is a Go client in a different repository.