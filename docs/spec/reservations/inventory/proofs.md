# `ReservationProofs.sol` action-lifecycle inventory for the m1/m2 split

**Sources read in full**

| Tree | Path | Lines | Branch | PR |
|---|---|---|---|---|
| guards | `solidity/contracts/bridge/ReservationProofs.sol` | 1470 | `feat/utxo-reservation-guards` | #1094 tip |
| partial | `solidity/contracts/bridge/ReservationProofs.sol` | 1697 | `feat/utxo-reservation-partial-redemption` | #1096 tip |

Supporting files read for the request side, timeout, slashing and the close/strand helpers (these do **not** live in `ReservationProofs.sol`): `Reservation.sol` (guards 1507 ln, partial 1662 ln), `ReservationRouter.sol`, `BridgeState.sol`, `Wallets.sol`, `vault/ReservationVault.sol`.

**PR-to-branch map**, verified from `../feature-spec.md:30-38` and `../epic-merge-plan.md:64-71`, cross-checked against `.git/packed-refs:77-83`:

| PR | Branch | Title |
|---|---|---|
| #1088 | `feat/utxo-reservation-core` | core reservation data model, `ReservationVault` |
| #1102 | `fix/utxo-reservation-review-followups` | merged into `-core` 2026-08-21 (`3566e059`) |
| #1090 | `feat/utxo-reservation-router` | delegatecall reservation router |
| #1091 | `feat/utxo-reservation-settlement` | two-phase authorize-then-prove settlement |
| #1092 | `feat/utxo-reservation-renewal` | bounded renewal, strict expiry |
| #1093 | `feat/utxo-reservation-backing` | claim-equals-anchor, financed in-kind fees |
| #1094 | `feat/utxo-reservation-guards` | wallet binding, pending-deposit guard, stranding |
| #1095 | `docs/utxo-reservation-release` | docs + tests |
| #1096 | `feat/utxo-reservation-partial-redemption` | partial reserved redemption |

**Decision-ID namespace.** The `PD-N` numbers below are **local to this
fragment** and are NOT the canonical register in `../milestone-inventory.md`
section 7, which renumbered them during synthesis. Nine of the twelve changed
number, so citing a bare `D-N` across the two documents resolves to the wrong
decision. Concordance:

| this fragment | canonical register |
|---|---|
| PD-1 | D-4 |
| PD-2 | D-5 |
| PD-3 | D-8 |
| PD-4 | D-9 |
| PD-5 | D-1 |
| PD-6 | D-6 |
| PD-7 | D-7 |
| PD-8 | D-15 |
| PD-9 | D-14 |
| PD-10 | D-1, D-10 |
| PD-11 | D-1, D-10, D-11 |
| PD-12 | resolved 2026-08-21, no register entry |

PD-12 never reached the register. It said `m1-variant-comparison.md` cited
`ReservationProofs.sol:715`/`:836` as "the redemption path (unreachable)" when
`:836` is the re-anchor source-wallet count decrement, reachable in m1. That
citation was corrected at `m1-variant-comparison.md:251-259`. Cite the register
for everything else.

**Column conventions.** `m1` = the item is in the variant-B m1 rewrite. `m2` = m2 must add or modify this item. Both `yes` means m1 ships it and m2 changes it. `flagged` means a `DECISION NEEDED` applies. Unqualified `File.sol:N` citations are the **guards** tree; partial-tree citations are prefixed `partial `.

**PR attribution caveat.** Only two *Solidity* snapshots were read as working trees (the guards tip and the partial-redemption tip; keep-core `#4238` is the third, non-Solidity one) and `gh` is unavailable. All ten branch refs are fetched, so any is readable via `git show`, but per-PR attribution below was not re-derived per branch. The guards-to-partial delta (§8) is measured exactly and its `#1096` attributions are verified. Attributions to #1088/#1090-#1094 are derived from PR titles (`feature-spec.md:30-38`) plus explicit file-and-line attributions in `roadmap.md:886-894` and `m1-variant-comparison.md:486-501`; each such row carries `?`-qualified confidence in the Note where the evidence is title-level only.

---

## 0. What PR #1096 adds relative to guards

Ten changes, all verified by reading both files end to end.

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `event ReservationPartiallyRedeemed` | partial `ReservationProofs.sol:87-93` | #1096 | event | no | yes | New event; no guards counterpart. |
| `prepareReservationForSettlement` gains `action` param | partial `:262-267` vs `:254-258` | #1096 | internal | yes | yes | Signature change. m1 must pick one shape; see PD-9. |
| Source-anchor check folded into `prepareReservationForSettlement` | partial `:270-273` | #1096 | invariant | yes | no | Absorbs the guards helper below. |
| `requireCurrentSourceAnchor` helper **deleted** | guards `:308-313`; 3 call sites `:665`, `:778`, `:955` | #1096 | internal | flagged | flagged | Deleted in partial. Shared m1+m2 helper (rule 5). |
| `anchorUtxoKey` reuses the computed hash | partial `:288` vs guards `:275-282` | #1096 | internal | yes | no | Pure de-duplication, same value. |
| `submitReservedRedemptionProof` branches on `action.isPartial` | partial `:665-687` | #1096 | entry-point | no | yes | Dispatch to partial vs whole settle. |
| `retireRetryCreditForGeneration` | partial `:702-715` | #1096 | internal | no | yes | Writes `reservation.retryCredit=false` (`:712`), deletes `reservationRetryCreditActionNonce` (`:713`). |
| `settleWholeRedemption` extracted | partial `:720-786` (was inline guards `:674-728`) | #1096 | internal | no | yes | Body otherwise identical to guards. |
| `settlePartialRedemption` | partial `:795-876` | #1096 | internal | no | yes | New. |
| `validatePartialOutputs` | partial `:882-931` | #1096 | view | no | yes | New; 2-output shape. |
| `resolveLateRedemptionAgainstPending` renamed `resolveLateAgainstPending` | guards `:1206` -> partial `:1407` | #1096 | internal | no | yes | New params `redeemerValue`, `effectiveRedeemAmount`. |
| Late-match predicate tightened | partial `:1434-1439` vs guards `:1222-1228` | #1096 | invariant | no | yes | Adds `isPartial ==` and `amount ==` equality. |
| `unwindPendingAction` restore now passes `action.isPartial` | partial `:1448-1453` vs guards `:1231` | #1096 | internal | no | yes | Guards hardcodes `false`. |
| `unwindPendingAction` writes retry-credit source nonce | partial `:1487-1491` vs guards `:1263-1264` | #1096 | storage-write | flagged | yes | See PD-2. |
| `ReservationAction.isPartial` | partial `Reservation.sol:263` | #1096 | storage-write | flagged | yes | See PD-1. |
| `ReservationAction.retryCreditSourceNonce` | partial `Reservation.sol:268` | #1096 | storage-write | flagged | yes | See PD-1. |
| `BridgeState.Storage.reservationRetryCreditActionNonce` | partial `BridgeState.sol:461` | #1096 | storage-write | flagged | yes | See PD-2. |
| `requestPartialReservedRedemption` | partial `Reservation.sol:685-718` | #1096 | entry-point | no | yes | |
| `_requestReservedRedemption` shared body | partial `Reservation.sol:723-852` | #1096 | internal | no | yes | |
| `consumeRetryCredit` | partial `Reservation.sol:860-895` | #1096 | internal | no | yes | Amount/shape binding at `:880-884`. |

`strandReservation` and `closeReservation` bodies are byte-identical between trees (guards `Reservation.sol:1442-1487`/`:1490-1506`, partial `:1594-1639`/`:1644-1660`); only doc comments differ.

