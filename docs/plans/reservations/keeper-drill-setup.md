# Reservation keeper drill setup

This procedure uses STE-inspired English.
It prepares one independent keeper and one monitoring writer for the Sepolia drill.
It does not authorize deployment, npm publication, key access or external alert configuration.

## 1. Release prerequisites

Do not start the drill before O6 is complete.

1. Use the final Sepolia deployment artifacts from the Sepolia rehearsal branch.
2. Publish the Sepolia contract package through the human-held release process.
3. Regenerate keep-core bindings and verify the vendored ABI parity.
4. Compile the future activation block B into the released client.
5. Confirm that every signing operator runs that client.
6. Read `Bridge.reservationParameters()` with the deployed ABI.
7. Require its vault address to match the deployed ReservationVault.
8. Activate the trusted vault only at or after B.

The keeper fix is on the drill-fix branch at `d53c0c8d2` or a descendant.
The release still needs the real Sepolia bindings and activation block.
The fork-only Go overlay in the acceptance run must never enter a release build.

## 2. Keeper account and configuration

Use one separate `keep-client maintainer` process.
No operator stake is required.
Use a human-provided keyfile and external `KEEP_ETHEREUM_PASSWORD` environment variable.
Do not put the password or a private key into this runbook or the repository.

The observer sends no transactions.
The surrounding maintainer also runs proof loops and permissionless reservation watchers.
Fund its account with Sepolia gas.
If it must submit SPV proofs, authorize the account through the existing SPV-maintainer allow-list process.
Do not grant signing-operator stake or unrelated governance authority.

Start from the final Sepolia operator configuration so contract addresses remain correct.
Set these sections for the independent keeper:

```toml
[Maintainer.Spv]
Enabled = true
ReservationProofsEnabled = true
HistoryDepth = 50400
TransactionLimit = 20
RestartBackoffTime = "30m"
IdleBackoffTime = "10m"
MaxProofHeaders = 144

[ClientInfo]
Port = 9601
```

Start the process with external environment values:

```sh
: "${KEEPER_CONFIG:?}"
: "${KEEPER_KEY_FILE:?}"
: "${KEEP_ETHEREUM_PASSWORD:?}"
: "${SEPOLIA_WS_URL:?}"
: "${TESTNET4_ELECTRUM_URL:?}"

keep-client maintainer --testnet --spv \
  --config "$KEEPER_CONFIG" \
  --ethereum.url "$SEPOLIA_WS_URL" \
  --ethereum.keyFile "$KEEPER_KEY_FILE" \
  --bitcoin.electrum.url "$TESTNET4_ELECTRUM_URL" \
  --clientInfo.port 9601
```

The explicit `--spv` avoids launching the Bitcoin difficulty module.
The generic SPV proof loops cannot be disabled individually.
`Tbtc.ReservationsEnabled` is not required for keeper gauge registration.
Signing clients still require that separate flag in their existing configuration.

Use the released contract defaults or verified configuration overrides.
Validate the staking, beacon, registry, relay and maintainer-proxy addresses at startup.
The reservation router binding targets the Bridge fallback, not the standalone router address.
The keeper reads the vault address from Bridge parameters at runtime.

Keep the client-info HTTP endpoint private.
Allow access only from the monitoring network and operator administration path.
Do not expose `/metrics` or `/diagnostics` through a public service.

## 3. Private SDK4 monitoring build

Use the private monitor build from the same rehearsal-source SDK4 tree.
The installed public SDK used before this change cannot serve the Testnet4 configuration.
The published latest and development versions inspected during acceptance also lack Testnet4 support.

The build order is SDK, then monitor dependency installation, then monitor compilation.
Use the SDK's pinned Yarn 4.12.0 and the monitor's pinned Yarn 1.22.22 with their respective lockfiles.
The monitoring package consumes the local `../typescript` SDK package.
Build the primary Dockerfile from the repository root:

```sh
: "${MONITOR_IMAGE_TAG:?}"
docker build -f monitoring/Dockerfile -t "$MONITOR_IMAGE_TAG" .
```

Do not publish the SDK to npm as part of this procedure.
Do not substitute the old registry SDK after the private build.
Record the source commit and the produced image digest in the drill log.

Verified source: tbtc-v2 branch the Sepolia rehearsal branch on the private remote, head `52401a693`.
On the host it builds, passes 51 of 51 monitor tests, and completes a manager run against Testnet4 and read-only Sepolia with report `success`.
The readiness plan records an earlier Docker image pass; the image was not rebuilt here.
Use `52401a693` or a descendant (for example an earlier Sepolia rehearsal branch @ `ec50d8e3e`, which contains it), not the older `be2e89ddd`: that older monitor fails on Sepolia because the tBTC total supply is 0.

