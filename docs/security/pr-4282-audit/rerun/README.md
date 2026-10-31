# PR #4282 Phase C re-audit (merged head, 2026-10-06 to 2026-10-08)

## What was audited
- **Go:** the merged fix head. This is the epic `49c4df13d` plus all Phase A fixes, merged with keep-core
  `origin/dev` `d72bca6fc`.
  - The PR diff is now `git diff origin/dev <head>`: 73 non-test Go files.
- **Solidity:** the frozen merge worktree. Its `solidity/` is byte-identical to `origin/dev`; the PR changes no Solidity.
  - Dev changed 80 contract files since the first audit, including the vendored sortition contracts.
  - Those suites' results concern pre-existing code and are not part of this record.
- **Methods (decision D7):**
  - x-ray on the Go PR scope and on both Solidity suites.
  - solidity-auditor, 5 passes x 12 agents with memory on, seeded with the first-audit ledgers so every item
    is tagged KNOWN or NEW, on S1, S2 and G1-G5.
  - Go bundles: the first-audit file lists plus newly assigned files (the scope lists are internal run
    records and are not part of this record).
- **Raw outputs:** the scan runs, reports and ledgers are an internal run record and are not part of this
  record.

## Scan record
| Scan | Stamp | Agents per pass | Ledger rows before -> after |
|---|---|---|---|
| S1 ecdsa | 20261006-111610 | 12/12 x 5 | 44 -> 117 |
| S2 random-beacon | 20261006-111358 | 12/12 x 5 | 61 -> 134 |
| G1 go-signing | 20261006-144426 | 12/12 x 5 | 44 -> 82 |
| G2 go-spv-chain | 20261006-144147 | 12/12 x 5 | 40 -> 74 |
| G3 go-chain-legacy | 20261007-061322 | 12/12 x 5 | 70 -> 169 |
| G4 go-node-protocol | 20261006-144615 | 12/12 x 5 | 89 -> 948 |
| G5 go-periphery | 20261006-144851 | 12/12 x 5 | 17 -> 143 |

A session restart (2026-10-06 ~21:15 UTC) killed every scan worker after 0-3 passes. The scans were resumed
from disk; the run headers and `scope.tsv` name the agents that were re-run. G3 had no usable state, so it
restarted under a new stamp; the empty stamp `20261006-144140` is kept as a record.

## x-ray
- Go: EXPOSED (unchanged; capped by test depth).
  - The first audit's only Enforced=No invariant, I-11 (watchers compare wall clock to chain time), now holds
    on every reservation path.
  - Two Enforced=No invariants remain, both in pre-existing code, so they route outside this record:
    - I-18: moving-funds safety margin on the host clock.
    - I-19: deposit-sweep minimum age on the host clock.
- The Solidity x-rays (both FRAGILE, unchanged) describe the pre-existing contract suites and are not part
  of this record.
- The Go x-ray report and its delta are committed at [xray-go.md](./xray-go.md) and
  [x-ray-go/](./x-ray-go/).

## Prior findings on the merged head
| Scan | Fixed | Still present | Changed | Notes |
|---|---|---|---|---|
| S1 | - | - | - | Pre-existing contract suite; reported privately, not part of this record. |
| S2 | - | - | - | Pre-existing contract suite; reported privately, not part of this record. |
| G1 | 8 | 5 | 1 | G1-F11 changed: P4 covers the reservation paths only. |
| G2 | 17 | 7 | 1 | |
| G3 | - | - | - | Pre-existing Go paths; reported privately, not part of this record. |
| G4 | 2 (with residual) | 9 | 3 | |
| G5 | 4 | 6 | 1 | G5-F1 (pprof) fixed by dev's private mux `864d65817` (the human chose dev's fix at the merge). |

## New items and where they went
- **Second checks:** the orchestrator re-checked every Medium-or-higher item and every item claimed as PR
  code. See [orchestrator-overrides.md](./orchestrator-overrides.md), [origin-G1.md](./origin-G1.md),
  [pr-fixes.md](./pr-fixes.md) and [pr-fixes-2.md](./pr-fixes-2.md).
  - The scan Origin labels were often wrong in both directions. Final origin comes from function-body hashes
    at `a7ac8989`, the epic, `origin/dev` and the head.
- **Fixed in the PR**, on the audit-fix line merged into the integrated fix line. Each fix has a test
  that fails without it, checked in scratch worktrees.

  | Item | Fix |
  |---|---|
  | G1-R6 | bare-floor int64 overflow (`95a3a7c64`) |
  | G2-R1, G2-R2, G2-R6 | stale-deposit cursor, one head read per tick, budget judged on chain time (`0b0c89cb1`) |
  | G1-R11 | parked RPC goroutines bounded by a global slot pool that does not block independent queries (`106322562`) |
  | G1-R29 | reveal lookup centered on chain time (`d92646e8f`, test `2a8134a8f`) |
  | gas margin | saturating margin helper (`fc9cf18d5`) and the integer margin at the 8 reservation call sites (`f9235d320`) |

- **Disputed or false positive PR-code items (no change), with reasons in the files above:**
  - G2-R3 (Electrum history includes spends).
  - G2-R7 (`dissolutionEligibleAt` is never 0 for an Active reservation).
  - G4-R1 (unreachable estimator errors).
  - G4-R8, G4-R43 (unmarshal gates).
  - G4-R14 (maintainer metrics).
  - G1-R19 (contradicts D9; the worker's change was reverted).
  - G1-R22, G1-R23 (unreachable; reverted).
  - G1-R17/R18/R20/R21/R28, G2-R4/R5, G4-R23/R41/R42/R44/R45/R47/R51, and the periphery items listed in
    [pr-fixes-2.md](./pr-fixes-2.md).
- **Out of scope for keep-core:** G1-R7. A third party can hold a Live target's re-anchor capacity until the
  action times out. The on-chain `requestReservationReanchor` decides this (tbtc-v2 note).
- **Pre-existing and dev items:** every item whose final origin is pre-existing code routes outside this
  record; none of them is listed here.

## Verification of the final head
- `GOTOOLCHAIN=go1.25.7 GOFLAGS=-mod=readonly`: `go build ./...` passes.
- `go vet ./...` shows only the known `tssResult` copylocks warning.
- `go test -count=1 ./...`: one failure under full-suite load, `TestRateLimiter_RequestsPerSecondLimitOnly`
  (dev's ethutil package, identical to `origin/dev`). It passed when re-run alone.
- `-race` on maintainer/..., tbtc, tbtcpg/... and clientinfo (run at the merged fix head): the only failures are the
  btcdiff and clientinfo fixture races, which `origin/dev` shows identically (26 races, 3 tests). E3's
  `RACE_EXCLUDE` already lists both packages.