# tbtc-signer (bootstrap)

This crate is the first implementation slice of the Rust rewrite plan tracked
in `docs/rust-rewrite-bootstrap.md`.

## Current scope

- Exposes a C ABI (`libfrost_tbtc`) with coarse operations keyed by `session_id`:
  - `RunDKG`
  - `StartSignRound`
  - `FinalizeSignRound`
  - `BuildTaprootTx`
  - `RefreshShares`
- Exposes ROAST liveness policy metadata via:
  - `RoastLivenessPolicy`
- Exposes hardening/runtime counters via:
  - `HardeningMetrics`
- Exposes transcript-accountability and blame-proof helpers via:
  - `RoastTranscriptAudit`
  - `VerifyBlameProof`
- Exposes auto-quarantine status via:
  - `QuarantineStatus`
- Exposes refresh cadence + emergency rekey controls via:
  - `RefreshCadenceStatus`
  - `TriggerEmergencyRekey`
- Exposes differential safety harness and canary rollout controls via:
  - `RunDifferentialFuzzing`
  - `CanaryRolloutStatus`
  - `PromoteCanary`
  - `RollbackCanary`
- Enforces idempotency semantics per `session_id` for retries, with optional
  file-backed state persistence.
- Uses deterministic JSON request/response envelopes across the FFI boundary.
- Provides explicit, typed error codes for retry-safe orchestration.
- Keeps bootstrap synthetic finalize behavior fail-closed by default; enable it
  explicitly with `TBTC_SIGNER_ALLOW_BOOTSTRAP=true`.

## Not yet implemented

- ROAST coordinator logic.
- Full Taproot script-tree construction/signing policy semantics (current
  `BuildTaprootTx` path assembles validated unsigned transactions from provided
  inputs/outputs).
- Canonical non-JSON serialization compatibility rules/tests for the FFI
  boundary.

## Build

```bash
cd pkg/tbtc/signer
cargo build
```

For a dynamic library artifact:

```bash
cd pkg/tbtc/signer
cargo build --release
# target/release/libfrost_tbtc.{so,dylib,dll}
```

## Test

```bash
cd pkg/tbtc/signer
cargo test
```

## Admission Checker (P0-M1)

Run the pre-admission checker for operator onboarding policy:

```bash
cd pkg/tbtc/signer
cargo run --bin admission_checker -- \
  --policy scripts/admission-policy-v1.sample.json \
  --candidate scripts/admission-candidate.sample.json \
  --existing scripts/admission-existing.sample.json
```

Exit codes:

- `0`: candidate satisfies policy
- `1`: candidate rejected (see JSON reason codes in stdout)
- `2`: checker input/config error

To evaluate a governance override, pass both:

- `--override <path>` for the signed override artifact
- `--override-registry <path>` for the consumed-override replay-protection registry

Note: the override registry assumes single-writer access. Do not run concurrent
`admission_checker` invocations against the same `--override-registry` path.

`scripts/admission-override.sample.json` documents the artifact schema and
requires a real Schnorr signature over `payload_json`.

Sample input schemas are provided in:

- `pkg/tbtc/signer/scripts/admission-policy-v1.sample.json`
- `pkg/tbtc/signer/scripts/admission-candidate.sample.json`
- `pkg/tbtc/signer/scripts/admission-existing.sample.json`
- `pkg/tbtc/signer/scripts/admission-override.sample.json`
- `pkg/tbtc/signer/scripts/admission-override-registry.sample.json`

## TEE Governance Registry Checker (Phase A)

Run the governance registry validator for TEE-required signer policy artifacts:

```bash
cd pkg/tbtc/signer
cargo run --bin tee_registry_checker -- \
  --registry scripts/tee-governance-registry-v1.sample.json \
  --events scripts/tee-governance-audit-events-v1.sample.json \
  --now-unix 1700100000
```

Flags:

- `--registry <path>` (required): path to governance registry JSON
- `--events <path>` (optional): path to governance audit events JSON
- `--now-unix <seconds>` (optional): override current Unix timestamp for
  deterministic validation (useful for CI and testing)

Exit codes:

- `0`: registry/events satisfy Phase A schema and workflow checks
- `1`: policy or workflow violations were detected (see JSON reason codes)
- `2`: checker input/config error

Output schema (exit codes 0 and 1):

```json
{
  "decision": "allow | reject",
  "reasons": [{ "code": "reason_code", "detail": "human-readable detail" }],
  "validated_at_unix": 1700100000
}
```

Sample input schemas are provided in:

- `pkg/tbtc/signer/scripts/tee-governance-registry-v1.sample.json`
- `pkg/tbtc/signer/scripts/tee-governance-audit-events-v1.sample.json`

