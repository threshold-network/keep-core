// Phase 7.2b-4: single round-2 signature-share verification.
//
// Backs the Go host's Round2ShareVerifier (member-blame classifier, 4a). Given
// one retained share + the attempt's signing package, it answers whether the
// share verifies against the group's own (taproot-tweaked) verifying material -
// pure FROST share verification, no envelope/operator-signature inspection
// (that is the Go layer's job; frozen Q1 boundary).
//
// It returns an explicit tri-state verdict (Valid / Invalid / Indeterminate),
// not a pass/fail + error: only the engine can distinguish a member's malformed
// signed scalar (blame) from a malformed package/context (don't blame), so it
// makes that call here rather than forcing the Go host to infer it from an error
// string. The verifying material + taproot tweak are resolved EXACTLY as
// InteractiveAggregate does (same session DKG package, same
// canonicalize_taproot_merkle_root_hex, same `.tweak()`), so the verdict matches
// what aggregation would conclude for the same share - pinned by the
// standalone-vs-aggregate equivalence tests.

use super::*;

use crate::api::{
    ShareVerificationVerdict, VerifySignatureShareRequest, VerifySignatureShareResult,
};

fn verdict(v: ShareVerificationVerdict) -> VerifySignatureShareResult {
    VerifySignatureShareResult { verdict: v }
}

pub fn verify_signature_share(
    request: VerifySignatureShareRequest,
) -> Result<VerifySignatureShareResult, EngineError> {
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    // An undecodable signing package is COORDINATOR-authored input, not the
    // member's fault -> indeterminate (never blame the member for it).
    let signing_package_bytes = match decode_hex_field(
        "VerifySignatureShare",
        "signing_package_hex",
        &request.signing_package_hex,
    ) {
        Ok(bytes) => bytes,
        Err(_) => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
    };
    let signing_package = match frost::SigningPackage::deserialize(&signing_package_bytes) {
        Ok(package) => package,
        Err(_) => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
    };

    // Canonicalize + apply the taproot root EXACTLY as InteractiveAggregate (the
    // tweak path must not drift; None vs Some([]) must resolve identically).
    let mut taproot_merkle_root_hex = request.taproot_merkle_root_hex.clone();
    let taproot_merkle_root = canonicalize_taproot_merkle_root_hex(&mut taproot_merkle_root_hex)?;

    let member_identifier =
        match participant_identifier_to_frost_identifier(request.member_identifier) {
            Ok(identifier) => identifier,
            // An out-of-range member id is a caller/package issue, not the
            // member's signed-share fault.
            Err(_) => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
        };

    // The member operator-signed these share bytes: if they are undecodable, that
    // is self-incriminating member fault -> invalid (the Go layer already
    // authenticated the envelope; the inner FROST scalar is the member's).
    let signature_share_bytes = match decode_hex_field(
        "VerifySignatureShare",
        "signature_share_hex",
        &request.signature_share_hex,
    ) {
        Ok(bytes) => bytes,
        Err(_) => return Ok(verdict(ShareVerificationVerdict::Invalid)),
    };
    let signature_share = match frost::round2::SignatureShare::deserialize(&signature_share_bytes) {
        Ok(share) => share,
        Err(_) => return Ok(verdict(ShareVerificationVerdict::Invalid)),
    };

    // Resolve the group's public key package from the session's own DKG state -
    // never the request - mirroring InteractiveAggregate. A missing session or
    // incomplete DKG is not the member's fault -> indeterminate.
    let public_key_package = {
        let guard = state()?
            .lock()
            .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
        let session = match guard.sessions.get(&request.session_id) {
            Some(session) => session,
            None => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
        };
        if session.dkg_result.is_none() {
            return Ok(verdict(ShareVerificationVerdict::Indeterminate));
        }
        match session.dkg_public_key_package.as_ref() {
            Some(package) => package.clone(),
            None => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
        }
    };

    // Same tweak expression as InteractiveAggregate (strict input parity).
    let verification_key_package = match taproot_merkle_root.as_ref() {
        Some(root) => public_key_package.clone().tweak(Some(root.as_slice())),
        None => public_key_package.clone(),
    };

    let verifying_share = match verification_key_package
        .verifying_shares()
        .get(&member_identifier)
    {
        Some(verifying_share) => verifying_share,
        // The member has no verifying share for this group / is not in the
        // package's set - a coordinator/package matter, not member-share fault.
        None => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
    };

    match frost_core::verify_signature_share(
        member_identifier,
        verifying_share,
        &signature_share,
        &signing_package,
        verification_key_package.verifying_key(),
    ) {
        Ok(()) => Ok(verdict(ShareVerificationVerdict::Valid)),
        // The member's signed share fails the FROST verification equation.
        Err(frost_core::Error::InvalidSignatureShare { .. }) => {
            Ok(verdict(ShareVerificationVerdict::Invalid))
        }
        // UnknownIdentifier / commitment / other: a package or context issue, not
        // attributable member-share fault.
        Err(_) => Ok(verdict(ShareVerificationVerdict::Indeterminate)),
    }
}
