# Sepolia e2e test plan - step catalog, part 2 (sections E-K)

Actor legend, Sepolia sizing and the capacity facts are in [part 1](steps-1.md).

## E. Stranding

### S-50 Stranding after a moving-funds timeout
- Actor: `anyone`, then spv.
- How: the source wallet misses its moving-funds timeout and terminates; a late re-anchor or
  acceptance proof arrives afterwards, or `notifyReservationStranded(key)` is called.
- Expected: `ReservationStranded`, the position becomes `Stranded`, the active count drops by one,
  the anchor mapping clears.
- Pass: count 2 -> 1 and a second strand reverts `Reservation is not active`.
- Covers: US-13, US-15, US-49, SM-R8, SM-W4, SM-W5, INV-103, INV-104, INV-201, EC-11, TM-07, DR-02,
  S-82.
- Slot: **releases one**.
- Fits: D6 (order 6).

### S-51 Stranding after a moved-funds-sweep timeout
- Actor: `anyone`.
- How: as S-50, driven by a moved-funds-sweep request that times out.
- Pass: same transition, count 1 -> 2 -> 1 for the fresh position.
- Covers: US-13, US-49, INV-103, SM-W4, SM-W5, PAR-20, DR-02.
- Slot: **releases one**.
- Fits: D7 (order 7).

### S-52 Stranding after a fraud challenge
- Actor: thirdParty (challenger).
- How: submit and win a fraud proof against a test wallet that custodies an accepted position.
- Pass: the position strands and the count drops.
- Covers: US-13, US-49, INV-103, SM-W4, SM-W5, TM-08, DR-02.
- Slot: **releases one**.
- Fits: D8 (order 8). Requires a coordinated test-wallet setup; otherwise record "not run".

### S-53 Stale reserved deposit, permissionless
- Actor: `anyone`.
- How: a reserved deposit revealed and never requested; call `notifyStaleReservedDeposit(depositKey)`
  before the deadline (reverts `Deposit refund deadline has not elapsed`) and after
  (`ReservedDepositMarkedStale`), then with an acceptance pending (reverts
  `Acceptance authorization pending`).
- Pass: the notify is only accepted at or after the snapshotted deadline and exactly once.
- Covers: US-10, US-48, SM-D3, SM-D4, SM-D5, INV-108, INV-109, INV-206, EC-10, TM-12, PAR-K1,
  XR-F-15, PAR-K1.
- Slot: none.
- Fits: D9 (order 0), in the background.

### S-54 Governance force-stale
- Actor: governance.
- How: `forceStaleReservedDeposit(depositKey)` on a pending reserved deposit whose deadline is far
  away; then `updateReservationParameters` succeeds.
- Pass: the counter drops without waiting for the deadline.
- Covers: US-65, US-10, INV-109, INV-206, TM-12, PAR-1, INV-105.
- Slot: none. Wall time: 1 h.

### S-55 Stranding on a Closing wallet after dissolution and the closing period
- Actor: `anyone`.
- How: on a wallet in `Closing` past `dissolutionEligibleAt` and past `closingStartedAt +
  walletClosingPeriod`, call `notifyReservationStranded`; an earlier attempt reverts
  `Wallet is not terminated, closed, or a dissolution-eligible closing wallet`.
- Pass: the transition happens only after both conditions and exactly once.
- Covers: US-14, US-15, INV-103, INV-104, SM-W5, PAR-4, PAR-20, EC-11, DR-02, FM-09.
- Slot: **releases one**. Wall time: 24 h dissolution delay plus the closing period.
- Fits: alongside D6.

---

## F. Vault fees and fee debt

### S-61 Vault credit decomposition at acceptance
- Actor: keeper, reading the vault and Bank.
- How: after S-22, read the Bank mint, the vault TBTC balance, `initiationFeeBps`, the depositor's
  Bank balance, and the `ReservationCreditProcessed` event.
- Pass: the mint is gross; the vault retains the fee; the depositor receives the net; no residual
  balance sits with the vault or the depositor.
- Covers: US-11, US-80, US-81, INV-209, INV-701, INV-704, PAR-X7, SM-C5, SM-C7, XR-M-10, TM-11.
- Slot: none. Wall time: 15 min.
- Fits: right after each D1 acceptance.

