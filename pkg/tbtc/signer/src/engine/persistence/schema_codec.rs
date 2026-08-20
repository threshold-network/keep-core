/// Persisted schema codec: pure encode/decode and `TryFrom` projection,
/// plus schema-version validation. Moved from `persistence.rs` as part of the
/// C2 persistence-deepening refactor (see
/// `docs/specs/frost-signer-persistence-deepening.md`).
use super::*;

#[derive(Clone, Deserialize, Serialize)]
pub(crate) struct PersistedKeyPackage {
    pub(crate) identifier: u16,
    pub(crate) key_package_hex: SecretString,
}

// Hand-written Debug: `SecretString` is `Zeroizing<String>`, whose
// derived Debug prints the inner string verbatim. `key_package_hex`
// holds serialized signing-share material, so it MUST be redacted -
// otherwise any `{:?}` of this struct (log line, panic, the derived
// Debug of an enclosing struct) spills a key share.
impl std::fmt::Debug for PersistedKeyPackage {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("PersistedKeyPackage")
            .field("identifier", &self.identifier)
            .field("key_package_hex", &"<redacted>")
            .finish()
    }
}

#[derive(Clone, Deserialize, Serialize)]
pub(crate) struct PersistedSessionState {
    pub(crate) dkg_request_fingerprint: Option<String>,
    pub(crate) dkg_key_packages: Option<Vec<PersistedKeyPackage>>,
    pub(crate) dkg_public_key_package_hex: Option<String>,
    pub(crate) dkg_result: Option<DkgResult>,
    /// DKG signing-policy firewall compatibility check. `#[serde(default)]`
    /// keeps state written before this field existed loadable (absent -> 0).
    #[serde(default)]
    pub(crate) policy_snapshot_version: u32,
    pub(crate) sign_request_fingerprint: Option<String>,
    pub(crate) sign_message_hex: Option<SecretString>,
    pub(crate) round_state: Option<RoundState>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) active_attempt_context: Option<AttemptContext>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) attempt_transition_records: Vec<TranscriptAuditRecord>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_attempt_ids: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_sign_round_ids: Vec<String>,
    pub(crate) finalize_request_fingerprint: Option<String>,
    pub(crate) signature_result: Option<SignatureResult>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_finalize_round_ids: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_finalize_request_fingerprints: Vec<String>,
    pub(crate) build_tx_request_fingerprint: Option<String>,
    pub(crate) tx_result: Option<TransactionResult>,
    pub(crate) refresh_request_fingerprint: Option<String>,
    pub(crate) refresh_result: Option<RefreshSharesResult>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) refresh_history: Vec<RefreshHistoryRecord>,
    #[serde(default)]
    pub(crate) refresh_count: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) emergency_rekey_event: Option<EmergencyRekeyEvent>,
    // Phase 7.1 interactive consumption markers - the ONLY durable
    // artifact of interactive sessions (markers-only durability: live
    // interactive state, including nonces, never persists).
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) consumed_interactive_attempt_markers: Vec<String>,
    // Phase 7.2b InteractiveAggregate completion markers (see SessionState).
    // serde(default) keeps state written before 7.2b loadable: an absent field
    // deserializes to an empty set.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) aggregated_interactive_attempt_markers: Vec<String>,
    // The wallet key_group a per-signing (cross-session) attempt is bound to. Durable
    // ALONGSIDE the interactive markers: for a distributed-DKG wallet the signing
    // session has no dkg_result, so this is the ONLY link back to the wallet DKG. It
    // must survive a restart between Round2 (shares consumed, markers written) and
    // InteractiveAggregate, or Aggregate/verify_share would resolve neither dkg_result
    // nor bound_key_group and return DkgNotReady, stranding the collected shares. Public
    // (a key group id), not secret. serde(default) keeps pre-existing state loadable.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) bound_key_group: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) retired_interactive_at_unix: Option<u64>,
    // Fixed-size exact Aggregate authorizations and successful-package replay
    // identities (see SessionState).
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) authorized_interactive_aggregate_markers: Vec<String>,
}

