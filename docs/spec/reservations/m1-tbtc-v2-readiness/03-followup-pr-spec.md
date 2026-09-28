# Follow-up PR Spec: Close the Two Confirmed M1 Gaps

Source: `01-gap-analysis.md` Blocker and Major findings (2026-09-03 audit).
This doc originally turned those two findings into buildable work; both are
now resolved (one built as specified, one superseded by a later decision).
Retained as the historical record of what was built and why, not as an open
work queue.

**Status (2026-09-28):**
- **PR I** (dissolution-gate fix) — **Done.** Merged as tbtc-v2
  [#1120](https://github.com/threshold-network/tbtc-v2/pull/1120)
  (`m1/reanchor-dissolution-gate-fix`), 2026-09-03.
- **Fix for PR #1111** (vault redemption entry points) — **Superseded, not
  built as originally specified.** PR
  [#1111](https://github.com/threshold-network/tbtc-v2/pull/1111) itself
  merged 2026-09-03, but the amendment this section specified
  (`redeemReservation`/`retryRedeemReservation` gated by
  `redemptionsPaused`) was never added to it. The 2026-09-24 Option B
  decision (`m1-b-implementation.md` §3/§4.2, commit `661b06a04`)
  supersedes the requirement this section was written against: the M1
  vault instead ships deliberately minimal (commit `4d549e64`), and M2
  delivers redemption/renewal via a **new vault deployment plus a
  depositor migration ceremony**, not by unpausing flags on this vault
  (the vault is not proxy-upgradeable, so there is no flag to flip after
  the fact). See the superseded section below for what was originally
  asked for and why it's no longer the right design.

## Routing decision (historical — both items since resolved)

| Finding | Where the bad code lived | Correct fix vehicle | Why | Outcome |
| :--- | :--- | :--- | :--- | :--- |
| Blocker — vault missing `redeemReservation`/`retryRedeemReservation` | `m1/vault-pause-flags` branch (PR **#1111**) | Amend PR #1111 directly | The PR that was supposed to deliver this hadn't merged yet at audit time. | **Superseded** — PR #1111 merged as-is (without the amendment); the requirement itself was later withdrawn by the Option B decision. See "Fix for PR #1111 (superseded)" below. |
| Major — `dissolutionEligibleAt` gate never deleted from re-anchor | `Reservation.sol` on `reservations-upgrade` (merged via PR D / `m1/reanchor-core`) | New PR, letter `I` — `m1/reanchor-dissolution-gate-fix` | PR D had already merged; the bug was in code every later PR built on top of. | **Done** — merged as PR #1120, 2026-09-03. |

---

## PR I: `m1/reanchor-dissolution-gate-fix` — Done (merged as #1120)

### Target
- Base branch: `reservations-upgrade`.
- File: `solidity/contracts/bridge/Reservation.sol`
- Function: `requestReservationReanchor`

### Problem
`m1-b-implementation.md:190-192` states variant B's design deletes re-anchor's
`< dissolutionEligibleAt` timing gate, making re-anchor unbounded in time —
this is why acceptance still writes `dissolutionEligibleAt` even though the
spec's own field-level note says "no m1 reader" (`milestone-inventory.md`
line 102/108): once this gate is gone, that claim becomes literally true.
At audit time it had not been deleted: the `require` had been present
unchanged since the first re-anchor extraction (PR #1094) and survived a
34-of-38-finding review pass and a test-coverage commit untouched.

**Behavior before the fix:** a reservation that had aged past
`dissolutionEligibleAt` could never be re-anchored again — not even by a
`privileged` (governance) caller — while its wallet stayed healthy. Since
m1 ships no dissolution or redemption path, that position had no exit until
its custodying wallet itself terminated (the one path this gate didn't
block, since stranding doesn't check `dissolutionEligibleAt`).

### Change (as landed in #1120)

**1. Delete the gate.** In `requestReservationReanchor`, the following was
removed:

```solidity
require(
    block.timestamp < reservation.dissolutionEligibleAt,
    "Reservation is dissolution-eligible"
);
```

`reservation.dissolutionEligibleAt`'s declaration and its write in
`ReservationProofs.sol` (`settleAcceptance`) were left untouched — that
write remains correctly load-bearing for m2's dissolution feature per the
spec's "do not drop the write" instruction (`m1-b-implementation.md:192`).
This PR removed the reader, not the field. The `reanchorCooldownUntil`
check and the wallet-state gate (`Live` requires `privileged`, else
`MovingFunds`/`Closing`) were both left in place, unrelated and
still-correct.

**2. Test rewrite.** `Bridge.ReservationStrandingLibrary.test.ts`'s
`"rejects when reservation is dissolution-eligible"` test, which previously
asserted the re-anchor request reverts after time-traveling 400 days, was
rewritten to assert the opposite: a dissolution-eligible reservation *can*
still be re-anchored (same request succeeds, `RequestNonce` increments,
`ReservationReanchorRequested` emits, capacity reserved on target wallet).

### Acceptance criteria (met)
- `requestReservationReanchor` no longer reverts solely because
  `block.timestamp >= reservation.dissolutionEligibleAt`. **Met** — no
  `dissolutionEligibleAt` check remains in the function; a code comment
  documents the removal ("Re-anchor is intentionally unbounded in time in
  milestone 1, with no dissolution path available yet").
- The rewritten test passes: an aged-past-eligibility reservation
  successfully re-anchors. **Met.**
- No other test in `Bridge.ReservationSettlement.test.ts`,
  `Bridge.ReservationSourceAnchorBinding.test.ts`, or
  `Bridge.ReservationStrandingLibrary.test.ts` regressed. **Met.**

---

## Fix for PR #1111 (superseded — do not build as specified below)

**This section is retained only as the historical record of what was
originally asked for.** It described adding `redeemReservation` and
`retryRedeemReservation` to `ReservationVault.sol`, gated by
`redemptionsPaused`, per D-13's "ship both initiation functions behind the
pause flag" recommendation. That recommendation is superseded by the
2026-09-24 Option B decision: the M1 vault ships deliberately minimal
instead (confirmed in code — the current `ReservationVault.sol` has 7
external functions plus constructor: `receiveBalanceIncrease`,
`financeInKindFee`, `repayInKindFeeDebt`, `updateFeeReserveTarget`,
`sweepFees`, `updateInitiationFee`, `receiveBalanceApproval`, plus internal
`_burnFromReserve` — no `redeemReservation`, `retryRedeemReservation`,
`redemptionsPaused`, `pauseRedemptions`, or `unpauseRedemptions` exist at
all).

**The M2 path is a new vault deployment plus a depositor migration
ceremony, not a flag flip on this vault.** The original argument for the
pause-flag design was that it would let M2 "unpause" redemption without a
vault swap. Consequence to state honestly: because the M1 vault is not
proxy-upgradeable, and re-pointing `Bridge.reservationVault` to a new vault
requires total system quiescence
(`reservationTotalAmount == 0 && pendingReservedDeposits == 0`), M2 now
needs exactly the migration ceremony the flag design was meant to avoid —
depositors will need to move their claims from the M1 vault to the M2 vault
rather than simply seeing a flag flip. See
`../m1-keep-core-readiness/01-gap-analysis.md` and
`../m1-keep-core-readiness/02-user-stories.md` "Out of M1 scope" for the
keep-core-side implication (no coordinator/executor wiring for redemption
exists either, correctly, since the vault never gained these entry
points).

### Original target (not built)
- Branch: `m1/vault-pause-flags` (PR #1111) — merged 2026-09-03 without this amendment.
- File: `solidity/contracts/vault/ReservationVault.sol`

### Original problem statement (superseded)
`m1-b-implementation.md` §3 required `redeemReservation` and
`retryRedeemReservation` to exist and be initiation-gated by
`redemptionsPaused` (D-13's recommendation). Neither function was ever
added; the `redemptionsPaused` flag and its toggle functions were removed
entirely from the vault by the time the M1 code went minimal (commit `4d549e64`),
rather than being left in place gating nothing.

### Do not build this
Do not add `redeemReservation`, `retryRedeemReservation`,
`redemptionsPaused`, `pauseRedemptions`, or `unpauseRedemptions` to the M1
`ReservationVault.sol`. If M2 redemption/renewal work is picked up, build
it against a **new** vault contract plus a migration path, per the Option B
decision above — not as an amendment to the M1 vault.

---

## Tracking

Both items tracked by this document are closed. PR I merged as tbtc-v2
#1120 (2026-09-03). The PR #1111 amendment is superseded — no further
tracking entry needed; M2's redemption/renewal work should be scoped as a
new vault + migration effort against `requirements.md`/`architecture.md`
§9 ("What M2 changes"), not as a continuation of this spec.
