// Encrypted state-file persistence: envelope codec, key providers, corruption recovery, persisted<->live conversions.

use super::*;

#[derive(Clone, Deserialize, Serialize)]
pub(crate) struct PersistedKeyPackage {
    pub(crate) identifier: u16,
    pub(crate) key_package_hex: SecretString,
}

// Hand-written Debug: `SecretString` is `Zeroizing<String>`, whose
// derived Debug prints the inner string verbatim. `key_package_hex`
// holds serialized signing-share material, so it MUST be redacted -
// otherwise any `{:?}` of this struct (log line, panic, the derived
// Debug of an enclosing struct) spills a key share.
impl std::fmt::Debug for PersistedKeyPackage {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("PersistedKeyPackage")
            .field("identifier", &self.identifier)
            .field("key_package_hex", &"<redacted>")
            .finish()
    }
}

#[derive(Clone, Deserialize, Serialize)]
pub(crate) struct PersistedSessionState {
    pub(crate) dkg_request_fingerprint: Option<String>,
    pub(crate) dkg_key_packages: Option<Vec<PersistedKeyPackage>>,
    pub(crate) dkg_public_key_package_hex: Option<String>,
    pub(crate) dkg_result: Option<DkgResult>,
    pub(crate) sign_request_fingerprint: Option<String>,
    pub(crate) sign_message_hex: Option<SecretString>,
    pub(crate) round_state: Option<RoundState>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) active_attempt_context: Option<AttemptContext>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) attempt_transition_records: Vec<TranscriptAuditRecord>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_attempt_ids: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_sign_round_ids: Vec<String>,
    pub(crate) finalize_request_fingerprint: Option<String>,
    pub(crate) signature_result: Option<SignatureResult>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_finalize_round_ids: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_finalize_request_fingerprints: Vec<String>,
    pub(crate) build_tx_request_fingerprint: Option<String>,
    pub(crate) tx_result: Option<TransactionResult>,
    pub(crate) refresh_request_fingerprint: Option<String>,
    pub(crate) refresh_result: Option<RefreshSharesResult>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) refresh_history: Vec<RefreshHistoryRecord>,
    #[serde(default)]
    pub(crate) refresh_count: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) emergency_rekey_event: Option<EmergencyRekeyEvent>,
    // Phase 7.1 interactive consumption markers - the ONLY durable
    // artifact of interactive sessions (markers-only durability: live
    // interactive state, including nonces, never persists).
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_interactive_attempt_markers: Vec<String>,
    // Phase 7.2b InteractiveAggregate completion markers (see SessionState).
    // serde(default) keeps state written before 7.2b loadable: an absent field
    // deserializes to an empty set.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) aggregated_interactive_attempt_markers: Vec<String>,
    // The wallet key_group a per-signing (cross-session) attempt is bound to. Durable
    // ALONGSIDE the interactive markers: for a distributed-DKG wallet the signing
    // session has no dkg_result, so this is the ONLY link back to the wallet DKG. It
    // must survive a restart between Round2 (shares consumed, markers written) and
    // InteractiveAggregate, or Aggregate/verify_share would resolve neither dkg_result
    // nor bound_key_group and return DkgNotReady, stranding the collected shares. Public
    // (a key group id), not secret. serde(default) keeps pre-existing state loadable.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) bound_key_group: Option<String>,
}

