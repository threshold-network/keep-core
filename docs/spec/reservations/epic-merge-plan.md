# UTXO Reservations — Epic Branch, Review, and Merge Plan

Status: ACTIVE — branch/retarget setup executed 2026-08-19. Review and merge
phases below are not yet started. Companion doc:
`feature-spec.md` (the reverse-engineered spec and
gap analysis this plan assumes).

This document is the reference record of the eight-PR stack, kept because the m1 rewrite extracts from those PRs. See `pr-strategy.md` for how m1 actually ships and `roadmap.md` for scope.
As of 2026-08-21, milestone 1 is defined as **variant B**, an essentials-only rewrite of the reservation feature (see `roadmap.md` §1). Therefore, this document's original purpose — sequencing the review and merge of the eight tbtc-v2 PRs as a delivery plan for m1 — is superseded.

**What survives:**
The per-PR review findings, the invariant lists, the conflict/rebase state, and the dependency reasoning remain valid. The rewrite extracts from these PRs, so knowing which parts were reviewed and which are known-broken is essential.

**What does not survive:**
- The combined-parity audit item (superseded; see `m1-b-implementation.md` for the live audit gate for m1 B).
- The epic-branch-to-main landing procedure (superseded; see `m1-b-implementation.md` for the live landing procedure for m1 B).
- Any step whose sole purpose was landing all eight PRs onto a shared tip (the rewrite does not use these PRs as a delivery vehicle).
- The keep-core Option A versus Option B decision (superseded; see `m1-b-implementation.md` for the live keep-core plan for m1 B).

## What the variant B decision superseded (2026-08-21)

- The milestone scope and merge sequence (originally in the "Milestone scope" section) are superseded; they remain as reference for extraction but do not describe the m1 delivery path.
- The review plan's merge gates and sequencing are superseded; the review focus and dependency reasoning remain valid for extraction, but the gates no longer describe a path to m1 merge. This section remains as reference for what needs to be verified during the rewrite.
- The two-phase keep-core rework discussion is not part of m1; #4238 as-is represents the original single-phase design which is not shipped in m1 B. This section remains as reference for understanding the keep-core PR's intent.
- The audit gate and its checklist are superseded; the audit for m1 B will target the rewritten code, and the checklist items may require adjustment. This section remains as reference for the original plan's audit preparation.
- The landing procedure is not for m1 B; the epic branch may still be used as an integration base for the rewrite, but the steps and gates are superseded. This section remains as reference for the original landing process.
- #1096 is no longer the "one clean PR omission"; it is simply one of several unwritten m2 features, so its special status in the merge narrative is gone. [This repairs the sentence that was cut in half.]

