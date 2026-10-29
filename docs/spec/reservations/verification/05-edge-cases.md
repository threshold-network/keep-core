# M1 UTXO Reservations: Boundary and Corner Cases (EC)

Scope: M1 reservations in both pinned repos. Contracts at
`tbtc-v2` @ `9d2086965` (cited as `tbtc-v2:solidity/<path>:<line>`);
client at `keep-core` @ `f99ef96e3` (cited as
`keep-core:<path>:<line>`). Status vocabulary: `holds` (code enforces
and a named test pins), `holds-untested` (code enforces, no test),
`violated` (code breaks the expected behaviour), `design-only` (planned,
not in code), `unknown` (decision not derivable from the pinned code).

D31 (privileged hand-off plus acceptance reserve) is implemented at later
later pins: contracts at `tbtc-v2@806b1106f`, client at
`keep-core@d40f6aadc`. Entries and rows that describe D31 behavior cite those
revisions explicitly; unqualified citations keep describing the original pins.
Nothing is deployed on Sepolia or mainnet.

Expected behaviour is the canonical M1 contract; the actual behaviour
column is what the pinned code does. A reviewer checks an entry by
reading the cited lines and running the named test.

## Summary

| ID | Case | Actual behaviour (short) | Test | Status |
|---|---|---|---|---|
| EC-01 | Acceptance timeout exact equality | `>=` check: notify allowed at exactly `timeoutAt`; late window reopens settlement | `test/bridge/Reservation.test.ts:1712` ("should succeed when called after advancing time past timeoutAt") | holds |
| EC-02 | Re-anchor cooldown exact equality | `now == reanchorCooldownUntil` passes the cooldown gate; only the governance-discriminated request bypasses it | `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:1610`; exact equality not established by that test | holds-untested |
| EC-03 | Refund deadline exact equality | stale notify requires strictly `>` deadline; request-time overlap bound is `<=` (equality passes) | `test/bridge/Reservation.test.ts:405`, `:433` | holds |
| EC-04 | Late acceptance window (timeoutAt + term) | settles only while chain head `<= timeoutAt + termSeconds` (exact bound inclusive) | `test/bridge/ReservationProofs.test.ts` section 8 | holds-untested |
| EC-05 | Term + 24h reveal-ahead bound | reveal cap `term + 24h` inclusive; feasibility guard mirrors it; gauge mirrors it | `test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:319` | holds |
| EC-06 | Amount at minimum | `depositAmount > minAmount + 2 * maxFee`; using the full allowance at the boundary leaves anchor `minAmount + maxFee + 1`, enough for one hand-off | `test/bridge/Reservation.test.ts` "guard 11" | holds |
| EC-07 | Fee exactly `txMaxFee` | `fee == txMaxFee` accepted at request, signing, assembly, settlement | `test/bridge/Reservation.test.ts:1100`, `keep-core` `TestAssembleReservationTransactions_FeeBoundaries` | holds |
| EC-08 | Zero amount caps | `0` disables `reservationMaxSingleAmount`, `maxReservationsAmountPerWallet`, `reservationMaxTotalAmount` | `test/bridge/Reservation.test.ts:545`, `:669` | holds |
| EC-09 | Zero structural caps | `maxActiveReservations = 0` blocks all new acceptances; `maxReservationsPerWallet` cannot be written as 0; `maxThirdPartyReanchorHoldsPerWallet = 0` blocks third parties only | `test/bridge/Reservation.test.ts:709`, `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:250` | holds |
| EC-10 | Caps changed while actions pending | Proof accounting preserves confirmed spends. At `9d2086965` signing re-checked current re-anchor target caps; at `806b1106f` the validator no longer re-checks them (D31) | `test/bridge/Bridge.ReservationAcceptanceAuthorization.test.ts:472`; at `806b1106f` `tbtc-v2@806b1106f:solidity/test/bridge/WalletProposalValidator.test.ts:4322-4382` and `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1205` | holds |
| EC-11 | Wallet state change mid-action | settlement strands on the spot only when the custodying/target wallet is Closed or Terminated; a Closing target keeps the position Active and the re-anchor right. Permissionless stranding covers Terminated and Closed always, a Live wallet past `dissolutionEligibleAt`, and a Closing wallet once eligibility **and** the closing period have passed (`tbtc-v2@d1bf5bc281:solidity/contracts/bridge/Reservation.sol:1323-1338`, `tbtc-v2@d1bf5bc281:solidity/contracts/bridge/ReservationProofs.sol:234-253`) | `tbtc-v2@d1bf5bc281:solidity/test/bridge/ReservationProofs.test.ts:1308` (Closed strands on an on-time proof), `:1434` (Closing is not stranded); stranding gate `tbtc-v2@d1bf5bc281:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:186`, `:222`, `:263`, `:313`, `:386`, `:397` | holds at `tbtc-v2@d1bf5bc281` for the corrected rule; the row recorded before VF-10 also listed Closing and omitted the Live and closing-period conditions. The published code and the intended policy agree: a position on a healthy (Live) wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was taken on 2026-10-29 and the code does not change. |
| EC-12 | Ethereum reorg | 12-block event-scan lag limits exposure; deeper cursor rollback is not established | `TestReservationScanRange`; restart scan tests are not a deep-reorg test | holds-untested |
| EC-13 | Bitcoin reorg | Required confirmation/work checks and confirmed-candidate selection reduce exposure; no automatic post-settlement rollback is demonstrated | `TestProveReservationTransaction_SelectsConfirmedRBFCandidate` | holds-untested for deep post-proof reorg recovery |
| EC-14 | Proofs for superseded/vetoed generations | rejected: only `Pending` or `TimedOut` is settleable; late settlement marks newer pending generation `Superseded` | `test/bridge/ReservationProofs.test.ts:353` ("Superseded"), `:1072` | holds |
| EC-15 | Same UTXO used twice | acceptance: `sweptAt` guard; re-anchor: `requireCurrentSourceAnchor` + `spentMainUTXOs` latch | `test/bridge/ReservationProofs.test.ts:805`, `test/bridge/Bridge.ReservationSourceAnchorBinding.test.ts:412` | holds |
| EC-16 | P2PKH vs P2WPKH anchors | both script types accepted on-chain; client emits P2WPKH only | `test/bridge/ReservationProofs.test.ts:659` (P2WPKH accepted), `keep-core` `TestAssembleReservationTransactions_HappyPathShape` | holds |
| EC-17 | Activation block equal to current | Gate accepts work block B; actual work follows coordination cadence; all builds must agree on B | `TestReservationScanRange`; exact coordination-boundary test not identified | holds-untested |
| EC-18 | Last slot race | occupancy counter blocks new requests at the cap; late settlement re-takes the slot without cap checks; unwound generations release it; at `806b1106f` the acceptance request is also bounded by the reserve gate `P + 1 <= max(0, L - R) * N` | `test/bridge/Reservation.test.ts:709`, `test/bridge/Bridge.ReservationAcceptanceAuthorization.test.ts:472`; at `806b1106f` `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322` | holds |
| EC-19 | Concurrent acceptance and re-anchor on one wallet | request-time counters race in the same direction; the client recovers a lost race by excluding the target; at `806b1106f` a privileged hand-off from a non-Live source skips the target caps instead of reverting (PAR-F2) | `keep-core` `TestReservationReanchorTask_CapRevertLeadsToNextCandidate`; at `806b1106f` `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:427` | holds |
| EC-20 | Vault deactivated mid-flight | settlement still credits via the deposit's immutable vault; untrusted or zero vault credits the depositor's Bank balance directly | `test/bridge/ReservationProofs.test.ts:1757` | holds |
| EC-21 | `liveWalletsCount` falls to zero | at `9d2086965` no reservation gate read it; at `806b1106f` the acceptance reserve gate reads it and `L <= R` (including zero) rejects every acceptance; moving-funds commitment and timeout reset revert | `test/bridge/Bridge.MovingFunds.test.ts:455`, `:942`; at `806b1106f` `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379` | holds |
| EC-22 | Deployment progress and network identity | JSON cast is not schema validation; Sepolia config already declares 11155111. The guard merged at `4c98e6b82` rejects a malformed artifact and refuses a provider chain that does not match the execution mode | `tbtc-v2@d1bf5bc281:solidity/test/prepare-artifact-guard.test.ts` | holds |
| EC-23 | D28 wallets registered before reservation activation | at `9c4e1b221` registration discovery starts at block 0 with its own cursor; old retiring sources raise the no-target count and old eligible targets prevent a false one | `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_wallet_discovery_test.go:12-73` | holds |
| EC-24 | D28 observation freshness and catch-up windows | a catch-up scan compares keeper time with the latest chain head; exactly the maximum deviation is fresh in either direction and one second beyond it is a Critical health event | `tbtc-v2@8eb26c2e2:monitoring/test/reservation-keeper-monitor.test.ts:259-328` | holds |

