// Stateless FROST primitives: dkg_part1..3, nonces, signing package, share, aggregate.

use super::*;

pub fn dkg_part1(request: DkgPart1Request) -> Result<DkgPart1Result, EngineError> {
    enforce_provenance_gate()?;

    if request.max_signers == 0 {
        return Err(EngineError::Validation(
            "DKGPart1: max_signers is zero".to_string(),
        ));
    }
    if request.min_signers == 0 {
        return Err(EngineError::Validation(
            "DKGPart1: min_signers is zero".to_string(),
        ));
    }
    if request.min_signers > request.max_signers {
        return Err(EngineError::Validation(
            "DKGPart1: min_signers exceeds max_signers".to_string(),
        ));
    }

    let identifier = parse_frost_identifier(
        "DKGPart1",
        "participant_identifier",
        &request.participant_identifier,
    )?;
    let rng = zeroizing_rng_from_os();
    let (mut secret_package, package) =
        frost::keys::dkg::part1(identifier, request.max_signers, request.min_signers, rng)
            .map_err(|e| EngineError::Validation(format!("DKGPart1 failed: {e}")))?;

    let package_bytes = match package.serialize() {
        Ok(package_bytes) => package_bytes,
        Err(err) => {
            secret_package.zeroize();
            return Err(EngineError::Internal(format!(
                "failed to serialize DKG part1 package: {err}"
            )));
        }
    };
    let secret_package_bytes_result = secret_package.serialize();
    secret_package.zeroize();
    let mut secret_package_bytes = secret_package_bytes_result
        .map_err(|e| EngineError::Internal(format!("failed to serialize DKG part1 secret: {e}")))?;

    let result = DkgPart1Result {
        secret_package_hex: hex::encode(&secret_package_bytes),
        package: DkgRound1Package {
            identifier: frost_identifier_to_go_string(identifier),
            package_hex: hex::encode(package_bytes),
        },
    };
    secret_package_bytes.zeroize();

    Ok(result)
}

pub fn dkg_part2(request: DkgPart2Request) -> Result<DkgPart2Result, EngineError> {
    enforce_provenance_gate()?;

    let mut secret_package_bytes = decode_hex_field(
        "DKGPart2",
        "secret_package_hex",
        &request.secret_package_hex,
    )?;
    let secret_package_result =
        frost::keys::dkg::round1::SecretPackage::deserialize(&secret_package_bytes);
    secret_package_bytes.zeroize();
    let mut secret_package = secret_package_result
        .map_err(|e| EngineError::Validation(format!("DKGPart2: invalid secret package: {e}")))?;

    let round1_packages = match decode_round1_package_map("DKGPart2", &request.round1_packages) {
        Ok(round1_packages) => round1_packages,
        Err(err) => {
            secret_package.zeroize();
            return Err(err);
        }
    };
    let (mut round2_secret_package, round2_packages) =
        frost::keys::dkg::part2(secret_package, &round1_packages)
            .map_err(|e| EngineError::Validation(format!("DKGPart2 failed: {e}")))?;

    let mut packages = Vec::with_capacity(round2_packages.len());
    for (identifier, package) in round2_packages {
        let mut package_bytes = match package.serialize() {
            Ok(package_bytes) => package_bytes,
            Err(err) => {
                round2_secret_package.zeroize();
                return Err(EngineError::Internal(format!(
                    "failed to serialize DKG part2 package: {err}"
                )));
            }
        };
        packages.push(DkgRound2Package {
            identifier: frost_identifier_to_go_string(identifier),
            sender_identifier: None,
            package_hex: hex::encode(&package_bytes),
        });
        package_bytes.zeroize();
    }

    let round2_secret_package_bytes_result = round2_secret_package.serialize();
    round2_secret_package.zeroize();
    let mut round2_secret_package_bytes = round2_secret_package_bytes_result
        .map_err(|e| EngineError::Internal(format!("failed to serialize DKG part2 secret: {e}")))?;

    let result = DkgPart2Result {
        secret_package_hex: hex::encode(&round2_secret_package_bytes),
        packages,
    };
    round2_secret_package_bytes.zeroize();

    Ok(result)
}

pub fn dkg_part3(request: DkgPart3Request) -> Result<DkgPart3Result, EngineError> {
    enforce_provenance_gate()?;

    let mut secret_package_bytes = decode_hex_field(
        "DKGPart3",
        "secret_package_hex",
        &request.secret_package_hex,
    )?;
    let secret_package_result =
        frost::keys::dkg::round2::SecretPackage::deserialize(&secret_package_bytes);
    secret_package_bytes.zeroize();
    let mut secret_package = secret_package_result
        .map_err(|e| EngineError::Validation(format!("DKGPart3: invalid secret package: {e}")))?;

    let round1_packages = match decode_round1_package_map("DKGPart3", &request.round1_packages) {
        Ok(round1_packages) => round1_packages,
        Err(err) => {
            secret_package.zeroize();
            return Err(err);
        }
    };
    let round2_packages = match decode_round2_package_map(
        "DKGPart3",
        &request.round2_packages,
        Some(*secret_package.identifier()),
    ) {
        Ok(round2_packages) => round2_packages,
        Err(err) => {
            secret_package.zeroize();
            return Err(err);
        }
    };
    let dkg_result = frost::keys::dkg::part3(&secret_package, &round1_packages, &round2_packages);
    secret_package.zeroize();
    let (key_package, public_key_package) =
        dkg_result.map_err(|e| EngineError::Validation(format!("DKGPart3 failed: {e}")))?;

    let is_even_y = public_key_package.has_even_y();
    let key_package = key_package.into_even_y(Some(is_even_y));
    let public_key_package = public_key_package.into_even_y(Some(is_even_y));

    let native_public_key_package = native_public_key_package_from_frost(&public_key_package)?;
    let mut key_package_bytes = key_package
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize DKG key package: {e}")))?;
    let result = DkgPart3Result {
        key_package: NativeFrostKeyPackage {
            identifier: frost_identifier_to_go_string(*key_package.identifier()),
            data_hex: hex::encode(&key_package_bytes),
        },
        public_key_package: native_public_key_package,
    };
    key_package_bytes.zeroize();

    Ok(result)
}

