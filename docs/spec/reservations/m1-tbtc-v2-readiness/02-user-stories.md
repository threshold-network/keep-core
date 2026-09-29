# User Stories: M1 tbtc-v2 (Solidity/Bridge) Readiness

*Consolidated M1 user stories: `requirements.md` §6; these are the per-repo detailed versions.*

This document enumerates the M1 reservation scenarios the tbtc-v2 Bridge
contracts must handle, derived from direct code analysis of the
`reservations-upgrade` branch, originally against landed tip `8c5a2f4d` plus
queued PR F (`ReservationVault`) and PR G (Bridge activation wiring);
re-verified against `reservations-upgrade` @ `9f8f5ef1` (2026-09-28), by
which point the full A-G stack (PRs #1106-#1112 plus the #1120 fix and the
#1121/#1122 revert) had landed (H is keep-core PR #4274 on the
`reservations-epic` branch, a separate repo). **Scope note:** M1 is decided variant B —
creation, custody and re-anchor only; redemption, dissolution, renewal and
the watchtower veto are M2 (`feature-spec.md:9-15`, `milestone-inventory.md:7`,
`roadmap.md` §0.1-0.2). Companion to `../m1-keep-core-readiness/02-user-stories.md`
(Go client side, which consumes the entry points documented here).

### Story 1: Deposit reveal reserves capacity
- Actor: Depositor (via deposit-reveal producer)
- Goal: Mark a fresh SegWit deposit as a reservation candidate at reveal time, before any acceptance decision is made.
- Benefit: The position is tracked and counted against pending-reserved-deposit limits from the moment it's revealed, so capacity can't be double-booked between reveal and acceptance.
- Precondition: A fresh SegWit deposit is revealed with the reservation
  flag set.
- Trigger: `revealDepositWithExtraData` (Bridge deposit path) stores the
  position under `reservations[key]` with `owner` set, state `Unknown`
  until acceptance.
- Expected behavior: Deposit is tracked as a UTXO reservation candidate;
  counts against `pendingReservedDeposits` until acceptance settles or the
  deposit is marked stale.
- Acceptance criteria: `pendingReservedDeposits()` increments by exactly one on reveal and decrements exactly once when the position is either accepted or marked stale (`notifyStaleReservedDeposit`/`forceStaleReservedDeposit`).
- Current status: **implemented** — landed on `-ru`.
- Test level needed: unit + integration (deposit-reveal producer stub is
  out of scope for this repo; consumed as given).

