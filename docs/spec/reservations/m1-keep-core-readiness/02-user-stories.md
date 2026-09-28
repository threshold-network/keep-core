# User Stories: M1 Reservation Readiness

*Consolidated M1 user stories: `requirements.md` §6; these are the per-repo detailed versions.*

This document enumerates the M1 reservation scenarios keep-core's client must handle. It was originally derived from direct code analysis of `reservations-epic` @ `b14a3885` (PR #4274); re-verified against `reservations-epic` @ `f66f11240` (2026-09-28, 28 commits later, PRs #4276-#4280 and #4324 included). **Scope note:** M1 is decided variant B — creation, custody and re-anchor only; redemption, dissolution, renewal and the watchtower veto are M2 (`feature-spec.md:9-15`, `milestone-inventory.md:7`, `roadmap.md` §0.1-0.2). Stories below cover only the four M1-reachable paths: acceptance, re-anchor, action-timeout, stranding, plus the cap/parameter surface acceptance depends on. M2-scoped scenarios are listed in `## Out of M1 scope` for completeness, not as stories needing M1 test coverage.

### Story 1: Reservation Acceptance Happy Path
- Actor: Reservation Owner
- Goal: Have a revealed reserved deposit turned into a live, custodied reservation without manual intervention.
- Benefit: The owner's deposit is trustlessly anchored to a wallet the moment the Bridge authorizes it, with no separate claim step.
- Precondition: Bridge issued a `ReservationAnchorProposal` (`pkg/tbtc/reservation.go:257-272`)
- Trigger: Client receives proposal and successfully constructs SPV proof.
- Expected keep-core behavior: Construct unsigned anchor transaction, assemble transaction builder (`pkg/tbtc/reservation.go:319`, `AssembleReservationAnchorTransaction`), submit to Bitcoin chain, and return to Bridge.
- Acceptance criteria: the assembled transaction has exactly one input and one output paying the target wallet (`TestAssembleReservationTransactions_HappyPathShape`); `ReservationAcceptanceTask.Run` (`pkg/tbtcpg/reservation_acceptance.go:243`) is reachable from the coordination checklist (see `01-gap-analysis.md` Blocker row, resolved) and produces a proposal that passes `ValidateReservationAnchorProposal`.
- Current status: implemented (`pkg/tbtcpg/reservation_acceptance.go:243` `Run`, the `ReservationAcceptanceTask` entry point)
- Test level needed: unit
- Why: Logic for proposal acceptance and transaction assembly is complex and needs isolated verification.