// Hand-written Debug: `sign_message_hex` is `SecretString`
// (`Zeroizing<String>`), whose derived Debug renders the inner value
// verbatim. It is redacted here (presence preserved, content hidden);
// `dkg_key_packages` redacts via PersistedKeyPackage's own Debug.
// NOTE: any future secret-bearing field MUST be redacted here too.
impl std::fmt::Debug for PersistedSessionState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("PersistedSessionState")
            .field("dkg_request_fingerprint", &self.dkg_request_fingerprint)
            .field("dkg_key_packages", &self.dkg_key_packages)
            .field(
                "dkg_public_key_package_hex",
                &self.dkg_public_key_package_hex,
            )
            .field("dkg_result", &self.dkg_result)
            .field("policy_snapshot_version", &self.policy_snapshot_version)
            .field("sign_request_fingerprint", &self.sign_request_fingerprint)
            .field(
                "sign_message_hex",
                &self.sign_message_hex.as_ref().map(|_| "<redacted>"),
            )
            .field("round_state", &self.round_state)
            .field("active_attempt_context", &self.active_attempt_context)
            .field(
                "attempt_transition_records",
                &self.attempt_transition_records,
            )
            .field("consumed_attempt_ids", &self.consumed_attempt_ids)
            .field("consumed_sign_round_ids", &self.consumed_sign_round_ids)
            .field(
                "finalize_request_fingerprint",
                &self.finalize_request_fingerprint,
            )
            .field("signature_result", &self.signature_result)
            .field(
                "consumed_finalize_round_ids",
                &self.consumed_finalize_round_ids,
            )
            .field(
                "consumed_finalize_request_fingerprints",
                &self.consumed_finalize_request_fingerprints,
            )
            .field(
                "build_tx_request_fingerprint",
                &self.build_tx_request_fingerprint,
            )
            .field("tx_result", &self.tx_result)
            .field(
                "refresh_request_fingerprint",
                &self.refresh_request_fingerprint,
            )
            .field("refresh_result", &self.refresh_result)
            .field("refresh_history", &self.refresh_history)
            .field("emergency_rekey_event", &self.emergency_rekey_event)
            .field(
                "consumed_interactive_attempt_markers",
                &self.consumed_interactive_attempt_markers,
            )
            .field(
                "aggregated_interactive_attempt_markers",
                &self.aggregated_interactive_attempt_markers,
            )
            .field("bound_key_group", &self.bound_key_group)
            .field(
                "retired_interactive_at_unix",
                &self.retired_interactive_at_unix,
            )
            .field(
                "authorized_interactive_aggregate_markers",
                &self.authorized_interactive_aggregate_markers,
            )
            .finish()
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]

pub(crate) struct PersistedEngineState {
    pub(crate) schema_version: u16,
    pub(crate) sessions: HashMap<String, PersistedSessionState>,
    pub(crate) refresh_epoch_counter: u64,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub(crate) operator_fault_scores: BTreeMap<u16, u64>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub(crate) quarantined_operator_identifiers: Vec<u16>,
    #[serde(default)]
    pub(crate) canary_rollout: CanaryRolloutState,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub(crate) struct PersistedEncryptedEngineStateEnvelope {
    pub(crate) schema_version: u16,
    pub(crate) encryption_algorithm: String,
    pub(crate) key_provider: String,
    pub(crate) key_id: String,
    pub(crate) nonce: String,
    pub(crate) ciphertext: String,
    pub(crate) authentication_tag: String,
}

pub(crate) enum PersistedStateStorageFormat {
    EncryptedEnvelope {
        persisted: PersistedEngineState,
        should_rewrite: bool,
    },
    LegacyPlaintext(PersistedEngineState),
}

pub(crate) const PERSISTED_STATE_SCHEMA_VERSION: u16 = 1;

pub(crate) const PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION_V2: u16 = 2;

pub(crate) const PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION: u16 = 3;

impl TryFrom<PersistedEngineState> for EngineState {
    type Error = EngineError;

