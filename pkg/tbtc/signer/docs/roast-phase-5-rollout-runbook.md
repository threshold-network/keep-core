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


## 10. Operator SOP — Staged Config Rollout Under Process-Fatal Init-Config Demand

Date added: 2026-08-18
Scope: GitHub issue #4252. Decision Log entry 7 in
`pkg/tbtc/signer/docs/roast-phase-5-security-rollout-gates.md` (init-config
demand is process-fatal in every profile) makes any state where the FROST-native
engine does not come up under `TBTC_SIGNER_INIT_CONFIG_PATH` terminate the host
process. Operators accept fleet-wide-DoS-on-bad-config-push risk as the cost of
the strongest posture; this SOP is the operational mitigation. The fatal exit
itself is upstream of this crate (the Go host reads the env-var, posts the
install request via `frost_tbtc_init_signer_config` in
`pkg/tbtc/signer/src/lib.rs:77-87`, and kills the process if the install
fails — see `pkg/tbtc/signer/docs/phase-7-sidecar-transport-addendum.md` for
the sidecar variant of the same demand). The Rust engine never calls
`std::process::exit`; it surfaces typed `EngineError` variants (see
`pkg/tbtc/signer/src/errors.rs` and the FFI mapping in
`pkg/tbtc/signer/src/ffi.rs:162-178`) so the host can decide the abort.

### 10.1 Staged Rollout Procedure

The existing Section 3 stages are correct in principle (5% canary -> 25%
expanded -> 100% GA), but operators need a concrete promotion gate that
survives a fleet-wide process-fatal mistake. Apply this procedure verbatim
on every config change — not just on the rollout-gated ones — because a
binary upgrade that tightens init-time validation can turn yesterday's
good config into today's bad one (Section 2 prerequisite 7 explicitly
notes this coupling between signer-library upgrades and config changes).

1. **Pre-flight (operator-only, no fleet action yet).** Run the validation
   pass described in Section 10.3 below against the candidate config file.
   Capture the validation output as part of the change ticket; do not skip
   this step even for "trivial" knob changes.
2. **Canary cohort.** Deploy to exactly **1 node** (never 0, never more
   than 1 for the first wave) and observe for **at least 30 minutes** of
   uptime plus one full attempt of every protected operation the config
   affects (DKG for `admission_*` knobs, `BuildTaprootTx` for firewall
   knobs, `InteractiveSessionOpen/Round1/Round2/Aggregate` for any
   `interactive_*` knob, provenance-gated operations for the provenance
   knobs). One node is the minimum that exercises the install FFI plus
   every gated operation against the live state file; the gates-doc Gate
   3 progressive-rollout percentages are too coarse for a fatal-install
   environment where every additional canary is a fresh chance for a
   correlated outage.
3. **Hold + sample window.** Keep the canary on the new config for the
   longer of:
   - `TBTC_SIGNER_CANARY_MAX_SAMPLE_AGE_SECONDS` (default 3600s, hard cap
     604800s; see `pkg/tbtc/signer/src/engine/config.rs:211-226`), AND
   - the existing rollout `Hold` thresholds from Section 4 (success
     rate `>= 99.0%` over rolling 6h, p95 latency delta `<= +25%` for
     1h, terminal failures `<= 0.5%` over 1h).
   Sample age is bounded by the env var because a stale evidence window
   blocks `PromoteCanary` regardless of the live metrics (see README
   "Canary promotion runbook" and `frost_tbtc_canary_rollout_status` in
   `pkg/tbtc/signer/src/lib.rs:172-179`). Restarting the canary resets
   the evidence epoch and blocks promotion until the window rebuilds.
4. **Stage 2 — Expanded cohort.** Roll to **25% of the fleet**, picking
   nodes in at least two distinct geographic/operator jurisdictions so
   a single-region outage is not reproduced at fleet scale. Hold for
   the same canary sample-age window (or longer, never shorter).
