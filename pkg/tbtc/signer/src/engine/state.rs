// In-memory engine/session state, the state-file lock, and registry capacity guards.

use super::*;

pub(crate) type SecretString = Zeroizing<String>;

pub(crate) type SecretBytes = Zeroizing<Vec<u8>>;

pub(crate) struct ZeroizingChaCha20Rng {
    pub(crate) inner: ChaCha20Rng,
}

impl ZeroizingChaCha20Rng {
    pub(crate) fn from_seed(seed: [u8; 32]) -> Self {
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

// Phase 7.1 interactive session state. Lives ONLY in memory: the
// nonces must never persist (frozen spec, markers-only durability),
// and without them the rest of this struct is useless after a
// restart, so none of it is mirrored into PersistedSessionState.
// The durable artifact is SessionState.consumed_interactive_attempt_markers.
pub(crate) struct InteractiveSigningState {
    pub(crate) open_request_fingerprint: String,
    pub(crate) attempt_context: AttemptContext,
    pub(crate) canonical_included_participants: Vec<u16>,
    pub(crate) member_identifier: u16,
    pub(crate) threshold: u16,
    pub(crate) message_bytes: SecretBytes,
    pub(crate) taproot_merkle_root: Option<[u8; 32]>,
    /// Validated non-transaction authorization for this live attempt. It is
    /// deliberately transient alongside the nonce state: after a restart no
    /// Round2 share can be released, so the host must open and validate a fresh
    /// intent rather than relying on a durable generic-message allowlist.
    pub(crate) signing_intent: Option<InteractiveSigningIntent>,
    pub(crate) key_package: frost::keys::KeyPackage,
    pub(crate) opened_at_unix: u64,
    pub(crate) round1: Option<InteractiveRound1State>,
}

// Secret round-1 nonces and the public commitments they correspond
// to. The nonces are zeroized at every exit path (consumption, abort,
// expiry, replacement) by the interactive module; the Drop impl is
// the backstop for paths that drop the struct without going through
// one of those.
pub(crate) struct InteractiveRound1State {
    pub(crate) nonces: frost::round1::SigningNonces,
    pub(crate) commitments_hex: String,
}

impl Drop for InteractiveRound1State {
    fn drop(&mut self) {
        self.nonces.zeroize();
    }
}

#[derive(Default)]
pub(crate) struct SessionState {
    pub(crate) dkg_request_fingerprint: Option<String>,
    pub(crate) dkg_key_packages: Option<BTreeMap<u16, frost::keys::KeyPackage>>,
    pub(crate) dkg_public_key_package: Option<frost::keys::PublicKeyPackage>,
    pub(crate) dkg_result: Option<DkgResult>,
    pub(crate) sign_request_fingerprint: Option<String>,
    pub(crate) sign_message_bytes: Option<SecretBytes>,
    pub(crate) round_state: Option<RoundState>,
    pub(crate) active_attempt_context: Option<AttemptContext>,
    pub(crate) attempt_transition_records: Vec<TranscriptAuditRecord>,
    pub(crate) consumed_attempt_ids: HashSet<String>,
    pub(crate) consumed_sign_round_ids: HashSet<String>,
    pub(crate) finalize_request_fingerprint: Option<String>,
    pub(crate) signature_result: Option<SignatureResult>,
    pub(crate) consumed_finalize_round_ids: HashSet<String>,
    pub(crate) consumed_finalize_request_fingerprints: HashSet<String>,
    pub(crate) build_tx_request_fingerprint: Option<String>,
    pub(crate) tx_result: Option<TransactionResult>,
    pub(crate) refresh_request_fingerprint: Option<String>,
    pub(crate) refresh_result: Option<RefreshSharesResult>,
    pub(crate) refresh_history: Vec<RefreshHistoryRecord>,
    /// Legacy count written by the retired synthetic refresh implementation.
    /// Retained only so existing pre-release state remains decodable; lifecycle
    /// status deliberately treats it as non-authoritative.
    pub(crate) refresh_count: u64,
    pub(crate) emergency_rekey_event: Option<EmergencyRekeyEvent>,
    /// Transient per-wallet budget for accepted heartbeat Opens. Like the
    /// process-global BuildTaprootTx limiter, this operational throttle resets on
    /// restart and is never written into the encrypted session state.
    pub(crate) heartbeat_rate_limiter: PolicyRateLimiterState,
    // Multi-seat: a process-global engine may hold several LOCAL members (seats)
    // signing the same session concurrently, each on its own attempt timeline.
    // Keyed by member_identifier; each entry is independent (own attempt, nonces,
    // replace/round2/expiry). Was Option (one member per session).
    pub(crate) interactive_signing: BTreeMap<u16, InteractiveSigningState>,
    // The key_group this per-signing session signs for, set at InteractiveSessionOpen.
    // Interactive signing runs under a fresh RoastSessionID per message, so a wallet's
    // DKG material lives under a DIFFERENT (wallet/DKG) session; this binds the signing
    // session to its wallet key so Round2/Aggregate resolve the same material by
    // key_group. Persisted even though live nonce state is not: Aggregate may run
    // after restart using only public material, and the full-lifetime role binding
    // prevents this per-signing session from later becoming an unrelated DKG owner.
    pub(crate) bound_key_group: Option<String>,
    pub(crate) consumed_interactive_attempt_markers: HashSet<String>,
    // Phase 7.2b InteractiveAggregate completion markers: an attempt whose
    // aggregate signature has been produced is recorded here so a repeat
    // InteractiveAggregate is rejected (re-aggregation is not a recovery path;
    // a lost signature is recovered with a fresh attempt). Durable like the
    // consumed markers (markers-only durability) and bounded the same way; not
    // security-load-bearing (the aggregate is deterministic over public data),
    // but the frozen Phase 7 spec marks the session complete.
    pub(crate) aggregated_interactive_attempt_markers: HashSet<String>,
}

#[derive(Default)]
pub(crate) struct EngineState {
    pub(crate) sessions: HashMap<String, SessionState>,
    pub(crate) refresh_epoch_counter: u64,
    pub(crate) operator_fault_scores: BTreeMap<u16, u64>,
    pub(crate) quarantined_operator_identifiers: HashSet<u16>,
    pub(crate) canary_rollout: CanaryRolloutState,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub(crate) struct RefreshHistoryRecord {
    pub(crate) refresh_epoch: u64,
    pub(crate) refreshed_at_unix: u64,
    pub(crate) share_count: u16,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) key_group: Option<String>,
    /// Legacy request fingerprint retained for persisted-schema compatibility.
    /// No record produced by the retired one-shot implementation represents a
    /// cryptographically valid share refresh.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) request_fingerprint: Option<String>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub(crate) struct EmergencyRekeyEvent {
    pub(crate) reason: String,
    pub(crate) triggered_at_unix: u64,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub(crate) struct CanaryRolloutState {
    pub(crate) current_percent: u8,
    pub(crate) previous_percent: u8,
    pub(crate) config_version: u64,
    pub(crate) last_action_unix: u64,
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

pub(crate) const TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION: usize = 128;

pub(crate) const TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION: usize = 256;

pub(crate) static ENGINE_STATE: OnceLock<Mutex<EngineState>> = OnceLock::new();

pub(crate) static STATE_FILE_LOCK: OnceLock<Mutex<Option<StateFileLock>>> = OnceLock::new();

pub(crate) static STATE_PATH_OVERRIDE_WARNED: OnceLock<()> = OnceLock::new();

pub(crate) enum CorruptStatePolicy {
    FailClosed,
    QuarantineAndReset,
}

pub(crate) struct StateFileLock {
    pub(crate) _file: fs::File,
    pub(crate) state_path: PathBuf,
    pub(crate) lock_path: PathBuf,
}

impl StateFileLock {
    pub(crate) fn acquire(state_path: &Path) -> Result<Self, EngineError> {
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

pub(crate) fn state_file_lock_slot() -> &'static Mutex<Option<StateFileLock>> {
    STATE_FILE_LOCK.get_or_init(|| Mutex::new(None))
}

#[cfg(unix)]
pub(crate) fn is_lock_contention_errno(errno: i32) -> bool {
    errno == EAGAIN || errno == EWOULDBLOCK
}

pub(crate) fn state() -> Result<&'static Mutex<EngineState>, EngineError> {
    ensure_state_file_lock()?;
    warn_disabled_policy_gates();

    if let Some(state) = ENGINE_STATE.get() {
        return Ok(state);
    }

    let loaded_state = load_engine_state_from_storage()?;
    Ok(ENGINE_STATE.get_or_init(|| Mutex::new(loaded_state)))
}

pub(crate) fn state_file_path() -> Result<PathBuf, EngineError> {
    let configured_path = signer_env_var(TBTC_SIGNER_STATE_PATH_ENV)
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
            "{} (or the state_path field of the init-time signer config) must be \
             set when {}={}; refusing to use the implicit temp-dir signer state path",
            TBTC_SIGNER_STATE_PATH_ENV, TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION
        )));
    }

