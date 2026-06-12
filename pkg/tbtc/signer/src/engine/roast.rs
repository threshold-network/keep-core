// ROAST/RFC-21 attempt machinery: request fingerprints, round/attempt ids, attempt-context and transition-evidence validation.

use super::*;

pub(crate) const ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN: &str =
    "FROST-ROAST-INCLUDED-FPR-v1";

pub(crate) const ROAST_ATTEMPT_ID_DOMAIN: &str = "FROST-ROAST-ATTEMPT-ID-v1";

pub(crate) const ROUND_ID_NO_ATTEMPT_CONTEXT_COMPONENT: &str = "none";

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

pub(crate) fn canonicalize_dkg_request_for_fingerprint(request: &RunDkgRequest) -> RunDkgRequest {
    let mut canonical_request = request.clone();
    canonical_request
        .participants
        .sort_unstable_by(|left, right| {
            left.identifier
                .cmp(&right.identifier)
                .then_with(|| left.public_key_hex.cmp(&right.public_key_hex))
        });
    canonical_request
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

pub(crate) fn canonicalize_attempt_transition_evidence_for_fingerprint(
    transition_evidence: &mut Option<AttemptTransitionEvidence>,
) {
    if let Some(transition_evidence) = transition_evidence.as_mut() {
        transition_evidence.from_attempt_id = transition_evidence
            .from_attempt_id
            .trim()
            .to_ascii_lowercase();
        if let Some(exclusion_evidence) = transition_evidence.exclusion_evidence.as_mut() {
            exclusion_evidence.reason = exclusion_evidence.reason.trim().to_ascii_lowercase();
            exclusion_evidence
                .excluded_member_identifiers
                .sort_unstable();
            if let Some(proof_fingerprint) =
                exclusion_evidence.invalid_share_proof_fingerprint.as_mut()
            {
                *proof_fingerprint = proof_fingerprint.trim().to_ascii_lowercase();
            }
        }
    }
}

pub(crate) fn start_sign_round_request_fingerprint(
    request: &StartSignRoundRequest,
    member_identifier: u16,
) -> Result<String, EngineError> {
    start_sign_round_request_fingerprint_internal(request, member_identifier, false)
}

pub(crate) fn start_sign_round_request_fingerprint_including_transition_evidence(
    request: &StartSignRoundRequest,
    member_identifier: u16,
) -> Result<String, EngineError> {
    start_sign_round_request_fingerprint_internal(request, member_identifier, true)
}

pub(crate) fn start_sign_round_request_fingerprint_internal(
    request: &StartSignRoundRequest,
    member_identifier: u16,
    include_transition_evidence: bool,
) -> Result<String, EngineError> {
    let mut canonical_request = request.clone();
    canonical_request.member_identifier = member_identifier;
    if let Some(signing_participants) = canonical_request.signing_participants.as_mut() {
        signing_participants.sort_unstable();
    }
    canonicalize_attempt_context_for_fingerprint(&mut canonical_request.attempt_context);
    if include_transition_evidence {
        canonicalize_attempt_transition_evidence_for_fingerprint(
            &mut canonical_request.attempt_transition_evidence,
        );
    } else {
        // Transition evidence authorizes creation of a new active attempt but is
        // one-shot material. Once the active attempt context is established,
        // other members may reuse the round without resending the evidence.
        canonical_request.attempt_transition_evidence = None;
    }

    fingerprint(&canonical_request)
}

pub(crate) fn round_attempt_id_component(attempt_context: Option<&AttemptContext>) -> String {
    attempt_context
        .map(|attempt_context| attempt_context.attempt_id.to_ascii_lowercase())
        .unwrap_or_else(|| ROUND_ID_NO_ATTEMPT_CONTEXT_COMPONENT.to_string())
}

pub(crate) fn derive_round_id(
    session_id: &str,
    key_group: &str,
    message_hex: &str,
    taproot_merkle_root_hex: Option<&str>,
    signing_participants_fingerprint: &str,
    attempt_context: Option<&AttemptContext>,
) -> String {
    let attempt_id_component = round_attempt_id_component(attempt_context);
    let taproot_merkle_root_component = taproot_merkle_root_hex.unwrap_or("no-taproot-merkle-root");
    hash_hex(
        format!(
            "round:{}:{}:{}:{}:{}:{}",
            session_id,
            key_group,
            message_hex,
            taproot_merkle_root_component,
            signing_participants_fingerprint,
            attempt_id_component
        )
        .as_bytes(),
    )
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

pub(crate) fn canonical_attempt_context(attempt_context: &AttemptContext) -> AttemptContext {
    let mut canonical = Some(attempt_context.clone());
    canonicalize_attempt_context_for_fingerprint(&mut canonical);
    canonical.expect("attempt context canonicalization preserves value")
}

pub(crate) enum ActiveAttemptMatchOutcome {
    MatchActive,
    AdvanceAuthorized,
}

pub(crate) fn validate_transition_exclusion_evidence(
    exclusion_evidence: Option<&AttemptExclusionEvidence>,
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: &AttemptContext,
) -> Result<(), EngineError> {
    let exclusion_evidence = exclusion_evidence.ok_or_else(|| {
        EngineError::Validation(
            "attempt_transition_evidence.exclusion_evidence is required for attempt advancement"
                .to_string(),
        )
    })?;

    let reason = exclusion_evidence.reason.trim().to_ascii_lowercase();
    if reason != ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT
        && reason != ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF
    {
        return Err(EngineError::Validation(format!(
            "attempt_transition_evidence.exclusion_evidence.reason [{}] is unsupported",
            exclusion_evidence.reason
        )));
    }

    let mut excluded_member_identifiers = HashSet::new();
    for member_identifier in &exclusion_evidence.excluded_member_identifiers {
        if *member_identifier == 0 {
            return Err(EngineError::Validation(
                "attempt_transition_evidence.exclusion_evidence.excluded_member_identifiers must contain non-zero identifiers".to_string(),
            ));
        }
        if !excluded_member_identifiers.insert(*member_identifier) {
            return Err(EngineError::Validation(format!(
                "attempt_transition_evidence.exclusion_evidence.excluded_member_identifiers contains duplicate identifier [{}]",
                member_identifier
            )));
        }
        if !active_attempt_context
            .included_participants
            .contains(member_identifier)
        {
            return Err(EngineError::Validation(format!(
                "attempt_transition_evidence.exclusion_evidence.excluded_member_identifiers contains identifier [{}] not present in active attempt context",
                member_identifier
            )));
        }
    }

    for member_identifier in &excluded_member_identifiers {
        if incoming_attempt_context
            .included_participants
            .contains(member_identifier)
        {
            return Err(EngineError::Validation(format!(
                "attempt_transition_evidence.exclusion_evidence identifier [{}] must not remain in incoming attempt_context.included_participants",
                member_identifier
            )));
        }
    }

    if excluded_member_identifiers.contains(&incoming_attempt_context.coordinator_identifier) {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.exclusion_evidence must not exclude incoming attempt_context.coordinator_identifier".to_string(),
        ));
    }

    match reason.as_str() {
        ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT => {
            // `coordinator_timeout` may intentionally exclude zero members.
            // This models coordinator rotation without participant-level fault
            // attribution, so no auto-quarantine penalty is applied.
            if exclusion_evidence.invalid_share_proof_fingerprint.is_some() {
                return Err(EngineError::Validation(
                    "attempt_transition_evidence.exclusion_evidence.invalid_share_proof_fingerprint must be omitted for coordinator_timeout reason".to_string(),
                ));
            }
        }
        ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF => {
            if excluded_member_identifiers.is_empty() {
                return Err(EngineError::Validation(
                    "attempt_transition_evidence.exclusion_evidence.excluded_member_identifiers must contain at least one identifier for invalid_share_proof reason".to_string(),
                ));
            }
            let proof_fingerprint = exclusion_evidence
                .invalid_share_proof_fingerprint
                .as_deref()
                .ok_or_else(|| {
                    EngineError::Validation(
                        "attempt_transition_evidence.exclusion_evidence.invalid_share_proof_fingerprint is required for invalid_share_proof reason".to_string(),
                    )
                })?;
            let proof_fingerprint = proof_fingerprint.trim();
            if proof_fingerprint.is_empty() {
                return Err(EngineError::Validation(
                    "attempt_transition_evidence.exclusion_evidence.invalid_share_proof_fingerprint must be non-empty valid hex".to_string(),
                ));
            }
            hex::decode(proof_fingerprint).map_err(|_| {
                EngineError::Validation(
                    "attempt_transition_evidence.exclusion_evidence.invalid_share_proof_fingerprint must be valid hex".to_string(),
                )
            })?;
        }
        _ => unreachable!("reason value filtered above"),
    }

    Ok(())
}

