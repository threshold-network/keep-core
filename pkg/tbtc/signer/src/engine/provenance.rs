// Runtime provenance attestation gate.

use super::*;

#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub(crate) struct ParsedVersionTriplet {
    pub(crate) major: u64,
    pub(crate) minor: u64,
    pub(crate) patch: u64,
    pub(crate) has_prerelease_suffix: bool,
}

pub(crate) fn parse_version_triplet(version: &str) -> Option<ParsedVersionTriplet> {
    let mut core_version = version.trim();
    if let Some((prefix, _)) = core_version.split_once('+') {
        core_version = prefix;
    }
    let has_prerelease_suffix = core_version.contains('-');
    if let Some((prefix, _)) = core_version.split_once('-') {
        core_version = prefix;
    }

    let mut segments = core_version.split('.');
    let major = segments.next()?.parse::<u64>().ok()?;
    let minor = segments.next()?.parse::<u64>().ok()?;
    let patch = segments.next()?.parse::<u64>().ok()?;
    if segments.next().is_some() {
        return None;
    }

    Some(ParsedVersionTriplet {
        major,
        minor,
        patch,
        has_prerelease_suffix,
    })
}

pub(crate) fn runtime_satisfies_minimum_version(
    runtime_version: ParsedVersionTriplet,
    minimum_version: ParsedVersionTriplet,
) -> bool {
    if runtime_version.major != minimum_version.major {
        return runtime_version.major > minimum_version.major;
    }
    if runtime_version.minor != minimum_version.minor {
        return runtime_version.minor > minimum_version.minor;
    }
    if runtime_version.patch != minimum_version.patch {
        return runtime_version.patch > minimum_version.patch;
    }

    if runtime_version.has_prerelease_suffix && !minimum_version.has_prerelease_suffix {
        return false;
    }

    true
}

#[derive(Clone, Debug, Deserialize)]
pub(crate) struct ProvenanceAttestationPayload {
    pub(crate) status: String,
    pub(crate) runtime_version: String,
    #[serde(default)]
    pub(crate) expires_at_unix: Option<u64>,
}

pub(crate) fn parse_provenance_trust_root_pubkey(
    trust_root: &str,
) -> Result<XOnlyPublicKey, EngineError> {
    let trust_root_bytes =
        hex::decode(trust_root).map_err(|_| EngineError::ProvenanceGateRejected {
            reason_code: "invalid_trust_root_format".to_string(),
            detail: format!(
                "env [{}] must be 32-byte x-only public key hex",
                TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV
            ),
        })?;

    if trust_root_bytes.len() != 32 {
        return Err(EngineError::ProvenanceGateRejected {
            reason_code: "invalid_trust_root_format".to_string(),
            detail: format!(
                "env [{}] must decode to 32-byte x-only public key",
                TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV
            ),
        });
    }

    XOnlyPublicKey::from_slice(&trust_root_bytes).map_err(|_| EngineError::ProvenanceGateRejected {
        reason_code: "invalid_trust_root_format".to_string(),
        detail: format!(
            "env [{}] must decode to valid x-only secp256k1 public key",
            TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV
        ),
    })
}

pub(crate) fn parse_provenance_attestation_payload(
    payload: &str,
) -> Result<ProvenanceAttestationPayload, EngineError> {
    serde_json::from_str::<ProvenanceAttestationPayload>(payload).map_err(|_| {
        EngineError::ProvenanceGateRejected {
            reason_code: "invalid_attestation_payload".to_string(),
            detail: format!(
                "env [{}] must be JSON with fields [status, runtime_version]",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV
            ),
        }
    })
}

pub(crate) fn verify_provenance_attestation_signature(
    attestation_payload: &str,
    attestation_signature_hex: &str,
    trust_root_pubkey: &XOnlyPublicKey,
) -> Result<(), EngineError> {
    let signature_bytes = hex::decode(attestation_signature_hex).map_err(|_| {
        EngineError::ProvenanceGateRejected {
            reason_code: "invalid_attestation_signature_format".to_string(),
            detail: format!(
                "env [{}] must be schnorr signature hex",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV
            ),
        }
    })?;
    let signature = SchnorrSignature::from_slice(&signature_bytes).map_err(|_| {
        EngineError::ProvenanceGateRejected {
            reason_code: "invalid_attestation_signature_format".to_string(),
            detail: format!(
                "env [{}] must decode to valid schnorr signature bytes",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV
            ),
        }
    })?;

    let payload_digest = Sha256::digest(attestation_payload.as_bytes());
    let message = SecpMessage::from_digest_slice(&payload_digest).map_err(|e| {
        EngineError::Internal(format!(
            "failed to construct provenance signature digest: {e}"
        ))
    })?;
    let secp = Secp256k1::verification_only();
    secp.verify_schnorr(&signature, &message, trust_root_pubkey)
        .map_err(|e| EngineError::ProvenanceGateRejected {
            reason_code: "attestation_signature_verification_failed".to_string(),
            detail: format!("failed to verify attestation signature: {e}"),
        })
}

pub(crate) fn reject_provenance_gate(
    reason_code: &str,
    detail: impl Into<String>,
) -> Result<(), EngineError> {
    Err(EngineError::ProvenanceGateRejected {
        reason_code: reason_code.to_string(),
        detail: detail.into(),
    })
}

