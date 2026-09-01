# Delta Changes: keep-core M1 Reservation Readiness

This document enumerates the concrete keep-core changes needed to close the gaps identified in Phase 1 (01-gap-analysis.md) and fulfill the M1 user stories (02-user-stories.md).

**Scope Rule:** This list is strictly limited to M1-in-scope (creation, custody, re-anchor). Redemption, dissolution, renewal, and watchtower veto are M2 and are not covered.

| File | Change | Traces to | Test needed |
| :--- | :--- | :--- | :--- |
| `pkg/maintainer/spv/reservation_wiring.go` | Wire `submitReservationReanchorProof` to the `WalletMovingFunds` wallet-state-change event, following the pattern for `WatchWallet` registration. | 01-gap-analysis.md Major row 2 / S6 | Integration: Verify `WalletMovingFunds` trigger calls `SubmitReservationReanchorProof`. |
| `pkg/maintainer/spv/reservation_action_timeout_watch.go` | Replace `ReservationActionTimeoutWatcher.Run()`'s placeholder loop with a real integration watcher invoking `CheckReservationActionTimeouts`. | 01-gap-analysis.md Minor row 1 | Unit: Verify the loop correctly triggers `CheckReservationActionTimeouts` for expired actions. |
| `pkg/tbtc/reservation.go` | Switch `ReservationAnchorProposal` and `ReservationReanchorProposal` implementations of `Marshal()` and `Unmarshal()` from JSON to protobuf. | 01-gap-analysis.md Major row 1 | Unit: `TestReservationProposals_MarshalingRoundtrip` updated to use proto schema. |
| `pkg/tbtcpg/reservation_acceptance.go` | Extract `buildReservationAnchorTransaction` logic into a shared helper accessible by both `tbtc` and `tbtcpg`, or add an explicit cross-reference test asserting byte-identical output. | 01-gap-analysis.md Minor "Redundant anchor assembly code" | Unit/Integration: Add test asserting `tbtcpg` and `tbtc` logic produce identical transaction bytes for identical inputs. |
| `pkg/chain/ethereum/tbtc.go` | Add regression test in `pkg/chain/ethereum/tbtc_test.go` asserting correct mapping for `ReservationParameters` converter. | 01-gap-analysis.md Minor "ReservationParameters converter mapping" | Unit: Assert full 10-tuple mapping between Solidity struct and Go representation. |
| `pkg/chain/ethereum/tbtc.go` | Add regression test in `pkg/chain/ethereum/tbtc_test.go` ensuring `GetReservation` converter handles fees correctly. | 01-gap-analysis.md Minor "GetReservation converter fee field drop" | Unit: Assert fee field persistence during conversion (or explicit justification if drop remains). |
| `pkg/tbtc/reservation.go` | Add regression test in `pkg/tbtc/reservation_test.go` for `assembleReservationAnchorTransaction`. Spot-checked: existing call at `reservation_test.go:587` only exercises the nil-deposit error path ("deposit is required"), not happy-path output shape. | 01-gap-analysis.md Minor "ReservationAnchorProposal assembler logic" | Unit: Verify anchor transaction construction and inputs against known-good transaction. |
| `pkg/tbtc/reservation.go` | Add regression test in `pkg/tbtc/reservation_test.go` for `assembleReservationReanchorTransaction`. | 01-gap-analysis.md Minor "ReservationReanchorProposal assembler logic" | Unit: Verify re-anchor transaction construction and inputs against known-good transaction. |
| `pkg/tbtcpg/reservation_acceptance.go` | Add tests in `pkg/tbtcpg/reservation_acceptance_test.go` for boundary conditions of `MaxReservationsPerWallet`, `ReservationMinAmount`, and `ReservationMaxTotalAmount`. Spot-checked: `TestReservationAcceptanceTask_BoundedLookback` (`:458-530`) sets these fields as fixture data for a block-lookback-window test, it does not exercise at-limit/over-limit boundary enforcement. | S7 | Unit: Verify boundary-value cap enforcement (at-limit, one-over-limit). |
| `pkg/chain/ethereum/tbtc.go` | Add tests in `pkg/chain/ethereum/tbtc_test.go` for `ValidateReservationAnchorProposal` and `ValidateReservationReanchorProposal`. | S8 | Unit: Verify security validation against various chain states. |
| `pkg/tbtcpg/reservation_acceptance.go` | Add test in `pkg/tbtcpg/reservation_acceptance_test.go` demonstrating live fetching of `ReservationParameters` during generation. | S9 | Unit: Assert `rat.chain.ReservationParameters()` is called live, not cached, between sequential generation attempts with different parameter values. |

## New findings not in 01/02
None discovered during this delta-change planning phase.