## 0. What was just set up (done, 2026-08-19)
Note: This section describes the setup for the original eight-PR stack. It applies to the old stack, not necessarily to m1.
  - Mechanically this required the `gh-stack` extension (`gh extension
    install github/gh-stack`), because plain `gh pr edit --base` is blocked
    ("Cannot change the base branch because the pull request is part of a
    stack") once GitHub recognizes a PR chain as a stack. Sequence used:
    `gh stack unstack 1097` (dissolve — safe, no PR in the stack was merged
    or move-queued) -> `gh pr edit 1088 --base reservations-epic` -> `gh
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
  - **`#1102`** (`fix: #1088 review follow-ups`, based on `feat/utxo-reservation-core`,
    addressing 30 findings from #1088's review per `pr-review-followups.md`) is **not**
    included in the `gh stack link` command above and is not part of stack `#1101`. **It was
    merged on 2026-08-21** (`3566e059`) into `feat/utxo-reservation-core` — which is the
    head branch of `#1088`. Its merged state needs no retarget: because `#1088`'s stack-
    base evaluation runs against `reservations-epic` (per `gh-stack` semantics in §0), the
    fold is complete and the review surface is `#1088`'s tip. Confirm nothing in #1102 got
    rebased away when it merged before treating the review surface as complete.

### 0.1 Full reservation-PR inventory (verified 2026-08-21)

Complete re-sweep of both repos (`gh search prs` across `utxo`, `UTXO`,
`reserve`, `Reservation`, `reservation`; `gh pr list` by base/head branch and
by open state) against `reservations-epic`. This is the exhaustive set —
everything else that matches those terms is a false positive (list in the
"excluded" block below).

**tbtc-v2 — the 8-PR stack (stack `#1101`, bases verified 2026-08-21):**

| # | Title | Base branch | Head branch | State |
|---|---|---|---|---|
| 1088 | draft: UTXO reservations — segregated custody with in-kind redemption | `reservations-epic` | `feat/utxo-reservation-core` | OPEN / MERGEABLE |
| 1090 | delegatecall reservation router (EIP-170) + RFC 13 | `feat/utxo-reservation-core` | `feat/utxo-reservation-router` | OPEN / MERGEABLE |
| 1091 | two-phase authorize-then-prove reservation settlement | `feat/utxo-reservation-router` | `feat/utxo-reservation-settlement` | OPEN |
| 1092 | bounded permissionless renewal and strict expiry semantics | `feat/utxo-reservation-settlement` | `feat/utxo-reservation-renewal` | OPEN / MERGEABLE |
| 1093 | claim-equals-anchor backing model with financed in-kind fees | `feat/utxo-reservation-renewal` | `feat/utxo-reservation-backing` | OPEN / MERGEABLE |
| 1094 | reveal-side wallet binding, pending-deposit guard, stranding and monitoring | `feat/utxo-reservation-backing` | `feat/utxo-reservation-guards` | OPEN / MERGEABLE |
| 1095 | docs+test: reservation release completeness (M-09) | `feat/utxo-reservation-guards` | `docs/utxo-reservation-release` | OPEN / MERGEABLE |
| 1096 | partial reserved redemption (1-in-2-out split) | `docs/utxo-reservation-release` | `feat/utxo-reservation-partial-redemption` | OPEN / MERGEABLE |

The stack's root `#1088` targets `reservations-epic`; every sibling PR is
base-chained to the prior one, and per `gh-stack` semantics all checks review
against `reservations-epic`. `#1090` is up to date with its base (`feat/utxo-reservation-core`) and contains the `#1102` fold (see `agent-docs/inventory/pr-map.md`). However, `#1091` is 13 commits behind its base (`feat/utxo-reservation-router`) and `#1092` is 5 behind its base (`feat/utxo-reservation-settlement`), and `#1088` is 2 behind `reservations-epic`. Consequently, the `#1102` fold is present on `feat/utxo-reservation-core` and `feat/utxo-reservation-router` but ABSENT from `feat/utxo-reservation-guards` and `feat/utxo-reservation-partial-redemption`.

**tbtc-v2 — folded follow-up:**

| # | Title | Base branch | Head branch | State |
|---|---|---|---|---|
| 1102 | fix(reservation): address multi-agent review findings on #1088 | `feat/utxo-reservation-core` | `fix/utxo-reservation-review-followups` | **MERGED 2026-08-21** (`3566e059`) |

Not part of stack `#1101` by `gh-stack` linking, but merged into the root's
own branch, so its 30 findings are folded into `#1088`'s tip (§0 note, §3 note).

**keep-core — the standalone counterpart:**

| # | Title | Base branch | Head branch | State |
|---|---|---|---|---|
| 4238 | draft: UTXO reservation wallet-side foundations | `reservations-epic` | `feat/utxo-reservation-wallet-support` | OPEN, draft |

Retargeted directly to `reservations-epic` (§0). **Corrected 2026-08-21:** this
previously said `#4238` "implements the original single-phase design". Measured
against the source that is wrong - all four proposal structs carry
`RequestNonce` and the chain interface reads action records by generation via
`GetReservationAction(reservationKey, requestNonce)`, both two-phase
constructs. What `#4238` actually lacks is the **executor**: `pkg/tbtcpg` has
no reservation task at all, and the chain interface exposes only reads and
validators with no submission method, so the client cannot participate in the
protocol. See `inventory/keep-core.md` and `milestone-inventory.md` C-8.

The keep-core m1 client therefore remains **not yet an open PR** and must be
created, but it is new code on a reusable type layer rather than a rework of a
superseded design.

**Excluded as false positives (verified, not reservation work):**
- tbtc-v2: `#911` (non-fungible, closed), `#971` (FROST Taproot, base
  `frost-upgrade`), `#1003` (Trail of Bits covenant remediation, base
  `feat/psbt-covenant-bridge-port`), `#1036` (viem SDK migration), `#1043`-
  `#1051` / `#1082` (watchtower improvements, closed/merged), pre-2026
  historical `utxo` PRs (deposit/redemption pipeline, unrelated).
- keep-core: `#3866` (FROST/ROAST Go node), `#4169` (CovenantSigner EIP-712
  approvals — its `REDEEM`/`RENEW` are covenant PSBT actions, not
  `ReservationAction`s; the reservation action enum has no `RENEW`
  (`None|Acceptance|Redemption|Reanchor|Dissolution`)), `#4199` / `#4226`
  (FROST signer state-anchor / anchor-integrity on FROST scaffold branches),
  `#4243` (P2TR script derivation, FROST).

### 0.2 Milestone scope of this inventory

The inventory above covers all 8 tbtc-v2 PRs. It is **not** an m1 delivery
scope: m1 is variant B, an essentials-only rewrite (`roadmap.md` §1), so no
subset of these PRs constitutes m1.

Two points from the superseded plan survive as facts worth keeping:

- **`#1096` has no special status.** The old plan called it "the one clean PR
  omission" because a create-only m1 could defer it alone. Under B it is
  simply one of several unwritten m2 features (`roadmap.md` §3.1).
- **`#1092` cannot be treated as optional.** Its expiry model is structural to
  `#1093`+, not an additive layer (`roadmap.md` §0.1), so the rewrite must
  carry its snapshot semantics even though renewal itself is deferred.

Create-only behaviour comes from the m1 vault exposing no reachable redemption
or renewal entry point (`roadmap.md` §0.2), never from omitting PRs.

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

## 3. Review plan — tbtc-v2 stack (extraction guidance)

| Step | PR | Extraction guidance |
|---|---|---|
| 1 | #1088 | Extract core data model and `ReservationVault` (storage foundation). **Use post-#1091 form** for anything #1091 touched. |
| 2 | #1090 | Extract router delegatecall + EIP-170 workaround, RFC 13. (No later rewrite noted) |
| 3 | #1091 | Extract two-phase authorize-then-prove settlement. **This is m1's core; highest-value source.** |
| 4 | #1092 | Extract expiry/snapshot half (**window < term** arithmetic and dissolution-eligibility snapshotting). Skip renewal half. |
| 5 | #1093 | Extract claim-equals-anchor backing and financed in-kind fees. **Note: #1093 reworks the backing model established by #1091 -> use post-#1093 form for mintedAmount/anchorAmount.** |
| 6 | #1094 | Extract designated-wallet binding, pending-deposit guard, stranding, monitoring. (Stranding is m1's only close path) |
| 7 | #1095 | Reference only (docs and one test line). |
| 8 | #1096 | Skip (redemption is m2). |

**Note on extraction:** The #1102 fold is absent above #1091, so for files in the core branch (Reservation.sol, etc.) one must either extract from #1090's tip and re-apply #1091-#1095 changes, or rebase #1091 first.

## 4. Sections dropped as superseded

Three sections of the original plan described procedures that no longer apply.
They are recorded here rather than silently deleted:

| Dropped section | Why | Live version |
|---|---|---|
| keep-core Option A versus Option B (review `#4238` now or after a two-phase rework) | Both options assumed `#4238` was the delivery vehicle. Under B it is a rewrite, so the question does not arise | `pr-strategy.md` §8 |
| The combined-parity audit gate over all 8 PRs | m1 is a rewrite, so the combined parity of the original eight is not the audit target | `m1-b-implementation.md` §4.5, `roadmap.md` §2.1 |
| The `reservations-epic` to `main` landing procedure | It sequenced a stack merge that will not happen | `pr-strategy.md` §4 and §9 |

## 5. Immediate next actions (unblocked today)

- [ ] Decide on `reservations-epic` branch protection (§2) — currently
      unprotected in both repos.
- [ ] Start bottom-up review of the tbtc-v2 stack at `#1088` (§3) — for extraction guidance, see the review-plan table.
- [ ] Rebase `feat/utxo-reservation-router` (#1090) over the `#1102` fold —
      it is up to date with its base and contains the #1102 fold, but note that #1091 is 13 behind and #1092 is 5 behind.
- [ ] Verify the `#1102` merge (`3566e059`) is fully present on #1088's tip
      (nothing rebased away in the fold) before reviewing `#1088`.