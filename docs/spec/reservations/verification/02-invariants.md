# M1 UTXO Reservations - Invariants (INV-)

This file lists every invariant of the M1 (variant B) reservation protocol, in both
repos, checkable against the pinned code. It is the reference for (a) whether the
design is correct and (b) whether the code implements it. The code at the pinned
commits is the source of truth; prior docs are background only.

**Pinned commits**

| Repo | Pin | Branch | Citation prefix |
|---|---|---|---|
| tbtc-v2 contracts | `9d2086965` | `reservations-upgrade` | `tbtc-v2:solidity/...` |
| keep-core client | `f99ef96e3` | `reservations-epic` | `keep-core:pkg/...` |

Every normative claim cites `repo:path:line` at the pin. Line numbers were read
from the pinned blobs. Every testable item has a `Test` field (the existing test
that would fail if the rule broke, or `none`) and a `Status`.

**D26 update (post-pin).** `releaseReservedDepositToPool` (D26) is implemented
in tbtc-v2 at `d20cf74ba` (feature commit `638c48663`, one test-fix commit on
top), after the pin `9d2086965`. Rows that describe it cite
`tbtc-v2@d20cf74ba:path:line`. Other rows use the original evidence pins.
The release entry point is absent from the `9d2086965` contracts.
The original keep-core evidence remains at `f99ef96e3`.
The later client at `4b95f0a06` adds the release binding and post-release sweep tests.
The SDK module is implemented at tbtc-v2 `7aabfc502`; see the model and user stories.

**D31 update (post-pin).** The D31 hand-off exemption and acceptance reserve
gate (INV-113, INV-114) are implemented in tbtc-v2 at `806b1106f` (branch
`reservations-upgrade`) and in the keep-core testnet client at
`d40f6aadc` (branch `reservations-epic`), on the reservations branches; nothing
is deployed on Sepolia or mainnet. Rows that describe them cite
`tbtc-v2@806b1106f:path:line` or `keep-core@d40f6aadc:path:line`. Every other
row, including the target-cap rows in INV-203 and INV-204, keeps the original
evidence pins (`9d2086965`, `f99ef96e3`) and describes the behavior before D31.

**Status legend**

- `holds` - the code enforces it and a test pins it.
- `holds-untested` - the code enforces it, no test found.
- `violated` - the code breaks it (see Findings).
- `design-only` - planned, not in the pinned code.
- `unknown` - missing information to decide.

**Vocabulary** (enum values at the pin)

- Position state (`Reservation.ReservationState`, `Reservation.sol:74-109`):
  `Unknown(0)`, `Active(1)`, `ActionPending(2)`, `Closed(3)`, `Stranded(4)`.
- Action type (`Reservation.sol:110-116`): `None, Acceptance, Redemption,
  Reanchor, Dissolution` (Redemption/Dissolution are M2, unimplemented).
- Action state (`Reservation.sol:119-135`): `Unknown(0), Pending(1),
  Settled(2), TimedOut(3), Vetoed(4), Superseded(5)` (Vetoed is M2).
- Wallet state: `Live, MovingFunds, Closing, Closed, Terminated`.

M1 is create-only: no redemption, renewal, or dissolution; the vault is Ownable
and not upgradeable. `closeReservation` and the `Vetoed`/`Closed`-position
writer are M2 and unreachable in M1.

---

## 1. State legality

Which combinations of position state, action state, wallet state,
pending-deposit state, and vault status are legal.

### INV-101 One position state per key; pending acceptance is action-tracked, pending re-anchor sets ActionPending

- **Statement.** For every reservation key exactly one `state` value is
  observable. A pending *acceptance* generation leaves the position `Unknown`
  and is tracked only in the action record; a pending *re-anchor* generation
  sets the position to `ActionPending`. No position is ever observed in two
  states at once.
- **Why.** Prevents a position from being both "settled" and "has a pending
  action", which would double-spend the anchor or double-count capacity.
- **Enforcing code.** `requestReservationAcceptance` requires
  `state == Unknown` and no pending action (`Reservation.sol:531-539`);
  `requestReservationReanchor` sets `state = ActionPending` atomically with the
  `Pending` re-anchor action (`Reservation.sol:906, 915`); the `Unknown`
  comment states `ActionPending` is not set for acceptance
  (`Reservation.sol:75-83`).
- **Test.** `Reservation.test.ts` "should revert if reservation already exists
  (state == Active)" (:269); "should unwind pending reanchor action"
  (`ReservationProofs.test.ts:1072`).
- **Status.** holds.

### INV-102 A wallet that custodies live reservations cannot be finalized to Closed

