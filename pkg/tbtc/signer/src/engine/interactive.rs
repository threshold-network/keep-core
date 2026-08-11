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
// (consumption-before-release).
//
// Be precise about what those markers buy, because it is easy to credit
// them with the wrong guarantee. A second share under the SAME nonces is
// impossible regardless of them: nonces live only in memory, are
// zeroized at first use, and are never restored on load, so no restart
// and no durable-state rollback can hand a process a usable nonce. What
// the markers give is at-most-once re-execution of an ATTEMPT - a
// consumed attempt_id cannot be re-opened to mint fresh nonces against
// the same coordinator-visible attempt - and, once externally
// acknowledged, evidence that the release happened.
//
// Attempt contexts are strict-mode only: there is no legacy-shape
// fallback on this path. All entry points are idempotent or fail
// closed; none of them can be made to release more than one signature
// share per nonce pair.

use super::*;

#[cfg(test)]
static INTERACTIVE_CLOCK_OFFSET_SECONDS: std::sync::atomic::AtomicU64 =
    std::sync::atomic::AtomicU64::new(0);

pub(crate) fn interactive_now() -> Instant {
    #[cfg(test)]
    {
        let offset_seconds =
            INTERACTIVE_CLOCK_OFFSET_SECONDS.load(std::sync::atomic::Ordering::SeqCst);
        Instant::now()
            .checked_add(Duration::from_secs(offset_seconds))
            .expect("interactive test clock offset fits the monotonic clock")
    }
    #[cfg(not(test))]
    {
        Instant::now()
    }
}

#[cfg(test)]
pub(crate) fn advance_interactive_clock_for_tests(seconds: u64) {
    INTERACTIVE_CLOCK_OFFSET_SECONDS
        .fetch_update(
            std::sync::atomic::Ordering::SeqCst,
            std::sync::atomic::Ordering::SeqCst,
            |current| current.checked_add(seconds),
        )
        .expect("interactive test clock offset does not overflow");
}

#[cfg(test)]
pub(crate) fn reset_interactive_clock_for_tests() {
    INTERACTIVE_CLOCK_OFFSET_SECONDS.store(0, std::sync::atomic::Ordering::SeqCst);
}

#[cfg(test)]
static INTERACTIVE_AGGREGATE_HOLD_AFTER_UNLOCK: std::sync::atomic::AtomicBool =
    std::sync::atomic::AtomicBool::new(false);
#[cfg(test)]
static INTERACTIVE_AGGREGATE_UNLOCK_HELD: std::sync::atomic::AtomicBool =
    std::sync::atomic::AtomicBool::new(false);
#[cfg(test)]
static INTERACTIVE_AGGREGATE_RELEASE_AFTER_UNLOCK: std::sync::atomic::AtomicBool =
    std::sync::atomic::AtomicBool::new(true);

#[cfg(test)]
pub(crate) fn arm_interactive_aggregate_unlock_hold_for_tests() {
    use std::sync::atomic::Ordering;
    INTERACTIVE_AGGREGATE_UNLOCK_HELD.store(false, Ordering::SeqCst);
    INTERACTIVE_AGGREGATE_RELEASE_AFTER_UNLOCK.store(false, Ordering::SeqCst);
    INTERACTIVE_AGGREGATE_HOLD_AFTER_UNLOCK.store(true, Ordering::SeqCst);
}

#[cfg(test)]
pub(crate) fn interactive_aggregate_unlock_held_for_tests() -> bool {
    INTERACTIVE_AGGREGATE_UNLOCK_HELD.load(std::sync::atomic::Ordering::SeqCst)
}

#[cfg(test)]
pub(crate) fn release_interactive_aggregate_unlock_for_tests() {
    INTERACTIVE_AGGREGATE_RELEASE_AFTER_UNLOCK.store(true, std::sync::atomic::Ordering::SeqCst);
}

#[cfg(test)]
fn maybe_hold_interactive_aggregate_after_unlock_for_tests() {
    use std::sync::atomic::Ordering;
    if INTERACTIVE_AGGREGATE_HOLD_AFTER_UNLOCK.swap(false, Ordering::SeqCst) {
        INTERACTIVE_AGGREGATE_UNLOCK_HELD.store(true, Ordering::SeqCst);
        while !INTERACTIVE_AGGREGATE_RELEASE_AFTER_UNLOCK.load(Ordering::SeqCst) {
            std::thread::yield_now();
        }
    }
}

// Multi-seat: a session's interactive consumed-nonce markers are keyed per
// (attempt_id, member_identifier), so independent local seats can each consume
// their own nonces for the same attempt without colliding. The marker is written
// BEFORE a share leaves the engine (consumption-before-release). Legacy bare
// attempt_id markers (written by the pre-multi-seat single-member engine, and
// possibly reloaded from durable state) are honored FAIL-CLOSED on read: a bare
// marker means the attempt is consumed for every member.
pub(crate) fn interactive_consumed_marker(attempt_id: &str, member_identifier: u16) -> String {
    // Keep the schema-1 wire representation understood by the immediately
    // previous signer. A binary rollback must continue to see the attempt as
    // consumed and fail closed rather than releasing a second share.
    format!("m{member_identifier}@{attempt_id}")
}

pub(crate) fn interactive_attempt_consumed(
    markers: &HashSet<String>,
    attempt_id: &str,
    member_identifier: u16,
) -> bool {
    markers.contains(&interactive_consumed_marker(attempt_id, member_identifier))
        // Transitional marker written by an unreleased intermediate build.
        // Continue honoring it fail-closed when upgrading that state.
        || markers.contains(&format!("m{member_identifier}@{attempt_id}@v2"))
        || markers.contains(attempt_id)
}

// Fixed-size, exact authorization written atomically with the Round2 consumed
// marker. Hash canonical package bytes rather than caller-provided hex so
// alternate encodings cannot create distinct persistent records.
pub(crate) fn interactive_aggregate_authorization_marker(
    attempt_id: &str,
    signing_package: &frost::SigningPackage,
    taproot_merkle_root: Option<&[u8; 32]>,
) -> Result<String, EngineError> {
    let signing_package_bytes = signing_package.serialize().map_err(|error| {
        EngineError::Internal(format!(
            "failed to serialize signing package for Aggregate authorization: {error}"
        ))
    })?;
    let mut hasher = Sha256::new();
    hasher.update(b"tbtc-signer/interactive-aggregate-authorization/v1");
    hasher.update((attempt_id.len() as u64).to_be_bytes());
    hasher.update(attempt_id.as_bytes());
    match taproot_merkle_root {
        Some(root) => {
            hasher.update([1]);
            hasher.update(root);
        }
        None => hasher.update([0]),
    }
    hasher.update((signing_package_bytes.len() as u64).to_be_bytes());
    hasher.update(&signing_package_bytes);
    Ok(hex::encode(hasher.finalize()))
}