### S-62 In-kind fee debt from a re-anchor with a zero reserve
- Actor: the Bridge on behalf of spv; `anyone` for repayment.
- How: settle a re-anchor (S-37) while `feeReserveTarget == 0`; read `inKindFeeDebtSat` and
  `performance_reservation_vault_fee_debt_sat`; then call `repayInKindFeeDebt(amountSat)` from an
  outside address with exactly the outstanding debt, then with an over-payment (capped) and with
  zero (reverts `Amount must not be zero`), and with no debt outstanding (reverts
  `No debt to repay`).
- Pass: debt equals the re-anchor miner fee, the repayment burns exactly that much TBTC, an
  over-payment is capped, and the zero/no-debt calls revert.
- Covers: US-69, US-82, INV-207, INV-705, DR-09, PAR-X7, AR-12, SM-C5, XR-M-10, XR-M-6, TM-19.
- Slot: none. Wall time: 30 min.
- Fits: after S-37.

### S-63 sweepFees to the treasury
- Actor: vaultOwner.
- How: after fees accumulate, `sweepFees(treasury)`; then a call with the zero address (reverts
  `Recipient must not be zero`), from a non-owner (reverts), and a sweep when the balance is at or
  below the reserve (no-op).
- Pass: the excess leaves the vault, the reserve target remains, the debt is repaid first.
- Covers: US-81, US-68, INV-207, PAR-X7, DR-12, AR-03.
- Slot: none. Wall time: 30 min.

### S-64 Fee-reserve target and initiation fee governance
- Actor: vaultOwner and a non-owner.
- How: `updateFeeReserveTarget(x)` from the owner and from a non-owner (revert);
  `updateInitiationFee(bps)` from the owner, above `MAX_FEE_BASIS_POINTS` (reverts
  `Fee exceeds the maximum`), and at exactly the maximum.
- Pass: only the owner writes; the bound holds.
- Covers: US-68, PAR-X7, SM-A6, AR-03, AR-12, INV-207.
- Slot: none. Wall time: 30 min.

### S-65 Vault surface negatives
- Actor: `anyone`.
- How: `receiveBalanceIncrease` from a non-bank address (reverts `Caller is not the Bank`),
  `receiveBalanceApproval` (always reverts `Balance approvals not supported`),
  `financeInKindFee` from a non-bridge address (reverts `Caller is not the Bridge`).
- Pass: all three revert with the exact strings.
- Covers: US-83, US-69, US-80, INV-207, TM-11, SM-C5, AR-16.
- Slot: none. Wall time: 30 min.

---

## G. Keeper alerts (each alert actually firing)

Prerequisite for S-71..S-76: the keeper runs with the reservation gauges registered, the metrics
endpoint reachable by the monitor, `RESERVATION_KEEPER_OBSERVATION_MAX_AGE_SECONDS = 1800`, and an
approved receiver. Each step records the gauge value and the received event.

### S-71 Healthy keeper, no events
- Pass: clean metrics produce no events; all four alert counters stay 0.
- Covers: US-100, PAR-K2, XR-M-8, XR-M-9, XR-M-10, XR-M-11, XR-M-12, EC-24, INV-115.
- Slot: none. Wall time: 15 min at any point.

### S-72 Late-proof-at-risk alert
- Actor: operator (do not settle a timed-out acceptance).
- How: leave an acceptance generation timed out and unsettled; read
  `performance_reservation_acceptance_late_proof_at_risk_count`.
- Pass: the count rises above 0 within the risk window and the receiver logs a critical event.
- Covers: US-47, US-48, PAR-K1, PAR-K2, US-10, US-06, XR-M-8, SM-C16, EC-24.
- Slot: none (read only). Wall time: 24 h + up to 7 days of window.
- Fits: from D2 onwards.

### S-73 No-eligible-re-anchor-target alert
- How: retire wallets until every remaining Live target is at its caps while a MovingFunds wallet
  still custodies a position.
- Pass: `performance_reservation_reanchor_no_eligible_target_count > 0` and a critical event.
- Covers: US-27, US-49, US-51, SM-C16, XR-M-7, XR-M-9, PAR-X4, INV-113, DR-01, FM-08.
- Slot: none (read only). Wall time: 30 min once saturated.
- Fits: at the end of D6 or D8.

### S-74 Vault fee debt warning
- How: read `performance_reservation_vault_fee_debt_sat` after S-62.
- Pass: a positive debt produces the warning event.
- Covers: INV-207, INV-705, DR-09, AR-12, PAR-X7, XR-M-10, XR-M-6, US-69.
- Slot: none. Wall time: 10 min after S-62.

