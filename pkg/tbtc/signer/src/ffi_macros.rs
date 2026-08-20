//! Crate-internal macros for FFI entry points.
//!
//! Each macro here uses bare `macro_rules!` + `pub(crate) use` re-export
//! rather than `#[macro_export]`. `#[macro_export]` always places a macro at
//! the crate root as part of the crate's public surface; these macros are
//! deliberately crate-private and only ever expand within this crate.
//!
//! Inside each macro body we use plain `crate::ffi::...` paths (not
//! `$crate::ffi::...`): `$crate` is the canonical hygiene token for
//! `#[macro_export]`-exported macros invoked from other crates; here all
//! invocations are inside `pkg/tbtc/signer`, so `crate::` is simpler and
//! equally correct.

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
                let request: $request = crate::ffi::parse_request(request_ptr, request_len)?;
                let response = $engine_fn(request)?;
                crate::ffi::serialize_response(&response)
            })
        }
    };
}
pub(crate) use ffi_request_response;

/// Same shape as `ffi_request_response!`, but for engine responses that
/// carry secret material (DKG secret packages / key packages): routes
/// through `serialize_secret_response` (pre-sized, growth-avoiding buffer)
/// instead of `serialize_response` (`serde_json::to_vec`), so the
/// serialization step itself does not leave un-zeroized intermediate copies
/// of the secret-bearing JSON on the heap. See `ffi::serialize_secret_response`.
macro_rules! ffi_request_secret_response {
    ($symbol:ident, $request:ty, $engine_fn:path) => {
        #[no_mangle]
        pub extern "C" fn $symbol(
            request_ptr: *const u8,
            request_len: usize,
        ) -> crate::ffi::TbtcSignerResult {
            crate::ffi::ffi_entry(|| {
                let request: $request = crate::ffi::parse_request(request_ptr, request_len)?;
                let response = $engine_fn(request)?;
                crate::ffi::serialize_secret_response(&response)
            })
        }
    };
}
pub(crate) use ffi_request_secret_response;

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
            crate::ffi::ffi_entry(|| crate::ffi::serialize_response(&$engine_fn()))
        }
    };
}
pub(crate) use ffi_no_request_infallible;