## TEE Admission Token Checker (Phase B)

Run the verifier/token checker for threshold-signed admission tokens:

```bash
cd pkg/tbtc/signer
cargo run --bin tee_token_checker -- \
  --registry scripts/tee-governance-registry-v1.sample.json \
  --keyset scripts/tee-verifier-keyset-v1.sample.json \
  --token scripts/tee-admission-token.sample.json \
  --revocation-registry scripts/tee-token-revocation-registry-v1.sample.json \
  --now-unix 1700100000
```

Flags:

- `--registry <path>` (required): governance registry JSON
- `--keyset <path>` (required): verifier keyset JSON (`m-of-n` threshold config)
- `--token <path>` (required): admission token artifact JSON (`payload_json` + signatures)
- `--revocation-registry <path>` (required): token/key revocation registry JSON
- `--now-unix <seconds>` (optional): deterministic time override for CI/testing

The checker requires `profile_status = mandatory` in the governance registry.
Token signatures cover the exact `payload_json` byte string in the token
artifact; issuers and verifiers must not deserialize and reserialize that value
before signature verification. The token checker enforces a two-trust-root and
two-verifier-instance floor for quorum diversity; broader vendor concentration
limits are enforced by the runtime checker.

Exit codes:

- `0`: token satisfies Phase B checks
- `1`: token/keyset/revocation violations detected (see JSON reason codes)
- `2`: checker input/config error

Output schema (exit codes 0 and 1):

```json
{
  "decision": "allow | reject",
  "reasons": [{ "code": "reason_code", "detail": "human-readable detail" }],
  "validated_at_unix": 1700100000
}
```

Sample input schemas are provided in:

- `pkg/tbtc/signer/scripts/tee-verifier-keyset-v1.sample.json`
- `pkg/tbtc/signer/scripts/tee-admission-token.sample.json`
- `pkg/tbtc/signer/scripts/tee-token-revocation-registry-v1.sample.json`

`tee-admission-token.sample.json` and `tee-verifier-keyset-v1.sample.json` are
schema-only examples and require real verifier keys/signatures to pass.

## TEE Runtime Enforcement Checker (Phase C)

Run the runtime selection/session checker for attestation-token and denylist
enforcement:

```bash
cd pkg/tbtc/signer
cargo run --bin tee_runtime_checker -- \
  --registry scripts/tee-runtime-governance-registry-v1.sample.json \
  --session scripts/tee-runtime-session-start-v1.sample.json \
  --now-unix 1700100000
```

Flags:

- `--registry <path>` (required): governance registry JSON for runtime checks
- `--session <path>` (required): runtime session/cohort snapshot JSON
- `--now-unix <seconds>` (optional): deterministic time override for CI/testing

Runtime session phase values:

- `session_start`: strict token validity required
- `mid_session`: expired tokens tolerated only within `grace_period_seconds`

Hard safety ceilings enforced by the checker:

- `grace_period_seconds <= 3600`
- `attestation_max_age_seconds <= 86400`
- `denylist_max_staleness_seconds` in `1..=300`
- `max_single_vendor_share_percent` in `1..=60`

Exit codes:

- `0`: runtime session satisfies Phase C checks
- `1`: runtime policy violations detected (see JSON reason codes)
- `2`: checker input/config error

Output schema (exit codes 0 and 1):

```json
{
  "decision": "allow | reject",
  "reasons": [{ "code": "reason_code", "detail": "human-readable detail" }],
  "validated_at_unix": 1700100000
}
```

Sample input schemas are provided in:

- `pkg/tbtc/signer/scripts/tee-runtime-governance-registry-v1.sample.json`
- `pkg/tbtc/signer/scripts/tee-runtime-session-start-v1.sample.json`
- `pkg/tbtc/signer/scripts/tee-runtime-session-mid-session-grace-v1.sample.json`
- `pkg/tbtc/signer/scripts/tee-runtime-session-vendor-outage-v1.sample.json`

## TEE Phase D Enforcement Checker (Phase D)

Run the Phase D mode/waiver checker that applies canary and full-enforcement
policy over a Phase C runtime decision:

```bash
cd pkg/tbtc/signer
cargo run --bin tee_enforcement_checker -- \
  --registry scripts/tee-governance-registry-v1.sample.json \
  --context scripts/tee-enforcement-context-monitor-v1.sample.json \
  --now-unix 1700100000
```

Flags:

- `--registry <path>` (required): governance registry JSON with break-glass policy
- `--context <path>` (required): Phase D session context JSON
- `--now-unix <seconds>` (optional): deterministic time override for CI/testing