- **Statement.** Two distinct ledgers gate the wallet lifecycle.
  (a) Routing, `Live -> Closing` versus `Live -> MovingFunds`: a wallet that
  still custodies a **proven** reservation anchor (`walletAnchoredInfo[w].count > 0`)
  goes to `MovingFunds` so the anchor can be re-anchored away. Capacity that is
  only reserved for a pending acceptance or re-anchor target is not custody and
  does not keep the wallet out of `Closing` (INV-211).
  (b) Finalization, `Closing -> Closed`: requires
  `walletReservationInfo[w].count == 0`, the reserved-capacity ledger, so a
  pending-only reservation still blocks the final close.
- **Why.** Closing a wallet that still custodies an anchor would strand funds
  with no re-anchor path; finalizing closure under a live reserved capacity
  would leave a pending authorization without a signer.
- **Enforcing code.** Routing: `moveFunds` calls `beginWalletClosing` only when
  `mainUtxoHash == 0 && walletAnchoredInfo[w].count == 0`
  (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Wallets.sol:623-641`,
  gate at :629-631); the closing threshold on `notifyWalletCloseable` adds the
  proven-anchor amount (`:350-356`). Finalization:
  `notifyWalletClosingPeriodElapsed` hard-requires
  `walletReservationInfo[w].count == 0` before `finalizeWalletClosing`
  (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Wallets.sol:392-395`).
  `beginWalletClosing`/`finalizeWalletClosing` deliberately skip both checks
  (`Wallets.sol:649`, `:668`).
- **Test.** Routing both ways: `tbtc-v2@86ce2e79b:solidity/test/bridge/Reservation.test.ts:2105`
  (a proven anchor keeps the wallet in MovingFunds) and
  `tbtc-v2@86ce2e79b:solidity/test/bridge/Reservation.test.ts:2155`
  (pending-only capacity routes straight to Closing). The `Closing -> Closed`
  count gate has `none` dedicated.
- **Status.** holds at `tbtc-v2@86ce2e79b` for the routed cases; `holds-untested`
  for the `Closing -> Closed` count gate. The version recorded before VF-11
  named `walletReservationInfo` for the routing clause and did not match the published code (VF-11).

### INV-103 Termination permits an unresolved position; cleanup requires an explicit caller

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

- **Statement.** Termination may leave an `Active` or `ActionPending`
  reservation on the terminated wallet. This is a legal cleanup state.
  Permissionless stranding can release an `Active` position
  (`Terminated` and `Closed` always, `Live` past `dissolutionEligibleAt`, and
  `Closing` past both `dissolutionEligibleAt` and the closing period); a
  `MovingFunds` wallet alone is not enough. A pending re-anchor must first time
  out or settle.
  Eventual cleanup requires a functioning caller; the EVM does not schedule it.
- **Why.** Distinguishes available recovery from a guaranteed state transition.
- **Enforcing code.** `terminateWallet` (`Wallets.sol:690-701`, no count
  check); `notifyReservationStranded` (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1308-1340`,
  wallet-state require at :1328-1340); client watcher dispatches
  `Closed`/`Terminated` to the stranding check, `Closing` to the
  dissolution-and-closing-period variant, and `Live` to the
  dissolution-eligible variant
  (`keep-core@4ced1c839:pkg/maintainer/spv/reservation_stranding_watch.go:155-176`).
  The `Live` branch runs on the startup catch-up scan and on the action-timeout
  recheck queue, so a `Live` position outside both paths still needs an external
  caller.
- **Test.** `Bridge.ReservationStranding.test.ts` "strands an Active
  reservation on a wallet driven to Terminated by a real moving-funds-timeout
  call chain" (:554); `Bridge.ReservationOccupancy.test.ts` "
  notifyReservationStranded decrements counter on a Terminated wallet" (:143).
- **Status.** holds for the permitted transitions. Eventual cleanup is conditional
  on caller availability and is not an unconditional on-chain guarantee.

### INV-104 Normal wallet closing requires zero reservation count; termination permits unresolved positions

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

- **Statement.** The normal `Closing -> Closed` path requires zero
  `walletReservationInfo` count. `Terminated` may coexist with an `Active` or
  `ActionPending` position until an explicit cleanup or settlement transaction.
  Such a pairing is not an illegal storage state.
  Proof settlement strands on a **Closed or Terminated** target only: a
  `Closing` target keeps the position `Active` and keeps the owner's re-anchor
  right, which `requestReservationReanchor` still accepts from a Closing source.
- **Why.** A safety gate on closing does not imply automatic cleanup after termination.
- **Enforcing code.** Normal closing checks zero reserved count
  (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Wallets.sol:392-395`); the
  routing gate reads the proven-anchor ledger instead
  (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Wallets.sol:629-631`, INV-102).
  Termination has no such gate (`Wallets.sol:690`).
  Permissionless stranding requires an `Active` position
  (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/Reservation.sol:1315-1318`).
  The settlement hook strands on `Closed` or `Terminated` only
  (`tbtc-v2@86ce2e79b:solidity/contracts/bridge/ReservationProofs.sol:234-253`,
  state check at :243-246; calls at :608 and :772).
