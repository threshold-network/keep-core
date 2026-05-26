# ROAST Phase 1.5: Consumed-Registry Integration

Date: 2026-02-27
Status: Signer-side complete (keep-core compatibility confirmed at contract level)
Owner: Threshold Labs
Scope: bind ROAST attempt identity to signer round identity before coordinator-policy phases

## Objective

Define and start implementing how `attempt_id` interacts with signer
single-use/replay keys (`round_id`) so later ROAST phases can support
multi-attempt semantics without weakening nonce/replay protections.

## Decisions Implemented In This Increment

1. `round_id` is now derived with an explicit attempt component.
2. When `attempt_context` is present, the attempt component is
   `attempt_context.attempt_id` canonicalized to lowercase.
3. When `attempt_context` is absent, a stable sentinel (`none`) is used to
   preserve deterministic round-id derivation for legacy/non-strict flow.
4. `attempt_context` fingerprint canonicalization now lowercases
   `included_participants_fingerprint` and `attempt_id` to avoid false
   idempotency conflicts from hex case variance.

## Rationale

- Keeps existing `round_id`-bound nonce-safety model intact while making round
  identity attempt-aware.
- Avoids mixed-case hex drift between validation (`eq_ignore_ascii_case`) and
  idempotency fingerprinting.
- Preserves backward compatibility for non-strict mode by keeping deterministic
  round-id behavior when attempt context is omitted.

## Evidence (Code + Tests)

- Round-id derivation helper:
  `tools/tbtc-signer/src/engine.rs` (`derive_round_id`,
  `round_attempt_id_component`).
- Attempt-context canonicalization fix:
  `tools/tbtc-signer/src/engine.rs` (`canonicalize_attempt_context_for_fingerprint`).
- Hash golden vectors:
  `engine::tests::roast_attempt_context_hash_vectors_match_expected_values`.
- Round-id attempt binding test:
  `engine::tests::derive_round_id_binds_attempt_id_case_insensitive_component`.
- Case-variant idempotent retry test:
  `engine::tests::start_sign_round_accepts_hex_case_variant_attempt_context_idempotent_retry`.
- Consumed sign-round registry capacity with attempt context:
  `engine::tests::start_sign_round_rejects_when_consumed_sign_round_registry_is_at_capacity_with_attempt_context`.
- Consumed finalize request-fingerprint registry capacity with attempt context:
  `engine::tests::finalize_sign_round_rejects_when_consumed_request_registry_is_at_capacity_with_attempt_context`.
- Consumed finalize round-id registry capacity with attempt context:
  `engine::tests::finalize_sign_round_rejects_when_consumed_round_registry_is_at_capacity_with_attempt_context`.

## Compatibility Confirmation

- Signer request/response contract remains backward compatible for keep-core:
  `attempt_context` is optional at the API layer and strictness is controlled by
  `TBTC_SIGNER_ENABLE_ROAST_STRICT`.
- Replay and nonce single-use guards remain additive and fail-closed:
  round-id consumed registries remain authoritative, and `attempt_context` is
  folded into round identity instead of replacing existing guards.
- Existing keep-core retry/cohort wiring evidence remains valid under this
  model (see `docs/frost-migration/rust-rewrite-bootstrap.md` keep-core
  integration notes and linked `threshold-network/keep-core` commits).

## Remaining Work

1. Continue Phase 2 coordinator policy enforcement:
   `docs/frost-migration/roast-phase-2-coordinator-policy-enforcement.md`.
