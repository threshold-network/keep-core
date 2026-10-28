# Sepolia e2e test plan - step catalog, part 1 (sections A-D)

Actor legend: `governance` = Bridge owner (BridgeGovernance, a contract on Sepolia), `vaultOwner`
(Ownable owner of ReservationVault, the same governance address), `spv` = SPV maintainer
(`isSpvMaintainer`), `depositor` = the revealing Ethereum address, `sdk` = the depositor tooling
(`ReservationsService`), `operator` = a source-wallet signing-group member running keep-core,
`keeper` = the independent unstaked maintainer running keep-core plus the monitoring receiver,
`thirdParty` = an outside EOA that is not owner, maintainer or member, `anyone` = permissionless.

Sepolia sizing used throughout (tbtc-v2 `6237b2741`, `scripts/reservations-sepolia-rehearsal.ts`):
`maxActiveReservations` 2, `maxReservationsPerWallet` 1, `maxReservationsAmountPerWallet`
2,000,000 sat, `reservationMaxSingleAmount` 2,000,000 sat, `reservationMaxTotalAmount`
4,000,000 sat, `maxThirdPartyReanchorHoldsPerWallet` 1, `handoffReserveWallets` 2,
`reservationMinAmount` 10,000 sat, `reservationTxMaxFee` 1,000 sat, `reservationTermSeconds`
7,776,000 (90 d), `reservationDissolutionDelay` 86,400, `reservationActionTimeout` 86,400,
`reservationRenewalWindowSeconds` 86,400, `feeReserveTarget` 0, deposit dust threshold
1,000,000 sat, 4 Live wallets, Bitcoin testnet4. Governance delay is real time.

Two sizing facts drive the whole order:
- Global capacity is **2**. Every accepted position holds a slot until an acceptance timeout or a
  stranding. A successful re-anchor does not free one.
- The hand-off reserve gate is `countAfterThis <= max(0, liveWalletsCount - 2) * 1`
  (`Reservation.sol:1070-1078`). With 4 Live wallets the budget is 2; after two retirements the
  budget is 0 and every fresh acceptance reverts with `Hand-off reserve exceeded`.

D1-D12 keep their ids from `docs/security/pr-4282-audit/testnet-drill-runbook.md` section 3.

---

## A. Deploy and activation

### S-01 Deployment, proxy upgrade, state preservation
- Actor: governance (deployer/ProxyAdmin).
- Preconditions: Sepolia fork or live network, all four live addresses known.
- How: `PHASE=prepare npx hardhat run --network sepolia scripts/reservations-sepolia-rehearsal.ts`
  (steps 02a-04).
- Expected: shared libraries, bridge implementation, `ReservationRouter`, `ReservationVault`,
  `BridgeGovernance` deployed; vault ownership transferred to governance; proxy admin upgrades the
  Bridge; the before/after snapshot of `activeWalletPubKeyHash`, `depositParameters`,
  `liveWalletsCount` and every registered wallet is identical.
- Pass: script step `04-state-preservation` PASS with no `<key> changed on upgrade`.
- Covers: SM-C1..C8, SM-A5, SM-A6, SM-A8, AR-03, AR-16, PAR-9, XR-G-1, XR-M-1.
- Slot: none. Wall time: about 15 min.
- Fits: run first, before D0.

### S-02 Governance transfer begin/finalize, with the delay negatives
- Actor: governance (old then new BridgeGovernance).
- How: `beginBridgeGovernanceTransfer(newGov)`; attempt `finalizeBridgeGovernanceTransfer()`
  immediately (expect `Governance delay has not elapsed`); wait the real delay; finalize; attempt a
  second finalize (expect `Change not initiated`).
- Pass: governance storage slot 0 equals `newGov`; both negative strings observed.
- Covers: US-64, PAR-20 (delay), DR-03.
- Slot: none. Wall time: the real delay (about 15 min on the rehearsal delay, 2 days on the
  mainnet-scale delay - the rehearsal uses 60 s).
- Fits: immediately after S-01.

### S-03 setReservationRouter, one-shot
- Actor: governance.
- How: `setReservationRouter(router)` once; assert `reservationRouter()`; attempt a second call
  and a zero-address call (both revert; the value is written once).
