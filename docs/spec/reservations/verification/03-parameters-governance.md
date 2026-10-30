# M1 UTXO Reservations - Parameters and Governance (PAR)

Pinned: `tbtc-v2` at `9d2086965`, `keep-core` at `f99ef96e3`. Every parameter
row cites the on-chain rule and the test (or `none`) that would fail if the
rule broke. Units are satoshi unless noted; durations in seconds. The
release-to-pool row PAR-X9 and the D26 note use the later tbtc-v2 pin
`d20cf74ba` (decision D26, commit `638c48663`); every other citation is at
`9d2086965`.

D31 (privileged hand-off plus acceptance reserve) is implemented at later
later pins: contracts, deploy scripts and SDK at `tbtc-v2@806b1106f`, client
at `keep-core@d40f6aadc`. Every D31 citation below carries that explicit
revision. Unqualified citations in this file keep resolving at the original
pins (`9d2086965`, `f99ef96e3`) and describe the original-pin state. Nothing is
deployed on Sepolia or mainnet.

IDs: PAR- (this file). Cross-references to SM-/INV-/AR- by topic are allowed;
invented IDs are not.

## Reservation parameters

`updateReservationParameters` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1362`,
governance-staged via `beginReservationParametersUpdate`/
`finalizeReservationParametersUpdate` (`BridgeGovernance.sol:1834`, `:1863`),
gated `onlyGovernance` on the router (`ReservationRouter.sol:377`). Governance
delay: `finalize*` enforces `block.timestamp - initiated >= governanceDelay`
(`BridgeGovernanceParameters.sol:1688`); the delay is `172800` (48h) on
mainnet and `60` on Sepolia (`tbtc-v2:solidity/deploy/09_deploy_bridge_governance.ts:23`).

| ID | Parameter (type, storage slot region) | Meaning | Validation rule (cite) | Who may set | Units | Retroactivity | Status | Test |
|---|---|---|---|---|---|---|---|---|
| PAR-1 | `reservationMinAmount` (uint64) | Min anchor a reservation must carry; also the re-anchor grind floor. | `reservationMinAmount > reservationTxMaxFee` at `tbtc-v2@9d2086965:solidity/contracts/bridge/Reservation.sol:1379` | governance | sats | Snapshotted into action record `minAmount` at acceptance request; not retroactive | `holds` | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new total amount cap drops below the dust threshold" (indirect); `Reservation.test.ts` guard 11 |
| PAR-2 | `reservationTxMaxFee` (uint64) | Max BTC miner fee for a 1-in-1-out lifecycle tx. | `> 0` at `tbtc-v2@9d2086965:solidity/contracts/bridge/Reservation.sol:1375` | governance | sats | Snapshotted per action (`txMaxFee`) | `holds` | `Reservation.test.ts` guards 11-14 |
| PAR-3 | `reservationTermSeconds` (uint32) | Custody term length. | `[MIN_RESERVATION_TERM, MAX_RESERVATION_TERM]` at `tbtc-v2@9d2086965:solidity/contracts/bridge/Reservation.sol:1383-1385`; also feeds the feasibility guard (PAR-X3). | governance | s | Future acceptances; existing `expiresAt` values remain snapshotted (`tbtc-v2@9d2086965:solidity/contracts/bridge/ReservationProofs.sol:557`, `:565`) | `holds-untested` for the exact bounds-revert string | none for the exact bounds-revert string; `tbtc-v2@9d2086965:solidity/test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:170` covers PAR-X3 |
| PAR-4 | `reservationDissolutionDelay` (uint32) | Delay after expiry before a Closing-wallet position becomes strand-eligible. | The setter rejects a delay above `MAX_RESERVATION_DISSOLUTION_DELAY` (730 days), the constant declared at `tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:84`; the bound is a maximum only, with no minimum. The check sits in the Bridge-side setter `Reservation.updateReservationParameters` (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1635-1639`, revert "Reservation dissolution delay out of protocol bounds") and in the governance staging step `BridgeGovernanceParameters.beginReservationParametersUpdate` (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/BridgeGovernanceParameters.sol:1667-1671`, same revert string). At the original pin the delay had no setter bound beyond uint32 and the checked addition computed eligibility (`tbtc-v2@9d2086965:solidity/contracts/bridge/ReservationProofs.sol:570`). | governance | s | Snapshotted into the action and used for `dissolutionEligibleAt`; read by stranding (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1325`, in `notifyReservationStranded`) | `holds` | `tbtc-v2@86ce2e79b:solidity/test/bridge/Bridge.ReservationDissolutionDelay.test.ts:133` ("accepts exactly 730 days"), `:140` ("reverts at 730 days + 1 second"), `:275` ("should revert at the begin stage"), `:469` ("should revert at the finalize stage (forwarded to library setter)"); `Bridge.ReservationStranding.test.ts` "notifyReservationStranded"; `Bridge.ReservationStrandingLibrary.test.ts:1060` |
| PAR-5 | `reservationMaxTotalAmount` (uint64) | Global cap on sats locked under active reservations; 0 disables. | Decision-1 invariant (PAR-X2); feasibility dust rule (PAR-X3) | governance | sats | Next request; not retroactive | `holds` | `Bridge.ReservationCaps.test.ts` "reverts when the total amount cap exceeds slot capacity" |
| PAR-6 | `maxReservationsPerWallet` (uint32) | Per-wallet position-count cap; must be > 0 while reservations enabled. | `> 0` at `tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1654` | governance | count | Prospective | `holds` | `Reservation.test.ts` "should revert when wallet reservations count exceeds maxReservationsPerWallet" |
| PAR-7 | `reservationActionTimeout` (uint32) | How long before a requested action may be reported timed out. | `> REQUEST_TIMEOUT_SAFETY_MARGIN + DEPOSIT_MIN_AGE + 1` (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1647-1651`), i.e. 2h + 2h + 1 s, so the smallest accepted value is 14,402 s. The original pin stated only `> REQUEST_TIMEOUT_SAFETY_MARGIN` (2h) at `tbtc-v2@9d2086965:solidity/contracts/bridge/Reservation.sol:1392-1396`, which a same-block reveal cannot sign inside | governance | s | Snapshotted per request | `holds` | `Bridge.ReservationCaps.test.ts` "action timeout floor" group `tbtc-v2@86ce2e79b:solidity/test/bridge/Bridge.ReservationCaps.test.ts:98-130`: `:122` ("reverts at safety margin + deposit min age + 1") and `:128` ("accepts safety margin + deposit min age + 2, leaving one signing second for a same-block reveal") |
| PAR-8 | `reservationRenewalWindowSeconds` (uint32) | Renewal window; unread in M1. | `> 0` and `< reservationTermSeconds` at `tbtc-v2@9d2086965:solidity/contracts/bridge/Reservation.sol:1388` | governance | s | design-only (m2) | `design-only` (storage completeness, no M1 reader) | `none` |
| PAR-9 | `reservationVault` (address) | The reservation vault; deposits revealed to it are reservations. | Re-pointing a non-zero vault requires `maxActiveReservations > 0` and `reservationTotalAmount == 0 && pendingReservedDeposits == 0` at `tbtc-v2@9d2086965:solidity/contracts/bridge/Reservation.sol:1404` | governance | address | Irreversible once any reservation is accepted (AR) | `holds` | `Bridge.ReservationCaps.test.ts` "bootstrap ordering" group |
| PAR-10 | `reservationRouter` (address) | delegatecall target; settable exactly once. | `reservationRouter == 0` (must be unset) and `code.length > 0` at `tbtc-v2@9d2086965:solidity/contracts/bridge/BridgeState.sol:1170` | governance | address | One-time; changing later requires a Bridge upgrade | `holds-untested` | `none` |

## Reservation caps

`updateReservationCaps` at `Reservation.sol:1494` (four arguments, selector
`0xf858967d`, at `9d2086965`), staged via
`beginReservationCapsUpdate`/`finalizeReservationCapsUpdate`
(`BridgeGovernance.sol:1893`, `:1912`), `onlyGovernance` on the router
(`ReservationRouter.sol:455`). Rows PAR-11 to PAR-14 cite the original pins.
At `tbtc-v2@806b1106f` the setter has five arguments
(`updateReservationCaps(uint64,uint64,uint32,uint32,uint32)`, selector
`0x78d76eda`; `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1687-1694`),
the router entry is `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:476-482`
and `beginReservationCapsUpdate` takes five arguments (selector `0x4c1cda81`;
`tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896-1902`).
The fifth argument is `handoffReserveWallets` (PAR-F1). The setter and event
shapes changed; the four rows below describe unchanged parameters.

Read the staged slot before staging one: `pendingReservationCapsUpdate()`
returns the five values in `beginReservationCapsUpdate`'s argument order
followed by the initiation timestamp, zero when nothing is staged, because a
second `begin` overwrites the single staged slot without a warning, so a
capacity reduction and a cap raise can silently replace each other. The view
is unrestricted, as the same values are already public through
`ReservationCapsUpdateStarted`, and it is read-only: it shows what a `begin`
is about to replace, it does not prevent the replacement
(`tbtc-v2@86ce2e79b:solidity/contracts/bridge/BridgeGovernance.sol:1999-2019`).
tbtc-v2#1179 adds that getter and the freeze pin carries it.

| ID | Parameter (type) | Meaning | Validation rule (cite) | Who may set | Status | Test |
|---|---|---|---|---|---|---|
| PAR-11 | `maxReservationsAmountPerWallet` (uint64) | Per-wallet anchor-amount cap; 0 disables. | Feasibility dust rule (PAR-X3) | governance | `holds` | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new per-wallet amount cap drops below the dust threshold" |
| PAR-12 | `reservationMaxSingleAmount` (uint64) | Per-position amount cap; 0 disables. | Feasibility dust rule (PAR-X3); feeds Decision-1 (PAR-X2) | governance | `holds` | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new single-amount cap drops below the dust threshold" |
| PAR-13 | `maxActiveReservations` (uint32) | Global open-position occupancy cap. | The setter (`updateReservationCaps`) requires `> 0` at `Reservation.sol:1502` - this is the M1 launch gate, so a zero value is rejected on set and the cap can never be "disabled" to zero on a live network. At request time `reserveAcceptanceCapacity` enforces `activeReservationsCount < maxActiveReservations` (`Reservation.sol:1006`, revert "Active reservations cap exceeded"); a zero value would block every acceptance, not merely disable a counter. The `activeReservationsCount()` view returns the tuple `(count, maxActive)` (`ReservationRouter.sol:617`-`:624`), so the SDK/consumers read the cap from the second tuple element. At `806b1106f` the setter takes five arguments and still requires `maxActiveReservations > 0` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1695-1698`, string on `:1697`); the occupancy check is unchanged and keeps its revert "Active reservations cap exceeded" (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1059`), and the hand-off reserve gate runs after it (PAR-F1, PAR-X4). | governance | `holds` | `Bridge.ReservationCaps.test.ts` "reverts when maxActiveReservations is set to zero" (`:153`); at `806b1106f` the same test is `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationCaps.test.ts:157` |
| PAR-14 | `maxThirdPartyReanchorHoldsPerWallet` (uint32) | Per-target cap on concurrently pending third-party re-anchor holds; 0 blocks third parties. | No on-chain lower bound; exempts owner/maintainer/member/governance at `Reservation.sol:866` (original pins). At `806b1106f` it is the fourth of the five setter arguments (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1692`, stored at `:1734-1735`) and the check still applies only to non-privileged callers (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:896-906`, revert "Target third-party re-anchor holds exceeded" on `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:901`). A privileged hand-off neither counts as a hold nor needs a free hold slot (PAR-F2). | governance | `holds` | `Bridge.ReservationThirdPartyReanchorHolds.test.ts` "third-party hold cap (request-time)"; at `806b1106f` also `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:583` (governance staging of the hold cap) |

## D31 hand-off reserve and privileged hand-off (implemented at `806b1106f`)

At the original pins (`9d2086965`/`7aabfc502`/`f99ef96e3`/`4b95f0a06`) neither
row below existed: the caps setter had four arguments, there was no reserve
view, and the re-anchor request and the signing validator enforced the target
count and amount caps for every caller. Rows PAR-F1 and PAR-F2 keep their IDs;
they were `design-only` there and are graded at `tbtc-v2@806b1106f` and
`keep-core@d40f6aadc` now. Per-surface plans: ``handoff-capacity-plan.md``
(D31). D28 is not part of this table (see the note after it).

| ID | Parameter / behaviour | Rule at the D31 pins | Status | Test |
|---|---|---|---|---|
| PAR-F1 | `handoffReserveWallets` (uint32, >= 1): fifth caps-setter argument, new `reservationHandoffReserveWallets()` view, acceptance reserve gate | Bullets "PAR-F1 rule" below | `holds` | Gate: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:276` (describe "hand-off reserve gate at acceptance"), `:322`, `:346`, `:379`. Zero reserve: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationCaps.test.ts:163`. Staging: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4705`, `:4789`. ABI: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts:36`, `:57` |
| PAR-F2 | Privileged hand-off cap bypass: a privileged re-anchor from a non-Live source skips the target's count and amount caps; the signing validator no longer re-checks them; third parties and Live-source governance rotation keep the caps | Bullets "PAR-F2 rule" below | `holds` | Contracts: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:359` (describe), `:427`, `:435`, `:443`, `:452`, `:466`, `:485`, `:494`, `:502`, `:553`, `:565`. Validator: `tbtc-v2@806b1106f:solidity/test/bridge/WalletProposalValidator.test.ts:4322-4382`; real validator harness `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1193`, `:1205`, `:1219` |

PAR-F1 rule (all at `tbtc-v2@806b1106f`):
- Storage: `uint32 handoffReserveWallets` in `BridgeState.Storage` (`tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeState.sol:509`), packed into slot 39 at offset 4; `__gap` stays `uint256[37]` (`tbtc-v2@806b1106f:solidity/test/fixtures/BridgeState.storageLayout.snapshot.json:556-559`).
- View: `reservationHandoffReserveWallets()` returns `uint32` (selector `0xcf098fc5`; `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:519`, interface `tbtc-v2@806b1106f:solidity/contracts/bridge/IReservationBridge.sol:618`). `reservationCaps()` stays a four-value view (`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:495`).
- Setter: five-argument `updateReservationCaps` (selector `0x78d76eda`; `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1687-1694`) rejects zero with "Hand-off reserve must be greater than zero" (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`), stores the value (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1736`) and emits the five-field `ReservationCapsUpdated` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1738-1744`; topic0 `0x964ef9bd68fafce7168d52b1d95f0da86e3be014f9843a793b021652e4e28daa`, was `0x11af1b44d097c7893ef437ea940dea2e21b5c822807b98a3c4d4d9f7f0f11e5d` with four fields). The router entry stays `onlyGovernance` (`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:476-482`).
- Staging: `beginReservationCapsUpdate` takes five arguments (selector `0x4c1cda81`, was `0x3684e886`; `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896-1902`); the staged value is a struct field carried by the started event (`tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernanceParameters.sol:126`, `:375`; stored `:1741`, cleared `:1772`); `finalizeReservationCapsUpdate` (selector `0x7d87eff2`, unchanged) passes `staged.newHandoffReserveWallets` (`tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1926`).
- Gate: `reserveAcceptanceCapacity` requires `activeReservationsCount + 1 <= max(0, L - R) * N` after the global cap (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1052-1077`, revert "Hand-off reserve exceeded" on `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`). Sizing and limits: PAR-X4.
- Sepolia value: R = 2 (D30/D31), L = 4, N = 1: budget 2, equal to max active 2 (value sets below). Nothing is deployed: a read-only check on 2026-10-13 found `reservationCaps()` reverting on the Sepolia Bridge (orchestrator-recorded).
- Deploy: the local caps step passes `handoffReserveWallets` = 1 (`tbtc-v2@806b1106f:solidity/deploy/97_set_reservation_parameters.ts:141`); the mainnet calldata script stages the five-argument call (`tbtc-v2@806b1106f:solidity/deploy/98_generate_reservation_mainnet_calldata.ts:163-178`), reads `RESERVATION_HANDOFF_RESERVE_WALLETS` and rejects values <= 0 (`tbtc-v2@806b1106f:solidity/deploy/98_generate_reservation_mainnet_calldata.ts:506-523`); test "should throw when RESERVATION_HANDOFF_RESERVE_WALLETS is zero" (`tbtc-v2@806b1106f:solidity/test/deploy/98_generate_reservation_mainnet_calldata.test.ts:486`). The storage field reads 0 on an upgraded Bridge until the first five-argument caps update (governance step 5 below).
- SDK: feasibility reason `handoff-reserve-exceeded` (`tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:255-262`), acceptance preflight failure (`:567-574`), and `handoffReserveExceeded`, which mirrors the formula but returns false when `maxReservationsPerWallet` is 0 although on-chain N = 0 would reject every acceptance (`tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:707-722`). The view is read at `tbtc-v2@806b1106f:typescript/src/lib/ethereum/reservations.ts:333`. Tests: `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:414` (describe "hand-off reserve gate"), `:479`, `:542`, `:559`.
- Client bindings: `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:501` (reserve view caller) and `:1003` (five-argument `UpdateReservationCaps`). No production keep-core code calls the setter (governance-only).

