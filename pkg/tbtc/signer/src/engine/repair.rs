//! Offline-authorized repair of one lost FROST signing share.
//!
//! The upstream repairable-threshold primitive intentionally implements only
//! the scalar arithmetic. This module supplies the protocol boundary it does
//! not have: a signed, expiring context; exact helper-set validation; endpoint
//! binding for every delta and sigma; public-package commitment checks; and an
//! atomic install into the descriptor-bound signer store.

use super::*;

use ed25519_dalek::{Signature, VerifyingKey};
use frost::keys::repairable::{
    repair_share_part1 as frost_repair_share_part1, repair_share_part2 as frost_repair_share_part2,
    repair_share_part3 as frost_repair_share_part3, Delta, Sigma,
};

pub(crate) const TBTC_SIGNER_SHARE_REPAIR_AUTHORIZATION_SCHEMA: &str =
    "tbtc-frost-share-repair-authorization/v1";
pub(crate) const TBTC_SIGNER_SHARE_REPAIR_INSTALL_RESULT_SCHEMA: &str =
    "tbtc-frost-share-repair-install-result/v1";

const SHARE_REPAIR_AUTHORIZATION_DOMAIN: &[u8] = b"tbtc-frost-share-repair-authorization/v1\0";
const SHARE_REPAIR_MAX_AUTHORIZATION_LIFETIME_SECONDS: u64 = 24 * 60 * 60;

#[cfg(test)]
static TEST_SHARE_REPAIR_AUTHORITY: OnceLock<Mutex<Option<[u8; 32]>>> = OnceLock::new();

#[cfg(test)]
pub(crate) fn set_share_repair_authority_for_tests(public_key: Option<[u8; 32]>) {
    *TEST_SHARE_REPAIR_AUTHORITY
        .get_or_init(|| Mutex::new(None))
        .lock()
        .expect("share-repair test authority lock") = public_key;
}

#[derive(Clone)]
struct ValidatedShareRepairAuthorization {
    digest: [u8; 32],
    wallet_id: [u8; 32],
    compressed_key_group: [u8; 33],
    public_key_package_commitment: [u8; 32],
    target_identifier: frost::Identifier,
    helper_identifiers: Vec<frost::Identifier>,
    new_store_fingerprint: [u8; 32],
}

fn validation_error(operation: &str, detail: impl std::fmt::Display) -> EngineError {
    EngineError::Validation(format!("{operation}: {detail}"))
}

fn write_length_prefixed(digest: &mut Sha256, value: &[u8]) -> Result<(), EngineError> {
    let length = u32::try_from(value.len()).map_err(|_| {
        EngineError::Validation(
            "share-repair authorization contains a field longer than u32::MAX".to_string(),
        )
    })?;
    digest.update(length.to_be_bytes());
    digest.update(value);
    Ok(())
}

fn require_nonzero_bytes32(value: [u8; 32], label: &str) -> Result<[u8; 32], EngineError> {
    if value == [0u8; 32] {
        return Err(EngineError::Validation(format!("{label} must not be zero")));
    }
    Ok(value)
}

#[allow(clippy::too_many_arguments)]
fn share_repair_authorization_signing_digest(
    authorization: &ShareRepairAuthorization,
    wallet_id: [u8; 32],
    compressed_key_group: [u8; 33],
    public_key_package_commitment: [u8; 32],
    old_store_fingerprint: [u8; 32],
    new_store_fingerprint: [u8; 32],
    nonce: [u8; 32],
) -> Result<[u8; 32], EngineError> {
    let mut transcript = Sha256::new();
    transcript.update(SHARE_REPAIR_AUTHORIZATION_DOMAIN);
    write_length_prefixed(&mut transcript, authorization.session_id.as_bytes())?;
    transcript.update(wallet_id);
    transcript.update(compressed_key_group);
    transcript.update(public_key_package_commitment);
    transcript.update(authorization.target_identifier.to_be_bytes());
    transcript.update(
        u16::try_from(authorization.helper_identifiers.len())
            .map_err(|_| {
                EngineError::Validation(
                    "share-repair helper count does not fit the signing transcript".to_string(),
                )
            })?
            .to_be_bytes(),
    );
    for helper in &authorization.helper_identifiers {
        transcript.update(helper.to_be_bytes());
    }
    transcript.update(authorization.threshold.to_be_bytes());
    transcript.update(authorization.participant_count.to_be_bytes());
    transcript.update(old_store_fingerprint);
    transcript.update(new_store_fingerprint);
    transcript.update(authorization.recovery_epoch.to_be_bytes());
    transcript.update(authorization.issued_at_unix.to_be_bytes());
    transcript.update(authorization.not_before_unix.to_be_bytes());
    transcript.update(authorization.expires_at_unix.to_be_bytes());
    transcript.update(nonce);
    Ok(transcript.finalize().into())
}

