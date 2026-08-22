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
| `m1/storage-layout` | #1088 (post-#1102), #1090, #1091, #1092, #1093, #1094, **#1096** | Full storage layout declaration (all fields, enums, structs) and governance parameters. Must be written exactly as in m2 (no omissions). **#1096 is required** for three declare-only fields that exist only on the partial branch — `ReservationAction.isPartial`, `retryCreditSourceNonce`, `BridgeState.Storage.reservationRetryCreditActionNonce` (`milestone-inventory.md` §4). **Also required post-fold**: `ReservationRequest.cumulativeReanchorFee`, `maxCumulativeReanchorFee` and `reservationDissolutionTxMaxFee`, which are on the core line only and are absent from `milestone-inventory.md` §2.1 (see its §1.3). Assert the layout field-for-field against §2.1 and §4 — `m1-b-implementation.md` §4.5 is a launch gate. | +1200 -50 | First: foundation. No behavior depends on it yet, but all later PRs require these declarations. |
| `m1/router-minimal` | #1090 (post-#1102) through **#1094** — verified 2026-08-21: only `submitReservationProof` and `updateReservationParameters` exist at #1090; `requestReservationAcceptance`/`requestReservationReanchor`/`notifyReservationActionTimeout` first appear at #1091, `updateReservationCaps` at #1093, `notifyStaleReservedDeposit`/`notifyReservationStranded` at #1094 (#1102 does not touch `ReservationRouter.sol`, so no fold gap here) | Minimal router: 8 retained entry points (acceptance, re-anchor, submitProof, timeout, stale deposit, stranding, update parameters, update caps), 11 views, 1 new view (`activeReservationsCount` — genuinely new, not present at any source tip), 4 delegatecall invariants, EIP-170 workaround. | +600 -200 | Second: depends on storage layout. Enables entry-point stubs. |
| `m1/acceptance-core` | #1091 (acceptance half) | Two-phase acceptance: request side (vault set, deposit revealed, wallet Live, nonce increment), proof side (SPV, consumeAcceptedDeposit, emit events), settlement path (writes `expiresAt`, `dissolutionEligibleAt`, `mintedAmount`, `anchorAmount`, `state=Active`, external mint). | +1800 -400 | Third: uses router and storage. First reachable path. |
| `m1/reanchor-core` | #1091 (re-anchor half) | Re-anchor request and proof (unbounded, no dissolution gate), settlement path (wallet amount/count transfers, `walletPubKeyHash` update, `anchorAmount` rewrite, external in-kind fee call). | +1000 -300 | Fourth: builds on acceptance; shares settlement helpers. |
| `m1/timeout-and-stranding` | #1091 (timeout), #1094 (stale deposit cleanup + stranding) | Timeout slashing path (`notifyReservationActionTimeout`), stranding group from #1094 — stale deposit cleanup (`notifyStaleReservedDeposit`) and stranding (`notifyReservationStranded`, `strandReservation`, `strandLateSettlementIfTargetWalletClosed`); `notifyStaleReservedDeposit` does not exist at #1091, only from #1094 on. | +800 -150 | Fifth: uses storage and router; no new state writes beyond existing fields. |
| `m1/vault-pause-flags` | tbtc-v2 #1088 (base vault) + **#1093** (in-kind fee financing) | ReservationVault with pause flags on initiation-only entry points (`redemptionsPaused` constructor-default `true` for redemptions, `renewalsPaused` constructor-default `true` for renewals), The fee-financing surface (`financeInKindFee` `:529`, `repayInKindFeeDebt` `:568`, `inKindFeeDebtSat`, `updateFeeReserveTarget` `:599`) must be **present and ungated** — re-anchor is B's only unpin and calls it on the settlement path (`m1-b-implementation.md` §3). Note `pauseRenewals`/`unpauseRenewals` already ship (`:409`, `:415`); what is new is the `redemptionsPaused` trio. Plus restrictive `pauseRenewals` and `unpauseRenewals` functions. | +662 −1 | Sixth: isolates vault changes; depends on storage layout for state reads. |
| `m1/bridge-integration-seams` | New work | Bridge integration seams: `Deposit.sol` reveal path, `Wallets.sol` lifecycle gates, `WalletProposalValidator.sol` (sweep-exclusion guard plus m1 proposal validators), `BridgeGovernance.sol` and `BridgeGovernanceParameters.sol` wiring, and `Bridge.sol`'s `isReservedDeposit`, `setReservationRouter` and `fallback`. Review focus must note that `WalletProposalValidator` is non-upgradeable, so its m1 content is a real decision rather than a free omission. | +800 -50 | Seventh: integrates vault with bridge; depends on storage layout and router. |