pub(crate) fn build_attempt_transition_telemetry(
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: &AttemptContext,
    transition_evidence: Option<&AttemptTransitionEvidence>,
) -> Option<AttemptTransitionTelemetry> {
    let exclusion_evidence = transition_evidence?.exclusion_evidence.as_ref()?;
    let mut excluded_member_identifiers = exclusion_evidence.excluded_member_identifiers.clone();
    excluded_member_identifiers.sort_unstable();

    Some(AttemptTransitionTelemetry {
        from_attempt_number: active_attempt_context.attempt_number,
        to_attempt_number: incoming_attempt_context.attempt_number,
        from_coordinator_identifier: active_attempt_context.coordinator_identifier,
        to_coordinator_identifier: incoming_attempt_context.coordinator_identifier,
        reason: exclusion_evidence.reason.trim().to_ascii_lowercase(),
        excluded_member_identifiers,
        coordinator_rotated: active_attempt_context.coordinator_identifier
            != incoming_attempt_context.coordinator_identifier,
    })
}

pub(crate) fn build_transcript_audit_record(
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: &AttemptContext,
    transition_evidence: &AttemptTransitionEvidence,
) -> Result<TranscriptAuditRecord, EngineError> {
    let exclusion_evidence = transition_evidence
        .exclusion_evidence
        .as_ref()
        .ok_or_else(|| {
            EngineError::Internal("missing exclusion evidence for transcript record".to_string())
        })?;

    let mut excluded_member_identifiers = exclusion_evidence.excluded_member_identifiers.clone();
    excluded_member_identifiers.sort_unstable();

    let reason = exclusion_evidence.reason.trim().to_ascii_lowercase();
    let invalid_share_proof_fingerprint = exclusion_evidence
        .invalid_share_proof_fingerprint
        .as_deref()
        .map(|fingerprint| fingerprint.trim().to_ascii_lowercase());
    let mut record = TranscriptAuditRecord {
        from_attempt_number: active_attempt_context.attempt_number,
        to_attempt_number: incoming_attempt_context.attempt_number,
        from_attempt_id: active_attempt_context.attempt_id.to_ascii_lowercase(),
        to_attempt_id: incoming_attempt_context.attempt_id.to_ascii_lowercase(),
        previous_round_id: transition_evidence.previous_round_id.clone(),
        previous_sign_request_fingerprint: transition_evidence
            .previous_sign_request_fingerprint
            .clone(),
        from_coordinator_identifier: active_attempt_context.coordinator_identifier,
        to_coordinator_identifier: incoming_attempt_context.coordinator_identifier,
        reason,
        excluded_member_identifiers,
        invalid_share_proof_fingerprint,
        transcript_hash: String::new(),
        recorded_at_unix: now_unix(),
    };
    // Two-pass hash: fingerprint the canonical record with an empty
    // `transcript_hash` sentinel, then persist the resulting hash value.
    let transcript_hash = fingerprint(&record)?;
    record.transcript_hash = transcript_hash;
    Ok(record)
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

pub(crate) fn auto_quarantine_penalty_for_record(
    record: &TranscriptAuditRecord,
    auto_quarantine_config: &AutoQuarantineConfig,
) -> u64 {
    if record.reason == ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF {
        auto_quarantine_config.invalid_share_penalty
    } else {
        auto_quarantine_config.timeout_penalty
    }
}

pub(crate) fn apply_auto_quarantine_faults_for_transition(
    engine_state: &mut EngineState,
    session_id: &str,
    record: &TranscriptAuditRecord,
    auto_quarantine_config: Option<&AutoQuarantineConfig>,
) {
    let Some(auto_quarantine_config) = auto_quarantine_config else {
        return;
    };

    let penalty = auto_quarantine_penalty_for_record(record, auto_quarantine_config);
    for excluded_member_identifier in &record.excluded_member_identifiers {
        if auto_quarantine_config
            .dao_allowlist_identifiers
            .contains(excluded_member_identifier)
        {
            // Governance allowlist acts as explicit manual re-enable path.
            engine_state
                .quarantined_operator_identifiers
                .remove(excluded_member_identifier);
            continue;
        }

        let score = engine_state
            .operator_fault_scores
            .entry(*excluded_member_identifier)
            .or_insert(0);
        *score = score.saturating_add(penalty);
        record_hardening_telemetry(|telemetry| {
            telemetry.auto_quarantine_fault_events_total = telemetry
                .auto_quarantine_fault_events_total
                .saturating_add(1);
        });

        if *score >= auto_quarantine_config.fault_threshold
            && engine_state
                .quarantined_operator_identifiers
                .insert(*excluded_member_identifier)
        {
            record_hardening_telemetry(|telemetry| {
                telemetry.auto_quarantine_enforcements_total = telemetry
                    .auto_quarantine_enforcements_total
                    .saturating_add(1);
            });
            log_policy_decision(
                "auto_quarantine",
                session_id,
                "quarantine",
                "fault_threshold_reached",
            );
        }
    }
}

pub(crate) fn validate_attempt_transition_evidence(
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: &AttemptContext,
    transition_evidence: Option<&AttemptTransitionEvidence>,
    round_state: Option<&RoundState>,
    sign_request_fingerprint: Option<&str>,
) -> Result<(), EngineError> {
    let transition_evidence = transition_evidence.ok_or_else(|| {
        EngineError::Validation(
            "attempt_context.attempt_number advancement requires attempt_transition_evidence"
                .to_string(),
        )
    })?;

    if incoming_attempt_context.attempt_number != active_attempt_context.attempt_number + 1 {
        return Err(EngineError::Validation(format!(
            "attempt_context.attempt_number [{}] is ahead of active attempt_number [{}] without transition authorization",
            incoming_attempt_context.attempt_number, active_attempt_context.attempt_number
        )));
    }

    if transition_evidence.from_attempt_number != active_attempt_context.attempt_number {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.from_attempt_number does not match active attempt context"
                .to_string(),
        ));
    }

    if !transition_evidence
        .from_attempt_id
        .eq_ignore_ascii_case(&active_attempt_context.attempt_id)
    {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.from_attempt_id does not match active attempt context"
                .to_string(),
        ));
    }

    if transition_evidence.from_coordinator_identifier
        != active_attempt_context.coordinator_identifier
    {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.from_coordinator_identifier does not match active attempt context".to_string(),
        ));
    }

    validate_transition_exclusion_evidence(
        transition_evidence.exclusion_evidence.as_ref(),
        active_attempt_context,
        incoming_attempt_context,
    )?;

    let round_state = round_state.ok_or_else(|| {
        EngineError::Validation(
            "attempt_transition_evidence requires active round state".to_string(),
        )
    })?;
    if transition_evidence.previous_round_id != round_state.round_id {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.previous_round_id does not match active round state"
                .to_string(),
        ));
    }

    let sign_request_fingerprint = sign_request_fingerprint.ok_or_else(|| {
        EngineError::Validation(
            "attempt_transition_evidence requires active sign request fingerprint".to_string(),
        )
    })?;
    if transition_evidence.previous_sign_request_fingerprint != sign_request_fingerprint {
        return Err(EngineError::Validation(
            "attempt_transition_evidence.previous_sign_request_fingerprint does not match active sign request".to_string(),
        ));
    }

    if incoming_attempt_context
        .attempt_id
        .eq_ignore_ascii_case(&active_attempt_context.attempt_id)
    {
        return Err(EngineError::Validation(
            "attempt_context.attempt_id must change when advancing attempt_number".to_string(),
        ));
    }

    Ok(())
}

