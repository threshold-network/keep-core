# UTXO Reservation Milestone Inventory

Status: DRAFT - synthesized 2026-08-21 from seven verified inventory fragments.

This document is the single feature ledger that answers "is anything falling through the crack between milestone 1 and milestone 2". It covers the Solidity surface (§2.1-§2.8), the keep-core wallet side (§2.9), and the non-code obligations other documents in this set carry (§2.10).

Milestone 1 is a decided essentials-only REWRITE of the tbtc-v2 UTXO-reservation feature ("variant B with a minimal router"). It ships creation, custody and re-anchoring, and omits dissolution, redemption and renewal. Milestone 2 restores the rest. The decision is settled; this document does not re-litigate it.

## 1. Preamble

### 1.1 What this ledger is for

Every item the m1 rewrite must ship, declare, or build is listed below with its source, PR attribution, and milestone assignment. The four trap sections (3 through 6) catch what a straightforward extraction would miss: items with no extraction source, fields carried for layout only, writes that look dead but are load-bearing, and claims in the existing docs that are wrong. Section 2.10 catches the other class of loss: obligations that are not code rows at all.

### 1.2 The m1/m2 assignment rules (established facts, compressed)

1. `ReservationVault` is NOT upgradeable. Re-pointing needs `reservationTotalAmount == 0 && pendingReservedDeposits == 0`, unreachable in m1. **Superseded 2026-09-24 ("Option B"; see §1.4 and D-12/D-13 in §7).** The M1 vault does not ship the full entry-point surface behind pause flags: it ships MINIMAL (create/custody/re-anchor accounting only — `financeInKindFee`, `repayInKindFeeDebt`, `updateFeeReserveTarget`, `sweepFees`, `updateInitiationFee`, `receiveBalanceIncrease`, `receiveBalanceApproval`). M2 delivers redemption and renewal via a NEW vault deployment plus a depositor migration ceremony, not by unpausing this vault. The old argument for the flag design (avoid a migration) is void: because the vault is not upgradeable, M2 needs the migration ceremony anyway. (vault.md section 5; m1-b-implementation.md section 1 — both historical estimates; see §1.4, §2.4)

2. A pause flag may gate initiation, NEVER settlement or accounting. `financeInKindFee` is on the re-anchor settlement path and must stay unconditionally callable. This rule holds in the M1 code via a different route than expected: the M1 vault has no pause flag of any kind gating `receiveBalanceIncrease`, because Option B removed the initiation-path entry points (redemption, renewal) it would otherwise have gated, rather than adding a flag to them. (vault.md section 2; m1-b-implementation.md section 3 — historical)

