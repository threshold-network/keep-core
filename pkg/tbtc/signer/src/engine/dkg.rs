// run_dkg session flow and production gates for the transitional dealer path.

use super::*;

pub fn run_dkg(request: RunDkgRequest) -> Result<DkgResult, EngineError> {
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::RunDkg);
    validate_session_id(&request.session_id)?;
    enforce_bootstrap_dealer_dkg_disabled_in_production(&request.session_id)?;

    record_hardening_telemetry(|telemetry| {
        telemetry.run_dkg_calls_total = telemetry.run_dkg_calls_total.saturating_add(1);
    });
    enforce_provenance_gate()?;
    enforce_admission_policy(&request)?;

    if request.participants.len() < 2 {
        return Err(EngineError::Validation(
            "participants must contain at least 2 entries".to_string(),
        ));
    }

    if request.threshold < 2 || usize::from(request.threshold) > request.participants.len() {
        return Err(EngineError::Validation(
            "threshold must be between 2 and number of participants".to_string(),
        ));
    }

    let mut unique_identifiers = HashSet::new();
    for participant in &request.participants {
        if participant.identifier == 0 {
            return Err(EngineError::Validation(
                "participant identifier must be non-zero".to_string(),
            ));
        }

        if !unique_identifiers.insert(participant.identifier) {
            return Err(EngineError::Validation(
                "participant identifiers must be unique".to_string(),
            ));
        }
    }

    let request_fingerprint = fingerprint(&canonicalize_dkg_request_for_fingerprint(&request))?;

    {
        let guard = state()?
            .lock()
            .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
        if let Some(session) = guard.sessions.get(&request.session_id) {
            if let Some(existing) = &session.dkg_request_fingerprint {
                if existing == &request_fingerprint {
                    return session.dkg_result.clone().ok_or_else(|| {
                        EngineError::Internal("missing DKG result cache".to_string())
                    });
                }

                return Err(EngineError::SessionConflict {
                    session_id: request.session_id,
                });
            }
        } else {
            ensure_session_insert_capacity(&guard.sessions, &request.session_id)?;
        }
    }

    let mut participant_identifiers: Vec<u16> = request
        .participants
        .iter()
        .map(|participant| participant.identifier)
        .collect();
    participant_identifiers.sort_unstable();

    let auto_quarantine_config = load_auto_quarantine_config()?;
    let quarantined_operator_identifiers = {
        let guard = state()?
            .lock()
            .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
        guard.quarantined_operator_identifiers.clone()
    };
    enforce_not_quarantined_identifiers(
        &request.session_id,
        &participant_identifiers,
        &quarantined_operator_identifiers,
        auto_quarantine_config.as_ref(),
    )?;

    let frost_identifiers: Vec<frost::Identifier> = participant_identifiers
        .iter()
        .map(|identifier| participant_identifier_to_frost_identifier(*identifier))
        .collect::<Result<Vec<_>, _>>()?;

    let mut keygen_rng_seed = development_dealer_dkg_seed(request.dkg_seed_hex.as_deref())?;
    let keygen_rng = ZeroizingChaCha20Rng::from_seed(keygen_rng_seed);
    keygen_rng_seed.zeroize();

    let (secret_shares, public_key_package) = frost::keys::generate_with_dealer(
        request.participants.len() as u16,
        request.threshold,
        frost::keys::IdentifierList::Custom(&frost_identifiers),
        keygen_rng,
    )
    .map_err(|e| EngineError::Internal(format!("failed to generate key shares: {e}")))?;

    let mut participant_identifier_by_frost_identifier = HashMap::new();
    for (participant_identifier, frost_identifier) in
        participant_identifiers.iter().zip(frost_identifiers.iter())
    {
        participant_identifier_by_frost_identifier.insert(
            hex::encode(frost_identifier.serialize()),
            *participant_identifier,
        );
    }

    let mut key_packages = BTreeMap::new();
    for (frost_identifier, secret_share) in secret_shares {
        let participant_identifier = participant_identifier_by_frost_identifier
            .get(&hex::encode(frost_identifier.serialize()))
            .copied()
            .ok_or_else(|| {
                EngineError::Internal(
                    "missing participant identifier mapping for generated key share".to_string(),
                )
            })?;

        let key_package = frost::keys::KeyPackage::try_from(secret_share)
            .map_err(|e| EngineError::Internal(format!("failed to convert secret share: {e}")))?;

        key_packages.insert(participant_identifier, key_package);
    }

    if key_packages.len() != request.participants.len() {
        return Err(EngineError::Internal(
            "generated key package count mismatch".to_string(),
        ));
    }

    // The `frost-secp256k1-tr` ciphersuite post-processes DKG output before
    // returning these packages. This serialized verifying key is the protocol
    // wallet key exported to Go/on-chain; later Taproot tweaks are applied
    // relative to this exported key.
    let key_group = public_key_package
        .verifying_key()
        .serialize()
        .map(hex::encode)
        .map_err(|e| EngineError::Internal(format!("failed to serialize verifying key: {e}")))?;

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    ensure_session_insert_capacity(&guard.sessions, &request.session_id)?;

    let session = guard
        .sessions
        .entry(request.session_id.clone())
        .or_insert_with(SessionState::default);

    if let Some(existing) = &session.dkg_request_fingerprint {
        if existing == &request_fingerprint {
            return session
                .dkg_result
                .clone()
                .ok_or_else(|| EngineError::Internal("missing DKG result cache".to_string()));
        }

        return Err(EngineError::SessionConflict {
            session_id: request.session_id,
        });
    }

    let result = DkgResult {
        session_id: request.session_id,
        key_group,
        participant_count: request.participants.len() as u16,
        threshold: request.threshold,
        created_at_unix: now_unix(),
    };

    session.dkg_request_fingerprint = Some(request_fingerprint);
    session.dkg_key_packages = Some(key_packages);
    session.dkg_public_key_package = Some(public_key_package);
    session.dkg_result = Some(result.clone());
    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.run_dkg_success_total = telemetry.run_dkg_success_total.saturating_add(1);
    });

    Ok(result)
}