### S-75 Unrequested-deposit deadline warning
- How: leave a reserved reveal unrequested until it is near its one-day request deadline.
- Pass: `performance_reservation_unrequested_deposit_deadline_at_risk_count > 0` and a warning
  event; it clears after the request or the stale notify.
- Covers: US-10, US-48, PAR-K1, XR-M-11, SM-C16, SM-D5.
- Slot: none. Wall time: about 24 h.
- Fits: from D9 onwards.

### S-76 Stale keeper-observation alert
- How: stop the keeper for more than `RESERVATION_KEEPER_OBSERVATION_MAX_AGE_SECONDS` (30 min).
- Pass: `performance_reservation_keeper_observation_timestamp` goes stale and a critical event
  fires; it clears when the keeper resumes.
- Covers: PAR-K2, EC-24, US-51, US-100, SM-C16, XR-M-12, INV-115, EC-23.
- Slot: none. Wall time: 35 min.

---

## H. Client restart and fault injection

### S-81 Watcher panic and restart
- Actor: keeper operator.
- How: inject a panic or exit into one reservation watcher on one node.
- Expected: `spv_reservation_watcher_deaths_total` rises, a "recovered from panic" log line
  appears within about 10 s, and later timeouts are still sent.
- Pass: no silent watcher loss; no unreconciled "recovered from panic" line at the end.
- Covers: FM-03, FM-04, US-47, US-48, US-49, US-51, US-100, XR-M-5, SM-C16, SM-A8.
- Slot: none. Wall time: 30 min. This is D10.

### S-82 Chain-read failure in the wallet-closed handler
- Actor: keeper operator (RPC fault injection).
- How: make the wallet-closed handler's chain read fail once during a stranding step.
- Pass: the wallet is rechecked and stranded later with no manual step.
- Covers: FM-01, FM-02, FM-12, US-13, US-49, US-51, TM-13, SM-C16, EC-22.
- Slot: none. Wall time: 1 h. This is D11.

### S-83 Keeper restart mid-proof-loop
- Actor: keeper operator.
- How: restart the node while a proof loop is scanning for a pending generation.
- Pass: the loop resumes its scan and eventually settles; no generation is skipped and none is
  submitted twice.
- Covers: US-46, US-40, US-41, US-51, FM-03, FM-05, SM-C16, TM-12, TM-13, EC-12.
- Slot: none. Wall time: 30 min.
- Fits: during D1 or D3, while a generation is pending.

---

## I. Negative and revert cases collected on chain

### S-91 Acceptance request guard set
As listed in S-17, run in one pass and recorded as a table of (string, tx or `eth_call`, block).
Covers: US-04, US-05, EC-06..EC-10, EC-18, INV-101, INV-108, INV-203, TM-03, TM-12, PAR-X4,
SM-R2, XR-P-2. Slot: none. Wall time: 4 h spread across the drill.

### S-92 Re-anchor request guard set
`Reservation is not active`, `Reanchor cooldown in effect`,
`Caller is not a member of the source wallet`, `Only governance can rotate a Live wallet's
anchor`, `Source wallet must be in MovingFunds or Closing state`,
`Target wallet must differ from the source wallet`, `Target wallet must be in Live state`,
`Reanchor authorization has no signing window`, `Reanchor would fall below the minimum reservation
amount`, `Target third-party re-anchor holds exceeded`, `Wallet reservations cap exceeded`,
`Wallet reserved amount cap exceeded`, `Reservation is not in ActionPending state`,
`Unsupported action type for timeout`, `Action is not pending`, `Action has not timed out`.
Covers: US-04, US-05, US-32, US-70..US-74, EC-02, EC-09, EC-19, INV-401, INV-402, INV-503,
INV-601, INV-602, INV-112, PAR-F2, PAR-X1, PAR-X8, SM-A3, SM-A7, SM-X3, TM-02, TM-03, TM-06,
TM-16, AR-06, AR-14, AR-15, XR-P-5, XR-P-6, XR-F-1, XR-F-20, XR-R-1, XR-R-2.
Slot: none. Wall time: 6 h spread across the drill.

