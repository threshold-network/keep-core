# Reservation Emergency Exit — Alternative Designs

Status: investigation result. Companion to `proposal.md` (Mechanism 1, the retained
escrow-gated committee reference - deferred per the Decision in `README.md`),
`addendum.md` (mechanisms 1-4 and
their rejections), and `../stranding-compensation-proposal.md` (the loss-absorption
baseline).

This document reports a parallel investigation into whether any design beats the committee
reference. The headline is that the search space turns out to be **closed by a proof rather than
by a lack of imagination**, and that closure hands us exactly one previously-unconsidered
mechanism.

## 1. The result

Two independent analyses, run on different models with no shared context, converged on the same
verdict and the same counterexample.

**The design space is one dial: how long the depositor's claim is illiquid.**

| Illiquid window | Consequence |
|---|---|
| Zero (claim liquid throughout) | A live co-signer at exercise time is unavoidable. This is the adopted committee. |
| A bounded dispute window | The adopted mechanism. Arming escrows the claim temporarily; a committee is still needed to authorize the Bitcoin spend during the window Bitcoin cannot see. |
| The position's whole `Active` life | **No committee is needed for R1-R5.** A permanent lien plus a bare timelocked owner-key branch satisfies all five written requirements — but it caps custody at the timelock (§3.1) and bypasses the redemption watchtower (§3.2), which is likely disqualifying. |
| Zero, and R2 relaxed to deterrence | Owner-bonded slashable collateral. Insurance, not enforcement. |

Everything else examined — pre-signed transactions, covenants, BitVM, timelock encryption, wallet
revival, custody topology changes, bonding — is either a point on this dial or provably dead.

Two findings do not fit on the dial and matter more than anything on it.

**The written requirements are incomplete.** Every mechanism that removes the live party also
removes the redemption watchtower's ability to screen, delay, or veto an exit, and more generally
cannot see any Ethereum state that changed after the anchor was created — including the position's
own renewed lifetime. Those are two unwritten requirements (R6 and R7, §7), and they are what
actually justify a live party, rather than R2 as the design record currently claims.

**"Live party" does not mean "committee."** Case 3 is a cost spectrum, not a fixed shape. But the
cheaper point on it turns out to be a staffing choice rather than a new mechanism: Council-appointed
membership of a distinct threshold group drops the new-trust cost while leaving the machinery cost
intact (§7). The machinery itself is forced, and §7 records why the one shape that looked cheaper
cannot be built.

## 2. Why the space is closed

### 2.1 The case split

Any Bitcoin-side emergency branch ultimately checks a signature against some key K fixed at anchor
creation, because a UTXO's script is immutable once created and `CHECKSIG` only checks signatures
against keys. Partition on who can sign under K at exercise time:

1. **K is the depositor's key.** Producing a signature is unconditional local computation over a
   private key, not a stateful query. So the depositor can sell the claim, wait out any timelock,
   sign, and keep the BTC. **R2 fails.**
2. **K is the wallet's key.** Requires the dead component. **R3 fails** by definition of the
   scenario.
3. **K is a third party's key.** Someone who must exist, hold key material, and be alive and
   honest at exercise time. **This is a new trusted party.**

There is no fourth case that helps. Signature aggregation (multisig, threshold, MuSig2, FROST) does
not add one, because every required share must still come from a live party. Deferring *when* the
controller of K is fixed (key derivation, a later-published MuSig2 tweak) changes the timing but not
the identity partition: whoever ends up holding the scalar still has to be alive and willing to sign
a specific Bitcoin sighash.

Adversarial review found one genuine omission and one genuine gap.

**The omission: a no-key bucket exists.** `<CSV T> DROP OP_TRUE`, or a bare hashlock with no pubkey
check, gates on something other than a signature — so "`CHECKSIG` only checks signatures against
keys" is not a blanket truth about Bitcoin Script. It is dominated rather than exploitable: with no
activated covenant opcode to constrain the spending transaction's outputs, whoever supplies the
witness chooses the destination, making it a public race the depositor has no privilege to win. Worth
naming so it is not rediscovered.