Supported `enforcement_mode` values:

- `monitor_only`: record runtime violations without blocking
- `soft_enforcement`: warnings + exclusion preference without blocking
- `hard_enforcement_canary`: blocking applies to canary sessions only
- `full_enforcement`: blocking applies to all sessions

Break-glass enforcement controls:

- scope coverage for selected operators
- required quorum (`break_glass_quorum_bps`)
- waiver TTL bounded by policy (`break_glass_ttl_seconds`) and hard maximum (7 days)
- activation cap per 7 days (`break_glass_max_activations_per_7d`) for new activations
- minimum cooldown (`break_glass_cooldown_seconds`) for new activations
- reused incident tickets bounded by their history-derived activation time and TTL

Exit codes:

- `0`: `allow` or `allow_with_warnings`
- `1`: `reject`
- `2`: checker input/config error

Output schema (exit codes 0 and 1):

```json
{
  "decision": "allow | allow_with_warnings | reject",
  "reasons": [{ "code": "reason_code", "detail": "human-readable detail" }],
  "validated_at_unix": 1700100000
}
```

Sample input schemas are provided in:

- `pkg/tbtc/signer/scripts/tee-enforcement-context-monitor-v1.sample.json`
- `pkg/tbtc/signer/scripts/tee-enforcement-context-hard-canary-v1.sample.json`
- `pkg/tbtc/signer/scripts/tee-enforcement-context-full-break-glass-v1.sample.json`

## Encrypted State Key Providers

Signer state persistence is encrypted at rest. Key-provider behavior is controlled
by the following environment variables:

- `TBTC_SIGNER_STATE_KEY_PROVIDER`:
  - `env` (default): read key from `TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX`.
  - `command`: execute `TBTC_SIGNER_STATE_KEY_COMMAND` and read key from stdout.
- `TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX`:
  - 64 hex chars (32 bytes) when provider is `env`.
- `TBTC_SIGNER_STATE_KEY_COMMAND`:
  - shell command executed via `/bin/sh -lc` when provider is `command`.
- `TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS`:
  - timeout for command-provider execution in seconds (default `30`, range `1..300`).
- `TBTC_SIGNER_PROFILE`:
  - when set to `production`, provider `env` is rejected fail-closed.

Command-provider contract (`TBTC_SIGNER_STATE_KEY_COMMAND`):

- Must exit with status `0`.
- Must write a single 32-byte key as hex (64 chars) to stdout.
- Trailing newline is allowed.
- Must return the same key across signer restarts for the same state file.
- Should not log key material.

The encrypted envelope stores a derived key identifier (`sha256:<digest>`), and
load fails closed if the configured provider returns a different key.
State files written before this change with legacy
`key_id=TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX` remain readable for compatibility.

### Local/dev example (env provider)

```bash
export TBTC_SIGNER_STATE_KEY_PROVIDER=env
export TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX="$(openssl rand -hex 32)"
```

### AWS KMS example (command provider)

Assumes a ciphertext blob was produced earlier and stored on disk.

```bash
export TBTC_SIGNER_PROFILE=production
export TBTC_SIGNER_STATE_KEY_PROVIDER=command
export TBTC_SIGNER_STATE_KEY_COMMAND='aws kms decrypt \
  --region "$AWS_REGION" \
  --ciphertext-blob fileb://"$TBTC_SIGNER_STATE_KEY_BLOB_PATH" \
  --query Plaintext --output text \
  | base64 --decode \
  | xxd -p -c 256'
```

### GCP KMS example (command provider)

```bash
export TBTC_SIGNER_PROFILE=production
export TBTC_SIGNER_STATE_KEY_PROVIDER=command
export TBTC_SIGNER_STATE_KEY_COMMAND='tmp="$(mktemp)"; \
  gcloud kms decrypt \
    --location "$GCP_KMS_LOCATION" \
    --keyring "$GCP_KMS_KEYRING" \
    --key "$GCP_KMS_KEY" \
    --ciphertext-file "$TBTC_SIGNER_STATE_KEY_BLOB_PATH" \
    --plaintext-file "$tmp" >/dev/null && \
  xxd -p -c 256 "$tmp"; \
  rc=$?; rm -f "$tmp"; exit $rc'
```

### HSM/agent example (command provider)

If a local HSM-backed agent is available:

```bash
export TBTC_SIGNER_PROFILE=production
export TBTC_SIGNER_STATE_KEY_PROVIDER=command
export TBTC_SIGNER_STATE_KEY_COMMAND='/opt/tbtc-signer/bin/state-key-agent \
  --key tbtc-signer-state-v1 \
  --format hex'
```

### Rotation, Recovery, and Failure Modes