5. **Stage 3 — General availability.** Roll to **100%** using the
   rollout tool's standard procedure once Stages 1 and 2 stay within
   the Section 4 thresholds for the full hold window. Record the GA
   decision in the change ticket per Section 7.
6. **Promotion criteria (call into `PromoteCanary`).** Only call
   `frost_tbtc_promote_canary` (declared in
   `pkg/tbtc/signer/src/lib.rs` extern block, used via the Go bridge)
   after both:
   - `frost_tbtc_canary_rollout_status` returns a
     `CanaryRolloutStatusResult`
     (`pkg/tbtc/signer/src/api.rs:909-919`) where
     `promotion_gate_passed == true` AND `gate_failures` is empty AND
     `now() - last_action_unix >= 600s` (operator-side dwell: at
     least 600s of evidence must have accumulated since the canary
     last took a rollout action, since the struct exposes the last
     action timestamp but no per-evidence-window timestamps), AND
   - `last_updated_unix` from `frost_tbtc_hardening_metrics`
     (`pkg/tbtc/signer/src/engine/telemetry.rs:332-500`,
     `SignerHardeningMetricsResult`) is within the canary
     `TBTC_SIGNER_CANARY_MAX_SAMPLE_AGE_SECONDS` of now.
   Evidence resets after every promotion/rollback, and a restart blocks
   promotion until the current stage rebuilds its window — budget for
   this in the hold time.

The exact canary/expanded/GA percentages may differ by environment, but
**never** start at more than one node and never skip the per-stage hold.
The process-fatal posture means a bad push at 5% is a five-percent
outage, at 50% is a majority outage, and at 100% is a fleet-wide
halt — the staging discipline is what bounds the worst case to a
single-node restart.

### 10.2 Manual Rollback Gate (Detect, Roll Back, Recover)

The fastest way to recover from a bad config push is to revert the
config file and restart every signer whose `TbtcSignerResult.status_code`
on `frost_tbtc_init_signer_config` was non-zero. Detection and recovery
are both trivial when one node is down and painful when fifty are down,
which is the entire reason this SOP exists.

**Detection signals to wire to paging (in priority order):**

1. **Process exit / restart loop on a config-mode fleet member.** With
   `TBTC_SIGNER_INIT_CONFIG_PATH` set, a host that cannot install the
   config exits and the supervisor restarts it. Look for the systemd
   unit cycling in `systemctl status` or the container's
   `CrashLoopBackOff` status in Kubernetes. **Any single config-mode
   node that has restarted more than 3 times in 10 minutes after a
   config push is a page.** The Rust engine itself does not call
   `std::process::exit` (no `process::exit` in
   `pkg/tbtc/signer/src/`); the upstream Go host does on a failed
   install (the contract documented in
   `pkg/tbtc/signer/docs/roast-phase-5-security-rollout-gates.md`
   Decision Log entry 7).
2. **FFI error response on the install call.** Hosts log the
   `TbtcSignerResult.status_code = 1` return and the `ErrorResponse`
   JSON (`{"code":"...","message":"...","recovery_class":"..."}`
   per `pkg/tbtc/signer/src/api.rs:1029-1046`). For init-config
   installs the failing validation function is one of the seven calls
   in `pkg/tbtc/signer/src/engine/init_config.rs:175-195`
   (`validate_candidate_config`), and the resulting error message
   contains the failing field name and value (see tests
   `init_signer_config_rejects_zero_heartbeat_rate_limit_without_installing`,
   `init_signer_config_rejects_invalid_profile_without_installing`,
   `init_signer_config_rolls_back_install_when_policy_validation_fails`
   in `pkg/tbtc/signer/src/engine/tests.rs` for representative
   messages). Grep host logs for the install call site plus
   `status_code=1`.
3. **Policy-decision log spike.** Every gate reject emits a
   `policy_decision stage=<stage> session_id=<id> decision=reject
   reason_code=<code>` line via `log_policy_decision` in
   `pkg/tbtc/signer/src/engine/policy.rs:173-188`. If the same
   `stage`/`reason_code` pair appears on multiple nodes within
   seconds of a config push, the push is the cause. The first
   stages to check are `admission_policy`, `signing_policy_firewall`,
   `auto_quarantine`, and `lifecycle_policy`.
