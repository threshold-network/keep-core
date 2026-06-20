// Phase 7.1: the hardened interactive signing session layer.
//
// Implements sections 4-5 of the frozen spec
// (docs/phase-7-interactive-session-spec-freeze.md). The defining
// property is engine-held nonce custody: round-1 nonces are generated
// from OS randomness, live only in in-memory session state bound to
// (session_id, attempt_id), are zeroized on consumption, abort, and
// expiry, and are NEVER serialized into a response or persisted state.
// The only durable artifacts are per-attempt consumption markers,
// written BEFORE a signature share leaves the engine
// (consumption-before-release), so a restart can never lead to a
// second share under the same nonces.
//
// Attempt contexts are strict-mode only: there is no legacy-shape
// fallback on this path. All entry points are idempotent or fail
// closed; none of them can be made to release more than one signature
// share per nonce pair.

use super::*;

// Multi-seat: a session's interactive consumed-nonce markers are keyed per
// (attempt_id, member_identifier), so independent local seats can each consume
// their own nonces for the same attempt without colliding. The marker is written
// BEFORE a share leaves the engine (consumption-before-release). Legacy bare
// attempt_id markers (written by the pre-multi-seat single-member engine, and
// possibly reloaded from durable state) are honored FAIL-CLOSED on read: a bare
// marker means the attempt is consumed for every member.
pub(crate) fn interactive_consumed_marker(attempt_id: &str, member_identifier: u16) -> String {
    format!("m{member_identifier}@{attempt_id}")
}

pub(crate) fn interactive_attempt_consumed(
    markers: &HashSet<String>,
    attempt_id: &str,
    member_identifier: u16,
) -> bool {
    markers.contains(&interactive_consumed_marker(attempt_id, member_identifier))
        || markers.contains(attempt_id)
}

// The aggregate completion marker binds attempt_id to the AGGREGATED message digest,
// so the durable "this attempt is final" record cannot be set for one attempt id via
// a valid aggregate over a DIFFERENT message - which would otherwise let a replayed
// aggregate preempt an unrelated live attempt's Round2 (the Round2 completion gate).
// interactive_aggregate writes it from the package it aggregated; Round2 and the
// re-aggregate guard recompute it from the message they actually hold.
pub(crate) fn interactive_aggregated_marker(attempt_id: &str, message_digest_hex: &str) -> String {
    format!("{attempt_id}@{message_digest_hex}")
}

// Aggregate-completion check honoring legacy markers: the new bound form for THIS
// message, OR a legacy bare attempt_id marker (written by the pre-binding engine and
// reloaded from durable state) - the latter fail-closed, exactly like the consumed
// markers, so a completion persisted before this format change stays final (no repeat
// aggregate, no fresh Round2 share) after an upgrade.
pub(crate) fn interactive_attempt_aggregated(
    markers: &HashSet<String>,
    attempt_id: &str,
    message_digest_hex: &str,
) -> bool {
    markers.contains(&interactive_aggregated_marker(
        attempt_id,
        message_digest_hex,
    )) || markers.contains(attempt_id)
}

