# 07 - Disaster recovery (M1 UTXO reservations)

Scope: M1 reservations only, both repos. Pinned commits: tbtc-v2 `reservations-upgrade` @ `9d2086965` (`tbtc-v2`), keep-core `reservations-epic` @ `f99ef96e3` (`keep-core`). The code is the source of truth. D31 (privileged hand-off plus acceptance reserve) is integrated on the reservations branches at tbtc-v2 `reservations-upgrade` @ `806b1106f` and keep-core `reservations-epic` @ `d40f6aadc`; those facts carry an explicit revision in their citation, and an unqualified citation still means the original pin. Nothing is deployed on Sepolia or mainnet.

Reading rules for this file.

- Each DR item names the bad state, then gives: detection, decision owner, exact recovery steps with function calls and who signs, preconditions, what cannot be undone, money and data at risk, how to verify recovery, and whether the step was ever rehearsed.
- Status legend: `holds` (code enforces the rule and a test pins it), `holds-untested` (code enforces it, no test), `violated` (code breaks it: give the path), `design-only` (planned, not in code), `unknown`.
- "Governance" means the active BridgeGovernance contract and its authorized owner acting through its wrappers. Proxy-admin authority is a separate deployment role. A proxy-upgrade timelock is not a delay on every owner setter.
- "SPV maintainer" means the keep-core maintainer process authorized in the `isSpvMaintainer` allow-list; it is the only caller of the two proof functions.
- "Anyone" means a permissionless caller; keep-core's watchers run these but any EOA can.

## Role quick reference

| Role | On-chain identity | Signs |
|---|---|---|
| Governance | BridgeGovernance contract; authorized owner controls its wrappers | Vault trust, router setup, maintainer authorization and begin/finalize ceremonies (`tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:308`, `:1811`) |
| SPV maintainer | allow-listed in `isSpvMaintainer` | `submitReservationAcceptanceProof`, `submitReservationReanchorProof` (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:268`, `:294`) |
| Wallet member | ECDSA signing group member of the source wallet | `requestReservationReanchor` with a non-empty `walletMembersIDs` proof (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:239`) |
| Reservation owner | `reservation.owner` | Re-anchor request exempt from the third-party hold cap, but not the cooldown or the Live-source governance gate (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:758-799`, at `9d2086965`). At `806b1106f` a request off a non-Live source also skips the target caps (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`); the cooldown bypass stays governance-only (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:786-791`) |
| Anyone | any EOA (keep-core watchers) | `notify*` timeouts, stranding, stale deposits, `repayInKindFeeDebt` |

---

## DR-01 Saturation: a retiring wallet has no target with capacity

Bad state. A retiring wallet's anchors cannot move because no eligible
Live target has count or amount capacity. At `9d2086965` that alone blocked
every re-anchor. At `806b1106f` it blocks only third-party requests and
governance rotation off a Live source: a privileged request (owner, SPV
maintainer, proven source-wallet member, or governance) off a `MovingFunds`
or `Closing` source skips both target caps
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`,
`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:928-933`,
`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:942-948`).
The privileged hand-off does not remove the need for a Live target, a
signing quorum, a delivered proof or a funded caller. The acceptance reserve
(`handoffReserveWallets`) bounds new acceptances only, at request time
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1077`).
It does not guarantee amount headroom, and hand-offs above the cap
concentrate anchors on one wallet.
Raising only `maxActiveReservations` does not free a target slot.

- Detection: inspect each target's state, reserved count, reserved amount and pending holds.
  Global occupancy gauges alone do not show usable target capacity.
  Read `reservationHandoffReserveWallets()` and `liveWalletsCount` to see
  whether the acceptance gate is shrinking admission
  (`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:519`).
  D28 adds a Critical no-target alert for privileged retiring-source hand-offs
  (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:123-208`,
  `tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:165-172`).
  It does not attest cooldown expiry, member authorization or signing-quorum availability.
