use std::panic::{catch_unwind, AssertUnwindSafe};

use serde::de::DeserializeOwned;

use zeroize::Zeroize;

use crate::api::ErrorResponse;
use crate::errors::EngineError;

/// SAFETY: the caller MUST guarantee `ptr` points to a heap allocation of
/// EXACTLY `len` bytes (`Box<[u8]>::into_raw`), and MUST release it exactly
/// once via `frost_tbtc_free_buffer`. Misuse (double-free, sized mismatch,
/// non-heap pointer) is undefined behavior.
#[repr(C)]
pub struct TbtcBuffer {
    pub ptr: *mut u8,
    pub len: usize,
}

#[repr(C)]
pub struct TbtcSignerResult {
    pub status_code: i32,
    pub buffer: TbtcBuffer,
}

const STATUS_OK: i32 = 0;
const STATUS_ERROR: i32 = 1;
const MAX_REQUEST_BYTES: usize = 16 * 1024 * 1024;

pub fn success_from_serialized(payload: Vec<u8>) -> TbtcSignerResult {
    TbtcSignerResult {
        status_code: STATUS_OK,
        buffer: to_ffi_buffer(payload),
    }
}

pub fn success_from_string(message: String) -> TbtcSignerResult {
    success_from_serialized(message.into_bytes())
}

pub fn parse_request<T: DeserializeOwned>(ptr: *const u8, len: usize) -> Result<T, EngineError> {
    let bytes = request_bytes(ptr, len)?;
    serde_json::from_slice(bytes)
        .map_err(|e| EngineError::Validation(format!("invalid JSON request payload: {e}")))
}

pub fn serialize_response<T: serde::Serialize>(response: &T) -> Result<Vec<u8>, EngineError> {
    serde_json::to_vec(response)
        .map_err(|e| EngineError::Internal(format!("failed to encode response: {e}")))
}

// Install a panic hook that redacts the panic payload outside the development
// profile. `catch_unwind` does not suppress Rust's default hook, so a panic
// carrying a path / config value / secret would otherwise print verbatim to
// stderr before it is converted to a redacted ErrorResponse. Installed once.
fn install_redacting_panic_hook() {
    static INSTALLED: std::sync::Once = std::sync::Once::new();
    INSTALLED.call_once(|| {
        // Wrap the hook in a single-use mechanism so the install-set_hook
        // branch can drop it into the closure, and the failure branch can
        // retrieve it back to restore the global state.
        let default_hook_cell: std::sync::Mutex<Option<_>> =
            std::sync::Mutex::new(Some(std::panic::take_hook()));
        // set_hook can panic during Box::new under allocator pressure. If it
        // does, restore the previously-installed hook so the process still has
        // SOME panic handler (the default hook we captured is still in memory).
        // Otherwise a partial install leaves the Once poisoned (no further
        // installs) AND no panic hook at all (the default was take_hook'd).
        let install = catch_unwind(AssertUnwindSafe(|| {
            let mut guard = default_hook_cell.lock().expect("poisoned");
            let default_hook = guard.take().expect("default_hook available");
            std::panic::set_hook(Box::new(move |info| {
                let development_profile =
                    crate::engine::signer_env_var(crate::engine::TBTC_SIGNER_PROFILE_ENV)
                        .map(|raw| {
                            raw.trim().eq_ignore_ascii_case(
                                crate::engine::TBTC_SIGNER_PROFILE_DEVELOPMENT,
                            )
                        })
                        .unwrap_or(false);
                if development_profile {
                    default_hook(info);
                } else if let Some(location) = info.location() {
                    eprintln!(
                        "panic at {}:{} (payload redacted)",
                        location.file(),
                        location.line()
                    );
                } else {
                    eprintln!("panic (payload redacted)");
                }
            }));
        }));
        if install.is_err() {
            // Restore the hook we previously captured so the global state is
            // consistent. Once will not re-fire on the next ffi_entry.
            if let Some(hook) = default_hook_cell.lock().expect("poisoned").take() {
                let _ = catch_unwind(AssertUnwindSafe(|| {
                    std::panic::set_hook(hook);
                }));
            }
        }
    });
}

pub fn ffi_entry<F>(f: F) -> TbtcSignerResult
where
    F: FnOnce() -> Result<Vec<u8>, EngineError>,
{
    install_redacting_panic_hook();
    match catch_unwind(AssertUnwindSafe(f)) {
        Ok(Ok(bytes)) => success_from_serialized(bytes),
        Ok(Err(err)) => error_result(err),
        Err(payload) => error_result(EngineError::Internal(panic_boundary_message(payload))),
    }
}

