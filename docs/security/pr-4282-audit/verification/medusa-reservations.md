# Stateful invariant suite for reservation accounting

- Repo: tbtc-v2, fuzz-tooling branch, based on `d20cf74ba` (the published reservation upgrade line, which includes `releaseReservedDepositToPool`). The accounting suite and the liveness property are both on that branch.
- Files: `solidity/test-foundry/reservations/` (7 Solidity files, each under 500 lines), `solidity/medusa-reservations.json`, `solidity/medusa-reservations-liveness.json`, a section in `solidity/test-foundry/README.md`.
- Test-only change. No production contract changed.
- Tools: Foundry 1.5.1 (forge invariant tests) and Medusa 1.5.1 (property mode). Both run the same handler.

## History

Two workers stopped in the research phase without writing a suite. The second worker's design used an SPV header with nBits `0x1d ffffff` and claimed it passes the work check "with probability 1 - 2^-232". That is wrong: the target is about 2^232, so a random header hash meets it with probability about 2^-24. Agent wrote the suite directly with the design below.

## Design

`ReservationInvariantHarness` holds one `BridgeState.Storage` and calls the real library code against it:

- `Reservation`: `requestReservationAcceptance`, `notifyReservationAcceptanceTimedOut`, `requestReservationReanchor`, `notifyReservationActionTimeout`, `notifyReservationStranded`, `notifyStaleReservedDeposit`, `forceStaleReservedDeposit`, `releaseReservedDepositToPool`, `updateReservationCaps`, `updateReservationParameters`, `releaseThirdPartyReanchorHold`.
- `ReservationProofs`: `loadSettleableAction`, `consumeAcceptedDeposit`, `validateAnchorOutput`, `settleAcceptance`, `prepareReservationForSettlement`, `requireCurrentSourceAnchor`, `consumeAnchor`, `parseSingleOutput`, `settleReanchorAccounting`, `unwindPendingAction`, `strandLateSettlementIfTargetWalletClosed`.
- A real `Bank` receives acceptance credits. A recorder contract receives re-anchor fee financing.

Simplifications, each stated in the harness and the README:

1. `revealReservedDeposit` writes the reveal-time records that `Deposit.revealDeposit` produces for a reserved deposit.
2. `proveAcceptance` and `proveReanchor` run the bodies of `submitReservationAcceptanceProof` and `submitReservationReanchorProof` without `validateProof`. The SPV header and merkle check is not under test. The Bitcoin input and output vectors are real and the library parses them. These two bodies are copies and must be kept in step with the library.
3. Wallet lifecycle transitions (Live, MovingFunds, Closing, Closed, Terminated) are direct state writes that keep `liveWalletsCount` in step.
4. Re-anchor requests never carry a wallet-member proof.

`ReservationHandler` is the only fuzz target. It has 14 actions: reveal, request/timeout/prove acceptance, request/timeout/prove re-anchor (owner, third party or governance), strand, stale/force-stale/release-to-pool, retire or terminate a wallet, add a wallet, change caps, change per-wallet and total limits, and warp time. Proofs pick any generation up to the current one, so late settlement and supersede paths are reachable. Starting configuration: 4 Live wallets (up to 8), max active 4, per-wallet count 2, per-wallet amount 3,000,000, single 2,000,000, total 6,000,000, third-party holds 1, fee cap 10,000, minimum 100,000, action timeout 6 hours.

## Properties

| Property | Rule | Fire test |
|---|---|---|
| Active count | `activeReservationsCount` = open positions (Active, ActionPending) + pending acceptances | `test_activeCountPropertyFires` |
| Total amount | `reservationTotalAmount` = open anchor amounts + pending acceptance amounts | `test_totalAmountPropertyFires` |
| Wallet counters | per wallet: count and amount = its open positions + pending actions targeting it | `test_walletCountersPropertyFires` |
| Third-party holds | per wallet: holds = pending third-party re-anchors targeting it | `test_thirdPartyHoldsPropertyFires` |
| Pending deposits | `pendingReservedDeposits` = deposits still carrying a pending marker | `test_pendingReservedDepositsPropertyFires` |
| Anchor index | every open position is found from its current anchor outpoint | `test_anchorIndexPropertyFires` |
| Final actions | a Settled or Superseded generation never changes | `test_finalActionsPropertyFires` |
| Caps | a successful request leaves global, per-wallet, single and hold counters within caps | `test_capsPropertyFires` |
| Credits | Bank credits = settled anchor values; financed fees = re-anchor miner fees | `test_creditsPropertyFires` |

