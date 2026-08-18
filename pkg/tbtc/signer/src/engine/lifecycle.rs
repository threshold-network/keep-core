// Operational lifecycle: canary rollout, refresh cadence/shares, emergency rekey, quarantine status.

use super::*;

#[cfg(test)]
static CANARY_PROMOTION_HOLD_NEXT_LOCK: std::sync::atomic::AtomicBool =
    std::sync::atomic::AtomicBool::new(false);
#[cfg(test)]
static CANARY_PROMOTION_LOCK_HELD: std::sync::atomic::AtomicBool =
    std::sync::atomic::AtomicBool::new(false);
#[cfg(test)]
static CANARY_PROMOTION_RELEASE_LOCK: std::sync::atomic::AtomicBool =
    std::sync::atomic::AtomicBool::new(true);
#[cfg(test)]
static CANARY_PROMOTION_LOCK_ATTEMPTS: std::sync::atomic::AtomicUsize =
    std::sync::atomic::AtomicUsize::new(0);

#[cfg(test)]
pub(crate) fn arm_canary_promotion_lock_hold_for_tests() {
    use std::sync::atomic::Ordering;
    CANARY_PROMOTION_LOCK_ATTEMPTS.store(0, Ordering::SeqCst);
    CANARY_PROMOTION_LOCK_HELD.store(false, Ordering::SeqCst);
    CANARY_PROMOTION_RELEASE_LOCK.store(false, Ordering::SeqCst);
    CANARY_PROMOTION_HOLD_NEXT_LOCK.store(true, Ordering::SeqCst);
}

#[cfg(test)]
pub(crate) fn canary_promotion_lock_attempts_for_tests() -> usize {
    CANARY_PROMOTION_LOCK_ATTEMPTS.load(std::sync::atomic::Ordering::SeqCst)
}

#[cfg(test)]
pub(crate) fn canary_promotion_lock_held_for_tests() -> bool {
    CANARY_PROMOTION_LOCK_HELD.load(std::sync::atomic::Ordering::SeqCst)
}

#[cfg(test)]
pub(crate) fn release_canary_promotion_lock_for_tests() {
    CANARY_PROMOTION_RELEASE_LOCK.store(true, std::sync::atomic::Ordering::SeqCst);
}

#[cfg(test)]
fn maybe_hold_canary_promotion_lock_for_tests() {
    use std::sync::atomic::Ordering;
    if CANARY_PROMOTION_HOLD_NEXT_LOCK.swap(false, Ordering::SeqCst) {
        CANARY_PROMOTION_LOCK_HELD.store(true, Ordering::SeqCst);
        while !CANARY_PROMOTION_RELEASE_LOCK.load(Ordering::SeqCst) {
            std::thread::yield_now();
        }
    }
}

fn positive_signer_env_u64(name: &str) -> Option<u64> {
    signer_env_var(name)
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| *value > 0)
}

fn canary_latency_threshold_ms(primary: &str, legacy: &str, default: u64) -> u64 {
    positive_signer_env_u64(primary)
        .or_else(|| positive_signer_env_u64(legacy))
        .unwrap_or(default)
}

pub(crate) fn canary_max_interactive_round1_p95_ms() -> u64 {
    canary_latency_threshold_ms(
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND1_P95_MS_ENV,
        TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV,
        TBTC_SIGNER_DEFAULT_CANARY_MAX_START_SIGN_ROUND_P95_MS,
    )
}

pub(crate) fn canary_max_interactive_round2_p95_ms() -> u64 {
    canary_latency_threshold_ms(
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND2_P95_MS_ENV,
        TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV,
        TBTC_SIGNER_DEFAULT_CANARY_MAX_START_SIGN_ROUND_P95_MS,
    )
}

pub(crate) fn canary_max_interactive_aggregate_p95_ms() -> u64 {
    canary_latency_threshold_ms(
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_AGGREGATE_P95_MS_ENV,
        TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS_ENV,
        TBTC_SIGNER_DEFAULT_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS,
    )
}

