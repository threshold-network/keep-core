# TEE-Required Signer Plan (DAO-Whitelisted Operators)

Date: 2026-03-01
Status: Draft
Owner: Threshold Labs
Scope: operator-admission and runtime enforcement model for TEE-required
signers in a DAO-whitelisted signer set

## 1. Objective

Define a production-usable policy and implementation plan for running ROAST/FROST
signers with:

1. DAO-approved operator whitelist, and
2. TEE-backed signer runtime admission.

This plan assumes maximizing permissionless signer scale is not the primary
objective; operator accountability and controlled hardening are prioritized.

This plan is a draft for a future mandatory hardening profile. It is not active
for production rollouts until the activation gate in Section 12 is approved and
recorded in governance artifacts.

## 2. Design Principles

1. Cryptographic protocol safety remains primary (ROAST/FROST controls do not
   depend on TEEs).
2. TEE is additive hardening for runtime integrity and key protection.
3. Liveness must not depend on a single vendor, verifier, or attestation root.
4. Policy changes are governance-controlled, explicit, and auditable.
5. No silent downgrade from TEE-required to non-TEE operation.

## 3. Security Goals

1. Admit only authorized operators running approved signer binaries in approved
   TEE environments.
2. Detect and remove revoked/non-compliant signers with bounded exposure time.
3. Preserve quorum liveness during partial TEE/verifier outages.
4. Maintain full audit trail for admissions, revocations, and emergency waivers.

## 4. Non-Goals

1. Replacing ROAST/FROST replay/transition protections with attestation logic.
2. Requiring all ecosystem participants to use a single TEE vendor stack.
3. Proving physical side-channel resistance of all enclave platforms.

## 5. Policy Model

### 5.1 Operator Admission Record

Each signer admission record should include:

1. `operator_id` (DAO-known identity)
2. `signer_identifier` (runtime signer identity/public key)
3. `status` (`active`, `suspended`, `revoked`)
4. `allowed_tee_types` (e.g., SGX/SEV-SNP/TDX)
5. `allowed_measurements` (approved signer binary measurements)
6. `attestation_max_age_seconds`
7. `grace_period_seconds`
8. `effective_from` and optional `effective_until`

### 5.2 Enforcement Parameters (Initial Defaults)

| Parameter | Initial Default | Notes |
| --- | --- | --- |
| `attestation_max_age_seconds` | `3600` | re-attestation required hourly |
| `grace_period_seconds` | `900` | temporary verifier/vendor disruptions |
| `min_attested_signers_per_cohort` | `threshold + 1` | avoid edge-of-quorum fragility |
| `max_single_vendor_share_percent` | `40` | cap correlated vendor risk |
| `denylist_max_staleness_seconds` | `60` | session-start denylist freshness bound |
| `break_glass_ttl_seconds` | `21600` | 6-hour emergency override max |
| `break_glass_max_activations_per_7d` | `2` | prevent break-glass chaining abuse |
| `break_glass_cooldown_seconds` | `86400` | 24-hour cooldown between activations |
| `break_glass_scope` | `named_operator_ids_only` | no global suspension in default policy |
| `break_glass_quorum_bps` | `6700` | supermajority quorum for emergency break-glass actions |
| `activation_gate_required_quorum_bps` | `6700` | independent quorum threshold for `draft -> mandatory` activation gate; hard floor of 6700 bps enforced by checker |
| `re_attestation_poll_interval_seconds` | `300` | signer refresh cadence |

Values should be tuned with canary data and incident drills.

## 6. Control Plane Architecture

### 6.1 Components

1. **Governance Whitelist Registry**
   - DAO-controlled source of truth for operator admission records.
   - every change emits immutable governance event (`add`, `suspend`, `revoke`,
     `measurement_update`, `break_glass_activate`, `break_glass_expire`).
2. **Attestation Verifier Service**
   - validates evidence against vendor trust roots,
   - checks measurement allowlist,
   - issues short-lived signed admission tokens.
3. **Revocation and Audit Service**
   - immediate denylist propagation,
   - immutable audit event stream for admissions/revocations.

Verifier trust model requirements:

1. at least two independent verifier instances operated on separate trust roots.
2. admission tokens must be threshold-signed by verifier quorum (`m-of-n`,
   initial `2-of-3`) or include equivalent multi-verifier attestations.
3. verifier signing keys rotate every 30 days maximum, with overlap window and
   published key-set versioning.
4. verifier compromise response:
   - key revocation event published within 15 minutes of detection,
   - compromised key removed from accepted verifier set immediately,
   - all tokens signed solely by compromised key invalidated.
5. verifier issuance controls:
   - per-operator and per-signer token issuance rate limits,
   - anomaly alerts on issuance spikes, unknown signer identifiers, or repeated
     failed attestation proofs.