// A panic crossing the FFI boundary must not reflect its raw payload to the
// host in production: the panic message could carry a filesystem path, a
// config value, or other internal detail. Only the development profile keeps
// the payload, for operator diagnostics.
//
// This reads the profile directly and fails CLOSED (redacts) for any
// non-development value - including a missing or malformed profile - rather
// than calling signer_profile_is_production(), which panics on a malformed
// profile. Re-validating the profile here could otherwise turn a handled
// panic into a second panic on the FFI error path and unwind into C.
fn panic_boundary_message(payload: Box<dyn std::any::Any + Send>) -> String {
    let development_profile = crate::engine::signer_env_var(crate::engine::TBTC_SIGNER_PROFILE_ENV)
        .map(|raw| {
            raw.trim()
                .eq_ignore_ascii_case(crate::engine::TBTC_SIGNER_PROFILE_DEVELOPMENT)
        })
        .unwrap_or(false);

    if development_profile {
        format!(
            "panic crossed FFI boundary: {}",
            panic_payload_message(payload)
        )
    } else {
        "panic crossed FFI boundary".to_string()
    }
}

pub fn free_buffer(ptr: *mut u8, len: usize) {
    if ptr.is_null() || len == 0 {
        return;
    }

    unsafe {
        let mut buffer = Box::from_raw(std::ptr::slice_from_raw_parts_mut(ptr, len));
        // Wipe any plaintext secret material (e.g. FROST nonces, DKG/key-package
        // bytes) before deallocation rather than trusting every FFI caller to do
        // it correctly. Leaking a nonce after a share is produced can expose the
        // signing share. `zeroize` is a volatile wipe the optimizer cannot elide.
        buffer.zeroize();
        drop(buffer);
    }
}

fn error_result(error: EngineError) -> TbtcSignerResult {
    let payload = ErrorResponse {
        code: error.code().to_string(),
        message: error.to_string(),
        recovery_class: error.recovery_class().to_string(),
        candidate_culprits: error.candidate_culprits().to_vec(),
    };

    let bytes = serde_json::to_vec(&payload).unwrap_or_else(|_| {
        b"{\"code\":\"internal_error\",\"message\":\"failed to encode error\",\"recovery_class\":\"terminal\"}".to_vec()
    });

    TbtcSignerResult {
        status_code: STATUS_ERROR,
        buffer: to_ffi_buffer(bytes),
    }
}

fn panic_payload_message(payload: Box<dyn std::any::Any + Send>) -> String {
    if let Some(message) = payload.downcast_ref::<&str>() {
        return (*message).to_string();
    }
    if let Some(message) = payload.downcast_ref::<String>() {
        return message.clone();
    }

    "non-string panic payload".to_string()
}

fn request_bytes<'a>(ptr: *const u8, len: usize) -> Result<&'a [u8], EngineError> {
    if len > MAX_REQUEST_BYTES {
        return Err(EngineError::Validation(format!(
            "request buffer length [{}] exceeds maximum [{}]",
            len, MAX_REQUEST_BYTES
        )));
    }

    if ptr.is_null() {
        return Err(EngineError::Validation(
            "request buffer pointer must be non-null".to_string(),
        ));
    }

    unsafe { Ok(std::slice::from_raw_parts(ptr, len)) }
}

