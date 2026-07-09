// ROAST/RFC-21 attempt machinery: request fingerprints, round/attempt ids, attempt-context and transition-evidence validation.

use super::*;

pub(crate) const ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN: &str =
    "FROST-ROAST-INCLUDED-FPR-v1";

pub(crate) const ROAST_ATTEMPT_ID_DOMAIN: &str = "FROST-ROAST-ATTEMPT-ID-v1";

pub(crate) const ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT: &str = "coordinator_timeout";

pub(crate) const ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF: &str = "invalid_share_proof";

pub fn roast_liveness_policy() -> RoastLivenessPolicyResult {
    RoastLivenessPolicyResult {
        coordinator_timeout_ms: roast_coordinator_timeout_ms(),
        timeout_source: "keep_core_wall_clock".to_string(),
        advance_trigger: "coordinator_timeout".to_string(),
        exclusion_evidence_policy: "timeout_or_invalid_share_proof".to_string(),
    }
}

pub(crate) fn fingerprint<T: serde::Serialize>(value: &T) -> Result<String, EngineError> {
    let mut bytes = serde_json::to_vec(value)
        .map_err(|e| EngineError::Internal(format!("failed to encode request: {e}")))?;
    let value_fingerprint = hash_hex(&bytes);
    bytes.zeroize();
    Ok(value_fingerprint)
}

pub(crate) fn canonicalize_refresh_shares_request_for_fingerprint(
    request: &RefreshSharesRequest,
) -> RefreshSharesRequest {
    let mut canonical_request = request.clone();
    canonical_request
        .current_shares
        .sort_unstable_by(|left, right| {
            left.identifier
                .cmp(&right.identifier)
                .then_with(|| left.encrypted_share_hex.cmp(&right.encrypted_share_hex))
        });
    canonical_request
}

pub(crate) fn canonicalize_taproot_merkle_root_hex(
    taproot_merkle_root_hex: &mut Option<String>,
) -> Result<Option<[u8; 32]>, EngineError> {
    let Some(raw_taproot_merkle_root_hex) = taproot_merkle_root_hex.as_mut() else {
        return Ok(None);
    };

    let normalized_taproot_merkle_root_hex =
        raw_taproot_merkle_root_hex.trim().to_ascii_lowercase();
    let taproot_merkle_root_bytes =
        hex::decode(&normalized_taproot_merkle_root_hex).map_err(|_| {
            EngineError::Validation("taproot_merkle_root_hex must be valid hex".to_string())
        })?;
    if taproot_merkle_root_bytes.len() != 32 {
        return Err(EngineError::Validation(
            "taproot_merkle_root_hex must decode to 32 bytes".to_string(),
        ));
    }

    let mut taproot_merkle_root = [0_u8; 32];
    taproot_merkle_root.copy_from_slice(&taproot_merkle_root_bytes);
    *raw_taproot_merkle_root_hex = normalized_taproot_merkle_root_hex;

    Ok(Some(taproot_merkle_root))
}

pub(crate) fn canonicalize_attempt_context_for_fingerprint(
    attempt_context: &mut Option<AttemptContext>,
) {
    if let Some(attempt_context) = attempt_context.as_mut() {
        attempt_context.included_participants.sort_unstable();
        attempt_context.included_participants_fingerprint = attempt_context
            .included_participants_fingerprint
            .to_ascii_lowercase();
        attempt_context.attempt_id = attempt_context.attempt_id.to_ascii_lowercase();
    }
}

pub(crate) fn canonicalize_included_participants(
    included_participants: &[u16],
) -> Result<Vec<u16>, EngineError> {
    if included_participants.is_empty() {
        return Err(EngineError::Validation(
            "attempt_context.included_participants must not be empty".to_string(),
        ));
    }

    let mut canonical = included_participants.to_vec();
    canonical.sort_unstable();

    let mut seen = HashSet::new();
    for participant_identifier in &canonical {
        if *participant_identifier == 0 {
            return Err(EngineError::Validation(
                "attempt_context.included_participants must contain non-zero identifiers"
                    .to_string(),
            ));
        }
        if !seen.insert(*participant_identifier) {
            return Err(EngineError::Validation(format!(
                "attempt_context.included_participants contains duplicate identifier [{}]",
                participant_identifier
            )));
        }
    }

    Ok(canonical)
}

