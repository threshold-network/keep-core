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
/// The retired v1 journal magic. It is never written and never repaired; it is
/// recognized only so a v1 store fails closed with an actionable migration
/// error instead of a generic "invalid commitment".
const TBTC_SIGNER_STATE_WITNESS_MAGIC_V1: &[u8; 16] = b"TBTCWITNESSv1\0\0\0";
pub(crate) const TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH: usize = 48;
/// The journal is a fixed-width header followed by fixed-width records; the
/// tests build on-disk fixtures from this geometry, so it is part of the
/// crate-visible store contract.
pub(crate) const TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH: usize = 105;
const TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE: u8 = 1;
const TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT: u8 = 2;
const TBTC_SIGNER_STATE_WITNESS_RECORD_ABORT: u8 = 3;

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
    witness_name: OsString,
    witness_file: fs::File,
    witness_identity: OpenedObjectIdentity,
    witness_history: Vec<StateWitness>,
    pending_witness: Option<StateWitness>,
    witness_length: usize,
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

        let witness_name = state_witness_file_name(&state_name);
        validate_entry_name(&witness_name, "state witness")?;
        let (witness_file, witness_identity, witness_history, pending_witness, witness_length) =
            open_or_create_state_witness(
                &directory,
                &witness_name,
                &identity,
                current_state_file.as_ref(),
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
            witness_name,
            witness_file,
            witness_identity,
            witness_history,
            pending_witness,
            witness_length,
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
        let backup_parent = backup_path.parent().unwrap_or_else(|| Path::new("."));
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
    pub(crate) fn revalidate_store_entries(&self) -> Result<(), EngineError> {
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
        let witness_bytes = read_file_at(&self.witness_file, "signer state witness journal")?;
        if witness_bytes.len() != self.witness_length {
            return Err(EngineError::Internal(
                "signer state witness journal length changed outside the locked store".to_string(),
            ));
        }
        let (history, pending) = parse_state_witness_journal(
            &witness_bytes,
            &self.identity.store_id,
            &self.identity.fingerprint,
        )?;
        if pending.is_some() || history != self.witness_history {
            return Err(EngineError::Internal(
                "signer state witness journal changed outside the locked store".to_string(),
            ));
        }
        Ok(())
    }

    #[cfg(unix)]
    pub(crate) fn validate_state_image(&self) -> Result<(), EngineError> {
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
    fn revalidate(&self) -> Result<(), EngineError> {
        self.validate_state_image()
    }

    #[cfg(not(unix))]
    pub(crate) fn revalidate_store_entries(&self) -> Result<(), EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    #[cfg(not(unix))]
    pub(crate) fn validate_state_image(&self) -> Result<(), EngineError> {
        Err(EngineError::Internal(
            "descriptor-bound durable signer storage is unavailable on this platform".to_string(),
        ))
    }

    #[cfg(not(unix))]
    fn revalidate(&self) -> Result<(), EngineError> {
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
        let ancestor_index = self
            .witness_history
            .iter()
            .position(|entry| {
                entry.generation == ancestor_generation && entry.commitment == ancestor_commitment
            })
            .ok_or_else(|| {
                EngineError::Validation(
                    "state witness proof ancestor is not in the active store history".to_string(),
                )
            })?;
        let target_index = self
            .witness_history
            .iter()
            .position(|entry| {
                entry.generation == target_generation && entry.commitment == target_commitment
            })
            .ok_or_else(|| {
                EngineError::Validation(
                    "state witness proof target is not in the active store history".to_string(),
                )
            })?;
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
        self.append_witness_record(TBTC_SIGNER_STATE_WITNESS_RECORD_PREPARE, &witness)?;
        self.pending_witness = Some(witness);
        Ok(())
    }

    #[cfg(unix)]
    fn commit_pending_witness(&mut self) -> Result<(), EngineError> {
        let pending = self.pending_witness.clone().ok_or_else(|| {
            EngineError::Internal("no prepared state witness to commit".to_string())
        })?;
        self.append_witness_record(TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT, &pending)?;
        self.witness_history.push(pending);
        self.pending_witness = None;
        Ok(())
    }

    #[cfg(unix)]
    fn abort_pending_witness(&mut self) -> Result<(), EngineError> {
        let pending = self.pending_witness.clone().ok_or_else(|| {
            EngineError::Internal("no prepared state witness to abort".to_string())
        })?;
        self.append_witness_record(TBTC_SIGNER_STATE_WITNESS_RECORD_ABORT, &pending)?;
        self.pending_witness = None;
        Ok(())
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
        Ok(())
    }

    #[cfg(all(test, unix))]
    pub(crate) fn release_lock_for_tests(&mut self) -> Result<(), EngineError> {
        let result = unsafe { libc::flock(self._file.as_raw_fd(), libc::LOCK_UN) };
        if result != 0 {
            return Err(EngineError::Internal(format!(
                "failed to release signer durable store lock for test: {}",
                std::io::Error::last_os_error()
            )));
        }
        self.lock_held = false;
        Ok(())
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

fn state_commitment(
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

#[cfg(unix)]
fn open_or_create_store_id(
    directory: &fs::File,
    name: &OsStr,
) -> Result<(fs::File, [u8; 32], OpenedObjectIdentity), EngineError> {
    let existing = openat_optional(
        directory.as_raw_fd(),
        name,
        libc::O_RDWR,
        "signer durable store ID file",
    )?;
    let (file, created) = match existing {
        Some(file) => (file, false),
        None => (
            openat_regular(
                directory.as_raw_fd(),
                name,
                libc::O_RDWR | libc::O_CREAT | libc::O_EXCL,
                0o600,
                "signer durable store ID file",
            )?,
            true,
        ),
    };
    validate_owned_unlinked_regular(&file, "signer durable store ID file")?;
    set_owner_only_permissions(&file, "signer durable store ID file")?;
    validate_secure_regular_file(&file, "signer durable store ID file")?;

    let store_id = if created {
        let mut id = [0u8; 32];
        loop {
            OsRng.fill_bytes(&mut id);
            if id != [0u8; 32] {
                break;
            }
        }
        write_file_at(&file, &id, "signer durable store ID file")?;
        file.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync signer durable store ID file: {error}"
            ))
        })?;
        id
    } else {
        read_store_id(&file)?
    };
    let identity = descriptor_identity(&file, "signer durable store ID file")?;
    Ok((file, store_id, identity))
}

