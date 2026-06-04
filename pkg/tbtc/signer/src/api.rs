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
    pub nonces_hex: String,
    pub key_package_identifier: String,
    pub key_package_hex: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct SignShareResult {
    pub signature_share: NativeFrostSignatureShare,
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
    pub signing_participants: Option<Vec<u16>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub attempt_transition_telemetry: Option<AttemptTransitionTelemetry>,
    pub own_contribution: RoundContribution,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct FinalizeSignRoundRequest {
    pub session_id: String,
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
    pub last_updated_unix: u64,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct ErrorResponse {
    pub code: String,
    pub message: String,
    pub recovery_class: String,
}