pub(crate) fn push_framed_component(
    payload: &mut Vec<u8>,
    component: &[u8],
) -> Result<(), EngineError> {
    let component_len = u32::try_from(component.len()).map_err(|_| {
        EngineError::Validation("attempt_context component exceeds u32 framing limit".to_string())
    })?;
    payload.extend_from_slice(&component_len.to_be_bytes());
    payload.extend_from_slice(component);
    Ok(())
}

pub(crate) fn roast_hash_hex_with_components(
    domain: &str,
    components: &[&[u8]],
) -> Result<String, EngineError> {
    let mut payload = Vec::new();
    push_framed_component(&mut payload, domain.as_bytes())?;
    for component in components {
        push_framed_component(&mut payload, component)?;
    }

    Ok(hash_hex(&payload))
}

/// Computes the RFC-21 `MessageDigest` from the raw signing message
/// bytes, mirroring keep-core's `messageDigestFromBigInt` exactly: the
/// message **is** the digest (in BIP-340 production the signed message is
/// already a 32-byte sighash), leading zero bytes are insignificant
/// (Go round-trips through `big.Int`, which strips them), the value is
/// big-endian left-padded with zeros to exactly 32 bytes, and anything
/// longer than 32 significant bytes is rejected.
///
/// This is deliberately NOT the engine's internal transcript digest
/// (`hash_hex(message_bytes)` = SHA256 of the message), which continues
/// to feed `round_id`/`attempt_id` derivations. Feeding the transcript
/// digest into the shuffle seed was the cross-language divergence this
/// helper exists to prevent: the Go RFC-21 layer seeds from the padded
/// message itself.
pub(crate) fn rfc21_message_digest(message_bytes: &[u8]) -> Result<[u8; 32], EngineError> {
    let significant_bytes = {
        let first_significant_index = message_bytes
            .iter()
            .position(|byte| *byte != 0)
            .unwrap_or(message_bytes.len());
        &message_bytes[first_significant_index..]
    };

    if significant_bytes.len() > 32 {
        return Err(EngineError::Validation(format!(
            "message length [{}] exceeds the RFC-21 32-byte message digest; \
             attempt contexts only bind 32-byte signing digests",
            significant_bytes.len()
        )));
    }

    let mut digest = [0_u8; 32];
    digest[32 - significant_bytes.len()..].copy_from_slice(significant_bytes);
    Ok(digest)
}

/// Derives the legacy `i64` coordinator-shuffle seed per RFC-21 Annex A
/// (normative; see `docs/roast-coordinator-seed-derivation.md`):
///
/// ```text
/// AttemptSeed32   = SHA256(KeyGroupBytes || SessionID || MessageDigest)
/// ShuffleSeed_i64 = int64_from_be_bytes(AttemptSeed32[0..8])
/// ```
///
/// `key_group` is the canonical FROST key-group handle (for this engine:
/// the lowercase hex encoding of the serialized group verifying key); its
/// UTF-8 bytes feed the hash as an opaque string, matching keep-core's
/// `attempt.DeriveAttemptSeed` + `foldAttemptSeed` composition exactly.
/// `rfc21_message_digest` is the padded raw signing message (see
/// `rfc21_message_digest`), NOT the engine's SHA256 transcript digest.
/// The shuffle-source composition adds the RFC-21 **0-based** attempt
/// number; callers holding the 1-based wire attempt number must subtract
/// one before composing.
///
/// Cross-language agreement is pinned by
/// `testdata/coordinator_seed_vectors.json`, a byte-identical copy of the
/// canonical file generated from the Go implementation in
/// `pkg/frost/roast` on the RFC-21 branch.
pub(crate) fn roast_attempt_shuffle_seed(
    key_group: &str,
    session_id: &str,
    rfc21_message_digest: &[u8; 32],
) -> Result<i64, EngineError> {
    let mut hasher = Sha256::new();
    hasher.update(key_group.as_bytes());
    hasher.update(session_id.as_bytes());
    hasher.update(rfc21_message_digest);
    let attempt_seed = hasher.finalize();

    let mut seed_bytes = [0_u8; 8];
    seed_bytes.copy_from_slice(&attempt_seed[..8]);
    Ok(i64::from_be_bytes(seed_bytes))
}