---

## 1. Acceptance

### 1a. Request side

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `ReservationRouter.requestReservationAcceptance` | `ReservationRouter.sol:242-247` | #1090 | entry-point | yes | no | Permissionless wrapper, no modifier. |
| `Reservation.requestReservationAcceptance` | `Reservation.sol:426-590` | #1091 (two-phase) | entry-point | yes | no | Title-level attribution. |
| Validates vault set, deposit revealed, not swept, `isReserved` | `Reservation.sol:431-444` | #1091/#1094 | invariant | yes | no | `isReserved` guard is #1094 (`pendingReservedDeposit`). |
| Validates designated wallet binding (twice) | `Reservation.sol:446-450` and `:472-475` | #1094 | invariant | yes | no | Duplicate require; harmless. |
| Validates position `Unknown`, no pending acceptance | `Reservation.sol:453-461` | #1091 | invariant | yes | no | |
| Validates wallet `Live` | `Reservation.sol:477-481` | #1091 | invariant | yes | no | |
| Validates `amount >= minAmount + txMaxFee` | `Reservation.sol:483-487` | #1091 | invariant | yes | no | Makes proof-time fee bound sufficient. |
| Validates signing window vs `DEPOSIT_MIN_AGE` and timeout margin | `Reservation.sol:493-509` | #1091 | invariant | yes | no | |
| Validates window vs exact reveal-time refund deadline | `Reservation.sol:516-531` | #1094 | invariant | yes | no | |
| Cap: single-reservation | `Reservation.sol:533-537` | #1102 | invariant | yes | no | `reservationMaxSingleAmount`; caps are #1102/#1093 line. |
| **Reserves `reservationTotalAmount`** | `Reservation.sol:541-546` | #1091 | storage-write | yes | no | initiation-path. |
| **Reserves `walletReservationsCount`** | `Reservation.sol:548-553` | #1091 | storage-write | yes | no | initiation-path. |
| **Reserves `walletReservationsAmount`** | `Reservation.sol:555-562` | #1093 | storage-write | yes | no | initiation-path. |
| **`++reservation.requestNonce`** | `Reservation.sol:564` | #1091 | storage-write | yes | no | |
| Snapshot: `actionType`, `state`, `requestedAt`, `timeoutAt`, `txMaxFee`, `targetWalletPubKeyHash`, `amount` | `Reservation.sol:571-578` | #1091 | storage-write | yes | no | No `sourceAnchorUtxoHash`, no `actionDataHash`: acceptance has no source anchor. |
| `emit ReservationAcceptanceRequested` | `Reservation.sol:580-588` | #1091 | event | yes | no | |

### 1b. Proof side

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `ReservationRouter.submitReservationProof` | `ReservationRouter.sol:322-338` | #1090 | entry-point | yes | no | Sole external settlement entry. **Never pause-gate.** |
| `ReservationProofs.submitReservationProof` dispatcher | `:134-180` | #1091 | entry-point | yes | yes | m1 drops the `Redemption` (`:151`) and `Dissolution` (`:169`) arms. |
| `enum ProofType` | `:117-122` | #1091 | invariant | yes | yes | m1 must keep numbering stable or m2 breaks encoding. See PD-7. |
| `loadSettleableAction` | `:186-206` | #1091 | internal | yes | no | **Shared across all four action types (rule 5): m1 work.** |
| `submitReservationAcceptanceProof` | `:343-390` | #1091 | entry-point | yes | no | |
| Requires position `Unknown` | `:360-364` | #1091 | invariant | yes | no | |
| Requires `pendingReservedDeposit.isReserved` | `:365-368` | #1094 | invariant | yes | no | |
| **SPV: `self.validateProof(anchorTx, anchorProof)`** | `:370` | #1091 | internal | yes | no | Existing `BitcoinTx` machinery. |
| `consumeAcceptedDeposit` | `:400-432` | #1091/#1094 | internal | yes | no | Acceptance-exclusive. |
| - input must spend the reserved deposit outpoint | `:405-412` | #1091 | invariant | yes | no | Sole-input parse via `OutboundTx.parseWalletOutboundTxInput`. |
| - requires `deposit.sweptAt == 0` | `:414-415` | #1091 | invariant | yes | no | |
| - **writes `deposit.sweptAt`** | `:418` | #1091 | storage-write | yes | no | Blocks regular sweep; enables fraud-defeat. |
| - **deletes `pendingReservedDeposit` marker fields** | `:427-429` | #1094 | storage-write | yes | no | Idempotent against a prior stale notification. |
| - **`pendingReservedDeposits -= 1`** | `:430` | #1094 | storage-write | yes | no | |
| `validateAnchorOutput` | `:437-452` | #1091 | view | yes | no | |
| - exactly one output (`parseSingleOutput`) | `:442`, helper `:1315-1330` | #1091 | view | yes | no | **Shared helper (rule 5).** |
| - output pays `action.targetWalletPubKeyHash` | `:444-447` | #1091 | invariant | yes | no | |
| - `action.amount - anchorAmount <= action.txMaxFee` | `:448-451` | #1091 | invariant | yes | no | |
| **`action.state = Settled`** | `:467` | #1091 | storage-write | yes | no | Only `action.state` transition on the acceptance proof path. |

### 1c. Settlement and accounting

All rows below are **settlement-path**. None may be pause-gated (rule 2).

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `settleAcceptance` | `:458-592` | #1091 | internal | yes | no | |
| Late: unwind a newer pending acceptance generation | `:469-492`, call `:490` | #1091 | internal | yes | no | Reachable in m1. |
| Late: **`reservationTotalAmount += anchorAmount`** | `:498` | #1091 | storage-write | yes | no | settlement-path. Deliberately no cap re-check. |
| Late: **`walletReservationsCount[target] += 1`** | `:499` | #1091 | storage-write | yes | no | settlement-path. |
| Late: **`walletReservationsAmount[target] += anchorAmount`** | `:500-502` | #1093 | storage-write | yes | no | settlement-path. |
| Late: `emit ReservationLateSettled` | `:503-507` | #1091 | event | yes | no | |
| Not late: **`reservationTotalAmount -= (amount - anchorAmount)`** | `:511` | #1091 | storage-write | yes | no | Releases the miner-fee delta. settlement-path. |
| Not late: **`walletReservationsAmount[target] -= (amount - anchorAmount)`** | `:512-514` | #1093 | storage-write | yes | no | settlement-path. |
| **Position creation: `owner`** | `:527` | #1091 | storage-write | yes | no | |
| **`mintedAmount = anchorAmount`** | `:528` | #1093 | storage-write | yes | no | claim==anchor. |
| **`acceptedAt`** | `:530` | #1091 | storage-write | yes | no | |
| **`walletPubKeyHash`** | `:531` | #1091 | storage-write | yes | no | |
| **`anchorAmount`** | `:532` | #1091 | storage-write | yes | no | |
| **`expiresAt`** | `:533` | #1091 | storage-write | yes | no | m2 renewal (#1092) also writes it; m1 write makes it storage-complete. |
| **`anchorTxHash`** | `:534` | #1091 | storage-write | yes | no | |
| **`anchorTxOutputIndex = 0`** | `:535` | #1091 | storage-write | yes | no | m2 partial writes `1` (partial `:845`). |
| **`state = Active`** | `:536` | #1091 | storage-write | yes | no | |
| **`dissolutionEligibleAt`** | `:537-539` | #1093 | storage-write | yes | no | **Rule 1: m1 must write it even though m1 deletes every reader (rule 4).** |
| **`reservationsByAnchorUtxo[keccak(txHash,0)] = key`** | `:541-543` | #1091 | storage-write | yes | no | |
| **`addWalletReservationKey`** | `:544-548`, body `Reservation.sol:1405-1414` | #1094 | storage-write | yes | no | Enumeration. |
| `emit ReservationAccepted` | `:551-559` | #1091 | event | yes | no | |
| **External call: `bank.increaseBalanceAndCall(deposit.vault, [depositor], [anchorAmount])`** | `:574-578` | #1088/#1091 | internal | yes | no | **Mint. settlement-path. Never pause-gate.** Uses the deposit's immutable `vault`, not live `reservationVault`. |
| `strandLateSettlementIfTargetWalletClosed(..., evidenceAlreadyEmitted=false)` | `:585-591` | #1094 | internal | yes | no | Position-closing site; see §4. |