pub fn interactive_session_open(
    mut request: InteractiveSessionOpenRequest,
) -> Result<InteractiveSessionOpenResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_session_open_calls_total = telemetry
            .interactive_session_open_calls_total
            .saturating_add(1);
    });
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    if request.member_identifier == 0 {
        return Err(EngineError::Validation(
            "member_identifier must be non-zero".to_string(),
        ));
    }
    if request.threshold == 0 {
        return Err(EngineError::Validation(
            "threshold must be non-zero".to_string(),
        ));
    }

    let message_bytes = hex::decode(&request.message_hex)
        .map_err(|_| EngineError::Validation("message_hex must be valid hex".to_string()))?;
    if message_bytes.is_empty() {
        return Err(EngineError::Validation(
            "message_hex must not be empty".to_string(),
        ));
    }
    // Canonicalize message_hex to lowercase before it feeds the
    // open-request fingerprint below. attempt_id and
    // taproot_merkle_root_hex are already canonicalized
    // case-insensitively (and the coarse signing path lowercases the
    // signing message likewise), but the fingerprint serializes
    // message_hex verbatim - so without this a re-cased retry of an
    // otherwise identical open would mismatch the fingerprint and be
    // rejected as a SessionConflict instead of returning idempotent.
    // The decoded message_bytes are unaffected by hex casing.
    request.message_hex = request.message_hex.to_ascii_lowercase();
    let message_digest_hex = hash_hex(&message_bytes);
    let taproot_merkle_root =
        canonicalize_taproot_merkle_root_hex(&mut request.taproot_merkle_root_hex)?;

    // Canonicalize the attempt context before anything keys off it -
    // lowercases the hex hash fields and sorts the included set,
    // exactly as the coarse start_sign_round path does. The wire
    // accepts attempt_id/fingerprint case-insensitively, so the marker
    // registry and live-state comparisons MUST run on the canonical
    // form or a re-cased retry of a consumed attempt would miss the
    // marker and sign again.
    request.attempt_context = canonical_attempt_context(&request.attempt_context);

    let request_fingerprint = interactive_open_request_fingerprint(&request)?;
    let attempt_id = request.attempt_context.attempt_id.clone();

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    sweep_expired_interactive_state(&mut guard);

    let auto_quarantine_config = load_auto_quarantine_config()?;

    // The session must already exist with completed DKG. Key material
    // lives in the engine's own DKG-populated state and is NEVER
    // supplied through the request, so no signing secret crosses the
    // FFI/host boundary (frozen spec section 4). Resolve the member's
    // key package, run the policy gates, and validate the strict
    // attempt context against the DKG threshold/key group - mirroring
    // the coarse start_sign_round - all under one immutable borrow,
    // then do the mutable install.
    let (key_package, canonical_included_participants) = {
        let session = guard.sessions.get(&request.session_id).ok_or_else(|| {
            EngineError::SessionNotFound {
                session_id: request.session_id.clone(),
            }
        })?;
        let dkg = session
            .dkg_result
            .as_ref()
            .ok_or_else(|| EngineError::DkgNotReady {
                session_id: request.session_id.clone(),
            })?;
        if request.key_group != dkg.key_group {
            return Err(EngineError::Validation(
                "key_group does not match DKG output for this session".to_string(),
            ));
        }
        if request.threshold != dkg.threshold {
            return Err(EngineError::Validation(format!(
                "threshold [{}] does not match the DKG threshold [{}] for this session",
                request.threshold, dkg.threshold
            )));
        }
        let dkg_key_packages = session
            .dkg_key_packages
            .as_ref()
            .ok_or_else(|| EngineError::Internal("missing DKG key package cache".to_string()))?;
        let key_package = dkg_key_packages
            .get(&request.member_identifier)
            .ok_or_else(|| {
                EngineError::Validation(
                    "member_identifier is not a DKG participant for this session".to_string(),
                )
            })?
            .clone();

        // Lifecycle + quarantine + signing-policy-firewall gates (frozen
        // spec section 5: Open "checks policy gates"). The SAME helper
        // runs again at Round2 (the share-release moment) so a policy
        // change recorded after Open - emergency rekey, finalization,
        // quarantine, or a re-bound policy-checked tx - cannot let a
        // share escape. At Open only this node's own member is known to
        // sign; Round2 re-checks quarantine over the actual chosen
        // subset.
        enforce_interactive_signing_gates(
            &request.session_id,
            &[request.member_identifier],
            &request.message_hex,
            session.emergency_rekey_event.as_ref(),
            session.finalize_request_fingerprint.is_some(),
            session.tx_result.as_ref(),
            &guard.quarantined_operator_identifiers,
            auto_quarantine_config.as_ref(),
        )?;

        // Strict-mode-only attempt context: required, fully validated
        // against the DKG threshold/key group, coordinator recomputed
        // per RFC-21 Annex A.
        let canonical_included_participants = validate_attempt_context(
            &request.session_id,
            &dkg.key_group,
            &message_bytes,
            &message_digest_hex,
            dkg.threshold,
            Some(&request.attempt_context),
            true,
        )?
        .ok_or_else(|| {
            EngineError::Internal(
                "strict attempt context validation returned no participants".to_string(),
            )
        })?;
        if !canonical_included_participants.contains(&request.member_identifier) {
            return Err(EngineError::Validation(
                "member_identifier must be included in attempt_context.included_participants"
                    .to_string(),
            ));
        }
        // Every included participant must be a real DKG member of this
        // session. Otherwise a caller could pad the included set with
        // phantom identifiers to bias the RFC-21 coordinator/attempt
        // derivation, and Round2 could release a share under an attempt
        // context that is not a genuine DKG subset.
        for participant in &canonical_included_participants {
            if !dkg_key_packages.contains_key(participant) {
                return Err(EngineError::Validation(format!(
                    "attempt_context.included_participants contains [{participant}], \
                     which is not a DKG participant for this session"
                )));
            }
        }
        (key_package, canonical_included_participants)
    };

    // Disposition over the (now-confirmed) existing session: consumed
    // marker, idempotent/conflicting reopen of this exact attempt, and
    // the live attempt (id + number) for the replacement decision.
    let member_identifier = request.member_identifier;
    let (already_consumed, matching_attempt_idempotent, live_attempt) = {
        let session = guard
            .sessions
            .get(&request.session_id)
            .expect("session existed under the held engine lock");
        // Per-member consumed check: this member's composite marker, or a legacy
        // bare attempt_id marker (fail-closed for the whole attempt).
        let already_consumed = interactive_attempt_consumed(
            &session.consumed_interactive_attempt_markers,
            &attempt_id,
            member_identifier,
        );
        // Disposition is scoped to THIS member's live entry; sibling seats are
        // independent and on their own attempt timelines.
        let live = session.interactive_signing.get(&member_identifier);
        let matching_attempt_idempotent = live
            .filter(|interactive| interactive.attempt_context.attempt_id == attempt_id)
            .map(|interactive| interactive.open_request_fingerprint == request_fingerprint);
        let live_attempt = live.map(|interactive| {
            (
                interactive.attempt_context.attempt_id.clone(),
                interactive.attempt_context.attempt_number,
            )
        });
        (already_consumed, matching_attempt_idempotent, live_attempt)
    };

    if already_consumed {
        return Err(EngineError::ConsumedNonceReplay {
            session_id: request.session_id.clone(),
            attempt_id,
        });
    }

    match matching_attempt_idempotent {
        Some(true) => {
            return Ok(InteractiveSessionOpenResult {
                session_id: request.session_id,
                attempt_id,
                idempotent: true,
            });
        }
        Some(false) => {
            return Err(EngineError::SessionConflict {
                session_id: request.session_id.clone(),
            });
        }
        None => {}
    }

    // A DIFFERENT live attempt FOR THIS MEMBER is replaced ONLY by a strictly
    // newer attempt: this seat's retry loop advanced. A stale/delayed open for an
    // older or equal attempt must not roll this member back and wipe its newer
    // nonces. A sibling seat on a different (even newer) attempt is irrelevant -
    // seats advance independently, exactly as separate processes would.
    let replacing = live_attempt.is_some();
    if let Some((live_attempt_id, live_attempt_number)) = live_attempt {
        // By construction the live attempt here is a DIFFERENT attempt:
        // a live attempt with the same attempt_id would have been
        // resolved above as idempotent (Some(true)) or conflicting
        // (Some(false)) and returned. Assert that invariant instead of
        // re-testing it at runtime - the assert stays so that a future
        // change to the matching_attempt_idempotent logic which let an
        // equal attempt_id reach here trips loudly, rather than silently
        // rolling back a live attempt's nonces.
        debug_assert_ne!(
            live_attempt_id, attempt_id,
            "a live attempt with a matching attempt_id must have been resolved above"
        );
        if request.attempt_context.attempt_number <= live_attempt_number {
            return Err(EngineError::Validation(format!(
                "attempt_number [{}] does not advance member [{}]'s live interactive attempt [{}]; \
                 refusing to roll back to an older or equal attempt",
                request.attempt_context.attempt_number, member_identifier, live_attempt_number
            )));
        }
    }

    // Capacity counts every live interactive session. When replacing,
    // this session already holds one of those slots, so the cap does
    // not apply; when not replacing, a new slot is being taken.
    // Capacity bounds resident nonce/key exposure, so it counts every live member
    // ENTRY across all sessions. Replacing this member's own entry takes no new
    // slot; a new member (even in an existing session) takes one.
    if !replacing {
        let live_interactive_members: usize = guard
            .sessions
            .values()
            .map(|session| session.interactive_signing.len())
            .sum();
        if live_interactive_members >= max_live_interactive_sessions_limit() {
            return Err(EngineError::Internal(format!(
                "live interactive member count [{live_interactive_members}] reached max [{}]; \
                 abort idle sessions or increase {}",
                max_live_interactive_sessions_limit(),
                TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV
            )));
        }
    }

    let session = guard
        .sessions
        .get_mut(&request.session_id)
        .expect("session existed under the held engine lock");

    // Replace only THIS member's prior entry (zeroizing its old nonces); sibling
    // seats' entries are untouched.
    if let Some(mut replaced) = session.interactive_signing.remove(&member_identifier) {
        zeroize_interactive_round1(&mut replaced);
    }

    session.interactive_signing.insert(
        member_identifier,
        InteractiveSigningState {
            open_request_fingerprint: request_fingerprint,
            attempt_context: request.attempt_context,
            canonical_included_participants,
            member_identifier,
            threshold: request.threshold,
            message_bytes: Zeroizing::new(message_bytes),
            taproot_merkle_root,
            key_package,
            opened_at_unix: now_unix(),
            round1: None,
        },
    );

    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_session_open_success_total = telemetry
            .interactive_session_open_success_total
            .saturating_add(1);
    });

    Ok(InteractiveSessionOpenResult {
        session_id: request.session_id,
        attempt_id,
        idempotent: false,
    })
}

