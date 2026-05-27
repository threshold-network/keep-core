mod api;
mod engine;
mod errors;
mod ffi;
mod go_math_rand;

#[cfg(test)]
use std::sync::OnceLock;

use api::{
    BuildTaprootTxRequest, DifferentialFuzzRequest, FinalizeSignRoundRequest, PromoteCanaryRequest,
    QuarantineStatusRequest, RefreshCadenceStatusRequest, RefreshSharesRequest,
    RollbackCanaryRequest, RunDkgRequest, StartSignRoundRequest, TranscriptAuditRequest,
    TriggerEmergencyRekeyRequest, VerifyBlameProofRequest,
};
use ffi::{
    ffi_entry, free_buffer, parse_request, serialize_response, success_from_string,
    TbtcSignerResult,
};

pub use ffi::TbtcBuffer;

const TBTC_SIGNER_VERSION: &str = "tbtc-signer/0.1.0-bootstrap";
const TBTC_SIGNER_ALLOW_BOOTSTRAP_ENV: &str = "TBTC_SIGNER_ALLOW_BOOTSTRAP";
const TBTC_SIGNER_PROFILE_ENV: &str = "TBTC_SIGNER_PROFILE";
const TBTC_SIGNER_PROFILE_PRODUCTION: &str = "production";
const TBTC_SIGNER_PROFILE_DEVELOPMENT: &str = "development";
#[cfg(test)]
static TEST_BOOTSTRAP_MODE_OVERRIDE: OnceLock<std::sync::Mutex<Option<bool>>> = OnceLock::new();

fn bootstrap_mode_flag_enabled(raw_value: &str) -> bool {
    matches!(
        raw_value.trim().to_ascii_lowercase().as_str(),
        "1" | "true" | "yes" | "on"
    )
}

fn bootstrap_mode_enabled_from_env() -> bool {
    if signer_profile_is_production() {
        return false;
    }

    std::env::var(TBTC_SIGNER_ALLOW_BOOTSTRAP_ENV)
        .map(|raw_value| bootstrap_mode_flag_enabled(&raw_value))
        .unwrap_or(false)
}

fn signer_profile_is_production() -> bool {
    let raw = std::env::var(TBTC_SIGNER_PROFILE_ENV).unwrap_or_default();
    let normalized = raw.trim().to_ascii_lowercase();
    match normalized.as_str() {
        TBTC_SIGNER_PROFILE_PRODUCTION => true,
        // See engine::signer_profile_is_production for the rationale: missing
        // is tolerated as development so parallel cargo test runs don't race
        // on env mutation, but typos still panic so operators cannot silently
        // bypass production-mode gates via `TBTC_SIGNER_PROFILE=prod`.
        TBTC_SIGNER_PROFILE_DEVELOPMENT | "" => false,
        other => panic!(
            "{} must be '{}' or '{}'; got {:?}",
            TBTC_SIGNER_PROFILE_ENV,
            TBTC_SIGNER_PROFILE_PRODUCTION,
            TBTC_SIGNER_PROFILE_DEVELOPMENT,
            other
        ),
    }
}

#[cfg(test)]
fn test_bootstrap_mode_override() -> &'static std::sync::Mutex<Option<bool>> {
    TEST_BOOTSTRAP_MODE_OVERRIDE.get_or_init(|| std::sync::Mutex::new(None))
}

fn bootstrap_mode_enabled() -> bool {
    #[cfg(test)]
    {
        if let Some(value) = *test_bootstrap_mode_override()
            .lock()
            .expect("bootstrap mode override lock poisoned")
        {
            return value;
        }
    }

    bootstrap_mode_enabled_from_env()
}

/// FFI ownership contract:
/// - On return, `TbtcSignerResult.buffer` (if non-null) is owned by the caller.
/// - The caller must release that buffer exactly once via `frost_tbtc_free_buffer`.
#[no_mangle]
pub extern "C" fn frost_tbtc_version() -> TbtcSignerResult {
    success_from_string(TBTC_SIGNER_VERSION.to_string())
}

#[no_mangle]
pub extern "C" fn frost_tbtc_roast_liveness_policy() -> TbtcSignerResult {
    ffi_entry(|| serialize_response(&engine::roast_liveness_policy()))
}

#[no_mangle]
pub extern "C" fn frost_tbtc_hardening_metrics() -> TbtcSignerResult {
    ffi_entry(|| serialize_response(&engine::hardening_metrics()))
}

