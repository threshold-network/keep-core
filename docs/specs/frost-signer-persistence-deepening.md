---
title: FROST signer — carve persistence into seams
date: 2026-08-18
status: draft
tags: [frost-signer, architecture, deepening, security, persistence]
---

# FROST signer — carve persistence into seams

## Problem Statement

The `pkg/tbtc/signer/src/engine/persistence.rs` module is 2,389 lines containing four orthogonal concerns that share one file:

1. **Schema codec** — `TryFrom<PersistedEngineState>` / `TryFrom<PersistedSessionState>` and the inverse flatten/projection. Versioned, schema-version-validated, hex-decoded, FROST key-package-deserialized on load. ~600 lines.
2. **Envelope I/O** — file open, lock acquisition, atomic rename, directory sync, corrupt-state recovery, backup retention. ~400 lines.
3. **Key providers** — env/command subprocess, pipe draining, kill, timeout, stdio redirection. ~500 lines.
4. **Pending-operation registry** — marker tracking, snapshot covering, durable retry on next state-lock acquisition. ~400 lines.

The four concerns are independently testable but currently require each other in the test environment. A test of the schema codec needs the file I/O scaffolding; a test of the key-provider subprocess needs the codec to assemble fake state. The seam between concerns is invisible — and the highest-stakes functions in the codebase (AEAD envelope verification, key-provider subprocess management, schema-version migration) are the ones that most need clean per-concern test surfaces.

The blast radius is significant: the persistence layer is the security boundary for secret-bearing material (the state encryption key, the FROST key packages, the signing session markers). Bugs in any of the four concerns have the same severity class and the same debugging difficulty.

## Solution

Split `engine::persistence` into four `pub(crate)` modules with intentionally narrow interfaces. Each module owns one concern; each interface is small enough that a future maintainer can hold the whole interface in their head.

```
engine::persistence::schema_codec   // pure: encode/decode/TryFrom, version + schema validation
engine::persistence::envelope_io   // file I/O, lock, atomic rename, corrupt recovery, backup retention
engine::persistence::key_provider  // trait + 3 adapters (env, command, process-lifetime cache) + subprocess machinery
engine::persistence::pending_ops   // registry + snapshot covering + durable retry
```

The trait `StateKeyProvider` is the test seam for the key-provider machinery: tests define their own lightweight `StateKeyProvider` impls locally (the existing `tests.rs` pattern is `TrackingMockKeyProvider` + a `StdArcMockProvider` newtype wrapper, exercised directly or through `CachedStateKeyProvider`) rather than a canned fake shipped by the module. Production wires `EnvKeyProvider` or `CommandKeyProvider`, both wrapped in `CachedStateKeyProvider`, based on the `TBTC_SIGNER_STATE_KEY_PROVIDER` env value.

The persisted schema (`PersistedEncryptedEngineStateEnvelope`, `PersistedEngineState`, `PersistedSessionState`, `PersistedKeyPackage`) is unchanged. The on-disk file format is byte-for-byte stable.

## User Stories

1. As a maintainer, I want to test the schema codec without spinning a temp file so that the schema-version migration logic is testable in isolation.
2. As a maintainer, I want to test the key-provider subprocess machinery without involving the schema codec so that a pipe-drain bug can be fixed without touching the FROST serialization.
3. As a maintainer, I want to test the pending-ops registry against a HashMap-backed fake state so that the snapshot-covering logic is exercisable without a real state file.
4. As a security reviewer, I want the AEAD envelope code in one module so that I can audit the cryptographic boundary in one sitting.
5. As a security reviewer, I want the key-provider subprocess management in one module so that I can audit the KMS/HSM integration in one sitting.
6. As a maintainer, I want the four modules to have importable interfaces (small enough to read in one go) so that adding a new operation (e.g. a future multiplexing key provider) is a contained change.
7. As a test author, I want to implement `StateKeyProvider` directly with a local test double (as `TrackingMockKeyProvider` already does) so that the test environment does not need a real KMS/HSM subprocess for command-provider tests.
8. As a Go host integrator, I want the on-disk file format unchanged so that the existing state files continue to load after the refactor.

## Implementation Decisions

### Module layout

The new modules live under `pkg/tbtc/signer/src/engine/persistence/`. The existing `persistence.rs` becomes a `mod.rs` that re-exports the four submodules. The glob re-export pattern in `engine/mod.rs` (`pub(crate) use persistence::*;`) keeps the existing call sites working without per-call-site import churn.