pub fn interactive_round1(
    request: InteractiveRound1Request,
) -> Result<InteractiveRound1Result, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_round1_calls_total =
            telemetry.interactive_round1_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::InteractiveRound1);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    // The live state and markers are keyed on the canonical (lowercase)
    // attempt_id; the wire form may differ in casing.
    let attempt_id = canonical_attempt_id(&request.attempt_id);

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    sweep_expired_interactive_state(&mut guard);

    let session = guard.sessions.get_mut(&request.session_id).ok_or_else(|| {
        EngineError::SessionNotFound {
            session_id: request.session_id.clone(),
        }
    })?;

    if interactive_attempt_consumed(
        &session.consumed_interactive_attempt_markers,
        &attempt_id,
        request.member_identifier,
    ) {
        return Err(EngineError::ConsumedNonceReplay {
            session_id: request.session_id.clone(),
            attempt_id,
        });
    }

    let interactive = interactive_state_for_attempt_mut(
        session,
        &request.session_id,
        &attempt_id,
        request.member_identifier,
    )?;

    if let Some(round1) = interactive.round1.as_ref() {
        // Idempotent until consumed: the commitments are public and
        // re-sending them is safe; the nonces never leave.
        return Ok(InteractiveRound1Result {
            commitments_hex: round1.commitments_hex.clone(),
        });
    }

    let mut rng = zeroizing_rng_from_os();
    let (nonces, commitments) =
        frost::round1::commit(interactive.key_package.signing_share(), &mut rng);
    let commitment_bytes = commitments.serialize().map_err(|e| {
        EngineError::Internal(format!("failed to serialize signing commitments: {e}"))
    })?;
    let commitments_hex = hex::encode(commitment_bytes);

    interactive.round1 = Some(InteractiveRound1State {
        nonces,
        commitments_hex: commitments_hex.clone(),
    });

    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_round1_success_total =
            telemetry.interactive_round1_success_total.saturating_add(1);
    });

    Ok(InteractiveRound1Result { commitments_hex })
}

