# Reservations testnet drill - runbook (DRAFT)

Status: draft, updated 2026-10-13. Deployment and operator inputs remain TBD. Do not start the drill until every TBD is filled.

The complete Sepolia test plan, with every spec ID mapped to a step, is in [sepolia-e2e/README.md](sepolia-e2e/README.md) (draft, 2026-10-21). This runbook stays the operator run sheet for D1-D12.

## 1. Inputs

| Item | Value |
|---|---|
| Client build | keep-core, the drill build (2026-10-20: the keeper tests, then the change that keeps proving redemptions past their timeout): the hand-off bindings (`1bed10dee`), the keeper gauges and the keeper wallet-discovery fix, plus the Sepolia bindings and activation block commit (TBD) |
| Contracts | the reservation contracts at the reconciled contracts commit (they will be in the public PR, plus the prepare-file guard and deploy script). Use this branch for prepare and activate. The older rehearsal builds do not contain the moved public PR head `c27b71281`; do not deploy them. |
| SDK acceptance request (prerequisite) | Code done on an SDK tooling branch (`5533415a3`) (plan T4.9) and included in the reconciled line @ `d30f99f65`. `ReservationsService.requestAcceptance(fundingTxHash, fundingOutputIndex, walletPublicKeyHash)` builds the SPV proof; the funding transaction needs the Bridge's required confirmations first. Use `d30f99f65` for the depositor tooling. Run on a mainnet fork against the reconciled contracts (fixture Bitcoin client, seeded test relay) with 43 pass / 0 fail / 0 skip; not yet run against a real Bitcoin client and relay, so the drill is the first such run. |
| Depositor tooling | tbtc-v2 SDK `ReservationsService` (branch in the row above). Run `checkReserveFeasibility` before funding and reveal before its `revealBy` time. `initiateReservedDeposit` takes the intended amount and picks a Live wallet with room for it. The SDK's ordinary 180-day refund locktime is rejected for reserved deposits on Sepolia (limit: now + term + 24 h = 91 days); the reservation module picks a locktime inside the on-chain window. Use a Bitcoin wallet that supports testnet4 and can sign a script-path refund. |
| Network | Sepolia, Bitcoin testnet4 (`config/_electrum_urls/testnet4`) |
| Activation block | `reservationsActivationBlocks[Sepolia]` = TBD |
| Operator set | TBD (number of operators, who runs them, contact channel) |
| SPV maintainer | TBD (address authorized with `isSpvMaintainer`) |
| Keeper and receiver | One independent unstaked maintainer plus the monitoring image from the same rehearsal checkout. Operator, metrics endpoint and approved Sentry receiver are TBD. Code and local host/image alert acceptance are verified. |
| Third-party hold cap | `maxThirdPartyReanchorHoldsPerWallet` = 1 (deploy default) |
| Reservation caps | Per-wallet and single amount caps 2,000,000 sats; max active 2; max total 4,000,000; hand-off reserve 2; ordinary count 1 per wallet; term 90 days. Each reserved deposit must be between the Bridge dust threshold (1,000,000 sats on Sepolia) and 2,000,000 sats. |
| Capacity limit | Before each fresh acceptance, require `count + 1 <= min(maxActive, max(0, Live count - reserve) * per-wallet count)`, plus ordinary target and amount headroom. Privileged hand-offs from retiring sources may exceed ordinary target caps. Accepted positions keep their global slot until a release such as stranding. Never raise max active or lower the reserve to reset the drill. |

### Keeper and monitoring setup

1. Start one independent unstaked `maintainer` with the client build in section 1.
2. Enable `Tbtc.ReservationsEnabled` or `Maintainer.Spv.ReservationProofsEnabled` so the keeper gauges are registered.
3. Enable the client metrics endpoint. Restrict access to the monitor and authorized operators.
4. Build the monitoring image from the rehearsal repository root. Use `docker build -f monitoring/Dockerfile -t tbtc-v2-monitoring:drill .`.
5. Set `ENVIRONMENT=testnet` and an approved `ETHEREUM_URL` for Sepolia. Set `ELECTRUM_URL` to a Bitcoin testnet4 server.
6. Set `RESERVATION_KEEPER_METRICS_URL` to the keeper metrics endpoint. The monitor does not register this check without it.
7. Set `SENTRY_DSN` from the approved receiver configuration. All four keeper alerts are warnings or critical events and go to Sentry. Discord receives informational events only.
8. Use a persistent `DATA_DIR_PATH` for checkpoints and receiver retry queues. Keep `RESERVATION_KEEPER_OBSERVATION_MAX_AGE_SECONDS` at 1800 unless the operators approve another positive value.
9. Before the drill, confirm that the observation timestamp is fresh and that the real receiver gets the four alert types.

