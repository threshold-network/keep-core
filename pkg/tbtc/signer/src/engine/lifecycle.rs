// Operational lifecycle: canary rollout, refresh cadence/shares, emergency rekey, quarantine status.

use super::*;

/// Upper bound on per-session `refresh_history` length. Older records are
/// dropped once this is exceeded, bounding persisted-state size for a long-lived
/// / frequently-refreshed session. Also bounds the stale-fingerprint detection
/// window (retries older than this many refreshes are no longer recognized).
const MAX_REFRESH_HISTORY: usize = 256;

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
    session
        .dkg_result
        .as_ref()
        .map(|result| result.key_group.clone())
        .or_else(|| {
            session
                .refresh_history
                .iter()
                .find_map(|record| record.key_group.clone())
        })
}

pub(crate) fn refresh_history_continuity_preserved(session: &SessionState) -> bool {
    let mut last_refresh_epoch = 0_u64;
    let mut reference_key_group: Option<&str> = None;

    for refresh_record in &session.refresh_history {
        if refresh_record.refresh_epoch == 0 || refresh_record.refresh_epoch <= last_refresh_epoch {
            return false;
        }
        last_refresh_epoch = refresh_record.refresh_epoch;

        if let Some(record_key_group) = refresh_record.key_group.as_deref() {
            if let Some(reference_key_group) = reference_key_group {
                if !record_key_group.eq_ignore_ascii_case(reference_key_group) {
                    return false;
                }
            } else {
                reference_key_group = Some(record_key_group);
            }
        }
    }

    true
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
    let last_refresh_record = session.refresh_history.last();
    let now = now_unix();
    let next_refresh_due_unix = last_refresh_record
        .map(|record| record.refreshed_at_unix.saturating_add(cadence_seconds))
        .unwrap_or_else(|| now.saturating_add(cadence_seconds));
    let overdue = now > next_refresh_due_unix;
    let continuity_reference_key_group = refresh_continuity_reference_key_group(session);
    let emergency_rekey_reason = session
        .emergency_rekey_event
        .as_ref()
        .map(|event| event.reason.clone());

    Ok(RefreshCadenceStatusResult {
        session_id: request.session_id,
        refresh_count: session.refresh_count,
        last_refresh_epoch: last_refresh_record
            .map(|record| record.refresh_epoch)
            .unwrap_or(0),
        cadence_seconds,
        next_refresh_due_unix,
        overdue,
        continuity_preserved: refresh_history_continuity_preserved(session),
        continuity_reference_key_group,
        emergency_rekey_required: session.emergency_rekey_event.is_some(),
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
            if session.dkg_result.is_some() {
                None
            } else {
                session.bound_key_group.clone()
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
    if session.emergency_rekey_event.is_some() {
        return Err(EngineError::Validation(format!(
            "emergency rekey already triggered for session [{target_session_id}]; event is immutable"
        )));
    }
    let triggered_at_unix = now_unix();
    let previous_emergency_rekey_event =
        session.emergency_rekey_event.replace(EmergencyRekeyEvent {
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
            rollback_session.emergency_rekey_event = previous_emergency_rekey_event;
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
        clear_persistence_pending_operation(&pending_operation);
        reset_canary_promotion_evidence();
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
        clear_persistence_pending_operation(&pending_operation);
        reset_canary_promotion_evidence();
        if let Some(result) = matching_result {
            return Ok(result);
        }
    }
    let previous_canary_rollout = guard.canary_rollout.clone();
    let from_percent = guard.canary_rollout.current_percent;
    let to_percent = guard.canary_rollout.previous_percent.min(from_percent);
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

    if request.current_shares.is_empty() {
        return Err(EngineError::Validation(
            "current_shares must not be empty".to_string(),
        ));
    }
    let mut unique_share_identifiers = HashSet::new();
    for share in &request.current_shares {
        if share.identifier == 0 {
            return Err(EngineError::Validation(
                "current_shares identifiers must be non-zero".to_string(),
            ));
        }
        if !unique_share_identifiers.insert(share.identifier) {
            return Err(EngineError::Validation(format!(
                "current_shares contains duplicate identifier [{}]",
                share.identifier
            )));
        }
    }

    let request_fingerprint = fingerprint(&canonicalize_refresh_shares_request_for_fingerprint(
        &request,
    ))?;
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;

    if let Some(pending_operation) = pending_refresh_operation(&request.session_id) {
        persist_engine_state_to_storage(&guard)
            .map_err(PersistEngineStateError::into_engine_error)?;
        clear_persistence_pending_operation(&pending_operation);
    }

    if let Some(emergency_rekey_event) = guard
        .sessions
        .get(&request.session_id)
        .and_then(|session| session.emergency_rekey_event.as_ref())
    {
        return Err(EngineError::LifecyclePolicyRejected {
            session_id: request.session_id.clone(),
            reason_code: "emergency_rekey_required".to_string(),
            detail: format!(
                "refresh blocked: emergency rekey required since [{}]: {}",
                emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
            ),
        });
    }

    if let Some(session) = guard.sessions.get(&request.session_id) {
        if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
            return Err(EngineError::LifecyclePolicyRejected {
                session_id: request.session_id.clone(),
                reason_code: "emergency_rekey_required".to_string(),
                detail: format!(
                    "refresh blocked: emergency rekey required since [{}]: {}",
                    emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
                ),
            });
        }

        if let Some(existing) = &session.refresh_request_fingerprint {
            if existing == &request_fingerprint {
                // Idempotent replay of the *same* (most-recent) refresh request:
                // return the cached result.
                return session
                    .refresh_result
                    .clone()
                    .ok_or_else(|| EngineError::Internal("missing refresh cache".to_string()));
            }

            // A fingerprint we have already accepted before (but which is no
            // longer the most recent) is a stale / out-of-order retry, not a new
            // refresh. Reject it rather than re-deriving the older share set and
            // bumping the epoch forward, which would roll the session back behind
            // a newer refresh. A genuinely new fingerprint falls through to
            // perform the refresh (supporting repeatable periodic reshares).
            if session.refresh_history.iter().any(|record| {
                record.request_fingerprint.as_deref() == Some(request_fingerprint.as_str())
            }) {
                return Err(EngineError::SessionConflict {
                    session_id: request.session_id.clone(),
                });
            }
        }
    }
    ensure_session_insert_capacity(&guard.sessions, &request.session_id)?;

    let refresh_session_id = request.session_id.clone();
    let accepted_request_fingerprint = request_fingerprint.clone();
    let previous_refresh_epoch_counter = guard.refresh_epoch_counter;
    let previous_refresh_state = guard.sessions.get(&refresh_session_id).map(|session| {
        (
            session.refresh_request_fingerprint.clone(),
            session.refresh_result.clone(),
            session.refresh_history.clone(),
            session.refresh_count,
        )
    });

    let mut new_shares: Vec<ShareMaterial> = request
        .current_shares
        .into_iter()
        .map(|share| ShareMaterial {
            identifier: share.identifier,
            encrypted_share_hex: hash_hex(
                format!(
                    "refresh:{}:{}:{}",
                    request.session_id, share.identifier, share.encrypted_share_hex
                )
                .as_bytes(),
            ),
        })
        .collect();

    new_shares.sort_by_key(|share| share.identifier);

    guard.refresh_epoch_counter = guard.refresh_epoch_counter.saturating_add(1);
    let refresh_epoch = guard.refresh_epoch_counter;

    let result = RefreshSharesResult {
        session_id: request.session_id,
        refresh_epoch,
        new_shares,
    };

    let session = guard
        .sessions
        .entry(result.session_id.clone())
        .or_insert_with(SessionState::default);
    if let Some(emergency_rekey_event) = session.emergency_rekey_event.as_ref() {
        return Err(EngineError::LifecyclePolicyRejected {
            session_id: result.session_id.clone(),
            reason_code: "emergency_rekey_required".to_string(),
            detail: format!(
                "refresh blocked: emergency rekey required since [{}]: {}",
                emergency_rekey_event.triggered_at_unix, emergency_rekey_event.reason
            ),
        });
    }
    // Preserve the previously-accepted fingerprint before overwriting it. If the
    // last accepted refresh predates RefreshHistoryRecord.request_fingerprint
    // (loaded from legacy state, where history records deserialize with None), its
    // fingerprint lives only in refresh_request_fingerprint; backfill it onto the
    // most-recent history record so a delayed retry of it is still recognized as
    // stale instead of being re-executed as a new refresh.
    if let Some(previous_fingerprint) = session.refresh_request_fingerprint.clone() {
        let already_tracked = session.refresh_history.iter().any(|record| {
            record.request_fingerprint.as_deref() == Some(previous_fingerprint.as_str())
        });
        if !already_tracked {
            if let Some(last) = session.refresh_history.last_mut() {
                if last.request_fingerprint.is_none() {
                    last.request_fingerprint = Some(previous_fingerprint);
                }
            } else {
                // Legacy/degraded state can carry a fingerprint with an EMPTY
                // history (refresh_history postdates refresh_request_fingerprint),
                // so there is no record to backfill onto. Synthesize one carrying
                // the fingerprint so a delayed retry is still recognized for
                // stale-retry rejection instead of being re-executed as a new
                // refresh (which would advance the epoch). Prefer the cached
                // result's epoch/share_count; when the result is absent (a
                // truncated/legacy blob that kept only the fingerprint, or a
                // corrupt state where refresh_result deserialized to None) fall
                // back to an epoch one below the new refresh so the history stays
                // strictly increasing, and a zero share_count. refresh_epoch_counter
                // is persisted, so a prior accepted refresh implies refresh_epoch >= 2
                // and the fallback stays non-zero.
                let previous_result = session.refresh_result.clone();
                let synthesized_epoch = previous_result
                    .as_ref()
                    .map(|previous| previous.refresh_epoch)
                    .filter(|&epoch| epoch != 0 && epoch < refresh_epoch)
                    .unwrap_or_else(|| refresh_epoch.saturating_sub(1).max(1));
                let synthesized_share_count = previous_result
                    .as_ref()
                    .map(|previous| previous.new_shares.len().min(u16::MAX as usize) as u16)
                    .unwrap_or(0);
                session.refresh_history.push(RefreshHistoryRecord {
                    refresh_epoch: synthesized_epoch,
                    refreshed_at_unix: now_unix(),
                    share_count: synthesized_share_count,
                    key_group: session.dkg_result.as_ref().map(|dkg| dkg.key_group.clone()),
                    request_fingerprint: Some(previous_fingerprint),
                });
            }
        }
    }
    // Monotonic total refresh count, independent of refresh_history pruning;
    // backfilled from the retained history length for sessions written before
    // this field existed.
    session.refresh_count = session
        .refresh_count
        .max(session.refresh_history.len() as u64)
        .saturating_add(1);
    session.refresh_request_fingerprint = Some(request_fingerprint.clone());
    session.refresh_result = Some(result.clone());
    session.refresh_history.push(RefreshHistoryRecord {
        refresh_epoch,
        refreshed_at_unix: now_unix(),
        share_count: result.new_shares.len().min(u16::MAX as usize) as u16,
        key_group: session.dkg_result.as_ref().map(|dkg| dkg.key_group.clone()),
        request_fingerprint: Some(request_fingerprint),
    });
    // Bound per-session history growth (state-at-rest size + stale-detection
    // window). Keep the most recent records; epochs stay strictly increasing so
    // refresh_history_continuity_preserved still holds.
    if session.refresh_history.len() > MAX_REFRESH_HISTORY {
        let excess = session.refresh_history.len() - MAX_REFRESH_HISTORY;
        session.refresh_history.drain(0..excess);
    }
    if let Err(persist_error) = persist_engine_state_to_storage(&guard) {
        let state_file_replaced = persist_error.state_file_replaced();
        let persist_error = persist_error.into_engine_error();
        if state_file_replaced {
            mark_persistence_pending(PersistencePendingOperation::RefreshShares {
                session_id: refresh_session_id.clone(),
                request_fingerprint: accepted_request_fingerprint,
            });
        } else {
            guard.refresh_epoch_counter = previous_refresh_epoch_counter;
            if let Some((request_fingerprint, result, history, count)) = previous_refresh_state {
                let rollback_session =
                    guard.sessions.get_mut(&refresh_session_id).ok_or_else(|| {
                        EngineError::Internal(format!(
                            "refresh session [{refresh_session_id}] disappeared while rolling back a failed persist: {persist_error}"
                        ))
                    })?;
                rollback_session.refresh_request_fingerprint = request_fingerprint;
                rollback_session.refresh_result = result;
                rollback_session.refresh_history = history;
                rollback_session.refresh_count = count;
            } else {
                guard.sessions.remove(&refresh_session_id);
            }
        }
        return Err(persist_error);
    }
    record_hardening_telemetry(|telemetry| {
        telemetry.refresh_shares_success_total =
            telemetry.refresh_shares_success_total.saturating_add(1);
    });

    Ok(result)
}