pub(crate) fn canary_max_policy_reject_rate_bps() -> u64 {
    signer_env_var(TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV)
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| *value <= TBTC_SIGNER_MAX_POLICY_REJECT_RATE_BPS)
        .unwrap_or(TBTC_SIGNER_DEFAULT_CANARY_MAX_POLICY_REJECT_RATE_BPS)
}

pub(crate) fn canary_min_samples() -> u64 {
    positive_signer_env_u64(TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV)
        .map(|value| value.min(TBTC_SIGNER_MAX_CANARY_MIN_SAMPLES))
        .unwrap_or(TBTC_SIGNER_DEFAULT_CANARY_MIN_SAMPLES)
}

pub(crate) fn canary_min_policy_samples() -> u64 {
    positive_signer_env_u64(TBTC_SIGNER_CANARY_MIN_POLICY_SAMPLES_ENV)
        .map(|value| value.min(TBTC_SIGNER_MAX_CANARY_MIN_SAMPLES))
        // Preserve the pre-knob safety posture unless an operator explicitly
        // tunes the lower-volume policy evidence window independently.
        .unwrap_or_else(canary_min_samples)
}

pub(crate) fn canary_max_sample_age_seconds() -> u64 {
    positive_signer_env_u64(TBTC_SIGNER_CANARY_MAX_SAMPLE_AGE_SECONDS_ENV)
        .map(|value| value.min(TBTC_SIGNER_MAX_CANARY_SAMPLE_AGE_SECONDS))
        .unwrap_or(TBTC_SIGNER_DEFAULT_CANARY_MAX_SAMPLE_AGE_SECONDS)
}

pub(crate) fn next_canary_percent(current_percent: u8) -> Option<u8> {
    match current_percent {
        10 => Some(50),
        50 => Some(100),
        _ => None,
    }
}

pub(crate) fn can_promote_to_target_percent(current_percent: u8, target_percent: u8) -> bool {
    next_canary_percent(current_percent).is_some_and(|next| next == target_percent)
}

pub(crate) fn refresh_continuity_reference_key_group(session: &SessionState) -> Option<String> {
    session.dkg.result
        .as_ref()
        .map(|result| result.key_group.clone())
}

/// Returns whether this session contains metadata written by the retired
/// synthetic `RefreshShares` implementation. No cryptographically valid refresh
/// record can exist until a versioned multi-round protocol is implemented, so
/// these fields are retained only for persisted-schema compatibility and must
/// never establish cadence or continuity.
pub(crate) fn legacy_synthetic_refresh_artifacts_present(session: &SessionState) -> bool {
    session.lifecycle.refresh_request_fingerprint.is_some()
        || session.lifecycle.refresh_result.is_some()
        || !session.lifecycle.refresh_history.is_empty()
        || session.lifecycle.refresh_count != 0
}

pub(crate) fn refresh_history_continuity_preserved(session: &SessionState) -> bool {
    !legacy_synthetic_refresh_artifacts_present(session)
}

pub(crate) fn refresh_cadence_due_unix(
    session: &SessionState,
    cadence_seconds: u64,
) -> Option<u64> {
    if let Some(dkg_result) = session.dkg.result.as_ref() {
        return Some(dkg_result.created_at_unix.saturating_add(cadence_seconds));
    }

    // The retired synthetic RefreshShares path could create a persisted session
    // without ever running DKG. Such state has no trustworthy cadence anchor and
    // must fail closed instead of receiving a new `now + cadence` deadline on
    // every query. Unix epoch is the explicit "already due" sentinel shared by
    // status and telemetry.
    legacy_synthetic_refresh_artifacts_present(session).then_some(0)
}

pub(crate) fn refresh_cadence_is_overdue(now_unix: u64, due_unix: u64) -> bool {
    // A zero deadline is the explicit sentinel for unanchored legacy synthetic
    // refresh state. It remains overdue even if the system clock rolls back to
    // or before UNIX_EPOCH and `now_unix()` saturates to zero.
    due_unix == 0 || now_unix > due_unix
}