### Story 2: Acceptance Timeout
- Actor: Client
- Goal: Stop tracking an acceptance proposal once its validity window has elapsed, instead of waiting indefinitely.
- Benefit: Frees the coordinator to retry acceptance on a fresh proposal instead of getting stuck behind a stale one.
- Precondition: `ReservationAnchorProposal` issued and received (`pkg/tbtc/reservation.go:257-272`).
- Trigger: `ValidityBlocks` (`pkg/tbtc/reservation.go:280-282`) elapsed before proposal is submitted.
- Expected keep-core behavior: Client aborts acceptance; reservation state remains `ReservationStatePending` or transitions to `ReservationStateTerminated` if anchor is abandoned.
- Acceptance criteria: `CheckReservationActionTimeouts` (`pkg/maintainer/spv/reservation_action_timeout_watch.go:478`) notifies the Bridge exactly once per expired action; `Run` (`:224`) polls continuously and returns promptly on `ctx` cancellation.
- Current status: implemented and unit-tested (`pkg/maintainer/spv/reservation_action_timeout_watch.go:186` `CheckReservationActionTimeouts`; poll loop in `Run` — PR #4276). `Run` is invoked directly by `WireReservationWatchers` (`reservation_wiring.go:451-466`), which runs an initial synchronous `pollPendingActions` pass before starting the loop — fully wired in production, no outstanding integration gap.
- Test level needed: unit
- Why: Action timeout monitoring ensures proposals don't linger beyond their expiry.

**On-chain stranding precondition (`Reservation.sol notifyReservationStranded`,
`9f8f5ef1`, ~1090-1114):** the reservation must be `Active`, and the wallet
must be `Terminated`, OR `Closed`, OR (`Closing` AND
`block.timestamp >= dissolutionEligibleAt`). Stories 3-5 below cover only the
`Terminated` case (the three termination causes keep-core's watcher was
built against); `Closed` is also covered by the same triggers (see Story 3's
trigger note). The one on-chain-eligible case with **no** client trigger is
`Closing` past `dissolutionEligibleAt` — tracked as a gap, not a story, in
`01-gap-analysis.md`'s "Stranding coverage gap for
`Closing`-past-`dissolutionEligibleAt` wallets" row.

### Story 3: Stranding via Moving Funds Timeout
- Actor: Client
- Goal: Notice when a reservation's custodying wallet has been forcibly terminated and release its position.
- Benefit: The owner's minted tBTC balance is never orphaned behind a wallet that can no longer sign for it — it converts to an ordinary pooled claim.
- Precondition: Wallet is `Terminated` (one of the three on-chain-eligible
  wallet states; see the stranding-precondition note above) due to
  moving-funds timeout (`pkg/maintainer/spv/reservation_stranding_watch.go:64-66`).
- Trigger: Wallet transitions to `StateTerminated` due to moving-funds timeout — surfaced to the client via `OnWalletClosed` (`chain.go:238-264`: fires "when the wallet is closed or terminated"), the stranding startup scan, or `drainStrandingRechecks` (`reservation_action_timeout_watch.go:666-713`); all three gate on `wallet.State == StateClosed || StateTerminated`.
- Expected keep-core behavior: Client notifies Bridge via `NotifyReservationStranded` (`pkg/maintainer/spv/reservation_stranding_watch.go:28-30`).
- Acceptance criteria: `checkReservationStrandingForWallet` notifies the Bridge for every `Active` reservation custodied by the terminated wallet, and is idempotent if the Bridge already marked it `Stranded`.
- Current status: implemented (`pkg/maintainer/spv/reservation_stranding_watch.go:97`, `checkReservationStrandingForWallet`)
- Test level needed: unit
- Why: Cause-agnostic stranding watcher detects all termination causes identically.

### Story 4: Stranding via Moved Funds Sweep Timeout
- Actor: Client
- Goal: Same as Story 3, for the moved-funds-sweep-timeout termination cause.
- Benefit: Same as Story 3.
- Precondition: Wallet is `Terminated` due to moved-funds sweep timeout (`pkg/maintainer/spv/reservation_stranding_watch.go:64-66`).
- Trigger: same three trigger paths as Story 3.
- Expected keep-core behavior: Client notifies Bridge via `NotifyReservationStranded` (`pkg/maintainer/spv/reservation_stranding_watch.go:28-30`).
- Acceptance criteria: same as Story 3.
- Current status: implemented (`pkg/maintainer/spv/reservation_stranding_watch.go:97`, `checkReservationStrandingForWallet`)
- Test level needed: unit
- Why: Same cause-agnostic stranding watcher as Story 3.

### Story 5: Stranding via Fraud Challenge Defeat
- Actor: Client
- Goal: Same as Story 3, for the fraud-challenge-defeat termination cause.
- Benefit: Same as Story 3.
- Precondition: Wallet is `Terminated` due to fraud-challenge defeat (`pkg/maintainer/spv/reservation_stranding_watch.go:64-66`).
- Trigger: same three trigger paths as Story 3.
- Expected keep-core behavior: Client notifies Bridge via `NotifyReservationStranded` (`pkg/maintainer/spv/reservation_stranding_watch.go:28-30`).
- Acceptance criteria: same as Story 3.
- Current status: implemented (`pkg/maintainer/spv/reservation_stranding_watch.go:97`, `checkReservationStrandingForWallet`)
- Test level needed: unit
- Why: Same cause-agnostic stranding watcher as Story 3.

### Story 6: Re-anchor on Wallet Rotation
- Actor: Client
- Goal: Move a reservation's custody from a wallet that is rotating out (`MovingFunds`/`Closing`) to a healthy `Live` wallet, and prove it on-chain promptly.
- Benefit: The reservation survives wallet rotation without stranding, and the source wallet can finish closing once it sheds all reservations.
- Precondition: Wallet rotation triggered; new wallet is assigned.
- Trigger: `WalletMovingFunds` state transition. `ReservationReanchorTask` (`pkg/tbtcpg/reservation_reanchor.go:65-83`) generates and broadcasts the re-anchor proposal on this trigger; the gap was downstream, in SPV proof submission (`pkg/maintainer/spv/spv.go`'s generic proof loop could not supply the `(reservationKey, requestNonce)` pair `SubmitReservationReanchorProof` needs — see `01-gap-analysis.md` Major row, resolved), not in the trigger/proposal path itself.
- Expected keep-core behavior: Client constructs and submits `ReservationReanchorProposal` promptly (`pkg/tbtc/reservation.go:284-337`), then the SPV maintainer submits the re-anchor proof once the transaction confirms.
- Acceptance criteria: `submitReservationActionProof` re-derives the `(reservationKey, requestNonce)` pair for the confirmed re-anchor transaction and calls `SubmitReservationReanchorProof`, submitting exactly once per re-anchor.
- Current status: implemented (`pkg/maintainer/spv/reservation_reanchor_proof.go`: `submitReservationReanchorProof` discovers the unproven transaction via `ReservationReanchorRequested` events + `ReservationByAnchorUtxo`; `submitReservationActionProof` re-derives the key/nonce pair and submits — PR #4276)
- Test level needed: unit
- Why: Promptness is critical during rotation to avoid stranding anchors on retiring wallets.

### Story 7: Cap enforcement awareness
- Actor: Coordinator (acceptance proposal generator)
- Goal: Never propose an acceptance that would breach a governance-set cap.
- Benefit: Keeps the Bridge's on-chain caps meaningful — the coordinator self-polices instead of relying solely on the Bridge to reject an overshoot after the fact.
- Precondition: `ReservationParameters` fetched live for the current acceptance attempt.
- Trigger: A new deposit is being considered for reservation acceptance.
- Expected keep-core behavior: Reject the candidate before proposing if it would exceed `MaxReservationsPerWallet`, fall below `ReservationMinAmount`, or push `ReservationTotalAmount` over `ReservationMaxTotalAmount`.
- Acceptance criteria: at-limit values pass, one-over-limit values are rejected, for each of the three checks independently.
- Current status: implemented (`pkg/tbtcpg/reservation_acceptance.go:539` `ReservationMinAmount`, `:988-993` `MaxReservationsPerWallet`, `:1037-1052` `ReservationMaxTotalAmount`, all inside `checkReservationAcceptanceEligibility`) (line citations corrected 2026-09-28: previously cited `:424`, `:443`, `:475-478`, which are unrelated caps/deposit-min-age fetch setup code, not the boundary comparisons themselves)
- Test level needed: unit
- Why: Three independent cap checks with distinct boundary conditions — each needs its own boundary-value test (at-limit, one-over-limit).

### Story 8: Wallet proposal validation
- Actor: Client
- Goal: Confirm a generated proposal is still valid against current chain state before spending signing-group effort broadcasting it.
- Benefit: Avoids wasting a signing round on a proposal the Bridge would reject, and catches proposal-generation bugs before they reach the network.
- Precondition: An acceptance or re-anchor proposal has been generated.
- Trigger: Client validates the proposal against current chain state before broadcasting.
- Expected keep-core behavior: Call `ValidateReservationAnchorProposal` (`pkg/chain/ethereum/tbtc.go:482-512`) or `ValidateReservationReanchorProposal` (`:569-597`), both real calls into `tc.walletProposalValidator`.
- Acceptance criteria: a proposal that violates a Bridge-side rule is rejected before broadcast; both validators are real on-chain calls, not local approximations. No dedicated unit test exists yet for these two functions themselves (see `01-gap-analysis.md` Minor row, deferred — no simulated-backend test infrastructure in this package); they are exercised indirectly through `pkg/tbtcpg`'s interface-fake tests.
- Current status: implemented (`pkg/chain/ethereum/tbtc.go:482-597`) (line citations corrected 2026-09-28: previously cited `:2541-2647`, past the end of the 1,820-line file)
- Test level needed: unit
- Why: Essential security gate before any proposal is broadcast to the wallet signing group.

### Story 9: Governance parameter live refresh
- Actor: Coordinator (acceptance proposal generator)
- Goal: Always apply the current governance-set caps and fees, never a stale cached copy.
- Benefit: A governance parameter update takes effect on the very next proposal generation, closing the window where a coordinator could accept against outdated limits.
- Precondition: Governance has updated `ReservationParameters` on-chain since the last proposal.
- Trigger: A new acceptance proposal generation begins.
- Expected keep-core behavior: Fetch `ReservationParameters` fresh via `rat.chain.ReservationParameters()` for every generation, not a cached/stale copy.
- Acceptance criteria: two sequential generations with an intervening parameter change each observe the parameter value in effect at that generation, not the first generation's value (`TestReservationAcceptanceTask_ReservationParametersFetchedLive`).
- Current status: implemented (`pkg/tbtcpg/reservation_acceptance.go:339`) (line citation corrected 2026-09-28: previously cited `:133`, which is the unrelated `reservationAcceptanceFundingTxCandidate` type declaration)
- Test level needed: unit
- Why: A cached-parameters bug would silently let stale caps/fees apply after a governance update — a live-fetch-vs-cache regression is exactly what a unit test with two sequential fetches and an intervening parameter change would catch.

## Out of M1 scope

These scenarios are real and eventually need coverage, but are M2 per the settled variant B decision (`feature-spec.md:9-15`, `milestone-inventory.md:7`, `roadmap.md` §0.1-0.2) — not M1 test-readiness items. Listed for completeness only.

| Scenario | Why out of scope |
| :--- | :--- |
| Renewal window handling | Renewal is M2; "They cannot... renew" (`roadmap.md:12`); `renewalsPaused = true` at the vault (`roadmap.md:107`). |
| Dissolution (incl. `dissolutionEligibleAt` gate) | Dissolution is M2; "nothing in m1 B reads `expiresAt` or `dissolutionEligibleAt`" (`roadmap.md:77-83`). |
| Whole-redemption | Redemption is M2; no coordinator/executor wiring exists by design (see `01-gap-analysis.md` Out of M1 scope). |
| Partial-redemption (1-in-2-out remainder) | Same as whole-redemption; only the `IsPartial bool` field is retained on `ReservationAction` (`pkg/tbtc/reservation.go:218-220`, alongside the `ReservationActionTypeRedemption`/`ReservationActionTypeDissolution` enum values at `:92,94`) to satisfy the enum-positional storage rule — no partial-redemption assembler exists anywhere in `pkg/tbtc/reservation.go` (correction 2026-09-28: previously cited a non-existent assembler at `:449-552`, which is the unrelated M1-scoped `reservationAnchorAction` internal type). |
| Watchtower-delay gating / veto | M2; "vacuous — no redemptions exist" in M1 (`roadmap.md:66`). |

## Coverage summary

| Status | Count |
| :--- | :--- |
| Implemented | 9 |
| Partial | 0 |
| Missing | 0 |

| Test level | Count |
| :--- | :--- |
| Unit | 9 |
| Integration | 0 |
| E2E | 0 |