### 6.2 Admission Token Claims

Tokens should contain at minimum:

1. `operator_id`
2. `signer_identifier`
3. `tee_type`
4. measurement digest
5. issue/expiry timestamps
6. registry snapshot/version ID
7. verifier key ID
8. `token_id` (unique `jti`) for token-level revocation
9. `token_revocation_epoch` for monotonic revocation checkpoints

## 7. Runtime Enforcement Model

### 7.1 Coordinator / Runtime Selection

1. Select only `active` + currently attested signers.
2. Enforce vendor diversity cap during cohort assembly.
3. Enforce live denylist check for `operator_id` and `signer_identifier` before
   cohort selection (freshness <= `denylist_max_staleness_seconds`).
4. Reject cohort construction if policy constraints cannot be met.

### 7.2 Signer Runtime

1. Periodically refresh attestation token.
2. Refuse signing when token expired and outside grace period.
3. Refuse signing when token_id or token_revocation_epoch is revoked.
4. Emit structured telemetry on attestation status transitions.

### 7.3 Session Behavior

1. Session start requires:
   - valid attestation token for all selected signers,
   - live denylist check for all selected signers,
   - denylist freshness <= `denylist_max_staleness_seconds`.
2. Mid-session expiry within grace window: allow completion, block new sessions.
3. Mid-session expiry beyond grace: fail closed and trigger retry/reselection.
4. Maximum revocation TOCTOU window for new sessions is bounded by
   `denylist_max_staleness_seconds`; deployments must not exceed 60 seconds.

## 8. Governance and Emergency Controls

### 8.1 Normal Governance Actions

1. add operator/signer
2. rotate measurement allowlist
3. suspend/revoke operator
4. update enforcement parameters

### 8.2 Break-Glass Mode

Break-glass allows temporary policy relaxation only via explicit DAO/quorum
approval and strict TTL.

Requirements:

1. explicit incident ticket reference
2. automatic expiry
3. complete audit logs of all sessions admitted under waiver
4. mandatory post-incident review before reactivation
5. maximum activations per 7-day window:
   `break_glass_max_activations_per_7d` (default 2)
6. minimum cooldown between activations:
   `break_glass_cooldown_seconds` (default 24h)
7. scope limited to named `operator_id` set; global suspension is disallowed in
   default policy
8. break-glass quorum explicitly set to `break_glass_quorum_bps` (default 67%)

## 9. Failure Modes and Handling

1. **Verifier outage**:
   - use grace window,
   - if exceeded, stop admitting new sessions.
2. **Single vendor outage**:
   - preserve safety-first ordering:
     1. keep `min_attested_signers_per_cohort = threshold + 1` as hard floor
     2. if vendor cap blocks liveness, allow graduated temporary relaxation:
        `40% -> 50% -> 60%` (max) during declared vendor outage only
     3. each relaxation step expires automatically in 6 hours unless renewed
        by governance action
   - if liveness still cannot be restored, require scoped break-glass.
3. **Measurement drift after upgrade**:
   - staged rollout with pre-approved next measurements,
   - reject unknown measurements by default,
   - emergency fast-path for critical fixes:
     1. emergency measurement proposal with incident reference
     2. reduced-latency governance vote (target <= 6 hours)
     3. explicit emergency quorum requirement
     4. automatic rollback of emergency measurement if not ratified in 48 hours
        by normal governance flow.
4. **Operator compromise**:
   - immediate revoke/suspend,
   - denylist propagation and cohort reselection,
   - denylist propagation target <= 60 seconds to all coordinators.

## 10. Implementation Phases

### Phase A (Policy + Registry)

1. implement DAO admission schema
2. implement operator status lifecycle
3. define governance workflows and audit events
4. codify activation gate from "draft profile" to "mandatory enforcement"

### Phase B (Verifier + Tokens)

1. deploy attestation verifier service
2. issue/validate admission tokens
3. integrate denylist and key rotation
4. implement multi-verifier threshold token issuance
5. implement token-level revocation (`token_id`, `token_revocation_epoch`)

### Phase C (Runtime Enforcement)

1. add selection-time and session-time token checks
2. enforce vendor diversity caps
3. add telemetry and alerts
4. enforce live denylist freshness bound at session start
5. implement graduated diversity-cap relaxation controls

### Phase D (Canary + Hard Enforcement)

1. monitor-only mode (no blocking)
2. soft enforcement (warnings + exclusion preference)
3. hard enforcement for canary cohort
4. full enforcement after gate pass
5. enforce break-glass abuse controls (activation caps + cooldown + scope)

### 10.2 Phase A Scaffold Artifacts