PAR-F2 rule (all at `tbtc-v2@806b1106f` unless a `keep-core@d40f6aadc` path is named):
- The router passes `msg.sender == governance` as the `privileged` flag (`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:255`, call at `:246-259`). Only that flag bypasses the cooldown ("Reanchor cooldown in effect", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:789`).
- Privilege is the governance flag, the reservation owner, an SPV maintainer, or a proven member of the source wallet (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:814-818`). A non-empty member proof that fails reverts "Caller is not a member of the source wallet" (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:811`; test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:235`).
- Cap bypass: `skipTargetCaps = isPrivileged && sourceState != Live` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`). It skips the count cap ("Wallet reservations cap exceeded", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:931`) and the amount cap ("Wallet reserved amount cap exceeded", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:946`); the target count and amount are still added on every path (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:934-935`, `:949-950`).
- Kept caps: only the governance flag may rotate a Live wallet's anchor ("Only governance can rotate a Live wallet's anchor", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:827`) and it keeps both target caps (test `:485`). Third parties keep the caps and the hold cap (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:896-906`; tests `:466`, `:502`). A hold on the target's ordinary slot does not block a privileged hand-off (test `:553`); a member succeeds after the hold times out (`:565`).
- Signing validator: the doc block states that target count and amount capacity is enforced once at request time and not repeated (`tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1092-1098`); the target must still be Live ("Target wallet must be in Live state", `tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1159`). At `9d2086965` the validator re-checked both caps (VF-01; EC-10).
- Client: a fresh re-anchor request gathers a member proof (`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:262-290`); target search prefers a candidate with headroom and falls back to the least-loaded eligible Live wallet only for privileged member requests (`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:1153-1241`). Tests: `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1479` (`TestReservationReanchorTask_MemberFallsBackToLeastLoadedFullWallet`), `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1611` (`TestReservationReanchorTask_NonMemberStillSendsNothingWhenFull`).
- Not changed: timeouts release target capacity and third-party holds while the source keeps capacity until settlement; hand-offs above the cap concentrate anchors on one wallet. This closes the D19 limit that the original pins still carry (a full target blocked every hand-off).
- Integration evidence (orchestrator-recorded; see [approved design deltas](13-approved-design-deltas.md)): mainnet fork a local mainnet-fork run (no pin), 34/34; Sepolia script run at tbtc-v2 `4306c2345`, fork mode 24 pass / 3 off-chain skips, live mode on a local Sepolia fork with reserve 2 read back.

