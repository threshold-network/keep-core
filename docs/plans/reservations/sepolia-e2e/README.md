# Sepolia end-to-end test plan - UTXO reservations (DRAFT, 2026-10-21)

Status: draft for human review. **Do not run `PHASE=prepare` until the human approves this plan and every entry gate in section 3 is met.** The test does **not** upgrade the shared Sepolia deployment. It first deploys an own copy of the pre-reservation contracts (section 1a), then runs `PHASE=prepare` and `PHASE=activate` on that copy. The lab scenarios (merge step 0b) run first because they are faster.

Files:
- this plan (scope, entry gates, roles, phases, open items, exit criteria);
- the step catalog: [part 1](steps-1.md) (sections A-D) and [part 2](steps-2.md) (sections E-K, with the capacity schedule);
- the traceability table [trace.tsv](trace.tsv): one row per spec ID.

`testnet-drill-runbook.md` stays the operator run sheet. Its scenarios D1-D12 keep their IDs and are mapped onto the catalog steps in part 2, section K.

## 1. What runs on Sepolia

| Item | Value |
|---|---|
| Contracts and deploy script | the reservation contracts at `6237b2741` |
| Public contracts PR | tbtc-v2 #1116 (`reservations-upgrade`), open, draft, mergeable, head `c27b71281` (checked 2026-10-21) |
| #1116 against the deploy source | `git rev-list --left-right --count c27b71281...6237b2741` = `0 112`: the PR head is fully contained, and the deploy source adds 112 commits. Contracts differ in 11 files (+975/-133): the private fixes that go public at merge step 6 (`d47be35eb`). The deploy source adds 36 commits and 22 files (deploy script, prepare guard, rehearsal artifact) on top of `d47be35eb`, none under `solidity/contracts` |
| Open #1116 review item | comment of 2026-10-15: script 98 trusts the BridgeGovernance address in the deployments record and only checks its owner. Mainnet activation only; not used on Sepolia (the Sepolia script is separate), but unresolved |
| Client | keep-core the drill-fix branch @ `5799d58b0`, plus the Sepolia bindings and activation block commit (not yet made) |
| SDK and depositor tooling | tbtc-v2 SDK `ReservationsService` from the same tbtc-v2 line |
| Networks | Ethereum Sepolia; Bitcoin testnet4 |
| Parameters | caps 2,000,000 / 2,000,000 / max active 2 / 1 per wallet; max total 4,000,000; hand-off reserve 2; third-party hold cap 1; term 90 days; action timeout 86,400 s (script header and drill runbook section 1) |

So Sepolia tests the #1116 contracts **plus** the private fixes, not #1116 alone. A pass on Sepolia is evidence for the tbtc-v2 candidate `d47be35eb`; it says nothing about #1116 as it stands today.

## 1a. Own contract copy

| Item | Value |
|---|---|
| Base contracts | tbtc-v2 `502cd3982` (the lab pin). Its `solidity/contracts` equal `origin/main` `e99676d0e` and the merge base `40a11d1dc` of the deploy source (no diff, checked 2026-10-21). Not yet compared with the bytecode that mainnet runs |
| What is deployed | the whole stack the Bridge needs: T token and staking, random beacon, ECDSA wallet registry and sortition pool, Bridge, Bank, TBTC vault, light relay and the governance contracts, with this test's own owner and governance accounts |
| Deploy method | the tbtc-v2 and keep-core deploy scripts at the base commit, on a new Hardhat network name with its own `deployments/` record, so the `sepolia` record of the shared deployment is not changed. The lab's deploy path (`lib/deploy.sh`) is the model |
| Operators | own test operators, staked and registered in the copy's sortition pool; enough for the copy's group size. Wallet group size and DKG parameters are set by governance of the copy |
| Wallets | create enough wallets by DKG before the schedule starts (section 5, item 1) |
| Upgrade | `PHASE=prepare` then `PHASE=activate` from `6237b2741`, with the deployment record of the copy; the client bindings use the copy's addresses |

Changes that this needs before step 1a:
- a deploy profile for the copy (network name, deployments record, owner accounts) on a tooling branch;
- the prepare and activate scripts must read the copy's record; check every hard-coded Sepolia address in `6237b2741` scripts and the drill runbook;
- keep-core client config and bindings for the copy's addresses;
- Sepolia test ETH and test T for the deployer, the operators and the depositor.