pub fn refresh_cadence_status(
    request: RefreshCadenceStatusRequest,
) -> Result<RefreshCadenceStatusResult, EngineError> {
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
    let cadence_seconds = refresh_cadence_seconds();
    let now = now_unix();
    let next_refresh_due_unix = refresh_cadence_due_unix(session, cadence_seconds)
        .unwrap_or_else(|| now.saturating_add(cadence_seconds));
    let overdue = refresh_cadence_is_overdue(now, next_refresh_due_unix);
    let continuity_reference_key_group = refresh_continuity_reference_key_group(session);
    let emergency_rekey_reason = session.lifecycle.emergency_rekey_event
        .as_ref()
        .map(|event| event.reason.clone());

    Ok(RefreshCadenceStatusResult {
        session_id: request.session_id,
        // No cryptographically valid refresh can be reported by this build.
        // Legacy synthetic metadata is deliberately ignored.
        refresh_count: 0,
        last_refresh_epoch: 0,
        cadence_seconds,
        next_refresh_due_unix,
        overdue,
        continuity_preserved: refresh_history_continuity_preserved(session),
        continuity_reference_key_group,
        emergency_rekey_required: session.lifecycle.emergency_rekey_event.is_some(),
        emergency_rekey_reason,
    })
}

pub fn trigger_emergency_rekey(
    request: TriggerEmergencyRekeyRequest,
) -> Result<TriggerEmergencyRekeyResult, EngineError> {
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;
    let reason = request.reason.trim();
    if reason.is_empty() {
        return Err(EngineError::Validation(
            "reason must not be empty".to_string(),
        ));
    }
    // Cap the human-readable reason so a malformed or adversarial request
    // cannot persist an unbounded string into the durable emergency-rekey
    // event (or echo it back through subsequent reads and canary-rollback
    // telemetry). 256 bytes is plenty for an operator-supplied free-text
    // explanation; anything longer is rejected up-front rather than
    // truncated silently.
    if reason.len() > 256 {
        return Err(EngineError::Validation(
            "reason exceeds max length 256 bytes".to_string(),
        ));
    }

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;

    // Emergency rekey is a WALLET-level kill switch, and interactive Round2 reads it
    // from the wallet (DKG) session resolved by key_group. Defense in depth: if a
    // caller passes a per-signing session id (a distinct RoastSessionID bound to a
    // wallet key but holding no DKG of its own), record the event on the WALLET session
    // it serves, so the writer lands the kill switch exactly where every reader looks -
    // the writer and reader can never diverge. A session that already holds the DKG
    // resolves to itself, so co-located callers are unchanged.
    let target_session_id = guard
        .sessions
        .get(&request.session_id)
        .and_then(|session| {
            if session.dkg.result.is_some() {
                None
            } else {
                session.interactive.bound_key_group.clone()
            }
        })
        .and_then(|key_group| resolve_wallet_session_id(&guard, &request.session_id, &key_group))
        .unwrap_or_else(|| request.session_id.clone());

    if let Some(pending_operation) = pending_emergency_rekey_operation(&target_session_id) {
        let matching_result = match &pending_operation {
            PersistencePendingOperation::EmergencyRekey { result } if result.reason == reason => {
                Some(result.clone())
            }
            _ => None,
        };
        persist_engine_state_to_storage(&guard)
            .map_err(PersistEngineStateError::into_engine_error)?;
        clear_persistence_pending_operation(&pending_operation);
        if let Some(result) = matching_result {
            return Ok(result);
        }
    }

    let session =
        guard
            .sessions
            .get_mut(&target_session_id)
            .ok_or_else(|| EngineError::SessionNotFound {
                session_id: request.session_id.clone(),
            })?;
    if session.lifecycle.emergency_rekey_event.is_some() {
        return Err(EngineError::Validation(format!(
            "emergency rekey already triggered for session [{target_session_id}]; event is immutable"
        )));
    }
    let triggered_at_unix = now_unix();
    let previous_emergency_rekey_event =
        session.lifecycle.emergency_rekey_event.replace(EmergencyRekeyEvent {
            reason: reason.to_string(),
            triggered_at_unix,
        });
    let result = TriggerEmergencyRekeyResult {
        session_id: target_session_id.clone(),
        emergency_rekey_required: true,
        reason: reason.to_string(),
        triggered_at_unix,
        recommended_new_session_id: format!("{target_session_id}-rekey-{triggered_at_unix}"),
    };
    if let Err(persist_error) = persist_engine_state_to_storage(&guard) {
        let state_file_replaced = persist_error.state_file_replaced();
        let persist_error = persist_error.into_engine_error();
        if state_file_replaced {
            mark_persistence_pending(PersistencePendingOperation::EmergencyRekey {
                result: result.clone(),
            });
        } else {
            let rollback_session = guard.sessions.get_mut(&target_session_id).ok_or_else(|| {
                EngineError::Internal(format!(
                    "emergency rekey session [{target_session_id}] disappeared while rolling back a failed persist: {persist_error}"
                ))
            })?;
            rollback_session.lifecycle.emergency_rekey_event = previous_emergency_rekey_event;
        }
        return Err(persist_error);
    }

    Ok(result)
}

