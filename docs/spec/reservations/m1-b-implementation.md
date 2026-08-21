# m1 Variant B — Implementation Scope

Status: DRAFT — decided 2026-08-21.

Milestone 1 is **variant B with a minimal router**: an essentials-only rewrite
that ships creation, custody and re-anchoring, and omits dissolution.
`roadmap.md` §1 is the authoritative scope statement; `m1-variant-comparison.md`
§5-§6 holds the argument that was weighed, including the case against B, which
is retained rather than deleted.

This document is the buildable form of that decision: what the router contains,
what the vault must contain, and the gates that must close before activation.

---

## 1. Two layers, opposite rules

The decision to minimise the router is safe **because** router code is
replaceable; the same reasoning forbids minimising the vault (`roadmap.md`
§0.7).

| | `ReservationRouter` | `ReservationVault` |
|---|---|---|
| Nature | Bridge code reached by `delegatecall` | Separate contract, plain `Ownable` |
| Replacing it costs | A Bridge implementation upgrade, which m2 needs anyway | Deploying v2 and re-pointing `Bridge.reservationVault` |
| Gate on replacement | Proxy-admin ceremony | `reservationTotalAmount == 0 && pendingReservedDeposits == 0` (`Reservation.sol:1267-1274`) |
| Reachable in B? | Yes | **No** — in B positions close only by stranding, so quiescence means every custodying wallet has been terminated |
| m1 posture | **Minimise** | **Ship complete, behaviour disabled** |

The asymmetry is the single most important implementation consequence of
choosing B. A router entry point omitted in m1 costs nothing to add later. A
vault entry point omitted in m1 **cannot be added** while the product is in
use.

## 2. Minimal router surface

The stacked router has 24 entry points. B removes 5 and adds 1, for **20**.

### 2.1 Retained — state-changing

| Entry point | Stacked line | Why B needs it |
|---|---|---|
| `requestReservationAcceptance` | `:242` | The product's entry gate |
| `requestReservationReanchor` | `:286` | B's **only** unpin path; load-bearing, not optional |
| `submitReservationProof` | `:322` | Proof dispatcher, simplified: acceptance and re-anchor only |
| `notifyReservationActionTimeout` | `:351` | Required cleanup; also the slashing path |
| `notifyStaleReservedDeposit` | `:450` | Releases un-accepted revealed deposits |
| `notifyReservationStranded` | `:461` | B's only position-closing path |
| `updateReservationParameters` | `:421` | Governance; also the vault re-point |
| `updateReservationCaps` | `:476` | Governance; the only safety valve at launch |

### 2.2 Retained — views

`reservationCaps` (`:487`), `walletReservationsAmount` (`:503`),
`walletReservationsCount` (`:514`), `walletReservations` (`:524`),
`reservationByAnchorUtxo` (`:538`, subject to §4.3),
`reservedDepositWallet` (`:555`), `pendingReservedDeposits` (`:565`),
`reservations` (`:573`), `reservationActions` (`:585`),
`reservationParameters` (`:609`), `reservationRouter` (`:640`).

`walletReservationsCount` and `pendingReservedDeposits` are not optional
conveniences: the first is the free-slot monitor's data source (§5), the second
is read by the vault-swap gate.

### 2.3 Removed

| Entry point | Stacked line | Reason |
|---|---|---|
| `requestReservedRedemption` | `:262` | Redemption deferred to m2 |
| `notifyReservedRedemptionVeto` | `:366` | Veto is vacuous with no redemptions |
| `extendReservation` | `:382` | Renewal deferred to m2 |
| `requestReservationDissolution` | `:302` | B's defining cut |
| `walletPendingDissolution` | `:600` | Dissolution view |

### 2.4 Added

`activeReservationsCount` — the global position counter §4.1 requires.
`BridgeState.Storage` today has `liveWalletsCount` (`:253`),
`reservationTotalAmount` (`:378`) and per-wallet counts, but **no global
position count**, so both the storage field and its view are new.

### 2.5 Whether the router is needed at all

Keep it. The question is decidable by compiler and was analysed in
`m1-variant-comparison.md` §5.2: the surface costs `Bridge` roughly 2,244 B
inline by subtraction (not the router's standalone 4,245 B, which double-counts
`Governable`/`Initializable`), against 2,173 B of measured margin — so the
`#1090`-era gap was 71 B, and every PR since added surface. More decisively,
m2 must add ~4,641 production lines and ten-plus entry points, which is
**larger than everything B removed**, so the router is needed back regardless.
Deleting it in m1 buys ~736 lines and costs a re-architecture.

