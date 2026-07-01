use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Once, OnceLock};
use std::time::{SystemTime, UNIX_EPOCH};

use criterion::{black_box, criterion_group, criterion_main, BatchSize, Criterion};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

const MESSAGE_HEX: &str = "4b2f57fd3d2e4fd8d68abf9f6ba5e8d51f68de3a63f4f47c8aa2d43f0ca1bc52";
const ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN: &str = "FROST-ROAST-INCLUDED-FPR-v1";
const ROAST_ATTEMPT_ID_DOMAIN: &str = "FROST-ROAST-ATTEMPT-ID-v1";
const ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT: &str = "coordinator_timeout";
const ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF: &str = "invalid_share_proof";

static BENCH_ENV_INIT: Once = Once::new();
static SESSION_COUNTER: AtomicU64 = AtomicU64::new(1);
static BENCHMARK_COORDINATORS: OnceLock<BenchmarkCoordinators> = OnceLock::new();

macro_rules! call_raw {
    ($fn_name:path, $request:expr) => {{
        let request_bytes = serde_json::to_vec(&$request).expect("request serialization");
        let result = $fn_name(request_bytes.as_ptr(), request_bytes.len());
        let status_code = result.status_code;
        let response_bytes = if result.buffer.ptr.is_null() || result.buffer.len == 0 {
            Vec::new()
        } else {
            unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len).to_vec() }
        };
        frost_tbtc::frost_tbtc_free_buffer(result.buffer.ptr, result.buffer.len);

        (status_code, response_bytes)
    }};
}