    fn try_from(persisted: PersistedEngineState) -> Result<Self, Self::Error> {
        if persisted.schema_version != PERSISTED_STATE_SCHEMA_VERSION {
            return Err(EngineError::Internal(format!(
                "unsupported signer state schema version: expected [{}], got [{}]",
                PERSISTED_STATE_SCHEMA_VERSION, persisted.schema_version
            )));
        }

        let mut sessions = HashMap::new();
        let mut key_group_owners = HashMap::<String, String>::new();
        for (session_id, session_state) in persisted.sessions {
            let session_state: SessionState = session_state.try_into()?;
            if let Some(dkg_result) = session_state.dkg.result.as_ref() {
                if let Some(existing_owner) =
                    key_group_owners.insert(dkg_result.key_group.clone(), session_id.clone())
                {
                    return Err(EngineError::Internal(format!(
                        "duplicate persisted DKG key_group [{}] owned by sessions [{}] and [{}]",
                        dkg_result.key_group, existing_owner, session_id
                    )));
                }
            }
            sessions.insert(session_id, session_state);
        }
        // State written before the retirement tier existed restores no live
        // interactive nonces by construction. Classify its idle, bound
        // per-message entries into the retired tier during load so a full
        // legacy registry cannot remain permanently wedged after upgrade.
        let migration_retired_at = now_unix().max(1);
        for session in sessions.values_mut() {
            if session.capacity_pins.retired_interactive_at_unix.is_none()
                && session.interactive.interactive_signing.is_empty()
                && per_message_interactive_session(session)
            {
                session.capacity_pins.retired_interactive_at_unix = Some(migration_retired_at);
            }
        }
        let mut quarantined_operator_identifiers = HashSet::new();
        for operator_identifier in persisted.quarantined_operator_identifiers {
            if operator_identifier == 0 {
                return Err(EngineError::Internal(
                    "persisted quarantined operator identifier must be non-zero".to_string(),
                ));
            }
            if !quarantined_operator_identifiers.insert(operator_identifier) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted quarantined operator identifier [{}]",
                    operator_identifier
                )));
            }
        }
        for operator_identifier in persisted.operator_fault_scores.keys() {
            if *operator_identifier == 0 {
                return Err(EngineError::Internal(
                    "persisted operator fault score identifier must be non-zero".to_string(),
                ));
            }
        }
        let canary_rollout = persisted.canary_rollout;
        if !matches!(canary_rollout.current_percent, 10 | 50 | 100) {
            return Err(EngineError::Internal(format!(
                "persisted canary current_percent [{}] must be one of [10, 50, 100]",
                canary_rollout.current_percent
            )));
        }
        if !matches!(canary_rollout.previous_percent, 10 | 50 | 100) {
            return Err(EngineError::Internal(format!(
                "persisted canary previous_percent [{}] must be one of [10, 50, 100]",
                canary_rollout.previous_percent
            )));
        }
        if canary_rollout.config_version == 0 {
            return Err(EngineError::Internal(
                "persisted canary config_version must be positive".to_string(),
            ));
        }

        let mut engine_state = EngineState {
            sessions,
            refresh_epoch_counter: persisted.refresh_epoch_counter,
            operator_fault_scores: persisted.operator_fault_scores,
            quarantined_operator_identifiers,
            canary_rollout,
        };
        drop(compact_retired_per_message_sessions(
            &mut engine_state,
            None,
        ));
        ensure_session_registry_persisted_bound(&engine_state.sessions)?;
        Ok(engine_state)
    }
}

impl TryFrom<&EngineState> for PersistedEngineState {
    type Error = EngineError;

    fn try_from(engine_state: &EngineState) -> Result<Self, Self::Error> {
        ensure_session_registry_persisted_bound(&engine_state.sessions)?;
        let mut sessions = HashMap::new();
        for (session_id, session_state) in &engine_state.sessions {
            sessions.insert(session_id.clone(), session_state.try_into()?);
        }
        let mut quarantined_operator_identifiers = engine_state
            .quarantined_operator_identifiers
            .iter()
            .copied()
            .collect::<Vec<_>>();
        quarantined_operator_identifiers.sort_unstable();

        Ok(PersistedEngineState {
            schema_version: PERSISTED_STATE_SCHEMA_VERSION,
            sessions,
            refresh_epoch_counter: engine_state.refresh_epoch_counter,
            operator_fault_scores: engine_state.operator_fault_scores.clone(),
            quarantined_operator_identifiers,
            canary_rollout: engine_state.canary_rollout.clone(),
        })
    }
}

impl TryFrom<PersistedSessionState> for SessionState {
    type Error = EngineError;

