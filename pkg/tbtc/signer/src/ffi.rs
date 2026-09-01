use std::panic::{catch_unwind, AssertUnwindSafe};

use serde::de::DeserializeOwned;

use zeroize::Zeroize;

use crate::api::{ErrorResponse, StateAnchorTrustCheckpoint, StateAnchorTrustRecoveryRequired};
use crate::errors::EngineError;

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
        let default_hook = std::panic::take_hook();
        std::panic::set_hook(Box::new(move |info| {
            let development_profile =
                crate::engine::signer_env_var(crate::engine::TBTC_SIGNER_PROFILE_ENV)
                    .map(|raw| {
                        raw.trim()
                            .eq_ignore_ascii_case(crate::engine::TBTC_SIGNER_PROFILE_DEVELOPMENT)
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
        Err(payload) => {
            // `panic_boundary_message` already performs its own profile-aware
            // redaction specifically for this case (a fixed, safe "panic
            // crossed FFI boundary" marker in production, the full payload in
            // development). Route it through `error_response_bytes` directly
            // with that resolved message, bypassing `ffi_redacted_message`'s
            // generic `Internal` redaction - applying that a second time
            // would collapse the meaningful, already-safe "a panic occurred"
            // signal down to the same generic "detail redacted" text every
            // other Internal error produces, losing the distinction.
            let message = panic_boundary_message(payload);
            let response = error_response_bytes(&EngineError::Internal(String::new()), message);
            TbtcSignerResult {
                status_code: STATUS_ERROR,
                buffer: to_ffi_buffer(response),
            }
        }
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
    let message = ffi_redacted_message(&error);
    let bytes = error_response_bytes(&error, message);
    TbtcSignerResult {
        status_code: STATUS_ERROR,
        buffer: to_ffi_buffer(bytes),
    }
}

/// Builds the serialized `ErrorResponse` for `error`, using `message` as
/// the (already profile-resolved) message field. Split out of
/// `error_result` so the panic-boundary path in `ffi_entry` can supply its
/// own pre-redacted message (from `panic_boundary_message`) without routing
/// it through `ffi_redacted_message`'s generic `Internal` redaction a
/// second time.
fn error_response_bytes(error: &EngineError, message: String) -> Vec<u8> {
    let (requested_generation, witness_base_generation) = match error {
        EngineError::HistoryPruned {
            requested_generation,
            witness_base_generation,
        } => (Some(*requested_generation), Some(*witness_base_generation)),
        _ => (None, None),
    };
    let state_anchor_trust_recovery = error.state_anchor_trust_recovery().map(|context| {
        let bytes32 = |value: [u8; 32]| format!("0x{}", hex::encode(value));
        let certificate_count = context.certificate_digests.len();
        debug_assert_eq!(
            certificate_count,
            context.certificate_sequences.len(),
            "verified recovery selector vectors remain aligned"
        );
        StateAnchorTrustRecoveryRequired {
            schema: "tbtc-signer-state-anchor-trust-recovery-required/v1".to_string(),
            store_fingerprint: bytes32(context.store_fingerprint),
            certificate_count: certificate_count.to_string(),
            first_certificate_sequence: context
                .certificate_sequences
                .first()
                .copied()
                .unwrap_or_default()
                .to_string(),
            ordered_certificate_digests: context
                .certificate_digests
                .iter()
                .copied()
                .map(bytes32)
                .collect(),
            final_certificate_sequence: context
                .certificate_sequences
                .last()
                .copied()
                .unwrap_or_default()
                .to_string(),
            final_certificate_digest: context
                .certificate_digests
                .last()
                .copied()
                .map(bytes32)
                .unwrap_or_else(|| bytes32([0u8; 32])),
            target_binding_hash: bytes32(context.target_binding_hash),
            target_service_epoch: context.target_service_epoch.to_string(),
            target_revision: context.target_revision.to_string(),
            target_checkpoint: StateAnchorTrustCheckpoint {
                store_fingerprint: bytes32(context.target_checkpoint_store_fingerprint),
                generation: context.target_checkpoint_generation.to_string(),
                previous_state_commitment: bytes32(
                    context.target_checkpoint_previous_state_commitment,
                ),
                state_image_digest: bytes32(context.target_checkpoint_state_image_digest),
                state_commitment: bytes32(context.target_checkpoint_state_commitment),
            },
        }
    });
    let payload = ErrorResponse {
        code: error.code().to_string(),
        message,
        recovery_class: error.recovery_class().to_string(),
        requested_generation,
        witness_base_generation,
        candidate_culprits: error.candidate_culprits().to_vec(),
        state_anchor_trust_recovery,
    };

    serde_json::to_vec(&payload).unwrap_or_else(|_| {
        b"{\"code\":\"internal_error\",\"message\":\"failed to encode error\",\"recovery_class\":\"terminal\"}".to_vec()
    })
}

/// Returns the message string for `ErrorResponse`.
///
/// Only `Internal` carries absolute paths, syscall errors, env-var values,
/// or other host-identifying detail (it wraps ad hoc `format!(...)` text
/// built from filesystem operations throughout the engine). Its message is
/// replaced by a fixed redacted string outside the development profile.
/// Every other variant, including `Validation`, is user-facing
/// business-rule text by construction (bounds checks, schema mismatches,
/// malformed field errors) built from request fields and named constants,
/// never from filesystem paths or syscall errors - redacting it would
/// break the caller-facing feedback those variants exist to provide, for
/// no confidentiality benefit.
///
/// Profile detection uses `development_profile_active`, which reads the
/// profile env var directly and fails CLOSED for any missing or malformed
/// value. Routing through `signer_profile_is_production` would panic on a
/// malformed profile, which would convert this FFI error path into a second
/// panic across the C boundary - exactly the failure mode the panic hook
/// exists to prevent.
fn ffi_redacted_message(error: &EngineError) -> String {
    let rendered = error.to_string();
    if !matches!(error, EngineError::Internal(_)) {
        return rendered;
    }
    if crate::engine::development_profile_active() {
        rendered
    } else {
        "signer error detail redacted (see server log)".to_string()
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

    #[test]
    fn history_pruned_error_exposes_machine_readable_generations() {
        let result = error_result(EngineError::HistoryPruned {
            requested_generation: 41,
            witness_base_generation: 42,
        });
        assert_eq!(result.status_code, STATUS_ERROR);
        let bytes = unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len) };
        let response: ErrorResponse =
            serde_json::from_slice(bytes).expect("decode history-pruned error");
        assert_eq!(response.code, "history_pruned");
        assert_eq!(response.requested_generation, Some(41));
        assert_eq!(response.witness_base_generation, Some(42));
        free_buffer(result.buffer.ptr, result.buffer.len);
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
    // `Internal` messages carry path-bearing detail by construction (e.g.
    // `format!("failed to open [{}]: {e}", path.display())`). Production
    // must suppress that detail across the FFI boundary; development must
    // keep it verbatim for operator diagnostics. The redaction is applied
    // by the FFI error-result construction (`ffi_redacted_message`), not by
    // every call site - which is what makes a stray path in any of the ~30
    // `EngineError::Internal` constructors in store.rs and persistence.rs
    // still fail to leak. `Validation` is deliberately excluded: it is
    // user-facing business-rule text, never built from filesystem paths in
    // real code. Serialized under the shared test state lock because the
    // profile is a process-global env var.
    #[test]
    fn error_result_redacts_internal_paths_in_production_profile() {
        use std::path::PathBuf;

        let _guard = crate::engine::lock_test_state();

        let decode_message = |result: &TbtcSignerResult| -> String {
            assert_eq!(result.status_code, STATUS_ERROR);
            let bytes = unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len) };
            let response: ErrorResponse =
                serde_json::from_slice(bytes).expect("decode error response");
            response.message
        };

        // Production: `Internal` messages must not reflect either a
        // `Path::display()` rendering or a `PathBuf`-shaped `{:?}` rendering
        // to the host. Use a synthetic absolute path so the assertion cannot
        // accidentally pass on a benign prefix.
        let sensitive_path = PathBuf::from("/secret/absolute/signer-state-leak");
        std::env::set_var(
            crate::engine::TBTC_SIGNER_PROFILE_ENV,
            crate::engine::TBTC_SIGNER_PROFILE_PRODUCTION,
        );

        let internal_display = error_result(EngineError::Internal(format!(
            "failed to open signer state file at {}",
            sensitive_path.display()
        )));
        let internal_msg = decode_message(&internal_display);
        assert!(
            !internal_msg.contains(&sensitive_path.display().to_string()),
            "production Internal message leaked .display() path: {internal_msg}"
        );
        assert!(
            !internal_msg.contains("/secret/absolute"),
            "production Internal message leaked absolute path prefix: {internal_msg}"
        );
        free_buffer(internal_display.buffer.ptr, internal_display.buffer.len);

        let internal_debug = error_result(EngineError::Internal(format!(
            "failed to open signer state file at {sensitive_path:?}"
        )));
        let internal_msg = decode_message(&internal_debug);
        assert!(
            !internal_msg.contains("/secret/absolute"),
            "production Internal message leaked {{:?}}-formatted PathBuf: {internal_msg}"
        );
        free_buffer(internal_debug.buffer.ptr, internal_debug.buffer.len);

        // `Validation` is user-facing business-rule text by construction
        // (bounds checks, schema mismatches) and is never built from
        // filesystem paths in real code - it must pass through unchanged so
        // callers still see actionable validation feedback. This uses a
        // synthetic path only to prove the pass-through, not because real
        // `Validation` errors carry one.
        let validation_display = error_result(EngineError::Validation(format!(
            "validation rejected envelope at {}",
            sensitive_path.display()
        )));
        let validation_msg = decode_message(&validation_display);
        assert!(
            validation_msg.contains("/secret/absolute"),
            "Validation must pass through unchanged, even in production: {validation_msg}"
        );
        free_buffer(validation_display.buffer.ptr, validation_display.buffer.len);

        // Every other variant must NOT be redacted either: their messages
        // are bounded by construction (variant fields carry ids / sequences /
        // digests, not paths), so the host still receives the diagnostic that
        // tells it which condition was matched.
        let passthrough = error_result(EngineError::StateAnchorTrustHeadAbsent);
        let passthrough_msg = decode_message(&passthrough);
        assert!(
            passthrough_msg.contains("state-anchor trust head is absent"),
            "non-Internal variant must pass through unchanged: {passthrough_msg}"
        );
        free_buffer(passthrough.buffer.ptr, passthrough.buffer.len);

        // Development: every detail is preserved verbatim so operators see the
        // path / syscall detail that production redacts.
        std::env::set_var(
            crate::engine::TBTC_SIGNER_PROFILE_ENV,
            crate::engine::TBTC_SIGNER_PROFILE_DEVELOPMENT,
        );
        let dev_internal = error_result(EngineError::Internal(format!(
            "failed to open signer state file at {}",
            sensitive_path.display()
        )));
        let dev_msg = decode_message(&dev_internal);
        assert!(
            dev_msg.contains(sensitive_path.display().to_string().as_str()),
            "development Internal message must preserve the path: {dev_msg}"
        );
        assert!(
            dev_msg.contains("internal error:"),
            "development Internal message keeps the original Display prefix: {dev_msg}"
        );
        free_buffer(dev_internal.buffer.ptr, dev_internal.buffer.len);
    }
}