The first local acceptance run used testnet4 Electrum responses, raised keep-core exposition gauges, live Sepolia RPC and a local Sentry sink. A second run used a real keeper and real contract state on a local mainnet fork (see `keeper-alert-acceptance.md`). Neither deployed the live keeper or alert receiver.


## 2. Activation order (C-4 gate)

Follow this order. A wrong order leaves reserved reveals that no client serves.

1. Deploy the contracts and run the governance actions, but **not** `setVaultStatus(vault, true)`.
2. Pick the activation block B, at least 1 day ahead.
3. Release the client build with `reservationsActivationBlocks[Sepolia] = B`.
4. Wait until every drill operator runs that build. Check with each operator's `/diagnostics` client version.
5. Only after block B, check the wiring first (AR-16, D32): read `reservationParameters()` through the Bridge and confirm that its first value, `reservationVault`, is the deployed ReservationVault. If it is not, stop: a trusted vault that is not wired can receive ordinary pooled credit through the reservation fee and mint path. Then execute `setVaultStatus(vault, true)`.
6. Record the block and tx of each step in the drill log.

## 3. Scenarios

The drill uses two long-lived positions, P1 and P2. D3, D4 and D5 reuse them. A successful re-anchor does not free a global position slot. Before another fresh acceptance, verify the count released by D2's timeout or a later stranding. A pending generation's replacement is not a reset for an accepted position.

Before each step, check the expected active count. Before a fresh acceptance, check the admission formula in section 1 using the current Live wallet count and reserve. Before retiring source wallets, identify at least one other wallet that will remain Live. A source-member hand-off may use that target even if its normal caps are full.

Two rules shape the order:
- An outside EOA (not the owner, an SPV maintainer, or a source member) needs a `MovingFunds` or `Closing` source and ordinary target headroom. A Live source rejects it. A privileged request from a retiring source may exceed target count and amount caps, including capacity held by another reservation's third-party action. The exception does not bypass the same-position pending-action rule or the non-governance cooldown.
- keep-core resumes and signs any pending re-anchor of its wallet, including one a third party requested (`pkg/tbtcpg/reservation_reanchor.go`, the ActionPending resume path; code read, not run). A third-party request on P1 therefore replaces the member request that D3 checks, so D3 runs first, on its own retirement.

| Order | ID | Scenario | How to force it | Count before -> after | Pass when |
|---|---|---|---|---|---|
| 0 | D9 | Stale reserved deposit (start first; runs in the background) | Reveal a reserved deposit and never request acceptance. Its refund deadline is at least the 15-day reveal-ahead period away | no change (a reveal holds no slot) | `notifyStaleReservedDeposit` is sent after the deadline and never before. |
| 1 | D2 | Acceptance timeout (Story 2) | Reveal a reserved deposit to a Live wallet, then stop that wallet's operators below quorum so the anchor is never signed | 0 -> 1 at request, 1 -> 0 at timeout | `notifyReservationAcceptanceTimedOut` is sent after `timeoutAt`. The count returns to 0 and `spv_reservation_action_overdue_total` rises. Restart the operators. |
| 2 | D1 (P1) | Acceptance happy path (Story 1) | Reveal a reserved deposit to Live wallet W1 | 0 -> 1 | `ReservationAcceptanceRequested`, then the anchor tx confirms on Bitcoin, then the SPV maintainer submits `submitReservationAcceptanceProof`. The reservation is Active and the vault mints. All within TBD hours. |
| 3 | D1 (P2) | Second acceptance | As D1, to Live wallet W2 | 1 -> 2 (full) | As D1. Any further acceptance request now reverts with "Active reservations cap exceeded". |
| 4 | D12 | Release to pool | Reveal a reserved deposit to W1 (full), see the acceptance request fail its preflight, then call `releaseReservedDepositToPool` | 2 -> 2 | The deposit is no longer reserved, the next deposit sweep includes it with the chosen vault, and the depositor receives ordinary TBTC. A release after an acceptance request reverts. |
| 5 | D3 + D5 | Re-anchor on rotation (Story 6), chained on P1 | Start moving funds on W1 only, with healthy operators. Send no outside requests | 2 -> 2 | The source wallet's members send `requestReservationReanchor` with a member proof (the tx input carries non-empty `walletMembersIDs`). The re-anchor tx confirms, the proof settles, P1 now sits on a free Live wallet Wx, and `pendingThirdPartyReanchorHolds(Wx)` stays 0. Acceptance and re-anchor have settled in successive windows on one reservation key (D5). Wy is the remaining free Live wallet. |
| 6 | D4 + D6 | Third-party hold cap and privileged evacuation, then moving-funds stranding | Start moving funds on Wx (holds P1, healthy operators) and W2 (holds P2, operators below quorum). Confirm both sources are retiring, both positions are Active, and cooldowns have expired. Before keep-core claims P1, an outside EOA requests P1 to Wy, then P2 to Wy. | 2 -> 2, then 2 -> 1 at stranding | D4: P1's request succeeds and Wy has one third-party hold. The outside P2 request rejects with "Target third-party re-anchor holds exceeded". The owner or a proven W2 member then requests P2 to Wy successfully, despite Wy's occupied ordinary slot. Wy's count exceeds its normal cap, the third-party hold remains one, and the global position count remains two. Wx's operators settle P1. W2 cannot sign its move; P2's re-anchor authorization times out and releases target capacity. D6: after W2 misses its moving-funds timeout, wait until P2 is Active with no pending action. `notifyReservationStranded` then strands P2 and reduces the count to one. If keep-core claims P1 first, record D4 as not run and repeat it on another eligible pair; do not claim the hold-cap observation. |
| 7 | D7 | Stranding, moved-funds-sweep timeout (Story 4) | Accept a fresh position P3 on a wallet chosen for the test, then let a moved-funds sweep request on it time out | 1 -> 2 -> 1 | As D6, for P3. |
| 8 | D8 | Stranding, fraud defeat (Story 5) | Accept a fresh position P4 on a test wallet, then submit and win a fraud challenge against it | 1 -> 2 -> 1 | As D6, for P4. |
| any | D10 | Watcher restart (L2 fix) | Make a reservation watcher panic or exit (fault injection on one node; method TBD) | no change | `spv_reservation_watcher_deaths_total` rises, a restart log line appears after about 10 s, and later timeouts are still sent. |
| with 6-8 | D11 | Stranding recheck (L15 fix) | During one of the stranding steps, make the wallet-closed handler's chain read fail once (RPC fault injection; method TBD) | as that step | The wallet is rechecked and stranded later, with no manual step. |