```
pkg/tbtc/signer/src/engine/persistence/
├── mod.rs                    // re-exports the four submodules + the legacy top-level fns that orchestrate them
├── schema_codec.rs           // pure encode/decode/TryFrom + schema-version validation
├── envelope_io.rs            // file I/O, lock, atomic rename, corrupt recovery, backup retention
├── key_provider.rs           // StateKeyProvider trait + EnvKeyProvider + CommandKeyProvider + subprocess machinery
└── pending_ops.rs            // registry types + snapshot covering + durable retry
```

### `schema_codec` interface

```rust
pub(crate) fn encode_state(state: &EngineState) -> Result<PersistedEngineState, EngineError>;
pub(crate) fn decode_state(persisted: PersistedEngineState) -> Result<EngineState, EngineError>;
pub(crate) fn encode_session(session: &SessionState) -> Result<PersistedSessionState, EngineError>;
pub(crate) fn decode_session(persisted: PersistedSessionState) -> Result<SessionState, EngineError>;
pub(crate) const PERSISTED_STATE_SCHEMA_VERSION: u16;     // moved here
pub(crate) const PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION: u16;  // moved here
```

The two `TryFrom` impls (`PersistedEngineState → EngineState` and `PersistedSessionState → SessionState`, plus their inverses) move into `schema_codec.rs` as concrete functions. The `From` / `TryFrom` traits are NOT implemented on the new module's functions — the function-call form is preferred for testability.

### `envelope_io` interface

```rust
pub(crate) fn load_engine_state_from_storage() -> Result<EngineState, EngineError>;
pub(crate) fn persist_engine_state_to_storage(state: &EngineState) -> Result<(), PersistEngineStateError>;
pub(crate) fn recover_or_fail_from_corrupted_state_file(...) -> Result<EngineState, EngineError>;
fn legacy_plaintext_state_permitted() -> bool;  // module-private; only called from within envelope_io
pub(crate) fn decode_persisted_state_storage_format(bytes: &[u8]) -> Result<PersistedStateStorageFormat, EngineError>;
pub(crate) fn enforce_corrupted_state_backup_retention(path: &Path) -> Result<(), EngineError>;
pub(crate) fn sync_state_file_parent_directory(path: &Path) -> Result<(), EngineError>;
pub(crate) fn state_file_parent_directory(path: &Path) -> Option<&Path>;
pub(crate) fn corrupted_state_backup_path(path: &Path) -> PathBuf;
pub(crate) fn sorted_corrupted_state_backups(path: &Path) -> Result<Vec<PathBuf>, EngineError>;
```

The I/O module is testable with a temp directory and the existing `LockHelperProcessGuard` (or equivalent). The corruption-recovery paths are independently testable by writing a known-bad envelope to a temp file.

### `key_provider` interface

```rust
pub(crate) trait StateKeyProvider: Send + Sync {
    fn material(&self) -> Result<StateEncryptionKeyMaterial, EngineError>;
    fn key_id(&self) -> &str;
}

pub(crate) struct EnvKeyProvider; // reads TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX
impl StateKeyProvider for EnvKeyProvider { ... }

pub(crate) struct CommandKeyProvider { /* runs TBTC_SIGNER_STATE_KEY_COMMAND */ }
impl StateKeyProvider for CommandKeyProvider { ... }

/// Process-lifetime cache keyed by the wrapped provider's `key_id()`.
pub(crate) struct CachedStateKeyProvider { /* wraps Box<dyn StateKeyProvider> */ }
impl StateKeyProvider for CachedStateKeyProvider { ... }

pub(crate) fn resolve_state_key_provider() -> Result<Box<dyn StateKeyProvider>, EngineError>; // already cache-wrapped
pub(crate) fn state_encryption_key_material() -> Result<StateEncryptionKeyMaterial, EngineError>; // thin wrapper: resolve_state_key_provider()?.material(); called cross-module from envelope_io (both load and persist paths)
pub(crate) fn state_key_command_timeout_secs() -> u64;
```

No test-only fake type moves into this module. Tests continue to define their own `StateKeyProvider` impls locally (the existing `tests.rs` pattern: `TrackingMockKeyProvider` + `StdArcMockProvider`), matching the current convention.

Production wires `EnvKeyProvider` or `CommandKeyProvider` based on the `TBTC_SIGNER_STATE_KEY_PROVIDER` env, then wraps the choice in `CachedStateKeyProvider` — this is exactly what `resolve_state_key_provider()` already does today. The existing `state_encryption_key_material()` function stays a thin wrapper: `resolve_state_key_provider()?.material()`.

### `pending_ops` interface

