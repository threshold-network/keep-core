# Solidity coverage of the reservation contracts

- Repo: tbtc-v2, branch a coverage-tooling branch, head `020220857`, based on `29e84a35c` (the integrated reservation branch):
  - `7cdd94fd4` adds `solidity-coverage` 0.8.17, the `coverage` script and `.solcover.js` (skips `contracts/test/`; lcov, json and text reports into `coverage/`).
  - `7a9a676d1` adds a coverage-only hardfork pin.
  - `967caa2f8` applies the pin only when `COVERAGE_HARDFORK` is set. The earlier form broke every normal Hardhat run (HH11 "Invalid hardfork name undefined").
  - `020220857` documents the coverage command, artifacts, and instrumentation limits.
- Raw data: `lcov-29e84a35c.info`, `coverage-final-29e84a35c.json` (this directory); older base: `lcov-9d208696.info`, `coverage-final-9d208696.json`.

## How to run

```sh
cd solidity
env -u FORKING_URL -u FORKING_BLOCK NODE_OPTIONS=--max-old-space-size=6144 yarn coverage
# one file: ... COVERAGE_HARDFORK=prague npx hardhat coverage --testfiles <file>
```

`yarn coverage` sets `COVERAGE_HARDFORK=prague`. Reason: coverage compiles without the optimizer, so the largest linked-library deploys exceed the EIP-7825 per-transaction gas cap of the default Osaka hardfork (Hardhat 2.29), and two reservation suites ran out of gas in their hooks. Prague has no per-transaction cap. solidity-coverage also lifts the contract size limit; normal builds keep it.

Checks run by the orchestrator after the fix:
- `npx hardhat compile` and `Bridge.ReservationStranding.test.ts` without coverage: 12 passing.
- Coverage of `Bridge.ReservationStrandingLibrary.test.ts` (one of the two suites that ran out of gas): 34 passing.
- `yarn install --immutable` completed successfully with the existing peer-dependency warnings.

## Results (full suite, base 29e84a35c)

The full run: 3,496 passing, 19 failing. 18 failures are gas-amount checks that instrumentation breaks (BitcoinTx, StarkNet gas analysis, MaintainerProxy and LightRelayMaintainerProxy refunds). The 19th is a real test defect, see below.

| Contract | Statements | Branches | Functions | Lines |
|---|---|---|---|---|
| Reservation.sol | 98.7% | 86.3% (157/182) | 95.0% | 98.3% |
| ReservationProofs.sol | 98.1% | 84.8% (78/92) | 100% | 98.7% |
| ReservationRouter.sol | 90.5% | 50.0% (6/12) | 88.0% | 92.3% |
| ReservationVault.sol | 100% | 97.6% (41/42) | 100% | 100% |
| WalletProposalValidator.sol | 100% | 92.9% (145/156) | 100% | 100% |
| BridgeGovernance.sol | 98.4% | 95.9% (165/172) | 96.4% | 98.5% |
| Bridge.sol | 88.5% | 59.1% (39/66) | 96.8% | 93.8% |

Base 9d208696 (reservation test files only, before the ethers v6 test repair): Reservation.sol 88.4% statements and 72.0% branches, ReservationProofs.sol 85.6% and 68.5%. At that base `ReservationProofs.test.ts` failed in every hook, so its 38 tests contributed nothing.

## Uncovered branches (base 29e84a35c)

Almost all are the revert side of a `require`: no test checks that the guard rejects.

Reservation.sol (25 of 182):
- 617: refund-deadline check on acceptance request.
- 698, 702, 703: acceptance timeout notice when the action is not an acceptance, not pending, or not yet timed out.
- 850: re-anchor timeout window too short.
- 1098: release of a third-party hold when none is held.
- 1162-1163: `closeReservation` preconditions; the function is never called by any test (and has no caller in production code).
- 1225: strand check for a Terminated or Closed wallet (one side).
- 1308, 1317, 1320: force-stale on a deposit that is not pending, already swept, or has a pending acceptance.
- 1379: release to pool on an already swept deposit.
- 1464: the caps invariant when the single cap or the max active count is 0 (both sides).
- 1510-1545: governance parameter rejections: zero tx max fee, min amount not above fee, term bounds, renewal window, action timeout, zero max per wallet, zero max active, vault reassignment with a non-zero reserved total.

ReservationProofs.sol (14 of 92):
- 204: late acceptance proof after the late window.
- 363, 368: acceptance proof for an existing reservation, or for a deposit not revealed as reserved.
- 463: anchor amount below the action minimum.
- 483, 664: proof for a stale request nonce (acceptance and re-anchor).
- 621: credit through a trusted vault (the non-vault side).
- 680, 687, 695: re-anchor output to the wrong wallet, fee above the cap, anchor at or below the fee.
- 711: late re-anchor settlement while the reservation is ActionPending.
- 771: miner fee of zero in the fee financer path.
- 862: unwind of a pending acceptance action (else-if side).
- 908: source anchor mismatch.

## Finding: one test regression on the private merge branch

`test/bridge/Bridge.Parameters.test.ts:65` (updateDepositParameters, "before all" hook) fails with "Single-reservation cap below dust threshold", with and without coverage, at `29e84a35c` and on the Sepolia rehearsal branch (`e77dbc033`). The feasibility guard from `84ea4dda2` correctly rejects a new dust threshold that sits above the fixture's single-reservation cap. The test fixture needs to change (lower the new dust value, or raise the cap first); the guard is right. The public PR head does not have the guard, so it is not affected.

Fixed in `6798e8eae` on the integrated reservation branch: the test now halves the dust threshold (500,000). That stays at or below the fixture caps (1,000,000) and above the new tx max fee (300,000). `Bridge.Parameters.test.ts`: 95 passing, 0 failing (before: 90 passing, 1 failing, with 5 tests blocked by the hook). The the Sepolia rehearsal branch branch still carries the old fixture until it merges the dev-merge branch again.

## Limits

- Hardhat unit tests only; fork rehearsal scripts are not included.
- solidity-coverage counts the two sides of each `require` as branches; the revert side counts as uncovered until a test triggers it.
- ReservationRouter's uncovered branches are in the delegate-call dispatch; agent did not list them per line.
