// Forensics: transcript audit, blame-proof verification, differential fuzzing references.
// Split from the former single-file engine.rs (2026-06); see mod.rs.

use super::*;

pub(crate) fn reference_roast_hash_hex(
    domain: &str,
    components: &[Vec<u8>],
) -> Result<String, EngineError> {
    let mut payload = Vec::new();
    let domain_bytes = domain.as_bytes();
    let domain_len = u32::try_from(domain_bytes.len()).map_err(|_| {
        EngineError::Validation("reference hash domain exceeds u32 framing limit".to_string())
    })?;
    payload.extend_from_slice(&domain_len.to_be_bytes());
    payload.extend_from_slice(domain_bytes);

    for component in components {
        let component_len = u32::try_from(component.len()).map_err(|_| {
            EngineError::Validation(
                "reference hash component exceeds u32 framing limit".to_string(),
            )
        })?;
        payload.extend_from_slice(&component_len.to_be_bytes());
        payload.extend_from_slice(component);
    }

    Ok(hash_hex(&payload))
}

pub(crate) fn reference_roast_included_participants_fingerprint_hex(
    included_participants: &[u16],
) -> Result<String, EngineError> {
    let mut participant_payload = Vec::new();
    for participant_identifier in included_participants {
        let participant_component = participant_identifier.to_be_bytes();
        let component_len = u32::try_from(participant_component.len()).map_err(|_| {
            EngineError::Validation(
                "reference participant component exceeds u32 framing limit".to_string(),
            )
        })?;
        participant_payload.extend_from_slice(&component_len.to_be_bytes());
        participant_payload.extend_from_slice(&participant_component);
    }

    reference_roast_hash_hex(
        ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN,
        &[participant_payload],
    )
}

pub(crate) fn reference_roast_attempt_id_hex(
    session_id: &str,
    message_digest_hex: &str,
    attempt_number: u32,
    coordinator_identifier: u16,
    included_participants_fingerprint_hex: &str,
) -> Result<String, EngineError> {
    reference_roast_hash_hex(
        ROAST_ATTEMPT_ID_DOMAIN,
        &[
            session_id.as_bytes().to_vec(),
            message_digest_hex.as_bytes().to_vec(),
            attempt_number.to_be_bytes().to_vec(),
            coordinator_identifier.to_be_bytes().to_vec(),
            included_participants_fingerprint_hex.as_bytes().to_vec(),
        ],
    )
}

pub(crate) fn differential_case_count(case_count: u32) -> u32 {
    if case_count == 0 {
        return TBTC_SIGNER_DIFFERENTIAL_FUZZ_DEFAULT_CASES;
    }

    case_count.min(TBTC_SIGNER_DIFFERENTIAL_FUZZ_MAX_CASES)
}