pub(crate) fn enforce_bootstrap_dealer_dkg_disabled_in_production(
    session_id: &str,
) -> Result<(), EngineError> {
    if signer_profile_is_production() {
        return Err(EngineError::LifecyclePolicyRejected {
            session_id: session_id.to_string(),
            reason_code: "bootstrap_dealer_dkg_disabled_in_production".to_string(),
            detail: format!(
                "bootstrap dealer DKG is disabled when {TBTC_SIGNER_PROFILE_ENV}={TBTC_SIGNER_PROFILE_PRODUCTION}; production requires distributed DKG wiring"
            ),
        });
    }

    Ok(())
}

/// The transitional StartSignRound/FinalizeSignRound flow derives round-1
/// nonces deterministically (see `RoundNonceBinding`) and only operates on
/// dealer-DKG sessions where one engine holds every participant's key
/// package. Blocking dealer DKG in production (above) is not enough on its
/// own: persisted state created under a development profile could be carried
/// into a production-profile process and signed with there. Gate the signing
/// entry points themselves so a production signer can never execute the
/// deterministic-nonce path, regardless of how its on-disk state was created.
/// Production signing must use the interactive FROST path, which draws
/// nonces from OS randomness.
pub(crate) fn enforce_transitional_signing_disabled_in_production(
    session_id: &str,
) -> Result<(), EngineError> {
    if signer_profile_is_production() {
        return Err(EngineError::LifecyclePolicyRejected {
            session_id: session_id.to_string(),
            reason_code: "transitional_deterministic_signing_disabled_in_production".to_string(),
            detail: format!(
                "transitional deterministic-nonce signing (StartSignRound/FinalizeSignRound) is disabled when {TBTC_SIGNER_PROFILE_ENV}={TBTC_SIGNER_PROFILE_PRODUCTION}; production signing must use the interactive FROST path with OS-random nonces"
            ),
        });
    }

    Ok(())
}

pub(crate) fn development_dealer_dkg_seed(
    dkg_seed_hex: Option<&str>,
) -> Result<[u8; 32], EngineError> {
    let Some(seed_hex) = dkg_seed_hex else {
        let mut seed = [0_u8; 32];
        OsRng.fill_bytes(&mut seed);
        return Ok(seed);
    };

    let seed =
        Zeroizing::new(hex::decode(seed_hex).map_err(|e| {
            EngineError::Validation(format!("dkg_seed_hex must be valid hex: {e}"))
        })?);
    if seed.len() != 32 {
        return Err(EngineError::Validation(format!(
            "dkg_seed_hex decoded to [{}] bytes, expected 32",
            seed.len()
        )));
    }

    let mut output = [0u8; 32];
    output.copy_from_slice(&seed);

    Ok(output)
}
