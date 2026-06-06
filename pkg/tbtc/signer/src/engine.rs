use bitcoin::{
    absolute::LockTime,
    consensus::encode::{deserialize, serialize_hex},
    secp256k1::{
        schnorr::Signature as SchnorrSignature, Message as SecpMessage, Secp256k1, XOnlyPublicKey,
    },
    transaction::Version,
    Amount, OutPoint, ScriptBuf, Sequence, Transaction, TxIn, TxOut, Txid, Witness,
};
use chacha20poly1305::aead::{Aead, KeyInit, OsRng, Payload};
use chacha20poly1305::{XChaCha20Poly1305, XNonce};
#[cfg(unix)]
use libc::{flock, EAGAIN, EWOULDBLOCK, LOCK_EX, LOCK_NB};
use std::collections::{BTreeMap, HashMap, HashSet, VecDeque};
use std::fs;
use std::io::{Read, Write};
#[cfg(unix)]
use std::os::unix::fs::OpenOptionsExt;
#[cfg(unix)]
use std::os::unix::process::CommandExt;
use std::path::{Path, PathBuf};
use std::process::{Output, Stdio};
use std::str::FromStr;
use std::sync::{mpsc, Mutex, OnceLock};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use frost_secp256k1_tr::{self as frost, keys::EvenY};
use rand_chacha::rand_core::{CryptoRng, Error as RandCoreError, RngCore, SeedableRng};
use rand_chacha::ChaCha20Rng;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use zeroize::{Zeroize, Zeroizing};

use crate::api::{
    AggregateRequest, AggregateResult, AttemptContext, AttemptExclusionEvidence,
    AttemptTransitionEvidence, AttemptTransitionTelemetry, BlameProofVerificationResult,
    BuildTaprootTxRequest, CanaryRolloutStatusResult, DifferentialDivergence,
    DifferentialFuzzRequest, DifferentialFuzzResult, DkgPart1Request, DkgPart1Result,
    DkgPart2Request, DkgPart2Result, DkgPart3Request, DkgPart3Result, DkgResult, DkgRound1Package,
    DkgRound2Package, FinalizeSignRoundRequest, GenerateNoncesAndCommitmentsRequest,
    GenerateNoncesAndCommitmentsResult, NativeFrostCommitment, NativeFrostKeyPackage,
    NativeFrostPublicKeyPackage, NativeFrostSignatureShare, NewSigningPackageRequest,
    NewSigningPackageResult, PromoteCanaryRequest, PromoteCanaryResult, QuarantineStatusRequest,
    QuarantineStatusResult, RefreshCadenceStatusRequest, RefreshCadenceStatusResult,
    RefreshSharesRequest, RefreshSharesResult, RoastLivenessPolicyResult, RollbackCanaryRequest,
    RollbackCanaryResult, RoundContribution, RoundState, RunDkgRequest, ShareMaterial,
    SignShareRequest, SignShareResult, SignatureResult, SignerHardeningMetricsResult,
    StartSignRoundRequest, TransactionResult, TranscriptAuditRecord, TranscriptAuditRequest,
    TranscriptAuditResult, TriggerEmergencyRekeyRequest, TriggerEmergencyRekeyResult,
    VerifyBlameProofRequest,
};
use crate::errors::EngineError;
use crate::go_math_rand::select_coordinator_identifier;

type SecretString = Zeroizing<String>;
type SecretBytes = Zeroizing<Vec<u8>>;

struct ZeroizingChaCha20Rng {
    inner: ChaCha20Rng,
}

impl ZeroizingChaCha20Rng {
    fn from_seed(seed: [u8; 32]) -> Self {
        Self {
            inner: ChaCha20Rng::from_seed(seed),
        }
    }
}

impl RngCore for ZeroizingChaCha20Rng {
    fn next_u32(&mut self) -> u32 {
        self.inner.next_u32()
    }

    fn next_u64(&mut self) -> u64 {
        self.inner.next_u64()
    }

    fn fill_bytes(&mut self, dest: &mut [u8]) {
        self.inner.fill_bytes(dest)
    }

    fn try_fill_bytes(&mut self, dest: &mut [u8]) -> Result<(), RandCoreError> {
        self.inner.try_fill_bytes(dest)
    }
}

impl CryptoRng for ZeroizingChaCha20Rng {}

impl Drop for ZeroizingChaCha20Rng {
    fn drop(&mut self) {
        // ChaCha20Rng does not expose a zeroizing Drop. Wipe its in-memory
        // state once the cryptographic operation consuming it has returned.
        unsafe {
            let rng_bytes = std::slice::from_raw_parts_mut(
                (&mut self.inner as *mut ChaCha20Rng).cast::<u8>(),
                std::mem::size_of::<ChaCha20Rng>(),
            );
            rng_bytes.zeroize();
        }
    }
}

#[derive(Default)]
struct SessionState {
    dkg_request_fingerprint: Option<String>,
    dkg_key_packages: Option<BTreeMap<u16, frost::keys::KeyPackage>>,
    dkg_public_key_package: Option<frost::keys::PublicKeyPackage>,
    dkg_result: Option<DkgResult>,
    sign_request_fingerprint: Option<String>,
    sign_message_bytes: Option<SecretBytes>,
    round_state: Option<RoundState>,
    active_attempt_context: Option<AttemptContext>,
    attempt_transition_records: Vec<TranscriptAuditRecord>,
    consumed_attempt_ids: HashSet<String>,
    consumed_sign_round_ids: HashSet<String>,
    finalize_request_fingerprint: Option<String>,
    signature_result: Option<SignatureResult>,
    consumed_finalize_round_ids: HashSet<String>,
    consumed_finalize_request_fingerprints: HashSet<String>,
    build_tx_request_fingerprint: Option<String>,
    tx_result: Option<TransactionResult>,
    refresh_request_fingerprint: Option<String>,
    refresh_result: Option<RefreshSharesResult>,
    refresh_history: Vec<RefreshHistoryRecord>,
    emergency_rekey_event: Option<EmergencyRekeyEvent>,
}

#[derive(Default)]
struct EngineState {
    sessions: HashMap<String, SessionState>,
    refresh_epoch_counter: u64,
    operator_fault_scores: BTreeMap<u16, u64>,
    quarantined_operator_identifiers: HashSet<u16>,
    canary_rollout: CanaryRolloutState,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct RefreshHistoryRecord {
    refresh_epoch: u64,
    refreshed_at_unix: u64,
    share_count: u16,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    key_group: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct EmergencyRekeyEvent {
    reason: String,
    triggered_at_unix: u64,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct CanaryRolloutState {
    current_percent: u8,
    previous_percent: u8,
    config_version: u64,
    last_action_unix: u64,
}

impl Default for CanaryRolloutState {
    fn default() -> Self {
        Self {
            current_percent: 10,
            previous_percent: 10,
            config_version: 1,
            last_action_unix: now_unix(),
        }
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct PersistedKeyPackage {
    identifier: u16,
    key_package_hex: SecretString,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct PersistedSessionState {
    dkg_request_fingerprint: Option<String>,
    dkg_key_packages: Option<Vec<PersistedKeyPackage>>,
    dkg_public_key_package_hex: Option<String>,
    dkg_result: Option<DkgResult>,
    sign_request_fingerprint: Option<String>,
    sign_message_hex: Option<SecretString>,
    round_state: Option<RoundState>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    active_attempt_context: Option<AttemptContext>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    attempt_transition_records: Vec<TranscriptAuditRecord>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    consumed_attempt_ids: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    consumed_sign_round_ids: Vec<String>,
    finalize_request_fingerprint: Option<String>,
    signature_result: Option<SignatureResult>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    consumed_finalize_round_ids: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    consumed_finalize_request_fingerprints: Vec<String>,
    build_tx_request_fingerprint: Option<String>,
    tx_result: Option<TransactionResult>,
    refresh_request_fingerprint: Option<String>,
    refresh_result: Option<RefreshSharesResult>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    refresh_history: Vec<RefreshHistoryRecord>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    emergency_rekey_event: Option<EmergencyRekeyEvent>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct PersistedEngineState {
    schema_version: u16,
    sessions: HashMap<String, PersistedSessionState>,
    refresh_epoch_counter: u64,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    operator_fault_scores: BTreeMap<u16, u64>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    quarantined_operator_identifiers: Vec<u16>,
    #[serde(default)]
    canary_rollout: CanaryRolloutState,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct PersistedEncryptedEngineStateEnvelope {
    schema_version: u16,
    encryption_algorithm: String,
    key_provider: String,
    key_id: String,
    nonce: String,
    ciphertext: String,
    authentication_tag: String,
}

enum PersistedStateStorageFormat {
    EncryptedEnvelope {
        persisted: PersistedEngineState,
        should_rewrite: bool,
    },
    LegacyPlaintext(PersistedEngineState),
}

struct StateEncryptionKeyMaterial {
    key: Zeroizing<[u8; 32]>,
    key_provider: &'static str,
    key_id: String,
}

const PERSISTED_STATE_SCHEMA_VERSION: u16 = 1;
const PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION_V2: u16 = 2;
const PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION: u16 = 3;
const TBTC_SIGNER_STATE_ENCRYPTION_ALGORITHM_XCHACHA20POLY1305: &str = "xchacha20poly1305";
const TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT: &str = "env";
const TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND: &str = "command";
// Env-var selector for key provider implementation (`env` or `command`).
const TBTC_SIGNER_STATE_KEY_PROVIDER_ENV: &str = "TBTC_SIGNER_STATE_KEY_PROVIDER";
const TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX: &str = "TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX";
const TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV: &str = "TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX";
const TBTC_SIGNER_STATE_KEY_COMMAND_ENV: &str = "TBTC_SIGNER_STATE_KEY_COMMAND";
const TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV: &str =
    "TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS";
const TBTC_SIGNER_DEFAULT_STATE_KEY_COMMAND_TIMEOUT_SECS: u64 = 30;
const TBTC_SIGNER_MIN_STATE_KEY_COMMAND_TIMEOUT_SECS: u64 = 1;
const TBTC_SIGNER_MAX_STATE_KEY_COMMAND_TIMEOUT_SECS: u64 = 300;
const TBTC_SIGNER_PROFILE_ENV: &str = "TBTC_SIGNER_PROFILE";
const TBTC_SIGNER_PROFILE_PRODUCTION: &str = "production";
const TBTC_SIGNER_PROFILE_DEVELOPMENT: &str = "development";
const TBTC_SIGNER_STATE_ENVELOPE_NONCE_BYTES: usize = 24;
const TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES: usize = 16;
#[cfg(test)]
const TEST_STATE_ENCRYPTION_KEY_HEX: &str =
    "1111111111111111111111111111111111111111111111111111111111111111";
const TBTC_SIGNER_STATE_PATH_ENV: &str = "TBTC_SIGNER_STATE_PATH";
const TBTC_SIGNER_DEFAULT_STATE_FILENAME: &str = "frost_tbtc_engine_state.json";
const TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV: &str = "TBTC_SIGNER_STATE_CORRUPTION_POLICY";
const TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET: &str = "quarantine_and_reset";
const TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT_ENV: &str = "TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT";
const TBTC_SIGNER_DEFAULT_CORRUPT_BACKUP_LIMIT: usize = 5;
const TBTC_SIGNER_MAX_SESSIONS_ENV: &str = "TBTC_SIGNER_MAX_SESSIONS";
const TBTC_SIGNER_DEFAULT_MAX_SESSIONS: usize = 1024;
const TBTC_SIGNER_STATE_LOCKFILE_SUFFIX: &str = ".lock";
const TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV: &str = "TBTC_SIGNER_ENABLE_ROAST_STRICT";
#[cfg(any(test, feature = "bench-restart-hook"))]
const TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK_ENV: &str = "TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK";
const TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV: &str =
    "TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS";
const TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS: u64 = 30_000;
const TBTC_SIGNER_MIN_ROAST_COORDINATOR_TIMEOUT_MS: u64 = 1_000;
const TBTC_SIGNER_MAX_ROAST_COORDINATOR_TIMEOUT_MS: u64 = 300_000;
const TBTC_SIGNER_RUNTIME_VERSION: &str = env!("CARGO_PKG_VERSION");
const TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV: &str = "TBTC_SIGNER_ENFORCE_PROVENANCE_GATE";
const TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV: &str =
    "TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS";
const TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV: &str =
    "TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD";
const TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV: &str =
    "TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX";
const TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV: &str = "TBTC_SIGNER_PROVENANCE_TRUST_ROOT";
const TBTC_SIGNER_MIN_APPROVED_VERSION_ENV: &str = "TBTC_SIGNER_MIN_APPROVED_VERSION";
const TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED: &str = "approved";
const TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS: u64 = 7 * 24 * 3600;
const TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV: &str = "TBTC_SIGNER_ENFORCE_ADMISSION_POLICY";
const TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS_ENV: &str = "TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS";
const TBTC_SIGNER_ADMISSION_MIN_THRESHOLD_ENV: &str = "TBTC_SIGNER_ADMISSION_MIN_THRESHOLD";
const TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS_ENV: &str =
    "TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS";
const TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV: &str =
    "TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS";
const TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV: &str =
    "TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL";
const TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV: &str =
    "TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES";
const TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV: &str = "TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT";
const TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV: &str =
    "TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS";
const TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV: &str =
    "TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS";
const TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV: &str =
    "TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR";
const TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV: &str = "TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR";
const TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV: &str =
    "TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE";
const TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV: &str = "TBTC_SIGNER_ENABLE_AUTO_QUARANTINE";
const TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV: &str =
    "TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD";
const TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV: &str =
    "TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY";
const TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV: &str =
    "TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY";
const TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV: &str =
    "TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS";
const TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_FAULT_THRESHOLD: u64 = 3;
const TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_TIMEOUT_PENALTY: u64 = 1;
const TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_INVALID_SHARE_PENALTY: u64 = 2;
const TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV: &str = "TBTC_SIGNER_REFRESH_CADENCE_SECONDS";
const TBTC_SIGNER_DEFAULT_REFRESH_CADENCE_SECONDS: u64 = 24 * 60 * 60;
const TBTC_SIGNER_MIN_REFRESH_CADENCE_SECONDS: u64 = 60;
const TBTC_SIGNER_MAX_REFRESH_CADENCE_SECONDS: u64 = 30 * 24 * 60 * 60;
const TBTC_SIGNER_DIFFERENTIAL_FUZZ_MAX_CASES: u32 = 512;
const TBTC_SIGNER_DIFFERENTIAL_FUZZ_DEFAULT_CASES: u32 = 64;
const TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV: &str =
    "TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS";
const TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS_ENV: &str =
    "TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS";
const TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV: &str =
    "TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS";
const TBTC_SIGNER_DEFAULT_CANARY_MAX_START_SIGN_ROUND_P95_MS: u64 = 5_000;
const TBTC_SIGNER_DEFAULT_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS: u64 = 5_000;
const TBTC_SIGNER_DEFAULT_CANARY_MAX_POLICY_REJECT_RATE_BPS: u64 = 1_000;
const TBTC_SIGNER_MAX_POLICY_REJECT_RATE_BPS: u64 = 10_000;
const BITCOIN_MAX_MONEY_SATS: u64 = 2_100_000_000_000_000;
const TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION: usize = 128;
const TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION: usize = 256;

static ENGINE_STATE: OnceLock<Mutex<EngineState>> = OnceLock::new();
static STATE_FILE_LOCK: OnceLock<Mutex<Option<StateFileLock>>> = OnceLock::new();
static STATE_PATH_OVERRIDE_WARNED: OnceLock<()> = OnceLock::new();
static POLICY_GATE_WARNING_EMITTED: OnceLock<()> = OnceLock::new();
static HARDENING_TELEMETRY: OnceLock<Mutex<HardeningTelemetryState>> = OnceLock::new();
static BUILD_TX_RATE_LIMITER: OnceLock<Mutex<BuildTxRateLimiterState>> = OnceLock::new();
#[cfg(test)]
static PERSIST_FAULT_INJECTION_POINT: OnceLock<Mutex<Option<PersistFaultInjectionPoint>>> =
    OnceLock::new();
const BOOTSTRAP_SYNTHETIC_CONTRIBUTION_DOMAIN: &str = "tbtc-signer-bootstrap-contribution-v1";
const ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN: &str = "FROST-ROAST-INCLUDED-FPR-v1";
const ROAST_ATTEMPT_ID_DOMAIN: &str = "FROST-ROAST-ATTEMPT-ID-v1";
const ROUND_ID_NO_ATTEMPT_CONTEXT_COMPONENT: &str = "none";
const ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT: &str = "coordinator_timeout";
const ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF: &str = "invalid_share_proof";
const BUILD_TX_RATE_LIMIT_TOKEN_SCALE: u128 = 1_000_000;
const BUILD_TX_RATE_LIMIT_SECONDS_PER_MINUTE: u128 = 60;
const HARDENING_LATENCY_SAMPLE_WINDOW: usize = 256;

enum CorruptStatePolicy {
    FailClosed,
    QuarantineAndReset,
}

#[derive(Default)]
struct HardeningLatencyTracker {
    samples_ms: VecDeque<u64>,
}

impl HardeningLatencyTracker {
    fn record(&mut self, duration_ms: u64) {
        if self.samples_ms.len() >= HARDENING_LATENCY_SAMPLE_WINDOW {
            self.samples_ms.pop_front();
        }
        self.samples_ms.push_back(duration_ms);
    }

    fn p95_ms(&self) -> u64 {
        if self.samples_ms.is_empty() {
            return 0;
        }

        let mut sorted_samples = self.samples_ms.iter().copied().collect::<Vec<_>>();
        sorted_samples.sort_unstable();
        let p95_index = (sorted_samples.len() * 95).div_ceil(100).saturating_sub(1);
        sorted_samples[p95_index]
    }

    fn sample_count(&self) -> u64 {
        self.samples_ms.len() as u64
    }
}

#[derive(Default)]
struct HardeningTelemetryState {
    run_dkg_calls_total: u64,
    run_dkg_success_total: u64,
    run_dkg_admission_reject_total: u64,
    start_sign_round_calls_total: u64,
    start_sign_round_success_total: u64,
    build_taproot_tx_calls_total: u64,
    build_taproot_tx_success_total: u64,
    build_taproot_tx_policy_reject_total: u64,
    finalize_sign_round_calls_total: u64,
    finalize_sign_round_success_total: u64,
    refresh_shares_calls_total: u64,
    refresh_shares_success_total: u64,
    roast_transcript_audit_calls_total: u64,
    roast_transcript_audit_success_total: u64,
    verify_blame_proof_calls_total: u64,
    verify_blame_proof_success_total: u64,
    attempt_transition_total: u64,
    coordinator_failover_total: u64,
    auto_quarantine_fault_events_total: u64,
    auto_quarantine_enforcements_total: u64,
    differential_fuzz_runs_total: u64,
    differential_fuzz_critical_divergence_total: u64,
    canary_promotions_total: u64,
    canary_rollbacks_total: u64,
    run_dkg_latency: HardeningLatencyTracker,
    start_sign_round_latency: HardeningLatencyTracker,
    build_taproot_tx_latency: HardeningLatencyTracker,
    finalize_sign_round_latency: HardeningLatencyTracker,
    refresh_shares_latency: HardeningLatencyTracker,
    last_updated_unix: u64,
}

#[derive(Clone, Copy)]
enum HardeningOperation {
    RunDkg,
    StartSignRound,
    BuildTaprootTx,
    FinalizeSignRound,
    RefreshShares,
}

struct HardeningOperationLatencyGuard {
    operation: HardeningOperation,
    started_at: Instant,
}

impl HardeningOperationLatencyGuard {
    fn new(operation: HardeningOperation) -> Self {
        Self {
            operation,
            started_at: Instant::now(),
        }
    }
}

impl Drop for HardeningOperationLatencyGuard {
    fn drop(&mut self) {
        // Record latency with millisecond precision and ceil semantics so
        // sub-millisecond calls still contribute non-zero samples.
        let elapsed_micros = self.started_at.elapsed().as_micros();
        let elapsed_ms = elapsed_micros.div_ceil(1000).clamp(1, u64::MAX as u128) as u64;
        record_hardening_operation_latency(self.operation, elapsed_ms);
    }
}

#[derive(Default)]
struct BuildTxRateLimiterState {
    last_refill_unix: u64,
    token_microunits: u128,
    configured_rate_limit_per_minute: u64,
}

#[derive(Clone, Debug)]
struct AdmissionPolicyConfig {
    min_participants: usize,
    min_threshold: u16,
    required_identifiers: HashSet<u16>,
    allowlist_identifiers: Option<HashSet<u16>>,
}

#[derive(Clone, Debug)]
struct SigningPolicyFirewallConfig {
    allowed_script_classes: HashSet<String>,
    max_output_count: usize,
    max_output_value_sats: u64,
    max_total_output_value_sats: u64,
    allowed_utc_start_hour: Option<u8>,
    allowed_utc_end_hour: Option<u8>,
    rate_limit_per_minute: u64,
}

#[derive(Clone, Debug)]
struct AutoQuarantineConfig {
    fault_threshold: u64,
    timeout_penalty: u64,
    invalid_share_penalty: u64,
    dao_allowlist_identifiers: HashSet<u16>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum PersistFaultInjectionPoint {
    AfterTempSyncBeforeRename,
    AfterRenameBeforeDirectorySync,
}

struct StateFileLock {
    _file: fs::File,
    state_path: PathBuf,
    lock_path: PathBuf,
}

impl StateFileLock {
    fn acquire(state_path: &Path) -> Result<Self, EngineError> {
        let lock_path = state_lock_file_path(state_path);
        if let Some(parent) = lock_path.parent() {
            fs::create_dir_all(parent).map_err(|e| {
                EngineError::Internal(format!(
                    "failed to create signer state lock directory [{}]: {e}",
                    parent.display()
                ))
            })?;
        }

        let mut lock_file = fs::OpenOptions::new()
            .create(true)
            .truncate(false)
            .read(true)
            .write(true)
            .open(&lock_path)
            .map_err(|e| {
                EngineError::Internal(format!(
                    "failed to open signer state lock file [{}]: {e}",
                    lock_path.display()
                ))
            })?;

        #[cfg(unix)]
        {
            use std::os::fd::AsRawFd;

            let rc = unsafe { flock(lock_file.as_raw_fd(), LOCK_EX | LOCK_NB) };
            if rc != 0 {
                let lock_error = std::io::Error::last_os_error();
                if lock_error
                    .raw_os_error()
                    .is_some_and(is_lock_contention_errno)
                {
                    return Err(EngineError::Internal(format!(
                        "signer state lock already held by another process [{}]",
                        lock_path.display()
                    )));
                }

                return Err(EngineError::Internal(format!(
                    "failed to lock signer state file [{}]: {lock_error}",
                    lock_path.display()
                )));
            }
        }

        lock_file.set_len(0).map_err(|e| {
            EngineError::Internal(format!(
                "failed to truncate signer state lock file [{}]: {e}",
                lock_path.display()
            ))
        })?;
        writeln!(
            lock_file,
            "pid={}\nstate_path={}",
            std::process::id(),
            state_path.display()
        )
        .map_err(|e| {
            EngineError::Internal(format!(
                "failed to write signer state lock file [{}]: {e}",
                lock_path.display()
            ))
        })?;
        lock_file.sync_all().map_err(|e| {
            EngineError::Internal(format!(
                "failed to sync signer state lock file [{}]: {e}",
                lock_path.display()
            ))
        })?;

        Ok(Self {
            _file: lock_file,
            state_path: state_path.to_path_buf(),
            lock_path,
        })
    }
}

fn state_file_lock_slot() -> &'static Mutex<Option<StateFileLock>> {
    STATE_FILE_LOCK.get_or_init(|| Mutex::new(None))
}

#[cfg(unix)]
fn is_lock_contention_errno(errno: i32) -> bool {
    errno == EAGAIN || errno == EWOULDBLOCK
}

fn state() -> Result<&'static Mutex<EngineState>, EngineError> {
    ensure_state_file_lock()?;
    warn_disabled_policy_gates();

    if let Some(state) = ENGINE_STATE.get() {
        return Ok(state);
    }

    let loaded_state = load_engine_state_from_storage()?;
    Ok(ENGINE_STATE.get_or_init(|| Mutex::new(loaded_state)))
}

fn state_file_path() -> Result<PathBuf, EngineError> {
    let configured_path = std::env::var(TBTC_SIGNER_STATE_PATH_ENV)
        .ok()
        .map(|path| path.trim().to_string())
        .filter(|path| !path.is_empty())
        .map(PathBuf::from);

    if let Some(path) = configured_path {
        STATE_PATH_OVERRIDE_WARNED.get_or_init(|| {
            eprintln!(
                "warning: {} override is set to [{}]; ensure this path is operator-restricted",
                TBTC_SIGNER_STATE_PATH_ENV,
                path.display()
            );
        });
        return Ok(path);
    }

    if signer_profile_is_production() {
        return Err(EngineError::Internal(format!(
            "{} must be set when {}={}; refusing to use the implicit temp-dir signer state path",
            TBTC_SIGNER_STATE_PATH_ENV, TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION
        )));
    }

    Ok(std::env::temp_dir().join(TBTC_SIGNER_DEFAULT_STATE_FILENAME))
}

fn active_state_file_path() -> Result<PathBuf, EngineError> {
    let lock_slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;

    if let Some(lock) = lock_slot.as_ref() {
        return Ok(lock.state_path.clone());
    }

    state_file_path()
}

fn state_lock_file_path(state_path: &Path) -> PathBuf {
    let state_filename = state_path
        .file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_else(|| TBTC_SIGNER_DEFAULT_STATE_FILENAME.to_string());
    let lock_filename = format!("{state_filename}{TBTC_SIGNER_STATE_LOCKFILE_SUFFIX}");

    if let Some(parent) = state_path.parent() {
        parent.join(&lock_filename)
    } else {
        PathBuf::from(lock_filename)
    }
}

fn ensure_state_file_lock() -> Result<(), EngineError> {
    let state_path = state_file_path()?;
    let mut lock_slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;

    if let Some(existing_lock) = lock_slot.as_ref() {
        if existing_lock.state_path == state_path {
            return Ok(());
        }

        return Err(EngineError::Internal(format!(
            "state file lock already initialized for [{}] with lock [{}]; refusing to switch to [{}] in-process",
            existing_lock.state_path.display(),
            existing_lock.lock_path.display(),
            state_path.display()
        )));
    }

    *lock_slot = Some(StateFileLock::acquire(&state_path)?);
    Ok(())
}

fn state_corruption_policy() -> CorruptStatePolicy {
    let policy = std::env::var(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV)
        .ok()
        .map(|value| value.trim().to_ascii_lowercase())
        .unwrap_or_default();

    if policy == TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET {
        CorruptStatePolicy::QuarantineAndReset
    } else {
        CorruptStatePolicy::FailClosed
    }
}

fn state_corrupt_backup_limit() -> usize {
    std::env::var(TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT_ENV)
        .ok()
        .and_then(|value| value.trim().parse::<usize>().ok())
        .unwrap_or(TBTC_SIGNER_DEFAULT_CORRUPT_BACKUP_LIMIT)
}

fn max_sessions_limit() -> usize {
    std::env::var(TBTC_SIGNER_MAX_SESSIONS_ENV)
        .ok()
        .and_then(|value| value.trim().parse::<usize>().ok())
        .filter(|limit| *limit > 0)
        .unwrap_or(TBTC_SIGNER_DEFAULT_MAX_SESSIONS)
}

fn roast_coordinator_timeout_ms() -> u64 {
    std::env::var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV)
        .ok()
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|timeout_ms| {
            *timeout_ms >= TBTC_SIGNER_MIN_ROAST_COORDINATOR_TIMEOUT_MS
                && *timeout_ms <= TBTC_SIGNER_MAX_ROAST_COORDINATOR_TIMEOUT_MS
        })
        .unwrap_or(TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS)
}

fn refresh_cadence_seconds() -> u64 {
    std::env::var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV)
        .ok()
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| {
            *value >= TBTC_SIGNER_MIN_REFRESH_CADENCE_SECONDS
                && *value <= TBTC_SIGNER_MAX_REFRESH_CADENCE_SECONDS
        })
        .unwrap_or(TBTC_SIGNER_DEFAULT_REFRESH_CADENCE_SECONDS)
}

fn canary_max_start_sign_round_p95_ms() -> u64 {
    std::env::var(TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV)
        .ok()
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| *value > 0)
        .unwrap_or(TBTC_SIGNER_DEFAULT_CANARY_MAX_START_SIGN_ROUND_P95_MS)
}

fn canary_max_finalize_sign_round_p95_ms() -> u64 {
    std::env::var(TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS_ENV)
        .ok()
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| *value > 0)
        .unwrap_or(TBTC_SIGNER_DEFAULT_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS)
}

fn canary_max_policy_reject_rate_bps() -> u64 {
    std::env::var(TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV)
        .ok()
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| *value <= TBTC_SIGNER_MAX_POLICY_REJECT_RATE_BPS)
        .unwrap_or(TBTC_SIGNER_DEFAULT_CANARY_MAX_POLICY_REJECT_RATE_BPS)
}

fn next_canary_percent(current_percent: u8) -> Option<u8> {
    match current_percent {
        10 => Some(50),
        50 => Some(100),
        _ => None,
    }
}

fn can_promote_to_target_percent(current_percent: u8, target_percent: u8) -> bool {
    next_canary_percent(current_percent).is_some_and(|next| next == target_percent)
}

pub fn roast_liveness_policy() -> RoastLivenessPolicyResult {
    RoastLivenessPolicyResult {
        coordinator_timeout_ms: roast_coordinator_timeout_ms(),
        timeout_source: "keep_core_wall_clock".to_string(),
        advance_trigger: "coordinator_timeout".to_string(),
        exclusion_evidence_policy: "timeout_or_invalid_share_proof".to_string(),
    }
}

fn hardening_telemetry_state() -> &'static Mutex<HardeningTelemetryState> {
    HARDENING_TELEMETRY.get_or_init(|| Mutex::new(HardeningTelemetryState::default()))
}

fn build_tx_rate_limiter_state() -> &'static Mutex<BuildTxRateLimiterState> {
    BUILD_TX_RATE_LIMITER.get_or_init(|| Mutex::new(BuildTxRateLimiterState::default()))
}

fn record_hardening_telemetry<F>(update: F)
where
    F: FnOnce(&mut HardeningTelemetryState),
{
    match hardening_telemetry_state().lock() {
        Ok(mut telemetry) => {
            update(&mut telemetry);
            telemetry.last_updated_unix = now_unix();
        }
        Err(error) => {
            eprintln!("warning: hardening telemetry mutex poisoned: {error}");
        }
    }
}

fn record_hardening_operation_latency(operation: HardeningOperation, duration_ms: u64) {
    record_hardening_telemetry(|telemetry| match operation {
        HardeningOperation::RunDkg => telemetry.run_dkg_latency.record(duration_ms),
        HardeningOperation::StartSignRound => {
            telemetry.start_sign_round_latency.record(duration_ms)
        }
        HardeningOperation::BuildTaprootTx => {
            telemetry.build_taproot_tx_latency.record(duration_ms)
        }
        HardeningOperation::FinalizeSignRound => {
            telemetry.finalize_sign_round_latency.record(duration_ms)
        }
        HardeningOperation::RefreshShares => telemetry.refresh_shares_latency.record(duration_ms),
    });
}

fn provenance_gate_enforced() -> bool {
    if signer_profile_is_production() {
        return true;
    }

    std::env::var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

fn admission_policy_enforced() -> bool {
    std::env::var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

fn signing_policy_firewall_enforced() -> bool {
    std::env::var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

fn warn_disabled_policy_gates() {
    POLICY_GATE_WARNING_EMITTED.get_or_init(|| {
        if !provenance_gate_enforced() {
            eprintln!(
                "warning: provenance gate is DISABLED; set {}=true to enforce signed attestation verification",
                TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV
            );
        }
        if !admission_policy_enforced() {
            eprintln!(
                "warning: admission policy is DISABLED; set {}=true to enforce DKG admission controls",
                TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV
            );
        }
        if !signing_policy_firewall_enforced() {
            eprintln!(
                "warning: signing policy firewall is DISABLED; set {}=true to enforce transaction policy controls",
                TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV
            );
        }
    });
}

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
struct ParsedVersionTriplet {
    major: u64,
    minor: u64,
    patch: u64,
    has_prerelease_suffix: bool,
}

fn parse_version_triplet(version: &str) -> Option<ParsedVersionTriplet> {
    let mut core_version = version.trim();
    if let Some((prefix, _)) = core_version.split_once('+') {
        core_version = prefix;
    }
    let has_prerelease_suffix = core_version.contains('-');
    if let Some((prefix, _)) = core_version.split_once('-') {
        core_version = prefix;
    }

    let mut segments = core_version.split('.');
    let major = segments.next()?.parse::<u64>().ok()?;
    let minor = segments.next()?.parse::<u64>().ok()?;
    let patch = segments.next()?.parse::<u64>().ok()?;
    if segments.next().is_some() {
        return None;
    }

    Some(ParsedVersionTriplet {
        major,
        minor,
        patch,
        has_prerelease_suffix,
    })
}

fn runtime_satisfies_minimum_version(
    runtime_version: ParsedVersionTriplet,
    minimum_version: ParsedVersionTriplet,
) -> bool {
    if runtime_version.major != minimum_version.major {
        return runtime_version.major > minimum_version.major;
    }
    if runtime_version.minor != minimum_version.minor {
        return runtime_version.minor > minimum_version.minor;
    }
    if runtime_version.patch != minimum_version.patch {
        return runtime_version.patch > minimum_version.patch;
    }

    if runtime_version.has_prerelease_suffix && !minimum_version.has_prerelease_suffix {
        return false;
    }

    true
}

#[derive(Clone, Debug, Deserialize)]
struct ProvenanceAttestationPayload {
    status: String,
    runtime_version: String,
    #[serde(default)]
    expires_at_unix: Option<u64>,
}

fn parse_provenance_trust_root_pubkey(trust_root: &str) -> Result<XOnlyPublicKey, EngineError> {
    let trust_root_bytes =
        hex::decode(trust_root).map_err(|_| EngineError::ProvenanceGateRejected {
            reason_code: "invalid_trust_root_format".to_string(),
            detail: format!(
                "env [{}] must be 32-byte x-only public key hex",
                TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV
            ),
        })?;

    if trust_root_bytes.len() != 32 {
        return Err(EngineError::ProvenanceGateRejected {
            reason_code: "invalid_trust_root_format".to_string(),
            detail: format!(
                "env [{}] must decode to 32-byte x-only public key",
                TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV
            ),
        });
    }

    XOnlyPublicKey::from_slice(&trust_root_bytes).map_err(|_| EngineError::ProvenanceGateRejected {
        reason_code: "invalid_trust_root_format".to_string(),
        detail: format!(
            "env [{}] must decode to valid x-only secp256k1 public key",
            TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV
        ),
    })
}

fn parse_provenance_attestation_payload(
    payload: &str,
) -> Result<ProvenanceAttestationPayload, EngineError> {
    serde_json::from_str::<ProvenanceAttestationPayload>(payload).map_err(|_| {
        EngineError::ProvenanceGateRejected {
            reason_code: "invalid_attestation_payload".to_string(),
            detail: format!(
                "env [{}] must be JSON with fields [status, runtime_version]",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV
            ),
        }
    })
}

fn verify_provenance_attestation_signature(
    attestation_payload: &str,
    attestation_signature_hex: &str,
    trust_root_pubkey: &XOnlyPublicKey,
) -> Result<(), EngineError> {
    let signature_bytes = hex::decode(attestation_signature_hex).map_err(|_| {
        EngineError::ProvenanceGateRejected {
            reason_code: "invalid_attestation_signature_format".to_string(),
            detail: format!(
                "env [{}] must be schnorr signature hex",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV
            ),
        }
    })?;
    let signature = SchnorrSignature::from_slice(&signature_bytes).map_err(|_| {
        EngineError::ProvenanceGateRejected {
            reason_code: "invalid_attestation_signature_format".to_string(),
            detail: format!(
                "env [{}] must decode to valid schnorr signature bytes",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV
            ),
        }
    })?;

    let payload_digest = Sha256::digest(attestation_payload.as_bytes());
    let message = SecpMessage::from_digest_slice(&payload_digest).map_err(|e| {
        EngineError::Internal(format!(
            "failed to construct provenance signature digest: {e}"
        ))
    })?;
    let secp = Secp256k1::verification_only();
    secp.verify_schnorr(&signature, &message, trust_root_pubkey)
        .map_err(|e| EngineError::ProvenanceGateRejected {
            reason_code: "attestation_signature_verification_failed".to_string(),
            detail: format!("failed to verify attestation signature: {e}"),
        })
}

fn reject_provenance_gate(reason_code: &str, detail: impl Into<String>) -> Result<(), EngineError> {
    Err(EngineError::ProvenanceGateRejected {
        reason_code: reason_code.to_string(),
        detail: detail.into(),
    })
}

fn enforce_provenance_gate() -> Result<(), EngineError> {
    if !provenance_gate_enforced() {
        return Ok(());
    }

    let attestation_status = std::env::var(TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV)
        .unwrap_or_default()
        .trim()
        .to_ascii_lowercase();
    if attestation_status.is_empty() {
        return reject_provenance_gate(
            "missing_attestation_status",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV
            ),
        );
    }
    if attestation_status != TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED {
        return reject_provenance_gate(
            "unapproved_attestation_status",
            format!(
                "attestation status must be [{}], got [{}]",
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED, attestation_status
            ),
        );
    }

    let trust_root = std::env::var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV)
        .unwrap_or_default()
        .trim()
        .to_string();
    if trust_root.is_empty() {
        return reject_provenance_gate(
            "missing_trust_root",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV
            ),
        );
    }
    let trust_root_pubkey = parse_provenance_trust_root_pubkey(&trust_root)?;

    let raw_attestation_payload =
        std::env::var(TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV).unwrap_or_default();
    let attestation_payload = raw_attestation_payload.trim().to_string();
    if attestation_payload.len() != raw_attestation_payload.len() {
        eprintln!(
            "provenance_gate: warning: env [{}] had leading/trailing whitespace (trimmed {} bytes)",
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            raw_attestation_payload
                .len()
                .saturating_sub(attestation_payload.len())
        );
    }
    if attestation_payload.is_empty() {
        return reject_provenance_gate(
            "missing_attestation_payload",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV
            ),
        );
    }

    let attestation_signature_hex =
        std::env::var(TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV)
            .unwrap_or_default()
            .trim()
            .to_string();
    if attestation_signature_hex.is_empty() {
        return reject_provenance_gate(
            "missing_attestation_signature",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV
            ),
        );
    }

    verify_provenance_attestation_signature(
        &attestation_payload,
        &attestation_signature_hex,
        &trust_root_pubkey,
    )?;
    let parsed_attestation_payload = parse_provenance_attestation_payload(&attestation_payload)?;
    let attestation_payload_status = parsed_attestation_payload
        .status
        .trim()
        .to_ascii_lowercase();
    if attestation_payload_status != attestation_status {
        return reject_provenance_gate(
            "attestation_status_mismatch",
            format!(
                "attestation payload status [{}] does not match env status [{}]",
                attestation_payload_status, attestation_status
            ),
        );
    }
    if parsed_attestation_payload.runtime_version.trim() != TBTC_SIGNER_RUNTIME_VERSION {
        return reject_provenance_gate(
            "runtime_version_not_attested",
            format!(
                "attestation payload runtime version [{}] does not match runtime version [{}]",
                parsed_attestation_payload.runtime_version, TBTC_SIGNER_RUNTIME_VERSION
            ),
        );
    }
    let now_unix_seconds = now_unix();
    if now_unix_seconds == 0 {
        return reject_provenance_gate(
            "clock_unavailable",
            "system clock returned epoch zero; cannot verify attestation freshness",
        );
    }

    let expires_at_unix = parsed_attestation_payload.expires_at_unix.ok_or_else(|| {
        EngineError::ProvenanceGateRejected {
            reason_code: "missing_attestation_expiry".to_string(),
            detail: format!(
                "attestation payload must include expires_at_unix (max TTL: {} seconds)",
                TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS
            ),
        }
    })?;

    if now_unix_seconds > expires_at_unix {
        return reject_provenance_gate(
            "attestation_expired",
            format!(
                "attestation expired at [{}], now [{}]",
                expires_at_unix, now_unix_seconds
            ),
        );
    }

    let max_expiry_unix =
        now_unix_seconds.saturating_add(TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS);
    if expires_at_unix > max_expiry_unix {
        return reject_provenance_gate(
            "attestation_expiry_too_far_in_future",
            format!(
                "attestation expires_at_unix [{}] exceeds max TTL [{} seconds] from now [{}]",
                expires_at_unix,
                TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS,
                now_unix_seconds
            ),
        );
    }

    let min_approved_version = std::env::var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV)
        .unwrap_or_default()
        .trim()
        .to_string();
    if min_approved_version.is_empty() {
        return reject_provenance_gate(
            "missing_minimum_approved_version",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_MIN_APPROVED_VERSION_ENV
            ),
        );
    }

    let runtime_version = parse_version_triplet(TBTC_SIGNER_RUNTIME_VERSION).ok_or_else(|| {
        EngineError::Internal(format!(
            "invalid runtime version format [{}]",
            TBTC_SIGNER_RUNTIME_VERSION
        ))
    })?;
    let required_version = parse_version_triplet(&min_approved_version).ok_or_else(|| {
        EngineError::ProvenanceGateRejected {
            reason_code: "invalid_minimum_approved_version".to_string(),
            detail: format!(
                "minimum approved version [{}] is not semver triplet",
                min_approved_version
            ),
        }
    })?;

    if !runtime_satisfies_minimum_version(runtime_version, required_version) {
        return reject_provenance_gate(
            "runtime_version_below_minimum",
            format!(
                "runtime version [{}] below minimum approved version [{}]",
                TBTC_SIGNER_RUNTIME_VERSION, min_approved_version
            ),
        );
    }

    Ok(())
}

fn parse_identifier_set_from_env(env_name: &str) -> Result<Option<HashSet<u16>>, EngineError> {
    let Ok(raw_value) = std::env::var(env_name) else {
        return Ok(None);
    };

    let raw_value = raw_value.trim();
    if raw_value.is_empty() {
        return Err(EngineError::Internal(format!(
            "identifier list env [{}] must be unset or contain at least one identifier",
            env_name
        )));
    }

    let mut identifiers = HashSet::new();
    for token in raw_value.split(',') {
        let token = token.trim();
        if token.is_empty() {
            continue;
        }

        let identifier = token.parse::<u16>().map_err(|_| {
            EngineError::Internal(format!(
                "failed to parse identifier [{}] from env [{}]",
                token, env_name
            ))
        })?;
        if identifier == 0 {
            return Err(EngineError::Internal(format!(
                "identifier list env [{}] contains zero identifier",
                env_name
            )));
        }
        identifiers.insert(identifier);
    }

    Ok(Some(identifiers))
}

fn parse_usize_from_env_with_default(
    env_name: &str,
    default_value: usize,
) -> Result<usize, EngineError> {
    let Ok(raw_value) = std::env::var(env_name) else {
        return Ok(default_value);
    };

    let parsed = raw_value.trim().parse::<usize>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse usize env [{}] value [{}]",
            env_name, raw_value
        ))
    })?;
    Ok(parsed)
}

fn parse_u64_from_env_with_default(env_name: &str, default_value: u64) -> Result<u64, EngineError> {
    let Ok(raw_value) = std::env::var(env_name) else {
        return Ok(default_value);
    };

    let parsed = raw_value.trim().parse::<u64>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse u64 env [{}] value [{}]",
            env_name, raw_value
        ))
    })?;
    Ok(parsed)
}

fn parse_usize_from_env_required(env_name: &str) -> Result<usize, EngineError> {
    let raw_value = std::env::var(env_name)
        .map_err(|_| EngineError::Internal(format!("missing required env [{}]", env_name)))?;
    raw_value.trim().parse::<usize>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse usize env [{}] value [{}]",
            env_name, raw_value
        ))
    })
}

fn parse_u64_from_env_required(env_name: &str) -> Result<u64, EngineError> {
    let raw_value = std::env::var(env_name)
        .map_err(|_| EngineError::Internal(format!("missing required env [{}]", env_name)))?;
    raw_value.trim().parse::<u64>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse u64 env [{}] value [{}]",
            env_name, raw_value
        ))
    })
}

fn parse_u8_from_env_optional(env_name: &str) -> Result<Option<u8>, EngineError> {
    let Ok(raw_value) = std::env::var(env_name) else {
        return Ok(None);
    };

    let parsed = raw_value.trim().parse::<u8>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse u8 env [{}] value [{}]",
            env_name, raw_value
        ))
    })?;
    if parsed > 23 {
        return Err(EngineError::Internal(format!(
            "hour env [{}] must be in range 0..=23, got [{}]",
            env_name, parsed
        )));
    }
    Ok(Some(parsed))
}

fn parse_script_class_set_required(env_name: &str) -> Result<HashSet<String>, EngineError> {
    let raw_value = std::env::var(env_name)
        .map_err(|_| EngineError::Internal(format!("missing required env [{}]", env_name)))?;
    let raw_value = raw_value.trim();
    if raw_value.is_empty() {
        return Err(EngineError::Internal(format!(
            "required env [{}] must not be empty",
            env_name
        )));
    }

    let mut script_classes = HashSet::new();
    for token in raw_value.split(',') {
        let normalized = token.trim().to_ascii_lowercase();
        if normalized.is_empty() {
            continue;
        }
        script_classes.insert(normalized);
    }

    if script_classes.is_empty() {
        return Err(EngineError::Internal(format!(
            "required env [{}] produced an empty script class set",
            env_name
        )));
    }

    Ok(script_classes)
}

fn load_admission_policy_config() -> Result<Option<AdmissionPolicyConfig>, EngineError> {
    if !admission_policy_enforced() {
        return Ok(None);
    }

    let min_participants =
        parse_usize_from_env_with_default(TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS_ENV, 2)?;
    let min_threshold =
        parse_u64_from_env_with_default(TBTC_SIGNER_ADMISSION_MIN_THRESHOLD_ENV, 2)?
            .try_into()
            .map_err(|_| {
                EngineError::Internal(format!(
                    "env [{}] exceeds u16 bounds",
                    TBTC_SIGNER_ADMISSION_MIN_THRESHOLD_ENV
                ))
            })?;
    let required_identifiers =
        parse_identifier_set_from_env(TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS_ENV)?
            .unwrap_or_default();
    let allowlist_identifiers =
        parse_identifier_set_from_env(TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV)?;

    Ok(Some(AdmissionPolicyConfig {
        min_participants,
        min_threshold,
        required_identifiers,
        allowlist_identifiers,
    }))
}

fn sanitize_policy_log_field(value: &str) -> String {
    value
        .chars()
        .map(|character| {
            if character.is_ascii_alphanumeric() || matches!(character, '-' | '_' | '.' | ':' | '/')
            {
                character
            } else {
                '_'
            }
        })
        .collect()
}

fn log_policy_decision(stage: &str, session_id: &str, decision: &str, reason_code: &str) {
    let stage = sanitize_policy_log_field(stage);
    let session_id = sanitize_policy_log_field(session_id);
    let decision = sanitize_policy_log_field(decision);
    let reason_code = sanitize_policy_log_field(reason_code);

    eprintln!(
        "policy_decision stage={} session_id={} decision={} reason_code={}",
        stage, session_id, decision, reason_code
    );
}

fn reject_admission_policy(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<(), EngineError> {
    let detail = detail.into();
    record_hardening_telemetry(|telemetry| {
        telemetry.run_dkg_admission_reject_total =
            telemetry.run_dkg_admission_reject_total.saturating_add(1);
    });
    log_policy_decision("admission_policy", session_id, "reject", reason_code);
    Err(EngineError::AdmissionPolicyRejected {
        session_id: session_id.to_string(),
        reason_code: reason_code.to_string(),
        detail,
    })
}

fn enforce_admission_policy(request: &RunDkgRequest) -> Result<(), EngineError> {
    let policy = match load_admission_policy_config() {
        Ok(Some(policy)) => policy,
        Ok(None) => return Ok(()),
        Err(error) => {
            return reject_admission_policy(
                &request.session_id,
                "invalid_policy_configuration",
                error.to_string(),
            )
        }
    };

    if request.participants.len() < policy.min_participants {
        return reject_admission_policy(
            &request.session_id,
            "participant_count_below_policy_minimum",
            format!(
                "participant count [{}] below policy minimum [{}]",
                request.participants.len(),
                policy.min_participants
            ),
        );
    }

    if request.threshold < policy.min_threshold {
        return reject_admission_policy(
            &request.session_id,
            "threshold_below_policy_minimum",
            format!(
                "threshold [{}] below policy minimum [{}]",
                request.threshold, policy.min_threshold
            ),
        );
    }

    let participant_identifiers: HashSet<u16> = request
        .participants
        .iter()
        .map(|participant| participant.identifier)
        .collect();
    if let Some(required_identifier) = policy
        .required_identifiers
        .iter()
        .find(|identifier| !participant_identifiers.contains(identifier))
    {
        return reject_admission_policy(
            &request.session_id,
            "required_identifier_missing",
            format!(
                "required identifier [{}] missing from request",
                required_identifier
            ),
        );
    }

    if let Some(allowlist_identifiers) = policy.allowlist_identifiers.as_ref() {
        if let Some(unknown_identifier) = participant_identifiers
            .iter()
            .find(|identifier| !allowlist_identifiers.contains(identifier))
        {
            return reject_admission_policy(
                &request.session_id,
                "participant_identifier_not_allowlisted",
                format!(
                    "participant identifier [{}] not present in configured allowlist",
                    unknown_identifier
                ),
            );
        }
    }

    log_policy_decision("admission_policy", &request.session_id, "allow", "ok");
    Ok(())
}

fn load_signing_policy_firewall_config() -> Result<Option<SigningPolicyFirewallConfig>, EngineError>
{
    if !signing_policy_firewall_enforced() {
        return Ok(None);
    }

    let allowed_script_classes =
        parse_script_class_set_required(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV)?;
    let max_output_count = parse_usize_from_env_required(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV)?;
    let max_output_value_sats =
        parse_u64_from_env_required(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV)?;
    let max_total_output_value_sats =
        parse_u64_from_env_required(TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV)?;
    let allowed_utc_start_hour =
        parse_u8_from_env_optional(TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV)?;
    let allowed_utc_end_hour =
        parse_u8_from_env_optional(TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV)?;
    let rate_limit_per_minute =
        parse_u64_from_env_with_default(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV, 60)?;

    if rate_limit_per_minute == 0 {
        return Err(EngineError::Internal(format!(
            "env [{}] must be positive",
            TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV
        )));
    }

    if allowed_utc_start_hour.is_some() != allowed_utc_end_hour.is_some() {
        return Err(EngineError::Internal(format!(
            "env [{}] and [{}] must be configured together",
            TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV,
            TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV
        )));
    }

    Ok(Some(SigningPolicyFirewallConfig {
        allowed_script_classes,
        max_output_count,
        max_output_value_sats,
        max_total_output_value_sats,
        allowed_utc_start_hour,
        allowed_utc_end_hour,
        rate_limit_per_minute,
    }))
}

fn auto_quarantine_enabled() -> bool {
    std::env::var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

fn load_auto_quarantine_config() -> Result<Option<AutoQuarantineConfig>, EngineError> {
    if !auto_quarantine_enabled() {
        return Ok(None);
    }

    let fault_threshold = parse_u64_from_env_with_default(
        TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV,
        TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_FAULT_THRESHOLD,
    )?;
    let timeout_penalty = parse_u64_from_env_with_default(
        TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV,
        TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_TIMEOUT_PENALTY,
    )?;
    let invalid_share_penalty = parse_u64_from_env_with_default(
        TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV,
        TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_INVALID_SHARE_PENALTY,
    )?;
    let dao_allowlist_identifiers =
        parse_identifier_set_from_env(TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV)?
            .unwrap_or_default();

    if fault_threshold == 0 {
        return Err(EngineError::Internal(format!(
            "env [{}] must be positive",
            TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV
        )));
    }
    if timeout_penalty == 0 {
        return Err(EngineError::Internal(format!(
            "env [{}] must be positive",
            TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV
        )));
    }
    if invalid_share_penalty == 0 {
        return Err(EngineError::Internal(format!(
            "env [{}] must be positive",
            TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV
        )));
    }

    Ok(Some(AutoQuarantineConfig {
        fault_threshold,
        timeout_penalty,
        invalid_share_penalty,
        dao_allowlist_identifiers,
    }))
}

fn reject_quarantine_policy(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<(), EngineError> {
    let detail = detail.into();
    log_policy_decision("auto_quarantine", session_id, "reject", reason_code);
    Err(EngineError::QuarantinePolicyRejected {
        session_id: session_id.to_string(),
        reason_code: reason_code.to_string(),
        detail,
    })
}

fn reject_lifecycle_policy<T>(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<T, EngineError> {
    let detail = detail.into();
    log_policy_decision("lifecycle_policy", session_id, "reject", reason_code);
    Err(EngineError::LifecyclePolicyRejected {
        session_id: session_id.to_string(),
        reason_code: reason_code.to_string(),
        detail,
    })
}

fn reject_signing_policy(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<(), EngineError> {
    let detail = detail.into();
    record_hardening_telemetry(|telemetry| {
        telemetry.build_taproot_tx_policy_reject_total = telemetry
            .build_taproot_tx_policy_reject_total
            .saturating_add(1);
    });
    log_policy_decision("signing_policy_firewall", session_id, "reject", reason_code);
    Err(EngineError::SigningPolicyRejected {
        session_id: session_id.to_string(),
        reason_code: reason_code.to_string(),
        detail,
    })
}

fn current_utc_hour() -> u8 {
    ((now_unix() / 3600) % 24) as u8
}

fn utc_hour_in_window(hour: u8, start_hour: u8, end_hour: u8) -> bool {
    if start_hour == end_hour {
        return true;
    }
    if start_hour < end_hour {
        return hour >= start_hour && hour < end_hour;
    }

    hour >= start_hour || hour < end_hour
}

fn enforce_build_tx_rate_limit(
    session_id: &str,
    rate_limit_per_minute: u64,
) -> Result<(), EngineError> {
    let mut limiter = build_tx_rate_limiter_state()
        .lock()
        .map_err(|_| EngineError::Internal("build tx rate limiter mutex poisoned".to_string()))?;

    let now = now_unix();
    let max_tokens =
        (rate_limit_per_minute as u128).saturating_mul(BUILD_TX_RATE_LIMIT_TOKEN_SCALE);
    if limiter.last_refill_unix == 0 {
        limiter.last_refill_unix = now;
        limiter.token_microunits = max_tokens;
        limiter.configured_rate_limit_per_minute = rate_limit_per_minute;
    }

    if limiter.configured_rate_limit_per_minute != rate_limit_per_minute {
        limiter.configured_rate_limit_per_minute = rate_limit_per_minute;
        limiter.token_microunits = limiter.token_microunits.min(max_tokens);
    }

    let elapsed_seconds = now.saturating_sub(limiter.last_refill_unix);
    if elapsed_seconds > 0 {
        let refill_microunits = (elapsed_seconds as u128)
            .saturating_mul(rate_limit_per_minute as u128)
            .saturating_mul(BUILD_TX_RATE_LIMIT_TOKEN_SCALE)
            / BUILD_TX_RATE_LIMIT_SECONDS_PER_MINUTE;
        limiter.token_microunits = limiter
            .token_microunits
            .saturating_add(refill_microunits)
            .min(max_tokens);
        limiter.last_refill_unix = now;
    }

    if limiter.token_microunits < BUILD_TX_RATE_LIMIT_TOKEN_SCALE {
        return reject_signing_policy(
            session_id,
            "rate_limit_per_minute_exceeded",
            format!("rate limit [{}] per minute exceeded", rate_limit_per_minute),
        );
    }

    limiter.token_microunits = limiter
        .token_microunits
        .saturating_sub(BUILD_TX_RATE_LIMIT_TOKEN_SCALE);
    Ok(())
}

fn classify_script_pubkey(script_pubkey: &ScriptBuf) -> &'static str {
    if script_pubkey.is_p2tr() {
        "p2tr"
    } else if script_pubkey.is_p2wpkh() {
        "p2wpkh"
    } else if script_pubkey.is_p2wsh() {
        "p2wsh"
    } else if script_pubkey.is_p2pkh() {
        "p2pkh"
    } else if script_pubkey.is_p2sh() {
        "p2sh"
    } else {
        "other"
    }
}

fn enforce_signing_policy_firewall_inner(
    session_id: &str,
    outputs: &[TxOut],
    total_output_value_sats: u64,
    charge_rate_limit: bool,
) -> Result<(), EngineError> {
    let policy = match load_signing_policy_firewall_config() {
        Ok(Some(policy)) => policy,
        Ok(None) => return Ok(()),
        Err(error) => {
            return reject_signing_policy(
                session_id,
                "invalid_policy_configuration",
                error.to_string(),
            )
        }
    };

    if outputs.len() > policy.max_output_count {
        return reject_signing_policy(
            session_id,
            "output_count_exceeds_policy_limit",
            format!(
                "output count [{}] exceeds policy max [{}]",
                outputs.len(),
                policy.max_output_count
            ),
        );
    }

    if total_output_value_sats > policy.max_total_output_value_sats {
        return reject_signing_policy(
            session_id,
            "total_output_value_exceeds_policy_limit",
            format!(
                "total output value [{}] exceeds policy max [{}]",
                total_output_value_sats, policy.max_total_output_value_sats
            ),
        );
    }

    for output in outputs {
        let output_value_sats = output.value.to_sat();
        if output_value_sats > policy.max_output_value_sats {
            return reject_signing_policy(
                session_id,
                "single_output_value_exceeds_policy_limit",
                format!(
                    "output value [{}] exceeds policy max [{}]",
                    output_value_sats, policy.max_output_value_sats
                ),
            );
        }

        let script_class = classify_script_pubkey(&output.script_pubkey).to_string();
        if !policy.allowed_script_classes.contains(&script_class) {
            return reject_signing_policy(
                session_id,
                "script_class_not_allowlisted",
                format!(
                    "script class [{}] not in allowlist {:?}",
                    script_class, policy.allowed_script_classes
                ),
            );
        }
    }

    if let (Some(start_hour), Some(end_hour)) =
        (policy.allowed_utc_start_hour, policy.allowed_utc_end_hour)
    {
        let current_hour = current_utc_hour();
        if !utc_hour_in_window(current_hour, start_hour, end_hour) {
            return reject_signing_policy(
                session_id,
                "request_outside_allowed_utc_window",
                format!(
                    "current UTC hour [{}] not in window [{}..{})",
                    current_hour, start_hour, end_hour
                ),
            );
        }
    }

    if charge_rate_limit {
        enforce_build_tx_rate_limit(session_id, policy.rate_limit_per_minute)?;
    }
    log_policy_decision("signing_policy_firewall", session_id, "allow", "ok");
    Ok(())
}

fn enforce_signing_policy_firewall(
    session_id: &str,
    outputs: &[TxOut],
    total_output_value_sats: u64,
) -> Result<(), EngineError> {
    enforce_signing_policy_firewall_inner(session_id, outputs, total_output_value_sats, true)
}

fn recheck_signing_policy_firewall_without_rate_limit(
    session_id: &str,
    outputs: &[TxOut],
    total_output_value_sats: u64,
) -> Result<(), EngineError> {
    enforce_signing_policy_firewall_inner(session_id, outputs, total_output_value_sats, false)
}

fn policy_bound_signing_message_hex(tx_hex: &str) -> Result<String, EngineError> {
    let tx_bytes = hex::decode(tx_hex).map_err(|_| {
        EngineError::Internal("policy-checked build tx hex is not valid hex".to_string())
    })?;
    Ok(hash_hex(&tx_bytes))
}

fn enforce_signing_message_binding_to_policy_checked_build_tx(
    session_id: &str,
    signing_message_hex: &str,
    tx_result: Option<&TransactionResult>,
) -> Result<(), EngineError> {
    if !signing_policy_firewall_enforced() {
        return Ok(());
    }

    let tx_result = match tx_result {
        Some(tx_result) => tx_result,
        None => {
            return reject_signing_policy(
                session_id,
                "missing_policy_checked_build_tx",
                "signing policy firewall requires build_taproot_tx to run before signing for this session",
            )
        }
    };

    let expected_signing_message_hex = policy_bound_signing_message_hex(&tx_result.tx_hex)
        .map_err(|error| EngineError::SigningPolicyRejected {
            session_id: session_id.to_string(),
            reason_code: "invalid_policy_checked_build_tx_artifact".to_string(),
            detail: error.to_string(),
        })?;
    let signing_message_hex = signing_message_hex.trim().to_ascii_lowercase();
    if signing_message_hex != expected_signing_message_hex {
        return reject_signing_policy(
            session_id,
            "signing_message_not_bound_to_policy_checked_build_tx",
            format!(
                "signing message [{}] does not match policy-checked build tx digest [{}]",
                signing_message_hex, expected_signing_message_hex
            ),
        );
    }

    Ok(())
}

pub fn hardening_metrics() -> SignerHardeningMetricsResult {
    let mut result = SignerHardeningMetricsResult {
        runtime_version: TBTC_SIGNER_RUNTIME_VERSION.to_string(),
        provenance_enforced: provenance_gate_enforced(),
        admission_policy_enforced: admission_policy_enforced(),
        signing_policy_firewall_enforced: signing_policy_firewall_enforced(),
        run_dkg_calls_total: 0,
        run_dkg_success_total: 0,
        run_dkg_admission_reject_total: 0,
        start_sign_round_calls_total: 0,
        start_sign_round_success_total: 0,
        build_taproot_tx_calls_total: 0,
        build_taproot_tx_success_total: 0,
        build_taproot_tx_policy_reject_total: 0,
        finalize_sign_round_calls_total: 0,
        finalize_sign_round_success_total: 0,
        refresh_shares_calls_total: 0,
        refresh_shares_success_total: 0,
        roast_transcript_audit_calls_total: 0,
        roast_transcript_audit_success_total: 0,
        verify_blame_proof_calls_total: 0,
        verify_blame_proof_success_total: 0,
        attempt_transition_total: 0,
        coordinator_failover_total: 0,
        auto_quarantine_fault_events_total: 0,
        auto_quarantine_enforcements_total: 0,
        quarantined_operator_count: 0,
        refresh_cadence_overdue_sessions: 0,
        emergency_rekey_sessions_total: 0,
        differential_fuzz_runs_total: 0,
        differential_fuzz_critical_divergence_total: 0,
        canary_promotions_total: 0,
        canary_rollbacks_total: 0,
        run_dkg_latency_p95_ms: 0,
        run_dkg_latency_samples: 0,
        start_sign_round_latency_p95_ms: 0,
        start_sign_round_latency_samples: 0,
        build_taproot_tx_latency_p95_ms: 0,
        build_taproot_tx_latency_samples: 0,
        finalize_sign_round_latency_p95_ms: 0,
        finalize_sign_round_latency_samples: 0,
        refresh_shares_latency_p95_ms: 0,
        refresh_shares_latency_samples: 0,
        last_updated_unix: 0,
    };

    match hardening_telemetry_state().lock() {
        Ok(telemetry) => {
            result.run_dkg_calls_total = telemetry.run_dkg_calls_total;
            result.run_dkg_success_total = telemetry.run_dkg_success_total;
            result.run_dkg_admission_reject_total = telemetry.run_dkg_admission_reject_total;
            result.start_sign_round_calls_total = telemetry.start_sign_round_calls_total;
            result.start_sign_round_success_total = telemetry.start_sign_round_success_total;
            result.build_taproot_tx_calls_total = telemetry.build_taproot_tx_calls_total;
            result.build_taproot_tx_success_total = telemetry.build_taproot_tx_success_total;
            result.build_taproot_tx_policy_reject_total =
                telemetry.build_taproot_tx_policy_reject_total;
            result.finalize_sign_round_calls_total = telemetry.finalize_sign_round_calls_total;
            result.finalize_sign_round_success_total = telemetry.finalize_sign_round_success_total;
            result.refresh_shares_calls_total = telemetry.refresh_shares_calls_total;
            result.refresh_shares_success_total = telemetry.refresh_shares_success_total;
            result.roast_transcript_audit_calls_total =
                telemetry.roast_transcript_audit_calls_total;
            result.roast_transcript_audit_success_total =
                telemetry.roast_transcript_audit_success_total;
            result.verify_blame_proof_calls_total = telemetry.verify_blame_proof_calls_total;
            result.verify_blame_proof_success_total = telemetry.verify_blame_proof_success_total;
            result.attempt_transition_total = telemetry.attempt_transition_total;
            result.coordinator_failover_total = telemetry.coordinator_failover_total;
            result.auto_quarantine_fault_events_total =
                telemetry.auto_quarantine_fault_events_total;
            result.auto_quarantine_enforcements_total =
                telemetry.auto_quarantine_enforcements_total;
            result.differential_fuzz_runs_total = telemetry.differential_fuzz_runs_total;
            result.differential_fuzz_critical_divergence_total =
                telemetry.differential_fuzz_critical_divergence_total;
            result.canary_promotions_total = telemetry.canary_promotions_total;
            result.canary_rollbacks_total = telemetry.canary_rollbacks_total;
            result.run_dkg_latency_p95_ms = telemetry.run_dkg_latency.p95_ms();
            result.run_dkg_latency_samples = telemetry.run_dkg_latency.sample_count();
            result.start_sign_round_latency_p95_ms = telemetry.start_sign_round_latency.p95_ms();
            result.start_sign_round_latency_samples =
                telemetry.start_sign_round_latency.sample_count();
            result.build_taproot_tx_latency_p95_ms = telemetry.build_taproot_tx_latency.p95_ms();
            result.build_taproot_tx_latency_samples =
                telemetry.build_taproot_tx_latency.sample_count();
            result.finalize_sign_round_latency_p95_ms =
                telemetry.finalize_sign_round_latency.p95_ms();
            result.finalize_sign_round_latency_samples =
                telemetry.finalize_sign_round_latency.sample_count();
            result.refresh_shares_latency_p95_ms = telemetry.refresh_shares_latency.p95_ms();
            result.refresh_shares_latency_samples = telemetry.refresh_shares_latency.sample_count();
            result.last_updated_unix = telemetry.last_updated_unix;
        }
        Err(error) => {
            eprintln!("warning: hardening telemetry mutex poisoned: {error}");
        }
    }

    if let Ok(state) = state() {
        if let Ok(engine_state) = state.lock() {
            result.quarantined_operator_count =
                engine_state.quarantined_operator_identifiers.len() as u64;
            result.emergency_rekey_sessions_total = engine_state
                .sessions
                .values()
                .filter(|session| session.emergency_rekey_event.is_some())
                .count() as u64;
            result.refresh_cadence_overdue_sessions = engine_state
                .sessions
                .values()
                .filter(|session| {
                    session.refresh_history.last().is_some_and(|last_refresh| {
                        now_unix()
                            > last_refresh
                                .refreshed_at_unix
                                .saturating_add(refresh_cadence_seconds())
                    })
                })
                .count() as u64;
        }
    }

    result
}

fn canary_policy_reject_rate_bps(metrics: &SignerHardeningMetricsResult) -> u64 {
    if metrics.build_taproot_tx_calls_total == 0 {
        return 0;
    }

    metrics
        .build_taproot_tx_policy_reject_total
        .saturating_mul(TBTC_SIGNER_MAX_POLICY_REJECT_RATE_BPS)
        .saturating_div(metrics.build_taproot_tx_calls_total)
}

fn canary_promotion_gate_failures(metrics: &SignerHardeningMetricsResult) -> Vec<String> {
    let mut failures = Vec::new();

    let max_start_sign_round_p95_ms = canary_max_start_sign_round_p95_ms();
    if metrics.start_sign_round_latency_samples > 0
        && metrics.start_sign_round_latency_p95_ms > max_start_sign_round_p95_ms
    {
        failures.push(format!(
            "start_sign_round p95 latency [{}ms] exceeds canary gate [{}ms]",
            metrics.start_sign_round_latency_p95_ms, max_start_sign_round_p95_ms
        ));
    }

    let max_finalize_sign_round_p95_ms = canary_max_finalize_sign_round_p95_ms();
    if metrics.finalize_sign_round_latency_samples > 0
        && metrics.finalize_sign_round_latency_p95_ms > max_finalize_sign_round_p95_ms
    {
        failures.push(format!(
            "finalize_sign_round p95 latency [{}ms] exceeds canary gate [{}ms]",
            metrics.finalize_sign_round_latency_p95_ms, max_finalize_sign_round_p95_ms
        ));
    }

    let max_policy_reject_rate_bps = canary_max_policy_reject_rate_bps();
    let policy_reject_rate_bps = canary_policy_reject_rate_bps(metrics);
    if policy_reject_rate_bps > max_policy_reject_rate_bps {
        failures.push(format!(
            "build_taproot_tx policy reject rate [{}bps] exceeds canary gate [{}bps]",
            policy_reject_rate_bps, max_policy_reject_rate_bps
        ));
    }

    failures
}

fn refresh_continuity_reference_key_group(session: &SessionState) -> Option<String> {
    session
        .dkg_result
        .as_ref()
        .map(|result| result.key_group.clone())
        .or_else(|| {
            session
                .refresh_history
                .iter()
                .find_map(|record| record.key_group.clone())
        })
}

fn refresh_history_continuity_preserved(session: &SessionState) -> bool {
    let mut last_refresh_epoch = 0_u64;
    let mut reference_key_group: Option<&str> = None;

    for refresh_record in &session.refresh_history {
        if refresh_record.refresh_epoch == 0 || refresh_record.refresh_epoch <= last_refresh_epoch {
            return false;
        }
        last_refresh_epoch = refresh_record.refresh_epoch;

        if let Some(record_key_group) = refresh_record.key_group.as_deref() {
            if let Some(reference_key_group) = reference_key_group {
                if !record_key_group.eq_ignore_ascii_case(reference_key_group) {
                    return false;
                }
            } else {
                reference_key_group = Some(record_key_group);
            }
        }
    }

    true
}

pub fn refresh_cadence_status(
    request: RefreshCadenceStatusRequest,
) -> Result<RefreshCadenceStatusResult, EngineError> {
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let session =
        guard
            .sessions
            .get(&request.session_id)
            .ok_or_else(|| EngineError::SessionNotFound {
                session_id: request.session_id.clone(),
            })?;
    let cadence_seconds = refresh_cadence_seconds();
    let last_refresh_record = session.refresh_history.last();
    let now = now_unix();
    let next_refresh_due_unix = last_refresh_record
        .map(|record| record.refreshed_at_unix.saturating_add(cadence_seconds))
        .unwrap_or_else(|| now.saturating_add(cadence_seconds));
    let overdue = now > next_refresh_due_unix;
    let continuity_reference_key_group = refresh_continuity_reference_key_group(session);
    let emergency_rekey_reason = session
        .emergency_rekey_event
        .as_ref()
        .map(|event| event.reason.clone());

    Ok(RefreshCadenceStatusResult {
        session_id: request.session_id,
        refresh_count: session.refresh_history.len() as u64,
        last_refresh_epoch: last_refresh_record
            .map(|record| record.refresh_epoch)
            .unwrap_or(0),
        cadence_seconds,
        next_refresh_due_unix,
        overdue,
        continuity_preserved: refresh_history_continuity_preserved(session),
        continuity_reference_key_group,
        emergency_rekey_required: session.emergency_rekey_event.is_some(),
        emergency_rekey_reason,
    })
}

pub fn trigger_emergency_rekey(
    request: TriggerEmergencyRekeyRequest,
) -> Result<TriggerEmergencyRekeyResult, EngineError> {
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;
    let reason = request.reason.trim();
    if reason.is_empty() {
        return Err(EngineError::Validation(
            "reason must not be empty".to_string(),
        ));
    }

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let session = guard.sessions.get_mut(&request.session_id).ok_or_else(|| {
        EngineError::SessionNotFound {
            session_id: request.session_id.clone(),
        }
    })?;
    if session.emergency_rekey_event.is_some() {
        return Err(EngineError::Validation(format!(
            "emergency rekey already triggered for session [{}]; event is immutable",
            request.session_id
        )));
    }
    let triggered_at_unix = now_unix();
    session.emergency_rekey_event = Some(EmergencyRekeyEvent {
        reason: reason.to_string(),
        triggered_at_unix,
    });
    persist_engine_state_to_storage(&guard)?;

    Ok(TriggerEmergencyRekeyResult {
        session_id: request.session_id.clone(),
        emergency_rekey_required: true,
        reason: reason.to_string(),
        triggered_at_unix,
        recommended_new_session_id: format!("{}-rekey-{}", request.session_id, triggered_at_unix),
    })
}

fn reference_roast_hash_hex(domain: &str, components: &[Vec<u8>]) -> Result<String, EngineError> {
    let mut payload = Vec::new();
    let domain_bytes = domain.as_bytes();
    let domain_len = u32::try_from(domain_bytes.len()).map_err(|_| {
        EngineError::Validation("reference hash domain exceeds u32 framing limit".to_string())
    })?;
    payload.extend_from_slice(&domain_len.to_be_bytes());
    payload.extend_from_slice(domain_bytes);

    for component in components {
        let component_len = u32::try_from(component.len()).map_err(|_| {
            EngineError::Validation(
                "reference hash component exceeds u32 framing limit".to_string(),
            )
        })?;
        payload.extend_from_slice(&component_len.to_be_bytes());
        payload.extend_from_slice(component);
    }

    Ok(hash_hex(&payload))
}

fn reference_roast_included_participants_fingerprint_hex(
    included_participants: &[u16],
) -> Result<String, EngineError> {
    let mut participant_payload = Vec::new();
    for participant_identifier in included_participants {
        let participant_component = participant_identifier.to_be_bytes();
        let component_len = u32::try_from(participant_component.len()).map_err(|_| {
            EngineError::Validation(
                "reference participant component exceeds u32 framing limit".to_string(),
            )
        })?;
        participant_payload.extend_from_slice(&component_len.to_be_bytes());
        participant_payload.extend_from_slice(&participant_component);
    }

    reference_roast_hash_hex(
        ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN,
        &[participant_payload],
    )
}

fn reference_roast_attempt_id_hex(
    session_id: &str,
    message_digest_hex: &str,
    attempt_number: u32,
    coordinator_identifier: u16,
    included_participants_fingerprint_hex: &str,
) -> Result<String, EngineError> {
    reference_roast_hash_hex(
        ROAST_ATTEMPT_ID_DOMAIN,
        &[
            session_id.as_bytes().to_vec(),
            message_digest_hex.as_bytes().to_vec(),
            attempt_number.to_be_bytes().to_vec(),
            coordinator_identifier.to_be_bytes().to_vec(),
            included_participants_fingerprint_hex.as_bytes().to_vec(),
        ],
    )
}

fn differential_case_count(case_count: u32) -> u32 {
    if case_count == 0 {
        return TBTC_SIGNER_DIFFERENTIAL_FUZZ_DEFAULT_CASES;
    }

    case_count.min(TBTC_SIGNER_DIFFERENTIAL_FUZZ_MAX_CASES)
}

pub fn run_differential_fuzzing(
    request: DifferentialFuzzRequest,
) -> Result<DifferentialFuzzResult, EngineError> {
    enforce_provenance_gate()?;
    let case_count = differential_case_count(request.case_count);
    let seed = if request.seed == 0 {
        0xD1FF_E2E0_A11C_0001
    } else {
        request.seed
    };
    let mut rng = ChaCha20Rng::seed_from_u64(seed);
    let mut divergences = Vec::new();
    let mut critical_divergence_count = 0_u32;

    for case_index in 0..case_count {
        let mut participants = Vec::new();
        let participant_count = (rng.next_u32() % 4 + 2) as usize;
        while participants.len() < participant_count {
            let candidate = (rng.next_u32() % 30 + 1) as u16;
            if !participants.contains(&candidate) {
                participants.push(candidate);
            }
        }
        if participants.len() > 1 {
            let swap_index = (rng.next_u32() as usize) % participants.len();
            participants.swap(0, swap_index);
        }

        let mut digest_bytes = [0_u8; 32];
        rng.fill_bytes(&mut digest_bytes);
        let message_digest_hex = hex::encode(digest_bytes);
        let session_id = format!("differential-session-{seed:016x}-{case_index}");
        let attempt_number = (rng.next_u32() % 16) + 1;
        let coordinator_identifier = participants[(rng.next_u32() as usize) % participants.len()];

        let primary_fingerprint = roast_included_participants_fingerprint_hex(&participants)?;
        let reference_fingerprint =
            reference_roast_included_participants_fingerprint_hex(&participants)?;
        if primary_fingerprint != reference_fingerprint {
            critical_divergence_count = critical_divergence_count.saturating_add(1);
            divergences.push(DifferentialDivergence {
                case_index,
                check: "included_participants_fingerprint".to_string(),
                severity: "critical".to_string(),
                detail: format!(
                    "primary [{}] != reference [{}]",
                    primary_fingerprint, reference_fingerprint
                ),
            });
        }

        let primary_attempt_id = roast_attempt_id_hex(
            &session_id,
            &message_digest_hex,
            attempt_number,
            coordinator_identifier,
            &primary_fingerprint,
        )?;
        let reference_attempt_id = reference_roast_attempt_id_hex(
            &session_id,
            &message_digest_hex,
            attempt_number,
            coordinator_identifier,
            &reference_fingerprint,
        )?;
        if primary_attempt_id != reference_attempt_id {
            critical_divergence_count = critical_divergence_count.saturating_add(1);
            divergences.push(DifferentialDivergence {
                case_index,
                check: "attempt_id".to_string(),
                severity: "critical".to_string(),
                detail: format!(
                    "primary [{}] != reference [{}]",
                    primary_attempt_id, reference_attempt_id
                ),
            });
        }

        let mut txid_bytes = [0_u8; 32];
        rng.fill_bytes(&mut txid_bytes);
        let txid_hex = hex::encode(txid_bytes);
        let txid = Txid::from_str(&txid_hex).map_err(|_| {
            EngineError::Internal("failed to build differential fuzz txid".to_string())
        })?;
        let mut script_pubkey = vec![0x51, 0x20];
        let mut witness_program = [0_u8; 32];
        rng.fill_bytes(&mut witness_program);
        script_pubkey.extend_from_slice(&witness_program);
        let tx = Transaction {
            version: Version::TWO,
            lock_time: LockTime::ZERO,
            input: vec![TxIn {
                previous_output: OutPoint {
                    txid,
                    vout: rng.next_u32() % 4,
                },
                script_sig: ScriptBuf::new(),
                sequence: Sequence::MAX,
                witness: Witness::default(),
            }],
            output: vec![TxOut {
                value: Amount::from_sat((rng.next_u32() as u64 % 1_000_000) + 1),
                script_pubkey: ScriptBuf::from_bytes(script_pubkey),
            }],
        };
        let tx_hex = serialize_hex(&tx);
        let primary_message_digest_hex = policy_bound_signing_message_hex(&tx_hex)?;
        let tx_bytes = hex::decode(&tx_hex).map_err(|_| {
            EngineError::Internal("failed to decode differential tx hex".to_string())
        })?;
        let tx_roundtrip: Transaction = deserialize(&tx_bytes).map_err(|error| {
            EngineError::Internal(format!("failed to deserialize differential tx: {error}"))
        })?;
        let reference_message_digest_hex =
            hash_hex(&bitcoin::consensus::encode::serialize(&tx_roundtrip));
        if primary_message_digest_hex != reference_message_digest_hex {
            critical_divergence_count = critical_divergence_count.saturating_add(1);
            divergences.push(DifferentialDivergence {
                case_index,
                check: "policy_bound_message_digest".to_string(),
                severity: "critical".to_string(),
                detail: format!(
                    "primary [{}] != reference [{}]",
                    primary_message_digest_hex, reference_message_digest_hex
                ),
            });
        }
    }

    record_hardening_telemetry(|telemetry| {
        telemetry.differential_fuzz_runs_total =
            telemetry.differential_fuzz_runs_total.saturating_add(1);
        telemetry.differential_fuzz_critical_divergence_total = telemetry
            .differential_fuzz_critical_divergence_total
            .saturating_add(critical_divergence_count as u64);
    });

    Ok(DifferentialFuzzResult {
        seed,
        case_count,
        divergences,
        critical_divergence_count,
        unresolved_critical_divergence: critical_divergence_count > 0,
    })
}

pub fn canary_rollout_status() -> Result<CanaryRolloutStatusResult, EngineError> {
    enforce_provenance_gate()?;
    let metrics = hardening_metrics();
    let gate_failures = canary_promotion_gate_failures(&metrics);
    let gate_passed = gate_failures.is_empty();
    let (current_percent, previous_percent, config_version, last_action_unix) =
        if let Ok(state) = state() {
            if let Ok(guard) = state.lock() {
                (
                    guard.canary_rollout.current_percent,
                    guard.canary_rollout.previous_percent,
                    guard.canary_rollout.config_version,
                    guard.canary_rollout.last_action_unix,
                )
            } else {
                let default = CanaryRolloutState::default();
                (
                    default.current_percent,
                    default.previous_percent,
                    default.config_version,
                    default.last_action_unix,
                )
            }
        } else {
            let default = CanaryRolloutState::default();
            (
                default.current_percent,
                default.previous_percent,
                default.config_version,
                default.last_action_unix,
            )
        };

    Ok(CanaryRolloutStatusResult {
        current_percent,
        previous_percent,
        config_version,
        promotion_gate_passed: gate_passed,
        gate_failures,
        recommended_next_percent: if gate_passed {
            next_canary_percent(current_percent)
        } else {
            None
        },
        last_action_unix,
    })
}

pub fn promote_canary(request: PromoteCanaryRequest) -> Result<PromoteCanaryResult, EngineError> {
    enforce_provenance_gate()?;
    if !matches!(request.target_percent, 10 | 50 | 100) {
        return Err(EngineError::Validation(
            "target_percent must be one of [10, 50, 100]".to_string(),
        ));
    }

    let metrics = hardening_metrics();
    let gate_failures = canary_promotion_gate_failures(&metrics);
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let current_percent = guard.canary_rollout.current_percent;

    if request.target_percent == current_percent {
        return Ok(PromoteCanaryResult {
            from_percent: current_percent,
            to_percent: current_percent,
            config_version: guard.canary_rollout.config_version,
            promoted_at_unix: guard.canary_rollout.last_action_unix,
        });
    }

    if !can_promote_to_target_percent(current_percent, request.target_percent) {
        return reject_lifecycle_policy(
            "canary-rollout",
            "invalid_canary_promotion_step",
            format!(
                "canary promotion must follow 10->50->100 progression; current [{}], target [{}]",
                current_percent, request.target_percent
            ),
        );
    }
    if !gate_failures.is_empty() {
        return reject_lifecycle_policy(
            "canary-rollout",
            "canary_slo_gate_failed",
            gate_failures.join("; "),
        );
    }

    guard.canary_rollout.previous_percent = current_percent;
    guard.canary_rollout.current_percent = request.target_percent;
    guard.canary_rollout.config_version = guard.canary_rollout.config_version.saturating_add(1);
    guard.canary_rollout.last_action_unix = now_unix();
    let result = PromoteCanaryResult {
        from_percent: current_percent,
        to_percent: request.target_percent,
        config_version: guard.canary_rollout.config_version,
        promoted_at_unix: guard.canary_rollout.last_action_unix,
    };
    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.canary_promotions_total = telemetry.canary_promotions_total.saturating_add(1);
    });

    Ok(result)
}

pub fn rollback_canary(
    request: RollbackCanaryRequest,
) -> Result<RollbackCanaryResult, EngineError> {
    enforce_provenance_gate()?;
    let reason = request.reason.trim();
    if reason.is_empty() {
        return Err(EngineError::Validation(
            "reason must not be empty".to_string(),
        ));
    }

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let from_percent = guard.canary_rollout.current_percent;
    let to_percent = guard.canary_rollout.previous_percent.min(from_percent);
    guard.canary_rollout.current_percent = to_percent;
    guard.canary_rollout.previous_percent = to_percent;
    guard.canary_rollout.config_version = guard.canary_rollout.config_version.saturating_add(1);
    guard.canary_rollout.last_action_unix = now_unix();
    let result = RollbackCanaryResult {
        from_percent,
        to_percent,
        config_version: guard.canary_rollout.config_version,
        reason: reason.to_string(),
        rolled_back_at_unix: guard.canary_rollout.last_action_unix,
    };
    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.canary_rollbacks_total = telemetry.canary_rollbacks_total.saturating_add(1);
    });

    Ok(result)
}

pub fn roast_transcript_audit(
    request: TranscriptAuditRequest,
) -> Result<TranscriptAuditResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.roast_transcript_audit_calls_total = telemetry
            .roast_transcript_audit_calls_total
            .saturating_add(1);
    });
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let session =
        guard
            .sessions
            .get(&request.session_id)
            .ok_or_else(|| EngineError::SessionNotFound {
                session_id: request.session_id.clone(),
            })?;
    let records = session.attempt_transition_records.clone();

    let result = TranscriptAuditResult {
        session_id: request.session_id,
        transition_count: records.len() as u64,
        records,
    };
    record_hardening_telemetry(|telemetry| {
        telemetry.roast_transcript_audit_success_total = telemetry
            .roast_transcript_audit_success_total
            .saturating_add(1);
    });

    Ok(result)
}

pub fn verify_blame_proof(
    request: VerifyBlameProofRequest,
) -> Result<BlameProofVerificationResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.verify_blame_proof_calls_total =
            telemetry.verify_blame_proof_calls_total.saturating_add(1);
    });
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;
    if request.from_attempt_number == 0 {
        return Err(EngineError::Validation(
            "from_attempt_number must be at least 1".to_string(),
        ));
    }
    if request.accused_member_identifier == 0 {
        return Err(EngineError::Validation(
            "accused_member_identifier must be non-zero".to_string(),
        ));
    }

    let reason = request.reason.trim().to_ascii_lowercase();
    if reason != ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT
        && reason != ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF
    {
        return Err(EngineError::Validation(format!(
            "reason [{}] is unsupported",
            request.reason
        )));
    }

    let requested_invalid_share_proof_fingerprint = request
        .invalid_share_proof_fingerprint
        .as_deref()
        .map(|fingerprint| fingerprint.trim().to_ascii_lowercase());
    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let session =
        guard
            .sessions
            .get(&request.session_id)
            .ok_or_else(|| EngineError::SessionNotFound {
                session_id: request.session_id.clone(),
            })?;

    let maybe_record = session
        .attempt_transition_records
        .iter()
        .find(|record| record.from_attempt_number == request.from_attempt_number);
    let (verified, detail, transcript_hash) = if let Some(record) = maybe_record {
        if record.reason != reason {
            (
                false,
                format!(
                    "reason mismatch: requested [{}], recorded [{}]",
                    reason, record.reason
                ),
                Some(record.transcript_hash.clone()),
            )
        } else if !record
            .excluded_member_identifiers
            .contains(&request.accused_member_identifier)
        {
            (
                false,
                format!(
                    "operator [{}] is not excluded in recorded transition",
                    request.accused_member_identifier
                ),
                Some(record.transcript_hash.clone()),
            )
        } else if reason == ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF
            && record.invalid_share_proof_fingerprint != requested_invalid_share_proof_fingerprint
        {
            (
                false,
                "invalid_share_proof_fingerprint does not match recorded transition evidence"
                    .to_string(),
                Some(record.transcript_hash.clone()),
            )
        } else {
            (
                true,
                "blame proof verified against persisted transcript record".to_string(),
                Some(record.transcript_hash.clone()),
            )
        }
    } else {
        (
            false,
            format!(
                "no persisted transition record for from_attempt_number [{}]",
                request.from_attempt_number
            ),
            None,
        )
    };

    if verified {
        record_hardening_telemetry(|telemetry| {
            telemetry.verify_blame_proof_success_total =
                telemetry.verify_blame_proof_success_total.saturating_add(1);
        });
    }

    Ok(BlameProofVerificationResult {
        session_id: request.session_id,
        from_attempt_number: request.from_attempt_number,
        accused_member_identifier: request.accused_member_identifier,
        reason,
        verified,
        transcript_hash,
        detail,
    })
}

pub fn quarantine_status(
    request: QuarantineStatusRequest,
) -> Result<QuarantineStatusResult, EngineError> {
    enforce_provenance_gate()?;
    if request.operator_identifier == 0 {
        return Err(EngineError::Validation(
            "operator_identifier must be non-zero".to_string(),
        ));
    }

    let auto_quarantine_config = load_auto_quarantine_config()?;
    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let fault_score = guard
        .operator_fault_scores
        .get(&request.operator_identifier)
        .copied()
        .unwrap_or(0);
    let quarantined = guard
        .quarantined_operator_identifiers
        .contains(&request.operator_identifier);
    let dao_override_allowlisted = auto_quarantine_config.as_ref().is_some_and(|config| {
        config
            .dao_allowlist_identifiers
            .contains(&request.operator_identifier)
    });

    Ok(QuarantineStatusResult {
        operator_identifier: request.operator_identifier,
        auto_quarantine_enabled: auto_quarantine_config.is_some(),
        fault_score,
        quarantine_threshold: auto_quarantine_config
            .as_ref()
            .map(|config| config.fault_threshold)
            .unwrap_or(0),
        quarantined: quarantined && !dao_override_allowlisted,
        dao_override_allowlisted,
    })
}

#[cfg(any(test, feature = "bench-restart-hook"))]
pub fn reload_state_from_storage_for_benchmarks() -> Result<(), EngineError> {
    if !bench_restart_hook_enabled() {
        return Err(EngineError::Validation(format!(
            "benchmark restart hook disabled; set {}=true to enable",
            TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK_ENV
        )));
    }

    if let Ok(mut lock_slot) = state_file_lock_slot().lock() {
        *lock_slot = None;
    }
    ensure_state_file_lock()?;

    let loaded_state = load_engine_state_from_storage()?;
    let state = state()?;
    let mut guard = state
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    *guard = loaded_state;
    Ok(())
}

fn corrupted_state_backup_prefix(path: &Path) -> String {
    let state_filename = path
        .file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_else(|| TBTC_SIGNER_DEFAULT_STATE_FILENAME.to_string());
    format!("{state_filename}.corrupt-")
}

fn corrupted_state_backup_path(path: &Path) -> PathBuf {
    let backup_prefix = corrupted_state_backup_prefix(path);
    let backup_filename = format!(
        "{}{}-{}",
        backup_prefix,
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|duration| duration.as_nanos())
            .unwrap_or(0),
        std::process::id()
    );

    if let Some(parent) = path.parent() {
        parent.join(&backup_filename)
    } else {
        PathBuf::from(backup_filename)
    }
}

fn sorted_corrupted_state_backups(path: &Path) -> Result<Vec<PathBuf>, EngineError> {
    let Some(parent) = path.parent() else {
        return Ok(Vec::new());
    };
    let backup_prefix = corrupted_state_backup_prefix(path);

    let mut backups = fs::read_dir(parent)
        .map_err(|e| {
            EngineError::Internal(format!(
                "failed to read signer state directory [{}] for backup retention: {e}",
                parent.display()
            ))
        })?
        .filter_map(|entry| entry.ok())
        .filter_map(|entry| {
            let file_name = entry.file_name();
            let file_name = file_name.to_string_lossy();
            if !file_name.starts_with(&backup_prefix) {
                return None;
            }

            let modified = entry
                .metadata()
                .ok()
                .and_then(|metadata| metadata.modified().ok())
                .unwrap_or(UNIX_EPOCH);
            Some((entry.path(), modified))
        })
        .collect::<Vec<_>>();

    backups.sort_by(|left, right| right.1.cmp(&left.1).then_with(|| right.0.cmp(&left.0)));

    Ok(backups.into_iter().map(|(path, _)| path).collect())
}

fn enforce_corrupted_state_backup_retention(path: &Path) -> Result<(), EngineError> {
    let backup_limit = state_corrupt_backup_limit();
    if backup_limit == 0 {
        return Ok(());
    }

    let backup_paths = sorted_corrupted_state_backups(path)?;
    if backup_paths.len() <= backup_limit {
        return Ok(());
    }

    for backup_path in backup_paths.into_iter().skip(backup_limit) {
        fs::remove_file(&backup_path).map_err(|e| {
            EngineError::Internal(format!(
                "failed to evict old corrupted signer state backup [{}]: {e}",
                backup_path.display()
            ))
        })?;
    }

    Ok(())
}

fn recover_or_fail_from_corrupted_state_file(
    path: &Path,
    reason: String,
) -> Result<EngineState, EngineError> {
    match state_corruption_policy() {
        CorruptStatePolicy::FailClosed => Err(EngineError::Internal(format!(
            "{reason}; refusing to continue with corrupted signer state file [{}]. \
set {}={} to quarantine the file and continue with clean state",
            path.display(),
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET
        ))),
        CorruptStatePolicy::QuarantineAndReset => {
            let backup_path = corrupted_state_backup_path(path);
            fs::rename(path, &backup_path).map_err(|e| {
                EngineError::Internal(format!(
                    "failed to quarantine corrupted signer state file [{}] to [{}]: {e}",
                    path.display(),
                    backup_path.display()
                ))
            })?;

            eprintln!(
                "warning: quarantined corrupted signer state file [{}] to [{}]: {}",
                path.display(),
                backup_path.display(),
                reason
            );
            enforce_corrupted_state_backup_retention(path)?;
            Ok(EngineState::default())
        }
    }
}

fn signer_profile_is_production() -> bool {
    let raw = std::env::var(TBTC_SIGNER_PROFILE_ENV).unwrap_or_default();
    let normalized = raw.trim().to_ascii_lowercase();
    match normalized.as_str() {
        TBTC_SIGNER_PROFILE_PRODUCTION | "" => true,
        TBTC_SIGNER_PROFILE_DEVELOPMENT => false,
        other => panic!(
            "{} must be '{}' or '{}'; got {:?}",
            TBTC_SIGNER_PROFILE_ENV,
            TBTC_SIGNER_PROFILE_PRODUCTION,
            TBTC_SIGNER_PROFILE_DEVELOPMENT,
            other
        ),
    }
}

fn state_key_command_timeout_secs() -> u64 {
    std::env::var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV)
        .ok()
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| {
            *value >= TBTC_SIGNER_MIN_STATE_KEY_COMMAND_TIMEOUT_SECS
                && *value <= TBTC_SIGNER_MAX_STATE_KEY_COMMAND_TIMEOUT_SECS
        })
        .unwrap_or(TBTC_SIGNER_DEFAULT_STATE_KEY_COMMAND_TIMEOUT_SECS)
}

fn decode_state_encryption_key_hex(
    mut raw_key_hex: String,
    source_label: &str,
) -> Result<Zeroizing<[u8; 32]>, EngineError> {
    let key_len = raw_key_hex.trim().len();
    if key_len != 64 {
        raw_key_hex.zeroize();
        return Err(EngineError::Internal(format!(
            "state encryption key from [{}] must be exactly 64 hex chars (32 bytes)",
            source_label
        )));
    }
    let trimmed_key_hex = raw_key_hex.trim().to_string();
    raw_key_hex.zeroize();

    let decode_result = hex::decode(&trimmed_key_hex);
    let mut trimmed_key_hex = trimmed_key_hex;
    trimmed_key_hex.zeroize();
    let mut key_bytes = decode_result.map_err(|_| {
        EngineError::Internal(format!(
            "state encryption key from [{}] must be valid hex",
            source_label
        ))
    })?;

    if key_bytes.len() != 32 {
        key_bytes.zeroize();
        return Err(EngineError::Internal(format!(
            "state encryption key from [{}] must decode to exactly 32 bytes",
            source_label
        )));
    }

    let mut key = [0u8; 32];
    key.copy_from_slice(&key_bytes);
    key_bytes.zeroize();
    Ok(Zeroizing::new(key))
}

fn state_key_identifier(key: &[u8; 32]) -> String {
    format!("sha256:{}", hex::encode(hash_bytes(key)))
}

fn push_aad_field(aad: &mut Vec<u8>, label: &[u8], value: &[u8]) {
    aad.extend_from_slice(&(label.len() as u32).to_be_bytes());
    aad.extend_from_slice(label);
    aad.extend_from_slice(&(value.len() as u32).to_be_bytes());
    aad.extend_from_slice(value);
}

fn encrypted_state_envelope_aad(
    schema_version: u16,
    encryption_algorithm: &str,
    key_provider: &str,
    key_id: &str,
    nonce: &str,
) -> Vec<u8> {
    let mut aad = Vec::new();
    push_aad_field(&mut aad, b"schema_version", &schema_version.to_be_bytes());
    push_aad_field(
        &mut aad,
        b"encryption_algorithm",
        encryption_algorithm.as_bytes(),
    );
    push_aad_field(&mut aad, b"key_provider", key_provider.as_bytes());
    push_aad_field(&mut aad, b"key_id", key_id.as_bytes());
    push_aad_field(&mut aad, b"nonce", nonce.as_bytes());
    aad
}

fn drain_command_pipe<R>(mut pipe: R) -> mpsc::Receiver<std::io::Result<Vec<u8>>>
where
    R: Read + Send + 'static,
{
    let (sender, receiver) = mpsc::channel();
    std::thread::spawn(move || {
        let mut bytes = Vec::new();
        let result = match pipe.read_to_end(&mut bytes) {
            Ok(_) => Ok(bytes),
            Err(err) => {
                bytes.zeroize();
                Err(err)
            }
        };
        if let Err(mpsc::SendError(Ok(mut bytes))) = sender.send(result) {
            bytes.zeroize();
        }
    });
    receiver
}

fn read_command_pipe(
    receiver: mpsc::Receiver<std::io::Result<Vec<u8>>>,
    stream_name: &str,
    timeout: Duration,
) -> Result<Vec<u8>, EngineError> {
    match receiver.recv_timeout(timeout) {
        Ok(Ok(bytes)) => Ok(bytes),
        Ok(Err(e)) => Err(EngineError::Internal(format!(
            "failed to read state key command {stream_name}: {e}"
        ))),
        Err(mpsc::RecvTimeoutError::Timeout) => Err(EngineError::Internal(format!(
            "state key command {stream_name} pipe timed out waiting for EOF"
        ))),
        Err(mpsc::RecvTimeoutError::Disconnected) => Err(EngineError::Internal(format!(
            "state key command {stream_name} reader exited without a result"
        ))),
    }
}

fn zeroize_command_pipe_if_ready(receiver: mpsc::Receiver<std::io::Result<Vec<u8>>>) {
    if let Ok(Ok(mut bytes)) = receiver.try_recv() {
        bytes.zeroize();
    }
}

#[cfg(unix)]
fn configure_state_key_command_process_group(command: &mut std::process::Command) {
    unsafe {
        command.pre_exec(|| {
            if libc::setpgid(0, 0) == 0 {
                Ok(())
            } else {
                Err(std::io::Error::last_os_error())
            }
        });
    }
}

#[cfg(not(unix))]
fn configure_state_key_command_process_group(_command: &mut std::process::Command) {}

#[cfg(unix)]
fn kill_state_key_command_process_group(child_id: u32) {
    let pgid = -(child_id as i32);
    unsafe {
        let _ = libc::kill(pgid, libc::SIGKILL);
    }
}

#[cfg(not(unix))]
fn kill_state_key_command_process_group(_child_id: u32) {}

fn terminate_state_key_command(child: &mut std::process::Child, child_id: u32) {
    kill_state_key_command_process_group(child_id);
    let _ = child.kill();
    let _ = child.wait();
}

fn remaining_timeout(deadline: Instant) -> Duration {
    deadline
        .checked_duration_since(Instant::now())
        .unwrap_or(Duration::ZERO)
}

fn execute_state_key_command(command_spec: &str) -> Result<Output, EngineError> {
    let timeout_secs = state_key_command_timeout_secs();
    let timeout = Duration::from_secs(timeout_secs);
    let deadline = Instant::now() + timeout;
    let mut command = std::process::Command::new("/bin/sh");
    command
        .arg("-c")
        .arg(command_spec)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    configure_state_key_command_process_group(&mut command);

    let mut child = command.spawn().map_err(|e| {
        EngineError::Internal(format!(
            "failed to execute state key command from [{}]: {e}",
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV
        ))
    })?;
    let child_id = child.id();
    let stdout = child.stdout.take().ok_or_else(|| {
        EngineError::Internal("state key command stdout pipe unavailable".to_string())
    })?;
    let stderr = child.stderr.take().ok_or_else(|| {
        EngineError::Internal("state key command stderr pipe unavailable".to_string())
    })?;
    let stdout_receiver = drain_command_pipe(stdout);
    let stderr_receiver = drain_command_pipe(stderr);
    let started_at = Instant::now();

    loop {
        match child.try_wait().map_err(|e| {
            EngineError::Internal(format!(
                "failed while waiting for state key command from [{}]: {e}",
                TBTC_SIGNER_STATE_KEY_COMMAND_ENV
            ))
        })? {
            Some(status) => {
                let stdout_result =
                    read_command_pipe(stdout_receiver, "stdout", remaining_timeout(deadline));
                let stdout = match stdout_result {
                    Ok(stdout) => stdout,
                    Err(err) => {
                        terminate_state_key_command(&mut child, child_id);
                        zeroize_command_pipe_if_ready(stderr_receiver);
                        return Err(err);
                    }
                };
                let stderr_result =
                    read_command_pipe(stderr_receiver, "stderr", remaining_timeout(deadline));
                let stderr = match stderr_result {
                    Ok(stderr) => stderr,
                    Err(err) => {
                        let mut stdout = stdout;
                        stdout.zeroize();
                        terminate_state_key_command(&mut child, child_id);
                        return Err(err);
                    }
                };
                return Ok(Output {
                    status,
                    stdout,
                    stderr,
                });
            }
            None => {
                if started_at.elapsed() >= Duration::from_secs(timeout_secs) {
                    terminate_state_key_command(&mut child, child_id);
                    zeroize_command_pipe_if_ready(stdout_receiver);
                    zeroize_command_pipe_if_ready(stderr_receiver);
                    return Err(EngineError::Internal(format!(
                        "state key command from [{}] timed out after [{}] seconds",
                        TBTC_SIGNER_STATE_KEY_COMMAND_ENV, timeout_secs
                    )));
                }
                std::thread::sleep(Duration::from_millis(25));
            }
        }
    }
}

fn state_encryption_key_material() -> Result<StateEncryptionKeyMaterial, EngineError> {
    let provider = std::env::var(TBTC_SIGNER_STATE_KEY_PROVIDER_ENV)
        .map(|value| value.trim().to_ascii_lowercase())
        .unwrap_or_else(|_| TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT.to_string());

    match provider.as_str() {
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT => {
            if signer_profile_is_production() {
                return Err(EngineError::Internal(format!(
                    "state key provider [{}] is not allowed in profile [{}]; configure [{}]={} with [{}] returning a 32-byte hex key sourced from KMS/HSM",
                    TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
                    TBTC_SIGNER_PROFILE_PRODUCTION,
                    TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
                    TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
                    TBTC_SIGNER_STATE_KEY_COMMAND_ENV
                )));
            }

            let raw_key_hex =
                std::env::var(TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV).map_err(|_| {
                    EngineError::Internal(format!(
                        "missing required state encryption key env [{}]",
                        TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV
                    ))
                })?;
            let key = decode_state_encryption_key_hex(
                raw_key_hex,
                TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
            )?;
            let key_id = state_key_identifier(&key);
            Ok(StateEncryptionKeyMaterial {
                key,
                key_provider: TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
                key_id,
            })
        }
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND => {
            let command_spec = std::env::var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV).map_err(|_| {
                EngineError::Internal(format!(
                    "missing required state key command env [{}]",
                    TBTC_SIGNER_STATE_KEY_COMMAND_ENV
                ))
            })?;
            if command_spec.trim().is_empty() {
                return Err(EngineError::Internal(format!(
                    "state key command env [{}] must be non-empty",
                    TBTC_SIGNER_STATE_KEY_COMMAND_ENV
                )));
            }

            let mut output = execute_state_key_command(&command_spec)?;

            if !output.status.success() {
                output.stdout.zeroize();
                output.stderr.zeroize();
                return Err(EngineError::Internal(format!(
                    "state key command from [{}] exited with non-zero status [{}]",
                    TBTC_SIGNER_STATE_KEY_COMMAND_ENV, output.status
                )));
            }

            let command_stdout_bytes = std::mem::take(&mut output.stdout);
            output.stderr.zeroize();
            let mut command_stdout = String::from_utf8(command_stdout_bytes).map_err(|error| {
                let mut command_stdout_raw = error.into_bytes();
                command_stdout_raw.zeroize();
                EngineError::Internal(format!(
                    "state key command from [{}] must output UTF-8 hex key bytes",
                    TBTC_SIGNER_STATE_KEY_COMMAND_ENV
                ))
            })?;
            let key = decode_state_encryption_key_hex(
                std::mem::take(&mut command_stdout),
                TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
            )?;
            command_stdout.zeroize();
            let key_id = state_key_identifier(&key);
            Ok(StateEncryptionKeyMaterial {
                key,
                key_provider: TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
                key_id,
            })
        }
        _ => Err(EngineError::Internal(format!(
            "unsupported state key provider [{}]; expected [{}] or [{}]",
            provider,
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND
        ))),
    }
}

fn encode_encrypted_state_envelope(
    persisted: &PersistedEngineState,
) -> Result<Zeroizing<Vec<u8>>, EngineError> {
    let mut plaintext = Zeroizing::new(
        serde_json::to_vec(persisted)
            .map_err(|e| EngineError::Internal(format!("failed to encode signer state: {e}")))?,
    );
    let key_material = state_encryption_key_material()?;
    let cipher = XChaCha20Poly1305::new_from_slice(&key_material.key[..]).map_err(|e| {
        EngineError::Internal(format!("failed to initialize state encryption cipher: {e}"))
    })?;

    let mut nonce_bytes = [0u8; TBTC_SIGNER_STATE_ENVELOPE_NONCE_BYTES];
    OsRng.fill_bytes(&mut nonce_bytes);
    let nonce = XNonce::from_slice(&nonce_bytes);
    let nonce_hex = hex::encode(nonce_bytes);
    let schema_version = PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION;
    let encryption_algorithm = TBTC_SIGNER_STATE_ENCRYPTION_ALGORITHM_XCHACHA20POLY1305;
    let key_provider = key_material.key_provider.to_string();
    let key_id = key_material.key_id;
    let aad = encrypted_state_envelope_aad(
        schema_version,
        encryption_algorithm,
        &key_provider,
        &key_id,
        &nonce_hex,
    );

    let mut ciphertext_and_tag = cipher
        .encrypt(
            nonce,
            Payload {
                msg: plaintext.as_ref(),
                aad: &aad,
            },
        )
        .map_err(|e| {
            EngineError::Internal(format!("failed to encrypt signer state payload: {e}"))
        })?;
    plaintext.zeroize();

    if ciphertext_and_tag.len() < TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES {
        ciphertext_and_tag.zeroize();
        return Err(EngineError::Internal(
            "encrypted signer state payload shorter than authentication tag".to_string(),
        ));
    }

    let mut authentication_tag = ciphertext_and_tag
        .split_off(ciphertext_and_tag.len() - TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES);
    let envelope = PersistedEncryptedEngineStateEnvelope {
        schema_version,
        encryption_algorithm: encryption_algorithm.to_string(),
        key_provider,
        key_id,
        nonce: nonce_hex,
        ciphertext: hex::encode(&ciphertext_and_tag),
        authentication_tag: hex::encode(&authentication_tag),
    };
    ciphertext_and_tag.zeroize();
    authentication_tag.zeroize();

    let serialized = serde_json::to_vec(&envelope).map_err(|e| {
        EngineError::Internal(format!(
            "failed to encode encrypted signer state envelope: {e}"
        ))
    })?;
    Ok(Zeroizing::new(serialized))
}

fn decode_encrypted_state_envelope(
    mut envelope: PersistedEncryptedEngineStateEnvelope,
) -> Result<PersistedEngineState, EngineError> {
    if envelope.schema_version != PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION
        && envelope.schema_version != PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION_V2
    {
        return Err(EngineError::Internal(format!(
            "unsupported encrypted signer state schema version: expected [{}] or [{}], got [{}]",
            PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION,
            PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION_V2,
            envelope.schema_version
        )));
    }
    if envelope.encryption_algorithm != TBTC_SIGNER_STATE_ENCRYPTION_ALGORITHM_XCHACHA20POLY1305 {
        return Err(EngineError::Internal(format!(
            "unsupported state encryption algorithm [{}]",
            envelope.encryption_algorithm
        )));
    }
    let envelope_aad = if envelope.schema_version == PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION {
        Some(encrypted_state_envelope_aad(
            envelope.schema_version,
            &envelope.encryption_algorithm,
            &envelope.key_provider,
            &envelope.key_id,
            &envelope.nonce,
        ))
    } else {
        None
    };
    let nonce_decode = hex::decode(&envelope.nonce);
    envelope.nonce.zeroize();
    let mut nonce_bytes = nonce_decode
        .map_err(|_| EngineError::Internal("invalid envelope nonce hex".to_string()))?;
    if nonce_bytes.len() != TBTC_SIGNER_STATE_ENVELOPE_NONCE_BYTES {
        nonce_bytes.zeroize();
        return Err(EngineError::Internal(format!(
            "invalid envelope nonce size: expected [{}], got [{}]",
            TBTC_SIGNER_STATE_ENVELOPE_NONCE_BYTES,
            nonce_bytes.len()
        )));
    }

    let ciphertext_decode = hex::decode(&envelope.ciphertext);
    envelope.ciphertext.zeroize();
    let mut ciphertext = ciphertext_decode
        .map_err(|_| EngineError::Internal("invalid envelope ciphertext hex".to_string()))?;
    let auth_tag_decode = hex::decode(&envelope.authentication_tag);
    envelope.authentication_tag.zeroize();
    let mut authentication_tag = auth_tag_decode.map_err(|_| {
        EngineError::Internal("invalid envelope authentication_tag hex".to_string())
    })?;
    if authentication_tag.len() != TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES {
        ciphertext.zeroize();
        authentication_tag.zeroize();
        nonce_bytes.zeroize();
        return Err(EngineError::Internal(format!(
            "invalid envelope authentication tag size: expected [{}], got [{}]",
            TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES,
            authentication_tag.len()
        )));
    }

    let key_material = state_encryption_key_material()?;
    if envelope.key_provider != key_material.key_provider {
        ciphertext.zeroize();
        authentication_tag.zeroize();
        nonce_bytes.zeroize();
        return Err(EngineError::Internal(format!(
            "state key provider mismatch: envelope [{}], configured [{}]",
            envelope.key_provider, key_material.key_provider
        )));
    }
    let allows_legacy_env_key_id = envelope.schema_version
        == PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION_V2
        && envelope.key_provider == TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT
        && envelope.key_id == TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX;
    if envelope.key_id != key_material.key_id && !allows_legacy_env_key_id {
        ciphertext.zeroize();
        authentication_tag.zeroize();
        nonce_bytes.zeroize();
        return Err(EngineError::Internal(format!(
            "state key identifier mismatch: envelope [{}], configured [{}]",
            envelope.key_id, key_material.key_id
        )));
    }
    let cipher = XChaCha20Poly1305::new_from_slice(&key_material.key[..]).map_err(|e| {
        EngineError::Internal(format!("failed to initialize state encryption cipher: {e}"))
    })?;

    ciphertext.extend_from_slice(&authentication_tag);
    authentication_tag.zeroize();

    let nonce = XNonce::from_slice(&nonce_bytes);
    let decrypted = if let Some(aad) = envelope_aad {
        cipher.decrypt(
            nonce,
            Payload {
                msg: ciphertext.as_ref(),
                aad: &aad,
            },
        )
    } else {
        cipher.decrypt(nonce, ciphertext.as_ref())
    }
    .map_err(|e| EngineError::Internal(format!("failed to decrypt signer state envelope: {e}")))?;
    ciphertext.zeroize();
    nonce_bytes.zeroize();
    let plaintext = Zeroizing::new(decrypted);
    serde_json::from_slice(&plaintext)
        .map_err(|e| EngineError::Internal(format!("failed to decode decrypted signer state: {e}")))
}

fn decode_persisted_state_storage_format(
    bytes: &[u8],
) -> Result<PersistedStateStorageFormat, EngineError> {
    if let Ok(envelope) = serde_json::from_slice::<PersistedEncryptedEngineStateEnvelope>(bytes) {
        let should_rewrite = envelope.schema_version != PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION
            || envelope.key_id == TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX;
        let persisted = decode_encrypted_state_envelope(envelope)?;
        return Ok(PersistedStateStorageFormat::EncryptedEnvelope {
            persisted,
            should_rewrite,
        });
    }

    let persisted = serde_json::from_slice::<PersistedEngineState>(bytes).map_err(|e| {
        EngineError::Internal(format!("failed to decode signer state file payload: {e}"))
    })?;
    Ok(PersistedStateStorageFormat::LegacyPlaintext(persisted))
}

fn load_engine_state_from_storage() -> Result<EngineState, EngineError> {
    let path = active_state_file_path()?;
    if !path.exists() {
        return Ok(EngineState::default());
    }

    let mut bytes = fs::read(&path).map_err(|e| {
        EngineError::Internal(format!(
            "failed to read signer state file [{}]: {e}",
            path.display()
        ))
    })?;
    if bytes.is_empty() {
        eprintln!(
            "warning: signer state file [{}] exists but is empty; initializing with clean state",
            path.display()
        );
        bytes.zeroize();
        return Ok(EngineState::default());
    }

    let decoded_format = decode_persisted_state_storage_format(&bytes);
    bytes.zeroize();
    let (persisted, should_rewrite_state): (PersistedEngineState, bool) = match decoded_format {
        Ok(PersistedStateStorageFormat::EncryptedEnvelope {
            persisted,
            should_rewrite,
        }) => (persisted, should_rewrite),
        Ok(PersistedStateStorageFormat::LegacyPlaintext(persisted)) => (persisted, true),
        Err(e) => {
            return recover_or_fail_from_corrupted_state_file(
                &path,
                format!(
                    "failed to decode signer state file [{}]: {e}",
                    path.display()
                ),
            )
        }
    };

    let engine_state: EngineState = persisted.try_into().or_else(|e| {
        recover_or_fail_from_corrupted_state_file(
            &path,
            format!(
                "failed to validate signer state file [{}]: {e}",
                path.display()
            ),
        )
    })?;

    if should_rewrite_state && path.exists() {
        persist_engine_state_to_storage(&engine_state).map_err(|e| {
            EngineError::Internal(format!(
                "loaded legacy signer state file [{}] but failed to migrate to current encrypted envelope: {e}",
                path.display()
            ))
        })?;
    }

    Ok(engine_state)
}

#[cfg(test)]
fn persist_fault_injection_label(point: PersistFaultInjectionPoint) -> &'static str {
    match point {
        PersistFaultInjectionPoint::AfterTempSyncBeforeRename => "after_temp_sync_before_rename",
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync => {
            "after_rename_before_directory_sync"
        }
    }
}

fn maybe_inject_persist_fault(point: PersistFaultInjectionPoint) -> Result<(), EngineError> {
    #[cfg(test)]
    {
        let slot = PERSIST_FAULT_INJECTION_POINT.get_or_init(|| Mutex::new(None));
        let configured_point = *slot.lock().map_err(|_| {
            EngineError::Internal("persist fault injection mutex poisoned".to_string())
        })?;
        if configured_point == Some(point) {
            return Err(EngineError::Internal(format!(
                "injected persist fault at [{}]",
                persist_fault_injection_label(point)
            )));
        }
    }

    #[cfg(not(test))]
    let _ = point;

    Ok(())
}

#[cfg(test)]
fn set_persist_fault_injection_for_tests(point: PersistFaultInjectionPoint) {
    if let Ok(mut slot) = PERSIST_FAULT_INJECTION_POINT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        *slot = Some(point);
    }
}

#[cfg(test)]
fn clear_persist_fault_injection_for_tests() {
    if let Ok(mut slot) = PERSIST_FAULT_INJECTION_POINT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        *slot = None;
    }
}

fn persist_engine_state_to_storage(engine_state: &EngineState) -> Result<(), EngineError> {
    let path = active_state_file_path()?;
    let persisted: PersistedEngineState = engine_state.try_into()?;
    let mut bytes = encode_encrypted_state_envelope(&persisted)?;
    drop(persisted);
    let temp_path = path.with_extension(format!("tmp-{}", std::process::id()));
    let persist_result = (|| -> Result<(), EngineError> {
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| {
                EngineError::Internal(format!(
                    "failed to create signer state directory [{}]: {e}",
                    parent.display()
                ))
            })?;
        }

        {
            let mut temp_file = {
                let mut options = fs::OpenOptions::new();
                options.create(true).truncate(true).write(true);
                #[cfg(unix)]
                options.mode(0o600);
                options.open(&temp_path).map_err(|e| {
                    EngineError::Internal(format!(
                        "failed to open signer state temp file [{}]: {e}",
                        temp_path.display()
                    ))
                })?
            };
            temp_file.write_all(bytes.as_ref()).map_err(|e| {
                EngineError::Internal(format!(
                    "failed to write signer state temp file [{}]: {e}",
                    temp_path.display()
                ))
            })?;
            temp_file.sync_all().map_err(|e| {
                EngineError::Internal(format!(
                    "failed to sync signer state temp file [{}]: {e}",
                    temp_path.display()
                ))
            })?;
        }
        maybe_inject_persist_fault(PersistFaultInjectionPoint::AfterTempSyncBeforeRename)?;

        fs::rename(&temp_path, &path).map_err(|e| {
            EngineError::Internal(format!(
                "failed to move signer state temp file [{}] to [{}]: {e}",
                temp_path.display(),
                path.display()
            ))
        })?;
        maybe_inject_persist_fault(PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync)?;

        if let Some(parent) = path.parent() {
            let directory = fs::File::open(parent).map_err(|e| {
                EngineError::Internal(format!(
                    "failed to open signer state directory [{}] for sync: {e}",
                    parent.display()
                ))
            })?;
            directory.sync_all().map_err(|e| {
                EngineError::Internal(format!(
                    "failed to sync signer state directory [{}]: {e}",
                    parent.display()
                ))
            })?;
        }

        Ok(())
    })();

    if persist_result.is_err() {
        let _ = fs::remove_file(&temp_path);
    }

    bytes.zeroize();
    persist_result
}

impl TryFrom<PersistedEngineState> for EngineState {
    type Error = EngineError;

    fn try_from(persisted: PersistedEngineState) -> Result<Self, Self::Error> {
        if persisted.schema_version != PERSISTED_STATE_SCHEMA_VERSION {
            return Err(EngineError::Internal(format!(
                "unsupported signer state schema version: expected [{}], got [{}]",
                PERSISTED_STATE_SCHEMA_VERSION, persisted.schema_version
            )));
        }

        let mut sessions = HashMap::new();
        for (session_id, session_state) in persisted.sessions {
            sessions.insert(session_id, session_state.try_into()?);
        }
        ensure_session_registry_persisted_bound(sessions.len())?;
        let mut quarantined_operator_identifiers = HashSet::new();
        for operator_identifier in persisted.quarantined_operator_identifiers {
            if operator_identifier == 0 {
                return Err(EngineError::Internal(
                    "persisted quarantined operator identifier must be non-zero".to_string(),
                ));
            }
            if !quarantined_operator_identifiers.insert(operator_identifier) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted quarantined operator identifier [{}]",
                    operator_identifier
                )));
            }
        }
        for operator_identifier in persisted.operator_fault_scores.keys() {
            if *operator_identifier == 0 {
                return Err(EngineError::Internal(
                    "persisted operator fault score identifier must be non-zero".to_string(),
                ));
            }
        }
        let canary_rollout = persisted.canary_rollout;
        if !matches!(canary_rollout.current_percent, 10 | 50 | 100) {
            return Err(EngineError::Internal(format!(
                "persisted canary current_percent [{}] must be one of [10, 50, 100]",
                canary_rollout.current_percent
            )));
        }
        if !matches!(canary_rollout.previous_percent, 10 | 50 | 100) {
            return Err(EngineError::Internal(format!(
                "persisted canary previous_percent [{}] must be one of [10, 50, 100]",
                canary_rollout.previous_percent
            )));
        }
        if canary_rollout.config_version == 0 {
            return Err(EngineError::Internal(
                "persisted canary config_version must be positive".to_string(),
            ));
        }

        Ok(EngineState {
            sessions,
            refresh_epoch_counter: persisted.refresh_epoch_counter,
            operator_fault_scores: persisted.operator_fault_scores,
            quarantined_operator_identifiers,
            canary_rollout,
        })
    }
}

impl TryFrom<&EngineState> for PersistedEngineState {
    type Error = EngineError;

    fn try_from(engine_state: &EngineState) -> Result<Self, Self::Error> {
        ensure_session_registry_persisted_bound(engine_state.sessions.len())?;
        let mut sessions = HashMap::new();
        for (session_id, session_state) in &engine_state.sessions {
            sessions.insert(session_id.clone(), session_state.try_into()?);
        }
        let mut quarantined_operator_identifiers = engine_state
            .quarantined_operator_identifiers
            .iter()
            .copied()
            .collect::<Vec<_>>();
        quarantined_operator_identifiers.sort_unstable();

        Ok(PersistedEngineState {
            schema_version: PERSISTED_STATE_SCHEMA_VERSION,
            sessions,
            refresh_epoch_counter: engine_state.refresh_epoch_counter,
            operator_fault_scores: engine_state.operator_fault_scores.clone(),
            quarantined_operator_identifiers,
            canary_rollout: engine_state.canary_rollout.clone(),
        })
    }
}

impl TryFrom<PersistedSessionState> for SessionState {
    type Error = EngineError;

    fn try_from(persisted: PersistedSessionState) -> Result<Self, Self::Error> {
        let dkg_key_packages = persisted
            .dkg_key_packages
            .map(|persisted_key_packages| {
                let mut key_packages = BTreeMap::new();

                for persisted_key_package in persisted_key_packages {
                    let identifier = persisted_key_package.identifier;
                    if identifier == 0 {
                        return Err(EngineError::Internal(
                            "persisted key package identifier must be non-zero".to_string(),
                        ));
                    }

                    let key_package_bytes_result =
                        hex::decode(persisted_key_package.key_package_hex.as_str());
                    let mut key_package_bytes = key_package_bytes_result.map_err(|_| {
                        EngineError::Internal(format!(
                            "failed to decode persisted key package for identifier [{}]",
                            identifier
                        ))
                    })?;
                    let key_package_result =
                        frost::keys::KeyPackage::deserialize(&key_package_bytes);
                    key_package_bytes.zeroize();
                    let key_package = key_package_result.map_err(|e| {
                        EngineError::Internal(format!(
                            "failed to deserialize persisted key package for identifier [{}]: {e}",
                            identifier
                        ))
                    })?;

                    if key_packages.insert(identifier, key_package).is_some() {
                        return Err(EngineError::Internal(format!(
                            "duplicate persisted key package identifier [{}]",
                            identifier
                        )));
                    }
                }

                Ok(key_packages)
            })
            .transpose()?;

        let dkg_public_key_package = persisted
            .dkg_public_key_package_hex
            .map(|mut public_key_package_hex| {
                let public_key_package_bytes_result = hex::decode(&public_key_package_hex);
                public_key_package_hex.zeroize();
                let mut public_key_package_bytes =
                    public_key_package_bytes_result.map_err(|_| {
                        EngineError::Internal(
                            "failed to decode persisted DKG public key package".to_string(),
                        )
                    })?;
                let public_key_package_result =
                    frost::keys::PublicKeyPackage::deserialize(&public_key_package_bytes);
                public_key_package_bytes.zeroize();
                public_key_package_result.map_err(|e| {
                    EngineError::Internal(format!(
                        "failed to deserialize persisted DKG public key package: {e}"
                    ))
                })
            })
            .transpose()?;

        let sign_message_bytes = persisted
            .sign_message_hex
            .map(|message_hex| {
                let mut sign_message_bytes = hex::decode(message_hex.as_str()).map_err(|_| {
                    EngineError::Internal("failed to decode persisted sign message".to_string())
                })?;
                let secret = Zeroizing::new(std::mem::take(&mut sign_message_bytes));
                sign_message_bytes.zeroize();
                Ok(secret)
            })
            .transpose()?;

        let mut consumed_attempt_ids = HashSet::new();
        for attempt_id in persisted.consumed_attempt_ids {
            if attempt_id.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed attempt ID must be non-empty".to_string(),
                ));
            }

            if !consumed_attempt_ids.insert(attempt_id.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed attempt ID [{}]",
                    attempt_id
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_attempt_ids.len(),
            "consumed_attempt_ids",
        )?;

        let mut consumed_sign_round_ids = HashSet::new();
        for round_id in persisted.consumed_sign_round_ids {
            if round_id.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed sign round ID must be non-empty".to_string(),
                ));
            }

            if !consumed_sign_round_ids.insert(round_id.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed sign round ID [{}]",
                    round_id
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_sign_round_ids.len(),
            "consumed_sign_round_ids",
        )?;

        let mut consumed_finalize_round_ids = HashSet::new();
        for round_id in persisted.consumed_finalize_round_ids {
            if round_id.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed finalize round ID must be non-empty".to_string(),
                ));
            }

            if !consumed_finalize_round_ids.insert(round_id.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed finalize round ID [{}]",
                    round_id
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_finalize_round_ids.len(),
            "consumed_finalize_round_ids",
        )?;

        let mut consumed_finalize_request_fingerprints = HashSet::new();
        for request_fingerprint in persisted.consumed_finalize_request_fingerprints {
            if request_fingerprint.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed finalize request fingerprint must be non-empty".to_string(),
                ));
            }

            if !consumed_finalize_request_fingerprints.insert(request_fingerprint.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed finalize request fingerprint [{}]",
                    request_fingerprint
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_finalize_request_fingerprints.len(),
            "consumed_finalize_request_fingerprints",
        )?;
        if persisted.attempt_transition_records.len()
            > TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION
        {
            return Err(EngineError::Internal(format!(
                "persisted attempt_transition_records size [{}] exceeds max [{}]",
                persisted.attempt_transition_records.len(),
                TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION
            )));
        }
        let mut last_refresh_epoch = 0_u64;
        for refresh_record in &persisted.refresh_history {
            if refresh_record.refresh_epoch == 0 {
                return Err(EngineError::Internal(
                    "persisted refresh_history refresh_epoch must be positive".to_string(),
                ));
            }
            if refresh_record.refresh_epoch <= last_refresh_epoch {
                return Err(EngineError::Internal(
                    "persisted refresh_history refresh_epoch must be strictly increasing"
                        .to_string(),
                ));
            }
            last_refresh_epoch = refresh_record.refresh_epoch;
        }
        if let Some(emergency_rekey_event) = persisted.emergency_rekey_event.as_ref() {
            if emergency_rekey_event.reason.trim().is_empty() {
                return Err(EngineError::Internal(
                    "persisted emergency_rekey_event reason must be non-empty".to_string(),
                ));
            }
        }

        Ok(SessionState {
            dkg_request_fingerprint: persisted.dkg_request_fingerprint,
            dkg_key_packages,
            dkg_public_key_package,
            dkg_result: persisted.dkg_result,
            sign_request_fingerprint: persisted.sign_request_fingerprint,
            sign_message_bytes,
            round_state: persisted.round_state,
            active_attempt_context: persisted.active_attempt_context,
            attempt_transition_records: persisted.attempt_transition_records,
            consumed_attempt_ids,
            consumed_sign_round_ids,
            finalize_request_fingerprint: persisted.finalize_request_fingerprint,
            signature_result: persisted.signature_result,
            consumed_finalize_round_ids,
            consumed_finalize_request_fingerprints,
            build_tx_request_fingerprint: persisted.build_tx_request_fingerprint,
            tx_result: persisted.tx_result,
            refresh_request_fingerprint: persisted.refresh_request_fingerprint,
            refresh_result: persisted.refresh_result,
            refresh_history: persisted.refresh_history,
            emergency_rekey_event: persisted.emergency_rekey_event,
        })
    }
}

impl TryFrom<&SessionState> for PersistedSessionState {
    type Error = EngineError;

    fn try_from(session_state: &SessionState) -> Result<Self, Self::Error> {
        let dkg_key_packages = session_state
            .dkg_key_packages
            .as_ref()
            .map(|key_packages| {
                key_packages
                    .iter()
                    .map(|(identifier, key_package)| {
                        let mut key_package_bytes = key_package.serialize().map_err(|e| {
                            EngineError::Internal(format!(
                                "failed to serialize DKG key package for identifier [{}]: {e}",
                                identifier
                            ))
                        })?;
                        let key_package_hex = Zeroizing::new(hex::encode(&key_package_bytes));
                        key_package_bytes.zeroize();
                        Ok(PersistedKeyPackage {
                            identifier: *identifier,
                            key_package_hex,
                        })
                    })
                    .collect::<Result<Vec<_>, _>>()
            })
            .transpose()?;

        let dkg_public_key_package_hex = session_state
            .dkg_public_key_package
            .as_ref()
            .map(|public_key_package| {
                let mut public_key_package_bytes = public_key_package.serialize().map_err(|e| {
                    EngineError::Internal(format!(
                        "failed to serialize DKG public key package: {e}"
                    ))
                })?;
                let public_key_package_hex = hex::encode(&public_key_package_bytes);
                public_key_package_bytes.zeroize();
                Ok(public_key_package_hex)
            })
            .transpose()?;

        let sign_message_hex = session_state
            .sign_message_bytes
            .as_ref()
            .map(|sign_message_bytes| Zeroizing::new(hex::encode(sign_message_bytes.as_slice())));
        ensure_consumed_registry_persisted_bound(
            session_state.consumed_attempt_ids.len(),
            "consumed_attempt_ids",
        )?;
        ensure_consumed_registry_persisted_bound(
            session_state.consumed_sign_round_ids.len(),
            "consumed_sign_round_ids",
        )?;
        ensure_consumed_registry_persisted_bound(
            session_state.consumed_finalize_round_ids.len(),
            "consumed_finalize_round_ids",
        )?;
        ensure_consumed_registry_persisted_bound(
            session_state.consumed_finalize_request_fingerprints.len(),
            "consumed_finalize_request_fingerprints",
        )?;
        if session_state.attempt_transition_records.len()
            > TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION
        {
            return Err(EngineError::Internal(format!(
                "attempt_transition_records size [{}] exceeds max [{}]",
                session_state.attempt_transition_records.len(),
                TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION
            )));
        }
        let mut consumed_attempt_ids = session_state
            .consumed_attempt_ids
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_attempt_ids.sort_unstable();
        let mut consumed_sign_round_ids = session_state
            .consumed_sign_round_ids
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_sign_round_ids.sort_unstable();
        let mut consumed_finalize_round_ids = session_state
            .consumed_finalize_round_ids
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_finalize_round_ids.sort_unstable();
        let mut consumed_finalize_request_fingerprints = session_state
            .consumed_finalize_request_fingerprints
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_finalize_request_fingerprints.sort_unstable();

        Ok(PersistedSessionState {
            dkg_request_fingerprint: session_state.dkg_request_fingerprint.clone(),
            dkg_key_packages,
            dkg_public_key_package_hex,
            dkg_result: session_state.dkg_result.clone(),
            sign_request_fingerprint: session_state.sign_request_fingerprint.clone(),
            sign_message_hex,
            round_state: session_state.round_state.clone(),
            active_attempt_context: session_state.active_attempt_context.clone(),
            attempt_transition_records: session_state.attempt_transition_records.clone(),
            consumed_attempt_ids,
            consumed_sign_round_ids,
            finalize_request_fingerprint: session_state.finalize_request_fingerprint.clone(),
            signature_result: session_state.signature_result.clone(),
            consumed_finalize_round_ids,
            consumed_finalize_request_fingerprints,
            build_tx_request_fingerprint: session_state.build_tx_request_fingerprint.clone(),
            tx_result: session_state.tx_result.clone(),
            refresh_request_fingerprint: session_state.refresh_request_fingerprint.clone(),
            refresh_result: session_state.refresh_result.clone(),
            refresh_history: session_state.refresh_history.clone(),
            emergency_rekey_event: session_state.emergency_rekey_event.clone(),
        })
    }
}

fn now_unix() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}

fn hash_hex(bytes: &[u8]) -> String {
    hex::encode(hash_bytes(bytes))
}

fn hash_bytes(bytes: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(bytes);
    let digest = hasher.finalize();

    let mut output = [0u8; 32];
    output.copy_from_slice(&digest);
    output
}

fn deterministic_seed(parts: &[&[u8]]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    for part in parts {
        // Length-prefix each part so embedded 0x00 bytes cannot blur boundaries.
        hasher.update((part.len() as u64).to_le_bytes());
        hasher.update(part);
    }

    let digest = hasher.finalize();
    let mut output = [0u8; 32];
    output.copy_from_slice(&digest);
    output
}

fn ensure_consumed_registry_persisted_bound(
    registry_len: usize,
    registry_name: &str,
) -> Result<(), EngineError> {
    if registry_len > TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
        return Err(EngineError::Internal(format!(
            "persisted {registry_name} registry size [{registry_len}] exceeds max [{}]",
            TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION
        )));
    }

    Ok(())
}

fn ensure_session_registry_persisted_bound(session_count: usize) -> Result<(), EngineError> {
    let max_sessions = max_sessions_limit();
    if session_count > max_sessions {
        return Err(EngineError::Internal(format!(
            "persisted session registry size [{session_count}] exceeds max [{max_sessions}]"
        )));
    }

    Ok(())
}

fn ensure_session_insert_capacity(
    sessions: &HashMap<String, SessionState>,
    session_id: &str,
) -> Result<(), EngineError> {
    if sessions.contains_key(session_id) {
        return Ok(());
    }

    let max_sessions = max_sessions_limit();
    if sessions.len() >= max_sessions {
        return Err(EngineError::Internal(format!(
            "session registry size [{}] reached max [{max_sessions}]; use an existing session_id or increase {}",
            sessions.len(),
            TBTC_SIGNER_MAX_SESSIONS_ENV
        )));
    }

    Ok(())
}

fn ensure_consumed_registry_insert_capacity(
    registry: &HashSet<String>,
    entry: &str,
    registry_name: &str,
    session_id: &str,
) -> Result<(), EngineError> {
    if !registry.contains(entry)
        && registry.len() >= TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION
    {
        return Err(EngineError::Internal(format!(
            "{registry_name} registry size [{}] reached max [{}] for session [{}]; use a new session_id",
            registry.len(),
            TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION,
            session_id
        )));
    }

    Ok(())
}

fn ensure_attempt_transition_record_insert_capacity(
    records: &[TranscriptAuditRecord],
    session_id: &str,
) -> Result<(), EngineError> {
    if records.len() >= TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION {
        return Err(EngineError::Internal(format!(
            "attempt_transition_records size [{}] reached max [{}] for session [{}]; use a new session_id",
            records.len(),
            TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION,
            session_id
        )));
    }

    Ok(())
}

fn participant_identifier_to_frost_identifier(
    participant_identifier: u16,
) -> Result<frost::Identifier, EngineError> {
    participant_identifier.try_into().map_err(|e| {
        EngineError::Validation(format!(
            "invalid participant identifier [{}]: {e}",
            participant_identifier
        ))
    })
}

fn frost_identifier_to_go_string(identifier: frost::Identifier) -> String {
    serde_json::to_string(&hex::encode(identifier.serialize()))
        .expect("serializing hex identifier as JSON string cannot fail")
}

fn parse_frost_identifier(
    operation: &str,
    field_name: &str,
    raw_identifier: &str,
) -> Result<frost::Identifier, EngineError> {
    if raw_identifier.trim().is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: {field_name} is empty"
        )));
    }

    let trimmed = raw_identifier.trim();
    let normalized_hex = if trimmed.starts_with('"') {
        serde_json::from_str::<String>(trimmed).map_err(|e| {
            EngineError::Validation(format!(
                "{operation}: {field_name} must be a JSON string-wrapped hex identifier: {e}"
            ))
        })?
    } else {
        trimmed.to_string()
    };

    let bytes = hex::decode(&normalized_hex).map_err(|_| {
        EngineError::Validation(format!(
            "{operation}: {field_name} must be a hex-encoded FROST identifier"
        ))
    })?;

    frost::Identifier::deserialize(&bytes)
        .map_err(|e| EngineError::Validation(format!("{operation}: invalid {field_name}: {e}")))
}

fn decode_hex_field(
    operation: &str,
    field_name: &str,
    value: &str,
) -> Result<Vec<u8>, EngineError> {
    if value.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: {field_name} is empty"
        )));
    }

    hex::decode(value).map_err(|_| {
        EngineError::Validation(format!("{operation}: {field_name} must be valid hex"))
    })
}

fn zeroizing_rng_from_os() -> ZeroizingChaCha20Rng {
    let mut seed = [0u8; 32];
    OsRng.fill_bytes(&mut seed);
    let rng = ZeroizingChaCha20Rng::from_seed(seed);
    seed.zeroize();
    rng
}

fn decode_round1_package_map(
    operation: &str,
    packages: &[DkgRound1Package],
) -> Result<BTreeMap<frost::Identifier, frost::keys::dkg::round1::Package>, EngineError> {
    if packages.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: round1_packages must not be empty"
        )));
    }

    let mut package_map = BTreeMap::new();
    for (index, package) in packages.iter().enumerate() {
        let identifier = parse_frost_identifier(
            operation,
            &format!("round1_packages[{index}].identifier"),
            &package.identifier,
        )?;
        let package_bytes = decode_hex_field(
            operation,
            &format!("round1_packages[{index}].package_hex"),
            &package.package_hex,
        )?;
        let round1_package = frost::keys::dkg::round1::Package::deserialize(&package_bytes)
            .map_err(|e| {
                EngineError::Validation(format!(
                    "{operation}: invalid round1 package [{index}]: {e}"
                ))
            })?;

        if package_map.insert(identifier, round1_package).is_some() {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate round1 package identifier [{}]",
                package.identifier
            )));
        }
    }

    Ok(package_map)
}

fn decode_round2_package_map(
    operation: &str,
    packages: &[DkgRound2Package],
    expected_recipient: Option<frost::Identifier>,
) -> Result<BTreeMap<frost::Identifier, frost::keys::dkg::round2::Package>, EngineError> {
    if packages.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: round2_packages must not be empty"
        )));
    }

    let mut package_map = BTreeMap::new();
    for (index, package) in packages.iter().enumerate() {
        let recipient_identifier = parse_frost_identifier(
            operation,
            &format!("round2_packages[{index}].identifier"),
            &package.identifier,
        )?;
        if let Some(expected_recipient) = expected_recipient {
            if recipient_identifier != expected_recipient {
                return Err(EngineError::Validation(format!(
                    "{operation}: round2 package [{index}] recipient identifier does not match local DKG participant"
                )));
            }
        }

        let sender_identifier = package.sender_identifier.as_ref().ok_or_else(|| {
            EngineError::Validation(format!(
                "{operation}: round2_packages[{index}].sender_identifier is empty"
            ))
        })?;
        let sender_identifier = parse_frost_identifier(
            operation,
            &format!("round2_packages[{index}].sender_identifier"),
            sender_identifier,
        )?;
        let mut package_bytes = decode_hex_field(
            operation,
            &format!("round2_packages[{index}].package_hex"),
            &package.package_hex,
        )?;
        let round2_package_result = frost::keys::dkg::round2::Package::deserialize(&package_bytes);
        package_bytes.zeroize();
        let round2_package = round2_package_result.map_err(|e| {
            EngineError::Validation(format!(
                "{operation}: invalid round2 package [{index}]: {e}"
            ))
        })?;

        if package_map
            .insert(sender_identifier, round2_package)
            .is_some()
        {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate round2 package sender identifier"
            )));
        }
    }

    Ok(package_map)
}

fn x_only_verifying_key_hex(
    public_key_package: &frost::keys::PublicKeyPackage,
) -> Result<String, EngineError> {
    let compressed = public_key_package
        .verifying_key()
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize verifying key: {e}")))?;

    if compressed.len() != 33 || compressed[0] != 0x02 {
        return Err(EngineError::Internal(
            "expected even-Y compressed FROST verifying key".to_string(),
        ));
    }

    Ok(hex::encode(&compressed[1..]))
}

fn native_public_key_package_from_frost(
    public_key_package: &frost::keys::PublicKeyPackage,
) -> Result<NativeFrostPublicKeyPackage, EngineError> {
    let mut verifying_shares = BTreeMap::new();
    for (identifier, verifying_share) in public_key_package.verifying_shares() {
        let share_bytes = verifying_share.serialize().map_err(|e| {
            EngineError::Internal(format!("failed to serialize verifying share: {e}"))
        })?;
        verifying_shares.insert(
            frost_identifier_to_go_string(*identifier),
            hex::encode(share_bytes),
        );
    }

    Ok(NativeFrostPublicKeyPackage {
        verifying_shares,
        verifying_key: x_only_verifying_key_hex(public_key_package)?,
    })
}

fn native_public_key_package_to_frost(
    operation: &str,
    public_key_package: &NativeFrostPublicKeyPackage,
) -> Result<frost::keys::PublicKeyPackage, EngineError> {
    if public_key_package.verifying_key.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: public_key_package.verifying_key is empty"
        )));
    }
    if public_key_package.verifying_shares.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: public_key_package.verifying_shares is empty"
        )));
    }

    let mut verifying_key_bytes = decode_hex_field(
        operation,
        "public_key_package.verifying_key",
        &public_key_package.verifying_key,
    )?;
    if verifying_key_bytes.len() != 32 {
        verifying_key_bytes.zeroize();
        return Err(EngineError::Validation(format!(
            "{operation}: public_key_package.verifying_key must be a 32-byte x-only key"
        )));
    }

    let mut compressed_verifying_key = Vec::with_capacity(33);
    compressed_verifying_key.push(0x02);
    compressed_verifying_key.extend_from_slice(&verifying_key_bytes);
    verifying_key_bytes.zeroize();
    let verifying_key =
        frost::VerifyingKey::deserialize(&compressed_verifying_key).map_err(|e| {
            EngineError::Validation(format!(
                "{operation}: invalid public_key_package.verifying_key: {e}"
            ))
        })?;
    compressed_verifying_key.zeroize();

    let mut verifying_shares = BTreeMap::new();
    for (identifier, share_hex) in &public_key_package.verifying_shares {
        let identifier = parse_frost_identifier(
            operation,
            "public_key_package.verifying_shares identifier",
            identifier,
        )?;
        let share_bytes = decode_hex_field(
            operation,
            "public_key_package.verifying_shares value",
            share_hex,
        )?;
        let verifying_share =
            frost::keys::VerifyingShare::deserialize(&share_bytes).map_err(|e| {
                EngineError::Validation(format!(
                    "{operation}: invalid public_key_package verifying share: {e}"
                ))
            })?;
        if verifying_shares
            .insert(identifier, verifying_share)
            .is_some()
        {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate public_key_package verifying share identifier"
            )));
        }
    }

    Ok(frost::keys::PublicKeyPackage::new(
        verifying_shares,
        verifying_key,
        None,
    ))
}

fn decode_key_package(
    operation: &str,
    key_package_identifier: &str,
    key_package_hex: &str,
) -> Result<frost::keys::KeyPackage, EngineError> {
    let expected_identifier =
        parse_frost_identifier(operation, "key_package_identifier", key_package_identifier)?;
    let mut key_package_bytes = decode_hex_field(operation, "key_package_hex", key_package_hex)?;
    let key_package_result = frost::keys::KeyPackage::deserialize(&key_package_bytes);
    key_package_bytes.zeroize();
    let key_package = key_package_result
        .map_err(|e| EngineError::Validation(format!("{operation}: invalid key package: {e}")))?;

    if *key_package.identifier() != expected_identifier {
        return Err(EngineError::Validation(format!(
            "{operation}: key_package_identifier does not match serialized key package"
        )));
    }

    Ok(key_package)
}

fn decode_signing_commitment_map(
    operation: &str,
    commitments: &[NativeFrostCommitment],
) -> Result<BTreeMap<frost::Identifier, frost::round1::SigningCommitments>, EngineError> {
    if commitments.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: commitments must not be empty"
        )));
    }

    let mut commitment_map = BTreeMap::new();
    for (index, commitment) in commitments.iter().enumerate() {
        let identifier = parse_frost_identifier(
            operation,
            &format!("commitments[{index}].identifier"),
            &commitment.identifier,
        )?;
        let commitment_bytes = decode_hex_field(
            operation,
            &format!("commitments[{index}].data_hex"),
            &commitment.data_hex,
        )?;
        let signing_commitment = frost::round1::SigningCommitments::deserialize(&commitment_bytes)
            .map_err(|e| {
                EngineError::Validation(format!(
                    "{operation}: invalid signing commitment [{index}]: {e}"
                ))
            })?;
        if commitment_map
            .insert(identifier, signing_commitment)
            .is_some()
        {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate commitment identifier [{}]",
                commitment.identifier
            )));
        }
    }

    Ok(commitment_map)
}

fn decode_signature_share_map(
    operation: &str,
    signature_shares: &[NativeFrostSignatureShare],
) -> Result<BTreeMap<frost::Identifier, frost::round2::SignatureShare>, EngineError> {
    if signature_shares.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: signature_shares must not be empty"
        )));
    }

    let mut signature_share_map = BTreeMap::new();
    for (index, signature_share) in signature_shares.iter().enumerate() {
        let identifier = parse_frost_identifier(
            operation,
            &format!("signature_shares[{index}].identifier"),
            &signature_share.identifier,
        )?;
        let mut signature_share_bytes = decode_hex_field(
            operation,
            &format!("signature_shares[{index}].data_hex"),
            &signature_share.data_hex,
        )?;
        let signature_share = frost::round2::SignatureShare::deserialize(&signature_share_bytes)
            .map_err(|e| {
                EngineError::Validation(format!(
                    "{operation}: invalid signature share [{index}]: {e}"
                ))
            })?;
        signature_share_bytes.zeroize();
        if signature_share_map
            .insert(identifier, signature_share)
            .is_some()
        {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate signature share identifier"
            )));
        }
    }

    Ok(signature_share_map)
}

pub fn dkg_part1(request: DkgPart1Request) -> Result<DkgPart1Result, EngineError> {
    enforce_provenance_gate()?;

    if request.max_signers == 0 {
        return Err(EngineError::Validation(
            "DKGPart1: max_signers is zero".to_string(),
        ));
    }
    if request.min_signers == 0 {
        return Err(EngineError::Validation(
            "DKGPart1: min_signers is zero".to_string(),
        ));
    }
    if request.min_signers > request.max_signers {
        return Err(EngineError::Validation(
            "DKGPart1: min_signers exceeds max_signers".to_string(),
        ));
    }

    let identifier = parse_frost_identifier(
        "DKGPart1",
        "participant_identifier",
        &request.participant_identifier,
    )?;
    let rng = zeroizing_rng_from_os();
    let (mut secret_package, package) =
        frost::keys::dkg::part1(identifier, request.max_signers, request.min_signers, rng)
            .map_err(|e| EngineError::Validation(format!("DKGPart1 failed: {e}")))?;

    let package_bytes = match package.serialize() {
        Ok(package_bytes) => package_bytes,
        Err(err) => {
            secret_package.zeroize();
            return Err(EngineError::Internal(format!(
                "failed to serialize DKG part1 package: {err}"
            )));
        }
    };
    let secret_package_bytes_result = secret_package.serialize();
    secret_package.zeroize();
    let mut secret_package_bytes = secret_package_bytes_result
        .map_err(|e| EngineError::Internal(format!("failed to serialize DKG part1 secret: {e}")))?;

    let result = DkgPart1Result {
        secret_package_hex: hex::encode(&secret_package_bytes),
        package: DkgRound1Package {
            identifier: frost_identifier_to_go_string(identifier),
            package_hex: hex::encode(package_bytes),
        },
    };
    secret_package_bytes.zeroize();

    Ok(result)
}

pub fn dkg_part2(request: DkgPart2Request) -> Result<DkgPart2Result, EngineError> {
    enforce_provenance_gate()?;

    let mut secret_package_bytes = decode_hex_field(
        "DKGPart2",
        "secret_package_hex",
        &request.secret_package_hex,
    )?;
    let secret_package_result =
        frost::keys::dkg::round1::SecretPackage::deserialize(&secret_package_bytes);
    secret_package_bytes.zeroize();
    let mut secret_package = secret_package_result
        .map_err(|e| EngineError::Validation(format!("DKGPart2: invalid secret package: {e}")))?;

    let round1_packages = match decode_round1_package_map("DKGPart2", &request.round1_packages) {
        Ok(round1_packages) => round1_packages,
        Err(err) => {
            secret_package.zeroize();
            return Err(err);
        }
    };
    let (mut round2_secret_package, round2_packages) =
        frost::keys::dkg::part2(secret_package, &round1_packages)
            .map_err(|e| EngineError::Validation(format!("DKGPart2 failed: {e}")))?;

    let mut packages = Vec::with_capacity(round2_packages.len());
    for (identifier, package) in round2_packages {
        let mut package_bytes = match package.serialize() {
            Ok(package_bytes) => package_bytes,
            Err(err) => {
                round2_secret_package.zeroize();
                return Err(EngineError::Internal(format!(
                    "failed to serialize DKG part2 package: {err}"
                )));
            }
        };
        packages.push(DkgRound2Package {
            identifier: frost_identifier_to_go_string(identifier),
            sender_identifier: None,
            package_hex: hex::encode(&package_bytes),
        });
        package_bytes.zeroize();
    }

    let round2_secret_package_bytes_result = round2_secret_package.serialize();
    round2_secret_package.zeroize();
    let mut round2_secret_package_bytes = round2_secret_package_bytes_result
        .map_err(|e| EngineError::Internal(format!("failed to serialize DKG part2 secret: {e}")))?;

    let result = DkgPart2Result {
        secret_package_hex: hex::encode(&round2_secret_package_bytes),
        packages,
    };
    round2_secret_package_bytes.zeroize();

    Ok(result)
}

pub fn dkg_part3(request: DkgPart3Request) -> Result<DkgPart3Result, EngineError> {
    enforce_provenance_gate()?;

    let mut secret_package_bytes = decode_hex_field(
        "DKGPart3",
        "secret_package_hex",
        &request.secret_package_hex,
    )?;
    let secret_package_result =
        frost::keys::dkg::round2::SecretPackage::deserialize(&secret_package_bytes);
    secret_package_bytes.zeroize();
    let mut secret_package = secret_package_result
        .map_err(|e| EngineError::Validation(format!("DKGPart3: invalid secret package: {e}")))?;

    let round1_packages = match decode_round1_package_map("DKGPart3", &request.round1_packages) {
        Ok(round1_packages) => round1_packages,
        Err(err) => {
            secret_package.zeroize();
            return Err(err);
        }
    };
    let round2_packages = match decode_round2_package_map(
        "DKGPart3",
        &request.round2_packages,
        Some(*secret_package.identifier()),
    ) {
        Ok(round2_packages) => round2_packages,
        Err(err) => {
            secret_package.zeroize();
            return Err(err);
        }
    };
    let dkg_result = frost::keys::dkg::part3(&secret_package, &round1_packages, &round2_packages);
    secret_package.zeroize();
    let (key_package, public_key_package) =
        dkg_result.map_err(|e| EngineError::Validation(format!("DKGPart3 failed: {e}")))?;

    let is_even_y = public_key_package.has_even_y();
    let key_package = key_package.into_even_y(Some(is_even_y));
    let public_key_package = public_key_package.into_even_y(Some(is_even_y));

    let native_public_key_package = native_public_key_package_from_frost(&public_key_package)?;
    let mut key_package_bytes = key_package
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize DKG key package: {e}")))?;
    let result = DkgPart3Result {
        key_package: NativeFrostKeyPackage {
            identifier: frost_identifier_to_go_string(*key_package.identifier()),
            data_hex: hex::encode(&key_package_bytes),
        },
        public_key_package: native_public_key_package,
    };
    key_package_bytes.zeroize();

    Ok(result)
}

pub fn generate_nonces_and_commitments(
    request: GenerateNoncesAndCommitmentsRequest,
) -> Result<GenerateNoncesAndCommitmentsResult, EngineError> {
    enforce_provenance_gate()?;

    let key_package = decode_key_package(
        "GenerateNoncesAndCommitments",
        &request.key_package_identifier,
        &request.key_package_hex,
    )?;
    let mut rng = zeroizing_rng_from_os();
    let (mut nonces, commitments) = frost::round1::commit(key_package.signing_share(), &mut rng);
    let commitment_bytes = match commitments.serialize() {
        Ok(commitment_bytes) => commitment_bytes,
        Err(err) => {
            nonces.zeroize();
            return Err(EngineError::Internal(format!(
                "failed to serialize signing commitments: {err}"
            )));
        }
    };
    let nonces_bytes_result = nonces.serialize();
    nonces.zeroize();
    let mut nonces_bytes = nonces_bytes_result
        .map_err(|e| EngineError::Internal(format!("failed to serialize signing nonces: {e}")))?;

    let result = GenerateNoncesAndCommitmentsResult {
        nonces_hex: hex::encode(&nonces_bytes),
        commitment: NativeFrostCommitment {
            identifier: frost_identifier_to_go_string(*key_package.identifier()),
            data_hex: hex::encode(commitment_bytes),
        },
    };
    nonces_bytes.zeroize();

    Ok(result)
}

pub fn new_signing_package(
    request: NewSigningPackageRequest,
) -> Result<NewSigningPackageResult, EngineError> {
    enforce_provenance_gate()?;

    let message = if request.message_hex.is_empty() {
        Vec::new()
    } else {
        hex::decode(&request.message_hex).map_err(|_| {
            EngineError::Validation("NewSigningPackage: message_hex must be valid hex".to_string())
        })?
    };
    let commitments = decode_signing_commitment_map("NewSigningPackage", &request.commitments)?;
    let signing_package = frost::SigningPackage::new(commitments, &message);
    let signing_package_bytes = signing_package
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize signing package: {e}")))?;

    Ok(NewSigningPackageResult {
        signing_package_hex: hex::encode(signing_package_bytes),
    })
}

pub fn sign_share(request: SignShareRequest) -> Result<SignShareResult, EngineError> {
    enforce_provenance_gate()?;

    let signing_package_bytes = decode_hex_field(
        "SignShare",
        "signing_package_hex",
        &request.signing_package_hex,
    )?;
    let signing_package = frost::SigningPackage::deserialize(&signing_package_bytes)
        .map_err(|e| EngineError::Validation(format!("SignShare: invalid signing package: {e}")))?;

    let mut nonces_bytes = decode_hex_field("SignShare", "nonces_hex", &request.nonces_hex)?;
    let nonces_result = frost::round1::SigningNonces::deserialize(&nonces_bytes);
    nonces_bytes.zeroize();
    let mut nonces = nonces_result
        .map_err(|e| EngineError::Validation(format!("SignShare: invalid nonces: {e}")))?;

    let key_package = match decode_key_package(
        "SignShare",
        &request.key_package_identifier,
        &request.key_package_hex,
    ) {
        Ok(key_package) => key_package,
        Err(err) => {
            nonces.zeroize();
            return Err(err);
        }
    };
    let signature_share_result = frost::round2::sign(&signing_package, &nonces, &key_package);
    nonces.zeroize();
    let signature_share = signature_share_result
        .map_err(|e| EngineError::Validation(format!("SignShare failed: {e}")))?;
    let mut signature_share_bytes = signature_share.serialize();
    let result = SignShareResult {
        signature_share: NativeFrostSignatureShare {
            identifier: frost_identifier_to_go_string(*key_package.identifier()),
            data_hex: hex::encode(&signature_share_bytes),
        },
    };
    signature_share_bytes.zeroize();

    Ok(result)
}

pub fn aggregate(request: AggregateRequest) -> Result<AggregateResult, EngineError> {
    enforce_provenance_gate()?;

    let signing_package_bytes = decode_hex_field(
        "Aggregate",
        "signing_package_hex",
        &request.signing_package_hex,
    )?;
    let signing_package = frost::SigningPackage::deserialize(&signing_package_bytes)
        .map_err(|e| EngineError::Validation(format!("Aggregate: invalid signing package: {e}")))?;
    let signature_shares = decode_signature_share_map("Aggregate", &request.signature_shares)?;
    let public_key_package =
        native_public_key_package_to_frost("Aggregate", &request.public_key_package)?;
    let signature = frost::aggregate(&signing_package, &signature_shares, &public_key_package)
        .map_err(|e| EngineError::Validation(format!("Aggregate failed: {e}")))?;
    let signature_bytes = signature
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize aggregate: {e}")))?;

    Ok(AggregateResult {
        signature_hex: hex::encode(signature_bytes),
    })
}

fn build_deterministic_round_nonce_and_commitment(
    key_package: &frost::keys::KeyPackage,
    session_id: &str,
    round_id: &str,
    message_bytes: &[u8],
    participant_identifier: u16,
) -> (
    frost::round1::SigningNonces,
    frost::round1::SigningCommitments,
) {
    // Defense-in-depth: bind nonces directly to message bytes in addition to
    // `round_id` so future round ID schema changes cannot weaken nonce safety.
    let mut signing_share_bytes = key_package.signing_share().serialize();
    let mut nonce_seed = deterministic_seed(&[
        b"round-nonce",
        &signing_share_bytes,
        session_id.as_bytes(),
        round_id.as_bytes(),
        message_bytes,
        &participant_identifier.to_le_bytes(),
    ]);
    signing_share_bytes.zeroize();
    let mut nonce_rng = ZeroizingChaCha20Rng::from_seed(nonce_seed);
    nonce_seed.zeroize();

    frost::round1::commit(key_package.signing_share(), &mut nonce_rng)
}

fn fingerprint<T: serde::Serialize>(value: &T) -> Result<String, EngineError> {
    let mut bytes = serde_json::to_vec(value)
        .map_err(|e| EngineError::Internal(format!("failed to encode request: {e}")))?;
    let value_fingerprint = hash_hex(&bytes);
    bytes.zeroize();
    Ok(value_fingerprint)
}

fn canonicalize_dkg_request_for_fingerprint(request: &RunDkgRequest) -> RunDkgRequest {
    let mut canonical_request = request.clone();
    canonical_request
        .participants
        .sort_unstable_by(|left, right| {
            left.identifier
                .cmp(&right.identifier)
                .then_with(|| left.public_key_hex.cmp(&right.public_key_hex))
        });
    canonical_request
}

fn canonicalize_refresh_shares_request_for_fingerprint(
    request: &RefreshSharesRequest,
) -> RefreshSharesRequest {
    let mut canonical_request = request.clone();
    canonical_request
        .current_shares
        .sort_unstable_by(|left, right| {
            left.identifier
                .cmp(&right.identifier)
                .then_with(|| left.encrypted_share_hex.cmp(&right.encrypted_share_hex))
        });
    canonical_request
}

fn truthy_env_flag(raw_value: &str) -> bool {
    matches!(
        raw_value.trim().to_ascii_lowercase().as_str(),
        "1" | "true" | "yes" | "on"
    )
}

fn roast_strict_mode_enabled() -> bool {
    if signer_profile_is_production() {
        return true;
    }

    std::env::var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

#[cfg(any(test, feature = "bench-restart-hook"))]
fn bench_restart_hook_enabled() -> bool {
    std::env::var(TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

fn canonicalize_attempt_context_for_fingerprint(attempt_context: &mut Option<AttemptContext>) {
    if let Some(attempt_context) = attempt_context.as_mut() {
        attempt_context.included_participants.sort_unstable();
        attempt_context.included_participants_fingerprint = attempt_context
            .included_participants_fingerprint
            .to_ascii_lowercase();
        attempt_context.attempt_id = attempt_context.attempt_id.to_ascii_lowercase();
    }
}

fn canonicalize_attempt_transition_evidence_for_fingerprint(
    transition_evidence: &mut Option<AttemptTransitionEvidence>,
) {
    if let Some(transition_evidence) = transition_evidence.as_mut() {
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

fn round_attempt_id_component(attempt_context: Option<&AttemptContext>) -> String {
    attempt_context
        .map(|attempt_context| attempt_context.attempt_id.to_ascii_lowercase())
        .unwrap_or_else(|| ROUND_ID_NO_ATTEMPT_CONTEXT_COMPONENT.to_string())
}

fn derive_round_id(
    session_id: &str,
    key_group: &str,
    message_hex: &str,
    signing_participants_fingerprint: &str,
    attempt_context: Option<&AttemptContext>,
) -> String {
    let attempt_id_component = round_attempt_id_component(attempt_context);
    hash_hex(
        format!(
            "round:{}:{}:{}:{}:{}",
            session_id,
            key_group,
            message_hex,
            signing_participants_fingerprint,
            attempt_id_component
        )
        .as_bytes(),
    )
}

fn canonicalize_included_participants(
    included_participants: &[u16],
) -> Result<Vec<u16>, EngineError> {
    if included_participants.is_empty() {
        return Err(EngineError::Validation(
            "attempt_context.included_participants must not be empty".to_string(),
        ));
    }

    let mut canonical = included_participants.to_vec();
    canonical.sort_unstable();

    let mut seen = HashSet::new();
    for participant_identifier in &canonical {
        if *participant_identifier == 0 {
            return Err(EngineError::Validation(
                "attempt_context.included_participants must contain non-zero identifiers"
                    .to_string(),
            ));
        }
        if !seen.insert(*participant_identifier) {
            return Err(EngineError::Validation(format!(
                "attempt_context.included_participants contains duplicate identifier [{}]",
                participant_identifier
            )));
        }
    }

    Ok(canonical)
}

fn push_framed_component(payload: &mut Vec<u8>, component: &[u8]) -> Result<(), EngineError> {
    let component_len = u32::try_from(component.len()).map_err(|_| {
        EngineError::Validation("attempt_context component exceeds u32 framing limit".to_string())
    })?;
    payload.extend_from_slice(&component_len.to_be_bytes());
    payload.extend_from_slice(component);
    Ok(())
}

fn roast_hash_hex_with_components(
    domain: &str,
    components: &[&[u8]],
) -> Result<String, EngineError> {
    let mut payload = Vec::new();
    push_framed_component(&mut payload, domain.as_bytes())?;
    for component in components {
        push_framed_component(&mut payload, component)?;
    }

    Ok(hash_hex(&payload))
}

fn roast_attempt_seed_from_message_digest_hex(
    message_digest_hex: &str,
) -> Result<i64, EngineError> {
    let message_digest_bytes = hex::decode(message_digest_hex).map_err(|_| {
        EngineError::Internal("message digest hex must decode for attempt seed".to_string())
    })?;

    if message_digest_bytes.len() < 8 {
        return Err(EngineError::Internal(
            "message digest must be at least 8 bytes for attempt seed".to_string(),
        ));
    }

    let mut seed_bytes = [0_u8; 8];
    seed_bytes.copy_from_slice(&message_digest_bytes[..8]);
    Ok(i64::from_be_bytes(seed_bytes))
}

fn roast_included_participants_fingerprint_hex(
    included_participants: &[u16],
) -> Result<String, EngineError> {
    let mut participant_payload = Vec::new();
    for participant_identifier in included_participants {
        push_framed_component(
            &mut participant_payload,
            &participant_identifier.to_be_bytes(),
        )?;
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
) -> Result<String, EngineError> {
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

fn validate_attempt_context(
    session_id: &str,
    message_digest_hex: &str,
    threshold: u16,
    attempt_context: Option<&AttemptContext>,
    strict_mode_enabled: bool,
) -> Result<Option<Vec<u16>>, EngineError> {
    let Some(attempt_context) = attempt_context else {
        if strict_mode_enabled {
            return Err(EngineError::Validation(
                "attempt_context is required when ROAST strict mode is enabled".to_string(),
            ));
        }
        return Ok(None);
    };

    if attempt_context.attempt_number == 0 {
        return Err(EngineError::Validation(
            "attempt_context.attempt_number must be at least 1".to_string(),
        ));
    }

    if attempt_context.coordinator_identifier == 0 {
        return Err(EngineError::Validation(
            "attempt_context.coordinator_identifier must be non-zero".to_string(),
        ));
    }

    let canonical_included_participants =
        canonicalize_included_participants(&attempt_context.included_participants)?;

    if canonical_included_participants.len() < usize::from(threshold) {
        return Err(EngineError::Validation(format!(
            "attempt_context.included_participants must contain at least threshold members [{}]",
            threshold
        )));
    }

    if !canonical_included_participants.contains(&attempt_context.coordinator_identifier) {
        return Err(EngineError::Validation(
            "attempt_context.coordinator_identifier must be included in attempt_context.included_participants".to_string(),
        ));
    }

    let attempt_seed = roast_attempt_seed_from_message_digest_hex(message_digest_hex)?;
    let expected_coordinator_identifier = select_coordinator_identifier(
        &canonical_included_participants,
        attempt_seed,
        attempt_context.attempt_number,
    )
    .ok_or_else(|| {
        EngineError::Validation(
            "attempt_context.included_participants must not be empty".to_string(),
        )
    })?;
    if expected_coordinator_identifier != attempt_context.coordinator_identifier {
        return Err(EngineError::Validation(
            "attempt_context.coordinator_identifier does not match deterministic coordinator selection".to_string(),
        ));
    }

    let expected_included_participants_fingerprint_hex =
        roast_included_participants_fingerprint_hex(&canonical_included_participants)?;

    if !attempt_context
        .included_participants_fingerprint
        .eq_ignore_ascii_case(&expected_included_participants_fingerprint_hex)
    {
        return Err(EngineError::Validation(
            "attempt_context.included_participants_fingerprint does not match canonical participants".to_string(),
        ));
    }

    let expected_attempt_id_hex = roast_attempt_id_hex(
        session_id,
        message_digest_hex,
        attempt_context.attempt_number,
        attempt_context.coordinator_identifier,
        &expected_included_participants_fingerprint_hex,
    )?;

    if !attempt_context
        .attempt_id
        .eq_ignore_ascii_case(&expected_attempt_id_hex)
    {
        return Err(EngineError::Validation(
            "attempt_context.attempt_id does not match canonical attempt context".to_string(),
        ));
    }

    Ok(Some(canonical_included_participants))
}

fn canonical_attempt_context(attempt_context: &AttemptContext) -> AttemptContext {
    let mut canonical = Some(attempt_context.clone());
    canonicalize_attempt_context_for_fingerprint(&mut canonical);
    canonical.expect("attempt context canonicalization preserves value")
}

enum ActiveAttemptMatchOutcome {
    MatchActive,
    AdvanceAuthorized,
}

fn validate_transition_exclusion_evidence(
    exclusion_evidence: Option<&AttemptExclusionEvidence>,
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: &AttemptContext,
) -> Result<(), EngineError> {
    let exclusion_evidence = exclusion_evidence.ok_or_else(|| {
        EngineError::Validation(
            "attempt_transition_evidence.exclusion_evidence is required for attempt advancement"
                .to_string(),
        )
    })?;

    let reason = exclusion_evidence.reason.trim().to_ascii_lowercase();
    if reason != ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT
        && reason != ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF
    {
        return Err(EngineError::Validation(format!(
            "attempt_transition_evidence.exclusion_evidence.reason [{}] is unsupported",
            exclusion_evidence.reason
        )));
    }

    let mut excluded_member_identifiers = HashSet::new();
    for member_identifier in &exclusion_evidence.excluded_member_identifiers {
        if *member_identifier == 0 {
            return Err(EngineError::Validation(
                "attempt_transition_evidence.exclusion_evidence.excluded_member_identifiers must contain non-zero identifiers".to_string(),
            ));
        }
        if !excluded_member_identifiers.insert(*member_identifier) {
            return Err(EngineError::Validation(format!(
                "attempt_transition_evidence.exclusion_evidence.excluded_member_identifiers contains duplicate identifier [{}]",
                member_identifier
            )));
        }
        if !active_attempt_context
            .included_participants
            .contains(member_identifier)
        {
            return Err(EngineError::Validation(format!(
                "attempt_transition_evidence.exclusion_evidence.excluded_member_identifiers contains identifier [{}] not present in active attempt context",
                member_identifier
            )));
        }
    }

    for member_identifier in &excluded_member_identifiers {
        if incoming_attempt_context
            .included_participants
            .contains(member_identifier)
        {
            return Err(EngineError::Validation(format!(
                "attempt_transition_evidence.exclusion_evidence identifier [{}] must not remain in incoming attempt_context.included_participants",
                member_identifier
            )));
        }
    }

    if excluded_member_identifiers.contains(&incoming_attempt_context.coordinator_identifier) {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.exclusion_evidence must not exclude incoming attempt_context.coordinator_identifier".to_string(),
        ));
    }

    match reason.as_str() {
        ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT => {
            // `coordinator_timeout` may intentionally exclude zero members.
            // This models coordinator rotation without participant-level fault
            // attribution, so no auto-quarantine penalty is applied.
            if exclusion_evidence.invalid_share_proof_fingerprint.is_some() {
                return Err(EngineError::Validation(
                    "attempt_transition_evidence.exclusion_evidence.invalid_share_proof_fingerprint must be omitted for coordinator_timeout reason".to_string(),
                ));
            }
        }
        ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF => {
            if excluded_member_identifiers.is_empty() {
                return Err(EngineError::Validation(
                    "attempt_transition_evidence.exclusion_evidence.excluded_member_identifiers must contain at least one identifier for invalid_share_proof reason".to_string(),
                ));
            }
            let proof_fingerprint = exclusion_evidence
                .invalid_share_proof_fingerprint
                .as_deref()
                .ok_or_else(|| {
                    EngineError::Validation(
                        "attempt_transition_evidence.exclusion_evidence.invalid_share_proof_fingerprint is required for invalid_share_proof reason".to_string(),
                    )
                })?;
            let proof_fingerprint = proof_fingerprint.trim();
            if proof_fingerprint.is_empty() {
                return Err(EngineError::Validation(
                    "attempt_transition_evidence.exclusion_evidence.invalid_share_proof_fingerprint must be non-empty valid hex".to_string(),
                ));
            }
            hex::decode(proof_fingerprint).map_err(|_| {
                EngineError::Validation(
                    "attempt_transition_evidence.exclusion_evidence.invalid_share_proof_fingerprint must be valid hex".to_string(),
                )
            })?;
        }
        _ => unreachable!("reason value filtered above"),
    }

    Ok(())
}

fn build_attempt_transition_telemetry(
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: &AttemptContext,
    transition_evidence: Option<&AttemptTransitionEvidence>,
) -> Option<AttemptTransitionTelemetry> {
    let exclusion_evidence = transition_evidence?.exclusion_evidence.as_ref()?;
    let mut excluded_member_identifiers = exclusion_evidence.excluded_member_identifiers.clone();
    excluded_member_identifiers.sort_unstable();

    Some(AttemptTransitionTelemetry {
        from_attempt_number: active_attempt_context.attempt_number,
        to_attempt_number: incoming_attempt_context.attempt_number,
        from_coordinator_identifier: active_attempt_context.coordinator_identifier,
        to_coordinator_identifier: incoming_attempt_context.coordinator_identifier,
        reason: exclusion_evidence.reason.trim().to_ascii_lowercase(),
        excluded_member_identifiers,
        coordinator_rotated: active_attempt_context.coordinator_identifier
            != incoming_attempt_context.coordinator_identifier,
    })
}

fn build_transcript_audit_record(
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: &AttemptContext,
    transition_evidence: &AttemptTransitionEvidence,
) -> Result<TranscriptAuditRecord, EngineError> {
    let exclusion_evidence = transition_evidence
        .exclusion_evidence
        .as_ref()
        .ok_or_else(|| {
            EngineError::Internal("missing exclusion evidence for transcript record".to_string())
        })?;

    let mut excluded_member_identifiers = exclusion_evidence.excluded_member_identifiers.clone();
    excluded_member_identifiers.sort_unstable();

    let reason = exclusion_evidence.reason.trim().to_ascii_lowercase();
    let invalid_share_proof_fingerprint = exclusion_evidence
        .invalid_share_proof_fingerprint
        .as_deref()
        .map(|fingerprint| fingerprint.trim().to_ascii_lowercase());
    let mut record = TranscriptAuditRecord {
        from_attempt_number: active_attempt_context.attempt_number,
        to_attempt_number: incoming_attempt_context.attempt_number,
        from_attempt_id: active_attempt_context.attempt_id.to_ascii_lowercase(),
        to_attempt_id: incoming_attempt_context.attempt_id.to_ascii_lowercase(),
        previous_round_id: transition_evidence.previous_round_id.clone(),
        previous_sign_request_fingerprint: transition_evidence
            .previous_sign_request_fingerprint
            .clone(),
        from_coordinator_identifier: active_attempt_context.coordinator_identifier,
        to_coordinator_identifier: incoming_attempt_context.coordinator_identifier,
        reason,
        excluded_member_identifiers,
        invalid_share_proof_fingerprint,
        transcript_hash: String::new(),
        recorded_at_unix: now_unix(),
    };
    // Two-pass hash: fingerprint the canonical record with an empty
    // `transcript_hash` sentinel, then persist the resulting hash value.
    let transcript_hash = fingerprint(&record)?;
    record.transcript_hash = transcript_hash;
    Ok(record)
}

fn enforce_not_quarantined_identifiers(
    session_id: &str,
    member_identifiers: &[u16],
    quarantined_operator_identifiers: &HashSet<u16>,
    auto_quarantine_config: Option<&AutoQuarantineConfig>,
) -> Result<(), EngineError> {
    let Some(auto_quarantine_config) = auto_quarantine_config else {
        return Ok(());
    };

    for member_identifier in member_identifiers {
        if auto_quarantine_config
            .dao_allowlist_identifiers
            .contains(member_identifier)
        {
            continue;
        }
        if quarantined_operator_identifiers.contains(member_identifier) {
            return reject_quarantine_policy(
                session_id,
                "operator_auto_quarantined",
                format!(
                    "operator identifier [{}] is auto-quarantined and requires DAO allowlist override",
                    member_identifier
                ),
            );
        }
    }

    Ok(())
}

fn auto_quarantine_penalty_for_record(
    record: &TranscriptAuditRecord,
    auto_quarantine_config: &AutoQuarantineConfig,
) -> u64 {
    if record.reason == ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF {
        auto_quarantine_config.invalid_share_penalty
    } else {
        auto_quarantine_config.timeout_penalty
    }
}

fn apply_auto_quarantine_faults_for_transition(
    engine_state: &mut EngineState,
    session_id: &str,
    record: &TranscriptAuditRecord,
    auto_quarantine_config: Option<&AutoQuarantineConfig>,
) {
    let Some(auto_quarantine_config) = auto_quarantine_config else {
        return;
    };

    let penalty = auto_quarantine_penalty_for_record(record, auto_quarantine_config);
    for excluded_member_identifier in &record.excluded_member_identifiers {
        if auto_quarantine_config
            .dao_allowlist_identifiers
            .contains(excluded_member_identifier)
        {
            // Governance allowlist acts as explicit manual re-enable path.
            engine_state
                .quarantined_operator_identifiers
                .remove(excluded_member_identifier);
            continue;
        }

        let score = engine_state
            .operator_fault_scores
            .entry(*excluded_member_identifier)
            .or_insert(0);
        *score = score.saturating_add(penalty);
        record_hardening_telemetry(|telemetry| {
            telemetry.auto_quarantine_fault_events_total = telemetry
                .auto_quarantine_fault_events_total
                .saturating_add(1);
        });

        if *score >= auto_quarantine_config.fault_threshold
            && engine_state
                .quarantined_operator_identifiers
                .insert(*excluded_member_identifier)
        {
            record_hardening_telemetry(|telemetry| {
                telemetry.auto_quarantine_enforcements_total = telemetry
                    .auto_quarantine_enforcements_total
                    .saturating_add(1);
            });
            log_policy_decision(
                "auto_quarantine",
                session_id,
                "quarantine",
                "fault_threshold_reached",
            );
        }
    }
}

fn validate_attempt_transition_evidence(
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: &AttemptContext,
    transition_evidence: Option<&AttemptTransitionEvidence>,
    round_state: Option<&RoundState>,
    sign_request_fingerprint: Option<&str>,
) -> Result<(), EngineError> {
    let transition_evidence = transition_evidence.ok_or_else(|| {
        EngineError::Validation(
            "attempt_context.attempt_number advancement requires attempt_transition_evidence"
                .to_string(),
        )
    })?;

    if incoming_attempt_context.attempt_number != active_attempt_context.attempt_number + 1 {
        return Err(EngineError::Validation(format!(
            "attempt_context.attempt_number [{}] is ahead of active attempt_number [{}] without transition authorization",
            incoming_attempt_context.attempt_number, active_attempt_context.attempt_number
        )));
    }

    if transition_evidence.from_attempt_number != active_attempt_context.attempt_number {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.from_attempt_number does not match active attempt context"
                .to_string(),
        ));
    }

    if !transition_evidence
        .from_attempt_id
        .eq_ignore_ascii_case(&active_attempt_context.attempt_id)
    {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.from_attempt_id does not match active attempt context"
                .to_string(),
        ));
    }

    if transition_evidence.from_coordinator_identifier
        != active_attempt_context.coordinator_identifier
    {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.from_coordinator_identifier does not match active attempt context".to_string(),
        ));
    }

    validate_transition_exclusion_evidence(
        transition_evidence.exclusion_evidence.as_ref(),
        active_attempt_context,
        incoming_attempt_context,
    )?;

    let round_state = round_state.ok_or_else(|| {
        EngineError::Validation(
            "attempt_transition_evidence requires active round state".to_string(),
        )
    })?;
    if transition_evidence.previous_round_id != round_state.round_id {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.previous_round_id does not match active round state"
                .to_string(),
        ));
    }

    let sign_request_fingerprint = sign_request_fingerprint.ok_or_else(|| {
        EngineError::Validation(
            "attempt_transition_evidence requires active sign request fingerprint".to_string(),
        )
    })?;
    if transition_evidence.previous_sign_request_fingerprint != sign_request_fingerprint {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.previous_sign_request_fingerprint does not match active sign request".to_string(),
        ));
    }

    if incoming_attempt_context
        .attempt_id
        .eq_ignore_ascii_case(&active_attempt_context.attempt_id)
    {
        return Err(EngineError::Validation(
            "attempt_context.attempt_id must change when advancing attempt_number".to_string(),
        ));
    }

    Ok(())
}

fn enforce_active_attempt_context_match(
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: Option<&AttemptContext>,
    transition_evidence: Option<&AttemptTransitionEvidence>,
    round_state: Option<&RoundState>,
    sign_request_fingerprint: Option<&str>,
    strict_mode_enabled: bool,
) -> Result<ActiveAttemptMatchOutcome, EngineError> {
    let Some(incoming_attempt_context) = incoming_attempt_context else {
        if !strict_mode_enabled {
            return Ok(ActiveAttemptMatchOutcome::MatchActive);
        }
        return Err(EngineError::Validation(
            "attempt_context is required when ROAST strict mode is enabled or an active attempt context exists".to_string(),
        ));
    };

    let incoming_attempt_context = canonical_attempt_context(incoming_attempt_context);

    if incoming_attempt_context.attempt_number < active_attempt_context.attempt_number {
        return Err(EngineError::Validation(format!(
            "attempt_context.attempt_number [{}] is stale; active attempt_number is [{}]",
            incoming_attempt_context.attempt_number, active_attempt_context.attempt_number
        )));
    }

    if incoming_attempt_context.attempt_number > active_attempt_context.attempt_number {
        validate_attempt_transition_evidence(
            active_attempt_context,
            &incoming_attempt_context,
            transition_evidence,
            round_state,
            sign_request_fingerprint,
        )?;

        return Ok(ActiveAttemptMatchOutcome::AdvanceAuthorized);
    }

    if incoming_attempt_context.coordinator_identifier
        != active_attempt_context.coordinator_identifier
    {
        return Err(EngineError::Validation(format!(
            "attempt_context.coordinator_identifier [{}] does not match active coordinator [{}]",
            incoming_attempt_context.coordinator_identifier,
            active_attempt_context.coordinator_identifier
        )));
    }

    if incoming_attempt_context.included_participants
        != active_attempt_context.included_participants
    {
        return Err(EngineError::Validation(
            "attempt_context.included_participants does not match active attempt context"
                .to_string(),
        ));
    }

    if incoming_attempt_context.included_participants_fingerprint
        != active_attempt_context.included_participants_fingerprint
    {
        return Err(EngineError::Validation(
            "attempt_context.included_participants_fingerprint does not match active attempt context"
                .to_string(),
        ));
    }

    if incoming_attempt_context.attempt_id != active_attempt_context.attempt_id {
        return Err(EngineError::Validation(
            "attempt_context.attempt_id does not match active attempt context".to_string(),
        ));
    }

    Ok(ActiveAttemptMatchOutcome::MatchActive)
}

fn validate_session_id(session_id: &str) -> Result<(), EngineError> {
    if session_id.is_empty() {
        return Err(EngineError::Validation(
            "session_id must be non-empty".to_string(),
        ));
    }

    if session_id.len() > 128 {
        return Err(EngineError::Validation(
            "session_id exceeds max length 128 bytes".to_string(),
        ));
    }

    if session_id.bytes().any(|byte| {
        byte.is_ascii_control() || byte == b' ' || byte == b'=' || byte == b'"' || byte == b'\\'
    }) {
        return Err(EngineError::Validation(
            "session_id contains disallowed characters (control, space, =, \", \\)".to_string(),
        ));
    }

    Ok(())
}

fn clear_session_signing_material(session: &mut SessionState) {
    // Intentionally retain `dkg_result` and `dkg_request_fingerprint` because
    // RefreshShares is an independent post-DKG flow.
    //
    // Best-effort zeroization: clear byte/string material we own directly
    // before dropping Option containers.
    if let Some(sign_request_fingerprint) = session.sign_request_fingerprint.as_mut() {
        sign_request_fingerprint.zeroize();
    }
    if let Some(sign_message_bytes) = session.sign_message_bytes.as_mut() {
        sign_message_bytes.zeroize();
    }
    if let Some(round_state) = session.round_state.as_mut() {
        round_state.session_id.zeroize();
        round_state.round_id.zeroize();
        round_state.message_digest_hex.zeroize();
        if let Some(signing_participants) = round_state.signing_participants.as_mut() {
            signing_participants.zeroize();
        }
        if let Some(transition_telemetry) = round_state.attempt_transition_telemetry.as_mut() {
            transition_telemetry.from_attempt_number.zeroize();
            transition_telemetry.to_attempt_number.zeroize();
            transition_telemetry.from_coordinator_identifier.zeroize();
            transition_telemetry.to_coordinator_identifier.zeroize();
            transition_telemetry.reason.zeroize();
            transition_telemetry.excluded_member_identifiers.zeroize();
            transition_telemetry.coordinator_rotated = false;
        }
        round_state.own_contribution.identifier.zeroize();
        round_state.own_contribution.signature_share_hex.zeroize();
    }
    if let Some(active_attempt_context) = session.active_attempt_context.as_mut() {
        active_attempt_context.included_participants.zeroize();
        active_attempt_context
            .included_participants_fingerprint
            .zeroize();
        active_attempt_context.attempt_id.zeroize();
    }

    session.dkg_key_packages = None;
    session.dkg_public_key_package = None;
    session.sign_request_fingerprint = None;
    session.sign_message_bytes = None;
    session.round_state = None;
    session.active_attempt_context = None;
}

fn clear_active_sign_round_for_attempt_transition(session: &mut SessionState) {
    if let Some(sign_request_fingerprint) = session.sign_request_fingerprint.as_mut() {
        sign_request_fingerprint.zeroize();
    }
    if let Some(sign_message_bytes) = session.sign_message_bytes.as_mut() {
        sign_message_bytes.zeroize();
    }
    if let Some(round_state) = session.round_state.as_mut() {
        round_state.session_id.zeroize();
        round_state.round_id.zeroize();
        round_state.message_digest_hex.zeroize();
        if let Some(signing_participants) = round_state.signing_participants.as_mut() {
            signing_participants.zeroize();
        }
        if let Some(transition_telemetry) = round_state.attempt_transition_telemetry.as_mut() {
            transition_telemetry.from_attempt_number.zeroize();
            transition_telemetry.to_attempt_number.zeroize();
            transition_telemetry.from_coordinator_identifier.zeroize();
            transition_telemetry.to_coordinator_identifier.zeroize();
            transition_telemetry.reason.zeroize();
            transition_telemetry.excluded_member_identifiers.zeroize();
            transition_telemetry.coordinator_rotated = false;
        }
        round_state.own_contribution.identifier.zeroize();
        round_state.own_contribution.signature_share_hex.zeroize();
    }

    session.sign_request_fingerprint = None;
    session.sign_message_bytes = None;
    session.round_state = None;
}

pub fn run_dkg(request: RunDkgRequest) -> Result<DkgResult, EngineError> {
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::RunDkg);
    validate_session_id(&request.session_id)?;
    enforce_bootstrap_dealer_dkg_disabled_in_production(&request.session_id)?;

    record_hardening_telemetry(|telemetry| {
        telemetry.run_dkg_calls_total = telemetry.run_dkg_calls_total.saturating_add(1);
    });
    enforce_provenance_gate()?;
    enforce_admission_policy(&request)?;

    if request.participants.len() < 2 {
        return Err(EngineError::Validation(
            "participants must contain at least 2 entries".to_string(),
        ));
    }

    if request.threshold < 2 || usize::from(request.threshold) > request.participants.len() {
        return Err(EngineError::Validation(
            "threshold must be between 2 and number of participants".to_string(),
        ));
    }

    let mut unique_identifiers = HashSet::new();
    for participant in &request.participants {
        if participant.identifier == 0 {
            return Err(EngineError::Validation(
                "participant identifier must be non-zero".to_string(),
            ));
        }

        if !unique_identifiers.insert(participant.identifier) {
            return Err(EngineError::Validation(
                "participant identifiers must be unique".to_string(),
            ));
        }
    }

    let request_fingerprint = fingerprint(&canonicalize_dkg_request_for_fingerprint(&request))?;

    {
        let guard = state()?
            .lock()
            .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
        if let Some(session) = guard.sessions.get(&request.session_id) {
            if let Some(existing) = &session.dkg_request_fingerprint {
                if existing == &request_fingerprint {
                    return session.dkg_result.clone().ok_or_else(|| {
                        EngineError::Internal("missing DKG result cache".to_string())
                    });
                }

                return Err(EngineError::SessionConflict {
                    session_id: request.session_id,
                });
            }
        } else {
            ensure_session_insert_capacity(&guard.sessions, &request.session_id)?;
        }
    }

    let mut participant_identifiers: Vec<u16> = request
        .participants
        .iter()
        .map(|participant| participant.identifier)
        .collect();
    participant_identifiers.sort_unstable();

    let auto_quarantine_config = load_auto_quarantine_config()?;
    let quarantined_operator_identifiers = {
        let guard = state()?
            .lock()
            .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
        guard.quarantined_operator_identifiers.clone()
    };
    enforce_not_quarantined_identifiers(
        &request.session_id,
        &participant_identifiers,
        &quarantined_operator_identifiers,
        auto_quarantine_config.as_ref(),
    )?;

    let frost_identifiers: Vec<frost::Identifier> = participant_identifiers
        .iter()
        .map(|identifier| participant_identifier_to_frost_identifier(*identifier))
        .collect::<Result<Vec<_>, _>>()?;

    let mut keygen_rng_seed = [0u8; 32];
    OsRng.fill_bytes(&mut keygen_rng_seed);
    let keygen_rng = ZeroizingChaCha20Rng::from_seed(keygen_rng_seed);
    keygen_rng_seed.zeroize();

    let (secret_shares, public_key_package) = frost::keys::generate_with_dealer(
        request.participants.len() as u16,
        request.threshold,
        frost::keys::IdentifierList::Custom(&frost_identifiers),
        keygen_rng,
    )
    .map_err(|e| EngineError::Internal(format!("failed to generate key shares: {e}")))?;

    let mut participant_identifier_by_frost_identifier = HashMap::new();
    for (participant_identifier, frost_identifier) in
        participant_identifiers.iter().zip(frost_identifiers.iter())
    {
        participant_identifier_by_frost_identifier.insert(
            hex::encode(frost_identifier.serialize()),
            *participant_identifier,
        );
    }

    let mut key_packages = BTreeMap::new();
    for (frost_identifier, secret_share) in secret_shares {
        let participant_identifier = participant_identifier_by_frost_identifier
            .get(&hex::encode(frost_identifier.serialize()))
            .copied()
            .ok_or_else(|| {
                EngineError::Internal(
                    "missing participant identifier mapping for generated key share".to_string(),
                )
            })?;

        let key_package = frost::keys::KeyPackage::try_from(secret_share)
            .map_err(|e| EngineError::Internal(format!("failed to convert secret share: {e}")))?;

        key_packages.insert(participant_identifier, key_package);
    }

    if key_packages.len() != request.participants.len() {
        return Err(EngineError::Internal(
            "generated key package count mismatch".to_string(),
        ));
    }

    let key_group = public_key_package
        .verifying_key()
        .serialize()
        .map(hex::encode)
        .map_err(|e| EngineError::Internal(format!("failed to serialize verifying key: {e}")))?;

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    ensure_session_insert_capacity(&guard.sessions, &request.session_id)?;

    let session = guard
        .sessions
        .entry(request.session_id.clone())
        .or_insert_with(SessionState::default);

    if let Some(existing) = &session.dkg_request_fingerprint {
        if existing == &request_fingerprint {
            return session
                .dkg_result
                .clone()
                .ok_or_else(|| EngineError::Internal("missing DKG result cache".to_string()));
        }

        return Err(EngineError::SessionConflict {
            session_id: request.session_id,
        });
    }

    let result = DkgResult {
        session_id: request.session_id,
        key_group,
        participant_count: request.participants.len() as u16,
        threshold: request.threshold,
        created_at_unix: now_unix(),
    };

    session.dkg_request_fingerprint = Some(request_fingerprint);
    session.dkg_key_packages = Some(key_packages);
    session.dkg_public_key_package = Some(public_key_package);
    session.dkg_result = Some(result.clone());
    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.run_dkg_success_total = telemetry.run_dkg_success_total.saturating_add(1);
    });

    Ok(result)
}

fn enforce_bootstrap_dealer_dkg_disabled_in_production(
    session_id: &str,
) -> Result<(), EngineError> {
    if signer_profile_is_production() {
        return Err(EngineError::LifecyclePolicyRejected {
            session_id: session_id.to_string(),
            reason_code: "bootstrap_dealer_dkg_disabled_in_production".to_string(),
            detail: format!(
                "bootstrap dealer DKG is disabled when {TBTC_SIGNER_PROFILE_ENV}={TBTC_SIGNER_PROFILE_PRODUCTION}; production requires distributed DKG wiring"
            ),
        });
    }

    Ok(())
}

pub fn start_sign_round(request: StartSignRoundRequest) -> Result<RoundState, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.start_sign_round_calls_total =
            telemetry.start_sign_round_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::StartSignRound);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    if request.member_identifier == 0 {
        return Err(EngineError::Validation(
            "member_identifier must be non-zero".to_string(),
        ));
    }

    let message_bytes = hex::decode(&request.message_hex)
        .map_err(|_| EngineError::Validation("message_hex must be valid hex".to_string()))?;
    let message_digest_hex = hash_hex(&message_bytes);
    let strict_roast_mode_enabled = roast_strict_mode_enabled();

    let request_fingerprint = {
        let mut canonical_request = request.clone();
        if let Some(signing_participants) = canonical_request.signing_participants.as_mut() {
            signing_participants.sort_unstable();
        }
        canonicalize_attempt_context_for_fingerprint(&mut canonical_request.attempt_context);
        canonicalize_attempt_transition_evidence_for_fingerprint(
            &mut canonical_request.attempt_transition_evidence,
        );
        fingerprint(&canonical_request)?
    };
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let auto_quarantine_config = load_auto_quarantine_config()?;
    let quarantined_operator_identifiers = guard.quarantined_operator_identifiers.clone();

    let mut pending_transition_record = None;
    let round_state =
        {
            let session = guard.sessions.get_mut(&request.session_id).ok_or_else(|| {
                EngineError::SessionNotFound {
                    session_id: request.session_id.clone(),
                }
            })?;

            let dkg = session
                .dkg_result
                .clone()
                .ok_or_else(|| EngineError::DkgNotReady {
                    session_id: request.session_id.clone(),
                })?;

            if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
                return Err(EngineError::LifecyclePolicyRejected {
                    session_id: request.session_id.clone(),
                    reason_code: "emergency_rekey_required".to_string(),
                    detail: format!(
                        "emergency rekey required for session [{}] since [{}]: {}",
                        request.session_id,
                        emergency_rekey_event.triggered_at_unix,
                        emergency_rekey_event.reason
                    ),
                });
            }

            if session.finalize_request_fingerprint.is_some() {
                // Lifecycle terminal state: once finalize succeeds for a session, we
                // intentionally return SessionFinalized and require a new session_id
                // for any subsequent StartSignRound call on that session ID.
                return Err(EngineError::SessionFinalized {
                    session_id: request.session_id,
                });
            }

            if request.key_group != dkg.key_group {
                return Err(EngineError::Validation(
                    "key_group does not match DKG output for this session".to_string(),
                ));
            }

            {
                let dkg_key_packages = session.dkg_key_packages.as_ref().ok_or_else(|| {
                    EngineError::Internal("missing DKG key package cache".to_string())
                })?;

                if !dkg_key_packages.contains_key(&request.member_identifier) {
                    return Err(EngineError::Validation(
                        "member_identifier is not a DKG participant for this session".to_string(),
                    ));
                }
            }
            enforce_signing_message_binding_to_policy_checked_build_tx(
                &request.session_id,
                &request.message_hex,
                session.tx_result.as_ref(),
            )?;

            // Guard against partial legacy state where sign material was cleared but
            // active attempt context was not.
            if session.sign_request_fingerprint.is_none() || session.round_state.is_none() {
                session.active_attempt_context = None;
            }

            let canonical_attempt_context = request
                .attempt_context
                .as_ref()
                .map(canonical_attempt_context);
            let mut attempt_transition_telemetry = None;
            let mut attempt_transition_record = None;
            if let Some(active_attempt_context) = session.active_attempt_context.as_ref() {
                let active_attempt_match_outcome = enforce_active_attempt_context_match(
                    active_attempt_context,
                    canonical_attempt_context.as_ref(),
                    request.attempt_transition_evidence.as_ref(),
                    session.round_state.as_ref(),
                    session.sign_request_fingerprint.as_deref(),
                    strict_roast_mode_enabled,
                )?;

                if let ActiveAttemptMatchOutcome::AdvanceAuthorized = active_attempt_match_outcome {
                    let incoming_attempt_context =
                        canonical_attempt_context.as_ref().ok_or_else(|| {
                            EngineError::Internal(
                                "missing incoming attempt context for authorized transition"
                                    .to_string(),
                            )
                        })?;
                    let transition_evidence = request
                        .attempt_transition_evidence
                        .as_ref()
                        .ok_or_else(|| {
                            EngineError::Internal(
                                "missing attempt_transition_evidence for authorized transition"
                                    .to_string(),
                            )
                        })?;
                    attempt_transition_telemetry = build_attempt_transition_telemetry(
                        active_attempt_context,
                        incoming_attempt_context,
                        Some(transition_evidence),
                    );
                    if attempt_transition_telemetry.is_none() {
                        return Err(EngineError::Internal(
                            "missing transition telemetry evidence for authorized transition"
                                .to_string(),
                        ));
                    }
                    attempt_transition_record = Some(build_transcript_audit_record(
                        active_attempt_context,
                        incoming_attempt_context,
                        transition_evidence,
                    )?);
                    clear_active_sign_round_for_attempt_transition(session);
                }
            }

            if let Some(existing) = &session.sign_request_fingerprint {
                if existing == &request_fingerprint {
                    return session.round_state.clone().ok_or_else(|| {
                        EngineError::Internal("missing round state cache".to_string())
                    });
                }

                return Err(EngineError::SessionConflict {
                    session_id: request.session_id,
                });
            }

            let signing_participants = {
                let dkg_key_packages = session.dkg_key_packages.as_ref().ok_or_else(|| {
                    EngineError::Internal("missing DKG key package cache".to_string())
                })?;
                resolve_signing_participants(&request, dkg.threshold, dkg_key_packages)?
            };
            if let Some(canonical_attempt_signing_participants) = validate_attempt_context(
                &request.session_id,
                &message_digest_hex,
                dkg.threshold,
                request.attempt_context.as_ref(),
                strict_roast_mode_enabled,
            )? {
                if canonical_attempt_signing_participants != signing_participants {
                    return Err(EngineError::Validation(
                    "attempt_context.included_participants must match resolved signing_participants"
                        .to_string(),
                ));
                }
            }
            enforce_not_quarantined_identifiers(
                &request.session_id,
                &signing_participants,
                &quarantined_operator_identifiers,
                auto_quarantine_config.as_ref(),
            )?;

            let signing_participants_fingerprint = fingerprint(&signing_participants)?;
            let consumed_attempt_id = canonical_attempt_context
                .as_ref()
                .map(|attempt_context| attempt_context.attempt_id.clone());
            if let Some(attempt_id) = consumed_attempt_id.as_ref() {
                if session.consumed_attempt_ids.contains(attempt_id) {
                    return Err(EngineError::ConsumedAttemptReplay {
                        session_id: request.session_id.clone(),
                        attempt_id: attempt_id.clone(),
                    });
                }
                ensure_consumed_registry_insert_capacity(
                    &session.consumed_attempt_ids,
                    attempt_id,
                    "consumed_attempt_ids",
                    &request.session_id,
                )?;
            }
            let round_id = derive_round_id(
                &request.session_id,
                &request.key_group,
                &request.message_hex,
                &signing_participants_fingerprint,
                canonical_attempt_context.as_ref(),
            );
            if session.consumed_sign_round_ids.contains(&round_id) {
                return Err(EngineError::ConsumedRoundReplay {
                    session_id: request.session_id.clone(),
                    round_id: round_id.clone(),
                });
            }
            ensure_consumed_registry_insert_capacity(
                &session.consumed_sign_round_ids,
                &round_id,
                "consumed_sign_round_ids",
                &request.session_id,
            )?;
            let own_contribution = {
                let dkg_key_packages = session.dkg_key_packages.as_ref().ok_or_else(|| {
                    EngineError::Internal("missing DKG key package cache".to_string())
                })?;
                build_real_signature_share_contribution(
                    dkg_key_packages,
                    &signing_participants,
                    &request,
                    &round_id,
                    &message_bytes,
                )?
            };

            if let Some(transition_telemetry) = attempt_transition_telemetry.as_ref() {
                record_hardening_telemetry(|telemetry| {
                    telemetry.attempt_transition_total =
                        telemetry.attempt_transition_total.saturating_add(1);
                    if transition_telemetry.coordinator_rotated {
                        telemetry.coordinator_failover_total =
                            telemetry.coordinator_failover_total.saturating_add(1);
                    }
                });
            }
            if let Some(transition_record) = attempt_transition_record.as_ref() {
                ensure_attempt_transition_record_insert_capacity(
                    &session.attempt_transition_records,
                    &request.session_id,
                )?;
                session
                    .attempt_transition_records
                    .push(transition_record.clone());
                pending_transition_record = Some(transition_record.clone());
            }

            let round_state = RoundState {
                session_id: request.session_id.clone(),
                round_id: round_id.clone(),
                required_contributions: dkg.threshold,
                message_digest_hex: message_digest_hex.clone(),
                signing_participants: Some(signing_participants),
                attempt_transition_telemetry,
                own_contribution,
            };

            session.sign_request_fingerprint = Some(request_fingerprint);
            session.sign_message_bytes = Some(Zeroizing::new(message_bytes));
            session.round_state = Some(round_state.clone());
            session.active_attempt_context = canonical_attempt_context;
            if let Some(attempt_id) = consumed_attempt_id {
                session.consumed_attempt_ids.insert(attempt_id);
            }
            session.consumed_sign_round_ids.insert(round_id);

            round_state
        };

    if let Some(transition_record) = pending_transition_record.as_ref() {
        apply_auto_quarantine_faults_for_transition(
            &mut guard,
            &request.session_id,
            transition_record,
            auto_quarantine_config.as_ref(),
        );
    }

    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.start_sign_round_success_total =
            telemetry.start_sign_round_success_total.saturating_add(1);
    });

    Ok(round_state)
}

fn resolve_signing_participants(
    request: &StartSignRoundRequest,
    threshold: u16,
    dkg_key_packages: &BTreeMap<u16, frost::keys::KeyPackage>,
) -> Result<Vec<u16>, EngineError> {
    let mut signing_participants = request
        .signing_participants
        .clone()
        .unwrap_or_else(|| dkg_key_packages.keys().copied().collect());
    if signing_participants.is_empty() {
        return Err(EngineError::Validation(
            "signing_participants must not be empty".to_string(),
        ));
    }

    signing_participants.sort_unstable();
    let mut unique_signing_participants = HashSet::new();

    for signing_participant in &signing_participants {
        if *signing_participant == 0 {
            return Err(EngineError::Validation(
                "signing_participants must contain non-zero identifiers".to_string(),
            ));
        }

        if !unique_signing_participants.insert(*signing_participant) {
            return Err(EngineError::Validation(format!(
                "signing_participants contains duplicate identifier [{}]",
                signing_participant
            )));
        }

        if !dkg_key_packages.contains_key(signing_participant) {
            return Err(EngineError::Validation(format!(
                "signing_participant [{}] is not a DKG participant for this session",
                signing_participant
            )));
        }
    }

    if signing_participants.len() < usize::from(threshold) {
        return Err(EngineError::Validation(format!(
            "signing_participants must contain at least threshold members [{}]",
            threshold
        )));
    }

    if !unique_signing_participants.contains(&request.member_identifier) {
        return Err(EngineError::Validation(
            "member_identifier must be included in signing_participants".to_string(),
        ));
    }

    Ok(signing_participants)
}

fn build_real_signature_share_contribution(
    dkg_key_packages: &BTreeMap<u16, frost::keys::KeyPackage>,
    signing_participants: &[u16],
    request: &StartSignRoundRequest,
    round_id: &str,
    message_bytes: &[u8],
) -> Result<RoundContribution, EngineError> {
    let mut commitments = BTreeMap::new();
    let mut own_nonces = None;

    for participant_identifier in signing_participants {
        let key_package = dkg_key_packages
            .get(participant_identifier)
            .ok_or_else(|| {
                EngineError::Internal(format!(
                    "missing DKG key package for signing participant [{}]",
                    participant_identifier
                ))
            })?;
        let frost_identifier = participant_identifier_to_frost_identifier(*participant_identifier)?;
        let (mut nonces, participant_commitments) = build_deterministic_round_nonce_and_commitment(
            key_package,
            &request.session_id,
            round_id,
            message_bytes,
            *participant_identifier,
        );
        commitments.insert(frost_identifier, participant_commitments);

        if *participant_identifier == request.member_identifier {
            // `SigningNonces` derives `ZeroizeOnDrop`; if a later `?` returns
            // early in this function, this cached own nonce is still wiped
            // when `own_nonces` drops during unwind of the error path.
            own_nonces = Some(nonces);
        } else {
            nonces.zeroize();
        }
    }

    let mut own_nonces = own_nonces.ok_or_else(|| {
        EngineError::Validation(
            "member_identifier is missing from generated participant nonces".to_string(),
        )
    })?;

    let own_key_package = dkg_key_packages
        .get(&request.member_identifier)
        .ok_or_else(|| {
            EngineError::Validation(
                "member_identifier key package is missing from DKG cache".to_string(),
            )
        })?;

    let signing_package = frost::SigningPackage::new(commitments, message_bytes);
    let signature_share_result =
        frost::round2::sign(&signing_package, &own_nonces, own_key_package);
    own_nonces.zeroize();
    let signature_share = signature_share_result
        .map_err(|e| EngineError::Internal(format!("failed to create signature share: {e}")))?;

    let mut signature_share_bytes = signature_share.serialize();
    let signature_share_hex = hex::encode(&signature_share_bytes);
    signature_share_bytes.zeroize();

    Ok(RoundContribution {
        identifier: request.member_identifier,
        signature_share_hex,
    })
}

pub fn finalize_sign_round(
    request: FinalizeSignRoundRequest,
    bootstrap_mode_enabled: bool,
) -> Result<SignatureResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.finalize_sign_round_calls_total =
            telemetry.finalize_sign_round_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::FinalizeSignRound);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;
    let strict_roast_mode_enabled = roast_strict_mode_enabled();

    let request_fingerprint = {
        let mut canonical_attempt_context = request.attempt_context.clone();
        canonicalize_attempt_context_for_fingerprint(&mut canonical_attempt_context);

        let mut canonical_contributions = request.round_contributions.clone();
        canonical_contributions.sort_unstable_by(|left, right| {
            left.identifier
                .cmp(&right.identifier)
                .then_with(|| left.signature_share_hex.cmp(&right.signature_share_hex))
        });

        fingerprint(&FinalizeSignRoundRequest {
            session_id: request.session_id.clone(),
            round_contributions: canonical_contributions,
            attempt_context: canonical_attempt_context,
        })?
    };
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;

    let session = guard.sessions.get_mut(&request.session_id).ok_or_else(|| {
        EngineError::SessionNotFound {
            session_id: request.session_id.clone(),
        }
    })?;
    if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
        return Err(EngineError::LifecyclePolicyRejected {
            session_id: request.session_id.clone(),
            reason_code: "emergency_rekey_required".to_string(),
            detail: format!(
                "finalize blocked: emergency rekey required since [{}]: {}",
                emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
            ),
        });
    }

    if session.round_state.is_none() {
        session.active_attempt_context = None;
    }

    let canonical_attempt_context = request
        .attempt_context
        .as_ref()
        .map(canonical_attempt_context);
    if let Some(active_attempt_context) = session.active_attempt_context.as_ref() {
        enforce_active_attempt_context_match(
            active_attempt_context,
            canonical_attempt_context.as_ref(),
            None,
            session.round_state.as_ref(),
            session.sign_request_fingerprint.as_deref(),
            strict_roast_mode_enabled,
        )?;
    }

    if let Some(existing) = &session.finalize_request_fingerprint {
        if existing == &request_fingerprint {
            return session.signature_result.clone().ok_or_else(|| {
                EngineError::Internal("missing finalize signature cache".to_string())
            });
        }

        return Err(EngineError::SessionConflict {
            session_id: request.session_id,
        });
    }
    if session
        .consumed_finalize_request_fingerprints
        .contains(&request_fingerprint)
    {
        return Err(EngineError::Validation(format!(
            "finalize request fingerprint [{}] already consumed in session [{}]",
            request_fingerprint, request.session_id
        )));
    }

    let round_state =
        session
            .round_state
            .clone()
            .ok_or_else(|| EngineError::SignRoundNotStarted {
                session_id: request.session_id.clone(),
            })?;
    if signing_policy_firewall_enforced() {
        let sign_message_hex = session
            .sign_message_bytes
            .as_ref()
            .map(|bytes| hex::encode(bytes.as_slice()))
            .ok_or_else(|| EngineError::Internal("missing sign message cache".to_string()))?;
        enforce_signing_message_binding_to_policy_checked_build_tx(
            &request.session_id,
            &sign_message_hex,
            session.tx_result.as_ref(),
        )?;
    }
    // This consumed-round check depends on `round_state` being present to
    // recover `round_id`. If prior finalize already purged round_state,
    // SignRoundNotStarted fails closed before this branch.
    if session
        .consumed_finalize_round_ids
        .contains(&round_state.round_id)
    {
        return Err(EngineError::Validation(format!(
            "round_id [{}] already consumed for finalize in session [{}]",
            round_state.round_id, request.session_id
        )));
    }

    if request.round_contributions.is_empty() {
        return Err(EngineError::Validation(
            "round_contributions must not be empty".to_string(),
        ));
    }

    if request.round_contributions.len() < usize::from(round_state.required_contributions) {
        return Err(EngineError::Validation(format!(
            "insufficient round contributions: expected at least {}",
            round_state.required_contributions
        )));
    }

    if let Some(canonical_attempt_signing_participants) = validate_attempt_context(
        &request.session_id,
        &round_state.message_digest_hex,
        round_state.required_contributions,
        request.attempt_context.as_ref(),
        strict_roast_mode_enabled,
    )? {
        let mut canonical_round_signing_participants =
            round_state.signing_participants.clone().ok_or_else(|| {
                EngineError::Internal(
                    "missing round signing participants for attempt context validation".to_string(),
                )
            })?;
        canonical_round_signing_participants.sort_unstable();
        canonical_round_signing_participants.dedup();
        if canonical_attempt_signing_participants != canonical_round_signing_participants {
            return Err(EngineError::Validation(
                "attempt_context.included_participants must match round signing participants"
                    .to_string(),
            ));
        }
    }

    let mut ordered_contributions = request.round_contributions;
    ordered_contributions.sort_by_key(|contribution| contribution.identifier);
    let is_synthetic = uses_bootstrap_synthetic_contributions(&round_state, &ordered_contributions);

    if !bootstrap_mode_enabled && is_synthetic {
        return Err(EngineError::SyntheticContributionRejected {
            session_id: request.session_id,
        });
    }

    let signature_result = if is_synthetic {
        build_bootstrap_synthetic_signature_result(
            &request.session_id,
            &round_state,
            &ordered_contributions,
        )?
    } else {
        let dkg_key_packages = session
            .dkg_key_packages
            .as_ref()
            .ok_or_else(|| EngineError::Internal("missing DKG key package cache".to_string()))?;

        let dkg_public_key_package = session.dkg_public_key_package.as_ref().ok_or_else(|| {
            EngineError::Internal("missing DKG public key package cache".to_string())
        })?;

        let sign_message_bytes = session
            .sign_message_bytes
            .as_ref()
            .ok_or_else(|| EngineError::Internal("missing sign message cache".to_string()))?;

        let signing_participants = round_state
            .signing_participants
            .clone()
            .unwrap_or_else(|| dkg_key_packages.keys().copied().collect());

        let mut signing_participant_set = HashSet::new();
        for signing_participant in &signing_participants {
            if !signing_participant_set.insert(*signing_participant) {
                return Err(EngineError::Internal(format!(
                    "duplicate signing participant identifier [{}] in round state",
                    signing_participant
                )));
            }
        }

        let mut commitments = BTreeMap::new();
        for signing_participant in &signing_participants {
            let key_package = dkg_key_packages.get(signing_participant).ok_or_else(|| {
                EngineError::Internal(format!(
                    "missing DKG key package for signing participant [{}]",
                    signing_participant
                ))
            })?;
            let frost_identifier =
                participant_identifier_to_frost_identifier(*signing_participant)?;
            let (mut participant_nonces, participant_commitments) =
                build_deterministic_round_nonce_and_commitment(
                    key_package,
                    &round_state.session_id,
                    &round_state.round_id,
                    sign_message_bytes,
                    *signing_participant,
                );
            participant_nonces.zeroize();
            commitments.insert(frost_identifier, participant_commitments);
        }

        let mut contributing_identifiers = Vec::with_capacity(ordered_contributions.len());
        let mut signature_shares = BTreeMap::new();
        for contribution in &ordered_contributions {
            if !signing_participant_set.contains(&contribution.identifier) {
                return Err(EngineError::Validation(format!(
                    "round contribution identifier [{}] is not in signing participant set",
                    contribution.identifier
                )));
            }

            let frost_identifier =
                participant_identifier_to_frost_identifier(contribution.identifier)?;

            if signature_shares.contains_key(&frost_identifier) {
                return Err(EngineError::Validation(format!(
                    "duplicate round contribution identifier [{}]",
                    contribution.identifier
                )));
            }

            let mut signature_share_bytes = hex::decode(&contribution.signature_share_hex)
                .map_err(|_| {
                    EngineError::Validation(format!(
                        "invalid signature_share_hex for identifier [{}]",
                        contribution.identifier
                    ))
                })?;
            let signature_share_result =
                frost::round2::SignatureShare::deserialize(&signature_share_bytes);
            signature_share_bytes.zeroize();
            let signature_share = signature_share_result.map_err(|e| {
                EngineError::Validation(format!(
                    "invalid signature share for identifier [{}]: {e}",
                    contribution.identifier
                ))
            })?;

            contributing_identifiers.push(contribution.identifier);
            signature_shares.insert(frost_identifier, signature_share);
        }

        if contributing_identifiers.len() != signing_participants.len() {
            return Err(EngineError::Validation(format!(
                "round contribution identifiers must match signing participants for real finalize: expected {:?}, got {:?}",
                signing_participants, contributing_identifiers
            )));
        }

        let signing_package = frost::SigningPackage::new(commitments, sign_message_bytes);
        let signature =
            frost::aggregate(&signing_package, &signature_shares, dkg_public_key_package).map_err(
                |e| EngineError::Validation(format!("failed to aggregate signature shares: {e}")),
            )?;
        let signature_bytes = signature.serialize().map_err(|e| {
            EngineError::Internal(format!("failed to serialize aggregate signature: {e}"))
        })?;

        SignatureResult {
            session_id: request.session_id.clone(),
            round_id: round_state.round_id.clone(),
            signature_hex: hex::encode(signature_bytes),
        }
    };

    let consumed_round_id = round_state.round_id.clone();
    ensure_consumed_registry_insert_capacity(
        &session.consumed_finalize_round_ids,
        &consumed_round_id,
        "consumed_finalize_round_ids",
        &request.session_id,
    )?;
    ensure_consumed_registry_insert_capacity(
        &session.consumed_finalize_request_fingerprints,
        &request_fingerprint,
        "consumed_finalize_request_fingerprints",
        &request.session_id,
    )?;

    session.finalize_request_fingerprint = Some(request_fingerprint.clone());
    session.signature_result = Some(signature_result.clone());
    session
        .consumed_finalize_round_ids
        .insert(consumed_round_id);
    session
        .consumed_finalize_request_fingerprints
        .insert(request_fingerprint);
    clear_session_signing_material(session);
    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.finalize_sign_round_success_total = telemetry
            .finalize_sign_round_success_total
            .saturating_add(1);
    });

    Ok(signature_result)
}

fn build_bootstrap_synthetic_signature_result(
    session_id: &str,
    round_state: &RoundState,
    ordered_contributions: &[RoundContribution],
) -> Result<SignatureResult, EngineError> {
    let mut contribution_bytes = serde_json::to_vec(ordered_contributions)
        .map_err(|e| EngineError::Internal(format!("failed to encode contributions: {e}")))?;
    let mut contribution_hash = hash_hex(&contribution_bytes);
    contribution_bytes.zeroize();

    let mut signature_material = format!(
        "signature:{}:{}:{}",
        round_state.session_id, round_state.round_id, contribution_hash
    );
    contribution_hash.zeroize();
    let signature_hex = hash_hex(signature_material.as_bytes());
    signature_material.zeroize();

    Ok(SignatureResult {
        session_id: session_id.to_string(),
        round_id: round_state.round_id.clone(),
        signature_hex,
    })
}

fn uses_bootstrap_synthetic_contributions(
    round_state: &RoundState,
    contributions: &[RoundContribution],
) -> bool {
    contributions.iter().all(|contribution| {
        contribution
            .signature_share_hex
            .eq_ignore_ascii_case(&bootstrap_synthetic_share_hex(
                round_state,
                contribution.identifier,
            ))
    })
}

fn bootstrap_synthetic_share_hex(round_state: &RoundState, identifier: u16) -> String {
    bootstrap_synthetic_share_hex_for_round(
        &round_state.session_id,
        &round_state.round_id,
        &round_state.message_digest_hex,
        identifier,
    )
}

fn bootstrap_synthetic_share_hex_for_round(
    session_id: &str,
    round_id: &str,
    message_digest_hex: &str,
    identifier: u16,
) -> String {
    hash_hex(
        format!(
            "{}:{}:{}:{}:{}",
            BOOTSTRAP_SYNTHETIC_CONTRIBUTION_DOMAIN,
            session_id,
            round_id,
            message_digest_hex,
            identifier,
        )
        .as_bytes(),
    )
}

pub fn build_taproot_tx(request: BuildTaprootTxRequest) -> Result<TransactionResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.build_taproot_tx_calls_total =
            telemetry.build_taproot_tx_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::BuildTaprootTx);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    if request.inputs.is_empty() {
        return Err(EngineError::Validation(
            "inputs must not be empty".to_string(),
        ));
    }

    if request.outputs.is_empty() {
        return Err(EngineError::Validation(
            "outputs must not be empty".to_string(),
        ));
    }

    if request.script_tree_hex.is_some() {
        return Err(EngineError::Validation(
            "script_tree_hex is not yet supported; provide fully-derived output script_pubkey_hex values".to_string(),
        ));
    }

    let request_fingerprint = fingerprint(&request)?;
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;

    if let Some(session) = guard.sessions.get(&request.session_id) {
        if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
            return Err(EngineError::LifecyclePolicyRejected {
                session_id: request.session_id.clone(),
                reason_code: "emergency_rekey_required".to_string(),
                detail: format!(
                    "build_taproot_tx blocked: emergency rekey required since [{}]: {}",
                    emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
                ),
            });
        }

        if let Some(existing) = &session.build_tx_request_fingerprint {
            if existing == &request_fingerprint {
                let cached_result = session
                    .tx_result
                    .clone()
                    .ok_or_else(|| EngineError::Internal("missing build tx cache".to_string()))?;
                let cached_tx_bytes = hex::decode(&cached_result.tx_hex).map_err(|_| {
                    EngineError::Internal("cached build tx hex is not valid hex".to_string())
                })?;
                let cached_tx: Transaction = deserialize(&cached_tx_bytes).map_err(|_| {
                    EngineError::Internal(
                        "cached build tx hex is not a valid transaction".to_string(),
                    )
                })?;
                let total_output_value_sats =
                    cached_tx.output.iter().try_fold(0u64, |total, output| {
                        total.checked_add(output.value.to_sat()).ok_or_else(|| {
                            EngineError::Internal(
                                "cached build tx output total overflowed u64 bounds".to_string(),
                            )
                        })
                    })?;
                if total_output_value_sats > BITCOIN_MAX_MONEY_SATS {
                    return Err(EngineError::Internal(format!(
                        "cached build tx output total [{}] exceeds Bitcoin max money [{}]",
                        total_output_value_sats, BITCOIN_MAX_MONEY_SATS
                    )));
                }

                // Idempotent retries return the cached transaction without consuming a
                // new rate-limit token, but still re-check current non-rate policy gates.
                recheck_signing_policy_firewall_without_rate_limit(
                    &request.session_id,
                    &cached_tx.output,
                    total_output_value_sats,
                )?;
                return Ok(cached_result);
            }

            return Err(EngineError::SessionConflict {
                session_id: request.session_id,
            });
        }
    }
    ensure_session_insert_capacity(&guard.sessions, &request.session_id)?;

    // BuildTaprootTx is an assembly-only step. `input.value_sats` values are
    // trusted caller-supplied metadata and are not verified against chain state.
    let mut total_input_value_sats = 0u64;
    let mut seen_input_keys = HashSet::new();
    let mut inputs = Vec::with_capacity(request.inputs.len());
    for input in request.inputs {
        if input.value_sats > BITCOIN_MAX_MONEY_SATS {
            return Err(EngineError::Validation(format!(
                "input value_sats [{}] exceeds Bitcoin max money [{}]",
                input.value_sats, BITCOIN_MAX_MONEY_SATS
            )));
        }

        total_input_value_sats = total_input_value_sats
            .checked_add(input.value_sats)
            .ok_or_else(|| {
                EngineError::Validation("input value_sats total overflowed u64 bounds".to_string())
            })?;
        if total_input_value_sats > BITCOIN_MAX_MONEY_SATS {
            return Err(EngineError::Validation(format!(
                "input value_sats total [{}] exceeds Bitcoin max money [{}]",
                total_input_value_sats, BITCOIN_MAX_MONEY_SATS
            )));
        }

        let txid = Txid::from_str(&input.txid_hex).map_err(|_| {
            EngineError::Validation(format!("invalid input txid_hex [{}]", input.txid_hex))
        })?;
        let input_key = format!("{txid}:{}", input.vout);
        if !seen_input_keys.insert(input_key.clone()) {
            return Err(EngineError::Validation(format!(
                "duplicate input outpoint [{}]",
                input_key
            )));
        }

        let previous_output = OutPoint {
            txid,
            vout: input.vout,
        };

        inputs.push(TxIn {
            previous_output,
            script_sig: ScriptBuf::new(),
            // Use final sequence for deterministic non-RBF transaction assembly.
            sequence: Sequence::MAX,
            witness: Witness::new(),
        });
    }

    let mut total_output_value_sats = 0u64;
    let mut outputs = Vec::with_capacity(request.outputs.len());
    for output in request.outputs {
        if output.value_sats > BITCOIN_MAX_MONEY_SATS {
            return Err(EngineError::Validation(format!(
                "output value_sats [{}] exceeds Bitcoin max money [{}]",
                output.value_sats, BITCOIN_MAX_MONEY_SATS
            )));
        }

        total_output_value_sats = total_output_value_sats
            .checked_add(output.value_sats)
            .ok_or_else(|| {
                EngineError::Validation("output value_sats total overflowed u64 bounds".to_string())
            })?;
        if total_output_value_sats > BITCOIN_MAX_MONEY_SATS {
            return Err(EngineError::Validation(format!(
                "output value_sats total [{}] exceeds Bitcoin max money [{}]",
                total_output_value_sats, BITCOIN_MAX_MONEY_SATS
            )));
        }

        let script_pubkey_bytes = hex::decode(&output.script_pubkey_hex).map_err(|_| {
            EngineError::Validation(format!(
                "invalid output script_pubkey_hex [{}]",
                output.script_pubkey_hex
            ))
        })?;
        let script_pubkey = ScriptBuf::from_bytes(script_pubkey_bytes);
        if let Some(script_error) = script_pubkey
            .instructions()
            .find_map(|instruction| instruction.err())
        {
            return Err(EngineError::Validation(format!(
                "invalid output script_pubkey_hex [{}]: {script_error}",
                output.script_pubkey_hex
            )));
        }
        outputs.push(TxOut {
            value: Amount::from_sat(output.value_sats),
            script_pubkey,
        });
    }

    if total_output_value_sats > total_input_value_sats {
        return Err(EngineError::Validation(format!(
            "output value_sats total [{}] exceeds input value_sats total [{}]",
            total_output_value_sats, total_input_value_sats
        )));
    }
    enforce_signing_policy_firewall(&request.session_id, &outputs, total_output_value_sats)?;

    let tx = Transaction {
        // Version 2 + zero locktime are bootstrap defaults for immediate-spend txs.
        version: Version::TWO,
        lock_time: LockTime::ZERO,
        input: inputs,
        output: outputs,
    };

    let result = TransactionResult {
        session_id: request.session_id,
        tx_hex: serialize_hex(&tx),
    };

    // BuildTaprootTx is keyed into the shared session namespace for idempotency
    // caching only; this session entry may intentionally not have DKG/signing
    // state populated.
    let session = guard
        .sessions
        .entry(result.session_id.clone())
        .or_insert_with(SessionState::default);
    session.build_tx_request_fingerprint = Some(request_fingerprint);
    session.tx_result = Some(result.clone());
    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.build_taproot_tx_success_total =
            telemetry.build_taproot_tx_success_total.saturating_add(1);
    });

    Ok(result)
}

pub fn refresh_shares(request: RefreshSharesRequest) -> Result<RefreshSharesResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.refresh_shares_calls_total =
            telemetry.refresh_shares_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::RefreshShares);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    if request.current_shares.is_empty() {
        return Err(EngineError::Validation(
            "current_shares must not be empty".to_string(),
        ));
    }
    let mut unique_share_identifiers = HashSet::new();
    for share in &request.current_shares {
        if share.identifier == 0 {
            return Err(EngineError::Validation(
                "current_shares identifiers must be non-zero".to_string(),
            ));
        }
        if !unique_share_identifiers.insert(share.identifier) {
            return Err(EngineError::Validation(format!(
                "current_shares contains duplicate identifier [{}]",
                share.identifier
            )));
        }
    }

    let request_fingerprint = fingerprint(&canonicalize_refresh_shares_request_for_fingerprint(
        &request,
    ))?;
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;

    if let Some(session) = guard.sessions.get(&request.session_id) {
        if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
            return Err(EngineError::LifecyclePolicyRejected {
                session_id: request.session_id.clone(),
                reason_code: "emergency_rekey_required".to_string(),
                detail: format!(
                    "refresh blocked: emergency rekey required since [{}]: {}",
                    emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
                ),
            });
        }

        if let Some(existing) = &session.refresh_request_fingerprint {
            if existing == &request_fingerprint {
                return session
                    .refresh_result
                    .clone()
                    .ok_or_else(|| EngineError::Internal("missing refresh cache".to_string()));
            }

            return Err(EngineError::SessionConflict {
                session_id: request.session_id,
            });
        }
    }
    ensure_session_insert_capacity(&guard.sessions, &request.session_id)?;

    let mut new_shares: Vec<ShareMaterial> = request
        .current_shares
        .into_iter()
        .map(|share| ShareMaterial {
            identifier: share.identifier,
            encrypted_share_hex: hash_hex(
                format!(
                    "refresh:{}:{}:{}",
                    request.session_id, share.identifier, share.encrypted_share_hex
                )
                .as_bytes(),
            ),
        })
        .collect();

    new_shares.sort_by_key(|share| share.identifier);

    guard.refresh_epoch_counter = guard.refresh_epoch_counter.saturating_add(1);
    let refresh_epoch = guard.refresh_epoch_counter;

    let result = RefreshSharesResult {
        session_id: request.session_id,
        refresh_epoch,
        new_shares,
    };

    let session = guard
        .sessions
        .entry(result.session_id.clone())
        .or_insert_with(SessionState::default);
    if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
        return Err(EngineError::LifecyclePolicyRejected {
            session_id: result.session_id.clone(),
            reason_code: "emergency_rekey_required".to_string(),
            detail: format!(
                "refresh blocked: emergency rekey required since [{}]: {}",
                emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
            ),
        });
    }
    session.refresh_request_fingerprint = Some(request_fingerprint);
    session.refresh_result = Some(result.clone());
    session.refresh_history.push(RefreshHistoryRecord {
        refresh_epoch,
        refreshed_at_unix: now_unix(),
        share_count: result.new_shares.len().min(u16::MAX as usize) as u16,
        key_group: session.dkg_result.as_ref().map(|dkg| dkg.key_group.clone()),
    });
    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.refresh_shares_success_total =
            telemetry.refresh_shares_success_total.saturating_add(1);
    });

    Ok(result)
}

#[cfg(test)]
static TEST_MUTEX: OnceLock<Mutex<()>> = OnceLock::new();

#[cfg(test)]
pub fn lock_test_state() -> std::sync::MutexGuard<'static, ()> {
    let guard = TEST_MUTEX
        .get_or_init(|| Mutex::new(()))
        .lock()
        .expect("test lock should not be poisoned");
    // Pin the signer profile to development at lock acquisition. Tests that
    // need to exercise production-mode behavior set the env explicitly after
    // taking the lock; doing this here prevents one test's `set_var` from
    // leaking into the next locked test's body and (for example) routing the
    // encrypted-state-envelope proptest into the production-rejects-env-key-
    // provider gate that #414 introduced.
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_DEVELOPMENT);
    guard
}

#[cfg(test)]
pub fn reset_for_tests() {
    clear_persist_fault_injection_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
    );
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV);
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV);
    // Tests default to the explicit development profile so the production-safe
    // missing-env default does not route unrelated tests through production
    // policy gates.
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_DEVELOPMENT);
    std::env::set_var(
        TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
        TEST_STATE_ENCRYPTION_KEY_HEX,
    );

    if let Ok(mut lock_slot) = state_file_lock_slot().lock() {
        *lock_slot = None;
    }
    if let Ok(mut telemetry) = hardening_telemetry_state().lock() {
        *telemetry = HardeningTelemetryState::default();
    }
    if let Ok(mut limiter) = build_tx_rate_limiter_state().lock() {
        *limiter = BuildTxRateLimiterState::default();
    }

    if let Ok(state) = state() {
        if let Ok(mut guard) = state.lock() {
            guard.sessions.clear();
            guard.refresh_epoch_counter = 0;
            guard.operator_fault_scores.clear();
            guard.quarantined_operator_identifiers.clear();
            guard.canary_rollout = CanaryRolloutState::default();
            let _ = persist_engine_state_to_storage(&guard);
        }
    }
}

#[cfg(test)]
pub fn reload_state_from_storage_for_tests() {
    let loaded_state = load_engine_state_from_storage().expect("load engine state from storage");
    let state = state().expect("engine state should initialize");
    let mut guard = state.lock().expect("engine lock");
    *guard = loaded_state;
}

#[cfg(test)]
pub fn simulate_process_restart_for_tests() {
    if let Ok(mut lock_slot) = state_file_lock_slot().lock() {
        *lock_slot = None;
    }

    if let Some(state) = ENGINE_STATE.get() {
        if let Ok(mut guard) = state.lock() {
            guard.sessions.clear();
            guard.refresh_epoch_counter = 0;
            guard.operator_fault_scores.clear();
            guard.quarantined_operator_identifiers.clear();
            guard.canary_rollout = CanaryRolloutState::default();
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use proptest::prelude::*;
    use serde::Deserialize;
    #[cfg(unix)]
    use std::os::unix::fs::PermissionsExt;
    use std::path::{Path, PathBuf};
    #[cfg(unix)]
    use std::{
        process::Command,
        thread,
        time::{Duration, Instant},
    };

    #[derive(Deserialize)]
    struct AttemptContextVectorDomains {
        included_participants_fingerprint: String,
        attempt_id: String,
    }

    #[derive(Deserialize)]
    struct AttemptContextVector {
        id: String,
        session_id: String,
        message_digest_hex: String,
        attempt_number: u32,
        coordinator_identifier: u16,
        included_participants: Vec<u16>,
        expected_included_participants_fingerprint: String,
        expected_attempt_id: String,
    }

    #[derive(Deserialize)]
    struct AttemptContextVectorSuite {
        schema_version: String,
        hash_domains: AttemptContextVectorDomains,
        vectors: Vec<AttemptContextVector>,
    }

    fn load_attempt_context_vector_suite() -> AttemptContextVectorSuite {
        let vectors_path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("test/vectors/roast-attempt-context-v1.json");
        let vector_bytes = std::fs::read(&vectors_path).unwrap_or_else(|err| {
            panic!(
                "failed to read attempt-context vector file [{}]: {err}",
                vectors_path.display()
            )
        });

        serde_json::from_slice(&vector_bytes).expect("attempt-context vectors decode")
    }

    struct InteractiveDkgFixture {
        pre_normalization_even_y: bool,
        part3_requests: BTreeMap<u16, DkgPart3Request>,
    }

    fn deterministic_interactive_dkg_fixture(seed: u8) -> InteractiveDkgFixture {
        let participant_ids = [1u16, 2, 3];
        let participant_identifiers: BTreeMap<u16, frost::Identifier> = participant_ids
            .iter()
            .copied()
            .map(|id| {
                (
                    id,
                    participant_identifier_to_frost_identifier(id).expect("participant identifier"),
                )
            })
            .collect();
        let participant_id_by_identifier_hex: BTreeMap<String, u16> = participant_identifiers
            .iter()
            .map(|(id, identifier)| (hex::encode(identifier.serialize()), *id))
            .collect();

        let mut part1_secrets = BTreeMap::new();
        let mut part1_packages = BTreeMap::new();
        for id in participant_ids {
            let mut rng_seed = [0u8; 32];
            rng_seed[0] = seed;
            rng_seed[1..3].copy_from_slice(&id.to_be_bytes());
            let rng = ZeroizingChaCha20Rng::from_seed(rng_seed);
            let (secret_package, package) = frost::keys::dkg::part1(
                participant_identifiers[&id],
                participant_ids.len() as u16,
                2,
                rng,
            )
            .expect("DKG part1");

            part1_secrets.insert(id, secret_package);
            part1_packages.insert(
                id,
                DkgRound1Package {
                    identifier: frost_identifier_to_go_string(participant_identifiers[&id]),
                    package_hex: hex::encode(package.serialize().expect("round1 package")),
                },
            );
        }

        let round1_packages_for = |recipient_id: u16| -> Vec<DkgRound1Package> {
            participant_ids
                .iter()
                .copied()
                .filter(|id| *id != recipient_id)
                .map(|id| part1_packages[&id].clone())
                .collect()
        };

        let mut part2_secrets = BTreeMap::new();
        let mut round2_packages_by_recipient: BTreeMap<u16, Vec<DkgRound2Package>> =
            BTreeMap::new();
        for sender_id in participant_ids {
            let round1_packages =
                decode_round1_package_map("TestDKGPart2", &round1_packages_for(sender_id))
                    .expect("round1 package map");
            let (round2_secret, round2_packages) = frost::keys::dkg::part2(
                part1_secrets
                    .remove(&sender_id)
                    .expect("part1 secret package"),
                &round1_packages,
            )
            .expect("DKG part2");

            part2_secrets.insert(sender_id, round2_secret);
            for (recipient_identifier, package) in round2_packages {
                let recipient_id = participant_id_by_identifier_hex
                    .get(&hex::encode(recipient_identifier.serialize()))
                    .copied()
                    .expect("recipient identifier mapping");
                round2_packages_by_recipient
                    .entry(recipient_id)
                    .or_default()
                    .push(DkgRound2Package {
                        identifier: frost_identifier_to_go_string(recipient_identifier),
                        sender_identifier: Some(frost_identifier_to_go_string(
                            participant_identifiers[&sender_id],
                        )),
                        package_hex: hex::encode(package.serialize().expect("round2 package")),
                    });
            }
        }

        let first_participant = participant_ids[0];
        let round1_packages =
            decode_round1_package_map("TestDKGPart3", &round1_packages_for(first_participant))
                .expect("round1 package map");
        let round2_packages = decode_round2_package_map(
            "TestDKGPart3",
            &round2_packages_by_recipient[&first_participant],
            Some(participant_identifiers[&first_participant]),
        )
        .expect("round2 package map");
        let (_, pre_normalization_public_key_package) = frost::keys::dkg::part3(
            part2_secrets
                .get(&first_participant)
                .expect("round2 secret package"),
            &round1_packages,
            &round2_packages,
        )
        .expect("DKG part3");

        let mut part3_requests = BTreeMap::new();
        for id in participant_ids {
            let secret_package = part2_secrets.get(&id).expect("round2 secret package");
            let secret_package_bytes = secret_package.serialize().expect("round2 secret");
            part3_requests.insert(
                id,
                DkgPart3Request {
                    secret_package_hex: hex::encode(secret_package_bytes),
                    round1_packages: round1_packages_for(id),
                    round2_packages: round2_packages_by_recipient
                        .get(&id)
                        .expect("round2 packages")
                        .clone(),
                },
            );
        }

        InteractiveDkgFixture {
            pre_normalization_even_y: pre_normalization_public_key_package.has_even_y(),
            part3_requests,
        }
    }

    fn deterministic_odd_y_interactive_dkg_fixture() -> InteractiveDkgFixture {
        for seed in 0u8..=u8::MAX {
            let fixture = deterministic_interactive_dkg_fixture(seed);
            if !fixture.pre_normalization_even_y {
                return fixture;
            }
        }

        panic!("could not find deterministic odd-Y DKG fixture");
    }

    #[test]
    fn dkg_part3_normalizes_odd_y_group_key_and_secret_shares() {
        let _guard = lock_test_state();
        reset_for_tests();

        let fixture = deterministic_odd_y_interactive_dkg_fixture();
        assert!(
            !fixture.pre_normalization_even_y,
            "fixture must exercise the odd-Y normalization branch"
        );

        let mut part3_results = BTreeMap::new();
        for (id, request) in fixture.part3_requests {
            let result = dkg_part3(request).expect("DKG part3");
            let expected_identifier = frost_identifier_to_go_string(
                participant_identifier_to_frost_identifier(id).unwrap(),
            );
            assert_eq!(result.key_package.identifier, expected_identifier);
            assert_eq!(result.public_key_package.verifying_key.len(), 64);
            part3_results.insert(id, result);
        }

        let exported_x_only_key = part3_results[&1].public_key_package.verifying_key.clone();
        for result in part3_results.values() {
            assert_eq!(result.public_key_package.verifying_key, exported_x_only_key);
            assert_eq!(
                result.public_key_package.verifying_shares,
                part3_results[&1].public_key_package.verifying_shares
            );
        }

        let signing_participants = [1u16, 2];
        let mut commitments = Vec::new();
        let mut nonces_by_participant = BTreeMap::new();
        for id in signing_participants {
            let result = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
                key_package_identifier: part3_results[&id].key_package.identifier.clone(),
                key_package_hex: part3_results[&id].key_package.data_hex.clone(),
            })
            .expect("generate nonces");
            commitments.push(result.commitment);
            nonces_by_participant.insert(id, result.nonces_hex);
        }

        let message = [0x42u8; 32];
        let signing_package = new_signing_package(NewSigningPackageRequest {
            message_hex: hex::encode(message),
            commitments,
        })
        .expect("new signing package");

        let mut signature_shares = Vec::new();
        for id in signing_participants {
            let result = sign_share(SignShareRequest {
                signing_package_hex: signing_package.signing_package_hex.clone(),
                nonces_hex: nonces_by_participant
                    .remove(&id)
                    .expect("participant nonces"),
                key_package_identifier: part3_results[&id].key_package.identifier.clone(),
                key_package_hex: part3_results[&id].key_package.data_hex.clone(),
            })
            .expect("sign share");
            signature_shares.push(result.signature_share);
        }

        let aggregate = aggregate(AggregateRequest {
            signing_package_hex: signing_package.signing_package_hex,
            signature_shares,
            public_key_package: part3_results[&1].public_key_package.clone(),
        })
        .expect("aggregate");

        let signature_bytes = hex::decode(aggregate.signature_hex).expect("signature hex");
        let signature = SchnorrSignature::from_slice(&signature_bytes).expect("BIP340 signature");
        let public_key_bytes = hex::decode(exported_x_only_key).expect("verifying key hex");
        let public_key = XOnlyPublicKey::from_slice(&public_key_bytes).expect("x-only public key");
        let message = SecpMessage::from_digest(message);
        Secp256k1::verification_only()
            .verify_schnorr(&signature, &message, &public_key)
            .expect("aggregate verifies under normalized x-only key");
    }

    fn seeded_round_state(session_id: &str) -> RoundState {
        let run_dkg_request = RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let start_request = StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };

        start_sign_round(start_request).expect("start sign round")
    }

    fn configure_test_state_path(suffix: &str) -> PathBuf {
        let path = std::env::temp_dir().join(format!(
            "frost_tbtc_engine_state_{suffix}_{}.json",
            std::process::id()
        ));
        clear_state_storage_policy_overrides();
        cleanup_test_state_artifacts(&path);
        std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &path);
        path
    }

    fn clear_state_storage_policy_overrides() {
        std::env::remove_var(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV);
        std::env::remove_var(TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT_ENV);
        std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
        std::env::remove_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV);
        std::env::remove_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV);
        std::env::remove_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV);
        std::env::remove_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV);
        std::env::remove_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV);
        std::env::remove_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV);
        std::env::remove_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV);
        std::env::remove_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV);
        std::env::remove_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV);
        std::env::remove_var(TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS_ENV);
        std::env::remove_var(TBTC_SIGNER_ADMISSION_MIN_THRESHOLD_ENV);
        std::env::remove_var(TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS_ENV);
        std::env::remove_var(TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV);
        std::env::remove_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV);
        std::env::remove_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV);
        std::env::remove_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV);
        std::env::remove_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV);
        std::env::remove_var(TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV);
        std::env::remove_var(TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV);
        std::env::remove_var(TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV);
        std::env::remove_var(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV);
        std::env::remove_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV);
        std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV);
        std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV);
        std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV);
        std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV);
        std::env::remove_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV);
        std::env::remove_var(TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV);
        std::env::remove_var(TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS_ENV);
        std::env::remove_var(TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV);
        std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV);
        std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV);
        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_DEVELOPMENT);
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
            TEST_STATE_ENCRYPTION_KEY_HEX,
        );
    }

    fn configure_required_signing_policy_limits_for_tests() {
        std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV, "64");
        std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV, "100000000");
        std::env::set_var(
            TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV,
            "2100000000000000",
        );
    }

    fn build_signed_provenance_attestation(
        status: &str,
        runtime_version: &str,
        expires_at_unix: Option<u64>,
    ) -> (String, String, String) {
        let mut payload = serde_json::json!({
            "status": status,
            "runtime_version": runtime_version,
        });
        if let Some(expires_at_unix) = expires_at_unix {
            payload["expires_at_unix"] = serde_json::json!(expires_at_unix);
        }
        let payload = payload.to_string();

        let secp = Secp256k1::new();
        let secret_key =
            bitcoin::secp256k1::SecretKey::from_slice(&[0x11; 32]).expect("secret key");
        let keypair = bitcoin::secp256k1::Keypair::from_secret_key(&secp, &secret_key);
        let (trust_root_pubkey, _) = XOnlyPublicKey::from_keypair(&keypair);

        let payload_digest = Sha256::digest(payload.as_bytes());
        let message = SecpMessage::from_digest_slice(&payload_digest).expect("message digest");
        let signature = secp.sign_schnorr_no_aux_rand(&message, &keypair);

        (
            trust_root_pubkey.to_string(),
            payload,
            signature.to_string(),
        )
    }

    fn configure_valid_provenance_attestation_for_tests() {
        let (trust_root, attestation_payload, attestation_signature) =
            build_signed_provenance_attestation(
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
                TBTC_SIGNER_RUNTIME_VERSION,
                Some(now_unix() + 3600),
            );

        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, trust_root);
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            attestation_payload,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            attestation_signature,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");
    }

    fn cleanup_test_state_artifacts(path: &Path) {
        let _ = std::fs::remove_file(path);
        let _ = std::fs::remove_file(state_lock_file_path(path));
        let _ = std::fs::remove_file(path.with_extension(format!("tmp-{}", std::process::id())));

        if let Ok(backups) = sorted_corrupted_state_backups(path) {
            for backup in backups {
                let _ = std::fs::remove_file(backup);
            }
        }
    }

    fn persisted_session_state_fixture() -> PersistedSessionState {
        PersistedSessionState {
            dkg_request_fingerprint: None,
            dkg_key_packages: None,
            dkg_public_key_package_hex: None,
            dkg_result: None,
            sign_request_fingerprint: None,
            sign_message_hex: None,
            round_state: None,
            active_attempt_context: None,
            attempt_transition_records: vec![],
            consumed_attempt_ids: vec![],
            consumed_sign_round_ids: vec![],
            finalize_request_fingerprint: None,
            signature_result: None,
            consumed_finalize_round_ids: vec![],
            consumed_finalize_request_fingerprints: vec![],
            build_tx_request_fingerprint: None,
            tx_result: None,
            refresh_request_fingerprint: None,
            refresh_result: None,
            refresh_history: vec![],
            emergency_rekey_event: None,
        }
    }

    fn expect_internal_error_contains(err: EngineError, expected_substring: &str) {
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains(expected_substring),
            "unexpected internal error message: {message}"
        );
    }

    fn state_mutation_request(session_id: &str) -> RefreshSharesRequest {
        RefreshSharesRequest {
            session_id: session_id.to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "1111".to_string(),
            }],
        }
    }

    fn mutate_state_for_key_provider_test(
        session_id: &str,
    ) -> Result<RefreshSharesResult, EngineError> {
        refresh_shares(state_mutation_request(session_id))
    }

    #[test]
    fn run_dkg_rejects_bootstrap_dealer_dkg_in_production_profile() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);

        let err = run_dkg(RunDkgRequest {
            session_id: "session-production-bootstrap-dkg".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("production profile should reject bootstrap dealer DKG");

        let EngineError::LifecyclePolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "bootstrap_dealer_dkg_disabled_in_production");
    }

    #[test]
    fn run_dkg_rejects_bootstrap_dealer_dkg_when_profile_is_missing_or_empty() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        for profile_value in [None, Some(" ")] {
            match profile_value {
                Some(value) => std::env::set_var(TBTC_SIGNER_PROFILE_ENV, value),
                None => std::env::remove_var(TBTC_SIGNER_PROFILE_ENV),
            }

            let err = run_dkg(RunDkgRequest {
                session_id: format!(
                    "session-default-production-bootstrap-dkg-{}",
                    profile_value.unwrap_or("missing").trim()
                ),
                participants: vec![
                    crate::api::DkgParticipant {
                        identifier: 1,
                        public_key_hex: "02aa".to_string(),
                    },
                    crate::api::DkgParticipant {
                        identifier: 2,
                        public_key_hex: "02bb".to_string(),
                    },
                ],
                threshold: 2,
            })
            .expect_err("missing/empty profile should reject bootstrap dealer DKG");

            let EngineError::LifecyclePolicyRejected { reason_code, .. } = err else {
                panic!("unexpected error variant");
            };
            assert_eq!(reason_code, "bootstrap_dealer_dkg_disabled_in_production");
        }
    }

    #[test]
    fn production_profile_forces_provenance_gate_without_env_flag() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::remove_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV);
        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
        assert!(provenance_gate_enforced());

        std::env::remove_var(TBTC_SIGNER_PROFILE_ENV);
        assert!(provenance_gate_enforced());

        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_DEVELOPMENT);
        assert!(!provenance_gate_enforced());
    }

    #[test]
    fn run_dkg_rejects_when_provenance_gate_requires_attestation() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, "sigstore-main");
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-gate-missing-attestation".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected provenance gate rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "missing_attestation_status");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn canary_rollout_status_rejects_when_provenance_gate_requires_attestation() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");

        let err = canary_rollout_status().expect_err("expected provenance gate rejection");
        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "missing_attestation_status");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_accepts_valid_signed_provenance_attestation() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let (trust_root, attestation_payload, attestation_signature_hex) =
            build_signed_provenance_attestation(
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
                TBTC_SIGNER_RUNTIME_VERSION,
                Some(now_unix().saturating_add(300)),
            );

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            &attestation_payload,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            &attestation_signature_hex,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let result = run_dkg(RunDkgRequest {
            session_id: "session-provenance-signed-attestation-accept".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        });
        assert!(result.is_ok(), "expected signed attestation acceptance");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_when_provenance_attestation_signature_missing() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let (trust_root, attestation_payload, _) = build_signed_provenance_attestation(
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
            TBTC_SIGNER_RUNTIME_VERSION,
            Some(now_unix().saturating_add(300)),
        );

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            &attestation_payload,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-signed-attestation-missing-signature".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected missing signature rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "missing_attestation_signature");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_when_provenance_attestation_signature_invalid() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let (trust_root, attestation_payload, mut attestation_signature_hex) =
            build_signed_provenance_attestation(
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
                TBTC_SIGNER_RUNTIME_VERSION,
                Some(now_unix().saturating_add(300)),
            );
        let replacement = if attestation_signature_hex.ends_with('0') {
            "1"
        } else {
            "0"
        };
        attestation_signature_hex.replace_range(
            attestation_signature_hex.len() - 1..attestation_signature_hex.len(),
            replacement,
        );

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            &attestation_payload,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            &attestation_signature_hex,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-signed-attestation-invalid-signature".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected signature verification rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "attestation_signature_verification_failed");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_when_provenance_attestation_expired() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let (trust_root, attestation_payload, attestation_signature_hex) =
            build_signed_provenance_attestation(
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
                TBTC_SIGNER_RUNTIME_VERSION,
                Some(now_unix().saturating_sub(1)),
            );

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            &attestation_payload,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            &attestation_signature_hex,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-signed-attestation-expired".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected attestation expiry rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "attestation_expired");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_when_provenance_attestation_missing_expiry() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let (trust_root, attestation_payload, attestation_signature_hex) =
            build_signed_provenance_attestation(
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
                TBTC_SIGNER_RUNTIME_VERSION,
                None,
            );

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            &attestation_payload,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            &attestation_signature_hex,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-signed-attestation-missing-expiry".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected attestation missing expiry rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "missing_attestation_expiry");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_when_provenance_attestation_expiry_too_far_in_future() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let (trust_root, attestation_payload, attestation_signature_hex) =
            build_signed_provenance_attestation(
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
                TBTC_SIGNER_RUNTIME_VERSION,
                Some(
                    now_unix().saturating_add(TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS)
                        + 1,
                ),
            );

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            &attestation_payload,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            &attestation_signature_hex,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-expiry-too-far".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected attestation expiry too far rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "attestation_expiry_too_far_in_future");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_when_provenance_trust_root_mismatches_signature_key() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let (_trust_root, attestation_payload, attestation_signature_hex) =
            build_signed_provenance_attestation(
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
                TBTC_SIGNER_RUNTIME_VERSION,
                Some(now_unix().saturating_add(300)),
            );

        let secp = Secp256k1::new();
        let wrong_secret_key =
            bitcoin::secp256k1::SecretKey::from_slice(&[0x22; 32]).expect("secret key");
        let wrong_keypair = bitcoin::secp256k1::Keypair::from_secret_key(&secp, &wrong_secret_key);
        let (wrong_trust_root, _) = XOnlyPublicKey::from_keypair(&wrong_keypair);

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV,
            wrong_trust_root.to_string(),
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            &attestation_payload,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            &attestation_signature_hex,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-wrong-trust-root".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected trust-root mismatch rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "attestation_signature_verification_failed");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_when_signed_attestation_runtime_version_mismatch() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let (trust_root, attestation_payload, attestation_signature_hex) =
            build_signed_provenance_attestation(
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
                "99.99.99",
                Some(now_unix().saturating_add(300)),
            );

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            &attestation_payload,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            &attestation_signature_hex,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-runtime-version-mismatch".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected runtime version mismatch rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "runtime_version_not_attested");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_when_signed_attestation_status_mismatches_env() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let (trust_root, attestation_payload, attestation_signature_hex) =
            build_signed_provenance_attestation(
                "pending",
                TBTC_SIGNER_RUNTIME_VERSION,
                Some(now_unix().saturating_add(300)),
            );

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            &attestation_payload,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            &attestation_signature_hex,
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-status-mismatch".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected status mismatch rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "attestation_status_mismatch");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_invalid_curve_point_trust_root() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        );
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV,
            "0000000000000000000000000000000000000000000000000000000000000000",
        );
        std::env::set_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV, "{}");
        std::env::set_var(
            TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
            "aa".repeat(64),
        );
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-provenance-invalid-curve-point-trust-root".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected invalid trust root rejection");

        let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "invalid_trust_root_format");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn provenance_gate_rejects_runtime_prerelease_for_release_minimum() {
        let runtime_version = parse_version_triplet("1.2.3-rc1").expect("runtime parse");
        let minimum_version = parse_version_triplet("1.2.3").expect("minimum parse");
        assert!(!runtime_satisfies_minimum_version(
            runtime_version,
            minimum_version
        ));

        let runtime_version = parse_version_triplet("1.2.3").expect("runtime parse");
        let minimum_version = parse_version_triplet("1.2.3-rc1").expect("minimum parse");
        assert!(runtime_satisfies_minimum_version(
            runtime_version,
            minimum_version
        ));
    }

    #[test]
    fn run_dkg_rejects_session_id_with_disallowed_characters() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let err = run_dkg(RunDkgRequest {
            session_id: "session-log\ninject".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected session_id validation rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("session_id contains disallowed characters"),
            "unexpected validation message: {message}"
        );

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_non_allowlisted_participant_under_admission_policy() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV, "true");
        std::env::set_var(TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV, "1,2");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-admission-allowlist-reject".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected admission policy rejection");

        let EngineError::AdmissionPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "participant_identifier_not_allowlisted");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_maps_admission_policy_config_error_to_rejection_with_counter() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV, "true");
        std::env::set_var(TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS_ENV, "not-a-number");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-admission-invalid-policy-config".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected admission policy config rejection");

        let EngineError::AdmissionPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "invalid_policy_configuration");

        let metrics = hardening_metrics();
        assert_eq!(metrics.run_dkg_calls_total, 1);
        assert_eq!(metrics.run_dkg_admission_reject_total, 1);
        assert_eq!(metrics.run_dkg_success_total, 0);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_empty_admission_allowlist_as_invalid_config() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV, "true");
        std::env::set_var(TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV, "");

        let err = run_dkg(RunDkgRequest {
            session_id: "session-admission-empty-allowlist".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected admission policy config rejection");

        let EngineError::AdmissionPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "invalid_policy_configuration");

        let metrics = hardening_metrics();
        assert_eq!(metrics.run_dkg_calls_total, 1);
        assert_eq!(metrics.run_dkg_admission_reject_total, 1);

        clear_state_storage_policy_overrides();
    }

    fn build_policy_test_request(session_id: &str) -> BuildTaprootTxRequest {
        BuildTaprootTxRequest {
            session_id: session_id.to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 9_000,
            }],
            script_tree_hex: None,
        }
    }

    fn policy_bound_message_hex_from_tx_result(tx_result: &TransactionResult) -> String {
        let tx_bytes = hex::decode(&tx_result.tx_hex).expect("tx hex");
        hash_hex(&tx_bytes)
    }

    #[test]
    fn build_taproot_tx_signing_policy_firewall_rejects_disallowed_script_class() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
        configure_required_signing_policy_limits_for_tests();

        let err = build_taproot_tx(build_policy_test_request(
            "session-signing-policy-script-class-reject",
        ))
        .expect_err("expected signing policy rejection");

        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "script_class_not_allowlisted");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_signing_policy_firewall_rejects_excess_output_count() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr");
        std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV, "1");
        std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV, "100000000");
        std::env::set_var(
            TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV,
            "2100000000000000",
        );

        let mut request = build_policy_test_request("session-signing-policy-output-count-reject");
        request.inputs[0].value_sats = 20_000;
        request.outputs.push(crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "33".repeat(32)),
            value_sats: 9_000,
        });

        let err =
            build_taproot_tx(request).expect_err("expected signing policy output count rejection");

        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "output_count_exceeds_policy_limit");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_signing_policy_firewall_rejects_excess_single_output_value() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV, "64");
        std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV, "5000");
        std::env::set_var(
            TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV,
            "2100000000000000",
        );

        let err = build_taproot_tx(build_policy_test_request(
            "session-signing-policy-single-output-value-reject",
        ))
        .expect_err("expected signing policy single output value rejection");

        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "single_output_value_exceeds_policy_limit");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_signing_policy_firewall_rejects_excess_total_output_value() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV, "64");
        std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV, "100000000");
        std::env::set_var(TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV, "5000");

        let err = build_taproot_tx(build_policy_test_request(
            "session-signing-policy-total-output-value-reject",
        ))
        .expect_err("expected signing policy total output value rejection");

        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "total_output_value_exceeds_policy_limit");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_rejects_total_input_value_above_bitcoin_max_money() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("build_taproot_tx_max_input_total");
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let request = BuildTaprootTxRequest {
            session_id: "session-build-tx-max-input-total".to_string(),
            inputs: vec![
                crate::api::TxInput {
                    txid_hex: "11".repeat(32),
                    vout: 0,
                    value_sats: BITCOIN_MAX_MONEY_SATS,
                },
                crate::api::TxInput {
                    txid_hex: "22".repeat(32),
                    vout: 0,
                    value_sats: 1,
                },
            ],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "33".repeat(32)),
                value_sats: 1,
            }],
            script_tree_hex: None,
        };

        let err = build_taproot_tx(request).expect_err("expected max money rejection");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant: {err:?}");
        };
        assert!(
            message.contains("input value_sats total")
                && message.contains("exceeds Bitcoin max money"),
            "unexpected validation message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_rejects_total_output_value_above_bitcoin_max_money() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("build_taproot_tx_max_output_total");
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let request = BuildTaprootTxRequest {
            session_id: "session-build-tx-max-output-total".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: BITCOIN_MAX_MONEY_SATS,
            }],
            outputs: vec![
                crate::api::TxOutput {
                    script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                    value_sats: BITCOIN_MAX_MONEY_SATS / 2 + 1,
                },
                crate::api::TxOutput {
                    script_pubkey_hex: format!("5120{}", "33".repeat(32)),
                    value_sats: BITCOIN_MAX_MONEY_SATS / 2 + 1,
                },
            ],
            script_tree_hex: None,
        };

        let err = build_taproot_tx(request).expect_err("expected max money rejection");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant: {err:?}");
        };
        assert!(
            message.contains("output value_sats total")
                && message.contains("exceeds Bitcoin max money"),
            "unexpected validation message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_signing_policy_firewall_rejects_outside_utc_window() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        configure_required_signing_policy_limits_for_tests();
        let current_hour = current_utc_hour();
        let start_hour = (current_hour + 1) % 24;
        let end_hour = (current_hour + 2) % 24;
        std::env::set_var(
            TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV,
            start_hour.to_string(),
        );
        std::env::set_var(
            TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV,
            end_hour.to_string(),
        );

        let err = build_taproot_tx(build_policy_test_request(
            "session-signing-policy-utc-window-reject",
        ))
        .expect_err("expected signing policy UTC window rejection");

        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "request_outside_allowed_utc_window");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn hardening_metrics_tracks_policy_rejections() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
        configure_required_signing_policy_limits_for_tests();

        let _ = build_taproot_tx(build_policy_test_request(
            "session-hardening-metrics-policy-reject",
        ));

        let metrics = hardening_metrics();
        assert_eq!(metrics.build_taproot_tx_calls_total, 1);
        assert_eq!(metrics.build_taproot_tx_policy_reject_total, 1);
        assert_eq!(metrics.build_taproot_tx_success_total, 0);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn hardening_metrics_count_calls_before_provenance_gate_rejection() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
        std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, "sigstore-main");
        std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

        let run_dkg_err = run_dkg(RunDkgRequest {
            session_id: "session-metrics-provenance-run-dkg".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected run_dkg provenance gate rejection");
        assert!(matches!(
            run_dkg_err,
            EngineError::ProvenanceGateRejected { .. }
        ));

        let build_tx_err = build_taproot_tx(BuildTaprootTxRequest {
            session_id: "session-metrics-provenance-build-tx".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("0014{}", "33".repeat(20)),
                value_sats: 9_000,
            }],
            script_tree_hex: None,
        })
        .expect_err("expected build_taproot_tx provenance gate rejection");
        assert!(matches!(
            build_tx_err,
            EngineError::ProvenanceGateRejected { .. }
        ));

        let finalize_err = finalize_sign_round(
            FinalizeSignRoundRequest {
                session_id: "session-metrics-provenance-finalize".to_string(),
                round_contributions: vec![],
                attempt_context: None,
            },
            true,
        )
        .expect_err("expected finalize_sign_round provenance gate rejection");
        assert!(matches!(
            finalize_err,
            EngineError::ProvenanceGateRejected { .. }
        ));

        let metrics = hardening_metrics();
        assert_eq!(metrics.run_dkg_calls_total, 1);
        assert_eq!(metrics.start_sign_round_calls_total, 0);
        assert_eq!(metrics.build_taproot_tx_calls_total, 1);
        assert_eq!(metrics.finalize_sign_round_calls_total, 1);
        assert_eq!(metrics.refresh_shares_calls_total, 0);
        assert_eq!(metrics.run_dkg_success_total, 0);
        assert_eq!(metrics.start_sign_round_success_total, 0);
        assert_eq!(metrics.build_taproot_tx_success_total, 0);
        assert_eq!(metrics.finalize_sign_round_success_total, 0);
        assert_eq!(metrics.refresh_shares_success_total, 0);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn hardening_metrics_track_start_sign_round_and_refresh_shares_counters() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: "session-metrics-start-refresh".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let _ = start_sign_round(StartSignRoundRequest {
            session_id: "session-metrics-start-refresh".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect("start sign round");

        let _ = refresh_shares(RefreshSharesRequest {
            session_id: "session-metrics-refresh-only".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "aaaa".to_string(),
            }],
        })
        .expect("refresh shares");

        let metrics = hardening_metrics();
        assert_eq!(metrics.start_sign_round_calls_total, 1);
        assert_eq!(metrics.start_sign_round_success_total, 1);
        assert_eq!(metrics.refresh_shares_calls_total, 1);
        assert_eq!(metrics.refresh_shares_success_total, 1);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn roast_transcript_audit_and_verify_blame_proof_roundtrip() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-transcript-audit-roundtrip";
        let message_hex = "deadbeef";
        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2, 3]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round attempt 1");

        let mut transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
            excluded_member_identifiers: vec![3],
            invalid_share_proof_fingerprint: Some("ab".repeat(32)),
        });

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect("start sign round attempt 2");

        let audit = roast_transcript_audit(crate::api::TranscriptAuditRequest {
            session_id: session_id.to_string(),
        })
        .expect("transcript audit");
        assert_eq!(audit.transition_count, 1);
        assert_eq!(audit.records.len(), 1);
        let record = &audit.records[0];
        assert_eq!(record.from_attempt_number, 1);
        assert_eq!(record.to_attempt_number, 2);
        assert_eq!(record.reason, ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF);
        assert_eq!(record.excluded_member_identifiers, vec![3]);
        assert!(!record.transcript_hash.is_empty());

        let verified = verify_blame_proof(crate::api::VerifyBlameProofRequest {
            session_id: session_id.to_string(),
            from_attempt_number: 1,
            accused_member_identifier: 3,
            reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
            invalid_share_proof_fingerprint: Some("ab".repeat(32)),
        })
        .expect("verify blame proof");
        assert!(verified.verified);
        assert_eq!(
            verified.transcript_hash,
            Some(record.transcript_hash.clone())
        );

        let not_verified = verify_blame_proof(crate::api::VerifyBlameProofRequest {
            session_id: session_id.to_string(),
            from_attempt_number: 1,
            accused_member_identifier: 2,
            reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
            invalid_share_proof_fingerprint: Some("ab".repeat(32)),
        })
        .expect("verify blame proof mismatch");
        assert!(!not_verified.verified);

        let metrics = hardening_metrics();
        assert_eq!(metrics.roast_transcript_audit_calls_total, 1);
        assert_eq!(metrics.roast_transcript_audit_success_total, 1);
        assert_eq!(metrics.verify_blame_proof_calls_total, 2);
        assert_eq!(metrics.verify_blame_proof_success_total, 1);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn roast_transcript_audit_records_persist_across_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("transcript_audit_persist_reload");
        reset_for_tests();
        clear_state_storage_policy_overrides();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-transcript-audit-persist";
        let message_hex = "deadbeef";
        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2, 3]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round attempt 1");

        let mut transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT.to_string(),
            excluded_member_identifiers: vec![],
            invalid_share_proof_fingerprint: None,
        });
        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2, 3]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2, 3]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect("start sign round attempt 2");

        reload_state_from_storage_for_tests();

        let audit = roast_transcript_audit(crate::api::TranscriptAuditRequest {
            session_id: session_id.to_string(),
        })
        .expect("transcript audit after reload");
        assert_eq!(audit.transition_count, 1);
        assert_eq!(audit.records.len(), 1);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn auto_quarantine_enforces_threshold_and_honors_dao_allowlist_override() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        std::env::set_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV, "true");
        std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV, "2");
        std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV, "1");
        std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV, "2");

        let session_id = "session-auto-quarantine-threshold";
        let message_hex = "deadbeef";
        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2, 3]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round attempt 1");

        let mut transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
            excluded_member_identifiers: vec![3],
            invalid_share_proof_fingerprint: Some("cd".repeat(32)),
        });

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect("start sign round attempt 2");

        let status = quarantine_status(crate::api::QuarantineStatusRequest {
            operator_identifier: 3,
        })
        .expect("quarantine status");
        assert!(status.auto_quarantine_enabled);
        assert_eq!(status.fault_score, 2);
        assert_eq!(status.quarantine_threshold, 2);
        assert!(status.quarantined);
        assert!(!status.dao_override_allowlisted);

        let err = run_dkg(RunDkgRequest {
            session_id: "session-auto-quarantine-rejected".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected auto-quarantine rejection");
        let EngineError::QuarantinePolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "operator_auto_quarantined");

        std::env::set_var(
            TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV,
            "3",
        );
        let allowlisted_status = quarantine_status(crate::api::QuarantineStatusRequest {
            operator_identifier: 3,
        })
        .expect("allowlisted quarantine status");
        assert!(allowlisted_status.dao_override_allowlisted);
        assert!(!allowlisted_status.quarantined);

        let _allowlisted_dkg = run_dkg(RunDkgRequest {
            session_id: "session-auto-quarantine-allowlisted".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("allowlisted operator should bypass quarantine rejection");

        let metrics = hardening_metrics();
        assert!(metrics.auto_quarantine_fault_events_total >= 1);
        assert!(metrics.auto_quarantine_enforcements_total >= 1);
        assert!(metrics.quarantined_operator_count >= 1);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn auto_quarantine_persists_across_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("auto_quarantine_persist_reload");
        reset_for_tests();
        clear_state_storage_policy_overrides();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        std::env::set_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV, "true");
        std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV, "2");
        std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV, "1");
        std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV, "2");

        let session_id = "session-auto-quarantine-persist-reload";
        let message_hex = "deadbeef";
        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2, 3]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round attempt 1");

        let mut transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
            excluded_member_identifiers: vec![3],
            invalid_share_proof_fingerprint: Some("ef".repeat(32)),
        });

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect("start sign round attempt 2");

        let status_before_reload = quarantine_status(crate::api::QuarantineStatusRequest {
            operator_identifier: 3,
        })
        .expect("quarantine status before reload");
        assert!(status_before_reload.quarantined);
        assert_eq!(status_before_reload.fault_score, 2);

        reload_state_from_storage_for_tests();

        let status_after_reload = quarantine_status(crate::api::QuarantineStatusRequest {
            operator_identifier: 3,
        })
        .expect("quarantine status after reload");
        assert!(status_after_reload.quarantined);
        assert_eq!(status_after_reload.fault_score, 2);

        let err = run_dkg(RunDkgRequest {
            session_id: "session-auto-quarantine-persist-reload-reject".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect_err("expected quarantine rejection after reload");
        let EngineError::QuarantinePolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "operator_auto_quarantined");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn refresh_cadence_status_tracks_overdue_and_emergency_rekey_persistence() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("refresh_cadence_status");
        reset_for_tests();
        clear_state_storage_policy_overrides();
        std::env::set_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV, "60");

        let session_id = "session-refresh-cadence";
        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let refresh_result = refresh_shares(RefreshSharesRequest {
            session_id: session_id.to_string(),
            current_shares: vec![
                ShareMaterial {
                    identifier: 1,
                    encrypted_share_hex: "11".repeat(16),
                },
                ShareMaterial {
                    identifier: 2,
                    encrypted_share_hex: "22".repeat(16),
                },
            ],
        })
        .expect("refresh shares");
        let initial_status = refresh_cadence_status(RefreshCadenceStatusRequest {
            session_id: session_id.to_string(),
        })
        .expect("refresh cadence status");
        assert_eq!(initial_status.refresh_count, 1);
        assert_eq!(
            initial_status.last_refresh_epoch,
            refresh_result.refresh_epoch
        );
        assert_eq!(
            initial_status.continuity_reference_key_group,
            Some(dkg_result.key_group)
        );
        assert!(initial_status.continuity_preserved);
        assert!(!initial_status.overdue);
        assert!(!initial_status.emergency_rekey_required);

        {
            let state = state().expect("state initialization");
            let mut guard = state.lock().expect("engine lock");
            let session = guard.sessions.get_mut(session_id).expect("session state");
            let refresh_record = session
                .refresh_history
                .last_mut()
                .expect("refresh history entry");
            refresh_record.refreshed_at_unix = refresh_record.refreshed_at_unix.saturating_sub(600);
            persist_engine_state_to_storage(&guard).expect("persist stale refresh history");
        }

        let stale_status = refresh_cadence_status(RefreshCadenceStatusRequest {
            session_id: session_id.to_string(),
        })
        .expect("stale refresh cadence status");
        assert!(stale_status.overdue);

        trigger_emergency_rekey(TriggerEmergencyRekeyRequest {
            session_id: session_id.to_string(),
            reason: "key compromise drill".to_string(),
        })
        .expect("trigger emergency rekey");
        reload_state_from_storage_for_tests();

        let post_rekey_status = refresh_cadence_status(RefreshCadenceStatusRequest {
            session_id: session_id.to_string(),
        })
        .expect("refresh cadence status after rekey");
        assert!(post_rekey_status.emergency_rekey_required);
        assert_eq!(
            post_rekey_status.emergency_rekey_reason,
            Some("key compromise drill".to_string())
        );

        let start_err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: post_rekey_status
                .continuity_reference_key_group
                .expect("continuity reference key group"),
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect_err("expected start sign round emergency rekey rejection");
        let EngineError::LifecyclePolicyRejected { reason_code, .. } = start_err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "emergency_rekey_required");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn differential_fuzzing_reports_no_unresolved_critical_divergence() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let result = run_differential_fuzzing(DifferentialFuzzRequest {
            seed: 0xD1FF_2026_0302_0001,
            case_count: 64,
        })
        .expect("run differential fuzzing");
        assert_eq!(result.case_count, 64);
        assert_eq!(result.critical_divergence_count, 0);
        assert!(!result.unresolved_critical_divergence);

        let metrics = hardening_metrics();
        assert!(metrics.differential_fuzz_runs_total >= 1);
        assert_eq!(metrics.differential_fuzz_critical_divergence_total, 0);
    }

    #[test]
    fn canary_promotion_and_rollback_controls_persist_across_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("canary_rollout_controls");
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let initial_status = canary_rollout_status().expect("canary rollout status");
        assert_eq!(initial_status.current_percent, 10);
        assert_eq!(initial_status.recommended_next_percent, Some(50));

        let promoted_50 = promote_canary(PromoteCanaryRequest { target_percent: 50 })
            .expect("promote canary to 50%");
        assert_eq!(promoted_50.from_percent, 10);
        assert_eq!(promoted_50.to_percent, 50);

        let promoted_100 = promote_canary(PromoteCanaryRequest {
            target_percent: 100,
        })
        .expect("promote canary to 100%");
        assert_eq!(promoted_100.from_percent, 50);
        assert_eq!(promoted_100.to_percent, 100);

        let rolled_back = rollback_canary(RollbackCanaryRequest {
            reason: "slo regression drill".to_string(),
        })
        .expect("rollback canary");
        assert_eq!(rolled_back.from_percent, 100);
        assert_eq!(rolled_back.to_percent, 50);

        reload_state_from_storage_for_tests();
        let post_reload_status =
            canary_rollout_status().expect("canary rollout status after reload");
        assert_eq!(post_reload_status.current_percent, 50);
        assert_eq!(post_reload_status.previous_percent, 50);

        let metrics = hardening_metrics();
        assert!(metrics.canary_promotions_total >= 2);
        assert!(metrics.canary_rollbacks_total >= 1);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn canary_promotion_halts_when_policy_reject_rate_exceeds_gate() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
        configure_required_signing_policy_limits_for_tests();

        let rejected = build_taproot_tx(build_policy_test_request("session-canary-gate-fail"))
            .expect_err("expected policy rejection");
        let EngineError::SigningPolicyRejected { reason_code, .. } = rejected else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "script_class_not_allowlisted");

        std::env::set_var(TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV, "0");
        let err = promote_canary(PromoteCanaryRequest { target_percent: 50 })
            .expect_err("expected canary gate rejection");
        let EngineError::LifecyclePolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "canary_slo_gate_failed");
    }

    #[test]
    fn emergency_rekey_blocks_finalize_and_build_taproot_tx_for_session() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let round_state = seeded_round_state("session-emergency-rekey-finalize");
        trigger_emergency_rekey(TriggerEmergencyRekeyRequest {
            session_id: round_state.session_id.clone(),
            reason: "compromise containment".to_string(),
        })
        .expect("trigger emergency rekey");

        let finalize_err = finalize_sign_round(
            FinalizeSignRoundRequest {
                session_id: round_state.session_id.clone(),
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
                attempt_context: None,
            },
            true,
        )
        .expect_err("expected finalize emergency rekey rejection");
        let EngineError::LifecyclePolicyRejected { reason_code, .. } = finalize_err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "emergency_rekey_required");

        let build_err = build_taproot_tx(build_policy_test_request(&round_state.session_id))
            .expect_err("expected build tx emergency rekey rejection");
        let EngineError::LifecyclePolicyRejected { reason_code, .. } = build_err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "emergency_rekey_required");
    }

    #[test]
    fn build_taproot_tx_rate_limiter_uses_token_bucket_refill() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        std::env::set_var(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV, "2");
        configure_required_signing_policy_limits_for_tests();

        {
            let mut limiter = build_tx_rate_limiter_state()
                .lock()
                .expect("build tx rate limiter lock");
            limiter.last_refill_unix = now_unix().saturating_sub(1);
            limiter.token_microunits = 0;
            limiter.configured_rate_limit_per_minute = 2;
        }

        let rejected = build_taproot_tx(build_policy_test_request("session-rate-limited-initial"))
            .expect_err("expected rate-limit rejection with sub-token refill");
        let EngineError::SigningPolicyRejected { reason_code, .. } = rejected else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "rate_limit_per_minute_exceeded");

        {
            let mut limiter = build_tx_rate_limiter_state()
                .lock()
                .expect("build tx rate limiter lock");
            limiter.last_refill_unix = now_unix().saturating_sub(30);
            limiter.token_microunits = 0;
            limiter.configured_rate_limit_per_minute = 2;
        }

        let allowed = build_taproot_tx(build_policy_test_request("session-rate-limited-refill"));
        assert!(allowed.is_ok(), "expected one token after 30s refill");

        let rejected_again =
            build_taproot_tx(build_policy_test_request("session-rate-limited-followup"))
                .expect_err("expected immediate follow-up rejection without full refill");
        let EngineError::SigningPolicyRejected { reason_code, .. } = rejected_again else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "rate_limit_per_minute_exceeded");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_cache_hit_rechecks_signing_policy_firewall() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        configure_required_signing_policy_limits_for_tests();

        let request = build_policy_test_request("session-build-tx-cache-policy-recheck");

        let first_result = build_taproot_tx(request.clone()).expect("first build tx");
        assert!(!first_result.tx_hex.is_empty());

        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
        let err =
            build_taproot_tx(request).expect_err("expected cache-hit firewall policy rejection");
        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "script_class_not_allowlisted");

        let metrics = hardening_metrics();
        assert_eq!(metrics.build_taproot_tx_calls_total, 2);
        assert_eq!(metrics.build_taproot_tx_success_total, 1);
        assert_eq!(metrics.build_taproot_tx_policy_reject_total, 1);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_cached_retry_does_not_charge_rate_limit() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        std::env::set_var(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV, "1");
        configure_required_signing_policy_limits_for_tests();

        let request = build_policy_test_request("session-build-tx-cache-rate-limit");

        let first_result = build_taproot_tx(request.clone()).expect("first build tx");
        assert!(!first_result.tx_hex.is_empty());

        {
            let mut limiter = build_tx_rate_limiter_state()
                .lock()
                .expect("build tx rate limiter lock");
            limiter.last_refill_unix = now_unix();
            limiter.token_microunits = 0;
            limiter.configured_rate_limit_per_minute = 1;
        }

        let retry_result = build_taproot_tx(request).expect("cached retry must not rate-limit");
        assert_eq!(first_result, retry_result);

        let rejected =
            build_taproot_tx(build_policy_test_request("session-build-tx-rate-limit-new"))
                .expect_err("new build tx should still be rate-limited");
        let EngineError::SigningPolicyRejected { reason_code, .. } = rejected else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "rate_limit_per_minute_exceeded");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_signing_policy_firewall_rejects_without_policy_checked_build_tx() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let session_id = "session-signing-policy-start-missing-build-tx";
        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        configure_required_signing_policy_limits_for_tests();

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect_err("expected signing policy reject without build tx binding");
        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "missing_policy_checked_build_tx");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_signing_policy_firewall_rejects_message_not_bound_to_build_tx() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let session_id = "session-signing-policy-start-message-mismatch";
        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        configure_required_signing_policy_limits_for_tests();

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        build_taproot_tx(build_policy_test_request(session_id)).expect("build tx");

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect_err("expected signing policy reject for message mismatch");
        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(
            reason_code,
            "signing_message_not_bound_to_policy_checked_build_tx"
        );

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_signing_policy_firewall_accepts_policy_bound_message() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let session_id = "session-signing-policy-start-bound-message";
        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        configure_required_signing_policy_limits_for_tests();

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let tx_result = build_taproot_tx(build_policy_test_request(session_id)).expect("build tx");
        let message_hex = policy_bound_message_hex_from_tx_result(&tx_result);

        let round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex,
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect("expected start_sign_round allow for policy-bound message");
        assert_eq!(round_state.session_id, session_id);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_signing_policy_firewall_rejects_missing_policy_checked_build_tx() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let session_id = "session-signing-policy-finalize-missing-build-tx";
        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        configure_required_signing_policy_limits_for_tests();

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let tx_result = build_taproot_tx(build_policy_test_request(session_id)).expect("build tx");
        let message_hex = policy_bound_message_hex_from_tx_result(&tx_result);
        let round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex,
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect("start sign round");

        {
            let mut guard = state().expect("state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut(session_id)
                .expect("session should exist");
            session.tx_result = None;
            session.build_tx_request_fingerprint = None;
        }

        let err = finalize_sign_round(
            FinalizeSignRoundRequest {
                session_id: session_id.to_string(),
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
                attempt_context: None,
            },
            true,
        )
        .expect_err("expected finalize reject without policy-checked build tx");
        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "missing_policy_checked_build_tx");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_signing_policy_firewall_rejects_message_mismatch_after_tx_result_swap() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let session_id = "session-signing-policy-finalize-tx-result-swap";
        std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
        std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
        configure_required_signing_policy_limits_for_tests();

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let tx_result = build_taproot_tx(build_policy_test_request(session_id)).expect("build tx");
        let message_hex = policy_bound_message_hex_from_tx_result(&tx_result);
        let round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex,
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect("start sign round");

        {
            let mut guard = state().expect("state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut(session_id)
                .expect("session should exist");
            session.tx_result = Some(TransactionResult {
                session_id: session_id.to_string(),
                tx_hex: "00".to_string(),
            });
        }

        let err = finalize_sign_round(
            FinalizeSignRoundRequest {
                session_id: session_id.to_string(),
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
                attempt_context: None,
            },
            true,
        )
        .expect_err("expected finalize reject for tx_result swap");
        let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(
            reason_code,
            "signing_message_not_bound_to_policy_checked_build_tx"
        );

        clear_state_storage_policy_overrides();
    }

    #[cfg(unix)]
    fn wait_for_file(path: &Path, timeout: Duration) -> bool {
        let start = Instant::now();
        while start.elapsed() < timeout {
            if path.exists() {
                return true;
            }
            thread::sleep(Duration::from_millis(50));
        }
        path.exists()
    }

    #[cfg(unix)]
    struct LockHelperProcessGuard {
        child: Option<std::process::Child>,
        release_path: PathBuf,
    }

    #[cfg(unix)]
    impl LockHelperProcessGuard {
        fn new(child: std::process::Child, release_path: PathBuf) -> Self {
            Self {
                child: Some(child),
                release_path,
            }
        }

        fn signal_release(&self) {
            let _ = std::fs::write(&self.release_path, b"release");
        }

        fn wait_for_success(mut self) {
            self.signal_release();
            let mut child = self.child.take().expect("helper child should be present");
            let child_status = child.wait().expect("wait for lock helper process");
            assert!(
                child_status.success(),
                "lock helper process failed with status: {child_status}"
            );
        }
    }

    #[cfg(unix)]
    impl Drop for LockHelperProcessGuard {
        fn drop(&mut self) {
            self.signal_release();

            let Some(mut child) = self.child.take() else {
                return;
            };

            let start = Instant::now();
            while start.elapsed() < Duration::from_secs(2) {
                match child.try_wait() {
                    Ok(Some(_)) => return,
                    Ok(None) => thread::sleep(Duration::from_millis(50)),
                    Err(_) => break,
                }
            }

            let _ = child.kill();
            let _ = child.wait();
        }
    }

    fn build_attempt_context(
        session_id: &str,
        message_hex: &str,
        attempt_number: u32,
        coordinator_identifier: u16,
        included_participants: Vec<u16>,
    ) -> AttemptContext {
        let canonical_included_participants =
            canonicalize_included_participants(&included_participants)
                .expect("canonical included participants");
        let message_bytes = hex::decode(message_hex).expect("message hex");
        let message_digest_hex = hash_hex(&message_bytes);
        let included_participants_fingerprint =
            roast_included_participants_fingerprint_hex(&canonical_included_participants)
                .expect("included participants fingerprint");
        let attempt_id = roast_attempt_id_hex(
            session_id,
            &message_digest_hex,
            attempt_number,
            coordinator_identifier,
            &included_participants_fingerprint,
        )
        .expect("attempt id");

        AttemptContext {
            attempt_number,
            coordinator_identifier,
            included_participants,
            included_participants_fingerprint,
            attempt_id,
        }
    }

    fn build_deterministic_attempt_context(
        session_id: &str,
        message_hex: &str,
        attempt_number: u32,
        included_participants: Vec<u16>,
    ) -> AttemptContext {
        let canonical_included_participants =
            canonicalize_included_participants(&included_participants)
                .expect("canonical included participants");
        let message_bytes = hex::decode(message_hex).expect("message hex");
        let message_digest_hex = hash_hex(&message_bytes);
        let attempt_seed = roast_attempt_seed_from_message_digest_hex(&message_digest_hex)
            .expect("attempt seed from message digest");
        let coordinator_identifier = select_coordinator_identifier(
            &canonical_included_participants,
            attempt_seed,
            attempt_number,
        )
        .expect("deterministic coordinator");

        build_attempt_context(
            session_id,
            message_hex,
            attempt_number,
            coordinator_identifier,
            included_participants,
        )
    }

    fn build_attempt_transition_evidence_from_active_session(
        session_id: &str,
    ) -> AttemptTransitionEvidence {
        let guard = state()
            .expect("engine state")
            .lock()
            .expect("engine state lock");
        let session = guard
            .sessions
            .get(session_id)
            .expect("session should exist for transition evidence");
        let active_attempt_context = session
            .active_attempt_context
            .as_ref()
            .expect("active attempt context should exist");
        let round_state = session
            .round_state
            .as_ref()
            .expect("round state should exist for transition evidence");
        let sign_request_fingerprint = session
            .sign_request_fingerprint
            .as_ref()
            .expect("sign request fingerprint should exist");

        AttemptTransitionEvidence {
            from_attempt_number: active_attempt_context.attempt_number,
            from_attempt_id: active_attempt_context.attempt_id.clone(),
            from_coordinator_identifier: active_attempt_context.coordinator_identifier,
            previous_round_id: round_state.round_id.clone(),
            previous_sign_request_fingerprint: sign_request_fingerprint.clone(),
            exclusion_evidence: Some(AttemptExclusionEvidence {
                reason: ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT.to_string(),
                excluded_member_identifiers: vec![],
                invalid_share_proof_fingerprint: None,
            }),
        }
    }

    #[test]
    fn roast_attempt_context_hash_vectors_match_expected_values() {
        let included_participants_fingerprint =
            roast_included_participants_fingerprint_hex(&[1, 3, 5])
                .expect("included participants fingerprint");
        assert_eq!(
            included_participants_fingerprint,
            "0c9258935f0a30c065befcd746cb1564e9f3c91936c0f0f1c78853fa2d6713dc"
        );

        let attempt_id = roast_attempt_id_hex(
            "vector-session-1",
            "5f78c33274e43fa9de5659265c1d917e25c03722dcb0b8d27db8d5feaa813953",
            7,
            3,
            &included_participants_fingerprint,
        )
        .expect("attempt id");
        assert_eq!(
            attempt_id,
            "dbc7a4df9bc3ef8dee3a9f5a47ff519e22e8d6f9b0461dd415077176e4e6ee95"
        );
    }

    #[test]
    fn formal_verification_roast_attempt_context_shared_vectors_match_expected_values() {
        let vector_suite = load_attempt_context_vector_suite();
        assert_eq!(vector_suite.schema_version, "roast-attempt-context-v1");
        assert_eq!(
            vector_suite.hash_domains.included_participants_fingerprint,
            ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN
        );
        assert_eq!(
            vector_suite.hash_domains.attempt_id,
            ROAST_ATTEMPT_ID_DOMAIN
        );
        assert!(
            !vector_suite.vectors.is_empty(),
            "expected at least one shared attempt-context vector"
        );

        for vector in vector_suite.vectors {
            let canonical_participants =
                canonicalize_included_participants(&vector.included_participants)
                    .expect("vector participants should canonicalize");
            let included_participants_fingerprint =
                roast_included_participants_fingerprint_hex(&canonical_participants)
                    .expect("included participants fingerprint");
            assert_eq!(
                included_participants_fingerprint,
                vector
                    .expected_included_participants_fingerprint
                    .to_ascii_lowercase(),
                "included participants fingerprint mismatch for vector [{}]",
                vector.id
            );

            let attempt_id = roast_attempt_id_hex(
                &vector.session_id,
                &vector.message_digest_hex.to_ascii_lowercase(),
                vector.attempt_number,
                vector.coordinator_identifier,
                &included_participants_fingerprint,
            )
            .expect("attempt id");
            assert_eq!(
                attempt_id,
                vector.expected_attempt_id.to_ascii_lowercase(),
                "attempt id mismatch for vector [{}]",
                vector.id
            );
        }
    }

    fn participant_set_strategy() -> impl Strategy<Value = Vec<u16>> {
        prop::collection::btree_set(1_u16..=1024_u16, 2..=16)
            .prop_map(|participants| participants.into_iter().collect())
    }

    proptest! {
        #![proptest_config(ProptestConfig::with_cases(64))]

        #[test]
        fn formal_verification_attempt_context_is_stable_under_participant_permutations(
            session_suffix in any::<u32>(),
            attempt_number in 1_u32..=16_u32,
            participants in participant_set_strategy(),
            message_bytes in prop::collection::vec(any::<u8>(), 1..=128),
        ) {
            let session_id = format!("formal-attempt-session-{session_suffix}");
            let message_hex = hex::encode(message_bytes);
            let mut reversed_participants = participants.clone();
            reversed_participants.reverse();

            let canonical_attempt_context = build_deterministic_attempt_context(
                &session_id,
                &message_hex,
                attempt_number,
                participants.clone(),
            );
            let permuted_attempt_context = build_deterministic_attempt_context(
                &session_id,
                &message_hex,
                attempt_number,
                reversed_participants,
            );

            prop_assert_eq!(
                &canonical_attempt_context.included_participants_fingerprint,
                &permuted_attempt_context.included_participants_fingerprint
            );
            prop_assert_eq!(
                &canonical_attempt_context.attempt_id,
                &permuted_attempt_context.attempt_id
            );

            let message_digest_hex = hash_hex(
                &hex::decode(&message_hex).expect("message hex should decode for validation"),
            );
            let validated_participants = validate_attempt_context(
                &session_id,
                &message_digest_hex,
                2,
                Some(&permuted_attempt_context),
                true,
            )
            .expect("attempt context should validate")
            .expect("validated attempt context should return canonical participants");

            let mut expected_canonical_participants = participants;
            expected_canonical_participants.sort_unstable();
            prop_assert_eq!(validated_participants, expected_canonical_participants);
        }

        #[test]
        fn formal_verification_attempt_context_rejects_tampered_attempt_id(
            session_suffix in any::<u32>(),
            attempt_number in 1_u32..=16_u32,
            participants in participant_set_strategy(),
            message_bytes in prop::collection::vec(any::<u8>(), 1..=128),
        ) {
            let session_id = format!("formal-attempt-tamper-session-{session_suffix}");
            let message_hex = hex::encode(message_bytes);

            let mut tampered_attempt_context = build_deterministic_attempt_context(
                &session_id,
                &message_hex,
                attempt_number,
                participants,
            );
            tampered_attempt_context.attempt_id = "11".repeat(32);

            let message_digest_hex = hash_hex(
                &hex::decode(&message_hex).expect("message hex should decode for validation"),
            );
            let err = validate_attempt_context(
                &session_id,
                &message_digest_hex,
                2,
                Some(&tampered_attempt_context),
                true,
            )
            .expect_err("tampered attempt id must be rejected");
            prop_assert!(matches!(
                err,
                EngineError::Validation(message)
                if message.contains("attempt_context.attempt_id")
            ));
        }

        #[test]
        fn formal_verification_encrypted_state_envelope_fails_closed_on_key_id_mismatch(
            refresh_epoch_counter in any::<u64>(),
            mismatched_key_id_suffix in any::<u16>(),
        ) {
            let _guard = lock_test_state();
            std::env::set_var(
                TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
                TEST_STATE_ENCRYPTION_KEY_HEX,
            );

            let persisted = PersistedEngineState {
                schema_version: PERSISTED_STATE_SCHEMA_VERSION,
                sessions: HashMap::new(),
                refresh_epoch_counter,
                operator_fault_scores: BTreeMap::new(),
                quarantined_operator_identifiers: vec![],
                canary_rollout: CanaryRolloutState::default(),
            };
            let encoded =
                encode_encrypted_state_envelope(&persisted).expect("state envelope encode");
            let envelope: PersistedEncryptedEngineStateEnvelope =
                serde_json::from_slice(encoded.as_ref()).expect("state envelope decode");

            let decoded = decode_encrypted_state_envelope(envelope.clone())
                .expect("untampered envelope should decode");
            prop_assert_eq!(decoded.schema_version, persisted.schema_version);
            prop_assert_eq!(decoded.refresh_epoch_counter, persisted.refresh_epoch_counter);
            prop_assert_eq!(decoded.sessions.len(), persisted.sessions.len());

            let mut tampered_envelope = envelope;
            tampered_envelope.key_id = format!(
                "{}-{}",
                TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX, mismatched_key_id_suffix
            );
            let err = decode_encrypted_state_envelope(tampered_envelope)
                .expect_err("tampered key_id must fail closed");
            prop_assert!(matches!(
                err,
                EngineError::Internal(message)
                if message.contains("state key identifier mismatch")
            ));
        }
    }

    #[test]
    fn formal_verification_derive_round_id_binds_attempt_id_case_insensitive_component() {
        let request_session_id = "round-id-attempt-case-session";
        let key_group = "key-group";
        let message_hex = "deadbeef";
        let signing_participants_fingerprint = "participants-fingerprint";

        let lowercase_attempt_context = AttemptContext {
            attempt_number: 1,
            coordinator_identifier: 1,
            included_participants: vec![1, 2],
            included_participants_fingerprint: "aa".repeat(32),
            attempt_id: "ab".repeat(32),
        };
        let uppercase_attempt_context = AttemptContext {
            attempt_id: lowercase_attempt_context.attempt_id.to_ascii_uppercase(),
            ..lowercase_attempt_context.clone()
        };

        let round_id_lowercase_attempt = derive_round_id(
            request_session_id,
            key_group,
            message_hex,
            signing_participants_fingerprint,
            Some(&lowercase_attempt_context),
        );
        let round_id_uppercase_attempt = derive_round_id(
            request_session_id,
            key_group,
            message_hex,
            signing_participants_fingerprint,
            Some(&uppercase_attempt_context),
        );
        assert_eq!(round_id_lowercase_attempt, round_id_uppercase_attempt);

        let different_attempt_context = AttemptContext {
            attempt_id: "cd".repeat(32),
            ..lowercase_attempt_context.clone()
        };
        let round_id_different_attempt = derive_round_id(
            request_session_id,
            key_group,
            message_hex,
            signing_participants_fingerprint,
            Some(&different_attempt_context),
        );
        assert_ne!(round_id_lowercase_attempt, round_id_different_attempt);

        let round_id_without_attempt = derive_round_id(
            request_session_id,
            key_group,
            message_hex,
            signing_participants_fingerprint,
            None,
        );
        assert_ne!(round_id_lowercase_attempt, round_id_without_attempt);
    }

    struct RoastStrictModeGuard {
        previous_value: Option<String>,
    }

    impl RoastStrictModeGuard {
        fn set(value: Option<&str>) -> Self {
            let previous_value = std::env::var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV).ok();
            match value {
                Some(value) => std::env::set_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV, value),
                None => std::env::remove_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV),
            }

            Self { previous_value }
        }

        fn enable() -> Self {
            Self::set(Some("true"))
        }
    }

    impl Drop for RoastStrictModeGuard {
        fn drop(&mut self) {
            match &self.previous_value {
                Some(value) => std::env::set_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV, value),
                None => std::env::remove_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV),
            }
        }
    }

    struct SignerProfileGuard {
        previous_value: Option<String>,
    }

    impl SignerProfileGuard {
        fn set(value: Option<&str>) -> Self {
            let previous_value = std::env::var(TBTC_SIGNER_PROFILE_ENV).ok();
            match value {
                Some(value) => std::env::set_var(TBTC_SIGNER_PROFILE_ENV, value),
                None => std::env::remove_var(TBTC_SIGNER_PROFILE_ENV),
            }

            Self { previous_value }
        }

        fn production() -> Self {
            Self::set(Some(TBTC_SIGNER_PROFILE_PRODUCTION))
        }
    }

    impl Drop for SignerProfileGuard {
        fn drop(&mut self) {
            match &self.previous_value {
                Some(value) => std::env::set_var(TBTC_SIGNER_PROFILE_ENV, value),
                None => std::env::remove_var(TBTC_SIGNER_PROFILE_ENV),
            }
        }
    }

    #[test]
    #[cfg(unix)]
    #[ignore]
    fn state_file_lock_contention_helper() {
        if std::env::var("TBTC_SIGNER_LOCK_HELPER").ok().as_deref() != Some("1") {
            return;
        }

        let state_path = active_state_file_path().expect("resolve helper state path");
        let _lock = StateFileLock::acquire(&state_path).expect("acquire helper lock");

        let ready_path = std::env::var("TBTC_SIGNER_LOCK_READY_PATH")
            .expect("helper ready path env should be set");
        std::fs::write(&ready_path, b"ready").expect("write helper ready file");

        let release_path = std::env::var("TBTC_SIGNER_LOCK_RELEASE_PATH")
            .expect("helper release path env should be set");
        assert!(
            wait_for_file(Path::new(&release_path), Duration::from_secs(20)),
            "timed out waiting for helper release signal"
        );
    }

    #[test]
    fn start_sign_round_rejects_missing_attempt_context_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: "session-roast-strict-start-missing-attempt-context".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let err = start_sign_round(StartSignRoundRequest {
            session_id: "session-roast-strict-start-missing-attempt-context".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect_err("expected attempt context validation");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("attempt_context is required"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn production_profile_forces_roast_strict_mode_without_env_flag() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("production_forces_roast_strict");
        reset_for_tests();
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
            format!("printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
        );

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: "session-production-forces-roast-strict".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("seed non-production dkg");

        // RAII guards restore the prior env on Drop so a panic or early return
        // does not leak production-profile state into subsequent tests.
        configure_valid_provenance_attestation_for_tests();
        let _signer_profile = SignerProfileGuard::production();
        let _roast_strict_mode = RoastStrictModeGuard::set(Some("false"));

        let err = start_sign_round(StartSignRoundRequest {
            session_id: "session-production-forces-roast-strict".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect_err("production profile should require ROAST attempt context");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("attempt_context is required"),
            "unexpected validation message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_accepts_valid_attempt_context_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-start-valid-attempt-context";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
        let round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect("start sign round");

        assert_eq!(round_state.required_contributions, 2);
    }

    #[test]
    fn start_sign_round_rejects_invalid_attempt_context_fingerprint_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-start-invalid-attempt-context-fingerprint";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let mut attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        attempt_context.included_participants_fingerprint = "00".repeat(32);

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect_err("expected attempt context fingerprint validation");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("included_participants_fingerprint"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_invalid_attempt_context_attempt_id_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-start-invalid-attempt-id";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let mut attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        attempt_context.attempt_id = "11".repeat(32);

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect_err("expected attempt context attempt-id validation");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("attempt_context.attempt_id"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_attempt_number_zero_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-start-attempt-number-zero";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let mut attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        attempt_context.attempt_number = 0;

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect_err("expected attempt number validation");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("attempt_context.attempt_number"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_zero_coordinator_identifier_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-start-coordinator-zero";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let mut attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        attempt_context.coordinator_identifier = 0;

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect_err("expected coordinator identifier validation");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("attempt_context.coordinator_identifier"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_nondeterministic_coordinator_identifier_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-start-coordinator-nondeterministic";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let deterministic_attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        let mismatched_coordinator_identifier =
            if deterministic_attempt_context.coordinator_identifier == 1 {
                2
            } else {
                1
            };
        let invalid_attempt_context = build_attempt_context(
            session_id,
            message_hex,
            1,
            mismatched_coordinator_identifier,
            vec![1, 2],
        );

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(invalid_attempt_context),
            attempt_transition_evidence: None,
        })
        .expect_err("expected deterministic coordinator validation");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("deterministic coordinator"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_sub_threshold_attempt_participants_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-start-sub-threshold-attempt-participants";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1]);

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect_err("expected attempt participants threshold validation");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("at least threshold members"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_duplicate_attempt_participants_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-start-duplicate-attempt-participants";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_context = AttemptContext {
            attempt_number: 1,
            coordinator_identifier: 1,
            included_participants: vec![1, 1, 2],
            included_participants_fingerprint: "00".repeat(32),
            attempt_id: "11".repeat(32),
        };

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect_err("expected duplicate attempt participant validation");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("duplicate identifier"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn start_sign_round_accepts_hex_case_variant_attempt_context_idempotent_retry() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-start-case-variant-idempotency";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let mut uppercase_attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
        uppercase_attempt_context.included_participants_fingerprint = uppercase_attempt_context
            .included_participants_fingerprint
            .to_ascii_uppercase();
        uppercase_attempt_context.attempt_id =
            uppercase_attempt_context.attempt_id.to_ascii_uppercase();

        let first_round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(uppercase_attempt_context),
            attempt_transition_evidence: None,
        })
        .expect("first start sign round");

        let lowercase_attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        let second_round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![2, 1]),
            attempt_context: Some(lowercase_attempt_context),
            attempt_transition_evidence: None,
        })
        .expect("second start sign round retry");

        assert_eq!(first_round_state, second_round_state);
    }

    #[test]
    fn finalize_sign_round_rejects_missing_attempt_context_in_roast_strict_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-strict-finalize-missing-attempt-context";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
        let round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect("start sign round");

        let err = finalize_sign_round(
            FinalizeSignRoundRequest {
                session_id: session_id.to_string(),
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
            },
            true,
        )
        .expect_err("expected attempt context validation");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("attempt_context is required"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn finalize_sign_round_accepts_missing_attempt_context_when_not_strict_with_active_attempt_context(
    ) {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let session_id = "session-roast-phase2-nonstrict-finalize-missing-attempt-context";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        let round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect("start sign round");

        let signature_result = finalize_sign_round(
            FinalizeSignRoundRequest {
                session_id: session_id.to_string(),
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
            },
            true,
        )
        .expect("finalize without attempt context in non-strict mode");

        assert_eq!(signature_result.round_id, round_state.round_id);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_accepts_missing_attempt_context_after_reload_when_not_strict() {
        let _guard = lock_test_state();
        let state_path =
            configure_test_state_path("phase2_nonstrict_finalize_missing_after_reload");
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let session_id = "session-roast-phase2-nonstrict-finalize-reload";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
        let round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect("start sign round");

        reload_state_from_storage_for_tests();

        let signature_result = finalize_sign_round(
            FinalizeSignRoundRequest {
                session_id: session_id.to_string(),
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
            },
            true,
        )
        .expect("finalize without attempt context after reload in non-strict mode");

        assert_eq!(signature_result.round_id, round_state.round_id);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_returns_session_conflict_for_attempt_context_presence_mismatch_in_non_strict_mode(
    ) {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        let session_id = "session-roast-phase2-nonstrict-start-presence-mismatch";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        })
        .expect("start sign round with attempt context");

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: None,
            attempt_transition_evidence: None,
        })
        .expect_err("expected session conflict on payload mismatch");

        assert!(matches!(err, EngineError::SessionConflict { .. }));
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_rejects_stale_attempt_number_against_active_attempt_context() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase2-stale-start-attempt";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 2");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect_err("expected stale attempt rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("stale"),
            "expected stale-attempt validation message, got: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_future_attempt_number_without_transition_authorization() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase2-future-start-attempt";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: None,
        })
        .expect_err("expected future attempt rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("attempt_transition_evidence"),
            "expected future-attempt validation message, got: {message}"
        );
    }

    #[test]
    fn start_sign_round_allows_next_attempt_with_valid_transition_evidence() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase2-transition-evidence-valid";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        let round_state_one = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let round_state_two = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect("start sign round for authorized attempt 2");

        assert_ne!(round_state_one.round_id, round_state_two.round_id);
        let transition_telemetry = round_state_two
            .attempt_transition_telemetry
            .expect("attempt transition telemetry");
        assert_eq!(transition_telemetry.from_attempt_number, 1);
        assert_eq!(transition_telemetry.to_attempt_number, 2);
        assert_eq!(
            transition_telemetry.reason,
            ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT
        );
        assert!(transition_telemetry.excluded_member_identifiers.is_empty());

        let stale_attempt =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(stale_attempt),
            attempt_transition_evidence: None,
        })
        .expect_err("expected stale rejection after authorized advancement");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("stale"),
            "expected stale-attempt validation message, got: {message}"
        );
    }

    #[test]
    fn start_sign_round_allows_next_attempt_with_valid_transition_evidence_after_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("phase2_transition_evidence_valid_reload");
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase2-transition-evidence-valid-reload";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        let round_state_one = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        reload_state_from_storage_for_tests();

        let transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let round_state_two = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect("start sign round for authorized attempt 2 after reload");

        assert_ne!(round_state_one.round_id, round_state_two.round_id);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_rejects_stale_attempt_after_authorized_transition_across_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("phase2_transition_stale_after_reload");
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase2-transition-stale-after-reload";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one.clone()),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect("start sign round for authorized attempt 2");

        reload_state_from_storage_for_tests();

        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect_err("expected stale attempt rejection after reload");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("stale"),
            "expected stale-attempt validation message, got: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_rejects_next_attempt_with_invalid_transition_evidence() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase2-transition-evidence-invalid";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let mut invalid_transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        invalid_transition_evidence.previous_round_id = "invalid-round-id".to_string();

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(invalid_transition_evidence),
        })
        .expect_err("expected invalid transition evidence rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("previous_round_id"),
            "expected transition-evidence previous_round_id validation message, got: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_far_future_attempt_even_with_transition_evidence() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase2-transition-evidence-far-future";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
        let attempt_three =
            build_deterministic_attempt_context(session_id, message_hex, 3, vec![1, 2]);
        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_three),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect_err("expected far-future attempt rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("ahead of active attempt_number"),
            "expected far-future validation message, got: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_next_attempt_without_exclusion_evidence() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase4-transition-missing-exclusion-evidence";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let mut transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        transition_evidence.exclusion_evidence = None;

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect_err("expected missing exclusion evidence rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("exclusion_evidence"),
            "expected exclusion-evidence validation message, got: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_timeout_reason_with_invalid_share_fingerprint() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase4-timeout-reason-fingerprint-rejection";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let mut transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT.to_string(),
            excluded_member_identifiers: vec![],
            invalid_share_proof_fingerprint: Some("ab".repeat(32)),
        });

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect_err("expected timeout-reason proof fingerprint rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("must be omitted"),
            "expected timeout-reason proof-fingerprint validation message, got: {message}"
        );
    }

    #[test]
    fn start_sign_round_accepts_invalid_share_proof_exclusion_evidence() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase4-invalid-share-proof-evidence-valid";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2, 3]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let mut transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
            excluded_member_identifiers: vec![3],
            invalid_share_proof_fingerprint: Some("ab".repeat(32)),
        });

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let round_state_two = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect("start sign round for attempt 2 with invalid-share-proof evidence");

        assert_eq!(round_state_two.required_contributions, 2);
        let transition_telemetry = round_state_two
            .attempt_transition_telemetry
            .expect("attempt transition telemetry");
        assert_eq!(transition_telemetry.from_attempt_number, 1);
        assert_eq!(transition_telemetry.to_attempt_number, 2);
        assert_eq!(
            transition_telemetry.reason,
            ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF
        );
        assert_eq!(transition_telemetry.excluded_member_identifiers, vec![3]);
    }

    #[test]
    fn start_sign_round_rejects_invalid_share_proof_without_fingerprint() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase4-invalid-share-proof-fingerprint-required";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2, 3]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let mut transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
            excluded_member_identifiers: vec![3],
            invalid_share_proof_fingerprint: None,
        });

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect_err("expected invalid-share-proof fingerprint required rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("invalid_share_proof_fingerprint is required"),
            "expected invalid-share-proof fingerprint-required message, got: {message}"
        );
    }

    #[test]
    fn start_sign_round_rejects_invalid_share_proof_with_empty_fingerprint() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase4-invalid-share-proof-empty-fingerprint";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let attempt_one =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
        start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![1, 2, 3]),
            attempt_context: Some(attempt_one),
            attempt_transition_evidence: None,
        })
        .expect("start sign round for attempt 1");

        let mut transition_evidence =
            build_attempt_transition_evidence_from_active_session(session_id);
        transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
            excluded_member_identifiers: vec![3],
            invalid_share_proof_fingerprint: Some("   ".to_string()),
        });

        let attempt_two =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let err = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_two),
            attempt_transition_evidence: Some(transition_evidence),
        })
        .expect_err("expected invalid-share-proof empty-fingerprint rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("must be non-empty valid hex"),
            "expected invalid-share-proof empty-fingerprint message, got: {message}"
        );
    }

    #[test]
    fn finalize_sign_round_rejects_coordinator_mismatch_against_active_attempt_context() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase2-finalize-coordinator-mismatch";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let start_attempt =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        let round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(start_attempt),
            attempt_transition_evidence: None,
        })
        .expect("start sign round");

        let mismatched_attempt = build_attempt_context(session_id, message_hex, 1, 1, vec![1, 2]);
        let err = finalize_sign_round(
            FinalizeSignRoundRequest {
                session_id: session_id.to_string(),
                attempt_context: Some(mismatched_attempt),
                round_contributions: vec![
                    round_state.own_contribution.clone(),
                    RoundContribution {
                        identifier: 2,
                        signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                    },
                ],
            },
            true,
        )
        .expect_err("expected coordinator mismatch rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("coordinator_identifier"),
            "expected coordinator mismatch validation message, got: {message}"
        );
    }

    #[test]
    fn finalize_sign_round_rejects_stale_attempt_number_against_active_attempt_context() {
        let _guard = lock_test_state();
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-roast-phase2-finalize-stale-attempt";
        let message_hex = "deadbeef";

        let dkg_result = run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("run dkg");

        let start_attempt =
            build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
        let round_state = start_sign_round(StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(start_attempt),
            attempt_transition_evidence: None,
        })
        .expect("start sign round");

        let stale_attempt =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        let err = finalize_sign_round(
            FinalizeSignRoundRequest {
                session_id: session_id.to_string(),
                attempt_context: Some(stale_attempt),
                round_contributions: vec![
                    round_state.own_contribution.clone(),
                    RoundContribution {
                        identifier: 2,
                        signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                    },
                ],
            },
            true,
        )
        .expect_err("expected stale attempt rejection");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("stale"),
            "expected stale-attempt validation message, got: {message}"
        );
    }

    #[test]
    fn finalize_rejects_bootstrap_synthetic_contributions_outside_bootstrap_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let round_state = seeded_round_state("session-synthetic-rejected");

        let request = FinalizeSignRoundRequest {
            session_id: "session-synthetic-rejected".to_string(),
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

        let err = finalize_sign_round(request, false).expect_err("expected synthetic rejection");
        assert!(matches!(
            err,
            EngineError::SyntheticContributionRejected { .. }
        ));
    }

    #[test]
    fn finalize_accepts_bootstrap_synthetic_contributions_in_bootstrap_mode() {
        let _guard = lock_test_state();
        reset_for_tests();
        let round_state = seeded_round_state("session-synthetic-accepted");

        let request = FinalizeSignRoundRequest {
            session_id: "session-synthetic-accepted".to_string(),
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

        let result =
            finalize_sign_round(request, true).expect("expected bootstrap synthetic acceptance");
        assert_eq!(result.round_id, round_state.round_id);
    }

    #[test]
    fn finalize_aggregates_real_contributions_outside_bootstrap_mode() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-real-finalize".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-real-finalize".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request.clone()).expect("start sign round");
        let signing_participants = round_state
            .signing_participants
            .clone()
            .expect("round signing participants");

        let (dkg_key_packages, dkg_public_key_package, sign_message_bytes) = {
            let guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get(&start_request.session_id)
                .expect("session state");

            (
                session.dkg_key_packages.clone().expect("dkg key packages"),
                session
                    .dkg_public_key_package
                    .clone()
                    .expect("dkg public key package"),
                session
                    .sign_message_bytes
                    .clone()
                    .expect("sign message bytes"),
            )
        };

        let member_two_request = StartSignRoundRequest {
            member_identifier: 2,
            attempt_transition_evidence: None,
            ..start_request
        };
        let member_two_contribution = build_real_signature_share_contribution(
            &dkg_key_packages,
            &signing_participants,
            &member_two_request,
            &round_state.round_id,
            &hex::decode(&member_two_request.message_hex).expect("message decode"),
        )
        .expect("member two contribution");
        let member_three_request = StartSignRoundRequest {
            member_identifier: 3,
            attempt_transition_evidence: None,
            ..member_two_request.clone()
        };
        let member_three_contribution = build_real_signature_share_contribution(
            &dkg_key_packages,
            &signing_participants,
            &member_three_request,
            &round_state.round_id,
            &hex::decode(&member_three_request.message_hex).expect("message decode"),
        )
        .expect("member three contribution");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-real-finalize".to_string(),
            attempt_context: None,
            round_contributions: vec![
                round_state.own_contribution.clone(),
                member_two_contribution,
                member_three_contribution,
            ],
        };

        let first_result = finalize_sign_round(finalize_request.clone(), false).expect("finalize");
        let second_result = finalize_sign_round(finalize_request, false).expect("finalize retry");

        assert_eq!(first_result, second_result);
        assert_eq!(first_result.round_id, round_state.round_id);
        let signature_bytes = hex::decode(&first_result.signature_hex).expect("signature decode");
        assert_eq!(signature_bytes.len(), 64);
        let signature = frost::Signature::deserialize(&signature_bytes).expect("signature parse");
        dkg_public_key_package
            .verifying_key()
            .verify(&sign_message_bytes, &signature)
            .expect("signature verification");
    }

    #[test]
    fn finalize_aggregates_real_threshold_subset_outside_bootstrap_mode() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-real-threshold-subset".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-real-threshold-subset".to_string(),
            member_identifier: 1,
            message_hex: "cafef00d".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request.clone()).expect("start sign round");
        let signing_participants = round_state
            .signing_participants
            .clone()
            .expect("round signing participants");

        let (dkg_key_packages, dkg_public_key_package, sign_message_bytes) = {
            let guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get(&start_request.session_id)
                .expect("session state");

            (
                session.dkg_key_packages.clone().expect("dkg key packages"),
                session
                    .dkg_public_key_package
                    .clone()
                    .expect("dkg public key package"),
                session
                    .sign_message_bytes
                    .clone()
                    .expect("sign message bytes"),
            )
        };

        let member_two_request = StartSignRoundRequest {
            member_identifier: 2,
            attempt_transition_evidence: None,
            ..start_request
        };
        let member_two_contribution = build_real_signature_share_contribution(
            &dkg_key_packages,
            &signing_participants,
            &member_two_request,
            &round_state.round_id,
            &hex::decode(&member_two_request.message_hex).expect("message decode"),
        )
        .expect("member two contribution");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-real-threshold-subset".to_string(),
            attempt_context: None,
            round_contributions: vec![
                round_state.own_contribution.clone(),
                member_two_contribution,
            ],
        };

        let first_result = finalize_sign_round(finalize_request.clone(), false).expect("finalize");
        let second_result = finalize_sign_round(finalize_request, false).expect("finalize retry");

        assert_eq!(first_result, second_result);
        assert_eq!(first_result.round_id, round_state.round_id);
        let signature_bytes = hex::decode(&first_result.signature_hex).expect("signature decode");
        assert_eq!(signature_bytes.len(), 64);
        let signature = frost::Signature::deserialize(&signature_bytes).expect("signature parse");
        dkg_public_key_package
            .verifying_key()
            .verify(&sign_message_bytes, &signature)
            .expect("signature verification");
    }

    #[test]
    fn deterministic_round_nonce_and_commitment_is_message_bound() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-nonce-message-bound".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        run_dkg(run_dkg_request).expect("run dkg");

        let key_package = {
            let guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get("session-nonce-message-bound")
                .expect("session state");

            session
                .dkg_key_packages
                .as_ref()
                .expect("dkg key packages")
                .get(&1)
                .expect("key package")
                .clone()
        };

        let session_id = "session-nonce-message-bound";
        let round_id = "fixed-round-id";
        let participant_identifier = 1u16;
        let message_one = hex::decode("deadbeef").expect("message one decode");
        let message_two = hex::decode("cafebabe").expect("message two decode");

        let (_, commitments_one) = build_deterministic_round_nonce_and_commitment(
            &key_package,
            session_id,
            round_id,
            &message_one,
            participant_identifier,
        );
        let (_, commitments_one_retry) = build_deterministic_round_nonce_and_commitment(
            &key_package,
            session_id,
            round_id,
            &message_one,
            participant_identifier,
        );
        let (_, commitments_two) = build_deterministic_round_nonce_and_commitment(
            &key_package,
            session_id,
            round_id,
            &message_two,
            participant_identifier,
        );

        assert_eq!(commitments_one, commitments_one_retry);
        assert_ne!(commitments_one, commitments_two);
    }

    #[test]
    fn deterministic_seed_disambiguates_embedded_zero_bytes() {
        let parts_a = [b"\xaa\x00".as_slice(), b"\x01".as_slice()];
        let parts_b = [b"\xaa".as_slice(), b"\x00\x01".as_slice()];

        assert_ne!(deterministic_seed(&parts_a), deterministic_seed(&parts_b));
    }

    #[test]
    fn finalize_rejects_tampered_session_message_bytes() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-message-tamper".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-message-tamper".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request.clone()).expect("start sign round");
        let signing_participants = round_state
            .signing_participants
            .clone()
            .expect("round signing participants");

        let dkg_key_packages = {
            let guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get(&start_request.session_id)
                .expect("session state");

            session.dkg_key_packages.clone().expect("dkg key packages")
        };

        let member_two_request = StartSignRoundRequest {
            member_identifier: 2,
            attempt_transition_evidence: None,
            ..start_request.clone()
        };
        let member_two_contribution = build_real_signature_share_contribution(
            &dkg_key_packages,
            &signing_participants,
            &member_two_request,
            &round_state.round_id,
            &hex::decode(&member_two_request.message_hex).expect("message decode"),
        )
        .expect("member two contribution");

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut(&start_request.session_id)
                .expect("session state");

            session.sign_message_bytes = Some(Zeroizing::new(
                hex::decode("cafebabe").expect("tamper decode"),
            ));
        }

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-message-tamper".to_string(),
            attempt_context: None,
            round_contributions: vec![
                round_state.own_contribution.clone(),
                member_two_contribution,
            ],
        };

        let err = finalize_sign_round(finalize_request, false).expect_err("expected failure");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };

        assert!(
            message.contains("failed to aggregate signature shares"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn finalize_rejects_real_contributor_set_mismatch_with_explicit_error() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-real-contributor-set-mismatch".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-real-contributor-set-mismatch".to_string(),
            member_identifier: 1,
            message_hex: "b16b00b5".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request.clone()).expect("start sign round");
        let signing_participants = round_state
            .signing_participants
            .clone()
            .expect("round signing participants");

        let dkg_key_packages = {
            let guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get(&start_request.session_id)
                .expect("session state");

            session.dkg_key_packages.clone().expect("dkg key packages")
        };

        let member_two_request = StartSignRoundRequest {
            member_identifier: 2,
            attempt_transition_evidence: None,
            ..start_request
        };
        let member_two_contribution = build_real_signature_share_contribution(
            &dkg_key_packages,
            &signing_participants,
            &member_two_request,
            &round_state.round_id,
            &hex::decode(&member_two_request.message_hex).expect("message decode"),
        )
        .expect("member two contribution");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-real-contributor-set-mismatch".to_string(),
            attempt_context: None,
            round_contributions: vec![
                round_state.own_contribution.clone(),
                member_two_contribution,
            ],
        };

        let err = finalize_sign_round(finalize_request, false).expect_err("expected mismatch");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };

        assert!(
            message.contains(
                "round contribution identifiers must match signing participants for real finalize"
            ),
            "unexpected validation message: {message}"
        );
        assert!(
            message.contains("[1, 2, 3]"),
            "expected identifier set in message: {message}"
        );
        assert!(
            message.contains("[1, 2]"),
            "expected contributor set in message: {message}"
        );
    }

    #[test]
    fn finalize_rejects_real_contribution_identifier_outside_signing_cohort() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-real-outside-signing-cohort".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-real-outside-signing-cohort".to_string(),
            member_identifier: 1,
            message_hex: "facefeed".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-real-outside-signing-cohort".to_string(),
            attempt_context: None,
            round_contributions: vec![
                round_state.own_contribution,
                RoundContribution {
                    identifier: 3,
                    signature_share_hex: "abcd".to_string(),
                },
            ],
        };

        let err = finalize_sign_round(finalize_request, false).expect_err("expected rejection");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("round contribution identifier [3] is not in signing participant set"),
            "unexpected validation message: {message}"
        );
    }

    #[test]
    fn run_dkg_conflict_persists_across_storage_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("run_dkg_conflict_persists");
        reset_for_tests();

        let request_a = RunDkgRequest {
            session_id: "session-persisted-conflict".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let mut request_b = request_a.clone();
        request_b.participants.push(crate::api::DkgParticipant {
            identifier: 3,
            public_key_hex: "02cc".to_string(),
        });

        run_dkg(request_a).expect("initial run dkg");
        reload_state_from_storage_for_tests();

        let err = run_dkg(request_b).expect_err("expected persisted session conflict");
        assert!(matches!(err, EngineError::SessionConflict { .. }));

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn persisted_engine_state_rejects_session_registry_over_limit() {
        let _guard = lock_test_state();
        clear_state_storage_policy_overrides();
        std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

        let mut sessions = HashMap::new();
        sessions.insert("session-a".to_string(), persisted_session_state_fixture());
        sessions.insert("session-b".to_string(), persisted_session_state_fixture());
        sessions.insert("session-c".to_string(), persisted_session_state_fixture());

        let persisted = PersistedEngineState {
            schema_version: PERSISTED_STATE_SCHEMA_VERSION,
            sessions,
            refresh_epoch_counter: 0,
            operator_fault_scores: BTreeMap::new(),
            quarantined_operator_identifiers: vec![],
            canary_rollout: CanaryRolloutState::default(),
        };

        let err = match EngineState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "persisted session registry size [3] exceeds max [2]");

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn max_sessions_limit_env_parser_is_strict_positive() {
        let _guard = lock_test_state();
        clear_state_storage_policy_overrides();

        assert_eq!(max_sessions_limit(), TBTC_SIGNER_DEFAULT_MAX_SESSIONS);

        std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "not-a-number");
        assert_eq!(max_sessions_limit(), TBTC_SIGNER_DEFAULT_MAX_SESSIONS);

        std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "0");
        assert_eq!(max_sessions_limit(), TBTC_SIGNER_DEFAULT_MAX_SESSIONS);

        std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "-1");
        assert_eq!(max_sessions_limit(), TBTC_SIGNER_DEFAULT_MAX_SESSIONS);

        std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "   7   ");
        assert_eq!(max_sessions_limit(), 7);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn roast_coordinator_timeout_ms_env_parser_is_strict_bounds() {
        let _guard = lock_test_state();
        clear_state_storage_policy_overrides();

        assert_eq!(
            roast_coordinator_timeout_ms(),
            TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
        );

        std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "not-a-number");
        assert_eq!(
            roast_coordinator_timeout_ms(),
            TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
        );

        std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "0");
        assert_eq!(
            roast_coordinator_timeout_ms(),
            TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
        );

        std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "999");
        assert_eq!(
            roast_coordinator_timeout_ms(),
            TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
        );

        std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "300001");
        assert_eq!(
            roast_coordinator_timeout_ms(),
            TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
        );

        std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, " 45000 ");
        assert_eq!(roast_coordinator_timeout_ms(), 45_000);

        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_rejects_new_session_when_session_registry_is_at_capacity() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("run_dkg_session_capacity");
        reset_for_tests();
        std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "1");

        let request_a = RunDkgRequest {
            session_id: "session-capacity-a".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        run_dkg(request_a.clone()).expect("initial run dkg");
        run_dkg(request_a).expect("idempotent run dkg at capacity");

        let request_b = RunDkgRequest {
            session_id: "session-capacity-b".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "03aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "03bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let err = run_dkg(request_b).expect_err("expected session cap rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("session registry size [1] reached max [1]"),
            "unexpected internal message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_uses_secret_entropy_for_new_sessions_and_cache_for_retries() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("run_dkg_secret_entropy");
        reset_for_tests();

        let request_a = RunDkgRequest {
            session_id: "session-secret-entropy-a".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };
        let mut request_b = request_a.clone();
        request_b.session_id = "session-secret-entropy-b".to_string();

        let result_a = run_dkg(request_a.clone()).expect("run dkg a");
        let retry_a = run_dkg(request_a).expect("retry dkg a");
        let result_b = run_dkg(request_b).expect("run dkg b");

        assert_eq!(result_a, retry_a);
        assert_ne!(
            result_a.key_group, result_b.key_group,
            "new sessions with the same public DKG request shape must not derive dealer entropy from public request data"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn run_dkg_retry_is_participant_order_insensitive() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("run_dkg_participant_order_retry");
        reset_for_tests();

        let request = RunDkgRequest {
            session_id: "session-dkg-participant-order-retry".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let mut retry_request = request.clone();
        retry_request.participants.reverse();

        let first_result = run_dkg(request).expect("initial DKG");
        let retry_result = run_dkg(retry_request).expect("equivalent DKG retry");

        assert_eq!(first_result, retry_result);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_rejects_new_session_when_session_registry_is_at_capacity() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("build_taproot_tx_session_capacity");
        reset_for_tests();
        std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "1");

        let first_request = BuildTaprootTxRequest {
            session_id: "session-build-tx-capacity-a".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 8_000,
            }],
            script_tree_hex: None,
        };
        build_taproot_tx(first_request.clone()).expect("first build tx");
        build_taproot_tx(first_request).expect("idempotent build tx at capacity");

        let second_request = BuildTaprootTxRequest {
            session_id: "session-build-tx-capacity-b".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "33".repeat(32),
                vout: 0,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "44".repeat(32)),
                value_sats: 8_000,
            }],
            script_tree_hex: None,
        };
        let err = build_taproot_tx(second_request).expect_err("expected session cap rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("session registry size [1] reached max [1]"),
            "unexpected internal message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn refresh_shares_rejects_new_session_when_session_registry_is_at_capacity() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("refresh_session_capacity");
        reset_for_tests();
        std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "1");

        let first_request = RefreshSharesRequest {
            session_id: "session-refresh-capacity-a".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "aa11".to_string(),
            }],
        };
        refresh_shares(first_request.clone()).expect("first refresh");
        refresh_shares(first_request).expect("idempotent refresh at capacity");

        let second_request = RefreshSharesRequest {
            session_id: "session-refresh-capacity-b".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "bb22".to_string(),
            }],
        };
        let err = refresh_shares(second_request).expect_err("expected session cap rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("session registry size [1] reached max [1]"),
            "unexpected internal message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn refresh_shares_retry_is_share_order_insensitive() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("refresh_share_order_retry");
        reset_for_tests();

        let request = RefreshSharesRequest {
            session_id: "session-refresh-share-order-retry".to_string(),
            current_shares: vec![
                ShareMaterial {
                    identifier: 3,
                    encrypted_share_hex: "cccc".to_string(),
                },
                ShareMaterial {
                    identifier: 1,
                    encrypted_share_hex: "aaaa".to_string(),
                },
                ShareMaterial {
                    identifier: 2,
                    encrypted_share_hex: "bbbb".to_string(),
                },
            ],
        };
        let mut retry_request = request.clone();
        retry_request.current_shares.reverse();

        let first_result = refresh_shares(request).expect("initial refresh");
        let retry_result = refresh_shares(retry_request).expect("equivalent refresh retry");

        assert_eq!(first_result, retry_result);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn refresh_shares_rejects_duplicate_current_share_identifiers() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("refresh_duplicate_share_identifier");
        reset_for_tests();

        let err = refresh_shares(RefreshSharesRequest {
            session_id: "session-refresh-duplicate-share-id".to_string(),
            current_shares: vec![
                ShareMaterial {
                    identifier: 1,
                    encrypted_share_hex: "aaaa".to_string(),
                },
                ShareMaterial {
                    identifier: 1,
                    encrypted_share_hex: "bbbb".to_string(),
                },
            ],
        })
        .expect_err("expected duplicate share identifier rejection");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("current_shares contains duplicate identifier [1]"),
            "unexpected validation message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn refresh_shares_rejects_zero_current_share_identifier() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("refresh_zero_share_identifier");
        reset_for_tests();

        let err = refresh_shares(RefreshSharesRequest {
            session_id: "session-refresh-zero-share-id".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 0,
                encrypted_share_hex: "aaaa".to_string(),
            }],
        })
        .expect_err("expected zero share identifier rejection");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("current_shares identifiers must be non-zero"),
            "unexpected validation message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn sign_round_and_finalize_idempotency_persist_across_storage_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("sign_finalize_idempotency");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-persisted-idempotency".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let start_request = StartSignRoundRequest {
            session_id: "session-persisted-idempotency".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let first_round_state = start_sign_round(start_request.clone()).expect("start sign round");

        reload_state_from_storage_for_tests();
        let second_round_state = start_sign_round(start_request).expect("persisted start retry");
        assert_eq!(first_round_state, second_round_state);

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-persisted-idempotency".to_string(),
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&first_round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&first_round_state, 2),
                },
            ],
        };

        let first_signature =
            finalize_sign_round(finalize_request.clone(), true).expect("initial finalize");
        reload_state_from_storage_for_tests();
        let second_signature =
            finalize_sign_round(finalize_request, true).expect("persisted finalize retry");
        assert_eq!(first_signature, second_signature);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn persisted_session_state_rejects_empty_consumed_attempt_id() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_attempt_ids = vec!["".to_string()];

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "persisted consumed attempt ID must be non-empty");
    }

    #[test]
    fn persisted_session_state_rejects_duplicate_consumed_attempt_id() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_attempt_ids = vec!["attempt-a".to_string(), "attempt-a".to_string()];

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "duplicate persisted consumed attempt ID [attempt-a]");
    }

    #[test]
    fn persisted_session_state_rejects_empty_consumed_sign_round_id() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_sign_round_ids = vec!["".to_string()];

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "persisted consumed sign round ID must be non-empty");
    }

    #[test]
    fn persisted_session_state_rejects_duplicate_consumed_sign_round_id() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_sign_round_ids = vec!["round-a".to_string(), "round-a".to_string()];

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "duplicate persisted consumed sign round ID [round-a]");
    }

    #[test]
    fn persisted_session_state_rejects_empty_consumed_finalize_round_id() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_finalize_round_ids = vec!["".to_string()];

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(
            err,
            "persisted consumed finalize round ID must be non-empty",
        );
    }

    #[test]
    fn persisted_session_state_rejects_duplicate_consumed_finalize_round_id() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_finalize_round_ids = vec!["round-b".to_string(), "round-b".to_string()];

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(
            err,
            "duplicate persisted consumed finalize round ID [round-b]",
        );
    }

    #[test]
    fn persisted_session_state_rejects_empty_consumed_finalize_request_fingerprint() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_finalize_request_fingerprints = vec!["".to_string()];

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(
            err,
            "persisted consumed finalize request fingerprint must be non-empty",
        );
    }

    #[test]
    fn persisted_session_state_rejects_duplicate_consumed_finalize_request_fingerprint() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_finalize_request_fingerprints =
            vec!["fp-1".to_string(), "fp-1".to_string()];

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(
            err,
            "duplicate persisted consumed finalize request fingerprint [fp-1]",
        );
    }

    #[test]
    fn persisted_session_state_rejects_consumed_attempt_registry_over_limit() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_attempt_ids = (0
            ..=TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION)
            .map(|idx| format!("attempt-{idx}"))
            .collect();

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "persisted consumed_attempt_ids registry size");
    }

    #[test]
    fn persisted_session_state_rejects_consumed_sign_round_registry_over_limit() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_sign_round_ids = (0
            ..=TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION)
            .map(|idx| format!("round-{idx}"))
            .collect();

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "persisted consumed_sign_round_ids registry size");
    }

    #[test]
    fn persisted_session_state_rejects_consumed_finalize_round_registry_over_limit() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_finalize_round_ids = (0
            ..=TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION)
            .map(|idx| format!("round-{idx}"))
            .collect();

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "persisted consumed_finalize_round_ids registry size");
    }

    #[test]
    fn persisted_session_state_rejects_consumed_finalize_request_registry_over_limit() {
        let mut persisted = persisted_session_state_fixture();
        persisted.consumed_finalize_request_fingerprints = (0
            ..=TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION)
            .map(|idx| format!("fp-{idx}"))
            .collect();

        let err = match SessionState::try_from(persisted) {
            Ok(_) => panic!("expected decode rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(
            err,
            "persisted consumed_finalize_request_fingerprints registry size",
        );
    }

    #[test]
    fn start_sign_round_rejects_consumed_round_id_when_sign_cache_is_missing() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("sign_round_consumed_nonce_enforcement");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-sign-round-consumed-nonce".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let start_request = StartSignRoundRequest {
            session_id: "session-sign-round-consumed-nonce".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let first_round_state = start_sign_round(start_request.clone()).expect("start sign round");

        reload_state_from_storage_for_tests();

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut("session-sign-round-consumed-nonce")
                .expect("session state");
            assert!(session
                .consumed_sign_round_ids
                .contains(&first_round_state.round_id));
            session.sign_request_fingerprint = None;
            session.sign_message_bytes = None;
            session.round_state = None;
            persist_engine_state_to_storage(&guard).expect("persist tampered sign cache state");
        }

        reload_state_from_storage_for_tests();
        let err = start_sign_round(start_request).expect_err("expected consumed round rejection");
        let EngineError::ConsumedRoundReplay {
            round_id,
            session_id: _,
        } = err
        else {
            panic!("unexpected error variant");
        };
        assert_eq!(round_id, first_round_state.round_id);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_replay_guard_survives_process_restart_with_sign_cache_loss() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("sign_round_consumed_nonce_restart_replay");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-sign-round-consumed-nonce-restart".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let start_request = StartSignRoundRequest {
            session_id: "session-sign-round-consumed-nonce-restart".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let first_round_state = start_sign_round(start_request.clone()).expect("start sign round");

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut("session-sign-round-consumed-nonce-restart")
                .expect("session state");
            assert!(session
                .consumed_sign_round_ids
                .contains(&first_round_state.round_id));
            session.sign_request_fingerprint = None;
            session.sign_message_bytes = None;
            session.round_state = None;
            persist_engine_state_to_storage(&guard).expect("persist tampered sign cache state");
        }

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();
        let err = start_sign_round(start_request).expect_err("expected consumed round rejection");
        let EngineError::ConsumedRoundReplay {
            round_id,
            session_id: _,
        } = err
        else {
            panic!("unexpected error variant");
        };
        assert_eq!(round_id, first_round_state.round_id);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_rejects_consumed_attempt_id_when_sign_cache_is_missing() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("sign_round_consumed_attempt_enforcement");
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-sign-round-consumed-attempt";
        let message_hex = "deadbeef";
        let run_dkg_request = RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
        let expected_attempt_id = attempt_context.attempt_id.clone();
        let start_request = StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        };
        start_sign_round(start_request.clone()).expect("start sign round");

        reload_state_from_storage_for_tests();

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard.sessions.get_mut(session_id).expect("session state");
            assert!(session.consumed_attempt_ids.contains(&expected_attempt_id));
            session.sign_request_fingerprint = None;
            session.sign_message_bytes = None;
            session.round_state = None;
            persist_engine_state_to_storage(&guard).expect("persist tampered sign cache state");
        }

        reload_state_from_storage_for_tests();
        let err =
            start_sign_round(start_request).expect_err("expected consumed attempt-id rejection");
        let EngineError::ConsumedAttemptReplay {
            attempt_id,
            session_id: _,
        } = err
        else {
            panic!("unexpected error variant");
        };
        assert_eq!(attempt_id, expected_attempt_id);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_attempt_replay_guard_survives_process_restart_with_sign_cache_loss() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("sign_round_consumed_attempt_restart_replay");
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-sign-round-consumed-attempt-restart";
        let message_hex = "deadbeef";
        let run_dkg_request = RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
        let expected_attempt_id = attempt_context.attempt_id.clone();
        let start_request = StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        };
        start_sign_round(start_request.clone()).expect("start sign round");

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard.sessions.get_mut(session_id).expect("session state");
            assert!(session.consumed_attempt_ids.contains(&expected_attempt_id));
            session.sign_request_fingerprint = None;
            session.sign_message_bytes = None;
            session.round_state = None;
            persist_engine_state_to_storage(&guard).expect("persist tampered sign cache state");
        }

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();
        let err =
            start_sign_round(start_request).expect_err("expected consumed attempt-id rejection");
        let EngineError::ConsumedAttemptReplay {
            attempt_id,
            session_id: _,
        } = err
        else {
            panic!("unexpected error variant");
        };
        assert_eq!(attempt_id, expected_attempt_id);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn persist_fault_after_temp_sync_before_rename_preserves_previous_state_on_restart() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("persist_fault_before_rename");
        reset_for_tests();

        let existing_request = RunDkgRequest {
            session_id: "session-persist-fault-existing".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        run_dkg(existing_request).expect("seed existing persisted session");

        let failed_request = RunDkgRequest {
            session_id: "session-persist-fault-before-rename".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "03aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "03bb".to_string(),
                },
            ],
            threshold: 2,
        };

        set_persist_fault_injection_for_tests(
            PersistFaultInjectionPoint::AfterTempSyncBeforeRename,
        );
        let err = run_dkg(failed_request).expect_err("expected injected persist failure");
        clear_persist_fault_injection_for_tests();

        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("injected persist fault at [after_temp_sync_before_rename]"),
            "unexpected persist fault message: {message}"
        );
        assert!(
            !state_path
                .with_extension(format!("tmp-{}", std::process::id()))
                .exists(),
            "persist temp state file should be cleaned up on failure"
        );

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();

        {
            let guard = state().expect("engine state").lock().expect("engine lock");
            assert!(guard
                .sessions
                .contains_key("session-persist-fault-existing"));
            assert!(!guard
                .sessions
                .contains_key("session-persist-fault-before-rename"));
        }

        run_dkg(RunDkgRequest {
            session_id: "session-persist-fault-recovery".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "04aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "04bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("post-fault recovery run dkg");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_rejects_when_consumed_sign_round_registry_is_at_capacity() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("sign_round_consumed_capacity");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-sign-round-consumed-capacity".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut("session-sign-round-consumed-capacity")
                .expect("session state");

            for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
                session
                    .consumed_sign_round_ids
                    .insert(format!("preused-round-{idx}"));
            }
            persist_engine_state_to_storage(&guard)
                .expect("persist prefilled consumed sign rounds");
        }

        let start_request = StartSignRoundRequest {
            session_id: "session-sign-round-consumed-capacity".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let err = start_sign_round(start_request).expect_err("expected capacity rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("consumed_sign_round_ids registry size"),
            "unexpected internal message: {message}"
        );
        assert!(
            message.contains("reached max"),
            "unexpected internal message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_rejects_when_consumed_sign_round_registry_is_at_capacity_with_attempt_context(
    ) {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("sign_round_consumed_capacity_attempt_context");
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-sign-round-consumed-capacity-attempt-context";
        let message_hex = "deadbeef";
        let run_dkg_request = RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard.sessions.get_mut(session_id).expect("session state");

            for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
                session
                    .consumed_sign_round_ids
                    .insert(format!("preused-round-{idx}"));
            }
            persist_engine_state_to_storage(&guard)
                .expect("persist prefilled consumed sign rounds");
        }

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
        let start_request = StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        };
        let err = start_sign_round(start_request).expect_err("expected capacity rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("consumed_sign_round_ids registry size"),
            "unexpected internal message: {message}"
        );
        assert!(
            message.contains("reached max"),
            "unexpected internal message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_rejects_when_consumed_attempt_registry_is_at_capacity_with_attempt_context()
    {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("sign_round_consumed_attempt_capacity");
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-sign-round-consumed-attempt-capacity";
        let message_hex = "deadbeef";
        let run_dkg_request = RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard.sessions.get_mut(session_id).expect("session state");

            for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
                session
                    .consumed_attempt_ids
                    .insert(format!("preused-attempt-{idx}"));
            }
            persist_engine_state_to_storage(&guard)
                .expect("persist prefilled consumed attempt IDs");
        }

        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
        let start_request = StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context),
            attempt_transition_evidence: None,
        };
        let err = start_sign_round(start_request).expect_err("expected capacity rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("consumed_attempt_ids registry size"),
            "unexpected internal message: {message}"
        );
        assert!(
            message.contains("reached max"),
            "unexpected internal message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_rejects_consumed_round_id_when_finalize_cache_is_missing() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("finalize_consumed_round_enforcement");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-consumed-round".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-consumed-round".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-consumed-round".to_string(),
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
        finalize_sign_round(finalize_request.clone(), true).expect("first finalize");

        reload_state_from_storage_for_tests();

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut("session-finalize-consumed-round")
                .expect("session state");
            assert!(session
                .consumed_finalize_round_ids
                .contains(&round_state.round_id));
            session.finalize_request_fingerprint = None;
            session.signature_result = None;
            session.round_state = Some(round_state.clone());
            persist_engine_state_to_storage(&guard).expect("persist tampered finalize cache state");
        }

        let round_only_replay_request = FinalizeSignRoundRequest {
            session_id: finalize_request.session_id.clone(),
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: format!(
                        "{}00",
                        bootstrap_synthetic_share_hex(&round_state, 1)
                    ),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
        };

        reload_state_from_storage_for_tests();
        let err = finalize_sign_round(round_only_replay_request, true)
            .expect_err("expected consumed round-id rejection");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("already consumed for finalize"),
            "unexpected validation message: {message}"
        );
        assert!(
            message.contains(&round_state.round_id),
            "unexpected validation message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn persist_fault_after_rename_before_directory_sync_keeps_state_loadable_after_restart() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("persist_fault_after_rename");
        reset_for_tests();

        let existing_request = RunDkgRequest {
            session_id: "session-persist-fault-existing-after-rename".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        run_dkg(existing_request).expect("seed existing persisted session");

        let renamed_request = RunDkgRequest {
            session_id: "session-persist-fault-after-rename".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "03aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "03bb".to_string(),
                },
            ],
            threshold: 2,
        };

        set_persist_fault_injection_for_tests(
            PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
        );
        let err = run_dkg(renamed_request.clone()).expect_err("expected injected persist failure");
        clear_persist_fault_injection_for_tests();

        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("injected persist fault at [after_rename_before_directory_sync]"),
            "unexpected persist fault message: {message}"
        );
        assert!(
            !state_path
                .with_extension(format!("tmp-{}", std::process::id()))
                .exists(),
            "persist temp state file should not remain after post-rename failure"
        );

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();

        {
            let guard = state().expect("engine state").lock().expect("engine lock");
            assert!(guard
                .sessions
                .contains_key("session-persist-fault-existing-after-rename"));
            assert!(guard
                .sessions
                .contains_key("session-persist-fault-after-rename"));
        }

        let retry_result = run_dkg(renamed_request).expect("retry request after reload");
        assert_eq!(
            retry_result.session_id,
            "session-persist-fault-after-rename"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_rejects_when_consumed_request_registry_is_at_capacity() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("finalize_consumed_request_capacity");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-consumed-request-capacity".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-consumed-request-capacity".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut("session-finalize-consumed-request-capacity")
                .expect("session state");

            for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
                session
                    .consumed_finalize_request_fingerprints
                    .insert(format!("prefilled-fingerprint-{idx}"));
            }
            persist_engine_state_to_storage(&guard)
                .expect("persist prefilled consumed finalize request fingerprints");
        }

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-consumed-request-capacity".to_string(),
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
        let err =
            finalize_sign_round(finalize_request, true).expect_err("expected capacity rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("consumed_finalize_request_fingerprints registry size"),
            "unexpected internal message: {message}"
        );
        assert!(
            message.contains("reached max"),
            "unexpected internal message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_rejects_when_consumed_request_registry_is_at_capacity_with_attempt_context(
    ) {
        let _guard = lock_test_state();
        let state_path =
            configure_test_state_path("finalize_consumed_request_capacity_attempt_context");
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-finalize-consumed-request-capacity-attempt-context";
        let message_hex = "deadbeef";
        let run_dkg_request = RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let mut uppercase_attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
        uppercase_attempt_context.included_participants_fingerprint = uppercase_attempt_context
            .included_participants_fingerprint
            .to_ascii_uppercase();
        uppercase_attempt_context.attempt_id =
            uppercase_attempt_context.attempt_id.to_ascii_uppercase();

        let start_request = StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(uppercase_attempt_context.clone()),
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard.sessions.get_mut(session_id).expect("session state");

            for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
                session
                    .consumed_finalize_request_fingerprints
                    .insert(format!("prefilled-fingerprint-{idx}"));
            }
            persist_engine_state_to_storage(&guard)
                .expect("persist prefilled consumed finalize request fingerprints");
        }

        let finalize_request = FinalizeSignRoundRequest {
            session_id: session_id.to_string(),
            attempt_context: Some(uppercase_attempt_context),
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
        let err =
            finalize_sign_round(finalize_request, true).expect_err("expected capacity rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("consumed_finalize_request_fingerprints registry size"),
            "unexpected internal message: {message}"
        );
        assert!(
            message.contains("reached max"),
            "unexpected internal message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_rejects_when_consumed_round_registry_is_at_capacity() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("finalize_consumed_round_capacity");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-consumed-round-capacity".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-consumed-round-capacity".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut("session-finalize-consumed-round-capacity")
                .expect("session state");

            for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
                session
                    .consumed_finalize_round_ids
                    .insert(format!("prefilled-round-{idx}"));
            }
            persist_engine_state_to_storage(&guard)
                .expect("persist prefilled consumed finalize round IDs");
        }

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-consumed-round-capacity".to_string(),
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
        let err =
            finalize_sign_round(finalize_request, true).expect_err("expected capacity rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("consumed_finalize_round_ids registry size"),
            "unexpected internal message: {message}"
        );
        assert!(
            message.contains("reached max"),
            "unexpected internal message: {message}"
        );

        {
            let guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get("session-finalize-consumed-round-capacity")
                .expect("session state");
            assert!(session.finalize_request_fingerprint.is_none());
            assert!(session.signature_result.is_none());
        }

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_rejects_when_consumed_round_registry_is_at_capacity_with_attempt_context(
    ) {
        let _guard = lock_test_state();
        let state_path =
            configure_test_state_path("finalize_consumed_round_capacity_attempt_context");
        reset_for_tests();
        let _roast_strict_mode = RoastStrictModeGuard::enable();

        let session_id = "session-finalize-consumed-round-capacity-attempt-context";
        let message_hex = "deadbeef";
        let run_dkg_request = RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let attempt_context =
            build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
        let start_request = StartSignRoundRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: message_hex.to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![1, 2]),
            attempt_context: Some(attempt_context.clone()),
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard.sessions.get_mut(session_id).expect("session state");

            for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
                session
                    .consumed_finalize_round_ids
                    .insert(format!("prefilled-round-{idx}"));
            }
            persist_engine_state_to_storage(&guard)
                .expect("persist prefilled consumed finalize round IDs");
        }

        let finalize_request = FinalizeSignRoundRequest {
            session_id: session_id.to_string(),
            attempt_context: Some(attempt_context),
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
        let err =
            finalize_sign_round(finalize_request, true).expect_err("expected capacity rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("consumed_finalize_round_ids registry size"),
            "unexpected internal message: {message}"
        );
        assert!(
            message.contains("reached max"),
            "unexpected internal message: {message}"
        );

        {
            let guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard.sessions.get(session_id).expect("session state");
            assert!(session.finalize_request_fingerprint.is_none());
            assert!(session.signature_result.is_none());
        }

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_rejects_consumed_request_fingerprint_when_round_state_missing() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("finalize_consumed_request_fingerprint");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-consumed-request-fingerprint".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-consumed-request-fingerprint".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-consumed-request-fingerprint".to_string(),
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
        let mut canonical_contributions = finalize_request.round_contributions.clone();
        canonical_contributions.sort_unstable_by(|left, right| {
            left.identifier
                .cmp(&right.identifier)
                .then_with(|| left.signature_share_hex.cmp(&right.signature_share_hex))
        });
        let expected_request_fingerprint = fingerprint(&FinalizeSignRoundRequest {
            session_id: finalize_request.session_id.clone(),
            attempt_context: None,
            round_contributions: canonical_contributions,
        })
        .expect("finalize request fingerprint");

        finalize_sign_round(finalize_request.clone(), true).expect("first finalize");
        reload_state_from_storage_for_tests();

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut("session-finalize-consumed-request-fingerprint")
                .expect("session state");
            assert!(session
                .consumed_finalize_request_fingerprints
                .contains(&expected_request_fingerprint));
            assert!(session.round_state.is_none());
            session.finalize_request_fingerprint = None;
            session.signature_result = None;
            persist_engine_state_to_storage(&guard)
                .expect("persist tampered finalize request cache state");
        }

        reload_state_from_storage_for_tests();
        let err = finalize_sign_round(finalize_request, true)
            .expect_err("expected consumed request fingerprint rejection");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("finalize request fingerprint"),
            "unexpected validation message: {message}"
        );
        assert!(
            message.contains("already consumed"),
            "unexpected validation message: {message}"
        );
        assert!(
            message.contains(&expected_request_fingerprint),
            "unexpected validation message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_sign_round_replay_guard_survives_process_restart_with_finalize_cache_loss() {
        let _guard = lock_test_state();
        let state_path =
            configure_test_state_path("finalize_consumed_request_fingerprint_restart_replay");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-consumed-request-fingerprint-restart".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-consumed-request-fingerprint-restart".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-consumed-request-fingerprint-restart".to_string(),
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
        let mut canonical_contributions = finalize_request.round_contributions.clone();
        canonical_contributions.sort_unstable_by(|left, right| {
            left.identifier
                .cmp(&right.identifier)
                .then_with(|| left.signature_share_hex.cmp(&right.signature_share_hex))
        });
        let expected_request_fingerprint = fingerprint(&FinalizeSignRoundRequest {
            session_id: finalize_request.session_id.clone(),
            attempt_context: None,
            round_contributions: canonical_contributions,
        })
        .expect("finalize request fingerprint");

        finalize_sign_round(finalize_request.clone(), true).expect("first finalize");

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();

        {
            let mut guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get_mut("session-finalize-consumed-request-fingerprint-restart")
                .expect("session state");
            assert!(session
                .consumed_finalize_request_fingerprints
                .contains(&expected_request_fingerprint));
            assert!(session.round_state.is_none());
            session.finalize_request_fingerprint = None;
            session.signature_result = None;
            persist_engine_state_to_storage(&guard)
                .expect("persist tampered finalize request cache state");
        }

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();
        let err = finalize_sign_round(finalize_request, true)
            .expect_err("expected consumed request fingerprint rejection");
        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("finalize request fingerprint"),
            "unexpected validation message: {message}"
        );
        assert!(
            message.contains("already consumed"),
            "unexpected validation message: {message}"
        );
        assert!(
            message.contains(&expected_request_fingerprint),
            "unexpected validation message: {message}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn start_sign_round_accepts_reordered_participant_idempotent_retry() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-start-round-reordered-idempotency".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let first_request = StartSignRoundRequest {
            session_id: "session-start-round-reordered-idempotency".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![3, 1, 2]),
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let first_round_state = start_sign_round(first_request).expect("first start sign round");

        let second_request = StartSignRoundRequest {
            session_id: "session-start-round-reordered-idempotency".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![2, 3, 1]),
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let second_round_state =
            start_sign_round(second_request).expect("second start sign round retry");

        assert_eq!(first_round_state, second_round_state);
    }

    #[test]
    fn start_sign_round_rejects_materially_different_retry_after_canonicalization() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-start-round-canonicalization-conflict".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 3,
                    public_key_hex: "02cc".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let first_request = StartSignRoundRequest {
            session_id: "session-start-round-canonicalization-conflict".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group.clone(),
            signing_participants: Some(vec![3, 1, 2]),
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        start_sign_round(first_request).expect("first start sign round");

        let second_request = StartSignRoundRequest {
            session_id: "session-start-round-canonicalization-conflict".to_string(),
            member_identifier: 1,
            message_hex: "cafebabe".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: Some(vec![2, 3, 1]),
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let err = start_sign_round(second_request).expect_err("expected session conflict");
        assert!(matches!(err, EngineError::SessionConflict { .. }));
    }

    #[test]
    fn finalize_sign_round_accepts_reordered_contribution_idempotent_retry() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-reordered-idempotency".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-reordered-idempotency".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        let first_finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-reordered-idempotency".to_string(),
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

        let second_finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-reordered-idempotency".to_string(),
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
            ],
        };

        let first_signature =
            finalize_sign_round(first_finalize_request, true).expect("first finalize");
        let second_signature =
            finalize_sign_round(second_finalize_request, true).expect("second finalize retry");

        assert_eq!(first_signature, second_signature);
    }

    #[test]
    fn finalize_sign_round_rejects_materially_different_retry_after_canonicalization() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-canonicalization-conflict".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-canonicalization-conflict".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        let first_finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-canonicalization-conflict".to_string(),
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
        finalize_sign_round(first_finalize_request, true).expect("first finalize");

        let second_finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-canonicalization-conflict".to_string(),
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: format!(
                        "00{}",
                        bootstrap_synthetic_share_hex(&round_state, 1)
                    ),
                },
            ],
        };
        let err =
            finalize_sign_round(second_finalize_request, true).expect_err("expected conflict");
        assert!(matches!(err, EngineError::SessionConflict { .. }));
    }

    #[test]
    fn refresh_epoch_counter_persists_across_storage_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("refresh_epoch_counter");
        reset_for_tests();

        let first_result = refresh_shares(RefreshSharesRequest {
            session_id: "session-persisted-refresh-1".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "aaaa".to_string(),
            }],
        })
        .expect("first refresh");
        assert_eq!(first_result.refresh_epoch, 1);

        reload_state_from_storage_for_tests();

        let second_result = refresh_shares(RefreshSharesRequest {
            session_id: "session-persisted-refresh-2".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "bbbb".to_string(),
            }],
        })
        .expect("second refresh");
        assert_eq!(second_result.refresh_epoch, 2);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn state_lock_path_is_bound_and_rejects_in_process_path_switch() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("state_lock_path_binding");
        let alternate_state_path = std::env::temp_dir().join(format!(
            "frost_tbtc_engine_state_state_lock_path_binding_alt_{}.json",
            std::process::id()
        ));
        cleanup_test_state_artifacts(&alternate_state_path);
        reset_for_tests();

        refresh_shares(RefreshSharesRequest {
            session_id: "session-lock-path-initial".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "aaaa".to_string(),
            }],
        })
        .expect("initial refresh");

        std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &alternate_state_path);

        let err = refresh_shares(RefreshSharesRequest {
            session_id: "session-lock-path-switch".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "bbbb".to_string(),
            }],
        })
        .expect_err("expected path switch rejection");
        let EngineError::Internal(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("refusing to switch"),
            "unexpected lock path switch error: {message}"
        );

        std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &state_path);
        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        cleanup_test_state_artifacts(&alternate_state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn restart_reload_recovers_persisted_state_across_operation_types() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("restart_reload_integration");
        reset_for_tests();

        let dkg_request = RunDkgRequest {
            session_id: "session-restart-dkg".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let dkg_result = run_dkg(dkg_request.clone()).expect("run dkg");

        let build_request = BuildTaprootTxRequest {
            session_id: "session-restart-buildtx".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 9_000,
            }],
            script_tree_hex: None,
        };
        let build_result = build_taproot_tx(build_request.clone()).expect("build taproot tx");

        let refresh_request = RefreshSharesRequest {
            session_id: "session-restart-refresh".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "abba".to_string(),
            }],
        };
        let refresh_result = refresh_shares(refresh_request.clone()).expect("refresh shares");

        let finalize_dkg_request = RunDkgRequest {
            session_id: "session-restart-finalize".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "03aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "03bb".to_string(),
                },
            ],
            threshold: 2,
        };
        let finalize_dkg_result = run_dkg(finalize_dkg_request).expect("run finalize dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-restart-finalize".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: finalize_dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request).expect("start sign round");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-restart-finalize".to_string(),
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
        let finalize_result =
            finalize_sign_round(finalize_request.clone(), true).expect("finalize sign round");

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();

        {
            let guard = state().expect("engine state").lock().expect("engine lock");
            assert!(guard.sessions.contains_key("session-restart-dkg"));
            assert!(guard.sessions.contains_key("session-restart-buildtx"));
            assert!(guard.sessions.contains_key("session-restart-refresh"));
            assert!(guard.sessions.contains_key("session-restart-finalize"));
        }

        let dkg_retry_result = run_dkg(dkg_request).expect("retry run dkg");
        assert_eq!(dkg_result, dkg_retry_result);

        let build_retry_result = build_taproot_tx(build_request).expect("retry build taproot tx");
        assert_eq!(build_result, build_retry_result);

        let refresh_retry_result = refresh_shares(refresh_request).expect("retry refresh shares");
        assert_eq!(refresh_result, refresh_retry_result);

        let finalize_retry_result =
            finalize_sign_round(finalize_request, true).expect("retry finalize sign round");
        assert_eq!(finalize_result, finalize_retry_result);

        let new_session_result = run_dkg(RunDkgRequest {
            session_id: "session-restart-new".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "04aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "04bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("post-restart run dkg");
        assert!(!new_session_result.key_group.is_empty());

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    #[cfg(unix)]
    fn state_lock_rejects_multi_process_contention() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("state_lock_multi_process_contention");
        let ready_path = std::env::temp_dir().join(format!(
            "frost_tbtc_lock_ready_{}_{}.flag",
            std::process::id(),
            now_unix()
        ));
        let release_path = std::env::temp_dir().join(format!(
            "frost_tbtc_lock_release_{}_{}.flag",
            std::process::id(),
            now_unix()
        ));
        let _ = std::fs::remove_file(&ready_path);
        let _ = std::fs::remove_file(&release_path);
        reset_for_tests();

        if let Ok(mut lock_slot) = state_file_lock_slot().lock() {
            *lock_slot = None;
        }

        let child = Command::new(std::env::current_exe().expect("current test binary path"))
            .arg("--exact")
            .arg("engine::tests::state_file_lock_contention_helper")
            .arg("--ignored")
            .arg("--nocapture")
            .env(TBTC_SIGNER_STATE_PATH_ENV, &state_path)
            .env("TBTC_SIGNER_LOCK_HELPER", "1")
            .env("TBTC_SIGNER_LOCK_READY_PATH", &ready_path)
            .env("TBTC_SIGNER_LOCK_RELEASE_PATH", &release_path)
            .spawn()
            .expect("spawn lock holder helper process");
        let helper_guard = LockHelperProcessGuard::new(child, release_path.clone());

        assert!(
            wait_for_file(&ready_path, Duration::from_secs(10)),
            "helper did not report lock acquisition"
        );

        let err = match ensure_state_file_lock() {
            Ok(_) => panic!("expected lock contention error"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "signer state lock already held by another process");

        helper_guard.wait_for_success();

        let _ = std::fs::remove_file(&ready_path);
        let _ = std::fs::remove_file(&release_path);
        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    #[cfg(unix)]
    fn persisted_state_file_uses_owner_only_permissions() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("state_file_permissions");
        reset_for_tests();

        refresh_shares(RefreshSharesRequest {
            session_id: "session-state-file-permissions".to_string(),
            current_shares: vec![ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "aaaa".to_string(),
            }],
        })
        .expect("persist state via refresh");

        let mode = std::fs::metadata(&state_path)
            .expect("state file metadata")
            .permissions()
            .mode()
            & 0o777;
        assert_eq!(
            mode, 0o600,
            "state file should be owner read/write only, got mode {mode:o}"
        );

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn build_taproot_tx_idempotency_persists_across_storage_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("build_taproot_tx_idempotency");
        reset_for_tests();

        let request = BuildTaprootTxRequest {
            session_id: "session-build-tx".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 9_000,
            }],
            script_tree_hex: None,
        };

        let first_result = build_taproot_tx(request.clone()).expect("first build tx");
        assert!(!first_result.tx_hex.is_empty());

        reload_state_from_storage_for_tests();
        let second_result = build_taproot_tx(request).expect("persisted build tx retry");
        assert_eq!(first_result, second_result);

        let conflict_request = BuildTaprootTxRequest {
            session_id: "session-build-tx".to_string(),
            inputs: vec![crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: 10_000,
            }],
            outputs: vec![crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: 8_000,
            }],
            script_tree_hex: None,
        };

        let err = build_taproot_tx(conflict_request).expect_err("expected build tx conflict");
        assert!(matches!(err, EngineError::SessionConflict { .. }));

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn finalize_clears_signing_material_and_rejects_sign_round_restart() {
        let _guard = lock_test_state();
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-clears-signing-material".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-clears-signing-material".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request.clone()).expect("start sign round");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-clears-signing-material".to_string(),
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

        let first_result = finalize_sign_round(finalize_request.clone(), true).expect("finalize");

        {
            let guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get("session-finalize-clears-signing-material")
                .expect("session state");

            assert!(session.finalize_request_fingerprint.is_some());
            assert!(session.signature_result.is_some());
            assert!(session.dkg_key_packages.is_none());
            assert!(session.dkg_public_key_package.is_none());
            assert!(session.sign_request_fingerprint.is_none());
            assert!(session.sign_message_bytes.is_none());
            assert!(session.round_state.is_none());
        }

        let second_result =
            finalize_sign_round(finalize_request, true).expect("finalize idempotent retry");
        assert_eq!(first_result, second_result);

        let err = start_sign_round(start_request).expect_err("start sign round should fail");
        assert!(matches!(err, EngineError::SessionFinalized { .. }));
    }

    #[test]
    fn finalize_purge_persists_across_storage_reload() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("finalize_purge_persist_reload");
        reset_for_tests();

        let run_dkg_request = RunDkgRequest {
            session_id: "session-finalize-purge-persist-reload".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        };

        let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
        let start_request = StartSignRoundRequest {
            session_id: "session-finalize-purge-persist-reload".to_string(),
            member_identifier: 1,
            message_hex: "deadbeef".to_string(),
            key_group: dkg_result.key_group,
            signing_participants: None,
            attempt_context: None,
            attempt_transition_evidence: None,
        };
        let round_state = start_sign_round(start_request.clone()).expect("start sign round");

        let finalize_request = FinalizeSignRoundRequest {
            session_id: "session-finalize-purge-persist-reload".to_string(),
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

        let first_result = finalize_sign_round(finalize_request.clone(), true).expect("finalize");

        reload_state_from_storage_for_tests();
        {
            let guard = state().expect("engine state").lock().expect("engine lock");
            let session = guard
                .sessions
                .get("session-finalize-purge-persist-reload")
                .expect("session state");

            assert!(session.finalize_request_fingerprint.is_some());
            assert!(session.signature_result.is_some());
            assert!(session.dkg_key_packages.is_none());
            assert!(session.dkg_public_key_package.is_none());
            assert!(session.sign_request_fingerprint.is_none());
            assert!(session.sign_message_bytes.is_none());
            assert!(session.round_state.is_none());
        }

        let second_result =
            finalize_sign_round(finalize_request, true).expect("persisted finalize retry");
        assert_eq!(first_result, second_result);

        let err = start_sign_round(start_request).expect_err("start sign round should fail");
        assert!(matches!(err, EngineError::SessionFinalized { .. }));

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn corrupt_state_file_fails_closed_by_default() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("corrupt_state_fail_closed");
        reset_for_tests();

        std::fs::write(&state_path, b"{invalid-state").expect("write corrupt state file");

        let err = match load_engine_state_from_storage() {
            Ok(_) => panic!("expected corruption failure"),
            Err(err) => err,
        };
        assert!(matches!(err, EngineError::Internal(_)));

        let err_message = err.to_string();
        assert!(err_message.contains("refusing to continue with corrupted signer state file"));
        assert!(err_message.contains(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV));
        assert!(state_path.exists());

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn truncated_state_file_fails_closed_by_default() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("truncated_state_fail_closed");
        reset_for_tests();

        run_dkg(RunDkgRequest {
            session_id: "session-truncated-state-fail-closed".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("seed persisted state");

        let persisted_bytes = std::fs::read(&state_path).expect("read persisted state file");
        assert!(
            persisted_bytes.len() > 1,
            "persisted state should be larger than one byte"
        );
        std::fs::write(&state_path, &persisted_bytes[..persisted_bytes.len() - 1])
            .expect("write truncated state file");

        let err = match load_engine_state_from_storage() {
            Ok(_) => panic!("expected corruption failure"),
            Err(err) => err,
        };
        assert!(matches!(err, EngineError::Internal(_)));

        let err_message = err.to_string();
        assert!(err_message.contains("refusing to continue with corrupted signer state file"));
        assert!(err_message.contains(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV));
        assert!(state_path.exists());

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn corrupt_state_file_quarantines_and_resets_when_enabled() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("corrupt_state_quarantine_reset");
        reset_for_tests();

        std::env::set_var(
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
        );
        std::fs::write(&state_path, b"{invalid-state").expect("write corrupt state file");

        let loaded = load_engine_state_from_storage().expect("recover from corrupted state file");
        assert!(loaded.sessions.is_empty());
        assert_eq!(loaded.refresh_epoch_counter, 0);
        assert!(!state_path.exists());

        let backups =
            sorted_corrupted_state_backups(&state_path).expect("list corrupted state backups");
        assert_eq!(backups.len(), 1);
        let backup_contents = std::fs::read(&backups[0]).expect("read backup file contents");
        assert_eq!(backup_contents, b"{invalid-state");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn truncated_state_file_quarantines_and_resets_when_enabled() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("truncated_state_quarantine_reset");
        reset_for_tests();

        std::env::set_var(
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
        );

        run_dkg(RunDkgRequest {
            session_id: "session-truncated-state-quarantine-reset".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("seed persisted state");

        let persisted_bytes = std::fs::read(&state_path).expect("read persisted state file");
        assert!(
            persisted_bytes.len() > 1,
            "persisted state should be larger than one byte"
        );
        let truncated_bytes = persisted_bytes[..persisted_bytes.len() - 1].to_vec();
        std::fs::write(&state_path, &truncated_bytes).expect("write truncated state file");

        let loaded = load_engine_state_from_storage().expect("recover from truncated state file");
        assert!(loaded.sessions.is_empty());
        assert_eq!(loaded.refresh_epoch_counter, 0);
        assert!(!state_path.exists());

        let backups =
            sorted_corrupted_state_backups(&state_path).expect("list corrupted state backups");
        assert_eq!(backups.len(), 1);
        let backup_contents = std::fs::read(&backups[0]).expect("read backup file contents");
        assert_eq!(backup_contents, truncated_bytes);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn schema_mismatch_state_file_fails_closed_by_default() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("schema_mismatch_fail_closed");
        reset_for_tests();

        let unsupported_schema_version = if PERSISTED_STATE_SCHEMA_VERSION == u16::MAX {
            0
        } else {
            PERSISTED_STATE_SCHEMA_VERSION + 1
        };
        let persisted = PersistedEngineState {
            schema_version: unsupported_schema_version,
            sessions: HashMap::new(),
            refresh_epoch_counter: 0,
            operator_fault_scores: BTreeMap::new(),
            quarantined_operator_identifiers: vec![],
            canary_rollout: CanaryRolloutState::default(),
        };
        let persisted_bytes = serde_json::to_vec(&persisted).expect("encode mismatched schema");
        std::fs::write(&state_path, &persisted_bytes).expect("write mismatched schema state file");

        let err = match load_engine_state_from_storage() {
            Ok(_) => panic!("expected schema mismatch failure"),
            Err(err) => err,
        };
        assert!(matches!(err, EngineError::Internal(_)));

        let err_message = err.to_string();
        assert!(err_message.contains("failed to validate signer state file"));
        assert!(err_message.contains("unsupported signer state schema version"));
        assert!(err_message.contains(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV));
        assert!(state_path.exists());

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn schema_mismatch_state_file_quarantines_and_resets_when_enabled() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("schema_mismatch_quarantine_reset");
        reset_for_tests();

        std::env::set_var(
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
        );

        let unsupported_schema_version = if PERSISTED_STATE_SCHEMA_VERSION == u16::MAX {
            0
        } else {
            PERSISTED_STATE_SCHEMA_VERSION + 1
        };
        let persisted = PersistedEngineState {
            schema_version: unsupported_schema_version,
            sessions: HashMap::new(),
            refresh_epoch_counter: 0,
            operator_fault_scores: BTreeMap::new(),
            quarantined_operator_identifiers: vec![],
            canary_rollout: CanaryRolloutState::default(),
        };
        let persisted_bytes = serde_json::to_vec(&persisted).expect("encode mismatched schema");
        std::fs::write(&state_path, &persisted_bytes).expect("write mismatched schema state file");

        let loaded = load_engine_state_from_storage().expect("recover from schema mismatch state");
        assert!(loaded.sessions.is_empty());
        assert_eq!(loaded.refresh_epoch_counter, 0);
        assert!(!state_path.exists());

        let backups =
            sorted_corrupted_state_backups(&state_path).expect("list corrupted state backups");
        assert_eq!(backups.len(), 1);
        let backup_contents = std::fs::read(&backups[0]).expect("read backup file contents");
        assert_eq!(backup_contents, persisted_bytes);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn corrupt_state_backup_retention_evicts_old_backups() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("corrupt_state_retention");
        reset_for_tests();

        std::env::set_var(
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
            TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
        );
        std::env::set_var(TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT_ENV, "2");

        for seed in 0..4 {
            std::fs::write(&state_path, format!("{{invalid-state-{seed}"))
                .expect("write corrupt state");
            let loaded =
                load_engine_state_from_storage().expect("recover from corrupt state iteration");
            assert!(loaded.sessions.is_empty());
        }

        let backups =
            sorted_corrupted_state_backups(&state_path).expect("list corrupted state backups");
        assert_eq!(backups.len(), 2);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn persisted_state_is_encrypted_envelope() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("encrypted_envelope_persist");
        reset_for_tests();

        run_dkg(RunDkgRequest {
            session_id: "session-encrypted-envelope".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("seed persisted encrypted state");

        let persisted_bytes = std::fs::read(&state_path).expect("read persisted state file");
        let envelope: PersistedEncryptedEngineStateEnvelope =
            serde_json::from_slice(&persisted_bytes).expect("decode encrypted envelope");
        assert_eq!(
            envelope.schema_version,
            PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION
        );
        assert_eq!(
            envelope.encryption_algorithm,
            TBTC_SIGNER_STATE_ENCRYPTION_ALGORITHM_XCHACHA20POLY1305
        );
        assert_eq!(
            envelope.key_provider,
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT
        );
        assert!(envelope.key_id.starts_with("sha256:"));
        assert_eq!(
            envelope.authentication_tag.len(),
            TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES * 2
        );
        assert!(!envelope.ciphertext.is_empty());

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn legacy_plaintext_state_migrates_to_encrypted_envelope_on_load() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("legacy_plaintext_migration");
        reset_for_tests();

        let mut sessions = HashMap::new();
        sessions.insert(
            "legacy-session".to_string(),
            persisted_session_state_fixture(),
        );
        let plaintext_state = PersistedEngineState {
            schema_version: PERSISTED_STATE_SCHEMA_VERSION,
            sessions,
            refresh_epoch_counter: 7,
            operator_fault_scores: BTreeMap::new(),
            quarantined_operator_identifiers: vec![],
            canary_rollout: CanaryRolloutState::default(),
        };
        let plaintext_bytes = serde_json::to_vec(&plaintext_state).expect("encode plaintext state");
        std::fs::write(&state_path, &plaintext_bytes).expect("write plaintext state file");

        let loaded = load_engine_state_from_storage().expect("load and migrate legacy plaintext");
        assert_eq!(loaded.sessions.len(), 1);
        assert_eq!(loaded.refresh_epoch_counter, 7);

        let migrated_bytes = std::fs::read(&state_path).expect("read migrated state file");
        let envelope: PersistedEncryptedEngineStateEnvelope =
            serde_json::from_slice(&migrated_bytes).expect("decode migrated encrypted envelope");
        assert_eq!(
            envelope.schema_version,
            PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION
        );
        assert!(!envelope.ciphertext.is_empty());

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn encrypted_state_load_fails_closed_when_key_missing() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("encrypted_state_missing_key");
        reset_for_tests();

        run_dkg(RunDkgRequest {
            session_id: "session-encrypted-state-missing-key".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("seed encrypted state file");

        std::env::remove_var(TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV);
        let err = match load_engine_state_from_storage() {
            Ok(_) => panic!("expected encrypted state load failure"),
            Err(err) => err,
        };
        let err_message = err.to_string();
        assert!(err_message.contains("missing required state encryption key env"));
        assert!(err_message.contains(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV));
        assert!(state_path.exists());

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn encrypted_state_load_rejects_tampered_legacy_key_id_format() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("encrypted_state_legacy_key_id");
        reset_for_tests();

        let session_id = "session-encrypted-state-legacy-key-id";
        run_dkg(RunDkgRequest {
            session_id: session_id.to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("seed encrypted state file");

        let persisted_bytes = std::fs::read(&state_path).expect("read persisted state file");
        let mut envelope: PersistedEncryptedEngineStateEnvelope =
            serde_json::from_slice(&persisted_bytes).expect("decode encrypted envelope");
        envelope.key_id = TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX.to_string();
        let mutated_bytes = serde_json::to_vec(&envelope).expect("encode legacy key_id envelope");
        std::fs::write(&state_path, mutated_bytes).expect("write legacy key_id envelope");

        let err = match load_engine_state_from_storage() {
            Ok(_) => panic!("tampered legacy key_id envelope should fail closed"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "state key identifier mismatch");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn legacy_v2_encrypted_state_rewrites_with_current_key_id() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("encrypted_state_v2_legacy_key_id");
        reset_for_tests();

        let persisted_state = PersistedEngineState {
            schema_version: PERSISTED_STATE_SCHEMA_VERSION,
            sessions: HashMap::new(),
            refresh_epoch_counter: 11,
            operator_fault_scores: BTreeMap::new(),
            quarantined_operator_identifiers: vec![],
            canary_rollout: CanaryRolloutState::default(),
        };
        let mut plaintext =
            serde_json::to_vec(&persisted_state).expect("encode persisted state fixture");
        let key_material = state_encryption_key_material().expect("load test state key");
        let cipher = XChaCha20Poly1305::new_from_slice(&key_material.key[..])
            .expect("initialize test cipher");
        let nonce_bytes = [7u8; TBTC_SIGNER_STATE_ENVELOPE_NONCE_BYTES];
        let nonce = XNonce::from_slice(&nonce_bytes);
        let mut ciphertext_and_tag = cipher
            .encrypt(nonce, plaintext.as_ref())
            .expect("encrypt legacy v2 envelope fixture");
        plaintext.zeroize();
        let mut authentication_tag = ciphertext_and_tag
            .split_off(ciphertext_and_tag.len() - TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES);
        let envelope = PersistedEncryptedEngineStateEnvelope {
            schema_version: PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION_V2,
            encryption_algorithm: TBTC_SIGNER_STATE_ENCRYPTION_ALGORITHM_XCHACHA20POLY1305
                .to_string(),
            key_provider: TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT.to_string(),
            key_id: TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX.to_string(),
            nonce: hex::encode(nonce_bytes),
            ciphertext: hex::encode(&ciphertext_and_tag),
            authentication_tag: hex::encode(&authentication_tag),
        };
        ciphertext_and_tag.zeroize();
        authentication_tag.zeroize();
        std::fs::write(
            &state_path,
            serde_json::to_vec(&envelope).expect("encode legacy v2 envelope"),
        )
        .expect("write legacy v2 envelope");

        let loaded = load_engine_state_from_storage().expect("load legacy v2 envelope");
        assert_eq!(loaded.refresh_epoch_counter, 11);

        let rewritten_bytes = std::fs::read(&state_path).expect("read rewritten envelope");
        let rewritten: PersistedEncryptedEngineStateEnvelope =
            serde_json::from_slice(&rewritten_bytes).expect("decode rewritten envelope");
        assert_eq!(
            rewritten.schema_version,
            PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION
        );
        assert!(rewritten.key_id.starts_with("sha256:"));
        assert_ne!(rewritten.key_id, TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX);

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn env_key_provider_is_rejected_in_production_profile() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("production_rejects_env_provider");
        reset_for_tests();

        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
        configure_valid_provenance_attestation_for_tests();
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
        );

        let err = mutate_state_for_key_provider_test("session-production-rejects-env-provider")
            .expect_err("production profile should reject env provider");
        expect_internal_error_contains(err, "is not allowed in profile [production]");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn production_profile_rejects_implicit_temp_state_path() {
        let _guard = lock_test_state();
        reset_for_tests();
        clear_state_storage_policy_overrides();

        std::env::remove_var(TBTC_SIGNER_STATE_PATH_ENV);
        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
        configure_valid_provenance_attestation_for_tests();
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
            format!("printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
        );

        let err =
            mutate_state_for_key_provider_test("session-production-rejects-implicit-state-path")
                .expect_err("production profile should reject implicit state path");
        expect_internal_error_contains(
            err,
            "refusing to use the implicit temp-dir signer state path",
        );

        reset_for_tests();
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn unknown_state_key_provider_is_rejected() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("unknown_state_key_provider");
        reset_for_tests();

        std::env::set_var(TBTC_SIGNER_STATE_KEY_PROVIDER_ENV, "hsm");

        let err = mutate_state_for_key_provider_test("session-unknown-state-key-provider")
            .expect_err("unsupported state key provider should fail closed");
        expect_internal_error_contains(err, "unsupported state key provider");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn command_key_provider_rejects_non_zero_exit() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("production_command_provider_non_zero_exit");
        reset_for_tests();

        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
        configure_valid_provenance_attestation_for_tests();
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
        );
        std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV, "exit 17");

        let err =
            mutate_state_for_key_provider_test("session-production-command-provider-non-zero-exit")
                .expect_err("non-zero command exit should fail closed");
        expect_internal_error_contains(err, "exited with non-zero status");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn command_key_provider_rejects_bad_output() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("production_command_provider_bad_output");
        reset_for_tests();

        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
        configure_valid_provenance_attestation_for_tests();
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
            "printf 'zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz\\n'",
        );

        let err =
            mutate_state_for_key_provider_test("session-production-command-provider-bad-output")
                .expect_err("bad command output should fail closed");
        expect_internal_error_contains(err, "must be valid hex");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn command_key_provider_drains_large_stderr_without_deadlock() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("production_command_provider_large_stderr");
        reset_for_tests();

        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
        configure_valid_provenance_attestation_for_tests();
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
        );
        std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV, "2");
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
            format!(
                "dd if=/dev/zero bs=70000 count=1 1>&2 2>/dev/null; printf '{}\\n'",
                TEST_STATE_ENCRYPTION_KEY_HEX
            ),
        );

        mutate_state_for_key_provider_test("session-production-command-provider-large-stderr")
            .expect("large stderr from state key command should not deadlock");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn encrypted_state_load_rejects_mismatched_key_id() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("encrypted_state_mismatched_key_id");
        reset_for_tests();

        run_dkg(RunDkgRequest {
            session_id: "session-encrypted-state-mismatched-key-id".to_string(),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
        })
        .expect("seed encrypted state file");

        std::env::set_var(
            TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
            "2222222222222222222222222222222222222222222222222222222222222222",
        );
        let err = match load_engine_state_from_storage() {
            Ok(_) => panic!("expected key_id mismatch rejection"),
            Err(err) => err,
        };
        expect_internal_error_contains(err, "state key identifier mismatch");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn command_key_provider_times_out_fail_closed() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("production_command_provider_timeout");
        reset_for_tests();

        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
        configure_valid_provenance_attestation_for_tests();
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
        );
        std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV, "1");
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
            format!("sleep 2; printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
        );

        let err = mutate_state_for_key_provider_test("session-production-command-provider-timeout")
            .expect_err("state key command timeout should fail closed");
        expect_internal_error_contains(err, "timed out");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    #[cfg(unix)]
    fn command_key_provider_times_out_when_background_descendant_keeps_pipe_open() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("production_command_provider_background_pipe");
        reset_for_tests();

        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
        configure_valid_provenance_attestation_for_tests();
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
        );
        std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV, "1");
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
            format!("sleep 5 & printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
        );

        let started_at = Instant::now();
        let err = mutate_state_for_key_provider_test(
            "session-production-command-provider-background-pipe",
        )
        .expect_err("state key command pipe timeout should fail closed");
        assert!(
            started_at.elapsed() < Duration::from_secs(4),
            "state key command should not wait for background descendant pipe EOF"
        );
        expect_internal_error_contains(err, "timed out");

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }

    #[test]
    fn command_key_provider_survives_restart_with_stable_key() {
        let _guard = lock_test_state();
        let state_path = configure_test_state_path("production_command_provider");
        reset_for_tests();

        std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
        configure_valid_provenance_attestation_for_tests();
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
            format!("printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
        );

        mutate_state_for_key_provider_test("session-production-command-provider")
            .expect("seed encrypted state with command provider");

        simulate_process_restart_for_tests();
        reload_state_from_storage_for_tests();

        {
            let state = state().expect("engine state should initialize");
            let guard = state.lock().expect("engine lock");
            assert!(guard
                .sessions
                .contains_key("session-production-command-provider"));
        }

        reset_for_tests();
        cleanup_test_state_artifacts(&state_path);
        clear_state_storage_policy_overrides();
    }
}
