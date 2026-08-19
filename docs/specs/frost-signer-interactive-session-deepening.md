---
title: FROST signer — collapse interactive-session entry points
date: 2026-08-18
status: draft
tags: [frost-signer, architecture, deepening, phase-7-1]
---

# FROST signer — collapse interactive-session entry points

## Problem Statement

The `pkg/tbtc/signer/src/engine/interactive.rs` module exposes five `pub fn` phase handlers — `interactive_session_open`, `interactive_round1`, `interactive_round2`, `interactive_aggregate`, `interactive_session_abort` — each shaped as a monolith of 80 to 510 lines that interleaves:

- **Lock-prologue**: prior to this consolidation, `state()?.lock().map_err(...)?` + `sweep_expired_interactive_state_durably(&mut guard)?` (2 statements, byte-identical, repeated at all 5 phase-entry sites: interactive.rs:321, 789, 881, 1273, 1647; the string `"engine lock poisoned"` also appears once more at interactive.rs:1456, but that is a nested lock acquisition inside `interactive_aggregate`'s body, not a 6th phase-entry prologue). After consolidation, those 2-statement prologues collapse to a single `let [mut] guard = enter_phase()?;` call at interactive.rs:359, 824, 913, 1301, 1671; the nested `"engine lock poisoned"` string moves to interactive.rs:1482.
- **Auto-quarantine config load**: `load_auto_quarantine_config()?` is phase-specific, not part of the shared prologue — only `open` (interactive.rs:361, immediately after the consolidated `enter_phase()?` call) and `round2` (interactive.rs:924, several lines AFTER the marker-durability flush below) call it; `round1`, `aggregate`, and `abort` never call it. The two call sites are not even at the same relative position, so this cannot be folded into a single `enter_phase()` step without reordering round2's marker-durability flush behind a fallible config parse — see "Out-of-scope changes."
- **Marker-durability dance**: re-persist if an earlier marker write failed its directory sync, then write the new marker (3 lines, repeated 3 times across open / round2 / aggregate).
- **Per-phase business logic**: the actual session lookup, validation, signing-package deserialization, share release, or aggregation.

The repetition is the load-bearing problem. The 5 phase-entry lock-prologues are byte-identical. The marker-durability dance has the same shape at all 3 sites with phase-specific predicates. Read alongside the `engine::tests` suite (200+ tests pinning `engine::tests::<name>` paths via `scripts/run_phase5_chaos_suite.sh`), the shallowness is visible: any future Phase-7.x milestone that touches the signing path must re-read and re-validate the same 2-statement lock-prologue five times and the 3-statement marker-durability dance three times.

The blast radius is widened by the fact that the marker-durability invariant (write-before-persist, fail-closed on prior pending marker) is the security-load-bearing property of the interactive signing path. It is currently enforced by hand at each call site.

## Solution

Consolidate the shared prologue and the marker-durability dance into two private helpers inside `engine::interactive`. The five `pub fn` phase handlers stay where they are; they call the new helpers as the first and (where applicable) last step of their work. The chaos suite test paths and the FFI surface are unchanged.

The new module shape:

```
pub fn interactive_session_open(...)        // unchanged signature
pub fn interactive_round1(...)               // unchanged signature
pub fn interactive_round2(...)               // unchanged signature
pub fn interactive_aggregate(...)            // unchanged signature
pub fn interactive_session_abort(...)        // unchanged signature

pub(crate) fn enter_phase() -> Result<MutexGuard<EngineState>, EngineError>
pub(crate) fn flush_pending_marker(guard, predicate) -> Result<(), EngineError>
```

`enter_phase` consolidates only the lock-acquisition + sweep — the 2-statement shape that is byte-identical across all 5 phase-entry sites. `flush_pending_marker` consolidates the re-persist-on-pending-marker step. Both are `pub(crate)` so the test suite can exercise them in isolation; both are private to the FFI surface. `load_auto_quarantine_config()?` is deliberately NOT folded into `enter_phase` — see "Out-of-scope changes" for why. The five `pub fn` phase handlers continue to own all phase-specific logic, including their own `load_auto_quarantine_config()?` calls where they exist today.

## User Stories

1. As a maintainer, I want to extend the lock-prologue with a new step (e.g. a fresh sweep) so that the change is one diff line, not five.
2. As a maintainer, I want to add a new marker-durability predicate (e.g. for a future Phase-7.3 entry) so that the persistence-on-pending step is a single helper call, not a copy-paste.
3. As a maintainer, I want the marker-durability invariant (write-before-persist, fail-closed on prior pending marker) to be enforced at one seam so that a future bug in one phase cannot silently diverge from the other two.
4. As a test author, I want to call `flush_pending_marker` directly in a test so that the round2-failed-marker scenario can be exercised without spinning an entire Open → Round2 cycle.
5. As a test author, I want to call `enter_phase` directly so that the lock-prologue's poison-recovery and sweep behavior can be tested in isolation, independent of any phase's config-loading.
6. As a Go host integrator, I want the FFI surface unchanged (same 5 `pub fn` signatures) so that the bridge does not need to bump its ABI version.
7. As a chaos-suite runner, I want `engine::tests::<name>` paths unchanged so that the pinned test paths in `scripts/run_phase5_chaos_suite.sh` continue to resolve.
8. As a security reviewer, I want the locked-nonce zeroization path (the `HardeningOperationLatencyGuard::success_only` drop, present at round1/round2/aggregate) to be unaffected by the consolidation so that the consumption-before-release invariant is preserved.

## Implementation Decisions

### Module location

The new helpers live inside `pkg/tbtc/signer/src/engine/interactive.rs`. No new module is created. The `engine::mod.rs` re-export pass-through is unchanged. The `scripts/run_phase5_chaos_suite.sh` test-path pin contract is preserved.

### `enter_phase` contract

```rust
pub(crate) fn enter_phase() -> Result<std::sync::MutexGuard<'static, EngineState>, EngineError> {
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    sweep_expired_interactive_state_durably(&mut guard)?;
    Ok(guard)
}
```

`enter_phase` deliberately does NOT also call `load_auto_quarantine_config()?`. The two current call sites are not at the same relative position: `open` (interactive.rs:361) calls it immediately after the consolidated `enter_phase()?` call, but `round2` (interactive.rs:924) calls it AFTER the marker-durability flush (the re-persist that repairs a prior fail-closed marker write). Folding config-loading into `enter_phase` would run it before the marker-durability flush at round2 — reordering a security-relevant durability repair behind a fallible config parse, so a misconfigured `TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV`/allowlist env var could skip a fail-closed marker repair it doesn't skip today. `round1`, `aggregate`, and `abort` never call `load_auto_quarantine_config` to…

Each phase keeps its own `load_auto_quarantine_config()?` call at its current position: `open` right after `enter_phase()`, `round2` right after its `flush_pending_marker(...)` call.

The function returns the guard. Callers retain full access to the guarded engine state for their phase-specific reads. The lock is released on `Drop` of the guard.

### `flush_pending_marker` contract

```rust
pub(crate) fn flush_pending_marker<F>(
    guard: &MutexGuard<'static, EngineState>,
    predicate: F,
) -> Result<(), EngineError>
where
    F: FnOnce() -> bool,
{
    if predicate() {
        persist_engine_state_to_storage(guard).map_err(PersistEngineStateError::into_engine_error)?;
    }
    Ok(())
}
```

The predicate is a callback that the caller supplies to detect phase-specific pending markers (round2-pending, aggregate-pending, open-pending). The helper is intentionally a step in the larger phase, not a full wrapper.

### Per-phase shape after the consolidation

Each of the 5 `pub fn` phase handlers is restructured to:

```
#[no_mangle / pub fn]
fn phase_handler(request) -> Result<Response, EngineError> {
    record_hardening_telemetry(...);
    let mut guard = enter_phase()?;
    // ... phase-specific validation, session lookup, business logic ...
    // open and round2 additionally call `let auto_quarantine_config = load_auto_quarantine_config()?;`
    // at their existing position (see "enter_phase contract" above for why it stays out of enter_phase).
    Ok(response)
}
```

The marker-durability variant of the pattern (used in round2 and aggregate) adds a `flush_pending_marker(&guard, || { ...pending predicate... })?;` step before the new marker write.

### Validation prologue

The input-validation prologue (validate_session_id, validate_attempt_id, hex decode, attempt_id canonicalization, message canonicalization) is intentionally NOT extracted in this spec. The validation is heterogeneous across the 5 phases — open validates the request structure in detail; round2 only checks session_id and attempt_id; aggregate trusts the package's authoritative shape. Forcing a shared prologue would require a payload-enum or a too-generic function. The duplication that matters (lock + sweep + marker-durability) is extracted; the validation stays phase-specific.

### Latency tracking

The `HardeningOperationLatencyGuard::success_only(...)` is used in round1, round2, and aggregate (interactive.rs:815, 892, 1260). It stays at the top of each phase, not in `enter_phase`. Adding it to `enter_phase` would mean the open and abort phases also pay the latency-tracking cost for what is currently a no-op there.

### Helper visibility rationale

The helpers are `pub(crate)` (not `pub`, not `fn`) because:

- `pub(crate)` lets `engine::tests` reach them directly for unit tests at the helper boundary.
- They are not `pub` because the FFI surface is the five `pub fn`, and the helpers are internal mechanics.
- They are not `fn` (private to the module) because the spec explicitly wants the test suite to exercise them in isolation.

### Out-of-scope changes

- `enter_phase` does NOT consolidate the `load_auto_quarantine_config` call. Each of `open` and `round2` keeps its own explicit `load_auto_quarantine_config()?` call at its current position (see "`enter_phase` contract" above). `round1`, `aggregate`, and `abort` never call it and gain no new `?` error exit.
- The five `pub fn` bodies are NOT rewritten line-by-line. The mechanical change is: replace the lock+sweep prologue with one `enter_phase()` call; replace the 3-line marker-durability block with one `flush_pending_marker(...)` call. Bodies that have one-off prologue variations (e.g. open's wallet-session resolution) keep their variation.
- The persistence-layer seam (Candidate 2) is NOT touched in this spec.
- The `SessionState` grouping (Candidate 5) is NOT touched in this spec.

## Testing Decisions

### Test surface

The chaos suite (`scripts/run_phase5_chaos_suite.sh`) pins `engine::tests::<name>` paths. The 5 `pub fn` phase handlers keep their names. No test paths are renamed.

The new helpers get unit tests at the `engine::tests::enter_phase_*` and `engine::tests::flush_pending_marker_*` paths. These are additive — the existing test suite untouched.

### What makes a good test here

- A good test of `enter_phase` verifies that the returned guard is non-poisoned and the sweep ran — nothing else. `enter_phase` never touches the auto-quarantine config, so no config-parsing error can surface from it.
- A good test of `flush_pending_marker` verifies that the predicate-true branch re-persists, the predicate-false branch is a no-op, and the helper fails closed if the persist itself fails.
- A good test of the phase handlers verifies the phase-specific behavior (open binds a session, round2 releases a share, aggregate produces a signature) — the prelude is no longer the test target.

### Prior art

The existing `engine::tests::interactive_round2_persist_fault_leaves_nonces_live` test (tests.rs:9713) already exercises the marker-durability dance by injecting a `PersistFaultInjectionPoint`. The new `flush_pending_marker` helper can be tested by reusing the same fault-injection seam.

### Definition of Done

- The 5 `pub fn` phase handlers reflect the new structure: 1 `enter_phase()` call instead of the 2-statement lock-prologue; 1 `flush_pending_marker()` call instead of 3 marker-durability lines; `open` and `round2` keep their own explicit `load_auto_quarantine_config()?` calls at their current positions.
- `cargo test --lib --package frost-signer` passes (no behavioral change).
- The chaos suite (`scripts/run_phase5_chaos_suite.sh`) passes.
- New `engine::tests::enter_phase_*` and `engine::tests::flush_pending_marker_*` tests cover the helper boundaries.
- No public API change. The ABI version (`TBTC_SIGNER_ABI_*`) is not bumped.

## Out of Scope

- Splitting `interactive.rs` into multiple files. The module-level organization is fine; the consolidation is at the function level.
- Renaming the 5 `pub fn` phase handlers.
- Changing the marker-durability invariants themselves (fail-closed, write-before-persist). The behavior is preserved; only the duplication is removed.
- Touching the FFI entry points in `lib.rs`. The FFI surface is unchanged.
- Touching the persistence layer (Candidate 2) or the `SessionState` grouping (Candidate 5). They are independent specs.

## Further Notes

### Open questions

- (Resolved) Whether the auto-quarantine config should load inside `enter_phase`, conditionally or otherwise: no, never. `open`'s and `round2`'s `load_auto_quarantine_config()?` calls stay in their own bodies at their existing positions, because folding round2's call into `enter_phase` would run it before round2's marker-durability flush instead of after — see "`enter_phase` contract" above.
- The `flush_pending_marker` predicate is invoked while the engine-state lock is held, and its persist call takes `&guard` directly rather than re-acquiring the lock (`std::sync::Mutex` is not reentrant, so a real second acquisition would deadlock). This is correct as written; a code review should confirm no call reachable from the predicate or from `persist_engine_state_to_storage` ever tries to lock `state()` again while the guard is live.

### Risks

- The `predicate: FnOnce() -> bool` callback in `flush_pending_marker` is called inside the lock. If the predicate ever reads from a different mutex, the lock-ordering bug is silent. The phase-specific predicates are local bool expressions today; a future refactor that adds a database read or a subprocess call inside the predicate would deadlock. The spec should keep the predicates pure.
- The `EnterPhase` helper takes the lock for the duration of the phase. A long-running phase (e.g. an aggregate that holds state across multiple sub-iterations) blocks all other engine calls. The existing 5 phase-entry sites already do this; the consolidation does not change the blocking model. A future decoupling could split the lock into per-session locks; that is out of scope for this spec.
- The `HardeningOperationLatencyGuard` is at the top of round1, round2, and aggregate, not in `enter_phase`. If a future phase adds latency tracking, the placement must be decided per phase. Documenting the convention in the spec protects against drift.

### Alternatives considered

- **Macro: `with_engine_state!(guard => { ... })`** — comparable to the FFI macro proposal (Candidate 6). Adds a language-level indirection that is harder to debug than a function call. The `enter_phase` helper is preferred for stack-trace clarity.
- **Struct-of-methods on `InteractiveSession`** — each phase becomes a method on a struct that holds the per-call state. Strongest testability but a larger API change. The user picked the in-place helper approach (Candidate 1 seam = option 0).
- **Leave the dance open-coded, extract only the lock-prologue** — smallest change but fails the user's stated goal of consolidating the marker-durability invariant. The user picked the `flush_pending_marker` helper.

### Related specs

- **Candidate 2** (persistence split): independent, can run in parallel.
- **Candidate 5** (SessionState grouping): independent, can run in parallel; the field-access patterns simplify after the grouping.
- **Candidate 6** (FFI macro): independent, can run in parallel; the FFI entry points remain intact.
