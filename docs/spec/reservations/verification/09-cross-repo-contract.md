# M1 UTXO Reservations - Cross-Repo Contract (XR)

Scope: every assumption that keep-core makes about the tbtc-v2 contracts and
vice versa, at the two pins: `tbtc-v2` at `9d2086965` (branch
`reservations-upgrade`, cited `tbtc-v2:solidity/<path>:<line>`) and
`keep-core` at `f99ef96e3` (branch `reservations-epic`, cited
`keep-core:<path>:<line>`). This file owns the XR- prefix; it may reference
SM-/PAR-/INV-/US-/EC-/FM-/DR-/TM-/AR- by topic but invents no IDs outside XR-.

Status vocabulary: `holds` (both sides cited and a named test pins the
agreement), `holds-untested` (both sides cited, no test that would fail on a
drift found by reading), `violated` (the two sides disagree: exact paths
given), `design-only` (planned, not in either pinned codebase), `unknown`.

ABI rows identify the generated binding, its consumer and the contract entry.
Unqualified citations use the original evidence pins.
D26 additions use contracts at `d20cf74ba` and client at `4b95f0a06`.
The SDK is implemented at `7aabfc502`; see the model and user stories.
D31 additions use contracts at `806b1106f` and client at `d40f6aadc`; neither is deployed.

## Recorded decisions that bind this file (do not reopen)

- **M1 variant B (create-only, minimal vault):** no redemption, renewal, or
  dissolution entry point; the vault is `Ownable`, not upgradeable. This file
  therefore treats the redemption/dissolution `ActionType` values and
  `ActionState` values as storage-stability-only (see XR-S-2).
- **D19 (third-party re-anchor holds):** re-anchor from a MovingFunds/Closing
  source is permissionless; a non-privileged request reserves the target's
  ordinary slot, capped per target by `maxThirdPartyReanchorHoldsPerWallet`;
  the owner, SPV maintainers, proven source-wallet members, and governance are
  exempt. Accepted for testnet; the fact that a third-party hold still consumes
  the target's ordinary slot is an accepted risk to fix before mainnet. D31
  narrows the effect: a privileged hand-off from a retiring source skips the
  target's caps, so a hold no longer blocks it.
- **D25 (feasibility guard):** `assertReservedDepositsFeasible` runs on both
  reservation parameter setters and on `updateDepositParameters` while a vault
  is set; keep-core mirrors it with the `reservation_params_infeasible` gauge.
- **D26:** `releaseReservedDepositToPool` is implemented at tbtc-v2 `d20cf74ba`. It is absent at `9d2086965`. The client binding is absent at `f99ef96e3` and added in `8a714f2c1`; client `4b95f0a06` adds post-release sweep regressions.
- **D27:** the reservation SDK is implemented at `7aabfc502`. A client-side preflight is advisory and cannot reserve on-chain capacity.
- **D31 (privileged hand-off from a retiring source, acceptance reserve gate):**
  a request from the owner, an SPV maintainer, a proven source-wallet member or
  governance, from a source that is not Live, skips the target's per-wallet
  count and amount caps (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`).
  A Live source stays governance-only and keeps both target caps. The signing
  validator no longer re-checks them (XR-F-20). `requestReservationAcceptance`
  also enforces a hand-off reserve gate: active reservations after the
  request must be at most `max(0, liveWalletsCount - handoffReserveWallets) *
  maxReservationsPerWallet` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1077`).
  The caps setter takes the reserve as a fifth argument (XR-F-2), and its view
  is XR-F-23. Implemented and tested at tbtc-v2 `806b1106f` and keep-core
  `d40f6aadc`. Not deployed on Sepolia or mainnet.
- **C-4 gate:** operators must receive the client build before governance activates the vault at or after B. `ReservationsActivationBlock` gates client work (`keep-core:pkg/tbtc/coordination.go:110`). The contracts do not enforce B.

## A. ABI: contract functions and views keep-core calls

Selector = keccak256(signature)[:4] as it appears on-chain and in the committed
Go binding. "Drift status" is against the spec pin `9d2086965` (the surface
the spec pin must match); a separate gate-ref drift is captured in section H.
XR-F-22 uses the later implementation pins. Do not attribute the release
binding or its regression tests to the original client pin.
D31 rows (XR-F-2, XR-F-10, XR-F-20, XR-F-23) use the later pins
`tbtc-v2@806b1106f` and `keep-core@d40f6aadc`, like XR-F-22. Each such row
keeps the original-pin state (at `9d2086965` / `f99ef96e3`) identifiable.