- **Test.** `Bridge.ReservationStrandingLibrary.test.ts` "strands an Active
  reservation on a Closed wallet" (:471) and "strands an Active reservation on
  a Terminated wallet" (:357); `tbtc-v2@86ce2e79b:solidity/test/bridge/ReservationProofs.test.ts:1308`
  (a Closed target strands on an on-time proof) and
  `tbtc-v2@86ce2e79b:solidity/test/bridge/ReservationProofs.test.ts:1434`
  (a Closing target is not stranded, state stays Active, no capacity released).
- **Status.** holds at `tbtc-v2@86ce2e79b` for the corrected statement. The
  version recorded before VF-10 listed Closing among the settlement-strand
  targets and did not match the published code (VF-10).

### INV-105 A Stranded position is terminal except for late-settlement reconstruction of a timed-out generation

- **Statement.** Once `state == Stranded`, the position cannot return to
  `Active` by any path except a *late* settlement of a previously timed-out
  generation whose anchor was the (since-released) stranded anchor. That path
  reconstructs the counters and re-strands if the wallet is still dead.
- **Why.** Prevents a permanently dead reservation from silently reviving, or
  a reconstruction from double-counting.
- **Enforcing code.** `strandReservation` sets `Stranded` and releases
  (`Reservation.sol:1091-1128`); `prepareReservationForSettlement` permits
  `(Stranded && late)` and restores counters (`ReservationProofs.sol:262-304`,
  state check :269-275, re-take :297-303); a late re-anchor settle then sets
  `Active` (`ReservationProofs.sol:821`).
- **Test.** `Bridge.ReservationOccupancy.test.ts` "prepareReservationForSettlement
  restores activeReservationsCount on late settlement of stranded reservation"
  (:238).
- **Status.** holds.

### INV-106 The Closed position state is never observable in M1

- **Statement.** `closeReservation` is the only writer of `state = Closed`
  and has zero call sites at the pin; therefore no `Closed` position is
  observable in M1. It is M2 settlement-only.
- **Why.** M1 is create-only; a `Closed` position would mean a redemption or
  dissolution occurred, neither of which exists in M1.
- **Enforcing code.** `closeReservation` (`Reservation.sol:1141-1165`) with
  the doc "Intended for milestone 2 settlement call sites (both currently
  unreachable in m1)" (:1134-1139). Grep of the pinned tree: the only
  reference is the definition; no router/bridge entry point calls it.
- **Test.** `none` (dead path, no test).
- **Status.** design-only (unreachable in M1).

### INV-107 An ActionPending position is always paired with a current Pending Reanchor generation

- **Statement.** `state == ActionPending` is observable only while that
  position's current generation is a `Pending` `Reanchor`. The `ActionPending`
  state and the pending re-anchor are created and cleared together.
- **Why.** A lone `ActionPending` with no pending re-anchor would hold the
  target's reserved capacity forever (a leak).
- **Enforcing code.** Set together: `requestReservationReanchor`
  (`Reservation.sol:906, 915`). Cleared together: `notifyReservationActionTimeout`
  requires `state == ActionPending` and sets `TimedOut` + `state = Active`
  (`Reservation.sol:951, 963, 981`); a settled re-anchor sets `Active`
  (`ReservationProofs.sol:734, 821`).
- **Test.** `Bridge.ReservationStrandingLibrary.test.ts` "releases target wallet
  capacity on Reanchor timeout" (:947) pins the `ActionPending -> Active` +
  `TimedOut` release.
- **Status.** holds.

### INV-108 A Pending Acceptance generation is only observable while the position is Unknown and no other action is Pending

- **Statement.** `action.state == Pending && actionType == Acceptance` implies
  the position is `Unknown` and there is no other `Pending` generation for the
  key. A second acceptance request reverts.
- **Why.** Prevents two competing acceptance authorizations for one deposit.
- **Enforcing code.** `requestReservationAcceptance` requires `state == Unknown`
  and current-action `!= Pending` (`Reservation.sol:531-539`).
- **Test.** `Reservation.test.ts` "should revert if acceptance is already
  pending" (:299).
- **Status.** holds.

### INV-109 Pending reserved deposit record: a reserved deposit that is neither accepted, stale nor released keeps `pendingReservedDeposits > 0`, which blocks a vault change

- **Statement.** While a revealed reserved deposit is not yet accepted,
  marked stale or (from `d20cf74ba`) released to the pool,
  `pendingReservedDeposits > 0`, and a change of `reservationVault` reverts.
  The record's `walletPubKeyHash` is the only source of truth for "pending".
