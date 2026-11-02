# PR #4282 - implementation tracker (started 2026-10-05)

The human asked to "continue with those items" on 2026-10-05. That replaces decision D11 ("stop at the
plan") and lifts the audit-only rule for the PR fixes. Source of truth for each fix:
[recommendations-pr-4282.md](./recommendations-pr-4282.md). Status values: todo / in progress / done /
blocked.

State at the start:
- `origin/reservations-epic` = `49c4df13d` (the audited code). It is 195 commits behind `origin/dev`
  (`d72bca6fc`) and has 170 commits that dev does not.

## Decisions (2026-10-05, interactive)
- Fix commits: one commit per package on this PR, with neutral messages and no exploit detail.
- Dev sync: one merge of `origin/dev` into the fix line after Phase A. The merge-tree check found 19
  conflicting files. No rebase, no force-push.
- Tooling: one local branch per item for E1-E5, from `origin/dev`, each in its own worktree. One local
  branch for E6-E8. Nothing pushed.
- Execution: work packages that share files go to one worker. A foundation commit comes first, adding the
  chain-time and wallet-parameter accessors that P3, P8 and P2 share. The orchestrator verifies (build, vet,
  tests, red-on-base check) before landing each package.

## Phase A - PR fixes (36 findings + the PR-new leads the packages name, 12 packages)
| # | Package | IDs | Status | Verified by |
|---|---|---|---|---|
| A1 | P1 Acceptance timeout notice path (High) | G2-F2, G2-F18, G2-F19, G2-L9 | done (`41fb820a5`) | `TestG2F2_...SendsAcceptanceTimeoutNotice` fails when the zero-wallet early return is put back (orchestrator mutation check); spv tests pass |
| A2 | P2 Fixed protocol fee minimum | G1-F6, G1-F7, G1-F8, G4-F10, G4-F11, G1-S3, G1-L26 | done (`a83ba04ef`); G1-L26 not implemented (lead, the plan says "if adopted") | Reject tests on all 5 paths fail on base logic (orchestrator red check); the honest-leader table test passes; `pkg/tbtc` tests pass |
| A3 | P3 Re-anchor custody gate (follower required) | G1-F2, G1-F12, G1-L21, G1-L24, G1-L25, G1-L27 | done: follower in `a83ba04ef`, leader in `5a7b125fd`, G1-L25 re-read included; G1-L21/L24/L27 skipped (no concrete fix in the plan) | `TestReservationReanchorAction_RejectsYoungTarget` fails on base (orchestrator check); 8 tbtcpg tests pass and fail on base logic |
| A4 | P4 RPC deadlines in reservation execute paths | G1-F11 | done (`a83ba04ef`) | `TestReservationExecute_RPCTimeoutReleasesDispatcher`: on base the dispatcher lock is held (reported by the fix-up worker); passes on HEAD |
| A5 | P5 Pprof gating on the metrics port | G5-F1, G5-F2 | superseded at merge: `d8dacd993` dropped pprof (human decision, 2026-10-05), but `origin/dev` already fixed G5-F1 with a private mux (`864d65817`, on top of the keep-common clientinfo move `c7fb09fda`). Human chose dev's version at the merge (2026-10-06). `pkg/clientinfo`, `docs/profiling.md` and `docs/index.adoc` match dev; our pprof test is dropped because dev's `pprof_exposure_test.go` covers flag-off 404 | `TestMetricsServer_DoesNotServePprof` passed on the fix branch and failed on `49c4df13d`; on the merged head dev's `TestClientInfoServerDoesNotExposePprofWhenDisabled` covers it |
| A6 | P6 Ephemeral key shape validation | G4-F8, G4-F9 | done (`c5688f92c`) | `TestEphemeralPublicKeyMessage_RejectsMalformedEphemeralKey` (dkg + signing) passes and fails on base; package tests pass |
| A7 | P7 Eviction and unbounded growth in proof loops | G2-F1, G2-F16, G2-L8, G2-L10 | done (`41fb820a5`); rule decided by the human: evict when chain head > TimeoutAt + 4 x (TimeoutAt - RequestedAt) | inverted retention tests pass; spv tests pass |
| A8 | P8 Chain time vs wall clock | G1-S1, G1-S2, G1-L28, G2-L7, G2-L11, G2-L13 | done: G1 part `7c34c4e12`; G2 leads `a17044076`. The TimedOut late-window eviction, the action-timeout notify gate and the stale-deposit refund gate now use chain head time, and an unreadable head skips the decision | 6 `TestG2L7/L11/L13_*` tests pass and fail with the three production files reverted to `41fb820a5` (orchestrator check); spv `-race` passes |
| A9 | P9 Scan cost in the SPV maintainers | G2-F3, G2-F4, G2-F5, G2-F10, G2-F11 | done (`41fb820a5`, `a29a69690`) | per-tick cap test (50 reads); bounded start-block tests; the F10/F11 helper test fails on base (nil bridge panic) |
| A10 | P10 Error isolation in reservation watchers | G2-F8, G2-F9, G2-F15, G2-F17, G2-F22, G2-L2, G2-L5, G2-L15 | done (`41fb820a5`, `a29a69690`); G2-L2 and G2-L15 skipped (the plan gives no concrete design) | `TestG2F15_...` fails when the `err == nil` guard is removed (orchestrator mutation check); F9 `queried=[1,3]` |
| A11 | P11 Reservation flag wiring | G4-F6, G2-F24, G2-F25 | done. G2-F24/F25: `41fb820a5` (F24 uses the reservation's own `DissolutionEligibleAt` and chain time; no change to WalletChainData). G4-F6: deploy check `bc9bb78fe`, plus client alert `dd6c32ea4` (Warn log and counter `spv_reservation_action_overdue_total` when a Pending action passes its timeoutAt in chain time) | F25 table covers both directions. `TestG4F6_OverdueActionRaisesAlert` and `..._AlertRepeatsOncePerRenotifyInterval` fail when the alert call is removed (orchestrator mutation check) |
| A12 | P12 Periphery fixes | G5-S2, G5-S3, G5-S6 | done (`6cf4251e9`, `cf9c78db2`, `f25747bc7`; gofmt applied) | 3 new tests pass, all fail on base; `-race` on pkg/net/local fails only in the pre-existing `TestUnregisterHandler`/`TestUnregisterWhenHandling` (the same on base) |
| A0 | Foundation: chain head time + wallet params on `BridgeChain`, `IsReanchorTargetAged` | (shared) | done (`d59e38fd4`) | tbtc, tbtcpg, ethereum, maintainer and cmd tests pass; new API, so the base red-check is n/a |

Done for each package means:
- the fix is in place;
- the plan's regression tests are added, and each fails on `49c4df13d` and passes on the fix;
- `go build ./...` and `go vet` pass on the touched packages;
- `go test` passes on the touched packages.

## Phase B - Bring the epic up to date with dev
| # | Task | Status | Verified by |
|---|---|---|---|
| B1 | Integrate `origin/dev` into the fixed epic (method decided with the human) | done: final head of the integrated fix line. Commits: the dev merge (`813e94cab` merges `origin/dev` `d72bca6fc`); a test-alignment commit corrects the merged tests; two merges bring in the audit-fix commits (at `41fb820a5`, then `dd6c32ea4`) | |
| B2 | Resolve conflicts; full `go build ./...`, `go test ./...` (short), Solidity unaffected check | done: 24 conflicted files. On the second merge, `reservation_regression_test.go` was moved from keep-common's chain package to the in-repo `pkg/chain/ethereumutil` | Final head of the integrated fix line, with `GOTOOLCHAIN=go1.25.7 GOFLAGS=-mod=readonly`: build passes; `go test -count=1 ./...` has 0 failures; vet shows only the known `tssResult` copylocks warning; gofmt clean. `-race` on maintainer/..., tbtc, tbtcpg/... and clientinfo: spv, tbtc and tbtcpg pass. btcdiff (`TestWaitForCurrentEpochUpdate_Successful`, `TestBitcoinDifficultyMaintainer_Integration`) and clientinfo (`TestObserve`) fail with 26 races in test fixtures. `origin/dev` `d72bca6fc` shows the same 3 failures and 26 races, and the PR does not touch those files, so they predate this work. `solidity/` is byte-identical to `origin/dev`, and the fix commits touch no Solidity |
| B3 | Re-run the Phase A regression tests on the integrated head | done | Every Phase A regression test is a permanent test inside the full `./...` run above (0 failures). Separate runs: tecdsa ephemeral tests `-count=20`, plus tbtc, tbtcpg, clientinfo, maintainer, ethereum and cmd |

## Phase C - Full audit re-run on the integrated head (decision D7)
| # | Task | Status | Verified by |
|---|---|---|---|
| C1 | x-ray on the merged head: Go PR scope (now `git diff origin/dev <head>`, 73 non-test Go files) plus the ecdsa and random-beacon suites (D7: all bundles) | done. ecdsa and beacon on a frozen merge worktree (Solidity equals dev): both FRAGILE (unchanged). Go on the merged head: EXPOSED (unchanged, capped by tests); old I-11 (wall clock vs chain time) now holds on every reservation path. Two Enforced=No invariants remain in pre-existing code, unchanged since merge-base `cbd6e4d2b`: I-18 `pkg/tbtc/moving_funds.go:479` (safety margin on the host clock) and I-19 `pkg/tbtcpg/deposit_sweep.go:223,288` (deposit min age on the host clock) | the Phase C x-ray delta for the Go scope is committed at [rerun/xray-go.md](./rerun/xray-go.md) and [rerun/x-ray-go/](./rerun/x-ray-go/) |
| C2 | solidity-auditor 5 passes x 12 agents with the first-audit ledgers (KNOWN/NEW diff) on every bundle: S1, S2, G1-G5 (D7). Go bundles = old file list plus newly assigned files | done: all 7 scans ran 5 x 12/12 (resumed after the 2026-10-06 restart; G3 restarted under a new stamp). The scan outputs are an internal run record and are not part of this record | full-report.md present for each scan; per-scan triage in `rerun/` |
| C3 | Triage new items against the findings contract; diff against the full findings list of the first audit | done. See [rerun/README.md](./rerun/README.md). VerifySolReportOnly traced the 8 report-only S1/S2 verdicts; PR-origin items fixed on the fix line: the bare-floor int64 overflow, the stale-deposit cursor/head-read/budget trio, the parked-RPC slot bound, the chain-time reveal lookup, the saturating gas margin and the integer gas margin at the 8 reservation call sites (commits `95a3a7c64`, `0b0c89cb1`, `fc9cf18d5`, `f9235d320`, `d92646e8f`, `106322562`, `2a8134a8f`). The worker's G1-R19/R22/R23 changes were reverted (`4a9ad6bfb`, `ab19c5070`). Items against pre-existing code are routed outside this record and are not listed here | every fix test fails without its fix (orchestrator scratch-worktree checks); full suite on the integrated fix head passes except one load-timing flake in dev's ethutil that passes alone |
| C4 | Update the findings report, docs copy, commit to the audit branch | done: committed locally on the audit branch | no audit branch was pushed to the public origin |

## Log
- 2026-10-05: tracker created.
- 2026-10-05: wave 1 verified and landed (A0, A5, A6, A12). Wave 2 running: the tbtc worker (P2, P3
  follower, P4, G4-F6) and the tbtcpg worker (P3 leader, P8 G1 part). The SPV group (P1, P7, P9, P10,
  G2-F24/F25) is still running. The P8 G2 leads (L7/L11/L13) follow after SPV lands.
- 2026-10-05: wave 2 landed after corrective workers. Both wave-2 branches failed to compile or to commit on
  first delivery. The fix line is at `813e94cab`: build passes, vet passes, gofmt is clean, and tests pass in
  tbtc, tbtcpg, cmd, maintainer, clientinfo, ethereum, net/local and beacon/dkg.
- 2026-10-05: the SPV group landed as `a29a69690` and `41fb820a5`. The add/add conflict on
  `tbtc_chain_time.go` was resolved to the foundation version. Build, vet and gofmt are clean, and tests pass
  in maintainer, ethereum, cmd, tbtc and tbtcpg. Dev merge running from `813e94cab`; the SPV commits are
  merged in after it. Remaining: the P8 G2 leads and the G4-F6 alert.
- 2026-10-06: dev merge committed (24 conflicted files). pprof went to dev's private-mux
  version (human decision). The orchestrator then corrected two worker choices: the tecdsa ephemeral-key fuzz
  round-trip now generates valid keys, so dev's error check stays (the worker had dropped it); and the
  honest-leader model in `proposal_fee_check_test.go` and the reject-gate comment now follow dev's
  floor-then-buffer order (honest minimum 7 sat/vB, still above the protocol minimum of 5). Dev's fee-order
  change moved anchor fixtures from 710 to 994 and re-anchor fixtures from 550 to 770.
- 2026-10-06: `a17044076` (chain-time gates) and `dd6c32ea4` (overdue alert) landed. Both were red-checked by
  the orchestrator and fast-forwarded onto the fix line, then merged into the integrated head as
  that merged head. Phase C started on frozen detached worktrees, each seeded with the first-audit ledgers.
- 2026-10-08: Phase C done. All 7 scans finished 5 x 12/12 (resumed after the 2026-10-06 restart). The
  orchestrator second-checked every Medium-or-higher item and every PR-code claim, re-classified origins by
  function-body hash, and landed the PR-origin fixes listed in C3. Corrections to worker output: the
  label-keyed RPC in-flight registry blocked unrelated wallets (raised by an advisory; replaced by a global
  slot bound), two fix tests passed without their fixes (strengthened), and the G1-R19 tolerance change
  contradicted D9 (reverted). The head of the integrated fix line carries the result.