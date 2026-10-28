# Mutation testing - tBTC reservations feature

Mutation campaigns over the reservation code in keep-core PR #4282 (Go) and
tbtc-v2 PR #1116 (Solidity), measuring how well the existing test suites kill
seeded faults. This is the final report; all numbers are from completed runs.

## Bases

- Go: worktree `<checkout>` (removed 2026-10-15; recreate
  with `git worktree add --detach <path> fe73e9799`), detached at
  the reservation review branch @ `fe73e97991a52b0d946754eff172d4570ae97d5c`.
- Solidity main campaigns: worktree
  `<host path>`, detached at
  the integrated reservation branch @ `9d208696588a86d0d139a6142b27ac502e356a89`.
- Solidity proof re-run: worktree
  `<host path>`, detached at
  `29e84a35c32e012ce65b45c565b4f1dca775efc4` ("test(reservations): repair
  reservation proof and validator suites that never ran under ethers v6").
  The proof mutation score is quoted from this base because at `9d208696` the
  `ReservationProofs.test.ts` file failed in a setup hook and never ran.

No production source was left modified. Every contract in both worktrees was
verified byte-identical to the pre-campaign clean backup
(`<mutation backup>/`) after the campaigns; `lib/forge-std`
and any `.tmp*` files were removed from the worktrees. No process started by
this agent is still running.

Raw outputs are copied into this directory under `raw-go/` and
`raw-solidity/` (paths listed in section 6).

## 1. Go (gremlins)

Gremlins was run with `--workers 4`, restricted to the reservation files by
excluding every other non-test file in each package (`-E` regexes). GOCACHE
was redirected to a private on-disk directory (`<gocache>`)
because `/dev/shm` (the default) is 100% full.

Two passes used timeout coefficients 5 and 8. The second pass produced far
fewer timeouts. The earlier "0 survived" result cannot establish test
strength: most mutants had received no verdict.

### Authoritative results (coefficient 8)

Verdict score = killed / (killed + survived). The conservative column also
includes timeouts in the denominator. Neither score counts untested or
non-compiling mutants as kills.

| Scope | Counted mutants | Killed | Survived | Timed out | Not covered | Verdict score | Including timeouts |
|---|---|---|---|---|---|---|---|
| SPV reservation files (8 files) | 477 | 351 | 64 | 2 | 60 | 84.6% | 84.2% |
| Proposal generation: acceptance and re-anchor | 284 | 229 | 38 | 2 | 15 | 85.8% | 85.1% |
| `pkg/tbtc/reservation.go` | 147 | 2 | 0 | 57 | 88 | 100% on only 2 verdicts | 3.4% |

The `pkg/tbtc/reservation.go` row is superseded: the run on 2026-10-17 got a verdict for 119 of
the 154 mutants (the other 35 do not compile) with a two-stage harness. See `rerun-2026-10-17.md`.

The raw proposal-generation scan also included 95 uncovered mutations in
`internal/test/` helpers. These are excluded from the production-code table.
Gremlins' `mutants_total` counts only killed and survived mutants, not every
generated mutant. All counts above come from the per-file status records.

Surviving-mutant counts by file (pass 2):

| File | survived |
|---|---|
| spv/reservation_wiring.go | 18 |
| spv/reservation_proof_loop.go | 13 |
| spv/reservation_stale_deposit_watch.go | 9 |
| spv/reservation_action_timeout_watch.go | 8 |
| spv/reservation_stranding_watch.go | 7 |
| spv/reservation_event_scan.go | 4 |
| spv/reservation_params_feasibility.go | 4 |
| spv/reservation_reanchor_proof.go | 1 |
| pg/reservation_acceptance.go | 23 |
| pg/reservation_reanchor.go | 15 |
| **Total survived (spv + pg)** | **102** |

### What the `TIMED OUT` counts mean, with evidence

- SPV timeouts fell from 348 at coefficient 5 to 2 at coefficient 8.
- Proposal-generation timeouts fell from 231 at coefficient 5 to 2 at
  coefficient 8.
- Changing both the allowance and the host load prevents attributing the
  reduction to load alone. The second pass provides more verdicts; a
  timeout remains inconclusive, not a kill or an equivalence result.
- `pkg/tbtc/reservation.go` retained 57 timed-out mutants at coefficients
  8 and 16. Only 2 mutants received verdicts. A slow suite or a mutant
  hang may cause those timeouts; the runs do not distinguish the two.
  This package does not have a useful mutation score from this pilot.

### Surviving-mutant table (Go reservation files)

Locations and operators are observed in the raw results. The proposed causes
and additional scenarios below are [INFERENCE], except for the separately
rechecked fee boundary. Survival is not proof that a mutant changes useful
behavior: diagnostic-only and equivalent mutants can also survive.

`pkg/maintainer/spv/reservation_proof_loop.go` (13)

| location | mutation | why tests miss it | test that would kill it |
|---|---|---|---|
| 179:14 | `!=` (NEG of `if txHash == since`) | cache first-index scan for `sinceTransactions` not exercised at the equal-hash boundary | a test seeding a `since` hash equal to a cached tx then asserting the returned slice start |
| 189:40 (2 mutants) | slice-offset ARITHMETIC_BASE and INVERT_NEGATIVES | cache-slice offset not asserted | test asserting the exact slice returned from the wallet-transaction cache hit path |
| 627:10 | `if err == nil` (NEG of `if err != nil`) | proof-submit error path not asserted as non-recorded | test that a failed submit is *not* added to `submittedReservations` |
| 678:10 | `fee <= 0` boundary | positive/negative fees tested, `fee == 0` not | a test where the anchor fee is exactly 0 |
| 678:53 | `TxMaxFee` boundary | exact-max-fee case not asserted | test with `fee == TxMaxFee` |
| 716:13 | sort comparator `c != 0` boundary | ordering of tied reservation keys not asserted | test asserting the generation order for tied keys |
| 716:13 | `c < 0` NEG | same as above | same |
| 718:16 | `a.pending` short-circuit NEG | pending/non-pending tiebreak not asserted | test ordering a pending and a settled generation with equal key |
| 1127:10 | refresh error path NEG | backoff refresh error not asserted | test asserting the backed-off wallet re-refresh after an error |
| 1166:70 | `RequestedAt > TimeoutAt` boundary | equal-timestamps case skipped | test with `RequestedAt == TimeoutAt` |
| 1202:52 | reanchor `fee <= 0` boundary | reanchor fee-zero case not asserted | test with a reanchor fee of exactly 0 |
| 1292:62 | `accumulatedConfirmations >= required` boundary | exact-confirmation-count case not asserted | test where confirmations reach the threshold exactly |

`pkg/maintainer/spv/reservation_reanchor_proof.go` (1)

| location | mutation | why | kill test |
|---|---|---|---|
| 100:36 | `OutputIndex >= len(Outputs)` off-by-one (BOUNDARY) | out-of-bounds rejection not asserted at the boundary | test with `OutputIndex == len(Outputs)` exactly |

`pkg/maintainer/spv/reservation_action_timeout_watch.go` (8)

| location | mutation | why | kill test |
|---|---|---|---|
| 291:19 | `interval <= 0` -> `< 0` | `interval == 0` guard not exercised | test configuring a zero poll interval |
| 307:44 | poll-error path NEG | poll failure not asserted | test forcing a `pollPendingActions` error |
| 435:9 | renotify-window boundary | `now < notifiedAt` edge not asserted | test at the exact renotify boundary |
| 436:25 | renotify-window boundary | same | same |
| 460:22 | `notifiedAt == 0` NEG | never-notified case not asserted | test with a fresh, unnotified action |
| 491:14 | `headNow >= deadline` boundary | exact-deadline case not asserted | test where head time equals the deadline |
| 499:11 | error path NEG | not asserted | test forcing the error branch |
| 676:26 | preload-reuse condition NEG | preload-reuse path not asserted | test the preload-reuse shortcut |

`pkg/maintainer/spv/reservation_event_scan.go` (4)

| location | mutation | why | kill test |
|---|---|---|---|
| 52:18 | `currentBlock > confirmations` boundary | scan-confirmation edge not asserted | test at the confirmation boundary |
| 111:16 | `startBlock > endBlock` | empty-range case not asserted | test an empty scan range |
| 117:15 | `chunkEnd > endBlock` boundary | chunk-edge not asserted | test a chunk flush at the boundary |
| 117:38 | `chunkEnd < chunkStart` | degenerate chunk not asserted | test a zero-width chunk |

`pkg/maintainer/spv/reservation_stale_deposit_watch.go` (9)

| location | mutation | why | kill test |
|---|---|---|---|
| 290:13 | `headNow <= deadline` boundary | stale-deadline edge not asserted | test head time exactly at the refund deadline |
| 342:10 / 342:41 | renotify-window boundaries | not asserted | test the stale-deposit renotify boundary |
| 444:14 | `headNow <= deadline+offset` boundary | offset edge not asserted | test with the offset exactly at the boundary |
| 553:16 | `startBlock > endBlock && len(pending)==0` | empty-pending early-exit not asserted | test an empty pending set with a reversed range |
| 611:36 | `headNow >= refundDeadline` boundary | exact-deadline case not asserted | test head time exactly at refund deadline |
| 613:9 / 614:12 | renotify-window boundaries | not asserted | test the stale-deposit renotify boundary |
| 722:14 | `interval <= 0` -> `< 0` | zero-interval guard not exercised | test a zero poll interval |

`pkg/maintainer/spv/reservation_stranding_watch.go` (7)

| location | mutation | why | kill test |
|---|---|---|---|
| 93:11 | `attempt+1` in the retry log (ARITH) | diagnostic-only output change | no regression test recommended for log wording |
| 98:14 | `attempt < max-1` retry-loop boundary | max-retry edge not asserted | test the exact retry limit |
| 98:49 | `retryDelay` (ARITH/INVERT) | retry delay not asserted | test the sleep delay |
| 213:14 | `causeErr != nil` NEG | cause-error path not asserted | test a stranding with a cause error |
| 290:62 | `NotifyReservationStranded` error NEG | notify-failure path not asserted | test a notify-stranded failure |

`pkg/maintainer/spv/reservation_wiring.go` (18)

| location | mutation | why | kill test |
|---|---|---|---|
| 189:21 | `intervalSeconds == 0` poll-interval helper | zero-interval return not asserted | test a sub-second poll interval |
| 352:9 | startup-scan error NEG | registration-scan error not asserted | test a failed registration scan |
| 378:17 | wallet-fetch retry error NEG | 3-attempt wallet fetch not asserted | test the wallet-fetch retry loop |
| 383:12 | `attempt+1` log (ARITH) | diagnostic-only output change | no regression test recommended for log wording |
| 417:12 | `attempt+1` log (ARITH) | diagnostic-only output change | no regression test recommended for log wording |
| 422:15 | check-error path NEG | not asserted | test a wallet-check error |
| 474:28 | `len(unresolvedWallets) > 0` / NEG boundary | unresolved-wallets retry trigger not asserted | test a stranding with unresolved wallets |
| 497:28 | `registrationScanOK := err == nil` NEG | scan-OK signal not asserted | test the registration-scan self-check |
| 546:90 | stale-deposit watcher restart error NEG | restart path not asserted | test a stale-deposit watcher restart |
| 560:35 | `!shouldRestart \|\| ctx.Err() != nil` watcher-restart loop NEG | restart-loop condition not asserted | test the watcher restart backoff loop |
| 600:19 | `initialErr != nil` NEG | initial action-timeout scan not asserted | test the initial action-timeout scan error |
| 609:25 | `scanOK: initialErr == nil` NEG | scan-OK signal not asserted | same |
| 615:49 | action-timeout watcher restart NEG | not asserted | test the action-timeout watcher restart |
| 742:18 | `currentBlock > lookBack` boundary | look-back start edge not asserted | test the look-back start boundary |
| 815:10 | stranding-check error NEG | not asserted | test a stranding-check failure |
| 872:11 | stranding-check error NEG | not asserted | same |
| 912:19 | `currentBlock > window` boundary | scan-window edge not asserted | test the scan window boundary |

`pkg/maintainer/spv/reservation_params_feasibility.go` (4)

| location | mutation | why | kill test |
|---|---|---|---|
| 48:17 | `revealAhead > 0` boundary | zero reveal-ahead edge not asserted | test a zero reveal-ahead |
| 49:15 | `revealAhead > term+margin` boundary | exact-margin case not asserted | test reveal-ahead equal to term+margin |
| 154:16 | `len(reasons) > 0` | non-empty-reasons flag not asserted | test infeasible params that produce reasons |
| 243:67 | `ok && pm == nil` NEG | metrics-guard path not asserted | test with a real PerformanceMetrics recorder |

`pkg/tbtcpg/reservation_acceptance.go` (23)

| location | mutation | why | kill test |
|---|---|---|---|
| 408:60 | `ReservationActionTimeout <= depositMinAge` boundary | timeout==minAge edge not asserted | test the exact-equality case |
| 460:57 | `safetyMargin >= ...` boundary | not asserted | test the margin boundary |
| 536:57 | `safetyMargin >= ...` boundary | not asserted | same |
| 549:25 | `candidatesExamined >= max` cap boundary | exact-cap case not asserted | test the candidate cap being hit exactly |
| 558:21 | `candidatesExamined++` (INCREMENT) | counter not asserted | test the examined-candidate count |
| 757:18 | `currentBlock > lookBack` boundary | not asserted | test the look-back start boundary |
| 764:43 | `chunkStart <= currentBlock` loop boundary | not asserted | test the chunk-scan loop bound |
| 766:30 | `currentBlock-chunkStart >= chunk` boundary | not asserted | test the chunk-size boundary |
| 799:23 | `RequestNonce > existing` dedupe boundary | equal-nonce case not asserted | test a dedupe with equal nonce |
| 809:26 / 810:31 / 812:28 / 813:33 | result-sort comparator (`TimeoutAt`, `BlockNumber`, ties) | sort order of tied results not asserted | test result ordering with equal timeouts/blocks |
| 815:65 | `ReservationKey.Cmp < 0` final tiebreak | not asserted | same |
| 840:43 / 841:47 | reveal-lookback `termSeconds/margin` arithmetic | not asserted | test the lookback computation |
| 844:18 | `requestBlock > lookBack` boundary | not asserted | test the look-back boundary |
| 945:16 | `minPlusFee < action.MinAmount` min-amount boundary | exact-min-amount case not asserted | test an amount exactly at the minimum |
| 1074:13 | `workers > len(candidates)` cap boundary | worker cap edge not asserted | test the worker-count cap |

`pkg/tbtcpg/reservation_reanchor.go` (15)

| location | mutation | why | kill test |
|---|---|---|---|
| 307:58 | `safetyMargin >= ...` boundary | not asserted | test the margin boundary |
| 424:69 | `AnchorUtxo.Value <= 0` boundary | value==0 case not asserted | test a zero-value anchor |
| 438:31 | `headTime < ReanchorCooldownUntil` boundary | cooldown boundary not asserted | test head time exactly at the cooldown end |
| 865:34 | `currentBlock+1` wait-loop start (ARITH) | wait-loop offset not asserted | test the reanchor wait-loop start |
| 865:51 | `blockHeight <= currentBlock+wait` boundary | not asserted | test the wait-loop bound |
| 865:116 | `blockHeight++` (INCREMENT) | loop counter not asserted | same |
| 880:31 | `RequestNonce > preRequestNonce` boundary | equal-nonce case not asserted | test a reanchor with equal nonce |
| 1051:48 | `noHeadroomFromAnchor` reuse boundary | not asserted | test the no-headroom cache reuse |
| 1199:19 | `currentBlock > ReanchorLookBack` boundary | not asserted | test the reanchor look-back boundary |
| 1314:70 | cached-wallet eviction NEG | cache-eviction path not asserted | test target-wallet cache eviction |
| 1376:32 | `mainUtxo.Value < DustThreshold` boundary | exact-dust case not asserted | test a main UTXO exactly at the dust threshold |
| 1442:9 | `err != nil` NEG | not asserted | test the below-dust notify error |
| 1485:9 | `err != nil` NEG | not asserted | test the reanchor-as-source scan error |
| 1501:9 | `err != nil` NEG | not asserted | test the reanchor-as-target scan error |

The 102 survivors are raw tool outcomes, not 102 confirmed defects or
necessarily 102 worthwhile test additions. The pilot did not establish
semantic equivalence for every Go survivor. Boundary and error-path cases
need priority; log-counter differences do not justify wording tests.

## 2. Solidity (slither-mutate, foundry compile + hardhat kill)

slither-mutate's built-in hardhat platform is broken in this build (it treats
the single `.sol` target as a directory root and appends
`/artifacts/build-info`, and the hardhat `paths.artifacts` here is `./build`
with no `build-info` subdirectory), so the campaigns forced the **foundry**
compile framework (forge builds each contract cleanly) while the kill command
is a **hardhat** test run (there are no reservation forge tests in this repo).
Per the task, a mutant that fails to compile (or breaks the 24,576-byte
limit, which the forge build enforces) is "not viable" and is excluded from
the kill/survive score.

