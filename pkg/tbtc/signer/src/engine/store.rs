//! Descriptor-bound durable signer store.
//!
//! The state image is atomically replaced, so its inode cannot be the store
//! identity.  The identity is instead anchored by a stable, fsynced store ID,
//! a stable exclusively-locked lock file, and the no-follow directory handle
//! through which every state operation is performed.
//!
//! This stable v2 identity proves which local store is open; it intentionally
//! does not claim pre-start state freshness or key-package inventory. A caller
//! must reconcile those through the separate inventory/witness contract.
//!
//! # Stable identity versus volatile descriptors (transcript v2)
//!
//! The store fingerprint that anchors the state-commitment chain binds ONLY the
//! stable, fsynced `.store-id` bytes. Path, device, and inode descriptors are
//! still validated on every access - they are the real defense against store
//! substitution - and are still reported for diagnostics, but they are NOT part
//! of any committed transcript. Under the retired v1 transcript they were: a
//! deleted lock file, a restore-from-backup at the same path, a renamed
//! directory, or a remount that moved `st_dev` silently invalidated every
//! committed record and left the signer unstartable, with `rm .state-witness`
//! (a generation-1 re-genesis, i.e. exactly the rollback the journal exists to
//! detect) as the only remaining operator action.

use super::*;

use std::ffi::{CString, OsStr, OsString};
use std::io::{Seek, SeekFrom};

#[cfg(unix)]
use std::os::fd::{AsRawFd, FromRawFd, RawFd};
#[cfg(unix)]
use std::os::unix::ffi::OsStrExt;

pub(crate) const TBTC_SIGNER_DURABLE_STORE_IDENTITY_SCHEMA: &str =
    "tbtc-signer-durable-session-store-identity/v2";
pub(crate) const TBTC_SIGNER_DURABLE_STORE_BACKEND: &str = "encrypted-file-v1";
pub(crate) const TBTC_SIGNER_DURABLE_STORE_ID_SUFFIX: &str = ".store-id";
pub(crate) const TBTC_SIGNER_STATE_WITNESS_SUFFIX: &str = ".state-witness";
pub(crate) const TBTC_SIGNER_STATE_ANCHOR_SUFFIX: &str = ".state-anchor";
pub(crate) const TBTC_SIGNER_STATE_ANCHOR_TRUST_SUFFIX: &str = ".state-anchor-trust";
pub(crate) const TBTC_SIGNER_STATE_ANCHOR_TRUST_INTENT_SUFFIX: &str = ".state-anchor-trust.intent";
const TBTC_SIGNER_STATE_WITNESS_NEXT_SUFFIX: &str = ".next";
const TBTC_SIGNER_STATE_WITNESS_PREVIOUS_SUFFIX: &str = ".previous";

const TBTC_SIGNER_DURABLE_STORE_FINGERPRINT_DOMAIN: &[u8] =
    b"tbtc-signer-durable-session-store-fingerprint-v2\0";
const TBTC_SIGNER_DURABLE_STORE_PATH_FINGERPRINT_DOMAIN: &[u8] =
    b"tbtc-signer-durable-session-store-canonical-path-v1\0";
const TBTC_SIGNER_DURABLE_STORE_FILESYSTEM_FINGERPRINT_DOMAIN: &[u8] =
    b"tbtc-signer-durable-session-store-filesystem-v1\0";
const TBTC_SIGNER_DURABLE_STORE_LOCK_FINGERPRINT_DOMAIN: &[u8] =
    b"tbtc-signer-durable-session-store-lock-v1\0";
const TBTC_SIGNER_STATE_IMAGE_DIGEST_DOMAIN: &[u8] = b"tbtc-signer-durable-state-image-digest-v1\0";
const TBTC_SIGNER_STATE_WITNESS_GENESIS_DOMAIN: &[u8] = b"tbtc-signer-state-witness-genesis-v2\0";
const TBTC_SIGNER_STATE_COMMITMENT_DOMAIN: &[u8] = b"tbtc-signer-state-witness-commitment-v2\0";
const TBTC_SIGNER_STATE_WITNESS_MAGIC: &[u8; 16] = b"TBTCWITNESSv2\0\0\0";
const TBTC_SIGNER_STATE_WITNESS_SEGMENT_MAGIC: &[u8; 16] = b"TBTCWITNESSSEG1\0";
/// The retired v1 journal magic. It is never written and never repaired; it is
/// recognized only so a v1 store fails closed with an actionable migration
/// error instead of a generic "invalid commitment".
const TBTC_SIGNER_STATE_WITNESS_MAGIC_V1: &[u8; 16] = b"TBTCWITNESSv1\0\0\0";
pub(crate) const TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH: usize = 48;
pub(crate) const TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH: usize = 472;
/// The journal is a fixed-width header followed by fixed-width records; the
/// tests build on-disk fixtures from this geometry, so it is part of the
/// crate-visible store contract.
pub(crate) const TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH: usize = 105;
const TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE: u8 = 1;
const TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT: u8 = 2;
const TBTC_SIGNER_STATE_WITNESS_RECORD_ABORT: u8 = 3;
const TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_VERSION: u32 = 1;
const TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_DOMAIN: &[u8] =
    b"tbtc-signer-state-witness-segment-header/v1\0";

const TBTC_SIGNER_STATE_ANCHOR_MAGIC: &[u8; 16] = b"TBTCSTATEANCH1\0\0";
const TBTC_SIGNER_STATE_ANCHOR_VERSION: u32 = 1;
// Fixed-width canonical encoding of every field in
// `StateAnchorAcknowledgement`: fourteen bytes32 values, one 64-byte
// signature, five u64 values, and one status byte followed by seven reserved
// zero bytes.
const TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH: usize = 560;
const TBTC_SIGNER_STATE_ANCHOR_METADATA_LENGTH: usize =
    16 + 4 + 32 + 4 + 3 * TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH + 32;
const TBTC_SIGNER_STATE_ANCHOR_METADATA_DOMAIN: &[u8] = b"tbtc-signer-state-anchor-metadata/v1\0";

#[cfg(test)]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
// The `After` prefix is load-bearing: every fault injects after the named
// durable step, and the shared prefix keeps that invariant explicit.
#[allow(clippy::enum_variant_names)]
enum StateAnchorTrustTransitionFaultInjectionPoint {
    AfterIntentPublication,
    AfterPrepareBatch,
    AfterNextWitnessPublication,
    AfterPreviousWitnessPublication,
    AfterCurrentWitnessPublication,
    AfterTargetAnchorPublication,
    AfterCommitPublication,
    AfterPreviousWitnessRetirement,
}

#[cfg(test)]
static STATE_ANCHOR_TRUST_TRANSITION_FAULT_INJECTION_POINT: OnceLock<
    Mutex<Option<StateAnchorTrustTransitionFaultInjectionPoint>>,
> = OnceLock::new();

#[cfg(test)]
static REPLACE_TRUST_LOCK_AFTER_GUARDED_PUBLICATION: std::sync::atomic::AtomicBool =
    std::sync::atomic::AtomicBool::new(false);

#[cfg(test)]
static REPLACE_TRUST_LOCK_AFTER_FLOCK: std::sync::atomic::AtomicBool =
    std::sync::atomic::AtomicBool::new(false);

#[cfg(test)]
fn maybe_inject_state_anchor_trust_transition_fault(
    point: StateAnchorTrustTransitionFaultInjectionPoint,
) -> Result<(), EngineError> {
    let configured = *STATE_ANCHOR_TRUST_TRANSITION_FAULT_INJECTION_POINT
        .get_or_init(|| Mutex::new(None))
        .lock()
        .map_err(|_| {
            EngineError::Internal(
                "state-anchor trust transition fault-injection mutex poisoned".to_string(),
            )
        })?;
    if configured == Some(point) {
        return Err(EngineError::Internal(format!(
            "injected state-anchor trust transition fault at {point:?}"
        )));
    }
    Ok(())
}

#[cfg(test)]
fn set_state_anchor_trust_transition_fault_for_tests(
    point: StateAnchorTrustTransitionFaultInjectionPoint,
) {
    if let Ok(mut configured) = STATE_ANCHOR_TRUST_TRANSITION_FAULT_INJECTION_POINT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        *configured = Some(point);
    }
}

#[cfg(test)]
fn clear_state_anchor_trust_transition_fault_for_tests() {
    if let Ok(mut configured) = STATE_ANCHOR_TRUST_TRANSITION_FAULT_INJECTION_POINT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        *configured = None;
    }
}

#[cfg(test)]
fn maybe_replace_trust_lock_after_guarded_publication(
    recovery_guard: Option<&StateAnchorTrustRecoveryGuard<'_>>,
) -> Result<(), EngineError> {
    if !REPLACE_TRUST_LOCK_AFTER_GUARDED_PUBLICATION
        .swap(false, std::sync::atomic::Ordering::SeqCst)
    {
        return Ok(());
    }
    let guard = recovery_guard.ok_or_else(|| {
        EngineError::Internal(
            "post-publication lock replacement requires a recovery guard".to_string(),
        )
    })?;
    let mut displaced_name = guard.lock_name.to_os_string();
    displaced_name.push(".test-post-publication-displaced");
    validate_entry_name(&displaced_name, "displaced test lock")?;
    ensure_entry_absent(
        guard.directory.as_raw_fd(),
        &displaced_name,
        "displaced test lock",
    )?;
    renameat_same_directory(
        guard.directory.as_raw_fd(),
        guard.lock_name,
        &displaced_name,
        "displace trust-recovery lock in publication-window test",
    )?;
    let replacement = openat_regular(
        guard.directory.as_raw_fd(),
        guard.lock_name,
        libc::O_RDWR | libc::O_CREAT | libc::O_EXCL,
        0o600,
        "replacement test lock",
    )?;
    set_owner_only_permissions(&replacement, "replacement test lock")?;
    replacement.sync_all().map_err(|error| {
        EngineError::Internal(format!("failed to sync replacement test lock: {error}"))
    })?;
    guard.directory.sync_all().map_err(|error| {
        EngineError::Internal(format!(
            "failed to sync publication-window lock replacement: {error}"
        ))
    })
}

#[cfg(test)]
fn maybe_replace_trust_lock_after_flock(
    directory: &fs::File,
    lock_name: &OsStr,
) -> Result<(), EngineError> {
    if !REPLACE_TRUST_LOCK_AFTER_FLOCK.swap(false, std::sync::atomic::Ordering::SeqCst) {
        return Ok(());
    }
    let mut displaced_name = lock_name.to_os_string();
    displaced_name.push(".test-post-flock-displaced");
    validate_entry_name(&displaced_name, "post-flock displaced test lock")?;
    ensure_entry_absent(
        directory.as_raw_fd(),
        &displaced_name,
        "post-flock displaced test lock",
    )?;
    renameat_same_directory(
        directory.as_raw_fd(),
        lock_name,
        &displaced_name,
        "displace lock after flock",
    )?;
    let replacement = openat_regular(
        directory.as_raw_fd(),
        lock_name,
        libc::O_RDWR | libc::O_CREAT | libc::O_EXCL,
        0o600,
        "post-flock replacement test lock",
    )?;
    set_owner_only_permissions(&replacement, "post-flock replacement test lock")?;
    replacement.sync_all().map_err(|error| {
        EngineError::Internal(format!(
            "failed to sync post-flock replacement lock: {error}"
        ))
    })?;
    directory.sync_all().map_err(|error| {
        EngineError::Internal(format!(
            "failed to sync post-flock lock replacement: {error}"
        ))
    })
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct OpenedObjectIdentity {
    device: u64,
    inode: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct DurableStoreIdentity {
    pub(crate) store_id: [u8; 32],
    /// Diagnostic-only descriptors. They are recomputed and CHECKED on every
    /// store access, but deliberately do not enter the state-commitment
    /// transcript: each of them changes under benign operations (lock-file
    /// cleanup, restore-from-backup, directory rename, remount).
    pub(crate) canonical_path_fingerprint: [u8; 32],
    pub(crate) filesystem_fingerprint: [u8; 32],
    pub(crate) lock_fingerprint: [u8; 32],
    /// The stable transcript anchor: a function of `store_id` alone. This is
    /// the value bound into every state commitment and into the witness
    /// genesis root.
    pub(crate) fingerprint: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct StateWitness {
    pub(crate) generation: u64,
    pub(crate) previous_commitment: [u8; 32],
    pub(crate) commitment: [u8; 32],
    pub(crate) state_image_digest: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct StateWitnessSegmentHeader {
    store_fingerprint: [u8; 32],
    base: StateWitness,
    binding_hash: [u8; 32],
    service_epoch: u64,
    revision: u64,
    previous_event_root: [u8; 32],
    event_root: [u8; 32],
    operation_id: [u8; 32],
    transition_digest: [u8; 32],
    committed_at_unix_ms: u64,
    acknowledgement_digest: [u8; 32],
    signature: [u8; 64],
    header_commitment: [u8; 32],
}

#[cfg(unix)]
struct ParsedStateWitnessJournal {
    history: Vec<StateWitness>,
    pending: Option<StateWitness>,
    length: usize,
    header_length: usize,
    header_bytes: Vec<u8>,
    segment_header: Option<StateWitnessSegmentHeader>,
    tail_record: [u8; TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH],
}

#[cfg(unix)]
struct OpenedStateWitnessJournal {
    file: fs::File,
    identity: OpenedObjectIdentity,
    parsed: ParsedStateWitnessJournal,
}

#[cfg(unix)]
type OpenedStateAnchor = (
    Option<fs::File>,
    Option<OpenedObjectIdentity>,
    Option<StateAnchorMetadata>,
    Option<Vec<u8>>,
);

#[cfg(unix)]
type OpenedStateAnchorTrustJournal = (
    Option<fs::File>,
    Option<OpenedObjectIdentity>,
    Option<StateAnchorTrustJournalModel>,
    Option<FileChangeStamp>,
    Option<Vec<u8>>,
);

#[cfg(unix)]
#[derive(Clone, Copy)]
struct StateWitnessRotationNames<'a> {
    current: &'a OsStr,
    next: &'a OsStr,
    previous: &'a OsStr,
}

#[cfg(unix)]
struct StateAnchorTrustRecoveryGuard<'a> {
    directory: &'a fs::File,
    canonical_parent: &'a Path,
    directory_identity: OpenedObjectIdentity,
    lock_name: &'a OsStr,
    lock_file: &'a fs::File,
    lock_identity: OpenedObjectIdentity,
    store_id_name: &'a OsStr,
    store_id_file: &'a fs::File,
    store_id_identity: OpenedObjectIdentity,
    store_id: [u8; 32],
    state_name: &'a OsStr,
    state_file: Option<&'a fs::File>,
    state_identity: Option<OpenedObjectIdentity>,
}

#[cfg(unix)]
impl StateAnchorTrustRecoveryGuard<'_> {
    fn revalidate(&self) -> Result<(), EngineError> {
        let live_directory = open_absolute_directory_nofollow(self.canonical_parent)?;
        if descriptor_identity(&live_directory, "live signer state directory")?
            != self.directory_identity
        {
            return Err(replacement_error("signer state directory"));
        }
        validate_live_entry(
            self.directory,
            self.lock_name,
            self.lock_identity,
            "signer state lock file",
        )?;
        validate_live_entry(
            self.directory,
            self.store_id_name,
            self.store_id_identity,
            "signer durable store ID file",
        )?;
        validate_secure_regular_file(self.lock_file, "signer state lock file")?;
        validate_secure_regular_file(self.store_id_file, "signer durable store ID file")?;
        if read_store_id(self.store_id_file)? != self.store_id {
            return Err(EngineError::Internal(
                "signer durable store ID changed during trust recovery".to_string(),
            ));
        }
        match (self.state_file, self.state_identity) {
            (Some(file), Some(identity)) => {
                validate_live_entry(
                    self.directory,
                    self.state_name,
                    identity,
                    "signer state file",
                )?;
                validate_secure_regular_file(file, "signer state file")?;
            }
            (None, None) => ensure_entry_absent(
                self.directory.as_raw_fd(),
                self.state_name,
                "signer state file",
            )?,
            _ => {
                return Err(EngineError::Internal(
                    "signer state descriptor invariant is inconsistent during trust recovery"
                        .to_string(),
                ))
            }
        }
        Ok(())
    }
}

/// Cheap, exact evidence that a file has not been written since it was last
/// inspected: size plus the mtime and ctime pairs. Any write moves at least one
/// of them, and ctime cannot be back-dated by an unprivileged writer. A stamp
/// mismatch never admits anything - it only forces a full re-verification.
#[cfg(unix)]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct FileChangeStamp {
    size: u64,
    modified_seconds: u64,
    modified_nanoseconds: u64,
    changed_seconds: u64,
    changed_nanoseconds: u64,
}

/// The verified prefix of the append-only witness journal.
///
/// The journal is append-only, so verification is incremental: the bytes below
/// `verified_length` have already been parsed and matched against the in-memory
/// history, and only newly appended bytes need to be read back. The anchor -
/// last verified commitment and generation - plus the exact trailing record
/// bytes and the file change stamp are what a later access re-checks in O(1)
/// before trusting the prefix.
///
/// This cache lives only in the `StateFileLock` instance, so it is never a
/// trust anchor across process restarts: a fresh open always re-parses and
/// re-hashes the entire journal.
#[cfg(unix)]
#[derive(Clone, Debug)]
struct WitnessJournalPrefix {
    identity: OpenedObjectIdentity,
    stamp: FileChangeStamp,
    verified_length: usize,
    history_length: usize,
    tip_generation: u64,
    tip_commitment: [u8; 32],
    tail_record: [u8; TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH],
}

/// Counts full journal re-parses. The incremental path must keep this flat as
/// the journal grows; the test suite asserts exactly that.
#[cfg(all(test, unix))]
pub(crate) static WITNESS_FULL_VERIFICATIONS: std::sync::atomic::AtomicU64 =
    std::sync::atomic::AtomicU64::new(0);

/// Counts verifications served from the verified prefix.
#[cfg(all(test, unix))]
pub(crate) static WITNESS_INCREMENTAL_VERIFICATIONS: std::sync::atomic::AtomicU64 =
    std::sync::atomic::AtomicU64::new(0);

/// Counts journal bytes read for verification. This is the direct measure of
/// the fix: it must grow with the bytes appended, not with accesses times
/// journal length.
#[cfg(all(test, unix))]
pub(crate) static WITNESS_VERIFIED_BYTES_READ: std::sync::atomic::AtomicU64 =
    std::sync::atomic::AtomicU64::new(0);

#[cfg(all(test, unix))]
pub(crate) fn reset_witness_verification_counters() {
    use std::sync::atomic::Ordering;
    WITNESS_FULL_VERIFICATIONS.store(0, Ordering::SeqCst);
    WITNESS_INCREMENTAL_VERIFICATIONS.store(0, Ordering::SeqCst);
    WITNESS_VERIFIED_BYTES_READ.store(0, Ordering::SeqCst);
}

/// `(full re-parses, incremental verifications, journal bytes read)`.
#[cfg(all(test, unix))]
pub(crate) fn witness_verification_counters() -> (u64, u64, u64) {
    use std::sync::atomic::Ordering;
    (
        WITNESS_FULL_VERIFICATIONS.load(Ordering::SeqCst),
        WITNESS_INCREMENTAL_VERIFICATIONS.load(Ordering::SeqCst),
        WITNESS_VERIFIED_BYTES_READ.load(Ordering::SeqCst),
    )
}

/// A process-lifetime handle to the exact durable store opened by the signer.
///
/// The public path fields are retained for diagnostics and existing tests. All
/// security-sensitive operations use `directory` plus `openat`/`renameat` and
/// compare live directory entries with the held descriptors before proceeding.
pub(crate) struct StateFileLock {
    pub(crate) _file: fs::File,
    pub(crate) state_path: PathBuf,
    pub(crate) lock_path: PathBuf,
    directory: fs::File,
    canonical_parent: PathBuf,
    state_name: OsString,
    lock_name: OsString,
    store_id_name: OsString,
    directory_identity: OpenedObjectIdentity,
    lock_identity: OpenedObjectIdentity,
    store_id_file: fs::File,
    store_id_identity: OpenedObjectIdentity,
    trust_name: OsString,
    trust_file: Option<fs::File>,
    trust_identity: Option<OpenedObjectIdentity>,
    trust_journal: Option<StateAnchorTrustJournalModel>,
    trust_stamp: Option<FileChangeStamp>,
    trust_bytes: Option<Vec<u8>>,
    trust_intent_name: OsString,
    trust_head_inspection: bool,
    anchor_configuration: Option<StateAnchorConfiguration>,
    anchor_name: OsString,
    anchor_file: Option<fs::File>,
    anchor_identity: Option<OpenedObjectIdentity>,
    anchor_metadata: Option<StateAnchorMetadata>,
    anchor_bytes: Option<Vec<u8>>,
    witness_name: OsString,
    witness_next_name: OsString,
    witness_previous_name: OsString,
    witness_file: fs::File,
    witness_identity: OpenedObjectIdentity,
    witness_history: Vec<StateWitness>,
    pending_witness: Option<StateWitness>,
    witness_length: usize,
    witness_max_records: usize,
    witness_rotation_threshold: Option<usize>,
    witness_header_length: usize,
    witness_header_bytes: Vec<u8>,
    witness_segment_header: Option<StateWitnessSegmentHeader>,
    /// The verified prefix of the journal. `None` means "nothing is cached",
    /// which forces the next verification to parse the whole journal. It is
    /// deliberately `None` on every fresh open.
    #[cfg(unix)]
    witness_prefix: Option<WitnessJournalPrefix>,
    /// Bytes of the most recently appended record, used to verify the append
    /// read-back and to anchor the cached prefix.
    #[cfg(unix)]
    last_appended_record: [u8; TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH],
    /// Exact file stamp captured immediately after the appended record was
    /// fsynced. The append read-back must observe this stamp before adopting a
    /// new verified-prefix baseline.
    #[cfg(unix)]
    last_appended_stamp: Option<FileChangeStamp>,
    current_state_file: Option<fs::File>,
    current_state_identity: Option<OpenedObjectIdentity>,
    identity: DurableStoreIdentity,
    lock_held: bool,
}

#[derive(Clone, Debug)]
pub(crate) struct StateAnchorTrustTransitionStoreOutcome {
    pub(crate) idempotent: bool,
    pub(crate) applied_certificate_count: usize,
    pub(crate) trust_head: VerifiedStateAnchorTrustCertificate,
    pub(crate) tip: StateWitness,
    pub(crate) base: StateWitness,
    pub(crate) anchor: StateAnchorMetadata,
}

#[cfg(unix)]
enum StateFileLockAcquireMode<'a> {
    Ordinary,
    BootstrapFactsProvisioning,
    TrustHeadInspection,
    TrustTransition(&'a VerifiedStateAnchorTrustTransition),
}

impl StateFileLock {
    #[cfg(unix)]
    pub(crate) fn acquire(state_path: &Path) -> Result<Self, EngineError> {
        Self::acquire_with_mode(state_path, StateFileLockAcquireMode::Ordinary)
    }

    #[cfg(unix)]
    pub(crate) fn acquire_for_trust_transition(
        state_path: &Path,
        transition: &VerifiedStateAnchorTrustTransition,
    ) -> Result<Self, EngineError> {
        Self::acquire_with_mode(
            state_path,
            StateFileLockAcquireMode::TrustTransition(transition),
        )
    }

    #[cfg(unix)]
    pub(crate) fn acquire_for_trust_head_inspection(
        state_path: &Path,
    ) -> Result<Self, EngineError> {
        Self::acquire_with_mode(state_path, StateFileLockAcquireMode::TrustHeadInspection)
    }

    #[cfg(unix)]
    pub(crate) fn acquire_for_bootstrap_facts(state_path: &Path) -> Result<Self, EngineError> {
        Self::acquire_with_mode(
            state_path,
            StateFileLockAcquireMode::BootstrapFactsProvisioning,
        )
    }

