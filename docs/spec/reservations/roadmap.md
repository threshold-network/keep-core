# UTXO Reservations — Milestone-Based Roadmap (create-only first release)

Status: DRAFT — for review. Last substantively revised 2026-09-24 (the Option
B vault decision, §1.2/§2.2/§6/§7); earlier revisions: 2026-08-21 (reversed
the earlier "cut #1092" decision, §0.1), 2026-08-23 (§5.1 line-count
corrections), 2026-09-07 (§7 open-question resolutions).

Goals, non-goals, functional requirements and user stories live in
`requirements.md`; the M1 architecture, components and diagrams live in
`architecture.md`. This document is the scope-decision history — read it for
*why* the M1 surface is what it is, not as the requirement list itself.

Objective: ship the smallest *reachable* surface, and make every later fix
an upgrade rather than a migration. Agent-delegated rework is cheap, so a
feature's cost is its v1 surface mass — but a PR's cost is not the same as a
feature's cost, and §0.1 is where that distinction bites.

**Milestone 1 is a rails release, not a product release.** Users can create
reservations. They cannot redeem in-kind or renew.

Companions: `feature-spec.md` (§11, the deploy-inert pattern),
`epic-merge-plan.md` (§0.1-§3, stack topology and extraction guidance — its §5
audit gate was dropped as superseded, see its §4), `timeline-estimate.md`,
`testing-plan.md`, `exit/alternatives.md` (§3.1 and §6, custody-term cost).

## 0. Source-verified facts that determine the scope

Citations are `Reservation.sol` unless noted, with the branch named — this
matters, because **the expiry model differs across the stack**.

### 0.1 #1092 is structural to the upper stack, not an additive layer