    fn try_from(persisted: PersistedSessionState) -> Result<Self, Self::Error> {
        let dkg_key_packages = persisted
            .dkg_key_packages
            .map(|persisted_key_packages| {
                let mut key_packages = BTreeMap::new();

                for persisted_key_package in persisted_key_packages {
                    let identifier = persisted_key_package.identifier;
                    if identifier == 0 {
                        return Err(EngineError::Internal(
                            "persisted key package identifier must be non-zero".to_string(),
                        ));
                    }

                    let key_package_bytes_result =
                        hex::decode(persisted_key_package.key_package_hex.as_str());
                    let mut key_package_bytes = key_package_bytes_result.map_err(|_| {
                        EngineError::Internal(format!(
                            "failed to decode persisted key package for identifier [{}]",
                            identifier
                        ))
                    })?;
                    let key_package_result =
                        frost::keys::KeyPackage::deserialize(&key_package_bytes);
                    key_package_bytes.zeroize();
                    let key_package = key_package_result.map_err(|e| {
                        EngineError::Internal(format!(
                            "failed to deserialize persisted key package for identifier [{}]: {e}",
                            identifier
                        ))
                    })?;

                    if key_packages.insert(identifier, key_package).is_some() {
                        return Err(EngineError::Internal(format!(
                            "duplicate persisted key package identifier [{}]",
                            identifier
                        )));
                    }
                }

                Ok(key_packages)
            })
            .transpose()?;

        let dkg_public_key_package = persisted
            .dkg_public_key_package_hex
            .map(|mut public_key_package_hex| {
                let public_key_package_bytes_result = hex::decode(&public_key_package_hex);
                public_key_package_hex.zeroize();
                let mut public_key_package_bytes =
                    public_key_package_bytes_result.map_err(|_| {
                        EngineError::Internal(
                            "failed to decode persisted DKG public key package".to_string(),
                        )
                    })?;
                let public_key_package_result =
                    frost::keys::PublicKeyPackage::deserialize(&public_key_package_bytes);
                public_key_package_bytes.zeroize();
                public_key_package_result.map_err(|e| {
                    EngineError::Internal(format!(
                        "failed to deserialize persisted DKG public key package: {e}"
                    ))
                })
            })
            .transpose()?;

        let sign_message_bytes = persisted
            .sign_message_hex
            .map(|message_hex| {
                let mut sign_message_bytes = hex::decode(message_hex.as_str()).map_err(|_| {
                    EngineError::Internal("failed to decode persisted sign message".to_string())
                })?;
                let secret = Zeroizing::new(std::mem::take(&mut sign_message_bytes));
                sign_message_bytes.zeroize();
                Ok(secret)
            })
            .transpose()?;

        let mut consumed_attempt_ids = HashSet::new();
        for attempt_id in persisted.consumed_attempt_ids {
            if attempt_id.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed attempt ID must be non-empty".to_string(),
                ));
            }

            if !consumed_attempt_ids.insert(attempt_id.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed attempt ID [{}]",
                    attempt_id
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_attempt_ids.len(),
            "consumed_attempt_ids",
        )?;

        let mut consumed_sign_round_ids = HashSet::new();
        for round_id in persisted.consumed_sign_round_ids {
            if round_id.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed sign round ID must be non-empty".to_string(),
                ));
            }

            if !consumed_sign_round_ids.insert(round_id.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed sign round ID [{}]",
                    round_id
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_sign_round_ids.len(),
            "consumed_sign_round_ids",
        )?;

        let mut consumed_finalize_round_ids = HashSet::new();
        for round_id in persisted.consumed_finalize_round_ids {
            if round_id.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed finalize round ID must be non-empty".to_string(),
                ));
            }

            if !consumed_finalize_round_ids.insert(round_id.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed finalize round ID [{}]",
                    round_id
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_finalize_round_ids.len(),
            "consumed_finalize_round_ids",
        )?;

        let mut consumed_finalize_request_fingerprints = HashSet::new();
        for request_fingerprint in persisted.consumed_finalize_request_fingerprints {
            if request_fingerprint.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed finalize request fingerprint must be non-empty".to_string(),
                ));
            }

            if !consumed_finalize_request_fingerprints.insert(request_fingerprint.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed finalize request fingerprint [{}]",
                    request_fingerprint
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_finalize_request_fingerprints.len(),
            "consumed_finalize_request_fingerprints",
        )?;

        let mut consumed_interactive_attempt_markers = HashSet::new();
        for attempt_marker in persisted.consumed_interactive_attempt_markers {
            if attempt_marker.is_empty() {
                return Err(EngineError::Internal(
                    "persisted consumed interactive attempt marker must be non-empty".to_string(),
                ));
            }

            if !consumed_interactive_attempt_markers.insert(attempt_marker.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted consumed interactive attempt marker [{}]",
                    attempt_marker
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            consumed_interactive_attempt_markers.len(),
            "consumed_interactive_attempt_markers",
        )?;

        let mut authorized_interactive_aggregate_markers = HashSet::new();
        for authorization_marker in persisted.authorized_interactive_aggregate_markers {
            let canonical_sha256 = authorization_marker.len() == 64
                && authorization_marker
                    .bytes()
                    .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte));
            if !canonical_sha256 {
                return Err(EngineError::Internal(
                    "persisted interactive Aggregate authorization marker must be canonical 64-character lowercase hex"
                        .to_string(),
                ));
            }
            if !authorized_interactive_aggregate_markers.insert(authorization_marker.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted interactive Aggregate authorization marker [{authorization_marker}]"
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            authorized_interactive_aggregate_markers.len(),
            "authorized_interactive_aggregate_markers",
        )?;

        let mut aggregated_interactive_attempt_markers = HashSet::new();
        for attempt_marker in persisted.aggregated_interactive_attempt_markers {
            if attempt_marker.is_empty() {
                return Err(EngineError::Internal(
                    "persisted aggregated interactive attempt marker must be non-empty".to_string(),
                ));
            }

            if !aggregated_interactive_attempt_markers.insert(attempt_marker.clone()) {
                return Err(EngineError::Internal(format!(
                    "duplicate persisted aggregated interactive attempt marker [{}]",
                    attempt_marker
                )));
            }
        }
        ensure_consumed_registry_persisted_bound(
            aggregated_interactive_attempt_markers.len(),
            "aggregated_interactive_attempt_markers",
        )?;
        if persisted.attempt_transition_records.len()
            > TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION
        {
            return Err(EngineError::Internal(format!(
                "persisted attempt_transition_records size [{}] exceeds max [{}]",
                persisted.attempt_transition_records.len(),
                TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION
            )));
        }
        let mut last_refresh_epoch = 0_u64;
        for refresh_record in &persisted.refresh_history {
            if refresh_record.refresh_epoch == 0 {
                return Err(EngineError::Internal(
                    "persisted refresh_history refresh_epoch must be positive".to_string(),
                ));
            }
            if refresh_record.refresh_epoch <= last_refresh_epoch {
                return Err(EngineError::Internal(
                    "persisted refresh_history refresh_epoch must be strictly increasing"
                        .to_string(),
                ));
            }
            last_refresh_epoch = refresh_record.refresh_epoch;
        }
        if let Some(emergency_rekey_event) = persisted.emergency_rekey_event.as_ref() {
            if emergency_rekey_event.reason.trim().is_empty() {
                return Err(EngineError::Internal(
                    "persisted emergency_rekey_event reason must be non-empty".to_string(),
                ));
            }
        }

        if persisted.retired_interactive_at_unix == Some(0) {
            return Err(EngineError::Internal(
                "persisted retired_interactive_at_unix must be positive".to_string(),
            ));
        }

        let session = SessionState {
            dkg: DkgSessionState {
                request_fingerprint: persisted.dkg_request_fingerprint,
                key_packages: dkg_key_packages,
                public_key_package: dkg_public_key_package,
                result: persisted.dkg_result,
                // Persisted schema carries this field as a u32 counter (serde-defaulted
                // on PersistedSessionState so state written before the field existed
                // still loads as 0). Round-tripped through this TryFrom - it has no
                // production reader today but persistence is already wired.
                policy_snapshot_version: persisted.policy_snapshot_version,
            },
            signing: LegacySigningSessionState {
                request_fingerprint: persisted.sign_request_fingerprint,
                message_bytes: sign_message_bytes,
                round_state: persisted.round_state,
                active_attempt_context: persisted.active_attempt_context,
                finalize_request_fingerprint: persisted.finalize_request_fingerprint,
                signature_result: persisted.signature_result,
                build_tx_request_fingerprint: persisted.build_tx_request_fingerprint,
                tx_result: persisted.tx_result,
                consumed_attempt_ids,
                consumed_sign_round_ids,
                consumed_finalize_round_ids,
                consumed_finalize_request_fingerprints,
            },
            interactive: InteractiveSessionState {
                // Live interactive state never restores: nonces are gone by
                // construction after a restart, so the attempt fails safe and
                // only the consumption markers survive. Empty map (no live members).
                interactive_signing: BTreeMap::new(),
                // Restore the wallet binding: for a cross-session signing session it
                // is the only durable link to the wallet DKG, needed so an
                // InteractiveAggregate that runs after a restart (past a member's
                // Round2) can still resolve the wallet by key_group. Public data;
                // survives with the consumed/aggregate markers.
                bound_key_group: persisted.bound_key_group,
                consumed_attempt_markers: consumed_interactive_attempt_markers,
                authorized_aggregate_markers: authorized_interactive_aggregate_markers,
                aggregated_attempt_markers: aggregated_interactive_attempt_markers,
            },
            audit: AuditTrail(persisted.attempt_transition_records),
            lifecycle: LifecycleState {
                refresh_request_fingerprint: persisted.refresh_request_fingerprint,
                refresh_result: persisted.refresh_result,
                // Preserve the legacy synthetic count losslessly for schema
                // compatibility and diagnostics. Lifecycle status deliberately
                // ignores it until a versioned cryptographic refresh protocol
                // exists. Computed before refresh_history is moved below.
                refresh_count: persisted
                    .refresh_count
                    .max(persisted.refresh_history.len() as u64),
                refresh_history: persisted.refresh_history,
                emergency_rekey_event: persisted.emergency_rekey_event,
            },
            capacity_pins: OperationalState {
                // Transient: never written into the persisted schema.
                heartbeat_rate_limiter: PolicyRateLimiterState::default(),
                // Persisted: round-trips through this TryFrom.
                retired_interactive_at_unix: persisted.retired_interactive_at_unix,
                // Transient: never persisted; no operation can remain in flight
                // across a restart.
                aggregate_eviction_pin: Arc::new(()),
            },
        };
        if session.capacity_pins.retired_interactive_at_unix.is_some()
            && !per_message_interactive_session(&session)
        {
            return Err(EngineError::Internal(
                "persisted retired interactive session must have the per-message role".to_string(),
            ));
        }
        Ok(session)
    }
}

