# Follow-up PR Spec: Close the Two Confirmed M1 Gaps

Source: `01-gap-analysis.md` Blocker and Major findings (2026-09-03 audit).
This doc turns those two findings into buildable work. They target
**different git objects** and therefore are **not the same PR** — see
routing below before assigning either to an implementor.

## Routing decision

| Finding | Where the bad code lives | Correct fix vehicle | Why |
| :--- | :--- | :--- | :--- |
| Blocker — vault missing `redeemReservation`/`retryRedeemReservation` | `m1/vault-pause-flags` branch (PR **#1111**, currently OPEN, unmerged) | **Amend PR #1111 directly** — push a commit onto `m1/vault-pause-flags` | The PR that's supposed to deliver this hasn't merged yet. Opening a second PR to patch a still-open PR just fragments review and doubles the rebase burden. Fix it before #1111 merges. |
| Major — `dissolutionEligibleAt` gate never deleted from re-anchor | `Reservation.sol` on `reservations-upgrade` (merged via PR D / `m1/reanchor-core`, already landed at tip `8f30da66`) | **New PR, letter `I`** — `m1/reanchor-dissolution-gate-fix`, based on `reservations-upgrade`@`8f30da66` | PR D already merged. The bug is in code every PR built on top of (`reservations-upgrade`'s shared base). A genuine new follow-up PR is the only option; there's nothing open to amend. |

Both are independent, single-file-focused, small diffs. Nothing blocks
starting them in parallel.

---

## PR I: `m1/reanchor-dissolution-gate-fix`

### Target
- Base branch: `reservations-upgrade` at current tip `8f30da66a537b5acbcce2542c3f07a373e2bc384` (verify via `git rev-parse origin/reservations-upgrade` before branching — PRs #1111/#1112 may land and move this tip; rebase onto whatever is current).
- File: `solidity/contracts/bridge/Reservation.sol`
- Function: `requestReservationReanchor`

### Problem
`m1-b-implementation.md:190-192` states variant B's design deletes re-anchor's
`< dissolutionEligibleAt` timing gate, making re-anchor unbounded in time —
this is why acceptance still writes `dissolutionEligibleAt` even though the
spec's own field-level note says "no m1 reader" (`milestone-inventory.md`
line 102/108): once this gate is gone, that claim becomes literally true.
It never was deleted. `git log -L 750,756:solidity/contracts/bridge/Reservation.sol`
shows the `require` has been present unchanged since the first re-anchor
extraction (`8320906c`, 2026-08-24, from PR #1094) and survived a
34-of-38-finding review pass (`d600a8bf`) and a test-coverage commit
(`a5630007`) untouched.

**Current behavior:** a reservation that has aged past `dissolutionEligibleAt`
can never be re-anchored again — not even by a `privileged` (governance)
caller — while its wallet stays healthy. Since m1 ships no dissolution or
redemption path, that position has no exit until its custodying wallet
itself terminates (the one path this gate doesn't block, since stranding
doesn't check `dissolutionEligibleAt`).

### Change

**1. Delete the gate.** In `requestReservationReanchor`, remove:

```solidity
require(
    block.timestamp < reservation.dissolutionEligibleAt,
    "Reservation is dissolution-eligible"
);
```

Leave the `/* solhint-disable not-rely-on-time */` / `/* solhint-enable
not-rely-on-time */` pair around the remaining `reanchorCooldownUntil`
check intact (that check stays; only the dissolution-eligibility require is
removed). Do not touch `reservation.dissolutionEligibleAt`'s declaration
(`Reservation.sol:208`) or its write in `ReservationProofs.sol:579`
(`settleAcceptance` still needs to compute and store it — that write is
correctly load-bearing for m2's dissolution feature per the spec's own
"do not drop the write" instruction, `m1-b-implementation.md:192`). This PR
removes the reader, not the field.

**2. Update the one test that exercises the deleted behavior.**
`solidity/test/bridge/Bridge.ReservationStrandingLibrary.test.ts:2718-2741`,
`"rejects when reservation is dissolution-eligible"`, currently asserts the
re-anchor request reverts after time-traveling 400 days. Rewrite it to
assert the opposite: a dissolution-eligible reservation *can* still be
re-anchored. Reuse the happy-path assertions from the neighboring
`"successfully requests reservation re-anchor from Live wallet by
governance (privileged happy path)"` test (lines 2651-2695, same file) for
what to assert post-call — same request succeeds, `RequestNonce`
increments, `ReservationReanchorRequested` emits, capacity reserved on
target wallet. Rename the `it()` description to something like `"succeeds
when reservation is dissolution-eligible (re-anchor is unbounded in time,
m1-b-implementation.md section 1.5)"`.

**3. Search for any other test asserting the old revert string**, in case
the audit's single-file grep missed a duplicate assertion elsewhere:
`grep -rn "Reservation is dissolution-eligible" solidity/test/`. As of this
writing that string appears in exactly one place (the test above); if a
second hit exists by the time this PR is built, apply the same rewrite
there.

### Non-goals
- Do not touch the `reanchorCooldownUntil` check three lines above (unrelated, still-correct behavior).
- Do not touch the wallet-state gate below it (`Live` requires `privileged`, else `MovingFunds`/`Closing`) — confirmed correct against D-9, out of scope.
- Do not add a new test file; this is a one-line deletion plus one existing-test rewrite.

### Acceptance criteria
- `requestReservationReanchor` no longer reverts solely because
  `block.timestamp >= reservation.dissolutionEligibleAt`.
- The rewritten test passes: an aged-past-eligibility reservation
  successfully re-anchors.
- No other test in `Bridge.ReservationSettlement.test.ts`,
  `Bridge.ReservationSourceAnchorBinding.test.ts`, or
  `Bridge.ReservationStrandingLibrary.test.ts` regresses (run the full
  `test/bridge/Reservation*.test.ts` + `test/bridge/Bridge.Reservation*.test.ts`
  suite, not just the touched file — the gate's removal changes reachable
  state space for any test that seeds an aged reservation).
- `yarn lint:sol` and `yarn format --check` clean (repo convention; see
  `Makefile`/existing PR review-fix commits in this stack for the pattern).

---

## Fix for PR #1111 (amend in place, not a new PR)

### Target
- Branch: `m1/vault-pause-flags` (PR #1111), currently based on stale
  `reservations-upgrade@76eab846` — **rebase onto the current tip before or
  alongside this fix**; #1111 already shows `mergeable: CONFLICTING` for
  an unrelated reason (base has moved since #1110 merged), so a rebase is
  needed regardless of this change.
- File: `solidity/contracts/vault/ReservationVault.sol`

### Problem
`m1-b-implementation.md` §3 requires `redeemReservation` and
`retryRedeemReservation` to exist and be initiation-gated by
`redemptionsPaused` (D-13's explicit recommendation: "the flag gates the
two initiation functions only"). Neither function exists in the branch as
currently written — 10 functions total, confirmed by full-file grep. The
`redemptionsPaused` flag, `pauseRedemptions`, and `unpauseRedemptions` are
present and correct, but gate nothing. `receiveBalanceApproval`'s own
NatSpec (`:370-372`) already documents the intended design: "the reservation
vault does not support the balance approval flow; reserved redemptions are
initiated via `redeemReservation`" — describing a function that doesn't
exist yet.

### Change
Add two initiation-only entry points to `ReservationVault.sol`, each
reverting while `redemptionsPaused == true` (the constructor already
defaults it `true`, per PR #1111's own row in `pr-strategy.md` §4.1):

1. **`redeemReservation`** — initiates a whole or partial in-kind
   redemption of a reservation the caller owns. Per the M1-out-of-scope
   framing (this is M2 functionality shipped-but-disabled in M1, per §1's
   "vault ships complete, behavior disabled" rule), the exact
   parameter/return shape should mirror whatever `#1091`'s/`#1096`'s
   reference `requestReservedRedemption`-adjacent vault call expects on the
   Bridge side, since M2 will wire this to the Bridge's redemption request
   path without changing the vault's signature (`m1-b-implementation.md`
   §1's "M2 is an upgrade, not a migration" property — the same rule
   `unwindPendingAction`'s D-10/D-11 decisions already apply on the Bridge
   side). Guard: `require(!redemptionsPaused, "Redemptions are paused")` as
   the first statement.
2. **`retryRedeemReservation`** — same guard, covering the retry-credit
   redemption path (`ReservationAction.retryCredit`/`usedRetryCredit`
   fields already declared on the Bridge side per D-5, inert in m1).

Both functions are pure initiation-path additions: per D-13, neither should
touch `financeInKindFee`, `repayInKindFeeDebt`, or any other
settlement-adjacent accounting function, which must stay ungated
(re-anchor's in-kind fee call on the settlement path depends on those
staying callable regardless of `redemptionsPaused`).

### Acceptance criteria
- `grep -n "function redeemReservation\|function retryRedeemReservation" solidity/contracts/vault/ReservationVault.sol` returns two matches.
- Both revert with `redemptionsPaused == true` (the constructor default) and are exercised by a new unit test pair confirming the revert.
- `receiveBalanceApproval`'s NatSpec claim becomes accurate (function it names now exists).
- No change to `financeInKindFee`/`repayInKindFeeDebt`/`updateFeeReserveTarget` (settlement-path functions, must stay ungated per D-13).

---

## Tracking

Once PR I is opened, add it to `pr-strategy.md` §4.1's decomposition table
(new row, letter `I`, base `reservations-upgrade`) and to
`agent-docs/m1/STATUS.md`'s queue table. The #1111 fix doesn't get a new
row — it's a commit onto the existing PR F entry.