pub(crate) fn enforce_provenance_gate() -> Result<(), EngineError> {
    if !provenance_gate_enforced() {
        return Ok(());
    }

    let attestation_status = signer_env_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV)
        .unwrap_or_default()
        .trim()
        .to_ascii_lowercase();
    if attestation_status.is_empty() {
        return reject_provenance_gate(
            "missing_attestation_status",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV
            ),
        );
    }
    if attestation_status != TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED {
        return reject_provenance_gate(
            "unapproved_attestation_status",
            format!(
                "attestation status must be [{}], got [{}]",
                TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED, attestation_status
            ),
        );
    }

    let trust_root = signer_env_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV)
        .unwrap_or_default()
        .trim()
        .to_string();
    if trust_root.is_empty() {
        return reject_provenance_gate(
            "missing_trust_root",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV
            ),
        );
    }
    let trust_root_pubkey = parse_provenance_trust_root_pubkey(&trust_root)?;

    let raw_attestation_payload =
        signer_env_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV).unwrap_or_default();
    let attestation_payload = raw_attestation_payload.trim().to_string();
    if attestation_payload.len() != raw_attestation_payload.len() {
        eprintln!(
            "provenance_gate: warning: env [{}] had leading/trailing whitespace (trimmed {} bytes)",
            TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
            raw_attestation_payload
                .len()
                .saturating_sub(attestation_payload.len())
        );
    }
    if attestation_payload.is_empty() {
        return reject_provenance_gate(
            "missing_attestation_payload",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV
            ),
        );
    }

    let attestation_signature_hex =
        signer_env_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV)
            .unwrap_or_default()
            .trim()
            .to_string();
    if attestation_signature_hex.is_empty() {
        return reject_provenance_gate(
            "missing_attestation_signature",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV
            ),
        );
    }

    verify_provenance_attestation_signature(
        &attestation_payload,
        &attestation_signature_hex,
        &trust_root_pubkey,
    )?;
    let parsed_attestation_payload = parse_provenance_attestation_payload(&attestation_payload)?;
    let attestation_payload_status = parsed_attestation_payload
        .status
        .trim()
        .to_ascii_lowercase();
    if attestation_payload_status != attestation_status {
        return reject_provenance_gate(
            "attestation_status_mismatch",
            format!(
                "attestation payload status [{}] does not match env status [{}]",
                attestation_payload_status, attestation_status
            ),
        );
    }
    if parsed_attestation_payload.runtime_version.trim() != TBTC_SIGNER_RUNTIME_VERSION {
        return reject_provenance_gate(
            "runtime_version_not_attested",
            format!(
                "attestation payload runtime version [{}] does not match runtime version [{}]",
                parsed_attestation_payload.runtime_version, TBTC_SIGNER_RUNTIME_VERSION
            ),
        );
    }
    let now_unix_seconds = now_unix();
    if now_unix_seconds == 0 {
        return reject_provenance_gate(
            "clock_unavailable",
            "system clock returned epoch zero; cannot verify attestation freshness",
        );
    }

    let expires_at_unix = parsed_attestation_payload.expires_at_unix.ok_or_else(|| {
        EngineError::ProvenanceGateRejected {
            reason_code: "missing_attestation_expiry".to_string(),
            detail: format!(
                "attestation payload must include expires_at_unix (max TTL: {} seconds)",
                TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS
            ),
        }
    })?;

    if now_unix_seconds > expires_at_unix {
        return reject_provenance_gate(
            "attestation_expired",
            format!(
                "attestation expired at [{}], now [{}]",
                expires_at_unix, now_unix_seconds
            ),
        );
    }

    let max_expiry_unix =
        now_unix_seconds.saturating_add(TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS);
    if expires_at_unix > max_expiry_unix {
        return reject_provenance_gate(
            "attestation_expiry_too_far_in_future",
            format!(
                "attestation expires_at_unix [{}] exceeds max TTL [{} seconds] from now [{}]",
                expires_at_unix,
                TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS,
                now_unix_seconds
            ),
        );
    }

    let min_approved_version = signer_env_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV)
        .unwrap_or_default()
        .trim()
        .to_string();
    if min_approved_version.is_empty() {
        return reject_provenance_gate(
            "missing_minimum_approved_version",
            format!(
                "missing required env [{}]",
                TBTC_SIGNER_MIN_APPROVED_VERSION_ENV
            ),
        );
    }

    let runtime_version = parse_version_triplet(TBTC_SIGNER_RUNTIME_VERSION).ok_or_else(|| {
        EngineError::Internal(format!(
            "invalid runtime version format [{}]",
            TBTC_SIGNER_RUNTIME_VERSION
        ))
    })?;
    let required_version = parse_version_triplet(&min_approved_version).ok_or_else(|| {
        EngineError::ProvenanceGateRejected {
            reason_code: "invalid_minimum_approved_version".to_string(),
            detail: format!(
                "minimum approved version [{}] is not semver triplet",
                min_approved_version
            ),
        }
    })?;

    if !runtime_satisfies_minimum_version(runtime_version, required_version) {
        return reject_provenance_gate(
            "runtime_version_below_minimum",
            format!(
                "runtime version [{}] below minimum approved version [{}]",
                TBTC_SIGNER_RUNTIME_VERSION, min_approved_version
            ),
        );
    }

    Ok(())
}