4. **Hardening-metrics counters flatlining.** After the canary window,
   `SignerHardeningMetricsResult`
   (`pkg/tbtc/signer/src/engine/telemetry.rs:332-500`) should show
   non-zero `*_calls_total` counters for the operations the config
   enables (e.g. `start_sign_round_calls_total`,
   `build_taproot_tx_calls_total`,
   `interactive_session_open_calls_total`). A node where every
   operational counter is stuck at 0 for > 15 minutes after the hold
   window elapsed has not installed or has degraded silently.
5. **One-shot env-fallback warning.** The line
   `warning: TBTC_SIGNER_* knobs are being read from the process environment; production hosts should install an init-time config via frost_tbtc_init_signer_config`
   (emitted once per process by `warn_production_env_fallback_once`
   in `pkg/tbtc/signer/src/engine/init_config.rs:86-104`) appears when
   the install was skipped; it is normally expected on the
   non-config-mode fallback path and is not by itself a signal.

**Rollback steps (single-node scenario first, then fleet):**

1. Identify the previous known-good config file. Config files MUST be
   versioned in the config repo with a content-addressable name (e.g.
   git SHA prefix) so the rollback is `cp <last-good-sha>.json
   <TBTC_SIGNER_INIT_CONFIG_PATH>` followed by
   `systemctl restart tbtc-signer`. Do not edit the bad file in place
   — a half-written file is the worst case because the next restart
   picks up a parse error.
2. On the single canary, restart and confirm the install succeeds by
   checking the host log for the install success path (the install
   FFI returns `status_code = 0` and an `InitSignerConfigResult` with
   `installed: true`, `idempotent: false`,
   `config_fingerprint: <sha>` per
   `pkg/tbtc/signer/src/api.rs:1165-1166`). The fingerprint is the
   audit anchor for the rollback — record it in the ticket.
3. **Only after** the canary is healthy, repeat on the rest of the
   fleet. A bad-push incident becomes a full-fleet outage if operators
   race to roll back every node before the canary is confirmed.
4. While rollback is in progress, pause canary promotion: do not call
   `frost_tbtc_promote_canary` or `frost_tbtc_rollback_canary`
   (`pkg/tbtc/signer/src/lib.rs:191-201`); both reset the evidence
   window and `RollbackCanary` would clobber the manual rollback.
5. Update the SLO dashboards with the rollback fingerprint and the
   timestamps of the bad/good install attempts; the Section 7 evidence
   capture applies to rollbacks too.

**Expected recovery time:** with the staged procedure above, a
single-node bad push is detected within the supervisor's first
restart cycle (typically < 60s) and recovered with a `cp` plus
`systemctl restart` (typically < 30s end-to-end). A fleet-wide
bad push recovers in `ceil(N / restart_parallelism) * 30s` for `N`
affected nodes; e.g. 100 nodes at 8 parallel restarts = ~13 minutes,
which is the upper bound the staged rollout is designed to avoid.

### 10.3 Config Validation Pre-Flight

Before pushing a candidate config file to any signer, run the same
validation the engine runs at install, but in a host-only dry-run
mode so a typo or missing knob never reaches a fleet. The install
validation lives in
`pkg/tbtc/signer/src/engine/init_config.rs:175-195`
(`validate_candidate_config`) and exercises seven loaders, in this
exact order — every one of them must succeed for the install to
publish the config:

1. `load_admission_policy_config`
   (`pkg/tbtc/signer/src/engine/policy.rs:129-157`). Checked only
   when `TBTC_SIGNER_ENFORCE_ADMISSION_POLICY=true`. The host must
   also set `TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS` (default 2),
   `TBTC_SIGNER_ADMISSION_MIN_THRESHOLD` (default 2, must fit `u16`),
   and either set or omit
   `TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS` (unset disables the
   check; **empty string is invalid**). Set
   `TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS` as a comma-separated
   u16 list.