Each fire test reaches a real state, corrupts one value (or mocks one cap or fee read), and asserts that the property returns false. All 9 pass, so no property is inert.

The caps property is checked right after each successful request. Late settlement deliberately re-takes capacity without a cap check, and governance may lower caps below current use, so a global "always within caps" rule would be false by design.

Three scripted lifecycle tests keep every property true at each step: accept, third-party re-anchor, settle, terminate and strand; acceptance timeout then late proof; re-anchor timeout then late proof.

## Results

| Run | Command | Result |
|---|---|---|
| Unit and fire tests | `forge test --match-path 'test-foundry/reservations/*'` | 13 passed (12 tests plus a 64-run invariant pass of 6,400 calls) |
| Forge invariant, `ci` profile | `FOUNDRY_PROFILE=ci forge test --match-test invariant_` | passed, 256 runs, 25,600 calls, about 1,800 calls per action |
| Medusa smoke | `medusa fuzz --config medusa-reservations.json --timeout 180` | 9/9 properties passed, 84,028 calls |
| Medusa campaign | `medusa fuzz --config medusa-reservations.json --timeout 1800` (4 workers) | 9/9 properties passed; 801,028 calls, 8,128 sequences, 2,355 branches, corpus 121, 29m57s |

No property failed, so there was nothing to triage. Medusa found no reservation accounting defect.

Medusa line coverage from the campaign:

| File | Lines | Not reached |
|---|---|---|
| `Reservation.sol` | 308/317 (97.2%) | 792-800, the wallet-member proof branch; 1430-1431, release to a non-zero vault |
| `ReservationProofs.sol` | 167/261 (64.0%) | the bodies of `submitReservationProof`, `submitReservationAcceptanceProof` and `submitReservationReanchorProof` (reproduced in the harness), and line 622, the trusted-vault credit branch |

Every internal `ReservationProofs` function the harness calls is fully covered except line 622.

Raw outputs: `medusa-raw/` beside this report (`medusa-resv-30m.log`, `medusa-resv-smoke.log`, `forge-resv-inv.log`, `forge-resv-all.log`, `lcov-reservations.info`). Medusa's corpus and HTML coverage stay in the worktree under `solidity/cache_forge/medusa-reservations/`.

## Limits

- `validateProof` (SPV) is not exercised. The copied proof bodies can drift from the library; a change to either `submit*Proof` function needs the same change in the harness.
- The wallet-member proof path and the trusted-vault credit path are not reached.
- Reveal and wallet transitions are modelled, not run through `Deposit` and `Wallets`.
- `ReservationVault` token accounting is not in this suite. The partial-repayment gap the mutation pilot found is now covered by a vault unit test (see the mutation report).
- The accounting properties check safety only. Liveness is checked separately below and is not part of the Foundry invariant run.

## Hand-off liveness property

`ReservationLivenessHandler` (on the fuzz-tooling branch) extends the handler with `property_everyRetiringPositionHasAnExit`. For every Active position on a MovingFunds or Closing wallet, the property requires an exit now:

- a re-anchor request that the capacity and amount rules accept, or
- a strand, when the wallet is Closing and the position is dissolution-eligible.

The re-anchor cooldown is ignored because it always ends. `firstStuckPosition` reports the first blocked position and the reason: `1` (no Live wallet has capacity) or `2` (the anchor is at or below the re-anchor floor `min + txMaxFee`).

The property is expected to fail with the current design. It runs only with `medusa-reservations-liveness.json` and is not in the Foundry invariant run, which CI would execute.

Three Foundry tests (`ReservationLiveness.t.sol`, all passing) check that the property agrees with the real re-anchor code:

| Test | State | Property | Real `requestReservationReanchor` |
|---|---|---|---|
| `test_holdsWhenALiveWalletHasCapacity` | custodian retires, other wallets empty | holds | succeeds |
| `test_firesWhenEveryLiveWalletIsFull` | per-wallet cap 1, one position on each of 4 wallets, custodian retires | fails, reason 1 | rejected for all 3 targets |
| same test, after governance raises the per-wallet cap to 2 | | holds | succeeds |
| `test_firesWhenTheAnchorIsAtTheReanchorFloor` | anchor = `min + txMaxFee`, other wallets empty | fails, reason 2 | rejected |

Medusa campaign (`medusa fuzz --config medusa-reservations-liveness.json --timeout 600`, 4 workers): 305,374 calls in 9m58s. The 9 accounting properties passed. The liveness property failed. Medusa shrank the failure to 4 calls: reveal a deposit at the minimum (`amountSeed` 0, so amount = `min + txMaxFee`), request acceptance, prove it with a non-zero miner fee, retire the custodian. Log: `medusa-raw/medusa-liveness.log`.

### Finding: positions near the minimum can never be handed off

- Acceptance requires `deposit.amount >= reservationMinAmount + reservationTxMaxFee` (`Reservation.sol:577-580`). The anchor may be as low as `reservationMinAmount` (`ReservationProofs.sol:452-466`).
- A re-anchor requires `anchorAmount > reservationTxMaxFee + reservationMinAmount` (`Reservation.sol:869-875`). That floor caps fee grinding across hops.
- So every position whose anchor is at most `min + txMaxFee` is accepted but can never be re-anchored. Deposits at or near the minimum produce such positions, depending on the miner fee the wallet pays.
- When the custodian retires, such a position has no exit while the wallet is MovingFunds. It can be stranded only when the wallet is Closed or Terminated, or Closing after `dissolutionEligibleAt` (term plus dissolution delay). Stranding turns the owner's claim into an ordinary pooled claim. No funds are lost, but the owner loses the in-kind position.
- Origin: introduced by the reservations PR.
- The capacity case (reason 1) is the saturation finding from the earlier investigation, now shown by execution, including the governance fix of raising the per-wallet cap.

### Fix: raised acceptance floor (human decision, 2026-10-12)

| Item | Commit (private) | Check |
|---|---|---|
| Contract: `requestReservationAcceptance` requires `deposit.amount > minAmount + 2 * txMaxFee` | tbtc-v2 the integrated reservation branch `d4397f208` | "guard 11" tests: 2 failing before the change, 3 passing after. All 21 reservation-related Hardhat suites: 834 passing. Reservation library 16,937 bytes (7,639 left); Bridge unchanged. |
| SDK pre-checks (`checkReserveFeasibility`, `checkAcceptancePreflight`) and README | `dc5c2ed71` | 2 failing before, 48 passing after; `tsc`, ESLint and Prettier clean. |
| Spec (requirements, architecture, verification EC-06, inventory rows) | keep-core, the reservation specification work (a separate worktree: the main spec worktree has another session's uncommitted edits) | text only |
| Suite tests for the new rule | tbtc-v2, the fuzz-tooling branch | 18 forge tests pass (`ci` invariant 25,600 calls). |

Unchanged on purpose: `WalletProposalValidator.sol:1049` and keep-core `reservation_acceptance.go:944` check that the anchor stays at or above `min` after the wallet's fee; that rule is still correct.

After the fix, on the fuzz-tooling branch, 10 minutes each:

- Accounting config: 9/9 properties pass, 367,887 calls.
- Liveness config: the property still fails, 388,236 calls. Medusa's shortest failure is now the capacity case: every other wallet retires, so no Live target exists (7 calls). The floor case now needs two hops that each pay the full fee (`test_secondHandOffCanStillBeBlocked`); the acceptance floor guarantees one hand-off, not every one.

Logs: `medusa-raw/medusa-floor-fix-accounting.log`, `medusa-raw/medusa-floor-fix-liveness.log`, `medusa-raw/forge-floor-fix.log`.
