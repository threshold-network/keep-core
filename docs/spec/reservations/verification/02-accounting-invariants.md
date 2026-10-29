# Accounting invariants

Evidence pins and vocabulary: [invariants index](02-invariants.md).

## 2. Accounting arithmetic

Equations apply after a successful transaction, not between internal EVM writes.
Let P be positions in Active or ActionPending.
Let A be current Pending Acceptance generations on Unknown positions.
Let H be Pending Reanchor generations whose target capacity is still held.
Under D31 (INV-114) a privileged re-anchor off a retiring source can skip the
target caps; it still holds target capacity, so it is part of H and every
equation below keeps its form.
Use `count(set)` for cardinality and `sum(set, field)` for satoshi totals.
Solidity checks arithmetic overflow; that alone does not prove conservation.

### INV-201 `activeReservationsCount`

- **Conservation equation.**
  `activeReservationsCount = count(P) + count(A)`.
  A re-anchor's extra target hold is not a second global position.
- **Increment sites (all `+= 1`).**
  - `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1008`
    (`reserveAcceptanceCapacity`, called from `requestReservationAcceptance`
    at :621) - gated by `count < maxActiveReservations` (:1004-1006).
  - `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:533` (late
    acceptance re-take after a timeout release).
  - `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:302`
    (`prepareReservationForSettlement` stranded reconstruction).
- **Decrement sites (all `-= 1`).**
  - `Reservation.sol:1049` (`releaseAcceptanceCapacity`), called from:
    - `Reservation.sol:700` (`notifyReservationAcceptanceTimedOut`),
    - `Reservation.sol:1102` (`strandReservation`),
    - `Reservation.sol:1150` (`closeReservation`, M2, no M1 caller),
    - `ReservationProofs.sol:867` (`unwindPendingAction` acceptance branch).
- **Transition check.** A request adds one global slot.
  Acceptance timeout, stranding or pending-acceptance unwind releases it.
  Re-anchor request, settle and timeout do not change global occupancy.
  Late reconstruction can re-take a released position.
  The mapped sites support this equation; the listed tests cover specific transitions, not every possible execution.