pub(crate) fn roast_included_participants_fingerprint_hex(
    included_participants: &[u16],
) -> Result<String, EngineError> {
    let mut participant_payload = Vec::new();
    for participant_identifier in included_participants {
        push_framed_component(
            &mut participant_payload,
            &participant_identifier.to_be_bytes(),
        )?;
    }

    roast_hash_hex_with_components(
        ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN,
        &[&participant_payload],
    )
}

pub(crate) fn roast_attempt_id_hex(
    session_id: &str,
    message_digest_hex: &str,
    attempt_number: u32,
    coordinator_identifier: u16,
    included_participants_fingerprint_hex: &str,
) -> Result<String, EngineError> {
    roast_hash_hex_with_components(
        ROAST_ATTEMPT_ID_DOMAIN,
        &[
            session_id.as_bytes(),
            message_digest_hex.as_bytes(),
            &attempt_number.to_be_bytes(),
            &coordinator_identifier.to_be_bytes(),
            included_participants_fingerprint_hex.as_bytes(),
        ],
    )
}

pub(crate) fn validate_attempt_context(
    session_id: &str,
    key_group: &str,
    message_bytes: &[u8],
    message_digest_hex: &str,
    threshold: u16,
    attempt_context: Option<&AttemptContext>,
    strict_mode_enabled: bool,
) -> Result<Option<Vec<u16>>, EngineError> {
    let Some(attempt_context) = attempt_context else {
        if strict_mode_enabled {
            return Err(EngineError::Validation(
                "attempt_context is required when ROAST strict mode is enabled".to_string(),
            ));
        }
        return Ok(None);
    };

    if attempt_context.attempt_number == 0 {
        return Err(EngineError::Validation(
            "attempt_context.attempt_number must be at least 1".to_string(),
        ));
    }

    if attempt_context.coordinator_identifier == 0 {
        return Err(EngineError::Validation(
            "attempt_context.coordinator_identifier must be non-zero".to_string(),
        ));
    }

    let canonical_included_participants =
        canonicalize_included_participants(&attempt_context.included_participants)?;

    if canonical_included_participants.len() < usize::from(threshold) {
        return Err(EngineError::Validation(format!(
            "attempt_context.included_participants must contain at least threshold members [{}]",
            threshold
        )));
    }

    if !canonical_included_participants.contains(&attempt_context.coordinator_identifier) {
        return Err(EngineError::Validation(
            "attempt_context.coordinator_identifier must be included in attempt_context.included_participants".to_string(),
        ));
    }

    // The shuffle seed binds the RFC-21 MessageDigest -- the padded raw
    // signing message, exactly as the Go layer's
    // `messageDigestFromBigInt` produces it -- NOT the engine's SHA256
    // transcript digest (`message_digest_hex`), which feeds only the
    // `attempt_id` derivation below. Mixing the two was the
    // coordinator-selection divergence flagged on the seed-unification
    // review.
    let attempt_seed =
        roast_attempt_shuffle_seed(key_group, session_id, &rfc21_message_digest(message_bytes)?)?;
    // The wire attempt_number is 1-based (enforced above); the RFC-21
    // Annex A shuffle composition uses the 0-based attempt number.
    let expected_coordinator_identifier = select_coordinator_identifier(
        &canonical_included_participants,
        attempt_seed,
        attempt_context.attempt_number - 1,
    )
    .ok_or_else(|| {
        EngineError::Validation(
            "attempt_context.included_participants must not be empty".to_string(),
        )
    })?;
    if expected_coordinator_identifier != attempt_context.coordinator_identifier {
        return Err(EngineError::Validation(
            "attempt_context.coordinator_identifier does not match deterministic coordinator selection".to_string(),
        ));
    }

    let expected_included_participants_fingerprint_hex =
        roast_included_participants_fingerprint_hex(&canonical_included_participants)?;

    if !attempt_context
        .included_participants_fingerprint
        .eq_ignore_ascii_case(&expected_included_participants_fingerprint_hex)
    {
        return Err(EngineError::Validation(
            "attempt_context.included_participants_fingerprint does not match canonical participants".to_string(),
        ));
    }

    let expected_attempt_id_hex = roast_attempt_id_hex(
        session_id,
        message_digest_hex,
        attempt_context.attempt_number,
        attempt_context.coordinator_identifier,
        &expected_included_participants_fingerprint_hex,
    )?;

    if !attempt_context
        .attempt_id
        .eq_ignore_ascii_case(&expected_attempt_id_hex)
    {
        return Err(EngineError::Validation(
            "attempt_context.attempt_id does not match canonical attempt context".to_string(),
        ));
    }

    Ok(Some(canonical_included_participants))
}

