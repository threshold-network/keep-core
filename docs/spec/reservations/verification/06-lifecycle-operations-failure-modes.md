# Lifecycle, governance, RPC and fee-market failure modes

Evidence pins and vocabulary: [failure modes index](06-failure-modes.md).

### FM-08 Saturation: unproven funds, slashing and termination

Trigger: a wallet in MovingFunds holds reservation anchors but cannot hand
them off. At `9d2086965` a full target blocked it: every target at
`maxReservationsPerWallet` or its amount cap, too young to be a target, or
with its third-party hold cap full
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:869`). At `806b1106f`
a privileged request (reservation owner, SPV maintainer, proven
source-wallet member, or governance) off a source that is not Live skips
both target caps
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`,
`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:928-933`,
`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:942-948`), and
privileged callers are exempt from the hold cap
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:896-906`).
So full targets and third-party holds alone no longer cause this failure
for a privileged request. A third-party request still hits them
(`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`).
What still blocks the hand-off: no Live target other than the source
("Target wallet must be in Live state",
`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:845`), a target
too young for the client's age gate, a source keyset below its signing
quorum (FM-06), or a proof that is never delivered (FM-01, FM-02). The
re-anchor is then never provable; the moving funds timeout elapses with
the funds unproven. The acceptance reserve gate lowers how often this
state is reached: it rejects a new acceptance once the active count passes
`(liveWalletsCount - handoffReserveWallets) * maxReservationsPerWallet`
("Hand-off reserve exceeded",
`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`). It is a
count bound at the acceptance request only. It does not guarantee a Live
target, a signing quorum, proof delivery, amount headroom or fee room, and
a hand-off above the cap concentrates anchors on one wallet.

Detection: `wallet_reservations_count` at the cap and
`active_reservations_count` vs `max_active_reservations`
(`keep-core:pkg/clientinfo/performance.go:770`, `:767`);
`reanchor_waiting_on_aged_target` = 1 when only the target age blocks
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:598`); the acceptance
revert "Hand-off reserve exceeded" and the
`reservationHandoffReserveWallets()` view
(`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:519`);
on chain,
`MovingFundsTimedOut` (`tbtc-v2:solidity/contracts/bridge/MovingFunds.sol:587`)
followed by `WalletTerminated`
(`tbtc-v2:solidity/contracts/bridge/Wallets.sol:703`) and
`ReservationStranded`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:372`).

At the D28 pins, Active positions on retiring sources without an eligible privileged target raise a dedicated Critical alert
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:123-208`,
`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:165-172`).
Wallet discovery includes registrations before activation. The count does not prove signing-quorum availability.

On-chain effect: `notifyMovingFundsTimeout` seizes the signing group
at `movingFundsTimeoutSlashingAmount`
(`tbtc-v2:solidity/contracts/bridge/Wallets.sol:485`) and terminates
the wallet. Each Active reservation then becomes eligible for permissionless
stranding. A submitted `notifyReservationStranded` releases its capacity
counters and leaves the owner's minted balance as an ordinary pooled claim.
This is the accepted M1 cost of saturation: the stranding path is the
release valve, and it needs a caller.

User impact: reservation owners lose the in-kind right on the
affected positions (stranded); custodian operators are slashed.

Automatic mitigation: the re-anchor task retries every window and
skips filled targets to the next candidate
(`keep-core` `TestReservationReanchorTask_CapRevertLeadsToNextCandidate`,
`pkg/tbtcpg/reservation_reanchor_test.go:996`, at `f99ef96e3`). At
`d40f6aadc`, when a fresh request carries a source-wallet member proof
(`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:262-290`) and no
candidate has headroom, the task falls back to the least-loaded eligible
Live wallet (`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:1153-1241`;
`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1479`). A
non-member operator still sends nothing when every target is full
(`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1611`). The
stranding watcher clears the positions after termination.

Manual mitigation: apply the "Saturation: a wallet cannot move its
reservation because no target has room" topic. Have a privileged caller
(the owner, an SPV maintainer, or a proven member) submit the hand-off;
it does not need target capacity. Add Live wallets if none exists. To
help third-party requests too, raise `maxReservationsPerWallet` or the
amount caps.

Test: privileged hand-off to a full target, a third party reverting, and a
hold-occupied target
(`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:452`,
`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`,
`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:553`); the gate
(`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322`,
`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379`). The mainnet fork run
(a local mainnet-fork run (no pin))
exercised the gate rejection and a privileged above-cap hand-off. The full
saturated retirement is not rehearsed on Sepolia, and nothing is deployed
there.