- **Why.** Re-pointing the vault with unanchored reserved deposits would make
  them pool-sweepable while still minting through the old vault's callback.
- **Enforcing code.** Record + increment at reveal (`Deposit.sol:388-395`);
  vault-change gate `require(pendingReservedDeposits == 0)`
  (`Reservation.sol:1413-1414`); decrement on stale/force-stale
  (`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1264, 1312`) and on acceptance consumption
  (`ReservationProofs.sol:434`). At `d20cf74ba` the release exit also
  decrements it
  (`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1445`), and
  the vault-change gate sits at `:1550` (`pendingReservedDeposits == 0`).
- **Test.** The gate itself: `none` at the pin. At `d20cf74ba`,
  `Bridge.ReservationReleaseToPool.test.ts` "lets governance re-point the
  reservation vault once the last pending reserved deposit is released"
  (tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts:434) exercises the gate after a release. The counter's own conservation
  is pinned (see INV-206).
- **Status.** holds (gate, at `d20cf74ba` via tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts:434); holds-untested at the
  pin. The counter it reads is `holds` (INV-206).

### INV-110 Vault status is the only switch that routes a settlement credit to the vault vs. the depositor

- **Statement.** `isVaultTrusted[deposit.vault]` decides, at settlement credit
  time, whether the gross anchor is minted through the vault (Bank) or paid
  directly to the depositor. The credit always targets the deposit's immutable
  `vault`, never the live `reservationVault`.
- **Why.** A re-pointed/disabled live vault must not reroute an already-
  authorized deposit's credit.
- **Enforcing code.** `ReservationProofs.sol:599-631` (uses
  `self.deposits[reservationKey].vault`, gate at :621-624).
- **Test.** `ReservationProofs.test.ts` "should credit the deposit's immutable
  vault, not a reservationVault repointed mid-flight" (:1757).
- **Status.** holds.

### INV-111 Release to pool (D26): the record is cleared and the deposit re-routed; refused after any acceptance request and for the reservation vault

- **Scope.** Exists only at `d20cf74ba`. Entry point:
  `tbtc-v2@d20cf74ba:solidity/contracts/bridge/ReservationRouter.sol:433-437`
  (router, delegates to the library; no modifier, the depositor check is in
  the library). Event `ReservedDepositReleasedToPool`:
  `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:423`, `tbtc-v2@d20cf74ba:solidity/contracts/bridge/ReservationRouter.sol:166`.
- **Statement (preconditions, all in
  `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1373-1451`).**
  The call succeeds only if all hold:
  1. the deposit is revealed (:1379) and not swept (:1380);
  2. `msg.sender == deposit.depositor` (:1381-1384);
  3. `pendingReservedDeposit[k].isReserved` (:1388-1391) and
     `walletPubKeyHash != 0` (:1392-1395), so a stale-marked or already
     released record is rejected;
  4. `reservations[k].requestNonce == 0` (:1404-1407): **no acceptance
     request was ever made.** `requestNonce` is bumped by every acceptance
     and re-anchor request and never reset (`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:623, 907` at the
     pin), so a pending, timed-out, superseded or settled acceptance all keep
     it non-zero and the release reverts with "Acceptance was already
     requested". A settled acceptance is also refused by (1) and (3);
  5. the refund window still has the 24-hour margin: `refundDeadline > 24h`
     and `block.timestamp < refundDeadline - 24h`, strict, equality refused
     (tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1416-1426, `DEPOSIT_REFUND_SAFETY_MARGIN` from
     `WalletProposalValidatorConstants.sol:13`);
  6. `vault == address(0)` or (`isVaultTrusted[vault]` and
     `vault != reservationVault`) (tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1428-1433): **the reservation vault is
     refused**, as is any untrusted vault.
- **Effects (exhaustive, tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1441-1446).** `isReserved = false`,
  `walletPubKeyHash = 0`, `refundDeadline = 0`,
  `refundDeadlineValidated = false` (all four record fields cleared);
  `pendingReservedDeposits -= 1`; `deposits[k].vault = vault`. Nothing else
  is written. `sweptAt`, `treasuryFee`, the position, the action records,
  `reservationsByAnchorUtxo` and `spentMainUTXOs` are untouched.
- **Counter effects.**

  | Counter | Change on release | Why |
  |---|---|---|
  | `pendingReservedDeposits` | `-1` (tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1445) | the only counter the deposit contributes to |
  | `activeReservationsCount` | none | slot reserved only by an acceptance request (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:621` at the pin), which bumps `requestNonce` (tbtc-v2:solidity/contracts/bridge/Reservation.sol:623) |
  | `reservationTotalAmount` | none | same: reserved only at acceptance request |
  | `walletReservationInfo[w].count/.amount` | none | same |
  | `pendingThirdPartyReanchorHolds[w]` | none | a hold needs an `Active` position (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:753-756` at the pin); `requestNonce == 0` means the position is `Unknown` |
  | `inKindFeeDebtSat`, `cumulativeReanchorFee`, `mintedAmount` | none | no settlement ever occurred |

