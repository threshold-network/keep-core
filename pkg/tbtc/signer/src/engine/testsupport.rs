// Cross-module test helpers (cfg(test)): state lock, reset, restart simulation.

use super::*;

#[cfg(test)]
pub(crate) static TEST_MUTEX: OnceLock<Mutex<()>> = OnceLock::new();

#[cfg(test)]
pub fn lock_test_state() -> std::sync::MutexGuard<'static, ()> {
    let guard = TEST_MUTEX
        .get_or_init(|| Mutex::new(()))
        .lock()
        .expect("test lock should not be poisoned");
    // Pin the signer profile to development at lock acquisition. Tests that
    // need to exercise production-mode behavior set the env explicitly after
    // taking the lock; doing this here prevents one test's `set_var` from
    // leaking into the next locked test's body and (for example) routing the
    // encrypted-state-envelope proptest into the production-rejects-env-key-
    // provider gate that #414 introduced.
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_DEVELOPMENT);
    guard
}

#[cfg(test)]
pub fn reset_for_tests() {
    clear_persist_fault_injection_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
    );
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV);
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV);
    // Tests default to the explicit development profile so the production-safe
    // missing-env default does not route unrelated tests through production
    // policy gates.
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_DEVELOPMENT);
    std::env::set_var(
        TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
        TEST_STATE_ENCRYPTION_KEY_HEX,
    );

    if let Ok(mut lock_slot) = state_file_lock_slot().lock() {
        *lock_slot = None;
    }
    if let Ok(mut telemetry) = hardening_telemetry_state().lock() {
        *telemetry = HardeningTelemetryState::default();
    }
    if let Ok(mut limiter) = build_tx_rate_limiter_state().lock() {
        *limiter = BuildTxRateLimiterState::default();
    }

    if let Ok(state) = state() {
        if let Ok(mut guard) = state.lock() {
            guard.sessions.clear();
            guard.refresh_epoch_counter = 0;
            guard.operator_fault_scores.clear();
            guard.quarantined_operator_identifiers.clear();
            guard.canary_rollout = CanaryRolloutState::default();
            let _ = persist_engine_state_to_storage(&guard);
        }
    }
}

#[cfg(test)]
pub fn reload_state_from_storage_for_tests() {
    let loaded_state = load_engine_state_from_storage().expect("load engine state from storage");
    let state = state().expect("engine state should initialize");
    let mut guard = state.lock().expect("engine lock");
    *guard = loaded_state;
}

#[cfg(test)]
pub fn simulate_process_restart_for_tests() {
    if let Ok(mut lock_slot) = state_file_lock_slot().lock() {
        *lock_slot = None;
    }

    if let Some(state) = ENGINE_STATE.get() {
        if let Ok(mut guard) = state.lock() {
            guard.sessions.clear();
            guard.refresh_epoch_counter = 0;
            guard.operator_fault_scores.clear();
            guard.quarantined_operator_identifiers.clear();
            guard.canary_rollout = CanaryRolloutState::default();
        }
    }
}