impl TryFrom<&SessionState> for PersistedSessionState {
    type Error = EngineError;

    fn try_from(session_state: &SessionState) -> Result<Self, Self::Error> {
        let dkg_key_packages = session_state
            .dkg
            .key_packages
            .as_ref()
            .map(|key_packages| {
                key_packages
                    .iter()
                    .map(|(identifier, key_package)| {
                        let mut key_package_bytes = key_package.serialize().map_err(|e| {
                            EngineError::Internal(format!(
                                "failed to serialize DKG key package for identifier [{}]: {e}",
                                identifier
                            ))
                        })?;
                        let key_package_hex = Zeroizing::new(hex::encode(&key_package_bytes));
                        key_package_bytes.zeroize();
                        Ok(PersistedKeyPackage {
                            identifier: *identifier,
                            key_package_hex,
                        })
                    })
                    .collect::<Result<Vec<_>, _>>()
            })
            .transpose()?;

        let dkg_public_key_package_hex = session_state
            .dkg
            .public_key_package
            .as_ref()
            .map(|public_key_package| {
                let mut public_key_package_bytes = public_key_package.serialize().map_err(|e| {
                    EngineError::Internal(format!(
                        "failed to serialize DKG public key package: {e}"
                    ))
                })?;
                let public_key_package_hex = hex::encode(&public_key_package_bytes);
                public_key_package_bytes.zeroize();
                Ok(public_key_package_hex)
            })
            .transpose()?;

        let sign_message_hex = session_state
            .signing
            .message_bytes
            .as_ref()
            .map(|sign_message_bytes| Zeroizing::new(hex::encode(sign_message_bytes.as_slice())));
        ensure_consumed_registry_persisted_bound(
            session_state.signing.consumed_attempt_ids.len(),
            "consumed_attempt_ids",
        )?;
        ensure_consumed_registry_persisted_bound(
            session_state.signing.consumed_sign_round_ids.len(),
            "consumed_sign_round_ids",
        )?;
        ensure_consumed_registry_persisted_bound(
            session_state.signing.consumed_finalize_round_ids.len(),
            "consumed_finalize_round_ids",
        )?;
        ensure_consumed_registry_persisted_bound(
            session_state
                .signing
                .consumed_finalize_request_fingerprints
                .len(),
            "consumed_finalize_request_fingerprints",
        )?;
        ensure_consumed_registry_persisted_bound(
            session_state.interactive.consumed_attempt_markers.len(),
            "consumed_interactive_attempt_markers",
        )?;
        ensure_consumed_registry_persisted_bound(
            session_state.interactive.authorized_aggregate_markers.len(),
            "authorized_interactive_aggregate_markers",
        )?;
        if session_state.audit.0.len() > TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION {
            return Err(EngineError::Internal(format!(
                "attempt_transition_records size [{}] exceeds max [{}]",
                session_state.audit.0.len(),
                TBTC_SIGNER_MAX_ATTEMPT_TRANSITION_RECORDS_PER_SESSION
            )));
        }
        let mut consumed_attempt_ids = session_state
            .signing
            .consumed_attempt_ids
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_attempt_ids.sort_unstable();
        let mut consumed_sign_round_ids = session_state
            .signing
            .consumed_sign_round_ids
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_sign_round_ids.sort_unstable();
        let mut consumed_finalize_round_ids = session_state
            .signing
            .consumed_finalize_round_ids
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_finalize_round_ids.sort_unstable();
        let mut consumed_finalize_request_fingerprints = session_state
            .signing
            .consumed_finalize_request_fingerprints
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_finalize_request_fingerprints.sort_unstable();
        let mut consumed_interactive_attempt_markers = session_state
            .interactive
            .consumed_attempt_markers
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        consumed_interactive_attempt_markers.sort_unstable();
        let mut aggregated_interactive_attempt_markers = session_state
            .interactive
            .aggregated_attempt_markers
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        aggregated_interactive_attempt_markers.sort_unstable();
        let mut authorized_interactive_aggregate_markers = session_state
            .interactive
            .authorized_aggregate_markers
            .iter()
            .cloned()
            .collect::<Vec<_>>();
        authorized_interactive_aggregate_markers.sort_unstable();

        Ok(PersistedSessionState {
            dkg_request_fingerprint: session_state.dkg.request_fingerprint.clone(),
            dkg_key_packages,
            dkg_public_key_package_hex,
            dkg_result: session_state.dkg.result.clone(),
            sign_request_fingerprint: session_state.signing.request_fingerprint.clone(),
            sign_message_hex,
            round_state: session_state.signing.round_state.clone(),
            active_attempt_context: session_state.signing.active_attempt_context.clone(),
            attempt_transition_records: session_state.audit.0.clone(),
            consumed_attempt_ids,
            consumed_sign_round_ids,
            finalize_request_fingerprint: session_state
                .signing
                .finalize_request_fingerprint
                .clone(),
            signature_result: session_state.signing.signature_result.clone(),
            consumed_finalize_round_ids,
            consumed_finalize_request_fingerprints,
            build_tx_request_fingerprint: session_state
                .signing
                .build_tx_request_fingerprint
                .clone(),
            tx_result: session_state.signing.tx_result.clone(),
            refresh_request_fingerprint: session_state
                .lifecycle
                .refresh_request_fingerprint
                .clone(),
            refresh_result: session_state.lifecycle.refresh_result.clone(),
            refresh_history: session_state.lifecycle.refresh_history.clone(),
            refresh_count: session_state.lifecycle.refresh_count,
            emergency_rekey_event: session_state.lifecycle.emergency_rekey_event.clone(),
            consumed_interactive_attempt_markers,
            aggregated_interactive_attempt_markers,
            bound_key_group: session_state.interactive.bound_key_group.clone(),
            retired_interactive_at_unix: session_state.capacity_pins.retired_interactive_at_unix,
            authorized_interactive_aggregate_markers,
            policy_snapshot_version: session_state.dkg.policy_snapshot_version,
        })
    }
}
