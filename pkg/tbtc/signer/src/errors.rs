use thiserror::Error;

#[derive(Debug, Error)]
pub enum EngineError {
    #[error("validation failed: {0}")]
    Validation(String),
    #[error("provenance gate rejected: {reason_code}: {detail}")]
    ProvenanceGateRejected { reason_code: String, detail: String },
    #[error("admission policy rejected for session {session_id}: {reason_code}: {detail}")]
    AdmissionPolicyRejected {
        session_id: String,
        reason_code: String,
        detail: String,
    },
    #[error("signing policy rejected for session {session_id}: {reason_code}: {detail}")]
    SigningPolicyRejected {
        session_id: String,
        reason_code: String,
        detail: String,
    },
    #[error("quarantine policy rejected for session {session_id}: {reason_code}: {detail}")]
    QuarantinePolicyRejected {
        session_id: String,
        reason_code: String,
        detail: String,
    },
    #[error("lifecycle policy rejected for session {session_id}: {reason_code}: {detail}")]
    LifecyclePolicyRejected {
        session_id: String,
        reason_code: String,
        detail: String,
    },
    #[error(
        "synthetic contributions rejected for session {session_id}: bootstrap-only finalize payload is not allowed"
    )]
    SyntheticContributionRejected { session_id: String },
    #[error("session conflict for {session_id}: repeated call must use identical payload")]
    SessionConflict { session_id: String },
    #[error("session finalized for {session_id}: start_sign_round requires a new session_id")]
    SessionFinalized { session_id: String },
    #[error("session not found: {session_id}")]
    SessionNotFound { session_id: String },
    #[error("DKG must be completed before signing session {session_id}")]
    DkgNotReady { session_id: String },
    #[error("sign round not started for session {session_id}")]
    SignRoundNotStarted { session_id: String },
    /// Returned when an `attempt_id` that has already been consumed for a sign
    /// attempt in this session arrives again. Distinct from the generic
    /// `Validation` error so cross-language callers can match on the
    /// `consumed_attempt_replay` code instead of substring-matching the
    /// message wording.
    #[error(
        "attempt_id [{attempt_id}] already consumed for sign attempt in session [{session_id}]"
    )]
    ConsumedAttemptReplay {
        session_id: String,
        attempt_id: String,
    },
    /// Returned when a derived `round_id` (a function of session, key group,
    /// message digest, signing-participants fingerprint, and attempt context)
    /// has already been consumed for a sign contribution. Distinct from
    /// `ConsumedAttemptReplay` because a single attempt context can produce
    /// multiple round IDs through canonicalization disagreements; callers
    /// match on `consumed_round_replay` rather than the message.
    #[error(
        "round_id [{round_id}] already consumed for sign contribution in session [{session_id}]"
    )]
    ConsumedRoundReplay {
        session_id: String,
        round_id: String,
    },
    #[error("internal error: {0}")]
    Internal(String),
}

impl EngineError {
    pub fn code(&self) -> &'static str {
        match self {
            Self::Validation(_) => "validation_error",
            Self::ProvenanceGateRejected { .. } => "provenance_gate_rejected",
            Self::AdmissionPolicyRejected { .. } => "admission_policy_rejected",
            Self::SigningPolicyRejected { .. } => "signing_policy_rejected",
            Self::QuarantinePolicyRejected { .. } => "quarantine_policy_rejected",
            Self::LifecyclePolicyRejected { .. } => "lifecycle_policy_rejected",
            Self::SyntheticContributionRejected { .. } => "synthetic_contribution_rejected",
            Self::SessionConflict { .. } => "session_conflict",
            Self::SessionFinalized { .. } => "session_finalized",
            Self::SessionNotFound { .. } => "session_not_found",
            Self::DkgNotReady { .. } => "dkg_not_ready",
            Self::SignRoundNotStarted { .. } => "sign_round_not_started",
            Self::ConsumedAttemptReplay { .. } => "consumed_attempt_replay",
            Self::ConsumedRoundReplay { .. } => "consumed_round_replay",
            Self::Internal(_) => "internal_error",
        }
    }

    pub fn recovery_class(&self) -> &'static str {
        match self {
            Self::Validation(_) => "recoverable",
            Self::ProvenanceGateRejected { .. } => "terminal",
            Self::AdmissionPolicyRejected { .. } => "recoverable",
            Self::SigningPolicyRejected { .. } => "recoverable",
            Self::QuarantinePolicyRejected { .. } => "recoverable",
            Self::LifecyclePolicyRejected { .. } => "recoverable",
            Self::SyntheticContributionRejected { .. } => "recoverable",
            Self::SessionConflict { .. } => "recoverable",
            Self::DkgNotReady { .. } => "recoverable",
            Self::SignRoundNotStarted { .. } => "recoverable",
            // ConsumedAttemptReplay / ConsumedRoundReplay are recoverable in
            // the sense that a fresh attempt with a new identifier can be
            // started. They cannot be retried with the same identifier — the
            // consumer (keep-core) treats them as a signal to mint a new
            // attempt_id rather than retransmit.
            Self::ConsumedAttemptReplay { .. } => "recoverable",
            Self::ConsumedRoundReplay { .. } => "recoverable",
            Self::SessionFinalized { .. } => "terminal",
            Self::SessionNotFound { .. } => "terminal",
            Self::Internal(_) => "terminal",
        }
    }
}

#[cfg(test)]
mod tests {
    use super::EngineError;

    #[test]
    fn consumed_attempt_replay_has_stable_code_and_message_format() {
        let err = EngineError::ConsumedAttemptReplay {
            session_id: "session-a".to_string(),
            attempt_id: "attempt-1".to_string(),
        };
        assert_eq!(err.code(), "consumed_attempt_replay");
        assert_eq!(err.recovery_class(), "recoverable");
        // Wire wording must remain stable across releases so legacy keep-core
        // builds that substring-match the message keep working until they
        // migrate to the code field.
        assert_eq!(
            err.to_string(),
            "attempt_id [attempt-1] already consumed for sign attempt in session [session-a]",
        );
    }

    #[test]
    fn consumed_round_replay_has_stable_code_and_message_format() {
        let err = EngineError::ConsumedRoundReplay {
            session_id: "session-a".to_string(),
            round_id: "round-1".to_string(),
        };
        assert_eq!(err.code(), "consumed_round_replay");
        assert_eq!(err.recovery_class(), "recoverable");
        assert_eq!(
            err.to_string(),
            "round_id [round-1] already consumed for sign contribution in session [session-a]",
        );
    }

    #[test]
    fn recovery_class_maps_retryable_and_terminal_errors() {
        assert_eq!(
            EngineError::Validation("bad request".to_string()).recovery_class(),
            "recoverable"
        );
        assert_eq!(
            EngineError::SessionConflict {
                session_id: "session-a".to_string(),
            }
            .recovery_class(),
            "recoverable"
        );
        assert_eq!(
            EngineError::ProvenanceGateRejected {
                reason_code: "missing_attestation_status".to_string(),
                detail: "missing env".to_string(),
            }
            .recovery_class(),
            "terminal"
        );
        assert_eq!(
            EngineError::AdmissionPolicyRejected {
                session_id: "session-a".to_string(),
                reason_code: "required_identifier_missing".to_string(),
                detail: "detail".to_string(),
            }
            .recovery_class(),
            "recoverable"
        );
        assert_eq!(
            EngineError::SessionFinalized {
                session_id: "session-a".to_string(),
            }
            .recovery_class(),
            "terminal"
        );
        assert_eq!(
            EngineError::Internal("panic".to_string()).recovery_class(),
            "terminal"
        );
    }
}