## D28 keeper and monitoring parameters

D28 adds no on-chain parameter. Its read-only observer and alert rules use later pins.
The keeper runs through the existing maintainer role; SM-C16 records its deployment boundary.

| ID | Parameter or rule | Implementation | Status | Test |
|---|---|---|---|---|
| PAR-K1 | Seven-day late-proof risk window; one-day unrequested-deposit risk window | Constants `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:10-23`; inclusive counts at `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:100-104` and `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:265-270`. These warn; they do not extend contract deadlines. | holds | `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:288-351`, `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:406-474` |
| PAR-K2 | `RESERVATION_KEEPER_OBSERVATION_MAX_AGE_SECONDS` | Default 1800, positive safe integer only (`tbtc-v2@8eb26c2e2:monitoring/src/context.ts:72-86`). A deviation greater than this limit, behind or ahead of latest chain time, produces Critical (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:192-225`). | holds | Equality, one-second excess and catch-up cases: `tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:259-328` |

The keeper polls once per minute (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_wiring.go:58-66`, `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_wiring.go:668-671`).
Wallet-state caching has a separate ten-minute refresh interval (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:33-38`).
Neither interval changes on-chain signing, refund or late-proof deadlines.

## Deposit / moving-funds parameters reservations depend on

Reservations read these at request and settle time. Setters are on
`Bridge`/`BridgeGovernance`, gated by the feasibility guard when a vault is
set (PAR-X3).

| ID | Parameter | Meaning | Reservations dependency | Status | Test |
|---|---|---|---|---|---|
| PAR-15 | `depositDustThreshold` (uint64) | Min deposit size. | A reserved reveal carries >= dust; every non-zero reservation cap must be >= dust (feasibility). `BridgeState.sol:698` | `holds` | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new dust threshold exceeds the single-amount cap" |
| PAR-16 | `depositRevealAheadPeriod` (uint32) | Min window between reveal and refund locktime. | Reveal gate: a reserved reveal must satisfy `now + revealAhead <= refundDeadline`; the deposit-reveal refund cap for reservations is `term + DEPOSIT_REFUND_SAFETY_MARGIN` (`Deposit.sol:244`). `BridgeState.sol:690` | `holds` | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new reveal-ahead period exceeds the term plus margin" |
| PAR-17 | `depositTxMaxFee` (uint64) | Max BTC fee per swept deposit. | `depositDustThreshold > depositTxMaxFee` (`BridgeState.sol:627`). | `holds` | `Bridge.ReservationFeasibilityGuard.test.ts` "accepts every feasible deposit parameter update" |
| PAR-18 | `depositTreasuryFeeDivisor` (uint64) | Treasury fee for ordinary deposits. | Reservation claims are minted gross; the deposit treasury fee is NOT charged on reservations (`Deposit.sol:398`, `ReservationProofs.sol:611`). | `holds` (by-design, no reader) | `none` (reservation path ignores it by design) |
| PAR-19 | `walletCreationPeriod` (uint32) | Wallet creation period (moving funds). | keep-core re-anchor target-age gate uses it (`keep-core:pkg/tbtcpg/reservation_reanchor.go:1064`, `reservation_target_age.go:20`). | `holds` | `keep-core:pkg/tbtcpg/reservation_reanchor_custody_test.go` "SkipsYoungResumeTarget" |
| PAR-20 | `walletClosingPeriod` / `movingFundsTimeout` (uint32) | Wallet lifecycle timers. | Closing/termination paths that strand or re-anchor reservations. `Wallets.sol:377`, `:471`. | `holds` | `Bridge.ReservationStranding.test.ts` "real moving-funds-timeout call chain" |
| PAR-21 | `liveWalletsCount` (uint32, derived) | Count of Live wallets. | Saturation sizing (PAR-X4): at the original pins `maxActive` must sit below `liveWalletsCount * maxReservationsPerWallet`. At `806b1106f` the acceptance reserve gate reads it as L (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1068`; storage `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeState.sol:269`). | `holds` (derived, not settable; read by the gate at `806b1106f`) | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:346` (bound tightens when a Live wallet leaves Live) |

## Cross-parameter rules

### PAR-X1 Re-anchor min-amount floor
At re-anchor request, the anchor must stay strictly above `minAmount + txMaxFee`
(`Reservation.sol:854`); this caps cumulative fee grinding across hops.
Status `holds`. Test: `Bridge.ReservationSettlement.test.ts`
"bounds the grind at a request-time amount floor, not the settlement-time dust floor".
The acceptance floor `deposit.amount > minAmount + 2 * txMaxFee`
(`tbtc-v2@dc5c2ed71:solidity/contracts/bridge/Reservation.sol:577-584`)
makes every new position clear this floor for its first hop. Test:
`tbtc-v2@dc5c2ed71:solidity/test/bridge/Reservation.test.ts:369` ("guard 11").

### PAR-X2 Decision-1 relational invariant
`reservationMaxTotalAmount <= maxActiveReservations * reservationMaxSingleAmount`
(skipped when either operand is 0). Enforced at BOTH setters via
`validateReservationCapsInvariant` (`Reservation.sol:1322`, callers `:1448`
and `:1511`). Status `holds`. Test: `Bridge.ReservationCaps.test.ts`
"reverts when the total amount cap exceeds slot capacity" and "reverts when
lowering the position cap below the standing total".

### PAR-X3 Feasibility guard (`assertReservedDepositsFeasible`)
While a vault is set (`reservationVault != 0`), no combination of a non-zero
amount cap below the dust threshold, and no `depositRevealAheadPeriod` longer
than `term + DEPOSIT_REFUND_SAFETY_MARGIN` (24h) is permitted; the check runs
on `updateDepositParameters` (`BridgeState.sol:643`) and on both reservation
setters (`Reservation.sol:1427`, `:1521`). Rationale: a reserved reveal
carries >= dust and its refund locktime is bounded by `term + 24h`, so any
cap below dust is unreachable and a longer reveal-ahead empties the window.
Status `holds`. Test: `Bridge.ReservationFeasibilityGuard.test.ts`
("reverts when the new term leaves no reveal window", "reverts when the new
single-amount cap drops below the dust threshold", and the `updateDepositParameters`
group). The keep-core off-chain mirror is the gauge
`reservation_params_infeasible`
(`keep-core:pkg/clientinfo/performance.go:797`,
`pkg/maintainer/spv/reservation_params_feasibility.go:37`). Test:
`keep-core:pkg/maintainer/spv/reservation_params_feasibility_test.go`
"TestReservationParamsInfeasibleRevealWindow". Status `holds`.

### PAR-X10 Deploy-script composite preflight (script-only)
Deploy script `98_generate_reservation_mainnet_calldata.ts` checks one composite
bound before it generates calldata, and this check is a script-only preflight,
not the on-chain feasibility guard of PAR-X3. When the on-chain
`depositRevealAheadPeriod` is non-zero it takes
`max(revealAheadPeriod + REVEAL_DRIFT_SECONDS, RES_ACTION_TIMEOUT + DEPOSIT_REFUND_SAFETY_MARGIN + REVEAL_DRIFT_SECONDS)`
as the lowest reveal-to-refund distance, derives a minimum feasible term from it,
and throws when `RESERVATION_TERM_SECONDS` is below that term
(`tbtc-v2@86ce2e79b:solidity/deploy/98_generate_reservation_mainnet_calldata.ts:784-829`).
The on-chain guard remains PAR-X3. Status `holds`.
Test:
`tbtc-v2@86ce2e79b:solidity/test/deploy/98_generate_reservation_mainnet_calldata.test.ts:545`
(reveal-ahead period rejected) and `:549` (longest accepted reveal-ahead period).

### PAR-X4 Saturation sizing
At the original pins (`9d2086965`): `maxActiveReservations` is a governance-time cap on `activeReservationsCount`,
sized below the slot floor `liveWalletsCount * maxReservationsPerWallet`
(`BridgeState.sol:436`). At saturation a re-anchor has no target and an
anchored wallet cannot close; the cap turns that cliff into a revert. This was
a runbook constraint, not an on-chain check - nothing enforces
`maxActive <= liveWalletsCount * maxReservationsPerWallet` in a setter. It was
graded `holds-untested` there (documented, off-chain only; the calldata script
checked it, not a test).

At `tbtc-v2@806b1106f` the acceptance request also enforces a count-admission
bound on-chain (D31, design A). `reserveAcceptanceCapacity` applies the global
cap first ("Active reservations cap exceeded",
`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1059`), then requires
`P + 1 <= max(0, L - R) * N`, where P = `activeReservationsCount` (pending
acceptances included), L = `liveWalletsCount`, R = `handoffReserveWallets`
(PAR-F1) and N = `maxReservationsPerWallet`
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1068-1077`, revert
"Hand-off reserve exceeded" on `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`).
Its only caller is `requestReservationAcceptance`
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:643`). `L <= R` rejects every
acceptance. The bound is checked only at the acceptance request: never at
proof time, at re-anchor (which leaves P unchanged) or on settlement. A Live
wallet leaving Live lowers L and tightens the bound for later acceptances; it
does not evict open positions. The bound does not guarantee a Live target, a
signing quorum, proof delivery, amount headroom or fee room, and hand-offs
above the ordinary cap concentrate anchors on one wallet (PAR-F2). The setters
still do not check `maxActive` against `L * N`; the manual sizing above remains
the way to choose `maxActive`, and the gate bounds admission regardless.
Sepolia result: L = 4 (readings of 2026-10-09 and 2026-10-13), R = 2, N = 1
gives (4 - 2) * 1 = 2, equal to max active 2 (value sets below).
Status `holds` for the on-chain count-admission bound; the sizing of `maxActive` in the setter stays a governance choice.

Test: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322` (reverts
past `(L - R) * N` with max-active room), `:346` (tightens when a Live wallet
leaves Live), `:379` (blocks every acceptance while `L <= R`); SDK mirror
`tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:414`.