## 4. Monitor environment and first run

The private monitor maps `ENVIRONMENT=testnet` to Sepolia and Bitcoin Testnet4.
Use a real Testnet4 Electrum endpoint, not Testnet3 or regtest.
Use the same Sepolia chain as the keeper.

| Variable | Required value or purpose |
|---|---|
| `ENVIRONMENT` | `testnet` |
| `ETHEREUM_URL` | Human-provided Sepolia RPC endpoint |
| `ELECTRUM_URL` | Human-provided Testnet4 Electrum endpoint |
| `RESERVATION_KEEPER_METRICS_URL` | Private keeper `/metrics` endpoint |
| `SENTRY_DSN` | Existing Sentry project with the approved incident route |
| `DATA_DIR_PATH` | Persistent directory owned by the single monitor writer |
| `RESERVATION_KEEPER_OBSERVATION_MAX_AGE_SECONDS` | `1800` |

Sentry handles all four keeper alerts.
Discord handles only Informational events and cannot replace Sentry for these alerts.
Do not copy endpoint values, DSNs or keys into the drill log.

Before enabling the incident route:

1. Confirm the Ethereum chain ID is 11155111.
2. Confirm that the SDK identifies the Electrum network as Testnet4.
3. Confirm that the keeper endpoint exports all five `performance_reservation_*` samples.
4. Require a fresh `performance_reservation_keeper_observation_timestamp` value.
5. Run the actual monitoring entry point once.
6. Require its manager report to say `success` with no errors.
7. Confirm the checkpoint advances and the Sentry outbox is empty.
8. Confirm receiver delivery through the existing Sentry project.

A listening metrics port does not prove that an observation pass succeeded.
A zero process exit code does not prove manager success.
The acceptance driver checked the actual report and persistence state for this reason.

## 5. Scheduling and persistent state

Reuse keep-core's existing monitoring CronJob convention:
`infrastructure/kube/templates/tbtc-v2-monitoring/tbtc-v2-monitoring-cronjob.yaml`.
That template defines a persistent volume, `concurrencyPolicy: Forbid`, a 240-second deadline and no job retries.

For the drill deployment:

- Use the verified private image digest instead of `latest`.
- Set the schedule to `* * * * *`.
- Retain `concurrencyPolicy: Forbid` and one persistent-data writer.
- Set the environment to `testnet`.
- Point the Electrum secret at Testnet4.
- Add `RESERVATION_KEEPER_METRICS_URL` through the existing private configuration mechanism.
- Set `RESERVATION_KEEPER_OBSERVATION_MAX_AGE_SECONDS` to `1800`.
- Retain the Sentry secret reference and persistent data volume.

Do not run a second CronJob or manual process against the same data directory.
Do not delete `db.json` to reset a failed scan or undelivered alert.
Preserve the pending range and outbox so the next run can retry.
Back up the persistent directory before maintenance.

## 6. Observation delay and response

The keeper polls once per minute.
Event discovery waits for 12 Ethereum confirmations.
The first pass also discovers wallets registered before reservation activation.
It scans registration history in bounded chunks before publishing a successful observation.
Wallet state refreshes use a ten-minute interval.
Allow that interval, the next observation and the monitor schedule before expecting a retirement alert.

If a required read fails, the observer retains its last values and timestamp.
Treat an unavailable endpoint, malformed samples or stale observation as a keeper health incident.
Do not treat an initial zero count as healthy before the first complete pass.

| Alert | Level | Operator response |
|---|---|---|
| Late acceptance proof at risk | Critical | Contact SPV maintainers before the late-settlement edge; escalate unresolved proof delivery to governance. |
| Re-anchor has no eligible target | Critical | Check wallet state, target age, anchor floor and signing window. Restore usable capacity or parameters through the approved governance process. |
| Vault fee debt | Warning | Notify treasury and governance. Decide on repayment; the keeper does not repay automatically. |
| Unrequested deposit deadline at risk | Warning | Notify the depositor before the request window closes. Explain the permitted release or refund path. |
| Keeper metrics stale | Critical | Check the keeper process, chain access and observation logs. Restore valid observations before trusting risk counts. |

## 7. Evidence and remaining live actions

`keeper-alert-acceptance.md` records the mainnet-fork alert proof and its limits.
The fixture used a difficulty-0 relay and did not prove Bitcoin mainnet proof validity.
The run did not deploy a keeper, monitor or Sentry incident route on live Sepolia infrastructure.

The human still owns the O6 deployer actions, package publication, operator rollout and final activation.
The deployment operator must provide the private endpoint access, persistent volume and existing Sentry route.
These are real deployment prerequisites, not waived acceptance checks.