// Session-scoped identity for a successfully aggregated canonical package.
// It deliberately excludes attempt_id: the inner FROST package does not carry
// one, so a non-signing coordinator can validate only the live coordinator
// context plus package shape. Persisting this identity prevents the same valid
// package/share set from being replayed under fresh canonical attempts to fill
// the bounded completion registry.
pub(crate) fn interactive_aggregate_package_completion_marker(
    signing_package: &frost::SigningPackage,
    taproot_merkle_root: Option<&[u8; 32]>,
) -> Result<String, EngineError> {
    let signing_package_bytes = signing_package.serialize().map_err(|error| {
        EngineError::Internal(format!(
            "failed to serialize signing package for Aggregate completion: {error}"
        ))
    })?;
    let mut hasher = Sha256::new();
    hasher.update(b"tbtc-signer/interactive-aggregate-package-completion/v1");
    match taproot_merkle_root {
        Some(root) => {
            hasher.update([1]);
            hasher.update(root);
        }
        None => hasher.update([0]),
    }
    hasher.update((signing_package_bytes.len() as u64).to_be_bytes());
    hasher.update(&signing_package_bytes);
    Ok(hex::encode(hasher.finalize()))
}

// A signer proves live authorization with the exact Round1 commitment included
// in the package. The elected coordinator is also allowed to aggregate a strict
// first-t package that omits it: its validated live attempt authorizes the
// common message/threshold/included-subset shape, while the package-completion
// marker above prevents cross-attempt reuse of that otherwise attempt-less
// FROST package. An omitted non-coordinator never receives this fallback.
fn interactive_aggregate_has_live_authorization(
    session: &SessionState,
    attempt_id: &str,
    signing_package: &frost::SigningPackage,
    taproot_merkle_root: Option<&[u8; 32]>,
) -> Result<bool, EngineError> {
    for interactive in session.interactive_signing.values().filter(|interactive| {
        interactive.attempt_context.attempt_id == attempt_id
            && interactive.taproot_merkle_root.as_ref() == taproot_merkle_root
            && interactive.round1.is_some()
    }) {
        match verify_round2_signing_package(interactive, signing_package) {
            Ok(()) => return Ok(true),
            Err(error @ EngineError::Internal(_)) => return Err(error),
            Err(_) => {
                let own_identifier =
                    participant_identifier_to_frost_identifier(interactive.member_identifier)?;
                let is_omitted_coordinator = interactive.member_identifier
                    == interactive.attempt_context.coordinator_identifier
                    && !signing_package
                        .signing_commitments()
                        .contains_key(&own_identifier);
                if is_omitted_coordinator {
                    match verify_interactive_signing_package_context(interactive, signing_package) {
                        Ok(()) => return Ok(true),
                        Err(error @ EngineError::Internal(_)) => return Err(error),
                        Err(_) => {}
                    }
                }
            }
        }
    }
    Ok(false)
}