pub fn interactive_round2(
    request: InteractiveRound2Request,
) -> Result<InteractiveRound2Result, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_round2_calls_total =
            telemetry.interactive_round2_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::InteractiveRound2);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    let mut signing_package_bytes = decode_hex_field(
        "InteractiveRound2",
        "signing_package_hex",
        &request.signing_package_hex,
    )?;
    let signing_package_result = frost::SigningPackage::deserialize(&signing_package_bytes);
    signing_package_bytes.zeroize();
    let signing_package = signing_package_result.map_err(|e| {
        EngineError::Validation(format!("InteractiveRound2: invalid signing package: {e}"))
    })?;

    // The live state and markers are keyed on the canonical (lowercase)
    // attempt_id; the wire form may differ in casing.
    let attempt_id = canonical_attempt_id(&request.attempt_id);

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    sweep_expired_interactive_state(&mut guard);

    // Quarantine inputs must be read before the session is borrowed
    // mutably from the same guard below.
    let auto_quarantine_config = load_auto_quarantine_config()?;
    let quarantined_operator_identifiers = guard.quarantined_operator_identifiers.clone();

    let session = guard.sessions.get_mut(&request.session_id).ok_or_else(|| {
        EngineError::SessionNotFound {
            session_id: request.session_id.clone(),
        }
    })?;

    if interactive_attempt_consumed(
        &session.consumed_interactive_attempt_markers,
        &attempt_id,
        request.member_identifier,
    ) {
        return Err(EngineError::ConsumedNonceReplay {
            session_id: request.session_id.clone(),
            attempt_id,
        });
    }

    // A completed attempt releases no further shares. Once interactive_aggregate has
    // produced the attempt's signature, an open sibling seat that never signed has NO
    // per-member consumed marker, so the consumed gate above does not cover it. Gate
    // on the completion marker too - but matched to the MESSAGE this member opened
    // (the marker binds attempt_id to the aggregated message digest), so a replayed
    // aggregate carrying a different message for this attempt id cannot preempt this
    // member's live Round2. It fires only for a genuine same-message completion; the
    // member's entry for that finalized attempt is then dead, so free it (zeroizing
    // its nonces) rather than holding a live-member slot until the TTL sweep.
    let member_attempt_message_digest = session
        .interactive_signing
        .get(&request.member_identifier)
        .filter(|entry| entry.attempt_context.attempt_id == attempt_id)
        .map(|entry| hash_hex(&entry.message_bytes));
    let attempt_finalized = member_attempt_message_digest
        .as_deref()
        .is_some_and(|digest| {
            interactive_attempt_aggregated(
                &session.aggregated_interactive_attempt_markers,
                &attempt_id,
                digest,
            )
        });
    if attempt_finalized {
        if let Some(mut removed) = session
            .interactive_signing
            .remove(&request.member_identifier)
        {
            zeroize_interactive_round1(&mut removed);
        }
        return Err(EngineError::InteractiveAttemptAlreadyAggregated {
            session_id: request.session_id.clone(),
            attempt_id,
        });
    }

    // Per-member consumed marker (composite): independent seats consume their own
    // nonces for the same attempt without colliding.
    let consumed_marker = interactive_consumed_marker(&attempt_id, request.member_identifier);
    ensure_consumed_registry_insert_capacity(
        &session.consumed_interactive_attempt_markers,
        &consumed_marker,
        "consumed_interactive_attempt_markers",
        &request.session_id,
    )?;

    // Re-evaluate the signing gates at the share-release moment. The
    // gates checked at Open are stale here: a kill switch recorded
    // after Open (emergency rekey, finalization, quarantine, or a
    // re-bound policy-checked tx) must stop the share leaving the
    // engine. Read via immutable borrows of the live attempt before the
    // mutable consume/sign borrow below. Skipped when no matching live
    // attempt exists - there is no share to release in that case, and
    // interactive_state_for_attempt_mut produces the canonical error.
    if let Some(interactive) = session
        .interactive_signing
        .get(&request.member_identifier)
        .filter(|interactive| interactive.attempt_context.attempt_id == attempt_id)
    {
        let bound_message_hex = hex::encode(interactive.message_bytes.as_slice());
        // Fast-path lifecycle/firewall and this node's own quarantine.
        // The full chosen signing subset is quarantine-checked after the
        // package is verified (below), once it is known to be a real
        // subset of the included set.
        enforce_interactive_signing_gates(
            &request.session_id,
            &[request.member_identifier],
            &bound_message_hex,
            session.emergency_rekey_event.as_ref(),
            session.finalize_request_fingerprint.is_some(),
            session.tx_result.as_ref(),
            &quarantined_operator_identifiers,
            auto_quarantine_config.as_ref(),
        )?;
    }

    let interactive = interactive_state_for_attempt_mut(
        session,
        &request.session_id,
        &attempt_id,
        request.member_identifier,
    )?;

    if interactive.round1.is_none() {
        return Err(EngineError::SignRoundNotStarted {
            session_id: request.session_id.clone(),
        });
    }

    // ALL verification precedes consumption (frozen spec section 5,
    // Round2): a package that fails any check leaves the nonce handle
    // live, so an invalid package cannot burn the attempt. At most one
    // share per handle still holds against two VALID packages because
    // the consumption marker is written before the share is released.
    verify_round2_signing_package(interactive, &signing_package)?;

    // The package is now confirmed to be a threshold-sized subset of the
    // attempt's included set, so the chosen signing subset is known.
    // Quarantine-check ALL of it before releasing a share: this node
    // must not contribute to a signature whose subset includes a
    // locally quarantined co-signer, matching the coarse path's
    // all-signing-participants quarantine enforcement.
    let signing_subset = round2_signing_subset(interactive, &signing_package)?;
    enforce_not_quarantined_identifiers(
        &request.session_id,
        &signing_subset,
        &quarantined_operator_identifiers,
        auto_quarantine_config.as_ref(),
    )?;

    // Consumption-before-release: the durable marker is persisted
    // BEFORE the share is computed and returned. If persistence fails,
    // the marker is rolled back and the nonces remain live - no share
    // has left the engine. If share computation fails after the marker
    // persisted, the attempt is dead (fail closed): the marker stays,
    // the nonces are destroyed, and no share was released.
    session
        .consumed_interactive_attempt_markers
        .insert(consumed_marker.clone());
    if let Err(persist_error) = persist_engine_state_to_storage(&guard) {
        let session = guard
            .sessions
            .get_mut(&request.session_id)
            .expect("session existed under the held engine lock");
        session
            .consumed_interactive_attempt_markers
            .remove(&consumed_marker);
        return Err(persist_error);
    }

    let session = guard
        .sessions
        .get_mut(&request.session_id)
        .expect("session existed under the held engine lock");
    let interactive = session
        .interactive_signing
        .get_mut(&request.member_identifier)
        .expect("interactive state existed under the held engine lock");

    let mut round1 = interactive
        .round1
        .take()
        .expect("round1 state existed under the held engine lock");

    let signature_share_result =
        if let Some(taproot_merkle_root) = interactive.taproot_merkle_root.as_ref() {
            frost::round2::sign_with_tweak(
                &signing_package,
                &round1.nonces,
                &interactive.key_package,
                Some(taproot_merkle_root.as_slice()),
            )
        } else {
            frost::round2::sign(&signing_package, &round1.nonces, &interactive.key_package)
        };
    round1.nonces.zeroize();
    drop(round1);

    // Round2 is terminal for THIS member's participation in the attempt: the
    // marker is durable and the nonces are gone, so free this member's entry now
    // rather than letting it (and its resident key package + message) linger until
    // the TTL sweep. This also returns its capacity slot immediately; sibling seats
    // stay live. Done on both the success and share-computation-failure paths: the
    // attempt is consumed for this member either way, and the durable marker
    // carries all further replay protection.
    session
        .interactive_signing
        .remove(&request.member_identifier);

    let signature_share = signature_share_result
        .map_err(|e| EngineError::Internal(format!("failed to create signature share: {e}")))?;

    let mut signature_share_bytes = signature_share.serialize();
    let signature_share_hex = hex::encode(&signature_share_bytes);
    signature_share_bytes.zeroize();

    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_round2_success_total =
            telemetry.interactive_round2_success_total.saturating_add(1);
    });

    Ok(InteractiveRound2Result {
        session_id: request.session_id,
        attempt_id,
        signature_share_hex,
    })
}

