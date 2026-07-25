use std::fmt;

use serde::{Deserialize, Serialize};
use zeroize::{Zeroize, ZeroizeOnDrop, Zeroizing};

/// A hex-encoded secret whose owned Rust allocation is wiped on drop and whose
/// `Debug` representation never exposes its contents. Serde remains transparent
/// so the C-ABI JSON contract continues to carry an ordinary string.
#[derive(Clone, Default, Deserialize, Eq, PartialEq, Serialize)]
#[serde(transparent)]
pub struct SecretHex(Zeroizing<String>);

impl SecretHex {
    pub fn new(value: String) -> Self {
        Self(Zeroizing::new(value))
    }

    /// Borrows the secret for the narrow decode/serialization boundary without
    /// creating another unmanaged `String` allocation.
    pub fn expose_secret(&self) -> &str {
        self.0.as_str()
    }
}

impl From<String> for SecretHex {
    fn from(value: String) -> Self {
        Self::new(value)
    }
}

impl fmt::Debug for SecretHex {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str("<redacted>")
    }
}

impl Zeroize for SecretHex {
    fn zeroize(&mut self) {
        self.0.zeroize();
    }
}

impl ZeroizeOnDrop for SecretHex {}

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
    pub package_hex: SecretHex,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart1Request {
    pub participant_identifier: String,
    pub max_signers: u16,
    pub min_signers: u16,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart1Result {
    pub secret_package_hex: SecretHex,
    pub package: DkgRound1Package,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart2Request {
    pub secret_package_hex: SecretHex,
    pub round1_packages: Vec<DkgRound1Package>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart2Result {
    pub secret_package_hex: SecretHex,
    pub packages: Vec<DkgRound2Package>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct NativeFrostKeyPackage {
    pub identifier: String,
    pub data_hex: SecretHex,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct NativeFrostPublicKeyPackage {
    pub verifying_shares: std::collections::BTreeMap<String, String>,
    pub verifying_key: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart3Request {
    pub secret_package_hex: SecretHex,
    pub round1_packages: Vec<DkgRound1Package>,
    pub round2_packages: Vec<DkgRound2Package>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DkgPart3Result {
    pub key_package: NativeFrostKeyPackage,
    pub public_key_package: NativeFrostPublicKeyPackage,
}

/// Persists the result of a DISTRIBUTED FROST DKG for one seat: this node's own
/// key package (from Part3) plus the group public key package, into the engine
/// session state that the interactive signing path loads. Unlike the dealer
/// `run_dkg`, a distributed node holds only its OWN secret key package.
#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct PersistDistributedDkgKeyPackageRequest {
    pub session_id: String,
    pub participant_identifier: u16,
    pub threshold: u16,
    pub participant_count: u16,
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
pub struct NewSigningPackageRequest {
    pub message_hex: String,
    pub commitments: Vec<NativeFrostCommitment>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct NewSigningPackageResult {
    pub signing_package_hex: String,
}

// Phase 7.1 hardened interactive signing session (frozen spec
// docs/phase-7-interactive-session-spec-freeze.md, section 5). Secret
// nonces NEVER appear in these requests or results: the engine
// generates, holds, consumes, and zeroizes them internally, keyed by
// (session_id, attempt_id).

/// Narrow non-transaction signing intents accepted by the production signing
/// policy firewall. This enum is internally tagged so the wire shape is
/// explicit and future variants cannot be confused with an arbitrary message
/// allowlist.
#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum InteractiveSigningIntent {
    /// A tBTC wallet heartbeat message. `message_hex` is the raw 16-byte
    /// heartbeat preimage, not the 32-byte digest being signed.
    Heartbeat { message_hex: String },
}

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
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub signing_intent: Option<InteractiveSigningIntent>,
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

/// The verdict of a single-share verification (VerifySignatureShare). It is a
/// deliberate THREE-way value, not pass/fail: the boundary between a
/// member-attributable failure (blame) and a not-the-member's-fault failure
/// (don't blame) is security-critical, and the engine - the only layer that can
/// tell "the member's signed scalar is malformed" from "the coordinator's
/// package/context is malformed" - decides it here so the Go host never has to.
#[derive(Clone, Copy, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum ShareVerificationVerdict {
    /// The share is a valid FROST signature share under the (tweaked) package.
    Valid,
    /// MEMBER-attributable: the share is mathematically invalid, OR the member's
    /// own operator-signed share bytes are undecodable (self-incriminating).
    /// NOTE: like InteractiveAggregate's candidate-culprit list, this verdict is
    /// framable by a coordinator that verifies an honest share against a
    /// mismatched package/root, so it is an INPUT to the Go host's f+1
    /// envelope-bound adjudication (Phase 7.2b spec, section 6), NOT authoritative
    /// blame on its own.
    Invalid,
    /// Not the member's fault: undecodable signing package (coordinator input),
    /// missing/unknown verifying share, session not ready, ambiguous context.
    /// Fail closed against blame.
    Indeterminate,
}

/// Request to verify ONE retained round-2 signature share against an attempt's
/// authoritative signing package, using FROST share verification. The verifying
/// material is resolved from the session's own DKG state (never the request),
/// and the taproot root is canonicalized + applied exactly as InteractiveAggregate
/// does, so the verdict matches what aggregation would conclude for that share.
#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct VerifySignatureShareRequest {
    pub session_id: String,
    pub signing_package_hex: String,
    pub signature_share_hex: String,
    pub member_identifier: u16,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub taproot_merkle_root_hex: Option<String>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct VerifySignatureShareResult {
    pub verdict: ShareVerificationVerdict,
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
pub struct AttemptContext {
    pub attempt_number: u32,
    pub coordinator_identifier: u16,
    pub included_participants: Vec<u16>,
    pub included_participants_fingerprint: String,
    pub attempt_id: String,
}

/// Request to derive the canonical interactive attempt context (plus the
/// per-participant FROST identifiers) from an attempt's public inputs, so the
/// host never re-implements the engine's domain-separated derivations - the
/// cross-language divergence class. Stateless and secret-free: no DKG lookup,
/// no nonce/session state, no policy decision.
#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DeriveInteractiveAttemptContextRequest {
    pub session_id: String,
    pub message_hex: String,
    pub key_group: String,
    /// Validation gate only - the derivation requires `included_participants`
    /// to hold at least `threshold` members; it is NOT an input to the
    /// fingerprint, attempt-id, or coordinator derivation.
    pub threshold: u16,
    /// 1-based wire attempt number (the host's 0-based value + 1), matching
    /// `AttemptContext.attempt_number`.
    pub attempt_number: u32,
    pub included_participants: Vec<u16>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DeriveInteractiveAttemptContextResult {
    /// The canonical attempt context, re-validated against strict-mode
    /// `validate_attempt_context` before returning, so the host can pass it
    /// verbatim to `InteractiveSessionOpenRequest.attempt_context` and the
    /// engine will accept it.
    pub attempt_context: AttemptContext,
    /// One FROST identifier string per included participant, in canonical
    /// (ascending) participant order - the exact key-package encoding the
    /// signing-package and aggregate paths expect.
    pub frost_identifiers: Vec<ParticipantFrostIdentifier>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct ParticipantFrostIdentifier {
    pub participant_identifier: u16,
    pub frost_identifier: String,
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
    /// Script pubkey of the output being spent. BIP-341 SIGHASH_DEFAULT commits
    /// to every input's ordered prevout amount and script pubkey, so the signer
    /// cannot derive the messages it is allowed to sign without this metadata.
    pub script_pubkey_hex: String,
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
    /// One BIP-341 key-spend SIGHASH_DEFAULT message per transaction input, in
    /// input order. `serde(default)` lets pre-ABI-3 persisted state decode so the
    /// policy gate can reject its empty legacy artifact explicitly and fail closed.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub taproot_key_spend_sighashes_hex: Vec<String>,
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

/// Reserved response shape for a future cryptographic share-refresh protocol.
/// The current one-shot endpoint always rejects because it cannot safely run
/// the required multi-round FROST refresh, returning the terminal error code
/// `cryptographic_refresh_not_supported`.
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
    /// Number of cryptographically valid refreshes. Always zero until the
    /// versioned multi-round protocol is implemented.
    pub refresh_count: u64,
    /// Epoch of the last cryptographically valid refresh. Always zero while
    /// `RefreshShares` is reserved and fail-closed.
    pub last_refresh_epoch: u64,
    pub cadence_seconds: u64,
    /// Durable DKG creation deadline, or zero when untrusted legacy refresh
    /// metadata has no DKG anchor and must be treated as immediately overdue.
    pub next_refresh_due_unix: u64,
    pub overdue: bool,
    /// False when persisted metadata from the retired synthetic refresh stub is
    /// detected; that metadata never establishes key continuity.
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

/// The FFI CONTRACT version reported by `frost_tbtc_abi_version`, so a Go bridge can
/// fail closed against an incompatible `libfrost_tbtc` rather than silently
/// misinterpreting a changed contract.
///
/// `abi_major` covers any INCOMPATIBLE change to the Go<->Rust contract: C signatures,
/// JSON field meaning, required fields, enum/status values, serialization, memory
/// ownership, or crypto transcript/domain semantics the bridge relies on. `abi_minor`
/// covers a cumulative ADDITIVE, backward-compatible change - a new symbol or a new
/// optional field that old bridges safely ignore. A consumer requires
/// `lib.abi_major == its_major` AND `lib.abi_minor >= the_minor_it_needs`.
#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct FrostTbtcAbiVersionResult {
    pub abi_major: u32,
    pub abi_minor: u32,
}

/// Runtime identity of the exact durable session store the signer opened and
/// locked. The affirmative safety claims are mandatory on the Go side; this
/// response is emitted only after descriptor/path revalidation succeeds.
/// This stable identity does not attest state freshness or key inventory.
#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct DurableStoreIdentityResult {
    pub schema: String,
    pub backend: String,
    pub store_id: String,
    pub canonical_path_fingerprint: String,
    pub filesystem_fingerprint: String,
    pub lock_fingerprint: String,
    pub fingerprint: String,
    pub durable: bool,
    pub exclusive_lock_held: bool,
    pub symlink_free: bool,
    pub replacement_protected: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RetainedKeyPackageInventoryPackage {
    pub participant_seat: u16,
    pub key_package_commitment: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RetainedKeyPackageInventoryEntry {
    pub wallet_id: String,
    pub key_group: String,
    pub threshold: u16,
    pub participant_count: u16,
    pub share_epoch: u64,
    pub public_key_package_commitment: String,
    pub key_packages: Vec<RetainedKeyPackageInventoryPackage>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RetainedKeyPackageInventoryResult {
    pub schema: String,
    pub store_fingerprint: String,
    pub state_generation: u64,
    pub state_commitment: String,
    pub previous_state_commitment: String,
    pub state_image_digest: String,
    pub inventory_commitment: String,
    pub entries: Vec<RetainedKeyPackageInventoryEntry>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
pub struct StateWitnessProofRequest {
    pub schema: String,
    pub store_fingerprint: String,
    pub ancestor_generation: u64,
    pub ancestor_commitment: String,
    pub target_generation: u64,
    pub target_commitment: String,
    pub maximum_entries: u16,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct StateWitnessProofEntry {
    pub generation: u64,
    pub previous_state_commitment: String,
    pub state_commitment: String,
    pub state_image_digest: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct StateWitnessProofResult {
    pub schema: String,
    pub store_fingerprint: String,
    pub ancestor_generation: u64,
    pub ancestor_commitment: String,
    pub target_generation: u64,
    pub target_commitment: String,
    pub complete: bool,
    pub entries: Vec<StateWitnessProofEntry>,
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
    #[serde(default)]
    pub heartbeat_signing_policy_reject_total: u64,
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
    pub state_witness_max_records: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub permit_plaintext_state_rollback: Option<bool>,
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
    pub policy_heartbeat_rate_limit_per_minute: Option<u64>,
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
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canary_max_interactive_round1_p95_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canary_max_interactive_round2_p95_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canary_max_interactive_aggregate_p95_ms: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canary_min_samples: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canary_min_policy_samples: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canary_max_sample_age_seconds: Option<u64>,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
pub struct InitSignerConfigResult {
    pub installed: bool,
    pub idempotent: bool,
    pub config_fingerprint: String,
    pub configured_key_count: u32,
}

#[cfg(test)]
mod tests {
    use super::*;

    const SECRET_SENTINEL: &str =
        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef";

    fn secret_hex() -> SecretHex {
        SecretHex::new(SECRET_SENTINEL.to_string())
    }

    fn round1_package() -> DkgRound1Package {
        DkgRound1Package {
            identifier: "round1-identifier".to_string(),
            package_hex: "public-round1-package".to_string(),
        }
    }

    fn round2_package() -> DkgRound2Package {
        DkgRound2Package {
            identifier: "round2-recipient".to_string(),
            sender_identifier: Some("round2-sender".to_string()),
            package_hex: secret_hex(),
        }
    }

    fn public_key_package() -> NativeFrostPublicKeyPackage {
        NativeFrostPublicKeyPackage {
            verifying_shares: std::collections::BTreeMap::new(),
            verifying_key: "public-verifying-key".to_string(),
        }
    }

    fn key_package() -> NativeFrostKeyPackage {
        NativeFrostKeyPackage {
            identifier: "key-package-identifier".to_string(),
            data_hex: secret_hex(),
        }
    }

    #[test]
    fn dkg_secret_hex_zeroizes_and_is_zeroize_on_drop() {
        fn assert_zeroize_on_drop<T: ZeroizeOnDrop>() {}

        assert_zeroize_on_drop::<SecretHex>();

        let mut secret = secret_hex();
        secret.zeroize();
        assert!(secret.expose_secret().is_empty());
    }

    #[test]
    fn dkg_secret_fields_preserve_json_string_wire_shape() {
        let encoded_holder =
            serde_json::to_string(&secret_hex()).expect("secret holder serializes");
        assert_eq!(encoded_holder, format!("\"{SECRET_SENTINEL}\""));
        let decoded_holder: SecretHex =
            serde_json::from_str(&encoded_holder).expect("secret holder deserializes");
        assert_eq!(decoded_holder.expose_secret(), SECRET_SENTINEL);

        let serialized_fields = [
            serde_json::to_value(DkgRound2Package {
                identifier: "recipient".to_string(),
                sender_identifier: Some("sender".to_string()),
                package_hex: secret_hex(),
            })
            .expect("round2 package serializes")["package_hex"]
                .clone(),
            serde_json::to_value(DkgPart1Result {
                secret_package_hex: secret_hex(),
                package: round1_package(),
            })
            .expect("part1 result serializes")["secret_package_hex"]
                .clone(),
            serde_json::to_value(DkgPart2Request {
                secret_package_hex: secret_hex(),
                round1_packages: vec![round1_package()],
            })
            .expect("part2 request serializes")["secret_package_hex"]
                .clone(),
            serde_json::to_value(DkgPart2Result {
                secret_package_hex: secret_hex(),
                packages: vec![round2_package()],
            })
            .expect("part2 result serializes")["secret_package_hex"]
                .clone(),
            serde_json::to_value(DkgPart3Request {
                secret_package_hex: secret_hex(),
                round1_packages: vec![round1_package()],
                round2_packages: vec![round2_package()],
            })
            .expect("part3 request serializes")["secret_package_hex"]
                .clone(),
            serde_json::to_value(key_package()).expect("key package serializes")["data_hex"]
                .clone(),
        ];

        for serialized_field in serialized_fields {
            assert_eq!(serialized_field, serde_json::json!(SECRET_SENTINEL));
        }
    }

    #[test]
    fn dkg_secret_fields_redact_direct_and_nested_debug_output() {
        let rendered = [
            format!("{:?}", round2_package()),
            format!(
                "{:?}",
                DkgPart1Result {
                    secret_package_hex: secret_hex(),
                    package: round1_package(),
                }
            ),
            format!(
                "{:?}",
                DkgPart2Request {
                    secret_package_hex: secret_hex(),
                    round1_packages: vec![round1_package()],
                }
            ),
            format!(
                "{:?}",
                DkgPart2Result {
                    secret_package_hex: secret_hex(),
                    packages: vec![round2_package()],
                }
            ),
            format!("{:?}", key_package()),
            format!(
                "{:?}",
                DkgPart3Request {
                    secret_package_hex: secret_hex(),
                    round1_packages: vec![round1_package()],
                    round2_packages: vec![round2_package()],
                }
            ),
            format!(
                "{:?}",
                DkgPart3Result {
                    key_package: key_package(),
                    public_key_package: public_key_package(),
                }
            ),
            format!(
                "{:?}",
                PersistDistributedDkgKeyPackageRequest {
                    session_id: "debug-redaction-session".to_string(),
                    participant_identifier: 1,
                    threshold: 2,
                    participant_count: 3,
                    key_package: key_package(),
                    public_key_package: public_key_package(),
                }
            ),
        ];

        for rendered_value in rendered {
            assert!(
                !rendered_value.contains(SECRET_SENTINEL),
                "Debug output leaked DKG secret material: {rendered_value}"
            );
            assert!(
                rendered_value.contains("<redacted>"),
                "Debug output did not mark DKG secret material as redacted: {rendered_value}"
            );
        }
    }
}