pub(crate) fn enforce_active_attempt_context_match(
    active_attempt_context: &AttemptContext,
    incoming_attempt_context: Option<&AttemptContext>,
    transition_evidence: Option<&AttemptTransitionEvidence>,
    round_state: Option<&RoundState>,
    sign_request_fingerprint: Option<&str>,
    strict_mode_enabled: bool,
) -> Result<ActiveAttemptMatchOutcome, EngineError> {
    let Some(incoming_attempt_context) = incoming_attempt_context else {
        if !strict_mode_enabled {
            return Ok(ActiveAttemptMatchOutcome::MatchActive);
        }
        return Err(EngineError::Validation(
            "attempt_context is required when ROAST strict mode is enabled or an active attempt context exists".to_string(),
        ));
    };

    let incoming_attempt_context = canonical_attempt_context(incoming_attempt_context);

    if incoming_attempt_context.attempt_number < active_attempt_context.attempt_number {
        return Err(EngineError::Validation(format!(
            "attempt_context.attempt_number [{}] is stale; active attempt_number is [{}]",
            incoming_attempt_context.attempt_number, active_attempt_context.attempt_number
        )));
    }

    if incoming_attempt_context.attempt_number > active_attempt_context.attempt_number {
        validate_attempt_transition_evidence(
            active_attempt_context,
            &incoming_attempt_context,
            transition_evidence,
            round_state,
            sign_request_fingerprint,
        )?;

        return Ok(ActiveAttemptMatchOutcome::AdvanceAuthorized);
    }

    if incoming_attempt_context.coordinator_identifier
        != active_attempt_context.coordinator_identifier
    {
        return Err(EngineError::Validation(format!(
            "attempt_context.coordinator_identifier [{}] does not match active coordinator [{}]",
            incoming_attempt_context.coordinator_identifier,
            active_attempt_context.coordinator_identifier
        )));
    }

    if incoming_attempt_context.included_participants
        != active_attempt_context.included_participants
    {
        return Err(EngineError::Validation(
            "attempt_context.included_participants does not match active attempt context"
                .to_string(),
        ));
    }

    if incoming_attempt_context.included_participants_fingerprint
        != active_attempt_context.included_participants_fingerprint
    {
        return Err(EngineError::Validation(
            "attempt_context.included_participants_fingerprint does not match active attempt context"
                .to_string(),
        ));
    }

    if incoming_attempt_context.attempt_id != active_attempt_context.attempt_id {
        return Err(EngineError::Validation(
            "attempt_context.attempt_id does not match active attempt context".to_string(),
        ));
    }

    Ok(ActiveAttemptMatchOutcome::MatchActive)
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

pub(crate) fn clear_session_signing_material(session: &mut SessionState) {
    // Intentionally retain `dkg_result` and `dkg_request_fingerprint` because
    // RefreshShares is an independent post-DKG flow.
    //
    // Best-effort zeroization: clear byte/string material we own directly
    // before dropping Option containers.
    if let Some(sign_request_fingerprint) = session.sign_request_fingerprint.as_mut() {
        sign_request_fingerprint.zeroize();
    }
    if let Some(sign_message_bytes) = session.sign_message_bytes.as_mut() {
        sign_message_bytes.zeroize();
    }
    if let Some(round_state) = session.round_state.as_mut() {
        round_state.session_id.zeroize();
        round_state.round_id.zeroize();
        round_state.message_digest_hex.zeroize();
        if let Some(signing_participants) = round_state.signing_participants.as_mut() {
            signing_participants.zeroize();
        }
        if let Some(transition_telemetry) = round_state.attempt_transition_telemetry.as_mut() {
            transition_telemetry.from_attempt_number.zeroize();
            transition_telemetry.to_attempt_number.zeroize();
            transition_telemetry.from_coordinator_identifier.zeroize();
            transition_telemetry.to_coordinator_identifier.zeroize();
            transition_telemetry.reason.zeroize();
            transition_telemetry.excluded_member_identifiers.zeroize();
            transition_telemetry.coordinator_rotated = false;
        }
        round_state.own_contribution.identifier.zeroize();
        round_state.own_contribution.signature_share_hex.zeroize();
    }
    if let Some(active_attempt_context) = session.active_attempt_context.as_mut() {
        active_attempt_context.included_participants.zeroize();
        active_attempt_context
            .included_participants_fingerprint
            .zeroize();
        active_attempt_context.attempt_id.zeroize();
    }

    session.dkg_key_packages = None;
    session.dkg_public_key_package = None;
    session.sign_request_fingerprint = None;
    session.sign_message_bytes = None;
    session.round_state = None;
    session.active_attempt_context = None;
}

pub(crate) fn clear_active_sign_round_for_attempt_transition(session: &mut SessionState) {
    if let Some(sign_request_fingerprint) = session.sign_request_fingerprint.as_mut() {
        sign_request_fingerprint.zeroize();
    }
    if let Some(sign_message_bytes) = session.sign_message_bytes.as_mut() {
        sign_message_bytes.zeroize();
    }
    if let Some(round_state) = session.round_state.as_mut() {
        round_state.session_id.zeroize();
        round_state.round_id.zeroize();
        round_state.message_digest_hex.zeroize();
        if let Some(signing_participants) = round_state.signing_participants.as_mut() {
            signing_participants.zeroize();
        }
        if let Some(transition_telemetry) = round_state.attempt_transition_telemetry.as_mut() {
            transition_telemetry.from_attempt_number.zeroize();
            transition_telemetry.to_attempt_number.zeroize();
            transition_telemetry.from_coordinator_identifier.zeroize();
            transition_telemetry.to_coordinator_identifier.zeroize();
            transition_telemetry.reason.zeroize();
            transition_telemetry.excluded_member_identifiers.zeroize();
            transition_telemetry.coordinator_rotated = false;
        }
        round_state.own_contribution.identifier.zeroize();
        round_state.own_contribution.signature_share_hex.zeroize();
    }

    session.sign_request_fingerprint = None;
    session.sign_message_bytes = None;
    session.round_state = None;
}
