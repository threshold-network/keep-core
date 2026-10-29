# Deposit, timing and governance boundaries

Evidence pins and vocabulary: [edge cases index](05-edge-cases.md).

### EC-01 Exact-equality timeout at `timeoutAt`

Expected: a pending acceptance generation is reportable as timed out the
instant its window closes, and stays settleable as a late acceptance
after the report.

Actual: `notifyReservationAcceptanceTimedOut` reverts only when
`block.timestamp < action.timeoutAt`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:688`); a call at
`block.timestamp == timeoutAt` succeeds and releases the reserved
capacity. The re-anchor equivalent uses the same `>=` check
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:958`). A late proof
of the timed-out generation still settles until the chain head passes
`timeoutAt + termSeconds`
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:204`).

Test: `test/bridge/Reservation.test.ts:1712` ("should succeed when
called after advancing time past timeoutAt"); `test/bridge/Reservation.test.ts:1687`
(the not-yet-timed-out revert).

Status: holds.

### EC-02 Re-anchor cooldown exact equality

Expected: after a timed-out re-anchor, an ordinary request waits until
the cooldown deadline. Equality passes this guard.

Actual: timeout sets `reanchorCooldownUntil = now + (timeoutAt - requestedAt)`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:984`).
The request requires `block.timestamp >= reanchorCooldownUntil`
when its `privileged` argument is false
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:758-762`).
Only the governance-discriminated router entry supplies true.
The owner, SPV maintainer and proven source member bypass the hold cap,
not the cooldown. They must also wait.
Equality passes the cooldown check; all other request guards still apply.
The client uses chain time for the same deadline
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:472`).

Test: `test/bridge/Bridge.ReservationStrandingLibrary.test.ts:1610`
("rejects when reanchor cooldown is in effect");
`keep-core` `TestReservationReanchorTask_CooldownGateUsesChainTime`
(`pkg/tbtcpg/reservation_reanchor_custody_test.go:265`).

Status: holds-untested (the cited tests cover an active cooldown, not the exact equality second).

### EC-03 Refund deadline exact equality

Expected: the depositor's on-chain refund path and the wallet's signing
path can never overlap; the boundary second belongs to the refund path.

