# Gap Analysis: tbtc-v2 (Solidity) M1 Reservation Readiness

## Summary

Audit of tbtc-v2's `reservations-upgrade` branch, originally against landed
tip `8c5a2f4d` plus queued PR F (`ReservationVault`) and PR G (`Bridge`
activation wiring); re-verified against `reservations-upgrade` @ `9f8f5ef1`
(2026-09-28, PRs A-H all merged into `reservations-upgrade`, see
`../m1-keep-core-readiness/04-implementation-plan.md` Milestone 0) against
`feature-spec.md`, `m1-b-implementation.md`, and `milestone-inventory.md`.
Scope: does the Solidity implementation match what the spec says variant B
ships for milestone 1 (creation, custody, re-anchor;
redemption/dissolution/renewal/veto out of scope)?

Companion to `../m1-keep-core-readiness/01-gap-analysis.md` (Go client side).
This doc covers the Solidity/Bridge side only.

- **Blocker:** 0 (1 found, superseded by the 2026-09-24 Option B decision — see resolved row below)
- **Major:** 0 (1 found and resolved this session — see resolved row below)
- **Doc-staleness (spec text under/over-claims landed code):** 4
- **Confirmed matches:** see [Verified compliant](#verified-compliant)

## Blocker (resolved)

| Gap | Severity | Evidence | Spec/PR ref |
| :--- | :--- | :--- | :--- |
| `ReservationVault.sol` omitted `redeemReservation`/`retryRedeemReservation` while shipping a `redemptionsPaused` flag that gated nothing | Superseded (was Blocker) | This finding measured the vault against a requirement that no longer exists. The 2026-09-24 Option B decision (`m1-b-implementation.md` §3/§4.2, commit `661b06a04`) supersedes D-13's "ship both initiation functions behind `redemptionsPaused`" recommendation: the M1 `ReservationVault` ships deliberately minimal (per commit `4d549e64`), and M2 delivers redemption/renewal via a **new vault deployment plus a depositor migration ceremony**, not via unpausing flags on the M1 vault. Confirmed in code: the current `ReservationVault.sol` (`9f8f5ef1`) has exactly 7 external functions plus constructor — `receiveBalanceIncrease`, `financeInKindFee`, `repayInKindFeeDebt`, `updateFeeReserveTarget`, `sweepFees`, `updateInitiationFee`, `receiveBalanceApproval` (plus internal `_burnFromReserve`) — with no `redeemReservation`, `retryRedeemReservation`, `redemptionsPaused`, `pauseRedemptions`, or `unpauseRedemptions` at all; the vault is not proxy-upgradeable, which is exactly why the migration ceremony (not a flag flip) is required in M2. | 2026-09-24 Option B decision; `ReservationVault.sol` (`9f8f5ef1`) |

## Major (resolved)

| Gap | Severity | Evidence | Spec/PR ref |
| :--- | :--- | :--- | :--- |
| Re-anchor's `dissolutionEligibleAt` timing gate was not deleted, contradicting variant B's own design | Resolved (was Major) | `m1-b-implementation.md:190-192` states as fact that "B deletes its only reader — re-anchor's `< dissolutionEligibleAt` gate." At the time this finding was raised, `Reservation.sol`'s `requestReservationReanchor` still had `require(block.timestamp < reservation.dissolutionEligibleAt, "Reservation is dissolution-eligible");`, unconditionally, contradicting the spec. Fixed: the current `requestReservationReanchor` (`9f8f5ef1`) has no `dissolutionEligibleAt` check at all — a code comment explicitly documents the removal ("Re-anchor is intentionally unbounded in time in milestone 1, with no dissolution path available yet"). The one test that asserted the old revert ("rejects when reservation is dissolution-eligible") was rewritten to assert the opposite. | Resolved via PR [#1120](https://github.com/threshold-network/tbtc-v2/pull/1120) (`m1/reanchor-dissolution-gate-fix`, merged 2026-09-03) — see `03-followup-pr-spec.md` PR I |

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
| Router surface table under-counts the retained entry points | `milestone-inventory.md` §2 (router surface) enumerates 8 state-changing entry points as retained in m1. | `ReservationRouter.sol` has 11 state-changing entry points as of the current tip (`9f8f5ef1`), not 9 as an earlier revision of this row concluded: `requestReservationAcceptance`, `requestReservationReanchor`, `submitReservationAcceptanceProof`, `submitReservationReanchorProof` (the router exposes acceptance/re-anchor proof submission as two separate entry points, not a single `submitReservationProof` dispatcher — that name is only the internal `ReservationProofs` library function), `notifyReservationActionTimeout`, `notifyReservationAcceptanceTimedOut` (added during review to close a permissionless-release gap), `updateReservationParameters`, `notifyStaleReservedDeposit`, `forceStaleReservedDeposit` (governance-only, added later, paired with `notifyStaleReservedDeposit`), `notifyReservationStranded`, `updateReservationCaps`. Legitimate scope additions across multiple review rounds, not a spec deviation — the router-surface table should be updated to list 11 retained state-changing functions (plus 11 views), not 8 or 9. | `ReservationRouter.sol` (current tip `9f8f5ef1`) |

## Verified compliant

Confirmed matching spec/design intent by direct read of the landed
(`-ru`, current tip `9f8f5ef1`) tree — listed so this isn't read as an
all-gaps report:

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
  despite having no m1 reader — the one reader it had (the re-anchor gate)
  was removed by PR #1120 (see Major, resolved, above); the field is
  retained purely for M2's dissolution feature to consume later.
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
