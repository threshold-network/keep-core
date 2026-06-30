// TBTC_SIGNER_* environment surface: constant names, defaults, and parsers.

use super::*;

pub(crate) const TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT: &str = "env";

pub(crate) const TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND: &str = "command";

// Env-var selector for key provider implementation (`env` or `command`).
pub(crate) const TBTC_SIGNER_STATE_KEY_PROVIDER_ENV: &str = "TBTC_SIGNER_STATE_KEY_PROVIDER";

pub(crate) const TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX: &str =
    "TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX";

pub(crate) const TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV: &str =
    "TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX";

pub(crate) const TBTC_SIGNER_STATE_KEY_COMMAND_ENV: &str = "TBTC_SIGNER_STATE_KEY_COMMAND";

pub(crate) const TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV: &str =
    "TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS";

pub(crate) const TBTC_SIGNER_DEFAULT_STATE_KEY_COMMAND_TIMEOUT_SECS: u64 = 30;

pub(crate) const TBTC_SIGNER_MIN_STATE_KEY_COMMAND_TIMEOUT_SECS: u64 = 1;

pub(crate) const TBTC_SIGNER_MAX_STATE_KEY_COMMAND_TIMEOUT_SECS: u64 = 300;

pub(crate) const TBTC_SIGNER_PROFILE_ENV: &str = "TBTC_SIGNER_PROFILE";

pub(crate) const TBTC_SIGNER_PROFILE_PRODUCTION: &str = "production";

pub(crate) const TBTC_SIGNER_PROFILE_DEVELOPMENT: &str = "development";

pub(crate) const TBTC_SIGNER_STATE_PATH_ENV: &str = "TBTC_SIGNER_STATE_PATH";

pub(crate) const TBTC_SIGNER_DEFAULT_STATE_FILENAME: &str = "frost_tbtc_engine_state.json";

pub(crate) const TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV: &str =
    "TBTC_SIGNER_STATE_CORRUPTION_POLICY";

pub(crate) const TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET: &str =
    "quarantine_and_reset";

pub(crate) const TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT_ENV: &str =
    "TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT";

pub(crate) const TBTC_SIGNER_DEFAULT_CORRUPT_BACKUP_LIMIT: usize = 5;

pub(crate) const TBTC_SIGNER_MAX_SESSIONS_ENV: &str = "TBTC_SIGNER_MAX_SESSIONS";

pub(crate) const TBTC_SIGNER_DEFAULT_MAX_SESSIONS: usize = 1024;

// Phase 7.1 interactive session bounds. Live interactive sessions hold
// secret nonces in memory, so they get a dedicated, smaller cap than
// the overall session registry, and a TTL after which an abandoned
// attempt's nonces are destroyed (expiry has abort semantics).
pub(crate) const TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV: &str =
    "TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS";

pub(crate) const TBTC_SIGNER_DEFAULT_MAX_LIVE_INTERACTIVE_SESSIONS: usize = 64;

pub(crate) const TBTC_SIGNER_INTERACTIVE_SESSION_TTL_SECONDS_ENV: &str =
    "TBTC_SIGNER_INTERACTIVE_SESSION_TTL_SECONDS";

pub(crate) const TBTC_SIGNER_DEFAULT_INTERACTIVE_SESSION_TTL_SECONDS: u64 = 3600;

pub(crate) const TBTC_SIGNER_STATE_LOCKFILE_SUFFIX: &str = ".lock";

pub(crate) const TBTC_SIGNER_ALLOW_BOOTSTRAP_ENV: &str = "TBTC_SIGNER_ALLOW_BOOTSTRAP";

pub(crate) const TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV: &str = "TBTC_SIGNER_ENABLE_ROAST_STRICT";

pub(crate) const TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK_ENV: &str =
    "TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK";

pub(crate) const TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV: &str =
    "TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS";

pub(crate) const TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS: u64 = 30_000;

pub(crate) const TBTC_SIGNER_MIN_ROAST_COORDINATOR_TIMEOUT_MS: u64 = 1_000;

pub(crate) const TBTC_SIGNER_MAX_ROAST_COORDINATOR_TIMEOUT_MS: u64 = 300_000;

pub(crate) const TBTC_SIGNER_RUNTIME_VERSION: &str = env!("CARGO_PKG_VERSION");

pub(crate) const TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV: &str =
    "TBTC_SIGNER_ENFORCE_PROVENANCE_GATE";

pub(crate) const TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV: &str =
    "TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS";

pub(crate) const TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV: &str =
    "TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD";

pub(crate) const TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV: &str =
    "TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX";

pub(crate) const TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV: &str = "TBTC_SIGNER_PROVENANCE_TRUST_ROOT";

pub(crate) const TBTC_SIGNER_MIN_APPROVED_VERSION_ENV: &str = "TBTC_SIGNER_MIN_APPROVED_VERSION";