- **Overflow/underflow.** Capped at request time (`maxActiveReservations`,
  tbtc-v2:solidity/contracts/bridge/Reservation.sol:1004). Late re-take (tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:302, tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:533) and stranded reconstruction do **not**
  re-check the cap - deliberate, documented "caps are request-time throttles"
  (`ReservationProofs.sol:531-533`). Decrement is a plain `-= 1` with no
  `require(count > 0)` guard; Solidity checked arithmetic reverts on
  underflow. **No truncation** (uint32 throughout).
  At `tbtc-v2@806b1106f` `reserveAcceptanceCapacity` also enforces the
  hand-off reserve gate after the increment
  (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1061`,
  `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`; INV-113).
  It only adds a revert, so the increment and decrement sites and the
  equation are unchanged.
- **Test.** `Bridge.ReservationOccupancy.test.ts` "requestReservationAcceptance
  increments activeReservationsCount and emits ReservationOccupancyChanged"
  (:30), "notifyReservationAcceptanceTimedOut decrements ..." (:68), "
  prepareReservationForSettlement restores activeReservationsCount on late
  settlement of stranded reservation" (:238); `Reservation.test.ts` "should
  allow a new request after strandReservation decrements activeReservationsCount"
  (:749).
- **Status.** holds.

### INV-202 `reservationTotalAmount`

- **Conservation equation.**
  `reservationTotalAmount = sum(P, anchorAmount) + sum(A, amount)`.
  Pending acceptance reserves the full deposit amount until proof adjusts its fee.
  A pending re-anchor target is excluded from this global amount.
  Its amount is temporarily counted in the target's wallet total instead.
- **Increment sites.**
  - `Reservation.sol:1017` (`reserveAcceptanceCapacity`: `total += deposit.amount`).
  - `ReservationProofs.sol:524` (late acceptance re-take: `total += anchorAmount`).
  - `ReservationProofs.sol:297` (stranded reconstruction: `total += reservation.anchorAmount`).
- **Decrement sites.**
  - `Reservation.sol:1048` (`releaseAcceptanceCapacity`: `total -= amount`)
    [timeout :700, strand :1102, unwind-acceptance :867].
  - `ReservationProofs.sol:544` (on-time acceptance: `total -= feeDelta`
    where `feeDelta = action.amount - anchorAmount`).
  - `ReservationProofs.sol:809` (re-anchor settle: `total -= minerFee`).
- **Transition check.** Request books the deposit; on-time acceptance removes its fee delta.
  Timeout releases the deposit amount; late acceptance re-takes the actual anchor.
  Re-anchor settlement removes the miner fee globally.
  A re-anchor request does not add its target amount to `reservationTotalAmount`.
- **Overflow/underflow.** Capped at request time
  (`reservationMaxTotalAmount`, `Reservation.sol:1011-1016`). No truncation
  (uint64).
- **Test.** `Reservation.test.ts` "should record action, increment nonce,
  update counters, and emit ReservationAcceptanceRequested" (:804); "should
  revert when total reserved amount exceeds reservationMaxTotalAmount" (:558).
- **Status.** holds.

### INV-203 `walletReservationInfo[w].count`

- **Conservation equation.**
  `walletCount[w] = count(P with wallet=w) + count(A with target=w) + count(H with target=w)`.
  A pending acceptance holds a wallet slot before an Active position exists.
  A pending re-anchor holds both its source position slot and a target slot.
  Third-party holds are a subset of H, not an additional term.
- **Increment sites.**
  - `Reservation.sol:1026` (acceptance: `count += 1`, wallet = target).
  - `Reservation.sol:889` (re-anchor request: target `count += 1`).
  - `ReservationProofs.sol:525` (late acceptance re-take: target `count += 1`).
  - `ReservationProofs.sol:298` (stranded reconstruction: source `count += 1`).
  - `ReservationProofs.sol:704` (late re-anchor re-take: target `count += 1`).
- **Decrement sites.**
  - `Reservation.sol:1046` (`releaseAcceptanceCapacity`) [timeout/strand/unwind].
  - `Reservation.sol:1060` (`releaseReanchorTargetCapacity`, target)
    [timeout :970, unwind-reanchor `ReservationProofs.sol:852`].
  - `ReservationProofs.sol:804` (re-anchor settle: source `count -= 1`).
- **Transition check.** Acceptance request reserves the target slot.
  Re-anchor request adds a target slot while retaining the source.
  Settlement releases the source; timeout or unwind releases the target.
  Late settlement reconstructs released tracking as required.
  The same target count rules apply to third-party and exempt requests.
  At `tbtc-v2@806b1106f` a privileged request off a MovingFunds/Closing
  source adds the target count without the cap check (INV-114), so a target
  can exceed `maxReservationsPerWallet`. The equation is unchanged because
  the count is still added on every path
  (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:934-935`).