**A rejected refinement, recorded because it is an attractive-looking error.** Adversarial review
proposed that "wallet death" is not monolithic: that `HonestThreshold` 51 is a cryptographic floor
while `GroupQuorum` 90 is a per-signature liveness bar, leaving a band between 51 and 90 where a
group is cryptographically capable of signing but cannot clear its participation bar — and that an
emergency ceremony with a relaxed bar could therefore rescue outages with no new party at all.

**That band does not exist.** The two parameters gate different operations, and the code says so
directly: `GroupQuorum` is "the minimum number of active participants... needed to generate a
group" and `HonestThreshold` is "the minimum number... needed to generate a signature"
(`pkg/tbtc/tbtc.go:32-38`). Every non-test use of `GroupQuorum` is on a DKG path — `dkg.go:824`,
`dkg_loop.go:223`, `dkg_loop.go:369`, `dkg_submit.go:115` — and none is on a signing path.
`wallet.go:502` constrains the membership list formed at DKG time to `[GroupQuorum, GroupSize]`,
which is a property of group formation, not a per-signature requirement.

So 90 gates wallet *creation* and 51 gates *signing*. A wallet with 60 honest operators signs today
with no relaxed ceremony and nothing to rescue; below 51 the key is simply gone. There is no
intermediate state to exploit, and no pass should be spent on one.

What survives is only the narrow point: R3 forbids *any* wallet dependency at any degree of
degradation, and that all-or-nothing framing is a scoping choice rather than a fact about either
chain. It just has no exploitable band behind it.

**Corollary, and adversarial review broke the version first written here.** The original claim was
"the committee is not a design choice, it is what case 3 looks like." The first half is right and
near-tautological: case 3 needs a live honest external party. The second half does not follow — it
silently equates case 3 with *Mechanism 1's specific cost structure* (a DKG'd threshold group with
resharing, monitoring, governance, and colluding-quorum risk). Case 3 is a **cost spectrum**, and
§7 records where the cheaper points on it actually are, and why one apparent one is unbuildable.

### 2.2 The lemma

> You cannot simultaneously have (A) a claim balance that is genuinely, permissionlessly
> transferable at all times, and (B) mechanically guaranteed burn of that specific balance at an
> arbitrary future exit time.

Because Bitcoin cannot read Ethereum state, there is no way to reach the claim at exit time unless
it was held somewhere reachable all along.

**One correction to how this was first justified.** The original wording said "ERC-20 balances carry
no lineage once transferred." That is too strong: a modified transfer function can preserve lineage
through direct peer-to-peer hops, the way fee-on-transfer and rebasing tokens already do. The
correct sufficient condition is narrower and survives scrutiny: **lineage cannot survive commingling
in a shared or pooled balance.** The moment a tagged token enters an AMM reserve, a lending pool, or
an exchange hot wallet, there is no fact of the matter about which unit carries which lien. Since
genuine liquidity — the thing property (A) is buying — requires exactly that poolability,
lineage-preserving transferability is only available in a peer-to-peer-only form that is a much
weaker product, and which collapses back into segregation anyway.

**And one scoping point that matters for future work.** Relaxing R2 from "destroy the specific
claim" to "preserve aggregate solvency" reopens the insurance and bonding family, because burning
*any* equal claim restores the backing ratio. That family is rejected here by policy, not by
impossibility: it lets the specific thief keep both the sale proceeds and the BTC while everyone
else absorbs the dilution. So **R2's literal strength is a contingent anti-moral-hazard choice of
this feature, not a property of Bitcoin or Ethereum.** Anyone revisiting this design space should
know that is the dial with the most give in it.

### 2.3 The asymmetry that is often misread

The wall is strictly one-directional, and it is worth being precise because it looks like it
should help more than it does.

**Ethereum already verifies Bitcoin richly.** `Fraud.sol:152-159` verifies Bitcoin ECDSA
signatures on Ethereum via `CheckBitcoinSigs.checkSig(walletPublicKey, sighash, v, r, s)`, and the
contract reconstructs BIP-143 sighashes from preimages itself rather than trusting a caller
(`Fraud.sol:149`, `:235`, and sighash-type extraction at `:571-579`). Permissionless SPV proofs are
already how every reservation action settles.

So Ethereum can detect an emergency Bitcoin spend and attribute it to a key, with no new
primitive. What it cannot do is *prevent* one. That is why the fix is never "gate the Bitcoin
spend" and always "make sure the claim is still there when Ethereum finds out."