- Decision owner: governance for parameter changes; operators for eligible wallet creation and proof submission.
- Recovery steps:
  1. Determine the limiting guard. If the blocked request is privileged
     and the source is not Live, target capacity is not the limit:
     check for a Live target, the signing quorum and the proof path
     instead. For a third-party or Live-source request, count capacity is
     controlled by `maxReservationsPerWallet` in `beginReservationParametersUpdate`.
     Amount capacity is controlled by `maxReservationsAmountPerWallet`
     in `beginReservationCapsUpdate`.
  2. Have a privileged caller submit the hand-off. keep-core attaches a
     source-wallet member proof to a fresh request
     (`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:262-290`) and,
     when no candidate has headroom, falls back to the least-loaded eligible
     Live wallet
     (`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:1153-1241`).
     A non-member operator still sends nothing when every target is full
     (`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1611`).
  3. If caps must change, preserve the configured vault and all unrelated
     parameters. `beginReservationCapsUpdate` takes five arguments at
     `806b1106f`: max amount per wallet, max single amount, max active
     reservations, max third-party holds per wallet, and the hand-off reserve
     (`tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896`,
     selector `0x4c1cda81`; the four-argument form `0x3684e886` was the shape
     at `9d2086965`). A corrective update must resend the current values of
     the fields it does not change, and the reserve must be above zero
     ("Hand-off reserve must be greater than zero",
     `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`).
     Simulate the corrective values against the current feasibility and Decision-1 guards.
     Stage and finalize only the approved changes after their governance delay
     (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1362-1460`, `:1494-1536`, at `9d2086965`).
     New Live wallets are another source of capacity; their creation is not instantaneous.
  4. Wait for any timed-out target holds to be explicitly released.
     An eligible caller requests re-anchor; the wallet signs and an authorized maintainer proves it.
  5. If the source remains `MovingFunds` with a below-dust main balance,
     zero reservations permit `notifyMovingFundsBelowDust`.
     If it is already `Closing`, finalize after zero count and the closing period.

Two wallet states require different responses.
Without a main-funds proof, `MovingFunds` can reach its slashing timeout.
After a main-funds proof, `notifyWalletFundsMoved` calls `beginWalletClosing`
(`tbtc-v2:solidity/contracts/bridge/Wallets.sol:438-441`, `:649-660`).
That source is `Closing`, not permanently `MovingFunds`.
It remains re-anchorable and can be stranded after `dissolutionEligibleAt`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:802-804`, `:1192-1216`).

- Preconditions: the source must still support the requested transition.
  A Terminated or Closed source cannot start an ordinary re-anchor.
  Governance recovery may take longer than the remaining timeout.
  A privileged request still needs a Live target other than the source.
- Cannot be undone: stranding releases accounting, not Bitcoin custody or TBTC already minted.
- Money at risk: anchor value, retained in-kind backing and, on the unproven MovingFunds path, operator stake.
- Verify recovery: the proof updates custody; the source count reaches zero.
  A privileged hand-off can leave the target above its caps, so target
  headroom is not a recovery condition for it. A retired source does not return to Live.
- Test: a privileged hand-off to a full target succeeds for the owner,
  an SPV maintainer and a proven member, a third party reverts, and a
  hold-occupied target does not block it
  (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:427`,
  `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`,
  `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:553`).
  The mainnet fork run (a local mainnet-fork run (no pin)) exercised
  a privileged above-cap hand-off. The complete saturation recovery has
  not been rehearsed on Sepolia, and nothing is deployed there.
- Status: `holds-untested`.

## DR-02 Stranded positions: the custodying wallet is Terminated or Closed

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

Bad state. A wallet is Terminated or Closed while an unresolved position remains. The protocol no longer permits an ordinary re-anchor request from that source. Stranding releases tracking; it does not prove that the Bitcoin output is physically unspendable.

- Detection: a `WalletTerminated` or `WalletClosed` event. keep-core's stranding watcher subscribes to `OnWalletClosed` and re-checks at startup (`keep-core:pkg/maintainer/spv/reservation_wiring.go:337`, `:462`).
- Decision owner: anyone (permissionless). keep-core's SPV maintainer is the standing caller.
- Recovery steps:
  1. Call `notifyReservationStranded(reservationKey)`. For a Terminated or Closed wallet it is immediately admissible; for a Closing wallet it is admissible only once `block.timestamp >= dissolutionEligibleAt` **and** `block.timestamp >= closingStartedAt + walletClosingPeriod`; for a Live wallet once `block.timestamp >= dissolutionEligibleAt` (`tbtc-v2@4c98e6b82:solidity/contracts/bridge/Reservation.sol:1311-1322`). The closing-period half is the owner's exclusive re-anchor window; notifying earlier reverts. The stranding watcher dispatches by state and takes the `Live` branch since keep-core #4365, notifying once the reservation reaches `dissolutionEligibleAt`; the branch runs on the startup catch-up scan and on the action-timeout recheck queue, so a dissolution-eligible `Live` position outside both still needs another caller (`keep-core@4ced1c839:pkg/maintainer/spv/reservation_stranding_watch.go:155-176`).
  2. On-chain, `strandReservation` releases the global and per-wallet capacity, deletes the reverse anchor index, sets the position to `Stranded`, and emits `ReservationStranded` as recovery evidence exactly once (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1101`).