    Ok(std::env::temp_dir().join(TBTC_SIGNER_DEFAULT_STATE_FILENAME))
}

pub(crate) fn active_state_file_path() -> Result<PathBuf, EngineError> {
    let lock_slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;

    if let Some(lock) = lock_slot.as_ref() {
        return Ok(lock.state_path.clone());
    }

    state_file_path()
}

pub(crate) fn state_lock_file_path(state_path: &Path) -> PathBuf {
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

pub(crate) fn ensure_state_file_lock() -> Result<(), EngineError> {
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

pub(crate) fn state_corruption_policy() -> CorruptStatePolicy {
    let policy = signer_env_var(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV)
        .map(|value| value.trim().to_ascii_lowercase())
        .unwrap_or_default();

    if policy == TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET {
        CorruptStatePolicy::QuarantineAndReset
    } else {
        CorruptStatePolicy::FailClosed
    }
}

pub(crate) fn state_corrupt_backup_limit() -> usize {
    signer_env_var(TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT_ENV)
        .and_then(|value| value.trim().parse::<usize>().ok())
        .unwrap_or(TBTC_SIGNER_DEFAULT_CORRUPT_BACKUP_LIMIT)
}

pub(crate) fn max_sessions_limit() -> usize {
    signer_env_var(TBTC_SIGNER_MAX_SESSIONS_ENV)
        .and_then(|value| value.trim().parse::<usize>().ok())
        .filter(|limit| *limit > 0)
        .unwrap_or(TBTC_SIGNER_DEFAULT_MAX_SESSIONS)
}

pub(crate) fn ensure_consumed_registry_persisted_bound(
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

pub(crate) fn ensure_session_registry_persisted_bound(
    session_count: usize,
) -> Result<(), EngineError> {
    let max_sessions = max_sessions_limit();
    if session_count > max_sessions {
        return Err(EngineError::Internal(format!(
            "persisted session registry size [{session_count}] exceeds max [{max_sessions}]"
        )));
    }

    Ok(())
}

// A per-message interactive session is safe to reclaim once it has no live
// nonce state and either completed aggregation or never accumulated any
// durable/non-interactive state. Keep this predicate exhaustive: adding a new
// SessionState field must force an explicit decision about whether reclaiming a
// session carrying that field is safe.
pub(crate) fn reclaimable_per_message_session(
    session: &SessionState,
    include_completed: bool,
) -> bool {
    let SessionState {
        dkg_request_fingerprint,
        dkg_key_packages,
        dkg_public_key_package,
        dkg_result,
        sign_request_fingerprint,
        sign_message_bytes,
        round_state,
        active_attempt_context,
        attempt_transition_records,
        consumed_attempt_ids,
        consumed_sign_round_ids,
        finalize_request_fingerprint,
        signature_result,
        consumed_finalize_round_ids,
        consumed_finalize_request_fingerprints,
        build_tx_request_fingerprint,
        tx_result,
        refresh_request_fingerprint,
        refresh_result,
        refresh_history,
        refresh_count,
        emergency_rekey_event,
        heartbeat_rate_limiter,
        interactive_signing,
        bound_key_group,
        consumed_interactive_attempt_markers,
        aggregated_interactive_attempt_markers,
    } = session;

    // Open-created per-message sessions are bound to a wallet key but never own
    // DKG material. Wallet/DKG sessions are permanent and must not be compacted.
    let per_message_role = bound_key_group.is_some()
        && dkg_request_fingerprint.is_none()
        && dkg_key_packages.is_none()
        && dkg_public_key_package.is_none()
        && dkg_result.is_none();
    if !per_message_role || !interactive_signing.is_empty() {
        return false;
    }

    // These fields belong to other session workflows. Their presence makes the
    // role ambiguous, so retain the entry. BuildTaprootTx fields are handled
    // separately below because they are the policy artifact for this same
    // per-message signing flow.
    let carries_other_workflow_state = sign_request_fingerprint.is_some()
        || sign_message_bytes.is_some()
        || round_state.is_some()
        || active_attempt_context.is_some()
        || !attempt_transition_records.is_empty()
        || !consumed_attempt_ids.is_empty()
        || !consumed_sign_round_ids.is_empty()
        || finalize_request_fingerprint.is_some()
        || signature_result.is_some()
        || !consumed_finalize_round_ids.is_empty()
        || !consumed_finalize_request_fingerprints.is_empty()
        || refresh_request_fingerprint.is_some()
        || refresh_result.is_some()
        || !refresh_history.is_empty()
        || *refresh_count != 0
        || emergency_rekey_event.is_some()
        || heartbeat_rate_limiter.last_refill_unix != 0
        || heartbeat_rate_limiter.token_microunits != 0
        || heartbeat_rate_limiter.configured_rate_limit_per_minute != 0;
    if carries_other_workflow_state {
        return false;
    }

    // Successful aggregation is terminal for this per-message flow. Preserve
    // its completion tombstone until capacity pressure requires compaction, so
    // immediate/restart retries retain their existing typed error semantics.
    if include_completed && !aggregated_interactive_attempt_markers.is_empty() {
        return true;
    }

    // Abort/TTL shells with no consumed share and no transaction-policy artifact
    // are safe to recreate from Open. A BuildTaprootTx artifact must survive a
    // failed attempt because the outer retry loop reuses this stable session ID.
    aggregated_interactive_attempt_markers.is_empty()
        && consumed_interactive_attempt_markers.is_empty()
        && build_tx_request_fingerprint.is_none()
        && tx_result.is_none()
}

pub(crate) fn reclaim_per_message_sessions(
    sessions: &mut HashMap<String, SessionState>,
    include_completed: bool,
) -> usize {
    let before = sessions.len();
    sessions.retain(|_, session| !reclaimable_per_message_session(session, include_completed));
    before.saturating_sub(sessions.len())
}

pub(crate) fn ensure_session_insert_capacity(
    sessions: &mut HashMap<String, SessionState>,
    session_id: &str,
) -> Result<(), EngineError> {
    if sessions.contains_key(session_id) {
        return Ok(());
    }

    let max_sessions = max_sessions_limit();
    if sessions.len() >= max_sessions {
        // Completed per-message sessions are bounded tombstones, not wallet
        // ownership state. Compact them only when a new session needs a slot;
        // the caller's ensuing durable mutation persists the compacted map.
        reclaim_per_message_sessions(sessions, true);
    }
    if sessions.len() >= max_sessions {
        return Err(EngineError::Internal(format!(
            "session registry size [{}] reached max [{max_sessions}]; use an existing session_id or increase {}",
            sessions.len(),
            TBTC_SIGNER_MAX_SESSIONS_ENV
        )));
    }

    Ok(())
}

pub(crate) fn ensure_consumed_registry_insert_capacity(
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