pub fn run_differential_fuzzing(
    request: DifferentialFuzzRequest,
) -> Result<DifferentialFuzzResult, EngineError> {
    enforce_provenance_gate()?;
    let case_count = differential_case_count(request.case_count);
    let seed = if request.seed == 0 {
        0xD1FF_E2E0_A11C_0001
    } else {
        request.seed
    };
    let mut rng = ChaCha20Rng::seed_from_u64(seed);
    let mut divergences = Vec::new();
    let mut critical_divergence_count = 0_u32;

    for case_index in 0..case_count {
        let mut participants = Vec::new();
        let participant_count = (rng.next_u32() % 4 + 2) as usize;
        while participants.len() < participant_count {
            let candidate = (rng.next_u32() % 30 + 1) as u16;
            if !participants.contains(&candidate) {
                participants.push(candidate);
            }
        }
        if participants.len() > 1 {
            let swap_index = (rng.next_u32() as usize) % participants.len();
            participants.swap(0, swap_index);
        }

        let mut digest_bytes = [0_u8; 32];
        rng.fill_bytes(&mut digest_bytes);
        let message_digest_hex = hex::encode(digest_bytes);
        let session_id = format!("differential-session-{seed:016x}-{case_index}");
        let attempt_number = (rng.next_u32() % 16) + 1;
        let coordinator_identifier = participants[(rng.next_u32() as usize) % participants.len()];

        let primary_fingerprint = roast_included_participants_fingerprint_hex(&participants)?;
        let reference_fingerprint =
            reference_roast_included_participants_fingerprint_hex(&participants)?;
        if primary_fingerprint != reference_fingerprint {
            critical_divergence_count = critical_divergence_count.saturating_add(1);
            divergences.push(DifferentialDivergence {
                case_index,
                check: "included_participants_fingerprint".to_string(),
                severity: "critical".to_string(),
                detail: format!(
                    "primary [{}] != reference [{}]",
                    primary_fingerprint, reference_fingerprint
                ),
            });
        }

        let primary_attempt_id = roast_attempt_id_hex(
            &session_id,
            &message_digest_hex,
            attempt_number,
            coordinator_identifier,
            &primary_fingerprint,
        )?;
        let reference_attempt_id = reference_roast_attempt_id_hex(
            &session_id,
            &message_digest_hex,
            attempt_number,
            coordinator_identifier,
            &reference_fingerprint,
        )?;
        if primary_attempt_id != reference_attempt_id {
            critical_divergence_count = critical_divergence_count.saturating_add(1);
            divergences.push(DifferentialDivergence {
                case_index,
                check: "attempt_id".to_string(),
                severity: "critical".to_string(),
                detail: format!(
                    "primary [{}] != reference [{}]",
                    primary_attempt_id, reference_attempt_id
                ),
            });
        }

        let mut txid_bytes = [0_u8; 32];
        rng.fill_bytes(&mut txid_bytes);
        let txid_hex = hex::encode(txid_bytes);
        let txid = Txid::from_str(&txid_hex).map_err(|_| {
            EngineError::Internal("failed to build differential fuzz txid".to_string())
        })?;
        let mut script_pubkey = vec![0x51, 0x20];
        let mut witness_program = [0_u8; 32];
        rng.fill_bytes(&mut witness_program);
        script_pubkey.extend_from_slice(&witness_program);
        let tx = Transaction {
            version: Version::TWO,
            lock_time: LockTime::ZERO,
            input: vec![TxIn {
                previous_output: OutPoint {
                    txid,
                    vout: rng.next_u32() % 4,
                },
                script_sig: ScriptBuf::new(),
                sequence: Sequence::MAX,
                witness: Witness::default(),
            }],
            output: vec![TxOut {
                value: Amount::from_sat((rng.next_u32() as u64 % 1_000_000) + 1),
                script_pubkey: ScriptBuf::from_bytes(script_pubkey),
            }],
        };
        let tx_hex = serialize_hex(&tx);
        let primary_message_digest_hex = policy_bound_signing_message_hex(&tx_hex)?;
        let tx_bytes = hex::decode(&tx_hex).map_err(|_| {
            EngineError::Internal("failed to decode differential tx hex".to_string())
        })?;
        let tx_roundtrip: Transaction = deserialize(&tx_bytes).map_err(|error| {
            EngineError::Internal(format!("failed to deserialize differential tx: {error}"))
        })?;
        let reference_message_digest_hex =
            hash_hex(&bitcoin::consensus::encode::serialize(&tx_roundtrip));
        if primary_message_digest_hex != reference_message_digest_hex {
            critical_divergence_count = critical_divergence_count.saturating_add(1);
            divergences.push(DifferentialDivergence {
                case_index,
                check: "policy_bound_message_digest".to_string(),
                severity: "critical".to_string(),
                detail: format!(
                    "primary [{}] != reference [{}]",
                    primary_message_digest_hex, reference_message_digest_hex
                ),
            });
        }
    }

    record_hardening_telemetry(|telemetry| {
        telemetry.differential_fuzz_runs_total =
            telemetry.differential_fuzz_runs_total.saturating_add(1);
        telemetry.differential_fuzz_critical_divergence_total = telemetry
            .differential_fuzz_critical_divergence_total
            .saturating_add(critical_divergence_count as u64);
    });

    Ok(DifferentialFuzzResult {
        seed,
        case_count,
        divergences,
        critical_divergence_count,
        unresolved_critical_divergence: critical_divergence_count > 0,
    })
}