| ID | Signature / selector | keep-core binding + call site (both) | Contract function (matching) | Drift status @ pin |
|---|---|---|---|---|
| XR-F-1 | `requestReservationReanchor(uint256,bytes20,uint32[],uint256)` = `0x3bf5b58f` | Binding: `keep-core:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:864` (selector note) + caller `PendingThirdPartyReanchorHolds`-free method at `:866`; call site `keep-core:pkg/chain/ethereum/tbtc.go:859` (`RequestReservationReanchor`), gas-est at `:865`, tx at `:878`; SPV/PG task `keep-core:pkg/tbtcpg/reservation_reanchor.go:673` (`ProposeReservationReanchor`) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:742`, router entry `:239`, interface `IReservationBridge.sol:319`. Args: `reservationKey`, `targetWalletPubKeyHash` (20-byte), `walletMembersIDs` (non-empty = member proof), `walletMemberIndex` | `holds` (selector pinned in the ABI-snapshot test, see XR-G-3; both sides agree at pin) |
| XR-F-2 | `updateReservationCaps(uint64,uint64,uint32,uint32,uint32)` = `0x78d76eda` (D31, five arguments). At the original pins it was `updateReservationCaps(uint64,uint64,uint32,uint32)` = `0xf858967d`. Governance staging: `beginReservationCapsUpdate(uint64,uint64,uint32,uint32,uint32)` = `0x4c1cda81` (four-argument `0x3684e886` at the original pin); `finalizeReservationCapsUpdate()` = `0x7d87eff2` (unchanged). Event `ReservationCapsUpdated(uint64,uint64,uint32,uint32,uint32)` topic0 `0x964ef9bd68fafce7168d52b1d95f0da86e3be014f9843a793b021652e4e28daa` (four-field topic0 `0x11af1b44d097c7893ef437ea940dea2e21b5c822807b98a3c4d4d9f7f0f11e5d` at the original pins) | Binding at `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:1003` and free-call wrapper `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc/gen/contract/ReservationRouter.go:1737`; call site: none in production keep-core (governance-only entry point; keep-core has a binding but no caller outside generated code). At the original pins the four-argument binding was `keep-core:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:948`. | At `806b1106f`: library `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1687-1745` (zero reserve rejected, "Hand-off reserve must be greater than zero" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`; stored at `:1736`), router `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:476` (fifth parameter at `:481`), interface `tbtc-v2@806b1106f:solidity/contracts/bridge/IReservationBridge.sol:579` (fifth parameter at `:584`). Staging `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896`, finalize `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1917`. Event declared at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:402-408`. Args: `maxReservationsAmountPerWallet`, `reservationMaxSingleAmount`, `maxActiveReservations`, `maxThirdPartyReanchorHoldsPerWallet`, `handoffReserveWallets`. At `9d2086965` (four arguments): `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1494`, router `:450`, interface `IReservationBridge.sol:527`. | `holds` (the 5-tuple is the D31 shape; selector `0x78d76eda` and the event topic0 are pinned by the ABI snapshot test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts:36` (selectors) and `:57` (event topics) against the fixture `tbtc-v2@806b1106f:solidity/test/fixtures/ReservationAbi.snapshot.json:147` and `:223`. Zero reserve reverts: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationCaps.test.ts:163`. Five-argument staging and finalize: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4705` (begin), `:4789` (finalize). keep-core encodes the tuple, does not call it. The four-argument `0xf858967d` shape was pinned by the same snapshot at `9d2086965`. The setter enforces reserve > 0 only when it runs: an upgraded Bridge reads 0 until the first five-argument update, which the deploy steps run before activation) |
| XR-F-3 | `pendingThirdPartyReanchorHolds(bytes20)` = `0x18576b18` | Binding: `keep-core:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:353` (Caller method) + Transactor-free view at `:350`; generated free-call wrapper `keep-core:pkg/chain/ethereum/tbtc/gen/contract/ReservationRouter.go:2092`; **no production keep-core caller** (the D19 headroom is computed off `WalletReservationsCount`/`WalletReservationsAmount` views instead, see XR-F-9/XR-F-10) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol` (mapping write under `requestReservationReanchor`), view declared `IReservationBridge.sol:563`, router `:489`, storage `BridgeState.sol:504` | `holds-untested` (binding + view exist and agree at pin; no production caller, so no keep-core test pins the round-trip. The gate-ref gap is XR-G-2) |
| XR-F-4 | `activeReservationsCount() -> (uint32 count, uint32 maxActive)` = `0x93fe5eab` | Binding caller `keep-core:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go` (`ActiveReservationsCount` returns a struct with `.Count`/`.MaxActive`); call site `keep-core:pkg/chain/ethereum/tbtc.go:1671` maps to `(Count, MaxActive, err)`; SPV/PG gauge `keep-core:pkg/tbtcpg/reservation_acceptance.go:701` and `:705` | `tbtc-v2:solidity/contracts/bridge/Reservation.sol` (reads `self.activeReservationsCount`, `self.maxActiveReservations`), view `IReservationBridge.sol:693` (2-tuple), router `:617` | `holds` (the 2-tuple return is mirrored by keep-core's 2-value mapping; a type change to a 1-tuple breaks the binding struct and the `make verify-vendored-fallback` whole-ABI diff) |
| XR-F-5 | `walletReservationsCount(bytes20) -> uint32` | Binding caller `keep-core:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go` (`WalletReservationsCount`); call site `keep-core:pkg/chain/ethereum/tbtc.go:1291`; used as a cheap short-circuit in `WalletReservations` at `:1335` (count==0 returns nil, skipping the unbounded event scan) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol` (reads `walletReservationInfo[wallet].count`), view `IReservationBridge.sol:653`, router `:512` | `holds` (used by `keep-core:pkg/chain/ethereum/tbtc.go:1318` and `:1338`) |
| XR-F-6 | `walletReservationsAmount(bytes20) -> uint64` | Binding caller; call site `keep-core:pkg/chain/ethereum/tbtc.go:1274` | `tbtc-v2:solidity/contracts/bridge/Reservation.sol` (`walletReservationInfo[wallet].amount`), view `IReservationBridge.sol:644`, router (near `:512`) | `holds-untested` (single call site, no dedicated round-trip test found by reading) |
| XR-F-7 | `reservations(uint256) -> ReservationRequest` | Binding caller; call site `keep-core:pkg/chain/ethereum/tbtc.go:420` (`GetReservation`) returns a `*tbtc.Reservation` with `State` mapped to the keep-core enum (XR-S-1) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol` (`self.reservations[key]`), view `IReservationBridge.sol:584`, router | `holds` (state enum translation pinned by `keep-core:pkg/tbtcpg/reservation_reanchor_inflight_test.go` resume cases) |
| XR-F-8 | `reservationActions(uint256,uint64) -> ReservationAction` | Binding caller; call site `keep-core:pkg/chain/ethereum/tbtc.go:446` (`GetReservationAction`) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol` (`self.reservationActions[actionKey]`), view `IReservationBridge.sol:597` | `holds` (the action's `requestNonce` is what `validateReservationReanchorProposal` requires to be a Pending Reanchor; pinned by `keep-core:pkg/tbtcpg/reservation_reanchor_member_proof_test.go`) |
| XR-F-9 | `reservationParameters() -> (reservationVault, minAmount, txMaxFee, termSeconds, dissolutionDelay, ...)` = `0xf75b4b1c` | Binding caller; call site `keep-core:pkg/chain/ethereum/tbtc.go:478` (`ReservationParameters`) returns a `*tbtc.ReservationParameters`; `params.ReservationTxMaxFee` and `params.ReservationMinAmount` feed the re-anchor headroom check at `keep-core:pkg/tbtcpg/reservation_reanchor.go:481` and the fee assembly at `:767` | `tbtc-v2:solidity/contracts/bridge/Reservation.sol` (`self.reservation*` struct), view `IReservationBridge.sol:623` | `holds` (keep-core mirrors every field on-chain sets; the feasibility subset is XR-M-3/XR-T-4) |
| XR-F-10 | `reservationCaps() -> (maxReservationsAmountPerWallet, reservationMaxSingleAmount, maxActiveReservations, maxThirdPartyReanchorHoldsPerWallet)` | Binding caller; call site `keep-core:pkg/chain/ethereum/tbtc.go:1256` (`ReservationCaps`) returns `(MaxReservationsAmountPerWallet, ReservationMaxSingleAmount, err)` (4-field on-chain struct, keep-core surfaces 2) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol` (`self.*` caps), view `IReservationBridge.sol:548`, router. At `806b1106f` (D31) it stays a four-value view: `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:495`, `tbtc-v2@806b1106f:solidity/contracts/bridge/IReservationBridge.sol:601`, snapshot fixture `tbtc-v2@806b1106f:solidity/test/fixtures/ReservationAbi.snapshot.json:82` (four outputs). The hand-off reserve has its own view (XR-F-23). | `holds` (the 4-tuple includes the D19 `maxThirdPartyReanchorHoldsPerWallet`; keep-core reads the two amount caps for headroom; D31 adds no fifth output, and the ABI snapshot test pins the four outputs) |
| XR-F-11 | `submitReservationAcceptanceProof((bytes4,bytes,bytes,bytes4),(bytes,uint256,bytes,bytes32,bytes),uint256,uint64)` | Binding ABI at `keep-core:pkg/chain/ethereum/tbtc/gen/ReservationRouter.fallback-artifact.json`; adapter `keep-core:pkg/chain/ethereum/tbtc.go:896`; SPV consumer `keep-core:pkg/maintainer/spv/reservation_proof_loop.go:666` | `tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:263-287`: BitcoinTx.Info, BitcoinTx.Proof, reservation key, uint64 nonce | holds by inspected canonical ABI; behavior evidence in proof-loop item-error and contract SPV tests |
| XR-F-12 | `submitReservationReanchorProof((bytes4,bytes,bytes,bytes4),(bytes,uint256,bytes,bytes32,bytes),uint256,uint64)` | Same fallback ABI; adapter `keep-core:pkg/chain/ethereum/tbtc.go:951`; SPV consumer `keep-core:pkg/maintainer/spv/reservation_proof_loop.go:1149` | `tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:289-313`: the same tuple types, key and uint64 nonce | holds by inspected canonical ABI; not a `(uint256,uint256,bytes,bytes)` interface |
| XR-F-13 | `notifyReservationActionTimeout(uint256)` | Binding caller; call site `keep-core:pkg/chain/ethereum/tbtc.go:999`; SPV watch `keep-core:pkg/maintainer/spv/reservation_action_timeout_watch.go` (fires the overdue counter XR-M-5) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:934`, router `:315`, interface `IReservationBridge.sol:392` | `holds-untested` (watcher fires; no dedicated revert-round-trip test found by reading) |
| XR-F-14 | `notifyReservationAcceptanceTimedOut(uint256)` | Binding caller; call site `keep-core:pkg/chain/ethereum/tbtc.go:1188` | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:667`, router `:334`, interface `IReservationBridge.sol:408` | `holds-untested` |
| XR-F-15 | `notifyStaleReservedDeposit(uint256)` | Binding caller; call site `keep-core:pkg/chain/ethereum/tbtc.go:1024` (`NotifyStaleReservedDeposit`, tx at `:1037`) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1229`, router `:396`, interface `IReservationBridge.sol:474` | `holds-untested` |
| XR-F-16 | `notifyReservationStranded(uint256)` | Binding caller; call site `keep-core:pkg/chain/ethereum/tbtc.go:1049` | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1192`, router `:425`, interface `IReservationBridge.sol:502` | `holds-untested` |
| XR-F-17 | `requestReservationAcceptance((bytes4,bytes,bytes,bytes4),(bytes,uint256,bytes,bytes32,bytes),uint32,bytes20)` = `0x19821f16` | Binding `keep-core@4ced1c839:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:895-911`: the generated ABI and transactor carry the four-argument form, so the two repositories agree. **No production keep-core caller** (depositor-only entry; keep-core exposes the binding and the generated CLI, and does not send it in M1). SDK consumer: `tbtc-v2@d1bf5bc281:typescript/src/services/reservations/reservations-service.ts:539-563`. | `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/ReservationRouter.sol:230-247` (funding transaction, SPV proof and output index; key derived from the proven hash), `Reservation.sol:499`. At the original pin the same entry was `(uint256,bytes20)` with no proof, and the binding at that pin matched it (VF-12) | `holds` at `tbtc-v2@d1bf5bc281` + `keep-core@4ced1c839`: the ABI agrees on both sides. The earlier two-argument form does not match the published target (VF-12) |
| XR-F-18 | `forceStaleReservedDeposit(uint256)` = `0x9fa3f062` | Binding caller `keep-core:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:738`; **no production keep-core caller** (governance-only) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1285`, router `:410`, interface `IReservationBridge.sol:488` | `holds-untested` |
| XR-F-19 | `validateReservationAnchorProposal(ReservationAnchorProposal,DepositExtraInfo) -> bool` = `0xf9179ad2` | WVP binding `keep-core:pkg/chain/ethereum/tbtc/gen/abi/WalletProposalValidator.go`; adapter `keep-core:pkg/chain/ethereum/tbtc.go:507-547` | `tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:963`; called on the validator contract, not the router | holds; real-validator behavior harness `keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:760` |
| XR-F-20 | `validateReservationReanchorProposal(ReservationReanchorProposal) -> bool` | Same WVP binding; adapter `keep-core:pkg/chain/ethereum/tbtc.go:598-629`; client proposal site `keep-core:pkg/tbtcpg/reservation_reanchor.go:673` | At `9d2086965`: `tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:1108`, and the validator re-checked the target's count and amount caps at signing (`WalletProposalValidator.sol:1171`, `:1183`; finding VF-01). At `806b1106f` (D31): `tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1115`; the validator no longer re-checks target caps, and its doc block says capacity is enforced once at request time and not repeated (`tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1092-1098`). The target Live check stays: "Target wallet must be in Live state" `tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1159`. | holds; at the original pins the real-validator harness `keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:1006` rejected a target above the count cap (`:1191`). At `keep-core@d40f6aadc` the harness `TestValidateReservationReanchorProposal` (`keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1014`) accepts a target at the count cap (`keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1193`), above the count cap (`:1205`) and above the amount cap (`:1219`). Mocked-bridge contract test: `tbtc-v2@806b1106f:solidity/test/bridge/WalletProposalValidator.test.ts:4322-4382` |
| XR-F-21 | `isReservedDeposit(uint256) -> bool` | Binding caller `keep-core:pkg/chain/ethereum/tbtc/gen/` (Bridge binding, vendored fragment `Bridge.reservation-methods-fallback.json`); call site `keep-core:pkg/chain/ethereum/tbtc.go` `IsReservedDeposit` | `tbtc-v2:solidity/contracts/bridge/Bridge.sol:1629` | `holds` (used to route reserved deposits; the deposit sweep gates on it) |
| XR-F-22 | `releaseReservedDepositToPool(uint256,address)` = `0x49fdfb19` | Added binding: `keep-core@4b95f0a06:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:846`. No production keep-core transaction caller is required; the depositor submits it. Sweep uses stored routing and `IsReservedDeposit` (`keep-core:pkg/tbtcpg/deposit_sweep.go:252-264`). | `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1373-1453`: clears reserved state, decrements pending count and writes the destination vault. Router entry: `tbtc-v2@d20cf74ba:solidity/contracts/bridge/ReservationRouter.sol:433`. | `holds` by source inspection with behavioral regressions at `keep-core@4b95f0a06:pkg/tbtcpg/deposit_sweep_release_test.go:91` (`TestFindDepositsToSweep_IncludesReleasedDeposit`) and `keep-core@4b95f0a06:pkg/tbtc/deposit_sweep_release_test.go:21` (`TestValidateDepositSweepProposal_AcceptsReleasedDeposit`). |
| XR-F-23 | `reservationHandoffReserveWallets() -> uint32` = `0xcf098fc5` (D31; absent at the original pins) | Binding: `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:501` (caller) and free-call wrapper `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc/gen/contract/ReservationRouter.go:2433`. **No production keep-core caller** (checked with `git grep` for `ReservationHandoffReserveWallets` at `d40f6aadc` outside the generated bindings). SDK consumer: `tbtc-v2@806b1106f:typescript/src/lib/ethereum/reservations.ts:333`, read-handle test `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:559`. | `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:519`, interface `tbtc-v2@806b1106f:solidity/contracts/bridge/IReservationBridge.sol:618`, storage `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeState.sol:509`; read by the acceptance reserve gate `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1077` | `holds` (selector and `uint32` output pinned by the ABI snapshot test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts:36` against `tbtc-v2@806b1106f:solidity/test/fixtures/ReservationAbi.snapshot.json:88`; the binding is covered by the drift check in section H, recorded 8/8 at `d40f6aadc`. keep-core does not call it, so no keep-core behavior test exists) |
| XR-F-24 | `reservationByAnchorUtxo(bytes32,uint32) -> uint256` = `0x79731f67` | Binding caller `keep-core@4ced1c839:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:415` (selector note `:412`); adapter `keep-core@4ced1c839:pkg/chain/ethereum/tbtc.go:1695-1713` (`ReservationByAnchorUtxo`), declared on the SPV chain interface `keep-core@4ced1c839:pkg/maintainer/spv/chain.go:223`. No production keeper caller yet; the generated CLI exposes it (`keep-core@4ced1c839:pkg/chain/ethereum/tbtc/gen/cmd/ReservationRouter.go:285`) | `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/ReservationRouter.sol:584` (router entry), interface `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/IReservationBridge.sol:736`; reads the `reservationsByAnchorUtxo` index and returns zero for an untracked outpoint | `holds-untested` (the contract view round-trips in `tbtc-v2@d1bf5bc281:solidity/test/bridge/Bridge.ReservationSourceAnchorBinding.test.ts:625-633`, which pins the output-index semantics; no keep-core test round-trips the adapter) |
| XR-F-25 | `reservedDepositWallet(uint256) -> bytes20` = `0x56803b55` | Binding caller `keep-core@4ced1c839:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:679` (selector note `:676`); adapter `keep-core@4ced1c839:pkg/chain/ethereum/tbtc.go:1718-1731` (`ReservedDepositWallet`); production caller `keep-core@4ced1c839:pkg/maintainer/spv/reservation_stale_deposit_watch.go:310`, which resolves the designated wallet before notifying the Bridge | `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/ReservationRouter.sol:601` (router entry), interface `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/IReservationBridge.sol:746`; returns `pendingReservedDeposit[key].walletPubKeyHash`, zero when the deposit is not a pending reserved deposit | `holds-untested` (the contract view is read in `tbtc-v2@d1bf5bc281:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts:381`; the adapter is exercised only through a mock in `keep-core@4ced1c839:pkg/maintainer/spv/reservation_stale_deposit_watch_test.go`) |

Note on `updateReservationParameters` / `setVaultStatus`: governance-only, not
invoked by keep-core; `setVaultStatus` is gated to the C-4 activation block
(XR-T-5) but the call itself lives in deploy tooling, not keep-core.

## B. Events keep-core scans or subscribes to

Reservation libraries execute through delegatecall, so their emitted logs
have the Bridge proxy address, not the deployed router address.
keep-core attaches the router ABI to `bridgeAddress`
(`keep-core:pkg/chain/ethereum/tbtc.go:369-378`).
Indexing below preserves declaration order; non-indexed fields remain data.
SPV and timeout scans use 10,000-block chunks
(`keep-core:pkg/maintainer/spv/reservation_event_scan.go:19`, `:115-133`).
`keep-core:pkg/chain/ethereum/tbtc.go` `WalletReservations` further bounds a
wallet's scan by that wallet's earliest `NewWalletRegistered` block
(`keep-core:pkg/chain/ethereum/tbtc.go:1335`, resolver under `:1335`).

| ID | Event (declaration order and indexing) | keep-core read site | Contract emit site | Lookback bound | Drift status |
|---|---|---|---|---|---|
| XR-E-1 | `ReservationAcceptanceRequested(uint256 indexed reservationKey,uint64 requestNonce,bytes20 indexed walletPubKeyHash,uint64 depositAmount,uint64 txMaxFee,uint32 timeoutAt)` | `keep-core:pkg/chain/ethereum/tbtc.go:1854`, `:1887`; proof loop :463 and timeout watcher :324 | `tbtc-v2:solidity/contracts/bridge/IReservationBridge.sol:41-48`; library emit `Reservation.sol:642` under Bridge delegatecall | Activation block; chunked scan | `holds-untested` for real log decoding; source fields match |
| XR-E-2 | `ReservationReanchorRequested(uint256 indexed reservationKey,uint64 requestNonce,bytes20 indexed sourceWalletPubKeyHash,bytes20 indexed targetWalletPubKeyHash,uint64 txMaxFee)` | `keep-core:pkg/chain/ethereum/tbtc.go:1935`, `:1967`; proof loop :958 and timeout watcher :347 | `tbtc-v2:solidity/contracts/bridge/IReservationBridge.sol:86-92`; three indexed fields, two data fields; emit `Reservation.sol:925` | Activation block; chunked scan | `holds-untested` for real log decoding |
| XR-E-3 | `NewWalletRegistered(bytes32 indexed ecdsaWalletID,bytes20 indexed walletPubKeyHash)` | Wallet registration scans used by custody reconstruction and target discovery | `tbtc-v2:solidity/contracts/bridge/Bridge.sol:145-148` | Consumer-specific history search; some fallback ranges span history | `holds-untested`; no wallet-member-ID array exists in this event |
| XR-E-4 | `DepositRevealed(bytes32 fundingTxHash,uint32 fundingOutputIndex,address indexed depositor,uint64 amount,bytes8 blindingFactor,bytes20 indexed walletPubKeyHash,bytes20 refundPubKeyHash,bytes4 refundLocktime,address vault)` | `keep-core:pkg/tbtcpg/deposit_sweep.go:252-264`: event vault is a candidate filter; the current reserved flag decides exclusion | `tbtc-v2:solidity/contracts/bridge/Bridge.sol:77-87`; deposit key is derived, not an indexed event field | Deposit consumer lookback | `holds` for the post-release routing regression at client `4b95f0a06`; event-only classification would be wrong |

`ReservationOccupancyChanged`, `ReservationAccepted`,
`ReservationCapsUpdated` and `ReservationParametersUpdated` are not the
request-history events used by the cited SPV/timeout consumers.
Custody reconstruction scans acceptance and re-anchor requests
(`keep-core:pkg/chain/ethereum/tbtc.go:1364-1378`).
Logs, views and their ABI remain separate review surfaces.

## C. Revert strings keep-core matches versus the contract

`keep-core` matches a single substring `isReservationCapRevertError`
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:618`) = `strings.Contains(err,
"cap exceeded")` to decide "target capacity changed since headroom pre-check,
exclude and try the next target" (`:559`). It must mirror BOTH on-chain
sources that end in `cap exceeded`:

| ID | keep-core matcher (cite) | Contract exact string (cite) | Mirror target | Drift status |
|---|---|---|---|---|
| XR-R-1 | `strings.Contains(err.Error(), "cap exceeded")` at `keep-core:pkg/tbtcpg/reservation_reanchor.go:618` (original pin); at `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:630` (function at `:626`, use at `:571`) | At `9d2086965`: `tbtc-v2:solidity/contracts/bridge/Reservation.sol:887` "Wallet reservations cap exceeded" (in `requestReservationReanchor`), `:900` "Wallet reserved amount cap exceeded"; AND `WalletProposalValidator.sol:1171` "Wallet reservations cap exceeded", `:1183` "Wallet reserved amount cap exceeded". At `806b1106f` (D31): the validator strings are gone; the request path keeps "Wallet reservations cap exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:931` and "Wallet reserved amount cap exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:946`, skipped when `skipTargetCaps` is set (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`) | At the original pins the two request-path and two validator-path `require`s ended in "cap exceeded", so one substring covered all four. At `806b1106f` the two request-path strings remain for third-party requests and for governance from a Live source; a privileged member request from a retiring source no longer reverts on caps, so the client expects this revert only on the non-privileged and Live-source paths. | `holds-untested` at `d40f6aadc`. At the original pins this was `holds`, pinned by `keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:1191` `mustContainError(err, "Wallet reservations cap exceeded")`; that harness case now expects acceptance, so no cross-repo test pins the wording. Contract tests pin the strings: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466` (third party, count), `:485` (governance from a Live source), `:502` (third party, amount). The client matcher is pinned only against fake errors: `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:997` (`TestReservationReanchorTask_CapRevertLeadsToNextCandidate`). **Drift risk: if the contracts change the tail from "cap exceeded" to another wording, `isReservationCapRevertError` stops matching and a genuine concurrent-cap collision is misread as a fatal error.** |
| XR-R-2 | cooldown error: `keep-core:pkg/tbtcpg/reservation_reanchor_test.go:1874` `strings.Contains(err.Error(), "Reanchor cooldown in effect")` | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:761` "Reanchor cooldown in effect" | re-anchor request reverts while the previous generation's `timeoutAt` (with the safety margin, XR-T-3) has not passed | `holds` (exact string pinned by the test assertion at `keep-core:pkg/tbtcpg/reservation_reanchor_test.go:1874`) |

