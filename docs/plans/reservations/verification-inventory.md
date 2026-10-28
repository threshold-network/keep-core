# Verification inventory: reservations PRs

Baseline: keep-core PR #4282 at `981d1788` and tbtc-v2 PR #1116 at `eec999aa`. Measurements and approved tooling additions are separate below.

## Inspected public PR heads

The inspected PR heads use example-based tests and static analysis. Go also has randomized round-trip tests. Neither head has reservation-specific coverage-guided fuzzing, mutation testing, or formal verification. The approved additions below live on separate tooling branches, not on these public heads.

The observed check lists contained only CodeRabbit. This is not evidence that tests or static analysis passed. Both PRs had merge conflicts; the cause of the absent workflow checks was not established.

## tbtc-v2 (contracts)

| Kind | Tool | What it covers in the PR |
|---|---|---|
| Unit and integration tests | Hardhat with mocha, chai and waffle | Reservation lifecycle, authorization, caps, occupancy, proof settlement, stranding, vault accounting, and wallet proposal validation |
| Structural checks | Custom Hardhat tests | Storage-layout snapshot, reservation ABI snapshot, router selector disjointness, router-to-Bridge storage parity, 32 tests for the mainnet calldata generator |
| Static analysis | Slither 0.9.0 in `contracts.yml` | Whole `contracts/` tree except tests; the reservation contracts carry 15 `slither-disable` comments (Reservation 3, ReservationProofs 4, ReservationVault 8) |
| Lint | solhint, eslint, prettier | Style only |
| Bytecode size | `hardhat-contract-sizer` and the optimizer `runs` setting | Bridge is at 23,567 of 24,576 bytes |
| Fork rehearsal | Scripts on private branches only | Mainnet and Sepolia fork dry-runs, 23 pass and 3 skip; not in CI |

- Coverage: no `solidity-coverage` dependency at the inspected public head. A test mentioning an entry point does not establish line or branch coverage.
- Fuzzing: no reservation campaign at that head. `randomBytes` generates fixture values only. The older private Medusa campaign at `dev` commit `8f38e4fb` targeted existing bridge and vault code, not the reservation contracts.

## keep-core (Go client)

| Kind | Tool | What it covers in the PR |
|---|---|---|
| Unit tests | `go test` through gotestsum; total coverage gate 14% | About 230 new reservation tests: acceptance, re-anchor, proof loop, timeout and stranding watchers, wiring, marshaling |
| Randomized round-trip | `google/gofuzz` through `pbutils.FuzzFuncs` | 2 new tests: random anchor and re-anchor proposals marshal and unmarshal to equal values. Random values only; not native `testing.F` targets and not coverage-guided |
| Input-rejection tests | Hand-written | Nil fields, zero hash, zero fee, oversized reservation key |
| Static analysis | gosec (`@master`, excludes G115 and G118), staticcheck 2025.1.1, `go vet`, gofmt | Whole client. `go vet` runs with no package argument, so it checks only the root package (older than the PR) |
| Artifact checks | Two new CI jobs in `client.yml` | One checks reservation ABI fallback artifacts and generated bindings against pinned tbtc-v2 contracts. The other checks the vendored btcec copy and its tests against its own upstream source. |
| Benchmarks | `client-bench` with benchstat | Performance only |

Measured statement coverage at the PR head (`go test -coverprofile`):

| Package | Package total | Reservation files |
|---|---|---|
| `pkg/tbtcpg` | 79.8% | `reservation_acceptance.go` 89.6%, `reservation_reanchor.go` 84.1% |
| `pkg/maintainer/spv` | 82.7% | 67.6% (`reservation_reanchor_proof.go`) to 92.9% (`reservation_event_scan.go`) |
| `pkg/tbtc` | 77.6% | `reservation.go` 64.5% |
| `pkg/chain/ethereum` | 22.1% | not split out |

Total over the 11 reservation files: 83.4% of 1,568 statements.

## Existing private verification outside reservations

| Tool | Where | Covers reservations |
|---|---|---|
| About 43 native Go fuzz targets, ClusterFuzzLite per PR and nightly | the private tooling fork | No; there is a generic coordination-message target, none for the anchor or re-anchor proposal types |
| `rapid` property tests | the private tooling fork | No (retry selection, redemption event conversion) |
| fizz (Medusa/Echidna) suites and CI | the private tooling fork, ecdsa and random-beacon contracts | No |
| Lean 4 proofs and Go formal-verification tests | the private tooling fork (open PR) | No (beacon relay, DKG, timing) |
| Audit tooling branches E1-E5 | local tooling branches, done | No reservation targets |
| Mutation testing | None at the inspected heads | The new pilot is separate below. Red regression tests validate individual fixes, not the entire suite. |

## Approved additions

The human approved all four items. They do not change production logic.

### Solidity coverage

Branch: tbtc-v2, coverage-tooling branch, based on `d20cf74ba`.
Tool: `solidity-coverage` 0.8.17. The coverage script selects Prague and
permits instrumented oversized contracts; normal tests retain deployment limits.