**This reframes R2 usefully:** R2 does not require Bitcoin to check Ethereum. It requires the claim
to be present when Ethereum learns of the Bitcoin spend. The adopted mechanism achieves that for a
bounded window by arming. A lien achieves it permanently, without a committee.

## 3. Mechanism 5: escrow-at-mint lien

The one design the closure argument hands us, and the only committee-free construction found that
satisfies R2 literally.

**Ethereum side.** At acceptance, do not mint the full `mintedAmount` as liquid TBTC. Credit a
fraction `(1 − f)` as ordinary transferable TBTC, and hold `f · mintedAmount` as a **locked escrow
entry inside the vault**, tied to `reservation.owner` — an internal ledger row, not a token in the
owner's wallet.

This detail is what makes the mechanism cheap. It is not a transfer hook on TBTC, and it does not
change how TBTC behaves for anyone else. There is nothing to sell, because the locked portion was
never a spendable balance. It reuses the shape of machinery the vault already has: the vault
already escrows the claim at request time during normal redemption (`ReservationVault.sol:301-317`).

**Bitcoin side.** The anchor script degenerates to a plain two-path script with no third party:

```
IF   <wallet pubkey> CHECKSIG                              -- cooperative, as today
ELSE <CSV T> DROP <depositor refund pubkey> CHECKSIG       -- emergency, owner alone
ENDIF
```

**Flow.** Wallet dies. After `T`, the owner spends the emergency branch using only their own key
(case 1, no wallet dependency, no committee). Anyone submits a permissionless SPV proof of that
spend — the same primitive the feature already uses everywhere. The vault burns the locked escrow
entry it already holds, with no cooperation from the owner, because the owner never had the power
to move it.

**Requirements.**

| | Verdict |
|---|---|
| R1 | Satisfied. Fixed at mint, zero Bitcoin-side refresh ever. |
| R2 | Satisfied literally at `f = 1`. Proportional below that (see §4). |
| R3 | Satisfied. The wallet is needed only to create the anchor at setup. |
| R4 | **Not satisfied as originally argued — see §3.1.** There is no maximum term to set `T` beyond. |
| R5 | Satisfied against theft, but see §3.2: it defeats a live compliance control. |

**The R5 comparison, stated carefully.** An earlier draft of this section claimed the lien's
residual R5 risk is simply a better *kind* of risk than the committee's. Adversarial review found
that overclaimed, and the corrected version is narrower.

What is genuinely better, and structurally so: an Ethereum contract bug cannot forge a Bitcoin
ECDSA signature, so the emergency exit's own success is cryptographically insulated from vault
bugs. A committee-mediated exit cannot claim that, because its signing is orchestrated through
Ethereum state that a quorum-honesty failure can corrupt directly.

What does **not** follow is that the lien is lower-risk overall. Two corrections:

- **Base rates cut the other way.** "Statically auditable" describes tractability, not realized
  failure frequency. Access-control bugs are among the most common causes of real on-chain loss,
  and this mechanism adds correctness obligations across every owner-gated release path — the vault
  already has five (`ReservationVault.sol:301-304`, `:368-371`, `:429-432`, `:491-494`, `:592-595`).
  Against a 100-member group where breaching the 90-of-100 quorum needs many simultaneous
  defections, the committee's failure probability may well be *lower* in practice.
- **The worst case was understated.** The fail-to-burn direction (an unburned escrow row on a dead
  reservation) is a benign bookkeeping lag. But the mirror failure is not: an access-control bug
  that erroneously *releases* the locked fraction as liquid TBTC with no Bitcoin-side exercise
  collapses the position to full unprotected exposure while the depositor still believes they are
  covered. Under R5's own wording that is "an exit that should not have happened," so the lien has a
  genuine fail-open mode of its own.

**It also dissolves two open problems the committee has.** There is no committee key to rotate, so
the unresolved "anchor created years ago embeds that era's committee key" staleness problem
disappears. And there is no colluding-committee exploit, so the disarm-refund attack surface and
the whole byzantine-versus-crash-fault distinction disappear with it.

