# Reservation Shortfall - Design Space

Status: DRAFT analysis. Not scoped in any of the 9 reservation PRs. Companion to

**Not adopted and not scoped (2026-08-21).** Space C is viable only conditional
on an unbuilt `anchorAmount`/`mintedAmount` decoupling, and milestone 1 is
variant B, which builds none of it. Retained as the who-pays analysis that rules
out Spaces A and B. The accepted loss-story build is
`stranding-compensation-proposal.md` Tiers 0-1.
`feature-spec.md` (what the PRs build),
`stranding-compensation-proposal.md` (Space A, now rejected here), and
`exit/proposal.md` (recovery under wallet failure, unaffected).

Records two agent retractions, in §2 and §4.2. Both are noted where they land so the rejected
reasoning does not get re-proposed.

## 1. The question, and why it is forced

When a reservation's wallet dies, the anchor BTC is inaccessible but the `mintedAmount` tBTC still
exists. Something must happen to that tBTC. There is no option in which nothing happens.

Today (`notifyReservationStranded`, #1094) the depositor keeps it as an ordinary pooled claim and
the shortfall is socialized across all tBTC holders. That is a choice, not a default, and it is
the choice under review.

Three regions, by who bears the loss:

| | Who pays | Verdict |
|---|---|---|
| A | The network (all tBTC holders) | **Rejected** - §2, no funding source scales with exposure |
| B | The depositor, in full, by extinguishing the claim | **Rejected** - §3, one variant unenforceable, the other product-fatal |
| C | The depositor, in part, priced up front | **Viable with an explicit rule** - §4 |

## 2. Space A - the network bears it

This is today's behavior plus a compensation module to repair the damage. It requires a funding
source that scales with BTC at risk. **No such source exists in tBTC.**

Every slashing amount is a flat `uint96` T quantity with no BTC term:

| Parameter | Value | Source |
|---|---|---|
| `groupSize` | 100 | `EcdsaDkg.sol:140` |
| `fraudSlashingAmount` | 100 T | `Bridge.sol:338` |
| `movingFundsTimeoutSlashingAmount` | 100 T | `Bridge.sol:329` |
| `movedFundsSweepTimeoutSlashingAmount` | 100 T | `Bridge.sol:334` |
| `redemptionTimeoutSlashingAmount` | 100 T | `Bridge.sol:323` |

Maximum seizure is 100 members x 100 T = **10,000 T**, invariant to the 50 BTC at risk. At T
around $0.03 and BTC around $100k that is roughly **0.006%** of a single-wallet stranding. At T =
$1 it is 0.2%. The conclusion does not depend on the price assumption.

**Parameter regime (added 2026-08-21).** The values above are
`feature-spec.md` §10's **steady-state provisional** figures, not the m1 launch
posture. At m1 launch `roadmap.md` §1.4 sets `maxReservationsPerWallet = 1` and
a tiny `reservationMaxTotalAmount`, so a single termination strands **one**
position. The correlated-batch argument therefore applies from the first cap
raise onward, not at launch — but Tier 0 must record per-position from day one,
because batching becomes observable only after the raise.

**The cheapest attack recovers nothing at all.** `notifyOperatorInactivity`
(`WalletRegistry.sol:1078-1129`) applies exactly one penalty:
`sortitionPool.setRewardIneligibility(ineligibleOperators, block.timestamp + _sortitionPoolRewardsBanDuration)`
at line 1111. No `seize`, no stake touched. Going dark costs forgone rewards for a ban period.

Fees are no better: at full utilization of the 100 BTC global cap, first-year income is about 0.4
BTC against 50 BTC of per-wallet exposure, roughly 0.8%.

So Space A is rejected **on funding invariance, not on fee arithmetic**. The distinction matters:
"fees are too small" invites "then fund it from slashing," and slashing is worse. The compensation
module remains useful as an accounting and payout *rail* (Tier 0 liability accounting and Tier 1
fee restitution are both cheap and worth doing), but it cannot be the answer to solvency.

**Retraction.** An earlier claim that reservations "sit on top of" a wallet's economic security
budget was wrong. No such budget exists. A 100 T penalty is not economic bonding, so the main UTXO
is not stake-protected either; it rests on a 51-of-100 honest-majority assumption plus fraud
detection. Anchors inherit exactly that. Reservations breach no ratio because no ratio exists.

What reservations do add is narrower and still real: an **individually attributable** loss that
gets socialized, and a depositor-side payoff from wallet failure that did not previously exist.

The mechanism is not that the depositor holds a claim through the failure. It is that **the
depositor can sell the minted tBTC and walk away before the failure lands.** The tBTC is an
ordinary ERC-20 from the moment it is minted, so a depositor may lock 50 BTC, mint 50 tBTC, sell
it at parity, and hold no position at all when the wallet dies. The anchor freezes, the buyer's
tBTC loses its backing, and the depositor is out only the 40 bps fee.

| | Attacker cost | Backing destroyed | Ratio |
|---|---|---|---|
| Today, claim-equals-anchor | 0.2 BTC (fee) | 50 BTC | **250 : 1** |
| Space C at 80% LTV | 10.2 BTC (unborrowed + fee) | 40 BTC | 4 : 1 |

So the locked Bitcoin is **not** the depositor's capital at risk under the current design; it
becomes the buyer's. This is the same fungible-mint leak as B1 in §3, seen from the funding side
rather than the enforcement side, and it is why an unborrowed fraction is the only thing that puts
real skin in the game.

What this is **not** is a new theft surface, and an earlier verbal claim by agent that operators
could "reserve into their own wallet, stop signing, and keep the tBTC" was wrong on two counts.
Without a signing coalition the attack is self-harming: the anchor freezes with the operator's own
Bitcoin inside it, so they spend 50 BTC to destroy 50 BTC. With one, they can already take the
main UTXO, which is larger. And the coalitions are nearly identical: `groupThreshold = 51` of
`groupSize = 100` (`EcdsaDkgValidator.sol:45,51`), so **denying a signature takes 50 operators and
producing one takes 51.** Anyone able to kill a wallet can almost certainly rob it instead, for
more. Reservations add a smaller target under an unchanged trust assumption.

The surviving concern needs no coalition at all, which is what makes it serious: it is adverse
selection. A depositor pays for segregated custody but bears none of its failure, so nothing
draws reservation demand toward reliable wallets, and the fee cannot be priced against risk the
depositor does not carry.

## 3. Space B - the depositor bears it in full

Requires the claim to be extinguished or never fungible.

**B1, burn `mintedAmount` on stranding. Unenforceable.** tBTC is a freely transferable ERC-20 and
stranding is publicly predictable (a wallet goes quiet, heartbeats fail, timeouts elapse), so the
holder sells first and the burn hits nobody. This is the same wall that killed Mechanism 3 in
`exit/addendum.md` §A.3, in a different costume: **a clawback against a
transferable token with a publicly observable trigger is not enforceable.** Third time this
constraint has decided a design in this feature.

**B2, never mint fungible tBTC; issue a non-transferable receipt. Product-fatal.** Structurally
this is the strongest option in the whole space. It would delete the escrow machinery, the
committee, the compensation module, and the entire double-claim class, because a claim that can
only ever be satisfied from its own anchor cannot be redirected at the pool.

It is nonetheless dead on the product. The originating proposal (tbtc-v2 #911, closed 2026-06-22
with no review comments) is titled "feat: non fungible", but that refers to the **UTXO** being
specific, not the claim being non-transferable. Its stated goal is *tax-efficient custody*:
reserve specific UTXOs "while still being able to mint and use tBTC tokens." The tax argument
needs both halves - never disposing of the specific coins, and usable liquidity. B2 keeps the
first and destroys the second, so it removes the reason the feature exists.

**Retraction.** Agent previously recommended B2. That recommendation is withdrawn on the product
grounds above. B2 was never designed by anyone; the fungible-mint leak has been in this feature
since its first proposal.

## 4. Space C - the depositor bears part of it, priced up front

#911's own framing points here. Tax-efficient structures are **loans**, and loans are
overcollateralized. So mint less than the anchor: lock 20 BTC, mint 14 tBTC at 70% LTV.

Fungible tBTC survives, so the product and the tax argument survive.

**This requires an accounting change upstream of everything else in this section.** Today
`anchorAmount` and `mintedAmount` are not independent fields; every proof, whole or partial,
drops both by the same `redeemAmount` (`feature-spec.md` L325-327), which only stays
consistent if they started equal. Space C's premise is that they start unequal - `anchorAmount =
20`, `mintedAmount = 14` - a state the current contracts have no representation for. Both
redemption paths would need `anchorAmount` and `mintedAmount` to move by different amounts on the
same proof, which is a change to the core reservation struct and both settlement paths, not a
parameter. §4.4's Route 1 vs Route 2 question is downstream of this and does not remove it; it
decides how the *assessment* is denominated once decoupled accounting already exists.

### 4.1 What this achieves automatically

Once decoupled accounting exists (§4's opening paragraph), stranding costs the depositor their
surplus with nothing further built. At 70% LTV a depositor who strands has locked 20 and keeps
14, losing 6. **The free option of §2 is closed**, and beyond the accounting change itself, this
part needs no governance action and no cooperation from anyone.

The systemic hole shrinks proportionally, from `X` to `X * LTV`.

### 4.2 What it does NOT achieve, and this was agent's error

**A position's surplus dies with its own anchor.** The surplus was never held aside; it was the
excess BTC *inside* that anchor. Ledger, healthy pool `P = S`, one position, LTV 0.7:

```
open:            backing P+20, supply S+14   ->  surplus +6
after strand:    backing P,    supply S+14   ->  deficit -14   ( = X*LTV, not 0 )
```

The `LTV <= 1 - c` invariant of §4.5 is derived correctly, but it only asserts **aggregate**
backing at the instant of stranding, and it does so using surplus contributed by *other* open
reservations. That surplus is **depositor equity, not protocol equity.**

So the buffer is transient. Trace it with `c = 0.5`, `LTV = 0.5`, `L = 100` split between A and B:

```
open:                    backing P+100, supply S+50  ->  surplus +50
A strands (X=50):        backing P+50,  supply S+50  ->  surplus   0    invariant holds
B redeems in-kind:       backing P,     supply S+25  ->  deficit -25    ( = X*LTV )
```

B always held a complete claim on B's own BTC, having borrowed only 25 against 50. Exercising it
is honest behavior, not an attack, and it takes the buffer with it. **The first honest in-kind
redemption after a stranding reopens the full `X * LTV` hole.**

Presenting automatic full absorption as a property of Space C would have been a third retraction.
The durable automatic effect is the proportional reduction in §4.1 and nothing more.

### 4.3 Full absorption requires an explicit seizure rule

To close the remainder, the protocol must take the remaining reservation holders' surplus at
stranding time. Unlike B1 this **is** enforceable, because the position lives in contract storage
and settlement runs through the contract. There is no transferable-token escape.

Rule: on stranding of `X`, assess `X * LTV` across all open reservations pro-rata by locked
amount, capped at each holder's own surplus.

Verified against the ledger: at the invariant boundary the assessment closes the hole exactly, and
below the boundary it closes with headroom.

| `c` | LTV | Deficit | Remaining surplus | Final | Surplus consumed |
|---|---|---|---|---|---|
| 0.10 | 0.90 | 9.00 | 9.00 | 0.00 | **100%** |
| 0.10 | 0.80 | 8.00 | 18.00 | 0.00 | 44% |
| 0.50 | 0.50 | 25.00 | 25.00 | 0.00 | **100%** |
| 0.50 | 0.70 | 35.00 | 15.00 | **-20.00** | 100%, insufficient |

### 4.4 Settlement denomination - Route 2 was wrong, not just weaker

Two ways to implement the assessment were proposed. Route 2 does not survive checking against the
spec it claimed to reuse, and this is agent's second wrong claim about this feature.

**Route 1, raise the tBTC repayment.** The depositor receives every one of their own satoshis and
burns more tBTC to get them, which keeps 1-to-1 lineage perfectly intact and reads naturally in
the loan framing, where repayment is denominated in the borrowed asset. Cost: the assessed holder
may owe tBTC they do not hold and must acquire (§4.8's nine each need 0.89 beyond what they
minted), which is a real disposal and needs the `k` premium haircut of §4.5.

**Route 2, take the assessment from the anchor, was claimed to reuse existing machinery. It does
not.** `requestPartialReservedRedemption` (#1096) is the wrong shape for what Route 2 needs, on
the points checked against `feature-spec.md` §4.2:

- Its remainder **stays in the same reservation** - "output 1 re-anchors the remainder to the
  custodying wallet ... position stays `Active` on the new remainder outpoint" (L321-327). Route 2
  needs the assessed BTC to leave the reservation and become pooled backing. #1096 has no path
  where value exits to the pool; it only ever moves between a redeemer and the same position.
- Reserved BTC becoming pooled backing is not merely unbuilt, it is **explicitly walled off**:
  "a reserved deposit is never swept into the pooled main UTXO (`DepositSweep` rejects deposits
  with `pendingReservedDeposit.isReserved == true`)" (§14 invariant 3). Route 2 needs the reverse
  direction of exactly this rule.

A third point, checked and then withdrawn: `redeemAmount = 9.11 > mintedAmount = 8.00` does fail
today's `redeemAmount < mintedAmount` guard (L329-330), but that is **not evidence against Route 2
specifically.** That guard, and the "claim-equals-anchor to the satoshi" wording next to it
(L321-323), describe the current *lockstep* mechanics where BTC-released and tBTC-burned are the
same field by construction. §4's opening paragraph already established that Space C breaks this
lockstep for **both** routes - Route 1 also releases the full anchor while burning more than
`mintedAmount`, which the same guard would equally reject unmodified. So this is the shared
accounting prerequisite showing up again, not a Route-2-specific defect, and it does not belong
in this list.

So Route 2's true cost is not "returns fewer coins than were locked" as first written. It is
**a deliberate breach of segregated custody for the assessed holder** - the one person in the
scenario who did nothing wrong - plus an entirely new reserved-to-pooled transfer path that does
not exist and that the design currently forbids in the direction Route 2 needs. That is a bigger,
not smaller, engineering and trust cost than Route 1's market-acquisition friction, and it is a
cost specific to Route 2: Route 1 never asks BTC to leave a reservation for the pool at all.

**Route 1 is the route that does not require breaking the reserved-versus-pooled wall.** Both
routes equally require the shared accounting prerequisite of §4. Only Route 2 additionally
requires reserved BTC to become pooled backing, which §14 invariant 3 forbids outright and which has no
existing or partial substitute in `#1096`. Route 1's residual cost (§4.5's disposal and `k`
premium) is paid by the assessed holder in the open market; Route 2's cost is paid by the
protocol, in the segregation guarantee itself. Recorded as resolved, not open: use Route 1, and
treat Route 2 as requiring a from-scratch reserved-to-pooled transfer design if ever revisited.