## 3. Vault surface: ship it all, initiation disabled

Every path m2 intends to reach must have its vault entry point present in m1
behind a pause flag. The shipped vault already does this for renewal and not
for redemption.

**The rule that bounds this: a pause flag may gate initiation, never
settlement or accounting.** A confirmed Bitcoin spend must always be able to
settle, so any function on the settlement path must stay unconditionally
callable. The vault states this itself for the in-kind fee: if the reserve
cannot cover the amount, "the shortfall is recorded as `inKindFeeDebtSat` and
the call still succeeds: a confirmed Bitcoin spend must never fail to settle
because of the reserve level" (`ReservationVault.sol:524-528`). A flag over
such a function would reintroduce exactly the revert the design removed.

| Vault entry point | Line | m1 state | Rationale |
|---|---|---|---|
| `receiveBalanceIncrease` | `:234` | **Active** | The mint callback; `onlyBank` |
| `financeInKindFee` | `:529` | **Active — must not be gated** | On the settlement path. Called by the re-anchor proof at `ReservationProofs.sol:874-875`, immediately before `action.state = Settled` (`:878`) — and re-anchor is B's only unpin, so this is reachable in m1 and load-bearing. Its other caller, `submitReservationDissolutionProof` (`:921`, financing at `:995-996`), is the one B cuts |
| `repayInKindFeeDebt` | `:568` | **Active** | Permissionless burn-down of an over-supply re-anchor can create. Gating it would remove a safety valve while leaving the debt |
| `redeemReservation` | `:293` | Present, **new** `redemptionsPaused` flag | Initiation only |
| `retryRedeemReservation` | `:469` | Present, same flag | Initiation only |
| `extendCustody` | `:367` | Present, gated by `renewalsPaused`, constructor-`true` (`:222`) | Initiation only; already correct as shipped |

Add `redemptionsPaused` by copying renewal's pattern within the same file:
default `true` in the constructor, restrictive setter on
`onlyGuardianOrOwner`, restorative `unpauseRedemptions` on `onlyOwner`
(mirroring `:409`/`:415`). Scope it to the two initiation functions only.
Then m2's redemption enablement is one owner transaction instead of an
unreachable vault swap.

Note that this makes m1's fee machinery **live**, not dormant: re-anchor
charges an in-kind miner fee, so the fee reserve, `inKindFeeDebtSat` and
`updateFeeReserveTarget` are all in use from the first re-anchor. Monitoring
the debt belongs in §5.

## 4. Launch gates

These are the residual risks `m1-variant-comparison.md` §5.4 identified.
Under the B decision they are **gates, not tradeoffs**: without them B's
failure mode is honest operators slashed and depositors stranded, reached by
arithmetic rather than by an attacker (§5.3 of that document).

### 4.1 Global active-position cap, enforced at acceptance

The one gate that converts a silent cliff into a revert. B never frees a
wallet slot, so occupancy is monotonic toward
`liveWalletsCount x maxReservationsPerWallet`; at saturation re-anchor has no
target (`Reservation.sol:820-827`), an anchored wallet cannot retire because
`beginWalletClosing` requires a zero reservation count
(`Wallets.sol:675-677`, repeated `:707-709`), its MovingFunds clock expires,
and `notifyWalletMovingFundsTimeout` seizes operator stake and terminates the
wallet (`:493-523`).

Add `activeReservationsCount` and a governance `maxActiveReservations` set
below the slot floor, required at acceptance. Roughly 20 lines plus a
parameter. Also relate it to the amount cap: nothing on-chain today prevents
raising `reservationMaxTotalAmount` past slot capacity, so either add the
relational check or emit both sides and make it a runbook gate.

### 4.2 Vault ships complete behind flags

§3. Failing this gate does not delay m2's redemption, it forecloses it.

### 4.3 `reservationsByAnchorUtxo` reconciliation

`#1091` writes the mapping (`ReservationProofs.sol:465`), `#1094` writes it
again for stranding, and `#1102` removed it from the merged base in favour of
`spentMainUTXOs`. Two write sites and one removal must be reconciled in the
rewrite, because stranding depends on it and stranding is B's only close path.

### 4.4 Keep writing `dissolutionEligibleAt`

Acceptance sets
`dissolutionEligibleAt = expiresAt + reservationDissolutionDelay`
(`ReservationProofs.sol:537-539`). B deletes its only reader — re-anchor's
`< dissolutionEligibleAt` gate — so an essentials-only rewrite would naturally
drop the write as dead code. Do not. Without it, m2's dissolution has no
eligibility date for any m1-era position, and the snapshot semantics that keep
governance changes non-retroactive (`:180-184`) cannot be reconstructed.

