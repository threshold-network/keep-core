# ROAST Phase 5: Security/Review Gates And Rollout

Date: 2026-02-28
Status: In progress
Owner: Threshold Labs
Scope: define rollout decision gates, provisional rollback thresholds, and
evidence requirements for ROAST enablement

## Objective

Translate the Phase 5 goals from `roast-implementation-plan.md` into explicit
go/no-go checks that can be used during staged rollout decisions.

This increment adds draft operational thresholds (requested in prior review) so
rollout decisions are bounded before final canary execution begins.

## Gate Framework

### Gate 1: Security/Correctness Sign-Off

Required before any production canary:

1. Adversarial review packet complete with no unresolved CRITICAL/HIGH findings.
2. Replay, transition-authorization, and restart-safety test suites green.
3. Cross-repo contract compatibility verified for:
   - `recovery_class`
   - `exclusion_evidence`
   - `attempt_transition_telemetry`

### Gate 2: Canary Readiness

Required before stage 1 canary:

1. Baseline metrics captured for pre-ROAST control window:
   - success rate
   - coordinator rotations per signing request
   - p95 and p99 signing latency
2. Observability dashboards include transition reason and recovery class splits.
3. Rollback playbook validated in a dry-run incident simulation.

### Gate 3: Progressive Rollout

Recommended stages:

1. Stage 1: 5% signer fleet / limited wallet cohort, hold for 24h.
2. Stage 2: 25% signer fleet / broader cohort, hold for 24h.
3. Stage 3: 100% rollout after Phase 5 acceptance criteria remain green.

## Provisional Rollback Thresholds (Draft)

These thresholds are intentionally conservative and should be tuned once the
baseline window is recorded.

1. Attempt success rate:
   - `hold` if `< 99.0%` over any rolling 6-hour canary window.
   - `rollback` if `< 97.0%` over any rolling 1-hour window.
2. Coordinator rotations per signing request:
   - `hold` if `> 0.35` average over rolling 6 hours.
   - `rollback` if `> 0.60` average over rolling 1 hour.
3. Signing latency deltas vs baseline:
   - `hold` if p95 delta `> +25%` for 1 hour.
   - `rollback` if p99 delta `> +40%` for 30 minutes.
4. Terminal failure ratio:
   - `hold` if terminal failures exceed `0.5%` of signing attempts in 1 hour.
   - `rollback` if terminal failures exceed `1.0%` in 30 minutes.

## No-Go Triggers

Immediate rollout pause and incident response escalation:

1. Any evidence of unauthorized attempt advancement acceptance.
2. Any replay-protection regression for consumed attempt/round identifiers.
3. Any state-restart inconsistency causing divergent transition decisions.
4. Missing telemetry fields required for operator triage in canary incidents.

## Evidence Checklist

Before final sign-off, collect and archive:

1. Security review packet with explicit GO/Conditional GO decision.
2. Benchmark output for:
   - happy path
   - single-member failure
   - coordinator-timeout recovery
3. Chaos/failure-matrix results for:
   - network delay/duplication
   - process crash during active attempt
   - recovery after restart
4. Rollout metrics snapshots for each canary stage and final production cutover.
5. Final approval record attached to the release or governance decision.
6. Baseline calibration worksheet:
   - `docs/frost-migration/roast-phase-5-baseline-calibration.md`

## Initial Benchmark Scaffold (Implemented)

- Benchmark harness added at `pkg/tbtc/signer/benches/phase5_roast.rs`.
- Run command:
  `cd pkg/tbtc/signer && cargo bench --features bench-restart-hook --bench phase5_roast`
- Current benchmark groups:
  - `phase5/ffi_run_dkg`
  - `phase5/ffi_start_sign_round`
  - `phase5/ffi_finalize_sign_round`
  - `phase5/ffi_start_sign_round_recovery`
    - `timeout_transition_authorized`
    - `invalid_share_proof_transition_with_rotation`
  - `phase5/ffi_start_sign_round_replay_guard`
    - `stale_attempt_rejected_after_transition`
  - `phase5/ffi_start_sign_round_restart_paths`
    - `authorized_transition_after_reload`
    - `stale_attempt_rejected_after_reload`
- Phase 5 benchmark and chaos evidence is summarized in this rollout gate
  packet.

## Chaos/Failure Injection Suite (Implemented)

- Suite runner:
  `pkg/tbtc/signer/scripts/run_phase5_chaos_suite.sh`
- Run command:
  `cd pkg/tbtc/signer && ./scripts/run_phase5_chaos_suite.sh`
- Scenario pass/fail criteria:
  - `stale_payload_replay_or_duplication`:
    stale attempt payloads remain fail-closed after authorized advancement and
    reload.
  - `restart_recovery_authorized_transition`:
    authorized transition succeeds after restart/reload with deterministic
    attempt context.
  - `process_crash_active_attempt`:
    consumed-attempt replay guard survives simulated crash and cache loss.
  - `persist_fault_pre_rename`:
    previous durable state remains intact after injected pre-rename persist
    fault.
  - `persist_fault_post_rename`:
    renamed durable state remains loadable after injected post-rename persist
    fault.

## Rollout Runbook (Implemented)

- Runbook artifact:
  `docs/frost-migration/roast-phase-5-rollout-runbook.md`
- Future mandatory TEE hardening profile
  (activation-gated):
  `docs/frost-migration/tee-whitelisted-signer-enforcement-plan.md`

## Baseline Calibration Worksheet (Prepared)

- Worksheet artifact:
  `docs/frost-migration/roast-phase-5-baseline-calibration.md`
- Current blocker:
  environment readiness for baseline data collection.

## Remaining Phase 5 Work

1. Populate baseline worksheet and record final threshold values.
2. Complete required human approval entries in the release or governance
   record.