pub fn canary_rollout_status() -> Result<CanaryRolloutStatusResult, EngineError> {
    enforce_provenance_gate()?;
    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    // Snapshot rollout state and its process-local evidence under the same
    // engine lock used by PromoteCanary. This avoids reporting one stage with
    // another stage's evidence during a concurrent transition.
    let gate_failures = canary_promotion_gate_failures();
    let gate_passed = gate_failures.is_empty();
    let current_percent = guard.canary_rollout.current_percent;
    let previous_percent = guard.canary_rollout.previous_percent;
    let config_version = guard.canary_rollout.config_version;
    let last_action_unix = guard.canary_rollout.last_action_unix;

    Ok(CanaryRolloutStatusResult {
        current_percent,
        previous_percent,
        config_version,
        promotion_gate_passed: gate_passed,
        gate_failures,
        recommended_next_percent: if gate_passed {
            next_canary_percent(current_percent)
        } else {
            None
        },
        last_action_unix,
    })
}

fn record_recovered_canary_transition(pending_operation: &PersistencePendingOperation) {
    record_hardening_telemetry(|telemetry| match pending_operation {
        PersistencePendingOperation::CanaryPromotion { .. } => {
            telemetry.canary_promotions_total = telemetry.canary_promotions_total.saturating_add(1);
        }
        PersistencePendingOperation::CanaryRollback { .. } => {
            telemetry.canary_rollbacks_total = telemetry.canary_rollbacks_total.saturating_add(1);
        }
        _ => {}
    });
}