The four requested operators were `RR` (revert), `LOR` (logical), `ROR`
(relational), and `AOR` (arithmetic). Their generated sample was uneven:
the bridge libraries had Slither IR errors, and their viable mutation sets
were small. The router kill command ran only two reservation test files,
not the full suite. Scores describe those kill sets, not every repo test.

Per-contract results:

| Contract | Base | Revert (killed/verdicts) | Tweak (killed/verdicts) | Survived | Compile-fail | Overall score |
|---|---|---|---|---|---|---|
| `ReservationVault.sol` | 9d208696 | 34/34 | 72/85 | 13 uncaught | 61 | 106/119 = 89.1% |
| `Reservation.sol` | 9d208696 | 2/2 | 4/4 | 0 | 16 | 6/6 = 100% |
| `ReservationProofs.sol` | 9d208696 (narrow kill) | 2/2 | 0/0 | 0 | 7 | 100% |
| `ReservationProofs.sol` | **29e84a35c (repaired test)** | 2/2 | 0/0 | 0 | 7 | 100% |
| `ReservationRouter.sol` | 9d208696 | 23/33 | 0/0 | 10 uncaught | 4 | 69.7% |

Mutation scope was narrower than a whole-contract campaign:
`Reservation.sol` targeted only `actionKey` and
`validateReservationCapsInvariant`; `ReservationProofs.sol` targeted only
`parseSingleOutput`. Their perfect sample scores do not measure lifecycle
or settlement accounting.

