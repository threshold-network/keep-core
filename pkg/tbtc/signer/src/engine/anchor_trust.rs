//! Offline-authority state-anchor trust transitions.
//!
//! The activation manifest is static policy, not a self-referential trust
//! floor. A two-stage Ed25519 certificate first authorizes the immutable core,
//! then ratifies the exact successor acknowledgement. The signed transcript is
//! a fixed-width direct concatenation shared byte-for-byte with the Go anchor
//! service. Durable journal and crash-recovery mechanics live in `store`.

use super::*;

use crate::api::{AcknowledgeStateWitnessCheckpointRequest, RecoverStateWitnessCheckpointRequest};
#[cfg(test)]
use crate::api::{RequiredNullableStateAnchorTrustEndpoint, StateWitnessCheckpointRequest};
use base64ct::{Base64, Encoding};
use ed25519_dalek::{Signature, VerifyingKey};
#[cfg(test)]
use ed25519_dalek::{Signer, SigningKey};

#[allow(dead_code)]
pub(crate) const STATE_ANCHOR_TRUST_CERTIFICATE_SCHEMA: &str =
    "tbtc-frost-native-signer-state-anchor-trust-certificate/v1";
#[allow(dead_code)]
pub(crate) const STATE_ANCHOR_TRUST_TRANSITION_SCHEMA: &str =
    "tbtc-signer-state-anchor-trust-transition/v1";
#[allow(dead_code)]
pub(crate) const STATE_ANCHOR_TRUST_TRANSITION_RESULT_SCHEMA: &str =
    "tbtc-signer-state-anchor-trust-transition-result/v1";
#[allow(dead_code)]
pub(crate) const STATE_ANCHOR_TRUST_HEAD_SCHEMA: &str = "tbtc-signer-state-anchor-trust-head/v1";
#[allow(dead_code)]
pub(crate) const STATE_ANCHOR_BOOTSTRAP_FACTS_SCHEMA: &str =
    "tbtc-signer-state-anchor-bootstrap-facts/v1";

const TRUST_CORE_DOMAIN: &[u8] =
    b"tbtc-frost-native-signer-state-anchor-trust-transition-core/v1\0";
const TRUST_OPERATION_ID_DOMAIN: &[u8] =
    b"tbtc-frost-native-signer-state-anchor-trust-transition-operation-id/v1\0";
const TRUST_TRANSITION_DIGEST_DOMAIN: &[u8] =
    b"tbtc-frost-native-signer-state-anchor-trust-transition-digest/v1\0";
const TRUST_FINAL_DOMAIN: &[u8] = b"tbtc-frost-native-signer-state-anchor-trust-certificate/v1\0";
const TRUST_CERTIFICATE_DIGEST_DOMAIN: &[u8] =
    b"tbtc-frost-native-signer-state-anchor-trust-certificate-digest/v1\0";
const TRUST_JOURNAL_HEADER_DOMAIN: &[u8] = b"tbtc-signer-state-anchor-trust-journal-header/v1\0";
const TRUST_JOURNAL_RECORD_DOMAIN: &[u8] = b"tbtc-signer-state-anchor-trust-journal-record/v1\0";
const TRUST_TRANSITION_INTENT_DOMAIN: &[u8] =
    b"tbtc-signer-state-anchor-trust-transition-intent/v1\0";
const TRUST_JOURNAL_MAGIC: &[u8; 16] = b"TBTCTRUSTJOURN1\0";
const TRUST_JOURNAL_VERSION: u32 = 1;
pub(crate) const STATE_ANCHOR_TRUST_JOURNAL_HEADER_LENGTH: usize = 84;
const TRUST_JOURNAL_RECORD_PREPARE: u8 = 1;
const TRUST_JOURNAL_RECORD_COMMIT: u8 = 2;
const TRUST_JOURNAL_RECORD_FIXED_LENGTH: usize = 116;
pub(crate) const STATE_ANCHOR_TRUST_MAX_RECORD_LENGTH: usize = 128 * 1024;
pub(crate) const STATE_ANCHOR_TRUST_MAX_CERTIFICATE_JSON_LENGTH: usize = 120 * 1024;
pub(crate) const STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH: usize = 256 * 1024 * 1024;
const TRUST_INTENT_MAGIC: &[u8; 16] = b"TBTCTRUSTINTNT1\0";
const TRUST_INTENT_VERSION: u32 = 1;
const TRUST_INTENT_HEADER_LENGTH: usize = 56;
const TRUST_INTENT_TRAILER_LENGTH: usize = 32;
pub(crate) const STATE_ANCHOR_TRUST_MAX_INTENT_LENGTH: usize = 16 * 1024 * 1024;

pub(crate) const STATE_ANCHOR_TRUST_MAX_CERTIFICATES_PER_REQUEST: usize = 64;
pub(crate) const STATE_ANCHOR_TRUST_MAX_ACKNOWLEDGEMENT_BYTES: usize = 64 * 1024;
pub(crate) const STATE_ANCHOR_TRUST_MAX_READ_RESPONSE_BYTES: usize = 128 * 1024;
pub(crate) const STATE_ANCHOR_TRUST_MAX_REVISION_DISTANCE: u64 = 4_096;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum StateAnchorTrustCertificateKind {
    Bootstrap,
    Rotation,
}

impl StateAnchorTrustCertificateKind {
    fn parse(value: &str) -> Result<Self, EngineError> {
        match value {
            "bootstrap" => Ok(Self::Bootstrap),
            "rotation" => Ok(Self::Rotation),
            _ => Err(EngineError::Validation(
                "state-anchor trust certificate kind must be 'bootstrap' or 'rotation'".to_string(),
            )),
        }
    }

