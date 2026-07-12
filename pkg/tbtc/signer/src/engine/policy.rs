// Admission, signing-policy firewall, rate limiting, and auto-quarantine enforcement.

use super::*;

pub(crate) const BITCOIN_MAX_MONEY_SATS: u64 = 2_100_000_000_000_000;

/// Conservative built-in signing-policy-firewall defaults, applied when the
/// firewall is enforced (always in production, see `signing_policy_firewall_enforced`)
/// but the operator has not set explicit policy env. The script-class allowlist is
/// the meaningful default: it fails closed on any non-standard output form
/// (`classify_script_pubkey` returns "other"), which is the on-signer guard against
/// an authorized coordinator getting an unusual/unauthorized script signed. The
/// numeric caps default to permissive bounds (operators tighten them per wallet
/// sizing) -- a too-tight static cap would false-reject legitimate large
/// redemptions/sweeps. Transaction signing remains bound to a policy-checked
/// build artifact; the only non-transaction alternative is the narrow heartbeat
/// intent validated independently at Open and Round2.
pub(crate) const DEFAULT_ALLOWED_SCRIPT_CLASSES: &[&str] =
    &["p2pkh", "p2sh", "p2wpkh", "p2wsh", "p2tr"];
pub(crate) const DEFAULT_MAX_OUTPUT_COUNT: usize = 10_000;

pub(crate) static POLICY_GATE_WARNING_EMITTED: OnceLock<()> = OnceLock::new();

pub(crate) static BUILD_TX_RATE_LIMITER: OnceLock<Mutex<PolicyRateLimiterState>> = OnceLock::new();

pub(crate) const BUILD_TX_RATE_LIMIT_TOKEN_SCALE: u128 = 1_000_000;

pub(crate) const BUILD_TX_RATE_LIMIT_SECONDS_PER_MINUTE: u128 = 60;

#[derive(Default)]
pub(crate) struct PolicyRateLimiterState {
    pub(crate) last_refill_unix: u64,
    pub(crate) token_microunits: u128,
    pub(crate) configured_rate_limit_per_minute: u64,
}

#[derive(Clone, Debug)]
pub(crate) struct AdmissionPolicyConfig {
    pub(crate) min_participants: usize,
    pub(crate) min_threshold: u16,
    pub(crate) required_identifiers: HashSet<u16>,
    pub(crate) allowlist_identifiers: Option<HashSet<u16>>,
}

#[derive(Clone, Debug)]
pub(crate) struct SigningPolicyFirewallConfig {
    pub(crate) allowed_script_classes: HashSet<String>,
    pub(crate) max_output_count: usize,
    pub(crate) max_output_value_sats: u64,
    pub(crate) max_total_output_value_sats: u64,
    pub(crate) allowed_utc_start_hour: Option<u8>,
    pub(crate) allowed_utc_end_hour: Option<u8>,
    pub(crate) rate_limit_per_minute: u64,
}

#[derive(Clone, Debug)]
pub(crate) struct AutoQuarantineConfig {
    pub(crate) fault_threshold: u64,
    pub(crate) dao_allowlist_identifiers: HashSet<u16>,
}

pub(crate) fn build_tx_rate_limiter_state() -> &'static Mutex<PolicyRateLimiterState> {
    BUILD_TX_RATE_LIMITER.get_or_init(|| Mutex::new(PolicyRateLimiterState::default()))
}