pub fn promote_canary(request: PromoteCanaryRequest) -> Result<PromoteCanaryResult, EngineError> {
    enforce_provenance_gate()?;
    if !matches!(request.target_percent, 10 | 50 | 100) {
        return Err(EngineError::Validation(
            "target_percent must be one of [10, 50, 100]".to_string(),
        ));
    }

    #[cfg(test)]
    CANARY_PROMOTION_LOCK_ATTEMPTS.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    #[cfg(test)]
    maybe_hold_canary_promotion_lock_for_tests();
    if let Some(pending_operation) = pending_canary_operation() {
        let matching_result = match &pending_operation {
            PersistencePendingOperation::CanaryPromotion { result }
                if result.to_percent == request.target_percent =>
            {
                Some(result.clone())
            }
            _ => None,
        };
        persist_engine_state_to_storage(&guard)
            .map_err(PersistEngineStateError::into_engine_error)?;
        // The post-replacement error path already activated the new stage and
        // reset prior-stage evidence. This retry confirms durability only, so
        // preserve evidence accumulated since that transition and count the
        // recovered operation exactly once before clearing its pending marker.
        record_recovered_canary_transition(&pending_operation);
        clear_persistence_pending_operation(&pending_operation);
        if let Some(result) = matching_result {
            return Ok(result);
        }
    }
    let current_percent = guard.canary_rollout.current_percent;

    if request.target_percent == current_percent {
        return Ok(PromoteCanaryResult {
            from_percent: current_percent,
            to_percent: current_percent,
            config_version: guard.canary_rollout.config_version,
            promoted_at_unix: guard.canary_rollout.last_action_unix,
        });
    }

    if !can_promote_to_target_percent(current_percent, request.target_percent) {
        return reject_lifecycle_policy(
            "canary-rollout",
            "invalid_canary_promotion_step",
            format!(
                "canary promotion must follow 10->50->100 progression; current [{}], target [{}]",
                current_percent, request.target_percent
            ),
        );
    }
    // This snapshot is deliberately taken while the rollout-state lock is held
    // and immediately before mutation. Concurrent 50%/100% requests therefore
    // cannot reuse one stage's evidence, and samples cannot age out while a
    // request waits for the state lock.
    let gate_failures = canary_promotion_gate_failures();
    if !gate_failures.is_empty() {
        return reject_lifecycle_policy(
            "canary-rollout",
            "canary_slo_gate_failed",
            gate_failures.join("; "),
        );
    }

    let previous_canary_rollout = guard.canary_rollout.clone();
    guard.canary_rollout.previous_percent = current_percent;
    guard.canary_rollout.current_percent = request.target_percent;
    guard.canary_rollout.config_version = guard.canary_rollout.config_version.saturating_add(1);
    guard.canary_rollout.last_action_unix = now_unix();
    let result = PromoteCanaryResult {
        from_percent: current_percent,
        to_percent: request.target_percent,
        config_version: guard.canary_rollout.config_version,
        promoted_at_unix: guard.canary_rollout.last_action_unix,
    };
    if let Err(persist_error) = persist_engine_state_to_storage(&guard) {
        let state_file_replaced = persist_error.state_file_replaced();
        let persist_error = persist_error.into_engine_error();
        if state_file_replaced {
            mark_persistence_pending(PersistencePendingOperation::CanaryPromotion {
                result: result.clone(),
            });
            // The replacement state already moved to the next cohort. Treat it
            // as the active stage immediately even though directory durability
            // still needs repair; prior-stage evidence must not authorize the
            // following promotion while the retry is pending.
            reset_canary_promotion_evidence();
        } else {
            guard.canary_rollout = previous_canary_rollout;
        }
        return Err(persist_error);
    }
    record_hardening_telemetry(|telemetry| {
        telemetry.canary_promotions_total = telemetry.canary_promotions_total.saturating_add(1);
    });
    reset_canary_promotion_evidence();

    Ok(result)
}