- Preconditions: the position is `Active` (`tbtc-v2@4c98e6b82:solidity/contracts/bridge/Reservation.sol:1301-1304`). A pending action generation cannot be stranded; it waits for its timeout.
- Cannot be undone: stranding is terminal for the in-kind position. The owner keeps the ordinary pooled TBTC but loses the in-kind option. There is no M1 on-chain path to restore it except a late proof of an already-signed, timed-out re-anchor (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:700`, DR-10).
- Money and data at risk: the anchor's in-kind claim; the shortfall is absorbed by all TBTC holders. There is no on-chain compensation; `ReservationStranded` is evidence only (`keep-core` `pr-4282-audit/m1-integrator-due-diligence.md` D6 row).
- Verify recovery: `reservations[key].state == Stranded`, `activeReservationsCount()` decremented, `walletReservationsCount()` of the dead wallet at 0, the `ReservationStranded` log present.
- Rehearsed: unit-tested for Terminated (`tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts:554`), Closed and Terminated decrement (`test/bridge/Bridge.ReservationOccupancy.test.ts:110`, `:143`), and the keep-core watcher has unit tests (`keep-core:pkg/maintainer/spv/reservation_stranding_watch_test.go`). Not rehearsed on a fork/Sepolia: never.
- Status: `holds` (code enforces stranding; unit tests pin the transition and counter decrement). The Closing step of this procedure was incomplete before VF-10 - it named dissolution eligibility only, which does not match the published code - and the Live step was missing; both are corrected here (`tbtc-v2@4c98e6b82:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:222`, `:263`, `:313`, `:397`).

## DR-03 Wrong parameters staged or finalized (governance typo)

Bad state. A `begin*Update` staged a wrong value, or a `finalize*Update` applied one. Two sub-cases: a staged-but-not-finalized value, and an already-finalized wrong value.

- Detection: the `*UpdateStarted` / `*Update` event log (`tbtc-v2:solidity/contracts/bridge/BridgeGovernanceParameters.sol:356`). The keep-core SPV reads live parameters and raises `reservation_params_infeasible` if reserved deposits become impossible (`keep-core:pkg/maintainer/spv/reservation_params_feasibility.go:91`).
- Decision owner: governance.
- Recovery steps:
  - Staged, not yet finalized: sign a new `begin*Update` with the corrected value. A second begin overwrites the staged struct and the `changeInitiatedTimestamp`, so the first value is discarded before it can finalize.
  - Already finalized: sign a corrective `begin*Update` + `finalize*Update` for the right value.
  - Wrong vault address inside a parameter update: it only applies while `reservationTotalAmount == 0` and `pendingReservedDeposits == 0` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1409`). If positions exist, the finalize reverts with "Active reservations exist" and nothing changed; the recovery is to correct the value and re-finalize.
- Preconditions: for a vault re-point, the system must be empty of reservations and pending reserved deposits. While a vault is set, every parameter and cap write is also run through the feasibility guard (`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:681`).
- Cannot be undone: a finalized parameter change applies prospectively to future terms and actions; it never rewrites a position that already snapshotted the old values (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:325`, `:330`). So a wrong `reservationTermSeconds` that was already snapshotted into accepted positions cannot be retro-fixed.
- Money and data at risk: if a wrong cap or fee is finalized while positions exist, in-kind claims may be miscalculated on the next hop; a wrong `reservationMaxTotalAmount` can make new acceptances revert.
- Verify recovery: the corrected `ReservationParametersUpdated` / `ReservationCapsUpdated` event; `reservationParameters()` and `reservationCaps()` views return the intended values. At `806b1106f` the caps ceremony has five arguments and `ReservationCapsUpdated` carries the hand-off reserve (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:402-408`), but `reservationCaps()` stays a four-value view (`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:495`); read the reserve with `reservationHandoffReserveWallets()` (`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:519`). A typo in any argument is overwritten by a new `beginReservationCapsUpdate` before finalize (`tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896`).
- Rehearsed: the begin/finalize ceremony and the "stages and finalizes the cap through the begin/finalize ceremony" case are unit-tested (`tbtc-v2:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:415`, `:432`, at `9d2086965`; at `806b1106f` the cap staging is `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:583`, and the five-argument begin and finalize are `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4705` and `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4789`). The mainnet-calldata script rejects a zero reserve (`tbtc-v2@806b1106f:solidity/test/deploy/98_generate_reservation_mainnet_calldata.test.ts:486`). Not rehearsed on Sepolia: nothing is deployed there.
- Status: `holds` (the overwrite-before-finalize and empty-system re-point rules are in code and unit-tested).

## DR-04 Vault compromised or misconfigured

Bad state. The reservation vault is set to an address whose `receiveBalanceIncrease` behaves wrongly, or its fee state is mis-sized. Two distinct faults.

