# User Stories: M1 tbtc-v2 (Solidity/Bridge) Readiness

This document enumerates the M1 reservation scenarios the tbtc-v2 Bridge
contracts must handle, derived from direct code analysis of the
`reservations-upgrade` branch (landed tip `8c5a2f4d`) plus queued PR F
(`ReservationVault`) and PR G (Bridge activation wiring). **Scope note:**
M1 is decided variant B — creation, custody and re-anchor only; redemption,
dissolution, renewal and the watchtower veto are M2
(`feature-spec.md:9-15`, `milestone-inventory.md:7`, `roadmap.md` §0.1-0.2).
Companion to `../m1-keep-core-readiness/02-user-stories.md` (Go client
side, which consumes the entry points documented here).

### Story 1: Deposit reveal reserves capacity
- Actor: Depositor (via deposit-reveal producer)
- Precondition: A fresh SegWit deposit is revealed with the reservation
  flag set.
- Trigger: `revealDepositWithExtraData` (Bridge deposit path) stores the
  position under `reservations[key]` with `owner` set, state `Unknown`
  until acceptance.
- Expected behavior: Deposit is tracked as a UTXO reservation candidate;
  counts against `pendingReservedDeposits` until acceptance settles or the
  deposit is marked stale.
- Current status: **implemented** — landed on `-ru`.
- Test level needed: unit + integration (deposit-reveal producer stub is
  out of scope for this repo; consumed as given).

