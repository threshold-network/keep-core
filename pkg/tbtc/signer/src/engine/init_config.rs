// Init-time signer configuration: a typed, FFI-installed snapshot that
// replaces the process environment as the source of TBTC_SIGNER_* knobs.
//
// Install guarantee: a candidate config is validated through the same
// loaders the runtime gates use while visible only to the validating
// thread (thread-local override below). It is published to the global
// slot only after validation succeeds, so an unvalidated or rejected
// config can never be observed by any other caller, and init results are
// truthful under concurrent initialization (idempotent success is only
// ever reported against a validated, installed config).

use super::*;
use std::cell::RefCell;
use std::sync::{Arc, RwLock};

static INSTALLED_SIGNER_CONFIG: OnceLock<RwLock<Option<Arc<InstalledSignerConfig>>>> =
    OnceLock::new();
static ENV_FALLBACK_WARNING_EMITTED: OnceLock<()> = OnceLock::new();

thread_local! {
    // Candidate config visible ONLY to the thread running init-time
    // validation. Keeping the candidate off the global slot until it
    // validates means no other caller can ever observe an unvalidated
    // config, and a failed init has no observable side effects.
    static VALIDATION_CANDIDATE: RefCell<Option<Arc<InstalledSignerConfig>>> =
        const { RefCell::new(None) };
}

struct ValidationCandidateGuard;

impl ValidationCandidateGuard {
    fn install(candidate: Arc<InstalledSignerConfig>) -> Self {
        VALIDATION_CANDIDATE.with(|slot| *slot.borrow_mut() = Some(candidate));
        ValidationCandidateGuard
    }
}

impl Drop for ValidationCandidateGuard {
    fn drop(&mut self) {
        VALIDATION_CANDIDATE.with(|slot| *slot.borrow_mut() = None);
    }
}

fn validation_candidate() -> Option<Arc<InstalledSignerConfig>> {
    VALIDATION_CANDIDATE.with(|slot| slot.borrow().clone())
}

pub(crate) struct InstalledSignerConfig {
    pub(crate) values: HashMap<String, String>,
    pub(crate) fingerprint: String,
}

fn installed_signer_config_slot() -> &'static RwLock<Option<Arc<InstalledSignerConfig>>> {
    INSTALLED_SIGNER_CONFIG.get_or_init(|| RwLock::new(None))
}

fn installed_signer_config() -> Option<Arc<InstalledSignerConfig>> {
    installed_signer_config_slot()
        .read()
        .unwrap_or_else(|poisoned| poisoned.into_inner())
        .clone()
}

/// Single chokepoint for every `TBTC_SIGNER_*` operational read.
///
/// With an installed init-time config the process environment is NOT
/// consulted: the snapshot is the sole source of truth and an absent key
/// means the built-in default. Without an installed config this falls
/// through to the process environment (test/development behavior, and the
/// transitional path for hosts that have not adopted the init FFI yet).
///
/// The state-encryption key (`TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX`) is
/// deliberately NOT routed through here: secrets stay on the dedicated
/// env/command key-provider channel and never ride the config FFI.
pub(crate) fn signer_env_var(name: &str) -> Option<String> {
    if let Some(candidate) = validation_candidate() {
        return candidate.values.get(name).cloned();
    }
    if let Some(config) = installed_signer_config() {
        return config.values.get(name).cloned();
    }
    warn_production_env_fallback_once(name);
    std::env::var(name).ok()
}

fn warn_production_env_fallback_once(name: &str) {
    // The production check reads the environment directly: routing it through
    // signer_env_var would recurse, and on this path no config is installed
    // so the environment is the authoritative source anyway.
    if name == TBTC_SIGNER_PROFILE_ENV {
        return;
    }
    ENV_FALLBACK_WARNING_EMITTED.get_or_init(|| {
        let raw = std::env::var(TBTC_SIGNER_PROFILE_ENV).unwrap_or_default();
        let normalized = raw.trim().to_ascii_lowercase();
        if normalized.as_str() != TBTC_SIGNER_PROFILE_DEVELOPMENT {
            eprintln!(
                "warning: TBTC_SIGNER_* knobs are being read from the process \
                 environment; production hosts should install an init-time \
                 config via frost_tbtc_init_signer_config"
            );
        }
    });
}