What a shared-deployment upgrade would have tested and the copy does not: the real Sepolia state from before the upgrade (old deposits and wallets). The copy starts empty; the base stack must therefore create wallets and run a few pre-upgrade deposits and redemptions first, so the upgrade meets live state.

## 2. Coverage

The spec is the verification specification at keep-core `c9bfa021f` (`docs/spec/reservations/verification/`). Its traceability matrix lists **352** canonical IDs, one row each. The count comes from `tools/sepolia-plan/check_trace.py`, which reads those rows.

Every ID has exactly one disposition in `trace.tsv`:

| Disposition | Meaning |
|---|---|
| `sepolia` | forced live on Sepolia by a catalog step `S-xx`, including negative cases with the exact revert string |
| `sepolia-observe` | checked by a read or assertion `C-xx` during or after the live steps |
| `offchain` | proven by a named existing test (Hardhat, Go, Medusa or fork run); `reason` says why it is not run live |
| `not-testable` | neither; `reason` names why and the substitute |
| `n/a` | matrix rows with status `model` or `pointer` only |

Counts (output of the checker, 0 errors):

| Prefix | sepolia | sepolia-observe | offchain | not-testable | n/a |
|---|---|---|---|---|---|
| SM system model | 23 | 19 | 0 | 1 | 15 |
| INV invariants | 31 | 8 | 2 | 1 | 0 |
| PAR parameters and governance | 33 | 1 | 0 | 0 | 0 |
| US user stories | 72 | 0 | 0 | 0 | 1 |
| EC edge cases | 20 | 0 | 3 | 1 | 0 |
| FM failure modes | 13 | 0 | 0 | 1 | 0 |
| DR disaster recovery | 7 | 1 | 0 | 4 | 0 |
| TM threat model | 15 | 0 | 3 | 2 | 0 |
| XR cross-repo contract | 41 | 15 | 1 | 2 | 0 |
| AR accepted risks | 11 | 0 | 1 | 4 | 0 |

Check command:

```
python3 tools/sepolia-plan/check_trace.py --repo <keep-core checkout> --spec-rev c9bfa021f \
  --trace docs/security/pr-4282-audit/sepolia-e2e/trace.tsv \
  --steps docs/security/pr-4282-audit/sepolia-e2e/steps-1.md docs/security/pr-4282-audit/sepolia-e2e/steps-2.md
```

It fails on a missing, extra or duplicated ID, an unknown disposition, a step that the catalog does not define, an `offchain` row without a test, a `not-testable` row without a reason, and `n/a` on a row that is not `model` or `pointer`. The script is `tools/sepolia-plan/check_trace.py` on (run record not published) (this branch is docs only).

Limits of this mapping:
- A row's disposition is a judgment. The checker proves that every ID is assigned to a defined step, not that the step really exercises the behavior. The human or a reviewer must read the steps.
- The spec is pinned at older commits (`9d2086965`, D31 at `985740b21`). It was not re-pinned to `6237b2741`. Two spec entries disagree with the code at `6237b2741` (section 6).
- Every revert string in quotes in the catalog was checked to exist in `solidity/contracts` at `6237b2741` (2026-10-21). Events were checked by name only.

## 3. Entry gates (before `PHASE=prepare`)

1. The human approves this plan (scope, schedule, wallet setup in section 5).
2. Inputs filled in `testnet-drill-runbook.md` section 1: operator set, SPV maintainer address, keeper operator, metrics endpoint and alert receiver.
3. The own copy is deployed (section 1a), its operators are registered, and it has enough Live wallets for every fresh acceptance in the schedule (section 5, item 1), plus some pre-upgrade deposits and redemptions.
4. The fault-injection method for D10 and D11 is chosen, or both are marked "not run" with the reason.
5. A decision on D8 (fraud stranding): a coordinated test-wallet setup, or "not run".
6. The Sepolia RPC endpoint and the deployer key are available to the human who runs the script.
7. The prepare fork rehearsal is re-run on `6237b2741` against a Sepolia fork that contains the own copy (the 2026-10-17 run, 24 pass / 0 fail / 3 off-chain skips, was against the shared deployment).
8. The heavy jobs (merge step 0a) and the lab scenarios L-01 to L-08 (merge step 0b) passed on the same contracts and client. A lab failure blocks step 1a.

## 4. Phases