**Total estimated lines**: the `+` column above sums to **6,862**, against
`m1-variant-comparison.md` §3's **5,261** for B-with-router — 1,601 lines above
the audited scope. The `-` column sums to ~1,150. **Reconcile before opening PR
#A**: either the per-PR estimates are high or the milestone figure is low; they
have never been derived from each other. (Corrected 2026-08-21: this line
previously stated ~5261 as though it were this table's total, and a bullet below
cited a third unsourced figure of ~6300.)

**Justification against one atomic PR**:
- An atomic PR would bundle the whole 6,862-line `+` total, making review impractical and increasing the risk of missed errors.
- Decomposition isolates concerns: storage (layout), router (interface), acceptance (core product), re-anchor (unpin), timeout/stranding (cleanup), vault (side-car), bridge integration.
- Each PR can be tested independently against the epic branch using existing test harnesses (9,908 lines of reservation tests).
- Alternative (one atomic PR) forces reviewers to re-verify the entire storage layout and behavior in one sitting, which is error-prone and violates the principle of reviewable increments.

### 4.2 keep-core PRs

| PR Name (keep-core repo, targeting the keep-core epic branch) | Source PRs | Review Focus | Rough Size (Go +/-) | Ordering Justification |
|---------------------------------|------------|--------------|----------------------------|------------------------|
| `m1/keep-core-client` | New work | Keep-core client rework for acceptance and re-anchor with nonce-carrying proposals, regenerated ABI bindings, and executor duties. | ~1,100-1,400 +1,200 | First: client work; must wait for stable tbtc-v2 ABI surface. |

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

