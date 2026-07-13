use bitcoin::secp256k1::{
    schnorr::Signature as SchnorrSignature, Message as SecpMessage, Secp256k1, XOnlyPublicKey,
};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::env;
use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

const SECONDS_PER_DAY: u64 = 86_400;
const DEFAULT_DAO_OVERRIDE_MAX_TTL_SECONDS: u64 = 7 * SECONDS_PER_DAY;

#[derive(Clone, Debug, Deserialize)]
struct AdmissionPolicyV1 {
    #[serde(default)]
    max_operators_per_provider: Option<usize>,
    #[serde(default)]
    max_operators_per_region: Option<usize>,
    allowed_custody_classes: Vec<String>,
    required_attestation_status: String,
    min_patch_sla_days_remaining: u64,
    require_incident_response_contact: bool,
    #[serde(default)]
    dao_override_trust_root_pubkey_hex: Option<String>,
    #[serde(default)]
    dao_override_max_ttl_seconds: Option<u64>,
}

#[derive(Clone, Debug, Deserialize)]
struct AdmissionCandidate {
    operator_id: String,
    provider: String,
    region: String,
    custody_class: String,
    attestation_status: String,
    patch_sla_expires_at_unix: u64,
    #[serde(default)]
    incident_response_contact: Option<String>,
}

#[derive(Clone, Debug, Deserialize)]
struct ExistingOperator {
    operator_id: String,
    provider: String,
    region: String,
}

#[derive(Clone, Debug, Serialize)]
struct AdmissionReason {
    code: String,
    detail: String,
}

#[derive(Clone, Debug, Deserialize)]
struct AdmissionOverrideArtifact {
    payload_json: String,
    signature_hex: String,
}

#[derive(Clone, Debug, Deserialize)]
struct AdmissionOverridePayload {
    override_id: String,
    operator_id: String,
    decision: String,
    reason: String,
    approved_by: String,
    approved_at_unix: u64,
    expires_at_unix: u64,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct ConsumedOverrideRecord {
    override_id: String,
    operator_id: String,
    approved_by: String,
    approved_at_unix: u64,
    expires_at_unix: u64,
    consumed_at_unix: u64,
}

#[derive(Clone, Debug, Default, Deserialize, Serialize)]
struct OverrideReplayRegistry {
    #[serde(default)]
    consumed_override_ids: HashMap<String, ConsumedOverrideRecord>,
}

impl OverrideReplayRegistry {
    // Remove expired entries to bound registry growth.
    //
    // Safety: pruning expired entries does not create a replay window because
    // apply_dao_override independently rejects expired overrides via the
    // expires_at_unix < now_unix_seconds temporal guard. If validation ordering
    // changes, replay protection invariants must be re-evaluated.
    fn prune_expired(&mut self, now_unix_seconds: u64) {
        self.consumed_override_ids
            .retain(|_, record| record.expires_at_unix >= now_unix_seconds);
    }

    fn lookup(&self, override_id: &str) -> Option<&ConsumedOverrideRecord> {
        self.consumed_override_ids.get(override_id)
    }

    fn insert(
        &mut self,
        override_id: String,
        operator_id: String,
        approved_by: String,
        approved_at_unix: u64,
        expires_at_unix: u64,
        consumed_at_unix: u64,
    ) {
        self.consumed_override_ids.insert(
            override_id.clone(),
            ConsumedOverrideRecord {
                override_id,
                operator_id,
                approved_by,
                approved_at_unix,
                expires_at_unix,
                consumed_at_unix,
            },
        );
    }
}

#[derive(Clone, Debug, Serialize)]
struct AdmissionDecision {
    decision: String,
    reasons: Vec<AdmissionReason>,
    #[serde(default)]
    override_applied: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    override_reference: Option<String>,
    evaluated_at_unix: u64,
}

#[derive(Debug)]
struct CliArgs {
    policy_path: PathBuf,
    candidate_path: PathBuf,
    existing_path: Option<PathBuf>,
    override_path: Option<PathBuf>,
    override_registry_path: Option<PathBuf>,
    now_unix_override: Option<u64>,
}

fn usage() -> String {
    "Usage: admission_checker --policy <path> --candidate <path> [--existing <path>] [--override <path>] [--override-registry <path>] [--now-unix <seconds>]".to_string()
}

fn parse_args(args: &[String]) -> Result<CliArgs, String> {
    let mut policy_path: Option<PathBuf> = None;
    let mut candidate_path: Option<PathBuf> = None;
    let mut existing_path: Option<PathBuf> = None;
    let mut override_path: Option<PathBuf> = None;
    let mut override_registry_path: Option<PathBuf> = None;
    let mut now_unix_override: Option<u64> = None;

    let mut i = 0usize;
    while i < args.len() {
        match args[i].as_str() {
            "--policy" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --policy".to_string());
                }
                policy_path = Some(PathBuf::from(&args[i]));
            }
            "--candidate" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --candidate".to_string());
                }
                candidate_path = Some(PathBuf::from(&args[i]));
            }
            "--existing" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --existing".to_string());
                }
                existing_path = Some(PathBuf::from(&args[i]));
            }
            "--override" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --override".to_string());
                }
                override_path = Some(PathBuf::from(&args[i]));
            }
            "--override-registry" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --override-registry".to_string());
                }
                override_registry_path = Some(PathBuf::from(&args[i]));
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

    let policy_path = policy_path.ok_or_else(|| "missing required --policy".to_string())?;
    let candidate_path =
        candidate_path.ok_or_else(|| "missing required --candidate".to_string())?;
    if override_path.is_some() && override_registry_path.is_none() {
        return Err("--override requires --override-registry for replay protection".to_string());
    }

    Ok(CliArgs {
        policy_path,
        candidate_path,
        existing_path,
        override_path,
        override_registry_path,
        now_unix_override,
    })
}