### 4.5 The invariant, and what it actually means

With `c` = the largest share of total reserved BTC sitting in any one wallet, remaining holders'
surplus covers the deficit iff:

```
(1-c)L(1-LTV) >= cL*LTV   <=>   LTV <= 1 - c
```

Same inequality as before, but its meaning is now correct: **it is the solvency condition for a
mutual-insurance scheme, not a description of automatic behavior.** It tells you when the §4.3
rule can succeed.

Repayment stays rational for an assessed holder iff the assessment does not exceed their surplus.
Exercising is worth `p - A`, abandoning is worth `p * LTV`, so exercising wins iff
`A <= p(1-LTV)`. The §4.3 cap is exactly this bound, so default is never rational under the rule.
No lower bound on LTV follows from this.

**Under Route 1 that bound needs a haircut.** It assumes the assessment is denominated in an asset
the holder already values at parity, but §4.4 shows they may have to buy the tBTC. If tBTC trades
at a premium `k` to BTC when they redeem, the true cost is `A * k` and the condition becomes
`A <= p(1-LTV)/k`. So the §4.3 cap is exactly right under Route 2 and slightly **too generous**
under Route 1, and the shortfall is worst precisely when the assessment itself is bidding tBTC up.

At the boundary `LTV = 1 - c`, absorption consumes **100% of every innocent depositor's surplus**.
Real headroom requires LTV strictly below `1 - c`.

