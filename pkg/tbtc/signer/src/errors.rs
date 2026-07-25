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
        "cryptographic share refresh is not supported for session {session_id}: a multi-round, zero-constant FROST refresh protocol is required"
    )]
    CryptographicRefreshNotSupported { session_id: String },
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
    /// Returned when an interactive attempt whose nonce handle was already
    /// consumed (a signature share was released, or release was durably
    /// committed) is touched again - a second Round2 with the same handle,
    /// or Round1/SessionOpen for a consumed attempt. The caller must mint a
    /// new attempt; the engine will never release a second share under one
    /// nonce pair (frozen Phase 7 spec, section 4).
    #[error(
        "interactive attempt [{attempt_id}] already consumed its nonces in session [{session_id}]"
    )]
    ConsumedNonceReplay {
        session_id: String,
        attempt_id: String,
    },
    /// Returned when InteractiveAggregate is invoked again for an attempt that
    /// already produced an aggregate signature in this session. The per-attempt
    /// "aggregated" marker is durable, so a completed attempt stays completed
    /// across restart; re-aggregation is rejected rather than recomputed
    /// (a lost signature is recovered with a fresh attempt, not by replay).
    /// Distinct code so callers match on
    /// `interactive_attempt_already_aggregated` rather than the message.
    #[error("interactive attempt [{attempt_id}] already aggregated in session [{session_id}]")]
    InteractiveAttemptAlreadyAggregated {
        session_id: String,
        attempt_id: String,
    },
    /// Returned when InteractiveAggregate fails because one or more signature
    /// shares did not verify against the (tweaked) group verifying material.
    /// Unlike the generic `Validation` failure, this carries the FROST-identified
    /// CANDIDATE culprits as u16 Go member identifiers - every member whose share
    /// failed, via `CheaterDetection::AllCheaters` - so the Go host can feed them
    /// into envelope-bound blame adjudication. CANDIDATES, not a verdict: the
    /// engine verifies pure FROST shares against the group's own verifying
    /// material and never inspects operator-signed envelopes (frozen Q1
    /// boundary); a coordinator that aggregated honest shares against a
    /// substituted package or root would make those honest shares appear here.
    /// Authoritative blame is the Go host's at an f+1 accuser quorum. Fail-closed:
    /// no signature is produced. Distinct code so callers match on
    /// `aggregate_share_verification_failed` rather than the message.
    #[error(
        "InteractiveAggregate: {} signature share(s) failed verification for attempt [{attempt_id}] in session [{session_id}]",
        candidate_culprits.len()
    )]
    AggregateShareVerificationFailed {
        session_id: String,
        attempt_id: String,
        candidate_culprits: Vec<u16>,
    },
    /// The requested witness ancestor predates the independently acknowledged
    /// retained base. It cannot be recovered by retrying this signer; the host
    /// must use its independent checkpoint/anchor evidence.
    #[error(
        "state witness history pruned: requested generation [{requested_generation}] precedes retained base [{witness_base_generation}]"
    )]
    HistoryPruned {
        requested_generation: u64,
        witness_base_generation: u64,
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
            Self::CryptographicRefreshNotSupported { .. } => "cryptographic_refresh_not_supported",
            Self::SessionConflict { .. } => "session_conflict",
            Self::SessionFinalized { .. } => "session_finalized",
            Self::SessionNotFound { .. } => "session_not_found",
            Self::DkgNotReady { .. } => "dkg_not_ready",
            Self::SignRoundNotStarted { .. } => "sign_round_not_started",
            Self::ConsumedNonceReplay { .. } => "consumed_nonce_replay",
            Self::InteractiveAttemptAlreadyAggregated { .. } => {
                "interactive_attempt_already_aggregated"
            }
            Self::AggregateShareVerificationFailed { .. } => "aggregate_share_verification_failed",
            Self::HistoryPruned { .. } => "history_pruned",
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
            Self::CryptographicRefreshNotSupported { .. } => "terminal",
            Self::SessionConflict { .. } => "recoverable",
            Self::DkgNotReady { .. } => "recoverable",
            Self::SignRoundNotStarted { .. } => "recoverable",
            // ConsumedNonceReplay is recoverable in the sense that a fresh
            // attempt with a new identifier can be started. It cannot be
            // retried with the same identifier — the consumer (keep-core)
            // treats it as a signal to mint a new attempt_id rather than
            // retransmit.
            Self::ConsumedNonceReplay { .. } => "recoverable",
            // The aggregate is deterministic over public data and the attempt
            // is durably marked complete; a re-aggregation request is a benign
            // duplicate the caller should not retry, not an engine fault.
            Self::InteractiveAttemptAlreadyAggregated { .. } => "recoverable",
            // A fresh attempt that excludes the candidate culprits can still
            // produce a signature, so this is recoverable: the caller mints a
            // new attempt after the Go host adjudicates blame.
            Self::AggregateShareVerificationFailed { .. } => "recoverable",
            Self::HistoryPruned { .. } => "terminal",
            Self::SessionFinalized { .. } => "terminal",
            Self::SessionNotFound { .. } => "terminal",
            Self::Internal(_) => "terminal",
        }
    }

    /// The CANDIDATE culprits carried by this error. Non-empty only for
    /// `AggregateShareVerificationFailed`; empty for every other variant. The
    /// FFI layer uses this to surface the list to the Go host without matching
    /// the variant inline.
    pub fn candidate_culprits(&self) -> &[u16] {
        match self {
            Self::AggregateShareVerificationFailed {
                candidate_culprits, ..
            } => candidate_culprits,
            _ => &[],
        }
    }
}

#[cfg(test)]
mod tests {
    use super::EngineError;

    #[test]
    fn interactive_attempt_already_aggregated_has_stable_code_and_message_format() {
        let err = EngineError::InteractiveAttemptAlreadyAggregated {
            session_id: "session-a".to_string(),
            attempt_id: "attempt-1".to_string(),
        };
        assert_eq!(err.code(), "interactive_attempt_already_aggregated");
        assert_eq!(err.recovery_class(), "recoverable");
        assert_eq!(
            err.to_string(),
            "interactive attempt [attempt-1] already aggregated in session [session-a]",
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
        let unsupported_refresh = EngineError::CryptographicRefreshNotSupported {
            session_id: "session-refresh".to_string(),
        };
        assert_eq!(
            unsupported_refresh.code(),
            "cryptographic_refresh_not_supported"
        );
        assert_eq!(unsupported_refresh.recovery_class(), "terminal");
        assert!(unsupported_refresh
            .to_string()
            .contains("multi-round, zero-constant FROST refresh protocol"));
    }

    #[test]
    fn aggregate_share_verification_failed_code_message_and_culprits() {
        let err = EngineError::AggregateShareVerificationFailed {
            session_id: "session-a".to_string(),
            attempt_id: "attempt-1".to_string(),
            candidate_culprits: vec![2, 3],
        };
        assert_eq!(err.code(), "aggregate_share_verification_failed");
        assert_eq!(err.recovery_class(), "recoverable");
        // The count is rendered; the member identifiers travel in the structured
        // candidate_culprits list, not the message string.
        assert_eq!(
            err.to_string(),
            "InteractiveAggregate: 2 signature share(s) failed verification for attempt [attempt-1] in session [session-a]",
        );
        assert_eq!(err.candidate_culprits(), &[2, 3]);

        // Every non-aggregate error exposes no culprits.
        assert!(EngineError::Validation("x".to_string())
            .candidate_culprits()
            .is_empty());
    }
}