### Surviving (uncaught) mutants - Solidity

`ReservationVault.sol` (13 uncaught, all in tweak operators - no uncaught
revert mutants):

| Location | Mutation | Classification |
|---|---|---|
| 171 (ROR) | loop `< length` -> `!= length` | equivalent for the monotonic integer loop |
| 180 (ROR) | loop `< length` -> `!= length` | equivalent for the monotonic integer loop |
| 218 (ROR) | `feeSat == 0` -> `feeSat < 0` | not equivalent: removes the zero-fee early return and emits an additional event |
| 218 (ROR) | `feeSat == 0` -> `feeSat <= 0` | equivalent for unsigned integers |
| 225 (ROR) | `shortfallSat > 0` -> `>= 0` | state-equivalent when it adds zero; gas changes |
| 225 (ROR) | `shortfallSat > 0` -> `!= 0` | equivalent for unsigned integers |
| 305 (ROR) | `inKindFeeDebtSat > 0` -> `>= 0` | extra zero-amount helper call; no debt change, equivalence not established |
| 305 (ROR) | `inKindFeeDebtSat > 0` -> `!= 0` | equivalent for unsigned integers |
| 307 (ROR) | `repaidSat > 0` -> `>= 0` | may emit a zero-amount repayment event |
| 307 (ROR) | `repaidSat > 0` -> `!= 0` | equivalent for unsigned integers |
| 368 (ROR) | `burnedSat > 0` -> `>= 0` | extra zero-amount token and Bank calls; equivalence not established |
| 368 (ROR) | `burnedSat > 0` -> `!= 0` | equivalent for unsigned integers |
| 309 (AOR) | `inKindFeeDebtSat -= repaidSat` -> `%= repaidSat` | real accounting difference in `sweepFees` under partial repayment; existing 48-test vault suite still passes |

