use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};
use std::env;
use std::fs;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

const MAX_RELAXED_VENDOR_SHARE_PERCENT: u64 = 60;
const RELAXATION_TTL_SECONDS: u64 = 21_600;
const MAX_GRACE_PERIOD_SECONDS: u64 = 3_600;
const MAX_ATTESTATION_MAX_AGE_SECONDS: u64 = 86_400;
const MAX_DENYLIST_STALENESS_SECONDS: u64 = 300;
const MIN_ATTESTED_SIGNERS_PER_COHORT_FLOOR: u64 = 2;

#[derive(Clone, Debug, Deserialize)]
struct TeeGovernanceRegistryV1 {
    profile_status: String,
    enforcement: TeeEnforcementParameters,
    operators: Vec<TeeOperatorAdmissionRecord>,
}

#[derive(Clone, Debug, Deserialize)]
struct TeeEnforcementParameters {
    attestation_max_age_seconds: u64,
    grace_period_seconds: u64,
    min_attested_signers_per_cohort: u64,
    max_single_vendor_share_percent: u64,
    denylist_max_staleness_seconds: u64,
}

#[derive(Clone, Debug, Deserialize)]
struct TeeOperatorAdmissionRecord {
    operator_id: String,
    signer_identifier: String,
    status: String,
    effective_from: u64,
    #[serde(default)]
    effective_until: Option<u64>,
}

#[derive(Clone, Debug, Deserialize)]
struct RuntimeSessionInputV1 {
    session_id: String,
    phase: String,
    threshold: u64,
    selected_signers: Vec<RuntimeSelectedSigner>,
    denylist: RuntimeDenylistSnapshot,
    #[serde(default)]
    vendor_outage: Option<VendorOutageRelaxation>,
}

#[derive(Clone, Debug, Deserialize)]
struct RuntimeSelectedSigner {
    operator_id: String,
    signer_identifier: String,
    vendor_id: String,
    token: RuntimeTokenSnapshot,
}

#[derive(Clone, Debug, Deserialize)]
struct RuntimeTokenSnapshot {
    token_id: String,
    issued_at_unix: u64,
    expires_at_unix: u64,
    token_revocation_epoch: u64,
}

#[derive(Clone, Debug, Deserialize)]
struct RuntimeDenylistSnapshot {
    refreshed_at_unix: u64,
    #[serde(default)]
    revoked_operator_ids: Vec<String>,
    #[serde(default)]
    revoked_signer_identifiers: Vec<String>,
    #[serde(default)]
    revoked_token_ids: Vec<String>,
    #[serde(default)]
    min_token_revocation_epoch: u64,
}

#[derive(Clone, Debug, Deserialize)]
struct VendorOutageRelaxation {
    declared: bool,
    declared_at_unix: u64,
    relaxed_max_single_vendor_share_percent: u64,
    expires_at_unix: u64,
}

#[derive(Clone, Debug, Serialize)]
struct ValidationReason {
    code: String,
    detail: String,
}

#[derive(Clone, Debug, Serialize)]
struct ValidationDecision {
    decision: String,
    reasons: Vec<ValidationReason>,
    validated_at_unix: u64,
}

#[derive(Debug)]
struct CliArgs {
    registry_path: PathBuf,
    session_path: PathBuf,
    now_unix_override: Option<u64>,
}

#[derive(Copy, Clone, Debug, PartialEq, Eq)]
enum OperatorStatus {
    Active,
    Suspended,
    Revoked,
}

#[derive(Copy, Clone, Debug, PartialEq, Eq)]
enum RuntimePhase {
    SessionStart,
    MidSession,
}

fn usage() -> String {
    "Usage: tee_runtime_checker --registry <path> --session <path> [--now-unix <seconds>]"
        .to_string()
}

fn parse_args(args: &[String]) -> Result<CliArgs, String> {
    let mut registry_path: Option<PathBuf> = None;
    let mut session_path: Option<PathBuf> = None;
    let mut now_unix_override: Option<u64> = None;

    let mut i = 0usize;
    while i < args.len() {
        match args[i].as_str() {
            "--registry" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --registry".to_string());
                }
                registry_path = Some(PathBuf::from(&args[i]));
            }
            "--session" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --session".to_string());
                }
                session_path = Some(PathBuf::from(&args[i]));
            }
            "--now-unix" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --now-unix".to_string());
                }
                now_unix_override = Some(
                    args[i]
                        .parse::<u64>()
                        .map_err(|_| "invalid value for --now-unix".to_string())?,
                );
            }
            "--help" | "-h" => {
                return Err(usage());
            }
            unknown => {
                return Err(format!("unknown argument [{unknown}]"));
            }
        }
        i += 1;
    }

    let registry_path = registry_path.ok_or_else(|| "missing required --registry".to_string())?;
    let session_path = session_path.ok_or_else(|| "missing required --session".to_string())?;

    Ok(CliArgs {
        registry_path,
        session_path,
        now_unix_override,
    })
}

