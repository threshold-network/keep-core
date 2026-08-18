// Forensics: transcript audit, blame-proof verification, differential fuzzing references.

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

// Independent BIP-341 SIGHASH_DEFAULT/key-spend construction for the
// differential fuzzer. This deliberately does not call rust-bitcoin's sighash
// module: the production path uses SighashCache, while this reference follows
// the SigMsg field order directly and uses consensus serialization only for the
// committed transaction primitives.
fn reference_bip341_key_spend_sighash_default(
    tx: &Transaction,
    prevouts: &[TxOut],
    input_index: usize,
) -> Result<String, EngineError> {
    if prevouts.len() != tx.input.len() || input_index >= tx.input.len() {
        return Err(EngineError::Internal(
            "invalid differential BIP-341 reference inputs".to_string(),
        ));
    }

    let mut prevouts_hasher = Sha256::new();
    let mut amounts_hasher = Sha256::new();
    let mut script_pubkeys_hasher = Sha256::new();
    let mut sequences_hasher = Sha256::new();
    for (input, prevout) in tx.input.iter().zip(prevouts) {
        prevouts_hasher.update(bitcoin::consensus::serialize(&input.previous_output));
        amounts_hasher.update(prevout.value.to_sat().to_le_bytes());
        script_pubkeys_hasher.update(bitcoin::consensus::serialize(&prevout.script_pubkey));
        sequences_hasher.update(input.sequence.0.to_le_bytes());
    }

    let mut outputs_hasher = Sha256::new();
    for output in &tx.output {
        outputs_hasher.update(bitcoin::consensus::serialize(output));
    }

    let mut sig_msg = Vec::with_capacity(175);
    sig_msg.push(0x00); // SIGHASH_DEFAULT.
    sig_msg.extend_from_slice(&tx.version.0.to_le_bytes());
    sig_msg.extend_from_slice(&tx.lock_time.to_consensus_u32().to_le_bytes());
    sig_msg.extend_from_slice(&prevouts_hasher.finalize());
    sig_msg.extend_from_slice(&amounts_hasher.finalize());
    sig_msg.extend_from_slice(&script_pubkeys_hasher.finalize());
    sig_msg.extend_from_slice(&sequences_hasher.finalize());
    sig_msg.extend_from_slice(&outputs_hasher.finalize());
    sig_msg.push(0x00); // ext_flag=0, annex_present=0.
    sig_msg.extend_from_slice(&(input_index as u32).to_le_bytes());

    let tag_hash = Sha256::digest(b"TapSighash");
    let mut tagged_hasher = Sha256::new();
    tagged_hasher.update(tag_hash);
    tagged_hasher.update(tag_hash);
    tagged_hasher.update([0x00]); // Epoch byte.
    tagged_hasher.update(sig_msg);
    Ok(hex::encode(tagged_hasher.finalize()))
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
        let script_pubkey = ScriptBuf::from_bytes(script_pubkey);
        let output_value_sats = (rng.next_u32() as u64 % 1_000_000) + 1;
        let prevouts = vec![TxOut {
            value: Amount::from_sat(output_value_sats.saturating_add(1_000)),
            script_pubkey: script_pubkey.clone(),
        }];
        let tx = Transaction {
            // Match the production BuildTaprootTx shape. The reference leg below
            // constructs BIP-341 SigMsg directly rather than calling SighashCache.
            version: Version::ONE,
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
                value: Amount::from_sat(output_value_sats),
                script_pubkey,
            }],
        };
        let primary_message_digests_hex = policy_bound_signing_messages_hex(&tx, &prevouts)?;
        let primary_message_digest_hex = primary_message_digests_hex
            .first()
            .ok_or_else(|| EngineError::Internal("missing differential sighash".to_string()))?;
        let reference_message_digest_hex =
            reference_bip341_key_spend_sighash_default(&tx, &prevouts, 0)?;
        if primary_message_digest_hex != &reference_message_digest_hex {
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
            "reason [{reason}] is unsupported"
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
        let recorded_reason = record.reason.trim().to_ascii_lowercase();
        if recorded_reason != reason {
            (
                false,
                format!(
                    "reason mismatch: requested [{}], recorded [{}]",
                    reason, recorded_reason
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
