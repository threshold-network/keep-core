# ROAST Phase 5 Rollout Runbook

Date: 2026-03-01
Status: Draft (awaiting baseline calibration)
Owner: Threshold Labs
Scope: staged ROAST rollout operations, monitoring, hold/rollback actions

## 1. Objective

Provide the operator procedure for staged ROAST rollout with explicit gate
checks, incident actions, and evidence capture requirements.

This runbook is paired with:

- `pkg/tbtc/signer/docs/roast-phase-5-security-rollout-gates.md`
- Future mandatory TEE hardening profile
  (activation-gated):
  `pkg/tbtc/signer/docs/tee-whitelisted-signer-enforcement-plan.md`

## 2. Prerequisites

Before Stage 1 canary:

1. Security/correctness gate checks are green.
2. Fresh interactive latency-window evidence is available for the stage being
   promoted, including the required sample counts and p95 values. The retired
   coarse-path `phase5_roast` benchmark is not a rollout gate.
3. Chaos/failure suite is green:
   - `cd pkg/tbtc/signer && ./scripts/run_phase5_chaos_suite.sh`
4. Pre-ROAST baseline window captured for:
   - attempt success rate
   - coordinator rotations per signing request
   - p95/p99 signing latency
5. Baseline worksheet populated:
   - `pkg/tbtc/signer/docs/roast-phase-5-baseline-calibration.md`
6. Provenance attestation rotation cadence scheduled: a production
   signer installs its configuration once at process start (the
   init-time config FFI, `frost_tbtc_init_signer_config`) and the
   attestation material in it is immutable for the process lifetime,
   while attestation TTL is capped at 7 days
   (`TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS`). Operators
   MUST restart (re-init) each signer with fresh attestation material
   within every attestation window, and rollout stage scheduling must
   absorb that restart cadence. Live re-attestation without a restart
   is deliberately unsupported: it would require a dedicated,
   narrowly-scoped FFI, never general config mutation, which would
   reopen the split-brain risk the immutable install design closed.
7. Config-file pushes are canaried node-by-node: an unmet init-config
   demand (`TBTC_SIGNER_INIT_CONFIG_PATH` set but the FROST-native
   engine did not come up) terminates the process in every profile
   (gates-doc Decision Log, decision 7). A bad config template pushed
   fleet-wide therefore produces a visible, correlated outage instead
   of silent capability loss - push to a single node, confirm a clean
   start, then roll out. The same applies to signer-library upgrades
   that tighten init-time validation: a config that installed
   yesterday can be rejected after an upgrade, so upgrade + config
   changes are canaried together. Note this also enforces prerequisite
   6's attestation cadence: a node restarted with expired attestation
   material will not start until re-attested. Scope the variable to
   the signer service unit (e.g. the systemd unit's `Environment=`),
   never the host-global environment: every binary importing the
   signing package honors the same demand, so a host-global export
   plus a broken config would also kill maintenance tooling and test
   binaries run on that host.

## 3. Rollout Stages

1. Stage 1 (Canary):
   - scope: 5% signer fleet, limited wallet cohort
   - hold: 24 hours minimum
2. Stage 2 (Expanded):
   - scope: 25% signer fleet, broader cohort
   - hold: 24 hours minimum
3. Stage 3 (General Availability):
   - scope: 100% signer fleet
   - start only if Stage 1 and Stage 2 remained within thresholds

## 4. Monitoring And Decision Thresholds

Use the thresholds from
`pkg/tbtc/signer/docs/roast-phase-5-security-rollout-gates.md`.

Hold thresholds:

1. success rate `< 99.0%` over rolling 6 hours
2. coordinator rotations/request `> 0.35` over rolling 6 hours
3. p95 latency delta `> +25%` for 1 hour
4. terminal failures `> 0.5%` over 1 hour

Rollback thresholds:

1. success rate `< 97.0%` over rolling 1 hour
2. coordinator rotations/request `> 0.60` over rolling 1 hour
3. p99 latency delta `> +40%` for 30 minutes
4. terminal failures `> 1.0%` over 30 minutes

## 5. Immediate No-Go Triggers

Pause rollout immediately and open incident response if any are observed:

1. unauthorized attempt advancement acceptance
2. consumed attempt/round replay-protection regression
3. restart inconsistency with divergent transition decisions
4. missing transition/recovery telemetry needed for operator triage

## 6. Incident Response Steps

1. Freeze progression to the next stage.
2. Record trigger metric(s), start/end time, and affected scope.
3. Capture logs/events for:
   - attempt transition reason
   - coordinator rotation counts
   - excluded participant evidence
4. Classify outcome:
   - `hold` (within hold-only threshold breach)
   - `rollback` (rollback threshold/no-go breach)
5. If rollback is required:
   - disable ROAST rollout flag for current scope
   - return traffic to previous stable config
   - verify metric recovery in the next 30-60 minutes

## 7. Evidence Capture Per Stage

For each stage, archive:

1. start/end timestamps (UTC) and signer/wallet scope
2. metric snapshots for success, rotations, p95/p99 latency, terminal failures
3. count of recovery-class events by reason
4. incident tickets opened and closure status
5. decision record: `proceed`, `hold`, or `rollback`

## 8. Exit Criteria

Rollout is complete when:

1. Stage 1 and Stage 2 hold windows complete without rollback/no-go triggers.
2. Stage 3 reaches steady-state without threshold breach.
3. Required security, signer, runtime, and governance approvals are recorded in
   the release or governance record.

## 9. Post-Activation Cleanup

Once the production activation packet is approved and Stage 3 has reached
steady-state, the readiness-gate machinery has served its purpose and is
removed from the tree. In a dedicated cleanup PR, delete:

1. `scripts/formal/check_frost_activation_gate.mjs`,
   `check_frost_funded_live_run_gate.mjs`,
   `check_frost_operator_dry_run_gate.mjs`,
   `check_frost_production_indexing_gate.mjs`,
   `check_frost_release_artifact_gate.mjs`, and
   `check_p2tr_fraud_gas_dos_gate.mjs`, plus any helpers in
   `scripts/formal/` that become orphaned.
2. The matching evidence manifests and runbooks under `docs/operations/`
   (`frost-roast-*-evidence-v0.json` and the associated
   `frost-roast-*-runbook-*.md` / packet template files).
3. The `readiness:gates:check` script entry in the root `package.json` and
   the chained call from `formal:vectors:check`.

The signed activation packet and merged code are the durable record after
activation; the gate scripts only exist to keep the pre-activation door
closed and should not outlive that role.