macro_rules! call_json {
    ($fn_name:path, $request:expr) => {{
        let (status_code, response_bytes) = call_raw!($fn_name, $request);
        if status_code != 0 {
            panic!(
                "ffi call failed [{}]: {}",
                stringify!($fn_name),
                String::from_utf8_lossy(&response_bytes)
            );
        }

        response_bytes
    }};
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
struct DkgParticipant {
    identifier: u16,
    public_key_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
struct RunDkgRequest {
    session_id: String,
    participants: Vec<DkgParticipant>,
    threshold: u16,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq)]
struct DkgResult {
    key_group: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
struct AttemptContext {
    attempt_number: u32,
    coordinator_identifier: u16,
    included_participants: Vec<u16>,
    included_participants_fingerprint: String,
    attempt_id: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
struct AttemptExclusionEvidence {
    reason: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    excluded_member_identifiers: Vec<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    invalid_share_proof_fingerprint: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
struct AttemptTransitionEvidence {
    from_attempt_number: u32,
    from_attempt_id: String,
    from_coordinator_identifier: u16,
    previous_round_id: String,
    previous_sign_request_fingerprint: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    exclusion_evidence: Option<AttemptExclusionEvidence>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
struct StartSignRoundRequest {
    session_id: String,
    member_identifier: u16,
    message_hex: String,
    key_group: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    signing_participants: Option<Vec<u16>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    attempt_context: Option<AttemptContext>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    attempt_transition_evidence: Option<AttemptTransitionEvidence>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
struct RoundContribution {
    identifier: u16,
    signature_share_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq)]
struct AttemptTransitionTelemetry {
    reason: String,
    #[serde(default)]
    excluded_member_identifiers: Vec<u16>,
    coordinator_rotated: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq)]
struct RoundState {
    session_id: String,
    round_id: String,
    message_digest_hex: String,
    #[serde(default)]
    attempt_transition_telemetry: Option<AttemptTransitionTelemetry>,
    own_contribution: RoundContribution,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
struct FinalizeSignRoundRequest {
    session_id: String,
    round_contributions: Vec<RoundContribution>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    attempt_context: Option<AttemptContext>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq)]
struct SignatureResult {
    signature_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq)]
struct ErrorResponse {
    code: String,
    message: String,
    recovery_class: String,
}

#[derive(Clone, Debug)]
struct BenchmarkCoordinators {
    attempt_one_all_members: u16,
    attempt_two_all_members: u16,
}

fn hash_hex(bytes: &[u8]) -> String {
    hex::encode(Sha256::digest(bytes))
}

fn canonicalize_included_participants(mut included_participants: Vec<u16>) -> Vec<u16> {
    included_participants.sort_unstable();
    included_participants.dedup();
    assert!(
        included_participants
            .iter()
            .all(|identifier| *identifier != 0),
        "included participants must be non-zero"
    );
    included_participants
}

fn push_framed_component(payload: &mut Vec<u8>, component: &[u8]) {
    let component_len = u32::try_from(component.len()).expect("component length within u32");
    payload.extend_from_slice(&component_len.to_be_bytes());
    payload.extend_from_slice(component);
}

fn roast_hash_hex_with_components(domain: &str, components: &[&[u8]]) -> String {
    let mut payload = Vec::new();
    push_framed_component(&mut payload, domain.as_bytes());
    for component in components {
        push_framed_component(&mut payload, component);
    }

    hash_hex(&payload)
}

fn message_digest_hex() -> String {
    let message_bytes = hex::decode(MESSAGE_HEX).expect("message hex");
    hash_hex(&message_bytes)
}

fn roast_included_participants_fingerprint_hex(included_participants: &[u16]) -> String {
    let mut participant_payload = Vec::new();
    for participant_identifier in included_participants {
        push_framed_component(
            &mut participant_payload,
            &participant_identifier.to_be_bytes(),
        );
    }

    roast_hash_hex_with_components(
        ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN,
        &[&participant_payload],
    )
}

fn roast_attempt_id_hex(
    session_id: &str,
    message_digest_hex: &str,
    attempt_number: u32,
    coordinator_identifier: u16,
    included_participants_fingerprint_hex: &str,
) -> String {
    roast_hash_hex_with_components(
        ROAST_ATTEMPT_ID_DOMAIN,
        &[
            session_id.as_bytes(),
            message_digest_hex.as_bytes(),
            &attempt_number.to_be_bytes(),
            &coordinator_identifier.to_be_bytes(),
            included_participants_fingerprint_hex.as_bytes(),
        ],
    )
}

fn canonicalize_start_sign_round_request_for_fingerprint(request: &mut StartSignRoundRequest) {
    if let Some(signing_participants) = request.signing_participants.as_mut() {
        signing_participants.sort_unstable();
    }

    if let Some(attempt_context) = request.attempt_context.as_mut() {
        attempt_context.included_participants.sort_unstable();
        attempt_context.included_participants_fingerprint = attempt_context
            .included_participants_fingerprint
            .to_ascii_lowercase();
        attempt_context.attempt_id = attempt_context.attempt_id.to_ascii_lowercase();
    }

    if let Some(transition_evidence) = request.attempt_transition_evidence.as_mut() {
        transition_evidence.from_attempt_id = transition_evidence
            .from_attempt_id
            .trim()
            .to_ascii_lowercase();
        if let Some(exclusion_evidence) = transition_evidence.exclusion_evidence.as_mut() {
            exclusion_evidence.reason = exclusion_evidence.reason.trim().to_ascii_lowercase();
            exclusion_evidence
                .excluded_member_identifiers
                .sort_unstable();
            if let Some(proof_fingerprint) =
                exclusion_evidence.invalid_share_proof_fingerprint.as_mut()
            {
                *proof_fingerprint = proof_fingerprint.trim().to_ascii_lowercase();
            }
        }
    }
}

fn sign_request_fingerprint(request: &StartSignRoundRequest) -> String {
    let mut canonical_request = request.clone();
    canonicalize_start_sign_round_request_for_fingerprint(&mut canonical_request);
    let bytes = serde_json::to_vec(&canonical_request).expect("fingerprint request serialization");
    hash_hex(&bytes)
}

fn ensure_benchmark_environment() {
    BENCH_ENV_INIT.call_once(|| {
        let bench_nonce = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("unix epoch")
            .as_nanos();
        let state_path =
            std::env::temp_dir().join(format!("frost_tbtc_phase5_bench_state_{bench_nonce}.json"));
        let _ = std::fs::remove_file(&state_path);

        std::env::set_var("TBTC_SIGNER_STATE_PATH", &state_path);
        std::env::set_var("TBTC_SIGNER_MAX_SESSIONS", "200000");
        std::env::set_var("TBTC_SIGNER_ALLOW_BOOTSTRAP", "true");
        std::env::set_var("TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK", "true");
        // The signer treats a missing profile as production, and the default
        // `env` state-key provider requires an encryption key for persistence.
        // Seed both (and pin the provider) so the README-documented
        // `cargo bench --features bench-restart-hook --bench phase5_roast`
        // runs in a clean shell without any pre-set TBTC_SIGNER_* variables;
        // otherwise the first RunDkg persist fails.
        std::env::set_var("TBTC_SIGNER_PROFILE", "development");
        std::env::set_var("TBTC_SIGNER_STATE_KEY_PROVIDER", "env");
        std::env::set_var(
            "TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX",
            "0c9258935f0a30c065befcd746cb1564e9f3c91936c0f0f1c78853fa2d6713dc",
        );
    });
}

fn next_session_id(prefix: &str) -> String {
    let index = SESSION_COUNTER.fetch_add(1, Ordering::Relaxed);
    format!("phase5-bench-{prefix}-{index}")
}

fn run_dkg(session_id: &str) -> DkgResult {
    let request = RunDkgRequest {
        session_id: session_id.to_string(),
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

    serde_json::from_slice(&call_json!(frost_tbtc::frost_tbtc_run_dkg, request))
        .expect("dkg response")
}

fn build_attempt_context(
    session_id: &str,
    attempt_number: u32,
    coordinator_identifier: u16,
    included_participants: Vec<u16>,
) -> AttemptContext {
    let canonical_included_participants = canonicalize_included_participants(included_participants);
    let included_participants_fingerprint =
        roast_included_participants_fingerprint_hex(&canonical_included_participants);
    let attempt_id = roast_attempt_id_hex(
        session_id,
        &message_digest_hex(),
        attempt_number,
        coordinator_identifier,
        &included_participants_fingerprint,
    );

    AttemptContext {
        attempt_number,
        coordinator_identifier,
        included_participants: canonical_included_participants,
        included_participants_fingerprint,
        attempt_id,
    }
}

fn probe_deterministic_coordinator(attempt_number: u32, included_participants: Vec<u16>) -> u16 {
    let canonical_included_participants = canonicalize_included_participants(included_participants);
    let probe_session_id = next_session_id("coord-probe");
    let dkg_result = run_dkg(&probe_session_id);

    let mut errors = Vec::new();
    for candidate in &canonical_included_participants {
        let request = StartSignRoundRequest {
            session_id: probe_session_id.clone(),
            member_identifier: 1,
            message_hex: MESSAGE_HEX.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(canonical_included_participants.clone()),
            attempt_context: Some(build_attempt_context(
                &probe_session_id,
                attempt_number,
                *candidate,
                canonical_included_participants.clone(),
            )),
            attempt_transition_evidence: None,
        };

        let (status_code, response_bytes) =
            call_raw!(frost_tbtc::frost_tbtc_start_sign_round, request);
        if status_code == 0 {
            return *candidate;
        }

        errors.push(String::from_utf8_lossy(&response_bytes).to_string());
    }

    panic!(
        "failed to resolve deterministic coordinator for attempt [{}] participants {:?}: {}",
        attempt_number,
        canonical_included_participants,
        errors.join(" | ")
    );
}

fn benchmark_coordinators() -> &'static BenchmarkCoordinators {
    BENCHMARK_COORDINATORS.get_or_init(|| BenchmarkCoordinators {
        attempt_one_all_members: probe_deterministic_coordinator(1, vec![1, 2, 3]),
        attempt_two_all_members: probe_deterministic_coordinator(2, vec![1, 2, 3]),
    })
}

fn participants_excluding(excluded_member_identifier: u16) -> Vec<u16> {
    canonicalize_included_participants(
        [1_u16, 2_u16, 3_u16]
            .into_iter()
            .filter(|identifier| *identifier != excluded_member_identifier)
            .collect(),
    )
}

fn start_sign_round(session_id: &str, key_group: &str) -> RoundState {
    let request = StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: MESSAGE_HEX.to_string(),
        key_group: key_group.to_string(),
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };

    serde_json::from_slice(&call_json!(
        frost_tbtc::frost_tbtc_start_sign_round,
        request
    ))
    .expect("start sign round response")
}

fn bootstrap_synthetic_share_hex(round_state: &RoundState, identifier: u16) -> String {
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

fn setup_timeout_transition_request() -> StartSignRoundRequest {
    ensure_benchmark_environment();

    let coordinators = benchmark_coordinators();
    let session_id = next_session_id("transition-timeout");
    let dkg_result = run_dkg(&session_id);

    let attempt_one_request = StartSignRoundRequest {
        session_id: session_id.clone(),
        member_identifier: 1,
        message_hex: MESSAGE_HEX.to_string(),
        key_group: dkg_result.key_group.clone(),
        signing_participants: None,
        attempt_context: Some(build_attempt_context(
            &session_id,
            1,
            coordinators.attempt_one_all_members,
            vec![1, 2, 3],
        )),
        attempt_transition_evidence: None,
    };
    let attempt_one_fingerprint = sign_request_fingerprint(&attempt_one_request);
    let attempt_one_round_state: RoundState = serde_json::from_slice(&call_json!(
        frost_tbtc::frost_tbtc_start_sign_round,
        attempt_one_request.clone()
    ))
    .expect("attempt one round state");

    let transition_evidence = AttemptTransitionEvidence {
        from_attempt_number: 1,
        from_attempt_id: attempt_one_request
            .attempt_context
            .expect("attempt one context")
            .attempt_id,
        from_coordinator_identifier: coordinators.attempt_one_all_members,
        previous_round_id: attempt_one_round_state.round_id,
        previous_sign_request_fingerprint: attempt_one_fingerprint,
        exclusion_evidence: Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT.to_string(),
            excluded_member_identifiers: vec![],
            invalid_share_proof_fingerprint: None,
        }),
    };

    StartSignRoundRequest {
        session_id: session_id.clone(),
        member_identifier: 1,
        message_hex: MESSAGE_HEX.to_string(),
        key_group: dkg_result.key_group,
        signing_participants: None,
        attempt_context: Some(build_attempt_context(
            &session_id,
            2,
            coordinators.attempt_two_all_members,
            vec![1, 2, 3],
        )),
        attempt_transition_evidence: Some(transition_evidence),
    }
}

fn setup_invalid_share_transition_request() -> StartSignRoundRequest {
    ensure_benchmark_environment();

    let coordinators = benchmark_coordinators();
    let excluded_member_identifier = coordinators.attempt_one_all_members;
    let incoming_included_participants = participants_excluding(excluded_member_identifier);
    let incoming_coordinator_identifier =
        probe_deterministic_coordinator(2, incoming_included_participants.clone());
    let session_id = next_session_id("transition-invalid-share");
    let dkg_result = run_dkg(&session_id);

    let attempt_one_request = StartSignRoundRequest {
        session_id: session_id.clone(),
        member_identifier: 1,
        message_hex: MESSAGE_HEX.to_string(),
        key_group: dkg_result.key_group.clone(),
        signing_participants: None,
        attempt_context: Some(build_attempt_context(
            &session_id,
            1,
            coordinators.attempt_one_all_members,
            vec![1, 2, 3],
        )),
        attempt_transition_evidence: None,
    };
    let attempt_one_fingerprint = sign_request_fingerprint(&attempt_one_request);
    let attempt_one_round_state: RoundState = serde_json::from_slice(&call_json!(
        frost_tbtc::frost_tbtc_start_sign_round,
        attempt_one_request.clone()
    ))
    .expect("attempt one round state");

    let transition_evidence = AttemptTransitionEvidence {
        from_attempt_number: 1,
        from_attempt_id: attempt_one_request
            .attempt_context
            .expect("attempt one context")
            .attempt_id,
        from_coordinator_identifier: coordinators.attempt_one_all_members,
        previous_round_id: attempt_one_round_state.round_id,
        previous_sign_request_fingerprint: attempt_one_fingerprint,
        exclusion_evidence: Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
            excluded_member_identifiers: vec![excluded_member_identifier],
            invalid_share_proof_fingerprint: Some("aa55".to_string()),
        }),
    };

    StartSignRoundRequest {
        session_id: session_id.clone(),
        member_identifier: 1,
        message_hex: MESSAGE_HEX.to_string(),
        key_group: dkg_result.key_group,
        signing_participants: Some(incoming_included_participants.clone()),
        attempt_context: Some(build_attempt_context(
            &session_id,
            2,
            incoming_coordinator_identifier,
            incoming_included_participants,
        )),
        attempt_transition_evidence: Some(transition_evidence),
    }
}

