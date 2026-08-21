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
*slashes the wallet* (`notifyWalletRedemptionTimeout` →
`ecdsaWalletRegistry.seize`) and can terminate an entirely honest,
unresponsive-maintainer-blocked wallet, stranding every position it
custodies. Enumerate the mainnet `isSpvMaintainer` set before launch;
`MaintainerProxy.sol` wraps the four pooled-path proofs but has no
`submitReservationProof` wrapper, so reservation settlement may have no
mainnet submitter wired at all as of #1091 — verify against #1094/#1095.

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

## 5. Unbounded re-anchor grinding — RESOLVED directly in #1088
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
`Bridge.Reservation.test.ts` → "leaves residual backing independent of claim
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

