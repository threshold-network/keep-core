# Reservations - traceability matrix

This matrix maps every canonical review ID to its owning entry, for the prefixes SM, PAR, INV, US, EC, FM, DR, TM, XR and AR.
The VF- findings are owned by [finding adjudication](12-finding-adjudication.md) and have no row here.
Each ID appears once. A heading entry owns its ID; a summary-table row for the same ID must record the same status.
Rows are derived from the owning entries: the title, the most cautious recorded status, and the test or rehearsal field.
When a table has no test column, the evidence cell uses the status cell if that cell names a test.
`model` denotes a descriptive row; `pointer` refers to other items and adds no coverage.
No status in this matrix asserts that the named tests ran during this documentation task.

## Coverage summary

| Chapter | Items | holds | holds-untested | design-only | unknown | violated | model | pointer |
|---|---|---|---|---|---|---|---|---|
| System model | 58 | 36 | 5 | 2 | 0 | 0 | 15 | 0 |
| Invariants | 44 | 32 | 11 | 1 | 0 | 0 | 0 | 0 |
| Parameters and governance | 34 | 31 | 2 | 1 | 0 | 0 | 0 | 0 |
| User stories | 74 | 59 | 14 | 0 | 0 | 0 | 0 | 1 |
| Edge cases | 24 | 19 | 5 | 0 | 0 | 0 | 0 | 0 |
| Failure modes | 14 | 11 | 2 | 1 | 0 | 0 | 0 | 0 |
| Disaster recovery | 12 | 4 | 5 | 2 | 1 | 0 | 0 | 0 |
| Threat model | 20 | 8 | 9 | 1 | 2 | 0 | 0 | 0 |
| Cross-repo contract | 63 | 35 | 26 | 0 | 0 | 1 | 0 | 1 |
| Accepted risks | 16 | 10 | 4 | 2 | 0 | 0 | 0 | 0 |
| **Total** | 359 | 245 | 83 | 10 | 3 | 1 | 15 | 2 |

## Review queue rule

Start with the `violated`, `unknown`, `design-only` and `holds-untested` rows below.
Read [finding adjudication](12-finding-adjudication.md) before treating a draft observation as a defect.
[Approved design deltas](13-approved-design-deltas.md) separate the D31 and D28 target design from the inspected implementation.
`none recorded` means that the owning entry has no test or rehearsal field. It does not mean that the repository has no related tests.

## Item matrix

