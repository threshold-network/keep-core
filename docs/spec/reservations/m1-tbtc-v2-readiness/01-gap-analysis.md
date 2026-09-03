# Gap Analysis: tbtc-v2 (Solidity) M1 Reservation Readiness

## Summary

Audit of the tbtc-v2 `reservations-upgrade` branch (landed tip `8c5a2f4d`,
plus queued PR F `ReservationVault` and PR G `Bridge` activation wiring)
against `feature-spec.md`, `m1-b-implementation.md`, and
`milestone-inventory.md`. Scope: does the Solidity implementation match what
the spec says variant B ships for milestone 1 (creation, custody, re-anchor;
redemption/dissolution/renewal/veto out of scope)?

Companion to `../m1-keep-core-readiness/01-gap-analysis.md` (Go client side).
This doc covers the Solidity/Bridge side only.

- **Blocker:** 1
- **Major:** 1
- **Doc-staleness (spec under/over-claims landed code):** 4
- **Confirmed matches:** see [Verified compliant](#verified-compliant)

## Blocker

| Gap | Severity | Evidence | Spec/PR ref |
| :--- | :--- | :--- | :--- |
| `ReservationVault.sol` (queued PR F) omits `redeemReservation` and `retryRedeemReservation` entirely | Blocker | `m1-b-implementation.md` §3 lists both as **Present** (initiation only, gated by the new `redemptionsPaused` flag) — this is D-13's explicit recommendation ("the flag gates the two initiation functions only"). The actual file (`/tmp/spec-audit-f/solidity/contracts/vault/ReservationVault.sol`) has exactly 10 functions: `receiveBalanceIncrease`, `financeInKindFee`, `repayInKindFeeDebt`, `updateFeeReserveTarget`, `sweepFees`, `updateFees`, `pauseRedemptions`, `unpauseRedemptions`, `receiveBalanceApproval`. Neither `redeemReservation` nor `retryRedeemReservation` exists anywhere in the file — confirmed by full-file grep. The pause machinery (`redemptionsPaused` flag, `pauseRedemptions`/`unpauseRedemptions`) is present and correctly gates nothing, because there is nothing to gate: `receiveBalanceApproval` is a stub that unconditionally reverts ("Balance approvals not supported"), and its own NatSpec says "reserved redemptions are initiated via `redeemReservation`" — a function that was never written. | `m1-b-implementation.md` §3, §1 ("Vault: ship complete, behaviour disabled"); D-13 |

**Why this is a blocker, not a minor omission:** `m1-b-implementation.md` §1's core architectural rule is "two layers, opposite rules" — Bridge code is replaceable (upgradeable, can add functions later), but the Vault is not: an m1 vault entry point that isn't shipped now cannot be added in m2 without a full vault swap. `roadmap.md` §0.7 states this precisely: a vault entry point m1 omits cannot be reached in m2 by a governance transaction alone, because re-pointing `reservationVault` to a new vault requires `reservationTotalAmount == 0 && pendingReservedDeposits == 0` (total system quiescence) — a state that, once wallets hold live positions, is realistically unreachable without deliberately draining the system first. If PR F ships as-is, m2's redemption feature is not a vault upgrade; it is a full vault migration requiring the entire reservation book to be emptied first. This should be fixed before PR F merges, not deferred.

## Major

| Gap | Severity | Evidence | Spec/PR ref |
| :--- | :--- | :--- | :--- |
| Re-anchor's `dissolutionEligibleAt` timing gate was not deleted, contradicting variant B's own design | Major | `m1-b-implementation.md:190-192` states as fact (not a recommendation — a description of what variant B does): "B deletes its only reader — re-anchor's `< dissolutionEligibleAt` gate — so an essentials-only rewrite would naturally drop the [acceptance-time] write as dead code. Do not [drop the write]." The premise of that instruction is that the re-anchor gate is gone. It isn't: `Reservation.sol`'s `requestReservationReanchor` (landed, `-ru`) still has `require(block.timestamp < reservation.dissolutionEligibleAt, "Reservation is dissolution-eligible");`, unconditionally — it applies even to governance-privileged (`privileged == true`) re-anchors, unlike the separate cooldown check three lines above it which is skipped when `privileged`. Effect: once a reservation ages past its `dissolutionEligibleAt` timestamp, it can never be re-anchored again by anyone, including governance, while its wallet stays healthy. Since m1 has no dissolution or redemption path, an aged-out reservation on a `Live`/`MovingFunds` wallet that never terminates has no exit at all — it is stuck until the custodying wallet itself is terminated (which triggers stranding, the one path this gate doesn't block). | `m1-b-implementation.md:190-192`; `Reservation.sol` `requestReservationReanchor` |

## Doc-staleness (spec text predates later review fixes; code is correct, doc needs updating)

These are not implementation gaps — the landed code is doing the right thing.
`milestone-inventory.md`'s field-by-field ledger was written against an
earlier commit and has not been refreshed since three PR-review fixes
landed. Flagging so the ledger doesn't mislead a future reader into thinking
these are still open.

| Item | What the doc says | What's actually landed | Evidence |
| :--- | :--- | :--- | :--- |
| D-2 cap-relational-validation gap | `milestone-inventory.md` §2.6 (lines 603-604, 610-611) says `reservationMaxTotalAmount`, `maxReservationsPerWallet`, `maxReservationsAmountPerWallet`, `reservationMaxSingleAmount` are all "assigned, NO require... no validation at all," and D-2 lists this as blocking with a recommendation to add validation. | Resolved. `Reservation.sol` has `validateReservationCapsInvariant(reservationMaxTotalAmount, reservationMaxSingleAmount, maxActiveReservations)`, called from both `updateReservationParameters` and `updateReservationCaps`, enforcing `reservationMaxTotalAmount <= maxActiveReservations * reservationMaxSingleAmount` (skipped only when an operand is the sentinel disabled-value 0). Commented `// Decision 1 (option 2)` — this is the same decision already recorded in `agent-docs/m1/STATUS.md` from an earlier session. | `Reservation.sol:1324` (`validateReservationCapsInvariant`), call sites in `updateReservationParameters`/`updateReservationCaps` |
| `ReservationRequest` struct field table incomplete | `milestone-inventory.md` §2.1 lists 12 fields for `ReservationRequest`. | Struct has 14: the 12 listed, plus `cumulativeReanchorFee` and `reanchorCooldownUntil`, both explicitly comment-marked "Appended to the end of the struct" — the fee-grinding-cap fix (`pr-review-followups.md` item 7 / PR #1088 review fix) and its accompanying re-anchor cooldown. | `Reservation.sol:224-227` |
| `ReservationAction` struct field table incomplete | `milestone-inventory.md` §2.1 lists 17 fields for `ReservationAction`. | Struct has 19: the 17 listed, plus `termSeconds` and `dissolutionDelay`, both snapshotted at acceptance request time and consumed by `ReservationProofs.loadSettleableAction` to compute `expiresAt`/`dissolutionEligibleAt` from the generation record instead of the live governance parameter — this is the fix for the late-acceptance-settlement-uses-live-parameter bug found and closed during PR review. | `Reservation.sol:311-321` |
| Router surface table missing one retained entry point | `milestone-inventory.md` §2 (router surface) enumerates 8 state-changing entry points as retained in m1. | `ReservationRouter.sol` has 9: the 8 listed, plus `notifyReservationAcceptanceTimedOut(uint256 reservationKey)` — a router-level forwarder added during review to close a real gap (a pending acceptance whose designated wallet never signs would otherwise have no permissionless release path; only the redundant `notifyReservationActionTimeout` name existed before). Legitimate scope addition, not a spec deviation — the router-surface table should be updated to list 9 retained state-changing functions, not 8. | `ReservationRouter.sol:289` |

## Verified compliant

Confirmed matching spec/design intent by direct read of the landed
(`-ru`, tip `8c5a2f4d`) and queued (PR F `/tmp/spec-audit-f`, PR G
`/tmp/spec-audit-g`) trees — listed so this isn't read as an all-gaps
report:

- **Router surface removals** — all 5 functions the spec says variant B
  removes (`requestReservedRedemption`, `notifyReservedRedemptionVeto`,
  `extendReservation`, plus `requestReservationDissolution` and
  `walletPendingDissolution` as *router entry points*) are genuinely absent
  from `ReservationRouter.sol`. The two hits found under
  `requestReservationDissolution`/`walletPendingDissolution` are (a) a code
  comment and (b) the *storage mapping* being read/cleared inside
  `notifyReservationDissolutionTimedOut` and `unwindPendingAction`'s
  Dissolution branch — both m2-only-reachable per D-10 ("ship all four
  `unwindPendingAction` branches," verbatim as recommended), never actually
  triggered in m1 since nothing ever constructs a Dissolution action.
- **Governance access control** — `ReservationRouter.updateReservationParameters`/
  `updateReservationCaps` are `external onlyGovernance` via `Governable`
  (same base contract every other `onlyGovernance` Bridge setter uses,
  including the existing `setReservationRouter`). This is not a
  delay-bypass: the only route to the governance address's authority is
  through `BridgeGovernance`'s staged begin/finalize delay flow, matching
  `milestone-inventory.md:626`'s claim that no reservation parameter
  bypasses the governance delay.
- **8 parameter-validation checks** in `updateReservationParameters`/
  `updateReservationCaps` (`reservationTxMaxFee > 0`, min-amount-vs-fee,
  term bounds, renewal-window-vs-term, action-timeout-vs-safety-margin,
  vault-change quiescence gate, plus the now-added cap invariant above) all
  present and matching spec description.
- **D-7** (four-member `ProofType` enum kept verbatim, not shrunk) —
  confirmed, `ReservationProofs.sol` still declares
  `{Acceptance, Redemption, Reanchor, Dissolution}`.
- **D-9** (wallet-state gate on re-anchor kept; governance-only rotation of
  a `Live` wallet's anchor) — confirmed present in
  `requestReservationReanchor`: `Live` source requires `privileged`,
  otherwise source must be `MovingFunds` or `Closing`.
- **Wallet-closing reservation guard** — `Wallets.sol`'s
  `walletReservationsCount[walletPubKeyHash] == 0` guard sits in
  `notifyWalletClosingPeriodElapsed`, the sole caller of
  `finalizeWalletClosing` (verified: `finalizeWalletClosing` has exactly one
  call site). `beginWalletClosing` itself has no such guard, but that's
  correct, not a gap — entering `Closing` doesn't strand anything (the
  source wallet is still `re-anchor`-eligible per the state gate above);
  only the transition to `Closed` needs to be blocked while reservations
  remain, and that's exactly where the guard sits.
- **`reservationsByAnchorUtxo` reverse-index reconciliation** (D-16) — both
  write sites present and paired: acceptance-settlement write in
  `ReservationProofs.sol`, stranding delete in `Reservation.sol`.
- **`dissolutionEligibleAt` write survives** in acceptance settlement
  despite having no m1 reader other than the (incorrectly still-present,
  see Major above) re-anchor gate.
- **Storage layout parity** — `Bridge.StorageLayout.test.ts` exists and is
  an append-only layout-parity assertion suite, consistent with §4.5's
  launch gate.
- **PR G Bridge/governance activation wiring** — deploy scripts wire the
  freshly deployed `ReservationVault` into the Bridge on a clean deploy
  path, matching `roadmap.md`'s activation-sequencing intent.

## Not yet independently re-verified this pass

- **`unwindPendingAction`'s superseded-acceptance capacity release** and the
  **acceptance-timeout capacity release** logic (D-1, D-4, D-5's shared
  helper) were confirmed present but not re-traced arithmetically this
  session; prior session traced this in detail (see
  `pr-review-followups.md` items 1-3) and found no regression as of that
  check.
- **`ReservationVault` fee accounting** (`financeInKindFee`,
  `repayInKindFeeDebt`, fee reserve target) beyond the redemption-function
  gap above was not re-audited line-by-line this pass.