    #[cfg(unix)]
    fn acquire_with_mode(
        state_path: &Path,
        mode: StateFileLockAcquireMode<'_>,
    ) -> Result<Self, EngineError> {
        let state_name = state_path
            .file_name()
            .filter(|name| !name.is_empty())
            .ok_or_else(|| {
                EngineError::Internal(format!(
                    "signer state path [{}] has no file name",
                    state_path.display()
                ))
            })?
            .to_os_string();
        validate_entry_name(&state_name, "state")?;
        let trust_intent_name = state_anchor_trust_intent_file_name(&state_name);
        validate_entry_name(&trust_intent_name, "state anchor trust transition intent")?;

        let configured_parent = state_path
            .parent()
            .filter(|parent| !parent.as_os_str().is_empty())
            .unwrap_or_else(|| Path::new("."));
        let canonical_parent = match fs::canonicalize(configured_parent) {
            Ok(parent) => parent,
            Err(error)
                if error.kind() == std::io::ErrorKind::NotFound
                    && matches!(&mode, StateFileLockAcquireMode::TrustHeadInspection) =>
            {
                // Trust-head inspection is read-only. An absent parent proves
                // there cannot be a durable intent or committed head and must
                // not create the configured directory hierarchy.
                return Err(EngineError::StateAnchorTrustHeadAbsent);
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                fs::create_dir_all(configured_parent).map_err(|create_error| {
                    EngineError::Internal(format!(
                        "failed to create signer state directory [{}]: {create_error}",
                        configured_parent.display()
                    ))
                })?;
                fs::canonicalize(configured_parent).map_err(|canonicalize_error| {
                    EngineError::Internal(format!(
                        "failed to canonicalize newly created signer state directory [{}]: \
                         {canonicalize_error}",
                        configured_parent.display()
                    ))
                })?
            }
            Err(error) => {
                return Err(EngineError::Internal(format!(
                    "failed to canonicalize signer state directory [{}]: {error}",
                    configured_parent.display()
                )));
            }
        };
        if !canonical_parent.is_absolute() {
            return Err(EngineError::Internal(format!(
                "canonical signer state directory [{}] is not absolute",
                canonical_parent.display()
            )));
        }

        // Traverse the canonical absolute path one component at a time. A
        // concurrent symlink substitution therefore fails with ELOOP instead
        // of redirecting the store open.
        let directory = open_absolute_directory_nofollow(&canonical_parent)?;
        let directory_identity = descriptor_identity(&directory, "signer state directory")?;

        let lock_path = state_lock_file_path(state_path);
        let lock_name = lock_path
            .file_name()
            .ok_or_else(|| {
                EngineError::Internal(format!(
                    "signer state lock path [{}] has no file name",
                    lock_path.display()
                ))
            })?
            .to_os_string();
        validate_entry_name(&lock_name, "lock")?;

        let intent_existed_before_lock = live_entry_stat(
            directory.as_raw_fd(),
            &trust_intent_name,
            "state anchor trust transition intent",
        )?
        .is_some();
        let mut lock_file = match openat_optional(
            directory.as_raw_fd(),
            &lock_name,
            libc::O_RDWR,
            "signer state lock file",
        )? {
            Some(file) => file,
            None if intent_existed_before_lock => {
                return Err(EngineError::Internal(
                    "durable state-anchor trust intent exists without its signer state lock; \
                     refusing to recreate recovery prerequisites"
                        .to_string(),
                ));
            }
            None => openat_regular(
                directory.as_raw_fd(),
                &lock_name,
                libc::O_RDWR | libc::O_CREAT | libc::O_EXCL,
                0o600,
                "signer state lock file",
            )?,
        };
        validate_owned_unlinked_regular(&lock_file, "signer state lock file")?;
        acquire_exclusive_lock(&lock_file, &lock_path)?;
        let lock_identity = descriptor_identity(&lock_file, "signer state lock file")?;
        #[cfg(test)]
        maybe_replace_trust_lock_after_flock(&directory, &lock_name)?;
        validate_live_entry(
            &directory,
            &lock_name,
            lock_identity,
            "signer state lock file",
        )?;
        let recovery_intent_present = live_entry_stat(
            directory.as_raw_fd(),
            &trust_intent_name,
            "state anchor trust transition intent",
        )?
        .is_some();
        if recovery_intent_present {
            // A local intent is evidence only. Until a fresh transition request
            // is supplied, inspection must not chmod, truncate, rewrite, create,
            // rename, unlink, or fsync any recovery prerequisite.
            validate_secure_regular_file(&lock_file, "signer state lock file")?;
        } else {
            set_owner_only_permissions(&lock_file, "signer state lock file")?;
            validate_secure_regular_file(&lock_file, "signer state lock file")?;
            lock_file.set_len(0).map_err(|error| {
                EngineError::Internal(format!(
                    "failed to truncate signer state lock file [{}]: {error}",
                    lock_path.display()
                ))
            })?;
            lock_file.seek(SeekFrom::Start(0)).map_err(|error| {
                EngineError::Internal(format!(
                    "failed to seek signer state lock file [{}]: {error}",
                    lock_path.display()
                ))
            })?;
            writeln!(
                lock_file,
                "pid={}\ncanonical_state_path={}",
                std::process::id(),
                canonical_parent.join(&state_name).display()
            )
            .map_err(|error| {
                EngineError::Internal(format!(
                    "failed to write signer state lock file [{}]: {error}",
                    lock_path.display()
                ))
            })?;
            lock_file.sync_all().map_err(|error| {
                EngineError::Internal(format!(
                    "failed to sync signer state lock file [{}]: {error}",
                    lock_path.display()
                ))
            })?;
        }

        let store_id_name = durable_store_id_file_name(&state_name);
        validate_entry_name(&store_id_name, "store ID")?;
        let (store_id_file, store_id, store_id_identity) = if recovery_intent_present {
            let file = openat_optional(
                directory.as_raw_fd(),
                &store_id_name,
                libc::O_RDONLY,
                "signer durable store ID file",
            )?
            .ok_or_else(|| {
                EngineError::Internal(
                    "durable state-anchor trust intent exists without its signer durable store \
                     ID; refusing to recreate recovery prerequisites"
                        .to_string(),
                )
            })?;
            validate_owned_unlinked_regular(&file, "signer durable store ID file")?;
            validate_secure_regular_file(&file, "signer durable store ID file")?;
            let store_id = read_store_id(&file)?;
            let identity = descriptor_identity(&file, "signer durable store ID file")?;
            (file, store_id, identity)
        } else {
            open_or_create_store_id(&directory, &store_id_name)?
        };

        // Persist the creation of both stable anchor entries before claiming
        // durability to the host.
        if !recovery_intent_present {
            directory.sync_all().map_err(|error| {
                EngineError::Internal(format!(
                    "failed to sync signer state directory [{}]: {error}",
                    canonical_parent.display()
                ))
            })?;
        }

        let (current_state_file, current_state_identity) =
            open_optional_state(&directory, &state_name)?;

        let canonical_path_fingerprint = hash_fields(
            TBTC_SIGNER_DURABLE_STORE_PATH_FINGERPRINT_DOMAIN,
            &[
                canonical_parent.as_os_str().as_bytes(),
                state_name.as_bytes(),
                &directory_identity.device.to_be_bytes(),
                &directory_identity.inode.to_be_bytes(),
            ],
        );
        let filesystem_fingerprint = hash_fields(
            TBTC_SIGNER_DURABLE_STORE_FILESYSTEM_FINGERPRINT_DOMAIN,
            &[&directory_identity.device.to_be_bytes()],
        );
        let lock_fingerprint = hash_fields(
            TBTC_SIGNER_DURABLE_STORE_LOCK_FINGERPRINT_DOMAIN,
            &[
                lock_name.as_bytes(),
                &lock_identity.device.to_be_bytes(),
                &lock_identity.inode.to_be_bytes(),
            ],
        );
        // Only the stable store ID anchors the committed transcript. The three
        // descriptor fingerprints above stay in the identity for diagnostics
        // and are enforced by `revalidate_store_entries`, but binding them into
        // the commitment would make every committed record unverifiable after a
        // benign lock-file, path, inode, or device change.
        let fingerprint = durable_store_fingerprint(&store_id);
        let identity = DurableStoreIdentity {
            store_id,
            canonical_path_fingerprint,
            filesystem_fingerprint,
            lock_fingerprint,
            fingerprint,
        };

        let configured_anchor = configured_state_anchor()?;
        let trust_name = state_anchor_trust_file_name(&state_name);
        validate_entry_name(&trust_name, "state anchor trust journal")?;
        let anchor_name = state_anchor_file_name(&state_name);
        validate_entry_name(&anchor_name, "state anchor")?;
        let witness_name = state_witness_file_name(&state_name);
        validate_entry_name(&witness_name, "state witness")?;
        let mut witness_next_name = witness_name.clone();
        witness_next_name.push(TBTC_SIGNER_STATE_WITNESS_NEXT_SUFFIX);
        validate_entry_name(&witness_next_name, "next state witness")?;
        let mut witness_previous_name = witness_name.clone();
        witness_previous_name.push(TBTC_SIGNER_STATE_WITNESS_PREVIOUS_SUFFIX);
        validate_entry_name(&witness_previous_name, "previous state witness")?;
        let witness_max_records = state_witness_max_records()?;

        let opened_recovery_intent = open_state_anchor_trust_transition_intent(
            &directory,
            &trust_intent_name,
            &identity.fingerprint,
        )?;
        if recovery_intent_present && opened_recovery_intent.is_none() {
            return Err(EngineError::Internal(
                "state-anchor trust transition intent disappeared while its store lock was held"
                    .to_string(),
            ));
        }
        if let Some(intent_bytes) = opened_recovery_intent {
            if matches!(&mode, StateFileLockAcquireMode::BootstrapFactsProvisioning) {
                return Err(EngineError::Validation(
                    "bootstrap facts require a pristine store without a trust-transition intent"
                        .to_string(),
                ));
            }
            let request =
                parse_state_anchor_trust_transition_intent(&intent_bytes, &identity.fingerprint)?;
            let persisted_transition = verify_state_anchor_trust_transition_request(request, false)
                .map_err(|error| {
                    EngineError::Internal(format!(
                        "durable state-anchor trust transition intent failed verification: \
                         {error}"
                    ))
                })?;
            let requested_transition = match &mode {
                StateFileLockAcquireMode::TrustTransition(requested) => {
                    let persisted_final = persisted_transition
                        .certificates
                        .last()
                        .expect("verified durable transition is nonempty");
                    if requested.request.schema != persisted_transition.request.schema
                        || requested.request.certificate_chain
                            != persisted_transition.request.certificate_chain
                        || requested.certificates.len() != persisted_transition.certificates.len()
                        || requested
                            .certificates
                            .iter()
                            .zip(&persisted_transition.certificates)
                            .any(|(requested, persisted)| {
                                requested.certificate_sequence != persisted.certificate_sequence
                                    || requested.certificate_digest != persisted.certificate_digest
                                    || requested.target_acknowledgement_bytes
                                        != persisted.target_acknowledgement_bytes
                            })
                        || requested.target_read_acknowledgement_bytes
                            != persisted_final.target_acknowledgement_bytes
                        || requested.target_read_acknowledgement
                            != persisted_final.target_acknowledgement
                    {
                        return Err(EngineError::Validation(
                            "state-anchor trust transition certificate chain or freshly read \
                             target acknowledgement differs from the durable recovery intent"
                                .to_string(),
                        ));
                    }
                    *requested
                }
                _ => {
                    let final_certificate = persisted_transition
                        .certificates
                        .last()
                        .expect("verified durable transition is nonempty");
                    return Err(EngineError::StateAnchorTrustRecoveryRequired {
                        context: Box::new(StateAnchorTrustRecoveryContext {
                            store_fingerprint: identity.fingerprint,
                            certificate_sequences: persisted_transition
                                .certificates
                                .iter()
                                .map(|certificate| certificate.certificate_sequence)
                                .collect(),
                            certificate_digests: persisted_transition
                                .certificates
                                .iter()
                                .map(|certificate| certificate.certificate_digest)
                                .collect(),
                            target_binding_hash: final_certificate.to.binding_hash,
                            target_service_epoch: final_certificate.to.reference.service_epoch,
                            target_revision: final_certificate.to.reference.revision,
                            target_checkpoint_store_fingerprint: final_certificate
                                .to
                                .reference
                                .checkpoint
                                .store_fingerprint,
                            target_checkpoint_generation: final_certificate
                                .to
                                .reference
                                .checkpoint
                                .generation,
                            target_checkpoint_previous_state_commitment: final_certificate
                                .to
                                .reference
                                .checkpoint
                                .previous_state_commitment,
                            target_checkpoint_state_image_digest: final_certificate
                                .to
                                .reference
                                .checkpoint
                                .state_image_digest,
                            target_checkpoint_state_commitment: final_certificate
                                .to
                                .reference
                                .checkpoint
                                .state_commitment,
                        }),
                    });
                }
            };
            recheck_state_anchor_admission_expiry(
                requested_transition.target_read_expires_at_unix_ms,
            )?;
            let recovery_guard = StateAnchorTrustRecoveryGuard {
                directory: &directory,
                canonical_parent: &canonical_parent,
                directory_identity,
                lock_name: &lock_name,
                lock_file: &lock_file,
                lock_identity,
                store_id_name: &store_id_name,
                store_id_file: &store_id_file,
                store_id_identity,
                store_id,
                state_name: &state_name,
                state_file: current_state_file.as_ref(),
                state_identity: current_state_identity,
            };
            recovery_guard.revalidate()?;
            recover_state_anchor_trust_transition(
                &recovery_guard,
                &trust_name,
                &trust_intent_name,
                &anchor_name,
                StateWitnessRotationNames {
                    current: &witness_name,
                    next: &witness_next_name,
                    previous: &witness_previous_name,
                },
                &identity,
                current_state_file.as_ref(),
                witness_max_records,
                configured_anchor.as_ref(),
                &intent_bytes,
                requested_transition,
            )?;
        }
        ensure_entry_absent(
            directory.as_raw_fd(),
            &trust_intent_name,
            "state anchor trust transition intent",
        )?;
        let trust_required = matches!(
            &mode,
            StateFileLockAcquireMode::Ordinary | StateFileLockAcquireMode::TrustHeadInspection
        ) && configured_anchor
            .as_ref()
            .and_then(|configuration| configuration.trust.as_ref())
            .is_some();
        let (trust_file, trust_identity, trust_journal, trust_stamp, trust_bytes) =
            open_state_anchor_trust_journal(
                &directory,
                &trust_name,
                &identity.fingerprint,
                trust_required,
            )?;
        let anchor_configuration = match &mode {
            StateFileLockAcquireMode::Ordinary => {
                if let (Some(journal), Some(configuration)) =
                    (trust_journal.as_ref(), configured_anchor.as_ref())
                {
                    validate_state_anchor_trust_journal_head(
                        journal,
                        configuration,
                        &identity.fingerprint,
                    )?;
                }
                configured_anchor.clone()
            }
            StateFileLockAcquireMode::BootstrapFactsProvisioning => {
                if configured_anchor.is_some() || trust_journal.is_some() {
                    return Err(EngineError::Validation(
                        "bootstrap facts require a pristine store without anchor or trust state"
                            .to_string(),
                    ));
                }
                None
            }
            StateFileLockAcquireMode::TrustHeadInspection => {
                let journal = trust_journal
                    .as_ref()
                    .ok_or(EngineError::StateAnchorTrustHeadAbsent)?;
                let target_configuration = configured_anchor.as_ref().ok_or_else(|| {
                    EngineError::Internal(
                        "state-anchor trust-head inspection requires installed target pins"
                            .to_string(),
                    )
                })?;
                validate_state_anchor_trust_journal_stable_pins(
                    journal,
                    target_configuration,
                    &identity.fingerprint,
                )?;
                Some(
                    journal
                        .head()
                        .expect("stable-pin validation requires a head")
                        .to
                        .anchor_configuration()?,
                )
            }
            StateFileLockAcquireMode::TrustTransition(transition) => {
                if trust_journal
                    .as_ref()
                    .is_some_and(|journal| !journal.pending.is_empty())
                {
                    return Err(EngineError::Internal(
                        "state-anchor trust PREPARE records exist without a durable intent"
                            .to_string(),
                    ));
                }
                if let Some(head) = trust_journal
                    .as_ref()
                    .and_then(StateAnchorTrustJournalModel::head)
                {
                    Some(head.to.anchor_configuration()?)
                } else if let Some(from) = transition
                    .certificates
                    .first()
                    .and_then(|certificate| certificate.from.as_ref())
                {
                    Some(from.anchor_configuration()?)
                } else {
                    configured_anchor.clone()
                }
            }
        };
        let certified_floors = trust_journal
            .as_ref()
            .map(StateAnchorTrustJournalModel::certified_floors)
            .unwrap_or_default();
        let (mut anchor_file, mut anchor_identity, mut anchor_metadata, mut anchor_bytes) =
            open_state_anchor(
                &directory,
                &anchor_name,
                &identity.fingerprint,
                anchor_configuration.as_ref(),
                &certified_floors,
            )?;

        let promote_pending_anchor = recover_state_witness_rotation(
            &directory,
            StateWitnessRotationNames {
                current: &witness_name,
                next: &witness_next_name,
                previous: &witness_previous_name,
            },
            &identity,
            current_state_file.as_ref(),
            anchor_metadata.as_ref(),
            witness_max_records,
            true,
            None,
        )?;
        if promote_pending_anchor {
            let current = anchor_metadata.as_ref().ok_or_else(|| {
                EngineError::Internal(
                    "rotation recovery completed without state anchor metadata".to_string(),
                )
            })?;
            let pending = current.pending_witness_base.clone().ok_or_else(|| {
                EngineError::Internal(
                    "rotation recovery completed without a pending signed base".to_string(),
                )
            })?;
            let normalized = StateAnchorMetadata {
                latest: current.latest.clone(),
                witness_base: Some(pending),
                pending_witness_base: None,
            };
            let bytes = encode_state_anchor_metadata(&identity.fingerprint, &normalized);
            let configuration = anchor_configuration.as_ref().ok_or_else(|| {
                EngineError::Internal(
                    "rotation recovery cannot normalize anchor metadata without manifest pins"
                        .to_string(),
                )
            })?;
            parse_state_anchor_metadata(
                &bytes,
                &identity.fingerprint,
                configuration,
                &certified_floors,
            )?;
            let (file, entry_identity) =
                replace_state_anchor_entry(&directory, &anchor_name, &bytes)?;
            anchor_file = Some(file);
            anchor_identity = Some(entry_identity);
            anchor_metadata = Some(normalized);
            anchor_bytes = Some(bytes);
        }
        let opened_witness = open_or_create_state_witness(
            &directory,
            &witness_name,
            &identity,
            current_state_file.as_ref(),
            witness_max_records,
            anchor_metadata.as_ref(),
        )?;
        if let Some(head) = trust_journal
            .as_ref()
            .and_then(StateAnchorTrustJournalModel::head)
        {
            let anchor = anchor_metadata.as_ref().ok_or_else(|| {
                EngineError::Internal(
                    "committed state-anchor trust head requires persisted anchor metadata"
                        .to_string(),
                )
            })?;
            if opened_witness.parsed.segment_header.is_none() {
                return Err(EngineError::Internal(
                    "committed state-anchor trust head requires an authenticated witness segment"
                        .to_string(),
                ));
            }
            validate_state_anchor_trust_reference_descendant(
                &head.to.reference,
                &StateAnchorTrustReferenceModel::from_acknowledgement(&anchor.latest),
                "persisted state-anchor reference",
            )
            .map_err(|error| {
                EngineError::Internal(format!(
                    "persisted state anchor exceeds its certified restart window: {error}"
                ))
            })?;
        }
        // Persist a newly-created witness entry before exposing either the
        // static identity or dynamic state tip.
        directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory after witness initialization: {error}"
            ))
        })?;

        let mut store = Self {
            _file: lock_file,
            state_path: state_path.to_path_buf(),
            lock_path,
            directory,
            canonical_parent,
            state_name,
            lock_name,
            store_id_name,
            directory_identity,
            lock_identity,
            store_id_file,
            store_id_identity,
            trust_name,
            trust_file,
            trust_identity,
            trust_journal,
            trust_stamp,
            trust_bytes,
            trust_intent_name,
            trust_head_inspection: matches!(
                &mode,
                StateFileLockAcquireMode::TrustHeadInspection
                    | StateFileLockAcquireMode::TrustTransition(_)
            ),
            anchor_configuration: anchor_configuration.clone(),
            anchor_name,
            anchor_file,
            anchor_identity,
            anchor_metadata,
            anchor_bytes,
            witness_name,
            witness_next_name,
            witness_previous_name,
            witness_file: opened_witness.file,
            witness_identity: opened_witness.identity,
            witness_history: opened_witness.parsed.history,
            pending_witness: opened_witness.parsed.pending,
            witness_length: opened_witness.parsed.length,
            witness_max_records,
            witness_rotation_threshold: anchor_configuration
                .map(|configuration| configuration.rotation_threshold_records),
            witness_header_length: opened_witness.parsed.header_length,
            witness_header_bytes: opened_witness.parsed.header_bytes,
            witness_segment_header: opened_witness.parsed.segment_header,
            witness_prefix: None,
            last_appended_record: [0u8; TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH],
            last_appended_stamp: None,
            current_state_file,
            current_state_identity,
            identity,
            lock_held: true,
        };
        store.reconcile_pending_witness()?;
        // Startup loading must be able to inspect a malformed state image and
        // apply the explicit corruption policy. Validate every held descriptor
        // and the journal itself here; the state-image commitment is checked
        // separately and authenticated rollback evidence always fails closed.
        match mode {
            StateFileLockAcquireMode::Ordinary | StateFileLockAcquireMode::TrustHeadInspection => {
                store.revalidate_store_entries()?
            }
            StateFileLockAcquireMode::BootstrapFactsProvisioning => {
                store.revalidate_store_entries()?;
                store.validate_bootstrap_facts_pristine()?;
            }
            StateFileLockAcquireMode::TrustTransition(_) => {
                store.settle_pending_state_witness_rotation_inner(false)?;
                if store.pending_witness.is_some() {
                    return Err(EngineError::Internal(
                        "cannot transition state-anchor trust while a state transaction is pending"
                            .to_string(),
                    ));
                }
                let current_digest = current_state_image_digest(store.current_state_file.as_ref())?;
                if store
                    .witness_history
                    .last()
                    .is_none_or(|tip| tip.state_image_digest != current_digest)
                {
                    return Err(EngineError::Internal(
                        "state image does not match its witness before trust transition"
                            .to_string(),
                    ));
                }
            }
        }
        Ok(store)
    }

    #[cfg(not(unix))]
    pub(crate) fn acquire(state_path: &Path) -> Result<Self, EngineError> {
        Err(EngineError::Internal(format!(
            "descriptor-bound durable signer storage is unavailable on this platform for [{}]",
            state_path.display()
        )))
    }

    #[cfg(not(unix))]
    pub(crate) fn acquire_for_trust_transition(
        state_path: &Path,
        _transition: &VerifiedStateAnchorTrustTransition,
    ) -> Result<Self, EngineError> {
        Self::acquire(state_path)
    }

    #[cfg(not(unix))]
    pub(crate) fn acquire_for_trust_head_inspection(
        state_path: &Path,
    ) -> Result<Self, EngineError> {
        Self::acquire(state_path)
    }

    #[cfg(not(unix))]
    pub(crate) fn acquire_for_bootstrap_facts(state_path: &Path) -> Result<Self, EngineError> {
        Self::acquire(state_path)
    }

    pub(crate) fn identity(&mut self) -> Result<DurableStoreIdentity, EngineError> {
        self.reconcile_pending_witness()?;
        self.revalidate()?;
        Ok(self.identity.clone())
    }

    #[cfg(all(test, unix))]
    pub(crate) fn read_state(&mut self) -> Result<Option<Vec<u8>>, EngineError> {
        self.reconcile_pending_witness()?;
        self.revalidate()?;
        let Some(state_file) = self.current_state_file.as_ref() else {
            return Ok(None);
        };
        read_file_at(state_file, "signer state file").map(Some)
    }

    /// Reads the exact held state descriptor during startup after structural
    /// store validation, but before validating the image against the witness.
    /// This narrow path lets the loader distinguish malformed state, which may
    /// use the explicit corruption policy, from a valid-but-rolled-back image,
    /// which always fails closed.
    #[cfg(unix)]
    pub(crate) fn read_state_for_load(&mut self) -> Result<Option<Vec<u8>>, EngineError> {
        self.reconcile_pending_witness()?;
        self.settle_pending_state_witness_rotation()?;
        self.revalidate_store_entries()?;
        let Some(state_file) = self.current_state_file.as_ref() else {
            return Ok(None);
        };
        read_file_at(state_file, "signer state file").map(Some)
    }

    #[cfg(not(unix))]
    pub(crate) fn read_state_for_load(&mut self) -> Result<Option<Vec<u8>>, EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    #[cfg(all(test, not(unix)))]
    pub(crate) fn read_state(&mut self) -> Result<Option<Vec<u8>>, EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    /// Atomically replaces the encrypted state image through the held directory
    /// descriptor. `Ok(true)` means the destination was replaced even if a later
    /// durability check fails; callers preserve the existing retry semantics.
    #[cfg(unix)]
    pub(crate) fn replace_state(&mut self, bytes: &[u8]) -> Result<(), StoreReplaceError> {
        if let Err(error) = self.reconcile_pending_witness() {
            return Err(StoreReplaceError::before_replacement(error));
        }
        if let Err(error) = self.revalidate() {
            return Err(StoreReplaceError::before_replacement(error));
        }

        let temp_name = match unique_temp_name(&self.state_name) {
            Ok(name) => name,
            Err(error) => return Err(StoreReplaceError::before_replacement(error)),
        };
        let temp_file = match openat_regular(
            self.directory.as_raw_fd(),
            &temp_name,
            libc::O_RDWR | libc::O_CREAT | libc::O_EXCL,
            0o600,
            "signer state temp file",
        ) {
            Ok(file) => file,
            Err(error) => return Err(StoreReplaceError::before_replacement(error)),
        };

        let before_prepare_result =
            (|| -> Result<(OpenedObjectIdentity, fs::File), EngineError> {
                set_owner_only_permissions(&temp_file, "signer state temp file")?;
                validate_secure_regular_file(&temp_file, "signer state temp file")?;
                write_file_at(&temp_file, bytes, "signer state temp file")?;
                temp_file.sync_all().map_err(|error| {
                    EngineError::Internal(format!("failed to sync signer state temp file: {error}"))
                })?;
                maybe_inject_persist_fault(PersistFaultInjectionPoint::AfterTempSyncBeforeRename)?;
                self.revalidate()?;
                let identity = descriptor_identity(&temp_file, "new signer state file")?;
                let retained = temp_file.try_clone().map_err(|error| {
                    EngineError::Internal(format!(
                        "failed to retain new signer state descriptor: {error}"
                    ))
                })?;
                Ok((identity, retained))
            })();
        let (new_state_identity, new_state_file) = match before_prepare_result {
            Ok(result) => result,
            Err(error) => {
                let _ = unlinkat_entry(self.directory.as_raw_fd(), &temp_name);
                return Err(StoreReplaceError::before_replacement(error));
            }
        };

        let next_witness = match self.next_state_witness(state_image_digest(Some(bytes))) {
            Ok(witness) => witness,
            Err(error) => {
                let _ = unlinkat_entry(self.directory.as_raw_fd(), &temp_name);
                return Err(StoreReplaceError::before_replacement(error));
            }
        };
        if let Err(error) = self.prepare_witness(next_witness) {
            let _ = unlinkat_entry(self.directory.as_raw_fd(), &temp_name);
            return Err(StoreReplaceError::before_replacement(error));
        }

        if let Err(rename_error) = renameat_same_directory(
            self.directory.as_raw_fd(),
            &temp_name,
            &self.state_name,
            "replace signer state file",
        ) {
            let _ = unlinkat_entry(self.directory.as_raw_fd(), &temp_name);
            let error = match self.abort_pending_witness() {
                Ok(()) => rename_error,
                Err(abort_error) => EngineError::Internal(format!(
                    "{rename_error}; additionally failed to abort prepared state witness: {abort_error}"
                )),
            };
            return Err(StoreReplaceError::before_replacement(error));
        }

        // renameat preserves the prepared temp descriptor's identity. Publish
        // it immediately so every recovery path hashes the replacement image.
        self.current_state_identity = Some(new_state_identity);
        self.current_state_file = Some(new_state_file);

        let after_replacement_result = (|| -> Result<(), EngineError> {
            maybe_inject_persist_fault(PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync)?;
            self.directory.sync_all().map_err(|error| {
                EngineError::Internal(format!(
                    "failed to sync signer state directory [{}]: {error}",
                    self.canonical_parent.display()
                ))
            })?;
            self.commit_pending_witness()?;
            self.revalidate()?;
            Ok(())
        })();

        after_replacement_result.map_err(|error| StoreReplaceError {
            error,
            replaced: true,
        })
    }

    #[cfg(not(unix))]
    pub(crate) fn replace_state(&mut self, _bytes: &[u8]) -> Result<(), StoreReplaceError> {
        Err(StoreReplaceError::before_replacement(
            EngineError::Internal(
                "descriptor-bound durable signer storage is unavailable on this platform"
                    .to_string(),
            ),
        ))
    }

    #[cfg(unix)]
    pub(crate) fn quarantine_state(&mut self, backup_path: &Path) -> Result<(), EngineError> {
        self.reconcile_pending_witness()?;
        self.settle_pending_state_witness_rotation()?;
        // Quarantine is the sole operation allowed to proceed when the live
        // state bytes differ from the committed image, and only after the
        // caller has selected the explicit quarantine-and-reset policy. All
        // directory, anchor, inode, permission, and journal checks still hold.
        self.revalidate_store_entries()?;
        if self.current_state_identity.is_none() {
            return Err(EngineError::Internal(format!(
                "cannot quarantine absent signer state file [{}]",
                self.state_path.display()
            )));
        }
        let backup_parent = backup_path
            .parent()
            .filter(|parent| !parent.as_os_str().is_empty())
            .unwrap_or_else(|| Path::new("."));
        let canonical_backup_parent = fs::canonicalize(backup_parent).map_err(|error| {
            EngineError::Internal(format!(
                "failed to canonicalize signer state backup directory [{}]: {error}",
                backup_parent.display()
            ))
        })?;
        if canonical_backup_parent != self.canonical_parent {
            return Err(EngineError::Internal(format!(
                "refusing to quarantine signer state outside the opened store directory [{}]",
                self.canonical_parent.display()
            )));
        }
        let backup_name = backup_path.file_name().ok_or_else(|| {
            EngineError::Internal(format!(
                "signer state backup path [{}] has no file name",
                backup_path.display()
            ))
        })?;
        validate_entry_name(backup_name, "state backup")?;
        ensure_entry_absent(self.directory.as_raw_fd(), backup_name, "state backup")?;
        let next_witness = self.next_state_witness(state_image_digest(None))?;
        self.prepare_witness(next_witness)?;
        if let Err(rename_error) = renameat_same_directory(
            self.directory.as_raw_fd(),
            &self.state_name,
            backup_name,
            "quarantine signer state file",
        ) {
            let error = match self.abort_pending_witness() {
                Ok(()) => rename_error,
                Err(abort_error) => EngineError::Internal(format!(
                    "{rename_error}; additionally failed to abort prepared state witness: {abort_error}"
                )),
            };
            return Err(error);
        }
        self.current_state_file = None;
        self.current_state_identity = None;
        self.directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory after quarantine: {error}"
            ))
        })?;
        self.commit_pending_witness()?;
        self.revalidate()?;
        Ok(())
    }

    #[cfg(not(unix))]
    pub(crate) fn quarantine_state(&mut self, _backup_path: &Path) -> Result<(), EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    #[cfg(unix)]
    pub(crate) fn revalidate_store_entries(&mut self) -> Result<(), EngineError> {
        if !self.lock_held {
            return Err(EngineError::Internal(
                "signer durable store exclusive lock is no longer held".to_string(),
            ));
        }

        let live_directory = open_absolute_directory_nofollow(&self.canonical_parent)?;
        let live_directory_identity =
            descriptor_identity(&live_directory, "live signer state directory")?;
        if live_directory_identity != self.directory_identity {
            return Err(replacement_error("signer state directory"));
        }

        validate_live_entry(
            &self.directory,
            &self.lock_name,
            self.lock_identity,
            "signer state lock file",
        )?;
        validate_live_entry(
            &self.directory,
            &self.store_id_name,
            self.store_id_identity,
            "signer durable store ID file",
        )?;
        validate_secure_regular_file(&self._file, "signer state lock file")?;
        validate_secure_regular_file(&self.store_id_file, "signer durable store ID file")?;
        let live_store_id = read_store_id(&self.store_id_file)?;
        if live_store_id != self.identity.store_id {
            return Err(EngineError::Internal(
                "signer durable store ID changed after the store was opened".to_string(),
            ));
        }

        match (
            self.trust_identity,
            self.trust_file.as_ref(),
            self.trust_journal.as_ref(),
            self.trust_stamp,
            self.trust_bytes.as_ref(),
        ) {
            (
                Some(identity),
                Some(file),
                Some(journal),
                Some(expected_stamp),
                Some(expected_bytes),
            ) => {
                validate_live_entry(
                    &self.directory,
                    &self.trust_name,
                    identity,
                    "state-anchor trust journal",
                )?;
                validate_secure_regular_file(file, "state-anchor trust journal")?;
                if file_change_stamp(file, "state-anchor trust journal")? != expected_stamp {
                    return Err(EngineError::Internal(
                        "state-anchor trust journal changed after startup verification".to_string(),
                    ));
                }
                if usize::try_from(
                    file.metadata()
                        .map_err(|error| {
                            EngineError::Internal(format!(
                                "failed to stat state-anchor trust journal: {error}"
                            ))
                        })?
                        .len(),
                )
                .ok()
                    != Some(expected_bytes.len())
                {
                    return Err(EngineError::Internal(
                        "state-anchor trust journal length changed after startup verification"
                            .to_string(),
                    ));
                }
                let configuration = configured_state_anchor()?.ok_or_else(|| {
                    EngineError::Internal(
                        "state-anchor trust journal requires configured anchor pins".to_string(),
                    )
                })?;
                if self.trust_head_inspection {
                    validate_state_anchor_trust_journal_stable_pins(
                        journal,
                        &configuration,
                        &self.identity.fingerprint,
                    )?;
                } else {
                    validate_state_anchor_trust_journal_head(
                        journal,
                        &configuration,
                        &self.identity.fingerprint,
                    )?;
                }
            }
            (None, None, None, None, None) => ensure_entry_absent(
                self.directory.as_raw_fd(),
                &self.trust_name,
                "state-anchor trust journal",
            )?,
            _ => {
                return Err(EngineError::Internal(
                    "state-anchor trust journal descriptor invariant is inconsistent".to_string(),
                ))
            }
        }
        ensure_entry_absent(
            self.directory.as_raw_fd(),
            &self.trust_intent_name,
            "state-anchor trust transition intent",
        )?;

        match (
            self.anchor_identity,
            self.anchor_file.as_ref(),
            self.anchor_metadata.as_ref(),
            self.anchor_bytes.as_ref(),
        ) {
            (Some(identity), Some(file), Some(metadata), Some(expected_bytes)) => {
                validate_live_entry(
                    &self.directory,
                    &self.anchor_name,
                    identity,
                    "signer state anchor metadata",
                )?;
                validate_secure_regular_file(file, "signer state anchor metadata")?;
                let live_bytes = read_file_at(file, "signer state anchor metadata")?;
                if &live_bytes != expected_bytes {
                    return Err(EngineError::Internal(
                        "signer state anchor metadata changed outside the locked store".to_string(),
                    ));
                }
                let configuration = self.anchor_configuration.as_ref().ok_or_else(|| {
                    EngineError::Internal(
                        "persisted signer state anchor metadata requires manifest pins".to_string(),
                    )
                })?;
                let parsed = parse_state_anchor_metadata(
                    &live_bytes,
                    &self.identity.fingerprint,
                    configuration,
                    &self
                        .trust_journal
                        .as_ref()
                        .map(StateAnchorTrustJournalModel::certified_floors)
                        .unwrap_or_default(),
                )?;
                if &parsed != metadata {
                    return Err(EngineError::Internal(
                        "signer state anchor metadata no longer matches its verified model"
                            .to_string(),
                    ));
                }
                if let Some(head) = self
                    .trust_journal
                    .as_ref()
                    .and_then(StateAnchorTrustJournalModel::head)
                {
                    validate_state_anchor_trust_reference_descendant(
                        &head.to.reference,
                        &StateAnchorTrustReferenceModel::from_acknowledgement(&parsed.latest),
                        "persisted state-anchor reference",
                    )
                    .map_err(|error| {
                        EngineError::Internal(format!(
                            "persisted state anchor exceeds its certified restart window: {error}"
                        ))
                    })?;
                }
            }
            (None, None, None, None) => {
                if self
                    .trust_journal
                    .as_ref()
                    .and_then(StateAnchorTrustJournalModel::head)
                    .is_some()
                {
                    return Err(EngineError::Internal(
                        "committed state-anchor trust head requires persisted anchor metadata"
                            .to_string(),
                    ));
                }
                ensure_entry_absent(
                    self.directory.as_raw_fd(),
                    &self.anchor_name,
                    "signer state anchor metadata",
                )?
            }
            _ => {
                return Err(EngineError::Internal(
                    "signer state anchor descriptor invariant is inconsistent".to_string(),
                ))
            }
        }
        ensure_entry_absent(
            self.directory.as_raw_fd(),
            &self.witness_next_name,
            "next signer state witness journal",
        )?;
        ensure_entry_absent(
            self.directory.as_raw_fd(),
            &self.witness_previous_name,
            "previous signer state witness journal",
        )?;

        match (
            self.current_state_identity,
            self.current_state_file.as_ref(),
        ) {
            (Some(identity), Some(file)) => {
                validate_live_entry(
                    &self.directory,
                    &self.state_name,
                    identity,
                    "signer state file",
                )?;
                validate_secure_regular_file(file, "signer state file")?;
            }
            (None, None) => ensure_entry_absent(
                self.directory.as_raw_fd(),
                &self.state_name,
                "signer state file",
            )?,
            _ => {
                return Err(EngineError::Internal(
                    "signer durable store state descriptor invariant is inconsistent".to_string(),
                ))
            }
        }

        validate_live_entry(
            &self.directory,
            &self.witness_name,
            self.witness_identity,
            "signer state witness journal",
        )?;
        validate_secure_regular_file(&self.witness_file, "signer state witness journal")?;
        self.verify_state_witness_journal()?;
        if self
            .trust_journal
            .as_ref()
            .and_then(StateAnchorTrustJournalModel::head)
            .is_some()
            && self.witness_segment_header.is_none()
        {
            return Err(EngineError::Internal(
                "committed state-anchor trust head requires an authenticated witness segment"
                    .to_string(),
            ));
        }
        Ok(())
    }

    /// Verifies the journal against the in-memory history.
    ///
    /// The journal is append-only and is written only by this process while the
    /// exclusive lock is held, so re-reading and re-hashing every record ever
    /// written on every access is pure waste that grows without bound in
    /// lifetime persist count. Instead the verified prefix is cached and the
    /// O(1) anchor - file identity, change stamp, header, trailing record, and
    /// the last verified generation/commitment - is re-checked. ANY mismatch,
    /// including a file whose identity moved underneath, falls through to a
    /// full re-parse, which is what produces the precise failure. Bytes
    /// appended since the last verification are read back and checked at append
    /// time, so no byte is ever trusted without having been read from disk.
    ///
    /// The cache is per-`StateFileLock`, so a tampered prefix is still caught
    /// in full on any fresh open.
    #[cfg(unix)]
    fn verify_state_witness_journal(&mut self) -> Result<(), EngineError> {
        let stamp = witness_change_stamp(&self.witness_file)?;
        if let Some(prefix) = self.witness_prefix.as_ref() {
            let tip = self
                .witness_history
                .last()
                .map(|tip| (tip.generation, tip.commitment));
            if prefix.identity == self.witness_identity
                && prefix.stamp == stamp
                && prefix.verified_length == self.witness_length
                && prefix.history_length == self.witness_history.len()
                && tip == Some((prefix.tip_generation, prefix.tip_commitment))
                && self.witness_anchor_matches(prefix)?
            {
                #[cfg(test)]
                WITNESS_INCREMENTAL_VERIFICATIONS.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                return Ok(());
            }
        }
        self.verify_state_witness_journal_fully()
    }

    /// Re-reads the two fixed anchors of the cached prefix: the header, which
    /// binds this store's ID, and the trailing record. Returns `false` - never
    /// an error - when either differs, so the caller falls back to the full
    /// parse that reports the real problem.
    #[cfg(unix)]
    fn witness_anchor_matches(&self, prefix: &WitnessJournalPrefix) -> Result<bool, EngineError> {
        const LABEL: &str = "signer state witness journal";
        if prefix.verified_length
            < self.witness_header_length + TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH
        {
            return Ok(false);
        }
        // Take stamps around the anchor reads. A writer that changes the file
        // between the caller's initial stat and these reads must not be
        // admitted merely because it restores the same length.
        let before = witness_change_stamp(&self.witness_file)?;
        if before != prefix.stamp {
            return Ok(false);
        }
        let header = read_file_range_at(&self.witness_file, 0, self.witness_header_length, LABEL)?;
        if header != self.witness_header_bytes {
            return Ok(false);
        }
        let tail = read_file_range_at(
            &self.witness_file,
            prefix.verified_length - TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH,
            TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH,
            LABEL,
        )?;
        let after = witness_change_stamp(&self.witness_file)?;
        #[cfg(test)]
        WITNESS_VERIFIED_BYTES_READ.fetch_add(
            (header.len() + tail.len()) as u64,
            std::sync::atomic::Ordering::SeqCst,
        );
        Ok(before == after && after == prefix.stamp && tail == prefix.tail_record)
    }

    #[cfg(unix)]
    fn verify_state_witness_journal_fully(&mut self) -> Result<(), EngineError> {
        let anchor = self.anchor_metadata.clone();
        self.verify_state_witness_journal_fully_with_anchor(anchor.as_ref())
    }

    #[cfg(unix)]
    fn verify_state_witness_journal_fully_with_anchor(
        &mut self,
        validation_anchor: Option<&StateAnchorMetadata>,
    ) -> Result<(), EngineError> {
        #[cfg(test)]
        WITNESS_FULL_VERIFICATIONS.fetch_add(1, std::sync::atomic::Ordering::SeqCst);

        let before = witness_change_stamp(&self.witness_file)?;
        let parsed = read_state_witness_journal_streaming(
            &self.witness_file,
            &self.identity.store_id,
            &self.identity.fingerprint,
            self.witness_max_records,
            validation_anchor,
        )?;
        if parsed.length != self.witness_length
            || parsed.header_length != self.witness_header_length
            || parsed.header_bytes != self.witness_header_bytes
            || parsed.segment_header != self.witness_segment_header
        {
            return Err(EngineError::Internal(
                "signer state witness journal length changed outside the locked store".to_string(),
            ));
        }
        if parsed.pending != self.pending_witness || parsed.history != self.witness_history {
            return Err(EngineError::Internal(
                "signer state witness journal changed outside the locked store".to_string(),
            ));
        }

        // Only cache a prefix whose bytes provably did not move while they were
        // being read.
        let after = witness_change_stamp(&self.witness_file)?;
        if before != after {
            self.witness_prefix = None;
            return Err(EngineError::Internal(
                "signer state witness journal changed during full verification".to_string(),
            ));
        }
        self.witness_prefix = self.build_witness_prefix(after, &parsed.tail_record);
        Ok(())
    }

    /// Builds the cached prefix from the current in-memory model. Returns
    /// `None` when there is nothing to anchor to, which simply disables the
    /// incremental path.
    #[cfg(unix)]
    fn build_witness_prefix(
        &self,
        stamp: FileChangeStamp,
        tail_record: &[u8],
    ) -> Option<WitnessJournalPrefix> {
        let tip = self.witness_history.last()?;
        if tail_record.len() != TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH
            || self.witness_length
                < self.witness_header_length + TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH
        {
            return None;
        }
        let mut tail = [0u8; TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH];
        tail.copy_from_slice(tail_record);
        Some(WitnessJournalPrefix {
            identity: self.witness_identity,
            stamp,
            verified_length: self.witness_length,
            history_length: self.witness_history.len(),
            tip_generation: tip.generation,
            tip_commitment: tip.commitment,
            tail_record: tail,
        })
    }

    /// Reads back the record that was just appended and extends the verified
    /// prefix over it. This is the "verify only the bytes appended since"
    /// half of the incremental scheme: every journal byte is still read from
    /// disk and checked exactly once.
    #[cfg(unix)]
    fn extend_witness_prefix(&mut self) -> Result<(), EngineError> {
        const LABEL: &str = "signer state witness journal";
        let appended_stamp = self.last_appended_stamp.take();
        let Some(previous) = self.witness_prefix.clone() else {
            // Nothing verified yet; the next access parses the whole journal.
            return Ok(());
        };
        let Some(appended_stamp) = appended_stamp else {
            self.witness_prefix = None;
            return Err(EngineError::Internal(
                "signer state witness append has no post-sync change stamp".to_string(),
            ));
        };
        let appended_offset = previous.verified_length;
        if appended_offset.checked_add(TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH)
            != Some(self.witness_length)
        {
            self.witness_prefix = None;
            return Err(EngineError::Internal(
                "signer state witness journal length did not advance by exactly one record"
                    .to_string(),
            ));
        }
        let before = witness_change_stamp(&self.witness_file)?;
        if before != appended_stamp || before.size != self.witness_length as u64 {
            self.witness_prefix = None;
            return Err(EngineError::Internal(
                "signer state witness journal changed after the append was synced".to_string(),
            ));
        }
        let header = read_file_range_at(&self.witness_file, 0, self.witness_header_length, LABEL)?;
        let old_tail = read_file_range_at(
            &self.witness_file,
            previous.verified_length - TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH,
            TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH,
            LABEL,
        )?;
        let appended = read_file_range_at(
            &self.witness_file,
            appended_offset,
            TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH,
            LABEL,
        )?;
        #[cfg(test)]
        WITNESS_VERIFIED_BYTES_READ.fetch_add(
            (header.len() + old_tail.len() + appended.len()) as u64,
            std::sync::atomic::Ordering::SeqCst,
        );
        let header_matches = header == self.witness_header_bytes;
        if !header_matches
            || old_tail != previous.tail_record
            || appended != self.last_appended_record
        {
            self.witness_prefix = None;
            return Err(EngineError::Internal(
                "signer state witness journal prefix or append read-back changed during append"
                    .to_string(),
            ));
        }
        let after = witness_change_stamp(&self.witness_file)?;
        if before != after {
            self.witness_prefix = None;
            return Err(EngineError::Internal(
                "signer state witness journal changed during append read-back".to_string(),
            ));
        }
        self.witness_prefix = self.build_witness_prefix(after, &self.last_appended_record);
        Ok(())
    }

    #[cfg(unix)]
    pub(crate) fn validate_state_image(&mut self) -> Result<(), EngineError> {
        self.settle_pending_state_witness_rotation()?;
        self.revalidate_store_entries()?;
        let current_digest = current_state_image_digest(self.current_state_file.as_ref())?;
        let history = &self.witness_history;
        if history
            .last()
            .map(|tip| tip.state_image_digest != current_digest)
            .unwrap_or(true)
        {
            return Err(EngineError::Internal(
                "signer state image does not match the committed witness tip".to_string(),
            ));
        }

        Ok(())
    }

    #[cfg(unix)]
    fn revalidate(&mut self) -> Result<(), EngineError> {
        self.validate_state_image()
    }

    #[cfg(not(unix))]
    pub(crate) fn revalidate_store_entries(&mut self) -> Result<(), EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    #[cfg(not(unix))]
    pub(crate) fn validate_state_image(&mut self) -> Result<(), EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    #[cfg(not(unix))]
    fn revalidate(&mut self) -> Result<(), EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    pub(crate) fn state_witness_tip(&mut self) -> Result<StateWitness, EngineError> {
        self.reconcile_pending_witness()?;
        self.revalidate()?;
        self.witness_history.last().cloned().ok_or_else(|| {
            EngineError::Internal("signer state witness journal has no committed tip".to_string())
        })
    }

    pub(crate) fn state_witness_tip_snapshot(
        &mut self,
    ) -> Result<StateWitnessTipSnapshot, EngineError> {
        self.reconcile_pending_witness()?;
        self.revalidate()?;
        self.normalize_published_pending_anchor()?;
        let tip = self.witness_history.last().cloned().ok_or_else(|| {
            EngineError::Internal("signer state witness journal has no committed tip".to_string())
        })?;
        let base = self.witness_history.first().cloned().ok_or_else(|| {
            EngineError::Internal("signer state witness journal has no retained base".to_string())
        })?;
        Ok(StateWitnessTipSnapshot {
            store_fingerprint: self.identity.fingerprint,
            tip,
            base,
            anchor: self.anchor_metadata.clone(),
        })
    }

    pub(crate) fn state_anchor_trust_head_snapshot(
        &mut self,
    ) -> Result<StateAnchorTrustTransitionStoreOutcome, EngineError> {
        self.reconcile_pending_witness()?;
        self.revalidate()?;
        self.normalize_published_pending_anchor()?;
        self.state_anchor_trust_transition_outcome(true, 0)
    }

    pub(crate) fn state_anchor_bootstrap_facts_snapshot(
        &mut self,
    ) -> Result<([u8; 32], StateWitness), EngineError> {
        self.revalidate()?;
        self.validate_bootstrap_facts_pristine()?;
        let tip = self.witness_history.last().cloned().ok_or_else(|| {
            EngineError::Internal(
                "bootstrap provisioning witness has no committed genesis".to_string(),
            )
        })?;
        Ok((self.identity.fingerprint, tip))
    }

    fn validate_bootstrap_facts_pristine(&self) -> Result<(), EngineError> {
        let exact_genesis_length =
            TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH + 2 * TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH;
        let pristine = self.trust_journal.is_none()
            && self.trust_bytes.is_none()
            && self.anchor_metadata.is_none()
            && self.anchor_bytes.is_none()
            && self.witness_segment_header.is_none()
            && self.pending_witness.is_none()
            && self.current_state_file.is_none()
            && self.current_state_identity.is_none()
            && self.witness_history.len() == 1
            && self.witness_history[0].generation == 1
            && self.witness_length == exact_genesis_length;
        if !pristine {
            return Err(EngineError::Validation(
                "state-anchor bootstrap facts require a pristine genesis-only store".to_string(),
            ));
        }
        Ok(())
    }

    #[cfg(unix)]
    pub(crate) fn transition_state_witness_anchor(
        &mut self,
        transition: &VerifiedStateAnchorTrustTransition,
    ) -> Result<StateAnchorTrustTransitionStoreOutcome, EngineError> {
        // In transition mode the rotating endpoint may still be the prior
        // head, but immutable trust pins and every live descriptor must be
        // revalidated before the first durable mutation.
        self.revalidate()?;
        let idempotent = self.validate_state_anchor_trust_transition_local(transition)?;
        recheck_state_anchor_admission_expiry(transition.target_read_expires_at_unix_ms)?;
        if idempotent {
            // Trust-transition acquisition deliberately permits the prior head
            // while selecting a missing suffix. An exact replay is already at
            // the installed target, so it must perform the full steady-state
            // descriptor and state-image validation before returning claims
            // from held descriptors.
            self.trust_head_inspection = false;
            self.revalidate()?;
            self.normalize_published_pending_anchor()?;
            return self.state_anchor_trust_transition_outcome(true, 0);
        }

        let journal_growth = self.state_anchor_trust_transition_journal_growth(transition)?;
        self.ensure_state_anchor_trust_transition_journal_capacity(journal_growth)?;
        let intent_bytes = encode_state_anchor_trust_transition_intent(
            &self.identity.fingerprint,
            &transition.request,
        )?;
        self.revalidate_trust_transition_mutation_guard()?;
        self.create_state_anchor_trust_transition_intent(
            &intent_bytes,
            transition.target_read_expires_at_unix_ms,
            journal_growth,
        )?;
        #[cfg(test)]
        maybe_inject_state_anchor_trust_transition_fault(
            StateAnchorTrustTransitionFaultInjectionPoint::AfterIntentPublication,
        )?;
        self.revalidate_state_anchor_trust_intent(&intent_bytes)?;

        self.ensure_state_anchor_trust_journal()?;
        for certificate in &transition.certificates {
            self.revalidate_state_anchor_trust_intent(&intent_bytes)?;
            self.revalidate_trust_transition_mutation_guard()?;
            let previous = self
                .trust_journal
                .as_ref()
                .ok_or_else(|| {
                    EngineError::Internal(
                        "state-anchor trust journal disappeared during PREPARE".to_string(),
                    )
                })?
                .last_record_commitment;
            let record = encode_state_anchor_trust_prepare_record(
                &self.identity.fingerprint,
                &previous,
                certificate,
            )?;
            self.publish_state_anchor_trust_journal_record(&record)?;
        }
        #[cfg(test)]
        maybe_inject_state_anchor_trust_transition_fault(
            StateAnchorTrustTransitionFaultInjectionPoint::AfterPrepareBatch,
        )?;

        let final_certificate = transition
            .certificates
            .last()
            .expect("verified transition chain is nonempty");
        let target_acknowledgement = final_certificate.target_acknowledgement.clone();
        let target_anchor = StateAnchorMetadata {
            latest: target_acknowledgement.clone(),
            witness_base: Some(target_acknowledgement.clone()),
            pending_witness_base: None,
        };
        self.revalidate_state_anchor_trust_intent(&intent_bytes)?;
        self.revalidate_trust_transition_mutation_guard()?;
        self.rotate_state_witness_segment_for_trust_transition(
            &target_acknowledgement,
            &target_anchor,
        )?;

        self.anchor_configuration = Some(final_certificate.to.anchor_configuration()?);
        self.revalidate_state_anchor_trust_intent(&intent_bytes)?;
        self.revalidate_trust_transition_mutation_guard()?;
        self.persist_state_anchor_metadata(target_anchor)?;
        #[cfg(test)]
        maybe_inject_state_anchor_trust_transition_fault(
            StateAnchorTrustTransitionFaultInjectionPoint::AfterTargetAnchorPublication,
        )?;

        for certificate in &transition.certificates {
            self.revalidate_state_anchor_trust_intent(&intent_bytes)?;
            self.revalidate_trust_transition_mutation_guard()?;
            let previous = self
                .trust_journal
                .as_ref()
                .ok_or_else(|| {
                    EngineError::Internal(
                        "state-anchor trust journal disappeared before COMMIT".to_string(),
                    )
                })?
                .last_record_commitment;
            let record = encode_state_anchor_trust_commit_record(
                &self.identity.fingerprint,
                &previous,
                certificate,
            )?;
            self.publish_state_anchor_trust_journal_record(&record)?;
            #[cfg(test)]
            maybe_inject_state_anchor_trust_transition_fault(
                StateAnchorTrustTransitionFaultInjectionPoint::AfterCommitPublication,
            )?;
        }

        let configured = configured_state_anchor()?.ok_or_else(|| {
            EngineError::Internal(
                "state-anchor config disappeared during trust transition".to_string(),
            )
        })?;
        validate_state_anchor_trust_journal_head(
            self.trust_journal.as_ref().ok_or_else(|| {
                EngineError::Internal(
                    "state-anchor trust journal is absent after COMMIT".to_string(),
                )
            })?,
            &configured,
            &self.identity.fingerprint,
        )?;

        self.revalidate_state_anchor_trust_intent(&intent_bytes)?;
        self.revalidate_trust_transition_mutation_guard()?;
        unlinkat_entry(self.directory.as_raw_fd(), &self.witness_previous_name)?;
        self.directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync trust transition after retiring previous witness: {error}"
            ))
        })?;
        #[cfg(test)]
        maybe_inject_state_anchor_trust_transition_fault(
            StateAnchorTrustTransitionFaultInjectionPoint::AfterPreviousWitnessRetirement,
        )?;
        self.revalidate_trust_transition_mutation_guard()?;
        unlinkat_entry(self.directory.as_raw_fd(), &self.trust_intent_name)?;
        self.directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync trust transition intent removal: {error}"
            ))
        })?;
        self.state_anchor_trust_mutation_guard().revalidate()?;

        self.trust_head_inspection = false;
        self.revalidate_store_entries()?;
        self.state_anchor_trust_transition_outcome(false, transition.certificates.len())
    }

    #[cfg(unix)]
    fn validate_state_anchor_trust_transition_local(
        &self,
        transition: &VerifiedStateAnchorTrustTransition,
    ) -> Result<bool, EngineError> {
        let first = transition
            .certificates
            .first()
            .expect("verified transition chain is nonempty");
        let final_certificate = transition
            .certificates
            .last()
            .expect("verified transition chain is nonempty");
        if transition.certificates.iter().any(|certificate| {
            certificate.signer_store_fingerprint != self.identity.fingerprint
                || certificate.to.reference.checkpoint.store_fingerprint
                    != self.identity.fingerprint
        }) {
            return Err(EngineError::Validation(
                "state-anchor trust certificate targets a different durable signer store"
                    .to_string(),
            ));
        }
        let tip = self.witness_history.last().ok_or_else(|| {
            EngineError::Internal("state witness has no committed tip".to_string())
        })?;
        let local_checkpoint =
            StateAnchorTrustCheckpointModel::from_witness(self.identity.fingerprint, tip);

        if let Some(head) = self
            .trust_journal
            .as_ref()
            .and_then(StateAnchorTrustJournalModel::head)
        {
            if first.certificate_sequence == head.certificate_sequence {
                let local_anchor = self.anchor_metadata.as_ref().ok_or_else(|| {
                    EngineError::Internal(
                        "state-anchor trust head exists without local anchor metadata".to_string(),
                    )
                })?;
                if transition.certificates.len() != 1
                    || first.certificate_digest != head.certificate_digest
                    || first.wire != head.wire
                    || StateAnchorTrustReferenceModel::from_acknowledgement(&local_anchor.latest)
                        != StateAnchorTrustReferenceModel::from_acknowledgement(
                            &transition.target_read_acknowledgement,
                        )
                {
                    return Err(EngineError::Validation(
                        "completed trust transition accepts only a one-certificate exact replay \
                         plus a fresh Read of the current local descendant"
                            .to_string(),
                    ));
                }
                let local_reference =
                    StateAnchorTrustReferenceModel::from_acknowledgement(&local_anchor.latest);
                validate_state_anchor_trust_reference_descendant(
                    &head.to.reference,
                    &local_reference,
                    "completed trust transition local anchor",
                )?;
                if local_reference.checkpoint != local_checkpoint {
                    return Err(EngineError::Validation(
                        "completed trust transition replay requires the current signed anchor \
                         checkpoint to exactly match the local witness tip"
                            .to_string(),
                    ));
                }
                return Ok(true);
            }
            let expected_sequence = head.certificate_sequence.checked_add(1).ok_or_else(|| {
                EngineError::Validation(
                    "state-anchor trust certificate sequence overflows u64".to_string(),
                )
            })?;
            let local_anchor = self.anchor_metadata.as_ref().ok_or_else(|| {
                EngineError::Internal(
                    "state-anchor trust head exists without local anchor metadata".to_string(),
                )
            })?;
            let local_reference =
                StateAnchorTrustReferenceModel::from_acknowledgement(&local_anchor.latest);
            validate_state_anchor_trust_reference_descendant(
                &head.to.reference,
                &local_reference,
                "persisted local anchor",
            )?;
            let mut expected_from = head.to.clone();
            expected_from.reference = local_reference;
            if first.certificate_sequence != expected_sequence
                || first.previous_certificate_digest != head.certificate_digest
                || first.from.as_ref() != Some(&expected_from)
            {
                return Err(EngineError::Validation(
                    "first missing trust certificate does not extend the exact local trust and \
                     anchor head"
                        .to_string(),
                ));
            }
        } else if let Some(local_anchor) = self.anchor_metadata.as_ref() {
            let from = first.from.as_ref().ok_or_else(|| {
                EngineError::Validation(
                    "legacy anchored adoption requires a rotation certificate".to_string(),
                )
            })?;
            if first.kind != StateAnchorTrustCertificateKind::Rotation
                || first.certificate_sequence != 1
                || first.previous_certificate_digest != [0u8; 32]
                || from.binding_hash != local_anchor.latest.binding_hash
                || from.response_public_key
                    != self
                        .anchor_configuration
                        .as_ref()
                        .map(|configuration| configuration.response_public_key)
                        .unwrap_or([0u8; 32])
                || !from.reference.matches_acknowledgement(&local_anchor.latest)
            {
                return Err(EngineError::Validation(
                    "legacy adoption certificate does not exactly authenticate the persisted \
                     anchor"
                        .to_string(),
                ));
            }
        } else if first.kind != StateAnchorTrustCertificateKind::Bootstrap
            || first.certificate_sequence != 1
            || self.witness_segment_header.is_some()
        {
            return Err(EngineError::Validation(
                "unanchored store requires a sequence-1 bootstrap certificate and clean \
                 unsegmented witness"
                    .to_string(),
            ));
        }

        if final_certificate.to.reference.checkpoint != local_checkpoint {
            return Err(EngineError::Validation(
                "final trust certificate checkpoint does not exactly match the local witness tip"
                    .to_string(),
            ));
        }
        for certificate in &transition.certificates {
            if let Some(from) = certificate.from.as_ref() {
                self.validate_trust_checkpoint_against_retained_witness(
                    &from.reference.checkpoint,
                    "certificate from.reference checkpoint",
                )?;
            }
            self.validate_trust_checkpoint_against_retained_witness(
                &certificate.to.reference.checkpoint,
                "certificate to.reference checkpoint",
            )?;
        }
        if transition.target_read_acknowledgement_bytes
            != final_certificate.target_acknowledgement_bytes
            || transition.target_read_acknowledgement != final_certificate.target_acknowledgement
        {
            return Err(EngineError::Validation(
                "fresh final Read does not contain the exact final certificate acknowledgement"
                    .to_string(),
            ));
        }
        Ok(false)
    }

    fn validate_trust_checkpoint_against_retained_witness(
        &self,
        checkpoint: &StateAnchorTrustCheckpointModel,
        label: &str,
    ) -> Result<(), EngineError> {
        if checkpoint.store_fingerprint != self.identity.fingerprint {
            return Err(EngineError::Validation(format!(
                "{label} targets a different durable signer store"
            )));
        }
        let base = self.witness_history.first().ok_or_else(|| {
            EngineError::Internal(
                "state witness has no retained base during trust transition".to_string(),
            )
        })?;
        let tip = self.witness_history.last().ok_or_else(|| {
            EngineError::Internal(
                "state witness has no committed tip during trust transition".to_string(),
            )
        })?;
        if checkpoint.generation > tip.generation {
            return Err(EngineError::Validation(format!(
                "{label} is ahead of the local witness tip"
            )));
        }
        if checkpoint.generation < base.generation {
            // The offline authority authenticates this historical checkpoint;
            // a compacted local segment cannot prove data older than its signed
            // retained base.
            return Ok(());
        }
        let index = usize::try_from(checkpoint.generation - base.generation).map_err(|_| {
            EngineError::Validation(format!("{label} index does not fit this platform"))
        })?;
        let witness = self.witness_history.get(index).ok_or_else(|| {
            EngineError::Validation(format!("{label} is missing from retained witness history"))
        })?;
        if StateAnchorTrustCheckpointModel::from_witness(self.identity.fingerprint, witness)
            != *checkpoint
        {
            return Err(EngineError::Validation(format!(
                "{label} disagrees with retained witness history"
            )));
        }
        Ok(())
    }

    #[cfg(unix)]
    fn state_anchor_trust_transition_outcome(
        &self,
        idempotent: bool,
        applied_certificate_count: usize,
    ) -> Result<StateAnchorTrustTransitionStoreOutcome, EngineError> {
        let trust_head = self
            .trust_journal
            .as_ref()
            .and_then(StateAnchorTrustJournalModel::head)
            .cloned()
            .ok_or_else(|| {
                EngineError::Internal(
                    "state-anchor trust transition completed without a trust head".to_string(),
                )
            })?;
        let tip = self.witness_history.last().cloned().ok_or_else(|| {
            EngineError::Internal("state witness has no committed tip".to_string())
        })?;
        let base = self.witness_history.first().cloned().ok_or_else(|| {
            EngineError::Internal("state witness has no retained base".to_string())
        })?;
        let anchor = self.anchor_metadata.clone().ok_or_else(|| {
            EngineError::Internal(
                "state-anchor trust transition completed without anchor metadata".to_string(),
            )
        })?;
        Ok(StateAnchorTrustTransitionStoreOutcome {
            idempotent,
            applied_certificate_count,
            trust_head,
            tip,
            base,
            anchor,
        })
    }

    #[cfg(unix)]
    fn ensure_state_anchor_trust_journal(&mut self) -> Result<(), EngineError> {
        if self.trust_journal.is_some() {
            return Ok(());
        }
        let bytes = encode_state_anchor_trust_journal_header(&self.identity.fingerprint);
        self.publish_state_anchor_trust_journal(&bytes)
    }

    #[cfg(unix)]
    fn publish_state_anchor_trust_journal_record(
        &mut self,
        record: &[u8],
    ) -> Result<(), EngineError> {
        let mut bytes = self.trust_bytes.clone().ok_or_else(|| {
            EngineError::Internal(
                "state-anchor trust journal bytes are unavailable during transition".to_string(),
            )
        })?;
        if bytes
            .len()
            .checked_add(record.len())
            .is_none_or(|length| length > STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH)
        {
            return Err(EngineError::Validation(
                "state-anchor trust journal reached its durable size bound".to_string(),
            ));
        }
        bytes.extend_from_slice(record);
        self.publish_state_anchor_trust_journal(&bytes)
    }

    #[cfg(unix)]
    fn publish_state_anchor_trust_journal(&mut self, bytes: &[u8]) -> Result<(), EngineError> {
        let parsed = parse_state_anchor_trust_journal(bytes, &self.identity.fingerprint)?;
        let (file, identity) = if self.trust_head_inspection {
            let guard = self.state_anchor_trust_mutation_guard();
            replace_durable_entry_with_guard(
                &self.directory,
                &self.trust_name,
                bytes,
                "state-anchor trust journal",
                Some(&guard),
            )?
        } else {
            replace_durable_entry(
                &self.directory,
                &self.trust_name,
                bytes,
                "state-anchor trust journal",
            )?
        };
        let before = file_change_stamp(&file, "state-anchor trust journal")?;
        let published_bytes = read_file_at(&file, "state-anchor trust journal")?;
        let after = file_change_stamp(&file, "state-anchor trust journal")?;
        if before != after || published_bytes != bytes {
            return Err(EngineError::Internal(
                "state-anchor trust journal changed during publication verification".to_string(),
            ));
        }
        self.trust_file = Some(file);
        self.trust_identity = Some(identity);
        self.trust_journal = Some(parsed);
        self.trust_stamp = Some(after);
        self.trust_bytes = Some(bytes.to_vec());
        Ok(())
    }

    #[cfg(unix)]
    fn revalidate_state_anchor_trust_intent(
        &self,
        expected_bytes: &[u8],
    ) -> Result<(), EngineError> {
        let file = openat_optional(
            self.directory.as_raw_fd(),
            &self.trust_intent_name,
            libc::O_RDONLY,
            "state-anchor trust transition intent",
        )?
        .ok_or_else(|| {
            EngineError::Internal(
                "state-anchor trust transition intent disappeared during mutation".to_string(),
            )
        })?;
        validate_secure_regular_file(&file, "state-anchor trust transition intent")?;
        let length = usize::try_from(
            file.metadata()
                .map_err(|error| {
                    EngineError::Internal(format!(
                        "failed to stat state-anchor trust transition intent: {error}"
                    ))
                })?
                .len(),
        )
        .map_err(|_| {
            EngineError::Internal(
                "state-anchor trust transition intent length does not fit this platform"
                    .to_string(),
            )
        })?;
        if length > STATE_ANCHOR_TRUST_MAX_INTENT_LENGTH {
            return Err(EngineError::Internal(
                "state-anchor trust transition intent exceeds its durable bound".to_string(),
            ));
        }
        let live_bytes = read_file_at(&file, "state-anchor trust transition intent")?;
        if live_bytes != expected_bytes {
            return Err(EngineError::Internal(
                "state-anchor trust transition intent changed during mutation".to_string(),
            ));
        }
        parse_state_anchor_trust_transition_intent(&live_bytes, &self.identity.fingerprint)?;
        Ok(())
    }

    #[cfg(unix)]
    fn state_anchor_trust_transition_journal_growth(
        &self,
        transition: &VerifiedStateAnchorTrustTransition,
    ) -> Result<usize, EngineError> {
        let mut growth = if self.trust_bytes.is_none() {
            STATE_ANCHOR_TRUST_JOURNAL_HEADER_LENGTH
        } else {
            0
        };
        for certificate in &transition.certificates {
            let prepare = encode_state_anchor_trust_prepare_record(
                &self.identity.fingerprint,
                &[0u8; 32],
                certificate,
            )?;
            let commit = encode_state_anchor_trust_commit_record(
                &self.identity.fingerprint,
                &[0u8; 32],
                certificate,
            )?;
            growth = growth
                .checked_add(prepare.len())
                .and_then(|length| length.checked_add(commit.len()))
                .ok_or_else(|| {
                    EngineError::Validation(
                        "state-anchor trust journal batch length overflows this platform"
                            .to_string(),
                    )
                })?;
        }
        Ok(growth)
    }

    #[cfg(unix)]
    fn ensure_state_anchor_trust_transition_journal_capacity(
        &self,
        journal_growth: usize,
    ) -> Result<(), EngineError> {
        let current_length = match (&self.trust_journal, &self.trust_bytes) {
            (Some(_), Some(bytes)) => bytes.len(),
            (None, None) => 0,
            _ => {
                return Err(EngineError::Internal(
                    "state-anchor trust journal byte/model invariant is inconsistent".to_string(),
                ))
            }
        };
        ensure_state_anchor_trust_transition_journal_capacity_for_length(
            current_length,
            journal_growth,
        )
    }

    #[cfg(unix)]
    fn create_state_anchor_trust_transition_intent(
        &mut self,
        bytes: &[u8],
        admission_expires_at_unix_ms: u64,
        journal_growth: usize,
    ) -> Result<(), EngineError> {
        const LABEL: &str = "state-anchor trust transition intent";
        let temp_name = unique_temp_name(&self.trust_intent_name)?;
        let temp_file = openat_regular(
            self.directory.as_raw_fd(),
            &temp_name,
            libc::O_RDWR | libc::O_CREAT | libc::O_EXCL,
            0o600,
            LABEL,
        )?;
        let outcome = (|| {
            validate_owned_unlinked_regular(&temp_file, LABEL)?;
            set_owner_only_permissions(&temp_file, LABEL)?;
            validate_secure_regular_file(&temp_file, LABEL)?;
            write_file_at(&temp_file, bytes, LABEL)?;
            temp_file.sync_all().map_err(|error| {
                EngineError::Internal(format!("failed to sync new {LABEL}: {error}"))
            })?;

            // Resolve absence before the final admission check because even
            // fstatat can stall. The descriptor guard, capacity, and expiry
            // checks must be the last operations before publication. The
            // resulting intent is durable evidence only; resumed mutation
            // still requires a newly verified fresh Read.
            ensure_entry_absent(self.directory.as_raw_fd(), &self.trust_intent_name, LABEL)?;
            self.revalidate()?;
            self.ensure_state_anchor_trust_transition_journal_capacity(journal_growth)?;
            recheck_state_anchor_admission_expiry(admission_expires_at_unix_ms)?;
            renameat_same_directory(
                self.directory.as_raw_fd(),
                &temp_name,
                &self.trust_intent_name,
                LABEL,
            )?;
            self.directory.sync_all().map_err(|error| {
                EngineError::Internal(format!(
                    "failed to sync signer state directory after publishing {LABEL}: {error}"
                ))
            })?;
            let identity = descriptor_identity(&temp_file, LABEL)?;
            validate_live_entry(&self.directory, &self.trust_intent_name, identity, LABEL)?;
            self.state_anchor_trust_mutation_guard().revalidate()
        })();
        if outcome.is_err() {
            let _ = unlinkat_entry(self.directory.as_raw_fd(), &temp_name);
        }
        outcome
    }

    #[cfg(unix)]
    fn state_anchor_trust_mutation_guard(&self) -> StateAnchorTrustRecoveryGuard<'_> {
        StateAnchorTrustRecoveryGuard {
            directory: &self.directory,
            canonical_parent: &self.canonical_parent,
            directory_identity: self.directory_identity,
            lock_name: &self.lock_name,
            lock_file: &self._file,
            lock_identity: self.lock_identity,
            store_id_name: &self.store_id_name,
            store_id_file: &self.store_id_file,
            store_id_identity: self.store_id_identity,
            store_id: self.identity.store_id,
            state_name: &self.state_name,
            state_file: self.current_state_file.as_ref(),
            state_identity: self.current_state_identity,
        }
    }

    #[cfg(unix)]
    fn revalidate_trust_transition_mutation_guard(&self) -> Result<(), EngineError> {
        self.state_anchor_trust_mutation_guard().revalidate()?;
        validate_live_entry(
            &self.directory,
            &self.witness_name,
            self.witness_identity,
            "signer state witness journal",
        )?;
        if let Some(identity) = self.current_state_identity {
            validate_live_entry(
                &self.directory,
                &self.state_name,
                identity,
                "signer state file",
            )?;
        } else {
            ensure_entry_absent(
                self.directory.as_raw_fd(),
                &self.state_name,
                "signer state file",
            )?;
        }
        match self.trust_identity {
            Some(identity) => validate_live_entry(
                &self.directory,
                &self.trust_name,
                identity,
                "state-anchor trust journal",
            )?,
            None => ensure_entry_absent(
                self.directory.as_raw_fd(),
                &self.trust_name,
                "state-anchor trust journal",
            )?,
        }
        match self.anchor_identity {
            Some(identity) => validate_live_entry(
                &self.directory,
                &self.anchor_name,
                identity,
                "signer state anchor metadata",
            )?,
            None => ensure_entry_absent(
                self.directory.as_raw_fd(),
                &self.anchor_name,
                "signer state anchor metadata",
            )?,
        }
        Ok(())
    }

    #[cfg(unix)]
    pub(crate) fn acknowledge_state_witness_checkpoint(
        &mut self,
        acknowledgement: StateAnchorAcknowledgement,
        rotation_threshold: usize,
        allow_unanchored_recovery: bool,
        admission_expires_at_unix_ms: u64,
    ) -> Result<AnchorAcknowledgeOutcome, EngineError> {
        self.reconcile_pending_witness()?;
        self.revalidate()?;
        self.normalize_published_pending_anchor()?;
        recheck_state_anchor_admission_expiry(admission_expires_at_unix_ms)?;
        if self.witness_rotation_threshold != Some(rotation_threshold) {
            return Err(EngineError::Internal(
                "state witness rotation threshold changed after the durable store was opened"
                    .to_string(),
            ));
        }
        validate_anchor_acknowledgement_shape(&acknowledgement, &self.identity.fingerprint)?;
        let tip = self.witness_history.last().ok_or_else(|| {
            EngineError::Internal("signer state witness journal has no committed tip".to_string())
        })?;
        if acknowledgement.checkpoint_store_fingerprint != self.identity.fingerprint
            || acknowledgement.checkpoint_generation != tip.generation
            || acknowledgement.checkpoint_previous_commitment != tip.previous_commitment
            || acknowledgement.checkpoint_state_image_digest != tip.state_image_digest
            || acknowledgement.checkpoint_state_commitment != tip.commitment
        {
            return Err(EngineError::Validation(
                "state-anchor acknowledgement checkpoint does not exactly match the current \
                 durable witness tip"
                    .to_string(),
            ));
        }

        let idempotent = validate_anchor_monotonic_update(
            self.anchor_metadata.as_ref(),
            &acknowledgement,
            allow_unanchored_recovery,
        )?;
        if let Some(head) = self
            .trust_journal
            .as_ref()
            .and_then(StateAnchorTrustJournalModel::head)
        {
            validate_state_anchor_trust_reference_descendant(
                &head.to.reference,
                &StateAnchorTrustReferenceModel::from_acknowledgement(&acknowledgement),
                "state-anchor acknowledgement",
            )?;
        }

        // A failed state transaction appends PREPARE+ABORT without advancing
        // the tip. Once that reaches the threshold, an exact replay of the
        // already-anchored tip must still be able to compact the segment.
        // A newly rotated segment has zero records, so the count gate alone
        // prevents duplicate rotations.
        let should_rotate = self.witness_record_count()? >= rotation_threshold;
        let witness_base = self
            .anchor_metadata
            .as_ref()
            .and_then(|metadata| metadata.witness_base.clone());
        let pending_witness_base = if should_rotate {
            Some(acknowledgement.clone())
        } else {
            self.anchor_metadata
                .as_ref()
                .and_then(|metadata| metadata.pending_witness_base.clone())
        };
        let next_metadata = StateAnchorMetadata {
            latest: acknowledgement.clone(),
            witness_base,
            pending_witness_base,
        };
        if self.anchor_metadata.as_ref() != Some(&next_metadata) {
            self.persist_state_anchor_metadata(next_metadata)?;
        }
        let rotated = if should_rotate {
            self.rotate_state_witness_segment(&acknowledgement)?;
            self.persist_state_anchor_metadata(StateAnchorMetadata {
                latest: acknowledgement.clone(),
                witness_base: Some(acknowledgement),
                pending_witness_base: None,
            })?;
            true
        } else {
            false
        };
        let snapshot = self.state_witness_tip_snapshot()?;
        Ok(AnchorAcknowledgeOutcome {
            idempotent,
            rotated,
            snapshot,
        })
    }

    #[cfg(not(unix))]
    pub(crate) fn acknowledge_state_witness_checkpoint(
        &mut self,
        _acknowledgement: StateAnchorAcknowledgement,
        _rotation_threshold: usize,
        _allow_unanchored_recovery: bool,
        _admission_expires_at_unix_ms: u64,
    ) -> Result<AnchorAcknowledgeOutcome, EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    #[cfg(unix)]
    fn persist_state_anchor_metadata(
        &mut self,
        metadata: StateAnchorMetadata,
    ) -> Result<(), EngineError> {
        let configuration = self.anchor_configuration.as_ref().ok_or_else(|| {
            EngineError::Internal(
                "cannot persist signer state anchor metadata without manifest pins".to_string(),
            )
        })?;
        let bytes = encode_state_anchor_metadata(&self.identity.fingerprint, &metadata);
        let parsed = parse_state_anchor_metadata(
            &bytes,
            &self.identity.fingerprint,
            configuration,
            &self
                .trust_journal
                .as_ref()
                .map(StateAnchorTrustJournalModel::certified_floors)
                .unwrap_or_default(),
        )?;
        if parsed != metadata {
            return Err(EngineError::Internal(
                "new signer state anchor metadata failed canonical round-trip".to_string(),
            ));
        }
        let (file, identity) = if self.trust_head_inspection {
            let guard = self.state_anchor_trust_mutation_guard();
            replace_durable_entry_with_guard(
                &self.directory,
                &self.anchor_name,
                &bytes,
                "signer state anchor metadata",
                Some(&guard),
            )?
        } else {
            replace_state_anchor_entry(&self.directory, &self.anchor_name, &bytes)?
        };
        self.anchor_file = Some(file);
        self.anchor_identity = Some(identity);
        self.anchor_metadata = Some(metadata);
        self.anchor_bytes = Some(bytes);
        Ok(())
    }

    #[cfg(unix)]
    fn normalize_published_pending_anchor(&mut self) -> Result<(), EngineError> {
        let Some(metadata) = self.anchor_metadata.as_ref() else {
            return Ok(());
        };
        let Some(pending) = metadata.pending_witness_base.as_ref() else {
            return Ok(());
        };
        let published = self
            .witness_segment_header
            .as_ref()
            .is_some_and(|header| acknowledgement_matches_witness(pending, Some(&header.base)));
        if !published {
            return Ok(());
        }
        let normalized = StateAnchorMetadata {
            latest: metadata.latest.clone(),
            witness_base: Some(pending.clone()),
            pending_witness_base: None,
        };
        self.persist_state_anchor_metadata(normalized)
    }

    /// Completes a durably authorized witness rotation before any caller can
    /// observe a tip or perform another state mutation. This is the
    /// same-process counterpart of acquisition-time crash recovery: an I/O
    /// failure after the pending acknowledgement or any rename boundary must
    /// never leave maintenance to occur lazily after the host installs its
    /// rollback/output barrier.
    #[cfg(unix)]
    fn settle_pending_state_witness_rotation(&mut self) -> Result<(), EngineError> {
        self.settle_pending_state_witness_rotation_inner(true)
    }

    #[cfg(unix)]
    fn settle_pending_state_witness_rotation_inner(
        &mut self,
        revalidate_steady_store: bool,
    ) -> Result<(), EngineError> {
        if self
            .anchor_metadata
            .as_ref()
            .and_then(|metadata| metadata.pending_witness_base.as_ref())
            .is_none()
        {
            return Ok(());
        }
        if self.pending_witness.is_some() {
            return Err(EngineError::Internal(
                "cannot settle state witness rotation while a state update is pending".to_string(),
            ));
        }
        if !recover_state_witness_rotation(
            &self.directory,
            StateWitnessRotationNames {
                current: &self.witness_name,
                next: &self.witness_next_name,
                previous: &self.witness_previous_name,
            },
            &self.identity,
            self.current_state_file.as_ref(),
            self.anchor_metadata.as_ref(),
            self.witness_max_records,
            true,
            None,
        )? {
            return Err(EngineError::Internal(
                "pending state witness rotation did not produce a promoted base".to_string(),
            ));
        }

        let opened = open_or_create_state_witness(
            &self.directory,
            &self.witness_name,
            &self.identity,
            self.current_state_file.as_ref(),
            self.witness_max_records,
            self.anchor_metadata.as_ref(),
        )?;
        self.witness_file = opened.file;
        self.witness_identity = opened.identity;
        self.witness_history = opened.parsed.history;
        self.pending_witness = opened.parsed.pending;
        self.witness_length = opened.parsed.length;
        self.witness_header_length = opened.parsed.header_length;
        self.witness_header_bytes = opened.parsed.header_bytes;
        self.witness_segment_header = opened.parsed.segment_header;
        self.witness_prefix = None;
        self.last_appended_record = [0u8; TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH];
        self.last_appended_stamp = None;
        self.verify_state_witness_journal_fully()?;
        self.normalize_published_pending_anchor()?;
        if revalidate_steady_store {
            self.revalidate_store_entries()?;
        }
        Ok(())
    }

    #[cfg(unix)]
    fn rotate_state_witness_segment(
        &mut self,
        acknowledgement: &StateAnchorAcknowledgement,
    ) -> Result<(), EngineError> {
        let validation_anchor = self.anchor_metadata.clone();
        self.rotate_state_witness_segment_inner(acknowledgement, validation_anchor.as_ref(), true)
    }

    #[cfg(unix)]
    fn rotate_state_witness_segment_for_trust_transition(
        &mut self,
        acknowledgement: &StateAnchorAcknowledgement,
        target_anchor: &StateAnchorMetadata,
    ) -> Result<(), EngineError> {
        self.rotate_state_witness_segment_inner(acknowledgement, Some(target_anchor), false)
    }

    #[cfg(unix)]
    fn rotate_state_witness_segment_inner(
        &mut self,
        acknowledgement: &StateAnchorAcknowledgement,
        validation_anchor: Option<&StateAnchorMetadata>,
        retire_previous: bool,
    ) -> Result<(), EngineError> {
        if self.pending_witness.is_some() {
            return Err(EngineError::Internal(
                "cannot rotate a state witness segment while an update is pending".to_string(),
            ));
        }
        let tip = self.witness_history.last().cloned().ok_or_else(|| {
            EngineError::Internal("signer state witness journal has no committed tip".to_string())
        })?;
        if acknowledgement.checkpoint_generation != tip.generation
            || acknowledgement.checkpoint_previous_commitment != tip.previous_commitment
            || acknowledgement.checkpoint_state_image_digest != tip.state_image_digest
            || acknowledgement.checkpoint_state_commitment != tip.commitment
        {
            return Err(EngineError::Internal(
                "state witness rotation acknowledgement no longer matches the current tip"
                    .to_string(),
            ));
        }
        ensure_entry_absent(
            self.directory.as_raw_fd(),
            &self.witness_next_name,
            "next signer state witness journal",
        )?;
        ensure_entry_absent(
            self.directory.as_raw_fd(),
            &self.witness_previous_name,
            "previous signer state witness journal",
        )?;
        let bytes =
            encode_state_witness_segment_header(&self.identity.fingerprint, acknowledgement)?;
        let (next_file, next_identity) = if retire_previous {
            create_entry_atomically(
                &self.directory,
                &self.witness_next_name,
                &bytes,
                "next signer state witness journal",
            )?
        } else {
            let guard = self.state_anchor_trust_mutation_guard();
            create_entry_atomically_with_guard(
                &self.directory,
                &self.witness_next_name,
                &bytes,
                "next signer state witness journal",
                Some(&guard),
            )?
        };
        let parsed = read_state_witness_journal_streaming(
            &next_file,
            &self.identity.store_id,
            &self.identity.fingerprint,
            self.witness_max_records,
            validation_anchor,
        )?;
        if parsed.segment_header.is_none()
            || parsed.pending.is_some()
            || parsed.history.as_slice() != [tip.clone()]
        {
            let _ = unlinkat_entry(self.directory.as_raw_fd(), &self.witness_next_name);
            return Err(EngineError::Internal(
                "new state witness segment failed pre-publication verification".to_string(),
            ));
        }
        #[cfg(test)]
        if !retire_previous {
            maybe_inject_state_anchor_trust_transition_fault(
                StateAnchorTrustTransitionFaultInjectionPoint::AfterNextWitnessPublication,
            )?;
        }
        let current_digest = current_state_image_digest(self.current_state_file.as_ref())?;
        if tip.state_image_digest != current_digest {
            let _ = unlinkat_entry(self.directory.as_raw_fd(), &self.witness_next_name);
            return Err(EngineError::Internal(
                "new state witness segment base does not commit the current state image"
                    .to_string(),
            ));
        }
        if !retire_previous {
            self.state_anchor_trust_mutation_guard().revalidate()?;
        }
        if let Err(error) = renameat_same_directory(
            self.directory.as_raw_fd(),
            &self.witness_name,
            &self.witness_previous_name,
            "retain previous state witness segment",
        ) {
            let _ = unlinkat_entry(self.directory.as_raw_fd(), &self.witness_next_name);
            return Err(error);
        }
        self.directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory after retaining previous witness \
                 segment: {error}"
            ))
        })?;
        if !retire_previous {
            self.state_anchor_trust_mutation_guard().revalidate()?;
            #[cfg(test)]
            maybe_inject_state_anchor_trust_transition_fault(
                StateAnchorTrustTransitionFaultInjectionPoint::AfterPreviousWitnessPublication,
            )?;
        }
        renameat_same_directory(
            self.directory.as_raw_fd(),
            &self.witness_next_name,
            &self.witness_name,
            "publish next state witness segment",
        )?;
        self.directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory after publishing witness segment: {error}"
            ))
        })?;
        validate_live_entry(
            &self.directory,
            &self.witness_name,
            next_identity,
            "signer state witness journal",
        )?;
        if !retire_previous {
            self.state_anchor_trust_mutation_guard().revalidate()?;
            #[cfg(test)]
            maybe_inject_state_anchor_trust_transition_fault(
                StateAnchorTrustTransitionFaultInjectionPoint::AfterCurrentWitnessPublication,
            )?;
        }

        self.witness_file = next_file;
        self.witness_identity = next_identity;
        self.witness_history = parsed.history;
        self.pending_witness = parsed.pending;
        self.witness_length = parsed.length;
        self.witness_header_length = parsed.header_length;
        self.witness_header_bytes = parsed.header_bytes;
        self.witness_segment_header = parsed.segment_header;
        self.witness_prefix = None;
        self.last_appended_record = [0u8; TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH];
        self.last_appended_stamp = None;

        // The new name and complete signed header are durable and verified.
        // Only now may the previous segment be retired.
        self.verify_state_witness_journal_fully_with_anchor(validation_anchor)?;
        if self
            .witness_history
            .last()
            .map(|entry| entry.state_image_digest != current_digest)
            .unwrap_or(true)
        {
            return Err(EngineError::Internal(
                "published state witness segment does not commit the current state image; \
                 retaining the previous segment"
                    .to_string(),
            ));
        }
        if retire_previous {
            unlinkat_entry(self.directory.as_raw_fd(), &self.witness_previous_name)?;
            self.directory.sync_all().map_err(|error| {
                EngineError::Internal(format!(
                    "failed to sync signer state directory after retiring previous witness \
                     segment: {error}"
                ))
            })?;
        }
        Ok(())
    }

    pub(crate) fn state_witness_proof(
        &mut self,
        ancestor_generation: u64,
        ancestor_commitment: [u8; 32],
        target_generation: u64,
        target_commitment: [u8; 32],
        maximum_entries: usize,
    ) -> Result<(Vec<StateWitness>, bool), EngineError> {
        self.reconcile_pending_witness()?;
        self.revalidate()?;
        if maximum_entries == 0 || maximum_entries > 256 {
            return Err(EngineError::Validation(
                "state witness proof maximumEntries must be between 1 and 256".to_string(),
            ));
        }
        let ancestor_index = resolve_witness_history_index(
            &self.witness_history,
            ancestor_generation,
            ancestor_commitment,
            "ancestor",
        )?;
        let target_index = resolve_witness_history_index(
            &self.witness_history,
            target_generation,
            target_commitment,
            "target",
        )?;
        if target_index < ancestor_index {
            return Err(EngineError::Validation(
                "state witness proof target precedes the requested ancestor".to_string(),
            ));
        }
        if target_index == ancestor_index {
            return Ok((Vec::new(), true));
        }

        let end = (ancestor_index + 1 + maximum_entries).min(target_index + 1);
        let entries = self.witness_history[(ancestor_index + 1)..end].to_vec();
        Ok((entries, end == target_index + 1))
    }

    #[cfg(unix)]
    fn next_state_witness(
        &self,
        state_image_digest: [u8; 32],
    ) -> Result<StateWitness, EngineError> {
        let tip = self
            .witness_history
            .last()
            .expect("a durable store always has a genesis state witness");
        let generation = tip.generation.checked_add(1).ok_or_else(|| {
            EngineError::Internal("signer state witness generation exhausted u64".to_string())
        })?;
        let previous_commitment = tip.commitment;
        let commitment = state_commitment(
            &self.identity.fingerprint,
            generation,
            &previous_commitment,
            &state_image_digest,
        );
        Ok(StateWitness {
            generation,
            previous_commitment,
            commitment,
            state_image_digest,
        })
    }

    #[cfg(unix)]
    fn prepare_witness(&mut self, witness: StateWitness) -> Result<(), EngineError> {
        if self.pending_witness.is_some() {
            return Err(EngineError::Internal(
                "cannot prepare a state witness while another update is pending".to_string(),
            ));
        }
        let expected = self.next_state_witness(witness.state_image_digest)?;
        if witness != expected || witness.generation == 0 {
            return Err(EngineError::Internal(
                "prepared state witness does not extend the active witness tip".to_string(),
            ));
        }
        if let Some(threshold) = self.witness_rotation_threshold {
            if self.witness_record_count()? >= threshold {
                return Err(EngineError::Internal(
                    "signer state witness rotation threshold reached; a fresh manifest-pinned, \
                     authority-signed acknowledgement of the current tip is required before \
                     additional state writes"
                        .to_string(),
                ));
            }
        }
        self.ensure_witness_record_capacity(2)?;
        self.append_witness_record(TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE, &witness)?;
        self.pending_witness = Some(witness);
        self.extend_witness_prefix()
    }

    #[cfg(unix)]
    fn commit_pending_witness(&mut self) -> Result<(), EngineError> {
        let pending = self.pending_witness.clone().ok_or_else(|| {
            EngineError::Internal("no prepared state witness to commit".to_string())
        })?;
        self.append_witness_record(TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT, &pending)?;
        self.witness_history.push(pending);
        self.pending_witness = None;
        self.extend_witness_prefix()
    }

    #[cfg(unix)]
    fn abort_pending_witness(&mut self) -> Result<(), EngineError> {
        let pending = self.pending_witness.clone().ok_or_else(|| {
            EngineError::Internal("no prepared state witness to abort".to_string())
        })?;
        self.append_witness_record(TBTC_SIGNER_STATE_WITNESS_RECORD_ABORT, &pending)?;
        self.pending_witness = None;
        self.extend_witness_prefix()
    }

    #[cfg(unix)]
    fn reconcile_pending_witness(&mut self) -> Result<(), EngineError> {
        let Some(pending) = self.pending_witness.clone() else {
            return Ok(());
        };
        let current_digest = current_state_image_digest(self.current_state_file.as_ref())?;
        let committed = self.witness_history.last().ok_or_else(|| {
            EngineError::Internal("signer state witness journal has no committed tip".to_string())
        })?;
        if current_digest == pending.state_image_digest {
            // The state rename won. Make that directory entry durable before
            // appending COMMIT, including recovery from a crash/fault directly
            // after rename.
            self.directory.sync_all().map_err(|error| {
                EngineError::Internal(format!(
                    "failed to sync signer state directory while recovering witness: {error}"
                ))
            })?;
            return self.commit_pending_witness();
        }
        if current_digest == committed.state_image_digest {
            return self.abort_pending_witness();
        }
        Err(EngineError::Internal(
            "ambiguous signer state witness update: current state matches neither the committed nor prepared image"
                .to_string(),
        ))
    }

    #[cfg(not(unix))]
    fn reconcile_pending_witness(&mut self) -> Result<(), EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    #[cfg(unix)]
    fn append_witness_record(
        &mut self,
        record_type: u8,
        witness: &StateWitness,
    ) -> Result<(), EngineError> {
        // A cooperating append must never turn an unverified or externally
        // changed prefix into a new trusted cache entry. Validate the exact
        // pre-append journal first; this also checks the fixed anchors and
        // forces a streaming full parse on any stamp mismatch.
        self.verify_state_witness_journal()?;
        self.ensure_witness_record_capacity(1)?;
        let stat = descriptor_stat(&self.witness_file, "signer state witness journal")?;
        if stat.st_size < 0 || stat.st_size as usize != self.witness_length {
            return Err(EngineError::Internal(
                "signer state witness journal length changed before append".to_string(),
            ));
        }
        let record = encode_state_witness_record(record_type, witness);
        append_file_at(
            &self.witness_file,
            self.witness_length,
            &record,
            "signer state witness journal",
        )?;
        self.witness_file.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state witness journal: {error}"
            ))
        })?;
        let appended_stamp = witness_change_stamp(&self.witness_file)?;
        self.witness_length += record.len();
        self.last_appended_record.copy_from_slice(&record);
        self.last_appended_stamp = Some(appended_stamp);
        Ok(())
    }

    #[cfg(unix)]
    fn witness_record_count(&self) -> Result<usize, EngineError> {
        self.witness_length
            .checked_sub(self.witness_header_length)
            .filter(|bytes| bytes % TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH == 0)
            .map(|bytes| bytes / TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH)
            .ok_or_else(|| {
                EngineError::Internal(
                    "signer state witness journal length is not record-aligned".to_string(),
                )
            })
    }

    #[cfg(unix)]
    fn ensure_witness_record_capacity(&self, additional: usize) -> Result<(), EngineError> {
        let required = self
            .witness_record_count()?
            .checked_add(additional)
            .ok_or_else(|| {
                EngineError::Internal(
                    "signer state witness journal record count overflowed".to_string(),
                )
            })?;
        if required > self.witness_max_records {
            return Err(EngineError::Internal(format!(
                "signer state witness journal record ceiling [{}] reached; refusing unsigned \
                 local compaction or re-genesis. Install a future manifest-pinned, \
                 authority-signed checkpoint through the checkpoint ABI before resuming writes",
                self.witness_max_records
            )));
        }
        Ok(())
    }
}

