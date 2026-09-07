# Maintainer monitoring

These rules cover the standalone SPV maintainer's availability, Ethereum and
Bitcoin RPC health, proof processing, and relay difficulty constraints. They
require the maintainer telemetry added by the #4187 / #4190 / #4290 stack. They
complement the tBTC event monitor's redemption request and timeout notifications.

## Rollout

1. Build and publish a client image containing the telemetry stack. Set the
   maintainer overlay's image tag to that release. The production overlay still
   pins `v2.1.0`; this configuration change does not publish or select a new image.
2. Render the applicable overlay, review it, and apply it through the usual
   deployment process. The upgraded client defaults to client-info port 9601;
   the template exposes `/metrics` through the internal `keep-maintainer-metrics`
   ClusterIP service. No new CLI flag is passed, preserving compatibility with
   older images until the image rollout. Metrics require the upgraded image.
   Production rendering requires the existing local `.secret` files. The template
   alone can be rendered without credentials:
   `kubectl kustomize infrastructure/kube/templates/keep-maintainer`.
3. Mount `rules.yml` into the cluster's existing Prometheus and merge the
   `rule_files` and `scrape_configs` sections from `prometheus.example.yml` into its
   configuration. Adjust the service namespace and environment label. The example
   target is for the existing single-replica deployment; for multiple replicas,
   discover individual pod endpoints so one failed replica cannot hide behind
   the Service. Restrict port 9601 to the monitoring network.
4. Verify the target is UP and all health/timestamp series in
   `KeepMaintainerTelemetryMissing` are present. This catches a scrape endpoint
   running an older image. Use a separate job for a Bitcoin-difficulty-only
   maintainer: these rules expect SPV processing to be enabled.
5. Configure Prometheus's `alerting.alertmanagers` with the actual Alertmanager
   endpoint. Route `component="keep-maintainer"` to the existing on-call receiver
   and preserve `environment` and `instance` in notifications. The example leaves
   this endpoint unset because it is installation-specific. Validate routing with
   a temporary test receiver before enabling paging. If using Sentry, use an
   existing Sentry integration that translates Alertmanager notifications; a
   Sentry DSN is not an Alertmanager webhook endpoint.
6. Confirm a notification and its resolution arrive through the intended route
   in staging, then remove the test alert. Verify RPC outage, process outage, and
   recovery using controlled staging interruptions. Keep #3664 open until this
   delivery check and the remaining redemption-event rollout are complete.

## Semantics and tuning

- RPC health is the last completed probe's result. Separate freshness rules catch
  hung probes even if the last result was healthy. The rules allow three probe
  intervals plus one minute for metric sampling and five minutes before firing.
- SPV activity records task starts and completions. The stalled rule allows the
  configured maximum idle/restart backoff plus 30 minutes for work, then requires
  another ten minutes without progress. Tune the processing allowance against
  observed proof durations; an idle maintainer need not submit proofs regularly.
- Task failure state persists until a later successful complete cycle, including
  failures before the first scrape. A successful cycle may find no pending
  transactions; it does not imply a redemption was proven. Timestamps have
  one-second resolution: a failure and recovery in the same second remain
  conservative until the next successful cycle.
- Relay-range and header-limit counters count attempts, including retries of the
  same Bitcoin transaction. They are distinct skip reasons, not task failures or
  counts of unique stuck redemptions. Their alerts require an observed counter
  increase and can miss a single skip before the first scrape; inspect absolute
  counter values during rollout. A recurring skip triggers on subsequent attempts.
- These rules do not prove end-to-end alert delivery or diagnose a particular
  redemption. Correlate maintainer logs, redemption request deadlines, relay
  updates, and accepted `RedemptionsCompleted` events before intervening.

## Validation and rollback

From this directory, with Prometheus `promtool` installed:

```sh
promtool check config prometheus.example.yml
promtool test rules rules.test.yml
```

Tests cover healthy operation, unavailable/missing telemetry, RPC failures and
hung probes, configured long backoffs, first-scrape task failures and recovery,
and both difficulty skip reasons. Remove the scrape job and rule file to roll
back monitoring. On the upgraded client, use `--clientInfo.port=0` to disable the
endpoint, and remove the metrics Service and container port declaration.