State-key rotation must be planned as an operator runbook, not an automatic
startup behavior. To rotate, stop the signer, back up the encrypted state file,
decrypt or unwrap the current state key through the existing provider, re-encrypt
the state file with the new provider key in an offline maintenance step, then
start with the new command provider and verify the envelope `key_id` matches the
new provider. Do not delete the old KMS/HSM material until restart/load evidence
has been captured and rollback has been approved.

Recovery requires restoring both the encrypted state file and the provider-side
key material or wrapped key blob for the envelope `key_id`. If either side is
missing, the signer must remain stopped or quarantined; replacing the provider
with a different key intentionally fails closed with a key-id mismatch.

Failure-mode responses:

- Missing command, non-zero exit, timeout, non-UTF-8 output, malformed hex, or
  key-id mismatch: leave `TBTC_SIGNER_PROFILE=production` enabled, keep the
  signer out of service, and repair the provider or restore matching key
  material. Do not fall back to `env` in production.
- KMS/HSM outage: keep the node failed closed, confirm other operators preserve
  threshold availability, and use the approved provider recovery path before
  restarting.
- Suspected provider compromise: stop the signer, preserve logs and state
  artifacts, rotate through the offline process above, and require security-owner
  approval before returning to service.

## Benchmarks (Phase 5 Scaffold)

Run the Phase 5 benchmark harness:

```bash
cd pkg/tbtc/signer
cargo bench --features bench-restart-hook --bench phase5_roast
```

Current benchmark groups:

- `phase5/ffi_run_dkg` (`RunDKG` happy path)
- `phase5/ffi_start_sign_round` (`StartSignRound` happy path)
- `phase5/ffi_finalize_sign_round` (bootstrap finalize happy path)
- `phase5/ffi_start_sign_round_recovery`:
  - `timeout_transition_authorized`
  - `invalid_share_proof_transition_with_rotation`
- `phase5/ffi_start_sign_round_replay_guard`:
  - `stale_attempt_rejected_after_transition`
- `phase5/ffi_start_sign_round_restart_paths`:
  - `authorized_transition_after_reload`
  - `stale_attempt_rejected_after_reload`

## Chaos Suite (Phase 5)

Run the Phase 5 chaos/failure-injection suite:

```bash
cd pkg/tbtc/signer
./scripts/run_phase5_chaos_suite.sh
```

Scenario coverage and pass criteria:

- `stale_payload_replay_or_duplication`: stale attempt payloads remain fail-closed
  after authorized advancement and reload.
- `restart_recovery_authorized_transition`: authorized transition succeeds after
  restart/reload with deterministic attempt context.
- `process_crash_active_attempt`: consumed-attempt replay guard survives
  simulated crash and cache loss.
- `persist_fault_pre_rename`: previous durable state remains intact after
  injected pre-rename persist fault.
- `persist_fault_post_rename`: renamed durable state remains loadable after
  injected post-rename persist fault.

## FFI contract

- Header: `pkg/tbtc/signer/include/frost_tbtc.h`
- All API payloads are JSON bytes.
- Success: `status_code = 0`, response envelope in `buffer`.
- Error: `status_code = 1`,
  `{"code":"...","message":"...","recovery_class":"..."}` JSON in `buffer`.
- `recovery_class` values:
  - `recoverable`: caller can retry with corrected/updated input.
  - `terminal`: session state is terminal for the current operation/session.
- `frost_tbtc_roast_liveness_policy` response:
  - `coordinator_timeout_ms`: effective coordinator-timeout policy in
    milliseconds.
  - `timeout_source`: timeout clock/source identifier (`keep_core_wall_clock`).
  - `advance_trigger`: policy trigger used for attempt advancement
    (`coordinator_timeout`).
  - `exclusion_evidence_policy`: evidence policy marker
    (`timeout_or_invalid_share_proof`).
- `frost_tbtc_hardening_metrics` response includes:
  - runtime version and enforcement flags for provenance/admission/policy gates
  - counters for DKG calls/successes/admission rejects
  - counters for start-sign-round calls/successes
  - counters for build-tx calls/successes/policy rejects
  - counters for refresh-shares calls/successes
  - counters for transcript-audit and blame-proof verification calls/successes
  - counters for finalize calls/successes and attempt transition/failover events
  - counters for auto-quarantine fault events/enforcements and current
    quarantined-operator count
  - counters for overdue refresh sessions and emergency-rekey-required sessions
  - counters for differential-fuzz runs/critical divergences and canary
    promotions/rollbacks
  - p95 latency and sample-count fields for `run_dkg`, `start_sign_round`,
    `build_taproot_tx`, `finalize_sign_round`, and `refresh_shares`