fn resolve_witness_history_index(
    history: &[StateWitness],
    generation: u64,
    commitment: [u8; 32],
    label: &str,
) -> Result<usize, EngineError> {
    let base_generation = history
        .first()
        .map(|entry| entry.generation)
        .ok_or_else(|| {
            EngineError::Internal("signer state witness journal has no retained base".to_string())
        })?;
    if generation < base_generation {
        return Err(EngineError::HistoryPruned {
            requested_generation: generation,
            witness_base_generation: base_generation,
        });
    }
    let index = generation
        .checked_sub(base_generation)
        .and_then(|value| usize::try_from(value).ok())
        .ok_or_else(|| {
            EngineError::Validation(format!(
                "state witness proof {label} is not in the active store history"
            ))
        })?;
    match history.get(index) {
        Some(entry) if entry.generation == generation && entry.commitment == commitment => {
            Ok(index)
        }
        _ => Err(EngineError::Validation(format!(
            "state witness proof {label} is not in the active store history"
        ))),
    }
}

#[cfg(unix)]
fn ensure_state_anchor_trust_transition_journal_capacity_for_length(
    current_length: usize,
    journal_growth: usize,
) -> Result<(), EngineError> {
    if current_length
        .checked_add(journal_growth)
        .is_none_or(|length| length > STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH)
    {
        return Err(EngineError::Validation(
            "complete state-anchor trust transition batch would exceed the durable journal size \
             bound"
                .to_string(),
        ));
    }
    Ok(())
}