- **Overflow/underflow.** Capped at request time
  (`maxReservationsPerWallet`, `Reservation.sol:1022-1025` and :885-889,
  where the re-anchor check at :885-889 applied to every caller at `9d2086965`).
  At `tbtc-v2@806b1106f` the re-anchor count check
  (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:928-933`) is
  skipped for a privileged request off a non-Live source; the acceptance
  check is unchanged. No truncation (uint32).
- **Test.** `Reservation.test.ts` "should revert when wallet reservations
  count exceeds maxReservationsPerWallet" (:595);
  `Bridge.ReservationStrandingLibrary.test.ts` "rejects when target wallet
  reservation count cap is exceeded" (:1684);
  `Bridge.ReservationThirdPartyReanchorHolds.test.ts` "a third-party hold
  still uses the target's ordinary per-wallet slot" (:386). D31 at
  `tbtc-v2@806b1106f`: a privileged hand-off adds the target count above
  the cap (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:427`,
  "owner: a MovingFunds re-anchor to a full target succeeds and the target count becomes cap+1"),
  and a third party into a full target still reverts
  (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`).
- **Status.** holds.

### INV-204 `walletReservationInfo[w].amount`

- **Conservation equation.**
  `walletAmount[w] = sum(P with wallet=w, anchorAmount) + sum(A with target=w, amount) + sum(H with target=w, amount)`.
  The pending re-anchor target temporarily books the pre-hop amount.
  `sum(walletAmount) = reservationTotalAmount + sum(H, amount)`.
  This intentional temporary overlap must not be mistaken for extra BTC backing.
- **Increment sites.**
  - `Reservation.sol:1036` (acceptance: `amount += deposit.amount`).
  - `Reservation.sol:902` (re-anchor request: target `amount += anchorAmount`).
  - `ReservationProofs.sol:527-528` (late acceptance re-take).
  - `ReservationProofs.sol:300-301` (stranded reconstruction: source).
  - `ReservationProofs.sol:706-707` (late re-anchor re-take: target).
- **Decrement sites.**
  - `Reservation.sol:1047` (`releaseAcceptanceCapacity`).
  - `Reservation.sol:1061` (`releaseReanchorTargetCapacity`).
  - `ReservationProofs.sol:546-547` (on-time acceptance: `-= feeDelta`).
  - `ReservationProofs.sol:806-807` (re-anchor: source `-= oldAnchorAmount`).
  - `ReservationProofs.sol:808` (re-anchor: target `-= minerFee`).
- **Transition check.** The request reserves the pre-hop amount at the target.
  Settlement removes the source amount and target fee delta.
  Timeout or unwind releases the held target amount.
  The mutation map supports the conservation equation; no universal runtime proof is claimed.
- **Overflow/underflow.** Capped at request time
  (`maxReservationsAmountPerWallet`, `Reservation.sol:1031-1036` and
  :897-903 at `9d2086965`); a `0` cap disables the check. At
  `tbtc-v2@806b1106f` the re-anchor amount check
  (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:942-948`) is
  skipped for a privileged request off a non-Live source, and the amount is
  still added (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:949-950`).
  No truncation (uint64).
- **Test.** `Reservation.test.ts` "should revert when wallet reservations
  amount exceeds maxReservationsAmountPerWallet" (:631);
  `Bridge.ReservationStrandingLibrary.test.ts` "rejects when target wallet
  reserved amount cap is exceeded" (:1703). D31 at `tbtc-v2@806b1106f`:
  privileged above the amount cap succeeds
  (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:494`) and a
  third party above it reverts
  (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:502`).
- **Status.** holds.

### INV-205 `pendingThirdPartyReanchorHolds[w]`

- **Conservation equation.**
  `holds ==` #(pending re-anchor generations targeting `w` whose request was
  made by a third party, i.e. `thirdPartyHold == true`, not yet released).
  Privileged generations never touch it.
- **Increment sites.**
  - `Reservation.sol:873` (`requestReservationReanchor`, `holds += 1`), only
    when `!isPrivileged`, gated by `holds < maxThirdPartyReanchorHoldsPerWallet`
    (:869-871). `0` cap blocks third parties entirely.
- **Decrement sites (exactly once per generation).**
  - `Reservation.sol:1082` (`releaseThirdPartyReanchorHold`, `holds -= 1`,
    guarded by `require(holds > 0)` at :1081), called from:
    - `Reservation.sol:975` (`notifyReservationActionTimeout`),
    - `ReservationProofs.sol:742` (on-time re-anchor settle, `!late` only),
    - `ReservationProofs.sol:857` (`unwindPendingAction` re-anchor branch).
  - The `thirdPartyHold` flag (`Reservation.sol:923`) ensures a privileged
    generation (flag `false`) never calls the release, so a repeated release
    cannot underflow.
- **Transition check.** Each third-party request adds one hold.
  Timeout, on-time settlement or supersede-unwind releases that generation's hold.
  A non-zero guard prevents counter wrap; a stale duplicate release would revert.
- **Overflow/underflow.** Capped at request time
  (`maxThirdPartyReanchorHoldsPerWallet`, tbtc-v2:solidity/contracts/bridge/Reservation.sol:869-871). No truncation (uint32).
- **Test.** `Bridge.ReservationThirdPartyReanchorHolds.test.ts` "blocks a
  third party once the target's pending holds reach the cap" (:133), "blocks
  third parties entirely when the cap is zero" (:250), "drops on action
  timeout" (:271), "drops on supersede-unwind of the pending generation"
  (:288), "drops on on-time re-anchor settlement, so a third party can request
  again" (:303).
- **Status.** holds.

### INV-206 `pendingReservedDeposits`

- **Conservation equation.**
  `pendingReservedDeposits ==` #(deposit keys `k` with
  `pendingReservedDeposit[k].walletPubKeyHash != 0`) - revealed reserved
  deposits that are not yet accepted, not marked stale, and (from
  `d20cf74ba`) not released to the pool. At the pin (`9d2086965`) the
  equation has three exits; at `d20cf74ba` it has four. The release exit is
  D26 (`releaseReservedDepositToPool`), see INV-111.
- **Increment sites.**
  - `tbtc-v2:solidity/contracts/bridge/Deposit.sol:395` (`revealDeposit`,
    `+= 1` when the deposit is reserved). Unchanged at `d20cf74ba`.
- **Decrement sites (exactly once per deposit key).**
  - `ReservationProofs.sol:434` (`consumeAcceptedDeposit`, guarded by
    `reservedDeposit.walletPubKeyHash != bytes20(0)` at :427-431 - idempotent).
  - `Reservation.sol:1264` (`notifyStaleReservedDeposit`, guarded by
    `walletPubKeyHash != 0` at tbtc-v2:solidity/contracts/bridge/Reservation.sol:1235).
  - `Reservation.sol:1312` (`forceStaleReservedDeposit`, guarded by
    `walletPubKeyHash != 0` at tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1291).
  - **Release to pool (D26, not at the pin):**
    `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1445`
    (`releaseReservedDepositToPool`, `pendingReservedDeposits -= 1`),
    guarded by `walletPubKeyHash != 0` at :1392-1395 and `isReserved` at
    :1388-1391. At `d20cf74ba` the stale/force-stale decrements sit at
    `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1281` and `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1329` (shifted by the new event).
- **Transition check.** Reveal increments the pending counter.
  Consumption, stale cleanup, force-stale cleanup and D26 release clear the designated-wallet marker.
  Each decrement is guarded by that marker or skips when it is already clear.
  D26 changes this pending counter only; it does not consume a global active slot.
- **Overflow/underflow.** No cap (a count of pending reserved deposits). No
  truncation (uint64).
- **Test.** `ReservationProofs.test.ts` "should not double-decrement
  pendingReservedDeposits when the pending marker is already cleared" (:1841);
  `Bridge.ReservationStrandingLibrary.test.ts` "decrements the
  pendingReservedDeposits counter exactly once" (:913). Release exit:
  `tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts`
  "lets governance re-point the reservation vault once the last pending
  reserved deposit is released" (:434) - passes only if the counter returns to
  zero.
- **Status.** holds (pin exits). The release exit is `holds` at `d20cf74ba`
  and absent at the pin.

### INV-207 `inKindFeeDebtSat` (vault)

- **Conservation equation (all terms in sats).**
  `feeDebt = sum(financed miner fees) - sum(immediate reserve coverage during financeInKindFee) - sum(external debt repayments) - sum(sweepFees debt repayments)`.
  Immediate fee coverage and later debt repayment are disjoint terms.
  Do not count a debt burn twice as both reserve coverage and repayment.
- **Increment sites.**
  - `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:229`
    (`financeInKindFee`, `+= shortfallSat` when the reserve cannot cover the
    full fee; :223-229).
- **Decrement sites.**
  - `ReservationVault.sol:262` (`repayInKindFeeDebt`, `-= repaySat`, repay
    capped at the outstanding debt at :246).
  - `ReservationVault.sol:309` (`sweepFees`, debt repaid before sweeping the
    excess over `feeReserveTarget`, :305-309).
- **Transition check.** Only a fee shortfall increases debt.
  External repayment and debt-first fee sweeping decrease it.
  Repayment is capped at outstanding debt; fee-reserve balance and debt use different units.
- **Overflow/underflow.** `repayInKindFeeDebt` `require(inKindFeeDebtSat > 0)`
  (tbtc-v2:solidity/contracts/vault/ReservationVault.sol:242); `sweepFees` only burns `min(debt, coverable)`. No truncation
  (uint64).
- **Test.** `ReservationVault.test.ts` "should record the entire fee as debt"
  (:379), "should cap an over-repayment at the outstanding debt" (:519),
  "should repay the debt before sweeping the excess" (:703).
- **Status.** holds.

### INV-208 `cumulativeReanchorFee` (per position)

- **Conservation equation.**
  `cumulativeReanchorFee ==` sum(miner fees of every settled re-anchor hop for
  this position). Monotonically non-decreasing; a lifetime total.
- **Increment sites.**
  - `ReservationProofs.sol:810` (`settleReanchorAccounting`,
    `+= minerFee`).
- **Decrement sites.** None (by design - it is a running total).
- **Balance check.** One `+=` per settled re-anchor hop. **Balances.**
- **Overflow/underflow.** Bounded above by the initial anchor minus the
  minimum (see INV-303). No truncation (uint64).
- **Test.** `none` pins the running total directly (the grind bound is pinned
  by `Bridge.ReservationSettlement.test.ts` "bounds the grind at a request-time
  amount floor" :500, and the per-hop `minerFee` by
  `ReservationProofs.test.ts` :1571).
- **Status.** holds-untested (the field's running total is written but no test
  asserts its value).

### INV-209 `mintedAmount == anchorAmount`

- **Statement.** The protocol fields `mintedAmount` and `anchorAmount`
  hold the same gross satoshi value after acceptance and re-anchor settlement.
  They are not the owner's current ERC-20 token balance.
  The depositor receives gross TBTC minus the initiation fee, and can transfer those tokens.
- **Enforcing code.** `ReservationProofs.sol:560, 564` writes both fields at acceptance.
  `ReservationProofs.sol:817-818` writes both after re-anchor.
  `ReservationVault.sol:180-184` computes the net ERC-20 transfer.
- **Why.** Separate the protocol's backing record from fungible token ownership and fee revenue.
- **Test.** `ReservationProofs.test.ts` "should successfully execute
  end-to-end acceptance settlement" asserts `reservation.mintedAmount ==
  anchorAmount` (:555, check at :578).
- **Status.** holds.

### INV-210 Counter width / truncation safety

- **Statement.** Counter arithmetic uses checked uint32/uint64 operations.
  Explicit `uint32(block.timestamp)` casts can truncate after the uint32 horizon.
  Checked arithmetic does not protect an explicit narrowing cast.
  The current deployment horizon is a stated assumption, not an unlimited-time safety guarantee.
- **Enforcing code.** `activeReservationsCount`,
  `walletReservationInfo[w].count`, `maxActiveReservations`,
  `pendingThirdPartyReanchorHolds`, `maxThirdPartyReanchorHoldsPerWallet` are
  `uint32` (`BridgeState.sol:435, 471, 496, 504`); `reservationTotalAmount`,
  `pendingReservedDeposits`, `inKindFeeDebtSat`, `cumulativeReanchorFee`,
  `mintedAmount`, `anchorAmount` are `uint64`
  (`BridgeState.sol:393, 431`; `ReservationVault.sol:85`;
  `Reservation.sol:156, 166, 238`). Timestamps `timeoutAt`, `expiresAt`,
  `dissolutionEligibleAt`, `reanchorCooldownUntil` are `uint32` with the 2106
  ceiling documented (`Reservation.sol:170-176, 208-220, 241`).
- **Test.** `none` identified for the timestamp narrowing horizon.
  A boundary regression is possible in a local EVM; it is not excluded by the argument that real time is currently smaller.
- **Status.** holds-untested within the documented horizon.

Let `C` be positions in Active or ActionPending (the proven-anchor set).

### INV-211 `walletAnchoredInfo[w]` - proven anchors, distinct from reserved capacity

- **Conservation equation.**
  `walletAnchoredCount[w] = count(C with custody=w)` and
  `walletAnchoredAmount[w] = sum(C with custody=w, anchorAmount)`.
  Only a settled acceptance or a settled re-anchor adds a proven anchor; only a
  source-side settlement, a strand or a close removes one. Reserved capacity
  for a pending acceptance or re-anchor target is **not** in this ledger, which
  is why `walletAnchoredInfo` and `walletReservationInfo` (INV-203, INV-204)
  diverge whenever a pending generation holds capacity.
- **Why.** The wallet lifecycle must tell BTC the wallet actually custodies from
  capacity some request merely reserved; routing on the reserved ledger let an
  empty wallet be pushed into the slash-prone moving-funds window, and would
  let a wallet holding a proven anchor close underneath it.
- **Increment sites** (`Reservation.addAnchoredCustody`).
  - `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/ReservationProofs.sol:577` (acceptance settlement, target wallet).
  - `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/ReservationProofs.sol:308` (late acceptance re-take).
  - `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/ReservationProofs.sol:828` (re-anchor settlement, target wallet).
- **Decrement sites** (`Reservation.removeAnchoredCustody`).
  - `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/ReservationProofs.sol:823` (re-anchor settlement, source wallet).
  - `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/Reservation.sol:1208` (`strandReservation`).
  - `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/Reservation.sol:1261` (`closeReservation`, unreachable in M1).
- **Consumers.**
  `moveFunds` routes on it (`tbtc-v2@d1bf5bc281:solidity/contracts/bridge/Wallets.sol:629-631`),
  `notifyWalletCloseable` adds its amount to the closing threshold
  (`:350-356`), and the wallet-lifecycle invariants read it (INV-102, INV-104).
- **Storage.** `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/BridgeState.sol:519-524`,
  a `WalletReservationInfo` mapping appended into the storage gap; `uint32`
  count and `uint64` amount, so INV-210's width argument applies.
- **Transition check.** An ordinary re-anchor settlement decrements the source
  and increments the target in the same call; a strand releases the anchor
  without a replacement; a late acceptance re-take re-adds the anchor its
  position still custodies. No M1 path decrements below zero because every
  decrement is paired with a custody that a prior increment created.
- **Test.** `tbtc-v2@d1bf5bc281:solidity/test/bridge/Reservation.test.ts:2105`
  (a proven anchor keeps the wallet in MovingFunds) and
  `tbtc-v2@d1bf5bc281:solidity/test/bridge/Reservation.test.ts:2155`
  (pending-only capacity does not). `none` identified for a dedicated
  conservation test of the six mutation sites.
- **Status.** holds by inspected mutation sites and routing tests;
  `holds-untested` for an exhaustive conservation test.

### INV-212 Whole-system fungible supply exceeds backing by exactly the recorded fee debt

- **Conservation equation (all terms in sats).**
  `supplyInSats - backing == feeDebt` at a transaction boundary, where
  `supplyInSats` is the total TBTC supply divided by `SATOSHI_MULTIPLIER`,
  `backing` is the Bitcoin the Bridge holds as backing for that supply, and
  `feeDebt` is the vault's `inKindFeeDebtSat` (INV-207).
- **Why the reservation subsystem satisfies it.** A credit mints TBTC gross
  against the same gross of new backing, so the two move together
  (`tbtc-v2@d1bf5bc281:solidity/contracts/vault/ReservationVault.sol:173-174`).
  A financed miner fee lowers backing by the whole fee and supply by whatever
  the reserve could burn (`_burnFromReserve`, `:354-371`), so the uncovered part
  lands in the debt (`financeInKindFee`, `:211-231`); the function's doc block
  states that while the debt is non-zero "the system is over-supplied by
  exactly that amount" (`:207-210`). A repayment (`repayInKindFeeDebt`,
  `:237-262`) and the debt-first half of `sweepFees` (`:297-319`) lower supply
  and debt by the same amount. A sweep of the excess above
  `feeReserveTarget` moves tokens and changes neither term.
  INV-705 states this same relation per fee increment; this entry states the
  at-rest whole-system form.
- **Pooling caveat.** The vault pools the fee reserve and the debt across every
  depositor and position (INV-207), so the equation is aggregate: it does not
  say which position's satoshi are uncovered, and it must not be used to derive
  a per-position claim. INV-209 and INV-302 are per-position records whose
  backing is that position's own anchor, not the pool, and neither is weakened
  by this entry. The equation also holds only while every non-reservation mint,
  unmint and backing change is balanced: Bitcoin in flight for an ordinary
  redemption, an unminted deposit, and any loss outside the reservation path
  break it until they settle, which is the same limit INV-705 records.
- **Enforcing code.** `tbtc-v2@d1bf5bc281:solidity/contracts/vault/ReservationVault.sol:211-231`
  (burn what the reserve covers, record the shortfall as debt),
  `tbtc-v2@d1bf5bc281:solidity/contracts/vault/ReservationVault.sol:354-371`
  (unmint TBTC and decrease the Bank balance in one call),
  `tbtc-v2@d1bf5bc281:solidity/contracts/vault/ReservationVault.sol:237-262`
  (repay: pull, burn, decrement the debt),
  `tbtc-v2@d1bf5bc281:solidity/contracts/vault/ReservationVault.sol:297-319`
  (sweep: repay the debt first, then move the excess).
- **Test.** `ReservationVault.test.ts` shortfall, repayment and sweep cases
  cited in INV-207. `none` identified for an end-to-end check of the
  bitcoind-backed equality.
- **Status.** `holds-untested` for the at-rest equality: the sites above keep it
  by inspection and INV-705 covers the incremental form, but no test asserts the
  whole-system form.

---