Fault A, incorrect credit provenance. A trusted vault not wired as the configured reservation vault can process ordinary pooled sweep credit through the reservation fee/mint handler. Its `onlyBank` check remains in force (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:147-195`). VF-02 confirms misrouting; it does not establish fabricated Bitcoin or arbitrary minting.

Fault B, empty fee reserve. `feeReserveTarget` is 0 on Sepolia (D21), so `sweepFees` may sweep the vault's entire TBTC balance to the treasury, leaving no reserve to finance in-kind fees; each shortfall grows `inKindFeeDebtSat` (DR-09).

- Detection: Fault A is not observable after the fact without an on-chain cross-check; it is a pre-activation risk. Fault B is the `reservation_vault_fee_debt_sat` gauge rising (`keep-core:pkg/chain/ethereum/tbtc.go:1779`).
- Decision owner: governance (deactivates the vault) plus the proxy admin for a code fix.
- Recovery steps:
  1. Governance signs `setVaultStatus(vault, false)` (`tbtc-v2:solidity/contracts/bridge/Bridge.sol:1288`). For a late acceptance proof that arrives after deactivation, the depositor is credited directly with no fee, not through the buggy vault (documented in `m1-guide.md:120`). This stops the untrusted vault from receiving new credits.
  2. While `isVaultTrusted(vault) == false`, `requestReservationAcceptance` still requires `deposit.vault == self.reservationVault` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:521`), so deactivating the trust flag does not by itself disable new acceptances; governance must also re-point the vault (a new deploy) or disable reservations (`reservationVault = 0`, which requires the system to be empty).
- Preconditions: vault re-pointing requires zero `reservationTotalAmount`
  and zero `pendingReservedDeposits` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1409-1414`).
  Acceptance timeout can release pending capacity; it does not close an accepted position.
  Revoking trust is allowed without satisfying these vault-change guards.
- Cannot be undone: TBTC already transferred is not clawed back by trust revocation.
- Money at risk: a misrouted ordinary credit can break reservation provenance.
  A Bank credit alone does not prove arbitrary unbacked minting.
  Fee debt separately measures the uncompensated Bitcoin fees described in DR-09.
- Verify recovery: `isVaultTrusted(vault) == false`; no new `ReservationCreditProcessed` through the vault; `inKindFeeDebtSat` stable or falling.
- Rehearsed: none. Fault A is a known gap with no rehearsed mitigation; the deactivation path (`setVaultStatus(false)`) has no reservation-specific fork/Sepolia rehearsal. Never.
- Status: `holds-untested` for trust revocation and the complete incident procedure. The provenance fix is not implemented; its design is not silently selected by this recovery guide.

## DR-05 Router set to the wrong address (write-once)

Bad state. `setReservationRouter` was finalized with a wrong or broken address. The Bridge fallback delegates every unmatched selector to `self.reservationRouter` (`tbtc-v2:solidity/contracts/bridge/Bridge.sol:2119`); if that address holds bad code, every reservation call reverts. `setReservationRouter` can only run while the slot is zero (`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:1169`), so the address cannot be re-set.

- Detection: `getReservationRouter()` (`tbtc-v2:solidity/contracts/bridge/Bridge.sol:2098`) returns the wrong address; every reservation selector reverts.
- Decision owner: governance plus the Bridge proxy admin.
- Recovery steps: there is no on-chain re-point. The only path is a Bridge implementation upgrade (the proxy admin swaps the Bridge implementation so the fallback or the `reservationRouter` slot is corrected), which is the same ceremony as any Bridge logic change. Because the router is a plain non-upgradeable contract, the code at the stored address cannot be replaced in place (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:89`).
- Preconditions: the proxy admin key is available; a new Bridge implementation is built and deployed.
- Cannot be undone: the reservation subsystem is dead until the upgrade. Any reserved deposit revealed in the meantime can only be refunded on Bitcoin after its locktime; it cannot be accepted because the acceptance surface is the router.
- Money and data at risk: all in-flight reservations are frozen; pending acceptances time out and depositors refund.
- Verify recovery: after the upgrade, `getReservationRouter()` is correct and a no-op reservation selector succeeds.
- Rehearsed: the set-once and non-zero and must-be-a-contract reverts are unit-tested (`tbtc-v2:solidity/test/bridge/Bridge.Parameters.test.ts:2016`, `:2051`, `:2072`). The wrong-address recovery (Bridge upgrade) was not rehearsed: never.
- Status: `design-only` (the recovery is a proxy-admin ceremony, not a reservation-subsystem feature; the write-once lock itself is in code and unit-tested, so the constraint `holds`).

## DR-06 Bridge upgrade gone wrong (router replacement breaks storage parity)