fn enforce_share_repair_authorization_time(
    authorization: &ShareRepairAuthorization,
) -> Result<(), EngineError> {
    let now = now_unix();
    if now == 0 {
        return Err(EngineError::Internal(
            "share-repair authorization: system clock is before UNIX epoch".to_string(),
        ));
    }
    if now < authorization.not_before_unix {
        return Err(EngineError::Validation(format!(
            "share-repair authorization is not valid before [{}]",
            authorization.not_before_unix
        )));
    }
    if now >= authorization.expires_at_unix {
        return Err(EngineError::Validation(format!(
            "share-repair authorization expired at [{}]",
            authorization.expires_at_unix
        )));
    }
    Ok(())
}

fn validate_share_repair_authorization(
    operation: &str,
    authorization: &ShareRepairAuthorization,
    enforce_time: bool,
) -> Result<ValidatedShareRepairAuthorization, EngineError> {
    if authorization.schema != TBTC_SIGNER_SHARE_REPAIR_AUTHORIZATION_SCHEMA {
        return Err(validation_error(
            operation,
            "unsupported share-repair authorization schema",
        ));
    }
    validate_session_id(&authorization.session_id)?;
    if authorization.threshold < 2
        || authorization.participant_count < authorization.threshold
        || authorization.participant_count > 100
    {
        return Err(validation_error(
            operation,
            format!(
                "threshold [{}] must be between 2 and participant_count [{}], with at most 100 participants",
                authorization.threshold, authorization.participant_count
            ),
        ));
    }
    if authorization.helper_identifiers.len() != authorization.threshold as usize {
        return Err(validation_error(
            operation,
            format!(
                "helper_identifiers must contain exactly threshold [{}] members",
                authorization.threshold
            ),
        ));
    }
    if authorization.target_identifier == 0
        || authorization.target_identifier > authorization.participant_count
    {
        return Err(validation_error(
            operation,
            "target_identifier is outside the participant set",
        ));
    }
    let mut previous_helper = 0u16;
    for helper in &authorization.helper_identifiers {
        if *helper == 0 || *helper > authorization.participant_count {
            return Err(validation_error(
                operation,
                "helper identifier is outside the participant set",
            ));
        }
        if *helper <= previous_helper {
            return Err(validation_error(
                operation,
                "helper_identifiers must be distinct and strictly ascending",
            ));
        }
        if *helper == authorization.target_identifier {
            return Err(validation_error(
                operation,
                "target_identifier must not be a helper",
            ));
        }
        previous_helper = *helper;
    }
    if authorization.recovery_epoch == 0 {
        return Err(validation_error(
            operation,
            "recovery_epoch must be non-zero",
        ));
    }
    if authorization.issued_at_unix > authorization.not_before_unix
        || authorization.not_before_unix >= authorization.expires_at_unix
    {
        return Err(validation_error(
            operation,
            "authorization timestamps are not ordered",
        ));
    }
    let lifetime = authorization
        .expires_at_unix
        .checked_sub(authorization.issued_at_unix)
        .ok_or_else(|| validation_error(operation, "authorization lifetime underflow"))?;
    if lifetime > SHARE_REPAIR_MAX_AUTHORIZATION_LIFETIME_SECONDS {
        return Err(validation_error(
            operation,
            format!(
                "authorization lifetime exceeds [{}] seconds",
                SHARE_REPAIR_MAX_AUTHORIZATION_LIFETIME_SECONDS
            ),
        ));
    }

    let wallet_id = require_nonzero_bytes32(
        parse_canonical_bytes32(&authorization.wallet_id, "wallet_id")?,
        "wallet_id",
    )?;
    let public_key_package_commitment = require_nonzero_bytes32(
        parse_canonical_bytes32(
            &authorization.public_key_package_commitment,
            "public_key_package_commitment",
        )?,
        "public_key_package_commitment",
    )?;
    let old_store_fingerprint = require_nonzero_bytes32(
        parse_canonical_bytes32(
            &authorization.old_store_fingerprint,
            "old_store_fingerprint",
        )?,
        "old_store_fingerprint",
    )?;
    let new_store_fingerprint = require_nonzero_bytes32(
        parse_canonical_bytes32(
            &authorization.new_store_fingerprint,
            "new_store_fingerprint",
        )?,
        "new_store_fingerprint",
    )?;
    if old_store_fingerprint == new_store_fingerprint {
        return Err(validation_error(
            operation,
            "old_store_fingerprint and new_store_fingerprint must differ",
        ));
    }
    let nonce = require_nonzero_bytes32(
        parse_canonical_bytes32(&authorization.nonce, "nonce")?,
        "nonce",
    )?;

    let (derived_wallet_id, compressed_key_group) =
        super::inventory::parse_key_group(&authorization.key_group).map_err(|error| {
            validation_error(
                operation,
                format!("key_group is not canonical compressed SEC1: {error}"),
            )
        })?;
    if derived_wallet_id != wallet_id {
        return Err(validation_error(
            operation,
            "wallet_id does not match key_group",
        ));
    }

    let target_identifier =
        participant_identifier_to_frost_identifier(authorization.target_identifier)?;
    let helper_identifiers = authorization
        .helper_identifiers
        .iter()
        .copied()
        .map(participant_identifier_to_frost_identifier)
        .collect::<Result<Vec<_>, _>>()?;

    let digest = share_repair_authorization_signing_digest(
        authorization,
        wallet_id,
        compressed_key_group,
        public_key_package_commitment,
        old_store_fingerprint,
        new_store_fingerprint,
        nonce,
    )?;

    #[cfg(test)]
    let test_authority = TEST_SHARE_REPAIR_AUTHORITY
        .get_or_init(|| Mutex::new(None))
        .lock()
        .expect("share-repair test authority lock")
        .as_ref()
        .copied();
    #[cfg(not(test))]
    let test_authority: Option<[u8; 32]> = None;
    let authority_public_key = if let Some(test_authority) = test_authority {
        test_authority
    } else {
        let configuration = configured_state_anchor()?.ok_or_else(|| {
            validation_error(
                operation,
                "state-anchor trust configuration is required for share repair",
            )
        })?;
        configuration
            .trust
            .ok_or_else(|| {
                validation_error(
                    operation,
                    "offline-authority trust configuration is required for share repair",
                )
            })?
            .offline_authority_public_key
    };
    let signature = parse_canonical_signature(&authorization.signature_hex)?;
    let verifying_key = VerifyingKey::from_bytes(&authority_public_key).map_err(|error| {
        EngineError::Internal(format!(
            "configured share-repair authority key is invalid: {error}"
        ))
    })?;
    verifying_key
        .verify_strict(&digest, &Signature::from_bytes(&signature))
        .map_err(|_| validation_error(operation, "authorization signature is invalid"))?;

    if enforce_time {
        enforce_share_repair_authorization_time(authorization)?;
    }

    Ok(ValidatedShareRepairAuthorization {
        digest,
        wallet_id,
        compressed_key_group,
        public_key_package_commitment,
        target_identifier,
        helper_identifiers,
        new_store_fingerprint,
    })
}