fn now_unix() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or(0)
}

fn load_json_file<T: for<'de> Deserialize<'de>>(path: &PathBuf) -> Result<T, String> {
    let bytes =
        fs::read(path).map_err(|e| format!("failed to read file [{}]: {e}", path.display()))?;
    serde_json::from_slice(&bytes)
        .map_err(|e| format!("failed to parse JSON file [{}]: {e}", path.display()))
}

fn load_override_replay_registry(path: &PathBuf) -> Result<OverrideReplayRegistry, String> {
    if !path.exists() {
        return Ok(OverrideReplayRegistry::default());
    }
    load_json_file(path)
}

// Acquires an exclusive, non-blocking inter-process lock for the override
// replay registry. Held across load -> validate -> insert -> persist so two
// concurrent checker invocations cannot both consume the same one-time
// override marker. Mirrors the signer state lock (engine::state).
fn acquire_override_registry_lock(registry_path: &Path) -> Result<fs::File, String> {
    let lock_path = registry_path.with_extension("lock");
    let lock_file = fs::OpenOptions::new()
        .create(true)
        .truncate(false)
        .read(true)
        .write(true)
        .open(&lock_path)
        .map_err(|error| {
            format!(
                "failed to open override replay registry lock file [{}]: {error}",
                lock_path.display()
            )
        })?;

    #[cfg(unix)]
    {
        use libc::{flock, EAGAIN, EWOULDBLOCK, LOCK_EX, LOCK_NB};
        use std::os::fd::AsRawFd;

        let rc = unsafe { flock(lock_file.as_raw_fd(), LOCK_EX | LOCK_NB) };
        if rc != 0 {
            let lock_error = std::io::Error::last_os_error();
            if lock_error
                .raw_os_error()
                .is_some_and(|errno| errno == EWOULDBLOCK || errno == EAGAIN)
            {
                return Err(format!(
                    "override replay registry lock already held by another process [{}]",
                    lock_path.display()
                ));
            }

            return Err(format!(
                "failed to lock override replay registry [{}]: {lock_error}",
                lock_path.display()
            ));
        }
    }

    Ok(lock_file)
}

fn persist_override_replay_registry(
    path: &PathBuf,
    registry: &OverrideReplayRegistry,
) -> Result<(), String> {
    let serialized = serde_json::to_vec_pretty(registry)
        .map_err(|error| format!("failed to serialize override replay registry: {error}"))?;
    let tmp_path = path.with_extension(format!("tmp-{}", std::process::id()));

    // Write + fsync the temp file, then atomically rename and fsync the
    // parent directory so a consumed-override marker survives power loss.
    // Mirrors the signer state persistence path (engine::persistence).
    {
        let mut tmp_file = fs::OpenOptions::new()
            .create(true)
            .truncate(true)
            .write(true)
            .open(&tmp_path)
            .map_err(|error| {
                format!(
                    "failed to open override replay registry temp file [{}]: {error}",
                    tmp_path.display()
                )
            })?;
        tmp_file.write_all(&serialized).map_err(|error| {
            let _ = fs::remove_file(&tmp_path);
            format!(
                "failed to write override replay registry temp file [{}]: {error}",
                tmp_path.display()
            )
        })?;
        tmp_file.sync_all().map_err(|error| {
            let _ = fs::remove_file(&tmp_path);
            format!(
                "failed to sync override replay registry temp file [{}]: {error}",
                tmp_path.display()
            )
        })?;
    }

    fs::rename(&tmp_path, path).map_err(|error| {
        let _ = fs::remove_file(&tmp_path);
        format!(
            "failed to persist override replay registry [{}]: {error}",
            path.display()
        )
    })?;

    let directory_path = path
        .parent()
        .filter(|parent| !parent.as_os_str().is_empty())
        .map_or_else(|| PathBuf::from("."), |parent| parent.to_path_buf());
    let directory = fs::File::open(&directory_path).map_err(|error| {
        format!(
            "failed to open override replay registry directory [{}] for sync: {error}",
            directory_path.display()
        )
    })?;
    directory.sync_all().map_err(|error| {
        format!(
            "failed to sync override replay registry directory [{}]: {error}",
            directory_path.display()
        )
    })
}

fn trimmed_lowercase(value: &str) -> String {
    value.trim().to_ascii_lowercase()
}

fn push_override_rejection_reason(decision: &mut AdmissionDecision, code: &str, detail: String) {
    decision.decision = "reject".to_string();
    decision.reasons.push(AdmissionReason {
        code: code.to_string(),
        detail,
    });
}

fn parse_override_trust_root_pubkey(trust_root_hex: &str) -> Result<XOnlyPublicKey, String> {
    let trust_root_hex = trust_root_hex.trim();
    if trust_root_hex.is_empty() {
        return Err("dao override trust root pubkey must be non-empty hex".to_string());
    }

    let trust_root_bytes = hex::decode(trust_root_hex)
        .map_err(|_| "dao override trust root pubkey must be valid hex".to_string())?;
    if trust_root_bytes.len() != 32 {
        return Err("dao override trust root pubkey must decode to 32 bytes".to_string());
    }

    XOnlyPublicKey::from_slice(&trust_root_bytes).map_err(|_| {
        "dao override trust root pubkey must be valid x-only secp256k1 key".to_string()
    })
}