#[no_mangle]
pub extern "C" fn frost_tbtc_roast_transcript_audit(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: TranscriptAuditRequest = parse_request(request_ptr, request_len)?;
        let response = engine::roast_transcript_audit(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_verify_blame_proof(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: VerifyBlameProofRequest = parse_request(request_ptr, request_len)?;
        let response = engine::verify_blame_proof(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_quarantine_status(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: QuarantineStatusRequest = parse_request(request_ptr, request_len)?;
        let response = engine::quarantine_status(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_refresh_cadence_status(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: RefreshCadenceStatusRequest = parse_request(request_ptr, request_len)?;
        let response = engine::refresh_cadence_status(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_trigger_emergency_rekey(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: TriggerEmergencyRekeyRequest = parse_request(request_ptr, request_len)?;
        let response = engine::trigger_emergency_rekey(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_run_differential_fuzzing(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: DifferentialFuzzRequest = parse_request(request_ptr, request_len)?;
        let response = engine::run_differential_fuzzing(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_canary_rollout_status() -> TbtcSignerResult {
    ffi_entry(|| {
        let response = engine::canary_rollout_status()?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_promote_canary(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: PromoteCanaryRequest = parse_request(request_ptr, request_len)?;
        let response = engine::promote_canary(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_rollback_canary(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: RollbackCanaryRequest = parse_request(request_ptr, request_len)?;
        let response = engine::rollback_canary(request)?;
        serialize_response(&response)
    })
}

#[cfg(any(test, feature = "bench-restart-hook"))]
#[doc(hidden)]
pub fn frost_tbtc_reload_state_from_storage_for_benchmarks() -> Result<(), String> {
    engine::reload_state_from_storage_for_benchmarks().map_err(|error| error.to_string())
}

#[no_mangle]
pub extern "C" fn frost_tbtc_free_buffer(ptr: *mut u8, len: usize) {
    free_buffer(ptr, len)
}

#[no_mangle]
pub extern "C" fn frost_tbtc_run_dkg(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: RunDkgRequest = parse_request(request_ptr, request_len)?;
        let response = engine::run_dkg(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_start_sign_round(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: StartSignRoundRequest = parse_request(request_ptr, request_len)?;
        let response = engine::start_sign_round(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_finalize_sign_round(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: FinalizeSignRoundRequest = parse_request(request_ptr, request_len)?;
        let response = engine::finalize_sign_round(request, bootstrap_mode_enabled())?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_build_taproot_tx(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: BuildTaprootTxRequest = parse_request(request_ptr, request_len)?;
        let response = engine::build_taproot_tx(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_refresh_shares(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: RefreshSharesRequest = parse_request(request_ptr, request_len)?;
        let response = engine::refresh_shares(request)?;
        serialize_response(&response)
    })
}

#[cfg(test)]
mod tests {
    use bitcoin::consensus::encode::deserialize;
    use pretty_assertions::assert_eq;
    use sha2::{Digest, Sha256};

    use crate::api::{
        BuildTaprootTxRequest, CanaryRolloutStatusResult, DifferentialFuzzRequest,
        DifferentialFuzzResult, DkgParticipant, ErrorResponse, FinalizeSignRoundRequest,
        PromoteCanaryRequest, QuarantineStatusRequest, QuarantineStatusResult,
        RefreshCadenceStatusRequest, RefreshCadenceStatusResult, RefreshSharesRequest,
        RoastLivenessPolicyResult, RollbackCanaryRequest, RoundContribution, RunDkgRequest,
        ShareMaterial, SignerHardeningMetricsResult, StartSignRoundRequest, TransactionResult,
        TranscriptAuditRequest, TriggerEmergencyRekeyRequest, VerifyBlameProofRequest,
    };
    use crate::{
        frost_tbtc_build_taproot_tx, frost_tbtc_canary_rollout_status,
        frost_tbtc_finalize_sign_round, frost_tbtc_free_buffer, frost_tbtc_hardening_metrics,
        frost_tbtc_promote_canary, frost_tbtc_quarantine_status, frost_tbtc_refresh_cadence_status,
        frost_tbtc_refresh_shares, frost_tbtc_roast_liveness_policy,
        frost_tbtc_roast_transcript_audit, frost_tbtc_rollback_canary,
        frost_tbtc_run_differential_fuzzing, frost_tbtc_run_dkg, frost_tbtc_start_sign_round,
        frost_tbtc_trigger_emergency_rekey, frost_tbtc_verify_blame_proof,
    };

    fn bootstrap_synthetic_share_hex(
        round_state: &crate::api::RoundState,
        identifier: u16,
    ) -> String {
        let mut hasher = Sha256::new();
        hasher.update(
            format!(
                "tbtc-signer-bootstrap-contribution-v1:{}:{}:{}:{}",
                round_state.session_id,
                round_state.round_id,
                round_state.message_digest_hex,
                identifier
            )
            .as_bytes(),
        );
        hex::encode(hasher.finalize())
    }

    fn call_ffi<T: serde::Serialize>(
        request: &T,
        f: extern "C" fn(*const u8, usize) -> crate::ffi::TbtcSignerResult,
    ) -> (i32, Vec<u8>) {
        let bytes = serde_json::to_vec(request).expect("request serialization");
        let result = f(bytes.as_ptr(), bytes.len());

        let response_bytes = if result.buffer.ptr.is_null() || result.buffer.len == 0 {
            Vec::new()
        } else {
            unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len).to_vec() }
        };

        frost_tbtc_free_buffer(result.buffer.ptr, result.buffer.len);
        (result.status_code, response_bytes)
    }

    fn call_ffi_no_input(f: extern "C" fn() -> crate::ffi::TbtcSignerResult) -> (i32, Vec<u8>) {
        let result = f();

        let response_bytes = if result.buffer.ptr.is_null() || result.buffer.len == 0 {
            Vec::new()
        } else {
            unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len).to_vec() }
        };

        frost_tbtc_free_buffer(result.buffer.ptr, result.buffer.len);
        (result.status_code, response_bytes)
    }

    struct BootstrapModeGuard {
        previous_value: Option<bool>,
    }

    impl BootstrapModeGuard {
        fn set(value: Option<bool>) -> Self {
            let mut guard = super::test_bootstrap_mode_override()
                .lock()
                .expect("bootstrap mode override lock poisoned");
            let previous_value = *guard;
            *guard = value;

            Self { previous_value }
        }

        fn enable() -> Self {
            Self::set(Some(true))
        }

        fn disable() -> Self {
            Self::set(Some(false))
        }
    }

    impl Drop for BootstrapModeGuard {
        fn drop(&mut self) {
            let mut guard = super::test_bootstrap_mode_override()
                .lock()
                .expect("bootstrap mode override lock poisoned");
            *guard = self.previous_value;
        }
    }

    struct EnvVarGuard {
        key: &'static str,
        previous_value: Option<String>,
    }

    impl EnvVarGuard {
        fn set(key: &'static str, value: &str) -> Self {
            let previous_value = std::env::var(key).ok();
            std::env::set_var(key, value);

            Self {
                key,
                previous_value,
            }
        }

        #[allow(dead_code)]
        fn unset(key: &'static str) -> Self {
            let previous_value = std::env::var(key).ok();
            std::env::remove_var(key);

            Self {
                key,
                previous_value,
            }
        }
    }

    impl Drop for EnvVarGuard {
        fn drop(&mut self) {
            match &self.previous_value {
                Some(value) => std::env::set_var(self.key, value),
                None => std::env::remove_var(self.key),
            }
        }
    }

    #[test]
    fn run_dkg_is_idempotent_for_identical_request() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = RunDkgRequest {
            session_id: "session-a".to_string(),
            participants: vec![
                DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };

        let (status_first, first_payload) = call_ffi(&request, frost_tbtc_run_dkg);
        let (status_second, second_payload) = call_ffi(&request, frost_tbtc_run_dkg);

        assert_eq!(status_first, 0);
        assert_eq!(status_second, 0);
        assert_eq!(first_payload, second_payload);
    }

    #[test]
    fn run_dkg_rejects_conflicting_repeat_request_for_same_session() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request_a = RunDkgRequest {
            session_id: "session-conflict".to_string(),
            participants: vec![
                DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let mut request_b = request_a.clone();
        request_b.participants.push(DkgParticipant {
            identifier: 3,
            public_key_hex: "02cc".to_string(),
        });

        let (status_first, _) = call_ffi(&request_a, frost_tbtc_run_dkg);
        let (status_second, payload_second) = call_ffi(&request_b, frost_tbtc_run_dkg);

        assert_eq!(status_first, 0);
        assert_eq!(status_second, 1);

        let error: ErrorResponse =
            serde_json::from_slice(&payload_second).expect("error payload decode");
        assert_eq!(error.code, "session_conflict");
        assert_eq!(error.recovery_class, "recoverable");
    }

    #[test]
    fn roast_liveness_policy_reports_default_contract() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let (status, payload) = call_ffi_no_input(frost_tbtc_roast_liveness_policy);
        assert_eq!(status, 0);

        let policy: RoastLivenessPolicyResult =
            serde_json::from_slice(&payload).expect("policy payload decode");
        assert_eq!(policy.coordinator_timeout_ms, 30_000);
        assert_eq!(policy.timeout_source, "keep_core_wall_clock");
        assert_eq!(policy.advance_trigger, "coordinator_timeout");
        assert_eq!(
            policy.exclusion_evidence_policy,
            "timeout_or_invalid_share_proof"
        );
    }

    #[test]
    fn hardening_metrics_reports_runtime_and_counters() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let (status_before, payload_before) = call_ffi_no_input(frost_tbtc_hardening_metrics);
        assert_eq!(status_before, 0);
        let metrics_before: SignerHardeningMetricsResult =
            serde_json::from_slice(&payload_before).expect("metrics payload decode");
        assert!(!metrics_before.runtime_version.is_empty());
        assert_eq!(metrics_before.run_dkg_calls_total, 0);
        assert_eq!(metrics_before.run_dkg_success_total, 0);
        assert_eq!(metrics_before.start_sign_round_calls_total, 0);
        assert_eq!(metrics_before.start_sign_round_success_total, 0);
        assert_eq!(metrics_before.refresh_shares_calls_total, 0);
        assert_eq!(metrics_before.refresh_shares_success_total, 0);
        assert_eq!(metrics_before.run_dkg_latency_samples, 0);
        assert_eq!(metrics_before.run_dkg_latency_p95_ms, 0);

        let dkg_request = RunDkgRequest {
            session_id: "hardening-metrics-session".to_string(),
            participants: vec![
                DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let (dkg_status, _) = call_ffi(&dkg_request, frost_tbtc_run_dkg);
        assert_eq!(dkg_status, 0);

        let (status_after, payload_after) = call_ffi_no_input(frost_tbtc_hardening_metrics);
        assert_eq!(status_after, 0);
        let metrics_after: SignerHardeningMetricsResult =
            serde_json::from_slice(&payload_after).expect("metrics payload decode");
        assert_eq!(metrics_after.run_dkg_calls_total, 1);
        assert_eq!(metrics_after.run_dkg_success_total, 1);
        assert_eq!(metrics_after.start_sign_round_calls_total, 0);
        assert_eq!(metrics_after.start_sign_round_success_total, 0);
        assert_eq!(metrics_after.refresh_shares_calls_total, 0);
        assert_eq!(metrics_after.refresh_shares_success_total, 0);
        assert_eq!(metrics_after.run_dkg_latency_samples, 1);
        assert!(metrics_after.run_dkg_latency_p95_ms >= 1);
    }

    #[test]
    fn quarantine_status_reports_default_disabled_state() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = QuarantineStatusRequest {
            operator_identifier: 1,
        };
        let (status, payload) = call_ffi(&request, frost_tbtc_quarantine_status);
        assert_eq!(status, 0);

        let result: QuarantineStatusResult =
            serde_json::from_slice(&payload).expect("quarantine status payload decode");
        assert_eq!(result.operator_identifier, 1);
        assert!(!result.auto_quarantine_enabled);
        assert_eq!(result.fault_score, 0);
        assert_eq!(result.quarantine_threshold, 0);
        assert!(!result.quarantined);
    }

    #[test]
    fn transcript_endpoints_return_session_not_found_for_unknown_session() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let audit_request = TranscriptAuditRequest {
            session_id: "missing-session".to_string(),
        };
        let (audit_status, audit_payload) =
            call_ffi(&audit_request, frost_tbtc_roast_transcript_audit);
        assert_eq!(audit_status, 1);
        let audit_error: ErrorResponse =
            serde_json::from_slice(&audit_payload).expect("audit error decode");
        assert_eq!(audit_error.code, "session_not_found");

        let verify_request = VerifyBlameProofRequest {
            session_id: "missing-session".to_string(),
            from_attempt_number: 1,
            accused_member_identifier: 1,
            reason: "coordinator_timeout".to_string(),
            invalid_share_proof_fingerprint: None,
        };
        let (verify_status, verify_payload) =
            call_ffi(&verify_request, frost_tbtc_verify_blame_proof);
        assert_eq!(verify_status, 1);
        let verify_error: ErrorResponse =
            serde_json::from_slice(&verify_payload).expect("verify error decode");
        assert_eq!(verify_error.code, "session_not_found");
    }

    #[test]
    fn refresh_cadence_status_returns_session_not_found_for_unknown_session() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = RefreshCadenceStatusRequest {
            session_id: "missing-refresh-session".to_string(),
        };
        let (status, payload) = call_ffi(&request, frost_tbtc_refresh_cadence_status);
        assert_eq!(status, 1);

        let error: ErrorResponse =
            serde_json::from_slice(&payload).expect("refresh cadence error decode");
        assert_eq!(error.code, "session_not_found");
    }

    #[test]
    fn differential_fuzzing_reports_no_critical_divergence() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = DifferentialFuzzRequest {
            seed: 0xD1FF_2026_0302_0001,
            case_count: 64,
        };
        let (status, payload) = call_ffi(&request, frost_tbtc_run_differential_fuzzing);
        assert_eq!(status, 0);

        let result: DifferentialFuzzResult =
            serde_json::from_slice(&payload).expect("differential fuzz payload decode");
        assert_eq!(result.case_count, 64);
        assert_eq!(result.critical_divergence_count, 0);
        assert!(!result.unresolved_critical_divergence);
    }

    #[test]
    fn canary_rollout_promote_and_rollback_roundtrip() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let (status_initial, payload_initial) = call_ffi_no_input(frost_tbtc_canary_rollout_status);
        assert_eq!(status_initial, 0);
        let initial: CanaryRolloutStatusResult =
            serde_json::from_slice(&payload_initial).expect("canary status decode");
        assert_eq!(initial.current_percent, 10);
        assert_eq!(initial.recommended_next_percent, Some(50));

        let promote_50 = PromoteCanaryRequest { target_percent: 50 };
        let (status_promote_50, payload_promote_50) =
            call_ffi(&promote_50, frost_tbtc_promote_canary);
        assert_eq!(status_promote_50, 0);
        let promoted_50: crate::api::PromoteCanaryResult =
            serde_json::from_slice(&payload_promote_50).expect("promote 50 decode");
        assert_eq!(promoted_50.from_percent, 10);
        assert_eq!(promoted_50.to_percent, 50);

        let promote_100 = PromoteCanaryRequest {
            target_percent: 100,
        };
        let (status_promote_100, payload_promote_100) =
            call_ffi(&promote_100, frost_tbtc_promote_canary);
        assert_eq!(status_promote_100, 0);
        let promoted_100: crate::api::PromoteCanaryResult =
            serde_json::from_slice(&payload_promote_100).expect("promote 100 decode");
        assert_eq!(promoted_100.from_percent, 50);
        assert_eq!(promoted_100.to_percent, 100);

        let rollback = RollbackCanaryRequest {
            reason: "slo regression".to_string(),
        };
        let (status_rollback, payload_rollback) = call_ffi(&rollback, frost_tbtc_rollback_canary);
        assert_eq!(status_rollback, 0);
        let rolled_back: crate::api::RollbackCanaryResult =
            serde_json::from_slice(&payload_rollback).expect("rollback decode");
        assert_eq!(rolled_back.from_percent, 100);
        assert_eq!(rolled_back.to_percent, 50);

        let (status_after, payload_after) = call_ffi_no_input(frost_tbtc_canary_rollout_status);
        assert_eq!(status_after, 0);
        let after: CanaryRolloutStatusResult =
            serde_json::from_slice(&payload_after).expect("canary status after rollback decode");
        assert_eq!(after.current_percent, 50);
    }

    #[test]
    fn emergency_rekey_blocks_start_sign_round_for_session() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let dkg_request = RunDkgRequest {
            session_id: "session-emergency-rekey".to_string(),
            participants: vec![
                DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let (dkg_status, dkg_payload) = call_ffi(&dkg_request, frost_tbtc_run_dkg);
        assert_eq!(dkg_status, 0);
        let dkg_result: crate::api::DkgResult =
            serde_json::from_slice(&dkg_payload).expect("dkg payload decode");

        let rekey_request = TriggerEmergencyRekeyRequest {
            session_id: "session-emergency-rekey".to_string(),
            reason: "key compromise drill".to_string(),
        };
        let (rekey_status, rekey_payload) =
            call_ffi(&rekey_request, frost_tbtc_trigger_emergency_rekey);
        assert_eq!(rekey_status, 0);
        let rekey_result: crate::api::TriggerEmergencyRekeyResult =
            serde_json::from_slice(&rekey_payload).expect("rekey payload decode");
        assert!(rekey_result.emergency_rekey_required);

        let start_request = StartSignRoundRequest {
            session_id: "session-emergency-rekey".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let (start_status, start_payload) = call_ffi(&start_request, frost_tbtc_start_sign_round);
        assert_eq!(start_status, 1);
        let start_error: ErrorResponse =
            serde_json::from_slice(&start_payload).expect("start error decode");
        assert_eq!(start_error.code, "lifecycle_policy_rejected");

        let cadence_request = RefreshCadenceStatusRequest {
            session_id: "session-emergency-rekey".to_string(),
        };
        let (cadence_status, cadence_payload) =
            call_ffi(&cadence_request, frost_tbtc_refresh_cadence_status);
        assert_eq!(cadence_status, 0);
        let cadence_result: RefreshCadenceStatusResult =
            serde_json::from_slice(&cadence_payload).expect("cadence status payload decode");
        assert!(cadence_result.emergency_rekey_required);
    }

    #[test]
    fn start_and_finalize_sign_round_support_idempotent_retries() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();
        let _bootstrap_mode_guard = BootstrapModeGuard::enable();

        let dkg = RunDkgRequest {
            session_id: "session-sign".to_string(),
            participants: vec![
                DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };

        let (dkg_status, dkg_payload) = call_ffi(&dkg, frost_tbtc_run_dkg);
        assert_eq!(dkg_status, 0);

        let dkg_result: crate::api::DkgResult =
            serde_json::from_slice(&dkg_payload).expect("dkg payload decode");

        let start = StartSignRoundRequest {
            session_id: "session-sign".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };

        let (start_status, start_payload) = call_ffi(&start, frost_tbtc_start_sign_round);
        assert_eq!(start_status, 0);

        let round_state: crate::api::RoundState =
            serde_json::from_slice(&start_payload).expect("round payload decode");

        let finalize = FinalizeSignRoundRequest {
            session_id: "session-sign".to_string(),
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
        };

        let (finalize_status_first, finalize_payload_first) =
            call_ffi(&finalize, frost_tbtc_finalize_sign_round);
        let (finalize_status_second, finalize_payload_second) =
            call_ffi(&finalize, frost_tbtc_finalize_sign_round);

        assert_eq!(finalize_status_first, 0);
        assert_eq!(finalize_status_second, 0);
        assert_eq!(finalize_payload_first, finalize_payload_second);

        let signature: crate::api::SignatureResult =
            serde_json::from_slice(&finalize_payload_first).expect("signature payload decode");
        assert_eq!(signature.round_id, round_state.round_id);
    }

    #[test]
    fn start_and_finalize_sign_round_rejects_synthetic_contributions_when_bootstrap_disabled() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();
        let _bootstrap_mode_guard = BootstrapModeGuard::disable();

        let dkg = RunDkgRequest {
            session_id: "session-sign-bootstrap-disabled".to_string(),
            participants: vec![
                DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };

        let (dkg_status, dkg_payload) = call_ffi(&dkg, frost_tbtc_run_dkg);
        assert_eq!(dkg_status, 0);

        let dkg_result: crate::api::DkgResult =
            serde_json::from_slice(&dkg_payload).expect("dkg payload decode");

        let start = StartSignRoundRequest {
            session_id: "session-sign-bootstrap-disabled".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };

        let (start_status, start_payload) = call_ffi(&start, frost_tbtc_start_sign_round);
        assert_eq!(start_status, 0);

        let round_state: crate::api::RoundState =
            serde_json::from_slice(&start_payload).expect("round payload decode");

        let finalize = FinalizeSignRoundRequest {
            session_id: "session-sign-bootstrap-disabled".to_string(),
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
        };

        let (finalize_status, finalize_payload) =
            call_ffi(&finalize, frost_tbtc_finalize_sign_round);
        assert_eq!(finalize_status, 1);

        let error: ErrorResponse =
            serde_json::from_slice(&finalize_payload).expect("error payload decode");
        assert_eq!(error.code, "synthetic_contribution_rejected");
        assert_eq!(error.recovery_class, "recoverable");
    }

    #[test]
    fn start_sign_round_returns_session_conflict_for_non_finalized_payload_mismatch() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let dkg = RunDkgRequest {
            session_id: "session-sign-conflict".to_string(),
            participants: vec![
                DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let (dkg_status, dkg_payload) = call_ffi(&dkg, frost_tbtc_run_dkg);
        assert_eq!(dkg_status, 0);
        let dkg_result: crate::api::DkgResult =
            serde_json::from_slice(&dkg_payload).expect("dkg payload decode");

        let start_first = StartSignRoundRequest {
            session_id: "session-sign-conflict".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let (start_first_status, _) = call_ffi(&start_first, frost_tbtc_start_sign_round);
        assert_eq!(start_first_status, 0);

        let start_second = StartSignRoundRequest {
            session_id: "session-sign-conflict".to_string(),
            member_identifier: 1,
            message_hex: "cafebabe".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![2, 1]),
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let (start_second_status, start_second_payload) =
            call_ffi(&start_second, frost_tbtc_start_sign_round);
        assert_eq!(start_second_status, 1);

        let error: ErrorResponse =
            serde_json::from_slice(&start_second_payload).expect("error payload decode");
        assert_eq!(error.code, "session_conflict");
        assert_eq!(error.recovery_class, "recoverable");
    }

    #[test]
    fn start_sign_round_returns_session_finalized_after_finalize() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();
        let _bootstrap_mode_guard = BootstrapModeGuard::enable();

        let dkg = RunDkgRequest {
            session_id: "session-sign-finalized".to_string(),
            participants: vec![
                DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let (dkg_status, dkg_payload) = call_ffi(&dkg, frost_tbtc_run_dkg);
        assert_eq!(dkg_status, 0);
        let dkg_result: crate::api::DkgResult =
            serde_json::from_slice(&dkg_payload).expect("dkg payload decode");

        let start = StartSignRoundRequest {
            session_id: "session-sign-finalized".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let (start_status, start_payload) = call_ffi(&start, frost_tbtc_start_sign_round);
        assert_eq!(start_status, 0);
        let round_state: crate::api::RoundState =
            serde_json::from_slice(&start_payload).expect("round payload decode");

        let finalize = FinalizeSignRoundRequest {
            session_id: "session-sign-finalized".to_string(),
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
        };
        let (finalize_status, _) = call_ffi(&finalize, frost_tbtc_finalize_sign_round);
        assert_eq!(finalize_status, 0);

        let (restart_status, restart_payload) = call_ffi(&start, frost_tbtc_start_sign_round);
        assert_eq!(restart_status, 1);
        let error: ErrorResponse =
            serde_json::from_slice(&restart_payload).expect("error payload decode");
        assert_eq!(error.code, "session_finalized");
        assert_eq!(error.recovery_class, "terminal");
    }

    #[test]
    fn start_sign_round_returns_session_not_found_for_unknown_session() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let start = StartSignRoundRequest {
            session_id: "session-sign-missing".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: "missing".to_string(),
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };

        let (status, payload) = call_ffi(&start, frost_tbtc_start_sign_round);
        assert_eq!(status, 1);
        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload decode");
        assert_eq!(error.code, "session_not_found");
        assert_eq!(error.recovery_class, "terminal");
    }

    #[test]
    fn build_taproot_tx_is_idempotent_and_conflict_checked() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 1,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 9_000,
            }],
            script_tree_hex: None,
        };

        let (status_first, payload_first) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        let (status_second, payload_second) = call_ffi(&request, frost_tbtc_build_taproot_tx);

        assert_eq!(status_first, 0);
        assert_eq!(status_second, 0);
        assert_eq!(payload_first, payload_second);

        let result: TransactionResult =
            serde_json::from_slice(&payload_first).expect("transaction payload decode");
        assert_eq!(result.session_id, "session-tx");
        let tx_bytes = hex::decode(&result.tx_hex).expect("decode tx hex");
        let tx: bitcoin::Transaction = deserialize(&tx_bytes).expect("decode transaction");
        assert_eq!(tx.input.len(), 1);
        assert_eq!(tx.output.len(), 1);

        let conflict_request = BuildTaprootTxRequest {
            session_id: "session-tx".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 1,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 8_000,
            }],
            script_tree_hex: None,
        };

        let (conflict_status, conflict_payload) =
            call_ffi(&conflict_request, frost_tbtc_build_taproot_tx);
        assert_eq!(conflict_status, 1);

        let error: ErrorResponse =
            serde_json::from_slice(&conflict_payload).expect("conflict error payload");
        assert_eq!(error.code, "session_conflict");
        assert_eq!(error.recovery_class, "recoverable");
    }

    #[test]
    fn build_taproot_tx_rejects_script_tree_payload() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx-script-tree".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 1,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 9_000,
            }],
            script_tree_hex: Some("00".to_string()),
        };

        let (status, payload) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        assert_eq!(status, 1);

        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload");
        assert_eq!(error.code, "validation_error");
        assert!(error
            .message
            .contains("script_tree_hex is not yet supported"));
    }

    #[test]
    fn build_taproot_tx_rejects_overspend_outputs() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx-overspend".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 1,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 10_001,
            }],
            script_tree_hex: None,
        };

        let (status, payload) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        assert_eq!(status, 1);

        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload");
        assert_eq!(error.code, "validation_error");
        assert!(error.message.contains("exceeds input value_sats total"));
    }

    #[test]
    fn build_taproot_tx_rejects_output_total_above_bitcoin_max_money() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let max_money_outputs: Vec<crate::api::TxOutput> = (0..9_000)
            .map(|index| crate::api::TxOutput {
                script_pubkey_hex: format!("5120{:064x}", index + 1),
                value_sats: 2_100_000_000_000_000,
            })
            .collect();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx-max-money-output-sum".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 1,
                value_sats: 2_100_000_000_000_000,
            }],
            outputs: max_money_outputs,
            script_tree_hex: None,
        };

        let (status, payload) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        assert_eq!(status, 1);

        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload");
        assert_eq!(error.code, "validation_error");
        assert!(error
            .message
            .contains("output value_sats total [4200000000000000] exceeds Bitcoin max money"));
    }

    #[test]
    fn build_taproot_tx_rejects_input_total_above_bitcoin_max_money() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let max_money_inputs: Vec<crate::api::TxInput> = (0..9_000)
            .map(|index| crate::api::TxInput {
                txid_hex: format!("{:064x}", index + 1),
                vout: 0,
                value_sats: 2_100_000_000_000_000,
            })
            .collect();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx-max-money-input-sum".to_string(),
            inputs: max_money_inputs,
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 1,
            }],
            script_tree_hex: None,
        };

        let (status, payload) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        assert_eq!(status, 1);

        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload");
        assert_eq!(error.code, "validation_error");
        assert!(error
            .message
            .contains("input value_sats total [4200000000000000] exceeds Bitcoin max money"));
    }

    #[test]
    fn build_taproot_tx_rejects_output_value_above_bitcoin_max_money() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx-output-above-max-money".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 1,
                value_sats: 2_100_000_000_000_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 2_100_000_000_000_001,
            }],
            script_tree_hex: None,
        };

        let (status, payload) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        assert_eq!(status, 1);

        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload");
        assert_eq!(error.code, "validation_error");
        assert!(error
            .message
            .contains("output value_sats [2100000000000001] exceeds Bitcoin max money"));
    }

    #[test]
    fn build_taproot_tx_rejects_input_value_above_bitcoin_max_money() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx-input-above-max-money".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 1,
                value_sats: 2_100_000_000_000_001,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 1,
            }],
            script_tree_hex: None,
        };

        let (status, payload) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        assert_eq!(status, 1);

        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload");
        assert_eq!(error.code, "validation_error");
        assert!(error
            .message
            .contains("input value_sats [2100000000000001] exceeds Bitcoin max money"));
    }

    #[test]
    fn build_taproot_tx_rejects_invalid_input_txid_hex() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx-invalid-input-txid".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "zz".to_string(),
                vout: 1,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 1,
            }],
            script_tree_hex: None,
        };

        let (status, payload) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        assert_eq!(status, 1);

        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload");
        assert_eq!(error.code, "validation_error");
        assert_eq!(error.recovery_class, "recoverable");
        assert!(error.message.contains("invalid input txid_hex [zz]"));
    }

    #[test]
    fn build_taproot_tx_rejects_malformed_output_script() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx-malformed-output-script".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 1,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                // OP_PUSHDATA1 length=2 with only one data byte.
                script_pubkey_hex: "4c02aa".to_string(),
                value_sats: 1,
            }],
            script_tree_hex: None,
        };

        let (status, payload) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        assert_eq!(status, 1);

        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload");
        assert_eq!(error.code, "validation_error");
        assert!(error
            .message
            .contains("invalid output script_pubkey_hex [4c02aa]"));
    }

    #[test]
    fn build_taproot_tx_rejects_duplicate_inputs() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx-duplicate-inputs".to_string(),
            inputs: vec![
                crate::api::TxInput {
                    txid_hex: "11".repeat(32),
                    vout: 1,
                    value_sats: 10_000,
                },
                crate::api::TxInput {
                    txid_hex: "11".repeat(32),
                    vout: 1,
                    value_sats: 10_000,
                },
            ],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 10_000,
            }],
            script_tree_hex: None,
        };

        let (status, payload) = call_ffi(&request, frost_tbtc_build_taproot_tx);
        assert_eq!(status, 1);

        let error: ErrorResponse = serde_json::from_slice(&payload).expect("error payload");
        assert_eq!(error.code, "validation_error");
        assert!(error.message.contains("duplicate input outpoint"));
    }

    #[test]
    fn refresh_shares_is_idempotent() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = RefreshSharesRequest {
            session_id: "session-refresh".to_string(),
            current_shares: vec![
                ShareMaterial {
                    identifier: 1,
                    encrypted_share_hex: "abcd".to_string(),
                },
                ShareMaterial {
                    identifier: 2,
                    encrypted_share_hex: "ef01".to_string(),
                },
            ],
        };

        let (status_first, payload_first) = call_ffi(&request, frost_tbtc_refresh_shares);
        let (status_second, payload_second) = call_ffi(&request, frost_tbtc_refresh_shares);

        assert_eq!(status_first, 0);
        assert_eq!(status_second, 0);
        assert_eq!(payload_first, payload_second);
    }

    #[test]
    fn refresh_shares_uses_monotonic_epoch_counter() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request_first = RefreshSharesRequest {
            session_id: "session-refresh-epoch-1".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "1111".to_string(),
            }],
        };

        let request_second = RefreshSharesRequest {
            session_id: "session-refresh-epoch-2".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "2222".to_string(),
            }],
        };

        let (status_first, payload_first) = call_ffi(&request_first, frost_tbtc_refresh_shares);
        let (status_first_retry, payload_first_retry) =
            call_ffi(&request_first, frost_tbtc_refresh_shares);
        let (status_second, payload_second) = call_ffi(&request_second, frost_tbtc_refresh_shares);

        assert_eq!(status_first, 0);
        assert_eq!(status_first_retry, 0);
        assert_eq!(payload_first, payload_first_retry);
        assert_eq!(status_second, 0);

        let first_result: crate::api::RefreshSharesResult =
            serde_json::from_slice(&payload_first).expect("first refresh payload decode");
        let second_result: crate::api::RefreshSharesResult =
            serde_json::from_slice(&payload_second).expect("second refresh payload decode");

        assert_eq!(first_result.refresh_epoch, 1);
        assert_eq!(second_result.refresh_epoch, 2);
    }

    #[test]
    fn bootstrap_mode_flag_parser_is_strict() {
        let test_cases = vec![
            ("", false),
            ("0", false),
            ("false", false),
            (" bootstrap ", false),
            ("1", true),
            ("true", true),
            ("TRUE", true),
            ("yes", true),
            ("on", true),
            (" true ", true),
        ];

        for (value, expected) in test_cases {
            assert_eq!(
                super::bootstrap_mode_flag_enabled(value),
                expected,
                "unexpected bootstrap-mode flag classification for [{value:?}]",
            );
        }
    }

    #[test]
    fn bootstrap_mode_env_is_ignored_in_production_profile() {
        let _guard = crate::engine::lock_test_state();
        let _bootstrap_mode_guard = BootstrapModeGuard::set(None);
        let _allow_bootstrap_env = EnvVarGuard::set(super::TBTC_SIGNER_ALLOW_BOOTSTRAP_ENV, "true");
        let _profile_env = EnvVarGuard::set(super::TBTC_SIGNER_PROFILE_ENV, "production");

        assert!(!super::bootstrap_mode_enabled_from_env());
    }

    #[test]
    fn bootstrap_mode_rechecks_production_profile_each_call() {
        let _guard = crate::engine::lock_test_state();
        let _bootstrap_mode_guard = BootstrapModeGuard::set(None);
        let _allow_bootstrap_env = EnvVarGuard::set(super::TBTC_SIGNER_ALLOW_BOOTSTRAP_ENV, "true");
        let _profile_env = EnvVarGuard::set(super::TBTC_SIGNER_PROFILE_ENV, "development");

        assert!(super::bootstrap_mode_enabled());

        std::env::set_var(super::TBTC_SIGNER_PROFILE_ENV, "production");

        assert!(!super::bootstrap_mode_enabled());
    }
}
