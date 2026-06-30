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
/// redemptions/sweeps, and `enforce_signing_message_binding_to_policy_checked_build_tx`
/// remains the primary control that the signed digest matches a policy-checked tx.
pub(crate) const DEFAULT_ALLOWED_SCRIPT_CLASSES: &[&str] =
    &["p2pkh", "p2sh", "p2wpkh", "p2wsh", "p2tr"];
pub(crate) const DEFAULT_MAX_OUTPUT_COUNT: usize = 10_000;

pub(crate) static POLICY_GATE_WARNING_EMITTED: OnceLock<()> = OnceLock::new();

pub(crate) static BUILD_TX_RATE_LIMITER: OnceLock<Mutex<BuildTxRateLimiterState>> = OnceLock::new();

pub(crate) const BUILD_TX_RATE_LIMIT_TOKEN_SCALE: u128 = 1_000_000;

pub(crate) const BUILD_TX_RATE_LIMIT_SECONDS_PER_MINUTE: u128 = 60;

#[derive(Default)]
pub(crate) struct BuildTxRateLimiterState {
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
    pub(crate) timeout_penalty: u64,
    pub(crate) invalid_share_penalty: u64,
    pub(crate) dao_allowlist_identifiers: HashSet<u16>,
}

pub(crate) fn build_tx_rate_limiter_state() -> &'static Mutex<BuildTxRateLimiterState> {
    BUILD_TX_RATE_LIMITER.get_or_init(|| Mutex::new(BuildTxRateLimiterState::default()))
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

pub(crate) fn enforce_admission_policy(request: &RunDkgRequest) -> Result<(), EngineError> {
    let policy = match load_admission_policy_config() {
        Ok(Some(policy)) => policy,
        Ok(None) => return Ok(()),
        Err(error) => {
            return reject_admission_policy(
                &request.session_id,
                "invalid_policy_configuration",
                error.to_string(),
            )
        }
    };

    if request.participants.len() < policy.min_participants {
        return reject_admission_policy(
            &request.session_id,
            "participant_count_below_policy_minimum",
            format!(
                "participant count [{}] below policy minimum [{}]",
                request.participants.len(),
                policy.min_participants
            ),
        );
    }

    if request.threshold < policy.min_threshold {
        return reject_admission_policy(
            &request.session_id,
            "threshold_below_policy_minimum",
            format!(
                "threshold [{}] below policy minimum [{}]",
                request.threshold, policy.min_threshold
            ),
        );
    }

    let participant_identifiers: HashSet<u16> = request
        .participants
        .iter()
        .map(|participant| participant.identifier)
        .collect();
    if let Some(required_identifier) = policy
        .required_identifiers
        .iter()
        .find(|identifier| !participant_identifiers.contains(identifier))
    {
        return reject_admission_policy(
            &request.session_id,
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
                &request.session_id,
                "participant_identifier_not_allowlisted",
                format!(
                    "participant identifier [{}] not present in configured allowlist",
                    unknown_identifier
                ),
            );
        }
    }

    log_policy_decision("admission_policy", &request.session_id, "allow", "ok");
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
    let timeout_penalty = parse_u64_from_env_with_default(
        TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV,
        TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_TIMEOUT_PENALTY,
    )?;
    let invalid_share_penalty = parse_u64_from_env_with_default(
        TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV,
        TBTC_SIGNER_DEFAULT_AUTO_QUARANTINE_INVALID_SHARE_PENALTY,
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
    if timeout_penalty == 0 {
        return Err(EngineError::Internal(format!(
            "env [{}] must be positive",
            TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV
        )));
    }
    if invalid_share_penalty == 0 {
        return Err(EngineError::Internal(format!(
            "env [{}] must be positive",
            TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV
        )));
    }

    Ok(Some(AutoQuarantineConfig {
        fault_threshold,
        timeout_penalty,
        invalid_share_penalty,
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

pub(crate) fn reject_signing_policy(
    session_id: &str,
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<(), EngineError> {
    let detail = detail.into();
    record_hardening_telemetry(|telemetry| {
        telemetry.build_taproot_tx_policy_reject_total = telemetry
            .build_taproot_tx_policy_reject_total
            .saturating_add(1);
    });
    log_policy_decision("signing_policy_firewall", session_id, "reject", reason_code);
    Err(EngineError::SigningPolicyRejected {
        session_id: session_id.to_string(),
        reason_code: reason_code.to_string(),
        detail,
    })
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

pub(crate) fn enforce_build_tx_rate_limit(
    session_id: &str,
    rate_limit_per_minute: u64,
) -> Result<(), EngineError> {
    let mut limiter = build_tx_rate_limiter_state()
        .lock()
        .map_err(|_| EngineError::Internal("build tx rate limiter mutex poisoned".to_string()))?;

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
        return reject_signing_policy(
            session_id,
            "rate_limit_per_minute_exceeded",
            format!("rate limit [{}] per minute exceeded", rate_limit_per_minute),
        );
    }

    limiter.token_microunits = limiter
        .token_microunits
        .saturating_sub(BUILD_TX_RATE_LIMIT_TOKEN_SCALE);
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

pub(crate) fn policy_bound_signing_message_hex(tx_hex: &str) -> Result<String, EngineError> {
    let tx_bytes = hex::decode(tx_hex).map_err(|_| {
        EngineError::Internal("policy-checked build tx hex is not valid hex".to_string())
    })?;
    Ok(hash_hex(&tx_bytes))
}

pub(crate) fn enforce_signing_message_binding_to_policy_checked_build_tx(
    session_id: &str,
    signing_message_hex: &str,
    tx_result: Option<&TransactionResult>,
) -> Result<(), EngineError> {
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

    let expected_signing_message_hex = policy_bound_signing_message_hex(&tx_result.tx_hex)
        .map_err(|error| EngineError::SigningPolicyRejected {
            session_id: session_id.to_string(),
            reason_code: "invalid_policy_checked_build_tx_artifact".to_string(),
            detail: error.to_string(),
        })?;
    let signing_message_hex = signing_message_hex.trim().to_ascii_lowercase();
    if signing_message_hex != expected_signing_message_hex {
        return reject_signing_policy(
            session_id,
            "signing_message_not_bound_to_policy_checked_build_tx",
            format!(
                "signing message [{}] does not match policy-checked build tx digest [{}]",
                signing_message_hex, expected_signing_message_hex
            ),
        );
    }

    Ok(())
}