**The cost is liquidity, and it is severe.** At `f = 1` the depositor receives zero liquid TBTC for
the life of the reservation. They hold segregated custody plus a redemption right, not liquidity
against their BTC. That is arguably a different product from the one reservations are selling.
Notably it costs **no extra capital** — unlike a bond or a same-asset BTC collateral covenant, both
of which require the depositor to lock value *in addition to* the deposit.

### 3.1 There is no maximum term, so no finite `T` is safe

The R4 argument above ("choose `T` beyond the maximum term") does not survive contact with the
code. **No maximum custody duration and no renewal cap exist.** Renewal adds exactly one term to
the expiry (`Reservation.sol:1336`, `newExpiresAt = expiresAt + term`), gated only to a window
immediately before expiry (`:1330-1333`), with no limit on how many times it may happen.

So any finite `T` is eventually crossed during perfectly healthy, renewed custody, after which the
owner can spend the emergency branch at will against a live wallet.

The consequence is narrower than theft, and worth stating precisely. At `f = 1` the locked escrow
is burned by the SPV proof, so nothing is left unbacked and R2 and R5 survive. What breaks is that
the emergency path becomes a *cheaper* alternative to normal redemption — it skips the 20 bps
redemption fee and needs no wallet cooperation — and the custody term becomes unenforceable,
because the owner can always leave after `T`. The 20 bps/yr custody fee model erodes with it,
since nobody renews past `T`.

Refreshing `T` on renewal would fix it and is not available: a new timelock means a new anchor
script, which is a Bitcoin action, which is precisely the R1 dependency this mechanism exists to
avoid.

The honest repair is to accept `T` as a **de facto maximum custody horizon** and say so: custody is
renewable up to `T`, after which the position must be redeemed or it becomes owner-exitable. That
is a real product constraint the current design does not have, and it should be priced as one
rather than hidden.

**And that constraint is not only this mechanism's cost.** §7 records why the adopted committee
design needs a bounded term as well: resharing keeps the committee key alive only while the group
keeps refreshing, so an unbounded custody term is an unbounded key-liveness obligation on a party
that must outlive the position. Capping `T` bounds the lien's exit *and* the committee's duty. It is
a shared requirement that surfaced here first because the lien fails loudly without it.

**Stated as a lemma, because it generalizes past this mechanism.** Adversarial review confirmed the
finding independently and sharpened it: the only ceiling on renewal is the incidental `uint32`
timestamp rollover in February 2106, which the code itself flags with an `XXX` comment — an
accidental limit roughly eight decades out, not a designed maximum term.

> **R1 and R4 cannot both hold for a case-1 mechanism once the custody term is unboundedly
> renewable.** A Bitcoin-side timelock fixed at anchor creation and an Ethereum-side term extendable
> without limit are mechanically independent, so any `T` short enough to be useful within a human
> lifetime will eventually be crossed by a healthy, still-renewing position.

The only in-class repair is already in the graveyard: refreshing `T` on renewal means re-anchoring
on a schedule, which is the "re-anchor-invalidated dead-man switch" that §5 rejects for forcing
periodic Bitcoin actions (R1) and letting a fee spike open the exit against a healthy wallet (R5).

This is the deeper form of the §3.2 objection rather than a separate problem. A fixed Bitcoin script
cannot track *any* post-creation mutable Ethereum state — not a compliance decision, and not the
position's own renewed lifetime. A live party can read both before acting.

### 3.2 The harder objection: it bypasses the redemption watchtower

This is structural, and it is the strongest argument against Mechanism 5.

The sanctioned exit is watchtower-gated three ways. A redemption request requires
`isSafeReservedRedemption(reservation.owner, redeemer)` and reverts otherwise
(`Reservation.sol:782-788`) — note it screens the **owner**, not merely the payout script. Requests
are subject to an amount-keyed delay schedule. And the watchtower can veto outright
(`Reservation.sol:1228-1263`): the action becomes `Vetoed`, the position returns to `Active`, and
at `:1263` the surrendered balance is transferred **to the watchtower** — seized, not refunded.

A bare `CSV T` owner-key branch requires no Ethereum call, so none of those three controls can
engage. The owner the watchtower exists to stop — one whose `isSafeReservedRedemption` returns
false, or whose redemption was already vetoed and whose funds were confiscated — simply waits out
`T` and takes the BTC on Bitcoin anyway.