---

## 2. Re-anchor

### 2a. Request side

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `ReservationRouter.requestReservationReanchor` | `ReservationRouter.sol:286-295` | #1090 | entry-point | yes | no | Sets `privileged = msg.sender == governance` at `:293`. |
| `Reservation.requestReservationReanchor` | `Reservation.sol:771-868` | #1091 | entry-point | yes | no | |
| Requires `state == Active` | `Reservation.sol:781-784` | #1091 | invariant | yes | no | |
| **Requires `block.timestamp < dissolutionEligibleAt`** | `Reservation.sol:785-788` | #1094 | invariant | **no** | yes | **Rule 4: deleted in m1.** m2 restores it alongside dissolution. |
| Source wallet must be `MovingFunds`, or `Live` with `privileged` | `Reservation.sol:790-806` | #1094 | invariant | yes | no | **Not relaxed by rule 4.** See PD-4. |
| Target differs from source; target must be `Live` | `Reservation.sol:808-817` | #1091 | invariant | yes | no | |
| **Reserves `walletReservationsCount[target]`** | `Reservation.sol:820-827` | #1094 | storage-write | yes | no | initiation-path. Cap check `<= maxReservationsPerWallet`. |
| **Reserves `walletReservationsAmount[target]`** | `Reservation.sol:829-837` | #1093 | storage-write | yes | no | initiation-path. |
| **`state = ActionPending`** | `Reservation.sol:839` | #1091 | storage-write | yes | no | |
| **`++requestNonce`** | `Reservation.sol:840` | #1091 | storage-write | yes | no | |
| Snapshot `actionType/state/requestedAt/timeoutAt/txMaxFee/target/amount/sourceAnchorUtxoHash` | `Reservation.sol:847-858` | #1091 | storage-write | yes | no | `sourceAnchorUtxoHash` at `:858` via `anchorUtxoHash` (`:382-396`). |
| `emit ReservationReanchorRequested` | `Reservation.sol:860-866` | #1091 | event | yes | no | |

### 2b. Proof side

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `submitReservationReanchorProof` | `:743-899` | #1091 | entry-point | yes | no | |
| `loadSettleableAction(..., Reanchor)` | `:753-758` | #1091 | internal | yes | no | |
| `evidenceAlreadyEmitted = state == Stranded` | `:763-764` | #1094 | internal | yes | no | |
| `prepareReservationForSettlement` | `:765-770`, body `:254-301` | #1091/#1094 | internal | yes | no | **Shared across redemption, re-anchor, dissolution (rule 5): m1 work.** |
| - requires `Active` or `ActionPending` or (`Stranded` and `late`) | `:262-267` | #1094 | invariant | yes | no | |
| - requires `!spentMainUTXOs[anchorUtxoKey]` when stranded | `:285-288` | #1094 | invariant | yes | no | |
| - **reconstructs `reservationTotalAmount`** | `:290` | #1094 | storage-write | yes | no | settlement-path. |
| - **reconstructs `walletReservationsCount`** | `:291` | #1094 | storage-write | yes | no | settlement-path. |
| - **reconstructs `walletReservationsAmount`** | `:292-294` | #1094 | storage-write | yes | no | settlement-path. |
| - **reconstructs `addWalletReservationKey`** | `:295-299` | #1094 | storage-write | yes | no | settlement-path. |
| - **reconstructs `reservationsByAnchorUtxo`** | `:300` | #1094 | storage-write | yes | no | settlement-path. |
| Not late: requires `requestNonce == requestNonce` | `:772-775` | #1091 | invariant | yes | no | |
| `requireCurrentSourceAnchor` | `:778`, body `:308-313` | #1091 | view | yes | yes | **Shared helper. Deleted by #1096** (folded into `prepareReservationForSettlement`). See PD-9. |
| **SPV: `validateProof`** | `:780` | #1091 | internal | yes | no | |
| `consumeAnchor` | `:782`, body `:1334-1358` | #1091 | internal | yes | no | **Shared with redemption (rule 5): m1 work.** |
| - input must point at the current anchor outpoint | `:1342-1346` | #1091 | invariant | yes | no | |
| - **`spentMainUTXOs[anchorUtxoKey] = true`** | `:1356` | #1091 | storage-write | yes | no | Fraud-defeat recognition. See PD-8 on the #1102 convergence. |
| - **`delete reservationsByAnchorUtxo[anchorUtxoKey]`** | `:1357` | #1091 | storage-write | yes | no | |
| Exactly one output | `:784` (`parseSingleOutput` `:1315-1330`) | #1091 | view | yes | no | |
| Output pays `action.targetWalletPubKeyHash` | `:788-791` | #1091 | invariant | yes | no | |
| `anchorAmount - newAnchorAmount <= txMaxFee` | `:795-798` | #1091 | invariant | yes | no | |
| **Dust floor `newAnchorAmount > txMaxFee`** | `:803-806` | #1093 | invariant | yes | no | Preserves positive redemption value; keep even though m1 has no redemption. |
| **`action.state = Settled`** | `:878` | #1091 | storage-write | yes | no | The re-anchor `action.state` transition. |

### 2c. Settlement and accounting

All rows **settlement-path**.

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| Late: **`walletReservationsCount[new] += 1`** | `:812` | #1091 | storage-write | yes | no | Re-takes released capacity. No cap check. |
| Late: **`walletReservationsAmount[new] += anchorAmount`** | `:813-814` | #1093 | storage-write | yes | no | |
| Late: unwind a newer pending generation, `restoreRetryCredit = true` | `:818-822` | #1091/#1093 | internal | yes | yes | The `true` argument only matters for a pending **Redemption** (`:1262`), so in m1 it is inert. |
| Late: `emit ReservationLateSettled` | `:825-829` | #1091 | event | yes | no | |
| **`walletReservationsCount[source] -= 1`** | `:836` | #1091 | storage-write | yes | no | **Not a position close.** See §4-W1. |
| **`walletReservationsAmount[source] -= anchorAmount`** | `:837-839` | #1093 | storage-write | yes | no | |
| **`walletReservationsAmount[new] -= (anchorAmount - newAnchorAmount)`** | `:840-841` | #1093 | storage-write | yes | no | Releases the miner-fee delta on the target. |
| **`reservationTotalAmount -= minerFee`** | `:843`, `:846` | #1093 | storage-write | yes | no | |
| **`removeWalletReservationKey(source)`** | `:848-852`, body `Reservation.sol:1418-1437` | #1094 | storage-write | yes | no | Enumeration move, not a close. |
| **`addWalletReservationKey(new)`** | `:853-857` | #1094 | storage-write | yes | no | |
| **`reservation.walletPubKeyHash = new`** | `:859` | #1091 | storage-write | yes | no | |
| **`reservation.anchorAmount = newAnchorAmount`** | `:860` | #1091 | storage-write | yes | no | |
| **`reservation.mintedAmount = newAnchorAmount`** | `:868` | #1093 | storage-write | yes | no | Claim write-down; claim==anchor. |
| **`reservation.anchorTxHash = reanchorTxHash`** | `:869` | #1091 | storage-write | yes | no | |
| **`reservation.anchorTxOutputIndex = 0`** | `:870` | #1091 | storage-write | yes | no | |
| **`reservation.state = Active`** | `:871` | #1091 | storage-write | yes | no | Position stays open. |
| **External call: `IReservationFeeFinancer(deposit.vault).financeInKindFee(minerFee)`** | `:873-876` | #1093 | internal | yes | no | **settlement-path. Never pause-gate.** Verified non-reverting: `ReservationVault.sol:529-557` has one require (caller is Bridge, `:530`), returns early on zero (`:532-533`), and records a shortfall as `inKindFeeDebtSat` (`:556`) rather than reverting; rationale at `:523-528`. |
| **`reservationsByAnchorUtxo[keccak(reanchorTxHash,0)] = key`** | `:880-882` | #1091 | storage-write | yes | no | |
| `emit ReservationReanchored` | `:885-891` | #1091 | event | yes | no | |
| `strandLateSettlementIfTargetWalletClosed(..., evidenceAlreadyEmitted)` | `:893-899` | #1094 | internal | yes | no | Position-closing site; see §4. |