### S-93 Proof submission guard set
`Caller is not SPV maintainer`, `Unsupported reservation proof type`, `Action type mismatch`,
`Action is not settleable`, `Reservation is not settleable`, `Late acceptance settlement window
expired`, `Not the current generation`, `Reservation anchor already spent`,
`Action source anchor is no longer current`, `Transaction input must spend the reserved deposit`,
`Transaction input must point to the reservation anchor`,
`Reservation transaction must have a single output`, `Anchor output must pay the authorized
wallet`, `Output must pay the authorized target wallet`, `Transaction fee is too high`,
`Anchor below reservation minimum`, `Re-anchor amount below the dust floor`,
`No pending action to unwind`.
Covers: US-42, US-43, US-44, US-45, EC-13, EC-14, EC-15, INV-301, INV-304, INV-502, INV-503,
SM-X6, TM-04, TM-05, XR-E-1, XR-E-2, XR-F-11, XR-F-12, XR-P-1, XR-G-3.
Slot: none. Wall time: 8 h spread across the drill.

### S-94 Deposit sweep refuses reserved deposits
- Actor: operator proposing an ordinary sweep.
- How: assemble a deposit-sweep proposal that includes a reserved deposit and validate it.
- Expected: `Reserved deposits must not be swept`; and a reservation-parameter read error fails
  the proposal closed rather than defaulting.
- Pass: neither a reserved deposit nor an unreadable reservation parameter can enter a sweep.
- Covers: US-29, INV-109, TM-12, PAR-1, SM-D1, SM-D2, EC-01.
- Slot: none. Wall time: 1 h.

### S-95 Reveal-time guard set
`Refund locktime too far in the future for a reservation`, `Vault is not trusted`,
`Wallet must be in Live state`, the ordinary `Deposit refund locktime is too close`.
Covers: US-02, EC-06, PAR-X6, TM-01, INV-603, XR-P-4, SM-W1. Slot: none. Wall time: 1 h.

### S-96 Notify guard set
`Not a pending reserved deposit`, `Deposit already swept`, `Deposit refund deadline has not
elapsed`, `Acceptance authorization pending`, `Reservation is not active`,
`Wallet is not terminated, closed, or a dissolution-eligible closing wallet`,
`Acceptance was already requested`.
Covers: US-10, US-13, US-15, EC-03, EC-11, INV-103, INV-104, SM-W5, SM-X3, TM-12, TM-07, AR-01.
Slot: none. Wall time: 2 h spread across the stranding steps.

### S-97 Governance ceremony guard set
`Governance delay has not elapsed`, `Change not initiated`, `Pending reserved deposits exist`,
`Active reservations exist`, `Amount cap exceeds slot capacity`,
`Active reservations cap must be greater than zero`, `Hand-off reserve must be greater than zero`,
`Max active reservations must be greater than zero`,
`Max reservations per wallet must be greater than zero`,
`Reservation minimum amount must be greater than the reservation TX max fee`,
`Reservation term out of protocol bounds`, `Renewal window must be shorter than the term`,
`Reservation action timeout leaves no signing window`,
`Reservation transaction max fee must be greater than zero`, and the write-once router revert.
Covers: PAR-2, PAR-3, PAR-5, PAR-7, PAR-8, PAR-13, PAR-F1, PAR-X1..PAR-X5, US-60, US-61, US-63,
US-64, US-66, DR-03, DR-05, FM-10, FM-11, TM-14, AR-14, INV-109, INV-112, EC-08, EC-09, EC-10.
Slot: none. Wall time: 4 h.

### S-98 Bridge and router surface negatives
- Actor: `anyone`.
- How: call the router's re-anchor entry with an unknown key (reverts `Reservation is not active`);
  submit a proof from a non-maintainer (`Caller is not SPV maintainer`); read
  `reservationRouter()` and confirm the fallback path is a plain delegatecall with no selector
  shadowing.
- Pass: all three observed.
- Covers: US-42, US-43, SM-C2, SM-C3, SM-C4, SM-C6, TM-09, TM-10, TM-15, XR-F-23, XR-G-3, INV-401.
- Slot: none. Wall time: 30 min.

---

## J. Checks (read/assert only, no transaction forced)

### C-01 Depositor identity
`ReservationAcceptanceRequested` and `DepositRevealed` both carry the revealing `depositor`; the
acceptance request's `msg.sender` equals it. Covers SM-A1, INV-401.

### C-02 Owner field after settlement
`reservations(key).owner == deposit.depositor`, and the owner is the depositor that requested.
Covers SM-A2, AR-02.