fn verify_override_signature(
    payload_json: &str,
    signature_hex: &str,
    trust_root_pubkey: &XOnlyPublicKey,
) -> Result<(), String> {
    let signature_bytes = hex::decode(signature_hex.trim())
        .map_err(|_| "dao override signature must be valid hex".to_string())?;
    let signature = SchnorrSignature::from_slice(&signature_bytes)
        .map_err(|_| "dao override signature must be valid schnorr bytes".to_string())?;
    let payload_digest = Sha256::digest(payload_json.as_bytes());
    let message = SecpMessage::from_digest_slice(&payload_digest)
        .map_err(|_| "failed to construct override signature digest".to_string())?;

    Secp256k1::verification_only()
        .verify_schnorr(&signature, &message, trust_root_pubkey)
        .map_err(|_| "dao override signature verification failed".to_string())
}

fn apply_dao_override(
    policy: &AdmissionPolicyV1,
    candidate: &AdmissionCandidate,
    now_unix_seconds: u64,
    mut decision: AdmissionDecision,
    override_artifact: Option<&AdmissionOverrideArtifact>,
    replay_registry: Option<&mut OverrideReplayRegistry>,
) -> AdmissionDecision {
    if decision.decision == "allow" {
        return decision;
    }

    let Some(override_artifact) = override_artifact else {
        return decision;
    };

    let trust_root_hex = match policy.dao_override_trust_root_pubkey_hex.as_ref() {
        Some(trust_root_hex) if !trust_root_hex.trim().is_empty() => trust_root_hex,
        _ => {
            push_override_rejection_reason(
                &mut decision,
                "dao_override_policy_not_configured",
                "policy must define dao_override_trust_root_pubkey_hex to apply overrides"
                    .to_string(),
            );
            return decision;
        }
    };
    let trust_root_pubkey = match parse_override_trust_root_pubkey(trust_root_hex) {
        Ok(trust_root_pubkey) => trust_root_pubkey,
        Err(detail) => {
            push_override_rejection_reason(
                &mut decision,
                "dao_override_invalid_trust_root",
                detail,
            );
            return decision;
        }
    };

    let payload_json = override_artifact.payload_json.trim();
    if payload_json.is_empty() {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_payload_invalid",
            "dao override payload_json must be non-empty".to_string(),
        );
        return decision;
    }

    if let Err(detail) = verify_override_signature(
        payload_json,
        &override_artifact.signature_hex,
        &trust_root_pubkey,
    ) {
        push_override_rejection_reason(&mut decision, "dao_override_invalid_signature", detail);
        return decision;
    }

    let override_payload = match serde_json::from_str::<AdmissionOverridePayload>(payload_json) {
        Ok(override_payload) => override_payload,
        Err(error) => {
            push_override_rejection_reason(
                &mut decision,
                "dao_override_payload_invalid",
                format!("failed to parse dao override payload_json: {error}"),
            );
            return decision;
        }
    };

    let override_id = trimmed_lowercase(&override_payload.override_id);
    if override_id.is_empty() {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_id_missing",
            "override override_id must be non-empty".to_string(),
        );
        return decision;
    }

    let Some(replay_registry) = replay_registry else {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_replay_registry_not_configured",
            "override replay protection requires --override-registry <path>".to_string(),
        );
        return decision;
    };
    replay_registry.prune_expired(now_unix_seconds);
    if let Some(record) = replay_registry.lookup(&override_id) {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_replay_detected",
            format!(
                "override_id [{}] already consumed at [{}] for operator_id [{}]",
                record.override_id, record.consumed_at_unix, record.operator_id
            ),
        );
        return decision;
    }

    let override_operator_id = trimmed_lowercase(&override_payload.operator_id);
    let candidate_operator_id = trimmed_lowercase(&candidate.operator_id);
    if override_operator_id.is_empty() || override_operator_id != candidate_operator_id {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_candidate_mismatch",
            format!(
                "override operator_id [{}] does not match candidate operator_id [{}]",
                override_payload.operator_id, candidate.operator_id
            ),
        );
        return decision;
    }

    if trimmed_lowercase(&override_payload.decision) != "allow" {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_decision_not_allow",
            format!(
                "override decision must be [allow], got [{}]",
                override_payload.decision
            ),
        );
        return decision;
    }

    if override_payload.reason.trim().is_empty() {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_reason_missing",
            "override reason must be non-empty".to_string(),
        );
        return decision;
    }

    if override_payload.approved_by.trim().is_empty() {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_approver_missing",
            "override approved_by must be non-empty".to_string(),
        );
        return decision;
    }

    if override_payload.approved_at_unix > now_unix_seconds {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_not_yet_valid",
            format!(
                "override approved_at_unix [{}] is in the future relative to now [{}]",
                override_payload.approved_at_unix, now_unix_seconds
            ),
        );
        return decision;
    }

    if override_payload.expires_at_unix < now_unix_seconds {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_expired",
            format!(
                "override expired at [{}], now [{}]",
                override_payload.expires_at_unix, now_unix_seconds
            ),
        );
        return decision;
    }

    if override_payload.expires_at_unix < override_payload.approved_at_unix {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_expiry_invalid",
            format!(
                "override expires_at_unix [{}] is before approved_at_unix [{}]",
                override_payload.expires_at_unix, override_payload.approved_at_unix
            ),
        );
        return decision;
    }

    let max_ttl_seconds = policy
        .dao_override_max_ttl_seconds
        .unwrap_or(DEFAULT_DAO_OVERRIDE_MAX_TTL_SECONDS);
    let override_ttl_seconds = override_payload.expires_at_unix - override_payload.approved_at_unix;
    if override_ttl_seconds > max_ttl_seconds {
        push_override_rejection_reason(
            &mut decision,
            "dao_override_ttl_exceeds_policy",
            format!(
                "override TTL [{}] exceeds policy max [{}]",
                override_ttl_seconds, max_ttl_seconds
            ),
        );
        return decision;
    }

    replay_registry.insert(
        override_id,
        candidate_operator_id.clone(),
        override_payload.approved_by.trim().to_string(),
        override_payload.approved_at_unix,
        override_payload.expires_at_unix,
        now_unix_seconds,
    );

    decision.decision = "allow".to_string();
    decision.override_applied = true;
    decision.override_reference = Some(format!(
        "{}:{}",
        override_payload.approved_by.trim(),
        override_payload.approved_at_unix
    ));
    decision.reasons.push(AdmissionReason {
        code: "dao_override_applied".to_string(),
        detail: format!(
            "governance override applied by [{}] for operator_id [{}]: {}",
            override_payload.approved_by.trim(),
            override_payload.operator_id.trim(),
            override_payload.reason.trim()
        ),
    });
    decision
}