Actual: two different comparisons meet at the deadline.
`notifyStaleReservedDeposit` requires strictly
`block.timestamp > pendingDeposit.refundDeadline`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1250`), so at
`==` it reverts and succeeds one second later. The request-time
overlap guard instead allows equality: `timeoutAt + 24h <= refundDeadline`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:594`), and the
signing-window guard requires
`earliestSigningAt < refundDeadline - 24h`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:600`). At the
boundary `timeoutAt + 24h == refundDeadline` the request succeeds and
the earliest proposal timestamp lands exactly on the validator's
refund-margin cutoff.

Test: `test/bridge/Reservation.test.ts:405` ("should succeed when
timeoutAt + 24h == refundDeadline (close bound exact)");
`test/bridge/Reservation.test.ts:433` (one second later, revert);
`test/bridge/Bridge.ReservationStranding.test.ts:727` (stale notify at
an elapsed deadline).

Status: holds.

### EC-04 Late acceptance window: `timeoutAt + term`

Expected: a timed-out acceptance stays provable for one full term
after its timeout, then never.

Actual: `loadSettleableAction` reverts a late acceptance only when
`block.timestamp > timeoutAt + termSeconds`
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:204`), so
the bound is inclusive. The `termSeconds` is the request-time snapshot
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:639`), so a later
governance change of the live term does not shrink or extend a
generation's window. The client evicts a tracked TimedOut candidate on
the same chain-head comparison
(`keep-core:pkg/maintainer/spv/reservation_proof_loop.go:547`).

Test: `test/bridge/ReservationProofs.test.ts:851` ("should settle a
timed-out older generation without newer generation (late
settlement)"); `keep-core`
`TestProveReservationAcceptanceActions_TimedOutLateWindow`
(`pkg/maintainer/spv/reservation_proof_loop_test.go:1814`).

Status: holds-untested (no test pins the inclusive boundary second of
the late window itself).

### EC-05 Term + 24h: reveal-ahead period bound

Expected: a reserved deposit is feasible only while the reveal-ahead
period leaves a refund window; the margin is 24 hours
(`DEPOSIT_REFUND_SAFETY_MARGIN`, `tbtc-v2:solidity/contracts/bridge/WalletProposalValidatorConstants.sol:13`).

Actual: a reserved reveal reverts when its refund locktime exceeds
`now + reservationTermSeconds + 24h`
(`tbtc-v2:solidity/contracts/bridge/Deposit.sol:244`); the feasibility
guard bakes the same rule into every parameter setter while a vault is
set (`depositRevealAheadPeriod > term + 24h` reverts,
`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:689`), including
`updateDepositParameters`
(`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:642`). The client
gauge mirrors the rule with its own 24h constant
(`keep-core:pkg/maintainer/spv/reservation_params_feasibility.go:19`).
The guard is the D25 decision: it makes "no reserved deposit is
possible" a revert at write time and a `1` gauge off-chain.

Test: `test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:319`
("reverts when the new reveal-ahead period exceeds the term plus
margin"); `keep-core`
`TestReservationParamsInfeasibleRevealWindow` in
`pkg/maintainer/spv/reservation_params_feasibility_test.go`.

Status: holds.

### EC-06 Deposit amount at the minimum

Expected: the smallest reservable deposit carries the minimum plus two fee
allowances, so an anchor that pays the full fee still clears the re-anchor
floor and can be handed off once.

Actual: the request reverts when
`deposit.amount <= reservationMinAmount + 2 * reservationTxMaxFee`
(`tbtc-v2@dc5c2ed71:solidity/contracts/bridge/Reservation.sol:577-584`). At the
boundary `depositAmount == minAmount + 2 * maxFee + 1`, spending the full
allowance leaves the anchor at `minAmount + maxFee + 1`, just above the
re-anchor floor. A re-anchor hop that also pays the full fee can leave the
anchor at that floor; the position then cannot move again.
The settlement still checks
`amount - anchorAmount <= txMaxFee`
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:453`) and
the defense-in-depth `anchorAmount >= minAmount`
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:464`).
Governance keeps `minAmount > txMaxFee`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1379`).

Test: `test/bridge/Reservation.test.ts:367` (below, revert);
`test/bridge/Reservation.test.ts:385` (at boundary, success).

Status: holds.

### EC-07 Fee exactly `txMaxFee`

Expected: the fee bound is inclusive on every surface, so a wallet can
spend the full allowance.