2. `load_signing_policy_firewall_config`
   (`pkg/tbtc/signer/src/engine/policy.rs:286-370`). Checked only
   when `TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL=true`. Every
   script class must be one of the conservative default set
   (`p2pkh,p2sh,p2wpkh,p2wsh,p2tr`); any other class fails closed.
   `TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR` and
   `..._END_HOUR` must not be equal (equal bounds are rejected;
   omit both for a 24-hour window).
3. `heartbeat_rate_limit_per_minute`
   (`pkg/tbtc/signer/src/engine/policy.rs:372-389`). Must be a
   positive integer; `0` is rejected by the test
   `init_signer_config_rejects_zero_heartbeat_rate_limit_without_installing`.
4. `load_auto_quarantine_config`
   (`pkg/tbtc/signer/src/engine/policy.rs:393-...`). Checked only
   when `TBTC_SIGNER_ENABLE_AUTO_QUARANTINE=true`. Fault threshold,
   timeout penalty, and invalid-share penalty must be sane
   (`TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_FAULT_THRESHOLD = 3`).
5. `state_file_path`
   (`pkg/tbtc/signer/src/engine/state.rs:402-...`). The path must
   resolve to a writable location; production profiles reject the
   implicit temp-dir state path. Verify the parent directory
   exists, the signer process can write to it, and
   `TBTC_SIGNER_STATE_PATH` is set on every production config.
6. `resolve_state_key_provider_plan`
   (`pkg/tbtc/signer/src/engine/persistence.rs:1046-...`). Production
   rejects the `env` provider (`TBTC_SIGNER_STATE_KEY_PROVIDER=env`)
   fail-closed; production configs MUST set
   `state_key_provider: "command"` plus `state_key_command` (the
   command string is itself part of the config request — do not
   inline the key into it). The provider plan is resolved without
   reading the secret or executing the command, so a missing or
   malformed command string fails the init even though no secret is
   touched.
7. `enforce_provenance_gate`
   (`pkg/tbtc/signer/src/engine/provenance.rs:161-...`). Production
   forces this on. The complete attestation set is required at install:
   `TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS` (must equal `approved`),
   `TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD` (signed JSON containing
   `status`, `runtime_version`, and required `expires_at_unix`),
   `TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX` (Schnorr
   signature over `sha256(payload_bytes)`),
   `TBTC_SIGNER_PROVENANCE_TRUST_ROOT` (32-byte x-only secp256k1
   public key, 64 hex chars), and
   `TBTC_SIGNER_MIN_APPROVED_VERSION`. The init-time pass does not
   exempt runtime re-checks — TTL aging (max 7 days, see
   `TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS` in
   `pkg/tbtc/signer/src/engine/config.rs:106`) still applies per call.
   A config with an attestation set that expires inside the rollout
   window will install cleanly but the first protected operation
   after expiry will reject.

**Host-side pre-flight recipe** (operators MUST run this before
pushing a candidate config):

1. Static: pipe the candidate JSON through `jq` to confirm every key
   is one of the `InitSignerConfigRequest` fields in
   `pkg/tbtc/signer/src/api.rs:1060-1163`. The Rust struct uses
   `serde(deny_unknown_fields)` (`pkg/tbtc/signer/src/api.rs:1059`),
   so a typo'd field is rejected at install — but operators want to
   know before the install that the field would be rejected, because
   every install failure on a config-mode fleet is a process exit.
2. Profile cross-check: if `profile != "development"` (case-insensitive
   trimmed; `signer_profile_is_production` in
   `pkg/tbtc/signer/src/engine/config.rs:424-451` treats unset,
   unknown, and `production` all as production-by-default), every
   production-required knob above MUST be set. The engine warns once
   per process for unrecognized profile values via
   `PROFILE_VALUE_WARNING_EMITTED`
   (`pkg/tbtc/signer/src/engine/config.rs:422-451`) and treats the
   value as production.
