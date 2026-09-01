# Gap Analysis: keep-core M1 Reservation Readiness

## Summary
M1 is decided variant B scope: **creation, custody and re-anchor only** — redemption, dissolution, renewal and the watchtower veto are M2 and their Bridge-side code is absent from M1 by design, not deployed-and-gated (`feature-spec.md:9-15`, `milestone-inventory.md:7`, `roadmap.md` §0.1-0.2). keep-core's PR #4274 declares the full `ReservationActionType` enum (redemption/dissolution included) only because the storage/enum-positional rule requires every variant to keep its numeric position (`milestone-inventory.md` rule #5) — this is expected, not partial redemption/dissolution support. Findings below are scoped strictly to acceptance, re-anchor, action-timeout, and stranding — the four M1-reachable paths (`roadmap.md` §0.2). Out-of-scope code (redemption/dissolution executor absence, unenforced watchtower delay) is listed in `## Out of M1 scope` for audit-trail completeness, not as gaps.

- **Blocker:** 1
- **Major:** 2
- **Minor:** 6

## Blocker

| Gap | Severity | Evidence | Spec/PR ref |
| :--- | :--- | :--- | :--- |
| CI `make generate` fails for generated bindings | Blocker | `client-build-test-publish` CI job error: `No rule to make target '_address/ReservationRouter'`. | Verified fact #2 / PR #4274 CI |

## Major

| Gap | Severity | Evidence | Spec/PR ref |
| :--- | :--- | :--- | :--- |
| Protobuf marshaling is unimplemented (JSON-only) | Major | `pkg/tbtc/reservation.go:207,257,312,365`: `TODO: Switch to protobuf-based marshaling`; affects all four proposal types including the two M1-scoped ones (`ReservationAnchorProposal` `:210`, `ReservationReanchorProposal` `:315`). | `feature-spec.md` §13 |
| Re-anchor SPV proof never submitted - generic proof-loop signature can't carry `(reservationKey, requestNonce)` | Major | The re-anchor proposal/trigger path is **not** the gap: `ReservationReanchorTask` is registered in `tbtcpg.NewProposalGenerator` (`pkg/tbtcpg/tbtcpg.go:91`) and its `Run` already checks `StateMovingFunds` (`pkg/tbtcpg/reservation_reanchor.go:65-83`) - proposals are generated and broadcast today. The real gap is SPV-side: `pkg/maintainer/spv/spv.go`'s generic `unprovenTransactionsGetter`/`transactionProofSubmitter` signatures carry only a Bitcoin transaction hash, so the `ActionReservationReanchor` proof-loop entry used a placeholder getter (returned no transactions) and a deliberately no-op submitter (`noopReanchorProofSubmitter`, own comment: "SubmitReservationReanchorProof requires the (reservationKey, requestNonce) pair that the generic proof loop cannot supply") - re-anchor SPV proofs were never submitted to the Bridge. Re-anchor is explicitly M1-in-scope (`roadmap.md` §0.2, "Re-anchor... Yes — and this is desirable"). | `feature-spec.md` §13 |

## Minor

