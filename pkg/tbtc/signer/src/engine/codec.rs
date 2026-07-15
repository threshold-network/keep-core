// Hex/struct codecs and Go<->frost identifier conversions.

use super::*;

pub(crate) fn now_unix() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0)
}

pub(crate) fn hash_hex(bytes: &[u8]) -> String {
    hex::encode(hash_bytes(bytes))
}

pub(crate) fn hash_bytes(bytes: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(bytes);
    let digest = hasher.finalize();

    let mut output = [0u8; 32];
    output.copy_from_slice(&digest);
    output
}

// Test-only: the production coarse-FROST callers (round-id/seed derivation)
// were removed with the transitional signing path; only unit tests still
// exercise this length-prefixed domain-separation helper.
#[cfg(test)]
pub(crate) fn deterministic_seed(parts: &[&[u8]]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    for part in parts {
        // Length-prefix each part so embedded 0x00 bytes cannot blur boundaries.
        hasher.update((part.len() as u64).to_le_bytes());
        hasher.update(part);
    }

    let digest = hasher.finalize();
    let mut output = [0u8; 32];
    output.copy_from_slice(&digest);
    output
}

pub(crate) fn participant_identifier_to_frost_identifier(
    participant_identifier: u16,
) -> Result<frost::Identifier, EngineError> {
    participant_identifier.try_into().map_err(|e| {
        EngineError::Validation(format!(
            "invalid participant identifier [{}]: {e}",
            participant_identifier
        ))
    })
}

pub(crate) fn frost_identifier_to_go_string(identifier: frost::Identifier) -> String {
    serde_json::to_string(&hex::encode(identifier.serialize()))
        .expect("serializing hex identifier as JSON string cannot fail")
}

/// Map a FROST aggregate error to the CANDIDATE culprits it identifies, as u16
/// Go member identifiers (the same identifier space as
/// `excluded_member_identifiers`, so the Go host consumes them directly).
///
/// Returns the members FROST flagged for an invalid signature share
/// (`Error::InvalidSignatureShare`, the full set under
/// `CheaterDetection::AllCheaters`). Every other error class - malformed
/// package, wrong share count, group/field errors - yields an empty list: those
/// are not per-member share attributions, so the caller surfaces them as a
/// generic validation failure instead. Identifiers that do not map to a u16 are
/// dropped: they cannot belong to a real group member (every submitted share
/// carries a u16-derived identifier), so they are foreign to the Go host's
/// member set. CANDIDATES only - pure FROST verdicts, not adjudicated fault.
pub(crate) fn aggregate_candidate_culprits(error: &frost::Error) -> Vec<u16> {
    match error {
        frost_core::Error::InvalidSignatureShare { culprits } => culprits
            .iter()
            .filter_map(|identifier| frost_identifier_to_u16(*identifier))
            .collect(),
        _ => Vec::new(),
    }
}

/// Recover the u16 Go member identifier from a FROST participant identifier -
/// the inverse of `participant_identifier_to_frost_identifier`. FROST(secp256k1)
/// serializes the scalar big-endian, so this requires every byte above the low
/// two to be zero and reads the trailing two big-endian. Returns None for an
/// identifier that does not fit a u16.
pub(crate) fn frost_identifier_to_u16(identifier: frost::Identifier) -> Option<u16> {
    let bytes = identifier.serialize();
    let split = bytes.len().checked_sub(2)?;
    if bytes[..split].iter().any(|&b| b != 0) {
        return None;
    }
    Some(u16::from_be_bytes([bytes[split], bytes[split + 1]]))
}

pub(crate) fn parse_frost_identifier(
    operation: &str,
    field_name: &str,
    raw_identifier: &str,
) -> Result<frost::Identifier, EngineError> {
    if raw_identifier.trim().is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: {field_name} is empty"
        )));
    }

    let trimmed = raw_identifier.trim();
    let normalized_hex = if trimmed.starts_with('"') {
        serde_json::from_str::<String>(trimmed).map_err(|e| {
            EngineError::Validation(format!(
                "{operation}: {field_name} must be a JSON string-wrapped hex identifier: {e}"
            ))
        })?
    } else {
        trimmed.to_string()
    };

    let bytes = hex::decode(&normalized_hex).map_err(|_| {
        EngineError::Validation(format!(
            "{operation}: {field_name} must be a hex-encoded FROST identifier"
        ))
    })?;

    frost::Identifier::deserialize(&bytes)
        .map_err(|e| EngineError::Validation(format!("{operation}: invalid {field_name}: {e}")))
}

pub(crate) fn decode_hex_field(
    operation: &str,
    field_name: &str,
    value: &str,
) -> Result<Vec<u8>, EngineError> {
    if value.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: {field_name} is empty"
        )));
    }

    hex::decode(value).map_err(|_| {
        EngineError::Validation(format!("{operation}: {field_name} must be valid hex"))
    })
}

