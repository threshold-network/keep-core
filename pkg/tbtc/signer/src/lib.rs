mod api;
mod engine;
mod errors;
mod ffi;
mod go_math_rand;

use api::{
    BuildTaprootTxRequest, DeriveInteractiveAttemptContextRequest, DifferentialFuzzRequest,
    DkgPart1Request, DkgPart2Request, DkgPart3Request, FrostTbtcAbiVersionResult,
    InitSignerConfigRequest, InteractiveAggregateRequest, InteractiveRound1Request,
    InteractiveRound2Request, InteractiveSessionAbortRequest, InteractiveSessionOpenRequest,
    NewSigningPackageRequest, PersistDistributedDkgKeyPackageRequest, PromoteCanaryRequest,
    QuarantineStatusRequest, RefreshCadenceStatusRequest, RefreshSharesRequest,
    RollbackCanaryRequest, TranscriptAuditRequest, TriggerEmergencyRekeyRequest,
    VerifyBlameProofRequest,
};
use ffi::{
    ffi_entry, free_buffer, parse_request, serialize_response, success_from_string,
    TbtcSignerResult,
};

pub use ffi::TbtcBuffer;

const TBTC_SIGNER_VERSION: &str = "tbtc-signer/0.1.0-bootstrap";

/// The FFI CONTRACT version (see api::FrostTbtcAbiVersionResult), reported by
/// frost_tbtc_abi_version so a Go bridge can fail closed against an incompatible lib.
/// Starts at 1.0 - NOT 0.x, to avoid semver pre-1.0 ambiguity - distinct from the
/// human-readable TBTC_SIGNER_VERSION string above, which stays informational only.
///
/// Bump rules: bump TBTC_SIGNER_ABI_MAJOR on ANY incompatible change to the Go<->Rust
/// contract; bump TBTC_SIGNER_ABI_MINOR on a purely ADDITIVE, backward-compatible
/// change. A minor bump is valid ONLY if old consumers safely ignore the addition - if
/// a new field or enum value can appear in an existing response that an old bridge does
/// not tolerate, that is a MAJOR bump.
// Major 3: BuildTaprootTx inputs now require the spent output's script_pubkey_hex
// and results carry the ordered BIP-341 key-spend SIGHASH_DEFAULT messages. The
// required request field is an incompatible wire-contract change, so bridges and
// the signer library must move from major 2 to major 3 in lockstep.
const TBTC_SIGNER_ABI_MAJOR: u32 = 3;
// Minor 1 adds an optional, narrowly typed heartbeat intent to Interactive Open.
// ABI-3.0 callers remain valid because an absent intent preserves transaction-only
// signing-policy behavior.
// Minor 2 adds the optional heartbeat rate-limit config field and a dedicated
// heartbeat policy-rejection metric. Older callers safely omit/ignore both.
const TBTC_SIGNER_ABI_MINOR: u32 = 2;
#[cfg(test)]
use engine::TBTC_SIGNER_PROFILE_ENV;

/// FFI ownership contract:
/// - On return, `TbtcSignerResult.buffer` (if non-null) is owned by the caller.
/// - The caller must release that buffer exactly once via `frost_tbtc_free_buffer`.
#[no_mangle]
pub extern "C" fn frost_tbtc_version() -> TbtcSignerResult {
    success_from_string(TBTC_SIGNER_VERSION.to_string())
}

