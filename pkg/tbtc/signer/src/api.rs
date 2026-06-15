use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgParticipant {
    pub identifier: u16,
    pub public_key_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RunDkgRequest {
    pub session_id: String,
    pub participants: Vec<DkgParticipant>,
    pub threshold: u16,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub dkg_seed_hex: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgResult {
    pub session_id: String,
    pub key_group: String,
    pub participant_count: u16,
    pub threshold: u16,
    pub created_at_unix: u64,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgRound1Package {
    pub identifier: String,
    pub package_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgRound2Package {
    pub identifier: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sender_identifier: Option<String>,
    pub package_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart1Request {
    pub participant_identifier: String,
    pub max_signers: u16,
    pub min_signers: u16,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart1Result {
    pub secret_package_hex: String,
    pub package: DkgRound1Package,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart2Request {
    pub secret_package_hex: String,
    pub round1_packages: Vec<DkgRound1Package>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart2Result {
    pub secret_package_hex: String,
    pub packages: Vec<DkgRound2Package>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct NativeFrostKeyPackage {
    pub identifier: String,
    pub data_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct NativeFrostPublicKeyPackage {
    pub verifying_shares: std::collections::BTreeMap<String, String>,
    pub verifying_key: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart3Request {
    pub secret_package_hex: String,
    pub round1_packages: Vec<DkgRound1Package>,
    pub round2_packages: Vec<DkgRound2Package>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart3Result {
    pub key_package: NativeFrostKeyPackage,
    pub public_key_package: NativeFrostPublicKeyPackage,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct NativeFrostCommitment {
    pub identifier: String,
    pub data_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct NativeFrostSignatureShare {
    pub identifier: String,
    pub data_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct GenerateNoncesAndCommitmentsRequest {
    pub key_package_identifier: String,
    pub key_package_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct GenerateNoncesAndCommitmentsResult {
    /// Secret one-time FROST signing nonces serialized as hex.
    ///
    /// The caller owns this secret after it crosses the FFI boundary. It must
    /// be supplied to `SignShareRequest::nonces_hex` at most once and erased by
    /// the caller immediately afterward. Reuse for another signing package or
    /// message can reveal the private signing share.
    pub nonces_hex: String,
    pub commitment: NativeFrostCommitment,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct NewSigningPackageRequest {
    pub message_hex: String,
    pub commitments: Vec<NativeFrostCommitment>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct NewSigningPackageResult {
    pub signing_package_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct SignShareRequest {
    pub signing_package_hex: String,
    /// Secret one-time nonces returned by `GenerateNoncesAndCommitmentsResult`.
    ///
    /// This stateless endpoint cannot remember consumed nonces across FFI
    /// calls. The caller is cryptographically responsible for single use.
    pub nonces_hex: String,
    pub key_package_identifier: String,
    pub key_package_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct SignShareResult {
    pub signature_share: NativeFrostSignatureShare,
}

// Phase 7.1 hardened interactive signing session (frozen spec
// docs/phase-7-interactive-session-spec-freeze.md, section 5). Unlike
// the stateless primitives above, secret nonces NEVER appear in these
// requests or results: the engine generates, holds, consumes, and
// zeroizes them internally, keyed by (session_id, attempt_id).

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveSessionOpenRequest {
    pub session_id: String,
    pub member_identifier: u16,
    pub message_hex: String,
    pub key_group: String,
    /// Signing threshold; must equal the session's DKG threshold. The
    /// key material itself is resolved from the engine's DKG state and
    /// is never carried in this request - no signing secret crosses the
    /// FFI (frozen spec section 4).
    pub threshold: u16,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub taproot_merkle_root_hex: Option<String>,
    /// Required: interactive sessions are strict-mode only; there is
    /// no legacy-shape fallback on this path.
    pub attempt_context: AttemptContext,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveSessionOpenResult {
    pub session_id: String,
    pub attempt_id: String,
    pub idempotent: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveRound1Request {
    pub session_id: String,
    pub attempt_id: String,
    pub member_identifier: u16,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveRound1Result {
    /// The member's public signing commitments. Idempotent until the
    /// attempt's nonces are consumed; the secret nonces they
    /// correspond to never leave the engine.
    pub commitments_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveRound2Request {
    pub session_id: String,
    pub attempt_id: String,
    pub member_identifier: u16,
    /// The coordinator's signing package (the chosen responsive
    /// subset's commitment list). Verified in full - membership,
    /// subset-of-included, exact threshold size, message binding, and
    /// byte-identity of this member's own commitment entry - BEFORE
    /// the nonces are consumed.
    pub signing_package_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveRound2Result {
    pub session_id: String,
    pub attempt_id: String,
    pub signature_share_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveAggregateRequest {
    pub session_id: String,
    pub attempt_id: String,
    /// The signing package the shares were produced over (carries the
    /// message and the chosen subset's commitments).
    pub signing_package_hex: String,
    /// The collected signature shares from the responsive subset. Each is
    /// verified against the member's verifying share (resolved from the
    /// session's DKG public key package) before aggregation. If any share fails,
    /// the call fails closed with no signature and the
    /// `aggregate_share_verification_failed` error, which carries the CANDIDATE
    /// culprits - every member whose share failed (Phase 7.2b-3). These are
    /// pure-crypto candidates for the Go host's envelope-bound blame
    /// adjudication (frozen Phase 7.2b spec, section 6); the engine never
    /// inspects operator-signed envelopes itself.
    pub signature_shares: Vec<NativeFrostSignatureShare>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub taproot_merkle_root_hex: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveAggregateResult {
    pub session_id: String,
    pub attempt_id: String,
    /// The aggregated BIP-340 Schnorr signature, hex-encoded.
    pub signature_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveSessionAbortRequest {
    pub session_id: String,
    /// When set, abort only if the live attempt matches; when unset,
    /// abort whatever attempt is live for the session.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attempt_id: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InteractiveSessionAbortResult {
    pub session_id: String,
    pub aborted: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct AggregateRequest {
    pub signing_package_hex: String,
    pub signature_shares: Vec<NativeFrostSignatureShare>,
    pub public_key_package: NativeFrostPublicKeyPackage,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct AggregateResult {
    pub signature_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct StartSignRoundRequest {
    pub session_id: String,
    pub member_identifier: u16,
    pub message_hex: String,
    pub key_group: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub taproot_merkle_root_hex: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub signing_participants: Option<Vec<u16>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attempt_context: Option<AttemptContext>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attempt_transition_evidence: Option<AttemptTransitionEvidence>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RoundContribution {
    pub identifier: u16,
    pub signature_share_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct AttemptTransitionTelemetry {
    pub from_attempt_number: u32,
    pub to_attempt_number: u32,
    pub from_coordinator_identifier: u16,
    pub to_coordinator_identifier: u16,
    pub reason: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub excluded_member_identifiers: Vec<u16>,
    pub coordinator_rotated: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RoundState {
    pub session_id: String,
    pub round_id: String,
    pub required_contributions: u16,
    pub message_digest_hex: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub taproot_merkle_root_hex: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub signing_participants: Option<Vec<u16>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attempt_transition_telemetry: Option<AttemptTransitionTelemetry>,
    pub own_contribution: RoundContribution,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct FinalizeSignRoundRequest {
    pub session_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub taproot_merkle_root_hex: Option<String>,
    pub round_contributions: Vec<RoundContribution>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attempt_context: Option<AttemptContext>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct AttemptContext {
    pub attempt_number: u32,
    pub coordinator_identifier: u16,
    pub included_participants: Vec<u16>,
    pub included_participants_fingerprint: String,
    pub attempt_id: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct AttemptExclusionEvidence {
    pub reason: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub excluded_member_identifiers: Vec<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub invalid_share_proof_fingerprint: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct AttemptTransitionEvidence {
    pub from_attempt_number: u32,
    pub from_attempt_id: String,
    pub from_coordinator_identifier: u16,
    pub previous_round_id: String,
    pub previous_sign_request_fingerprint: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub exclusion_evidence: Option<AttemptExclusionEvidence>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct TranscriptAuditRequest {
    pub session_id: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct TranscriptAuditRecord {
    pub from_attempt_number: u32,
    pub to_attempt_number: u32,
    pub from_attempt_id: String,
    pub to_attempt_id: String,
    pub previous_round_id: String,
    pub previous_sign_request_fingerprint: String,
    pub from_coordinator_identifier: u16,
    pub to_coordinator_identifier: u16,
    pub reason: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub excluded_member_identifiers: Vec<u16>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub invalid_share_proof_fingerprint: Option<String>,
    pub transcript_hash: String,
    pub recorded_at_unix: u64,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct TranscriptAuditResult {
    pub session_id: String,
    pub transition_count: u64,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub records: Vec<TranscriptAuditRecord>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct VerifyBlameProofRequest {
    pub session_id: String,
    pub from_attempt_number: u32,
    pub accused_member_identifier: u16,
    pub reason: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub invalid_share_proof_fingerprint: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct BlameProofVerificationResult {
    pub session_id: String,
    pub from_attempt_number: u32,
    pub accused_member_identifier: u16,
    pub reason: String,
    pub verified: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub transcript_hash: Option<String>,
    pub detail: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct QuarantineStatusRequest {
    pub operator_identifier: u16,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct QuarantineStatusResult {
    pub operator_identifier: u16,
    pub auto_quarantine_enabled: bool,
    pub fault_score: u64,
    pub quarantine_threshold: u64,
    pub quarantined: bool,
    pub dao_override_allowlisted: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct SignatureResult {
    pub session_id: String,
    pub round_id: String,
    pub signature_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct TxInput {
    pub txid_hex: String,
    pub vout: u32,
    pub value_sats: u64,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct TxOutput {
    pub script_pubkey_hex: String,
    pub value_sats: u64,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct BuildTaprootTxRequest {
    pub session_id: String,
    pub inputs: Vec<TxInput>,
    pub outputs: Vec<TxOutput>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub script_tree_hex: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct TransactionResult {
    pub session_id: String,
    pub tx_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct ShareMaterial {
    pub identifier: u16,
    pub encrypted_share_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RefreshSharesRequest {
    pub session_id: String,
    pub current_shares: Vec<ShareMaterial>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RefreshSharesResult {
    pub session_id: String,
    pub refresh_epoch: u64,
    pub new_shares: Vec<ShareMaterial>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RefreshCadenceStatusRequest {
    pub session_id: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RefreshCadenceStatusResult {
    pub session_id: String,
    pub refresh_count: u64,
    pub last_refresh_epoch: u64,
    pub cadence_seconds: u64,
    pub next_refresh_due_unix: u64,
    pub overdue: bool,
    pub continuity_preserved: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub continuity_reference_key_group: Option<String>,
    pub emergency_rekey_required: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub emergency_rekey_reason: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct TriggerEmergencyRekeyRequest {
    pub session_id: String,
    pub reason: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct TriggerEmergencyRekeyResult {
    pub session_id: String,
    pub emergency_rekey_required: bool,
    pub reason: String,
    pub triggered_at_unix: u64,
    pub recommended_new_session_id: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DifferentialFuzzRequest {
    #[serde(default)]
    pub seed: u64,
    #[serde(default)]
    pub case_count: u32,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DifferentialDivergence {
    pub case_index: u32,
    pub check: String,
    pub severity: String,
    pub detail: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DifferentialFuzzResult {
    pub seed: u64,
    pub case_count: u32,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub divergences: Vec<DifferentialDivergence>,
    pub critical_divergence_count: u32,
    pub unresolved_critical_divergence: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct PromoteCanaryRequest {
    pub target_percent: u8,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct PromoteCanaryResult {
    pub from_percent: u8,
    pub to_percent: u8,
    pub config_version: u64,
    pub promoted_at_unix: u64,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RollbackCanaryRequest {
    pub reason: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RollbackCanaryResult {
    pub from_percent: u8,
    pub to_percent: u8,
    pub config_version: u64,
    pub reason: String,
    pub rolled_back_at_unix: u64,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct CanaryRolloutStatusResult {
    pub current_percent: u8,
    pub previous_percent: u8,
    pub config_version: u64,
    pub promotion_gate_passed: bool,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub gate_failures: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub recommended_next_percent: Option<u8>,
    pub last_action_unix: u64,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct RoastLivenessPolicyResult {
    pub coordinator_timeout_ms: u64,
    pub timeout_source: String,
    pub advance_trigger: String,
    pub exclusion_evidence_policy: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct SignerHardeningMetricsResult {
    pub runtime_version: String,
    pub provenance_enforced: bool,
    pub admission_policy_enforced: bool,
    pub signing_policy_firewall_enforced: bool,
    pub run_dkg_calls_total: u64,
    pub run_dkg_success_total: u64,
    pub run_dkg_admission_reject_total: u64,
    pub start_sign_round_calls_total: u64,
    pub start_sign_round_success_total: u64,
    pub build_taproot_tx_calls_total: u64,
    pub build_taproot_tx_success_total: u64,
    pub build_taproot_tx_policy_reject_total: u64,
    pub finalize_sign_round_calls_total: u64,
    pub finalize_sign_round_success_total: u64,
    pub refresh_shares_calls_total: u64,
    pub refresh_shares_success_total: u64,
    pub roast_transcript_audit_calls_total: u64,
    pub roast_transcript_audit_success_total: u64,
    pub verify_blame_proof_calls_total: u64,
    pub verify_blame_proof_success_total: u64,
    pub attempt_transition_total: u64,
    pub coordinator_failover_total: u64,
    pub auto_quarantine_fault_events_total: u64,
    pub auto_quarantine_enforcements_total: u64,
    pub quarantined_operator_count: u64,
    pub refresh_cadence_overdue_sessions: u64,
    pub emergency_rekey_sessions_total: u64,
    pub differential_fuzz_runs_total: u64,
    pub differential_fuzz_critical_divergence_total: u64,
    pub canary_promotions_total: u64,
    pub canary_rollbacks_total: u64,
    pub run_dkg_latency_p95_ms: u64,
    pub run_dkg_latency_samples: u64,
    pub start_sign_round_latency_p95_ms: u64,
    pub start_sign_round_latency_samples: u64,
    pub build_taproot_tx_latency_p95_ms: u64,
    pub build_taproot_tx_latency_samples: u64,
    pub finalize_sign_round_latency_p95_ms: u64,
    pub finalize_sign_round_latency_samples: u64,
    pub refresh_shares_latency_p95_ms: u64,
    pub refresh_shares_latency_samples: u64,
    #[serde(default)]
    pub interactive_session_open_calls_total: u64,
    #[serde(default)]
    pub interactive_session_open_success_total: u64,
    #[serde(default)]
    pub interactive_round1_calls_total: u64,
    #[serde(default)]
    pub interactive_round1_success_total: u64,
    #[serde(default)]
    pub interactive_round2_calls_total: u64,
    #[serde(default)]
    pub interactive_round2_success_total: u64,
    #[serde(default)]
    pub interactive_session_abort_calls_total: u64,
    #[serde(default)]
    pub interactive_session_abort_success_total: u64,
    #[serde(default)]
    pub interactive_aggregate_calls_total: u64,
    #[serde(default)]
    pub interactive_aggregate_success_total: u64,
    #[serde(default)]
    pub interactive_round1_latency_p95_ms: u64,
    #[serde(default)]
    pub interactive_round1_latency_samples: u64,
    #[serde(default)]
    pub interactive_round2_latency_p95_ms: u64,
    #[serde(default)]
    pub interactive_round2_latency_samples: u64,
    #[serde(default)]
    pub interactive_aggregate_latency_p95_ms: u64,
    #[serde(default)]
    pub interactive_aggregate_latency_samples: u64,
    pub last_updated_unix: u64,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct ErrorResponse {
    pub code: String,
    pub message: String,
    pub recovery_class: String,
    /// CANDIDATE culprits for an `aggregate_share_verification_failed` error:
    /// the u16 Go member identifiers whose FROST signature shares failed
    /// verification (the same identifier space as `excluded_member_identifiers`).
    /// Empty - and omitted from the JSON via skip_serializing_if - for every
    /// other error, so existing Go clients are unaffected. These are pure-crypto
    /// candidates, not adjudicated blame; the Go host performs the envelope-bound
    /// adjudication (frozen Phase 7.2b spec, section 6).
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub candidate_culprits: Vec<u16>,
}

/// Init-time signer configuration installed once by the host over FFI.
///
/// Every field mirrors one `TBTC_SIGNER_*` environment variable (field name =
/// lowercased variable suffix). Once a config is installed the process
/// environment is no longer consulted for any covered knob: unset fields mean
/// the built-in default, not the environment value. The state-encryption key
/// (`TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX`) is deliberately absent — secrets
/// stay on the dedicated env/command key-provider channel and never ride the
/// config FFI. Unknown fields are rejected so a typo'd knob fails the init
/// instead of silently running on a default.
#[derive(Clone, Debug, Default, Deserialize, PartialEq, Eq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct InitSignerConfigRequest {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub profile: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_bootstrap: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enable_roast_strict: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_bench_restart_hook: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub roast_coordinator_timeout_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub refresh_cadence_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub state_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub state_corruption_policy: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub state_corrupt_backup_limit: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_sessions: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_live_interactive_sessions: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub interactive_session_ttl_seconds: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub state_key_provider: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub state_key_command: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub state_key_command_timeout_secs: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enforce_provenance_gate: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provenance_attestation_status: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provenance_attestation_payload: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provenance_attestation_signature_hex: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provenance_trust_root: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_approved_version: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enforce_admission_policy: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub admission_min_participants: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub admission_min_threshold: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub admission_required_identifiers: Option<Vec<u16>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub admission_allowlist_identifiers: Option<Vec<u16>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enforce_signing_policy_firewall: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub policy_allowed_script_classes: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub policy_max_output_count: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub policy_max_output_value_sats: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub policy_max_total_output_value_sats: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub policy_allowed_utc_start_hour: Option<u8>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub policy_allowed_utc_end_hour: Option<u8>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub policy_rate_limit_per_minute: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enable_auto_quarantine: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auto_quarantine_fault_threshold: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auto_quarantine_timeout_penalty: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auto_quarantine_invalid_share_penalty: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auto_quarantine_dao_allowlist_identifiers: Option<Vec<u16>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canary_max_start_sign_round_p95_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canary_max_finalize_sign_round_p95_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canary_max_policy_reject_rate_bps: Option<u64>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InitSignerConfigResult {
    pub installed: bool,
    pub idempotent: bool,
    pub config_fingerprint: String,
    pub configured_key_count: u32,
}
