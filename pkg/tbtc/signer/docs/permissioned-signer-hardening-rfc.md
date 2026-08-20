# RFC: Permissioned Signer Set Hardening Roadmap (Post-ROAST/FROST)

Date: 2026-03-01
Status: Draft for review
Owner: Threshold Labs + DAO Operations
Scope: Additional security and operability hardening for DAO-whitelisted
FROST/ROAST signer sets.

## Context

The ROAST/FROST migration delivers core cryptographic and orchestration
capability. The next risk-reduction layer is operational hardening for a
permissioned signer model.

This RFC defines a phased plan that does not require formal verification or
TEEs as prerequisites, while remaining compatible with either in future.

## Goals

1. Improve accountability for signer and coordinator behavior.
2. Reduce operator concentration and supply-chain risk.
3. Improve liveness during partial failures and targeted abuse.
4. Make releases safer with deterministic rollout and rollback controls.

## Non-goals

1. Transition to a permissionless signer set.
2. Replace FROST/ROAST protocol primitives.
3. Use TEEs as a mandatory trust anchor for baseline production safety.

## Phase Plan

### Phase P0 (Weeks 0-6): Baseline Controls

| Milestone | Deliverables | Primary owners | Exit criteria |
| --- | --- | --- | --- |
| `P0-M1` Signer admission policy v1 | Operator policy spec (geo/provider diversity, HSM/KMS class, patch SLA, incident-response contact), automated pre-admission checker, DAO override workflow | Protocol + Ops + Governance | New admissions are policy-gated, and non-compliant operators are blocked with reason codes |
| `P0-M2` Deterministic builds + provenance enforcement (Complete) | Reproducible signer build recipe, signed provenance artifacts, startup attestation verification, minimum-approved-version gate | Platform + Security | Status: Complete. Evidence: `Cargo.lock` pins all deps; `--locked` enforced via `build.sh`; `enforce_provenance_gate()` enforced at every protected operation entry; `frost_tbtc_init_signer_config` validates attestation set at startup via `validate_attestation_set`; `TBTC_SIGNER_MIN_APPROVED_VERSION` gate implemented and active. Exit: (1) `cargo build --release --locked` exits 0 on a fresh clone; (2) `enforce_provenance_gate()` panics or refuses when attestation set is empty; (3) `TBTC_SIGNER_MIN_APPROVED_VERSION` set to current version gates startup; (4) Provenance gate logs proof on cold-start. |
| `P0-M3` Signing policy firewall v1 | Rule engine for allowed transaction/script classes, value/rate/time-window controls, policy decision logging | Protocol + Security | Unauthorized signing requests are rejected pre-signing with auditable policy events |
| `P0-M4` Telemetry + SLO baseline | Attempt-level metrics, signer/coordinator health metrics, alert thresholds, weekly ops report template | Platform + Ops | Dashboard and alerts cover attempt success, latency, and policy reject/fault rates |

### Phase P1 (Weeks 6-12): Accountability + Liveness Hardening

| Milestone | Deliverables | Primary owners | Exit criteria |
| --- | --- | --- | --- |
| `P1-M1` Accountable ROAST transcripts | Attempt transcript hashing/signing, evidence persistence, verifier for blame proofs (equivocation/non-participation) | Protocol + Security | Faults can be proven and attributed from persisted evidence |
| `P1-M2` Reputation + auto-quarantine | Operator scoring model (latency/fault/policy violations), auto-exclusion thresholds, manual DAO re-enable path | Ops + Governance + Protocol | Repeatedly faulty operators are automatically excluded within one policy epoch |
| `P1-M3` Active-active coordinators + anti-DoS transport limits | Coordinator failover protocol, authenticated transport budget/rate limits, replay-resistant request envelopes | Protocol + Platform | Coordinator loss does not halt signing; abuse load is rate-limited without breaking healthy flow |
| `P1-M4` Chaos and fault-injection program | Monthly drills (coordinator crash, signer loss, partition, stale attempt replay), drill runbook, corrective action tracker | Ops + Security | Drills run on schedule and unresolved critical findings block promotion |