This is not a parameter choice. **Any branch that works when the wallet is dead also works when the
owner is sanctioned**, because both conditions are equally invisible to Bitcoin. It is the same
one-directional blindness from §2.3, applied to policy rather than to solvency.

**This reframes the whole comparison, and it is the finding that most changes the recommendation.**
A live party at exercise time does not only close R2; it is the only construction that can apply a
discretionary check at the moment of exit. A committee can decline to co-sign for a vetoed owner. A
timer cannot consult anything. So the committee's trust-surface cost buys **policy enforcement**,
which is a capability no case-1 design can have at any parameter setting — a stronger justification
for Mechanism 1 than any currently recorded in the design docs.

It also suggests the requirement set is incomplete: an unwritten sixth requirement (formalized as
**R6** in §7), that the exit
remain subject to the same compliance controls as normal redemption, has been assumed throughout
and never stated. R1-R5 do not capture it, and Mechanism 5 satisfies R1-R5 while violating it.

### 3.3 Every redemption path would have to be rewritten

A third objection, smaller than §3.2 but larger than it first looks, because it is not an
add-on but a rewrite of how the vault handles claims.

Every cooperative exit today sources the surrendered claim from the **owner's own wallet balance**.
Partial redemption does `IERC20(tbtcToken).safeTransferFrom(msg.sender, address(this), grossTbtc +
fee)` (`ReservationVault.sol:379-383`), and the whole-redemption and retry-credit paths do the
same shape (`:313-317`, `:436`, `:605-609`).

At `f = 1` the owner holds no liquid TBTC at all, so **every one of those transfers reverts** and
all five owner-gated redemption paths stop working. The lien does not simply sit alongside the
existing flows; each must be rewritten to draw the claim down from the escrow row instead of
pulling tokens from the owner.

That is five call sites (`:301-304`, `:368-371`, `:429-432`, `:491-494`, `:592-595`), plus the
requirement that the escrow be drawn down in exact lockstep with the anchor across partial
redemption, which spends the anchor 1-in-2-out and creates a smaller one. Any drift between escrow
and anchor value produces either an over- or under-collateralized burn.

This matters beyond effort: an ordering bug in that rewrite is precisely the erroneous-early-release
fail-open mode identified in the R5 discussion above. The mechanism's headline appeal was that it
needs no new infrastructure, and that is true of the Bitcoin side but false of the Ethereum side.

## 4. Correction: there is no partial-lien threshold

One agent reported a break-even at `f ≥ 0.5`. That is wrong, and the error matters because it
would otherwise look like a cheap escape from the liquidity cost.

Correct accounting, per unit deposited:

- **Honest redemption.** Surrender the whole claim, receive 1 BTC. Net versus the deposit: zero.
- **Sell-then-exit under lien `f`.** Sell the liquid `(1 − f)` for `(1 − f)`. Exercise the
  emergency branch for 1 BTC. The locked `f` is burned. End holdings: `(1 − f) + 1 = 2 − f`.

Gain over honest behavior is `(2 − f) − 1 = 1 − f`, which is zero only at `f = 1`.

The reported break-even came from counting the burned `f` as a cost of attacking. It is not
incremental: honest redemption surrenders the claim too. The `f` leg is a wash between the two
strategies, and the whole differential is the `(1 − f)` leg, which the attacker collects twice
(once as a sale, once inside the full-anchor extraction) and the honest redeemer collects once.

Adversarial review confirmed that derivation and then found four refinements, three of which
matter.

**The redemption fee leaves a residual even at `f = 1`.** The emergency branch has no Ethereum-side
request step, so it avoids the 20 bps redemption fee `r`. Honest net is `1 − r`, attack net is
`2 − f`, so gain is `1 − f + r` and the break-even is `f* = 1 + r`, outside the feasible range.
At `f = 1` the gain is not zero but `r`. Small, but it means a full lien makes the emergency path
permanently *cheaper* than cooperative redemption rather than merely equal — which is exactly the
R4 leak §3.1 describes, and it does not close at any `f`.