| Gap | Severity | Evidence | Spec/PR ref |
| :--- | :--- | :--- | :--- |
| `ReservationActionTimeoutWatcher.Run()` is an admitted placeholder | Minor | `pkg/maintainer/spv/reservation_action_timeout_watch.go:141-144` doc comment: "Run is a placeholder for the integration wiring in this PR... m1 ships the synchronous `CheckReservationActionTimeouts` for one reservation key (tests + integration) and the interface surface to wire the loop in a follow-up PR." Action-timeout is M1-in-scope (`roadmap.md` §0.2, "required cleanup") and applies to acceptance/re-anchor generations, not just M2 paths — no autonomous, continuously-running watcher exists yet. | PR #4274, self-documented |
| `GetReservation` converter fee field drop | Minor | `pkg/chain/ethereum/tbtc.go:3211`: intentionally drops `CumulativeReanchorFee` in the Go-side representation; own comment states this is acceptable because "m1 has no fee-ceiling enforcement". | `milestone-inventory.md` §2.3 |
| `ReservationParameters` converter mapping | Minor | `pkg/chain/ethereum/tbtc.go:2779-2793` (`convertReservationParametersFromAbiType`): converts the ReservationRouter 10-tuple field-by-field; field count/order not yet cross-checked against the live Solidity struct in `/tmp/m1-g`. | `milestone-inventory.md` §2.6 |
| `ReservationAnchorProposal` assembler logic | Minor | `pkg/tbtc/reservation.go:398-441` (`assembleReservationAnchorTransaction`): 1-in-1-out shape present; fee-handling edge cases not yet cross-checked against tbtc-v2's current anchor tx validation in `/tmp/m1-g`. | `milestone-inventory.md` §2.2 |
| `ReservationReanchorProposal` assembler logic | Minor | `pkg/tbtc/reservation.go:581-621` (`assembleReservationReanchorTransaction`): 1-in-1-out shape present; not yet cross-checked against the current re-anchor settlement path in `ReservationProofs.sol` (`/tmp/m1-g`). | `milestone-inventory.md` §2.2 |
| Stranding watch is cause-agnostic by design | Minor | `pkg/maintainer/spv/reservation_stranding_watch.go:77-165` (`WatchWallet`/`CheckReservationStrandingForWallet`): no branching on termination cause, matching `feature-spec.md` §13's "regardless of which of three paths caused termination" framing. Not a gap — noted for completeness. | `feature-spec.md` §13 |
| Redundant anchor assembly code | Minor | `pkg/tbtcpg/reservation_acceptance.go:578-582` (`buildReservationAnchorTransaction`): own comment states "the tbtcpg package cannot call the helper directly because it lives in a different package, so the assembly logic is duplicated here. Any change to the anchor transaction shape must be applied to both sites" — two independently-maintained copies of the same tx-building logic; anchor construction is M1-in-scope. | PR #4274 |

## Out of M1 scope (verified correct, not gaps)

These four items were investigated and found "missing" or "unwired" — but per the settled variant B decision (`feature-spec.md:9-15`, `milestone-inventory.md:7`, `roadmap.md` §0.1-0.2) their absence is intentional M1 design, not a defect. Listed for audit-trail completeness only; no action needed for M1 launch.

| Item | Evidence | Why out of scope |
| :--- | :--- | :--- |
| No redemption coordinator/executor wiring | `grep -rln "ReservedRedemptionProposal" pkg/tbtcpg pkg/maintainer/spv` returns zero files; only the declare-only type/assembler (`pkg/tbtc/reservation.go:449-552`) and a stub validator exist. | Redemption is M2; "Bridge-side code is absent from m1 entirely rather than deployed-and-gated" (`feature-spec.md:11-12`). |
| Watchtower delay gate never enforced | `WatchtowerDefaultDelay`/`WatchtowerLevelOneDelay`/`WatchtowerLevelTwoDelay` declared at `pkg/tbtc/chain.go:939-941`, never consumed. | Watchtower veto is M2 and "vacuous — no redemptions exist" in M1 (`roadmap.md:66`). |
| `ValidateReservedRedemptionProposal` returns a hardcoded error | `pkg/chain/ethereum/tbtc.go:2599-2614`; own doc comment: "does not expose a `validateReservedRedemptionProposal` entry... only anchor and re-anchor validators are present at this milestone". | Confirms, not contradicts, M2 scoping — the Solidity validator for redemption is correctly absent at M1. |
| `ValidateReservationDissolutionProposal` returns a hardcoded error | `pkg/chain/ethereum/tbtc.go:2649-2665`; same pattern. | Dissolution is M2 — "nothing in m1 B reads `expiresAt` or `dissolutionEligibleAt`" (`roadmap.md:77-83`). |
