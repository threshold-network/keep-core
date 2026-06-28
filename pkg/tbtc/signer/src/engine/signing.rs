// start/finalize sign-round session flows and bootstrap synthetic contributions.

use super::*;

pub(crate) const BOOTSTRAP_SYNTHETIC_CONTRIBUTION_DOMAIN: &str =
    "tbtc-signer-bootstrap-contribution-v1";

// The sign-round persist-pending markers live in the persistence module
// (`mark_sign_round_persist_pending` / `sign_round_persist_pending`), keyed PER
// SESSION. ANY successful persist clears them all, not only `start_sign_round`'s
// own -- otherwise a later unrelated persist that makes the round durable would
// leave the marker stale and force an idempotent replay to re-persist during a
// state-key outage. They are keyed per session so one session's failed persist
// cannot force an unrelated, already-durable session's replay to re-persist (and
// fail) during the same outage.

pub fn start_sign_round(mut request: StartSignRoundRequest) -> Result<RoundState, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.start_sign_round_calls_total =
            telemetry.start_sign_round_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::StartSignRound);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;
    enforce_transitional_signing_disabled_in_production(&request.session_id)?;

    if request.member_identifier == 0 {
        return Err(EngineError::Validation(
            "member_identifier must be non-zero".to_string(),
        ));
    }

    let message_bytes = hex::decode(&request.message_hex)
        .map_err(|_| EngineError::Validation("message_hex must be valid hex".to_string()))?;
    let message_digest_hex = hash_hex(&message_bytes);
    let taproot_merkle_root =
        canonicalize_taproot_merkle_root_hex(&mut request.taproot_merkle_root_hex)?;
    let strict_roast_mode_enabled = roast_strict_mode_enabled();

    let request_fingerprint = start_sign_round_request_fingerprint(&request, 0)?;
    // Before multi-seat round reuse, persisted active rounds were bound to the
    // concrete member identifier. Accept that legacy fingerprint so an upgrade
    // does not invalidate an in-flight signing round.
    let legacy_member_request_fingerprint =
        start_sign_round_request_fingerprint(&request, request.member_identifier)?;
    // The previous round-reuse implementation included one-shot transition
    // evidence in the persisted active-round fingerprint. Accept that shape
    // when callers still resend the evidence, then migrate to the stable form.
    let legacy_canonical_with_transition_evidence_fingerprint =
        start_sign_round_request_fingerprint_including_transition_evidence(&request, 0)?;
    let legacy_member_with_transition_evidence_fingerprint =
        start_sign_round_request_fingerprint_including_transition_evidence(
            &request,
            request.member_identifier,
        )?;
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let auto_quarantine_config = load_auto_quarantine_config()?;
    let quarantined_operator_identifiers = guard.quarantined_operator_identifiers.clone();

    let mut pending_transition_record = None;
    let round_state = {
        let session = guard.sessions.get_mut(&request.session_id).ok_or_else(|| {
            EngineError::SessionNotFound {
                session_id: request.session_id.clone(),
            }
        })?;

        let dkg = session
            .dkg_result
            .clone()
            .ok_or_else(|| EngineError::DkgNotReady {
                session_id: request.session_id.clone(),
            })?;

        if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
            return Err(EngineError::LifecyclePolicyRejected {
                session_id: request.session_id.clone(),
                reason_code: "emergency_rekey_required".to_string(),
                detail: format!(
                    "emergency rekey required for session [{}] since [{}]: {}",
                    request.session_id,
                    emergency_rekey_event.triggered_at_unix,
                    emergency_rekey_event.reason
                ),
            });
        }

        if session.finalize_request_fingerprint.is_some() {
            // Lifecycle terminal state: once finalize succeeds for a session, we
            // intentionally return SessionFinalized and require a new session_id
            // for any subsequent StartSignRound call on that session ID.
            return Err(EngineError::SessionFinalized {
                session_id: request.session_id,
            });
        }

        if request.key_group != dkg.key_group {
            return Err(EngineError::Validation(
                "key_group does not match DKG output for this session".to_string(),
            ));
        }

        {
            let dkg_key_packages = session.dkg_key_packages.as_ref().ok_or_else(|| {
                EngineError::Internal("missing DKG key package cache".to_string())
            })?;

            if !dkg_key_packages.contains_key(&request.member_identifier) {
                return Err(EngineError::Validation(
                    "member_identifier is not a DKG participant for this session".to_string(),
                ));
            }
        }
        enforce_signing_message_binding_to_policy_checked_build_tx(
            &request.session_id,
            &request.message_hex,
            session.tx_result.as_ref(),
        )?;

        // Guard against partial legacy state where sign material was cleared but
        // active attempt context was not.
        if session.sign_request_fingerprint.is_none() || session.round_state.is_none() {
            session.active_attempt_context = None;
        }

        let canonical_attempt_context = request
            .attempt_context
            .as_ref()
            .map(canonical_attempt_context);
        let mut attempt_transition_telemetry = None;
        let mut attempt_transition_record = None;
        // Set when an attempt advance is authorized below. The actual clear of
        // the prior round is deferred until the replacement round has passed
        // every fallible check, so a failed advance cannot strand the session.
        let mut attempt_transition_authorized = false;
        if let Some(active_attempt_context) = session.active_attempt_context.as_ref() {
            let active_attempt_match_outcome = enforce_active_attempt_context_match(
                active_attempt_context,
                canonical_attempt_context.as_ref(),
                request.attempt_transition_evidence.as_ref(),
                session.round_state.as_ref(),
                session.sign_request_fingerprint.as_deref(),
                strict_roast_mode_enabled,
            )?;

            if let ActiveAttemptMatchOutcome::AdvanceAuthorized = active_attempt_match_outcome {
                let incoming_attempt_context =
                    canonical_attempt_context.as_ref().ok_or_else(|| {
                        EngineError::Internal(
                            "missing incoming attempt context for authorized transition"
                                .to_string(),
                        )
                    })?;
                let transition_evidence =
                    request
                        .attempt_transition_evidence
                        .as_ref()
                        .ok_or_else(|| {
                            EngineError::Internal(
                                "missing attempt_transition_evidence for authorized transition"
                                    .to_string(),
                            )
                        })?;
                attempt_transition_telemetry = build_attempt_transition_telemetry(
                    active_attempt_context,
                    incoming_attempt_context,
                    Some(transition_evidence),
                );
                if attempt_transition_telemetry.is_none() {
                    return Err(EngineError::Internal(
                        "missing transition telemetry evidence for authorized transition"
                            .to_string(),
                    ));
                }
                attempt_transition_record = Some(build_transcript_audit_record(
                    active_attempt_context,
                    incoming_attempt_context,
                    transition_evidence,
                )?);
                // Validate the incoming attempt context against the
                // deterministic RFC-21 coordinator selection BEFORE the active
                // round is touched. A malformed advance (e.g. a forged
                // coordinator_identifier that satisfies the transition evidence
                // but fails deterministic validation) must be rejected here.
                validate_attempt_context(
                    &request.session_id,
                    &dkg.key_group,
                    &message_bytes,
                    &message_digest_hex,
                    dkg.threshold,
                    request.attempt_context.as_ref(),
                    strict_roast_mode_enabled,
                )?;
                // Authorize the advance but DEFER clearing the active round.
                // The replacement round must still pass several fallible
                // fresh-path checks below (participant resolution, included-set
                // equality, quarantine, consumed-replay, share construction).
                // Clearing here would let any of those failures destroy the
                // in-memory active round with no validated, persisted
                // replacement -- stranding the session (round material gone,
                // transition record unwritten) so the next StartSignRound could
                // start a fresh attempt without transition evidence until a
                // restart reloads durable state. The clear runs just before the
                // replacement round is installed and persisted.
                attempt_transition_authorized = true;
            }
        }

        if attempt_transition_authorized {
            // An authorized attempt advance is in progress: the prior round
            // material is still in memory, but a new attempt is starting. Skip
            // the idempotent/conflict match against the old fingerprint and fall
            // through to establish (and persist) the replacement round below.
        } else if let Some(existing) = &session.sign_request_fingerprint {
            let matches_canonical_fingerprint = existing == &request_fingerprint;
            let matches_legacy_fingerprint = !matches_canonical_fingerprint
                && (existing == &legacy_member_request_fingerprint
                    || existing == &legacy_canonical_with_transition_evidence_fingerprint
                    || existing == &legacy_member_with_transition_evidence_fingerprint);

            if matches_canonical_fingerprint || matches_legacy_fingerprint {
                let mut round_state = session.round_state.clone().ok_or_else(|| {
                    EngineError::Internal("missing round state cache".to_string())
                })?;
                let sign_message_bytes = session.sign_message_bytes.as_ref().ok_or_else(|| {
                    EngineError::Internal("missing sign message cache".to_string())
                })?;
                let signing_participants =
                    round_state.signing_participants.clone().ok_or_else(|| {
                        EngineError::Internal(
                            "missing round signing participants cache".to_string(),
                        )
                    })?;
                let dkg_key_packages = session.dkg_key_packages.as_ref().ok_or_else(|| {
                    EngineError::Internal("missing DKG key package cache".to_string())
                })?;
                let dkg_public_key_package =
                    session.dkg_public_key_package.as_ref().ok_or_else(|| {
                        EngineError::Internal("missing DKG public key package cache".to_string())
                    })?;

                round_state.own_contribution = build_real_signature_share_contribution(
                    dkg_key_packages,
                    dkg_public_key_package,
                    &signing_participants,
                    &request,
                    &round_state.round_id,
                    sign_message_bytes,
                    taproot_merkle_root.as_ref(),
                )?;

                if matches_legacy_fingerprint {
                    session.sign_request_fingerprint = Some(request_fingerprint.clone());
                }

                // Persist the cached round before serving it when either (a) we
                // just upgraded a legacy fingerprint, or (b) the round was
                // established in memory but its original persist has not yet
                // succeeded -- e.g. a prior StartSignRound mutated the
                // consumed-replay markers and round state, then failed to persist
                // (state-key-provider or disk error) and returned an error,
                // serving no shares. Serving shares here without persisting in
                // that case would let a restart replay the round with no durable
                // consumed marker. When the original persist already succeeded
                // (the common case) serve the cached round WITHOUT persisting, so
                // the idempotent replay still survives a state-key-provider
                // outage, as build_taproot_tx does.
                if matches_legacy_fingerprint || sign_round_persist_pending(&request.session_id) {
                    // persist_engine_state_to_storage clears the pending markers on
                    // success (see the persistence module). Only THIS session's own
                    // not-yet-durable round forces a re-persist here; an unrelated
                    // session's pending persist does not.
                    persist_engine_state_to_storage(&guard)?;
                }

                return Ok(round_state);
            }

            return Err(EngineError::SessionConflict {
                session_id: request.session_id,
            });
        }

        let signing_participants = {
            let dkg_key_packages = session.dkg_key_packages.as_ref().ok_or_else(|| {
                EngineError::Internal("missing DKG key package cache".to_string())
            })?;
            resolve_signing_participants(&request, dkg.threshold, dkg_key_packages)?
        };
        if let Some(canonical_attempt_signing_participants) = validate_attempt_context(
            &request.session_id,
            &dkg.key_group,
            &message_bytes,
            &message_digest_hex,
            dkg.threshold,
            request.attempt_context.as_ref(),
            strict_roast_mode_enabled,
        )? {
            if canonical_attempt_signing_participants != signing_participants {
                return Err(EngineError::Validation(
                    "attempt_context.included_participants must match resolved signing_participants"
                        .to_string(),
                ));
            }
        }
        enforce_not_quarantined_identifiers(
            &request.session_id,
            &signing_participants,
            &quarantined_operator_identifiers,
            auto_quarantine_config.as_ref(),
        )?;

        let signing_participants_fingerprint = fingerprint(&signing_participants)?;
        let consumed_attempt_id = canonical_attempt_context
            .as_ref()
            .map(|attempt_context| attempt_context.attempt_id.clone());
        if let Some(attempt_id) = consumed_attempt_id.as_ref() {
            if session.consumed_attempt_ids.contains(attempt_id) {
                return Err(EngineError::ConsumedAttemptReplay {
                    session_id: request.session_id.clone(),
                    attempt_id: attempt_id.clone(),
                });
            }
            ensure_consumed_registry_insert_capacity(
                &session.consumed_attempt_ids,
                attempt_id,
                "consumed_attempt_ids",
                &request.session_id,
            )?;
        }
        let round_id = derive_round_id(
            &request.session_id,
            &request.key_group,
            &request.message_hex,
            request.taproot_merkle_root_hex.as_deref(),
            &signing_participants_fingerprint,
            canonical_attempt_context.as_ref(),
        );
        if session.consumed_sign_round_ids.contains(&round_id) {
            return Err(EngineError::ConsumedRoundReplay {
                session_id: request.session_id.clone(),
                round_id: round_id.clone(),
            });
        }
        ensure_consumed_registry_insert_capacity(
            &session.consumed_sign_round_ids,
            &round_id,
            "consumed_sign_round_ids",
            &request.session_id,
        )?;
        let own_contribution = {
            let dkg_key_packages = session.dkg_key_packages.as_ref().ok_or_else(|| {
                EngineError::Internal("missing DKG key package cache".to_string())
            })?;
            let dkg_public_key_package =
                session.dkg_public_key_package.as_ref().ok_or_else(|| {
                    EngineError::Internal("missing DKG public key package cache".to_string())
                })?;
            build_real_signature_share_contribution(
                dkg_key_packages,
                dkg_public_key_package,
                &signing_participants,
                &request,
                &round_id,
                &message_bytes,
                taproot_merkle_root.as_ref(),
            )?
        };

        if let Some(transition_telemetry) = attempt_transition_telemetry.as_ref() {
            record_hardening_telemetry(|telemetry| {
                telemetry.attempt_transition_total =
                    telemetry.attempt_transition_total.saturating_add(1);
                if transition_telemetry.coordinator_rotated {
                    telemetry.coordinator_failover_total =
                        telemetry.coordinator_failover_total.saturating_add(1);
                }
            });
        }
        if let Some(transition_record) = attempt_transition_record.as_ref() {
            ensure_attempt_transition_record_insert_capacity(
                &session.attempt_transition_records,
                &request.session_id,
            )?;
            session
                .attempt_transition_records
                .push(transition_record.clone());
            pending_transition_record = Some(transition_record.clone());
        }

        let round_state = RoundState {
            session_id: request.session_id.clone(),
            round_id: round_id.clone(),
            required_contributions: dkg.threshold,
            message_digest_hex: message_digest_hex.clone(),
            taproot_merkle_root_hex: request.taproot_merkle_root_hex.clone(),
            signing_participants: Some(signing_participants),
            attempt_transition_telemetry,
            own_contribution,
        };

        // Every fallible fresh-round check has now passed and the replacement
        // round is fully built. Scrub the prior round material as part of the
        // attempt transition -- deferred to here (not the AdvanceAuthorized
        // decision) so a malformed advance that failed a later check could not
        // have destroyed the active round without a validated replacement.
        if attempt_transition_authorized {
            clear_active_sign_round_for_attempt_transition(session);
        }
        session.sign_request_fingerprint = Some(request_fingerprint);
        session.sign_message_bytes = Some(Zeroizing::new(message_bytes));
        session.round_state = Some(round_state.clone());
        session.active_attempt_context = canonical_attempt_context;
        if let Some(attempt_id) = consumed_attempt_id {
            session.consumed_attempt_ids.insert(attempt_id);
        }
        session.consumed_sign_round_ids.insert(round_id);
        // The round is now established in memory but not yet durable. Mark this
        // session's persist as pending so a later idempotent cached serve
        // re-persists if the persist below fails; cleared by the next successful
        // persist (of any kind). Keyed per session so an unrelated, already-durable
        // session's replay is not forced to re-persist during an outage.
        mark_sign_round_persist_pending(&request.session_id);

        round_state
    };

    if let Some(transition_record) = pending_transition_record.as_ref() {
        apply_auto_quarantine_faults_for_transition(
            &mut guard,
            &request.session_id,
            transition_record,
            auto_quarantine_config.as_ref(),
        );
    }

    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.start_sign_round_success_total =
            telemetry.start_sign_round_success_total.saturating_add(1);
    });

    Ok(round_state)
}