fn now_unix() -> Result<u64, String> {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .map_err(|error| format!("system clock must be after UNIX epoch: {error}"))
}

fn load_json_file<T: for<'de> Deserialize<'de>>(path: &PathBuf) -> Result<T, String> {
    let bytes = fs::read(path)
        .map_err(|error| format!("failed to read file [{}]: {error}", path.display()))?;
    serde_json::from_slice(&bytes)
        .map_err(|error| format!("failed to parse JSON file [{}]: {error}", path.display()))
}

fn trimmed_lowercase(value: &str) -> String {
    value.trim().to_ascii_lowercase()
}

fn parse_operator_status(status: &str) -> Option<OperatorStatus> {
    match trimmed_lowercase(status).as_str() {
        "active" => Some(OperatorStatus::Active),
        "suspended" => Some(OperatorStatus::Suspended),
        "revoked" => Some(OperatorStatus::Revoked),
        _ => None,
    }
}

fn parse_runtime_phase(phase: &str) -> Option<RuntimePhase> {
    match trimmed_lowercase(phase).as_str() {
        "session_start" => Some(RuntimePhase::SessionStart),
        "mid_session" => Some(RuntimePhase::MidSession),
        _ => None,
    }
}

fn push_rejection_reason(reasons: &mut Vec<ValidationReason>, code: &str, detail: String) {
    reasons.push(ValidationReason {
        code: code.to_string(),
        detail,
    });
}

fn normalize_denylist(
    snapshot: &RuntimeDenylistSnapshot,
) -> (HashSet<String>, HashSet<String>, HashSet<String>) {
    let revoked_operator_ids = snapshot
        .revoked_operator_ids
        .iter()
        .map(|operator_id| trimmed_lowercase(operator_id))
        .collect::<HashSet<_>>();
    let revoked_signer_identifiers = snapshot
        .revoked_signer_identifiers
        .iter()
        .map(|signer| trimmed_lowercase(signer))
        .collect::<HashSet<_>>();
    let revoked_token_ids = snapshot
        .revoked_token_ids
        .iter()
        .map(|token_id| trimmed_lowercase(token_id))
        .collect::<HashSet<_>>();

    (
        revoked_operator_ids,
        revoked_signer_identifiers,
        revoked_token_ids,
    )
}

fn find_operator<'a>(
    registry: &'a TeeGovernanceRegistryV1,
    operator_id: &str,
) -> Option<&'a TeeOperatorAdmissionRecord> {
    let operator_id = trimmed_lowercase(operator_id);
    registry
        .operators
        .iter()
        .find(|operator| trimmed_lowercase(&operator.operator_id) == operator_id)
}

