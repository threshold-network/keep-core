//! Manifest-pinned state-anchor acknowledgement verification.
//!
//! The online anchor acknowledgement is deliberately persisted outside
//! `EngineState`: accepting it must not advance the state witness whose exact
//! tip the service just acknowledged. The durable store owns the descriptor-
//! bound metadata and segment rotation; this module owns strict wire parsing,
//! pin validation, the frozen signing transcript, and Ed25519 verification.

use super::*;

use crate::api::{
    AcknowledgeStateWitnessCheckpointRequest, AcknowledgeStateWitnessCheckpointResult,
    RecoverStateWitnessCheckpointRequest, RecoverStateWitnessCheckpointResult,
    StateWitnessTipResult,
};
use ed25519_dalek::{Signature, VerifyingKey};

pub(crate) const TBTC_SIGNER_STATE_WITNESS_TIP_SCHEMA: &str = "tbtc-signer-state-witness-tip/v1";
pub(crate) const TBTC_SIGNER_STATE_WITNESS_CHECKPOINT_ACK_SCHEMA: &str =
    "tbtc-signer-state-witness-checkpoint-ack/v1";
pub(crate) const TBTC_SIGNER_STATE_WITNESS_CHECKPOINT_ACK_RESULT_SCHEMA: &str =
    "tbtc-signer-state-witness-checkpoint-ack-result/v1";
pub(crate) const TBTC_SIGNER_STATE_WITNESS_CHECKPOINT_RECOVERY_RESULT_SCHEMA: &str =
    "tbtc-signer-state-witness-checkpoint-recovery-result/v1";
const STATE_ANCHOR_READ_RESPONSE_SCHEMA: &str =
    "tbtc-frost-native-signer-state-anchor-read-response/v1";

const STATE_ANCHOR_SERVICE_RESPONSE_DOMAIN: &[u8] =
    b"tbtc-native-signer-state-anchor-service-response/v1\0";
const STATE_ANCHOR_ACKNOWLEDGEMENT_DOMAIN: &[u8] = b"tbtc-signer-state-anchor-acknowledgement/v1\0";
const STATE_ANCHOR_READ_RESPONSE_DOMAIN: &[u8] =
    b"tbtc-native-signer-state-anchor-read-response/v1\0";
const STATE_ANCHOR_EVENT_ROOT_DOMAIN: &[u8] = b"tbtc-native-signer-state-anchor-event/v1\0";
const ED25519_SPKI_PREFIX: &[u8] =
    &hex_literal_ed25519_spki_prefix::ED25519_SUBJECT_PUBLIC_KEY_INFO_PREFIX;
const ACKNOWLEDGEMENT_MAX_TTL_MILLISECONDS: u64 = 30_000;
const ACKNOWLEDGEMENT_MAX_FUTURE_SKEW_MILLISECONDS: u64 = 5_000;