/// Reports the structured FFI CONTRACT version (api::FrostTbtcAbiVersionResult JSON)
/// so a Go bridge can fail closed on an incompatible libfrost_tbtc. See
/// TBTC_SIGNER_ABI_MAJOR / TBTC_SIGNER_ABI_MINOR for the bump rules. This response
/// shape is the ROOT compatibility surface and must stay stable: {abi_major, abi_minor}
/// as unsigned integers.
#[no_mangle]
pub extern "C" fn frost_tbtc_abi_version() -> TbtcSignerResult {
    ffi_entry(|| {
        serialize_response(&FrostTbtcAbiVersionResult {
            abi_major: TBTC_SIGNER_ABI_MAJOR,
            abi_minor: TBTC_SIGNER_ABI_MINOR,
        })
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_init_signer_config(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: InitSignerConfigRequest = parse_request(request_ptr, request_len)?;
        let response = engine::init_signer_config(request)?;
        serialize_response(&response)
    })
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
pub extern "C" fn frost_tbtc_dkg_part1(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: DkgPart1Request = parse_request(request_ptr, request_len)?;
        let response = engine::dkg_part1(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_dkg_part2(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: DkgPart2Request = parse_request(request_ptr, request_len)?;
        let response = engine::dkg_part2(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_dkg_part3(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: DkgPart3Request = parse_request(request_ptr, request_len)?;
        let response = engine::dkg_part3(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_persist_distributed_dkg_key_package(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: PersistDistributedDkgKeyPackageRequest =
            parse_request(request_ptr, request_len)?;
        let response = engine::persist_distributed_dkg_key_package(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_new_signing_package(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: NewSigningPackageRequest = parse_request(request_ptr, request_len)?;
        let response = engine::new_signing_package(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_verify_signature_share(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: crate::api::VerifySignatureShareRequest =
            parse_request(request_ptr, request_len)?;
        let response = engine::verify_signature_share(request)?;
        serialize_response(&response)
    })
}

// Phase 7.1 hardened interactive signing session (frozen spec
// docs/phase-7-interactive-session-spec-freeze.md). Additive ABI: the
// Go host adopts these in Phase 7.3; nothing breaks until it calls
// them. Secret nonces never cross this boundary in either direction.

#[no_mangle]
pub extern "C" fn frost_tbtc_interactive_session_open(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: InteractiveSessionOpenRequest = parse_request(request_ptr, request_len)?;
        let response = engine::interactive_session_open(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_interactive_round1(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: InteractiveRound1Request = parse_request(request_ptr, request_len)?;
        let response = engine::interactive_round1(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_interactive_round2(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: InteractiveRound2Request = parse_request(request_ptr, request_len)?;
        let response = engine::interactive_round2(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_interactive_session_abort(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: InteractiveSessionAbortRequest = parse_request(request_ptr, request_len)?;
        let response = engine::interactive_session_abort(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_interactive_aggregate(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: InteractiveAggregateRequest = parse_request(request_ptr, request_len)?;
        let response = engine::interactive_aggregate(request)?;
        serialize_response(&response)
    })
}

#[no_mangle]
pub extern "C" fn frost_tbtc_derive_interactive_attempt_context(
    request_ptr: *const u8,
    request_len: usize,
) -> TbtcSignerResult {
    ffi_entry(|| {
        let request: DeriveInteractiveAttemptContextRequest =
            parse_request(request_ptr, request_len)?;
        let response = engine::derive_interactive_attempt_context(request)?;
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
    use bitcoin::secp256k1::XOnlyPublicKey;
    use pretty_assertions::assert_eq;

    use crate::api::{
        BuildTaprootTxRequest, CanaryRolloutStatusResult, DifferentialFuzzRequest,
        DifferentialFuzzResult, DkgPart1Request, DkgPart1Result, DkgPart2Request, DkgPart2Result,
        DkgPart3Request, DkgPart3Result, DkgRound1Package, DkgRound2Package, ErrorResponse,
        FrostTbtcAbiVersionResult, PromoteCanaryRequest, QuarantineStatusRequest,
        QuarantineStatusResult, RefreshCadenceStatusRequest, RefreshCadenceStatusResult,
        RefreshSharesRequest, RoastLivenessPolicyResult, RollbackCanaryRequest, ShareMaterial,
        SignerHardeningMetricsResult, TransactionResult, TranscriptAuditRequest,
        TriggerEmergencyRekeyRequest, VerifyBlameProofRequest,
    };
    use crate::{
        frost_tbtc_abi_version, frost_tbtc_build_taproot_tx, frost_tbtc_canary_rollout_status,
        frost_tbtc_dkg_part1, frost_tbtc_dkg_part2, frost_tbtc_dkg_part3, frost_tbtc_free_buffer,
        frost_tbtc_hardening_metrics, frost_tbtc_promote_canary, frost_tbtc_quarantine_status,
        frost_tbtc_refresh_cadence_status, frost_tbtc_refresh_shares,
        frost_tbtc_roast_liveness_policy, frost_tbtc_roast_transcript_audit,
        frost_tbtc_rollback_canary, frost_tbtc_run_differential_fuzzing,
        frost_tbtc_trigger_emergency_rekey, frost_tbtc_verify_blame_proof,
    };

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

    fn taproot_prevout_script_hex() -> String {
        format!("5120{}", "33".repeat(32))
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
    fn interactive_session_ffi_dispatch_smoke() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();
        let _profile_env = EnvVarGuard::set(super::TBTC_SIGNER_PROFILE_ENV, "development");
        let _provenance_env = EnvVarGuard::set("TBTC_SIGNER_ENFORCE_PROVENANCE_GATE", "false");

        // Structurally valid requests whose semantics fail: proves
        // symbol -> parse -> engine -> structured-error dispatch for
        // every Phase 7.1 export without standing up a signing fixture
        // (the engine tests own the cryptographic contracts).
        let open = crate::api::InteractiveSessionOpenRequest {
            session_id: "ffi-interactive-smoke".to_string(),
            member_identifier: 1,
            message_hex: "11".repeat(32),
            key_group: "ffi-smoke-key-group".to_string(),
            threshold: 2,
            taproot_merkle_root_hex: None,
            signing_intent: None,
            attempt_context: crate::api::AttemptContext {
                attempt_number: 1,
                coordinator_identifier: 1,
                included_participants: vec![1, 2],
                included_participants_fingerprint: "00".to_string(),
                attempt_id: "ffi-smoke-attempt".to_string(),
            },
        };
        // No wallet key exists for this key_group, so Open fails closed with
        // dkg_not_ready (key material is resolved from engine DKG state by
        // key_group, never the request).
        let (status, payload) = call_ffi(&open, super::frost_tbtc_interactive_session_open);
        assert_ne!(status, 0);
        let error: ErrorResponse = serde_json::from_slice(&payload).expect("open error payload");
        assert_eq!(error.code, "dkg_not_ready");

        let round1 = crate::api::InteractiveRound1Request {
            session_id: "ffi-interactive-smoke-missing".to_string(),
            attempt_id: "missing".to_string(),
            member_identifier: 1,
        };
        let (status, payload) = call_ffi(&round1, super::frost_tbtc_interactive_round1);
        assert_ne!(status, 0);
        let error: ErrorResponse = serde_json::from_slice(&payload).expect("round1 error payload");
        assert_eq!(error.code, "session_not_found");

        let round2 = crate::api::InteractiveRound2Request {
            session_id: "ffi-interactive-smoke-missing".to_string(),
            attempt_id: "missing".to_string(),
            member_identifier: 1,
            signing_package_hex: "00".to_string(),
        };
        let (status, payload) = call_ffi(&round2, super::frost_tbtc_interactive_round2);
        assert_ne!(status, 0);
        let error: ErrorResponse = serde_json::from_slice(&payload).expect("round2 error payload");
        assert_eq!(error.code, "validation_error");

        let abort = crate::api::InteractiveSessionAbortRequest {
            session_id: "ffi-interactive-smoke-missing".to_string(),
            attempt_id: None,
        };
        let (status, payload) = call_ffi(&abort, super::frost_tbtc_interactive_session_abort);
        assert_eq!(status, 0);
        let result: crate::api::InteractiveSessionAbortResult =
            serde_json::from_slice(&payload).expect("abort result payload");
        assert!(!result.aborted);

        // Aggregate fails closed: the malformed signing package is
        // rejected at parse (before the session lookup), proving the
        // symbol -> parse -> engine -> structured-error dispatch.
        let aggregate = crate::api::InteractiveAggregateRequest {
            session_id: "ffi-interactive-smoke-missing".to_string(),
            attempt_id: "missing".to_string(),
            signing_package_hex: "00".to_string(),
            signature_shares: vec![crate::api::NativeFrostSignatureShare {
                identifier: "00".to_string(),
                data_hex: "00".to_string(),
            }],
            taproot_merkle_root_hex: None,
        };
        let (status, payload) = call_ffi(&aggregate, super::frost_tbtc_interactive_aggregate);
        assert_ne!(status, 0);
        let error: ErrorResponse =
            serde_json::from_slice(&payload).expect("aggregate error payload");
        assert_eq!(error.code, "validation_error");
    }

    #[test]
    fn derive_interactive_attempt_context_ffi_roundtrip() {
        // Hermetic env (development profile, provenance gate off) so the new
        // front-door gate does not reject; the engine tests own the gate's
        // fail-closed behavior.
        let _guard = crate::engine::lock_test_state();
        // Stateless + secret-free: unlike the session calls above, a valid
        // request SUCCEEDS with no DKG fixture, proving the
        // symbol -> parse -> engine -> serialize_response SUCCESS path for the
        // derivation export (the engine tests own the derivation contract).
        let request = crate::api::DeriveInteractiveAttemptContextRequest {
            session_id: "ffi-derive-smoke".to_string(),
            message_hex: "11".repeat(32),
            key_group: "ffi-derive-key-group".to_string(),
            threshold: 2,
            attempt_number: 1,
            included_participants: vec![3, 1, 2],
        };
        let (status, payload) = call_ffi(
            &request,
            super::frost_tbtc_derive_interactive_attempt_context,
        );
        assert_eq!(status, 0);
        let result: crate::api::DeriveInteractiveAttemptContextResult =
            serde_json::from_slice(&payload).expect("derive result payload");
        assert_eq!(result.attempt_context.included_participants, vec![1, 2, 3]);
        assert_eq!(result.attempt_context.attempt_number, 1);
        assert!(result.attempt_context.coordinator_identifier >= 1);
        assert_eq!(
            result
                .attempt_context
                .included_participants_fingerprint
                .len(),
            64
        );
        assert_eq!(result.attempt_context.attempt_id.len(), 64);
        assert_eq!(result.frost_identifiers.len(), 3);
    }

    fn native_frost_identifier(member_index: u8) -> String {
        let mut identifier = [0u8; 32];
        identifier[0] = member_index;
        serde_json::to_string(&hex::encode(identifier))
            .expect("identifier JSON encoding cannot fail")
    }

    #[test]
    fn dkg_part_ffi_roundtrip_produces_consistent_key_material() {
        // Serialize with every other env-touching test. This test mutates
        // process-global TBTC_SIGNER_* env vars (profile, provenance gate),
        // and env is shared across all parallel test threads; without the
        // lock its EnvVarGuard set/restore races with the serialized tests
        // and can leak a `production` profile into a concurrent state test,
        // which then panics while holding the engine lock and poisons it.
        // Declared first so it drops last - after the EnvVarGuards restore.
        let _guard = crate::engine::lock_test_state();
        let _profile_env = EnvVarGuard::set(super::TBTC_SIGNER_PROFILE_ENV, "development");
        let _provenance_env = EnvVarGuard::set("TBTC_SIGNER_ENFORCE_PROVENANCE_GATE", "false");

        let participant_ids = [1u8, 2u8, 3u8];
        let participant_identifiers: std::collections::BTreeMap<u8, String> = participant_ids
            .iter()
            .map(|id| (*id, native_frost_identifier(*id)))
            .collect();

        let mut part1_results = std::collections::BTreeMap::new();
        for id in participant_ids {
            let request = DkgPart1Request {
                participant_identifier: participant_identifiers[&id].clone(),
                max_signers: 3,
                min_signers: 2,
            };
            let (status, payload) = call_ffi(&request, frost_tbtc_dkg_part1);
            assert_eq!(status, 0);
            let result: DkgPart1Result =
                serde_json::from_slice(&payload).expect("part1 response decode");
            assert_eq!(result.package.identifier, participant_identifiers[&id]);
            assert!(!result.secret_package_hex.is_empty());
            assert!(!result.package.package_hex.is_empty());
            part1_results.insert(id, result);
        }

        let mut part2_results = std::collections::BTreeMap::new();
        for id in participant_ids {
            let round1_packages: Vec<DkgRound1Package> = participant_ids
                .iter()
                .filter(|other_id| **other_id != id)
                .map(|other_id| part1_results[other_id].package.clone())
                .collect();
            let request = DkgPart2Request {
                secret_package_hex: part1_results[&id].secret_package_hex.clone(),
                round1_packages,
            };
            let (status, payload) = call_ffi(&request, frost_tbtc_dkg_part2);
            assert_eq!(status, 0);
            let result: DkgPart2Result =
                serde_json::from_slice(&payload).expect("part2 response decode");
            assert_eq!(result.packages.len(), 2);
            assert!(result
                .packages
                .iter()
                .all(|pkg| pkg.sender_identifier.is_none()));
            part2_results.insert(id, result);
        }

        let mut part3_results = std::collections::BTreeMap::new();
        for id in participant_ids {
            let round1_packages: Vec<DkgRound1Package> = participant_ids
                .iter()
                .filter(|other_id| **other_id != id)
                .map(|other_id| part1_results[other_id].package.clone())
                .collect();
            let round2_packages: Vec<DkgRound2Package> = participant_ids
                .iter()
                .filter(|sender_id| **sender_id != id)
                .map(|sender_id| {
                    let mut package = part2_results[sender_id]
                        .packages
                        .iter()
                        .find(|pkg| pkg.identifier == participant_identifiers[&id])
                        .expect("round2 package for recipient")
                        .clone();
                    package.sender_identifier = Some(participant_identifiers[sender_id].clone());
                    package
                })
                .collect();
            let request = DkgPart3Request {
                secret_package_hex: part2_results[&id].secret_package_hex.clone(),
                round1_packages,
                round2_packages,
            };
            let (status, payload) = call_ffi(&request, frost_tbtc_dkg_part3);
            assert_eq!(status, 0);
            let result: DkgPart3Result =
                serde_json::from_slice(&payload).expect("part3 response decode");
            assert_eq!(result.key_package.identifier, participant_identifiers[&id]);
            assert_eq!(result.public_key_package.verifying_key.len(), 64);
            assert_eq!(result.public_key_package.verifying_shares.len(), 3);
            part3_results.insert(id, result);
        }

        let verifying_key = part3_results[&1].public_key_package.verifying_key.clone();
        for id in participant_ids {
            assert_eq!(
                part3_results[&id].public_key_package.verifying_key,
                verifying_key
            );
            assert_eq!(
                part3_results[&id].public_key_package.verifying_shares,
                part3_results[&1].public_key_package.verifying_shares
            );
        }

        // The exported DKG group key is a valid BIP-340 x-only public key.
        // The signing round trip that used to consume it through the removed
        // stateless FFI ops now lives in the engine tests, which drive the
        // frost primitives directly and verify a BIP-340 signature end to end.
        let public_key_bytes = hex::decode(verifying_key).expect("verifying key hex");
        assert_eq!(public_key_bytes.len(), 32);
        XOnlyPublicKey::from_slice(&public_key_bytes).expect("x-only public key");
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
    fn abi_version_reports_the_contract_version() {
        let (status, payload) = call_ffi_no_input(frost_tbtc_abi_version);
        assert_eq!(status, 0);

        let abi: FrostTbtcAbiVersionResult =
            serde_json::from_slice(&payload).expect("abi version payload decode");
        // The enforced FFI contract starts at 1.0; bump deliberately per the
        // TBTC_SIGNER_ABI_MAJOR / TBTC_SIGNER_ABI_MINOR rules. This test pins the
        // current value so an accidental bump is caught. BuildTaprootTx now requires
        // prevout scripts and returns BIP-341 SIGHASH_DEFAULT messages; the
        // incompatible request shape is ABI 3. Optional typed heartbeat intent is
        // the first backward-compatible minor addition; its independent rate-limit
        // config and rejection metric are the second.
        assert_eq!(abi.abi_major, 3);
        assert_eq!(abi.abi_minor, 2);
    }

    #[test]
    fn interactive_heartbeat_intent_has_the_pinned_wire_shape() {
        let intent = crate::api::InteractiveSigningIntent::Heartbeat {
            message_hex: "ff".repeat(8) + &"00".repeat(8),
        };
        assert_eq!(
            serde_json::to_value(intent).expect("heartbeat intent serializes"),
            serde_json::json!({
                "type": "heartbeat",
                "message_hex": "ffffffffffffffff0000000000000000"
            })
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
        assert_eq!(metrics_before.build_taproot_tx_calls_total, 0);
        assert_eq!(metrics_before.build_taproot_tx_success_total, 0);
        assert_eq!(metrics_before.heartbeat_signing_policy_reject_total, 0);
        assert_eq!(metrics_before.refresh_shares_calls_total, 0);
        assert_eq!(metrics_before.refresh_shares_success_total, 0);
        assert_eq!(metrics_before.build_taproot_tx_latency_samples, 0);
        assert_eq!(metrics_before.build_taproot_tx_latency_p95_ms, 0);

        let build_request = BuildTaprootTxRequest {
            session_id: "hardening-metrics-session".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: 10_000,
                script_pubkey_hex: taproot_prevout_script_hex(),
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 9_000,
            }],
            script_tree_hex: None,
        };
        let (build_status, _) = call_ffi(&build_request, frost_tbtc_build_taproot_tx);
        assert_eq!(build_status, 0);

        let (status_after, payload_after) = call_ffi_no_input(frost_tbtc_hardening_metrics);
        assert_eq!(status_after, 0);
        let metrics_after: SignerHardeningMetricsResult =
            serde_json::from_slice(&payload_after).expect("metrics payload decode");
        assert_eq!(metrics_after.build_taproot_tx_calls_total, 1);
        assert_eq!(metrics_after.build_taproot_tx_success_total, 1);
        assert_eq!(metrics_after.refresh_shares_calls_total, 0);
        assert_eq!(metrics_after.refresh_shares_success_total, 0);
        assert_eq!(metrics_after.build_taproot_tx_latency_samples, 1);
        assert!(metrics_after.build_taproot_tx_latency_p95_ms >= 1);
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
    fn emergency_rekey_blocks_build_taproot_tx_for_session() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let build_request = BuildTaprootTxRequest {
            session_id: "session-emergency-rekey".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: 10_000,
                script_pubkey_hex: taproot_prevout_script_hex(),
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 9_000,
            }],
            script_tree_hex: None,
        };
        // A successful build creates+persists the session so the emergency
        // rekey trigger has a target to mark.
        let (build_status, _) = call_ffi(&build_request, frost_tbtc_build_taproot_tx);
        assert_eq!(build_status, 0);

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

        // Once rekey is required, build_taproot_tx for the session is blocked
        // before it can reach the cached result.
        let (blocked_status, blocked_payload) =
            call_ffi(&build_request, frost_tbtc_build_taproot_tx);
        assert_eq!(blocked_status, 1);
        let blocked_error: ErrorResponse =
            serde_json::from_slice(&blocked_payload).expect("blocked error decode");
        assert_eq!(blocked_error.code, "lifecycle_policy_rejected");

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
    fn build_taproot_tx_is_idempotent_and_conflict_checked() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-tx".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 1,
                value_sats: 10_000,
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                script_pubkey_hex: taproot_prevout_script_hex(),
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
                    script_pubkey_hex: taproot_prevout_script_hex(),
                },
                crate::api::TxInput {
                    txid_hex: "11".repeat(32),
                    vout: 1,
                    value_sats: 10_000,
                    script_pubkey_hex: taproot_prevout_script_hex(),
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
                super::engine::truthy_env_flag(value),
                expected,
                "unexpected bootstrap-mode flag classification for [{value:?}]",
            );
        }
    }

    #[test]
    fn init_signer_config_ffi_round_trip_installs_and_reports_fingerprint() {
        let _guard = crate::engine::lock_test_state();
        crate::engine::reset_for_tests();

        let request = crate::api::InitSignerConfigRequest {
            profile: Some("development".to_string()),
            roast_coordinator_timeout_ms: Some(45_000),
            policy_heartbeat_rate_limit_per_minute: Some(12),
            ..crate::api::InitSignerConfigRequest::default()
        };
        let (status, response_bytes) = call_ffi(&request, crate::frost_tbtc_init_signer_config);
        assert_eq!(
            status,
            0,
            "init must succeed: {:?}",
            String::from_utf8_lossy(&response_bytes)
        );

        let response: crate::api::InitSignerConfigResult =
            serde_json::from_slice(&response_bytes).expect("response parses");
        assert!(response.installed);
        assert!(!response.idempotent);
        assert_eq!(response.configured_key_count, 3);
        assert!(!response.config_fingerprint.is_empty());

        // Clear the installed snapshot so env-driven tests are unaffected.
        crate::engine::reset_for_tests();
    }
}