## Full entries

- [Deposit, timing and governance boundaries](05-deposit-timing-edge-cases.md): EC-01 through EC-10.
- [Lifecycle, chain and concurrency boundaries](05-lifecycle-chain-edge-cases.md): EC-11 through EC-24.


## Findings

The live-cap signing observation was reviewed as VF-01.
At `9d2086965` its guards and governance-lowering consequence were real, and the validator stated that current capacity was a signing requirement.
Calling that a confirmed defect would have assumed a different policy without an approved requirement.
D31 then chose the policy: at `tbtc-v2@806b1106f` the validator no longer re-checks target count or amount caps (`tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1092-1098`; EC-10).
See [finding adjudication](12-finding-adjudication.md).

F-2 (low, operational): the activation-order gap (C-4) has no on-chain
alarm. Reproduction idea: call `setVaultStatus(vault, true)` before the
client activation block B; reserved deposits are then revealed on-chain
while clients do not coordinate acceptance. Pending deposits block a
change of the configured `reservationVault` until they clear
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1409-1414`).
Governance can still revoke vault trust with `setVaultStatus(vault, false)`.
That stops new reserved reveals; it does not erase pending deposits or positions.

## Doc drift

| Doc location | What it says | What the code does |
|---|---|---|
| `m1-b-implementation.md:403` | Fee-reserve gauge is named `reservation_vault_fee_reserve_tbtc` | The registered gauge is `reservation_vault_fee_reserve_tbtc_base_units` (`keep-core:pkg/clientinfo/performance.go:786`) |
| `requirements.md:261` (caveat, watcher registration "had zero production callers") | describes a stale state | `WireReservationWatchers` wires all three watchers with restart and death accounting (`keep-core:pkg/maintainer/spv/reservation_wiring.go:301`); the caveat is marked stale in that same file |
| `architecture.md:848` | discusses "unpause flag on the milestone-1 vault" | the pinned vault is `Ownable`, not upgradeable and with no pause surface (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:46`); Option B is confirmed |