pub fn init_signer_config(
    request: InitSignerConfigRequest,
) -> Result<InitSignerConfigResult, EngineError> {
    let config_fingerprint = fingerprint(&request)?;
    let values = config_values_from_request(&request)?;
    let configured_key_count = values.len() as u32;
    let candidate = Arc::new(InstalledSignerConfig {
        values,
        fingerprint: config_fingerprint.clone(),
    });

    // Fast path against an already-installed (and therefore already
    // validated) config; the authoritative re-check happens under the write
    // lock below.
    if let Some(existing) = installed_signer_config() {
        return reinit_result(&existing, &config_fingerprint);
    }

    // Fail-closed validation BEFORE anything is published: the candidate is
    // visible only to this thread's loaders via the thread-local override,
    // so no other caller can ever observe an unvalidated config and a failed
    // init leaves prior state (installed config or environment fallback)
    // untouched. Validation runs the same loaders the runtime gates use plus
    // the state-path, key-provider and provenance-gate requirements; knobs the runtime warn-and-defaults on
    // keep that behavior.
    {
        let _candidate_guard = ValidationCandidateGuard::install(Arc::clone(&candidate));
        validate_candidate_config()?;
    }

    // Publish, re-checking under the write lock: two threads may have
    // validated identical (or conflicting) candidates concurrently.
    let mut guard = installed_signer_config_slot()
        .write()
        .unwrap_or_else(|poisoned| poisoned.into_inner());
    if let Some(existing) = guard.as_ref() {
        let existing = Arc::clone(existing);
        drop(guard);
        return reinit_result(&existing, &config_fingerprint);
    }
    *guard = Some(candidate);

    Ok(InitSignerConfigResult {
        installed: true,
        idempotent: false,
        config_fingerprint,
        configured_key_count,
    })
}

fn reinit_result(
    existing: &InstalledSignerConfig,
    config_fingerprint: &str,
) -> Result<InitSignerConfigResult, EngineError> {
    if existing.fingerprint == config_fingerprint {
        return Ok(InitSignerConfigResult {
            installed: true,
            idempotent: true,
            config_fingerprint: config_fingerprint.to_string(),
            configured_key_count: existing.values.len() as u32,
        });
    }
    Err(EngineError::Validation(format!(
        "signer config already installed with fingerprint [{}]; \
         conflicting re-initialization rejected",
        existing.fingerprint
    )))
}

fn validate_candidate_config() -> Result<(), EngineError> {
    load_admission_policy_config()?;
    load_signing_policy_firewall_config()?;
    heartbeat_rate_limit_per_minute()?;
    load_auto_quarantine_config()?;
    // Production (explicit or by profile-omission default) requires an
    // explicit state path; surfacing this at init beats failing the first
    // state access after a host migrates to the config FFI.
    state_file_path()?;
    // The append-only witness has no unsigned/local-only compaction path.
    // Reject unusable ceilings at init instead of discovering them after the
    // signer has begun serving stateful operations.
    state_witness_max_records()?;
    // The key-provider settings must be structurally usable too (production
    // forbids the env provider; the command provider requires a command).
    // Resolved WITHOUT reading the secret or executing the key command.
    resolve_state_key_provider_plan()?;
    // Production forces the provenance gate, so a production config without
    // a complete, verifiable attestation set is unusable for every protected
    // operation - reject it at init. The gate self-gates (no-op when not
    // enforced), reads only candidate values plus local crypto, and runtime
    // calls still re-check it: an init-time pass does not exempt TTL aging.
    enforce_provenance_gate()?;
    Ok(())
}

