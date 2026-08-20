use super::key_provider::state_encryption_key_material;
use super::pending_ops::clear_snapshot_covered_operations;
/// Envelope I/O: file open, lock acquisition, atomic rename, directory sync,
/// corrupt-state recovery, backup retention, AEAD encode/decode of the persisted
/// state envelope. Moved from `persistence.rs` as part of the C2
/// persistence-deepening refactor.
///
/// Cross-module dependencies: calls `key_provider::state_encryption_key_material`
/// from both the load and persist paths (to decrypt the envelope on load and
/// encrypt on persist), and `pending_ops::clear_snapshot_covered_operations` from
/// `persist_engine_state_to_storage` on successful snapshot write only.
use super::*;

pub(crate) const TBTC_SIGNER_STATE_ENCRYPTION_ALGORITHM_XCHACHA20POLY1305: &str =
    "xchacha20poly1305";

pub(crate) const TBTC_SIGNER_STATE_ENVELOPE_NONCE_BYTES: usize = 24;

pub(crate) const TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES: usize = 16;

/// Hard upper bound on the decoded ciphertext length of a persisted state
/// envelope. The AEAD path spends linear time authenticating arbitrary input
/// the moment a tampered or accidentally-grown envelope is loaded off disk;
/// bound it before allocating the decoded buffer the AEAD will walk. Calibrated
/// to the FFI's `MAX_REQUEST_BYTES` ceiling plus the AEAD tag; a healthy state
/// file is orders of magnitude smaller than this cap.
pub(crate) const TBTC_SIGNER_STATE_ENVELOPE_MAX_CIPHERTEXT_BYTES: usize =
    16 * 1024 * 1024 + TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES;

#[cfg(test)]
pub(crate) static PERSIST_FAULT_INJECTION_POINT: OnceLock<
    Mutex<Option<PersistFaultInjectionPoint>>,
> = OnceLock::new();

#[cfg(test)]
static STATE_FILE_PARENT_DIRECTORY_SYNCS: std::sync::atomic::AtomicUsize =
    std::sync::atomic::AtomicUsize::new(0);

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

pub(crate) fn corrupted_state_backup_prefix(path: &Path) -> String {
    let state_filename = path
        .file_name()
        .map(|name| name.to_string_lossy().into_owned())
        .unwrap_or_else(|| TBTC_SIGNER_DEFAULT_STATE_FILENAME.to_string());
    format!("{state_filename}.corrupt-")
}

pub(crate) fn state_file_parent_directory(path: &Path) -> Option<&Path> {
    path.parent().map(|parent| {
        if parent.as_os_str().is_empty() {
            Path::new(".")
        } else {
            parent
        }
    })
}

pub(crate) fn sync_state_file_parent_directory(path: &Path) -> Result<(), EngineError> {
    let Some(parent) = state_file_parent_directory(path) else {
        return Ok(());
    };
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
    #[cfg(test)]
    STATE_FILE_PARENT_DIRECTORY_SYNCS.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
    Ok(())
}

pub(crate) fn sync_existing_state_file_parent_directory(path: &Path) -> Result<(), EngineError> {
    match fs::symlink_metadata(path) {
        Ok(_) => sync_state_file_parent_directory(path),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(EngineError::Internal(format!(
            "failed to inspect signer state file [{}] before directory sync: {error}",
            path.display()
        ))),
    }
}

#[cfg(test)]
pub(crate) fn state_file_parent_directory_syncs_for_tests() -> usize {
    STATE_FILE_PARENT_DIRECTORY_SYNCS.load(std::sync::atomic::Ordering::SeqCst)
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
    let Some(parent) = state_file_parent_directory(path) else {
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

    // Bound the ciphertext hex length BEFORE hex::decode. hex::decode on an
    // adversarially-grown input still allocates to the input size; closing the
    // amplification at the hex-string layer avoids decoding Gigabytes just to
    // reject them.
    if envelope.ciphertext.len() > TBTC_SIGNER_STATE_ENVELOPE_MAX_CIPHERTEXT_BYTES * 2 {
        return Err(EngineError::Internal(format!(
            "envelope ciphertext hex exceeds max length [{}] bytes",
            TBTC_SIGNER_STATE_ENVELOPE_MAX_CIPHERTEXT_BYTES * 2
        )));
    }
    let ciphertext_decode = hex::decode(&envelope.ciphertext);
    envelope.ciphertext.zeroize();
    let mut ciphertext = ciphertext_decode
        .map_err(|_| EngineError::Internal("invalid envelope ciphertext hex".to_string()))?;
    // Belt-and-braces bound on the DECODED ciphertext length: hex::decode
    // halves the byte count, so the worst-case allocation is MAX/2. The AEAD
    // would otherwise walk and MAC every byte, including an attacker-controlled
    // peak of MAX bytes.
    if ciphertext.len() > TBTC_SIGNER_STATE_ENVELOPE_MAX_CIPHERTEXT_BYTES {
        ciphertext.zeroize();
        nonce_bytes.zeroize();
        return Err(EngineError::Internal(format!(
            "envelope ciphertext exceeds max length [{}] bytes, got [{}]",
            TBTC_SIGNER_STATE_ENVELOPE_MAX_CIPHERTEXT_BYTES,
            ciphertext.len()
        )));
    }
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
    serde_json::from_slice(&plaintext).map_err(|_| {
        EngineError::Internal(
            "failed to deserialize persisted state after successful AEAD decryption".to_string(),
        )
    })
}

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

    // An intermediate schema-1 writer allowed independent active and retired
    // allowances. Conversion below can safely compact its retired excess, but
    // the old oversized envelope must also be replaced immediately; otherwise
    // an emergency rollback before the next ordinary write still fails the
    // previous reader's total-count validation.
    let session_registry_requires_rewrite = persisted.sessions.len() > max_sessions_limit();
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
    if (should_rewrite_state || session_registry_requires_rewrite) && !recovered_from_corruption {
        persist_engine_state_to_storage(&engine_state).map_err(|e| {
            EngineError::Internal(format!(
                "loaded signer state file [{}] but failed to rewrite its migrated state: {e}",
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
        if let Some(parent) = state_file_parent_directory(&path) {
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

        sync_state_file_parent_directory(&path)?;

        Ok(())
    })();

    if persist_result.is_err() {
        let _ = fs::remove_file(&temp_path);
    }

    bytes.zeroize();
    match persist_result {
        Ok(()) => {
            clear_snapshot_covered_operations(engine_state);
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
