//! Retained FROST key-package readiness and dynamic state-witness readback.

use super::*;

use crate::api::{
    RetainedKeyPackageInventoryEntry, RetainedKeyPackageInventoryPackage,
    RetainedKeyPackageInventoryResult, StateWitnessProofEntry, StateWitnessProofRequest,
    StateWitnessProofResult,
};

pub(crate) const TBTC_SIGNER_RETAINED_KEY_PACKAGE_INVENTORY_SCHEMA: &str =
    "tbtc-signer-retained-key-package-inventory/v1";
pub(crate) const TBTC_SIGNER_STATE_WITNESS_PROOF_REQUEST_SCHEMA: &str =
    "tbtc-signer-state-witness-proof-request/v1";
pub(crate) const TBTC_SIGNER_STATE_WITNESS_PROOF_SCHEMA: &str =
    "tbtc-signer-state-witness-proof/v1";

const INVENTORY_COMMITMENT_DOMAIN: &[u8] =
    b"tbtc-signer-retained-key-package-inventory-commitment-v1\0";
const PUBLIC_KEY_PACKAGE_COMMITMENT_DOMAIN: &[u8] =
    b"tbtc-signer-retained-public-key-package-commitment-v1\0";
const KEY_PACKAGE_COMMITMENT_DOMAIN: &[u8] = b"tbtc-signer-retained-key-package-commitment-v1\0";

#[derive(Clone)]
struct ValidatedInventoryPackage {
    participant_seat: u16,
    commitment: [u8; 32],
}

#[derive(Clone)]
struct ValidatedInventoryEntry {
    wallet_id: [u8; 32],
    key_group: String,
    threshold: u16,
    participant_count: u16,
    share_epoch: u64,
    public_key_package_commitment: [u8; 32],
    key_packages: Vec<ValidatedInventoryPackage>,
}

pub(crate) fn retained_key_package_inventory(
) -> Result<RetainedKeyPackageInventoryResult, EngineError> {
    // Keep the engine guard through store-tip capture. Every state mutation
    // takes ENGINE_STATE before the store mutex, so this order is deadlock-free
    // and prevents inventory from being paired with another mutation's tip.
    let engine = state()?;
    let guard = engine
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;

    let mut validated_entries = Vec::new();
    for (session_id, session) in &guard.sessions {
        let Some(dkg_result) = session.dkg_result.as_ref() else {
            if session.dkg_key_packages.is_some() || session.dkg_public_key_package.is_some() {
                return Err(EngineError::Internal(format!(
                    "session [{session_id}] has retained DKG material without a DKG result"
                )));
            }
            continue;
        };
        validated_entries.push(validate_inventory_entry(session_id, session, dkg_result)?);
    }
    validated_entries.sort_by_key(|entry| entry.wallet_id);
    for pair in validated_entries.windows(2) {
        if pair[0].wallet_id == pair[1].wallet_id {
            return Err(EngineError::Internal(format!(
                "multiple retained sessions claim wallet [{}]",
                bytes32_hex(pair[0].wallet_id)
            )));
        }
    }
    let inventory_commitment = compute_inventory_commitment(&validated_entries)?;

    let (store_identity, state_tip) = with_state_file_lock(|store| {
        let identity = store.identity()?;
        let tip = store.state_witness_tip()?;
        Ok((identity, tip))
    })?;

    let entries = validated_entries
        .into_iter()
        .map(|entry| RetainedKeyPackageInventoryEntry {
            wallet_id: bytes32_hex(entry.wallet_id),
            key_group: entry.key_group,
            threshold: entry.threshold,
            participant_count: entry.participant_count,
            share_epoch: entry.share_epoch,
            public_key_package_commitment: bytes32_hex(entry.public_key_package_commitment),
            key_packages: entry
                .key_packages
                .into_iter()
                .map(|package| RetainedKeyPackageInventoryPackage {
                    participant_seat: package.participant_seat,
                    key_package_commitment: bytes32_hex(package.commitment),
                })
                .collect(),
        })
        .collect();

    Ok(RetainedKeyPackageInventoryResult {
        schema: TBTC_SIGNER_RETAINED_KEY_PACKAGE_INVENTORY_SCHEMA.to_string(),
        store_fingerprint: bytes32_hex(store_identity.fingerprint),
        state_generation: state_tip.generation,
        state_commitment: bytes32_hex(state_tip.commitment),
        previous_state_commitment: bytes32_hex(state_tip.previous_commitment),
        state_image_digest: bytes32_hex(state_tip.state_image_digest),
        inventory_commitment: bytes32_hex(inventory_commitment),
        entries,
    })
}

