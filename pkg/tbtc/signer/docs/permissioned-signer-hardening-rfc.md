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

### P0-M3 Rate-Limit Specification (Decision 7 extension)

The P0-M3 milestone introduced per-call value/script-class rate
controls for the signing path (`BuildTaprootTx` and its peers),
mounted via the signing policy firewall. Decision 7 of the PR #4005
review extends the same rate-limit discipline to the interactive
session entry points — `InteractiveSessionOpen` and
`InteractiveRound1` — because both are the canonical per-operator
amplifiers: a hostile or misconfigured operator can otherwise
inflate attempt throughput on a single key group without bound.
The shape mirrors the existing `BuildTaprootTx` rate-limit
configuration (token-bucket refill, env-var tunable, fail-closed
rejection) but uses two buckets per operation rather than one, so
that the per-caller and per-key-group budgets are independently
observable.

#### `InteractiveSessionOpen` rate-limit buckets

| Bucket | Scope | Trigger | Reason code on exhaustion | Env-var knob | Default |
| --- | --- | --- | --- | --- | --- |
| Primary | per-`(sender, key_group)`, includes the attempt-context fingerprint so each fresh attempt has independent budget (replay protection) | `InteractiveSessionOpen` for a given `(sender, key_group)` exceeds the budget | `interactive_rate_limit_exceeded` | `TBTC_SIGNER_INTERACTIVE_OPEN_RATE_LIMIT_PER_MINUTE` | 60/min |
| Cross-operator | per-`(member, key_group)`, aggregates across attempts to bound a member's effective work rate on a given wallet | Sum of Open calls for a `(member, key_group)` exceeds the cross-operator cap | `interactive_cross_operator_cap_exceeded` | `TBTC_SIGNER_INTERACTIVE_OPEN_CROSS_OPERATOR_CAP_PER_MINUTE` | 5/min |

Both buckets are enforced at `InteractiveSessionOpen` (charged in order:
primary bucket first, then cross-operator cap), implemented by
`enforce_interactive_open_rate_limit` and
`enforce_interactive_open_cross_operator_cap` in `src/engine/policy.rs`.
The primary bucket is the per-caller throttle; the cross-operator cap is
the per-`(member, key_group)` cap that prevents a single operator from
inflating the effective budget by rotating `sender` identifiers or
attempt contexts.

#### `InteractiveRound1` rate-limit bucket

`InteractiveRound1` has its own independent primary bucket — it does
NOT reuse the Open cross-operator bucket. Implemented by
`enforce_interactive_round1_rate_limit` in `src/engine/policy.rs`.

| Bucket | Scope | Trigger | Reason code on exhaustion | Env-var knob | Default |
| --- | --- | --- | --- | --- | --- |
| Primary | per-`(sender, key_group)`, includes the attempt-context fingerprint | `InteractiveRound1` for a given `(sender, key_group)` exceeds the budget | `interactive_round1_rate_limit_exceeded` | `TBTC_SIGNER_INTERACTIVE_ROUND1_RATE_LIMIT_PER_MINUTE` | 60/min |

There is no separate cross-operator cap on `InteractiveRound1` today —
only the per-`(sender, key_group)` primary bucket. A cross-operator cap
on Round1 (mirroring Open's) is a candidate future hardening, not
implemented in Decision 7.

#### Operator knobs and defaults

All three knobs (`TBTC_SIGNER_INTERACTIVE_OPEN_RATE_LIMIT_PER_MINUTE`,
`TBTC_SIGNER_INTERACTIVE_OPEN_CROSS_OPERATOR_CAP_PER_MINUTE`,
`TBTC_SIGNER_INTERACTIVE_ROUND1_RATE_LIMIT_PER_MINUTE`) are env-var
tunable, follow the existing `TBTC_SIGNER_*_ENV` pattern from
`src/engine/config.rs`, and are parsed with the same bounded-parse
rule as the rest of the policy surface. Defaults: 60/min for both
primary buckets, 5/min for the cross-operator cap — deliberately much
tighter than the primary bucket, since a compromised or misbehaving
operator rotating `sender`/attempt identities is the threat this cap
specifically targets. Each rejection emits a structured policy event
with the `(sender, key_group)` / `(member, key_group)` tuple and the
active bucket state at the time of rejection; the rejection is
fail-closed (no exception carve-out for the host). All rate-limit
state is process-local (in-memory token buckets) and resets on signer
restart — it is never persisted or durable.

#### Cumulative-rejection budget (Decision 8)

The P0-M3 rate-limit discipline is independent of the
`wallet_deadline_exceeded` terminal error class from Decision 8.
Rate-limit rejections consume the per-(sender, key_group) and
cross-operator buckets but do NOT consume the wallet-level attempt
budget — a flood of rejects from a hostile operator therefore does
not exhaust the wallet's deadline. The wallet-level deadline is
only consumed by attempts that were admitted by all rate-limit
buckets and produced a signing-flow outcome. This separation is
load-bearing: keeping the rate-limit and the wallet-level deadline
independent means a rate-limited operator cannot DoS the wallet's
attempts by spending the deadline on its own rejections.

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