fn effective_vendor_cap_percent(
    policy_cap_percent: u64,
    vendor_outage: Option<&VendorOutageRelaxation>,
    now_unix_seconds: u64,
    reasons: &mut Vec<ValidationReason>,
) -> u64 {
    let Some(vendor_outage) = vendor_outage else {
        return policy_cap_percent;
    };

    if !vendor_outage.declared {
        push_rejection_reason(
            reasons,
            "vendor_outage_not_declared",
            "vendor_outage object provided but declared=false".to_string(),
        );
        return policy_cap_percent;
    }

    if vendor_outage.declared_at_unix == 0 {
        push_rejection_reason(
            reasons,
            "vendor_outage_declared_at_invalid",
            "vendor_outage.declared_at_unix must be > 0".to_string(),
        );
        return policy_cap_percent;
    }

    if vendor_outage.declared_at_unix > now_unix_seconds {
        push_rejection_reason(
            reasons,
            "vendor_outage_declared_in_future",
            format!(
                "vendor_outage.declared_at_unix [{}] is in the future relative to now [{}]",
                vendor_outage.declared_at_unix, now_unix_seconds
            ),
        );
        return policy_cap_percent;
    }

    if vendor_outage.relaxed_max_single_vendor_share_percent < policy_cap_percent {
        push_rejection_reason(
            reasons,
            "vendor_outage_relaxation_below_policy",
            format!(
                "vendor_outage relaxed cap [{}] cannot be below policy cap [{}]",
                vendor_outage.relaxed_max_single_vendor_share_percent, policy_cap_percent
            ),
        );
        return policy_cap_percent;
    }

    if vendor_outage.relaxed_max_single_vendor_share_percent > MAX_RELAXED_VENDOR_SHARE_PERCENT {
        push_rejection_reason(
            reasons,
            "vendor_outage_relaxation_exceeds_maximum",
            format!(
                "vendor_outage relaxed cap [{}] exceeds maximum [{}]",
                vendor_outage.relaxed_max_single_vendor_share_percent,
                MAX_RELAXED_VENDOR_SHARE_PERCENT
            ),
        );
        return policy_cap_percent;
    }

    if ![40u64, 50u64, 60u64].contains(&vendor_outage.relaxed_max_single_vendor_share_percent) {
        push_rejection_reason(
            reasons,
            "vendor_outage_relaxation_step_invalid",
            format!(
                "vendor_outage relaxed cap [{}] must be one of [40, 50, 60]",
                vendor_outage.relaxed_max_single_vendor_share_percent
            ),
        );
        return policy_cap_percent;
    }

    if vendor_outage.expires_at_unix <= vendor_outage.declared_at_unix {
        push_rejection_reason(
            reasons,
            "vendor_outage_expiry_window_invalid",
            format!(
                "vendor_outage.expires_at_unix [{}] must be greater than declared_at_unix [{}]",
                vendor_outage.expires_at_unix, vendor_outage.declared_at_unix
            ),
        );
        return policy_cap_percent;
    }

    if vendor_outage.expires_at_unix <= now_unix_seconds {
        push_rejection_reason(
            reasons,
            "vendor_outage_relaxation_expired",
            format!(
                "vendor_outage relaxation expired at [{}], now [{}]",
                vendor_outage.expires_at_unix, now_unix_seconds
            ),
        );
        return policy_cap_percent;
    }

    let ttl_seconds = vendor_outage
        .expires_at_unix
        .saturating_sub(vendor_outage.declared_at_unix);
    if ttl_seconds > RELAXATION_TTL_SECONDS {
        push_rejection_reason(
            reasons,
            "vendor_outage_relaxation_ttl_exceeds_maximum",
            format!(
                "vendor_outage relaxation ttl [{}] exceeds maximum [{}]",
                ttl_seconds, RELAXATION_TTL_SECONDS
            ),
        );
        return policy_cap_percent;
    }

    vendor_outage.relaxed_max_single_vendor_share_percent
}