#[cfg(test)]
pub(crate) fn share_repair_authorization_digest_for_tests(
    authorization: &ShareRepairAuthorization,
) -> Result<[u8; 32], EngineError> {
    let wallet_id = parse_canonical_bytes32(&authorization.wallet_id, "wallet_id")?;
    let (_, compressed_key_group) = super::inventory::parse_key_group(&authorization.key_group)?;
    let public_key_package_commitment = parse_canonical_bytes32(
        &authorization.public_key_package_commitment,
        "public_key_package_commitment",
    )?;
    let old_store_fingerprint = parse_canonical_bytes32(
        &authorization.old_store_fingerprint,
        "old_store_fingerprint",
    )?;
    let new_store_fingerprint = parse_canonical_bytes32(
        &authorization.new_store_fingerprint,
        "new_store_fingerprint",
    )?;
    let nonce = parse_canonical_bytes32(&authorization.nonce, "nonce")?;
    share_repair_authorization_signing_digest(
        authorization,
        wallet_id,
        compressed_key_group,
        public_key_package_commitment,
        old_store_fingerprint,
        new_store_fingerprint,
        nonce,
    )
}

fn validate_public_key_package(
    operation: &str,
    authorization: &ShareRepairAuthorization,
    validated: &ValidatedShareRepairAuthorization,
    public_key_package: &NativeFrostPublicKeyPackage,
) -> Result<(frost::keys::PublicKeyPackage, frost::keys::PublicKeyPackage), EngineError> {
    let stored_shape = native_public_key_package_to_frost(operation, public_key_package)?;
    if stored_shape.max_signers() != authorization.participant_count {
        return Err(validation_error(
            operation,
            format!(
                "public key package has [{}] participants; expected [{}]",
                stored_shape.max_signers(),
                authorization.participant_count
            ),
        ));
    }
    for identifier in stored_shape.verifying_shares().keys() {
        let participant = frost_identifier_to_u16(*identifier).ok_or_else(|| {
            validation_error(
                operation,
                "public key package contains a non-canonical participant identifier",
            )
        })?;
        if participant == 0 || participant > authorization.participant_count {
            return Err(validation_error(
                operation,
                "public key package identifier is outside the participant set",
            ));
        }
    }
    if !stored_shape
        .verifying_shares()
        .contains_key(&validated.target_identifier)
        || validated
            .helper_identifiers
            .iter()
            .any(|helper| !stored_shape.verifying_shares().contains_key(helper))
    {
        return Err(validation_error(
            operation,
            "public key package does not contain the authorized target and helper set",
        ));
    }

    let serialized_group_key = stored_shape.verifying_key().serialize().map_err(|error| {
        EngineError::Internal(format!(
            "{operation}: failed to serialize public verifying key: {error}"
        ))
    })?;
    if serialized_group_key.as_slice() != validated.compressed_key_group {
        return Err(validation_error(
            operation,
            "public key package verifying key does not match key_group",
        ));
    }
    let serialized_public_package = stored_shape.serialize().map_err(|error| {
        EngineError::Internal(format!(
            "{operation}: failed to serialize public key package: {error}"
        ))
    })?;
    let commitment = public_key_package_commitment(
        &validated.wallet_id,
        &authorization.key_group,
        authorization.threshold,
        authorization.participant_count,
        0,
        &serialized_public_package,
    );
    if commitment != validated.public_key_package_commitment {
        return Err(validation_error(
            operation,
            "public key package does not match its authorized commitment",
        ));
    }

    let repair_shape = frost::keys::PublicKeyPackage::new(
        stored_shape.verifying_shares().clone(),
        *stored_shape.verifying_key(),
        Some(authorization.threshold),
    );
    Ok((stored_shape, repair_shape))
}