Bad state. A Bridge implementation upgrade intended to change the router (or otherwise) mis-handles the reservation router's storage model. The router runs via `delegatecall` on the Bridge's own storage and is required to inherit the same storage-bearing bases in the same order, with `BridgeState.Storage self` as its only state variable (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:59`). A mismatch corrupts every reservation read and write.

- Detection: the storage-parity and selector-disjointness guards (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:66`). After an upgrade, run `Bridge.StorageLayout.test.ts` and `Bridge.RouterStorageParity.test.ts`; a drift in the `self` slot breaks every reservation call.
- Decision owner: governance plus the proxy admin.
- Recovery steps: there is no partial undo. The proxy admin must re-upgrade to a known-good Bridge implementation whose router storage layout matches. Any reservation state written through the broken layout is suspect and must be audited before continued use.
- Preconditions: a known-good implementation is available; the reservation state written during the broken window is identified and validated.
- Cannot be undone: corrupted reservation slots cannot be rolled back by a later upgrade unless the upgrade itself repairs the layout; data written under the wrong layout is lost.
- Money and data at risk: all reservation position state (owner, anchor, amount, expiry) is at risk during the broken window.
- Verify recovery: storage-parity and selector-disjointness tests pass; a read of a live reservation returns coherent fields.
- Rehearsed: the invariants are unit-tested (storage parity, selector disjointness). A wrong-upgrade recovery was not rehearsed on a fork/Sepolia: never.
- Status: `holds-untested` (the guards are in code and tested; the recovery ceremony is not rehearsed).

## Deployment prerequisites

Next to the C-4 activation ordering gate (`architecture.md` §8) there is a second ordering prerequisite. `Redemption.sol` calls `RebateStaking(...).isRebateAuthorized` directly and reverts when the staking contract lacks that selector (`tbtc-v2@4c98e6b82:solidity/contracts/bridge/Redemption.sol:274-284`), so a compatible staking upgrade must land before or with the Bridge. The staking upgrade is an outstanding prerequisite; this set does not name who arranges it.

## DR-07 keep-core release with the wrong activation block

Bad state. A client binary ships with `reservationsActivationBlocks[network]` set to the wrong block B (too early, or a placeholder). If B is too early, governance's `setVaultStatus(vault, true)` can fire before operators run the binary, so reserved deposits arrive with no client to coordinate anchors. The gate is the single source of truth; a network with no entry falls through to `math.MaxUint64` and the feature never activates (`keep-core:pkg/tbtc/coordination.go:110`).

- Detection: each operator's `/diagnostics` client version versus the shipped activation table; the mismatch between the on-chain activation block and the binary's B. The drill runbook makes this step 2-5 of the C-4 order (`testnet-drill-runbook.md:20`).
- Decision owner: the release owner (choose B, publish the binary) and governance (do not activate before B).
- Recovery steps:
  1. If the binary was released with a wrong B and activation has not happened, ship a corrected binary with the right B and roll it out to every operator; verify each operator's client version before proceeding.
  2. If activation happened too early, revoke vault trust to stop new reveals. Restore the corrected client and proof service. Before timing out signed actions, check whether their Bitcoin spends confirmed and remain provable. Reactivate only after operator readiness and the corrected block B.
- Preconditions: a corrected binary is built; every operator upgrades before reactivation; for a network with no table entry, an entry must be added (the table is empty in the drill build, `keep-core:pkg/tbtc/coordination.go:94`).
- Cannot be undone: a too-early activation that minted positions cannot be taken back; those positions are real. Only the forward reactivation is controllable.
- Money at risk: unspent funding can refund after locktime, or use D26 release before any acceptance request. If an anchor already spent the funding output, refund is impossible and proof recovery becomes urgent.
- Verify recovery: every operator's client version matches the corrected table; `ReservationsActivationBlock(network)` on each node returns the intended B; no reservation action precedes activation.
- Rehearsed: the "no entry means never" behavior is unit-tested (`keep-core:pkg/tbtc/coordination_test.go:1608`, `TestReservationsActivationBlock_SanityCheck`). The wrong-B recovery (re-ship + deactivate/reactivate) was not rehearsed: never.
- Status: `holds-untested` (the gate and the fall-through are in code and tested; the recovery procedure is not rehearsed).

## DR-08 Mass timeout: a large batch of actions all time out together

Bad state. A chain halt, a keyset outage, or a fee spike makes no wallet sign and no SPV proof submit. Many pending generations cross their `timeoutAt` at once. Capacity reserved at request time is held until someone calls the timeout, so the caps stay consumed and new acceptances revert.

