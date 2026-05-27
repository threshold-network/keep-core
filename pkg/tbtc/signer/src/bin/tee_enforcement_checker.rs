use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::env;
use std::fs;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

const SECONDS_PER_7_DAYS: u64 = 7 * 24 * 60 * 60;
const MAX_BREAK_GLASS_TTL_SECONDS: u64 = SECONDS_PER_7_DAYS;

#[derive(Clone, Debug, Deserialize)]
struct TeeGovernanceRegistryV1 {
    profile_status: String,
    enforcement: TeeEnforcementParameters,
}

#[derive(Clone, Debug, Deserialize)]
struct TeeEnforcementParameters {
    break_glass_ttl_seconds: u64,
    break_glass_max_activations_per_7d: u64,
    break_glass_cooldown_seconds: u64,
    break_glass_scope: String,
    break_glass_quorum_bps: u64,
}

#[derive(Clone, Debug, Deserialize)]
struct PhaseDEnforcementContextV1 {
    session_id: String,
    canary_session: bool,
    selected_operator_ids: Vec<String>,
    runtime_decision: RuntimeDecisionSnapshot,
    enforcement_mode: String,
    #[serde(default)]
    break_glass: Option<BreakGlassActivation>,
    #[serde(default)]
    break_glass_history: Vec<BreakGlassActivationHistoryRecord>,
}

#[derive(Clone, Debug, Deserialize)]
struct RuntimeDecisionSnapshot {
    decision: String,
    #[serde(default)]
    reasons: Vec<ValidationReason>,
    validated_at_unix: u64,
}

#[derive(Clone, Debug, Deserialize)]
struct BreakGlassActivation {
    incident_ticket: String,
    declared_at_unix: u64,
    expires_at_unix: u64,
    approver_quorum_bps: u64,
    scope_operator_ids: Vec<String>,
    #[serde(default)]
    is_new_activation: bool,
}

#[derive(Clone, Debug, Deserialize)]
struct BreakGlassActivationHistoryRecord {
    incident_ticket: String,
    activated_at_unix: u64,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
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
    context_path: PathBuf,
    now_unix_override: Option<u64>,
}

#[derive(Copy, Clone, Debug, PartialEq, Eq)]
enum EnforcementMode {
    MonitorOnly,
    SoftEnforcement,
    HardEnforcementCanary,
    FullEnforcement,
}

#[derive(Copy, Clone, Debug, PartialEq, Eq)]
enum RuntimeDecisionState {
    Allow,
    AllowWithWarnings,
    Reject,
}

#[derive(Clone, Debug)]
struct BreakGlassEvaluation {
    provided: bool,
    valid: bool,
    reasons: Vec<ValidationReason>,
}

fn usage() -> String {
    "Usage: tee_enforcement_checker --registry <path> --context <path> [--now-unix <seconds>]"
        .to_string()
}

