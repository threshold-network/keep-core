#[cfg(any(test, feature = "bench-restart-hook"))]
use super::envelope_io::load_engine_state_from_storage;
/// Pending-operation registry: marker tracking, snapshot covering, durable
/// retry on next state-lock acquisition. Also owns the test-only
/// `reload_state_from_storage_for_benchmarks` hook which clears the registry
/// before reloading. Moved from `persistence.rs` as part of the C2
/// persistence-deepening refactor.
use super::*;
#[cfg(any(test, feature = "bench-restart-hook"))]
use crate::engine::config::bench_restart_hook_enabled;

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum PersistencePendingOperation {
    BuildTaprootTx {
        session_id: String,
        request_fingerprint: String,
    },
    EmergencyRekey {
        result: TriggerEmergencyRekeyResult,
    },
    CanaryPromotion {
        result: PromoteCanaryResult,
    },
    CanaryRollback {
        result: RollbackCanaryResult,
    },
    InteractiveRound2 {
        session_id: String,
        consumed_marker: String,
    },
    InteractiveAggregate {
        session_id: String,
        aggregated_marker: String,
    },
    InteractiveState {
        session_id: String,
    },
}

static PERSISTENCE_PENDING_OPERATIONS: OnceLock<Mutex<Vec<PersistencePendingOperation>>> =
    OnceLock::new();

fn persistence_pending_operations() -> &'static Mutex<Vec<PersistencePendingOperation>> {
    PERSISTENCE_PENDING_OPERATIONS.get_or_init(|| Mutex::new(Vec::new()))
}

fn persistence_pending_same_slot(
    existing: &PersistencePendingOperation,
    replacement: &PersistencePendingOperation,
) -> bool {
    match (existing, replacement) {
        (
            PersistencePendingOperation::BuildTaprootTx {
                session_id: existing,
                ..
            },
            PersistencePendingOperation::BuildTaprootTx {
                session_id: replacement,
                ..
            },
        ) => existing == replacement,
        (
            PersistencePendingOperation::EmergencyRekey { result: existing },
            PersistencePendingOperation::EmergencyRekey {
                result: replacement,
            },
        ) => existing.session_id == replacement.session_id,
        (
            PersistencePendingOperation::CanaryPromotion { .. }
            | PersistencePendingOperation::CanaryRollback { .. },
            PersistencePendingOperation::CanaryPromotion { .. }
            | PersistencePendingOperation::CanaryRollback { .. },
        ) => true,
        (
            PersistencePendingOperation::InteractiveRound2 {
                session_id: existing_session,
                consumed_marker: existing_marker,
            },
            PersistencePendingOperation::InteractiveRound2 {
                session_id: replacement_session,
                consumed_marker: replacement_marker,
            },
        ) => existing_session == replacement_session && existing_marker == replacement_marker,
        (
            PersistencePendingOperation::InteractiveAggregate {
                session_id: existing_session,
                aggregated_marker: existing_marker,
            },
            PersistencePendingOperation::InteractiveAggregate {
                session_id: replacement_session,
                aggregated_marker: replacement_marker,
            },
        ) => existing_session == replacement_session && existing_marker == replacement_marker,
        (
            PersistencePendingOperation::InteractiveState {
                session_id: existing_session,
            },
            PersistencePendingOperation::InteractiveState {
                session_id: replacement_session,
            },
        ) => existing_session == replacement_session,
        _ => false,
    }
}

pub(crate) fn mark_persistence_pending(operation: PersistencePendingOperation) {
    let mut pending = persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner);
    pending.retain(|existing| !persistence_pending_same_slot(existing, &operation));
    pending.push(operation);
}

#[cfg(any(test, feature = "bench-restart-hook"))]
pub(crate) fn clear_persistence_pending_operations() {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .clear();
}

pub(crate) fn clear_persistence_pending_operation(operation: &PersistencePendingOperation) {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .retain(|pending| pending != operation);
}

pub(crate) fn persistence_pending_session_ids() -> HashSet<String> {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .filter_map(|operation| match operation {
            PersistencePendingOperation::BuildTaprootTx { session_id, .. }
            | PersistencePendingOperation::InteractiveRound2 { session_id, .. }
            | PersistencePendingOperation::InteractiveAggregate { session_id, .. }
            | PersistencePendingOperation::InteractiveState { session_id } => {
                Some(session_id.clone())
            }
            PersistencePendingOperation::EmergencyRekey { result } => {
                Some(result.session_id.clone())
            }
            PersistencePendingOperation::CanaryPromotion { .. }
            | PersistencePendingOperation::CanaryRollback { .. } => None,
        })
        .collect()
}

