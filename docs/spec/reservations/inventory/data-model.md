# Data model and governance surface inventory

Source-verified on `solidity/contracts/bridge/Reservation.sol` at the
`feat/utxo-reservation-guards` tip
(1507 lines, the `#1094` guards tip). See `pr-map.md` §3: this tip predates the
`#1102` fold, which changed `Reservation.sol` by +342 -95.

## Status vs M1 code (2026-09-28)

This fragment is pinned to the `feat/utxo-reservation-guards` tip (`#1094`,
`bcfed23f`) and its findings below (esp. §7's "no validation at all") are
correct at that pin. The actual M1 code (`reservations-upgrade` @
`9f8f5ef1`) diverges:

- **Governance setters now validate.** `updateReservationParameters` (M1
  `Reservation.sol:1260-1354`) requires `maxReservationsPerWallet > 0` and
  calls a new `validateReservationCapsInvariant` helper requiring
  `reservationMaxTotalAmount <= maxActiveReservations *
  reservationMaxSingleAmount` (skipped only when either operand is
  zero/disabled); `updateReservationCaps` (`:1376-1406`) requires
  `maxActiveReservations > 0` and calls the same invariant helper. §7's "not
  a single `require`" / "unconstrained by the contract" claims are resolved
  in the M1 code — the only cap-shaped parameter still unconstrained by
  any `require` at M1 tip is `maxReservationsAmountPerWallet` (the per-wallet
  amount cap).
- **Storage repacking and dropped fields.** M1's `BridgeState.Storage` packs
  the per-wallet count/amount into a single `walletReservationInfo` struct
  and drops `walletReservationKeys`, `walletReservationKeyIndex`,
  `maxCumulativeReanchorFee`, `reservationDissolutionTxMaxFee`,
  `walletPendingDissolution`, `reservationRetryCreditActionNonce`, and
  `reservationMaxBackingFractionBps` as unused/dead (`__gap` shrinks to 9 of
  48 reservation-eligible slots consumed, down from 15). None of this is
  named in §4/§6/§7 below, which describe the guards-tip layout.
- `ReservationRequest` and `ReservationAction` also gained fields not listed
  in §4/§6's tables: `cumulativeReanchorFee`, `reanchorCooldownUntil`,
  `retryCreditSourceNonce`, `isPartial`, `termSeconds`, and a dissolution-
  delay snapshot field — see the notes after each table.

## 1. The storage-completeness rule needs a sharper edge

`m1-b-implementation.md` §4.4/§4.5 states the rule as "storage-complete means
written, not merely declared". Field-by-field reader analysis shows that rule is
right but **too blunt**, and applying it literally would create make-work.

The distinction that matters is **which path writes the field**:

- A field written by an **m1-reachable** path (acceptance, re-anchor, timeout)
  will be populated on m1-era records. If m2 reads it, m1 must keep writing it.
  Dropping the write is a silent repudiation.
- A field written **only by an m2-exclusive** path (a redemption request, a
  renewal) is never populated on any m1-era record, because m1 never creates
  that kind of action. It only needs to **exist** in the layout. There is nothing
  to lose by not writing it.

Both categories must be declared. Only the first must be written. The docs
currently conflate them, which overstates the m1 obligation.

### Worked examples

| Field | Written by | m1 obligation |
|---|---|---|
| `dissolutionEligibleAt` | `settleAcceptance` (`ReservationProofs.sol:537`) - **m1-reachable** | **Must keep writing.** Every m1 position gets one, and m2's dissolution needs it as the eligibility date. This is the real trap. |
| `expiresAt` | `settleAcceptance` (`ReservationProofs.sol:533`) - **m1-reachable** | **Must keep writing.** Same reasoning. |
| `watchtowerDefaultDelay`, `watchtowerLevelOneDelay`, `watchtowerLevelTwoDelay` | only `requestReservedRedemption` (`Reservation.sol:725-727`) - **m2-exclusive** | **Declare only.** No m1 action record is a redemption, so nothing is lost. |
| `redeemer`, `feePaid`, `usedRetryCredit` | `requestReservedRedemption` (`:716-718`) - **m2-exclusive** write, but **read by an m1 path** (see §2) | **Declare, and keep the readers.** |

## 2. Finding: the timeout path m1 keeps reads redemption fields

`notifyReservationActionTimeout` is m1 work (it is the required cleanup and the
slashing path). It reads fields that only redemption ever writes:

| Site | Code | Consequence |
|---|---|---|
| `Reservation.sol:1009` | `if (action.feePaid) {` | Refund branch |
| `Reservation.sol:1022` | `self.bank.transferBalance(action.redeemer, action.amount)` | Refunds the redeemer |
| `Reservation.sol:1010` | `reservation.retryCredit = true` | Mints retry credit |

And the shared internal helper `unwindPendingAction`
(`ReservationProofs.sol:1262-1270`) reads `pendingAction.usedRetryCredit` and
`pendingAction.redeemer`, and writes `reservation.retryCredit`.

So `retryCredit` is **not** a redemption-only field: its only *writer* other than
redemption is the m1 timeout path, which sets it to `true` on any timed-out
fee-paid action. In m1 no action is fee-paid, so the branch is unreachable, but
the helper is shared and stays.

**Implication for the rewrite:** `unwindPendingAction` must be extracted intact,
including its retry-credit and redeemer handling, even though m1 can never take
those branches. Stripping them makes the helper diverge from what m2 needs and
breaks the "m2 is an upgrade, not a migration" property.

**DECISION NEEDED:** does m1 keep the unreachable refund branch at
`Reservation.sol:1009-1022`, or delete it and re-add in m2? Deleting is safe for
storage (fields stay), and this is Bridge code so it is replaceable. Keeping it
costs a few lines and keeps the diff to m2 smaller. Recommend keeping, but it is
a real choice.

## 3. Enums

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `ReservationState.Unknown` | `Reservation.sol:82` | #1088 | storage | yes | yes | Zero value |
| `ReservationState.Active` | `:86` | #1088 | storage | yes | yes | |
| `ReservationState.ActionPending` | `:91` | #1091 | storage | yes | yes | Two-phase machine |
| `ReservationState.Closed` | `:95` | #1088 | storage | **declare only** | yes | Reached only by redemption or dissolution settlement, both m2 |
| `ReservationState.Stranded` | `:100` | #1094 | storage | yes | yes | m1's only terminal state |
| `ActionType.None` | `:105` | #1091 | storage | yes | yes | Zero value |
| `ActionType.Acceptance` | `:106` | #1091 | storage | yes | yes | |
| `ActionType.Redemption` | `:107` | #1091 | storage | **declare only** | yes | Never constructed in m1 |
| `ActionType.Reanchor` | `:108` | #1091 | storage | yes | yes | |
| `ActionType.Dissolution` | `:109` | #1091 | storage | **declare only** | yes | Never constructed in m1 |
| `ActionState.Unknown` | `:115` | #1091 | storage | yes | yes | Zero value |
| `ActionState.Pending` | `:120` | #1091 | storage | yes | yes | |
| `ActionState.Settled` | `:122` | #1091 | storage | yes | yes | |
| `ActionState.TimedOut` | `:128` | #1091 | storage | yes | yes | |
| `ActionState.Vetoed` | `:132` | #1091 | storage | **declare only** | yes | Veto is redemption-only |
| `ActionState.Superseded` | `:136` | #1091 | storage | yes | yes | Reachable via conflicting re-anchor |

**Enum variants must keep their numeric positions.** Removing `Redemption` from
`ActionType` in m1 would renumber `Reanchor` and `Dissolution`, silently
reinterpreting every stored action record when m2 restores the variant. The enums
must be extracted verbatim, not pruned. This is a subtle layout hazard the
existing docs do not name.

## 4. `ReservationRequest` - one row per field

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `owner` | `Reservation.sol:143` | #1088 | storage | write+read | yes | |
| `mintedAmount` | `:147` | #1093 | storage | write+read | yes | Backing invariant; also read by the vault (6 sites) |
| `acceptedAt` | `:150` | #1088 | storage | write+read | yes | |
| `walletPubKeyHash` | `:153` | #1088 | storage | write+read | yes | Custodian; re-anchor rewrites it |
| `anchorAmount` | `:157` | #1093 | storage | write+read | yes | Claim-equals-anchor; 21 read sites |
| `expiresAt` | `:161` | #1092 | storage | **write, no m1 reader** | yes | Written by acceptance; every m1 reader is in a deleted function |
| `anchorTxHash` | `:163` | #1088 | storage | write+read | yes | |
| `anchorTxOutputIndex` | `:166` | #1088 | storage | write+read | yes | |
| `state` | `:168` | #1088 | storage | write+read | yes | |
| `requestNonce` | `:173` | #1091 | storage | write+read | yes | Two-phase anti-replay; 16 sites |
| `retryCredit` | `:180` | #1091 | storage | write via timeout | yes | See §2 |
| `dissolutionEligibleAt` | `:187` | #1092 | storage | **write, no m1 reader** | yes | The named trap; see §2 and §5 |

**Not exhaustive past this pin.** M1 code adds `cumulativeReanchorFee` and
`reanchorCooldownUntil` to `ReservationRequest` (~`Reservation.sol:225-236`
at M1 tip) — neither existed at the guards tip and neither is listed above.

## 5. Confirmed: in m1 the custody term has no on-chain reader at all

Every reader of `expiresAt` and `dissolutionEligibleAt`, enumerated with its
enclosing function:

| Field | Read at | Enclosing function | m1? |
|---|---|---|---|
| `expiresAt` | `Reservation.sol:667` | `requestReservedRedemption` | deleted |
| `expiresAt` | `:1165`, `:1195` | `extendReservation` | deleted |
| `dissolutionEligibleAt` | `:786` | `requestReservationReanchor` | **gate deleted to make re-anchor unbounded** |
| `dissolutionEligibleAt` | `:900` | `requestReservationDissolution` | deleted |
| `dissolutionEligibleAt` | `:1196` | `extendReservation` | deleted |

Both fields are written by `settleAcceptance` (`ReservationProofs.sol:533`,
`:537`) and read by nothing m1 retains. This confirms
`m1-b-implementation.md` §4.4's claim exactly, and it is the strongest single
argument in the doc set: the term is a **commitment held in storage for m2 to
honour**, enforced by nothing in m1.

## 6. `ReservationAction` - one row per field

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `targetWalletPubKeyHash` | `Reservation.sol:202` | #1091 | storage | write+read | yes | Re-anchor destination |
| `requestedAt` | `:204` | #1091 | storage | write+read | yes | |
| `timeoutAt` | `:206` | #1091 | storage | write+read | yes | 5 read sites |
| `txMaxFee` | `:208` | #1091 | storage | write+read | yes | 16 sites |
| `actionType` | `:210` | #1091 | storage | write+read | yes | |
| `state` | `:212` | #1091 | storage | write+read | yes | |
| `feePaid` | `:216` | #1091 | storage | declare; read by timeout | yes | §2 |
| `redeemer` | `:219` | #1091 | storage | declare; read by timeout | yes | §2; also 4 sites in `Redemption.sol` |
| `amount` | `:223` | #1091 | storage | write+read | yes | |
| `actionDataHash` | `:229` | #1091 | storage | write+read | yes | Snapshot-at-request digest; 11 sites |
| `sourceAnchorUtxoHash` | `:233` | #1091 | storage | write+read | yes | Re-anchor source binding |
| `usedRetryCredit` | `:237` | #1091 | storage | declare; read by helper | yes | §2 |
| `watchtowerDefaultDelay` | `:242` | #1091 | storage | **declare only** | yes | Only writer and reader is `requestReservedRedemption:725` |
| `watchtowerLevelOneDelay` | `:245` | #1091 | storage | **declare only** | yes | Same, `:726` |
| `watchtowerLevelTwoDelay` | `:248` | #1091 | storage | **declare only** | yes | Same, `:727` |

**Not exhaustive past this pin.** M1 code adds `retryCreditSourceNonce`,
`isPartial`, `termSeconds`, and a dissolution-delay snapshot field to
`ReservationAction` (~`Reservation.sol:305-330` at M1 tip) — none existed at
the guards tip and none are listed above.

## 7. Governance parameters and validation

`updateReservationParameters` (`Reservation.sol:1227-1295`) validates:

| Check | Source | Kind |
|---|---|---|
| `reservationTxMaxFee > 0` | `:1239-1242` | absolute |
| `reservationMinAmount > reservationTxMaxFee` | `:1243-1246` | **relational** |
| `MIN_RESERVATION_TERM <= reservationTermSeconds <= MAX_RESERVATION_TERM` | `:1247-1251` | absolute bounds |
| `0 < reservationRenewalWindowSeconds < reservationTermSeconds` | `:1252-1256` | **relational** |
| `reservationActionTimeout > REQUEST_TIMEOUT_SAFETY_MARGIN` | `:1257-1261` | relational to a constant |
| vault change requires `reservationTotalAmount == 0` and `pendingReservedDeposits == 0` | `:1263-1274` | **quiescence gate** |

### Two parameters have no validation at all

`reservationMaxTotalAmount` (`:1280`) and `maxReservationsPerWallet` (`:1281`)
are assigned with **no `require` of any kind**. Nothing bounds either.

### `updateReservationCaps` validates nothing whatsoever

`updateReservationCaps` (`:1390-1401`) assigns
`maxReservationsAmountPerWallet` (`:1395`) and `reservationMaxSingleAmount`
(`:1396`), emits (`:1398`), and returns. **There is not a single `require` in the
function.** No bound, no relational check against
`reservationMaxTotalAmount`, no ordering check between the per-wallet and
single-position caps.

This confirms and extends `m1-b-implementation.md` §4.1's concern. The doc says
"nothing on-chain today prevents raising `reservationMaxTotalAmount` past slot
capacity". The stronger statement is: **neither cap setter validates anything at
all**, so all four cap-shaped parameters
(`reservationMaxTotalAmount`, `maxReservationsPerWallet`,
`maxReservationsAmountPerWallet`, `reservationMaxSingleAmount`) are unconstrained
by the contract and mutually unrelated.

**Historical at this pin; resolved in M1 code.** The guards tip genuinely has
zero relational `require`s here. The M1 code (`reservations-upgrade`
@ `9f8f5ef1`) closed this gap: `updateReservationParameters` now requires
`maxReservationsPerWallet > 0` (M1 `Reservation.sol:1295-1298`) and
cross-checks `reservationMaxTotalAmount` via a new
`validateReservationCapsInvariant` helper (`:1330-1334`); `updateReservationCaps`
now requires `maxActiveReservations > 0` (`:1382-1385`) and calls the same
invariant (`:1392-1396`), which enforces `reservationMaxTotalAmount <=
maxActiveReservations * reservationMaxSingleAmount` (skipped only when either
operand is zero/disabled). This resolves Open Question #2 below. The two
subsections above should be read as narrowed to the one cap-shaped parameter
still unconstrained by any `require` at M1 tip:
`maxReservationsAmountPerWallet` (the per-wallet amount cap) — not all four.

### Retroactivity

Parameters are stored on `BridgeState.Storage` and read live, **not** snapshotted
per position, except where an action record snapshots a derived value at request
time. So a governance raise applies to existing wallets immediately. The
exceptions are the per-position snapshots: `expiresAt` and
`dissolutionEligibleAt` are computed once at acceptance
(`ReservationProofs.sol:533`, `:537`) and never recomputed, which is what makes
governance changes non-retroactive for already-granted terms.

## 8. Events

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `ReservationAcceptanceRequested` | `:251` | #1091 | yes | yes | |
| `ReservationExtended` | `:260` | #1092 | no | yes | Renewal |
| `ReservedRedemptionRequested` | `:267` | #1091 | no | yes | |
| `ReservationReanchorRequested` | `:277` | #1091 | yes | yes | |
| `ReservationDissolutionRequested` | `:285` | #1091 | no | yes | |
| `ReservationActionTimedOut` | `:293` | #1091 | yes | yes | |
| `ReservedRedemptionVetoed` | `:299` | #1091 | no | yes | |
| `ReservationRetryCreditMinted` | `:304` | #1091 | yes | yes | Emitted by the m1 timeout path |
| `ReservedDepositMarkedStale` | `:306` | #1094 | yes | yes | |
| `ReservationStranded` | `:308` | #1094 | yes | yes | m1's only close event; carries `anchorAmount` |
| `ReservationParametersUpdated` | `:315` | #1088 | yes | yes | |
| `ReservationVaultUpdated` | `:326` | #1088 | yes | yes | |
| `ReservationCapsUpdated` | `:328` | #1093 | yes | yes | |

Events are not storage, so omitting an m2 event from m1 costs nothing structural.
But monitoring depends on them, and `ReservationStranded` is the only signal that
an m1 position closed, so it is load-bearing for the operational duties.

## Open questions

1. **DECISION NEEDED: keep or delete the unreachable refund branch** at
   `Reservation.sol:1009-1022` in the m1 timeout path? Fields stay either way;
   this is about code, which is replaceable. Recommend keeping for a smaller m2
   diff.
2. **RESOLVED in the M1 code.** §7's `validateReservationCapsInvariant`
   plus the two new `require`s in `updateReservationParameters` and
   `updateReservationCaps` (M1 `Reservation.sol:1220-1232`, `:1295-1298`,
   `:1382-1396`) add exactly the relational validation this question asked
   for — see the §7 note.
3. **Enum positions are a layout hazard nobody has written down.** Confirm the
   rewrite extracts all three enums verbatim including m2-only variants, rather
   than pruning them.
4. **RESOLVED in the M1 code.** M1 keeps the relational check verbatim
   (`0 < reservationRenewalWindowSeconds < reservationTermSeconds`, M1
   `Reservation.sol:1286-1289`) even though the field still has no m1
   reader — the "keep it" option was taken.
5. **UNVERIFIED at the `#1102` fold.** All line numbers above are guards-tip.
   `#1102` changed `Reservation.sol` by +342 -95, so the parameter validation
   block in particular should be re-read post-fold before implementation.