Seven survivors are structurally equivalent. Others need interpretation,
not an automatic "missing test" label. The clearest accounting gap is line
309: full repayment gives zero under both operations, but with debt 10 and
repayment 3, subtraction leaves 7 while modulo leaves 1.

`ReservationRouter.sol` (10 uncaught revert mutants):

| Location | Mutation | Interpretation |
|---|---|---|
| 533 | `reservationByAnchorUtxo` -> `revert()` | not detected by the two-file kill set |
| 548 | `reservedDepositWallet` -> `revert()` | not detected by the two-file kill set |
| 603, 605-610 (7) | `reservationParameters()` tuple assignment -> `revert()` | not detected by the two-file kill set |
| 630 | `reservationRouter` getter -> `revert()` | not detected by the two-file kill set |

The tuple mutation at line 604 was killed. The result does not prove that
the full repository suite lacks getter tests.

### The `Reservation.sol` line-1327 "survivor" - cleared

At `9d208696` the first `Reservation.sol` campaign used a narrow kill command
(`test/bridge/Reservation.test.ts` only). That command did not exercise the
`validateReservationCapsInvariant` require, so the RR revert mutant at line
1327 appeared to "survive" (1 caught / 2). Re-running with the corrected kill
command (`test/bridge/Reservation.test.ts` + `test/bridge/Bridge.ReservationCaps.test.ts`,
which calls `updateReservationCaps` and exercises the
invariant) gives **2/2 revert and 4/4 tweak mutants caught, 0 survived**.
The line-1327 revert mutant is therefore *killed*, not a survivor. No
`Reservation.sol` mutant survives.