// Kept in a private module so the byte literal is compile-time checked without
// adding a second hex-decoding dependency or a runtime parse.
mod hex_literal_ed25519_spki_prefix {
    pub(super) const ED25519_SUBJECT_PUBLIC_KEY_INFO_PREFIX: [u8; 12] = [
        0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x03, 0x21, 0x00,
    ];
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct StateAnchorConfiguration {
    pub(crate) binding_hash: [u8; 32],
    pub(crate) response_public_key: [u8; 32],
    pub(crate) response_public_key_spki_sha256: [u8; 32],
    pub(crate) rotation_threshold_records: usize,
    pub(crate) trust: Option<StateAnchorTrustConfiguration>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct StateAnchorTrustConfiguration {
    pub(crate) protocol_id: [u8; 32],
    pub(crate) stream_id: [u8; 32],
    pub(crate) activation_manifest_hash: [u8; 32],
    pub(crate) activation_manifest_sequence: u64,
    pub(crate) offline_authority_public_key: [u8; 32],
    pub(crate) offline_authority_public_key_spki_sha256: [u8; 32],
    pub(crate) certificate_sequence: u64,
    pub(crate) certificate_digest: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct StateAnchorAcknowledgement {
    pub(crate) binding_hash: [u8; 32],
    pub(crate) request_digest: [u8; 32],
    pub(crate) nonce: [u8; 32],
    pub(crate) status: u8,
    pub(crate) service_epoch: u64,
    pub(crate) revision: u64,
    pub(crate) previous_event_root: [u8; 32],
    pub(crate) event_root: [u8; 32],
    pub(crate) checkpoint_store_fingerprint: [u8; 32],
    pub(crate) checkpoint_generation: u64,
    pub(crate) checkpoint_previous_commitment: [u8; 32],
    pub(crate) checkpoint_state_image_digest: [u8; 32],
    pub(crate) checkpoint_state_commitment: [u8; 32],
    pub(crate) operation_id: [u8; 32],
    pub(crate) transition_digest: [u8; 32],
    pub(crate) committed_at_unix_ms: u64,
    pub(crate) expires_at_unix_ms: u64,
    pub(crate) signing_digest: [u8; 32],
    pub(crate) signature: [u8; 64],
    pub(crate) configured_spki_hash: [u8; 32],
    pub(crate) acknowledgement_digest: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct StateAnchorMetadata {
    pub(crate) latest: StateAnchorAcknowledgement,
    /// The acknowledgement that authorized the current witness-segment base.
    /// `None` means the original unpruned v2 genesis journal is still active.
    pub(crate) witness_base: Option<StateAnchorAcknowledgement>,
    /// A durably accepted acknowledgement authorizing an in-progress rotation.
    /// Retaining both old and next bases makes every rename boundary
    /// independently recoverable; it is promoted to `witness_base` only after
    /// the new current segment is durable and verified.
    pub(crate) pending_witness_base: Option<StateAnchorAcknowledgement>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct StateWitnessTipSnapshot {
    pub(crate) store_fingerprint: [u8; 32],
    pub(crate) tip: StateWitness,
    pub(crate) base: StateWitness,
    pub(crate) anchor: Option<StateAnchorMetadata>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct AnchorAcknowledgeOutcome {
    pub(crate) idempotent: bool,
    pub(crate) rotated: bool,
    pub(crate) snapshot: StateWitnessTipSnapshot,
}

pub(crate) fn validate_state_anchor_configuration() -> Result<(), EngineError> {
    configured_state_anchor().map(|_| ())
}

pub(crate) fn configured_state_anchor() -> Result<Option<StateAnchorConfiguration>, EngineError> {
    let binding_hash = signer_env_var(TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH_ENV);
    let response_public_key = signer_env_var(TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_ENV);
    let spki_hash = signer_env_var(TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256_ENV);
    let rotation_threshold =
        signer_env_var(TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS_ENV);
    let protocol_id = signer_env_var(TBTC_SIGNER_STATE_ANCHOR_PROTOCOL_ID_ENV);
    let stream_id = signer_env_var(TBTC_SIGNER_STATE_ANCHOR_STREAM_ID_ENV);
    let activation_manifest_hash =
        signer_env_var(TBTC_SIGNER_STATE_ANCHOR_ACTIVATION_MANIFEST_HASH_ENV);
    let activation_manifest_sequence =
        signer_env_var(TBTC_SIGNER_STATE_ANCHOR_ACTIVATION_MANIFEST_SEQUENCE_ENV);
    let offline_authority_public_key =
        signer_env_var(TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_ENV);
    let offline_authority_spki_hash =
        signer_env_var(TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_SPKI_SHA256_ENV);
    let certificate_sequence =
        signer_env_var(TBTC_SIGNER_STATE_ANCHOR_TRUST_CERTIFICATE_SEQUENCE_ENV);
    let certificate_digest = signer_env_var(TBTC_SIGNER_STATE_ANCHOR_TRUST_CERTIFICATE_DIGEST_ENV);

    if binding_hash.is_none()
        && response_public_key.is_none()
        && spki_hash.is_none()
        && rotation_threshold.is_none()
        && protocol_id.is_none()
        && stream_id.is_none()
        && activation_manifest_hash.is_none()
        && activation_manifest_sequence.is_none()
        && offline_authority_public_key.is_none()
        && offline_authority_spki_hash.is_none()
        && certificate_sequence.is_none()
        && certificate_digest.is_none()
    {
        return Ok(None);
    }
    let require = |value: Option<String>, name: &str| {
        value.ok_or_else(|| {
            EngineError::Validation(format!(
                "state-anchor configuration is partial; missing [{name}]"
            ))
        })
    };
    let binding_hash = parse_canonical_bytes32(
        &require(binding_hash, TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH_ENV)?,
        TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH_ENV,
    )?;
    if binding_hash == [0u8; 32] {
        return Err(EngineError::Validation(format!(
            "{} must be nonzero",
            TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH_ENV
        )));
    }
    let response_public_key = parse_canonical_bytes32(
        &require(
            response_public_key,
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_ENV,
        )?,
        TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_ENV,
    )?;
    let response_public_key_spki_sha256 = parse_canonical_bytes32(
        &require(
            spki_hash,
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256_ENV,
        )?,
        TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256_ENV,
    )?;
    validate_strong_ed25519_verifying_key(
        &response_public_key,
        TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_ENV,
    )?;
    let computed_spki_hash = ed25519_spki_sha256(&response_public_key);
    if computed_spki_hash != response_public_key_spki_sha256 {
        return Err(EngineError::Validation(format!(
            "{} does not match the configured raw Ed25519 public key",
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256_ENV
        )));
    }

    let threshold_raw = require(
        rotation_threshold,
        TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS_ENV,
    )?;
    let rotation_threshold_records = parse_canonical_usize(
        &threshold_raw,
        TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS_ENV,
    )?;
    let maximum_records = state_witness_max_records()?;
    let threshold_with_terminal = rotation_threshold_records
        .checked_add(TBTC_SIGNER_STATE_WITNESS_ROTATION_TERMINAL_RECORD_RESERVATION)
        .ok_or_else(|| {
            EngineError::Validation(
                "state witness rotation threshold overflows its terminal-record reservation"
                    .to_string(),
            )
        })?;
    if rotation_threshold_records < 2 || threshold_with_terminal > maximum_records {
        return Err(EngineError::Validation(format!(
            "{} must be at least 2 and leave four records below {} [{}]; got [{}]",
            TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS_ENV,
            TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS_ENV,
            maximum_records,
            rotation_threshold_records
        )));
    }

    let trust_values_are_absent = protocol_id.is_none()
        && stream_id.is_none()
        && activation_manifest_hash.is_none()
        && activation_manifest_sequence.is_none()
        && offline_authority_public_key.is_none()
        && offline_authority_spki_hash.is_none()
        && certificate_sequence.is_none()
        && certificate_digest.is_none();
    let trust = if trust_values_are_absent && !signer_profile_is_production() {
        // Development-only compatibility for the crate's pre-transition
        // low-level fixtures. Production and every FFI-installed anchor config
        // must pin the complete trust head and therefore cannot enter this path.
        None
    } else {
        let protocol_id =
            parse_required_nonzero_bytes32(protocol_id, TBTC_SIGNER_STATE_ANCHOR_PROTOCOL_ID_ENV)?;
        let stream_id =
            parse_required_nonzero_bytes32(stream_id, TBTC_SIGNER_STATE_ANCHOR_STREAM_ID_ENV)?;
        let activation_manifest_hash = parse_required_nonzero_bytes32(
            activation_manifest_hash,
            TBTC_SIGNER_STATE_ANCHOR_ACTIVATION_MANIFEST_HASH_ENV,
        )?;
        let activation_manifest_sequence = parse_required_nonzero_u64(
            activation_manifest_sequence,
            TBTC_SIGNER_STATE_ANCHOR_ACTIVATION_MANIFEST_SEQUENCE_ENV,
        )?;
        let offline_authority_public_key = parse_required_nonzero_bytes32(
            offline_authority_public_key,
            TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_ENV,
        )?;
        validate_strong_ed25519_verifying_key(
            &offline_authority_public_key,
            TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_ENV,
        )?;
        let offline_authority_public_key_spki_sha256 = parse_required_nonzero_bytes32(
            offline_authority_spki_hash,
            TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_SPKI_SHA256_ENV,
        )?;
        if ed25519_spki_sha256(&offline_authority_public_key)
            != offline_authority_public_key_spki_sha256
        {
            return Err(EngineError::Validation(format!(
                "{} does not match the configured raw Ed25519 public key",
                TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_SPKI_SHA256_ENV
            )));
        }
        if offline_authority_public_key == response_public_key {
            return Err(EngineError::Validation(
                "state-anchor offline authority and online response keys must be role-distinct"
                    .to_string(),
            ));
        }
        let certificate_sequence = parse_required_nonzero_u64(
            certificate_sequence,
            TBTC_SIGNER_STATE_ANCHOR_TRUST_CERTIFICATE_SEQUENCE_ENV,
        )?;
        let certificate_digest = parse_required_nonzero_bytes32(
            certificate_digest,
            TBTC_SIGNER_STATE_ANCHOR_TRUST_CERTIFICATE_DIGEST_ENV,
        )?;
        Some(StateAnchorTrustConfiguration {
            protocol_id,
            stream_id,
            activation_manifest_hash,
            activation_manifest_sequence,
            offline_authority_public_key,
            offline_authority_public_key_spki_sha256,
            certificate_sequence,
            certificate_digest,
        })
    };

    Ok(Some(StateAnchorConfiguration {
        binding_hash,
        response_public_key,
        response_public_key_spki_sha256,
        rotation_threshold_records,
        trust,
    }))
}

fn parse_required_nonzero_bytes32(
    value: Option<String>,
    name: &str,
) -> Result<[u8; 32], EngineError> {
    let value = value.ok_or_else(|| {
        EngineError::Validation(format!(
            "state-anchor trust configuration is partial; missing [{name}]"
        ))
    })?;
    let parsed = parse_canonical_bytes32(&value, name)?;
    if parsed == [0u8; 32] {
        return Err(EngineError::Validation(format!("{name} must be nonzero")));
    }
    Ok(parsed)
}

fn parse_required_nonzero_u64(value: Option<String>, name: &str) -> Result<u64, EngineError> {
    let value = value.ok_or_else(|| {
        EngineError::Validation(format!(
            "state-anchor trust configuration is partial; missing [{name}]"
        ))
    })?;
    let parsed = parse_canonical_u64(&value, name)?;
    if parsed == 0 {
        return Err(EngineError::Validation(format!("{name} must be nonzero")));
    }
    Ok(parsed)
}

pub(crate) fn state_witness_tip() -> Result<StateWitnessTipResult, EngineError> {
    // First load/migration can advance the witness. Take the same
    // ENGINE_STATE -> durable-store lock order as every mutation and keep the
    // engine guard through tip capture so a caller cannot observe a
    // pre-migration or concurrently superseded tip.
    let engine = state()?;
    let _guard = engine
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let snapshot = with_state_file_lock(|store| store.state_witness_tip_snapshot())?;
    Ok(state_witness_tip_result(&snapshot))
}

pub(crate) fn acknowledge_state_witness_checkpoint(
    request: AcknowledgeStateWitnessCheckpointRequest,
) -> Result<AcknowledgeStateWitnessCheckpointResult, EngineError> {
    let configuration = configured_state_anchor()?.ok_or_else(|| {
        EngineError::Validation(
            "state-anchor acknowledgement is disabled because its manifest pins are not configured"
                .to_string(),
        )
    })?;
    let acknowledgement = validate_acknowledgement_request(request, &configuration, true)?;
    let admission_expires_at_unix_ms = acknowledgement.expires_at_unix_ms;
    let outcome = with_state_file_lock_before_startup_rewrite(|store| {
        store.acknowledge_state_witness_checkpoint(
            acknowledgement,
            configuration.rotation_threshold_records,
            false,
            admission_expires_at_unix_ms,
        )
    })?;
    let snapshot = &outcome.snapshot;
    let anchor = snapshot.anchor.as_ref().ok_or_else(|| {
        EngineError::Internal(
            "durable store accepted an acknowledgement without retaining anchor metadata"
                .to_string(),
        )
    })?;
    Ok(AcknowledgeStateWitnessCheckpointResult {
        schema: TBTC_SIGNER_STATE_WITNESS_CHECKPOINT_ACK_RESULT_SCHEMA.to_string(),
        acknowledged: true,
        idempotent: outcome.idempotent,
        rotated: outcome.rotated,
        store_fingerprint: bytes32_hex(snapshot.store_fingerprint),
        generation: snapshot.tip.generation.to_string(),
        state_commitment: bytes32_hex(snapshot.tip.commitment),
        witness_base_generation: snapshot.base.generation.to_string(),
        witness_base_commitment: bytes32_hex(snapshot.base.commitment),
        anchor_service_epoch: anchor.latest.service_epoch.to_string(),
        anchor_service_revision: anchor.latest.revision.to_string(),
        anchor_event_root: bytes32_hex(anchor.latest.event_root),
        anchor_acknowledgement_digest: bytes32_hex(anchor.latest.acknowledgement_digest),
    })
}

pub(crate) fn recover_state_witness_checkpoint(
    request: RecoverStateWitnessCheckpointRequest,
) -> Result<RecoverStateWitnessCheckpointResult, EngineError> {
    let configuration = configured_state_anchor()?.ok_or_else(|| {
        EngineError::Validation(
            "state-anchor recovery is disabled because its manifest pins are not configured"
                .to_string(),
        )
    })?;
    let (acknowledgement, wrapper_expires_at_unix_ms) =
        validate_recovery_request(request, &configuration)?;
    let outcome = with_state_file_lock_before_startup_rewrite(|store| {
        store.acknowledge_state_witness_checkpoint(
            acknowledgement,
            configuration.rotation_threshold_records,
            true,
            wrapper_expires_at_unix_ms,
        )
    })?;
    let snapshot = &outcome.snapshot;
    let anchor = snapshot.anchor.as_ref().ok_or_else(|| {
        EngineError::Internal(
            "durable store recovered a checkpoint without retaining anchor metadata".to_string(),
        )
    })?;
    Ok(RecoverStateWitnessCheckpointResult {
        schema: TBTC_SIGNER_STATE_WITNESS_CHECKPOINT_RECOVERY_RESULT_SCHEMA.to_string(),
        recovered: true,
        idempotent: outcome.idempotent,
        rotated: outcome.rotated,
        store_fingerprint: bytes32_hex(snapshot.store_fingerprint),
        generation: snapshot.tip.generation.to_string(),
        state_commitment: bytes32_hex(snapshot.tip.commitment),
        witness_base_generation: snapshot.base.generation.to_string(),
        witness_base_commitment: bytes32_hex(snapshot.base.commitment),
        anchor_service_epoch: anchor.latest.service_epoch.to_string(),
        anchor_service_revision: anchor.latest.revision.to_string(),
        anchor_event_root: bytes32_hex(anchor.latest.event_root),
        anchor_acknowledgement_digest: bytes32_hex(anchor.latest.acknowledgement_digest),
    })
}

fn state_witness_tip_result(snapshot: &StateWitnessTipSnapshot) -> StateWitnessTipResult {
    let zero = [0u8; 32];
    let (binding_hash, epoch, revision, event_root, acknowledgement_digest) =
        match snapshot.anchor.as_ref() {
            Some(anchor) => (
                anchor.latest.binding_hash,
                anchor.latest.service_epoch,
                anchor.latest.revision,
                anchor.latest.event_root,
                anchor.latest.acknowledgement_digest,
            ),
            None => (zero, 0, 0, zero, zero),
        };
    StateWitnessTipResult {
        schema: TBTC_SIGNER_STATE_WITNESS_TIP_SCHEMA.to_string(),
        store_fingerprint: bytes32_hex(snapshot.store_fingerprint),
        generation: snapshot.tip.generation.to_string(),
        previous_state_commitment: bytes32_hex(snapshot.tip.previous_commitment),
        state_image_digest: bytes32_hex(snapshot.tip.state_image_digest),
        state_commitment: bytes32_hex(snapshot.tip.commitment),
        witness_base_generation: snapshot.base.generation.to_string(),
        witness_base_commitment: bytes32_hex(snapshot.base.commitment),
        anchor_binding_hash: bytes32_hex(binding_hash),
        anchor_service_epoch: epoch.to_string(),
        anchor_revision: revision.to_string(),
        anchor_event_root: bytes32_hex(event_root),
        anchor_acknowledgement_digest: bytes32_hex(acknowledgement_digest),
    }
}

fn validate_recovery_request(
    request: RecoverStateWitnessCheckpointRequest,
    configuration: &StateAnchorConfiguration,
) -> Result<(StateAnchorAcknowledgement, u64), EngineError> {
    let (acknowledgement, expires_at, _) = validate_read_response_with_rules(
        request,
        configuration,
        AcknowledgementParentRule::Ordinary,
        AcknowledgementTimeMode::Fresh,
        AcknowledgementTimeMode::Recovery,
    )?;
    Ok((acknowledgement, expires_at))
}

pub(crate) fn validate_certified_transition_read_response(
    request: RecoverStateWitnessCheckpointRequest,
    configuration: &StateAnchorConfiguration,
    certified_previous_event_root: [u8; 32],
    require_fresh: bool,
) -> Result<(StateAnchorAcknowledgement, u64, Vec<u8>), EngineError> {
    validate_read_response_with_rules(
        request,
        configuration,
        AcknowledgementParentRule::CertifiedEpochGenesis(certified_previous_event_root),
        if require_fresh {
            AcknowledgementTimeMode::Fresh
        } else {
            // A persisted intent is parsed intrinsically only to authenticate
            // its bounded certificate selector. It never authorizes recovery:
            // mutation resumes only through a separately verified fresh Read.
            AcknowledgementTimeMode::IntrinsicOnly
        },
        AcknowledgementTimeMode::IntrinsicOnly,
    )
}

fn validate_read_response_with_rules(
    request: RecoverStateWitnessCheckpointRequest,
    configuration: &StateAnchorConfiguration,
    parent_rule: AcknowledgementParentRule,
    wrapper_time_mode: AcknowledgementTimeMode,
    nested_time_mode: AcknowledgementTimeMode,
) -> Result<(StateAnchorAcknowledgement, u64, Vec<u8>), EngineError> {
    if request.schema != STATE_ANCHOR_READ_RESPONSE_SCHEMA {
        return Err(EngineError::Validation(format!(
            "state-anchor recovery schema must be [{STATE_ANCHOR_READ_RESPONSE_SCHEMA}]"
        )));
    }
    if request.status != "present" {
        return Err(EngineError::Validation(
            "state-anchor recovery read status must be 'present'".to_string(),
        ));
    }
    let binding_hash = parse_canonical_bytes32(&request.binding_hash, "bindingHash")?;
    if binding_hash != configuration.binding_hash {
        return Err(EngineError::Validation(
            "state-anchor recovery bindingHash does not match the manifest pin".to_string(),
        ));
    }
    let request_digest = parse_canonical_bytes32(&request.request_digest, "requestDigest")?;
    let nonce = parse_canonical_bytes32(&request.nonce, "nonce")?;
    let service_epoch = parse_canonical_u64(&request.service_epoch, "serviceEpoch")?;
    let revision = parse_canonical_u64(&request.revision, "revision")?;
    let event_root = parse_canonical_bytes32(&request.event_root, "eventRoot")?;
    let checkpoint_store_fingerprint = parse_canonical_bytes32(
        &request.checkpoint.store_fingerprint,
        "checkpoint.storeFingerprint",
    )?;
    let checkpoint_generation =
        parse_canonical_u64(&request.checkpoint.generation, "checkpoint.generation")?;
    let checkpoint_previous_commitment = parse_canonical_bytes32(
        &request.checkpoint.previous_state_commitment,
        "checkpoint.previousStateCommitment",
    )?;
    let checkpoint_state_image_digest = parse_canonical_bytes32(
        &request.checkpoint.state_image_digest,
        "checkpoint.stateImageDigest",
    )?;
    let checkpoint_state_commitment = parse_canonical_bytes32(
        &request.checkpoint.state_commitment,
        "checkpoint.stateCommitment",
    )?;
    let operation_id = parse_canonical_bytes32(&request.operation_id, "operationID")?;
    let transition_digest =
        parse_canonical_bytes32(&request.transition_digest, "transitionDigest")?;
    let committed_at_unix_ms =
        parse_canonical_u64(&request.committed_at_unix_ms, "committedAtUnixMs")?;
    let expires_at_unix_ms = parse_canonical_u64(&request.expires_at_unix_ms, "expiresAtUnixMs")?;
    validate_acknowledgement_lifetime(committed_at_unix_ms, expires_at_unix_ms, wrapper_time_mode)?;
    let checkpoint_ack_digest =
        parse_canonical_bytes32(&request.checkpoint_ack_digest, "checkpointAckDigest")?;
    let signature = parse_canonical_signature(&request.signature)?;
    if request_digest == [0u8; 32]
        || nonce == [0u8; 32]
        || service_epoch == 0
        || revision == 0
        || event_root == [0u8; 32]
        || checkpoint_store_fingerprint == [0u8; 32]
        || checkpoint_generation == 0
        || checkpoint_state_image_digest == [0u8; 32]
        || checkpoint_state_commitment == [0u8; 32]
        || operation_id == [0u8; 32]
        || transition_digest == [0u8; 32]
        || checkpoint_ack_digest == [0u8; 32]
    {
        return Err(EngineError::Validation(
            "state-anchor recovery read contains an incomplete authenticated summary".to_string(),
        ));
    }
    if checkpoint_state_commitment
        != state_commitment(
            &checkpoint_store_fingerprint,
            checkpoint_generation,
            &checkpoint_previous_commitment,
            &checkpoint_state_image_digest,
        )
    {
        return Err(EngineError::Validation(
            "state-anchor recovery checkpoint commitment is invalid".to_string(),
        ));
    }

    let raw_acknowledgement = request.checkpoint_ack.get().as_bytes();
    let raw_acknowledgement_digest: [u8; 32] = Sha256::digest(raw_acknowledgement).into();
    let signing_digest = state_anchor_read_response_signing_digest(
        &binding_hash,
        &request_digest,
        &nonce,
        service_epoch,
        revision,
        &event_root,
        &checkpoint_store_fingerprint,
        checkpoint_generation,
        &checkpoint_previous_commitment,
        &checkpoint_state_image_digest,
        &checkpoint_state_commitment,
        &operation_id,
        &transition_digest,
        committed_at_unix_ms,
        expires_at_unix_ms,
        &checkpoint_ack_digest,
        &raw_acknowledgement_digest,
    );
    let verifying_key =
        VerifyingKey::from_bytes(&configuration.response_public_key).map_err(|error| {
            EngineError::Internal(format!(
                "configured Ed25519 state-anchor response key became invalid: {error}"
            ))
        })?;
    verifying_key
        .verify_strict(&signing_digest, &Signature::from_bytes(&signature))
        .map_err(|_| {
            EngineError::Validation("state-anchor recovery read signature is invalid".to_string())
        })?;

    let nested_request: AcknowledgeStateWitnessCheckpointRequest =
        serde_json::from_slice(raw_acknowledgement).map_err(|error| {
            EngineError::Validation(format!(
                "state-anchor recovery nested acknowledgement is invalid: {error}"
            ))
        })?;
    // The fresh read wrapper, not the historical nested response, supplies
    // replay freshness. Every intrinsic timestamp, signature, pin, transcript,
    // event-root, and checkpoint rule on the original response still applies.
    let acknowledgement = validate_acknowledgement_request_with_rules(
        nested_request,
        configuration,
        nested_time_mode,
        parent_rule,
        AcknowledgementStatusRule::AllowAppliedOrReplay,
    )?;
    if acknowledgement.service_epoch != service_epoch
        || acknowledgement.revision != revision
        || acknowledgement.event_root != event_root
        || acknowledgement.checkpoint_store_fingerprint != checkpoint_store_fingerprint
        || acknowledgement.checkpoint_generation != checkpoint_generation
        || acknowledgement.checkpoint_previous_commitment != checkpoint_previous_commitment
        || acknowledgement.checkpoint_state_image_digest != checkpoint_state_image_digest
        || acknowledgement.checkpoint_state_commitment != checkpoint_state_commitment
        || acknowledgement.operation_id != operation_id
        || acknowledgement.transition_digest != transition_digest
        || acknowledgement.acknowledgement_digest != checkpoint_ack_digest
    {
        return Err(EngineError::Validation(
            "state-anchor recovery read summary differs from its exact nested acknowledgement"
                .to_string(),
        ));
    }
    Ok((
        acknowledgement,
        expires_at_unix_ms,
        raw_acknowledgement.to_vec(),
    ))
}

#[allow(clippy::too_many_arguments)]
fn state_anchor_read_response_signing_digest(
    binding_hash: &[u8; 32],
    request_digest: &[u8; 32],
    nonce: &[u8; 32],
    service_epoch: u64,
    revision: u64,
    event_root: &[u8; 32],
    checkpoint_store_fingerprint: &[u8; 32],
    checkpoint_generation: u64,
    checkpoint_previous_commitment: &[u8; 32],
    checkpoint_state_image_digest: &[u8; 32],
    checkpoint_state_commitment: &[u8; 32],
    operation_id: &[u8; 32],
    transition_digest: &[u8; 32],
    committed_at_unix_ms: u64,
    expires_at_unix_ms: u64,
    checkpoint_ack_digest: &[u8; 32],
    raw_acknowledgement_digest: &[u8; 32],
) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(STATE_ANCHOR_READ_RESPONSE_DOMAIN);
    digest.update(binding_hash);
    digest.update(request_digest);
    digest.update(nonce);
    digest.update([1u8]); // present
    digest.update(service_epoch.to_be_bytes());
    digest.update(revision.to_be_bytes());
    digest.update(event_root);
    digest.update(checkpoint_store_fingerprint);
    digest.update(checkpoint_generation.to_be_bytes());
    digest.update(checkpoint_previous_commitment);
    digest.update(checkpoint_state_image_digest);
    digest.update(checkpoint_state_commitment);
    digest.update(operation_id);
    digest.update(transition_digest);
    digest.update(committed_at_unix_ms.to_be_bytes());
    digest.update(expires_at_unix_ms.to_be_bytes());
    digest.update(checkpoint_ack_digest);
    digest.update(raw_acknowledgement_digest);
    digest.finalize().into()
}

#[cfg(test)]
#[allow(clippy::too_many_arguments)]
pub(crate) fn state_anchor_read_response_signing_digest_for_tests(
    binding_hash: &[u8; 32],
    request_digest: &[u8; 32],
    nonce: &[u8; 32],
    service_epoch: u64,
    revision: u64,
    event_root: &[u8; 32],
    checkpoint_store_fingerprint: &[u8; 32],
    checkpoint_generation: u64,
    checkpoint_previous_commitment: &[u8; 32],
    checkpoint_state_image_digest: &[u8; 32],
    checkpoint_state_commitment: &[u8; 32],
    operation_id: &[u8; 32],
    transition_digest: &[u8; 32],
    committed_at_unix_ms: u64,
    expires_at_unix_ms: u64,
    checkpoint_ack_digest: &[u8; 32],
    raw_acknowledgement_digest: &[u8; 32],
) -> [u8; 32] {
    state_anchor_read_response_signing_digest(
        binding_hash,
        request_digest,
        nonce,
        service_epoch,
        revision,
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
        checkpoint_ack_digest,
        raw_acknowledgement_digest,
    )
}

fn validate_acknowledgement_request(
    request: AcknowledgeStateWitnessCheckpointRequest,
    configuration: &StateAnchorConfiguration,
    require_fresh: bool,
) -> Result<StateAnchorAcknowledgement, EngineError> {
    validate_acknowledgement_request_with_rules(
        request,
        configuration,
        if require_fresh {
            AcknowledgementTimeMode::Fresh
        } else {
            AcknowledgementTimeMode::Recovery
        },
        AcknowledgementParentRule::Ordinary,
        AcknowledgementStatusRule::AllowAppliedOrReplay,
    )
}

pub(crate) fn validate_certified_transition_acknowledgement(
    request: AcknowledgeStateWitnessCheckpointRequest,
    configuration: &StateAnchorConfiguration,
    certified_previous_event_root: [u8; 32],
) -> Result<StateAnchorAcknowledgement, EngineError> {
    validate_acknowledgement_request_with_rules(
        request,
        configuration,
        AcknowledgementTimeMode::IntrinsicOnly,
        AcknowledgementParentRule::CertifiedEpochGenesis(certified_previous_event_root),
        AcknowledgementStatusRule::AppliedOnly,
    )
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum AcknowledgementParentRule {
    Ordinary,
    CertifiedEpochGenesis([u8; 32]),
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum AcknowledgementStatusRule {
    AllowAppliedOrReplay,
    AppliedOnly,
}

fn validate_acknowledgement_request_with_rules(
    request: AcknowledgeStateWitnessCheckpointRequest,
    configuration: &StateAnchorConfiguration,
    time_mode: AcknowledgementTimeMode,
    parent_rule: AcknowledgementParentRule,
    status_rule: AcknowledgementStatusRule,
) -> Result<StateAnchorAcknowledgement, EngineError> {
    if request.schema != TBTC_SIGNER_STATE_WITNESS_CHECKPOINT_ACK_SCHEMA {
        return Err(EngineError::Validation(format!(
            "state-anchor acknowledgement schema must be [{}]",
            TBTC_SIGNER_STATE_WITNESS_CHECKPOINT_ACK_SCHEMA
        )));
    }
    let binding_hash = parse_canonical_bytes32(&request.binding_hash, "bindingHash")?;
    if binding_hash != configuration.binding_hash {
        return Err(EngineError::Validation(
            "state-anchor acknowledgement bindingHash does not match the manifest pin".to_string(),
        ));
    }
    let request_digest = parse_canonical_bytes32(&request.request_digest, "requestDigest")?;
    let nonce = parse_canonical_bytes32(&request.nonce, "nonce")?;
    let status = match request.status.as_str() {
        "applied" => 1,
        "already-applied" if status_rule == AcknowledgementStatusRule::AllowAppliedOrReplay => 2,
        _ => {
            let allowed = if status_rule == AcknowledgementStatusRule::AppliedOnly {
                "'applied'"
            } else {
                "'applied' or 'already-applied'"
            };
            return Err(EngineError::Validation(format!(
                "state-anchor acknowledgement status must be {allowed}"
            )));
        }
    };
    let service_epoch = parse_canonical_u64(&request.service_epoch, "serviceEpoch")?;
    let revision = parse_canonical_u64(&request.revision, "revision")?;
    let previous_event_root =
        parse_canonical_bytes32(&request.previous_event_root, "previousEventRoot")?;
    let event_root = parse_canonical_bytes32(&request.event_root, "eventRoot")?;
    let checkpoint_store_fingerprint = parse_canonical_bytes32(
        &request.checkpoint.store_fingerprint,
        "checkpoint.storeFingerprint",
    )?;
    let checkpoint_generation =
        parse_canonical_u64(&request.checkpoint.generation, "checkpoint.generation")?;
    let checkpoint_previous_commitment = parse_canonical_bytes32(
        &request.checkpoint.previous_state_commitment,
        "checkpoint.previousStateCommitment",
    )?;
    let checkpoint_state_image_digest = parse_canonical_bytes32(
        &request.checkpoint.state_image_digest,
        "checkpoint.stateImageDigest",
    )?;
    let checkpoint_state_commitment = parse_canonical_bytes32(
        &request.checkpoint.state_commitment,
        "checkpoint.stateCommitment",
    )?;
    let operation_id = parse_canonical_bytes32(&request.operation_id, "operationID")?;
    let transition_digest =
        parse_canonical_bytes32(&request.transition_digest, "transitionDigest")?;
    let committed_at_unix_ms =
        parse_canonical_u64(&request.committed_at_unix_ms, "committedAtUnixMs")?;
    let expires_at_unix_ms = parse_canonical_u64(&request.expires_at_unix_ms, "expiresAtUnixMs")?;
    if request_digest == [0u8; 32]
        || nonce == [0u8; 32]
        || service_epoch == 0
        || revision == 0
        || event_root == [0u8; 32]
        || checkpoint_store_fingerprint == [0u8; 32]
        || checkpoint_generation == 0
        || checkpoint_state_image_digest == [0u8; 32]
        || checkpoint_state_commitment == [0u8; 32]
        || operation_id == [0u8; 32]
        || transition_digest == [0u8; 32]
        || !match parent_rule {
            AcknowledgementParentRule::Ordinary => {
                (revision == 1 && previous_event_root == [0u8; 32])
                    || (revision > 1 && previous_event_root != [0u8; 32])
            }
            AcknowledgementParentRule::CertifiedEpochGenesis(certified_parent) => {
                (revision == 1 && previous_event_root == certified_parent)
                    || (revision > 1 && previous_event_root != [0u8; 32])
            }
        }
    {
        return Err(EngineError::Validation(
            "state-anchor acknowledgement contains an incomplete authenticated summary".to_string(),
        ));
    }
    if checkpoint_state_commitment
        != state_commitment(
            &checkpoint_store_fingerprint,
            checkpoint_generation,
            &checkpoint_previous_commitment,
            &checkpoint_state_image_digest,
        )
    {
        return Err(EngineError::Validation(
            "state-anchor acknowledgement checkpoint commitment is invalid".to_string(),
        ));
    }
    validate_acknowledgement_lifetime(committed_at_unix_ms, expires_at_unix_ms, time_mode)?;
    let signature = parse_canonical_signature(&request.signature)?;

    let signing_digest = state_anchor_signing_digest(
        &binding_hash,
        &request_digest,
        &nonce,
        status,
        service_epoch,
        revision,
        &previous_event_root,
        &event_root,
        &checkpoint_store_fingerprint,
        checkpoint_generation,
        &checkpoint_previous_commitment,
        &checkpoint_state_image_digest,
        &checkpoint_state_commitment,
        &operation_id,
        &transition_digest,
        committed_at_unix_ms,
        expires_at_unix_ms,
    );
    let verifying_key =
        VerifyingKey::from_bytes(&configuration.response_public_key).map_err(|error| {
            EngineError::Internal(format!(
                "configured Ed25519 state-anchor response key became invalid: {error}"
            ))
        })?;
    let signature_value = Signature::from_bytes(&signature);
    verifying_key
        .verify_strict(&signing_digest, &signature_value)
        .map_err(|_| {
            EngineError::Validation(
                "state-anchor acknowledgement Ed25519 signature is invalid".to_string(),
            )
        })?;
    let acknowledgement_digest = state_anchor_acknowledgement_digest(
        &signing_digest,
        &signature,
        &configuration.response_public_key_spki_sha256,
    );

    let acknowledgement = StateAnchorAcknowledgement {
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
        configured_spki_hash: configuration.response_public_key_spki_sha256,
        acknowledgement_digest,
    };
    validate_state_anchor_event_root(&acknowledgement)?;
    Ok(acknowledgement)
}

pub(crate) fn validate_persisted_state_anchor_acknowledgement(
    acknowledgement: &StateAnchorAcknowledgement,
    configuration: &StateAnchorConfiguration,
) -> Result<(), EngineError> {
    if acknowledgement.binding_hash != configuration.binding_hash
        || acknowledgement.configured_spki_hash != configuration.response_public_key_spki_sha256
    {
        return Err(EngineError::Internal(
            "persisted state-anchor acknowledgement does not match manifest pins".to_string(),
        ));
    }
    validate_acknowledgement_lifetime(
        acknowledgement.committed_at_unix_ms,
        acknowledgement.expires_at_unix_ms,
        AcknowledgementTimeMode::IntrinsicOnly,
    )
    .map_err(|error| {
        EngineError::Internal(format!(
            "persisted state-anchor acknowledgement lifetime is invalid: {error}"
        ))
    })?;
    validate_state_anchor_event_root(acknowledgement).map_err(|error| {
        EngineError::Internal(format!(
            "persisted state-anchor acknowledgement event root is invalid: {error}"
        ))
    })?;
    let signing_digest = state_anchor_signing_digest(
        &acknowledgement.binding_hash,
        &acknowledgement.request_digest,
        &acknowledgement.nonce,
        acknowledgement.status,
        acknowledgement.service_epoch,
        acknowledgement.revision,
        &acknowledgement.previous_event_root,
        &acknowledgement.event_root,
        &acknowledgement.checkpoint_store_fingerprint,
        acknowledgement.checkpoint_generation,
        &acknowledgement.checkpoint_previous_commitment,
        &acknowledgement.checkpoint_state_image_digest,
        &acknowledgement.checkpoint_state_commitment,
        &acknowledgement.operation_id,
        &acknowledgement.transition_digest,
        acknowledgement.committed_at_unix_ms,
        acknowledgement.expires_at_unix_ms,
    );
    if signing_digest != acknowledgement.signing_digest {
        return Err(EngineError::Internal(
            "persisted state-anchor signing digest is invalid".to_string(),
        ));
    }
    let verifying_key =
        VerifyingKey::from_bytes(&configuration.response_public_key).map_err(|error| {
            EngineError::Internal(format!(
                "configured Ed25519 state-anchor response key became invalid: {error}"
            ))
        })?;
    verifying_key
        .verify_strict(
            &signing_digest,
            &Signature::from_bytes(&acknowledgement.signature),
        )
        .map_err(|_| {
            EngineError::Internal(
                "persisted state-anchor acknowledgement signature is invalid".to_string(),
            )
        })?;
    let acknowledgement_digest = state_anchor_acknowledgement_digest(
        &signing_digest,
        &acknowledgement.signature,
        &configuration.response_public_key_spki_sha256,
    );
    if acknowledgement_digest != acknowledgement.acknowledgement_digest {
        return Err(EngineError::Internal(
            "persisted state-anchor acknowledgement digest is invalid".to_string(),
        ));
    }
    Ok(())
}

fn validate_state_anchor_event_root(
    acknowledgement: &StateAnchorAcknowledgement,
) -> Result<(), EngineError> {
    let expected = state_anchor_event_root(acknowledgement);
    if expected != acknowledgement.event_root {
        return Err(EngineError::Validation(
            "state-anchor acknowledgement eventRoot is invalid".to_string(),
        ));
    }
    Ok(())
}

fn state_anchor_event_root(acknowledgement: &StateAnchorAcknowledgement) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(STATE_ANCHOR_EVENT_ROOT_DOMAIN);
    digest.update(acknowledgement.binding_hash);
    digest.update(acknowledgement.service_epoch.to_be_bytes());
    digest.update(acknowledgement.revision.to_be_bytes());
    digest.update(acknowledgement.previous_event_root);
    digest.update(acknowledgement.request_digest);
    digest.update(acknowledgement.nonce);
    digest.update([acknowledgement.status]);
    digest.update(acknowledgement.checkpoint_store_fingerprint);
    digest.update(acknowledgement.checkpoint_generation.to_be_bytes());
    digest.update(acknowledgement.checkpoint_previous_commitment);
    digest.update(acknowledgement.checkpoint_state_image_digest);
    digest.update(acknowledgement.checkpoint_state_commitment);
    digest.update(acknowledgement.operation_id);
    digest.update(acknowledgement.transition_digest);
    digest.update(acknowledgement.committed_at_unix_ms.to_be_bytes());
    digest.update(acknowledgement.expires_at_unix_ms.to_be_bytes());
    digest.finalize().into()
}

#[cfg(test)]
pub(crate) fn state_anchor_event_root_for_tests(
    acknowledgement: &StateAnchorAcknowledgement,
) -> [u8; 32] {
    state_anchor_event_root(acknowledgement)
}

#[cfg(test)]
fn validate_acknowledgement_time(
    committed_at_unix_ms: u64,
    expires_at_unix_ms: u64,
) -> Result<(), EngineError> {
    validate_acknowledgement_lifetime(
        committed_at_unix_ms,
        expires_at_unix_ms,
        AcknowledgementTimeMode::Fresh,
    )
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum AcknowledgementTimeMode {
    IntrinsicOnly,
    Recovery,
    Fresh,
}

fn validate_acknowledgement_lifetime(
    committed_at_unix_ms: u64,
    expires_at_unix_ms: u64,
    mode: AcknowledgementTimeMode,
) -> Result<(), EngineError> {
    if committed_at_unix_ms == 0 || committed_at_unix_ms >= expires_at_unix_ms {
        return Err(EngineError::Validation(
            "state-anchor acknowledgement committedAtUnixMs must be nonzero and precede \
             expiresAtUnixMs"
                .to_string(),
        ));
    }
    let ttl = expires_at_unix_ms
        .checked_sub(committed_at_unix_ms)
        .ok_or_else(|| {
            EngineError::Validation(
                "state-anchor acknowledgement timestamp subtraction overflowed".to_string(),
            )
        })?;
    if ttl > ACKNOWLEDGEMENT_MAX_TTL_MILLISECONDS {
        return Err(EngineError::Validation(format!(
            "state-anchor acknowledgement TTL [{ttl}] exceeds 30000 milliseconds"
        )));
    }
    if mode == AcknowledgementTimeMode::IntrinsicOnly {
        return Ok(());
    }
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| {
            EngineError::Internal(
                "system clock is before the Unix epoch while validating state-anchor acknowledgement"
                    .to_string(),
            )
        })?
        .as_millis();
    let now = u64::try_from(now).map_err(|_| {
        EngineError::Internal("system Unix-millisecond clock does not fit in u64".to_string())
    })?;
    let latest_committed = now
        .checked_add(ACKNOWLEDGEMENT_MAX_FUTURE_SKEW_MILLISECONDS)
        .ok_or_else(|| {
            EngineError::Internal(
                "system clock overflowed the state-anchor future-skew window".to_string(),
            )
        })?;
    if committed_at_unix_ms > latest_committed {
        return Err(EngineError::Validation(
            "state-anchor acknowledgement committedAtUnixMs exceeds the 5000ms future-skew window"
                .to_string(),
        ));
    }
    if mode == AcknowledgementTimeMode::Fresh && now >= expires_at_unix_ms {
        return Err(EngineError::Validation(
            "state-anchor acknowledgement is expired".to_string(),
        ));
    }
    Ok(())
}

pub(crate) fn recheck_state_anchor_admission_expiry(
    expires_at_unix_ms: u64,
) -> Result<(), EngineError> {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| {
            EngineError::Internal(
                "system clock is before the Unix epoch while rechecking state-anchor freshness"
                    .to_string(),
            )
        })?
        .as_millis();
    let now = u64::try_from(now).map_err(|_| {
        EngineError::Internal("system Unix-millisecond clock does not fit in u64".to_string())
    })?;
    if now >= expires_at_unix_ms {
        return Err(EngineError::Validation(
            "state-anchor admission expired while waiting for serialized persistence".to_string(),
        ));
    }
    Ok(())
}

#[allow(clippy::too_many_arguments)]
fn state_anchor_signing_digest(
    binding_hash: &[u8; 32],
    request_digest: &[u8; 32],
    nonce: &[u8; 32],
    status: u8,
    service_epoch: u64,
    revision: u64,
    previous_event_root: &[u8; 32],
    event_root: &[u8; 32],
    checkpoint_store_fingerprint: &[u8; 32],
    checkpoint_generation: u64,
    checkpoint_previous_commitment: &[u8; 32],
    checkpoint_state_image_digest: &[u8; 32],
    checkpoint_state_commitment: &[u8; 32],
    operation_id: &[u8; 32],
    transition_digest: &[u8; 32],
    committed_at_unix_ms: u64,
    expires_at_unix_ms: u64,
) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(STATE_ANCHOR_SERVICE_RESPONSE_DOMAIN);
    digest.update(binding_hash);
    digest.update(request_digest);
    digest.update(nonce);
    digest.update([status]);
    digest.update(service_epoch.to_be_bytes());
    digest.update(revision.to_be_bytes());
    digest.update(previous_event_root);
    digest.update(event_root);
    digest.update(checkpoint_store_fingerprint);
    digest.update(checkpoint_generation.to_be_bytes());
    digest.update(checkpoint_previous_commitment);
    digest.update(checkpoint_state_image_digest);
    digest.update(checkpoint_state_commitment);
    digest.update(operation_id);
    digest.update(transition_digest);
    digest.update(committed_at_unix_ms.to_be_bytes());
    digest.update(expires_at_unix_ms.to_be_bytes());
    digest.finalize().into()
}

fn state_anchor_acknowledgement_digest(
    signing_digest: &[u8; 32],
    signature: &[u8; 64],
    configured_spki_hash: &[u8; 32],
) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(STATE_ANCHOR_ACKNOWLEDGEMENT_DOMAIN);
    digest.update(signing_digest);
    digest.update(signature);
    digest.update(configured_spki_hash);
    digest.finalize().into()
}

#[cfg(test)]
pub(crate) fn state_anchor_acknowledgement_digest_for_tests(
    signing_digest: &[u8; 32],
    signature: &[u8; 64],
    configured_spki_hash: &[u8; 32],
) -> [u8; 32] {
    state_anchor_acknowledgement_digest(signing_digest, signature, configured_spki_hash)
}

pub(crate) fn ed25519_spki_sha256(public_key: &[u8; 32]) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(ED25519_SPKI_PREFIX);
    digest.update(public_key);
    digest.finalize().into()
}

pub(crate) fn validate_strong_ed25519_verifying_key(
    public_key: &[u8; 32],
    label: &str,
) -> Result<VerifyingKey, EngineError> {
    use curve25519_dalek::edwards::CompressedEdwardsY;

    let point = CompressedEdwardsY(*public_key)
        .decompress()
        .ok_or_else(|| {
            EngineError::Validation(format!(
                "{label} is not a canonical compressed Edwards25519 point"
            ))
        })?;
    if point.compress().to_bytes() != *public_key {
        return Err(EngineError::Validation(format!(
            "{label} is not a canonical compressed Edwards25519 point"
        )));
    }
    if point.is_small_order() || !point.is_torsion_free() {
        return Err(EngineError::Validation(format!(
            "{label} must be a non-identity prime-subgroup Ed25519 public key"
        )));
    }
    VerifyingKey::from_bytes(public_key).map_err(|error| {
        EngineError::Validation(format!(
            "{label} is not a valid Ed25519 public key: {error}"
        ))
    })
}

pub(crate) fn parse_canonical_bytes32(value: &str, label: &str) -> Result<[u8; 32], EngineError> {
    if value.len() != 66
        || !value.starts_with("0x")
        || value.as_bytes().iter().any(u8::is_ascii_uppercase)
    {
        return Err(EngineError::Validation(format!(
            "{label} must be canonical lowercase 0x-prefixed bytes32"
        )));
    }
    let bytes = hex::decode(&value[2..]).map_err(|_| {
        EngineError::Validation(format!(
            "{label} must be canonical lowercase 0x-prefixed bytes32"
        ))
    })?;
    let mut result = [0u8; 32];
    result.copy_from_slice(&bytes);
    Ok(result)
}

pub(crate) fn parse_canonical_signature(value: &str) -> Result<[u8; 64], EngineError> {
    if value.len() != 130
        || !value.starts_with("0x")
        || value.as_bytes().iter().any(u8::is_ascii_uppercase)
    {
        return Err(EngineError::Validation(
            "signature must be canonical lowercase 0x-prefixed 64-byte hex".to_string(),
        ));
    }
    let bytes = hex::decode(&value[2..]).map_err(|_| {
        EngineError::Validation(
            "signature must be canonical lowercase 0x-prefixed 64-byte hex".to_string(),
        )
    })?;
    let mut signature = [0u8; 64];
    signature.copy_from_slice(&bytes);
    Ok(signature)
}

pub(crate) fn parse_canonical_u64(value: &str, label: &str) -> Result<u64, EngineError> {
    if value.is_empty()
        || (value.len() > 1 && value.starts_with('0'))
        || !value.bytes().all(|byte| byte.is_ascii_digit())
    {
        return Err(EngineError::Validation(format!(
            "{label} must be a canonical unsigned decimal string"
        )));
    }
    value.parse::<u64>().map_err(|_| {
        EngineError::Validation(format!(
            "{label} must be a canonical unsigned decimal u64 string"
        ))
    })
}

fn parse_canonical_usize(value: &str, label: &str) -> Result<usize, EngineError> {
    let parsed = parse_canonical_u64(value, label)?;
    usize::try_from(parsed)
        .map_err(|_| EngineError::Validation(format!("{label} does not fit this platform")))
}

#[cfg(test)]
pub(crate) fn state_anchor_signing_digest_for_tests(
    acknowledgement: &StateAnchorAcknowledgement,
) -> [u8; 32] {
    state_anchor_signing_digest(
        &acknowledgement.binding_hash,
        &acknowledgement.request_digest,
        &acknowledgement.nonce,
        acknowledgement.status,
        acknowledgement.service_epoch,
        acknowledgement.revision,
        &acknowledgement.previous_event_root,
        &acknowledgement.event_root,
        &acknowledgement.checkpoint_store_fingerprint,
        acknowledgement.checkpoint_generation,
        &acknowledgement.checkpoint_previous_commitment,
        &acknowledgement.checkpoint_state_image_digest,
        &acknowledgement.checkpoint_state_commitment,
        &acknowledgement.operation_id,
        &acknowledgement.transition_digest,
        acknowledgement.committed_at_unix_ms,
        acknowledgement.expires_at_unix_ms,
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::{Signer, SigningKey};

    fn acknowledgement_wire(
        acknowledgement: &StateAnchorAcknowledgement,
    ) -> AcknowledgeStateWitnessCheckpointRequest {
        AcknowledgeStateWitnessCheckpointRequest {
            schema: TBTC_SIGNER_STATE_WITNESS_CHECKPOINT_ACK_SCHEMA.to_string(),
            binding_hash: bytes32_hex(acknowledgement.binding_hash),
            request_digest: bytes32_hex(acknowledgement.request_digest),
            nonce: bytes32_hex(acknowledgement.nonce),
            status: match acknowledgement.status {
                1 => "applied",
                2 => "already-applied",
                _ => panic!("invalid fixture status"),
            }
            .to_string(),
            service_epoch: acknowledgement.service_epoch.to_string(),
            revision: acknowledgement.revision.to_string(),
            previous_event_root: bytes32_hex(acknowledgement.previous_event_root),
            event_root: bytes32_hex(acknowledgement.event_root),
            checkpoint: crate::api::StateWitnessCheckpointRequest {
                store_fingerprint: bytes32_hex(acknowledgement.checkpoint_store_fingerprint),
                generation: acknowledgement.checkpoint_generation.to_string(),
                previous_state_commitment: bytes32_hex(
                    acknowledgement.checkpoint_previous_commitment,
                ),
                state_image_digest: bytes32_hex(acknowledgement.checkpoint_state_image_digest),
                state_commitment: bytes32_hex(acknowledgement.checkpoint_state_commitment),
            },
            operation_id: bytes32_hex(acknowledgement.operation_id),
            transition_digest: bytes32_hex(acknowledgement.transition_digest),
            committed_at_unix_ms: acknowledgement.committed_at_unix_ms.to_string(),
            expires_at_unix_ms: acknowledgement.expires_at_unix_ms.to_string(),
            signature: format!("0x{}", hex::encode(acknowledgement.signature)),
        }
    }

    fn signed_acknowledgement_fixture(
        now: u64,
    ) -> (
        SigningKey,
        StateAnchorConfiguration,
        StateAnchorAcknowledgement,
    ) {
        let signing_key = SigningKey::from_bytes(&[0x01; 32]);
        let response_public_key = signing_key.verifying_key().to_bytes();
        let configured_spki_hash = ed25519_spki_sha256(&response_public_key);
        let checkpoint_store_fingerprint = [0x66; 32];
        let checkpoint_generation = 7;
        let checkpoint_previous_commitment = [0x77; 32];
        let checkpoint_state_image_digest = [0x88; 32];
        let checkpoint_state_commitment = state_commitment(
            &checkpoint_store_fingerprint,
            checkpoint_generation,
            &checkpoint_previous_commitment,
            &checkpoint_state_image_digest,
        );
        let mut acknowledgement = StateAnchorAcknowledgement {
            binding_hash: [0x11; 32],
            request_digest: [0x22; 32],
            nonce: [0x33; 32],
            status: 1,
            service_epoch: 2,
            revision: 3,
            previous_event_root: [0x44; 32],
            event_root: [0; 32],
            checkpoint_store_fingerprint,
            checkpoint_generation,
            checkpoint_previous_commitment,
            checkpoint_state_image_digest,
            checkpoint_state_commitment,
            operation_id: [0xaa; 32],
            transition_digest: [0xbb; 32],
            committed_at_unix_ms: now - 60_000,
            expires_at_unix_ms: now - 30_000,
            signing_digest: [0; 32],
            signature: [0; 64],
            configured_spki_hash,
            acknowledgement_digest: [0; 32],
        };
        resign_acknowledgement(&mut acknowledgement, &signing_key);
        let configuration = StateAnchorConfiguration {
            binding_hash: acknowledgement.binding_hash,
            response_public_key,
            response_public_key_spki_sha256: configured_spki_hash,
            rotation_threshold_records: 8,
            trust: None,
        };
        (signing_key, configuration, acknowledgement)
    }

    fn resign_acknowledgement(
        acknowledgement: &mut StateAnchorAcknowledgement,
        signing_key: &SigningKey,
    ) {
        acknowledgement.event_root = state_anchor_event_root(acknowledgement);
        acknowledgement.signing_digest = state_anchor_signing_digest_for_tests(acknowledgement);
        acknowledgement.signature = signing_key.sign(&acknowledgement.signing_digest).to_bytes();
        acknowledgement.acknowledgement_digest = state_anchor_acknowledgement_digest(
            &acknowledgement.signing_digest,
            &acknowledgement.signature,
            &acknowledgement.configured_spki_hash,
        );
    }

    fn recovery_wire(
        acknowledgement: &StateAnchorAcknowledgement,
        signing_key: &SigningKey,
        raw_acknowledgement: String,
        committed_at_unix_ms: u64,
        expires_at_unix_ms: u64,
    ) -> RecoverStateWitnessCheckpointRequest {
        let request_digest = [0xcc; 32];
        let nonce = [0xdd; 32];
        let raw_acknowledgement_digest: [u8; 32] =
            Sha256::digest(raw_acknowledgement.as_bytes()).into();
        let signing_digest = state_anchor_read_response_signing_digest(
            &acknowledgement.binding_hash,
            &request_digest,
            &nonce,
            acknowledgement.service_epoch,
            acknowledgement.revision,
            &acknowledgement.event_root,
            &acknowledgement.checkpoint_store_fingerprint,
            acknowledgement.checkpoint_generation,
            &acknowledgement.checkpoint_previous_commitment,
            &acknowledgement.checkpoint_state_image_digest,
            &acknowledgement.checkpoint_state_commitment,
            &acknowledgement.operation_id,
            &acknowledgement.transition_digest,
            committed_at_unix_ms,
            expires_at_unix_ms,
            &acknowledgement.acknowledgement_digest,
            &raw_acknowledgement_digest,
        );
        RecoverStateWitnessCheckpointRequest {
            schema: STATE_ANCHOR_READ_RESPONSE_SCHEMA.to_string(),
            binding_hash: bytes32_hex(acknowledgement.binding_hash),
            request_digest: bytes32_hex(request_digest),
            nonce: bytes32_hex(nonce),
            status: "present".to_string(),
            service_epoch: acknowledgement.service_epoch.to_string(),
            revision: acknowledgement.revision.to_string(),
            event_root: bytes32_hex(acknowledgement.event_root),
            checkpoint: crate::api::StateWitnessCheckpointRequest {
                store_fingerprint: bytes32_hex(acknowledgement.checkpoint_store_fingerprint),
                generation: acknowledgement.checkpoint_generation.to_string(),
                previous_state_commitment: bytes32_hex(
                    acknowledgement.checkpoint_previous_commitment,
                ),
                state_image_digest: bytes32_hex(acknowledgement.checkpoint_state_image_digest),
                state_commitment: bytes32_hex(acknowledgement.checkpoint_state_commitment),
            },
            operation_id: bytes32_hex(acknowledgement.operation_id),
            transition_digest: bytes32_hex(acknowledgement.transition_digest),
            committed_at_unix_ms: committed_at_unix_ms.to_string(),
            expires_at_unix_ms: expires_at_unix_ms.to_string(),
            checkpoint_ack: serde_json::value::RawValue::from_string(raw_acknowledgement)
                .expect("valid raw acknowledgement"),
            checkpoint_ack_digest: bytes32_hex(acknowledgement.acknowledgement_digest),
            signature: format!(
                "0x{}",
                hex::encode(signing_key.sign(&signing_digest).to_bytes())
            ),
        }
    }

    fn now_milliseconds() -> u64 {
        u64::try_from(
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .expect("clock after epoch")
                .as_millis(),
        )
        .expect("clock fits u64")
    }

    #[test]
    fn acknowledgement_time_window_rejects_expired_future_and_oversized_ttl() {
        let now = now_milliseconds();
        validate_acknowledgement_time(now.saturating_sub(1), now + 1_000)
            .expect("fresh acknowledgement");
        assert!(
            validate_acknowledgement_time(now.saturating_sub(2_000), now.saturating_sub(1))
                .is_err()
        );
        assert!(validate_acknowledgement_time(now + 6_000, now + 7_000).is_err());
        assert!(validate_acknowledgement_time(now, now + 30_001).is_err());
        assert!(validate_acknowledgement_time(now, now).is_err());
    }

    #[test]
    fn canonical_wire_scalars_reject_alias_encodings() {
        assert_eq!(parse_canonical_u64("0", "value").expect("zero"), 0);
        assert!(parse_canonical_u64("00", "value").is_err());
        assert!(parse_canonical_u64("+1", "value").is_err());
        assert!(parse_canonical_bytes32(&format!("0x{}", "ab".repeat(32)), "hash").is_ok());
        assert!(parse_canonical_bytes32(&format!("0x{}", "AB".repeat(32)), "hash").is_err());
        assert!(parse_canonical_signature(&format!("0x{}", "ab".repeat(64))).is_ok());
        assert!(parse_canonical_signature(&format!("0x{}", "ab".repeat(63))).is_err());
    }

    #[test]
    fn anchor_transcripts_match_frozen_go_vectors() {
        let acknowledgement = StateAnchorAcknowledgement {
            binding_hash: [0x11; 32],
            request_digest: [0x22; 32],
            nonce: [0x33; 32],
            status: 1,
            service_epoch: 2,
            revision: 3,
            previous_event_root: [0x44; 32],
            event_root: [0x55; 32],
            checkpoint_store_fingerprint: [0x66; 32],
            checkpoint_generation: 7,
            checkpoint_previous_commitment: [0x77; 32],
            checkpoint_state_image_digest: [0x88; 32],
            checkpoint_state_commitment: [0x99; 32],
            operation_id: [0xaa; 32],
            transition_digest: [0xbb; 32],
            committed_at_unix_ms: 1_700_000_000_000,
            expires_at_unix_ms: 1_700_000_030_000,
            signing_digest: [0; 32],
            signature: [0; 64],
            configured_spki_hash: [0; 32],
            acknowledgement_digest: [0; 32],
        };
        let signing_digest = state_anchor_signing_digest_for_tests(&acknowledgement);
        assert_eq!(
            hex::encode(signing_digest),
            "55f88c32a0b168003cedfb88cf47a467b607dbd1f2ab6f20ddc7976bd396b239"
        );
        let signing_key = SigningKey::from_bytes(&[0x01; 32]);
        let signature = signing_key.sign(&signing_digest).to_bytes();
        assert_eq!(
            hex::encode(signature),
            concat!(
                "0a60e68808285197c4ddb4b68dc10439aad6cbde085fd93b7cf863b7abf819713",
                "1d73f35304862ea80dc5cfd88d0cac80f9fa42b54efa036b0a82956c62f0608"
            )
        );
        let spki_hash = ed25519_spki_sha256(&signing_key.verifying_key().to_bytes());
        assert_eq!(
            hex::encode(state_anchor_acknowledgement_digest(
                &signing_digest,
                &signature,
                &spki_hash,
            )),
            "4c30e2aa6a048993fede1a754a0567a6faef8180544398ff284567f722c6ad01"
        );
        assert_eq!(
            hex::encode(state_anchor_event_root(&acknowledgement)),
            "251cf2f635ea82533f55d323104232ecfd47a748a45fbbe16e8ed212c8c69a90"
        );

        let raw_acknowledgement_digest: [u8; 32] = Sha256::digest(br#"{"x":1}"#).into();
        let read_digest = state_anchor_read_response_signing_digest(
            &[0x11; 32],
            &[0x22; 32],
            &[0x33; 32],
            2,
            3,
            &[0x44; 32],
            &[0x55; 32],
            6,
            &[0x66; 32],
            &[0x77; 32],
            &[0x88; 32],
            &[0x99; 32],
            &[0xaa; 32],
            1_700_000_000_000,
            1_700_000_030_000,
            &[0xbb; 32],
            &raw_acknowledgement_digest,
        );
        assert_eq!(
            hex::encode(read_digest),
            "bc595335e39a91bdaf49fc749f6df910be31385ad394089ba633bec359f47a20"
        );
    }

    #[test]
    fn fresh_read_recovers_expired_nested_ack_and_binds_its_exact_bytes() {
        let now = now_milliseconds();
        let (signing_key, configuration, acknowledgement) = signed_acknowledgement_fixture(now);
        let raw_acknowledgement = serde_json::to_string(&acknowledgement_wire(&acknowledgement))
            .expect("serialize acknowledgement");
        let request = recovery_wire(
            &acknowledgement,
            &signing_key,
            raw_acknowledgement,
            now - 1,
            now + 10_000,
        );
        let (validated, wrapper_expiry) =
            validate_recovery_request(request, &configuration).expect("fresh recovery");
        assert_eq!(validated, acknowledgement);
        assert_eq!(wrapper_expiry, now + 10_000);

        let mut tampered = recovery_wire(
            &acknowledgement,
            &signing_key,
            serde_json::to_string(&acknowledgement_wire(&acknowledgement))
                .expect("serialize acknowledgement"),
            now - 1,
            now + 10_000,
        );
        let raw = tampered.checkpoint_ack.get().replacen('{', "{ ", 1);
        tampered.checkpoint_ack =
            serde_json::value::RawValue::from_string(raw).expect("valid whitespace JSON");
        assert!(validate_recovery_request(tampered, &configuration).is_err());

        let expired_wrapper = recovery_wire(
            &acknowledgement,
            &signing_key,
            serde_json::to_string(&acknowledgement_wire(&acknowledgement))
                .expect("serialize acknowledgement"),
            now - 20_000,
            now - 10_000,
        );
        assert!(validate_recovery_request(expired_wrapper, &configuration).is_err());
    }

    #[test]
    fn acknowledgement_shape_rejects_signed_zero_audit_fields() {
        let now = now_milliseconds();
        let (signing_key, configuration, mut acknowledgement) = signed_acknowledgement_fixture(now);
        acknowledgement.committed_at_unix_ms = now - 1;
        acknowledgement.expires_at_unix_ms = now + 10_000;
        resign_acknowledgement(&mut acknowledgement, &signing_key);
        validate_acknowledgement_request(
            acknowledgement_wire(&acknowledgement),
            &configuration,
            true,
        )
        .expect("positive control");

        for mutate in [
            |value: &mut StateAnchorAcknowledgement| value.request_digest = [0; 32],
            |value: &mut StateAnchorAcknowledgement| value.nonce = [0; 32],
            |value: &mut StateAnchorAcknowledgement| value.operation_id = [0; 32],
            |value: &mut StateAnchorAcknowledgement| value.transition_digest = [0; 32],
        ] {
            let mut invalid = acknowledgement.clone();
            mutate(&mut invalid);
            resign_acknowledgement(&mut invalid, &signing_key);
            assert!(validate_acknowledgement_request(
                acknowledgement_wire(&invalid),
                &configuration,
                true,
            )
            .is_err());
        }
    }

    #[test]
    fn configured_anchor_rejects_zero_binding_hash() {
        let _guard = lock_test_state();
        let signing_key = SigningKey::from_bytes(&[0x01; 32]);
        let public_key = signing_key.verifying_key().to_bytes();
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH_ENV,
            bytes32_hex([0; 32]),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_ENV,
            bytes32_hex(public_key),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256_ENV,
            bytes32_hex(ed25519_spki_sha256(&public_key)),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS_ENV,
            "8",
        );
        let error = configured_state_anchor().expect_err("zero binding hash");
        assert!(error.to_string().contains("must be nonzero"));
    }

    #[test]
    fn configured_anchor_reserves_four_terminal_witness_records() {
        let _guard = lock_test_state();
        let signing_key = SigningKey::from_bytes(&[0x01; 32]);
        let public_key = signing_key.verifying_key().to_bytes();
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH_ENV,
            bytes32_hex([0x11; 32]),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_ENV,
            bytes32_hex(public_key),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256_ENV,
            bytes32_hex(ed25519_spki_sha256(&public_key)),
        );
        std::env::set_var(
            TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS_ENV,
            "2",
        );
        std::env::set_var(TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS_ENV, "5");
        let error = configured_state_anchor().expect_err("three records are insufficient");
        assert!(error.to_string().contains("leave four records"));

        std::env::set_var(TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS_ENV, "6");
        assert_eq!(
            configured_state_anchor()
                .expect("six records satisfy the configured reservation")
                .expect("anchor configuration")
                .rotation_threshold_records,
            2
        );
    }

    #[test]
    fn configured_anchor_rejects_non_prime_subgroup_online_and_offline_keys() {
        let _guard = lock_test_state();
        let corpus = [
            "0100000000000000000000000000000000000000000000000000000000000000",
            "0000000000000000000000000000000000000000000000000000000000000000",
            "9970c93c125fd998ebc1642abe30619e2fd971dbcbeaeb8ccfe919cbfd13b6cf",
        ];
        for weak_role_is_online in [true, false] {
            for encoded in corpus {
                establish_clean_signer_test_env();
                let weak: [u8; 32] = hex::decode(encoded)
                    .expect("vector hex")
                    .try_into()
                    .expect("32-byte vector");
                let online_key = SigningKey::from_bytes(&[0x21; 32])
                    .verifying_key()
                    .to_bytes();
                let offline_key = SigningKey::from_bytes(&[0x22; 32])
                    .verifying_key()
                    .to_bytes();
                let response = if weak_role_is_online {
                    weak
                } else {
                    online_key
                };
                let authority = if weak_role_is_online {
                    offline_key
                } else {
                    weak
                };
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_BINDING_HASH_ENV,
                    bytes32_hex([0x31; 32]),
                );
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_ENV,
                    bytes32_hex(response),
                );
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_RESPONSE_PUBLIC_KEY_SPKI_SHA256_ENV,
                    bytes32_hex(ed25519_spki_sha256(&response)),
                );
                std::env::set_var(
                    TBTC_SIGNER_STATE_WITNESS_ROTATION_THRESHOLD_RECORDS_ENV,
                    "8",
                );
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_PROTOCOL_ID_ENV,
                    bytes32_hex([0x32; 32]),
                );
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_STREAM_ID_ENV,
                    bytes32_hex([0x33; 32]),
                );
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_ACTIVATION_MANIFEST_HASH_ENV,
                    bytes32_hex([0x34; 32]),
                );
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_ACTIVATION_MANIFEST_SEQUENCE_ENV,
                    "1",
                );
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_ENV,
                    bytes32_hex(authority),
                );
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_SPKI_SHA256_ENV,
                    bytes32_hex(ed25519_spki_sha256(&authority)),
                );
                std::env::set_var(TBTC_SIGNER_STATE_ANCHOR_TRUST_CERTIFICATE_SEQUENCE_ENV, "1");
                std::env::set_var(
                    TBTC_SIGNER_STATE_ANCHOR_TRUST_CERTIFICATE_DIGEST_ENV,
                    bytes32_hex([0x35; 32]),
                );
                let error = configured_state_anchor()
                    .expect_err("non-prime-subgroup configured key rejected");
                let error_text = error.to_string();
                // The all-zero encoding is rejected either by a reserved-value
                // nonzero pre-check (offline authority) or, where it still
                // decompresses to a small-order point, by the prime-subgroup
                // check (response key). Every other vector must fail the
                // prime-subgroup check itself.
                let accepted = if weak.iter().all(|&byte| byte == 0) {
                    error_text.contains("must be nonzero") || error_text.contains("prime-subgroup")
                } else {
                    error_text.contains("prime-subgroup")
                };
                assert!(accepted, "unexpected configured-key error: {error_text}");
            }
        }
    }
}