pub(crate) fn zeroizing_rng_from_os() -> ZeroizingChaCha20Rng {
    let mut seed = [0u8; 32];
    OsRng.fill_bytes(&mut seed);
    let rng = ZeroizingChaCha20Rng::from_seed(seed);
    seed.zeroize();
    rng
}

pub(crate) fn decode_round1_package_map(
    operation: &str,
    packages: &[DkgRound1Package],
) -> Result<BTreeMap<frost::Identifier, frost::keys::dkg::round1::Package>, EngineError> {
    if packages.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: round1_packages must not be empty"
        )));
    }

    let mut package_map = BTreeMap::new();
    for (index, package) in packages.iter().enumerate() {
        let identifier = parse_frost_identifier(
            operation,
            &format!("round1_packages[{index}].identifier"),
            &package.identifier,
        )?;
        let package_bytes = decode_hex_field(
            operation,
            &format!("round1_packages[{index}].package_hex"),
            &package.package_hex,
        )?;
        let round1_package = frost::keys::dkg::round1::Package::deserialize(&package_bytes)
            .map_err(|e| {
                EngineError::Validation(format!(
                    "{operation}: invalid round1 package [{index}]: {e}"
                ))
            })?;

        if package_map.insert(identifier, round1_package).is_some() {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate round1 package identifier [{}]",
                package.identifier
            )));
        }
    }

    Ok(package_map)
}

pub(crate) fn decode_round2_package_map(
    operation: &str,
    packages: &[DkgRound2Package],
    expected_recipient: Option<frost::Identifier>,
) -> Result<BTreeMap<frost::Identifier, frost::keys::dkg::round2::Package>, EngineError> {
    if packages.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: round2_packages must not be empty"
        )));
    }

    let mut package_map = BTreeMap::new();
    for (index, package) in packages.iter().enumerate() {
        let recipient_identifier = parse_frost_identifier(
            operation,
            &format!("round2_packages[{index}].identifier"),
            &package.identifier,
        )?;
        if let Some(expected_recipient) = expected_recipient {
            if recipient_identifier != expected_recipient {
                return Err(EngineError::Validation(format!(
                    "{operation}: round2 package [{index}] recipient identifier does not match local DKG participant"
                )));
            }
        }

        let sender_identifier = package.sender_identifier.as_ref().ok_or_else(|| {
            EngineError::Validation(format!(
                "{operation}: round2_packages[{index}].sender_identifier is empty"
            ))
        })?;
        let sender_identifier = parse_frost_identifier(
            operation,
            &format!("round2_packages[{index}].sender_identifier"),
            sender_identifier,
        )?;
        let mut package_bytes = decode_hex_field(
            operation,
            &format!("round2_packages[{index}].package_hex"),
            package.package_hex.expose_secret(),
        )?;
        let round2_package_result = frost::keys::dkg::round2::Package::deserialize(&package_bytes);
        package_bytes.zeroize();
        let round2_package = round2_package_result.map_err(|e| {
            EngineError::Validation(format!(
                "{operation}: invalid round2 package [{index}]: {e}"
            ))
        })?;

        if package_map
            .insert(sender_identifier, round2_package)
            .is_some()
        {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate round2 package sender identifier"
            )));
        }
    }

    Ok(package_map)
}

pub(crate) fn x_only_verifying_key_hex(
    public_key_package: &frost::keys::PublicKeyPackage,
) -> Result<String, EngineError> {
    let compressed = public_key_package
        .verifying_key()
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize verifying key: {e}")))?;

    if compressed.len() != 33 || compressed[0] != 0x02 {
        return Err(EngineError::Internal(
            "expected even-Y compressed FROST verifying key".to_string(),
        ));
    }

    Ok(hex::encode(&compressed[1..]))
}

pub(crate) fn native_public_key_package_from_frost(
    public_key_package: &frost::keys::PublicKeyPackage,
) -> Result<NativeFrostPublicKeyPackage, EngineError> {
    let mut verifying_shares = BTreeMap::new();
    for (identifier, verifying_share) in public_key_package.verifying_shares() {
        let share_bytes = verifying_share.serialize().map_err(|e| {
            EngineError::Internal(format!("failed to serialize verifying share: {e}"))
        })?;
        verifying_shares.insert(
            frost_identifier_to_go_string(*identifier),
            hex::encode(share_bytes),
        );
    }

    Ok(NativeFrostPublicKeyPackage {
        verifying_shares,
        verifying_key: x_only_verifying_key_hex(public_key_package)?,
    })
}

