# Service, signing and fee-reserve failure modes

Evidence pins and vocabulary: [failure modes index](06-failure-modes.md).

### FM-01 SPV maintainer down

Trigger: the SPV maintainer process (or every operator running the
proof loop) is down. Anchor and re-anchor transactions the
coordination nodes broadcast still confirm on Bitcoin, but no
`submitReservationProof` calls happen.

Detection: process/scrape unavailability, stale success timestamps, or an inactive
`spv_maintainer_active` gauge when the exporter remains available
(`keep-core:pkg/clientinfo/performance.go:680`).
`spv_proof_task_failures_total`
(`keep-core:pkg/clientinfo/performance.go:664`). On chain,
`ReservationAcceptanceTimedOut`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:384`) and
`ReservationReanchorTimedOut`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:379`) appear only
after a caller submits the timeout notification. Passing a deadline emits no event by itself.

At the D28 pins, the independent keeper counts unproven acceptance generations near their late-proof edge
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:85-106`).
The monitor sends a Critical risk event and detects unavailable or stale keeper observations
(`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:157-164`,
`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:192-225`).
This is detection, not proof delivery or authorization of a replacement maintainer.

On-chain effect: an eligible caller can notify acceptance timeout and release capacity.
The notified generation remains settleable within its late window
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:204`).
Re-anchor timeout notification releases target capacity and sets cooldown
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:984`).
No deadline releases capacity without that transaction.
For a MovingFunds source wallet, unproven moving funds is the path to
termination and slashing (FM-08).

User impact: reservation owners see no state change until proofs
arrive; minted amounts are credited only at settlement, so a long
outage delays credits and the recorded custody update. If the funding
output is still unspent, the depositor can use its Bitcoin refund script
after the refund locktime. If an acceptance transaction already spent
that output, the refund path is no longer available.

Automatic mitigation: the proof loop idle-retries after each pass
(`keep-core:pkg/maintainer/spv/reservation_proof_loop.go:436`); a
crashed loop restarts via the maintainer's restart loop with backoff.
Other operators' maintainers continue.

Manual mitigation: restart the maintainer; if the outage reaches the
late-acceptance cutoff, apply the "Relay or SPV outage near the
late-proof cutoff" topic and manually submit outstanding proofs with
direct SPV submissions. If actions mass-time out, apply the "Mass
timeout" topic.

Status: holds.

### FM-02 Relay stale

Trigger: the SPV relay's difficulty-epoch view lags the real chain
(for example a long reorg or a stale relay node), so assembled proofs
fall outside the relay's accepted range, or no decisive header exists
within `maxProofHeaders`.

Detection: `spv_proof_skipped_outside_relay_range_total` and
`spv_proof_skipped_exceeded_max_headers_total` counters
(`keep-core:pkg/clientinfo/performance.go:690`, `:694`), incremented
from the reservation proof path
(`keep-core:pkg/maintainer/spv/reservation_proof_loop.go:1418`,
`:1434`). The "exceeded max headers" case is logged as possibly
permanent (`keep-core:pkg/maintainer/spv/reservation_proof_loop.go:1428`).

On-chain effect: no proof transaction reaches the Bridge.
Timeout cleanup requires an explicit notification transaction.
For a notified `TimedOut` acceptance, settlement rejects after
`timeoutAt + termSeconds`.
A still-`Pending` acceptance uses a different proof-state rule.
The late cutoff is checked only when the action state is `TimedOut`
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:192-210`).

User impact depends on Bitcoin state.
If the funding output is unspent, the depositor can refund after its locktime.
If an anchor transaction spent it, that refund is impossible.
A notified acceptance whose late window expires can leave confirmed custody
uncredited. M1 has no ordinary recovery call for that expired generation.
A missing re-anchor proof also leaves recorded custody on the source wallet,
even when Bitcoin custody has already moved.

Automatic mitigation: the candidate walk skips unprovable candidates
and retries on later passes (`keep-core:pkg/maintainer/spv/reservation_proof_loop.go:1362`);
nothing auto-cures a stale relay.

