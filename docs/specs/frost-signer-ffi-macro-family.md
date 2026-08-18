---
title: FROST signer — macro the FFI entry-point pattern
date: 2026-08-18
status: draft
tags: [frost-signer, architecture, deepening, ffi]
---

# FROST signer — macro the FFI entry-point pattern

## Problem Statement

`pkg/tbtc/signer/src/lib.rs` exposes 29 `#[no_mangle] pub extern "C"` FFI entry points in 1,566 lines. Of the 29 symbols in `include/frost_tbtc.h`: 23 share the same six-line body — `ffi_entry(|| { parse_request::<T>(..)?, engine::fn(request)?, serialize_response(&response) })`; a further 3 (`frost_tbtc_canary_rollout_status`, `frost_tbtc_roast_liveness_policy`, `frost_tbtc_hardening_metrics`) share a shorter, related no-request body; and the remaining 3 are outliers with distinct shapes. 23 + 3 = 26 macro-eligible entries; 3 are hand-written outliers.

The 26 mechanical entries are byte-equivalent at the macro level. The variation is purely in the type triple `(RequestType, engine_fn, ResultType)`. Today the variation is hand-written, which means:

- A new FFI entry (e.g. a future Phase-7.3 surface) is a 10-line copy-paste.
- A future refactor that changes the parse / call / serialize pipeline must touch 26 entries.
- The 3 outliers (`frost_tbtc_version`, `frost_tbtc_abi_version`, `frost_tbtc_free_buffer`) are visually similar to the 26 and could be silently introduced to the wrong shape.

The blast radius is bounded: the FFI surface is the C-ABI contract, and the macro expansion is byte-equivalent to the current hand-written body. The risk is on the type level — a future engine-fn return-type change that the macro does not track.

## Solution

Introduce a 3-macro family in a new `ffi_macros` module:

```rust
ffi_request_response!(symbol, RequestType, engine_fn);              // 23 entries
ffi_no_request!(symbol, engine_fn);                                  // 1 entry (canary_rollout_status)
ffi_no_request_infallible!(symbol, engine_fn);                       // 2 entries (roast_liveness_policy, hardening_metrics)
```

The 3 outliers (`frost_tbtc_version`, `frost_tbtc_abi_version`, `frost_tbtc_free_buffer`) hand-write their bodies — each with a one-liner comment explaining why the macro does not apply.

The safety narrative for each entry moves to `include/frost_tbtc.h` (the C header that the Go host reads). The Rust module entries are one-line macro invocations. The C header is the canonical contract documentation.

The wire contract (`TBTC_SIGNER_ABI_*`) is unchanged. The panic-redaction wrapper, the `?`-propagated `EngineError` envelope, the request length/null-ptr checks, and the `TbtcBuffer` ownership semantics are preserved.

## User Stories

1. As a maintainer, I want to add a new FFI entry so that the change is a 1-line macro invocation.
2. As a maintainer, I want the parse / call / serialize pipeline in one place so that a future refactor is a single diff.
3. As a Go host integrator, I want the FFI surface unchanged so that the bridge does not need to bump its ABI version.
4. As a maintainer, I want the 3 outliers named explicitly so that a future well-meaning refactor cannot silently route them through the wrong macro.
5. As a security reviewer, I want the panic-redaction hook installation behaviour preserved so that the redacting panic hook still installs on the first call to any `ffi_entry`-using entry.
6. As a reader of the C header, I want the safety narrative in `include/frost_tbtc.h` so that the documentation lives at the boundary the bridge reads.
7. As a maintainer, I want the macro expansion to be byte-equivalent to the current hand-written body so that the diff against the previous code is reviewable as a pure source-form compression.

## Implementation Decisions

### Module layout