pub fn interactive_aggregate(
    request: InteractiveAggregateRequest,
) -> Result<InteractiveAggregateResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_aggregate_calls_total = telemetry
            .interactive_aggregate_calls_total
            .saturating_add(1);
    });
    let _latency_guard =
        HardeningOperationLatencyGuard::new(HardeningOperation::InteractiveAggregate);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;
    let attempt_id = canonical_attempt_id(&request.attempt_id);
    // The completion marker persists attempt_id, and the reload path rejects an
    // empty key; reject an empty attempt_id here too so a malformed (or
    // malicious) request cannot write durable state that fails to reload after
    // a restart.
    if attempt_id.is_empty() {
        return Err(EngineError::Validation(
            "InteractiveAggregate: attempt_id must not be empty".to_string(),
        ));
    }

    let mut signing_package_bytes = decode_hex_field(
        "InteractiveAggregate",
        "signing_package_hex",
        &request.signing_package_hex,
    )?;
    let signing_package_result = frost::SigningPackage::deserialize(&signing_package_bytes);
    signing_package_bytes.zeroize();
    let signing_package = signing_package_result.map_err(|e| {
        EngineError::Validation(format!(
            "InteractiveAggregate: invalid signing package: {e}"
        ))
    })?;
    // The completion marker binds attempt_id to THIS aggregated message digest, so a
    // valid aggregate over a different message cannot finalize this attempt id (and
    // so cannot, via the Round2 completion gate, preempt an unrelated live attempt).
    let aggregated_message_digest = hash_hex(signing_package.message().as_slice());
    let aggregated_marker = interactive_aggregated_marker(&attempt_id, &aggregated_message_digest);
    let signature_shares =
        decode_signature_share_map("InteractiveAggregate", &request.signature_shares)?;
    let mut taproot_merkle_root_hex = request.taproot_merkle_root_hex.clone();
    let taproot_merkle_root = canonicalize_taproot_merkle_root_hex(&mut taproot_merkle_root_hex)?;

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    // Aggregate takes the engine lock like every other interactive entry
    // point, so it sweeps expired interactive state too: the TTL
    // guarantee (a nonce handle gone within the TTL of inactivity) must
    // hold even when the only post-expiry traffic is aggregate calls.
    sweep_expired_interactive_state(&mut guard);

    // Resolve the group's public key package (the verifying shares used
    // to check each contribution) from the session's own DKG state, not
    // the request - consistent with the no-secret-on-the-FFI discipline
    // and so a caller cannot substitute verifying material. The session
    // must exist with completed DKG.
    let public_key_package = {
        let session = guard.sessions.get(&request.session_id).ok_or_else(|| {
            EngineError::SessionNotFound {
                session_id: request.session_id.clone(),
            }
        })?;
        // Reject a completed attempt: re-aggregation is not a recovery path (a
        // lost signature is recovered with a fresh attempt), and the marker is
        // durable so a completed attempt stays rejected across a restart.
        if interactive_attempt_aggregated(
            &session.aggregated_interactive_attempt_markers,
            &attempt_id,
            &aggregated_message_digest,
        ) {
            return Err(EngineError::InteractiveAttemptAlreadyAggregated {
                session_id: request.session_id.clone(),
                attempt_id,
            });
        }
        if session.dkg_result.is_none() {
            return Err(EngineError::DkgNotReady {
                session_id: request.session_id.clone(),
            });
        }
        session
            .dkg_public_key_package
            .as_ref()
            .ok_or_else(|| {
                EngineError::Internal("missing DKG public key package cache".to_string())
            })?
            .clone()
    };
    drop(guard);

    // Aggregation uses only public material (commitments, shares,
    // verifying shares), so no policy gate runs here - the secret-bearing
    // step is each signer's Round2, where lifecycle/quarantine/firewall
    // were already enforced (including the full-subset quarantine check).
    //
    // frost verifies every share and names which failed. This path now surfaces
    // those as CANDIDATE culprits (Phase 7.2b-3): the engine reports the members
    // whose shares did not verify against the group's own verifying material,
    // but it does NOT adjudicate fault. The engine cannot bind these public
    // inputs (signing package, taproot root) to what each member signed at
    // Round2, so a coordinator that aggregated honest shares against a
    // substituted package/root would make those honest shares fail and appear
    // here. Authoritative, envelope-bound blame is the Go host's job at an f+1
    // accuser quorum (frozen Phase 7.2b spec, section 6), using the signed
    // signing-package envelopes; this candidate list is its input. Fail-closed
    // either way: no signature leaves on a verification failure.
    let verification_key_package = match taproot_merkle_root.as_ref() {
        Some(root) => public_key_package.clone().tweak(Some(root.as_slice())),
        None => public_key_package.clone(),
    };

    // Aggregate with AllCheaters detection. The frost-secp256k1-tr
    // aggregate/aggregate_with_tweak wrappers hardcode FirstCheater, so a
    // failure would name only one member; AllCheaters names EVERY member whose
    // share failed. verification_key_package is the (taproot-tweaked, when a
    // root is set) public key package - exactly what aggregate_with_tweak
    // derives internally - so this is equivalent to those wrappers on the
    // success path. Cheater detection only runs after the aggregate signature
    // itself fails to verify, so there is no happy-path cost.
    let signature = match frost_core::aggregate_custom(
        &signing_package,
        &signature_shares,
        &verification_key_package,
        frost_core::CheaterDetection::AllCheaters,
    ) {
        Ok(signature) => signature,
        Err(error) => {
            let candidate_culprits = aggregate_candidate_culprits(&error);
            if candidate_culprits.is_empty() {
                // Not a per-member share attribution (malformed package, wrong
                // share count, group/field error): fail closed with the generic
                // validation error, no blame.
                return Err(EngineError::Validation(format!(
                    "InteractiveAggregate: failed to aggregate: {error}"
                )));
            }
            return Err(EngineError::AggregateShareVerificationFailed {
                session_id: request.session_id.clone(),
                attempt_id,
                candidate_culprits,
            });
        }
    };

    // Self-verify the aggregate against the (tweaked) group verifying
    // key before releasing it, matching the coarse finalize path.
    verification_key_package
        .verifying_key()
        .verify(signing_package.message().as_slice(), &signature)
        .map_err(|e| {
            EngineError::Validation(format!(
                "InteractiveAggregate: aggregate signature failed self-verification: {e}"
            ))
        })?;

    let signature_bytes = signature
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize aggregate: {e}")))?;
    let signature_hex = hex::encode(signature_bytes);

    // Mark the attempt complete before reporting success, so a repeat
    // InteractiveAggregate is rejected rather than recomputed (Phase 7.2b design
    // section 6). The engine lock was dropped for the aggregation crypto above;
    // re-acquire it, re-check the marker (a concurrent aggregate may have
    // completed first), insert it, and persist before reporting success; on
    // persist failure roll the marker back and fail closed.
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let session = guard.sessions.get_mut(&request.session_id).ok_or_else(|| {
        EngineError::SessionNotFound {
            session_id: request.session_id.clone(),
        }
    })?;
    // A concurrent aggregate that raced past the pre-check may have completed
    // this attempt first; if the marker is now present, reject this call's
    // re-aggregation - the winner already produced the attempt's signature.
    if interactive_attempt_aggregated(
        &session.aggregated_interactive_attempt_markers,
        &attempt_id,
        &aggregated_message_digest,
    ) {
        return Err(EngineError::InteractiveAttemptAlreadyAggregated {
            session_id: request.session_id.clone(),
            attempt_id,
        });
    }
    ensure_consumed_registry_insert_capacity(
        &session.aggregated_interactive_attempt_markers,
        &aggregated_marker,
        "aggregated_interactive_attempt_markers",
        &request.session_id,
    )?;
    session
        .aggregated_interactive_attempt_markers
        .insert(aggregated_marker.clone());
    if let Err(persist_error) = persist_engine_state_to_storage(&guard) {
        let session = guard
            .sessions
            .get_mut(&request.session_id)
            .expect("session existed under the held engine lock");
        session
            .aggregated_interactive_attempt_markers
            .remove(&aggregated_marker);
        return Err(persist_error);
    }
    drop(guard);

    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_aggregate_success_total = telemetry
            .interactive_aggregate_success_total
            .saturating_add(1);
    });

    Ok(InteractiveAggregateResult {
        session_id: request.session_id,
        attempt_id,
        signature_hex,
    })
}

