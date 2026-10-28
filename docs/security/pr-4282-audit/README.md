# PR #4282 (UTXO reservations epic) audit - findings index

Repo: `threshold-network/keep-core`, branch `reservations-epic` (PR #4282). The audit describes the epic
head `49c4df13d`; the re-audit in `rerun/` describes the epic with the audit fixes applied and merged with
`dev` (the merged fix head; its fixes are in the published head `f03cc61dc`). At audit time the PR was 128 commits ahead of and 187 behind `dev` and conflicting,
so every result here describes the epic branch, not a merged result. Merge-base with `dev`: `a7ac8989`;
the PR diff is `git diff a7ac8989 49c4df13d`. The PR changes no Solidity contract logic; its real change
is the Go UTXO-reservation client.

## Scope of this record

This record covers the UTXO-reservation feature added by PR #4282. It lists every Confirmed, Plausible,
Disputed and False-positive item the audit produced on reservation code, plus the executed evidence, the
verification runs and the fix plan that the PR commits implement.

The audit also reviewed code that predates this PR. Those results describe issues in code running on
mainnet today; they are reported privately to the maintainers and are deliberately not part of this
record. Counts, tables and cross-references here reflect the reservation subset only, so the report is not
a completeness statement about the repository.

Speculative leads and the per-source lead tables are likewise not published, with one exception: the
leads that the fix plan references are listed in the appendix so that the fix tracker's references
resolve.

The reservation operations material from the same audit - the M1 guide, the testnet drill runbook, the
Sepolia handoff and end-to-end plan, the keeper drill and alert acceptance notes, the lab scenarios and the
verification inventory - lives under `docs/plans/reservations/`, next to the public epic tracker.

State of record: all finding IDs are uniform (`G2-F2`, `G1-S3`, …) and every changed file carries an
`## ID mapping` section for old-to-new IDs. Executed evidence lives in `evidence/` next to this file; the
scan and x-ray reports cited by the triage files are internal run records and are not part of this
record.

Update 2026-10-08: the fixes are implemented and merged with `dev` (Phase A and B), and the full audit was
re-run on that head (Phase C). The status of every finding below on the merged head, the new items from
the re-run, and the PR fixes that came out of it are in [rerun/README.md](./rerun/README.md). The sections
below still describe the epic `49c4df13d`.

## 1. Scope and methods

Audited surface:

- Go PR diff split into five bundles: G1 go-signing (`pkg/tbtcpg` reservation acceptance/reanchor/fee/deposit-sweep
  plus the `pkg/tbtc` reservation, proposal-fee, deposit-reveal-lookup, marshaling, coordination, and
  node-proposals files), G2 go-spv-chain (the reservation SPV watchers and `pkg/chain/ethereum/tbtc.go`),
  G3 go-chain-legacy (chain-adapter split-outs, `tbtc/chain.go`, `tbtcpg/chain.go`, legacy SPV proof and
  proposal paths), G4 go-node-protocol (tbtc node/wallet/coordination, teccsa DKG and signing protocol,
  cmd, beacon node, libp2p), G5 go-periphery (clientinfo, flags, checkabi, ephemeral keys, btcdiff, net
  channels, marshaling).
- Manual review: 14 small-diff files (1-6 changed lines in big files), 21 generated abigen bindings, 3
  test-helper files.
- On-chain reference: tbtc-v2 @ `eec999aad` (pinned copy, the Go-side guard contracts `Bridge`,
  `Reservation`, `ReservationProofs`, `WalletProposalValidator`, `MovingFunds`, `Redemption`,
  `DepositSweep`, `LightRelay`).
- The audit also covered the `solidity/ecdsa` and `solidity/random-beacon` contract suites; those results
  concern pre-existing code and are not part of this record.

Methods, in order:

- x-ray pre-audit of the 90-file Go PR scope (verdict EXPOSED; 75 G / 17 I / 8 X / 4 E invariants; the
  only Enforced=No invariant is I-11, watchers comparing wall clock to chain time). The x-ray reports for
  the Go scope are committed in this tree under [rerun/x-ray-go/](./rerun/x-ray-go/).
- solidity-auditor, 5 passes x 12 agents per scan root, confidence threshold 75, memory ledger on. For the
  Go bundles this is an adaptation, not literal skill behavior: the skill is Solidity-only; the Go runs
  used filename mode over the bundle lists, with the pinned on-chain contracts consulted as guard
  references. The adaptation deviations are recorded in the triage files themselves.
- Supplementary single-agent runs for lost scan agents, merged into the triage files as `S` rows: G1
  first-principles (pass-3 agent 7 died) into `go-signing.md` as G1-S1/S2 (2 new leads); G3 flow-gap
  (agent 12) plus periphery (agent 6); G5 periphery, execution-trace, and numerical-gap (pass-3 agent 6
  empty, pass-4 agents 4 and 10 placeholder) into `go-periphery.md` as supplement S rows (three of them
  are in this record).
- Manual triage of every finding and lead against the findings contract (verdicts: Confirmed / Plausible /
  Disputed / False positive / Duplicate), one triage file per source.
- Executed second checks: Go tests in detached worktrees (G4), scratch Go programs and foundry-style
  reproductions (G1, G2, G5), and bytecode/line inspection where execution was not possible. Executed
  evidence was saved in this pass: `Evidence check:` lines were added to the triage files, and the scratch
  tests, tables and response captures are kept under `evidence/` (`g1/`, `g2/`, `g4/`, `g5/`,
  `reservation-keeper-alerts/`).

Not covered: reading live mainnet chain state beyond targeted bytecode fetches; Byzantine simulation of
the Go protocol; mutation testing; formal verification; the tbtc-v2 on-chain half as an audit target
(used only as guard reference); the merged-PR result at the time of the first pass (the epic branch
diverges 187 commits from `dev` and conflicts - the Phase C re-run covers the merged head).

## 2. Totals

Verdict counts below are read from each file's summary table (tables counted row by row; where a file's
prose count and its table disagree, the table wins - noted in the caveats in section 7). Severity counts
apply to Confirmed + Plausible only. `S` rows (supplement triage) are included.