### 4.6 The bootstrapping hole

Mutual insurance is undefined for the **first** reservation. With one open position `c = 1`, so
`LTV <= 0` and there is nobody to assess. More generally the scheme is weakest exactly when the
feature is youngest and utilization is low, because `c` is measured against *actual* locked BTC,
not against the cap. Either LTV is set for the worst case, or utilization is gated, or early
positions run without the absorption guarantee. Unresolved.

### 4.7 The fairness cost, stated plainly

Under §4.3, **innocent reservation depositors mutually insure each other.** A depositor whose own
wallet is perfectly healthy pays when someone else's wallet dies, up to their entire surplus.

That is defensible - they opted into the same feature and benefit from its existence - but it is a
materially different product than "your coins, segregated, safe," and it must be disclosed rather
than discovered. It also means the honest pitch for Space C is not "no socialization." It is
"socialization confined to the feature's own users instead of all tBTC holders."

### 4.8 Worked example

Ten depositors, 10 BTC each, LTV 80%, so 100 BTC locked and 80 tBTC minted. Ledger verified.

| Step | Backing | Supply | Aggregate |
|---|---|---|---|
| Ten positions open | +100 | +80 | +20 |
| Alice's wallet dies, her 10 BTC unreachable | +90 | +80 | **+10** |
| The other nine redeem in-kind, no assessment | 0 | +8 | **-8** |
| The other nine redeem under §4.3 | 0 | 0 | **0** |