#[derive(Debug)]
pub(crate) struct StoreReplaceError {
    error: EngineError,
    replaced: bool,
}

impl StoreReplaceError {
    fn before_replacement(error: EngineError) -> Self {
        Self {
            error,
            replaced: false,
        }
    }

    pub(crate) fn replaced(&self) -> bool {
        self.replaced
    }

    pub(crate) fn into_engine_error(self) -> EngineError {
        self.error
    }
}

#[cfg(test)]
pub(crate) fn durable_store_id_file_path(state_path: &Path) -> PathBuf {
    let name = state_path
        .file_name()
        .map(durable_store_id_file_name)
        .unwrap_or_else(|| OsString::from("signer-state.store-id"));
    state_path
        .parent()
        .map(|parent| parent.join(&name))
        .unwrap_or_else(|| PathBuf::from(name))
}

#[cfg(test)]
pub(crate) fn state_witness_file_path(state_path: &Path) -> PathBuf {
    let name = state_path
        .file_name()
        .map(state_witness_file_name)
        .unwrap_or_else(|| OsString::from("signer-state.state-witness"));
    state_path
        .parent()
        .map(|parent| parent.join(&name))
        .unwrap_or_else(|| PathBuf::from(name))
}

#[cfg(test)]
pub(crate) fn state_anchor_file_path(state_path: &Path) -> PathBuf {
    let name = state_path
        .file_name()
        .map(state_anchor_file_name)
        .unwrap_or_else(|| OsString::from("signer-state.state-anchor"));
    state_path
        .parent()
        .map(|parent| parent.join(&name))
        .unwrap_or_else(|| PathBuf::from(name))
}

#[cfg(test)]
#[allow(dead_code)]
pub(crate) fn state_anchor_trust_file_path(state_path: &Path) -> PathBuf {
    let name = state_path
        .file_name()
        .map(state_anchor_trust_file_name)
        .unwrap_or_else(|| OsString::from("signer-state.state-anchor-trust"));
    state_path
        .parent()
        .map(|parent| parent.join(&name))
        .unwrap_or_else(|| PathBuf::from(name))
}

#[cfg(test)]
#[allow(dead_code)]
pub(crate) fn state_anchor_trust_intent_file_path(state_path: &Path) -> PathBuf {
    let name = state_path
        .file_name()
        .map(state_anchor_trust_intent_file_name)
        .unwrap_or_else(|| OsString::from("signer-state.state-anchor-trust.intent"));
    state_path
        .parent()
        .map(|parent| parent.join(&name))
        .unwrap_or_else(|| PathBuf::from(name))
}

fn durable_store_id_file_name(state_name: &OsStr) -> OsString {
    let mut name = state_name.to_os_string();
    name.push(TBTC_SIGNER_DURABLE_STORE_ID_SUFFIX);
    name
}

fn state_witness_file_name(state_name: &OsStr) -> OsString {
    let mut name = state_name.to_os_string();
    name.push(TBTC_SIGNER_STATE_WITNESS_SUFFIX);
    name
}

fn state_anchor_file_name(state_name: &OsStr) -> OsString {
    let mut name = state_name.to_os_string();
    name.push(TBTC_SIGNER_STATE_ANCHOR_SUFFIX);
    name
}

fn state_anchor_trust_file_name(state_name: &OsStr) -> OsString {
    let mut name = state_name.to_os_string();
    name.push(TBTC_SIGNER_STATE_ANCHOR_TRUST_SUFFIX);
    name
}

fn state_anchor_trust_intent_file_name(state_name: &OsStr) -> OsString {
    let mut name = state_name.to_os_string();
    name.push(TBTC_SIGNER_STATE_ANCHOR_TRUST_INTENT_SUFFIX);
    name
}

/// The Go/Rust v2 store fingerprint: the stable anchor of the state-commitment
/// transcript.
///
/// Inputs are the two compile-time contract constants and the 32 fsynced
/// `.store-id` bytes, length-prefixed exactly as `hash_fields` prescribes.
/// Nothing volatile (path, device, inode, lock file) may ever be added here:
/// this value is recomputed on every start and every committed record must
/// still verify under it.
pub(crate) fn durable_store_fingerprint(store_id: &[u8; 32]) -> [u8; 32] {
    hash_fields(
        TBTC_SIGNER_DURABLE_STORE_FINGERPRINT_DOMAIN,
        &[
            TBTC_SIGNER_DURABLE_STORE_IDENTITY_SCHEMA.as_bytes(),
            TBTC_SIGNER_DURABLE_STORE_BACKEND.as_bytes(),
            store_id,
        ],
    )
}

/// The retired v1 fingerprint transcript, kept only so the frozen cross-language
/// v1 vector stays pinned and so v1 journal fixtures can be built for the
/// rejection-path regression tests.
#[cfg(test)]
pub(crate) fn durable_store_fingerprint_v1(
    store_id: &[u8; 32],
    canonical_path_fingerprint: &[u8; 32],
    filesystem_fingerprint: &[u8; 32],
    lock_fingerprint: &[u8; 32],
) -> [u8; 32] {
    hash_fields(
        b"tbtc-signer-durable-session-store-fingerprint-v1\0",
        &[
            b"tbtc-signer-durable-session-store-identity/v1",
            TBTC_SIGNER_DURABLE_STORE_BACKEND.as_bytes(),
            store_id,
            canonical_path_fingerprint,
            filesystem_fingerprint,
            lock_fingerprint,
        ],
    )
}

fn state_image_digest(state_bytes: Option<&[u8]>) -> [u8; 32] {
    match state_bytes {
        Some(bytes) => hash_fields(TBTC_SIGNER_STATE_IMAGE_DIGEST_DOMAIN, &[&[1], bytes]),
        None => hash_fields(TBTC_SIGNER_STATE_IMAGE_DIGEST_DOMAIN, &[&[0], &[]]),
    }
}

pub(crate) fn state_commitment(
    store_fingerprint: &[u8; 32],
    generation: u64,
    previous_commitment: &[u8; 32],
    state_image_digest: &[u8; 32],
) -> [u8; 32] {
    // Frozen Go/Rust v2 transcript: fields are fixed-width and therefore are
    // concatenated directly, without the length prefixes used by hash_fields.
    // `store_fingerprint` MUST be the stable `durable_store_fingerprint`.
    let mut digest = Sha256::new();
    digest.update(TBTC_SIGNER_STATE_COMMITMENT_DOMAIN);
    digest.update(store_fingerprint);
    digest.update(generation.to_be_bytes());
    digest.update(previous_commitment);
    digest.update(state_image_digest);
    digest.finalize().into()
}

fn state_witness_genesis(store_fingerprint: &[u8; 32]) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(TBTC_SIGNER_STATE_WITNESS_GENESIS_DOMAIN);
    digest.update(store_fingerprint);
    digest.finalize().into()
}

/// The retired v1 commitment transcript, retained for the frozen v1 vectors and
/// for building v1 journal fixtures in the rejection-path regression tests.
#[cfg(test)]
pub(crate) fn state_commitment_v1(
    store_fingerprint: &[u8; 32],
    generation: u64,
    previous_commitment: &[u8; 32],
    state_image_digest: &[u8; 32],
) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(b"tbtc-signer-state-witness-commitment-v1\0");
    digest.update(store_fingerprint);
    digest.update(generation.to_be_bytes());
    digest.update(previous_commitment);
    digest.update(state_image_digest);
    digest.finalize().into()
}

/// The retired v1 genesis transcript. See [`state_commitment_v1`].
#[cfg(test)]
pub(crate) fn state_witness_genesis_v1(store_fingerprint: &[u8; 32]) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(b"tbtc-signer-state-witness-genesis-v1\0");
    digest.update(store_fingerprint);
    digest.finalize().into()
}

/// Builds a complete, well-formed v1 journal image (header + PREPARE + COMMIT)
/// for the v1 rejection-path regression test. Only the v1 magic is needed to
/// recognize such a journal, but a fixture that is otherwise valid proves the
/// rejection is driven by the transcript version and not by incidental damage.
#[cfg(test)]
pub(crate) fn encode_v1_state_witness_genesis_journal(
    store_id: &[u8; 32],
    v1_store_fingerprint: &[u8; 32],
    state_image_digest: &[u8; 32],
) -> Vec<u8> {
    let previous_commitment = state_witness_genesis_v1(v1_store_fingerprint);
    let genesis = StateWitness {
        generation: 1,
        previous_commitment,
        commitment: state_commitment_v1(
            v1_store_fingerprint,
            1,
            &previous_commitment,
            state_image_digest,
        ),
        state_image_digest: *state_image_digest,
    };
    let mut bytes = Vec::new();
    bytes.extend_from_slice(TBTC_SIGNER_STATE_WITNESS_MAGIC_V1);
    bytes.extend_from_slice(store_id);
    bytes.extend_from_slice(&encode_state_witness_record(
        TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE,
        &genesis,
    ));
    bytes.extend_from_slice(&encode_state_witness_record(
        TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT,
        &genesis,
    ));
    bytes
}

fn hash_fields(domain: &[u8], fields: &[&[u8]]) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(domain);
    for field in fields {
        digest.update((field.len() as u32).to_be_bytes());
        digest.update(field);
    }
    digest.finalize().into()
}

#[cfg(unix)]
fn validate_entry_name(name: &OsStr, label: &str) -> Result<(), EngineError> {
    if name.is_empty() || name.as_bytes().contains(&b'/') || name.as_bytes().contains(&0) {
        return Err(EngineError::Internal(format!(
            "invalid signer {label} file name"
        )));
    }
    Ok(())
}

#[cfg(not(unix))]
fn validate_entry_name(_name: &OsStr, _label: &str) -> Result<(), EngineError> {
    Ok(())
}

#[cfg(unix)]
fn os_str_cstring(value: &OsStr, label: &str) -> Result<CString, EngineError> {
    CString::new(value.as_bytes())
        .map_err(|_| EngineError::Internal(format!("signer {label} path contains a NUL byte")))
}

#[cfg(unix)]
fn open_absolute_directory_nofollow(path: &Path) -> Result<fs::File, EngineError> {
    use std::path::Component;

    if !path.is_absolute() {
        return Err(EngineError::Internal(format!(
            "signer store directory [{}] is not absolute",
            path.display()
        )));
    }

    let root = CString::new("/").expect("root contains no NUL");
    let root_fd = unsafe {
        libc::open(
            root.as_ptr(),
            libc::O_RDONLY | libc::O_DIRECTORY | libc::O_CLOEXEC | libc::O_NOFOLLOW,
        )
    };
    if root_fd < 0 {
        return Err(EngineError::Internal(format!(
            "failed to open filesystem root without following symlinks: {}",
            std::io::Error::last_os_error()
        )));
    }
    let mut directory = unsafe { fs::File::from_raw_fd(root_fd) };

    for component in path.components() {
        let Component::Normal(component) = component else {
            if matches!(component, Component::RootDir) {
                continue;
            }
            return Err(EngineError::Internal(format!(
                "canonical signer store directory [{}] contains a non-normal component",
                path.display()
            )));
        };
        let component = os_str_cstring(component, "directory component")?;
        let next_fd = unsafe {
            libc::openat(
                directory.as_raw_fd(),
                component.as_ptr(),
                libc::O_RDONLY | libc::O_DIRECTORY | libc::O_CLOEXEC | libc::O_NOFOLLOW,
            )
        };
        if next_fd < 0 {
            return Err(EngineError::Internal(format!(
                "failed to traverse canonical signer store directory [{}] without following symlinks: {}",
                path.display(),
                std::io::Error::last_os_error()
            )));
        }
        directory = unsafe { fs::File::from_raw_fd(next_fd) };
    }
    Ok(directory)
}

#[cfg(unix)]
fn openat_regular(
    directory_fd: RawFd,
    name: &OsStr,
    flags: i32,
    mode: libc::mode_t,
    label: &str,
) -> Result<fs::File, EngineError> {
    let name_c = os_str_cstring(name, label)?;
    let fd = unsafe {
        libc::openat(
            directory_fd,
            name_c.as_ptr(),
            flags | libc::O_CLOEXEC | libc::O_NOFOLLOW | libc::O_NONBLOCK,
            mode as libc::c_uint,
        )
    };
    if fd < 0 {
        return Err(EngineError::Internal(format!(
            "failed to open {label} without following symlinks: {}",
            std::io::Error::last_os_error()
        )));
    }
    Ok(unsafe { fs::File::from_raw_fd(fd) })
}

/// Creates a store entry whose ENTIRE content is written in one all-or-nothing
/// step.
///
/// `O_CREAT|O_EXCL` followed by a plain write is not atomic: a hard kill in that
/// window leaves a short file at the final name, and every short length of the
/// store ID and of the genesis journal is fatal on the next start - inside
/// `acquire`, where the corruption policy cannot reach. The complete image is
/// therefore written to a temp entry in the same directory, fsynced, renamed
/// over the target, and the DIRECTORY is fsynced so the rename itself is
/// durable. A crash anywhere before the rename leaves the target absent, which
/// is a state the opener already handles by creating it.
///
/// The temp entry is created with `openat` + `O_EXCL` + `O_NOFOLLOW` under the
/// held no-follow directory descriptor and mode 0600, so it is never a
/// symlink-following hazard, and the returned descriptor is the one that
/// survives the rename - no reopen by name is involved.
#[cfg(unix)]
fn create_entry_atomically(
    directory: &fs::File,
    name: &OsStr,
    bytes: &[u8],
    label: &str,
) -> Result<(fs::File, OpenedObjectIdentity), EngineError> {
    create_entry_atomically_with_guard(directory, name, bytes, label, None)
}

#[cfg(unix)]
fn create_entry_atomically_with_guard(
    directory: &fs::File,
    name: &OsStr,
    bytes: &[u8],
    label: &str,
    recovery_guard: Option<&StateAnchorTrustRecoveryGuard<'_>>,
) -> Result<(fs::File, OpenedObjectIdentity), EngineError> {
    let temp_name = unique_temp_name(name)?;
    let temp_file = openat_regular(
        directory.as_raw_fd(),
        &temp_name,
        libc::O_RDWR | libc::O_CREAT | libc::O_EXCL,
        0o600,
        label,
    )?;

    let outcome = (|| -> Result<OpenedObjectIdentity, EngineError> {
        validate_owned_unlinked_regular(&temp_file, label)?;
        set_owner_only_permissions(&temp_file, label)?;
        validate_secure_regular_file(&temp_file, label)?;
        write_file_at(&temp_file, bytes, label)?;
        temp_file.sync_all().map_err(|error| {
            EngineError::Internal(format!("failed to sync new {label}: {error}"))
        })?;
        // Publish only over an absent name. The exclusive store lock is held,
        // so nothing that participates in this protocol can be racing us here;
        // an entry that appeared anyway is not ours to overwrite.
        ensure_entry_absent(directory.as_raw_fd(), name, label)?;
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
        renameat_same_directory(directory.as_raw_fd(), &temp_name, name, label)?;
        directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory after publishing {label}: {error}"
            ))
        })?;
        let identity = descriptor_identity(&temp_file, label)?;
        validate_live_entry(directory, name, identity, label)?;
        #[cfg(test)]
        maybe_replace_trust_lock_after_guarded_publication(recovery_guard)?;
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
        Ok(identity)
    })();

    match outcome {
        Ok(identity) => Ok((temp_file, identity)),
        Err(error) => {
            let _ = unlinkat_entry(directory.as_raw_fd(), &temp_name);
            Err(error)
        }
    }
}

#[cfg(unix)]
fn open_or_create_store_id(
    directory: &fs::File,
    name: &OsStr,
) -> Result<(fs::File, [u8; 32], OpenedObjectIdentity), EngineError> {
    const LABEL: &str = "signer durable store ID file";

    if let Some(file) = openat_optional(directory.as_raw_fd(), name, libc::O_RDWR, LABEL)? {
        validate_owned_unlinked_regular(&file, LABEL)?;
        set_owner_only_permissions(&file, LABEL)?;
        validate_secure_regular_file(&file, LABEL)?;
        let store_id = read_store_id(&file)?;
        let identity = descriptor_identity(&file, LABEL)?;
        return Ok((file, store_id, identity));
    }

    let mut store_id = [0u8; 32];
    loop {
        OsRng.fill_bytes(&mut store_id);
        if store_id != [0u8; 32] {
            break;
        }
    }
    let (file, identity) = create_entry_atomically(directory, name, &store_id, LABEL)?;
    Ok((file, store_id, identity))
}

#[cfg(unix)]
fn open_state_anchor(
    directory: &fs::File,
    name: &OsStr,
    store_fingerprint: &[u8; 32],
    configuration: Option<&StateAnchorConfiguration>,
    certified_floors: &[StateAnchorTrustReferenceModel],
) -> Result<OpenedStateAnchor, EngineError> {
    const LABEL: &str = "signer state anchor metadata";
    let Some(file) = openat_optional(directory.as_raw_fd(), name, libc::O_RDWR, LABEL)? else {
        return Ok((None, None, None, None));
    };
    let configuration = configuration.ok_or_else(|| {
        EngineError::Internal(
            "persisted signer state anchor metadata requires all manifest-pinned anchor \
             configuration values"
                .to_string(),
        )
    })?;
    validate_owned_unlinked_regular(&file, LABEL)?;
    set_owner_only_permissions(&file, LABEL)?;
    validate_secure_regular_file(&file, LABEL)?;
    let bytes = read_file_at(&file, LABEL)?;
    let metadata =
        parse_state_anchor_metadata(&bytes, store_fingerprint, configuration, certified_floors)?;
    let identity = descriptor_identity(&file, LABEL)?;
    Ok((Some(file), Some(identity), Some(metadata), Some(bytes)))
}

#[cfg(unix)]
fn open_state_anchor_trust_journal(
    directory: &fs::File,
    name: &OsStr,
    store_fingerprint: &[u8; 32],
    required: bool,
) -> Result<OpenedStateAnchorTrustJournal, EngineError> {
    const LABEL: &str = "state-anchor trust journal";
    let Some(file) = openat_optional(directory.as_raw_fd(), name, libc::O_RDWR, LABEL)? else {
        if required {
            return Err(EngineError::StateAnchorTrustHeadAbsent);
        }
        return Ok((None, None, None, None, None));
    };
    validate_secure_regular_file(&file, LABEL)?;
    let metadata = file
        .metadata()
        .map_err(|error| EngineError::Internal(format!("failed to stat {LABEL}: {error}")))?;
    let length = usize::try_from(metadata.len())
        .map_err(|_| EngineError::Internal(format!("{LABEL} length does not fit this platform")))?;
    if length > STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH {
        return Err(EngineError::Internal(format!(
            "{LABEL} exceeds its maximum durable length"
        )));
    }
    let identity = descriptor_identity(&file, LABEL)?;
    let before = file_change_stamp(&file, LABEL)?;
    let bytes = read_file_at(&file, LABEL)?;
    let parsed = parse_state_anchor_trust_journal(&bytes, store_fingerprint)?;
    let after = file_change_stamp(&file, LABEL)?;
    if before != after {
        return Err(EngineError::Internal(
            "state-anchor trust journal changed while it was being opened".to_string(),
        ));
    }
    Ok((
        Some(file),
        Some(identity),
        Some(parsed),
        Some(after),
        Some(bytes),
    ))
}

#[cfg(unix)]
fn open_state_anchor_trust_transition_intent(
    directory: &fs::File,
    name: &OsStr,
    store_fingerprint: &[u8; 32],
) -> Result<Option<Vec<u8>>, EngineError> {
    const LABEL: &str = "state-anchor trust transition intent";
    let Some(file) = openat_optional(directory.as_raw_fd(), name, libc::O_RDONLY, LABEL)? else {
        return Ok(None);
    };
    validate_owned_unlinked_regular(&file, LABEL)?;
    validate_secure_regular_file(&file, LABEL)?;
    let length = usize::try_from(
        file.metadata()
            .map_err(|error| EngineError::Internal(format!("failed to stat {LABEL}: {error}")))?
            .len(),
    )
    .map_err(|_| EngineError::Internal(format!("{LABEL} length does not fit this platform")))?;
    if length > STATE_ANCHOR_TRUST_MAX_INTENT_LENGTH {
        return Err(EngineError::Internal(format!(
            "{LABEL} exceeds its maximum durable length"
        )));
    }
    let bytes = read_file_at(&file, LABEL)?;
    parse_state_anchor_trust_transition_intent(&bytes, store_fingerprint)?;
    Ok(Some(bytes))
}

fn encode_state_anchor_acknowledgement(acknowledgement: &StateAnchorAcknowledgement) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH);
    bytes.extend_from_slice(&acknowledgement.binding_hash);
    bytes.extend_from_slice(&acknowledgement.request_digest);
    bytes.extend_from_slice(&acknowledgement.nonce);
    bytes.push(acknowledgement.status);
    bytes.extend_from_slice(&[0u8; 7]);
    bytes.extend_from_slice(&acknowledgement.service_epoch.to_be_bytes());
    bytes.extend_from_slice(&acknowledgement.revision.to_be_bytes());
    bytes.extend_from_slice(&acknowledgement.previous_event_root);
    bytes.extend_from_slice(&acknowledgement.event_root);
    bytes.extend_from_slice(&acknowledgement.checkpoint_store_fingerprint);
    bytes.extend_from_slice(&acknowledgement.checkpoint_generation.to_be_bytes());
    bytes.extend_from_slice(&acknowledgement.checkpoint_previous_commitment);
    bytes.extend_from_slice(&acknowledgement.checkpoint_state_image_digest);
    bytes.extend_from_slice(&acknowledgement.checkpoint_state_commitment);
    bytes.extend_from_slice(&acknowledgement.operation_id);
    bytes.extend_from_slice(&acknowledgement.transition_digest);
    bytes.extend_from_slice(&acknowledgement.committed_at_unix_ms.to_be_bytes());
    bytes.extend_from_slice(&acknowledgement.expires_at_unix_ms.to_be_bytes());
    bytes.extend_from_slice(&acknowledgement.signing_digest);
    bytes.extend_from_slice(&acknowledgement.signature);
    bytes.extend_from_slice(&acknowledgement.configured_spki_hash);
    bytes.extend_from_slice(&acknowledgement.acknowledgement_digest);
    debug_assert_eq!(bytes.len(), TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH);
    bytes
}

fn decode_state_anchor_acknowledgement(
    bytes: &[u8],
) -> Result<StateAnchorAcknowledgement, EngineError> {
    if bytes.len() != TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH {
        return Err(EngineError::Internal(format!(
            "persisted state-anchor acknowledgement has invalid length [{}]",
            bytes.len()
        )));
    }
    let mut offset = 0usize;
    let binding_hash = take_fixed::<32>(bytes, &mut offset, "anchor binding hash")?;
    let request_digest = take_fixed::<32>(bytes, &mut offset, "anchor request digest")?;
    let nonce = take_fixed::<32>(bytes, &mut offset, "anchor nonce")?;
    let status = take_fixed::<1>(bytes, &mut offset, "anchor status")?[0];
    if take_fixed::<7>(bytes, &mut offset, "anchor reserved bytes")? != [0u8; 7] {
        return Err(EngineError::Internal(
            "persisted state-anchor acknowledgement reserved bytes are nonzero".to_string(),
        ));
    }
    let service_epoch =
        u64::from_be_bytes(take_fixed::<8>(bytes, &mut offset, "anchor service epoch")?);
    let revision = u64::from_be_bytes(take_fixed::<8>(bytes, &mut offset, "anchor revision")?);
    let previous_event_root = take_fixed::<32>(bytes, &mut offset, "anchor previous event root")?;
    let event_root = take_fixed::<32>(bytes, &mut offset, "anchor event root")?;
    let checkpoint_store_fingerprint =
        take_fixed::<32>(bytes, &mut offset, "anchor checkpoint store fingerprint")?;
    let checkpoint_generation = u64::from_be_bytes(take_fixed::<8>(
        bytes,
        &mut offset,
        "anchor checkpoint generation",
    )?);
    let checkpoint_previous_commitment =
        take_fixed::<32>(bytes, &mut offset, "anchor checkpoint previous commitment")?;
    let checkpoint_state_image_digest =
        take_fixed::<32>(bytes, &mut offset, "anchor checkpoint state image digest")?;
    let checkpoint_state_commitment =
        take_fixed::<32>(bytes, &mut offset, "anchor checkpoint state commitment")?;
    let operation_id = take_fixed::<32>(bytes, &mut offset, "anchor operation ID")?;
    let transition_digest = take_fixed::<32>(bytes, &mut offset, "anchor transition digest")?;
    let committed_at_unix_ms = u64::from_be_bytes(take_fixed::<8>(
        bytes,
        &mut offset,
        "anchor committed timestamp",
    )?);
    let expires_at_unix_ms = u64::from_be_bytes(take_fixed::<8>(
        bytes,
        &mut offset,
        "anchor expiration timestamp",
    )?);
    let signing_digest = take_fixed::<32>(bytes, &mut offset, "anchor signing digest")?;
    let signature = take_fixed::<64>(bytes, &mut offset, "anchor signature")?;
    let configured_spki_hash = take_fixed::<32>(bytes, &mut offset, "anchor configured SPKI hash")?;
    let acknowledgement_digest =
        take_fixed::<32>(bytes, &mut offset, "anchor acknowledgement digest")?;
    debug_assert_eq!(offset, bytes.len());
    Ok(StateAnchorAcknowledgement {
        binding_hash,
        request_digest,
        nonce,
        status,
        service_epoch,
        revision,
        previous_event_root,
        event_root,
        checkpoint_store_fingerprint,
        checkpoint_generation,
        checkpoint_previous_commitment,
        checkpoint_state_image_digest,
        checkpoint_state_commitment,
        operation_id,
        transition_digest,
        committed_at_unix_ms,
        expires_at_unix_ms,
        signing_digest,
        signature,
        configured_spki_hash,
        acknowledgement_digest,
    })
}

fn encode_state_anchor_metadata(
    store_fingerprint: &[u8; 32],
    metadata: &StateAnchorMetadata,
) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(TBTC_SIGNER_STATE_ANCHOR_METADATA_LENGTH);
    bytes.extend_from_slice(TBTC_SIGNER_STATE_ANCHOR_MAGIC);
    bytes.extend_from_slice(&TBTC_SIGNER_STATE_ANCHOR_VERSION.to_be_bytes());
    bytes.extend_from_slice(store_fingerprint);
    let flags = u8::from(metadata.witness_base.is_some())
        | (u8::from(metadata.pending_witness_base.is_some()) << 1);
    bytes.push(flags);
    bytes.extend_from_slice(&[0u8; 3]);
    bytes.extend_from_slice(&encode_state_anchor_acknowledgement(&metadata.latest));
    match metadata.witness_base.as_ref() {
        Some(base) => bytes.extend_from_slice(&encode_state_anchor_acknowledgement(base)),
        None => bytes.extend_from_slice(&vec![0u8; TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH]),
    }
    match metadata.pending_witness_base.as_ref() {
        Some(base) => bytes.extend_from_slice(&encode_state_anchor_acknowledgement(base)),
        None => bytes.extend_from_slice(&vec![0u8; TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH]),
    }
    let commitment = state_anchor_metadata_commitment(&bytes);
    bytes.extend_from_slice(&commitment);
    debug_assert_eq!(bytes.len(), TBTC_SIGNER_STATE_ANCHOR_METADATA_LENGTH);
    bytes
}

fn parse_state_anchor_metadata(
    bytes: &[u8],
    expected_store_fingerprint: &[u8; 32],
    configuration: &StateAnchorConfiguration,
    certified_floors: &[StateAnchorTrustReferenceModel],
) -> Result<StateAnchorMetadata, EngineError> {
    if bytes.len() != TBTC_SIGNER_STATE_ANCHOR_METADATA_LENGTH {
        return Err(EngineError::Internal(format!(
            "signer state anchor metadata has invalid length [{}], expected [{}]",
            bytes.len(),
            TBTC_SIGNER_STATE_ANCHOR_METADATA_LENGTH
        )));
    }
    let committed_prefix = &bytes[..bytes.len() - 32];
    let expected_commitment = state_anchor_metadata_commitment(committed_prefix);
    if bytes[bytes.len() - 32..] != expected_commitment {
        return Err(EngineError::Internal(
            "signer state anchor metadata commitment is invalid".to_string(),
        ));
    }
    if &bytes[..16] != TBTC_SIGNER_STATE_ANCHOR_MAGIC
        || u32::from_be_bytes(bytes[16..20].try_into().expect("fixed anchor version"))
            != TBTC_SIGNER_STATE_ANCHOR_VERSION
        || &bytes[20..52] != expected_store_fingerprint
    {
        return Err(EngineError::Internal(
            "signer state anchor metadata header or store fingerprint is invalid".to_string(),
        ));
    }
    let flags = bytes[52];
    if flags & !0b11 != 0 {
        return Err(EngineError::Internal(
            "signer state anchor metadata flags are invalid".to_string(),
        ));
    }
    let base_present = flags & 1 != 0;
    let pending_base_present = flags & 2 != 0;
    if bytes[53..56] != [0u8; 3] {
        return Err(EngineError::Internal(
            "signer state anchor metadata reserved bytes are nonzero".to_string(),
        ));
    }
    let latest_end = 56 + TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH;
    let base_end = latest_end + TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH;
    let pending_base_end = base_end + TBTC_SIGNER_STATE_ANCHOR_ACK_LENGTH;
    let latest = decode_state_anchor_acknowledgement(&bytes[56..latest_end])?;
    let base_bytes = &bytes[latest_end..base_end];
    let witness_base = if base_present {
        Some(decode_state_anchor_acknowledgement(base_bytes)?)
    } else {
        if base_bytes.iter().any(|byte| *byte != 0) {
            return Err(EngineError::Internal(
                "signer state anchor metadata has an unflagged witness-base value".to_string(),
            ));
        }
        None
    };
    let pending_base_bytes = &bytes[base_end..pending_base_end];
    let pending_witness_base = if pending_base_present {
        Some(decode_state_anchor_acknowledgement(pending_base_bytes)?)
    } else {
        if pending_base_bytes.iter().any(|byte| *byte != 0) {
            return Err(EngineError::Internal(
                "signer state anchor metadata has an unflagged pending witness-base value"
                    .to_string(),
            ));
        }
        None
    };
    validate_persisted_state_anchor_acknowledgement(&latest, configuration)?;
    validate_anchor_acknowledgement_shape(&latest, expected_store_fingerprint)?;
    validate_persisted_anchor_parent(&latest, certified_floors)?;
    if let Some(base) = witness_base.as_ref() {
        validate_persisted_state_anchor_acknowledgement(base, configuration)?;
        validate_anchor_acknowledgement_shape(base, expected_store_fingerprint)?;
        validate_persisted_anchor_parent(base, certified_floors)?;
        if base.service_epoch != latest.service_epoch
            || base.revision > latest.revision
            || (base.revision == latest.revision && base != &latest)
            || (base.revision.checked_add(1) == Some(latest.revision)
                && latest.previous_event_root != base.event_root)
        {
            return Err(EngineError::Internal(
                "signer state anchor witness-base acknowledgement is inconsistent with the \
                 latest acknowledgement"
                    .to_string(),
            ));
        }
    }
    if let Some(pending) = pending_witness_base.as_ref() {
        validate_persisted_state_anchor_acknowledgement(pending, configuration)?;
        validate_anchor_acknowledgement_shape(pending, expected_store_fingerprint)?;
        validate_persisted_anchor_parent(pending, certified_floors)?;
        if pending != &latest {
            return Err(EngineError::Internal(
                "pending signer state witness base is not the latest accepted acknowledgement"
                    .to_string(),
            ));
        }
    }
    Ok(StateAnchorMetadata {
        latest,
        witness_base,
        pending_witness_base,
    })
}