fn validate_runtime(
    registry: &TeeGovernanceRegistryV1,
    session: &RuntimeSessionInputV1,
    now_unix_seconds: u64,
) -> ValidationDecision {
    let mut reasons = Vec::new();

    if trimmed_lowercase(&registry.profile_status) != "mandatory" {
        push_rejection_reason(
            &mut reasons,
            "governance_profile_not_mandatory",
            format!(
                "governance registry profile_status [{}] is not mandatory",
                registry.profile_status
            ),
        );
    }

    if registry.enforcement.grace_period_seconds > MAX_GRACE_PERIOD_SECONDS {
        push_rejection_reason(
            &mut reasons,
            "grace_period_exceeds_hard_ceiling",
            format!(
                "grace_period_seconds [{}] exceeds hard ceiling [{}]",
                registry.enforcement.grace_period_seconds, MAX_GRACE_PERIOD_SECONDS
            ),
        );
    }

    if registry.enforcement.attestation_max_age_seconds > MAX_ATTESTATION_MAX_AGE_SECONDS {
        push_rejection_reason(
            &mut reasons,
            "attestation_max_age_exceeds_hard_ceiling",
            format!(
                "attestation_max_age_seconds [{}] exceeds hard ceiling [{}]",
                registry.enforcement.attestation_max_age_seconds, MAX_ATTESTATION_MAX_AGE_SECONDS
            ),
        );
    }

    if registry.enforcement.denylist_max_staleness_seconds == 0 {
        push_rejection_reason(
            &mut reasons,
            "denylist_max_staleness_invalid_zero",
            "denylist_max_staleness_seconds must be > 0".to_string(),
        );
    } else if registry.enforcement.denylist_max_staleness_seconds > MAX_DENYLIST_STALENESS_SECONDS {
        push_rejection_reason(
            &mut reasons,
            "denylist_max_staleness_exceeds_hard_ceiling",
            format!(
                "denylist_max_staleness_seconds [{}] exceeds hard ceiling [{}]",
                registry.enforcement.denylist_max_staleness_seconds, MAX_DENYLIST_STALENESS_SECONDS
            ),
        );
    }

    if registry.enforcement.min_attested_signers_per_cohort < MIN_ATTESTED_SIGNERS_PER_COHORT_FLOOR
    {
        push_rejection_reason(
            &mut reasons,
            "min_attested_signers_below_absolute_floor",
            format!(
                "min_attested_signers_per_cohort [{}] below absolute floor [{}]",
                registry.enforcement.min_attested_signers_per_cohort,
                MIN_ATTESTED_SIGNERS_PER_COHORT_FLOOR
            ),
        );
    }

    if registry.enforcement.max_single_vendor_share_percent == 0
        || registry.enforcement.max_single_vendor_share_percent > MAX_RELAXED_VENDOR_SHARE_PERCENT
    {
        push_rejection_reason(
            &mut reasons,
            "max_single_vendor_share_percent_out_of_bounds",
            format!(
                "max_single_vendor_share_percent [{}] must be in range [1, {}]",
                registry.enforcement.max_single_vendor_share_percent,
                MAX_RELAXED_VENDOR_SHARE_PERCENT
            ),
        );
    }

    let runtime_phase = match parse_runtime_phase(&session.phase) {
        Some(runtime_phase) => runtime_phase,
        None => {
            push_rejection_reason(
                &mut reasons,
                "runtime_phase_invalid",
                format!(
                    "session phase [{}] must be one of [session_start, mid_session]",
                    session.phase
                ),
            );
            RuntimePhase::SessionStart
        }
    };

    if session.session_id.trim().is_empty() {
        push_rejection_reason(
            &mut reasons,
            "session_id_missing",
            "session_id must be non-empty".to_string(),
        );
    }

    if session.threshold == 0 {
        push_rejection_reason(
            &mut reasons,
            "threshold_invalid",
            "threshold must be > 0".to_string(),
        );
    }

    let required_min_attested = session.threshold.saturating_add(1);
    if registry.enforcement.min_attested_signers_per_cohort < required_min_attested {
        push_rejection_reason(
            &mut reasons,
            "min_attested_signers_below_threshold_plus_one",
            format!(
                "policy min_attested_signers_per_cohort [{}] below required threshold+1 [{}]",
                registry.enforcement.min_attested_signers_per_cohort, required_min_attested
            ),
        );
    }

    if session.selected_signers.len()
        < registry.enforcement.min_attested_signers_per_cohort as usize
    {
        push_rejection_reason(
            &mut reasons,
            "selected_signers_below_policy_minimum",
            format!(
                "selected_signers [{}] below min_attested_signers_per_cohort [{}]",
                session.selected_signers.len(),
                registry.enforcement.min_attested_signers_per_cohort
            ),
        );
    }

    if session.selected_signers.is_empty() {
        push_rejection_reason(
            &mut reasons,
            "selected_signers_empty",
            "selected_signers must contain at least one signer".to_string(),
        );
    }

    if session.denylist.refreshed_at_unix == 0 {
        push_rejection_reason(
            &mut reasons,
            "denylist_refreshed_at_invalid",
            "denylist.refreshed_at_unix must be > 0".to_string(),
        );
    } else if session.denylist.refreshed_at_unix > now_unix_seconds {
        push_rejection_reason(
            &mut reasons,
            "denylist_refreshed_at_in_future",
            format!(
                "denylist.refreshed_at_unix [{}] is in the future relative to now [{}]",
                session.denylist.refreshed_at_unix, now_unix_seconds
            ),
        );
    } else {
        let denylist_age_seconds =
            now_unix_seconds.saturating_sub(session.denylist.refreshed_at_unix);
        if denylist_age_seconds > registry.enforcement.denylist_max_staleness_seconds {
            push_rejection_reason(
                &mut reasons,
                "denylist_stale",
                format!(
                    "denylist age [{}] exceeds max staleness [{}]",
                    denylist_age_seconds, registry.enforcement.denylist_max_staleness_seconds
                ),
            );
        }
    }

    let (revoked_operator_ids, revoked_signer_identifiers, revoked_token_ids) =
        normalize_denylist(&session.denylist);

    let vendor_cap_percent = effective_vendor_cap_percent(
        registry.enforcement.max_single_vendor_share_percent,
        session.vendor_outage.as_ref(),
        now_unix_seconds,
        &mut reasons,
    );

    let mut vendor_counts: HashMap<String, usize> = HashMap::new();
    let selected_signer_count = session.selected_signers.len();

    let mut seen_operators = HashSet::new();
    let mut seen_signer_identifiers = HashSet::new();
    let mut seen_token_ids = HashSet::new();

    for selected_signer in &session.selected_signers {
        let operator_id = trimmed_lowercase(&selected_signer.operator_id);
        let signer_identifier = trimmed_lowercase(&selected_signer.signer_identifier);
        let vendor_id = trimmed_lowercase(&selected_signer.vendor_id);
        let token_id = trimmed_lowercase(&selected_signer.token.token_id);

        if operator_id.is_empty() {
            push_rejection_reason(
                &mut reasons,
                "selected_operator_id_missing",
                "selected signer operator_id must be non-empty".to_string(),
            );
            continue;
        }

        if signer_identifier.is_empty() {
            push_rejection_reason(
                &mut reasons,
                "selected_signer_identifier_missing",
                "selected signer signer_identifier must be non-empty".to_string(),
            );
            continue;
        }

        if vendor_id.is_empty() {
            push_rejection_reason(
                &mut reasons,
                "selected_vendor_id_missing",
                format!(
                    "selected signer [{}] vendor_id must be non-empty",
                    selected_signer.operator_id
                ),
            );
            continue;
        }

        if token_id.is_empty() {
            push_rejection_reason(
                &mut reasons,
                "selected_token_id_missing",
                format!(
                    "selected signer [{}] token.token_id must be non-empty",
                    selected_signer.operator_id
                ),
            );
            continue;
        }

        if !seen_operators.insert(operator_id.clone()) {
            push_rejection_reason(
                &mut reasons,
                "selected_operator_id_duplicate",
                format!(
                    "operator_id [{}] appears more than once in selected_signers",
                    selected_signer.operator_id
                ),
            );
        }

        if !seen_signer_identifiers.insert(signer_identifier.clone()) {
            push_rejection_reason(
                &mut reasons,
                "selected_signer_identifier_duplicate",
                format!(
                    "signer_identifier [{}] appears more than once in selected_signers",
                    selected_signer.signer_identifier
                ),
            );
        }

        if !seen_token_ids.insert(token_id.clone()) {
            push_rejection_reason(
                &mut reasons,
                "selected_token_id_duplicate",
                format!(
                    "token_id [{}] appears more than once in selected_signers",
                    selected_signer.token.token_id
                ),
            );
        }

        let Some(operator) = find_operator(registry, &operator_id) else {
            push_rejection_reason(
                &mut reasons,
                "selected_operator_not_in_registry",
                format!(
                    "selected operator_id [{}] not found in governance registry",
                    selected_signer.operator_id
                ),
            );
            continue;
        };

        if parse_operator_status(&operator.status) != Some(OperatorStatus::Active) {
            push_rejection_reason(
                &mut reasons,
                "selected_operator_not_active",
                format!(
                    "selected operator_id [{}] has non-active status [{}]",
                    operator.operator_id, operator.status
                ),
            );
        }

        if trimmed_lowercase(&operator.signer_identifier) != signer_identifier {
            push_rejection_reason(
                &mut reasons,
                "selected_signer_identifier_registry_mismatch",
                format!(
                    "selected signer_identifier [{}] does not match registry signer_identifier [{}]",
                    selected_signer.signer_identifier, operator.signer_identifier
                ),
            );
        }

        if selected_signer.token.issued_at_unix < operator.effective_from {
            push_rejection_reason(
                &mut reasons,
                "selected_token_before_operator_effective_from",
                format!(
                    "selected token for operator_id [{}] issued_at_unix [{}] before effective_from [{}]",
                    selected_signer.operator_id,
                    selected_signer.token.issued_at_unix,
                    operator.effective_from
                ),
            );
        }

        if let Some(effective_until) = operator.effective_until {
            if selected_signer.token.issued_at_unix > effective_until {
                push_rejection_reason(
                    &mut reasons,
                    "selected_token_after_operator_effective_until",
                    format!(
                        "selected token for operator_id [{}] issued_at_unix [{}] after effective_until [{}]",
                        selected_signer.operator_id,
                        selected_signer.token.issued_at_unix,
                        effective_until
                    ),
                );
            }
        }

        if selected_signer.token.issued_at_unix > now_unix_seconds {
            push_rejection_reason(
                &mut reasons,
                "selected_token_not_yet_valid",
                format!(
                    "selected token [{}] issued_at_unix [{}] is in the future relative to now [{}]",
                    selected_signer.token.token_id,
                    selected_signer.token.issued_at_unix,
                    now_unix_seconds
                ),
            );
        }

        if selected_signer.token.expires_at_unix <= selected_signer.token.issued_at_unix {
            push_rejection_reason(
                &mut reasons,
                "selected_token_expiry_invalid",
                format!(
                    "selected token [{}] expires_at_unix [{}] must be greater than issued_at_unix [{}]",
                    selected_signer.token.token_id,
                    selected_signer.token.expires_at_unix,
                    selected_signer.token.issued_at_unix
                ),
            );
        }

        let token_ttl_seconds = selected_signer
            .token
            .expires_at_unix
            .saturating_sub(selected_signer.token.issued_at_unix);
        if token_ttl_seconds > registry.enforcement.attestation_max_age_seconds {
            push_rejection_reason(
                &mut reasons,
                "selected_token_ttl_exceeds_attestation_max_age",
                format!(
                    "selected token [{}] ttl [{}] exceeds attestation_max_age_seconds [{}]",
                    selected_signer.token.token_id,
                    token_ttl_seconds,
                    registry.enforcement.attestation_max_age_seconds
                ),
            );
        }

        if selected_signer.token.token_revocation_epoch
            < session.denylist.min_token_revocation_epoch
        {
            push_rejection_reason(
                &mut reasons,
                "selected_token_revocation_epoch_below_minimum",
                format!(
                    "selected token [{}] token_revocation_epoch [{}] below minimum [{}]",
                    selected_signer.token.token_id,
                    selected_signer.token.token_revocation_epoch,
                    session.denylist.min_token_revocation_epoch
                ),
            );
        }

        if revoked_operator_ids.contains(&operator_id) {
            push_rejection_reason(
                &mut reasons,
                "selected_operator_revoked",
                format!(
                    "selected operator_id [{}] is present in denylist revoked_operator_ids",
                    selected_signer.operator_id
                ),
            );
        }

        if revoked_signer_identifiers.contains(&signer_identifier) {
            push_rejection_reason(
                &mut reasons,
                "selected_signer_revoked",
                format!(
                    "selected signer_identifier [{}] is present in denylist revoked_signer_identifiers",
                    selected_signer.signer_identifier
                ),
            );
        }

        if revoked_token_ids.contains(&token_id) {
            push_rejection_reason(
                &mut reasons,
                "selected_token_revoked",
                format!(
                    "selected token_id [{}] is present in denylist revoked_token_ids",
                    selected_signer.token.token_id
                ),
            );
        }

        if now_unix_seconds > selected_signer.token.expires_at_unix {
            let elapsed_since_expiry =
                now_unix_seconds.saturating_sub(selected_signer.token.expires_at_unix);
            match runtime_phase {
                RuntimePhase::SessionStart => {
                    push_rejection_reason(
                        &mut reasons,
                        "selected_token_expired_for_session_start",
                        format!(
                            "selected token [{}] expired at [{}] before session start now [{}]",
                            selected_signer.token.token_id,
                            selected_signer.token.expires_at_unix,
                            now_unix_seconds
                        ),
                    );
                }
                RuntimePhase::MidSession => {
                    if elapsed_since_expiry > registry.enforcement.grace_period_seconds {
                        push_rejection_reason(
                            &mut reasons,
                            "selected_token_expired_beyond_grace",
                            format!(
                                "selected token [{}] expired [{}] seconds ago, exceeding grace_period_seconds [{}]",
                                selected_signer.token.token_id,
                                elapsed_since_expiry,
                                registry.enforcement.grace_period_seconds
                            ),
                        );
                    }
                }
            }
        }

        *vendor_counts.entry(vendor_id).or_insert(0usize) += 1;
    }

    if selected_signer_count > 0 {
        for (vendor_id, vendor_count) in vendor_counts {
            let vendor_share_percent =
                (vendor_count as u64).saturating_mul(100) / (selected_signer_count as u64);
            if vendor_share_percent > vendor_cap_percent {
                push_rejection_reason(
                    &mut reasons,
                    "vendor_diversity_cap_exceeded",
                    format!(
                        "vendor [{}] share [{}%] exceeds cap [{}%]",
                        vendor_id, vendor_share_percent, vendor_cap_percent
                    ),
                );
            }
        }
    }

    ValidationDecision {
        decision: if reasons.is_empty() {
            "allow".to_string()
        } else {
            "reject".to_string()
        },
        reasons,
        validated_at_unix: now_unix_seconds,
    }
}