- Detection: `spv_reservation_action_overdue_total` climbing; `activeReservationsCount()` pinned near the cap with many `Pending` actions past `timeoutAt`.
- Decision owner: anyone (timeouts are permissionless). keep-core's `ReservationActionTimeoutWatcher` is the standing caller (`keep-core:pkg/maintainer/spv/reservation_action_timeout_watch.go:64`).
- Recovery steps:
  1. For each re-anchor generation, call `notifyReservationActionTimeout(reservationKey)` once `block.timestamp >= action.timeoutAt` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:958`). This releases the target's reserved count and amount, drops any third-party hold, restores the position to `Active` on the source wallet, and sets a re-anchor cooldown equal to the signing window (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:984`).
  2. For each acceptance generation, call `notifyReservationAcceptanceTimedOut(reservationKey)` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:688`). This releases the reserved capacity and leaves the deposit eligible for a fresh request.
  3. The keep-core watcher re-offers a still-Pending action every `actionTimeoutRenotifyInterval` (10 minutes) so a dropped notification retries (`keep-core:pkg/maintainer/spv/reservation_action_timeout_watch.go:25`).
- Preconditions: the action is still `Pending` and the position is in the matching state (`ActionPending` for re-anchor, `Unknown` for acceptance).
- Cannot be undone: a re-anchor timeout starts a cooldown; during it only a governance call may request a new re-anchor. A signed-but-unproven generation that times out can still be settled late (DR-10), so the timeout does not forfeit a confirmed Bitcoin tx.
- Money and data at risk: capacity stays locked only until the timeout lands; the anchors themselves are unaffected by the timeout.
- Verify recovery: acceptance timeouts decrement global occupancy. Re-anchor timeouts leave global occupancy unchanged and release only target capacity and third-party holds. A privileged hand-off that skipped the target caps released the same count and amount, because both are added on every path and released unconditionally (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:934-935`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1130-1131`). Confirm `TimedOut` actions and the expected per-wallet counter changes.
- Rehearsed: the decrement-and-event behavior of acceptance timeouts is unit-tested (`tbtc-v2:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:68`); the "drops on action timeout" hold release is unit-tested (`test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:271`). A mass-timeout storm was not rehearsed: never.
- Status: `holds` (the timeout paths are in code and unit-tested; the keep-core retry loop has unit tests).

## DR-09 Fee debt growth: the vault cannot finance in-kind miner fees

Bad state. `inKindFeeDebtSat` is non-zero and rising: re-anchor hops are consuming more Bitcoin miner fee than the vault's TBTC reserve can burn, so total TBTC supply exceeds the Bitcoin backing by that amount (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:80`). On Sepolia the reserve target is 0 (D21), so `sweepFees` can drain the reserve and the debt grows instead of settling.

- Detection: retain the original fee/reserve gauges. D28's observer reads the configured vault debt (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:170-187`); any positive debt produces a Warning (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:173-180`). This is not a trend-only alarm.
- Decision owner: the vault owner for the reserve target and sweep; anyone for the repayment.
- Recovery steps:
  1. Anyone signs `repayInKindFeeDebt(amountSat)` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:241`): the caller's TBTC is pulled, burned with the matching Bank balance, and the debt reduced. This is permissionless and the primary sink.
  2. The vault owner raises `feeReserveTarget` (`updateFeeReserveTarget`, `:278`) so `sweepFees` keeps enough TBTC to finance future hops; `sweepFees` repays debt from the full balance before moving surplus to the treasury (`:302`).
- Preconditions: for `repayInKindFeeDebt`, the caller holds TBTC. For a raise, the owner key controls the vault.
- Cannot be undone: a repayment burns TBTC; it is not reversible. A too-high reserve target only withholds fee revenue from the treasury, it does not risk settlement.
- Money and data at risk: while the debt is non-zero, supply is over by that exact amount, publicly visible. It is not a user loss per se; it is a protocol solvency gap until repaid.
- Verify recovery: `inKindFeeDebtSat` returns to 0; the `InKindFeeDebtRepaid` log.
- Rehearsed: shortfall and repayment are unit-tested at the original pin (`tbtc-v2:solidity/test/vault/ReservationVault.test.ts:273`, `:449`). The D28 local fork also created 1000 sat of debt through a real re-anchor, delivered the Warning and cleared debt through `repayInKindFeeDebt`; the repayment control emitted no keeper alert (`keeper-alert-acceptance.md:6-49`). This used the labelled fork relay fixture, not a live Sepolia incident or an automatic repayment policy.
- Status: `holds` (the debt and repayment mechanics are in code and unit-tested).

## DR-10 Relay or SPV outage near the late-proof cutoff (timeoutAt + termSeconds)

Bad state. A confirmed acceptance spend has no successful SPV settlement and its action has been marked `TimedOut`. After `timeoutAt + termSeconds`, its late proof reverts (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:192-210`). The spent funding output cannot use the depositor's refund script. This is an uncredited-custody failure, not a refund recovery.