```rust
pub(crate) enum PersistencePendingOperation { /* existing */ }
pub(crate) fn mark_persistence_pending(operation: PersistencePendingOperation);
pub(crate) fn clear_persistence_pending_operation(operation: &PersistencePendingOperation);
pub(crate) fn persistence_pending_session_ids() -> HashSet<String>;
pub(crate) fn pending_build_taproot_tx_operation(session_id: &str) -> Option<PersistencePendingOperation>;
pub(crate) fn pending_emergency_rekey_operation(...) -> Option<PersistencePendingOperation>;
pub(crate) fn pending_canary_operation() -> Option<PersistencePendingOperation>;
pub(crate) fn clear_snapshot_covered_operations(engine_state: &EngineState);  // lives in pending_ops; called cross-module from envelope_io::persist_engine_state_to_storage (not load — see Wired data flow)
pub(crate) fn interactive_round2_persistence_pending(session_id: &str, marker: &str) -> bool;
pub(crate) fn interactive_aggregate_persistence_pending(session_id: &str, marker: &str) -> bool;
pub(crate) fn interactive_state_persistence_pending() -> bool;

#[cfg(any(test, feature = "bench-restart-hook"))]
pub(crate) fn clear_persistence_pending_operations();
```

The pending-ops module is testable with a HashMap-backed fake state. The snapshot-covering logic is testable without I/O.

### Subprocess machinery (unix-only)

The current `configure_state_key_command_process_group` / `kill_state_key_command_process_group` / `terminate_state_key_command` are `#[cfg(unix)]` / `#[cfg(not(unix))]` pair. They move into `key_provider.rs` as `pub(crate)` machinery. The Windows stub remains a no-op.

### Wired data flow

```
load_engine_state_from_storage (envelope_io)
  └── decode_persisted_state_storage_format (envelope_io)
      └── decode_encrypted_state_envelope (envelope_io)
          └── state_encryption_key_material (key_provider)  ← decrypts the envelope
  └── decode_state (schema_codec)  ← pure

persist_engine_state_to_storage (envelope_io)
  └── state_encryption_key_material (key_provider)  ← encrypts the envelope
  └── persist_engine_state_to_storage_with_key (envelope_io)
      └── encode_state (schema_codec)  ← pure
      └── atomic state-file replacement (envelope_io)
      └── sync_state_file_parent_directory (envelope_io)
      └── clear_snapshot_covered_operations (pending_ops)  ← only on successful replacement, not on load
```