fn setup_stale_attempt_replay_request() -> StartSignRoundRequest {
    ensure_benchmark_environment();

    let coordinators = benchmark_coordinators();
    let session_id = next_session_id("stale-attempt-replay");
    let dkg_result = run_dkg(&session_id);

    let attempt_one_request = StartSignRoundRequest {
        session_id: session_id.clone(),
        member_identifier: 1,
        message_hex: MESSAGE_HEX.to_string(),
        key_group: dkg_result.key_group.clone(),
        signing_participants: None,
        attempt_context: Some(build_attempt_context(
            &session_id,
            1,
            coordinators.attempt_one_all_members,
            vec![1, 2, 3],
        )),
        attempt_transition_evidence: None,
    };
    let attempt_one_fingerprint = sign_request_fingerprint(&attempt_one_request);
    let attempt_one_round_state: RoundState = serde_json::from_slice(&call_json!(
        frost_tbtc::frost_tbtc_start_sign_round,
        attempt_one_request.clone()
    ))
    .expect("attempt one round state");

    let transition_evidence = AttemptTransitionEvidence {
        from_attempt_number: 1,
        from_attempt_id: attempt_one_request
            .attempt_context
            .as_ref()
            .expect("attempt one context")
            .attempt_id
            .clone(),
        from_coordinator_identifier: coordinators.attempt_one_all_members,
        previous_round_id: attempt_one_round_state.round_id,
        previous_sign_request_fingerprint: attempt_one_fingerprint,
        exclusion_evidence: Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT.to_string(),
            excluded_member_identifiers: vec![],
            invalid_share_proof_fingerprint: None,
        }),
    };

    let attempt_two_request = StartSignRoundRequest {
        session_id: session_id.clone(),
        member_identifier: 1,
        message_hex: MESSAGE_HEX.to_string(),
        key_group: dkg_result.key_group,
        signing_participants: None,
        attempt_context: Some(build_attempt_context(
            &session_id,
            2,
            coordinators.attempt_two_all_members,
            vec![1, 2, 3],
        )),
        attempt_transition_evidence: Some(transition_evidence),
    };
    let _: RoundState = serde_json::from_slice(&call_json!(
        frost_tbtc::frost_tbtc_start_sign_round,
        attempt_two_request
    ))
    .expect("attempt two round state");

    attempt_one_request
}