// Hand-written Debug: `sign_message_hex` is `SecretString`
// (`Zeroizing<String>`), whose derived Debug renders the inner value
// verbatim. It is redacted here (presence preserved, content hidden);
// `dkg_key_packages` redacts via PersistedKeyPackage's own Debug.
// NOTE: any future secret-bearing field MUST be redacted here too.
impl std::fmt::Debug for PersistedSessionState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("PersistedSessionState")
            .field("dkg_request_fingerprint", &self.dkg_request_fingerprint)
            .field("dkg_key_packages", &self.dkg_key_packages)
            .field(
                "dkg_public_key_package_hex",
                &self.dkg_public_key_package_hex,
            )
            .field("dkg_result", &self.dkg_result)
            .field("sign_request_fingerprint", &self.sign_request_fingerprint)
            .field(
                "sign_message_hex",
                &self.sign_message_hex.as_ref().map(|_| "<redacted>"),
            )
            .field("round_state", &self.round_state)
            .field("active_attempt_context", &self.active_attempt_context)
            .field(
                "attempt_transition_records",
                &self.attempt_transition_records,
            )
            .field("consumed_attempt_ids", &self.consumed_attempt_ids)
            .field("consumed_sign_round_ids", &self.consumed_sign_round_ids)
            .field(
                "finalize_request_fingerprint",
                &self.finalize_request_fingerprint,
            )
            .field("signature_result", &self.signature_result)
            .field(
                "consumed_finalize_round_ids",
                &self.consumed_finalize_round_ids,
            )
            .field(
                "consumed_finalize_request_fingerprints",
                &self.consumed_finalize_request_fingerprints,
            )
            .field(
                "build_tx_request_fingerprint",
                &self.build_tx_request_fingerprint,
            )
            .field("tx_result", &self.tx_result)
            .field(
                "refresh_request_fingerprint",
                &self.refresh_request_fingerprint,
            )
            .field("refresh_result", &self.refresh_result)
            .field("refresh_history", &self.refresh_history)
            .field("emergency_rekey_event", &self.emergency_rekey_event)
            .field(
                "consumed_interactive_attempt_markers",
                &self.consumed_interactive_attempt_markers,
            )
            .field(
                "aggregated_interactive_attempt_markers",
                &self.aggregated_interactive_attempt_markers,
            )
            .field("bound_key_group", &self.bound_key_group)
            .finish()
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub(crate) struct PersistedEngineState {
    pub(crate) schema_version: u16,
    pub(crate) sessions: HashMap<String, PersistedSessionState>,
    pub(crate) refresh_epoch_counter: u64,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) operator_fault_scores: BTreeMap<u16, u64>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) quarantined_operator_identifiers: Vec<u16>,
    #[serde(default)]
    pub(crate) canary_rollout: CanaryRolloutState,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub(crate) struct PersistedEncryptedEngineStateEnvelope {
    pub(crate) schema_version: u16,
    pub(crate) encryption_algorithm: String,
    pub(crate) key_provider: String,
    pub(crate) key_id: String,
    pub(crate) nonce: String,
    pub(crate) ciphertext: String,
    pub(crate) authentication_tag: String,
}

pub(crate) enum PersistedStateStorageFormat {
    EncryptedEnvelope {
        persisted: PersistedEngineState,
        should_rewrite: bool,
    },
    LegacyPlaintext(PersistedEngineState),
}

pub(crate) struct StateEncryptionKeyMaterial {
    pub(crate) key: Zeroizing<[u8; 32]>,
    pub(crate) key_provider: &'static str,
    pub(crate) key_id: String,
}

pub(crate) const PERSISTED_STATE_SCHEMA_VERSION: u16 = 1;

pub(crate) const PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION_V2: u16 = 2;

pub(crate) const PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION: u16 = 3;

pub(crate) const TBTC_SIGNER_STATE_ENCRYPTION_ALGORITHM_XCHACHA20POLY1305: &str =
    "xchacha20poly1305";

pub(crate) const TBTC_SIGNER_STATE_ENVELOPE_NONCE_BYTES: usize = 24;

pub(crate) const TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES: usize = 16;

#[cfg(test)]
pub(crate) const TEST_STATE_ENCRYPTION_KEY_HEX: &str =
    "1111111111111111111111111111111111111111111111111111111111111111";

#[cfg(test)]
pub(crate) static PERSIST_FAULT_INJECTION_POINT: OnceLock<
    Mutex<Option<PersistFaultInjectionPoint>>,
