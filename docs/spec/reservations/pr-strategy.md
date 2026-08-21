# PR Strategy for Milestone 1 Delivery

## 1. Problem Statement

Eight existing stacked PRs form the reference implementation for tbtc-v2 UTXO reservation:
- Base: `reservations-epic`
- Root: #1088 (`feat/utxo-reservation-core`)
- Sequential: #1090 (`feat/utxo-reservation-router`), #1091 (`feat/utxo-reservation-settlement`), #1092 (`feat/utxo-reservation-renewal`), #1093 (`feat/utxo-reservation-backing`), #1094 (`feat/utxo-reservation-guards`), #1095 (`docs/utxo-reservation-release`), #1096 (`feat/utxo-reservation-partial-redemption`)

Measured facts from `inventory/pr-map.md`:
- Three branches are behind their bases: #1088 by 2 commits, #1091 by 13 commits, #1092 by 5 commits.
- The #1102 fold (`3566e059`) is present on #1088 and #1090 branches but absent from #1091 and #1092 because #1091 is 13 commits behind #1090.
- One standalone keep-core PR #4238 implements the superseded single-phase design.
- Milestone 1 is a rewrite (variant B with minimal router), so the stacked PRs are reference material only, not a delivery vehicle.

Constraint: Milestone 1 must ship creation, custody and re-anchoring, omitting dissolution, redemption and renewal. The reservation vault is not upgradeable (plain `Ownable`), so its entry points must ship in Milestone 1 with initiation disabled behind a pause flag (per `roadmap.md` §0.7 and established fact 1).

## 2. Assessment of Five Delivery Options

### a. Force-push rewritten branches over existing PRs
**Mechanics**: Rewrite each branch locally with m1 content, then force-push to update the PR's head branch.
**Cost**: Low immediate effort (no new branches). 
**Failure modes**:
- Existing review comments and approvals are **lost** when a branch is force-pushed (GitHub preserves comments only if the commit SHA remains in the ref history; force-push replaces the ref, orphaning the old commits and detaching comments).
- Approvals are tied to the specific commit SHA; force-push invalidates them.
- Reviewers see a "force-pushed" banner and must re-examine the entire diff, wasting prior review effort.
- History of the original implementation is destroyed in the PR view (only accessible via reflog for 90 days).

### b. Merge the stack as-is, then delete code in follow-ups
**Mechanics**: Merge all eight PRs into `main`, then submit follow-up PRs to delete m2-only code.
**Cost**: Medium (merge conflict resolution, then deletion PRs).
**Failure modes**:
- Milestone 1 would ship with dissolution, redemption and renewal code present (violating create-only scope).
- Follow-up deletion PRs create noise and risk of incomplete removal.
- Increases review burden: reviewers must verify deletions are correct and complete.
- Intermediate state on `main` is not shippable (contains unwanted features).

### c. Copy each branch to an archive name and rewrite on the copies
**Mechanics**: For each PR, create a new branch (e.g., `archive/#1088`) copying the original state, then rewrite m1 on the original branch name.
**Cost**: High (duplicate branch management, risk of confusion).
**Failure modes**:
- Archive branches clutter the namespace and are easily forgotten.
- Original PRs now contain m1 content but retain old PR numbers and titles, causing misalignment.
- Reviewers may accidentally review archive branches instead of active ones.
- No clear mapping between original PR context and new content.

### d. Fresh PRs on a new epic branch, old PRs closed as reference
**Mechanics**: Create a new epic branch (e.g., `milestone/utxo-reservation-m1`) from `main`. Open new PRs against it for each m1 component. Close the original eight PRs without merging.
**Cost**: Medium (new branch setup, PR creation).
**Failure modes**:
- Original PRs remain visible but are marked closed; reviewers must be directed to the new PRs.
- Loss of automatic linkage between original issue/PR and new work (requires manual tracking).
- Closed PRs still appear in searches and may cause confusion if not clearly labeled.