fn evaluate_admission(
    policy: &AdmissionPolicyV1,
    candidate: &AdmissionCandidate,
    existing: &[ExistingOperator],
    now_unix_seconds: u64,
) -> AdmissionDecision {
    let mut reasons: Vec<AdmissionReason> = Vec::new();

    let candidate_operator_id = trimmed_lowercase(&candidate.operator_id);
    if candidate_operator_id.is_empty() {
        reasons.push(AdmissionReason {
            code: "operator_id_missing".to_string(),
            detail: "candidate operator_id must be non-empty".to_string(),
        });
    } else if existing
        .iter()
        .any(|operator| trimmed_lowercase(&operator.operator_id) == candidate_operator_id)
    {
        reasons.push(AdmissionReason {
            code: "operator_id_already_registered".to_string(),
            detail: format!(
                "operator_id [{}] already exists in operator set",
                candidate_operator_id
            ),
        });
    }

    let candidate_provider = trimmed_lowercase(&candidate.provider);
    if candidate_provider.is_empty() {
        reasons.push(AdmissionReason {
            code: "provider_missing".to_string(),
            detail: "candidate provider must be non-empty".to_string(),
        });
    }

    let candidate_region = trimmed_lowercase(&candidate.region);
    if candidate_region.is_empty() {
        reasons.push(AdmissionReason {
            code: "region_missing".to_string(),
            detail: "candidate region must be non-empty".to_string(),
        });
    }

    if let Some(max_per_provider) = policy.max_operators_per_provider {
        let mut provider_counts = HashMap::new();
        for operator in existing {
            let provider = trimmed_lowercase(&operator.provider);
            if provider.is_empty() {
                continue;
            }
            *provider_counts
                .entry(provider.to_string())
                .or_insert(0usize) += 1;
        }
        let current_count = provider_counts
            .get(&candidate_provider)
            .copied()
            .unwrap_or_default();
        if current_count.saturating_add(1) > max_per_provider {
            reasons.push(AdmissionReason {
                code: "provider_diversity_violation".to_string(),
                detail: format!(
                    "provider [{}] would exceed max_operators_per_provider [{}]",
                    candidate_provider, max_per_provider
                ),
            });
        }
    }

    if let Some(max_per_region) = policy.max_operators_per_region {
        let mut region_counts = HashMap::new();
        for operator in existing {
            let region = trimmed_lowercase(&operator.region);
            if region.is_empty() {
                continue;
            }
            *region_counts.entry(region.to_string()).or_insert(0usize) += 1;
        }
        let current_count = region_counts
            .get(&candidate_region)
            .copied()
            .unwrap_or_default();
        if current_count.saturating_add(1) > max_per_region {
            reasons.push(AdmissionReason {
                code: "geo_diversity_violation".to_string(),
                detail: format!(
                    "region [{}] would exceed max_operators_per_region [{}]",
                    candidate_region, max_per_region
                ),
            });
        }
    }

    let allowed_custody_classes = policy
        .allowed_custody_classes
        .iter()
        .map(|value| trimmed_lowercase(value))
        .collect::<Vec<_>>();
    let candidate_custody_class = trimmed_lowercase(&candidate.custody_class);
    if candidate_custody_class.is_empty() {
        reasons.push(AdmissionReason {
            code: "custody_class_missing".to_string(),
            detail: "candidate custody_class must be non-empty".to_string(),
        });
    } else if !allowed_custody_classes
        .iter()
        .any(|allowed| allowed == &candidate_custody_class)
    {
        reasons.push(AdmissionReason {
            code: "custody_class_not_allowed".to_string(),
            detail: format!(
                "custody_class [{}] not in allowed set {:?}",
                candidate_custody_class, policy.allowed_custody_classes
            ),
        });
    }

    let required_attestation_status = trimmed_lowercase(&policy.required_attestation_status);
    let candidate_attestation_status = trimmed_lowercase(&candidate.attestation_status);
    if required_attestation_status.is_empty() {
        reasons.push(AdmissionReason {
            code: "required_attestation_status_missing".to_string(),
            detail: "policy required_attestation_status must be non-empty".to_string(),
        });
    }
    if candidate_attestation_status.is_empty() {
        reasons.push(AdmissionReason {
            code: "attestation_status_missing".to_string(),
            detail: "candidate attestation_status must be non-empty".to_string(),
        });
    } else if !required_attestation_status.is_empty()
        && candidate_attestation_status != required_attestation_status
    {
        reasons.push(AdmissionReason {
            code: "attestation_status_not_approved".to_string(),
            detail: format!(
                "candidate attestation_status [{}] does not match required [{}]",
                candidate.attestation_status, policy.required_attestation_status
            ),
        });
    }

    let required_remaining_seconds = policy
        .min_patch_sla_days_remaining
        .saturating_mul(SECONDS_PER_DAY);
    let minimum_expiry = now_unix_seconds.saturating_add(required_remaining_seconds);
    if candidate.patch_sla_expires_at_unix < minimum_expiry {
        reasons.push(AdmissionReason {
            code: "patch_sla_below_minimum_remaining".to_string(),
            detail: format!(
                "patch_sla_expires_at_unix [{}] is below minimum required [{}] ({} days remaining)",
                candidate.patch_sla_expires_at_unix,
                minimum_expiry,
                policy.min_patch_sla_days_remaining
            ),
        });
    }

    if policy.require_incident_response_contact {
        let has_contact = candidate
            .incident_response_contact
            .as_ref()
            .is_some_and(|value| !value.trim().is_empty());
        if !has_contact {
            reasons.push(AdmissionReason {
                code: "incident_response_contact_missing".to_string(),
                detail: "candidate incident_response_contact is required".to_string(),
            });
        }
    }

    AdmissionDecision {
        decision: if reasons.is_empty() {
            "allow".to_string()
        } else {
            "reject".to_string()
        },
        reasons,
        override_applied: false,
        override_reference: None,
        evaluated_at_unix: now_unix_seconds,
    }
}