> = OnceLock::new();

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum PersistFaultInjectionPoint {
    AfterTempSyncBeforeRename,
    AfterRenameBeforeDirectorySync,
}

#[derive(Debug)]
pub(crate) struct PersistEngineStateError {
    error: EngineError,
    state_file_replaced: bool,
}

impl PersistEngineStateError {
    fn before_state_file_replacement(error: EngineError) -> Self {
        Self {
            error,
            state_file_replaced: false,
        }
    }

    fn after_state_file_replacement(error: EngineError) -> Self {
        Self {
            error,
            state_file_replaced: true,
        }
    }

    pub(crate) fn state_file_replaced(&self) -> bool {
        self.state_file_replaced
    }

    pub(crate) fn into_engine_error(self) -> EngineError {
        self.error
    }
}

impl std::fmt::Display for PersistEngineStateError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        self.error.fmt(f)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum PersistencePendingOperation {
    BuildTaprootTx {
        session_id: String,
        request_fingerprint: String,
    },
    EmergencyRekey {
        result: TriggerEmergencyRekeyResult,
    },
    CanaryPromotion {
        result: PromoteCanaryResult,
    },
    CanaryRollback {
        result: RollbackCanaryResult,
    },
    InteractiveRound2 {
        session_id: String,
        consumed_marker: String,
    },
    InteractiveAggregate {
        session_id: String,
        aggregated_marker: String,
    },
}

static PERSISTENCE_PENDING_OPERATIONS: OnceLock<Mutex<Vec<PersistencePendingOperation>>> =
    OnceLock::new();

fn persistence_pending_operations() -> &'static Mutex<Vec<PersistencePendingOperation>> {
    PERSISTENCE_PENDING_OPERATIONS.get_or_init(|| Mutex::new(Vec::new()))
}

fn persistence_pending_same_slot(
    existing: &PersistencePendingOperation,
    replacement: &PersistencePendingOperation,
) -> bool {
    match (existing, replacement) {
        (
            PersistencePendingOperation::BuildTaprootTx {
                session_id: existing,
                ..
            },
            PersistencePendingOperation::BuildTaprootTx {
                session_id: replacement,
                ..
            },
        ) => existing == replacement,
        (
            PersistencePendingOperation::EmergencyRekey { result: existing },
            PersistencePendingOperation::EmergencyRekey {
                result: replacement,
            },
        ) => existing.session_id == replacement.session_id,
        (
            PersistencePendingOperation::CanaryPromotion { .. }
            | PersistencePendingOperation::CanaryRollback { .. },
            PersistencePendingOperation::CanaryPromotion { .. }
            | PersistencePendingOperation::CanaryRollback { .. },
        ) => true,
        (
            PersistencePendingOperation::InteractiveRound2 {
                session_id: existing_session,
                consumed_marker: existing_marker,
            },
            PersistencePendingOperation::InteractiveRound2 {
                session_id: replacement_session,
                consumed_marker: replacement_marker,
            },
        ) => existing_session == replacement_session && existing_marker == replacement_marker,
        (
            PersistencePendingOperation::InteractiveAggregate {
                session_id: existing_session,
                aggregated_marker: existing_marker,
            },
            PersistencePendingOperation::InteractiveAggregate {
                session_id: replacement_session,
                aggregated_marker: replacement_marker,
            },
        ) => existing_session == replacement_session && existing_marker == replacement_marker,
        _ => false,
    }
}

pub(crate) fn mark_persistence_pending(operation: PersistencePendingOperation) {
    let mut pending = persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner);
    pending.retain(|existing| !persistence_pending_same_slot(existing, &operation));
    pending.push(operation);
}

#[cfg(any(test, feature = "bench-restart-hook"))]
pub(crate) fn clear_persistence_pending_operations() {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .clear();
}

pub(crate) fn clear_persistence_pending_operation(operation: &PersistencePendingOperation) {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .retain(|pending| pending != operation);
}