fn validate_persisted_anchor_parent(
    acknowledgement: &StateAnchorAcknowledgement,
    certified_floors: &[StateAnchorTrustReferenceModel],
) -> Result<(), EngineError> {
    let ordinary = (acknowledgement.revision == 1
        && acknowledgement.previous_event_root == [0u8; 32])
        || (acknowledgement.revision > 1 && acknowledgement.previous_event_root != [0u8; 32]);
    let certified = acknowledgement.revision == 1
        && acknowledgement.previous_event_root != [0u8; 32]
        && certified_floors
            .iter()
            .any(|floor| floor.matches_acknowledgement(acknowledgement));
    if ordinary || certified {
        return Ok(());
    }
    Err(EngineError::Internal(
        "persisted revision-1 state-anchor acknowledgement lacks its exact offline-certified \
         epoch-genesis trust record"
            .to_string(),
    ))
}

fn validate_anchor_acknowledgement_shape(
    acknowledgement: &StateAnchorAcknowledgement,
    expected_store_fingerprint: &[u8; 32],
) -> Result<(), EngineError> {
    if !matches!(acknowledgement.status, 1 | 2)
        || acknowledgement.service_epoch == 0
        || acknowledgement.revision == 0
        || acknowledgement.event_root == [0u8; 32]
        || acknowledgement.request_digest == [0u8; 32]
        || acknowledgement.nonce == [0u8; 32]
        || acknowledgement.checkpoint_store_fingerprint != *expected_store_fingerprint
        || acknowledgement.checkpoint_generation == 0
        || acknowledgement.checkpoint_state_image_digest == [0u8; 32]
        || acknowledgement.checkpoint_state_commitment == [0u8; 32]
        || acknowledgement.checkpoint_state_commitment
            != state_commitment(
                expected_store_fingerprint,
                acknowledgement.checkpoint_generation,
                &acknowledgement.checkpoint_previous_commitment,
                &acknowledgement.checkpoint_state_image_digest,
            )
        || acknowledgement.operation_id == [0u8; 32]
        || acknowledgement.transition_digest == [0u8; 32]
    {
        return Err(EngineError::Internal(
            "persisted state-anchor acknowledgement has invalid structural fields".to_string(),
        ));
    }
    Ok(())
}

fn validate_anchor_monotonic_update(
    existing: Option<&StateAnchorMetadata>,
    acknowledgement: &StateAnchorAcknowledgement,
    allow_unanchored_recovery: bool,
) -> Result<bool, EngineError> {
    let Some(existing) = existing else {
        let is_first_revision =
            acknowledgement.revision == 1 && acknowledgement.previous_event_root == [0u8; 32];
        let is_recovery_revision = allow_unanchored_recovery
            && acknowledgement.revision > 1
            && acknowledgement.previous_event_root != [0u8; 32];
        if !is_first_revision && !is_recovery_revision {
            return Err(EngineError::Validation(
                "first state-anchor acknowledgement must have revision 1 and a zero \
                 previousEventRoot unless admitted by a fresh recovery response"
                    .to_string(),
            ));
        }
        return Ok(false);
    };
    let latest = &existing.latest;
    if acknowledgement.service_epoch != latest.service_epoch {
        return Err(EngineError::Validation(
            "state-anchor service epoch changed; an offline recovery certificate is required"
                .to_string(),
        ));
    }
    if acknowledgement.revision == latest.revision {
        if acknowledgement != latest {
            return Err(EngineError::Validation(
                "state-anchor acknowledgement equivocates at an already accepted revision"
                    .to_string(),
            ));
        }
        return Ok(true);
    }
    let expected_revision = latest.revision.checked_add(1).ok_or_else(|| {
        EngineError::Internal("state-anchor service revision exhausted u64".to_string())
    })?;
    if acknowledgement.revision != expected_revision
        || acknowledgement.previous_event_root != latest.event_root
    {
        return Err(EngineError::Validation(format!(
            "state-anchor acknowledgement must extend revision [{}] and its event root exactly",
            latest.revision
        )));
    }
    Ok(false)
}

fn validate_anchor_history(
    anchor: Option<&StateAnchorMetadata>,
    history: &[StateWitness],
) -> Result<(), EngineError> {
    let Some(anchor) = anchor else {
        return Ok(());
    };
    let base_generation = history
        .first()
        .map(|entry| entry.generation)
        .ok_or_else(|| {
            EngineError::Internal(
                "state witness history is empty while validating signed anchor metadata"
                    .to_string(),
            )
        })?;
    let acknowledgement_is_retained = |acknowledgement: &StateAnchorAcknowledgement| -> bool {
        acknowledgement
            .checkpoint_generation
            .checked_sub(base_generation)
            .and_then(|offset| usize::try_from(offset).ok())
            .and_then(|index| history.get(index))
            .is_some_and(|entry| {
                entry.generation == acknowledgement.checkpoint_generation
                    && entry.previous_commitment == acknowledgement.checkpoint_previous_commitment
                    && entry.state_image_digest == acknowledgement.checkpoint_state_image_digest
                    && entry.commitment == acknowledgement.checkpoint_state_commitment
            })
    };
    if !acknowledgement_is_retained(&anchor.latest) {
        return Err(EngineError::Internal(
            "latest signed state-anchor checkpoint is not present in the active witness \
             segment"
                .to_string(),
        ));
    }
    if let Some(pending) = anchor.pending_witness_base.as_ref() {
        if !acknowledgement_is_retained(pending) {
            return Err(EngineError::Internal(
                "pending signed state-anchor witness base is not present in the active witness \
                 segment"
                    .to_string(),
            ));
        }
    }
    let pending_is_active_base = anchor.pending_witness_base.as_ref().is_some_and(|pending| {
        history.first().is_some_and(|entry| {
            entry.generation == pending.checkpoint_generation
                && entry.previous_commitment == pending.checkpoint_previous_commitment
                && entry.state_image_digest == pending.checkpoint_state_image_digest
                && entry.commitment == pending.checkpoint_state_commitment
        })
    });
    if let Some(base) = anchor.witness_base.as_ref() {
        if !pending_is_active_base && !acknowledgement_is_retained(base) {
            return Err(EngineError::Internal(
                "signed state-anchor witness base is not present in the active witness segment"
                    .to_string(),
            ));
        }
    }
    Ok(())
}

fn state_anchor_metadata_commitment(bytes: &[u8]) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(TBTC_SIGNER_STATE_ANCHOR_METADATA_DOMAIN);
    digest.update(bytes);
    digest.finalize().into()
}

fn take_fixed<const N: usize>(
    bytes: &[u8],
    offset: &mut usize,
    label: &str,
) -> Result<[u8; N], EngineError> {
    let end = offset
        .checked_add(N)
        .ok_or_else(|| EngineError::Internal(format!("persisted {label} offset overflowed")))?;
    let value: [u8; N] = bytes
        .get(*offset..end)
        .ok_or_else(|| EngineError::Internal(format!("persisted {label} is truncated")))?
        .try_into()
        .expect("range length is fixed");
    *offset = end;
    Ok(value)
}

#[cfg(unix)]
fn replace_state_anchor_entry(
    directory: &fs::File,
    name: &OsStr,
    bytes: &[u8],
) -> Result<(fs::File, OpenedObjectIdentity), EngineError> {
    replace_durable_entry(directory, name, bytes, "signer state anchor metadata")
}

#[cfg(unix)]
fn replace_durable_entry(
    directory: &fs::File,
    name: &OsStr,
    bytes: &[u8],
    label: &str,
) -> Result<(fs::File, OpenedObjectIdentity), EngineError> {
    replace_durable_entry_with_guard(directory, name, bytes, label, None)
}

#[cfg(unix)]
fn replace_durable_entry_with_guard(
    directory: &fs::File,
    name: &OsStr,
    bytes: &[u8],
    label: &str,
    recovery_guard: Option<&StateAnchorTrustRecoveryGuard<'_>>,
) -> Result<(fs::File, OpenedObjectIdentity), EngineError> {
    let temp_name = unique_temp_name(name)?;
    let temp_file = openat_regular(
        directory.as_raw_fd(),
        &temp_name,
        libc::O_RDWR | libc::O_CREAT | libc::O_EXCL,
        0o600,
        label,
    )?;
    let outcome = (|| {
        validate_owned_unlinked_regular(&temp_file, label)?;
        set_owner_only_permissions(&temp_file, label)?;
        validate_secure_regular_file(&temp_file, label)?;
        write_file_at(&temp_file, bytes, label)?;
        temp_file.sync_all().map_err(|error| {
            EngineError::Internal(format!("failed to sync new {label}: {error}"))
        })?;
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
        renameat_same_directory(directory.as_raw_fd(), &temp_name, name, label)?;
        directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory after publishing {label}: {error}"
            ))
        })?;
        let identity = descriptor_identity(&temp_file, label)?;
        validate_live_entry(directory, name, identity, label)?;
        #[cfg(test)]
        maybe_replace_trust_lock_after_guarded_publication(recovery_guard)?;
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
        Ok(identity)
    })();
    match outcome {
        Ok(identity) => Ok((temp_file, identity)),
        Err(error) => {
            let _ = unlinkat_entry(directory.as_raw_fd(), &temp_name);
            Err(error)
        }
    }
}

fn encode_state_witness_segment_header(
    store_fingerprint: &[u8; 32],
    acknowledgement: &StateAnchorAcknowledgement,
) -> Result<Vec<u8>, EngineError> {
    validate_anchor_acknowledgement_shape(acknowledgement, store_fingerprint)?;
    let mut bytes = Vec::with_capacity(TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH);
    bytes.extend_from_slice(TBTC_SIGNER_STATE_WITNESS_SEGMENT_MAGIC);
    bytes.extend_from_slice(&TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_VERSION.to_be_bytes());
    bytes
        .extend_from_slice(&(TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH as u32).to_be_bytes());
    bytes.extend_from_slice(store_fingerprint);
    bytes.extend_from_slice(&acknowledgement.checkpoint_generation.to_be_bytes());
    bytes.extend_from_slice(&acknowledgement.checkpoint_previous_commitment);
    bytes.extend_from_slice(&acknowledgement.checkpoint_state_image_digest);
    bytes.extend_from_slice(&acknowledgement.checkpoint_state_commitment);
    bytes.extend_from_slice(&acknowledgement.binding_hash);
    bytes.extend_from_slice(&acknowledgement.service_epoch.to_be_bytes());
    bytes.extend_from_slice(&acknowledgement.revision.to_be_bytes());
    bytes.extend_from_slice(&acknowledgement.previous_event_root);
    bytes.extend_from_slice(&acknowledgement.event_root);
    bytes.extend_from_slice(&acknowledgement.operation_id);
    bytes.extend_from_slice(&acknowledgement.transition_digest);
    bytes.extend_from_slice(&acknowledgement.committed_at_unix_ms.to_be_bytes());
    bytes.extend_from_slice(&acknowledgement.acknowledgement_digest);
    bytes.extend_from_slice(&acknowledgement.signature);
    debug_assert_eq!(bytes.len(), 440);
    let mut digest = Sha256::new();
    digest.update(TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_DOMAIN);
    digest.update(&bytes);
    bytes.extend_from_slice(&<[u8; 32]>::from(digest.finalize()));
    debug_assert_eq!(bytes.len(), TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH);
    Ok(bytes)
}

fn parse_state_witness_segment_header(
    bytes: &[u8],
    expected_store_fingerprint: &[u8; 32],
    anchor: Option<&StateAnchorMetadata>,
) -> Result<StateWitnessSegmentHeader, EngineError> {
    if bytes.len() != TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH {
        return Err(EngineError::Internal(format!(
            "state witness segment header has invalid length [{}]",
            bytes.len()
        )));
    }
    if &bytes[..16] != TBTC_SIGNER_STATE_WITNESS_SEGMENT_MAGIC
        || u32::from_be_bytes(bytes[16..20].try_into().expect("fixed segment version"))
            != TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_VERSION
        || u32::from_be_bytes(bytes[20..24].try_into().expect("fixed segment length"))
            != TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH as u32
    {
        return Err(EngineError::Internal(
            "state witness segment header magic, version, or length is invalid".to_string(),
        ));
    }
    let mut offset = 24usize;
    let store_fingerprint =
        take_fixed::<32>(bytes, &mut offset, "witness segment store fingerprint")?;
    let generation = u64::from_be_bytes(take_fixed::<8>(
        bytes,
        &mut offset,
        "witness segment base generation",
    )?);
    let previous_commitment = take_fixed::<32>(
        bytes,
        &mut offset,
        "witness segment base previous commitment",
    )?;
    let state_image_digest =
        take_fixed::<32>(bytes, &mut offset, "witness segment base image digest")?;
    let commitment = take_fixed::<32>(bytes, &mut offset, "witness segment base commitment")?;
    let base = StateWitness {
        generation,
        previous_commitment,
        commitment,
        state_image_digest,
    };
    let binding_hash = take_fixed::<32>(bytes, &mut offset, "witness segment binding hash")?;
    let service_epoch = u64::from_be_bytes(take_fixed::<8>(
        bytes,
        &mut offset,
        "witness segment service epoch",
    )?);
    let revision = u64::from_be_bytes(take_fixed::<8>(
        bytes,
        &mut offset,
        "witness segment revision",
    )?);
    let previous_event_root =
        take_fixed::<32>(bytes, &mut offset, "witness segment previous event root")?;
    let event_root = take_fixed::<32>(bytes, &mut offset, "witness segment event root")?;
    let operation_id = take_fixed::<32>(bytes, &mut offset, "witness segment operation ID")?;
    let transition_digest =
        take_fixed::<32>(bytes, &mut offset, "witness segment transition digest")?;
    let committed_at_unix_ms = u64::from_be_bytes(take_fixed::<8>(
        bytes,
        &mut offset,
        "witness segment committed timestamp",
    )?);
    let acknowledgement_digest =
        take_fixed::<32>(bytes, &mut offset, "witness segment acknowledgement digest")?;
    let signature = take_fixed::<64>(bytes, &mut offset, "witness segment signature")?;
    debug_assert_eq!(offset, 440);
    let header_commitment =
        take_fixed::<32>(bytes, &mut offset, "witness segment header commitment")?;
    let mut digest = Sha256::new();
    digest.update(TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_DOMAIN);
    digest.update(&bytes[..440]);
    let expected_header_commitment: [u8; 32] = digest.finalize().into();
    if header_commitment != expected_header_commitment
        || store_fingerprint != *expected_store_fingerprint
        || generation == 0
        || state_image_digest == [0u8; 32]
        || commitment == [0u8; 32]
        || commitment
            != state_commitment(
                expected_store_fingerprint,
                generation,
                &previous_commitment,
                &state_image_digest,
            )
    {
        return Err(EngineError::Internal(
            "state witness segment header commitment or base is invalid".to_string(),
        ));
    }
    let metadata = anchor.ok_or_else(|| {
        EngineError::Internal(
            "rotated state witness segment has no retained signed base acknowledgement; \
             offline recovery certification is required"
                .to_string(),
        )
    })?;
    let header_matches_acknowledgement = |acknowledgement: &StateAnchorAcknowledgement| {
        acknowledgement.checkpoint_store_fingerprint == store_fingerprint
            && acknowledgement.checkpoint_generation == base.generation
            && acknowledgement.checkpoint_previous_commitment == base.previous_commitment
            && acknowledgement.checkpoint_state_image_digest == base.state_image_digest
            && acknowledgement.checkpoint_state_commitment == base.commitment
            && acknowledgement.binding_hash == binding_hash
            && acknowledgement.service_epoch == service_epoch
            && acknowledgement.revision == revision
            && acknowledgement.previous_event_root == previous_event_root
            && acknowledgement.event_root == event_root
            && acknowledgement.operation_id == operation_id
            && acknowledgement.transition_digest == transition_digest
            && acknowledgement.committed_at_unix_ms == committed_at_unix_ms
            && acknowledgement.acknowledgement_digest == acknowledgement_digest
            && acknowledgement.signature == signature
    };
    if ![
        metadata.witness_base.as_ref(),
        metadata.pending_witness_base.as_ref(),
    ]
    .into_iter()
    .flatten()
    .any(header_matches_acknowledgement)
    {
        return Err(EngineError::Internal(
            "state witness segment header disagrees with every retained signed base \
             acknowledgement"
                .to_string(),
        ));
    }
    Ok(StateWitnessSegmentHeader {
        store_fingerprint,
        base,
        binding_hash,
        service_epoch,
        revision,
        previous_event_root,
        event_root,
        operation_id,
        transition_digest,
        committed_at_unix_ms,
        acknowledgement_digest,
        signature,
        header_commitment,
    })
}

#[cfg(unix)]
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum TrustRecoveryAnchorStage {
    Prior,
    Target,
}

/// Completes every publication boundary authorized by a durable trust
/// transition intent. This runs while the descriptor-bound store lock is held
/// and before ordinary anchor/witness opening, because a crash can leave the
/// rotating endpoint, witness segment, and trust journal at different
/// individually durable stages.
#[cfg(unix)]
#[allow(clippy::too_many_arguments)]
fn recover_state_anchor_trust_transition(
    recovery_guard: &StateAnchorTrustRecoveryGuard<'_>,
    trust_name: &OsStr,
    intent_name: &OsStr,
    anchor_name: &OsStr,
    witness_names: StateWitnessRotationNames<'_>,
    store_identity: &DurableStoreIdentity,
    current_state_file: Option<&fs::File>,
    maximum_records: usize,
    target_configuration: Option<&StateAnchorConfiguration>,
    expected_intent_bytes: &[u8],
    transition: &VerifiedStateAnchorTrustTransition,
) -> Result<(), EngineError> {
    let directory = recovery_guard.directory;
    let target_configuration = target_configuration.ok_or_else(|| {
        EngineError::Internal(
            "durable state-anchor trust recovery requires installed target pins".to_string(),
        )
    })?;
    let final_certificate = transition
        .certificates
        .last()
        .expect("verified trust transition is nonempty");

    revalidate_state_anchor_trust_transition_intent_entry(
        directory,
        intent_name,
        &store_identity.fingerprint,
        expected_intent_bytes,
    )?;
    let (_, _, mut journal, _, mut journal_bytes) =
        open_state_anchor_trust_journal(directory, trust_name, &store_identity.fingerprint, false)?;
    if journal.is_none() {
        let header = encode_state_anchor_trust_journal_header(&store_identity.fingerprint);
        recovery_guard.revalidate()?;
        replace_durable_entry_with_guard(
            directory,
            trust_name,
            &header,
            "state-anchor trust journal during recovery",
            Some(recovery_guard),
        )?;
        journal = Some(parse_state_anchor_trust_journal(
            &header,
            &store_identity.fingerprint,
        )?);
        journal_bytes = Some(header);
    }
    let mut journal = journal.expect("journal initialized above");
    let mut journal_bytes = journal_bytes.expect("journal bytes initialized above");
    validate_trust_recovery_journal(&journal, transition)?;

    // PREPARE every missing certificate before changing the online endpoint.
    // A partial COMMIT implies that the complete PREPARE batch was already
    // durable, which the validation helper enforces.
    loop {
        let (committed_count, prepared_count) =
            trust_recovery_transition_progress(&journal, transition)?;
        if prepared_count == transition.certificates.len() {
            break;
        }
        if committed_count != 0 {
            return Err(EngineError::Internal(
                "state-anchor trust journal committed a partial transition before all PREPARE \
                 records were durable"
                    .to_string(),
            ));
        }
        revalidate_state_anchor_trust_transition_intent_entry(
            directory,
            intent_name,
            &store_identity.fingerprint,
            expected_intent_bytes,
        )?;
        recovery_guard.revalidate()?;
        let certificate = &transition.certificates[prepared_count];
        let record = encode_state_anchor_trust_prepare_record(
            &store_identity.fingerprint,
            &journal.last_record_commitment,
            certificate,
        )?;
        journal_bytes.extend_from_slice(&record);
        let (next_journal, next_bytes) = publish_state_anchor_trust_journal_for_recovery(
            directory,
            trust_name,
            &store_identity.fingerprint,
            &journal_bytes,
            recovery_guard,
        )?;
        journal = next_journal;
        journal_bytes = next_bytes;
    }

    let certified_floors = journal.certified_floors();
    let (prior_anchor, anchor_stage) = open_state_anchor_for_trust_recovery(
        directory,
        anchor_name,
        &store_identity.fingerprint,
        transition,
        &certified_floors,
    )?;
    let target_acknowledgement = final_certificate.target_acknowledgement.clone();
    let rotation_anchor = match (anchor_stage, prior_anchor.as_ref()) {
        (Some(TrustRecoveryAnchorStage::Prior), Some(prior)) => StateAnchorMetadata {
            latest: prior.latest.clone(),
            witness_base: prior.witness_base.clone(),
            pending_witness_base: Some(target_acknowledgement.clone()),
        },
        (Some(TrustRecoveryAnchorStage::Target), Some(_)) | (None, None) => StateAnchorMetadata {
            latest: target_acknowledgement.clone(),
            witness_base: Some(target_acknowledgement.clone()),
            pending_witness_base: Some(target_acknowledgement.clone()),
        },
        _ => {
            return Err(EngineError::Internal(
                "state-anchor trust recovery anchor stage is inconsistent".to_string(),
            ))
        }
    };

    revalidate_state_anchor_trust_transition_intent_entry(
        directory,
        intent_name,
        &store_identity.fingerprint,
        expected_intent_bytes,
    )?;
    if !recover_state_witness_rotation(
        directory,
        witness_names,
        store_identity,
        current_state_file,
        Some(&rotation_anchor),
        maximum_records,
        false,
        Some(recovery_guard),
    )? {
        return Err(EngineError::Internal(
            "state-anchor trust recovery did not publish the certified witness base".to_string(),
        ));
    }

    let target_anchor = StateAnchorMetadata {
        latest: target_acknowledgement.clone(),
        witness_base: Some(target_acknowledgement),
        pending_witness_base: None,
    };
    let target_anchor_bytes =
        encode_state_anchor_metadata(&store_identity.fingerprint, &target_anchor);
    parse_state_anchor_metadata(
        &target_anchor_bytes,
        &store_identity.fingerprint,
        &final_certificate.to.anchor_configuration()?,
        &certified_floors,
    )?;
    revalidate_state_anchor_trust_transition_intent_entry(
        directory,
        intent_name,
        &store_identity.fingerprint,
        expected_intent_bytes,
    )?;
    recovery_guard.revalidate()?;
    replace_durable_entry_with_guard(
        directory,
        anchor_name,
        &target_anchor_bytes,
        "signer state anchor metadata during trust recovery",
        Some(recovery_guard),
    )?;

    // COMMIT each already-prepared certificate with a COW publication. The
    // parser represents a crash after a COMMIT prefix as a committed prefix
    // plus the still-pending tail, so this loop naturally resumes at the exact
    // next certificate.
    loop {
        let (committed_count, prepared_count) =
            trust_recovery_transition_progress(&journal, transition)?;
        if committed_count == transition.certificates.len() {
            break;
        }
        if prepared_count != transition.certificates.len() {
            return Err(EngineError::Internal(
                "state-anchor trust recovery reached COMMIT without a complete PREPARE batch"
                    .to_string(),
            ));
        }
        revalidate_state_anchor_trust_transition_intent_entry(
            directory,
            intent_name,
            &store_identity.fingerprint,
            expected_intent_bytes,
        )?;
        recovery_guard.revalidate()?;
        let certificate = &transition.certificates[committed_count];
        let record = encode_state_anchor_trust_commit_record(
            &store_identity.fingerprint,
            &journal.last_record_commitment,
            certificate,
        )?;
        journal_bytes.extend_from_slice(&record);
        let (next_journal, next_bytes) = publish_state_anchor_trust_journal_for_recovery(
            directory,
            trust_name,
            &store_identity.fingerprint,
            &journal_bytes,
            recovery_guard,
        )?;
        journal = next_journal;
        journal_bytes = next_bytes;
    }

    validate_state_anchor_trust_journal_head(
        &journal,
        target_configuration,
        &store_identity.fingerprint,
    )?;
    revalidate_state_anchor_trust_transition_intent_entry(
        directory,
        intent_name,
        &store_identity.fingerprint,
        expected_intent_bytes,
    )?;
    recovery_guard.revalidate()?;
    unlinkat_entry(directory.as_raw_fd(), witness_names.previous)?;
    directory.sync_all().map_err(|error| {
        EngineError::Internal(format!(
            "failed to sync trust recovery after retiring previous witness: {error}"
        ))
    })?;
    recovery_guard.revalidate()?;
    unlinkat_entry(directory.as_raw_fd(), intent_name)?;
    directory.sync_all().map_err(|error| {
        EngineError::Internal(format!(
            "failed to sync completed state-anchor trust intent removal: {error}"
        ))
    })?;
    recovery_guard.revalidate()?;
    Ok(())
}

#[cfg(unix)]
fn validate_trust_recovery_journal(
    journal: &StateAnchorTrustJournalModel,
    transition: &VerifiedStateAnchorTrustTransition,
) -> Result<(), EngineError> {
    trust_recovery_transition_progress(journal, transition).map(|_| ())
}

/// Returns `(committed_count, prepared_or_committed_count)` within the intent
/// suffix after proving the durable journal contains no conflicting or extra
/// certificate at any suffix position.
#[cfg(unix)]
fn trust_recovery_transition_progress(
    journal: &StateAnchorTrustJournalModel,
    transition: &VerifiedStateAnchorTrustTransition,
) -> Result<(usize, usize), EngineError> {
    let first = transition
        .certificates
        .first()
        .expect("verified trust transition is nonempty");
    let prior_count = usize::try_from(first.certificate_sequence - 1).map_err(|_| {
        EngineError::Internal(
            "state-anchor trust recovery sequence does not fit this platform".to_string(),
        )
    })?;
    if journal.committed.len() < prior_count {
        return Err(EngineError::Internal(
            "state-anchor trust recovery journal is missing the intent predecessor".to_string(),
        ));
    }
    if prior_count > 0
        && journal.committed[prior_count - 1].certificate_digest
            != first.previous_certificate_digest
    {
        return Err(EngineError::Internal(
            "state-anchor trust recovery intent does not extend the durable predecessor"
                .to_string(),
        ));
    }
    if journal.committed.len()
        > prior_count
            .checked_add(transition.certificates.len())
            .ok_or_else(|| {
                EngineError::Internal(
                    "state-anchor trust recovery certificate count overflowed".to_string(),
                )
            })?
    {
        return Err(EngineError::Internal(
            "state-anchor trust recovery journal is ahead of its durable intent".to_string(),
        ));
    }
    let committed_count = journal.committed.len() - prior_count;
    for (index, certificate) in journal.committed[prior_count..].iter().enumerate() {
        if certificate.wire != transition.certificates[index].wire {
            return Err(EngineError::Internal(
                "state-anchor trust recovery committed certificate conflicts with its intent"
                    .to_string(),
            ));
        }
    }
    if committed_count
        .checked_add(journal.pending.len())
        .is_none_or(|count| count > transition.certificates.len())
    {
        return Err(EngineError::Internal(
            "state-anchor trust recovery journal has an extra PREPARE certificate".to_string(),
        ));
    }
    for (offset, certificate) in journal.pending.iter().enumerate() {
        if certificate.wire != transition.certificates[committed_count + offset].wire {
            return Err(EngineError::Internal(
                "state-anchor trust recovery PREPARE certificate conflicts with its intent"
                    .to_string(),
            ));
        }
    }
    if committed_count != 0
        && committed_count + journal.pending.len() != transition.certificates.len()
    {
        return Err(EngineError::Internal(
            "state-anchor trust recovery has a partial COMMIT without the complete PREPARE batch"
                .to_string(),
        ));
    }
    Ok((committed_count, committed_count + journal.pending.len()))
}

#[cfg(unix)]
fn publish_state_anchor_trust_journal_for_recovery(
    directory: &fs::File,
    name: &OsStr,
    store_fingerprint: &[u8; 32],
    bytes: &[u8],
    recovery_guard: &StateAnchorTrustRecoveryGuard<'_>,
) -> Result<(StateAnchorTrustJournalModel, Vec<u8>), EngineError> {
    if bytes.len() > STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH {
        return Err(EngineError::Internal(
            "state-anchor trust recovery journal exceeds its durable bound".to_string(),
        ));
    }
    let parsed = parse_state_anchor_trust_journal(bytes, store_fingerprint)?;
    replace_durable_entry_with_guard(
        directory,
        name,
        bytes,
        "state-anchor trust journal during recovery",
        Some(recovery_guard),
    )?;
    Ok((parsed, bytes.to_vec()))
}

#[cfg(unix)]
fn open_state_anchor_for_trust_recovery(
    directory: &fs::File,
    name: &OsStr,
    store_fingerprint: &[u8; 32],
    transition: &VerifiedStateAnchorTrustTransition,
    certified_floors: &[StateAnchorTrustReferenceModel],
) -> Result<
    (
        Option<StateAnchorMetadata>,
        Option<TrustRecoveryAnchorStage>,
    ),
    EngineError,
> {
    const LABEL: &str = "signer state anchor metadata during trust recovery";
    let Some(file) = openat_optional(directory.as_raw_fd(), name, libc::O_RDWR, LABEL)? else {
        let first = transition
            .certificates
            .first()
            .expect("verified trust transition is nonempty");
        if first.from.is_some() {
            return Err(EngineError::Internal(
                "rotation/adoption trust recovery is missing its persisted prior anchor"
                    .to_string(),
            ));
        }
        return Ok((None, None));
    };
    validate_owned_unlinked_regular(&file, LABEL)?;
    validate_secure_regular_file(&file, LABEL)?;
    let bytes = read_file_at(&file, LABEL)?;
    let first = transition
        .certificates
        .first()
        .expect("verified trust transition is nonempty");
    let final_certificate = transition
        .certificates
        .last()
        .expect("verified trust transition is nonempty");

    if let Some(from) = first.from.as_ref() {
        if let Ok(metadata) = parse_state_anchor_metadata(
            &bytes,
            store_fingerprint,
            &from.anchor_configuration()?,
            certified_floors,
        ) {
            if from.reference.matches_acknowledgement(&metadata.latest) {
                return Ok((Some(metadata), Some(TrustRecoveryAnchorStage::Prior)));
            }
        }
    }
    if let Ok(metadata) = parse_state_anchor_metadata(
        &bytes,
        store_fingerprint,
        &final_certificate.to.anchor_configuration()?,
        certified_floors,
    ) {
        if metadata.latest == final_certificate.target_acknowledgement
            && metadata.witness_base.as_ref() == Some(&final_certificate.target_acknowledgement)
            && metadata.pending_witness_base.is_none()
        {
            return Ok((Some(metadata), Some(TrustRecoveryAnchorStage::Target)));
        }
    }
    Err(EngineError::Internal(
        "persisted state anchor matches neither the certified prior nor target recovery stage"
            .to_string(),
    ))
}

#[cfg(unix)]
fn revalidate_state_anchor_trust_transition_intent_entry(
    directory: &fs::File,
    name: &OsStr,
    store_fingerprint: &[u8; 32],
    expected_bytes: &[u8],
) -> Result<(), EngineError> {
    let live = open_state_anchor_trust_transition_intent(directory, name, store_fingerprint)?
        .ok_or_else(|| {
            EngineError::Internal(
                "state-anchor trust transition intent disappeared during recovery".to_string(),
            )
        })?;
    if live != expected_bytes {
        return Err(EngineError::Internal(
            "state-anchor trust transition intent changed during recovery".to_string(),
        ));
    }
    Ok(())
}

#[cfg(unix)]
#[allow(clippy::too_many_arguments)]
fn recover_state_witness_rotation(
    directory: &fs::File,
    names: StateWitnessRotationNames<'_>,
    store_identity: &DurableStoreIdentity,
    current_state_file: Option<&fs::File>,
    anchor: Option<&StateAnchorMetadata>,
    maximum_records: usize,
    retire_previous: bool,
    recovery_guard: Option<&StateAnchorTrustRecoveryGuard<'_>>,
) -> Result<bool, EngineError> {
    let StateWitnessRotationNames {
        current: current_name,
        next: next_name,
        previous: previous_name,
    } = names;
    let current_exists =
        live_entry_stat(directory.as_raw_fd(), current_name, "state witness journal")?.is_some();
    let mut next_exists = live_entry_stat(
        directory.as_raw_fd(),
        next_name,
        "next state witness journal",
    )?
    .is_some();
    let mut previous_exists = live_entry_stat(
        directory.as_raw_fd(),
        previous_name,
        "previous state witness journal",
    )?
    .is_some();
    let pending = anchor.and_then(|metadata| metadata.pending_witness_base.as_ref());
    if !next_exists && !previous_exists {
        let Some(pending) = pending else {
            return Ok(false);
        };
        if !current_exists {
            return Err(EngineError::Internal(
                "pending signed state witness rotation has no current journal".to_string(),
            ));
        }
        let current_file = openat_optional(
            directory.as_raw_fd(),
            current_name,
            libc::O_RDWR,
            "current state witness journal",
        )?
        .ok_or_else(|| {
            EngineError::Internal(
                "current state witness journal disappeared during rotation recovery".to_string(),
            )
        })?;
        validate_secure_regular_file(&current_file, "current state witness journal")?;
        let parsed = read_state_witness_journal_streaming(
            &current_file,
            &store_identity.store_id,
            &store_identity.fingerprint,
            maximum_records,
            anchor,
        )?;
        validate_anchor_history(anchor, &parsed.history)?;
        if parsed.pending.is_some() {
            return Err(EngineError::Internal(
                "cannot resume state witness rotation while a state update is pending".to_string(),
            ));
        }
        let current_digest = current_state_image_digest(current_state_file)?;
        let tip = parsed.history.last().ok_or_else(|| {
            EngineError::Internal(
                "current state witness journal has no committed tip during rotation recovery"
                    .to_string(),
            )
        })?;
        if tip.state_image_digest != current_digest {
            return Err(EngineError::Internal(
                "pending state witness rotation does not commit the current state image"
                    .to_string(),
            ));
        }
        if parsed.length == parsed.header_length
            && parsed.segment_header.as_ref().is_some_and(|header| {
                state_witness_segment_header_matches_acknowledgement(header, pending)
            })
        {
            // Publication and old-segment retirement completed; only promotion
            // of the already-verified pending base remains.
            return Ok(true);
        }
        if !acknowledgement_matches_witness(pending, Some(tip)) {
            return Err(EngineError::Internal(
                "pending signed state witness rotation no longer matches the current tip"
                    .to_string(),
            ));
        }
        let bytes = encode_state_witness_segment_header(&store_identity.fingerprint, pending)?;
        create_entry_atomically_with_guard(
            directory,
            next_name,
            &bytes,
            "next signer state witness journal during recovery",
            recovery_guard,
        )?;
        next_exists = true;
    }
    let pending = pending.ok_or_else(|| {
        EngineError::Internal(
            "state witness rotation artifacts exist without a pending signed base".to_string(),
        )
    })?;

    if next_exists && current_exists && !previous_exists {
        validate_rotation_candidate(
            directory,
            next_name,
            store_identity,
            anchor,
            maximum_records,
        )?;
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
        renameat_same_directory(
            directory.as_raw_fd(),
            current_name,
            previous_name,
            "retain previous state witness journal during recovery",
        )?;
        directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync state witness recovery after retaining previous segment: {error}"
            ))
        })?;
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
        renameat_same_directory(
            directory.as_raw_fd(),
            next_name,
            current_name,
            "publish recovered state witness segment",
        )?;
        directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync state witness recovery after publishing segment: {error}"
            ))
        })?;
        validate_rotation_candidate(
            directory,
            current_name,
            store_identity,
            anchor,
            maximum_records,
        )?;
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
        previous_exists = true;
    } else if next_exists && !current_exists && previous_exists {
        validate_rotation_candidate(
            directory,
            next_name,
            store_identity,
            anchor,
            maximum_records,
        )?;
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
        renameat_same_directory(
            directory.as_raw_fd(),
            next_name,
            current_name,
            "publish recovered state witness segment",
        )?;
        directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync recovered state witness segment publication: {error}"
            ))
        })?;
        validate_rotation_candidate(
            directory,
            current_name,
            store_identity,
            anchor,
            maximum_records,
        )?;
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
    } else if !next_exists && current_exists && previous_exists {
        validate_rotation_candidate(
            directory,
            current_name,
            store_identity,
            anchor,
            maximum_records,
        )?;
    } else {
        return Err(EngineError::Internal(
            "ambiguous state witness rotation entries; refusing to discard either segment"
                .to_string(),
        ));
    }

    let parsed = validate_rotation_candidate(
        directory,
        current_name,
        store_identity,
        anchor,
        maximum_records,
    )?;
    if !acknowledgement_matches_witness(pending, parsed.history.first()) {
        return Err(EngineError::Internal(
            "recovered state witness segment does not use the pending signed base; retaining \
             the previous segment"
                .to_string(),
        ));
    }
    let current_digest = current_state_image_digest(current_state_file)?;
    if parsed
        .history
        .last()
        .map(|tip| tip.state_image_digest != current_digest)
        .unwrap_or(true)
    {
        return Err(EngineError::Internal(
            "recovered state witness segment does not commit the current state image; \
             retaining the previous segment"
                .to_string(),
        ));
    }
    if previous_exists && retire_previous {
        if let Some(guard) = recovery_guard {
            guard.revalidate()?;
        }
        unlinkat_entry(directory.as_raw_fd(), previous_name)?;
    }
    directory.sync_all().map_err(|error| {
        EngineError::Internal(format!(
            "failed to sync state witness recovery after finalizing segment publication: {error}"
        ))
    })?;
    if let Some(guard) = recovery_guard {
        guard.revalidate()?;
    }
    Ok(true)
}