### `ReservationProofs.sol` - base `29e84a35c`

At `9d208696` the `ReservationProofs.test.ts` file failed in its `before
each` hook (see section 4), so no proof test could kill any mutant. At
`29e84a35c` the test file is repaired (38 passing baseline), and the same
`parseSingleOutput`-targeted campaign gives **2/2 revert mutants caught,
0 survived, 0 tweak mutants generated**. The proof score is quoted from
`29e84a35c`; the `9d208696` figure (also 2/2) was produced with a
substitute kill command and is noted for completeness but is not the
authoritative proof score.

## 3. What is and is not covered

Covered with verdicts (all verdict-bearing mutants in the authoritative Go
pass 2 and all Solidity verdicts):

- SPV reservation files: 351 killed and 64 survived; 2 timeouts.
- Proposal generation: 229 killed and 38 survived; 2 timeouts.
- Vault: 106/119 killed, 13 survived; seven structurally equivalent,
  one independently reproduced accounting-test gap, and five other
  changes requiring interpretation.
- Reservation library: all six viable sampled mutants killed.
- Proof library at `29e84a35c`: both viable sampled mutants killed.
- Router: 23/33 killed under a two-file kill set.

Not covered / limited:

- The viable bridge-library samples are too small to support a strong
  conclusion: six mutants for Reservation, two for ReservationProofs.
  Slither IR failures prevented most arithmetic, logical and relational
  rewrites. Four arithmetic tweaks did work for Reservation.