- Detection: an `spv_proof_task_failures_total` or `spv_proof_skipped_*` counter rising with a known signed Bitcoin tx that is unproven; the SPV process log.
  D28 adds a dedicated Critical count for unproven acceptance generations near their snapshotted edge
  (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:85-106`,
  `tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:157-164`).
  Its local delivery is recorded in the approved-delta chapter; it does not establish recovery across the cutoff.
- Decision owner: the authorized SPV maintainer restores proof service. Governance can authorize an independent maintainer when its administrative keys remain available. No key sharing is required.
- Recovery steps:
  1. Bring the SPV maintainer back and let its proof loop catch up. The loop walks from the activation block in chunks and stays 12 blocks behind the tip (`keep-core:pkg/maintainer/spv/reservation_proof_loop.go:460`), so a caught-up rescan finds the missed anchor and re-anchor tx.
  2. The maintainer signs `submitReservationAcceptanceProof` or `submitReservationReanchorProof` for the pending generation (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:268`, `:294`).
  3. If the generation already timed out, it settles as a late proof. For a late acceptance the `timeoutAt + termSeconds` bound applies; for a late re-anchor the target capacity is re-taken with no cap check (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:700`), and a newer pending generation that references the same anchor is unwound (`:714`).
- Preconditions: the SPV maintainer key is in the allow-list; the Bitcoin tx is confirmed with enough work; for a late re-anchor the source anchor is still the current one (`requireCurrentSourceAnchor`, `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:311`).
- Cannot be undone through ordinary M1 calls: an expired `TimedOut` acceptance cannot settle late.
  A still-`Pending` action has a different admission rule, but relying on missing timeout cleanup is not a recovery procedure.
- Money at risk: the confirmed anchor may remain uncredited. A new request cannot recreate the already-spent funding input.
- Verify recovery: `ReservationLateSettled` or `ReservationReanchored` event; the position back to `Active` on the target; `cumulativeReanchorFee` advanced.
- Rehearsed: late settlement of a timed-out older generation and the late-strand-on-closed-target path are unit-tested (`tbtc-v2:solidity/test/bridge/ReservationProofs.test.ts:851`, `:1156`). The window bound itself (the string "Late acceptance settlement window expired", `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:210`) is pinned by unit tests on the tbtc-v2 reservations branch (not at the default pin): `tbtc-v2@998451722:solidity/test/bridge/ReservationProofs.test.ts:437` (a late acceptance still loads at the last second), `:449` (it reverts one second later) and `:462` (a timed-out re-anchor is not bounded). They ran green on 2026-10-12, and each one fails when its part of the guard is mutated. The near-cutoff outage scenario was not rehearsed on a fork or on Sepolia.
- Status: `unknown` for the full incident. The settlement bound and dedicated D28 alert are implemented and tested, but the outage and proof recovery across the cutoff remain unrehearsed. A separate keeper's proof authority is still a deployment fact; local alert delivery does not establish live allow-list or signing readiness.

## DR-11 Key loss: the governance (Bridge owner) key is lost

Bad state. The Bridge owner (Council Safe / Timelock on mainnet, the deployer on Sepolia) cannot act. Every governance-gated recovery in this file (cap raises, parameter fixes, vault deactivation, router set, SPV maintainer allow-list changes) is blocked.

- Detection: failed or unavailable authorized administrative signatures. Caller-check reverts show wrong identity; they do not prove key loss.
- Decision owner: depends on the owner's shape. On mainnet the owner is a Safe plus Timelock, so loss of one signing key is absorbed by the remaining Safe signers and the timelock path; total loss of the Safe threshold is unrecoverable on-chain. On Sepolia the owner is a single deployer EOA; loss of that key is unrecoverable on-chain.
- Recovery steps:
  - Partial loss: if the owner still meets its signing threshold, transfer ownership of the governance contract with `transferOwnership`. Replacing the Bridge's governance contract is a different operation: stage a validated new contract with `beginBridgeGovernanceTransfer`, then finalize after the delay (`tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:362-397`).
  - Full loss: no on-chain path. The governance surface is frozen.
- Preconditions: for a transfer, the current Safe still meets its threshold and the timelock delay can elapse.
- Cannot be undone: a governance transfer is prospective; the old owner is displaced. A full loss is permanent for the reservation subsystem (and all of tBTC governance).
- Money and data at risk: every governance-gated recovery in DR-01, DR-03, DR-04, DR-05, DR-06, DR-09 is unavailable; reservations can only move by permissionless re-anchor off a retiring wallet or by stranding.
- Verify recovery: `governance()` returns the new owner; a governance call from it succeeds.
- Rehearsed: none. Key loss has no fork/Sepolia rehearsal. Never.
- Status: `design-only` (the transfer path is in code and is the only mitigation; a total loss has no M1 recovery and is recorded in the no-recovery list below).

## DR-12 Key loss: the vault owner key is lost

Bad state. The reservation vault is `Ownable` and not upgradeable (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:46`). The vault owner controls `sweepFees`, `updateInitiationFee`, and `updateFeeReserveTarget`. Losing that key freezes those functions.

