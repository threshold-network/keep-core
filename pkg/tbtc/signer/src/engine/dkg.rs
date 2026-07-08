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

/// Persists a DISTRIBUTED FROST DKG result for one seat so the interactive
/// signing path can load its key. The dealer `run_dkg` above persists ALL key
/// packages (it generates them all); a distributed DKG instead runs Part1/2/3
/// across nodes and each node's Part3 returns only ITS OWN secret key package.
/// This op stores that key package (keyed by this node's participant identifier)
/// together with the group public key package, then persists - the exact session
/// shape interactive signing consumes (own key package by member_identifier; the
/// public key package for the participant set and aggregation). A MULTI-SEAT
/// operator calls it once per local seat and the key packages accumulate under
/// one session (same key group). There is NO production gate: this is the real
/// distributed path, not the transitional dealer one.
pub fn persist_distributed_dkg_key_package(
    mut request: PersistDistributedDkgKeyPackageRequest,
) -> Result<DkgResult, EngineError> {
    const OP: &str = "persist_distributed_dkg_key_package";
    validate_session_id(&request.session_id)?;
    // Gate BEFORE decoding or persisting any key material: this op writes signing
    // material to durable state that interactive signing trusts after restart, so
    // an unattested runtime must not be able to install it - the same gate run_dkg
    // and every interactive op enforce.
    enforce_provenance_gate()?;

    if request.participant_identifier == 0 {
        return Err(EngineError::Validation(format!(
            "{OP}: participant identifier must be non-zero"
        )));
    }
    if request.threshold < 2 || request.participant_count < request.threshold {
        return Err(EngineError::Validation(format!(
            "{OP}: threshold [{}] must be between 2 and participant_count [{}]",
            request.threshold, request.participant_count
        )));
    }

    let public_key_package = native_public_key_package_to_frost(OP, &request.public_key_package)?;

    // The group public key package is the authoritative participant set. EVERY
    // verifying share must have a canonical (u16-derived) identifier: a
    // non-canonical one cannot be a real group member, and silently dropping it
    // would let it slip past the admission allowlist/required checks below while
    // still inflating the participant count.
    let mut admission_participant_identifiers = HashSet::new();
    for identifier in public_key_package.verifying_shares().keys() {
        match frost_identifier_to_u16(*identifier) {
            Some(participant_identifier) => {
                admission_participant_identifiers.insert(participant_identifier);
            }
            None => {
                return Err(EngineError::Validation(format!(
                    "{OP}: public key package contains a non-canonical participant identifier"
                )))
            }
        }
    }

    // The caller's participant_count must match the authoritative public-package
    // set, or downstream consumers of the stored DkgResult get the wrong group
    // size for this key material.
    if request.participant_count as usize != admission_participant_identifiers.len() {
        return Err(EngineError::Validation(format!(
            "{OP}: participant_count [{}] does not match the public key package [{}]",
            request.participant_count,
            admission_participant_identifiers.len()
        )));
    }

    // Enforce the SAME DKG admission policy the dealer run_dkg enforces, over the
    // participant set derived from the public key package. Otherwise a caller could
    // persist a package that omits a required participant or includes a
    // non-allowlisted one, and interactive signing would later trust it.
    enforce_admission_policy_for(
        &request.session_id,
        admission_participant_identifiers.len(),
        &admission_participant_identifiers,
        request.threshold,
    )?;

    // Enforce operator quarantine over the same derived participant set, exactly
    // as the dealer run_dkg does: a distributed DKG whose group includes a
    // quarantined operator must not be persisted and then trusted by later
    // interactive signing sessions.
    let auto_quarantine_config = load_auto_quarantine_config()?;
    let quarantined_operator_identifiers = {
        let guard = state()?
            .lock()
            .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
        guard.quarantined_operator_identifiers.clone()
    };
    let participant_identifiers: Vec<u16> =
        admission_participant_identifiers.iter().copied().collect();
    enforce_not_quarantined_identifiers(
        &request.session_id,
        &participant_identifiers,
        &quarantined_operator_identifiers,
        auto_quarantine_config.as_ref(),
    )?;

    let key_package = decode_key_package(
        OP,
        &request.key_package.identifier,
        &request.key_package.data_hex,
    )?;
    // data_hex is the serialized SECRET signing share. serde owns this String
    // independently of the C-side request buffer the Go caller scrubs, so wipe it here
    // (decode_key_package already zeroized the decoded bytes); otherwise it would sit
    // in freed Rust heap until reallocation.
    request.key_package.data_hex.zeroize();

    // The key package must belong to this participant AND be consistent with the
    // group public key package: matching identifier, embedded threshold, group
    // verifying key, and this participant's verifying share. An inconsistent
    // package (e.g. min_signers 3 vs a stored threshold of 2, or a share from a
    // different DKG) would let interactive signing open an attempt it can never
    // complete and burn it at share release.
    let frost_identifier =
        participant_identifier_to_frost_identifier(request.participant_identifier)?;
    if *key_package.identifier() != frost_identifier {
        return Err(EngineError::Validation(format!(
            "{OP}: key package identifier does not match participant_identifier"
        )));
    }
    if *key_package.min_signers() != request.threshold {
        return Err(EngineError::Validation(format!(
            "{OP}: key package min_signers [{}] does not match threshold [{}]",
            *key_package.min_signers(),
            request.threshold
        )));
    }
    if key_package.verifying_key() != public_key_package.verifying_key() {
        return Err(EngineError::Validation(format!(
            "{OP}: key package group verifying key does not match the public key package"
        )));
    }
    match public_key_package.verifying_shares().get(&frost_identifier) {
        None => {
            return Err(EngineError::Validation(format!(
                "{OP}: participant_identifier is not a member of the public key package"
            )))
        }
        Some(verifying_share) if verifying_share != key_package.verifying_share() => {
            return Err(EngineError::Validation(format!(
                "{OP}: key package verifying share does not match the public key package"
            )))
        }
        Some(_) => {}
    }

    // The checks above only trust the PUBLIC verifying share embedded in the key
    // package; Round2 signs with the embedded SECRET signing share, and
    // deserialization does not prove the signing scalar derives to that public
    // share. Verify signing_share -> verifying_share, so a corrupt or malformed
    // key package cannot be stored and then burn signing attempts producing shares
    // that never verify.
    // signing_share() is Copy (frost-core SigningShare is Copy + DefaultIsZeroes, NOT
    // ZeroizeOnDrop), so bind the extracted copy and zeroize it right after the check -
    // otherwise the secret scalar lingers as un-wiped stack residue. (The copy frost's
    // own by-value VerifyingShare::from makes internally is beyond our reach.)
    let mut signing_share = *key_package.signing_share();
    let derives_to_verifying_share =
        frost::keys::VerifyingShare::from(signing_share) == *key_package.verifying_share();
    signing_share.zeroize();
    if !derives_to_verifying_share {
        return Err(EngineError::Validation(format!(
            "{OP}: key package signing share does not derive to its verifying share"
        )));
    }

    let key_group = public_key_package
        .verifying_key()
        .serialize()
        .map(hex::encode)
        .map_err(|e| {
            EngineError::Internal(format!("{OP}: failed to serialize verifying key: {e}"))
        })?;

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    ensure_session_insert_capacity(&guard.sessions, &request.session_id)?;

    let session = guard
        .sessions
        .entry(request.session_id.clone())
        .or_insert_with(SessionState::default);

    // A session may already hold a DKG result: this seat re-persisting (idempotent)
    // or, for a MULTI-SEAT operator, a sibling seat of the SAME distributed DKG.
    // Same key group -> accumulate this seat's key package into the session; a
    // different key group for the same session is a conflict.
    if let Some(existing) = &session.dkg_result {
        if existing.key_group != key_group {
            return Err(EngineError::SessionConflict {
                session_id: request.session_id,
            });
        }
        // Same group key is NOT enough: a sibling seat of the SAME distributed DKG
        // must carry the SAME threshold, participant count, and public key package.
        // Otherwise a second seat could be validated against a different submitted
        // public package while the session keeps the first, so later signing would
        // use public material inconsistent with this seat's key.
        if existing.threshold != request.threshold
            || existing.participant_count != request.participant_count
        {
            return Err(EngineError::Validation(format!(
                "{OP}: threshold/participant_count does not match the stored DKG for this session"
            )));
        }
        if session.dkg_public_key_package.as_ref() != Some(&public_key_package) {
            return Err(EngineError::Validation(format!(
                "{OP}: public key package does not match the stored DKG for this session"
            )));
        }
    } else {
        session.dkg_result = Some(DkgResult {
            session_id: request.session_id.clone(),
            key_group,
            participant_count: request.participant_count,
            threshold: request.threshold,
            created_at_unix: now_unix(),
        });
        session.dkg_public_key_package = Some(public_key_package);
    }

    session
        .dkg_key_packages
        .get_or_insert_with(BTreeMap::new)
        .insert(request.participant_identifier, key_package);

    // Clone the result before the `&guard` persist call so the mutable `session`
    // borrow ends here (mirrors run_dkg's ordering).
    let result = session
        .dkg_result
        .clone()
        .expect("dkg_result was just set for this session");
    persist_engine_state_to_storage(&guard)?;

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
