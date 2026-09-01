# User Stories: M1 Reservation Readiness

This document enumerates the M1 reservation scenarios keep-core's client must handle. It is derived from direct code analysis of `reservations-epic` @ `b14a3885` (PR #4274). **Scope note:** M1 is decided variant B — creation, custody and re-anchor only; redemption, dissolution, renewal and the watchtower veto are M2 (`feature-spec.md:9-15`, `milestone-inventory.md:7`, `roadmap.md` §0.1-0.2). Stories below cover only the four M1-reachable paths: acceptance, re-anchor, action-timeout, stranding, plus the cap/parameter surface acceptance depends on. M2-scoped scenarios are listed in `## Out of M1 scope` for completeness, not as stories needing M1 test coverage.

### Story 1: Reservation Acceptance Happy Path
- Actor: Reservation Owner
- Precondition: Bridge issued a `ReservationAnchorProposal` (`pkg/tbtc/reservation.go:179-192`)
- Trigger: Client receives proposal and successfully constructs SPV proof.
- Expected keep-core behavior: Construct unsigned anchor transaction, assemble transaction builder (`pkg/tbtc/reservation.go:392-441`), submit to Bitcoin chain, and return to Bridge.
- Current status: implemented (`pkg/tbtcpg/reservation_acceptance.go:56` `Run`, the `ReservationAcceptanceTask` entry point)
- Test level needed: unit
- Why: Logic for proposal acceptance and transaction assembly is complex and needs isolated verification.

### Story 2: Acceptance Timeout
- Actor: Client
- Precondition: `ReservationAnchorProposal` issued and received (`pkg/tbtc/reservation.go:179-192`).
- Trigger: `ValidityBlocks` (`pkg/tbtc/reservation.go:200-203`) elapsed before proposal is submitted.
- Expected keep-core behavior: Client aborts acceptance; reservation state remains `ReservationStatePending` or transitions to `ReservationStateTerminated` if anchor is abandoned.
- Current status: implemented (`pkg/maintainer/spv/reservation_action_timeout_watch.go:186` `CheckReservationActionTimeouts`)
- Test level needed: unit
- Why: Action timeout monitoring ensures proposals don't linger beyond their expiry.

### Story 3: Stranding via Moving Funds Timeout
- Actor: Client
- Precondition: Wallet is `Terminated` (`pkg/maintainer/spv/reservation_stranding_watch.go:64-66`).
- Trigger: Wallet transitions to `StateTerminated` due to moving-funds timeout.
- Expected keep-core behavior: Client notifies Bridge via `NotifyReservationStranded` (`pkg/maintainer/spv/reservation_stranding_watch.go:28-30`).
- Current status: implemented (`pkg/maintainer/spv/reservation_stranding_watch.go:107-165`)
- Test level needed: unit
- Why: Cause-agnostic stranding watcher (`pkg/maintainer/spv/reservation_stranding_watch.go:77-165`) detects all termination causes identically.

### Story 4: Stranding via Moved Funds Sweep Timeout
- Actor: Client
- Precondition: Wallet is `Terminated` (`pkg/maintainer/spv/reservation_stranding_watch.go:64-66`).
- Trigger: Wallet transitions to `StateTerminated` due to moved-funds sweep timeout.
- Expected keep-core behavior: Client notifies Bridge via `NotifyReservationStranded` (`pkg/maintainer/spv/reservation_stranding_watch.go:28-30`).
- Current status: implemented (`pkg/maintainer/spv/reservation_stranding_watch.go:107-165`)
- Test level needed: unit
- Why: Same cause-agnostic stranding watcher (`pkg/maintainer/spv/reservation_stranding_watch.go:77-165`) as Story 3.

### Story 5: Stranding via Fraud Challenge Defeat
- Actor: Client
- Precondition: Wallet is `Terminated` (`pkg/maintainer/spv/reservation_stranding_watch.go:64-66`).
- Trigger: Wallet transitions to `StateTerminated` due to fraud-challenge defeat.
- Expected keep-core behavior: Client notifies Bridge via `NotifyReservationStranded` (`pkg/maintainer/spv/reservation_stranding_watch.go:28-30`).
- Current status: implemented (`pkg/maintainer/spv/reservation_stranding_watch.go:107-165`)
- Test level needed: unit
- Why: Same cause-agnostic stranding watcher (`pkg/maintainer/spv/reservation_stranding_watch.go:77-165`) as Story 3.