pub fn rollback_canary(
    request: RollbackCanaryRequest,
) -> Result<RollbackCanaryResult, EngineError> {
    enforce_provenance_gate()?;
    let reason = request.reason.trim();
    if reason.is_empty() {
        return Err(EngineError::Validation(
            "reason must not be empty".to_string(),
        ));
    }
    // See trigger_emergency_rekey: cap the operator-supplied reason so it
    // cannot balloon the durable canary-rollback record. 256 bytes is plenty
    // for a free-text operator note.
    if reason.len() > 256 {
        return Err(EngineError::Validation(
            "reason exceeds max length 256 bytes".to_string(),
        ));
    }

    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    if let Some(pending_operation) = pending_canary_operation() {
        let matching_result = match &pending_operation {
            PersistencePendingOperation::CanaryRollback { result } if result.reason == reason => {
                Some(result.clone())
            }
            _ => None,
        };
        persist_engine_state_to_storage(&guard)
            .map_err(PersistEngineStateError::into_engine_error)?;
        // See promote_canary: repairing directory durability does not start a
        // second rollout stage and must not discard current-stage evidence.
        record_recovered_canary_transition(&pending_operation);
        clear_persistence_pending_operation(&pending_operation);
        if let Some(result) = matching_result {
            return Ok(result);
        }
    }
    let from_percent = guard.canary_rollout.current_percent;
    let to_percent = guard.canary_rollout.previous_percent.min(from_percent);

    if to_percent == from_percent {
        let state_path = active_state_file_path()?;
        sync_existing_state_file_parent_directory(&state_path)?;
        return Ok(RollbackCanaryResult {
            from_percent,
            to_percent,
            config_version: guard.canary_rollout.config_version,
            reason: reason.to_string(),
            rolled_back_at_unix: guard.canary_rollout.last_action_unix,
        });
    }

    let previous_canary_rollout = guard.canary_rollout.clone();
    guard.canary_rollout.current_percent = to_percent;
    guard.canary_rollout.previous_percent = to_percent;
    guard.canary_rollout.config_version = guard.canary_rollout.config_version.saturating_add(1);
    guard.canary_rollout.last_action_unix = now_unix();
    let result = RollbackCanaryResult {
        from_percent,
        to_percent,
        config_version: guard.canary_rollout.config_version,
        reason: reason.to_string(),
        rolled_back_at_unix: guard.canary_rollout.last_action_unix,
    };
    if let Err(persist_error) = persist_engine_state_to_storage(&guard) {
        let state_file_replaced = persist_error.state_file_replaced();
        let persist_error = persist_error.into_engine_error();
        if state_file_replaced {
            mark_persistence_pending(PersistencePendingOperation::CanaryRollback {
                result: result.clone(),
            });
            reset_canary_promotion_evidence();
        } else {
            guard.canary_rollout = previous_canary_rollout;
        }
        return Err(persist_error);
    }
    record_hardening_telemetry(|telemetry| {
        telemetry.canary_rollbacks_total = telemetry.canary_rollbacks_total.saturating_add(1);
    });
    reset_canary_promotion_evidence();

    Ok(result)
}

pub fn quarantine_status(
    request: QuarantineStatusRequest,
) -> Result<QuarantineStatusResult, EngineError> {
    enforce_provenance_gate()?;
    if request.operator_identifier == 0 {
        return Err(EngineError::Validation(
            "operator_identifier must be non-zero".to_string(),
        ));
    }

    let auto_quarantine_config = load_auto_quarantine_config()?;
    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let fault_score = guard
        .operator_fault_scores
        .get(&request.operator_identifier)
        .copied()
        .unwrap_or(0);
    let quarantined = guard
        .quarantined_operator_identifiers
        .contains(&request.operator_identifier);
    let dao_override_allowlisted = auto_quarantine_config.as_ref().is_some_and(|config| {
        config
            .dao_allowlist_identifiers
            .contains(&request.operator_identifier)
    });

    Ok(QuarantineStatusResult {
        operator_identifier: request.operator_identifier,
        auto_quarantine_enabled: auto_quarantine_config.is_some(),
        fault_score,
        quarantine_threshold: auto_quarantine_config
            .as_ref()
            .map(|config| config.fault_threshold)
            .unwrap_or(0),
        quarantined: quarantined && !dao_override_allowlisted,
        dao_override_allowlisted,
    })
}

pub fn refresh_shares(request: RefreshSharesRequest) -> Result<RefreshSharesResult, EngineError> {
    record_hardening_telemetry(|telemetry| {
        telemetry.refresh_shares_calls_total =
            telemetry.refresh_shares_calls_total.saturating_add(1);
    });
    let _latency_guard = HardeningOperationLatencyGuard::new(HardeningOperation::RefreshShares);
    enforce_provenance_gate()?;
    validate_session_id(&request.session_id)?;

    log_policy_decision(
        "lifecycle_policy",
        &request.session_id,
        "reject",
        "cryptographic_refresh_not_supported",
    );
    Err(EngineError::CryptographicRefreshNotSupported {
        session_id: request.session_id,
    })
}