fn to_ffi_buffer(mut bytes: Vec<u8>) -> TbtcBuffer {
    let len = bytes.len();
    if len == 0 {
        return TbtcBuffer {
            ptr: std::ptr::null_mut(),
            len: 0,
        };
    }

    // Copy into an exact-capacity boxed slice, then wipe the source Vec.
    // `bytes.into_boxed_slice()` shrink-to-fits and reallocates when capacity > len
    // (serde_json::to_vec over-allocates secret-bearing JSON), which would free the
    // original secret buffer WITHOUT zeroizing it. free_buffer wipes the boxed
    // slice on free; we wipe the source here so no un-zeroized copy survives.
    let boxed: Box<[u8]> = bytes.as_slice().into();
    bytes.zeroize();
    let ptr = Box::into_raw(boxed) as *mut u8;

    TbtcBuffer { ptr, len }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ffi_buffer_free_handles_vec_capacity_greater_than_len() {
        let mut payload = Vec::with_capacity(1024);
        payload.extend_from_slice(b"ok");
        assert!(payload.capacity() > payload.len());

        let result = success_from_serialized(payload);
        assert_eq!(result.status_code, STATUS_OK);
        assert_eq!(result.buffer.len, 2);

        let bytes = unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len) };
        assert_eq!(bytes, b"ok");

        free_buffer(result.buffer.ptr, result.buffer.len);
    }

    #[test]
    fn request_bytes_rejects_payloads_above_max_without_dereferencing() {
        let err = request_bytes(
            std::ptr::NonNull::<u8>::dangling().as_ptr(),
            MAX_REQUEST_BYTES + 1,
        )
        .expect_err("oversized request should be rejected");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("exceeds maximum"),
            "unexpected validation message: {message}"
        );
    }

    // A panic payload may carry internal detail (paths, config). It must be
    // withheld from the host in production and only surfaced under the
    // development profile. Serialized under the shared test-state lock because
    // the profile is read from a process-global env var.
    #[test]
    fn ffi_entry_redacts_panic_payload_in_production_and_preserves_in_development() {
        let _guard = crate::engine::lock_test_state();
        let secret_detail = "panic detail with /secret/path and a config value";

        let decode_message = |result: &TbtcSignerResult| -> String {
            assert_eq!(result.status_code, STATUS_ERROR);
            let bytes = unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len) };
            let response: ErrorResponse =
                serde_json::from_slice(bytes).expect("decode error response");
            assert_eq!(response.code, "internal_error");
            response.message
        };

        // Production (and any non-development profile): payload withheld.
        std::env::set_var(
            crate::engine::TBTC_SIGNER_PROFILE_ENV,
            crate::engine::TBTC_SIGNER_PROFILE_PRODUCTION,
        );
        let result = ffi_entry(|| -> Result<Vec<u8>, EngineError> {
            panic!("{secret_detail}");
        });
        let message = decode_message(&result);
        assert!(
            !message.contains(secret_detail),
            "production must not reflect the panic payload to the host: {message}"
        );
        assert!(
            message.contains("panic crossed FFI boundary"),
            "production should still report a generic boundary panic: {message}"
        );
        free_buffer(result.buffer.ptr, result.buffer.len);

        // Development: payload preserved for operator diagnostics.
        std::env::set_var(
            crate::engine::TBTC_SIGNER_PROFILE_ENV,
            crate::engine::TBTC_SIGNER_PROFILE_DEVELOPMENT,
        );
        let result = ffi_entry(|| -> Result<Vec<u8>, EngineError> {
            panic!("{secret_detail}");
        });
        let message = decode_message(&result);
        assert!(
            message.contains(secret_detail),
            "development must preserve the panic payload: {message}"
        );
        free_buffer(result.buffer.ptr, result.buffer.len);
    }

    // Regression guard for the C6 FFI-macro refactor. The three outliers
    // (`frost_tbtc_version`, `frost_tbtc_abi_version`, `frost_tbtc_free_buffer`)
    // must stay hand-written: the spec deliberately excludes them from the
    // `ffi_request_response!` / `ffi_no_request!` / `ffi_no_request_infallible!`
    // macros because each has a distinct shape (success-only probe, inline
    // `FrostTbtcAbiVersionResult` construction, and a `void`-returning free
    // helper respectively). An accidental future refactor that routes any of
    // them through a macro must fail this test. The check is source-text
    // rather than runtime because the contract being guarded is "the source
    // still looks like the spec said it should".
    #[test]
    fn outliers_are_hand_written() {
        let source = include_str!("lib.rs");

        for symbol in [
            "frost_tbtc_version",
            "frost_tbtc_abi_version",
            "frost_tbtc_free_buffer",
        ] {
            // (1) The symbol must NOT be invoked via any of the three FFI macros.
            // Each invocation has the form `<macro>!(<symbol>, ...)` or
            // `<macro>!(<symbol>)`; either is captured by the `<macro>!(<symbol>`
            // prefix check.
            for macro_invocation in [
                "ffi_request_response!",
                "ffi_no_request!",
                "ffi_no_request_infallible!",
            ] {
                let bad_pattern = format!("{}({}", macro_invocation, symbol);
                assert!(
                    !source.contains(&bad_pattern),
                    "outlier {symbol} must remain hand-written, but source contains `{bad_pattern}`",
                );
            }

            // (2) The function definition must be present and have an adjacent
            // `// Outlier:` explanatory comment. The window covers the
            // preceding text back through the `#[no_mangle]` attribute (which
            // always sits between the comment block and the `fn` line).
            let fn_signature = format!("fn {symbol}");
            let fn_offset = source
                .find(&fn_signature)
                .unwrap_or_else(|| panic!("{symbol} not found in lib.rs source"));
            let window_start = fn_offset.saturating_sub(400);
            let preceding = &source[window_start..fn_offset];
            assert!(
                preceding.contains("// Outlier"),
                "outlier {symbol} must have a `// Outlier:` explanatory comment above its definition",
            );
        }
    }
}
