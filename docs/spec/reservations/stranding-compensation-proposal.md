# Reservation Stranding Loss Absorption - Design Sketch

Status: **SUPERSEDED IN PART.** This document is "Space A" (the network bears the shortfall) in
`shortfall-design-space.md`, which **rejects Space A as the answer to solvency** on
funding invariance: every slashing amount in tBTC is a flat `uint96` T quantity with no BTC term
(max seizure 10,000 T against 50 BTC of exposure, roughly 0.006%), and the cheapest stranding
attack, going dark, triggers no seizure at all - `notifyOperatorInactivity` only calls
`setRewardIneligibility` (`WalletRegistry.sol:1111`). No funding source scales with BTC at risk.

Read the §3 fee arithmetic below as *one instance* of that general result, not as the argument.
"Fees are too small" invites "then fund it from slashing," and slashing is worse.

**What survives and is still worth building:** Tier 0 (cumulative stranded-liability accounting)
and Tier 1 (fee restitution) from §10. Both are cheap, both are useful under any design in the
space, and Tier 0 is what produces the stranding-frequency evidence needed to decide anything
else. The §4 buyback remains a sound *rail* for discretionary governance spending; it is simply
not a solvency mechanism.

Original framing follows. Fills the gap the spec flags as "no stranding-compensation mechanism
beyond an event stub" (`feature-spec.md` §7 H-06, §15, §16). Companion to
`exit/proposal.md`, which addresses the same failure from the other side.

Named "loss absorption" rather than "compensation" deliberately - §2 explains why the
compensation framing points at the wrong victim.

## 1. What exists today

`notifyReservationStranded` (permissionless, wallet must be `Terminated`) moves the position to
`Stranded`, releases capacity and the wallet's reservation count, unwinds any pending action, and
emits `ReservationStranded(key, wallet, owner, amount)`. The owner keeps their minted TBTC as an
ordinary pooled claim. The backing shortfall is socialized "like a terminated wallet's main UTXO
today."

The event is explicitly the evidence hook for a future module, with no storage or interface
stubbed. So today there is no accounting of accumulated stranding liability, no restitution, and
no repair path. The loss simply spreads.

One property makes this tractable, and it is the same property that motivates reservations at
all: because `mintedAmount` always equals the anchor value (spec §6, claim-equals-anchor), a
stranded position's shortfall is **exactly quantified on-chain, per position, with no oracle**.
Pooled losses cannot be attributed this way. Segregated custody makes the liability precise, which
is the precondition for doing anything about it.

## 2. Who is actually harmed, and it is not mainly the depositor

This needs stating first because it inverts the intuitive framing and changes what the module
should do.

When a reservation strands, the depositor keeps `mintedAmount` in fungible TBTC. Pooled TBTC is
backed by the whole pool, and a stranding is capped at 50 BTC per wallet against a system holding
orders of magnitude more. So the depositor's claim remains worth approximately face value. Their
concrete loss is (a) their pro-rata share of the socialized hole, which is small, and (b) the
premium they paid for a guarantee that was not delivered.

The party that actually absorbs the 50 BTC is **every TBTC holder**, via a permanently degraded
backing ratio.

Two consequences:

- The module's principal job is **solvency repair owed to all holders**, not making a depositor
  whole. The depositor mostly already is.
- The depositor's genuine claim is narrower and cheaper: **restitution of fees paid** for an
  undelivered service.

This is probably why the module was never built. Nobody is visibly harmed at the moment of
stranding, so there is no claimant applying pressure, while the cost accrues diffusely to holders
who cannot see it.

## 3. Why self-insurance from reservation fees does not work

Worth doing the arithmetic before designing a fund, because it rules out the obvious shape.

Exposure, at the launch parameters (spec §10):

| | Value |
|---|---|
| Global reserved cap | 100 BTC |
| Per-wallet amount cap | 50 BTC |
| Per-wallet count cap | 10 positions |
| Single-position cap | 25 BTC |

Fee income, at full utilization of the global cap:

| | Value |
|---|---|
| Initiation, 40 bps on 100 BTC | 0.4 BTC, one-time |
| Custody, 20 bps/yr on 100 BTC | 0.2 BTC/yr |
| Total after year one | ~0.6 BTC |

A single wallet termination strands up to 50 BTC. So a fully utilized feature's entire first-year
fee income covers roughly **1.2%** of one wallet's stranding event, and the fee reserve's primary
job is already financing in-kind miner fees (`inKindFeeDebtSat`), so most of it is committed
elsewhere.

Self-insurance is off by about two orders of magnitude. Any design that promises to make
depositors whole out of reservation fees is arithmetic fiction. Either the fee schedule rises by
~100x, which destroys the product, or the module must work honestly at partial funding.

## 4. The mechanism: a voluntary proportional buyback

The design that survives §3 is not an insurance payout but an **exchange offer**, because an
exchange is solvent at any funding level, including zero.

Governance funds a buyback pot and sets a rate. A stranded position's owner may **voluntarily
surrender any portion of their stranded claim** in exchange for payout at that rate. The contract
burns exactly the surrendered portion and pays exactly what was bought.

```
surrender X of mintedAmount  ->  burn X  ->  pay X * rate
```

Properties that make this the right shape:

- **Never insolvent.** The burn is always proportional to the payout, so the module cannot promise
  more than it holds. At zero funding it simply makes no offer.
- **Repairs the backing ratio.** Every satoshi burned removes unbacked supply. Even a small
  partial buyback strictly improves solvency for all holders, which is the §2 harm.