Other reservation reverts keep-core observes but does not special-case (it just
surfaces them): "Reservations are disabled" `Reservation.sol:506`, "Reservation
is not active" `:755`, "Caller is not a member of the source wallet" `:783`,
"Source wallet must be in MovingFunds or Closing state" `:805`, "Target wallet
must differ from the source wallet" `:812`, "Reanchor authorization has no
signing window" `:842`, "Reanchor would fall below the minimum reservation
amount" `:856`, "Target third-party re-anchor holds exceeded" `:871`.

## D. Proposal validation: `WalletProposalValidator` checks versus keep-core builds

Keep-core builds the proposal struct and fee, then runs the validator's exact
view on-chain before the wallet signs; a view that reverts or returns false
means the proposal cannot be signed. The validator is `external view` (pure
read), so it is safe to call repeatedly.

| ID | Validator check (contract cite) | keep-core build/site | Safety-margin | Drift status |
|---|---|---|---|---|
| XR-P-1 | Anchor fee: `0 < anchorTxFee <= action.txMaxFee`; re-anchor fee: `0 < reanchorTxFee <= action.txMaxFee` | Assembly enforces the action's fee allowance (`keep-core:pkg/tbtc/reservation.go:419`, `:474`) | `tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:1033-1040`, `:1186-1193` | `holds`; ordinary sweep-fee formulas do not define reservation fees |
| XR-P-2 | Acceptance requires `deposit.amount >= action.minAmount + anchorTxFee` | Acceptance uses the generation's snapshotted minimum; re-anchor headroom is a different request policy | `tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:1048-1051` | `holds` for the immutable-generation rule; see the real validator harness, not re-anchor headroom as acceptance evidence |
| XR-P-3 | Deposit age: `block.timestamp > depositRequest.revealedAt + DEPOSIT_MIN_AGE` `WVP:1017`, `DEPOSIT_MIN_AGE = 2 hours` (`WVPConstants:12`) | keep-core's acceptance task only proposes once the deposit is old enough (deposit sweep timing) | 2 hours | `holds-untested` |
| XR-P-4 | Refund window: `block.timestamp + DEPOSIT_REFUND_SAFETY_MARGIN < refund locktime` `WVP:1065`, `DEPOSIT_REFUND_SAFETY_MARGIN = 24 hours` (`WVPConstants:13`) | keep-core assembles `refundLocktime` inside the reveal window | 24 hours | `holds-untested` |
| XR-P-5 | Re-anchor signing window: `block.timestamp < action.timeoutAt - REQUEST_TIMEOUT_SAFETY_MARGIN` `WVP:1133`, "Re-anchor action has timed out"; `REQUEST_TIMEOUT_SAFETY_MARGIN = 2 hours` (`WVPConstants:14`), mirrored as the public `REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN` `WVP:198` | keep-core asserts the same window before proposing: `keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:87` `const requestTimeoutSafetyMarginSeconds uint32 = 2*60*60` "must equal tbtc-v2's `WalletProposalValidatorConstants.REQUEST_TIMEOUT_SAFETY_MARGIN`" (the constant is deliberately hard-coded, not re-derived) | 2 hours | `holds` (the hard-coded 2h constant at `:87` is asserted to mirror the on-chain constant; if the on-chain margin changes the harness at `:963`/`:1161` flips accept/reject at exactly the boundary). **Drift risk: a change to `REQUEST_TIMEOUT_SAFETY_MARGIN` in `WVPConstants` silently breaks the `:87` mirror; there is no cross-repo test that reads the on-chain constant - the agreement is by manual constant.** |
| XR-P-6 | Re-anchor target: `action.targetWalletPubKeyHash == proposal.targetWalletPubKeyHash` `WVP:1133` region "Target wallet does not match the authorized action"; `action.actionType == Reanchor` and `action.state == Pending` `WVP:1108` | keep-core builds `targetWalletPubKeyHash` to the current action target: `keep-core:pkg/tbtcpg/reservation_reanchor.go:673` | n/a | `holds` |
| XR-P-7 | Reserved-deposit routing: "Deposit was not revealed as reserved" `WVP:1025`; "Deposit not routed to the reservation vault" `WVP:1030` | keep-core's deposit sweep only routes reserved deposits when the vault is set and active (XR-T-5) | n/a | `holds-untested` |