Manual mitigation: point the relay at a healthy node, or apply the
"Relay or SPV outage near the late-proof cutoff" topic.
Restore an accepted relay difficulty range and an authorized maintainer.
Manual submission must still satisfy the Bridge's SPV checks.

Status: holds.

### FM-03 keep-core node bug or crash

Trigger: a node process crashes or a bug makes a node misbehave
(wrong proposal, wrong target, or repeated failures).

Detection: `spv_maintainer_last_failure_timestamp_seconds` and
`spv_maintainer_active` (`keep-core:pkg/clientinfo/performance.go:680`);
`spv_reservation_watcher_deaths_total` when a watcher goroutine dies
(`keep-core:pkg/clientinfo/performance.go:671`);
`wallet_action_reservation_*_failed_total` counters for the
reservation action types, registered dynamically via
`WalletActionMetricName` (`keep-core:pkg/clientinfo/performance.go:823`).
On chain, a bad proposal simply reverts in the on-chain validator;
the `ReservationReanchorTimedOut` event marks a request that nobody
completed.

On-chain effect: a single node failing is invisible: the other
signers and the other operators' leaders continue. A wrong proposal
reverts in `validateReservationAnchorProposal` /
`validateReservationReanchorProposal`
(`tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:963`,
`:1108`) without state change. The system-visible effect is a missed
or late action, not a broken state.

User impact: delayed anchor credits and re-anchors while one or more
operators are down; no loss if redundancy exists.

Automatic mitigation: the coordinator elects a leader per window
from the remaining healthy nodes; watcher loops self-restart
(`keep-core:pkg/maintainer/spv/reservation_wiring.go:520`).

Manual mitigation: redeploy the node; for a release that changed the
activation block wrongly, apply the "keep-core release with the wrong
activation block" topic.

Status: holds.

### FM-04 Watcher death

Trigger: a reservation watcher (stranding, stale-deposit, or
action-timeout) exits with an error or a panic.

Detection: `spv_reservation_watcher_deaths_total` is incremented once
per death by the wiring, for panics and for Run errors
(`keep-core:pkg/maintainer/spv/reservation_wiring.go:203`, `:523`);
the ERROR log line names the watcher.

On-chain effect: the permissionless notifications each watcher
submits stop coming from that operator. Concretely: timed-out
actions are not reported, so capacity stays reserved and the
re-anchor cooldown is not set until someone else reports; stale
deposits are not cleared, so `pendingReservedDeposits` blocks vault
re-pointing longer; stranded reservations are not reported, so a
Terminated wallet's positions retain their accounting until eligible cleanup.
A Closing wallet can also retain a blocked finalization while its count is non-zero.
Each operator staggers its first attempt by a deterministic offset,
so the fleet as a whole usually still reports
(`keep-core:pkg/maintainer/spv/reservation_wiring.go:158`).

User impact: delays only, plus a longer window in which the
re-pointing-vault governance action is blocked by pending entries.

Automatic mitigation: the wiring restarts every dead watcher with
backoff (`keep-core:pkg/maintainer/spv/reservation_wiring.go:520`).

Manual mitigation: restart the process; if a whole fleet of
watchers is down, submit the overdue notifications manually; apply
the "Mass timeout" topic for the capacity effects.

Status: holds.

### FM-05 Coordination window missed

Trigger: the window's leader fails, or the leader's checklist finds
no eligible work, or the action task's candidate budget (50 per run,
`keep-core:pkg/tbtcpg/reservation_acceptance.go:120`) defers work.