---

## 3. Redemption (m2)

> **Recovery gap.** Subsections 3a (request side) and 3b (proof side) were lost
> when this fragment was recovered from a paged agent artifact, and the artifact
> itself elides rows, so they cannot be restored verbatim. Only 3c below
> survives intact. This is low-stakes for the milestone split: redemption is m2
> in whole and in part, so no m1 row depends on it, and the two facts the rest
> of this document needs from the redemption path are both carried in §4 -
> `closeReservation` at `ReservationProofs.sol:715` (position-closing site
> C1a, unreachable in variant B) and the retry-credit and watchtower-delay
> fields written only by `requestReservedRedemption`. Re-derive 3a and 3b from
> source before m2 planning.

### 3c. Settlement and accounting

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| **`self.closeReservation(...)`** | `:715`; partial `:772` | #1091 | storage-write | no | yes | Position-closing site; see §4-C1a. |
| `emit ReservedRedemptionCompleted` | `:718-722` | #1091 | event | no | yes | |
| **Burn: `bank.decreaseBalance(action.amount)`, not-late only** | `:726-727`; partial `:784` | #1091 | internal | no | yes | **settlement-path. Never pause-gate.** |
| Partial: **`reservationTotalAmount -= action.amount`** | partial `:838` | #1096 | storage-write | no | yes | settlement-path. |
| Partial: **`walletReservationsAmount -= action.amount`** | partial `:839-840` | #1096 | storage-write | no | yes | settlement-path. |
| Partial: **`mintedAmount -= action.amount`** | partial `:842` | #1096 | storage-write | no | yes | |
| Partial: **`anchorAmount = remainderValue`**, **`anchorTxHash`**, **`anchorTxOutputIndex = 1`**, **`state = Active`** | partial `:843-846` | #1096 | storage-write | no | yes | Position stays open. |
| Partial: **`reservationsByAnchorUtxo[keccak(txHash,1)]`** | partial `:848-850` | #1096 | storage-write | no | yes | |
| Partial: `emit ReservationPartiallyRedeemed` | partial `:852-858` | #1096 | event | no | yes | |
| Partial: **burn `bank.decreaseBalance(action.amount)`** | partial `:863` | #1096 | internal | no | yes | **settlement-path.** |
| Partial: re-strand residual | partial `:873-874` | #1096 | storage-write | no | yes | Position-closing site; see §4-S1d. |
| **Veto: `action.state = Vetoed`, `reservation.state = Active`** | `Reservation.sol:1107-1108` | #1091 | storage-write | no | yes | |
| **Veto: `bank.transferBalance(watchtower, action.amount)`** | `Reservation.sol:1113` | #1091 | internal | no | yes | **settlement-path.** |

---

## 4. Position-closing sites (the critical output)

**Framing.** No code path anywhere deletes a `reservations[key]` entry. Positions are only state-transitioned. A "position close" is therefore: `state = Closed` (`closeReservation`), `state = Stranded` (`strandReservation`), or a `walletReservationsCount` decrement. The count-decrement class splits cleanly into true closes (inside `closeReservation`/`strandReservation`) and capacity moves/releases that leave the position open. Both classes are listed.

### 4.1 True closing sites, complete

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| **C1 body** `closeReservation`: count `-=1`, amount `-=`, total `-=`, `removeWalletReservationKey`, `state = Closed` | `Reservation.sol:1490-1506` (writes at `:1495`, `:1496-1498`, `:1499`, `:1500-1504`, `:1505`) | #1091 | storage-write | flagged | yes | Body is m1 work only if a caller exists. See PD-3. |
| **C1a** `closeReservation` call in whole redemption settlement | `ReservationProofs.sol:715`; partial `:772` | #1091 | storage-write | no | yes | **UNREACHABLE in variant B** — redemption is m2 (rule 3). |
| **C1b** `closeReservation` call in `settleDissolution`, non-terminated wallet | `ReservationProofs.sol:1142` | #1091 | storage-write | no | yes | **UNREACHABLE in variant B** — dissolution is m2 (rule 3). |
| **S1 body** `strandReservation`: count `-=1`, amount `-=`, total `-=`, `removeWalletReservationKey`, `state = Stranded`, `delete reservationsByAnchorUtxo`, conditional `emit ReservationStranded` | `Reservation.sol:1442-1487` (writes at `:1450`, `:1451-1453`, `:1454`, `:1455-1459`, `:1460`, `:1462-1472`; event `:1478-1483`) | #1094 | storage-write | yes | no | **REACHABLE.** |
| **S1a-i** `strandLateSettlementIfTargetWalletClosed` -> `strandReservation` reached from **late acceptance** | call `ReservationProofs.sol:585-591`; helper `:218-243`; strand call `:241` | #1094 | storage-write | yes | no | **REACHABLE in m1.** Conditions: `late == true` (`:224-226`); target wallet `Closing` or `Closed` (`:232-234`). The `Terminated` arm (`:235-236`) is dead from this call site because `evidenceAlreadyEmitted` is hardcoded `false` at `:590`. |
| **S1a-ii** same helper reached from **late re-anchor** | call `ReservationProofs.sol:893-899`; strand call `:241` | #1094 | storage-write | yes | no | **REACHABLE in m1.** Conditions: `late == true`; new wallet `Closing`/`Closed`, **or** `Terminated` with `evidenceAlreadyEmitted` (computed at `:763-764`), which also re-latches `reservation.state = Stranded` at `:239`. |
| **S1b** `strandReservation` in `settleDissolution`, terminated wallet | `ReservationProofs.sol:1140` | #1094 | storage-write | no | yes | **UNREACHABLE in variant B** — dissolution is m2. |
| **S1c** `notifyReservationStranded` -> `strandReservation` | entry `Reservation.sol:1363-1381`; strand call `:1380` | #1094 | entry-point | yes | no | **REACHABLE in m1** (rule 3 puts stranding in m1). Conditions: `state == Active` (`:1370-1373`) and custodying wallet `Terminated` (`:1374-1378`). Router wrapper `ReservationRouter.sol:461-463`, permissionless. |
| **S1d** re-strand residual after a late partial redemption | partial `ReservationProofs.sol:873-874` | #1096 | storage-write | no | yes | **UNREACHABLE in variant B.** |

### 4.2 Verdict for variant B

**Exactly two closing sites are reachable, and both terminate in `Stranded`, never `Closed`:**