### PAR-X5 Action-timeout vs REQUEST_TIMEOUT_SAFETY_MARGIN
`reservationActionTimeout > REQUEST_TIMEOUT_SAFETY_MARGIN + DEPOSIT_MIN_AGE + 1`
(2h + 2h + 1 s) at `tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1647-1651`,
so the wallet validator always has a signing window before the action can time
out, including for a deposit revealed in the same block as the request: the
first admissible signing timestamp is `revealedAt + DEPOSIT_MIN_AGE + 1` and the
last is `timeoutAt - REQUEST_TIMEOUT_SAFETY_MARGIN - 1`, because the validator's
bound is strict (`block.timestamp < action.timeoutAt - REQUEST_TIMEOUT_SAFETY_MARGIN`,
`tbtc-v2@86ce2e79b:solidity/contracts/bridge/WalletProposalValidator.sol:1000-1007`)
and proposal timestamps are integer seconds. The original pin required
only `> REQUEST_TIMEOUT_SAFETY_MARGIN` (`Reservation.sol:1393`), which left a
same-block reveal without a window. The re-anchor request additionally requires
a non-empty signing window against the safety-margined deadline
(`Reservation.sol:833`). Status `holds`. Test:
`tbtc-v2@86ce2e79b:solidity/test/bridge/Bridge.ReservationCaps.test.ts:98-130`
(the action-timeout floor cases: `:116` rejects the old floor `safety margin + 1`,
`:122` rejects `safety margin + deposit min age + 1`, `:128` accepts `+ 2`);
`Reservation.test.ts` "action timeout is too short for signing window" covers
the request-time window.