## E. Timing constants keep-core assumes

| ID | Constant (value) | keep-core site (cite) | On-site mirror (cite) | Drift status |
|---|---|---|---|---|
| XR-T-1 | Coordination frequency = 900 blocks | `keep-core:pkg/tbtc/coordination.go:33` `coordinationFrequencyBlocks = 900` (window index = `block / 900`, `:174`) | No on-chain counterpart (client-side cadence); reservation work runs inside each 900-block coordination window, gated by the activation block | `holds-untested` (client-side cadence; no on-chain value to match) |
| XR-T-2 | Action timeout and re-anchor cooldown are distinct deadlines | Reads live action timeout in `keep-core:pkg/chain/ethereum/tbtc.go:478`; pending actions retain their snapshot | Request sets `timeoutAt`; timeout notification sets `reanchorCooldownUntil = now + action duration`. Only governance bypasses `now >= cooldownUntil` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:758-762`, `:984-986`). Signing separately requires `now < timeoutAt - safetyMargin`. | `holds-untested` for the combined timing agreement |
| XR-T-3 | `REQUEST_TIMEOUT_SAFETY_MARGIN = 2 hours` | `keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:87` (hard-coded 7200s) | `tbtc-v2:solidity/contracts/bridge/WalletProposalValidatorConstants.sol:14` | `holds` (see XR-P-5 drift note: the two are equal by manual constant, not by a cross-repo read) |
| XR-T-4 | `DEPOSIT_MIN_AGE = 2h`, `DEPOSIT_REFUND_SAFETY_MARGIN = 24h` | keep-core deposit-sweep timing mirrors these windows (reserved-deposit routing in `keep-core:pkg/tbtc/deposit_sweep.go`) | `tbtc-v2:solidity/contracts/bridge/WalletProposalValidatorConstants.sol:12`,`:13` | `holds-untested` |
| XR-T-5 | C-4 activation block B | `keep-core:pkg/tbtc/coordination.go:94-119`: Developer starts at zero; an unconfigured public network uses `math.MaxUint64`. SPV wiring uses B as its scan start. | No on-chain B check exists. Trust gating protects reveals, while settlement can directly credit the depositor after trust revocation. | `holds-untested` for the operator/deployment ceremony; source gate is explicit |
| XR-T-6 | Acceptance-request look-back margin = 7,200 blocks (about one day at 12 s per block) | `keep-core:pkg/tbtcpg/reservation_acceptance.go:27` (`reservationAcceptanceRequestLookBackMarginBlocks = uint64(7200)`), added to the action-timeout-derived look-back window at `:754` | `requestReservationAcceptance` sets `timeoutAt = now + reservationActionTimeout` (`tbtc-v2@d1bf5bc281:solidity/contracts/bridge/Reservation.sol:600-601`), so a generation that can still be signed was requested within that timeout; the margin absorbs block-time drift and a moderate governance decrease of the timeout | `holds-untested` (a client-chosen window, not a value either side enforces; no test pins the block conversion) |
| XR-T-7 | Acceptance reveal look-back margin = 7 days | `keep-core:pkg/tbtcpg/reservation_acceptance.go:38` (`reservationAcceptanceRevealLookBackMargin = 7 * 24 * time.Hour`), applied to the reveal scan bound at `:825-841` | A reserved deposit's refund deadline is capped at reveal time + the then-current term + `DEPOSIT_REFUND_SAFETY_MARGIN` (`tbtc-v2@d1bf5bc281:solidity/contracts/bridge/Deposit.sol:244-251`), and the acceptance request requires `timeoutAt + DEPOSIT_REFUND_SAFETY_MARGIN <= refundDeadline` (`tbtc-v2@d1bf5bc281:solidity/contracts/bridge/Reservation.sol:631-636`), so the reveal preceded the request by at most one snapshotted term; the margin absorbs a term decrease of up to about a week plus block-time drift | `holds-untested` (a client-chosen window; no test pins the conversion) |

## F. State-machine assumptions keep-core encodes

| ID | Assumption | keep-core site | On-site contract rule | Drift status |
|---|---|---|---|---|
| XR-S-1 | keep-core reads `reservations(key).state` and acts on `Active` (resume/custody) and `ActionPending` (a re-anchor generation is pending). The Go enum `ReservationState` maps Unknown/Active/ActionPending/Closed/Stranded to the on-chain 5-value enum. | `keep-core:pkg/chain/ethereum/tbtc.go:420` + enum `keep-core:pkg/tbtc/reservation.go:98`; re-anchor task only acts on a pending Reanchor target (`keep-core:pkg/tbtcpg/reservation_reanchor.go:770`) | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:74` `enum ReservationState` (0 Unknown, 1 Active, 2 ActionPending, 3 Closed, 4 Stranded) | `holds` (the ordinal-to-name mapping is pinned by the resume tests in `keep-core:pkg/tbtcpg/reservation_reanchor_inflight_test.go`; a re-ordered enum would break name decoding, which the Go binding's numeric-enum decode would surface) |
| XR-S-2 | Enum ordinals must agree | `keep-core:pkg/tbtc/reservation.go` defines state/action enums | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:110-135`: Redemption, Dissolution and Vetoed are unreachable M2 values. Superseded is used by M1 late-settlement unwind. | `holds-untested` for complete ordinal drift; Superseded must not be classified M2-only |
| XR-S-3 | Pending re-anchor resumes the authorized nonce and target | `keep-core:pkg/tbtcpg/reservation_reanchor.go:296-331` | Request sets ActionPending/Pending Reanchor (`Reservation.sol:906-915`). Settlement returns Active or strands on a dead/Closing target; it does not close an M1 position. | `holds` for the existing resume regressions |

## G. Metrics keep-core exports and the on-chain value each mirrors

The original-pin metrics below are declared in `keep-core:pkg/clientinfo/performance.go`
and registered when `reservationsEnabled` (`:392-403`). The later D28 producer/consumer contract follows separately.

| ID | keep-core metric (constant, cite) | On-chain value it mirrors (cite) | Emits (cite) | Drift status |
|---|---|---|---|---|
| XR-M-1 | `active_reservations_count` (`MetricReservationActiveReservationsCount`, `keep-core:pkg/clientinfo/performance.go:767`) | `activeReservationsCount()` count (`IReservationBridge.sol:693`) | `keep-core:pkg/tbtcpg/reservation_acceptance.go:701` | `holds` |
| XR-M-2 | `max_active_reservations` (`keep-core:pkg/clientinfo/performance.go:768`) | `activeReservationsCount()` `maxActive` (`IReservationBridge.sol:693`) | `keep-core:pkg/tbtcpg/reservation_acceptance.go:705` | `holds` |
| XR-M-3 | `reservation_params_infeasible` (`keep-core:pkg/clientinfo/performance.go:797`), a 0/1 alarm | the feasibility rules the on-chain `assertReservedDepositsFeasible` enforces (`BridgeState.sol:681`, called at `:643` on `updateDepositParameters` and at `Reservation.sol:1427`,`:1521` on the two cap/param setters). keep-core recomputes the same feasibility and alarms when a parameter set would leave no reserved deposit possible (D25). | `keep-core:pkg/maintainer/spv/reservation_params_feasibility.go:246` `SetGauge` | `holds` (pinned by `keep-core:pkg/maintainer/spv/reservation_params_feasibility_test.go:165` `TestReservationParamsInfeasibleRevealWindow` and `:381` `...ThenFixedReturnsGaugeToZero`) |
| XR-M-4 | `wallet_reservations_count` (`keep-core:pkg/clientinfo/performance.go:770`) | `walletReservationsCount(hash)` (`IReservationBridge.sol:653`) | `keep-core:pkg/tbtcpg/reservation_acceptance.go:697` | `holds` |
| XR-M-5 | `spv_reservation_watcher_deaths_total` (`keep-core:pkg/clientinfo/performance.go:671`), `spv_reservation_action_overdue_total` (`keep-core:pkg/clientinfo/performance.go:678`) | No on-chain value; client-side SPV health (watcher crash / late notify). The overdue counter increments on every late `notifyReservation*` decision (`keep-core:pkg/maintainer/spv/reservation_action_timeout_watch.go:554`). | `keep-core:pkg/maintainer/spv/reservation_wiring.go:206` (deaths), `:554` (overdue) | `holds-untested` (client-side; no on-chain mirror by design) |
| XR-M-6 | `live_wallets_count` (`keep-core:pkg/clientinfo/performance.go:769`), `reservation_vault_fee_debt_sat` (`keep-core:pkg/clientinfo/performance.go:785`), `reservation_vault_fee_reserve_tbtc_base_units` (`keep-core:pkg/clientinfo/performance.go:786`) | `live_wallets_count` from `GetLiveWalletsCount`; the two vault gauges from `inKindFeeDebtSat` (`ReservationVault.sol:85`, view `inKindFeeDebtSat()`) and the vault's `TBTC.balanceOf(vault)` in 1e18 base units (`ReservationVault.sol`, TBTC binding). | `keep-core:pkg/tbtcpg/reservation_reanchor.go:167` (live), `keep-core:pkg/tbtcpg/reservation_acceptance.go:717`,`:732` (vault fees); wrappers `keep-core:pkg/chain/ethereum/tbtc.go:1779`,`:1806` | `holds-untested` |
| XR-M-7 | `reanchor_waiting_on_aged_target` (task-local gauge, not a clientinfo constant) | No on-chain value; 0/1 "a target wallet exists but none has aged out of its creation period". | `keep-core:pkg/tbtcpg/reservation_reanchor.go:586-599` | `holds-untested` |

### D28 keeper-to-monitor samples

At `9c4e1b221`, the exporter registers these gauges with the `performance_` namespace
(`keep-core@9c4e1b221:pkg/clientinfo/performance.go:392-405`,
`keep-core@9c4e1b221:pkg/clientinfo/performance.go:415-432`).
The monitor at `8eb26c2e2` consumes those exact names
(`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:7-16`).
The maintainer's reservation-proof flag wires the observer independently of the signing-client flag
(`keep-core@9c4e1b221:pkg/maintainer/spv/spv.go:86-135`).

| ID | Exported sample | Producer rule | Monitor rule | Status and test |
|---|---|---|---|---|
| XR-M-8 | `performance_reservation_acceptance_late_proof_at_risk_count` | Unproven acceptance generations near the snapshotted late-proof edge (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:85-106`). | Positive count is Critical (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:157-164`). | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:288-351`, `tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:117-129` |
| XR-M-9 | `performance_reservation_reanchor_no_eligible_target_count` | Active positions on retiring sources without an eligible privileged hand-off target (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:123-208`). | Positive count is Critical (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:165-172`). | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_wallet_discovery_test.go:12-73`, `tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:132-144` |
| XR-M-10 | `performance_reservation_vault_fee_debt_sat` | Read the configured vault debt and publish only after a complete observation (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:170-187`, `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:365-368`). | Any positive satoshi value is Warning (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:173-180`). | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:177-285`, `tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:146-155` |
| XR-M-11 | `performance_reservation_unrequested_deposit_deadline_at_risk_count` | Still-reserved deposits without a request near their last request time (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:220-274`). | Positive count is Warning (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:181-190`). | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:406-474`, `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:747-778`, `tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:157-170` |
| XR-M-12 | `performance_reservation_keeper_observation_timestamp` | Chain-head Unix seconds of the last complete pass; failed reads preserve it (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:93-190`, `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:369-372`). | Missing, invalid or excessively old/future time is Critical; use latest chain time during catch-up (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:98-103`, `tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:192-225`). | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:591-648`, `tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:243-351` |

Initial gauge zeros do not establish health.
The private metrics endpoint, matching network and actual Sentry route are operator prerequisites.
These samples predict risk; they do not authorize transactions or prove recoverability.

## H. Build / ABI drift gate

**How bindings are produced.** `keep-core:pkg/chain/ethereum/tbtc/gen/gen.go`
+ the `abigen` Makefile target generate Go bindings from the `@keep-network/
tbtc-v2` npm artifact (or a local `../tbtc-v2/solidity` build). The four
contracts are `Bridge`, `MaintainerProxy`, `LightRelay`, `WalletProposal
Validator`, `RedemptionWatchtower`, `ReservationRouter`, `ReservationVault`,
`TBTC`. Reservation-specific files `ReservationRouter` and `ReservationVault`
are not published in the npm package at development refs, so they are served
from vendored fallback JSON:
`keep-core:pkg/chain/ethereum/tbtc/gen/ReservationRouter.fallback-artifact
.json` and `ReservationVault.fallback-artifact.json` (Makefile rules
`keep-core:pkg/chain/ethereum/tbtc/gen/Makefile:107-131`), plus the fragments `Bridge.reservation-methods-fallback.json`
(the `isReservedDeposit` entry) and
`WalletProposalValidator.reservation-methods-fallback.json` (the two
`validate*Reservation*Proposal` entries).

**The check that catches drift.** `make verify-vendored-fallback`
(`keep-core:pkg/chain/ethereum/tbtc/gen/Makefile:208-220`) canonicalizes both
sides with `jq -S ... sort_by(.name)` and `diff`s:
- the whole `.abi` of `ReservationRouter.fallback-artifact.json` vs the
  compiled `ReservationRouter.json`;
- the `isReservedDeposit` entry of the Bridge fragment vs compiled `Bridge`;
- the two `validate*Reservation*Proposal` entries of the WVP fragment vs
  compiled `WalletProposalValidator`;
- the whole `.abi` of `ReservationVault.fallback-artifact.json` vs compiled
  `ReservationVault`;
- then, extracted from the committed Go binding's embedded
  `<Name>MetaData.ABI` string via `go run ./checkabi`
  (`keep-core:pkg/chain/ethereum/tbtc/gen/checkabi/main.go`), the same
  comparisons against the committed `abi/<Name>.go` bindings.

A divergence in any of: a missing/renamed function, an added/removed/
reordered parameter, a type change, or an event-field reorder fails the diff.
The tbtc-v2 side also pins the reservation ABI surface in its own suite:
`tbtc-v2:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts`
(regenerates `ReservationAbi.snapshot.json`) plus independently-pinned
EVM-canonical tuple selectors `submitReservationAcceptanceProof 0xe24ad1ae`,
`submitReservationReanchorProof 0x6af550d1`,
`validateReservationAnchorProposal 0xf9179ad2`, and a fourth, all asserted so
a regression to placeholder-style selector computation fails even if the JSON
is regenerated consistently wrong.

D31 drift evidence (orchestrator-recorded run, not a source line): at
`keep-core@d40f6aadc`, `make -C pkg/chain/ethereum/tbtc/gen
verify-vendored-fallback` passed 8/8 with no exclusions against compiled
contracts equal to `tbtc-v2@806b1106f`. The harness validator file matches the
compiled validator.

| ID | Gate | keep-core cite | On-site counterpart | Test that pins it | Drift status |
|---|---|---|---|---|---|
| XR-G-1 | CI ABI gate compiles tbtc-v2 at a fixed ref and diffs against the vendored fallbacks and the committed bindings | `keep-core:.github/workflows/client.yml:206` `TBTC_V2_REF: eec999a...`; `:273` `make -C pkg/chain/ethereum/tbtc/gen verify-vendored-fallback`; `:260`,`:265` assert the produced artifacts came from that ref | The compiled artifact of `TBTC_V2_REF` | the gate job itself (fails loudly on any diff); it was not executed in this documentation task | Not a separate finding: XR-G-2 records the gate-versus-binding mismatch at the committed ref and carries its status. |
| XR-G-2 | Current state: `TBTC_V2_REF` pins `4c98e6b82` (`keep-core:.github/workflows/client.yml:205`), which was the tbtc-v2 reservations branch head when it was set; the branch head, and the freeze pin, is now `d1bf5bc281`. The vendored artifacts were checked against the artifacts compiled at `4c98e6b82` (see [approved design deltas](13-approved-design-deltas.md)); `4c98e6b82..d1bf5bc281` changes no external signature, selector, event or storage slot in the four contract files it touches, so that comparison still describes the frozen surface, while the gate itself compiles the older ref. Moving `TBTC_V2_REF` to `d1bf5bc281` is pending and belongs to the freeze-day step. Original pins, kept as history: The gate ref `eec999a` is an ancestor of the spec pin `9d2086965`, and strictly predates tbtc-v2 commit `6aac91585` ("fix(bridge): cap third-party re-anchor holds per target wallet") which added `pendingThirdPartyReanchorHolds(bytes20)` and the D19 `maxThirdPartyReanchorHoldsPerWallet` field to the router/caps surface. `keep-core:f99ef96e3`'s vendored `ReservationRouter.fallback-artifact.json` and committed `abi/ReservationRouter.go` DO contain `pendingThirdPartyReanchorHolds`. | `keep-core:pkg/chain/ethereum/tbtc/gen/ReservationRouter.fallback-artifact.json` (contains the method); `keep-core:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:353` | `tbtc-v2` @ `9d2086965`: router `ReservationRouter.sol:489` and `IReservationBridge.sol:563` have it; `tbtc-v2` @ `eec999a` does not | `keep-core:.github/workflows/client.yml:206` `TBTC_V2_REF=eec999a` vs `:273` the diff; the gate job was not executed in this documentation task (VF-06) | The grade below is the original-pin value and has **not** been regraded at the freeze: regrading this row is a separate decision that this set does not take, so the current-state clause above and the status column are read together. At the original pins: `violated` (a source comparison, not a gate run: the gate's own invariant "compiled @ `TBTC_V2_REF` == committed bindings" does not hold at the committed ref. Repro: build the artifacts from `eec999a`, run `make -C pkg/chain/ethereum/tbtc/gen verify-vendored-fallback`, and the `ReservationRouter` whole-ABI diff reports `pendingThirdPartyReanchorHolds` present on the keep-core side, absent on the compiled side. Scope: the keep-core reservations branch; the epic branch head `2b63ae60d` has no hold-cap view in its fallback. The workflow checks out the tbtc-v2 repository, and `6aac91585` is not an ancestor of the spec pin. Approved sequencing (D33, 2026-10-12): bump `TBTC_V2_REF` together with the vendored harness validator, in the keep-core change that brings the hold-cap and D31 bindings to the public PR, once the tbtc-v2 branch carries that commit; run the gate locally against that commit before the push) |
| XR-G-3 | Selector pinning for the four tuple-input entry points | At the original pin `keep-core:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:864` `requestReservationReanchor = 0x3bf5b58f`, `:948` `updateReservationCaps = 0xf858967d`. At `keep-core@d40f6aadc` the caps setter binding is five-argument: `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:1003` `updateReservationCaps = 0x78d76eda`; `requestReservationReanchor` stays `0x3bf5b58f` (`keep-core@d40f6aadc:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:916`). | `tbtc-v2:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts` canonical tuple selectors (independent of the shared snapshot JSON) | that test asserts the canonical tuple selectors (`0xe24ad1ae` / `0x6af550d1` / `0xf9179ad2` / `0xa57f734f`); the `updateReservationCaps` selector (`0xf858967d` at `9d2086965`, `0x78d76eda` at `806b1106f`) is pinned by the snapshot's "function selectors match" case | `holds` (the on-chain selector and the Go binding selector are asserted equal; the setter selector changed with D31, see XR-F-2) |

