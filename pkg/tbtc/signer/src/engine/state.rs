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
    /// Monotonic time of the last successful activity for this member's live
    /// attempt. Exact Open and Round1 retries refresh it, as does a validated
    /// Round2 whose retry-preserving durability work fails. Rejected traffic
    /// does not extend nonce residency. This state is transient, so `Instant`
    /// never crosses the persistence boundary.
    pub(crate) last_activity_at: Instant,
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
    /// Epoch of the retained cryptographic key packages. The current ABI-4
    /// signer deliberately rejects synthetic share refresh, so zero is the only
    /// supported value until a real atomic replacement protocol is introduced.
    pub(crate) dkg_share_epoch: u64,
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
    // Idle per-message entries use the unoccupied portion of the shared persisted
    // session budget. The full entry is retained temporarily so delayed
    // Aggregate/verify-share calls and an outer retry's BuildTaprootTx policy
    // artifact keep working. Old retired entries are evicted FIFO-by-time when a
    // new active session needs their slot.
    pub(crate) retired_interactive_at_unix: Option<u64>,
    // Transient refcount pin for Aggregate's unlocked cryptographic section.
    // The session owns one reference; an in-flight Aggregate clones it while
    // holding the engine lock, and compaction skips any session with a clone.
    // Never persisted: no operation can remain in flight across a restart.
    pub(crate) aggregate_eviction_pin: Arc<()>,
    pub(crate) consumed_interactive_attempt_markers: HashSet<String>,
    // Fixed-size SHA-256 bindings. Round2 writes an exact
    // (attempt_id, signing package, taproot root) authorization; successful
    // Aggregate replaces it with a package/root completion identity so the
    // same attempt-less FROST package cannot fill completion storage under
    // fresh canonical attempt ids. Both survive restart.
    pub(crate) authorized_interactive_aggregate_markers: HashSet<String>,
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

// Loading can rewrite a legacy or stale encrypted envelope in place. A
// OnceLock serializes only the final in-memory installation, so it does not by
// itself prevent concurrent first callers from racing those fallible storage
// reads and migrations. Keep that entire path behind a process-local mutex
// that is deliberately separate from STATE_FILE_LOCK: the loader resolves the
// active path through STATE_FILE_LOCK and would deadlock if its slot guard were
// held here.
static ENGINE_STATE_INITIALIZATION_LOCK: Mutex<()> = Mutex::new(());

pub(crate) static STATE_FILE_LOCK: OnceLock<Mutex<Option<StateFileLock>>> = OnceLock::new();

pub(crate) static STATE_PATH_OVERRIDE_WARNED: OnceLock<()> = OnceLock::new();

pub(crate) enum CorruptStatePolicy {
    FailClosed,
    QuarantineAndReset,
}

pub(crate) fn state_file_lock_slot() -> &'static Mutex<Option<StateFileLock>> {
    STATE_FILE_LOCK.get_or_init(|| Mutex::new(None))
}

/// Executes the offline-certified trust transition before any ordinary store
/// or engine access can win the startup race. Lock order intentionally matches
/// `state()`: engine-initialization gate first, then the store slot.
pub(crate) fn with_startup_state_anchor_trust_transition<T>(
    transition: &VerifiedStateAnchorTrustTransition,
    operation: impl FnOnce(&mut StateFileLock) -> Result<T, EngineError>,
) -> Result<T, EngineError> {
    require_installed_signer_config()?;
    let _initialization_guard = ENGINE_STATE_INITIALIZATION_LOCK
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner);
    if ENGINE_STATE.get().is_some() {
        return Err(EngineError::Validation(
            "state-anchor trust transition is startup-only and the signer engine is initialized"
                .to_string(),
        ));
    }
    let slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;
    if slot.is_some() {
        return Err(EngineError::Validation(
            "state-anchor trust transition is startup-only and the durable store was opened"
                .to_string(),
        ));
    }
    let state_path = state_file_path()?;
    let mut store = StateFileLock::acquire_for_trust_transition(&state_path, transition)?;
    operation(&mut store)
}