**Time value gives a real break-even below 1, which is nonetheless not usable.** The liquid tranche
sells at time zero while the extraction waits out `T`. Discounting at rate `ρ`, gain is
`δ^T − f` with `δ = e^{−ρ}`, so `f* = e^{−ρT}` — for example roughly 0.85 at `ρ = 8%` and
`T = 2` years. But the mechanism must be safe against the *most patient* depositor, and as `ρ → 0`
the threshold returns to 1. A depositor who can finance the deferred claim at near-risk-free rates
is effectively that patient. So this is an argument *for* `f = 1`, not a cheaper setting, and the
blanket claim "zero only at `f = 1`" holds strictly in the zero-discount limit.

**Market impact cannot flip the sign.** If the liquid tranche realizes only `p` of face value, gain
is `p(1 − f) + r`, which stays positive for all `p, f ∈ [0,1]`. Even at `p = 0` the residual is `r`.
A peg premium only widens the gain.

**The sharpest correction: a partial lien can make aggregate loss worse than doing nothing.**
Measured against the true baseline — socialized stranding, where the depositor keeps TBTC worth
about face value — the depositor's gain is still `1 − f`. But that framing exposes a second victim.
The buyer of the sold `(1 − f)` holds fungible TBTC with no provenance, so once the drain surfaces
the same backstop must make them whole too. Total system loss from one dead wallet is therefore
`2 − f`, against `1` under no Mechanism 5 at all. **For any `f < 1`, layering a partial lien on top
of an active compensation backstop increases total losses**, because the same 1 BTC of collateral is
claimed against the backstop twice.

Two further corrections to how the residual was characterized. The depositor's gain is genuinely
linear in `f` with no step behaviour, since selling the whole liquid tranche dominates for any
`f < 1`. But the claim that the unlocked `(1 − f)` is "precisely as exposed as today's fully-liquid
design" is wrong: today's exposure is exogenous and population-average, whereas this tranche is
sold by the one party who can unilaterally drain the collateral and who knows the wallet is failing.
That is adverse selection, and conditional on such a sale the buyer's loss approaches certainty
rather than a base rate. The mechanism creates a risk channel that does not exist today.

So a partial lien does not buy proportional loss reduction. It buys proportional *enforcement* on
the locked fraction while creating a new informed-dumping channel on the rest, and in combination
with compensation it can raise total losses. **`f = 1` or nothing.**

## 5. What is dead, and why

Each of these was investigated in its own right; the reasons are recorded so they are not
re-proposed.

| Family | Verdict |
|---|---|
| Pre-signed refund transactions | Case 1. A bearer signature exercisable from day zero; collapses into rejected Mechanism 3. |
| Re-anchor-invalidated dead-man switch | Rejected Mechanism 4 with a different clock. Forces periodic Bitcoin actions (R1) and lets a fee spike or congestion event open the exit against a healthy wallet (R5). |
| Chained/laddered pre-signing | Trades R1 for a worse R2/R4, because Ethereum-only renewal desynchronizes from Bitcoin locktimes baked in at acceptance. |
| Hash-locks, adaptor signatures, scriptless scripts | A commitment can only encode a value the committer already knows, and here the committer is the depositor. Any secret fixed at setup is, in exercisability, identical to a static timelock. |
| Bundled position transfer (position + locked claim move atomically) | Dead. The depositor refund pubkey is immutable in the anchor script, so a buyer cannot inherit an exercisable key without a Bitcoin transaction, and the seller would retain one. tbtc-v2 PR #911 ("feat: non fungible") explored a non-fungible representation and was abandoned. |
| Rescuing that via re-anchor | Dead. `requestReservationReanchor` moves anchors between *wallets* and never touches `owner` (`Reservation.sol:1003-1012`), it is governance-only while the source wallet is `Live` (`:942-946`, "Only governance can rotate a Live wallet's anchor"), it is barred once dissolution-eligible (`:932-935`), and it needs the source wallet's threshold signature — a liveness dependency on the party whose failure the mechanism exists to survive. |
| BitVM / BitVM2 | Breaks wall 1 in principle but relocates trust rather than removing it: a bonded operator/prover, an oracle-signed Ethereum-state assertion, and an economically-motivated watcher market. Fails R1, because the disprove circuit is frozen at funding and cannot verify mutable Ethereum state years later without re-anchoring. Its ceremony does not amortize across per-depositor anchors the way a wallet-level DKG does. |
| Covenant opcodes (CTV, CSFS, APO, CAT, OP_VAULT) | None activated on mainnet. Only CSFS is even relevant, and verifying a signature over arbitrary data requires a signer of that data — an oracle by construction, so it relocates trust rather than eliminating it. |
| Timelock encryption, VDFs (drand tlock) | Pure clocks with no visibility into Ethereum state. Wall 2 material, wrong category of primitive. |
| Wallet revival via resharing | Mainnet parameters are `GroupSize` 100, `GroupQuorum` 90, `HonestThreshold` 51 (`pkg/tbtc/tbtc.go:70-74`). Signing needs 51; below that the key is gone, and threshold resharing itself needs threshold participation. Governance-forced reconstitution would be a backdoor to all wallet funds. |
| Economic bonding / underwriting | Deterrence plus insurance, not enforcement — explicitly what R2 defines as failure. Existing slashing is a flat governable parameter per failure mode, seized via `ecdsaWalletRegistry.seize` (`Wallets.sol:265-266`, `:507-508`, `:548-549`, `:589-590`), and does not scale with BTC at risk. |
| Proportional exit (exit only in proportion to the claim you still hold) | Dead, and instructively so. The proportion is an Ethereum-side fact, so Bitcoin cannot check it at spend time: the owner signs whatever outputs they like and simply takes the whole anchor. It collapses into case 1. A pre-committed split cannot help either, because the proportion is unknown at acceptance. Separately, the remainder output would have to pay back to a wallet that is dead by hypothesis and can never spend it, so even an honest split strands the remainder permanently. |