- Pass: router address readable through the Bridge; the repeat call reverts.
- Covers: US-63, DR-05, PAR-10, SM-C2, SM-C3, SM-C4, SM-C5, XR-F-9, INV-703.
- Slot: none. Wall time: 2 min.

### S-04 Feasibility assertion on the sized Sepolia values
- Actor: governance (offline assert, then the on-chain guard).
- How: run `assertReservedDepositsFeasible` with the rehearsal values; then submit a deliberately
  infeasible cap (single cap below the 1,000,000 sat dust threshold) through the begin/finalize
  ceremony and record the revert.
- Pass: the assert passes for the rehearsal values; the infeasible cap reverts in finalize.
- Covers: PAR-X3, US-66, PAR-1, PAR-11, PAR-12, PAR-15, PAR-16, PAR-5, EC-05, FM-10, TM-14,
  DR-03, INV-704.
- Slot: none. Wall time: 30 min including the delay.

### S-05 Staged caps begin/finalize
- Actor: governance.
- How: `beginReservationCapsUpdate(2000000, 2000000, 2, 1, 2)`; finalize after the delay; read
  `reservationCaps()` and `reservationHandoffReserveWallets()`.
- Expected: `ReservationCapsUpdated` with five values; the views return exactly the staged values.
- Pass: caps equal the table above and the reserve equals 2.
- Covers: PAR-6, PAR-13, PAR-14, PAR-F1, PAR-X4, US-61, XR-F-2, XR-F-10, XR-F-23, SM-S4, C-08.
- Slot: none. Wall time: 30 min.

### S-06 Staged parameters begin/finalize
- Actor: governance.
- How: `beginReservationParametersUpdate(vault, 10000, 1000, 7776000, 86400, 4000000, 1, 86400,
  86400)`; finalize after the delay; read `reservationParameters()`.
- Pass: the first tuple value is the deployed `ReservationVault` (the AR-16 wiring check) and
  `reservationTotalAmount` (index 6) is 0.
- Covers: PAR-2, PAR-3, PAR-4, PAR-7, PAR-8, PAR-9, PAR-19, PAR-X5, PAR-X6, PAR-X7, US-60,
  US-28, US-62, XR-F-9, XR-T-1..XR-T-5, SM-C15, AR-16, INV-703.
- Slot: none. Wall time: 30 min.

### S-07 Activation block B, wiring read, setVaultStatus(true)
- Actor: governance, after every drill operator runs the client build that carries
  `reservationsActivationBlocks[Sepolia] = B`.
- How: read `reservationParameters()` through the Bridge and confirm element 0 is the deployed
  vault; then `setVaultStatus(vault, true)`; confirm the tx mined at or after B.
- Expected: `isVaultTrusted(vault) == true`.
- Pass: runbook section 2 step 5 satisfied; no reserved reveal is ever unserved.
- Covers: SM-C10, FM-11, PAR-9, US-62, INV-703, EC-17, AR-16, TM-15, SM-A4.
- Slot: none. Wall time: pick B at least 1 day ahead, then wait.

### S-08 Post-activation baseline read
- Actor: keeper.
- How: read `activeReservationsCount()`, `pendingReservedDeposits()`, `reservationParameters()`,
  `reservationCaps()`, `isVaultTrusted`, `liveWalletsCount`.
- Pass: every counter is 0, caps and parameters match the table, vault trusted.
- Covers: INV-201, INV-202, INV-203, INV-204, INV-206, SM-S4, SM-V1, SM-W1, XR-F-3..XR-F-6,
  XR-F-21, XR-M-1..XR-M-4, XR-M-6, SM-D1, SM-D5.
- Slot: none. Wall time: 10 min.

### S-09 Governance guard set (negative parameter and cap writes)
- Actor: governance.
- How: attempt, each reverted, `maxActiveReservations = 0`
  (`Active reservations cap must be greater than zero`), `handoffReserveWallets = 0`
  (`Hand-off reserve must be greater than zero`), `reservationTxMaxFee = 0`
  (`Reservation transaction max fee must be greater than zero`),
  `reservationRenewalWindowSeconds >= termSeconds` (`Renewal window must be shorter than the term`),
  an out-of-bounds term (`Reservation term out of protocol bounds`), an action timeout that leaves
  no signing window (`Reservation action timeout leaves no signing window`),
  `maxReservationsPerWallet = 0` (`Max reservations per wallet must be greater than zero`),
  a total cap above slot capacity (`Amount cap exceeds slot capacity`), a cap below the standing
  total (`Active reservations exist`), and each `finalize` before the delay
  (`Governance delay has not elapsed`).
