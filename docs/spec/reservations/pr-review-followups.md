# PR review — follow-up items only (tbtc-v2 #1088, UTXO reservation core)

Source: adversarial multi-lens review of `feat/utxo-reservation-core`
(commit `3d4335d7`) in a standalone `tbtc-v2` checkout — this is the tip of
**tbtc-v2 PR #1088** ("Core reservation data model, `ReservationVault`,
original single-phase mechanics"), the bottom of the 8-PR reservation stack
tracked in `epic-merge-plan.md`.

This file intentionally omits the review's "can fix immediately" items
(deploy-script ownership transfer, a missing zero-address check, a missing
`extendCustody` slippage bound, test-coverage gaps, etc.) — those are
contained, PR-local fixes, not epic-level follow-ups. Only items needing a
new mechanism or a design decision are listed here.

**Every item below was cross-referenced against the H-/M-/C- issue catalog
already in `feature-spec.md` §"tracking table"
(lines ~704-730).** That catalog was written from a review of the *later*
PRs in the stack, not from this review — the matches below are the
orchestrator's correlation between two independent reviews of the same
feature at different stack depths, not a re-verification of #1091-#1094's
actual diffs. Treat "closed by #10XX" as "verify, don't assume."

**Update (re-verified 2026-08-20 against #1088 @ `36d471b4`):** two more
commits landed on #1088 directly since the review above was written —
`d89a649a` ("address PR #1088 multi-agent review findings") and `36d471b4`
(matching test coverage). They fixed several items from the original
full review, including the "can fix immediately" bucket this file omits
(the `ReservationVault` ownership-transfer gap and the zero-address/EOA
vault check are now both implemented directly on #1088). More relevant to
this follow-up list: **item 5 below is now resolved directly in #1088**,
not just claimed-closed further up the stack, and two new findings
surfaced that aren't in the original H-/M-/C- catalog at all (see the
addendum at the bottom). Items 1-4 and 6 are unchanged — checked directly
against the current code, all still open.

**Update (2026-08-21, from tbtc-v2 #1102 @ `53fe9229`):** item 7 added. It
comes from the multi-agent review of **#1102** (the follow-up PR that
addresses #1088's review findings, stacked on `feat/utxo-reservation-core`),
so this file now spans two reviews at two stack depths. Item 7 is a residual
on item 5, not a new gap: the cap item 5 credits as resolved has no bound of
its own. A "Minor" section after item 7 records the single P3 finding from
#1102's review that was deliberately not implemented; everything else that
review confirmed (30 findings) is fixed and pushed on #1102 itself.

**Status vs M1 code (2026-09-28)** — tbtc-v2 `reservations-upgrade` @
`9f8f5ef1`, keep-core `reservations-epic` @ `f66f11240`. Index, not a
renumbering (item numbers below are unchanged; the three unnumbered
sections between items 7 and 8 are listed here for navigability):

| # | Item | Status at M1 code |
|---|---|---|
| 1 | Wallet termination strands active reservations | CLOSED — `notifyReservationStranded` reachable from all three termination paths, confirmed unchanged in substance |
| 2 | Vault rotation blocked while any reservation outstanding | OPEN — accepted m1 liveness tradeoff, revisit at m2 |
| 3 | No permissionless fallback if SPV maintainer stalls | OPEN — accepted m1 risk with operational (not code) mitigation |
| 4 | Live governance parameters applied retroactively | CLOSED — `reservationTxMaxFee` confirmed snapshotted; `reservationGracePeriod` was renamed `reservationDissolutionDelay`, also snapshotted per action |
| 5 | Unbounded re-anchor grinding | SUPERSEDED for m1 — `maxCumulativeReanchorFee` dropped; a request-time amount floor (Reservation.sol:805-808) bounds the grind instead, see Resolution section |
| 6 | Redeemer output-script check bypassable via P2SH/P2WSH | MOOT for m1 — no reserved-redemption entry point exists yet (m1 has no redemption); pooled-path bug is real and unaffected |
| 7 | `maxCumulativeReanchorFee` itself unbounded | RESOLVED for m1 — lever 4 (leave unbounded) plus the item-5 request-time floor, characterization-test-pinned; see Resolution section |
| Minor | The one #1102 finding left unimplemented | Unaffected by m1 rewrite; not independently re-verified here |
| Addendum | Two items outside the original review's scope | Unaffected by m1 rewrite; not independently re-verified here |
| Resolution | Resolution of items 5 and 7 for milestone 1 | Reconciled below against the true M1 code (previously verified against an interim `#1093` tip) |
| 8 | Aggregate in-kind fee debt unbounded and only voluntarily repayable | PARTIALLY RESOLVED — `sweepFees` no longer reverts in the high-debt regime (fixed via an early return, not the debt-capped-at-target design decided 2026-09-07); `financeInKindFee` is still uncapped, so the no-ceiling half stays open |
| 9 | Permissionless re-anchor requests let an outsider dictate migration targets | NARROWED — a `reanchorCooldownUntil` cooldown (implemented, not "post-m1 work" as originally framed) now gates the immediate re-request the attacker relied on |

---

## 1. Wallet termination strands active reservations with no recovery path
**Severity: High.** `notifyWalletMovingFundsTimeout`, `notifyWalletMovedFundsSweepTimeout`,
and `notifyWalletFraudChallengeDefeatTimeout` (`Wallets.sol:468, 502, 545` in
#1088) all call `terminateWallet` unconditionally. Only
`notifyWalletClosingPeriodElapsed` checks `walletReservationsCount == 0`
(`Wallets.sol:382`). A wallet slashed into termination via any of the other
three paths permanently strands every reservation it was custodying: the
owner keeps their TBTC but loses the in-kind BTC claim forever, and the
anchor UTXO becomes unspendable by anyone.

Naively porting the `walletReservationsCount == 0` guard onto the
punitive/timeout paths would itself be griefable — a wallet operator could
open a dust reservation purely to block its own deserved slashing forever —
so this needs a real design (a forced-unwind step as part of termination),
not a guard clause.

**Catalog match:** `H-06 — Voluntary/involuntary termination could strand
live anchors with no recovery path`, closed by **#1094**
(`notifyReservationStranded`, permissionless, wallet must be `Terminated`:
moves position to `Stranded`, releases capacity, unwinds any pending
action). Description matches this finding closely. **Action: confirm
`notifyReservationStranded` is reachable from all three timeout/slashing
paths above, not only from the graceful-closing path** — the catalog entry
doesn't distinguish which termination triggers it covers.

**Re-verified 2026-09-07 against live `reservations-upgrade` (post #1088-#1122,
tip `52bf2822`):** confirmed, unchanged. The three call sites are unchanged at
`Wallets.sol:468, 502, 545` (`notifyWalletMovingFundsTimeout`,
`notifyWalletMovedFundsSweepTimeout`, `notifyWalletFraudChallengeDefeatTimeout`)
— all still call `terminateWallet` (`Wallets.sol:687-709`) with no
reservation-count guard. The graceful path (`finalizeWalletClosing`,
`Wallets.sol:666-682`) still has the `walletReservationInfo[wallet].count ==
0` require. `notifyReservationStranded`'s underlying `strandReservation` call
(`ReservationProofs.sol:238-239,267-268`) triggers on wallet state `Closed`
**or** `Terminated` with no distinction by which of the three paths produced
the `Terminated` state — so the recovery path is confirmed reachable from all
three, not only the graceful one. Item CLOSED: verified as claimed, no
further action needed.

**Status at M1 code (2026-09-28):** unchanged from the 2026-09-07
re-verification above (only the `terminateWallet` line citation drifted,
`Wallets.sol:687-709` at `9f8f5ef1`; corrected above). CLOSED.

## 2. Vault rotation is blocked while any reservation is outstanding
**Severity: High.** `updateReservationParameters` refuses to change
`reservationVault` while `reservationTotalAmount != 0`
(`Reservation.sol:1011-1018` in #1088). A single reservation owner who never
redeems (cost: one min-size reservation) can block vault migration
indefinitely — nobody but the owner (redeem) or the wallet/SPV-maintainer
(dissolve, and only after the grace period) can close it out, and neither
party is incentivized to act for someone else's benefit.

**Catalog match:** `M-04 — Vault change could orphan already-revealed
reserved deposits`, closed by **#1094** ("pending-deposit tracking +
vault-migration guard"). **Caveat: the catalog description is scoped to
*pending* (revealed-but-not-yet-accepted) deposits, not *already-Active*
reservations.** This finding is about the latter — an existing accepted
reservation blocking rotation via the `reservationTotalAmount != 0` check.
**Action: verify #1094's vault-migration guard also has an answer for
already-Active reservations sitting on the old vault, not just pending
deposits** — these read as two different facets of the same "vault
migration is unsafe with live state" problem, and the catalog only
confirms one is fixed.

**Re-verified 2026-09-07 against live `reservations-upgrade` (tip
`52bf2822`):** confirmed, unchanged in substance (at `9f8f5ef1` the function
starts at `Reservation.sol:1260`, with the guard at `:1308-1312`). The guard
visibly also checks `self.pendingReservedDeposits == 0` in the same `if`
block — so the M-04
pending-deposit fix and this Active-reservation guard live side by side in
one function, not stacked or superseding one another. Both are real,
independent gates. The liveness cost stands exactly as described: a single
active reservation (min size) that its owner never redeems or lets dissolve
blocks vault rotation indefinitely. This is a genuine accepted-tradeoff/
policy question, not a code defect — item stays OPEN pending a governance
decision (e.g., a governance-forced dissolution override), not a fix.

**Decided 2026-09-07 (session decision, via `unblock-gaps`):** accept as-is
for m1; revisit once m2's dissolution executor exists. A governance-forced
dissolution override cannot be built before dissolution itself, which is out
of m1 scope — so no code fix is available before m2 regardless of the
policy answer. Documented as an accepted m1 liveness tradeoff.

**Status at M1 code (2026-09-28):** unchanged — still the same accepted
m1 liveness tradeoff decided 2026-09-07, no new code since. OPEN
(accepted).

## 3. No permissionless fallback if the SPV maintainer stalls
**Severity: High.** In #1088, all four reservation lifecycle proofs
(Acceptance/Redemption/Reanchor/Dissolution) route through one
`onlySpvMaintainer`-gated entry point. There's no permissionless fallback
for Reanchor/Dissolution the way `notifyRedemptionTimeout` exists on the
pooled path — if the SPV maintainer stalls, an expired reservation can't be
dissolved by anyone else.

**Catalog match: confirmed by the 2026-08-21 multi-agent review — proof
submission REMAINS maintainer-gated, only the request side opened up.**
#1091's "two-phase authorize-then-prove reservation settlement" makes
`requestReservationDissolution` (eligible once `now > dissolutionEligibleAt`)
and `notifyReservationActionTimeout` permissionless on the *request* side,
but proof submission stays behind the single `onlySpvMaintainer`-gated
entry point (`ReservationRouter.sol:208-212,:303-312`; corroborated at
`Bridge.sol:327-330,:866`). **This item's premise stands: verified, not
resolved.** A maintainer stall still blocks dissolution with no
permissionless fallback — worse, a stalled-then-timed-out dissolution
*slashes the wallet* (`notifyWalletRedemptionTimeout` ->
`ecdsaWalletRegistry.seize`) and can terminate an entirely honest,
unresponsive-maintainer-blocked wallet, stranding every position it
custodies. Enumerate the mainnet `isSpvMaintainer` set before launch;
`MaintainerProxy.sol` wraps the four pooled-path proofs but has no
`submitReservationProof` wrapper, so reservation settlement may have no
mainnet submitter wired at all as of #1091 — verify against #1094/#1095.

**Re-verified 2026-09-07 against live `reservations-upgrade` (tip
`52bf2822`):** confirmed, unchanged in substance, but the entry-point
description above needs a correction the 2026-09-07 pass got wrong: there is
no `ReservationRouter.submitReservationProof` dispatcher. Proof submission
is instead two separate, independently `onlySpvMaintainer`-gated external
entry points — `submitReservationAcceptanceProof`
(`ReservationRouter.sol:252-267`) and `submitReservationReanchorProof`
(`ReservationRouter.sol:278-293`), each decorated `external onlySpvMaintainer`
directly — with no `proofType`-dispatch parameter exposed externally
(`ReservationProofs.submitReservationProof` is an internal library helper
each of the two callers invokes separately). The underlying conclusion is
unaffected: no permissionless fallback exists at either entry point.
Re-anchor proof submission goes through
this same gate. Dissolution's proof path is not yet wired in m1 (dissolution
itself is declared-only per `m1-b-implementation.md`), so the live-today risk
is specifically re-anchor: a stalled SPV maintainer blocks re-anchor proof
submission with no fallback. Item stays OPEN — a genuine design/policy gap,
not a code defect to patch here.

**Decided 2026-09-07 (session decision, via `unblock-gaps`):** accept for m1
plus an operational mitigation — governance sets a multisig/multi-operator
maintainer set (not a single EOA) for `isSpvMaintainer`, plus
monitoring/alerting on re-anchor proof-submission latency so a stall is
caught before it compounds. `isSpvMaintainer` (`BridgeState.sol:302`) is
pre-existing shared Bridge state, not reservations-specific infrastructure
that needs building — governance already has the lever to set it.
Documented as an accepted m1 risk with an operational (not code) mitigation.

**Status at M1 code (2026-09-28):** unchanged in substance from the
2026-09-07 pass above (with the entry-point-count correction folded in
directly there). Still the same accepted m1 risk with an operational
mitigation. OPEN (accepted).

## 4. Live (non-snapshotted) governance parameters applied retroactively
**Severity: High.** `updateReservationParameters` in #1088 changes
`reservationGracePeriod` and `reservationTxMaxFee` for every reservation
immediately, including already-Active ones — nothing is snapshotted at
acceptance time. Governance shrinking `reservationGracePeriod` mid-flight
can flip Active reservations straight into "dissolvable now" or lock out
pending extensions with no transition window.

**Catalog match: strong, three-way.**
- `M-01 — Snapshot policy for term/fee/timeout parameters`, closed by **#1091**.
- `M-06 — Mid-flight governance parameter changes vs in-flight renewal`, closed by **#1092**.
- `M-09 — Grace-period governance rollback could retroactively move eligibility`, closed by **#1092** (mechanics) / **#1095** (docs+tests).

This is the best-covered item on this list — three separate catalogued
fixes target exactly this class of bug. Low residual risk; spot-check that
#1091's snapshot covers *both* fields this finding names
(`reservationGracePeriod` and `reservationTxMaxFee`), not just one.

**Re-verified 2026-09-28 against M1 code (`9f8f5ef1`) — this item never got
a 2026-09-07-style pass; overdue.** `reservationGracePeriod` does not exist
anywhere in the M1 contracts (0 matches in `solidity/contracts/`); it was
renamed `reservationDissolutionDelay` (`BridgeState.sol:386`,
`IReservationBridge.sol`, `ReservationRouter.sol`), and it is snapshotted
per action exactly like `reservationTxMaxFee`:
`action.dissolutionDelay = self.reservationDissolutionDelay`
(`Reservation.sol:628`). `reservationTxMaxFee` is confirmed snapshotted onto
`ReservationAction.txMaxFee` at both call sites
(`Reservation.sol:623,846`). The open "spot-check" action from the original
finding is closed: both fields this finding names are snapshotted, just
one of them under a new name. Item CLOSED for the fields as they exist
today; note for whoever tracks the catalog IDs that `reservationGracePeriod`
in M-01/M-06/M-09 above should now read `reservationDissolutionDelay`.

**Status at M1 code (2026-09-28):** CLOSED, see re-verification above.

## 5. Unbounded re-anchor grinding — RESOLVED directly in #1088 for the stack; cap later dropped for m1, see "Resolution of items 5 and 7" below
**Severity was High, self-limiting; now closed at the source, not just
further up the stack.** `submitReservationReanchorProof` originally had no
nonce, cooldown, or cumulative fee-budget cap; the code's own comment
deferred this to a "follow-up." Commit `d89a649a` on #1088 fixed it
directly: `updateReservationParameters` now carries a governance-set
`maxCumulativeReanchorFee`, `ReservationRequest.cumulativeReanchorFee`
tracks the running total per reservation
(`reservation.cumulativeReanchorFee <= self.maxCumulativeReanchorFee`
enforced on every re-anchor), and a dust floor was added to the re-anchor
amount itself. Verified directly against current `Reservation.sol` and
confirmed passing: `rejects a re-anchor paying an excessive fee` and
`rejects a re-anchor landing at or below the dust floor` in
`Bridge.Reservation.test.ts`.

**Catalog match:** `H-04 (backing) — Dissolution permanently underbacks by
cumulative in-kind fees; re-anchor temporarily underbacks`, also claimed
closed by **#1093** further up the stack. Two independent fixes for the
same gap (one on #1088 itself, one claimed on #1093) is a good sign, not a
conflict — **action: when #1093 is reviewed, confirm its backing model is
compatible with (or supersedes) #1088's `maxCumulativeReanchorFee` cap
rather than silently stacking two different caps.**

**Status at M1 code (2026-09-28): SUPERSEDED, not simply resolved.**
`maxCumulativeReanchorFee` does not exist in the M1 contracts at all —
dropped as unused/dead (`BridgeState.sol:492-495`,
`BridgeGovernanceParameters.sol:1620-1626`). What ships instead is a
request-time amount floor, `anchorAmount > txMaxFee + minAmount`
(`Reservation.sol:805-808`), which bounds the grind by a different
mechanism than the one this item credits. See "Resolution of items 5 and 7
for milestone 1" below — the record there is now corrected to match.

## 6. Redeemer output-script check is bypassable via P2SH/P2WSH — no catalog match, still open
**Severity: Medium-High. Not found anywhere in the existing H-/M-/C-
catalog — genuinely new, needs its own ticket. Re-verified against
current #1088; unchanged.**

`requestReservedRedemption`'s "must not pay back to the wallet" guard uses
`BTCUtils.extractHashAt`, which only compares against `walletPubKeyHash`
when the script payload is exactly 20 bytes (P2PKH/P2WPKH). A P2SH script
(20-byte *redeem-script* hash, not a pubkey hash) or any P2WSH script
(32 bytes, skips the length==20 branch entirely) sails through even when
the underlying script is trivially satisfiable by the wallet's own key
(e.g. P2WSH wrapping `<walletPubKey> OP_CHECKSIG`). A deceived or
colluding redeemer's reservation can be routed straight back to the
wallet operator, burning the full `mintedAmount` while the operator
recaptures the entire anchor.

**Update:** as of `d89a649a`, the check was extracted into a new shared
library function, `OutboundTx.validateRedeemerOutputScript`
(`Redemption.sol:113-137`), explicitly documented as "Shared by both the
pooled redemption and reserved redemption request paths, which apply the
identical check." This confirms the bug is not a reservation-specific
regression — it's the pooled path's pre-existing, already-deployed check,
now formally shared rather than duplicated. Reinforces the original
recommendation: **track as a ticket against `Redemption.sol`/`OutboundTx`,
independent of the reservation epic** — no PR in this stack (nor the
source review) would fix it, since fixing it changes live mainnet pooled
redemption behavior, not just reservation code.

**Status at M1 code (2026-09-28): MOOT for m1, not verified against #1088.**
Neither `requestReservedRedemption` nor
`OutboundTx.validateRedeemerOutputScript` exists anywhere in the M1
contracts snapshot (0 matches for both) — reserved redemption is entirely
out of m1 B scope (no redemption at all, per the roadmap decision), so
there is currently no reserved-redemption entry point for this bug to
affect. The underlying P2SH/P2WSH bypass is real and unchanged, but lives
only in the pooled path's `requestRedemption`-family function
(`Redemption.sol:536-555`), inline, never extracted into a shared library
in this snapshot — independent of the reservation epic either way. Re-check
this item once m2 ships a reserved-redemption path.

## 7. `maxCumulativeReanchorFee` itself is unbounded — residual on item 5
**Severity: Medium. Provenance: multi-agent review of #1102, not the
original #1088 review and not in the H-/M-/C- catalog.** Line refs below are
against #1102 @ `53fe9229`, where they were verified.

Item 5 credits `d89a649a` with closing the unbounded re-anchor grinding gap
via `maxCumulativeReanchorFee`. The cap exists and is enforced on every hop.
What it lacks is a bound of its own: `updateReservationParameters` validates
only `maxCumulativeReanchorFee > 0` (`Reservation.sol:1236-1239`), while the
relational check sitting four lines above it does constrain its neighbour —
`reservationMinAmount > reservationTxMaxFee` (`Reservation.sol:1228-1231`).
So the per-reservation unreconciled fee loss is bounded by governance
parameter choice alone:

```
min(maxCumulativeReanchorFee, mintedAmount - reservationTxMaxFee - 1)
  + reservationDissolutionTxMaxFee
```

With #1102's fixture values (`reservationMinAmount` 10000, `reservationTxMaxFee`
2000, `reservationDissolutionTxMaxFee` 1500, `maxCumulativeReanchorFee` 100000)
the dust floor (`Reservation.sol:980-983`) is strict —
`newAnchorAmount > reservationTxMaxFee`, hence the `- 1` above — so maximal
grinding always lands the anchor on 2001, and dissolution then takes up to 1500.

**The satoshi left backing a fully-ground reservation is therefore a constant**
— `reservationTxMaxFee + 1 - reservationDissolutionTxMaxFee`, here **501** —
**and it does not depend on how large the claim was.** That is the whole
finding, and it runs opposite to the intuitive reading:

| Claim | Max grind | Anchor at dissolution | Backing left | Loss |
|---|---|---|---|---|
| 10000 (minimum) | 7999 (floor binds) | 2001 | 501 | 94.99% |
| **102001 (peak)** | **100000 (cap binds exactly)** | 2001 | 501 | **99.51%** |
| 1000000 | 100000 (cap binds early) | 900000 | 898500 | 10.15% |

Fractional underbacking *grows* with position size and peaks at
`maxCumulativeReanchorFee + reservationTxMaxFee + 1`, where the budget and the
dust floor bind simultaneously. Above that the cap stops the grind early and
the ratio improves. So the exposure is worst on mid-sized positions, not the
minimum-sized ones — my first pass on this item asserted the opposite.

**What is executed vs computed.** The *invariant* is executed:
`Bridge.Reservation.test.ts` -> "leaves residual backing independent of claim
size after maximal grinding" grinds a minimum-size and a peak-size reservation
to the dust floor and asserts both leave identical backing, that the peak case
exhausts the cumulative budget exactly, and that fractional loss is worse for
the larger claim. Mutation-checked by making the dust floor proportional
(`newAnchorAmount * 2 > mintedAmount`), which fails it.

It runs a scaled parameter regime (`reservationTxMaxFee` 100,
`reservationDissolutionTxMaxFee` 60, `maxCumulativeReanchorFee` 400,
`reservationMinAmount` 150) to keep the grind to a handful of hops — the peak
claim there is 501, not 102001. **Every figure in the table above is arithmetic
from the invariant at fixture values, not a measured result**; no test exercises
the 2000/1500/100000 regime, which would need 50 hops. The mechanism is
verified; the specific fixture numbers are derived.

Medium rather than High, for three reasons — but the first reason splits
by adversary, per the 2026-08-21 multi-agent review:
- **Outside griefer:** the value goes to Bitcoin miners, not to the
  attacker. Griefing cost is roughly 1:1 with the damage, and there is no
  profit unless the attacker also captures the miner fee.
- **Custodying wallet operator (the case that actually matters):** this
  premise does NOT hold. The re-anchor miner fee is deducted from the
  *anchor* — the depositor's backing, not the operator's stake (§6 "the
  anchor shrinks by the miner fee") — so a Byzantine operator's
  out-of-pocket cost is ~zero, the action is authorized and provable (no
  fraud slashing, unlike a raw stolen-spend), and it is repeatable across
  the whole global cap. This adversary's griefing cost is not 1:1; treat it
  as the severity-driving case, not the outside-griefer case.
- It needs a Byzantine wallet operator *and* SPV-maintainer proof
  submission (confirmed gated, item 3 above) — not a permissionless path,
  which does bound *frequency* even though it does not bound the operator's
  own-position griefing cost above.
- The loss class is not novel. `MovingFunds.submitMovingFundsProof`
  (`MovingFunds.sol:412-415`) caps migration fees against
  `movingFundsTxMaxTotalFee` and reconciles nothing either, and that
  parameter is likewise only `> 0`-validated (`BridgeState.sol:759-762`).
  Pool-socialized operational Bitcoin fees are existing, shipped tBTC policy.

**The decision required is a ratio**, which is why it is not a drive-by fix:
what fraction of a reservation's own value may governance permit to evaporate
into miner fees before the parameter is rejected?

The constant-backing result above settles the shape of the answer:
**no bound expressed on the fee caps can deliver a fractional guarantee.**
Backing left after maximal grinding is `txMaxFee + 1 - dissolutionCap`, an
absolute number, while the claim it backs is arbitrary. Caps can only move
*where* the peak sits, never bound the ratio at it. That splits the levers into
cosmetic and structural:

1. A relational `require`, e.g. `maxCumulativeReanchorFee < reservationMinAmount`
   — cheapest, and matches the `reservationMinAmount > reservationTxMaxFee`
   convention directly above it. **Cosmetic.** It moves the peak from
   `cap + floor` down to `minAmount + floor`, i.e. 99.51% -> ~95.8% on fixture
   values. Better, but still an unbounded-in-ratio loss, and the improvement
   shrinks as `reservationMinAmount` rises.
2. A per-reservation bound at acceptance as a fraction of `mintedAmount` —
   **structural.** Scales with the individual position, so it is one of only
   two levers that can state a ratio guarantee at all. Costs per-reservation
   state or recomputation.
3. Make the dust floor proportional to `mintedAmount` instead of the flat
   `reservationTxMaxFee` (`Reservation.sol:980-983`) — **structural, and the
   cheapest of the two**: it bounds the ratio directly at the one line where
   the constant is currently introduced. But it reverses an explicit design
   decision, not just a value: the comment at `:975-979` states the floor is
   "deliberately a dust floor rather than `reservationMinAmount`: a
   minimum-sized reservation must remain migratable." A proportional floor
   limits how far any position can migrate, so this needs the migratability
   requirement re-litigated, not merely a constant changed.
4. Leave it unbounded, matching `movingFundsTxMaxTotalFee` exactly, and rely
   on governance review. Defensible *given* that precedent, but then it
   should be recorded as an accepted risk rather than an omission — and the
   characterization test above is what keeps that record honest.

`reservationDissolutionTxMaxFee` sits outside levers 1-3 and supplies 1500 of
the constant. Bounding the re-anchor term alone leaves that residue, so a
ratio guarantee has to cover both fees, not just the grinding one.

**Not the same shortfall as `shortfall-design-space.md`.** That
document analyses the wallet-death case (anchor BTC inaccessible, `mintedAmount`
still outstanding) and its Space A/B/C framing turns on who pays when a wallet
dies. This item is the fee-loss shortfall on the *healthy* path: a reservation
that re-anchors and dissolves normally, with every party behaving. Same
direction of harm, different trigger, different bound — Space C's "priced up
front" answer does not reach it, because there is no failure event to price.

**Action:** pick among the four levers when #1093's backing model is reviewed —
item 5 already flags that review as the place to reconcile the two caps, and
this is the same conversation. If lever 4 (leave unbounded) wins, record it
explicitly: #1102 documents the tradeoff in its PR body and now emits
`ReservationDissolved(reservationKey, walletPubKeyHash, dissolutionTxHash,
mintedAmount, anchorAmount, dissolutionFee)`, so realized loss per dissolution
is measurable off-chain. That makes "monitor and revisit" a defensible answer
rather than an unexamined one — but only if someone is actually watching the
metric.

**RESOLVED 2026-08-23 for milestone 1: lever 4, plus a post-m1 structural
lever.** See "Resolution of items 5 and 7 for milestone 1" at the end of this
file. Item 5's open action ("confirm `#1093`'s backing model is compatible
with, or supersedes, `#1088`'s cap rather than silently stacking two different
caps") is closed by the same entry: it does neither, it drops the cap, and that
is now an explicitly accepted and test-pinned regression rather than an
omission. The "monitor the metric" answer above was superseded by something
stronger, a characterization test that fails if the ceiling is ever ported.

**Status at M1 code (2026-09-28):** the 2026-08-23 "Resolution" this item
points to is itself corrected below — the record there previously said
lever 4 was implemented verbatim; the true M1 code (`9f8f5ef1`) instead layers lever
4 with the item-5 request-time floor (`Reservation.sol:805-808`), which
changes the bound but not the "still unbounded in ratio" conclusion. See
the corrected Resolution section.

## Minor: the one #1102 finding left unimplemented
**Severity: Low (P3). Below this file's usual bar** — it is neither a new
mechanism nor a design decision, so by the scope rule at the top it would
normally be omitted. Recorded anyway because it is the only confirmed finding
from #1102's review that was deliberately not implemented, and "deliberately
not implemented" is worth distinguishing from "overlooked".

`notifyReservedRedemptionTimeout` and `notifyReservedRedemptionVeto`
(`Reservation.sol:757-774` and `:840-858`) each inline a similar
settle-and-reopen sequence, roughly ten duplicated lines. The review's own
recommended fix was conditional: *"consider extracting a shared
`_settleAndReopenReservation` helper if a third call site ever appears;
optional for this PR."*

Left as-is on purpose. The two blocks are not identical — the fault-flag
branching differs and the surrounding balance movement differs — so extracting
now would mean a helper with a parameter for each difference, which is the
usual way a premature abstraction ends up harder to read than the duplication
it replaced.

**Trigger, not a task:** if a third settle-and-reopen call site is ever added,
extract the helper at that point. Two call sites is duplication; three is a
pattern. No action while the count stays at two.

---

## Addendum: two items outside the original review's scope, surfaced by `d89a649a`

Neither of these came from the original 6-lens review or the existing
H-/M-/C- catalog — they're visible only because `d89a649a`'s commit
message names them directly. Not independently re-derived from first
principles here; flagging so they're on the epic's radar and don't fall
through a gap between two independent review passes.

**Redemption timeout/veto settlement race (labeled P0-1/P0-2 by whoever
fixed it).** Commit message: "Fix redemption timeout/veto settlement race
so a late-arriving SPV proof for an already-settled anchor spend is
acknowledged instead of reverting or double-crediting." A P0 label implies
this was originally a fund-safety-critical bug (double-crediting a
settlement). **Action: #1091 introduces its own two-phase
authorize-then-prove settlement model — that's structurally the same
shape of problem (a request phase and a later, possibly-delayed proof
phase can race). Whoever reviews #1091 should confirm it isn't
reintroducing the same late-proof race in its own settlement path, not
assume #1088's fix travels with it.**

**`RedemptionWatchtower` reserved veto state now keyed per request
generation, not the bare reservation key.** Commit message: "key
RedemptionWatchtower's reserved veto state per request generation instead
of the bare reservation key." This directly overlaps catalog item `M-03 —
Watchtower objection state not scoped per generation`, claimed closed by
**#1091**. Two independent fixes converging on the same gap (one on
#1088, one claimed on #1091) corroborates that M-03 is a real, correctly
scoped finding — lower-priority to double check than item 5's cap
overlap above, but worth the same "confirm compatible, not stacked"
treatment when #1091 is reviewed.


---

## Resolution of items 5 and 7 for milestone 1 (2026-08-23)

Roadmap-owner decision, recorded here because item 7's lever 4 requires that
choosing it be written down "as an accepted risk rather than an omission".

**Decided:** lever 4 for milestone 1 (leave the absolute ceiling unbounded),
plus a committed post-m1 work item for a structural lever. **Not** lever 1.

**Why not lever 1**, a flat governance-set `maxCumulativeReanchorFee`
equivalent: §7's own constant-backing result establishes that no bound
expressed on the fee caps can deliver a fractional guarantee. Porting one adds
a `ReservationRequest` storage field under the deployed TIP-109
upgrade-safety gate plus a parameter validated only `> 0`, and still leaves the
ratio unbounded. Cost without a guarantee.

**What `#1093` actually does**, verified against tip `e63b2a48`:
- No cumulative-fee field of any kind. Zero occurrences of anything resembling
  `cumulativeReanchorFee` in `solidity/contracts/`.
- Per-hop fee bounded by the snapshotted `action.txMaxFee`
  (`ReservationProofs.sol:643`).
- The only terminal bound is the dust floor `newAnchorAmount > action.txMaxFee`
  (`:651`), so cumulative loss is `initialAnchorAmount - (txMaxFee + 1)`. That
  scales with the claim rather than being capped at a constant.
- The loss is financed in kind, not socialised silently: the claim is written
  down (`:704`), the vault burns TBTC and Bank balance in lockstep, and any
  shortfall is recorded as `inKindFeeDebtSat` (item 8 below).

| | `#1102` | `#1093` |
|---|---|---|
| absolute ceiling | `maxCumulativeReanchorFee`, 100,000 sat | none |
| 10,000 sat claim | 4 hops, 8,000 sat lost | identical |
| 2,998,500 sat claim | capped at 100,000 sat | 2,598,499 sat lost, measured |

**The load-bearing assumption.** This is acceptable only because `#1093`
governance-gates every hop whose source wallet is `Live`
(`Reservation.sol:775-779`, with `privileged = msg.sender == governance` at
`ReservationRouter.sol:284`), and because a re-anchor target must itself be
`Live` (`:792-796`), so every hop after the first needs governance again. Item
7 scored this Medium on a Byzantine custodying wallet operator acting
unilaterally at "~zero cost, repeatable"; on `#1093` that operator cannot make
the loop run at all. **If that gate is ever relaxed, delegated, or extended to
non-`Live` source states, this acceptance is void and must be revisited.**

**How the record is kept honest.** Item 7 asked for a monitored metric; this
is stronger, an executable characterization test. In
`Bridge.ReservationSettlement.test.ts`, `describe("cumulative re-anchor fee
exposure (accepted regression)")`, added 2026-08-23:
- grinds one reservation at the maximum permitted fee per hop, **deriving** the
  hop count rather than hardcoding it, until the dust floor blocks settlement,
  and asserts cumulative loss of 2,598,499 sat, 86.7% of a 2,998,500 sat
  anchor, over 7 hops in a scaled fee regime;
- asserts that figure exceeds `#1102`'s 100,000 sat ceiling, so the test
  **fails if an absolute ceiling is later ported**, which is what makes the
  accepted regression executable rather than an assertion in prose;
- asserts the financing invariant exactly: reserve burn plus debt delta equals
  the cumulative fee, so nothing is unaccounted;
- separately asserts a third party cannot rotate a `Live` wallet's anchor,
  which is the tripwire for the assumption above.

Both cases pass on `#1093` at `e63b2a48`.

**Post-m1 commitment:** the ratio question moves to lever 2 or lever 3. Lever
3 is the cheaper of the two but reverses the explicit design decision at
`Reservation.sol:975-979` that the floor is "deliberately a dust floor rather
than `reservationMinAmount`: a minimum-sized reservation must remain
migratable". It therefore requires that migratability requirement to be
re-litigated, not merely a constant changed. Tracked in `roadmap.md`.

**Correction — status at M1 code (2026-09-28):** the "Decided" framing above
(written 2026-08-23 against the `#1093` stack tip) does not match what
actually landed in the M1 code (`reservations-upgrade` @ `9f8f5ef1`). Lever
4 ("leave the absolute ceiling unbounded") was not the final word: a fifth
mechanism this record didn't anticipate — a *request-time* amount floor,
`anchorAmount > reservationTxMaxFee + reservationMinAmount`
(`Reservation.sol:805-808`, from `d600a8bf`, a P3 review-remediation fix,
not a roadmap decision) — now gates every re-anchor request before it is
even authorized, firing well before the settlement-time dust floor `#1102`/
`#1093` characterized. `Bridge.ReservationSettlement.test.ts`'s "bounds the
grind at a request-time amount floor, not the settlement-time dust floor"
confirms this directly: a real, substantial loss still occurs — well above
`#1102`'s 100,000 sat fixture ceiling, and its ratio scales with
`reservationMinAmount` relative to the claim rather than being a protocol
constant — but it is bounded, not unbounded. In lever terms this lands
closest to lever 1, keyed off `reservationMinAmount` instead of a
dedicated cap, and arrived at through the general review-remediation pass
rather than through this decision. The **"Post-m1 commitment"** paragraph
below (moving the ratio question to lever 2 or 3, post-m1) is superseded in
part: a structural bound already exists in m1. Whether it satisfies the
original fractional-guarantee ask depends on governance's
`reservationMinAmount` choice relative to typical claim sizes — that
narrower question is the live successor to this commitment, not the
original "unbounded vs. bounded" one.

## 8. Aggregate in-kind fee debt is unbounded and only voluntarily repayable
**Severity: Medium. Provenance: step-3 review of `#1093`, 2026-08-23.** Not in
the H-/M-/C- catalog. Separate from item 7 and not resolved by it: item 7
bounds what a single reservation contributes, this is the sum across every
reservation that ever re-anchors or dissolves.

Verified from code at `#1093` tip `e63b2a48`:
- `uint64 public inKindFeeDebtSat` (`ReservationVault.sol:122`) is one
  **global** counter. No ceiling field exists anywhere in
  `solidity/contracts/`.
- `financeInKindFee` (`:529-561`) burns what the reserve can cover then
  accumulates the remainder unconditionally (`:556`). It never reverts, by
  design (`:523-528`): a confirmed Bitcoin spend must not fail to settle
  because of reserve level. While the debt is non-zero the system is
  over-supplied by exactly that amount.
- **There is no debt-first logic.** Fee revenue arriving while debt is
  outstanding simply becomes reserve; nothing applies it to the debt.
  Initiation fees (`:233-245`), redemption fees (`:308-313`) and renewal fees
  (`:386-398`) each "stay in the vault" per their own comments.
- `sweepFees` (`:613-624`) is `onlyOwner` and manual, and moves only
  `balance - feeReserveTarget` to the treasury. It never reads or writes the
  debt.
- `repayInKindFeeDebt` (`:568-586`) is the only path that reduces the debt. It
  is voluntary and permissionless, with no keeper obligation anywhere in the
  contract.
- No invariant, assertion, or test in `solidity/test/` observes or bounds
  `inKindFeeDebtSat`.

Economic inference, flagged as inference: settlement fees are mandatory and
system-driven (every re-anchor `ReservationProofs.sol:709-712`, every
dissolution `:822-825`) and Bitcoin miner fees are exogenous, while every
inbound fee is user-elective. So no code path and no evident economic force
drives the debt toward zero.

The `uint64` type is not a practical bound: its ceiling is about 1.8e19 sat,
roughly 8,800 times the total Bitcoin supply and far above the protocol's own
`RESERVATION_MAX_TOTAL` of 2.1e15 sat.

**Independent of the sweep decision.** `vault.md`'s open "DECISION NEEDED" on
sweep safety does not gate this. Sweep operates on `balance - feeReserveTarget`
and never touches `:556`, so even a fully automatic sweep leaves the debt
dynamics unchanged. The decision this actually needs is a different one, absent
from the code entirely: **should arriving fee revenue reduce outstanding debt
before it counts as reserve?**

**Action:** decide that question, and note that reverting on settlement is
explicitly off the table so the answer cannot be a `require`. Cheapest
credible options: apply arriving fees to outstanding debt before retaining
them as reserve; or add a governance-set alarm threshold that emits rather than
blocks. Until then, add an observability assertion on the debt to the item 7
characterization test, so growth is visible in CI rather than discovered later.

**Re-verified 2026-09-07 against live `reservations-upgrade` (tip
`52bf2822`):** the "no debt-first logic" claim above is stale.
`ReservationVault.sol` relocated from `bridge/` to `vault/` since the
`e63b2a48` reference, and `sweepFees` (`:377-396`) now burns down
`inKindFeeDebtSat` from the reserve before releasing any excess above
`feeReserveTarget` to the treasury — debt-first ordering already exists,
gated on governance calling `sweepFees`. `financeInKindFee` still
accumulates debt unconditionally with no ceiling (unchanged), and nothing
repays it between sweeps, but the sweep itself is no longer "manual and
unrelated to the debt."

**Further correction 2026-09-07 (same session):** the debt-first mechanism
described above is inert once debt is large. `_burnFromReserve` (`:546-561`)
burns `min(debt, full current balance)` with no floor at `feeReserveTarget`,
and `sweepFees`'s `require(balance > feeReserveTarget)` (`:391`) runs on the
*post-burn* balance. A Solidity revert unwinds the whole call, so whenever
debt is large enough that repaying it would drag balance to or below
`feeReserveTarget`, `sweepFees` reverts entirely — undoing the repayment
attempt too. The debt-first mechanism only functions in the low-debt
regime; it is completely inert in the high-debt regime this item exists to
address.

**Decided 2026-09-07 (session decision, via `unblock-gaps`, corrected
twice):** fix `_burnFromReserve`'s call site in `sweepFees` to cap debt
repayment at the reserve target — repay `min(debt, balance -
feeReserveTarget)` instead of `min(debt, balance)`. A large debt then
degrades to slow partial repayment across many sweeps rather than making
`sweepFees` revert outright; ordinary fee sweeping keeps working regardless
of debt size. tbtc-v2 implementation pending, separate repo/scope. Add the
observability assertion from the Action paragraph above regardless.

**Scope note for the implementer:** this caps *repayment* against the
target, not *financing* — `financeInKindFee` is unchanged and still burns
the reserve down to zero to cover miner fees unconditionally (that is how
debt arises in the first place). If fee income stops entirely, debt never
fully clears under the capped rule either, since repayment is bounded by
whatever sits above the target. The regression test needs both cases: the
previously-reverting scenario now succeeding with partial repayment, and
the no-excess scenario where repayment is correctly a no-op rather than a
revert.

**Status at M1 code (2026-09-28):** the "Decided ... fix `_burnFromReserve`'s
call site... repay `min(debt, balance - feeReserveTarget)`" plan above did
**not** get implemented as specified. What was implemented instead
(`ReservationVault.sol:297-319`, `_burnFromReserve` at `:354-372`):
`sweepFees` now repays `min(debt, full reserve)` unconditionally — not
capped at `feeReserveTarget` as decided — and then simply `return`s without
transferring anything if the post-repayment balance is at or below
`feeReserveTarget`, instead of the old `require` that used to revert the
whole call. This closes the "further correction" bug (`sweepFees` no
longer reverts in the high-debt regime) by a different route than
specified: it is *more* aggressive than "Decided" asked for (debt
repayment can now drain the reserve below `feeReserveTarget`, where the
decided design meant to protect that floor), not merely unstuck. The
underlying finding's core claim is unaffected: `financeInKindFee`
(`:215-234`) still accumulates debt unconditionally with no ceiling
(confirmed unchanged at `9f8f5ef1`), so "aggregate debt is unbounded"
stays OPEN even though "the repayment mechanism can get stuck" is now
CLOSED, just not via the mechanism this record specified.

## 9. Permissionless re-anchor requests let an outsider dictate migration targets
**Severity: Low, rising to Medium for any wallet operator whose signing policy
forbids executing a transaction it did not itself propose. Provenance: step-3
review of `#1093`, 2026-08-23.** Not in the catalog, not raised by `#1102`.

`requestReservationReanchor` is permissionless when the source wallet is
`MovingFunds` (`Reservation.sol:780-784`; router entrypoint
`ReservationRouter.sol:277-286` has no modifier), and nothing checks that the
caller owns the reservation. A request requires `state == Active` and moves the
reservation to `ActionPending` (`:760-763`, `:819`), and the re-anchor timeout
returns it to `Active` with no slashing or penalty (`:1003-1009`) — but,
unlike when this item was written, a timed-out re-anchor now sets
`reservation.reanchorCooldownUntil = now + (action.timeoutAt -
action.requestedAt)` (~48h, `Reservation.sol:904-907`), and
`requestReservationReanchor` enforces `block.timestamp >=
reservation.reanchorCooldownUntil` for non-privileged callers
(`:736-741`). So the original claim — that one contract can combine the
timeout and a fresh request in a single transaction, making the
reservation "never observably `Active` to a competing caller" — no longer
holds: after a timeout, the same non-privileged attacker is gated by the
same ~48h cooldown as anyone else before it can re-request. See the
status note below.

**This is not a liveness block.** The attacker's pending action is itself a
fulfillable authorization: the operator can build the re-anchor transaction to
the named `Live` target and have the SPV maintainer prove it, which settles,
decrements the source wallet's reservation count
(`ReservationProofs.sol:683-686`) and clears
`require(walletReservationsCount == 0, "Wallet still custodies reservations")`
at `Wallets.sol:674-677`, `:706-709` and `:437-441`. The attacker cannot
shorten that window, because `notifyReservationActionTimeout` requires
`block.timestamp >= action.timeoutAt` (`Reservation.sol:884-885`; line
drifted from the `:966-971` originally cited, and the operator is `>=` not
`>`), so every cycle hands the operator a full 48h settleable window.

What survives is narrower: the **target** of every executable migration is
chosen by the attacker rather than the operator for as long as the attacker
pays gas; a passive operator's own retirement stalls one 48h cycle at a time up
to `dissolutionEligibleAt` (`:765-768`); and repeated naming of one target can
steer several migrating reservations onto a single wallet up to
`maxReservationsPerWallet`. No funds move and nothing leaves the protocol,
since every target must be a registered `Live` wallet. The attacker also cannot
create the precondition, as `MovingFunds` is a protocol-driven transition.

**Action:** document the operational escape first, because the intuitive
response to an unsolicited request is to refuse, and refusing is exactly what
turns this from Low into Medium. The cooldown-after-timeout lever below was
evaluated and implemented; what remains open is bounding re-anchor
generations per reservation. Note that restricting the permissionless path
itself would reverse a deliberate design decision asserted by
`Bridge.ReservationStrandingLibrary.test.ts:1179,1226` ("successfully
requests reservation re-anchor from MovingFunds/Closing wallet (unprivileged
happy path)"), so that route needs the design decision re-litigated first.

**Documented 2026-09-07 (session decision, via `unblock-gaps`) — the
operational escape:** if a wallet operator receives an unsolicited
`requestReservationReanchor` naming a target it did not choose, the correct
response is to **execute it**, not refuse it. The request is a fulfillable
authorization with no funds at risk (every target must be a registered
`Live` wallet, and the source-wallet transition itself is protocol-driven,
not attacker-created) — refusing is what turns this finding from Low into
Medium, since a refusal-driven stall is what actually blocks the operator's
own retirement path. Evaluating a bound on re-anchor generations per
reservation remains open; restricting the permissionless path itself would
reverse the deliberate design decision now asserted by
`Bridge.ReservationStrandingLibrary.test.ts:1179,1226` (the
`Bridge.Reservation.test.ts:3473` citation this note originally used no
longer exists — that file was split, see `testing-plan.md` §1) and needs
that decision re-litigated first, independent of this note.

**Status at M1 code (2026-09-28):** NARROWED, not closed. The cooldown
lever this item's own "Action" paragraph proposed as post-m1 work
(`reanchorCooldownUntil`, `Reservation.sol:904-907`/`:736-741`) is already
implemented, confirmed by `Reservation.test.ts:1607-1612` ("Reanchor
cooldown is set to block.timestamp + (action.timeoutAt -
action.requestedAt)") and `Bridge.ReservationStrandingLibrary.test.ts:1608-1609`
(asserts revert `"Reanchor cooldown in effect"`). The attacker can still
eventually re-grab the target after the cooldown elapses — the finding's
"target chosen by attacker for as long as it pays gas" conclusion survives
in a throttled form — but the "atomic timeout-plus-re-request, reservation
never observably Active" mechanism this item's severity was built on is
gone. The remaining bound-on-generations question stays open.

**Caveat:** "execute it" depends on the SPV maintainer actually submitting
the re-anchor proof — the step item 3 above just accepted as a live stall
risk with no permissionless fallback. The operational escape is only
available while the maintainer is responsive; a stalled maintainer
collapses this finding back to the Medium case even for a fully
cooperating operator, since the operator can build the transaction but
cannot submit it themselves.