### Story 2: Reservation Owner requests acceptance
- Actor: Reservation Owner (or any caller on the owner's behalf; the call
  itself is permissionless, capacity is what's gated)
- Goal: Turn a revealed reserved deposit into a Pending acceptance action against a wallet with capacity headroom.
- Benefit: Locks in capacity and a settlement deadline for the deposit, so a subsequent SPV proof can settle it to `Active` without a capacity race against other acceptances.
- Precondition: A revealed reserved deposit exists and is not yet accepted
  or marked stale; global/per-wallet caps have headroom.
- Trigger: `ReservationRouter.requestReservationAcceptance`.
- Expected behavior: Reserves capacity against the designated wallet and
  the global `activeReservationsCount`/`reservationTotalAmount` caps,
  creates a Pending `ReservationAction` (Acceptance), snapshots
  `termSeconds`/`dissolutionDelay` for later settlement.
- Acceptance criteria: capacity counters are reserved atomically with action creation (no window where capacity is spent but no action exists); a request that would exceed a cap reverts instead of partially reserving.
- Current status: **implemented** — landed on `-ru`, including the
  snapshot fields that fix the late-settlement-uses-live-parameter bug.
- Test level needed: unit (covered, `Bridge.ReservationAcceptanceAuthorization.test.ts`).

### Story 3: Custodian wallet proves acceptance (SPV)
- Actor: Wallet signing group / custodian
- Goal: Settle a Pending acceptance action to `Active` by proving the anchor deposit was actually made.
- Benefit: Converts a capacity reservation into a live, custodied reservation the owner can rely on, with the Bridge's own record of the anchor outpoint.
- Precondition: Acceptance action is Pending and not timed out.
- Trigger: `ReservationRouter.submitReservationAcceptanceProof`.
- Expected behavior: Validates the SPV proof against the deposit output,
  settles the reservation to `Active`, writes `anchorAmount`/`anchorTxHash`/
  `expiresAt`/`dissolutionEligibleAt`, emits `ReservationAccepted`.
- Acceptance criteria: settlement uses the action-record snapshot of `termSeconds`/`dissolutionDelay` taken at request time, not the live governance parameter, even for a late-arriving proof within the bounded settlement window.
- Current status: **implemented** — landed on `-ru`
  (`ReservationProofs.sol`). Bounded late-settlement window (settle only
  within `termSeconds` after timeout) correctly uses the action-record
  snapshot, not the live governance parameter.
- Test level needed: unit (covered, `Bridge.ReservationAcceptanceAuthorization.test.ts`, `Bridge.ReservationSettlement.test.ts`).

### Story 4: Acceptance authorization times out
- Actor: Anyone (permissionless)
- Goal: Release a Pending acceptance's reserved capacity if the designated wallet never signs within the timeout window.
- Benefit: Prevents a non-responsive wallet from permanently locking capacity that a later acceptance attempt could use.
- Precondition: Acceptance action Pending, `timeoutAt` elapsed, no proof
  submitted.
- Trigger: `ReservationRouter.notifyReservationAcceptanceTimedOut`.
- Expected behavior: Releases the reserved capacity, marks the action
  TimedOut, position stays available for a later acceptance request. This
  router-level forwarder was added during PR review (see gap-analysis doc,
  "router surface table under-counts the retained entry points") — without it there
  was no permissionless release path if the designated wallet never signs.
- Acceptance criteria: calling before `timeoutAt` reverts; calling after releases exactly the capacity the original request reserved and leaves the underlying deposit eligible for a fresh acceptance request.
- Current status: **implemented** — landed on `-ru`.
- Test level needed: unit (covered, `Bridge.ReservationAcceptanceAuthorization.test.ts`).

### Story 5: Governance (via wallet or otherwise) re-anchors a healthy reservation
- Actor: Governance (privileged=true required for a `Live` source wallet)
- Goal: Rotate a healthy, aged reservation's custody to a new `Live` wallet at governance's discretion, with no time bound on when this can happen.
- Benefit: Gives governance an operational lever to redistribute custody load across wallets, and — since M1 ships no dissolution or redemption exit — is the reservation's only path off an aging position while its wallet stays healthy.
- Precondition: Reservation Active, source wallet `Live`, target wallet
  `Live` and different from source, target has capacity headroom, source
  reservation not past cooldown.
- Trigger: `ReservationRouter.requestReservationReanchor`.
- Expected behavior per spec: unbounded in time (variant B deletes the
  dissolution-eligibility gate on re-anchor, `m1-b-implementation.md:190-192`).
- Acceptance criteria: a reservation aged past its (still-written, unread-in-m1) `dissolutionEligibleAt` timestamp still re-anchors successfully; `RequestNonce` increments and `ReservationReanchorRequested` emits on success.
- Current status: **implemented** — the `dissolutionEligibleAt` gate that
  previously blocked this (tracked as a Major finding, see
  `01-gap-analysis.md`) was removed by PR
  [#1120](https://github.com/threshold-network/tbtc-v2/pull/1120)
  (`m1/reanchor-dissolution-gate-fix`, merged 2026-09-03). The current
  `requestReservationReanchor` has no dissolution-eligibility check at all
  — re-anchor is unbounded in time for both privileged and permissionless
  callers, matching the spec.
- Test level needed: unit (covered — the test previously asserting a
  revert past `dissolutionEligibleAt` was rewritten by PR #1120 to assert
  the opposite; see `Bridge.ReservationStrandingLibrary.test.ts`).

### Story 6: MovingFunds/Closing wallet re-anchors its own reservation away
- Actor: Anyone (permissionless once source is `MovingFunds`/`Closing`)
- Goal: Move a reservation off a wallet that's in the process of shutting down, without waiting for governance.
- Benefit: Lets the source wallet finish closing (Story 8's guard needs its reservation count at zero) without a governance intervention for the common, non-privileged rotation case.
- Precondition: Same as Story 5 minus the `privileged` requirement.
- Trigger: same entry point.
- Expected behavior: source wallet vacates its reservations onto a `Live`
  target before it finishes closing, so `finalizeWalletClosing`'s
  reservation-count guard (Story 8) can eventually pass.
- Acceptance criteria: a non-privileged caller succeeds when source is `MovingFunds`/`Closing` and fails with "Only governance can rotate a Live wallet's anchor" if source is still `Live`.
- Current status: **implemented** — landed on `-ru`.
- Test level needed: unit (covered).

### Story 7: Custodian wallet proves a re-anchor (SPV)
- Actor: Target wallet signing group
- Goal: Settle a Pending re-anchor action by proving the new anchor transaction, moving custody to the target wallet.
- Benefit: Completes the rotation started by Story 5/6 with an on-chain proof, so the Bridge's `walletPubKeyHash` record for the reservation always matches the wallet that can actually sign for it.
- Precondition: Re-anchor action Pending, current source anchor unspent.
- Trigger: `ReservationRouter.submitReservationReanchorProof`.
- Expected behavior: Validates SPV proof, moves custody to the target
  wallet, updates `walletReservationsCount`/`walletReservationsAmount` on
  both wallets, accumulates `cumulativeReanchorFee`, sets
  `reanchorCooldownUntil`, emits `ReservationReanchored`.
- Acceptance criteria: source wallet's count/amount decrease and target wallet's increase by the same amounts in the same transaction; `cumulativeReanchorFee` accumulates the Bitcoin miner fee spent, bounding repeated-reanchor fee grinding.
- Current status: **implemented** — landed on `-ru`, including the
  fee-grinding cap fix (`cumulativeReanchorFee`, cooldown) added during PR
  review after the original spec ledger was written (see gap-analysis doc
  doc-staleness items).
- Test level needed: unit (covered, `Bridge.ReservationSettlement.test.ts`,
  `Bridge.ReservationSourceAnchorBinding.test.ts`).

### Story 8: Wallet fully closes only after shedding all reservations
- Actor: Bridge (via `notifyWalletClosingPeriodElapsed`)
- Goal: Block a wallet from finishing its closing transition while it still custodies live reservations.
- Benefit: Guarantees a `Closed` wallet never holds a reservation anchor with no signing group left to prove a re-anchor or stranding notification against — every reservation is re-anchored or stranded before the wallet fully closes.
- Precondition: Wallet in `Closing`, closing period elapsed.
- Trigger: `Wallets.notifyWalletClosingPeriodElapsed`.
- Expected behavior: Reverts with "Wallet still custodies reservations" if
  `walletReservationsCount[wallet] != 0`; otherwise finalizes to `Closed`.
- Acceptance criteria: a wallet with `walletReservationsCount > 0` cannot transition to `Closed` through any call path; the guard sits at `finalizeWalletClosing`'s sole call site, so there's no bypass.
- Current status: **implemented** — landed on `-ru`; confirmed the guard
  sits at the only call site of `finalizeWalletClosing`, so no closing path
  can bypass it.
- Test level needed: unit.

### Story 9: Reservation on a terminated wallet is stranded
- Actor: Anyone (permissionless)
- Goal: Close out a reservation whose custodying wallet can no longer sign for it, converting the owner's position to an ordinary pooled claim.
- Benefit: Prevents an owner's minted tBTC from being permanently frozen behind a dead wallet — the claim survives, just without the dedicated anchor.
- Precondition: Wallet `Terminated`, or `Closed`, or (`Closing` and
  `dissolutionEligibleAt` has passed); reservation `Active` (not
  mid-action).
- Trigger: `ReservationRouter.notifyReservationStranded`.
- Expected behavior: Releases capacity (count/amount/total), deletes the
  reverse anchor index, closes the position to `Stranded` (m1's only
  terminal state), emits `ReservationStranded(key, wallet, owner,
  anchorAmount)`. The owner's minted tBTC balance never changes; it becomes
  an ordinary pooled claim.
- Acceptance criteria: the call succeeds for a `Terminated` wallet, for a `Closed` wallet, and for a `Closing` wallet only once `dissolutionEligibleAt` has passed — and reverts for any other wallet state.
- Current status: **implemented** — landed on `-ru`.
- Test level needed: unit (covered, `Bridge.ReservationStranding.test.ts`).

### Story 10: Governance updates reservation parameters/caps
- Actor: Governance, via `BridgeGovernance`'s staged begin/finalize delay
- Goal: Change reservation term/fee/cap parameters without any bypass of the standard governance delay, and without accepting an internally inconsistent parameter set.
- Benefit: Lets governance tune caps and fees as the system scales while keeping the same timing guarantees and validation rigor every other Bridge parameter update gets.
- Precondition: n/a (any governance-delay-elapsed staged update).
- Trigger: `BridgeGovernance.finalizeReservationParametersUpdate` /
  `finalizeReservationCapsUpdate`, which call
  `ReservationRouter.updateReservationParameters`/`updateReservationCaps`
  (`onlyGovernance`).
- Expected behavior: Validates all relational constraints (8 checks, see
  gap-analysis doc), including the cap-vs-slot-capacity cross-invariant
  added since the spec ledger was written, then applies.
- Acceptance criteria: a parameter set violating any of the 8 relational checks (e.g. `reservationMaxTotalAmount > maxActiveReservations * reservationMaxSingleAmount`) reverts instead of applying.
- Current status: **implemented**, and stronger than the spec ledger
  currently documents (D-2 resolved beyond what `milestone-inventory.md`
  records).
- Test level needed: unit (covered,
  `Bridge.ReservationAcceptanceAuthorization.test.ts` governance-parameter
  section).

## Out of M1 scope (M2, confirmed correctly deferred)

- Redemption settlement (Bridge-side `ProofType.Redemption` proof
  handling) — `ActionType.Redemption` is declared (numeric position kept)
  but never constructed in m1.
- Dissolution request/settlement — `requestReservationDissolution` is
  absent from the router by design (variant B's "one genuine saving");
  `ActionType.Dissolution` and the Dissolution branch of
  `unwindPendingAction` are shipped (per D-10, all four branches ship) but
  unreachable, since nothing ever creates a Dissolution action.
- Renewal (`extendReservation`) — absent from the router by design.
- Watchtower veto (`notifyReservedRedemptionVeto`) — absent from the
  router; `watchtowerDefaultDelay`/`LevelOneDelay`/`LevelTwoDelay` action
  fields are declared but only ever written/read by the m2 redemption
  request path.
- **In-kind redemption entry points (`redeemReservation`/`retryRedeemReservation`)
  — out of M1 scope by design (Option B, 2026-09-24), not a pending
  implementation gap.** An earlier draft of this milestone (D-13) called for
  the M1 `ReservationVault` to ship both functions gated behind a
  `redemptionsPaused` flag, so M2 could simply flip the flag. The
  2026-09-24 Option B decision (`m1-b-implementation.md` §3/§4.2, commit
  `661b06a04`) supersedes that plan: the M1 vault ships deliberately
  minimal (commit `4d549e64`, confirmed in code — no `redeemReservation`,
  `retryRedeemReservation`, or `redemptionsPaused` machinery of any kind
  exists in the current `ReservationVault.sol`), and M2 delivers
  redemption/renewal via a **new vault deployment plus a depositor
  migration ceremony** instead, because the M1 vault is not
  proxy-upgradeable and re-pointing `reservationVault` requires total
  system quiescence (`reservationTotalAmount == 0 && pendingReservedDeposits == 0`).
  There is no M1 owner-facing story here because the entry points do not
  exist on this vault by design; see `../m1-keep-core-readiness/02-user-stories.md`
  "Out of M1 scope" for the keep-core-side implication (no coordinator/executor
  wiring for redemption either).

## Coverage summary

| Level | Stories with test coverage confirmed |
| :--- | :--- |
| Unit | 9 / 10 (Stories 1, 10 assessed at contract level, not story-specific test files) |
| Integration | 0 (deposit-reveal producer and full end-to-end flow are exercised via the same unit suites; no separate integration harness) |
| E2E | 0 |

0 stories have open implementation gaps as of the current tip (`9f8f5ef1`,
2026-09-28). Story 5's `dissolutionEligibleAt` gate deviation (tracked as a
Major finding in `01-gap-analysis.md`) was fixed by PR #1120. The
in-kind-redemption vault gap tracked against the earlier draft of Story 11
is superseded by the Option B decision, not an open gap — see "Out of M1
scope" above.