fn run() -> Result<AdmissionDecision, String> {
    let args = env::args().skip(1).collect::<Vec<_>>();
    let cli = parse_args(&args)?;
    let policy: AdmissionPolicyV1 = load_json_file(&cli.policy_path)?;
    let candidate: AdmissionCandidate = load_json_file(&cli.candidate_path)?;
    let existing: Vec<ExistingOperator> = match cli.existing_path.as_ref() {
        Some(path) => load_json_file(path)?,
        None => Vec::new(),
    };
    let now_unix_seconds = cli.now_unix_override.unwrap_or_else(now_unix);
    let override_artifact: Option<AdmissionOverrideArtifact> = match cli.override_path.as_ref() {
        Some(path) => Some(load_json_file(path)?),
        None => None,
    };
    // Hold the exclusive registry lock across the whole load -> apply ->
    // persist critical section so concurrent invocations cannot both accept
    // and consume the same one-time override marker. Bound for the lifetime
    // of `run`; released on drop after persistence completes.
    let _registry_lock = match cli.override_registry_path.as_ref() {
        Some(path) => Some(acquire_override_registry_lock(path)?),
        None => None,
    };
    let mut replay_registry: Option<OverrideReplayRegistry> =
        match cli.override_registry_path.as_ref() {
            Some(path) => Some(load_override_replay_registry(path)?),
            None => None,
        };
    let decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
    let decision = apply_dao_override(
        &policy,
        &candidate,
        now_unix_seconds,
        decision,
        override_artifact.as_ref(),
        replay_registry.as_mut(),
    );

    if decision.override_applied {
        let registry_path = cli.override_registry_path.as_ref().ok_or_else(|| {
            "override replay registry path is required when applying override".to_string()
        })?;
        let registry = replay_registry.as_ref().ok_or_else(|| {
            "override replay registry missing while applying override".to_string()
        })?;
        persist_override_replay_registry(registry_path, registry)?;
    }

    Ok(decision)
}