- Detection: the owner cannot provide the required signature. An `onlyOwner` revert from a different address is not evidence of key loss.
- Decision owner: the deployer that created the vault. In the Sepolia runbook, vault ownership is explicitly transferred to a dedicated role (`tbtc-v2` deploy script `96_transfer_reservation_vault_ownership.ts`), so loss of the transfer target key is the failure.
- Recovery steps:
  1. There is no `transferOwnership`-free reset. The vault inherits OZ `Ownable`, so the owner can call `transferOwnership(newOwner)` to a new key before losing the key; after total loss, the owner-gated functions are frozen.
  2. Fee debt (DR-09) stays repayable by anyone via `repayInKindFeeDebt`, which is not owner-gated, so the debt sink survives a lost owner key.
  3. Re-pointing the vault itself is governance-gated and requires the system to be empty (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1409`); a lost vault-owner key does not block a governance re-point, it blocks fee tuning and sweeps only.
- Preconditions: for the pre-loss transfer, the old owner key is still available.
- Cannot be undone: a frozen `sweepFees` and fee functions stay frozen; the vault code is not upgradeable, so the functions cannot be patched into a new key path without deploying a new vault and migrating.
- Money and data at risk: fee revenue is stranded in the vault beyond the target; the initiation fee stays at its last value. No deposit is at risk.
- Verify recovery: `owner()` returns the new key; an owner call from it succeeds.
- Rehearsed: none. Never.
- Status: `holds-untested` (the OZ `Ownable` transfer path is standard and the ownership transfer is scripted in deployment; a lost-key recovery is not rehearsed).

---

## States with no M1 recovery path

These bad states have no on-chain recovery inside M1; the only levers are the ones listed. They are deliberate properties of variant B.

| State | What cannot be recovered in M1 | Only lever | Why |
|---|---|---|---|
| Stranded in-kind claim (DR-02) | Ordinary M1 exit to the owner's Bitcoin address | Eligible late re-anchor proof, if one exists; otherwise no ordinary restore call | Stranding changes tracking and leaves TBTC balances unchanged; it does not transfer Bitcoin |
| `closeReservation` path | Position close for a live owner | None in M1 | `closeReservation` has no caller; it is reserved for M2 settlement (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1137`) |
| Wrong router address (DR-05) | Re-pointing `reservationRouter` | A Bridge proxy-admin upgrade only | `setReservationRouter` runs once (`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:1169`) |
| Governance key total loss (DR-11) | All governance-gated recovery | None, unless the Safe still has a threshold | Owner is the only caller of the begin/finalize and onlyOwner surface |
| Vault owner key total loss (DR-12) | `sweepFees`, fee tuning | A pre-loss `transferOwnership`; `repayInKindFeeDebt` is the one non-owner sink that survives | Vault is `Ownable`, not upgradeable |
| Accepted positions while vault change is blocked | Swapping the configured vault | Requires zero reservation amount and zero pending deposits; accepted positions do not clear by timeout | `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1409-1414` |
| Closing wallet that cannot finalize | Automatic transition to Closed | Re-anchor or eligible stranding, then explicit `notifyWalletClosingPeriodElapsed` after its period | The permissionless finalizer requires zero count (`Wallets.sol:363-389`) |

---

## Doc drift

| Doc path:line | What it says | What the code does |
|---|---|---|
| `keep-core` audit runbook R2 | Closing wallets have no automated re-anchor or stranding caller | Re-anchor handles Closing (`keep-core:pkg/tbtcpg/reservation_reanchor.go:170`). The stranding watcher has a Closing-eligible check (`keep-core:pkg/maintainer/spv/reservation_stranding_watch.go:139-162`). Finalization is a separate permissionless call. |
| `m1-guide.md:75` | "A late re-anchor proof has no time limit while the source anchor is still current." | Correct and in code: late re-anchor re-takes capacity with no cap check and no time bound (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:700`); only the late acceptance is time-bound (`:209`). No contradiction; recorded so a reviewer does not read the two as symmetric. |

---

## Findings

DR-04's routing observation was narrowed by VF-02.
The code supplies real trust-revocation and debt-repayment calls.
A complete operational incident recovery has not been rehearsed.
See [finding adjudication](12-finding-adjudication.md); no new code fix is claimed here.