fn acknowledgement_matches_witness(
    acknowledgement: &StateAnchorAcknowledgement,
    witness: Option<&StateWitness>,
) -> bool {
    witness.is_some_and(|witness| {
        acknowledgement.checkpoint_generation == witness.generation
            && acknowledgement.checkpoint_previous_commitment == witness.previous_commitment
            && acknowledgement.checkpoint_state_image_digest == witness.state_image_digest
            && acknowledgement.checkpoint_state_commitment == witness.commitment
    })
}

fn state_witness_segment_header_matches_acknowledgement(
    header: &StateWitnessSegmentHeader,
    acknowledgement: &StateAnchorAcknowledgement,
) -> bool {
    acknowledgement.checkpoint_store_fingerprint == header.store_fingerprint
        && acknowledgement.checkpoint_generation == header.base.generation
        && acknowledgement.checkpoint_previous_commitment == header.base.previous_commitment
        && acknowledgement.checkpoint_state_image_digest == header.base.state_image_digest
        && acknowledgement.checkpoint_state_commitment == header.base.commitment
        && acknowledgement.binding_hash == header.binding_hash
        && acknowledgement.service_epoch == header.service_epoch
        && acknowledgement.revision == header.revision
        && acknowledgement.previous_event_root == header.previous_event_root
        && acknowledgement.event_root == header.event_root
        && acknowledgement.operation_id == header.operation_id
        && acknowledgement.transition_digest == header.transition_digest
        && acknowledgement.committed_at_unix_ms == header.committed_at_unix_ms
        && acknowledgement.acknowledgement_digest == header.acknowledgement_digest
        && acknowledgement.signature == header.signature
}

#[cfg(unix)]
fn validate_rotation_candidate(
    directory: &fs::File,
    name: &OsStr,
    store_identity: &DurableStoreIdentity,
    anchor: Option<&StateAnchorMetadata>,
    maximum_records: usize,
) -> Result<ParsedStateWitnessJournal, EngineError> {
    let file = openat_optional(
        directory.as_raw_fd(),
        name,
        libc::O_RDWR,
        "state witness rotation candidate",
    )?
    .ok_or_else(|| {
        EngineError::Internal("state witness rotation candidate disappeared".to_string())
    })?;
    validate_secure_regular_file(&file, "state witness rotation candidate")?;
    let parsed = read_state_witness_journal_streaming(
        &file,
        &store_identity.store_id,
        &store_identity.fingerprint,
        maximum_records,
        anchor,
    )?;
    if parsed.segment_header.is_none() || parsed.pending.is_some() {
        return Err(EngineError::Internal(
            "state witness rotation candidate is not a complete signed segment".to_string(),
        ));
    }
    Ok(parsed)
}

#[cfg(unix)]
fn open_or_create_state_witness(
    directory: &fs::File,
    name: &OsStr,
    store_identity: &DurableStoreIdentity,
    current_state_file: Option<&fs::File>,
    maximum_records: usize,
    anchor: Option<&StateAnchorMetadata>,
) -> Result<OpenedStateWitnessJournal, EngineError> {
    const LABEL: &str = "signer state witness journal";

    if let Some(file) = openat_optional(directory.as_raw_fd(), name, libc::O_RDWR, LABEL)? {
        validate_owned_unlinked_regular(&file, LABEL)?;
        set_owner_only_permissions(&file, LABEL)?;
        validate_secure_regular_file(&file, LABEL)?;

        // The journal is a fixed header followed by fixed-width records, each
        // appended and fsynced individually, and the genesis header+PREPARE+
        // COMMIT image is published by an atomic rename. A crash can therefore
        // leave at most one torn trailing record - bytes past the last complete
        // record boundary - and those bytes are the only thing that may be
        // discarded, after the truncation itself is made durable. Every record
        // that survives is then parsed and validated in full, so a COMPLETE
        // record with invalid content still fails closed instead of being
        // truncated away. Any PREPARE that survives the repair is returned to
        // the caller, which reconciles it against the held state image.
        let length = truncate_incomplete_witness_record(
            &file,
            &store_identity.store_id,
            &store_identity.fingerprint,
            anchor,
        )?;
        let parsed = read_state_witness_journal_streaming(
            &file,
            &store_identity.store_id,
            &store_identity.fingerprint,
            maximum_records,
            anchor,
        )?;
        validate_anchor_history(anchor, &parsed.history)?;
        debug_assert_eq!(length, parsed.length);
        let identity = descriptor_identity(&file, LABEL)?;
        return Ok(OpenedStateWitnessJournal {
            file,
            identity,
            parsed,
        });
    }

    if anchor.is_some() {
        return Err(EngineError::Internal(
            "signed state-anchor metadata exists but the state witness journal is missing; \
             refusing to re-genesis without an offline recovery certificate"
                .to_string(),
        ));
    }

    // Genesis is the one write that is not a single fixed-width record, so it
    // is the one write `truncate_incomplete_witness_record` cannot repair: a
    // short genesis is fatal at every length (0-47 fails the header, 48-152
    // has no complete record, 153-257 has no committed genesis). Publish it
    // atomically so the window does not exist.
    if maximum_records < 2 {
        return Err(EngineError::Validation(format!(
            "signer state witness record ceiling must reserve two genesis records; got [{}]",
            maximum_records
        )));
    }
    let digest = current_state_image_digest(current_state_file)?;
    let genesis_root = state_witness_genesis(&store_identity.fingerprint);
    let genesis = StateWitness {
        generation: 1,
        previous_commitment: genesis_root,
        commitment: state_commitment(&store_identity.fingerprint, 1, &genesis_root, &digest),
        state_image_digest: digest,
    };
    let mut bytes = Vec::with_capacity(
        TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH + 2 * TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH,
    );
    bytes.extend_from_slice(TBTC_SIGNER_STATE_WITNESS_MAGIC);
    bytes.extend_from_slice(&store_identity.store_id);
    bytes.extend_from_slice(&encode_state_witness_record(
        TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE,
        &genesis,
    ));
    bytes.extend_from_slice(&encode_state_witness_record(
        TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT,
        &genesis,
    ));
    let (file, identity) = create_entry_atomically(directory, name, &bytes, LABEL)?;
    Ok(OpenedStateWitnessJournal {
        file,
        identity,
        parsed: ParsedStateWitnessJournal {
            history: vec![genesis],
            pending: None,
            length: bytes.len(),
            header_length: TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH,
            header_bytes: bytes[..TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH].to_vec(),
            segment_header: None,
            tail_record: bytes[bytes.len() - TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH..]
                .try_into()
                .expect("genesis journal has one trailing fixed-width record"),
        },
    })
}

/// Removes an incomplete trailing journal record, and nothing else.
///
/// Only bytes after the last complete record boundary are dropped: they can
/// only be a torn append from a crash, because a record is written and fsynced
/// as one unit. A well-formed-length record is never removed here even when its
/// content is invalid - that case is a corruption signal and must fail closed
/// in `parse_state_witness_journal`. A journal whose header is short or does not
/// bind this exact store is likewise left untouched: nothing that may not be
/// this store's journal is ever rewritten, only rejected.
#[cfg(unix)]
fn truncate_incomplete_witness_record(
    file: &fs::File,
    expected_store_id: &[u8; 32],
    expected_store_fingerprint: &[u8; 32],
    anchor: Option<&StateAnchorMetadata>,
) -> Result<usize, EngineError> {
    const LABEL: &str = "signer state witness journal";
    let stat = descriptor_stat(file, LABEL)?;
    if stat.st_size < 0 {
        return Err(EngineError::Internal(
            "signer state witness journal has a negative length".to_string(),
        ));
    }
    let length = usize::try_from(stat.st_size).map_err(|_| {
        EngineError::Internal(
            "signer state witness journal length does not fit this platform".to_string(),
        )
    })?;
    if length < TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH {
        return Ok(length);
    }
    let prefix_length = length.min(TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH);
    let prefix = read_file_range_at(file, 0, prefix_length, LABEL)?;
    let header_length = if prefix.starts_with(TBTC_SIGNER_STATE_WITNESS_MAGIC)
        && prefix.len() >= TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH
        && &prefix[TBTC_SIGNER_STATE_WITNESS_MAGIC.len()..TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH]
            == expected_store_id
    {
        TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH
    } else if prefix.starts_with(TBTC_SIGNER_STATE_WITNESS_SEGMENT_MAGIC)
        && prefix.len() >= TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH
        && parse_state_witness_segment_header(
            &prefix[..TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH],
            expected_store_fingerprint,
            anchor,
        )
        .is_ok()
    {
        TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH
    } else {
        return Ok(length);
    };
    let incomplete_len = (length - header_length) % TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH;
    if incomplete_len == 0 {
        return Ok(length);
    }

    let complete_len = length - incomplete_len;
    file.set_len(complete_len as u64).map_err(|error| {
        EngineError::Internal(format!(
            "failed to discard the incomplete trailing signer state witness record: {error}"
        ))
    })?;
    file.sync_all().map_err(|error| {
        EngineError::Internal(format!(
            "failed to sync the repaired signer state witness journal: {error}"
        ))
    })?;
    Ok(complete_len)
}

#[cfg(unix)]
fn current_state_image_digest(state_file: Option<&fs::File>) -> Result<[u8; 32], EngineError> {
    match state_file {
        Some(file) => {
            let bytes = read_file_at(file, "signer state file")?;
            Ok(state_image_digest(Some(&bytes)))
        }
        None => Ok(state_image_digest(None)),
    }
}

fn encode_state_witness_record(record_type: u8, witness: &StateWitness) -> Vec<u8> {
    let mut record = Vec::with_capacity(TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH);
    record.push(record_type);
    record.extend_from_slice(&witness.generation.to_be_bytes());
    record.extend_from_slice(&witness.previous_commitment);
    record.extend_from_slice(&witness.state_image_digest);
    record.extend_from_slice(&witness.commitment);
    debug_assert_eq!(record.len(), TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH);
    record
}

/// Parses and validates the journal one fixed-width record at a time.
///
/// Startup must verify the entire anti-rollback chain, but it must not first
/// materialize an attacker-controlled file-sized `Vec`. The configured hard
/// record ceiling is checked from descriptor metadata before allocating the
/// bounded history, and stamps around the streaming read reject concurrent
/// modification.
#[cfg(unix)]
fn read_state_witness_journal_streaming(
    file: &fs::File,
    expected_store_id: &[u8; 32],
    store_fingerprint: &[u8; 32],
    maximum_records: usize,
    anchor: Option<&StateAnchorMetadata>,
) -> Result<ParsedStateWitnessJournal, EngineError> {
    const LABEL: &str = "signer state witness journal";
    let stat = descriptor_stat(file, LABEL)?;
    if stat.st_size < 0 {
        return Err(EngineError::Internal(
            "signer state witness journal has a negative length".to_string(),
        ));
    }
    let before = witness_change_stamp(file)?;
    let length = usize::try_from(stat.st_size).map_err(|_| {
        EngineError::Internal(
            "signer state witness journal length does not fit this platform".to_string(),
        )
    })?;
    if before.size != length as u64 {
        return Err(EngineError::Internal(
            "signer state witness journal changed before streaming verification".to_string(),
        ));
    }
    let prefix_length = length.min(TBTC_SIGNER_STATE_WITNESS_MAGIC.len());
    let prefix = read_file_range_at(file, 0, prefix_length, LABEL)?;
    #[cfg(test)]
    WITNESS_VERIFIED_BYTES_READ.fetch_add(prefix.len() as u64, std::sync::atomic::Ordering::SeqCst);
    if is_retired_v1_state_witness_journal(&prefix) {
        return Err(retired_v1_state_witness_journal_error());
    }
    if length < TBTC_SIGNER_STATE_WITNESS_MAGIC.len() {
        return Err(truncated_state_witness_journal_error(format!(
            "signer state witness journal is [{length}] bytes, shorter than its magic"
        )));
    }
    let header_length = if prefix.as_slice() == TBTC_SIGNER_STATE_WITNESS_MAGIC {
        TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH
    } else if prefix.as_slice() == TBTC_SIGNER_STATE_WITNESS_SEGMENT_MAGIC {
        TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH
    } else {
        return Err(EngineError::Internal(
            "signer state witness journal magic is invalid".to_string(),
        ));
    };
    if length < header_length {
        return Err(truncated_state_witness_journal_error(format!(
            "signer state witness journal is [{length}] bytes, shorter than its \
             {header_length}-byte header"
        )));
    }
    let mut header_bytes = prefix;
    let header_tail = read_file_range_at(
        file,
        TBTC_SIGNER_STATE_WITNESS_MAGIC.len(),
        header_length - TBTC_SIGNER_STATE_WITNESS_MAGIC.len(),
        LABEL,
    )?;
    #[cfg(test)]
    WITNESS_VERIFIED_BYTES_READ.fetch_add(
        header_tail.len() as u64,
        std::sync::atomic::Ordering::SeqCst,
    );
    header_bytes.extend_from_slice(&header_tail);

    let (segment_header, mut history, requires_record) = if header_length
        == TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH
    {
        if &header_bytes[TBTC_SIGNER_STATE_WITNESS_MAGIC.len()..] != expected_store_id {
            return Err(EngineError::Internal(
                "signer state witness journal store ID is invalid".to_string(),
            ));
        }
        (None, Vec::new(), true)
    } else {
        let header = parse_state_witness_segment_header(&header_bytes, store_fingerprint, anchor)?;
        let base = header.base.clone();
        (Some(header), vec![base], false)
    };

    let record_bytes = length - header_length;
    if (requires_record && record_bytes == 0)
        || !record_bytes.is_multiple_of(TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH)
    {
        return Err(truncated_state_witness_journal_error(
            "signer state witness journal contains a missing or partial record".to_string(),
        ));
    }
    let record_count = record_bytes / TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH;
    if record_count > maximum_records {
        return Err(EngineError::Internal(format!(
            "signer state witness journal contains [{record_count}] records, exceeding the \
             configured fail-closed ceiling [{maximum_records}]"
        )));
    }

    history.reserve(record_count.div_ceil(2));
    let mut pending = None::<StateWitness>;
    let mut tail = [0u8; TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH];
    for index in 0..record_count {
        let offset = header_length + index * TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH;
        let record =
            read_file_range_at(file, offset, TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH, LABEL)?;
        #[cfg(test)]
        WITNESS_VERIFIED_BYTES_READ
            .fetch_add(record.len() as u64, std::sync::atomic::Ordering::SeqCst);
        apply_state_witness_record(&record, store_fingerprint, &mut history, &mut pending)?;
        if index + 1 == record_count {
            tail.copy_from_slice(&record);
        }
    }
    if history.is_empty() {
        return Err(truncated_state_witness_journal_error(
            "signer state witness journal has no committed genesis record".to_string(),
        ));
    }
    let after = witness_change_stamp(file)?;
    if before != after {
        return Err(EngineError::Internal(
            "signer state witness journal changed while it was being verified".to_string(),
        ));
    }
    Ok(ParsedStateWitnessJournal {
        history,
        pending,
        length,
        header_length,
        header_bytes,
        segment_header,
        tail_record: tail,
    })
}

#[cfg(test)]
fn parse_state_witness_journal(
    bytes: &[u8],
    expected_store_id: &[u8; 32],
    store_fingerprint: &[u8; 32],
) -> Result<(Vec<StateWitness>, Option<StateWitness>), EngineError> {
    if is_retired_v1_state_witness_journal(bytes) {
        return Err(retired_v1_state_witness_journal_error());
    }
    if bytes.len() < TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH {
        return Err(truncated_state_witness_journal_error(format!(
            "signer state witness journal is [{}] bytes, shorter than its \
             {TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH}-byte header",
            bytes.len()
        )));
    }
    if &bytes[..TBTC_SIGNER_STATE_WITNESS_MAGIC.len()] != TBTC_SIGNER_STATE_WITNESS_MAGIC
        || &bytes[TBTC_SIGNER_STATE_WITNESS_MAGIC.len()..TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH]
            != expected_store_id
    {
        return Err(EngineError::Internal(
            "signer state witness journal header or store ID is invalid".to_string(),
        ));
    }
    let records = &bytes[TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH..];
    let complete_records = records.chunks_exact(TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH);
    if records.is_empty() || !complete_records.remainder().is_empty() {
        return Err(truncated_state_witness_journal_error(
            "signer state witness journal contains a missing or partial record".to_string(),
        ));
    }

    let mut history = Vec::<StateWitness>::new();
    let mut pending = None::<StateWitness>;
    for record in complete_records {
        apply_state_witness_record(record, store_fingerprint, &mut history, &mut pending)?;
    }
    if history.is_empty() {
        return Err(truncated_state_witness_journal_error(
            "signer state witness journal has no committed genesis record".to_string(),
        ));
    }
    Ok((history, pending))
}

fn apply_state_witness_record(
    record: &[u8],
    store_fingerprint: &[u8; 32],
    history: &mut Vec<StateWitness>,
    pending: &mut Option<StateWitness>,
) -> Result<(), EngineError> {
    if record.len() != TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH {
        return Err(truncated_state_witness_journal_error(
            "signer state witness journal contains a missing or partial record".to_string(),
        ));
    }
    let record_type = record[0];
    let generation = u64::from_be_bytes(
        record[1..9]
            .try_into()
            .expect("fixed state witness generation slice"),
    );
    let mut previous_commitment = [0u8; 32];
    previous_commitment.copy_from_slice(&record[9..41]);
    let mut state_image_digest = [0u8; 32];
    state_image_digest.copy_from_slice(&record[41..73]);
    let mut commitment = [0u8; 32];
    commitment.copy_from_slice(&record[73..105]);
    let witness = StateWitness {
        generation,
        previous_commitment,
        commitment,
        state_image_digest,
    };
    if generation == 0
        || state_image_digest == [0u8; 32]
        || commitment == [0u8; 32]
        || commitment
            != state_commitment(
                store_fingerprint,
                generation,
                &previous_commitment,
                &state_image_digest,
            )
    {
        return Err(EngineError::Internal(
            "signer state witness journal contains an invalid commitment".to_string(),
        ));
    }

    match record_type {
        TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE => {
            if pending.is_some() {
                return Err(EngineError::Internal(
                    "signer state witness journal contains nested PREPARE records".to_string(),
                ));
            }
            let (expected_generation, expected_previous) = match history.last() {
                Some(tip) => (
                    tip.generation.checked_add(1).ok_or_else(|| {
                        EngineError::Internal(
                            "signer state witness generation exhausted u64".to_string(),
                        )
                    })?,
                    tip.commitment,
                ),
                None => (1, state_witness_genesis(store_fingerprint)),
            };
            if witness.generation != expected_generation
                || witness.previous_commitment != expected_previous
            {
                return Err(EngineError::Internal(
                    "signer state witness PREPARE does not extend the committed tip".to_string(),
                ));
            }
            *pending = Some(witness);
        }
        TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT => {
            if pending.as_ref() != Some(&witness) {
                return Err(EngineError::Internal(
                    "signer state witness COMMIT does not match its PREPARE".to_string(),
                ));
            }
            history.push(witness);
            *pending = None;
        }
        TBTC_SIGNER_STATE_WITNESS_RECORD_ABORT => {
            if pending.as_ref() != Some(&witness) {
                return Err(EngineError::Internal(
                    "signer state witness ABORT does not match its PREPARE".to_string(),
                ));
            }
            *pending = None;
        }
        _ => {
            return Err(EngineError::Internal(format!(
                "signer state witness journal contains unknown record type [{record_type}]"
            )))
        }
    }
    Ok(())
}

/// True when the journal carries the retired v1 magic. The store ID is not
/// consulted: a v1 journal must be recognized even when the caller cannot
/// recompute the v1 fingerprint any more, which is precisely the situation the
/// v2 transcript exists to fix.
fn is_retired_v1_state_witness_journal(bytes: &[u8]) -> bool {
    bytes.len() >= TBTC_SIGNER_STATE_WITNESS_MAGIC_V1.len()
        && &bytes[..TBTC_SIGNER_STATE_WITNESS_MAGIC_V1.len()] == TBTC_SIGNER_STATE_WITNESS_MAGIC_V1
}

fn retired_v1_state_witness_journal_error() -> EngineError {
    EngineError::Internal(format!(
        "signer state witness journal uses the retired v1 state-commitment transcript \
         (magic [{}]); this build commits under v2, whose store fingerprint binds only the \
         stable {TBTC_SIGNER_DURABLE_STORE_ID_SUFFIX} bytes. The journal was left byte-for-byte \
         intact. Run the documented v1->v2 witness re-anchor before starting this build; do NOT \
         delete the journal, which would silently re-genesis the anti-rollback chain at \
         generation 1",
        String::from_utf8_lossy(TBTC_SIGNER_STATE_WITNESS_MAGIC_V1).trim_end_matches('\0')
    ))
}

/// A short journal is never a torn create: the header, PREPARE, and COMMIT of a
/// genesis journal are written to a temp file, fsynced, and renamed into place
/// as one unit, and every later record is appended and fsynced as one
/// fixed-width unit whose torn remainder is repaired in place. Fail closed and
/// say what an operator can actually do.
fn truncated_state_witness_journal_error(detail: String) -> EngineError {
    EngineError::Internal(format!(
        "{detail}. The journal is created atomically and appended one fsynced fixed-width \
         record at a time, so this is damage from outside the signer, not a torn write. \
         Restore {TBTC_SIGNER_STATE_WITNESS_SUFFIX} together with the state image it commits \
         to, or run the documented re-anchor procedure; deleting the journal would silently \
         re-genesis the anti-rollback chain at generation 1"
    ))
}

#[cfg(unix)]
fn read_store_id(file: &fs::File) -> Result<[u8; 32], EngineError> {
    let bytes = read_file_at(file, "signer durable store ID file")?;
    if bytes.len() != 32 {
        return Err(EngineError::Internal(format!(
            "signer durable store ID file has invalid length [{}], expected 32. The store ID is \
             written to a temp entry, fsynced, and renamed into place, so a short file is not a \
             torn create: restore {TBTC_SIGNER_DURABLE_STORE_ID_SUFFIX} from the store's backup. \
             Replacing it with fresh bytes would orphan the state-witness journal, which binds \
             this exact store ID",
            bytes.len()
        )));
    }
    let mut id = [0u8; 32];
    id.copy_from_slice(&bytes);
    if id == [0u8; 32] {
        return Err(EngineError::Internal(
            "signer durable store ID must not be zero".to_string(),
        ));
    }
    Ok(id)
}

#[cfg(unix)]
fn open_optional_state(
    directory: &fs::File,
    state_name: &OsStr,
) -> Result<(Option<fs::File>, Option<OpenedObjectIdentity>), EngineError> {
    let Some(file) = openat_optional(
        directory.as_raw_fd(),
        state_name,
        libc::O_RDONLY,
        "signer state file",
    )?
    else {
        return Ok((None, None));
    };
    validate_secure_regular_file(&file, "signer state file")?;
    let identity = descriptor_identity(&file, "signer state file")?;
    Ok((Some(file), Some(identity)))
}

#[cfg(unix)]
fn openat_optional(
    directory_fd: RawFd,
    name: &OsStr,
    flags: i32,
    label: &str,
) -> Result<Option<fs::File>, EngineError> {
    let name_c = os_str_cstring(name, label)?;
    let fd = unsafe {
        libc::openat(
            directory_fd,
            name_c.as_ptr(),
            flags | libc::O_CLOEXEC | libc::O_NOFOLLOW | libc::O_NONBLOCK,
        )
    };
    if fd >= 0 {
        return Ok(Some(unsafe { fs::File::from_raw_fd(fd) }));
    }
    let error = std::io::Error::last_os_error();
    if error.raw_os_error() == Some(libc::ENOENT) {
        return Ok(None);
    }
    Err(EngineError::Internal(format!(
        "failed to open {label} without following symlinks: {error}"
    )))
}

#[cfg(unix)]
fn acquire_exclusive_lock(file: &fs::File, lock_path: &Path) -> Result<(), EngineError> {
    let result = unsafe { libc::flock(file.as_raw_fd(), libc::LOCK_EX | libc::LOCK_NB) };
    if result == 0 {
        return Ok(());
    }
    let error = std::io::Error::last_os_error();
    if error.raw_os_error().is_some_and(is_lock_contention_errno) {
        return Err(EngineError::Internal(format!(
            "signer state lock already held by another process [{}]",
            lock_path.display()
        )));
    }
    Err(EngineError::Internal(format!(
        "failed to lock signer state file [{}]: {error}",
        lock_path.display()
    )))
}

#[cfg(unix)]
fn is_lock_contention_errno(errno: i32) -> bool {
    errno == libc::EAGAIN || errno == libc::EWOULDBLOCK
}

#[cfg(unix)]
fn descriptor_stat(file: &fs::File, label: &str) -> Result<libc::stat, EngineError> {
    let mut stat = unsafe { std::mem::zeroed::<libc::stat>() };
    if unsafe { libc::fstat(file.as_raw_fd(), &mut stat) } != 0 {
        return Err(EngineError::Internal(format!(
            "failed to inspect opened {label}: {}",
            std::io::Error::last_os_error()
        )));
    }
    Ok(stat)
}

#[cfg(unix)]
fn descriptor_identity(file: &fs::File, label: &str) -> Result<OpenedObjectIdentity, EngineError> {
    let stat = descriptor_stat(file, label)?;
    Ok(stat_identity(stat))
}

/// `st_dev`/`st_ino` widths and signedness are platform-dependent, so the
/// identity is built once here - through one widening conversion that keeps the
/// frozen fingerprint bytes identical on every target - and then compared as a
/// whole value rather than field by field.
#[cfg(unix)]
fn stat_identity(stat: libc::stat) -> OpenedObjectIdentity {
    OpenedObjectIdentity {
        device: widen_stat_field(stat.st_dev),
        inode: widen_stat_field(stat.st_ino),
    }
}

#[cfg(unix)]
fn widen_stat_field<T: Into<i128>>(value: T) -> u64 {
    value.into() as u64
}

#[cfg(unix)]
fn validate_secure_regular_file(file: &fs::File, label: &str) -> Result<(), EngineError> {
    validate_owned_unlinked_regular(file, label)?;
    let stat = descriptor_stat(file, label)?;
    if stat.st_mode & 0o077 != 0 {
        return Err(EngineError::Internal(format!(
            "opened {label} is accessible by group or other users"
        )));
    }
    Ok(())
}

#[cfg(unix)]
fn validate_owned_unlinked_regular(file: &fs::File, label: &str) -> Result<(), EngineError> {
    let stat = descriptor_stat(file, label)?;
    if stat.st_mode & libc::S_IFMT != libc::S_IFREG {
        return Err(EngineError::Internal(format!(
            "opened {label} is not a regular file"
        )));
    }
    if stat.st_nlink != 1 {
        return Err(EngineError::Internal(format!(
            "opened {label} has [{}] hard links; exactly one is required",
            stat.st_nlink
        )));
    }
    if stat.st_uid != unsafe { libc::geteuid() } {
        return Err(EngineError::Internal(format!(
            "opened {label} is not owned by the signer user"
        )));
    }
    Ok(())
}

#[cfg(unix)]
fn set_owner_only_permissions(file: &fs::File, label: &str) -> Result<(), EngineError> {
    if unsafe { libc::fchmod(file.as_raw_fd(), 0o600) } != 0 {
        return Err(EngineError::Internal(format!(
            "failed to set owner-only permissions on {label}: {}",
            std::io::Error::last_os_error()
        )));
    }
    Ok(())
}

#[cfg(unix)]
fn live_entry_stat(
    directory_fd: RawFd,
    name: &OsStr,
    label: &str,
) -> Result<Option<libc::stat>, EngineError> {
    let name_c = os_str_cstring(name, label)?;
    let mut stat = unsafe { std::mem::zeroed::<libc::stat>() };
    let result = unsafe {
        libc::fstatat(
            directory_fd,
            name_c.as_ptr(),
            &mut stat,
            libc::AT_SYMLINK_NOFOLLOW,
        )
    };
    if result == 0 {
        return Ok(Some(stat));
    }
    let error = std::io::Error::last_os_error();
    if error.raw_os_error() == Some(libc::ENOENT) {
        return Ok(None);
    }
    Err(EngineError::Internal(format!(
        "failed to inspect live {label}: {error}"
    )))
}

#[cfg(unix)]
fn validate_live_entry(
    directory: &fs::File,
    name: &OsStr,
    expected: OpenedObjectIdentity,
    label: &str,
) -> Result<(), EngineError> {
    let Some(stat) = live_entry_stat(directory.as_raw_fd(), name, label)? else {
        return Err(replacement_error(label));
    };
    if stat.st_mode & libc::S_IFMT != libc::S_IFREG
        || stat.st_nlink != 1
        || stat_identity(stat) != expected
    {
        return Err(replacement_error(label));
    }
    Ok(())
}

#[cfg(unix)]
fn ensure_entry_absent(directory_fd: RawFd, name: &OsStr, label: &str) -> Result<(), EngineError> {
    if live_entry_stat(directory_fd, name, label)?.is_some() {
        return Err(EngineError::Internal(format!(
            "unexpected {label} appeared after the signer store was opened"
        )));
    }
    Ok(())
}

fn replacement_error(label: &str) -> EngineError {
    EngineError::Internal(format!(
        "{label} was removed, replaced, linked, or redirected after the signer store was opened"
    ))
}

#[cfg(unix)]
fn read_file_at(file: &fs::File, label: &str) -> Result<Vec<u8>, EngineError> {
    use std::os::unix::fs::FileExt;

    let stat = descriptor_stat(file, label)?;
    if stat.st_size < 0 {
        return Err(EngineError::Internal(format!(
            "opened {label} has a negative size"
        )));
    }
    let length = usize::try_from(stat.st_size)
        .map_err(|_| EngineError::Internal(format!("opened {label} is too large to read")))?;
    let mut bytes = vec![0u8; length];
    let mut offset = 0usize;
    while offset < length {
        let read = file
            .read_at(&mut bytes[offset..], offset as u64)
            .map_err(|error| {
                EngineError::Internal(format!("failed to read opened {label}: {error}"))
            })?;
        if read == 0 {
            return Err(EngineError::Internal(format!(
                "opened {label} was truncated while being read"
            )));
        }
        offset += read;
    }
    Ok(bytes)
}

/// Reads an exact byte range. Unlike `read_file_at` this never depends on - and
/// never pays for - the total file length, which is what makes verifying only
/// the newly appended journal bytes cheap.
#[cfg(unix)]
fn read_file_range_at(
    file: &fs::File,
    offset: usize,
    length: usize,
    label: &str,
) -> Result<Vec<u8>, EngineError> {
    use std::os::unix::fs::FileExt;

    let mut bytes = vec![0u8; length];
    let mut read_total = 0usize;
    while read_total < length {
        let position = offset
            .checked_add(read_total)
            .ok_or_else(|| EngineError::Internal(format!("opened {label} read offset overflow")))?;
        let read = file
            .read_at(&mut bytes[read_total..], position as u64)
            .map_err(|error| {
                EngineError::Internal(format!("failed to read opened {label}: {error}"))
            })?;
        if read == 0 {
            return Err(EngineError::Internal(format!(
                "opened {label} was truncated while being read"
            )));
        }
        read_total += read;
    }
    Ok(bytes)
}

#[cfg(unix)]
fn witness_change_stamp(file: &fs::File) -> Result<FileChangeStamp, EngineError> {
    file_change_stamp(file, "signer state witness journal")
}

#[cfg(unix)]
fn file_change_stamp(file: &fs::File, label: &str) -> Result<FileChangeStamp, EngineError> {
    let stat = descriptor_stat(file, label)?;
    Ok(FileChangeStamp {
        size: widen_stat_field(stat.st_size),
        modified_seconds: widen_stat_field(stat.st_mtime),
        modified_nanoseconds: widen_stat_field(stat.st_mtime_nsec),
        changed_seconds: widen_stat_field(stat.st_ctime),
        changed_nanoseconds: widen_stat_field(stat.st_ctime_nsec),
    })
}

#[cfg(unix)]
fn write_file_at(file: &fs::File, bytes: &[u8], label: &str) -> Result<(), EngineError> {
    use std::os::unix::fs::FileExt;

    file.set_len(0).map_err(|error| {
        EngineError::Internal(format!("failed to truncate opened {label}: {error}"))
    })?;
    let mut offset = 0usize;
    while offset < bytes.len() {
        let written = file
            .write_at(&bytes[offset..], offset as u64)
            .map_err(|error| {
                EngineError::Internal(format!("failed to write opened {label}: {error}"))
            })?;
        if written == 0 {
            return Err(EngineError::Internal(format!(
                "short write while writing opened {label}"
            )));
        }
        offset += written;
    }
    Ok(())
}

#[cfg(unix)]
fn append_file_at(
    file: &fs::File,
    initial_offset: usize,
    bytes: &[u8],
    label: &str,
) -> Result<(), EngineError> {
    use std::os::unix::fs::FileExt;

    let mut written_total = 0usize;
    while written_total < bytes.len() {
        let offset = initial_offset.checked_add(written_total).ok_or_else(|| {
            EngineError::Internal(format!("opened {label} append offset overflow"))
        })?;
        let written = file
            .write_at(&bytes[written_total..], offset as u64)
            .map_err(|error| {
                EngineError::Internal(format!("failed to append opened {label}: {error}"))
            })?;
        if written == 0 {
            return Err(EngineError::Internal(format!(
                "short write while appending opened {label}"
            )));
        }
        written_total += written;
    }
    Ok(())
}

#[cfg(unix)]
fn unique_temp_name(state_name: &OsStr) -> Result<OsString, EngineError> {
    let mut random = [0u8; 16];
    OsRng.fill_bytes(&mut random);
    let mut name = state_name.to_os_string();
    name.push(format!(
        ".tmp-{}-{}",
        std::process::id(),
        hex::encode(random)
    ));
    validate_entry_name(&name, "state temp")?;
    Ok(name)
}

#[cfg(unix)]
fn renameat_same_directory(
    directory_fd: RawFd,
    source: &OsStr,
    destination: &OsStr,
    label: &str,
) -> Result<(), EngineError> {
    let source = os_str_cstring(source, label)?;
    let destination = os_str_cstring(destination, label)?;
    if unsafe {
        libc::renameat(
            directory_fd,
            source.as_ptr(),
            directory_fd,
            destination.as_ptr(),
        )
    } != 0
    {
        return Err(EngineError::Internal(format!(
            "failed to {label}: {}",
            std::io::Error::last_os_error()
        )));
    }
    Ok(())
}

#[cfg(unix)]
fn unlinkat_entry(directory_fd: RawFd, name: &OsStr) -> Result<(), EngineError> {
    let name = os_str_cstring(name, "state temp")?;
    if unsafe { libc::unlinkat(directory_fd, name.as_ptr(), 0) } != 0 {
        let error = std::io::Error::last_os_error();
        if error.raw_os_error() != Some(libc::ENOENT) {
            return Err(EngineError::Internal(format!(
                "failed to remove signer state temp file: {error}"
            )));
        }
    }
    Ok(())
}

#[cfg(test)]
mod witness_transcript_tests {
    use super::*;
    use ed25519_dalek::{Signer, SigningKey};

    /// Entry name, inode, mode, length, mtime, mtime nanos, ctime, ctime
    /// nanos, and contents.
    #[cfg(unix)]
    type DirectorySnapshotEntry = (OsString, u64, u32, u64, i64, i64, i64, i64, Vec<u8>);

    #[cfg(unix)]
    #[derive(Debug, Eq, PartialEq)]
    struct DirectorySnapshot {
        directory: (u64, u32, u64, i64, i64, i64, i64),
        entries: Vec<DirectorySnapshotEntry>,
    }

    #[cfg(unix)]
    fn snapshot_directory_without_atime(path: &Path) -> DirectorySnapshot {
        use std::os::unix::fs::MetadataExt;

        let metadata_tuple = |metadata: &fs::Metadata| {
            (
                metadata.ino(),
                metadata.mode(),
                metadata.len(),
                metadata.mtime(),
                metadata.mtime_nsec(),
                metadata.ctime(),
                metadata.ctime_nsec(),
            )
        };
        let directory_metadata = fs::symlink_metadata(path).expect("snapshot directory metadata");
        let mut entries = fs::read_dir(path)
            .expect("snapshot directory entries")
            .map(|entry| {
                let entry = entry.expect("snapshot directory entry");
                let metadata = fs::symlink_metadata(entry.path()).expect("snapshot entry metadata");
                let (inode, mode, length, mtime, mtime_nsec, ctime, ctime_nsec) =
                    metadata_tuple(&metadata);
                let bytes = if metadata.file_type().is_file() {
                    fs::read(entry.path()).expect("snapshot regular entry bytes")
                } else {
                    Vec::new()
                };
                (
                    entry.file_name(),
                    inode,
                    mode,
                    length,
                    mtime,
                    mtime_nsec,
                    ctime,
                    ctime_nsec,
                    bytes,
                )
            })
            .collect::<Vec<_>>();
        entries.sort_by(|left, right| left.0.cmp(&right.0));
        DirectorySnapshot {
            directory: metadata_tuple(&directory_metadata),
            entries,
        }
    }

    /// The live cross-language transcript. The Go bridge must reproduce these
    /// bytes exactly; `store_fingerprint` here is a `durable_store_fingerprint`
    /// output, which under v2 is a function of the `.store-id` bytes alone.
    #[test]
    fn state_witness_transcripts_match_frozen_go_v2_vectors() {
        let store_fingerprint = [0x11; 32];
        assert_eq!(
            hex::encode(state_witness_genesis(&store_fingerprint)),
            "44085b42d29bf25f06207142f9e2db58eaf86f88d92b6e18104161ce59e98a89"
        );
        assert_eq!(
            hex::encode(state_commitment(
                &store_fingerprint,
                42,
                &[0x22; 32],
                &[0x33; 32],
            )),
            "ea5eb04a4776357e59875f683390a2ff4b7dd511ad394e588dfab147f94fa867"
        );
    }

