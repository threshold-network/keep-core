# ROAST Phase 0 Spec Freeze

Date: 2026-02-27
Status: Draft (for signer + keep-core owner approval)
Owner: Threshold Labs
Scope: canonical attempt-context contract and coordinator semantics for ROAST migration

## 1. Purpose

Freeze the minimum cross-repo contract required before ROAST code increments:

- attempt identity fields,
- deterministic hashing/domain separation,
- coordinator authorization semantics,
- fail-closed error taxonomy.

This spec is an input to Phase 1 implementation work in:

- `tbtc` (`pkg/tbtc/signer`), and
- `threshold-network/keep-core`.

## 2. Decisions (Frozen For Phase 1)

1. Attempt context is mandatory in strict ROAST mode for sign/finalize flow.
2. Attempt context is canonicalized identically in Rust and Go before hashing.
3. `attempt_id` is deterministic and transcript-bound.
4. Coordinator authority is enforced (not informational-only).
5. Stale or replayed attempt payloads fail closed.
6. Attempt advancement (`N -> N+1`) must be authorized by policy-defined
   evidence; no single actor can force advancement unilaterally.
7. Pre-ROAST persisted sessions under strict mode require explicit migration
   behavior (recommended fail-closed requiring session restart).

## 3. Attempt Context Contract

Attempt context fields:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `attempt_number` | `u32` | yes | 1-based monotonic per logical signing flow |
| `coordinator_identifier` | `u16` | yes | member identifier selected for this attempt |
| `included_participants` | `Vec<u16>` | yes | sorted unique non-zero participant IDs |
| `included_participants_fingerprint` | `hex(sha256)` | yes | canonical hash of included set |
| `attempt_id` | `hex(sha256)` | yes | canonical transcript identifier |

Validation rules:

- `attempt_number >= 1`.
- `included_participants` must be non-empty, unique, sorted ascending, and
  include `coordinator_identifier`.
- `included_participants.len() >= threshold`.
- `attempt_id` and `included_participants_fingerprint` must match recomputed
  canonical values.

## 4. Canonical Hashing Rules

Domain separation tags:

- `FROST-ROAST-INCLUDED-FPR-v1`
- `FROST-ROAST-ATTEMPT-ID-v1`

Canonical framing:

- Length-prefixed binary framing for every component:
  `len(component_u32_be) || component_bytes`.
- Integers encoded big-endian fixed width (`u16`, `u32`).
- Session/message identifiers encoded as raw bytes after strict validation.

Included participants fingerprint:

- `included_participants_fingerprint = SHA256(tag || framed(sorted_unique_ids))`

Attempt id:

- `attempt_id = SHA256(tag || framed(session_id) || framed(message_digest_hex) || framed(attempt_number) || framed(coordinator_identifier) || framed(included_participants_fingerprint))`

Output format:

- Lowercase hex string, no prefix.

## 5. Coordinator Semantics

- keep-core computes deterministic coordinator for each attempt using existing
  ROAST-style selection policy.
- Native signer validates that payload coordinator matches attempt context and
  included participants.
- Requests from non-authorized coordinator context are rejected in strict mode.
- Coordinator authorization applies per attempt; retries require new valid
  attempt context.

## 6. Attempt Transition Authorization

- Transition from attempt `N` to `N+1` requires policy-defined authorization
  evidence (for example quorum timeout evidence, blame evidence, or equivalent
  signed transition proof).
- Coordinator selection for `N+1` is deterministic but does not by itself
  authorize transition; authorization evidence is required.
- Future-attempt requests lacking valid transition authorization are rejected
  fail-closed.

## 7. Request Surface Changes (Phase 1 Input)

The following request families gain `attempt_context` in strict ROAST mode:

- `StartSignRound`
- `FinalizeSignRound`

Transitional gating:

- `TBTC_SIGNER_ENABLE_ROAST_STRICT=true` (or equivalent feature gate) enables
  strict enforcement.
- While disabled, attempt context may be accepted but is not mandatory.
- Pre-ROAST persisted sessions in strict mode follow explicit migration
  behavior (recommended fail-closed requiring session restart under new
  attempt-context-aware session).

## 8. Error Taxonomy (Fail-Closed)

Proposed stable codes:

| Code | Meaning |
| --- | --- |
| `attempt_context_missing` | required attempt context absent in strict mode |
| `attempt_context_invalid` | malformed fields or canonicalization violation |
| `attempt_id_mismatch` | provided attempt id differs from recomputed value |
| `coordinator_mismatch` | coordinator does not match authorized attempt context |
| `attempt_stale` | attempt number older than active/known session attempt |
| `attempt_future` | attempt number is ahead of local state without valid transition authorization |
| `attempt_transition_unauthorized` | attempt advancement evidence invalid/missing |
| `attempt_replay` | attempt id already consumed for this transcript |
| `attempt_conflict` | same session retry with materially different attempt context |
| `attempt_exhausted` | retry policy limit reached |
| `pre_roast_session_unsupported` | strict mode rejects session persisted without required ROAST attempt context |

Mapping guidance:

- Rust signer returns structured engine errors mapped to these stable codes at
  FFI boundary.
- keep-core treats `attempt_stale`, `attempt_replay`, `coordinator_mismatch`,
  `attempt_id_mismatch`, and `attempt_transition_unauthorized` as non-retriable
  for that attempt payload.

## 9. Replay, Restart, And Concurrency Invariants

1. Attempt id is single-use for a given `(session_id, message, cohort)` flow.
2. Stale attempts (lower `attempt_number`) are rejected after higher attempt is
   accepted.
3. Future attempts (`attempt_number > current`) are accepted only when
   transition authorization is valid; otherwise rejected fail-closed.
4. Persisted state reload/restart must preserve replay/stale-attempt rejection.
5. Bounded attempt registries must fail closed when capacity is reached (no
   eviction policy that weakens replay protection).

## 10. Consumed Registry Integration Notes

- Existing `round_id`-based consumed registries remain authoritative for
  nonce/single-use protection.
- `attempt_id` tracking is additive for coordinator/attempt-transition replay
  semantics (not a silent replacement of `round_id` protections).
- Cap policy for attempt registries vs existing consumed registries must be
  explicitly documented in Phase 1.5 implementation work.

## 11. Threat Model Notes

- Malicious coordinator:
  prevented from unilateral attempt advancement by transition authorization and
  coordinator-context validation.
- Malicious participant:
  exclusion requires policy-defined evidence; not based on unverified claims.
- Network adversary:
  replay/reorder/delay is constrained by attempt id, attempt number, and
  transition-authorization validation.
- Corrupt persisted state:
  stale/future/replay acceptance must remain fail-closed after restart/reload.

## 12. Out Of Scope For Phase 0

- Full liveness policy tuning (timeouts/backoff policy details).
- True late t-of-n finalize semantics.
- DKG architecture redesign.

## 13. Approval Checklist

Required approvers:

- signer owner,
- keep-core owner,
- security reviewer.

Sign-off criteria:

1. Both implementations can produce identical hashes for test vectors.
2. No ambiguity remains about coordinator authorization checks.
3. Error taxonomy is stable enough for integration tests and telemetry.
4. Strict-mode gate behavior is explicitly defined.
5. Transition-authorization behavior is specified for stale/future attempts.
6. Migration behavior for pre-ROAST persisted sessions is explicitly chosen.