Retirements reduce the Live wallet count. Before another fresh acceptance, wait for enough Live wallets to satisfy the unchanged admission budget. Do not increase the cap. The stranding scenarios require a coordinated test-wallet setup; unrelated operators and wallets must remain available.

Story 5 (D8) and the fault-injection scenarios (D10, D11) need a test operator setup. If one is not possible on Sepolia, mark it "not run" with the reason. Do not mark it passed.

## 4. Signals to watch

| Signal | Source | Meaning |
|---|---|---|
| `active_reservations_count`, `max_active_reservations`, `wallet_reservations_count`, `live_wallets_count` | client metrics | capacity use |
| `spv_reservation_action_overdue_total` | SPV maintainer | an action passed its deadline without settlement |
| `spv_reservation_watcher_deaths_total` | SPV maintainer | a watcher crashed (it should restart) |
| `spv_proof_task_failures_total`, `spv_proof_skipped_*` | SPV maintainer | proof submission problems |
| `performance_reservation_vault_fee_debt_sat`, `performance_reservation_vault_fee_reserve_tbtc_base_units` | keeper metrics | vault fee state; positive debt emits a warning |
| `pendingThirdPartyReanchorHolds(wallet)` | Bridge view | third-party re-anchor holds per target |
| "recovered from panic in ... the node keeps running" log lines | client logs | the node recovered a goroutine panic (T2 fix); each one needs triage |
| Monitor-only leads G1-L21, L24, L26, L27 | logs, chain | watch for the patterns in `findings/rerun/README.md`; record any sighting |
| `performance_reservation_acceptance_late_proof_at_risk_count` | keeper metrics | positive count emits a critical late-proof alert |
| `performance_reservation_reanchor_no_eligible_target_count` | keeper metrics | positive count emits a critical no-target alert |
| `performance_reservation_unrequested_deposit_deadline_at_risk_count` | keeper metrics | positive count emits an acceptance-request deadline warning |
| `performance_reservation_keeper_observation_timestamp` | keeper metrics | missing, invalid or stale observations emit a critical keeper-health alert |

## 5. Residual risks to observe, not to fix during the drill

- **R1 - concentration:** privileged hand-offs may place more anchors and satoshi on a target than its normal admission caps allow. Record target counts and amounts before and after each move. Third parties retain ordinary caps and the hold limit.
- **R2 - same-position pending action:** a third-party authorization on a reservation can still delay that reservation until settlement or timeout. Only the governance-discriminated request bypasses a subsequent cooldown. Capacity bypass does not remove these state rules.

## 6. Exit criteria

- D1, D2, D3, D5 and D6 pass.
- D4 and D9 pass.
- D7, D8, D10 and D11 pass, or are marked "not run" with the reason.
- No "recovered from panic" log line, and no watcher death without a restart, is left without triage.
- The drill log (block numbers, tx hashes, metric snapshots) is stored next to this runbook.
