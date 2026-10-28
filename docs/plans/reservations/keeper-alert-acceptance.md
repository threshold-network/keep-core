# Reservation keeper alert acceptance

Date: 2026-10-13. Scope: a local mainnet fork and actual application processes.
This report uses STE-inspired English.

## Result

All four required alerts reached the local Sentry capture endpoint through the normal monitoring CLI.
The healthy control and debt repayment control produced no keeper alerts.
Each successful monitoring run advanced its checkpoint and left no pending Sentry outbox entries.

| Alert | Observed value | Sentry level | Evidence file |
|---|---:|---|---|
| Late acceptance proof at risk | 1 generation | fatal | `monitor-age-fixed-age.json` |
| Re-anchor has no eligible target | 1 reservation | fatal | `monitor-no-target-fixed-no-target.json` |
| Vault fee debt | 1000 sat | warning | `monitor-fee-original-fee.json` |
| Unrequested deposit deadline at risk | 1 deposit | warning | `monitor-age-fixed-age.json` |

## Runtime and boundaries

- Ethereum: a separate loopback Anvil fork at mainnet block 26154824, chain ID 1.
- Contracts: the unmodified activation script at tbtc-v2 on the reservation upgrade line; 22 checks passed, none failed or skipped.
- Keeper: actual `keep-client maintainer --mainnet --spv`, with reservation proofs enabled and an unstaked local account.
- Client code before the fix: keep-core `d8fce4707`.
- Client code after the fix: keep-core `9c4e1b221`, integrated into the drill-fix branch.
- Monitoring: the normal `node dist/index.js` entry point of the tbtc-v2 `monitoring/` package, including all six existing chain monitors.
- Alert receiver: the actual Sentry SDK and HTTP envelope transport, pointed only at a local capture server.
- Bitcoin: a real mainnet Electrum server for application startup and existing monitoring reads.

The release activation map is empty until rollout.
The two fork acceptance builds used the same temporary Go overlay to set the local fork activation block to 26154824.
The overlay is not committed client source and must never enter a release build.

The proof fixture redirected the Bridge relay to `SystemTestRelay` with difficulty 0.
This is the labelled relay seed used by the existing fork lifecycle recipe.
The run does not prove Bitcoin mainnet proof validity, signing-quorum availability or live Sepolia deployment.
No mainnet or Sepolia transactions were sent.
No alert reached an external Sentry or Discord account.

## Contract conditions

The driver used existing mainnet wallets, not newly registered test wallets.
It revealed three reserved deposits through the Bridge.
It requested acceptance for A and B; C had no request.

A settled through a real acceptance proof and a real re-anchor proof.
The vault owner emptied the initiation-fee reserve through its real sweep method.
The re-anchor miner fee then created 1000 sat of in-kind fee debt.
Repayment used minted TBTC through `repayInKindFeeDebt` and cleared that debt.

The no-target scenario used real governance to set the action timeout to 7201 seconds.
It retired A's current custodian through `notifyWalletCloseable`.
The reservation remained Active and its anchor exceeded the amount floor.
An existing Live target remained available, but both governance and third-party requests reverted with `Reanchor authorization has no signing window`.
No wallet-state or fee-debt storage was seeded.

The age scenario checked B's TimedOut action and snapshotted late-proof edge.
At the recorded head, that edge was 217445 seconds away, inside the seven-day risk window.
C remained reserved with request nonce 0.
Its last request time was 41043 seconds away, inside the one-day risk window.
The driver mined 14 confirmation blocks.

An earlier fixture assertion also required the target time to remain within ten minutes of a twelve-hour estimate.
That incidental bound failed despite the actual one-day risk condition holding.
The driver now checks the actual window and can resume after that partial time advance.

## Discovery regression and fix

The original observer scanned wallet registrations from reservation activation.
Existing custodians and usable targets therefore disappeared from its wallet cache.

Two regression tests failed before the fix:
- An old retiring source produced a no-target count of 0 instead of 1.
- An old usable target produced a no-target count of 1 instead of 0.