- Coordinator timeout policy config:
  - env var: `TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS`
  - valid range: `1000..=300000`
  - default: `30000`
- Provenance gate config:
  - `TBTC_SIGNER_ENFORCE_PROVENANCE_GATE`
  - `TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS` (must be `approved`)
  - `TBTC_SIGNER_PROVENANCE_TRUST_ROOT` (required 32-byte x-only secp256k1 public key hex)
  - `TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD` (signed JSON containing `status`, `runtime_version`, and required `expires_at_unix`)
  - `TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX` (schnorr signature hex over `sha256(payload_bytes)`)
  - `TBTC_SIGNER_MIN_APPROVED_VERSION`
- Admission policy config:
  - `TBTC_SIGNER_ENFORCE_ADMISSION_POLICY`
  - `TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS`
  - `TBTC_SIGNER_ADMISSION_MIN_THRESHOLD`
  - `TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS` (comma-separated)
  - `TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS` (comma-separated; unset to disable, empty string is invalid)
- Signing policy firewall config:
  - `TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL`
  - `TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES` (comma-separated, e.g. `p2tr,p2wpkh`)
  - `TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT` (required when firewall is enabled)
  - `TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS` (required when firewall is enabled)
  - `TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS` (required when firewall is enabled)
  - `TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR` / `TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR`
    - Note: setting `ALLOWED_UTC_START_HOUR == ALLOWED_UTC_END_HOUR` opens a
      24-hour window (all hours permitted).
  - `TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE`
  - Signing-path binding: when the firewall is enabled, `StartSignRound.message_hex`
    must equal `sha256(tx_hex_bytes)` from the same-session `BuildTaprootTx`
    result; `FinalizeSignRound` re-validates the same binding.
- Transcript accountability / quarantine config:
  - `TBTC_SIGNER_ENABLE_AUTO_QUARANTINE`
  - `TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD`
  - `TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY`
  - `TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY`
  - `TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS`
  - `RoastTranscriptAudit` returns persisted attempt-transition records (hash +
    exclusion evidence) for a session.
  - `VerifyBlameProof` validates a claimed excluded operator/reason against the
    persisted transcript record for the requested attempt.
  - `QuarantineStatus` reports current score/quarantine state for an operator.
- Refresh cadence / emergency rekey:
  - `TBTC_SIGNER_REFRESH_CADENCE_SECONDS` (valid range: `60..=2592000`,
    default `86400`)
  - `RefreshCadenceStatus` reports continuity/overdue status and rekey flags.
  - `TriggerEmergencyRekey` marks a session as rekey-required and blocks
    additional signing starts for that session.
- Differential safety + canary controls:
  - `RunDifferentialFuzzing` runs deterministic differential checks for ROAST
    attempt context hashing and policy-bound signing message derivation.
  - `CanaryRolloutStatus` reports current rollout cohort and SLO gate posture.
    This endpoint is provenance-gated.
  - `PromoteCanary` enforces `10% -> 50% -> 100%` progression and halts on SLO
    gate failure.
  - `RollbackCanary` restores the previous cohort with persisted config
    versioning.
  - SLO gate env vars:
    - `TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS`
    - `TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS`
    - `TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS`
- Known limitations (P0 scope):
  - Policy gates default to disabled: provenance/admission/signing enforcement
    gates require explicit `=true` env vars.
- `StartSignRound.attempt_transition_evidence.exclusion_evidence` schema:
  - `reason`: `coordinator_timeout` or `invalid_share_proof`
  - `excluded_member_identifiers`: members excluded from the next attempt
  - `invalid_share_proof_fingerprint`: required only for
    `invalid_share_proof`, omitted for `coordinator_timeout`
- `StartSignRound` response telemetry:
  - `attempt_transition_telemetry` is included when attempt advancement is
    authorized, with:
    - from/to attempt numbers
    - from/to coordinator identifiers
    - transition reason
    - excluded member identifiers
    - `coordinator_rotated` flag
- Representative error codes:
  - `provenance_gate_rejected`: provenance/min-version gate rejected request.
  - `admission_policy_rejected`: DKG admission policy rejected request.
  - `signing_policy_rejected`: signing policy firewall rejected request.
  - `lifecycle_policy_rejected`: refresh/canary lifecycle policy rejected
    request.
  - `session_conflict`: same session retried with a different payload.
  - `session_finalized`: `StartSignRound` called after successful finalize on
    that session.
  - `synthetic_contribution_rejected`: synthetic finalize payload used while
    bootstrap mode is disabled.
- Call `frost_tbtc_free_buffer` for every returned buffer.