    /// End-to-end v2 chain vector: the `.store-id` bytes derive the store
    /// fingerprint, the fingerprint derives the genesis root, and the genesis
    /// record commits over it. The Go bridge must reproduce all three.
    #[test]
    fn state_witness_chain_matches_frozen_go_v2_vector() {
        let fingerprint = durable_store_fingerprint(&[0x11; 32]);
        assert_eq!(
            hex::encode(fingerprint),
            "8bb8d21c69e78916e8f165b0c861c0d84c5d7af5393f75b0321fe048f772abba"
        );
        let genesis_root = state_witness_genesis(&fingerprint);
        assert_eq!(
            hex::encode(genesis_root),
            "3179b8bc6614b0951b703f9c418b17cf7cd8b7f1bef1f86587385d4c150efab2"
        );
        assert_eq!(
            hex::encode(state_commitment(
                &fingerprint,
                1,
                &genesis_root,
                &[0x33; 32]
            )),
            "5387626d5314b17b324f9a7df1ab16fcbf10917a137527bf33c71847e1b77da0"
        );
    }

    /// Regression guard for the rejection path: the retired v1 transcript must
    /// keep producing its frozen bytes so v1 journal fixtures stay realistic,
    /// and it must be distinct from v2 in both the genesis root and the
    /// commitment.
    #[test]
    fn retired_v1_transcript_vectors_stay_frozen_and_distinct() {
        let store_fingerprint = [0x11; 32];
        assert_eq!(
            hex::encode(state_witness_genesis_v1(&store_fingerprint)),
            "639ab6bce7b111044aa40cbe05d2a79a789c47d83e0dbf5ac83af3e2c8717775"
        );
        assert_eq!(
            hex::encode(state_commitment_v1(
                &store_fingerprint,
                42,
                &[0x22; 32],
                &[0x33; 32],
            )),
            "903d154bca4b0e46f2cadda81db9559bdf2d719956065266f55bd845e64b7ced"
        );
        assert_ne!(
            state_witness_genesis(&store_fingerprint),
            state_witness_genesis_v1(&store_fingerprint)
        );
        assert_ne!(
            state_commitment(&store_fingerprint, 42, &[0x22; 32], &[0x33; 32]),
            state_commitment_v1(&store_fingerprint, 42, &[0x22; 32], &[0x33; 32])
        );
    }

