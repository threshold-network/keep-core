# Depositor, acceptance and stranding stories

Evidence pins and vocabulary: [user stories index](04-user-stories.md).

### Depositor / reservation owner

#### US-01 - Reserved deposit reveal
- **Actor:** Depositor.
- **Given/When/Then:** `Bridge.revealDeposit` is called with `reveal.vault == self.reservationVault`; the funding-output script hash matches the script template; `block.timestamp + depositRevealAheadPeriod <= refundLocktime <= block.timestamp + reservationTermSeconds + DEPOSIT_REFUND_SAFETY_MARGIN (24h)`; the custodying wallet is `Live` and the vault is trusted. **Then** a `PendingReservedDeposit` is stored (`pendingReservedDeposits += 1`) and `DepositRevealed` is emitted with the `vault` field set to the reservation vault.
- **Functions:** `Bridge.revealDeposit` -> `Deposit._revealDeposit` (`tbtc-v2:solidity/contracts/bridge/Deposit.sol:198`), reserved routing at `:215-253` and `:387-396`.
- **Events:** `DepositRevealed`; `vault` is a non-indexed data field, while depositor and wallet hash are indexed (`tbtc-v2:solidity/contracts/bridge/Bridge.sol:77-87`).
- **Reverts (negatives):** "Wallet must be in Live state" `tbtc-v2:solidity/contracts/bridge/Deposit.sol:207`; "Vault is not trusted" `tbtc-v2:solidity/contracts/bridge/Deposit.sol:212`; "Wrong 20-byte script hash" `tbtc-v2:solidity/contracts/bridge/Deposit.sol:338` / "Wrong 32-byte script hash" `tbtc-v2:solidity/contracts/bridge/Deposit.sol:345` / "Wrong script hash length" `tbtc-v2:solidity/contracts/bridge/Deposit.sol:348`; "Deposit already revealed" `tbtc-v2:solidity/contracts/bridge/Deposit.sol:367`; "Refund locktime too far in the future for a reservation" `tbtc-v2:solidity/contracts/bridge/Deposit.sol:250`; "Deposit amount too small" `tbtc-v2:solidity/contracts/bridge/Deposit.sol:373`; P2SH script hash mismatch at `tbtc-v2:solidity/contracts/bridge/Deposit.sol:334`. The treasury rebate is intentionally skipped for reserved deposits (`tbtc-v2:solidity/contracts/bridge/Deposit.sol:407-409`).
- **Test:** `test/bridge/Bridge.ReservationAcceptanceAuthorization.test.ts:419` (test helper that reveals via the production path); `test/bridge/Bridge.Deposit.test.ts` for the underlying script-validation paths. Status `holds`.

#### US-02 - Reserved reveal rejected: refund locktime beyond term + safety margin
- **Actor:** Depositor.
- **Given/When/Then:** Same as US-01 except `refundLocktime` is greater than `block.timestamp + reservationTermSeconds + 24h`. **Then** the reveal reverts.
- **Functions:** `Deposit._revealDeposit` (`tbtc-v2:solidity/contracts/bridge/Deposit.sol:243-253`).
- **Reverts:** "Refund locktime too far in the future for a reservation" `tbtc-v2:solidity/contracts/bridge/Deposit.sol:250`.
- **Test:** `none` (no test in `test/bridge/Bridge.Deposit.test.ts` asserts this string for a reserved deposit; the case is covered indirectly by the feasibility guard tests for the `reservationTermSeconds` setter). Status `holds-untested`.