fn benchmark_run_dkg(c: &mut Criterion) {
    ensure_benchmark_environment();

    let mut group = c.benchmark_group("phase5/ffi_run_dkg");
    group.bench_function("happy_path", |b| {
        b.iter(|| {
            let session_id = next_session_id("dkg");
            black_box(run_dkg(&session_id));
        });
    });
    group.finish();
}

fn benchmark_start_sign_round(c: &mut Criterion) {
    ensure_benchmark_environment();

    let mut group = c.benchmark_group("phase5/ffi_start_sign_round");
    group.bench_function("happy_path", |b| {
        b.iter_batched(
            || {
                let session_id = next_session_id("start");
                let dkg_result = run_dkg(&session_id);
                (session_id, dkg_result.key_group)
            },
            |(session_id, key_group)| {
                black_box(start_sign_round(&session_id, &key_group));
            },
            BatchSize::SmallInput,
        );
    });
    group.finish();
}

fn benchmark_finalize_sign_round_bootstrap(c: &mut Criterion) {
    ensure_benchmark_environment();

    let mut group = c.benchmark_group("phase5/ffi_finalize_sign_round");
    group.bench_function("bootstrap_happy_path", |b| {
        b.iter_batched(
            || {
                let session_id = next_session_id("finalize");
                let dkg_result = run_dkg(&session_id);
                let round_state = start_sign_round(&session_id, &dkg_result.key_group);
                let round_contributions = vec![
                    RoundContribution {
                        identifier: 1,
                        signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                    },
                    RoundContribution {
                        identifier: 2,
                        signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                    },
                    RoundContribution {
                        identifier: 3,
                        signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 3),
                    },
                ];

                FinalizeSignRoundRequest {
                    session_id,
                    round_contributions,
                    attempt_context: None,
                }
            },
            |request| {
                let response_bytes =
                    call_json!(frost_tbtc::frost_tbtc_finalize_sign_round, request);
                let finalize_result: SignatureResult =
                    serde_json::from_slice(&response_bytes).expect("finalize response");
                black_box(finalize_result.signature_hex);
            },
            BatchSize::SmallInput,
        );
    });
    group.finish();
}