pub fn interactive_session_abort(
    request: InteractiveSessionAbortRequest,
) -> Result<InteractiveSessionAbortResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_session_abort_calls_total = telemetry
            .interactive_session_abort_calls_total
            .saturating_add(1);
    });
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    // Canonicalize the optional attempt_id filter to match the
    // canonical form the live state is keyed on.
    let attempt_id_filter = request.attempt_id.as_deref().map(canonical_attempt_id);

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    // Abort takes the lock like every other entry point, so it sweeps
    // expired interactive state too: the TTL guarantee (nonces gone
    // within the TTL of inactivity) must hold even when the only
    // post-expiry traffic is aborts for other sessions.
    sweep_expired_interactive_state(&mut guard);

    let aborted = match guard.sessions.get_mut(&request.session_id) {
        Some(session) => {
            // Abort has no member parameter, so it is session-level over the map:
            // remove every member entry matching the optional attempt filter,
            // zeroizing each entry's nonces. Sibling members on a non-matching
            // attempt survive. Aborted iff at least one entry was removed.
            let members_to_abort: Vec<u16> = session
                .interactive_signing
                .iter()
                .filter(|(_, interactive)| {
                    attempt_id_filter.is_none()
                        || attempt_id_filter.as_deref()
                            == Some(interactive.attempt_context.attempt_id.as_str())
                })
                .map(|(member, _)| *member)
                .collect();
            for member in &members_to_abort {
                if let Some(mut removed) = session.interactive_signing.remove(member) {
                    zeroize_interactive_round1(&mut removed);
                }
            }
            !members_to_abort.is_empty()
        }
        None => false,
    };

    // Only count a success when live interactive state was actually
    // aborted. A no-op call (no session, or an attempt_id filter that
    // matched nothing) returns aborted == false and must not inflate the
    // success counter - the calls_total counter at the top already
    // records that the entry point ran.
    if aborted {
        record_hardening_telemetry(|telemetry| {
            telemetry.interactive_session_abort_success_total = telemetry
                .interactive_session_abort_success_total
                .saturating_add(1);
        });
    }

    Ok(InteractiveSessionAbortResult {
        session_id: request.session_id,
        aborted,
    })
}

