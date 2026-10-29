# M1 UTXO Reservations: Failure Modes (FM)

Scope: M1 reservations in both pinned repos. Contracts at
`tbtc-v2` @ `9d2086965` (cited as `tbtc-v2:solidity/<path>:<line>`);
client at `keep-core` @ `f99ef96e3` (cited as
`keep-core:<path>:<line>`). D31 (privileged hand-off plus acceptance
reserve) is integrated on the reservations branches: contracts at
`tbtc-v2@806b1106f`, testnet client at `keep-core@d40f6aadc`. Those facts
carry an explicit revision in their citation; an unqualified citation means
the original pin. Nothing is deployed on Sepolia or mainnet.
Status vocabulary: `holds` (code enforces
the described behaviour and a named test pins it), `holds-untested`
(code enforces it, no test), `design-only` (planned, not in code),
`unknown` (missing to decide).

Manual mitigations point to disaster-recovery topics by name in
`verification/07-disaster-recovery.md`, not by ID.

Every detection claim names an existing metric, counter, gauge, or
on-chain event; where none exists it says none.

## Summary

| ID | Failure mode | Primary detection | On-chain effect | Auto mitigation | Status |
|---|---|---|---|---|---|
| FM-01 | SPV maintainer down | Process/scrape unavailable or stale success timestamp | Proofs and explicit timeout cleanup delayed | Healthy redundant maintainers and restart | holds |
| FM-02 | SPV relay stale | `spv_proof_skipped_outside_relay_range_total` | proofs not provable until relay recovers; late windows close unproven | candidate walk skips, retries next pass | holds |
| FM-03 | keep-core node bug or crash | `spv_reservation_watcher_deaths_total`; `spv_maintainer_last_failure_timestamp_seconds` | missed proposals and proofs; actions time out | watcher restart backoff; other operators pick up windows | holds |
| FM-04 | watcher death | `spv_reservation_watcher_deaths_total` | stranding, stale-deposit, timeout notifications delayed | wiring restart loop with backoff | holds |
| FM-05 | coordination window missed | `coordination_windows_detected_total` up with `wallet_action_reservation_*` flat; `ReservationReanchorTimedOut` | pending requests age out; target capacity released; cooldown set | next window retries; unmined request resumed by receipt | holds |
| FM-06 | wallet below quorum | `wallet_action_reservation_anchor_failed_total` / `..._reanchor_failed_total`; `coordination_window_wallets_failed` | action times out; for a MovingFunds wallet, termination and slashing | none on chain; stranding cleanup after termination | holds |
| FM-07 | fee reserve empty | `reservation_vault_fee_debt_sat`; `InKindFeeFinanced` event with non-zero shortfall | settlement still succeeds; system over-supplied by the debt | shortfall debt recorded and publicly repayable | holds |
| FM-08 | saturation, unproven funds | `wallet_reservations_count` at cap; `reanchor_waiting_on_aged_target` | `MovingFundsTimedOut`; members slashed; wallet `WalletTerminated`; reservations `ReservationStranded`. At `806b1106f` a privileged hand-off skips the target caps, so full targets alone no longer cause this; no Live target, no quorum or no proof still do | below-dust and strand watchers retry; the member-proof fallback picks the least-loaded Live wallet | holds |
| FM-09 | Saturation after main funds are proven | `WalletClosing` with non-zero reservation count | source enters Closing; final Closed transition waits for zero count | re-anchor from Closing; eligible stranding; explicit finalization | holds-untested |
| FM-10 | Bad governance parameters | Feasibility gauge plus live parameter/event inspection | Acceptance proof uses snapshots; at `9d2086965` re-anchor signing re-checked live target caps, at `806b1106f` it does not (the validator checks only that the target is Live) | Write-time guards reject specific invalid values; the five-argument caps setter rejects a zero hand-off reserve | holds |
| FM-11 | activation order wrong (C-4) | none | `pendingReservedDeposits` grows unaccepted; vault change frozen | none | design-only |
| FM-12 | RPC failures | `spv_maintainer_max_backoff_seconds`; logs; `spv_maintainer_last_failure_timestamp_seconds` | delayed proposals/proofs; actions time out | chunk-scan cursor retry; fail-closed gates | holds |
| FM-13 | Bitcoin fee above txMaxFee | Action-failure and overdue signals | Fresh requests may be withheld; mined Pending actions need explicit cleanup | Re-estimate within the snapshotted allowance | holds |
| FM-14 | Ethereum gas price spike | Submission failures and overdue signals; no predictive affordability alarm | Requests, proofs and timeout cleanup can all be delayed | Retries require a healthy funded caller | holds-untested |

## Full entries

- [Service, signing and reserve failures](06-service-signing-failure-modes.md): FM-01 through FM-07.
- [Lifecycle, governance, RPC and fee-market failures](06-lifecycle-operations-failure-modes.md): FM-08 through FM-14.


## Findings

The earlier gas-detection claim is a candidate, not a confirmed defect.
Generic maintainer failure timestamps and proof-task failure counters can
detect submission failures before a late-proof cutoff.
A deadline by itself does not emit `ReservationAcceptanceTimedOut`.
That event requires a successful notification transaction.
See the adjudication register for the final verdict.

## Doc drift

| Doc location | What it says | What the code does |
|---|---|---|
| `m1-b-implementation.md:403` | gauges named `reservation_vault_fee_reserve_tbtc` | registered name is `reservation_vault_fee_reserve_tbtc_base_units` (`keep-core:pkg/clientinfo/performance.go:786`) |
| `m1-guide.md:253` (audit table) | lists `spv_reservation_watcher_deaths_total` as "A watcher died" without noting it counts restarts too | the counter increments once per death, i.e. per restart event, not per distinct crash (`keep-core:pkg/maintainer/spv/reservation_wiring.go:203`) |