// The aggregate completion marker binds attempt_id to the AGGREGATED message digest,
// so the durable "this attempt is final" record cannot be set for one attempt id via
// a valid aggregate over a DIFFERENT message - which would otherwise let a replayed
// aggregate preempt an unrelated live attempt's Round2 (the Round2 completion gate).
// interactive_aggregate writes it from the package it aggregated; Round2 and the
// re-aggregate guard recompute it from the message they actually hold.
pub(crate) fn interactive_aggregated_marker(
    attempt_id: &str,
    message_digest_hex: &str,
    taproot_merkle_root: Option<&[u8; 32]>,
) -> String {
    // The signature differs per taproot tweak, so the completion is per (message,
    // root). "keypath" (a key-path / None spend) cannot collide with a 64-hex root.
    let root = taproot_merkle_root
        .map(hex::encode)
        .unwrap_or_else(|| "keypath".to_string());
    format!("{attempt_id}@{message_digest_hex}@{root}")
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
    taproot_merkle_root: Option<&[u8; 32]>,
) -> bool {
    markers.contains(&interactive_aggregated_marker(
        attempt_id,
        message_digest_hex,
        taproot_merkle_root,
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
    if let Some(InteractiveSigningIntent::Heartbeat { message_hex }) =
        request.signing_intent.as_mut()
    {
        // The intent is part of the Open fingerprint. Treat hex casing as a
        // wire-format detail so an otherwise identical retry is idempotent.
        *message_hex = message_hex.to_ascii_lowercase();
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

    let request_fingerprint = interactive_open_request_fingerprint(&request)?;
    let attempt_id = request.attempt_context.attempt_id.clone();

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    sweep_expired_interactive_state_durably(&mut guard)?;

    let auto_quarantine_config = load_auto_quarantine_config()?;

    // The session must already exist with completed DKG. Key material
    // lives in the engine's own DKG-populated state and is NEVER
    // supplied through the request, so no signing secret crosses the
    // FFI/host boundary (frozen spec section 4). Resolve the member's
    // key package, run the policy gates, and validate the strict
    // attempt context against the DKG threshold/key group - mirroring
    // the coarse start_sign_round - all under one immutable borrow,
    // then do the mutable install.
    // The DKG key material is a WALLET-level asset keyed by key_group, not by the
    // per-signing session_id: interactive signing runs under a fresh RoastSessionID
    // per message, while the wallet key lives under the session its DKG completed in.
    // Resolve that wallet session by key_group so ANY signing session can reach the
    // material (and the wallet-level policy gates below); the per-signing state
    // (consumed markers, live attempt, nonces) still lives under request.session_id,
    // and the attempt context is still validated against request.session_id so
    // coordinator/attempt derivation is unchanged. DkgNotReady now means "no wallet
    // key for this key_group" rather than "this exact session lacks DKG".
    let wallet_session_id =
        resolve_wallet_session_id(&guard, &request.session_id, &request.key_group).ok_or_else(
            || EngineError::DkgNotReady {
                session_id: request.session_id.clone(),
            },
        )?;
    let (key_package, canonical_included_participants) =
        {
            let session = guard.sessions.get(&wallet_session_id).ok_or_else(|| {
                EngineError::SessionNotFound {
                    session_id: wallet_session_id.clone(),
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
            let dkg_key_packages = session.dkg_key_packages.as_ref().ok_or_else(|| {
                EngineError::Internal("missing DKG key package cache".to_string())
            })?;
            // The public key package carries a verifying share for EVERY DKG
            // participant, so it is the authoritative participant set. A distributed
            // DKG node holds only its OWN secret key package (dkg_key_packages has a
            // single entry), so the included-participants membership check below must
            // use the public package, not dkg_key_packages.
            let dkg_public_key_package =
                session.dkg_public_key_package.as_ref().ok_or_else(|| {
                    EngineError::Internal("missing DKG public key package".to_string())
                })?;
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
                guard
                    .sessions
                    .get(&request.session_id)
                    .and_then(|signing_session| signing_session.emergency_rekey_event.as_ref()),
                session.emergency_rekey_event.as_ref(),
                session.finalize_request_fingerprint.is_some(),
                guard
                    .sessions
                    .get(&request.session_id)
                    .and_then(|signing_session| signing_session.tx_result.as_ref()),
                taproot_merkle_root.as_ref(),
                request.signing_intent.as_ref(),
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
            // context that is not a genuine DKG subset. Checked against the public
            // key package (the full participant set) so it holds for a distributed
            // DKG node, which caches only its own secret key package.
            for participant in &canonical_included_participants {
                let participant_frost_identifier =
                    participant_identifier_to_frost_identifier(*participant)?;
                if !dkg_public_key_package
                    .verifying_shares()
                    .contains_key(&participant_frost_identifier)
                {
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
    let (already_consumed, matching_attempt_idempotent, live_attempt) =
        match guard.sessions.get(&request.session_id) {
            Some(session) => {
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
            }
            // A fresh per-signing session (a distinct RoastSessionID that is not the
            // wallet's DKG session) does not exist yet: no consumed markers, no live
            // attempt. It is created at the install below.
            None => (false, None, None),
        };

    if already_consumed {
        return Err(EngineError::ConsumedNonceReplay {
            session_id: request.session_id.clone(),
            attempt_id,
        });
    }

    match matching_attempt_idempotent {
        Some(true) => {
            let interactive = guard
                .sessions
                .get_mut(&request.session_id)
                .expect("idempotent Open session exists")
                .interactive_signing
                .get_mut(&member_identifier)
                .expect("idempotent Open member exists");
            interactive.last_activity_at = interactive_now();
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

    // A per-signing session is keyed by RoastSessionID (message/root/start-block), NOT
    // by key_group, so two DIFFERENT wallets signing the same digest at the same block
    // on a node that holds members of both could collide on one session id. A session
    // belongs to exactly ONE wallet key for its lifetime: reject an Open whose key_group
    // differs from the session's ESTABLISHED one - its DKG key group when it is a
    // co-located DKG session, else the key group bound by a prior Open. Rejecting
    // regardless of live members keeps bound_key_group and dkg_result mutually
    // consistent so Round2/Aggregate/verify_share always resolve the right wallet. This
    // closes both (a) the rebind window that outlived a member's Round2 (the live-entry
    // set is empty in the consumed-but-unaggregated gap) and (b) binding through another
    // wallet's idle DKG session (where dkg_result would otherwise win over bound_key_group
    // and sign B's share against A's material, bypassing B's rekey/finalization gates).
    if let Some(existing) = guard.sessions.get(&request.session_id) {
        let established = existing
            .dkg_result
            .as_ref()
            .map(|dkg| dkg.key_group.as_str())
            .or(existing.bound_key_group.as_deref());
        if let Some(established) = established {
            if established != request.key_group {
                return Err(EngineError::SessionConflict {
                    session_id: request.session_id.clone(),
                });
            }
        }
    }

    // Admission/reactivation is fallible at active capacity, or when every
    // retired slot at the shared total bound is protected. Preflight it before
    // charging the wallet's heartbeat budget; the engine lock prevents the
    // registry from changing before the actual install below.
    ensure_interactive_session_admission_capacity(&guard, &request.session_id)?;

    // A BuildTaprootTx-backed signing shell is persisted before its first Open.
    // Make the first wallet binding durable before reporting Open success, or a
    // crash would reload that old unbound shell as an active non-retirable entry.
    // A co-located DKG session already has a durable wallet role and needs no
    // additional write here; a previously bound per-message session likewise
    // reuses its durable identity.
    let persists_new_per_message_binding = match guard.sessions.get(&request.session_id) {
        Some(session) => session.dkg_result.is_none() && session.bound_key_group.is_none(),
        None => true,
    };
    let resolved_binding_state_key = if persists_new_per_message_binding {
        Some(state_encryption_key_material()?)
    } else {
        None
    };

    // A typed heartbeat never passes through BuildTaprootTx, so charge its own
    // per-wallet policy budget only after all validation and the exact-retry
    // return above. If the new binding cannot be persisted before replacement,
    // the limiter snapshot is restored along with the session registry, so a
    // durability failure cannot burn the caller's only legitimate token.
    let is_heartbeat = matches!(
        request.signing_intent.as_ref(),
        Some(InteractiveSigningIntent::Heartbeat { .. })
    );
    let previous_heartbeat_rate_limiter = if is_heartbeat {
        let wallet_session = guard.sessions.get_mut(&wallet_session_id).ok_or_else(|| {
            EngineError::SessionNotFound {
                session_id: wallet_session_id.clone(),
            }
        })?;
        let previous = wallet_session.heartbeat_rate_limiter.clone();
        enforce_heartbeat_rate_limit(
            &request.session_id,
            &mut wallet_session.heartbeat_rate_limiter,
        )?;
        Some(previous)
    } else {
        None
    };

    // Create (or reactivate) the per-signing session only after every other
    // fallible gate. A rejected heartbeat must not pull an idle tombstone back
    // into the active budget. DKG material remains solely in the wallet session.
    let session_existed = guard.sessions.contains_key(&request.session_id);
    let (previous_bound_key_group, previous_retired_at) = guard
        .sessions
        .get(&request.session_id)
        .map(|session| {
            (
                session.bound_key_group.clone(),
                session.retired_interactive_at_unix,
            )
        })
        .unwrap_or((None, None));
    reactivate_retired_per_message_session(&mut guard, &request.session_id)?;
    let compacted_retired_sessions =
        ensure_session_insert_capacity(&mut guard, &request.session_id)?;

    {
        let session = guard
            .sessions
            .entry(request.session_id.clone())
            .or_insert_with(SessionState::default);
        // Bind this signing session to the wallet key it signs for, so Round2 and
        // Aggregate resolve the same wallet material by key_group.
        session.bound_key_group = Some(request.key_group.clone());
    }

    if let Some(resolved_state_key) = resolved_binding_state_key.as_ref() {
        if let Err(persist_error) =
            persist_engine_state_to_storage_with_key(&guard, resolved_state_key)
        {
            let state_file_replaced = persist_error.state_file_replaced();
            let persist_error = persist_error.into_engine_error();

            if let Some(previous) = previous_heartbeat_rate_limiter {
                guard
                    .sessions
                    .get_mut(&wallet_session_id)
                    .expect("wallet session existed while rolling back Open")
                    .heartbeat_rate_limiter = previous;
            }

            if state_file_replaced {
                let session = guard
                    .sessions
                    .get_mut(&request.session_id)
                    .expect("Open session existed after state-file replacement");
                if session.interactive_signing.is_empty()
                    && per_message_interactive_session(session)
                {
                    session.retired_interactive_at_unix = Some(now_unix().max(1));
                }
                mark_persistence_pending(PersistencePendingOperation::InteractiveState {
                    session_id: request.session_id.clone(),
                });
            } else {
                if session_existed {
                    let session = guard
                        .sessions
                        .get_mut(&request.session_id)
                        .expect("Open session existed while rolling back binding");
                    session.bound_key_group = previous_bound_key_group;
                    session.retired_interactive_at_unix = previous_retired_at;
                } else {
                    guard.sessions.remove(&request.session_id);
                }
                restore_compacted_retired_sessions(&mut guard, compacted_retired_sessions);
            }
            return Err(persist_error);
        }
    }

    let session = guard
        .sessions
        .get_mut(&request.session_id)
        .expect("Open session existed after binding persistence");
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
            signing_intent: request.signing_intent,
            key_package,
            last_activity_at: interactive_now(),
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
    let mut latency_guard =
        HardeningOperationLatencyGuard::success_only(HardeningOperation::InteractiveRound1);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    // The live state and markers are keyed on the canonical (lowercase)
    // attempt_id; the wire form may differ in casing.
    let attempt_id = canonical_attempt_id(&request.attempt_id);

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    sweep_expired_interactive_state_durably(&mut guard)?;

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

    if let Some(commitments_hex) = interactive
        .round1
        .as_ref()
        .map(|round1| round1.commitments_hex.clone())
    {
        // Idempotent until consumed: the commitments are public and
        // re-sending them is safe; the nonces never leave.
        interactive.last_activity_at = interactive_now();
        return Ok(InteractiveRound1Result { commitments_hex });
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
    interactive.last_activity_at = interactive_now();

    latency_guard.mark_success();
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
    let mut latency_guard =
        HardeningOperationLatencyGuard::success_only(HardeningOperation::InteractiveRound2);
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
    let consumed_marker = interactive_consumed_marker(&attempt_id, request.member_identifier);

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    sweep_expired_interactive_state_durably(&mut guard)?;

    // An earlier marker write may have replaced the state file but failed its
    // directory sync. Flush that fail-closed marker before consulting the replay
    // gate; after a successful write, the marker below rejects the retry.
    if interactive_round2_persistence_pending(&request.session_id, &consumed_marker) {
        persist_engine_state_to_storage(&guard)
            .map_err(PersistEngineStateError::into_engine_error)?;
    }

    // Quarantine inputs must be read before the session is borrowed
    // mutably from the same guard below.
    let auto_quarantine_config = load_auto_quarantine_config()?;
    let quarantined_operator_identifiers = guard.quarantined_operator_identifiers.clone();

    // Finalization is a WALLET-level gate resolved from the DKG session by key_group.
    // Emergency rekey is normally wallet-level too, but an event triggered after
    // BuildTaprootTx and before Open binds the signing session must still be read from
    // that signing session. The policy-checked transaction is likewise per-signing-flow
    // state. Clone both signing-session values before the mutable borrow below.
    let (signing_tx_result, signing_emergency_rekey) = guard
        .sessions
        .get(&request.session_id)
        .map(|session| {
            (
                session.tx_result.clone(),
                session.emergency_rekey_event.clone(),
            )
        })
        .unwrap_or((None, None));
    let (wallet_emergency_rekey, wallet_finalized) = {
        let bound_key_group = guard.sessions.get(&request.session_id).and_then(|session| {
            session
                .dkg_result
                .as_ref()
                .map(|dkg| dkg.key_group.clone())
                .or_else(|| session.bound_key_group.clone())
        });
        match bound_key_group
            .and_then(|key_group| {
                resolve_wallet_session_id(&guard, &request.session_id, &key_group)
            })
            .and_then(|wallet_session_id| guard.sessions.get(&wallet_session_id))
        {
            Some(wallet) => (
                wallet.emergency_rekey_event.clone(),
                wallet.finalize_request_fingerprint.is_some(),
            ),
            None => (None, false),
        }
    };

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
    let member_attempt_finalization = session
        .interactive_signing
        .get(&request.member_identifier)
        .filter(|entry| entry.attempt_context.attempt_id == attempt_id)
        .map(|entry| (hash_hex(&entry.message_bytes), entry.taproot_merkle_root));
    let attempt_finalized =
        member_attempt_finalization
            .as_ref()
            .is_some_and(|(digest, taproot_merkle_root)| {
                interactive_attempt_aggregated(
                    &session.aggregated_interactive_attempt_markers,
                    &attempt_id,
                    digest,
                    taproot_merkle_root.as_ref(),
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
            signing_emergency_rekey.as_ref(),
            wallet_emergency_rekey.as_ref(),
            wallet_finalized,
            signing_tx_result.as_ref(),
            interactive.taproot_merkle_root.as_ref(),
            interactive.signing_intent.as_ref(),
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
    let aggregate_authorization_marker = interactive_aggregate_authorization_marker(
        &attempt_id,
        &signing_package,
        interactive.taproot_merkle_root.as_ref(),
    )?;
    ensure_consumed_registry_insert_capacity(
        &session.authorized_interactive_aggregate_markers,
        &aggregate_authorization_marker,
        "authorized_interactive_aggregate_markers",
        &request.session_id,
    )?;

    // Consumption-before-release: the durable marker is persisted BEFORE the
    // share is computed and returned. A failure before state-file replacement
    // rolls the marker back and leaves the nonces live. A failure after replacement
    // keeps the marker fail-closed, destroys the nonces, and records a pending
    // retry that must re-persist before the replay gate runs. No failure path
    // releases a share. If share computation itself later fails, the already-
    // durable marker likewise stays and the nonces are destroyed.
    // Resolve the state-encryption key under the held ENGINE_STATE guard, in the
    // same serialized order as the write, and BEFORE inserting the marker.
    // Resolving under the guard makes key selection match the write order, so the
    // last writer encrypts with the then-current key; a key resolved before the
    // lock could be stale and lose a rotation race, leaving the persisted envelope
    // tagged with an old key id that decode rejects on restart. Resolving before
    // the marker also keeps a key-provider outage failing the attempt cleanly (no
    // marker written) rather than escaping the rollback below via `?`.
    let resolved_state_key = match state_encryption_key_material() {
        Ok(key) => key,
        Err(error) => {
            // Key-provider commands can run long enough to cross the TTL. The
            // validated request still leaves this nonce handle retryable, so
            // measure its inactivity from failure completion, not command start.
            session
                .interactive_signing
                .get_mut(&request.member_identifier)
                .expect("validated Round2 interactive state remains retryable")
                .last_activity_at = interactive_now();
            return Err(error);
        }
    };
    session
        .consumed_interactive_attempt_markers
        .insert(consumed_marker.clone());
    session
        .authorized_interactive_aggregate_markers
        .insert(aggregate_authorization_marker.clone());
    let retires_session =
        session.interactive_signing.len() == 1 && per_message_interactive_session(session);
    let compacted_retired_sessions = if retires_session {
        session.retired_interactive_at_unix = Some(now_unix().max(1));
        compact_retired_per_message_sessions(&mut guard, Some(&request.session_id))
    } else {
        Vec::new()
    };
    if let Err(persist_error) =
        persist_engine_state_to_storage_with_key(&guard, &resolved_state_key)
    {
        let state_file_replaced = persist_error.state_file_replaced();
        let persist_error = persist_error.into_engine_error();
        let session = guard
            .sessions
            .get_mut(&request.session_id)
            .expect("session existed under the held engine lock");
        if state_file_replaced {
            mark_persistence_pending(PersistencePendingOperation::InteractiveRound2 {
                session_id: request.session_id.clone(),
                consumed_marker: consumed_marker.clone(),
            });
            if let Some(mut removed) = session
                .interactive_signing
                .remove(&request.member_identifier)
            {
                zeroize_interactive_round1(&mut removed);
            }
        } else {
            session
                .consumed_interactive_attempt_markers
                .remove(&consumed_marker);
            session
                .authorized_interactive_aggregate_markers
                .remove(&aggregate_authorization_marker);
            if retires_session {
                session.retired_interactive_at_unix = None;
            }
            // A pre-replacement failure deliberately leaves the nonce handle
            // retryable. Persistence may itself be slow, so restart inactivity
            // at failure completion before releasing the engine lock.
            session
                .interactive_signing
                .get_mut(&request.member_identifier)
                .expect("pre-replacement Round2 failure leaves interactive state")
                .last_activity_at = interactive_now();
            restore_compacted_retired_sessions(&mut guard, compacted_retired_sessions);
        }
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

    latency_guard.mark_success();
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
    let mut latency_guard =
        HardeningOperationLatencyGuard::success_only(HardeningOperation::InteractiveAggregate);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;
    let attempt_id = canonical_aggregate_attempt_id(&request.attempt_id)?;

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
    let signature_shares =
        decode_signature_share_map("InteractiveAggregate", &request.signature_shares)?;
    let mut taproot_merkle_root_hex = request.taproot_merkle_root_hex.clone();
    let taproot_merkle_root = canonicalize_taproot_merkle_root_hex(&mut taproot_merkle_root_hex)?;
    // The completion marker binds attempt_id to THIS aggregated message digest AND the
    // canonical taproot root, so a valid aggregate over a different message or root
    // cannot finalize this attempt id (and so cannot, via the Round2 completion gate,
    // preempt an unrelated live attempt or root - the signature differs per tweak).
    let aggregated_message_digest = hash_hex(signing_package.message().as_slice());
    let aggregated_marker = interactive_aggregated_marker(
        &attempt_id,
        &aggregated_message_digest,
        taproot_merkle_root.as_ref(),
    );
    let aggregate_authorization_marker = interactive_aggregate_authorization_marker(
        &attempt_id,
        &signing_package,
        taproot_merkle_root.as_ref(),
    )?;
    let aggregate_package_completion_marker = interactive_aggregate_package_completion_marker(
        &signing_package,
        taproot_merkle_root.as_ref(),
    )?;

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    // Sweep first so a failure while repairing any prior completion marker can
    // never postpone destruction of newly expired nonce handles.
    sweep_expired_interactive_state_durably(&mut guard)?;
    // A prior completion-marker write may have replaced the state file but failed
    // its directory sync. Re-persist that fail-closed marker before the completed
    // attempt check so a retry repairs durability and is then rejected normally.
    if interactive_aggregate_persistence_pending(&request.session_id, &aggregated_marker) {
        persist_engine_state_to_storage(&guard)
            .map_err(PersistEngineStateError::into_engine_error)?;
    }
    // Resolve the group's public key package (the verifying shares used
    // to check each contribution) from the session's own DKG state, not
    // the request - consistent with the no-secret-on-the-FFI discipline
    // and so a caller cannot substitute verifying material. The session
    // must exist with completed DKG.
    let (public_key_package, aggregate_eviction_pin) = {
        // The completion marker is per-signing-session state; read it - and the wallet
        // key_group this session serves - from request.session_id.
        let (key_group, aggregate_eviction_pin) = {
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
                taproot_merkle_root.as_ref(),
            ) {
                return Err(EngineError::InteractiveAttemptAlreadyAggregated {
                    session_id: request.session_id.clone(),
                    attempt_id,
                });
            }
            let authorized = session
                .authorized_interactive_aggregate_markers
                .contains(&aggregate_authorization_marker)
                || interactive_aggregate_has_live_authorization(
                    session,
                    &attempt_id,
                    &signing_package,
                    taproot_merkle_root.as_ref(),
                )?;
            if !authorized {
                return Err(EngineError::Validation(format!(
                    "InteractiveAggregate: package is not authorized for attempt_id [{attempt_id}] in session [{}]",
                    request.session_id
                )));
            }
            if session
                .authorized_interactive_aggregate_markers
                .contains(&aggregate_package_completion_marker)
            {
                return Err(EngineError::Validation(format!(
                    "InteractiveAggregate: signing package was already aggregated in session [{}]",
                    request.session_id
                )));
            }
            // The wallet key this signing session serves: its own DKG (co-located) or
            // the key_group bound at Open (distinct per-signing RoastSessionID).
            let key_group = session
                .dkg_result
                .as_ref()
                .map(|dkg| dkg.key_group.clone())
                .or_else(|| session.bound_key_group.clone())
                .ok_or_else(|| EngineError::DkgNotReady {
                    session_id: request.session_id.clone(),
                })?;
            (key_group, Arc::clone(&session.aggregate_eviction_pin))
        };
        // The group's public key package (the verifying shares used to check each
        // contribution) is a WALLET-level asset resolved by key_group, so a per-signing
        // session can verify shares. Read from the engine's own DKG state, not the
        // request, so a caller cannot substitute verifying material.
        let wallet_session_id = resolve_wallet_session_id(&guard, &request.session_id, &key_group)
            .ok_or_else(|| EngineError::DkgNotReady {
                session_id: request.session_id.clone(),
            })?;
        let session =
            guard
                .sessions
                .get(&wallet_session_id)
                .ok_or_else(|| EngineError::SessionNotFound {
                    session_id: wallet_session_id.clone(),
                })?;
        let public_key_package = session
            .dkg_public_key_package
            .as_ref()
            .ok_or_else(|| {
                EngineError::Internal("missing DKG public key package cache".to_string())
            })?
            .clone();
        (public_key_package, aggregate_eviction_pin)
    };
    drop(guard);
    #[cfg(test)]
    maybe_hold_interactive_aggregate_after_unlock_for_tests();

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
    // completed first), insert it, and persist before reporting success. A
    // pre-replacement failure rolls the marker back; a post-replacement failure
    // retains it, destroys matching live nonces, and records a pending retry that
    // is flushed before the next completion-marker check.
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
        taproot_merkle_root.as_ref(),
    ) {
        return Err(EngineError::InteractiveAttemptAlreadyAggregated {
            session_id: request.session_id.clone(),
            attempt_id,
        });
    }
    let authorized = session
        .authorized_interactive_aggregate_markers
        .contains(&aggregate_authorization_marker)
        || interactive_aggregate_has_live_authorization(
            session,
            &attempt_id,
            &signing_package,
            taproot_merkle_root.as_ref(),
        )?;
    if !authorized {
        return Err(EngineError::Validation(format!(
            "InteractiveAggregate: package authorization changed before completion for attempt_id [{attempt_id}] in session [{}]",
            request.session_id
        )));
    }
    if session
        .authorized_interactive_aggregate_markers
        .contains(&aggregate_package_completion_marker)
    {
        return Err(EngineError::Validation(format!(
            "InteractiveAggregate: signing package was already aggregated in session [{}]",
            request.session_id
        )));
    }
    ensure_consumed_registry_insert_capacity(
        &session.aggregated_interactive_attempt_markers,
        &aggregated_marker,
        "aggregated_interactive_attempt_markers",
        &request.session_id,
    )?;
    // A Round2 authorization is no longer needed once its exact package has
    // completed, so replace it in-place with the package replay marker. A
    // coordinator omitted from the signing subset has no Round2 authorization
    // to replace and therefore consumes one normal bounded slot.
    let replaces_aggregate_authorization = session
        .authorized_interactive_aggregate_markers
        .contains(&aggregate_authorization_marker);
    if !replaces_aggregate_authorization {
        ensure_consumed_registry_insert_capacity(
            &session.authorized_interactive_aggregate_markers,
            &aggregate_package_completion_marker,
            "authorized_interactive_aggregate_markers",
            &request.session_id,
        )?;
    }
    // Resolve the state-encryption key under the held ENGINE_STATE guard, in the
    // same serialized order as the write, and BEFORE inserting the marker.
    // Resolving under the guard makes key selection match the write order, so the
    // last writer encrypts with the then-current key; a key resolved before the
    // lock could be stale and lose a rotation race, leaving the persisted envelope
    // tagged with an old key id that decode rejects on restart. Resolving before
    // the marker also keeps a key-provider outage failing the attempt cleanly (no
    // marker written) rather than escaping the rollback below via `?`.
    let resolved_state_key = state_encryption_key_material()?;
    session
        .aggregated_interactive_attempt_markers
        .insert(aggregated_marker.clone());
    let removed_aggregate_authorization = session
        .authorized_interactive_aggregate_markers
        .remove(&aggregate_authorization_marker);
    session
        .authorized_interactive_aggregate_markers
        .insert(aggregate_package_completion_marker.clone());
    if let Err(persist_error) =
        persist_engine_state_to_storage_with_key(&guard, &resolved_state_key)
    {
        let state_file_replaced = persist_error.state_file_replaced();
        let persist_error = persist_error.into_engine_error();
        let session = guard
            .sessions
            .get_mut(&request.session_id)
            .expect("session existed under the held engine lock");
        if state_file_replaced {
            remove_finalized_interactive_members(
                session,
                &attempt_id,
                &aggregated_message_digest,
                taproot_merkle_root.as_ref(),
            );
            // Stage retirement BEFORE registering the pending operation. Generic
            // retirement deliberately skips pending sessions to protect uncertain
            // markers from eviction; registering first would therefore strand this
            // now-idle shell as active. With retirement staged first, the pending
            // operation protects the tombstone and any later successful full-state
            // snapshot (the exact retry or an unrelated writer) durably covers both
            // the completion marker and retirement before clearing pending.
            if session.interactive_signing.is_empty() && per_message_interactive_session(session) {
                session.retired_interactive_at_unix = Some(now_unix().max(1));
            }
            mark_persistence_pending(PersistencePendingOperation::InteractiveAggregate {
                session_id: request.session_id.clone(),
                aggregated_marker: aggregated_marker.clone(),
            });
        } else {
            session
                .aggregated_interactive_attempt_markers
                .remove(&aggregated_marker);
            session
                .authorized_interactive_aggregate_markers
                .remove(&aggregate_package_completion_marker);
            if removed_aggregate_authorization {
                session
                    .authorized_interactive_aggregate_markers
                    .insert(aggregate_authorization_marker.clone());
            }
        }
        if state_file_replaced {
            retire_idle_per_message_sessions(&mut guard, Some(&request.session_id));
        }
        return Err(persist_error);
    }

    // The attempt is now final for (attempt_id, message, root). A LOCAL sibling seat
    // that opened/Round1'd this same attempt + root but is NOT in the signing subset
    // never calls Round2, so free those entries now - zeroizing their nonces and
    // returning their live-member slots - rather than leaving them resident until the
    // TTL sweep. The signers' own entries were already removed at their Round2; a
    // sibling on a DIFFERENT root is a distinct signing task and is left untouched.
    let session = guard
        .sessions
        .get_mut(&request.session_id)
        .expect("session existed under the held engine lock");
    remove_finalized_interactive_members(
        session,
        &attempt_id,
        &aggregated_message_digest,
        taproot_merkle_root.as_ref(),
    );
    retire_idle_per_message_sessions(&mut guard, Some(&request.session_id));
    drop(guard);
    // Keep the target unevictable through both lock sections and the durable
    // completion write. Error returns and unwinding release the clone by RAII.
    drop(aggregate_eviction_pin);

    latency_guard.mark_success();
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
    // post-expiry traffic is aborts for other sessions. The durable helper also
    // repairs a prior post-rename Abort snapshot before an idempotent retry can
    // return `aborted: false` without writing.
    sweep_expired_interactive_state_durably(&mut guard)?;

    let members_to_abort = match guard.sessions.get(&request.session_id) {
        Some(session) => {
            // Abort has no member parameter, so it is session-level over the map:
            // select every member entry matching the optional attempt filter.
            // Keep the nonce-bearing entries live until the durable retirement
            // snapshot has replaced the state file, so a pre-replacement failure
            // remains cleanly retryable.
            session
                .interactive_signing
                .iter()
                .filter(|(_, interactive)| {
                    attempt_id_filter.is_none()
                        || attempt_id_filter.as_deref()
                            == Some(interactive.attempt_context.attempt_id.as_str())
                })
                .map(|(member, _)| *member)
                .collect::<Vec<_>>()
        }
        None => Vec::new(),
    };

    if members_to_abort.is_empty() {
        return Ok(InteractiveSessionAbortResult {
            session_id: request.session_id,
            aborted: false,
        });
    }

    // Resolve the key before staging retirement. A key-provider outage must not
    // consume the live nonces or turn a retryable attempt into an inert shell.
    let resolved_state_key = state_encryption_key_material()?;
    let (retires_session, previous_retired_at) = {
        let session = guard
            .sessions
            .get_mut(&request.session_id)
            .expect("selected abort session existed under the held engine lock");
        let retires_session = members_to_abort.len() == session.interactive_signing.len()
            && per_message_interactive_session(session);
        let previous_retired_at = session.retired_interactive_at_unix;
        if retires_session {
            session.retired_interactive_at_unix = Some(now_unix().max(1));
        }
        (retires_session, previous_retired_at)
    };
    let compacted_retired_sessions =
        compact_retired_per_message_sessions(&mut guard, Some(&request.session_id));

    // Interactive nonce state is intentionally never serialized. Persist while
    // it is still held in memory: the snapshot durably carries the Open binding,
    // policy artifact, and (for a last-member Abort) retirement timestamp. Only
    // after replacement is it safe to destroy the selected nonce handles and
    // report success.
    if let Err(persist_error) =
        persist_engine_state_to_storage_with_key(&guard, &resolved_state_key)
    {
        let state_file_replaced = persist_error.state_file_replaced();
        let persist_error = persist_error.into_engine_error();
        if state_file_replaced {
            let session = guard
                .sessions
                .get_mut(&request.session_id)
                .expect("abort session existed after state-file replacement");
            for member in &members_to_abort {
                if let Some(mut removed) = session.interactive_signing.remove(member) {
                    zeroize_interactive_round1(&mut removed);
                }
            }
            mark_persistence_pending(PersistencePendingOperation::InteractiveState {
                session_id: request.session_id.clone(),
            });
        } else {
            if retires_session {
                guard
                    .sessions
                    .get_mut(&request.session_id)
                    .expect("abort session existed while rolling back retirement")
                    .retired_interactive_at_unix = previous_retired_at;
            }
            restore_compacted_retired_sessions(&mut guard, compacted_retired_sessions);
        }
        return Err(persist_error);
    }

    let session = guard
        .sessions
        .get_mut(&request.session_id)
        .expect("abort session existed after durable retirement");
    for member in &members_to_abort {
        if let Some(mut removed) = session.interactive_signing.remove(member) {
            zeroize_interactive_round1(&mut removed);
        }
    }

    // Only count a success when live interactive state was actually
    // aborted. A no-op call (no session, or an attempt_id filter that
    // matched nothing) returns aborted == false and must not inflate the
    // success counter - the calls_total counter at the top already
    // records that the entry point ran.
    record_hardening_telemetry(|telemetry| {
        telemetry.interactive_session_abort_success_total = telemetry
            .interactive_session_abort_success_total
            .saturating_add(1);
    });

    Ok(InteractiveSessionAbortResult {
        session_id: request.session_id,
        aborted: true,
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

// Checks shared by a signer releasing a share and an elected coordinator
// aggregating a strict first-t package that does not include itself.
fn verify_interactive_signing_package_context(
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

    Ok(())
}

// The frozen spec's Round2 checks (a)-(f). Returns Ok only when every
// check passes; the caller consumes the nonces strictly afterwards.
fn verify_round2_signing_package(
    interactive: &InteractiveSigningState,
    signing_package: &frost::SigningPackage,
) -> Result<(), EngineError> {
    verify_interactive_signing_package_context(interactive, signing_package)?;
    let package_commitments = signing_package.signing_commitments();

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
    signing_emergency_rekey_event: Option<&EmergencyRekeyEvent>,
    wallet_emergency_rekey_event: Option<&EmergencyRekeyEvent>,
    session_finalized: bool,
    tx_result: Option<&TransactionResult>,
    taproot_merkle_root: Option<&[u8; 32]>,
    signing_intent: Option<&InteractiveSigningIntent>,
    quarantined_operator_identifiers: &HashSet<u16>,
    auto_quarantine_config: Option<&AutoQuarantineConfig>,
) -> Result<(), EngineError> {
    // A rekey triggered before Open may live on the per-signing session because
    // BuildTaprootTx has created it but Open has not yet bound it to a key_group.
    // Once bound, new events are redirected to the wallet session. Treat either
    // location as authoritative so the transition between ownership domains can
    // never clear the kill switch.
    if let Some(emergency_rekey_event) =
        signing_emergency_rekey_event.or(wallet_emergency_rekey_event)
    {
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
    enforce_signing_message_binding_to_policy_checked_build_tx(
        session_id,
        message_hex,
        taproot_merkle_root,
        tx_result,
        signing_intent,
    )
}

// Canonical key form for an attempt_id at the round entry points,
// matching canonicalize_attempt_context_for_fingerprint (which
// lowercases attempt_id). The wire accepts attempt_id case-
// insensitively, so the marker registry and live-state lookups must
// operate on this form to be replay-safe.
fn canonical_attempt_id(attempt_id: &str) -> String {
    attempt_id.to_ascii_lowercase()
}

fn canonical_aggregate_attempt_id(attempt_id: &str) -> Result<String, EngineError> {
    if attempt_id.len() != 64 || !attempt_id.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err(EngineError::Validation(
            "InteractiveAggregate: attempt_id must be exactly 64 hexadecimal characters"
                .to_string(),
        ));
    }

    Ok(canonical_attempt_id(attempt_id))
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

fn remove_finalized_interactive_members(
    session: &mut SessionState,
    attempt_id: &str,
    message_digest: &str,
    taproot_merkle_root: Option<&[u8; 32]>,
) {
    let finalized_members: Vec<u16> = session
        .interactive_signing
        .iter()
        .filter(|(_, entry)| {
            // Match the FULL finalized identity (attempt_id + message + root), not
            // just (attempt_id, root): a mismatched aggregate must not destroy the
            // live nonce state of a differently-messaged attempt.
            entry.attempt_context.attempt_id == attempt_id
                && hash_hex(&entry.message_bytes) == message_digest
                && entry.taproot_merkle_root.as_ref() == taproot_merkle_root
        })
        .map(|(member, _)| *member)
        .collect();
    for member in finalized_members {
        if let Some(mut removed) = session.interactive_signing.remove(&member) {
            zeroize_interactive_round1(&mut removed);
        }
    }
}

pub(crate) fn zeroize_interactive_round1(interactive: &mut InteractiveSigningState) {
    if let Some(mut round1) = interactive.round1.take() {
        round1.nonces.zeroize();
    }
}

// Lazy TTL enforcement: every nonce-bearing or mutating interactive endpoint
// sweeps before acting, so an abandoned session's nonces are destroyed before
// another secret can be released or state can change. VerifySignatureShare is
// deliberately read-only for delayed blame checks and never performs this
// sweep. Expiry has abort semantics - durable consumption markers are
// untouched.
/// Resolve the session that holds the DKG key material for `key_group`.
///
/// Interactive signing runs under a fresh RoastSessionID per message, but a wallet's
/// DKG key material is a WALLET-level asset that lives under the session its DKG
/// completed in. This returns that wallet session so any per-signing session can reach
/// the material by key_group:
///  - prefer `session_id` itself if it already holds this wallet's DKG output (the
///    co-located case: DKG and signing share one session, as in the coarse path and
///    the single-session tests);
///  - otherwise find the session whose completed DKG produced `key_group`.
///
/// Returns None when no completed DKG for `key_group` exists (i.e. no wallet key), which
/// callers map to DkgNotReady.
pub(crate) fn resolve_wallet_session_id(
    engine_state: &EngineState,
    session_id: &str,
    key_group: &str,
) -> Option<String> {
    // Prefer the request's own session (the co-located DKG+signing case).
    if let Some(session) = engine_state.sessions.get(session_id) {
        if session
            .dkg_result
            .as_ref()
            .is_some_and(|dkg| dkg.key_group == key_group)
        {
            return Some(session_id.to_string());
        }
    }
    // Otherwise find the wallet session whose completed DKG produced this key_group.
    engine_state
        .sessions
        .iter()
        .find(|(_, session)| {
            session
                .dkg_result
                .as_ref()
                .is_some_and(|dkg| dkg.key_group == key_group)
        })
        .map(|(id, _)| id.clone())
}

pub(crate) fn sweep_expired_interactive_state(engine_state: &mut EngineState) -> Vec<String> {
    let ttl = Duration::from_secs(interactive_session_ttl_seconds());
    let now = interactive_now();
    let mut changed_session_ids = HashSet::new();
    // Open requires an existing wallet DKG, but production signing normally
    // uses a distinct per-message session bound to that wallet. Expiry clears
    // each live attempt's nonces; retirement below retains its bounded policy
    // and replay state until active admission needs the shared slot.
    for (session_id, session) in &mut engine_state.sessions {
        // Per-member expiry: each seat's entry expires independently by its own
        // last successful activity; non-expired sibling seats in the same session
        // survive. Rejected traffic cannot keep nonce state resident.
        let expired_members: Vec<u16> = session
            .interactive_signing
            .iter()
            .filter(|(_, interactive)| {
                now.saturating_duration_since(interactive.last_activity_at) > ttl
            })
            .map(|(member, _)| *member)
            .collect();
        if !expired_members.is_empty() {
            changed_session_ids.insert(session_id.clone());
        }
        for member in &expired_members {
            if let Some(mut removed) = session.interactive_signing.remove(member) {
                zeroize_interactive_round1(&mut removed);
            }
        }
    }
    // Expiry has abort semantics. Retire idle per-message entries while
    // retaining their bounded policy/replay tombstones for delayed retries.
    changed_session_ids.extend(retire_idle_per_message_session_ids(engine_state, None));
    changed_session_ids.retain(|session_id| engine_state.sessions.contains_key(session_id));
    changed_session_ids.into_iter().collect()
}

pub(crate) fn sweep_expired_interactive_state_durably(
    engine_state: &mut EngineState,
) -> Result<(), EngineError> {
    // Persist every session whose live member set changed, not only sessions
    // whose last member expired. Open binds a Build-backed per-message shell
    // in memory, while live interactive state is intentionally omitted from
    // snapshots. If one sibling expired and another remained live, skipping
    // this write would let a crash reload the old unbound shell; persisting the
    // binding lets load classify it as retired once the nonces disappear.
    let changed_session_ids = sweep_expired_interactive_state(engine_state);
    let repairs_pending_interactive_state = interactive_state_persistence_pending();
    // An existing Abort/expiry repair must be retried even when this sweep found
    // no additional expiry. Avoid key-provider/filesystem work on the ordinary
    // no-op fast path.
    if changed_session_ids.is_empty() && !repairs_pending_interactive_state {
        return Ok(());
    }

    if let Err(persist_error) = persist_engine_state_to_storage(engine_state) {
        for session_id in changed_session_ids {
            mark_persistence_pending(PersistencePendingOperation::InteractiveState { session_id });
        }
        return Err(persist_error.into_engine_error());
    }

    // Pending sessions are deliberately excluded from retirement until a
    // successful snapshot covers their uncertain post-rename state. If this
    // sweep expired the last surviving sibling of such an Abort, the write above
    // cleared its protection; classify and persist that now-idle session before
    // returning from the same entry point.
    if repairs_pending_interactive_state {
        let retired_after_repair = sweep_expired_interactive_state(engine_state);
        if let Err(persist_error) = if retired_after_repair.is_empty() {
            Ok(())
        } else {
            persist_engine_state_to_storage(engine_state)
        } {
            for session_id in retired_after_repair {
                mark_persistence_pending(PersistencePendingOperation::InteractiveState {
                    session_id,
                });
            }
            return Err(persist_error.into_engine_error());
        }
    }

    Ok(())
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