### Story 2: Reservation Owner requests acceptance
- Actor: Reservation Owner (or any caller on the owner's behalf; the call
  itself is permissionless, capacity is what's gated)
- Precondition: A revealed reserved deposit exists and is not yet accepted
  or marked stale; global/per-wallet caps have headroom.
- Trigger: `ReservationRouter.requestReservationAcceptance`.
- Expected behavior: Reserves capacity against the designated wallet and
  the global `activeReservationsCount`/`reservationTotalAmount` caps,
  creates a Pending `ReservationAction` (Acceptance), snapshots
  `termSeconds`/`dissolutionDelay` for later settlement.
- Current status: **implemented** — landed on `-ru`, including the
  snapshot fields that fix the late-settlement-uses-live-parameter bug.
- Test level needed: unit (covered, `Bridge.ReservationAcceptanceAuthorization.test.ts`).

### Story 3: Custodian wallet proves acceptance (SPV)
- Actor: Wallet signing group / custodian
- Precondition: Acceptance action is Pending and not timed out.
- Trigger: `ReservationRouter.submitReservationProof` with `ProofType.Acceptance`.
- Expected behavior: Validates the SPV proof against the deposit output,
  settles the reservation to `Active`, writes `anchorAmount`/`anchorTxHash`/
  `expiresAt`/`dissolutionEligibleAt`, emits `ReservationAccepted`.
- Current status: **implemented** — landed on `-ru`
  (`ReservationProofs.sol`). Bounded late-settlement window (settle only
  within `termSeconds` after timeout) correctly uses the action-record
  snapshot, not the live governance parameter.
- Test level needed: unit (covered, `Bridge.ReservationAcceptanceAuthorization.test.ts`, `Bridge.ReservationSettlement.test.ts`).

### Story 4: Acceptance authorization times out
- Actor: Anyone (permissionless)
- Precondition: Acceptance action Pending, `timeoutAt` elapsed, no proof
  submitted.
- Trigger: `ReservationRouter.notifyReservationAcceptanceTimedOut`.
- Expected behavior: Releases the reserved capacity, marks the action
  TimedOut, position stays available for a later acceptance request. This
  router-level forwarder was added during PR review (see gap-analysis doc,
  "router surface missing one retained entry point") — without it there
  was no permissionless release path if the designated wallet never signs.
- Current status: **implemented** — landed on `-ru`.
- Test level needed: unit (covered, `Bridge.ReservationAcceptanceAuthorization.test.ts`).

### Story 5: Governance (via wallet or otherwise) re-anchors a healthy reservation
- Actor: Governance (privileged=true required for a `Live` source wallet)
- Precondition: Reservation Active, source wallet `Live`, target wallet
  `Live` and different from source, target has capacity headroom, source
  reservation not past cooldown, **and not past `dissolutionEligibleAt`**.
- Trigger: `ReservationRouter.requestReservationReanchor`.
- Expected behavior per spec: unbounded in time (variant B deletes the
  dissolution-eligibility gate on re-anchor, `m1-b-implementation.md:190-192`).
- Current status: **partially implemented, deviation confirmed** — the
  `dissolutionEligibleAt` gate is still present and unconditional (applies
  even to `privileged` callers). See gap-analysis doc, Major finding. A
  reservation that ages past its dissolution-eligible timestamp cannot be
  re-anchored by anyone, including governance, while its wallet stays
  healthy.
- Test level needed: unit (existing coverage exercises the gate as
  currently written, not the spec's intended unbounded behavior — see
  `ReservationSettlement.test.ts` "Reanchor would fall below the minimum
  reservation amount" and neighboring tests).

### Story 6: MovingFunds/Closing wallet re-anchors its own reservation away
- Actor: Anyone (permissionless once source is `MovingFunds`/`Closing`)
- Precondition: Same as Story 5 minus the `privileged` requirement.
- Trigger: same entry point.
- Expected behavior: source wallet vacates its reservations onto a `Live`
  target before it finishes closing, so `finalizeWalletClosing`'s
  reservation-count guard (Story 8) can eventually pass.
- Current status: **implemented** — landed on `-ru`.
- Test level needed: unit (covered).

### Story 7: Custodian wallet proves a re-anchor (SPV)
- Actor: Target wallet signing group
- Precondition: Re-anchor action Pending, current source anchor unspent.
- Trigger: `ReservationRouter.submitReservationProof` with `ProofType.Reanchor`.
- Expected behavior: Validates SPV proof, moves custody to the target
  wallet, updates `walletReservationsCount`/`walletReservationsAmount` on
  both wallets, accumulates `cumulativeReanchorFee`, sets
  `reanchorCooldownUntil`, emits `ReservationReanchored`.
- Current status: **implemented** — landed on `-ru`, including the
  fee-grinding cap fix (`cumulativeReanchorFee`, cooldown) added during PR
  review after the original spec ledger was written (see gap-analysis doc
  doc-staleness items).
- Test level needed: unit (covered, `Bridge.ReservationSettlement.test.ts`,
  `Bridge.ReservationSourceAnchorBinding.test.ts`).

### Story 8: Wallet fully closes only after shedding all reservations
- Actor: Bridge (via `notifyWalletClosingPeriodElapsed`)
- Precondition: Wallet in `Closing`, closing period elapsed.
- Trigger: `Wallets.notifyWalletClosingPeriodElapsed`.
- Expected behavior: Reverts with "Wallet still custodies reservations" if
  `walletReservationsCount[wallet] != 0`; otherwise finalizes to `Closed`.
- Current status: **implemented** — landed on `-ru`; confirmed the guard
  sits at the only call site of `finalizeWalletClosing`, so no closing path
  can bypass it.
- Test level needed: unit.

### Story 9: Reservation on a terminated wallet is stranded
- Actor: Anyone (permissionless)
- Precondition: Wallet `Terminated` (any of the three non-malicious
  termination causes), reservation `Active` (not mid-action).
- Trigger: `ReservationRouter.notifyReservationStranded`.
- Expected behavior: Releases capacity (count/amount/total), deletes the
  reverse anchor index, closes the position to `Stranded` (m1's only
  terminal state), emits `ReservationStranded(key, wallet, owner,
  anchorAmount)`. The owner's minted tBTC balance never changes; it becomes
  an ordinary pooled claim.
- Current status: **implemented** — landed on `-ru`.
- Test level needed: unit (covered, `Bridge.ReservationStranding.test.ts`).

### Story 10: Governance updates reservation parameters/caps
- Actor: Governance, via `BridgeGovernance`'s staged begin/finalize delay
- Precondition: n/a (any governance-delay-elapsed staged update).
- Trigger: `BridgeGovernance.finalizeReservationParametersUpdate` /
  `finalizeReservationCapsUpdate`, which call
  `ReservationRouter.updateReservationParameters`/`updateReservationCaps`
  (`onlyGovernance`).
- Expected behavior: Validates all relational constraints (8 checks, see
  gap-analysis doc), including the cap-vs-slot-capacity cross-invariant
  added since the spec ledger was written, then applies.
- Current status: **implemented**, and stronger than the spec ledger
  currently documents (D-2 resolved beyond what `milestone-inventory.md`
  records).
- Test level needed: unit (covered,
  `Bridge.ReservationAcceptanceAuthorization.test.ts` governance-parameter
  section).

### Story 11: Vault owner initiates in-kind redemption (M2, but flagged for M1 posture)
- Actor: Reservation Owner
- Precondition: Reservation Active, owner holds the minted claim.
- Trigger: `ReservationVault.redeemReservation` / `retryRedeemReservation`,
  expected to be gated by `redemptionsPaused` (paused at launch per D-13).
- Expected behavior per spec: functions exist and revert while
  `redemptionsPaused == true`; governance unpauses in m2.
- Current status: **not implemented** — see gap-analysis doc Blocker.
  Neither function exists in queued PR F's `ReservationVault.sol`; the
  pause flag and its toggle functions exist but gate nothing.
- Test level needed: n/a until the functions are written.

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

## Coverage summary

| Level | Stories with test coverage confirmed |
| :--- | :--- |
| Unit | 9 / 11 (Stories 1, 10 assessed at contract level, not story-specific test files) |
| Integration | 0 (deposit-reveal producer and full end-to-end flow are exercised via the same unit suites; no separate integration harness) |
| E2E | 0 |

Two stories (5, 11) have confirmed implementation gaps blocking correct
behavior — see `01-gap-analysis.md` for full evidence and severity.