3. Local install dry-run on a non-production host with the exact same
   binary build (`cargo build --release --locked` in
   `pkg/tbtc/signer/`). Run `frost_tbtc_init_signer_config` with the
   candidate request and confirm `status_code == 0` and the response
   includes `installed: true`. If the dry-run host cannot be a
   production binary, run the unit tests
   `pkg/tbtc/signer/src/engine/tests.rs:6514-6744` against the
   candidate (the tests in the `init_signer_config_*` family are the
   behavioral ground truth for the validators above).
4. Attestation TTL check: confirm
   `expires_at_unix - now() < TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS`
   (7 days) **and** that the rollout window plus a 24-hour buffer
   fits inside that TTL. If not, re-attest before pushing.
5. Diff the candidate against the last known-good config (git
   `diff` is fine) and review with at least one other operator. The
   staged rollout only bounds blast radius; it does not catch a
   "we both missed it" review error.

### 10.4 Auto-Rollout Tool Integration (Ansible/Kubernetes/Terraform)

The rollout tool itself — whichever orchestrator the operator uses
to push the config file — is the load-bearing piece of this SOP. A
tool that ships a known-bad config to the entire fleet in parallel
converts a routine config change into a fleet-wide outage. The
following guards are REQUIRED on every rollout tool that touches
`TBTC_SIGNER_INIT_CONFIG_PATH` on a config-mode fleet member; they
mirror the principles in Section 2 prerequisite 7 ("config-file
pushes are canaried node-by-node").

1. **Canary-then-wait health gate.** No rollout tool may push to more
   than one node at a time without an explicit per-node health gate.
   The gate must wait for:
   - The target node's supervisor reports the signer process `active`
     (systemd `ActiveState=active` or container `Ready=True`) for
     **at least 60 seconds**.
   - The signer process has logged the install success path (no
     `policy_decision decision=reject` lines from any of the seven
     `validate_candidate_config` loaders, no restart loop, no
     `status_code=1` FFI response on
     `frost_tbtc_init_signer_config`).
   - One sample of the operation the config affects has succeeded
     (e.g. one `BuildTaprootTx` for firewall changes, one
     `InteractiveRound2` for interactive knobs). Use the
     `*_calls_total` and `*_success_total` fields in
     `SignerHardeningMetricsResult` to verify, NOT just liveness.
2. **Rolling-update parallelism = 1.** Equivalent of Kubernetes
   `maxUnavailable: 1` and `maxSurge: 0` for the signer service;
   for Ansible `serial: 1` with `throttle: 1`; for Terraform a
   `create_before_destroy = false` with explicit per-instance
   dependencies. Never batch. A bad push at `serial: 1` is a
   single-node outage; a bad push at `serial: 50%` is a majority
   outage. The cluster can absorb the former, not the latter.
3. **Pause-on-restart-loop.** If a target node has restarted the
   signer process more than `3` times in `10` minutes (a
   conservative CrashLoopBackOff threshold), the rollout tool MUST
   halt the entire rollout, mark the change as failed, and page the
   operator. Do not skip the bad node and continue — the next node
   will hit the same config failure.
4. **Versioned config file names.** The tool MUST push a
   content-addressable file (e.g.
   `tbtc-signer-config-<sha256>.json`) and atomic-rename to
   `TBTC_SIGNER_INIT_CONFIG_PATH` so partial writes never reach the
   signer. The atomic rename MUST be on the same filesystem as the
   signer reads from, otherwise the rename is not atomic and a crash
   between write and rename produces a parse-error restart loop.
5. **No host-global env export.** `TBTC_SIGNER_INIT_CONFIG_PATH`
   (and every other `TBTC_SIGNER_*` knob) MUST be scoped to the
   signer service unit (systemd `Environment=` line, Kubernetes
   container `env:` entry for the signer container only). A
   host-global export — `/etc/environment.d/tbtc-signer.sh` on the
   whole VM, for example — would also kill maintenance tooling and
   test binaries that import the signing package (Section 2
   prerequisite 7 explicitly calls this out). The rollout tool
   should write the env into the service definition, not the host
   shell.
6. **Pre-flight hook integration.** The tool MUST expose a dry-run
   hook that runs the Section 10.3 pre-flight against the candidate
   config in CI before any push, and the tool MUST refuse to start
   a rollout if the pre-flight fails. This is the cheapest place to
   catch a typo; every push past the pre-flight hook is a coin flip
   between "good config" and "fleet outage."
7. **Rollback automation.** The same tool MUST expose a `rollback`
   verb that re-pushes the previous known-good config file and
   restarts the signer. The rollback path is the same artifact as
   the forward path (just an older SHA), so the tool should treat
   rollbacks as first-class, not as ad-hoc `ssh` and `cp`.
   Operators under incident pressure should not have to remember the
   rollback procedure; the tool should be one command.

These seven rules are not specific to one orchestrator. Map them to
the primitives of whatever tool is in use; the principles are what
matter. A team that ships a rollout tool without all seven is
running an unattended `scp` script and should expect the
process-fatal semantics to express themselves at scale.

### 10.5 On-Call Runbook — "Multiple Signer Nodes Just Died Simultaneously After a Config Push"

This is the script to follow when paged. It assumes the failure mode
has already been correlated to a config push by Section 10.2
detection signals; if the page reason is unclear, go to step 0 first.

**Step 0 — Confirm correlation (5 minutes max).** If the page did
not arrive because of a known config push, gather first:
- **Concrete discriminator (do this first, under 1 minute; needs ONE
  surviving signer host — prefer the canary or a node that is still
  serving signing traffic).** Call
  `frost_tbtc_canary_rollout_status` on that host and read
  `config_version` (a `u64`) from the returned
  `CanaryRolloutStatusResult`
  (`pkg/tbtc/signer/src/api.rs:909-919`). This is the same
  monotonic `u64` that `PromoteCanaryResult.config_version`
  (`pkg/tbtc/signer/src/api.rs:888`) and
  `RollbackCanaryResult.config_version`
  (`pkg/tbtc/signer/src/api.rs:902`) report and that the install
  path advances on every successful init-time config install. Compare
  the on-host value against the `config_version` recorded in the
  change ticket for the install that produced the currently-deployed
  config (the same value `InitSignerConfigResult` reports via
  `config_fingerprint` to anchor the install audit on). If
  `config_version` is UNCHANGED from the change-ticket value AND the
  rollout tool's audit log shows no push entry timestamped inside
  the incident window, then NO config push has reached the fleet
  via this signer — STOP, treat this as a regular outage, hand off
  to the regular on-call path, and do NOT run steps 1-6 (the
  staged-rollout SOP does not apply). If `config_version` HAS
  advanced (or the rollout tool's audit log does contain a push
  entry inside the window whose `config_version` matches what the
  FFI call now reports), the bad push is the lead suspect — continue
  with the correlation evidence below to confirm.
- Host logs from three dead nodes (systemd journal for the signer
  unit, or `kubectl logs` for the container).
- `journalctl -u tbtc-signer --since '15 minutes ago'` should show a
  `status_code=1` `ErrorResponse` from
  `frost_tbtc_init_signer_config`, or a
  `policy_decision stage=... decision=reject reason_code=...` line
  within seconds of the most recent restart. The failing
  `reason_code` identifies which of the seven
  `validate_candidate_config` loaders rejected.
- The candidate config SHA that was pushed (from the rollout tool's
  audit log, not from memory).

If the failures are NOT correlated to a config push (no common SHA,
no `policy_decision` cluster, multiple unrelated `reason_code`
values), stop and treat this as a regular outage, not a
bad-config incident — the staged-rollout SOP does not apply.

**Step 1 — Halt the rollout tool (30 seconds).** Disable any
further pushes from the orchestrator. In Ansible: pause the
playbook (`Ctrl-Z` or `--limit ""` reset). In Kubernetes:
`kubectl rollout pause deployment/tbtc-signer`. In Terraform:
`terraform apply -lock=false` after setting the count to current
and refusing further changes. The goal is to stop the bleeding
before diagnosis; the bad push may still be in flight.

**Step 2 — Identify the last known-good config (1-2 minutes).**
Pull the previous content-addressable config file from the config
repo (the same SHA the canary was running on, or the prior GA
config). Confirm it parses by feeding it through the Section 10.3
pre-flight locally. Do not skip the pre-flight on the rollback
target — a "known-good" config that has been edited in place by
someone else is not known-good.

**Step 3 — Roll back the canary (2-3 minutes).** Pick the
previously-confirmed-healthy node and push the last known-good
config to it via the rollout tool's rollback verb (Section 10.4
rule 7). Confirm it installs cleanly: `active` for 60 seconds, no
`policy_decision decision=reject` lines, one successful operation
sample from `SignerHardeningMetricsResult`. **Do not proceed past
this step until the canary is healthy.** A fleet-wide rollback
without a canary check is two failures deep.