Status: holds.

### FM-09 Saturation after main funds are proven: Closing cannot finalize

Trigger: the ordinary moving-funds transaction is proven while the wallet
still holds reservation anchors. At `9d2086965` its target wallets had no
free capacity, which blocked every re-anchor. At `806b1106f` a privileged
request off a `Closing` source skips the target caps
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`), so
target capacity is no longer required for that request. A Live target,
a signing quorum and proof delivery still are.

Actual state: `notifyWalletFundsMoved` deletes the main UTXO and calls
`beginWalletClosing` (`tbtc-v2:solidity/contracts/bridge/Wallets.sol:438-441`).
That function sets the wallet to `Closing` without a reservation-count gate
(`tbtc-v2:solidity/contracts/bridge/Wallets.sol:649-660`).
The wallet does not remain in `MovingFunds` after this proof.

Detection: `WalletClosing` plus non-zero `walletReservationsCount`.
`notifyWalletClosingPeriodElapsed` rejects with
"Wallet still custodies reservations" until the count reaches zero
(`tbtc-v2:solidity/contracts/bridge/Wallets.sol:363-389`).
D28 also covers Active positions on Closing sources in its no-target count
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:171-203`).
The resulting Critical alert does not finalize Closing or submit a re-anchor.

On-chain effect: final closing is blocked, but the reservation is not
unrecoverable by definition. Re-anchor accepts a `Closing` source.
An `Active` position can also be stranded once
`now >= dissolutionEligibleAt`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:802-804`, `:1192-1216`).
The MovingFunds-timeout branch requires `MovingFunds`, not `Closing`.

Automatic mitigation: keep-core's re-anchor task includes `StateClosing`
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:170-174`).
The stranding watcher also has a Closing-eligible check
(`keep-core:pkg/maintainer/spv/reservation_stranding_watch.go:139-162`).
Availability of these callers remains a liveness assumption.

Manual mitigation: at `9d2086965` make target capacity available, then
re-anchor. At `806b1106f` have a privileged caller (the owner, an SPV
maintainer or a proven member) request the re-anchor off the `Closing`
source; the owner `Closing` case reaches a full target
(`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:435`).
A third-party request still needs target capacity.
Alternatively, submit eligible stranding and accept its economic consequences.
After the reservation count reaches zero and the closing period passes,
call `notifyWalletClosingPeriodElapsed` to finalize.
Raising only `maxActiveReservations` does not create a target-wallet slot.

User impact: the retiring wallet cannot accept new positions.
Existing anchors remain under its Bitcoin key until they move.
Stranding releases protocol accounting; it does not transfer Bitcoin to the owner.

Test: the Closing-source request and stranding tests cited in the user stories
cover individual transitions. At `806b1106f` the owner `Closing` hand-off to a full target is pinned alone (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:435`). No named test covers this entire saturated recovery.

Status: holds-untested.

### FM-10 Bad governance parameters

Trigger: governance stages or finalizes a parameter or cap that is
legal at write time but operationally wrong: a `reservationTxMaxFee`
too small for the fee market, a `reservationMinAmount` so large that
no deposit qualifies, or caps so small that no realistic position
fits.

Detection: `reservation_params_infeasible` = 1 covers only the
infeasibility rules (reveal-ahead period past term + 24h, or a
non-zero cap below the dust threshold)
(`keep-core:pkg/clientinfo/performance.go:797`, mirror rules in
`keep-core:pkg/maintainer/spv/reservation_params_feasibility.go:37`).
For other bad values there is no metric: the on-chain
`ReservationParametersUpdated` / `ReservationCapsUpdated` events
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:389`, `:402`)
are the only record.

