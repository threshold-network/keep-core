# UTXO Reservations — Epic Branch, Review, and Merge Plan

Status: ACTIVE — branch/retarget setup executed 2026-08-19. Review and merge
phases below are not yet started. Companion doc:
`reservations-feature-spec.md` (the reverse-engineered spec and
gap analysis this plan assumes).

## 0. What was just set up (done, 2026-08-19)

- Created `reservations-epic` in both repos, cut from `main` at the time:
  - `threshold-network/tbtc-v2`: `502cd3982b5b2dc3ff0e1e6085a24502f78cfe26`
  - `threshold-network/keep-core`: `a7ac8989b662d51d8a94fa28f1ac226058d5d6cc`
- **tbtc-v2**: the 8 reservation PRs are tracked as a native GitHub Stack
  (stack `#1101`, `github.com/threshold-network/tbtc-v2/stacks/1101`). Only
  the stack's bottom PR needed retargeting — GitHub stacks treat every PR in
  the chain as targeting the stack's trunk for checks/reviews/CODEOWNERS
  regardless of which sibling branch it literally points at, so retargeting
  the root moves the whole stack:
  - `#1088` (root) base: `main` -> `reservations-epic`
  - `#1090`-`#1096` bases: unchanged (`feat/utxo-reservation-*` /
    `docs/utxo-reservation-release` sibling branches) — GitHub evaluates
    their checks/reviews against `reservations-epic` now automatically.
  - Mechanically this required the `gh-stack` extension (`gh extension
    install github/gh-stack`), because plain `gh pr edit --base` is blocked
    ("Cannot change the base branch because the pull request is part of a
    stack") once GitHub recognizes a PR chain as a stack. Sequence used:
    `gh stack unstack 1097` (dissolve — safe, no PR in the stack was merged
    or merge-queued) -> `gh pr edit 1088 --base reservations-epic` -> `gh
    stack link --base reservations-epic 1088 1090 1091 1092 1093 1094 1095
    1096` (re-create the stack rooted at the new trunk). `gh stack link`
    without an explicit `--base` silently resets the root to the repo
    default branch — always pass `--base reservations-epic` explicitly if
    this stack needs re-linking again (e.g. after inserting/removing a
    layer with `gh stack modify`).
- **keep-core**: `#4238` is a standalone (non-stacked) PR — retargeted
  directly with `gh pr edit 4238 --base reservations-epic`. No stack
  tooling needed.
- Net effect: none of the 9 PRs' diffs, commits, or review state changed.
  Branch protection, required status checks, and required reviews on `main`
  no longer gate these PRs — they gate against `reservations-epic` instead,
  which currently has no protection rules of its own (§2 covers whether to
  add any).

## 1. Why an epic branch (rationale, for reviewers who ask)

- The design has a **hard deployment-sequencing constraint** (spec §11):
  the full storage layout (all 8 tbtc-v2 PRs) must land as one coordinated
  release — no live `ReservationAction` record may ever exist on an
  intermediate layout, because there's no migration path for one. Merging
  each PR straight to `main` one at a time would put partially-landed
  reservation storage on `main` for the duration of the stack's review,
  visible to every other `main`-based branch and release in the meantime.
- An epic branch isolates that partial-landing window: `main` stays exactly
  as it is today until the entire reservation feature (Solidity stack +
  its keep-core counterpart) is ready to land together, then one
  fast-forward merge (or a single PR) brings it all in at once.
- It also gives the keep-core follow-up work (§4) a stable integration
  target that already has the intended final contract shape, without
  waiting on `tbtc-v2` `main` to publish an npm package mid-review.

## 2. Branch protection on `reservations-epic` (decide before review starts)

`reservations-epic` currently has **no branch protection** — anyone with
push access can force-push or merge directly into it. Recommended, matching
`main`'s existing posture in both repos:
- Require the same status checks that gate `main` today
  (`contracts-build-and-test`, `contracts-slither`, `contracts-format` for
  tbtc-v2; `client-build-test-publish`, `client-lint`, `client-vet`,
  `contracts-slither` for keep-core).
- Require at least one review per PR (already true for stack members via
  the stack-base evaluation in §0).
- Do **not** enable auto-merge or a merge queue for the stack — GitHub
  Stacked PRs don't support either yet (confirmed via the gh-stack docs),
  so every merge in the chain is a manual, explicit action regardless.

This is a repo-admin action (branch protection rule creation), not
something to script blind — flag to a repo admin before review starts if
the team wants it enforced.

## 3. Review plan — tbtc-v2 stack (`#1101`: 1088 -> 1090 -> 1091 -> 1092 -> 1093 -> 1094 -> 1095 -> 1096)

GitHub stack semantics (verified in gh-stack's own FAQ) mean **every PR
below the one being reviewed must also pass** before that PR is mergeable —
review still has to happen bottom-up in practice even though GitHub doesn't
force reviewers to look at PRs in order.

| Step | PR | Review focus | Gate before moving on |
|---|---|---|---|
| 1 | #1088 | Core reservation data model, `ReservationVault`, original single-phase mechanics (superseded by later PRs but still the storage foundation) | Approved + CI green |
| 2 | #1090 | Router delegatecall correctness: storage parity, selector disjointness, no-standalone-authority — these are the three invariants that make the EIP-170 workaround safe, verify the tests actually assert them, not just describe them | Approved + CI green |
| 3 | #1091 | The two-phase state machine itself — this is the highest-value review target (closes C-01, H-01/02/03/05/07, M-01/02/03). Confirm snapshot-at-request is exhaustive (no field read live at proof time that should've been snapshotted) | Approved + CI green |
| 4 | #1092 | Renewal window arithmetic (`window < term` non-stacking proof), dissolution-eligibility snapshotting | Approved + CI green |
| 5 | #1093 | Backing invariant (claim == anchor across every action), in-kind fee debt accounting | Approved + CI green |
| 6 | #1094 | Guards: designated-wallet binding, pending-deposit/vault-migration guard, stranding | Approved + CI green |
| 7 | #1095 | Docs+tests only — lower bar, but confirm the runbook/frozen-spec numbers in it match what governance actually signs off (§6) | Approved + CI green |
| 8 | #1096 | Partial redemption — newest, most likely to have interaction bugs with retry-credit/late-settlement logic from #1091. Give this the second-highest scrutiny after #1091 | Approved + CI green |

**Cross-cutting review pass (after all 8 individually approved, before any
merge):** re-read the full diff top-to-bottom as one unit (`git diff
reservations-epic...feat/utxo-reservation-partial-redemption` once all 8
are approved) — stacked review catches per-PR issues but not
interactions that only show up across the whole feature. This is the
external-audit-equivalent gate the runbook itself hasn't passed yet (spec
§16) — do not treat individually-approved PRs as equivalent to this.

## 4. keep-core (`#4238`) — do not review this PR alone as "done"

Per the spec's gap analysis (§16), `#4238` implements the **original
single-phase design**. Two options, pick one explicitly before spending
review time on it:

- **Option A (recommended): hold #4238's review** until a follow-up PR
  reworks it for the two-phase ABI (nonce-carrying proposals,
  watchtower-delay-respecting executor, partial-redemption awareness — the
  concrete list is in spec §13). Reviewing #4238 as-is now means re-
  reviewing most of it again once that follow-up lands. Track the
  follow-up as a new PR based on `reservations-epic` (not on `#4238`'s
  branch) once the tbtc-v2 ABI is stable enough to bind against.
- **Option B**: review #4238 now as "foundational types only, not
  wired to the shipped design" and explicitly gate its merge on the
  follow-up PR merging first (or landing in the same epic-branch merge).
  Only choose this if the team wants the Go types/enums locked in early to
  unblock other work that depends on them.

Either way: **`#4238` alone must never merge to `main` believing it's
feature-complete.** Add a checklist item to whichever PR does the
epic-branch -> `main` merge (§6) confirming the two-phase keep-core rework
exists and is included.

## 5. Audit gate (blocking, not yet started)

Per spec §16, no external audit has happened. Before requesting one:
- [ ] All 8 tbtc-v2 PRs individually approved (§3) and merged into
      `reservations-epic`.
- [ ] `docs/utxo-reservation-review-findings.md` — referenced by the
      runbook but absent from the tree (confirmed missing on 3 branches,
      spec §15) — either gets written (consolidating the H/M/L findings
      table this spec reconstructed in §12) or the runbook's reference to
      it is removed. An auditor will ask for it; don't let this be
      discovered mid-audit.
- [ ] Two governance parameters with no proposed value yet
      (`reservationTxMaxFee`, `feeReserveTarget`) get provisional numbers —
      an auditor can't assess a parameter that doesn't exist.
- [ ] Runbook's own pre-audit checklist (spec §11) fully checked: Slither
      clean (already true per CI), fork dry-run of the full activation
      sequence (not yet done), storage-layout diff reviewed as
      append-only end-to-end across all 8 PRs combined (each PR's own
      parity test only checks its own increment).
- [ ] Engage the external auditor against `reservations-epic`, not against
      individual PR branches — the audit needs the fully-assembled feature.

## 6. Landing `reservations-epic` into `main`

Do this only after §3 (tbtc-v2 stack merged into `reservations-epic`), §4
resolved (keep-core two-phase rework exists), and §5 (audit) passed.

1. Rebase `reservations-epic` on current `main` (main will have moved during
   the review/audit window) and re-run the full storage-layout parity test
   against that rebased tip.
2. Re-run the runbook's pre-audit checklist one final time on the rebased
   tip (a rebase can silently reorder something the append-only discipline
   depends on, however unlikely — cheap to re-check, expensive to miss).
3. Open a single PR `reservations-epic -> main` per repo (or fast-forward
   merge if the org's branch-protection rules allow it) — this is the
   deploy-inert-then-activate boundary from the runbook (spec §11): landing
   on `main` is not the same as activating the feature. Activation is a
   separate, later, governance-gated sequence run against whatever's
   deployed from `main`.
4. Do **not** delete `reservations-epic` immediately after merging — keep
   it until the first mainnet activation transaction (`setVaultStatus`)
   confirms, in case a hotfix needs the same integration point before
   `main` is deployed.

## 7. Immediate next actions (unblocked today)

- [ ] Decide keep-core Option A vs B (§4) — this determines whether
      anyone should spend review time on `#4238` right now.
- [ ] Decide on `reservations-epic` branch protection (§2) — currently
      unprotected in both repos.
- [ ] Start bottom-up review of the tbtc-v2 stack at `#1088` (§3).
- [ ] Assign someone to write the missing
      `docs/utxo-reservation-review-findings.md` (§5) — this is on the
      critical path to the audit gate and nobody currently owns it.