- `pkg/tbtc/reservation.go` has 57 unresolved timeouts and 88 uncovered
  mutations. Its two kills do not validate transaction assembly generally.
- The router pilot ran only `Bridge.ReservationStranding.test.ts` and
  `Bridge.ReservationSourceAnchorBinding.test.ts`. Its survivors are
  narrow-kill-set results, not validated full-suite gaps.
- These campaigns predate the new native Go fuzz targets and the new
  reservation Medusa suite. They measure the existing example tests only.

## 4. Pre-existing test-suite bugs (fixed at `29e84a35c`, not here)

At `9d208696`, two reservation test files fail in a setup hook before any
assertion runs, independent of any mutation:

- `test/bridge/ReservationProofs.test.ts` line 156 called `relay.target` on a
  `Mock` handle, but the `Mock` type in `test/helpers/mock.ts` exposes
  `.address`, not `.target`, so the value was `null` and ethers rejected it
  (`unsupported addressable value ... target=null`), failing the whole file in
  `before each`. `29e84a35c` changes the call to `relay.address` (verified in
  the `29e84a35c` worktree).
- `test/bridge/WalletProposalValidator.test.ts` line ~3588 mixed a BigInt
  (from `DEPOSIT_REFUND_SAFETY_MARGIN()`) with a number, producing
  `Cannot mix BigInt and other types`. `29e84a35c` wraps it in `Number(...)`
  (verified).

Both fail identically at the clean `9d208696` baseline in the `resv-merge`
and sibling worktrees, so they are pre-existing test-harness bugs, not
mutation results. `29e84a35c` repairs both. `ReservationProofs.test.ts` and
`WalletProposalValidator.test.ts` were excluded from all `9d208696` kill
commands and `ReservationProofs.test.ts` is included in the `29e84a35c`
proofs kill command.

## 5. Limits

- Did not run a full (non-sampled) Solidity mutation campaign; the
  operator-targeted sample is the permitted scope and the counts reflect it.
- Untested Go mutants remain explicit gaps (60 SPV, 15 proposal-generation,
  88 transaction-assembly mutations); no whole-project mutation score is claimed.