#### US-03 - Acceptance request: happy path
- **Actor:** Depositor.
- **Given/When/Then:** A reserved deposit is revealed, the custodying wallet is `Live`, the deposit amount is above `reservationMinAmount + 2 * reservationTxMaxFee`, the action-timeout window is non-empty, and the authorization window does not overlap the deposit refund window. The depositor calls `requestReservationAcceptance(fundingTx, fundingProof, fundingOutputIndex, walletPubKeyHash)`, carrying the deposit's funding transaction and its SPV proof. **Then** the router derives the reservation key from the proven transaction hash and output index, the reservation is created, per-wallet and global capacity is reserved, the action record at the new nonce is `Pending`, and `ReservationAcceptanceRequested` is emitted.
- **Functions:** `ReservationRouter.requestReservationAcceptance` (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/ReservationRouter.sol:230-247`; `BitcoinTx.validateProof` at :241, key derivation at :242-246) -> `Reservation.requestReservationAcceptance` (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:513-664`), `reserveAcceptanceCapacity` (`:1013`).
- **Evidence limit (VF-12):** at `tbtc-v2@9d2086965` the same entry point took `(reservationKey, walletPubKeyHash)` and proved nothing, and `revealDeposit` still checks no proof. The proof-bearing form above is the published one; the earlier signature and its citations are kept as history, not as current guidance.
- **SDK side:** the SDK assembles the proof from its Bitcoin client with the difficulty the Bridge requires; see US-97.
- **Events:** `ReservationAcceptanceRequested(reservationKey, requestNonce, walletPubKeyHash, depositAmount, txMaxFee, timeoutAt)` at `tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:656`; `ReservationOccupancyChanged(activeReservationsCount)` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1009`.
- **Test:** `tbtc-v2@86ce2e79b:solidity/test/bridge/Reservation.test.ts:2039` ("should request reservation acceptance successfully"). Status `holds` at `tbtc-v2@86ce2e79b` for the four-argument form; the two-argument form recorded before VF-12 does not match the published code.

#### US-04 - Acceptance request: depositor and revealed-deposit gates (negative)
- **Actor:** Non-depositor caller or unrevealed/unswept/non-reserved/vault-mismatched deposit.
- **Given/When/Then:** Caller is not `deposit.depositor` (any of the depositor gates below).
- **Functions:** `Reservation.requestReservationAcceptance` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:499-650`).
- **Reverts (each is one story instance; the table maps guard -> revert string -> test):**

| Guard | Revert string | Test |
|---|---|---|
| Vault not set | "Reservations are disabled" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:506` | `test/bridge/Reservation.test.ts:146` |
| Deposit not revealed | "Deposit not revealed" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:510` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:157` (guard 2) |
| Caller is not depositor | "Caller is not the deposit's depositor" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:513` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:178` |
| Deposit already swept | "Deposit already swept" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:515` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:202` |
| Not flagged as reserved | "Deposit was not revealed as reserved" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:518` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:222` |
| `deposit.vault != reservationVault` | "Deposit not routed to the reservation vault" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:522` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:243` |
| `reservations[key].state != Unknown` | "Reservation already exists" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:533` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:280`, `tbtc-v2:solidity/test/bridge/Reservation.test.ts:294` |
| Current generation `Pending` | "Acceptance already pending" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:538` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:312` |
| `reservedDeposit.walletPubKeyHash != walletPubKeyHash` | "Wallet is not the deposit's designated wallet" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:548` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:264` |
| Wallet not `Live` | "Wallet must be in Live state" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:554` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:326`, `tbtc-v2:solidity/test/bridge/Reservation.test.ts:338`, `tbtc-v2:solidity/test/bridge/Reservation.test.ts:350`, `tbtc-v2:solidity/test/bridge/Reservation.test.ts:362` |
| `deposit.amount <= min + 2 * txMaxFee` | "Deposit amount too small for a reservation" `tbtc-v2@dc5c2ed71:solidity/contracts/bridge/Reservation.sol:583` | `tbtc-v2@dc5c2ed71:solidity/test/bridge/Reservation.test.ts:369` (guard 11) |
| No signing window | "Acceptance authorization has no signing window" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:586` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:483` |
| Authorization window overlaps refund window | "Authorization window would overlap the deposit refund window" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:598` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts:433`, `tbtc-v2:solidity/test/bridge/Reservation.test.ts:505`, `tbtc-v2:solidity/test/bridge/Reservation.test.ts:526` |
| Refund window allows no signing | "Deposit refund deadline allows no signing window" `tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:639-646` | (covered by guards 12-14 `tbtc-v2:solidity/test/bridge/Reservation.test.ts:404-528`); no input reaches this branch because the preceding guards dominate it; documentation-grade assertion, not a tested control |

- **Status:** `holds` for every row above.

#### US-05 - Acceptance request: capacity and amount-cap gates (negative)
- **Actor:** Depositor.
- **Given/When/Then:** The depositor passes all US-04 guards but the request would push the on-chain counters beyond a cap.
- **Functions:** `Reservation.requestReservationAcceptance` -> `reserveAcceptanceCapacity` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:999-1037`) and the request-time cap check at `:610-614`.
- **Reverts:**