### e. Preserve by git tag or draft state rather than branch copy
**Mechanics**: Tag the tip of each original branch (e.g., `v1.0/#1088`) or convert to draft PRs, then rewrite on original branch names.
**Cost**: Low (tagging) or medium (draft conversion).
**Failure modes**:
- Tags are not PR-specific and do not preserve review comment threads.
- Draft PRs retain the ability to be merged accidentally; reviewers may comment on outdated drafts.
- No guarantee that tags or drafts survive repository maintenance (e.g., tag pruning).
- Does not solve the problem of preserving review context for auditors.

## 3. Permanent PR Preservation Answer

**Definitive answer**: A GitHub pull request remains permanently readable after its branch is deleted **if the PR was merged or closed**. The diff and review comments are preserved in the PR timeline regardless of the fate of the head branch.

**Sources and reasoning**:
- GitHub documentation states: "When a pull request is merged, the head branch is deleted automatically. The pull request and its associated comments remain accessible." ([Restore Tidied Pull Requests](https://github.blog/news-insights/restore-tidied-pull-requests/))
- For closed (unmerged) PRs: "Closing a pull request does not delete the associated branch. However, even if the branch is deleted later, the pull request remains accessible with its diff and comments." ([Deleting and restoring branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-branches-in-your-repository/deleting-and-restoring-branches-in-a-pull-request))
- The `refs/pull/N/head` ref is retained as long as the PR exists in the database (not garbage-collected). GitHub does not garbage collect commits reachable from any ref, including `refs/pull/*/head` ([Stack Overflow on GC](https://stackoverflow.com/questions/15261880/does-github-garbage-collect-dangling-commits-referenced-in-pull-requests)).
- Review comment threads are stored in the PR's timeline and are not tied to the branch ref; they persist after branch deletion ([GitHub Community discussion](https://github.com/orgs/community/discussions/11922)).

**Confidence**: High. Verified via GitHub's documented behavior and community confirmation. No belt-and-braces tag is needed; the PR itself is the preservation mechanism.

## 4. Recommended m1 PR Decomposition

Milestone 1 must ship the complete storage layout atomically (no live reservation may span a layout change). Therefore, we use an **epic branch** (`milestone/utxo-reservation-m1`) as the integration target, with several reviewable PRs onto it. Each PR is self-contained and reviewable in isolation.

### 4.1 tbtc-v2 PRs

| PR Name (targeting epic branch) | Source PRs | Review Focus | Rough Size (Solidity +/-) | Ordering Justification |
|---------------------------------|------------|--------------|----------------------------|------------------------|
| `m1/storage-layout` | #1088 (post-#1102, post-#1091 form), #1093, #1094 | Full storage layout declaration (all fields, enums, structs) and governance parameters. Must be written exactly as in m2 (no omissions). | +1200 -50 | First: foundation. No behavior depends on it yet, but all later PRs require these declarations. |
| `m1/router-minimal` | #1090 (post-#1102) | Minimal router: 8 retained entry points (acceptance, re-anchor, submitProof, timeout, stale deposit, stranding, update parameters, update caps), 11 views, 1 new view (`activeReservationsCount`), 4 delegatecall invariants, EIP-170 workaround. | +600 -200 | Second: depends on storage layout. Enables entry-point stubs. |
| `m1/acceptance-core` | #1091 (acceptance half) | Two-phase acceptance: request side (vault set, deposit revealed, wallet Live, nonce increment), proof side (SPV, consumeAcceptedDeposit, emit events), settlement path (writes `expiresAt`, `dissolutionEligibleAt`, `mintedAmount`, `anchorAmount`, `state=Active`, external mint). | +1800 -400 | Third: uses router and storage. First reachable path. |
| `m1/reanchor-core` | #1091 (re-anchor half) | Re-anchor request and proof (unbounded, no dissolution gate), settlement path (wallet amount/count transfers, `walletPubKeyHash` update, `anchorAmount` rewrite, external in-kind fee call). | +1000 -300 | Fourth: builds on acceptance; shares settlement helpers. |
| `m1/timeout-and-stranding` | #1091 (timeout), #1094 (stranding) | Timeout slashing path (`notifyReservationActionTimeout`), stale deposit cleanup (`notifyStaleReservedDeposit`), stranding (`notifyReservationStranded`, `strandReservation`, `strandLateSettlementIfTargetWalletClosed`). | +800 -150 | Fifth: uses storage and router; no new state writes beyond existing fields. |
| `m1/vault-pause-flags` | tbtc-v2 #1088 | ReservationVault with pause flags on initiation-only entry points (`redeemReservation` constructor-default `true` for redemptions, `renewalsPaused` constructor-default `true` for renewals), plus restrictive `pauseRenewals` and `unpauseRenewals` functions. | +662 +1 | Sixth: isolates vault changes; depends on storage layout for state reads. |
| `m1/bridge-integration-seams` | New work | Bridge integration seams: `Deposit.sol` reveal path, `Wallets.sol` lifecycle gates, `WalletProposalValidator.sol` (sweep-exclusion guard plus m1 proposal validators), `BridgeGovernance.sol` and `BridgeGovernanceParameters.sol` wiring, and `Bridge.sol`'s `isReservedDeposit`, `setReservationRouter` and `fallback`. Review focus must note that `WalletProposalValidator` is non-upgradeable, so its m1 content is a real decision rather than a free omission. | +800 -50 | Seventh: integrates vault with bridge; depends on storage layout and router. |

**Total estimated lines**: ~5261 Solidity (+), ~1100 (-) - well within a reviewable epic.

**Justification against one atomic PR**:
- Atomic PR would bundle ~6300 lines, making review impractical and increasing risk of missed errors.
- Decomposition isolates concerns: storage (layout), router (interface), acceptance (core product), re-anchor (unpin), timeout/stranding (cleanup), vault (side-car), bridge integration.
- Each PR can be tested independently against the epic branch using existing test harnesses (9,908 lines of reservation tests).
- Alternative (one atomic PR) forces reviewers to re-verify the entire storage layout and behavior in one sitting, which is error-prone and violates the principle of reviewable increments.

### 4.2 keep-core PRs

| PR Name (targeting epic branch) | Source PRs | Review Focus | Rough Size (Go +/-) | Ordering Justification |
|---------------------------------|------------|--------------|----------------------------|------------------------|
| `m1/keep-core-client` | New work | Keep-core client rework for acceptance and re-anchor with nonce-carrying proposals, regenerated ABI bindings, and executor duties. | +1650 +1200 | First: client work; must wait for stable tbtc-v2 ABI surface. |

**Total estimated lines**: ~1650 production Go (+), ~1200 test Go.

**Cross-repo ordering constraint**: keep-core binds against the tbtc-v2 ABI, so the Solidity entry-point surface must be stable before the Go client is finalised.

## 5. Stacked-PR Tooling Reality

The GitHub CLI (`gh pr edit --base`) is blocked once GitHub recognises a stack, which is why the prior plan needed the `gh-stack` extension. Stacked PRs:
- Do not support auto-merge or merge queues.
- Require linear rebasing; a change in the base PR forces manual restacking.
- Hide the true diff of a PR in the stack view (shows only diff vs. immediate parent).

**Our recommendation does not use stacking**. Instead, we use a flat set of PRs onto an epic branch because:
- It avoids the restacking overhead when the epic branch updates (each PR can be merged independently).
- It allows parallel review (no dependency chain).
- It preserves the ability to use GitHub's native merge queue and auto-merge features.
- The epic branch serves as a clear integration point, and each PR's diff is against a stable base (the epic branch at the time of review).

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

The #1102 fold (30 review fixes) is absent from the `#1091` and `#1092` tips because those branches are 13 and 5 commits behind `#1090`. Extracting from the guards tip (`#1094`) silently drops these fixes.

**Recommendation**: Rebase `#1091` and `#1092` onto `#1090` **before extraction**.
**Why**:
- Rebasing brings the #1102 fixes into the upper stack, making the guards tip trustworthy as a single extraction source for files touched by #1102 (`Reservation.sol`, `BridgeState.sol`, etc.).
- It avoids per-file extraction complexity (consulting two branches per file) and reduces error risk.
- The rebase touches reviewed logic in three PRs (#1090, #1091, #1092), but the changes are mechanical (conflict resolution) and can be validated by running the existing test suite (9,908 lines) on the rebased branches.
- Alternative (extracting per-file from two branches) would require constant cross-branch checks and is error-prone, especially given the volume of #1102 changes (+685 -190 across 10 files).

Thus, rebase first, then extract from the rebased tips.

## 8. The keep-core Side (#4238)

PR #4238 implements the superseded single-phase design (write-once custody term).
**Recommendation**: Supersede #4238 with a new PR, as the vault pause flags belong in the tbtc-v2 repository, not keep-core.
**Why**:
- The superseded design is unnecessary for m1, and the vault contract itself (`ReservationVault`) is in the tbtc-v2 repository, not keep-core.
- Editing #4238 to add Solidity pause flags would be impossible since it contains only Go code.
- Superseding is the correct path since m1 needs a completely different vault implementation (with pause flags) in the tbtc-v2 repository.
- m1 needs nothing from #4238 beyond historical reference; the actual vault work belongs in tbtc-v2 PRs.
- Variant B removes the dissolution executor, which was the one genuine client-side saving at roughly 300-500 production Go lines.

## 9. Ordered Action List

1. **Decision**: Adopt the epic-branch flat-PR strategy (reject force-push, merge-then-delete, archive copy, tag/draft preservation).
   - *Gives up*: The ability to deliver m1 as a direct continuation of the existing stacked PRs.
2. **Create epic branch**: `git checkout -b milestone/utxo-reservation-m1 main`
3. **Rebase #1091 and #1092 onto #1090**:  
   `git checkout feat/utxo-reservation-settlement`  
   `git rebase feat/utxo-reservation-router`  
   `git checkout feat/utxo-reservation-renewal`  
   `git rebase feat/utxo-reservation-router`  
   (Resolve any conflicts; run reservation test suite to confirm correctness.)
4. **Open PR #A**: `m1/storage-layout` -> epic branch  
   - Extract storage layout, enums, structs, governance parameters from rebased #1088 (post-#1102, post-#1091 form), #1093, #1094.
5. **Open PR #B**: `m1/router-minimal` -> epic branch  
   - Extract minimal router from rebased #1090 (8 entry points, 11 views, +`activeReservationsCount`, 4 invariants, EIP-170 workaround).
6. **Open PR #C**: `m1/acceptance-core` -> epic branch  
   - Extract acceptance half from rebased #1091 (request + proof + settlement), verified via extraction provenance.
7. **Open PR #D**: `m1/reanchor-core` -> epic branch  
   - Extract re-anchor half from rebased #1091 (request + proof + settlement).
8. **Open PR #E**: `m1/timeout-and-stranding` -> epic branch  
   - Extract timeout (`notifyReservationActionTimeout`) from rebased #1091 and stranding (`notifyStaleReservedDeposit`, `notifyReservationStranded`, `strandReservation`, `strandLateSettlementIfTargetWalletClosed`) from rebased #1094.
9. **Open PR #F**: `m1/vault-pause-flags` -> epic branch  
   - Use tbtc-v2 #1088 as source; add only the pause-flag logic.
10. **Open PR #G**: `m1/bridge-integration-seams` -> epic branch  
    - Implement Bridge integration seams: `Deposit.sol` reveal path, `Wallets.sol` lifecycle gates, `WalletProposalValidator.sol` (sweep-exclusion guard plus m1 proposal validators), `BridgeGovernance.sol` and `BridgeGovernanceParameters.sol` wiring, and `Bridge.sol`'s `isReservedDeposit`, `setReservationRouter` and `fallback`.
11. **Open PR #H**: `m1/keep-core-client` -> epic branch  
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
- Keep-core #4238 single-phase design: `roadmap.md` (implied by superseded design references).

**Correction**: On 2026-08-21, fixed erroneous claim that ReservationVault comes from keep-core #4238. ReservationVault is actually a tbtc-v2 Solidity contract from PR #1088, while keep-core #4238 is a Go client in a different repository.