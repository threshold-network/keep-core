# Reservation Touchpoint Inventory: Non-Reservation Bridge Files

Sources verified against `solidity/contracts/bridge/` at the `feat/utxo-reservation-guards` tip (through PR #1094). PR attribution from `../epic-merge-plan.md` section 0.1. Where a modification spans multiple PRs in the stack (each layer builds on the prior), the PR listed is the one whose branch introduced the change to the shared tip.

---

## Deposit.sol

The deposit reveal path is the primary integration seam. A deposit is classified as reserved at reveal time, and that classification is permanent.

| Item | Source | PR | Kind | m1 | m2 | Note |
|------|--------|----|------|----|----|------|
| `DepositRevealInfo.vault` field (address) | `Deposit.sol:62` | #1088 | param | yes | yes | Optional vault routing address; zero for ordinary deposits. When set to `reservationVault`, the deposit is classified as reserved. |
| `DepositRequest.vault` field (address) | `Deposit.sol:120` | #1088 | storage | yes | yes | Stored on every deposit reveal at `Deposit.sol:355`. The vault address is part of the permanent deposit record. |
| `DepositRevealed` event includes `vault` param | `Deposit.sol:134` | #1088 | event | yes | yes | The `vault` parameter is the last field in the event, emitted at `Deposit.sol:404`. Off-chain consumers read this to determine reserved classification. |
| `isReservedDeposit` local variable in `_revealDeposit` | `Deposit.sol:210-211` | #1094 | internal | yes | yes | `bool isReservedDeposit = reveal.vault != address(0) && reveal.vault == self.reservationVault`. This is the reveal-time classification. It reads `self.reservationVault` from `BridgeState`. |
| Reserved-deposit refund deadline extraction | `Deposit.sol:220-226` | #1094 | internal | yes | yes | When `depositRevealAheadPeriod == 0` (ordinary deposits skip validation), reserved deposits still extract the refund deadline via `BTCUtils.reverseUint32(reveal.refundLocktime)`. This prevents acceptance requests for deposits whose script deadline the wallet can never sign. |
| `pendingReservedDeposit` mapping write | `Deposit.sol:361-369` | #1094 | storage | yes | yes | If `isReservedDeposit`, writes `BridgeState.PendingReservedDeposit(true, reveal.walletPubKeyHash, refundDeadline, refundDeadlineValidated)` and increments `self.pendingReservedDeposits`. The `isReserved` flag is permanent; the remaining fields may be cleared after acceptance or staleness. |
| `deposit.vault = reveal.vault` storage write | `Deposit.sol:355` | #1088 | storage | yes | yes | Every deposit stores its vault address, not just reserved ones. |

### Reveal path summary (item 2)

A deposit is revealed with a reservation and a designated wallet through the `_revealDeposit` function (`Deposit.sol:184`). The caller provides a `DepositRevealInfo` struct whose `vault` field (`:62`) is set to the reservation vault address and whose `walletPubKeyHash` field (`:54`) designates the wallet. The function:

1. Requires the wallet be in `Live` state (`:193-197`).
2. Validates that `reveal.vault` is either zero or a trusted vault (`:205-208`).
3. Classifies the deposit: `isReservedDeposit = reveal.vault != address(0) && reveal.vault == self.reservationVault` (`:210-211`).
4. For reserved deposits, extracts the refund deadline even when `depositRevealAheadPeriod` is zero (`:220-226`).
5. Stores the deposit request with `deposit.vault = reveal.vault` (`:355`).
6. If reserved, writes `self.pendingReservedDeposit[depositKey] = PendingReservedDeposit(true, reveal.walletPubKeyHash, refundDeadline, refundDeadlineValidated)` and increments `self.pendingReservedDeposits` (`:361-369`).
7. Emits `DepositRevealed` with the vault address (`:404`).

The `isReserved` flag in `PendingReservedDeposit` (`BridgeState.sol:38`) is permanent: it is never cleared, even after acceptance or staleness. This is the reveal-time classification that makes the deposit permanently ineligible for ordinary sweep and permanently eligible for reservation anchoring. The `Bridge.isReservedDeposit(uint256)` view (`Bridge.sol:1632-1638`) reads `self.pendingReservedDeposit[depositKey].isReserved`.

---

## Wallets.sol

The wallet lifecycle is deeply coupled to reservations. A wallet that still custodies reservations cannot close, cannot finalize closing, and its MovingFunds timeout ignores reservation obligations and slashes anyway.

| Item | Source | PR | Kind | m1 | m2 | Note |
|------|--------|----|------|----|----|------|
| `movingFundsRequestedAt` comment references reservations | `Wallets.sol:79-80` | #1094 | internal | yes | yes | Comment: "Zero marks a successfully proven generation whose remaining reservation or sweep obligations kept it in MovingFunds." The field itself is pre-existing; the comment was updated to reflect reservation-aware semantics. |
| `notifyWalletFundsMoved`: reservation count check before closing | `Wallets.sol:437-441` | #1094 | invariant | yes | yes | After a successful moving-funds proof, `walletReservationsCount[walletPubKeyHash] == 0 && pendingMovedFundsSweepRequestsCount == 0` is required to begin closing. If reservations remain, the wallet stays in MovingFunds with `movingFundsRequestedAt = 0` (completion sentinel). |
| `notifyWalletFundsMoved`: comment on reservation obligations | `Wallets.sol:431-433` | #1094 | internal | yes | yes | "A proof records successful completion even when reservation or incoming-sweep obligations prevent the state from entering Closing. Zero is the completion sentinel and cannot be reported as a timeout." |
| `notifyWalletMovingFundsBelowDust`: calls `beginWalletClosing` | `Wallets.sol:478-489` | #1094 | internal | yes | yes | Requires MovingFunds state, then calls `beginWalletClosing`. `beginWalletClosing` will revert if `walletReservationsCount != 0`, so a wallet with reservations and below-dust balance cannot close this way. |
| `notifyWalletMovingFundsTimeout`: NO reservation check | `Wallets.sol:498-516` | #1094 | internal | yes | yes | **Critical m1 failure mode.** The timeout handler does NOT check `walletReservationsCount`. It calls `seize` then `terminateWallet` unconditionally. See exact call sequence below. |
| `moveFunds`: reservation count check | `Wallets.sol:627-629` | #1094 | invariant | yes | yes | `wallet.mainUtxoHash == bytes32(0) && self.walletReservationsCount[walletPubKeyHash] == 0 && wallet.pendingMovedFundsSweepRequestsCount == 0` required to skip MovingFunds and begin closing immediately. A wallet with anchors but no main UTXO enters MovingFunds. |
| `moveFunds`: comment on reservation anchors | `Wallets.sol:635-639` | #1094 | internal | yes | yes | "The wallet holds funds: a main UTXO, reservation anchors, or both. A wallet with anchors but no main UTXO still enters the moving funds process -- its reservations must be re-anchored to other wallets (or redeemed/dissolved) before it can begin closing." |
| `beginWalletClosing`: reservation count precondition | `Wallets.sol:674-677` | #1094 | invariant | yes | yes | `require(self.walletReservationsCount[walletPubKeyHash] == 0, "Wallet still custodies reservations")`. A wallet with active reservations cannot enter Closing. |
| `beginWalletClosing`: comment on why reservations block closing | `Wallets.sol:668-672` | #1094 | internal | yes | yes | "A wallet that still custodies reservation anchors, reserved capacity, or pending moved-funds sweep requests has not finished moving its funds. Entering Closing would make those obligations unprocessable because their proof and timeout paths require a Live or MovingFunds wallet." |
| `finalizeWalletClosing`: reservation count precondition | `Wallets.sol:706-709` | #1094 | invariant | yes | yes | `require(self.walletReservationsCount[walletPubKeyHash] == 0, "Wallet still custodies reservations")`. Even in Closing, a late sweep can register, but a wallet with reservations cannot finalize. |
| `finalizeWalletClosing`: comment | `Wallets.sol:703-705` | #1094 | internal | yes | yes | "A wallet still custodying reservation anchors or targeted by pending moved-funds sweeps must not close ultimately." |
| `terminateWallet` | `Wallets.sol:733-757` | pre-existing | internal | yes | yes | Sets state to Terminated, decrements liveWalletsCount if Live, emits WalletTerminated, unsets active wallet if applicable, calls `ecdsaWalletRegistry.closeWallet`. No reservation cleanup here; stranding (`Reservation.sol`) handles that separately. |

### Wallet lifecycle coupling: exact MovingFunds timeout call sequence (item 3)

When a wallet custodies reservations and its MovingFunds clock expires, the exact call sequence is:

1. **External entry**: `MovingFunds.notifyMovingFundsTimeout(self, walletPubKeyHash, walletMembersIDs)` at `MovingFunds.sol:583`.
   - Requires `wallet.state == MovingFunds` (`:588-591`).
   - Requires `movingFundsRequestedAt != 0` (`:594-596`). Note: if the wallet completed its funds-move proof but retained reservations, `movingFundsRequestedAt` was deleted to zero (`Wallets.sol:434`), so this timeout path is NOT reachable for a wallet that proved funds moved but stayed for reservations. This is the completion sentinel.
   - Requires `block.timestamp > movingFundsRequestedAt + self.movingFundsTimeout` (`:599-602`).
   - Calls `self.notifyWalletMovingFundsTimeout(walletPubKeyHash, walletMembersIDs)` (`:604`).
   - Emits `MovingFundsTimedOut(walletPubKeyHash)` (`:607`).

2. **Internal slashing**: `Wallets.notifyWalletMovingFundsTimeout(self, walletPubKeyHash, walletMembersIDs)` at `Wallets.sol:498`.
   - Requires `wallet.state == MovingFunds` (`:502-505`).
   - Calls `self.ecdsaWalletRegistry.seize(self.movingFundsTimeoutSlashingAmount, self.movingFundsTimeoutNotifierRewardMultiplier, msg.sender, wallet.ecdsaWalletID, walletMembersIDs)` at `Wallets.sol:507-513`. This slashes the wallet operators.
   - Calls `terminateWallet(self, walletPubKeyHash)` at `Wallets.sol:515`.

3. **Termination**: `Wallets.terminateWallet(self, walletPubKeyHash)` at `Wallets.sol:733`.
   - If `wallet.state == Live`, decrements `self.liveWalletsCount` (`:738-739`). (In this path the wallet is MovingFunds, so this branch is skipped.)
   - Sets `wallet.state = Terminated` (`:741`).
   - Emits `WalletTerminated(wallet.ecdsaWalletID, walletPubKeyHash)` (`:744`).
   - If `self.activeWalletPubKeyHash == walletPubKeyHash`, deletes it (`:746-751`).
   - Calls `self.ecdsaWalletRegistry.closeWallet(wallet.ecdsaWalletID)` at `Wallets.sol:753`.

**What happens to the reservations**: The wallet is now `Terminated`. The reservations it custodied are NOT cleaned up by this path. They become stranded: `ReservationRouter.notifyReservationStranded(reservationKey)` (`ReservationRouter.sol:473`) is callable permissionlessly when the custodying wallet is `Terminated` and the reservation is `Active`. Stranding closes the reservation, releases the per-wallet count and amount, and the owner's minted balance remains an ordinary pooled claim.

**The m1 failure mode**: In m1 (variant B), re-anchor is unbounded (its `dissolutionEligibleAt` gate is deleted). So a wallet with reservations can be re-anchored to another wallet before the timeout fires. But if the wallet's `movingFundsRequestedAt` is non-zero and the timeout expires before re-anchor completes, the wallet is slashed and terminated with no reservation cleanup in the timeout path itself. The reservations become stranded.

---

## MovingFunds.sol

The MovingFunds library has one reservation-aware precondition in its below-dust notification.

| Item | Source | PR | Kind | m1 | m2 | Note |
|------|--------|----|------|----|----|------|
| `notifyMovingFundsBelowDust` docstring: reservation anchor precondition | `MovingFunds.sol:620-621` | #1094 | param | yes | yes | Docstring requirement: "The wallet must not custody reservation anchors or have pending moved-funds sweep requests." This is enforced by `beginWalletClosing` (`Wallets.sol:674-677`), which `notifyWalletMovingFundsBelowDust` calls (`Wallets.sol:488`). |
| `notifyMovingFundsTimeout` entry point | `MovingFunds.sol:583-608` | pre-existing | entry-point | yes | yes | The timeout handler itself has NO reservation check. See the Wallets.sol section for the exact call sequence. |

---

## Redemption.sol

The Redemption.sol file contains the `IRedemptionWatchtower` interface with reserved-redemption-specific functions, and the pooled redemption path has no direct reservation coupling. The sweep proposal guard is in `WalletProposalValidator.sol`, not here.

| Item | Source | PR | Kind | m1 | m2 | Note |
|------|--------|----|------|----|----|------|
| `IRedemptionWatchtower.getReservedRedemptionDelay` | `Redemption.sol:74-77` | #1091 | view | no | yes | Returns the veto delay for a pending reserved redemption generation. m2 only; m1 has no reserved redemption. |
| `IRedemptionWatchtower.getReservedRedemptionDelaySchedule` | `Redemption.sol:87-94` | #1091 | view | no | yes | Returns the three-level veto delay schedule for a newly requested reserved redemption. m2 only. |
| `IRedemptionWatchtower.isSafeReservedRedemption` | `Redemption.sol:105-108` | #1091 | view | no | yes | Safety check for reserved redemptions: neither owner nor redeemer may be banned. m2 only. |

### Redemption-path coupling (item 6)

The pooled redemption path (`Redemption.requestRedemption` at `Redemption.sol:528`) has NO direct reservation coupling. It operates on Bank balances and wallet main UTXOs, not on reservation anchors. The key interaction is indirect: reserved deposits are excluded from the sweep path by the `isReservedDeposit` guard in `WalletProposalValidator.sol:312-315`, so they never enter the ordinary sweep pipeline and never contribute to a wallet's main UTXO. The pooled redemption path is therefore unaffected by m1 deferring reserved redemption. The `IRedemptionWatchtower` interface functions for reserved redemptions (`:74-108`) are consumed only by `WalletProposalValidator.validateReservedRedemptionProposal` (`:1135-1142`), which is itself m2-only.

---

## WalletProposalValidator.sol

This non-upgradeable contract gained reservation-aware proposal validation functions and imports. The constants are shared with the existing deposit/redemption validators.

| Item | Source | PR | Kind | m1 | m2 | Note |
|------|--------|----|------|----|----|------|
| Import `IReservationBridge.sol` | `WalletProposalValidator.sol:25` | #1091 | param | yes | yes | Required for all reservation proposal validators that call `IReservationBridge(address(bridge))`. |
| Import `Reservation.sol` | `WalletProposalValidator.sol:26` | #1091 | param | yes | yes | Required for `Reservation.ActionType`, `Reservation.ActionState`, `Reservation.ReservationRequest`, `Reservation.ReservationAction`. |
| `DEPOSIT_MIN_AGE` constant | `WalletProposalValidator.sol:146-147` | pre-existing | param | yes | yes | 2 hours. Used by `validateReservationAnchorProposal` at `:1037` (deposit must be old enough before anchoring). Shared with `validateDepositSweepProposal`. |
| `DEPOSIT_REFUND_SAFETY_MARGIN` constant | `WalletProposalValidator.sol:164-165` | pre-existing | param | yes | yes | 24 hours. Used by `validateReservationAnchorProposal` at `:1084` (refund safety margin). Shared with `validateDepositSweepProposal`. |
| `REDEMPTION_REQUEST_MIN_AGE` constant | `WalletProposalValidator.sol:179` | pre-existing | param | yes | yes | 600 seconds (10 minutes). Used by `validateReservedRedemptionProposal` at `:1135` (reserved redemption min age). Shared with `validateRedemptionProposal`. |
| `REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN` constant | `WalletProposalValidator.sol:198-199` | pre-existing | param | yes | yes | 2 hours. Used by all four reservation proposal validators (`:1039`, `:1157`, `:1218`, `:1273`) for authorization timeout safety margin. Shared with `validateRedemptionProposal`. |
| `ReservationAnchorProposal` struct | `WalletProposalValidator.sol:918-927` | #1091 | param | yes | yes | Helper for acceptance anchor proposal validation. Contains `walletPubKeyHash`, `depositKey`, `requestNonce`, `anchorTxFee`. |
| `ReservedRedemptionProposal` struct | `WalletProposalValidator.sol:930-939` | #1091 | param | no | yes | Helper for reserved redemption proposal validation. m2 only (reserved redemption is deferred). |
| `ReservationReanchorProposal` struct | `WalletProposalValidator.sol:943-954` | #1091 | param | yes | yes | Helper for re-anchor proposal validation. Contains `sourceWalletPubKeyHash`, `reservationKey`, `requestNonce`, `targetWalletPubKeyHash`, `reanchorTxFee`. |
| `ReservationDissolutionProposal` struct | `WalletProposalValidator.sol:958-966` | #1091 | param | no | yes | Helper for dissolution proposal validation. m2 only (dissolution is deferred). |
| `requirePendingAction` internal helper | `WalletProposalValidator.sol:975-989` | #1091 | internal | yes | yes | Fetches a reservation action generation via `IReservationBridge(address(bridge)).reservationActions(reservationKey, requestNonce)` (`:980-983`) and reverts unless `actionType == expectedType && state == Pending` (`:984-988`). Used by all four reservation proposal validators. |
| `requireWalletLiveOrMovingFunds` internal helper | `WalletProposalValidator.sol:1291-1300` | #1094 | internal | yes | yes | Reverts unless wallet is `Live` or `MovingFunds`. Used by `validateReservationAnchorProposal` (`:1014`), `validateReservedRedemptionProposal` (`:1113`), `validateReservationDissolutionProposal` (`:1253`). NOT used by `validateReservationReanchorProposal` (which checks source wallet state differently). |
| `validateDepositSweepProposal`: reserved deposit exclusion | `WalletProposalValidator.sol:312-315` | #1094 | view | yes | yes | `require(!bridge.isReservedDeposit(depositKeyUint), "Reserved deposits must not be swept")`. This is the sweep-path guard that prevents reserved deposits from being swept in an ordinary deposit sweep. Reads `Bridge.isReservedDeposit` (`Bridge.sol:1632`). |
| `validateReservationAnchorProposal` | `WalletProposalValidator.sol:1010-1094` | #1091 | view | yes | yes | Validates acceptance anchor proposals: wallet Live or MovingFunds (`:1014`), pending Acceptance action (`:1025-1028`), authorized wallet match (`:1031-1034`), timeout safety margin (`:1037-1040`), deposit revealed/not swept/old enough (`:1043-1051`), deposit was revealed as reserved (`:1057-1059`), fee positive and within bound (`:1062-1068`), deposit extra info valid (`:1071-1085`), deposit controlled by proposal wallet (`:1088-1090`). |
| `validateReservedRedemptionProposal` | `WalletProposalValidator.sol:1110-1170` | #1091 | view | no | yes | Validates reserved redemption proposals. m2 only (reserved redemption is deferred). Calls `IRedemptionWatchtower.getReservedRedemptionDelay` (`:1138-1142`). |
| `validateReservationReanchorProposal` | `WalletProposalValidator.sol:1186-1235` | #1091 | view | yes | yes | Validates re-anchor proposals: reservation custodied by source wallet (`:1193-1195`), pending Reanchor action (`:1198-1201`), target wallet match (`:1204-1206`), target wallet must be Live (`:1209-1212`), timeout safety margin (`:1217-1219`), fee positive and within bound (`:1222-1228`), re-anchor amount above dust floor (`:1230-1232`). Does NOT call `requireWalletLiveOrMovingFunds` for the source wallet; checks source via reservation custody instead. |
| `validateReservationDissolutionProposal` | `WalletProposalValidator.sol:1250-1288` | #1091 | view | no | yes | Validates dissolution proposals. m2 only (dissolution is deferred). |

---

## BridgeGovernance.sol

The governance contract wires reservation parameter updates through the same governance-delay machinery as all other Bridge parameters. No bypass.

| Item | Source | PR | Kind | m1 | m2 | Note |
|------|--------|----|------|----|----|------|
| Import `IReservationBridge.sol` | `BridgeGovernance.sol:22` | #1090 | param | yes | yes | Required for `IReservationBridge(address(bridge)).updateReservationParameters` and `updateReservationCaps` calls in the finalize functions. |
| `using` for `ReservationData` | `BridgeGovernance.sol:36` | #1090 | param | yes | yes | Enables `BridgeGovernanceParameters.ReservationData` library functions. |
| `using` for `ReservationCapsData` | `BridgeGovernance.sol:37` | #1090 | param | yes | yes | Enables `BridgeGovernanceParameters.ReservationCapsData` library functions. |
| `reservationData` storage variable | `BridgeGovernance.sol:45` | #1090 | storage | yes | yes | `BridgeGovernanceParameters.ReservationData internal reservationData`. Stages all 9 reservation parameters. |
| `reservationCapsData` storage variable | `BridgeGovernance.sol:46` | #1090 | storage | yes | yes | `BridgeGovernanceParameters.ReservationCapsData internal reservationCapsData`. Stages the 2 reservation caps. |
| `ReservationParametersUpdateStarted` event | `BridgeGovernance.sol:300-311` | #1090 | event | yes | yes | Mirrors `BridgeGovernanceParameters.ReservationParametersUpdateStarted` for the BridgeGovernance ABI. |
| `ReservationCapsUpdateStarted` event | `BridgeGovernance.sol:312-316` | #1090 | event | yes | yes | Mirrors `BridgeGovernanceParameters.ReservationCapsUpdateStarted`. |
| `beginReservationParametersUpdate` | `BridgeGovernance.sol:1843-1865` | #1090 | entry-point | yes | yes | `onlyOwner`. Delegates to `reservationData.beginReservationParametersUpdate(...)` (`:1854`). Stages all 9 parameters. |
| `beginReservationCapsUpdate` | `BridgeGovernance.sol:1871-1879` | #1090 | entry-point | yes | yes | `onlyOwner`. Delegates to `reservationCapsData.beginReservationCapsUpdate(...)` (`:1875`). Stages the 2 caps. |
| `finalizeReservationCapsUpdate` | `BridgeGovernance.sol:1884-1892` | #1090 | entry-point | yes | yes | `onlyOwner`. Reads staged values, calls `reservationCapsData.finalizeReservationCapsUpdate(governanceDelay())` (`:1887`) which enforces the governance delay, then calls `IReservationBridge(address(bridge)).updateReservationCaps(...)` (`:1888-1891`). Uses the SAME `governanceDelay()` as all other parameters. |
| `finalizeReservationParametersUpdate` | `BridgeGovernance.sol:1897-1912` | #1090 | entry-point | yes | yes | `onlyOwner`. Reads staged values, calls `reservationData.finalizeReservationParametersUpdate(governanceDelay())` (`:1900`) which enforces the governance delay, then calls `IReservationBridge(address(bridge)).updateReservationParameters(...)` (`:1901-1911`). Uses the SAME `governanceDelay()` as all other parameters. |

### Governance wiring summary (item 4)

Reservation parameters reach the Bridge through the same two-phase governance-delay machinery as all other Bridge parameters. The flow is:
1. `beginReservationParametersUpdate` (`BridgeGovernance.sol:1843`) stages 9 parameters in `reservationData` storage and records `block.timestamp`.
2. After `governanceDelay()` elapses, `finalizeReservationParametersUpdate` (`:1897`) enforces the delay via `onlyAfterGovernanceDelay` (inside `BridgeGovernanceParameters.finalizeReservationParametersUpdate` at `:1700-1703`) and calls `IReservationBridge(address(bridge)).updateReservationParameters(...)` (`:1901`).
3. The caps follow the same pattern via `beginReservationCapsUpdate` (`:1871`) and `finalizeReservationCapsUpdate` (`:1884`).

No reservation parameter bypasses the governance delay. The `setReservationRouter` function (`Bridge.sol:2114`) is `onlyGovernance` but is a one-time setter, not a parameter update, so it does not use the delay machinery at all. This is by design (the router is a delegatecall target, equivalent to a Bridge implementation change).

---

## BridgeGovernanceParameters.sol

The library that implements the staging, delay enforcement, and event emission for reservation parameters.

| Item | Source | PR | Kind | m1 | m2 | Note |
|------|--------|----|------|----|----|------|
| `ReservationData` struct | `BridgeGovernanceParameters.sol:1575-1586` | #1090 | storage | yes | yes | 9 staged parameter fields plus `reservationParametersChangeInitiated` timestamp. |
| `ReservationCapsData` struct | `BridgeGovernanceParameters.sol:1588-1592` | #1090 | storage | yes | yes | 2 staged cap fields plus `reservationCapsChangeInitiated` timestamp. |
| `ReservationCapsUpdateStarted` event | `BridgeGovernanceParameters.sol:1594-1598` | #1090 | event | yes | yes | Emitted by `beginReservationCapsUpdate`. |
| `ReservationParametersUpdateStarted` event | `BridgeGovernanceParameters.sol:1600-1611` | #1090 | event | yes | yes | Emitted by `beginReservationParametersUpdate`. |
| `beginReservationParametersUpdate` library function | `BridgeGovernanceParameters.sol:1616-1652` | #1090 | internal | yes | yes | Stages all 9 parameters, sets `reservationParametersChangeInitiated = block.timestamp`, emits event. |
| `beginReservationCapsUpdate` library function | `BridgeGovernanceParameters.sol:1656-1671` | #1090 | internal | yes | yes | Stages both caps, sets `reservationCapsChangeInitiated = block.timestamp`, emits event. |
| `finalizeReservationCapsUpdate` library function | `BridgeGovernanceParameters.sol:1678-1689` | #1090 | internal | yes | yes | Enforces `onlyAfterGovernanceDelay(self.reservationCapsChangeInitiated, governanceDelay)` (`:1683-1686`), then clears the initiated timestamp (`:1688`). |
| `finalizeReservationParametersUpdate` library function | `BridgeGovernanceParameters.sol:1695-1706` | #1090 | internal | yes | yes | Enforces `onlyAfterGovernanceDelay(self.reservationParametersChangeInitiated, governanceDelay)` (`:1700-1703`), then clears the initiated timestamp (`:1705`). |

---

## Bridge.sol (supplementary: not in the assigned 7-file list but is a touched non-reservation file)

Bridge.sol is touched by the reservation feature in three places that are integration seams a rewrite must reproduce.

| Item | Source | PR | Kind | m1 | m2 | Note |
|------|--------|----|------|----|----|------|
| `isReservedDeposit` view function | `Bridge.sol:1632-1638` | #1094 | view | yes | yes | `external view returns (bool)`. Reads `self.pendingReservedDeposit[depositKey].isReserved`. Consumed by `WalletProposalValidator.validateDepositSweepProposal` (`:313`) and `validateReservationAnchorProposal` (`:1058`). |
| `setReservationRouter` entry point | `Bridge.sol:2114-2119` | #1090 | entry-point | yes | yes | `external onlyGovernance`. One-time setter. Delegates to `self.setReservationRouter(_reservationRouter)` (`BridgeState.sol:1014`). Replacing the router requires a Bridge implementation upgrade. |
| `fallback()` delegatecall dispatcher | `Bridge.sol:2135-2165` | #1090 | entry-point | yes | yes | Routes calls with unmatched selectors to `self.reservationRouter` via `delegatecall`. Reads `reservationRouter` at `:2136`, requires non-zero at `:2137`, executes inline assembly `delegatecall` at `:2148-2158`. This is how the reservation surface is exposed without growing the Bridge past EIP-170. |

---

## WalletProposalValidatorConstants.sol (supplementary)

The shared constants file. All constants predate the reservation feature but are consumed by reservation proposal validators.

| Item | Source | PR | Kind | m1 | m2 | Note |
|------|--------|----|------|----|----|------|
| `DEPOSIT_MIN_AGE` | `WalletProposalValidatorConstants.sol:11` | pre-existing | param | yes | yes | 2 hours. Consumed by `validateReservationAnchorProposal` via `WalletProposalValidator.DEPOSIT_MIN_AGE`. |
| `DEPOSIT_REFUND_SAFETY_MARGIN` | `WalletProposalValidatorConstants.sol:12` | pre-existing | param | yes | yes | 24 hours. Consumed by `validateReservationAnchorProposal` via `WalletProposalValidator.DEPOSIT_REFUND_SAFETY_MARGIN`. |
| `REQUEST_TIMEOUT_SAFETY_MARGIN` | `WalletProposalValidatorConstants.sol:13` | pre-existing | param | yes | yes | 2 hours. Consumed by all four reservation proposal validators via `WalletProposalValidator.REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN`. |

---

## Wallet proposal validation (item 5)

The reservation-aware proposal validators in `WalletProposalValidator.sol` are:

1. **`validateReservationAnchorProposal`** (`:1010`): m1 yes. Validates acceptance anchor proposals. Checks wallet Live or MovingFunds, pending Acceptance action, authorized wallet, timeout safety margin, deposit revealed/not swept/old enough, deposit classified as reserved, fee within bound, deposit extra info valid, deposit controlled by proposal wallet. Uses constants: `DEPOSIT_MIN_AGE` (2h), `DEPOSIT_REFUND_SAFETY_MARGIN` (24h), `REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN` (2h).

2. **`validateReservedRedemptionProposal`** (`:1110`): m1 no (reserved redemption deferred). Validates reserved redemption proposals. Uses `REDEMPTION_REQUEST_MIN_AGE` (600s) and `REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN` (2h). Calls `IRedemptionWatchtower.getReservedRedemptionDelay`.

3. **`validateReservationReanchorProposal`** (`:1186`): m1 yes. Validates re-anchor proposals. Checks reservation custody, pending Reanchor action, target wallet match, target wallet Live, timeout safety margin, fee within bound, amount above dust floor. Uses `REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN` (2h). Does NOT use `requireWalletLiveOrMovingFunds` for the source wallet.

4. **`validateReservationDissolutionProposal`** (`:1250`): m1 no (dissolution deferred). Validates dissolution proposals. Uses `REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN` (2h).

All four share the `requirePendingAction` helper (`:975`) which reads `IReservationBridge(address(bridge)).reservationActions(reservationKey, requestNonce)`.

The constants are not reservation-specific; they are the same timing constants used by the existing deposit sweep and pooled redemption validators. No new constants were added for reservations.

---

## Open questions

1. **DECISION NEEDED: Should `WalletProposalValidator.sol` ship its m2-only proposal validators (reserved redemption, dissolution) in m1 behind a flag, or omit them entirely?** The rules say router entry points may be omitted freely (rule 3) and dissolution/redemption are m2 (rule 4). But `WalletProposalValidator` is non-upgradeable (`WalletProposalValidator.sol:33-34`: "This contract is non-upgradeable and does not have any write functions"). Unlike router code reached by delegatecall, a non-upgradeable contract cannot be replaced by a Bridge implementation upgrade. If the m2 validators are omitted from the m1 deployment of `WalletProposalValidator`, they cannot be added later without redeploying the contract and updating the `bridge.walletProposalValidator` reference. Is the contract already deployed and referenced immutably, or can it be redeployed for m2?

2. **DECISION NEEDED: Does `beginWalletClosing` (`Wallets.sol:674`) need to remain reservation-gated in m1, or should it be relaxed?** In m1 variant B, re-anchor is the only unpin path and it is unbounded. A wallet in MovingFunds with reservations that has proven its funds moved (so `movingFundsRequestedAt == 0`) is stuck: it cannot close (blocked by `walletReservationsCount`), it cannot be timed out (blocked by the completion sentinel), and its only exit is re-anchor or stranding. Is this the intended m1 behavior, or should `beginWalletClosing` allow closing with active reservations in m1 (since there is no dissolution to lose)?

3. **DECISION NEEDED: The `ReservedRedemptionProposal` and `ReservationDissolutionProposal` structs (`WalletProposalValidator.sol:930`, `:958`) are m2-only in behavior but are struct definitions in a non-upgradeable contract. Should they be included in the m1 deployment for storage-layout ABI stability, or omitted?** If the contract is redeployed for m2, omission is safe. If it is not, the structs must be present in m1.

4. **DECISION NEEDED: The `IRedemptionWatchtower` interface functions for reserved redemptions (`Redemption.sol:74-108`) are part of the interface declaration in `Redemption.sol`, which is a library imported by `Bridge.sol`. Since `Bridge.sol` is upgradeable, the interface can be extended in m2. But the watchtower contract itself may be deployed at m1. Should the watchtower implement these functions at m1 (as no-ops returning zero/false), or should they be absent until m2?** An absent implementation means `validateReservedRedemptionProposal` would revert on the external call, but that function is m2-only anyway.