fn benchmark_start_sign_round_recovery(c: &mut Criterion) {
    ensure_benchmark_environment();
    let _ = benchmark_coordinators();

    let mut group = c.benchmark_group("phase5/ffi_start_sign_round_recovery");
    group.bench_function("timeout_transition_authorized", |b| {
        b.iter_batched(
            setup_timeout_transition_request,
            |request| {
                let response_bytes = call_json!(frost_tbtc::frost_tbtc_start_sign_round, request);
                let round_state: RoundState =
                    serde_json::from_slice(&response_bytes).expect("timeout transition response");
                let telemetry = round_state
                    .attempt_transition_telemetry
                    .expect("timeout transition telemetry");
                assert_eq!(telemetry.reason, ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT);
                assert!(telemetry.excluded_member_identifiers.is_empty());
                black_box(round_state.round_id);
            },
            BatchSize::SmallInput,
        );
    });

    group.bench_function("invalid_share_proof_transition_with_rotation", |b| {
        b.iter_batched(
            setup_invalid_share_transition_request,
            |request| {
                let expected_excluded_members = request
                    .attempt_transition_evidence
                    .as_ref()
                    .expect("transition evidence")
                    .exclusion_evidence
                    .as_ref()
                    .expect("exclusion evidence")
                    .excluded_member_identifiers
                    .clone();
                let response_bytes = call_json!(frost_tbtc::frost_tbtc_start_sign_round, request);
                let round_state: RoundState = serde_json::from_slice(&response_bytes)
                    .expect("invalid-share transition response");
                let telemetry = round_state
                    .attempt_transition_telemetry
                    .expect("invalid-share transition telemetry");
                assert_eq!(telemetry.reason, ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF);
                assert_eq!(
                    telemetry.excluded_member_identifiers,
                    expected_excluded_members
                );
                assert!(telemetry.coordinator_rotated);
                black_box(round_state.round_id);
            },
            BatchSize::SmallInput,
        );
    });

    group.finish();
}