Alice's automatic loss is the 2 BTC she did not borrow against, which is what closes the free
option of §2. Under today's design she would mint the full 10 and lose nothing.

Row 2 is the trap of §4.2: the `+10` looks solvent, but it is the other nine depositors' own
unborrowed Bitcoin sitting in their own live anchors. Row 3 is nine honest depositors each taking
back coins they always fully owned, and it drains the buffer to exactly `X * LTV`.

Row 4 applies the rule: `8.00 / 9 = 0.89` tBTC added to each remaining holder's repayment, against
a per-holder cap of 2.00, so 44% of the buffer is consumed and the books close at zero. Bob gives
10 BTC, gets his same 10 BTC back, borrowed 8.00 and repays 8.89, netting **-0.89**. That 0.89 is
the mutual-insurance premium of §4.7, and under Route 1 it is also the tBTC he must go and buy.

## 5. Parameter consequences

The per-wallet cap is the real lever, and it is currently set to the worst available value.

| | Today | Implication |
|---|---|---|
| Global cap | 100 BTC | |
| Per-wallet cap | 50 BTC | `c` can reach **0.5**, forcing `LTV <= 50%` |
| Per-wallet cap at 10 BTC | | `c` = 0.1, permits `LTV <= 90%` |

The spec's stated rationales confirm the caps were never sized for this: the global cap targets a
fraction of total backing (systemic dilution), and the per-wallet **count** cap of 10 is justified
as bounding re-anchor ceremonies in a rotation window (operational load). The 50 BTC per-wallet
amount cap has no stated rationale at all. None of these is a per-wallet loss-concentration
argument, which is what `c` measures.