## 6. Open verification items

Stated as unresolved rather than assumed, because each was either unverifiable from the repo or
was reported wrongly by an agent and corrected here.

- **Live slashing and timeout parameter values.** The mainnet-only deploy scripts
  `16_disable_fraud_challenges.ts`, `17_disable_redemptions.ts` and `18_disable_moving_funds.ts`
  set every slashing amount to zero and several timeouts to `uint32` max. These are **launch-time
  initialization only** and say nothing about current values: later scripts
  (`40_deploy_redemption_watchtower.ts`, `81_upgrade_bridge_v2_vault_fix.ts`,
  `85_deploy_tip109_governance_upgrade.ts`, `95_deploy_reservation_vault.ts`) show a live,
  governance-updated deployment, and reservations themselves run through live redemption
  machinery. Any economic design must read the current on-chain values of
  `fraudSlashingAmount`, `redemptionTimeoutSlashingAmount`, `movingFundsTimeoutSlashingAmount` and
  `movedFundsSweepTimeoutSlashingAmount` rather than assuming a default.
- **`covenantsigner` as partial prior art.** It exists on unmerged branches
  (`origin/codex/psbt-covenant-approval-envelope`, path `pkg/covenantsigner/`, verified via
  `git show`), described as "covenant signer substrate slices: durable submit/poll semantics,
  strict request validation, and a compatible HTTP surface for covenant recovery/presign signer
  jobs." It is built for covenant fund migration, not for UTXO reservations, and its "reservation
  artifact" is the unrelated migration-destination concept. **Unresolved: whether it is a single
  signer or a threshold group.** That determines whether the committee's headline cost is real or
  already partly paid, so it is worth settling before pricing Mechanism 1. One agent reported the
  package does not exist; that report is false.
- **Expected annual loss.** Still unquantified. This is the decisive input for whether *any*
  mechanism is worth building, and two attempts to derive it failed to produce a number. Needed:
  a defensible annual per-wallet termination probability, multiplied through the correlated
  per-wallet exposure (up to 50 BTC and 10 positions at once), compared against the build-and-carry
  cost of a second threshold signing group.

## 7. Recommendation

The cryptographic question is settled. What remains is a product and policy decision, and §3.2
changes which way it leans.

Three coherent packages, each paying a different price:

1. **Committee plus liquid claim** (adopted, Mechanism 1). Pay in trust surface: a new threshold
   group with DKG, resharing, monitoring and governance, R2/R4/R5 holding only against a
   non-colluding committee, and an unresolved key-rotation-versus-anchor-staleness problem. Buys
   claim liquidity *and* policy enforcement at exit time.
2. **Lien at `f = 1` plus no committee** (Mechanism 5). Pay in liquidity, in a hard maximum custody
   horizon `T` (§3.1), and in the loss of watchtower control over the exit (§3.2). In exchange
   every trust problem in option 1 disappears.