fn benchmark_start_sign_round_replay_guard(c: &mut Criterion) {
    ensure_benchmark_environment();
    let _ = benchmark_coordinators();

    let mut group = c.benchmark_group("phase5/ffi_start_sign_round_replay_guard");
    group.bench_function("stale_attempt_rejected_after_transition", |b| {
        b.iter_batched(
            setup_stale_attempt_replay_request,
            |request| {
                let (status_code, response_bytes) =
                    call_raw!(frost_tbtc::frost_tbtc_start_sign_round, request);
                assert_eq!(status_code, 1);
                let error: ErrorResponse =
                    serde_json::from_slice(&response_bytes).expect("error response");
                assert_eq!(error.code, "validation_error");
                assert!(error.message.contains("stale"));
                black_box(error.message);
            },
            BatchSize::SmallInput,
        );
    });
    group.finish();
}

fn benchmark_start_sign_round_restart_paths(c: &mut Criterion) {
    ensure_benchmark_environment();
    let _ = benchmark_coordinators();

    let mut group = c.benchmark_group("phase5/ffi_start_sign_round_restart_paths");
    group.bench_function("authorized_transition_after_reload", |b| {
        b.iter_batched(
            setup_timeout_transition_request,
            |request| {
                frost_tbtc::frost_tbtc_reload_state_from_storage_for_benchmarks()
                    .expect("reload signer state from storage");
                let response_bytes = call_json!(frost_tbtc::frost_tbtc_start_sign_round, request);
                let round_state: RoundState = serde_json::from_slice(&response_bytes)
                    .expect("authorized transition response after reload");
                let telemetry = round_state
                    .attempt_transition_telemetry
                    .expect("authorized transition telemetry after reload");
                assert_eq!(telemetry.reason, ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT);
                assert!(telemetry.excluded_member_identifiers.is_empty());
                black_box(round_state.round_id);
            },
            BatchSize::SmallInput,
        );
    });

    group.bench_function("stale_attempt_rejected_after_reload", |b| {
        b.iter_batched(
            setup_stale_attempt_replay_request,
            |request| {
                frost_tbtc::frost_tbtc_reload_state_from_storage_for_benchmarks()
                    .expect("reload signer state from storage");
                let (status_code, response_bytes) =
                    call_raw!(frost_tbtc::frost_tbtc_start_sign_round, request);
                assert_eq!(status_code, 1);
                let error: ErrorResponse =
                    serde_json::from_slice(&response_bytes).expect("error response");
                assert_eq!(error.code, "validation_error");
                assert!(error.message.contains("stale"));
                black_box(error.message);
            },
            BatchSize::SmallInput,
        );
    });
    group.finish();
}

criterion_group!(
    phase5_benches,
    benchmark_run_dkg,
    benchmark_start_sign_round,
    benchmark_finalize_sign_round_bootstrap,
    benchmark_start_sign_round_recovery,
    benchmark_start_sign_round_replay_guard,
    benchmark_start_sign_round_restart_paths
);
criterion_main!(phase5_benches);