On-chain effect: acceptance snapshots preserve its term, minimum and fee
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:636`).
That does not guarantee pending re-anchor signing under changed live target
caps at `9d2086965`: the validator re-checked both target caps at signing.
At `806b1106f` the validator no longer re-checks them; it enforces target
Live state only
(`tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1156-1160`,
doc block `tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1092-1098`),
and a later cap change does not reject an already-admitted re-anchor at
signing (the validator harness accepts an above-cap target,
`keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1205`).
The caps are enforced once, at request time, and only for non-privileged
requests and Live-source rotations. The caps update is a delayed
governance ceremony with five arguments; the fifth is the hand-off
reserve, and zero is rejected
("Hand-off reserve must be greater than zero",
`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`). A
reserve at or above the Live wallet count rejects every new acceptance
(`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379`);
`ReservationCapsUpdated` carries the reserve
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:402-408`).
A new fee estimate can reject before a request is sent
(`keep-core` `TestReservationReanchorTask_FeeAboveCap_SendsNoRequest`,
`pkg/tbtcpg/reservation_reanchor_inflight_test.go:524`).
Already-Pending actions still require explicit completion or timeout cleanup.
Write-time guards:
`min > txMaxFee` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1379`),
term within [90d, 730d]
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1383`),
timeout above `REQUEST_TIMEOUT_SAFETY_MARGIN + DEPOSIT_MIN_AGE + 1`
(smallest accepted value 14,402 s; `tbtc-v2@4c98e6b82:solidity/contracts/bridge/Reservation.sol:1625-1629`), the
feasibility guard on both setters and `updateDepositParameters` while
a vault is set (`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:681`),
and the slot-capacity invariant
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1331`).

User impact: requests, signatures and proofs can be delayed.
The safe refund path requires an unspent funding output.
Signed-and-confirmed spends require proof recovery before their applicable windows close.

Automatic mitigation: the write-time reverts are the mitigation
against the worst writes; nothing auto-corrects a legal-but-wrong
write.

Manual mitigation: apply the "Wrong parameters staged or finalized"
topic: re-stage and finalize corrected values; the D25 guard
prevents the specific "no deposit can ever be reserved" class.

Status: holds.

### FM-11 Activation order wrong (C-4)

Trigger: governance sets the reservation vault live
(`setVaultStatus(vault, true)`) before the keep-core activation block
B for the network is reached, or before operators are running a
binary containing the activation table entry.

Detection: the code has no single alarm comparing operator readiness with vault activation.
Reserved reveals with no matching signing/proof progress are the relevant symptom.
Depositor requests may already exist, so global occupancy need not be zero
(`keep-core:pkg/clientinfo/performance.go:767`).

On-chain effect: reserved deposits and depositor acceptance requests can
exist before clients coordinate anchor signing. keep-core does not submit
the depositor's acceptance request.
Pending deposits prevent a change of the configured reservation vault
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1409-1414`).
Governance can revoke vault trust at any occupancy level.
This blocks new reserved reveals
(`tbtc-v2:solidity/contracts/bridge/Deposit.sol:211`).
It does not erase pending deposits or disable settlement of signed actions.

User impact: unspent funding outputs remain refundable after locktime.
D26 release is also available before any acceptance request.
Already-spent funding outputs cannot be refunded.
Restore a correct client build before the relevant proof windows close.

Automatic mitigation: none by design; the C-4 rule exists precisely
because nothing auto-corrects order.