pub(crate) fn state_witness_proof(
    request: StateWitnessProofRequest,
) -> Result<StateWitnessProofResult, EngineError> {
    if request.schema != TBTC_SIGNER_STATE_WITNESS_PROOF_REQUEST_SCHEMA {
        return Err(EngineError::Validation(
            "unsupported state witness proof request schema".to_string(),
        ));
    }
    if request.ancestor_generation == 0 || request.target_generation == 0 {
        return Err(EngineError::Validation(
            "state witness proof generations must be positive".to_string(),
        ));
    }
    if request.maximum_entries == 0 || request.maximum_entries > 256 {
        return Err(EngineError::Validation(
            "state witness proof maximumEntries must be between 1 and 256".to_string(),
        ));
    }
    let requested_store_fingerprint =
        parse_bytes32(&request.store_fingerprint, "storeFingerprint")?;
    let ancestor_commitment = parse_bytes32(&request.ancestor_commitment, "ancestorCommitment")?;
    let target_commitment = parse_bytes32(&request.target_commitment, "targetCommitment")?;

    let (actual_store_fingerprint, entries, complete) = with_state_file_lock(|store| {
        let identity = store.identity()?;
        if identity.fingerprint != requested_store_fingerprint {
            return Err(EngineError::Validation(
                "state witness proof storeFingerprint does not match the active store".to_string(),
            ));
        }
        let (entries, complete) = store.state_witness_proof(
            request.ancestor_generation,
            ancestor_commitment,
            request.target_generation,
            target_commitment,
            request.maximum_entries as usize,
        )?;
        Ok((identity.fingerprint, entries, complete))
    })?;

    Ok(StateWitnessProofResult {
        schema: TBTC_SIGNER_STATE_WITNESS_PROOF_SCHEMA.to_string(),
        store_fingerprint: bytes32_hex(actual_store_fingerprint),
        ancestor_generation: request.ancestor_generation,
        ancestor_commitment: bytes32_hex(ancestor_commitment),
        target_generation: request.target_generation,
        target_commitment: bytes32_hex(target_commitment),
        complete,
        entries: entries
            .into_iter()
            .map(|entry| StateWitnessProofEntry {
                generation: entry.generation,
                previous_state_commitment: bytes32_hex(entry.previous_commitment),
                state_commitment: bytes32_hex(entry.commitment),
                state_image_digest: bytes32_hex(entry.state_image_digest),
            })
            .collect(),
    })
}