| Cap | Revert string | Test |
|---|---|---|
| Single-position amount | "Reservation exceeds the single-reservation cap" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:613` | `test/bridge/Reservation.test.ts:535` |
| Global occupancy | "Active reservations cap exceeded" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1006` | `test/bridge/Reservation.test.ts:746` |
| Hand-off reserve (gate added at `806b1106f`; absent at `9d2086965`) | "Hand-off reserve exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076` | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322`, `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:346`, `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379` |
| Total reserved amount | "Total reserved amount cap exceeded" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1015` | `test/bridge/Reservation.test.ts:592` |
| Per-wallet count | "Wallet reservations cap exceeded" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1024` | `test/bridge/Reservation.test.ts` "guards 15-18: caps validation" |
| Per-wallet amount | "Wallet reserved amount cap exceeded" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1034` | `test/bridge/Reservation.test.ts:666` |

- **Status:** `holds` for every row.
- **Hand-off reserve gate (D31, design A, at `tbtc-v2@806b1106f`):** `reserveAcceptanceCapacity` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1052-1077`) checks the global cap first ("Active reservations cap exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1059`), increments `activeReservationsCount` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1061`), then requires `activeReservationsCount <= max(0, liveWalletsCount - handoffReserveWallets) * maxReservationsPerWallet` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1077`). The total-amount, per-wallet count and per-wallet amount checks follow the gate. The only caller is `requestReservationAcceptance` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:643`). The gate runs only at the acceptance request. It does not run at proof time, at re-anchor, or on settlement, and it does not guarantee a Live target, a signing quorum, proof delivery, amount headroom or fee room. The three tests in the table row pin the revert past `(L - R) * N` with max-active room (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322`), the budget tightening when a Live wallet leaves Live (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:346`) and the full block while `L <= R` (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379`). Status `holds` at `806b1106f`. At `9d2086965` the gate did not exist and the other rows above carry the original-pin citations. Nothing is deployed on Sepolia or mainnet.

#### US-06 - Acceptance timeout and re-authorization
- **Actor:** SPV watcher (any address may call; see SM- for keep-core wiring).
- **Given/When/Then:** A `Pending` acceptance generation at the reservation's current nonce has reached its `timeoutAt`. The watcher calls `notifyReservationAcceptanceTimedOut(reservationKey)`. **Then** the action becomes `TimedOut`, the wallet, global, and per-wallet counters are released, `activeReservationsCount` decrements, and `ReservationAcceptanceTimedOut` is emitted. The depositor can submit a fresh `requestReservationAcceptance` for the same deposit because the position is back at `Unknown`.
- **Functions:** `ReservationRouter.notifyReservationAcceptanceTimedOut` (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:334`) -> `Reservation.notifyReservationAcceptanceTimedOut` (`Reservation.sol:667-703`), `releaseAcceptanceCapacity` (`:1041`).
- **Events:** `ReservationAcceptanceTimedOut(reservationKey, requestNonce)` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:702`; `ReservationOccupancyChanged(activeReservationsCount)` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1050`.
- **Test:** `test/bridge/Reservation.test.ts:1492` ("should time out a pending acceptance and release its capacity"); `test/bridge/Bridge.ReservationOccupancy.test.ts:68`; `test/bridge/Bridge.ReservationAcceptanceAuthorization.test.ts:604` ("releases capacity, allows re-authorization, and still settles the late anchor"). Status `holds`.
- **Negatives (also exercisable on this path):** "Action type mismatch" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:683`; "Action is not pending" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:685`; "Action has not timed out" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:689`. Tests: `test/bridge/Reservation.test.ts:1468` covers "Action type mismatch" on the action-loader path. The other two are enforced by the same guards in `notifyReservationAcceptanceTimedOut`; no dedicated test asserts them at the library entry. Status `holds-untested` for the per-guidance tests, `holds` for the type-mismatch path.

#### US-07 - Late acceptance settlement by SPV maintainer
- **Actor:** SPV maintainer.
- **Given/When/Then:** The deposit's anchor transaction is on Bitcoin with sufficient confirmations and the generation is `Pending` or `TimedOut`. The SPV maintainer calls `submitReservationAcceptanceProof(txInfo, proof, reservationKey, requestNonce)`. **Then** the anchor is consumed (`deposit.sweptAt = block.timestamp`), the reservation is created at `expiresAt = block.timestamp + termSeconds` and `mintedAmount = anchorAmount`, the bank balance is credited via the reservation vault (or directly if the vault is untrusted; see US-09), and `ReservationAccepted` is emitted.
- **Functions:** `ReservationRouter.submitReservationAcceptanceProof` (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:263`) -> `ReservationProofs.submitReservationProof` (`:137`) -> `submitReservationAcceptanceProof` (`:346-392`), `consumeAcceptedDeposit` (`:404`), `validateAnchorOutput` (`:441`), `settleAcceptance` (`:473`), and `Bank.increaseBalanceAndCall` to the vault at `:622`.
- **Events:** `ReservationAccepted(reservationKey, requestNonce, targetWalletPubKeyHash, owner, anchorTxHash, anchorAmount, expiresAt)` at `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:576`; `ReservationOccupancyChanged(activeReservationsCount)` at `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:534` (late) or implicit on the regular path.
- **Test:** `test/bridge/ReservationProofs.test.ts:555` ("should successfully execute end-to-end acceptance settlement"); `test/bridge/ReservationProofs.test.ts:1528` ("should settle through submitReservationProof with a real, valid SPV proof"). Status `holds`.

#### US-08 - Late acceptance: window expired (negative)
- **Actor:** SPV maintainer.
- **Given/When/Then:** A timed-out acceptance generation is proved after `block.timestamp > action.timeoutAt + action.termSeconds`.
- **Functions:** `ReservationProofs.loadSettleableAction` (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:204-211`).
- **Reverts:** "Late acceptance settlement window expired" `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:210`.
- **Test:** `tbtc-v2@86ce2e79b:solidity/test/bridge/ReservationProofs.test.ts:446` ("should reject a late acceptance one second after the window closes", reverting "Late acceptance settlement window expired"), with the inclusive end of the window pinned by `:434` ("should still load a late acceptance at the last second of the window") and `:459` ("should not apply the window to a late re-anchor") showing the bound is acceptance-only. Status `holds`.

#### US-09 - Superseding acceptance: late settlement of an older generation unwinds a newer pending generation
- **Actor:** SPV maintainer.
- **Given/When/Then:** Generation `n` is `TimedOut` and a fresh generation `n+1` is `Pending`; the older generation's anchor transaction later confirms. The SPV maintainer calls `submitReservationAcceptanceProof` for generation `n`. **Then** the late-settlement path in `settleAcceptance` re-takes the capacity that `notifyReservationAcceptanceTimedOut` released, unwinds the pending generation `n+1` (releasing its capacity and emitting `ReservationActionSuperseded`), and the position becomes `Active` against the original anchor.
- **Functions:** `ReservationProofs.settleAcceptance` (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:493-517`) calling `unwindPendingAction` (`:832-877`).
- **Events:** `ReservationAccepted` (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:576`), `ReservationLateSettled` (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:535`), `ReservationActionSuperseded(reservationKey, pendingNonce)` (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:876`).
- **Test:** `test/bridge/ReservationProofs.test.ts:920` ("should unwind pending newer acceptance generation when settling older generation late"). Status `holds`.

#### US-10 - Stale reserved deposit (permissionless) and force stale (governance)
- **Actor:** SPV watcher (or anyone) for `notifyStaleReservedDeposit`; governance (BridgeGovernance) for `forceStaleReservedDeposit`.
- **Given/When/Then:** A revealed reserved deposit whose snapshotted `refundDeadline` has elapsed, the deposit is not swept, and the current action generation is not `Pending`. The watcher calls `notifyStaleReservedDeposit(depositKey)`. **Then** the `pendingReservedDeposit` entry is cleared, `pendingReservedDeposits` decrements, and `ReservedDepositMarkedStale` is emitted. Governance may call `forceStaleReservedDeposit` at any time after the deposit is revealed, even before the deadline (no deadline check).
- **Functions:** `ReservationRouter.notifyStaleReservedDeposit` (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:396`) -> `Reservation.notifyStaleReservedDeposit` (`Reservation.sol:1229`); `ReservationRouter.forceStaleReservedDeposit` (`:410`, `onlyGovernance`) -> `Reservation.forceStaleReservedDeposit` (`:1285`).
- **Events:** `ReservedDepositMarkedStale(depositKey)` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1266` and `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1314`.
- **Reverts (both entry points share the same internal guards; only the deadline check differs):**

| Guard | Revert string | Test |
|---|---|---|
| `pendingReservedDeposit.walletPubKeyHash == 0` | "Not a pending reserved deposit" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1237` / `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1293` | `test/bridge/Bridge.ReservationStranding.test.ts:757`; `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:846` |
| `deposit.sweptAt != 0` | "Deposit already swept" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1247` / `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1300` | `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:865` |
| `block.timestamp <= pendingDeposit.refundDeadline` (only `notifyStaleReservedDeposit`) | "Deposit refund deadline has not elapsed" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1251` | `test/bridge/Bridge.ReservationStranding.test.ts:778`; `tbtc-v2:solidity/test/bridge/Bridge.ReservationStrandingLibrary.test.ts:868` |
| Action generation is `Pending` | "Acceptance authorization pending" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1258` / `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1306` | `test/bridge/Bridge.ReservationStranding.test.ts:830`; `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:910` |

- **Test:** `test/bridge/Bridge.ReservationStranding.test.ts:700`, `:727`, `:748`, `:761`, `:782`, `:803` (full notify-stale coverage); `keep-core:pkg/maintainer/spv/reservation_stale_deposit_watch_test.go:92` ("LiveWalletNotifiedAfterDeadline"), `:179` ("PendingActionNotNotifiedAfterDeadline"), `:969` ("FirstScanStartsAtActivationBlock"). `forceStaleReservedDeposit` has no test that pins its own guards, counter or event; the entry is exercised as setup in `tbtc-v2@86ce2e79b:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts:604-606` (a governance call through the router ABI attached to the Bridge marks the deposit stale, and the following release reverts "Not a pending reserved deposit"). Status `holds` for the permissionless path; `holds-untested` for `forceStaleReservedDeposit` (no test pins the positive or negative guards, the counter or the event at the on-chain entry).

#### US-11 - Mint, fee, and the credit path
- **Actor:** Vault owner (governance) configures fees; depositor is the recipient.
- **Given/When/Then:** On settlement (US-07), the bank credits the gross `anchorAmount` to the reservation vault. The vault mints TBTC gross, charges the `initiationFeeBps` fee, and transfers the net to the depositor; the fee stays in the vault as the in-kind fee reserve. If the reservation vault is not trusted at settlement time, the bank credits the depositor directly and no fee is charged.
- **Functions:** `ReservationProofs.settleAcceptance` credit branch (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:615-625`); `ReservationVault.receiveBalanceIncrease` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:161-197`); default `initiationFeeBps = 40` at `:135`.
- **Events:** `ReservationCreditProcessed(owner, satAmount, feeTbtc)` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:187`).
- **Reverts (vault):** "Caller is not the Bank" `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:103`; "No depositors specified" `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:165`; "Arrays must have the same length" `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:168`.
- **Test:** `test/vault/ReservationVault.test.ts:201` ("should transfer the net amount (gross minus fee) to the depositor"), `:205`, `:209`, `:214`, `:251` (multi-depositor), `:267`. Untrusted-vault fallback: `test/bridge/ReservationProofs.test.ts:1757` ("should credit the deposit's immutable vault, not a reservationVault repointed mid-flight"). The KNOWN GAP at `ReservationVault.sol:147-160` is recorded in Findings. Status `holds`.

#### US-12 - Deposit refund via Bitcoin script
- **Actor:** Depositor.
- **Given/When/Then:** The funding output is still unspent after its Bitcoin refund locktime. The depositor signs the refund branch with its refund key. A separate eligible `notifyStaleReservedDeposit` transaction clears on-chain pending bookkeeping; it does not run automatically. A confirmed acceptance spend removes this funding refund path.
- **Functions:** `Deposit._revealDeposit` script template (`tbtc-v2:solidity/contracts/bridge/Deposit.sol:257-318`); `Reservation.requestReservationAcceptance` refund-overlap guard (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:594-608`); `Reservation.notifyStaleReservedDeposit` (US-10) for the on-chain cleanup.
- **Reverts (on-chain):** "Authorization window would overlap the deposit refund window" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:598` if a future governance change re-pushes the deadline past `timeoutAt + 24h`; "Deposit refund deadline allows no signing window" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:607`.
- **Test:** Request/refund overlap guards are linked at `Reservation.test.ts:433`, `:505` and `:526`. `none` identified for an end-to-end reserved-deposit Bitcoin refund. Status `holds-untested` for that complete path.

#### US-13 - Stranding on a Terminated wallet (MovingFunds timeout, moved-funds-sweep timeout, or fraud defeat)
- **Actor:** Stranding watcher (SPV / keep-core).
- **Given/When/Then:** The custodying wallet becomes `Terminated` (moving-funds timeout, moved-funds-sweep timeout, or `FraudChallengeDefeatTimedOut`). The watcher calls `notifyReservationStranded(reservationKey)`. **Then** the position becomes `Stranded`, capacity is released (`releaseAcceptanceCapacity`), and `ReservationStranded(reservationKey, walletPubKeyHash, owner, anchorAmount)` is emitted. The owner's minted balance is unaffected; the in-kind claim is lost.
- **Functions:** `ReservationRouter.notifyReservationStranded` (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:425`) -> `Reservation.notifyReservationStranded` (`Reservation.sol:1192-1217`), `strandReservation` (`:1091-1130`); `keep-core:pkg/maintainer/spv/reservation_stranding_watch.go:126` (closed/terminated) and `:139` (closing-eligible).
- **Reverts:** "Reservation is not active" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1201`; "Wallet is not terminated, closed, or a dissolution-eligible closing wallet" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1213`.
- **Test:** `test/bridge/Bridge.ReservationStranding.test.ts:525` ("strands an Active reservation on a Terminated wallet: state transition, counter decrement, and event with exact args"); `:554` ("strands an Active reservation on a wallet driven to Terminated by a real moving-funds-timeout call chain, not direct state injection"); `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:357`; `test/bridge/Bridge.ReservationOccupancy.test.ts:110` (Terminated), `:143` (Closed); `keep-core:pkg/maintainer/spv/reservation_stranding_watch_test.go:58` ("NotifiesActiveReservation"), `:445` ("CapturesTerminationCause"). Moved-funds-sweep timeout and fraud-defeat termination use the same `notifyReservationStranded` path; the cause classification is inferred by `keep-core:pkg/chain/ethereum/tbtc.go:1101` (`WalletTerminationCause`) -> `:1168` (`resolveWalletTerminationCause`). The on-chain path is identical to US-13; the cause-specific stranding causes share the same proof. Status `holds` for the on-chain stranding; `holds-untested` for the cause-specific stranding test (no test asserts a `WalletTerminated` event from a `FraudChallengeDefeatTimedOut` or `MovedFundsSweepTimedOut` end-to-end).

#### US-14 - Stranding on a Closing wallet after dissolution eligibility and the closing period

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

- **Actor:** Stranding watcher (or any caller).
- **Given/When/Then:** A reservation's custodying wallet is in `Closing` and **both** `block.timestamp >= reservation.dissolutionEligibleAt` (= `expiresAt + dissolutionDelay`, set at `ReservationProofs.sol:570`) **and** `block.timestamp >= wallet.closingStartedAt + walletClosingPeriod` hold. The watcher calls `notifyReservationStranded`. **Then** the position strands per US-13. Until the closing period elapses the position is not strandable: that period is the owner's exclusive re-anchor window, and `requestReservationReanchor` accepts a Closing source.
- **Functions:** `Reservation.notifyReservationStranded` (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1326-1336`).
- **Reverts:** "Wallet is not terminated, closed, or a dissolution-eligible closing wallet" (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1336`) while either condition is unmet.
- **Tests:** `tbtc-v2@86ce2e79b:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:222` (dissolution-eligible but the closing period still runs: reverts), `:263` (both met: strands) and `:313` (exactly at the end of the closing period: strands).
- **Test:** `keep-core@pkg/maintainer/spv/reservation_wiring_test.go:1352` ("TestStartupStrandingScanChecksDissolutionEligibleClosingWallet") for the client eligibility gate. On-chain both the reject and the accept path are pinned (`tbtc-v2@86ce2e79b:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:186`, `:222`, `:263`, `:313`). Status `holds` at `tbtc-v2@86ce2e79b` for both conditions. Two limits remain: the keep-core watcher checked dissolution eligibility but not the closing period at the head then published, so an early notification reverted and the wallet was rechecked on a later pass (`keep-core@f03cc61dc:pkg/maintainer/spv/reservation_stranding_watch.go:282-291`), and keep-core #4365 closed that gap by holding a `Closing` wallet until `closingStartedAt + walletClosingPeriod` (`keep-core@4ced1c839:pkg/maintainer/spv/reservation_stranding_watch.go:379-392`); and the requirement recorded before VF-10 named dissolution eligibility alone, which does not match the published code.

#### US-15 - Stranding on a closed wallet, mid-stranding race
- **Actor:** Stranding watcher (or any caller).
- **Given/When/Then:** Same as US-13 but the wallet becomes `Closed` (e.g. closing period elapsed). The first call strands; a second call reverts because the position is no longer `Active`.
- **Functions:** `Reservation.notifyReservationStranded` + `strandReservation` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1091-1130`).
- **Reverts:** "Reservation is not active" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1201` for the second call.
- **Test:** `test/bridge/Bridge.ReservationStranding.test.ts:633` ("rejects a second stranding call after a successful strand (state is no longer Active)"); `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:471` (Closed-wallet strand happy). Status `holds`.

#### US-101 - Stranding on a dissolution-eligible Live wallet

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

- **Actor:** The keep-core stranding watcher takes this branch since keep-core #4365: `checkReservationStrandingForLiveWallet` notifies once the reservation is eligible (`keep-core@4ced1c839:pkg/maintainer/spv/reservation_stranding_watch.go:140-147`, gate at `:356-366`), reached from the startup catch-up scan (`pkg/maintainer/spv/reservation_wiring.go:349-441`) and the action-timeout recheck queue (`pkg/maintainer/spv/reservation_action_timeout_watch.go:856-935`). Any caller may also submit `notifyReservationStranded` directly, which is what a `Live` wallet that neither client path reaches relies on.
- **Given/When/Then:** A reservation's custodying wallet is in `Live` and `block.timestamp >= reservation.dissolutionEligibleAt` (= `expiresAt + dissolutionDelay`, set at `ReservationProofs.sol:570`). A caller invokes `notifyReservationStranded`. **Then** the position strands per US-13. Before eligibility the call reverts. M1 has no other release path for an expired position (`closeReservation` has no caller), and only governance can re-anchor from a Live source, so no owner action is preempted; a governance re-anchor that lands first leaves the position `Active` and the strand reverts on the `Active` gate.
- **Functions:** `Reservation.notifyReservationStranded` (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1325-1329`, the `Live && eligible` branch at :1329).
- **Reverts:** "Wallet is not terminated, closed, or a dissolution-eligible closing wallet" (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1336`) before `dissolutionEligibleAt`.
- **Test:** `tbtc-v2@86ce2e79b:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:386` (rejects on a Live wallet before eligibility) and `:397` ("notifyReservationStranded releases an expired position on a Live wallet"). Status `holds` at `tbtc-v2@86ce2e79b` for the contract and at `keep-core@4ced1c839` for the client (`TestA6_LiveWalletStrandsDissolutionEligibleReservation` and `TestA6_LiveWalletDoesNotStrandReservationBeforeDissolutionEligible`, `pkg/maintainer/spv/reservation_stranding_watch_a6_live_test.go`).