/// Derives the canonical interactive attempt context for an attempt from its
/// public inputs, so the host never re-implements the engine's domain-separated
/// derivations (the cross-language divergence class that bit the coordinator
/// seed). Stateless and secret-free: it touches no DKG, nonce, or session state.
///
/// The returned context is re-validated against strict-mode
/// `validate_attempt_context` for the same inputs before returning, so it is
/// guaranteed to be accepted by `interactive_session_open`; the per-participant
/// FROST identifiers use the canonical key-package encoding the
/// signing-package/aggregate paths require.
pub(crate) fn derive_interactive_attempt_context(
    request: DeriveInteractiveAttemptContextRequest,
) -> Result<DeriveInteractiveAttemptContextResult, EngineError> {
    // Mirror interactive_session_open's front door (and every other engine
    // endpoint, including the public-material-only verify_signature_share): an
    // unattested engine, or a session_id open would reject, must fail closed
    // here too rather than hand back a context the real open refuses.
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    let message_bytes = hex::decode(&request.message_hex)
        .map_err(|e| EngineError::Validation(format!("message_hex is not valid hex: {e}")))?;
    if message_bytes.is_empty() {
        return Err(EngineError::Validation(
            "message_hex must not be empty".to_string(),
        ));
    }
    if request.attempt_number == 0 {
        return Err(EngineError::Validation(
            "attempt_number must be at least 1".to_string(),
        ));
    }
    // interactive_session_open rejects threshold == 0 BEFORE validating the
    // context, and validate_attempt_context only checks len >= threshold (always
    // true for 0). Reject it here too so the helper never hands the host a
    // context open would reject - a missing/uninitialized threshold fails at the
    // derivation seam, not later at open.
    if request.threshold == 0 {
        return Err(EngineError::Validation(
            "threshold must be non-zero".to_string(),
        ));
    }

    let canonical_included_participants =
        canonicalize_included_participants(&request.included_participants)?;
    if canonical_included_participants.len() < usize::from(request.threshold) {
        return Err(EngineError::Validation(format!(
            "included_participants must contain at least threshold members [{}]",
            request.threshold
        )));
    }

    // Coordinator: the RFC-21 Annex A shuffle binds the padded raw message
    // digest and uses the 0-based attempt number.
    let attempt_seed = roast_attempt_shuffle_seed(
        &request.key_group,
        &request.session_id,
        &rfc21_message_digest(&message_bytes)?,
    )?;
    let coordinator_identifier = select_coordinator_identifier(
        &canonical_included_participants,
        attempt_seed,
        request.attempt_number - 1,
    )
    .ok_or_else(|| {
        EngineError::Validation("included_participants must not be empty".to_string())
    })?;

    // Fingerprint over the canonical set; the attempt_id binds the engine's
    // SHA256 transcript digest of the message (NOT the RFC-21 shuffle digest).
    let included_participants_fingerprint =
        roast_included_participants_fingerprint_hex(&canonical_included_participants)?;
    let message_digest_hex = hash_hex(&message_bytes);
    let attempt_id = roast_attempt_id_hex(
        &request.session_id,
        &message_digest_hex,
        request.attempt_number,
        coordinator_identifier,
        &included_participants_fingerprint,
    )?;

    let attempt_context = AttemptContext {
        attempt_number: request.attempt_number,
        coordinator_identifier,
        included_participants: canonical_included_participants.clone(),
        included_participants_fingerprint,
        attempt_id,
    };

    // Post-condition: the derived context MUST satisfy the same strict-mode
    // validator `interactive_session_open` runs, so the host can never be handed
    // a context the engine would later reject. A failure here is an internal
    // derivation inconsistency, surfaced rather than shipped.
    validate_attempt_context(
        &request.session_id,
        &request.key_group,
        &message_bytes,
        &message_digest_hex,
        request.threshold,
        Some(&attempt_context),
        true,
    )?;

    let frost_identifiers = canonical_included_participants
        .iter()
        .map(|participant| {
            Ok(ParticipantFrostIdentifier {
                participant_identifier: *participant,
                frost_identifier: frost_identifier_to_go_string(
                    participant_identifier_to_frost_identifier(*participant)?,
                ),
            })
        })
        .collect::<Result<Vec<_>, EngineError>>()?;

    Ok(DeriveInteractiveAttemptContextResult {
        attempt_context,
        frost_identifiers,
    })
}

