use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet, VecDeque};
use std::env;
use std::fs;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

const SECONDS_PER_DAY: u64 = 86_400;
const SECONDS_PER_7_DAYS: u64 = 7 * SECONDS_PER_DAY;
const MAX_ATTESTATION_MAX_AGE_SECONDS: u64 = SECONDS_PER_DAY;
const MAX_DENYLIST_STALENESS_SECONDS: u64 = 300;

#[derive(Clone, Debug, Deserialize)]
struct TeeGovernanceRegistryV1 {
    profile_status: String,
    enforcement: TeeEnforcementParameters,
    operators: Vec<TeeOperatorAdmissionRecord>,
    #[serde(default)]
    activation_gate: Option<TeeActivationGateRecord>,
}

#[derive(Clone, Debug, Deserialize)]
struct TeeOperatorAdmissionRecord {
    operator_id: String,
    signer_identifier: String,
    status: String,
    allowed_tee_types: Vec<String>,
    allowed_measurements: Vec<String>,
    attestation_max_age_seconds: u64,
    grace_period_seconds: u64,
    effective_from: u64,
    #[serde(default)]
    effective_until: Option<u64>,
}

#[derive(Clone, Debug, Deserialize)]
struct TeeEnforcementParameters {
    attestation_max_age_seconds: u64,
    grace_period_seconds: u64,
    min_attested_signers_per_cohort: u64,
    max_single_vendor_share_percent: u64,
    denylist_max_staleness_seconds: u64,
    break_glass_ttl_seconds: u64,
    break_glass_max_activations_per_7d: u64,
    break_glass_cooldown_seconds: u64,
    break_glass_scope: String,
    break_glass_quorum_bps: u64,
    activation_gate_required_quorum_bps: u64,
    re_attestation_poll_interval_seconds: u64,
}

#[derive(Clone, Debug, Deserialize)]
struct TeeActivationGateRecord {
    governance_decision_id: String,
    effective_at_unix: u64,
    quorum_denominator: u64,
    achieved_quorum_bps: u64,
    approvers: Vec<TeeActivationApprover>,
    profile_status_transition: String,
    rollback_condition: String,
    rollback_authority: String,
}

#[derive(Clone, Debug, Deserialize)]
struct TeeActivationApprover {
    approver_id: String,
    role: String,
    decision: String,
    decided_at_unix: u64,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "snake_case")]
enum GovernanceEventType {
    Add,
    Suspend,
    Revoke,
    MeasurementUpdate,
    BreakGlassActivate,
    BreakGlassExpire,
}

#[derive(Clone, Debug, Deserialize)]
struct GovernanceAuditEvent {
    event_id: String,
    event_type: GovernanceEventType,
    #[serde(default)]
    operator_id: Option<String>,
    #[serde(default)]
    signer_identifier: Option<String>,
    #[serde(default)]
    measurement_digest: Option<String>,
    governance_decision_id: String,
    effective_at_unix: u64,
    #[serde(default)]
    incident_ticket: Option<String>,
    #[serde(default)]
    scope_operator_ids: Option<Vec<String>>,
    #[serde(default)]
    expires_at_unix: Option<u64>,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(untagged)]
enum GovernanceAuditInput {
    Events(Vec<GovernanceAuditEvent>),
    Envelope { events: Vec<GovernanceAuditEvent> },
}

impl GovernanceAuditInput {
    fn into_events(self) -> Vec<GovernanceAuditEvent> {
        match self {
            GovernanceAuditInput::Events(events) => events,
            GovernanceAuditInput::Envelope { events } => events,
        }
    }
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
    events_path: Option<PathBuf>,
    now_unix_override: Option<u64>,
}

#[derive(Copy, Clone, Debug, PartialEq, Eq)]
enum ProfileStatus {
    Draft,
    Mandatory,
}

#[derive(Copy, Clone, Debug, PartialEq, Eq)]
enum OperatorStatus {
    Active,
    Suspended,
    Revoked,
}

fn usage() -> String {
    "Usage: tee_registry_checker --registry <path> [--events <path>] [--now-unix <seconds>]"
        .to_string()
}