pub(crate) fn resolve_signing_participants(
    request: &StartSignRoundRequest,
    threshold: u16,
    dkg_key_packages: &BTreeMap<u16, frost::keys::KeyPackage>,
) -> Result<Vec<u16>, EngineError> {
    let mut signing_participants = request
        .signing_participants
        .clone()
        .unwrap_or_else(|| dkg_key_packages.keys().copied().collect());
    if signing_participants.is_empty() {
        return Err(EngineError::Validation(
            "signing_participants must not be empty".to_string(),
        ));
    }

    signing_participants.sort_unstable();
    let mut unique_signing_participants = HashSet::new();

    for signing_participant in &signing_participants {
        if *signing_participant == 0 {
            return Err(EngineError::Validation(
                "signing_participants must contain non-zero identifiers".to_string(),
            ));
        }

        if !unique_signing_participants.insert(*signing_participant) {
            return Err(EngineError::Validation(format!(
                "signing_participants contains duplicate identifier [{}]",
                signing_participant
            )));
        }

        if !dkg_key_packages.contains_key(signing_participant) {
            return Err(EngineError::Validation(format!(
                "signing_participant [{}] is not a DKG participant for this session",
                signing_participant
            )));
        }
    }

    if signing_participants.len() < usize::from(threshold) {
        return Err(EngineError::Validation(format!(
            "signing_participants must contain at least threshold members [{}]",
            threshold
        )));
    }

    if !unique_signing_participants.contains(&request.member_identifier) {
        return Err(EngineError::Validation(
            "member_identifier must be included in signing_participants".to_string(),
        ));
    }

    Ok(signing_participants)
}

