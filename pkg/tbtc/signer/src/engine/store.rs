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
#[derive(Clone, Copy)]
struct StateWitnessRotationNames<'a> {
    current: &'a OsStr,
    next: &'a OsStr,
    previous: &'a OsStr,
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
    current_state_file: Option<fs::File>,
    current_state_identity: Option<OpenedObjectIdentity>,
    identity: DurableStoreIdentity,
    lock_held: bool,
}

impl StateFileLock {
    #[cfg(unix)]
    pub(crate) fn acquire(state_path: &Path) -> Result<Self, EngineError> {
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

        let configured_parent = state_path
            .parent()
            .filter(|parent| !parent.as_os_str().is_empty())
            .unwrap_or_else(|| Path::new("."));
        fs::create_dir_all(configured_parent).map_err(|error| {
            EngineError::Internal(format!(
                "failed to create signer state directory [{}]: {error}",
                configured_parent.display()
            ))
        })?;
        let canonical_parent = fs::canonicalize(configured_parent).map_err(|error| {
            EngineError::Internal(format!(
                "failed to canonicalize signer state directory [{}]: {error}",
                configured_parent.display()
            ))
        })?;
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

        let mut lock_file = openat_regular(
            directory.as_raw_fd(),
            &lock_name,
            libc::O_RDWR | libc::O_CREAT,
            0o600,
            "signer state lock file",
        )?;
        validate_owned_unlinked_regular(&lock_file, "signer state lock file")?;
        set_owner_only_permissions(&lock_file, "signer state lock file")?;
        validate_secure_regular_file(&lock_file, "signer state lock file")?;
        acquire_exclusive_lock(&lock_file, &lock_path)?;
        let lock_identity = descriptor_identity(&lock_file, "signer state lock file")?;

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

        let store_id_name = durable_store_id_file_name(&state_name);
        validate_entry_name(&store_id_name, "store ID")?;
        let (store_id_file, store_id, store_id_identity) =
            open_or_create_store_id(&directory, &store_id_name)?;

        // Persist the creation of both stable anchor entries before claiming
        // durability to the host.
        directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory [{}]: {error}",
                canonical_parent.display()
            ))
        })?;

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

        let anchor_configuration = configured_state_anchor()?;
        let anchor_name = state_anchor_file_name(&state_name);
        validate_entry_name(&anchor_name, "state anchor")?;
        let (mut anchor_file, mut anchor_identity, mut anchor_metadata, mut anchor_bytes) =
            open_state_anchor(
                &directory,
                &anchor_name,
                &identity.fingerprint,
                anchor_configuration.as_ref(),
            )?;

        let witness_name = state_witness_file_name(&state_name);
        validate_entry_name(&witness_name, "state witness")?;
        let mut witness_next_name = witness_name.clone();
        witness_next_name.push(TBTC_SIGNER_STATE_WITNESS_NEXT_SUFFIX);
        validate_entry_name(&witness_next_name, "next state witness")?;
        let mut witness_previous_name = witness_name.clone();
        witness_previous_name.push(TBTC_SIGNER_STATE_WITNESS_PREVIOUS_SUFFIX);
        validate_entry_name(&witness_previous_name, "previous state witness")?;
        let witness_max_records = state_witness_max_records()?;
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
            parse_state_anchor_metadata(&bytes, &identity.fingerprint, configuration)?;
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
            current_state_file,
            current_state_identity,
            identity,
            lock_held: true,
        };
        store.reconcile_pending_witness()?;
        // Startup loading must be able to inspect a malformed or stale state
        // image and apply the explicit corruption policy. Validate every held
        // descriptor and the journal itself here; the state-image commitment
        // is checked before a decoded state is admitted.
        store.revalidate_store_entries()?;
        Ok(store)
    }

    #[cfg(not(unix))]
    pub(crate) fn acquire(state_path: &Path) -> Result<Self, EngineError> {
        Err(EngineError::Internal(format!(
            "descriptor-bound durable signer storage is unavailable on this platform for [{}]",
            state_path.display()
        )))
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
    /// This narrow path lets the loader distinguish malformed state from a
    /// valid-but-rolled-back image and honor the explicit corruption policy.
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
                let configuration = configured_state_anchor()?.ok_or_else(|| {
                    EngineError::Internal(
                        "persisted signer state anchor metadata requires manifest pins".to_string(),
                    )
                })?;
                let parsed = parse_state_anchor_metadata(
                    &live_bytes,
                    &self.identity.fingerprint,
                    &configuration,
                )?;
                if &parsed != metadata {
                    return Err(EngineError::Internal(
                        "signer state anchor metadata no longer matches its verified model"
                            .to_string(),
                    ));
                }
            }
            (None, None, None, None) => ensure_entry_absent(
                self.directory.as_raw_fd(),
                &self.anchor_name,
                "signer state anchor metadata",
            )?,
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
        self.verify_state_witness_journal()
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
        #[cfg(test)]
        WITNESS_FULL_VERIFICATIONS.fetch_add(1, std::sync::atomic::Ordering::SeqCst);

        let before = witness_change_stamp(&self.witness_file)?;
        let parsed = read_state_witness_journal_streaming(
            &self.witness_file,
            &self.identity.store_id,
            &self.identity.fingerprint,
            self.witness_max_records,
            self.anchor_metadata.as_ref(),
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
        let Some(previous) = self.witness_prefix.clone() else {
            // Nothing verified yet; the next access parses the whole journal.
            return Ok(());
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
        if before.size != self.witness_length as u64 {
            self.witness_prefix = None;
            return Err(EngineError::Internal(
                "signer state witness journal size changed during append read-back".to_string(),
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
        let configuration = configured_state_anchor()?.ok_or_else(|| {
            EngineError::Internal(
                "cannot persist signer state anchor metadata without manifest pins".to_string(),
            )
        })?;
        let bytes = encode_state_anchor_metadata(&self.identity.fingerprint, &metadata);
        let parsed =
            parse_state_anchor_metadata(&bytes, &self.identity.fingerprint, &configuration)?;
        if parsed != metadata {
            return Err(EngineError::Internal(
                "new signer state anchor metadata failed canonical round-trip".to_string(),
            ));
        }
        let (file, identity) =
            replace_state_anchor_entry(&self.directory, &self.anchor_name, &bytes)?;
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
        self.verify_state_witness_journal_fully()?;
        self.normalize_published_pending_anchor()?;
        self.revalidate_store_entries()
    }

    #[cfg(unix)]
    fn rotate_state_witness_segment(
        &mut self,
        acknowledgement: &StateAnchorAcknowledgement,
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
        let (next_file, next_identity) = create_entry_atomically(
            &self.directory,
            &self.witness_next_name,
            &bytes,
            "next signer state witness journal",
        )?;
        let parsed = read_state_witness_journal_streaming(
            &next_file,
            &self.identity.store_id,
            &self.identity.fingerprint,
            self.witness_max_records,
            self.anchor_metadata.as_ref(),
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
        let current_digest = current_state_image_digest(self.current_state_file.as_ref())?;
        if tip.state_image_digest != current_digest {
            let _ = unlinkat_entry(self.directory.as_raw_fd(), &self.witness_next_name);
            return Err(EngineError::Internal(
                "new state witness segment base does not commit the current state image"
                    .to_string(),
            ));
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

        // The new name and complete signed header are durable and verified.
        // Only now may the previous segment be retired.
        self.verify_state_witness_journal_fully()?;
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
        unlinkat_entry(self.directory.as_raw_fd(), &self.witness_previous_name)?;
        self.directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory after retiring previous witness \
                 segment: {error}"
            ))
        })?;
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
        self.witness_length += record.len();
        self.last_appended_record.copy_from_slice(&record);
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
        renameat_same_directory(directory.as_raw_fd(), &temp_name, name, label)?;
        directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory after publishing {label}: {error}"
            ))
        })?;
        let identity = descriptor_identity(&temp_file, label)?;
        validate_live_entry(directory, name, identity, label)?;
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
    let metadata = parse_state_anchor_metadata(&bytes, store_fingerprint, configuration)?;
    let identity = descriptor_identity(&file, LABEL)?;
    Ok((Some(file), Some(identity), Some(metadata), Some(bytes)))
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
    if let Some(base) = witness_base.as_ref() {
        validate_persisted_state_anchor_acknowledgement(base, configuration)?;
        validate_anchor_acknowledgement_shape(base, expected_store_fingerprint)?;
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
        || (acknowledgement.revision == 1 && acknowledgement.previous_event_root != [0u8; 32])
        || (acknowledgement.revision > 1 && acknowledgement.previous_event_root == [0u8; 32])
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
    const LABEL: &str = "signer state anchor metadata";
    let temp_name = unique_temp_name(name)?;
    let temp_file = openat_regular(
        directory.as_raw_fd(),
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
        renameat_same_directory(
            directory.as_raw_fd(),
            &temp_name,
            name,
            "publish signer state anchor metadata",
        )?;
        directory.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer state directory after publishing anchor metadata: {error}"
            ))
        })?;
        let identity = descriptor_identity(&temp_file, LABEL)?;
        validate_live_entry(directory, name, identity, LABEL)?;
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
fn recover_state_witness_rotation(
    directory: &fs::File,
    names: StateWitnessRotationNames<'_>,
    store_identity: &DurableStoreIdentity,
    current_state_file: Option<&fs::File>,
    anchor: Option<&StateAnchorMetadata>,
    maximum_records: usize,
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
        create_entry_atomically(
            directory,
            next_name,
            &bytes,
            "next signer state witness journal during recovery",
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
        previous_exists = true;
    } else if next_exists && !current_exists && previous_exists {
        validate_rotation_candidate(
            directory,
            next_name,
            store_identity,
            anchor,
            maximum_records,
        )?;
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
    if previous_exists {
        unlinkat_entry(directory.as_raw_fd(), previous_name)?;
    }
    directory.sync_all().map_err(|error| {
        EngineError::Internal(format!(
            "failed to sync state witness recovery after retiring previous segment: {error}"
        ))
    })?;
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
    let stat = descriptor_stat(file, "signer state witness journal")?;
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
        ] {
            let _ = fs::remove_file(path);
        }
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