3. **Liquid claim, no committee, compensation and/or bonding.** Pay by giving up R2 literalism,
   accepting deterrence and insurance in place of enforcement. This is the
   `../stranding-compensation-proposal.md` path, and note it preserves the watchtower,
   because there is no alternative exit path to bypass it.
4. **Mechanism 6: ad hoc single bonded co-signer - raised by adversarial review, and it does not
   survive its own construction.** The proposal was: after SPV-provable wallet death, *reactively*
   select a single already-employed, already-slashable wallet operator as co-signer, 2-of-2 with
   the depositor, reusing existing stake and seize infrastructure (`Wallets.sol:265-266`,
   `:507-508`, `:548-549`, `:589-590`) instead of standing up a DKG'd group. Its appeal was
   shedding almost all of Mechanism 1's carrying cost while keeping R1-R6.

   **Reactive selection is not expressible.** Any key the anchor script checks is fixed at anchor
   creation - §2.1's own premise - so a co-signer chosen years later cannot be the party the
   script names. Both substitutes are worse than what they replace: pre-commit `N` candidate
   operator keys as 1-of-`N`, which is exactly the 1-of-N-safety failure the proposal's `§5.1`
   rejects for shared keys; or have a fixed party hold a dormant Bitcoin key for the anchor's
   whole lifetime, in which case that holder *is* the trusted party and the reactivity was
   illusory. Withdrawn.

   What survives is the corollary it was raised to prove: case 3 is a cost spectrum, and Mechanism
   1's specific cost structure is not entailed by case 3. The cheaper point is staffing, not shape.

**On Mechanism 5's standing.** An earlier draft of this document recommended recording it as a peer
of Mechanism 1. That was too generous, and three independent objections landed: it caps custody at a
timelock that the protocol permits positions to outlive (§3.1), it discards the watchtower's control
over the exit (§3.2), and it requires rewriting all five redemption paths to source claims from
escrow (§3.3). Its real contribution is **diagnostic**: it proves the committee was doing a second
job nobody wrote down, and that job — not R2 — is what actually justifies the trust surface.

**The requirement set should be amended.** Two requirements were assumed throughout and never
written, and both are what actually kill Mechanism 5:

- **R6, exit-time policy consultability.** The exit path must be able to consult *current*
  compliance and dispute state at the moment of exercise, not a decision frozen at anchor creation.
  R2 governs consequences after an exit, R4 is a cost deterrent, and R5 is about the mechanism not
  misfiring on its own terms; none of them captures this.
- **R7, mutable-state tracking.** More general and it subsumes R6: the exit path must track
  post-creation mutable Ethereum state, including the position's own renewed lifetime. Case-1
  designs are structurally incapable of both, for the same wall-1 reason.

**Recommendation: option 1, staffed per the proposal's `§5.1`, with option 3 as the baseline worth
building regardless.** Option 4 is withdrawn. The elimination is now sharper than "the space is
closed": among case-3 shapes, **threshold signing with share refresh is the only one that survives
multi-year anchor immutability**, because a plain multisig cannot change membership without changing
the script and a single fixed signer cannot be replaced at all. The machinery is forced; only
staffing is open, and Council-appointed membership of a distinct group is the cheapest answer there
- it adds no marginal trust over powers governance already holds, while keeping the Council out of
the anchor script so it remains available as the escalation tier.

**But the forced machinery carries an obligation that must be priced, not assumed.** Resharing keeps
the group key constant only while the group keeps existing and keeps refreshing. With no maximum
custody term (§3.1), that is an *unbounded key-liveness obligation*, and if the group ever falls
below `t` the emergency branch is permanently dead - the exact failure the mechanism exists to
prevent, now caused by the mechanism. Two consequences, both now recorded in the proposal's `§5`: a
terminal disposition backed by the compensation module is mandatory rather than a loose end, and
**capping the custody term is something the adopted design needs too**, not merely a cost of the
lien. §3.1 prices the unbounded term as an argument against Mechanism 5; it is equally an argument
for capping it whichever mechanism ships.

**On sequencing**, nothing here displaces the compensation baseline. Compensation Tiers 0 and 1
(liability accounting and fee restitution) remain worth building regardless, and Tier 0 is what
would finally produce the expected-loss number that decides between these packages.