- Pass: every attempt reverts with the exact string and no parameter changes.
- Covers: PAR-13, PAR-F1, PAR-2, PAR-8, PAR-3, PAR-7, PAR-X1, PAR-X2, PAR-X5, US-61, US-64,
  EC-08, EC-09, EC-10, TM-14, FM-10, DR-03, AR-14.
- Slot: none. Wall time: about 2 h (one begin/finalize pair per rejected value that gets far
  enough to be evaluated).

### S-10 Pending-reserved-deposit guard on a parameters write
- Actor: governance, while at least one reserved deposit is revealed but unserved.
- How: after S-13, attempt `beginReservationParametersUpdate`/`finalizeReservationParametersUpdate`
  with a different vault; finalize reverts with `Pending reserved deposits exist`. After the
  deposit clears (release to pool, stale notify or settlement), the same ceremony succeeds.
- Pass: the revert is observed while the marker exists and the write succeeds after it clears.
- Covers: INV-109, INV-106, INV-110, US-66, TM-12, DR-04.
- Slot: none (the reveal holds no slot). Wall time: 30 min plus one delay.
- Fits: after D12 (the released deposit) and before D1, or interleaved with D0/D9.

---

## B. Depositor and SDK flows

### S-11 SDK pre-funding go/no-go
- Actor: sdk.
- How: `checkReserveFeasibility` for 1,000,000 sat against a Live wallet with room (expect go),
  then for 500,000 sat (below dust), 2,500,000 sat (above single cap), and 2,500,000 sat again
  after the total cap is consumed (`global-count-cap-exceeded`).
- Pass: the service reports go only when every rule passes and names the failing rule otherwise.
- Covers: US-93, US-98, SM-C11, SM-C12, SM-C13, SM-C14, PAR-1, PAR-11, PAR-12, EC-06.
- Slot: none. Wall time: 15 min.

### S-12 SDK wallet selection and initiateReservedDeposit
- Actor: sdk.
- How: `selectReservationWallet` over the four Live wallets; `initiateReservedDeposit` with an
  explicit wallet and with the automatic choice; confirm the receipt's refund locktime lies inside
  `[now + 24 h, now + term + 24 h]` (91 days on Sepolia, so the SDK's ordinary 180-day locktime is
  refused and the reservation locktime is used).
- Pass: a deterministic wallet, a P2PKH or P2WPKH refund address, and an in-window locktime.
- Covers: US-94, US-95, US-96, EC-16, PAR-X6, INV-603, SM-C12.
- Slot: none. Wall time: 30 min plus a Bitcoin confirmation.

### S-13 Reserved reveal
- Actor: depositor, permissionless.
- How: reveal the funded output with `vault = reservationVault`; read the deposit, the pending
  reserved-deposit record and `pendingReservedDeposits()`.
- Expected: `DepositRevealed` with the reserved vault; `pendingReservedDeposits` rises by 1;
  `activeReservationsCount` unchanged.
- Pass: the reveal is recorded, no global slot is consumed, the refund deadline is snapshotted.
- Covers: US-01, SM-R1, SM-D2, SM-W1, SM-D1, INV-108, INV-109, PAR-X6, XR-E-4.
- Slot: none (this is the D0/D9 deposit and the D1 deposits). Wall time: 1 Bitcoin confirmation
  plus the deposit reveal.

### S-14 SDK status reads
- Actor: sdk.
- How: `getReservation(key)`, `getReservationAction(key, nonce)`, `reservationParameters()`,
  `isReservedDeposit(key)`, `walletReservationsCount/Amount(w)`, `activeReservationsCount()`,
  `pendingThirdPartyReanchorHolds(w)`, `reservationByAnchorUtxo(...)`.
- Pass: every read agrees with the raw `eth_call` result and with the emitted events.
- Covers: US-99, SM-C11, SM-C13, XR-F-3, XR-F-4, XR-F-5, XR-F-6, XR-F-7, XR-F-8, XR-F-21,
  XR-S-1, XR-S-2, PAR-X4.
- Slot: none. Wall time: 30 min. Runs at three points: after S-13, after D1, after D6.