| Phase | Steps | Owner | Gate to the next phase |
|---|---|---|---|
| P-1 copy | deploy the base copy, register operators, DKG for the wallets, pre-upgrade deposits and redemptions (section 1a) | human (deployer key), agent (scripts), operators | addresses recorded; wallet count met |
| P0 prepare | `PHASE=prepare` on the copy = S-01 to S-06; then S-09, S-97, S-98 negatives | human (deployer key) | prepare artifact written; addresses recorded |
| P1 client | bindings from the real addresses, `make verify-vendored-fallback`, activation block B (at least 1 day ahead), drill client release | agent (code), human (release approval) | every drill operator runs the build (`/diagnostics`) |
| P2 activate | S-07 (`PHASE=activate ACTIVATION_BLOCK=B`, wiring read, `setVaultStatus`), S-08 baseline, C-11 metric baseline | human | post-activation checks pass |
| P3 live lifecycle | capacity schedule order 0-8 (part 2, section K) | depositor, operators, SPV maintainer, outside account, governance | each step's pass condition recorded |
| P4 keeper and faults | S-71 to S-76, S-81 to S-83 | keeper operator, operators | each alert reaches the receiver |
| P5 closeout | C-01 to C-11 final snapshot, exit review | agent and human | section 7 |

Duration: not estimated yet. Known lower bounds from the parameters: an acceptance or action timeout is 24 h each; the stale-deposit step (D9 / S-53) waits for the refund deadline, at least the Sepolia reveal-ahead period after the reveal (drill runbook section 3 states 15 days); moving-funds and closing steps wait for the Sepolia wallet timeouts, which were not read from the chain yet. Governance ceremonies wait out the real governance delay. An earlier plan note says "about 2 weeks"; no source for that figure was found, so it is not used here.

## 5. Open items

1. **Live wallet count for D7 and D8 (blocking).** The hand-off reserve gate requires `active count after the acceptance <= (Live wallets - 2) * 1` (`Reservation.sol:1064-1079` at `6237b2741`). The schedule retires W1 (D3+D5), then Wx and W2 (D6). With 4 Live wallets at the start, only 1 is Live after D6, so the fresh acceptances for D7 and D8 revert with `Hand-off reserve exceeded` unless new wallets are created first: each needs at least `count + 3` Live wallets (4 when the count is 1). With the own copy this is solved in phase P-1: create at least 7 Live wallets before the schedule starts (4 for D7 or D8 after the 3 retirements), or create new wallets between D6 and D7.
2. Own-copy parameters not chosen yet (they set the schedule, and the copy's governance can set them): `depositRevealAheadPeriod`, wallet creation period, moving-funds timeout, closing period, governance delay, and the current Live wallet count. They set the schedule.
3. Drill runbook TBDs: activation block, operator set, SPV maintainer, keeper operator and receiver (runbook section 1).
4. Fault injection for D10 (watcher panic) and D11 (chain-read failure): method TBD.
5. D8 (fraud stranding, Story 5) needs a test operator setup.
6. Spec re-pin to `6237b2741`, and the two corrections in section 6.
7. Optional: staging #455 on the drill line (human decision pending); it does not change the reservation steps.

## 6. Spec entries that disagree with the code at `6237b2741`

Found while mapping; both checked at the cited lines.
- **INV-102.** The spec says the moving-funds path checks the wallet's reservation count. The code checks `walletAnchoredInfo[w].count == 0` (proven anchors) when it chooses between closing and moving funds (`Wallets.sol:628-637`). Fix the spec text; the catalog tests the code's behavior.
- **US-01 / US-02.** A reserved reveal also reverts with `Refund locktime too far in the future for a reservation` when the refund deadline is later than `now + term + 24 h` (`Deposit.sol:244-251`). The stories do not state this rule. S-18 covers it.

## 7. Exit criteria

- Every `sepolia` step in `trace.tsv` has a recorded result (pass, fail, or "not run" with the reason). A "not run" is never counted as a pass.
- The drill runbook exit criteria (section 6 there) hold.
- Every `sepolia-observe` check holds at the final snapshot (C-01 to C-11).
- Every failure has a finding or an issue, routed per the session rules.
- The checker passes on the final `trace.tsv`, with a result column added.
- The evidence (block numbers, tx hashes, metric snapshots, alert receipts) is stored with the drill log.