## 6. What Space C fixes, and what it does not

Fixes, once the accounting prerequisite of §4 is built:

- The depositor-side free option on wallet failure (§4.1).
- Proportional reduction of every stranding, `X -> X * LTV`.
- The funding-scale problem of §2: the buffer scales with exposure by construction, which neither
  fees nor slashing can do.

None of the above is free. §4's first paragraph is the gate: `anchorAmount` and `mintedAmount`
move in lockstep today (spec L325-327), so every item here needs that decoupled first, in the
reservation struct and in both the whole and partial redemption paths. Only what happens *after*
that change is automatic.

Does not fix:

- **The accounting prerequisite itself (§4).** Decoupling `anchorAmount` from `mintedAmount` is a
  change to core state and both settlement paths, not a parameter, and it precedes every other
  item in this section.
- Full absorption without the explicit rule in §4.3, and that rule shifts loss onto innocent
  depositors rather than eliminating it.
- The first-reservation and low-utilization case (§4.6).
- Operator theft of an anchor. That remains a 51-of-100 honest-majority question, identical to the
  main UTXO, with the same negligible slashing recovery.
- The in-kind guarantee under wallet failure. That is the emergency exit's job and is orthogonal.

## 7. Open questions

- **LTV value.** Anything at or above `1 - c` fails. Bitcoin-backed lending conventionally runs
  30-50%, which today's `c = 0.5` happens to permit, but that coincidence disappears if the
  per-wallet cap is tightened.
- **Is the §4.3 assessment wanted at all?** Space C without it is simpler, honest, and still
  closes the free option; it just leaves a smaller socialized hole. Mutual insurance may be worse
  product than a bounded socialized loss.
- **Does the assessment need a liquidation path** if an assessed depositor never redeems? The
  position simply sits, which may be acceptable.
- **Settlement route is resolved, not open (§4.4).** Route 1 (raise the tBTC owed) is the
  decision; Route 2 (take the assessment from the anchor) needed a reserved-to-pooled BTC
  transfer that #1096 does not provide and that the segregation invariant
  (`feature-spec.md` §14 invariant 3: "a reserved deposit is never swept into the pooled
  main UTXO") explicitly forbids. Route 1's residual cost is the assessed holder's market
  acquisition and the `k` haircut of §4.5, a market friction rather than an invariant break, and
  is the correct tradeoff to accept.
- **Counterparty concentration under Route 1.** In accounting terms the only tBTC in excess of
  backing after a stranding is the stranded position's own `X * LTV`, so it is tempting to
  conclude the strander becomes a monopoly seller who can hold out for rent. That does **not**
  follow while reservations are a small share of tBTC supply, because tBTC is fungible and the
  assessed holders buy from the whole market at market price, not from a named party. It does
  become real in exactly the low-utilization regime of §4.6, where reserved BTC is a large
  fraction of supply, so the two questions should be answered together rather than separately.
- **Tax treatment of the Route 1 disposal.** Raising the tBTC owed forces the assessed holder to
  acquire and dispose of tBTC, which may carry tax consequences distinct from redeeming their own
  coins. A question for someone qualified, not for this document.
- **Is `c` enforced or observed?** The invariant needs actual concentration, so either the
  contract tracks it and blocks reservations that would breach it, or governance monitors it
  off-chain the way the reserved-fraction target already is.
