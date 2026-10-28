# Reservations chaos lab scenarios (DRAFT, 2026-10-22)

Status: **stage 1 of the release test plan**. Run the lab before Sepolia (human decision 2026-10-21). Test code is on a lab branch in `tbtc-v2-chaosnet`; CI configuration is unchanged. Runs use `./lab remote run`. L-01 passed all 23 steps; L-02 passed all 29 steps; L-03 passed all 29 steps; L-04 failed at step 37 on all three attempts and is open (see its row); L-05 passed all 42 steps on its second attempt; L-06 passed all 28 steps on its third. All nine scenarios validate. L-09 cannot run against current client `a2cf27800` (section 3).

L-01's three failed attempts used client `5799d58b0`. The human then approved a private binding refresh and one more attempt. Client `a2cf27800` is a descendant of `5799d58b0`, generated from contracts `6237b2741`. Heavy job `20261023-100102-4abba4c3-debian` passed the full Go build, requested package tests, eight real-artifact ABI comparisons and CLI smoke. L-01 run `20261023t101441z-proslaniec-df56`, lab `d4cb86b`, passed with that client. L-02/L-03 use lab `e9cafbc`. Contract pins stay unchanged. Lab repairs remain local because the unchanged scanner still times out on the existing selftest file.

Upgrade path: each scenario deploys the pre-reservation stack first and then upgrades it, the same way the Sepolia copy does. The lab pin `LAB_TBTC_COMMIT=502cd3982` already is the pre-reservation base: its `solidity/contracts` equal those of `origin/main` `e99676d0e` and of the merge base `40a11d1dc` of the Sepolia source (no diff, checked 2026-10-21). The `reservations-activate` ceremony runs the upgrade from `6237b2741` (`PHASE=prepare`, then `PHASE=activate`) on that base, not deploy the new contracts fresh. `bootstrap` takes `reservations: true` to run the prepare between governance and the image build, and the ceremony activates after the wallets exist.

The scenario files live in the audit run record, `tools/lab-scenarios/` (this branch is docs only). There is no generator: the drafts were written by a throwaway script, then edited by hand, so the files are the artifact. Their final home is a PR to the internal chaosnet lab repository, together with the section 4 extensions (private repository with CODEOWNERS and no `AGENTS.md` at `70e2054`).

## 1. What the lab is and why the release needs it

The chaos lab brings up a private EVM chain (anvil), the full Threshold and tBTC contract stack, N real keep-core nodes, a Bitcoin regtest node with an Electrum server and the SPV maintainer. It then drives each tBTC ceremony and passes or fails on gates. A gate reads chain state, Bitcoin state or node diagnostics. Faults (stop a node, partition a node, stop Electrum, stop the maintainer) and checkpoints are scenario steps.

It is the only place where real keep-core nodes sign reservation anchors on Bitcoin and the SPV maintainer proves them, outside Sepolia. The Hardhat, Go, Medusa and fork runs test each side alone. A fork has no wallet keys.

The internal verification standard (`standard/verification.md`) makes it a requirement:
- Tier rule 3.1: code that will hold funds after its first deployment takes its future tier (T0 for keep-core and tbtc-v2) and must meet it before the first mainnet deployment.
- **LAB-1:** T0 node code and the contracts the node calls must have lab scenarios. A scenario passes or fails only on chain state, Bitcoin state and structured events. It must not match free log text, and no LLM decides the result. Each scenario names the registry IDs it checks and the PR or defect it covers.
- **LAB-2:** the scenarios must include a hostile participant (malformed, out-of-range, contradictory, delayed and replayed messages).
- **RUN-1, RUN-2:** lab runs are heavy jobs. They run on the verification machine `chaosnet-lab`, start from a GitHub workflow and report a check run with a result bundle. A result that stays on the machine is not release evidence.
- **EVID part 6:** the evidence bundle of a node or contract change lists the lab scenarios that ran, with registry IDs.

The lab's own record (`docs/proven-end-to-end.md` there) covers DKG, deposit, mint, redemption, moving funds, moved-funds sweep, faults, checkpoints and node counts 2, 3 and 10. It has no reservation ceremony. It pins `keep-network/tbtc-v2` `502cd3982` and `keep-network/keep-core` `a7ac8989b` (`lab.env`), so no existing lab run is evidence for `d47be35eb` or the drill client.

## 2. How to write a reservation scenario

Follow the lab's format (`docs/scenarios.md` there). Rules that matter here:

1. **One file, ordered steps.** The order is the scenario. Top-level keys: `name`, `description`, `nodes`, `registry_ids`, `covers`, `weights`, `chain`, `upgrade`, `steps`, `dkgValidation`. Validation is strict: an unknown key or name fails before Docker starts.
2. **Name the IDs in `registry_ids`.** The field lists the spec IDs the scenario checks: `KC-NNN` for keep-core's invariant registry, or the reservation trace IDs (`US-`, `INV-`, `EC-`, `FM-`, `DR-`, `AR-`, `PAR-`, `SM-`, `TM-`, `XR-<area>-`) that `sepolia-e2e/trace.tsv` assigns to the catalog steps. The validator accepts both and nothing else. `covers` names the PRs or issues. The `description` starts with the scenario ID (L-xx) and then states what the file proves, so a reader sees the claim before the steps.
3. **Gates read the chain, never log text.** Every new reservation gate reads Bridge or router state, an event filtered by contract address and topic, a transaction receipt, or the regtest chain. A gate that greps logs does not count for LAB-1.
4. **Scope every gate.** Put a `mark` before the work and give each gate `from: <mark>`, so a rerun or a second round cannot pass on an earlier event.
5. **Make negatives falsifiable.** A step that must fail gets `expect: "failure"` and a short `timeout`. Say in its `note` why the same gate would pass without the fault.
6. **Checkpoints are namespaced.** `<scenario name>-post-<phase>`. Two scenarios must never share a checkpoint name.
7. **End with the conservation gate.** Each scenario that changes reservation state ends with `reservation-counters-conserved` (catalog check C-06).
8. **Validate before running.** `./lab scenario validate <file>` and `./lab scenario run <file> --dry-run` cost seconds. A run that cannot work must fail there.

## 3. Scenario catalog

| ID | File | Nodes | Covers | Spec IDs | State |
|---|---|---|---|---|---|
| L-01 | `resv-acceptance-happy-path.json` | 2 | D1; S-07, S-13, S-15, S-21, S-22, C-06 | AR-16, US-01, US-03, US-20, US-07, INV-201 | PASS: `20261023t101441z-proslaniec-df56`, 23/23 steps; bundle fetched and read |
| L-02 | `resv-acceptance-timeout.json` | 2 | D2; S-15, S-24, C-06 | US-06, EC-01, INV-601, SM-X5, INV-201 | PASS: `20261023t150123z-proslaniec-dfe4` (attempt 2 in isolation), 29/29 steps, exit 0, 7270s; evidence at `2735e33c7` |
| L-03 | `resv-reanchor-on-rotation.json` | 3 | D3, D5; S-31, S-37, C-08, C-06 | US-23, US-33, INV-302, INV-201 | PASS: `20261023t170814z-proslaniec-2dd0` (attempt 2 in isolation), 29/29 steps, exit 0, 14767s; evidence at `730adca97` |
| L-04 | `resv-third-party-hold-and-strand.json` | 3 | D4, D6; S-32, S-33, S-50, C-06 | US-70, US-71, US-72, PAR-14, US-13, US-49, TM-07, INV-201 | FAIL, open: `20261027t124447z-proslaniec-e38e` (attempt 3), 37/43 steps, exit 1. Three attempts; the first stopped at configuration admission, the second and third at step 37 on `notifyReservationStranded` reverting `Reservation is not active` while the same key reads `Active`. Human decision 2026-10-27: record and continue |
| L-05 | `resv-sweep-strand-with-new-wallets.json` | 3 | D7; S-40, S-50, C-06 | INV-113, US-13, US-49, DR-02, INV-201 | PASS: `20261027t201203z-proslaniec-d0d9` (attempt 2), 42/42 steps, exit 0, 11652s, evidence at `07ced00fb`. Creates a new wallet, which is how the lab solves the D7/D8 wallet-count problem of the Sepolia plan. S-51 has no row in `trace.tsv`, so the strand is cited through S-50 |
| L-06 | `resv-stale-and-release.json` | 2 | D9, D12; S-16, S-53, C-06 | US-90, INV-111, SM-D3, US-10, US-48, INV-201 | PASS: `20261028t061528z-proslaniec-45ff` (attempt 3), 28/28 steps, exit 0, 7368s, evidence at `39043a6ba`. Step 22 now reveals with `--vault none`, because the client sweeps one vault group per window |
| L-07 | `resv-restart-and-outages.json` | 3 | D10 (restart half); S-81, S-83, S-22, C-06 | FM-03, FM-04, US-46, US-07, US-03, INV-201 | validates; not run |
| L-08 | `resv-chain-read-fault.json` | 3 | D11; S-82, S-50, C-06 | FM-12, US-13, US-49, EC-11, INV-201 | validates; not run. `partition-node` cuts peers, chain and Electrum together, a wider fault than D11 asks for |
| L-09 | `resv-hostile-participant.json` | 3 | D8, S-21, S-22, C-06; LAB-2 partly | US-03, US-20, US-21, US-07, INV-201 | validates; **not run against client `a2cf27800`**. Like its parent `5799d58b0`, this tree has no `pkg/net/byzantine`; `lib/hostile.sh` refuses to label an honest stock node hostile. The fraud-strand half of S-52 remains out of lab scope because no single share can produce the required wallet signature |