pub(crate) fn clear_snapshot_covered_operations(engine_state: &EngineState) {
    // Round2/Aggregate pending entries cache no result. Clear one only when the
    // successful snapshot actually contains its fail-closed marker; merely
    // writing some other snapshot must never erase a repair obligation.
    // InteractiveState carries no replay marker, so any snapshot still containing
    // its protected session covers the binding/retirement state that was uncertain
    // after an Open, Abort, or expiry write replaced the file.
    // Lifecycle/build/refresh entries additionally preserve the original
    // operation result, so keep those until that caller retries (one bounded
    // slot per session, plus one canary slot).
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .retain(|pending| match pending {
            PersistencePendingOperation::InteractiveRound2 {
                session_id,
                consumed_marker,
            } => !engine_state
                .sessions
                .get(session_id)
                .is_some_and(|session| {
                    session
                        .interactive
                        .consumed_attempt_markers
                        .contains(consumed_marker)
                }),
            PersistencePendingOperation::InteractiveAggregate {
                session_id,
                aggregated_marker,
            } => !engine_state
                .sessions
                .get(session_id)
                .is_some_and(|session| {
                    session
                        .interactive
                        .aggregated_attempt_markers
                        .contains(aggregated_marker)
                }),
            PersistencePendingOperation::InteractiveState { session_id } => {
                !engine_state.sessions.contains_key(session_id)
            }
            _ => true,
        });
}

pub(crate) fn pending_build_taproot_tx_operation(
    session_id: &str,
) -> Option<PersistencePendingOperation> {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .find(|operation| {
            matches!(
                operation,
                PersistencePendingOperation::BuildTaprootTx {
                    session_id: pending_session,
                    ..
                } if pending_session == session_id
            )
        })
        .cloned()
}

pub(crate) fn pending_emergency_rekey_operation(
    session_id: &str,
) -> Option<PersistencePendingOperation> {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .find(|operation| match operation {
            PersistencePendingOperation::EmergencyRekey { result } => {
                result.session_id == session_id
            }
            _ => false,
        })
        .cloned()
}

#[cfg(test)]
pub(crate) fn pending_emergency_rekey_result(
    session_id: &str,
    reason: &str,
) -> Option<TriggerEmergencyRekeyResult> {
    match pending_emergency_rekey_operation(session_id) {
        Some(PersistencePendingOperation::EmergencyRekey { result }) if result.reason == reason => {
            Some(result)
        }
        _ => None,
    }
}

pub(crate) fn pending_canary_operation() -> Option<PersistencePendingOperation> {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .find(|operation| {
            matches!(
                operation,
                PersistencePendingOperation::CanaryPromotion { .. }
                    | PersistencePendingOperation::CanaryRollback { .. }
            )
        })
        .cloned()
}

#[cfg(test)]
pub(crate) fn pending_canary_promotion_result(target_percent: u8) -> Option<PromoteCanaryResult> {
    match pending_canary_operation() {
        Some(PersistencePendingOperation::CanaryPromotion { result })
            if result.to_percent == target_percent =>
        {
            Some(result)
        }
        _ => None,
    }
}

#[cfg(test)]
pub(crate) fn pending_canary_rollback_result(reason: &str) -> Option<RollbackCanaryResult> {
    match pending_canary_operation() {
        Some(PersistencePendingOperation::CanaryRollback { result }) if result.reason == reason => {
            Some(result)
        }
        _ => None,
    }
}

pub(crate) fn interactive_round2_persistence_pending(
    session_id: &str,
    consumed_marker: &str,
) -> bool {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .any(|operation| {
            matches!(
                operation,
                PersistencePendingOperation::InteractiveRound2 {
                    session_id: pending_session,
                    consumed_marker: pending_marker,
                } if pending_session == session_id && pending_marker == consumed_marker
            )
        })
}

pub(crate) fn interactive_aggregate_persistence_pending(
    session_id: &str,
    aggregated_marker: &str,
) -> bool {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .any(|operation| {
            matches!(
                operation,
                PersistencePendingOperation::InteractiveAggregate {
                    session_id: pending_session,
                    aggregated_marker: pending_marker,
                } if pending_session == session_id && pending_marker == aggregated_marker
            )
        })
}

pub(crate) fn interactive_state_persistence_pending() -> bool {
    persistence_pending_operations()
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner)
        .iter()
        .any(|operation| {
            matches!(
                operation,
                PersistencePendingOperation::InteractiveState { .. }
            )
        })
}

#[cfg(any(test, feature = "bench-restart-hook"))]
pub fn reload_state_from_storage_for_benchmarks() -> Result<(), EngineError> {
    if !bench_restart_hook_enabled() {
        return Err(EngineError::Validation(format!(
            "benchmark restart hook disabled; set {}=true to enable",
            TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK_ENV
        )));
    }

    if let Ok(mut lock_slot) = state_file_lock_slot().lock() {
        *lock_slot = None;
    }
    clear_persistence_pending_operations();
    ensure_state_file_lock()?;

    let loaded_state = load_engine_state_from_storage()?;
    let state = state()?;
    let mut guard = state
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    *guard = loaded_state;
    Ok(())
}