| Source (file) | Findings | Conf | Plaus | Disp | False pos | Dup | C/P High | C/P Med |
|---|---|---|---|---|---|---|---|---|
| G1 go-signing ([go-signing.md](./go-signing.md)) | 13 | 4 | 5 | 3 | 0 | 1 | 0 | 6 |
| G2 go-spv-chain ([go-spv-chain.md](./go-spv-chain.md)) | 24 | 9 | 8 | 2 | 3 | 2 | 1 | 1 |
| G4 go-node-protocol ([go-node-protocol.md](./go-node-protocol.md)) | 8 | 5 | 0 | 2 | 1 | 0 | 0 | 2 |
| G5 go-periphery ([go-periphery.md](./go-periphery.md)) | 5 | 4 | 1 | 0 | 0 | 0 | 0 | 1 |
| Small diffs + bindings ([go-small-diffs-and-bindings.md](./go-small-diffs-and-bindings.md)) | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| **Total** | **50** | **22** | **14** | **7** | **4** | **3** | **1** | **10** |

- Totals row: findings 13+24+8+5+0 = 50; verdict columns sum to the findings column (22 Conf + 14 Plaus +
  7 Disp + 4 False pos + 3 Dup = 50).
- G1 = 11 main F rows + 3 supplement S rows (G1-S1/S2 Plausible Low, G1-S3 Confirmed Low); G5 = 3 main
  findings (F1-F3 rows, of which F1 and F2 are in this record) + 8 supplement S rows (of which S2, S3 and
  S6 are in this record).
- The small-diffs file records 2 Informational notes (section 3.1 and 3.2, both "not a finding" closures);
  it has no findings and no leads, so its row is all zeros.