pub(crate) fn build_real_signature_share_contribution(
    dkg_key_packages: &BTreeMap<u16, frost::keys::KeyPackage>,
    dkg_public_key_package: &frost::keys::PublicKeyPackage,
    signing_participants: &[u16],
    request: &StartSignRoundRequest,
    round_id: &str,
    message_bytes: &[u8],
    taproot_merkle_root: Option<&[u8; 32]>,
) -> Result<RoundContribution, EngineError> {
    let public_key_package_bytes = dkg_public_key_package.serialize().map_err(|e| {
        EngineError::Internal(format!("failed to serialize public key package: {e}"))
    })?;
    let mut commitments = BTreeMap::new();
    let mut own_nonces = None;

    for participant_identifier in signing_participants {
        let key_package = dkg_key_packages
            .get(participant_identifier)
            .ok_or_else(|| {
                EngineError::Internal(format!(
                    "missing DKG key package for signing participant [{}]",
                    participant_identifier
                ))
            })?;
        let frost_identifier = participant_identifier_to_frost_identifier(*participant_identifier)?;
        let (mut nonces, participant_commitments) = build_deterministic_round_nonce_and_commitment(
            key_package,
            &RoundNonceBinding {
                session_id: &request.session_id,
                round_id,
                public_key_package_bytes: &public_key_package_bytes,
                message_bytes,
                taproot_merkle_root,
                signing_participants,
                participant_identifier: *participant_identifier,
            },
        );
        commitments.insert(frost_identifier, participant_commitments);

        if *participant_identifier == request.member_identifier {
            // `SigningNonces` derives `ZeroizeOnDrop`; if a later `?` returns
            // early in this function, this cached own nonce is still wiped
            // when `own_nonces` drops during unwind of the error path.
            own_nonces = Some(nonces);
        } else {
            nonces.zeroize();
        }
    }

    let mut own_nonces = own_nonces.ok_or_else(|| {
        EngineError::Validation(
            "member_identifier is missing from generated participant nonces".to_string(),
        )
    })?;

    let own_key_package = dkg_key_packages
        .get(&request.member_identifier)
        .ok_or_else(|| {
            EngineError::Validation(
                "member_identifier key package is missing from DKG cache".to_string(),
            )
        })?;

    let signing_package = frost::SigningPackage::new(commitments, message_bytes);
    let signature_share_result = if let Some(taproot_merkle_root) = taproot_merkle_root {
        frost::round2::sign_with_tweak(
            &signing_package,
            &own_nonces,
            own_key_package,
            Some(taproot_merkle_root.as_slice()),
        )
    } else {
        frost::round2::sign(&signing_package, &own_nonces, own_key_package)
    };
    own_nonces.zeroize();
    let signature_share = signature_share_result
        .map_err(|e| EngineError::Internal(format!("failed to create signature share: {e}")))?;

    let mut signature_share_bytes = signature_share.serialize();
    let signature_share_hex = hex::encode(&signature_share_bytes);
    signature_share_bytes.zeroize();

    Ok(RoundContribution {
        identifier: request.member_identifier,
        signature_share_hex,
    })
}