pub(crate) const TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED: &str = "approved";

pub(crate) const TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS: u64 = 7 * 24 * 3600;

pub(crate) const TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV: &str =
    "TBTC_SIGNER_ENFORCE_ADMISSION_POLICY";

pub(crate) const TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS_ENV: &str =
    "TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS";

pub(crate) const TBTC_SIGNER_ADMISSION_MIN_THRESHOLD_ENV: &str =
    "TBTC_SIGNER_ADMISSION_MIN_THRESHOLD";

pub(crate) const TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS_ENV: &str =
    "TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS";

pub(crate) const TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV: &str =
    "TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS";

pub(crate) const TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV: &str =
    "TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL";

pub(crate) const TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV: &str =
    "TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK";

pub(crate) const TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV: &str =
    "TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES";

pub(crate) const TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV: &str =
    "TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT";

pub(crate) const TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV: &str =
    "TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS";

pub(crate) const TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV: &str =
    "TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS";

pub(crate) const TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV: &str =
    "TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR";

pub(crate) const TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV: &str =
    "TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR";

pub(crate) const TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV: &str =
    "TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE";

pub(crate) const TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV: &str =
    "TBTC_SIGNER_ENABLE_AUTO_QUARANTINE";

pub(crate) const TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV: &str =
    "TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD";

pub(crate) const TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV: &str =
    "TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY";

pub(crate) const TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV: &str =
    "TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY";

pub(crate) const TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV: &str =
    "TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS";

pub(crate) const TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_FAULT_THRESHOLD: u64 = 3;

pub(crate) const TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_TIMEOUT_PENALTY: u64 = 1;

pub(crate) const TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_INVALID_SHARE_PENALTY: u64 = 2;

pub(crate) const TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV: &str =
    "TBTC_SIGNER_REFRESH_CADENCE_SECONDS";

pub(crate) const TBTC_SIGNER_DEFAULT_REFRESH_CADENCE_SECONDS: u64 = 24 * 60 * 60;

pub(crate) const TBTC_SIGNER_MIN_REFRESH_CADENCE_SECONDS: u64 = 60;

pub(crate) const TBTC_SIGNER_MAX_REFRESH_CADENCE_SECONDS: u64 = 30 * 24 * 60 * 60;

pub(crate) const TBTC_SIGNER_DIFFERENTIAL_FUZZ_MAX_CASES: u32 = 512;

pub(crate) const TBTC_SIGNER_DIFFERENTIAL_FUZZ_DEFAULT_CASES: u32 = 64;

pub(crate) const TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV: &str =
    "TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS";

pub(crate) const TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS_ENV: &str =
    "TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS";

pub(crate) const TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV: &str =
    "TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS";

pub(crate) const TBTC_SIGNER_DEFAULT_CANARY_MAX_START_SIGN_ROUND_P95_MS: u64 = 5_000;

pub(crate) const TBTC_SIGNER_DEFAULT_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS: u64 = 5_000;

pub(crate) const TBTC_SIGNER_DEFAULT_CANARY_MAX_POLICY_REJECT_RATE_BPS: u64 = 1_000;

pub(crate) const TBTC_SIGNER_MAX_POLICY_REJECT_RATE_BPS: u64 = 10_000;

pub(crate) fn roast_coordinator_timeout_ms() -> u64 {
    signer_env_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV)
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|timeout_ms| {
            *timeout_ms >= TBTC_SIGNER_MIN_ROAST_COORDINATOR_TIMEOUT_MS
                && *timeout_ms <= TBTC_SIGNER_MAX_ROAST_COORDINATOR_TIMEOUT_MS
        })
        .unwrap_or(TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS)
}

pub(crate) fn refresh_cadence_seconds() -> u64 {
    signer_env_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV)
        .and_then(|value| value.trim().parse::<u64>().ok())
        .filter(|value| {
            *value >= TBTC_SIGNER_MIN_REFRESH_CADENCE_SECONDS
                && *value <= TBTC_SIGNER_MAX_REFRESH_CADENCE_SECONDS
        })
        .unwrap_or(TBTC_SIGNER_DEFAULT_REFRESH_CADENCE_SECONDS)
}

pub(crate) fn parse_identifier_set_from_env(
    env_name: &str,
) -> Result<Option<HashSet<u16>>, EngineError> {
    let Some(raw_value) = signer_env_var(env_name) else {
        return Ok(None);
    };

    let raw_value = raw_value.trim();
    if raw_value.is_empty() {
        return Err(EngineError::Internal(format!(
            "identifier list env [{}] must be unset or contain at least one identifier",
            env_name
        )));
    }

    let mut identifiers = HashSet::new();
    for token in raw_value.split(',') {
        let token = token.trim();
        if token.is_empty() {
            continue;
        }

        let identifier = token.parse::<u16>().map_err(|_| {
            EngineError::Internal(format!(
                "failed to parse identifier [{}] from env [{}]",
                token, env_name
            ))
        })?;
        if identifier == 0 {
            return Err(EngineError::Internal(format!(
                "identifier list env [{}] contains zero identifier",
                env_name
            )));
        }
        identifiers.insert(identifier);
    }

    Ok(Some(identifiers))
}