Manual mitigation: apply the "keep-core release with the wrong
activation block" topic: hold the activation entry at or after the
ship block. Set the vault untrusted to stop new reveals.
Explicit cleanup is still necessary: eligible deposits can use D26 release;
stale notifications require deadline and action guards
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1229`, `:1285`).
Records do not clear automatically when their deadlines pass.

Status: design-only (the order rule is a process gate; the pinned
activation table is empty for public networks,
`keep-core:pkg/tbtc/coordination.go:94`).

### FM-12 RPC failures

Trigger: Ethereum RPC endpoints fail or hang (maintainer or client
side); Bitcoin RPC endpoints fail (fee estimation, transaction
history).

Detection: `spv_maintainer_last_failure_timestamp_seconds` and the
backoff gauge `spv_maintainer_max_backoff_seconds`
(`keep-core:pkg/clientinfo/performance.go:680`); ERROR logs naming
the failing call; watcher deaths increment
`spv_reservation_watcher_deaths_total` when a loop does not recover.

On-chain effect: requests and proofs are not submitted; pending
actions time out as in FM-05. Reads are bounded by timeout so a hung
endpoint cannot wedge signing actions forever: reservation chain
reads run through `rpcCallWithTimeout`
(`keep-core:pkg/tbtc/reservation.go:43`), and a stuck parked call is
discarded (`keep-core`
`TestReservationExecute_RPCTimeoutReleasesDispatcher`,
`pkg/tbtc/reservation_test.go:1509`). The re-anchor target-age gate
fails closed on a head-time read error instead of allowing a young
target (`keep-core`
`TestReanchorCustodyGate_ChainHeadReadFailureFailsClosed`,
`pkg/tbtcpg/reservation_reanchor_custody_test.go:212`).

User impact: delayed or timed-out actions; no state corruption.

Automatic mitigation: chunk-scan failures keep the scan cursor at
the last good chunk and retry after the idle backoff instead of
restarting (`keep-core:pkg/maintainer/spv/reservation_proof_loop.go:20`);
per-item failures leave the skipped item to the next pass
(`keep-core:pkg/maintainer/spv/reservation_proof_loop.go:63`).

Manual mitigation: fail over RPC endpoints; near a late-proof cutoff,
apply the "Relay or SPV outage near the late-proof cutoff" topic.

Status: holds.

### FM-13 Bitcoin fee spike above txMaxFee

Trigger: the fee market moves so the estimated fee for an anchor or
re-anchor exceeds the snapshotted `reservationTxMaxFee`
(governance parameter, `keep-core` Sepolia value 1000 sats per
`RESERVATION_SEPOLIA_DEPLOYMENT.md` at `52f4f3005`).

Detection: `wallet_action_reservation_anchor_failed_total` /
`wallet_action_reservation_reanchor_failed_total`
(`keep-core:pkg/clientinfo/performance.go:823`);
`spv_reservation_action_overdue_total`
(`keep-core:pkg/clientinfo/performance.go:678`) as the pending
actions pass their windows. The specific client error strings are
"reservation acceptance estimated fee exceeds the maximum fee"
(`keep-core:pkg/tbtcpg/reservation_acceptance.go:1224`) and
"reservation re-anchor estimated fee exceeds the maximum fee"
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:1594`).

On-chain effect: the client fails before sending anything, so no
request is left stranded on chain: the new request path estimates
against the live `reservationTxMaxFee` and errors out before
`RequestReservationReanchor`
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:652`), and resumed
generations check against their own snapshotted bound
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:720`). Existing
pending generations that nobody signs time out and release capacity;
an already-broadcast transaction with a fee over the bound reverts at
settlement ("Transaction fee is too high",
`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:454`).
The re-anchor request floor `anchor > txMaxFee + minAmount`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:855`) blocks new
re-anchor requests when the anchor would grind below the minimum.

User impact: unspent funding remains refundable after locktime.
Already-confirmed spends are not protected by that refund; monitor proof deadlines and retirement timeouts.

Automatic mitigation: each window can re-estimate the fee.
If it fits the action's allowance, work can resume without a governance parameter change.

Manual mitigation: govern an increase of `reservationTxMaxFee`
(bad governance parameters topic does not cover this one: it is a
correct parameter, not a wrong write); mass timeouts from the spike
take the "Mass timeout" topic.

Status: holds.

### FM-14 Gas price spike

Trigger: Ethereum gas prices rise to where operators stop paying for
the reservation surface calls: `RequestReservationReanchor`,
`submitReservationProof`, timeout notifications, stranding, stale
clears.

Detection: failed submission attempts produce failure counters and logs.
The timeout watcher reports overdue actions before the late-proof cutoff.
An unmined transaction may need separate receipt/gas monitoring.
There is no demonstrated predictive gas-affordability alarm.
See VF-08 and the cited shared proof submitter.
D28's Critical late-proof alert detects a narrowing acceptance window
(`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:157-164`).
It does not predict gas affordability or guarantee mining before the cutoff.

On-chain effect: a request never mined creates no action.
A Pending generation retains its held capacity until a timeout notification succeeds.
Gas affordability can delay that notification and SPV proof submission.
If a TimedOut acceptance outlives `timeoutAt + termSeconds`,
its late proof is refused.

User impact: signatures, proofs and cleanup can be delayed.
An already-spent funding output cannot fall back to its depositor refund script.
Do not describe an expired, confirmed acceptance as a guaranteed no-loss outcome.

Automatic mitigation: existing retries and overdue signals depend on a healthy funded caller.
They do not guarantee that a transaction will mine.

Manual mitigation: operators raise their gas budgets or apply the
"Relay or SPV outage near the late-proof cutoff" topic to hand-submit
the proofs that matter before the cutoffs.

Status: holds-untested for the complete gas-affordability incident.
Failure and overdue detection exist; no end-to-end fee-spike rehearsal is cited.
