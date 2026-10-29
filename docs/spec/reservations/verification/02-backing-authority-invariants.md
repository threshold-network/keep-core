# Backing and authorization invariants

Evidence pins and vocabulary: [invariants index](02-invariants.md).

## 3. Bitcoin backing

### INV-301 Anchor outpoint uniqueness

- **Statement.** Two distinct live positions never reference the same anchor
  outpoint. The reverse index `reservationsByAnchorUtxo` maps an anchor
  outpoint to exactly one position, and a consumed anchor is removed before a
  new one is written.
- **Enforcing code.** The index add/remove pair is atomic per transition:
  - add on acceptance: `ReservationProofs.sol:572-574`;
  - add-on-reanchor only after remove: `consumeAnchor` deletes the old entry
    (`ReservationProofs.sol:918-923`) then `settleReanchorAccounting` writes
    the new one (:812-814);
  - remove on strand: `Reservation.sol:1103-1111`; on close:
    `Reservation.sol:1152-1164` (M2).
  - double-reconstruction is blocked by the `spentMainUTXOs` check
    ("Reservation anchor already spent", `ReservationProofs.sol:292-294`).
- **Why.** One anchor outpoint backing two positions would double-count the
  custodied sats.
- **Test.** `ReservationProofs.test.ts` "should settle re-anchor SPV proof,
  update reverse anchor index, and strand cleanly" (:1571).
- **Status.** holds.

### INV-302 The recorded reservation amount follows the current anchor

- **Statement.** `mintedAmount` is the protocol's gross satoshi record,
  not the owner's current fungible token balance.
  It equals the current `anchorAmount` after each successful settlement.
  Re-anchor miner fees shrink that anchor and are financed by reserve burn or fee debt.
- **Enforcing code.** `settleReanchorAccounting` recomputes the claim from the
  new anchor (`ReservationProofs.sol:817-818`); `financeInKindFee(minerFee)`
  is called for the fee (`ReservationProofs.sol:772-773` ->
  `ReservationVault.sol:215-236`).
- **Why.** The recorded in-kind amount must not exceed its anchor.
  Outstanding fungible supply additionally depends on fee burns, debt and pooled losses.
- **Test.** `Bridge.ReservationSettlement.test.ts` "bounds the grind at a
  request-time amount floor, not the settlement-time dust floor" (:500)
  asserts the cumulative shrink; `ReservationProofs.test.ts` :1571.
- **Status.** holds.

### INV-303 Fee bounds per hop and cumulative grind

- **Statement.**
  - Acceptance: `fee = deposit.amount - anchorAmount` with
    `fee <= action.txMaxFee` and `anchorAmount >= action.minAmount`.
  - Re-anchor: `minerFee = oldAnchor - newAnchor` with
    `minerFee <= action.txMaxFee` and `newAnchor > action.txMaxFee` (dust
    floor).
  - Cumulative identity: `initialAnchor - currentAnchor = cumulativeReanchorFee`
    across settled re-anchor hops of the same lineage.
    Request-time `anchorAmount > liveTxMaxFee + liveMinAmount` bounds the next hop.
    The floor uses current governance values, not an immutable lifetime minimum.
    Lowering the live minimum can permit further grind.
- **Enforcing code.**
  - acceptance: `validateAnchorOutput` (`ReservationProofs.sol:441-467`):
    fee bound :452-454, min :463-465; request-time floor
    `deposit.amount > minAmount + 2 * txMaxFee`
    (`tbtc-v2@dc5c2ed71:solidity/contracts/bridge/Reservation.sol:577-584`).
  - re-anchor: `submitReservationReanchorProof` (`ReservationProofs.sol:
    685-698`): fee bound :685-690, dust floor :692-698; request-time grind
    cap `anchorAmount > txMaxFee + minAmount` (`Reservation.sol:853-857`).
- **Why.** Without the per-hop and cumulative caps a re-anchor chain could
  grind the anchor to dust and destroy the redemption value.
- **Test.** `ReservationProofs.test.ts` "should revert if miner fee exceeds
  snapshotted txMaxFee" (:734); `Reservation.test.ts` "should revert when the
  fee exceeds the snapshotted bound" (:1086), "should revert on underflow when
  amount < anchorAmount" (:1112); `Bridge.ReservationSettlement.test.ts` :500
  (cumulative).
- **Status.** holds.

### INV-304 Single-input / single-output lineage

- **Statement.** Every reservation anchor transaction has exactly one input
  (the funding outpoint for acceptance, the current anchor for re-anchor) and
  exactly one P2(W)PKH output.
- **Enforcing code.** `parseSingleOutput` requires `outputsCount == 1`
  (`ReservationProofs.sol:881-892`); `consumeAnchor` asserts the single input
  points at the current anchor (`ReservationProofs.sol:906-914`);
  `consumeAcceptedDeposit` asserts the input spends the reserved deposit
  (`ReservationProofs.sol:411-416`).
- **Why.** The 1-in-1-out shape is what makes the anchor lineage auditable and
  the UTXO unambiguously one position's.
- **Test.** `ReservationProofs.test.ts` "should revert if output vector does
  not have exactly 1 output (0 outputs)" (:699), "should revert if input
  vector has not exactly 1 input (0 inputs)" (:767); `Reservation.test.ts`
  "should revert for a multi-output vector" (:1050).
- **Status.** holds.

---

## 4. Authorization

### INV-401 Who can move which state