The general rule: **storage-complete means written, not merely declared.**

**The deeper reason this matters: in m1 B the custody term has no on-chain
consumer at all.** Every reader of `expiresAt` and `dissolutionEligibleAt` is
cut or deleted — redemption's `<= expiresAt + gracePeriod`, renewal,
dissolution's `>= dissolutionEligibleAt`, and re-anchor's
`< dissolutionEligibleAt` (removed to make re-anchor unbounded). So the term
is not enforced by anything in m1; it is a **commitment held in storage for m2
to honour**. The storage *is* the promise, which makes dropping the write a
silent repudiation of it rather than a code-size optimisation.

### 4.5 Storage layout complete

§11's rule is that no live `ReservationAction` may ever span a layout change,
so every field m2 will read must exist at m1 — redemption settlements, veto
delay, retry credit, renewal window, `dissolutionEligibleAt`,
`walletPendingDissolution`. This is what makes m2 an upgrade rather than a
migration.

## 5. Operational duties

B's duty list is longer than A+'s because nothing closes a position on its own.

| Duty | Why B specifically |
|---|---|
| Re-anchor executor on `WalletMovingFunds`, with alerting | The only unpin; failure ends in slashing, not delay |
| Free-slot monitor (`walletReservationsCount < cap` across Live wallets) | Leading indicator of the §4.1 cliff |
| Occupancy monitor (`activeReservationsCount` vs `liveWalletsCount x cap`) | Alert well before saturation |
| Action-timeout watch | Pending acceptances and re-anchors approaching `reservationActionTimeout`, since expiry slashes |
| Stranding watcher on `Terminated` wallets | Releases capacity; B's only close path |
| Stale reserved-deposit cleanup | `notifyStaleReservedDeposit` |
| In-kind fee reserve and `inKindFeeDebtSat` watch | Re-anchor charges an in-kind miner fee (§3), so the reserve depletes and can enter debt in m1. Non-zero debt means the system is over-supplied by exactly that amount, publicly visible and repayable by anyone |
| Cap-dial runbook | Trigger, executor and accepted blast radius agreed **before** launch |
| Position-age report | Nothing closes, so age is the only proxy for accumulating permanent liability |

Note what B does **not** need: a dissolution executor. That saving is the
decision's operational upside, and it is real — but it is a saving of
~300-500 production Go lines against the duties above.

## 6. What m2 must then build

~4,641 production Solidity lines: whole redemption, renewal, veto integration
and their storage (~3,035), `#1096`'s partial redemption (~696), and
dissolution restored (~910). keep-core gains **two** action types, Redemption
and Dissolution, where A+ would have needed only Redemption.

Two inherited decisions m2 must make that A+ would not have created:

1. **Whether to restore re-anchor's eligibility gate.** B deletes
   `< dissolutionEligibleAt` (`Reservation.sol:785-788`) to make re-anchor
   unbounded, which is what makes dropping dissolution sound. Restoring
   dissolution does not automatically restore the gate, and leaving it out
   means a position can be rotated indefinitely past its eligibility date.
2. **Whether m1-era positions get the m2 semantics.** They will carry
   `dissolutionEligibleAt` values snapshotted under m1 parameters (§4.4), and
   `:180-184` makes those non-retroactive by design.

The milestone ratio is worth stating plainly: 5,261 : 4,641, or **1.13 : 1**.
Against A+'s 1.65 : 1 and the stacked plan's 13.2 : 1, B's split produces two
comparable projects rather than a large first release and a small follow-up
(`timeline-estimate.md` §7).

---

## Provenance

Derived 2026-08-21 from the variant B decision, `roadmap.md` §0.7/§1 and
`m1-variant-comparison.md` §5.2-§6. Source-verified on
`feat/utxo-reservation-guards` unless noted: router surface
(`ReservationRouter.sol:242-643`), vault surface
(`ReservationVault.sol:222-568`), vault re-point gate
(`Reservation.sol:1267-1274`), re-anchor target cap (`:820-827`), re-anchor
eligibility gate (`:785-788`), acceptance's `dissolutionEligibleAt` write
(`ReservationProofs.sol:537-539`), wallet closing preconditions
(`Wallets.sol:675-677`, `:707-709`), MovingFunds timeout slashing (`:493-523`),
storage fields (`BridgeState.sol:253`, `:378`), once-only router setter
(`:1018-1021`). Bytecode figures are `#1090`-era, quoted from `feature-spec.md`
§2 and not re-measured.

A scope decision, not a commitment of dates.