### System model

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [SM-A1](01-system-model.md) | Depositor (the `deposit.depositor` address that revealed a reserved deposit) | holds | none recorded |
| [SM-A2](01-system-model.md) | Reservation owner (in M1 this is the depositor, set at acceptance settlement: `reservation.owner = deposit.depositor`) | holds | none recorded |
| [SM-A3](01-system-model.md) | Wallet members / operators (source-wallet signing-group members) | holds | none recorded |
| [SM-A4](01-system-model.md) | SPV maintainer (approved on-chain via `Bridge.setSpvMaintainerStatus`) | holds | none recorded |
| [SM-A5](01-system-model.md) | Governance (the `BridgeGovernance` contract, owner via `onlyOwner`; the `governance` address is a contract, never an EOA) | holds | none recorded |
| [SM-A6](01-system-model.md) | Vault owner (the `Ownable` owner of `ReservationVault`, governance) | holds | none recorded |
| [SM-A7](01-system-model.md) | Third party (any address not owner/maintainer/governance/member) | holds | none recorded |
| [SM-A8](01-system-model.md) | keep-core node (orchestrator/tbtcpg maintainer + SPV maintainer) | holds | none recorded |
| [SM-C1](01-system-model.md) | Bridge proxy | model | none recorded |
| [SM-C10](01-system-model.md) | C-4 activation gate | model | none recorded |
| [SM-C11](01-system-model.md) | SDK reservation handle (`Reservations` / `EthereumReservations`) | holds | none recorded |
| [SM-C12](01-system-model.md) | SDK reservation service (`ReservationsService`) | holds | none recorded |
| [SM-C13](01-system-model.md) | SDK status reads (view surface) | holds | none recorded |
| [SM-C14](01-system-model.md) | SDK pre-funding TOCTOU boundary (limit) | holds-untested | Status `holds-untested` (the SDK check is unit-tested at `typescript/test/services/reservations.test.ts` ... |
| [SM-C15](01-system-model.md) | D31 hand-off admission (implemented at `tbtc-v2@806b1106f` and `keep-core@d40f6aadc`; not deployed) | holds | none recorded |
| [SM-C16](01-system-model.md) | D28 maintainer keepers + alerts (implemented; live rollout pending) | holds | Status `holds` for implementation and recorded local acceptance, not deployment. Gauges: ... |
| [SM-C2](01-system-model.md) | ReservationRouter (`delegatecall`) | model | none recorded |
| [SM-C3](01-system-model.md) | Reservation library | model | none recorded |
| [SM-C4](01-system-model.md) | ReservationProofs library | model | none recorded |
| [SM-C5](01-system-model.md) | ReservationVault | model | none recorded |
| [SM-C6](01-system-model.md) | WalletProposalValidator | model | none recorded |
| [SM-C7](01-system-model.md) | Bank / TBTC | model | none recorded |
| [SM-C8](01-system-model.md) | keep-core tasks (ReservationAcceptanceTask, ReservationReanchorTask) | model | none recorded |
| [SM-C9](01-system-model.md) | SPV maintainer (proof loop, watchers) | model | none recorded |
| [SM-D1](01-system-model.md) | (not reserved) | holds | `Reservation.test.ts` guard 5 |
| [SM-D2](01-system-model.md) | reserved/pending | holds | `ReservationProofs.test.ts` "12. Idempotency guard for consumeAcceptedDeposit" |
| [SM-D3](01-system-model.md) | reserved/pending | holds | `Bridge.ReservationStranding.test.ts` "notifyStaleReservedDeposit" |
| [SM-D4](01-system-model.md) | reserved/pending | holds-untested | `Bridge.ReservationStrandingLibrary.test.ts` |
| [SM-D5](01-system-model.md) | reserved/pending (never requested: `reservations[key].requestNonce == 0`) | holds | `tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts` "happy path" and "releaseReservedDepositToPool guards" groups |
| [SM-R1](01-system-model.md) | Unknown | holds | `tbtc-v2:solidity/test/bridge/Reservation.test.ts` ("step 20: state mutations and event emission") |
| [SM-R10](01-system-model.md) | Active / ActionPending | design-only | none in M1 |
| [SM-R11](01-system-model.md) | ActionPending | holds | `Bridge.ReservationStrandingLibrary.test.ts` timeout-capacity cases |
| [SM-R2](01-system-model.md) | Unknown | holds | `Reservation.test.ts` acceptance-timeout group |
| [SM-R3](01-system-model.md) | Unknown | holds | `ReservationProofs.test.ts` "5. End-to-end happy path" |
| [SM-R4](01-system-model.md) | Unknown | holds | `ReservationProofs.test.ts` "8. Late-settle and pending newer generation unwinding" |
| [SM-R5](01-system-model.md) | Active | holds | `Bridge.ReservationSourceAnchorBinding.test.ts`; `Bridge.ReservationThirdPartyReanchorHolds.test.ts` (at `806b1106f`: ... |
| [SM-R6](01-system-model.md) | Active | holds | `Bridge.ReservationStranding.test.ts` "notifyReservationStranded"; `tbtc-v2@4c98e6b82:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:186` (Closing ... |
| [SM-R7](01-system-model.md) | ActionPending (on-time), Active or qualifying Stranded (late) | holds | `ReservationProofs.test.ts` end-to-end SPV cases |
| [SM-R8](01-system-model.md) | Active / ActionPending / Stranded | holds | `ReservationProofs.test.ts` "9. Strand flow on late settlement"; `tbtc-v2@4c98e6b82:solidity/test/bridge/ReservationProofs.test.ts:1434` (Closing target stays ... |
| [SM-R9](01-system-model.md) | Active or ActionPending | holds | `ReservationProofs.test.ts` "8. ... unwinding" |
| [SM-S1](01-system-model.md) | Position record (`ReservationRequest`) | model | none recorded |
| [SM-S2](01-system-model.md) | Action record (`ReservationAction`) | model | none recorded |
| [SM-S3](01-system-model.md) | Per-wallet info (`WalletReservationInfo`) | model | none recorded |
| [SM-S4](01-system-model.md) | Global counters, and per-wallet mappings | model | none recorded |
| [SM-S5](01-system-model.md) | Pending deposit record (`PendingReservedDeposit`) | model | none recorded |
| [SM-V1](01-system-model.md) | `isVaultTrusted[vault]` | holds-untested | holds (on-time path pinned in `ReservationProofs.test.ts` "11. Vault-routing regression test" ... |
| [SM-W1](01-system-model.md) | Live | holds | `tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts` "driven to Terminated by a real moving-funds-timeout call chain" |
| [SM-W2](01-system-model.md) | Live | holds | `tbtc-v2:solidity/test/bridge/Bridge.Wallets.test.ts` "should change wallet's state to Closing" |
| [SM-W3](01-system-model.md) | Closing | holds-untested | `none` (the count gate itself has no dedicated Solidity test; it is exercised indirectly by `Bridge.ReservationOccupancy.test.ts`) |
| [SM-W4](01-system-model.md) | any | holds | `tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts` (terminates a real wallet mid-reservation) |
| [SM-W5](01-system-model.md) | Active-reservation on a Closing/Closed/Terminated wallet | holds | `tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts` "notifyReservationStranded" |
| [SM-X1](01-system-model.md) | Unknown | holds | writes a fresh `ReservationAction` record keyed by `(key, ++requestNonce)`. `Reservation.sol:631`, `:915` |
| [SM-X2](01-system-model.md) | Pending | holds | `ReservationProofs.sol:489`, `:734` |
| [SM-X3](01-system-model.md) | Pending | holds | acceptance: `Reservation.sol:692`; re-anchor: `Reservation.sol:963` (also reverts `reservation.state` to Active + sets cooldown) |
| [SM-X4](01-system-model.md) | Pending | holds | `ReservationProofs.sol:849` |
| [SM-X5](01-system-model.md) | TimedOut | holds-untested | re-takes capacity without re-checking caps; only Acceptance is window-bounded (`now <= timeoutAt + termSeconds`) ... |
| [SM-X6](01-system-model.md) | TimedOut | holds | late settlement closes the anchor lineage; no second refund. `ReservationProofs.sol:57` |
| [SM-X7](01-system-model.md) | Vetoed | design-only | `Reservation.sol:138` |

### Invariants

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [INV-101](02-invariants.md) | One position state per key; pending acceptance is action-tracked, pending re-anchor sets ActionPending | holds | `Reservation.test.ts` "should revert if reservation already exists (state == Active)" (:269) ... |
| [INV-102](02-invariants.md) | A wallet that custodies live reservations cannot be finalized to Closed | holds-untested | Routing both ways: `tbtc-v2@4c98e6b82:solidity/test/bridge/Reservation.test.ts:2007` (a proven anchor keeps the wallet in MovingFunds) and ... |
| [INV-103](02-invariants.md) | Termination permits an unresolved position; cleanup requires an explicit caller | holds | `Bridge.ReservationStranding.test.ts` "strands an Active reservation on a wallet driven to Terminated by a real moving-funds-timeout call chain" (:554) ... |
| [INV-104](02-invariants.md) | Normal wallet closing requires zero reservation count; termination permits unresolved positions | holds | `Bridge.ReservationStrandingLibrary.test.ts` "strands an Active reservation on a Closed wallet" (:471) and ... |
| [INV-105](02-invariants.md) | A Stranded position is terminal except for late-settlement reconstruction of a timed-out generation | holds | `Bridge.ReservationOccupancy.test.ts` "prepareReservationForSettlement restores activeReservationsCount on late settlement of stranded reservation" (:238). |
| [INV-106](02-invariants.md) | The Closed position state is never observable in M1 | design-only | `none` (dead path, no test). |
| [INV-107](02-invariants.md) | An ActionPending position is always paired with a current Pending Reanchor generation | holds | `Bridge.ReservationStrandingLibrary.test.ts` "releases target wallet capacity on Reanchor timeout" (:947) pins the `ActionPending -> Active` + `TimedOut` ... |
| [INV-108](02-invariants.md) | A Pending Acceptance generation is only observable while the position is Unknown and no other action is Pending | holds | `Reservation.test.ts` "should revert if acceptance is already pending" (:299). |
| [INV-109](02-invariants.md) | Pending reserved deposit record: a reserved deposit that is neither accepted, stale nor released keeps `pendingReservedDeposits > 0`, which blocks a vault change | holds-untested | The gate itself: `none` at the pin. At `d20cf74ba`, `Bridge.ReservationReleaseToPool.test.ts` ... |
| [INV-110](02-invariants.md) | Vault status is the only switch that routes a settlement credit to the vault vs. the depositor | holds | `ReservationProofs.test.ts` "should credit the deposit's immutable vault, not a reservationVault repointed mid-flight" (:1757). |
| [INV-111](02-invariants.md) | Release to pool (D26): the record is cleared and the deposit re-routed; refused after any acceptance request and for the reservation vault | holds-untested | - rule 4: "reverts after a pending acceptance request" (:501), "reverts after a timed-out acceptance request" (:515) ... |
| [INV-112](02-invariants.md) | Legal state relationships at transaction boundaries | holds-untested | The individual transition tests are listed in INV-101 through INV-111. `none` identified for the complete state-relation table. |
| [INV-113](02-invariants.md) | D31 acceptance reserve is a request-time count bound | holds | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:276` (describe "hand-off reserve gate at acceptance"): ... |
| [INV-114](02-invariants.md) | D31 hand-off exemption preserves bookkeeping and the remaining guards | holds | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:359` (describe ... |
| [INV-115](02-invariants.md) | D28 failed observations do not publish fresh risk data | holds | Failed action reads preserve the published timestamp and values (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:591-648`). Source ... |
| [INV-201](02-accounting-invariants.md) | `activeReservationsCount` | holds | `Bridge.ReservationOccupancy.test.ts` "requestReservationAcceptance increments activeReservationsCount and emits ReservationOccupancyChanged" (:30) ... |
| [INV-202](02-accounting-invariants.md) | `reservationTotalAmount` | holds | `Reservation.test.ts` "should record action, increment nonce, update counters, and emit ReservationAcceptanceRequested" (:804) ... |
| [INV-203](02-accounting-invariants.md) | `walletReservationInfo[w].count` | holds | `Reservation.test.ts` "should revert when wallet reservations count exceeds maxReservationsPerWallet" (:595) ... |
| [INV-204](02-accounting-invariants.md) | `walletReservationInfo[w].amount` | holds | `Reservation.test.ts` "should revert when wallet reservations amount exceeds maxReservationsAmountPerWallet" (:631) ... |
| [INV-205](02-accounting-invariants.md) | `pendingThirdPartyReanchorHolds[w]` | holds | `Bridge.ReservationThirdPartyReanchorHolds.test.ts` "blocks a third party once the target's pending holds reach the cap" (:133) ... |
| [INV-206](02-accounting-invariants.md) | `pendingReservedDeposits` | holds | `ReservationProofs.test.ts` "should not double-decrement pendingReservedDeposits when the pending marker is already cleared" (:1841) ... |
| [INV-207](02-accounting-invariants.md) | `inKindFeeDebtSat` (vault) | holds | `ReservationVault.test.ts` "should record the entire fee as debt" (:379), "should cap an over-repayment at the outstanding debt" (:519) ... |
| [INV-208](02-accounting-invariants.md) | `cumulativeReanchorFee` (per position) | holds-untested | `none` pins the running total directly (the grind bound is pinned by `Bridge.ReservationSettlement.test.ts` "bounds the grind at a request-time amount floor" ... |
| [INV-209](02-accounting-invariants.md) | `mintedAmount == anchorAmount` | holds | `ReservationProofs.test.ts` "should successfully execute end-to-end acceptance settlement" asserts `reservation.mintedAmount == anchorAmount` (:555 ... |
| [INV-210](02-accounting-invariants.md) | Counter width / truncation safety | holds-untested | `none` identified for the timestamp narrowing horizon. A boundary regression is possible in a local EVM ... |
| [INV-211](02-accounting-invariants.md) | `walletAnchoredInfo[w]` - proven anchors, distinct from reserved capacity | holds-untested | `tbtc-v2@4c98e6b82:solidity/test/bridge/Reservation.test.ts:2007` (a proven anchor keeps the wallet in MovingFunds) and ... |
| [INV-212](02-accounting-invariants.md) | Whole-system fungible supply exceeds backing by exactly the recorded fee debt | holds-untested | `ReservationVault.test.ts` shortfall, repayment and sweep cases cited in INV-207. `none` identified for an end-to-end check of the bitcoind-backed equality. |
| [INV-301](02-backing-authority-invariants.md) | Anchor outpoint uniqueness | holds | `ReservationProofs.test.ts` "should settle re-anchor SPV proof, update reverse anchor index, and strand cleanly" (:1571). |
| [INV-302](02-backing-authority-invariants.md) | The recorded reservation amount follows the current anchor | holds | `Bridge.ReservationSettlement.test.ts` ... |
| [INV-303](02-backing-authority-invariants.md) | Fee bounds per hop and cumulative grind | holds | `ReservationProofs.test.ts` "should revert if miner fee exceeds snapshotted txMaxFee" (:734) ... |
| [INV-304](02-backing-authority-invariants.md) | Single-input / single-output lineage | holds | `ReservationProofs.test.ts` "should revert if output vector does not have exactly 1 output (0 outputs)" (:699) ... |
| [INV-401](02-backing-authority-invariants.md) | Who can move which state | holds | none recorded |
| [INV-402](02-backing-authority-invariants.md) | Re-anchor source-wallet gating | holds | `Bridge.ReservationStrandingLibrary.test.ts` "rejects when source wallet is Live and call is not privileged" (:1640) ... |
| [INV-501](02-generation-time-invariants.md) | `requestNonce` is monotonic; each generation is uniquely `(key, nonce)` | holds | `ReservationProofs.test.ts` "should revert if action state is Settled (2)" (:315), "should revert if action state is Superseded (5)" (:353) ... |
| [INV-502](02-generation-time-invariants.md) | On-time settlement must name the current generation; late settlement names an older timed-out one | holds | `ReservationProofs.test.ts` "should unwind pending newer acceptance generation when settling older generation late" (:920) ... |
| [INV-503](02-generation-time-invariants.md) | A timed-out generation remains settleable only within its window; a re-anchored anchor cannot be replayed | holds | `Bridge.ReservationSourceAnchorBinding.test.ts` "rejects a timed-out re-anchor replay against a later anchor" (:412). |
| [INV-601](02-generation-time-invariants.md) | `timeoutAt` and the timeout notification boundary | holds | `Bridge.ReservationStrandingLibrary.test.ts` "reverts when action has not timed out" (:1027). |
| [INV-602](02-generation-time-invariants.md) | Re-anchor cooldown | holds | `Bridge.ReservationStrandingLibrary.test.ts` "rejects when reanchor cooldown is in effect" (:1610). |
| [INV-603](02-generation-time-invariants.md) | Term / dissolution-eligible / refund-deadline windows | holds | `Reservation.test.ts` "should succeed when timeoutAt + 24h == refundDeadline (close bound exact)" (:405) and ... |
| [INV-701](02-generation-time-invariants.md) | The vault mints gross and retains the fee; debt is the only mechanism that can under-cover a miner fee | holds | `ReservationVault.test.ts` "should transfer the net amount (gross minus fee) to the depositor" (:201), "should retain the fee in the vault" (:205) ... |
| [INV-702](02-generation-time-invariants.md) | Acceptance settlement uses the deposit's stored vault, subject to current trust | holds | `ReservationProofs.test.ts` "should credit the deposit's immutable vault, not a reservationVault repointed mid-flight" (:1757). |
| [INV-703](02-generation-time-invariants.md) | `setVaultStatus(vault, true)` is only safe at/after the keep-core activation block B | holds-untested | `TestReservationsActivationBlock_SanityCheck` (`keep-core:pkg/tbtc/coordination_test.go:1608`) pins that `Mainnet` and `Unknown` have no entry and therefore ... |
| [INV-704](02-generation-time-invariants.md) | Units, gross mint, net transfer and fee reserve | holds-untested | `ReservationVault.test.ts` net-transfer and fee-retention cases cited in INV-701. `none` recorded here for a combined contract/client/SDK unit-consistency ... |
| [INV-705](02-generation-time-invariants.md) | Miner-fee shortfall is an incremental backing difference, not the only possible system loss | holds-untested | `ReservationVault.test.ts` shortfall, repayment and sweep cases cited in INV-207. `none` identified for the complete Bitcoin-plus-token conservation property. |

### Parameters and governance

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [PAR-1](03-parameters-governance.md) | `reservationMinAmount` (uint64) | holds | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new total amount cap drops below the dust threshold" (indirect); `Reservation.test.ts` guard 11 |
| [PAR-10](03-parameters-governance.md) | `reservationRouter` (address) | holds-untested | `none` |
| [PAR-11](03-parameters-governance.md) | `maxReservationsAmountPerWallet` (uint64) | holds | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new per-wallet amount cap drops below the dust threshold" |
| [PAR-12](03-parameters-governance.md) | `reservationMaxSingleAmount` (uint64) | holds | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new single-amount cap drops below the dust threshold" |
| [PAR-13](03-parameters-governance.md) | `maxActiveReservations` (uint32) | holds | `Bridge.ReservationCaps.test.ts` "reverts when maxActiveReservations is set to zero" (`:153`) ... |
| [PAR-14](03-parameters-governance.md) | `maxThirdPartyReanchorHoldsPerWallet` (uint32) | holds | `Bridge.ReservationThirdPartyReanchorHolds.test.ts` "third-party hold cap (request-time)" ... |
| [PAR-15](03-parameters-governance.md) | `depositDustThreshold` (uint64) | holds | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new dust threshold exceeds the single-amount cap" |
| [PAR-16](03-parameters-governance.md) | `depositRevealAheadPeriod` (uint32) | holds | `Bridge.ReservationFeasibilityGuard.test.ts` "reverts when the new reveal-ahead period exceeds the term plus margin" |
| [PAR-17](03-parameters-governance.md) | `depositTxMaxFee` (uint64) | holds | `Bridge.ReservationFeasibilityGuard.test.ts` "accepts every feasible deposit parameter update" |
| [PAR-18](03-parameters-governance.md) | `depositTreasuryFeeDivisor` (uint64) | holds | `none` (reservation path ignores it by design) |
| [PAR-19](03-parameters-governance.md) | `walletCreationPeriod` (uint32) | holds | `keep-core:pkg/tbtcpg/reservation_reanchor_custody_test.go` "SkipsYoungResumeTarget" |
| [PAR-2](03-parameters-governance.md) | `reservationTxMaxFee` (uint64) | holds | `Reservation.test.ts` guards 11-14 |
| [PAR-20](03-parameters-governance.md) | `walletClosingPeriod` / `movingFundsTimeout` (uint32) | holds | `Bridge.ReservationStranding.test.ts` "real moving-funds-timeout call chain" |
| [PAR-21](03-parameters-governance.md) | `liveWalletsCount` (uint32, derived) | holds | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:346` (bound tightens when a Live wallet leaves Live) |
| [PAR-3](03-parameters-governance.md) | `reservationTermSeconds` (uint32) | holds-untested | none for the exact bounds-revert string; `Bridge.ReservationFeasibilityGuard.test.ts:170` covers PAR-X3 |
| [PAR-4](03-parameters-governance.md) | `reservationDissolutionDelay` (uint32) | holds | `Bridge.ReservationStranding.test.ts` "notifyReservationStranded"; `Bridge.ReservationStrandingLibrary.test.ts:1060` |
| [PAR-5](03-parameters-governance.md) | `reservationMaxTotalAmount` (uint64) | holds | `Bridge.ReservationCaps.test.ts` "reverts when the total amount cap exceeds slot capacity" |
| [PAR-6](03-parameters-governance.md) | `maxReservationsPerWallet` (uint32) | holds | `Reservation.test.ts` "should revert when wallet reservations count exceeds maxReservationsPerWallet" |
| [PAR-7](03-parameters-governance.md) | `reservationActionTimeout` (uint32) | holds | `Bridge.ReservationCaps.test.ts` "action timeout floor" group `tbtc-v2@4c98e6b82:solidity/test/bridge/Bridge.ReservationCaps.test.ts:98-130`: `:122` ... |
| [PAR-8](03-parameters-governance.md) | `reservationRenewalWindowSeconds` (uint32) | design-only | `none` |
| [PAR-9](03-parameters-governance.md) | `reservationVault` (address) | holds | `Bridge.ReservationCaps.test.ts` "bootstrap ordering" group |
| [PAR-F1](03-parameters-governance.md) | `handoffReserveWallets` (uint32, >= 1): fifth caps-setter argument, new `reservationHandoffReserveWallets()` view, acceptance reserve gate | holds | Gate: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:276` (describe "hand-off reserve gate at acceptance"), `:322`, `:346` ... |
| [PAR-F2](03-parameters-governance.md) | Privileged hand-off cap bypass: a privileged re-anchor from a non-Live source skips the target's count and amount caps; the signing validator no longer re-checks them; third parties and Live-source governance rotation keep the caps | holds | Contracts: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:359` (describe), `:427`, `:435`, `:443`, `:452`, `:466` ... |
| [PAR-K1](03-parameters-governance.md) | Seven-day late-proof risk window; one-day unrequested-deposit risk window | holds | `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:288-351` ... |
| [PAR-K2](03-parameters-governance.md) | `RESERVATION_KEEPER_OBSERVATION_MAX_AGE_SECONDS` | holds | Equality, one-second excess and catch-up cases: `tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:259-328` |
| [PAR-X1](03-parameters-governance.md) | Re-anchor min-amount floor | holds | none recorded |
| [PAR-X2](03-parameters-governance.md) | Decision-1 relational invariant | holds | none recorded |
| [PAR-X3](03-parameters-governance.md) | Feasibility guard (`assertReservedDepositsFeasible`) | holds | none recorded |
| [PAR-X4](03-parameters-governance.md) | Saturation sizing | holds | `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322` (reverts past `(L - R) * N` with max-active room) ... |
| [PAR-X5](03-parameters-governance.md) | Action-timeout vs REQUEST_TIMEOUT_SAFETY_MARGIN | holds | none recorded |
| [PAR-X6](03-parameters-governance.md) | Reveal-time refund-deadline cap for reservations | holds | none recorded |
| [PAR-X7](03-parameters-governance.md) | Fee settings (vault) | holds | `tbtc-v2:solidity/test/vault/ReservationVault.test.ts` ("receiveBalanceIncrease", "financeInKindFee", "sweepFees" groups). |
| [PAR-X8](03-parameters-governance.md) | Re-anchor min amount floor vs settle dust floor | holds | none recorded |
| [PAR-X9](03-parameters-governance.md) | Release-to-pool refund-margin guard (decision D26, at `d20cf74ba`) | holds | none recorded |

### User stories

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [US-01](04-deposit-stories.md) | Reserved deposit reveal | holds | `test/bridge/Bridge.ReservationAcceptanceAuthorization.test.ts:419` (test helper that reveals via the production path) ... |
| [US-02](04-deposit-stories.md) | Reserved reveal rejected: refund locktime beyond term + safety margin | holds-untested | `none` (no test in `test/bridge/Bridge.Deposit.test.ts` asserts this string for a reserved deposit ... |
| [US-03](04-deposit-stories.md) | Acceptance request: happy path | holds | `tbtc-v2@4c98e6b82:solidity/test/bridge/Reservation.test.ts:1941` ("should request reservation acceptance successfully"). Status `holds` at `tbtc-v2@4c98e6b82` ... |
| [US-04](04-deposit-stories.md) | Acceptance request: depositor and revealed-deposit gates (negative) | holds | none recorded |
| [US-05](04-deposit-stories.md) | Acceptance request: capacity and amount-cap gates (negative) | holds | none recorded |
| [US-06](04-deposit-stories.md) | Acceptance timeout and re-authorization | holds-untested | `test/bridge/Reservation.test.ts:1492` ("should time out a pending acceptance and release its capacity"); `test/bridge/Bridge.ReservationOccupancy.test.ts:68` ... |
| [US-07](04-deposit-stories.md) | Late acceptance settlement by SPV maintainer | holds | `test/bridge/ReservationProofs.test.ts:555` ("should successfully execute end-to-end acceptance settlement") ... |
| [US-08](04-deposit-stories.md) | Late acceptance: window expired (negative) | holds | `tbtc-v2@4c98e6b82:solidity/test/bridge/ReservationProofs.test.ts:446` ("should reject a late acceptance one second after the window closes" ... |
| [US-09](04-deposit-stories.md) | Superseding acceptance: late settlement of an older generation unwinds a newer pending generation | holds | `test/bridge/ReservationProofs.test.ts:920` ("should unwind pending newer acceptance generation when settling older generation late"). Status `holds`. |
| [US-10](04-deposit-stories.md) | Stale reserved deposit (permissionless) and force stale (governance) | holds-untested | `test/bridge/Bridge.ReservationStranding.test.ts:700`, `:727`, `:748`, `:761`, `:782`, `:803` (full notify-stale coverage) ... |
| [US-100](04-user-stories.md) | Independent keeper alerts reach the existing monitoring receiver | holds | Healthy control and four alert classifications `tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:111-192` ... |
| [US-101](04-deposit-stories.md) | Stranding on a dissolution-eligible Live wallet | holds | `tbtc-v2@4c98e6b82:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:386` (rejects on a Live wallet before eligibility) and `:397` ... |
| [US-11](04-deposit-stories.md) | Mint, fee, and the credit path | holds | `test/vault/ReservationVault.test.ts:201` ("should transfer the net amount (gross minus fee) to the depositor"), `:205`, `:209`, `:214` ... |
| [US-12](04-deposit-stories.md) | Deposit refund via Bitcoin script | holds-untested | Request/refund overlap guards are linked at `Reservation.test.ts:433`, `:505` and `:526`. `none` identified for an end-to-end reserved-deposit Bitcoin refund. ... |
| [US-13](04-deposit-stories.md) | Stranding on a Terminated wallet (MovingFunds timeout, moved-funds-sweep timeout, or fraud defeat) | holds-untested | `test/bridge/Bridge.ReservationStranding.test.ts:525` ... |
| [US-14](04-deposit-stories.md) | Stranding on a Closing wallet after dissolution eligibility and the closing period | holds | `tbtc-v2@4c98e6b82:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:222` (dissolution-eligible but the closing period still runs: reverts) ... |
| [US-15](04-deposit-stories.md) | Stranding on a closed wallet, mid-stranding race | holds | `test/bridge/Bridge.ReservationStranding.test.ts:633` ("rejects a second stranding call after a successful strand (state is no longer Active)") ... |
| [US-20](04-wallet-spv-stories.md) | Acceptance proposal: discover, sign, broadcast | holds | `keep-core:pkg/tbtcpg/reservation_acceptance_test.go:904` ("TestReservationAcceptanceTask_Run"), `:1384` ("BoundaryChecks") ... |
| [US-21](04-wallet-spv-stories.md) | Acceptance task: vault unconfigured or no candidate | holds | `keep-core:pkg/tbtcpg/reservation_acceptance_test.go:1075` ("TestReservationAcceptanceTask_NoCandidates") ... |
| [US-22](04-wallet-spv-stories.md) | Acceptance task: candidate inside timeout safety margin | holds | `keep-core:pkg/tbtcpg/reservation_acceptance_test.go:1872` ("TestReservationAcceptanceTask_SkipsCandidateInsideTimeoutSafetyMargin") ... |
| [US-23](04-wallet-spv-stories.md) | Re-anchor proposal: MovingFunds source, member proof | holds | `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:1191` ... |
| [US-24](04-wallet-spv-stories.md) | Re-anchor: Closing source, target search respects headroom | holds | `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:1240` ("successfully requests reservation re-anchor from Closing wallet (unprivileged happy path)") ... |
| [US-25](04-wallet-spv-stories.md) | Re-anchor: cap revert -> next candidate | holds | `keep-core:pkg/tbtcpg/reservation_reanchor_test.go:996` ("TestReservationReanchorTask_CapRevertLeadsToNextCandidate") ... |
| [US-26](04-wallet-spv-stories.md) | Re-anchor: in-flight resumption | holds | `keep-core:pkg/tbtcpg/reservation_reanchor_test.go:1329` ("TestReservationReanchorTask_ResumesPendingReanchorWithoutNewRequest") ... |
| [US-27](04-wallet-spv-stories.md) | Re-anchor: aged target, cooldown, request-time floor | holds | `keep-core:pkg/tbtcpg/reservation_reanchor_custody_test.go:18` ("SkipsYoungResumeTarget"), `:93` ("ReanchorTargetSearch_SkipsYoungCandidate") ... |
| [US-28](04-wallet-spv-stories.md) | Activation-block gating: client and on-chain coupling | holds | `keep-core:pkg/tbtc/coordination_test.go:1536` ("TestCoordinationExecutor_GetActionsChecklist_Reservations"), `:1470` ... |
| [US-29](04-wallet-spv-stories.md) | Deposit sweep rejects reserved deposits and fails closed on reservation parameter error | holds | `keep-core:pkg/tbtc/deposit_sweep_test.go:574` ("FailsClosedOnReservationParametersError"), `:612` ("RejectsReservedDeposit"). Status `holds`. |
| [US-30](04-wallet-spv-stories.md) | Re-anchor on Live source: governance privileged path | holds | `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:1295` ... |
| [US-31](04-wallet-spv-stories.md) | Re-anchor: third party, owner, SPV maintainer, member proof exempt the third-party hold cap | holds | `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:133` ("blocks a third party once the target's pending holds reach the cap"), `:151` (owner) ... |
| [US-32](04-wallet-spv-stories.md) | Re-anchor action timeout + cooldown | holds | `test/bridge/Reservation.test.ts:1538` ... |
| [US-33](04-wallet-spv-stories.md) | Re-anchor SPV proof settlement | holds-untested | `test/bridge/ReservationProofs.test.ts:1571` ("should settle re-anchor SPV proof, update reverse anchor index, and strand cleanly"), `:1037` ... |
| [US-34](04-wallet-spv-stories.md) | Wallets moveFunds/beginWalletClosing/finalizeWalletClosing with reservations | holds | `tbtc-v2@4c98e6b82:solidity/test/bridge/Reservation.test.ts:2007` ("routes Live wallet with zero main UTXO and a proven reservation anchor to MovingFunds") and ... |
| [US-40](04-wallet-spv-stories.md) | Acceptance SPV proof (positive) | holds | `keep-core:pkg/maintainer/spv/reservation_proof_loop_test.go:77` ("TestFindReservationAcceptanceTransaction"), `:555` ("TestProveReservationTransaction") ... |
| [US-41](04-wallet-spv-stories.md) | Re-anchor SPV proof (positive) | holds | `keep-core:pkg/maintainer/spv/reservation_proof_loop_test.go:325` ("TestFindReservationReanchorTransaction"), `:1011` ("TestProveReservationReanchorActions") ... |
| [US-42](04-wallet-spv-stories.md) | Proof submission gate (negative) | holds-untested | `none` (no test in `test/bridge/` asserts this exact revert string at the router entry ... |
| [US-43](04-wallet-spv-stories.md) | Wrong proof type (negative) | holds | `test/bridge/ReservationProofs.test.ts:203` ("should revert if proof type is out of range"), `:229` ... |
| [US-44](04-wallet-spv-stories.md) | Action not settleable (negative) | holds | `test/bridge/ReservationProofs.test.ts:296`, `:315`, `:334`, `:353`, `:372`; `test/bridge/Reservation.test.ts:1487`. Status `holds`. |
| [US-45](04-wallet-spv-stories.md) | Anchor / input / output reverts in the proof (negative) | holds-untested | `test/bridge/ReservationProofs.test.ts:699`, `:707`, `:719`, `:734`, `:767`, `:779`, `:791`, `:805` ... |
| [US-46](04-wallet-spv-stories.md) | Per-item errors do not block sibling actions; restart resumes scan | holds | `keep-core:pkg/maintainer/spv/reservation_proof_loop_test.go:2421` ("TestRunReservationProofLoop_ResumesScanAfterRestart") ... |
| [US-47](04-wallet-spv-stories.md) | Action timeout watcher notifies and alerts | holds | `keep-core:pkg/maintainer/spv/reservation_action_timeout_watch_test.go:43` ("NotifiesTimedOutPendingAction") ... |
| [US-48](04-wallet-spv-stories.md) | Stale-deposit watcher (US-10 keep-core side) | holds | `keep-core:pkg/maintainer/spv/reservation_stale_deposit_watch_test.go:60-875` (full coverage). Status `holds`. |
| [US-49](04-wallet-spv-stories.md) | Stranding watcher (US-13/14/15 keep-core side) | holds | `keep-core:pkg/maintainer/spv/reservation_stranding_watch_test.go:38-475` (full coverage) ... |
| [US-50](04-wallet-spv-stories.md) | Feasibility gauge (D25) | holds | `keep-core:pkg/maintainer/spv/reservation_params_feasibility_test.go:165` ("InfeasibleRevealWindow"), `:306` ("CapBelowDust") ... |
| [US-51](04-wallet-spv-stories.md) | Watcher restart and metrics | holds | As above. Status `holds`. |
| [US-60](04-governance-vault-stories.md) | begin/finalize staging for parameters | holds-untested | `test/bridge/Bridge.Governance.test.ts:4428` (`beginReservationParametersUpdate`), `:4505`, `:4536`, `:4556` (`finalizeReservationParametersUpdate`), `:4660` ... |
| [US-61](04-governance-vault-stories.md) | begin/finalize staging for caps | holds | `test/bridge/Bridge.Governance.test.ts:4705`, `:4785`, `:4862`, `:4875`; `test/bridge/Bridge.ReservationCaps.test.ts:153`, `:121`, `:132`, `:224`, `:252` ... |
| [US-62](04-governance-vault-stories.md) | setVaultStatus (vault activation / deactivation, C-4 gate) | holds-untested | `test/bridge/Bridge.Governance.test.ts:4889` ("setVaultStatus"); `test/bridge/ReservationProofs.test.ts:1757` ... |
| [US-63](04-governance-vault-stories.md) | setReservationRouter (one-shot) | holds | `test/bridge/Bridge.Parameters.test.ts:2007` (`setReservationRouter` describe block), `:2052`, `:2073`, `:2095`, `:2108`. Status `holds`. |
| [US-64](04-governance-vault-stories.md) | Revert when finalize runs before the delay elapses or the change is uninitialized | holds | `test/bridge/Bridge.Governance.test.ts` "context" blocks (e.g. `finalizeReservationCapsUpdate` "when the update process is not initialized" ... |
| [US-65](04-governance-vault-stories.md) | forceStaleReservedDeposit (governance override) | holds-untested | no test pins the positive or the negative guards of the on-chain entry, and no test reads the counter or the event it writes. The entry is exercised as setup ... |
| [US-66](04-governance-vault-stories.md) | updateDepositParameters feasibility guard (D25) | holds | `test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:281` ("reverts when the new dust threshold exceeds the single-amount cap"), `:291`, `:304`, `:319` ... |
| [US-67](04-governance-vault-stories.md) | Privileged re-anchor of a Live-wallet position by governance | holds | none recorded |
| [US-68](04-governance-vault-stories.md) | Vault fee and reserve management | holds | `test/vault/ReservationVault.test.ts:796` (non-owner), `:802` (happy), `:812` (max), `:551` (non-owner for `updateFeeReserveTarget`), `:557`, `:566` ... |
| [US-69](04-governance-vault-stories.md) | Debt repayment and in-kind fee financing | holds | `test/vault/ReservationVault.test.ts:275` (non-Bridge), `:291` (zero-fee no-op), `:335` (exact-cover burn), `:342`, `:347`, `:373`, `:379`, `:384`, `:427` ... |
| [US-70](04-governance-vault-stories.md) | Third-party re-anchor request within hold cap (positive) | holds | `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts` (the suite's setup and the `pendingThirdPartyReanchorHolds` count assertions indirectly cover ... |
| [US-71](04-governance-vault-stories.md) | Hold cap exhausted (negative) | holds | `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:133`. Status `holds`. |
| [US-72](04-governance-vault-stories.md) | Hold cap zero (negative) | holds | `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:250`. Status `holds`. |
| [US-73](04-governance-vault-stories.md) | Privileged caller exemption (positive) | holds | `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:151` (owner), `:179` (SPV maintainer), `:204` (member with valid proof). Status `holds`. |
| [US-74](04-governance-vault-stories.md) | Member proof failure (negative) | holds | `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:235`; `keep-core:pkg/tbtcpg/reservation_reanchor_member_proof_test.go:79` ... |
| [US-75](04-governance-vault-stories.md) | Hold release on timeout / supersede / on-time settlement | holds | `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:271` ("drops on action timeout") ... |
| [US-76](04-governance-vault-stories.md) | Permissionless notify (acceptance timeout, action timeout, stale deposit, stranding) | pointer | see US-06, US-10, US-32, US-13. |
| [US-80](04-governance-vault-stories.md) | Holder receives net of fee, fee retained (positive) | holds | none recorded |
| [US-81](04-governance-vault-stories.md) | Sweep excess to treasury (positive) | holds | none recorded |
| [US-82](04-governance-vault-stories.md) | Anyone repays in-kind fee debt (positive) | holds | none recorded |
| [US-83](04-governance-vault-stories.md) | balance approval always reverts (negative) | holds | `test/vault/ReservationVault.test.ts:830`. Status `holds`. |
| [US-90](04-user-stories.md) | Release a reserved deposit back to the pool (depositor gets ordinary sweep / TBTC) | holds-untested | `tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts:350` ... |
| [US-92](04-user-stories.md) | Release to pool guards (negative) | holds-untested | `tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts:404` (already swept), `:475` (caller is not the depositor), `:485` (ordinary ... |
| [US-93](04-sdk-stories.md) | Pre-funding go/no-go for a reserved deposit | holds | `reservations.test.ts` `checkReserveFeasibility` group - "should report go when all rules pass" ... |
| [US-94](04-sdk-stories.md) | Refund locktime window and reveal-by deadline | holds | `reservations.test.ts` "should choose the 45-day locktime with the default 30-day reveal slack" ... |
| [US-95](04-sdk-stories.md) | Deterministic wallet capacity selection | holds | `reservations.test.ts` `selectReservationWallet` group - "should return undefined when no registered wallet is eligible" ... |
| [US-96](04-sdk-stories.md) | Reserved deposit initiation and reveal | holds | `reservations.test.ts` `initiateReservedDeposit` group - "should generate a receipt with a refund locktime inside the window" ... |
| [US-97](04-sdk-stories.md) | Reserved reveal and acceptance request via the SDK | holds | `reservations.test.ts` `requestAcceptance` group - "should submit the acceptance request to the designated wallet" ... |
| [US-98](04-sdk-stories.md) | SDK pre-funding/preflight does not guarantee acceptance (TOCTOU limit) | holds-untested | the SDK checks themselves are unit-tested (US-93, US-97); the guarantee they do NOT provide is the on-chain re-check ... |
| [US-99](04-sdk-stories.md) | Release-to-pool and status reads via the SDK | holds | `reservations.test.ts` `releaseToPool` group - "should release to the zero address when no vault is given" ... |

### Edge cases

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [EC-01](05-deposit-timing-edge-cases.md) | Exact-equality timeout at `timeoutAt` | holds | `test/bridge/Reservation.test.ts:1712` ("should succeed when called after advancing time past timeoutAt") ... |
| [EC-02](05-deposit-timing-edge-cases.md) | Re-anchor cooldown exact equality | holds-untested | `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:1610` ("rejects when reanchor cooldown is in effect") ... |
| [EC-03](05-deposit-timing-edge-cases.md) | Refund deadline exact equality | holds | `test/bridge/Reservation.test.ts:405` ("should succeed when timeoutAt + 24h == refundDeadline (close bound exact)") ... |
| [EC-04](05-deposit-timing-edge-cases.md) | Late acceptance window: `timeoutAt + term` | holds-untested | `test/bridge/ReservationProofs.test.ts:851` ("should settle a timed-out older generation without newer generation (late settlement)") ... |
| [EC-05](05-deposit-timing-edge-cases.md) | Term + 24h: reveal-ahead period bound | holds | `test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:319` ("reverts when the new reveal-ahead period exceeds the term plus margin") ... |
| [EC-06](05-deposit-timing-edge-cases.md) | Deposit amount at the minimum | holds | `test/bridge/Reservation.test.ts:367` (below, revert); `test/bridge/Reservation.test.ts:385` (at boundary, success). |
| [EC-07](05-deposit-timing-edge-cases.md) | Fee exactly `txMaxFee` | holds | `test/bridge/Reservation.test.ts:1100` ("should succeed at the fee-bound boundary") ... |
| [EC-08](05-deposit-timing-edge-cases.md) | Zero values on amount caps: `0` means off | holds | `test/bridge/Reservation.test.ts:545` (single cap 0, success); `test/bridge/Reservation.test.ts:669` (per-wallet cap 0, success). |
| [EC-09](05-deposit-timing-edge-cases.md) | Zero values on structural caps: `0` means blocked | holds | `test/bridge/Reservation.test.ts:709` (occupancy at cap); `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:250` ... |
| [EC-10](05-deposit-timing-edge-cases.md) | Caps changed while actions are pending | holds | `test/bridge/Bridge.ReservationAcceptanceAuthorization.test.ts:472` ("settles an authorized acceptance even after the caps fill up") ... |
| [EC-11](05-lifecycle-chain-edge-cases.md) | Wallet state change mid-action | holds | `tbtc-v2@4c98e6b82:solidity/test/bridge/ReservationProofs.test.ts:1308` (a Closed wallet strands on an on-time proof) and ... |
| [EC-12](05-lifecycle-chain-edge-cases.md) | Ethereum reorg | holds-untested | `keep-core` `TestReservationScanRange` (`pkg/maintainer/spv/reservation_event_scan_test.go:306`) ... |
| [EC-13](05-lifecycle-chain-edge-cases.md) | Bitcoin reorg | holds-untested | `keep-core` `TestProveReservationTransaction_SelectsConfirmedRBFCandidate` (`pkg/maintainer/spv/reservation_proof_loop_test.go:1272`). |
| [EC-14](05-lifecycle-chain-edge-cases.md) | Proofs for superseded generations | holds | `test/bridge/ReservationProofs.test.ts:1072` ("should unwind pending reanchor action") ... |
| [EC-15](05-lifecycle-chain-edge-cases.md) | Same UTXO used twice | holds | `test/bridge/ReservationProofs.test.ts:805` ("should revert on double-consume (deposit already swept)") ... |
| [EC-16](05-lifecycle-chain-edge-cases.md) | P2PKH vs P2WPKH anchor outputs | holds | `test/bridge/ReservationProofs.test.ts:659` ("should accept P2WPKH anchor output vector format") ... |
| [EC-17](05-lifecycle-chain-edge-cases.md) | Activation block equal to the current block | holds-untested | `keep-core` `TestReservationScanRange` (`pkg/maintainer/spv/reservation_event_scan_test.go:306`); the inclusive comparison itself has no dedicated test. |
| [EC-18](05-lifecycle-chain-edge-cases.md) | Last slot race on `maxActiveReservations` | holds | `test/bridge/Bridge.ReservationOccupancy.test.ts:30` (request increments), `:101` (timeout decrements) ... |
| [EC-19](05-lifecycle-chain-edge-cases.md) | Concurrent acceptance and re-anchor on the same wallet | holds | `keep-core` `TestReservationReanchorTask_CapRevertLeadsToNextCandidate` (`pkg/tbtcpg/reservation_reanchor_test.go:996`) ... |
| [EC-20](05-lifecycle-chain-edge-cases.md) | Vault deactivated mid-flight | holds | `test/bridge/ReservationProofs.test.ts:1757` ("should credit the deposit's immutable vault, not a reservationVault repointed mid-flight"). |
| [EC-21](05-lifecycle-chain-edge-cases.md) | `liveWalletsCount` falling to zero | holds | `test/bridge/Bridge.MovingFunds.test.ts:455` ("No target wallets available"), `:942` ("Live wallets count must be zero") ... |
| [EC-22](05-lifecycle-chain-edge-cases.md) | Deployment progress files and chain identity | holds | `tbtc-v2@4c98e6b82:solidity/test/prepare-artifact-guard.test.ts` covers the malformed-artifact rejection, the chain preflight and the binding check ... |
| [EC-23](05-lifecycle-chain-edge-cases.md) | D28 wallets registered before reservation activation | holds | `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_wallet_discovery_test.go:12-73`. Both tests failed on the earlier client ... |
| [EC-24](05-lifecycle-chain-edge-cases.md) | D28 observation freshness and catch-up windows | holds | equality, one-second excess, catch-up and RPC skew (`tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:259-328`) ... |

### Failure modes

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [FM-01](06-service-signing-failure-modes.md) | SPV maintainer down | holds | none recorded |
| [FM-02](06-service-signing-failure-modes.md) | Relay stale | holds | none recorded |
| [FM-03](06-service-signing-failure-modes.md) | keep-core node bug or crash | holds | none recorded |
| [FM-04](06-service-signing-failure-modes.md) | Watcher death | holds | none recorded |
| [FM-05](06-service-signing-failure-modes.md) | Coordination window missed | holds | none recorded |
| [FM-06](06-service-signing-failure-modes.md) | Wallet below quorum | holds | none recorded |
| [FM-07](06-service-signing-failure-modes.md) | Fee reserve empty | holds | none recorded |
| [FM-08](06-lifecycle-operations-failure-modes.md) | Saturation: unproven funds, slashing and termination | holds | privileged hand-off to a full target, a third party reverting, and a hold-occupied target ... |
| [FM-09](06-lifecycle-operations-failure-modes.md) | Saturation after main funds are proven: Closing cannot finalize | holds-untested | the Closing-source request and stranding tests cited in the user stories cover individual transitions. At `806b1106f` the owner `Closing` hand-off to a full ... |
| [FM-10](06-lifecycle-operations-failure-modes.md) | Bad governance parameters | holds | none recorded |
| [FM-11](06-lifecycle-operations-failure-modes.md) | Activation order wrong (C-4) | design-only | none recorded |
| [FM-12](06-lifecycle-operations-failure-modes.md) | RPC failures | holds | none recorded |
| [FM-13](06-lifecycle-operations-failure-modes.md) | Bitcoin fee spike above txMaxFee | holds | none recorded |
| [FM-14](06-lifecycle-operations-failure-modes.md) | Gas price spike | holds-untested | none recorded |

### Disaster recovery

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [DR-01](07-disaster-recovery.md) | Saturation: a retiring wallet has no target with capacity | holds-untested | a privileged hand-off to a full target succeeds for the owner, an SPV maintainer and a proven member, a third party reverts ... |
| [DR-02](07-disaster-recovery.md) | Stranded positions: the custodying wallet is Terminated or Closed | holds | unit-tested for Terminated (`tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts:554`) ... |
| [DR-03](07-disaster-recovery.md) | Wrong parameters staged or finalized (governance typo) | holds | the begin/finalize ceremony and the "stages and finalizes the cap through the begin/finalize ceremony" case are unit-tested ... |
| [DR-04](07-disaster-recovery.md) | Vault compromised or misconfigured | holds-untested | none. Fault A is a known gap with no rehearsed mitigation; the deactivation path (`setVaultStatus(false)`) has no reservation-specific fork/Sepolia rehearsal. ... |
| [DR-05](07-disaster-recovery.md) | Router set to the wrong address (write-once) | design-only | the set-once and non-zero and must-be-a-contract reverts are unit-tested (`tbtc-v2:solidity/test/bridge/Bridge.Parameters.test.ts:2016`, `:2051` ... |
| [DR-06](07-disaster-recovery.md) | Bridge upgrade gone wrong (router replacement breaks storage parity) | holds-untested | the invariants are unit-tested (storage parity, selector disjointness). A wrong-upgrade recovery was not rehearsed on a fork/Sepolia: never. |
| [DR-07](07-disaster-recovery.md) | keep-core release with the wrong activation block | holds-untested | the "no entry means never" behavior is unit-tested (`keep-core:pkg/tbtc/coordination_test.go:1608` ... |
| [DR-08](07-disaster-recovery.md) | Mass timeout: a large batch of actions all time out together | holds | the decrement-and-event behavior of acceptance timeouts is unit-tested (`tbtc-v2:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:68`) ... |
| [DR-09](07-disaster-recovery.md) | Fee debt growth: the vault cannot finance in-kind miner fees | holds | shortfall and repayment are unit-tested at the original pin (`tbtc-v2:solidity/test/vault/ReservationVault.test.ts:273` ... |
| [DR-10](07-disaster-recovery.md) | Relay or SPV outage near the late-proof cutoff (timeoutAt + termSeconds) | unknown | late settlement of a timed-out older generation and the late-strand-on-closed-target path are unit-tested ... |
| [DR-11](07-disaster-recovery.md) | Key loss: the governance (Bridge owner) key is lost | design-only | none. Key loss has no fork/Sepolia rehearsal. Never. |
| [DR-12](07-disaster-recovery.md) | Key loss: the vault owner key is lost | holds-untested | none. Never. |

### Threat model

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [TM-01](08-threat-model.md) | Grief pending-reveal slots with a far-future refund locktime | holds-untested | the refund-locktime cap and the pending marker are pinned indirectly by the acceptance guard test set (`tbtc-v2:solidity/test/bridge/Reservation.test.ts:206` ... |
| [TM-02](08-threat-model.md) | Grief a target wallet's capacity via third-party re-anchor holds | holds | cap blocks a third party at the limit (`tbtc-v2:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:133`) ... |
| [TM-03](08-threat-model.md) | Front-run an acceptance or a re-anchor (Ethereum MEV) | holds-untested | the "Acceptance already pending" and single-generation guards are pinned (`tbtc-v2:solidity/test/bridge/Reservation.test.ts:298` ... |
| [TM-04](08-threat-model.md) | Replay a proof against the wrong generation | holds | a timed-out re-anchor replay against a later anchor reverts (`tbtc-v2:solidity/test/bridge/Bridge.ReservationSourceAnchorBinding.test.ts:412`). |
| [TM-05](08-threat-model.md) | Forge or replay a stale SPV proof | holds | the shared difficulty/merkle negatives are pinned for the sweep path (`tbtc-v2:solidity/test/bridge/Bridge.Deposit.test.ts:4223` ... |
| [TM-06](08-threat-model.md) | Grind fees across re-anchor hops | holds | the request-time floor and the no-Live-wallet-rotation gate are pinned (`tbtc-v2:solidity/test/bridge/Bridge.ReservationSettlement.test.ts:500`, `:587`). |
| [TM-07](08-threat-model.md) | Force stranding to dump a shortfall on TBTC holders | holds-untested | stranding transition and counter decrement are pinned (`tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts:525` ... |
| [TM-08](08-threat-model.md) | Slash honest operators via saturation | holds | the moving-funds and closing gates with reservations are pinned (`tbtc-v2:solidity/test/bridge/Reservation.test.ts:1813` ... |
| [TM-09](08-threat-model.md) | Shadow a selector through the delegatecall router | holds | `tbtc-v2:solidity/test/bridge/Bridge.RouterSelectorDisjointness.test.ts:53` (shares no 4-byte selector) and `Bridge.RouterStorageParity.test.ts:196` ... |
| [TM-10](08-threat-model.md) | Collide storage on a Bridge upgrade | holds | `tbtc-v2:solidity/test/bridge/Bridge.StorageLayout.test.ts:45` ("upgrade-safe against the deployed TIP-109 Bridge layout") ... |
| [TM-11](08-threat-model.md) | Mint TBTC for an unverified credit | holds-untested | `tbtc-v2:solidity/test/vault/ReservationVault.test.ts:137` pins the `onlyBank` guard and the gross-mint fee math ... |
| [TM-12](08-threat-model.md) | Kill a pending deposit via stale notify | holds-untested | `notifyStaleReservedDeposit` is pinned (`tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts:700` ... |
| [TM-13](08-threat-model.md) | DoS keep-core event scans via an unbounded scan or a hostile RPC provider | holds-untested | none for RPC-provider throttling; the bounded-scan behavior is unit-tested (`keep-core:pkg/maintainer/spv/reservation_event_scan_test.go`). |
| [TM-14](08-threat-model.md) | Abuse a parameter governance write | holds | the feasibility and slot-capacity reverts are pinned (`tbtc-v2:solidity/test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:170`, `:192`, `:235` ... |
| [TM-15](08-threat-model.md) | Activate on-chain before clients ship (C-4) | design-only | keep-core pins `ReservationsActivationBlock(Mainnet/Unknown) == MaxUint64` (`keep-core:pkg/tbtc/coordination_test.go:1608`). No on-chain guard exists. |
| [TM-16](08-threat-model.md) | Withheld or bad operator signatures (malicious / lazy wallet subset) | holds-untested | none for a specific withholding minority; the fee-floor reject and the on-chain timeout path are the observable effects. |
| [TM-17](08-threat-model.md) | Ship a compromised keep-core release | unknown | none against a poisoned binary; the derivation invariants are unit-tested (`keep-core:pkg/tbtc/coordination_test.go:1608`). |
| [TM-18](08-threat-model.md) | Compromised governance key | unknown | none (a compromise is out of test scope). |
| [TM-19](08-threat-model.md) | Compromised vault owner | holds-untested | `tbtc-v2:solidity/test/vault/ReservationVault.test.ts:273` pins `financeInKindFee` and the debt accounting ... |
| [TM-20](08-threat-model.md) | Bitcoin reorg or fee manipulation | holds-untested | none specific to reservation reorgs; the confirmation-depth behavior is the shared SPV path. |

### Cross-repo contract

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [XR-E-1](09-cross-repo-contract.md) | `ReservationAcceptanceRequested(uint256 indexed reservationKey,uint64 requestNonce,bytes20 indexed walletPubKeyHash,uint64 depositAmount,uint64 txMaxFee,uint32 timeoutAt)` | holds-untested | none recorded |
| [XR-E-2](09-cross-repo-contract.md) | `ReservationReanchorRequested(uint256 indexed reservationKey,uint64 requestNonce,bytes20 indexed sourceWalletPubKeyHash,bytes20 indexed targetWalletPubKeyHash,uint64 txMaxFee)` | holds-untested | none recorded |
| [XR-E-3](09-cross-repo-contract.md) | `NewWalletRegistered(bytes32 indexed ecdsaWalletID,bytes20 indexed walletPubKeyHash)` | holds-untested | none recorded |
| [XR-E-4](09-cross-repo-contract.md) | `DepositRevealed(bytes32 fundingTxHash,uint32 fundingOutputIndex,address indexed depositor,uint64 amount,bytes8 blindingFactor,bytes20 indexed walletPubKeyHash,bytes20 refundPubKeyHash,bytes4 refundLocktime,address vault)` | holds | none recorded |
| [XR-F-1](09-cross-repo-contract.md) | `requestReservationReanchor(uint256,bytes20,uint32[],uint256)` = `0x3bf5b58f` | holds | `holds` (selector pinned in the ABI-snapshot test, see XR-G-3; both sides agree at pin) |
| [XR-F-10](09-cross-repo-contract.md) | `reservationCaps() -> (maxReservationsAmountPerWallet, reservationMaxSingleAmount, maxActiveReservations, maxThirdPartyReanchorHoldsPerWallet)` | holds | `holds` (the 4-tuple includes the D19 `maxThirdPartyReanchorHoldsPerWallet`; keep-core reads the two amount caps for headroom ... |
| [XR-F-11](09-cross-repo-contract.md) | `submitReservationAcceptanceProof((bytes4,bytes,bytes,bytes4),(bytes,uint256,bytes,bytes32,bytes),uint256,uint64)` | holds | holds by inspected canonical ABI; behavior evidence in proof-loop item-error and contract SPV tests |
| [XR-F-12](09-cross-repo-contract.md) | `submitReservationReanchorProof((bytes4,bytes,bytes,bytes4),(bytes,uint256,bytes,bytes32,bytes),uint256,uint64)` | holds | none recorded |
| [XR-F-13](09-cross-repo-contract.md) | `notifyReservationActionTimeout(uint256)` | holds-untested | `holds-untested` (watcher fires; no dedicated revert-round-trip test found by reading) |
| [XR-F-14](09-cross-repo-contract.md) | `notifyReservationAcceptanceTimedOut(uint256)` | holds-untested | none recorded |
| [XR-F-15](09-cross-repo-contract.md) | `notifyStaleReservedDeposit(uint256)` | holds-untested | none recorded |
| [XR-F-16](09-cross-repo-contract.md) | `notifyReservationStranded(uint256)` | holds-untested | none recorded |
| [XR-F-17](09-cross-repo-contract.md) | `requestReservationAcceptance((bytes4,bytes,bytes,bytes4),(bytes,uint256,bytes,bytes32,bytes),uint32,bytes20)` = `0x19821f16` | holds | none recorded |
| [XR-F-18](09-cross-repo-contract.md) | `forceStaleReservedDeposit(uint256)` = `0x9fa3f062` | holds-untested | none recorded |
| [XR-F-19](09-cross-repo-contract.md) | `validateReservationAnchorProposal(ReservationAnchorProposal,DepositExtraInfo) -> bool` = `0xf9179ad2` | holds | holds; real-validator behavior harness `keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:760` |
| [XR-F-2](09-cross-repo-contract.md) | `updateReservationCaps(uint64,uint64,uint32,uint32,uint32)` = `0x78d76eda` (D31, five arguments). At the original pins it was `updateReservationCaps(uint64,uint64,uint32,uint32)` = `0xf858967d`. Governance staging: `beginReservationCapsUpdate(uint64,uint64,uint32,uint32,uint32)` = `0x4c1cda81` (four-argument `0x3684e886` at the original pin); `finalizeReservationCapsUpdate()` = `0x7d87eff2` (unchanged). Event `ReservationCapsUpdated(uint64,uint64,uint32,uint32,uint32)` topic0 `0x964ef9bd68fafce7168d52b1d95f0da86e3be014f9843a793b021652e4e28daa` (four-field topic0 `0x11af1b44d097c7893ef437ea940dea2e21b5c822807b98a3c4d4d9f7f0f11e5d` at the original pins) | holds | `holds` (the 5-tuple is the D31 shape; selector `0x78d76eda` and the event topic0 are pinned by the ABI snapshot test ... |
| [XR-F-20](09-cross-repo-contract.md) | `validateReservationReanchorProposal(ReservationReanchorProposal) -> bool` | holds | holds; at the original pins the real-validator harness `keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:1006` rejected a target above the count cap ... |
| [XR-F-21](09-cross-repo-contract.md) | `isReservedDeposit(uint256) -> bool` | holds | none recorded |
| [XR-F-22](09-cross-repo-contract.md) | `releaseReservedDepositToPool(uint256,address)` = `0x49fdfb19` | holds | `holds` by source inspection with behavioral regressions at `keep-core@4b95f0a06:pkg/tbtcpg/deposit_sweep_release_test.go:91` ... |
| [XR-F-23](09-cross-repo-contract.md) | `reservationHandoffReserveWallets() -> uint32` = `0xcf098fc5` (D31; absent at the original pins) | holds | `holds` (selector and `uint32` output pinned by the ABI snapshot test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts:36` against ... |
| [XR-F-24](09-cross-repo-contract.md) | `reservationByAnchorUtxo(bytes32,uint32) -> uint256` = `0x79731f67` | holds-untested | `holds-untested` (the contract view round-trips in `tbtc-v2@4c98e6b82:solidity/test/bridge/Bridge.ReservationSourceAnchorBinding.test.ts:625-633` ... |
| [XR-F-25](09-cross-repo-contract.md) | `reservedDepositWallet(uint256) -> bytes20` = `0x56803b55` | holds-untested | `holds-untested` (the contract view is read in `tbtc-v2@4c98e6b82:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts:381` ... |
| [XR-F-3](09-cross-repo-contract.md) | `pendingThirdPartyReanchorHolds(bytes20)` = `0x18576b18` | holds-untested | `holds-untested` (binding + view exist and agree at pin; no production caller, so no keep-core test pins the round-trip. The gate-ref gap is XR-G-2) |
| [XR-F-4](09-cross-repo-contract.md) | `activeReservationsCount() -> (uint32 count, uint32 maxActive)` = `0x93fe5eab` | holds | none recorded |
| [XR-F-5](09-cross-repo-contract.md) | `walletReservationsCount(bytes20) -> uint32` | holds | none recorded |
| [XR-F-6](09-cross-repo-contract.md) | `walletReservationsAmount(bytes20) -> uint64` | holds-untested | `holds-untested` (single call site, no dedicated round-trip test found by reading) |
| [XR-F-7](09-cross-repo-contract.md) | `reservations(uint256) -> ReservationRequest` | holds | `holds` (state enum translation pinned by `keep-core:pkg/tbtcpg/reservation_reanchor_inflight_test.go` resume cases) |
| [XR-F-8](09-cross-repo-contract.md) | `reservationActions(uint256,uint64) -> ReservationAction` | holds | `holds` (the action's `requestNonce` is what `validateReservationReanchorProposal` requires to be a Pending Reanchor ... |
| [XR-F-9](09-cross-repo-contract.md) | `reservationParameters() -> (reservationVault, minAmount, txMaxFee, termSeconds, dissolutionDelay, ...)` = `0xf75b4b1c` | holds | none recorded |
| [XR-G-1](09-cross-repo-contract.md) | CI ABI gate compiles tbtc-v2 at a fixed ref and diffs against the vendored fallbacks and the committed bindings | pointer | the gate job itself (fails loudly on any diff); it was not executed in this documentation task |
| [XR-G-2](09-cross-repo-contract.md) | The gate ref `eec999a` is an ancestor of the spec pin `9d2086965`, and strictly predates tbtc-v2 commit `6aac91585` ("fix(bridge): cap third-party re-anchor holds per target wallet") which added `pendingThirdPartyReanchorHolds(bytes20)` and the D19 `maxThirdPartyReanchorHoldsPerWallet` field to the router/caps surface. `keep-core:f99ef96e3`'s vendored `ReservationRouter.fallback-artifact.json` and committed `abi/ReservationRouter.go` DO contain `pendingThirdPartyReanchorHolds`. | violated | `keep-core:.github/workflows/client.yml:206` `TBTC_V2_REF=eec999a` vs `:273` the diff; the gate job was not executed in this documentation task (VF-06) |
| [XR-G-3](09-cross-repo-contract.md) | Selector pinning for the four tuple-input entry points | holds | that test asserts the canonical tuple selectors (`0xe24ad1ae` / `0x6af550d1` / `0xf9179ad2` / `0xa57f734f`) ... |
| [XR-M-1](09-cross-repo-contract.md) | `active_reservations_count` (`MetricReservationActiveReservationsCount`, `keep-core:pkg/clientinfo/performance.go:767`) | holds | none recorded |
| [XR-M-10](09-cross-repo-contract.md) | `performance_reservation_vault_fee_debt_sat` | holds | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:177-285` ... |
| [XR-M-11](09-cross-repo-contract.md) | `performance_reservation_unrequested_deposit_deadline_at_risk_count` | holds | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:406-474` ... |
| [XR-M-12](09-cross-repo-contract.md) | `performance_reservation_keeper_observation_timestamp` | holds | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:591-648` ... |
| [XR-M-2](09-cross-repo-contract.md) | `max_active_reservations` (`keep-core:pkg/clientinfo/performance.go:768`) | holds | none recorded |
| [XR-M-3](09-cross-repo-contract.md) | `reservation_params_infeasible` (`keep-core:pkg/clientinfo/performance.go:797`), a 0/1 alarm | holds | `holds` (pinned by `keep-core:pkg/maintainer/spv/reservation_params_feasibility_test.go:165` `TestReservationParamsInfeasibleRevealWindow` and `:381` ... |
| [XR-M-4](09-cross-repo-contract.md) | `wallet_reservations_count` (`keep-core:pkg/clientinfo/performance.go:770`) | holds | none recorded |
| [XR-M-5](09-cross-repo-contract.md) | `spv_reservation_watcher_deaths_total` (`keep-core:pkg/clientinfo/performance.go:671`), `spv_reservation_action_overdue_total` (`keep-core:pkg/clientinfo/performance.go:678`) | holds-untested | none recorded |
| [XR-M-6](09-cross-repo-contract.md) | `live_wallets_count` (`keep-core:pkg/clientinfo/performance.go:769`), `reservation_vault_fee_debt_sat` (`keep-core:pkg/clientinfo/performance.go:785`), `reservation_vault_fee_reserve_tbtc_base_units` (`keep-core:pkg/clientinfo/performance.go:786`) | holds-untested | none recorded |
| [XR-M-7](09-cross-repo-contract.md) | `reanchor_waiting_on_aged_target` (task-local gauge, not a clientinfo constant) | holds-untested | none recorded |
| [XR-M-8](09-cross-repo-contract.md) | `performance_reservation_acceptance_late_proof_at_risk_count` | holds | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:288-351` ... |
| [XR-M-9](09-cross-repo-contract.md) | `performance_reservation_reanchor_no_eligible_target_count` | holds | holds; `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_wallet_discovery_test.go:12-73` ... |
| [XR-P-1](09-cross-repo-contract.md) | Anchor fee: `0 < anchorTxFee <= action.txMaxFee`; re-anchor fee: `0 < reanchorTxFee <= action.txMaxFee` | holds | none recorded |
| [XR-P-2](09-cross-repo-contract.md) | Acceptance requires `deposit.amount >= action.minAmount + anchorTxFee` | holds | `holds` for the immutable-generation rule; see the real validator harness, not re-anchor headroom as acceptance evidence |
| [XR-P-3](09-cross-repo-contract.md) | Deposit age: `block.timestamp > depositRequest.revealedAt + DEPOSIT_MIN_AGE` `WVP:1017`, `DEPOSIT_MIN_AGE = 2 hours` (`WVPConstants:12`) | holds-untested | none recorded |
| [XR-P-4](09-cross-repo-contract.md) | Refund window: `block.timestamp + DEPOSIT_REFUND_SAFETY_MARGIN < refund locktime` `WVP:1065`, `DEPOSIT_REFUND_SAFETY_MARGIN = 24 hours` (`WVPConstants:13`) | holds-untested | none recorded |
| [XR-P-5](09-cross-repo-contract.md) | Re-anchor signing window: `block.timestamp < action.timeoutAt - REQUEST_TIMEOUT_SAFETY_MARGIN` `WVP:1133`, "Re-anchor action has timed out"; `REQUEST_TIMEOUT_SAFETY_MARGIN = 2 hours` (`WVPConstants:14`), mirrored as the public `REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN` `WVP:198` | holds | `holds` (the hard-coded 2h constant at `:87` is asserted to mirror the on-chain constant ... |
| [XR-P-6](09-cross-repo-contract.md) | Re-anchor target: `action.targetWalletPubKeyHash == proposal.targetWalletPubKeyHash` `WVP:1133` region "Target wallet does not match the authorized action"; `action.actionType == Reanchor` and `action.state == Pending` `WVP:1108` | holds | none recorded |
| [XR-P-7](09-cross-repo-contract.md) | Reserved-deposit routing: "Deposit was not revealed as reserved" `WVP:1025`; "Deposit not routed to the reservation vault" `WVP:1030` | holds-untested | none recorded |
| [XR-R-1](09-cross-repo-contract.md) | `strings.Contains(err.Error(), "cap exceeded")` at `keep-core:pkg/tbtcpg/reservation_reanchor.go:614` (original pin); at `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:630` (function at `:626`, use at `:571`) | holds-untested | `holds-untested` at `d40f6aadc`. At the original pins this was `holds`, pinned by `keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:1191` ... |
| [XR-R-2](09-cross-repo-contract.md) | cooldown error: `keep-core:pkg/tbtcpg/reservation_reanchor_test.go:1874` `strings.Contains(err.Error(), "Reanchor cooldown in effect")` | holds | `holds` (exact string pinned by the test assertion at `keep-core:pkg/tbtcpg/reservation_reanchor_test.go:1874`) |
| [XR-S-1](09-cross-repo-contract.md) | keep-core reads `reservations(key).state` and acts on `Active` (resume/custody) and `ActionPending` (a re-anchor generation is pending). The Go enum `ReservationState` maps Unknown/Active/ActionPending/Closed/Stranded to the on-chain 5-value enum. | holds | `holds` (the ordinal-to-name mapping is pinned by the resume tests in `keep-core:pkg/tbtcpg/reservation_reanchor_inflight_test.go` ... |
| [XR-S-2](09-cross-repo-contract.md) | Enum ordinals must agree | holds-untested | none recorded |
| [XR-S-3](09-cross-repo-contract.md) | Pending re-anchor resumes the authorized nonce and target | holds | none recorded |
| [XR-T-1](09-cross-repo-contract.md) | Coordination frequency = 900 blocks | holds-untested | none recorded |
| [XR-T-2](09-cross-repo-contract.md) | Action timeout and re-anchor cooldown are distinct deadlines | holds-untested | none recorded |
| [XR-T-3](09-cross-repo-contract.md) | `REQUEST_TIMEOUT_SAFETY_MARGIN = 2 hours` | holds | none recorded |
| [XR-T-4](09-cross-repo-contract.md) | `DEPOSIT_MIN_AGE = 2h`, `DEPOSIT_REFUND_SAFETY_MARGIN = 24h` | holds-untested | none recorded |
| [XR-T-5](09-cross-repo-contract.md) | C-4 activation block B | holds-untested | none recorded |
| [XR-T-6](09-cross-repo-contract.md) | Acceptance-request look-back margin = 7,200 blocks (about one day at 12 s per block) | holds-untested | `holds-untested` (a client-chosen window, not a value either side enforces; no test pins the block conversion) |
| [XR-T-7](09-cross-repo-contract.md) | Acceptance reveal look-back margin = 7 days | holds-untested | `holds-untested` (a client-chosen window; no test pins the conversion) |

### Accepted risks

| ID and owning file | Item | Status | Test / rehearsal reference |
|---|---|---|---|
| [AR-01](10-accepted-risks.md) | D19: a third-party hold still uses the target's ordinary slot | holds | none recorded |
| [AR-02](10-accepted-risks.md) | No M1 exit for the owner | holds | none recorded |
| [AR-03](10-accepted-risks.md) | The reservation vault is not upgradeable | holds | none recorded |
| [AR-04](10-accepted-risks.md) | M2 migration ceremony | design-only | none recorded |
| [AR-05](10-accepted-risks.md) | Compensation for a stranded position is off-chain only | design-only | none recorded |
| [AR-06](10-accepted-risks.md) | Re-anchor is unbounded in time; the in-kind claim can grind down | holds | none recorded |
| [AR-07](10-accepted-risks.md) | `closeReservation` is unreachable in M1 | holds | none recorded |
| [AR-08](10-accepted-risks.md) | Permissionless third-party re-anchor griefing (beyond D19's slot reuse) | holds | none recorded |
| [AR-09](10-accepted-risks.md) | Closing-wallet lifecycle has no production trigger in keep-core | holds-untested | none recorded |
| [AR-10](10-accepted-risks.md) | The late-acceptance window can be missed if the SPV maintainer is down | holds | none recorded |
| [AR-11](10-accepted-risks.md) | Expiry does not automatically release an accepted position | holds | none recorded |
| [AR-12](10-accepted-risks.md) | Fee debt can grow while the reserve target is zero | holds | none recorded |
| [AR-13](10-accepted-risks.md) | The stranding startup scan only looks back 30 days | holds-untested | none recorded |
| [AR-14](10-accepted-risks.md) | A re-anchor timeout starts a cooldown that blocks everyone but governance | holds | none recorded |
| [AR-15](10-accepted-risks.md) | `activeReservationsCount` under-reports per-wallet slot pressure during pending re-anchors | holds-untested | none recorded |
| [AR-16](10-accepted-risks.md) | The `receiveBalanceIncrease` ordering gap is enforced by the deploy script, not on-chain | holds-untested | none recorded |

## Maintenance

Change the owning entry, then regenerate this file with `tools/traceability.py`. Do not edit its rows by hand.
A reviewer can check any row against its linked entry: same title, same most cautious status, same test field.
Keep immutable implementation pins in the owning chapter. Do not replace a historical citation with a moving branch name.
Source inspection, a named behavior test and an exercised runtime scenario are different evidence levels.