pub(crate) fn provenance_gate_enforced() -> bool {
    if signer_profile_is_production() {
        return true;
    }

    signer_env_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

pub(crate) fn admission_policy_enforced() -> bool {
    signer_env_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

pub(crate) fn signing_policy_firewall_enforced() -> bool {
    // Mirror provenance_gate_enforced(): the signing-policy firewall is always
    // enforced in production. It resolves to conservative built-in policy
    // defaults (see load_signing_policy_firewall_config) so production does not
    // depend on every operator shipping explicit policy config -- closing the
    // fail-open default without making firewall config mandatory to boot.
    if signer_profile_is_production() {
        return true;
    }

    signer_env_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

pub(crate) fn warn_disabled_policy_gates() {
    POLICY_GATE_WARNING_EMITTED.get_or_init(|| {
        if !provenance_gate_enforced() {
            eprintln!(
                "warning: provenance gate is DISABLED; set {}=true to enforce signed attestation verification",
                TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV
            );
        }
        if !admission_policy_enforced() {
            eprintln!(
                "warning: admission policy is DISABLED; set {}=true to enforce DKG admission controls",
                TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV
            );
        }
        if !signing_policy_firewall_enforced() {
            eprintln!(
                "warning: signing policy firewall is DISABLED; set {}=true to enforce transaction policy controls",
                TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV
            );
        }
    });
}

pub(crate) fn load_admission_policy_config() -> Result<Option<AdmissionPolicyConfig>, EngineError> {
    if !admission_policy_enforced() {
        return Ok(None);
    }

    let min_participants =
        parse_usize_from_env_with_default(TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS_ENV, 2)?;
    let min_threshold =
        parse_u64_from_env_with_default(TBTC_SIGNER_ADMISSION_MIN_THRESHOLD_ENV, 2)?
            .try_into()
            .map_err(|_| {
                EngineError::Internal(format!(
                    "env [{}] exceeds u16 bounds",
                    TBTC_SIGNER_ADMISSION_MIN_THRESHOLD_ENV
                ))
            })?;
    let required_identifiers =
        parse_identifier_set_from_env(TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS_ENV)?
            .unwrap_or_default();
    let allowlist_identifiers =
        parse_identifier_set_from_env(TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV)?;

    Ok(Some(AdmissionPolicyConfig {
        min_participants,
        min_threshold,
        required_identifiers,
        allowlist_identifiers,
    }))
}

pub(crate) fn sanitize_policy_log_field(value: &str) -> String {
    value
        .chars()
        .map(|character| {
            if character.is_ascii_alphanumeric() || matches!(character, '-' | '_' | '.' | ':' | '/')
            {
                character
            } else {
                '_'
            }
        })
        .collect()
}

pub(crate) fn log_policy_decision(
    stage: &str,
    session_id: &str,
    decision: &str,
    reason_code: &str,
) {
    let stage = sanitize_policy_log_field(stage);
    let session_id = sanitize_policy_log_field(session_id);
    let decision = sanitize_policy_log_field(decision);
    let reason_code = sanitize_policy_log_field(reason_code);

    eprintln!(
        "policy_decision stage={} session_id={} decision={} reason_code={}",
        stage, session_id, decision, reason_code
    );
}

pub(crate) fn reject_admission_policy(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<(), EngineError> {
    let detail = detail.into();
    record_hardening_telemetry(|telemetry| {
        telemetry.run_dkg_admission_reject_total =
            telemetry.run_dkg_admission_reject_total.saturating_add(1);
    });
    log_policy_decision("admission_policy", session_id, "reject", reason_code);
    Err(EngineError::AdmissionPolicyRejected {
        session_id: session_id.to_string(),
        reason_code: reason_code.to_string(),
        detail,
    })
}

/// Admission checks over the raw participant primitives, used by the
/// distributed-DKG persist path (which derives the participant identifiers from
/// the group public key package).
pub(crate) fn enforce_admission_policy_for(
    session_id: &str,
    participant_count: usize,
    participant_identifiers: &HashSet<u16>,
    threshold: u16,
) -> Result<(), EngineError> {
    let policy = match load_admission_policy_config() {
        Ok(Some(policy)) => policy,
        Ok(None) => return Ok(()),
        Err(error) => {
            return reject_admission_policy(
                session_id,
                "invalid_policy_configuration",
                error.to_string(),
            )
        }
    };

    if participant_count < policy.min_participants {
        return reject_admission_policy(
            session_id,
            "participant_count_below_policy_minimum",
            format!(
                "participant count [{}] below policy minimum [{}]",
                participant_count, policy.min_participants
            ),
        );
    }

    if threshold < policy.min_threshold {
        return reject_admission_policy(
            session_id,
            "threshold_below_policy_minimum",
            format!(
                "threshold [{}] below policy minimum [{}]",
                threshold, policy.min_threshold
            ),
        );
    }

    if let Some(required_identifier) = policy
        .required_identifiers
        .iter()
        .find(|identifier| !participant_identifiers.contains(identifier))
    {
        return reject_admission_policy(
            session_id,
            "required_identifier_missing",
            format!(
                "required identifier [{}] missing from request",
                required_identifier
            ),
        );
    }

    if let Some(allowlist_identifiers) = policy.allowlist_identifiers.as_ref() {
        if let Some(unknown_identifier) = participant_identifiers
            .iter()
            .find(|identifier| !allowlist_identifiers.contains(identifier))
        {
            return reject_admission_policy(
                session_id,
                "participant_identifier_not_allowlisted",
                format!(
                    "participant identifier [{}] not present in configured allowlist",
                    unknown_identifier
                ),
            );
        }
    }

    log_policy_decision("admission_policy", session_id, "allow", "ok");
    Ok(())
}

pub(crate) fn load_signing_policy_firewall_config(
) -> Result<Option<SigningPolicyFirewallConfig>, EngineError> {
    if !signing_policy_firewall_enforced() {
        return Ok(None);
    }

    // Resolve to conservative built-in defaults when explicit policy env is not
    // set, so an enforced firewall (always on in production) does not require
    // every operator to ship full policy config to boot. The script-class
    // allowlist fails closed on non-standard forms; the numeric caps default
    // permissive and are operator-tunable.
    let allowed_script_classes = parse_script_class_set_with_default(
        TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV,
        DEFAULT_ALLOWED_SCRIPT_CLASSES,
    )?;
    let max_output_count = parse_usize_from_env_with_default(
        TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV,
        DEFAULT_MAX_OUTPUT_COUNT,
    )?;
    let max_output_value_sats = parse_u64_from_env_with_default(
        TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV,
        BITCOIN_MAX_MONEY_SATS,
    )?;
    let max_total_output_value_sats = parse_u64_from_env_with_default(
        TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV,
        BITCOIN_MAX_MONEY_SATS,
    )?;
    let allowed_utc_start_hour =
        parse_u8_from_env_optional(TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV)?;
    let allowed_utc_end_hour =
        parse_u8_from_env_optional(TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV)?;
    let rate_limit_per_minute =
        parse_u64_from_env_with_default(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV, 60)?;

    if rate_limit_per_minute == 0 {
        return Err(EngineError::Internal(format!(
            "env [{}] must be positive",
            TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV
        )));
    }

    if allowed_utc_start_hour.is_some() != allowed_utc_end_hour.is_some() {
        return Err(EngineError::Internal(format!(
            "env [{}] and [{}] must be configured together",
            TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV,
            TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV
        )));
    }

    if let (Some(start_hour), Some(end_hour)) = (allowed_utc_start_hour, allowed_utc_end_hour) {
        if start_hour >= 24 || end_hour >= 24 {
            return Err(EngineError::Internal(format!(
                "env [{}] and [{}] must be hours in the range 0..=23",
                TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV,
                TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV
            )));
        }
        // Reject a zero-width window. utc_hour_in_window treats start == end as
        // "always in window", so silently accepting equal bounds would disable
        // the time-of-day control entirely (fail-open) rather than restricting
        // it. An operator who wants no time restriction must leave both unset.
        if start_hour == end_hour {
            return Err(EngineError::Internal(format!(
                "env [{}] and [{}] must differ; an equal start and end hour does not define a restricted window",
                TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV,
                TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV
            )));
        }
    }

    Ok(Some(SigningPolicyFirewallConfig {
        allowed_script_classes,
        max_output_count,
        max_output_value_sats,
        max_total_output_value_sats,
        allowed_utc_start_hour,
        allowed_utc_end_hour,
        rate_limit_per_minute,
    }))
}

pub(crate) fn heartbeat_rate_limit_per_minute() -> Result<u64, EngineError> {
    let rate_limit_per_minute = parse_u64_from_env_with_default(
        TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV,
        TBTC_SIGNER_DEFAULT_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE,
    )?;
    if rate_limit_per_minute == 0 {
        return Err(EngineError::Internal(format!(
            "env [{}] must be positive",
            TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV
        )));
    }

    Ok(rate_limit_per_minute)
}

pub(crate) fn auto_quarantine_enabled() -> bool {
    signer_env_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV)
        .map(|raw_value| truthy_env_flag(&raw_value))
        .unwrap_or(false)
}

pub(crate) fn load_auto_quarantine_config() -> Result<Option<AutoQuarantineConfig>, EngineError> {
    if !auto_quarantine_enabled() {
        return Ok(None);
    }

    let fault_threshold = parse_u64_from_env_with_default(
        TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV,
        TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_FAULT_THRESHOLD,
    )?;
    let dao_allowlist_identifiers =
        parse_identifier_set_from_env(TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV)?
            .unwrap_or_default();

    if fault_threshold == 0 {
        return Err(EngineError::Internal(format!(
            "env [{}] must be positive",
            TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV
        )));
    }

    Ok(Some(AutoQuarantineConfig {
        fault_threshold,
        dao_allowlist_identifiers,
    }))
}

pub(crate) fn reject_quarantine_policy(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<(), EngineError> {
    let detail = detail.into();
    log_policy_decision("auto_quarantine", session_id, "reject", reason_code);
    Err(EngineError::QuarantinePolicyRejected {
        session_id: session_id.to_string(),
        reason_code: reason_code.to_string(),
        detail,
    })
}

pub(crate) fn reject_lifecycle_policy<T>(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<T, EngineError> {
    let detail = detail.into();
    log_policy_decision("lifecycle_policy", session_id, "reject", reason_code);
    Err(EngineError::LifecyclePolicyRejected {
        session_id: session_id.to_string(),
        reason_code: reason_code.to_string(),
        detail,
    })
}

fn reject_signing_policy_with_metric(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
    heartbeat: bool,
) -> Result<(), EngineError> {
    let detail = detail.into();
    record_hardening_telemetry(|telemetry| {
        if heartbeat {
            telemetry.heartbeat_signing_policy_reject_total = telemetry
                .heartbeat_signing_policy_reject_total
                .saturating_add(1);
        } else {
            telemetry.build_taproot_tx_policy_reject_total = telemetry
                .build_taproot_tx_policy_reject_total
                .saturating_add(1);
        }
    });
    log_policy_decision("signing_policy_firewall", session_id, "reject", reason_code);
    Err(EngineError::SigningPolicyRejected {
        session_id: session_id.to_string(),
        reason_code: reason_code.to_string(),
        detail,
    })
}

pub(crate) fn reject_signing_policy(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<(), EngineError> {
    reject_signing_policy_with_metric(session_id, reason_code, detail, false)
}

fn reject_heartbeat_signing_policy(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<(), EngineError> {
    reject_signing_policy_with_metric(session_id, reason_code, detail, true)
}

pub(crate) fn current_utc_hour() -> u8 {
    ((now_unix() / 3600) % 24) as u8
}

pub(crate) fn utc_hour_in_window(hour: u8, start_hour: u8, end_hour: u8) -> bool {
    if start_hour == end_hour {
        return true;
    }
    if start_hour < end_hour {
        return hour >= start_hour && hour < end_hour;
    }

    hour >= start_hour || hour < end_hour
}

fn consume_policy_rate_limit_token(
    limiter: &mut PolicyRateLimiterState,
    rate_limit_per_minute: u64,
) -> bool {
    let now = now_unix();
    let max_tokens =
        (rate_limit_per_minute as u128).saturating_mul(BUILD_TX_RATE_LIMIT_TOKEN_SCALE);
    if limiter.last_refill_unix == 0 {
        limiter.last_refill_unix = now;
        limiter.token_microunits = max_tokens;
        limiter.configured_rate_limit_per_minute = rate_limit_per_minute;
    }

    if limiter.configured_rate_limit_per_minute != rate_limit_per_minute {
        limiter.configured_rate_limit_per_minute = rate_limit_per_minute;
        limiter.token_microunits = limiter.token_microunits.min(max_tokens);
    }

    let elapsed_seconds = now.saturating_sub(limiter.last_refill_unix);
    if elapsed_seconds > 0 {
        let refill_microunits = (elapsed_seconds as u128)
            .saturating_mul(rate_limit_per_minute as u128)
            .saturating_mul(BUILD_TX_RATE_LIMIT_TOKEN_SCALE)
            / BUILD_TX_RATE_LIMIT_SECONDS_PER_MINUTE;
        limiter.token_microunits = limiter
            .token_microunits
            .saturating_add(refill_microunits)
            .min(max_tokens);
        limiter.last_refill_unix = now;
    }

    if limiter.token_microunits < BUILD_TX_RATE_LIMIT_TOKEN_SCALE {
        return false;
    }

    limiter.token_microunits = limiter
        .token_microunits
        .saturating_sub(BUILD_TX_RATE_LIMIT_TOKEN_SCALE);
    true
}

pub(crate) fn enforce_build_tx_rate_limit(
    session_id: &str,
    rate_limit_per_minute: u64,
) -> Result<(), EngineError> {
    let mut limiter = build_tx_rate_limiter_state()
        .lock()
        .map_err(|_| EngineError::Internal("build tx rate limiter mutex poisoned".to_string()))?;

    if !consume_policy_rate_limit_token(&mut limiter, rate_limit_per_minute) {
        return reject_signing_policy(
            session_id,
            "rate_limit_per_minute_exceeded",
            format!("rate limit [{}] per minute exceeded", rate_limit_per_minute),
        );
    }

    Ok(())
}

pub(crate) fn enforce_heartbeat_rate_limit(
    session_id: &str,
    limiter: &mut PolicyRateLimiterState,
) -> Result<(), EngineError> {
    let rate_limit_per_minute = match heartbeat_rate_limit_per_minute() {
        Ok(rate_limit_per_minute) => rate_limit_per_minute,
        Err(error) => {
            return reject_heartbeat_signing_policy(
                session_id,
                "invalid_policy_configuration",
                error.to_string(),
            )
        }
    };

    if !consume_policy_rate_limit_token(limiter, rate_limit_per_minute) {
        return reject_heartbeat_signing_policy(
            session_id,
            "heartbeat_rate_limit_per_minute_exceeded",
            format!(
                "heartbeat rate limit [{}] per minute exceeded",
                rate_limit_per_minute
            ),
        );
    }

    Ok(())
}

pub(crate) fn classify_script_pubkey(script_pubkey: &ScriptBuf) -> &'static str {
    if script_pubkey.is_p2tr() {
        "p2tr"
    } else if script_pubkey.is_p2wpkh() {
        "p2wpkh"
    } else if script_pubkey.is_p2wsh() {
        "p2wsh"
    } else if script_pubkey.is_p2pkh() {
        "p2pkh"
    } else if script_pubkey.is_p2sh() {
        "p2sh"
    } else {
        "other"
    }
}

pub(crate) fn enforce_signing_policy_firewall_inner(
    session_id: &str,
    outputs: &[TxOut],
    total_output_value_sats: u64,
    charge_rate_limit: bool,
) -> Result<(), EngineError> {
    let policy = match load_signing_policy_firewall_config() {
        Ok(Some(policy)) => policy,
        Ok(None) => return Ok(()),
        Err(error) => {
            return reject_signing_policy(
                session_id,
                "invalid_policy_configuration",
                error.to_string(),
            )
        }
    };

    if outputs.len() > policy.max_output_count {
        return reject_signing_policy(
            session_id,
            "output_count_exceeds_policy_limit",
            format!(
                "output count [{}] exceeds policy max [{}]",
                outputs.len(),
                policy.max_output_count
            ),
        );
    }

    if total_output_value_sats > policy.max_total_output_value_sats {
        return reject_signing_policy(
            session_id,
            "total_output_value_exceeds_policy_limit",
            format!(
                "total output value [{}] exceeds policy max [{}]",
                total_output_value_sats, policy.max_total_output_value_sats
            ),
        );
    }

    for output in outputs {
        let output_value_sats = output.value.to_sat();
        if output_value_sats > policy.max_output_value_sats {
            return reject_signing_policy(
                session_id,
                "single_output_value_exceeds_policy_limit",
                format!(
                    "output value [{}] exceeds policy max [{}]",
                    output_value_sats, policy.max_output_value_sats
                ),
            );
        }

        let script_class = classify_script_pubkey(&output.script_pubkey).to_string();
        if !policy.allowed_script_classes.contains(&script_class) {
            return reject_signing_policy(
                session_id,
                "script_class_not_allowlisted",
                format!(
                    "script class [{}] not in allowlist {:?}",
                    script_class, policy.allowed_script_classes
                ),
            );
        }
    }

    if let (Some(start_hour), Some(end_hour)) =
        (policy.allowed_utc_start_hour, policy.allowed_utc_end_hour)
    {
        let current_hour = current_utc_hour();
        if !utc_hour_in_window(current_hour, start_hour, end_hour) {
            return reject_signing_policy(
                session_id,
                "request_outside_allowed_utc_window",
                format!(
                    "current UTC hour [{}] not in window [{}..{})",
                    current_hour, start_hour, end_hour
                ),
            );
        }
    }

    if charge_rate_limit {
        enforce_build_tx_rate_limit(session_id, policy.rate_limit_per_minute)?;
    }
    log_policy_decision("signing_policy_firewall", session_id, "allow", "ok");
    Ok(())
}

pub(crate) fn enforce_signing_policy_firewall(
    session_id: &str,
    outputs: &[TxOut],
    total_output_value_sats: u64,
) -> Result<(), EngineError> {
    enforce_signing_policy_firewall_inner(session_id, outputs, total_output_value_sats, true)
}

pub(crate) fn recheck_signing_policy_firewall_without_rate_limit(
    session_id: &str,
    outputs: &[TxOut],
    total_output_value_sats: u64,
) -> Result<(), EngineError> {
    enforce_signing_policy_firewall_inner(session_id, outputs, total_output_value_sats, false)
}

pub(crate) fn policy_bound_signing_messages_hex(
    tx: &Transaction,
    prevouts: &[TxOut],
) -> Result<Vec<String>, EngineError> {
    if prevouts.len() != tx.input.len() {
        return Err(EngineError::Internal(format!(
            "BIP-341 prevout count [{}] does not match transaction input count [{}]",
            prevouts.len(),
            tx.input.len()
        )));
    }

    let prevouts = Prevouts::All(prevouts);
    let mut sighash_cache = SighashCache::new(tx);
    (0..tx.input.len())
        .map(|input_index| {
            sighash_cache
                .taproot_key_spend_signature_hash(
                    input_index,
                    &prevouts,
                    TapSighashType::Default,
                )
                .map(|sighash| hex::encode(sighash.to_byte_array()))
                .map_err(|error| {
                    EngineError::Internal(format!(
                        "failed to derive BIP-341 key-spend sighash for input [{input_index}]: {error}"
                    ))
                })
        })
        .collect()
}

fn invalid_policy_checked_build_tx_artifact(
    session_id: &str,
    detail: impl Into<String>,
) -> EngineError {
    let detail = detail.into();
    record_hardening_telemetry(|telemetry| {
        telemetry.build_taproot_tx_policy_reject_total = telemetry
            .build_taproot_tx_policy_reject_total
            .saturating_add(1);
    });
    log_policy_decision(
        "signing_policy_firewall",
        session_id,
        "reject",
        "invalid_policy_checked_build_tx_artifact",
    );
    EngineError::SigningPolicyRejected {
        session_id: session_id.to_string(),
        reason_code: "invalid_policy_checked_build_tx_artifact".to_string(),
        detail,
    }
}

fn enforce_heartbeat_signing_intent(
    session_id: &str,
    signing_message_hex: &str,
    taproot_merkle_root: Option<&[u8; 32]>,
    heartbeat_message_hex: &str,
) -> Result<(), EngineError> {
    if taproot_merkle_root.is_some() {
        return reject_heartbeat_signing_policy(
            session_id,
            "invalid_heartbeat_signing_intent",
            "heartbeat signing intent must not carry a Taproot merkle root",
        );
    }

    let heartbeat_message = match hex::decode(heartbeat_message_hex) {
        Ok(message) => message,
        Err(_) => {
            return reject_heartbeat_signing_policy(
                session_id,
                "invalid_heartbeat_signing_intent",
                "heartbeat signing intent message_hex must be valid hex",
            )
        }
    };
    if heartbeat_message.len() != 16 {
        return reject_heartbeat_signing_policy(
            session_id,
            "invalid_heartbeat_signing_intent",
            format!(
                "heartbeat signing intent must decode to exactly 16 bytes, got [{}]",
                heartbeat_message.len()
            ),
        );
    }
    if heartbeat_message[..8] != [0xff; 8] {
        return reject_heartbeat_signing_policy(
            session_id,
            "invalid_heartbeat_signing_intent",
            "heartbeat signing intent must start with eight 0xff bytes",
        );
    }

    let signing_message = match hex::decode(signing_message_hex) {
        Ok(message) => message,
        Err(_) => {
            return reject_heartbeat_signing_policy(
                session_id,
                "invalid_heartbeat_signing_intent",
                "heartbeat signing message must be valid hex",
            )
        }
    };
    if signing_message.len() != 32 {
        return reject_heartbeat_signing_policy(
            session_id,
            "invalid_heartbeat_signing_intent",
            format!(
                "heartbeat signing message must be exactly 32 bytes, got [{}]",
                signing_message.len()
            ),
        );
    }

    // Match Bitcoin's Hash256 convention used by the Go host and the on-chain
    // heartbeat contract: SHA256(SHA256(raw 16-byte heartbeat message)). Rust
    // derives this independently from the narrow preimage instead of trusting a
    // caller-supplied digest allowlist.
    let first_digest = Sha256::digest(&heartbeat_message);
    let heartbeat_digest = Sha256::digest(first_digest);
    if signing_message.as_slice() != heartbeat_digest.as_slice() {
        return reject_heartbeat_signing_policy(
            session_id,
            "heartbeat_signing_message_mismatch",
            "signing message does not equal Hash256 of the authorized heartbeat message",
        );
    }

    Ok(())
}

pub(crate) fn enforce_signing_message_binding_to_policy_checked_build_tx(
    session_id: &str,
    signing_message_hex: &str,
    taproot_merkle_root: Option<&[u8; 32]>,
    tx_result: Option<&TransactionResult>,
    signing_intent: Option<&InteractiveSigningIntent>,
) -> Result<(), EngineError> {
    if let Some(signing_intent) = signing_intent {
        if tx_result.is_some() {
            return reject_heartbeat_signing_policy(
                session_id,
                "ambiguous_signing_policy_artifact",
                "a signing session cannot carry both a transaction artifact and a non-transaction signing intent",
            );
        }

        return match signing_intent {
            InteractiveSigningIntent::Heartbeat { message_hex } => {
                enforce_heartbeat_signing_intent(
                    session_id,
                    signing_message_hex,
                    taproot_merkle_root,
                    message_hex,
                )
            }
        };
    }

    if !signing_policy_firewall_enforced() {
        return Ok(());
    }

    let tx_result = match tx_result {
        Some(tx_result) => tx_result,
        None => {
            return reject_signing_policy(
                session_id,
                "missing_policy_checked_build_tx",
                "signing policy firewall requires build_taproot_tx to run before signing for this session",
            )
        }
    };

    if tx_result.session_id != session_id {
        return Err(invalid_policy_checked_build_tx_artifact(
            session_id,
            format!(
                "policy-checked build tx belongs to session [{}], not signing session [{}]",
                tx_result.session_id, session_id
            ),
        ));
    }

    let tx_bytes = hex::decode(&tx_result.tx_hex).map_err(|_| {
        invalid_policy_checked_build_tx_artifact(
            session_id,
            "policy-checked build tx hex is not valid hex",
        )
    })?;
    let tx: Transaction = deserialize(&tx_bytes).map_err(|error| {
        invalid_policy_checked_build_tx_artifact(
            session_id,
            format!("policy-checked build tx is not a valid transaction: {error}"),
        )
    })?;

    if tx_result.taproot_key_spend_sighashes_hex.len() != tx.input.len()
        || tx_result.taproot_key_spend_sighashes_hex.is_empty()
    {
        return Err(invalid_policy_checked_build_tx_artifact(
            session_id,
            format!(
                "policy-checked BIP-341 sighash count [{}] does not match transaction input count [{}]",
                tx_result.taproot_key_spend_sighashes_hex.len(),
                tx.input.len()
            ),
        ));
    }
    for sighash_hex in &tx_result.taproot_key_spend_sighashes_hex {
        let sighash_bytes = hex::decode(sighash_hex).map_err(|_| {
            invalid_policy_checked_build_tx_artifact(
                session_id,
                "policy-checked BIP-341 sighash is not valid hex",
            )
        })?;
        if sighash_bytes.len() != 32 {
            return Err(invalid_policy_checked_build_tx_artifact(
                session_id,
                format!(
                    "policy-checked BIP-341 sighash length [{}] is not 32 bytes",
                    sighash_bytes.len()
                ),
            ));
        }
    }

    // The ordered sighashes are trusted because the complete persisted state is
    // authenticated by the encrypted AEAD envelope. Prevouts are intentionally not
    // duplicated in SessionState, so Open/Round2 shape-check this sealed list rather
    // than re-deriving it. A forged plaintext artifact is rejected at state load.
    // Re-run every current non-rate policy gate at Open and again at Round2 so a
    // restart with stricter script/value/time policy cannot authorize stale state.
    let total_output_value_sats = tx.output.iter().try_fold(0u64, |total, output| {
        total.checked_add(output.value.to_sat()).ok_or_else(|| {
            invalid_policy_checked_build_tx_artifact(
                session_id,
                "policy-checked build tx output total overflowed u64 bounds",
            )
        })
    })?;
    if total_output_value_sats > BITCOIN_MAX_MONEY_SATS {
        return Err(invalid_policy_checked_build_tx_artifact(
            session_id,
            format!(
                "policy-checked build tx output total [{}] exceeds Bitcoin max money [{}]",
                total_output_value_sats, BITCOIN_MAX_MONEY_SATS
            ),
        ));
    }
    recheck_signing_policy_firewall_without_rate_limit(
        session_id,
        &tx.output,
        total_output_value_sats,
    )?;

    let signing_message_hex = signing_message_hex.trim().to_ascii_lowercase();
    if !tx_result
        .taproot_key_spend_sighashes_hex
        .iter()
        .any(|expected| expected.eq_ignore_ascii_case(&signing_message_hex))
    {
        return reject_signing_policy(
            session_id,
            "signing_message_not_bound_to_policy_checked_build_tx",
            format!(
                "signing message [{}] is not an authorized BIP-341 key-spend sighash for the policy-checked transaction",
                signing_message_hex
            ),
        );
    }

    Ok(())
}