pub fn generate_nonces_and_commitments(
    request: GenerateNoncesAndCommitmentsRequest,
) -> Result<GenerateNoncesAndCommitmentsResult, EngineError> {
    enforce_provenance_gate()?;

    let key_package = decode_key_package(
        "GenerateNoncesAndCommitments",
        &request.key_package_identifier,
        &request.key_package_hex,
    )?;
    let mut rng = zeroizing_rng_from_os();
    let (mut nonces, commitments) = frost::round1::commit(key_package.signing_share(), &mut rng);
    let commitment_bytes = match commitments.serialize() {
        Ok(commitment_bytes) => commitment_bytes,
        Err(err) => {
            nonces.zeroize();
            return Err(EngineError::Internal(format!(
                "failed to serialize signing commitments: {err}"
            )));
        }
    };
    let nonces_bytes_result = nonces.serialize();
    nonces.zeroize();
    let mut nonces_bytes = nonces_bytes_result
        .map_err(|e| EngineError::Internal(format!("failed to serialize signing nonces: {e}")))?;

    let result = GenerateNoncesAndCommitmentsResult {
        nonces_hex: hex::encode(&nonces_bytes),
        commitment: NativeFrostCommitment {
            identifier: frost_identifier_to_go_string(*key_package.identifier()),
            data_hex: hex::encode(commitment_bytes),
        },
    };
    nonces_bytes.zeroize();

    Ok(result)
}

pub fn new_signing_package(
    request: NewSigningPackageRequest,
) -> Result<NewSigningPackageResult, EngineError> {
    enforce_provenance_gate()?;

    let message = if request.message_hex.is_empty() {
        Vec::new()
    } else {
        hex::decode(&request.message_hex).map_err(|_| {
            EngineError::Validation("NewSigningPackage: message_hex must be valid hex".to_string())
        })?
    };
    let commitments = decode_signing_commitment_map("NewSigningPackage", &request.commitments)?;
    let signing_package = frost::SigningPackage::new(commitments, &message);
    let signing_package_bytes = signing_package
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize signing package: {e}")))?;

    Ok(NewSigningPackageResult {
        signing_package_hex: hex::encode(signing_package_bytes),
    })
}

pub fn sign_share(request: SignShareRequest) -> Result<SignShareResult, EngineError> {
    enforce_provenance_gate()?;

    let signing_package_bytes = decode_hex_field(
        "SignShare",
        "signing_package_hex",
        &request.signing_package_hex,
    )?;
    let signing_package = frost::SigningPackage::deserialize(&signing_package_bytes)
        .map_err(|e| EngineError::Validation(format!("SignShare: invalid signing package: {e}")))?;

    let mut nonces_bytes = decode_hex_field("SignShare", "nonces_hex", &request.nonces_hex)?;
    let nonces_result = frost::round1::SigningNonces::deserialize(&nonces_bytes);
    nonces_bytes.zeroize();
    let mut nonces = nonces_result
        .map_err(|e| EngineError::Validation(format!("SignShare: invalid nonces: {e}")))?;

    let key_package = match decode_key_package(
        "SignShare",
        &request.key_package_identifier,
        &request.key_package_hex,
    ) {
        Ok(key_package) => key_package,
        Err(err) => {
            nonces.zeroize();
            return Err(err);
        }
    };
    let signature_share_result = frost::round2::sign(&signing_package, &nonces, &key_package);
    nonces.zeroize();
    let signature_share = signature_share_result
        .map_err(|e| EngineError::Validation(format!("SignShare failed: {e}")))?;
    let mut signature_share_bytes = signature_share.serialize();
    let result = SignShareResult {
        signature_share: NativeFrostSignatureShare {
            identifier: frost_identifier_to_go_string(*key_package.identifier()),
            data_hex: hex::encode(&signature_share_bytes),
        },
    };
    signature_share_bytes.zeroize();

    Ok(result)
}

pub fn aggregate(request: AggregateRequest) -> Result<AggregateResult, EngineError> {
    enforce_provenance_gate()?;

    let signing_package_bytes = decode_hex_field(
        "Aggregate",
        "signing_package_hex",
        &request.signing_package_hex,
    )?;
    let signing_package = frost::SigningPackage::deserialize(&signing_package_bytes)
        .map_err(|e| EngineError::Validation(format!("Aggregate: invalid signing package: {e}")))?;
    let signature_shares = decode_signature_share_map("Aggregate", &request.signature_shares)?;
    let public_key_package =
        native_public_key_package_to_frost("Aggregate", &request.public_key_package)?;
    let signature = frost::aggregate(&signing_package, &signature_shares, &public_key_package)
        .map_err(|e| EngineError::Validation(format!("Aggregate failed: {e}")))?;
    let signature_bytes = signature
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize aggregate: {e}")))?;

    Ok(AggregateResult {
        signature_hex: hex::encode(signature_bytes),
    })
}