fn load_helper_material(
    operation: &str,
    authorization: &ShareRepairAuthorization,
    validated: &ValidatedShareRepairAuthorization,
    helper_identifier: u16,
) -> Result<(frost::keys::KeyPackage, frost::keys::PublicKeyPackage), EngineError> {
    if authorization
        .helper_identifiers
        .binary_search(&helper_identifier)
        .is_err()
    {
        return Err(validation_error(
            operation,
            "helper_identifier is not in the authorized helper set",
        ));
    }
    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let session = guard
        .sessions
        .get(&authorization.session_id)
        .ok_or_else(|| EngineError::SessionNotFound {
            session_id: authorization.session_id.clone(),
        })?;
    let dkg_result = session
        .dkg_result
        .as_ref()
        .ok_or_else(|| EngineError::DkgNotReady {
            session_id: authorization.session_id.clone(),
        })?;
    if dkg_result.key_group != authorization.key_group
        || dkg_result.threshold != authorization.threshold
        || dkg_result.participant_count != authorization.participant_count
        || session.dkg_share_epoch != 0
    {
        return Err(validation_error(
            operation,
            "authorization does not match the retained DKG session",
        ));
    }
    let stored_public = session.dkg_public_key_package.clone().ok_or_else(|| {
        EngineError::Internal(format!(
            "{operation}: retained DKG session has no public key package"
        ))
    })?;
    let native_public = native_public_key_package_from_frost(&stored_public)?;
    let (validated_stored_public, _) =
        validate_public_key_package(operation, authorization, validated, &native_public)?;
    if validated_stored_public != stored_public {
        return Err(EngineError::Internal(format!(
            "{operation}: retained public key package failed canonical round trip"
        )));
    }
    let key_package = session
        .dkg_key_packages
        .as_ref()
        .and_then(|packages| packages.get(&helper_identifier))
        .cloned()
        .ok_or_else(|| {
            validation_error(
                operation,
                format!("local store has no key package for helper [{helper_identifier}]"),
            )
        })?;
    let frost_helper = participant_identifier_to_frost_identifier(helper_identifier)?;
    if *key_package.identifier() != frost_helper
        || *key_package.min_signers() != authorization.threshold
        || key_package.verifying_key() != stored_public.verifying_key()
        || stored_public.verifying_shares().get(&frost_helper)
            != Some(key_package.verifying_share())
    {
        return Err(EngineError::Internal(format!(
            "{operation}: retained helper key package is inconsistent with its DKG session"
        )));
    }
    Ok((key_package, stored_public))
}

