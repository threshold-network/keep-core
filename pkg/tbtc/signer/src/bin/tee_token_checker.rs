use bitcoin::secp256k1::{
    schnorr::Signature as SchnorrSignature, Message as SecpMessage, Secp256k1, XOnlyPublicKey,
};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::{HashMap, HashSet};
use std::env;
use std::fs;
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

const SECONDS_PER_DAY: u64 = 86_400;
const MAX_VERIFIER_KEY_ROTATION_SECONDS: u64 = 30 * SECONDS_PER_DAY;

#[derive(Clone, Debug, Deserialize)]
struct TeeGovernanceRegistryV1 {
    profile_status: String,
    enforcement: TeeEnforcementParameters,
    operators: Vec<TeeOperatorAdmissionRecord>,
}

#[derive(Clone, Debug, Deserialize)]
struct TeeEnforcementParameters {
    attestation_max_age_seconds: u64,
    denylist_max_staleness_seconds: u64,
}

#[derive(Clone, Debug, Deserialize)]
struct TeeOperatorAdmissionRecord {
    operator_id: String,
    signer_identifier: String,
    status: String,
    allowed_tee_types: Vec<String>,
    allowed_measurements: Vec<String>,
    attestation_max_age_seconds: u64,
    effective_from: u64,
    #[serde(default)]
    effective_until: Option<u64>,
}

#[derive(Clone, Debug, Deserialize)]
struct VerifierKeySetV1 {
    keyset_version: u64,
    threshold_m: usize,
    max_key_age_seconds: u64,
    keys: Vec<VerifierKeyRecord>,
}

#[derive(Clone, Debug, Deserialize)]
struct VerifierKeyRecord {
    key_id: String,
    verifier_instance_id: String,
    trust_root_id: String,
    pubkey_hex: String,
    valid_from_unix: u64,
    valid_until_unix: u64,
    #[serde(default)]
    revoked_at_unix: Option<u64>,
}

#[derive(Clone, Debug, Deserialize)]
struct AdmissionTokenArtifact {
    payload_json: String,
    signatures: Vec<TokenSignature>,
}