3. Router code is Bridge code reached by `delegatecall`, replaceable by a Bridge implementation upgrade m2 needs anyway, so router entry points may be omitted freely. The design-time estimate was that m1 would keep 21 of 24 (9 state-changing + 12 views; this corrects this document's own earlier arithmetic error). **M1 code ships 22** (11 state-changing + 11 views) — a different count and composition than either design-time estimate: `forceStaleReservedDeposit` was added (in neither estimate), `walletReservations` was dropped, and the single design-time `submitReservationProof` dispatcher was split into two router entry points. See §1.4, §2.3.

4. `WalletProposalValidator` is non-upgradeable as well, making it a third layer with its own rules distinct from router and vault. (touchpoints.md; `WalletProposalValidator.sol:33-34`)

5. Storage rule, in its sharp form: a field written by an m1-reachable path must keep being written; a field written only by an m2-exclusive path need only be declared. Enum variants must keep their numeric positions, so all three enums are extracted verbatim including m2-only variants. (data-model.md section 1, section 3)

6. In variant B exactly two position-closing sites are reachable and both end in `Stranded`, never `Closed`. A position held by a healthy `Live` wallet has no closing path at all. (proofs.md section 4.2)

### 1.3 Provenance caveat

All fragment line numbers come from the `feat/utxo-reservation-guards` tip (PR #1094), which predates the `#1102` fold (commit `3566e059`). The fold added +685 -190 across 10 production files, `Reservation.sol` most of all (+342 -95). The guards tip does NOT contain `#1102`'s 30 review fixes. Every line number cited from the fragments is a pre-fix line number. The parameter validation block in particular should be re-read post-fold before implementation. (pr-map.md section 3)

**The fold's risk is not only stale line numbers — it also hides whole fields
from this ledger (added 2026-08-21).** Section 2.1 was built from the guards tip,
so three fields the `#1102` fold introduced are absent from it entirely.
Verified on `origin/feat/utxo-reservation-core`, which carries the fold:

| Field | Where | Kind | Why it matters |
|---|---|---|---|
| `ReservationRequest.cumulativeReanchorFee` | `Reservation.sol:142` | position field | **The load-bearing one.** Rule 5 and `m1-b-implementation.md` section 4.5 both require every field m2 will read to exist at m1. A position field missing from the m1 layout cannot be added in m2 without exactly the live-state migration section 4.5 forbids |
| `BridgeState.Storage.maxCumulativeReanchorFee` | `BridgeState.sol:417` | governance parameter | The cap bounding re-anchor fee grinding; enforced at `Reservation.sol:1111-1113` (`cumulativeReanchorFee += fee`, then `<= maxCumulativeReanchorFee`) |
| `BridgeState.Storage.reservationDissolutionTxMaxFee` | `BridgeState.sol:396` | governance parameter | Second fee term in the residual-backing formula `pr-review-followups.md` item 7 derives |

Two consequences. First, `updateReservationParameters` takes more arguments on
the core line than the nine visible on guards (`Reservation.sol:248` and `:253`
are two of the extra ones), so an m1 signature copied from guards is already
incomplete. Second, and more seriously, **until section 2.1 is re-derived from a
rebased tip the storage layout behind section 4.5's launch gate is not verified
complete** — the gate can pass against an incomplete inventory. Re-read
`Reservation.sol` and `BridgeState.sol` post-fold and add these three rows before
treating section 4.5 as satisfied. (`pr-review-followups.md` items 5 and 7;
decision D-24 covers the rebase that makes this checkable.)

PR attribution caveat: attributions to #1088/#1090 through #1094 are derived from PR titles plus explicit file-and-line attributions in `roadmap.md` and `m1-variant-comparison.md`. Items from `inventory/vault.md` carry `?`-qualified PR confidence because the source files contain no PR references. (proofs.md PR-to-branch map; vault.md PR origin note)

### 1.4 Status vs M1 code (2026-09-28)

This ledger was synthesized 2026-08-21 from seven pre-rewrite inventory fragments (§ Provenance) as a planning artifact: what the m1 rewrite *should* ship. The actual M1 code now exists — tbtc-v2 `reservations-upgrade` @ `9f8f5ef1`, keep-core `reservations-epic` @ `f66f11240` — **implemented on integration branches, not merged to `dev`/`main`, not audited, not deployed** (tracker PRs: tbtc-v2 #1116, keep-core #4282, both OPEN drafts). Where the M1 code differs from what this ledger predicted or recommended, the divergence is corrected inline throughout §§1-7 (marked **M1 code:**) and summarized here for a reader who only reads this banner.

- **Router surface**: M1 code ships **22 entry points = 11 state-changing + 11 views** (`ReservationRouter.sol:211-598`), not the 20 or 21 this document and its siblings estimated at design time (§1.2 rule 3). `forceStaleReservedDeposit` (`onlyGovernance`, `:399-402`) is a real, new state-changing entry point with no design-time counterpart, documented nowhere in this doc set before this pass. The design-time single `submitReservationProof` router dispatcher was split into two public router functions, `submitReservationAcceptanceProof` and `submitReservationReanchorProof` (`:252-304`); the internal `ReservationProofs.submitReservationProof` dispatcher (§2.2) still exists but is not itself a router entry point. The `walletReservations` per-wallet key-list view was dropped along with its backing storage (see storage, below). See §2.3.

- **Vault ("Option B", decided 2026-09-24)**: the M1 `ReservationVault.sol` (373 lines) ships **minimal**, not the full flagged/pausable surface §1.2 rules 1-2 originally described: 7 external functions plus constructor (`receiveBalanceIncrease`, `financeInKindFee`, `repayInKindFeeDebt`, `updateFeeReserveTarget`, `sweepFees`, `updateInitiationFee`, `receiveBalanceApproval`). There is no `redeemReservation`, `retryRedeemReservation`, `extendCustody`, or any renewal/redemption pause machinery (`renewalsPaused`, `renewalGuardian`, `redemptionsPaused`, etc.), and the vault is not proxy-upgradeable. M2 delivers redemption and renewal via a **new** vault deployment plus a depositor migration ceremony, not via unpause flags on this vault — this supersedes every statement in this document that assumed the flagged design (§1.2 rules 1-2, §2.4, §3, D-12, D-13, D-20, D-21). See §2.4, §3, §7.

- **Storage**: `BridgeState.Storage` packs per-wallet count and amount into a single `WalletReservationInfo{amount, count}` struct behind one `walletReservationInfo` mapping (`BridgeState.sol:61-64,471`), not the separate `walletReservationsCount`/`walletReservationsAmount` mappings this ledger described. `walletReservationKeys`, `walletReservationKeyIndex`, `reservationRetryCreditActionNonce`, `maxCumulativeReanchorFee`, `reservationDissolutionTxMaxFee`, and `reservationMaxBackingFractionBps` were all dropped as dead, freeing 6 slots into a `uint256[39] __gap` (`:491-502`) — not merely "declared for m2" as D-5 recommended for the retry-credit nonce. `ReservationRequest` gained two fields this ledger's own `#1102`-fold caveat (§1.3) did not fully anticipate: `cumulativeReanchorFee` (predicted) and `reanchorCooldownUntil` (not predicted). `ReservationAction` gained `termSeconds`, `dissolutionDelay`, and `minAmount` (none predicted by the ledger's own struct-field tables or any fragment). See §1.3, §2.1.

- **Governance validation**: `updateReservationParameters` and `updateReservationCaps` now validate what D-2 flagged as unvalidated: `maxReservationsPerWallet > 0` and `maxActiveReservations > 0` are direct requires, and `validateReservationCapsInvariant` (`Reservation.sol:1220-1232`) cross-checks `reservationMaxTotalAmount <= maxActiveReservations * reservationMaxSingleAmount`. Only `maxReservationsAmountPerWallet` remains genuinely unvalidated. See §2.1, §2.6, D-2.

- **Re-anchor floor and cooldown**: the dissolution-eligibility time gate on re-anchor was removed as designed (rule 4, PR "I" / #1120), but two mechanisms this ledger did not fully carry now bound it instead: a request-time amount floor `anchorAmount > txMaxFee + minAmount` (`Reservation.sol:805-808`, restored per adjudication reversing a review finding that had dropped the `minAmount` term) and a post-timeout `reanchorCooldownUntil` that blocks non-privileged re-anchor requests until it elapses (`:736-740`, set at `:905-907`). The non-privileged source-wallet gate also accepts `Closing`, not only `MovingFunds` (`:753-759`) — see D-9.

- **keep-core executor**: the keep-core tip is `reservations-epic` @ `f66f11240` (28 commits ahead of `#4238`/`4390a7998`, including merged PRs #4274, #4276-#4280, #4284, #4324), and it now HAS an executor: `pkg/tbtcpg/reservation_acceptance.go` and `reservation_reanchor.go` (proposal-generator tasks), a full set of chain-interface write methods (`RequestReservationAcceptance`, `SubmitReservation*Proof`, `NotifyReservation*`), real Ethereum bindings (`pkg/chain/ethereum/tbtc.go`), and dedicated watchers (`pkg/maintainer/spv/reservation_stranding_watch.go`, `reservation_stale_deposit_watch.go`, `reservation_action_timeout_watch.go`). §2.9's "error stub" / "build new" characterization is stale — see §2.9, C-8.

- **Line numbers not re-derived from the M1 code in this pass** (flagged for a future pass, out of this reconciliation's assigned scope): §2.2's action-lifecycle tables and §2.5's Bridge-integration tables still cite pre-`#1102` guards-tip line numbers verbatim for rows whose behavior is unchanged. Rows whose behavior differs materially in the M1 code carry their own inline **M1 code:** note with a current citation — including `notifyReservationStranded`'s precondition (§2.2: M1 also allows `Closed` and `Closing`-once-`dissolutionEligibleAt`, not `Terminated` alone), the re-anchor source-wallet gate and its new cooldown and amount-floor checks (§2.2), the `walletReservationInfo` storage packing, and the wallet-closing reservation gate's move from `beginWalletClosing`/`finalizeWalletClosing`/`notifyWalletFundsMoved` to the new `notifyWalletClosingPeriodElapsed` (§2.2, §2.5). `unwindPendingAction` ships only 2 of the 4 branches D-10 discussed keeping (§7).

## 2. Per-area ledger

Column key: `yes` = present and active in m1; `flagged` = present but initiation-disabled behind a pause flag; `declare` = declared in storage layout but not written by any m1 path; `no` = omitted in m1. `m2` = m2 must add or modify. Both `yes` means m1 ships it and m2 changes it.

### 2.1 Data model and storage

#### ReservationState enum (data-model.md section 3)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `ReservationState.Unknown` | `Reservation.sol:82` | #1088 | yes | yes | Zero value |
| `ReservationState.Active` | `:86` | #1088 | yes | yes | |
| `ReservationState.ActionPending` | `:91` | #1091 | yes | yes | Two-phase machine |
| `ReservationState.Closed` | `:95` | #1088 | declare | yes | Reached only by redemption or dissolution settlement, both m2 |
| `ReservationState.Stranded` | `:100` | #1094 | yes | yes | m1's only terminal state |

#### ActionType enum (data-model.md section 3)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `ActionType.None` | `Reservation.sol:105` | #1091 | yes | yes | Zero value |
| `ActionType.Acceptance` | `:106` | #1091 | yes | yes | |
| `ActionType.Redemption` | `:107` | #1091 | declare | yes | Never constructed in m1; keep numeric position |
| `ActionType.Reanchor` | `:108` | #1091 | yes | yes | |
| `ActionType.Dissolution` | `:109` | #1091 | declare | yes | Never constructed in m1; keep numeric position |

#### ActionState enum (data-model.md section 3)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `ActionState.Unknown` | `Reservation.sol:115` | #1091 | yes | yes | Zero value |
| `ActionState.Pending` | `:120` | #1091 | yes | yes | |
| `ActionState.Settled` | `:122` | #1091 | yes | yes | |
| `ActionState.TimedOut` | `:128` | #1091 | yes | yes | |
| `ActionState.Vetoed` | `:132` | #1091 | declare | yes | Veto is redemption-only |
| `ActionState.Superseded` | `:136` | #1091 | yes | yes | Reachable via conflicting re-anchor |

#### ReservationRequest struct fields (data-model.md section 4)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `owner` | `Reservation.sol:143` | #1088 | yes | yes | |
| `mintedAmount` | `:147` | #1093 | yes | yes | Backing invariant; read by vault (6 sites) |
| `acceptedAt` | `:150` | #1088 | yes | yes | |
| `walletPubKeyHash` | `:153` | #1088 | yes | yes | Custodian; re-anchor rewrites it |
| `anchorAmount` | `:157` | #1093 | yes | yes | Claim-equals-anchor; 21 read sites |
| `expiresAt` | `:161` | #1092 | yes | yes | Written by acceptance; no m1 reader. See section 5 |
| `anchorTxHash` | `:163` | #1088 | yes | yes | |
| `anchorTxOutputIndex` | `:166` | #1088 | yes | yes | m2 partial writes `1` (partial `:845`) |
| `state` | `:168` | #1088 | yes | yes | |
| `requestNonce` | `:173` | #1091 | yes | yes | Two-phase anti-replay; 16 sites |
| `retryCredit` | `:180` | #1091 | yes | yes | Written by timeout path (inert in m1); read by m2. See section 4 |
| `dissolutionEligibleAt` | `:187` | #1092 | yes | yes | Written by acceptance; no m1 reader. See section 5 |
| `cumulativeReanchorFee` | partial `Reservation.sol:220-237` (final: `:237`) | #1096/M1 | yes | yes | **M1 code:** present, appended at the struct's end; written on every re-anchor settlement, unread in m1. See §1.3, §1.4 |
| `reanchorCooldownUntil` | new, M1 code (`Reservation.sol:240`) | M1 | yes | yes | **M1 code:** not in any fragment or the `#1102` caveat. Set on a non-privileged re-anchor timeout (`:905-907`); read by the next non-privileged re-anchor request (`:736-740`). See §1.4 |

#### ReservationAction struct fields (data-model.md section 6)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `targetWalletPubKeyHash` | `Reservation.sol:202` | #1091 | yes | yes | Re-anchor destination |
| `requestedAt` | `:204` | #1091 | yes | yes | |
| `timeoutAt` | `:206` | #1091 | yes | yes | 5 read sites |
| `txMaxFee` | `:208` | #1091 | yes | yes | 16 sites |
| `actionType` | `:210` | #1091 | yes | yes | |
| `state` | `:212` | #1091 | yes | yes | |
| `feePaid` | `:216` | #1091 | declare | yes | Only writer is redemption (m2); read by m1 timeout. See section 4 |
| `redeemer` | `:219` | #1091 | declare | yes | Only writer is redemption (m2); read by m1 timeout. See section 4 |
| `amount` | `:223` | #1091 | yes | yes | |
| `actionDataHash` | `:229` | #1091 | yes | yes | Snapshot-at-request digest; 11 sites |
| `sourceAnchorUtxoHash` | `:233` | #1091 | yes | yes | Re-anchor source binding |
| `usedRetryCredit` | `:237` | #1091 | declare | yes | Only writer is redemption (m2); read by m1 helper. See section 4 |
| `watchtowerDefaultDelay` | `:242` | #1091 | declare | yes | Only writer/reader is `requestReservedRedemption:725` (m2) |
| `watchtowerLevelOneDelay` | `:245` | #1091 | declare | yes | Same, `:726` (m2) |
| `watchtowerLevelTwoDelay` | `:248` | #1091 | declare | yes | Same, `:727` (m2) |
| `isPartial` | partial `Reservation.sol:263` | #1096 | declare | yes | Only writer is redemption request (m2). See D-4 |
| `retryCreditSourceNonce` | partial `:268` | #1096 | declare | yes | Only writer is redemption request (m2). See D-4 |
| `termSeconds` | `Reservation.sol:324-329` | M1 code | yes | yes | **M1 code:** not in this table before (the design-time struct-field estimate under-counted this by one field — see `minAmount` below). Snapshotted at acceptance request; `settleAcceptance` (`ReservationProofs.sol:557`) computes `expiresAt` from this generation field instead of the live governance parameter. Zero for non-acceptance actions |
| `dissolutionDelay` | `Reservation.sol:330-334` | M1 code | yes | yes | **M1 code:** not in this table before. Snapshotted at acceptance request; `settleAcceptance` (`:570`) computes `dissolutionEligibleAt` from this generation field. Zero for non-acceptance actions |
| `minAmount` | `Reservation.sol:335-341` | M1 code | yes | yes | **M1 code:** not predicted by the design-time struct-field estimate or any fragment (that estimate only named `termSeconds`/`dissolutionDelay`; the M1 struct has a third addition). Snapshotted minimum reservation amount at acceptance request time so a later governance raise of `reservationMinAmount` cannot revert an already-broadcast, Bitcoin-confirmed anchor transaction. Placed last in the struct. See §1.4 |

#### BridgeState.Storage reservation fields (router.md section 3)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `reservationMinAmount` | `BridgeState.sol:354` | #1088 | yes | yes | param |
| `reservationTermSeconds` | `:358` | #1088 | yes | yes | param; the promise clock, no on-chain consumer in m1 |
| `reservationVault` | `:361` | #1088 | yes | yes | The re-point target |
| `reservationTxMaxFee` | `:365` | #1088 | yes | yes | param; no proposed value yet |
| `reservationDissolutionDelay` | `:372` | #1088 | yes | yes | param; feeds dissolutionEligibleAt write m1 must keep |
| `reservationMaxTotalAmount` | `:375` | #1088 | yes | yes | Global amount cap. **M1 code:** no direct require, but now cross-checked via `validateReservationCapsInvariant` against `maxActiveReservations`/`reservationMaxSingleAmount` (`Reservation.sol:1220-1232`, called at `:1330-1334`). See D-2 (partially resolved) |
| `reservationTotalAmount` | `:378` | #1088 | yes | yes | Global amount in use; read by re-point gate |
| `maxReservationsPerWallet` | `:380` | #1088 | yes | yes | Per-wallet slot cap. **M1 code:** now validated directly, `require(maxReservationsPerWallet > 0, ...)` (`Reservation.sol:1295-1298`). See D-2 (resolved for this field) |
| `reservations` | `:384` | #1088 | yes | yes | Position records |
| `reservationsByAnchorUtxo` | `:388` | #1091 | yes | yes | Reverse index; two write sites. See section 6, D-16 |
| `walletReservationsCount` | `:391` | #1088 | yes | yes | Per-wallet slot occupancy. **M1 code:** no separate mapping — packed into `walletReservationInfo[wallet].count` (`WalletReservationInfo` struct, `BridgeState.sol:61-64,471`). See §1.4 |
| `reservationRouter` | `:404` | #1090 | yes | yes | Router address, one-time settable |
| `reservationActionTimeout` | `:409` | #1088 | yes | yes | param |
| `reservationRenewalWindowSeconds` | `:415` | #1088 | yes | yes | param; written but unread in m1. See D-3 |
| `reservationActions` | `:423` | #1091 | yes | yes | Action records |
| `walletReservationsAmount` | `:435` | #1093 | yes | yes | Per-wallet amount. **M1 code:** packed into `walletReservationInfo[wallet].amount`, same struct as the count above. See §1.4 |
| `maxReservationsAmountPerWallet` | `:438` | #1093 | yes | yes | Per-wallet amount cap. **M1 code:** still NO validation of any kind — the one D-2 field the M1 code leaves unvalidated. See D-2 |
| `reservationMaxSingleAmount` | `:441` | #1093 | yes | yes | Single-position cap. **M1 code:** no direct require, but cross-checked as an operand of `validateReservationCapsInvariant` (called at `Reservation.sol:1392-1396`). See D-2 (partially resolved) |
| `walletReservationKeys` | `:446` | #1094 | yes | yes | Per-wallet key list. **M1 code: removed as dead** (`BridgeState.sol` `__gap` comment `:491-496`); no per-wallet enumeration mapping ships. See §1.4 |
| `walletReservationKeyIndex` | `:449` | #1094 | yes | yes | Key list index. **M1 code: removed as dead**, same `__gap` comment. See §1.4 |
| `__gap` | `:463` | #1090 | yes | yes | `uint256[34]` at this tip; 14 slots consumed per parity test |
| `reservationRetryCreditActionNonce` | partial `:461` | #1096 | declare | yes | Only writer is redemption timeout (m2). D-5 recommended declaring it for m2. **M1 code: removed as dead** (`__gap` comment `:491-496`), not merely declared — D-5's resolution was superseded by outright deletion. See §1.4, D-5 |
| `activeReservationsCount` | new | m1 | yes | yes | NEW: no extraction source. See section 3. **M1 code: confirmed present** (`BridgeState.sol:432-435`) |
| `maxActiveReservations` | new | m1 | yes | yes | NEW: no extraction source. See section 3. **M1 code:** confirmed present and validated, `require(maxActiveReservations > 0, ...)` (`Reservation.sol:1382-1385`) — the M1 launch gate |

Adjacent pre-existing fields the feature depends on: `liveWalletsCount` (`BridgeState.sol:253`) and `spentMainUTXOs` (`:336`).

### 2.2 Action lifecycles (proofs.md)

#### Acceptance - request side (proofs.md section 1a)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `requestReservationAcceptance` | `ReservationRouter.sol:242-247` | #1090 | yes | no | Permissionless wrapper, no modifier |
| `Reservation.requestReservationAcceptance` | `Reservation.sol:426-590` | #1091 | yes | no | Two-phase entry gate |
| Validates vault set, deposit revealed, not swept, `isReserved` | `Reservation.sol:431-444` | #1091/#1094 | yes | no | `isReserved` guard is #1094 |
| Validates designated wallet binding | `Reservation.sol:446-450`, `:472-475` | #1094 | yes | no | Duplicate require; harmless |
| Validates position `Unknown`, no pending acceptance | `Reservation.sol:453-461` | #1091 | yes | no | |
| Validates wallet `Live` | `Reservation.sol:477-481` | #1091 | yes | no | |
| Validates `amount >= minAmount + txMaxFee` | `Reservation.sol:483-487` | #1091 | yes | no | Makes proof-time fee bound sufficient |
| Validates signing window | `Reservation.sol:493-531` | #1091/#1094 | yes | no | vs `DEPOSIT_MIN_AGE`, timeout margin, refund deadline |
| Cap: single-reservation `reservationMaxSingleAmount` | `Reservation.sol:533-537` | #1093 | yes | no | |
| Reserves `reservationTotalAmount` | `Reservation.sol:541-546` | #1091 | yes | no | initiation-path |
| Reserves `walletReservationsCount` | `Reservation.sol:548-553` | #1091 | yes | no | initiation-path. **M1 code:** no separate mapping — packed into `walletReservationInfo[wallet].count`, written by the new `reserveAcceptanceCapacity` helper (`:920-958`), which also folds in the global `activeReservationsCount` cap check (see §3) |
| Reserves `walletReservationsAmount` | `Reservation.sol:555-562` | #1093 | yes | no | initiation-path. **M1 code:** same packed struct and helper, `.amount` field |
| `++reservation.requestNonce` | `Reservation.sol:564` | #1091 | yes | no | |
| Action snapshot (type, state, requestedAt, timeoutAt, txMaxFee, target, amount) | `Reservation.sol:571-578` | #1091 | yes | no | No sourceAnchorUtxoHash or actionDataHash: acceptance has no source anchor |
| `emit ReservationAcceptanceRequested` | `Reservation.sol:580-588` | #1091 | yes | no | |

#### Acceptance - proof side (proofs.md section 1b)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `submitReservationProof` dispatcher | `ReservationProofs.sol:134-180` | #1091 | yes | yes | m1 drops Redemption and Dissolution arms. See D-7 |
| `enum ProofType` | `:117-122` | #1091 | yes | yes | Keep numbering stable. See D-7 |
| `loadSettleableAction` | `:186-206` | #1091 | yes | no | Shared across all four action types |
| `submitReservationAcceptanceProof` | `:343-390` | #1091 | yes | no | |
| Requires position `Unknown` | `:360-364` | #1091 | yes | no | |
| Requires `pendingReservedDeposit.isReserved` | `:365-368` | #1094 | yes | no | |
| SPV: `validateProof` | `:370` | #1091 | yes | no | Existing BitcoinTx machinery |
| `consumeAcceptedDeposit` | `:400-432` | #1091/#1094 | yes | no | Acceptance-exclusive. Writes `deposit.sweptAt` (`:418`), deletes `pendingReservedDeposit` marker (`:427-429`), decrements `pendingReservedDeposits` (`:430`) |
| `validateAnchorOutput` | `:437-452` | #1091 | yes | no | Uses shared `parseSingleOutput` (`:1315-1330`) |
| `action.state = Settled` | `:467` | #1091 | yes | no | Only action.state transition on this path |

#### Acceptance - settlement and accounting (proofs.md section 1c, all settlement-path)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `settleAcceptance` | `:458-592` | #1091 | yes | no | |
| Late: unwind newer pending acceptance generation | `:469-492` | #1091 | yes | no | |
| Late: `reservationTotalAmount += anchorAmount` | `:498` | #1091 | yes | no | settlement-path; no cap re-check |
| Late: `walletReservationsCount[target] += 1` | `:499` | #1091 | yes | no | settlement-path |
| Late: `walletReservationsAmount[target] += anchorAmount` | `:500-502` | #1093 | yes | no | settlement-path |
| Late: `emit ReservationLateSettled` | `:503-507` | #1091 | yes | no | |
| Not late: `reservationTotalAmount -= (amount - anchorAmount)` | `:511` | #1091 | yes | no | Releases miner-fee delta. settlement-path |
| Not late: `walletReservationsAmount[target] -= (amount - anchorAmount)` | `:512-514` | #1093 | yes | no | settlement-path |
| Position creation: `owner` | `:527` | #1091 | yes | no | |
| `mintedAmount = anchorAmount` | `:528` | #1093 | yes | no | claim==anchor |
| `acceptedAt` | `:530` | #1091 | yes | no | |
| `walletPubKeyHash` | `:531` | #1091 | yes | no | |
| `anchorAmount` | `:532` | #1091 | yes | no | |
| `expiresAt` | `:533` | #1091 | yes | no | m2 renewal also writes it; m1 write is storage-complete. See section 5 |
| `anchorTxHash` | `:534` | #1091 | yes | no | |
| `anchorTxOutputIndex = 0` | `:535` | #1091 | yes | no | m2 partial writes `1` |
| `state = Active` | `:536` | #1091 | yes | no | |
| `dissolutionEligibleAt` | `:537-539` | #1093 | yes | no | Must write even though m1 deletes every reader. See section 5 |
| `reservationsByAnchorUtxo[keccak(txHash,0)] = key` | `:541-543` | #1091 | yes | no | Write site 1 of 2. See section 6 |
| `addWalletReservationKey` | `:544-548` | #1094 | yes | no | Enumeration. **M1 code:** this helper does not exist — replaced by `walletReservationInfo[wallet].count`/`.amount` increments (`Reservation.sol:940-957`, `reserveAcceptanceCapacity`). See §1.4 |
| `emit ReservationAccepted` | `:551-559` | #1091 | yes | no | Carries `expiresAt`. See D-6 |
| `bank.increaseBalanceAndCall(deposit.vault, ...)` | `:574-578` | #1088/#1091 | yes | no | Mint. settlement-path. Never pause-gate. Uses deposit's immutable vault, not live `reservationVault` |
| `strandLateSettlementIfTargetWalletClosed` | `:585-591` | #1094 | yes | no | Position-closing site; see section 4 of proofs.md |

#### Re-anchor - request side (proofs.md section 2a)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `requestReservationReanchor` | `ReservationRouter.sol:286-295` | #1090 | yes | no | Sets `privileged = msg.sender == governance` at `:293` |
| `Reservation.requestReservationReanchor` | `Reservation.sol:771-868` | #1091 | yes | no | |
| Requires `state == Active` | `Reservation.sol:781-784` | #1091 | yes | no | |
| Requires `block.timestamp < dissolutionEligibleAt` | `Reservation.sol:785-788` | #1094 | no | yes | DELETED in m1. m2 restores alongside dissolution |
| Source wallet must be MovingFunds, or Live with privileged | `Reservation.sol:790-806` | #1094 | yes | no | Not relaxed. See D-9. **M1 code:** the non-privileged branch is broader than described — it accepts a source wallet in `MovingFunds` OR `Closing` (`Reservation.sol:753-759`), not `MovingFunds` alone; `Live` still requires `privileged` via a distinct `require` (`:748-752`) |
| Non-privileged cooldown gate | none at this pin | M1 | yes | no | **M1 code:** new mechanism absent from this fragment's pin entirely — `require(block.timestamp >= reservation.reanchorCooldownUntil)` for non-privileged callers (`Reservation.sol:736-740`), set on a non-privileged re-anchor timeout (`:905-907`). See §1.4 |
| Target differs from source; target must be Live | `Reservation.sol:808-817` | #1091 | yes | no | |
| Request-time amount floor | none at this pin | M1 | yes | no | **M1 code:** new mechanism absent from this fragment's pin — `require(anchorAmount > txMaxFee + minAmount)` (`Reservation.sol:805-808`); the only on-chain bound on cumulative re-anchor fee grinding. See §1.4 |
| Reserves `walletReservationsCount[target]` | `Reservation.sol:820-827` | #1094 | yes | no | Cap check `<= maxReservationsPerWallet`. **M1 code:** no separate mapping — packed into `walletReservationInfo[target].count` (inline at `Reservation.sol:807-816`, no helper function) |
| Reserves `walletReservationsAmount[target]` | `Reservation.sol:829-837` | #1093 | yes | no | initiation-path. **M1 code:** same packed struct, `.amount` field (`:817-829`) |
| `state = ActionPending` | `Reservation.sol:839` | #1091 | yes | no | |
| `++requestNonce` | `Reservation.sol:840` | #1091 | yes | no | |
| Action snapshot incl. `sourceAnchorUtxoHash` | `Reservation.sol:847-858` | #1091 | yes | no | |
| `emit ReservationReanchorRequested` | `Reservation.sol:860-866` | #1091 | yes | no | |

#### Re-anchor - proof side (proofs.md section 2b)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `submitReservationReanchorProof` | `:743-899` | #1091 | yes | no | |
| `loadSettleableAction(Reanchor)` | `:753-758` | #1091 | yes | no | |
| `evidenceAlreadyEmitted = state == Stranded` | `:763-764` | #1094 | yes | no | |
| `prepareReservationForSettlement` | `:765-770`, body `:254-301` | #1091/#1094 | yes | yes | Shared across redemption, re-anchor, dissolution. Signature changed by #1096 |
| `requireCurrentSourceAnchor` | `:778`, body `:308-313` | #1091 | yes | yes | Shared helper. Deleted by #1096. See D-14 |
| SPV: `validateProof` | `:780` | #1091 | yes | no | |
| `consumeAnchor` | `:782`, body `:1334-1358` | #1091 | yes | no | Shared with redemption. Writes `spentMainUTXOs` (`:1356`), deletes `reservationsByAnchorUtxo` (`:1357`) |
| Output validation (single output, pays target, fee bound) | `:784-806` | #1091 | yes | no | |
| Dust floor `newAnchorAmount > txMaxFee` | `:803-806` | #1093 | yes | no | Preserves positive redemption value; keep even though m1 has no redemption |
| `action.state = Settled` | `:878` | #1091 | yes | no | |

#### Re-anchor - settlement and accounting (proofs.md section 2c, all settlement-path)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| Late: `walletReservationsCount[new] += 1` | `:812` | #1091 | yes | no | No cap check |
| Late: `walletReservationsAmount[new] += anchorAmount` | `:813-814` | #1093 | yes | no | |
| Late: unwind newer pending generation, `restoreRetryCredit = true` | `:818-822` | #1091/#1093 | yes | yes | `true` arg only matters for pending Redemption; inert in m1. See D-11 |
| Late: `emit ReservationLateSettled` | `:825-829` | #1091 | yes | no | |
| `walletReservationsCount[source] -= 1` | `:836` | #1091 | yes | no | Not a close. Net-zero move. See section 6 correction |
| `walletReservationsAmount[source] -= anchorAmount` | `:837-839` | #1093 | yes | no | |
| `walletReservationsAmount[new] -= (anchorAmount - newAnchorAmount)` | `:840-841` | #1093 | yes | no | Releases miner-fee delta on target |
| `reservationTotalAmount -= minerFee` | `:843`, `:846` | #1093 | yes | no | True backing reduction |
| `removeWalletReservationKey(source)` | `:848-852` | #1094 | yes | no | Enumeration move, not a close. **M1 code:** this helper does not exist — replaced by `walletReservationInfo[source].count`/`.amount` decrements (`Reservation.sol:976-983`, `releaseReanchorTargetCapacity`, symmetric on the target side). See §1.4 |
| `addWalletReservationKey(new)` | `:853-857` | #1094 | yes | no | **M1 code:** this helper does not exist — replaced by the packed-struct increment. See §1.4 |
| `reservation.walletPubKeyHash = new` | `:859` | #1091 | yes | no | |
| `reservation.anchorAmount = newAnchorAmount` | `:860` | #1091 | yes | no | |
| `reservation.mintedAmount = newAnchorAmount` | `:868` | #1093 | yes | no | Claim write-down; claim==anchor |
| `reservation.anchorTxHash = reanchorTxHash` | `:869` | #1091 | yes | no | |
| `reservation.anchorTxOutputIndex = 0` | `:870` | #1091 | yes | no | |
| `reservation.state = Active` | `:871` | #1091 | yes | no | Position stays open |
| `financeInKindFee(minerFee)` | `:873-876` | #1093 | yes | no | settlement-path. Never pause-gate. Verified non-reverting |
| `reservationsByAnchorUtxo[keccak(reanchorTxHash,0)] = key` | `:880-882` | #1091 | yes | no | |
| `emit ReservationReanchored` | `:885-891` | #1091 | yes | no | |
| `strandLateSettlementIfTargetWalletClosed` | `:893-899` | #1094 | yes | no | Position-closing site |

#### Timeout and slashing (proofs.md section 5)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `notifyReservationActionTimeout` | `ReservationRouter.sol:351-356` | #1090 | yes | yes | Permissionless |
| `Reservation.notifyReservationActionTimeout` | `Reservation.sol:972-1062` | #1091 | yes | yes | m1 keeps Acceptance + Reanchor arms only |
| Requires pending action and elapsed `timeoutAt` | `Reservation.sol:987-991` | #1091 | yes | no | |
| `action.state = TimedOut` | `Reservation.sol:993` | #1091 | yes | no | Enables late settlement |
| Acceptance arm: capacity release | `Reservation.sol:997-1005` | #1091 | yes | no | No slashing; position never existed |
| Reanchor arm: `state = Active`, target capacity release | `Reservation.sol:1023-1029` | #1091 | yes | no | No slashing |
| Redemption arm: `state = Active`, retry credit, slash, refund | `Reservation.sol:1007-1022` | #1091 | no | yes | Unreachable in m1. See D-1, D-5 |
| Dissolution arm: slash, terminate | `Reservation.sol:1035-1053` | #1094 | no | yes | Unreachable in m1 |
| `emit ReservationActionTimedOut` | `Reservation.sol:1057-1061` | #1091 | yes | no | |
| `beginWalletClosing` requires `walletReservationsCount == 0` | `Wallets.sol:674-677`, `:706-709` | #1094 | yes | no | The pin. Blocks orderly retirement. **M1 code:** this row is now wrong for `beginWalletClosing`/`finalizeWalletClosing` — neither has a reservation check at all (`Wallets.sol:646-658`, `:665-676`). The gate moved to a new function, `notifyWalletClosingPeriodElapsed` (`Wallets.sol:360-386`), which requires `walletReservationInfo[wallet].count == 0` immediately before calling `finalizeWalletClosing`. A wallet CAN now enter `Closing` with open reservations; it can only be finalized to `Closed` once they clear. See §2.5 |
| Same guard in `moveFunds` | `Wallets.sol:627-630` | #1094 | yes | no | **M1 code:** shape unchanged (`mainUtxoHash == 0 && walletReservationInfo[wallet].count == 0` skips MovingFunds and closes immediately, `Wallets.sol:613-620`), packed struct only |
| Same guard in `notifyWalletFundsMoved` | `Wallets.sol:437-441` | #1094 | yes | no | **M1 code:** this row is wrong — `notifyWalletFundsMoved` (`Wallets.sol:405-436`) has no reservation check at all; it unconditionally calls `beginWalletClosing`, which (per the row above) no longer gates on reservations either. The wallet enters `Closing` regardless of open reservations |
| `notifyWalletMovingFundsTimeout` -> slash + terminate | `Wallets.sol:493-516` | pre-existing | yes | no | The only slashing path reachable in m1, fed indirectly |

#### Stranding (proofs.md section 6)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `notifyReservationStranded` | `ReservationRouter.sol:461-463` | #1090 | yes | no | Permissionless, no modifier |
| `Reservation.notifyReservationStranded` | `Reservation.sol:1363-1381` | #1094 | yes | no | |
| Requires `state == Active` | `Reservation.sol:1370-1373` | #1094 | yes | no | |
| Requires wallet `Terminated` | `Reservation.sol:1374-1378` | #1094 | yes | no | **M1 code:** the precondition is broader than this row states — the M1 `notifyReservationStranded` (`Reservation.sol:1090-1113`) accepts `Terminated` OR `Closed` OR (`Closing` AND `block.timestamp >= reservation.dissolutionEligibleAt`), not `Terminated` alone. A `Closing` wallet can now hold an active reservation at all because the wallet-closing gate that used to block it moved (see §2.5's Wallets.sol note) |
| `strandReservation` body | `Reservation.sol:1442-1487` | #1094 | yes | no | Count, amount, total decrement; `removeWalletReservationKey`; `state = Stranded`; `delete reservationsByAnchorUtxo` |
| `strandLateSettlementIfTargetWalletClosed` | `ReservationProofs.sol:218-243` | #1094 | yes | no | Shared between acceptance and re-anchor |
| `emit ReservationStranded` | `Reservation.sol:1478-1483` | #1094 | yes | no | Compensation evidence |

#### Position-closing sites (proofs.md section 4)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `closeReservation` body | `Reservation.sol:1490-1506` | #1091 | flagged | yes | Both call sites are m2. See D-8 |
| `closeReservation` call in redemption settlement | `ReservationProofs.sol:715` | #1091 | no | yes | Unreachable in variant B |
| `closeReservation` call in dissolution settlement | `ReservationProofs.sol:1142` | #1091 | no | yes | Unreachable in variant B |
| `strandReservation` from `notifyReservationStranded` | `Reservation.sol:1380` | #1094 | yes | no | REACHABLE: wallet Terminated + Active |
| `strandReservation` from late acceptance | `ReservationProofs.sol:241` via `:585` | #1094 | yes | no | REACHABLE: late + target Closing/Closed |
| `strandReservation` from late re-anchor | `ReservationProofs.sol:241` via `:893` | #1094 | yes | no | REACHABLE: late + target Closing/Closed/Terminated |
| `strandReservation` in `settleDissolution` (terminated wallet) | `ReservationProofs.sol:1140` | #1094 | no | yes | Unreachable in variant B |
| Re-strand residual after late partial redemption | partial `:873-874` | #1096 | no | yes | Unreachable in variant B |

#### Non-closing count/amount mutations (proofs.md section 4.3)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| Re-anchor source count `-= 1` | `:836` | #1091 | yes | no | Net-zero move; not a close |
| Re-anchor source amount `-=` | `:837-839` | #1093 | yes | no | |
| Re-anchor target amount `-= minerFee` delta | `:840-841` | #1093 | yes | no | |
| Re-anchor `reservationTotalAmount -= minerFee` | `:846` | #1093 | yes | no | True backing reduction |
| `unwindPendingAction` Reanchor branch | `:1276-1281` | #1091 | yes | no | Releases reserved capacity |
| `unwindPendingAction` Acceptance branch | `:1298-1304` | #1091 | yes | no | |
| `unwindPendingAction` `state = Superseded` | `:1259` | #1091 | yes | no | |
| `unwindPendingAction` Redemption branch | `:1261-1272` | #1091 | no | yes | Unreachable in m1 |
| `unwindPendingAction` Dissolution branch | `:1282-1292` | #1091 | no | yes | Unreachable in m1 |
| Acceptance-timeout capacity release | `Reservation.sol:1001-1005` | #1091 | yes | no | |
| Re-anchor-timeout target capacity release | `Reservation.sol:1026-1029` | #1091 | yes | no | |
| `prepareReservationForSettlement` stranded reconstruction | `:290-300` | #1094 | yes | no | Inverse of a close |
| `notifyStaleReservedDeposit`: `pendingReservedDeposits -= 1` | `Reservation.sol:1343` | #1094 | yes | no | Not a position |
| `consumeAcceptedDeposit`: `pendingReservedDeposits -= 1` | `:430` | #1094 | yes | no | |

#### Shared internal helpers (proofs.md section 8)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `loadSettleableAction` | `:186-206` | #1091 | yes | no | Shared (all 4 types) |
| `strandLateSettlementIfTargetWalletClosed` | `:218-243` | #1094 | yes | no | Shared (acceptance + re-anchor) |
| `prepareReservationForSettlement` | `:254-301` | #1091/#1094 | yes | yes | Shared (redemption, re-anchor, dissolution). Signature changed by #1096 |
| `requireCurrentSourceAnchor` | `:308-313` | #1091 | yes | yes | Shared (3 sites). Deleted by #1096. See D-14 |
| `consumeAcceptedDeposit` | `:400-432` | #1091/#1094 | yes | no | Acceptance-exclusive but acceptance is m1 |
| `validateAnchorOutput` | `:437-452` | #1091 | yes | no | Acceptance-exclusive, m1 |
| `settleAcceptance` | `:458-592` | #1091 | yes | no | Acceptance-exclusive, m1 |
| `parseSingleOutput` | `:1315-1330` | #1091 | yes | no | Shared (acceptance, redemption, re-anchor, dissolution) |
| `consumeAnchor` | `:1334-1358` | #1091 | yes | no | Shared (redemption + re-anchor); re-anchor is m1 |
| `unwindPendingAction` | `:1243-1311` | #1091 | yes | yes | Shared. m1 needs Acceptance, Reanchor, Superseded branches. See D-11 |
| `Reservation.anchorUtxoHash` | `Reservation.sol:382-396` | #1091 | yes | no | Shared |
| `Reservation.actionKey` | `Reservation.sol:335-345` | #1091 | yes | no | Shared |
| `Reservation.getAction` | `Reservation.sol:371-379` | #1091 | yes | no | Shared |
| `Reservation.addWalletReservationKey` | `Reservation.sol:1405-1414` | #1094 | yes | no | Shared. **M1 code:** does not exist; superseded by `walletReservationInfo` packed-struct writes throughout `Reservation.sol`. See §1.4 |
| `Reservation.removeWalletReservationKey` | `Reservation.sol:1418-1437` | #1094 | yes | no | Shared. **M1 code:** does not exist; same replacement. See §1.4 |
| `Reservation.strandReservation` | `Reservation.sol:1442-1487` | #1094 | yes | no | Shared, m1-reachable |
| `Reservation.closeReservation` | `Reservation.sol:1490-1506` | #1091 | flagged | yes | Both callers are m2. See D-8 |
| `resolveLateRedemptionAgainstPending` | `:1206-1232` | #1091 | no | yes | Redemption-exclusive; m1 may drop |
| `supersedeConflictingDissolution` | `:1156-1200` | #1091 | no | yes | Dissolution-exclusive; m1 may drop |
| `validateDissolutionOutput` | `:1003-1020` | #1091 | no | yes | Dissolution-exclusive; m1 may drop |
| `processDissolutionInputs` | `:1366-1410` | #1091 | no | yes | Dissolution-exclusive; m1 may drop |
| `consumeAnchorInputAt` | `:1413-1443` | #1091 | no | yes | Dissolution-exclusive; m1 may drop |
| `consumeMainUtxoInputAt` | `:1446-1468` | #1091 | no | yes | Dissolution-exclusive; m1 may drop |
| `settleDissolution` | `:1030-1153` | #1091 | no | yes | Dissolution-exclusive; m1 may drop |

#### Settlement-path functions that must never be pause-gated (proofs.md section 9)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `submitReservationProof` (dispatcher) | `:134` | #1091 | yes | yes | Never pause-gate |
| `notifyReservationActionTimeout` | `ReservationRouter.sol:351` | #1090 | yes | yes | Never pause-gate |
| `notifyReservationStranded` | `ReservationRouter.sol:461` | #1090 | yes | no | Never pause-gate |
| `settleAcceptance` | `:458` | #1091 | yes | no | Never pause-gate |
| `unwindPendingAction` | `:1243` | #1091 | yes | yes | Moves Bank balance at `:1269` |
| `prepareReservationForSettlement` | `:254` | #1091 | yes | yes | Never pause-gate |
| `strandLateSettlementIfTargetWalletClosed` | `:218` | #1094 | yes | no | Never pause-gate |
| `consumeAcceptedDeposit` | `:400` | #1091/#1094 | yes | no | Never pause-gate |
| `consumeAnchor` | `:1334` | #1091 | yes | no | Never pause-gate |
| `strandReservation` | `Reservation.sol:1442` | #1094 | yes | no | Never pause-gate |
| `notifyReservationActionTimeout` (library) | `Reservation.sol:972` | #1091 | yes | yes | Refunds, seizes |
| `notifyReservationStranded` (library) | `Reservation.sol:1363` | #1094 | yes | no | Never pause-gate |
| `bank.increaseBalanceAndCall` | `:574` | #1091 | yes | no | Acceptance mint |
| `bank.transferBalance` | `:1269` | #1091 | yes | yes | Unwind refund |
| `financeInKindFee` | `:875` | #1093 | yes | no | Re-anchor settlement. Must never revert. Verified safe at `ReservationVault.sol:529-557` |

### 2.3 Router surface (router.md)

**M1 code note (§1.4).** This subsection was written from the pre-`#1102` `feat/utxo-reservation-guards` design (PR #1094) and predates the actual M1 rewrite. The M1 `ReservationRouter.sol` has a different composition than either the design-time estimate below or the corrected design-time count (21 = 9+12): it ships **22 entry points = 11 state-changing + 11 views**. There is no `submitReservationProof` router entry point (that name is only the internal `ReservationProofs` dispatcher — see §2.2) and no `walletReservations` view. `forceStaleReservedDeposit` is a real, `onlyGovernance` M1 entry point with no design-time counterpart below.

#### Retained in m1 - state-changing (design-time estimate, 8; M1 code ships 10 of these plus 1 new) (router.md section 1)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `requestReservationAcceptance` | `ReservationRouter.sol:242` | #1091 | yes | yes | The product's entry gate |
| `requestReservationReanchor` | `:286` | #1091 | yes | yes | variant B's only unpin path; load-bearing |
| `submitReservationAcceptanceProof` | `:252` (design-time: single `submitReservationProof`) | #1091 | yes | yes | **M1 code:** the design-time single dispatcher entry point was split into two router functions; this is one half. `onlySpvMaintainer` |
| `submitReservationReanchorProof` | `:278` (design-time: single `submitReservationProof`) | #1091 | yes | yes | **M1 code:** the other half of the split. `onlySpvMaintainer` |
| `notifyReservationActionTimeout` | `:304` | #1091 | yes | yes | Required cleanup, and the slashing path. **M1 code:** narrowed to the Reanchor arm only (`require(action.actionType == ActionType.Reanchor)`, `Reservation.sol:873-876`) — see `notifyReservationAcceptanceTimedOut` below and D-1 |
| `notifyReservationAcceptanceTimedOut` | `:323-326` | #1091 | yes | yes | **M1 code:** the design-time table omitted this; a distinct router entry point, not folded into `notifyReservationActionTimeout` |
| `notifyStaleReservedDeposit` | `:385` | #1094 | yes | yes | Releases un-accepted revealed deposits |
| `notifyReservationStranded` | `:414` | #1094 | yes | yes | variant B's only position-closing *entry point* (the second closing site is inside acceptance/re-anchor settlement) |
| `updateReservationParameters` | `:356` | #1088 | yes | yes | `onlyGovernance`; also carries the vault re-point |
| `updateReservationCaps` | `:434` | #1093 | yes | yes | `onlyGovernance`; the only safety valve at launch |

#### Retained in m1 - views (design-time estimate, 11; M1 code ships 10 of these plus 1 new — `activeReservationsCount`, in "Added" below) (router.md section 1)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `reservationCaps` | `:449` | #1093 | yes | yes | Cap readback |
| `walletReservationsAmount` | `:467` | #1093 | yes | yes | Per-wallet exposure |
| `walletReservationsCount` | `:478` | #1088 | yes | yes | Load-bearing: the free-slot monitor's data source |
| `reservationByAnchorUtxo` | `:492` | #1091 | yes | yes | Reverse index reader; two write sites. See D-16 |
| `reservedDepositWallet` | `:509` | #1094 | yes | yes | Reveal-time binding readback |
| `pendingReservedDeposits` | `:519` | #1094 | yes | yes | Load-bearing: read by the vault re-point gate |
| `reservations` | `:527` | #1088 | yes | yes | Position readback |
| `reservationActions` | `:539` | #1091 | yes | yes | Action readback |
| `reservationParameters` | `:551` | #1088 | yes | yes | Parameter readback |
| `reservationRouter` | `:595` | #1090 | yes | yes | Router address readback |

`walletReservations` (design-time `:524`, per-wallet key-list view) is **not in the M1 code**: the underlying enumeration mapping (`walletReservationKeys`) was dropped as dead, so there is nothing for the view to read.

#### Removed in m1 (5) (router.md section 1)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `requestReservedRedemption` | `:262` | #1091 | no | yes | Redemption deferred |
| `requestReservationDissolution` | `:302` | #1091 | no | yes | variant B's defining cut; permissionless, no modifier |
| `notifyReservedRedemptionVeto` | `:366` | #1091 | no | yes | Veto is vacuous with no redemptions |
| `extendReservation` | `:382` | #1092 | no | yes | Renewal deferred |
| `walletPendingDissolution` | `:600` | #1091 | no | yes | Dissolution view |

#### Added in m1 (design-time: 1; M1 code adds 2) (router.md section 1)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `activeReservationsCount` | new | m1 | yes | yes | Global position counter; genuinely new. See section 3 |
| `forceStaleReservedDeposit` | new, M1 code (`ReservationRouter.sol:399-402`) | M1 | yes | yes | **M1 code:** `onlyGovernance` state-changing entry point with no design-time counterpart in any fragment; not previously documented anywhere in `docs/spec/reservations/` before this pass. Manual governance release of a stale reserved deposit. See §1.4 |

Design-time arithmetic (corrected): "24 - 5 + 1 = 20" was wrong; the design intended 21 (9 state-changing + 12 views, matching `m1-b-implementation.md` §2 and `inventory/router.md`). **M1 code ships 22 (11 state-changing + 11 views)** — a different number from either design-time estimate, and a different composition (see the M1 code note above). See §1.4.

#### Delegatecall invariants (router.md section 2)

| Invariant | Mechanism | Test assertion | m1 |
|---|---|---|---|
| Storage parity | Router and Bridge share storage-bearing bases; router appends to `BridgeState.Storage` with `__gap` decrement | `"should consume exactly fourteen slots from the deployed Bridge gap"` (`ReservationRouter.test.ts:145`) | yes |
| Selector disjointness | `Bridge.fallback` only sees unmatched selectors | `describe("selector disjointness")` (`:374`), `"should revert unknown selectors"` (`:542`) | yes |
| No standalone authority | Router runs on empty storage when called directly | `describe("standalone router hardening")` (`:567`), `"should reject state-changing calls made directly"` (`:587`) | yes |
| One-time setter | `setReservationRouter` reverts once set | `describe("setReservationRouter")` (`:411`), two revert cases (`:450`, `:465`) | yes |

All four tests are extractable as-is. (router.md section 2)

### 2.4 Vault surface (vault.md)

**Superseded 2026-09-24 ("Option B"; see §1.4 and the README.md decision log).** This entire subsection was written for a design where every vault entry point ships in m1 behind a pause flag (§1.2 rules 1-2, pre-2026-09-24 text). The decision actually taken, and the M1 `ReservationVault.sol`, is different: the M1 vault ships MINIMAL. Redemption (`redeemReservation`, `retryRedeemReservation`) and renewal (`extendCustody` and its guardian/pause machinery) are not vault entry points at all in M1 — there is nothing to pause. M2 restores them via a *new* vault deployment plus a depositor migration ceremony, not by flipping a flag on this vault (the vault is intentionally not upgradeable). See D-12, D-13. The tables below are retained as the design-time record; every "flagged" verdict is corrected by an inline M1 code note rather than deleted, per this document's own evidence-doc editing policy.

#### Entry points (vault.md section 1)

| Item | Source | PR | m1 (design-time) | m1 (M1 code) | m2 | Note |
|---|---|---|---|---|---|---|
| `receiveBalanceIncrease` | `ReservationVault.sol:161` | ? | flagged | **yes, unconditional** | yes | `onlyBank`; initiation-path. **M1 code:** no pause guard of any kind. D-12 superseded |
| `redeemReservation` | design-time `:293` | ? | flagged | **no** | yes | **M1 code:** does not exist in the M1 vault. Deferred to m2's new vault deployment (Option B). D-13 superseded |
| `extendCustody` | design-time `:367` | ? | flagged | **no** | yes | **M1 code:** does not exist in the M1 vault |
| `pauseRenewals` | design-time `:409` | ? | yes | **no** | yes | **M1 code:** no renewal machinery ships at all |
| `unpauseRenewals` | design-time `:415` | ? | yes | **no** | yes | Same |
| `blockRenewal` | design-time `:424` | ? | yes | **no** | yes | Same |
| `unblockRenewal` | design-time `:432` | ? | yes | **no** | yes | Same |
| `setRenewalGuardian` | design-time `:440` | ? | yes | **no** | yes | Same |
| `retryRedeemReservation` | design-time `:469` | ? | flagged | **no** | yes | Same as `redeemReservation`. D-13 superseded |
| `financeInKindFee` | `ReservationVault.sol:215` | ? | yes | yes | yes | Bridge-only; settlement-path; must NOT be gated. LIVE in m1 via re-anchor |
| `repayInKindFeeDebt` | `:241` | ? | yes | yes | yes | Permissionless; settlement-path adjunct |
| `updateFeeReserveTarget` | `:278` | ? | yes | yes | yes | `onlyOwner`; accounting-path; required activation step |
| `sweepFees` | `:297` | ? | yes | yes | yes | `onlyOwner`; accounting-path |
| `updateFees` (design-time name) | design-time `:629` | ? | yes | renamed | yes | **M1 code:** ships as `updateInitiationFee(uint16)` (`:330`), a single-parameter setter — there is no separate renewal/redemption fee to update |
| `receiveBalanceApproval` | `:343` | ? | yes | yes | yes | Always reverts; required by `IVault`; accounting-path stub |
| `constructor` | — | ? | yes | yes | yes | Sets immutables and `initiationFeeBps`. **M1 code:** no `renewalsPaused = true` (no renewal field exists) |

#### Vault storage (vault.md full inventory)

| Item | Source | PR | m1 (design-time) | m1 (M1 code) | m2 | Note |
|---|---|---|---|---|---|---|
| `SATOSHI_MULTIPLIER` | `:50` | ? | yes | yes | yes | `uint256 public constant`; 10^10 |
| `BASIS_POINTS` | `:53` | ? | yes | yes | yes | `uint256 public constant`; 10000 |
| `MAX_FEE_BASIS_POINTS` | `:56` | ? | yes | yes | yes | `uint256 public constant`; 500 |
| `bank` | `:58` | ? | yes | yes | yes | `Bank public immutable` |
| `tbtcVault` | `:59` | ? | yes | yes | yes | `TBTCVault public immutable` |
| `tbtcToken` | design-time `:71` | ? | yes | yes | yes | `TBTC public immutable` |
| `bridge` | design-time `:72` | ? | yes | yes | yes | `IReservationBridge public immutable` |
| `initiationFeeBps` | `:66` | ? | flagged | **yes, live** | yes | **M1 code:** live from deployment, not dormant — there is no initiation pause to gate it |
| `extensionFeeBps` | design-time `:80` | ? | flagged | **no** | yes | **M1 code:** does not exist |
| `redemptionFeeBps` | design-time `:85` | ? | yes | **no** | yes | **M1 code:** does not exist |
| `renewalsPaused` | design-time `:92` | ? | yes | **no** | yes | **M1 code:** does not exist — no renewal machinery ships |
| `renewalBlocked` | design-time `:98` | ? | yes | **no** | yes | Same |
| `renewalGuardian` | design-time `:106` | ? | yes | **no** | yes | Same |
| `feeReserveTarget` | `:78` | ? | yes | yes | yes | LIVE in m1 via re-anchor |
| `inKindFeeDebtSat` | `:85` | ? | yes | yes | yes | LIVE in m1 via re-anchor |
| `redemptionsPaused` | design-time, new | m1 | yes | **not built** | yes | **M1 code:** never built — Option B (2026-09-24) makes it unnecessary; see §3 |

#### Vault invariants (vault.md invariants section)

| Item | Source | m1 | m2 | Note |
|---|---|---|---|---|
| Gross-mint invariant | `ReservationVault.sol:248-249` (design-time) | yes | yes | TBTC minted always equals sats on-chain; fee is explicit transfer, never netted |
| Settlement-never-reverts | `:526-528` (design-time) | yes | yes | Confirmed Bitcoin spend must never fail to settle; shortfall recorded as debt |
| Pause-is-monotonic | `:87-91`, `:100-105` (design-time) | yes | yes | **M1 code:** moot — the M1 vault has no pause/block/renewal fields at all; this invariant only applies once m2's new vault ships them |
| Re-point-quiescence | `Reservation.sol:1264-1270` (design-time) | yes | yes | Vault address can change only at total quiescence. **M1 code:** confirmed still enforced, now at `Reservation.sol:1307-1314` |
| Claim-equals-anchor | `ReservationProofs.sol:866-872` (design-time) | yes | yes | Re-anchor writes `mintedAmount` down to `newAnchorAmount` |

### 2.5 Bridge integration seams (touchpoints.md)

#### Deposit.sol - reveal-time classification

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `DepositRevealInfo.vault` field | `Deposit.sol:62` | #1088 | yes | yes | Optional vault routing address; zero for ordinary deposits |
| `DepositRequest.vault` field | `Deposit.sol:120` | #1088 | yes | yes | Stored on every deposit reveal at `:355` |
| `DepositRevealed` event includes `vault` param | `Deposit.sol:134` | #1088 | yes | yes | Last field in event, emitted at `:404` |
| `isReservedDeposit` local variable in `_revealDeposit` | `Deposit.sol:210-211` | #1094 | yes | yes | `reveal.vault != address(0) && reveal.vault == self.reservationVault` |
| Reserved-deposit refund deadline extraction | `Deposit.sol:220-226` | #1094 | yes | yes | Extracts refund deadline even when `depositRevealAheadPeriod == 0` |
| `pendingReservedDeposit` mapping write | `Deposit.sol:361-369` | #1094 | yes | yes | Writes `PendingReservedDeposit(true, walletPubKeyHash, refundDeadline, refundDeadlineValidated)` and increments `pendingReservedDeposits` |
| `deposit.vault = reveal.vault` storage write | `Deposit.sol:355` | #1088 | yes | yes | Every deposit stores its vault address |

#### Wallets.sol - wallet lifecycle coupling

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `notifyWalletFundsMoved`: reservation count check | `Wallets.sol:437-441` | #1094 | yes | yes | Requires `walletReservationsCount == 0` to begin closing. **M1 code:** wrong — `notifyWalletFundsMoved` (`Wallets.sol:405-436`) has no reservation check; it unconditionally calls `beginWalletClosing`. See the two rows below |
| `notifyWalletMovingFundsBelowDust`: calls `beginWalletClosing` | `Wallets.sol:478-489` | #1094 | yes | yes | `beginWalletClosing` reverts if `walletReservationsCount != 0`. **M1 code:** wrong — `beginWalletClosing` no longer reverts on open reservations (see below); this call proceeds unconditionally |
| `notifyWalletMovingFundsTimeout`: NO reservation check | `Wallets.sol:498-516` | #1094 | yes | yes | Critical m1 failure mode. Calls `seize` then `terminateWallet` unconditionally |
| `moveFunds`: reservation count check | `Wallets.sol:627-629` | #1094 | yes | yes | `mainUtxoHash == 0 && walletReservationsCount == 0` to skip MovingFunds. **M1 code:** same shape, packed into `walletReservationInfo[wallet].count == 0` (`Wallets.sol:613-620`) |
| `beginWalletClosing`: reservation count precondition | `Wallets.sol:674-677` | #1094 | yes | yes | `require(walletReservationsCount == 0)`. **M1 code:** wrong — the M1 `beginWalletClosing` (`Wallets.sol:646-658`) has no reservation check at all; it unconditionally sets the wallet to `Closing`. See the row below for where the gate now lives |
| `finalizeWalletClosing`: reservation count precondition | `Wallets.sol:706-709` | #1094 | yes | yes | `require(walletReservationsCount == 0)`. **M1 code:** wrong — `finalizeWalletClosing` (`Wallets.sol:665-676`) has no reservation check either. The gate moved to a new function not present in the design-time surface, `notifyWalletClosingPeriodElapsed` (`Wallets.sol:360-386`): it requires the wallet be `Closing`, its closing period elapsed, AND `walletReservationInfo[wallet].count == 0`, then calls `finalizeWalletClosing`. A `Closing` wallet can therefore still custody open reservations — this is what makes `notifyReservationStranded`'s widened `Closing` branch (§2.2) reachable |
| `terminateWallet` | `Wallets.sol:733-757` | pre-existing | yes | yes | No reservation cleanup; stranding handles that separately |

#### MovingFunds.sol

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `notifyMovingFundsBelowDust` docstring | `MovingFunds.sol:620-621` | #1094 | yes | yes | Reservation anchor precondition enforced by `beginWalletClosing`. **M1 code:** stale on both counts — the M1 docstring (`MovingFunds.sol:590-601`) no longer mentions reservations at all, and `beginWalletClosing` no longer enforces any reservation precondition (§2.5 Wallets.sol note above) |
| `notifyMovingFundsTimeout` entry point | `MovingFunds.sol:583-608` | pre-existing | yes | yes | NO reservation check in the timeout handler itself |

#### Redemption.sol

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `IRedemptionWatchtower.getReservedRedemptionDelay` | `Redemption.sol:74-77` | #1091 | no | yes | Returns veto delay for pending reserved redemption. m2 only |
| `IRedemptionWatchtower.getReservedRedemptionDelaySchedule` | `:87-94` | #1091 | no | yes | Three-level veto delay schedule. m2 only |
| `IRedemptionWatchtower.isSafeReservedRedemption` | `:105-108` | #1091 | no | yes | Safety check for reserved redemptions. m2 only |

Pooled redemption path (`Redemption.requestRedemption` at `:528`) has NO direct reservation coupling. Reserved deposits are excluded from the sweep path by the `isReservedDeposit` guard in `WalletProposalValidator.sol:312-315`.

#### WalletProposalValidator.sol

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| Import `IReservationBridge.sol` | `:25` | #1091 | yes | yes | Required for reservation proposal validators |
| Import `Reservation.sol` | `:26` | #1091 | yes | yes | Required for Reservation types |
| `DEPOSIT_MIN_AGE` constant | `:146-147` | pre-existing | yes | yes | 2 hours. Used by `validateReservationAnchorProposal` at `:1037` |
| `DEPOSIT_REFUND_SAFETY_MARGIN` constant | `:164-165` | pre-existing | yes | yes | 24 hours. Used by `validateReservationAnchorProposal` at `:1084` |
| `REDEMPTION_REQUEST_MIN_AGE` constant | `:179` | pre-existing | yes | yes | 600 seconds. Used by `validateReservedRedemptionProposal` at `:1135` |
| `REDEMPTION_REQUEST_TIMEOUT_SAFETY_MARGIN` constant | `:198-199` | pre-existing | yes | yes | 2 hours. Used by all four reservation proposal validators |
| `ReservationAnchorProposal` struct | `:918-927` | #1091 | yes | yes | Acceptance anchor proposal validation helper |
| `ReservedRedemptionProposal` struct | `:930-939` | #1091 | no | yes | m2 only. Non-upgradeable contract. See D-17 |
| `ReservationReanchorProposal` struct | `:943-954` | #1091 | yes | yes | Re-anchor proposal validation helper |
| `ReservationDissolutionProposal` struct | `:958-966` | #1091 | no | yes | m2 only. Non-upgradeable contract. See D-17 |
| `requirePendingAction` helper | `:975-989` | #1091 | yes | yes | Fetches action generation via `IReservationBridge`. Used by all four validators |
| `requireWalletLiveOrMovingFunds` helper | `:1291-1300` | #1094 | yes | yes | Used by anchor, redemption, dissolution validators. NOT by re-anchor |
| `validateDepositSweepProposal`: reserved deposit exclusion | `:312-315` | #1094 | yes | yes | `require(!bridge.isReservedDeposit(depositKeyUint))` |
| `validateReservationAnchorProposal` | `:1010-1094` | #1091 | yes | yes | Acceptance anchor proposal validation |
| `validateReservedRedemptionProposal` | `:1110-1170` | #1091 | no | yes | m2 only |
| `validateReservationReanchorProposal` | `:1186-1235` | #1091 | yes | yes | Re-anchor proposal validation |
| `validateReservationDissolutionProposal` | `:1250-1288` | #1091 | no | yes | m2 only |

#### Bridge.sol

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `isReservedDeposit` view | `Bridge.sol:1632-1638` | #1094 | yes | yes | Reads `pendingReservedDeposit[depositKey].isReserved` |
| `setReservationRouter` | `Bridge.sol:2114-2119` | #1090 | yes | yes | `external onlyGovernance`. One-time setter |
| `fallback()` delegatecall dispatcher | `:2135-2165` | #1090 | yes | yes | Routes unmatched selectors to `reservationRouter` via `delegatecall` |

#### WalletProposalValidatorConstants.sol

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `DEPOSIT_MIN_AGE` | `:11` | pre-existing | yes | yes | 2 hours |
| `DEPOSIT_REFUND_SAFETY_MARGIN` | `:12` | pre-existing | yes | yes | 24 hours |
| `REQUEST_TIMEOUT_SAFETY_MARGIN` | `:13` | pre-existing | yes | yes | 2 hours |

### 2.6 Governance and parameters (data-model.md section 7; touchpoints.md governance)

#### Parameter validation in `updateReservationParameters` (`Reservation.sol:1227-1295`)

| Check | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `reservationTxMaxFee > 0` | `:1239-1242` | #1088 | yes | yes | absolute |
| `reservationMinAmount > reservationTxMaxFee` | `:1243-1246` | #1088 | yes | yes | relational |
| `MIN_RESERVATION_TERM <= term <= MAX_RESERVATION_TERM` | `:1247-1251` | #1088 | yes | yes | absolute bounds |
| `0 < renewalWindow < term` | `:1252-1256` | #1092 | yes | yes | relational. See D-3 |
| `actionTimeout > REQUEST_TIMEOUT_SAFETY_MARGIN` | `:1257-1261` | #1088 | yes | yes | relational to a constant |
| Vault change requires `reservationTotalAmount == 0` and `pendingReservedDeposits == 0` | `:1263-1274` | #1088 | yes | yes | quiescence gate |
| `reservationMaxTotalAmount` assigned, NO require | `:1280` | #1088 | yes | yes | no validation at all. See D-2 |
| `maxReservationsPerWallet` assigned, NO require | `:1281` | #1088 | yes | yes | no validation at all. See D-2 |

#### `updateReservationCaps` (`Reservation.sol:1390-1401`)

| Check | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `maxActiveReservations` assigned | `:1399` | M1 code | yes | yes | **M1 code:** new field (§3), validated directly: `require(maxActiveReservations > 0, ...)` (`Reservation.sol:1382-1385`) — the M1 launch gate. D-2 resolved for this field |
| `maxReservationsAmountPerWallet` assigned | `:1397` | #1093 | yes | yes | **M1 code:** still NO validation of any kind. See D-2 |
| `reservationMaxSingleAmount` assigned | `:1398` | #1093 | yes | yes | **M1 code:** no direct require, but cross-validated as an operand of `validateReservationCapsInvariant` (called at `Reservation.sol:1392-1396`), same invariant as the parameters-setter row above. D-2 partially resolved |
| Emits `ReservationCapsUpdated` | `:1398` | #1093 | yes | yes | |

#### Governance wiring (touchpoints.md BridgeGovernance)

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| Import `IReservationBridge.sol` | `BridgeGovernance.sol:22` | #1090 | yes | yes | Required for `updateReservationParameters` and `updateReservationCaps` calls |
| `reservationData` storage variable | `:45` | #1090 | yes | yes | Stages all 9 reservation parameters |
| `reservationCapsData` storage variable | `:46` | #1090 | yes | yes | Stages the 2 reservation caps |
| `beginReservationParametersUpdate` | `:1843-1865` | #1090 | yes | yes | `onlyOwner`. Stages 9 parameters |
| `beginReservationCapsUpdate` | `:1871-1879` | #1090 | yes | yes | `onlyOwner`. Stages 2 caps |
| `finalizeReservationCapsUpdate` | `:1884-1892` | #1090 | yes | yes | `onlyOwner`. Enforces governance delay, calls `updateReservationCaps` |
| `finalizeReservationParametersUpdate` | `:1897-1912` | #1090 | yes | yes | `onlyOwner`. Enforces governance delay, calls `updateReservationParameters` |

No reservation parameter bypasses the governance delay. `setReservationRouter` (`Bridge.sol:2114`) is `onlyGovernance` but is a one-time setter, not a parameter update. (touchpoints.md governance summary)

#### BridgeGovernanceParameters.sol

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `ReservationData` struct | `:1575-1586` | #1090 | yes | yes | 9 staged parameter fields plus timestamp |
| `ReservationCapsData` struct | `:1588-1592` | #1090 | yes | yes | 2 staged cap fields plus timestamp |
| `beginReservationParametersUpdate` (library) | `:1616-1652` | #1090 | yes | yes | Stages all 9 parameters, sets timestamp, emits event |
| `beginReservationCapsUpdate` (library) | `:1656-1671` | #1090 | yes | yes | Stages both caps, sets timestamp, emits event |
| `finalizeReservationCapsUpdate` (library) | `:1678-1689` | #1090 | yes | yes | Enforces `onlyAfterGovernanceDelay`, clears timestamp |
| `finalizeReservationParametersUpdate` (library) | `:1695-1706` | #1090 | yes | yes | Enforces `onlyAfterGovernanceDelay`, clears timestamp |

#### Retroactivity

Parameters are stored on `BridgeState.Storage` and read live, NOT snapshotted per position, except where an action record snapshots a derived value at request time. So a governance raise applies to existing wallets immediately. The exceptions: `expiresAt` and `dissolutionEligibleAt` are computed once at acceptance (`ReservationProofs.sol:533`, `:537`) and never recomputed, which is what makes governance changes non-retroactive for already-granted terms. (data-model.md section 7)

### 2.7 Events (data-model.md section 8; vault.md events)

#### Reservation.sol events

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `ReservationAcceptanceRequested` | `Reservation.sol:251` | #1091 | yes | yes | |
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

#### ReservationVault.sol events

| Item | Source | PR | m1 (design-time) | m1 (M1 code) | m2 | Note |
|---|---|---|---|---|---|---|
| `ReservationCreditProcessed` | `ReservationVault.sol:87-91` | ? | flagged | **yes, live** | yes | Emitted in `receiveBalanceIncrease`. **M1 code:** not dormant — there is no initiation pause (D-12 superseded) |
| `CustodyExtended` | design-time `:130` | ? | flagged | **no** | yes | **M1 code:** `extendCustody` does not exist; event not present in the M1 code |
| `ReservedRedemptionInitiated` | design-time `:136` | ? | yes | **no** | yes | **M1 code:** `redeemReservation`/`retryRedeemReservation` do not exist; event not present in the M1 code |
| `FeesUpdated` | `:93` | ? | yes | yes | yes | **M1 code:** emitted by `updateInitiationFee` (design-time name `updateFees`) |
| `ReservationRenewalBlocked` | design-time `:149` | ? | yes | **no** | yes | **M1 code:** `blockRenewal` does not exist |
| `ReservationRenewalUnblocked` | design-time `:151` | ? | yes | **no** | yes | **M1 code:** `unblockRenewal` does not exist |
| `ReservationRenewalsPaused` | design-time `:153` | ? | yes | **no** | yes | **M1 code:** `pauseRenewals` does not exist |
| `ReservationRenewalsUnpaused` | design-time `:155` | ? | yes | **no** | yes | **M1 code:** `unpauseRenewals` does not exist |
| `RenewalGuardianUpdated` | design-time `:157` | ? | yes | **no** | yes | **M1 code:** `setRenewalGuardian` does not exist |
| `InKindFeeFinanced` | `:95` | ? | yes | yes | yes | Emitted in `financeInKindFee`; LIVE in m1 via re-anchor |
| `InKindFeeDebtRepaid` | `:97` | ? | yes | yes | yes | Emitted in `repayInKindFeeDebt` |
| `FeeReserveTargetUpdated` | `:99` | ? | yes | yes | yes | Emitted in `updateFeeReserveTarget` |
| `FeesSwept` | `:100` | ? | yes | yes | yes | Emitted in `sweepFees` |

Events are not storage, so omitting an m2 event from m1 costs nothing structural. But monitoring depends on them, and `ReservationStranded` is the only signal that an m1 position closed, so it is load-bearing for operational duties. (data-model.md section 8)

### 2.8 Tests (router.md section 5)

| File | Lines | Covers | m1 extraction value |
|---|---|---|---|
| `Bridge.Reservation.test.ts` | 3717 | Core lifecycle | Primary asset |
| `Bridge.ReservationSettlement.test.ts` | 3258 | Two-phase machine | Primary asset |
| `Bridge.ReservationGuards.test.ts` | 959 | Binding, stranding, pending-deposit guard | Primary asset |
| `Bridge.ReservationBacking.test.ts` | 748 | Claim-equals-anchor, in-kind fee | Primary asset |
| `ReservationRouter.test.ts` | 634 | The four delegatecall invariants | Extractable as-is |
| `Bridge.ReservationInvariants.test.ts` | 496 | Feature-level invariants | Primary asset |
| `Bridge.StorageLayout.test.ts` | 96 | Append-only layout parity | Extractable as-is |

9,908 lines of reservation test code exist. The m1 rewrite should treat these as the primary extraction asset, not the production code: tests encode the reviewed intent, and the ones covering m1 behaviour transfer with far less rework than the implementation does. (router.md section 5)

### 2.9 keep-core wallet-side, PR #4238 (keep-core.md)

A different repository (`pkg/tbtc`, `pkg/tbtcpg`, `pkg/chain/ethereum`, `pkg/maintainer/spv`), so it
carries no storage-layout constraint and has different rules. Design-time diffstat against
`main` for `#4238` alone: 11 files, +1833 -9. **M1 code note (§1.4):** the keep-core tip has
moved from `#4238` to `reservations-epic` @ `f66f11240` (28 commits ahead, including the merged
executor PRs #4274/#4276-#4280/#4284/#4324). The rows below are corrected against that tip.

| Item | Source | PR | m1 | m2 | Note |
|---|---|---|---|---|---|
| `ReservationState`, `ReservationActionType`, `ReservationActionState` enums | `reservation.go:30`, `:86`, `:98` | #4238 | yes | yes | Mirror the Solidity enums |
| `Reservation`, `ReservationAction`, `ReservationParameters` structs | `:56`, `:112`, `:147` | #4238 | yes | yes | Chain-state mirrors |
| `ReservationAnchorProposal` + 4 methods | design-time `:181-231` | #4238 | yes | yes | Carries `RequestNonce` |
| `ReservationReanchorProposal` + 4 methods | design-time `:287-340` | #4238 | yes | yes | Carries `RequestNonce`; m1's unpin |
| `ReservedRedemptionProposal` + 4 methods | design-time `:233-285` | #4238 | declare only | yes | **M1 code:** the struct and its methods were removed entirely (not merely left `declare only`); `ActionReservedRedemption` (wire value 7) is now an anonymous reserved slot (`wallet.go:37`), not a named constant. D-27 superseded |
| `ReservationDissolutionProposal` + 4 methods | design-time `:342-396` | #4238 | declare only | yes | **M1 code:** same treatment; `ActionReservationDissolution` (wire value 9) is an anonymous reserved slot (`wallet.go:39`). D-27 superseded |
| `assembleReservationAnchorTransaction` | design-time `:398` | #4238 | yes | yes | Bitcoin tx assembly |
| `assembleReservationReanchorTransaction` | design-time `:581` | #4238 | yes | yes | |
| `computeReservationRedeemerOutputScriptHash` | design-time `:557` | #4238 | yes | yes | Shared helper |
| `assembleReservedRedemptionTransaction` | design-time `:449` | #4238 | no | yes | **M1 code:** removed along with the redemption proposal type |
| `assembleReservationDissolutionTransaction` | design-time `:628` | #4238 | no | yes | **M1 code:** removed along with the dissolution proposal type |
| `GetReservation` | `chain.go:653-654` | #4238 | yes | yes | **M1 code:** implemented (`ethereum/tbtc.go:395-401`, calls `reservationRouter.Reservations`), not an error stub. |
| `GetReservationAction(key, nonce)` | `chain.go:656-660` | #4238 | yes | yes | Implemented. Two-phase construct; see C-8 |
| `ReservationParameters` | `chain.go:662-664` | #4238 | yes | yes | Implemented |
| `ValidateReservationAnchorProposal` | design-time `chain.go:449` | #4238 | yes | yes | **M1 code:** implemented (`ethereum/tbtc.go:482-570`, builds a real ABI proposal), not an error stub. |
| `ValidateReservationReanchorProposal` | design-time `chain.go:469` | #4238 | yes | yes | **M1 code:** implemented (`ethereum/tbtc.go:573-...`), not an error stub |
| `ValidateReservedRedemptionProposal` | design-time `chain.go` | #4238 | no | yes | **M1 code:** removed from the chain interface entirely (not present, not an error stub) — grep of `chain.go` and `tbtc.go` returns zero hits. |
| `ValidateReservationDissolutionProposal` | design-time `chain.go:477` | #4238 | no | yes | **M1 code:** same — removed from the interface |
| `ActionReservationAnchor` (6), `ActionReservationReanchor` (8) | `wallet.go:34-38` | #4238 | yes | yes | Plus string and metrics names |
| `ActionReservedRedemption` (7), `ActionReservationDissolution` (9) | design-time `wallet.go` | #4238 | declare only | yes | **M1 code:** the wire values are retained as anonymous reserved slots (`wallet.go:37,39`), but the named constants and their case arms were removed — "positional wire decoding" is now an explicit `_` placeholder, not a named case. D-27 superseded |
| Reservation proposal-generator task | `pkg/tbtcpg/reservation_acceptance.go`, `reservation_reanchor.go` | #4274 | **built** | yes | **M1 code:** built. `ReservationAcceptanceTask` and `ReservationReanchorTask` exist, each scanning candidates and emitting a proposal. |
| Chain-interface write methods (request, submit proof, notify timeout) | `chain.go:590-643`, `ethereum/tbtc.go:822-1195` | #4274 | **built** | yes | **M1 code:** built. `RequestReservationAcceptance`, `RequestReservationReanchor`, `SubmitReservationAcceptanceProof`, `SubmitReservationReanchorProof`, `NotifyReservationActionTimeout`, `NotifyReservationAcceptanceTimedOut`, `NotifyStaleReservedDeposit`, `NotifyReservationStranded` all implemented |
| Re-anchor executor on `WalletMovingFunds` | `pkg/tbtcpg/reservation_reanchor.go` | #4274 | **built** | yes | **M1 code:** built (`ReservationReanchorTask`) |
| Below-dust report after the last re-anchor | `roadmap.md` §0.8 | #4274 | — | yes | Not independently re-checked this pass; the reanchor task's dust handling is the likely home. Flag for a follow-up read if this matters operationally |
| Stranding watcher, stale-deposit cleanup, action-timeout watch | `pkg/maintainer/spv/reservation_stranding_watch.go`, `reservation_stale_deposit_watch.go`, `reservation_action_timeout_watch.go` | #4274/#4324 | **built** | yes | **M1 code:** built, with tests |

**The shape of the keep-core work described by this document (`#4238` is a
types-and-assembly foundation only) was correct at the time, but the gap it
described has since been closed.** `reservations-epic` @ `f66f11240` (28
commits ahead of `#4238`, including PRs #4274/#4276-#4280/#4284/#4324, all
merged) adds the missing executor: proposal-generator tasks per action type,
the full set of chain-interface write methods, and dedicated watchers for
stranding, stale-deposit cleanup and action timeouts. The closing claim that
keep-core work is "mostly new code, not rework" was accurate as a **prediction**
about work not yet done; it is no longer a live gap assessment — the work is
done. See C-8, §1.4.

### 2.10 Non-code obligations carried by other documents

Everything above is derived from source. The doc set also carries obligations
that are not code rows but would fall through the same crack, so they get the
same m1/m2/deferred verdict.

#### Review follow-ups (`pr-review-followups.md`)

| Item | Subject | m1 | m2 | Verdict |
|---|---|---|---|---|
| 1 | Wallet termination strands active reservations with no recovery path | **gate** | yes | **m1 gate.** Under B stranding is the only close path, so this is not a tail risk, it is the main path. Ties to D-9 and `roadmap.md` §0.8 |
| 2 | Vault rotation blocked while any reservation is outstanding | yes | yes | Confirmed contract-enforced (C-5). In m1 vault ships minimal (`4d549e64`); m2 vault rotation achieved via depositor opt-in migration ceremony (Option B, decided 2026-09-24) |
| 3 | No permissionless fallback if the SPV maintainer stalls | **gate** | yes | **m1 gate.** All proofs sit behind one `onlySpvMaintainer` (`ReservationRouter.sol:322`). Since re-anchor is B's sole unpin, a stalled maintainer freezes the only escape from §0.8's drain |
| 4 | Live (non-snapshotted) governance parameters applied retroactively | yes | yes | Partly mitigated: `expiresAt` and `dissolutionEligibleAt` are snapshotted per position, the caps are not (§2.6) |
| 5 | Unbounded re-anchor grinding | closed | - | Resolved directly in `#1088` (`d89a649a`) |
| 6 | Redeemer output-script check bypassable via P2SH/P2WSH | no | yes | Redemption-only, so out of m1 scope by construction. Must be closed before m2 enables redemption |
| 7 | `maxCumulativeReanchorFee` itself unbounded | yes | yes | **Touches m1 directly** - re-anchor is an m1 path and the cap is a constant rather than a ratio. Deferred to the `#1093` backing review; the four levers are in `feature-spec.md` §15 |

Items 1, 3 and 7 all bear on re-anchor, which is the single m1 path everything
else depends on. That concentration is worth stating plainly: **B narrows the
system to one unpin, so every open finding against that unpin is promoted.**

#### Stranding compensation (`stranding-compensation-proposal.md`)

| Item | m1 | m2 | Verdict |
|---|---|---|---|
| Tier 0 - measurement instrument (record every stranding, its cause and amount) | **yes** | yes | The accepted build per `exit/README.md`. B makes it load-bearing: stranding is the only close path, so Tier 0 is how the stranding-frequency number gets produced at all |
| Tier 1 - discretionary compensation process | yes | yes | Accepted alongside Tier 0 |
| Tiers 2+ | no | no | Not accepted |

#### Emergency exit (`exit/proposal.md`, `exit/alternatives.md`, `exit/addendum.md`)

**Explicitly deferred, not planned.** The 2026-08-21 decision is that
Mechanism 1 is *not built* and `Stranded` remains the fallback
(`exit/README.md`). These documents are retained as design reference. Reopen
only on evidence, which Tier 0 is what produces.

#### FROST interaction (`frost-reservations-interaction.md` §6)

| Item | m1 | m2 | Verdict |
|---|---|---|---|
| Track FROST activation before assuming a `Live` FROST wallet can be a re-anchor target | yes | yes | Operational; no code today |
| Patch `ReservationProofs.sol`'s four `extractPubKeyHash` call sites when FROST activates | no | yes | Doc-only until a FROST wallet exists to re-anchor into (`feature-spec.md` §17) |
| Storage and merge-conflict watch between the two PR stacks | yes | yes | Live concern for the rewrite, since both touch `BridgeState.Storage` |

## 3. m1 must build from scratch

Items with no extraction source anywhere in the stack. These must be written new in the m1 rewrite.

| Item | Source | Why new | Note |
|---|---|---|---|
| `activeReservationsCount` (storage field) | `BridgeState.sol` has no global position counter (router.md section 3) | No `activeReservationsCount`, `reservationsCount`, or `totalReservations` exists. Storage has a global amount (`reservationTotalAmount`, `:378`) and per-wallet counts (`walletReservationsCount`, `:391`), but no global count of open positions | The gate that converts variant B's saturation cliff into a revert (m1-b-implementation.md section 4.1). **M1 code: confirmed present** (`BridgeState.sol:432-435`) |
| `maxActiveReservations` (governance parameter) | new parameter | No global position cap exists today | Must sit below `liveWalletsCount x maxReservationsPerWallet` with margin (m1-b-implementation.md section 4.1). **M1 code: confirmed present**, validated `> 0` (`Reservation.sol:1382-1385`) |
| `activeReservationsCount` (router view) | router.md section 1, Added in m1 | No existing view to extract | Pairs with the storage field above. **M1 code: confirmed present** (`ReservationRouter.sol:583-586`) |
| Acceptance cap check on `activeReservationsCount` | new invariant in `requestReservationAcceptance` | No existing check prevents reaching saturation | Roughly 20 lines plus a parameter (m1-b-implementation.md section 4.1). **M1 code: confirmed present** (`reserveAcceptanceCapacity`, `Reservation.sol:920-958`, the cap check at `:925-928`) |

**Superseded 2026-09-24 ("Option B"; see §1.4 and the README.md decision log).** The three rows this table previously carried for `redemptionsPaused`/`pauseRedemptions`/`unpauseRedemptions` are deleted: M1 code does not build them, and building them would contradict the Option B decision. Redemption ships in m2 via a new vault deployment and a depositor migration ceremony instead. See §2.4, §7 (D-12, D-13).

## 4. Declare but do not write

Fields and enum variants m1 carries for layout reasons only. Their only writer is an m2-exclusive path, so no m1-era record is ever populated for them. They must exist in the storage layout but need not be written.

### 4.1 Enum variants (data-model.md section 3)

| Variant | Source | Only writer | Why declare-only |
|---|---|---|---|
| `ReservationState.Closed` | `Reservation.sol:95` | `closeReservation` (`:1505`), called only from redemption (`:715`) and dissolution (`:1142`) settlements, both m2 | Reached only by m2 paths. Removing it would renumber `Stranded` at position 4, silently reinterpreting stored records |
| `ActionType.Redemption` | `:107` | Never constructed in m1 | Removing it would renumber `Reanchor` and `Dissolution`, silently reinterpreting every stored action record |
| `ActionType.Dissolution` | `:109` | Never constructed in m1 | Same layout hazard |
| `ActionState.Vetoed` | `:132` | `notifyReservedRedemptionVeto` (`:1107`), m2 only | Veto is redemption-only. Keep numeric position for m2 |

### 4.2 ReservationAction struct fields (data-model.md section 6; proofs.md PD-1)

| Field | Source | Only writer | Only reader | Why declare-only |
|---|---|---|---|---|
| `watchtowerDefaultDelay` | `Reservation.sol:242` | `requestReservedRedemption:725` (m2) | `requestReservedRedemption` (m2) | No m1 action record is a redemption, so nothing is lost |
| `watchtowerLevelOneDelay` | `:245` | `:726` (m2) | `:726` (m2) | Same |
| `watchtowerLevelTwoDelay` | `:248` | `:727` (m2) | `:727` (m2) | Same |
| `isPartial` | partial `:263` | Redemption request (m2, partial `:817`, `:825`) | `resolveLateAgainstPending` after confirming `actionType == Redemption` (m2) | No m1 action record carries this. See D-4 |
| `retryCreditSourceNonce` | partial `:268` | Redemption timeout (m2, partial `:1157-1161`) and `unwindPendingAction` Redemption branch (m2) | Redemption request path (m2) | Same shape as `retryCredit`. See D-4, D-5 |

### 4.3 ReservationAction fields read by an m1 path but written only by m2 (data-model.md section 2)

These fields must be declared AND their m1 readers must be kept, even though m1 never writes them. The m1 timeout path reads them through the shared `unwindPendingAction` helper.

| Field | Source | Only writer | m1 reader | Note |
|---|---|---|---|---|
| `feePaid` | `Reservation.sol:216` | `requestReservedRedemption` (`:716`, m2) | `notifyReservationActionTimeout` (`Reservation.sol:1009`): `if (action.feePaid) {` | Refund branch. In m1 no action is fee-paid, so the branch is unreachable, but the helper is shared and stays |
| `redeemer` | `:219` | `requestReservedRedemption` (`:717`, m2) | `notifyReservationActionTimeout` (`:1022`): `self.bank.transferBalance(action.redeemer, action.amount)` | Refund target. Also 4 sites in `Redemption.sol` (m2) |
| `usedRetryCredit` | `:237` | `requestReservedRedemption` (m2) | `unwindPendingAction` (`ReservationProofs.sol:1262-1270`) reads `pendingAction.usedRetryCredit` | Shared helper must be extracted intact |

### 4.4 BridgeState and per-wallet fields (data-model.md; proofs.md PD-5)

| Field | Source | Only writer | Why declare-only |
|---|---|---|---|
| `reservationRetryCreditActionNonce` | partial `BridgeState.sol:461` | Redemption timeout (m2) and `unwindPendingAction` Redemption branch (m2) | No m1-era position carries one. See D-5 |

## 5. m1-reachable writes that m2 depends on

The opposite trap: where an essentials-only rewrite would drop a write as dead code because m1 has no reader for it. Dropping any of these is a silent repudiation rather than an optimisation, because m2 needs the value and the snapshot semantics that keep governance changes non-retroactive cannot be reconstructed without it.

### 5.1 The named traps (data-model.md section 5; m1-b-implementation.md section 4.4)

| Field | Written at | Why it looks dead in m1 | Why dropping it is a silent repudiation |
|---|---|---|---|
| `expiresAt` | `ReservationProofs.sol:533` (acceptance) | Every m1 reader is deleted: redemption's strict `block.timestamp < expiresAt` (`Reservation.sol:666-669`, deleted), renewal's reads (`Reservation.sol:1165`, `:1195`, deleted). No on-chain consumer in m1 | m2's redemption gates on `block.timestamp < expiresAt` (`Reservation.sol:666-669`, strict, #1093). Without the m1 write, m2's redemption has no expiry date for any m1-era position. The term is a commitment held in storage for m2 to honour, enforced by nothing in m1 |
| `dissolutionEligibleAt` | `ReservationProofs.sol:537-539` (acceptance) | Every m1 reader is deleted: re-anchor's `< dissolutionEligibleAt` gate (`Reservation.sol:786`, deleted by rule 4 to make re-anchor unbounded), dissolution's `>= dissolutionEligibleAt` (`:900`, m2), renewal (`:1196`, m2) | m2's dissolution needs it as the eligibility date. Computed as `expiresAt + reservationDissolutionDelay`, snapshotted once at acceptance and never recomputed. Without it, the snapshot semantics that keep governance changes non-retroactive (`:180-184`) cannot be reconstructed. Dropping the write means m2 has no eligibility date for any m1-era position |

### 5.2 The deeper reason (m1-b-implementation.md section 4.4; roadmap.md section 0.2)

In m1 variant B the custody term has no on-chain consumer at all. Every reader of `expiresAt` and `dissolutionEligibleAt` is cut or deleted: redemption's strict `< expiresAt`, renewal, dissolution's `>= dissolutionEligibleAt`, and re-anchor's `< dissolutionEligibleAt` (removed to make re-anchor unbounded). So the term is not enforced by anything in m1; it is a commitment held in storage for m2 to honour. The storage IS the promise, which makes dropping the write a silent repudiation of it rather than a code-size optimisation.

### 5.3 Other m1-reachable writes m2 depends on (proofs.md; data-model.md)

These are writes that look live in m1 (they have m1 readers) but are also load-bearing for m2 semantics. They are listed here for completeness but are less dangerous because their m1 readers make them obviously non-dead.

| Field | Written at | m1 reader | m2 dependency |
|---|---|---|---|
| `mintedAmount` | `:528` (acceptance), `:868` (re-anchor) | Claim invariant, vault (6 sites) | m2 redemption burns `action.amount` against it; claim-equals-anchor depends on it |
| `anchorAmount` | `:532` (acceptance), `:860` (re-anchor) | 21 read sites | m2 partial redemption updates it; m2 dissolution reads it |
| `requestNonce` | `:564` (acceptance), `:840` (re-anchor) | 16 sites, two-phase anti-replay | m2 redemption and dissolution increment it |
| `retryCredit` | `:1009-1012` (timeout, redemption arm only - unreachable in m1) | None in m1 (all readers are m2) | Edge case: its only writer other than m2 redemption is the timeout path. In m1 no action is fee-paid, so the write is unreachable, but the field is declared and the helper stays. See D-5 |

## 6. Corrections to existing docs

The fragments found several current claims to be wrong. Each is collected below as: the claim, the doc and line, what the source actually says, and the corrected statement.

### C-1: `reservationsByAnchorUtxo` / `#1102` removal story (pr-map.md section 4; roadmap.md section 4.3)

| | |
|---|---|
| **Claim** | `#1091` writes the mapping, `#1094` writes it again, and `#1102` removed it from the merged base in favour of `spentMainUTXOs`. Two write sites and one removal must be reconciled. |
| **Doc and line** | `m1-b-implementation.md` section 4.3 (lines at the `### 4.3` heading and the paragraph below it); `roadmap.md` section 4.3 item 3 (earlier text, since corrected in the current version) |
| **What the source says** | `reservationsByAnchorUtxo` does NOT exist on `#1088`'s branch at all. It is introduced by `#1091`. Therefore `#1102`, which merged into `#1088`'s branch, cannot have removed it - there was nothing there to remove. `spentMainUTXOs` is a pre-existing Bridge registry (6 mentions in `Reservation.sol` on `#1088`'s branch, writes at `:1454` and `:1510`), not a replacement introduced by `#1102`. The two are not competing designs: `spentMainUTXOs` is the Bridge's honestly-spent-outpoint registry that reservations write into, and `reservationsByAnchorUtxo` is a reverse index from anchor outpoint to reservation key that `#1091` adds. (pr-map.md section 4) |
| **Corrected statement** | `reservationsByAnchorUtxo` is introduced by `#1091` and has two write sites (`#1091` at `ReservationProofs.sol:541-543` and `#1094`'s stranding write at `Reservation.sol:1462-1472`). Both must be carried because stranding is variant B's only position-closing path. `#1102` did not remove it. `spentMainUTXOs` is a pre-existing Bridge registry, not a replacement. There is no removal to reconcile, only two write sites. |
| **Applied? (2026-09-28)** | Yes — `m1-b-implementation.md` §4.3 and `roadmap.md` §4.3 item 3 (lines ~666-685) both state "two write sites, no removal," matching this corrected statement verbatim. |

### C-2: `ReservationProofs.sol:836` cited as a close site in three places (proofs.md section 4.2)

| | |
|---|---|
| **Claim** | `ReservationProofs.sol:715`/`:836` are "the redemption path (unreachable)". |
| **Doc and line** | Three locations, all corrected 2026-08-21: `m1-variant-comparison.md:251-259` (section 5.3, step 1); `roadmap.md` section 0.7 (the "B's close sites are stranding alone" paragraph); `roadmap.md` section 6 (the "remaining close sites" paragraph). The first was found by proofs.md PD-12; the two `roadmap.md` occurrences were missed by that pass and found in the 2026-08-21 contradiction review. `roadmap.md` section 0.7 is the more dangerous of the three, because `m1-b-implementation.md` section 1 and this document's section 1.2 both defer to it for the close-site list. |
| **What the source says** | Guards `:836` is `self.walletReservationsCount[reservation.walletPubKeyHash] -= 1;` inside `submitReservationReanchorProof`, not a redemption site. It IS reachable in m1 (re-anchor settlement, proofs.md section 4.3-W1). It is not a close: it pairs with the target-wallet `+= 1` taken at request time (`Reservation.sol:827`), so global occupancy is unchanged and the position stays `Active` (`:871`). `:715` IS the redemption close site and IS unreachable in variant B. (proofs.md section 4.2) |
| **Corrected statement** | `ReservationProofs.sol:715` is the redemption close site (unreachable in variant B). `ReservationProofs.sol:836` is the re-anchor source-wallet count decrement inside `submitReservationReanchorProof` (`:743`), which IS reachable in m1 and must NOT be cut. It pairs with the target-wallet `+= 1` taken at request time (`Reservation.sol:820-827`), so it is a transfer between wallets, not a close: global occupancy is unchanged and the position stays `Active` (`:871`). It is also load-bearing for wallet retirement — it is the write that lets a re-anchored-away wallet's `walletReservationsCount` fall to zero, the precondition for `beginWalletClosing` (`Wallets.sol:674-677`) and for the below-dust exit in `roadmap.md` section 0.8. Cutting it would leave the source slot occupied forever, making occupancy monotonic even with re-anchor working. The conclusion each doc drew (occupancy is monotonic in B) survives, because a net-zero move does not free a slot. |
| **Applied? (2026-09-28)** | Yes, at the time of writing (self-reported "corrected 2026-08-21" at all three cited locations); not independently re-verified beyond that self-report in this pass. |

### C-3: The reach of the `#1102` fold (pr-map.md section 3; m1-b-implementation.md provenance)

| | |
|---|---|
| **Claim** | The `m1-b-implementation.md` provenance block states its citations are "source-verified on `feat/utxo-reservation-guards`", implying the guards tip contains `#1102`'s fixes. |
| **Doc and line** | `m1-b-implementation.md` Provenance section (final block); `feature-spec.md` (same provenance claim) |
| **What the source says** | `#1102` (commit `3566e059`) is present on `feat/utxo-reservation-core` and `feat/utxo-reservation-router`, and ABSENT from `feat/utxo-reservation-guards` and `feat/utxo-reservation-partial-redemption`, because `#1091` is 13 commits behind `#1090`. The fold added +685 -190 across 10 production files, `Reservation.sol` most of all (+342 -95). (pr-map.md section 3) |
| **Corrected statement** | Every line number cited from the guards tip is a pre-`#1102` line number. The guards tip does not contain `#1102`'s 30 review fixes. Either extract affected files from `#1090`'s tip and re-apply the upper-stack changes, or rebase `#1091` first. This must be settled before any extraction begins. |
| **Applied? (2026-09-28)** | Ambiguous. `m1-b-implementation.md`'s Provenance section still reads "Source-verified on feat/utxo-reservation-guards unless noted" (lines ~311-321) without restating the wrong "contains #1102's fixes" implication, but also without this correction's caveat language. Moot regardless: the M1 code now exists and no longer cites guards-tip line numbers at all (§1.4, D-22). |

### C-4: Branch drift is worse than documented (pr-map.md section 2)

| | |
|---|---|
| **Claim** | `#1090` is the only CONFLICTING PR and the only rebase needed. |
| **Doc and line** | `epic-merge-plan.md` section 0.1 (referenced from pr-map.md section 2) |
| **What the source says** | `#1090` has since been rebased (0 behind, and it contains the `#1102` fold). The staleness has moved up the stack: `#1091` is 13 commits behind `#1090`, and `#1092` is 5 commits behind `#1091`. A branch can be behind without conflicting, and it is still a correctness problem for extraction, because reading the stale branch shows pre-fix code. (pr-map.md section 2) |
| **Corrected statement** | Three branches are behind their bases, not one. `#1091` (13 behind) and `#1092` (5 behind) are the stale branches. `#1090` is 0 behind (rebased). `#1088` is 2 behind due to an unrelated security fix (`#1098`), which is normal drift, not a stack problem. |
| **Applied? (2026-09-28)** | Yes — `inventory/pr-map.md` §2 (lines ~39-51, 82-85) documents the 13-behind/5-behind drift and that `#1090` is 0 behind, matching this corrected statement. |

### C-5: `feature-spec.md` section 15 vault-swap orphaning claim (roadmap.md section 2.2)

| | |
|---|---|
| **Claim** | An earlier draft of `roadmap.md` section 2.2 claimed a vault swap orphans revealed-but-unaccepted deposits. |
| **Doc and line** | `roadmap.md` section 2.2 (corrected in place); `feature-spec.md` section 15 (corrected per roadmap) |
| **What the source says** | The guard is contract-enforced, not merely discouraged: `updateReservationParameters` reverts a vault change unless `reservationTotalAmount == 0` AND `pendingReservedDeposits == 0` (`Reservation.sol:1263-1274`). Nothing is silently orphaned; the transaction simply fails. The consequence is worse than orphaning: it is a liveness constraint making the swap impossible until every position has closed and every revealed deposit has been accepted or marked stale. (roadmap.md section 2.2) |
| **Corrected statement** | A vault swap is not possible while the product is in use. The guard reverts rather than orphaning. `feature-spec.md` section 15 has been corrected to record the guard as enforced. |
| **Applied? (2026-09-28)** | Yes — `roadmap.md` §2.2 (lines ~492-500, superseded note at ~512) and `feature-spec.md` §15 (lines ~1019-1026, 1278-1280) both state the vault re-point guard is contract-enforced, not merely discouraged. |

### C-6: `feature-spec.md` lines 1067-1069 `#1102` convergence claim (proofs.md PD-8)

| | |
|---|---|
| **Claim** | *(historical — already corrected in place in `feature-spec.md` §15 on 2026-08-21; this row is retained only because D-15 still turns on the unverified `#1102` `spentMainUTXOs` shape)* `feature-spec.md:1067-1069` recorded that `#1102` "moved anchor consumption to `spentMainUTXOs`" on `feat/utxo-reservation-core`, and that a reverse index used by `strandReservation` "exists on `feat/utxo-reservation-guards` but was deleted from `feat/utxo-reservation-core` by the `#1102` merge." |
| **Doc and line** | `feature-spec.md:1067-1069` |
| **What the source says** | Only the guards and partial trees are available locally, so the exact shape of the `#1102` core-line implementation is `UNVERIFIED`. The two lineages have not been reconciled. `spentMainUTXOs` is pre-existing on `#1088`'s branch (6 mentions in `Reservation.sol`). The reverse index `reservationsByAnchorUtxo` is introduced by `#1091`, not removed by `#1102`. (proofs.md PD-8; pr-map.md section 4) |
| **Corrected statement** | The `#1102` lineage is `UNVERIFIED` against the guards tip. `spentMainUTXOs` is pre-existing, not introduced by `#1102`. The reverse index is introduced by `#1091`. Which lineage the m1 rewrite takes as its base is an open decision. See D-15. |
| **Applied? (2026-09-28)** | Yes, already marked historical/corrected-in-place per this row's own claim text; `feature-spec.md` does not currently restate the "moved anchor consumption... deleted by #1102" claim in the section checked. |

### C-7: `roadmap.md` section 5 dissolution-is-mandatory claim (roadmap.md section 5, reversed; section 4.4)

| | |
|---|---|
| **Claim** | The earlier `roadmap.md` argued dissolution was "not optional" for m1 because it is permissionless and cannot be gated off by a minimal vault. |
| **Doc and line** | `roadmap.md` section 5 item 1 (earlier text, now reversed); `roadmap.md` section 4.4 ("This reverses what this section said before") |
| **What the source says** | That reasoning was right for a create-only m1 built from the stack, where the dissolution entry point ships whether or not the client supports it. It does not apply to variant B: B removes `requestReservationDissolution` from the router entirely, so there is no entry point to leave unwired and no slashing vector to arm. (roadmap.md section 4.4, section 6 reversed item 3) |
| **Corrected statement** | Under variant B, keep-core m1 needs acceptance and re-anchor only, not dissolution. The dissolution slashing vector does not exist in m1 B because the entry point does not exist. This is B's one genuine saving, worth roughly 300-500 production Go lines. |
| **Applied? (2026-09-28)** | Yes — `roadmap.md` §4.4 (lines ~690-704, heading "keep-core #4238") contains the exact reversal text this row cites ("This reverses what this section said before"), and cross-references `milestone-inventory.md` C-7 by name. The claim that no §4.4 exists is wrong — rejected. |

### C-8: keep-core `#4238` described as the single-phase design (keep-core.md section 1)

| | |
|---|---|
| **Claim** | `#4238` implements the original single-phase design and needs rework for "nonce-carrying proposals, a watchtower-delay-respecting executor, and partial-redemption awareness". |
| **Doc and line** | `feature-spec.md` section 16; `roadmap.md` section 5; `epic-merge-plan.md` section 3 (keep-core subsection) |
| **What the source says** | Nonces are already present: `RequestNonce` appears 14 times and is a field on **all four** proposal structs (`reservation.go:181`, `:233`, `:287`, `:342`), and the chain interface reads action records by generation via `GetReservationAction(reservationKey, requestNonce)` (`chain.go:437-440`). Both are two-phase constructs. Separately, the gap is larger than "rework": `pkg/tbtcpg` contains a proposal-generator task per wallet action and has **no reservation task at all** (zero reservation mentions in the package), and the chain interface's reservation region contains only reads and validators with no `Submit*`, `Request*` or `Notify*` method. (keep-core.md sections 1-3) |
| **Corrected statement** | `#4238` is a types-and-assembly foundation, not a single-phase client. It already carries the two-phase nonce constructs. What it lacks is the entire executor: no proposal generation and no on-chain submission, so it cannot participate in the protocol. m1's Go work is therefore mostly new code rather than rework, which changes both the risk profile and the sizing. See D-25 and D-26. |
| **Applied? (2026-09-28)** | Yes — `feature-spec.md` (lines ~949-953, 1236-1240), `roadmap.md` (lines ~661-663, 697-706, 838-839) and `epic-merge-plan.md` (lines ~35-39, 116-120) all state `#4238` is not single-phase, carries `RequestNonce`, and needs an executor rather than a full rework. |

## 7. Open-decisions register

Every `DECISION NEEDED` from every fragment, deduplicated and numbered so other documents can cite them. Each gets a one-line question and a recommendation where a fragment offered one. Blocking decisions must be resolved before m1 can ship; deferrable ones can be answered during or after implementation.

**Recount, 2026-09-28 (post M1-code reconciliation; see §1.4).** This register groups 16 decisions as "Blocking for m1" (D-1 to D-16) and 11 as "Deferrable" (D-17 to D-27) — 27 total. Checked against the M1 code: of the 16 blocking decisions, 7 were decided exactly as recommended (D-3, D-4, D-6, D-7, D-8, D-11, D-14), 1 was decided but broader in scope than described (D-9), 1 was decided only in part (D-2), 1 was decided in part and superseded in part (D-5), 2 were superseded by an M1 design that took the opposite approach from the recommendation (D-1, D-10), 2 were superseded by the 2026-09-24 Option-B vault decision (D-12, D-13), and 2 were already marked resolved before this pass (D-15, D-16). **None of the 16 remain genuinely open** — every one now has a real answer in the M1 code, even where that answer differs from the recommendation. Of the 11 deferrable decisions: 1 was decided as recommended (D-19), 2 were decided opposite to their recommendation on a non-upgradeable surface that cannot be revisited without a redeploy or rebuild (D-17, D-27), 5 are moot because their premise no longer applies now that the rewrite is complete (D-20, D-22, D-23, D-24, D-26), 1 was already marked resolved (D-25), and **2 remain genuinely open** (D-18, D-21 — both operational/deployment concerns not verifiable against either code snapshot in this pass).

### Blocking for m1 (D-15 and D-16 are resolved but retained here for numbering stability; see the recount above)

**D-1. Keep or delete the unreachable refund branch in the timeout path?**
Question: Does m1 keep the unreachable redemption refund branch at `Reservation.sol:1009-1022` (which reads `action.feePaid`, `action.redeemer`, and writes `reservation.retryCredit`), or delete it and re-add in m2?
Recommendation: Keep. Fields stay either way; this is Bridge code so it is replaceable. Keeping costs a few lines and keeps the diff to m2 smaller. Stripping the retry-credit and redeemer handling from `unwindPendingAction` makes the helper diverge from what m2 needs and breaks the "m2 is an upgrade, not a migration" property.
Source: data-model.md section 2, open question 1; proofs.md PD-5, PD-10, PD-11.
Status: blocking. `unwindPendingAction` is a shared helper m1 must extract intact.
**2026-09-28 M1 code check: superseded.** The recommendation ("keep the branch") doesn't apply to the M1 code's structure: `notifyReservationActionTimeout` was split into two dedicated functions — `notifyReservationAcceptanceTimedOut` (`Reservation.sol:655-691`, capacity release only) and a Reanchor-only `notifyReservationActionTimeout` (`:860-914`, `require(action.actionType == ActionType.Reanchor)`) — and neither contains the `feePaid`/`redeemer`/`retryCredit` refund logic this decision was about. There is no redemption-arm code left to keep or delete.

**D-2. Do the four unvalidated cap parameters get relational validation in m1?**
Question: `reservationMaxTotalAmount` (`Reservation.sol:1280`) and `maxReservationsPerWallet` (`:1281`) are assigned with no `require` of any kind. `updateReservationCaps` (`:1390-1401`) assigns `maxReservationsAmountPerWallet` and `reservationMaxSingleAmount` with not a single `require`. Should m1 add relational validation to these setters?
Recommendation: Add validation to `updateReservationCaps`. It is cheap and is the natural place. The m1 global cap gate needs `maxActiveReservations` checked against slot capacity, and today no cap checks anything. Adding validation changes a reviewed function's semantics but prevents governance from setting self-contradictory caps.
Source: data-model.md section 7, open question 2.
Status: blocking. Without any validation, the launch gates (section 3) have no teeth.
**2026-09-28 M1 code check: decided (partially).** `maxReservationsPerWallet > 0` is now enforced (`Reservation.sol:1295-1298`), `maxActiveReservations > 0` is enforced (`:1382-1385`), and `validateReservationCapsInvariant` (`:1220-1232`, called from both setters) enforces `reservationMaxTotalAmount <= maxActiveReservations * reservationMaxSingleAmount`. `maxReservationsAmountPerWallet` alone remains unvalidated. See §2.6.

**D-3. Is `reservationRenewalWindowSeconds` still validated in m1?**
Question: It is a stored parameter with no m1 reader (renewal is deferred), but `updateReservationParameters` validates it relationally against the term (`0 < window < term`, `Reservation.sol:1252-1256`). Keep the validation or drop it as dead code?
Recommendation: Keep the validation. It is cheap and protects m2's semantics. The validator refuses to configure a system with no renewal window, and `#1093`+ depends on it.
Source: data-model.md open question 4; router.md open question 2; roadmap.md section 0.1.
Status: blocking. Dropping the validation would allow misconfiguring the system in a way m2 cannot recover from.
**2026-09-28 M1 code check: decided as recommended.** The relational check ships: `require(reservationRenewalWindowSeconds > 0 && reservationRenewalWindowSeconds < reservationTermSeconds, ...)` (`Reservation.sol:1286-1289`).

**D-4. Do action-record fields for deferred action types need to be written in m1?**
Question: `ReservationAction.isPartial` (partial `Reservation.sol:263`) and `retryCreditSourceNonce` (`:268`) are only ever written on the redemption request path (m2). Does rule 1 (storage-complete means written, not merely declared) bind per-generation action-record fields, or only the long-lived `ReservationRequest` position record?
Recommendation: Declare only. No m1 code path can write them, and no m1-created action record (Acceptance or Reanchor) is ever read for them. `resolveLateAgainstPending` reads `isPartial` only after confirming `actionType == Redemption`. Rule 1 should be interpreted as binding the position record, not transient action records.
Source: proofs.md PD-1.
Status: blocking. Must settle the rule 1 scope before extraction begins.
**2026-09-28 M1 code check: decided as recommended.** `isPartial` and `retryCreditSourceNonce` are declared (`Reservation.sol:317-323`, `:312-316`) but no m1 code path writes them; rule 1 was interpreted as bound to the position record, not transient action records.

**D-5. Must m1 declare and write `retryCredit` and `reservationRetryCreditActionNonce`?**
Question: `reservation.retryCredit` (`Reservation.sol:180`) is a position field, so rule 1 points at m1. But its only mint sites are the redemption timeout arm (`:1009-1012`, m2) and `unwindPendingAction`'s Redemption branch (`:1263-1264`, m2). Its only consumer is the redemption request path (m2). A declared-but-never-written field is a rule-1 violation as stated; a write with no m1 semantics is worse. `reservationRetryCreditActionNonce` (partial `BridgeState.sol:461`) is the same shape.
Recommendation: Declare both. Do not write them in m1. The timeout path that writes `retryCredit = true` is only reached for Redemption actions, which cannot exist in m1. The field exists in the layout for m2. Interpret rule 1 as requiring declaration for position fields whose only writer is m2-exclusive, not requiring a synthetic m1 write.
Source: proofs.md PD-2; data-model.md section 2, section 4.
Status: blocking. Same rule-1-scope question as D-4.
**2026-09-28 M1 code check: decided in part, superseded in part.** `reservation.retryCredit` is declared and never written in m1, as recommended (`Reservation.sol:197`). `reservationRetryCreditActionNonce`, by contrast, was NOT declared-and-unwritten — it was deleted outright from `BridgeState.Storage` (`__gap` comment, `BridgeState.sol:491-496`).

**D-6. Should m1 emit `ReservationAccepted` with `expiresAt` unchanged?**
Question: The `ReservationAccepted` event (`ReservationProofs.sol:551-559`) carries `expiresAt`. In m1, nothing enforces the expiry. Should m1 emit it unchanged, knowing off-chain consumers will read it as an enforced deadline that m1 does not enforce?
Recommendation: Emit it unchanged. The value is correct (it is written to storage at `:533`) and m2 will enforce it. Changing the event would break ABI compatibility. Off-chain monitoring should document that m1 does not enforce the term on-chain.
Source: proofs.md PD-6; m1-b-implementation.md section 4.4.
Status: blocking. Event ABI must be stable across milestones.
**2026-09-28 M1 code check: decided as recommended.** `ReservationAccepted` still carries `expiresAt` unchanged (`ReservationProofs.sol:72-80`, emitted at `:576-584`).

**D-7. Does m1 keep the four-member `ProofType` enum or shrink it?**
Question: `enum ProofType { Acceptance, Redemption, Reanchor, Dissolution }` (`ReservationProofs.sol:117-122`) is ABI-encoded as `uint8` by the router (`ReservationRouter.sol:323`). m1 has no Redemption or Dissolution handler. Keep the four-member enum with two arms reverting, or shrink to `{Acceptance, Reanchor}` and renumber?
Recommendation: Keep the four-member enum verbatim. Renumbering silently changes the meaning of `proofType == 1` for any client built against m1, and the dispatcher's `else` fallthrough (`:169`) means an out-of-range value would be routed rather than rejected. This is the same layout hazard as the other enums (rule 5).
Source: proofs.md PD-7; data-model.md section 3.
Status: blocking. ABI stability across the milestone boundary.
**2026-09-28 M1 code check: decided as recommended.** `enum ProofType { Acceptance, Redemption, Reanchor, Dissolution }` ships verbatim (`ReservationProofs.sol:113-118`); the Redemption/Dissolution arms of the internal `submitReservationProof` dispatcher revert as unsupported (comment at `:120-122`).

**D-8. Does `closeReservation` ship in m1?**
Question: `closeReservation` (`Reservation.sol:1490-1506`) is shared between two m2 actions only (redemption at `:715`, dissolution at `:1142`). Rule 5 says a helper shared between m1 and m2 is m1 work, but does not cover a helper shared exclusively between deferred actions. Shipping it in m1 leaves dead code; omitting it makes the m2 diff touch `Reservation.sol` again. Same question for `Reservation.notifyReservedRedemptionVeto` and the `Vetoed` enum member.
Recommendation: Ship it. The function is small (16 lines), and omitting it means m2 must re-add a function to a file m1 has already rewritten. The `Vetoed` enum variant must be declared regardless (rule 5). `notifyReservedRedemptionVeto` is Bridge code (replaceable), so it may be omitted and re-added in m2 without cost.
Source: proofs.md PD-3.
Status: blocking. Affects the m1 code surface.
**2026-09-28 M1 code check: decided as recommended.** `closeReservation` ships in `Reservation.sol` (`:1039-1064`), unreachable in m1 (no m1 caller), exactly as recommended.

**D-9. Is governance-only rotation the intended m1 unpin path?**
Question: Rule 4 deletes the `block.timestamp < dissolutionEligibleAt` gate (`Reservation.sol:785-788`) so re-anchor is unbounded in time. But it does NOT touch the wallet-state gate (`:790-806`): a re-anchor still requires the source wallet to be `MovingFunds`, or `Live` with `privileged == true` (governance). So in m1 a position on a healthy `Live` wallet is unpinnable only by a governance transaction. Is governance-only rotation the intended m1 unpin path, or should rule 4 also relax the source-wallet gate?
Recommendation: Keep the wallet-state gate. Governance-only rotation is acceptable at design-partner scale. Relaxing it would allow any caller to rotate positions off healthy wallets, which is a security change beyond the m1 scope. The spec should document that m1 unpin of a `Live` wallet requires governance.
Source: proofs.md PD-4; roadmap.md section 1.5.
Status: blocking. Defines m1's operational model.
**2026-09-28 M1 code check: decided, slightly broader than described.** Governance-only rotation off a `Live` wallet is enforced (`Reservation.sol:748-752`, `require(privileged, "Only governance can rotate a Live wallet's anchor")`). The non-privileged source-wallet gate is broader than this decision's premise: it now accepts `MovingFunds` OR `Closing` (`:753-759`), not `MovingFunds` alone — a refinement this row (and §2.2's re-anchor-request table) should note.

**D-10. Does m1 ship the full four-branch `unwindPendingAction` or only the m1 branches?**
Question: Rule 5 makes `unwindPendingAction` (`:1243`) m1 work because m1 reaches it from late acceptance (`:490`) and late re-anchor (`:821`). But two of its four branches are unreachable in m1: Redemption (`:1261-1272`, refunds escrow) and Dissolution (`:1282-1292`, releases the wallet lock). Ship all four, or only Acceptance and Reanchor?
Recommendation: Ship all four branches. Shipping all four requires D-1, D-4, and D-5 to be answered first (the Redemption branch writes `retryCredit` and reads `redeemer`/`usedRetryCredit`; the Dissolution branch writes `walletPendingDissolution`). Keeping the full body preserves the "m2 is an upgrade" property and avoids a signature change.
Source: proofs.md PD-10, PD-11.
Status: blocking. Depends on D-1, D-4, D-5.
**2026-09-28 M1 code check: superseded.** The M1 `unwindPendingAction` (`ReservationProofs.sol:819-859`) ships only the Acceptance and Reanchor branches. The Redemption and Dissolution branches were dropped — not included as unreachable stubs — so m2 will need to add them back, and the "m2 is an upgrade, not a migration" property this decision aimed to preserve was not fully kept for this helper.

**D-11. Does m1 keep the `restoreRetryCredit` parameter in `unwindPendingAction`?**
Question: `:821` passes `true` for `restoreRetryCredit`. That argument is consumed only inside the Redemption branch (`:1262`), so in m1 it is provably inert. Does m1 keep the parameter at all?
Recommendation: Keep the parameter. Dropping it makes the m2 restoration (partial `:1448-1453`, which passes `action.isPartial`) a signature change rather than an argument change.
Source: proofs.md PD-11.
Status: blocking. Signature stability for m2.
**2026-09-28 M1 code check: decided as recommended.** The `restoreRetryCredit` parameter is kept (`ReservationProofs.sol:824`) but is confirmed unused in m1 (`// solhint-disable-next-line no-unused-vars`; doc comment "Unused in milestone 1 (reserved for milestone 2 redemption retry-credit restoration)", `:817-818`).

**D-12. How is initiation disabled in m1?**
Question: The vault has no `initiationPaused` flag. The existing `renewalsPaused` gates `extendCustody` only, NOT `receiveBalanceIncrease`. The deploy script says "Vault trust is the safe activation boundary." Three options: (a) add a new `initiationPaused` flag with a `require(!initiationPaused)` guard in `receiveBalanceIncrease`; (b) rely solely on Bridge vault-trust status (`setVaultStatus`); (c) overload `renewalsPaused` to also guard `receiveBalanceIncrease`.
Recommendation: Option (b), rely on Bridge vault-trust status. The deploy script already establishes this as the safe activation boundary. The vault ships inert (untrusted) until governance activates it. This avoids adding a new flag and keeps the vault's existing pause machinery unchanged.
Source: vault.md open question 1, 4.
Status: blocking. Determines how the vault ships.
**Superseded 2026-09-24 ("Option B"; see §1.4 and the README.md decision log).** Neither option (a)/(b)/(c) was taken. `receiveBalanceIncrease` ships with no pause guard at all and is live from deployment (`ReservationVault.sol:161-212`); there is nothing to gate because Option B removes the entry points a flag would have covered.

**D-13. Is redemption paused in m1?**
Question: The milestone-1 rule says m2 restores redemption. The existing `renewalsPaused` does NOT cover `redeemReservation` or `retryRedeemReservation`. If redemption must be disabled in m1, a new `redemptionsPaused` flag is needed. But the rule also says "never gate settlement or accounting," and redemption is settlement-adjacent.
Recommendation: Yes, add `redemptionsPaused` (new, section 3). Redemption is initiation-path for the vault (the owner initiates the spend request), not settlement-path (the Bitcoin spend is settled by the Bridge independently). The flag gates the two initiation functions only, never `financeInKindFee` or `repayInKindFeeDebt`.
Source: vault.md open question 2, 3; m1-b-implementation.md section 3.
Status: blocking. Required for m1 launch posture.
**Superseded 2026-09-24 ("Option B"; see §1.4 and the README.md decision log).** `redeemReservation`/`retryRedeemReservation` do not exist in the M1 vault at all, so there is no redemption path to pause. `redemptionsPaused` was never built (§3).

**D-14. `requireCurrentSourceAnchor`: keep the guards helper or take `#1096`'s fold?**
Question: Guards keeps a standalone `requireCurrentSourceAnchor` (`:308-313`) called at three sites (`:665`, `:778`, `:955`), each after `prepareReservationForSettlement`. `#1096` deletes the helper and folds the check into `prepareReservationForSettlement` (partial `:270-273`), changing its signature. Which shape does m1 take?
Recommendation: Take the guards shape (standalone helper). m1 extracts from the guards tip. Taking the `#1096` shape means m1 ships a signature that only `#1096` motivates, and the revert-string ordering visible to clients changes. m2's `#1096` rebase must redo the fold, but that is an m2 task.
Source: proofs.md PD-9.
Status: blocking. Affects the m1 code surface and m2 rebase effort.
**2026-09-28 M1 code check: decided as recommended.** The guards shape (standalone helper) ships: `requireCurrentSourceAnchor` (`ReservationProofs.sol:311-...`), called at `submitReservationReanchorProof:670` and elsewhere.

**D-15. Which `spentMainUTXOs` lineage does the m1 rewrite take?**
Question: `consumeAnchor` writes `spentMainUTXOs[anchorUtxoKey] = true` (`:1356`). `feature-spec.md:1067-1069` records that `#1102` "moved anchor consumption to `spentMainUTXOs`" on `feat/utxo-reservation-core`. Only the guards and partial trees are available locally, so the exact shape of the `#1102` core-line implementation is `UNVERIFIED`. Which lineage is the m1 rewrite's base?
**RESOLVED 2026-08-23.** No reconciliation needed. `pr-strategy.md` §7 (corrected 2026-08-22) established `#1091`-`#1096` as the sole extraction source for reservation mechanics — `#1090`+`#1102` is not a competing lineage to merge or reconcile against, it's an incompatible parallel redesign (56/121 tests fail if merged; see `agent-docs/m1/step-01-execution-report.md`). Checked directly (`gh pr view 1102 --json body`): `#1102`'s PR body never names `spentMainUTXOs` as a fix target — it is pre-existing Bridge infrastructure (6 mentions already on `#1088`'s branch, before any reservation-specific development), not something `#1102` reshapes. The "deleted by `#1102`" claim that motivated the original `UNVERIFIED` framing was already independently refuted (`inventory/pr-map.md` §4: 0 hits for `reservationsByAnchorUtxo` on `#1088`/`#1090`'s own branches — it never existed there to be deleted). Take `#1091`-`#1096`'s `spentMainUTXOs` write (`ReservationProofs.sol:1356` on the guards tip) as-is; no `#1102` content applies here at all.
Source: inventory/proofs.md PD-8; inventory/pr-map.md section 4; correction C-1, C-6.
Status: **resolved**. Extraction may proceed without further reconciliation on this item.

**D-16. Does `reservationByAnchorUtxo` the view stay in m1 while the reverse index has two unreconciled write sites?**
Question: The view is cheap, but exposing a half-reconciled index is worse than not exposing it. Reconcile first, then decide.
**RESOLVED 2026-08-23.** The "unreconciled" framing assumed the two write sites might belong to competing lineages needing reconciliation. They don't: both `#1091`'s acceptance-settlement write (`:541-543`) and `#1094`'s stranding delete (`Reservation.sol:1462-1472`) are within the single canonical `#1091`-`#1096` lineage (§7) — a write-then-delete pair across one position's lifecycle, not two lineages in tension. Keep the view; its own recommendation below already reached the right conclusion, the "reconcile first" instruction is superseded now that there is nothing to reconcile.
Recommendation: Keep the view. Both write sites are in m1 code (acceptance settlement at `:541-543` and stranding at `Reservation.sol:1462-1472`). The index is consistent within m1: every write has a matching delete. The view is consumed by `consumeAnchor`'s delete and by off-chain monitoring.
Source: router.md open question 1; pr-map.md section 4.
Status: **resolved**. The view ships; no further reconciliation blocks it.

### Deferrable

**D-17. Should `WalletProposalValidator.sol` ship its m2-only proposal validators in m1?**
Question: `WalletProposalValidator` is non-upgradeable (`:31`). Unlike router code, a non-upgradeable contract cannot be replaced by a Bridge implementation upgrade. If the m2 validators (`validateReservedRedemptionProposal`, `validateReservationDissolutionProposal`) are omitted from m1, they cannot be added later without redeploying the contract.
Recommendation: Ship the m2-only validators in m1. The contract is non-upgradeable, so omitting them closes the path permanently. They are view functions (no storage writes), so they are inert until m2 adds the action types they validate. The `ReservedRedemptionProposal` and `ReservationDissolutionProposal` structs must also be present.
Source: touchpoints.md open question 1, 3.
Status: deferrable to the deployment decision, but must be resolved before m1 contract deployment (not before extraction).
**2026-09-28 M1 code check: decided, opposite of the recommendation.** The M1 `WalletProposalValidator.sol` does NOT ship `validateReservedRedemptionProposal`/`validateReservationDissolutionProposal` or their `ReservedRedemptionProposal`/`ReservationDissolutionProposal` structs — grep of the M1 file returns zero hits for either. Since the contract is non-upgradeable, this decision's own warning now applies for real: m2 cannot add these validators without redeploying `WalletProposalValidator`.

**D-18. Should the watchtower implement reserved-redemption interface functions at m1?**
Question: The `IRedemptionWatchtower` interface functions for reserved redemptions (`Redemption.sol:74-108`) are part of an interface in a library imported by `Bridge.sol` (upgradeable). The watchtower contract itself may be deployed at m1. Should it implement these as no-ops returning zero/false, or should they be absent until m2?
Recommendation: Absent until m2. `validateReservedRedemptionProposal` is m2-only and would revert on the external call if invoked, but no m1 code invokes it.
Source: touchpoints.md open question 4.
Status: deferrable. No m1 path calls these functions.
**2026-09-28: not independently re-verified** — no watchtower contract is present in either M1 code snapshot to check against. Still open/deferrable as stated.

**D-19. Does `beginWalletClosing` need to remain reservation-gated in m1?**
Question: In m1 variant B, a wallet in MovingFunds with reservations that has proven its funds moved (so `movingFundsRequestedAt == 0`) is stuck: it cannot close (blocked by `walletReservationsCount`), it cannot be timed out (blocked by the completion sentinel), and its only exit is re-anchor or stranding. Is this the intended m1 behavior, or should `beginWalletClosing` allow closing with active reservations in m1?
Recommendation: Keep the gate. Relaxing it would allow a wallet to close while still custodying anchor obligations, making those obligations unprocessable because their proof and timeout paths require a Live or MovingFunds wallet. The stuck state is the intended pressure that forces re-anchor.
Source: touchpoints.md open question 2.
Status: deferrable. The behavior is correct under variant B; relaxing it is a design change.
**2026-09-28 M1 code check: decided as recommended.** The reservation-count gates on `beginWalletClosing`/`moveFunds`/`notifyWalletFundsMoved` are unchanged in the M1 code (§2.5); the stuck state remains the intended pressure toward re-anchor.

**D-20. Does m1 need `dissolutionFeeBps` or a dissolution fee parameter?**
Question: The vault has no dissolution fee parameter; dissolution miner fees are computed on-chain by the Bridge (`ReservationProofs.sol:994`) and financed via `financeInKindFee`. Since dissolution is omitted in m1, the dissolution fee-financing call path is dormant. But `financeInKindFee` itself must be present and active.
Recommendation: Confirm: m1 ships `financeInKindFee` active, and the dissolution call path is simply not exercised because the dissolution proof submission is omitted. No dissolution fee parameter is needed in the vault.
Source: vault.md open question 6.
Status: deferrable. No m1 action reaches the dissolution fee path.
**Superseded by Option B (§1.4).** Dissolution stays entirely out of m1's vault surface; `financeInKindFee` is live via re-anchor as before. No dissolution fee parameter was added, consistent with the recommendation, though the premise (a vault that would otherwise ship dissolution-adjacent code) no longer applies under the minimal-vault design.

**D-21. Should m1 enforce a non-zero `feeReserveTarget` before enabling re-anchor?**
Question: If `feeReserveTarget` is set to zero and some residual TBTC exists, `sweepFees` could drain the reserve needed for re-anchor fee financing. The deploy script lists `updateFeeReserveTarget` as a required step.
Recommendation: Yes, enforce non-zero `feeReserveTarget` before activation. The deploy script already requires it as step 3. Re-anchor produces in-kind fee debt from the first transaction, so the reserve must be funded.
Source: vault.md open question 7.
Status: deferrable. Operational, not a code change.
**2026-09-28: not independently re-verified against deployment records** — `feeReserveTarget` enforcement is an operational/deploy-script concern outside both code snapshots' scope. Still open/deferrable as stated.

**D-22. Should the docs' guards-tip citations be re-verified against a rebased tip?**
Question: Every line number in `m1-b-implementation.md` and `feature-spec.md` was read from a pre-`#1102` tree. The line numbers are probably still close, but "probably" is not a standard this doc set has been holding itself to.
Recommendation: Re-verify after rebasing `#1091` and `#1092`. The rebase makes the guards tip trustworthy as a single extraction source, which is worth a lot given how many docs cite it.
Source: pr-map.md open question 3; pr-map.md section 2.
Status: deferrable. Can be done after extraction begins, but before implementation.
**2026-09-28: moot.** The rewrite already happened; the M1 code no longer cites guards-tip line numbers at all — it is its own tree. Re-verifying guards-tip citations against a rebase is no longer the relevant task; re-deriving this ledger's own citations against the M1 code (as this pass does) has taken its place.

**D-23. Is `reservations-epic` the right base for m1, given it now carries an unrelated security fix?**
Question: `reservations-epic` is 2 commits ahead of `#1088` due to an unrelated security fix (`#1098`). A fresh branch from current `main` would be cleaner for a rewrite, but loses the epic branch's isolation property.
Recommendation: Fresh branch from current `main`. The rewrite is not a merge of the stack; it is a new contract. The epic branch's isolation is less valuable than a clean base for a rewrite.
Source: pr-map.md open question 2.
Status: deferrable. Affects extraction logistics, not the code surface.
**2026-09-28: moot, decided differently.** The M1 code lives on dedicated feature branches (tbtc-v2 `reservations-upgrade`, keep-core `reservations-epic`), not a fresh branch from `main` at the time of the rewrite, and not the guards/epic branches this decision considered.

**D-24. Rebase `#1091` and `#1092`, or extract around them?**
Question: Rebasing makes the guards tip trustworthy as a single extraction source. Extracting around the gap means every extraction must consult two branches per file. Rebasing touches reviewed logic in three PRs.
Recommendation: Rebase first. The guards tip is cited by every doc in the set. A single trustworthy source is worth the rebase effort, which is mechanical (conflict resolution, no new logic).
Source: pr-map.md open question 1.
Status: deferrable. Affects extraction logistics.
**2026-09-28: moot.** Extraction is complete; the M1 code is not a rebase of `#1091`/`#1092`, it is an independent rewrite on `reservations-upgrade`. No rebase decision remains to make.

**D-25. Is `#4238` edited, superseded, or closed?**
Question: C-8 shows most of `#4238` is directly reusable (types, enums, proposals with nonces, assemblers, chain reads and validators) and that the m1 client is largely additive rather than corrective. That argues for building on it rather than closing it.
Recommendation: Build on it.
Source: keep-core.md open question 2.
Status: **resolved 2026-08-21.** `pr-strategy.md` §8 previously treated `#4238`
as superseded design and recommended superseding the PR, a framing that predated
C-8. That section now recommends building on it, so the two documents agree and
this decision no longer blocks the keep-core PR plan. The remaining work is
additive: supply the executor `#4238` does not contain.

**D-26. Rebuild the keep-core effort estimate bottom-up?**
Question: The 1,400-1,900 production Go figure in `roadmap.md` section 5.1 was derived assuming rework of a single-phase client. The work is instead a new executor plus new write plumbing on top of a reusable type layer.
Recommendation: Rebuild from the task list in keep-core.md section 4. The direction of the correction is not obvious: the type layer is free, but an executor written from nothing is more work than adapting one.
Source: keep-core.md open question 3.
Status: deferrable for the code surface, **blocking** for any schedule commitment.
**2026-09-28: no longer blocking — the schedule question is moot.** The keep-core executor described as missing by this figure has been built (`reservations-epic` @ `f66f11240`; #4274 and follow-ups, all merged). A bottom-up rebuild of the original estimate is no longer actionable; the real diffstat is available directly from the merged PRs if a retrospective figure is wanted. See §2.9.

**D-27. Does m1 keep all four Go proposal types, or only the two it uses?**
Question: The Solidity enums and the `WalletActionType` wire constants are positional and must keep all four. The proposal structs are not persisted on-chain, so dropping the redemption and dissolution ones is safe if the action-type constants remain.
Recommendation: Keep all four. They are already written and tested; deleting and later restoring them is churn for no benefit.
Source: keep-core.md open question 1.
Status: deferrable.
**2026-09-28 M1 code check: decided, opposite of the recommendation.** The M1 keep-core code removed `ReservedRedemptionProposal`/`ReservationDissolutionProposal` (structs and their methods) and the named `ActionReservedRedemption`(7)/`ActionReservationDissolution`(9) constants, replacing the latter with anonymous reserved wire slots (`wallet.go:37,39`). Only two of the four Go proposal types were kept, contrary to "keep all four." See §2.9.

---

## Provenance

Synthesized 2026-08-21 from seven verified inventory fragments under `inventory/`:

- `inventory/data-model.md` - structs field by field, enum variants, storage fields, governance parameters and their validation, events
- `inventory/proofs.md` - every action lifecycle, and the complete position-closing site analysis (its section 4 is the most important single result in the set)
- `inventory/router.md` - the 24 router entry points, the four delegatecall invariants and which tests assert them, BridgeState fields
- `inventory/vault.md` - the vault surface, initiation/settlement/accounting path classification, pause machinery, upgradeability
- `inventory/touchpoints.md` - integration seams in non-reservation Bridge files
- `inventory/pr-map.md` - measured per-PR diffstat, branch drift, the `#1102` fold reach, extraction hazards
- `inventory/keep-core.md` - PR #4238's Go surface, and the executor it does not contain

All fragment line numbers come from the `feat/utxo-reservation-guards` tip (PR #1094), which predates the `#1102` fold (commit `3566e059`). See section 1.3 and correction C-3.

**Fragment decision-ID namespace.** `inventory/proofs.md` numbered its own
twelve decisions before this register renumbered them, and nine of the twelve
changed number. Its labels are therefore `PD-N`, with a concordance table at
the top of that fragment. A bare `D-N` anywhere in this set means *this*
register. One fragment decision, `inventory/proofs.md` PD-12, never reached this register:
it reported that `m1-variant-comparison.md` cited `ReservationProofs.sol:715`
and `:836` as the unreachable redemption path when `:836` is the re-anchor
source-wallet count decrement and is reachable in m1. Corrected at
`m1-variant-comparison.md:251-259` on 2026-08-21; the section's conclusion was
unaffected, only its evidence.

`UNVERIFIED` markers carried forward: the `#1102` core-line `spentMainUTXOs` implementation shape (D-15, C-6); all vault.md PR attributions (marked `?`). EIP-170 margin — **resolved 2026-09-28**: `Bridge` 22,914 B out of 24,576 B (1,662 B headroom) on a clean build of `reservations-upgrade` @ `9f8f5ef1` (router.md §4).