pub fn finalize_sign_round(
    mut request: FinalizeSignRoundRequest,
    bootstrap_mode_enabled: bool,
) -> Result<SignatureResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.finalize_sign_round_calls_total =
            telemetry.finalize_sign_round_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::FinalizeSignRound);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;
    enforce_transitional_signing_disabled_in_production(&request.session_id)?;
    let strict_roast_mode_enabled = roast_strict_mode_enabled();
    let finalize_taproot_merkle_root =
        canonicalize_taproot_merkle_root_hex(&mut request.taproot_merkle_root_hex)?;

    let request_fingerprint = {
        let mut canonical_attempt_context = request.attempt_context.clone();
        canonicalize_attempt_context_for_fingerprint(&mut canonical_attempt_context);

        let mut canonical_contributions = request.round_contributions.clone();
        canonical_contributions.sort_unstable_by(|left, right| {
            left.identifier
                .cmp(&right.identifier)
                .then_with(|| left.signature_share_hex.cmp(&right.signature_share_hex))
        });

        fingerprint(&FinalizeSignRoundRequest {
            session_id: request.session_id.clone(),
            taproot_merkle_root_hex: request.taproot_merkle_root_hex.clone(),
            round_contributions: canonical_contributions,
            attempt_context: canonical_attempt_context,
        })?
    };
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;

    let session = guard.sessions.get_mut(&request.session_id).ok_or_else(|| {
        EngineError::SessionNotFound {
            session_id: request.session_id.clone(),
        }
    })?;
    if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
        return Err(EngineError::LifecyclePolicyRejected {
            session_id: request.session_id.clone(),
            reason_code: "emergency_rekey_required".to_string(),
            detail: format!(
                "finalize blocked: emergency rekey required since [{}]: {}",
                emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
            ),
        });
    }

    if session.round_state.is_none() {
        session.active_attempt_context = None;
    }

    let canonical_attempt_context = request
        .attempt_context
        .as_ref()
        .map(canonical_attempt_context);
    if let Some(active_attempt_context) = session.active_attempt_context.as_ref() {
        enforce_active_attempt_context_match(
            active_attempt_context,
            canonical_attempt_context.as_ref(),
            None,
            session.round_state.as_ref(),
            session.sign_request_fingerprint.as_deref(),
            strict_roast_mode_enabled,
        )?;
    }

    if let Some(existing) = &session.finalize_request_fingerprint {
        if existing == &request_fingerprint {
            return session.signature_result.clone().ok_or_else(|| {
                EngineError::Internal("missing finalize signature cache".to_string())
            });
        }

        return Err(EngineError::SessionConflict {
            session_id: request.session_id,
        });
    }
    if session
        .consumed_finalize_request_fingerprints
        .contains(&request_fingerprint)
    {
        return Err(EngineError::Validation(format!(
            "finalize request fingerprint [{}] already consumed in session [{}]",
            request_fingerprint, request.session_id
        )));
    }

    let round_state =
        session
            .round_state
            .clone()
            .ok_or_else(|| EngineError::SignRoundNotStarted {
                session_id: request.session_id.clone(),
            })?;
    if request.taproot_merkle_root_hex != round_state.taproot_merkle_root_hex {
        return Err(EngineError::Validation(
            "taproot_merkle_root_hex does not match active signing round".to_string(),
        ));
    }
    if signing_policy_firewall_enforced() {
        let sign_message_hex = session
            .sign_message_bytes
            .as_ref()
            .map(|bytes| hex::encode(bytes.as_slice()))
            .ok_or_else(|| EngineError::Internal("missing sign message cache".to_string()))?;
        enforce_signing_message_binding_to_policy_checked_build_tx(
            &request.session_id,
            &sign_message_hex,
            session.tx_result.as_ref(),
        )?;
    }
    // This consumed-round check depends on `round_state` being present to
    // recover `round_id`. If prior finalize already purged round_state,
    // SignRoundNotStarted fails closed before this branch.
    if session
        .consumed_finalize_round_ids
        .contains(&round_state.round_id)
    {
        return Err(EngineError::Validation(format!(
            "round_id [{}] already consumed for finalize in session [{}]",
            round_state.round_id, request.session_id
        )));
    }

    if request.round_contributions.is_empty() {
        return Err(EngineError::Validation(
            "round_contributions must not be empty".to_string(),
        ));
    }

    if request.round_contributions.len() < usize::from(round_state.required_contributions) {
        return Err(EngineError::Validation(format!(
            "insufficient round contributions: expected at least {}",
            round_state.required_contributions
        )));
    }

    let finalize_key_group = session
        .dkg_result
        .as_ref()
        .map(|dkg| dkg.key_group.clone())
        .ok_or_else(|| EngineError::Internal("missing DKG result cache".to_string()))?;
    // The raw signing message cached at StartSignRound feeds the RFC-21
    // shuffle-seed digest; `round_state.message_digest_hex` (the SHA256
    // transcript digest) keeps feeding the attempt_id check. Both were
    // stored by the same StartSignRound call.
    let finalize_message_bytes = session
        .sign_message_bytes
        .as_ref()
        .map(|message_bytes| message_bytes.to_vec())
        .ok_or_else(|| EngineError::Internal("missing sign message cache".to_string()))?;
    if let Some(canonical_attempt_signing_participants) = validate_attempt_context(
        &request.session_id,
        &finalize_key_group,
        &finalize_message_bytes,
        &round_state.message_digest_hex,
        round_state.required_contributions,
        request.attempt_context.as_ref(),
        strict_roast_mode_enabled,
    )? {
        let mut canonical_round_signing_participants =
            round_state.signing_participants.clone().ok_or_else(|| {
                EngineError::Internal(
                    "missing round signing participants for attempt context validation".to_string(),
                )
            })?;
        canonical_round_signing_participants.sort_unstable();
        canonical_round_signing_participants.dedup();
        if canonical_attempt_signing_participants != canonical_round_signing_participants {
            return Err(EngineError::Validation(
                "attempt_context.included_participants must match round signing participants"
                    .to_string(),
            ));
        }
    }

    let mut ordered_contributions = request.round_contributions;
    ordered_contributions.sort_by_key(|contribution| contribution.identifier);
    let is_synthetic = uses_bootstrap_synthetic_contributions(&round_state, &ordered_contributions);

    if !bootstrap_mode_enabled && is_synthetic {
        return Err(EngineError::SyntheticContributionRejected {
            session_id: request.session_id,
        });
    }
    if is_synthetic && round_state.taproot_merkle_root_hex.is_some() {
        return Err(EngineError::Validation(
            "synthetic contributions do not support taproot tweaked signing".to_string(),
        ));
    }

    let signature_result = if is_synthetic {
        build_bootstrap_synthetic_signature_result(
            &request.session_id,
            &round_state,
            &ordered_contributions,
        )?
    } else {
        let dkg_key_packages = session
            .dkg_key_packages
            .as_ref()
            .ok_or_else(|| EngineError::Internal("missing DKG key package cache".to_string()))?;

        let dkg_public_key_package = session.dkg_public_key_package.as_ref().ok_or_else(|| {
            EngineError::Internal("missing DKG public key package cache".to_string())
        })?;

        let sign_message_bytes = session
            .sign_message_bytes
            .as_ref()
            .ok_or_else(|| EngineError::Internal("missing sign message cache".to_string()))?;

        let signing_participants = round_state
            .signing_participants
            .clone()
            .unwrap_or_else(|| dkg_key_packages.keys().copied().collect());

        let mut signing_participant_set = HashSet::new();
        for signing_participant in &signing_participants {
            if !signing_participant_set.insert(*signing_participant) {
                return Err(EngineError::Internal(format!(
                    "duplicate signing participant identifier [{}] in round state",
                    signing_participant
                )));
            }
        }

        let public_key_package_bytes = dkg_public_key_package.serialize().map_err(|e| {
            EngineError::Internal(format!("failed to serialize public key package: {e}"))
        })?;
        let mut commitments = BTreeMap::new();
        for signing_participant in &signing_participants {
            let key_package = dkg_key_packages.get(signing_participant).ok_or_else(|| {
                EngineError::Internal(format!(
                    "missing DKG key package for signing participant [{}]",
                    signing_participant
                ))
            })?;
            let frost_identifier =
                participant_identifier_to_frost_identifier(*signing_participant)?;
            let (mut participant_nonces, participant_commitments) =
                build_deterministic_round_nonce_and_commitment(
                    key_package,
                    &RoundNonceBinding {
                        session_id: &round_state.session_id,
                        round_id: &round_state.round_id,
                        public_key_package_bytes: &public_key_package_bytes,
                        message_bytes: sign_message_bytes,
                        taproot_merkle_root: finalize_taproot_merkle_root.as_ref(),
                        signing_participants: &signing_participants,
                        participant_identifier: *signing_participant,
                    },
                );
            participant_nonces.zeroize();
            commitments.insert(frost_identifier, participant_commitments);
        }

        let mut contributing_identifiers = Vec::with_capacity(ordered_contributions.len());
        let mut signature_shares = BTreeMap::new();
        for contribution in &ordered_contributions {
            if !signing_participant_set.contains(&contribution.identifier) {
                return Err(EngineError::Validation(format!(
                    "round contribution identifier [{}] is not in signing participant set",
                    contribution.identifier
                )));
            }

            let frost_identifier =
                participant_identifier_to_frost_identifier(contribution.identifier)?;

            if signature_shares.contains_key(&frost_identifier) {
                return Err(EngineError::Validation(format!(
                    "duplicate round contribution identifier [{}]",
                    contribution.identifier
                )));
            }

            let mut signature_share_bytes = hex::decode(&contribution.signature_share_hex)
                .map_err(|_| {
                    EngineError::Validation(format!(
                        "invalid signature_share_hex for identifier [{}]",
                        contribution.identifier
                    ))
                })?;
            let signature_share_result =
                frost::round2::SignatureShare::deserialize(&signature_share_bytes);
            signature_share_bytes.zeroize();
            let signature_share = signature_share_result.map_err(|e| {
                EngineError::Validation(format!(
                    "invalid signature share for identifier [{}]: {e}",
                    contribution.identifier
                ))
            })?;

            contributing_identifiers.push(contribution.identifier);
            signature_shares.insert(frost_identifier, signature_share);
        }

        if contributing_identifiers.len() != signing_participants.len() {
            return Err(EngineError::Validation(format!(
                "round contribution identifiers must match signing participants for real finalize: expected {:?}, got {:?}",
                signing_participants, contributing_identifiers
            )));
        }

        let signing_package = frost::SigningPackage::new(commitments, sign_message_bytes);
        let signature = if let Some(taproot_merkle_root) = finalize_taproot_merkle_root.as_ref() {
            frost::aggregate_with_tweak(
                &signing_package,
                &signature_shares,
                dkg_public_key_package,
                Some(taproot_merkle_root.as_slice()),
            )
        } else {
            frost::aggregate(&signing_package, &signature_shares, dkg_public_key_package)
        }
        .map_err(|e| {
            EngineError::Validation(format!("failed to aggregate signature shares: {e}"))
        })?;

        let verification_key_package =
            if let Some(taproot_merkle_root) = finalize_taproot_merkle_root.as_ref() {
                dkg_public_key_package
                    .clone()
                    .tweak(Some(taproot_merkle_root.as_slice()))
            } else {
                dkg_public_key_package.clone()
            };
        verification_key_package
            .verifying_key()
            .verify(sign_message_bytes, &signature)
            .map_err(|e| {
                EngineError::Validation(format!(
                    "aggregate signature failed self-verification: {e}"
                ))
            })?;

        let signature_bytes = signature.serialize().map_err(|e| {
            EngineError::Internal(format!("failed to serialize aggregate signature: {e}"))
        })?;

        SignatureResult {
            session_id: request.session_id.clone(),
            round_id: round_state.round_id.clone(),
            signature_hex: hex::encode(signature_bytes),
        }
    };

    let consumed_round_id = round_state.round_id.clone();
    ensure_consumed_registry_insert_capacity(
        &session.consumed_finalize_round_ids,
        &consumed_round_id,
        "consumed_finalize_round_ids",
        &request.session_id,
    )?;
    ensure_consumed_registry_insert_capacity(
        &session.consumed_finalize_request_fingerprints,
        &request_fingerprint,
        "consumed_finalize_request_fingerprints",
        &request.session_id,
    )?;

    session.finalize_request_fingerprint = Some(request_fingerprint.clone());
    session.signature_result = Some(signature_result.clone());
    session
        .consumed_finalize_round_ids
        .insert(consumed_round_id);
    session
        .consumed_finalize_request_fingerprints
        .insert(request_fingerprint);
    clear_session_signing_material(session);
    persist_engine_state_to_storage(&guard)?;
    record_hardening_telemetry(|telemetry| {
        telemetry.finalize_sign_round_success_total = telemetry
            .finalize_sign_round_success_total
            .saturating_add(1);
    });

    Ok(signature_result)
}