fn validate_inventory_entry(
    session_id: &str,
    session: &SessionState,
    dkg_result: &DkgResult,
) -> Result<ValidatedInventoryEntry, EngineError> {
    if dkg_result.session_id != session_id {
        return Err(EngineError::Internal(format!(
            "retained DKG result session [{}] does not match owner session [{session_id}]",
            dkg_result.session_id
        )));
    }
    if session.dkg_share_epoch != 0 {
        return Err(EngineError::Internal(format!(
            "retained wallet [{}] has unsupported key-package share epoch [{}]",
            dkg_result.key_group, session.dkg_share_epoch
        )));
    }
    let (wallet_id, compressed_key_group) = parse_key_group(&dkg_result.key_group)?;
    if dkg_result.threshold < 2 || dkg_result.participant_count < dkg_result.threshold {
        return Err(EngineError::Internal(format!(
            "retained wallet [{}] has invalid threshold/participant count",
            dkg_result.key_group
        )));
    }
    let public_package = session.dkg_public_key_package.as_ref().ok_or_else(|| {
        EngineError::Internal(format!(
            "retained wallet [{}] is missing its public key package",
            dkg_result.key_group
        ))
    })?;
    let public_verifying_key = public_package
        .verifying_key()
        .serialize()
        .map_err(|error| {
            EngineError::Internal(format!(
                "failed to serialize retained wallet verifying key: {error}"
            ))
        })?;
    if public_verifying_key.as_slice() != compressed_key_group {
        return Err(EngineError::Internal(format!(
            "retained wallet key group [{}] differs from its public key package",
            dkg_result.key_group
        )));
    }
    if public_package.verifying_shares().len() != dkg_result.participant_count as usize {
        return Err(EngineError::Internal(format!(
            "retained wallet [{}] participant count differs from its public package",
            dkg_result.key_group
        )));
    }

    let key_packages = session.dkg_key_packages.as_ref().ok_or_else(|| {
        EngineError::Internal(format!(
            "retained wallet [{}] has no local key packages",
            dkg_result.key_group
        ))
    })?;
    if key_packages.is_empty() {
        return Err(EngineError::Internal(format!(
            "retained wallet [{}] has an empty local key-package set",
            dkg_result.key_group
        )));
    }

    let public_serialized = public_package.serialize().map_err(|error| {
        EngineError::Internal(format!(
            "failed to serialize retained public key package: {error}"
        ))
    })?;
    let public_key_package_commitment = public_key_package_commitment(
        &wallet_id,
        &dkg_result.key_group,
        dkg_result.threshold,
        dkg_result.participant_count,
        session.dkg_share_epoch,
        &public_serialized,
    );

    let mut packages = Vec::with_capacity(key_packages.len());
    for (participant_seat, key_package) in key_packages {
        if *participant_seat == 0 {
            return Err(EngineError::Internal(format!(
                "retained wallet [{}] has zero participant seat",
                dkg_result.key_group
            )));
        }
        let expected_identifier = participant_identifier_to_frost_identifier(*participant_seat)?;
        if *key_package.identifier() != expected_identifier {
            return Err(EngineError::Internal(format!(
                "retained wallet [{}] seat [{}] key-package identifier mismatch",
                dkg_result.key_group, participant_seat
            )));
        }
        if *key_package.min_signers() != dkg_result.threshold
            || key_package.verifying_key() != public_package.verifying_key()
        {
            return Err(EngineError::Internal(format!(
                "retained wallet [{}] seat [{}] key package has incompatible threshold or group key",
                dkg_result.key_group, participant_seat
            )));
        }
        let expected_share = public_package
            .verifying_shares()
            .get(&expected_identifier)
            .ok_or_else(|| {
                EngineError::Internal(format!(
                    "retained wallet [{}] seat [{}] is absent from its public package",
                    dkg_result.key_group, participant_seat
                ))
            })?;
        if key_package.verifying_share() != expected_share {
            return Err(EngineError::Internal(format!(
                "retained wallet [{}] seat [{}] verifying share mismatch",
                dkg_result.key_group, participant_seat
            )));
        }
        let mut signing_share = *key_package.signing_share();
        let derives = frost::keys::VerifyingShare::from(signing_share) == *expected_share;
        signing_share.zeroize();
        if !derives {
            return Err(EngineError::Internal(format!(
                "retained wallet [{}] seat [{}] signing share is not ready",
                dkg_result.key_group, participant_seat
            )));
        }

        let identifier_bytes = key_package.identifier().serialize();
        let verifying_share_bytes = key_package.verifying_share().serialize().map_err(|error| {
            EngineError::Internal(format!(
                "failed to serialize retained wallet verifying share: {error}"
            ))
        })?;
        packages.push(ValidatedInventoryPackage {
            participant_seat: *participant_seat,
            commitment: key_package_commitment(
                &wallet_id,
                &dkg_result.key_group,
                *participant_seat,
                session.dkg_share_epoch,
                identifier_bytes.as_ref(),
                verifying_share_bytes.as_ref(),
                &public_verifying_key,
                *key_package.min_signers(),
            ),
        });
    }
    packages.sort_by_key(|package| package.participant_seat);

    Ok(ValidatedInventoryEntry {
        wallet_id,
        key_group: dkg_result.key_group.clone(),
        threshold: dkg_result.threshold,
        participant_count: dkg_result.participant_count,
        share_epoch: session.dkg_share_epoch,
        public_key_package_commitment,
        key_packages: packages,
    })
}

fn parse_key_group(key_group: &str) -> Result<([u8; 32], [u8; 33]), EngineError> {
    if key_group.len() != 66 || key_group != key_group.to_ascii_lowercase() {
        return Err(EngineError::Internal(
            "retained wallet key group is not canonical lowercase compressed SEC1 hex".to_string(),
        ));
    }
    let bytes = hex::decode(key_group).map_err(|_| {
        EngineError::Internal("retained wallet key group is not valid hex".to_string())
    })?;
    let public_key = bitcoin::secp256k1::PublicKey::from_slice(&bytes).map_err(|_| {
        EngineError::Internal(
            "retained wallet key group is not a compressed secp256k1 public key".to_string(),
        )
    })?;
    let mut compressed = [0u8; 33];
    compressed.copy_from_slice(&bytes);
    if public_key.serialize() != compressed {
        return Err(EngineError::Internal(
            "retained wallet key group is not canonical compressed SEC1".to_string(),
        ));
    }
    let (x_only, _) = public_key.x_only_public_key();
    Ok((x_only.serialize(), compressed))
}