/// The opened journal descriptor, its identity, the committed history, a
/// surviving PREPARE the caller must reconcile, and the on-disk journal length.
#[cfg(unix)]
type OpenedStateWitnessJournal = (
    fs::File,
    OpenedObjectIdentity,
    Vec<StateWitness>,
    Option<StateWitness>,
    usize,
);

#[cfg(unix)]
fn open_or_create_state_witness(
    directory: &fs::File,
    name: &OsStr,
    store_identity: &DurableStoreIdentity,
    current_state_file: Option<&fs::File>,
) -> Result<OpenedStateWitnessJournal, EngineError> {
    let existing = openat_optional(
        directory.as_raw_fd(),
        name,
        libc::O_RDWR,
        "signer state witness journal",
    )?;
    let (file, created) = match existing {
        Some(file) => (file, false),
        None => (
            openat_regular(
                directory.as_raw_fd(),
                name,
                libc::O_RDWR | libc::O_CREAT | libc::O_EXCL,
                0o600,
                "signer state witness journal",
            )?,
            true,
        ),
    };
    validate_owned_unlinked_regular(&file, "signer state witness journal")?;
    set_owner_only_permissions(&file, "signer state witness journal")?;
    validate_secure_regular_file(&file, "signer state witness journal")?;

    let (history, pending, length) = if created {
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
        write_file_at(&file, &bytes, "signer state witness journal")?;
        file.sync_all().map_err(|error| {
            EngineError::Internal(format!(
                "failed to sync initialized signer state witness journal: {error}"
            ))
        })?;
        (vec![genesis], None, bytes.len())
    } else {
        // The journal is a fixed header followed by fixed-width records, each
        // appended and fsynced individually. A crash can therefore leave at
        // most one torn trailing record - bytes past the last complete record
        // boundary - and those bytes are the only thing that may be discarded,
        // after the truncation itself is made durable. Every record that
        // survives is then parsed and validated in full, so a COMPLETE record
        // with invalid content still fails closed instead of being truncated
        // away. Any PREPARE that survives the repair is returned to the caller,
        // which reconciles it against the held state image before use.
        let bytes = read_file_at(&file, "signer state witness journal")?;
        let bytes = truncate_incomplete_witness_record(&file, &store_identity.store_id, bytes)?;
        let (history, pending) = parse_state_witness_journal(
            &bytes,
            &store_identity.store_id,
            &store_identity.fingerprint,
        )?;
        (history, pending, bytes.len())
    };
    let identity = descriptor_identity(&file, "signer state witness journal")?;
    Ok((file, identity, history, pending, length))
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
    mut bytes: Vec<u8>,
) -> Result<Vec<u8>, EngineError> {
    if bytes.len() < TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH
        || &bytes[..TBTC_SIGNER_STATE_WITNESS_MAGIC.len()] != TBTC_SIGNER_STATE_WITNESS_MAGIC
        || &bytes[TBTC_SIGNER_STATE_WITNESS_MAGIC.len()..TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH]
            != expected_store_id
    {
        return Ok(bytes);
    }
    let incomplete_len = bytes[TBTC_SIGNER_STATE_WITNESS_HEADER_LENGTH..]
        .chunks_exact(TBTC_SIGNER_STATE_WITNESS_RECORD_LENGTH)
        .remainder()
        .len();
    if incomplete_len == 0 {
        return Ok(bytes);
    }

    let complete_len = bytes.len() - incomplete_len;
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
    bytes.truncate(complete_len);
    Ok(bytes)
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
                        "signer state witness PREPARE does not extend the committed tip"
                            .to_string(),
                    ));
                }
                pending = Some(witness);
            }
            TBTC_SIGNER_STATE_WITNESS_RECORD_COMMIT => {
                if pending.as_ref() != Some(&witness) {
                    return Err(EngineError::Internal(
                        "signer state witness COMMIT does not match its PREPARE".to_string(),
                    ));
                }
                history.push(witness);
                pending = None;
            }
            TBTC_SIGNER_STATE_WITNESS_RECORD_ABORT => {
                if pending.as_ref() != Some(&witness) {
                    return Err(EngineError::Internal(
                        "signer state witness ABORT does not match its PREPARE".to_string(),
                    ));
                }
                pending = None;
            }
            _ => {
                return Err(EngineError::Internal(format!(
                    "signer state witness journal contains unknown record type [{record_type}]"
                )))
            }
        }
    }
    if history.is_empty() {
        return Err(truncated_state_witness_journal_error(
            "signer state witness journal has no committed genesis record".to_string(),
        ));
    }
    Ok((history, pending))
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
            "signer durable store ID file has invalid length [{}], expected 32",
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
}