### S-15 SDK acceptance request
- Actor: sdk / depositor.
- How: `requestAcceptance(fundingTxHash, fundingOutputIndex, walletPubKeyHash)` once the funding
  transaction has the Bridge's required confirmations.
- Expected: `ReservationAcceptanceRequested(key, 1, wallet, amount, txMaxFee, timeoutAt)`;
  `activeReservationsCount` +1; `ReservationOccupancyChanged`; the per-wallet counters +1.
- Pass: the request lands within the signing window and the counters move together.
- Covers: US-03, US-97, US-04, US-98, SM-A1, SM-A2, SM-R3, SM-X1, SM-X2, INV-201, INV-202,
  INV-203, INV-204, EC-18, XR-E-1, XR-F-17, XR-F-4.
- Slot: **consumes one** (0->1 or 1->2). This is the request half of D1.

### S-16 Release to pool
- Actor: depositor (or governance repointing the vault afterwards).
- How: `releaseReservedDepositToPool(depositKey, zeroAddress or a trusted non-reservation vault)`
  through the Bridge; then let the ordinary deposit sweep include it; the depositor's Bank balance
  rises.
- Expected: the deposit is no longer reserved, `pendingReservedDeposits` drops, the ordinary sweep
  credits the depositor (or the chosen vault).
- Pass: the deposit leaves the reservation set and is sweepable as an ordinary deposit; the vault
  can then be repointed.
- Covers: US-90, US-99, US-80, SM-D3, SM-D4, INV-111, INV-206, AR-11, XR-F-22.
- Slot: none (the depositor must run this before any acceptance request).
- Fits: this is D12 (order 4); also re-run after D9 for the stale deposit.

### S-17 Boundary and guard set at acceptance request
- Actor: `anyone` and `depositor` in turn.
- How (each a reverted transaction or `eth_call` with the exact string):
  `Deposit amount too small for a reservation`, `Reservation exceeds the single-reservation cap`,
  `Total reserved amount cap exceeded`, `Wallet reservations cap exceeded`,
  `Wallet reserved amount cap exceeded`, `Active reservations cap exceeded`,
  `Caller is not the deposit's depositor`, `Wallet is not the deposit's designated wallet`,
  `Wallet must be in Live state`, `Deposit was not revealed as reserved`,
  `Deposit not routed to the reservation vault`, `Reservation already exists`,
  `Acceptance already pending`, `Deposit already swept`, `Deposit not revealed`,
  `Reservations are disabled`, `Acceptance authorization has no signing window`,
  `Authorization window would overlap the deposit refund window`,
  `Deposit refund deadline allows no signing window`.
- Pass: each observed exactly once with the recorded block number.
- Covers: US-04, US-05, US-02, US-92, EC-06, EC-07, EC-08, EC-09, EC-10, EC-18, EC-03, SM-R2,
  SM-X5, INV-101, INV-108, INV-112, INV-203, TM-03, TM-12, TM-16, PAR-X4, XR-P-2.
- Slot: none (all reverted). Wall time: about 4 h across the drill.
- Fits: S-15's window guards before D1, the caps after D1-P2 makes the count 2, the wallet caps
  after S-26.

### S-18 Refund-locktime window at reveal
- Actor: depositor.
- How: reveal with `refundLocktime` exactly `now + term + 24 h` (accepted) and with
  `now + term + 24 h + 1` (reverts `Refund locktime too far in the future for a reservation`);
  separately reveal with a locktime below `now + depositRevealAheadPeriod` (reverts
  `Deposit refund locktime is too close`, the ordinary guard at `Deposit.sol:512-517`).
- Pass: the boundary value is accepted and the one-second overshoot is refused.
- Covers: US-02, US-94, EC-06, EC-03, PAR-X6, INV-603, TM-01, XR-P-4.
- Slot: none. Wall time: 30 min.

### S-19 Deposit refund on Bitcoin
- Actor: depositor.
- Preconditions: a reserved reveal with the shortest admissible locktime (`now + about 25 h`)
  that is later released to the pool or marked stale.
- How: after the refund deadline passes, spend the funding output through the script-path refund.
- Pass: the output is spendable by the depositor's key and the on-chain record already reads
  non-reserved.
- Covers: US-12, US-90, AR-02, AR-11, EC-15, SM-A1.
- Slot: none. Wall time: about 26 h from reveal.