fn clear_snapshot_covered_marker_operations() {
    // Round2/Aggregate pending entries cache no result; once any complete state
    // snapshot succeeds, their retained markers are durable and the normal
    // replay gates are sufficient. Lifecycle/build/refresh entries additionally
    // preserve the original operation result, so keep those until that caller
    // retries (one bounded slot per session, plus one canary slot).
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .retain(|pending| {
            !matches!(
                pending,
                PersistencePendingOperation::InteractiveRound2 { .. }
                    | PersistencePendingOperation::InteractiveAggregate { .. }
            )
        });
}

pub(crate) fn pending_build_taproot_tx_operation(
    session_id: &str,
) -> Option<PersistencePendingOperation> {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .find(|operation| {
            matches!(
                operation,
                PersistencePendingOperation::BuildTaprootTx {
                    session_id: pending_session,
                    ..
                } if pending_session == session_id
            )
        })
        .cloned()
}

pub(crate) fn pending_emergency_rekey_operation(
    session_id: &str,
) -> Option<PersistencePendingOperation> {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .find(|operation| match operation {
            PersistencePendingOperation::EmergencyRekey { result } => {
                result.session_id == session_id
            }
            _ => false,
        })
        .cloned()
}

#[cfg(test)]
pub(crate) fn pending_emergency_rekey_result(
    session_id: &str,
    reason: &str,
) -> Option<TriggerEmergencyRekeyResult> {
    match pending_emergency_rekey_operation(session_id) {
        Some(PersistencePendingOperation::EmergencyRekey { result }) if result.reason == reason => {
            Some(result)
        }
        _ => None,
    }
}

pub(crate) fn pending_canary_operation() -> Option<PersistencePendingOperation> {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .find(|operation| {
            matches!(
                operation,
                PersistencePendingOperation::CanaryPromotion { .. }
                    | PersistencePendingOperation::CanaryRollback { .. }
            )
        })
        .cloned()
}

#[cfg(test)]
pub(crate) fn pending_canary_promotion_result(target_percent: u8) -> Option<PromoteCanaryResult> {
    match pending_canary_operation() {
        Some(PersistencePendingOperation::CanaryPromotion { result })
            if result.to_percent == target_percent =>
        {
            Some(result)
        }
        _ => None,
    }
}

#[cfg(test)]
pub(crate) fn pending_canary_rollback_result(reason: &str) -> Option<RollbackCanaryResult> {
    match pending_canary_operation() {
        Some(PersistencePendingOperation::CanaryRollback { result }) if result.reason == reason => {
            Some(result)
        }
        _ => None,
    }
}

pub(crate) fn interactive_round2_persistence_pending(
    session_id: &str,
    consumed_marker: &str,
) -> bool {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .any(|operation| {
            matches!(
                operation,
                PersistencePendingOperation::InteractiveRound2 {
                    session_id: pending_session,
                    consumed_marker: pending_marker,
                } if pending_session == session_id && pending_marker == consumed_marker
            )
        })
}

