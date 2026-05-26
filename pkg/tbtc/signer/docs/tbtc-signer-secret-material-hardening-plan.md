# tbtc-signer Secret Material Hardening Plan (Long-Term)

Date: 2026-03-01
Status: Proposed (pre-implementation)
Owner: Threshold Labs
Scope: `tools/tbtc-signer` persistent secret-material handling before FROST/ROAST
production rollout.

## Decision

Adopt the long-term hardening path:

1. Option 3: secret-aware in-process material handling and serialization
   boundaries.
2. Option 4A: encrypted-at-rest state envelope as default.
3. Option 4B: KMS/HSM-backed key provider integration as a pre-production
   gate.

Rationale:
- FROST/ROAST is not yet deployed to production.
- This window allows deeper correctness/security work before operational lock-in.
- It addresses the remaining audit concern around transient plaintext exposure
  more directly than incremental zeroization alone.

## Security Goals

1. Reduce plaintext lifetime of key material in process memory.
2. Eliminate plaintext-at-rest signer-state payloads by default.
3. Preserve restart/idempotency/replay invariants already established in ROAST
   phases.
4. Maintain fail-closed behavior for corrupt/missing/invalid encrypted state.

## Non-goals

1. Replacing the signer protocol or FROST/ROAST message semantics.
2. Requiring TEEs as a prerequisite for deployment.
3. Immediate mandatory KMS/HSM dependency for local/dev environments.

## Architecture Overview

### A. In-Process Secret Boundary (Option 3)

- Introduce secret-wrapper types for sensitive payloads (for example serialized
  key packages and signing message bytes) so accidental copies are minimized and
  explicit extraction is required.
- Keep persisted wire structs separate from runtime secret structs to avoid
  broad serde exposure.
- Centralize encode/decode in one `state_codec` boundary that:
  - decodes into temporary buffers,
  - converts to secret wrappers,
  - zeroizes intermediate decode/encode buffers,
  - avoids returning secret-bearing `String` values where possible.

### B. Encrypted-at-Rest Envelope (Option 4A)

- Replace plaintext JSON state file payload with:
  - small plaintext header (schema, algorithm, key-provider metadata, nonce),
  - authenticated ciphertext containing serialized state payload.
- Default behavior: encrypted state required unless explicitly in developer
  compatibility mode.
- Keep atomic write durability pattern (temp file -> fsync -> rename -> dir
  fsync) and state lock semantics unchanged.
- Fix cryptography baseline for implementation:
  - AEAD: `XChaCha20-Poly1305` (`xchacha20poly1305`).
  - Nonce: 192-bit random value from OS CSPRNG for each write.
  - Nonce policy: never reuse a nonce with the same key; do not use counters.
  - Authentication tag: stored separately from ciphertext for explicit envelope
    validation.

Suggested envelope fields:
- `schema_version`
- `encryption_algorithm`
- `key_provider`
- `key_id` (opaque)
- `nonce`
- `ciphertext`
- `authentication_tag`

## Key Management Strategy

### Phase 1 default provider

- Env-backed key provider (`TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX`) for
  controlled dev/test and pre-production environments only.
- Key must be exactly 32 bytes (64 hex chars); missing, truncated, or invalid
  key is fail-closed with startup abort and stable diagnostic output.
- Env provider is not an acceptable long-term production default.

### Production provider requirement (Option 4B)

- Provider trait allows later KMS/HSM integration without state-format redesign.
- KMS/HSM-backed provider is required before production FROST/ROAST rollout.
- KMS/HSM key retrieval and rotation semantics are a gated increment after P2.

## Migration Plan

1. Add schema version for encrypted envelope while retaining read compatibility
   for legacy plaintext state.
2. On successful plaintext load:
  - decode with existing path,
  - persist back in encrypted format atomically.
3. Add one-way migration guardrails:
  - fail-closed on mixed/corrupt envelope metadata,
  - explicit diagnostic logging for migration state.
4. Add rollout flag to temporarily permit plaintext for emergency rollback in
   non-production profiles only; compile-time disabled in release builds.

## Phased Work Breakdown

### P0 (Week 1-2): Secret-boundary refactor

- Introduce secret wrapper types and centralized state codec module.
- Refactor `PersistedKeyPackage` handling to avoid broad `String` secret spread.
- Preserve behavior and test parity.

Exit criteria:
- Existing restart/idempotency/replay tests pass unchanged.
- New tests verify intermediate buffer zeroization in codec paths.

### P1 (Week 3-4): Encrypted envelope + migration

- Add encrypted envelope schema and codec.
- Add env key provider and strict fail-closed startup behavior.
- Implement plaintext->encrypted migration on first successful load.
- Enforce nonce generation/reuse invariants in codec paths.

Exit criteria:
- No plaintext payload persisted in normal mode.
- Corruption/missing-key cases fail closed with stable error diagnostics.
- Crash-matrix persists encrypted state safely.
- Encryption algorithm and envelope fields are fixed and implemented as specified
  in this plan.

### P2 (Week 5-6): Operational hardening and review closure

- Add key-rotation operational docs and runbook hooks, including secure key
  provisioning for non-production env-provider deployments.
- Add chaos tests for key unavailability, malformed envelope, and migration
  interruptions.
- Complete independent adversarial review and remediation cycle.

Exit criteria:
- Security review recommendation: GO or Conditional GO with no unresolved
  CRITICAL/HIGH findings.
- Runbook and approval records updated with encrypted-state controls.

### P3 (Week 7+): KMS/HSM provider integration (required pre-production gate)

- Implement provider adapter(s) for selected KMS/HSM.
- Add bootstrap and outage-handling runbooks.
- Validate key-rotation and recovery procedures in staging.

Exit criteria:
- Production profile does not permit env-backed encryption key provider.
- KMS/HSM path passes restart/idempotency/replay and fail-closed test suites.

## Test Matrix

1. Unit tests:
  - codec encode/decode roundtrip for encrypted schema,
  - key/provider validation failures,
  - migration from plaintext schema,
  - zeroization behavior of temporary buffers.
2. Integration tests:
  - restart/reload across encrypted-state writes,
  - fail-closed startup on missing/invalid encryption key,
  - replay/idempotency invariants unchanged after migration.
3. Chaos tests:
  - crash between encrypt and rename,
  - crash after rename before directory sync,
  - malformed envelope metadata/ciphertext tampering.

## Rollout and Risk Controls

1. Ship behind explicit feature gate, then flip to default-encrypted mode before
   production FROST/ROAST rollout.
2. Preserve emergency rollback path for non-production testing only.
3. Require canary validation on:
  - startup reliability,
  - state reload success rate,
  - signer latency delta vs baseline.
4. Require at least 72h canary soak with zero state-reload failures before
   enabling production encrypted-state defaults.

## Remaining Decisions

1. Final KMS/HSM implementation target(s) and provider adapter priority.
2. Rotation cadence and key-ID lifecycle policy for production operations.