| Transition | Actor | Enforcing code | Test |
|---|---|---|---|
| `requestReservationAcceptance` | only the deposit's depositor | `Reservation.sol:511-514` | `Reservation.test.ts:171` |
| `requestReservationReanchor` | permissionless; Live source needs privilege | `Reservation.sol:742-933`; `ReservationRouter.sol:239-261 (privileged = msg.sender == governance at :248)` | `Bridge.ReservationStrandingLibrary.test.ts:1640` |
| `submitReservationAcceptanceProof` | SPV maintainer only | `ReservationRouter.sol:263-287` | `Bridge.ReservationAcceptanceAuthorization.test.ts:472` |
| `submitReservationReanchorProof` | SPV maintainer only | `ReservationRouter.sol:289-313` | `Bridge.ReservationSourceAnchorBinding.test.ts:412` |
| `notifyReservationActionTimeout` | permissionless | `ReservationRouter.sol:315-333` | `Bridge.ReservationStrandingLibrary.test.ts:1027` |
| `notifyReservationAcceptanceTimedOut` | permissionless | `ReservationRouter.sol:334-365` | `Bridge.ReservationOccupancy.test.ts:68` |
| `notifyStaleReservedDeposit` | permissionless | `ReservationRouter.sol:396-408` | `Bridge.ReservationStranding.test.ts:727` |
| `forceStaleReservedDeposit` | governance only | `ReservationRouter.sol:410-423` | `none` |
| `notifyReservationStranded` | permissionless | `ReservationRouter.sol:425-448` | `Bridge.ReservationStranding.test.ts:525` |
| `updateReservationParameters` | governance only | `ReservationRouter.sol:367-389` | `Bridge.ReservationFeasibilityGuard.test.ts:207` |
| `updateReservationCaps` | governance only (four arguments at `9d2086965`; five at `tbtc-v2@806b1106f`, adding `handoffReserveWallets`, which must be non-zero) | `ReservationRouter.sol:450-462` (at `9d2086965`); at `tbtc-v2@806b1106f`: router `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:476` (`onlyGovernance`), library `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1687-1745`, staging `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896` | `Bridge.ReservationCaps.test.ts:144` (at `9d2086965`); at `tbtc-v2@806b1106f`: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationCaps.test.ts:163` (zero reserve reverts), `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4705` (five-argument begin) |
| `closeReservation` | none (M2, no entry point) | `Reservation.sol:1141-1165` | `none` |
| `releaseReservedDepositToPool` (D26, `d20cf74ba` only) | the deposit's own depositor; router has no modifier | `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1381-1384`; `tbtc-v2@d20cf74ba:solidity/contracts/bridge/ReservationRouter.sol:433-437` | `tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts:475` |
| vault `financeInKindFee` | bridge only | `ReservationVault.sol:216` | `ReservationVault.test.ts:275` |
| vault `receiveBalanceIncrease` | bank only | `ReservationVault.sol:onlyBank modifier :102-104; receiveBalanceIncrease :164` | `ReservationVault.test.ts:209` |
| vault `sweepFees`/`updateFeeReserveTarget`/`updateInitiationFee` | owner only | `ReservationVault.sol:278, 302, 335` | `ReservationVault.test.ts:588` |
| vault `repayInKindFeeDebt` | anyone | `ReservationVault.sol:241` | `ReservationVault.test.ts:500` |
| `setVaultStatus` | governance only | `Bridge.sol:1288-1294` | `none` |

- **Why.** Only a trusted SPV maintainer can settle (they hold the Bitcoin
  proofs); only governance can change caps/params; the depositor is the only
  one who may authorize acceptance of *their* deposit; re-anchor is the one
  intentionally permissionless state move (source in `MovingFunds`/`Closing`),
  gated by the third-party hold cap.
- **Status.** holds. (The "revert when a non-empty member proof fails
  validation" is pinned by `Bridge.ReservationThirdPartyReanchorHolds.test.ts:235`;
  the third-party exemption by :151/:179/:204.)

### INV-402 Re-anchor source-wallet gating

- **Statement.** Any caller can request a re-anchor from `MovingFunds`
  or `Closing`, subject to cooldown, authorization and capacity guards.
  A `Live` source accepts only the governance-discriminated router call.
  The target must be `Live` and different from the source.
- **Enforcing code.** `requestReservationReanchor` checks its `privileged`
  argument for a Live source (`Reservation.sol:797`).
  The router passes `msg.sender == governance`
  (`ReservationRouter.sol:247-252`).
  The separate `isPrivileged` expression includes owner, maintainer and
  proven source members (`Reservation.sol:786-790`).
  It exempts them from the hold cap, not Live-source rotation or cooldown.
  Retiring-source and target guards are at :802, :810 and :814 (at `9d2086965`).
  At `tbtc-v2@806b1106f` the same `isPrivileged` expression
  (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:814-818`) also
  exempts a request off a non-Live source from the target count and amount
  caps (INV-114). It still does not authorize Live-source rotation
  ("Only governance can rotate a Live wallet's anchor"
  `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:827`; governance
  from a Live source into a full target still reverts,
  `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:485`).
- **Why.** Owner or maintainer status must not authorize ordinary Live-wallet rotation.
- **Test.** `Bridge.ReservationStrandingLibrary.test.ts` "rejects when source
  wallet is Live and call is not privileged" (:1640), "rejects when target
  wallet is not in Live state" (:1672), "successfully requests reservation
  re-anchor from MovingFunds wallet (unprivileged happy path)" (:1191), "
  ...from Closing wallet" (:1240), "...by governance (privileged happy
  path)" (:1295).
- **Status.** holds.

---