pub(crate) fn interactive_aggregate_persistence_pending(
    session_id: &str,
    aggregated_marker: &str,
) -> bool {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .any(|operation| {
            matches!(
                operation,
                PersistencePendingOperation::InteractiveAggregate {
                    session_id: pending_session,
                    aggregated_marker: pending_marker,
                } if pending_session == session_id && pending_marker == aggregated_marker
            )
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
    clear_persistence_pending_operations();
    ensure_state_file_lock()?;

    let loaded_state = load_engine_state_from_storage()?;
    let state = state()?;
    let mut guard = state
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    *guard = loaded_state;
    Ok(())
}

pub(crate) fn corrupted_state_backup_prefix(path: &Path) -> String {
    let state_filename = path
        .file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_else(|| TBTC_SIGNER_DEFAULT_STATE_FILENAME.to_string());
    format!("{state_filename}.corrupt-")
}

pub(crate) fn corrupted_state_backup_path(path: &Path) -> PathBuf {
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

pub(crate) fn sorted_corrupted_state_backups(path: &Path) -> Result<Vec<PathBuf>, EngineError> {
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

pub(crate) fn enforce_corrupted_state_backup_retention(path: &Path) -> Result<(), EngineError> {
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

pub(crate) fn recover_or_fail_from_corrupted_state_file(
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

pub(crate) fn state_key_command_timeout_secs() -> u64 {
    signer_env_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV)
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| {
            *value >= TBTC_SIGNER_MIN_STATE_KEY_COMMAND_TIMEOUT_SECS
                && *value <= TBTC_SIGNER_MAX_STATE_KEY_COMMAND_TIMEOUT_SECS
        })
        .unwrap_or(TBTC_SIGNER_DEFAULT_STATE_KEY_COMMAND_TIMEOUT_SECS)
}

pub(crate) fn decode_state_encryption_key_hex(
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

pub(crate) fn state_key_identifier(key: &[u8; 32]) -> String {
    format!("sha256:{}", hex::encode(hash_bytes(key)))
}

pub(crate) fn push_aad_field(aad: &mut Vec<u8>, label: &[u8], value: &[u8]) {
    aad.extend_from_slice(&(label.len() as u32).to_be_bytes());
    aad.extend_from_slice(label);
    aad.extend_from_slice(&(value.len() as u32).to_be_bytes());
    aad.extend_from_slice(value);
}

pub(crate) fn encrypted_state_envelope_aad(
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

pub(crate) fn drain_command_pipe<R>(mut pipe: R) -> mpsc::Receiver<std::io::Result<Vec<u8>>>
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

pub(crate) fn read_command_pipe(
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

pub(crate) fn zeroize_command_pipe_if_ready(receiver: mpsc::Receiver<std::io::Result<Vec<u8>>>) {
    if let Ok(Ok(mut bytes)) = receiver.try_recv() {
        bytes.zeroize();
    }
}

#[cfg(unix)]
pub(crate) fn configure_state_key_command_process_group(command: &mut std::process::Command) {
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
pub(crate) fn configure_state_key_command_process_group(_command: &mut std::process::Command) {}

#[cfg(unix)]
pub(crate) fn kill_state_key_command_process_group(child_id: u32) {
    let pgid = -(child_id as i32);
    unsafe {
        let _ = libc::kill(pgid, libc::SIGKILL);
    }
}

#[cfg(not(unix))]
pub(crate) fn kill_state_key_command_process_group(_child_id: u32) {}

pub(crate) fn terminate_state_key_command(child: &mut std::process::Child, child_id: u32) {
    kill_state_key_command_process_group(child_id);
    let _ = child.kill();
    let _ = child.wait();
}

pub(crate) fn remaining_timeout(deadline: Instant) -> Duration {
    deadline
        .checked_duration_since(Instant::now())
        .unwrap_or(Duration::ZERO)
}

pub(crate) fn execute_state_key_command(command_spec: &str) -> Result<Output, EngineError> {
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

/// Where the state-encryption key will come from, validated structurally:
/// provider selection, the production prohibition on the env provider, and
/// command-spec presence. Shared by `state_encryption_key_material` (which
/// goes on to read the secret or execute the command) and init-time config
/// validation (which deliberately must NOT touch the secret channel or run
/// commands).
pub(crate) enum StateKeyProviderPlan {
    Env,
    Command { command_spec: String },
}

pub(crate) fn resolve_state_key_provider_plan() -> Result<StateKeyProviderPlan, EngineError> {
    let provider = signer_env_var(TBTC_SIGNER_STATE_KEY_PROVIDER_ENV)
        .map(|value| value.trim().to_ascii_lowercase())
        .unwrap_or_else(|| TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT.to_string());

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
            Ok(StateKeyProviderPlan::Env)
        }
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND => {
            let command_spec =
                signer_env_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV).ok_or_else(|| {
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
            Ok(StateKeyProviderPlan::Command { command_spec })
        }
        _ => Err(EngineError::Internal(format!(
            "unsupported state key provider [{}]; expected [{}] or [{}]",
            provider,
            TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
            TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND
        ))),
    }
}

pub(crate) fn state_encryption_key_material() -> Result<StateEncryptionKeyMaterial, EngineError> {
    match resolve_state_key_provider_plan()? {
        StateKeyProviderPlan::Env => {
            let raw_key_hex =
                // Deliberately read from the real environment even when an init-time
                // config is installed: the state-encryption key is a secret and the
                // config FFI carries operational knobs only (see signer_env_var).
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
        StateKeyProviderPlan::Command { command_spec } => {
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
    }
}

pub(crate) fn encode_encrypted_state_envelope(
    persisted: &PersistedEngineState,
    key_material: &StateEncryptionKeyMaterial,
) -> Result<Zeroizing<Vec<u8>>, EngineError> {
    let mut plaintext = Zeroizing::new(
        serde_json::to_vec(persisted)
            .map_err(|e| EngineError::Internal(format!("failed to encode signer state: {e}")))?,
    );
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
    let key_id = key_material.key_id.clone();
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

pub(crate) fn decode_encrypted_state_envelope(
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

/// Whether the legacy unencrypted plaintext state path may be accepted.
///
/// Plaintext state is UNAUTHENTICATED, so accepting it would let anyone who can
/// write the state file forge it (cleared replay markers, attacker key material)
/// without holding the state-encryption key. Per the secret-material hardening
/// plan this is an emergency-rollback-only path. The load-bearing guard is the
/// runtime non-production check (a production profile NEVER accepts plaintext,
/// regardless of build); it is additionally gated off in optimized builds via
/// `debug_assertions` and behind an explicit opt-in env flag. (Note: a release
/// build compiled with `debug-assertions = on` would still require both the
/// non-production profile and the opt-in flag.)
fn legacy_plaintext_state_permitted() -> bool {
    cfg!(debug_assertions)
        && !signer_profile_is_production()
        && signer_env_var(TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV)
            .map(|raw_value| truthy_env_flag(&raw_value))
            .unwrap_or(false)
}

pub(crate) fn decode_persisted_state_storage_format(
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

    // The bytes are not an encrypted envelope. Only fall back to the legacy
    // UNAUTHENTICATED plaintext format on the gated emergency-rollback path;
    // otherwise refuse, so an attacker who can write the state file cannot
    // bypass the AEAD envelope (forged replay markers / key material) without
    // the state-encryption key.
    if !legacy_plaintext_state_permitted() {
        return Err(EngineError::Internal(
            "refusing to load unauthenticated plaintext signer state; an \
             encrypted state envelope is required (legacy plaintext is an \
             emergency-rollback-only path, disabled in production and release \
             builds)"
                .to_string(),
        ));
    }

    let persisted = serde_json::from_slice::<PersistedEngineState>(bytes).map_err(|e| {
        EngineError::Internal(format!("failed to decode signer state file payload: {e}"))
    })?;
    Ok(PersistedStateStorageFormat::LegacyPlaintext(persisted))
}

pub(crate) fn load_engine_state_from_storage() -> Result<EngineState, EngineError> {
    let path = active_state_file_path()?;
    match fs::symlink_metadata(&path) {
        Ok(_) => {}
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            return Ok(EngineState::default())
        }
        Err(error) => {
            return Err(EngineError::Internal(format!(
                "failed to inspect signer state file [{}]: {error}",
                path.display()
            )))
        }
    }

    let mut bytes = fs::read(&path).map_err(|e| {
        EngineError::Internal(format!(
            "failed to read signer state file [{}]: {e}",
            path.display()
        ))
    })?;
    if bytes.is_empty() {
        bytes.zeroize();
        return recover_or_fail_from_corrupted_state_file(
            &path,
            format!("signer state file [{}] exists but is empty", path.display()),
        );
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

    let (engine_state, recovered_from_corruption): (EngineState, bool) = match persisted.try_into()
    {
        Ok(engine_state) => (engine_state, false),
        Err(error) => (
            recover_or_fail_from_corrupted_state_file(
                &path,
                format!(
                    "failed to validate signer state file [{}]: {error}",
                    path.display()
                ),
            )?,
            true,
        ),
    };

    // Quarantine-and-reset intentionally renames the corrupt file away. Do not
    // recreate it as a migrated clean state during the same load; the next real
    // mutation will create a fresh encrypted state file. This explicit recovery
    // outcome replaces the former `Path::exists` probe without hiding metadata
    // errors or treating dangling symlinks as first initialization.
    if should_rewrite_state && !recovered_from_corruption {
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
pub(crate) fn persist_fault_injection_label(point: PersistFaultInjectionPoint) -> &'static str {
    match point {
        PersistFaultInjectionPoint::AfterTempSyncBeforeRename => "after_temp_sync_before_rename",
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync => {
            "after_rename_before_directory_sync"
        }
    }
}

pub(crate) fn maybe_inject_persist_fault(
    point: PersistFaultInjectionPoint,
) -> Result<(), EngineError> {
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
pub(crate) fn set_persist_fault_injection_for_tests(point: PersistFaultInjectionPoint) {
    if let Ok(mut slot) = PERSIST_FAULT_INJECTION_POINT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        *slot = Some(point);
    }
}

#[cfg(test)]
pub(crate) fn clear_persist_fault_injection_for_tests() {
    if let Ok(mut slot) = PERSIST_FAULT_INJECTION_POINT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        *slot = None;
    }
}

// Hot paths resolve the state-encryption key (a KMS/HSM subprocess for the
// `command` provider) UNDER the held ENGINE_STATE guard, at the persist site --
// after any idempotent-replay/pre-persist rejection has already returned (so a
// transient key-provider outage never fails a non-persisting call) and, where a
// durable marker is written, before that marker (so an outage fails the attempt
// cleanly with no marker). Resolving under the guard keeps key selection in the
// same serialized order as the writes the guard already serializes, so the last
// writer encrypts with the then-current key. Resolving the key BEFORE the lock
// instead would let a caller that resolved an older key win the final write after
// a rotation, tagging the persisted envelope with a stale key id that decode
// rejects on restart.
pub(crate) fn persist_engine_state_to_storage(
    engine_state: &EngineState,
) -> Result<(), PersistEngineStateError> {
    // Resolves the state-encryption key (which, for the `command` provider,
    // spawns the KMS/HSM subprocess) and then persists. Hot paths call this at
    // the persist site WITH the ENGINE_STATE guard held, so key resolution is
    // ordered with the write (see the note above). The startup legacy-envelope
    // rewrite in load_engine_state_from_storage also calls it, off-guard, before
    // the engine serves concurrent operations -- there is no concurrent write to
    // lose a rotation race against there. Sites that write a durable marker before
    // persisting instead resolve the key explicitly before the marker and call
    // persist_engine_state_to_storage_with_key.
    let key_material = state_encryption_key_material()
        .map_err(PersistEngineStateError::before_state_file_replacement)?;
    persist_engine_state_to_storage_with_key(engine_state, &key_material)
}

pub(crate) fn persist_engine_state_to_storage_with_key(
    engine_state: &EngineState,
    key_material: &StateEncryptionKeyMaterial,
) -> Result<(), PersistEngineStateError> {
    let path =
        active_state_file_path().map_err(PersistEngineStateError::before_state_file_replacement)?;
    let persisted: PersistedEngineState = engine_state
        .try_into()
        .map_err(PersistEngineStateError::before_state_file_replacement)?;
    let mut bytes = encode_encrypted_state_envelope(&persisted, key_material)
        .map_err(PersistEngineStateError::before_state_file_replacement)?;
    drop(persisted);
    let temp_path = path.with_extension(format!("tmp-{}", std::process::id()));
    let mut state_file_replaced = false;
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
        state_file_replaced = true;
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
    match persist_result {
        Ok(()) => {
            clear_snapshot_covered_marker_operations();
            Ok(())
        }
        Err(error) if state_file_replaced => {
            Err(PersistEngineStateError::after_state_file_replacement(error))
        }
        Err(error) => Err(PersistEngineStateError::before_state_file_replacement(
            error,
        )),
    }
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
        let mut key_group_owners = HashMap::<String, String>::new();
        for (session_id, session_state) in persisted.sessions {
            let session_state: SessionState = session_state.try_into()?;
            if let Some(dkg_result) = session_state.dkg_result.as_ref() {
                if let Some(existing_owner) =
                    key_group_owners.insert(dkg_result.key_group.clone(), session_id.clone())
                {
                    return Err(EngineError::Internal(format!(
                        "duplicate persisted DKG key_group [{}] owned by sessions [{}] and [{}]",
                        dkg_result.key_group, existing_owner, session_id
                    )));
                }
            }
            sessions.insert(session_id, session_state);
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

        let mut consumed_interactive_attempt_markers = HashSet::new();
        for attempt_marker in persisted.consumed_interactive_attempt_markers {
            if attempt_marker.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed interactive attempt marker must be non-empty".to_string(),
                ));
            }

            if !consumed_interactive_attempt_markers.insert(attempt_marker.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed interactive attempt marker [{}]",
                    attempt_marker
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_interactive_attempt_markers.len(),
            "consumed_interactive_attempt_markers",
        )?;

        let mut aggregated_interactive_attempt_markers = HashSet::new();
        for attempt_marker in persisted.aggregated_interactive_attempt_markers {
            if attempt_marker.is_empty() {
                return Err(EngineError::Internal(
                    "persisted aggregated interactive attempt marker must be non-empty".to_string(),
                ));
            }

            if !aggregated_interactive_attempt_markers.insert(attempt_marker.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted aggregated interactive attempt marker [{}]",
                    attempt_marker
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            aggregated_interactive_attempt_markers.len(),
            "aggregated_interactive_attempt_markers",
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
            // Preserve the legacy synthetic count losslessly for schema
            // compatibility and diagnostics. Lifecycle status deliberately
            // ignores it until a versioned cryptographic refresh protocol exists.
            // Evaluated before refresh_history is moved below.
            refresh_count: persisted
                .refresh_count
                .max(persisted.refresh_history.len() as u64),
            refresh_history: persisted.refresh_history,
            emergency_rekey_event: persisted.emergency_rekey_event,
            heartbeat_rate_limiter: PolicyRateLimiterState::default(),
            // Live interactive state never restores: nonces are gone by
            // construction after a restart, so the attempt fails safe and
            // only the consumption markers survive. Empty map (no live members).
            interactive_signing: BTreeMap::new(),
            // Restore the wallet binding: for a cross-session signing session it is the
            // only durable link to the wallet DKG, needed so an InteractiveAggregate that
            // runs after a restart (past a member's Round2) can still resolve the wallet
            // by key_group. Public data; survives with the consumed/aggregate markers.
            bound_key_group: persisted.bound_key_group,
            consumed_interactive_attempt_markers,
            aggregated_interactive_attempt_markers,
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
        ensure_consumed_registry_persisted_bound(
            session_state.consumed_interactive_attempt_markers.len(),
            "consumed_interactive_attempt_markers",
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
        let mut consumed_interactive_attempt_markers = session_state
            .consumed_interactive_attempt_markers
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_interactive_attempt_markers.sort_unstable();
        let mut aggregated_interactive_attempt_markers = session_state
            .aggregated_interactive_attempt_markers
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        aggregated_interactive_attempt_markers.sort_unstable();

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
            refresh_count: session_state.refresh_count,
            emergency_rekey_event: session_state.emergency_rekey_event.clone(),
            consumed_interactive_attempt_markers,
            aggregated_interactive_attempt_markers,
            bound_key_group: session_state.bound_key_group.clone(),
        })
    }
}