---

## C. Wallet and SPV

### S-21 Acceptance proposal discovery, signing, broadcast
- Actor: operator (keep-core `ReservationAcceptanceTask`).
- How: the operator reads the pending acceptance, assembles and broadcasts the anchor transaction.
- Expected: the anchor output pays the wallet and its fee is within the snapshotted bound.
- Pass: a signed anchor on testnet4 inside the signing window, without a cap-revert retry.
- Covers: US-20, US-21, US-22, US-29, XR-P-1, XR-P-2, XR-P-3, XR-P-4, XR-P-7, PAR-2, PAR-19.
- Slot: none extra (the request already took it). Wall time: one coordination window (about 5 min
  to 2 h depending on the operator set).
- This is the middle of D1.

### S-22 Acceptance SPV proof settlement
- Actor: spv.
- How: `submitReservationAcceptanceProof(txInfo, proof, key, nonce)`.
- Expected: the reservation becomes `Active`, the deposit is swept, the Bank mints gross, the vault
  keeps the fee and credits the depositor net, the anchor index records the outpoint.
- Pass: `state == Active`, `mintedAmount == anchorAmount`, vault fee retained, no debt because the
  reserve is 0 and the anchor fee is financed (or recorded as debt).
- Covers: US-07, US-11, US-33, US-40, SM-R3, SM-R5, SM-A4, INV-209, INV-701, INV-702, INV-301, EC-14,
  EC-16, EC-20, XR-F-11, XR-P-1.
- Slot: none extra. Wall time: the Bitcoin confirmation count plus one Ethereum block.
- This is the tail of D1.

### S-23 Late acceptance settlement unwinding a newer generation
- Actor: spv.
- Preconditions: an acceptance generation timed out (S-24), a second acceptance generation was
  requested for the same deposit, then the first generation's anchor confirms.
- How: submit the first generation's proof after the timeout.
- Expected: the newer pending generation is unwound (`Superseded`), the older generation settles as
  a late acceptance, the anchor lineage closes.
- Pass: final state `Active`, one settlement, no double refund.
- Covers: US-09, US-08, SM-R4, SM-R9, SM-X5, SM-X6, INV-502, INV-206, EC-04, EC-14, INV-105.
- Slot: one (whichever generation holds it at the end). Wall time: 24 h timeout + confirmations.

### S-24 Acceptance timeout, capacity release, late settlement
- Actor: `anyone` then spv.
- How: `notifyReservationAcceptanceTimedOut(key)` at `timeoutAt` (an attempt one block earlier
  reverts `Action has not timed out`); observe the count return to 0 and the re-authorization
  succeed; then submit the original anchor proof as a late acceptance.
- Pass: count 1 -> 0, a fresh generation can be requested, the late proof settles.
- Covers: US-06, US-47, US-48, EC-01, EC-10, SM-R2, SM-R5, SM-X5, INV-601, INV-201, INV-202,
  XR-F-13, XR-F-14, PAR-K1, PAR-K2, S-72 (keeper late-proof alert).
- Slot: **releases one**. Wall time: 24 h (the action timeout) + confirmations.
- Fits: this is D2 (order 1).

### S-25 Preflight passes, on-chain request reverts (TOCTOU)
- Actor: sdk then depositor.
- How: run `checkReserveFeasibility` while capacity is free, then let the other position take the
  last slot before the request is sent, and submit the request.
- Expected: the request reverts `Active reservations cap exceeded` even though the SDK said go.
- Pass: the documented TOCTOU limit is observed with a recorded block ordering.
- Covers: SM-C14, US-98, US-93, US-04, EC-18, INV-113, PAR-X4.
- Slot: none (the request reverts). Wall time: 30 min.
- Fits: between D1-P2 and D12, while the count is 2.

### S-26 Per-wallet count cap
- Actor: depositor.
- Preconditions: `maxReservationsPerWallet = 1` and a wallet with one active position.
- How: reveal a second reserved deposit to the same wallet and request acceptance.
- Expected: `Wallet reservations cap exceeded`.
- Pass: the wallet holds exactly one position and the second request is refused.
- Covers: INV-203, PAR-6, EC-08, EC-09, US-05, TM-02, TM-14.
- Slot: none. Wall time: 30 min.