- Did not fix any production code or test code: all changes were test-only
  (kill scripts, worktrees, GOCACHE redirection) plus the slither-mutate
  source rewrites that are reverted between mutants. The two pre-existing
  test bugs in section 4 are reported, not fixed, in this agent's worktrees
  (they are fixed on the base branch at `29e84a35c`).
- Survivor equivalence is partial. A state-preserving extra zero-amount
  operation or an additional event is not automatically a missing
  consumer-behavior test.

### Orchestrator checks

- Temporarily replaced `sweepFees` debt subtraction at line 309 with modulo
  in the isolated mutation worktree. `ReservationVault.test.ts` still
  passed all 48 tests. Restored the subtraction afterward.
- Temporarily changed the acceptance matcher at line 678 from `fee <= 0`
  to `fee < 0`. With `GOCACHE=<gocache> GOMAXPROCS=2`,
  `go test -count=1 -timeout 120s ./pkg/maintainer/spv` passed in 4.673s.
  Restored the original guard afterward.

### Gaps closed after the pilot

| Gap | Test added | Branch and commit | Red check |
|---|---|---|---|
| `sweepFees` `-=` replaced by `%=` | "should reduce the debt by exactly the repaid amount" in `ReservationVault.test.ts` (debt 500,000, about 200,000 repaid) | tbtc-v2 the integrated reservation branch `46b136039` | With `%=` injected in the isolated worktree: 48 passing, 1 failing, "expected 100000 to equal 300000". Real code: 49 passing. |
| Acceptance matcher `fee <= 0` replaced by `fee < 0` | "skips a transaction with a zero fee" in `TestFindReservationAcceptanceTransaction` | keep-core the reservation review branch `70cd22942` | With the mutant injected: the subtest fails at `reservation_proof_loop_test.go:278`. Real code: passes. |
| Acceptance matcher `fee > TxMaxFee` replaced by `>=` | "accepts a fee exactly at TxMaxFee" in the same test | same commit | With the mutant injected: the subtest fails at line 296. Real code: passes. |

Both isolated worktrees were restored and checked clean after each red run.

## 6. Reproduce and raw outputs

Versions: Gremlins reports `version dev`; Slither and slither-mutate are
from slither-analyzer 0.11.6. The raw files retain the observed outcomes.

Go replay commands and the Solidity kill commands are preserved beside
this report in `replay/`. `gremlins-rerun.sh` runs the three package scans
sequentially. Set an on-disk `GOCACHE` before running it.

The vault campaign command is:

```sh
cd tbtc-v2/solidity
slither-mutate contracts/vault/ReservationVault.sol --contract-names ReservationVault \
  --target-functions "receiveBalanceIncrease(address[],uint256[]),financeInKindFee(uint64),repayInKindFeeDebt(uint64),updateFeeReserveTarget(uint256),sweepFees(address),updateInitiationFee(uint16),receiveBalanceApproval(address,uint256,bytes),_burnFromReserve(uint64)" \
  --test-cmd "verification/replay/kill_vault.sh" --timeout 160 --output-dir out/mut-vault \
  --compile-force-framework foundry --mutators-to-run AOR,LOR,ROR,RR -v
```

The kill command runs `test/vault/ReservationVault.test.ts`.
Restore each source file between campaigns. Use isolated worktrees at the
listed commits, not a development worktree.

Raw outputs (also copied into this directory):
- Go: `raw-go/gremlins_spv_c8.json`, `raw-go/gremlins_pg_c8.json`,
  `raw-go/gremlins_tb_c8.json` (authoritative); `raw-go/gremlins_spv2.json`,
  `raw-go/gremlins_pg2.json`, `raw-go/gremlins_tb.json` (pass 1 / c5, kept for
  the timeout comparison); `raw-go/gremlins_tb_c8b.json` (coefficient-16
  tb re-run).
- Solidity: `raw-solidity/mut-vault.log`, `raw-solidity/mut-resv2.log`
  (corrected kill), `raw-solidity/mut-resv.log` (narrow kill, superseded),
  `raw-solidity/mut-proofs.log` (9d208696), `raw-solidity/mut-proofs2.log`
  (29e84a35c, authoritative proof score), `raw-solidity/mut-router.log`.
- Kill scripts: `verification/replay/kill_vault.sh`, `verification/replay/kill_resv.sh`,
  `verification/replay/kill_proofs2.sh`, `verification/replay/kill_router.sh`.
- Clean pre-campaign source backup: `<mutation backup>/`.