- Findings against pre-existing code outside this PR's diff are tracked privately and are not listed
  here. That is why the per-file finding counts in this table are lower than the number of items the same
  triage files discuss.

## 3. Ranked High / Medium Confirmed and Plausible

Every Confirmed or Plausible High or Medium item in this record, ranked: High first, then Medium. All of
them are PR-introduced. "Evidence" is `executed` when a named test/repro ran for it, else `code reading`.

| # | ID | Sev | Verdict | Impact (one line) | Trigger | Evidence | Section |
|---|---|---|---|---|---|---|---|
| 1 | G2-F2 | High | Confirmed | Zero-wallet check skips every first-generation acceptance-timeout notice; the standard operator notice path is fully dead for fresh reservations | No actor needed; always fires; griefing by permissionless late notification only | executed (scratch test `TestG2F2_ZeroWalletPendingAcceptance_SkipsAcceptanceTimeoutNotice`: 0 notices vs 1 control) | [G2](./go-spv-chain.md#g2-f2---zero-wallet-check-blocks-the-acceptance-timeout-notice-high-confirmed) |
| 2 | G1-F6 | Med | Confirmed | Below-relay sweep fee is warned, not rejected: a malicious leader jams the wallet's deposit sweep every window; on-chain validator enforces no minimum relay floor | Malicious coordination leader | code reading (second check held) | [G1](./go-signing.md#g1-f6-below-relay-sweep-fee-is-co-signed-by-every-follower) |
| 3 | G1-F7 | Med | Confirmed | Same root cause on the anchor-fee path: below-relay anchor fee co-signed by every follower; acceptance stalls until expiry | Malicious coordination leader | code reading | [G1](./go-signing.md#g1-f7-below-relay-anchor-fee-is-co-signed-by-every-follower) |
| 4 | G1-F8 | Med | Confirmed | Same root cause on the re-anchor path: below-relay re-anchor fee jams a MovingFunds wallet's recovery path indefinitely under a persistent leader | Malicious coordination leader | code reading | [G1](./go-signing.md#g1-f8-below-relay-re-anchor-fee-is-co-signed-by-every-follower) |
| 5 | G1-F2 | Med | Plausible | Re-anchor resume co-signs to the on-chain target with no custody gate: a permissionless request can direct a retiring wallet's anchors to an attacker's Live wallet | Any EOA + operator resume task | code reading | [G1](./go-signing.md#g1-f2-re-anchor-resume-signs-to-an-attacker-chosen-target) |
| 6 | G1-F12 | Med | Plausible | `reanchorTargetSearch.pick` returns the first Live wallet with capacity, no trust gate; a freshly registered attacker wallet can be chosen as the re-anchor target | Any attacker registering a Live wallet | code reading | [G1](./go-signing.md#g1-f12-re-anchor-target-search-picks-an-untrusted-wallet) |
| 7 | G1-F11 | Med | Plausible | Unbounded validation RPC calls in the reservation execute paths hold the per-wallet dispatcher lock: a hung Ethereum RPC endpoint freezes every signing action for that wallet | Malicious or negligent RPC provider | code reading | [G1](./go-signing.md#g1-f11-unbounded-validation-rpc-call-locks-a-wallets-dispatcher-forever) |
| 8 | G2-F1 | Med | Confirmed | Abandoned (TimedOut) re-anchor generations are never evicted; a permissionless requester grows per-pass RPC volume without bound on a target wallet | Any EOA cycling request -> timeout | executed (existing tests `TestProveReservationReanchorActions_TimedOutSourceAnchor` / `..._TimedOutUnbounded`) | [G2](./go-spv-chain.md#g2-f1---abandoned-reanchor-generations-never-get-evicted-medium-confirmed) |
| 9 | G4-F8 | Med | Confirmed | Deferred ephemeral-key parse: one corrupt targeted entry splits the DKG group's active view; a round that would complete fails | One operator inside the DKG group | executed (`go test -run TestSecondCheck_F8 ./pkg/tecdsa/dkg/...`) | [G4](./go-node-protocol.md#g4-f8-a-corrupt-ephemeral-key-splits-the-dkg-groups-active-view-confirmed-medium) |
| 10 | G4-F9 | Med | Confirmed | Identical mechanism in the signing protocol: a live wallet's signing round splits into different operating sets, stalling one attempt | One operator inside the signing group | executed (`go test -run TestSecondCheck_F9 ./pkg/tecdsa/signing/...`) | [G4](./go-node-protocol.md#g4-f9-a-corrupt-ephemeral-key-splits-the-signing-groups-active-view-confirmed-medium) |
| 11 | G5-F1 | Med | Confirmed | Named pprof import registers `/debug/pprof/*` on the all-interfaces metrics port regardless of `EnablePprof`; cmdline, heap and goroutine dumps reachable by anyone on the network; recalibrated High -> Medium on execution (see corrections log) | Anyone reaching the operator's 9601 (scrapers, co-tenants) | executed (`TestPprofSecondCheck`, 12 endpoints 200, no secret bytes in any body) | [G5](./go-periphery.md#g5-f1-enablepproffalse-still-exposes-debugpprof-on-the-metrics-port) |

All other Confirmed/Plausible items in this record are Low and detailed in their source files.

## 4. Findings introduced by PR #4282

The PR under review is the reservation client; these are its own defects (full detail in the linked
sections):

- G2-F2 (High, Confirmed): the reservation action-timeout watcher skips first-generation acceptances while
  `reservation.walletPubKeyHash` is zero, so the standard operator path never submits the permissionless
  `NotifyReservationAcceptanceTimedOut`. [go-spv-chain.md](./go-spv-chain.md#g2-f2---zero-wallet-check-blocks-the-acceptance-timeout-notice-high-confirmed)
- G5-F1 (Medium, Confirmed, recalibrated from High): the PR adds a named `net/http/pprof` import that
  registers the profiling endpoints on the all-interfaces metrics port unconditionally; `EnablePprof=false`
  changes nothing, and `EnablePprof=true` panics startup (G5-F2, Low, Confirmed). The PR's
  `docs/profiling.md` claim that the flag gates pprof is false; the base tree had no pprof import of any
  kind. [go-periphery.md](./go-periphery.md#g5-f1-enablepproffalse-still-exposes-debugpprof-on-the-metrics-port)
- G1-F6 / G1-F7 / G1-F8 and G4-F10 / G4-F11 (Medium/Low, Confirmed): the five wallet-tx fee paths (deposit
  sweep, anchor, re-anchor, moving funds, redemption) only warn on a below-relay fee and co-sign; a
  persistent malicious leader jams each path. All five are fixed in the PR by a hard reject below the fixed
  protocol minimum: the follower errors when the proposed fee is below
  `ProtocolMinWalletTxSatPerVByteFee (5) x (follower vsize - 1)`; a warning stays only for fees below the
  operator's own bare floor. Mixed-version safe, because pre-PR and post-PR leaders both emit at least
  5 sat/vB. The remaining on-chain relay floor is a separate structural item, tracked privately.
  [go-signing.md](./go-signing.md#g1-f6-below-relay-sweep-fee-is-co-signed-by-every-follower)
- G1-F2 / G1-F12 (Medium, Plausible): no custody gate on the re-anchor target, in the resume path (F2) and
  the target search (F12); a permissionless on-chain request can direct a retiring wallet's anchors to an
  attacker's Live wallet. [go-signing.md](./go-signing.md#g1-f2-re-anchor-resume-signs-to-an-attacker-chosen-target)
- G1-F11 (Medium, Plausible): reservation execute paths call Ethereum view bindings and Bitcoin RPC with no
  context deadline while holding the per-wallet dispatcher lock. [go-signing.md](./go-signing.md#g1-f11-unbounded-validation-rpc-call-locks-a-wallets-dispatcher-forever)
- G1-S1 / G1-S2 (Low, Plausible, supplement): the re-anchor and acceptance tasks gate their timeout checks
  on node wall clock, not on-chain timestamps (the x-ray I-11 class, extending it beyond the G2 watcher
  files). [go-signing.md](./go-signing.md#new-items)
- G1-S3 (Low, Confirmed): the follower's fee-floor warning compares the proposed fee against the operator's
  buffered floor while the leader emits the bare floor, so the warning fires on every honest quiet-mempool
  proposal (executed table test: `evidence/g1/fee_floor_leader_vs_follower.out`). Fixed in P2.
  [go-signing.md](./go-signing.md#new-items)
- G2-F1 (Medium, Confirmed): abandoned TimedOut re-anchor generations are retained forever; permissionless
  request -> timeout cycles grow per-pass RPC load. [go-spv-chain.md](./go-spv-chain.md#g2-f1---abandoned-reanchor-generations-never-get-evicted-medium-confirmed)
- G4-F8 / G4-F9 (Medium, Confirmed): the PR's deferred-parse change to the ephemeral-key map means one
  corrupt targeted entry is detected only at use time, splitting the group's active view.
  [go-node-protocol.md](./go-node-protocol.md#g4-f8-a-corrupt-ephemeral-key-splits-the-dkg-groups-active-view-confirmed-medium)
- G4-F6 (Low, Confirmed): a `Tbtc.ReservationsEnabled` without the paired maintainer flag warns and
  continues, stranding anchors with no proof loop; `WireReservationWatchers` returns nil for the
  quiet-network shape (evidence test in `evidence/g4/`). [go-node-protocol.md](./go-node-protocol.md#g4-f6-reservation-flag-asymmetry-strands-anchors-with-no-on-chain-proof-confirmed-low)
- G4-F10 / G4-F11 (Low, Confirmed): the moving-funds and redemption fee paths have the same warn-only
  behavior as G1-F6/F7/F8 and are fixed with them in P2. [go-node-protocol.md](./go-node-protocol.md#g4-f10-a-follower-only-warns-on-a-below-floor-moving-funds-fee-and-signs-anyway-confirmed-low)
- G5-S2 / G5-S3 / G5-S6 (Low, supplement): a local-broadcast retransmit can cross into the next session's
  channels after release; the 1.2x gas margin truncates to 0% for estimates of 4 units or less; a
  deserialized signer private-key share has no curve-order bound before signature use.
  [go-periphery.md](./go-periphery.md#new-items)

Disputed PR-introduced items (noted, not ranked): G1-F1 (vault rotation liveness stall; on-chain guard
stops the fund move), G1-F4 (follower anchor path does not re-check confirmations; SPV proof settlement is
the backstop), G1-F5 / G1-F9 (assembler dust-floor hardening; request-time invariants make it
unreachable), G4-F7 (failed watcher wiring is unrecoverable, but the fatal path is unreachable), G4-F22
(double `setPerformanceMetrics` call is unreachable; the real defect is the L50/L51 race).

## 5. Cross-method agreement

- x-ray's I-11 (the only Enforced=No invariant) is the shared root cause of the G2 clock-skew leads and of
  the G1 supplement rows G1-S1/S2: operators comparing wall clock to a `block.timestamp`-gated on-chain
  deadline. [go-spv-chain.md](./go-spv-chain.md#note-on-severity-framing) [go-signing.md](./go-signing.md#new-items)

## 6. Corrections log

Claims reversed or downgraded during verification, and why:

- G5-F1 (High -> Medium recalibration): the executed second check (`evidence/g5/pprof_secondcheck_test.go`,
  `TestPprofSecondCheck`, with the 12-endpoint response capture under `evidence/g5/pprof-dump/`) shows all
  12 pprof endpoints exposed with the flag off, but the heap profile holds allocation stacks, not memory
  contents; no secret bytes (ephemeral key, TSS key share, symmetric session key) leaked in any executed
  response. The metrics port is not EXPOSEd in the Dockerfile and is ClusterIP-only in the k8s deployment,
  so the reachable-attacker set is co-tenants of the node's network, not the internet. Recalibrated
  High -> Medium; the exposure finding itself stands.
  [go-periphery.md](./go-periphery.md#g5-f1-enablepproffalse-still-exposes-debugpprof-on-the-metrics-port)
- G5 pass record (12/12 -> 11/12 and 10/12): pass 3's agent 6 (periphery) returned an empty result and
  pass 4's agents 4/10 returned placeholder text; the run headers were corrected and the supplement
  recovered the three specialties (8 new G5-S rows).
  [go-periphery.md](./go-periphery.md#scan-deviations-stated-in-the-report-so-they-are-on-record)
- G4-F13 (fee-policy var race): false positive - the write happens-before the goroutine spawn edge; no
  second writer in production. [go-node-protocol.md](./go-node-protocol.md#g4-f13-package-level-fee-policy-variables-race-between-startup-write-and-reads-false-positive)
- G4-F22 (double `setPerformanceMetrics` call): disputed - exactly one production call site; the claimed
  leak is not established.
- G2-F13 / G2-F14 (zero-fee matchers): false positives - the Bridge's proposal validators require
  `fee > 0`, so no zero-fee anchor or re-anchor is ever signed; the matcher's rejection is correct.
- G2-F12 / G2-F21 (Go guards): disputed - an on-chain `!isReservedDeposit` guard and the vault-change gate
  at `Reservation.sol:1313` stop the claimed exploits.
- G1-F1 / G1-F4 / G1-F5: disputed - the on-chain `!isReservedDeposit` guard, the SPV proof settlement
  backstop, and the request-time dust-floor invariant each stop the fund move; the residual is liveness or
  hardening.
- P2 fee-floor design (2026-10-05): the earlier plan rejected a proposal whose fee sat below the operator's
  buffered floor. An executed table test (`evidence/g1/fee_floor_leader_vs_follower.out`) showed that an
  honest leader emits the bare floor (5 sat/vB) for estimates of 4 sat/vB or less, while the buffered floor
  at the defaults (floor 5, buffer 25) is 7 sat/vB, so that reject would have refused every honest
  quiet-mempool proposal. The earlier reasoning that "the floor is a constant" was also wrong: the floor
  and buffer are per-operator flags, so per-operator values could split a signing group. Replaced by the
  fixed protocol minimum (`ProtocolMinWalletTxSatPerVByteFee = 5`); the activation switch was dropped.
  [go-signing.md](./go-signing.md#new-items)
- Repo hygiene incident (root cause of the two `build/build.go` deletions): on 2026-10-02 subagent shells
  sitting at the repo root ran relative `rm -rf ... build` commands, deleting `build/build.go` twice; it
  was restored both times. `build/` now contains `build.go` only. No audit output depends on that
  directory.

## 7. Coverage gaps and caveats

- Code-reading-only evidence still applies to: all G1 findings except the executed G1-L28 gate expression
  (static Go plus on-chain validator reading), most G2 Low rows outside the executed `Evidence check:`
  lines named in the file, and G4-F22.
- Lost or degraded agents per scan: G1 pass 3 ran 11/12 (first-principles agent died; supplement G1-S1/S2
  added). G3's two lost agents were recovered in its supplement. G5's supplement covered periphery,
  execution-trace, and numerical-gap (8 new G5-S rows; record counts corrected to 11/12 and 10/12). G2's
  sub-orchestrator stopped at its 200-request budget after pass 1 and was resumed. G4 ran 12/12 per pass.
- Branch vs dev divergence: the epic is 128 ahead / 187 behind `dev` and conflicting; results describe the
  epic branch. None of the fix commits for the pre-existing items is on this branch.
- The two Solidity x-ray verdicts (FRAGILE) were re-tiered from incorrect initial tiers during the audit;
  they describe the pre-existing contract suites and are not part of this record.
- The small-diffs review re-ran the full abigen pipeline in a detached worktree: 18 of 21 generated files
  regenerate byte-identically; the 3 Bridge files differ only by upstream artifact drift. No hidden manual
  edits found.
- The G5 pprof evidence keeps the endpoint status capture and the in-package test source. The raw pprof
  response bodies are not published, because each one embeds the build path of the profiling host; the
  12-endpoint status capture and the secret-bytes scan are what the finding rests on.

## 8. Fixes, ordered by value

1. G2-F2 (High, PR-introduced): make the acceptance-timeout notice path work for first-generation
   reservations - the permissionless notice exists but no standard operator ever submits it; add a
   non-zero-wallet-capable sender or drop the zero-wallet skip. PR-blocking.
2. G5-F1 + G5-F2 (Medium + Low, PR-introduced): delete the `net/http/pprof` import and mount pprof on a
   private mux gated by `EnablePprof`; fix `docs/profiling.md`. PR-blocking - the flag is inert off and
   fatal on.
3. G1-F7/F8, G1-F6, G4-F10/F11 (Medium/Low, PR-introduced): add the fixed protocol minimum
   `ProtocolMinWalletTxSatPerVByteFee = 5` and make the follower return an error when the proposed fee is
   below `ProtocolMin x (follower vsize - 1)` on all five paths - `pkg/tbtc/reservation.go:579-592` and
   `pkg/tbtc/reservation.go:719-732` (anchor, re-anchor), `pkg/tbtc/deposit_sweep.go:258-265`,
   `pkg/tbtc/moving_funds.go:380-389`, and `pkg/tbtc/redemption.go:401-413` - keeping a warning only for
   fees below the operator's own bare floor, and validating the operator's
   `tbtc.walletTxSatPerVByteFloor` flag against the protocol minimum at startup.
4. G1-F2/F12 (Medium, PR-introduced): on-chain age check on the re-anchor target. A target is aged when
   `GetWallet(target).CreatedAt + walletCreationPeriod` <= the chain head time, read with the bounded
   `HeaderByNumber` pattern, never `time.Now()`.
   - Required on the follower: `reservationReanchorAction.execute` (`pkg/tbtc/reservation.go:692-777`)
     rejects un-aged targets, with a 2-block tolerance, because a malicious leader can skip any
     leader-side check.
   - Strict on the leader: the resume path and `reanchorTargetSearch.pick` skip un-aged targets, so honest
     leaders never propose one.
   - Fails closed when a read fails. No operator allowlist: per-operator lists split signing groups, and no
     on-chain list exists.
   - Known limits: a patient attacker holding a fabricated wallet can wait the period out, and re-anchors
     stall while no aged wallet has capacity.
5. G1-F11 (Medium, PR-introduced): wrap the reservation execute-path RPC calls in a bounded context
   deadline.
6. G4-F8/F9 (Medium, PR-introduced): validate each ephemeral-key entry's 33-byte shape at unmarshal
   (restores the pre-PR uniform-drop behavior).

Routing: every finding in this record is fixed in PR #4282 itself (see
[recommendations-pr-4282.md](./recommendations-pr-4282.md) for the change per package and
[fix-tracker.md](./fix-tracker.md) for the landed commits). Items against code that predates the PR are
routed outside this record and are not listed here.

The phase C re-run on the merged head adds PR fixes of its own; they are listed in
[rerun/README.md](./rerun/README.md).

## Appendix: leads referenced by the fix plan

The fix plan refers to a small number of speculative leads. They are listed here (and only here) so those
references resolve; the per-source lead tables are not published. A lead is an unvalidated observation, not
a finding.

| Lead | One-line verdict |
|---|---|
| G1-L21 | Unverified liveness trail: a target's own pending re-anchor can self-block; not shown to block the only viable target. |
| G1-L24 | Bounded gas griefing: cap-race burns operator gas per target; worst case bounded by candidate count. |
| G1-L25 | Race trail, not constructed: send without a final on-chain re-read; same window as the Run-level race. |
| G1-L26 | Liveness trail: the re-anchor floor read once per pass can go stale on a narrow governance-change window. |
| G1-L27 | Liveness, on-chain backstop exists: the below-dust check runs only on the zero-reservation path. |
| G1-L28 | Bounded gas griefing, Low: the cooldown race reverts the request; the node-clock-skew window is not measured. Evidence check: the exact gate expression from `reservation_reanchor.go:344` (`uint64(time.Now().Unix()) < uint64(reservation.ReanchorCooldownUntil)`) executed against `tbtcpg.LocalChain` (which mirrors `Reservation.sol`'s own `block.timestamp >= reservation.reanchorCooldownUntil` revert) - scratch test `evidence/g1/l28_cooldown_race_evidence_test.go`, `go test -v -run TestL28_ReanchorCooldownGate_HasNoSafetyMargin ./pkg/tbtcpg/`. With `cooldownUntil` 1s ahead of the gate's clock read the gate correctly skips (no chain write, no gas spent); at the exact boundary the gate has zero margin and production proceeds straight to the on-chain-mirroring call. Unlike the sibling `ActionPending` resume-path gate at `reservation_reanchor.go:288-290`, which applies `reservationRequestTimeoutSafetyMarginSeconds` (2 hours), the Active-state cooldown gate applies **zero** margin - so a node whose local clock runs even slightly ahead has no cushion before sending a write the chain may still reject, burning gas. Verdict and severity unchanged (Plausible, Low). |
| G2-L2 | Stale-deposit release depends on a second watcher staying alive: a dead action-timeout watcher leaves a Pending acceptance forever, so the stale-deposit watcher keeps the deposit tracked and re-reading it with no self-recovery. Existing `TestWireReservationWatchers_WatcherDeathIncrementsMetric` confirms the watcher-death recovery path exists and is observable only as a metric increment. |
| G2-L5 | Transient scan errors trigger the slow restart backoff: a transient chunk-fetch error forces the 30-minute `RestartBackoffTime` wait instead of the 10-minute idle wait. Existing `TestProveReservationAcceptanceActions_ChunkedScan` proves the error return with progress kept; the backoff branch selection is a direct read. |
| G2-L7 | Operator clock skew can evict a settleable late proof: eviction compares wall-clock `now` to `timeoutAt + termSeconds` while the Bridge compares `block.timestamp`. Existing `TestProveReservationAcceptanceActions_TimedOutLateWindow` proves the gate is wall-clock-only. |
| G2-L9 | A stale preloaded action can trigger a false timeout notice: a preload is reused whenever its nonce still matches; if another operator settles or times out the action in between, the notice reverts (cheaply) and retries next tick. |
| G2-L10 | Stuck pending entries are re-read every tick forever: a Pending entry whose notice never succeeds is never evicted. Scratch test `TestG2L10_StuckPendingActionEntryIsRetrackedEveryTick` shows one re-read per tick and continued tracking. |
| G2-L11 | Operator clock skew can submit a notice before the chain deadline: `now >= deadline` uses wall time while the Bridge gates on `block.timestamp >= action.timeoutAt`. |
| G2-L13 | Operator clock skew can submit a stale-deposit notice early: the deadline gate uses wall time while the Bridge gates on `block.timestamp > refundDeadline`. |
| G2-L15 | The live wallet-closed handler drops a failed stranding check: the one-shot `OnWalletClosed` goroutine logs and returns on a failed stranding check with no retry, so a read failure drops that wallet's stranding notice for this event. |

Leads not listed here are either pre-existing observations or trails that no fix references.