- **Conservation equation including release.**
  `pendingReservedDeposits ==` #{`k`: `pendingReservedDeposit[k].walletPubKeyHash != 0`}.
  Increment: reveal (`Deposit.sol:395`). Decrements: consume
  (`ReservationProofs.sol:434`), stale (`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1281`),
  force-stale (`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1329`), release (`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1445`). Each decrement is paired with
  clearing `walletPubKeyHash`, so exactly one fires per key. A deposit with
  `requestNonce == 0` contributes to no other counter, so release preserves
  every equation in INV-201 to INV-205 and INV-207 to INV-209 unchanged.
- **Post-release states (must hold).** The record reads all-zero, so:
  `isReservedDeposit(k)` is false (`Bridge.sol:1629-1635`); an acceptance
  request reverts at the `isReserved` require
  (`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:534`); stale and force-stale revert with "Not a
  pending reserved deposit"; `DepositSweep` no longer rejects it as reserved
  (`DepositSweep.sol:433-446`) and requires `deposit.vault == sweep vault`
  (so a sweep naming the reservation vault reverts with "Deposit should be
  routed to another vault").
- **Why.** Lets a depositor who never asked for custody recover the deposit
  into the pool before the refund window closes, without letting a deposit
  whose anchor a wallet may already have signed (`requestNonce != 0`) be
  swept, and without routing pooled credit into the reservation vault, whose
  `receiveBalanceIncrease` mints on any Bank credit (F-1).
- **Test (all `tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts`).**
  - rule 4: "reverts after a pending acceptance request" (:501), "reverts
    after a timed-out acceptance request" (:515);
  - rule 6: "reverts when the release vault is the reservation vault" (:532),
    "reverts when the release vault is not trusted" (:543), "cannot be swept
    with the vault parameter set to the reservation vault after a release"
    (:590);
  - rule 3: "reverts when the deposit was not revealed as reserved" (:485),
    "reverts when the deposit is marked stale" (:555);
  - rule 2: "reverts when the caller is not the deposit's depositor" (:475);
  - rule 5: "reverts when the refund margin for an ordinary sweep is
    exhausted" (:569);
  - effects: "releases to a trusted vault, an ordinary sweep proof succeeds
    and credits the vault through the Bank" (:350) asserts
    `pendingReservedDeposits` 1 -> 0, `isReservedDeposit` false and
    `reservedDepositWallet` zero (:357-371); "releases to the zero address
    and an ordinary sweep credits the depositor Bank balance" (:407);
    "leaves no acceptance request or stale notification possible after a
    release" (:609).
  - **No test:** rule 1 (revealed, not swept) has no direct test (the
    "Deposit already swept" revert at :404 is the ordinary sweep, not the
    release guard); and no test asserts that `activeReservationsCount`,
    `reservationTotalAmount`, `walletReservationInfo` or
    `pendingThirdPartyReanchorHolds` are unchanged by a release.
- **Status.** holds at `d20cf74ba` for rules 2 to 6 and the effects;
  holds-untested for rule 1 and for the "no other counter changes" claim
  (true by construction, see the counter table). Not present at the pin.
  The refusal of the reservation vault is evaluated at release time only
  (see F-5).

---

### INV-112 Legal state relationships at transaction boundaries

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

| Position and current action | Classification | Reason |
|---|---|---|
| Unknown with no action, or a TimedOut acceptance | Legal | No accepted position exists; a fresh request may be possible |
| Unknown with Pending Acceptance | Legal | Acceptance reserves capacity before creating the Active position |
| ActionPending with current Pending Reanchor | Legal | Source custody and target capacity coexist until settlement or timeout |
| Active with no current Pending action | Legal | The current action can be Settled, TimedOut or Superseded, depending on lineage |
| Active on Terminated or Closed | Legal unresolved cleanup state | Wallet transition need not strand the position in the same transaction; a later settlement strands it (INV-104) |
| Active on a dissolution-eligible Live wallet | Legal unresolved cleanup state | `notifyReservationStranded` admits it once `dissolutionEligibleAt` has passed (SM-R6); a Live anchor can also be rotated by governance |
| Active on a Closing wallet | Legal unresolved cleanup state | Strandable only once `dissolutionEligibleAt` and the closing period have both passed; until then it keeps the re-anchor right |
| Active on Closing after a settlement | Legal, and deliberately not stranded | Settlement strands only Closed and Terminated targets (INV-104), so a freshly settled position keeps its re-anchor window |
| Stranded with a previously TimedOut re-anchor | Legal | A qualifying late proof can reconstruct bookkeeping |
| ActionPending with no current Pending Reanchor | Illegal under M1 transition rules | Request, timeout and proof update the position/action pair together |
| Active with a current Pending Reanchor | Illegal at a successful transaction boundary | The request must set ActionPending |
| Unknown with a current Pending Reanchor | Illegal under M1 transition rules | Re-anchor request requires an accepted Active position |
| Closed position state | Unreachable in M1 | Its writer has no M1 caller |

- **Enforcing code.** `Reservation.sol:499-650`, `:742-933`, `:934-993`;
  `ReservationProofs.sol:262-304`, `:473-626`, `:640-821`;
  wallet closing/termination rules in `Wallets.sol:363-389`, `:649-713`.
- **Test.** The individual transition tests are listed in INV-101 through INV-111.
  `none` identified for the complete state-relation table.
- **Status.** holds-untested for exhaustive state-machine coverage.

### INV-113 D31 acceptance reserve is a request-time count bound

- **Statement.** Let L be `liveWalletsCount`, R be `handoffReserveWallets`, N be `maxReservationsPerWallet`, and G be `activeReservationsCount` (called P in SM-C15; it equals count(P) + count(A) in INV-201). At `tbtc-v2@806b1106f` a new acceptance request must pass the global cap first, then `G + 1 <= max(0, L - R) * N`, where G includes the pending acceptances and the request being made. While L <= R every acceptance reverts. The caps setter requires `R >= 1` whenever it runs. The bound is evaluated at the acceptance request only. It is not checked at proof time, at re-anchor or on settlement, and it does not promise permanent headroom after later wallet transitions. At `9d2086965` there is no reserve value and no such gate.
- **Enforcing code (all at `tbtc-v2@806b1106f`).** `reserveAcceptanceCapacity` checks the global cap ("Active reservations cap exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1059`), increments the counter (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1061`), computes `remaining = L > R ? L - R : 0` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1073`) and requires `activeAfterThis <= remaining * maxReservationsPerWallet` ("Hand-off reserve exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`). Its only caller is `requestReservationAcceptance` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:643`). Storage `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeState.sol:509` (packed into the slot of `maxThirdPartyReanchorHoldsPerWallet`, no slot shift). The setter rejects zero ("Hand-off reserve must be greater than zero" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`) and stores the value (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1736`). View `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:519`. The SDK preflight mirrors the rule at `tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:707-722`, called from `tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:255-262` and `tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:567-574`.
- **Limits.** A count-admission bound only. It does not prove a Live target, a signing quorum, proof delivery, amount headroom or fee room. Existing accepted positions stay real if L later falls to R or below; only new acceptances are refused. The SDK mirror returns false when N is 0, while the chain would reject every acceptance (the parameters setter requires N > 0). On an upgraded Bridge R reads 0 until the first five-argument caps update; the setter enforces `R >= 1` only when it runs, and the deploy scripts set it before activation (`tbtc-v2@806b1106f:solidity/deploy/97_set_reservation_parameters.ts:141`).
- **Test (all at `tbtc-v2@806b1106f`).** The describe block "hand-off reserve gate at acceptance" `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:276`: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322` reverts past (L - R) * N while max-active has room; `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:346` tightens when a Live wallet leaves Live; `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379` blocks every acceptance while L <= R. Zero reserve rejected: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationCaps.test.ts:163`. Five-argument staging: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4705` (begin), `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.Governance.test.ts:4789` (finalize). Selectors and event topic: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts:36`, `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationAbiSnapshot.test.ts:57`. SDK: `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:414` (describe "hand-off reserve gate"), pre-funding `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:434`, preflight `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:542`. Deploy script rejects a zero reserve: `tbtc-v2@806b1106f:solidity/test/deploy/98_generate_reservation_mainnet_calldata.test.ts:486`.
- **Status.** holds at `tbtc-v2@806b1106f`. The gate does not exist at the original pin.

### INV-114 D31 hand-off exemption preserves bookkeeping and the remaining guards

- **Statement.** At `tbtc-v2@806b1106f` a privileged request from a MovingFunds or Closing source may exceed the target's ordinary count and amount caps. Privileged means the router's governance flag, the reservation owner, an SPV maintainer, or a caller with a non-empty member proof. The request still adds the target count and amount on every path and releases source accounting only on settlement (the source keeps its capacity until then). An invalid member proof reverts. The target must be Live and differ from the source. Cooldown is bypassed only for the governance flag. A Live source can be rotated only by the governance flag, and that rotation keeps both target caps. A third-party request keeps both caps and the hold cap. At `9d2086965` every caller was checked against both target caps.
- **Enforcing code (all at `tbtc-v2@806b1106f`).** Router passes `msg.sender == governance` as `privileged` (`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:246-259`). Cooldown bypass only for that flag ("Reanchor cooldown in effect" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:789`). A non-empty proof is validated and failure reverts ("Caller is not a member of the source wallet" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:811`). `isPrivileged` is built at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:814-818`. A Live source needs the governance flag ("Only governance can rotate a Live wallet's anchor" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:827`). The target must differ and be Live ("Target wallet must differ from the source wallet" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:840`; "Target wallet must be in Live state" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:845`). Hold cap for non-privileged callers ("Target third-party re-anchor holds exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:901`). Exemption: `skipTargetCaps = isPrivileged && sourceState != Live` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`); the count check is guarded at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:928-933` ("Wallet reservations cap exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:931`) and the amount check at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:942-948` ("Wallet reserved amount cap exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:946`); the count and amount are written on every path (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:934-935`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:949-950`). Signing validator: the capacity check is not repeated at signing time (doc block `tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1092-1098`; Live target check "Target wallet must be in Live state" `tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1159`); at `9d2086965` the validator re-checked both caps (VF-01). Client: member proof gathered for a fresh request (`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:262-290`); the target search prefers a candidate with headroom and falls back to the least-loaded Live wallet only for a privileged member request (`keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:1153-1193`, `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:1195-1241`).
- **Limits.** The exemption removes the target-cap obstacle only. It does not create a Live wallet or a signing quorum, and it does not remove fee limits, cooldown, proof deadlines or chain availability. Hand-offs above the cap concentrate anchors on one wallet. A third-party hold still occupies the target's ordinary slot for third-party requests; it no longer blocks a privileged hand-off from a retiring source. Live-source rotation stays governance-only and keeps both caps. The cooldown bypass has no D31 test of its own; it is the pre-D31 guard.
- **Test (all at `tbtc-v2@806b1106f` unless the path says `keep-core@d40f6aadc`).** `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:359` (describe "privileged hand-off: a retiring source may exceed the target's caps"): owner from MovingFunds to a full target `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:427` (target count becomes cap + 1); owner from Closing `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:435`; SPV maintainer `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:443`; proven member `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:452`; third party into a full target still reverts `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`; governance from a Live source into a full target still reverts `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:485`; privileged above the amount cap `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:494`, third party above the amount cap reverts `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:502`; privileged skips the ordinary cap while a third-party hold occupies the slot `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:553`, member succeeds after the hold times out `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:565`; invalid member proof reverts `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:235`. Validator with a mocked bridge: `tbtc-v2@806b1106f:solidity/test/bridge/WalletProposalValidator.test.ts:4322-4382` (above-cap target passes). Real validator harness: `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1014` (count above the cap accepted `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1205`, amount above the cap accepted `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1219`). Client fallback: `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1479` (member falls back to the least-loaded full wallet), `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1611` (a non-member still sends nothing when every target is full). Integration runs are recorded in [approved deltas](13-approved-design-deltas.md).
- **Status.** holds at `tbtc-v2@806b1106f` and `keep-core@d40f6aadc`. The behavior does not exist at the original pins.

### INV-115 D28 failed observations do not publish fresh risk data

- **Statement.** If any required keeper observation read fails, the observer does not publish new risk gauges or advance its observation timestamp. A healthy endpoint alone does not prove a completed observation.
- **Enforcing code.** Required scans, action and wallet reads precede publication (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:98-190`). The four risk values and chain-head timestamp are published only on the successful path (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:346-373`).
- **Limits.** This is a published-data rule, not an atomic snapshot of every chain read or a rollback of incremental scan cursors. Wallet states can remain cached for ten minutes. The monitor must check freshness; numeric zeros alone are insufficient.
- **Test.** Failed action reads preserve the published timestamp and values (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:591-648`). Source and deposit read failures also fail the pass (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer_test.go:653-744`).
- **Status.** holds at `keep-core@9c4e1b221`; live deployment is not asserted.

## Detailed invariant groups

- [Accounting arithmetic and counter mutation sites](02-accounting-invariants.md): INV-201 through INV-211.
- [Bitcoin backing and authorization](02-backing-authority-invariants.md): INV-301 through INV-304 and INV-401 through INV-402.
- [Generation, timing and cross-contract rules](02-generation-time-invariants.md): INV-501 through INV-503, INV-601 through INV-603 and INV-701 through INV-705.

## Findings

The ledger distinguishes source observations from confirmed defects.
Counter mutation sites and equations above remain review obligations.
Checked arithmetic prevents wrapping; it does not prove every counter is balanced.

| Earlier ID | Retained observation | Disposition |
|---|---|---|
| F-1 | The vault does not bind Bank credit to a reservation key | VF-02 confirms a routing/provenance gap, not arbitrary unbacked minting |
| F-2 | Closed/Vetoed position paths are unreachable in M1 | Deliberate scope and storage compatibility, not a new defect |
| F-3 | Confirmed late settlement can exceed current governance caps | Explicit recovery accounting; caps are not universal conservation bounds |
| F-4 | A test header incorrectly describes the Closing path | Doc drift; `notifyWalletFundsMoved` enters Closing without a count gate |
| F-5 | D26 excludes the configured reservation vault only at release time | VF-09 confirms the point-in-time limit; released state does not become reserved again |

See [finding adjudication](12-finding-adjudication.md) for attempted refutations
and source evidence. No runtime arithmetic proof is claimed by this chapter.

## Doc drift

Rows where a doc or comment disagrees with the code at the commit named in the row (the pin unless a row cites `d20cf74ba`).

| Doc `path:line` | What it says | What the code does |
|---|---|---|
| `requirements.md:553` (FR-31) | `releaseReservedDepositToPool(depositKey, vault)` MUST let the depositor release an un-requested reserved deposit to the pool; revert unless the caller is the depositor, the record is pending, `requestNonce` is zero, the refund margin holds and the vault is the zero address or a trusted non-reservation vault. | Matches the code at `d20cf74ba`: `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1373-1451` (guards :1379-1433, effects :1441-1446), router `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:433-437`. Not in the pinned commit `9d2086965` (0 matches). No drift against `d20cf74ba`; the doc is ahead of the pin only. |
| `architecture.md:600` | Lists `releaseReservedDepositToPool` as a milestone-1 pending-deposit transition: record fully cleared, deposit re-routed to the chosen vault or the zero address, normal sweep can settle it. | Matches `d20cf74ba` (all four record fields cleared, tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1441-1444; `deposits[k].vault` rewritten, tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1446). Absent at the pin. |
| `m1-b-implementation.md:306-308` | "B deletes its only reader - re-anchor's `< dissolutionEligibleAt` gate". | At the pin the field's *sole on-chain reader* is retained: the `Closing` branch of `notifyReservationStranded` reads `block.timestamp >= dissolutionEligibleAt` (`Reservation.sol:1207-1212`, field comment `:198-213`). The doc's "reader removed" is stale relative to the pinned code. |
| `test/bridge/Bridge.ReservationStranding.test.ts:16-18` (header comment) | `Closing`/`Closed` unreachable for an Active reservation because `beginWalletClosing`/`finalizeWalletClosing` unconditionally require zero count. | The zero-count gate is in `moveFunds` (`Wallets.sol:618`) and `notifyWalletClosingPeriodElapsed` (`Wallets.sol:385`); `beginWalletClosing`/`finalizeWalletClosing` skip it, and `terminateWallet` (`Wallets.sol:690`) is ungated so a Terminated wallet can hold an Active reservation. |
| `m1-b-implementation.md` (stranding-watcher row, text before keep-core #4365) | The keep-core stranding watcher "triggers only on `StateClosed`/`StateTerminated`; no client path triggers the Closing-eligible branch - permissionless `notifyReservationStranded` is the backstop." | At the pinned keep-core (`f99ef96e3`) the watcher *does* handle `Closing`: `checkReservationStrandingForClosingWallet` (`keep-core:pkg/maintainer/spv/reservation_stranding_watch.go:139-147`, dispatch at `:148-162`) strands a Closing wallet's reservation once the chain head reaches `DissolutionEligibleAt`. The "no client path triggers it" claim is stale. |
| `tbtc-v2@d20cf74ba:solidity/contracts/bridge/BridgeState.sol:33` (struct doc) | "`isReserved` is permanent; the remaining fields ... may be cleared after acceptance or staleness." | `releaseReservedDepositToPool` clears `isReserved` too (`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1441`). The field-level comment at `tbtc-v2@d20cf74ba:solidity/contracts/bridge/BridgeState.sol:37-45` was updated; this struct doc and the mapping comment at `tbtc-v2@d20cf74ba:solidity/contracts/bridge/BridgeState.sol:474-477` ("permanent classification ... making a reserved deposit eligible for an ordinary sweep") were not. |
| `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1257-1258` (comment in `notifyStaleReservedDeposit`; same text in `forceStaleReservedDeposit`) | "no path clears `isReserved` once reveal sets it." | The release path clears it (`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1441`). The `assert(pendingDeposit.isReserved)` at tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1261 and tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1314 cannot fire: the `walletPubKeyHash != 0` require runs first and release zeroes that field. The comment is wrong, the assert is safe. |