    #[test]
    fn retired_v1_journals_are_recognized_by_magic_alone() {
        let journal =
            encode_v1_state_witness_genesis_journal(&[0x24; 32], &[0x11; 32], &[0x33; 32]);
        assert!(is_retired_v1_state_witness_journal(&journal));
        assert!(!is_retired_v1_state_witness_journal(
            TBTC_SIGNER_STATE_WITNESS_MAGIC
        ));

        let error = parse_state_witness_journal(&journal, &[0x24; 32], &[0x11; 32])
            .expect_err("a v1 journal must fail closed");
        let EngineError::Internal(message) = error else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains("retired v1 state-commitment transcript"),
            "unexpected v1 rejection message: {message}"
        );
        assert!(
            message.contains("re-anchor"),
            "the v1 rejection must be actionable: {message}"
        );
    }

    fn fixture_acknowledgement() -> StateAnchorAcknowledgement {
        let store_fingerprint = [0x11; 32];
        let previous_commitment = [0x22; 32];
        let state_image_digest = [0x33; 32];
        StateAnchorAcknowledgement {
            binding_hash: [0x44; 32],
            request_digest: [0x45; 32],
            nonce: [0x46; 32],
            status: 1,
            service_epoch: 7,
            revision: 1,
            previous_event_root: [0u8; 32],
            event_root: [0x55; 32],
            checkpoint_store_fingerprint: store_fingerprint,
            checkpoint_generation: 42,
            checkpoint_previous_commitment: previous_commitment,
            checkpoint_state_image_digest: state_image_digest,
            checkpoint_state_commitment: state_commitment(
                &store_fingerprint,
                42,
                &previous_commitment,
                &state_image_digest,
            ),
            operation_id: [0x66; 32],
            transition_digest: [0x77; 32],
            committed_at_unix_ms: 123_456_789,
            expires_at_unix_ms: 123_456_790,
            signing_digest: [0x88; 32],
            signature: [0x99; 64],
            configured_spki_hash: [0xaa; 32],
            acknowledgement_digest: [0xbb; 32],
        }
    }

    #[test]
    #[cfg(unix)]
    fn restored_expired_intent_never_authorizes_local_recovery() {
        let _guard = lock_test_state();
        let (state_path, fresh, tip) = bootstrap_trust_store_fixture("expired-intent-rollback");
        let now = u64::try_from(
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("clock after epoch")
                .as_millis(),
        )
        .expect("clock fits u64");
        let nested = &fresh.certificates[0].target_acknowledgement;
        let expired = bootstrap_state_anchor_trust_transition_for_tests(
            nested.checkpoint_store_fingerprint,
            &tip,
            nested.committed_at_unix_ms,
            nested.expires_at_unix_ms,
            now - 60_000,
            now - 30_000,
            false,
        )
        .expect("intrinsically valid expired transition");
        assert_eq!(
            expired.request.certificate_chain,
            fresh.request.certificate_chain
        );

        let store = StateFileLock::acquire_for_trust_transition(&state_path, &fresh)
            .expect("open pristine transition store");
        let intent_bytes = encode_state_anchor_trust_transition_intent(
            &store.identity.fingerprint,
            &expired.request,
        )
        .expect("encode restored expired intent");
        let _ = create_entry_atomically(
            &store.directory,
            &store.trust_intent_name,
            &intent_bytes,
            "restored expired trust intent",
        )
        .expect("restore old intent snapshot");
        let witness_before =
            fs::read(state_witness_file_path(&state_path)).expect("read pristine witness");
        drop(store);

        let recovery_error = match StateFileLock::acquire_for_trust_head_inspection(&state_path) {
            Ok(_) => panic!("expired local intent must not auto-recover"),
            Err(error) => error,
        };
        let EngineError::StateAnchorTrustRecoveryRequired { context } = recovery_error else {
            panic!("unexpected restored-intent preflight error: {recovery_error}");
        };
        assert_eq!(
            context.certificate_digests,
            vec![fresh.certificates[0].certificate_digest]
        );
        assert_eq!(
            fs::read(state_anchor_trust_intent_file_path(&state_path))
                .expect("intent remains after preflight"),
            intent_bytes
        );
        assert_eq!(
            fs::read(state_witness_file_path(&state_path))
                .expect("witness remains after preflight"),
            witness_before
        );
        assert!(!state_anchor_trust_file_path(&state_path).exists());
        assert!(!state_anchor_file_path(&state_path).exists());

        let expired_error = match StateFileLock::acquire_for_trust_transition(&state_path, &expired)
        {
            Ok(_) => panic!("expired wrapper must not resume restored intent"),
            Err(error) => error,
        };
        assert!(
            expired_error.to_string().contains("expired"),
            "unexpected expired recovery error: {expired_error}"
        );
        assert!(state_anchor_trust_intent_file_path(&state_path).exists());

        let mut resumed = StateFileLock::acquire_for_trust_transition(&state_path, &fresh)
            .expect("fresh wrapper resumes exact restored intent");
        let outcome = resumed
            .transition_state_witness_anchor(&fresh)
            .expect("post-recovery exact replay");
        assert!(outcome.idempotent);
        drop(resumed);
        assert!(!state_anchor_trust_intent_file_path(&state_path).exists());
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    #[cfg(unix)]
    fn recovery_required_preflight_is_byte_and_metadata_read_only() {
        let _guard = lock_test_state();

        let leave_intent = |state_path: &Path| {
            let (transition, _) = bootstrap_trust_store_fixture_at(state_path);
            let mut store = StateFileLock::acquire_for_trust_transition(state_path, &transition)
                .expect("open transition store");
            set_state_anchor_trust_transition_fault_for_tests(
                StateAnchorTrustTransitionFaultInjectionPoint::AfterIntentPublication,
            );
            store
                .transition_state_witness_anchor(&transition)
                .expect_err("leave intent before first transition mutation");
            clear_state_anchor_trust_transition_fault_for_tests();
            drop(store);
            transition
        };
        let unique_directory = |label: &str| {
            let mut random = [0u8; 12];
            OsRng.fill_bytes(&mut random);
            std::env::temp_dir().join(format!(
                "tbtc-signer-zero-write-{label}-{}-{}",
                std::process::id(),
                hex::encode(random)
            ))
        };

        let directory = unique_directory("intact");
        fs::create_dir(&directory).expect("create isolated signer directory");
        let state_path = directory.join("signer-state.json");
        let transition = leave_intent(&state_path);
        let before = snapshot_directory_without_atime(&directory);
        let error = match StateFileLock::acquire_for_trust_head_inspection(&state_path) {
            Ok(_) => panic!("intent inspection must require externally fresh recovery"),
            Err(error) => error,
        };
        let EngineError::StateAnchorTrustRecoveryRequired { context } = error else {
            panic!("unexpected intact recovery preflight error: {error}");
        };
        assert_eq!(
            context.certificate_digests,
            vec![transition.certificates[0].certificate_digest]
        );
        assert_eq!(
            snapshot_directory_without_atime(&directory),
            before,
            "recovery-required inspection changed directory entries, bytes, inode, mode, \
             mtime, or ctime"
        );
        cleanup_anchor_store_fixture(&state_path);
        fs::remove_dir(&directory).expect("remove isolated signer directory");

        let outer_directory = unique_directory("missing-parent");
        fs::create_dir(&outer_directory).expect("create missing-parent snapshot root");
        let missing_parent = outer_directory.join("absent-store");
        let missing_state_path = missing_parent.join("signer-state.json");
        let before = snapshot_directory_without_atime(&outer_directory);
        let error = match StateFileLock::acquire_for_trust_head_inspection(&missing_state_path) {
            Ok(_) => panic!("missing trust store cannot have a committed head"),
            Err(error) => error,
        };
        assert!(matches!(error, EngineError::StateAnchorTrustHeadAbsent));
        assert_eq!(
            snapshot_directory_without_atime(&outer_directory),
            before,
            "trust-head inspection created or modified a missing parent"
        );
        assert!(!missing_parent.exists());
        fs::remove_dir(&outer_directory).expect("remove missing-parent snapshot root");

        for missing in ["lock", "store-id"] {
            establish_clean_signer_test_env();
            let directory = unique_directory(missing);
            fs::create_dir(&directory).expect("create isolated missing-prerequisite directory");
            let state_path = directory.join("signer-state.json");
            let _transition = leave_intent(&state_path);
            let prerequisite = if missing == "lock" {
                state_lock_file_path(&state_path)
            } else {
                durable_store_id_file_path(&state_path)
            };
            fs::remove_file(&prerequisite).expect("remove recovery prerequisite");
            let before = snapshot_directory_without_atime(&directory);
            let error = match StateFileLock::acquire_for_trust_head_inspection(&state_path) {
                Ok(_) => panic!("missing {missing} must fail closed"),
                Err(error) => error,
            };
            assert!(
                error.to_string().contains("refusing to recreate"),
                "unexpected missing-{missing} error: {error}"
            );
            assert_eq!(
                snapshot_directory_without_atime(&directory),
                before,
                "missing-{missing} preflight recreated or modified recovery state"
            );
            cleanup_anchor_store_fixture(&state_path);
            fs::remove_dir(&directory).expect("remove missing-prerequisite directory");
        }
    }

    #[test]
    #[cfg(unix)]
    fn guarded_cow_publication_detects_lock_replacement_after_rename_and_fsync() {
        let _guard = lock_test_state();
        let (state_path, transition, _) = bootstrap_trust_store_fixture("guarded-cow-lock-swap");
        let store = StateFileLock::acquire_for_trust_transition(&state_path, &transition)
            .expect("open guarded publication store");
        let mut target_name = store.state_name.clone();
        target_name.push(".guarded-cow-test");
        validate_entry_name(&target_name, "guarded COW test target").expect("valid target name");
        ensure_entry_absent(
            store.directory.as_raw_fd(),
            &target_name,
            "guarded COW test target",
        )
        .expect("test target absent");

        let restore_lock = |store: &StateFileLock| {
            let mut displaced_name = store.lock_name.clone();
            displaced_name.push(".test-post-publication-displaced");
            unlinkat_entry(store.directory.as_raw_fd(), &store.lock_name)
                .expect("remove replacement lock");
            renameat_same_directory(
                store.directory.as_raw_fd(),
                &displaced_name,
                &store.lock_name,
                "restore publication-window test lock",
            )
            .expect("restore held lock");
            store.directory.sync_all().expect("sync restored lock");
            store
                .state_anchor_trust_mutation_guard()
                .revalidate()
                .expect("restored mutation guard");
        };

        let guard = store.state_anchor_trust_mutation_guard();
        REPLACE_TRUST_LOCK_AFTER_GUARDED_PUBLICATION
            .store(true, std::sync::atomic::Ordering::SeqCst);
        let create_error = create_entry_atomically_with_guard(
            &store.directory,
            &target_name,
            b"created-before-post-publication-revalidation",
            "guarded COW create test",
            Some(&guard),
        )
        .expect_err("post-publication lock replacement must fail guarded create");
        assert!(
            create_error.to_string().contains("replaced"),
            "unexpected guarded-create error: {create_error}"
        );
        assert_eq!(
            fs::read(state_path.with_file_name(&target_name)).expect("published create target"),
            b"created-before-post-publication-revalidation"
        );
        restore_lock(&store);

        let guard = store.state_anchor_trust_mutation_guard();
        REPLACE_TRUST_LOCK_AFTER_GUARDED_PUBLICATION
            .store(true, std::sync::atomic::Ordering::SeqCst);
        let replace_error = replace_durable_entry_with_guard(
            &store.directory,
            &target_name,
            b"replaced-before-post-publication-revalidation",
            "guarded COW replace test",
            Some(&guard),
        )
        .expect_err("post-publication lock replacement must fail guarded replace");
        assert!(
            replace_error.to_string().contains("replaced"),
            "unexpected guarded-replace error: {replace_error}"
        );
        assert_eq!(
            fs::read(state_path.with_file_name(&target_name))
                .expect("published replacement target"),
            b"replaced-before-post-publication-revalidation"
        );
        restore_lock(&store);
        REPLACE_TRUST_LOCK_AFTER_GUARDED_PUBLICATION
            .store(false, std::sync::atomic::Ordering::SeqCst);

        unlinkat_entry(store.directory.as_raw_fd(), &target_name).expect("remove COW test target");
        store.directory.sync_all().expect("sync COW target removal");
        drop(store);
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    #[cfg(unix)]
    fn recovery_preflight_detects_lock_name_replacement_after_flock() {
        let _guard = lock_test_state();
        let (state_path, transition, _) = bootstrap_trust_store_fixture("post-flock-lock-swap");
        let mut store = StateFileLock::acquire_for_trust_transition(&state_path, &transition)
            .expect("open transition store");
        set_state_anchor_trust_transition_fault_for_tests(
            StateAnchorTrustTransitionFaultInjectionPoint::AfterIntentPublication,
        );
        store
            .transition_state_witness_anchor(&transition)
            .expect_err("leave durable recovery intent");
        clear_state_anchor_trust_transition_fault_for_tests();
        let directory = store
            .directory
            .try_clone()
            .expect("clone directory descriptor");
        let lock_name = store.lock_name.clone();
        drop(store);

        REPLACE_TRUST_LOCK_AFTER_FLOCK.store(true, std::sync::atomic::Ordering::SeqCst);
        let error = match StateFileLock::acquire_for_trust_head_inspection(&state_path) {
            Ok(_) => panic!("post-flock lock replacement must fail preflight"),
            Err(error) => error,
        };
        assert!(
            error.to_string().contains("replaced"),
            "unexpected post-flock replacement error: {error}"
        );
        assert!(state_anchor_trust_intent_file_path(&state_path).exists());

        let mut displaced_name = lock_name.clone();
        displaced_name.push(".test-post-flock-displaced");
        unlinkat_entry(directory.as_raw_fd(), &lock_name).expect("remove replacement lock");
        renameat_same_directory(
            directory.as_raw_fd(),
            &displaced_name,
            &lock_name,
            "restore post-flock test lock",
        )
        .expect("restore original lock");
        directory.sync_all().expect("sync restored original lock");
        let recovery = match StateFileLock::acquire_for_trust_head_inspection(&state_path) {
            Ok(_) => panic!("restored intent still requires fresh recovery"),
            Err(error) => error,
        };
        assert!(matches!(
            recovery,
            EngineError::StateAnchorTrustRecoveryRequired { .. }
        ));
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    #[cfg(unix)]
    fn fresh_remote_read_for_a_later_checkpoint_cannot_resume_an_older_intent() {
        let _guard = lock_test_state();
        let (state_path, original, tip) = bootstrap_trust_store_fixture("advanced-read-recovery");
        let mut store = StateFileLock::acquire_for_trust_transition(&state_path, &original)
            .expect("open transition store");
        set_state_anchor_trust_transition_fault_for_tests(
            StateAnchorTrustTransitionFaultInjectionPoint::AfterIntentPublication,
        );
        store
            .transition_state_witness_anchor(&original)
            .expect_err("leave durable intent before mutation");
        clear_state_anchor_trust_transition_fault_for_tests();
        drop(store);

        let store_fingerprint = original.certificates[0].signer_store_fingerprint;
        let advanced_state_image_digest = [0x91; 32];
        let advanced_tip = StateWitness {
            generation: tip.generation + 1,
            previous_commitment: tip.commitment,
            commitment: state_commitment(
                &store_fingerprint,
                tip.generation + 1,
                &tip.commitment,
                &advanced_state_image_digest,
            ),
            state_image_digest: advanced_state_image_digest,
        };
        let now = u64::try_from(
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("clock after epoch")
                .as_millis(),
        )
        .expect("clock fits u64");
        let advanced = bootstrap_state_anchor_trust_transition_for_tests(
            store_fingerprint,
            &advanced_tip,
            now,
            now + 30_000,
            now,
            now + 30_000,
            true,
        )
        .expect("build fresh remote-advanced Read");

        let embedded = &original.certificates[0].target_acknowledgement;
        let refreshed_exact = bootstrap_state_anchor_trust_transition_for_tests(
            store_fingerprint,
            &tip,
            embedded.committed_at_unix_ms,
            embedded.expires_at_unix_ms,
            now,
            now + 30_000,
            true,
        )
        .expect("restore original pins with refreshed exact Read");
        assert_eq!(
            refreshed_exact.request.certificate_chain,
            original.request.certificate_chain
        );

        let mut mixed_request = refreshed_exact.request.clone();
        mixed_request.target_read_response_base64 =
            advanced.request.target_read_response_base64.clone();
        let mixed = verify_state_anchor_trust_transition_request(mixed_request, true)
            .expect("remote-advanced Read is otherwise fresh and correctly signed");
        assert_ne!(
            mixed
                .target_read_acknowledgement
                .checkpoint_state_commitment,
            original.certificates[0]
                .target_acknowledgement
                .checkpoint_state_commitment
        );

        let error = match StateFileLock::acquire_for_trust_transition(&state_path, &mixed) {
            Ok(_) => panic!("later remote checkpoint must not resume older local intent"),
            Err(error) => error,
        };
        assert!(
            error
                .to_string()
                .contains("freshly read target acknowledgement differs"),
            "unexpected advanced-Read recovery error: {error}"
        );
        assert!(state_anchor_trust_intent_file_path(&state_path).exists());
        assert!(!state_anchor_trust_file_path(&state_path).exists());
        assert!(!state_anchor_file_path(&state_path).exists());

        let mut resumed =
            StateFileLock::acquire_for_trust_transition(&state_path, &refreshed_exact)
                .expect("exact fresh Read resumes pending intent");
        let replay = resumed
            .transition_state_witness_anchor(&refreshed_exact)
            .expect("post-recovery exact replay");
        assert!(replay.idempotent);
        drop(resumed);
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    fn signed_segment_header_matches_frozen_472_byte_vector() {
        let acknowledgement = fixture_acknowledgement();
        let header = encode_state_witness_segment_header(
            &acknowledgement.checkpoint_store_fingerprint,
            &acknowledgement,
        )
        .expect("encode fixed segment header");
        assert_eq!(header.len(), 472);
        assert_eq!(
            hex::encode(&header),
            concat!(
                "544254435749544e455353534547310000000001000001d81111111111111111111111111111111111111111111111111111111111111111",
                "000000000000002a222222222222222222222222222222222222222222222222222222222222222233333333333333333333333333333333",
                "33333333333333333333333333333333ea5eb04a4776357e59875f683390a2ff4b7dd511ad394e588dfab147f94fa8674444444444444444",
                "4444444444444444444444444444444444444444444444440000000000000007000000000000000100000000000000000000000000000000",
                "0000000000000000000000000000000055555555555555555555555555555555555555555555555555555555555555556666666666666666",
                "6666666666666666666666666666666666666666666666667777777777777777777777777777777777777777777777777777777777777777",
                "00000000075bcd15bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb99999999999999999999999999999999",
                "999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999004c908654bcdcd2",
                "92d7711cecda4233a4adb5755107d0bfae36a7b8c4e454c0"
            )
        );
        let metadata = StateAnchorMetadata {
            latest: acknowledgement.clone(),
            witness_base: Some(acknowledgement),
            pending_witness_base: None,
        };
        let parsed = parse_state_witness_segment_header(&header, &[0x11; 32], Some(&metadata))
            .expect("parse frozen segment header");
        assert_eq!(parsed.base.generation, 42);
    }

    #[test]
    #[cfg(unix)]
    fn incomplete_witness_repair_covers_both_headers_and_fails_closed_elsewhere() {
        let store_id = [0x24; 32];
        let store_fingerprint = durable_store_fingerprint(&store_id);
        let state_digest = [0x33; 32];
        let previous_commitment = state_witness_genesis(&store_fingerprint);
        let genesis = StateWitness {
            generation: 1,
            previous_commitment,
            state_image_digest: state_digest,
            commitment: state_commitment(
                &store_fingerprint,
                1,
                &previous_commitment,
                &state_digest,
            ),
        };
        let mut genesis_journal = Vec::new();
        genesis_journal.extend_from_slice(TBTC_SIGNER_STATE_WITNESS_MAGIC);
        genesis_journal.extend_from_slice(&store_id);
        genesis_journal.extend_from_slice(&encode_state_witness_record(
            TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE,
            &genesis,
        ));
        genesis_journal.extend_from_slice(&encode_state_witness_record(
            TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT,
            &genesis,
        ));

        let acknowledgement = fixture_acknowledgement();
        let segment_header = encode_state_witness_segment_header(
            &acknowledgement.checkpoint_store_fingerprint,
            &acknowledgement,
        )
        .expect("encode segment header fixture");
        let anchor = StateAnchorMetadata {
            latest: acknowledgement.clone(),
            witness_base: Some(acknowledgement.clone()),
            pending_witness_base: None,
        };
        let next_digest = [0x34; 32];
        let segment_next = StateWitness {
            generation: acknowledgement.checkpoint_generation + 1,
            previous_commitment: acknowledgement.checkpoint_state_commitment,
            state_image_digest: next_digest,
            commitment: state_commitment(
                &acknowledgement.checkpoint_store_fingerprint,
                acknowledgement.checkpoint_generation + 1,
                &acknowledgement.checkpoint_state_commitment,
                &next_digest,
            ),
        };
        let segment_record =
            encode_state_witness_record(TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE, &segment_next);

        let mut random = [0u8; 12];
        OsRng.fill_bytes(&mut random);
        let fixture_path = std::env::temp_dir().join(format!(
            "tbtc-signer-witness-repair-{}-{}",
            std::process::id(),
            hex::encode(random)
        ));
        let file = fs::OpenOptions::new()
            .read(true)
            .write(true)
            .create_new(true)
            .open(&fixture_path)
            .expect("create witness repair fixture");
        let install = |bytes: &[u8]| {
            write_file_at(&file, bytes, "witness repair fixture")
                .expect("write witness repair fixture");
            file.sync_all().expect("sync witness repair fixture");
        };

        for partial_length in [1, TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH - 1] {
            let mut torn = genesis_journal.clone();
            torn.extend_from_slice(&segment_record[..partial_length]);
            install(&torn);
            let repaired =
                truncate_incomplete_witness_record(&file, &store_id, &store_fingerprint, None)
                    .expect("repair torn genesis-journal append");
            assert_eq!(repaired, genesis_journal.len());
            assert_eq!(
                usize::try_from(file.metadata().expect("stat repaired journal").len())
                    .expect("journal length fits usize"),
                genesis_journal.len()
            );
            read_state_witness_journal_streaming(&file, &store_id, &store_fingerprint, 8, None)
                .expect("repaired genesis journal verifies");
        }

        for partial_length in [1, TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH - 1] {
            let mut torn = segment_header.clone();
            torn.extend_from_slice(&segment_record[..partial_length]);
            install(&torn);
            let repaired = truncate_incomplete_witness_record(
                &file,
                &store_id,
                &acknowledgement.checkpoint_store_fingerprint,
                Some(&anchor),
            )
            .expect("repair torn signed-segment append");
            assert_eq!(repaired, TBTC_SIGNER_STATE_WITNESS_SEGMENT_HEADER_LENGTH);
            read_state_witness_journal_streaming(
                &file,
                &store_id,
                &acknowledgement.checkpoint_store_fingerprint,
                8,
                Some(&anchor),
            )
            .expect("repaired signed segment verifies");
        }

        for short_length in [
            0,
            TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH - 1,
            TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH,
            TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH + TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH,
        ] {
            let mut short = genesis_journal.clone();
            short.truncate(short_length);
            install(&short);
            let retained =
                truncate_incomplete_witness_record(&file, &store_id, &store_fingerprint, None)
                    .expect("short journal is inspected without repair");
            assert_eq!(retained, short_length);
            assert_eq!(
                usize::try_from(file.metadata().expect("stat short journal").len())
                    .expect("journal length fits usize"),
                short_length
            );
            let error = match read_state_witness_journal_streaming(
                &file,
                &store_id,
                &store_fingerprint,
                8,
                None,
            ) {
                Ok(_) => panic!("short or uncommitted genesis journal must fail closed"),
                Err(error) => error,
            };
            assert!(
                error.to_string().contains("shorter")
                    || error.to_string().contains("missing")
                    || error.to_string().contains("no committed genesis"),
                "unexpected short-journal error: {error}"
            );
        }

        let retired =
            encode_v1_state_witness_genesis_journal(&store_id, &[0x11; 32], &state_digest);
        install(&retired);
        let retained =
            truncate_incomplete_witness_record(&file, &store_id, &store_fingerprint, None)
                .expect("retired journal is left untouched");
        assert_eq!(retained, retired.len());
        let error = match read_state_witness_journal_streaming(
            &file,
            &store_id,
            &store_fingerprint,
            8,
            None,
        ) {
            Ok(_) => panic!("retired v1 journal must fail closed in production reader"),
            Err(error) => error,
        };
        assert!(error
            .to_string()
            .contains("retired v1 state-commitment transcript"));
        drop(file);
        fs::remove_file(fixture_path).expect("remove witness repair fixture");
    }

    #[test]
    fn ordinary_anchor_history_allows_a_retained_anchor_behind_the_local_tip() {
        let mut acknowledgement = fixture_acknowledgement();
        let anchored = StateWitness {
            generation: acknowledgement.checkpoint_generation,
            previous_commitment: acknowledgement.checkpoint_previous_commitment,
            state_image_digest: acknowledgement.checkpoint_state_image_digest,
            commitment: acknowledgement.checkpoint_state_commitment,
        };
        let next_image = [0x5c; 32];
        let tip = StateWitness {
            generation: anchored.generation + 1,
            previous_commitment: anchored.commitment,
            state_image_digest: next_image,
            commitment: state_commitment(
                &acknowledgement.checkpoint_store_fingerprint,
                anchored.generation + 1,
                &anchored.commitment,
                &next_image,
            ),
        };
        acknowledgement.checkpoint_generation = anchored.generation;
        let metadata = StateAnchorMetadata {
            latest: acknowledgement.clone(),
            witness_base: Some(acknowledgement),
            pending_witness_base: None,
        };

        validate_anchor_history(Some(&metadata), &[anchored, tip])
            .expect("ordinary reconciliation may advance a retained anchor to the local tip");
    }

    fn signed_fixture() -> (StateAnchorConfiguration, StateAnchorAcknowledgement) {
        let signing_key = SigningKey::from_bytes(&[0x07; 32]);
        let response_public_key = signing_key.verifying_key().to_bytes();
        let mut spki_digest = Sha256::new();
        spki_digest.update([
            0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x03, 0x21, 0x00,
        ]);
        spki_digest.update(response_public_key);
        let configured_spki_hash: [u8; 32] = spki_digest.finalize().into();
        let mut acknowledgement = fixture_acknowledgement();
        acknowledgement.configured_spki_hash = configured_spki_hash;
        acknowledgement.event_root = state_anchor_event_root_for_tests(&acknowledgement);
        acknowledgement.signing_digest = state_anchor_signing_digest_for_tests(&acknowledgement);
        acknowledgement.signature = signing_key.sign(&acknowledgement.signing_digest).to_bytes();
        let mut acknowledgement_digest = Sha256::new();
        acknowledgement_digest.update(b"tbtc-signer-state-anchor-acknowledgement/v1\0");
        acknowledgement_digest.update(acknowledgement.signing_digest);
        acknowledgement_digest.update(acknowledgement.signature);
        acknowledgement_digest.update(configured_spki_hash);
        acknowledgement.acknowledgement_digest = acknowledgement_digest.finalize().into();
        (
            StateAnchorConfiguration {
                binding_hash: acknowledgement.binding_hash,
                response_public_key,
                response_public_key_spki_sha256: configured_spki_hash,
                rotation_threshold_records: 8,
                trust: None,
            },
            acknowledgement,
        )
    }

    #[cfg(unix)]
    fn configure_anchor_store_fixture(
        state_path: &Path,
        signing_key: &SigningKey,
        binding_hash: [u8; 32],
    ) -> [u8; 32] {
        let response_public_key = signing_key.verifying_key().to_bytes();
        let mut spki_digest = Sha256::new();
        spki_digest.update([
            0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x03, 0x21, 0x00,
        ]);
        spki_digest.update(response_public_key);
        let configured_spki_hash: [u8; 32] = spki_digest.finalize().into();
        std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, state_path);
        std::env::set_var(TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS_ENV, "4");
        std::env::set_var(
            TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS_ENV,
            "2",
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH_ENV,
            bytes32_hex(binding_hash),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_ENV,
            bytes32_hex(response_public_key),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256_ENV,
            bytes32_hex(configured_spki_hash),
        );
        configured_spki_hash
    }

    #[cfg(unix)]
    fn signed_acknowledgement_for_tip(
        signing_key: &SigningKey,
        configured_spki_hash: [u8; 32],
        store_fingerprint: [u8; 32],
        tip: &StateWitness,
    ) -> StateAnchorAcknowledgement {
        let now = u64::try_from(
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("clock after epoch")
                .as_millis(),
        )
        .expect("clock fits u64");
        let mut acknowledgement = fixture_acknowledgement();
        acknowledgement.checkpoint_store_fingerprint = store_fingerprint;
        acknowledgement.checkpoint_generation = tip.generation;
        acknowledgement.checkpoint_previous_commitment = tip.previous_commitment;
        acknowledgement.checkpoint_state_image_digest = tip.state_image_digest;
        acknowledgement.checkpoint_state_commitment = tip.commitment;
        acknowledgement.committed_at_unix_ms = now;
        acknowledgement.expires_at_unix_ms = now + 30_000;
        acknowledgement.configured_spki_hash = configured_spki_hash;
        acknowledgement.event_root = state_anchor_event_root_for_tests(&acknowledgement);
        acknowledgement.signing_digest = state_anchor_signing_digest_for_tests(&acknowledgement);
        acknowledgement.signature = signing_key.sign(&acknowledgement.signing_digest).to_bytes();
        let mut acknowledgement_digest = Sha256::new();
        acknowledgement_digest.update(b"tbtc-signer-state-anchor-acknowledgement/v1\0");
        acknowledgement_digest.update(acknowledgement.signing_digest);
        acknowledgement_digest.update(acknowledgement.signature);
        acknowledgement_digest.update(configured_spki_hash);
        acknowledgement.acknowledgement_digest = acknowledgement_digest.finalize().into();
        acknowledgement
    }

    #[cfg(unix)]
    fn cleanup_anchor_store_fixture(state_path: &Path) {
        let witness_path = state_witness_file_path(state_path);
        let mut witness_next = witness_path.as_os_str().to_os_string();
        witness_next.push(TBTC_SIGNER_STATE_WITNESS_NEXT_SUFFIX);
        let mut witness_previous = witness_path.as_os_str().to_os_string();
        witness_previous.push(TBTC_SIGNER_STATE_WITNESS_PREVIOUS_SUFFIX);
        for path in [
            state_path.to_path_buf(),
            state_lock_file_path(state_path),
            durable_store_id_file_path(state_path),
            witness_path,
            PathBuf::from(witness_next),
            PathBuf::from(witness_previous),
            state_anchor_file_path(state_path),
            state_anchor_trust_file_path(state_path),
            state_anchor_trust_intent_file_path(state_path),
        ] {
            let _ = fs::remove_file(path);
        }
    }

    #[cfg(unix)]
    fn bootstrap_trust_store_fixture(
        label: &str,
    ) -> (PathBuf, VerifiedStateAnchorTrustTransition, StateWitness) {
        let mut random = [0u8; 12];
        OsRng.fill_bytes(&mut random);
        let state_path = std::env::temp_dir().join(format!(
            "tbtc-signer-trust-{label}-{}-{}",
            std::process::id(),
            hex::encode(random)
        ));
        let (transition, tip) = bootstrap_trust_store_fixture_at(&state_path);
        (state_path, transition, tip)
    }

    #[cfg(unix)]
    fn bootstrap_trust_store_fixture_at(
        state_path: &Path,
    ) -> (VerifiedStateAnchorTrustTransition, StateWitness) {
        std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, state_path);
        std::env::set_var(TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS_ENV, "4");
        let mut initial = StateFileLock::acquire(state_path).expect("open unanchored store");
        let tip = initial.state_witness_tip().expect("unanchored genesis tip");
        let store_fingerprint = initial.identity.fingerprint;
        drop(initial);

        let now = u64::try_from(
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("clock after epoch")
                .as_millis(),
        )
        .expect("clock fits u64");
        let transition = bootstrap_state_anchor_trust_transition_for_tests(
            store_fingerprint,
            &tip,
            now,
            now + 30_000,
            now,
            now + 30_000,
            true,
        )
        .expect("build verified bootstrap transition");
        (transition, tip)
    }

    #[test]
    #[cfg(unix)]
    fn bootstrap_trust_transition_succeeds_on_first_call_and_ordinary_reopen() {
        let _guard = lock_test_state();
        let (state_path, transition, tip) = bootstrap_trust_store_fixture("bootstrap");
        let mut store = StateFileLock::acquire_for_trust_transition(&state_path, &transition)
            .expect("acquire transition store");
        let outcome = store
            .transition_state_witness_anchor(&transition)
            .expect("bootstrap succeeds on its first call");
        assert!(!outcome.idempotent);
        assert_eq!(outcome.applied_certificate_count, 1);
        assert_eq!(outcome.tip, tip);
        assert_eq!(outcome.base, tip);
        assert_eq!(
            outcome.trust_head.certificate_digest,
            transition.certificates[0].certificate_digest
        );
        ensure_entry_absent(
            store.directory.as_raw_fd(),
            &store.trust_intent_name,
            "completed trust transition intent",
        )
        .expect("completed intent removed");
        drop(store);

        let mut reopened = StateFileLock::acquire(&state_path).expect("ordinary target reopen");
        let snapshot = reopened
            .state_anchor_trust_head_snapshot()
            .expect("ordinary reopened trust head");
        assert_eq!(snapshot.tip, tip);
        assert_eq!(snapshot.base, tip);
        assert_eq!(
            snapshot.trust_head.certificate_digest,
            transition.certificates[0].certificate_digest
        );
        assert_eq!(
            snapshot.anchor.latest,
            transition.certificates[0].target_acknowledgement
        );
        drop(reopened);
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    #[cfg(unix)]
    fn bootstrap_facts_store_acquisition_is_repeatable_ephemeral_and_pristine_only() {
        let _guard = lock_test_state();
        let mut random = [0u8; 12];
        OsRng.fill_bytes(&mut random);
        let state_path = std::env::temp_dir().join(format!(
            "tbtc-signer-bootstrap-facts-store-{}-{}",
            std::process::id(),
            hex::encode(random)
        ));
        if let Ok(mut slot) = state_file_lock_slot().lock() {
            *slot = None;
        }

        let first = {
            let mut store =
                StateFileLock::acquire_for_bootstrap_facts(&state_path).expect("first acquisition");
            store
                .state_anchor_bootstrap_facts_snapshot()
                .expect("first bootstrap facts")
        };
        let second = {
            let mut store = StateFileLock::acquire_for_bootstrap_facts(&state_path)
                .expect("repeat acquisition");
            store
                .state_anchor_bootstrap_facts_snapshot()
                .expect("repeat bootstrap facts")
        };
        assert_eq!(first, second);
        assert!(state_file_lock_slot().lock().expect("store slot").is_none());

        fs::write(&state_path, b"non-pristine-state-image").expect("publish state entry");
        let mut permissions = fs::metadata(&state_path)
            .expect("state entry metadata")
            .permissions();
        std::os::unix::fs::PermissionsExt::set_mode(&mut permissions, 0o600);
        fs::set_permissions(&state_path, permissions).expect("restrict state entry permissions");
        let error = match StateFileLock::acquire_for_bootstrap_facts(&state_path) {
            Ok(_) => panic!("non-pristine store must reject bootstrap facts"),
            Err(error) => error,
        };
        assert!(
            error.to_string().contains("pristine genesis-only store")
                || error.to_string().contains("state image"),
            "unexpected non-pristine rejection: {error}"
        );
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    #[cfg(unix)]
    fn provisioning_config_ffi_is_startup_only_and_capability_minimal() {
        let mut random = [0u8; 12];
        OsRng.fill_bytes(&mut random);
        let state_path = std::env::temp_dir().join(format!(
            "tbtc-signer-bootstrap-provisioning-{}-{}",
            std::process::id(),
            hex::encode(random)
        ));
        let output =
            std::process::Command::new(std::env::current_exe().expect("current test binary path"))
                .arg("--exact")
                .arg("engine::store::witness_transcript_tests::provisioning_config_ffi_helper")
                .arg("--ignored")
                .arg("--nocapture")
                .env("TBTC_SIGNER_BOOTSTRAP_PROVISIONING_HELPER", "1")
                .env("TBTC_SIGNER_BOOTSTRAP_PROVISIONING_STATE_PATH", &state_path)
                .output()
                .expect("spawn isolated provisioning helper");
        assert!(
            output.status.success(),
            "provisioning helper failed\nstdout:\n{}\nstderr:\n{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        assert!(
            String::from_utf8_lossy(&output.stdout).contains("1 passed"),
            "provisioning helper did not execute exactly one test\nstdout:\n{}",
            String::from_utf8_lossy(&output.stdout)
        );
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    #[ignore = "isolated subprocess helper"]
    #[cfg(unix)]
    fn provisioning_config_ffi_helper() {
        if std::env::var("TBTC_SIGNER_BOOTSTRAP_PROVISIONING_HELPER").as_deref() != Ok("1") {
            return;
        }
        assert!(ENGINE_STATE.get().is_none());
        assert!(state_file_lock_slot().lock().expect("store slot").is_none());
        let state_path = PathBuf::from(
            std::env::var("TBTC_SIGNER_BOOTSTRAP_PROVISIONING_STATE_PATH")
                .expect("helper state path"),
        );

        let call_with_json = |payload: Vec<u8>,
                              function: extern "C" fn(
            *const u8,
            usize,
        ) -> crate::ffi::TbtcSignerResult| {
            let result = function(payload.as_ptr(), payload.len());
            let bytes = if result.buffer.ptr.is_null() || result.buffer.len == 0 {
                Vec::new()
            } else {
                unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len).to_vec() }
            };
            crate::frost_tbtc_free_buffer(result.buffer.ptr, result.buffer.len);
            (result.status_code, bytes)
        };
        let call_without_json = |function: extern "C" fn() -> crate::ffi::TbtcSignerResult| {
            let result = function();
            let bytes = if result.buffer.ptr.is_null() || result.buffer.len == 0 {
                Vec::new()
            } else {
                unsafe { std::slice::from_raw_parts(result.buffer.ptr, result.buffer.len).to_vec() }
            };
            crate::frost_tbtc_free_buffer(result.buffer.ptr, result.buffer.len);
            (result.status_code, bytes)
        };

        let mut provisioning = InitSignerConfigRequest {
            purpose: Some("state_anchor_bootstrap_provisioning".to_string()),
            profile: Some("production".to_string()),
            state_path: Some(state_path.to_string_lossy().into_owned()),
            state_witness_max_records: Some(4),
            ..InitSignerConfigRequest::default()
        };
        provisioning.state_anchor_binding_hash = Some(bytes32_hex([0x41; 32]));
        let (invalid_status, invalid_payload) = call_with_json(
            serde_json::to_vec(&provisioning).expect("invalid config JSON"),
            crate::frost_tbtc_init_signer_config,
        );
        assert_eq!(invalid_status, 1);
        let invalid_error: crate::api::ErrorResponse =
            serde_json::from_slice(&invalid_payload).expect("invalid config error");
        assert!(
            invalid_error
                .message
                .contains("forbids populated field [state_anchor_binding_hash]"),
            "unexpected provisioning pin rejection: {}",
            invalid_error.message
        );

        provisioning.state_anchor_binding_hash = None;
        let (init_status, _) = call_with_json(
            serde_json::to_vec(&provisioning).expect("provisioning config JSON"),
            crate::frost_tbtc_init_signer_config,
        );
        assert_eq!(init_status, 0);

        let (first_status, first_payload) =
            call_without_json(crate::frost_tbtc_state_anchor_bootstrap_facts);
        let (second_status, second_payload) =
            call_without_json(crate::frost_tbtc_state_anchor_bootstrap_facts);
        assert_eq!(first_status, 0);
        assert_eq!(second_status, 0);
        assert_eq!(first_payload, second_payload);
        let facts: StateAnchorBootstrapFactsResult =
            serde_json::from_slice(&first_payload).expect("bootstrap facts result");
        assert_eq!(facts.schema, STATE_ANCHOR_BOOTSTRAP_FACTS_SCHEMA);
        assert_eq!(
            facts.store_fingerprint,
            facts.current_checkpoint.store_fingerprint
        );
        assert_eq!(facts.current_checkpoint.generation, "1");

        let (ordinary_status, ordinary_payload) =
            call_without_json(crate::frost_tbtc_durable_store_identity);
        assert_eq!(ordinary_status, 1);
        let ordinary_error: crate::api::ErrorResponse =
            serde_json::from_slice(&ordinary_payload).expect("ordinary operation error");
        assert!(ordinary_error.message.contains("normal_signer"));

        let dkg_request = DkgPart1Request {
            participant_identifier: "01".to_string(),
            max_signers: 3,
            min_signers: 2,
        };
        let (dkg_status, dkg_payload) = call_with_json(
            serde_json::to_vec(&dkg_request).expect("DKG request JSON"),
            crate::frost_tbtc_dkg_part1,
        );
        assert_eq!(dkg_status, 1);
        let dkg_error: crate::api::ErrorResponse =
            serde_json::from_slice(&dkg_payload).expect("DKG purpose error");
        assert!(dkg_error.message.contains("normal_signer"));

        assert!(ENGINE_STATE.get().is_none());
        assert!(state_file_lock_slot().lock().expect("store slot").is_none());
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    #[cfg(unix)]
    fn committed_trust_head_rejects_missing_anchor_and_legacy_witness_rollback_mix() {
        let _guard = lock_test_state();
        for remove_anchor in [true, false] {
            establish_clean_signer_test_env();
            let label = if remove_anchor {
                "rollback-missing-anchor"
            } else {
                "rollback-legacy-witness"
            };
            let (state_path, transition, _) = bootstrap_trust_store_fixture(label);
            let witness_path = state_witness_file_path(&state_path);
            let legacy_witness = fs::read(&witness_path).expect("read pre-bootstrap witness");
            let mut store = StateFileLock::acquire_for_trust_transition(&state_path, &transition)
                .expect("acquire bootstrap store");
            store
                .transition_state_witness_anchor(&transition)
                .expect("install committed trust head");
            drop(store);

            if remove_anchor {
                fs::remove_file(state_anchor_file_path(&state_path))
                    .expect("remove anchor rollback component");
                fs::write(&witness_path, &legacy_witness)
                    .expect("restore pre-bootstrap witness rollback component");
            } else {
                fs::write(&witness_path, &legacy_witness)
                    .expect("restore pre-bootstrap witness component");
            }
            let error = StateFileLock::acquire(&state_path)
                .err()
                .expect("mixed pre/post-bootstrap rollback state must fail closed");
            if remove_anchor {
                assert!(
                    error
                        .to_string()
                        .contains("requires persisted anchor metadata"),
                    "unexpected missing-anchor error: {error}"
                );
            } else {
                assert!(
                    error
                        .to_string()
                        .contains("requires an authenticated witness segment"),
                    "unexpected legacy-witness error: {error}"
                );
            }
            cleanup_anchor_store_fixture(&state_path);
        }
    }

    #[test]
    #[cfg(unix)]
    fn durable_bootstrap_intent_requires_fresh_resubmission_at_every_publication_boundary() {
        let _guard = lock_test_state();
        let fault_points = [
            StateAnchorTrustTransitionFaultInjectionPoint::AfterIntentPublication,
            StateAnchorTrustTransitionFaultInjectionPoint::AfterPrepareBatch,
            StateAnchorTrustTransitionFaultInjectionPoint::AfterNextWitnessPublication,
            StateAnchorTrustTransitionFaultInjectionPoint::AfterPreviousWitnessPublication,
            StateAnchorTrustTransitionFaultInjectionPoint::AfterCurrentWitnessPublication,
            StateAnchorTrustTransitionFaultInjectionPoint::AfterTargetAnchorPublication,
            StateAnchorTrustTransitionFaultInjectionPoint::AfterCommitPublication,
            StateAnchorTrustTransitionFaultInjectionPoint::AfterPreviousWitnessRetirement,
        ];
        for point in fault_points {
            establish_clean_signer_test_env();
            let (state_path, transition, tip) =
                bootstrap_trust_store_fixture(&format!("{point:?}"));
            let mut store = StateFileLock::acquire_for_trust_transition(&state_path, &transition)
                .expect("acquire transition store");
            set_state_anchor_trust_transition_fault_for_tests(point);
            let error = store
                .transition_state_witness_anchor(&transition)
                .expect_err("fault interrupts fresh transition");
            assert!(
                error
                    .to_string()
                    .contains("injected state-anchor trust transition fault"),
                "unexpected fault result at {point:?}: {error}"
            );
            clear_state_anchor_trust_transition_fault_for_tests();
            let intent_path = state_anchor_trust_intent_file_path(&state_path);
            assert!(
                intent_path.exists(),
                "durable intent must survive fault at {point:?}"
            );
            drop(store);

            let recovery_error = match StateFileLock::acquire_for_trust_head_inspection(&state_path)
            {
                Ok(_) => panic!("preflight must not recover locally at {point:?}"),
                Err(error) => error,
            };
            let EngineError::StateAnchorTrustRecoveryRequired { context } = recovery_error else {
                panic!("unexpected preflight result at {point:?}: {recovery_error}");
            };
            assert_eq!(
                context.store_fingerprint,
                transition.certificates[0].signer_store_fingerprint
            );
            assert_eq!(
                context.certificate_digests,
                vec![transition.certificates[0].certificate_digest]
            );
            assert!(
                intent_path.exists(),
                "preflight must leave the intent untouched at {point:?}"
            );

            let mut resumed = StateFileLock::acquire_for_trust_transition(&state_path, &transition)
                .unwrap_or_else(|error| {
                    panic!("fresh transition resumption failed at {point:?}: {error}")
                });
            let replay = resumed
                .transition_state_witness_anchor(&transition)
                .unwrap_or_else(|error| {
                    panic!("post-recovery replay failed at {point:?}: {error}")
                });
            assert!(replay.idempotent);
            drop(resumed);

            let mut inspection = StateFileLock::acquire_for_trust_head_inspection(&state_path)
                .unwrap_or_else(|error| {
                    panic!("post-recovery preflight failed at {point:?}: {error}")
                });
            let recovered = inspection
                .state_anchor_trust_head_snapshot()
                .unwrap_or_else(|error| {
                    panic!("recovered trust head failed at {point:?}: {error}")
                });
            assert_eq!(recovered.tip, tip, "tip changed at {point:?}");
            assert_eq!(recovered.base, tip, "base changed at {point:?}");
            assert_eq!(
                recovered.trust_head.certificate_digest,
                transition.certificates[0].certificate_digest,
                "wrong recovered head at {point:?}"
            );
            assert!(
                !intent_path.exists(),
                "preflight must retire recovered intent at {point:?}"
            );
            drop(inspection);

            let mut ordinary = StateFileLock::acquire(&state_path)
                .unwrap_or_else(|error| panic!("ordinary reopen failed at {point:?}: {error}"));
            assert_eq!(
                ordinary
                    .state_witness_tip()
                    .expect("ordinary recovered tip"),
                tip,
                "ordinary tip changed at {point:?}"
            );
            drop(ordinary);
            cleanup_anchor_store_fixture(&state_path);
        }
    }

    #[test]
    #[cfg(unix)]
    fn exact_bootstrap_replay_requires_a_fresh_read_and_never_recreates_intent() {
        let _guard = lock_test_state();
        let (state_path, transition, tip) = bootstrap_trust_store_fixture("replay");
        let mut first = StateFileLock::acquire_for_trust_transition(&state_path, &transition)
            .expect("acquire initial bootstrap");
        first
            .transition_state_witness_anchor(&transition)
            .expect("initial bootstrap");
        drop(first);

        let mut replay = StateFileLock::acquire_for_trust_transition(&state_path, &transition)
            .expect("acquire exact replay");
        let replayed = replay
            .transition_state_witness_anchor(&transition)
            .expect("fresh exact replay");
        assert!(replayed.idempotent);
        assert_eq!(replayed.applied_certificate_count, 0);
        drop(replay);

        let now = u64::try_from(
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("clock after epoch")
                .as_millis(),
        )
        .expect("clock fits u64");
        let nested = &transition.certificates[0].target_acknowledgement;
        let expired = bootstrap_state_anchor_trust_transition_for_tests(
            nested.checkpoint_store_fingerprint,
            &tip,
            nested.committed_at_unix_ms,
            nested.expires_at_unix_ms,
            now - 60_000,
            now - 30_000,
            false,
        )
        .expect("intrinsically valid expired Read");
        assert_eq!(
            expired.certificates[0].certificate_digest,
            transition.certificates[0].certificate_digest
        );
        let mut expired_store = StateFileLock::acquire_for_trust_transition(&state_path, &expired)
            .expect("acquire expired exact replay for store-level admission check");
        let error = expired_store
            .transition_state_witness_anchor(&expired)
            .expect_err("expired exact replay rejected");
        assert!(
            error.to_string().contains("expired"),
            "unexpected expired replay error: {error}"
        );
        ensure_entry_absent(
            expired_store.directory.as_raw_fd(),
            &expired_store.trust_intent_name,
            "expired exact replay intent",
        )
        .expect("expired replay cannot publish intent");
        drop(expired_store);
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    #[cfg(unix)]
    fn trust_journal_batch_capacity_preflight_rejects_before_intent_publication() {
        let _guard = lock_test_state();
        let (state_path, transition, _) = bootstrap_trust_store_fixture("capacity");
        let mut store = StateFileLock::acquire_for_trust_transition(&state_path, &transition)
            .expect("acquire capacity fixture");
        let certificate = &transition.certificates[0];
        let prepare_length = encode_state_anchor_trust_prepare_record(
            &store.identity.fingerprint,
            &[0u8; 32],
            certificate,
        )
        .expect("encode PREPARE")
        .len();
        let commit_length = encode_state_anchor_trust_commit_record(
            &store.identity.fingerprint,
            &[0u8; 32],
            certificate,
        )
        .expect("encode COMMIT")
        .len();
        for current_length in [
            STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH - prepare_length + 1,
            STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH - prepare_length - commit_length + 1,
        ] {
            let growth = store
                .state_anchor_trust_transition_journal_growth(&transition)
                .expect("compute exact journal growth");
            let error = ensure_state_anchor_trust_transition_journal_capacity_for_length(
                current_length,
                growth - STATE_ANCHOR_TRUST_JOURNAL_HEADER_LENGTH,
            )
            .expect_err("near-capacity batch rejected");
            assert!(error.to_string().contains("size bound"));
            ensure_entry_absent(
                store.directory.as_raw_fd(),
                &store.trust_intent_name,
                "capacity-rejected trust intent",
            )
            .expect("capacity preflight cannot publish intent");
        }
        let intent_bytes = encode_state_anchor_trust_transition_intent(
            &store.identity.fingerprint,
            &transition.request,
        )
        .expect("encode boundary-test intent");
        let error = store
            .create_state_anchor_trust_transition_intent(
                &intent_bytes,
                transition.target_read_expires_at_unix_ms,
                STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH + 1,
            )
            .expect_err("final pre-rename capacity check rejects publication");
        assert!(error.to_string().contains("size bound"));
        ensure_entry_absent(
            store.directory.as_raw_fd(),
            &store.trust_intent_name,
            "capacity-rejected live trust intent",
        )
        .expect("oversized batch cannot publish its intent");
        let temp_prefix = format!("{}.tmp-", store.trust_intent_name.to_string_lossy());
        let parent = state_path.parent().expect("fixture parent");
        assert!(
            fs::read_dir(parent)
                .expect("read fixture directory")
                .filter_map(Result::ok)
                .all(|entry| !entry
                    .file_name()
                    .to_string_lossy()
                    .starts_with(&temp_prefix)),
            "capacity-rejected intent temp must be cleaned up"
        );
        drop(store);
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    fn anchor_metadata_round_trips_and_rejects_recommitted_signature_tampering() {
        let (configuration, acknowledgement) = signed_fixture();
        let metadata = StateAnchorMetadata {
            latest: acknowledgement.clone(),
            witness_base: None,
            pending_witness_base: Some(acknowledgement.clone()),
        };
        let bytes =
            encode_state_anchor_metadata(&acknowledgement.checkpoint_store_fingerprint, &metadata);
        assert_eq!(
            parse_state_anchor_metadata(
                &bytes,
                &acknowledgement.checkpoint_store_fingerprint,
                &configuration,
                &[],
            )
            .expect("parse signed metadata"),
            metadata
        );

        const ACK_SIGNATURE_OFFSET: usize = 432;
        let mut tampered = bytes;
        tampered[56 + ACK_SIGNATURE_OFFSET] ^= 1;
        let commitment_offset = tampered.len() - 32;
        let commitment = state_anchor_metadata_commitment(&tampered[..commitment_offset]);
        tampered[commitment_offset..].copy_from_slice(&commitment);
        let error = parse_state_anchor_metadata(
            &tampered,
            &acknowledgement.checkpoint_store_fingerprint,
            &configuration,
            &[],
        )
        .expect_err("a recommitted but signature-tampered anchor must fail");
        assert!(error.to_string().contains("signature"));

        // The test-only path helper remains pinned to the externally documented
        // suffix used by offline recovery tooling.
        assert!(state_anchor_file_path(Path::new("/tmp/state")).ends_with("state.state-anchor"));
    }

    #[test]
    fn anchor_metadata_rejects_adjacent_base_latest_fork_splice() {
        let (configuration, base) = signed_fixture();
        let signing_key = SigningKey::from_bytes(&[0x07; 32]);
        let mut latest = base.clone();
        latest.revision = base.revision + 1;
        latest.previous_event_root = [0x99; 32];
        latest.event_root = state_anchor_event_root_for_tests(&latest);
        latest.signing_digest = state_anchor_signing_digest_for_tests(&latest);
        latest.signature = signing_key.sign(&latest.signing_digest).to_bytes();
        let mut acknowledgement_digest = Sha256::new();
        acknowledgement_digest.update(b"tbtc-signer-state-anchor-acknowledgement/v1\0");
        acknowledgement_digest.update(latest.signing_digest);
        acknowledgement_digest.update(latest.signature);
        acknowledgement_digest.update(latest.configured_spki_hash);
        latest.acknowledgement_digest = acknowledgement_digest.finalize().into();

        let metadata = StateAnchorMetadata {
            latest: latest.clone(),
            witness_base: Some(base.clone()),
            pending_witness_base: None,
        };
        let bytes = encode_state_anchor_metadata(&latest.checkpoint_store_fingerprint, &metadata);
        let error = parse_state_anchor_metadata(
            &bytes,
            &latest.checkpoint_store_fingerprint,
            &configuration,
            &[],
        )
        .expect_err("adjacent signed fork splice must fail");
        assert!(error.to_string().contains("inconsistent"));

        latest.previous_event_root = base.event_root;
        latest.event_root = state_anchor_event_root_for_tests(&latest);
        latest.signing_digest = state_anchor_signing_digest_for_tests(&latest);
        latest.signature = signing_key.sign(&latest.signing_digest).to_bytes();
        let mut acknowledgement_digest = Sha256::new();
        acknowledgement_digest.update(b"tbtc-signer-state-anchor-acknowledgement/v1\0");
        acknowledgement_digest.update(latest.signing_digest);
        acknowledgement_digest.update(latest.signature);
        acknowledgement_digest.update(latest.configured_spki_hash);
        latest.acknowledgement_digest = acknowledgement_digest.finalize().into();
        let linked = StateAnchorMetadata {
            latest: latest.clone(),
            witness_base: Some(base),
            pending_witness_base: None,
        };
        let bytes = encode_state_anchor_metadata(&latest.checkpoint_store_fingerprint, &linked);
        assert_eq!(
            parse_state_anchor_metadata(
                &bytes,
                &latest.checkpoint_store_fingerprint,
                &configuration,
                &[],
            )
            .expect("adjacent linked metadata"),
            linked
        );
    }

    #[test]
    fn monotonic_anchor_rules_reject_replays_forks_gaps_and_epoch_changes() {
        let (_, first) = signed_fixture();
        assert!(!validate_anchor_monotonic_update(None, &first, false).expect("first ack"));
        let existing = StateAnchorMetadata {
            latest: first.clone(),
            witness_base: None,
            pending_witness_base: None,
        };
        assert!(
            validate_anchor_monotonic_update(Some(&existing), &first, false).expect("exact replay")
        );

        let mut next = first.clone();
        next.revision = 2;
        next.previous_event_root = first.event_root;
        next.event_root = [0x56; 32];
        assert!(
            !validate_anchor_monotonic_update(Some(&existing), &next, false).expect("next ack")
        );

        let mut same_revision_fork = first.clone();
        same_revision_fork.event_root = [0x57; 32];
        assert!(
            validate_anchor_monotonic_update(Some(&existing), &same_revision_fork, false).is_err()
        );
        let mut wrong_parent = next.clone();
        wrong_parent.previous_event_root = [0x58; 32];
        assert!(validate_anchor_monotonic_update(Some(&existing), &wrong_parent, false).is_err());
        let mut gap = next.clone();
        gap.revision = 3;
        assert!(validate_anchor_monotonic_update(Some(&existing), &gap, false).is_err());
        let mut stale = first.clone();
        stale.revision = 0;
        assert!(validate_anchor_monotonic_update(Some(&existing), &stale, false).is_err());
        let mut epoch_change = next;
        epoch_change.service_epoch += 1;
        assert!(validate_anchor_monotonic_update(Some(&existing), &epoch_change, false).is_err());

        let mut recovered = first;
        recovered.revision = 7;
        recovered.previous_event_root = [0x59; 32];
        assert!(!validate_anchor_monotonic_update(None, &recovered, true)
            .expect("fresh recovery may restore a later revision without local metadata"));
        assert!(validate_anchor_monotonic_update(None, &recovered, false).is_err());
    }

    #[test]
    #[cfg(unix)]
    fn tip_settles_replayed_anchor_after_prepare_abort_reaches_rotation_threshold() {
        let _guard = lock_test_state();
        let mut random = [0u8; 12];
        OsRng.fill_bytes(&mut random);
        let state_path = std::env::temp_dir().join(format!(
            "tbtc-signer-anchor-abort-{}-{}",
            std::process::id(),
            hex::encode(random)
        ));
        std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &state_path);
        std::env::set_var(TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS_ENV, "4");
        std::env::set_var(
            TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS_ENV,
            "2",
        );

        let signing_key = SigningKey::from_bytes(&[0x07; 32]);
        let response_public_key = signing_key.verifying_key().to_bytes();
        let mut spki_digest = Sha256::new();
        spki_digest.update([
            0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x03, 0x21, 0x00,
        ]);
        spki_digest.update(response_public_key);
        let configured_spki_hash: [u8; 32] = spki_digest.finalize().into();
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH_ENV,
            bytes32_hex([0x44; 32]),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_ENV,
            bytes32_hex(response_public_key),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256_ENV,
            bytes32_hex(configured_spki_hash),
        );

        let mut store = StateFileLock::acquire(&state_path).expect("open anchored store");
        let tip = store.state_witness_tip().expect("genesis tip");
        let now = u64::try_from(
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("clock after epoch")
                .as_millis(),
        )
        .expect("clock fits u64");
        let mut acknowledgement = fixture_acknowledgement();
        acknowledgement.checkpoint_store_fingerprint = store.identity.fingerprint;
        acknowledgement.checkpoint_generation = tip.generation;
        acknowledgement.checkpoint_previous_commitment = tip.previous_commitment;
        acknowledgement.checkpoint_state_image_digest = tip.state_image_digest;
        acknowledgement.checkpoint_state_commitment = tip.commitment;
        acknowledgement.committed_at_unix_ms = now;
        acknowledgement.expires_at_unix_ms = now + 30_000;
        acknowledgement.configured_spki_hash = configured_spki_hash;
        acknowledgement.event_root = state_anchor_event_root_for_tests(&acknowledgement);
        acknowledgement.signing_digest = state_anchor_signing_digest_for_tests(&acknowledgement);
        acknowledgement.signature = signing_key.sign(&acknowledgement.signing_digest).to_bytes();
        let mut acknowledgement_digest = Sha256::new();
        acknowledgement_digest.update(b"tbtc-signer-state-anchor-acknowledgement/v1\0");
        acknowledgement_digest.update(acknowledgement.signing_digest);
        acknowledgement_digest.update(acknowledgement.signature);
        acknowledgement_digest.update(configured_spki_hash);
        acknowledgement.acknowledgement_digest = acknowledgement_digest.finalize().into();

        let first = store
            .acknowledge_state_witness_checkpoint(
                acknowledgement.clone(),
                2,
                false,
                acknowledgement.expires_at_unix_ms,
            )
            .expect("anchor and rotate genesis");
        assert!(first.rotated);
        assert_eq!(store.witness_record_count().expect("empty segment"), 0);

        let aborted = store
            .next_state_witness(state_image_digest(Some(b"aborted state")))
            .expect("next witness");
        store.prepare_witness(aborted).expect("prepare witness");
        store.abort_pending_witness().expect("abort witness");
        assert_eq!(store.state_witness_tip().expect("unchanged tip"), tip);
        assert_eq!(store.witness_record_count().expect("prepare plus abort"), 2);

        // Reproduce a crash after the exact replay's pending-anchor fsync but
        // before `.next` creation. The current signed segment has the same base
        // and tip as the replay, but is not a completed publication because it
        // still contains PREPARE+ABORT. A tip read must finish compaction rather
        // than falsely promoting the pending metadata and leaving writes
        // blocked forever.
        let witness_base = store
            .anchor_metadata
            .as_ref()
            .and_then(|metadata| metadata.witness_base.clone());
        store
            .persist_state_anchor_metadata(StateAnchorMetadata {
                latest: acknowledgement.clone(),
                witness_base,
                pending_witness_base: Some(acknowledgement.clone()),
            })
            .expect("persist replay rotation intent");
        let snapshot = store
            .state_witness_tip_snapshot()
            .expect("tip settles pending compaction");
        assert_eq!(snapshot.tip, tip);
        assert_eq!(snapshot.base, tip);
        let settled_anchor = snapshot.anchor.expect("settled anchor");
        assert_eq!(settled_anchor.witness_base, Some(acknowledgement));
        assert!(settled_anchor.pending_witness_base.is_none());
        assert_eq!(store.witness_record_count().expect("compacted segment"), 0);
        store
            .replace_state(b"write after compaction")
            .expect("writes resume after compaction");
        drop(store);

        let witness_path = state_witness_file_path(&state_path);
        let mut witness_next = witness_path.as_os_str().to_os_string();
        witness_next.push(TBTC_SIGNER_STATE_WITNESS_NEXT_SUFFIX);
        let mut witness_previous = witness_path.as_os_str().to_os_string();
        witness_previous.push(TBTC_SIGNER_STATE_WITNESS_PREVIOUS_SUFFIX);
        for path in [
            state_path.clone(),
            state_lock_file_path(&state_path),
            durable_store_id_file_path(&state_path),
            witness_path,
            PathBuf::from(witness_next),
            PathBuf::from(witness_previous),
            state_anchor_file_path(&state_path),
        ] {
            let _ = fs::remove_file(path);
        }
    }

    #[test]
    #[cfg(unix)]
    fn exact_anchor_replay_rotates_unchanged_tip_at_record_threshold() {
        let _guard = lock_test_state();
        let mut random = [0u8; 12];
        OsRng.fill_bytes(&mut random);
        let state_path = std::env::temp_dir().join(format!(
            "tbtc-signer-anchor-exact-replay-{}-{}",
            std::process::id(),
            hex::encode(random)
        ));
        let signing_key = SigningKey::from_bytes(&[0x07; 32]);
        let configured_spki_hash =
            configure_anchor_store_fixture(&state_path, &signing_key, [0x44; 32]);
        let mut store = StateFileLock::acquire(&state_path).expect("open anchored store");
        let tip = store.state_witness_tip().expect("genesis tip");
        let acknowledgement = signed_acknowledgement_for_tip(
            &signing_key,
            configured_spki_hash,
            store.identity.fingerprint,
            &tip,
        );
        assert!(
            store
                .acknowledge_state_witness_checkpoint(
                    acknowledgement.clone(),
                    2,
                    false,
                    acknowledgement.expires_at_unix_ms,
                )
                .expect("initial anchor rotation")
                .rotated
        );

        let aborted = store
            .next_state_witness(state_image_digest(Some(b"aborted state")))
            .expect("next witness");
        store.prepare_witness(aborted).expect("prepare witness");
        store.abort_pending_witness().expect("abort witness");
        assert_eq!(store.state_witness_tip().expect("unchanged tip"), tip);
        assert_eq!(store.witness_record_count().expect("threshold count"), 2);

        let replay = store
            .acknowledge_state_witness_checkpoint(
                acknowledgement.clone(),
                2,
                false,
                acknowledgement.expires_at_unix_ms,
            )
            .expect("exact replay compacts unchanged tip");
        assert!(replay.idempotent);
        assert!(replay.rotated);
        assert_eq!(store.witness_record_count().expect("compacted count"), 0);
        store
            .replace_state(b"write after exact-replay compaction")
            .expect("writes resume");
        drop(store);
        cleanup_anchor_store_fixture(&state_path);
    }

    #[test]
    #[cfg(unix)]
    fn startup_settles_pending_rotation_at_every_rename_boundary_before_tip_read() {
        let _guard = lock_test_state();
        let signing_key = SigningKey::from_bytes(&[0x07; 32]);
        for case in 0..=4 {
            let mut random = [0u8; 12];
            OsRng.fill_bytes(&mut random);
            let state_path = std::env::temp_dir().join(format!(
                "tbtc-signer-anchor-startup-{case}-{}-{}",
                std::process::id(),
                hex::encode(random)
            ));
            let configured_spki_hash =
                configure_anchor_store_fixture(&state_path, &signing_key, [0x44; 32]);
            let mut store = StateFileLock::acquire(&state_path).expect("open fixture store");
            let tip = store.state_witness_tip().expect("fixture genesis tip");
            let acknowledgement = signed_acknowledgement_for_tip(
                &signing_key,
                configured_spki_hash,
                store.identity.fingerprint,
                &tip,
            );
            store
                .persist_state_anchor_metadata(StateAnchorMetadata {
                    latest: acknowledgement.clone(),
                    witness_base: None,
                    pending_witness_base: Some(acknowledgement.clone()),
                })
                .expect("persist pending rotation");

            if case >= 1 {
                let header = encode_state_witness_segment_header(
                    &store.identity.fingerprint,
                    &acknowledgement,
                )
                .expect("encode next segment");
                create_entry_atomically(
                    &store.directory,
                    &store.witness_next_name,
                    &header,
                    "startup fixture next witness",
                )
                .expect("publish next fixture");
            }
            if case >= 2 {
                renameat_same_directory(
                    store.directory.as_raw_fd(),
                    &store.witness_name,
                    &store.witness_previous_name,
                    "startup fixture retain previous",
                )
                .expect("retain previous fixture");
            }
            if case >= 3 {
                renameat_same_directory(
                    store.directory.as_raw_fd(),
                    &store.witness_next_name,
                    &store.witness_name,
                    "startup fixture publish current",
                )
                .expect("publish current fixture");
            }
            if case >= 4 {
                unlinkat_entry(store.directory.as_raw_fd(), &store.witness_previous_name)
                    .expect("retire previous fixture");
            }
            store.directory.sync_all().expect("sync crash fixture");
            drop(store);

            let mut reopened =
                StateFileLock::acquire(&state_path).expect("startup completes rotation");
            let metadata = reopened
                .anchor_metadata
                .as_ref()
                .expect("normalized anchor metadata");
            assert_eq!(metadata.latest, acknowledgement);
            assert_eq!(metadata.witness_base, Some(acknowledgement.clone()));
            assert!(metadata.pending_witness_base.is_none());
            ensure_entry_absent(
                reopened.directory.as_raw_fd(),
                &reopened.witness_next_name,
                "settled next witness",
            )
            .expect("next absent after startup");
            ensure_entry_absent(
                reopened.directory.as_raw_fd(),
                &reopened.witness_previous_name,
                "settled previous witness",
            )
            .expect("previous absent after startup");

            let anchor_before =
                fs::read(state_anchor_file_path(&state_path)).expect("anchor before tip");
            let witness_before =
                fs::read(state_witness_file_path(&state_path)).expect("witness before tip");
            let snapshot = reopened
                .state_witness_tip_snapshot()
                .expect("tip after startup settlement");
            assert_eq!(snapshot.tip, tip);
            assert_eq!(snapshot.base, tip);
            assert_eq!(
                fs::read(state_anchor_file_path(&state_path)).expect("anchor after tip"),
                anchor_before,
                "case {case}: tip read must not lazily normalize anchor metadata"
            );
            assert_eq!(
                fs::read(state_witness_file_path(&state_path)).expect("witness after tip"),
                witness_before,
                "case {case}: tip read must not lazily rotate the witness"
            );
            drop(reopened);
            cleanup_anchor_store_fixture(&state_path);
        }
    }

    #[test]
    fn proof_lookup_reports_structured_history_pruned_below_rotated_base() {
        let base = StateWitness {
            generation: 42,
            previous_commitment: [0x21; 32],
            state_image_digest: [0x22; 32],
            commitment: [0x23; 32],
        };
        let error = resolve_witness_history_index(&[base], 41, [0x24; 32], "ancestor")
            .expect_err("generation before base must be pruned");
        assert_eq!(error.code(), "history_pruned");
        assert!(matches!(
            error,
            EngineError::HistoryPruned {
                requested_generation: 41,
                witness_base_generation: 42,
            }
        ));
    }

    #[cfg(unix)]
    fn crash_recovery_fixture(
        case: u8,
    ) -> (
        PathBuf,
        fs::File,
        OsString,
        OsString,
        OsString,
        DurableStoreIdentity,
        StateAnchorMetadata,
    ) {
        let mut random = [0u8; 12];
        OsRng.fill_bytes(&mut random);
        let path = std::env::temp_dir().join(format!(
            "tbtc-signer-witness-rotation-test-{}-{}",
            std::process::id(),
            hex::encode(random)
        ));
        fs::create_dir(&path).expect("create rotation fixture directory");
        let canonical = fs::canonicalize(&path).expect("canonical fixture directory");
        let directory =
            open_absolute_directory_nofollow(&canonical).expect("open fixture directory");
        let store_id = [0x19; 32];
        let fingerprint = durable_store_fingerprint(&store_id);
        let identity = DurableStoreIdentity {
            store_id,
            canonical_path_fingerprint: [0u8; 32],
            filesystem_fingerprint: [0u8; 32],
            lock_fingerprint: [0u8; 32],
            fingerprint,
        };
        let digest = state_image_digest(None);
        let previous = state_witness_genesis(&fingerprint);
        let base = StateWitness {
            generation: 1,
            previous_commitment: previous,
            state_image_digest: digest,
            commitment: state_commitment(&fingerprint, 1, &previous, &digest),
        };
        let mut acknowledgement = fixture_acknowledgement();
        acknowledgement.checkpoint_store_fingerprint = fingerprint;
        acknowledgement.checkpoint_generation = base.generation;
        acknowledgement.checkpoint_previous_commitment = base.previous_commitment;
        acknowledgement.checkpoint_state_image_digest = base.state_image_digest;
        acknowledgement.checkpoint_state_commitment = base.commitment;
        let metadata = StateAnchorMetadata {
            latest: acknowledgement.clone(),
            witness_base: None,
            pending_witness_base: Some(acknowledgement.clone()),
        };
        let current = OsString::from("state.state-witness");
        let next = OsString::from("state.state-witness.next");
        let previous_name = OsString::from("state.state-witness.previous");
        let mut legacy = Vec::new();
        legacy.extend_from_slice(TBTC_SIGNER_STATE_WITNESS_MAGIC);
        legacy.extend_from_slice(&store_id);
        legacy.extend_from_slice(&encode_state_witness_record(
            TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE,
            &base,
        ));
        legacy.extend_from_slice(&encode_state_witness_record(
            TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT,
            &base,
        ));
        create_entry_atomically(&directory, &current, &legacy, "fixture current witness")
            .expect("publish legacy current");
        if case >= 1 {
            let header = encode_state_witness_segment_header(&fingerprint, &acknowledgement)
                .expect("encode next fixture");
            create_entry_atomically(&directory, &next, &header, "fixture next witness")
                .expect("publish next fixture");
        }
        if case >= 2 {
            renameat_same_directory(
                directory.as_raw_fd(),
                &current,
                &previous_name,
                "fixture current to previous",
            )
            .expect("retain fixture previous");
        }
        if case >= 3 {
            renameat_same_directory(
                directory.as_raw_fd(),
                &next,
                &current,
                "fixture next to current",
            )
            .expect("publish fixture current");
        }
        if case >= 4 {
            unlinkat_entry(directory.as_raw_fd(), &previous_name).expect("retire fixture previous");
        }
        directory.sync_all().expect("sync fixture state");
        (
            path,
            directory,
            current,
            next,
            previous_name,
            identity,
            metadata,
        )
    }

    #[cfg(unix)]
    #[test]
    fn rotation_recovery_completes_every_durable_rename_boundary() {
        // 0: signed base durable, before .next; 1: .next durable; 2: current
        // renamed to .previous; 3: .next renamed to current; 4: .previous
        // retired before anchor normalization. Each state also represents the
        // corresponding pre/post directory-fsync crash image.
        for case in 0..=4 {
            let (path, directory, current, next, previous, identity, metadata) =
                crash_recovery_fixture(case);
            assert!(
                recover_state_witness_rotation(
                    &directory,
                    StateWitnessRotationNames {
                        current: &current,
                        next: &next,
                        previous: &previous,
                    },
                    &identity,
                    None,
                    Some(&metadata),
                    16,
                    true,
                    None,
                )
                .expect("recover signed rotation"),
                "case {case} must promote its pending base"
            );
            ensure_entry_absent(directory.as_raw_fd(), &next, "fixture next")
                .expect("next retired");
            ensure_entry_absent(directory.as_raw_fd(), &previous, "fixture previous")
                .expect("previous retired");
            let parsed =
                validate_rotation_candidate(&directory, &current, &identity, Some(&metadata), 16)
                    .expect("final current segment");
            assert!(acknowledgement_matches_witness(
                metadata
                    .pending_witness_base
                    .as_ref()
                    .expect("pending fixture base"),
                parsed.history.first()
            ));
            drop(directory);
            fs::remove_dir_all(path).expect("remove rotation fixture");
        }
    }
}