1. **S1c** `notifyReservationStranded` (`Reservation.sol:1363` -> `:1380` -> `:1460`), gated on the custodying wallet being `Terminated`.
2. **S1a** `strandLateSettlementIfTargetWalletClosed` (`ReservationProofs.sol:218` -> `:241` -> `:1460`), reachable only from a **late** acceptance settlement (`:585`) or a **late** re-anchor settlement (`:893`), and only when the target wallet is `Closing`/`Closed` (or `Terminated` with prior evidence, re-anchor only).

Both require the custodying or target wallet to be *already* retiring or terminated. **A position held by a healthy `Live` wallet has no closing path whatsoever in variant B.** `closeReservation` (`Reservation.sol:1490`) becomes an orphan: both of its call sites (`:715`, `:1142`) are m2.

**Correction to the existing spec.** `m1-variant-comparison.md:251-253` lists "`ReservationProofs.sol:715`/`:836` are the redemption path (unreachable)". The `:836` citation is wrong. Guards `:836` is `self.walletReservationsCount[reservation.walletPubKeyHash] -= 1;` inside `submitReservationReanchorProof`, not a redemption site, and it **is reachable in m1**. It is not a close: it pairs with the target-wallet `+= 1` taken at request time (`Reservation.sol:827`), so global occupancy is unchanged and the position stays `Active` (`:871`). The doc's conclusion (occupancy is monotonic in B) survives the correction, because a net-zero move does not free a slot.

### 4.3 Non-closing count and amount mutations, complete

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| **W1** re-anchor source-wallet `count -= 1` | `ReservationProofs.sol:836` | #1091 | storage-write | yes | no | **REACHABLE.** Net-zero move; not a close. |
| **W1b** re-anchor source-wallet `amount -=` | `:837-839` | #1093 | storage-write | yes | no | **REACHABLE.** |
| **W1c** re-anchor target `amount -=` miner-fee delta | `:840-841` | #1093 | storage-write | yes | no | **REACHABLE.** |
| **W1d** re-anchor `reservationTotalAmount -= minerFee` | `:846` | #1093 | storage-write | yes | no | **REACHABLE.** True backing reduction. |
| **W2** `unwindPendingAction` Reanchor branch: target `count -= 1`, `amount -=` | `:1276-1281` | #1091 | storage-write | yes | no | **REACHABLE** via late re-anchor (`:821`) when the newer pending generation is itself a Reanchor. Releases a *reservation of capacity*, not a position. |
| **W3** `unwindPendingAction` Acceptance branch: `total -=`, `count -= 1`, `amount -=` | `:1298-1304` | #1091 | storage-write | yes | no | **REACHABLE** via late acceptance (`:490`). |
| **W4** `unwindPendingAction` Redemption branch: `bank.transferBalance` refund | `:1269-1272` | #1091 | internal | no | yes | Unreachable in m1: no Redemption action can exist. |
| **W5** `unwindPendingAction` Dissolution branch: `delete walletPendingDissolution` | `:1285-1292` | #1091 | storage-write | no | yes | Unreachable in m1. |
| **W6** `unwindPendingAction` `pendingAction.state = Superseded` | `:1259` | #1091 | storage-write | yes | no | **REACHABLE** in m1 via `:490` and `:821`. |
| **W7** acceptance-timeout capacity release | `Reservation.sol:1001-1005` | #1091 | storage-write | yes | no | **REACHABLE.** initiation-side release. |
| **W8** re-anchor-timeout target capacity release | `Reservation.sol:1026-1029` | #1091 | storage-write | yes | no | **REACHABLE.** |
| **W9** settle-acceptance non-late miner-fee release | `ReservationProofs.sol:511-514` | #1091 | storage-write | yes | no | **REACHABLE.** |
| **W10** `prepareReservationForSettlement` stranded reconstruction (`+=` all three) | `:290-300` | #1094 | storage-write | yes | no | **REACHABLE** via late re-anchor on an already-stranded position. Inverse of a close. |
| **W11** partial redemption `total -=`, `amount -=` | partial `:838-840` | #1096 | storage-write | no | yes | Unreachable in m1. |
| **W12** `notifyStaleReservedDeposit`: `pendingReservedDeposits -= 1` | `Reservation.sol:1343` | #1094 | storage-write | yes | no | **REACHABLE** (rule 3). Not a position; the position does not exist yet. |
| **W13** `consumeAcceptedDeposit`: `pendingReservedDeposits -= 1` | `ReservationProofs.sol:430` | #1094 | storage-write | yes | no | **REACHABLE.** |

---

## 5. Timeout and slashing

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `ReservationRouter.notifyReservationActionTimeout` | `ReservationRouter.sol:351-356` | #1090 | entry-point | yes | yes | Permissionless. |
| `Reservation.notifyReservationActionTimeout` | `Reservation.sol:972-1062` | #1091 | entry-point | yes | yes | m1 keeps Acceptance + Reanchor arms only. |
| Requires pending action and elapsed `timeoutAt` | `Reservation.sol:987-991` | #1091 | invariant | yes | no | |
| **`action.state = TimedOut`** | `Reservation.sol:993` | #1091 | storage-write | yes | no | Enables late settlement. |
| Acceptance arm: capacity release | `Reservation.sol:997-1005` | #1091 | storage-write | yes | no | **No slashing.** Position never existed. |
| Reanchor arm: `state = Active`, target capacity release | `Reservation.sol:1023-1029` | #1091 | storage-write | yes | no | **No slashing.** |
| Redemption arm: `state = Active` | `Reservation.sol:1007` | #1091 | storage-write | no | yes | |
| Redemption arm: **mint retry credit** `retryCredit = true` + event | `Reservation.sol:1009-1012`; partial adds nonce write `:1157-1161` | #1091 / #1096 | storage-write | flagged | yes | See PD-2. |
| **Redemption arm: `notifyWalletRedemptionTimeout` -> SLASH** | `Reservation.sol:1016-1019`; body `Wallets.sol:245`, seize at `:265-270` with `redemptionTimeoutSlashingAmount` (`:266`) | #1091 | internal | no | yes | Fed by **Redemption** actions only. |
| **Redemption arm: refund `bank.transferBalance(redeemer, amount)`** | `Reservation.sol:1022` | #1091 | internal | no | yes | **settlement-path. Never pause-gate.** |
| **Dissolution arm: `notifyWalletRedemptionTimeout` -> SLASH** | `Reservation.sol:1042-1045` | #1094 | internal | no | yes | Fed by **Dissolution** actions only. |
| **Dissolution arm: `terminateWallet` if already MovingFunds** | `Reservation.sol:1035-1039`, `:1051-1053`; body `Wallets.sol:731` | #1094 | internal | no | yes | |
| Dissolution arm: `delete walletPendingDissolution` | `Reservation.sol:1033` | #1091 | storage-write | no | yes | |
| `emit ReservationActionTimedOut` | `Reservation.sol:1057-1061` | #1091 | event | yes | no | |
| **`beginWalletClosing` requires `walletReservationsCount == 0`** | `Wallets.sol:664-677` (`:675-677`), repeated `:706-709` | #1094 | invariant | yes | no | The pin. Blocks orderly retirement while any reservation is custodied. |
| Same guard in `moveFunds` fast path | `Wallets.sol:627-630` (`:628`) | #1094 | invariant | yes | no | |
| Same guard in the closing-eligibility check | `Wallets.sol:437-441` (`:438`) | #1094 | invariant | yes | no | |
| **`notifyWalletMovingFundsTimeout` -> SLASH + terminate** | `Wallets.sol:493-516`; seize `:507-511` with `movingFundsTimeoutSlashingAmount` (`:508`) | pre-existing | internal | yes | no | **The only slashing path reachable in variant B**, and it is fed by **no reservation action type**: it is reached because reservations block `beginWalletClosing`. This is the §5.3 endgame, verified. |

