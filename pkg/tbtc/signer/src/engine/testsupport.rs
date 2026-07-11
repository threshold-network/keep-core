// Cross-module test helpers (cfg(test)): state lock, reset, restart simulation.

use super::*;

#[cfg(test)]
pub(crate) static TEST_MUTEX: OnceLock<Mutex<()>> = OnceLock::new();

#[cfg(test)]
pub fn lock_test_state() -> std::sync::MutexGuard<'static, ()> {
    // Recover from poisoning rather than propagating it. The guarded
    // value is `()` - the mutex only serializes tests, it protects no
    // invariant - so a test that panics while holding it leaves nothing
    // corrupt. Without this, that panic poisons the mutex and every
    // subsequent test's lock acquisition panics too, turning one real
    // failure into a cascade of dozens that masks the original (and even
    // makes proptest record spurious regression seeds). Each test calls
    // reset_for_tests() next to clear engine state.
    let guard = TEST_MUTEX
        .get_or_init(|| Mutex::new(()))
        .lock()
        .unwrap_or_else(|poisoned| poisoned.into_inner());
    establish_clean_signer_test_env();
    guard
}

// Reset the process to a clean, hermetic TBTC_SIGNER_* environment at
// every test-lock acquisition. Any TBTC_SIGNER_* var a prior test set
// is removed - even if that test panicked before its own cleanup ran -
// so one test's `set_var` (firewall/quarantine/cap/policy toggles, etc.)
// cannot leak into the next locked test. The three baseline vars the
// engine needs to function in tests are then re-established (profile,
// state-key provider, state-encryption key); tests that need a
// production profile or other toggles set them explicitly after taking
// the lock. This centralizes leak-prevention so individual tests can use
// raw set_var without per-site teardown.
#[cfg(test)]
pub(crate) fn establish_clean_signer_test_env() {
    // Iterate with vars_os, not vars: std::env::vars panics if ANY env
    // var in the process (name or value) is not valid UTF-8 - even one
    // unrelated to the signer - which would abort every locked test in
    // such an environment. TBTC_SIGNER_* names are ASCII, so a key that
    // fails to_str cannot be one of ours.
    for (key_os, _) in std::env::vars_os() {
        if let Some(key) = key_os.to_str() {
            if key.starts_with("TBTC_SIGNER_") {
                std::env::remove_var(&key_os);
            }
        }
    }
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_DEVELOPMENT);
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
        TEST_STATE_ENCRYPTION_KEY_HEX,
    );
}

#[cfg(test)]
pub fn reset_for_tests() {
    clear_installed_signer_config_for_tests();
    clear_persist_fault_injection_for_tests();
    clear_persistence_pending_operations();
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
        *limiter = PolicyRateLimiterState::default();
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
    clear_persistence_pending_operations();
    let loaded_state = load_engine_state_from_storage().expect("load engine state from storage");
    let state = state().expect("engine state should initialize");
    let mut guard = state.lock().expect("engine lock");
    *guard = loaded_state;
}

#[cfg(test)]
pub fn simulate_process_restart_for_tests() {
    clear_persistence_pending_operations();
    if let Ok(mut telemetry) = hardening_telemetry_state().lock() {
        *telemetry = HardeningTelemetryState::default();
    }
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