### S-27 Anchor miner fee above the snapshotted bound
- Actor: operator, or `anyone` replaying a valid proof with a fee above `txMaxFee`.
- How: broadcast an anchor whose fee exceeds `reservationTxMaxFee` (1,000 sat), then submit the
  proof.
- Expected: `Transaction fee is too high` at proof submission (and the anchor itself is rejected by
  `validateReservationAnchorProposal` `Proposed transaction fee is too high` before signing).
- Pass: the exact string is observed and no state changes.
- Covers: INV-303, US-45, US-07, XR-P-1, PAR-2, EC-07, EC-15.
- Slot: none. Wall time: 1 h.

### S-28 Vault trust toggled mid-flight
- Actor: governance.
- How: after a settled acceptance, `setVaultStatus(vault, false)`; read `isVaultTrusted`; attempt a
  further reserved reveal (reverts `Vault is not trusted`) and read the already-settled credit.
- Expected: trust flips to false; the settled position keeps its credit; the routing switch is the
  only thing that decides a new settlement's destination.
- Pass: the vault status read is false, the settled credit is unchanged, and the negative reveal
  reverts.
- Covers: INV-110, INV-702, US-62, US-80, EC-20, DR-04, SM-V1, TM-11.
- Slot: none. Wall time: 30 min.
- Fits: after D1-P1 settles, before D3.

### S-29 Anchor double-consume replay
- Actor: `anyone`.
- How: resubmit an already settled acceptance proof, and an older anchor against a newer anchor.
- Expected: `Reservation anchor already spent`, and `Action source anchor is no longer current` for
  the stale-anchor case.
- Pass: both reverts with no counter movement.
- Covers: EC-15, EC-14, INV-301, US-45, US-44, TM-04, SM-X6.
- Slot: none. Wall time: 30 min.
- Fits: right after S-22 and after S-37.

---

## D. Re-anchor and hand-off

### S-31 Member hand-off request (source wallet)
- Actor: operator, as a proven member of the retiring source wallet.
- How: `requestReservationReanchor(key, target, walletMembersIDs, memberIndex)` from a MovingFunds
  source with the member proof in the call data, no outside accounts involved.