fn parse_args(args: &[String]) -> Result<CliArgs, String> {
    let mut registry_path: Option<PathBuf> = None;
    let mut events_path: Option<PathBuf> = None;
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
            "--events" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --events".to_string());
                }
                events_path = Some(PathBuf::from(&args[i]));
            }
            "--now-unix" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --now-unix".to_string());
                }
                let parsed = args[i]
                    .parse::<u64>()
                    .map_err(|_| "invalid value for --now-unix".to_string())?;
                now_unix_override = Some(parsed);
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

    Ok(CliArgs {
        registry_path,
        events_path,
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

fn push_rejection_reason(reasons: &mut Vec<ValidationReason>, code: &str, detail: String) {
    reasons.push(ValidationReason {
        code: code.to_string(),
        detail,
    });
}

fn parse_profile_status(status: &str) -> Option<ProfileStatus> {
    match trimmed_lowercase(status).as_str() {
        "draft" => Some(ProfileStatus::Draft),
        "mandatory" => Some(ProfileStatus::Mandatory),
        _ => None,
    }
}

fn parse_operator_status(status: &str) -> Option<OperatorStatus> {
    match trimmed_lowercase(status).as_str() {
        "active" => Some(OperatorStatus::Active),
        "suspended" => Some(OperatorStatus::Suspended),
        "revoked" => Some(OperatorStatus::Revoked),
        _ => None,
    }
}

fn is_sha256_digest(value: &str) -> bool {
    let normalized = value.trim();
    if normalized.len() != 71 {
        return false;
    }
    if !normalized
        .get(0..7)
        .is_some_and(|prefix| prefix.eq_ignore_ascii_case("sha256:"))
    {
        return false;
    }
    normalized.get(7..).is_some_and(|digest| {
        digest
            .chars()
            .all(|character| character.is_ascii_hexdigit())
    })
}

fn required_non_empty(
    field_name: &str,
    value: &str,
    reasons: &mut Vec<ValidationReason>,
    code: &str,
) -> Option<String> {
    let normalized = value.trim().to_string();
    if normalized.is_empty() {
        push_rejection_reason(
            reasons,
            code,
            format!("field [{field_name}] must be non-empty"),
        );
        return None;
    }
    Some(normalized)
}

fn validate_non_empty_string_list(
    field_name: &str,
    values: &[String],
    reasons: &mut Vec<ValidationReason>,
    empty_code: &str,
    item_code: &str,
) {
    if values.is_empty() {
        push_rejection_reason(
            reasons,
            empty_code,
            format!("field [{field_name}] must include at least one value"),
        );
        return;
    }

    for value in values {
        if value.trim().is_empty() {
            push_rejection_reason(
                reasons,
                item_code,
                format!("field [{field_name}] contains an empty item"),
            );
            return;
        }
    }
}

fn validate_enforcement(
    enforcement: &TeeEnforcementParameters,
    reasons: &mut Vec<ValidationReason>,
) {
    if enforcement.attestation_max_age_seconds == 0 {
        push_rejection_reason(
            reasons,
            "attestation_max_age_invalid",
            "enforcement.attestation_max_age_seconds must be > 0".to_string(),
        );
    } else if enforcement.attestation_max_age_seconds > MAX_ATTESTATION_MAX_AGE_SECONDS {
        push_rejection_reason(
            reasons,
            "attestation_max_age_exceeds_hard_ceiling",
            format!(
                "enforcement.attestation_max_age_seconds [{}] exceeds hard ceiling [{}]",
                enforcement.attestation_max_age_seconds, MAX_ATTESTATION_MAX_AGE_SECONDS
            ),
        );
    }

    if enforcement.grace_period_seconds > enforcement.attestation_max_age_seconds {
        push_rejection_reason(
            reasons,
            "grace_period_exceeds_attestation_max_age",
            format!(
                "enforcement.grace_period_seconds [{}] exceeds attestation_max_age_seconds [{}]",
                enforcement.grace_period_seconds, enforcement.attestation_max_age_seconds
            ),
        );
    }

    if enforcement.min_attested_signers_per_cohort == 0 {
        push_rejection_reason(
            reasons,
            "min_attested_signers_invalid",
            "enforcement.min_attested_signers_per_cohort must be > 0".to_string(),
        );
    }

    if !(1..=100).contains(&enforcement.max_single_vendor_share_percent) {
        push_rejection_reason(
            reasons,
            "max_single_vendor_share_percent_invalid",
            format!(
                "enforcement.max_single_vendor_share_percent [{}] must be within [1, 100]",
                enforcement.max_single_vendor_share_percent
            ),
        );
    }

    if enforcement.denylist_max_staleness_seconds == 0
        || enforcement.denylist_max_staleness_seconds > MAX_DENYLIST_STALENESS_SECONDS
    {
        push_rejection_reason(
            reasons,
            "denylist_max_staleness_out_of_bounds",
            format!(
                "enforcement.denylist_max_staleness_seconds [{}] must be within [1, {}]",
                enforcement.denylist_max_staleness_seconds, MAX_DENYLIST_STALENESS_SECONDS
            ),
        );
    }

    if enforcement.break_glass_ttl_seconds == 0
        || enforcement.break_glass_ttl_seconds > SECONDS_PER_7_DAYS
    {
        push_rejection_reason(
            reasons,
            "break_glass_ttl_invalid",
            format!(
                "enforcement.break_glass_ttl_seconds [{}] must be within [1, {}]",
                enforcement.break_glass_ttl_seconds, SECONDS_PER_7_DAYS
            ),
        );
    }

    if enforcement.break_glass_max_activations_per_7d == 0 {
        push_rejection_reason(
            reasons,
            "break_glass_max_activations_invalid",
            "enforcement.break_glass_max_activations_per_7d must be > 0".to_string(),
        );
    }

    if enforcement.break_glass_cooldown_seconds == 0
        || enforcement.break_glass_cooldown_seconds > SECONDS_PER_7_DAYS
    {
        push_rejection_reason(
            reasons,
            "break_glass_cooldown_invalid",
            format!(
                "enforcement.break_glass_cooldown_seconds [{}] must be within [1, {}]",
                enforcement.break_glass_cooldown_seconds, SECONDS_PER_7_DAYS
            ),
        );
    }

    if trimmed_lowercase(&enforcement.break_glass_scope) != "named_operator_ids_only" {
        push_rejection_reason(
            reasons,
            "break_glass_scope_not_supported",
            format!(
                "enforcement.break_glass_scope must be [named_operator_ids_only], got [{}]",
                enforcement.break_glass_scope
            ),
        );
    }

    if !(1..=10_000).contains(&enforcement.break_glass_quorum_bps) {
        push_rejection_reason(
            reasons,
            "break_glass_quorum_invalid",
            format!(
                "enforcement.break_glass_quorum_bps [{}] must be within [1, 10000]",
                enforcement.break_glass_quorum_bps
            ),
        );
    }

    if !(6_700..=10_000).contains(&enforcement.activation_gate_required_quorum_bps) {
        push_rejection_reason(
            reasons,
            "activation_gate_required_quorum_invalid",
            format!(
                "enforcement.activation_gate_required_quorum_bps [{}] must be within [6700, 10000]",
                enforcement.activation_gate_required_quorum_bps
            ),
        );
    }

    if enforcement.re_attestation_poll_interval_seconds == 0 {
        push_rejection_reason(
            reasons,
            "re_attestation_poll_interval_invalid",
            "enforcement.re_attestation_poll_interval_seconds must be > 0".to_string(),
        );
    } else if enforcement.re_attestation_poll_interval_seconds
        > enforcement.attestation_max_age_seconds
    {
        push_rejection_reason(
            reasons,
            "re_attestation_poll_interval_exceeds_attestation_max_age",
            format!(
                "enforcement.re_attestation_poll_interval_seconds [{}] exceeds attestation_max_age_seconds [{}]",
                enforcement.re_attestation_poll_interval_seconds,
                enforcement.attestation_max_age_seconds
            ),
        );
    }
}

fn validate_operator_records(
    operators: &[TeeOperatorAdmissionRecord],
    now_unix_seconds: u64,
    reasons: &mut Vec<ValidationReason>,
) {
    let mut operator_ids = HashSet::new();
    let mut signer_identifiers = HashSet::new();

    for operator in operators {
        let Some(operator_id) = required_non_empty(
            "operator_id",
            &operator.operator_id,
            reasons,
            "operator_id_missing",
        ) else {
            continue;
        };

        let operator_id_normalized = trimmed_lowercase(&operator_id);
        if !operator_ids.insert(operator_id_normalized.clone()) {
            push_rejection_reason(
                reasons,
                "operator_id_duplicate",
                format!(
                    "operator_id [{}] is duplicated in registry",
                    operator.operator_id
                ),
            );
        }

        let Some(signer_identifier) = required_non_empty(
            "signer_identifier",
            &operator.signer_identifier,
            reasons,
            "signer_identifier_missing",
        ) else {
            continue;
        };

        let signer_identifier_normalized = trimmed_lowercase(&signer_identifier);
        if !signer_identifiers.insert(signer_identifier_normalized) {
            push_rejection_reason(
                reasons,
                "signer_identifier_duplicate",
                format!(
                    "signer_identifier [{}] is duplicated in registry",
                    operator.signer_identifier
                ),
            );
        }

        if parse_operator_status(&operator.status).is_none() {
            push_rejection_reason(
                reasons,
                "operator_status_invalid",
                format!(
                    "operator [{}] has invalid status [{}]; expected one of [active, suspended, revoked]",
                    operator.operator_id, operator.status
                ),
            );
        }

        validate_non_empty_string_list(
            "allowed_tee_types",
            &operator.allowed_tee_types,
            reasons,
            "allowed_tee_types_missing",
            "allowed_tee_types_contains_empty",
        );

        validate_non_empty_string_list(
            "allowed_measurements",
            &operator.allowed_measurements,
            reasons,
            "allowed_measurements_missing",
            "allowed_measurements_contains_empty",
        );
        for measurement in &operator.allowed_measurements {
            if !is_sha256_digest(measurement) {
                push_rejection_reason(
                    reasons,
                    "allowed_measurement_digest_invalid",
                    format!(
                        "operator [{}] allowed_measurements entry [{}] must match sha256:<64 hex chars>",
                        operator.operator_id, measurement
                    ),
                );
            }
        }

        if operator.attestation_max_age_seconds == 0 {
            push_rejection_reason(
                reasons,
                "operator_attestation_max_age_invalid",
                format!(
                    "operator [{}] attestation_max_age_seconds must be > 0",
                    operator.operator_id
                ),
            );
        }

        if operator.grace_period_seconds > operator.attestation_max_age_seconds {
            push_rejection_reason(
                reasons,
                "operator_grace_period_exceeds_attestation_max_age",
                format!(
                    "operator [{}] grace_period_seconds [{}] exceeds attestation_max_age_seconds [{}]",
                    operator.operator_id,
                    operator.grace_period_seconds,
                    operator.attestation_max_age_seconds
                ),
            );
        }

        if operator.effective_from == 0 {
            push_rejection_reason(
                reasons,
                "operator_effective_from_invalid",
                format!(
                    "operator [{}] effective_from must be > 0",
                    operator.operator_id
                ),
            );
        }

        if let Some(effective_until) = operator.effective_until {
            if effective_until < operator.effective_from {
                push_rejection_reason(
                    reasons,
                    "operator_effective_window_invalid",
                    format!(
                        "operator [{}] effective_until [{}] is before effective_from [{}]",
                        operator.operator_id, effective_until, operator.effective_from
                    ),
                );
            }

            if effective_until < now_unix_seconds
                && parse_operator_status(&operator.status) == Some(OperatorStatus::Active)
            {
                push_rejection_reason(
                    reasons,
                    "active_operator_window_expired",
                    format!(
                        "operator [{}] is active but effective_until [{}] is in the past relative to now [{}]",
                        operator.operator_id, effective_until, now_unix_seconds
                    ),
                );
            }
        }
    }
}

fn validate_activation_gate(
    profile_status: ProfileStatus,
    activation_gate: Option<&TeeActivationGateRecord>,
    enforcement: &TeeEnforcementParameters,
    now_unix_seconds: u64,
    reasons: &mut Vec<ValidationReason>,
) {
    if profile_status == ProfileStatus::Mandatory && activation_gate.is_none() {
        push_rejection_reason(
            reasons,
            "mandatory_profile_missing_activation_gate",
            "profile_status [mandatory] requires activation_gate record".to_string(),
        );
        return;
    }

    let Some(activation_gate) = activation_gate else {
        return;
    };

    if activation_gate.governance_decision_id.trim().is_empty() {
        push_rejection_reason(
            reasons,
            "activation_gate_decision_id_missing",
            "activation_gate.governance_decision_id must be non-empty".to_string(),
        );
    }

    if activation_gate.effective_at_unix == 0 {
        push_rejection_reason(
            reasons,
            "activation_gate_effective_time_invalid",
            "activation_gate.effective_at_unix must be > 0".to_string(),
        );
    }

    if profile_status == ProfileStatus::Mandatory
        && activation_gate.effective_at_unix > now_unix_seconds
    {
        push_rejection_reason(
            reasons,
            "activation_gate_not_yet_effective",
            format!(
                "mandatory profile requires activation_gate.effective_at_unix [{}] <= now [{}]",
                activation_gate.effective_at_unix, now_unix_seconds
            ),
        );
    }

    if activation_gate.quorum_denominator == 0 {
        push_rejection_reason(
            reasons,
            "activation_gate_quorum_denominator_invalid",
            "activation_gate.quorum_denominator must be > 0".to_string(),
        );
    }

    if activation_gate.achieved_quorum_bps > 10_000 {
        push_rejection_reason(
            reasons,
            "activation_gate_achieved_quorum_invalid",
            format!(
                "activation_gate.achieved_quorum_bps [{}] must be <= 10000",
                activation_gate.achieved_quorum_bps
            ),
        );
    }

    if activation_gate.achieved_quorum_bps < enforcement.activation_gate_required_quorum_bps {
        push_rejection_reason(
            reasons,
            "activation_gate_quorum_below_required",
            format!(
                "activation_gate.achieved_quorum_bps [{}] is below required [{}]",
                activation_gate.achieved_quorum_bps,
                enforcement.activation_gate_required_quorum_bps
            ),
        );
    }

    let transition = trimmed_lowercase(&activation_gate.profile_status_transition).replace(' ', "");
    if transition != "draft->mandatory" {
        push_rejection_reason(
            reasons,
            "activation_gate_transition_invalid",
            format!(
                "activation_gate.profile_status_transition must be [draft -> mandatory], got [{}]",
                activation_gate.profile_status_transition
            ),
        );
    }

    if activation_gate.rollback_condition.trim().is_empty() {
        push_rejection_reason(
            reasons,
            "activation_gate_rollback_condition_missing",
            "activation_gate.rollback_condition must be non-empty".to_string(),
        );
    }

    if activation_gate.rollback_authority.trim().is_empty() {
        push_rejection_reason(
            reasons,
            "activation_gate_rollback_authority_missing",
            "activation_gate.rollback_authority must be non-empty".to_string(),
        );
    }

    if activation_gate.approvers.is_empty() {
        push_rejection_reason(
            reasons,
            "activation_gate_approvers_missing",
            "activation_gate.approvers must include required roles".to_string(),
        );
        return;
    }

    let mut role_decisions: HashMap<String, bool> = HashMap::new();
    let mut seen_approver_ids: HashSet<String> = HashSet::new();
    for approver in &activation_gate.approvers {
        if approver.approver_id.trim().is_empty() {
            push_rejection_reason(
                reasons,
                "activation_gate_approver_id_missing",
                "activation_gate approver_id must be non-empty".to_string(),
            );
        }

        let approver_id_normalized = trimmed_lowercase(&approver.approver_id);
        if !approver_id_normalized.is_empty() && !seen_approver_ids.insert(approver_id_normalized) {
            push_rejection_reason(
                reasons,
                "activation_gate_approver_id_duplicate",
                format!(
                    "activation_gate approver_id [{}] appears more than once",
                    approver.approver_id
                ),
            );
        }

        if approver.decided_at_unix == 0 {
            push_rejection_reason(
                reasons,
                "activation_gate_approver_timestamp_invalid",
                format!(
                    "activation_gate approver [{}] decided_at_unix must be > 0",
                    approver.approver_id
                ),
            );
        }

        let role_normalized = trimmed_lowercase(&approver.role);
        let decision_normalized = trimmed_lowercase(&approver.decision);
        let approved = decision_normalized == "approved";

        if !matches!(
            role_normalized.as_str(),
            "security_owner" | "signer_runtime_owner" | "governance_delegate"
        ) {
            push_rejection_reason(
                reasons,
                "activation_gate_role_invalid",
                format!(
                    "activation_gate approver role [{}] is unsupported",
                    approver.role
                ),
            );
            continue;
        }

        if role_decisions.contains_key(&role_normalized) {
            push_rejection_reason(
                reasons,
                "activation_gate_role_duplicate",
                format!(
                    "activation_gate approver role [{}] appears more than once",
                    approver.role
                ),
            );
            continue;
        }

        role_decisions.insert(role_normalized, approved);
    }

    for required_role in [
        "security_owner",
        "signer_runtime_owner",
        "governance_delegate",
    ] {
        match role_decisions.get(required_role) {
            Some(true) => {}
            Some(false) => push_rejection_reason(
                reasons,
                "activation_gate_role_not_approved",
                format!(
                    "activation_gate role [{}] is present but not approved",
                    required_role
                ),
            ),
            None => push_rejection_reason(
                reasons,
                "activation_gate_role_missing",
                format!("activation_gate missing required role [{}]", required_role),
            ),
        }
    }
}

fn validate_governance_events(
    registry: &TeeGovernanceRegistryV1,
    events: &[GovernanceAuditEvent],
    now_unix_seconds: u64,
    reasons: &mut Vec<ValidationReason>,
) {
    if events.is_empty() {
        push_rejection_reason(
            reasons,
            "audit_events_missing",
            "events file was provided but contains no governance events".to_string(),
        );
        return;
    }

    for window in events.windows(2) {
        if window[0].effective_at_unix > window[1].effective_at_unix {
            push_rejection_reason(
                reasons,
                "audit_events_not_chronological",
                format!(
                    "event [{}] has effective_at_unix [{}] later than following event [{}] at [{}]",
                    window[0].event_id,
                    window[0].effective_at_unix,
                    window[1].event_id,
                    window[1].effective_at_unix
                ),
            );
            break;
        }
    }

    let mut seen_event_ids: HashSet<String> = HashSet::new();
    let mut status_by_operator: HashMap<String, OperatorStatus> = HashMap::new();
    let mut signer_by_operator: HashMap<String, String> = HashMap::new();
    let mut active_break_glass_incidents: HashMap<String, u64> = HashMap::new();
    let mut recent_break_glass_activations: VecDeque<u64> = VecDeque::new();
    let mut last_break_glass_activation_unix: Option<u64> = None;

    for event in events {
        let event_id = match required_non_empty(
            "event_id",
            &event.event_id,
            reasons,
            "audit_event_id_missing",
        ) {
            Some(event_id) => event_id,
            None => continue,
        };

        let event_id_normalized = trimmed_lowercase(&event_id);
        if !seen_event_ids.insert(event_id_normalized) {
            push_rejection_reason(
                reasons,
                "audit_event_id_duplicate",
                format!("duplicate governance event_id [{}]", event.event_id),
            );
        }

        if event.effective_at_unix == 0 {
            push_rejection_reason(
                reasons,
                "audit_event_effective_time_invalid",
                format!("event [{}] effective_at_unix must be > 0", event.event_id),
            );
        }

        if event.governance_decision_id.trim().is_empty() {
            push_rejection_reason(
                reasons,
                "audit_event_decision_id_missing",
                format!(
                    "event [{}] governance_decision_id must be non-empty",
                    event.event_id
                ),
            );
        }

        match event.event_type {
            GovernanceEventType::Add => {
                let Some(operator_id) = event.operator_id.as_ref().and_then(|value| {
                    required_non_empty(
                        "operator_id",
                        value,
                        reasons,
                        "audit_add_operator_id_missing",
                    )
                }) else {
                    continue;
                };

                let Some(signer_identifier) = event.signer_identifier.as_ref().and_then(|value| {
                    required_non_empty(
                        "signer_identifier",
                        value,
                        reasons,
                        "audit_add_signer_identifier_missing",
                    )
                }) else {
                    continue;
                };

                let operator_id_normalized = trimmed_lowercase(&operator_id);
                if status_by_operator.contains_key(&operator_id_normalized) {
                    push_rejection_reason(
                        reasons,
                        "audit_add_duplicate_operator",
                        format!(
                            "event [{}] adds operator [{}] more than once",
                            event.event_id, operator_id
                        ),
                    );
                    continue;
                }

                status_by_operator.insert(operator_id_normalized.clone(), OperatorStatus::Active);
                signer_by_operator.insert(operator_id_normalized, signer_identifier);
            }
            GovernanceEventType::Suspend => {
                let Some(operator_id) = event.operator_id.as_ref().and_then(|value| {
                    required_non_empty(
                        "operator_id",
                        value,
                        reasons,
                        "audit_suspend_operator_id_missing",
                    )
                }) else {
                    continue;
                };

                let operator_id_normalized = trimmed_lowercase(&operator_id);
                let Some(status) = status_by_operator.get_mut(&operator_id_normalized) else {
                    push_rejection_reason(
                        reasons,
                        "audit_suspend_unknown_operator",
                        format!(
                            "event [{}] suspends unknown operator [{}]",
                            event.event_id, operator_id
                        ),
                    );
                    continue;
                };

                if *status == OperatorStatus::Revoked {
                    push_rejection_reason(
                        reasons,
                        "audit_suspend_after_revoke",
                        format!(
                            "event [{}] cannot suspend revoked operator [{}]",
                            event.event_id, operator_id
                        ),
                    );
                    continue;
                }

                *status = OperatorStatus::Suspended;
            }
            GovernanceEventType::Revoke => {
                let Some(operator_id) = event.operator_id.as_ref().and_then(|value| {
                    required_non_empty(
                        "operator_id",
                        value,
                        reasons,
                        "audit_revoke_operator_id_missing",
                    )
                }) else {
                    continue;
                };

                let operator_id_normalized = trimmed_lowercase(&operator_id);
                let Some(status) = status_by_operator.get_mut(&operator_id_normalized) else {
                    push_rejection_reason(
                        reasons,
                        "audit_revoke_unknown_operator",
                        format!(
                            "event [{}] revokes unknown operator [{}]",
                            event.event_id, operator_id
                        ),
                    );
                    continue;
                };

                *status = OperatorStatus::Revoked;
            }
            GovernanceEventType::MeasurementUpdate => {
                let Some(operator_id) = event.operator_id.as_ref().and_then(|value| {
                    required_non_empty(
                        "operator_id",
                        value,
                        reasons,
                        "audit_measurement_update_operator_id_missing",
                    )
                }) else {
                    continue;
                };

                let Some(measurement_digest) =
                    event.measurement_digest.as_ref().and_then(|value| {
                        required_non_empty(
                            "measurement_digest",
                            value,
                            reasons,
                            "audit_measurement_digest_missing",
                        )
                    })
                else {
                    continue;
                };

                let operator_id_normalized = trimmed_lowercase(&operator_id);
                if !status_by_operator.contains_key(&operator_id_normalized) {
                    push_rejection_reason(
                        reasons,
                        "audit_measurement_update_unknown_operator",
                        format!(
                            "event [{}] updates measurement for unknown operator [{}]",
                            event.event_id, operator_id
                        ),
                    );
                    continue;
                }

                if !is_sha256_digest(&measurement_digest) {
                    push_rejection_reason(
                        reasons,
                        "audit_measurement_digest_invalid_format",
                        format!(
                            "event [{}] measurement_digest [{}] must match sha256:<64 hex chars>",
                            event.event_id, measurement_digest
                        ),
                    );
                }
            }
            GovernanceEventType::BreakGlassActivate => {
                let Some(incident_ticket) = event.incident_ticket.as_ref().and_then(|value| {
                    required_non_empty(
                        "incident_ticket",
                        value,
                        reasons,
                        "audit_break_glass_ticket_missing",
                    )
                }) else {
                    continue;
                };

                let Some(expires_at_unix) = event.expires_at_unix else {
                    push_rejection_reason(
                        reasons,
                        "audit_break_glass_expiry_missing",
                        format!(
                            "event [{}] break_glass_activate requires expires_at_unix",
                            event.event_id
                        ),
                    );
                    continue;
                };

                if expires_at_unix <= event.effective_at_unix {
                    push_rejection_reason(
                        reasons,
                        "audit_break_glass_expiry_invalid",
                        format!(
                            "event [{}] expires_at_unix [{}] must be greater than effective_at_unix [{}]",
                            event.event_id, expires_at_unix, event.effective_at_unix
                        ),
                    );
                }

                let ttl_seconds = expires_at_unix.saturating_sub(event.effective_at_unix);
                if ttl_seconds > registry.enforcement.break_glass_ttl_seconds {
                    push_rejection_reason(
                        reasons,
                        "audit_break_glass_ttl_exceeds_policy",
                        format!(
                            "event [{}] break-glass ttl [{}] exceeds policy max [{}]",
                            event.event_id,
                            ttl_seconds,
                            registry.enforcement.break_glass_ttl_seconds
                        ),
                    );
                }

                let scope_operator_ids = event.scope_operator_ids.clone().unwrap_or_default();
                if scope_operator_ids.is_empty() {
                    push_rejection_reason(
                        reasons,
                        "audit_break_glass_scope_missing",
                        format!(
                            "event [{}] break_glass_activate requires non-empty scope_operator_ids",
                            event.event_id
                        ),
                    );
                }

                for scoped_operator_id in scope_operator_ids {
                    if scoped_operator_id.trim().is_empty() {
                        push_rejection_reason(
                            reasons,
                            "audit_break_glass_scope_contains_empty",
                            format!(
                                "event [{}] scope_operator_ids contains an empty operator_id",
                                event.event_id
                            ),
                        );
                        continue;
                    }

                    let scoped_operator_id_normalized = trimmed_lowercase(&scoped_operator_id);
                    match status_by_operator.get(&scoped_operator_id_normalized) {
                        None => {
                            push_rejection_reason(
                                reasons,
                                "audit_break_glass_scope_unknown_operator",
                                format!(
                                    "event [{}] scope operator [{}] has no prior add event",
                                    event.event_id, scoped_operator_id
                                ),
                            );
                        }
                        Some(OperatorStatus::Revoked) => {
                            push_rejection_reason(
                                reasons,
                                "audit_break_glass_scope_revoked_operator",
                                format!(
                                    "event [{}] scope operator [{}] is revoked; break-glass scope must target non-revoked operators",
                                    event.event_id, scoped_operator_id
                                ),
                            );
                        }
                        Some(_) => {}
                    }
                }

                while let Some(front) = recent_break_glass_activations.front() {
                    if event.effective_at_unix.saturating_sub(*front) > SECONDS_PER_7_DAYS {
                        let _ = recent_break_glass_activations.pop_front();
                    } else {
                        break;
                    }
                }
                recent_break_glass_activations.push_back(event.effective_at_unix);

                if recent_break_glass_activations.len()
                    > registry.enforcement.break_glass_max_activations_per_7d as usize
                {
                    push_rejection_reason(
                        reasons,
                        "audit_break_glass_activation_limit_exceeded",
                        format!(
                            "event [{}] exceeds break_glass_max_activations_per_7d [{}]",
                            event.event_id, registry.enforcement.break_glass_max_activations_per_7d
                        ),
                    );
                }

                if let Some(last_activation) = last_break_glass_activation_unix {
                    let elapsed = event.effective_at_unix.saturating_sub(last_activation);
                    if elapsed < registry.enforcement.break_glass_cooldown_seconds {
                        push_rejection_reason(
                            reasons,
                            "audit_break_glass_cooldown_violation",
                            format!(
                                "event [{}] violates break-glass cooldown: elapsed [{}] < required [{}]",
                                event.event_id, elapsed, registry.enforcement.break_glass_cooldown_seconds
                            ),
                        );
                    }
                }
                last_break_glass_activation_unix = Some(event.effective_at_unix);

                let incident_ticket_normalized = trimmed_lowercase(&incident_ticket);
                if active_break_glass_incidents
                    .insert(incident_ticket_normalized.clone(), expires_at_unix)
                    .is_some()
                {
                    push_rejection_reason(
                        reasons,
                        "audit_break_glass_duplicate_incident",
                        format!(
                            "event [{}] activates already-active break-glass incident [{}]",
                            event.event_id, incident_ticket
                        ),
                    );
                }
            }
            GovernanceEventType::BreakGlassExpire => {
                let Some(incident_ticket) = event.incident_ticket.as_ref().and_then(|value| {
                    required_non_empty(
                        "incident_ticket",
                        value,
                        reasons,
                        "audit_break_glass_expire_ticket_missing",
                    )
                }) else {
                    continue;
                };

                let incident_ticket_normalized = trimmed_lowercase(&incident_ticket);
                let Some(activated_expires_at_unix) =
                    active_break_glass_incidents.remove(&incident_ticket_normalized)
                else {
                    push_rejection_reason(
                        reasons,
                        "audit_break_glass_expire_without_activation",
                        format!(
                            "event [{}] expires unknown break-glass incident [{}]",
                            event.event_id, incident_ticket
                        ),
                    );
                    continue;
                };

                if event.effective_at_unix > activated_expires_at_unix {
                    push_rejection_reason(
                        reasons,
                        "audit_break_glass_expire_after_ttl",
                        format!(
                            "event [{}] expires incident [{}] after ttl deadline [{}]",
                            event.event_id, incident_ticket, activated_expires_at_unix
                        ),
                    );
                }
            }
        }
    }

    for (incident_ticket, expires_at_unix) in active_break_glass_incidents {
        if expires_at_unix <= now_unix_seconds {
            push_rejection_reason(
                reasons,
                "audit_break_glass_missing_expire_event",
                format!(
                    "break-glass incident [{}] expired at [{}] without break_glass_expire event",
                    incident_ticket, expires_at_unix
                ),
            );
        }
    }

    let registry_operator_ids: HashSet<String> = registry
        .operators
        .iter()
        .map(|operator| trimmed_lowercase(&operator.operator_id))
        .collect();

    for operator in &registry.operators {
        let operator_id = trimmed_lowercase(&operator.operator_id);
        let Some(expected_status) = parse_operator_status(&operator.status) else {
            continue;
        };

        match status_by_operator.get(&operator_id) {
            Some(actual_status) => {
                if actual_status != &expected_status {
                    push_rejection_reason(
                        reasons,
                        "operator_status_mismatch_with_events",
                        format!(
                            "operator [{}] registry status [{:?}] does not match event-derived status [{:?}]",
                            operator.operator_id, expected_status, actual_status
                        ),
                    );
                }
            }
            None => {
                push_rejection_reason(
                    reasons,
                    "operator_missing_add_event",
                    format!(
                        "operator [{}] exists in registry but has no corresponding add event",
                        operator.operator_id
                    ),
                );
            }
        }

        if let Some(event_signer_identifier) = signer_by_operator.get(&operator_id) {
            if trimmed_lowercase(event_signer_identifier)
                != trimmed_lowercase(&operator.signer_identifier)
            {
                push_rejection_reason(
                    reasons,
                    "operator_signer_identifier_mismatch_with_events",
                    format!(
                        "operator [{}] signer_identifier [{}] does not match add-event signer_identifier [{}]",
                        operator.operator_id, operator.signer_identifier, event_signer_identifier
                    ),
                );
            }
        }
    }

    for operator_id in status_by_operator.keys() {
        if !registry_operator_ids.contains(operator_id) {
            push_rejection_reason(
                reasons,
                "operator_present_in_events_missing_from_registry",
                format!(
                    "operator [{}] appears in events but is missing from registry",
                    operator_id
                ),
            );
        }
    }
}

fn validate_registry(
    registry: &TeeGovernanceRegistryV1,
    events: Option<&[GovernanceAuditEvent]>,
    now_unix_seconds: u64,
) -> ValidationDecision {
    let mut reasons: Vec<ValidationReason> = Vec::new();

    let profile_status = match parse_profile_status(&registry.profile_status) {
        Some(profile_status) => profile_status,
        None => {
            push_rejection_reason(
                &mut reasons,
                "profile_status_invalid",
                format!(
                    "profile_status [{}] must be one of [draft, mandatory]",
                    registry.profile_status
                ),
            );
            ProfileStatus::Draft
        }
    };

    if registry.operators.is_empty() {
        push_rejection_reason(
            &mut reasons,
            "operators_empty",
            "registry must contain at least one operator record".to_string(),
        );
    }

    validate_enforcement(&registry.enforcement, &mut reasons);
    validate_operator_records(&registry.operators, now_unix_seconds, &mut reasons);
    validate_activation_gate(
        profile_status,
        registry.activation_gate.as_ref(),
        &registry.enforcement,
        now_unix_seconds,
        &mut reasons,
    );

    if let Some(events) = events {
        validate_governance_events(registry, events, now_unix_seconds, &mut reasons);
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
    let events: Option<Vec<GovernanceAuditEvent>> = match cli.events_path.as_ref() {
        Some(path) => {
            let input: GovernanceAuditInput = load_json_file(path)?;
            Some(input.into_events())
        }
        None => None,
    };
    let now_unix_seconds = match cli.now_unix_override {
        Some(now_unix_override) => now_unix_override,
        None => now_unix()?,
    };

    Ok(validate_registry(
        &registry,
        events.as_deref(),
        now_unix_seconds,
    ))
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
                break_glass_ttl_seconds: 21_600,
                break_glass_max_activations_per_7d: 2,
                break_glass_cooldown_seconds: 86_400,
                break_glass_scope: "named_operator_ids_only".to_string(),
                break_glass_quorum_bps: 6_700,
                activation_gate_required_quorum_bps: 6_700,
                re_attestation_poll_interval_seconds: 300,
            },
            operators: vec![
                TeeOperatorAdmissionRecord {
                    operator_id: "operator-1".to_string(),
                    signer_identifier: "signer-1".to_string(),
                    status: "active".to_string(),
                    allowed_tee_types: vec!["sgx".to_string(), "sev-snp".to_string()],
                    allowed_measurements: vec![
                        "sha256:1111111111111111111111111111111111111111111111111111111111111111"
                            .to_string(),
                    ],
                    attestation_max_age_seconds: 3_600,
                    grace_period_seconds: 900,
                    effective_from: 1_700_000_000,
                    effective_until: None,
                },
                TeeOperatorAdmissionRecord {
                    operator_id: "operator-2".to_string(),
                    signer_identifier: "signer-2".to_string(),
                    status: "suspended".to_string(),
                    allowed_tee_types: vec!["tdx".to_string()],
                    allowed_measurements: vec![
                        "sha256:2222222222222222222222222222222222222222222222222222222222222222"
                            .to_string(),
                    ],
                    attestation_max_age_seconds: 3_600,
                    grace_period_seconds: 900,
                    effective_from: 1_700_000_100,
                    effective_until: None,
                },
                TeeOperatorAdmissionRecord {
                    operator_id: "operator-3".to_string(),
                    signer_identifier: "signer-3".to_string(),
                    status: "revoked".to_string(),
                    allowed_tee_types: vec!["sgx".to_string()],
                    allowed_measurements: vec![
                        "sha256:3333333333333333333333333333333333333333333333333333333333333333"
                            .to_string(),
                    ],
                    attestation_max_age_seconds: 3_600,
                    grace_period_seconds: 900,
                    effective_from: 1_700_000_200,
                    effective_until: Some(1_700_000_900),
                },
            ],
            activation_gate: Some(TeeActivationGateRecord {
                governance_decision_id: "proposal-42".to_string(),
                effective_at_unix: 1_700_001_000,
                quorum_denominator: 100_000,
                achieved_quorum_bps: 7_400,
                approvers: vec![
                    TeeActivationApprover {
                        approver_id: "security-owner-1".to_string(),
                        role: "security_owner".to_string(),
                        decision: "approved".to_string(),
                        decided_at_unix: 1_700_000_950,
                    },
                    TeeActivationApprover {
                        approver_id: "runtime-owner-1".to_string(),
                        role: "signer_runtime_owner".to_string(),
                        decision: "approved".to_string(),
                        decided_at_unix: 1_700_000_960,
                    },
                    TeeActivationApprover {
                        approver_id: "delegate-1".to_string(),
                        role: "governance_delegate".to_string(),
                        decision: "approved".to_string(),
                        decided_at_unix: 1_700_000_970,
                    },
                ],
                profile_status_transition: "draft -> mandatory".to_string(),
                rollback_condition: "critical verifier compromise".to_string(),
                rollback_authority: "security council multisig".to_string(),
            }),
        }
    }

    fn baseline_events() -> Vec<GovernanceAuditEvent> {
        vec![
            GovernanceAuditEvent {
                event_id: "evt-1".to_string(),
                event_type: GovernanceEventType::Add,
                operator_id: Some("operator-1".to_string()),
                signer_identifier: Some("signer-1".to_string()),
                measurement_digest: None,
                governance_decision_id: "proposal-10".to_string(),
                effective_at_unix: 1_700_000_010,
                incident_ticket: None,
                scope_operator_ids: None,
                expires_at_unix: None,
            },
            GovernanceAuditEvent {
                event_id: "evt-2".to_string(),
                event_type: GovernanceEventType::Add,
                operator_id: Some("operator-2".to_string()),
                signer_identifier: Some("signer-2".to_string()),
                measurement_digest: None,
                governance_decision_id: "proposal-11".to_string(),
                effective_at_unix: 1_700_000_020,
                incident_ticket: None,
                scope_operator_ids: None,
                expires_at_unix: None,
            },
            GovernanceAuditEvent {
                event_id: "evt-3".to_string(),
                event_type: GovernanceEventType::Add,
                operator_id: Some("operator-3".to_string()),
                signer_identifier: Some("signer-3".to_string()),
                measurement_digest: None,
                governance_decision_id: "proposal-12".to_string(),
                effective_at_unix: 1_700_000_030,
                incident_ticket: None,
                scope_operator_ids: None,
                expires_at_unix: None,
            },
            GovernanceAuditEvent {
                event_id: "evt-4".to_string(),
                event_type: GovernanceEventType::Suspend,
                operator_id: Some("operator-2".to_string()),
                signer_identifier: None,
                measurement_digest: None,
                governance_decision_id: "proposal-20".to_string(),
                effective_at_unix: 1_700_000_100,
                incident_ticket: None,
                scope_operator_ids: None,
                expires_at_unix: None,
            },
            GovernanceAuditEvent {
                event_id: "evt-5".to_string(),
                event_type: GovernanceEventType::Revoke,
                operator_id: Some("operator-3".to_string()),
                signer_identifier: None,
                measurement_digest: None,
                governance_decision_id: "proposal-21".to_string(),
                effective_at_unix: 1_700_000_200,
                incident_ticket: None,
                scope_operator_ids: None,
                expires_at_unix: None,
            },
            GovernanceAuditEvent {
                event_id: "evt-6".to_string(),
                event_type: GovernanceEventType::MeasurementUpdate,
                operator_id: Some("operator-1".to_string()),
                signer_identifier: None,
                measurement_digest: Some(
                    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
                        .to_string(),
                ),
                governance_decision_id: "proposal-22".to_string(),
                effective_at_unix: 1_700_000_300,
                incident_ticket: None,
                scope_operator_ids: None,
                expires_at_unix: None,
            },
            GovernanceAuditEvent {
                event_id: "evt-7".to_string(),
                event_type: GovernanceEventType::BreakGlassActivate,
                operator_id: None,
                signer_identifier: None,
                measurement_digest: None,
                governance_decision_id: "proposal-30".to_string(),
                effective_at_unix: 1_700_000_400,
                incident_ticket: Some("INC-123".to_string()),
                scope_operator_ids: Some(vec!["operator-2".to_string()]),
                expires_at_unix: Some(1_700_004_000),
            },
            GovernanceAuditEvent {
                event_id: "evt-8".to_string(),
                event_type: GovernanceEventType::BreakGlassExpire,
                operator_id: None,
                signer_identifier: None,
                measurement_digest: None,
                governance_decision_id: "proposal-31".to_string(),
                effective_at_unix: 1_700_001_000,
                incident_ticket: Some("INC-123".to_string()),
                scope_operator_ids: None,
                expires_at_unix: None,
            },
        ]
    }

    #[test]
    fn validate_registry_allows_compliant_registry_and_events() {
        let registry = baseline_registry();
        let events = baseline_events();

        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_100_000);
        assert_eq!(decision.decision, "allow");
        assert!(decision.reasons.is_empty());
    }

    #[test]
    fn validate_registry_rejects_mandatory_profile_without_activation_gate() {
        let mut registry = baseline_registry();
        registry.activation_gate = None;
        let events = baseline_events();

        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "mandatory_profile_missing_activation_gate"));
    }

    #[test]
    fn validate_registry_rejects_invalid_break_glass_cooldown() {
        let mut registry = baseline_registry();
        let mut events = baseline_events();

        events.push(GovernanceAuditEvent {
            event_id: "evt-9".to_string(),
            event_type: GovernanceEventType::BreakGlassActivate,
            operator_id: None,
            signer_identifier: None,
            measurement_digest: None,
            governance_decision_id: "proposal-32".to_string(),
            effective_at_unix: 1_700_001_100,
            incident_ticket: Some("INC-456".to_string()),
            scope_operator_ids: Some(vec!["operator-1".to_string()]),
            expires_at_unix: Some(1_700_005_000),
        });

        registry.enforcement.break_glass_cooldown_seconds = 86_400;
        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "audit_break_glass_cooldown_violation"));
    }

    #[test]
    fn validate_registry_rejects_break_glass_activation_limit_violation() {
        let mut registry = baseline_registry();
        let mut events = baseline_events();

        events.push(GovernanceAuditEvent {
            event_id: "evt-9".to_string(),
            event_type: GovernanceEventType::BreakGlassActivate,
            operator_id: None,
            signer_identifier: None,
            measurement_digest: None,
            governance_decision_id: "proposal-32".to_string(),
            effective_at_unix: 1_700_090_000,
            incident_ticket: Some("INC-456".to_string()),
            scope_operator_ids: Some(vec!["operator-1".to_string()]),
            expires_at_unix: Some(1_700_093_000),
        });
        events.push(GovernanceAuditEvent {
            event_id: "evt-10".to_string(),
            event_type: GovernanceEventType::BreakGlassActivate,
            operator_id: None,
            signer_identifier: None,
            measurement_digest: None,
            governance_decision_id: "proposal-33".to_string(),
            effective_at_unix: 1_700_180_000,
            incident_ticket: Some("INC-789".to_string()),
            scope_operator_ids: Some(vec!["operator-2".to_string()]),
            expires_at_unix: Some(1_700_183_000),
        });

        registry.enforcement.break_glass_max_activations_per_7d = 2;
        registry.enforcement.break_glass_cooldown_seconds = 10;

        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_200_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "audit_break_glass_activation_limit_exceeded"));
    }

    #[test]
    fn validate_registry_rejects_status_mismatch_with_events() {
        let mut registry = baseline_registry();
        let events = baseline_events();
        registry.operators[0].status = "suspended".to_string();

        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "operator_status_mismatch_with_events"));
    }

    #[test]
    fn validate_registry_rejects_operators_without_add_events() {
        let registry = baseline_registry();
        let events = vec![];

        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "audit_events_missing"));
    }

    #[test]
    fn validate_registry_rejects_invalid_enforcement_bounds() {
        let mut registry = baseline_registry();
        registry.enforcement.denylist_max_staleness_seconds = MAX_DENYLIST_STALENESS_SECONDS + 1;
        registry.enforcement.break_glass_scope = "global".to_string();

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "denylist_max_staleness_out_of_bounds"));
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_scope_not_supported"));
    }

    #[test]
    fn validate_registry_rejects_attestation_max_age_above_hard_ceiling() {
        let mut registry = baseline_registry();
        registry.enforcement.attestation_max_age_seconds = MAX_ATTESTATION_MAX_AGE_SECONDS + 1;

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "attestation_max_age_exceeds_hard_ceiling"));
    }

    #[test]
    fn validate_registry_rejects_zero_break_glass_cooldown() {
        let mut registry = baseline_registry();
        registry.enforcement.break_glass_cooldown_seconds = 0;

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_cooldown_invalid"));
    }

    #[test]
    fn validate_registry_rejects_break_glass_ttl_exceeding_7_day_max() {
        let mut registry = baseline_registry();
        registry.enforcement.break_glass_ttl_seconds = SECONDS_PER_7_DAYS + 1;

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "break_glass_ttl_invalid"));
    }

    #[test]
    fn validate_registry_rejects_activation_gate_quorum_below_dedicated_minimum() {
        let mut registry = baseline_registry();
        registry.enforcement.break_glass_quorum_bps = 5_000;
        registry.enforcement.activation_gate_required_quorum_bps = 6_700;
        if let Some(activation_gate) = registry.activation_gate.as_mut() {
            activation_gate.achieved_quorum_bps = 6_600;
        }

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "activation_gate_quorum_below_required"));
    }

    #[test]
    fn validate_registry_rejects_duplicate_activation_gate_approver_ids() {
        let mut registry = baseline_registry();
        if let Some(activation_gate) = registry.activation_gate.as_mut() {
            activation_gate.approvers[1].approver_id =
                activation_gate.approvers[0].approver_id.clone();
        }

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "activation_gate_approver_id_duplicate"));
    }

    #[test]
    fn validate_registry_rejects_break_glass_scope_for_revoked_operator() {
        let registry = baseline_registry();
        let mut events = baseline_events();
        events.push(GovernanceAuditEvent {
            event_id: "evt-9".to_string(),
            event_type: GovernanceEventType::BreakGlassActivate,
            operator_id: None,
            signer_identifier: None,
            measurement_digest: None,
            governance_decision_id: "proposal-40".to_string(),
            effective_at_unix: 1_700_090_000,
            incident_ticket: Some("INC-999".to_string()),
            scope_operator_ids: Some(vec!["operator-3".to_string()]),
            expires_at_unix: Some(1_700_093_000),
        });

        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_200_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "audit_break_glass_scope_revoked_operator"));
    }

    #[test]
    fn validate_registry_rejects_invalid_measurement_digest_format() {
        let mut registry = baseline_registry();
        registry.operators[0].allowed_measurements = vec!["sha256:nothex".to_string()];

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "allowed_measurement_digest_invalid"));
    }

    #[test]
    fn validate_registry_rejects_duplicate_operator_ids() {
        let mut registry = baseline_registry();
        registry.operators[1].operator_id = "operator-1".to_string();

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "operator_id_duplicate"));
    }

    #[test]
    fn validate_registry_rejects_duplicate_signer_identifiers() {
        let mut registry = baseline_registry();
        registry.operators[1].signer_identifier = "signer-1".to_string();

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "signer_identifier_duplicate"));
    }

    #[test]
    fn validate_registry_rejects_invalid_operator_status() {
        let mut registry = baseline_registry();
        registry.operators[0].status = "unknown".to_string();

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "operator_status_invalid"));
    }

    #[test]
    fn validate_registry_rejects_empty_allowed_tee_types() {
        let mut registry = baseline_registry();
        registry.operators[0].allowed_tee_types.clear();

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "allowed_tee_types_missing"));
    }

    #[test]
    fn validate_registry_rejects_active_operator_with_expired_window() {
        let mut registry = baseline_registry();
        registry.operators[0].effective_until = Some(1_700_000_500);

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "active_operator_window_expired"));
    }

    #[test]
    fn validate_registry_rejects_missing_activation_gate_rollback_condition() {
        let mut registry = baseline_registry();
        if let Some(activation_gate) = registry.activation_gate.as_mut() {
            activation_gate.rollback_condition.clear();
        }

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| { reason.code == "activation_gate_rollback_condition_missing" }));
    }

    #[test]
    fn validate_registry_rejects_activation_gate_role_not_approved() {
        let mut registry = baseline_registry();
        if let Some(activation_gate) = registry.activation_gate.as_mut() {
            activation_gate.approvers[0].decision = "rejected".to_string();
        }

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "activation_gate_role_not_approved"));
    }

    #[test]
    fn validate_registry_rejects_activation_gate_missing_required_role() {
        let mut registry = baseline_registry();
        if let Some(activation_gate) = registry.activation_gate.as_mut() {
            activation_gate
                .approvers
                .retain(|approver| approver.role != "governance_delegate");
        }

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "activation_gate_role_missing"));
    }

    #[test]
    fn validate_registry_rejects_non_chronological_events() {
        let registry = baseline_registry();
        let mut events = baseline_events();
        events[1].effective_at_unix = events[0].effective_at_unix.saturating_sub(1);

        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "audit_events_not_chronological"));
    }

    #[test]
    fn validate_registry_rejects_duplicate_event_ids() {
        let registry = baseline_registry();
        let mut events = baseline_events();
        events[1].event_id = events[0].event_id.clone();

        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "audit_event_id_duplicate"));
    }

    #[test]
    fn validate_registry_rejects_suspend_after_revoke() {
        let registry = baseline_registry();
        let mut events = baseline_events();
        events.push(GovernanceAuditEvent {
            event_id: "evt-extra".to_string(),
            event_type: GovernanceEventType::Suspend,
            operator_id: Some("operator-3".to_string()),
            signer_identifier: None,
            measurement_digest: None,
            governance_decision_id: "proposal-99".to_string(),
            effective_at_unix: 1_700_001_100,
            incident_ticket: None,
            scope_operator_ids: None,
            expires_at_unix: None,
        });

        let decision = validate_registry(&registry, Some(events.as_slice()), 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "audit_suspend_after_revoke"));
    }

    #[test]
    fn validate_registry_rejects_invalid_profile_status() {
        let mut registry = baseline_registry();
        registry.profile_status = "unknown".to_string();

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "profile_status_invalid"));
    }

    #[test]
    fn validate_registry_rejects_empty_operators_list() {
        let mut registry = baseline_registry();
        registry.operators.clear();

        let decision = validate_registry(&registry, None, 1_700_100_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "operators_empty"));
    }

    #[test]
    fn parse_args_accepts_required_flags() {
        let args = vec!["--registry".to_string(), "registry.json".to_string()];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.registry_path, PathBuf::from("registry.json"));
        assert!(parsed.events_path.is_none());
        assert!(parsed.now_unix_override.is_none());
    }

    #[test]
    fn parse_args_accepts_optional_flags() {
        let args = vec![
            "--registry".to_string(),
            "registry.json".to_string(),
            "--events".to_string(),
            "events.json".to_string(),
            "--now-unix".to_string(),
            "1700100000".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.registry_path, PathBuf::from("registry.json"));
        assert_eq!(parsed.events_path, Some(PathBuf::from("events.json")));
        assert_eq!(parsed.now_unix_override, Some(1_700_100_000));
    }

    #[test]
    fn parse_args_rejects_missing_registry() {
        let args = vec!["--events".to_string(), "events.json".to_string()];

        let error = parse_args(&args).expect_err("expected parse failure");
        assert_eq!(error, "missing required --registry");
    }
}