On `feat/utxo-reservation-settlement` (#1091) actions gate on
`expiresAt + gracePeriod`: redemption `<=` (:618, :735), re-anchor `<=`
(:742), `extendReservation` `<=` (:1083), dissolution `>` (:838).

On `feat/utxo-reservation-backing` (#1093) that model **is gone**:

- `dissolutionEligibleAt` is a struct field (:186), set to
  `expiresAt + reservationDissolutionDelay` whenever a term is granted, with
  explicit snapshot semantics: *"later governance changes never move the
  eligibility time of a term already granted"* (:180-184).
- The gates were rewritten: pre-eligibility actions require
  `block.timestamp < reservation.dissolutionEligibleAt` (:766); dissolution
  requires `>= dissolutionEligibleAt` (:880).
- `gracePeriod` **no longer exists**; `reservationDissolutionDelay` and
  `reservationRenewalWindowSeconds` are governance parameters (:308-314,
  :1212-1218).
- `updateReservationParameters` **requires**
  `reservationRenewalWindowSeconds > 0 && < reservationTermSeconds`
  (:1232-1236).

**Consequences.** #1093/#1094/#1095 are built on post-#1092 semantics, and
the parameter validator refuses to configure a system with no renewal
window. Excising #1092 would mean reverting gate rewrites inside three
downstream PRs and removing a struct field — intra-PR surgery on reviewed
code, not a rebase. Since m1 needs #1093 (backing, caps) and #1094 (guards,
stranding), **#1092's code must ship.** Create-only is therefore achieved by
controlling *reachability* (§0.2), not by omitting PRs.

### 0.2 Caller gates are the create-only control surface

Verified on both #1091 and #1093 branches:

| Path | Gate | Reachable in m1? |
|---|---|---|
| Acceptance (`requestReservationAcceptance`) | permissionless (:401) | **Yes** — this is the product |
| Redemption (`requestReservedRedemption`) | `msg.sender == self.reservationVault` (#1091 :584, #1093 :614) | **No** — the M1 vault (commit `4d549e64`) has no `redeemReservation`/`retryRedeemReservation` at all. Under the 2026-09-24 Option B decision m2 ships them via a **new** vault deployment plus a depositor migration ceremony, not a flag flip on this one (§1.2, §2.2) |
| Renewal (`extendReservation`) | `msg.sender == self.reservationVault` (#1091 :1064, #1093 :1133) | **No** — the M1 vault (commit `4d549e64`) has no `extendCustody` either, gated or otherwise. The reference stack (`feat/utxo-reservation-backing`) gated `extendCustody` (`:367`) by `renewalsPaused`, constructor-`true` (`:222`); Option B did not carry that pattern forward (§1.2, §2.2) |
| Re-anchor (`requestReservationReanchor`) | permissionless while source is `MovingFunds`; `privileged` required while `Live` (:718-757) | **Yes** — and this is desirable (§1.5) |
| Dissolution (`requestReservationDissolution`) | permissionless, post-eligibility only (:824, :880; router :302 has no modifier) | **Yes, on a timer** — deferred ~12 months by the term, then permanently open; must be wired (§0.6) |
| Action timeout | permissionless (:911) | **Yes** — required cleanup |
| Redemption veto | `msg.sender == self.redemptionWatchtower` (:1017) | Vacuous — no redemptions exist |
| Stranding | wallet `Terminated` + Active, no time gate (#1094 :1363-1378) | **Yes** — capacity valve |

**Under the B decision (§1) three of these rows describe the reference stack
rather than m1.** Redemption, renewal and dissolution Bridge code is not
deployed-and-gated in B, it is **absent**, so their gates are moot until m2
writes them. Two knock-on effects worth stating outright:

- **§0.6's slashing vector does not exist in m1 B.** With no
  `requestReservationDissolution`, nothing can be opened against an honest
  wallet and left to time out. This was B's headline benefit and it holds.
- **m1 B reads `dissolutionEligibleAt` in exactly one place: the
  Closing-wallet branch of `notifyReservationStranded`.** A `Closing`
  wallet's reservation can be stranded only once
  `block.timestamp >= reservation.dissolutionEligibleAt` (`Reservation.sol:1090-1113`
  in the M1 code). Every other would-be consumer is cut or deleted:
  redemption's strict `< expiresAt`, renewal, dissolution's
  `>= dissolutionEligibleAt`, and re-anchor's
  `< dissolutionEligibleAt` (removed to make re-anchor unbounded). So the
  custody term is **not enforced by any on-chain gate in m1** — it is a
  commitment recorded in storage for m2 to honour, which is exactly why the
  write must survive the rewrite (§0.7 item 3).

Because redemption and renewal are **vault-gated**, the m1 control surface is
the vault rather than Bridge code. §0.7 lays out two ways to use that: an
omitted vault entry point cannot be reached later without a vault swap, while
a flag-gated entry point can be unpaused by one governance transaction. **That
choice is now settled the other way.** The 2026-09-24 Option B decision
(`m1-b-implementation.md` §1/§3/§4.2) ships the m1 vault **without**
redemption or renewal entry points at all — no `redeemReservation`,
`retryRedeemReservation`, `extendCustody`, or pause flags for either path
(verified against the M1 vault, commit `4d549e64`: 7 external functions,
none of these) — and m2 restores both via a **new** vault deployment plus a
depositor opt-in migration ceremony (§1.2, §2.2). The earlier claim that "the
M1 vault does this for renewal but not redemption" no longer holds against
that commit: it has no renewal pause flag either.

**"Vault-side" was never automatically cheap, and the reference-stack vault
showed the cheap version for one path only — a design Option B did not
carry forward.** `ReservationVault` is plain `Ownable`, not
proxy-upgradeable (§2.2), and re-pointing `Bridge.reservationVault` requires
`reservationTotalAmount == 0 && pendingReservedDeposits == 0`
(`Reservation.sol:1263-1274`) — total quiescence. So a vault that *omits* an
entry point cannot gain it later while any position lives; that path is
closed, not deferred. This is the reasoning that originally recommended
flag-gating m1's vault entry points (§2.2's now-superseded recommendation);
Option B accepted the irreversibility instead (§0.7,
`m1-b-implementation.md` §3/§4.2), so the table below describes the
**reference-stack vault** (`feat/utxo-reservation-backing`, pre-Option-B),
not what m1 ships.

| Path | Reference-stack vault | Cost of adding this path later, given the guard above |
|---|---|---|
| Renewal | Entry point present (`ReservationVault.sol:367`), `renewalsPaused = true` set in the constructor (`:222`), `unpauseRenewals()` is `onlyOwner` (`:415-418`) | One owner transaction, on that stack |
| Redemption | `redeemReservation` present (`:293`) with **no pause flag** — `pauseRenewals`/`blockRenewal` (`:409`, `:424`) cover renewals only | Needs the flag added before launch, or a quiescence-gated vault swap, on that stack |

Neither function exists in the M1 vault at all (commit `4d549e64`; 7
external functions, none of these — verified above). The pattern this table
describes — copying renewal's constructor-paused flag onto redemption, "not
new machinery" since it was already written and audited in the reference
stack's file — was §2.2's original recommendation for m1's vault. Option B
rejected it; see §2.2's "Current answer" for what m1 ships instead, and for
the M2 requirement this choice imposes — the 2026-09-28 owner confirmation
and the Bridge-side vault-binding change m2's upgrade must make.

### 0.3 Minted tBTC is an ordinary fungible claim

The contract says so: on `Stranded`, *"the owner's minted balance remains an
ordinary pooled claim; the anchor is no longer tracked"* (:89-91); after
dissolution, *"the owner's minted balance simply remains an ordinary pooled
claim"* (:807-809).

So a create-only user is **not trapped** — they can sell their tBTC or
redeem via the ordinary pooled path. m1 withholds the *in-kind* guarantee,
which is the product's value-add. Hence: rails, not product.

The global invariant survives that exit: if an owner pooled-redeems X, then
supply `S−X`, pooled `P−X`, anchors unchanged at `A`; since `S = P + A`,
`S−X = (P−X) + A`. ✓ The exposure is **pooled liquidity, not solvency**,
bounded by the total reserved cap.

### 0.4 Bookkeeping-only closes are unsound (rejected design)

`closeReservation` (#1091 :1183-1192) only decrements the wallet count,
subtracts `anchorAmount` from `reservationTotalAmount`, and marks `Closed`.
That is loss *recognition* — valid for `Stranded` solely because a
Terminated wallet's BTC is already presumed gone.

On a Live wallet it would *create* the loss: the anchor UTXO remains outside
`mainUtxo`, no Bridge path authorizes its spend once `Closed`, and the owner
keeps their claim — a real `anchorAmount` shortfall, not §0.3's liquidity
mismatch. This is why dissolution carries a proof cycle at all:
`action.actionDataHash = wallet.mainUtxoHash` and
`action.sourceAnchorUtxoHash = anchorUtxoHash(reservation)` (:891-892).
**Any sound unpin must move BTC with an SPV proof.** A proposed "ten-line
governance force-close" was evaluated and rejected on this basis.

### 0.5 "Create" is itself two-phase, and already storage-complete

`requestReservationAcceptance` (:401) plus an acceptance proof — so m1
necessarily ships the action record, `ActionType.Acceptance`, the
designated-wallet binding, and `notifyReservationActionTimeout` (:911).

Acceptance **already populates every field**: `ReservationProofs.sol:527-539`
writes `owner`, `mintedAmount`, `acceptedAt`, `walletPubKeyHash`,
`anchorAmount`, `expiresAt`, `anchorTxHash`, `anchorTxOutputIndex`, `state`
and `dissolutionEligibleAt`.

**Corrected 2026-08-21.** This previously cited `:448-463` (a fee require, not
the position write) and listed `termSeconds` and `gracePeriod` among the fields
written. Neither identifier exists anywhere in the guards or partial trees —
zero grep hits — because #1092/#1093 replaced that expiry model, as §0.1 above
records. The surviving term state is `expiresAt` plus `dissolutionEligibleAt`. So storage-completeness is **already satisfied** — it is an invariant
to *preserve* under any re-scope, not a gap to close (§2.1).

### 0.6 Dissolution is permissionless — leaving it unwired arms a slashing vector

`requestReservationDissolution(uint256)` is `external` with **no modifier**
on the router (`ReservationRouter.sol:302-304`) and no `msg.sender` check in
the library (`Reservation.sol:887-890`, `feat/utxo-reservation-guards`) —
unlike redemption (:634) and renewal (:1153), which are vault-gated. Anyone
can request a dissolution once `block.timestamp >= dissolutionEligibleAt`.

It follows that dissolution **cannot be made unreachable by a minimal
vault**, so it is not deferrable the way redemption and renewal are. Worse,
leaving the keep-core side unwired is not dormancy but a live hazard: if the
wallet does not produce the dissolution Bitcoin transaction before
`reservationActionTimeout`, anyone calls `notifyReservationActionTimeout`
and **the wallet's operators are slashed**. The contract says so directly:
"redemption and dissolution timeouts slash the wallet operators exactly like
a pooled redemption timeout", with `walletMembersIDs` "only consulted for
redemption and dissolution timeouts (the slashing path)"
(`Reservation.sol:961-975`).

So an m1 that ships without keep-core dissolution support hands any
passer-by a **permissionless slashing vector against honest wallets**, armed
automatically the moment the first position passes its dissolution
eligibility date. The term length is therefore not only a promise clock
(§1.4) — it is the deadline by which keep-core dissolution must exist.

### 0.7 The two layers upgrade differently — and B closes one of them

This governs what "minimal" may safely mean, so it is the fact the m1 scope
turns on.

| Layer | Replaceable by | Cost | Minimise in m1? |
|---|---|---|---|
| `ReservationRouter` (Bridge code via `delegatecall`) | Bridge implementation upgrade — `setReservationRouter` is once-only (`BridgeState.sol:1018-1021`), so replacing router code *is* a proxy-admin ceremony (invariant 4, §2) | The ceremony m2 needs anyway for its ~4,641 lines | **Yes — safe** |
| `ReservationVault` (plain `Ownable`, immutables in bytecode, §2.2) | Deploying v2 and re-pointing `Bridge.reservationVault`, which `updateReservationParameters` gates on `reservationTotalAmount == 0 && pendingReservedDeposits == 0` (`Reservation.sol:1263-1274`) | Total quiescence | **Yes, as of 2026-09-24** — Option B accepts the irreversibility below rather than flag-gating (see the note after item 3) |

**In B the vault gate is unreachable while the product is in use.**
`reservationTotalAmount` decrements only when a position closes, and B's
close sites are stranding alone: redemption's (`ReservationProofs.sol:715`)
and dissolution's (`:1140-1142`) are both cut, leaving
`strandReservation` (permissionless, via `notifyReservationStranded`) and
`strandLateSettlementIfTargetWalletClosed`. Stranding needs the custodying
wallet to be `Terminated`, `Closed`, or a `Closing` wallet once its
`dissolutionEligibleAt` has passed (`Reservation.sol:1090-1113` in the M1
code). So reaching `reservationTotalAmount == 0` in B means **every
position has been stranded or otherwise released** — the §5.3 endgame, not
a maintenance window.

Three consequences, and they point in opposite directions:

1. **Minimising the router is free.** Its code was always going to be replaced
   by a Bridge upgrade, so a cut entry point costs no extra ceremony later.
2. **Minimising the vault is irreversible.** Any path whose vault entry point
   m1 omits cannot be reached in m2 by a governance transaction. The fallback
   is a Bridge upgrade that re-plumbs the caller gate — so §0.2's "m2 is a
   *vault-side* change" is **false under B**.
3. **Storage must still be written, not just declared.** Acceptance sets
  `dissolutionEligibleAt = expiresAt + reservationDissolutionDelay`
  (`ReservationProofs.sol:537-539`) and B deletes its only reader
  (re-anchor's gate). (In M1, the Closing branch of `notifyReservationStranded`
  is the one live reader; §0.2.) Drop the write as dead code and m2's dissolution
  has no eligibility date for any m1-era position, with the non-retroactive
  snapshot semantics (`:180-184`) unreconstructable.

**Superseded 2026-09-24 by the Option B vault decision**
(`m1-b-implementation.md` §4.2). This paragraph originally instructed
engineers to close the redemption gap by adding a pause flag, copying
renewal's constructor-paused pattern. **Current answer:** Option B rejected
that fix. The m1 vault ships minimal instead — no `redeemReservation`,
`retryRedeemReservation`, `extendCustody`, or pause flags for either path
(verified against the M1 vault, commit `4d549e64`: 7 external functions,
none of these). The asymmetry above is exactly why that is expensive: m2 must
reach redemption/renewal via a **new** vault deployment plus a depositor
opt-in migration ceremony — the cost this section warned a flag would avoid
(§1.2, §2.2). The in-kind fee pair (`financeInKindFee`, `repayInKindFeeDebt`)
still ships unpaused, as this section always required — that part was never
in question.

### 0.8 A retiring wallet's last exit — automated by keep-core's re-anchor executor

**Current answer:** the below-dust report is automated, not a manual duty.
keep-core's `ReservationReanchorTask` (`pkg/tbtcpg/reservation_reanchor.go`,
`reservations-epic` @ `f66f11240`) already calls
`notifyMovingFundsBelowDustIfEligible` once a drained `MovingFunds` wallet's
reservation count reaches zero, which submits `NotifyMovingFundsBelowDust`
to the chain itself (:664-720; regression test
`TestReservationReanchorTask_Run_NotifiesMovingFundsBelowDust`, :668). A
genuine deadlock still exists only under wallet-slot saturation, where
re-anchor itself has nowhere to drain to (§0.6/§4.1). The analysis below
predates that automation (written 2026-08-21, before `#4274` wired the
executor on 2026-09-03) and is kept as the reasoning trail for *why* the
report must happen, not as a statement that it is still unclaimed.

Added 2026-08-21 from a direct read of the wallet lifecycle, then **corrected
the same day** - the first version of this section claimed a permanent deadlock
and was wrong. The deadlock is real only under saturation; the general case is
an unclaimed operational duty. Both are recorded because the distinction is the
whole point.

`m1-b-implementation.md` §4.1 describes one chain: at saturation re-anchor has
no target, an anchored wallet cannot retire because closing requires a zero
reservation count (`Wallets.sol:674-677`, repeated `:706-709`), its MovingFunds
clock expires, and the timeout seizes operator stake and terminates the wallet
(`:507-513`, then `terminateWallet` at `:515`).

That chain needs the clock to still be running. `notifyWalletFundsMoved`
deletes it on a successful proof - `delete wallet.movingFundsRequestedAt`
(`Wallets.sol:434`) - commented "Zero is the completion sentinel and cannot be
reported as a timeout" (`:431-433`), and `notifyMovingFundsTimeout` requires
`movingFundsRequestedAt != 0` (`MovingFunds.sol:594-598`). So a wallet that
proves its funds moved while still holding anchors is **not slashable**.

It is also not stuck. The exit is
`MovingFunds.notifyMovingFundsBelowDust` (`:627`), which is `external` with
**no modifier** - permissionless. It computes the balance (`:634-637`), requires
it below the dust threshold (`:639-642`), and calls
`Wallets.notifyWalletMovingFundsBelowDust` (`:644`), which checks only
MovingFunds state (`:479-482`) before calling `beginWalletClosing` (`:484`).
Its natspec names the precondition directly: "The wallet must not custody
reservation anchors" (`:620-621`), enforced downstream by the count require.
In this case `mainUtxoHash` was deleted at `Wallets.sol:430`, so the balance is
zero and the dust check passes trivially.

**So the sequence is: re-anchor every position off the wallet until
`walletReservationsCount` reaches zero, then anyone reports below-dust and the
wallet closes cleanly - no slashing, no termination, no stranding.** That is a
better outcome than the §4.1 chain suggests.

The finding that survives is narrower and is an operational one. **Nothing
triggers that report.** Of the three `beginWalletClosing` call sites
(`Wallets.sol:441`, `:484`, `:633`), the one inside `notifyWalletFundsMoved`
(`:441`) runs once, at proof time, when the count is still non-zero, so it is
skipped (`:437-441`). It cannot be retried either: the same function deletes
`movingFundsTargetWalletsCommitmentHash` (`:435`), and re-submitting the proof
requires that hash to match (`:424-427`). The `:633` site is in `moveFunds`,
already past. So `:484` via the permissionless below-dust report is the only
remaining route, and it is a call someone must choose to make.

Two consequences:

1. **The below-dust report is a mandatory keep-core duty — and, as of
   `#4274` (merged 2026-09-03), a built one.** After the last re-anchor off
   a retiring wallet, something must call `notifyMovingFundsBelowDust` or
   the wallet sits in `MovingFunds` indefinitely - not blocked, just
   unattended. `ReservationReanchorTask.Run` now does this itself
   (`pkg/tbtcpg/reservation_reanchor.go:664-720`) immediately after the
   count-draining re-anchor that made it possible, so it needs no separate
   duty-list entry (§5 item 5).
2. **The genuine deadlock is saturation-specific.** If no free slot exists,
   re-anchor cannot drain the count, so all three exits are shut at once:
   closing is blocked on the count, the timeout is unreachable once funds are
   proven moved, and stranding needs a `Terminated` wallet
   (`Reservation.sol:1374-1378`) that never arrives. This is the real
   justification for §4.1's global position cap: the cap is what keeps the
   drain available, and without it the failure is silent rather than a revert.

## 1. Milestone 1 — create-only rails, variant B

**Decided 2026-08-21: m1 is variant B with a minimal router.** This supersedes
the earlier whole-PR plan below and `m1-variant-comparison.md` §6's
recommendation of A+; the argument against is retained there and the residual
risks it names are now **launch gates**, not tradeoffs
(`m1-b-implementation.md`).

### 1.1 Shape: a rewrite, not a PR selection

B is an essentials-only rewrite (`m1-variant-comparison.md` §5.2), so the
eight-PR stack becomes **reference material rather than the delivery
vehicle**. Consequences absorbed elsewhere: `epic-merge-plan.md`'s merge sequence becomes
an extraction order, and `timeline-estimate.md` §7 re-prices m1 from "review +
merge + rework" to writing 5,261 Solidity lines fresh.

What m1 implements: routing and permanent reveal-time classification ·
a minimal `ReservationRouter` (22 entry points: 11 state-changing + 11
views; `m1-b-implementation.md` §2) · two-phase acceptance with
designated-wallet binding · mint and the `mintedAmount == anchorAmount`
backing invariant · **unbounded** re-anchor · action timeout and unwind ·
stale-deposit cleanup · stranding · caps, governance parameters and the new
global position cap · the **complete** storage layout.

### 1.2 Not implemented in m1

- **Dissolution** — B's defining cut. Its storage stays and is still written
  (§0.7), and this is the decision that makes §1.5's pinning permanent.
- **In-kind redemption, whole and partial** (`#1096` included), **renewal**,
  redemption veto, retry credit, late settlement.
- **Bridge-side code for all of the above is simply absent** — not
  deployed-and-dead as in the whole-PR plan. m2 writes it (~4,641 production
  lines, `m1-variant-comparison.md` §5.1).

**Vault-side used to be treated as the opposite of the router, and still is
in one narrow sense.** The two layers upgrade differently (§0.7): router code
is replaceable by a Bridge implementation upgrade, but `ReservationVault` is
effectively immutable once positions exist, and in B they never all close.
**Current answer (Option B, decided 2026-09-24):** the m1 vault ships minimal
anyway, accepting that irreversibility rather than engineering around it — no
redemption or renewal entry points, flag-gated or otherwise (verified against
the M1 vault, commit `4d549e64`). m2 restores both through a **new**
vault deployment plus a depositor opt-in migration ceremony
(`m1-b-implementation.md` §3/§4.2). That migration is precisely the cost §2.2
originally argued a flag-gated vault would avoid; §2.2 is corrected
accordingly below. The superseded plan read: "the m1 vault must ship the full
entry-point surface behind pause flags, even though m1 calls none of it —
omitting a vault entry point in B does not defer that path, it closes it."
The reasoning about irreversibility there is still correct; the conclusion it
drove (flag everything) is not what m1 built.

### 1.3 Launch posture (decided)

Deploy inert, then activate for design partners under a tiny cap
(`feature-spec.md` §11's deploy-inert-then-activate). Small
`reservationMaxTotalAmount`; `maxReservationsPerWallet = 1`. No position
exists until governance flips the switch.

### 1.4 Parameters (decided, restated in upper-stack vocabulary)

`gracePeriod` does not exist in the M1 code (§0.1), so the earlier
"12 months + generous grace" becomes:

| Parameter | m1 value | Note |
|---|---|---|
| `reservationTermSeconds` | **12 months** — a mainnet planning decision, not a script-enforced constant | Promise clock; contributes to the Closing-wallet stranding eligibility timestamp (`dissolutionEligibleAt`). M1 has no dissolution-timeout slashing path (§0.6's vector is unreachable in B). The calldata script only bounds the value to 90–730 days, so the launch runbook must check the supplied value against this decision (`requirements.md` §9) |
| `reservationDissolutionDelay` | generous | Sets `dissolutionEligibleAt = expiresAt + delay`; snapshotted per term granted (:180-184) |
| `reservationRenewalWindowSeconds` | `> 0 && < term` | **Cannot be zero** (:1232-1236); unreachable anyway since renewal is vault-gated |
| `reservationMaxTotalAmount` | tiny | Bounds pooled-liquidity exposure (§0.3) |
| `maxReservationsPerWallet` | 1 | Bounds pinning blast radius |
| `reservationMinAmount` | partner-appropriate | — |

**The term is a promise clock.** Redemption gates on pre-eligibility
(§0.1/§0.2), so if in-kind redemption has not shipped before
`dissolutionEligibleAt`, the first cohort's in-kind option lapses silently
and their only exit was always the pool. 12 months is roughly double a
realistic m2 date. The clock is unextendable in m1 (renewal unreachable), so
publish the derived date with the frozen parameters.

### 1.5 Wallet pinning under B: re-anchor is the only unpin, and it is unbounded (revised three times)

**Revised 2026-08-21 by the B decision.** The prior conclusion — pinning
solved *inside* the term by re-anchor and *after* it by dissolution — assumed
dissolution ships. B removes it, so the rule is now simpler and the residual
risk is larger.

The history matters because two premises were falsified in turn. The original
decision accepted pinning because no unpin would ship; that was false, since
re-anchor is permissionless while the source wallet is `MovingFunds`
(`:718-757`) — exactly the retiring-wallet case. The interim conclusion was
that re-anchor alone solved it; that was also false on the stacked design,
because re-anchor required
`block.timestamp < reservation.dissolutionEligibleAt` (`:785-788`), so past
eligibility every path closed except dissolution.

**B resolves this by deleting the gate.** Re-anchor becomes unbounded, so it
works at any position age, and that is precisely what makes dropping
dissolution sound rather than reckless — a position past eligibility still has
a movement path. `m1-variant-comparison.md` §2 records the ordering
constraint: the two changes are a package, and shipping the cut without the
unbounding would leave positions with no unpin at all.

What B accepts in exchange:

- **Re-anchor needs a free slot,** so pinning relief is now conditional on
  target-wallet supply rather than on a date. `activeReservationsCount` and
  its cap exist to keep that condition true
  (`m1-b-implementation.md` §4.1); saturation is the failure mode
  (`m1-variant-comparison.md` §5.3).
- **There is no owner-side exit at any age.** Redemption and renewal are
  vault-gated and flag-paused, dissolution does not exist, and stranding needs
  the wallet `Terminated`. A position ends only when its custodying wallet
  dies.
- **Re-anchor is the sole keep-core execution duty for position movement,**
  which is B's saving — but it also removes the redundancy the two-path design
  had. Executor failure has no fallback.

Residual conditions:
- Re-anchor needs the keep-core executor to sign and prove (§5). This is
  now the only such duty, so it carries the whole pinning guarantee.
- While `Live`, re-anchor requires `privileged` — governance-driven rotation
  only. Acceptable at design-partner scale.

## 2. The upgradeability contract

### 2.1 Preserve storage-completeness (already true)

Acceptance writes every field today (§0.5). The rule is therefore
**defensive**: any re-scope of the acceptance path must keep writing
`expiresAt` and `dissolutionEligibleAt` (the two fields that exist; an earlier
version of this sentence also named `termSeconds`, which does not). If a future edit
dropped them, m2 would read zeros and every m1 position would be instantly
dissolution-eligible — permanently barring the earliest users from in-kind
redemption. Add a test asserting the fields are non-zero after acceptance.

### 2.2 m2 needs a Bridge upgrade under B, and the vault is NOT upgradeable

**Current answer:** m2 needs a full Bridge implementation upgrade
(redemption/renewal code is absent from m1's Bridge, not just gated);
`ReservationVault` is plain `Ownable` and cannot be upgraded in place; and —
per the 2026-09-24 Option B decision — m1 ships that vault minimal rather
than flag-gating a redemption path, so m2 also deploys a **new** vault and
runs a depositor migration ceremony. The paragraphs below trace how this
answer was reached across three corrections, ending with the (now
superseded) flag-gated recommendation this section originally made.

**Premise corrected 2026-08-21.** This section previously read "m2 needs no
Bridge upgrade" on the grounds that "redemption and renewal are vault-gated
(§0.2) and their Bridge-side code ships in m1, enabling them is a
**vault-side change**, not a Bridge storage migration". That held for the
create-only whole-PR plan, where the Bridge-side code shipped and sat
unreachable behind caller gates. **It is false under B** (§0.7 item 2): B does
not ship that code at all, so m2 must write and deploy roughly 4,641 production
Solidity lines of redemption, renewal, veto integration and dissolution
(§3.1). That is a Bridge implementation upgrade.

What survives is the storage half of the argument.
`feature-spec.md` §11's no-live-action-on-intermediate-layout rule is still
satisfied by construction, because m1 ships the **complete** layout (§2.1) and
the only m1 in-flight records are acceptance and re-anchor actions, both
transient (bounded by `reservationActionTimeout`). So m2 adds code against an
unchanged layout: an upgrade, not a migration.

**Resolved 2026-08-21: `ReservationVault` is not proxy-upgradeable.**
`contract ReservationVault is IVault, IReservationFeeFinancer, Ownable`
(`:57`) — plain `Ownable`, no
`Initializable`, and four `immutable` state variables (`bank`, `tbtcVault`,
`tbtcToken`, `bridge`) set in a `constructor`
(`contracts/vault/ReservationVault.sol:57`, `:69-72`, `:183-223`).
`deploy/95_deploy_
reservation_vault.ts:12-17` is a plain `deployments.deploy` with constructor
args and no proxy option. Immutables are baked into bytecode, so a proxy
cannot be retrofitted without refactoring the contract.

So enabling redemption means **replacing** the vault and re-pointing the
Bridge's `reservationVault`. That re-point is **contract-enforced, not
merely discouraged**: `updateReservationParameters` reverts a vault change
unless `reservationTotalAmount == 0` **and** `pendingReservedDeposits == 0`
(`Reservation.sol:1263-1274` on `feat/utxo-reservation-guards`). Nothing is
silently orphaned — the transaction simply fails. (An earlier draft of this
section claimed a swap orphans revealed-but-unaccepted deposits; that is
wrong, and `feature-spec.md` §15 has been corrected to record the guard as
enforced.)

The consequence is worse than orphaning would be, because it is a
**liveness** constraint: the swap is impossible until *every* position has
closed and *every* revealed deposit has been accepted or marked stale. In a
create-only m1 the only position-closing paths are permissionless
dissolution after `dissolutionEligibleAt` (§0.6) and stranding of a
`Terminated` wallet. With a 12-month term (§1.4), the m2 swap therefore
cannot happen until roughly a year after the *last* position was accepted —
and each new acceptance pushes that date out. A vault swap is effectively
unavailable for as long as the product is being used.

**Superseded 2026-09-24 by the Option B vault decision.** This subsection
used to recommend shipping the m1 vault with the redemption entry point
present but disabled by an owner-settable flag, "avoiding the swap entirely."
That recommendation was rejected. It is kept below, struck through, because
it is the exact argument the 2026-09-24 decision weighed and declined — not
because it was wrong about the swap's cost, which the rest of this section
still documents accurately.

~~**Recommended m1 vault design (avoids the swap entirely):** ship the m1
vault **with** the redemption entry point present but disabled by an
owner-settable flag, so enabling it later is a governance transaction rather
than a swap the guard blocks indefinitely. This keeps create-only behaviour
(unreachable while the flag is false). Costs: the vault's redemption plumbing
is deployed and audited in m1, and the flag becomes a governance-safety item
(accidental enable). Given the vault cannot be upgraded *and* the swap is
gated on total quiescence, this is no longer a convenience — without the
flag, m2's in-kind redemption promise has no reachable delivery path while
positions keep being created.~~

**Current answer:** the m1 vault ships minimal instead
(commit `4d549e64`; `m1-b-implementation.md` §3/§4.2) — no redemption entry
point, flagged or otherwise. m2 reaches redemption/renewal through a **new**
vault deployment plus a depositor opt-in migration ceremony, i.e. exactly the
"swap the guard blocks indefinitely" this paragraph warned against. That
cost was accepted deliberately: the project owner confirmed on 2026-09-28
that m2 will perform the new-vault migration. The choice imposes one M2
design requirement (verified in the M1 code): the m1 re-point guard —
`updateReservationParameters` only allows changing `Bridge.reservationVault`
while `reservationTotalAmount == 0 && pendingReservedDeposits == 0`
(`Reservation.sol` ~:1300-1317) — is under variant B reachable only once
every position has been stranded or released, so the migration requires m2's
Bridge upgrade to change the vault-binding rule (the mechanism, e.g.
per-position vault binding or a governed migration path, is an m2 design
choice; the guard is Bridge-side library code, replaceable by that upgrade),
and the m1 vault must keep serving existing positions' settlement paths
(e.g. `financeInKindFee` on re-anchor) until they migrate.

**Two claims here corrected 2026-08-21, now both moot under Option B.** The
struck-through paragraph previously said the flag makes m2 "a single
governance transaction (`setRedemptionsEnabled(true)`)" and called it "the
same principle already accepted for the Bridge-side redemption code
(§0.2/§1.2)". Both assumed the whole-PR plan; under B the Bridge-side
redemption code is **not** deployed either way, so neither claim describes
current or superseded reality — both were about a plan that no longer exists
in either form.

### 2.3 The Bitcoin side is the only true one-way door

Anchor shape (1-in-1-out to the designated wallet), the reveal-script
commitment, and anchor identification (`anchorTxHash` / output index) are on
Bitcoin for every accepted position and cannot be re-shaped by an upgrade.
Pre-launch scrutiny belongs here.

## 3. Milestone 2 - what it restores and what it inherits

m2 is a comparable project, not a follow-up:
5,261 production Solidity lines in m1 against 4,641 in m2, a ratio of
**1.13 : 1** (`m1-b-implementation.md` §6; derived from
`m1-variant-comparison.md` §5.1's 3,731 plus dissolution's 910). **This ratio
predates the 2026-09-24 Option B vault decision and has not been
re-derived** (§5.1): it counts a redemption pause flag inside m1's 5,261 and
no new-vault-deployment cost inside m2's 4,641, both of which the decision
changed (§1.2, §2.2).

### 3.1 What m2 restores

| Capability | Approximate production Solidity | What m1 has for this path |
|---|---|---|
| In-kind whole redemption, veto integration, retry credit, late settlement | ~3,035 with renewal and storage | Nothing — no `redeemReservation`/`retryRedeemReservation` in the m1 vault (commit `4d549e64`); m2 deploys a **new** `ReservationVault` carrying them plus a depositor migration ceremony (§1.2, §2.2) |
| Renewal / term extension | included above | Nothing — no `extendCustody` in the m1 vault either; same new-vault path as redemption |
| Partial redemption, 1-in-2-out (`#1096`) | ~696 | Nothing; shares the new vault's redemption entry points once m2 ships them |
| Dissolution restored | ~910 | none needed: dissolution is Bridge-side only |

Redemption and renewal are **vault-gated**, but under Option B (2026-09-24)
m1 ships none of that gate — m2 must deploy a whole new vault and run the
migration ceremony before those entry points exist at all (§2.2). This is not
merely necessary-but-not-sufficient the way a flag flip would have been; it
is the full cost, both the Bridge-side code (~3,035+696 lines, absent from
m1) and the vault redeploy. Dissolution is **not** vault-gated: it is
permissionless (§0.6) and lives entirely in Bridge code, so it needs no vault
action at all, only router and library code plus a keep-core executor. These
two halves of m2 have different risk profiles and need not ship together.

### 3.2 What m2 inherits from the B decision

Three consequences A+ would not have created. All three are decisions, not
chores, and none of them is forced by the code:

1. **Whether to restore re-anchor's eligibility gate.** m1 deletes
   `block.timestamp < reservation.dissolutionEligibleAt`
   (`Reservation.sol:785-788`) to make re-anchor unbounded, which is what
   makes dropping dissolution sound (§1.5). Restoring dissolution does not
   restore the gate. Leaving it out means a position can be rotated
   indefinitely past its eligibility date.
2. **Whether m1-era positions get m2 semantics.** They carry `expiresAt` and
   `dissolutionEligibleAt` snapshotted under m1 parameters, and `:180-184`
   makes those non-retroactive by design. So the first cohort's term is fixed
   at whatever m1 set, and cannot be extended by a later governance change.
3. **The in-kind promise has a dated deadline, not an open one.** The
   12-month term (§1.4) dates the first cohort's earliest
   `dissolutionEligibleAt`. If m2's redemption has not shipped by then, those
   positions' only exit remains the pool. The term converts a launch blocker
   into a scheduling commitment.

### 3.3 What m2 must not have to do — updated: the vault half no longer holds

**Current answer:** m2's *Bridge*-side upgrade is still constrained this way
— the complete storage layout (§2.1) and every field m2 reads must already be
written by an m1 path (`milestone-inventory.md`), so Bridge code is an
upgrade, not a migration. **The vault half of this goal was explicitly
abandoned by the 2026-09-24 Option B decision** (§1.2, §2.2): m1 does not
ship every vault entry point, so m2's vault side genuinely *is* a migration —
flag flip. That cost was accepted deliberately — confirmed by the project
owner on 2026-09-28 — and imposes one m2 design requirement: m2's Bridge
upgrade must change the vault-binding rule, because m1's re-point guard is
unreachable under B (§2.2 "Current answer").

The original goal, still true for Bridge storage: m2 must be an **upgrade,
not a migration**, on the layout side. That requires m1 to write the
complete storage layout (§2.1) and every field m2 reads, written where an m1
path writes it (`milestone-inventory.md`). Failing this turns a Bridge
upgrade into a full layout migration — still true, and still a launch gate.

## 4. How m1 ships relative to the existing PRs

This section replaces the earlier "optimal merge order" and "suggested edits to
existing PRs". Both assumed the eight-PR stack was the delivery vehicle. Under
B it is **reference material**, so the question changed from "in what order do
we merge these" to "what do we extract from them, and how do we preserve them".
`pr-strategy.md` holds the mechanics and the recommendation; this section holds
only the facts that constrain it.

### 4.1 Measured state of the stack (2026-08-21)

Three branches are behind their bases, not one. The earlier claim that `#1090`
was the only CONFLICTING PR is **out of date**:

| PR | Behind its base | Consequence |
|---|---|---|
| `#1088` | 2 commits | `reservations-epic` moved on with an unrelated security fix (`#1098`); the epic branch is not frozen |
| `#1090` | 0 | Since rebased; **contains** the `#1102` fold |
| `#1091` | **13 commits** | The fold stops here |
| `#1092` | 5 commits | |
| `#1093`-`#1096` | 0 | |

### 4.2 The `#1102` fold does not reach the tip anyone is citing

`3566e059` is present on `feat/utxo-reservation-core` and
`feat/utxo-reservation-router`, and **absent** from
`feat/utxo-reservation-guards` and `feat/utxo-reservation-partial-redemption`,
because `#1091` is 13 commits behind `#1090`.

This matters beyond extraction: `m1-b-implementation.md` and `feature-spec.md`
both source-verify against the guards tip, which predates 30 review fixes
touching 10 production files (+685 -190), `Reservation.sol` most of all
(+342 -95). Their line numbers are pre-fix. Resolving this is the first
concrete action in `pr-strategy.md`.

### 4.3 Extraction hazards, in priority order

1. **`#1091` rewrites `#1088` heavily** (-1266 production lines). Never
   extract the `#1088` form of anything `#1091` touched; the single-phase
   mechanics were replaced by the two-phase machine.
2. **`#1093` reworks `#1091`'s backing model** (the H-04 finding). Take the
   `#1093` form of anything touching `mintedAmount` or `anchorAmount`.
3. **The reverse anchor index has two write sites**, `#1091` and `#1094`'s
   stranding write. Both must be carried, because stranding is B's only
   position-closing path. **Correction to the earlier claim in this section:**
   `#1102` did not remove `reservationsByAnchorUtxo` in favour of
   `spentMainUTXOs`. The mapping does not exist on `#1088`'s branch at all -
   `#1091` introduces it - and `spentMainUTXOs` is a pre-existing Bridge
   registry that reservations write into (`Reservation.sol:1454`, `:1510`),
   not a competing design. There is no removal to reconcile, only two write
   sites.
4. **`#1092` cannot be skipped** even though renewal is deferred: its expiry
   and snapshot semantics are structural to `#1093`+ (§0.1).
5. **`#1095` contains no production Solidity.** Docs source only.

### 4.4 keep-core `#4238`

It models the whole-redemption *transaction* shape and predates the
machine, so it is a **rewrite**, not an edit. Under B it needs acceptance and
re-anchor as nonce-carrying proposals, and **not** dissolution - which is the
one place the B decision genuinely reduces client scope, worth roughly 300-500
production Go lines.

**This reverses what this section said before.** The earlier text argued
dissolution was "*not* optional" for m1 because it is permissionless and cannot
be gated off by a minimal vault. That reasoning was right for a create-only m1
built from the stack, where the dissolution entry point ships whether or not
the client supports it. It does not apply to B: B removes
`requestReservationDissolution` from the router entirely, so there is no entry
point to leave unwired and no slashing vector to arm (§0.6).

## 5. Implementation gaps for m1 (high level)

1. **keep-core two-phase client** — acceptance and re-anchor only, with
   nonce-carrying proposals, executor duties, and regenerated ABI bindings.
   **Corrected 2026-08-21:** this item previously read "acceptance, re-anchor,
   **and dissolution**... Dissolution is mandatory per §0.6" and called that
   the critical path. Under B that is false - the router has no
   `requestReservationDissolution`, so there is no vector to arm (§4.4, §6
   reversed item 3, `milestone-inventory.md` C-7). Dropping the dissolution
   executor is B's one genuine client-side saving, roughly 300-500 production
   Go.
   It remains the critical path, but for a different reason: `#4238` supplies
   types, proposals and Bitcoin-tx assembly and **contains no executor at
   all** - `pkg/tbtcpg` has no reservation task and the chain interface has no
   submission method - so this is new code rather than rework
   (`milestone-inventory.md` §2.9, C-8).
2. **m1 `ReservationVault`** ships minimal (Option B, 2026-09-24) — the
   acceptance/credit path plus the settlement-path fee functions
   (`financeInKindFee`, `repayInKindFeeDebt`) only. Redemption and renewal
   entry points are entirely absent (no `redeemReservation`,
   `retryRedeemReservation`, `extendCustody`, no pause flags), to be
   introduced by m2 via a new vault deployment and depositor migration
   ceremony (§1.2, §2.2). §0.7's irreversibility rule still explains why
   that migration is costly — it does not mean m1 must avoid it, since
   avoiding it is exactly what Option B declined to do.
3. **Anchor-index reconciliation** across `#1091`/`#1094` (§4.3 item 3).
4. **Parameter and activation wiring** — §1.4 values, the deploy-inert
   switch, and governance runbook steps.
5. **Executor duty: re-anchor on rotation — already implemented, not a
   remaining gap.** `pkg/tbtcpg/reservation_reanchor.go`'s
   `ReservationReanchorTask` (keep-core `reservations-epic` @ `f66f11240`,
   wired by `#4274`, merged 2026-09-03) drains a `MovingFunds` wallet's
   reservations via re-anchor proposals, prompted by the wallet entering
   `MovingFunds`, and — once none remain — calls
   `notifyMovingFundsBelowDustIfEligible` (:664-720) to close the §0.8 gap in
   the same task. Regression test:
   `TestReservationReanchorTask_Run_NotifiesMovingFundsBelowDust` (:668).
   §1.5's unpinning guarantee depends on this task continuing to run; it is
   no longer unbuilt.
6. **Monitoring** — anchored wallets (pinning watch); earliest
   `dissolutionEligibleAt` per position, tracked as **the promise clock
   only** (the §0.6 slashing vector does not arm in B — there is no
   `requestReservationDissolution` entry point to leave unwired, §0.6/§4.4);
   pooled-liquidity exposure vs cap. "Pending dissolution actions
   approaching timeout" is dropped from this list: no dissolution action
   type exists in m1 to monitor.
7. **Tests** — acceptance happy path; timeout and stale-deposit cleanup; cap
   enforcement; a storage-completeness assertion (§2.1). Two items from an
   earlier version of this list no longer apply and are removed: a
   **reachability test** for redemption/renewal vault callers (there is no
   `redeemReservation`/`extendCustody` entry point in the m1 vault to test
   against — Option B, 2026-09-24, §1.2/§2.2) and a **dissolution-timeout
   test** (there is no `requestReservationDissolution` entry point in m1 at
   all, §0.6/§4.4). Both move to m2 once their respective code ships.

## 5.1 Code mass: m1 vs m2, measured (2026-08-21)

**Superseded by the B decision (§1) as a statement about m1; retained as the
measurement of the stacked stack.** Everything in this subsection describes the
whole-PR plan: m1 = 7 PRs, 9,206 production lines, 445 deployed-but-unreachable,
93% of the feature audited in m1, dissolution reachable. Under B none of that
holds — m1 is a 5,261-line rewrite with no dead weight, and dissolution is not
reachable at all. The live figures are §3 and `m1-variant-comparison.md` §3/§5.1.

**Those "live" figures are themselves stale as of 2026-09-24.** The 5,261
(m1) / 4,641 (m2) split (§3) was derived assuming the flag-gated-vault plan
(pre-Option-B §0.7/§1.2/§2.2): it counts a redemption pause flag inside the
5,261 and no new-vault-deployment cost inside the 4,641. Both totals move
under Option B — down in m1 (no flag, no `financeInKindFee`/
`repayInKindFeeDebt` pause wiring to add) and up in m2 (a second
`ReservationVault` deployment plus migration code) — and have not been
re-measured post-decision.

Measured from the PR diffs (`gh api .../pulls/N/files`, additions only) and
function-level classification of the four reservation contracts at the m1 tip
(`feat/utxo-reservation-guards`). LoC is a weak proxy for audit cost, but it
is the requested unit and the ratios are informative.

### Shipped code, by milestone

| Bucket | m1 (`#1088`-`#1095`, 6 code PRs) | m2 (`#1096`) | Ratio |
|---|---|---|---|
| Production Solidity | **9,206** | **696** | 13 : 1 |
| Solidity tests | 15,896 | 2,441 | 6.5 : 1 |
| Deploy scripts | 432 | 0 | — |
| Docs in-repo | 1,340 | 108 | — |
| Other (ABI JSON, config, lockfiles) | 167 | 14 | — |
| **Total additions** | **27,041** | **3,259** | **8.3 : 1** |

**Two corrections, 2026-08-23, from re-measuring this table.**

1. **The unit is an additions-sum and is inflated by intra-stack churn.** The
   9,206 reproduces exactly (`#1088` 3,393 + `#1090` 874 + `#1091` 3,594 +
   `#1092` 351 + `#1093` 457 + `#1094` 537), so the arithmetic is sound. But the
   whole stack's **net** contract diff against `main` is only **5,958 additions
   / 48 deletions**. The 3,248-line difference is lines added by one PR and
   rewritten by a later one, plus non-composing PR bases from the `#1091`
   staleness (`pr-strategy.md` §7). Confirmation: `#1096`'s base was current, and
   its PR-additions (696) equal its net delta (696) exactly, while the stale
   branches' PR-diff deletions sum to 1,792 against a net 48.
2. **`#1095` contributes 0 production Solidity.** The bucket header said 7 PRs;
   only six carry contract code. `#1095` is docs and tests.

Downstream figures derived from 9,206 by subtraction — `m1-variant-comparison.md`
§3's 6,171 / 5,435 / 5,261 / 4,525 — inherit the inflated unit. Measured m1
content is **~4,500 net contract lines**
(`agent-docs/m1/step-04-line-count-reconciliation.md`).

keep-core `#4238` adds 919 production Go + 914 test Go, but models the
pre-two-phase design (§4.4), so it is a starting point rather than
shippable m1 content.

Test-to-production ratio on the m1 stack is 1.73:1 — the Solidity is heavily
tested, consistent with `feature-spec.md` §16's "thorough" verdict.

### What the carve-out actually costs

**m1 audits 93% of the feature's production Solidity** (9,206 of 9,902).
Deferring `#1096` removes 696 production lines — about 7%. The carve-out is
achieved by *gating* rather than removing code (§0.2), so almost nothing
leaves the audit surface.

Unreachable-but-deployed logic in m1, by function body (code lines, comments
excluded):

| Path | Functions | Code lines |
|---|---|---|
| Redemption | `requestReservedRedemption` (102), `submitReservedRedemptionProof` (88), `redeemReservation` (38), `retryRedeemReservation` (33), `notifyReservedRedemptionVeto` (31), `resolveLateRedemptionAgainstPending` (23), router wrappers (21) | **336** |
| Renewal | `extendReservation` (49+11), `extendCustody` (29), renewal pause/block/guardian (20) | **109** |
| **Dead weight total** | | **445** |
| Reachable in m1 | acceptance, re-anchor, dissolution, stranding, timeout, caps, params, getters | 1,701 |

So m1 carries **445 dead code lines against 1,701 reachable — 20.7% of the four
reservation contracts' function bodies**, or 4.8% of the 9,206-line
production-Solidity diff total. The two bases are not comparable: the table
above excludes comments, the 9,206 figure includes them and spans files the
classification never touched. The 20.7% figure is the one that matches the
table.

**Corrected 2026-08-21.** This previously read "445 code lines it cannot
execute — 4.8% of its production Solidity. That is the true price of the
carve-out, and it is small." The 4.8% divided a comments-excluded numerator by a
comments-included denominator, flattering the conclusion by roughly 4x. On the
table's own basis the carve-out costs a fifth of the reservation contracts'
function bodies, not a twentieth.

### Effort to prepare m1 for release

Nearly all of it is keep-core Go, not Solidity:

| Item | Est. LoC | Basis |
|---|---|---|
| keep-core two-phase rework (acceptance + re-anchor + **dissolution**) | ~1,400-1,900 prod Go, ~1,000-1,500 test Go | `#4238`'s 919+914 largely rewritten; 3 of 4 action types, nonce-carrying, 6 stubbed `TbtcChain` methods implemented |
| ~~Redemption pause flag on the vault~~ (superseded 2026-09-24) | 0 — not built | Option B ships the vault without redemption/renewal entry points at all (§1.2, §2.2); the deferred cost moves to m2's new-vault-deployment line below |
| Anchor-index reconciliation (`#1091` :465 vs `#1094`) | ~20-60 prod, ~100 test | §4.3 item 3 |
| m1-specific tests (reachability, storage-completeness, dissolution-timeout slashing) | ~200-400 test | §5 item 7 |
| Params, deploy and activation wiring | ~100-200 | §1.3/§1.4 |
| `#1090` rebase | ~0 net | Conflict resolution, no new logic |
| **Total** | **~2,900-4,200** | ~85% keep-core Go |

### Effort to upgrade m1 to m2

| Item | LoC | Note |
|---|---|---|
| `#1096` partial redemption | 3,259 | **Already written** — an open PR, not new work |
| Enable redemption | not applicable | Superseded 2026-09-24: the "recommended design"'s owner-settable flag was not built; m2 must deploy a **new** `ReservationVault` with `redeemReservation`/`retryRedeemReservation` and run the depositor migration ceremony (§2.2). That vault-deployment-plus-migration cost is not captured in this table's LoC estimates and has not been re-derived post-decision (§5.1). |
| keep-core redemption proposal + partial assembler | ~300-600 prod Go, ~300-500 test Go | The one genuinely new build |
| **New code to write** | **~600-1,100** | Everything else exists |

**Headline:** m1 is ~8x m2 in shipped lines (27.0k vs 3.3k), but the
asymmetry inverts for *remaining* work — m2's Solidity is already written,
so upgrading costs ~0.6-1.1k new lines plus a second audit engagement, while
reaching m1 costs ~2.9-4.2k. The carve-out buys a later deadline, not less
code (`timeline-estimate.md` §6), and it leaves 445 lines deployed but
unreachable to buy that deferral.

## 5.2 What an essentials-only rewrite would cost (2026-08-21)

Asked: if m1 were written from scratch with nothing but essentials, how much
smaller would it be? Estimated per file from §5.1's measurements, with an
explicit keep-factor per file rather than a blended ratio.

**Answer: a third to a half smaller, not an order of magnitude** — and the
reason is the useful part.

### Why the ceiling is low

Redemption and renewal are only **445 of 2,146** function code lines (21%)
in the four reservation contracts. The mass is the **anchor lifecycle** —
two-phase request/proof plumbing, SPV output validation, per-wallet
enumeration, action records and timeout unwinding — and every one of those is
required to create a single reservation. `settleAcceptance` (94),
`unwindPendingAction` (58), `submitReservationProof` (45),
`prepareReservationForSettlement` (42) are not redemption features; they are
what "create" costs.

### Variant A — essentials-only rewrite

Cut redemption and renewal entirely, along with the files that exist only to
serve them (`RedemptionWatchtower.sol` +415 veto integration,
`Redemption.sol` +109 pooled-path hooks), the storage they need
(`reservedRedemptionSettlements`, veto delay, retry credit, renewal window)
and their natspec and tests.

| | Production Solidity | + tests (1.7-1.9x) |
|---|---|---|
| m1 as stacked | 9,206 | ~24,900-26,700 |
| Variant A | **6,171** (-33%) | ~16,700-17,900 |
| Variant A, no router | **5,435** (-41%) | ~14,700-15,800 |

The router is the largest **single-file** swing in this estimate.
`ReservationRouter.sol` ships 1,051 production lines plus ~2,400 test lines
in `#1090`, and exists only because the full external surface breaks Bridge's
EIP-170 bytecode limit. An essentials-only surface has roughly 11 entry
points instead of 24, so it **may fit in `Bridge` directly** — worth an hour
with the compiler before assuming either way.

On a like-for-like basis, though, it is **not** the largest decision in this
section. Measured as within-variant deltas, deleting the router saves 736
production lines (~2,000 with tests), while the A+ -> B dissolution drop saves
910 (~2,500). Dissolution is the bigger swing on both axes; the 1,051/~3,400
figures above are the file's *stacked* mass, not the saving a rewrite
realises, because the rewritten router is itself leaner.

### variant B — also drop dissolution, make re-anchor unbounded

The deeper cut is available only when writing from scratch, because it
changes the protocol rather than omitting a PR. Drop dissolution from m1
(~310 function lines, ~910 shipped) and remove re-anchor's
`< dissolutionEligibleAt` gate (§1.5) so wallet rotation never expires.

| | Production Solidity | + tests |
|---|---|---|
| variant B | **5,261** (-43%) | ~14,200-15,300 |
| variant B, no router | **4,525** (-51%) | ~12,200-13,100 |

What B buys beyond the lines is worth more than the lines:

- **The §0.6 slashing vector disappears.** No dissolution entry point means
  no permissionless request that slashes a wallet on timeout.
- **The mandatory keep-core duty disappears with it** (~300-500 Go lines),
  leaving acceptance and re-anchor — and re-anchor is the one the wallet
  lifecycle needs anyway.
- **§1.5's pinning cliff disappears as a timer.** An unbounded re-anchor lets
  a wallet rotate its anchors out at any position age — but only into a Live
  wallet with a free slot, which is a weaker guarantee than it sounds (below).

What B costs is sharper than a cap resize. Nothing closes a position except
wallet termination, so **both** on-chain-enforced caps become cumulative-ever
rather than concurrent (feature-spec §10: the global amount and the per-wallet
count are the only two that are contract-verified):

- `reservationTotalAmount` never decrements, so the global amount cap must be
  sized for lifetime usage and raised by governance when it fills.
- More binding: with the decided `maxReservationsPerWallet = 1` (§1.3, §1.4),
  each live reservation **permanently occupies one wallet**. Re-anchor
  requires `targetCount <= maxReservationsPerWallet`
  (`Reservation.sol:820-827`), so a hop needs a Live target holding zero
  anchors, and the source slot is only released at settlement (`:819`) — an
  in-flight hop holds two.

So B's pinning fix is **conditional on free slots existing somewhere**, where
total slots are `walletCount x maxReservationsPerWallet`, whereas A+'s is
unconditional because dissolution recycles slots.

That condition is a governance dial, not a supply race. `maxReservationsPerWallet`
is validated only for the parameters around it — `updateReservationParameters`
bounds the fee, min amount, term, renewal window and action timeout, but
places **no bound on the count cap** (`Reservation.sol:1239-1281`) — and it is
stored live rather than snapshotted per position (`:1281`, read at `:824`), so
raising it takes effect for existing wallets immediately. Moving the m1 launch
value of 1 to the documented default of 10 (feature-spec §10) multiplies
capacity tenfold with zero new wallets.

The real asymmetry is therefore **depleting versus renewing**, not bounded
versus unbounded:

- **A+**: slots recycle on every close, so capacity is self-renewing and
  needs no attention.
- **B**: slots are consumed monotonically. The knob multiplies the budget but
  never refills it, and turning it up concentrates permanent pinning on
  individual wallets — which is exactly what §1.4 set the cap to 1 to bound.

So B trades a self-healing property for a managed one: an occupancy metric to
watch, two dials to turn, and an accepted rise in per-wallet pinning
concentration each time. At design-partner volumes (§1.3) the budget never
depletes and the duty is theoretical; at production volumes it does, which is
why B is explicitly a launch-posture design that m2 must replace.

Re-anchor cannot itself be cut: a wallet holding anchors cannot begin closing
until its reservation count reaches zero (§7 guards), so with no re-anchor
and no dissolution, every anchored wallet is pinned for the whole term.

### The variants are a ladder, not a menu

A and B are not alternatives: **B is A plus two further changes**, so its
numbers already include A's cuts. The two further changes are also ordered
rather than independent, which creates a useful intermediate rung.

**Unbounding re-anchor is a prerequisite for dropping dissolution, not a
companion to it.** In a create-only m1 the only paths that close a position
are `settleDissolution` (`ReservationProofs.sol:1140-1142`) and
`strandReservation`, which requires the wallet to be `Terminated`; the
remaining close site (`:715`) sits inside `submitReservedRedemptionProof` and
is unreachable (§0.2). **`ReservationProofs.sol:836` is not a close site** and
must not be cut: it is the re-anchor source-wallet count decrement inside
`submitReservationReanchorProof` (`:743`), paired with the target's increment
taken at request time (`Reservation.sol:820-827`). It is reachable in m1 and
load-bearing — it is the write that lets a re-anchored-away wallet's count fall
to zero, which is the precondition for `beginWalletClosing`
(`Wallets.sol:674-677`) and for §0.8's below-dust exit. Cut it and re-anchor
moves a position while leaving the source slot occupied forever, so occupancy
becomes monotonic even with re-anchor working (`milestone-inventory.md` C-2). Dropping
dissolution while keeping re-anchor's `< dissolutionEligibleAt` gate
therefore leaves a position past its eligibility date with **no unpin path at
all** — strictly worse than the stacked design, where dissolution eventually
opens. The converse is not true: re-anchor can be unbounded while dissolution
stays, which is exactly the §7 item 6 harvest.

| Rung | Change | Production Solidity | §1.5 pinning | §0.6 slashing vector | Amount cap | Wallet slots |
|---|---|---|---|---|---|---|
| 0 | m1 as stacked | 9,206 | cliff at eligibility | present, keep-core duty mandatory | concurrent | freed on close |
| 0+ | harvest only (§7 item 6) | 9,206 (-4) | fixed unconditionally | present, mandatory | concurrent | freed on close |
| A | cut redemption + renewal | 6,171 / 5,435 | cliff at eligibility | present, mandatory | concurrent | freed on close |
| A+ | A + unbounded re-anchor | 6,171 / 5,435 | fixed unconditionally | present, mandatory | concurrent | freed on close |
| B | A+ and drop dissolution | 5,261 / 4,525 | fixed **while free slots remain** | **removed** | cumulative-ever | never freed; budget = `walletCount x cap` |

(Second figure is the no-router case. Rungs 0+ and A+ cost nothing in lines —
they delete a `require` — so they are free relative to the rung below.)

**A+ is the free rung.** It closes the pinning cliff for the price of
deleting a `require`, and because dissolution survives, positions still close,
so both caps stay concurrent and slots keep recycling. Everything from rung 0
to A+ is either free or a pure omission, so A+ dominates 0, 0+ and A outright.
It does **not** dominate B — B is a further real trade, below.

**The A+ -> B step is the only real trade, and it has two axes.** It buys
removal of the §0.6 slashing vector and the mandatory keep-core dissolution
duty — the latter sitting on the serial pre-audit path (§5 item 1), so this is
a schedule saving and not only a code saving. It pays with (a) cumulative-ever
cap sizing and (b) a pinning guarantee that degrades from self-renewing to
governance-managed, since slots are consumed monotonically.

Axis (b) looked smaller than it is. The ceiling is raisable on demand — total
slots are `walletCount x maxReservationsPerWallet`, the count cap carries no
validation bound and applies live to existing wallets, and the m1 launch value
of 1 sits a factor of ten below the documented default. But raising it only
defers, because occupancy is monotonic when nothing closes, and
`m1-variant-comparison.md` §5.3 traces the terminal state: at saturation
re-anchor has no target, `beginWalletClosing` cannot pass its zero-count
requirement (`Wallets.sol:674-677`), the MovingFunds clock expires, and
`notifyWalletMovingFundsTimeout` seizes operator stake and terminates the
wallet (`:493-523`) — after which the depositor's position strands. Honest
operators slashed, depositors stranded, reached by arithmetic rather than by
an attacker.

That reframes the trade. §0.6's vector is a **liveness duty**, dischargeable
by wiring keep-core; B's cliff is a **capacity limit**, dischargeable only by
an on-chain bound. B is therefore acceptable only with a global
active-position cap held below the slot floor (`m1-variant-comparison.md` §5.4
item 1, ~20 lines plus a parameter, since no global position counter exists
today beside `liveWalletsCount`). **Recommendation: implement A+**; the full
argument, the B hardening list and the operational duties are in
`m1-variant-comparison.md` §5.3-§6.

### The cost side of a rewrite

None of this is free, and the comparison is not like-for-like:

- **9,206 lines already exist, reviewed.** They carry 15,896 test lines, a
  finding catalog (`feature-spec.md` §12), 30 findings folded from `#1102`,
  and Mediums found in adversarial re-review. A rewrite discards that and
  re-incurs the review and hardening cycle on new, unreviewed code.
- **The critical path barely moves.** keep-core is unwritten either way
  (§5.1); variant A leaves it unchanged and variant B trims ~300-500 Go
  lines. Since keep-core is the schedule driver
  (`feature-spec.md` §16 item 3), a Solidity rewrite mostly removes work
  that was **not** on the critical path.
- **Audit saving is real but second-order.** Roughly 40% less production
  Solidity is a genuine audit reduction — the strongest argument for a
  rewrite, and the only objective on which it clearly wins.

**Verdict:** a rewrite wins on deployed-and-audited mass (-33% to -51%) and
loses on time-to-first-release, because it trades reviewed code for
unreviewed code without shortening the keep-core path. It is justified only
if audited mass is the dominant objective *and* re-review is genuinely cheap.
The one element worth harvesting regardless of that decision is **variant B's
unbounded re-anchor**, which closes §1.5's pinning cliff in the stacked
design too — see §7 item 6. Note the limit of that harvest: it fixes the
pinning cliff **only**. It does not touch §0.6, because dissolution's
reachability and the timeout-slashing path are independent of re-anchor's
gate, so keep-core dissolution stays mandatory in the stacked design. The
slashing vector disappears only in variant B, where the dissolution entry
point does not exist.

## 6. Decisions confirmed

Reordered 2026-08-21 by the variant B decision. Items the decision reversed
are kept with their reversal marked, per this set's convention.

1. **m1 is variant B with a minimal router** (§1) — create, custody and
   re-anchor only; **no dissolution**; built as an essentials-only rewrite.
   `m1-b-implementation.md` is the build scope.
2. **Create-only m1** — users create only. *Restated under B:* redemption and
   renewal do not "exist on-chain but unreachable" as in the stacked plan;
   their **Bridge code is absent** and m2 writes it. Their *vault* entry
   points are absent too (Option B, 2026-09-24) — m2 restores both via a new
   vault deployment and depositor migration ceremony, not a flag flip (§1.2,
   §2.2).
3. **The vault ships minimal, not behind flags** (Option B, decided
   2026-09-24, superseding the item below) — `ReservationVault` (commit
   `4d549e64`) has no `redeemReservation`, `retryRedeemReservation`,
   `extendCustody`, or pause flags of any kind. Re-pointing the vault still
   needs quiescence B cannot reach while in use (§0.7), so this is an
   accepted migration cost for m2, not a gap m1 engineers around. The
   superseded version of this item read: "the vault ships its full
   entry-point surface behind flags... flags gate initiation only, never
   settlement or accounting" — kept below under "Reversed by the B decision"
   for the history.
4. **A global active-position cap is a launch gate**, not an option
   (`m1-b-implementation.md` §4.1) — without it B's saturation ends in seized
   operator stake and stranded depositors.
5. **Deploy inert, then activate for design partners**,
   `maxReservationsPerWallet = 1` (§1.3).
6. **Term 12 months**, with `reservationDissolutionDelay` as the buffer and a
   non-zero renewal window forced by the validator (§1.4). *Under B the term
   has no on-chain consumer* (§0.2) — it is a commitment held in storage, so
   the `dissolutionEligibleAt` write must survive the rewrite
   (`m1-b-implementation.md` §4.4).
7. **One audit per milestone** — but note B makes the two milestones
   comparable in size, 1.13 : 1, so this is two similar engagements rather
   than one large and one trivial (`timeline-estimate.md` §7).
8. **Branch stays local** — `docs/reservations-spec` not pushed.

### Reversed by the B decision

- ~~**Stack ships intact, only `#1096` deferred.**~~ B is a rewrite, so the
  eight-PR stack is reference material (§1.1). `#1096` is no longer a special
  case; it is one of several unwritten m2 features.
- ~~**Wallet pinning solved by re-anchor during the term and dissolution
  after it.**~~ B removes dissolution, so **nothing** relieves pinning after
  the term except wallet termination. Re-anchor is unbounded to compensate
  (its `< dissolutionEligibleAt` gate is deleted), which is what makes the cut
  sound at all, but a position past eligibility now has no owner-side exit.
  This is the accepted residual risk (§1.5 bounds it).
- ~~**keep-core dissolution ships in m1, not deferrable.**~~ Its premise was
  that an unwired permissionless dissolution is a slashing vector; B removes
  the entry point entirely, so there is nothing to wire and no vector (§0.2).
  keep-core m1 needs **acceptance and re-anchor only** — B's one genuine
  saving, ~300-500 production Go.

- ~~**The vault ships its full entry-point surface behind flags.**~~
  Reversed 2026-09-24 by the Option B vault decision (item 3 above): the
  vault ships minimal instead, with no redemption/renewal entry points or
  pause flags of any kind (commit `4d549e64`). Re-pointing it still needs
  total quiescence (§0.7), so the migration cost this reversed item tried to
  avoid is now an accepted m2 cost (`m1-b-implementation.md` §3/§4.2).

## 7. Open questions for review

Pruned 2026-08-21: items 2, 5, 6 and 7 were settled by the variant B decision
and are recorded as resolved rather than deleted.

### Still open

1. Is a rails release with **no reachable in-kind exit** acceptable in front
   of design partners, given the promise rests on an m2 governance action?
   B sharpens this: there is no dissolution either, so a position has **no
   owner-side exit at any age** in m1 (§6, reversed item 2). **RESOLVED
   2026-09-07:** accept as-is, ship under the active-position-cap mitigation
   (`m1-b-implementation.md` §4.1, §5); disclose the no-exit-until-m2 risk
   explicitly as part of design-partner activation. No new code.
   **Note (2026-09-24):** the "pause-flag" half of this mitigation no longer
   applies — the Option B vault decision ships the vault without a
   redemption pause flag at all (§1.2, §2.2), so the accepted mitigation is
   the active-position cap alone; the risk disclosure gets, if anything,
   sharper (no exit until m2 ships a new vault and migration, not until a
   governance unpause).
2. Concrete cap values — `reservationMaxTotalAmount`, and now also
   `maxActiveReservations` (`m1-b-implementation.md` §4.1), which must sit
   below `liveWalletsCount x maxReservationsPerWallet` with margin.
   **Decision RESOLVED 2026-09-07 (implementation pending — see below):**
   add the on-chain relational check
   (`activeReservationsCount < maxActiveReservations <=
   liveWalletsCount x maxReservationsPerWallet`), **enforced at acceptance
   time** (alongside the existing `activeReservationsCount <
   maxActiveReservations` check) rather than only at `updateReservationCaps`
   set-time — `liveWalletsCount` shrinks as wallets terminate, so a
   set-time-only check goes stale silently with no re-check. Tbtc-v2
   implementation pending, separate repo/scope.
   **Flag:** `98_generate_reservation_mainnet_calldata.ts`'s
   placeholder values (`maxReservationsPerWallet=10`, 1000 BTC total)
   contradict the decided launch posture (`=1`, tiny total) — a live risk if
   run unedited before this is caught downstream, independent of this fix.
3. Should reservation-eligible **wallets be allowlisted** at activation?
   Depositors pick the designated wallet at reveal, so any Live wallet can be
   selected; an allowlist adds surface no current PR has. B raises the stakes:
   every accepted position permanently occupies a wallet slot until that
   wallet is terminated. **RESOLVED 2026-09-07:** add a governance-set
   allowlist, reusing the existing `isVaultTrusted`/`isSpvMaintainer` shape
   (mapping + `onlyGovernance` setter + event) for wallets, checked alongside
   the existing `Live`-state gate. Tbtc-v2 implementation pending, separate
   repo/scope.
4. **Does m2 restore re-anchor's `< dissolutionEligibleAt` gate?** New,
   created by the decision. B deletes it to make re-anchor unbounded; when
   dissolution returns, leaving it out means a position can be rotated
   indefinitely past its eligibility date (`m1-b-implementation.md` §6).
   **REVIEWED 2026-09-07:** left exactly as-is — already correctly tracked
   as an inherited decision m2 must make once its dissolution design exists;
   forcing an answer now would be speculative. Revisit at m2 design kickoff.
5. **Post-m1 commitment, created by the 2026-08-23 step-3 decision: a
   structural bound on re-anchor fee loss as a *fraction* of the claim.**
   Milestone 1 accepts lever 4 of `pr-review-followups.md` item 7, leaving
   cumulative re-anchor loss bounded only by the dust floor, which scales with
   claim size. That acceptance is pinned by a characterization test and rests
   entirely on `#1093` governance-gating every hop from a `Live` source wallet.
   What remains open is which structural lever closes the ratio: item 7 lever 2
   (a per-reservation bound at acceptance as a fraction of `mintedAmount`,
   costing per-reservation state or recomputation) or lever 3 (a dust floor
   proportional to `mintedAmount` rather than the flat `reservationTxMaxFee`,
   cheaper but reversing the explicit "a minimum-sized reservation must remain
   migratable" decision, which must be re-litigated first). A flat governance
   ceiling is **not** a candidate: item 7's own constant-backing result shows no
   bound on the fee caps can deliver a fractional guarantee.
6. **Should arriving fee revenue pay down `inKindFeeDebtSat` before counting as
   reserve?** New, from the same review (`pr-review-followups.md` item 8). The
   aggregate in-kind fee debt has no ceiling, no automatic reduction path, and
   only voluntary repayment; reverting on settlement is explicitly off the
   table, so the answer cannot be a `require`. Independent of `vault.md`'s open
   sweep-safety decision, which never touches the debt. **RESOLVED
   2026-09-07 (corrected twice — see `pr-review-followups.md` item 8 for the
   full trail):** `sweepFees` does apply reserve balance to the debt before
   releasing excess to treasury, but `_burnFromReserve` burns from the full
   balance with no floor at `feeReserveTarget`, and the following
   `require(balance > feeReserveTarget)` reverts the whole call once debt is
   large enough to breach it — undoing the repayment too. The mechanism is
   inert in exactly the high-debt regime this item raises. Final decision:
   cap debt repayment at the reserve target in `sweepFees` so it degrades to
   partial repayment instead of reverting. Tbtc-v2 implementation pending,
   separate repo/scope.
   **Superseded 2026-09-28 by M1 decision D-3:** the 2026-09-07 cap-debt-at-target design was overridden. Keep the no-floor behavior: `sweepFees` repays `inKindFeeDebtSat` from the vault's full current balance before comparing against `feeReserveTarget`, so the target bounds only the sweepable surplus, not debt repayment. No contract logic change; the decision is recorded in `requirements.md` §12 and the tbtc-v2 runbook, and the `sweepFees` contract comment records it on `fix/m1-cross-repo-review` (`e635e229`).
   Scope note: this caps *repayment*, not *financing* — `financeInKindFee`
   is unchanged and still burns the reserve to zero unconditionally (how
   debt arises); if fee income stops, debt never fully clears under the
   capped rule either. See `pr-review-followups.md` item 8's "Scope note"
   for the regression-test implications.

### Settled by the B decision

- ~~**Flag-gated vault design — approved and now mandatory,** not merely
  recommended (§0.7, §6 item 3). Under B the redeploy alternative is not a
  hazard to weigh but an unreachable path, since draining
  `reservationTotalAmount` to zero requires terminating every custodying
  wallet. Scope correction from the original wording: the flag covers
  `redeemReservation` and `retryRedeemReservation` only — never
  `financeInKindFee` or `repayInKindFeeDebt`, which sit on the settlement
  path (`m1-b-implementation.md` §3).~~ **Reversed 2026-09-24 by the Option
  B vault decision** (§1.2, §2.2, §6 item 3): the "unreachable path"
  reasoning above was correct, but the decision accepted that cost rather
  than building the flag. The vault ships minimal — the redeploy/migration
  this bullet called a hazard to avoid is now the plan.
- **Does deploying unreachable-but-audited redemption code count against the
  surface objective?** Moot. B does not deploy it at all; the Bridge-side code
  is absent, so there is no unreachable mass to justify.
- **Harvest variant B's unbounded re-anchor into the stacked design?**
  Superseded — B is the design now, so the unbounded re-anchor ships as part
  of it rather than as a patch to `#1091`. The analysis that it cannot preempt
  a pending dissolution (`state == Active`, `:781`) still holds and is why the
  cut is sound.
- **Deployed-and-audited mass, or time-to-first-release?** Answered: mass.
  The decision takes the 43% m1 reduction and accepts the consequence that
  total program duration rises, because the milestones become comparable in
  size (1.13 : 1) and vendor audit windows do not compress
  (`timeline-estimate.md` §7).

---

## Provenance

Derived 2026-08-21 from `feature-spec.md` (§3-§7, §13, §15, §16),
`epic-merge-plan.md` (§3), `feature-spec.md` (§11), `timeline-estimate.md`, and the keep-core
§13 proposal inventory. **Verified against source**, branch-tagged because
the expiry model differs across the stack:
`feat/utxo-reservation-settlement` (#1091) — gate map (:618, :735, :742,
:838, :1083), two-phase acceptance (:401), action timeout (:911), pooled-claim
semantics (:89-91, :807-809), `closeReservation` (:1183-1192), dissolution
proof payload (:891-892), vault gates (:584, :1064), re-anchor authorization
(:718-757), `ReservationProofs.sol` field population (:448-463) and anchor-index
write (:465); `feat/utxo-reservation-backing` (#1093) —
`dissolutionEligibleAt` field and snapshot semantics (:180-186), rewritten
gates (:766, :880), parameter set (:308-314, :1212-1218), mandatory renewal
window (:1232-1236), vault gates (:614, :1133);
`feat/utxo-reservation-guards` (#1094) — `notifyReservationStranded`
(:1363-1378), permissionless dissolution (:887-890) and its router wrapper
(`ReservationRouter.sol:302-304`, no modifier), the dissolution-timeout
slashing contract (:961-975), and the vault-migration guard (:1267-1274);
`ReservationVault.sol:57`, `:69-72`, `:183-223` plus `deploy/95_deploy_reservation_vault.ts`
for non-upgradeability and the governance activation sequence. A scope
decomposition for decision, not a commitment of dates.