### PAR-X6 Reveal-time refund-deadline cap for reservations
A reserved deposit's refund locktime must satisfy
`refundDeadline <= now + reservationTermSeconds + DEPOSIT_REFUND_SAFETY_MARGIN`
(`Deposit.sol:244`). This bounds the "never-anchored" griefing window: stale
cleanup becomes eligible after the deadline, but a caller must submit
`notifyStaleReservedDeposit`. Status `holds`. Test:
`Bridge.ReservationStranding.test.ts` "notifyStaleReservedDeposit" group.

### PAR-X7 Fee settings (vault)
`initiationFeeBps` (default 40, cap `MAX_FEE_BASIS_POINTS`=500) and
`feeReserveTarget` are vault-owned. `financeInKindFee` records any shortfall as
`inKindFeeDebtSat` (`ReservationVault.sol:229`). A settlement against an
untrusted vault bypasses the fee entirely (direct credit). Status `holds`.
Test: `tbtc-v2:solidity/test/vault/ReservationVault.test.ts`
("receiveBalanceIncrease", "financeInKindFee", "sweepFees" groups).

### PAR-X8 Re-anchor min amount floor vs settle dust floor
Request-time floor is `minAmount + txMaxFee` (PAR-X1); settlement-time dust
floor is `newAnchorAmount > txMaxFee` (`ReservationProofs.sol:695`). The two
are distinct and the request-time floor is the binding one; a re-anchor may
settle between `txMaxFee` and `minAmount + txMaxFee` if the fee is bounded
(snapshotted), but the request-time check is what caps the grind.
Status `holds`. Test: `Bridge.ReservationSettlement.test.ts` PAR-X1 group.