pub(crate) fn config_values_from_request(
    request: &InitSignerConfigRequest,
) -> Result<HashMap<String, String>, EngineError> {
    let mut values = HashMap::new();

    if let Some(profile) = &request.profile {
        let normalized = profile.trim().to_ascii_lowercase();
        if normalized != TBTC_SIGNER_PROFILE_PRODUCTION
            && normalized != TBTC_SIGNER_PROFILE_DEVELOPMENT
        {
            return Err(EngineError::Validation(format!(
                "profile must be '{}' or '{}'; got [{}]",
                TBTC_SIGNER_PROFILE_PRODUCTION, TBTC_SIGNER_PROFILE_DEVELOPMENT, profile
            )));
        }
        values.insert(TBTC_SIGNER_PROFILE_ENV.to_string(), normalized);
    }

    insert_bool(
        &mut values,
        TBTC_SIGNER_ALLOW_BOOTSTRAP_ENV,
        request.allow_bootstrap,
    );
    insert_bool(
        &mut values,
        TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV,
        request.enable_roast_strict,
    );
    insert_bool(
        &mut values,
        TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK_ENV,
        request.allow_bench_restart_hook,
    );
    insert_bool(
        &mut values,
        TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV,
        request.enforce_provenance_gate,
    );
    insert_bool(
        &mut values,
        TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV,
        request.enforce_admission_policy,
    );
    insert_bool(
        &mut values,
        TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV,
        request.enforce_signing_policy_firewall,
    );
    // Make the emergency plaintext-state rollback opt-in reachable for hosts that
    // configure via init-time config (where signer_env_var reads the installed
    // config, not the process environment), not just raw env.
    insert_bool(
        &mut values,
        TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV,
        request.permit_plaintext_state_rollback,
    );
    insert_bool(
        &mut values,
        TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV,
        request.enable_auto_quarantine,
    );

    insert_u64(
        &mut values,
        TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV,
        request.roast_coordinator_timeout_ms,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV,
        request.refresh_cadence_seconds,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT_ENV,
        request.state_corrupt_backup_limit,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_STATE_WITNESS_MAX_RECORDS_ENV,
        request.state_witness_max_records,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_MAX_SESSIONS_ENV,
        request.max_sessions,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV,
        request.max_live_interactive_sessions,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_INTERACTIVE_SESSION_TTL_SECONDS_ENV,
        request.interactive_session_ttl_seconds,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV,
        request.state_key_command_timeout_secs,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS_ENV,
        request.admission_min_participants,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_ADMISSION_MIN_THRESHOLD_ENV,
        request.admission_min_threshold,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV,
        request.policy_max_output_count,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV,
        request.policy_max_output_value_sats,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV,
        request.policy_max_total_output_value_sats,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV,
        request.policy_rate_limit_per_minute,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV,
        request.policy_heartbeat_rate_limit_per_minute,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV,
        request.auto_quarantine_fault_threshold,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV,
        request.auto_quarantine_timeout_penalty,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV,
        request.auto_quarantine_invalid_share_penalty,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV,
        request.canary_max_start_sign_round_p95_ms,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS_ENV,
        request.canary_max_finalize_sign_round_p95_ms,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV,
        request.canary_max_policy_reject_rate_bps,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND1_P95_MS_ENV,
        request.canary_max_interactive_round1_p95_ms,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND2_P95_MS_ENV,
        request.canary_max_interactive_round2_p95_ms,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_AGGREGATE_P95_MS_ENV,
        request.canary_max_interactive_aggregate_p95_ms,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV,
        request.canary_min_samples,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_CANARY_MIN_POLICY_SAMPLES_ENV,
        request.canary_min_policy_samples,
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_CANARY_MAX_SAMPLE_AGE_SECONDS_ENV,
        request.canary_max_sample_age_seconds,
    );

    insert_u64(
        &mut values,
        TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV,
        request.policy_allowed_utc_start_hour.map(u64::from),
    );
    insert_u64(
        &mut values,
        TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV,
        request.policy_allowed_utc_end_hour.map(u64::from),
    );

    insert_string(&mut values, TBTC_SIGNER_STATE_PATH_ENV, &request.state_path)?;
    insert_string(
        &mut values,
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
        &request.state_corruption_policy,
    )?;
    insert_string(
        &mut values,
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        &request.state_key_provider,
    )?;
    insert_string(
        &mut values,
        TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        &request.state_key_command,
    )?;
    insert_string(
        &mut values,
        TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
        &request.provenance_attestation_status,
    )?;
    insert_string(
        &mut values,
        TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
        &request.provenance_attestation_payload,
    )?;
    insert_string(
        &mut values,
        TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
        &request.provenance_attestation_signature_hex,
    )?;
    insert_string(
        &mut values,
        TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV,
        &request.provenance_trust_root,
    )?;
    insert_string(
        &mut values,
        TBTC_SIGNER_MIN_APPROVED_VERSION_ENV,
        &request.min_approved_version,
    )?;

    insert_identifier_list(
        &mut values,
        TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS_ENV,
        &request.admission_required_identifiers,
    )?;
    insert_identifier_list(
        &mut values,
        TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV,
        &request.admission_allowlist_identifiers,
    )?;
    insert_identifier_list(
        &mut values,
        TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV,
        &request.auto_quarantine_dao_allowlist_identifiers,
    )?;

    if let Some(script_classes) = &request.policy_allowed_script_classes {
        if script_classes.is_empty() || script_classes.iter().any(|class| class.trim().is_empty()) {
            return Err(EngineError::Validation(format!(
                "config field for [{}] must contain at least one non-empty script class when set",
                TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV
            )));
        }
        values.insert(
            TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV.to_string(),
            script_classes.join(","),
        );
    }

    Ok(values)
}

fn insert_bool(values: &mut HashMap<String, String>, key: &str, value: Option<bool>) {
    if let Some(value) = value {
        values.insert(
            key.to_string(),
            if value { "true" } else { "false" }.to_string(),
        );
    }
}

fn insert_u64(values: &mut HashMap<String, String>, key: &str, value: Option<u64>) {
    if let Some(value) = value {
        values.insert(key.to_string(), value.to_string());
    }
}

fn insert_string(
    values: &mut HashMap<String, String>,
    key: &str,
    value: &Option<String>,
) -> Result<(), EngineError> {
    if let Some(value) = value {
        if value.trim().is_empty() {
            return Err(EngineError::Validation(format!(
                "config field for [{}] must not be empty when set",
                key
            )));
        }
        values.insert(key.to_string(), value.clone());
    }
    Ok(())
}

fn insert_identifier_list(
    values: &mut HashMap<String, String>,
    key: &str,
    identifiers: &Option<Vec<u16>>,
) -> Result<(), EngineError> {
    if let Some(identifiers) = identifiers {
        if identifiers.is_empty() {
            return Err(EngineError::Validation(format!(
                "config field for [{}] must contain at least one identifier when set",
                key
            )));
        }
        let mut sorted = identifiers.clone();
        sorted.sort_unstable();
        sorted.dedup();
        values.insert(
            key.to_string(),
            sorted
                .iter()
                .map(|identifier| identifier.to_string())
                .collect::<Vec<_>>()
                .join(","),
        );
    }
    Ok(())
}

#[cfg(test)]
pub(crate) fn clear_installed_signer_config_for_tests() {
    let mut guard = installed_signer_config_slot()
        .write()
        .unwrap_or_else(|poisoned| poisoned.into_inner());
    *guard = None;
}