// Looks up the live interactive state and pins the
// (attempt_id, member_identifier) binding every round call must carry.
fn interactive_state_for_attempt_mut<'session>(
    session: &'session mut SessionState,
    session_id: &str,
    attempt_id: &str,
    member_identifier: u16,
) -> Result<&'session mut InteractiveSigningState, EngineError> {
    let interactive = session
        .interactive_signing
        .get_mut(&member_identifier)
        .ok_or_else(|| EngineError::SessionNotFound {
            session_id: format!(
                "{session_id} (no live interactive attempt for member {member_identifier})"
            ),
        })?;

    if interactive.attempt_context.attempt_id != attempt_id {
        return Err(EngineError::Validation(format!(
            "attempt_id [{attempt_id}] does not match member [{member_identifier}]'s \
             live interactive attempt [{}]",
            interactive.attempt_context.attempt_id
        )));
    }

    Ok(interactive)
}

// The frozen spec's Round2 checks (a)-(f). Returns Ok only when every
// check passes; the caller consumes the nonces strictly afterwards.
fn verify_round2_signing_package(
    interactive: &InteractiveSigningState,
    signing_package: &frost::SigningPackage,
) -> Result<(), EngineError> {
    // (d) part 2 (deserialization already succeeded): the package must
    // target exactly the session's message. A package for any other
    // message - including the same message with different framing -
    // must never reach the nonces.
    if signing_package.message().as_slice() != interactive.message_bytes.as_slice() {
        return Err(EngineError::Validation(
            "signing package message does not match the open interactive session".to_string(),
        ));
    }

    let package_commitments = signing_package.signing_commitments();

    // (c) exactly threshold-many participants, deliberately not
    // at-least (frozen spec section 5).
    if package_commitments.len() != usize::from(interactive.threshold) {
        return Err(EngineError::Validation(format!(
            "signing package carries [{}] commitments; expected exactly threshold [{}]",
            package_commitments.len(),
            interactive.threshold
        )));
    }

    // (b) the chosen subset must be inside the attempt's included set.
    let included_identifiers = interactive
        .canonical_included_participants
        .iter()
        .map(|participant| participant_identifier_to_frost_identifier(*participant))
        .collect::<Result<BTreeSet<_>, _>>()?;
    for package_identifier in package_commitments.keys() {
        if !included_identifiers.contains(package_identifier) {
            return Err(EngineError::Validation(
                "signing package contains a participant outside the attempt's included set"
                    .to_string(),
            ));
        }
    }

    // (a) this member must be in the chosen subset.
    let own_identifier = participant_identifier_to_frost_identifier(interactive.member_identifier)?;
    let own_package_commitments = package_commitments.get(&own_identifier).ok_or_else(|| {
        EngineError::Validation(
            "signing package does not include this member's commitment".to_string(),
        )
    })?;

    // (f) the member's own commitment entry must be byte-identical to
    // its round-1 output. Without this, a malicious coordinator could
    // substitute the commitment, make this member's correctly-computed
    // share fail verification at aggregation, and manufacture false
    // blame evidence against an honest member.
    let own_package_commitment_bytes = own_package_commitments.serialize().map_err(|e| {
        EngineError::Internal(format!("failed to serialize package commitment: {e}"))
    })?;
    let round1 = interactive
        .round1
        .as_ref()
        .expect("caller verified round1 state exists");
    if hex::encode(own_package_commitment_bytes) != round1.commitments_hex {
        return Err(EngineError::Validation(
            "signing package commitment for this member does not match its round-1 output"
                .to_string(),
        ));
    }

    Ok(())
}