Detection: `coordination_windows_detected_total` keeps advancing while
`wallet_action_reservation_anchor_total` /
`wallet_action_reservation_reanchor_total` stay flat
(`keep-core:pkg/clientinfo/performance.go:711`, `:823`). On chain, the
result of the miss is visible as `ReservationReanchorTimedOut`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:379`) after the
action window elapses.

On-chain effect: a request never mined creates no action on the canonical chain.
A mined Pending action can pass its signing deadline without completing.
An explicit timeout notification releases its capacity and permits a later request.
The signing validator rejects after its safety-margin cutoff
(`tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:998`).

User impact: the anchor or re-anchor is delayed by one or more
windows (900 blocks each); for a MovingFunds wallet, repeated misses
are the entry path to FM-08.

Automatic mitigation: the next window's leader re-scans; an unmined
request from the previous round is resumed by receipt instead of
re-requested (`keep-core`
`TestReservationReanchorTask_UnminedRequest_MinedBeforeNextRound_Resumes`,
`pkg/tbtcpg/reservation_reanchor_inflight_test.go:214`).

Manual mitigation: none needed routinely; if misses persist across
the fleet, apply the "Mass timeout" topic.

Status: holds.

### FM-06 Wallet below quorum

Trigger: the signing group of a custodying or source wallet cannot
reach the signing threshold (member keys offline, lost, or a
faulty group). The reservation proposal for that wallet cannot be
signed.

Detection: `wallet_action_reservation_anchor_failed_total` and
`wallet_action_reservation_reanchor_failed_total` for that wallet
(`keep-core:pkg/clientinfo/performance.go:823`);
`coordination_window_wallets_failed`
(`keep-core:pkg/clientinfo/performance.go:727`). On chain, the
action times out: `ReservationReanchorTimedOut` /
`ReservationAcceptanceTimedOut`.

On-chain effect: for an acceptance, the deposit is unanchored and the
depositor keeps the refund right; capacity is released at timeout.
For a `MovingFunds` source that never proves its main-funds transfer,
an explicit `notifyMovingFundsTimeout` can slash and terminate the wallet
(`tbtc-v2:solidity/contracts/bridge/MovingFunds.sol:567`,
`tbtc-v2:solidity/contracts/bridge/Wallets.sol:485`).
A `Closing` wallet does not face that MovingFunds timeout.
It can re-anchor, or strand after `dissolutionEligibleAt`.
Neither timeout nor stranding occurs without a notification transaction.

User impact depends on the wallet's state and recovery timing.
A missing quorum delays a request; it does not strand every position immediately.
MovingFunds timeout can slash and terminate. Closing can be strand-eligible later.
Stranding leaves outstanding TBTC balances unchanged.

Automatic mitigation: none: a byzantine or offline quorum is the
pre-existing tBTC failure the stranding path exists to absorb. The
stranding watcher submits `notifyReservationStranded` once the
wallet is Terminated or Closed
(`keep-core:pkg/maintainer/spv/reservation_stranding_watch.go:290`).

Manual mitigation: apply the "Stranded positions" topic; custodian
key recovery and DKG reconstitution are pre-existing tBTC
machinery, out of M1 reservation scope.

Status: holds.

### FM-07 Fee reserve empty

Trigger: in-kind miner fees from re-anchors outpace the vault's
fee-reserve balance (initiation-fee revenue).

Detection: `reservation_vault_fee_debt_sat` gauge
(`keep-core:pkg/clientinfo/performance.go:785`), chain-read from the
vault's `inKindFeeDebtSat`
(`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:85`); the
`InKindFeeFinanced` event carries the shortfall
(`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:233`);
`InKindFeeDebtRepaid` on repayment
(`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:265`).
At the D28 pins, a complete keeper pass reads that debt even without a signing-client proposal
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:170-187`).
Any positive value produces a Warning through the existing monitoring service
(`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:173-180`).
The local fork recorded genuine re-anchor debt, receiver delivery and a quiet repayment control
(`keeper-alert-acceptance.md:6-49`).

On-chain effect: none to settlement: `financeInKindFee` records the
shortfall as debt and succeeds, so a confirmed Bitcoin re-anchor
never fails to settle (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:209`).
While the debt is non-zero the system is over-supplied by exactly
that amount of TBTC. `sweepFees` repays the debt from the full vault
balance before comparing against the reserve target.

User impact: none directly; the debt is a protocol-internal
over-supply, publicly visible and repayable by anyone via
`repayInKindFeeDebt`.

Automatic mitigation: the debt path avoids a reserve-shortfall settlement revert.
New fee revenue does not automatically reduce existing debt.
An owner `sweepFees` call repays from the reserve, or a caller explicitly repays debt.

Manual mitigation: apply the "Fee debt growth" topic: govern a
repayment or a fee/term parameter change to stop the outpace.

Status: holds.