fn parse_args(args: &[String]) -> Result<CliArgs, String> {
    let mut registry_path: Option<PathBuf> = None;
    let mut context_path: Option<PathBuf> = None;
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
            "--context" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --context".to_string());
                }
                context_path = Some(PathBuf::from(&args[i]));
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
    let context_path = context_path.ok_or_else(|| "missing required --context".to_string())?;

    Ok(CliArgs {
        registry_path,
        context_path,
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

fn parse_enforcement_mode(mode: &str) -> Option<EnforcementMode> {
    match trimmed_lowercase(mode).as_str() {
        "monitor_only" | "monitor-only" => Some(EnforcementMode::MonitorOnly),
        "soft_enforcement" | "soft-enforcement" => Some(EnforcementMode::SoftEnforcement),
        "hard_enforcement_canary" | "hard-enforcement-canary" => {
            Some(EnforcementMode::HardEnforcementCanary)
        }
        "full_enforcement" | "full-enforcement" => Some(EnforcementMode::FullEnforcement),
        _ => None,
    }
}

fn parse_runtime_decision_state(decision: &str) -> Option<RuntimeDecisionState> {
    match trimmed_lowercase(decision).as_str() {
        "allow" => Some(RuntimeDecisionState::Allow),
        "allow_with_warnings" | "allow-with-warnings" => {
            Some(RuntimeDecisionState::AllowWithWarnings)
        }
        "reject" => Some(RuntimeDecisionState::Reject),
        _ => None,
    }
}

fn push_reason(reasons: &mut Vec<ValidationReason>, code: &str, detail: String) {
    reasons.push(ValidationReason {
        code: code.to_string(),
        detail,
    });
}

fn append_runtime_reasons(target: &mut Vec<ValidationReason>, runtime: &RuntimeDecisionSnapshot) {
    if runtime.reasons.is_empty() {
        push_reason(
            target,
            "runtime_decision_reject_without_reasons",
            "runtime_decision is reject but reasons are empty".to_string(),
        );
        return;
    }

    for reason in &runtime.reasons {
        target.push(ValidationReason {
            code: format!("runtime_{}", reason.code),
            detail: reason.detail.clone(),
        });
    }
}

fn validate_context_structure(
    context: &PhaseDEnforcementContextV1,
    blocking_reasons: &mut Vec<ValidationReason>,
) {
    if context.session_id.trim().is_empty() {
        push_reason(
            blocking_reasons,
            "session_id_missing",
            "session_id must be non-empty".to_string(),
        );
    }

    if context.selected_operator_ids.is_empty() {
        push_reason(
            blocking_reasons,
            "selected_operator_ids_empty",
            "selected_operator_ids must contain at least one operator_id".to_string(),
        );
    }

    let mut seen = HashSet::new();
    for operator_id in &context.selected_operator_ids {
        let normalized = trimmed_lowercase(operator_id);
        if normalized.is_empty() {
            push_reason(
                blocking_reasons,
                "selected_operator_id_missing",
                "selected_operator_ids contains an empty operator_id".to_string(),
            );
            continue;
        }

        if !seen.insert(normalized) {
            push_reason(
                blocking_reasons,
                "selected_operator_id_duplicate",
                format!(
                    "selected_operator_ids contains duplicate operator_id [{}]",
                    operator_id
                ),
            );
        }
    }

    if context.runtime_decision.validated_at_unix == 0 {
        push_reason(
            blocking_reasons,
            "runtime_decision_validated_at_invalid",
            "runtime_decision.validated_at_unix must be > 0".to_string(),
        );
    }
}

fn evaluate_break_glass(
    registry: &TeeGovernanceRegistryV1,
    context: &PhaseDEnforcementContextV1,
    now_unix_seconds: u64,
) -> BreakGlassEvaluation {
    let Some(break_glass) = context.break_glass.as_ref() else {
        return BreakGlassEvaluation {
            provided: false,
            valid: false,
            reasons: Vec::new(),
        };
    };

    let mut reasons = Vec::new();
    let mut valid = true;

    if trimmed_lowercase(&registry.enforcement.break_glass_scope) != "named_operator_ids_only" {
        push_reason(
            &mut reasons,
            "break_glass_scope_not_supported",
            format!(
                "registry enforcement break_glass_scope [{}] must be [named_operator_ids_only]",
                registry.enforcement.break_glass_scope
            ),
        );
        valid = false;
    }

    if break_glass.incident_ticket.trim().is_empty() {
        push_reason(
            &mut reasons,
            "break_glass_incident_ticket_missing",
            "break_glass.incident_ticket must be non-empty".to_string(),
        );
        valid = false;
    }
    let current_incident_ticket_normalized = trimmed_lowercase(&break_glass.incident_ticket);

    if break_glass.declared_at_unix == 0 {
        push_reason(
            &mut reasons,
            "break_glass_declared_at_invalid",
            "break_glass.declared_at_unix must be > 0".to_string(),
        );
        valid = false;
    }

    if break_glass.declared_at_unix > now_unix_seconds {
        push_reason(
            &mut reasons,
            "break_glass_declared_in_future",
            format!(
                "break_glass.declared_at_unix [{}] is in the future relative to now [{}]",
                break_glass.declared_at_unix, now_unix_seconds
            ),
        );
        valid = false;
    }

    if break_glass.expires_at_unix <= break_glass.declared_at_unix {
        push_reason(
            &mut reasons,
            "break_glass_expiry_window_invalid",
            format!(
                "break_glass.expires_at_unix [{}] must be greater than declared_at_unix [{}]",
                break_glass.expires_at_unix, break_glass.declared_at_unix
            ),
        );
        valid = false;
    }

    if break_glass.expires_at_unix <= now_unix_seconds {
        push_reason(
            &mut reasons,
            "break_glass_expired",
            format!(
                "break_glass expires_at_unix [{}] is not after now [{}]",
                break_glass.expires_at_unix, now_unix_seconds
            ),
        );
        valid = false;
    }

    let ttl_policy_valid = registry.enforcement.break_glass_ttl_seconds > 0
        && registry.enforcement.break_glass_ttl_seconds <= MAX_BREAK_GLASS_TTL_SECONDS;
    if !ttl_policy_valid {
        push_reason(
            &mut reasons,
            "break_glass_ttl_policy_invalid",
            format!(
                "registry break_glass_ttl_seconds [{}] must be within [1, {}]",
                registry.enforcement.break_glass_ttl_seconds, MAX_BREAK_GLASS_TTL_SECONDS
            ),
        );
        valid = false;
    }

    if break_glass.expires_at_unix > break_glass.declared_at_unix {
        let ttl_seconds = break_glass
            .expires_at_unix
            .saturating_sub(break_glass.declared_at_unix);
        if ttl_seconds > MAX_BREAK_GLASS_TTL_SECONDS {
            push_reason(
                &mut reasons,
                "break_glass_ttl_exceeds_hard_max",
                format!(
                    "break_glass ttl [{}] exceeds hard maximum [{}]",
                    ttl_seconds, MAX_BREAK_GLASS_TTL_SECONDS
                ),
            );
            valid = false;
        }

        if ttl_policy_valid && ttl_seconds > registry.enforcement.break_glass_ttl_seconds {
            push_reason(
                &mut reasons,
                "break_glass_ttl_exceeds_policy",
                format!(
                    "break_glass ttl [{}] exceeds policy break_glass_ttl_seconds [{}]",
                    ttl_seconds, registry.enforcement.break_glass_ttl_seconds
                ),
            );
            valid = false;
        }
    }

    if break_glass.approver_quorum_bps > 10_000 {
        push_reason(
            &mut reasons,
            "break_glass_quorum_out_of_range",
            format!(
                "break_glass.approver_quorum_bps [{}] must be <= 10000",
                break_glass.approver_quorum_bps
            ),
        );
        valid = false;
    }

    if break_glass.approver_quorum_bps < registry.enforcement.break_glass_quorum_bps {
        push_reason(
            &mut reasons,
            "break_glass_quorum_below_required",
            format!(
                "break_glass.approver_quorum_bps [{}] below required [{}]",
                break_glass.approver_quorum_bps, registry.enforcement.break_glass_quorum_bps
            ),
        );
        valid = false;
    }

    if break_glass.scope_operator_ids.is_empty() {
        push_reason(
            &mut reasons,
            "break_glass_scope_empty",
            "break_glass.scope_operator_ids must be non-empty".to_string(),
        );
        valid = false;
    }

    let mut normalized_scope = HashSet::new();
    for scoped_operator_id in &break_glass.scope_operator_ids {
        let normalized = trimmed_lowercase(scoped_operator_id);
        if normalized.is_empty() {
            push_reason(
                &mut reasons,
                "break_glass_scope_operator_missing",
                "break_glass.scope_operator_ids contains an empty operator_id".to_string(),
            );
            valid = false;
            continue;
        }

        let _ = normalized_scope.insert(normalized);
    }

    for selected_operator_id in &context.selected_operator_ids {
        let normalized = trimmed_lowercase(selected_operator_id);
        if normalized.is_empty() {
            continue;
        }

        if !normalized_scope.contains(&normalized) {
            push_reason(
                &mut reasons,
                "break_glass_scope_operator_not_covered",
                format!(
                    "selected operator_id [{}] is not covered by break_glass scope_operator_ids",
                    selected_operator_id
                ),
            );
            valid = false;
        }
    }

    let mut history_incident_tickets = HashSet::new();
    let mut recent_activation_incident_tickets = HashSet::new();
    let mut latest_activation_unix: Option<u64> = None;
    let mut reused_incident_activated_at_unix: Option<u64> = None;
    let recent_window_start = now_unix_seconds.saturating_sub(SECONDS_PER_7_DAYS);
    for history_record in &context.break_glass_history {
        let history_incident_ticket_normalized = trimmed_lowercase(&history_record.incident_ticket);
        if history_incident_ticket_normalized.is_empty() {
            push_reason(
                &mut reasons,
                "break_glass_history_incident_ticket_missing",
                "break_glass_history contains an empty incident_ticket".to_string(),
            );
            valid = false;
            continue;
        }

        if !history_incident_tickets.insert(history_incident_ticket_normalized.clone()) {
            push_reason(
                &mut reasons,
                "break_glass_history_duplicate_incident_ticket",
                format!(
                    "break_glass_history contains duplicate incident_ticket [{}]",
                    history_record.incident_ticket
                ),
            );
            valid = false;
        }

        if history_incident_ticket_normalized == current_incident_ticket_normalized
            && reused_incident_activated_at_unix.is_none()
        {
            reused_incident_activated_at_unix = Some(history_record.activated_at_unix);
        }

        if history_record.activated_at_unix > now_unix_seconds {
            push_reason(
                &mut reasons,
                "break_glass_history_activation_in_future",
                format!(
                    "break_glass_history activated_at_unix [{}] is in the future relative to now [{}]",
                    history_record.activated_at_unix, now_unix_seconds
                ),
            );
            valid = false;
            continue;
        }

        if history_record.activated_at_unix >= recent_window_start {
            let _ = recent_activation_incident_tickets.insert(history_incident_ticket_normalized);
        }

        latest_activation_unix = Some(
            latest_activation_unix
                .map(|current| current.max(history_record.activated_at_unix))
                .unwrap_or(history_record.activated_at_unix),
        );
    }

    let inferred_new_activation = !current_incident_ticket_normalized.is_empty()
        && !history_incident_tickets.contains(&current_incident_ticket_normalized);

    if break_glass.is_new_activation != inferred_new_activation {
        push_reason(
            &mut reasons,
            "break_glass_activation_hint_mismatch",
            format!(
                "break_glass.is_new_activation [{}] does not match inferred value [{}] from incident history",
                break_glass.is_new_activation, inferred_new_activation
            ),
        );
    }

    if let Some(activated_at_unix) =
        reused_incident_activated_at_unix.filter(|_| !inferred_new_activation)
    {
        if break_glass.declared_at_unix != activated_at_unix {
            push_reason(
                &mut reasons,
                "break_glass_reused_incident_declared_at_mismatch",
                format!(
                    "reused incident_ticket [{}] declared_at_unix [{}] must match history activated_at_unix [{}]",
                    break_glass.incident_ticket, break_glass.declared_at_unix, activated_at_unix
                ),
            );
            valid = false;
        }

        if ttl_policy_valid {
            let historical_expires_at_unix =
                activated_at_unix.saturating_add(registry.enforcement.break_glass_ttl_seconds);
            if historical_expires_at_unix <= now_unix_seconds {
                push_reason(
                    &mut reasons,
                    "break_glass_reused_incident_expired",
                    format!(
                        "reused incident_ticket [{}] expired at [{}] based on history activated_at_unix [{}] and policy ttl [{}]",
                        break_glass.incident_ticket,
                        historical_expires_at_unix,
                        activated_at_unix,
                        registry.enforcement.break_glass_ttl_seconds
                    ),
                );
                valid = false;
            }

            if break_glass.expires_at_unix > historical_expires_at_unix {
                push_reason(
                    &mut reasons,
                    "break_glass_reused_incident_extends_ttl",
                    format!(
                        "reused incident_ticket [{}] expires_at_unix [{}] exceeds history-derived expiry [{}]",
                        break_glass.incident_ticket,
                        break_glass.expires_at_unix,
                        historical_expires_at_unix
                    ),
                );
                valid = false;
            }
        }
    }

    if inferred_new_activation {
        let recent_activations = recent_activation_incident_tickets.len();
        let projected_activations = recent_activations.saturating_add(1);
        if projected_activations > registry.enforcement.break_glass_max_activations_per_7d as usize
        {
            push_reason(
                &mut reasons,
                "break_glass_activation_limit_exceeded",
                format!(
                    "projected break-glass activations in 7d [{}] exceed max [{}]",
                    projected_activations, registry.enforcement.break_glass_max_activations_per_7d
                ),
            );
            valid = false;
        }

        if let Some(latest_activation_unix) = latest_activation_unix {
            let elapsed = now_unix_seconds.saturating_sub(latest_activation_unix);
            if elapsed < registry.enforcement.break_glass_cooldown_seconds {
                push_reason(
                    &mut reasons,
                    "break_glass_cooldown_violation",
                    format!(
                        "break-glass activation cooldown violated: elapsed [{}] < required [{}]",
                        elapsed, registry.enforcement.break_glass_cooldown_seconds
                    ),
                );
                valid = false;
            }
        }
    }

    BreakGlassEvaluation {
        provided: true,
        valid,
        reasons,
    }
}

fn validate_enforcement(
    registry: &TeeGovernanceRegistryV1,
    context: &PhaseDEnforcementContextV1,
    now_unix_seconds: u64,
) -> ValidationDecision {
    let mut blocking_reasons = Vec::new();
    let mut warning_reasons = Vec::new();

    validate_context_structure(context, &mut blocking_reasons);

    let runtime_state = match parse_runtime_decision_state(&context.runtime_decision.decision) {
        Some(runtime_state) => runtime_state,
        None => {
            push_reason(
                &mut blocking_reasons,
                "runtime_decision_invalid",
                format!(
                    "runtime_decision.decision [{}] must be one of [allow, allow_with_warnings, allow-with-warnings, reject]",
                    context.runtime_decision.decision
                ),
            );
            RuntimeDecisionState::Reject
        }
    };

    let enforcement_mode = match parse_enforcement_mode(&context.enforcement_mode) {
        Some(enforcement_mode) => enforcement_mode,
        None => {
            push_reason(
                &mut blocking_reasons,
                "enforcement_mode_invalid",
                format!(
                    "enforcement_mode [{}] must be one of [monitor_only, soft_enforcement, hard_enforcement_canary, full_enforcement]",
                    context.enforcement_mode
                ),
            );
            EnforcementMode::FullEnforcement
        }
    };

    if trimmed_lowercase(&registry.profile_status) != "mandatory"
        && matches!(
            enforcement_mode,
            EnforcementMode::HardEnforcementCanary | EnforcementMode::FullEnforcement
        )
    {
        push_reason(
            &mut blocking_reasons,
            "governance_profile_not_mandatory_for_strict_mode",
            format!(
                "registry profile_status [{}] must be mandatory for strict enforcement modes",
                registry.profile_status
            ),
        );
    }

    let break_glass_evaluation = evaluate_break_glass(registry, context, now_unix_seconds);
    let runtime_violation = runtime_state != RuntimeDecisionState::Allow;

    if !blocking_reasons.is_empty() {
        return ValidationDecision {
            decision: "reject".to_string(),
            reasons: blocking_reasons,
            validated_at_unix: now_unix_seconds,
        };
    }

    match enforcement_mode {
        EnforcementMode::MonitorOnly => {
            if runtime_violation {
                push_reason(
                    &mut warning_reasons,
                    "monitor_only_runtime_violation_observed",
                    "runtime decision indicates policy violations; monitor_only mode does not block"
                        .to_string(),
                );
                append_runtime_reasons(&mut warning_reasons, &context.runtime_decision);
            }

            if break_glass_evaluation.provided && !break_glass_evaluation.valid {
                warning_reasons.extend(break_glass_evaluation.reasons);
            }
        }
        EnforcementMode::SoftEnforcement => {
            if runtime_violation {
                push_reason(
                    &mut warning_reasons,
                    "soft_enforcement_violation_exclusion_preferred",
                    "runtime decision indicates policy violations; soft_enforcement mode prefers exclusion but does not block"
                        .to_string(),
                );
                append_runtime_reasons(&mut warning_reasons, &context.runtime_decision);
            }

            if break_glass_evaluation.provided && !break_glass_evaluation.valid {
                warning_reasons.extend(break_glass_evaluation.reasons);
            }
        }
        EnforcementMode::HardEnforcementCanary => {
            if runtime_violation {
                if context.canary_session {
                    if break_glass_evaluation.provided && break_glass_evaluation.valid {
                        push_reason(
                            &mut warning_reasons,
                            "hard_enforcement_canary_break_glass_applied",
                            "canary runtime violation allowed due to valid break-glass activation"
                                .to_string(),
                        );
                        append_runtime_reasons(&mut warning_reasons, &context.runtime_decision);
                    } else {
                        push_reason(
                            &mut blocking_reasons,
                            "hard_enforcement_canary_violation_blocked",
                            "canary runtime violation blocked in hard_enforcement_canary mode"
                                .to_string(),
                        );
                        append_runtime_reasons(&mut blocking_reasons, &context.runtime_decision);
                        if break_glass_evaluation.provided {
                            blocking_reasons.extend(break_glass_evaluation.reasons);
                        }
                    }
                } else {
                    push_reason(
                        &mut warning_reasons,
                        "hard_enforcement_canary_non_canary_soft_fallback",
                        "non-canary runtime violation observed; hard_enforcement_canary mode does not block non-canary sessions"
                            .to_string(),
                    );
                    append_runtime_reasons(&mut warning_reasons, &context.runtime_decision);

                    if break_glass_evaluation.provided && !break_glass_evaluation.valid {
                        warning_reasons.extend(break_glass_evaluation.reasons);
                    }
                }
            } else if break_glass_evaluation.provided && !break_glass_evaluation.valid {
                warning_reasons.extend(break_glass_evaluation.reasons);
            }
        }
        EnforcementMode::FullEnforcement => {
            if runtime_violation {
                if break_glass_evaluation.provided && break_glass_evaluation.valid {
                    push_reason(
                        &mut warning_reasons,
                        "full_enforcement_break_glass_applied",
                        "runtime violation allowed due to valid break-glass activation".to_string(),
                    );
                    append_runtime_reasons(&mut warning_reasons, &context.runtime_decision);
                } else {
                    push_reason(
                        &mut blocking_reasons,
                        "full_enforcement_violation_blocked",
                        "runtime violation blocked in full_enforcement mode".to_string(),
                    );
                    append_runtime_reasons(&mut blocking_reasons, &context.runtime_decision);
                    if break_glass_evaluation.provided {
                        blocking_reasons.extend(break_glass_evaluation.reasons);
                    }
                }
            } else if break_glass_evaluation.provided && !break_glass_evaluation.valid {
                warning_reasons.extend(break_glass_evaluation.reasons);
            }
        }
    }

    if !blocking_reasons.is_empty() {
        return ValidationDecision {
            decision: "reject".to_string(),
            reasons: blocking_reasons,
            validated_at_unix: now_unix_seconds,
        };
    }

    if warning_reasons.is_empty() {
        ValidationDecision {
            decision: "allow".to_string(),
            reasons: warning_reasons,
            validated_at_unix: now_unix_seconds,
        }
    } else {
        ValidationDecision {
            decision: "allow_with_warnings".to_string(),
            reasons: warning_reasons,
            validated_at_unix: now_unix_seconds,
        }
    }
}

fn run() -> Result<ValidationDecision, String> {
    let args = env::args().skip(1).collect::<Vec<_>>();
    let cli = parse_args(&args)?;

    let registry: TeeGovernanceRegistryV1 = load_json_file(&cli.registry_path)?;
    let context: PhaseDEnforcementContextV1 = load_json_file(&cli.context_path)?;
    let now_unix_seconds = match cli.now_unix_override {
        Some(now_unix_override) => now_unix_override,
        None => now_unix()?,
    };

    Ok(validate_enforcement(&registry, &context, now_unix_seconds))
}

fn main() {
    match run() {
        Ok(decision) => {
            let json = serde_json::to_string_pretty(&decision).unwrap_or_else(|_| {
                "{\"decision\":\"reject\",\"reasons\":[{\"code\":\"serialization_error\",\"detail\":\"failed to encode output\"}],\"validated_at_unix\":0}".to_string()
            });
            println!("{json}");
            if decision.decision == "reject" {
                std::process::exit(1);
            }
            std::process::exit(0);
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
                break_glass_ttl_seconds: 21_600,
                break_glass_max_activations_per_7d: 2,
                break_glass_cooldown_seconds: 86_400,
                break_glass_scope: "named_operator_ids_only".to_string(),
                break_glass_quorum_bps: 6_700,
            },
        }
    }

    fn runtime_allow() -> RuntimeDecisionSnapshot {
        RuntimeDecisionSnapshot {
            decision: "allow".to_string(),
            reasons: vec![],
            validated_at_unix: 1_700_100_000,
        }
    }

    fn runtime_reject() -> RuntimeDecisionSnapshot {
        RuntimeDecisionSnapshot {
            decision: "reject".to_string(),
            reasons: vec![ValidationReason {
                code: "vendor_diversity_cap_exceeded".to_string(),
                detail: "vendor-a share exceeds cap".to_string(),
            }],
            validated_at_unix: 1_700_100_000,
        }
    }

    fn baseline_context() -> PhaseDEnforcementContextV1 {
        PhaseDEnforcementContextV1 {
            session_id: "session-1".to_string(),
            canary_session: true,
            selected_operator_ids: vec!["operator-1".to_string(), "operator-2".to_string()],
            runtime_decision: runtime_allow(),
            enforcement_mode: "full_enforcement".to_string(),
            break_glass: None,
            break_glass_history: vec![],
        }
    }

    fn valid_break_glass() -> BreakGlassActivation {
        BreakGlassActivation {
            incident_ticket: "INC-123".to_string(),
            declared_at_unix: 1_700_099_000,
            expires_at_unix: 1_700_103_600,
            approver_quorum_bps: 7_100,
            scope_operator_ids: vec!["operator-1".to_string(), "operator-2".to_string()],
            is_new_activation: true,
        }
    }

    #[test]
    fn validate_enforcement_allows_full_enforcement_when_runtime_allows() {
        let registry = baseline_registry();
        let context = baseline_context();

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "allow");
        assert!(decision.reasons.is_empty());
    }

    #[test]
    fn validate_enforcement_rejects_full_enforcement_runtime_violation_without_break_glass() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "full_enforcement_violation_blocked"));
    }

    #[test]
    fn validate_enforcement_allows_monitor_only_with_warning_on_runtime_violation() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.enforcement_mode = "monitor_only".to_string();
        context.runtime_decision = runtime_reject();

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "allow_with_warnings");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "monitor_only_runtime_violation_observed"));
    }

    #[test]
    fn validate_enforcement_allows_soft_enforcement_with_warning_on_runtime_violation() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.enforcement_mode = "soft_enforcement".to_string();
        context.runtime_decision = runtime_reject();

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "allow_with_warnings");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "soft_enforcement_violation_exclusion_preferred" }));
    }

    #[test]
    fn validate_enforcement_rejects_hard_canary_runtime_violation_for_canary_session() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.enforcement_mode = "hard_enforcement_canary".to_string();
        context.runtime_decision = runtime_reject();
        context.canary_session = true;

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "hard_enforcement_canary_violation_blocked" }));
    }

    #[test]
    fn validate_enforcement_allows_hard_canary_runtime_violation_for_non_canary_session() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.enforcement_mode = "hard_enforcement_canary".to_string();
        context.runtime_decision = runtime_reject();
        context.canary_session = false;

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "allow_with_warnings");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "hard_enforcement_canary_non_canary_soft_fallback" }));
    }

    #[test]
    fn validate_enforcement_allows_hard_canary_runtime_violation_with_valid_break_glass() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.enforcement_mode = "hard_enforcement_canary".to_string();
        context.runtime_decision = runtime_reject();
        context.canary_session = true;
        context.break_glass = Some(valid_break_glass());

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "allow_with_warnings");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "hard_enforcement_canary_break_glass_applied" }));
    }

    #[test]
    fn validate_enforcement_allows_full_enforcement_runtime_violation_with_valid_break_glass() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(valid_break_glass());

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "allow_with_warnings");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "full_enforcement_break_glass_applied"));
    }

    #[test]
    fn validate_enforcement_rejects_break_glass_when_scope_does_not_cover_selected_operator() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(BreakGlassActivation {
            scope_operator_ids: vec!["operator-1".to_string()],
            ..valid_break_glass()
        });

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "break_glass_scope_operator_not_covered" }));
    }

    #[test]
    fn validate_enforcement_rejects_break_glass_when_activation_limit_exceeded() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(valid_break_glass());
        context.break_glass_history = vec![
            BreakGlassActivationHistoryRecord {
                incident_ticket: "INC-100".to_string(),
                activated_at_unix: 1_700_090_000,
            },
            BreakGlassActivationHistoryRecord {
                incident_ticket: "INC-101".to_string(),
                activated_at_unix: 1_700_095_000,
            },
        ];

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_activation_limit_exceeded"));
    }

    #[test]
    fn validate_enforcement_rejects_break_glass_when_cooldown_violated() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(valid_break_glass());
        context.break_glass_history = vec![BreakGlassActivationHistoryRecord {
            incident_ticket: "INC-100".to_string(),
            activated_at_unix: 1_700_099_999,
        }];

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_cooldown_violation"));
    }

    #[test]
    fn validate_enforcement_rejects_break_glass_when_quorum_below_required() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(BreakGlassActivation {
            approver_quorum_bps: 6_600,
            ..valid_break_glass()
        });

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_quorum_below_required"));
    }

    #[test]
    fn validate_enforcement_rejects_break_glass_when_ttl_exceeds_policy() {
        let mut registry = baseline_registry();
        registry.enforcement.break_glass_ttl_seconds = 1_800;
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(valid_break_glass());

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_ttl_exceeds_policy"));
    }

    #[test]
    fn validate_enforcement_infers_new_activation_from_history_when_hint_false() {
        let mut registry = baseline_registry();
        registry.enforcement.break_glass_max_activations_per_7d = 1;
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(BreakGlassActivation {
            is_new_activation: false,
            ..valid_break_glass()
        });
        context.break_glass_history = vec![BreakGlassActivationHistoryRecord {
            incident_ticket: "INC-100".to_string(),
            activated_at_unix: 1_700_099_000,
        }];

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_activation_limit_exceeded"));
    }

    #[test]
    fn validate_enforcement_treats_existing_incident_as_reuse_even_when_hint_true() {
        let mut registry = baseline_registry();
        registry.enforcement.break_glass_max_activations_per_7d = 1;
        registry.enforcement.break_glass_cooldown_seconds = 86_400;
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(BreakGlassActivation {
            incident_ticket: "INC-123".to_string(),
            is_new_activation: true,
            ..valid_break_glass()
        });
        context.break_glass_history = vec![BreakGlassActivationHistoryRecord {
            incident_ticket: "INC-123".to_string(),
            activated_at_unix: 1_700_099_000,
        }];

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "allow_with_warnings");
        assert!(!decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_activation_limit_exceeded"));
        assert!(!decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_cooldown_violation"));
    }

    #[test]
    fn validate_enforcement_rejects_reused_incident_with_refreshed_window() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(BreakGlassActivation {
            incident_ticket: "INC-123".to_string(),
            declared_at_unix: 1_700_199_000,
            expires_at_unix: 1_700_203_600,
            is_new_activation: false,
            ..valid_break_glass()
        });
        context.break_glass_history = vec![BreakGlassActivationHistoryRecord {
            incident_ticket: "INC-123".to_string(),
            activated_at_unix: 1_700_000_000,
        }];

        let decision = validate_enforcement(&registry, &context, 1_700_200_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_reused_incident_declared_at_mismatch"));
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_reused_incident_expired"));
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_reused_incident_extends_ttl"));
    }

    #[test]
    fn validate_enforcement_accepts_hyphenated_allow_with_warnings_runtime_decision() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.enforcement_mode = "monitor_only".to_string();
        context.runtime_decision.decision = "allow-with-warnings".to_string();

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "allow_with_warnings");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "monitor_only_runtime_violation_observed"));
    }

    #[test]
    fn validate_enforcement_rejects_invalid_enforcement_mode() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.enforcement_mode = "invalid_mode".to_string();

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "enforcement_mode_invalid"));
    }

    #[test]
    fn validate_enforcement_rejects_invalid_runtime_decision_state() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.runtime_decision.decision = "unknown".to_string();

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "runtime_decision_invalid"));
    }

    #[test]
    fn validate_enforcement_rejects_non_mandatory_profile_in_hard_enforcement_canary_mode() {
        let mut registry = baseline_registry();
        registry.profile_status = "draft".to_string();
        let mut context = baseline_context();
        context.enforcement_mode = "hard_enforcement_canary".to_string();

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "governance_profile_not_mandatory_for_strict_mode" }));
    }

    #[test]
    fn validate_enforcement_rejects_non_mandatory_profile_in_full_enforcement_mode() {
        let mut registry = baseline_registry();
        registry.profile_status = "draft".to_string();
        let context = baseline_context();

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "governance_profile_not_mandatory_for_strict_mode" }));
    }

    #[test]
    fn validate_enforcement_allows_soft_mode_even_with_invalid_break_glass() {
        let registry = baseline_registry();
        let mut context = baseline_context();
        context.enforcement_mode = "soft_enforcement".to_string();
        context.runtime_decision = runtime_reject();
        context.break_glass = Some(BreakGlassActivation {
            scope_operator_ids: vec![],
            ..valid_break_glass()
        });

        let decision = validate_enforcement(&registry, &context, 1_700_100_000);
        assert_eq!(decision.decision, "allow_with_warnings");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_scope_empty"));
    }

    #[test]
    fn parse_args_accepts_required_flags() {
        let args = vec![
            "--registry".to_string(),
            "registry.json".to_string(),
            "--context".to_string(),
            "context.json".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.registry_path, PathBuf::from("registry.json"));
        assert_eq!(parsed.context_path, PathBuf::from("context.json"));
        assert!(parsed.now_unix_override.is_none());
    }

    #[test]
    fn parse_args_accepts_now_unix() {
        let args = vec![
            "--registry".to_string(),
            "registry.json".to_string(),
            "--context".to_string(),
            "context.json".to_string(),
            "--now-unix".to_string(),
            "1700100000".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.now_unix_override, Some(1_700_100_000));
    }

    #[test]
    fn parse_args_rejects_missing_context_flag() {
        let args = vec!["--registry".to_string(), "registry.json".to_string()];

        let error = parse_args(&args).expect_err("expected parse failure");
        assert_eq!(error, "missing required --context");
    }
}