### PAR-X9 Release-to-pool refund-margin guard (decision D26, at `d20cf74ba`)
`releaseReservedDepositToPool` re-routes a revealed, never-requested reserved
deposit back to the ordinary pool only while the snapshotted refund deadline
still leaves the 24h `DEPOSIT_REFUND_SAFETY_MARGIN` open in the future
(`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1419`), and the
re-route target must be the zero address or a trusted vault that is not the
reservation vault itself (`:1430`). This is a runtime depositor call, not a
governance step: it introduces no new on-chain parameter and no new
`BridgeGovernance` begin/finalize pair. It does add one cross-parameter
relationship: the ordinary-sweep refund margin the validator enforces is the
same 24h constant the release re-checks, so a deposit whose margin is
exhausted cannot be released. After its refund deadline, an eligible caller
can submit `notifyStaleReservedDeposit` (PAR-X6). Status `holds`. Test:
`tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts`
"releaseReservedDepositToPool guards" ("reverts when the refund margin for an
ordinary sweep is exhausted", "reverts when the release vault is the
reservation vault", "reverts when the release vault is not trusted").

## Governance step order and irreversibility

Deploy-ordering source: `tbtc-v2:solidity/docs/RESERVATION_CAPS_DEPLOYMENT.md`
and the Sepolia runbook
(`tbtc-v2@52f4f3005:solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md` @ `52f4f3005`).
Step order:

1. **Caps before parameters.** `beginReservationCapsUpdate` ->
   `finalizeReservationCapsUpdate` MUST finalize before
   `beginReservationParametersUpdate` -> `finalizeReservationParametersUpdate`
   on bootstrap, so that when the parameters write runs its Decision-1 check
   the slot-capacity operands are real. Running parameters first is accepted
   on bootstrap only because both caps are still 0 (the check short-circuits);
   a later caps call then reverts if it cannot accommodate the committed total.
   `BridgeGovernance.sol:1880` doc; `docs/RESERVATION_CAPS_DEPLOYMENT.md`.
   Status `holds`. Test: `Bridge.ReservationCaps.test.ts` "bootstrap ordering".

2. **Vault fee-reserve target before activation.** `updateFeeReserveTarget`
   must run before `setVaultStatus(vault, true)`, else `sweepFees` can drain
   the in-kind fee reserve that M1 re-anchor settlement needs
   (`ReservationVault.sol:302`, `docs/RESERVATION_CAPS_DEPLOYMENT.md`
   "Fee-Reserve Target Governance Step"). Status `design-only` (runbook
   discipline, not on-chain enforced). Test: `none`.

3. **C-4 activation gate.** keep-core activates reservations only at block B
   (`reservationsActivationBlocks`); `setVaultStatus(vault, true)` must execute
   at or after B. Ordering: ship the keep-core release first, then choose B,
   then activate on-chain. A wrong order (on-chain first, B not reached)
   leaves a gap where reserved deposits can be revealed and accepted but no
   client coordinates the anchor. This gate is runbook discipline; nothing
   on-chain enforces it. `keep-core:pkg/tbtc/coordination.go:110`,
   `:721` (checklist gate), `:754` (`ReservationsActive` flag). Status
   `holds-untested` (code is present; the ordering itself is process). Test:
   `keep-core:pkg/tbtc/coordination_test.go` "TestReservationsActivationBlock_SanityCheck".

4. **D26 release-to-pool needs no governance step.** At `d20cf74ba` the
   release path is a permissionless-by-depositor runtime call
   (`releaseReservedDepositToPool`, SM-D5). It adds no new on-chain
   parameter, no new governance begin/finalize pair, and no new activation
   gate. It does not change the C-4 ordering: keep-core still activates at
   block B, and the release path is reachable whenever a reserved deposit
   exists. It interacts with step 1 only in that, because release decrements
   `pendingReservedDeposits` (SM-S4), a later re-point of `reservationVault`
   becomes possible sooner (the quiescence precondition
   `reservationTotalAmount == 0 && pendingReservedDeposits == 0` clears).
   Status `holds`. Test: `tbtc-v2@d20cf74ba:Bridge.ReservationReleaseToPool.test.ts`
   "lets governance re-point the reservation vault once the last pending
   reserved deposit is released".

5. **D31 hand-off reserve is set by the caps update.** At `806b1106f` the
   first caps update carries `handoffReserveWallets` as its fifth argument and
   rejects zero ("Hand-off reserve must be greater than zero",
   `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`); the deploy steps set it
   before activation (local step
   `tbtc-v2@806b1106f:solidity/deploy/97_set_reservation_parameters.ts:141`, mainnet calldata
   `tbtc-v2@806b1106f:solidity/deploy/98_generate_reservation_mainnet_calldata.ts:163-178`). On a
   Bridge upgraded to this code the storage field reads 0 until that first
   five-argument update (the setter enforces R > 0 only when it runs), so the
   gate then admits up to `L * N` positions. Status `holds-untested` for the
   pre-update zero state (derived from the code; no test pins it); the zero
   rejection and the staging are pinned under PAR-F1.

### Irreversible steps
- `reservationVault` re-pointing after the first acceptance (PAR-9) is
  irreversible under variant B - it can only be reset to 0 when
  `reservationTotalAmount == 0 && pendingReservedDeposits == 0`, which M1
  reaches only via timeout/stranding (no voluntary exit).
- `setReservationRouter` is one-time (PAR-10); replacement requires a Bridge
  upgrade.
- `maxActiveReservations` and amount caps are re-settable by another
  begin/finalize pair, but lowering them can strand capacity (Decision-1).

## Value sets

### Sepolia (D30)
Source: `tbtc-v2@52f4f3005:solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md`
(@ `52f4f3005`), Sepolia step 6. Live Sepolia readings used: dust 1,000,000,
`depositRevealAheadPeriod` 1,296,000 (15d), `liveWalletsCount` 4.

| Parameter | Sepolia value |
|---|---|
| `maxReservationsAmountPerWallet` | 2,000,000 |
| `reservationMaxSingleAmount` | 2,000,000 |
| `maxActiveReservations` | 2 |
| `maxThirdPartyReanchorHoldsPerWallet` | 1 |
| `reservationMinAmount` | 10,000 |
| `reservationTxMaxFee` | 1,000 |
| `reservationTermSeconds` | 7,776,000 (90d) |
| `reservationDissolutionDelay` | 86,400 |
| `reservationMaxTotalAmount` | 4,000,000 |
| `maxReservationsPerWallet` | 1 |
| `reservationActionTimeout` | 86,400 |
| `reservationRenewalWindowSeconds` | 86,400 |
| `handoffReserveWallets` | 2 (D30/D31; set by the five-argument caps update, PAR-F1) |

Rule checks (Sepolia, against live dust 1,000,000 / revealAhead 1,296,000 /
liveWalletsCount 4):

| Rule | Eval | Pass |
|---|---|---|
| PAR-X2 `total <= maxActive * single` | 4,000,000 <= 2 * 2,000,000 = 4,000,000 | yes (exactly equal) |
| PAR-X3 dust (single/perWallet/total >= 1,000,000) | 2,000,000 / 2,000,000 / 4,000,000 all >= 1,000,000 | yes |
| PAR-X3 revealAhead <= term + 24h | 1,296,000 <= 7,776,000 + 86,400 = 7,862,400 | yes |
| PAR-6 / PAR-13 positivity | maxReservationsPerWallet=1>0, maxActive=2>0 | yes |
| PAR-X1 min > fee | 10,000 > 1,000 | yes |
| PAR-X4 nominal count headroom (manual sizing, original pins) | 2 < 4 * 1 = 4 | Two nominal slots before additional holds or retirements; not a permanent guarantee |
| PAR-X4 reserve gate at `806b1106f` | `(L - R) * N = (4 - 2) * 1 = 2`; max active 2 | yes (exactly equal): the on-chain budget equals max active, so a Live wallet leaving Live lowers the budget below max active |
| PAR-3 term bounds [90d,730d] | 90d at the min bound | yes |
| PAR-7 actionTimeout > 2h | 86,400 > 7,200 | yes |

All D30 Sepolia values pass every rule. `maxActiveReservations = 2` against
4 live wallets is a headroom decision (D30), not a guarantee: if a wallet
retires the floor shrinks and governance must re-evaluate (PAR-X4). With D31
(`handoffReserveWallets` = 2, PAR-F1) the acceptance request enforces the same
budget on-chain (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1068-1077`): at
L = 4 it is 2, equal to max active 2; at L = 3 it would be 1 (derived from the
formula, not a recorded run).

### Mainnet fork dry-run
Source: `tbtc-v2@52f4f3005:solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md`
pre-flight step 1 (the local mainnet-fork run values). They are dry-run inputs,
not live values: no reservation parameter is set on mainnet yet, since the
feature's mainnet ceremony has not run. The one live mainnet number they are
fitted to is the deposit reveal-ahead period, 150 days
(`tbtc-v2@52f4f3005:solidity/deploy/14_set_deposit_parameters.ts:13-14`),
which puts a floor on any feasible term (PAR-X3). The recorded launch decision
is a 365-day term and a 365-day dissolution delay (`roadmap.md:408-409`;
`requirements.md` §9), so this table is not a record of the launch values.

| Parameter | Mainnet fork dry-run value |
|---|---|
| `maxReservationsAmountPerWallet` | 2,000,000 |
| `reservationMaxSingleAmount` | 2,000,000 |
| `maxActiveReservations` | 5 |
| `maxThirdPartyReanchorHoldsPerWallet` | 1 |
| `reservationMinAmount` | 10,000 |
| `reservationTxMaxFee` | 1,000 |
| `reservationTermSeconds` | 12,960,000 (150d) |
| `reservationDissolutionDelay` | 86,400 |
| `reservationMaxTotalAmount` | 10,000,000 |
| `maxReservationsPerWallet` | 1 |
| `reservationActionTimeout` | 86,400 |
| `reservationRenewalWindowSeconds` | 86,400 |

Rule checks (against the live dust threshold and the 150-day reveal-ahead period):

| Rule | Eval | Pass |
|---|---|---|
| PAR-X2 `total <= maxActive * single` | 10,000,000 <= 5 * 2,000,000 = 10,000,000 | yes (exactly equal) |
| PAR-X3 revealAhead (150d = 12,960,000) <= term + 24h | 12,960,000 <= 12,960,000 + 86,400 = 13,046,400 | yes (a 150-day term satisfies the bound; the launch decision is 365 days) |
| PAR-X3 dust | single/perWallet 2,000,000 and total 10,000,000 all >= dust | yes |
| PAR-X4 nominal headroom `5 < liveWalletsCount * 1` | Requires at least six Live wallets for this nominal inequality | Read the actual fork/live count; no exercised headroom assertion is claimed here |
| PAR-X4 reserve gate at `806b1106f`, any R >= 1 | `(L - R) * 1 >= 5` needs `L >= R + 5` | R is an input to the calldata script (`tbtc-v2@806b1106f:solidity/deploy/98_generate_reservation_mainnet_calldata.ts:506-508`); no mainnet R is recorded here. Check against the live count at ceremony time |
| PAR-3 term bounds | 150d within [90d,730d] | yes |

All mainnet dry-run values pass PAR-X2/X3. The nominal PAR-X4 inequality
passes only if the live
`liveWalletsCount` at ceremony time is >= 6. The Sepolia runbook's warning
"if `liveWalletsCount` is below 4, lower max active and max total to match"
applies to Sepolia. At the original pins the analogous mainnet check was a
runbook precondition, not a code gate. At `806b1106f` admission is also bounded
on-chain by the reserve gate (PAR-X4), but the setters still do not check
`maxActive` sizing.

### Constraints mainnet values must meet
A mainnet value set is valid iff it satisfies, simultaneously:
1. PAR-X2: `reservationMaxTotalAmount <= maxActiveReservations * reservationMaxSingleAmount`.
2. PAR-X3 dust: every non-zero amount cap >= `depositDustThreshold`.
3. PAR-X3 reveal: `depositRevealAheadPeriod <= reservationTermSeconds + 86400`
   (on mainnet the 150-day reveal-ahead forces a 150-day minimum term).
4. PAR-X4: `maxActiveReservations < liveWalletsCount * maxReservationsPerWallet`
   (runbook precondition at the original pins; no setter checks it). At
   `806b1106f` admission is also bounded on-chain by
   `activeReservationsCount + 1 <= max(0, liveWalletsCount - handoffReserveWallets) * maxReservationsPerWallet`
   (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1068-1077`), so every position
   below `maxActive` is only admissible while `(L - R) * N` leaves room.
5. PAR-6: `maxReservationsPerWallet > 0`; PAR-13: `maxActiveReservations > 0`;
   PAR-F1: `handoffReserveWallets > 0` (`806b1106f` caps setter).
6. PAR-3: term within [90d, 730d]; PAR-7: `reservationActionTimeout > REQUEST_TIMEOUT_SAFETY_MARGIN + DEPOSIT_MIN_AGE + 1` (2h + 2h + 1 s).

## Findings
- None `violated`. At the original pins PAR-X4 (saturation sizing) was a
  runbook constraint with no on-chain enforcement and no test. At `806b1106f`
  the count-admission bound is enforced and tested (PAR-X4, PAR-F1); choosing
  `maxActiveReservations` stays a governance decision and the risk is tracked
  in AR.

## Doc drift

Drift against this directory's prior specs; line numbers refer to those files.

| Doc path:line | What it says | What the code does |
|---|---|---|
| `requirements.md:719` | `reservationMaxTotalAmount` "Local/test: 10,000,000 sats" | 10,000,000 is the mainnet dry-run value; local test fixtures set caps to the dust threshold (see `9d2086965` commit subject) |
| `requirements.md:725` | `maxActiveReservations` "Local/test: 100" | 100 is the local/test caps ceremony value: `tbtc-v2@86ce2e79b:solidity/deploy/97_set_reservation_parameters.ts:138` passes `BigInt("100")` to `beginReservationCapsUpdate` (`:126`), next to the comment that these local/test values intentionally diverge from the mainnet launch values that `tbtc-v2@86ce2e79b:solidity/deploy/98_generate_reservation_mainnet_calldata.ts` derives from its own environment. It is not a contract default: no `Reservation` setter writes 100 |
| `requirements.md:720` | `maxReservationsPerWallet` "hard-enforced == 1" by the mainnet script | On-chain only requires `> 0` (`Reservation.sol:1398`); the `== 1` pin is a deploy-script check, not a require |
| `requirements.md:723` | `maxReservationsAmountPerWallet` "Local/test: 1,000,000 sats" | Consistent with the `9d2086965` local-dust-threshold caps |
| `m1-b-implementation.md:246-252` | cites `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:925-930` for occupancy, `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1359-1361`/`tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1382-1385` for the gate, `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1217-1231` for the invariant | Actual: occupancy check in `reserveAcceptanceCapacity` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1005` (called from `tbtc-v2:solidity/contracts/bridge/Reservation.sol:621`), launch gate `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1501` (caps) / `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1405` (parameters), invariant `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1322` with callers `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1448`/`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1511` |
| `requirements.md:533` (FR-12) | cites `tbtc-v2@9d2086965:solidity/contracts/bridge/Reservation.sol:1376-1406` for the nine `updateReservationParameters` checks | The check block is `:1374`-`:1460`; `:1376-1406` is only the first five requires |
| `requirements.md:435`, `requirements.md:908`, `requirements.md:909` | cite the Decision-1 invariant and the launch gate at `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1220-1406`, `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1215-1232`, `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1383-1385` | The invariant function is `validateReservationCapsInvariant` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1322` (callers `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1448`, `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1511`); the caps launch gate is `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1501`; `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1383-1385` is the term-bounds require, not the launch gate |