fn decode_repair_delta(
    operation: &str,
    index: usize,
    value: &SecretHex,
) -> Result<Delta, EngineError> {
    let wire = value.expose_secret();
    if wire.len() != 64 || wire.bytes().any(|byte| byte.is_ascii_uppercase()) {
        return Err(validation_error(
            operation,
            format!("deltas[{index}].data_hex must be canonical lowercase 32-byte hex"),
        ));
    }
    let mut bytes = decode_hex_field(operation, &format!("deltas[{index}].data_hex"), wire)?;
    let result = Delta::deserialize(&bytes).map_err(|error| {
        validation_error(
            operation,
            format!("invalid repair delta [{index}]: {error}"),
        )
    });
    bytes.zeroize();
    result
}

fn decode_repair_sigma(
    operation: &str,
    index: usize,
    value: &SecretHex,
) -> Result<Sigma, EngineError> {
    let wire = value.expose_secret();
    if wire.len() != 64 || wire.bytes().any(|byte| byte.is_ascii_uppercase()) {
        return Err(validation_error(
            operation,
            format!("sigmas[{index}].data_hex must be canonical lowercase 32-byte hex"),
        ));
    }
    let mut bytes = decode_hex_field(operation, &format!("sigmas[{index}].data_hex"), wire)?;
    let result = Sigma::deserialize(&bytes).map_err(|error| {
        validation_error(
            operation,
            format!("invalid repair sigma [{index}]: {error}"),
        )
    });
    bytes.zeroize();
    result
}

pub(crate) fn share_repair_part1(
    request: ShareRepairPart1Request,
) -> Result<ShareRepairPart1Result, EngineError> {
    const OP: &str = "share_repair_part1";
    enforce_provenance_gate()?;
    let validated = validate_share_repair_authorization(OP, &request.authorization, true)?;
    let (key_package, stored_public_key_package) = load_helper_material(
        OP,
        &request.authorization,
        &validated,
        request.helper_identifier,
    )?;
    let mut rng = zeroizing_rng_from_os();
    let deltas = frost_repair_share_part1::<frost::Secp256K1Sha256TR, _>(
        &validated.helper_identifiers,
        &key_package,
        &mut rng,
        validated.target_identifier,
    )
    .map_err(|error| validation_error(OP, format!("share repair part1 failed: {error}")))?;
    if deltas.len() != validated.helper_identifiers.len() {
        return Err(EngineError::Internal(format!(
            "{OP}: repair primitive returned an incomplete delta set"
        )));
    }

    let context_digest = bytes32_hex(validated.digest);
    let mut result_deltas = Vec::with_capacity(deltas.len());
    for (recipient, delta) in deltas {
        let recipient_identifier = frost_identifier_to_u16(recipient).ok_or_else(|| {
            EngineError::Internal(format!(
                "{OP}: repair primitive returned a foreign identifier"
            ))
        })?;
        let mut bytes = delta.serialize();
        let data_hex = SecretHex::new(hex::encode(&bytes));
        bytes.zeroize();
        result_deltas.push(ShareRepairDelta {
            context_digest: context_digest.clone(),
            sender_identifier: request.helper_identifier,
            recipient_identifier,
            data_hex,
        });
    }
    Ok(ShareRepairPart1Result {
        context_digest,
        helper_identifier: request.helper_identifier,
        public_key_package: native_public_key_package_from_frost(&stored_public_key_package)?,
        deltas: result_deltas,
    })
}