pub fn roast_transcript_audit(
    request: TranscriptAuditRequest,
) -> Result<TranscriptAuditResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.roast_transcript_audit_calls_total = telemetry
            .roast_transcript_audit_calls_total
            .saturating_add(1);
    });
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let session =
        guard
            .sessions
            .get(&request.session_id)
            .ok_or_else(|| EngineError::SessionNotFound {
                session_id: request.session_id.clone(),
            })?;
    let records = session.attempt_transition_records.clone();

    let result = TranscriptAuditResult {
        session_id: request.session_id,
        transition_count: records.len() as u64,
        records,
    };
    record_hardening_telemetry(|telemetry| {
        telemetry.roast_transcript_audit_success_total = telemetry
            .roast_transcript_audit_success_total
            .saturating_add(1);
    });

    Ok(result)
}

pub fn verify_blame_proof(
    request: VerifyBlameProofRequest,
) -> Result<BlameProofVerificationResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.verify_blame_proof_calls_total =
            telemetry.verify_blame_proof_calls_total.saturating_add(1);
    });
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;
    if request.from_attempt_number == 0 {
        return Err(EngineError::Validation(
            "from_attempt_number must be at least 1".to_string(),
        ));
    }
    if request.accused_member_identifier == 0 {
        return Err(EngineError::Validation(
            "accused_member_identifier must be non-zero".to_string(),
        ));
    }

    let reason = request.reason.trim().to_ascii_lowercase();
    if reason != ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT
        && reason != ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF
    {
        return Err(EngineError::Validation(format!(
            "reason [{}] is unsupported",
            request.reason
        )));
    }

    let requested_invalid_share_proof_fingerprint = request
        .invalid_share_proof_fingerprint
        .as_deref()
        .map(|fingerprint| fingerprint.trim().to_ascii_lowercase());
    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let session =
        guard
            .sessions
            .get(&request.session_id)
            .ok_or_else(|| EngineError::SessionNotFound {
                session_id: request.session_id.clone(),
            })?;

    let maybe_record = session
        .attempt_transition_records
        .iter()
        .find(|record| record.from_attempt_number == request.from_attempt_number);
    let (verified, detail, transcript_hash) = if let Some(record) = maybe_record {
        if record.reason != reason {
            (
                false,
                format!(
                    "reason mismatch: requested [{}], recorded [{}]",
                    reason, record.reason
                ),
                Some(record.transcript_hash.clone()),
            )
        } else if !record
            .excluded_member_identifiers
            .contains(&request.accused_member_identifier)
        {
            (
                false,
                format!(
                    "operator [{}] is not excluded in recorded transition",
                    request.accused_member_identifier
                ),
                Some(record.transcript_hash.clone()),
            )
        } else if reason == ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF
            && record.invalid_share_proof_fingerprint != requested_invalid_share_proof_fingerprint
        {
            (
                false,
                "invalid_share_proof_fingerprint does not match recorded transition evidence"
                    .to_string(),
                Some(record.transcript_hash.clone()),
            )
        } else {
            (
                true,
                "blame proof verified against persisted transcript record".to_string(),
                Some(record.transcript_hash.clone()),
            )
        }
    } else {
        (
            false,
            format!(
                "no persisted transition record for from_attempt_number [{}]",
                request.from_attempt_number
            ),
            None,
        )
    };

    if verified {
        record_hardening_telemetry(|telemetry| {
            telemetry.verify_blame_proof_success_total =
                telemetry.verify_blame_proof_success_total.saturating_add(1);
        });
    }

    Ok(BlameProofVerificationResult {
        session_id: request.session_id,
        from_attempt_number: request.from_attempt_number,
        accused_member_identifier: request.accused_member_identifier,
        reason,
        verified,
        transcript_hash,
        detail,
    })
}
