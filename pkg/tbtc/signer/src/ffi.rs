use std::panic::{catch_unwind, AssertUnwindSafe};

use serde::de::DeserializeOwned;

use crate::api::ErrorResponse;
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

pub fn ffi_entry<F>(f: F) -> TbtcSignerResult
where
    F: FnOnce() -> Result<Vec<u8>, EngineError>,
{
    match catch_unwind(AssertUnwindSafe(f)) {
        Ok(Ok(bytes)) => success_from_serialized(bytes),
        Ok(Err(err)) => error_result(err),
        Err(payload) => error_result(EngineError::Internal(format!(
            "panic crossed FFI boundary: {}",
            panic_payload_message(payload)
        ))),
    }
}

pub fn free_buffer(ptr: *mut u8, len: usize) {
    if ptr.is_null() || len == 0 {
        return;
    }

    unsafe {
        drop(Box::from_raw(std::ptr::slice_from_raw_parts_mut(ptr, len)));
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

fn to_ffi_buffer(bytes: Vec<u8>) -> TbtcBuffer {
    let len = bytes.len();
    if len == 0 {
        return TbtcBuffer {
            ptr: std::ptr::null_mut(),
            len: 0,
        };
    }

    let boxed = bytes.into_boxed_slice();
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
    fn ffi_entry_preserves_string_panic_payload() {
        let result = ffi_entry(|| -> Result<Vec<u8>, EngineError> {
            panic!("TBTC_SIGNER_PROFILE must be production or development");
        });
        assert_eq!(result.status_code, STATUS_ERROR);

        let bytes = unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len) };
        let response: ErrorResponse = serde_json::from_slice(bytes).expect("decode error response");
        assert_eq!(response.code, "internal_error");
        assert!(
            response
                .message
                .contains("TBTC_SIGNER_PROFILE must be production or development"),
            "panic payload was not preserved: {}",
            response.message
        );

        free_buffer(result.buffer.ptr, result.buffer.len);
    }
}