**Slashing summary for variant B.** Neither m1 action type (acceptance, re-anchor) has a slashing arm. The two reservation-driven slashing arms (`Reservation.sol:1016-1019` and `:1042-1045`) are both m2. The only stake seizure reachable in m1 is `Wallets.sol:507-513`, reached indirectly: a pinned wallet cannot pass `Wallets.sol:674-677`, so it sits in `MovingFunds` until its timeout fires. Honest operators, slashed by arithmetic.

---

## 6. Stranding

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `ReservationRouter.notifyReservationStranded` | `ReservationRouter.sol:461-463` | #1090 | entry-point | yes | no | Permissionless, no modifier. |
| `Reservation.notifyReservationStranded` | `Reservation.sol:1363-1381` | #1094 | entry-point | yes | no | |
| Requires `state == Active` | `Reservation.sol:1370-1373` | #1094 | invariant | yes | no | A pending action can never be stranded: its BTC tx may already be confirmed. |
| Requires wallet `Terminated` | `Reservation.sol:1374-1378` | #1094 | invariant | yes | no | |
| `strandReservation` body | `Reservation.sol:1442-1487` | #1094 | storage-write | yes | no | See §4.1-S1. |
| `evidenceAlreadyEmitted` latch suppresses duplicate `ReservationStranded` | `Reservation.sol:1447-1448`, `:1476-1484` | #1094 | invariant | yes | no | Reachable in m1 only via the late-re-anchor re-strand (`ReservationProofs.sol:239`). |
| `strandLateSettlementIfTargetWalletClosed` | `ReservationProofs.sol:218-243` | #1094 | internal | yes | no | **Shared between acceptance and re-anchor (rule 5): m1 work.** |
| `emit ReservationStranded` | `Reservation.sol:1478-1483` | #1094 | event | yes | no | Compensation evidence. |

---

## 7. Dissolution (m2) and renewal (m2)

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `ReservationRouter.requestReservationDissolution` | `ReservationRouter.sol:302-304` | #1090 | entry-point | no | yes | Permissionless. |
| `Reservation.requestReservationDissolution` | `Reservation.sol:887-953` | #1091/#1094 | entry-point | no | yes | Requires `Active` (`:891-894`), `>= dissolutionEligibleAt` (`:898-902`), wallet `Live`/`MovingFunds` (`:908-916`). |
| **`walletPendingDissolution[wallet] = key`** (per-wallet lock) | `Reservation.sol:917-921` | #1091 | storage-write | flagged | yes | See PD-5. |
| Snapshot incl. `actionDataHash = wallet.mainUtxoHash` | `Reservation.sol:931-943` (`:942`) | #1091 | storage-write | no | yes | |
| `submitReservationDissolutionProof` | `ReservationProofs.sol:921-998` | #1091 | entry-point | no | yes | |
| **SPV: `validateProof`** | `:957-960` | #1091 | internal | no | yes | |
| `processDissolutionInputs` | `:962-968`, body `:1366-1410` | #1091 | internal | no | yes | Dissolution-exclusive. |
| `consumeAnchorInputAt` | `:1413-1443`; writes `spentMainUTXOs` `:1436`, `delete reservationsByAnchorUtxo` `:1437` | #1091 | storage-write | no | yes | Dissolution-exclusive. |
| `consumeMainUtxoInputAt` | `:1446-1468`; writes `spentMainUTXOs` `:1465-1467` | #1091 | storage-write | no | yes | Dissolution-exclusive. |
| `validateDissolutionOutput` | `:1003-1020` | #1091 | view | no | yes | Uses shared `parseSingleOutput` (`:1010`). |
| `settleDissolution` | `:1030-1153` | #1091 | internal | no | yes | |
| **`wallet.mainUtxoHash = keccak(...)`** | `:1074-1076` | #1091 | storage-write | no | yes | Backing rejoins the pool. |
| **`delete wallet.mainUtxoHash`** (terminated) | `:1069` | #1094 | storage-write | no | yes | |
| `Wallets.rearmMovingFundsTimeout` | `:1077` | #1091 | internal | no | yes | |
| **Moved-funds sweep request on registry drift** (4 field writes + `pendingMovedFundsSweepRequestsCount++`) | `:1087-1106` (`:1106`) | #1091 | storage-write | no | yes | |
| `supersedeConflictingDissolution` | `:1055-1059`, body `:1156-1200` | #1091 | internal | no | yes | Dissolution-exclusive. |
| **`delete walletPendingDissolution`** | `:1126-1131` | #1091 | storage-write | no | yes | |
| **`action.state = Settled`** | `:1134` | #1091 | storage-write | no | yes | |
| **`strandReservation` / `closeReservation`** | `:1140` / `:1142` | #1091/#1094 | storage-write | no | yes | §4.1-S1b / C1b. |
| `emit ReservationDissolved` | `:1146-1151` | #1091 | event | no | yes | |
| **External call: `financeInKindFee(dissolutionFee)`** | `:993-997` | #1093 | internal | no | yes | **settlement-path.** |
| `Reservation.extendReservation` (renewal) | `Reservation.sol:1146-1204`; writes `expiresAt` `:1195`, `dissolutionEligibleAt` `:1196` | #1092 | entry-point | no | yes | |
| `ReservationVault.renewalsPaused` | `vault/ReservationVault.sol:92`, set `:222`, `:410`, `:416`; checked `:381` | #1092 | invariant | no | yes | **The only pause flag in the entire reservation surface.** It gates renewal initiation only. Rule 2 satisfied by construction. |

---