pub(crate) fn canonical_attempt_context(attempt_context: &AttemptContext) -> AttemptContext {
    let mut canonical = Some(attempt_context.clone());
    canonicalize_attempt_context_for_fingerprint(&mut canonical);
    canonical.expect("attempt context canonicalization preserves value")
}

pub(crate) fn enforce_not_quarantined_identifiers(
    session_id: &str,
    member_identifiers: &[u16],
    quarantined_operator_identifiers: &HashSet<u16>,
    auto_quarantine_config: Option<&AutoQuarantineConfig>,
) -> Result<(), EngineError> {
    let Some(auto_quarantine_config) = auto_quarantine_config else {
        return Ok(());
    };

    for member_identifier in member_identifiers {
        if auto_quarantine_config
            .dao_allowlist_identifiers
            .contains(member_identifier)
        {
            continue;
        }
        if quarantined_operator_identifiers.contains(member_identifier) {
            return reject_quarantine_policy(
                session_id,
                "operator_auto_quarantined",
                format!(
                    "operator identifier [{}] is auto-quarantined and requires DAO allowlist override",
                    member_identifier
                ),
            );
        }
    }

    Ok(())
}

pub(crate) fn validate_session_id(session_id: &str) -> Result<(), EngineError> {
    if session_id.is_empty() {
        return Err(EngineError::Validation(
            "session_id must be non-empty".to_string(),
        ));
    }

    if session_id.len() > 128 {
        return Err(EngineError::Validation(
            "session_id exceeds max length 128 bytes".to_string(),
        ));
    }

    if session_id.bytes().any(|byte| {
        byte.is_ascii_control() || byte == b' ' || byte == b'=' || byte == b'"' || byte == b'\\'
    }) {
        return Err(EngineError::Validation(
            "session_id contains disallowed characters (control, space, =, \", \\)".to_string(),
        ));
    }

    Ok(())
}