/// Reads the durable state-anchor trust head without making a read-only
/// preflight claim initialize the process-wide signer store. Before any normal
/// engine/store access, a dedicated inspection acquisition holds the same
/// descriptor-bound OS lock and performs the trust-specific validation, then
/// drops it when the operation completes. If the store is already open, use
/// that held descriptor set so replacement checks and in-process path
/// consistency remain identical to every other stateful call.
pub(crate) fn with_startup_state_anchor_trust_head_inspection<T>(
    operation: impl FnOnce(&mut StateFileLock) -> Result<T, EngineError>,
) -> Result<T, EngineError> {
    require_installed_signer_config()?;
    let _initialization_guard = ENGINE_STATE_INITIALIZATION_LOCK
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner);
    let state_path = state_file_path()?;
    let mut slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;

    if let Some(store) = slot.as_mut() {
        if store.state_path != state_path {
            return Err(EngineError::Internal(format!(
                "state file lock already initialized for [{}] with lock [{}]; refusing to switch to [{}] in-process",
                store.state_path.display(),
                store.lock_path.display(),
                state_path.display()
            )));
        }
        store.revalidate_store_entries()?;
        return operation(store);
    }

    let mut store = StateFileLock::acquire_for_trust_head_inspection(&state_path)?;
    operation(&mut store)
}

/// Provisioning-only, ephemeral acquisition used to export the stable store
/// fingerprint and exact pristine genesis checkpoint for offline bootstrap
/// certification. It must never populate the process-wide engine/store slots.
pub(crate) fn with_startup_state_anchor_bootstrap_facts<T>(
    operation: impl FnOnce(&mut StateFileLock) -> Result<T, EngineError>,
) -> Result<T, EngineError> {
    require_state_anchor_bootstrap_provisioning_config()?;
    let _initialization_guard = ENGINE_STATE_INITIALIZATION_LOCK
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner);
    if ENGINE_STATE.get().is_some() {
        return Err(EngineError::Validation(
            "state-anchor bootstrap provisioning is unavailable after engine initialization"
                .to_string(),
        ));
    }
    let slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;
    if slot.is_some() {
        return Err(EngineError::Validation(
            "state-anchor bootstrap provisioning requires an unopened process-wide store"
                .to_string(),
        ));
    }
    let state_path = state_file_path()?;
    let mut store = StateFileLock::acquire_for_bootstrap_facts(&state_path)?;
    operation(&mut store)
}

pub(crate) fn state() -> Result<&'static Mutex<EngineState>, EngineError> {
    warn_disabled_policy_gates();

    let engine = initialize_engine_state_with_loader(
        &ENGINE_STATE,
        &ENGINE_STATE_INITIALIZATION_LOCK,
        || {},
        load_engine_state_from_storage,
    )?;
    // The loader uses the startup-only validation path so it can apply the
    // explicit corruption policy. Once an EngineState exists, every caller
    // must satisfy the full state-image/witness check.
    ensure_state_file_lock()?;
    Ok(engine)
}

/// Installs the first engine state while serializing the complete fallible load
/// path, not just the final OnceLock write.
///
/// `after_initial_miss` is a no-op in production and lets concurrency tests
/// deterministically place multiple callers past the optimistic fast path.
/// The loader runs under `initialization_lock`; a failed load leaves
/// `engine_state` unset so a later call can retry.
pub(crate) fn initialize_engine_state_with_loader<'state, AfterInitialMiss, Load>(
    engine_state: &'state OnceLock<Mutex<EngineState>>,
    initialization_lock: &Mutex<()>,
    after_initial_miss: AfterInitialMiss,
    load: Load,
) -> Result<&'state Mutex<EngineState>, EngineError>
where
    AfterInitialMiss: FnOnce(),
    Load: FnOnce() -> Result<EngineState, EngineError>,
{
    if let Some(state) = engine_state.get() {
        return Ok(state);
    }

    after_initial_miss();

    // The mutex protects no data of its own. Recovering its guard after a
    // panic is safe, and the second OnceLock check determines whether the
    // previous caller completed installation before panicking.
    let _initialization_guard = initialization_lock
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner);

    if let Some(state) = engine_state.get() {
        return Ok(state);
    }

    let loaded_state = load()?;
    Ok(engine_state.get_or_init(|| Mutex::new(loaded_state)))
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
    require_normal_signer_purpose()?;
    let state_path = state_file_path()?;
    let mut lock_slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;

    if let Some(existing_lock) = lock_slot.as_mut() {
        if existing_lock.state_path == state_path {
            // `state()` is the front door for every stateful signer operation.
            // Revalidate the held no-follow store on every call so a lock,
            // store-ID, directory, witness, or state replacement after startup
            // cannot be hidden behind the initialized in-memory state.
            existing_lock.identity()?;
            return Ok(());
        }

        return Err(EngineError::Internal(format!(
            "state file lock already initialized for [{}] with lock [{}]; refusing to switch to [{}] in-process",
            existing_lock.state_path.display(),
            existing_lock.lock_path.display(),
            state_path.display()
        )));
    }

    // Acquisition validates descriptor structure and the journal. Validate the
    // state image against the committed tip before exposing the store through
    // the ordinary stateful-operation front door.
    let mut acquired = StateFileLock::acquire(&state_path)?;
    acquired.identity()?;
    *lock_slot = Some(acquired);
    Ok(())
}