## Doc drift

| Doc (cite) | What it says | What the code does at the pins |
|---|---|---|
| `keep-core:.github/workflows/client.yml:206` (a shipped config, not the spec `docs/spec`) | `TBTC_V2_REF = eec999a`, the ABI-gate ref | The spec pin is `9d2086965`, which is `eec999a` + D19 router surface + the feasibility guard; `eec999a`'s compiled ReservationRouter ABI is not the surface the committed keep-core bindings encode (Finding XR-G-2). Any doc that claims "the gate passes at its committed ref" is false at `eec999a`. |
| Existing spec `docs/spec/reservations/*` (background only) | describes the pre-M1 stack (#1088-#1096) and the reverted #1121 redemption surface | M1 variant B has no redemption/renewal/dissolution; those types persist only for storage stability (`Reservation.sol:110`,`:119`). Cross-repo claims in this file supersede any that reference a redemption surface. |
| `keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:87` | hard-codes the 2-hour `REQUEST_TIMEOUT_SAFETY_MARGIN` as a Go literal | the true source is `tbtc-v2:solidity/contracts/bridge/WalletProposalValidatorConstants.sol:14`; there is no cross-repo read that keeps the two equal, so the harness constant can silently drift from the on-chain margin (Finding: not a current violation - they agree at both pins - but the agreement is manual, see XR-P-5). |

## Findings

- **XR-G-2 (medium):** the CI ABI-drift gate's `TBTC_V2_REF` (`eec999a`,
  `keep-core:.github/workflows/client.yml:206`) predates the tbtc-v2 commit
  that added `pendingThirdPartyReanchorHolds` / the D19 caps surface, yet the
  keep-core vendored fallback and committed binding at `f99ef96e3` already
  contain that surface. The gate's whole-ABI diff on `ReservationRouter`
  therefore disagrees with itself at the committed ref. Repro: build artifacts
  from `eec999a` and run `make -C pkg/chain/ethereum/tbtc/gen
  verify-vendored-fallback`; the `ReservationRouter` diff lists
  `pendingThirdPartyReanchorHolds` as keep-core-side-only. Fix: bump
  `TBTC_V2_REF` to `6aac91585` or later, or regenerate the vendored fallback
  and binding from `eec999a`. Applied on the reservations branch: `TBTC_V2_REF`
  now names the tbtc-v2 reservations branch head, and the vendored artifacts were
  checked against the artifacts compiled there (see [approved design deltas](13-approved-design-deltas.md)).
  The row above records the state at the original pins.