## 8. Internal helpers, shared vs deferred

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `loadSettleableAction` | `:186-206` | #1091 | internal | yes | no | **Shared** (all 4 types). |
| `strandLateSettlementIfTargetWalletClosed` | `:218-243` | #1094 | internal | yes | no | **Shared** (acceptance + re-anchor). |
| `prepareReservationForSettlement` | `:254-301` | #1091/#1094 | internal | yes | yes | **Shared** (redemption, re-anchor, dissolution). Signature changed by #1096. |
| `requireCurrentSourceAnchor` | `:308-313` | #1091 | view | yes | yes | **Shared** (3 sites). Deleted by #1096; see PD-9. |
| `consumeAcceptedDeposit` | `:400-432` | #1091/#1094 | internal | yes | no | **Acceptance-exclusive** but acceptance is m1. |
| `validateAnchorOutput` | `:437-452` | #1091 | view | yes | no | Acceptance-exclusive, m1. |
| `settleAcceptance` | `:458-592` | #1091 | internal | yes | no | Acceptance-exclusive, m1. |
| `parseSingleOutput` | `:1315-1330` | #1091 | view | yes | no | **Shared** (acceptance, redemption, re-anchor, dissolution). |
| `consumeAnchor` | `:1334-1358` | #1091 | internal | yes | no | **Shared** (redemption + re-anchor); re-anchor is m1, so m1 work. |
| `unwindPendingAction` | `:1243-1311` | #1091 | internal | yes | yes | **Shared.** m1 needs the Acceptance (`:1293-1304`), Reanchor (`:1273-1281`) and `Superseded` (`:1259`) branches; the Redemption (`:1261-1272`) and Dissolution (`:1282-1292`) branches are m2-only and **may be dropped from m1** (rule 5's inverse). |
| `resolveLateRedemptionAgainstPending` | `:1206-1232` | #1091 | internal | no | yes | **Redemption-exclusive: m1 may drop.** |
| `supersedeConflictingDissolution` | `:1156-1200` | #1091 | internal | no | yes | **Dissolution-exclusive: m1 may drop.** |
| `validateDissolutionOutput` | `:1003-1020` | #1091 | view | no | yes | **Dissolution-exclusive: m1 may drop.** |
| `processDissolutionInputs` | `:1366-1410` | #1091 | internal | no | yes | **Dissolution-exclusive: m1 may drop.** |
| `consumeAnchorInputAt` | `:1413-1443` | #1091 | internal | no | yes | **Dissolution-exclusive: m1 may drop.** |
| `consumeMainUtxoInputAt` | `:1446-1468` | #1091 | internal | no | yes | **Dissolution-exclusive: m1 may drop.** |
| `settleDissolution` | `:1030-1153` | #1091 | internal | no | yes | **Dissolution-exclusive: m1 may drop.** |
| `Reservation.anchorUtxoHash` | `Reservation.sol:382-396` | #1091 | view | yes | no | **Shared.** |
| `Reservation.actionKey` | `Reservation.sol:335-345` | #1091 | view | yes | no | **Shared.** |
| `Reservation.getAction` | `Reservation.sol:371-379` | #1091 | view | yes | no | **Shared.** |
| `Reservation.addWalletReservationKey` | `Reservation.sol:1405-1414` | #1094 | internal | yes | no | **Shared.** |
| `Reservation.removeWalletReservationKey` | `Reservation.sol:1418-1437` | #1094 | internal | yes | no | **Shared.** |
| `Reservation.strandReservation` | `Reservation.sol:1442-1487` | #1094 | internal | yes | no | **Shared**, m1-reachable. |
| `Reservation.closeReservation` | `Reservation.sol:1490-1506` | #1091 | internal | flagged | yes | **Shared between two m2 actions only.** Rule 5 says an m1/m2-shared helper is m1 work, but both callers are m2. See PD-3. |
| `retireRetryCreditForGeneration` | partial `:702-715` | #1096 | internal | no | yes | Redemption-exclusive. |
| `settleWholeRedemption` / `settlePartialRedemption` / `validatePartialOutputs` | partial `:720-786` / `:795-876` / `:882-931` | #1096 | internal | no | yes | Redemption-exclusive. |
| `Reservation.consumeRetryCredit` | partial `Reservation.sol:860-895` | #1096 | internal | no | yes | Redemption-exclusive. |
| `Reservation._requestReservedRedemption` | partial `Reservation.sol:723-852` | #1096 | internal | no | yes | Redemption-exclusive. |

---

## 9. Settlement-path functions that must never be pause-gated

Rule 2 in full. Every function below participates in settling or accounting for a confirmed Bitcoin spend. Verified: **no pause check exists on any of them today** — grep for `pause` across `Reservation.sol`, `ReservationProofs.sol`, `ReservationRouter.sol` returns only doc-comment references (`Reservation.sol:1119`, `ReservationRouter.sol:378`), and the single real flag `renewalsPaused` (`vault/ReservationVault.sol:92`) is checked only at `:381`, inside renewal.

**External entry points**
- `ReservationRouter.submitReservationProof` — `ReservationRouter.sol:322`
- `ReservationRouter.notifyReservationActionTimeout` — `ReservationRouter.sol:351`
- `ReservationRouter.notifyReservationStranded` — `ReservationRouter.sol:461`
- `ReservationRouter.notifyReservedRedemptionVeto` — `ReservationRouter.sol:366` (m2)

**Library dispatch and per-type proof handlers**
- `ReservationProofs.submitReservationProof` — `:134`
- `submitReservationAcceptanceProof` — `:343`
- `submitReservationReanchorProof` — `:743`
- `submitReservedRedemptionProof` — `:612` (m2)
- `submitReservationDissolutionProof` — `:921` (m2)

**Settle and accounting internals**
- `settleAcceptance` — `:458`
- `settleDissolution` — `:1030` (m2)
- `settleWholeRedemption` / `settlePartialRedemption` — partial `:720` / `:795` (m2)
- `unwindPendingAction` — `:1243` (moves Bank balance at `:1269`)
- `prepareReservationForSettlement` — `:254`
- `strandLateSettlementIfTargetWalletClosed` — `:218`
- `supersedeConflictingDissolution` — `:1156` (m2)
- `resolveLateRedemptionAgainstPending` — `:1206` (m2)
- `consumeAcceptedDeposit` — `:400`; `consumeAnchor` — `:1334`; `consumeAnchorInputAt` — `:1413`; `consumeMainUtxoInputAt` — `:1446`
- `Reservation.closeReservation` — `Reservation.sol:1490`; `Reservation.strandReservation` — `:1442`
- `Reservation.notifyReservationActionTimeout` — `Reservation.sol:972` (refunds escrow `:1022`, seizes stake `:1016`/`:1042`)
- `Reservation.notifyReservationStranded` — `Reservation.sol:1363`
- `Reservation.notifyReservedRedemptionVeto` — `Reservation.sol:1078` (m2, moves balance `:1113`)

**Value-moving calls that must always succeed**
- `bank.increaseBalanceAndCall` — `ReservationProofs.sol:574` (acceptance mint)
- `bank.decreaseBalance` — `:727`; partial `:784`, `:863` (redemption burn, m2)
- `bank.transferBalance` — `:1269` (unwind refund); `Reservation.sol:1022` (timeout refund); `Reservation.sol:1113` (veto detain, m2)
- **`IReservationFeeFinancer.financeInKindFee`** — `ReservationProofs.sol:875` (re-anchor, **m1**) and `:996` (dissolution, m2). This is the only settlement-path call that leaves the Bridge for a governance-controlled contract. Verified safe: `ReservationVault.financeInKindFee` (`vault/ReservationVault.sol:529-557`) has exactly one `require` (caller is the Bridge, `:530`), returns early on zero (`:532-533`), and books any reserve shortfall as `inKindFeeDebtSat` (`:556`) instead of reverting; the rationale is stated at `:523-528` — "a confirmed Bitcoin spend must never fail to settle because of the reserve level." **The m1 rewrite must preserve this no-revert property.**

**Initiation-path only; a pause flag MAY gate these**
- `Reservation.requestReservationAcceptance` — `Reservation.sol:426`
- `Reservation.requestReservationReanchor` — `Reservation.sol:771`
- `Reservation.requestReservationDissolution` — `Reservation.sol:887` (m2)
- `Reservation.requestReservedRedemption` / `requestPartialReservedRedemption` and `bank.transferBalanceFrom` — `Reservation.sol:625`, `:743`; partial `:685`, `:851` (m2)
- `Reservation.extendReservation` — `Reservation.sol:1146` (m2; this is what `renewalsPaused` already gates)

---

## Open questions

**DECISION NEEDED PD-1 — action-record fields for deferred action types.**
Rule 1 says storage-complete means written, not merely declared. `ReservationAction.isPartial` (partial `Reservation.sol:263`) and `retryCreditSourceNonce` (`:268`) are only ever written on the redemption request path (partial `:817`, `:825`), which is m2. No m1 code path can write them, and no m1-created action record (Acceptance or Reanchor) is ever read for them: `resolveLateAgainstPending` reads `isPartial` only after confirming `actionType == Redemption` (partial `:1434-1435`). **Does rule 1 bind per-generation action-record fields, or only the long-lived `ReservationRequest` position record?** If it binds action records, m1 must ship a write it has no semantics for.