**Recommendation**: rebase **sequentially** before extraction — `#1091` onto `#1090`, then `#1092` onto the *rebased* `#1091`. Rebasing `#1092` directly onto `#1090` would replay `#1091`'s commits a second time on `#1092`'s branch, leaving two divergent copies of the settlement work and making `#1092`'s PR diff meaningless.
**Why**:
- Rebasing brings the #1102 fixes into the upper stack, making the guards tip trustworthy as a single extraction source for files touched by #1102 (`Reservation.sol`, `BridgeState.sol`, etc.).
- It avoids per-file extraction complexity (consulting two branches per file) and reduces error risk.
- The rebase touches reviewed logic in three PRs (#1090, #1091, #1092), but the changes are mechanical (conflict resolution) and can be validated by running the existing test suite (9,908 lines) on the rebased branches.
- Alternative (extracting per-file from two branches) would require constant cross-branch checks and is error-prone, especially given the volume of #1102 changes (+685 -190 across 10 files).

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
3. **Rebase #1091 and #1092 onto #1090**:  
   `pre_rebase_settlement_tip=$(git rev-parse feat/utxo-reservation-settlement)` (capture before rebasing — needed by the last command below)  
   `git checkout feat/utxo-reservation-settlement`  
   `git rebase feat/utxo-reservation-router`  
   `git checkout feat/utxo-reservation-renewal`  
   `git rebase --onto feat/utxo-reservation-settlement $pre_rebase_settlement_tip`  
   If the rebase conflicts, resolve it, then run the reservation test suite on each rebased branch. Do not extract until both suites pass.
4. **Open PR #A**: `m1/storage-layout` -> epic branch  
   - Extract storage layout, enums, structs, governance parameters from rebased #1088 (post-#1102), #1090, #1091, #1092, #1093, #1094, and **#1096** (required for `ReservationAction.isPartial`, `retryCreditSourceNonce`, `BridgeState.Storage.reservationRetryCreditActionNonce` — §4.1). **Gap**: step 3 above rebases only #1091 and #1092, not #1096 (which branches #1093->#1094->#1095->#1096). Per §7, #1093-#1096 inherit #1091's pre-rebase staleness regardless of being current with their own parent, so #1096 does not carry the #1102 fold either. Before extracting #1096's three fields, either rebase #1093-#1096 sequentially onto the rebased #1092, or verify none of the three fields sit in a file #1102 touched (`Reservation.sol`, `BridgeState.sol`, etc., per §7).
5. **Open PR #B**: `m1/router-minimal` -> epic branch  
   - Extract minimal router from #1090 through #1094 (tip), not #1090 alone: 6 of the 8 named entry points do not exist at #1090 (see §4.1 evidence). `ReservationRouter.sol` is not touched by #1102, so no rebase/fold gap applies here regardless of which of #1091-#1094 you read from.
6. **Open PR #C**: `m1/acceptance-core` -> epic branch  
   - Extract acceptance half from rebased #1091 (request + proof + settlement), verified via extraction provenance.
7. **Open PR #D**: `m1/reanchor-core` -> epic branch  
   - Extract re-anchor half from rebased #1091 (request + proof + settlement).
8. **Open PR #E**: `m1/timeout-and-stranding` -> epic branch  
   - Extract timeout (`notifyReservationActionTimeout`) from rebased #1091 and stranding (`notifyStaleReservedDeposit`, `notifyReservationStranded`, `strandReservation`, `strandLateSettlementIfTargetWalletClosed`) from rebased #1094.
9. **Open PR #F**: `m1/vault-pause-flags` -> epic branch  
   - Use tbtc-v2 #1088 as the base vault plus #1093's in-kind fee financing; add the pause-flag logic (initiation only — never the fee/settlement path).
10. **Open PR #G**: `m1/bridge-integration-seams` -> epic branch  
    - Implement Bridge integration seams: `Deposit.sol` reveal path, `Wallets.sol` lifecycle gates, `WalletProposalValidator.sol` (sweep-exclusion guard plus m1 proposal validators), `BridgeGovernance.sol` and `BridgeGovernanceParameters.sol` wiring, and `Bridge.sol`'s `isReservedDeposit`, `setReservationRouter` and `fallback`.
11. **Open PR #H** in `threshold-network/keep-core`: `m1/keep-core-client` -> the **keep-core** epic branch  
    - Implement keep-core client rework for acceptance and re-anchor with nonce-carrying proposals, regenerated ABI bindings, and executor duties.
12. **Review and merge** each PR in order A-H onto the epic branch.
13. **Final verification**: Run full reservation test suite on the epic branch to confirm m1 behavior.
14. **Ready for milestone**: The epic branch now contains the complete m1 implementation; open a final PR from `milestone/utxo-reservation-m1` to `main` when desired.

**What is given up**:
- The ability to deliver m1 by simply merging the existing stacked PRs (they are superseded by the rewrite).
- The perception of continuity in PR numbers (new PRs replace the old ones for active development).
- Some historical git metadata (the original commits remain accessible via tags or the old branches, but are not part of the m1 history).

## 10. Open Questions

- Should the epic branch be created from `main` or from `reservations-epic`? (Using `main` avoids carrying the unrelated security fix #1098, but loses the epic branch's isolation property.)
- What naming convention should be used for the m1 PRs to clearly indicate they target the epic branch? (e.g., `m1/...` vs `milestone/...`.)
- How should the extraction provenance links be formatted to remain stable if the original PR is later closed or the comment is resolved?

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