### Story 6: Re-anchor on Wallet Rotation
- Actor: Client
- Precondition: Wallet rotation triggered; new wallet is assigned.
- Trigger: `WalletMovingFunds` state transition. `ReservationReanchorTask` (`pkg/tbtcpg/reservation_reanchor.go:65-83`) already generates and broadcasts the re-anchor proposal on this trigger; the gap was downstream, in SPV proof submission (`pkg/maintainer/spv/spv.go`'s generic proof loop could not supply the `(reservationKey, requestNonce)` pair `SubmitReservationReanchorProof` needs - see `01-gap-analysis.md` Major row 2), not in the trigger/proposal path itself.
- Expected keep-core behavior: Client constructs and submits `ReservationReanchorProposal` promptly (`pkg/tbtc/reservation.go:284-337`), then the SPV maintainer submits the re-anchor proof once the transaction confirms.
- Current status: implemented (`pkg/maintainer/spv/reservation_reanchor_proof.go`: `getUnprovenReservationReanchorTransactions` discovers the unproven transaction via `ReservationReanchorRequested` events + `ReservationByAnchorUtxo`; `reservationReanchorTransactionProofSubmitter` re-derives the key/nonce pair and submits - PR #4276)
- Test level needed: unit
- Why: Promptness is critical during rotation to avoid stranding anchors on retiring wallets.

### Story 7: Cap enforcement awareness
- Actor: Coordinator (acceptance proposal generator)
- Precondition: `ReservationParameters` fetched live for the current acceptance attempt.
- Trigger: A new deposit is being considered for reservation acceptance.
- Expected keep-core behavior: Reject the candidate before proposing if it would exceed `MaxReservationsPerWallet`, fall below `ReservationMinAmount`, or push `ReservationTotalAmount` over `ReservationMaxTotalAmount`.
- Current status: implemented (`pkg/tbtcpg/reservation_acceptance.go:424`, `:443`, `:475-478`)
- Test level needed: unit
- Why: Three independent cap checks with distinct boundary conditions — each needs its own boundary-value test (at-limit, one-over-limit).

### Story 8: Wallet proposal validation
- Actor: Client
- Precondition: An acceptance or re-anchor proposal has been generated.
- Trigger: Client validates the proposal against current chain state before broadcasting.
- Expected keep-core behavior: Call `ValidateReservationAnchorProposal` (`pkg/chain/ethereum/tbtc.go:2541-2597`) or `ValidateReservationReanchorProposal` (`:2617-2647`), both real calls into `tc.walletProposalValidator`.
- Current status: implemented (`pkg/chain/ethereum/tbtc.go:2541-2647`)
- Test level needed: unit
- Why: Essential security gate before any proposal is broadcast to the wallet signing group.

### Story 9: Governance parameter live refresh
- Actor: Coordinator (acceptance proposal generator)
- Precondition: Governance has updated `ReservationParameters` on-chain since the last proposal.
- Trigger: A new acceptance proposal generation begins.
- Expected keep-core behavior: Fetch `ReservationParameters` fresh via `rat.chain.ReservationParameters()` for every generation, not a cached/stale copy.
- Current status: implemented (`pkg/tbtcpg/reservation_acceptance.go:133`)
- Test level needed: unit
- Why: A cached-parameters bug would silently let stale caps/fees apply after a governance update — a live-fetch-vs-cache regression is exactly what a unit test with two sequential fetches and an intervening parameter change would catch.

## Out of M1 scope

These scenarios are real and eventually need coverage, but are M2 per the settled variant B decision (`feature-spec.md:9-15`, `milestone-inventory.md:7`, `roadmap.md` §0.1-0.2) — not M1 test-readiness items. Listed for completeness only.

| Scenario | Why out of scope |
| :--- | :--- |
| Renewal window handling | Renewal is M2; "They cannot... renew" (`roadmap.md:12`); `renewalsPaused = true` at the vault (`roadmap.md:107`). |
| Dissolution (incl. `dissolutionEligibleAt` gate) | Dissolution is M2; "nothing in m1 B reads `expiresAt` or `dissolutionEligibleAt`" (`roadmap.md:77-83`). |
| Whole-redemption | Redemption is M2; no coordinator/executor wiring exists by design (see `01-gap-analysis.md` Out of M1 scope). |
| Partial-redemption (1-in-2-out remainder) | Same as whole-redemption; the assembler exists (`pkg/tbtc/reservation.go:449-552`) only to satisfy the enum-positional storage rule. |
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