**Step 4 — Roll back the rest of the fleet in `maxUnavailable=1`
increments (variable).** Use the same rollback verb. Monitor the
health gate at every increment; if any node fails the install,
STOP — the candidate config is not the only problem, or the
last-known-good is no longer good. Either way, halt and call in
another operator.

**Step 5 — Verify fleet health (10-30 minutes).** Confirm:
- All signer processes are `active` for at least one
  `TBTC_SIGNER_REFRESH_CADENCE_SECONDS` window (default 86400s,
  so this may need to run in the background; do not block the
  page on it — record the start time and check later).
- `frost_tbtc_hardening_metrics` (`SignerHardeningMetricsResult`)
  shows non-zero `*_calls_total` and `*_success_total` across the
  whole fleet for the operations the rolled-back config enables.
  Cross-check against the pre-incident baseline in
  `pkg/tbtc/signer/docs/roast-phase-5-baseline-calibration.md`.
- `frost_tbtc_canary_rollout_status` reports a clean cohort and
  no `promotion_gate_passed=false`. (Canary state was likely reset
  by `frost_tbtc_rollback_canary` or by the manual restart; that
  is expected.)
- Threshold budget from Section 4 is intact: success rate
  `>= 97.0%` over the rolling 1h (the rollback threshold), no
  rollback-trigger breaches.

