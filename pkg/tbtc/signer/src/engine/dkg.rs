// Distributed-DKG key-package persistence.

use super::*;

/// Persists a DISTRIBUTED FROST DKG result for one seat so the interactive
/// signing path can load its key. A distributed DKG runs Part1/2/3 across
/// nodes and each node's Part3 returns only ITS OWN secret key package.
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
    // data_hex is the serialized SECRET signing share. Move it into a zeroizing holder
    // BEFORE any fallible check (validation, admission, quarantine can all return first),
    // so serde's owned String is wiped on EVERY return path rather than dropped un-wiped.
    let data_hex = Zeroizing::new(std::mem::take(&mut request.key_package.data_hex));
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

    let key_package = decode_key_package(OP, &request.key_package.identifier, &data_hex)?;

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
    ensure_session_insert_capacity(&mut guard.sessions, &request.session_id)?;

    // A group verifying key identifies one wallet. Keeping the same key_group in
    // two sessions would make wallet lookup depend on randomized HashMap order and
    // could split local seats or wallet-level kill switches across the copies.
    // Reject the second owner before mutating either session; same-session
    // multi-seat accumulation remains valid below.
    if guard.sessions.iter().any(|(session_id, session)| {
        session_id != &request.session_id
            && session
                .dkg_result
                .as_ref()
                .is_some_and(|dkg| dkg.key_group == key_group)
    }) {
        return Err(EngineError::SessionConflict {
            session_id: request.session_id,
        });
    }

    // A session first created by Interactive Open is a per-signing flow, not a
    // wallet owner. Installing unrelated DKG material into that namespace would
    // make dkg_result take precedence over bound_key_group during Round2 and
    // could route share release around the original wallet's lifecycle gates.
    // Keep the two roles disjoint for the full lifetime of the session.
    if guard
        .sessions
        .get(&request.session_id)
        .is_some_and(|session| session.dkg_result.is_none() && session.bound_key_group.is_some())
    {
        return Err(EngineError::SessionConflict {
            session_id: request.session_id,
        });
    }

    let dkg_session_id = request.session_id.clone();
    let session_existed = guard.sessions.contains_key(&dkg_session_id);
    let session = guard
        .sessions
        .entry(dkg_session_id.clone())
        .or_insert_with(SessionState::default);
    let previous_dkg_result = session.dkg_result.clone();
    let previous_dkg_public_key_package = session.dkg_public_key_package.clone();
    let key_package_map_was_absent = session.dkg_key_packages.is_none();

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

    let replaced_key_package = session
        .dkg_key_packages
        .get_or_insert_with(BTreeMap::new)
        .insert(request.participant_identifier, key_package);

    // Clone the result before the `&guard` persist call so the mutable `session`
    // borrow ends here (mirrors run_dkg's ordering).
    let result = session
        .dkg_result
        .clone()
        .expect("dkg_result was just set for this session");
    if let Err(persist_error) = persist_engine_state_to_storage(&guard) {
        let state_file_replaced = persist_error.state_file_replaced();
        let persist_error = persist_error.into_engine_error();
        if !state_file_replaced {
            if session_existed {
                let rollback_session = guard.sessions.get_mut(&dkg_session_id).ok_or_else(|| {
                    EngineError::Internal(format!(
                        "distributed DKG session [{dkg_session_id}] disappeared while rolling back a failed persist: {persist_error}"
                    ))
                })?;
                rollback_session.dkg_result = previous_dkg_result;
                rollback_session.dkg_public_key_package = previous_dkg_public_key_package;
                if let Some(key_packages) = rollback_session.dkg_key_packages.as_mut() {
                    key_packages.remove(&request.participant_identifier);
                    if let Some(previous_key_package) = replaced_key_package {
                        key_packages.insert(request.participant_identifier, previous_key_package);
                    }
                }
                if key_package_map_was_absent
                    && rollback_session
                        .dkg_key_packages
                        .as_ref()
                        .is_some_and(BTreeMap::is_empty)
                {
                    rollback_session.dkg_key_packages = None;
                }
            } else {
                guard.sessions.remove(&dkg_session_id);
            }
        }
        return Err(persist_error);
    }

    Ok(result)
}