`persist_engine_state_to_storage_with_key` is a second, `pub(crate)` envelope_io entry point that takes an already-resolved key instead of resolving one itself — callers that need the key resolved under a specific lock ordering (e.g. Round2's durable-marker write, which must resolve the key before writing the marker) call it directly and skip the wrapper's own key resolution.

The four modules are wired through `mod.rs`. The orchestration logic (the "what to do in what order") lives in `mod.rs`; the per-concern mechanics live in each submodule.

### Out-of-scope changes

- The AEAD envelope algorithm (`xchacha20poly1305`) is unchanged.
- The schema-version check logic is unchanged. The constant `PERSISTED_STATE_SCHEMA_VERSION` moves to `schema_codec` but its value is preserved.
- The key-provider subprocess timeout (`TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS`) bounds are unchanged.
- The state-file lock contract (`STATE_FILE_LOCK`) is unchanged.
- The corrupt-state recovery policy (`quarantine_and_reset` vs fail-closed) is unchanged.
- The legacy plaintext state path is unchanged in behaviour; the `legacy_plaintext_state_permitted()` function moves to `envelope_io` but its semantics are preserved.

## Testing Decisions

### Test surface

The chaos suite (`scripts/run_phase5_chaos_suite.sh`) pins `engine::tests::<name>` paths. The `engine::tests::state_file_*` tests (around 15 of them) exercise the persistence layer end-to-end. They continue to pass.

The new modules get dedicated unit tests at the `engine::tests::persistence_*` paths. These are additive: existing tests untouched.

### What makes a good test here

- A good test of `schema_codec::decode_state` constructs a `PersistedEngineState` with hand-set fields and verifies the projection is correct — no I/O.
- A good test of `envelope_io::load_engine_state_from_storage` writes a known envelope to a temp file and verifies the load round-trips — no subprocess.
- A good test of `key_provider::EnvKeyProvider::resolve` sets the env var and asserts the decoded key — no subprocess.
- A good test of `key_provider::CommandKeyProvider::resolve` uses a script that prints a known key and asserts the decoded key — local subprocess, no real KMS/HSM.
- A good test of `pending_ops::clear_snapshot_covered_operations` populates a fake state and asserts the covered markers are removed — no I/O.

### Prior art

The existing `engine::tests::state_lock_rejects_multi_process_contention` test (tests.rs:5787) exercises the unix file lock via the `LockHelperProcessGuard` scaffolding (defined at tests.rs:3788). The new modules can reuse the same scaffolding.

### Definition of Done

- The 4 new modules are reachable from `engine::persistence::*` (re-exported via `mod.rs`).
- The existing `engine::tests` suite passes with no behavioral change.
- The chaos suite (`scripts/run_phase5_chaos_suite.sh`) passes.
- The on-disk file format is byte-for-byte stable: a state file written by the previous code loads cleanly with the new code, and vice versa.
- The `TBTC_SIGNER_ABI_*` constants are not bumped.
- The `EnterPhase` / `flush_pending_marker` helpers from Candidate 1 are unaffected.

## Out of Scope

- The marker-durability invariants in `pending_ops` (write-before-persist, fail-closed on prior pending marker). The behavior is preserved; only the duplication is removed.
- The key-provider subprocess timeout. The current bounds (1s minimum, 300s maximum, 30s default) are preserved.
- The schema-version migration logic. The current `for session in sessions.values_mut()` block in `TryFrom<PersistedSessionState>` is preserved; the consolidation only moves the code.
- The state-file lock contract. The `STATE_FILE_LOCK` mutex, the `STATE_FILE_LOCK_SUFFIX` constant, and the lock-acquisition semantics are unchanged.
- The unix-only subprocess machinery. The `#[cfg(unix)]` / `#[cfg(not(unix))]` branching is preserved.
- The `PersistedKeyPackage` Debug-redaction logic. The current `impl std::fmt::Debug for PersistedKeyPackage` is preserved; the consolidation does not change the security-relevant log behavior.

## Further Notes

### Open questions

- The `StateKeyProvider` trait is intentionally simple (`material` + `key_id`). A future tightening could add a `rotate()` method for key rotation, but that is out of scope for this spec.
- The `envelope_io` module's `decode_persisted_state_storage_format` detects the schema version and dispatches to the right decoder. If the schema bumps to v2, the new decoder must be added in `schema_codec` and the dispatcher updated in `envelope_io`. The spec does not propose a new schema version; it only organizes the existing code.

### Risks

- The trait `StateKeyProvider` already exists in current code (persistence.rs:1050); this spec relocates it, unchanged, into `key_provider.rs`. If a future key-provider variant (e.g. an HSM-backed provider with a different lifecycle) does not fit the trait shape, the trait must evolve. The risk is small — the trait is intentionally minimal — but the maintainer should be aware that the trait is a contract.
- The four modules are wired through `mod.rs`. The orchestration logic (the "what to do in what order") is centralized. If the orchestration grows, the `mod.rs` becomes a god file. The risk is mitigated by keeping the four modules narrowly scoped and relying on the trait for the key-provider boundary.
- The `CachedStateKeyProvider` decorator sits between production adapters and the process-lifetime static cache. If a future adapter's `key_id()` is not actually cheap (e.g. it touches the secret channel), the cache's identity check would leak the cost it exists to avoid. The risk is mitigated by the trait doc comment's explicit contract: `key_id()` MUST NOT touch the secret channel.

### Alternatives considered

- **Two modules: `schema_codec` + `persistence_io`** — keeps the I/O complexity grouped. The user picked the 4-module split (Candidate 2 seam = option 0).
- **Stay as one module, extract `schema_codec` only** — even more conservative. The user picked the 4-module split.

### Related specs

- **Candidate 1** (interactive-session collapse): independent, can run in parallel.
- **Candidate 4** (policy `reject_*` funnel): independent, can run in parallel; the rejection logic does not cross the persistence boundary.
- **Candidate 5** (SessionState grouping): NOT independent — both specs rewrite the bodies of `TryFrom<PersistedSessionState> for SessionState` and `TryFrom<&SessionState> for PersistedSessionState` (persistence.rs:1917-2233 and the inverse). This spec relocates those bodies into `schema_codec.rs` unchanged in shape; C5 reshapes what they project into (flat fields → 6 substructures). Sequence them: land whichever is simpler to rebase the other onto (either order works structurally), then hand-merge the shared `TryFrom` bodies — do not apply both to a worktree in parallel and expect a clean merge.
- **Candidate 6** (FFI macro): independent, can run in parallel; the FFI entry points remain unchanged.