| Contract | Statements | Branches |
|---|---|---|
| Reservation.sol | 98.7% | 86.3% |
| ReservationProofs.sol | 98.1% | 84.8% |
| ReservationRouter.sol | 90.5% | 50.0% |
| ReservationVault.sol | 100% | 97.6% |

The full coverage run had 3,496 passing and 19 failing tests. Eighteen
failures were gas assertions under instrumentation. One was a pre-existing
private-merge fixture conflict with the reservation feasibility guard.
Normal stranding tests passed 12/12; the previously out-of-gas coverage
suite passed 34/34.

See link: [coverage report](../../security/pr-4282-audit/verification/solidity-coverage.md).

### Native Go fuzzing

Branch: keep-core, fuzz-tooling branch, based on `58bb4c694`.
Ten native `testing.F` targets check wire round trips, transaction matching,
fees, feasibility, and re-anchor headroom. Each ran for at least 60 seconds.
The retained logs show 1,723,870 executions and no invariant violations.
Parent checks passed vet, seed tests, and two short fuzz campaigns.

The targets do not cover distributed signing or live-chain proof submission.
Interesting corpus-entry counts are not source coverage percentages.
See link: [Go fuzz report](../../security/pr-4282-audit/verification/go-reservation-fuzz.md).

### Mutation pilot

Gremlins generated Go mutants; slither-mutate generated Solidity mutants.
The pilot used isolated, pinned worktrees and existing example tests.
No automatic mutation CI gate was added.

SPV tests killed 351 mutants and missed 64; proposal-generation tests
killed 229 and missed 38. Timeouts and uncovered sites are reported
separately. Transaction assembly had only two conclusive verdicts.

The vault tests killed 106/119 viable mutants. Agent independently
confirmed that all 48 vault tests pass with a wrong partial-repayment
operation. Several other survivors were equivalent or diagnostic-only.
Bridge-library mutation samples were too small to support broad assurance.
Router scores came from only two selected test files.

See link: [mutation report](../../security/pr-4282-audit/verification/mutation-pilot.md).

### Reservation state-machine invariants

Branch: tbtc-v2, fuzz-tooling branch, based on
`d20cf74ba`. One handler drives the real `Reservation` and
`ReservationProofs` code, run by both forge invariant tests and Medusa.
Nine properties check active count, total amount, per-wallet counters,
third-party holds, pending deposits, the anchor index, final action
states, caps at request time, and Bank credits and fee financing.
A fire test shows each property fails on corrupted state.

The forge `ci` profile passed 256 runs (25,600 calls). A 30-minute Medusa
campaign passed all 9 properties over 801,028 calls. Medusa reached 97.2%
of `Reservation.sol` lines and every internal `ReservationProofs`
function the harness calls, except the trusted-vault credit branch.

The harness skips `validateProof` (SPV) and copies the bodies of the two
`submit*Proof` functions. Reveal and wallet transitions are modelled.
The accounting properties check safety. Hand-off liveness is checked by a
separate property, described below.

See link: [invariant suite report](../../security/pr-4282-audit/verification/medusa-reservations.md).

### Follow-ups done

| Item | Result |
|---|---|
| Vault partial-repayment test | tbtc-v2 `d9abace82`. Fails against the `%=` mutant, passes on real code (49 passing). |
| Acceptance matcher boundary tests | keep-core, the reservation review branch. Zero-fee and exact-`TxMaxFee` subtests each fail against their mutant and pass on real code. |
| `Bridge.Parameters` fixture | tbtc-v2 `b4d9fe8aa`. 95 passing, 0 failing. |
| Hand-off liveness property | tbtc-v2, the fuzz-tooling branch. Medusa-only, expected to fail. Three Foundry tests show it agrees with the real re-anchor code. |
| CI adoption plan | link: [plan](../../security/pr-4282-audit/verification/ci-adoption-plan.md). Needs four decisions from the human. |

### New finding from the liveness property

A position whose anchor is at most `reservationMinAmount + reservationTxMaxFee` passes acceptance but fails the re-anchor amount floor, so it can never be handed off. Deposits at or near the minimum produce such positions. If the custodian retires, the position has no exit until the wallet closes or is terminated, or the closing wallet's dissolution date passes; then stranding turns it into a pooled claim. Medusa found this in 4 calls. It was introduced by the reservations PR, and a fix needs a design choice. Details: link: [invariant suite report](../../security/pr-4282-audit/verification/medusa-reservations.md).

Fixed (human decision: raise the acceptance floor). Acceptance now requires `deposit.amount > minAmount + 2 * txMaxFee`, so every new position can make its first hand-off. Contract `d4397f208` and SDK `dc5c2ed71` on tbtc-v2, the integrated reservation branch; spec in the reservation specification work (keep-core). Each code change has a test that failed before it. After the fix, Medusa's shortest liveness failure is the capacity case; a second hand-off can still be blocked after two full-fee hops.

Raw `.log` files are committed under `docs/` with a `.log.txt` suffix,
because `docs/.gitignore` ignores `*.log`.

## Remaining assurance limits

- No formal proof of reservation safety or liveness was added.
- High source coverage does not establish correct call ordering or safety
  across concurrent wallets, late proofs, and governance updates.
- Tooling branches separate from the PRs do not supply passing checks on the public PRs.