pub(crate) fn share_repair_part2(
    request: ShareRepairPart2Request,
) -> Result<ShareRepairPart2Result, EngineError> {
    const OP: &str = "share_repair_part2";
    enforce_provenance_gate()?;
    let validated = validate_share_repair_authorization(OP, &request.authorization, true)?;
    // Loading the selected helper's key is a possession/admission check even
    // though Part2 itself only sums incoming scalars.
    let _ = load_helper_material(
        OP,
        &request.authorization,
        &validated,
        request.helper_identifier,
    )?;
    if request.deltas.len() != request.authorization.helper_identifiers.len() {
        return Err(validation_error(
            OP,
            "deltas must contain exactly one value from every authorized helper",
        ));
    }
    let context_digest = bytes32_hex(validated.digest);
    let mut decoded = Vec::with_capacity(request.deltas.len());
    for (index, (delta, expected_sender)) in request
        .deltas
        .iter()
        .zip(request.authorization.helper_identifiers.iter())
        .enumerate()
    {
        if delta.context_digest != context_digest
            || delta.sender_identifier != *expected_sender
            || delta.recipient_identifier != request.helper_identifier
        {
            return Err(validation_error(
                OP,
                format!("delta [{index}] has the wrong context, sender, or recipient"),
            ));
        }
        decoded.push(decode_repair_delta(OP, index, &delta.data_hex)?);
    }
    let sigma = frost_repair_share_part2(&decoded);
    let mut bytes = sigma.serialize();
    let data_hex = SecretHex::new(hex::encode(&bytes));
    bytes.zeroize();
    Ok(ShareRepairPart2Result {
        context_digest: context_digest.clone(),
        sigma: ShareRepairSigma {
            context_digest,
            helper_identifier: request.helper_identifier,
            data_hex,
        },
    })
}

fn exact_installed_repair(
    authorization: &ShareRepairAuthorization,
    validated: &ValidatedShareRepairAuthorization,
    public_key_package: &frost::keys::PublicKeyPackage,
) -> Result<Option<DkgResult>, EngineError> {
    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let Some(session) = guard.sessions.get(&authorization.session_id) else {
        return Ok(None);
    };
    let Some(recovered) = session
        .recovered_seats
        .get(&authorization.target_identifier)
    else {
        return Ok(None);
    };
    if recovered.recovery_epoch != authorization.recovery_epoch
        || recovered.authorization_digest != validated.digest
        || recovered.active_store_fingerprint != validated.new_store_fingerprint
    {
        return Ok(None);
    }
    let result = session.dkg_result.as_ref().ok_or_else(|| {
        EngineError::Internal("recovered seat has no retained DKG result".to_string())
    })?;
    let key_package = session
        .dkg_key_packages
        .as_ref()
        .and_then(|packages| packages.get(&authorization.target_identifier))
        .ok_or_else(|| {
            EngineError::Internal("recovered seat has no retained key package".to_string())
        })?;
    if result.key_group != authorization.key_group
        || result.threshold != authorization.threshold
        || result.participant_count != authorization.participant_count
        || session.dkg_public_key_package.as_ref() != Some(public_key_package)
        || *key_package.identifier() != validated.target_identifier
        || public_key_package
            .verifying_shares()
            .get(&validated.target_identifier)
            != Some(key_package.verifying_share())
    {
        return Err(EngineError::Internal(
            "recovered-seat metadata is inconsistent with retained key material".to_string(),
        ));
    }
    Ok(Some(result.clone()))
}

fn install_result(
    authorization: &ShareRepairAuthorization,
    validated: &ValidatedShareRepairAuthorization,
    result: DkgResult,
    idempotent: bool,
) -> InstallRepairedShareResult {
    InstallRepairedShareResult {
        schema: TBTC_SIGNER_SHARE_REPAIR_INSTALL_RESULT_SCHEMA.to_string(),
        session_id: result.session_id,
        key_group: result.key_group,
        target_identifier: authorization.target_identifier,
        recovery_epoch: authorization.recovery_epoch,
        authorization_digest: bytes32_hex(validated.digest),
        active_store_fingerprint: bytes32_hex(validated.new_store_fingerprint),
        idempotent,
    }
}