    fn transcript_byte(self) -> u8 {
        match self {
            Self::Bootstrap => 1,
            Self::Rotation => 2,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct StateAnchorTrustCheckpointModel {
    pub(crate) store_fingerprint: [u8; 32],
    pub(crate) generation: u64,
    pub(crate) previous_state_commitment: [u8; 32],
    pub(crate) state_image_digest: [u8; 32],
    pub(crate) state_commitment: [u8; 32],
}

impl StateAnchorTrustCheckpointModel {
    #[allow(dead_code)]
    pub(crate) fn from_witness(store_fingerprint: [u8; 32], witness: &StateWitness) -> Self {
        Self {
            store_fingerprint,
            generation: witness.generation,
            previous_state_commitment: witness.previous_commitment,
            state_image_digest: witness.state_image_digest,
            state_commitment: witness.commitment,
        }
    }

    #[allow(dead_code)]
    pub(crate) fn to_wire(&self) -> StateAnchorTrustCheckpoint {
        StateAnchorTrustCheckpoint {
            store_fingerprint: bytes32_hex(self.store_fingerprint),
            generation: self.generation.to_string(),
            previous_state_commitment: bytes32_hex(self.previous_state_commitment),
            state_image_digest: bytes32_hex(self.state_image_digest),
            state_commitment: bytes32_hex(self.state_commitment),
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct StateAnchorTrustReferenceModel {
    pub(crate) service_epoch: u64,
    pub(crate) revision: u64,
    pub(crate) previous_event_root: [u8; 32],
    pub(crate) event_root: [u8; 32],
    pub(crate) checkpoint_ack_digest: [u8; 32],
    pub(crate) checkpoint: StateAnchorTrustCheckpointModel,
}

impl StateAnchorTrustReferenceModel {
    pub(crate) fn from_acknowledgement(ack: &StateAnchorAcknowledgement) -> Self {
        Self {
            service_epoch: ack.service_epoch,
            revision: ack.revision,
            previous_event_root: ack.previous_event_root,
            event_root: ack.event_root,
            checkpoint_ack_digest: ack.acknowledgement_digest,
            checkpoint: StateAnchorTrustCheckpointModel {
                store_fingerprint: ack.checkpoint_store_fingerprint,
                generation: ack.checkpoint_generation,
                previous_state_commitment: ack.checkpoint_previous_commitment,
                state_image_digest: ack.checkpoint_state_image_digest,
                state_commitment: ack.checkpoint_state_commitment,
            },
        }
    }

    #[allow(dead_code)]
    pub(crate) fn to_wire(&self) -> StateAnchorTrustReference {
        StateAnchorTrustReference {
            service_epoch: self.service_epoch.to_string(),
            revision: self.revision.to_string(),
            previous_event_root: bytes32_hex(self.previous_event_root),
            event_root: bytes32_hex(self.event_root),
            checkpoint_ack_digest: bytes32_hex(self.checkpoint_ack_digest),
            checkpoint: self.checkpoint.to_wire(),
        }
    }

    pub(crate) fn matches_acknowledgement(&self, ack: &StateAnchorAcknowledgement) -> bool {
        self == &Self::from_acknowledgement(ack)
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct StateAnchorTrustEndpointModel {
    pub(crate) activation_manifest_hash: [u8; 32],
    pub(crate) activation_manifest_sequence: u64,
    pub(crate) binding_hash: [u8; 32],
    pub(crate) response_public_key: [u8; 32],
    pub(crate) response_public_key_spki_sha256: [u8; 32],
    pub(crate) offline_authority_public_key: [u8; 32],
    pub(crate) offline_authority_spki_sha256: [u8; 32],
    pub(crate) witness_maximum_records: u64,
    pub(crate) witness_rotation_threshold_records: u64,
    pub(crate) reference: StateAnchorTrustReferenceModel,
}

impl StateAnchorTrustEndpointModel {
    pub(crate) fn anchor_configuration(&self) -> Result<StateAnchorConfiguration, EngineError> {
        Ok(StateAnchorConfiguration {
            binding_hash: self.binding_hash,
            response_public_key: self.response_public_key,
            response_public_key_spki_sha256: self.response_public_key_spki_sha256,
            rotation_threshold_records: usize::try_from(self.witness_rotation_threshold_records)
                .map_err(|_| {
                    EngineError::Validation(
                        "certificate witnessRotationThresholdRecords does not fit this platform"
                            .to_string(),
                    )
                })?,
            trust: None,
        })
    }
}

#[allow(dead_code)]
#[derive(Clone, Debug)]
pub(crate) struct VerifiedStateAnchorTrustCertificate {
    pub(crate) wire: StateAnchorTrustCertificate,
    pub(crate) kind: StateAnchorTrustCertificateKind,
    pub(crate) certificate_sequence: u64,
    pub(crate) previous_certificate_digest: [u8; 32],
    pub(crate) protocol_id: [u8; 32],
    pub(crate) stream_id: [u8; 32],
    pub(crate) signer_store_fingerprint: [u8; 32],
    pub(crate) from: Option<StateAnchorTrustEndpointModel>,
    pub(crate) to: StateAnchorTrustEndpointModel,
    pub(crate) core_digest: [u8; 32],
    pub(crate) core_signature: [u8; 64],
    pub(crate) operation_id: [u8; 32],
    pub(crate) transition_digest: [u8; 32],
    pub(crate) target_acknowledgement_bytes: Vec<u8>,
    pub(crate) target_acknowledgement_sha256: [u8; 32],
    pub(crate) target_acknowledgement: StateAnchorAcknowledgement,
    pub(crate) final_signature: [u8; 64],
    pub(crate) certificate_digest: [u8; 32],
}

#[derive(Clone, Debug)]
pub(crate) struct VerifiedStateAnchorTrustTransition {
    pub(crate) request: TransitionStateWitnessAnchorRequest,
    pub(crate) certificates: Vec<VerifiedStateAnchorTrustCertificate>,
    pub(crate) target_read_acknowledgement_bytes: Vec<u8>,
    pub(crate) target_read_acknowledgement: StateAnchorAcknowledgement,
    pub(crate) target_read_expires_at_unix_ms: u64,
}

#[derive(Clone, Debug)]
pub(crate) struct StateAnchorTrustJournalModel {
    pub(crate) committed: Vec<VerifiedStateAnchorTrustCertificate>,
    pub(crate) pending: Vec<VerifiedStateAnchorTrustCertificate>,
    pub(crate) last_record_commitment: [u8; 32],
}

impl StateAnchorTrustJournalModel {
    pub(crate) fn head(&self) -> Option<&VerifiedStateAnchorTrustCertificate> {
        self.committed.last()
    }

    pub(crate) fn certified_floors(&self) -> Vec<StateAnchorTrustReferenceModel> {
        self.committed
            .iter()
            .chain(self.pending.iter())
            .map(|certificate| certificate.to.reference.clone())
            .collect()
    }
}

pub(crate) fn validate_state_anchor_trust_journal_head(
    journal: &StateAnchorTrustJournalModel,
    configuration: &StateAnchorConfiguration,
    store_fingerprint: &[u8; 32],
) -> Result<(), EngineError> {
    if !journal.pending.is_empty() {
        return Err(EngineError::Internal(
            "state-anchor trust journal has PREPARE records without a durable transition intent"
                .to_string(),
        ));
    }
    let trust = configuration.trust.as_ref().ok_or_else(|| {
        EngineError::Internal(
            "state-anchor trust journal exists without complete installed trust pins".to_string(),
        )
    })?;
    let head = journal.head().ok_or_else(|| {
        EngineError::Internal("state-anchor trust journal has no committed certificate".to_string())
    })?;
    let to = &head.to;
    if head.certificate_sequence != trust.certificate_sequence
        || head.certificate_digest != trust.certificate_digest
        || head.protocol_id != trust.protocol_id
        || head.stream_id != trust.stream_id
        || head.signer_store_fingerprint != *store_fingerprint
        || to.reference.checkpoint.store_fingerprint != *store_fingerprint
        || to.activation_manifest_hash != trust.activation_manifest_hash
        || to.activation_manifest_sequence != trust.activation_manifest_sequence
        || to.binding_hash != configuration.binding_hash
        || to.response_public_key != configuration.response_public_key
        || to.response_public_key_spki_sha256 != configuration.response_public_key_spki_sha256
        || to.offline_authority_public_key != trust.offline_authority_public_key
        || to.offline_authority_spki_sha256 != trust.offline_authority_public_key_spki_sha256
        || to.witness_maximum_records != state_witness_max_records()? as u64
        || to.witness_rotation_threshold_records != configuration.rotation_threshold_records as u64
    {
        return Err(EngineError::Internal(
            "durable state-anchor trust head does not exactly match installed config pins"
                .to_string(),
        ));
    }
    Ok(())
}

/// Authenticates a pre-transition journal under only the pins that are
/// immutable across an authorized service rotation. The installed config
/// already describes the target endpoint at this point, so requiring its
/// rotating manifest, binding, response key, sequence, or digest here would
/// make it impossible to read the prior head and select the missing suffix.
pub(crate) fn validate_state_anchor_trust_journal_stable_pins(
    journal: &StateAnchorTrustJournalModel,
    target_configuration: &StateAnchorConfiguration,
    store_fingerprint: &[u8; 32],
) -> Result<(), EngineError> {
    if !journal.pending.is_empty() {
        return Err(EngineError::Internal(
            "state-anchor trust journal has PREPARE records without recovery intent".to_string(),
        ));
    }
    let target_trust = target_configuration.trust.as_ref().ok_or_else(|| {
        EngineError::Internal(
            "state-anchor trust-head inspection requires complete installed trust pins".to_string(),
        )
    })?;
    let head = journal.head().ok_or_else(|| {
        EngineError::Internal("state-anchor trust journal has no committed certificate".to_string())
    })?;
    if head.protocol_id != target_trust.protocol_id
        || head.stream_id != target_trust.stream_id
        || head.signer_store_fingerprint != *store_fingerprint
        || head.to.reference.checkpoint.store_fingerprint != *store_fingerprint
        || head.to.offline_authority_public_key != target_trust.offline_authority_public_key
        || head.to.offline_authority_spki_sha256
            != target_trust.offline_authority_public_key_spki_sha256
        || head.to.witness_maximum_records != state_witness_max_records()? as u64
        || head.to.witness_rotation_threshold_records
            != target_configuration.rotation_threshold_records as u64
    {
        return Err(EngineError::Internal(
            "durable state-anchor trust head violates installed immutable trust pins".to_string(),
        ));
    }
    Ok(())
}

pub(crate) fn encode_state_anchor_trust_journal_header(store_fingerprint: &[u8; 32]) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(STATE_ANCHOR_TRUST_JOURNAL_HEADER_LENGTH);
    bytes.extend_from_slice(TRUST_JOURNAL_MAGIC);
    bytes.extend_from_slice(&TRUST_JOURNAL_VERSION.to_be_bytes());
    bytes.extend_from_slice(store_fingerprint);
    let mut digest = Sha256::new();
    digest.update(TRUST_JOURNAL_HEADER_DOMAIN);
    digest.update(&bytes);
    bytes.extend_from_slice(&<[u8; 32]>::from(digest.finalize()));
    debug_assert_eq!(bytes.len(), STATE_ANCHOR_TRUST_JOURNAL_HEADER_LENGTH);
    bytes
}

pub(crate) fn parse_state_anchor_trust_journal(
    bytes: &[u8],
    expected_store_fingerprint: &[u8; 32],
) -> Result<StateAnchorTrustJournalModel, EngineError> {
    if bytes.len() < STATE_ANCHOR_TRUST_JOURNAL_HEADER_LENGTH
        || bytes.len() > STATE_ANCHOR_TRUST_MAX_JOURNAL_LENGTH
    {
        return Err(EngineError::Internal(format!(
            "state-anchor trust journal length [{}] is outside its durable bounds",
            bytes.len()
        )));
    }
    if &bytes[..16] != TRUST_JOURNAL_MAGIC
        || u32::from_be_bytes(
            bytes[16..20]
                .try_into()
                .expect("fixed trust journal version"),
        ) != TRUST_JOURNAL_VERSION
        || &bytes[20..52] != expected_store_fingerprint
    {
        return Err(EngineError::Internal(
            "state-anchor trust journal header or store fingerprint is invalid".to_string(),
        ));
    }
    let mut header_digest = Sha256::new();
    header_digest.update(TRUST_JOURNAL_HEADER_DOMAIN);
    header_digest.update(&bytes[..52]);
    let expected_header_digest: [u8; 32] = header_digest.finalize().into();
    if bytes[52..84] != expected_header_digest {
        return Err(EngineError::Internal(
            "state-anchor trust journal header commitment is invalid".to_string(),
        ));
    }

    let mut offset = STATE_ANCHOR_TRUST_JOURNAL_HEADER_LENGTH;
    let mut previous_record_commitment = expected_header_digest;
    let mut committed: Vec<VerifiedStateAnchorTrustCertificate> = Vec::new();
    let mut pending: Vec<VerifiedStateAnchorTrustCertificate> = Vec::new();
    let mut next_commit_index = 0usize;
    while offset < bytes.len() {
        if bytes.len() - offset < 4 {
            return Err(EngineError::Internal(
                "state-anchor trust journal has a truncated record length".to_string(),
            ));
        }
        let record_length = u32::from_be_bytes(
            bytes[offset..offset + 4]
                .try_into()
                .expect("fixed record length"),
        ) as usize;
        if !(TRUST_JOURNAL_RECORD_FIXED_LENGTH..=STATE_ANCHOR_TRUST_MAX_RECORD_LENGTH)
            .contains(&record_length)
            || offset
                .checked_add(record_length)
                .is_none_or(|end| end > bytes.len())
        {
            return Err(EngineError::Internal(
                "state-anchor trust journal record length is invalid or truncated".to_string(),
            ));
        }
        let record = &bytes[offset..offset + record_length];
        let kind = record[4];
        if record[5..8] != [0u8; 3] {
            return Err(EngineError::Internal(
                "state-anchor trust journal record reserved bytes are nonzero".to_string(),
            ));
        }
        let sequence = u64::from_be_bytes(record[8..16].try_into().expect("fixed record sequence"));
        if record[16..48] != previous_record_commitment {
            return Err(EngineError::Internal(
                "state-anchor trust journal record chain is invalid".to_string(),
            ));
        }
        let payload_length =
            u32::from_be_bytes(record[48..52].try_into().expect("fixed payload length")) as usize;
        if TRUST_JOURNAL_RECORD_FIXED_LENGTH
            .checked_add(payload_length)
            .is_none_or(|expected| expected != record_length)
        {
            return Err(EngineError::Internal(
                "state-anchor trust journal payload length is invalid".to_string(),
            ));
        }
        let payload_end = 52 + payload_length;
        let payload = &record[52..payload_end];
        let payload_digest: [u8; 32] = record[payload_end..payload_end + 32]
            .try_into()
            .expect("fixed payload digest");
        if <[u8; 32]>::from(Sha256::digest(payload)) != payload_digest {
            return Err(EngineError::Internal(
                "state-anchor trust journal payload digest is invalid".to_string(),
            ));
        }
        let record_commitment: [u8; 32] = record[payload_end + 32..payload_end + 64]
            .try_into()
            .expect("fixed record commitment");
        let expected_record_commitment = state_anchor_trust_record_commitment(
            expected_store_fingerprint,
            kind,
            sequence,
            &previous_record_commitment,
            &payload_digest,
        );
        if record_commitment != expected_record_commitment {
            return Err(EngineError::Internal(
                "state-anchor trust journal record commitment is invalid".to_string(),
            ));
        }

        match kind {
            TRUST_JOURNAL_RECORD_PREPARE => {
                if next_commit_index != 0 {
                    return Err(EngineError::Internal(
                        "state-anchor trust journal interleaves PREPARE after COMMIT".to_string(),
                    ));
                }
                let wire: StateAnchorTrustCertificate =
                    serde_json::from_slice(payload).map_err(|error| {
                        EngineError::Internal(format!(
                            "state-anchor trust journal certificate is invalid JSON: {error}"
                        ))
                    })?;
                let certificate = verify_state_anchor_trust_certificate(wire).map_err(|error| {
                    EngineError::Internal(format!(
                        "state-anchor trust journal certificate is invalid: {error}"
                    ))
                })?;
                let expected_sequence = committed
                    .last()
                    .map(|head| head.certificate_sequence)
                    .unwrap_or(0)
                    .checked_add(pending.len() as u64)
                    .and_then(|value| value.checked_add(1))
                    .ok_or_else(|| {
                        EngineError::Internal(
                            "state-anchor trust journal sequence overflows u64".to_string(),
                        )
                    })?;
                if sequence != expected_sequence
                    || certificate.certificate_sequence != expected_sequence
                {
                    return Err(EngineError::Internal(
                        "state-anchor trust journal PREPARE sequence is invalid".to_string(),
                    ));
                }
                if let Some(previous) = pending.last().or_else(|| committed.last()) {
                    validate_certificate_link(previous, &certificate).map_err(|error| {
                        EngineError::Internal(format!(
                            "state-anchor trust journal PREPARE link is invalid: {error}"
                        ))
                    })?;
                } else if certificate.certificate_sequence != 1 {
                    return Err(EngineError::Internal(
                        "state-anchor trust journal must begin at certificate sequence 1"
                            .to_string(),
                    ));
                }
                pending.push(certificate);
            }
            TRUST_JOURNAL_RECORD_COMMIT => {
                let pending_certificate = pending.get(next_commit_index).ok_or_else(|| {
                    EngineError::Internal(
                        "state-anchor trust journal COMMIT has no matching PREPARE".to_string(),
                    )
                })?;
                if sequence != pending_certificate.certificate_sequence
                    || payload != pending_certificate.certificate_digest
                {
                    return Err(EngineError::Internal(
                        "state-anchor trust journal COMMIT differs from its PREPARE".to_string(),
                    ));
                }
                next_commit_index += 1;
                if next_commit_index == pending.len() {
                    committed.append(&mut pending);
                    next_commit_index = 0;
                }
            }
            _ => {
                return Err(EngineError::Internal(
                    "state-anchor trust journal record type is invalid".to_string(),
                ))
            }
        }
        previous_record_commitment = record_commitment;
        offset += record_length;
    }
    if next_commit_index != 0 {
        // A power loss between copy-on-write COMMIT publications leaves a
        // complete authenticated prefix. Keep the committed prefix active only
        // for transition recovery; the nonempty `pending` tail makes every
        // ordinary open fail closed until the durable intent completes it.
        let remaining = pending.split_off(next_commit_index);
        committed.append(&mut pending);
        pending = remaining;
    }
    Ok(StateAnchorTrustJournalModel {
        committed,
        pending,
        last_record_commitment: previous_record_commitment,
    })
}

pub(crate) fn encode_state_anchor_trust_prepare_record(
    store_fingerprint: &[u8; 32],
    previous_record_commitment: &[u8; 32],
    certificate: &VerifiedStateAnchorTrustCertificate,
) -> Result<Vec<u8>, EngineError> {
    let payload = serde_json::to_vec(&certificate.wire).map_err(|error| {
        EngineError::Internal(format!(
            "failed to encode verified state-anchor trust certificate: {error}"
        ))
    })?;
    encode_state_anchor_trust_record(
        store_fingerprint,
        TRUST_JOURNAL_RECORD_PREPARE,
        certificate.certificate_sequence,
        previous_record_commitment,
        &payload,
    )
}

pub(crate) fn encode_state_anchor_trust_commit_record(
    store_fingerprint: &[u8; 32],
    previous_record_commitment: &[u8; 32],
    certificate: &VerifiedStateAnchorTrustCertificate,
) -> Result<Vec<u8>, EngineError> {
    encode_state_anchor_trust_record(
        store_fingerprint,
        TRUST_JOURNAL_RECORD_COMMIT,
        certificate.certificate_sequence,
        previous_record_commitment,
        &certificate.certificate_digest,
    )
}

fn encode_state_anchor_trust_record(
    store_fingerprint: &[u8; 32],
    kind: u8,
    sequence: u64,
    previous_record_commitment: &[u8; 32],
    payload: &[u8],
) -> Result<Vec<u8>, EngineError> {
    let record_length = TRUST_JOURNAL_RECORD_FIXED_LENGTH
        .checked_add(payload.len())
        .ok_or_else(|| {
            EngineError::Internal("state-anchor trust record length overflowed".to_string())
        })?;
    if record_length > STATE_ANCHOR_TRUST_MAX_RECORD_LENGTH {
        return Err(EngineError::Validation(
            "state-anchor trust certificate exceeds the durable record bound".to_string(),
        ));
    }
    let mut record = Vec::with_capacity(record_length);
    record.extend_from_slice(&(record_length as u32).to_be_bytes());
    record.push(kind);
    record.extend_from_slice(&[0u8; 3]);
    record.extend_from_slice(&sequence.to_be_bytes());
    record.extend_from_slice(previous_record_commitment);
    record.extend_from_slice(&(payload.len() as u32).to_be_bytes());
    record.extend_from_slice(payload);
    let payload_digest: [u8; 32] = Sha256::digest(payload).into();
    record.extend_from_slice(&payload_digest);
    record.extend_from_slice(&state_anchor_trust_record_commitment(
        store_fingerprint,
        kind,
        sequence,
        previous_record_commitment,
        &payload_digest,
    ));
    debug_assert_eq!(record.len(), record_length);
    Ok(record)
}

fn state_anchor_trust_record_commitment(
    store_fingerprint: &[u8; 32],
    kind: u8,
    sequence: u64,
    previous_record_commitment: &[u8; 32],
    payload_digest: &[u8; 32],
) -> [u8; 32] {
    hash_direct(
        TRUST_JOURNAL_RECORD_DOMAIN,
        &[
            store_fingerprint,
            &[kind],
            &sequence.to_be_bytes(),
            previous_record_commitment,
            payload_digest,
        ],
    )
}

#[allow(dead_code)]
pub(crate) fn encode_state_anchor_trust_transition_intent(
    store_fingerprint: &[u8; 32],
    request: &TransitionStateWitnessAnchorRequest,
) -> Result<Vec<u8>, EngineError> {
    let payload = serde_json::to_vec(request).map_err(|error| {
        EngineError::Internal(format!(
            "failed to encode state-anchor trust transition intent: {error}"
        ))
    })?;
    let total_length = TRUST_INTENT_HEADER_LENGTH
        .checked_add(payload.len())
        .and_then(|value| value.checked_add(TRUST_INTENT_TRAILER_LENGTH))
        .ok_or_else(|| {
            EngineError::Internal("state-anchor trust intent length overflowed".to_string())
        })?;
    if total_length > STATE_ANCHOR_TRUST_MAX_INTENT_LENGTH {
        return Err(EngineError::Validation(
            "state-anchor trust transition intent exceeds its durable bound".to_string(),
        ));
    }
    let mut bytes = Vec::with_capacity(total_length);
    bytes.extend_from_slice(TRUST_INTENT_MAGIC);
    bytes.extend_from_slice(&TRUST_INTENT_VERSION.to_be_bytes());
    bytes.extend_from_slice(store_fingerprint);
    bytes.extend_from_slice(&(payload.len() as u32).to_be_bytes());
    bytes.extend_from_slice(payload.as_slice());
    let mut digest = Sha256::new();
    digest.update(TRUST_TRANSITION_INTENT_DOMAIN);
    digest.update(&bytes);
    bytes.extend_from_slice(&<[u8; 32]>::from(digest.finalize()));
    Ok(bytes)
}

pub(crate) fn parse_state_anchor_trust_transition_intent(
    bytes: &[u8],
    expected_store_fingerprint: &[u8; 32],
) -> Result<TransitionStateWitnessAnchorRequest, EngineError> {
    if bytes.len() < TRUST_INTENT_HEADER_LENGTH + TRUST_INTENT_TRAILER_LENGTH
        || bytes.len() > STATE_ANCHOR_TRUST_MAX_INTENT_LENGTH
    {
        return Err(EngineError::Internal(
            "state-anchor trust transition intent length is invalid".to_string(),
        ));
    }
    if &bytes[..16] != TRUST_INTENT_MAGIC
        || u32::from_be_bytes(bytes[16..20].try_into().expect("fixed intent version"))
            != TRUST_INTENT_VERSION
        || &bytes[20..52] != expected_store_fingerprint
    {
        return Err(EngineError::Internal(
            "state-anchor trust transition intent header is invalid".to_string(),
        ));
    }
    let payload_length = u32::from_be_bytes(
        bytes[52..56]
            .try_into()
            .expect("fixed intent payload length"),
    ) as usize;
    if TRUST_INTENT_HEADER_LENGTH
        .checked_add(payload_length)
        .and_then(|value| value.checked_add(TRUST_INTENT_TRAILER_LENGTH))
        != Some(bytes.len())
    {
        return Err(EngineError::Internal(
            "state-anchor trust transition intent payload length is invalid".to_string(),
        ));
    }
    let commitment_offset = bytes.len() - TRUST_INTENT_TRAILER_LENGTH;
    let mut digest = Sha256::new();
    digest.update(TRUST_TRANSITION_INTENT_DOMAIN);
    digest.update(&bytes[..commitment_offset]);
    if bytes[commitment_offset..] != <[u8; 32]>::from(digest.finalize()) {
        return Err(EngineError::Internal(
            "state-anchor trust transition intent commitment is invalid".to_string(),
        ));
    }
    serde_json::from_slice(&bytes[TRUST_INTENT_HEADER_LENGTH..commitment_offset]).map_err(|error| {
        EngineError::Internal(format!(
            "state-anchor trust transition intent request is invalid: {error}"
        ))
    })
}

pub(crate) fn verify_state_anchor_trust_certificate(
    wire: StateAnchorTrustCertificate,
) -> Result<VerifiedStateAnchorTrustCertificate, EngineError> {
    let canonical_length = serde_json::to_vec(&wire)
        .map_err(|error| {
            EngineError::Validation(format!(
                "state-anchor trust certificate cannot be canonically encoded: {error}"
            ))
        })?
        .len();
    if canonical_length > STATE_ANCHOR_TRUST_MAX_CERTIFICATE_JSON_LENGTH {
        return Err(EngineError::Validation(format!(
            "canonical state-anchor trust certificate exceeds {} bytes",
            STATE_ANCHOR_TRUST_MAX_CERTIFICATE_JSON_LENGTH
        )));
    }
    if wire.schema != STATE_ANCHOR_TRUST_CERTIFICATE_SCHEMA {
        return Err(EngineError::Validation(format!(
            "state-anchor trust certificate schema must be [{STATE_ANCHOR_TRUST_CERTIFICATE_SCHEMA}]"
        )));
    }

    let kind = StateAnchorTrustCertificateKind::parse(&wire.kind)?;
    let certificate_sequence =
        parse_canonical_u64(&wire.certificate_sequence, "certificateSequence")?;
    if certificate_sequence == 0 {
        return Err(EngineError::Validation(
            "certificateSequence must be nonzero".to_string(),
        ));
    }
    let previous_certificate_digest = parse_canonical_bytes32(
        &wire.previous_certificate_digest,
        "previousCertificateDigest",
    )?;
    let protocol_id = parse_nonzero_bytes32(&wire.protocol_id, "protocolID")?;
    let stream_id = parse_nonzero_bytes32(&wire.stream_id, "streamID")?;
    let signer_store_fingerprint =
        parse_nonzero_bytes32(&wire.signer_store_fingerprint, "signerStoreFingerprint")?;
    let from = wire
        .from
        .0
        .as_ref()
        .map(|endpoint| parse_endpoint(endpoint, "from"))
        .transpose()?;
    let to = parse_endpoint(&wire.to, "to")?;
    if to.reference.checkpoint.store_fingerprint != signer_store_fingerprint
        || from.as_ref().is_some_and(|endpoint| {
            endpoint.reference.checkpoint.store_fingerprint != signer_store_fingerprint
        })
    {
        return Err(EngineError::Validation(
            "state-anchor trust certificate endpoint checkpoints must target \
             signerStoreFingerprint"
                .to_string(),
        ));
    }

    match (kind, from.as_ref()) {
        (StateAnchorTrustCertificateKind::Bootstrap, None) => {
            if certificate_sequence != 1 || previous_certificate_digest != [0u8; 32] {
                return Err(EngineError::Validation(
                    "bootstrap certificate must use sequence 1 and a zero previous digest"
                        .to_string(),
                ));
            }
            if to.reference.service_epoch != 1
                || to.reference.revision != 1
                || to.reference.previous_event_root != [0u8; 32]
            {
                return Err(EngineError::Validation(
                    "bootstrap target must be epoch 1 revision 1 with a zero previous event root"
                        .to_string(),
                ));
            }
        }
        (StateAnchorTrustCertificateKind::Rotation, Some(from)) => {
            if certificate_sequence == 1 {
                if previous_certificate_digest != [0u8; 32] {
                    return Err(EngineError::Validation(
                        "sequence-1 legacy adoption must use a zero previous certificate digest"
                            .to_string(),
                    ));
                }
            } else if previous_certificate_digest == [0u8; 32] {
                return Err(EngineError::Validation(
                    "rotation certificate after sequence 1 must chain a nonzero previous digest"
                        .to_string(),
                ));
            }
            validate_rotation_endpoints(from, &to)?;
        }
        (StateAnchorTrustCertificateKind::Bootstrap, Some(_)) => {
            return Err(EngineError::Validation(
                "bootstrap certificate from must be explicit null".to_string(),
            ))
        }
        (StateAnchorTrustCertificateKind::Rotation, None) => {
            return Err(EngineError::Validation(
                "rotation certificate requires a non-null from endpoint".to_string(),
            ))
        }
    }

    let core_digest = trust_certificate_core_digest(
        kind,
        certificate_sequence,
        &previous_certificate_digest,
        &protocol_id,
        &stream_id,
        &signer_store_fingerprint,
        from.as_ref(),
        &to,
    );
    require_declared_digest(&wire.core_digest, "coreDigest", &core_digest)?;
    let core_signature = parse_canonical_base64_signature(&wire.core_signature, "coreSignature")?;
    let authority_public_key = from
        .as_ref()
        .map(|endpoint| endpoint.offline_authority_public_key)
        .unwrap_or(to.offline_authority_public_key);
    verify_ed25519_signature(
        &authority_public_key,
        &core_digest,
        &core_signature,
        "state-anchor trust core signature",
    )?;

    let operation_id = hash_direct(TRUST_OPERATION_ID_DOMAIN, &[&core_digest]);
    require_declared_digest(&wire.operation_id, "operationID", &operation_id)?;
    if operation_id == [0u8; 32] {
        return Err(EngineError::Validation(
            "derived state-anchor trust operationID is zero".to_string(),
        ));
    }
    let transition_digest = hash_direct(
        TRUST_TRANSITION_DIGEST_DOMAIN,
        &[&core_digest, &operation_id],
    );
    require_declared_digest(
        &wire.transition_digest,
        "transitionDigest",
        &transition_digest,
    )?;
    if transition_digest == [0u8; 32] {
        return Err(EngineError::Validation(
            "derived state-anchor trust transitionDigest is zero".to_string(),
        ));
    }

    let target_acknowledgement_bytes = decode_canonical_base64(
        &wire.target_acknowledgement_base64,
        STATE_ANCHOR_TRUST_MAX_ACKNOWLEDGEMENT_BYTES,
        "targetAcknowledgementBase64",
    )?;
    std::str::from_utf8(&target_acknowledgement_bytes).map_err(|_| {
        EngineError::Validation(
            "targetAcknowledgementBase64 must decode to UTF-8 JSON bytes".to_string(),
        )
    })?;
    let target_acknowledgement_sha256: [u8; 32] =
        Sha256::digest(&target_acknowledgement_bytes).into();
    require_declared_digest(
        &wire.target_acknowledgement_sha256,
        "targetAcknowledgementSHA256",
        &target_acknowledgement_sha256,
    )?;
    let target_ack_wire: AcknowledgeStateWitnessCheckpointRequest =
        serde_json::from_slice(&target_acknowledgement_bytes).map_err(|error| {
            EngineError::Validation(format!(
                "targetAcknowledgementBase64 contains invalid acknowledgement JSON: {error}"
            ))
        })?;
    let target_configuration = to.anchor_configuration()?;
    let target_acknowledgement = validate_certified_transition_acknowledgement(
        target_ack_wire,
        &target_configuration,
        to.reference.previous_event_root,
    )?;
    if target_acknowledgement.status != 1
        || target_acknowledgement.operation_id != operation_id
        || target_acknowledgement.transition_digest != transition_digest
        || !to
            .reference
            .matches_acknowledgement(&target_acknowledgement)
    {
        return Err(EngineError::Validation(
            "certified target acknowledgement differs from the exact target reference".to_string(),
        ));
    }

    let final_digest = trust_certificate_final_digest(
        &core_digest,
        &core_signature,
        &operation_id,
        &transition_digest,
        &to.reference,
        &target_acknowledgement_sha256,
    );
    let final_signature =
        parse_canonical_base64_signature(&wire.final_signature, "finalSignature")?;
    verify_ed25519_signature(
        &authority_public_key,
        &final_digest,
        &final_signature,
        "state-anchor trust final signature",
    )?;
    let certificate_digest = hash_direct(
        TRUST_CERTIFICATE_DIGEST_DOMAIN,
        &[
            &final_digest,
            &final_signature,
            &to.offline_authority_spki_sha256,
        ],
    );
    require_declared_digest(
        &wire.certificate_digest,
        "certificateDigest",
        &certificate_digest,
    )?;
    if certificate_digest == [0u8; 32] {
        return Err(EngineError::Validation(
            "derived state-anchor trust certificateDigest is zero".to_string(),
        ));
    }

    Ok(VerifiedStateAnchorTrustCertificate {
        wire,
        kind,
        certificate_sequence,
        previous_certificate_digest,
        protocol_id,
        stream_id,
        signer_store_fingerprint,
        from,
        to,
        core_digest,
        core_signature,
        operation_id,
        transition_digest,
        target_acknowledgement_bytes,
        target_acknowledgement_sha256,
        target_acknowledgement,
        final_signature,
        certificate_digest,
    })
}

pub(crate) fn verify_state_anchor_trust_transition_request(
    request: TransitionStateWitnessAnchorRequest,
    require_fresh_read: bool,
) -> Result<VerifiedStateAnchorTrustTransition, EngineError> {
    let canonical_request_length = serde_json::to_vec(&request)
        .map_err(|error| {
            EngineError::Validation(format!(
                "state-anchor trust transition cannot be canonically encoded: {error}"
            ))
        })?
        .len();
    if canonical_request_length
        .checked_add(TRUST_INTENT_HEADER_LENGTH + TRUST_INTENT_TRAILER_LENGTH)
        .is_none_or(|intent_length| intent_length > STATE_ANCHOR_TRUST_MAX_INTENT_LENGTH)
    {
        return Err(EngineError::Validation(
            "state-anchor trust transition exceeds the 16 MiB durable request bound".to_string(),
        ));
    }
    if request.schema != STATE_ANCHOR_TRUST_TRANSITION_SCHEMA {
        return Err(EngineError::Validation(format!(
            "state-anchor trust transition schema must be [{STATE_ANCHOR_TRUST_TRANSITION_SCHEMA}]"
        )));
    }
    if request.certificate_chain.is_empty()
        || request.certificate_chain.len() > STATE_ANCHOR_TRUST_MAX_CERTIFICATES_PER_REQUEST
    {
        return Err(EngineError::Validation(format!(
            "certificateChain must contain between 1 and {} certificates",
            STATE_ANCHOR_TRUST_MAX_CERTIFICATES_PER_REQUEST
        )));
    }
    let mut certificates = Vec::with_capacity(request.certificate_chain.len());
    for wire in request.certificate_chain.iter().cloned() {
        let certificate = verify_state_anchor_trust_certificate(wire)?;
        if let Some(previous) = certificates.last() {
            validate_certificate_link(previous, &certificate)?;
        }
        certificates.push(certificate);
    }
    let final_certificate = certificates
        .last()
        .expect("nonempty certificate chain checked above");
    validate_transition_chain_against_installed_config(&certificates)?;

    let target_read_response_bytes = decode_canonical_base64(
        &request.target_read_response_base64,
        STATE_ANCHOR_TRUST_MAX_READ_RESPONSE_BYTES,
        "targetReadResponseBase64",
    )?;
    std::str::from_utf8(&target_read_response_bytes).map_err(|_| {
        EngineError::Validation(
            "targetReadResponseBase64 must decode to UTF-8 JSON bytes".to_string(),
        )
    })?;
    let read_wire: RecoverStateWitnessCheckpointRequest =
        serde_json::from_slice(&target_read_response_bytes).map_err(|error| {
            EngineError::Validation(format!(
                "targetReadResponseBase64 contains invalid read-response JSON: {error}"
            ))
        })?;
    let final_configuration = final_certificate.to.anchor_configuration()?;
    let (
        target_read_acknowledgement,
        target_read_expires_at_unix_ms,
        target_read_acknowledgement_bytes,
    ) = validate_certified_transition_read_response(
        read_wire,
        &final_configuration,
        final_certificate.to.reference.previous_event_root,
        require_fresh_read,
    )?;
    if target_read_acknowledgement.binding_hash != final_certificate.to.binding_hash
        || target_read_acknowledgement.service_epoch != final_certificate.to.reference.service_epoch
        || target_read_acknowledgement.revision < final_certificate.to.reference.revision
    {
        return Err(EngineError::Validation(
            "final state-anchor Read is outside the certified target epoch".to_string(),
        ));
    }

    Ok(VerifiedStateAnchorTrustTransition {
        request,
        certificates,
        target_read_acknowledgement_bytes,
        target_read_acknowledgement,
        target_read_expires_at_unix_ms,
    })
}

fn parse_endpoint(
    wire: &StateAnchorTrustEndpoint,
    label: &str,
) -> Result<StateAnchorTrustEndpointModel, EngineError> {
    let activation_manifest_hash = parse_nonzero_bytes32(
        &wire.activation_manifest_hash,
        &format!("{label}.activationManifestHash"),
    )?;
    let activation_manifest_sequence = parse_nonzero_u64(
        &wire.activation_manifest_sequence,
        &format!("{label}.activationManifestSequence"),
    )?;
    let binding_hash = parse_nonzero_bytes32(&wire.binding_hash, &format!("{label}.bindingHash"))?;
    let response_public_key = parse_nonzero_bytes32(
        &wire.response_public_key,
        &format!("{label}.responsePublicKey"),
    )?;
    validate_strong_ed25519_verifying_key(
        &response_public_key,
        &format!("{label}.responsePublicKey"),
    )?;
    let response_public_key_spki_sha256 = parse_nonzero_bytes32(
        &wire.response_public_key_spki_sha256,
        &format!("{label}.responsePublicKeySpkiSha256"),
    )?;
    if ed25519_spki_sha256(&response_public_key) != response_public_key_spki_sha256 {
        return Err(EngineError::Validation(format!(
            "{label}.responsePublicKeySpkiSha256 does not match its raw Ed25519 key"
        )));
    }
    let offline_authority_public_key = parse_nonzero_bytes32(
        &wire.offline_authority_public_key,
        &format!("{label}.offlineAuthorityPublicKey"),
    )?;
    validate_strong_ed25519_verifying_key(
        &offline_authority_public_key,
        &format!("{label}.offlineAuthorityPublicKey"),
    )?;
    let offline_authority_spki_sha256 = parse_nonzero_bytes32(
        &wire.offline_authority_spki_sha256,
        &format!("{label}.offlineAuthoritySpkiSha256"),
    )?;
    if ed25519_spki_sha256(&offline_authority_public_key) != offline_authority_spki_sha256 {
        return Err(EngineError::Validation(format!(
            "{label}.offlineAuthoritySpkiSha256 does not match its raw Ed25519 key"
        )));
    }
    if response_public_key == offline_authority_public_key {
        return Err(EngineError::Validation(format!(
            "{label} online and offline Ed25519 keys must be role-distinct"
        )));
    }
    let witness_maximum_records = parse_nonzero_u64(
        &wire.witness_maximum_records,
        &format!("{label}.witnessMaximumRecords"),
    )?;
    if witness_maximum_records > TBTC_SIGNER_HARD_MAX_STATE_WITNESS_MAX_RECORDS as u64 {
        return Err(EngineError::Validation(format!(
            "{label}.witnessMaximumRecords exceeds the signer hard maximum"
        )));
    }
    let witness_rotation_threshold_records = parse_nonzero_u64(
        &wire.witness_rotation_threshold_records,
        &format!("{label}.witnessRotationThresholdRecords"),
    )?;
    // Same geometry rule the local configuration enforces: six terminal
    // records for an interrupted multi-snapshot retry plus the two-record
    // quarantine reserve that keeps corruption recovery available once the
    // journal parks at the terminal band. A certified endpoint that omitted
    // the reserve would pin a store with no supported exit from a corrupt
    // state image.
    if witness_rotation_threshold_records < 2
        || witness_rotation_threshold_records
            .checked_add(TBTC_SIGNER_STATE_WITNESS_ROTATION_TERMINAL_RECORD_RESERVATION as u64)
            .and_then(|reserved| {
                reserved.checked_add(TBTC_SIGNER_STATE_WITNESS_QUARANTINE_RECORD_RESERVATION as u64)
            })
            .is_none_or(|reserved| reserved > witness_maximum_records)
    {
        return Err(EngineError::Validation(format!(
            "{label}.witnessRotationThresholdRecords must be at least 2 and reserve six terminal \
             records and the quarantine pair"
        )));
    }
    let reference = parse_reference(&wire.reference, &format!("{label}.reference"))?;
    Ok(StateAnchorTrustEndpointModel {
        activation_manifest_hash,
        activation_manifest_sequence,
        binding_hash,
        response_public_key,
        response_public_key_spki_sha256,
        offline_authority_public_key,
        offline_authority_spki_sha256,
        witness_maximum_records,
        witness_rotation_threshold_records,
        reference,
    })
}

fn parse_reference(
    wire: &StateAnchorTrustReference,
    label: &str,
) -> Result<StateAnchorTrustReferenceModel, EngineError> {
    let service_epoch = parse_nonzero_u64(&wire.service_epoch, &format!("{label}.serviceEpoch"))?;
    let revision = parse_nonzero_u64(&wire.revision, &format!("{label}.revision"))?;
    let previous_event_root = parse_canonical_bytes32(
        &wire.previous_event_root,
        &format!("{label}.previousEventRoot"),
    )?;
    let event_root = parse_nonzero_bytes32(&wire.event_root, &format!("{label}.eventRoot"))?;
    let checkpoint_ack_digest = parse_nonzero_bytes32(
        &wire.checkpoint_ack_digest,
        &format!("{label}.checkpointAckDigest"),
    )?;
    Ok(StateAnchorTrustReferenceModel {
        service_epoch,
        revision,
        previous_event_root,
        event_root,
        checkpoint_ack_digest,
        checkpoint: parse_checkpoint(&wire.checkpoint, &format!("{label}.checkpoint"))?,
    })
}

fn parse_checkpoint(
    wire: &StateAnchorTrustCheckpoint,
    label: &str,
) -> Result<StateAnchorTrustCheckpointModel, EngineError> {
    let store_fingerprint = parse_nonzero_bytes32(
        &wire.store_fingerprint,
        &format!("{label}.storeFingerprint"),
    )?;
    let generation = parse_nonzero_u64(&wire.generation, &format!("{label}.generation"))?;
    let previous_state_commitment = parse_nonzero_bytes32(
        &wire.previous_state_commitment,
        &format!("{label}.previousStateCommitment"),
    )?;
    let state_image_digest = parse_nonzero_bytes32(
        &wire.state_image_digest,
        &format!("{label}.stateImageDigest"),
    )?;
    let parsed_state_commitment =
        parse_nonzero_bytes32(&wire.state_commitment, &format!("{label}.stateCommitment"))?;
    if parsed_state_commitment
        != state_commitment(
            &store_fingerprint,
            generation,
            &previous_state_commitment,
            &state_image_digest,
        )
    {
        return Err(EngineError::Validation(format!(
            "{label}.stateCommitment is invalid"
        )));
    }
    Ok(StateAnchorTrustCheckpointModel {
        store_fingerprint,
        generation,
        previous_state_commitment,
        state_image_digest,
        state_commitment: parsed_state_commitment,
    })
}

fn validate_rotation_endpoints(
    from: &StateAnchorTrustEndpointModel,
    to: &StateAnchorTrustEndpointModel,
) -> Result<(), EngineError> {
    if to.activation_manifest_hash == from.activation_manifest_hash
        || to.binding_hash == from.binding_hash
    {
        return Err(EngineError::Validation(
            "rotation must change both activationManifestHash and bindingHash".to_string(),
        ));
    }
    if to.activation_manifest_sequence
        != from
            .activation_manifest_sequence
            .checked_add(1)
            .ok_or_else(|| {
                EngineError::Validation(
                    "rotation activation manifest sequence overflows u64".to_string(),
                )
            })?
    {
        return Err(EngineError::Validation(
            "rotation activationManifestSequence must advance by exactly one".to_string(),
        ));
    }
    if to.reference.service_epoch
        != from.reference.service_epoch.checked_add(1).ok_or_else(|| {
            EngineError::Validation("rotation service epoch overflows u64".to_string())
        })?
        || to.reference.revision != 1
        || to.reference.previous_event_root != from.reference.event_root
    {
        return Err(EngineError::Validation(
            "rotation target must be the linked revision-1 genesis of the next epoch".to_string(),
        ));
    }
    if to.reference.checkpoint != from.reference.checkpoint {
        return Err(EngineError::Validation(
            "rotation target checkpoint must exactly equal its predecessor".to_string(),
        ));
    }
    if from.offline_authority_public_key != to.offline_authority_public_key
        || from.offline_authority_spki_sha256 != to.offline_authority_spki_sha256
    {
        return Err(EngineError::Validation(
            "offline state-anchor authority rotation is unsupported".to_string(),
        ));
    }
    if from.witness_maximum_records != to.witness_maximum_records
        || from.witness_rotation_threshold_records != to.witness_rotation_threshold_records
    {
        return Err(EngineError::Validation(
            "state-anchor trust rotation cannot change witness geometry".to_string(),
        ));
    }
    Ok(())
}

fn validate_certificate_link(
    previous: &VerifiedStateAnchorTrustCertificate,
    next: &VerifiedStateAnchorTrustCertificate,
) -> Result<(), EngineError> {
    let expected_sequence = previous
        .certificate_sequence
        .checked_add(1)
        .ok_or_else(|| {
            EngineError::Validation("state-anchor certificate sequence overflows u64".to_string())
        })?;
    let next_from = next.from.as_ref().ok_or_else(|| {
        EngineError::Validation(
            "certificateChain successor must be a rotation with a from endpoint".to_string(),
        )
    })?;
    if next.kind != StateAnchorTrustCertificateKind::Rotation
        || next.certificate_sequence != expected_sequence
        || next.previous_certificate_digest != previous.certificate_digest
        || next.protocol_id != previous.protocol_id
        || next.stream_id != previous.stream_id
        || next.signer_store_fingerprint != previous.signer_store_fingerprint
        || !state_anchor_trust_endpoint_static_identity_eq(next_from, &previous.to)
    {
        return Err(EngineError::Validation(
            "certificateChain is not an exact contiguous trust transition".to_string(),
        ));
    }
    validate_state_anchor_trust_reference_descendant(
        &previous.to.reference,
        &next_from.reference,
        "certificateChain successor from.reference",
    )?;
    Ok(())
}

pub(crate) fn state_anchor_trust_endpoint_static_identity_eq(
    left: &StateAnchorTrustEndpointModel,
    right: &StateAnchorTrustEndpointModel,
) -> bool {
    left.activation_manifest_hash == right.activation_manifest_hash
        && left.activation_manifest_sequence == right.activation_manifest_sequence
        && left.binding_hash == right.binding_hash
        && left.response_public_key == right.response_public_key
        && left.response_public_key_spki_sha256 == right.response_public_key_spki_sha256
        && left.offline_authority_public_key == right.offline_authority_public_key
        && left.offline_authority_spki_sha256 == right.offline_authority_spki_sha256
        && left.witness_maximum_records == right.witness_maximum_records
        && left.witness_rotation_threshold_records == right.witness_rotation_threshold_records
}

pub(crate) fn validate_state_anchor_trust_reference_descendant(
    floor: &StateAnchorTrustReferenceModel,
    candidate: &StateAnchorTrustReferenceModel,
    label: &str,
) -> Result<(), EngineError> {
    if candidate.service_epoch != floor.service_epoch || candidate.revision < floor.revision {
        return Err(EngineError::Validation(format!(
            "{label} must remain in the certified epoch and not precede its floor"
        )));
    }
    if candidate.revision - floor.revision > STATE_ANCHOR_TRUST_MAX_REVISION_DISTANCE {
        return Err(EngineError::Validation(format!(
            "{label} exceeds the certified floor by more than \
             {STATE_ANCHOR_TRUST_MAX_REVISION_DISTANCE} revisions"
        )));
    }
    if candidate.revision == floor.revision {
        if candidate != floor {
            return Err(EngineError::Validation(format!(
                "{label} equivocates at the certified floor revision"
            )));
        }
        return Ok(());
    }
    if candidate.previous_event_root == [0u8; 32]
        || candidate.checkpoint.generation < floor.checkpoint.generation
        || candidate.checkpoint.store_fingerprint != floor.checkpoint.store_fingerprint
    {
        return Err(EngineError::Validation(format!(
            "{label} is not a monotonic same-epoch descendant of its certified floor"
        )));
    }
    if candidate.checkpoint.generation == floor.checkpoint.generation
        && candidate.checkpoint != floor.checkpoint
    {
        return Err(EngineError::Validation(format!(
            "{label} changes a checkpoint without advancing its generation"
        )));
    }
    Ok(())
}

fn validate_transition_chain_against_installed_config(
    certificates: &[VerifiedStateAnchorTrustCertificate],
) -> Result<(), EngineError> {
    let configured = configured_state_anchor()?.ok_or_else(|| {
        EngineError::Validation(
            "state-anchor trust transition requires configured anchor pins".to_string(),
        )
    })?;
    let trust = configured.trust.as_ref().ok_or_else(|| {
        EngineError::Validation(
            "state-anchor trust transition requires complete trust-head pins".to_string(),
        )
    })?;
    let final_certificate = certificates
        .last()
        .expect("caller guarantees a nonempty chain");
    let to = &final_certificate.to;
    if final_certificate.protocol_id != trust.protocol_id
        || final_certificate.stream_id != trust.stream_id
        || final_certificate.certificate_sequence != trust.certificate_sequence
        || final_certificate.certificate_digest != trust.certificate_digest
        || to.activation_manifest_hash != trust.activation_manifest_hash
        || to.activation_manifest_sequence != trust.activation_manifest_sequence
        || to.binding_hash != configured.binding_hash
        || to.response_public_key != configured.response_public_key
        || to.response_public_key_spki_sha256 != configured.response_public_key_spki_sha256
        || to.offline_authority_public_key != trust.offline_authority_public_key
        || to.offline_authority_spki_sha256 != trust.offline_authority_public_key_spki_sha256
        || to.witness_maximum_records != state_witness_max_records()? as u64
        || to.witness_rotation_threshold_records != configured.rotation_threshold_records as u64
    {
        return Err(EngineError::Validation(
            "final state-anchor trust certificate does not exactly match installed config pins"
                .to_string(),
        ));
    }
    for certificate in certificates {
        if certificate.protocol_id != trust.protocol_id
            || certificate.stream_id != trust.stream_id
            || certificate.to.offline_authority_public_key != trust.offline_authority_public_key
            || certificate.to.offline_authority_spki_sha256
                != trust.offline_authority_public_key_spki_sha256
        {
            return Err(EngineError::Validation(
                "certificateChain changes a pinned protocol, stream, or offline authority"
                    .to_string(),
            ));
        }
    }
    Ok(())
}

#[allow(clippy::too_many_arguments)]
fn trust_certificate_core_digest(
    kind: StateAnchorTrustCertificateKind,
    certificate_sequence: u64,
    previous_certificate_digest: &[u8; 32],
    protocol_id: &[u8; 32],
    stream_id: &[u8; 32],
    signer_store_fingerprint: &[u8; 32],
    from: Option<&StateAnchorTrustEndpointModel>,
    to: &StateAnchorTrustEndpointModel,
) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(TRUST_CORE_DOMAIN);
    digest.update([kind.transcript_byte()]);
    digest.update(certificate_sequence.to_be_bytes());
    digest.update(previous_certificate_digest);
    digest.update(protocol_id);
    digest.update(stream_id);
    digest.update(signer_store_fingerprint);
    update_core_from_endpoint(&mut digest, from);
    update_core_to_endpoint(&mut digest, to);
    digest.finalize().into()
}

fn update_core_from_endpoint(
    digest: &mut Sha256,
    endpoint: Option<&StateAnchorTrustEndpointModel>,
) {
    let Some(endpoint) = endpoint else {
        // Frozen bootstrap transcript: every fixed-width FROM slot is zero.
        digest.update([0u8; 32]); // manifest hash
        digest.update([0u8; 8]); // manifest sequence
        digest.update([0u8; 32]); // binding
        digest.update([0u8; 32]); // online raw
        digest.update([0u8; 32]); // online SPKI hash
        digest.update([0u8; 32]); // authority raw
        digest.update([0u8; 32]); // authority SPKI hash
        digest.update([0u8; 8]); // max records
        digest.update([0u8; 8]); // rotation threshold
        digest.update([0u8; 8]); // epoch
        digest.update([0u8; 8]); // revision
        digest.update([0u8; 32]); // previous event root
        digest.update([0u8; 32]); // event root
        digest.update([0u8; 32]); // acknowledgement digest
        digest.update([0u8; 32]); // checkpoint store
        digest.update([0u8; 8]); // checkpoint generation
        digest.update([0u8; 32]); // checkpoint previous commitment
        digest.update([0u8; 32]); // checkpoint image
        digest.update([0u8; 32]); // checkpoint commitment
        return;
    };
    update_endpoint_identity_geometry(digest, endpoint);
    let reference = &endpoint.reference;
    digest.update(reference.service_epoch.to_be_bytes());
    digest.update(reference.revision.to_be_bytes());
    digest.update(reference.previous_event_root);
    digest.update(reference.event_root);
    digest.update(reference.checkpoint_ack_digest);
    update_checkpoint(digest, &reference.checkpoint);
}

fn update_core_to_endpoint(digest: &mut Sha256, endpoint: &StateAnchorTrustEndpointModel) {
    update_endpoint_identity_geometry(digest, endpoint);
    digest.update(endpoint.reference.service_epoch.to_be_bytes());
    update_checkpoint(digest, &endpoint.reference.checkpoint);
}

fn update_endpoint_identity_geometry(
    digest: &mut Sha256,
    endpoint: &StateAnchorTrustEndpointModel,
) {
    digest.update(endpoint.activation_manifest_hash);
    digest.update(endpoint.activation_manifest_sequence.to_be_bytes());
    digest.update(endpoint.binding_hash);
    digest.update(endpoint.response_public_key);
    digest.update(endpoint.response_public_key_spki_sha256);
    digest.update(endpoint.offline_authority_public_key);
    digest.update(endpoint.offline_authority_spki_sha256);
    digest.update(endpoint.witness_maximum_records.to_be_bytes());
    digest.update(endpoint.witness_rotation_threshold_records.to_be_bytes());
}

fn update_checkpoint(digest: &mut Sha256, checkpoint: &StateAnchorTrustCheckpointModel) {
    digest.update(checkpoint.store_fingerprint);
    digest.update(checkpoint.generation.to_be_bytes());
    digest.update(checkpoint.previous_state_commitment);
    digest.update(checkpoint.state_image_digest);
    digest.update(checkpoint.state_commitment);
}

fn trust_certificate_final_digest(
    core_digest: &[u8; 32],
    core_signature: &[u8; 64],
    operation_id: &[u8; 32],
    transition_digest: &[u8; 32],
    to: &StateAnchorTrustReferenceModel,
    target_acknowledgement_sha256: &[u8; 32],
) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(TRUST_FINAL_DOMAIN);
    digest.update(core_digest);
    digest.update(core_signature);
    digest.update(operation_id);
    digest.update(transition_digest);
    digest.update(to.service_epoch.to_be_bytes());
    digest.update(to.revision.to_be_bytes());
    digest.update(to.previous_event_root);
    digest.update(to.event_root);
    digest.update(to.checkpoint_ack_digest);
    update_checkpoint(&mut digest, &to.checkpoint);
    digest.update(target_acknowledgement_sha256);
    digest.finalize().into()
}

fn decode_canonical_base64(
    encoded: &str,
    maximum_decoded_length: usize,
    label: &str,
) -> Result<Vec<u8>, EngineError> {
    // A padded RFC 4648 string cannot decode to more than 3/4 its bytes. Check
    // before allocating, then require exact re-encoding to reject whitespace,
    // unpadded aliases, and noncanonical trailing bits.
    let maximum_encoded_length = maximum_decoded_length
        .checked_add(2)
        .and_then(|value| value.checked_div(3))
        .and_then(|value| value.checked_mul(4))
        .ok_or_else(|| EngineError::Internal("Base64 length bound overflowed".to_string()))?;
    if encoded.len() > maximum_encoded_length {
        return Err(EngineError::Validation(format!(
            "{label} exceeds the maximum decoded length"
        )));
    }
    let decoded = Base64::decode_vec(encoded).map_err(|_| {
        EngineError::Validation(format!(
            "{label} must be strict canonical padded RFC 4648 Base64"
        ))
    })?;
    if decoded.len() > maximum_decoded_length || Base64::encode_string(&decoded) != encoded {
        return Err(EngineError::Validation(format!(
            "{label} must be strict canonical padded RFC 4648 Base64"
        )));
    }
    Ok(decoded)
}

fn parse_canonical_base64_signature(encoded: &str, label: &str) -> Result<[u8; 64], EngineError> {
    let decoded = decode_canonical_base64(encoded, 64, label)?;
    decoded.try_into().map_err(|_| {
        EngineError::Validation(format!(
            "{label} must be canonical padded Base64 encoding exactly 64 bytes"
        ))
    })
}

fn parse_nonzero_bytes32(value: &str, label: &str) -> Result<[u8; 32], EngineError> {
    let parsed = parse_canonical_bytes32(value, label)?;
    if parsed == [0u8; 32] {
        return Err(EngineError::Validation(format!("{label} must be nonzero")));
    }
    Ok(parsed)
}

fn parse_nonzero_u64(value: &str, label: &str) -> Result<u64, EngineError> {
    let parsed = parse_canonical_u64(value, label)?;
    if parsed == 0 {
        return Err(EngineError::Validation(format!("{label} must be nonzero")));
    }
    Ok(parsed)
}

fn require_declared_digest(
    wire: &str,
    label: &str,
    expected: &[u8; 32],
) -> Result<(), EngineError> {
    if parse_canonical_bytes32(wire, label)? != *expected {
        return Err(EngineError::Validation(format!(
            "{label} does not match the frozen transcript"
        )));
    }
    Ok(())
}

fn verify_ed25519_signature(
    public_key: &[u8; 32],
    digest: &[u8; 32],
    signature: &[u8; 64],
    label: &str,
) -> Result<(), EngineError> {
    let verifying_key = VerifyingKey::from_bytes(public_key).map_err(|error| {
        EngineError::Validation(format!("{label} public key is invalid: {error}"))
    })?;
    verifying_key
        .verify_strict(digest, &Signature::from_bytes(signature))
        .map_err(|_| EngineError::Validation(format!("{label} is invalid")))
}

fn hash_direct(domain: &[u8], fields: &[&[u8]]) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(domain);
    for field in fields {
        digest.update(field);
    }
    digest.finalize().into()
}

#[allow(dead_code)]
pub(crate) fn state_anchor_trust_head_result(
    sequence: u64,
    digest: [u8; 32],
    endpoint: &StateAnchorTrustEndpointModel,
) -> StateAnchorTrustHeadResult {
    StateAnchorTrustHeadResult {
        schema: STATE_ANCHOR_TRUST_HEAD_SCHEMA.to_string(),
        certificate_sequence: sequence.to_string(),
        certificate_digest: bytes32_hex(digest),
        activation_manifest_sequence: endpoint.activation_manifest_sequence.to_string(),
        activation_manifest_hash: bytes32_hex(endpoint.activation_manifest_hash),
        binding_hash: bytes32_hex(endpoint.binding_hash),
        response_public_key_spki_sha256: bytes32_hex(endpoint.response_public_key_spki_sha256),
        offline_authority_spki_sha256: bytes32_hex(endpoint.offline_authority_spki_sha256),
        service_epoch: endpoint.reference.service_epoch.to_string(),
        certified_floor: endpoint.reference.to_wire(),
        witness_maximum_records: endpoint.witness_maximum_records.to_string(),
        witness_rotation_threshold_records: endpoint.witness_rotation_threshold_records.to_string(),
    }
}

#[allow(dead_code)]
fn transition_state_witness_anchor_result(
    outcome: StateAnchorTrustTransitionStoreOutcome,
) -> TransitionStateWitnessAnchorResult {
    let store_fingerprint = outcome.trust_head.signer_store_fingerprint;
    let trust_head = state_anchor_trust_head_result(
        outcome.trust_head.certificate_sequence,
        outcome.trust_head.certificate_digest,
        &outcome.trust_head.to,
    );

    TransitionStateWitnessAnchorResult {
        schema: STATE_ANCHOR_TRUST_TRANSITION_RESULT_SCHEMA.to_string(),
        installed: true,
        idempotent: outcome.idempotent,
        applied_certificate_count: outcome.applied_certificate_count.to_string(),
        trust_head,
        current_checkpoint: StateAnchorTrustCheckpointModel::from_witness(
            store_fingerprint,
            &outcome.tip,
        )
        .to_wire(),
        witness_base_checkpoint: StateAnchorTrustCheckpointModel::from_witness(
            store_fingerprint,
            &outcome.base,
        )
        .to_wire(),
        current_anchor_reference: StateAnchorTrustReferenceModel::from_acknowledgement(
            &outcome.anchor.latest,
        )
        .to_wire(),
    }
}

/// Verifies a fresh, offline-certified trust-transition request before opening
/// the durable store, then executes the crash-safe transition under the
/// startup gate. Trust replacement is intentionally unavailable once normal
/// engine/store initialization has begun.
#[allow(dead_code)]
pub(crate) fn transition_state_witness_anchor(
    request: TransitionStateWitnessAnchorRequest,
) -> Result<TransitionStateWitnessAnchorResult, EngineError> {
    let transition = verify_state_anchor_trust_transition_request(request, true)?;
    let outcome = with_startup_state_anchor_trust_transition(&transition, |store| {
        store.transition_state_witness_anchor(&transition)
    })?;
    Ok(transition_state_witness_anchor_result(outcome))
}

/// Returns the committed offline-certified trust head. A preflight call uses
/// an ephemeral, descriptor-bound inspection acquisition so observing the
/// head does not prevent the startup-only transition symbol from running.
#[allow(dead_code)]
pub(crate) fn state_anchor_trust_head() -> Result<StateAnchorTrustHeadResult, EngineError> {
    let outcome = with_startup_state_anchor_trust_head_inspection(|store| {
        store.state_anchor_trust_head_snapshot()
    })?;
    Ok(state_anchor_trust_head_result(
        outcome.trust_head.certificate_sequence,
        outcome.trust_head.certificate_digest,
        &outcome.trust_head.to,
    ))
}

#[allow(dead_code)]
pub(crate) fn state_anchor_bootstrap_facts() -> Result<StateAnchorBootstrapFactsResult, EngineError>
{
    let (store_fingerprint, checkpoint) = with_startup_state_anchor_bootstrap_facts(|store| {
        store.state_anchor_bootstrap_facts_snapshot()
    })?;
    Ok(StateAnchorBootstrapFactsResult {
        schema: STATE_ANCHOR_BOOTSTRAP_FACTS_SCHEMA.to_string(),
        store_fingerprint: bytes32_hex(store_fingerprint),
        current_checkpoint: StateAnchorTrustCheckpointModel::from_witness(
            store_fingerprint,
            &checkpoint,
        )
        .to_wire(),
    })
}

#[cfg(test)]
fn state_anchor_trust_endpoint_wire_for_tests(
    endpoint: &StateAnchorTrustEndpointModel,
) -> StateAnchorTrustEndpoint {
    StateAnchorTrustEndpoint {
        activation_manifest_hash: bytes32_hex(endpoint.activation_manifest_hash),
        activation_manifest_sequence: endpoint.activation_manifest_sequence.to_string(),
        binding_hash: bytes32_hex(endpoint.binding_hash),
        response_public_key: bytes32_hex(endpoint.response_public_key),
        response_public_key_spki_sha256: bytes32_hex(endpoint.response_public_key_spki_sha256),
        offline_authority_public_key: bytes32_hex(endpoint.offline_authority_public_key),
        offline_authority_spki_sha256: bytes32_hex(endpoint.offline_authority_spki_sha256),
        witness_maximum_records: endpoint.witness_maximum_records.to_string(),
        witness_rotation_threshold_records: endpoint.witness_rotation_threshold_records.to_string(),
        reference: endpoint.reference.to_wire(),
    }
}

#[cfg(test)]
fn state_anchor_acknowledgement_wire_for_trust_tests(
    acknowledgement: &StateAnchorAcknowledgement,
) -> AcknowledgeStateWitnessCheckpointRequest {
    AcknowledgeStateWitnessCheckpointRequest {
        schema: TBTC_SIGNER_STATE_WITNESS_CHECKPOINT_ACK_SCHEMA.to_string(),
        binding_hash: bytes32_hex(acknowledgement.binding_hash),
        request_digest: bytes32_hex(acknowledgement.request_digest),
        nonce: bytes32_hex(acknowledgement.nonce),
        status: "applied".to_string(),
        service_epoch: acknowledgement.service_epoch.to_string(),
        revision: acknowledgement.revision.to_string(),
        previous_event_root: bytes32_hex(acknowledgement.previous_event_root),
        event_root: bytes32_hex(acknowledgement.event_root),
        checkpoint: StateWitnessCheckpointRequest {
            store_fingerprint: bytes32_hex(acknowledgement.checkpoint_store_fingerprint),
            generation: acknowledgement.checkpoint_generation.to_string(),
            previous_state_commitment: bytes32_hex(acknowledgement.checkpoint_previous_commitment),
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

/// Builds and verifies a complete bootstrap certificate plus outer Read around
/// an actual store checkpoint. Keeping this in the verifier module lets
/// end-to-end store tests exercise production parsing and every frozen
/// transcript instead of constructing privileged `Verified*` values by hand.
#[cfg(test)]
#[allow(clippy::too_many_arguments)]
pub(crate) fn bootstrap_state_anchor_trust_transition_for_tests(
    store_fingerprint: [u8; 32],
    tip: &StateWitness,
    acknowledgement_committed_at_unix_ms: u64,
    acknowledgement_expires_at_unix_ms: u64,
    read_committed_at_unix_ms: u64,
    read_expires_at_unix_ms: u64,
    require_fresh_read: bool,
) -> Result<VerifiedStateAnchorTrustTransition, EngineError> {
    let online_signing_key = SigningKey::from_bytes(&[0x61; 32]);
    let offline_signing_key = SigningKey::from_bytes(&[0x62; 32]);
    let response_public_key = online_signing_key.verifying_key().to_bytes();
    let offline_authority_public_key = offline_signing_key.verifying_key().to_bytes();
    let response_public_key_spki_sha256 = ed25519_spki_sha256(&response_public_key);
    let offline_authority_spki_sha256 = ed25519_spki_sha256(&offline_authority_public_key);
    let protocol_id = [0x63; 32];
    let stream_id = [0x64; 32];
    let binding_hash = [0x65; 32];
    let checkpoint = StateAnchorTrustCheckpointModel::from_witness(store_fingerprint, tip);
    let mut target = StateAnchorTrustEndpointModel {
        activation_manifest_hash: [0x66; 32],
        activation_manifest_sequence: 1,
        binding_hash,
        response_public_key,
        response_public_key_spki_sha256,
        offline_authority_public_key,
        offline_authority_spki_sha256,
        witness_maximum_records: 10,
        witness_rotation_threshold_records: 2,
        reference: StateAnchorTrustReferenceModel {
            service_epoch: 1,
            revision: 1,
            previous_event_root: [0u8; 32],
            event_root: [0u8; 32],
            checkpoint_ack_digest: [0u8; 32],
            checkpoint,
        },
    };
    let previous_certificate_digest = [0u8; 32];
    let core_digest = trust_certificate_core_digest(
        StateAnchorTrustCertificateKind::Bootstrap,
        1,
        &previous_certificate_digest,
        &protocol_id,
        &stream_id,
        &store_fingerprint,
        None,
        &target,
    );
    let core_signature = offline_signing_key.sign(&core_digest).to_bytes();
    let operation_id = hash_direct(TRUST_OPERATION_ID_DOMAIN, &[&core_digest]);
    let transition_digest = hash_direct(
        TRUST_TRANSITION_DIGEST_DOMAIN,
        &[&core_digest, &operation_id],
    );

    let mut acknowledgement = StateAnchorAcknowledgement {
        binding_hash,
        request_digest: [0x67; 32],
        nonce: [0x68; 32],
        status: 1,
        service_epoch: 1,
        revision: 1,
        previous_event_root: [0u8; 32],
        event_root: [0u8; 32],
        checkpoint_store_fingerprint: store_fingerprint,
        checkpoint_generation: tip.generation,
        checkpoint_previous_commitment: tip.previous_commitment,
        checkpoint_state_image_digest: tip.state_image_digest,
        checkpoint_state_commitment: tip.commitment,
        operation_id,
        transition_digest,
        committed_at_unix_ms: acknowledgement_committed_at_unix_ms,
        expires_at_unix_ms: acknowledgement_expires_at_unix_ms,
        signing_digest: [0u8; 32],
        signature: [0u8; 64],
        configured_spki_hash: response_public_key_spki_sha256,
        acknowledgement_digest: [0u8; 32],
    };
    acknowledgement.event_root = state_anchor_event_root_for_tests(&acknowledgement);
    acknowledgement.signing_digest = state_anchor_signing_digest_for_tests(&acknowledgement);
    acknowledgement.signature = online_signing_key
        .sign(&acknowledgement.signing_digest)
        .to_bytes();
    acknowledgement.acknowledgement_digest = state_anchor_acknowledgement_digest_for_tests(
        &acknowledgement.signing_digest,
        &acknowledgement.signature,
        &response_public_key_spki_sha256,
    );
    target.reference = StateAnchorTrustReferenceModel::from_acknowledgement(&acknowledgement);

    let acknowledgement_wire = state_anchor_acknowledgement_wire_for_trust_tests(&acknowledgement);
    let acknowledgement_bytes = serde_json::to_vec(&acknowledgement_wire).map_err(|error| {
        EngineError::Internal(format!(
            "failed to encode test target acknowledgement: {error}"
        ))
    })?;
    let target_acknowledgement_sha256: [u8; 32] = Sha256::digest(&acknowledgement_bytes).into();
    let final_digest = trust_certificate_final_digest(
        &core_digest,
        &core_signature,
        &operation_id,
        &transition_digest,
        &target.reference,
        &target_acknowledgement_sha256,
    );
    let final_signature = offline_signing_key.sign(&final_digest).to_bytes();
    let certificate_digest = hash_direct(
        TRUST_CERTIFICATE_DIGEST_DOMAIN,
        &[
            &final_digest,
            &final_signature,
            &offline_authority_spki_sha256,
        ],
    );
    let certificate = StateAnchorTrustCertificate {
        schema: STATE_ANCHOR_TRUST_CERTIFICATE_SCHEMA.to_string(),
        kind: "bootstrap".to_string(),
        certificate_sequence: "1".to_string(),
        previous_certificate_digest: bytes32_hex(previous_certificate_digest),
        protocol_id: bytes32_hex(protocol_id),
        stream_id: bytes32_hex(stream_id),
        signer_store_fingerprint: bytes32_hex(store_fingerprint),
        from: RequiredNullableStateAnchorTrustEndpoint(None),
        to: state_anchor_trust_endpoint_wire_for_tests(&target),
        core_digest: bytes32_hex(core_digest),
        core_signature: Base64::encode_string(&core_signature),
        operation_id: bytes32_hex(operation_id),
        transition_digest: bytes32_hex(transition_digest),
        target_acknowledgement_base64: Base64::encode_string(&acknowledgement_bytes),
        target_acknowledgement_sha256: bytes32_hex(target_acknowledgement_sha256),
        final_signature: Base64::encode_string(&final_signature),
        certificate_digest: bytes32_hex(certificate_digest),
    };

    let read_request_digest = [0x69; 32];
    let read_nonce = [0x6a; 32];
    let raw_acknowledgement_digest: [u8; 32] = Sha256::digest(&acknowledgement_bytes).into();
    let read_signing_digest = state_anchor_read_response_signing_digest_for_tests(
        &binding_hash,
        &read_request_digest,
        &read_nonce,
        acknowledgement.service_epoch,
        acknowledgement.revision,
        &acknowledgement.event_root,
        &store_fingerprint,
        tip.generation,
        &tip.previous_commitment,
        &tip.state_image_digest,
        &tip.commitment,
        &operation_id,
        &transition_digest,
        read_committed_at_unix_ms,
        read_expires_at_unix_ms,
        &acknowledgement.acknowledgement_digest,
        &raw_acknowledgement_digest,
    );
    let read_signature = online_signing_key.sign(&read_signing_digest).to_bytes();
    let raw_acknowledgement = serde_json::value::RawValue::from_string(
        String::from_utf8(acknowledgement_bytes).map_err(|error| {
            EngineError::Internal(format!("test acknowledgement JSON was not UTF-8: {error}"))
        })?,
    )
    .map_err(|error| {
        EngineError::Internal(format!(
            "test acknowledgement JSON was not a raw JSON value: {error}"
        ))
    })?;
    let read = RecoverStateWitnessCheckpointRequest {
        schema: "tbtc-frost-native-signer-state-anchor-read-response/v1".to_string(),
        binding_hash: bytes32_hex(binding_hash),
        request_digest: bytes32_hex(read_request_digest),
        nonce: bytes32_hex(read_nonce),
        status: "present".to_string(),
        service_epoch: acknowledgement.service_epoch.to_string(),
        revision: acknowledgement.revision.to_string(),
        event_root: bytes32_hex(acknowledgement.event_root),
        checkpoint: StateWitnessCheckpointRequest {
            store_fingerprint: bytes32_hex(store_fingerprint),
            generation: tip.generation.to_string(),
            previous_state_commitment: bytes32_hex(tip.previous_commitment),
            state_image_digest: bytes32_hex(tip.state_image_digest),
            state_commitment: bytes32_hex(tip.commitment),
        },
        operation_id: bytes32_hex(operation_id),
        transition_digest: bytes32_hex(transition_digest),
        committed_at_unix_ms: read_committed_at_unix_ms.to_string(),
        expires_at_unix_ms: read_expires_at_unix_ms.to_string(),
        checkpoint_ack: raw_acknowledgement,
        checkpoint_ack_digest: bytes32_hex(acknowledgement.acknowledgement_digest),
        signature: format!("0x{}", hex::encode(read_signature)),
    };
    let read_bytes = serde_json::to_vec(&read).map_err(|error| {
        EngineError::Internal(format!("failed to encode test target Read: {error}"))
    })?;

    std::env::set_var(TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS_ENV, "10");
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
        bytes32_hex(response_public_key_spki_sha256),
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_ANCHOR_PROTOCOL_ID_ENV,
        bytes32_hex(protocol_id),
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_ANCHOR_STREAM_ID_ENV,
        bytes32_hex(stream_id),
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_ANCHOR_ACTIVATION_MANIFEST_HASH_ENV,
        bytes32_hex(target.activation_manifest_hash),
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_ANCHOR_ACTIVATION_MANIFEST_SEQUENCE_ENV,
        target.activation_manifest_sequence.to_string(),
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_ENV,
        bytes32_hex(offline_authority_public_key),
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_ANCHOR_OFFLINE_AUTHORITY_PUBLIC_KEY_SPKI_SHA256_ENV,
        bytes32_hex(offline_authority_spki_sha256),
    );
    std::env::set_var(TBTC_SIGNER_STATE_ANCHOR_TRUST_CERTIFICATE_SEQUENCE_ENV, "1");
    std::env::set_var(
        TBTC_SIGNER_STATE_ANCHOR_TRUST_CERTIFICATE_DIGEST_ENV,
        bytes32_hex(certificate_digest),
    );

    verify_state_anchor_trust_transition_request(
        TransitionStateWitnessAnchorRequest {
            schema: STATE_ANCHOR_TRUST_TRANSITION_SCHEMA.to_string(),
            certificate_chain: vec![certificate],
            target_read_response_base64: Base64::encode_string(&read_bytes),
        },
        require_fresh_read,
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    struct SharedValidVector {
        name: &'static str,
        canonical_json: String,
        core_digest: &'static str,
        operation_id: &'static str,
        transition_digest: &'static str,
        final_digest: &'static str,
        certificate_digest: &'static str,
        canonical_json_sha256: &'static str,
    }

    fn repeated_bytes32(value: u8) -> String {
        format!("0x{}", format!("{value:02x}").repeat(32))
    }

    #[allow(clippy::too_many_arguments)]
    fn endpoint_json(
        activation_manifest_hash: &str,
        activation_manifest_sequence: u64,
        binding_hash: &str,
        response_public_key: &str,
        response_public_key_spki_sha256: &str,
        service_epoch: u64,
        previous_event_root: &str,
        event_root: &str,
        checkpoint_ack_digest: &str,
    ) -> String {
        format!(
            concat!(
                r#"{{"activationManifestHash":"{activation_manifest_hash}","#,
                r#""activationManifestSequence":"{activation_manifest_sequence}","#,
                r#""bindingHash":"{binding_hash}","#,
                r#""responsePublicKey":"{response_public_key}","#,
                r#""responsePublicKeySpkiSha256":"{response_public_key_spki_sha256}","#,
                r#""offlineAuthorityPublicKey":"0xd04ab232742bb4ab3a1368bd4615e4e6d0224ab71a016baf8520a332c9778737","#,
                r#""offlineAuthoritySpkiSha256":"0x503d8ee46d581c649966569ae095e27041de83322e8e0480e5539e00d2d5d874","#,
                r#""witnessMaximumRecords":"1000","#,
                r#""witnessRotationThresholdRecords":"900","#,
                r#""reference":{{"serviceEpoch":"{service_epoch}","revision":"1","#,
                r#""previousEventRoot":"{previous_event_root}","#,
                r#""eventRoot":"{event_root}","#,
                r#""checkpointAckDigest":"{checkpoint_ack_digest}","#,
                r#""checkpoint":{{"storeFingerprint":"0x0303030303030303030303030303030303030303030303030303030303030303","#,
                r#""generation":"7","#,
                r#""previousStateCommitment":"0x3131313131313131313131313131313131313131313131313131313131313131","#,
                r#""stateImageDigest":"0x3232323232323232323232323232323232323232323232323232323232323232","#,
                r#""stateCommitment":"0xb6c6224de6df25c6fa82e94a76fee50897c3c16b863f4f3d8b67eb41982e481a"}}}}}}"#
            ),
            activation_manifest_hash = activation_manifest_hash,
            activation_manifest_sequence = activation_manifest_sequence,
            binding_hash = binding_hash,
            response_public_key = response_public_key,
            response_public_key_spki_sha256 = response_public_key_spki_sha256,
            service_epoch = service_epoch,
            previous_event_root = previous_event_root,
            event_root = event_root,
            checkpoint_ack_digest = checkpoint_ack_digest,
        )
    }

    #[allow(clippy::too_many_arguments)]
    fn target_acknowledgement_base64(
        binding_hash: &str,
        request_digest: &str,
        nonce: &str,
        service_epoch: u64,
        previous_event_root: &str,
        event_root: &str,
        operation_id: &str,
        transition_digest: &str,
        committed_at_unix_ms: u64,
        expires_at_unix_ms: u64,
        signature: &str,
    ) -> String {
        let acknowledgement = format!(
            concat!(
                r#"{{"schema":"tbtc-signer-state-witness-checkpoint-ack/v1","#,
                r#""bindingHash":"{binding_hash}","#,
                r#""requestDigest":"{request_digest}","#,
                r#""nonce":"{nonce}","status":"applied","#,
                r#""serviceEpoch":"{service_epoch}","revision":"1","#,
                r#""previousEventRoot":"{previous_event_root}","#,
                r#""eventRoot":"{event_root}","#,
                r#""checkpoint":{{"storeFingerprint":"0x0303030303030303030303030303030303030303030303030303030303030303","#,
                r#""generation":"7","#,
                r#""previousStateCommitment":"0x3131313131313131313131313131313131313131313131313131313131313131","#,
                r#""stateImageDigest":"0x3232323232323232323232323232323232323232323232323232323232323232","#,
                r#""stateCommitment":"0xb6c6224de6df25c6fa82e94a76fee50897c3c16b863f4f3d8b67eb41982e481a"}},"#,
                r#""operationID":"{operation_id}","#,
                r#""transitionDigest":"{transition_digest}","#,
                r#""committedAtUnixMs":"{committed_at_unix_ms}","#,
                r#""expiresAtUnixMs":"{expires_at_unix_ms}","#,
                r#""signature":"{signature}"}}"#
            ),
            binding_hash = binding_hash,
            request_digest = request_digest,
            nonce = nonce,
            service_epoch = service_epoch,
            previous_event_root = previous_event_root,
            event_root = event_root,
            operation_id = operation_id,
            transition_digest = transition_digest,
            committed_at_unix_ms = committed_at_unix_ms,
            expires_at_unix_ms = expires_at_unix_ms,
            signature = signature,
        );
        Base64::encode_string(acknowledgement.as_bytes())
    }

    #[allow(clippy::too_many_arguments)]
    fn certificate_json(
        kind: &str,
        certificate_sequence: u64,
        previous_certificate_digest: &str,
        from: &str,
        to: &str,
        core_digest: &str,
        core_signature: &str,
        operation_id: &str,
        transition_digest: &str,
        target_acknowledgement_base64: &str,
        target_acknowledgement_sha256: &str,
        final_signature: &str,
        certificate_digest: &str,
    ) -> String {
        format!(
            concat!(
                r#"{{"schema":"tbtc-frost-native-signer-state-anchor-trust-certificate/v1","#,
                r#""kind":"{kind}","certificateSequence":"{certificate_sequence}","#,
                r#""previousCertificateDigest":"{previous_certificate_digest}","#,
                r#""protocolID":"0x0101010101010101010101010101010101010101010101010101010101010101","#,
                r#""streamID":"0x0202020202020202020202020202020202020202020202020202020202020202","#,
                r#""signerStoreFingerprint":"0x0303030303030303030303030303030303030303030303030303030303030303","#,
                r#""from":{from},"to":{to},"coreDigest":"{core_digest}","#,
                r#""coreSignature":"{core_signature}","#,
                r#""operationID":"{operation_id}","#,
                r#""transitionDigest":"{transition_digest}","#,
                r#""targetAcknowledgementBase64":"{target_acknowledgement_base64}","#,
                r#""targetAcknowledgementSHA256":"{target_acknowledgement_sha256}","#,
                r#""finalSignature":"{final_signature}","#,
                r#""certificateDigest":"{certificate_digest}"}}"#
            ),
            kind = kind,
            certificate_sequence = certificate_sequence,
            previous_certificate_digest = previous_certificate_digest,
            from = from,
            to = to,
            core_digest = core_digest,
            core_signature = core_signature,
            operation_id = operation_id,
            transition_digest = transition_digest,
            target_acknowledgement_base64 = target_acknowledgement_base64,
            target_acknowledgement_sha256 = target_acknowledgement_sha256,
            final_signature = final_signature,
            certificate_digest = certificate_digest,
        )
    }

    fn shared_valid_vectors() -> Vec<SharedValidVector> {
        let zero = repeated_bytes32(0);
        let bootstrap_event_root =
            "0x5a2313f20d5a31c960106cc54a0068ed03b8b51eed74ea73cf6a014241a9108e";
        let bootstrap_endpoint = endpoint_json(
            &repeated_bytes32(0x04),
            9,
            &repeated_bytes32(0x05),
            "0xa09aa5f47a6759802ff955f8dc2d2a14a5c99d23be97f864127ff9383455a4f0",
            "0x744f36cca67eb1912cb282e08e86bd2fe0d09004a28cf6e7c6bd56cdd6bf65aa",
            1,
            &zero,
            bootstrap_event_root,
            "0x2f4a5b931123b7ef3e7837ac2adfdd0bc7731bbd084c53ee5a9f02de864670d9",
        );

        let bootstrap_core = "0xd3b5ea2a8c29dc4f4ef5250fc75efb3fb6bcd87167d523661c4a55e7898fb90d";
        let bootstrap_operation =
            "0x49351644f18779575f614ab4b50c14c6454d8bd93d0a287edf41c801826edea0";
        let bootstrap_transition =
            "0x50e2372f4008f0bd47c4e17c517fe5b5dc6062736492a1414ada049878447979";
        let bootstrap_ack = target_acknowledgement_base64(
            &repeated_bytes32(0x05),
            &repeated_bytes32(0x52),
            &repeated_bytes32(0x53),
            1,
            &zero,
            bootstrap_event_root,
            bootstrap_operation,
            bootstrap_transition,
            1_700_000_001_000,
            1_700_000_031_000,
            "0x8762cd1008f09417f766ee880f2900d663ea9ebc583d1af2cc9cad805b321090c173dfb48b3092ca2c9423f36053391792fa4dd8c8f56dc1248feb647b66e707",
        );
        let bootstrap_certificate_digest =
            "0x059967c2178a72c178e894fe54ac74fccd657aec73f3b2ce48b9e68bae098a0b";
        let bootstrap = certificate_json(
            "bootstrap",
            1,
            &zero,
            "null",
            &bootstrap_endpoint,
            bootstrap_core,
            "miRHGGKJkFCxOqCOg7mUp9JHrUZhr8p4v9iQ19ScnBTFwHz+AdVd7d265usGMEw8t3K5zjBzxSQNErq5gCthAg==",
            bootstrap_operation,
            bootstrap_transition,
            &bootstrap_ack,
            "0x9605e695983da86a907c2a4b32ae322aa0c242fa0884d9049150007bc463c91b",
            "Frwgl6F5VvKIimo3DIrTHnbA8wyi/vZtfCtsyszLDPb2YKEOkg0aXZ/k0GJZ7of5VV8r4zCqsZ0T+9zri99qAg==",
            bootstrap_certificate_digest,
        );

        let rotation_event_root =
            "0x700dc376026a7a07f57de52b6fd6435dda77e94eebabd0af4b20ad86fd5a1ec1";
        let rotation_endpoint = endpoint_json(
            &repeated_bytes32(0x34),
            10,
            &repeated_bytes32(0x35),
            "0x17cb79fb2b4120f2b1ec65e4198d6e08b28e813feb01e4a400839b85e18080ce",
            "0x3532fb2b134488a2a18f2a07cd6db85b4c1b808ddb10972f0ad5526856177bd4",
            2,
            bootstrap_event_root,
            rotation_event_root,
            "0x6f3e877f41bbe4f8d09a91fc6644222c9eeca34abc4888e95ba7fe579d193c8a",
        );
        let rotation_core = "0x87ce928827c85744e9a01422c83e72ea1cf6f27c2694e4b1d93f519f8a905866";
        let rotation_operation =
            "0x39f50c5787e4decb56b87979062aa7e75f773b1693e584e8872a860da55fec93";
        let rotation_transition =
            "0x1a130d6da974f81cdbfdd923d9e53cf46cd9d81455b3c75e4cad1fe9c15b4385";
        let rotation_ack = target_acknowledgement_base64(
            &repeated_bytes32(0x35),
            &repeated_bytes32(0x34),
            &repeated_bytes32(0x35),
            2,
            bootstrap_event_root,
            rotation_event_root,
            rotation_operation,
            rotation_transition,
            1_700_000_002_000,
            1_700_000_032_000,
            "0x8db44faf1906059574c4b89e3120f3c4ea17ef2ab79f1c1dedeb5fb009910fa9932840bb4024884af8eee1272bfc96349140d896876f1f811d55995d29fd5809",
        );
        let rotation_certificate_digest =
            "0x0d571f120304487645bad248d866f7455ea50bf9b2bc6521de47151b7d671f35";
        let rotation = certificate_json(
            "rotation",
            2,
            bootstrap_certificate_digest,
            &bootstrap_endpoint,
            &rotation_endpoint,
            rotation_core,
            "t2+gDyrgsnCPICB2pQXjuMuN8Hpi98tm62jEnBjQU2C9i++dd2kKkWy3ZOBROgo5poxrJ2FdJvqxKgo24Xg/BQ==",
            rotation_operation,
            rotation_transition,
            &rotation_ack,
            "0x4e0cf22084defa424302981a0b24f62c5f2e8c214df242b47e6b96ce55715a82",
            "JJqfkvtE0sptpDowjfGhmYOZ6qR0XgGayEBH1zrlXaUXD+bwq+yZlolJ9T68ddXBjlZx1/xyELsjn8lhBIqLDA==",
            rotation_certificate_digest,
        );

        let adoption_event_root =
            "0x41c8323190639f9933e170e869a5bd322776e0928e24adfd4cfc9ae7829d6971";
        let adoption_endpoint = endpoint_json(
            &repeated_bytes32(0x45),
            10,
            &repeated_bytes32(0x46),
            "0xd759793bbc13a2819a827c76adb6fba8a49aee007f49f2d0992d99b825ad2c48",
            "0xf98d04c078515fd84c171fd3bb49a676cf3731d990aca999176b4f8796bac3a4",
            2,
            bootstrap_event_root,
            adoption_event_root,
            "0xd956aa1efb4412ce56c8d41ff1e356c5d01ac4355f7a947c9355c29cc18bcaec",
        );
        let adoption_core = "0xb8028e79579f400c80cd76101473e9a8e02be0644c5c4544a7a582103d5320a4";
        let adoption_operation =
            "0xc4dc774b78b391f173756130abb5412e62958f174783302396df673cfb70c30d";
        let adoption_transition =
            "0xbc3bc28337f870c07f41353d597ea38c96a2dd7f9f2f80536404e1e043296991";
        let adoption_ack = target_acknowledgement_base64(
            &repeated_bytes32(0x46),
            &repeated_bytes32(0x45),
            &repeated_bytes32(0x46),
            2,
            bootstrap_event_root,
            adoption_event_root,
            adoption_operation,
            adoption_transition,
            1_700_000_001_000,
            1_700_000_031_000,
            "0xf4a348469d714a26cc79bd54545e0ab98bab23c8082201e1dc050227a9dc61b2221f30861f07c63546aff73d9c07aed2758552d53634d76242c0f93e474a4c00",
        );
        let adoption_certificate_digest =
            "0xa8af21e3da145313f4f0357aa5de40a99e01af9faa8ff2cb680f9531ccb271dd";
        let adoption = certificate_json(
            "rotation",
            1,
            &zero,
            &bootstrap_endpoint,
            &adoption_endpoint,
            adoption_core,
            "lG+bbynkFb7ucEVrJfdmWjRt21uZeVm4ppC8XyRXAqa7tJ2B7a4hiPVWBLco8AvO6zrqFpGoiT4uIQHZZNxqBA==",
            adoption_operation,
            adoption_transition,
            &adoption_ack,
            "0x5ea3c16df6ad7ae8b5abbbe53d466fea5a5f1b3b4750d8def650647ca2ab2567",
            "avYz3s+8I6kZ0xizacvbdE9LqkF36Ddps+DkBp8wriWiBT4x2KNv623//ZLVMwZsWVh6CWbiyQM/XBamcKvmDw==",
            adoption_certificate_digest,
        );

        vec![
            SharedValidVector {
                name: "bootstrap",
                canonical_json: bootstrap,
                core_digest: &bootstrap_core[2..],
                operation_id: &bootstrap_operation[2..],
                transition_digest: &bootstrap_transition[2..],
                final_digest: "0ae35d27aa3b74817a5dd99dcb1355975a74df6d5bcdda14288382eb8067535e",
                certificate_digest: &bootstrap_certificate_digest[2..],
                canonical_json_sha256:
                    "ea80af7129eb2a52d3116007d9caab2f5bf9df2edbc7d29811ff275b7f6d0412",
            },
            SharedValidVector {
                name: "rotation",
                canonical_json: rotation,
                core_digest: &rotation_core[2..],
                operation_id: &rotation_operation[2..],
                transition_digest: &rotation_transition[2..],
                final_digest: "19f98af5fb9e017470594782c632eeaabf352c681523c626d6f6bb59b43318a2",
                certificate_digest: &rotation_certificate_digest[2..],
                canonical_json_sha256:
                    "8f834f9b335854218f07fcd2af0fafb311ac046b026f5436c7fe89ff24c9ade8",
            },
            SharedValidVector {
                name: "adoption",
                canonical_json: adoption,
                core_digest: &adoption_core[2..],
                operation_id: &adoption_operation[2..],
                transition_digest: &adoption_transition[2..],
                final_digest: "aa69c3894ad0118c71189352565122950d2dfa5a20337f48c53c750dddae5969",
                certificate_digest: &adoption_certificate_digest[2..],
                canonical_json_sha256:
                    "e8b0fe13efbb0c667576411836c2f6ac22df1eb70904cb0d796ae0b3dfc06cdb",
            },
        ]
    }

    #[test]
    fn go_shared_valid_certificate_vectors_match_rust_verification_and_transcripts() {
        for vector in shared_valid_vectors() {
            let wire: StateAnchorTrustCertificate =
                serde_json::from_slice(vector.canonical_json.as_bytes()).unwrap_or_else(|error| {
                    panic!(
                        "strict {} certificate JSON failed to decode: {error}",
                        vector.name
                    )
                });
            assert_eq!(
                serde_json::to_vec(&wire).expect("certificate serialization"),
                vector.canonical_json.as_bytes(),
                "{} canonical certificate JSON changed",
                vector.name
            );

            let verified = verify_state_anchor_trust_certificate(wire).unwrap_or_else(|error| {
                panic!(
                    "Go shared {} certificate was rejected: {error}",
                    vector.name
                )
            });
            assert_eq!(hex::encode(verified.core_digest), vector.core_digest);
            assert_eq!(hex::encode(verified.operation_id), vector.operation_id);
            assert_eq!(
                hex::encode(verified.transition_digest),
                vector.transition_digest
            );
            assert_eq!(
                hex::encode(trust_certificate_final_digest(
                    &verified.core_digest,
                    &verified.core_signature,
                    &verified.operation_id,
                    &verified.transition_digest,
                    &verified.to.reference,
                    &verified.target_acknowledgement_sha256,
                )),
                vector.final_digest
            );
            assert_eq!(
                hex::encode(verified.certificate_digest),
                vector.certificate_digest
            );
            assert_eq!(
                hex::encode(Sha256::digest(vector.canonical_json.as_bytes())),
                vector.canonical_json_sha256,
                "{} canonical JSON SHA-256 changed",
                vector.name
            );
        }
    }

    #[test]
    fn certificate_endpoints_reject_non_prime_subgroup_online_and_offline_keys() {
        let vector = shared_valid_vectors().remove(0);
        let template: StateAnchorTrustCertificate =
            serde_json::from_slice(vector.canonical_json.as_bytes())
                .expect("shared bootstrap certificate");
        let corpus = [
            (
                "identity",
                "0100000000000000000000000000000000000000000000000000000000000000",
            ),
            (
                "order-four",
                "0000000000000000000000000000000000000000000000000000000000000000",
            ),
            (
                "mixed-order",
                "9970c93c125fd998ebc1642abe30619e2fd971dbcbeaeb8ccfe919cbfd13b6cf",
            ),
        ];
        for (name, encoded) in corpus {
            let bytes: [u8; 32] = hex::decode(encoded)
                .expect("vector hex")
                .try_into()
                .expect("32-byte vector");
            let spki = bytes32_hex(ed25519_spki_sha256(&bytes));
            // The all-zero encoding is rejected by the reserved-value check
            // before point decompression; the other vectors decompress and
            // fail the prime-subgroup check.
            let expected_fragment = if name == "order-four" {
                "must be nonzero"
            } else {
                "prime-subgroup"
            };

            let mut weak_online = template.clone();
            weak_online.to.response_public_key = bytes32_hex(bytes);
            weak_online.to.response_public_key_spki_sha256 = spki.clone();
            let error = verify_state_anchor_trust_certificate(weak_online)
                .err()
                .unwrap_or_else(|| panic!("{name} online key must be rejected"));
            assert!(
                error.to_string().contains(expected_fragment),
                "unexpected {name} online error: {error}"
            );

            let mut weak_offline = template.clone();
            weak_offline.to.offline_authority_public_key = bytes32_hex(bytes);
            weak_offline.to.offline_authority_spki_sha256 = spki;
            let error = verify_state_anchor_trust_certificate(weak_offline)
                .err()
                .unwrap_or_else(|| panic!("{name} offline key must be rejected"));
            assert!(
                error.to_string().contains(expected_fragment),
                "unexpected {name} offline error: {error}"
            );
        }
    }

    #[test]
    fn multi_certificate_journal_recovers_every_cow_commit_prefix() {
        let vectors = shared_valid_vectors();
        let certificates: Vec<_> = vectors[..2]
            .iter()
            .map(|vector| {
                let wire: StateAnchorTrustCertificate =
                    serde_json::from_slice(vector.canonical_json.as_bytes())
                        .expect("shared certificate JSON");
                verify_state_anchor_trust_certificate(wire).expect("shared certificate verifies")
            })
            .collect();
        let store_fingerprint = certificates[0].signer_store_fingerprint;
        let mut bytes = encode_state_anchor_trust_journal_header(&store_fingerprint);
        let mut journal =
            parse_state_anchor_trust_journal(&bytes, &store_fingerprint).expect("header");
        assert!(journal.committed.is_empty());
        assert!(journal.pending.is_empty());

        for (index, certificate) in certificates.iter().enumerate() {
            let record = encode_state_anchor_trust_prepare_record(
                &store_fingerprint,
                &journal.last_record_commitment,
                certificate,
            )
            .expect("PREPARE record");
            bytes.extend_from_slice(&record);
            journal = parse_state_anchor_trust_journal(&bytes, &store_fingerprint)
                .expect("PREPARE prefix parses");
            assert_eq!(journal.committed.len(), 0);
            assert_eq!(journal.pending.len(), index + 1);
        }

        let first_commit = encode_state_anchor_trust_commit_record(
            &store_fingerprint,
            &journal.last_record_commitment,
            &certificates[0],
        )
        .expect("first COMMIT");
        bytes.extend_from_slice(&first_commit);
        journal = parse_state_anchor_trust_journal(&bytes, &store_fingerprint)
            .expect("partial COMMIT prefix parses");
        assert_eq!(journal.committed.len(), 1);
        assert_eq!(journal.pending.len(), 1);
        assert_eq!(journal.committed[0].wire, certificates[0].wire);
        assert_eq!(journal.pending[0].wire, certificates[1].wire);

        let second_commit = encode_state_anchor_trust_commit_record(
            &store_fingerprint,
            &journal.last_record_commitment,
            &certificates[1],
        )
        .expect("second COMMIT");
        bytes.extend_from_slice(&second_commit);
        journal = parse_state_anchor_trust_journal(&bytes, &store_fingerprint)
            .expect("complete COMMIT batch parses");
        assert_eq!(journal.committed.len(), 2);
        assert!(journal.pending.is_empty());
        assert_eq!(journal.committed[1].wire, certificates[1].wire);
    }

    #[test]
    fn descendant_reference_allows_later_revision_but_bounds_restart_history() {
        let vector = shared_valid_vectors().remove(0);
        let wire: StateAnchorTrustCertificate =
            serde_json::from_slice(vector.canonical_json.as_bytes()).expect("bootstrap JSON");
        let certificate = verify_state_anchor_trust_certificate(wire).expect("bootstrap verifies");
        let floor = certificate.to.reference;

        let mut later = floor.clone();
        later.revision += 1;
        later.previous_event_root = floor.event_root;
        later.event_root = [0x71; 32];
        later.checkpoint_ack_digest = [0x72; 32];
        validate_state_anchor_trust_reference_descendant(&floor, &later, "later")
            .expect("same-checkpoint later revision is a descendant");

        let mut forked_checkpoint = later.clone();
        forked_checkpoint.checkpoint.state_image_digest[0] ^= 1;
        assert!(validate_state_anchor_trust_reference_descendant(
            &floor,
            &forked_checkpoint,
            "fork"
        )
        .is_err());

        let mut at_bound = later.clone();
        at_bound.revision = floor.revision + STATE_ANCHOR_TRUST_MAX_REVISION_DISTANCE;
        validate_state_anchor_trust_reference_descendant(&floor, &at_bound, "at-bound")
            .expect("revision exactly at the certified restart-history bound is accepted");

        let mut too_far = later;
        too_far.revision = floor.revision + STATE_ANCHOR_TRUST_MAX_REVISION_DISTANCE + 1;
        assert!(
            validate_state_anchor_trust_reference_descendant(&floor, &too_far, "too-far").is_err()
        );
    }
}