fn parse_bytes32(value: &str, label: &str) -> Result<[u8; 32], EngineError> {
    if value.len() != 66 || !value.starts_with("0x") || value != value.to_ascii_lowercase() {
        return Err(EngineError::Validation(format!(
            "{label} must be canonical lowercase 0x-prefixed bytes32"
        )));
    }
    let decoded = hex::decode(&value[2..]).map_err(|_| {
        EngineError::Validation(format!(
            "{label} must be canonical lowercase 0x-prefixed bytes32"
        ))
    })?;
    let mut result = [0u8; 32];
    result.copy_from_slice(&decoded);
    if result == [0u8; 32] {
        return Err(EngineError::Validation(format!("{label} must not be zero")));
    }
    Ok(result)
}

pub(crate) fn bytes32_hex(value: [u8; 32]) -> String {
    format!("0x{}", hex::encode(value))
}

fn public_key_package_commitment(
    wallet_id: &[u8; 32],
    key_group: &str,
    threshold: u16,
    participant_count: u16,
    share_epoch: u64,
    serialized_public_package: &[u8],
) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(PUBLIC_KEY_PACKAGE_COMMITMENT_DOMAIN);
    digest.update(wallet_id);
    write_length_prefixed(&mut digest, key_group.as_bytes());
    digest.update(threshold.to_be_bytes());
    digest.update(participant_count.to_be_bytes());
    digest.update(share_epoch.to_be_bytes());
    write_length_prefixed(&mut digest, serialized_public_package);
    digest.finalize().into()
}

#[allow(clippy::too_many_arguments)]
fn key_package_commitment(
    wallet_id: &[u8; 32],
    key_group: &str,
    participant_seat: u16,
    share_epoch: u64,
    identifier: &[u8],
    verifying_share: &[u8],
    verifying_key: &[u8],
    min_signers: u16,
) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(KEY_PACKAGE_COMMITMENT_DOMAIN);
    digest.update(wallet_id);
    write_length_prefixed(&mut digest, key_group.as_bytes());
    digest.update(participant_seat.to_be_bytes());
    digest.update(share_epoch.to_be_bytes());
    write_length_prefixed(&mut digest, identifier);
    write_length_prefixed(&mut digest, verifying_share);
    write_length_prefixed(&mut digest, verifying_key);
    digest.update(min_signers.to_be_bytes());
    digest.finalize().into()
}

fn compute_inventory_commitment(
    entries: &[ValidatedInventoryEntry],
) -> Result<[u8; 32], EngineError> {
    let entry_count = u32::try_from(entries.len()).map_err(|_| {
        EngineError::Internal("retained key-package inventory has too many entries".to_string())
    })?;
    let mut digest = Sha256::new();
    digest.update(INVENTORY_COMMITMENT_DOMAIN);
    digest.update(entry_count.to_be_bytes());
    for entry in entries {
        digest.update(entry.wallet_id);
        write_length_prefixed(&mut digest, entry.key_group.as_bytes());
        digest.update(entry.threshold.to_be_bytes());
        digest.update(entry.participant_count.to_be_bytes());
        digest.update(entry.share_epoch.to_be_bytes());
        digest.update(entry.public_key_package_commitment);
        let package_count = u32::try_from(entry.key_packages.len()).map_err(|_| {
            EngineError::Internal(
                "retained key-package inventory entry has too many packages".to_string(),
            )
        })?;
        digest.update(package_count.to_be_bytes());
        for package in &entry.key_packages {
            digest.update(package.participant_seat.to_be_bytes());
            digest.update(package.commitment);
        }
    }
    Ok(digest.finalize().into())
}

fn write_length_prefixed(destination: &mut Sha256, value: &[u8]) {
    destination.update((value.len() as u32).to_be_bytes());
    destination.update(value);
}

#[cfg(test)]
mod inventory_transcript_tests {
    use super::*;

    #[test]
    fn inventory_commitment_matches_frozen_go_v1_vector() {
        let entries = vec![ValidatedInventoryEntry {
            wallet_id: [0x11; 32],
            key_group: format!("02{}", "11".repeat(32)),
            threshold: 2,
            participant_count: 3,
            share_epoch: 0,
            public_key_package_commitment: [0x33; 32],
            key_packages: vec![
                ValidatedInventoryPackage {
                    participant_seat: 1,
                    commitment: [0x44; 32],
                },
                ValidatedInventoryPackage {
                    participant_seat: 3,
                    commitment: [0x55; 32],
                },
            ],
        }];

        assert_eq!(
            hex::encode(compute_inventory_commitment(&entries).expect("inventory commitment")),
            "bd6ec36fa27a57dd9926883bb2ff4dee7ececd28de940df7294f0e0f0dedd150"
        );
    }
}