### C-03 Pending marker only for reserved deposits
A reserved deposit appears in `pendingReservedDeposits`; an ordinary deposit never does.
Covers SM-D1, SM-D2, SM-S5, US-29.

### C-04 Pending-deposit conservation
`pendingReservedDeposits` equals the number of deposits carrying a live pending marker, re-read
after every release, stale notify and settlement. Covers SM-D3, SM-D4, INV-206, INV-109.

### C-05 Vault trust read
`isVaultTrusted(vault)` before, during and after S-28, plus element 0 of `reservationParameters()`.
Covers SM-V1, INV-110, AR-16, US-62.

### C-06 Counter conservation
`activeReservationsCount` equals open positions plus pending acceptances; `reservationTotalAmount`
equals their anchor amounts; per-wallet count and amount agree. Checked after every D-step.
Covers INV-201, INV-202, INV-203, INV-204, SM-S4, EC-18, PAR-X4.

### C-07 Vault credit decomposition
Gross mint, fee retained, net to the depositor, no residual balance, debt equal to any financed
miner fee. Covers INV-207, INV-209, INV-701, INV-704, INV-705.

### C-08 Amount follows the current anchor
After a re-anchor, the per-wallet amount and the recorded `amount` track the new anchor and the
source's counters are released. Covers INV-302, INV-303, SM-S3, INV-205.

### C-09 Generation log
`requestNonce` is monotonic and every `(key, nonce)` action is readable with its state (`Pending`,
`TimedOut`, `Settled`, `Superseded`). Covers INV-501, INV-502, SM-X1, SM-X2, SM-X5, SM-X6, XR-S-1,
XR-S-3.

### C-10 Capability matrix
For governance, spv, a proven member, a third party and anyone, record which of the mutating entry
points succeeded and which reverted. Covers INV-401, INV-402, SM-A3, SM-A4, SM-A5, SM-A7, PAR-F2,
XR-F-20.

### C-11 Metric baseline and final snapshot
Every reservation gauge and counter at S-08 and at the end of the drill, with the runbook section 4
signal list. Covers XR-M-1..XR-M-12, SM-C16, US-50, US-51, US-100, PAR-K1, PAR-K2.

## K. Capacity schedule across D1-D12 and the new steps

The count column is `activeReservationsCount` (cap 2).

| Order | Step | Count before -> after | Note |
|---|---|---|---|
| 0 | S-01..S-09, S-11, S-12, S-18, S-95, S-97, S-98 | 0 -> 0 | deploy, governance, no slot |
| 0 | S-13 + S-53 (D9) | 0 -> 0 | reveal only, no slot; runs in the background |
| 0.5 | S-13 + S-15 + S-35 | 0 -> 1 -> 1 | scratch position for the Live-source privileged path; strand or release before D1 proper |
| 1 | D2 / S-24 | 0 -> 1 -> 0 | timeout returns the slot |
| 2 | D1 / S-15 + S-21 + S-22 (P1) | 0 -> 1 | run S-14, S-17, S-26, S-27, S-28, S-29 against P1 |
| 3 | D1 (P2) | 1 -> 2 | full; run S-25, S-40 checks and S-91 cap strings here |
| 4 | D12 / S-16 | 2 -> 2 | release before any request |
| 5 | D3 + D5 / S-31 + S-36 + S-37 | 2 -> 2 | re-anchor does not free a slot; run S-29, S-62, S-63 after the settle |
| 6 | D4 + D6 / S-32, S-33, S-34, S-50, S-55, S-38 | 2 -> 2 -> 1 | one stranding frees one |
| 6 | D6 tail / S-53, S-96 | 1 -> 1 | stale and guard sets |
| 7 | D7 / S-51 | 1 -> 2 -> 1 | fresh position on a test wallet |
| 8 | D8 / S-52 | 1 -> 2 -> 1 | requires the fraud test setup |
| after 6-8 | S-40 | 1 -> 1 | the hand-off gate reverts once two wallets have retired |
| any | S-10, S-94, S-54 | unchanged | guard, sweep and force-stale steps |
| any | S-83 | unchanged | restart during a pending generation |
| any | D10 / S-81, D11 / S-82 | unchanged | fault injection |
| any | S-71..S-76 | unchanged | read-only alerts |
| +26 h | S-19 | unchanged | Bitcoin refund |
| any | C-01..C-11 | unchanged | assertions |