pub(crate) fn install_repaired_share(
    request: InstallRepairedShareRequest,
) -> Result<InstallRepairedShareResult, EngineError> {
    const OP: &str = "install_repaired_share";
    enforce_provenance_gate()?;
    // Verify the static certificate first. The endpoint recognizes an exact
    // already-committed replay after expiry as defense in depth; normal
    // recovery from an uncertain external-anchor outcome is process restart
    // plus authenticated startup reconciliation. Expiry still gates every
    // initial installation and all helper-side secret generation.
    let validated = validate_share_repair_authorization(OP, &request.authorization, false)?;
    let (stored_public, repair_public) = validate_public_key_package(
        OP,
        &request.authorization,
        &validated,
        &request.public_key_package,
    )?;
    let current_store_fingerprint = durable_store_identity()?.fingerprint;
    if current_store_fingerprint != validated.new_store_fingerprint {
        return Err(validation_error(
            OP,
            "authorization does not name the active durable store",
        ));
    }
    if let Some(result) =
        exact_installed_repair(&request.authorization, &validated, &stored_public)?
    {
        return Ok(install_result(
            &request.authorization,
            &validated,
            result,
            true,
        ));
    }
    enforce_share_repair_authorization_time(&request.authorization)?;

    if request.sigmas.len() != request.authorization.helper_identifiers.len() {
        return Err(validation_error(
            OP,
            "sigmas must contain exactly one value from every authorized helper",
        ));
    }
    let context_digest = bytes32_hex(validated.digest);
    let mut decoded = Vec::with_capacity(request.sigmas.len());
    for (index, (sigma, expected_helper)) in request
        .sigmas
        .iter()
        .zip(request.authorization.helper_identifiers.iter())
        .enumerate()
    {
        if sigma.context_digest != context_digest || sigma.helper_identifier != *expected_helper {
            return Err(validation_error(
                OP,
                format!("sigma [{index}] has the wrong context or helper"),
            ));
        }
        decoded.push(decode_repair_sigma(OP, index, &sigma.data_hex)?);
    }
    let key_package =
        frost_repair_share_part3(&decoded, validated.target_identifier, &repair_public)
            .map_err(|error| validation_error(OP, format!("share repair part3 failed: {error}")))?;
    let expected_verifying_share = stored_public
        .verifying_shares()
        .get(&validated.target_identifier)
        .ok_or_else(|| {
            EngineError::Internal(format!("{OP}: target verifying share disappeared"))
        })?;
    if key_package.verifying_share() != expected_verifying_share
        || key_package.verifying_key() != stored_public.verifying_key()
        || *key_package.min_signers() != request.authorization.threshold
    {
        return Err(validation_error(
            OP,
            "reconstructed share does not match the authorized public key package",
        ));
    }
    let mut signing_share = *key_package.signing_share();
    let derives = frost::keys::VerifyingShare::from(signing_share) == *expected_verifying_share;
    signing_share.zeroize();
    if !derives {
        return Err(validation_error(
            OP,
            "reconstructed signing share does not derive to the target verifying share",
        ));
    }

    let mut key_package_bytes = key_package.serialize().map_err(|error| {
        EngineError::Internal(format!(
            "{OP}: failed to serialize repaired key package: {error}"
        ))
    })?;
    let key_package_hex = SecretHex::new(hex::encode(&key_package_bytes));
    key_package_bytes.zeroize();
    let persistence_request = PersistDistributedDkgKeyPackageRequest {
        session_id: request.authorization.session_id.clone(),
        participant_identifier: request.authorization.target_identifier,
        threshold: request.authorization.threshold,
        participant_count: request.authorization.participant_count,
        key_package: NativeFrostKeyPackage {
            identifier: frost_identifier_to_go_string(validated.target_identifier),
            data_hex: key_package_hex,
        },
        public_key_package: request.public_key_package,
    };
    let outcome = persist_repaired_dkg_key_package(
        persistence_request,
        RecoveredSeatState {
            participant_identifier: request.authorization.target_identifier,
            recovery_epoch: request.authorization.recovery_epoch,
            authorization_digest: validated.digest,
            active_store_fingerprint: validated.new_store_fingerprint,
        },
    )?;
    Ok(install_result(
        &request.authorization,
        &validated,
        outcome.result,
        outcome.idempotent,
    ))
}
