# True Late t-of-n Finalize: Considerations And Tradeoffs

Date: 2026-02-27
Status: Discussion draft (future consideration only, not a committed delivery item)
Scope: Rust `tbtc-signer` + keep-core FROST orchestration

## 1. Context

Current signer posture supports:

- early subset selection via `StartSignRound.signing_participants`,
- real finalize over the exact round signing cohort,
- fail-closed replay and nonce lifecycle controls for that model.

It does **not** support true late subset selection at finalize time after shares
have already been produced for a larger cohort.

## 2. What "True Late t-of-n Finalize" Means

Given:

- DKG participant set `P` with `|P| = n`,
- threshold `t`,
- a round started for some eligible cohort `C` where `t <= |C| <= n`,

true late finalize means the coordinator can finalize using any responding
subset `S` such that:

- `S ⊆ C`,
- `|S| >= t`,
- finalize aggregates only shares/commitments from `S`,
- no full round restart is required purely because some members in `C` did not
  respond.

## 3. Why Consider It

Potential benefits:

- Better liveness under mid-round signer drop-off.
- Lower tail latency in degraded network conditions.
- Fewer full round restarts and lower coordination churn.
- Reduced wasted work when only a few signers fail late.

## 4. Tradeoffs And Costs

### 4.1 Security And Nonce Lifecycle Complexity

- Nonce safety reasoning becomes more complex because commitments may be
  produced for a broader cohort than final contributors.
- Replay/idempotency semantics must bind finalize to an exact subset `S`,
  exact commitment map, and exact message context.
- Subset-selection policy mistakes could accidentally widen acceptance in ways
  that are hard to audit.

### 4.2 State And Persistence Complexity

- Need richer durable round state (commitments, candidate contributors,
  contribution receipts, subset decision metadata).
- Larger persisted state and more edge cases around restart/reload recovery.
- More crash-matrix branches (subset chosen before/after partial persistence,
  partial contribution arrival, retry storms).

### 4.3 Coordinator And Retry Semantics

- Coordinator must choose subset policy: earliest responders, deterministic
  ranking, stake/priority policy, etc.
- Must ensure deterministic behavior under retries, restarts, and duplicate
  finalize requests.
- Attempt accounting in keep-core becomes more complex (round attempt vs subset
  attempt semantics).

### 4.4 Cross-Component Interface Impact

- Rust FFI request/response models likely need changes.
- keep-core native bridge operation contracts need updates.
- Integration tests in Go and Rust must expand materially.

### 4.5 Review And Operational Burden

- Larger security-review surface across nonce safety, replay safety, and
  persistence semantics.
- More observability requirements to debug subset decisions in production.
- More complex incident triage when finalize behavior differs across attempts.

## 5. Design Alternatives

### Current Early-Subset Model

- Subset fixed at `StartSignRound`.
- Finalize requires exact cohort alignment.
- Lowest complexity, strongest determinism, easiest audit posture.

### Full True Late t-of-n Finalize

- Start with broader eligible cohort.
- Finalize accepts any valid subset `S` with `|S| >= t`.
- Highest liveness upside, highest engineering/review complexity.

### Hybrid Bounded Late-Subset

- Allow late subset only under strict bounded policy (for example:
  deterministic fallback from `C` to canonical subset `S` once).
- Attempts to capture some liveness gains while limiting policy explosion.
- Still significantly more complex than the current early-subset model.

## 6. Required Changes If Implemented

## 6.1 Rust Signer Engine

- Extend round state to track:
  - full eligible cohort `C`,
  - commitment map keyed by participant,
  - accepted contribution set,
  - finalize subset decision metadata.
- Finalize must:
  - validate subset policy and cardinality (`|S| >= t`),
  - bind replay/idempotency fingerprints to subset-specific payload shape,
  - aggregate using commitments + shares restricted to `S`.
- Update replay guards for subset-sensitive finalize requests.

## 6.2 keep-core Integration

- Bridge payloads must carry enough structure for subset-finalize semantics.
- Signing orchestration must define deterministic subset selection policy.
- Retry logic must distinguish:
  - new round attempt,
  - subset adjustment within same round context.

## 6.3 Persistence And Crash Matrix

- Add tests for:
  - restart before subset selection,
  - restart after subset decision but before finalize cache persistence,
  - replay of old finalize payload with different subset,
  - idempotent retries for same subset payload.

## 6.4 Audit And Observability

- Add telemetry for:
  - eligible cohort size,
  - selected subset size/identifiers,
  - fallback reason (drop-off vs timeout vs policy).
- Add security-review packet specifically for subset/replay/nonce invariants.

## 7. Threat-Model Considerations

- Adversarial partial responders can influence which subset is chosen unless
  policy is carefully deterministic and bias-resistant.
- Coordinator bugs become higher impact because subset choice affects signature
  validity path and retry behavior.
- Any ambiguity in subset binding increases replay/confusion risk.

## 8. Testing Expectations

Minimum evidence bar if implemented:

- Unit tests:
  - subset validation matrix (`|S| < t`, `|S| = t`, `|S| > t`),
  - replay/idempotency for same subset vs changed subset,
  - nonce/reuse invariant preservation.
- Integration tests:
  - drop-off liveness scenarios without full restart,
  - restart/reload with in-flight subset selection,
  - keep-core/Rust bridge consistency.
- Adversarial tests:
  - duplicate/forged contributor identifiers,
  - subset oscillation across retries,
  - malformed subset ordering/canonicalization attempts.

## 9. Rollout Approach If Pursued

- Phase 0: design RFC and threat-model review.
- Phase 1: hidden implementation behind explicit runtime gate.
- Phase 2: CI + stress/fault testing with gate off in production.
- Phase 3: limited canary with heavy telemetry and rollback plan.
- Phase 4: broader rollout only after external review sign-off.

## 10. Recommendation For Current Program

For the current migration timeline, keep true late t-of-n finalize as
**future consideration only**.

Rationale:

- current implementation already supports early subset selection and achieves
  required migration path with lower risk,
- true late finalize adds substantial cross-stack complexity and review scope,
- immediate priority is closing existing production gates with strong evidence.

## 11. Decision Triggers To Revisit

Reopen this item if one or more are true:

- observed production liveness/latency pain from full-round restarts,
- clear SLO target cannot be met with early subset model,
- dedicated review bandwidth is available for protocol + persistence expansion,
- rollout risk budget explicitly includes the added complexity.
