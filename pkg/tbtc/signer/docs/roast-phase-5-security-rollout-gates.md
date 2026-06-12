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

## Cryptographic Dependency Audit Status (Gate 1 Input)

The signer pins `frost-secp256k1-tr = "=3.0.0"` (`Cargo.toml`), the Zcash
Foundation FROST implementation's Taproot (BIP-340/341) ciphersuite,
released 2025-04-23.

External audit coverage of that stack, verified against upstream
statements as of 2026-06-12:

- **NCC Group, "Zcash FROST Security Assessment"** (report dated
  2023-10-20, published October 2023): audited the **v0.6.0** release
  (commit `5fa17ed`) of `frost-core`, `frost-ed25519`, `frost-ed448`,
  `frost-p256`, `frost-secp256k1`, and `frost-ristretto255` - key
  generation (trusted dealer and DKG) and FROST signing. All findings
  were addressed and re-reviewed by NCC.
  Report: <https://www.nccgroup.com/media/m1yjijzn/_ncc_group_zcashfoundation_e008263_report_2023-10-20_v11-1.pdf>
- The upstream README states explicitly: *"This does not include
  frost-secp256k1-tr and rerandomized FROST."*
- **Least Authority, FROST Demo audit (Q1 2025)**: covered the
  `frost-client` and `frostd` demo tooling only - not the library
  crates this signer consumes.
  <https://zfnd.org/frost-demo-audit-frost-client-and-frostd/>
- No 2.x or 3.x release notes mention additional audit coverage.

**Consequence for Gate 1:** the exact ciphersuite this signer uses for
production signatures (`frost-secp256k1-tr`) and the v0.6.0 → 3.0.0
evolution of `frost-core` have **no external audit coverage**. The
NCC assessment establishes pedigree for the core protocol
implementation but cannot be cited as covering the pinned version
range.

**DECIDED (2026-06-12, MacLane): an external audit covering
`frost-core` 3.x and the `frost-secp256k1-tr` ciphersuite is a HARD
GATE for the ECDSA-retirement phases.** Gate 1 sign-off for those
phases requires the completed audit; canary stages before ECDSA
retirement may proceed under the existing gate criteria, but
retirement-phase rollout does not start without the audit report in
hand.

## Decision Log (2026-06-12)

Decisions taken on the post-merge follow-up checklist's open
architecture questions:

1. **External audit = hard gate for ECDSA retirement** (see above).
2. **Sidecar signer process** chosen over in-process cgo as the
   target architecture (stepping stone to TEE deployment). The
   in-process dlopen bridge remains the transitional integration; new
   isolation-sensitive work should assume the sidecar boundary. This
   unblocks scoping of the decision-gated TEE checker stack (#4007).
3. **Script-tree commitment vs timelocked recovery leaf for FROST
   wallets: explicitly OPEN.** Needs more evaluation time; multiple
   open questions remain. No work should bake in either assumption.
4. **Proof-carrying blame (follow-up item 7): deferred until
   production**, with a binding retention condition: telemetry and
   logging must retain enough signed bytes to diagnose whether
   targeted equivocation is occurring, so the revisit decision has
   data. (Retention of conflicting signed evidence envelopes at the
   detection points is implemented in the Go RFC-21 layer; full
   cross-member equivocation comparison arrives with item 7 itself.)
5. **t-of-included finalize (follow-up item 6): scheduled as the
   first engineering item of Phase 7**, not earlier. The transitional
   flow computes each member's signature share at StartSignRound
   against binding factors derived from the full included set's
   commitment list (finalize enforces contributions == included set),
   so first-t-responsive finalize requires computing shares after the
   responsive subset is known - the interactive two-round exchange
   that IS Phase 7's core. Pulling it earlier would implement the
   interactive path without its Go-side consumer.

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