The spec IDs are the ones `sepolia-e2e/trace.tsv` assigns to those catalog steps, taken from the "Covers" line of each step in `steps-1.md`/`steps-2.md` and checked against the map, not invented. Where a step the scenario runs has no row in `trace.tsv` (S-51, S-52), the closest traced step is cited and named in the row.

L-09 as written proves less than LAB-2 asks for: it shows that an honest quorum signs and settles a reservation while a member sends contradictory, delayed and replayed tECDSA messages. The fraud-challenge strand stays a Sepolia step.
There is no `wallet-fraud` ceremony and no `hostile-signing` fault kind: the hostile shape the lab already has is `fault kind=hostile_node` with a plan of contradictory, delayed and replayed signing messages, which needs a byzantine client build (`-tags byzantine` over a tree that carries `pkg/net/byzantine`).
LAB-2 also counts the hostile actors that the contracts face: L-04 uses an outside account against the hold cap, and L-02 and L-04 run with a signing group below quorum. Those are hostile counterparties for the contracts; L-09 is the hostile node the standard asks for.

Every catalog step that a lab scenario covers stays in the Sepolia plan: the lab proves the code path with real nodes and a fast clock, and Sepolia proves the deployment, the real operators and the real time.

## 4. Lab extensions

Owner: a change in the internal chaosnet lab repository, reviewed under that repository's rules (it is a T0 checker repository, REV-1). Implemented on the branch a lab branch; nothing merged.

What landed there: nine ceremonies (`reservations-activate`, `reserved-deposit`, `reservation-accept-request`, `reservation-reanchor-request`, `reservation-release`, `reservation-notify-timeout`, `reservation-notify-stale`, `reservation-notify-stranded`, `reservation-reveal-ahead`), ten gates (`reservations`, `reservations-active`, `reservation-accepted`, `reservation-acceptance-timed-out`, `reservation-reanchored`, `reservation-stranded`, `reservation-hold-cap-refused`, `reservation-counters-conserved`, `reserved-deposit-released`, `reserved-deposit-swept`, `reserved-deposit-stale`), the `chain` step key `warpSeconds`, the `bootstrap` flag `reservations`, and the offline mirror path `LAB_TBTC_OBJECT_MIRROR`/`LAB_KEEPCORE_OBJECT_MIRROR` (section 4.1). `wallet-retire` already existed.

What did not land, and why: no `wallet-fraud` ceremony and no `hostile-signing` fault kind. The fraud path needs a wallet signature over an unauthorized sighash, which no lab actor can produce, and the hostile shape the lab has is `hostile_node` with a signing-message plan, which needs a byzantine client build the drill tree does not carry.

Private source access is configured as of 2026-10-23. The human installed the repository token for the `lab` account. Remote admission builds the two bare mirrors before the scenario starts (`342236d`). The credential helper reads the token file without putting its value in arguments or run artifacts. The deploy and image resolvers still read local objects only, with no fetch or fallback. Revoke the token after the last run and remove its installed copies.

### 4.1 What each extension does

| Kind | Name | What it must do |
|---|---|---|
| source | private refs, as a mirror | remote admission fetches base `502cd3982`, target `6237b2741` and client `a2cf27800` into lab-relative bare mirrors. `LAB_TBTC_OBJECT_MIRROR` and `LAB_KEEPCORE_OBJECT_MIRROR` identify them. Existing resolvers require complete pinned commits and paths under the lab root. The deploy receipt and upgrade ledger record the resolved commits and trees. Credentials stay outside the run artifacts |
| source | lab bindings | the drill client needs reservation bindings and an activation block for the lab chain id |
| ceremony | `reservations-activate` | deploy and wire the reservation contracts (the 95-97 equivalents), set lab caps (`LAB_RESERVATION_*`, including the hand-off reserve per scenario), pick block B, read the vault wiring (AR-16), then `setVaultStatus(vault, true)` |
| ceremony | `reserved-deposit` | fund a reserved deposit on regtest and reveal it with the reservation vault |
| ceremony | `reservation-accept-request` | the depositor's `requestReservationAcceptance` with the funding proof |
| ceremony | `reservation-reanchor-request` | an outside account's `requestReservationReanchor` |
| ceremony | `reservation-release` | `releaseReservedDepositToPool` |
| ceremony | `reservation-notify-timeout`, `reservation-notify-stranded`, `reservation-notify-stale` | the permissionless notify calls |
| ceremony | `wallet-retire` | start a wallet's retirement when its signing group is below quorum |
| ceremony | `reservation-reveal-ahead` | set the Bridge's deposit reveal-ahead period as its governance, so a reserved deposit can be revealed inside the lab's window |
| gate | `reservations-active` | vault wired and trusted, read from the Bridge and the vault |
| gate | `reservation-accepted`, `reservation-reanchored`, `reservation-stranded`, `reservation-acceptance-timed-out` | the event from the Bridge plus the reservation status read back |
| gate | `reserved-deposit-released`, `reserved-deposit-stale` | the deposit's reserved flag and the event |
| gate | `reservation-hold-cap-refused` | the receipt of the refused outside request and `pendingThirdPartyReanchorHolds` |
| gate | `reservation-counters-conserved` | the global and per-wallet counts and amounts equal the sum over Active positions |
| chain key | `warpSeconds` | a `chain` step key over the existing `lab chain warp <seconds>`. The step spends the lag budget, mines a block, then reads the latest block's timestamp before and after and fails unless it moved by at least the requested seconds. The budget's floor (`LAB_CLOCK_MIN_LAG_SECONDS`) is respected inside `lab chain warp`, so a scenario cannot spend past the reserve |
| step key | `reservations` on `bootstrap` | prepare the reservation contract upgrade between governance and the image build, and set `LAB_RESERVATIONS_ENABLED` for every render in the run, so the client and the maintainer carry the reservation duties |

