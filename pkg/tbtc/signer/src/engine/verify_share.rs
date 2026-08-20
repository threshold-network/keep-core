// Phase 7.2b-4: single round-2 signature-share verification.
//
// Backs the Go host's Round2ShareVerifier (member-blame classifier, 4a). Given
// one retained share + the attempt's signing package, it answers whether the
// share verifies against the group's own (taproot-tweaked) verifying material -
// pure FROST share verification, no envelope/operator-signature inspection
// (that is the Go layer's job; frozen Q1 boundary).
//
// As with InteractiveAggregate's candidate culprits, an `Invalid` verdict is
// framable: this endpoint verifies the share against WHATEVER package/root the
// caller supplies, so a coordinator that verifies an honest share against a
// mismatched package/root would get `Invalid`. The engine cannot bind these
// public inputs to what the member signed at Round2; authoritative,
// envelope-bound blame is the Go host's job at an f+1 accuser quorum (frozen
// Phase 7.2b spec, section 6). This verdict is that adjudication's INPUT.
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
    // tweak path must not drift; None vs Some([]) must resolve identically). A
    // malformed root (non-hex / not 32 bytes) is COORDINATOR/wallet-context
    // input, never the member's signed-share fault, so it returns an in-band
    // Indeterminate verdict rather than escaping to the FFI error channel: the
    // contract is "a tri-state verdict for every input", so the Go host never
    // has to infer "don't blame" from an error code.
    let mut taproot_merkle_root_hex = request.taproot_merkle_root_hex.clone();
    let taproot_merkle_root =
        match canonicalize_taproot_merkle_root_hex(&mut taproot_merkle_root_hex) {
            Ok(root) => root,
            Err(_) => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
        };

    let member_identifier =
        match participant_identifier_to_frost_identifier(request.member_identifier) {
            Ok(identifier) => identifier,
            // An out-of-range member id is a caller/package issue, not the
            // member's signed-share fault.
            Err(_) => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
        };

    // Resolve the group's public key package from the session's own DKG state -
    // never the request - mirroring InteractiveAggregate. A missing session or
    // incomplete DKG is not the member's fault -> indeterminate.
    let public_key_package = {
        let mut guard = state()?
            .lock()
            .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
        // Verify-share takes the engine lock like every other interactive entry
        // point, so it sweeps expired interactive state too: the nonce-TTL
        // guarantee (an abandoned interactive nonce handle gone within the TTL of
        // inactivity) must hold even when the only post-expiry traffic is
        // verify-share blame rechecks. Mirrors InteractiveAggregate.
        sweep_expired_interactive_state_durably(&mut guard)?;
        // The public key package is a WALLET-level asset resolved by key_group, so a
        // per-signing session (a distinct RoastSessionID) can be blame-checked. The
        // key_group is this signing session's own DKG (co-located) or the one bound at
        // Open; a missing session/binding/DKG is not the member's fault -> indeterminate.
        let key_group = match guard.sessions.get(&request.session_id) {
            Some(session) => session
                .dkg
                .result
                .as_ref()
                .map(|dkg| dkg.key_group.clone())
                .or_else(|| session.interactive.bound_key_group.clone()),
            None => None,
        };
        let key_group = match key_group {
            Some(key_group) => key_group,
            None => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
        };
        let wallet_session_id =
            match resolve_wallet_session_id(&guard, &request.session_id, &key_group) {
                Some(id) => id,
                None => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
            };
        match guard
            .sessions
            .get(&wallet_session_id)
            .and_then(|session| session.dkg.public_key_package.as_ref())
        {
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
        // No verifying share in this GROUP (the member never received a DKG
        // share) - a coordinator/caller matter, not member-share fault.
        None => return Ok(verdict(ShareVerificationVerdict::Indeterminate)),
    };

    // The member must ALSO be a participant in THIS attempt's signing package
    // (its commitment set), not merely a group member. A package that omits the
    // member is coordinator/context input - the member never signed a share for
    // a package they are not in - so undecodable share bytes paired with such a
    // package must NOT read as self-incriminating. Without this guard the
    // omitted-member + undecodable-share case would hit the Invalid decode-
    // failure branch below before frost_core's own UnknownIdentifier check (which
    // already maps a DECODABLE omitted-member share to Indeterminate) is reached.
    // Fail closed against blame: Indeterminate.
    if !signing_package
        .signing_commitments()
        .contains_key(&member_identifier)
    {
        return Ok(verdict(ShareVerificationVerdict::Indeterminate));
    }

    // Only now that the session, completed DKG, the member's group membership,
    // AND the member's inclusion in this attempt's package are all established do
    // we judge the member's signed share bytes: if they are undecodable here that
    // is self-incriminating member fault -> invalid (the Go layer already
    // authenticated the envelope; the inner FROST scalar is the member's).
    // Decoding any earlier would let a malformed share for an unknown / not-ready
    // session, a non-member id, or a package that omits the member return Invalid
    // (blame) before the member context exists - it must be Indeterminate then.
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
