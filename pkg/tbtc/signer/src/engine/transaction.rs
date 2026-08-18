// Taproot transaction building.

use super::*;

pub fn build_taproot_tx(request: BuildTaprootTxRequest) -> Result<TransactionResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.build_taproot_tx_calls_total =
            telemetry.build_taproot_tx_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::BuildTaprootTx);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    if request.inputs.is_empty() {
        return Err(EngineError::Validation(
            "inputs must not be empty".to_string(),
        ));
    }
    // Cap the input count before any per-input work (txid parsing, script
    // pubkey decode, sighash derivation) so an adversarial request cannot
    // balloon the work into O(N) of an attacker-chosen N. Mirrors the
    // output-count cap that lives inside the signing-policy firewall; both
    // are operator-tunable via the policy config.
    let max_input_count = crate::engine::policy::policy_max_input_count();
    if request.inputs.len() > max_input_count {
        return Err(EngineError::Validation(format!(
            "input count [{}] exceeds policy max [{}]",
            request.inputs.len(),
            max_input_count
        )));
    }

    if request.outputs.is_empty() {
        return Err(EngineError::Validation(
            "outputs must not be empty".to_string(),
        ));
    }

    if request.script_tree_hex.is_some() {
        return Err(EngineError::Validation(
            "script_tree_hex is not yet supported; provide fully-derived output script_pubkey_hex values".to_string(),
        ));
    }

    let request_fingerprint = fingerprint(&request)?;
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;

    // A post-replacement failure left the accepted artifact fail-closed in
    // memory and on the replacement state file, but directory durability is
    // unconfirmed. Flush that exact state before either the cache-hit return or
    // a conflicting request is considered.
    if let Some(pending_operation) = pending_build_taproot_tx_operation(&request.session_id) {
        persist_engine_state_to_storage(&guard)
            .map_err(PersistEngineStateError::into_engine_error)?;
        clear_persistence_pending_operation(&pending_operation);
    }

    if let Some(session) = guard.sessions.get(&request.session_id) {
        if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
            return reject_lifecycle_policy(
                &request.session_id,
                "emergency_rekey_required",
                format!(
                    "build_taproot_tx blocked: emergency rekey required since [{}]: {}",
                    emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
                ),
            );
        }

        if let Some(existing) = &session.build_tx_request_fingerprint {
            if existing == &request_fingerprint {
                let cached_result = session
                    .tx_result
                    .clone()
                    .ok_or_else(|| EngineError::Internal("missing build tx cache".to_string()))?;
                let cached_tx_bytes = hex::decode(&cached_result.tx_hex).map_err(|_| {
                    EngineError::Internal("cached build tx hex is not valid hex".to_string())
                })?;
                let cached_tx: Transaction = deserialize(&cached_tx_bytes).map_err(|_| {
                    EngineError::Internal(
                        "cached build tx hex is not a valid transaction".to_string(),
                    )
                })?;
                let total_output_value_sats =
                    cached_tx.output.iter().try_fold(0u64, |total, output| {
                        total.checked_add(output.value.to_sat()).ok_or_else(|| {
                            EngineError::Internal(
                                "cached build tx output total overflowed u64 bounds".to_string(),
                            )
                        })
                    })?;
                if total_output_value_sats > BITCOIN_MAX_MONEY_SATS {
                    return Err(EngineError::Internal(format!(
                        "cached build tx output total [{}] exceeds Bitcoin max money [{}]",
                        total_output_value_sats, BITCOIN_MAX_MONEY_SATS
                    )));
                }

                // Idempotent retries return the cached transaction without consuming a
                // new rate-limit token, but still re-check current non-rate policy gates.
                recheck_signing_policy_firewall_without_rate_limit(
                    &request.session_id,
                    &cached_tx.output,
                    total_output_value_sats,
                )?;
                return Ok(cached_result);
            }

            return Err(EngineError::SessionConflict {
                session_id: request.session_id,
            });
        }
    }
    // Capacity rejection must precede policy rate-limit consumption. This is a
    // read-only preflight; the transactional reservation still occurs after all
    // request validation, immediately before the durable mutation.
    ensure_session_insert_admission_capacity(&guard, &request.session_id)?;
    // BuildTaprootTx is an assembly-only step. Input prevout values and scripts
    // are trusted caller-supplied metadata and are not verified against chain
    // state. Both are nevertheless required because BIP-341 SIGHASH_DEFAULT
    // commits to the ordered prevout TxOuts for every input.
    let mut total_input_value_sats = 0u64;
    let mut seen_input_keys = HashSet::new();
    let mut inputs = Vec::with_capacity(request.inputs.len());
    let mut prevouts = Vec::with_capacity(request.inputs.len());
    for input in request.inputs {
        if input.value_sats > BITCOIN_MAX_MONEY_SATS {
            return Err(EngineError::Validation(format!(
                "input value_sats [{}] exceeds Bitcoin max money [{}]",
                input.value_sats, BITCOIN_MAX_MONEY_SATS
            )));
        }

        total_input_value_sats = total_input_value_sats
            .checked_add(input.value_sats)
            .ok_or_else(|| {
                EngineError::Validation("input value_sats total overflowed u64 bounds".to_string())
            })?;
        if total_input_value_sats > BITCOIN_MAX_MONEY_SATS {
            return Err(EngineError::Validation(format!(
                "input value_sats total [{}] exceeds Bitcoin max money [{}]",
                total_input_value_sats, BITCOIN_MAX_MONEY_SATS
            )));
        }

        let txid = Txid::from_str(&input.txid_hex).map_err(|_| {
            EngineError::Validation(format!("invalid input txid_hex [{}]", input.txid_hex))
        })?;
        let input_key = format!("{txid}:{}", input.vout);
        if !seen_input_keys.insert(input_key.clone()) {
            return Err(EngineError::Validation(format!(
                "duplicate input outpoint [{}]",
                input_key
            )));
        }

        let previous_output = OutPoint {
            txid,
            vout: input.vout,
        };

        let script_pubkey_bytes = hex::decode(&input.script_pubkey_hex).map_err(|_| {
            EngineError::Validation(format!(
                "invalid input script_pubkey_hex [{}]",
                input.script_pubkey_hex
            ))
        })?;
        let script_pubkey = ScriptBuf::from_bytes(script_pubkey_bytes);
        if let Some(script_error) = script_pubkey
            .instructions()
            .find_map(|instruction| instruction.err())
        {
            return Err(EngineError::Validation(format!(
                "invalid input script_pubkey_hex [{}]: {script_error}",
                input.script_pubkey_hex
            )));
        }
        if !script_pubkey.is_p2tr() {
            return Err(EngineError::Validation(format!(
                "input script_pubkey_hex [{}] is not a P2TR prevout",
                input.script_pubkey_hex
            )));
        }

        inputs.push(TxIn {
            previous_output,
            script_sig: ScriptBuf::new(),
            // Use final sequence for deterministic non-RBF transaction assembly.
            sequence: Sequence::MAX,
            witness: Witness::new(),
        });
        prevouts.push(TxOut {
            value: Amount::from_sat(input.value_sats),
            script_pubkey,
        });
    }

    let mut total_output_value_sats = 0u64;
    let mut outputs = Vec::with_capacity(request.outputs.len());
    for output in request.outputs {
        if output.value_sats > BITCOIN_MAX_MONEY_SATS {
            return Err(EngineError::Validation(format!(
                "output value_sats [{}] exceeds Bitcoin max money [{}]",
                output.value_sats, BITCOIN_MAX_MONEY_SATS
            )));
        }

        total_output_value_sats = total_output_value_sats
            .checked_add(output.value_sats)
            .ok_or_else(|| {
                EngineError::Validation("output value_sats total overflowed u64 bounds".to_string())
            })?;
        if total_output_value_sats > BITCOIN_MAX_MONEY_SATS {
            return Err(EngineError::Validation(format!(
                "output value_sats total [{}] exceeds Bitcoin max money [{}]",
                total_output_value_sats, BITCOIN_MAX_MONEY_SATS
            )));
        }

        let script_pubkey_bytes = hex::decode(&output.script_pubkey_hex).map_err(|_| {
            EngineError::Validation(format!(
                "invalid output script_pubkey_hex [{}]",
                output.script_pubkey_hex
            ))
        })?;
        let script_pubkey = ScriptBuf::from_bytes(script_pubkey_bytes);
        if let Some(script_error) = script_pubkey
            .instructions()
            .find_map(|instruction| instruction.err())
        {
            return Err(EngineError::Validation(format!(
                "invalid output script_pubkey_hex [{}]: {script_error}",
                output.script_pubkey_hex
            )));
        }
        outputs.push(TxOut {
            value: Amount::from_sat(output.value_sats),
            script_pubkey,
        });
    }

    if total_output_value_sats > total_input_value_sats {
        return Err(EngineError::Validation(format!(
            "output value_sats total [{}] exceeds input value_sats total [{}]",
            total_output_value_sats, total_input_value_sats
        )));
    }
    if let Err(error) =
        enforce_signing_policy_firewall(&request.session_id, &outputs, total_output_value_sats)
    {
        if matches!(error, EngineError::SigningPolicyRejected { .. }) {
            record_canary_policy_outcome(true);
        }
        return Err(error);
    }
    // Count only first-time artifact decisions. Cache hits are intentionally
    // excluded so cheap idempotent replays cannot dilute the rolling policy
    // rejection rate used by promotion control.
    record_canary_policy_outcome(false);

    let tx = Transaction {
        // Match the Go host TransactionBuilder's canonical unsigned transaction.
        // Transaction-version drift changes every BIP-341 sighash and makes the
        // policy artifact unusable even when all inputs and outputs agree.
        version: Version::ONE,
        lock_time: LockTime::ZERO,
        input: inputs,
        output: outputs,
    };

    let taproot_key_spend_sighashes_hex = policy_bound_signing_messages_hex(&tx, &prevouts)?;

    let result = TransactionResult {
        session_id: request.session_id,
        tx_hex: serialize_hex(&tx),
        taproot_key_spend_sighashes_hex,
    };

    // BuildTaprootTx is keyed into the shared session namespace for idempotency
    // caching only; this session entry may intentionally not have DKG/signing
    // state populated.
    let build_session_id = result.session_id.clone();
    // All request and policy validation is complete. Reserve the durable
    // session slot now so a rejected request cannot evict a replay tombstone.
    let compacted_retired_sessions = ensure_session_insert_capacity(&mut guard, &build_session_id)?;
    let accepted_request_fingerprint = request_fingerprint.clone();
    let previous_build_state = guard.sessions.get(&build_session_id).map(|session| {
        (
            session.build_tx_request_fingerprint.clone(),
            session.tx_result.clone(),
        )
    });
    let session = guard
        .sessions
        .entry(build_session_id.clone())
        .or_insert_with(SessionState::default);
    session.build_tx_request_fingerprint = Some(request_fingerprint);
    session.tx_result = Some(result.clone());
    if let Err(persist_error) = persist_engine_state_to_storage(&guard) {
        let state_file_replaced = persist_error.state_file_replaced();
        let persist_error = persist_error.into_engine_error();
        if state_file_replaced {
            mark_persistence_pending(PersistencePendingOperation::BuildTaprootTx {
                session_id: build_session_id,
                request_fingerprint: accepted_request_fingerprint,
            });
        } else if let Some((previous_fingerprint, previous_result)) = previous_build_state {
            let session = guard.sessions.get_mut(&build_session_id).ok_or_else(|| {
                EngineError::Internal(format!(
                    "build transaction session [{build_session_id}] disappeared while rolling back a failed persist: {persist_error}"
                ))
            })?;
            session.build_tx_request_fingerprint = previous_fingerprint;
            session.tx_result = previous_result;
        } else {
            guard.sessions.remove(&build_session_id);
        }
        if !state_file_replaced {
            restore_compacted_retired_sessions(&mut guard, compacted_retired_sessions);
        }
        return Err(persist_error);
    }
    record_hardening_telemetry(|telemetry| {
        telemetry.build_taproot_tx_success_total =
            telemetry.build_taproot_tx_success_total.saturating_add(1);
    });

    Ok(result)
}