The fork reproduced the missed source alert with a fresh observation timestamp.
The original no-target run failed with all four risk gauges at 0 despite the on-chain rejection above.
The corrected binary, against the same fork state and activation pin, delivered the fatal no-target alert with value 1.

The fix scans wallet registration history independently of reservation activation, in bounded chunks.
Reservation acceptance and deposit-reveal scans retain their activation boundary.
Risk calculations moved into `reservation_keeper_risk.go` to keep both implementation files below 500 lines.

Verification run by the parent:
- The two regression tests failed on the original code.
- All `TestReservationKeeperObserver` tests passed after the fix.
- The full `pkg/maintainer/spv` package passed.
- `go vet ./pkg/maintainer/spv` passed.
- The normal client build and the fork-overlay builds passed.
- The normal monitoring CLI delivered the alert after the fix.

## Keeper health control

The outage run paused only the owned keeper process so that its metrics request timed out.
No script in the harness sends the pause signal, so the pause was a manual step, and no cleanup trap is recorded.
Afterwards the keeper was found still stopped (state `T`) and needed a manual SIGCONT. The cause was not found.
After every stale-stage run, check that the keeper is not in state `T` before the next stage, or the next stage fails its precondition with an unavailable endpoint.
The real metrics request timed out while the fork and capture receiver stayed available.
The monitoring CLI delivered one fatal `Reservation keeper metrics stale` event.
Its reason was `metrics endpoint unavailable`.
The manager succeeded, advanced its checkpoint and emptied the Sentry outbox.
Evidence: `monitor-stale-fixed-outage.json`.

## Evidence handling

The acceptance evidence includes actual metric samples, manager reports, checkpoints and Sentry delivery assertions.
The expected missing-alert run is retained alongside passing controls.
All endpoint values, passwords and keys are excluded from committed evidence.

Monitor build per stage: baseline, fee, repay, no-target and stale ran on the older monitor.
Only the `age` stage was repeated on `8eb26c2e2`.

Harness note: the Testnet4 startup smoke appends to its log and reuses its data directory, so a stale `status: 'failure'` line from an earlier run ends a later run after about one second.
Clear the log, result file and data directory before each run.

## Monitor build for Sepolia and Bitcoin Testnet4

The monitor to use is the one on tbtc-v2's reservation upgrade line, which carries `8eb26c2e2`.
It already builds against the in-repo SDK (`4.0.0-dev`), maps `ENVIRONMENT=testnet` to Sepolia and Bitcoin Testnet4, guards the zero-supply case, and has the repaired Docker build.
Nothing is published to npm.

The first acceptance runs used the older monitor, which the local rehearsal checkout still held.
The checks below repeat the relevant ones on `8eb26c2e2`.

| Check | Result |
|---|---|
| Registry SDK `v2.5.0-dev.5` network check | Does not recognize Testnet4 |
| Registry SDK `latest` (3.2.0) and `development` (3.2.1-dev.1) | No Testnet4; no 4.x is published |
| Older monitor started against Testnet4 | Failed during setup |
| `8eb26c2e2` monitor: `tsc`, test compile, `node dist-test/test/run.js` | Pass; 51 of 51 tests ok |
| `8eb26c2e2` monitor `age` stage on the fork | Pass; fatal late-proof, fatal no-eligible-target, warning unrequested-deadline, each delivered once; manager success, checkpoint advanced, outbox empty |
| `8eb26c2e2` monitor startup on Testnet4 with a real Testnet4 Electrum server and read-only Sepolia | Setup completes, network is Testnet4, manager report `success` with no errors |

The host build was checked in this pass.
The Docker image of `8eb26c2e2` was not rebuilt here.

Evidence: `monitor-age-rehearsal-head-age.json`, `rehearsal-head-testnet4-startup.json`, `legacy-testnet4-startup.json`, `testnet4-network-proof.json`.

Not proven: a live Sepolia keeper, a live Sentry route, or a Testnet4 proof of Bitcoin proof validity.
The fork proof uses a labelled difficulty-0 relay seed.
The Mainnet activation override used on the fork is a throwaway overlay and does not ship.