pub(crate) fn native_public_key_package_to_frost(
    operation: &str,
    public_key_package: &NativeFrostPublicKeyPackage,
) -> Result<frost::keys::PublicKeyPackage, EngineError> {
    if public_key_package.verifying_key.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: public_key_package.verifying_key is empty"
        )));
    }
    if public_key_package.verifying_shares.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: public_key_package.verifying_shares is empty"
        )));
    }

    let mut verifying_key_bytes = decode_hex_field(
        operation,
        "public_key_package.verifying_key",
        &public_key_package.verifying_key,
    )?;
    if verifying_key_bytes.len() != 32 {
        verifying_key_bytes.zeroize();
        return Err(EngineError::Validation(format!(
            "{operation}: public_key_package.verifying_key must be a 32-byte x-only key"
        )));
    }

    let mut compressed_verifying_key = Vec::with_capacity(33);
    compressed_verifying_key.push(0x02);
    compressed_verifying_key.extend_from_slice(&verifying_key_bytes);
    verifying_key_bytes.zeroize();
    let verifying_key =
        frost::VerifyingKey::deserialize(&compressed_verifying_key).map_err(|e| {
            EngineError::Validation(format!(
                "{operation}: invalid public_key_package.verifying_key: {e}"
            ))
        })?;
    compressed_verifying_key.zeroize();

    let mut verifying_shares = BTreeMap::new();
    for (identifier, share_hex) in &public_key_package.verifying_shares {
        let identifier = parse_frost_identifier(
            operation,
            "public_key_package.verifying_shares identifier",
            identifier,
        )?;
        let share_bytes = decode_hex_field(
            operation,
            "public_key_package.verifying_shares value",
            share_hex,
        )?;
        let verifying_share =
            frost::keys::VerifyingShare::deserialize(&share_bytes).map_err(|e| {
                EngineError::Validation(format!(
                    "{operation}: invalid public_key_package verifying share: {e}"
                ))
            })?;
        if verifying_shares
            .insert(identifier, verifying_share)
            .is_some()
        {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate public_key_package verifying share identifier"
            )));
        }
    }

    Ok(frost::keys::PublicKeyPackage::new(
        verifying_shares,
        verifying_key,
        None,
    ))
}

pub(crate) fn decode_key_package(
    operation: &str,
    key_package_identifier: &str,
    key_package_hex: &str,
) -> Result<frost::keys::KeyPackage, EngineError> {
    let expected_identifier =
        parse_frost_identifier(operation, "key_package_identifier", key_package_identifier)?;
    let mut key_package_bytes = decode_hex_field(operation, "key_package_hex", key_package_hex)?;
    let key_package_result = frost::keys::KeyPackage::deserialize(&key_package_bytes);
    key_package_bytes.zeroize();
    let key_package = key_package_result
        .map_err(|e| EngineError::Validation(format!("{operation}: invalid key package: {e}")))?;

    if *key_package.identifier() != expected_identifier {
        return Err(EngineError::Validation(format!(
            "{operation}: key_package_identifier does not match serialized key package"
        )));
    }

    Ok(key_package)
}

pub(crate) fn decode_signing_commitment_map(
    operation: &str,
    commitments: &[NativeFrostCommitment],
) -> Result<BTreeMap<frost::Identifier, frost::round1::SigningCommitments>, EngineError> {
    if commitments.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: commitments must not be empty"
        )));
    }

    let mut commitment_map = BTreeMap::new();
    for (index, commitment) in commitments.iter().enumerate() {
        let identifier = parse_frost_identifier(
            operation,
            &format!("commitments[{index}].identifier"),
            &commitment.identifier,
        )?;
        let commitment_bytes = decode_hex_field(
            operation,
            &format!("commitments[{index}].data_hex"),
            &commitment.data_hex,
        )?;
        let signing_commitment = frost::round1::SigningCommitments::deserialize(&commitment_bytes)
            .map_err(|e| {
                EngineError::Validation(format!(
                    "{operation}: invalid signing commitment [{index}]: {e}"
                ))
            })?;
        if commitment_map
            .insert(identifier, signing_commitment)
            .is_some()
        {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate commitment identifier [{}]",
                commitment.identifier
            )));
        }
    }

    Ok(commitment_map)
}

pub(crate) fn decode_signature_share_map(
    operation: &str,
    signature_shares: &[NativeFrostSignatureShare],
) -> Result<BTreeMap<frost::Identifier, frost::round2::SignatureShare>, EngineError> {
    if signature_shares.is_empty() {
        return Err(EngineError::Validation(format!(
            "{operation}: signature_shares must not be empty"
        )));
    }

    let mut signature_share_map = BTreeMap::new();
    for (index, signature_share) in signature_shares.iter().enumerate() {
        let identifier = parse_frost_identifier(
            operation,
            &format!("signature_shares[{index}].identifier"),
            &signature_share.identifier,
        )?;
        let mut signature_share_bytes = decode_hex_field(
            operation,
            &format!("signature_shares[{index}].data_hex"),
            &signature_share.data_hex,
        )?;
        let signature_share = frost::round2::SignatureShare::deserialize(&signature_share_bytes)
            .map_err(|e| {
                EngineError::Validation(format!(
                    "{operation}: invalid signature share [{index}]: {e}"
                ))
            })?;
        signature_share_bytes.zeroize();
        if signature_share_map
            .insert(identifier, signature_share)
            .is_some()
        {
            return Err(EngineError::Validation(format!(
                "{operation}: duplicate signature share identifier"
            )));
        }
    }

    Ok(signature_share_map)
}