pub(crate) fn build_bootstrap_synthetic_signature_result(
    session_id: &str,
    round_state: &RoundState,
    ordered_contributions: &[RoundContribution],
) -> Result<SignatureResult, EngineError> {
    let mut contribution_bytes = serde_json::to_vec(ordered_contributions)
        .map_err(|e| EngineError::Internal(format!("failed to encode contributions: {e}")))?;
    let mut contribution_hash = hash_hex(&contribution_bytes);
    contribution_bytes.zeroize();

    let mut signature_material = format!(
        "signature:{}:{}:{}",
        round_state.session_id, round_state.round_id, contribution_hash
    );
    contribution_hash.zeroize();
    let signature_hex = hash_hex(signature_material.as_bytes());
    signature_material.zeroize();

    Ok(SignatureResult {
        session_id: session_id.to_string(),
        round_id: round_state.round_id.clone(),
        signature_hex,
    })
}

pub(crate) fn uses_bootstrap_synthetic_contributions(
    round_state: &RoundState,
    contributions: &[RoundContribution],
) -> bool {
    contributions.iter().all(|contribution| {
        contribution
            .signature_share_hex
            .eq_ignore_ascii_case(&bootstrap_synthetic_share_hex(
                round_state,
                contribution.identifier,
            ))
    })
}

pub(crate) fn bootstrap_synthetic_share_hex(round_state: &RoundState, identifier: u16) -> String {
    bootstrap_synthetic_share_hex_for_round(
        &round_state.session_id,
        &round_state.round_id,
        &round_state.message_digest_hex,
        identifier,
    )
}

pub(crate) fn bootstrap_synthetic_share_hex_for_round(
    session_id: &str,
    round_id: &str,
    message_digest_hex: &str,
    identifier: u16,
) -> String {
    hash_hex(
        format!(
            "{}:{}:{}:{}:{}",
            BOOTSTRAP_SYNTHETIC_CONTRIBUTION_DOMAIN,
            session_id,
            round_id,
            message_digest_hex,
            identifier,
        )
        .as_bytes(),
    )
}
