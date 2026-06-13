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

    // Strict-mode-only attempt context: required, fully validated,
    // coordinator recomputed per RFC-21 Annex A.
    let canonical_included_participants = validate_attempt_context(
        &request.session_id,
        &request.key_group,
        &message_bytes,
        &message_digest_hex,
        request.threshold,
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

    let key_package = decode_key_package(
        "InteractiveSessionOpen",
        &request.key_package_identifier,
        &request.key_package_hex,
    )?;
    let expected_identifier =
        participant_identifier_to_frost_identifier(request.member_identifier)?;
    if *key_package.identifier() != expected_identifier {
        return Err(EngineError::Validation(
            "key_package_identifier must match member_identifier".to_string(),
        ));
    }

    let request_fingerprint = interactive_open_request_fingerprint(&request)?;
    let attempt_id = request.attempt_context.attempt_id.clone();

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    sweep_expired_interactive_state(&mut guard);

    ensure_session_insert_capacity(&guard.sessions, &request.session_id)?;

    // Session lifecycle gates (frozen spec section 5: Open "checks
    // policy gates"). The interactive path must refuse in exactly the
    // states the coarse start_sign_round refuses: a session under an
    // emergency rekey, or one already terminally finalized. Without
    // these, InteractiveRound1/Round2 could emit a share where the
    // established path would not.
    if let Some(existing_session) = guard.sessions.get(&request.session_id) {
        if let Some(emergency_rekey_event) = existing_session.emergency_rekey_event.as_ref() {
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
        if existing_session.finalize_request_fingerprint.is_some() {
            return Err(EngineError::SessionFinalized {
                session_id: request.session_id.clone(),
            });
        }
    }

    // Quarantine gate: this node is about to produce a share for
    // member_identifier, so an auto-quarantined member (absent a DAO
    // allowlist override) must not be able to sign through the
    // interactive path either.
    let auto_quarantine_config = load_auto_quarantine_config()?;
    enforce_not_quarantined_identifiers(
        &request.session_id,
        &[request.member_identifier],
        &guard.quarantined_operator_identifiers,
        auto_quarantine_config.as_ref(),
    )?;

    // Signing-policy firewall (frozen spec section 5: Open "checks
    // policy gates"). When the firewall is enabled, the message must be
    // bound to a prior policy-checked build_taproot_tx for this
    // session, exactly as the coarse start_sign_round path enforces it
    // - otherwise a caller holding a key package could open an
    // interactive session on a fresh session_id and sign an arbitrary
    // message. A session with no policy-checked tx fails closed here.
    enforce_signing_message_binding_to_policy_checked_build_tx(
        &request.session_id,
        &request.message_hex,
        guard
            .sessions
            .get(&request.session_id)
            .and_then(|session| session.tx_result.as_ref()),
    )?;

    // Decide everything from a read-only view BEFORE inserting anything,
    // so the reject paths (consumed marker, conflict, capacity) never
    // leave an empty SessionState behind. Returns: whether the attempt
    // is already consumed, the disposition of any live attempt under
    // this exact attempt_id (Some(true)=idempotent, Some(false)=
    // conflicting fingerprint, None=no matching live attempt), and
    // whether a live interactive attempt is being replaced.
    let (already_consumed, matching_attempt_idempotent, replacing) = {
        let existing = guard.sessions.get(&request.session_id);
        let already_consumed = existing.is_some_and(|session| {
            session
                .consumed_interactive_attempt_markers
                .contains(&attempt_id)
        });
        let matching_attempt_idempotent = existing
            .and_then(|session| session.interactive_signing.as_ref())
            .filter(|interactive| interactive.attempt_context.attempt_id == attempt_id)
            .map(|interactive| interactive.open_request_fingerprint == request_fingerprint);
        let replacing = existing.is_some_and(|session| session.interactive_signing.is_some());
        (already_consumed, matching_attempt_idempotent, replacing)
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
        // None: no live attempt under this attempt_id. If a DIFFERENT
        // attempt is live it is implicitly aborted below - the retry
        // loop has moved on and a stuck prior attempt must not strand
        // its nonces.
        None => {}
    }

    // Capacity counts every live interactive session. When replacing,
    // this session already holds one of those slots, so the cap does
    // not apply; when not replacing, a new slot is being taken.
    if !replacing {
        let live_interactive_sessions = guard
            .sessions
            .values()
            .filter(|session| session.interactive_signing.is_some())
            .count();
        if live_interactive_sessions >= max_live_interactive_sessions_limit() {
            return Err(EngineError::Internal(format!(
                "live interactive session count [{live_interactive_sessions}] reached max [{}]; \
                 abort idle sessions or increase {}",
                max_live_interactive_sessions_limit(),
                TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV
            )));
        }
    }

    let session = guard
        .sessions
        .entry(request.session_id.clone())
        .or_default();

    if let Some(mut replaced) = session.interactive_signing.take() {
        zeroize_interactive_round1(&mut replaced);
    }

    session.interactive_signing = Some(InteractiveSigningState {
        open_request_fingerprint: request_fingerprint,
        attempt_context: request.attempt_context,
        canonical_included_participants,
        member_identifier: request.member_identifier,
        threshold: request.threshold,
        message_bytes: Zeroizing::new(message_bytes),
        taproot_merkle_root,
        key_package,
        opened_at_unix: now_unix(),
        round1: None,
    });

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

    if session
        .consumed_interactive_attempt_markers
        .contains(&attempt_id)
    {
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

    let session = guard.sessions.get_mut(&request.session_id).ok_or_else(|| {
        EngineError::SessionNotFound {
            session_id: request.session_id.clone(),
        }
    })?;

    if session
        .consumed_interactive_attempt_markers
        .contains(&attempt_id)
    {
        return Err(EngineError::ConsumedNonceReplay {
            session_id: request.session_id.clone(),
            attempt_id,
        });
    }

    ensure_consumed_registry_insert_capacity(
        &session.consumed_interactive_attempt_markers,
        &attempt_id,
        "consumed_interactive_attempt_markers",
        &request.session_id,
    )?;

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

    // Consumption-before-release: the durable marker is persisted
    // BEFORE the share is computed and returned. If persistence fails,
    // the marker is rolled back and the nonces remain live - no share
    // has left the engine. If share computation fails after the marker
    // persisted, the attempt is dead (fail closed): the marker stays,
    // the nonces are destroyed, and no share was released.
    session
        .consumed_interactive_attempt_markers
        .insert(attempt_id.clone());
    if let Err(persist_error) = persist_engine_state_to_storage(&guard) {
        let session = guard
            .sessions
            .get_mut(&request.session_id)
            .expect("session existed under the held engine lock");
        session
            .consumed_interactive_attempt_markers
            .remove(&attempt_id);
        return Err(persist_error);
    }

    let session = guard
        .sessions
        .get_mut(&request.session_id)
        .expect("session existed under the held engine lock");
    let interactive = session
        .interactive_signing
        .as_mut()
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

    // Round2 is terminal for this member's participation in the
    // attempt: the marker is durable and the nonces are gone, so free
    // the live session state now rather than letting it (and its
    // resident key package + message) linger until the TTL sweep. This
    // also returns the live-session capacity slot immediately. Done on
    // both the success and share-computation-failure paths: the
    // attempt is consumed either way, and the durable marker carries
    // all further replay protection.
    session.interactive_signing = None;

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
        Some(session) => match session.interactive_signing.as_ref() {
            Some(interactive)
                if attempt_id_filter.is_none()
                    || attempt_id_filter.as_deref()
                        == Some(interactive.attempt_context.attempt_id.as_str()) =>
            {
                let mut removed = session
                    .interactive_signing
                    .take()
                    .expect("interactive state existed under the held engine lock");
                zeroize_interactive_round1(&mut removed);
                true
            }
            _ => false,
        },
        None => false,
    };

    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_session_abort_success_total = telemetry
            .interactive_session_abort_success_total
            .saturating_add(1);
    });

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
    let interactive =
        session
            .interactive_signing
            .as_mut()
            .ok_or_else(|| EngineError::SessionNotFound {
                session_id: format!("{session_id} (no live interactive attempt)"),
            })?;

    if interactive.attempt_context.attempt_id != attempt_id {
        return Err(EngineError::Validation(format!(
            "attempt_id [{attempt_id}] does not match the live interactive attempt [{}]",
            interactive.attempt_context.attempt_id
        )));
    }

    if interactive.member_identifier != member_identifier {
        return Err(EngineError::Validation(
            "member_identifier does not match the open interactive session".to_string(),
        ));
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

// Canonical key form for an attempt_id at the round entry points,
// matching canonicalize_attempt_context_for_fingerprint (which
// lowercases attempt_id). The wire accepts attempt_id case-
// insensitively, so the marker registry and live-state lookups must
// operate on this form to be replay-safe.
fn canonical_attempt_id(attempt_id: &str) -> String {
    attempt_id.to_ascii_lowercase()
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
    for session in engine_state.sessions.values_mut() {
        let expired = session
            .interactive_signing
            .as_ref()
            .is_some_and(|interactive| {
                now.saturating_sub(interactive.opened_at_unix) > ttl_seconds
            });
        if expired {
            if let Some(mut removed) = session.interactive_signing.take() {
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
    // The serialized request transiently contains key_package_hex;
    // wipe the buffer once the fingerprint digest is taken.
    let mut canonical = serde_json::to_vec(request).map_err(|e| {
        EngineError::Internal(format!(
            "failed to serialize InteractiveSessionOpen request for fingerprint: {e}"
        ))
    })?;
    let fingerprint = hash_hex(&canonical);
    canonical.zeroize();
    Ok(fingerprint)
}