pub(crate) fn parse_usize_from_env_with_default(
    env_name: &str,
    default_value: usize,
) -> Result<usize, EngineError> {
    let Some(raw_value) = signer_env_var(env_name) else {
        return Ok(default_value);
    };

    let parsed = raw_value.trim().parse::<usize>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse usize env [{}] value [{}]",
            env_name, raw_value
        ))
    })?;
    Ok(parsed)
}

pub(crate) fn parse_u64_from_env_with_default(
    env_name: &str,
    default_value: u64,
) -> Result<u64, EngineError> {
    let Some(raw_value) = signer_env_var(env_name) else {
        return Ok(default_value);
    };

    let parsed = raw_value.trim().parse::<u64>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse u64 env [{}] value [{}]",
            env_name, raw_value
        ))
    })?;
    Ok(parsed)
}

pub(crate) fn parse_usize_from_env_required(env_name: &str) -> Result<usize, EngineError> {
    let raw_value = signer_env_var(env_name)
        .ok_or_else(|| EngineError::Internal(format!("missing required env [{}]", env_name)))?;
    raw_value.trim().parse::<usize>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse usize env [{}] value [{}]",
            env_name, raw_value
        ))
    })
}

pub(crate) fn parse_u64_from_env_required(env_name: &str) -> Result<u64, EngineError> {
    let raw_value = signer_env_var(env_name)
        .ok_or_else(|| EngineError::Internal(format!("missing required env [{}]", env_name)))?;
    raw_value.trim().parse::<u64>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse u64 env [{}] value [{}]",
            env_name, raw_value
        ))
    })
}

pub(crate) fn parse_u8_from_env_optional(env_name: &str) -> Result<Option<u8>, EngineError> {
    let Some(raw_value) = signer_env_var(env_name) else {
        return Ok(None);
    };

    let parsed = raw_value.trim().parse::<u8>().map_err(|_| {
        EngineError::Internal(format!(
            "failed to parse u8 env [{}] value [{}]",
            env_name, raw_value
        ))
    })?;
    if parsed > 23 {
        return Err(EngineError::Internal(format!(
            "hour env [{}] must be in range 0..=23, got [{}]",
            env_name, parsed
        )));
    }
    Ok(Some(parsed))
}

pub(crate) fn parse_script_class_set_required(
    env_name: &str,
) -> Result<HashSet<String>, EngineError> {
    let raw_value = signer_env_var(env_name)
        .ok_or_else(|| EngineError::Internal(format!("missing required env [{}]", env_name)))?;
    let raw_value = raw_value.trim();
    if raw_value.is_empty() {
        return Err(EngineError::Internal(format!(
            "required env [{}] must not be empty",
            env_name
        )));
    }

    let mut script_classes = HashSet::new();
    for token in raw_value.split(',') {
        let normalized = token.trim().to_ascii_lowercase();
        if normalized.is_empty() {
            continue;
        }
        script_classes.insert(normalized);
    }

    if script_classes.is_empty() {
        return Err(EngineError::Internal(format!(
            "required env [{}] produced an empty script class set",
            env_name
        )));
    }

    Ok(script_classes)
}

pub(crate) fn truthy_env_flag(raw_value: &str) -> bool {
    matches!(
        raw_value.trim().to_ascii_lowercase().as_str(),
        "1" | "true" | "yes" | "on"
    )
}

pub(crate) fn roast_strict_mode_enabled() -> bool {
    if signer_profile_is_production() {
        return true;
    }

    signer_env_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

#[cfg(any(test, feature = "bench-restart-hook"))]
pub(crate) fn bench_restart_hook_enabled() -> bool {
    signer_env_var(TBTC_SIGNER_ALLOW_BENCH_RESTART_HOOK_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

pub(crate) fn signer_profile_is_production() -> bool {
    let raw = signer_env_var(TBTC_SIGNER_PROFILE_ENV).unwrap_or_default();
    let normalized = raw.trim().to_ascii_lowercase();
    match normalized.as_str() {
        TBTC_SIGNER_PROFILE_PRODUCTION | "" => true,
        TBTC_SIGNER_PROFILE_DEVELOPMENT => false,
        other => panic!(
            "{} must be '{}' or '{}'; got {:?}",
            TBTC_SIGNER_PROFILE_ENV,
            TBTC_SIGNER_PROFILE_PRODUCTION,
            TBTC_SIGNER_PROFILE_DEVELOPMENT,
            other
        ),
    }
}