/// Startup-only variant which validates the descriptor-bound store and witness
/// journal but defers the state-image digest comparison until after decoding.
/// This lets the explicit corruption policy quarantine malformed state without
/// allowing a valid rollback image into `EngineState`.
pub(crate) fn ensure_state_file_lock_for_load() -> Result<(), EngineError> {
    require_normal_signer_purpose()?;
    let state_path = state_file_path()?;
    let mut lock_slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;

    if let Some(existing_lock) = lock_slot.as_mut() {
        if existing_lock.state_path == state_path {
            existing_lock.revalidate_store_entries()?;
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

pub(crate) fn with_state_file_lock<T>(
    operation: impl FnOnce(&mut StateFileLock) -> Result<T, EngineError>,
) -> Result<T, EngineError> {
    ensure_state_file_lock()?;
    let mut lock_slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;
    let store = lock_slot.as_mut().ok_or_else(|| {
        EngineError::Internal("signer durable store lock is not initialized".to_string())
    })?;
    operation(store)
}

pub(crate) fn with_state_file_lock_for_load<T>(
    operation: impl FnOnce(&mut StateFileLock) -> Result<T, EngineError>,
) -> Result<T, EngineError> {
    ensure_state_file_lock_for_load()?;
    let mut lock_slot = state_file_lock_slot()
        .lock()
        .map_err(|_| EngineError::Internal("state file lock mutex poisoned".to_string()))?;
    let store = lock_slot.as_mut().ok_or_else(|| {
        EngineError::Internal("signer durable store lock is not initialized".to_string())
    })?;
    operation(store)
}

pub(crate) fn durable_store_identity() -> Result<DurableStoreIdentity, EngineError> {
    with_state_file_lock(|store| store.identity())
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

pub(crate) fn active_session_count(sessions: &HashMap<String, SessionState>) -> usize {
    sessions
        .values()
        .filter(|session| session.retired_interactive_at_unix.is_none())
        .count()
}

#[cfg(test)]
pub(crate) fn retired_interactive_session_count(sessions: &HashMap<String, SessionState>) -> usize {
    sessions
        .values()
        .filter(|session| session.retired_interactive_at_unix.is_some())
        .count()
}

pub(crate) fn ensure_session_registry_persisted_bound(
    sessions: &HashMap<String, SessionState>,
) -> Result<(), EngineError> {
    let max_sessions = max_sessions_limit();
    let session_count = sessions.len();
    if session_count > max_sessions {
        return Err(EngineError::Internal(format!(
            "persisted session registry size [{session_count}] exceeds max [{max_sessions}]"
        )));
    }

    Ok(())
}

// Production interactive signing uses one outer session per message. Such a
// session is bound to a wallet key but never owns DKG material; DKG installation
// enforces that role split. Keep this match exhaustive so a future SessionState
// field forces an explicit retirement-safety decision here.
pub(crate) fn per_message_interactive_session(session: &SessionState) -> bool {
    let SessionState {
        dkg_request_fingerprint,
        dkg_key_packages,
        dkg_public_key_package,
        dkg_result,
        dkg_share_epoch,
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
        retired_interactive_at_unix,
        aggregate_eviction_pin,
        consumed_interactive_attempt_markers,
        authorized_interactive_aggregate_markers,
        aggregated_interactive_attempt_markers,
    } = session;

    let _ = (
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
        retired_interactive_at_unix,
        aggregate_eviction_pin,
        consumed_interactive_attempt_markers,
        authorized_interactive_aggregate_markers,
        aggregated_interactive_attempt_markers,
        dkg_share_epoch,
    );

    bound_key_group.is_some()
        && dkg_request_fingerprint.is_none()
        && dkg_key_packages.is_none()
        && dkg_public_key_package.is_none()
        && dkg_result.is_none()
        && *dkg_share_epoch == 0
}

pub(crate) fn retire_idle_per_message_sessions(
    engine_state: &mut EngineState,
    protected_session_id: Option<&str>,
) -> usize {
    retire_idle_per_message_session_ids(engine_state, protected_session_id).len()
}

pub(crate) fn retire_idle_per_message_session_ids(
    engine_state: &mut EngineState,
    protected_session_id: Option<&str>,
) -> Vec<String> {
    let retired_at = now_unix().max(1);
    let pending_session_ids = persistence_pending_session_ids();
    let mut newly_retired = Vec::new();
    for (session_id, session) in &mut engine_state.sessions {
        if !pending_session_ids.contains(session_id)
            && session.retired_interactive_at_unix.is_none()
            && session.interactive_signing.is_empty()
            && per_message_interactive_session(session)
        {
            session.retired_interactive_at_unix = Some(retired_at);
            newly_retired.push(session_id.clone());
        }
    }

    drop(compact_retired_per_message_sessions(
        engine_state,
        protected_session_id,
    ));
    newly_retired.retain(|session_id| engine_state.sessions.contains_key(session_id));
    newly_retired
}

pub(crate) fn compact_retired_per_message_sessions(
    engine_state: &mut EngineState,
    protected_session_id: Option<&str>,
) -> Vec<(String, SessionState)> {
    compact_retired_per_message_sessions_to_total(
        engine_state,
        max_sessions_limit(),
        protected_session_id,
    )
}

fn compact_retired_per_message_sessions_to_total(
    engine_state: &mut EngineState,
    max_total_sessions: usize,
    protected_session_id: Option<&str>,
) -> Vec<(String, SessionState)> {
    // A post-replacement persistence failure leaves the replacement snapshot's
    // marker in memory and records a process-local repair operation. Evicting
    // that session before a later successful snapshot would persist the
    // marker's absence and then clear the repair record. Protect every
    // session-scoped pending operation until a successful snapshot covers it.
    let pending_session_ids = persistence_pending_session_ids();
    let mut removed = Vec::new();
    // Schema version 1 readers predating retirement enforce this same bound on
    // the TOTAL map. Retired tombstones therefore consume only the portion of
    // the shared budget not occupied by active sessions; preserving a separate
    // retired allowance would make an emergency binary rollback fail at load.
    while engine_state.sessions.len() > max_total_sessions {
        let oldest = engine_state
            .sessions
            .iter()
            .filter_map(|(session_id, session)| {
                if protected_session_id == Some(session_id.as_str())
                    || pending_session_ids.contains(session_id)
                    || Arc::strong_count(&session.aggregate_eviction_pin) > 1
                {
                    return None;
                }
                session
                    .retired_interactive_at_unix
                    .map(|retired_at| (retired_at, session_id.clone()))
            })
            .min();
        let Some((_, oldest_session_id)) = oldest else {
            break;
        };
        let removed_session = engine_state
            .sessions
            .remove(&oldest_session_id)
            .expect("selected retired session existed under the held engine lock");
        removed.push((oldest_session_id, removed_session));
    }
    removed
}

pub(crate) fn restore_compacted_retired_sessions(
    engine_state: &mut EngineState,
    removed: Vec<(String, SessionState)>,
) {
    for (session_id, session) in removed {
        let previous = engine_state.sessions.insert(session_id, session);
        debug_assert!(
            previous.is_none(),
            "a compacted retired session must not be recreated while the engine lock is held"
        );
    }
}

fn has_evictable_retired_session(engine_state: &EngineState) -> bool {
    let pending_session_ids = persistence_pending_session_ids();
    engine_state.sessions.iter().any(|(session_id, session)| {
        session.retired_interactive_at_unix.is_some()
            && !pending_session_ids.contains(session_id)
            && Arc::strong_count(&session.aggregate_eviction_pin) == 1
    })
}

pub(crate) fn ensure_session_insert_admission_capacity(
    engine_state: &EngineState,
    session_id: &str,
) -> Result<(), EngineError> {
    if engine_state.sessions.contains_key(session_id) {
        return Ok(());
    }

    let max_sessions = max_sessions_limit();
    let active_count = active_session_count(&engine_state.sessions);
    if active_count >= max_sessions {
        return Err(EngineError::Internal(format!(
            "active session registry size [{active_count}] reached max [{max_sessions}]; use an existing session_id or increase {}",
            TBTC_SIGNER_MAX_SESSIONS_ENV
        )));
    }
    if engine_state.sessions.len() >= max_sessions && !has_evictable_retired_session(engine_state) {
        return Err(EngineError::Internal(format!(
            "session registry size [{}] reached max [{max_sessions}] and no retired session is available for eviction; use an existing session_id or increase {}",
            engine_state.sessions.len(),
            TBTC_SIGNER_MAX_SESSIONS_ENV
        )));
    }

    Ok(())
}

pub(crate) fn ensure_interactive_session_admission_capacity(
    engine_state: &EngineState,
    session_id: &str,
) -> Result<(), EngineError> {
    let existing_session = engine_state.sessions.get(session_id);
    let needs_active_slot = existing_session
        .map(|session| session.retired_interactive_at_unix.is_some())
        .unwrap_or(true);
    if !needs_active_slot {
        return Ok(());
    }

    if existing_session.is_none() {
        return ensure_session_insert_admission_capacity(engine_state, session_id);
    }

    let max_sessions = max_sessions_limit();
    let active_count = active_session_count(&engine_state.sessions);
    if active_count >= max_sessions {
        return Err(EngineError::Internal(format!(
            "active session registry size [{active_count}] reached max [{max_sessions}]; abort idle sessions or increase {}",
            TBTC_SIGNER_MAX_SESSIONS_ENV
        )));
    }

    Ok(())
}

pub(crate) fn reactivate_retired_per_message_session(
    engine_state: &mut EngineState,
    session_id: &str,
) -> Result<(), EngineError> {
    let is_retired = engine_state
        .sessions
        .get(session_id)
        .is_some_and(|session| session.retired_interactive_at_unix.is_some());
    if !is_retired {
        return Ok(());
    }

    let max_sessions = max_sessions_limit();
    let active_count = active_session_count(&engine_state.sessions);
    if active_count >= max_sessions {
        return Err(EngineError::Internal(format!(
            "active session registry size [{active_count}] reached max [{max_sessions}]; abort idle sessions or increase {}",
            TBTC_SIGNER_MAX_SESSIONS_ENV
        )));
    }

    engine_state
        .sessions
        .get_mut(session_id)
        .expect("retired session existed under the held engine lock")
        .retired_interactive_at_unix = None;
    Ok(())
}

pub(crate) fn ensure_session_insert_capacity(
    engine_state: &mut EngineState,
    session_id: &str,
) -> Result<Vec<(String, SessionState)>, EngineError> {
    if engine_state.sessions.contains_key(session_id) {
        return Ok(Vec::new());
    }

    ensure_session_insert_admission_capacity(engine_state, session_id)?;
    let max_sessions = max_sessions_limit();
    // Reserve one slot for the caller's insertion. The returned tombstones let
    // durable callers restore the exact pre-call map if persistence fails before
    // replacing the state file.
    let compacted = compact_retired_per_message_sessions_to_total(
        engine_state,
        max_sessions.saturating_sub(1),
        None,
    );
    if engine_state.sessions.len() >= max_sessions {
        restore_compacted_retired_sessions(engine_state, compacted);
        return Err(EngineError::Internal(format!(
            "session registry size [{}] reached max [{max_sessions}] and no retired session is available for eviction; use an existing session_id or increase {}",
            engine_state.sessions.len(),
            TBTC_SIGNER_MAX_SESSIONS_ENV
        )));
    }

    Ok(compacted)
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