// The signing gates the interactive path enforces at BOTH Open and
// the Round2 share-release moment, mirroring the coarse
// start_sign_round: emergency-rekey and finalized lifecycle, quarantine
// of the signing participants, and the signing-policy firewall binding
// of the message to a policy-checked build_taproot_tx. Centralized in
// one function so the two call sites cannot drift apart.
//
// quarantine_identifiers is the set to quarantine-check: at Open only
// this node's own member is known to sign; at Round2 it is the full
// chosen signing subset (the package's participants), so this node
// refuses to contribute a share to a package that includes any
// quarantined co-signer - the same all-participants check the coarse
// path applies.
#[allow(clippy::too_many_arguments)]
fn enforce_interactive_signing_gates(
    session_id: &str,
    quarantine_identifiers: &[u16],
    message_hex: &str,
    emergency_rekey_event: Option<&EmergencyRekeyEvent>,
    session_finalized: bool,
    tx_result: Option<&TransactionResult>,
    quarantined_operator_identifiers: &HashSet<u16>,
    auto_quarantine_config: Option<&AutoQuarantineConfig>,
) -> Result<(), EngineError> {
    if let Some(emergency_rekey_event) = emergency_rekey_event {
        return Err(EngineError::LifecyclePolicyRejected {
            session_id: session_id.to_string(),
            reason_code: "emergency_rekey_required".to_string(),
            detail: format!(
                "emergency rekey required for session [{}] since [{}]: {}",
                session_id, emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
            ),
        });
    }
    if session_finalized {
        return Err(EngineError::SessionFinalized {
            session_id: session_id.to_string(),
        });
    }
    enforce_not_quarantined_identifiers(
        session_id,
        quarantine_identifiers,
        quarantined_operator_identifiers,
        auto_quarantine_config,
    )?;
    enforce_signing_message_binding_to_policy_checked_build_tx(session_id, message_hex, tx_result)
}

// Canonical key form for an attempt_id at the round entry points,
// matching canonicalize_attempt_context_for_fingerprint (which
// lowercases attempt_id). The wire accepts attempt_id case-
// insensitively, so the marker registry and live-state lookups must
// operate on this form to be replay-safe.
fn canonical_attempt_id(attempt_id: &str) -> String {
    attempt_id.to_ascii_lowercase()
}

// The chosen signing subset as Go u16 identifiers: the included
// participants whose commitment appears in the signing package. The
// caller MUST have run verify_round2_signing_package first (which
// confirms the package is a threshold-sized subset of the included
// set), so every package participant maps back to an included member.
fn round2_signing_subset(
    interactive: &InteractiveSigningState,
    signing_package: &frost::SigningPackage,
) -> Result<Vec<u16>, EngineError> {
    let package_identifiers = signing_package
        .signing_commitments()
        .keys()
        .copied()
        .collect::<BTreeSet<_>>();
    let mut subset = Vec::with_capacity(package_identifiers.len());
    for participant in &interactive.canonical_included_participants {
        let frost_identifier = participant_identifier_to_frost_identifier(*participant)?;
        if package_identifiers.contains(&frost_identifier) {
            subset.push(*participant);
        }
    }
    Ok(subset)
}

pub(crate) fn zeroize_interactive_round1(interactive: &mut InteractiveSigningState) {
    if let Some(mut round1) = interactive.round1.take() {
        round1.nonces.zeroize();
    }
}

// Lazy TTL enforcement: every interactive entry point sweeps before
// acting, so an abandoned session's nonces are destroyed the first
// time anything touches the engine after expiry. Expiry has abort
// semantics - the durable consumption markers are untouched.
pub(crate) fn sweep_expired_interactive_state(engine_state: &mut EngineState) {
    let ttl_seconds = interactive_session_ttl_seconds();
    let now = now_unix();
    // Interactive sessions always ride a DKG-populated session (Open
    // requires existing DKG state), so expiry only clears the live
    // attempt's nonces; the session itself - DKG material, consumed
    // markers - is retained for future signing.
    for session in engine_state.sessions.values_mut() {
        // Per-member expiry: each seat's entry expires independently by its own
        // opened_at_unix; non-expired sibling seats in the same session survive.
        let expired_members: Vec<u16> = session
            .interactive_signing
            .iter()
            .filter(|(_, interactive)| now.saturating_sub(interactive.opened_at_unix) > ttl_seconds)
            .map(|(member, _)| *member)
            .collect();
        for member in &expired_members {
            if let Some(mut removed) = session.interactive_signing.remove(member) {
                zeroize_interactive_round1(&mut removed);
            }
        }
    }
}

pub(crate) fn max_live_interactive_sessions_limit() -> usize {
    signer_env_var(TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV)
        .and_then(|value| value.trim().parse::<usize>().ok())
        .filter(|limit| *limit > 0)
        .unwrap_or(TBTC_SIGNER_DEFAULT_MAX_LIVE_INTERACTIVE_SESSIONS)
}

pub(crate) fn interactive_session_ttl_seconds() -> u64 {
    signer_env_var(TBTC_SIGNER_INTERACTIVE_SESSION_TTL_SECONDS_ENV)
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|ttl| *ttl > 0)
        .unwrap_or(TBTC_SIGNER_DEFAULT_INTERACTIVE_SESSION_TTL_SECONDS)
}

fn interactive_open_request_fingerprint(
    request: &InteractiveSessionOpenRequest,
) -> Result<String, EngineError> {
    // The serialized request transiently holds the signing inputs
    // (message_hex and the rest of the request) in plaintext; wipe the
    // buffer once the fingerprint digest is taken. No key material is
    // carried in the request - it is resolved from DKG state - so only
    // the request inputs are exposed here.
    let mut canonical = serde_json::to_vec(request).map_err(|e| {
        EngineError::Internal(format!(
            "failed to serialize InteractiveSessionOpen request for fingerprint: {e}"
        ))
    })?;
    let fingerprint = hash_hex(&canonical);
    canonical.zeroize();
    Ok(fingerprint)
}