**Step 6 — Postmortem (within 24h, not during the page).** Capture
per Section 7: the bad/good config SHAs, the install timestamps
per node, the `reason_code` of the original rejection, the
recovery time per node, and the gap between incident start and
the halt command. Two questions the postmortem MUST answer:
(a) why did the Section 10.3 pre-flight not catch this, and
(b) why did the rollout tool's Section 10.4 health gate not stop
at one node. A repeated incident without an answer to both is a
process failure, not a tooling failure.

**Quick-reference detection greps for the on-call:**
- `journalctl -u tbtc-signer --since '15 minutes ago' | grep -F 'status_code=1'` —
  catches any non-zero FFI return (the install path returns this
  on validation failure).
- `journalctl -u tbtc-signer --since '15 minutes ago' | grep -E 'policy_decision stage=.*decision=reject'` —
  catches any gate rejection across the seven
  `validate_candidate_config` loaders.
- `journalctl -u tbtc-signer --since '15 minutes ago' | grep -E 'init_signer_config'` —
  catches the install path itself (success and failure).
- `kubectl logs -l app=tbtc-signer --since=15m | grep -F 'frost_tbtc_init_signer_config'` —
  Kubernetes equivalent.
- For FFI call-site logs, the `ErrorResponse.code` values that
  matter most are `validation_error`, `provenance_gate_rejected`,
  `admission_policy_rejected`, and `signing_policy_rejected` (see
  the error code mapping in `pkg/tbtc/signer/src/errors.rs:4-...`
  and the FFI surface in `pkg/tbtc/signer/src/ffi.rs:162-178`). On
  init-time, only `validation_error` and `provenance_gate_rejected`
  can fire (the other gates run later, at operation time) — see
  `validate_candidate_config` for the exact list of init-time
  failures.