### Phase P2 (Weeks 12-20): Lifecycle + Deployment Safety

| Milestone | Deliverables | Primary owners | Exit criteria |
| --- | --- | --- | --- |
| `P2-M1` Periodic key refresh/reshare cadence | Reshare policy and tooling, no-address-rotation proof points, emergency rekey path | Protocol + Ops | Refresh is repeatable and does not violate wallet continuity invariants |
| `P2-M2` Implementation diversity + differential fuzzing | Independent verification path or secondary implementation checks, differential/fuzz harnesses, divergence triage workflow | Security + Protocol | Differential CI runs continuously with no unresolved critical divergence |
| `P2-M3` Canary rollout + instant rollback controls | 10%-50%-100% rollout policy, signer cohort canaries, one-command rollback and config pinning | Platform + Ops | Canary progression is automated by SLO gates; rollback is validated under incident drill |

## Acceptance Test Catalog

### P0 acceptance tests

- `AT-P0-01` Admission checks enforce policy:
  onboarding fails for provider-diversity violation, missing attestation, or
  expired patch SLA; compliant operator passes.
- `AT-P0-02` Provenance gate is fail-closed:
  signer startup exits non-zero with untrusted provenance; starts normally with
  approved provenance and version floor.
- `AT-P0-03` Policy firewall blocks unauthorized sign requests:
  disallowed script/value/rate requests are rejected with stable reason codes;
  canonical mint/redeem vectors pass.
- `AT-P0-04` Observability baseline exists:
  load scenario (100+ attempts) produces metrics for success, p95 latency,
  policy rejects, and coordinator failovers; alerts trigger on threshold breach.

### P1 acceptance tests

- `AT-P1-01` Transcript accountability:
  injected equivocation/non-participation faults produce verifiable blame
  proofs and operator attribution.
- `AT-P1-02` Quarantine automation:
  operator crossing fault threshold is auto-excluded within one epoch and
  cannot rejoin without explicit governance action.
- `AT-P1-03` Coordinator resilience:
  primary coordinator kill during attempt triggers standby takeover within SLO
  without double-signing or orphaned finalize.
- `AT-P1-04` Abuse resistance:
  high-rate malformed request traffic is dropped by limits while nominal flows
  remain within target latency budget.

### P2 acceptance tests

- `AT-P2-01` Refresh continuity:
  scheduled reshare completes without wallet identity drift and without
  violating signing availability SLO.
- `AT-P2-02` Differential safety:
  fuzz corpus and deterministic vectors run across both implementations/checkers
  with no unresolved critical divergence.
- `AT-P2-03` Upgrade safety:
  canary promotion halts automatically on SLO breach; rollback restores prior
  cohort config within declared recovery objective.

## Governance and Operational Decisions Needed

1. DAO ratifies minimum operator requirements and enforcement policy.
2. DAO ratifies automatic quarantine thresholds and override process.
3. Security owner ratifies provenance trust roots and signing keys.
4. Ops owner ratifies SLO targets used as rollout and rollback gates.

## Evidence and Reporting

Milestone evidence should be attached to the relevant implementation PRs or
release/governance records. This repository should retain durable design and
runbook material, not per-run packet scaffolds or raw execution logs.

## Resourcing Estimate

- Protocol/runtime: 2 engineers
- Platform/DevOps: 1 engineer
- Security/review: 0.5 FTE equivalent
- Operations/governance support: 0.5 FTE equivalent

Estimated timeline with parallel workstreams: 4-5 months.

## Open Questions

1. Should operator diversity constraints be hard requirements or weighted score
   factors at admission time?
2. Which components are mandatory for provenance enforcement in v1
   (binary-only vs binary+config bundles)?
3. Do we require dual approval for manual quarantine overrides?