- **No forced haircut.** A depositor offered 40 cents on the dollar can decline and keep their
  pooled claim, which is worth more. Nobody is worse off for the module existing. That matters,
  because a mandatory burn at a low rate would be confiscation.
- **Price discovery is honest.** If nobody accepts the offer, that is information: the pooled claim
  is worth more than governance is paying, and the loss is already adequately socialized.

The rate is a governance parameter, not a market. A market would be better in principle and much
worse in practice, since the volumes are tiny and manipulable.

## 5. Fee restitution, which is separate and actually affordable

Distinct from the buyback and should not be conditional on it, because the fee bought a service
that was not delivered regardless of what happens to the principal.

On stranding, the owner is owed back the initiation and extension fees paid on that position:
`40 + 20N` bps of the anchor. For a 25 BTC position held one year that is 60 bps, or 0.15 BTC.

This is affordable **by construction**: the reserve is made of exactly these fees, and refunding a
position's own contribution can never exceed what that position paid in. It needs a per-position
record of fees paid, which the spec does not appear to keep today and which is the one piece of
new storage this tier requires.

Redemption fees are not refundable, since a redemption that settled delivered its service.

## 6. Correlated stranding forces pro-rata, not first-come

A design constraint that falls directly out of the caps and is easy to miss: **strandings are
correlated by wallet.** One termination strands up to 10 positions and up to 50 BTC
simultaneously. Stranding events are not independent arrivals, they are batches.

So a shared pot with first-come-first-served claiming is both unfair and gameable via gas
auctions among co-victims of the same wallet. The module needs:

1. A **claim window** opened per stranding event (or per epoch), long enough that an offline
   depositor is not disenfranchised.
2. **Pro-rata settlement** across all claims registered in the window, when demand exceeds the
   pot.
3. No payout until the window closes, so ordering carries no advantage.

## 7. Funding waterfall

In priority order, most-aligned payer first:

1. **Reservation fee reserve**, for fee restitution only (§5). Bounded by construction.
2. **Slashed operator stake from the terminating wallet.** Termination already slashes; what does
   not exist is any routing of those proceeds to the positions that termination stranded. This is
   the accountability layer and the natural second loss. Note it does not scale with BTC at risk,
   since slashing is denominated per operator, so it will not cover a large stranding either.
3. **Governance treasury or a dedicated insurance allocation**, discretionary, funding the §4
   buyback at whatever rate is deemed worth paying.
4. **Residual: socialization**, which is today's behavior and remains the backstop for whatever
   is not bought back.

The module is best understood as **the rail, not the money**. Its value is that when governance
decides to spend, there is an auditable, non-gameable, correctly-accounted path to do it, instead
of an ad-hoc multisig transfer and a forum post.

## 8. Recovery and clawback

The anchor is deliberately **not** marked honestly spent on stranding, so the BTC may in principle
be recovered later (key recovery, or a governance-authorized sweep).

Rule: surrendering a portion of the claim under §4 transfers that portion of the owner's interest
in the anchor to the protocol. If the anchor is ever recovered, proceeds flow first to the buyback
pot up to what it paid out, then to remaining claim holders pro-rata. Without this, a recovery
would pay the depositor twice, once in buyback and once in recovered BTC.

Fee restitution under §5 carries no such transfer, since it is not a purchase of the claim.

## 9. Interaction with the emergency exit

If `exit/proposal.md` is ever built, the two are mutually exclusive
terminal paths for the same position and must be interlocked, or they become a double-claim in
exactly the way that document keeps rediscovering.

- No buyback payout while an emergency exit could still succeed. Compensation is claimable only
  from a state where the exit is definitively closed, including the `StrandedExitArmed` case that
  proposal's §6.2 introduces.
- Conversely, arming an exit should forfeit an unclaimed buyback offer for the same position.

Fee restitution is safe under either path and needs no interlock.

## 10. Suggested staging

Ordered by value per unit of work, and each tier is independently shippable.

| Tier | What | Cost | Why first |
|---|---|---|---|
| 0 | Storage + getters for cumulative stranded liability, per wallet and global | Very low | Governance currently has no number at all. Everything else needs this anyway |
| 1 | Fee restitution (§5) | Low | Affordable by construction, unambiguously fair, needs only a fees-paid record |
| 2 | Buyback rail (§4, §6, §8) unfunded, rate at zero | Medium | Ship the mechanism before it is needed under pressure; a rate of zero is a valid, safe state |
| 3 | Slashing proceeds routed to stranded positions (§7.2) | Medium-high | Real accountability, but touches the staking/slashing path |

Tiers 0 and 1 are worth doing regardless of any decision about the emergency exit, and Tier 0 is
arguably a prerequisite for making that decision on evidence rather than intuition.

## 11. Open questions

- **Payout asset.** TBTC from the pot is cleanest, since the depositor wanted BTC exposure. T
  would push token price risk onto the victim. ETH or stables introduce a treasury dependency.
- **Rate setting.** Who sets it, how often, and against what benchmark? A standing rate invites
  gaming around expected strandings; a per-event rate is slow and discretionary.
- **Does Tier 1 need a claim, or should it be automatic?** Automatic is friendlier but pushes gas
  onto whoever calls the notifier.
- **Is a permanent reserved-fraction cap the real answer instead?** If the global cap stays small
  relative to total backing, socialization may simply be the correct and sufficient policy, and
  Tiers 2-3 never need to exist. That is a legitimate outcome and Tier 0 is what would show it.
- **Fees-paid record.** Confirm whether any per-position fee accounting exists today; §5 assumes
  it does not and that this is new storage.