Actual: the on-chain validator accepts `fee <= txMaxFee`
(`tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:1038`,
`:1191`); the client assembly rejects only `fee > TxMaxFee`
(`keep-core:pkg/tbtc/reservation.go:419`, `:474`); the settlement fee
delta check is `<= txMaxFee`
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:454`, `:688`).
So a proposal at exactly the bound signs, assembles, settles, and is
credited. The follower-side hard floor (below 5 sat/vByte protocol
minimum) is a different, additive gate
(`keep-core:pkg/tbtc/reservation.go:728`, `:965`).

Test: `test/bridge/Reservation.test.ts:1100` ("should succeed at the
fee-bound boundary"); `keep-core`
`TestAssembleReservationTransactions_FeeBoundaries`
(`pkg/tbtc/reservation_test.go:488`).

Status: holds.

### EC-08 Zero values on amount caps: `0` means off

Expected: a zero amount cap disables that cap; none of the three
amount caps means blocked when zero.

Actual: every amount check short-circuits on zero:
`reservationMaxSingleAmount == 0` skips the single cap
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:611`);
`maxReservationsAmountPerWallet == 0` skips the per-wallet amount cap
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1032`, `:898`);
`reservationMaxTotalAmount == 0` skips the total cap
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1013`). The
Decision-1 slot-capacity invariant is likewise skipped when either
operand is zero
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1328`).

Test: `test/bridge/Reservation.test.ts:545` (single cap 0, success);
`test/bridge/Reservation.test.ts:669` (per-wallet cap 0, success).

Status: holds.

### EC-09 Zero values on structural caps: `0` means blocked

Expected: occupancy and per-wallet count caps cannot be disabled; the
third-party hold cap `0` means "third parties may not hold".

Actual: `maxActiveReservations` must be positive at write time
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1502`) and the
acceptance check `activeReservationsCount < maxActiveReservations`
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1005`) means a
written zero blocks every new acceptance
(`test/bridge/Bridge.ReservationCaps.test.ts:153` pins the write
revert). `maxReservationsPerWallet` cannot be written as 0
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1398`), and the
count check is `<=` at request time; late proof reconstruction can exceed current caps
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:886`, `:1023`).
`maxThirdPartyReanchorHoldsPerWallet = 0` reverts every non-privileged
request because `0 < 0` is false
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:869`), while the
privileged callers (owner, SPV maintainer, proven source member,
governance) are exempt by construction
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:786`); the doc
comment states the zero meaning explicitly
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:864`).

At `tbtc-v2@806b1106f` a privileged re-anchor from a non-Live source also
skips the target's count and amount caps
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`, `:928-933`, `:942-948`),
so the `<=` count check above applies to third parties and to
governance rotation of a Live source only (PAR-F2). The acceptance request
is additionally bounded by the reserve gate
(`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1074-1077`; PAR-X4).

Test: `test/bridge/Reservation.test.ts:709` (occupancy at cap);
`test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:250`
("blocks third parties entirely when the cap is zero").

Status: holds.

### EC-10 Caps changed while actions are pending

Expected: settlement of an already-authorized generation is a
request-time throttle, not a settlement-time gate; changing caps
afterward cannot make a proven transaction unprovable.

Actual: the acceptance late path re-takes total, wallet and occupancy
capacity with no cap checks
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:524`), and
the re-anchor late path does the same
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:704`); the
code comment states the rule
(`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:522`).
The Decision-1 setter checks configured total capacity against
`maxActiveReservations * reservationMaxSingleAmount`.
It does not establish a universal bound on actual late-settled occupancy.
At `9d2086965` the re-anchor signing validator separately read current target count and amount caps
(`tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:1155-1183`).
Lowering either below existing target occupancy could block a proposal.
That policy was explicit in the validator's requirements (:1087-1089).
It was not a demonstrated accounting defect; VF-01 rejected the confirmed-defect classification.
At `tbtc-v2@806b1106f` (D31) the validator no longer re-checks either cap.
Its doc block says target count and amount capacity is enforced once at request time and not repeated
(`tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1092-1098`);
the target must still be Live ("Target wallet must be in Live state",
`tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1159`).
A cap lowered after the request therefore cannot block signing of an already-authorized re-anchor.
Caps remain request-time throttles, and a privileged hand-off from a non-Live source skips them at the request (PAR-F2).

Test: `test/bridge/Bridge.ReservationAcceptanceAuthorization.test.ts:472`
("settles an authorized acceptance even after the caps fill up");
`test/bridge/Bridge.ReservationCaps.test.ts:121`.
At `806b1106f` the settlement test is
`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAcceptanceAuthorization.test.ts:479`
("settles an authorized acceptance even after the caps fill up").
Signing without the cap re-check: mocked bridge
`tbtc-v2@806b1106f:solidity/test/bridge/WalletProposalValidator.test.ts:4322-4382`;
real validator harness
`keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1193` (count at cap),
`:1205` (count above cap) and `:1219` (amount above cap), all accepted.

Status: holds.

At the original pins this entry was graded untested for the live-cap signing consequence; the validator re-check no longer exists at `806b1106f`.