**DECISION NEEDED PD-2 — `retryCredit` and `reservationRetryCreditActionNonce` in m1.**
`reservation.retryCredit` (`Reservation.sol:180`) is a *position* field, so rule 1 points at m1. But its only mint sites are the redemption timeout arm (`Reservation.sol:1009-1012`; partial adds the nonce write at `:1157-1161`) and `unwindPendingAction`'s Redemption branch (`:1263-1264`; partial `:1487-1491`), both m2. Its only consumer is the redemption request path (`Reservation.sol:671-674`; partial `consumeRetryCredit:860-895`), also m2. `reservationRetryCreditActionNonce` (partial `BridgeState.sol:461`) is the same shape. **Must m1 declare and write these, and if so, from where?** A declared-but-never-written field is a rule-1 violation as stated; a write with no m1 semantics is worse.

**DECISION NEEDED PD-3 — does `closeReservation` ship in m1?**
Rule 5 says a helper shared between an m1 action and an m2 action is m1 work. `closeReservation` (`Reservation.sol:1490-1506`) is shared between two **m2** actions only (`ReservationProofs.sol:715` redemption, `:1142` dissolution). Rule 5 does not cover a helper shared exclusively between deferred actions. Shipping it in m1 leaves dead code that will trip a linter and an auditor; omitting it makes the m2 diff touch `Reservation.sol` again. **Ship the orphan, or defer it?** Same question for `Reservation.notifyReservedRedemptionVeto` and the `Vetoed` enum member.

**DECISION NEEDED PD-4 — re-anchor cannot unpin a healthy Live wallet even after rule 4.**
Rule 4 deletes the `block.timestamp < dissolutionEligibleAt` gate (`Reservation.sol:785-788`) so re-anchor is unbounded in time. It does **not** touch the wallet-state gate (`Reservation.sol:790-806`): a re-anchor still requires the source wallet to be `MovingFunds`, or `Live` with `privileged == true`, where `privileged` is `msg.sender == governance` (`ReservationRouter.sol:293`). And the target must be `Live` with a free slot (`Reservation.sol:820-827`). So in m1 a position on a healthy `Live` wallet is unpinnable **only by a governance transaction**, and only if some other Live wallet has a free slot. **Is governance-only rotation the intended m1 unpin path, or should rule 4 also relax the source-wallet gate?** If the former, the m1 spec should say so explicitly, because §4.2 shows there is no other exit.

**DECISION NEEDED PD-5 — `walletPendingDissolution` write in m1.**
Rule 1 would put the `walletPendingDissolution[wallet] = key` write (`Reservation.sol:921`) in m1, but the only writer is `requestReservationDissolution` (m2) and the only readers are m2 (`:918`, `:1033`, `ReservationProofs.sol:1127`, `:1286`, `:1156-1170`). It is a per-wallet action lock, not position state, so no m1-era position can carry one. **Does rule 1 reach per-wallet action locks?**

**DECISION NEEDED PD-6 — `expiresAt` and `dissolutionEligibleAt` are written but unread in m1.**
Both are written at acceptance (`ReservationProofs.sol:533`, `:537-539`), which rule 1 requires. In m1 their only readers are deleted: `expiresAt` is read by the redemption expiry gate (`Reservation.sol:666-670`) and by renewal (`:1165`), both m2; `dissolutionEligibleAt` is read by the re-anchor gate (`:786`, deleted by rule 4) and the dissolution gate (`:900`, m2). This is the rule working as intended, but it means an m1 position advertises an expiry that nothing enforces. **Should m1 emit `ReservationAccepted.expiresAt` (`ReservationProofs.sol:558`) unchanged, knowing off-chain consumers will read it as an enforced deadline that m1 does not enforce?**

**DECISION NEEDED PD-7 — `ProofType` enum stability across the milestone boundary.**
`enum ProofType { Acceptance, Redemption, Reanchor, Dissolution }` (`:117-122`) is ABI-encoded as `uint8` by the router (`ReservationRouter.sol:323`). m1 has no Redemption or Dissolution handler. **Does m1 keep the four-member enum with two arms reverting on `ProofType(1)`/`ProofType(3)`, or shrink to `{Acceptance, Reanchor}` and renumber?** Renumbering silently changes the meaning of `proofType == 1` for any client built against m1, and the dispatcher's `else` fallthrough (`:169`) means an out-of-range value would be routed rather than rejected.

**DECISION NEEDED PD-8 — which `spentMainUTXOs` lineage does the m1 rewrite take?**
`consumeAnchor` writes `spentMainUTXOs[anchorUtxoKey] = true` (`:1356`) on the settlement line. Separately, `feature-spec.md:1067-1069` records that #1102 "moved anchor consumption to `spentMainUTXOs`" on `feat/utxo-reservation-core`, and that a reverse index used by `strandReservation` "exists on `feat/utxo-reservation-guards` but was deleted from `feat/utxo-reservation-core` by the #1102 merge." Only the guards and partial trees are available locally, so **the exact shape of the #1102 core-line implementation is `UNVERIFIED`** and the two lineages have not been reconciled anywhere I can read. **Which lineage is the m1 rewrite's base?** The answer changes whether `strandReservation`'s `delete self.reservationsByAnchorUtxo[...]` (`Reservation.sol:1462-1472`) still has an index to delete from.

**DECISION NEEDED PD-9 — `requireCurrentSourceAnchor`: keep the guards helper or take #1096's fold?**
Guards keeps a standalone `requireCurrentSourceAnchor` (`:308-313`) called at three sites (`:665`, `:778`, `:955`), each *after* `prepareReservationForSettlement`. #1096 deletes the helper and folds the check to the *top* of `prepareReservationForSettlement` (partial `:270-273`), changing its signature to take `action`. This reorders two reverts: partial checks the source anchor **before** the settleability state check (partial `:277-282`), guards checks it after (`:262-267` then `:665`). Acceptance calls neither (it has no source anchor). **Which shape does m1 take?** Taking the guards shape means m2's #1096 rebase must redo the fold; taking the partial shape means m1 ships a signature that only #1096 motivates, and the revert-string ordering visible to clients differs from every reviewed branch below #1096.

**DECISION NEEDED PD-10 — `unwindPendingAction`'s deferred branches.**
Rule 5 makes `unwindPendingAction` (`:1243`) m1 work because m1 reaches it from late acceptance (`:490`) and late re-anchor (`:821`). But two of its four branches are unreachable in m1: Redemption (`:1261-1272`, refunds escrow) and Dissolution (`:1282-1292`, releases the wallet lock). **Does m1 ship the full four-branch body, or only the Acceptance and Reanchor branches?** Shipping all four requires PD-2 and PD-5 to be answered first, since the Redemption branch writes `retryCredit` and the Dissolution branch writes `walletPendingDissolution`.

**DECISION NEEDED PD-11 — the `restoreRetryCredit` argument at the late-re-anchor call site.**
`:821` passes `true` for `restoreRetryCredit`. That argument is consumed only inside the Redemption branch (`:1262`), so in m1 it is provably inert. **Does m1 keep the `restoreRetryCredit` parameter at all?** Dropping it makes the m2 restoration (partial `:1448-1453`, which passes `action.isPartial`) a signature change rather than an argument change.

**DECISION NEEDED PD-12 — the spec's closing-site list is wrong and is load-bearing.**
`m1-variant-comparison.md:251-253` cites `ReservationProofs.sol:715`/`:836` as "the redemption path (unreachable)". `:836` is the re-anchor source-wallet count decrement and **is reachable in m1** (§4.3-W1). The §5.3 endgame conclusion still holds, but the cited evidence does not. **Should the spec be corrected before the m1 rewrite is cut**, given that §5.4's proposed `activeReservationsCount` mitigation is sized against this list?