## 5. Validation result

`./lab scenario validate` on each of the nine files against the branch a lab branch: **all nine pass, exit 0** (2026-10-22). The same validator run covers every pre-existing scenario in `scenarios/`, so the `warpSeconds` and `registry_ids` changes did not break the catalog. `bun test test/scenario.test.ts` passes 149 tests, including the new cases for the registry-ID shapes.

The validator checks structure, names, scoped gates and checkpoint names. It does not establish contract behavior or the reason a call reverts. L-01 now proves deployment, two DKGs, activation, reservation acceptance with a real keeper proof, conservation and checkpoint creation. The remaining scenarios still need their own end-to-end results.

At `70e2054` the same validator stopped six files on `warpSeconds` and the `hostile-signing` kind and confirmed 3 missing ceremonies and 4 missing gates. Those extensions are now in the branch, so the names resolve.

## 6. How to run

Prerequisites, per machine:

1. `jq` 1.7 or newer on `PATH`. The lab refuses 1.6.
2. For remote runs, install the repository token for the `lab` account in `~/.config/lab/gh-token` with mode `0600`. Remote admission builds mirrors containing `502cd3982`, `6237b2741` and `a2cf27800`. Local runs still need existing lab-relative mirrors named by `LAB_TBTC_OBJECT_MIRROR` and `LAB_KEEPCORE_OBJECT_MIRROR`; the source resolvers never fetch.
3. `LAB_DEPLOY_ECDSA=candidate` is the committed lab default and needs a package token with `read:packages` for the private npm registry. Without that scope the bootstrap fails at candidate preparation. `lab.local.env` (untracked) can override it to `upstream` for a local run, which tests a different deployment than a real run, so say so in the run record.
4. Docker with about 16 GiB free for a 3-node scenario.

The initial Ethereum clock lag must cover the scenario's deliberate time advances and leave the lab's safety margin. Use the following run overrides; protocol timeouts and caps are unchanged.

| Cases | `LAB_CLOCK_LAG_SECONDS` |
|---|---|
| L-01, L-07 | `43200` |
| L-02 | `172800` |
| L-03, L-05, L-08 | `90000` |
| L-04 | `691200` |
| L-06 | `1382400` |

For example: `./lab remote run -m chaosnet-lab -e LAB_CLOCK_LAG_SECONDS=172800 scenario run reservations/resv-acceptance-timeout.json`. The override belongs to remote admission; exporting it only in the local shell does not configure the remote run. The default 43200-second lag cannot support the day-long and multi-day advances in L-02, L-04 and L-06.

Then:

1. Local: `./lab down --volumes`, then `./lab scenario run scenarios/reservations/<file>.json`. A run over an existing volume set fails in `bootstrap` on purpose; the volumes carry a chain the scenario cannot reuse.
2. Dev run on the verification machine: `./lab remote run -m chaosnet-lab scenario run <file>` from the lab checkout, with the file in its `scenarios/`. The `heavy-jobs` skill is not the path for lab scenarios. One run uses one of the 2 lab slots (13 to 17 GiB RAM each); the bundle lands in the lab run directory (`runs/<id>/`). This is a measurement, not release evidence (RUN-2).
3. Record each run in the release-verification table of the audit run record (not part of this record) with the run id, the commits under test, the scenario file hash and the result.
4. Only a run started from a GitHub workflow and reported as a check run reaches class A. The `lab` job does not exist in the CI workflow yet; adding it needs the human's approval and touches the private CI workflow, not this branch.