Initial Phase A schema/workflow checks are implemented in:

1. `tools/tbtc-signer/src/bin/tee_registry_checker.rs`
2. `tools/tbtc-signer/scripts/tee-governance-registry-v1.sample.json`
3. `tools/tbtc-signer/scripts/tee-governance-audit-events-v1.sample.json`

### 10.3 Phase B Scaffold Artifacts

Initial Phase B verifier/token checks are implemented in:

1. `tools/tbtc-signer/src/bin/tee_token_checker.rs`
2. `tools/tbtc-signer/scripts/tee-verifier-keyset-v1.sample.json`
3. `tools/tbtc-signer/scripts/tee-admission-token.sample.json`
4. `tools/tbtc-signer/scripts/tee-token-revocation-registry-v1.sample.json`

### 10.4 Phase C Scaffold Artifacts

Initial Phase C runtime selection/session checks are implemented in:

1. `tools/tbtc-signer/src/bin/tee_runtime_checker.rs`
2. `tools/tbtc-signer/scripts/tee-runtime-governance-registry-v1.sample.json`
3. `tools/tbtc-signer/scripts/tee-runtime-session-start-v1.sample.json`
4. `tools/tbtc-signer/scripts/tee-runtime-session-mid-session-grace-v1.sample.json`
5. `tools/tbtc-signer/scripts/tee-runtime-session-vendor-outage-v1.sample.json`

### 10.5 Phase D Scaffold Artifacts

Initial Phase D canary/hard-enforcement checks are implemented in:

1. `tools/tbtc-signer/src/bin/tee_enforcement_checker.rs`
2. `tools/tbtc-signer/scripts/tee-enforcement-context-monitor-v1.sample.json`
3. `tools/tbtc-signer/scripts/tee-enforcement-context-hard-canary-v1.sample.json`
4. `tools/tbtc-signer/scripts/tee-enforcement-context-full-break-glass-v1.sample.json`

Phase D final readiness outcome:

- final review recommendation: `READY` for the scaffold branch,
- prior Phase D enforcement-mode blockers resolved: `8/8`,
- merge blockers remaining: `0`,
- Phase D unit tests passing: `24/24`,
- Phase A/B/C regression tests passing: `29/29`, `24/24`, `23/23`,
- sample enforcement commands verified for monitor-only, hard-canary, and
  full-enforcement break-glass contexts.

Non-blocking future hardening items remain for additional break-glass edge
cases, structural input validation, duplicate-history behavior, and
`serde(deny_unknown_fields)` policy consistency.

### 10.1 Mapping To ROAST Phase 5 Stages

1. ROAST Stage 1 (5% canary) requires TEE Phase C completed and TEE Phase D in
   monitor-only or soft-enforcement mode.
2. ROAST Stage 2 (25% expanded) requires TEE Phase D hard enforcement for the
   canary cohort and no unresolved CRITICAL/HIGH findings.
3. ROAST Stage 3 (100% GA) requires TEE Phase D full enforcement after Section
   12 activation gate approval.

## 11. Validation Matrix

Minimum required scenarios:

1. token expiry during active session
2. verifier unavailable for > grace period
3. operator revocation during signing activity
4. mixed-vendor cohort selection under load
5. governance break-glass activation/expiry correctness
6. token non-zero revocation epoch and token_id denylist enforcement
7. verifier key rotation and compromised-key invalidation drill
8. diversity-cap relaxation during declared vendor outage
9. emergency measurement fast-path and automatic rollback path
10. denylist freshness breach (stale denylist should fail closed)

## 12. Rollout Gates

Before hard enforcement in production:

1. all validation scenarios pass
2. no unresolved CRITICAL/HIGH findings in attestation path
3. incident runbook tested in simulation
4. policy and measurements approved by DAO governance process
5. activation gate approved in governance record:
   - profile status transitions from `draft` to `mandatory`
   - approval artifact:
     `docs/frost-migration/tee-whitelisted-signer-activation-gate-record.md`

### 12.1 Activation Gate Record Requirements

Activation gate record must include:

1. governance proposal/decision identifier
2. effective timestamp (UTC)
3. quorum denominator and achieved quorum
4. approver set with roles:
   - security owner
   - signer/runtime owner
   - governance delegate
5. explicit statement:
   - `profile_status_transition = draft -> mandatory`
6. rollback condition and rollback authority

## 13. Integration With Existing ROAST Phase 5 Artifacts

This plan is linked from:

1. `pkg/tbtc/signer/docs/roast-phase-5-security-rollout-gates.md`
2. `pkg/tbtc/signer/docs/roast-phase-5-rollout-runbook.md`

as a future mandatory TEE hardening profile for permissioned operator deployments
once Section 12 activation gate is approved.