fn run() -> Result<ValidationDecision, String> {
    let args = env::args().skip(1).collect::<Vec<_>>();
    let cli = parse_args(&args)?;
    let registry: TeeGovernanceRegistryV1 = load_json_file(&cli.registry_path)?;
    let session: RuntimeSessionInputV1 = load_json_file(&cli.session_path)?;
    let now_unix_seconds = match cli.now_unix_override {
        Some(now_unix_override) => now_unix_override,
        None => now_unix()?,
    };

    Ok(validate_runtime(&registry, &session, now_unix_seconds))
}

fn main() {
    match run() {
        Ok(decision) => {
            let json = serde_json::to_string_pretty(&decision).unwrap_or_else(|_| {
                "{\"decision\":\"reject\",\"reasons\":[{\"code\":\"serialization_error\",\"detail\":\"failed to encode output\"}],\"validated_at_unix\":0}".to_string()
            });
            println!("{json}");
            if decision.decision == "allow" {
                std::process::exit(0);
            }
            std::process::exit(1);
        }
        Err(error) => {
            eprintln!("{error}");
            eprintln!("{}", usage());
            std::process::exit(2);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn baseline_registry() -> TeeGovernanceRegistryV1 {
        TeeGovernanceRegistryV1 {
            profile_status: "mandatory".to_string(),
            enforcement: TeeEnforcementParameters {
                attestation_max_age_seconds: 3_600,
                grace_period_seconds: 900,
                min_attested_signers_per_cohort: 4,
                max_single_vendor_share_percent: 40,
                denylist_max_staleness_seconds: 60,
            },
            operators: vec![
                TeeOperatorAdmissionRecord {
                    operator_id: "operator-1".to_string(),
                    signer_identifier: "signer-1".to_string(),
                    status: "active".to_string(),
                    effective_from: 1_700_000_000,
                    effective_until: None,
                },
                TeeOperatorAdmissionRecord {
                    operator_id: "operator-2".to_string(),
                    signer_identifier: "signer-2".to_string(),
                    status: "active".to_string(),
                    effective_from: 1_700_000_000,
                    effective_until: None,
                },
                TeeOperatorAdmissionRecord {
                    operator_id: "operator-3".to_string(),
                    signer_identifier: "signer-3".to_string(),
                    status: "active".to_string(),
                    effective_from: 1_700_000_000,
                    effective_until: None,
                },
                TeeOperatorAdmissionRecord {
                    operator_id: "operator-4".to_string(),
                    signer_identifier: "signer-4".to_string(),
                    status: "active".to_string(),
                    effective_from: 1_700_000_000,
                    effective_until: None,
                },
            ],
        }
    }

    fn signer(
        operator_id: &str,
        signer_identifier: &str,
        vendor_id: &str,
        token_id: &str,
    ) -> RuntimeSelectedSigner {
        RuntimeSelectedSigner {
            operator_id: operator_id.to_string(),
            signer_identifier: signer_identifier.to_string(),
            vendor_id: vendor_id.to_string(),
            token: RuntimeTokenSnapshot {
                token_id: token_id.to_string(),
                issued_at_unix: 1_700_099_900,
                expires_at_unix: 1_700_100_300,
                token_revocation_epoch: 5,
            },
        }
    }

    fn baseline_session() -> RuntimeSessionInputV1 {
        RuntimeSessionInputV1 {
            session_id: "session-1".to_string(),
            phase: "session_start".to_string(),
            threshold: 3,
            selected_signers: vec![
                signer("operator-1", "signer-1", "vendor-a", "token-1"),
                signer("operator-2", "signer-2", "vendor-b", "token-2"),
                signer("operator-3", "signer-3", "vendor-c", "token-3"),
                signer("operator-4", "signer-4", "vendor-d", "token-4"),
            ],
            denylist: RuntimeDenylistSnapshot {
                refreshed_at_unix: 1_700_100_000,
                revoked_operator_ids: vec![],
                revoked_signer_identifiers: vec![],
                revoked_token_ids: vec![],
                min_token_revocation_epoch: 5,
            },
            vendor_outage: None,
        }
    }

    #[test]
    fn validate_runtime_allows_valid_session_start() {
        let registry = baseline_registry();
        let session = baseline_session();

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "allow");
        assert!(decision.reasons.is_empty());
    }

    #[test]
    fn validate_runtime_rejects_stale_denylist() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.denylist.refreshed_at_unix = 1_700_099_900;

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "denylist_stale"));
    }

    #[test]
    fn validate_runtime_rejects_vendor_diversity_cap_violation() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        for selected_signer in &mut session.selected_signers {
            selected_signer.vendor_id = "vendor-a".to_string();
        }

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "vendor_diversity_cap_exceeded"));
    }

    #[test]
    fn validate_runtime_allows_relaxed_vendor_cap_during_declared_outage() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.selected_signers[2].vendor_id = "vendor-a".to_string();
        session.vendor_outage = Some(VendorOutageRelaxation {
            declared: true,
            declared_at_unix: 1_700_099_500,
            relaxed_max_single_vendor_share_percent: 60,
            expires_at_unix: 1_700_101_000,
        });

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "allow");
    }

    #[test]
    fn validate_runtime_rejects_relaxed_vendor_cap_without_declared_outage() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.selected_signers[2].vendor_id = "vendor-a".to_string();
        session.vendor_outage = Some(VendorOutageRelaxation {
            declared: false,
            declared_at_unix: 1_700_099_500,
            relaxed_max_single_vendor_share_percent: 60,
            expires_at_unix: 1_700_101_000,
        });

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "vendor_outage_not_declared"));
    }

    #[test]
    fn validate_runtime_rejects_relaxed_vendor_cap_with_invalid_step() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.selected_signers[2].vendor_id = "vendor-a".to_string();
        session.vendor_outage = Some(VendorOutageRelaxation {
            declared: true,
            declared_at_unix: 1_700_099_500,
            relaxed_max_single_vendor_share_percent: 55,
            expires_at_unix: 1_700_101_000,
        });

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "vendor_outage_relaxation_step_invalid"));
    }

    #[test]
    fn validate_runtime_rejects_relaxed_vendor_cap_with_ttl_above_maximum() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.selected_signers[2].vendor_id = "vendor-a".to_string();
        session.vendor_outage = Some(VendorOutageRelaxation {
            declared: true,
            declared_at_unix: 1_700_078_000,
            relaxed_max_single_vendor_share_percent: 60,
            expires_at_unix: 1_700_100_001,
        });

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "vendor_outage_relaxation_ttl_exceeds_maximum" }));
    }

    #[test]
    fn validate_runtime_rejects_expired_token_for_session_start() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.selected_signers[0].token.expires_at_unix = 1_700_099_999;

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "selected_token_expired_for_session_start" }));
    }

    #[test]
    fn validate_runtime_allows_mid_session_expiry_within_grace_window() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.phase = "mid_session".to_string();
        session.selected_signers[0].token.issued_at_unix = 1_700_099_300;
        session.selected_signers[0].token.expires_at_unix = 1_700_099_700;

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "allow");
    }

    #[test]
    fn validate_runtime_rejects_mid_session_expiry_beyond_grace_window() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.phase = "mid_session".to_string();
        session.selected_signers[0].token.expires_at_unix = 1_700_098_000;

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "selected_token_expired_beyond_grace"));
    }

    #[test]
    fn validate_runtime_rejects_revoked_token() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.denylist.revoked_token_ids = vec!["token-2".to_string()];

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "selected_token_revoked"));
    }

    #[test]
    fn validate_runtime_rejects_revoked_operator() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.denylist.revoked_operator_ids = vec!["operator-2".to_string()];

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "selected_operator_revoked"));
    }

    #[test]
    fn validate_runtime_rejects_revoked_signer_identifier() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.denylist.revoked_signer_identifiers = vec!["signer-3".to_string()];

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "selected_signer_revoked"));
    }

    #[test]
    fn validate_runtime_rejects_token_revocation_epoch_below_minimum() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.denylist.min_token_revocation_epoch = 7;

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "selected_token_revocation_epoch_below_minimum" }));
    }

    #[test]
    fn validate_runtime_rejects_non_active_operator() {
        let mut registry = baseline_registry();
        registry.operators[0].status = "revoked".to_string();
        let session = baseline_session();

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "selected_operator_not_active"));
    }

    #[test]
    fn validate_runtime_rejects_when_profile_not_mandatory() {
        let mut registry = baseline_registry();
        registry.profile_status = "draft".to_string();
        let session = baseline_session();

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "governance_profile_not_mandatory"));
    }

    #[test]
    fn validate_runtime_rejects_grace_period_above_hard_ceiling() {
        let mut registry = baseline_registry();
        registry.enforcement.grace_period_seconds = MAX_GRACE_PERIOD_SECONDS + 1;
        let session = baseline_session();

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "grace_period_exceeds_hard_ceiling"));
    }

    #[test]
    fn validate_runtime_rejects_attestation_max_age_above_hard_ceiling() {
        let mut registry = baseline_registry();
        registry.enforcement.attestation_max_age_seconds = MAX_ATTESTATION_MAX_AGE_SECONDS + 1;
        let session = baseline_session();

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "attestation_max_age_exceeds_hard_ceiling"));
    }

    #[test]
    fn validate_runtime_rejects_denylist_max_staleness_above_hard_ceiling() {
        let mut registry = baseline_registry();
        registry.enforcement.denylist_max_staleness_seconds = MAX_DENYLIST_STALENESS_SECONDS + 1;
        let session = baseline_session();

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "denylist_max_staleness_exceeds_hard_ceiling" }));
    }

    #[test]
    fn validate_runtime_rejects_vendor_outage_expiry_not_after_declared() {
        let registry = baseline_registry();
        let mut session = baseline_session();
        session.selected_signers[2].vendor_id = "vendor-a".to_string();
        session.vendor_outage = Some(VendorOutageRelaxation {
            declared: true,
            declared_at_unix: 1_700_099_500,
            relaxed_max_single_vendor_share_percent: 50,
            expires_at_unix: 1_700_099_500,
        });

        let decision = validate_runtime(&registry, &session, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "vendor_outage_expiry_window_invalid"));
    }

    #[test]
    fn parse_args_accepts_required_flags() {
        let args = vec![
            "--registry".to_string(),
            "registry.json".to_string(),
            "--session".to_string(),
            "session.json".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.registry_path, PathBuf::from("registry.json"));
        assert_eq!(parsed.session_path, PathBuf::from("session.json"));
        assert!(parsed.now_unix_override.is_none());
    }

    #[test]
    fn parse_args_accepts_now_unix() {
        let args = vec![
            "--registry".to_string(),
            "registry.json".to_string(),
            "--session".to_string(),
            "session.json".to_string(),
            "--now-unix".to_string(),
            "1700100000".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.now_unix_override, Some(1_700_100_000));
    }

    #[test]
    fn parse_args_rejects_missing_session_flag() {
        let args = vec!["--registry".to_string(), "registry.json".to_string()];

        let error = parse_args(&args).expect_err("expected parse failure");
        assert_eq!(error, "missing required --session");
    }
}
