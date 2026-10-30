# Governance, third-party and vault-economics stories

Evidence pins and vocabulary: [user stories index](04-user-stories.md).

### Governance

#### US-60 - begin/finalize staging for parameters
- **Actor:** Governance (BridgeGovernance owner).
- **Given/When/Then:** Owner calls `beginReservationParametersUpdate`; then after the governance delay (48h mainnet, 60s Sepolia) the owner calls `finalizeReservationParametersUpdate`, which forwards staged values to `bridge.updateReservationParameters(...)` and clears the staging.
- **Functions:** `tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:1834` (`beginReservationParametersUpdate`), `:1863` (`finalizeReservationParametersUpdate`); staging at `BridgeGovernanceParameters.sol:1640`/`1683`; the underlying `ReservationRouter.updateReservationParameters` (`ReservationRouter.sol:367`, `onlyGovernance`).
- **Events:** `ReservationParametersUpdateStarted` (`tbtc-v2:solidity/contracts/bridge/BridgeGovernanceParameters.sol:1664`); `ReservationParametersUpdated` (`Reservation.sol:1462`).
- **Reverts (router):** "Caller is not the governance" (`node_modules/@keep-network/random-beacon/contracts/Governable.sol:36`, applied at `ReservationRouter.sol:377`).
- **Reverts (staging):** "Change not initiated" (`BridgeGovernanceParameters.sol:385`); "Governance delay has not elapsed" `:388`.
- **Reverts (parameter set):** See PAR- table in `03-parameters-governance.md`: PAR-1 min>fee, PAR-2 fee>0, PAR-3 term bounds, PAR-7 timeout>margin, PAR-6 count>0, PAR-9 vault-change gates ("Max active reservations must be greater than zero" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1406`, "Active reservations exist" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1411`, "Pending reserved deposits exist" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1415`), feasibility (D25) via `BridgeState.sol:695-710`; "Amount cap exceeds slot capacity" `Reservation.sol:1332`.
- **Test:** `test/bridge/Bridge.Governance.test.ts:4428` (`beginReservationParametersUpdate`), `:4505`, `:4536`, `:4556` (`finalizeReservationParametersUpdate`), `:4660`, `:4687`; `test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:170`, `:192`, `:207`, `:219`; `test/bridge/Bridge.ReservationCaps.test.ts:95`; `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:415`, `:432`. `test/bridge/Bridge.ReservationCaps.test.ts:221` ("Active reservations exist"), `:205-213` ("Max active reservations must be greater than zero"), `test/bridge/Bridge.ReservationSettlement.test.ts:1056` ("Pending reserved deposits exist"), `test/bridge/Bridge.ReservationReleaseToPool.test.ts:465` ("Pending reserved deposits exist"). All three vault-switch reverts have direct tests. Status `holds`.

#### US-61 - begin/finalize staging for caps
- **Actor:** Governance.
- **Given/When/Then:** Same as US-60 but for caps via `beginReservationCapsUpdate` / `finalizeReservationCapsUpdate`. `maxActiveReservations > 0` is the M1 launch gate; D25 feasibility applies while a vault is configured. At `9d2086965` the ceremony staged four arguments. At `tbtc-v2@806b1106f` (D31) it stages five: the fifth is `handoffReserveWallets` (the hand-off reserve R of the acceptance gate in US-05), and a zero reserve is rejected.
- **Functions (original pin `9d2086965`, four arguments):** `tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:1893` (`beginReservationCapsUpdate`), `:1912` (`finalizeReservationCapsUpdate`); `ReservationRouter.updateReservationCaps` (`ReservationRouter.sol:450`, `onlyGovernance`).
- **Functions (D31, five arguments, at `806b1106f`):** `beginReservationCapsUpdate` `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896` (fifth argument at `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1901`); `finalizeReservationCapsUpdate` `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1917`, which forwards `staged.newHandoffReserveWallets` at `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1926`; staged struct field `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernanceParameters.sol:126`; router entry `ReservationRouter.updateReservationCaps(uint64,uint64,uint32,uint32,uint32)` `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:476` (`onlyGovernance`); library setter `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1687-1745`, storing the reserve at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1736`. Selectors at `806b1106f`: begin `0x4c1cda81` (was `0x3684e886`), setter `0x78d76eda` (was `0xf858967d`), finalize `0x7d87eff2` (unchanged). The `reservationCaps()` view stays four values; the reserve is read with `reservationHandoffReserveWallets()` (`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:519`).
- **Events (D31):** `ReservationCapsUpdateStarted` carries the reserve (`tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernanceParameters.sol:375`); `ReservationCapsUpdated` has five fields, topic0 `0x964ef9bd68fafce7168d52b1d95f0da86e3be014f9843a793b021652e4e28daa` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:402-408`); the four-field topic at `9d2086965` was `0x11af1b44d097c7893ef437ea940dea2e21b5c822807b98a3c4d4d9f7f0f11e5d`.
- **Reverts:** "Active reservations cap must be greater than zero" (`Reservation.sol:1503`, original pin); "Amount cap exceeds slot capacity" `:1332` (original pin); feasibility guards. D31 adds "Hand-off reserve must be greater than zero" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`. The setter enforces the zero-reserve rule only when it runs: the storage field reads 0 on an upgraded Bridge until the first five-argument caps update (the gate then holds back no wallet and admits up to `L * N` positions). Deploy scripts set it before activation (`tbtc-v2@806b1106f:solidity/deploy/98_generate_reservation_mainnet_calldata.ts:163-178`; `RESERVATION_HANDOFF_RESERVE_WALLETS` rejected when <= 0 at `tbtc-v2@806b1106f:solidity/deploy/98_generate_reservation_mainnet_calldata.ts:506-523`, test `tbtc-v2@806b1106f:solidity/test/deploy/98_generate_reservation_mainnet_calldata.test.ts:486`).
- **Test (original pin):** `test/bridge/Bridge.Governance.test.ts:4705`, `:4785`, `:4862`, `:4875`; `test/bridge/Bridge.ReservationCaps.test.ts:153`, `:121`, `:132`, `:224`, `:252`; `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:415`, `:432`; `test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:235`, `:249`, `:264`. Status `holds`.
- **Test (D31, at `806b1106f`):** five-argument begin `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4705`; finalize `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4789`; zero reserve rejected `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationCaps.test.ts:163`; selectors `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts:36` and event topics `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts:57`. Status `holds` at `806b1106f`. Line numbers in the original-pin list above resolve at `9d2086965` and are kept as they were; the same test names moved at `806b1106f`. Nothing is deployed on Sepolia or mainnet.

#### US-62 - setVaultStatus (vault activation / deactivation, C-4 gate)
- **Actor:** Governance.
- **Given/When/Then:** Governance sets vault trust after operator readiness and block B as an operational requirement. It can later revoke trust at any occupancy level. Acceptance settlement then directly credits Bank balances without this vault's fee/mint callback. The flag is reversible; accepted positions and vault-change restrictions are not erased by changing it.
- **Functions:** `BridgeGovernance.setVaultStatus` (`tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:308`, `onlyOwner`); `Bridge.setVaultStatus` (`Bridge.sol:1288`, `onlyGovernance`); the direct-credit branch in `ReservationProofs.settleAcceptance:621-625`.
- **Events:** `VaultStatusUpdated(vault, isTrusted)` at `Bridge.sol:1293`.
- **Reverts:** "Ownable: caller is not the owner" (`node_modules/@openzeppelin/contracts/access/Ownable.sol:51`); "Caller is not the governance" for direct `Bridge.setVaultStatus` callers.
- **Test:** `test/bridge/Bridge.Governance.test.ts:4889` ("setVaultStatus"); `test/bridge/ReservationProofs.test.ts:1757` ("should credit the deposit's immutable vault, not a reservationVault repointed mid-flight") is the untrusted-vault fallback. The C-4 ordering gate is documented and logged by the deploy scripts (`deploy/97_set_reservation_parameters.ts`), not enforced by an on-chain check; status `holds-untested` for the C-4 ordering (operational gate). The known gap at `ReservationVault.sol:147-160` (vault trusts every Bank-routed credit unconditionally) is recorded in Findings.
- **Status:** `holds` for the vault-status update and the untrusted-vault direct-credit fallback; `holds-untested` for the C-4 ordering gate (operational, deploy-script enforced, no on-chain check).

#### US-63 - setReservationRouter (one-shot)
- **Actor:** Governance.
- **Given/When/Then:** Owner calls `BridgeGovernance.setReservationRouter`; the underlying `Bridge.setReservationRouter` calls `BridgeState.setReservationRouter` once. Subsequent calls revert. Setting the zero address or an EOA also reverts.
- **Functions:** `BridgeGovernance.setReservationRouter` (`tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:1811`); `Bridge.setReservationRouter` (`Bridge.sol:2088`); `BridgeState.setReservationRouter` (`BridgeState.sol:1165-1186`).
- **Reverts:** "Reservation router already set" `tbtc-v2:solidity/contracts/bridge/BridgeState.sol:1171`; "Reservation router address must not be 0x0" `tbtc-v2:solidity/contracts/bridge/BridgeState.sol:1176`; "Reservation router must be a contract" `tbtc-v2:solidity/contracts/bridge/BridgeState.sol:1181`; governance modifiers for the wrapper.
- **Test:** `test/bridge/Bridge.Parameters.test.ts:2007` (`setReservationRouter` describe block), `:2052`, `:2073`, `:2095`, `:2108`. Status `holds`.

#### US-64 - Revert when finalize runs before the delay elapses or the change is uninitialized
- **Actor:** Governance.
- **Given/When/Then:** Owner calls `finalizeReservationParametersUpdate` or `finalizeReservationCapsUpdate` before the staged `changeInitiated` timestamp + `governanceDelay` window, or without ever calling `begin*`. **Then** the staging library reverts.
- **Functions:** `BridgeGovernanceParameters.onlyAfterGovernanceDelay` (`tbtc-v2:solidity/contracts/bridge/BridgeGovernanceParameters.sol:380-392`).
- **Reverts:** "Change not initiated" `tbtc-v2:solidity/contracts/bridge/BridgeGovernanceParameters.sol:385`; "Governance delay has not elapsed" `tbtc-v2:solidity/contracts/bridge/BridgeGovernanceParameters.sol:388`.
- **Test:** `test/bridge/Bridge.Governance.test.ts` "context" blocks (e.g. `finalizeReservationCapsUpdate` "when the update process is not initialized" / "when the governance delay has not passed"); `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:432` ("reverts finalize before the governance delay elapses"). Status `holds`.

#### US-65 - forceStaleReservedDeposit (governance override)
- **Actor:** Governance.
- **Given/When/Then:** Governance calls `BridgeGovernance.forceStaleReservedDeposit`, which is `onlyOwner` and is NOT a staged begin/finalize setter: there is no governance delay, and the call takes effect in the transaction that makes it. This is deliberate incident response for a depositor who revealed a reserved deposit with a far-future locktime and never funded the anchor, and it matches the permissionless `notifyStaleReservedDeposit`, which also has no delay (`tbtc-v2@c8350d109:solidity/contracts/bridge/BridgeGovernance.sol:1939-1940`: the note reads "not have a governance delay, matching `notifyStaleReservedDeposit`'s" and continues with own no-delay, incident-response nature). The underlying call clears the pending marker regardless of the deadline, provided the deposit is not swept and no acceptance is pending.
- **Functions:** `BridgeGovernance.forceStaleReservedDeposit` (`tbtc-v2@c8350d109:solidity/contracts/bridge/BridgeGovernance.sol:1941`, `onlyOwner`, no delay); `ReservationRouter.forceStaleReservedDeposit` (`ReservationRouter.sol:410`, `onlyGovernance`); `Reservation.forceStaleReservedDeposit` (`Reservation.sol:1285`).
- **Reverts:** governance modifier; "Not a pending reserved deposit" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1293`; "Deposit already swept" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1300`; "Acceptance authorization pending" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1306`.
- **Test:** `Bridge.ReservationStrandingLibrary.test.ts:966-1015` (`describe("forceStaleReservedDeposit guards")`, "Acceptance authorization pending" at `:1014`), `test/audit-poc/ForceStaleReservedDeposit.t.sol`. The negative guards, the counter and the event are tested. Status `holds`.

#### US-66 - updateDepositParameters feasibility guard (D25)
- **Actor:** Governance.
- **Given/When/Then:** Governance updates a deposit parameter (`depositDustThreshold`, `depositTreasuryFeeDivisor`, `depositTxMaxFee`, `depositRevealAheadPeriod`). While a reservation vault is configured, the new values must still allow at least one reserved deposit; otherwise the call reverts.
- **Functions:** `BridgeState.updateDepositParameters` (`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:614-665`) calling `assertReservedDepositsFeasible` (`:681-712`).
- **Reverts:** "Reveal ahead period leaves no reserved deposit possible" `tbtc-v2:solidity/contracts/bridge/BridgeState.sol:695`; "Single-reservation cap below dust threshold" `tbtc-v2:solidity/contracts/bridge/BridgeState.sol:700`; "Per-wallet reservation cap below dust threshold" `tbtc-v2:solidity/contracts/bridge/BridgeState.sol:705`; "Total reservation cap below dust threshold" `tbtc-v2:solidity/contracts/bridge/BridgeState.sol:710`.
- **Test:** `test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:281` ("reverts when the new dust threshold exceeds the single-amount cap"), `:291`, `:304`, `:319`, `:338`. The "before vault is set" accept path is exercised at `:134`, `:149`. Status `holds`.

#### US-67 - Privileged re-anchor of a Live-wallet position by governance
See US-30. The status of "Only governance can rotate a Live wallet's anchor" is `holds` (US-30 tests cover the Live-gate reverts). Status `holds`.

#### US-68 - Vault fee and reserve management
- **Actor:** Vault owner (governance as `Ownable` of `ReservationVault`).
- **Given/When/Then:** Owner updates `initiationFeeBps` (up to 500 bps), `feeReserveTarget`, and calls `sweepFees(recipient)` to send the TBTC balance above the reserve target to the recipient (treasury). `sweepFees` first repays outstanding in-kind fee debt from the vault's current TBTC balance.
- **Functions:** `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:335` (`updateInitiationFee`), `:278` (`updateFeeReserveTarget`), `:302` (`sweepFees`); `_burnFromReserve` (`:359`).
- **Events:** `FeesUpdated` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:343`); `FeeReserveTargetUpdated` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:283`); `FeesSwept` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:323`); `InKindFeeDebtRepaid` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:311`, `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:265`).
- **Reverts:** "Ownable: caller is not the owner" (`node_modules/@openzeppelin/contracts/access/Ownable.sol:51`); "Fee exceeds the maximum" `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:338`; "Recipient must not be zero" `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:303`.
- **Test:** `test/vault/ReservationVault.test.ts:796` (non-owner), `:802` (happy), `:812` (max), `:551` (non-owner for `updateFeeReserveTarget`), `:557`, `:566`; `:588` (sweep non-owner), `:594` (zero recipient), `:600` (no-op under target), `:646`, `:657`, `:703`, `:715`, `:766`, `:776`. Status `holds`.

#### US-69 - Debt repayment and in-kind fee financing
- **Actor:** Anyone (TBTC holder / treasury / governance) repays in-kind fee debt; the Bridge (via the SPV re-anchor proof path) calls `financeInKindFee` on settlement.
- **Given/When/Then:** A `Pending` re-anchor has a non-zero `minerFee`. The Bridge calls `vault.financeInKindFee(feeSat)`. The vault burns TBTC up to the coverable portion from its current balance and records the shortfall in `inKindFeeDebtSat`; settlement does not fail. `repayInKindFeeDebt` lets anyone burn their own TBTC against the recorded debt (capped at the outstanding amount).
- **Functions:** `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:215` (`financeInKindFee`), `:241` (`repayInKindFeeDebt`), `:359` (`_burnFromReserve`); called from `ReservationProofs.submitReservationReanchorProof` at `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:771-774`.
- **Events:** `InKindFeeFinanced(feeSat, shortfallSat)` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:233`); `InKindFeeDebtRepaid` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:265`, `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:311`).
- **Reverts:** "Caller is not the Bridge" `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:216`; "No debt to repay" `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:242`; "Amount must not be zero" `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:243`.
- **Test:** `test/vault/ReservationVault.test.ts:275` (non-Bridge), `:291` (zero-fee no-op), `:335` (exact-cover burn), `:342`, `:347`, `:373`, `:379`, `:384`, `:427`, `:436`, `:441`, `:451` (no-debt revert), `:494` (zero-amount revert), `:500`, `:512`, `:519`, `:531`, `:536`. End-to-end through the re-anchor proof: `test/bridge/Bridge.ReservationSettlement.test.ts:500` ("bounds the grind at a request-time amount floor..."), `:573-580`. Status `holds`.

### Third party

#### US-70 - Third-party re-anchor request within hold cap (positive)
- **Actor:** Third party.
- **Given/When/Then:** Source wallet is `MovingFunds` or `Closing`. The third party calls `requestReservationReanchor(target, [], 0)`; the target's `pendingThirdPartyReanchorHolds < maxThirdPartyReanchorHoldsPerWallet`. **Then** the request succeeds with `thirdPartyHold = true`.
- **Functions:** `Reservation.requestReservationReanchor` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:866-876`).
- **Test:** `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts` (the suite's setup and the `pendingThirdPartyReanchorHolds` count assertions indirectly cover under-cap requests). Status `holds`.

#### US-71 - Hold cap exhausted (negative)
- **Actor:** Third party.
- **Given/When/Then:** As US-70 but the cap is reached.
- **Reverts:** "Target third-party re-anchor holds exceeded" (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:871`).
- **Test:** `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:133`. Status `holds`.

#### US-72 - Hold cap zero (negative)
- **Actor:** Third party.
- **Given/When/Then:** Governance has set `maxThirdPartyReanchorHoldsPerWallet = 0`. Any non-privileged request reverts.
- **Reverts:** "Target third-party re-anchor holds exceeded" `tbtc-v2:solidity/contracts/bridge/Reservation.sol:871` (every third-party request hits this immediately).
- **Test:** `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:250`. Status `holds`.

#### US-73 - Privileged caller exemption (positive)
- **Actor:** Reservation owner, SPV maintainer, or source-wallet member with valid member proof.
- **Given/When/Then:** As US-70 but the caller is privileged per `Reservation.sol:786-790`. The request exempts the third-party hold counter (`isPrivileged = true` at `:786`); the target's count and amount are reserved as for any other re-anchor, but the third-party hold counter is not touched.
- **Reverts:** Any caller that supplies a non-empty invalid member proof reverts, including an owner or maintainer. Their role does not excuse an invalid supplied proof.
- **Test:** `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:151` (owner), `:179` (SPV maintainer), `:204` (member with valid proof). Status `holds`.
- **D31 update (at `tbtc-v2@806b1106f`):** for a privileged caller whose source wallet is not Live, the request also skips the target count and amount caps (`skipTargetCaps` at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`), so the sentence "the target's count and amount are reserved as for any other re-anchor" above holds only for the accounting, not for the cap checks: the counters are still incremented (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:934-935`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:949-950`) but the caps are not enforced. Third parties and Live-source governance rotations keep the caps. Details and tests: US-23 and US-31 in [wallet stories](04-wallet-spv-stories.md). Test at `806b1106f`: owner `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:427`, SPV maintainer `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:443`, proven member `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:452`. Status `holds` at `806b1106f`.

#### US-74 - Member proof failure (negative)
- **Actor:** Caller passing a non-empty member proof.
- **Given/When/Then:** The caller supplies `walletMembersIDs` and `walletMemberIndex` but `isWalletMember(...)` returns false.
- **Reverts:** "Caller is not a member of the source wallet" (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:783`).
- **Test:** `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:235`; `keep-core:pkg/tbtcpg/reservation_reanchor_member_proof_test.go:79` ("TestReservationReanchorTask_NonMemberOperator_SendsNoRequest" - the operator returns before sending a request). Status `holds`.

#### US-75 - Hold release on timeout / supersede / on-time settlement
- **Actor:** Anyone.
- **Given/When/Then:** A pending third-party re-anchor's hold is released when (a) `notifyReservationActionTimeout` resolves the generation (`releaseThirdPartyReanchorHold` at `Reservation.sol:975-979`), (b) the generation is unwound on a later-proof supersede (`ReservationProofs.unwindPendingAction:857-861`), or (c) the re-anchor proof settles on time (`ReservationProofs.submitReservationReanchorProof:741-747`).
- **Functions:** `Reservation.releaseThirdPartyReanchorHold` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1072-1085`).
- **Reverts:** A counted release with no hold reverts with "No third-party re-anchor hold to release". The action's terminal-state guards prevent duplicate public cleanup; checked decrement prevents wrapping.
- **Test:** `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:271` ("drops on action timeout"), `:288` ("drops on supersede-unwind of the pending generation"), `:303` ("drops on on-time re-anchor settlement, so a third party can request again"), `:397` ("the member request succeeds after the third-party hold times out"). Status `holds`.

#### US-76 - Permissionless notify (acceptance timeout, action timeout, stale deposit, stranding)
All four notify entry points are permissionless; see US-06, US-10, US-32, US-13. No additional story.
- **Status:** see US-06, US-10, US-32, US-13.

### TBTC holder / vault economics

#### US-80 - Holder receives net of fee, fee retained (positive)
- See US-11. The TBTC holder (the depositor) receives `anchorAmount - initiationFeeBps * anchorAmount / 10000`; the fee is retained in the vault as the in-kind fee reserve. Status `holds`.

#### US-81 - Sweep excess to treasury (positive)
- See US-68. The treasury (or any recipient) receives TBTC when the vault's balance exceeds `feeReserveTarget` after debt repayment. Status `holds`.

#### US-82 - Anyone repays in-kind fee debt (positive)
- See US-69. The TBTC holder pulls TBTC from their own balance, transfers to the vault, and the vault burns it. The Bank balance decrements. Status `holds`.

#### US-83 - balance approval always reverts (negative)
- **Actor:** Anyone.
- **Given/When/Then:** A caller invokes `vault.receiveBalanceApproval(depositor, amount, data)`.
- **Reverts:** "Balance approvals not supported" (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:353`).
- **Test:** `test/vault/ReservationVault.test.ts:830`. Status `holds`.