- Expected: a pending re-anchor action on the target, target capacity reserved.
- Pass: keep-core does not race with an outside request; the member path is the one that settles.
- Covers: US-23, US-74, SM-A3, SM-R6, SM-X3, INV-107, INV-402, XR-F-1, XR-R-2.
- Slot: none (the position keeps its slot; the target's per-wallet counters move).
- Fits: D3 + D5 (order 5).

### S-32 Third-party re-anchor request
- Actor: thirdParty.
- Preconditions: a MovingFunds or Closing source, a Live target with ordinary headroom.
- How: an outside EOA calls `requestReservationReanchor`.
- Expected: success, `pendingThirdPartyReanchorHolds(target) == 1`.
- Pass: the hold counter rises by one and is visible through the router view.
- Covers: US-70, US-31, SM-A7, SM-R6, INV-205, AR-01, AR-08, PAR-F2, TM-02, US-67.
- Slot: none.
- Fits: D4 first half (order 6), before keep-core claims the position.

### S-33 Third-party hold cap exceeded
- Actor: thirdParty.
- How: a second outside EOA requests the same reservation while one hold is outstanding.
- Expected: `Target third-party re-anchor holds exceeded`.
- Pass: the revert and `pendingThirdPartyReanchorHolds(target)` stays 1.
- Covers: US-71, US-72, PAR-14, INV-205, SM-X3, EC-09, TM-02, AR-01, AR-08.
- Slot: none.
- Fits: D4 second half (order 6), immediately after S-32.

### S-34 Privileged hand-off to a full target
- Actor: governance (Bridge owner), spv, or a proven member.
- Preconditions: the target's ordinary count and amount caps are full or occupied by another
  position's hold.
- How: the privileged caller requests the re-anchor anyway.
- Expected: success; the target's count exceeds its normal cap; `pendingThirdPartyReanchorHolds`
  unchanged; the global active count unchanged.
- Pass: the bypass applies to caps but not to the same-position pending-action rule or the
  non-governance cooldown.
- Covers: US-67, US-73, US-30, PAR-F2, INV-114, INV-115 (bookkeeping), AR-01, AR-15, R1 residual,
  FM-08, DR-01, TM-08.
- Slot: none.
- Fits: D4 third half (order 6).

### S-35 Live-source privileged re-anchor
- Actor: governance.
- How: with the source still Live, governance rotates the anchor; the same call from a third party
  reverts `Only governance can rotate a Live wallet's anchor`, and a MovingFunds-source call from an
  outside account reverts `Source wallet must be in MovingFunds or Closing state`.
- Pass: only the privileged path succeeds from a Live source.
- Covers: US-30, US-67, INV-401, INV-402, SM-A3, SM-A5, TM-03, PAR-F2, XR-P-6, XR-F-20, EC-19.
- Slot: none. Wall time: 1 h.
- Fits: before any retirement starts (order 0.5), on a scratch position.

### S-36 Re-anchor task: cap revert, resume, aged target
- Actor: operator (keep-core `ReservationReanchorTask`).
- How: run with a target whose caps are full (expect the task to move to the next candidate), with
  an in-flight pending re-anchor (expect resume without a new request), and with only young targets
  (expect skip to the aged one).
- Pass: one request per generation, never two; the task picks an eligible target.
- Covers: US-25, US-26, US-27, US-33, EC-19, XR-R-1, XR-R-2, PAR-19, PAR-X1, PAR-X8, XR-M-7,
  XR-M-9, TM-03, AR-09.
- Slot: none. Wall time: 2 coordination windows.
- Fits: interleaved with S-31 and S-32.

### S-37 Re-anchor SPV proof settlement (on time)
- Actor: spv.
- How: `submitReservationReanchorProof(txInfo, proof, key, nonce)`.
- Expected: `state` returns to `Active` on the target wallet, the reverse anchor index moves, the
  source's custody ledger releases, the target's holds drop.
- Pass: one settlement; `pendingThirdPartyReanchorHolds(target) == 0` afterwards.
- Covers: US-33, US-41, SM-R5, SM-R7, SM-X4, INV-205, INV-301, INV-302, INV-304, INV-503, EC-14,
  US-75, XR-F-12, XR-P-6.
- Slot: none. Wall time: confirmations + one block.
- This is the tail of D3/D5 and of D4.

### S-38 Re-anchor action timeout, target capacity release, cooldown
- Actor: `anyone`, then thirdParty.
- How: with the source below quorum so the re-anchor is never signed, call
  `notifyReservationActionTimeout(key)` at `timeoutAt` (an earlier attempt reverts
  `Action has not timed out`); observe `Active` restored, the target's counters released and
  `reanchorCooldownUntil` set; then a third-party request reverts `Reanchor cooldown in effect`.
- Pass: capacity returns, the cooldown blocks non-governance callers, governance is unaffected.
- Covers: US-32, US-75, EC-01, EC-02, SM-X3, SM-X5, SM-X6, INV-107, INV-601, INV-602, INV-205,
  AR-14, AR-15, XR-F-13, TM-06, US-47.
- Slot: none. Wall time: 24 h.
- Fits: the tail of D4 (order 6), after the source fails to sign.

### S-39 Late re-anchor proof after the timeout
- Actor: spv.
- Preconditions: the re-anchor action timed out (S-38) and its anchor later confirms.
- How: submit the proof for the timed-out generation.
- Expected: settlement runs; the released target capacity is re-taken.
- Pass: the position settles once and the counters end consistent.
- Covers: SM-X6, SM-R9, INV-502, INV-503, US-41, US-33, AR-06, TM-04.
- Slot: none. Wall time: confirmations after the 24 h timeout.

### S-40 Hand-off reserve gate at acceptance
- Actor: depositor.
- Preconditions: retirements have taken `liveWalletsCount` to 2 or fewer, while
  `activeReservationsCount` is 1 (below `maxActiveReservations`).
- How: request a fresh acceptance.
- Expected: `Hand-off reserve exceeded`, even though the global cap has room.
- Pass: the gate reverts now rather than at the retirement cliff; then restore the Live count and
  the same request succeeds.
- Covers: INV-113, PAR-F1, PAR-X4, EC-18, INV-115, SM-C15, AR-15, US-05, INV-112.
- Slot: none. Wall time: 1 h.
- Fits: after D6/D7/D8 have retired two wallets, while the count is 1.

---


Continued in [part 2](steps-2.md).