#[derive(Clone, Debug, Deserialize)]
struct TokenSignature {
    verifier_key_id: String,
    signature_hex: String,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct AdmissionTokenPayload {
    token_id: String,
    operator_id: String,
    signer_identifier: String,
    tee_type: String,
    measurement_digest: String,
    issued_at_unix: u64,
    expires_at_unix: u64,
    registry_snapshot_version: u64,
    verifier_key_ids: Vec<String>,
    token_revocation_epoch: u64,
}

#[derive(Clone, Debug, Deserialize)]
struct TokenRevocationRegistryV1 {
    denylist_refreshed_at_unix: u64,
    #[serde(default)]
    min_token_revocation_epoch: u64,
    #[serde(default)]
    revoked_token_ids: HashMap<String, RevokedTokenRecord>,
    #[serde(default)]
    revoked_verifier_key_ids: HashMap<String, RevokedVerifierKeyRecord>,
}

#[derive(Clone, Debug, Deserialize)]
struct RevokedTokenRecord {
    revoked_at_unix: u64,
    #[serde(default)]
    reason: String,
}

#[derive(Clone, Debug, Deserialize)]
struct RevokedVerifierKeyRecord {
    revoked_at_unix: u64,
    #[serde(default)]
    reason: String,
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
    keyset_path: PathBuf,
    token_path: PathBuf,
    revocation_registry_path: PathBuf,
    now_unix_override: Option<u64>,
}

#[derive(Clone, Debug)]
struct ResolvedVerifierKey {
    verifier_instance_id: String,
    trust_root_id: String,
    pubkey: XOnlyPublicKey,
    valid_from_unix: u64,
    valid_until_unix: u64,
    revoked_at_unix: Option<u64>,
}

#[derive(Clone, Debug)]
struct ResolvedVerifierKeySet {
    threshold_m: usize,
    keys: HashMap<String, ResolvedVerifierKey>,
}

#[derive(Copy, Clone, Debug, PartialEq, Eq)]
enum OperatorStatus {
    Active,
    Suspended,
    Revoked,
}

fn usage() -> String {
    "Usage: tee_token_checker --registry <path> --keyset <path> --token <path> --revocation-registry <path> [--now-unix <seconds>]"
        .to_string()
}

fn parse_args(args: &[String]) -> Result<CliArgs, String> {
    let mut registry_path: Option<PathBuf> = None;
    let mut keyset_path: Option<PathBuf> = None;
    let mut token_path: Option<PathBuf> = None;
    let mut revocation_registry_path: Option<PathBuf> = None;
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
            "--keyset" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --keyset".to_string());
                }
                keyset_path = Some(PathBuf::from(&args[i]));
            }
            "--token" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --token".to_string());
                }
                token_path = Some(PathBuf::from(&args[i]));
            }
            "--revocation-registry" => {
                i += 1;
                if i >= args.len() {
                    return Err("missing value for --revocation-registry".to_string());
                }
                revocation_registry_path = Some(PathBuf::from(&args[i]));
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
    let keyset_path = keyset_path.ok_or_else(|| "missing required --keyset".to_string())?;
    let token_path = token_path.ok_or_else(|| "missing required --token".to_string())?;
    let revocation_registry_path = revocation_registry_path
        .ok_or_else(|| "missing required --revocation-registry".to_string())?;

    Ok(CliArgs {
        registry_path,
        keyset_path,
        token_path,
        revocation_registry_path,
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
    let bytes = fs::read(path)
        .map_err(|error| format!("failed to read file [{}]: {error}", path.display()))?;
    serde_json::from_slice(&bytes)
        .map_err(|error| format!("failed to parse JSON file [{}]: {error}", path.display()))
}

fn trimmed_lowercase(value: &str) -> String {
    value.trim().to_ascii_lowercase()
}

fn normalize_revocation_registry(
    mut registry: TokenRevocationRegistryV1,
) -> TokenRevocationRegistryV1 {
    registry.revoked_token_ids = registry
        .revoked_token_ids
        .into_iter()
        .map(|(key, value)| (trimmed_lowercase(&key), value))
        .collect();
    registry.revoked_verifier_key_ids = registry
        .revoked_verifier_key_ids
        .into_iter()
        .map(|(key, value)| (trimmed_lowercase(&key), value))
        .collect();
    registry
}

fn push_rejection_reason(reasons: &mut Vec<ValidationReason>, code: &str, detail: String) {
    reasons.push(ValidationReason {
        code: code.to_string(),
        detail,
    });
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

fn parse_xonly_pubkey_hex(pubkey_hex: &str) -> Result<XOnlyPublicKey, String> {
    let pubkey_hex = pubkey_hex.trim();
    if pubkey_hex.is_empty() {
        return Err("verifier pubkey hex must be non-empty".to_string());
    }

    let pubkey_bytes =
        hex::decode(pubkey_hex).map_err(|_| "verifier pubkey must be valid hex".to_string())?;
    if pubkey_bytes.len() != 32 {
        return Err("verifier pubkey must decode to 32 bytes".to_string());
    }

    XOnlyPublicKey::from_slice(&pubkey_bytes)
        .map_err(|_| "verifier pubkey must be valid x-only secp256k1 key".to_string())
}

fn verify_schnorr_signature(
    payload_json: &str,
    signature_hex: &str,
    pubkey: &XOnlyPublicKey,
) -> Result<(), String> {
    let signature_bytes = hex::decode(signature_hex.trim())
        .map_err(|_| "token signature must be valid hex".to_string())?;
    let signature = SchnorrSignature::from_slice(&signature_bytes)
        .map_err(|_| "token signature must be valid schnorr bytes".to_string())?;
    let payload_digest = Sha256::digest(payload_json.as_bytes());
    let message = SecpMessage::from_digest_slice(&payload_digest)
        .map_err(|_| "failed to construct token signature digest".to_string())?;

    Secp256k1::verification_only()
        .verify_schnorr(&signature, &message, pubkey)
        .map_err(|_| "token signature verification failed".to_string())
}

fn validate_verifier_keyset(
    keyset: &VerifierKeySetV1,
    now_unix_seconds: u64,
    reasons: &mut Vec<ValidationReason>,
) -> ResolvedVerifierKeySet {
    if keyset.keyset_version == 0 {
        push_rejection_reason(
            reasons,
            "keyset_version_invalid",
            "keyset_version must be > 0".to_string(),
        );
    }

    if keyset.threshold_m < 2 {
        push_rejection_reason(
            reasons,
            "keyset_threshold_invalid",
            format!(
                "threshold_m [{}] must be >= 2 for multi-verifier quorum",
                keyset.threshold_m
            ),
        );
    }

    if keyset.max_key_age_seconds == 0
        || keyset.max_key_age_seconds > MAX_VERIFIER_KEY_ROTATION_SECONDS
    {
        push_rejection_reason(
            reasons,
            "keyset_max_key_age_invalid",
            format!(
                "max_key_age_seconds [{}] must be within [1, {}]",
                keyset.max_key_age_seconds, MAX_VERIFIER_KEY_ROTATION_SECONDS
            ),
        );
    }

    if keyset.keys.len() < keyset.threshold_m {
        push_rejection_reason(
            reasons,
            "keyset_insufficient_keys",
            format!(
                "keyset has [{}] keys but threshold_m is [{}]",
                keyset.keys.len(),
                keyset.threshold_m
            ),
        );
    }

    let mut seen_key_ids: HashSet<String> = HashSet::new();
    let mut resolved_keys = HashMap::new();
    let mut active_key_count = 0usize;
    let mut active_trust_roots: HashSet<String> = HashSet::new();
    let mut active_instances: HashSet<String> = HashSet::new();

    for key in &keyset.keys {
        let Some(key_id) =
            required_non_empty("key_id", &key.key_id, reasons, "verifier_key_id_missing")
        else {
            continue;
        };
        let key_id_normalized = trimmed_lowercase(&key_id);
        if !seen_key_ids.insert(key_id_normalized.clone()) {
            push_rejection_reason(
                reasons,
                "verifier_key_id_duplicate",
                format!("verifier key_id [{}] is duplicated", key.key_id),
            );
            continue;
        }

        let Some(verifier_instance_id) = required_non_empty(
            "verifier_instance_id",
            &key.verifier_instance_id,
            reasons,
            "verifier_instance_id_missing",
        ) else {
            continue;
        };
        let Some(trust_root_id) = required_non_empty(
            "trust_root_id",
            &key.trust_root_id,
            reasons,
            "verifier_trust_root_id_missing",
        ) else {
            continue;
        };

        if key.valid_from_unix == 0 {
            push_rejection_reason(
                reasons,
                "verifier_valid_from_invalid",
                format!("verifier key [{}] valid_from_unix must be > 0", key.key_id),
            );
            continue;
        }

        if key.valid_until_unix <= key.valid_from_unix {
            push_rejection_reason(
                reasons,
                "verifier_validity_window_invalid",
                format!(
                    "verifier key [{}] valid_until_unix [{}] must be greater than valid_from_unix [{}]",
                    key.key_id, key.valid_until_unix, key.valid_from_unix
                ),
            );
            continue;
        }

        let key_age_seconds = key.valid_until_unix.saturating_sub(key.valid_from_unix);
        if keyset.max_key_age_seconds > 0 && key_age_seconds > keyset.max_key_age_seconds {
            push_rejection_reason(
                reasons,
                "verifier_key_age_exceeds_policy",
                format!(
                    "verifier key [{}] lifetime [{}] exceeds keyset max_key_age_seconds [{}]",
                    key.key_id, key_age_seconds, keyset.max_key_age_seconds
                ),
            );
        }

        if let Some(revoked_at_unix) = key.revoked_at_unix {
            if revoked_at_unix < key.valid_from_unix {
                push_rejection_reason(
                    reasons,
                    "verifier_key_revoked_before_valid_from",
                    format!(
                        "verifier key [{}] revoked_at_unix [{}] is before valid_from_unix [{}]",
                        key.key_id, revoked_at_unix, key.valid_from_unix
                    ),
                );
            }
        }

        let parsed_pubkey = match parse_xonly_pubkey_hex(&key.pubkey_hex) {
            Ok(parsed_pubkey) => parsed_pubkey,
            Err(detail) => {
                push_rejection_reason(
                    reasons,
                    "verifier_pubkey_invalid",
                    format!("verifier key [{}]: {detail}", key.key_id),
                );
                continue;
            }
        };

        let active_now = key.valid_from_unix <= now_unix_seconds
            && now_unix_seconds <= key.valid_until_unix
            && key
                .revoked_at_unix
                .is_none_or(|revoked_at_unix| now_unix_seconds < revoked_at_unix);
        if active_now {
            active_key_count = active_key_count.saturating_add(1);
            active_trust_roots.insert(trimmed_lowercase(&trust_root_id));
            active_instances.insert(trimmed_lowercase(&verifier_instance_id));
        }

        resolved_keys.insert(
            key_id_normalized,
            ResolvedVerifierKey {
                verifier_instance_id: verifier_instance_id.trim().to_string(),
                trust_root_id: trust_root_id.trim().to_string(),
                pubkey: parsed_pubkey,
                valid_from_unix: key.valid_from_unix,
                valid_until_unix: key.valid_until_unix,
                revoked_at_unix: key.revoked_at_unix,
            },
        );
    }

    if active_key_count < keyset.threshold_m {
        push_rejection_reason(
            reasons,
            "keyset_active_keys_below_threshold",
            format!(
                "active verifier keys [{}] below threshold_m [{}] at now [{}]",
                active_key_count, keyset.threshold_m, now_unix_seconds
            ),
        );
    }

    if active_key_count > 0 && active_trust_roots.len() < 2 {
        push_rejection_reason(
            reasons,
            "keyset_active_trust_roots_below_minimum",
            format!(
                "active verifier keys expose [{}] trust roots; require >= 2",
                active_trust_roots.len()
            ),
        );
    }

    if active_key_count > 0 && active_instances.len() < 2 {
        push_rejection_reason(
            reasons,
            "keyset_active_instances_below_minimum",
            format!(
                "active verifier keys expose [{}] verifier instances; require >= 2",
                active_instances.len()
            ),
        );
    }

    ResolvedVerifierKeySet {
        threshold_m: keyset.threshold_m,
        keys: resolved_keys,
    }
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

fn validate_token(
    registry: &TeeGovernanceRegistryV1,
    keyset: &VerifierKeySetV1,
    token_artifact: &AdmissionTokenArtifact,
    revocation_registry: &TokenRevocationRegistryV1,
    now_unix_seconds: u64,
) -> ValidationDecision {
    let mut reasons: Vec<ValidationReason> = Vec::new();

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

    let resolved_keyset = validate_verifier_keyset(keyset, now_unix_seconds, &mut reasons);

    if revocation_registry.denylist_refreshed_at_unix == 0 {
        push_rejection_reason(
            &mut reasons,
            "denylist_refreshed_at_invalid",
            "revocation registry denylist_refreshed_at_unix must be > 0".to_string(),
        );
    } else if revocation_registry.denylist_refreshed_at_unix > now_unix_seconds {
        push_rejection_reason(
            &mut reasons,
            "denylist_refreshed_at_in_future",
            format!(
                "denylist_refreshed_at_unix [{}] is in the future relative to now [{}]",
                revocation_registry.denylist_refreshed_at_unix, now_unix_seconds
            ),
        );
    } else {
        let denylist_age_seconds =
            now_unix_seconds.saturating_sub(revocation_registry.denylist_refreshed_at_unix);
        if denylist_age_seconds > registry.enforcement.denylist_max_staleness_seconds {
            push_rejection_reason(
                &mut reasons,
                "denylist_stale",
                format!(
                    "denylist age [{}] exceeds policy max staleness [{}]",
                    denylist_age_seconds, registry.enforcement.denylist_max_staleness_seconds
                ),
            );
        }
    }

    let payload_json = token_artifact.payload_json.trim();
    if payload_json.is_empty() {
        push_rejection_reason(
            &mut reasons,
            "token_payload_missing",
            "token artifact payload_json must be non-empty".to_string(),
        );
        return ValidationDecision {
            decision: "reject".to_string(),
            reasons,
            validated_at_unix: now_unix_seconds,
        };
    }

    let token_payload = match serde_json::from_str::<AdmissionTokenPayload>(payload_json) {
        Ok(token_payload) => token_payload,
        Err(error) => {
            push_rejection_reason(
                &mut reasons,
                "token_payload_invalid",
                format!("failed to parse token payload_json: {error}"),
            );
            return ValidationDecision {
                decision: "reject".to_string(),
                reasons,
                validated_at_unix: now_unix_seconds,
            };
        }
    };

    let Some(token_id) = required_non_empty(
        "token_id",
        &token_payload.token_id,
        &mut reasons,
        "token_id_missing",
    ) else {
        return ValidationDecision {
            decision: "reject".to_string(),
            reasons,
            validated_at_unix: now_unix_seconds,
        };
    };
    let token_id_normalized = trimmed_lowercase(&token_id);

    let Some(operator_id) = required_non_empty(
        "operator_id",
        &token_payload.operator_id,
        &mut reasons,
        "operator_id_missing",
    ) else {
        return ValidationDecision {
            decision: "reject".to_string(),
            reasons,
            validated_at_unix: now_unix_seconds,
        };
    };

    let Some(signer_identifier) = required_non_empty(
        "signer_identifier",
        &token_payload.signer_identifier,
        &mut reasons,
        "signer_identifier_missing",
    ) else {
        return ValidationDecision {
            decision: "reject".to_string(),
            reasons,
            validated_at_unix: now_unix_seconds,
        };
    };

    let Some(tee_type) = required_non_empty(
        "tee_type",
        &token_payload.tee_type,
        &mut reasons,
        "tee_type_missing",
    ) else {
        return ValidationDecision {
            decision: "reject".to_string(),
            reasons,
            validated_at_unix: now_unix_seconds,
        };
    };

    if !is_sha256_digest(&token_payload.measurement_digest) {
        push_rejection_reason(
            &mut reasons,
            "measurement_digest_invalid",
            format!(
                "measurement_digest [{}] must match sha256:<64 hex chars>",
                token_payload.measurement_digest
            ),
        );
    }

    if token_payload.issued_at_unix == 0 {
        push_rejection_reason(
            &mut reasons,
            "token_issued_at_invalid",
            "issued_at_unix must be > 0".to_string(),
        );
    }

    if token_payload.expires_at_unix <= token_payload.issued_at_unix {
        push_rejection_reason(
            &mut reasons,
            "token_expiry_invalid",
            format!(
                "expires_at_unix [{}] must be greater than issued_at_unix [{}]",
                token_payload.expires_at_unix, token_payload.issued_at_unix
            ),
        );
    }

    if token_payload.issued_at_unix > now_unix_seconds {
        push_rejection_reason(
            &mut reasons,
            "token_not_yet_valid",
            format!(
                "issued_at_unix [{}] is in the future relative to now [{}]",
                token_payload.issued_at_unix, now_unix_seconds
            ),
        );
    }

    if token_payload.expires_at_unix < now_unix_seconds {
        push_rejection_reason(
            &mut reasons,
            "token_expired",
            format!(
                "token expired at [{}], now [{}]",
                token_payload.expires_at_unix, now_unix_seconds
            ),
        );
    }

    if token_payload.registry_snapshot_version == 0 {
        push_rejection_reason(
            &mut reasons,
            "registry_snapshot_version_invalid",
            "registry_snapshot_version must be > 0".to_string(),
        );
    }

    if token_payload.verifier_key_ids.is_empty() {
        push_rejection_reason(
            &mut reasons,
            "token_verifier_key_ids_missing",
            "verifier_key_ids must contain at least one key_id".to_string(),
        );
    }

    let mut declared_verifier_key_ids: HashSet<String> = HashSet::new();
    for key_id in &token_payload.verifier_key_ids {
        let key_id_normalized = trimmed_lowercase(key_id);
        if key_id_normalized.is_empty() {
            push_rejection_reason(
                &mut reasons,
                "token_verifier_key_id_missing",
                "verifier_key_ids contains an empty key_id".to_string(),
            );
            continue;
        }

        if !declared_verifier_key_ids.insert(key_id_normalized.clone()) {
            push_rejection_reason(
                &mut reasons,
                "token_verifier_key_id_duplicate",
                format!("verifier_key_ids contains duplicate key_id [{}]", key_id),
            );
        }
    }

    if let Some(revoked_token_record) = revocation_registry
        .revoked_token_ids
        .get(&token_id_normalized)
    {
        if revoked_token_record.revoked_at_unix <= now_unix_seconds {
            let reason_suffix = if revoked_token_record.reason.trim().is_empty() {
                String::new()
            } else {
                format!(" (reason: {})", revoked_token_record.reason.trim())
            };
            push_rejection_reason(
                &mut reasons,
                "token_id_revoked",
                format!(
                    "token_id [{}] revoked at [{}]{}",
                    token_payload.token_id, revoked_token_record.revoked_at_unix, reason_suffix
                ),
            );
        }
    }

    if token_payload.token_revocation_epoch < revocation_registry.min_token_revocation_epoch {
        push_rejection_reason(
            &mut reasons,
            "token_revocation_epoch_below_minimum",
            format!(
                "token_revocation_epoch [{}] is below minimum [{}]",
                token_payload.token_revocation_epoch,
                revocation_registry.min_token_revocation_epoch
            ),
        );
    }

    let Some(operator_record) = find_operator(registry, &operator_id) else {
        push_rejection_reason(
            &mut reasons,
            "operator_not_found",
            format!(
                "operator_id [{}] not found in governance registry",
                operator_id
            ),
        );
        return ValidationDecision {
            decision: "reject".to_string(),
            reasons,
            validated_at_unix: now_unix_seconds,
        };
    };

    if parse_operator_status(&operator_record.status) != Some(OperatorStatus::Active) {
        push_rejection_reason(
            &mut reasons,
            "operator_not_active",
            format!(
                "operator_id [{}] status [{}] is not active",
                operator_record.operator_id, operator_record.status
            ),
        );
    }

    if trimmed_lowercase(&operator_record.signer_identifier)
        != trimmed_lowercase(&signer_identifier)
    {
        push_rejection_reason(
            &mut reasons,
            "signer_identifier_mismatch",
            format!(
                "token signer_identifier [{}] does not match registry signer_identifier [{}]",
                token_payload.signer_identifier, operator_record.signer_identifier
            ),
        );
    }

    let tee_type_allowed = operator_record
        .allowed_tee_types
        .iter()
        .any(|allowed| trimmed_lowercase(allowed) == trimmed_lowercase(&tee_type));
    if !tee_type_allowed {
        push_rejection_reason(
            &mut reasons,
            "tee_type_not_allowed",
            format!(
                "tee_type [{}] not present in operator allowlist {:?}",
                token_payload.tee_type, operator_record.allowed_tee_types
            ),
        );
    }

    let measurement_allowed = operator_record.allowed_measurements.iter().any(|allowed| {
        trimmed_lowercase(allowed) == trimmed_lowercase(&token_payload.measurement_digest)
    });
    if !measurement_allowed {
        push_rejection_reason(
            &mut reasons,
            "measurement_not_allowlisted",
            format!(
                "measurement_digest [{}] not present in operator allowlist",
                token_payload.measurement_digest
            ),
        );
    }

    if token_payload.issued_at_unix < operator_record.effective_from {
        push_rejection_reason(
            &mut reasons,
            "operator_not_yet_effective",
            format!(
                "token issued_at_unix [{}] is before operator effective_from [{}]",
                token_payload.issued_at_unix, operator_record.effective_from
            ),
        );
    }

    if let Some(effective_until) = operator_record.effective_until {
        if token_payload.issued_at_unix > effective_until {
            push_rejection_reason(
                &mut reasons,
                "operator_effective_window_expired",
                format!(
                    "token issued_at_unix [{}] exceeds operator effective_until [{}]",
                    token_payload.issued_at_unix, effective_until
                ),
            );
        }
    }

    let max_token_ttl_seconds = std::cmp::min(
        registry.enforcement.attestation_max_age_seconds,
        operator_record.attestation_max_age_seconds,
    );
    let token_ttl_seconds = token_payload
        .expires_at_unix
        .saturating_sub(token_payload.issued_at_unix);
    if token_ttl_seconds > max_token_ttl_seconds {
        push_rejection_reason(
            &mut reasons,
            "token_ttl_exceeds_attestation_max_age",
            format!(
                "token ttl [{}] exceeds max allowed [{}]",
                token_ttl_seconds, max_token_ttl_seconds
            ),
        );
    }

    if token_artifact.signatures.is_empty() {
        push_rejection_reason(
            &mut reasons,
            "token_signatures_missing",
            "token signatures must contain at least one signature".to_string(),
        );
    }

    let mut seen_signature_key_ids = HashSet::new();
    let mut valid_signature_count = 0usize;
    let mut valid_signature_trust_roots: HashSet<String> = HashSet::new();
    let mut valid_signature_instances: HashSet<String> = HashSet::new();

    for signature in &token_artifact.signatures {
        let Some(signature_key_id) = required_non_empty(
            "verifier_key_id",
            &signature.verifier_key_id,
            &mut reasons,
            "token_signature_key_id_missing",
        ) else {
            continue;
        };
        let signature_key_id_normalized = trimmed_lowercase(&signature_key_id);

        if !seen_signature_key_ids.insert(signature_key_id_normalized.clone()) {
            push_rejection_reason(
                &mut reasons,
                "token_signature_key_id_duplicate",
                format!(
                    "token signatures contain duplicate verifier_key_id [{}]",
                    signature.verifier_key_id
                ),
            );
            continue;
        }

        if !declared_verifier_key_ids.contains(&signature_key_id_normalized) {
            push_rejection_reason(
                &mut reasons,
                "token_signature_key_not_declared",
                format!(
                    "signature key_id [{}] not present in payload verifier_key_ids",
                    signature.verifier_key_id
                ),
            );
            continue;
        }

        let Some(resolved_key) = resolved_keyset.keys.get(&signature_key_id_normalized) else {
            push_rejection_reason(
                &mut reasons,
                "token_signature_key_unknown",
                format!(
                    "signature key_id [{}] not found in verifier keyset",
                    signature.verifier_key_id
                ),
            );
            continue;
        };

        if token_payload.issued_at_unix < resolved_key.valid_from_unix
            || token_payload.issued_at_unix > resolved_key.valid_until_unix
        {
            push_rejection_reason(
                &mut reasons,
                "token_signature_key_not_valid_at_issue_time",
                format!(
                    "signature key_id [{}] is not valid at issued_at_unix [{}]",
                    signature.verifier_key_id, token_payload.issued_at_unix
                ),
            );
            continue;
        }

        if resolved_key
            .revoked_at_unix
            .is_some_and(|revoked_at_unix| revoked_at_unix <= now_unix_seconds)
        {
            push_rejection_reason(
                &mut reasons,
                "token_signature_key_revoked",
                format!(
                    "signature key_id [{}] was revoked at or before now [{}]",
                    signature.verifier_key_id, now_unix_seconds
                ),
            );
            continue;
        }

        if let Some(revoked_key_record) = revocation_registry
            .revoked_verifier_key_ids
            .get(&signature_key_id_normalized)
        {
            if revoked_key_record.revoked_at_unix <= now_unix_seconds {
                let reason_suffix = if revoked_key_record.reason.trim().is_empty() {
                    String::new()
                } else {
                    format!(" (reason: {})", revoked_key_record.reason.trim())
                };
                push_rejection_reason(
                    &mut reasons,
                    "token_signature_key_revoked",
                    format!(
                        "signature key_id [{}] revoked in revocation registry at [{}]{}",
                        signature.verifier_key_id,
                        revoked_key_record.revoked_at_unix,
                        reason_suffix
                    ),
                );
                continue;
            }
        }

        if let Err(detail) =
            verify_schnorr_signature(payload_json, &signature.signature_hex, &resolved_key.pubkey)
        {
            push_rejection_reason(
                &mut reasons,
                "token_signature_verification_failed",
                format!("signature key_id [{}]: {detail}", signature.verifier_key_id),
            );
            continue;
        }

        valid_signature_count = valid_signature_count.saturating_add(1);
        valid_signature_trust_roots.insert(trimmed_lowercase(&resolved_key.trust_root_id));
        valid_signature_instances.insert(trimmed_lowercase(&resolved_key.verifier_instance_id));
    }

    if valid_signature_count < resolved_keyset.threshold_m {
        push_rejection_reason(
            &mut reasons,
            "token_signature_quorum_not_met",
            format!(
                "valid token signatures [{}] below threshold_m [{}]",
                valid_signature_count, resolved_keyset.threshold_m
            ),
        );
    }

    if valid_signature_count > 0 && valid_signature_trust_roots.len() < 2 {
        push_rejection_reason(
            &mut reasons,
            "token_signature_trust_root_diversity_violation",
            format!(
                "valid token signatures cover [{}] trust roots; require >= 2",
                valid_signature_trust_roots.len()
            ),
        );
    }

    if valid_signature_count > 0 && valid_signature_instances.len() < 2 {
        push_rejection_reason(
            &mut reasons,
            "token_signature_instance_diversity_violation",
            format!(
                "valid token signatures cover [{}] verifier instances; require >= 2",
                valid_signature_instances.len()
            ),
        );
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
    let keyset: VerifierKeySetV1 = load_json_file(&cli.keyset_path)?;
    let token_artifact: AdmissionTokenArtifact = load_json_file(&cli.token_path)?;
    let revocation_registry: TokenRevocationRegistryV1 =
        normalize_revocation_registry(load_json_file(&cli.revocation_registry_path)?);
    let now_unix_seconds = cli.now_unix_override.unwrap_or_else(now_unix);

    Ok(validate_token(
        &registry,
        &keyset,
        &token_artifact,
        &revocation_registry,
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

    struct SigningKeyFixture {
        key_id: String,
        keypair: bitcoin::secp256k1::Keypair,
        verifier_instance_id: String,
        trust_root_id: String,
        pubkey_hex: String,
    }

    fn signing_key(
        seed: u8,
        key_id: &str,
        verifier_instance_id: &str,
        trust_root_id: &str,
    ) -> SigningKeyFixture {
        let secp = Secp256k1::new();
        let secret_key =
            bitcoin::secp256k1::SecretKey::from_slice(&[seed; 32]).expect("secret key");
        let keypair = bitcoin::secp256k1::Keypair::from_secret_key(&secp, &secret_key);
        let (pubkey, _) = XOnlyPublicKey::from_keypair(&keypair);

        SigningKeyFixture {
            key_id: key_id.to_string(),
            keypair,
            verifier_instance_id: verifier_instance_id.to_string(),
            trust_root_id: trust_root_id.to_string(),
            pubkey_hex: pubkey.to_string(),
        }
    }

    fn sign_payload(payload_json: &str, keypair: &bitcoin::secp256k1::Keypair) -> String {
        let secp = Secp256k1::new();
        let digest = Sha256::digest(payload_json.as_bytes());
        let message = SecpMessage::from_digest_slice(&digest).expect("digest message");
        secp.sign_schnorr_no_aux_rand(&message, keypair).to_string()
    }

    fn baseline_signing_keys() -> Vec<SigningKeyFixture> {
        vec![
            signing_key(0x11, "verifier-key-1", "verifier-a", "trust-root-a"),
            signing_key(0x22, "verifier-key-2", "verifier-b", "trust-root-b"),
            signing_key(0x33, "verifier-key-3", "verifier-c", "trust-root-c"),
        ]
    }

    fn baseline_registry() -> TeeGovernanceRegistryV1 {
        TeeGovernanceRegistryV1 {
            profile_status: "mandatory".to_string(),
            enforcement: TeeEnforcementParameters {
                attestation_max_age_seconds: 3_600,
                denylist_max_staleness_seconds: 60,
            },
            operators: vec![TeeOperatorAdmissionRecord {
                operator_id: "operator-1".to_string(),
                signer_identifier: "signer-1".to_string(),
                status: "active".to_string(),
                allowed_tee_types: vec!["sgx".to_string(), "sev-snp".to_string()],
                allowed_measurements: vec![
                    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
                        .to_string(),
                ],
                attestation_max_age_seconds: 3_600,
                effective_from: 1_700_000_000,
                effective_until: None,
            }],
        }
    }

    fn baseline_keyset(signing_keys: &[SigningKeyFixture]) -> VerifierKeySetV1 {
        VerifierKeySetV1 {
            keyset_version: 1,
            threshold_m: 2,
            max_key_age_seconds: MAX_VERIFIER_KEY_ROTATION_SECONDS,
            keys: signing_keys
                .iter()
                .map(|key| VerifierKeyRecord {
                    key_id: key.key_id.clone(),
                    verifier_instance_id: key.verifier_instance_id.clone(),
                    trust_root_id: key.trust_root_id.clone(),
                    pubkey_hex: key.pubkey_hex.clone(),
                    valid_from_unix: 1_700_000_000,
                    valid_until_unix: 1_700_259_200,
                    revoked_at_unix: None,
                })
                .collect(),
        }
    }

    fn baseline_revocation_registry(now_unix_seconds: u64) -> TokenRevocationRegistryV1 {
        TokenRevocationRegistryV1 {
            denylist_refreshed_at_unix: now_unix_seconds,
            min_token_revocation_epoch: 5,
            revoked_token_ids: HashMap::new(),
            revoked_verifier_key_ids: HashMap::new(),
        }
    }

    fn baseline_token_payload(now_unix_seconds: u64) -> AdmissionTokenPayload {
        AdmissionTokenPayload {
            token_id: "token-operator-1-0001".to_string(),
            operator_id: "operator-1".to_string(),
            signer_identifier: "signer-1".to_string(),
            tee_type: "sgx".to_string(),
            measurement_digest:
                "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
                    .to_string(),
            issued_at_unix: now_unix_seconds.saturating_sub(120),
            expires_at_unix: now_unix_seconds.saturating_add(300),
            registry_snapshot_version: 1,
            verifier_key_ids: vec!["verifier-key-1".to_string(), "verifier-key-2".to_string()],
            token_revocation_epoch: 5,
        }
    }

    fn build_signed_artifact(
        payload: &AdmissionTokenPayload,
        signing_keys: &[SigningKeyFixture],
        signing_key_ids: &[&str],
    ) -> AdmissionTokenArtifact {
        let payload_json = serde_json::to_string(payload).expect("payload json");
        let signatures = signing_key_ids
            .iter()
            .map(|key_id| {
                let signing_key = signing_keys
                    .iter()
                    .find(|key| key.key_id == *key_id)
                    .expect("signing key fixture");
                TokenSignature {
                    verifier_key_id: signing_key.key_id.clone(),
                    signature_hex: sign_payload(&payload_json, &signing_key.keypair),
                }
            })
            .collect();

        AdmissionTokenArtifact {
            payload_json,
            signatures,
        }
    }

    #[test]
    fn validate_token_allows_valid_threshold_signed_token() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "allow");
        assert!(decision.reasons.is_empty());
    }

    #[test]
    fn validate_token_rejects_stale_denylist() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let mut revocation_registry = baseline_revocation_registry(now_unix_seconds);
        revocation_registry.denylist_refreshed_at_unix = now_unix_seconds.saturating_sub(61);
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "denylist_stale"));
    }

    #[test]
    fn validate_token_rejects_revoked_token_id() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let mut revocation_registry = baseline_revocation_registry(now_unix_seconds);
        revocation_registry.revoked_token_ids.insert(
            "token-operator-1-0001".to_string(),
            RevokedTokenRecord {
                revoked_at_unix: now_unix_seconds.saturating_sub(10),
                reason: "manual revoke".to_string(),
            },
        );
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_id_revoked"));
    }

    #[test]
    fn validate_token_rejects_token_revocation_epoch_below_minimum() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let mut revocation_registry = baseline_revocation_registry(now_unix_seconds);
        revocation_registry.min_token_revocation_epoch = 7;
        let mut payload = baseline_token_payload(now_unix_seconds);
        payload.token_revocation_epoch = 6;
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_revocation_epoch_below_minimum"));
    }

    #[test]
    fn validate_token_rejects_insufficient_signature_quorum() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(&payload, &signing_keys, &["verifier-key-1"]);

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_signature_quorum_not_met"));
    }

    #[test]
    fn validate_token_rejects_invalid_signature() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let payload = baseline_token_payload(now_unix_seconds);
        let mut artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );
        artifact.signatures[0].signature_hex = "00".repeat(64);

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_signature_verification_failed"));
    }

    #[test]
    fn validate_token_rejects_signature_key_not_declared_in_payload() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let mut payload = baseline_token_payload(now_unix_seconds);
        payload.verifier_key_ids = vec!["verifier-key-1".to_string()];
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_signature_key_not_declared"));
    }

    #[test]
    fn validate_token_rejects_when_operator_not_active() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let mut registry = baseline_registry();
        registry.operators[0].status = "suspended".to_string();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "operator_not_active"));
    }

    #[test]
    fn validate_token_rejects_measurement_not_allowlisted() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let mut payload = baseline_token_payload(now_unix_seconds);
        payload.measurement_digest =
            "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb".to_string();
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "measurement_not_allowlisted"));
    }

    #[test]
    fn validate_token_rejects_tee_type_not_allowed() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let mut payload = baseline_token_payload(now_unix_seconds);
        payload.tee_type = "tdx".to_string();
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "tee_type_not_allowed"));
    }

    #[test]
    fn validate_token_rejects_keyset_max_key_age_exceeding_30_days() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let mut keyset = baseline_keyset(&signing_keys);
        keyset.max_key_age_seconds = MAX_VERIFIER_KEY_ROTATION_SECONDS + 1;
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "keyset_max_key_age_invalid"));
    }

    #[test]
    fn validate_token_rejects_trust_root_diversity_violation() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let mut keyset = baseline_keyset(&signing_keys);
        keyset.keys[1].trust_root_id = keyset.keys[0].trust_root_id.clone();
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision.reasons.iter().any(|reason| {
            reason.code == "token_signature_trust_root_diversity_violation"
                || reason.code == "keyset_active_trust_roots_below_minimum"
        }));
    }

    #[test]
    fn validate_token_rejects_compromised_signature_key() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let mut revocation_registry = baseline_revocation_registry(now_unix_seconds);
        revocation_registry.revoked_verifier_key_ids.insert(
            "verifier-key-1".to_string(),
            RevokedVerifierKeyRecord {
                revoked_at_unix: now_unix_seconds.saturating_sub(100),
                reason: "compromised".to_string(),
            },
        );
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_signature_key_revoked"));
    }

    #[test]
    fn validate_token_rejects_revoked_token_id_case_insensitive() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let mut revocation_registry = baseline_revocation_registry(now_unix_seconds);
        revocation_registry.revoked_token_ids.insert(
            "TOKEN-OPERATOR-1-0001".to_string(),
            RevokedTokenRecord {
                revoked_at_unix: now_unix_seconds.saturating_sub(10),
                reason: "case-mismatch regression test".to_string(),
            },
        );
        revocation_registry = normalize_revocation_registry(revocation_registry);
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_id_revoked"));
    }

    #[test]
    fn validate_token_rejects_revoked_verifier_key_case_insensitive() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let mut revocation_registry = baseline_revocation_registry(now_unix_seconds);
        revocation_registry.revoked_verifier_key_ids.insert(
            "VERIFIER-KEY-1".to_string(),
            RevokedVerifierKeyRecord {
                revoked_at_unix: now_unix_seconds.saturating_sub(100),
                reason: "case-mismatch regression test".to_string(),
            },
        );
        revocation_registry = normalize_revocation_registry(revocation_registry);
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_signature_key_revoked"));
    }

    #[test]
    fn validate_token_rejects_when_governance_profile_not_mandatory() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let mut registry = baseline_registry();
        registry.profile_status = "draft".to_string();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let payload = baseline_token_payload(now_unix_seconds);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "governance_profile_not_mandatory"));
    }

    #[test]
    fn validate_token_rejects_expired_token() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let mut payload = baseline_token_payload(now_unix_seconds);
        payload.issued_at_unix = now_unix_seconds.saturating_sub(500);
        payload.expires_at_unix = now_unix_seconds.saturating_sub(1);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_expired"));
    }

    #[test]
    fn validate_token_rejects_future_issued_at() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let mut payload = baseline_token_payload(now_unix_seconds);
        payload.issued_at_unix = now_unix_seconds.saturating_add(600);
        payload.expires_at_unix = now_unix_seconds.saturating_add(900);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_not_yet_valid"));
    }

    #[test]
    fn validate_token_rejects_operator_not_found() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let mut payload = baseline_token_payload(now_unix_seconds);
        payload.operator_id = "operator-unknown".to_string();
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "operator_not_found"));
    }

    #[test]
    fn validate_token_rejects_signer_identifier_mismatch() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let mut payload = baseline_token_payload(now_unix_seconds);
        payload.signer_identifier = "signer-wrong".to_string();
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "signer_identifier_mismatch"));
    }

    #[test]
    fn validate_token_rejects_ttl_exceeding_attestation_max_age() {
        let now_unix_seconds = 1_700_100_000u64;
        let signing_keys = baseline_signing_keys();
        let registry = baseline_registry();
        let keyset = baseline_keyset(&signing_keys);
        let revocation_registry = baseline_revocation_registry(now_unix_seconds);
        let mut payload = baseline_token_payload(now_unix_seconds);
        payload.issued_at_unix = now_unix_seconds.saturating_sub(100);
        payload.expires_at_unix = payload.issued_at_unix.saturating_add(7_200);
        let artifact = build_signed_artifact(
            &payload,
            &signing_keys,
            &["verifier-key-1", "verifier-key-2"],
        );

        let decision = validate_token(
            &registry,
            &keyset,
            &artifact,
            &revocation_registry,
            now_unix_seconds,
        );
        assert_eq!(decision.decision, "reject");
        assert!(decision
            .reasons
            .iter()
            .any(|reason| reason.code == "token_ttl_exceeds_attestation_max_age"));
    }

    #[test]
    fn parse_args_accepts_required_flags() {
        let args = vec![
            "--registry".to_string(),
            "registry.json".to_string(),
            "--keyset".to_string(),
            "keyset.json".to_string(),
            "--token".to_string(),
            "token.json".to_string(),
            "--revocation-registry".to_string(),
            "revocations.json".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.registry_path, PathBuf::from("registry.json"));
        assert_eq!(parsed.keyset_path, PathBuf::from("keyset.json"));
        assert_eq!(parsed.token_path, PathBuf::from("token.json"));
        assert_eq!(
            parsed.revocation_registry_path,
            PathBuf::from("revocations.json")
        );
        assert!(parsed.now_unix_override.is_none());
    }

    #[test]
    fn parse_args_accepts_now_unix() {
        let args = vec![
            "--registry".to_string(),
            "registry.json".to_string(),
            "--keyset".to_string(),
            "keyset.json".to_string(),
            "--token".to_string(),
            "token.json".to_string(),
            "--revocation-registry".to_string(),
            "revocations.json".to_string(),
            "--now-unix".to_string(),
            "1700100000".to_string(),
        ];

        let parsed = parse_args(&args).expect("parse args");
        assert_eq!(parsed.now_unix_override, Some(1_700_100_000));
    }

    #[test]
    fn parse_args_rejects_missing_required_flags() {
        let args = vec![
            "--registry".to_string(),
            "registry.json".to_string(),
            "--keyset".to_string(),
            "keyset.json".to_string(),
            "--token".to_string(),
            "token.json".to_string(),
        ];

        let error = parse_args(&args).expect_err("expected parse failure");
        assert_eq!(error, "missing required --revocation-registry");
    }
}