fn main() {
    match run() {
        Ok(decision) => {
            let json = serde_json::to_string_pretty(&decision)
                .unwrap_or_else(|_| "{\"decision\":\"reject\",\"reasons\":[{\"code\":\"serialization_error\",\"detail\":\"failed to encode output\"}],\"evaluated_at_unix\":0}".to_string());
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

    fn baseline_policy() -> AdmissionPolicyV1 {
        AdmissionPolicyV1 {
            max_operators_per_provider: Some(2),
            max_operators_per_region: Some(2),
            allowed_custody_classes: vec!["hsm".to_string(), "kms".to_string()],
            required_attestation_status: "approved".to_string(),
            min_patch_sla_days_remaining: 7,
            require_incident_response_contact: true,
            dao_override_trust_root_pubkey_hex: None,
            dao_override_max_ttl_seconds: None,
        }
    }

    fn baseline_candidate() -> AdmissionCandidate {
        AdmissionCandidate {
            operator_id: "operator-3".to_string(),
            provider: "gcp".to_string(),
            region: "us-central1".to_string(),
            custody_class: "kms".to_string(),
            attestation_status: "approved".to_string(),
            patch_sla_expires_at_unix: 2_000_000_000,
            incident_response_contact: Some("ops@example.org".to_string()),
        }
    }

    fn baseline_existing() -> Vec<ExistingOperator> {
        vec![
            ExistingOperator {
                operator_id: "operator-1".to_string(),
                provider: "aws".to_string(),
                region: "us-east-1".to_string(),
            },
            ExistingOperator {
                operator_id: "operator-2".to_string(),
                provider: "gcp".to_string(),
                region: "europe-west1".to_string(),
            },
        ]
    }

    fn sign_override_payload(payload_json: String) -> (String, AdmissionOverrideArtifact) {
        let secp = Secp256k1::new();
        let secret_key =
            bitcoin::secp256k1::SecretKey::from_slice(&[0x33; 32]).expect("secret key");
        let keypair = bitcoin::secp256k1::Keypair::from_secret_key(&secp, &secret_key);
        let (trust_root_pubkey, _) = XOnlyPublicKey::from_keypair(&keypair);

        let payload_digest = Sha256::digest(payload_json.as_bytes());
        let message = SecpMessage::from_digest_slice(&payload_digest).expect("message digest");
        let signature = secp.sign_schnorr_no_aux_rand(&message, &keypair);
        let artifact = AdmissionOverrideArtifact {
            payload_json,
            signature_hex: signature.to_string(),
        };
        (trust_root_pubkey.to_string(), artifact)
    }

    fn build_signed_override_artifact(
        operator_id: &str,
        decision: &str,
        approved_at_unix: u64,
        expires_at_unix: u64,
    ) -> (String, AdmissionOverrideArtifact) {
        let payload_json = serde_json::json!({
            "override_id": format!(
                "override:{}:{}:{}",
                trimmed_lowercase(operator_id),
                approved_at_unix,
                expires_at_unix
            ),
            "operator_id": operator_id,
            "decision": decision,
            "reason": "manual governance approval",
            "approved_by": "dao-multisig-1",
            "approved_at_unix": approved_at_unix,
            "expires_at_unix": expires_at_unix,
        })
        .to_string();
        sign_override_payload(payload_json)
    }

    #[test]
    fn evaluate_admission_allows_compliant_candidate() {
        let policy = baseline_policy();
        let candidate = baseline_candidate();
        let existing = baseline_existing();

        let decision = evaluate_admission(&policy, &candidate, &existing, 1_700_000_000);
        assert_eq!(decision.decision, "allow");
        assert!(decision.reasons.is_empty());
    }

    #[test]
    fn evaluate_admission_rejects_provider_diversity_violation() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();

        let decision = evaluate_admission(&policy, &candidate, &existing, 1_700_000_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "provider_diversity_violation"));
    }

    #[test]
    fn evaluate_admission_rejects_provider_diversity_violation_case_insensitive() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "AWS".to_string();
        let existing = baseline_existing();

        let decision = evaluate_admission(&policy, &candidate, &existing, 1_700_000_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "provider_diversity_violation"));
    }

    #[test]
    fn evaluate_admission_rejects_region_diversity_violation_case_insensitive() {
        let mut policy = baseline_policy();
        policy.max_operators_per_region = Some(1);
        let mut candidate = baseline_candidate();
        candidate.region = "US-EAST-1".to_string();
        let mut existing = baseline_existing();
        existing.push(ExistingOperator {
            operator_id: "operator-99".to_string(),
            provider: "azure".to_string(),
            region: "us-east-1".to_string(),
        });

        let decision = evaluate_admission(&policy, &candidate, &existing, 1_700_000_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "geo_diversity_violation"));
    }

    #[test]
    fn evaluate_admission_rejects_duplicate_operator_id_case_insensitive() {
        let policy = baseline_policy();
        let mut candidate = baseline_candidate();
        candidate.operator_id = "Operator-1".to_string();
        let existing = baseline_existing();

        let decision = evaluate_admission(&policy, &candidate, &existing, 1_700_000_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "operator_id_already_registered"));
    }

    #[test]
    fn evaluate_admission_rejects_missing_contact_and_bad_attestation() {
        let policy = baseline_policy();
        let mut candidate = baseline_candidate();
        candidate.incident_response_contact = None;
        candidate.attestation_status = "pending".to_string();
        let existing = baseline_existing();

        let decision = evaluate_admission(&policy, &candidate, &existing, 1_700_000_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "incident_response_contact_missing"));
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "attestation_status_not_approved"));
    }

    #[test]
    fn evaluate_admission_rejects_empty_required_and_candidate_attestation_statuses() {
        let mut policy = baseline_policy();
        policy.required_attestation_status = "  \t".to_string();
        let mut candidate = baseline_candidate();
        candidate.attestation_status = " \n ".to_string();

        let decision = evaluate_admission(&policy, &candidate, &baseline_existing(), 1_700_000_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "required_attestation_status_missing"));
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "attestation_status_missing"));
    }

    #[test]
    fn evaluate_admission_reports_empty_attestation_fields_before_mismatch() {
        let mut empty_candidate = baseline_candidate();
        empty_candidate.attestation_status = "   ".to_string();
        let candidate_decision = evaluate_admission(
            &baseline_policy(),
            &empty_candidate,
            &baseline_existing(),
            1_700_000_000,
        );
        assert!(candidate_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "attestation_status_missing"));
        assert!(!candidate_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "attestation_status_not_approved"));

        let mut empty_policy = baseline_policy();
        empty_policy.required_attestation_status = "\t".to_string();
        let policy_decision = evaluate_admission(
            &empty_policy,
            &baseline_candidate(),
            &baseline_existing(),
            1_700_000_000,
        );
        assert!(policy_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "required_attestation_status_missing"));
        assert!(!policy_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "attestation_status_not_approved"));
    }

    #[test]
    fn evaluate_admission_normalizes_non_empty_attestation_statuses() {
        let mut policy = baseline_policy();
        policy.required_attestation_status = " Approved ".to_string();
        let mut candidate = baseline_candidate();
        candidate.attestation_status = "APPROVED".to_string();

        let decision = evaluate_admission(&policy, &candidate, &baseline_existing(), 1_700_000_000);
        assert_eq!(decision.decision, "allow");
        assert!(decision.reasons.is_empty());
    }

    #[test]
    fn evaluate_admission_rejects_expired_patch_sla() {
        let policy = baseline_policy();
        let mut candidate = baseline_candidate();
        candidate.patch_sla_expires_at_unix = 1_700_000_000;
        let existing = baseline_existing();

        let decision = evaluate_admission(&policy, &candidate, &existing, 1_700_000_000);
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "patch_sla_below_minimum_remaining"));
    }

    #[test]
    fn apply_dao_override_allows_rejected_candidate_when_signature_is_valid() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_unix_seconds.saturating_sub(60),
            now_unix_seconds.saturating_add(600),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);
        policy.dao_override_max_ttl_seconds = Some(3600);

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        assert_eq!(base_decision.decision, "reject");
        assert!(base_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "provider_diversity_violation"));
        let mut replay_registry = OverrideReplayRegistry::default();

        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(override_decision.decision, "allow");
        assert!(override_decision.override_applied);
        assert!(override_decision.override_reference.is_some());
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_applied"));
    }

    #[test]
    fn apply_dao_override_rejects_invalid_signature() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, mut override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_unix_seconds.saturating_sub(60),
            now_unix_seconds.saturating_add(600),
        );
        override_artifact.signature_hex = "00".repeat(64);
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let mut replay_registry = OverrideReplayRegistry::default();
        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(override_decision.decision, "reject");
        assert!(!override_decision.override_applied);
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_invalid_signature"));
    }

    #[test]
    fn apply_dao_override_rejects_candidate_mismatch() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, override_artifact) = build_signed_override_artifact(
            "different-operator",
            "allow",
            now_unix_seconds.saturating_sub(60),
            now_unix_seconds.saturating_add(600),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let mut replay_registry = OverrideReplayRegistry::default();
        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(override_decision.decision, "reject");
        assert!(!override_decision.override_applied);
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_candidate_mismatch"));
    }

    #[test]
    fn apply_dao_override_rejects_expired_artifact() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_unix_seconds.saturating_sub(3600),
            now_unix_seconds.saturating_sub(60),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let mut replay_registry = OverrideReplayRegistry::default();
        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(override_decision.decision, "reject");
        assert!(!override_decision.override_applied);
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_expired"));
    }

    #[test]
    fn apply_dao_override_rejects_ttl_exceeding_policy() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_unix_seconds.saturating_sub(60),
            now_unix_seconds.saturating_add(86_400 * 30),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);
        policy.dao_override_max_ttl_seconds = Some(3600);

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let mut replay_registry = OverrideReplayRegistry::default();
        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(override_decision.decision, "reject");
        assert!(!override_decision.override_applied);
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_ttl_exceeds_policy"));
    }

    #[test]
    fn apply_dao_override_rejects_not_yet_valid_artifact() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_unix_seconds.saturating_add(3600),
            now_unix_seconds.saturating_add(7200),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let mut replay_registry = OverrideReplayRegistry::default();
        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(override_decision.decision, "reject");
        assert!(!override_decision.override_applied);
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_not_yet_valid"));
    }

    #[test]
    fn apply_dao_override_rejects_when_policy_trust_root_not_configured() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (_, override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_unix_seconds.saturating_sub(60),
            now_unix_seconds.saturating_add(600),
        );

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let mut replay_registry = OverrideReplayRegistry::default();
        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(override_decision.decision, "reject");
        assert!(!override_decision.override_applied);
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_policy_not_configured"));
    }

    #[test]
    fn apply_dao_override_rejects_non_allow_decision() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "deny",
            now_unix_seconds.saturating_sub(60),
            now_unix_seconds.saturating_add(600),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let mut replay_registry = OverrideReplayRegistry::default();
        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(override_decision.decision, "reject");
        assert!(!override_decision.override_applied);
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_decision_not_allow"));
    }

    #[test]
    fn apply_dao_override_rejects_when_replay_registry_not_configured() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_unix_seconds.saturating_sub(60),
            now_unix_seconds.saturating_add(600),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            None,
        );
        assert_eq!(override_decision.decision, "reject");
        assert!(!override_decision.override_applied);
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_replay_registry_not_configured"));
    }

    #[test]
    fn apply_dao_override_rejects_replayed_override_id() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_unix_seconds.saturating_sub(60),
            now_unix_seconds.saturating_add(600),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);
        let mut replay_registry = OverrideReplayRegistry::default();

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let first_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(first_decision.decision, "allow");
        assert!(first_decision.override_applied);

        let second_base_decision =
            evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let second_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            second_base_decision,
            Some(&override_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(second_decision.decision, "reject");
        assert!(!second_decision.override_applied);
        assert!(second_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_replay_detected"));
    }

    #[test]
    fn apply_dao_override_rejects_missing_override_id() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();
        let now_unix_seconds = 1_700_000_000u64;

        let (trust_root_pubkey_hex, override_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_unix_seconds.saturating_sub(60),
            now_unix_seconds.saturating_add(600),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);

        let mut override_payload: serde_json::Value =
            serde_json::from_str(&override_artifact.payload_json).expect("override payload json");
        override_payload["override_id"] = serde_json::json!("");
        let (_, missing_id_artifact) = sign_override_payload(override_payload.to_string());

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_unix_seconds);
        let mut replay_registry = OverrideReplayRegistry::default();
        let override_decision = apply_dao_override(
            &policy,
            &candidate,
            now_unix_seconds,
            base_decision,
            Some(&missing_id_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(override_decision.decision, "reject");
        assert!(!override_decision.override_applied);
        assert!(override_decision
            .reasons
            .iter()
            .any(|reason| reason.code == "dao_override_id_missing"));
    }

    #[test]
    fn apply_dao_override_allows_new_override_after_previous_override_expires() {
        let mut policy = baseline_policy();
        policy.max_operators_per_provider = Some(1);
        let mut candidate = baseline_candidate();
        candidate.provider = "aws".to_string();
        let existing = baseline_existing();

        let now_first = 1_700_000_000u64;
        let (trust_root_pubkey_hex, first_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_first.saturating_sub(60),
            now_first.saturating_add(600),
        );
        policy.dao_override_trust_root_pubkey_hex = Some(trust_root_pubkey_hex);
        policy.dao_override_max_ttl_seconds = Some(86_400);
        let mut replay_registry = OverrideReplayRegistry::default();

        let base_decision = evaluate_admission(&policy, &candidate, &existing, now_first);
        let first_decision = apply_dao_override(
            &policy,
            &candidate,
            now_first,
            base_decision,
            Some(&first_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(first_decision.decision, "allow");
        assert!(first_decision.override_applied);

        let now_second = now_first.saturating_add(3_600);
        let (_, second_artifact) = build_signed_override_artifact(
            &candidate.operator_id,
            "allow",
            now_second.saturating_sub(60),
            now_second.saturating_add(600),
        );

        let second_base_decision = evaluate_admission(&policy, &candidate, &existing, now_second);
        let second_decision = apply_dao_override(
            &policy,
            &candidate,
            now_second,
            second_base_decision,
            Some(&second_artifact),
            Some(&mut replay_registry),
        );
        assert_eq!(second_decision.decision, "allow");
        assert!(second_decision.override_applied);
    }

    #[test]
    fn override_replay_registry_persists_and_reloads() {
        let tmp_dir = std::env::temp_dir().join(format!(
            "admission-override-registry-test-{}-{}",
            std::process::id(),
            now_unix()
        ));
        fs::create_dir_all(&tmp_dir).expect("create tmp dir");
        let registry_path = tmp_dir.join("override-registry.json");

        let mut registry = OverrideReplayRegistry::default();
        registry.insert(
            "test-override-id-1".to_string(),
            "operator-1".to_string(),
            "dao-approver-1".to_string(),
            1_700_000_000,
            1_700_003_600,
            1_700_000_100,
        );

        persist_override_replay_registry(&registry_path, &registry).expect("persist registry");
        let reloaded = load_override_replay_registry(&registry_path).expect("load registry");

        assert!(reloaded.lookup("test-override-id-1").is_some());
        assert!(reloaded.lookup("non-existent-override-id").is_none());
        let record = reloaded
            .lookup("test-override-id-1")
            .expect("reloaded override record");
        assert_eq!(record.operator_id, "operator-1");
        assert_eq!(record.consumed_at_unix, 1_700_000_100);

        let _ = fs::remove_dir_all(tmp_dir);
    }

    // The inter-process lock must be exclusive: a second acquisition while
    // the first is held fails, so two concurrent checker invocations cannot
    // both consume the same one-time override marker. flock locks are tied
    // to the open file description, so two separate opens contend even in
    // one process. Unix-only (the lock is a flock no-op elsewhere).
    #[cfg(unix)]
    #[test]
    fn override_registry_lock_is_exclusive_while_held() {
        let tmp_dir = std::env::temp_dir().join(format!(
            "admission-override-lock-test-{}-{}",
            std::process::id(),
            now_unix()
        ));
        fs::create_dir_all(&tmp_dir).expect("create tmp dir");
        let registry_path = tmp_dir.join("override-registry.json");

        let first = acquire_override_registry_lock(&registry_path).expect("first lock acquires");
        let second = acquire_override_registry_lock(&registry_path);
        assert!(
            second.is_err(),
            "second concurrent lock acquisition must fail while the first is held"
        );

        drop(first);
        let third = acquire_override_registry_lock(&registry_path);
        assert!(
            third.is_ok(),
            "lock must re-acquire after the holder releases"
        );
        drop(third);

        let _ = fs::remove_dir_all(tmp_dir);
    }

    #[test]
    fn parse_args_accepts_required_flags() {
        let args = vec![
            "--policy".to_string(),
            "policy.json".to_string(),
            "--candidate".to_string(),
            "candidate.json".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.policy_path, PathBuf::from("policy.json"));
        assert_eq!(parsed.candidate_path, PathBuf::from("candidate.json"));
        assert!(parsed.existing_path.is_none());
    }

    #[test]
    fn parse_args_accepts_override_flag() {
        let args = vec![
            "--policy".to_string(),
            "policy.json".to_string(),
            "--candidate".to_string(),
            "candidate.json".to_string(),
            "--override".to_string(),
            "override.json".to_string(),
            "--override-registry".to_string(),
            "override-registry.json".to_string(),
            "--now-unix".to_string(),
            "1700000000".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.override_path, Some(PathBuf::from("override.json")));
        assert_eq!(
            parsed.override_registry_path,
            Some(PathBuf::from("override-registry.json"))
        );
        assert_eq!(parsed.now_unix_override, Some(1_700_000_000));
    }

    #[test]
    fn parse_args_accepts_override_registry_flag() {
        let args = vec![
            "--policy".to_string(),
            "policy.json".to_string(),
            "--candidate".to_string(),
            "candidate.json".to_string(),
            "--override-registry".to_string(),
            "override-registry.json".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(
            parsed.override_registry_path,
            Some(PathBuf::from("override-registry.json"))
        );
    }

    #[test]
    fn parse_args_rejects_override_without_override_registry() {
        let args = vec![
            "--policy".to_string(),
            "policy.json".to_string(),
            "--candidate".to_string(),
            "candidate.json".to_string(),
            "--override".to_string(),
            "override.json".to_string(),
        ];

        let error = parse_args(&args).expect_err("expected parse failure");
        assert_eq!(
            error,
            "--override requires --override-registry for replay protection"
        );
    }
}