- **XR-F-22 (delta):** the historical client pin does not contain D26's
  binding. `8a714f2c1` adds it, and `4b95f0a06` adds behavior tests
  for released deposits with an immutable old reveal event and a new stored vault.
  Their names and code sites appear in XR-F-22.
  The declarative ABI extension is not a completed Sepolia drill.
- **XR-P-5 (low):** the `REQUEST_TIMEOUT_SAFETY_MARGIN` agreement between the
  on-chain constant (`WalletProposalValidatorConstants.sol:14`, 2h) and the
  keep-core harness literal (`tbtc_validator_harness_test.go:87`, 7200s) is
  maintained by a hard-coded constant with no cross-repo read. A future change
  to the on-chain margin would not be caught by any test; the boundary asserts
  at `:963`/`:1161` only prove the *current* 2h value is used consistently.
  No code change required at the pins; note for the margin's next change.
- **XR-R-1 (low):** the single-substring cap matcher
  (`keep-core:pkg/tbtcpg/reservation_reanchor.go:618`, "cap exceeded")
  depends on four on-chain `require` strings sharing that exact tail
  (`Reservation.sol:887`,`:900`; `WalletProposalValidator.sol:1171`,`:1183`).
  Rewording any one of them breaks the matcher's intent (a capacity-collision
  is misread as a fatal error). No regression test asserts the shared tail.
  At `806b1106f` (D31) the two validator strings no longer exist and a
  privileged hand-off from a retiring source skips the two request-path
  `require`s; the matcher now depends on `Reservation.sol:931` and `:946`
  (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:931`,
  `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:946`). The
  real-validator harness case that pinned the wording now expects acceptance (XR-R-1).