A new `pkg/tbtc/signer/src/ffi_macros.rs` module holds the 3 macros. `macro_rules!` has no visibility-modifier syntax on stable Rust (`pub(crate) macro_rules! ...` is a syntax error — E0364; `pub` on `macro_rules!` is gated behind the unstable, unstabilized `pub_macro_rules` feature). The crate-internal pattern is a bare `macro_rules!` definition immediately followed by a `pub(crate) use` re-export of the same name, NOT `#[macro_export]` (that attribute always places a macro at the crate root as part of the crate's exported surface, usable by external dependents — the opposite of the stated intent that these macros are not exported to crate consumers).

```rust
// pkg/tbtc/signer/src/ffi_macros.rs
macro_rules! ffi_request_response { ... }
pub(crate) use ffi_request_response;

macro_rules! ffi_no_request { ... }
pub(crate) use ffi_no_request;

macro_rules! ffi_no_request_infallible { ... }
pub(crate) use ffi_no_request_infallible;
```

The `lib.rs` module declaration gains `mod ffi_macros;` and each entry-point body becomes a 1-line invocation, e.g. `use crate::ffi_macros::ffi_request_response;` followed by `ffi_request_response!(frost_tbtc_init_signer_config, InitSignerConfigRequest, engine::init_signer_config);`.


### Macro definitions (paste-ready)

```rust
/// Canonical FFI entry: parse a JSON request, call an `engine::fn` that
/// returns `Result<R, EngineError>`, serialize `R` as JSON, run the whole
/// closure through the panic-redacting `ffi_entry` wrapper.
///
/// Expansion contract (must match the per-entry bodies in lib.rs today):
///   #[no_mangle] pub extern "C" fn $symbol(request_ptr, request_len)
///   -> TbtcSignerResult { ffi_entry(|| { let r: $request = parse_request(..)?;
///   let x = $engine_fn(r)?; serialize_response(&x) }) }
macro_rules! ffi_request_response {
    ($symbol:ident, $request:ty, $engine_fn:path) => {
        #[no_mangle]
        pub extern "C" fn $symbol(
            request_ptr: *const u8,
            request_len: usize,
        ) -> crate::ffi::TbtcSignerResult {
            crate::ffi::ffi_entry(|| {
                let request: $request =
                    crate::ffi::parse_request(request_ptr, request_len)?;
                let response = $engine_fn(request)?;
                crate::ffi::serialize_response(&response)
            })
        }
    };
}
pub(crate) use ffi_request_response;

/// No-arg FFI entry whose engine fn returns `Result<R, EngineError>`.
macro_rules! ffi_no_request {
    ($symbol:ident, $engine_fn:path) => {
        #[no_mangle]
        pub extern "C" fn $symbol() -> crate::ffi::TbtcSignerResult {
            crate::ffi::ffi_entry(|| {
                let response = $engine_fn()?;
                crate::ffi::serialize_response(&response)
            })
        }
    };
}
pub(crate) use ffi_no_request;

/// No-arg FFI entry whose engine fn returns `R` directly (no `Result`).
/// There is no `?` after the engine call; the body shape matches
/// `roast_liveness_policy` and `hardening_metrics` exactly.
macro_rules! ffi_no_request_infallible {
    ($symbol:ident, $engine_fn:path) => {
        #[no_mangle]
        pub extern "C" fn $symbol() -> crate::ffi::TbtcSignerResult {
            crate::ffi::ffi_entry(|| {
                crate::ffi::serialize_response(&$engine_fn())
            })
        }
    };
}
pub(crate) use ffi_no_request_infallible;
```

`crate::ffi::...` paths are used inside the macro body rather than `$crate::ffi::...`: `$crate` resolves to the defining crate at the macro's expansion site, which is correct for a `#[macro_export]`-exported macro usable from other crates, but is unnecessary indirection for a macro that only ever expands within this crate. A plain `crate::` path is simpler and equally correct here since every invocation site is inside `pkg/tbtc/signer`.

### Entry-point mapping

The 29 header symbols map to:

| Entry | Macro | Notes |
|---|---|---|
| `frost_tbtc_init_signer_config` | `ffi_request_response!` | |
| `frost_tbtc_roast_transcript_audit` | `ffi_request_response!` | |
| `frost_tbtc_verify_blame_proof` | `ffi_request_response!` | |
| `frost_tbtc_quarantine_status` | `ffi_request_response!` | |
| `frost_tbtc_refresh_cadence_status` | `ffi_request_response!` | |
| `frost_tbtc_trigger_emergency_rekey` | `ffi_request_response!` | |
| `frost_tbtc_run_differential_fuzzing` | `ffi_request_response!` | |
| `frost_tbtc_promote_canary` | `ffi_request_response!` | |
| `frost_tbtc_rollback_canary` | `ffi_request_response!` | |
| `frost_tbtc_dkg_part1` | `ffi_request_response!` | |
| `frost_tbtc_dkg_part2` | `ffi_request_response!` | |
| `frost_tbtc_dkg_part3` | `ffi_request_response!` | |
| `frost_tbtc_persist_distributed_dkg_key_package` | `ffi_request_response!` | |
| `frost_tbtc_new_signing_package` | `ffi_request_response!` | |
| `frost_tbtc_verify_signature_share` | `ffi_request_response!` | |
| `frost_tbtc_interactive_session_open` | `ffi_request_response!` | |
| `frost_tbtc_interactive_round1` | `ffi_request_response!` | |
| `frost_tbtc_interactive_round2` | `ffi_request_response!` | |
| `frost_tbtc_interactive_session_abort` | `ffi_request_response!` | |
| `frost_tbtc_interactive_aggregate` | `ffi_request_response!` | |
| `frost_tbtc_derive_interactive_attempt_context` | `ffi_request_response!` | |
| `frost_tbtc_build_taproot_tx` | `ffi_request_response!` | |
| `frost_tbtc_refresh_shares` | `ffi_request_response!` | |
| `frost_tbtc_canary_rollout_status` | `ffi_no_request!` | returns `Result<CanaryRolloutStatusResult, EngineError>` — fallible |
| `frost_tbtc_roast_liveness_policy` | `ffi_no_request_infallible!` | returns `RoastLivenessPolicyResult` directly |
| `frost_tbtc_hardening_metrics` | `ffi_no_request_infallible!` | returns `SignerHardeningMetricsResult` directly |
| `frost_tbtc_version` | **outlier** | no `ffi_entry`, no panic redaction |
| `frost_tbtc_abi_version` | **outlier** | inline `FrostTbtcAbiVersionResult` construction |
| `frost_tbtc_free_buffer` | **outlier** | distinct ABI, returns `void` |

The 26 mechanical entries become 1-line invocations. The 3 outliers hand-write their bodies with a one-liner comment:

```rust
// Outlier: success-only, no panic-redaction wrapper. The version probe must
// succeed unconditionally; routing it through ffi_entry would add error-path
// machinery that the original never had.
#[no_mangle]
pub extern "C" fn frost_tbtc_version() -> TbtcSignerResult {
    success_from_string(TBTC_SIGNER_VERSION.to_string())
}
```

### C header documentation

The C header (`include/frost_tbtc.h`) gains expanded comments for each entry that previously had a Rust-side doc comment. The safety narrative moves:

- "Caller MUST release `TbtcBuffer.buffer` exactly once via `frost_tbtc_free_buffer`" — stays in `frost_tbtc_free_buffer`'s Rust doc AND in the C header (the C side is the boundary the bridge reads).
- "Phase 7.1 hardened interactive signing session (frozen spec docs/phase-7-interactive-session-spec-freeze.md)" — moves to the C header for the 6 interactive entries.
- "Reserved response shape for a future cryptographic share-refresh protocol" — stays in `frost_tbtc_refresh_shares` Rust doc; the C header has a short comment.

The `TBTC_SIGNER_ABI_*` constants and the `LIBC FROST_TBTC` library export contract are unchanged.

### Out-of-scope changes

- The panic-redaction hook installation (`install_redacting_panic_hook`) is unchanged. The hook installs on the first call to any `ffi_entry`-using entry. The 3 outliers are carefully excluded so they cannot accidentally trigger the hook.
- The `Request_bytes` validation (length cap, null-ptr check) is unchanged.
- The `to_ffi_buffer` source-vec zeroize is unchanged.
- The `TbtcSignerResult` repr is unchanged.
- The `free_buffer` ABI (`(ptr: *mut u8, len: usize) -> void`) is unchanged.

## Testing Decisions

### Test surface

The `engine::tests` (and `ffi::tests`) suite is unchanged. The macro expansion is byte-equivalent to the current hand-written body, so the existing test suite exercises the same code paths.

The 3 outliers' one-liner comments are an explicit reminder that the macro does not apply. The `ffi::tests` module (currently 88 lines) gains a test that verifies the 3 outlier bodies are still hand-written (a regression test against accidental refactoring).

### What makes a good test here

- A good test of the macro expansion is a `compile-pass` test that the macro produces the expected symbol. The existing test suite exercises this implicitly.
- A good test of the 3 outliers' regression is a `grep`-based test that asserts the outlier bodies still have their one-liner comment.
- A good test of the C header documentation is a doc-comments test that the `frost_tbtc.h` header includes the safety narrative for each entry.

### Prior art

The existing `ffi::tests` module exercises `parse_request` end-to-end and the `TbtcBuffer` ownership contract. The new test fits the same pattern.

### Definition of Done

- The 3 macros are defined in `pkg/tbtc/signer/src/ffi_macros.rs`.
- The 26 mechanical entries become 1-line macro invocations.
- The 3 outliers (`frost_tbtc_version`, `frost_tbtc_abi_version`, `frost_tbtc_free_buffer`) hand-write their bodies with one-liner comments.
- The C header (`include/frost_tbtc.h`) gains the safety narrative for each entry.
- `cargo test --lib --package frost-signer` passes.
- The chaos suite (`scripts/run_phase5_chaos_suite.sh`) passes.
- The `TBTC_SIGNER_ABI_*` constants are not bumped.
- Each macro's expansion is byte-equivalent to the current hand-written body for its shape: the 23 `ffi_request_response!` entries keep the six-line parse/call/serialize body; the 1 `ffi_no_request!` and 2 `ffi_no_request_infallible!` entries keep their respective shorter bodies.

## Out of Scope

- The panic-redaction hook installation behaviour. The hook installs on the first call to any `ffi_entry`-using entry; the behaviour is preserved.
- The `Request_bytes` validation. The length cap, null-ptr check, and JSON parse are unchanged.
- The `to_ffi_buffer` source-vec zeroize. The secret-zeroize path is preserved.
- The `TbtcSignerResult` repr. The `#[repr(C)]` struct is unchanged.
- The `free_buffer` ABI. The `void` return type and the ownership contract are unchanged.
- A new `ffi_no_request_infallible!` macro for the success-only `frost_tbtc_version` case. The 3 outliers are intentionally hand-written; the macro is not extended.

## Further Notes

### Open questions

- (Resolved) The macros use plain `crate::ffi::*` paths, not `$crate::ffi::*`. `$crate` matters for a `#[macro_export]`-exported macro invoked from other crates; these macros are `pub(crate) use`-scoped and only ever expand within this crate, so `crate::` is simpler and equally correct.
- The 3 outliers' one-liner comments are a maintenance burden. A future tightening could replace them with a `compile_error!` directive that fires if the body is too long, but that is over-engineering for a 3-symbol case.
- The C header's expanded comments are a maintenance burden. The Rust doc comments and the C header comments must be kept in sync. A future tightening could generate the C header from the Rust doc comments, but that is a separate workstream.

### Risks

- The macro is a single point of failure. A future bug in the macro expansion affects all 26 entries. The risk is mitigated by the macro being a pure source-form compression (the expansion is byte-equivalent to the current hand-written body) and the existing test suite.
- The `?` after the engine call is critical for the `ffi_request_response!` and `ffi_no_request!` macros. The `ffi_no_request_infallible!` macro intentionally omits the `?`. A future refactor that turns an infallible engine fn into a fallible one must update the macro invocation.
- The `init_signer_config` global side effect is invisible to macro users. The macro expansion is identical to the hand-written body; the side effect is preserved. A future maintainer reading only the macro expansion must know that `init_signer_config` is the install entry.

### Alternatives considered

- **Outliers as a 4th macro**: adds a `ffi_success_from_string!` for `frost_tbtc_version`. The user picked the hand-written approach (Candidate 6 outliers = option 0).
- **Universal macro with all 4 shapes**: highest consolidation; greatest risk of mis-application. The user picked the 3-macro form.
- **Doc comments on each invocation**: keeps the doc narrative per entry. The user picked the C-header form (Candidate 6 doc-comments = option 2).

### Related specs

- **Candidate 1** (interactive-session collapse): independent, can run in parallel; the 6 interactive FFI entries continue to wrap the same `engine::interactive_*` functions.
- **Candidate 2** (persistence split): independent, can run in parallel; the persistence-engine FFI entry continues to call the same `engine::load_engine_state_from_storage` function.
- **Candidate 4** (policy `reject_*` funnel): independent, can run in parallel.
- **Candidate 5** (SessionState grouping): independent, can run in parallel; the FFI entry points are unaffected.
