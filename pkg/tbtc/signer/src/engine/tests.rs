// Kept as a single module on purpose: scripts/run_phase5_chaos_suite.sh pins
// `engine::tests::<name>` paths via `cargo test -- --exact`, and the phase
// docs reference them; splitting this file would break those contracts.

use super::*;
use proptest::prelude::*;
use serde::Deserialize;
#[cfg(unix)]
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
#[cfg(unix)]
use std::{
    process::Command,
    thread,
    time::{Duration, Instant},
};

// Test-only reimplementations of the removed stateless FROST primitives.
//
// The transitional coarse-FROST signing path (run_dkg dealer, start/finalize
// sign round, and these stateless generate/sign/aggregate ops) was deleted from
// the engine, API, and FFI surfaces. A handful of preserved tests still need a
// co-signer ("member 2") to sign through raw FROST while the hardened session
// API drives member 1, proving the two custody models interoperate. These
// helpers drive the frost crate directly (frost::round1::commit,
// frost::round2::sign, frost::aggregate) instead of the deleted engine ops, so
// coverage of the go-forward primitives is unchanged. They live here, gated to
// tests, and never re-enter the production/FFI surface.

#[derive(Clone, Debug)]
struct GenerateNoncesAndCommitmentsRequest {
    key_package_identifier: String,
    key_package_hex: SecretHex,
}

#[derive(Clone, Debug)]
struct GenerateNoncesAndCommitmentsResult {
    nonces_hex: String,
    commitment: NativeFrostCommitment,
}

#[derive(Clone, Debug)]
struct SignShareRequest {
    signing_package_hex: String,
    nonces_hex: String,
    key_package_identifier: String,
    key_package_hex: SecretHex,
}

#[derive(Clone, Debug)]
struct SignShareResult {
    signature_share: NativeFrostSignatureShare,
}

#[derive(Clone, Debug)]
struct AggregateRequest {
    signing_package_hex: String,
    signature_shares: Vec<NativeFrostSignatureShare>,
    public_key_package: NativeFrostPublicKeyPackage,
}

#[derive(Clone, Debug)]
struct AggregateResult {
    signature_hex: String,
}

fn generate_nonces_and_commitments(
    mut request: GenerateNoncesAndCommitmentsRequest,
) -> Result<GenerateNoncesAndCommitmentsResult, EngineError> {
    let key_package_hex = std::mem::take(&mut request.key_package_hex);
    enforce_provenance_gate()?;

    let key_package = decode_key_package(
        "GenerateNoncesAndCommitments",
        &request.key_package_identifier,
        key_package_hex.expose_secret(),
    )?;
    let mut rng = zeroizing_rng_from_os();
    let (mut nonces, commitments) = frost::round1::commit(key_package.signing_share(), &mut rng);
    let commitment_bytes = match commitments.serialize() {
        Ok(commitment_bytes) => commitment_bytes,
        Err(err) => {
            nonces.zeroize();
            return Err(EngineError::Internal(format!(
                "failed to serialize signing commitments: {err}"
            )));
        }
    };
    let nonces_bytes_result = nonces.serialize();
    nonces.zeroize();
    let mut nonces_bytes = nonces_bytes_result
        .map_err(|e| EngineError::Internal(format!("failed to serialize signing nonces: {e}")))?;

    let result = GenerateNoncesAndCommitmentsResult {
        nonces_hex: hex::encode(&nonces_bytes),
        commitment: NativeFrostCommitment {
            identifier: frost_identifier_to_go_string(*key_package.identifier()),
            data_hex: hex::encode(commitment_bytes),
        },
    };
    nonces_bytes.zeroize();

    Ok(result)
}

fn sign_share(mut request: SignShareRequest) -> Result<SignShareResult, EngineError> {
    let nonces_hex = Zeroizing::new(std::mem::take(&mut request.nonces_hex));
    let key_package_hex = std::mem::take(&mut request.key_package_hex);
    enforce_provenance_gate()?;

    let signing_package_bytes = decode_hex_field(
        "SignShare",
        "signing_package_hex",
        &request.signing_package_hex,
    )?;
    let signing_package = frost::SigningPackage::deserialize(&signing_package_bytes)
        .map_err(|e| EngineError::Validation(format!("SignShare: invalid signing package: {e}")))?;

    let mut nonces_bytes = decode_hex_field("SignShare", "nonces_hex", &nonces_hex)?;
    let nonces_result = frost::round1::SigningNonces::deserialize(&nonces_bytes);
    nonces_bytes.zeroize();
    let mut nonces = nonces_result
        .map_err(|e| EngineError::Validation(format!("SignShare: invalid nonces: {e}")))?;

    let key_package_result = decode_key_package(
        "SignShare",
        &request.key_package_identifier,
        key_package_hex.expose_secret(),
    );
    let key_package = match key_package_result {
        Ok(key_package) => key_package,
        Err(err) => {
            nonces.zeroize();
            return Err(err);
        }
    };
    let signature_share_result = frost::round2::sign(&signing_package, &nonces, &key_package);
    nonces.zeroize();
    let signature_share = signature_share_result
        .map_err(|e| EngineError::Validation(format!("SignShare failed: {e}")))?;
    let mut signature_share_bytes = signature_share.serialize();
    let result = SignShareResult {
        signature_share: NativeFrostSignatureShare {
            identifier: frost_identifier_to_go_string(*key_package.identifier()),
            data_hex: hex::encode(&signature_share_bytes),
        },
    };
    signature_share_bytes.zeroize();

    Ok(result)
}

fn aggregate(request: AggregateRequest) -> Result<AggregateResult, EngineError> {
    enforce_provenance_gate()?;

    let signing_package_bytes = decode_hex_field(
        "Aggregate",
        "signing_package_hex",
        &request.signing_package_hex,
    )?;
    let signing_package = frost::SigningPackage::deserialize(&signing_package_bytes)
        .map_err(|e| EngineError::Validation(format!("Aggregate: invalid signing package: {e}")))?;
    let signature_shares = decode_signature_share_map("Aggregate", &request.signature_shares)?;
    let public_key_package =
        native_public_key_package_to_frost("Aggregate", &request.public_key_package)?;
    let signature = frost::aggregate(&signing_package, &signature_shares, &public_key_package)
        .map_err(|e| EngineError::Validation(format!("Aggregate failed: {e}")))?;
    let signature_bytes = signature
        .serialize()
        .map_err(|e| EngineError::Internal(format!("failed to serialize aggregate: {e}")))?;

    Ok(AggregateResult {
        signature_hex: hex::encode(signature_bytes),
    })
}

#[derive(Deserialize)]
struct AttemptContextVectorDomains {
    included_participants_fingerprint: String,
    attempt_id: String,
}

#[derive(Deserialize)]
struct AttemptContextVector {
    id: String,
    session_id: String,
    message_digest_hex: String,
    attempt_number: u32,
    coordinator_identifier: u16,
    included_participants: Vec<u16>,
    expected_included_participants_fingerprint: String,
    expected_attempt_id: String,
}

#[derive(Deserialize)]
struct AttemptContextVectorSuite {
    schema_version: String,
    hash_domains: AttemptContextVectorDomains,
    vectors: Vec<AttemptContextVector>,
}

fn load_attempt_context_vector_suite() -> AttemptContextVectorSuite {
    let vectors_path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("test/vectors/roast-attempt-context-v1.json");
    let vector_bytes = std::fs::read(&vectors_path).unwrap_or_else(|err| {
        panic!(
            "failed to read attempt-context vector file [{}]: {err}",
            vectors_path.display()
        )
    });

    serde_json::from_slice(&vector_bytes).expect("attempt-context vectors decode")
}

#[derive(Deserialize)]
struct CoordinatorSeedVectorFile {
    #[allow(dead_code)]
    description: String,
    vectors: Vec<CoordinatorSeedVector>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct CoordinatorSeedVector {
    name: String,
    key_group: String,
    #[serde(rename = "sessionID")]
    session_id: String,
    message_digest_hex: String,
    included_members: Vec<u16>,
    attempt_number: u32,
    wire_attempt_number: u32,
    expected_shuffle_seed_int64: String,
    expected_coordinator: u16,
}

// Byte-identical copy of the canonical cross-language vector file
// generated from the Go implementation
// (pkg/frost/roast/testdata/coordinator_seed_vectors.json on the
// RFC-21 branch; regenerate there with ROAST_SEED_VECTORS_REGEN=1
// and re-copy). Pins the RFC-21 Annex A derivation end to end so a
// semantic change on either side fails that side's own suite
// instead of fracturing coordinator agreement in a mixed
// deployment.
#[test]
fn coordinator_seed_derivation_matches_cross_language_vectors() {
    let raw = include_str!("../../testdata/coordinator_seed_vectors.json");
    let file: CoordinatorSeedVectorFile =
        serde_json::from_str(raw).expect("coordinator seed vector file decodes");
    assert!(
        !file.vectors.is_empty(),
        "expected at least one coordinator seed vector"
    );

    let mut saw_negative_seed = false;
    for vector in &file.vectors {
        assert_eq!(
            vector.wire_attempt_number,
            vector.attempt_number + 1,
            "wire attempt number must be the 1-based encoding in vector [{}]",
            vector.name
        );

        // In production the engine receives the 32-byte signing
        // digest AS its raw message; the seed binds that padded
        // message directly. Treat the vector digest as the message
        // so this test exercises the exact production relationship.
        let message_bytes = hex::decode(&vector.message_digest_hex).expect("vector digest decodes");
        let vector_rfc21_digest =
            rfc21_message_digest(&message_bytes).expect("rfc21 message digest");
        assert_eq!(
            hex::encode(vector_rfc21_digest),
            vector.message_digest_hex.to_ascii_lowercase(),
            "32-byte vector digest must round-trip the rfc21 padding in [{}]",
            vector.name
        );
        let shuffle_seed =
            roast_attempt_shuffle_seed(&vector.key_group, &vector.session_id, &vector_rfc21_digest)
                .expect("shuffle seed derives");
        let expected_shuffle_seed: i64 = vector
            .expected_shuffle_seed_int64
            .parse()
            .expect("expected shuffle seed parses as i64");
        assert_eq!(
            shuffle_seed, expected_shuffle_seed,
            "shuffle seed mismatch in vector [{}]",
            vector.name
        );
        if expected_shuffle_seed < 0 {
            saw_negative_seed = true;
        }

        // The shuffle-source composition uses the RFC-21 0-based
        // attempt number, exactly as `validate_attempt_context`
        // composes it from the 1-based wire encoding.
        let coordinator = select_coordinator_identifier(
            &vector.included_members,
            shuffle_seed,
            vector.wire_attempt_number - 1,
        )
        .expect("coordinator selects");
        assert_eq!(
            coordinator, vector.expected_coordinator,
            "coordinator mismatch in vector [{}]",
            vector.name
        );

        // End to end: an attempt context carrying the wire-encoded
        // attempt number and the vector's coordinator passes the
        // engine's strict validation under the vector's key group.
        // The attempt_id is bound to the engine's SHA256 transcript
        // digest of the message, while the seed above bound the
        // padded message itself -- the two-digest split the Go layer
        // relies on.
        let engine_message_digest_hex = hash_hex(&message_bytes);
        let fingerprint = roast_included_participants_fingerprint_hex(&vector.included_members)
            .expect("fingerprint");
        let attempt_id = roast_attempt_id_hex(
            &vector.session_id,
            &engine_message_digest_hex,
            vector.wire_attempt_number,
            coordinator,
            &fingerprint,
        )
        .expect("attempt id");
        let attempt_context = AttemptContext {
            attempt_number: vector.wire_attempt_number,
            coordinator_identifier: coordinator,
            included_participants: vector.included_members.clone(),
            included_participants_fingerprint: fingerprint,
            attempt_id,
        };
        validate_attempt_context(
            &vector.session_id,
            &vector.key_group,
            &message_bytes,
            &engine_message_digest_hex,
            2,
            Some(&attempt_context),
            true,
        )
        .unwrap_or_else(|err| {
            panic!(
                "vector [{}] context failed engine validation: {err:?}",
                vector.name
            )
        });
    }

    assert!(
        saw_negative_seed,
        "vector file must pin at least one negative shuffle seed"
    );
}

struct InteractiveDkgFixture {
    pre_normalization_even_y: bool,
    part3_requests: BTreeMap<u16, DkgPart3Request>,
}

fn deterministic_interactive_dkg_fixture(seed: u8) -> InteractiveDkgFixture {
    let participant_ids = [1u16, 2, 3];
    let participant_identifiers: BTreeMap<u16, frost::Identifier> = participant_ids
        .iter()
        .copied()
        .map(|id| {
            (
                id,
                participant_identifier_to_frost_identifier(id).expect("participant identifier"),
            )
        })
        .collect();
    let participant_id_by_identifier_hex: BTreeMap<String, u16> = participant_identifiers
        .iter()
        .map(|(id, identifier)| (hex::encode(identifier.serialize()), *id))
        .collect();

    let mut part1_secrets = BTreeMap::new();
    let mut part1_packages = BTreeMap::new();
    for id in participant_ids {
        let mut rng_seed = [0u8; 32];
        rng_seed[0] = seed;
        rng_seed[1..3].copy_from_slice(&id.to_be_bytes());
        let rng = ZeroizingChaCha20Rng::from_seed(rng_seed);
        let (secret_package, package) = frost::keys::dkg::part1(
            participant_identifiers[&id],
            participant_ids.len() as u16,
            2,
            rng,
        )
        .expect("DKG part1");

        part1_secrets.insert(id, secret_package);
        part1_packages.insert(
            id,
            DkgRound1Package {
                identifier: frost_identifier_to_go_string(participant_identifiers[&id]),
                package_hex: hex::encode(package.serialize().expect("round1 package")),
            },
        );
    }

    let round1_packages_for = |recipient_id: u16| -> Vec<DkgRound1Package> {
        participant_ids
            .iter()
            .copied()
            .filter(|id| *id != recipient_id)
            .map(|id| part1_packages[&id].clone())
            .collect()
    };

    let mut part2_secrets = BTreeMap::new();
    let mut round2_packages_by_recipient: BTreeMap<u16, Vec<DkgRound2Package>> = BTreeMap::new();
    for sender_id in participant_ids {
        let round1_packages =
            decode_round1_package_map("TestDKGPart2", &round1_packages_for(sender_id))
                .expect("round1 package map");
        let (round2_secret, round2_packages) = frost::keys::dkg::part2(
            part1_secrets
                .remove(&sender_id)
                .expect("part1 secret package"),
            &round1_packages,
        )
        .expect("DKG part2");

        part2_secrets.insert(sender_id, round2_secret);
        for (recipient_identifier, package) in round2_packages {
            let recipient_id = participant_id_by_identifier_hex
                .get(&hex::encode(recipient_identifier.serialize()))
                .copied()
                .expect("recipient identifier mapping");
            round2_packages_by_recipient
                .entry(recipient_id)
                .or_default()
                .push(DkgRound2Package {
                    identifier: frost_identifier_to_go_string(recipient_identifier),
                    sender_identifier: Some(frost_identifier_to_go_string(
                        participant_identifiers[&sender_id],
                    )),
                    package_hex: hex::encode(package.serialize().expect("round2 package")).into(),
                });
        }
    }

    let first_participant = participant_ids[0];
    let round1_packages =
        decode_round1_package_map("TestDKGPart3", &round1_packages_for(first_participant))
            .expect("round1 package map");
    let round2_packages = decode_round2_package_map(
        "TestDKGPart3",
        &round2_packages_by_recipient[&first_participant],
        Some(participant_identifiers[&first_participant]),
    )
    .expect("round2 package map");
    let (_, pre_normalization_public_key_package) = frost::keys::dkg::part3(
        part2_secrets
            .get(&first_participant)
            .expect("round2 secret package"),
        &round1_packages,
        &round2_packages,
    )
    .expect("DKG part3");

    let mut part3_requests = BTreeMap::new();
    for id in participant_ids {
        let secret_package = part2_secrets.get(&id).expect("round2 secret package");
        let secret_package_bytes = secret_package.serialize().expect("round2 secret");
        part3_requests.insert(
            id,
            DkgPart3Request {
                secret_package_hex: hex::encode(secret_package_bytes).into(),
                round1_packages: round1_packages_for(id),
                round2_packages: round2_packages_by_recipient
                    .get(&id)
                    .expect("round2 packages")
                    .clone(),
            },
        );
    }

    InteractiveDkgFixture {
        pre_normalization_even_y: pre_normalization_public_key_package.has_even_y(),
        part3_requests,
    }
}

fn deterministic_odd_y_interactive_dkg_fixture() -> InteractiveDkgFixture {
    for seed in 0u8..=u8::MAX {
        let fixture = deterministic_interactive_dkg_fixture(seed);
        if !fixture.pre_normalization_even_y {
            return fixture;
        }
    }

    panic!("could not find deterministic odd-Y DKG fixture");
}

#[test]
fn dkg_part3_normalizes_odd_y_group_key_and_secret_shares() {
    let _guard = lock_test_state();
    reset_for_tests();

    let fixture = deterministic_odd_y_interactive_dkg_fixture();
    assert!(
        !fixture.pre_normalization_even_y,
        "fixture must exercise the odd-Y normalization branch"
    );

    let mut part3_results = BTreeMap::new();
    for (id, request) in fixture.part3_requests {
        let result = dkg_part3(request).expect("DKG part3");
        let expected_identifier =
            frost_identifier_to_go_string(participant_identifier_to_frost_identifier(id).unwrap());
        assert_eq!(result.key_package.identifier, expected_identifier);
        assert_eq!(result.public_key_package.verifying_key.len(), 64);
        part3_results.insert(id, result);
    }

    let exported_x_only_key = part3_results[&1].public_key_package.verifying_key.clone();
    for result in part3_results.values() {
        assert_eq!(result.public_key_package.verifying_key, exported_x_only_key);
        assert_eq!(
            result.public_key_package.verifying_shares,
            part3_results[&1].public_key_package.verifying_shares
        );
    }

    let signing_participants = [1u16, 2];
    let mut commitments = Vec::new();
    let mut nonces_by_participant = BTreeMap::new();
    for id in signing_participants {
        let result = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
            key_package_identifier: part3_results[&id].key_package.identifier.clone(),
            key_package_hex: part3_results[&id].key_package.data_hex.clone(),
        })
        .expect("generate nonces");
        commitments.push(result.commitment);
        nonces_by_participant.insert(id, result.nonces_hex);
    }

    let message = [0x42u8; 32];
    let signing_package = new_signing_package(NewSigningPackageRequest {
        message_hex: hex::encode(message),
        commitments,
    })
    .expect("new signing package");

    let mut signature_shares = Vec::new();
    for id in signing_participants {
        let result = sign_share(SignShareRequest {
            signing_package_hex: signing_package.signing_package_hex.clone(),
            nonces_hex: nonces_by_participant
                .remove(&id)
                .expect("participant nonces"),
            key_package_identifier: part3_results[&id].key_package.identifier.clone(),
            key_package_hex: part3_results[&id].key_package.data_hex.clone(),
        })
        .expect("sign share");
        signature_shares.push(result.signature_share);
    }

    let aggregate = aggregate(AggregateRequest {
        signing_package_hex: signing_package.signing_package_hex,
        signature_shares,
        public_key_package: part3_results[&1].public_key_package.clone(),
    })
    .expect("aggregate");

    let signature_bytes = hex::decode(aggregate.signature_hex).expect("signature hex");
    let signature = SchnorrSignature::from_slice(&signature_bytes).expect("BIP340 signature");
    let public_key_bytes = hex::decode(exported_x_only_key).expect("verifying key hex");
    let public_key = XOnlyPublicKey::from_slice(&public_key_bytes).expect("x-only public key");
    let message = SecpMessage::from_digest(message);
    Secp256k1::verification_only()
        .verify_schnorr(&signature, &message, &public_key)
        .expect("aggregate verifies under normalized x-only key");
}

fn configure_test_state_path(suffix: &str) -> PathBuf {
    let path = std::env::temp_dir().join(format!(
        "frost_tbtc_engine_state_{suffix}_{}.json",
        std::process::id()
    ));
    clear_state_storage_policy_overrides();
    cleanup_test_state_artifacts(&path);
    std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &path);
    path
}

fn clear_state_storage_policy_overrides() {
    std::env::remove_var(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV);
    std::env::remove_var(TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT_ENV);
    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    std::env::remove_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV);
    std::env::remove_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV);
    std::env::remove_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV);
    std::env::remove_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV);
    std::env::remove_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV);
    std::env::remove_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV);
    std::env::remove_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV);
    std::env::remove_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV);
    std::env::remove_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV);
    std::env::remove_var(TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS_ENV);
    std::env::remove_var(TBTC_SIGNER_ADMISSION_MIN_THRESHOLD_ENV);
    std::env::remove_var(TBTC_SIGNER_ADMISSION_REQUIRED_IDENTIFIERS_ENV);
    std::env::remove_var(TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV);
    std::env::remove_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV);
    std::env::remove_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV);
    std::env::remove_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND1_P95_MS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND2_P95_MS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_INTERACTIVE_AGGREGATE_P95_MS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MIN_POLICY_SAMPLES_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_SAMPLE_AGE_SECONDS_ENV);
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV);
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV);
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_DEVELOPMENT);
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
        TEST_STATE_ENCRYPTION_KEY_HEX,
    );
}

fn configure_required_signing_policy_limits_for_tests() {
    std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV, "64");
    std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV, "100000000");
    std::env::set_var(
        TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV,
        "2100000000000000",
    );
}

fn build_signed_provenance_attestation(
    status: &str,
    runtime_version: &str,
    expires_at_unix: Option<u64>,
) -> (String, String, String) {
    let mut payload = serde_json::json!({
        "status": status,
        "runtime_version": runtime_version,
    });
    if let Some(expires_at_unix) = expires_at_unix {
        payload["expires_at_unix"] = serde_json::json!(expires_at_unix);
    }
    let payload = payload.to_string();

    let secp = Secp256k1::new();
    let secret_key = bitcoin::secp256k1::SecretKey::from_slice(&[0x11; 32]).expect("secret key");
    let keypair = bitcoin::secp256k1::Keypair::from_secret_key(&secp, &secret_key);
    let (trust_root_pubkey, _) = XOnlyPublicKey::from_keypair(&keypair);

    let payload_digest = Sha256::digest(payload.as_bytes());
    let message = SecpMessage::from_digest_slice(&payload_digest).expect("message digest");
    let signature = secp.sign_schnorr_no_aux_rand(&message, &keypair);

    (
        trust_root_pubkey.to_string(),
        payload,
        signature.to_string(),
    )
}

fn configure_valid_provenance_attestation_for_tests() {
    let (trust_root, attestation_payload, attestation_signature) =
        build_signed_provenance_attestation(
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
            TBTC_SIGNER_RUNTIME_VERSION,
            Some(now_unix() + 3600),
        );

    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
    );
    std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, trust_root);
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
        attestation_payload,
    );
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
        attestation_signature,
    );
    std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");
}

fn cleanup_test_state_artifacts(path: &Path) {
    let _ = std::fs::remove_file(path);
    let _ = std::fs::remove_file(state_lock_file_path(path));
    let _ = std::fs::remove_file(path.with_extension(format!("tmp-{}", std::process::id())));

    if let Ok(backups) = sorted_corrupted_state_backups(path) {
        for backup in backups {
            let _ = std::fs::remove_file(backup);
        }
    }
}

fn persisted_session_state_fixture() -> PersistedSessionState {
    PersistedSessionState {
        dkg_request_fingerprint: None,
        dkg_key_packages: None,
        dkg_public_key_package_hex: None,
        dkg_result: None,
        sign_request_fingerprint: None,
        sign_message_hex: None,
        round_state: None,
        active_attempt_context: None,
        attempt_transition_records: vec![],
        consumed_attempt_ids: vec![],
        consumed_sign_round_ids: vec![],
        finalize_request_fingerprint: None,
        signature_result: None,
        consumed_finalize_round_ids: vec![],
        consumed_finalize_request_fingerprints: vec![],
        build_tx_request_fingerprint: None,
        tx_result: None,
        refresh_request_fingerprint: None,
        refresh_result: None,
        refresh_history: vec![],
        refresh_count: 0,
        emergency_rekey_event: None,
        consumed_interactive_attempt_markers: vec![],
        aggregated_interactive_attempt_markers: vec![],
        bound_key_group: None,
        retired_interactive_at_unix: None,
        authorized_interactive_aggregate_markers: vec![],
        policy_snapshot_version: 0,
    }
}

fn expect_internal_error_contains(err: EngineError, expected_substring: &str) {
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains(expected_substring),
        "unexpected internal error message: {message}"
    );
}

fn persist_state_for_key_provider_test(session_id: &str) -> Result<(), EngineError> {
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    guard
        .sessions
        .entry(session_id.to_string())
        .or_default()
        .interactive
        .bound_key_group = Some("state-key-provider-test".to_string());
    persist_engine_state_to_storage(&guard).map_err(PersistEngineStateError::into_engine_error)
}

// Regression for resolving the state-key (a KMS/HSM subprocess for the `command`
// provider) only at the actual persist site, under the held ENGINE_STATE lock: an
// idempotent replay returns its cached result WITHOUT ever resolving the key, so a
// transient key-provider outage cannot turn a cached read into a failure. Here a
// first build persists with the working provider; a replay then succeeds even
// though the state-key command now fails.
#[test]
fn idempotent_build_tx_replay_survives_state_key_outage() {
    let _guard = lock_test_state();
    let _state_path = configure_test_state_path("build_tx_replay_state_key_outage");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-build-tx-replay-key-outage";

    // First build persists with the default (working) state-key provider.
    let first = build_taproot_tx(build_policy_test_request(session_id))
        .expect("first build_taproot_tx should persist and succeed");

    // Now make the `command` state-key provider fail. The idempotent replay
    // returns the cached result without persisting, so it must not depend on the
    // key command succeeding.
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV, "exit 7");

    let replay = build_taproot_tx(build_policy_test_request(session_id)).expect(
        "idempotent build_taproot_tx replay must succeed despite a failing state-key command",
    );
    assert_eq!(
        replay.tx_hex, first.tx_hex,
        "replay must return the cached transaction"
    );

    std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV);
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_PROVIDER_ENV);
    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_persist_failures_roll_back_or_retry_durably() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("build_tx_persist_durability");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let existing_session = "session-build-tx-existing-state-rollback";
    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        guard.sessions.insert(
            existing_session.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some("existing-wallet-binding".to_string()),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState::default(),
                audit: AuditTrail::default(),
            },
        );
    }
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    build_taproot_tx(build_policy_test_request(existing_session))
        .expect_err("pre-replacement failure must restore an existing session's build fields");
    clear_persist_fault_injection_for_tests();
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = &guard.sessions[existing_session];
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some("existing-wallet-binding")
        );
        assert!(session.signing.build_tx_request_fingerprint.is_none());
        assert!(session.signing.tx_result.is_none());
    }

    let pre_replace_session = "session-build-tx-pre-replace-failure";
    let pre_replace_request = build_policy_test_request(pre_replace_session);
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let pre_replace_error = build_taproot_tx(pre_replace_request.clone())
        .expect_err("a pre-replacement failure must not cache success");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        pre_replace_error,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        assert!(
            !guard.sessions.contains_key(pre_replace_session),
            "a first-use BuildTaprootTx session must be removed on rollback"
        );
    }
    let pre_replace_result =
        build_taproot_tx(pre_replace_request).expect("retry performs and persists the build");

    let post_replace_session = "session-build-tx-post-replace-failure";
    let post_replace_request = build_policy_test_request(post_replace_session);
    let post_replace_fingerprint = fingerprint(&post_replace_request).expect("request fingerprint");
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let post_replace_error = build_taproot_tx(post_replace_request.clone())
        .expect_err("a post-replacement failure must report unconfirmed durability");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        post_replace_error,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(matches!(
        pending_build_taproot_tx_operation(post_replace_session),
        Some(PersistencePendingOperation::BuildTaprootTx {
            request_fingerprint,
            ..
        }) if request_fingerprint == post_replace_fingerprint
    ));

    // Prove the cache-hit retry attempts persistence before returning success.
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let retry_error = build_taproot_tx(post_replace_request.clone())
        .expect_err("retry must not return cached success while persistence still fails");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        retry_error,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(pending_build_taproot_tx_operation(post_replace_session).is_some());

    let post_replace_result = build_taproot_tx(post_replace_request.clone())
        .expect("retry repairs persistence then returns the cached artifact");
    assert!(pending_build_taproot_tx_operation(post_replace_session).is_none());

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    assert_eq!(
        build_taproot_tx(post_replace_request).expect("durable cached retry after restart"),
        post_replace_result
    );
    assert_eq!(
        build_taproot_tx(build_policy_test_request(pre_replace_session))
            .expect("pre-replacement retry also survived restart"),
        pre_replace_result
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_restores_evicted_retirement_on_pre_replace_failure() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("build_tx_retired_slot_rollback");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

    let retired_session = "build-slot-retired-replay-tombstone";
    let consumed_marker = interactive_consumed_marker(&hash_hex(b"build-slot-attempt"), 1);
    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        guard.sessions.insert(
            "build-slot-active-owner".to_string(),
            SessionState::default(),
        );
        guard.sessions.insert(
            retired_session.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some("build-slot-wallet-key".to_string()),
                    consumed_attempt_markers: HashSet::from([consumed_marker.clone()]),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState {
                    retired_interactive_at_unix: Some(1),
                    ..Default::default()
                },
                audit: AuditTrail::default(),
            },
        );
        persist_engine_state_to_storage(&guard).expect("persist full shared session budget");
    }

    let newcomer = "build-slot-new-active";
    let request = build_policy_test_request(newcomer);
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let faulted = build_taproot_tx(request.clone())
        .expect_err("pre-replacement Build fault rolls back slot reservation");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        faulted,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    {
        let guard = state().expect("state").lock().expect("engine lock");
        assert_eq!(guard.sessions.len(), 2);
        assert!(!guard.sessions.contains_key(newcomer));
        assert!(guard.sessions[retired_session]
            .interactive
            .consumed_attempt_markers
            .contains(&consumed_marker));
    }

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    build_taproot_tx(request).expect("healthy retry evicts the retired slot and persists");
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("engine lock");
        assert_eq!(guard.sessions.len(), 2);
        assert!(guard.sessions.contains_key("build-slot-active-owner"));
        assert!(guard.sessions.contains_key(newcomer));
        assert!(!guard.sessions.contains_key(retired_session));
    }

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_capacity_preflight_does_not_consume_policy_rate_token() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("build_tx_capacity_rate_preflight");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");
    std::env::set_var(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV, "1");

    let retired_session = "build-rate-protected-retired";
    let aggregate_pin = {
        let mut guard = state().expect("state").lock().expect("engine lock");
        guard.sessions.insert(
            "build-rate-active-owner".to_string(),
            SessionState::default(),
        );
        guard.sessions.insert(
            retired_session.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some("build-rate-wallet-key".to_string()),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState {
                    retired_interactive_at_unix: Some(1),
                    ..Default::default()
                },
                audit: AuditTrail::default(),
            },
        );
        Arc::clone(
            &guard.sessions[retired_session]
                .capacity_pins
                .aggregate_eviction_pin,
        )
    };

    let request = build_policy_test_request("build-rate-new-active");
    let rejected = build_taproot_tx(request.clone())
        .expect_err("a pinned full registry must reject before policy charging");
    assert!(matches!(
        rejected,
        EngineError::Internal(ref message)
            if message.contains("no retired session is available for eviction")
    ));

    drop(aggregate_pin);
    build_taproot_tx(request)
        .expect("the first policy token remains available after capacity recovers");

    std::env::remove_var(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV);
    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn production_profile_forces_provenance_gate_without_env_flag() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::remove_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV);
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    assert!(provenance_gate_enforced());

    std::env::remove_var(TBTC_SIGNER_PROFILE_ENV);
    assert!(provenance_gate_enforced());

    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_DEVELOPMENT);
    assert!(!provenance_gate_enforced());
}

#[test]
fn unknown_profile_value_fails_closed_to_production() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    // A typo / unknown profile value must be treated as production
    // (fail-closed) rather than panicking. The previous panic on the
    // unvalidated env-fallback path turned one typo into a process-wide DoS.
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, "staging");
    assert!(
        signer_profile_is_production(),
        "unrecognized profile must fail closed to production"
    );

    std::env::remove_var(TBTC_SIGNER_PROFILE_ENV);
}

#[test]
fn canary_rollout_status_rejects_when_provenance_gate_requires_attestation() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");

    let err = canary_rollout_status().expect_err("expected provenance gate rejection");
    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "missing_attestation_status");

    clear_state_storage_policy_overrides();
}

// Real per-seat distributed-DKG material in the native (Go-facing) form the
// persist op takes: each member's OWN key package plus the shared public key
// package. Dealer-generated here for brevity, but shaped like a distributed
// Part3 output (one key package per member + one public key package).
fn sample_distributed_dkg_native_material(
    seed: u8,
) -> (
    crate::api::NativeFrostPublicKeyPackage,
    BTreeMap<u16, crate::api::NativeFrostKeyPackage>,
) {
    let identifiers = [1_u16, 2, 3]
        .iter()
        .map(|m| participant_identifier_to_frost_identifier(*m).expect("frost identifier"))
        .collect::<Vec<_>>();
    let rng = ZeroizingChaCha20Rng::from_seed([seed; 32]);
    let (shares, public_key_package) = frost::keys::generate_with_dealer(
        3,
        2,
        frost::keys::IdentifierList::Custom(&identifiers),
        rng,
    )
    .expect("generate_with_dealer");

    // Normalize to even-Y exactly as dkg_part3 does, so this material matches a
    // real distributed DKG's output (the x-only verifying key is even-Y per
    // BIP-340); raw generate_with_dealer output is odd-Y for some seeds.
    let is_even_y = public_key_package.has_even_y();
    let public_key_package = public_key_package.into_even_y(Some(is_even_y));
    let native_public =
        native_public_key_package_from_frost(&public_key_package).expect("native public package");

    let mut native_key_packages = BTreeMap::new();
    for member in [1_u16, 2, 3] {
        let frost_id =
            participant_identifier_to_frost_identifier(member).expect("frost identifier");
        let share = shares.get(&frost_id).expect("share for member").clone();
        let key_package = frost::keys::KeyPackage::try_from(share)
            .expect("key package")
            .into_even_y(Some(is_even_y));
        native_key_packages.insert(
            member,
            crate::api::NativeFrostKeyPackage {
                identifier: frost_identifier_to_go_string(*key_package.identifier()),
                data_hex: hex::encode(key_package.serialize().expect("serialize key package"))
                    .into(),
            },
        );
    }

    (native_public, native_key_packages)
}

// A multi-seat operator persists several local seats of the SAME distributed DKG
// into one session; the key packages must accumulate (not overwrite), so every
// local seat can later open an interactive signing session.
#[test]
fn persist_distributed_dkg_key_package_accumulates_seats_under_one_session() {
    let _guard = lock_test_state();
    reset_for_tests();

    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(7);
    let session_id = "session-distributed-persist-accumulate".to_string();

    let result1 =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: session_id.clone(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: native_public.clone(),
        })
        .expect("persist seat 1");
    assert_eq!(result1.threshold, 2);
    assert_eq!(result1.participant_count, 3);
    assert!(!result1.key_group.is_empty());

    let result2 =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: session_id.clone(),
            participant_identifier: 2,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&2).expect("seat 2").clone(),
            public_key_package: native_public.clone(),
        })
        .expect("persist seat 2");
    assert_eq!(
        result2.key_group, result1.key_group,
        "sibling seats of one DKG must share the key group"
    );

    let guard = state().expect("state").lock().expect("engine lock");
    let session = guard.sessions.get(&session_id).expect("session exists");
    let key_packages = session
        .dkg
        .key_packages
        .as_ref()
        .expect("key packages present");
    assert!(
        key_packages.contains_key(&1) && key_packages.contains_key(&2),
        "both accumulated seats must be stored (got {:?})",
        key_packages.keys().collect::<Vec<_>>()
    );
    assert!(session.dkg.public_key_package.is_some());
}

#[test]
fn persist_distributed_dkg_key_package_rejects_second_key_group_owner() {
    let _guard = lock_test_state();
    reset_for_tests();

    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(17);
    persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
        session_id: "wallet-owner-a".to_string(),
        participant_identifier: 1,
        threshold: 2,
        participant_count: 3,
        key_package: native_key_packages.get(&1).expect("seat 1").clone(),
        public_key_package: native_public.clone(),
    })
    .expect("first wallet owner persists");

    let duplicate =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "wallet-owner-b".to_string(),
            participant_identifier: 2,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&2).expect("seat 2").clone(),
            public_key_package: native_public,
        })
        .expect_err("one key_group must not have two wallet-session owners");
    assert!(
        matches!(duplicate, EngineError::SessionConflict { ref session_id }
            if session_id == "wallet-owner-b"),
        "unexpected error: {duplicate:?}"
    );

    let guard = state().expect("state").lock().expect("engine lock");
    assert!(guard.sessions.contains_key("wallet-owner-a"));
    assert!(!guard.sessions.contains_key("wallet-owner-b"));
}

#[test]
fn persist_distributed_dkg_key_package_rejects_a_bound_signing_session() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "roast-session-already-bound";
    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        guard.sessions.insert(
            session_id.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some("original-wallet-key-group".to_string()),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState::default(),
                audit: AuditTrail::default(),
            },
        );
    }

    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(19);
    let error =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: session_id.to_string(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: native_public,
        })
        .expect_err("a per-signing session must never become a DKG wallet owner");
    assert!(
        matches!(error, EngineError::SessionConflict { session_id: ref rejected }
            if rejected == session_id),
        "unexpected error: {error:?}"
    );

    let guard = state().expect("state").lock().expect("engine lock");
    let session = guard
        .sessions
        .get(session_id)
        .expect("bound session remains");
    assert!(session.dkg.result.is_none());
    assert!(session.dkg.key_packages.is_none());
    assert_eq!(
        session.interactive.bound_key_group.as_deref(),
        Some("original-wallet-key-group")
    );
}

#[test]
fn persist_distributed_dkg_key_package_pre_replace_failure_restores_retired_slot() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("distributed_dkg_persist_rollback");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

    let retired_session = "distributed-dkg-retired-replay-tombstone";
    let consumed_marker = interactive_consumed_marker(&hash_hex(b"distributed-dkg-attempt"), 1);
    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        guard.sessions.insert(
            "distributed-dkg-active-owner".to_string(),
            SessionState::default(),
        );
        guard.sessions.insert(
            retired_session.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some("distributed-dkg-retired-key".to_string()),
                    consumed_attempt_markers: HashSet::from([consumed_marker.clone()]),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState {
                    retired_interactive_at_unix: Some(1),
                    ..Default::default()
                },
                audit: AuditTrail::default(),
            },
        );
        persist_engine_state_to_storage(&guard).expect("persist full shared session budget");
    }

    let session_id = "session-distributed-dkg-persist-rollback";
    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(23);
    let request = crate::api::PersistDistributedDkgKeyPackageRequest {
        session_id: session_id.to_string(),
        participant_identifier: 1,
        threshold: 2,
        participant_count: 3,
        key_package: native_key_packages.get(&1).expect("seat 1").clone(),
        public_key_package: native_public,
    };

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let error = persist_distributed_dkg_key_package(request.clone())
        .expect_err("pre-replacement DKG persist fault must roll back");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        error,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        assert!(
            !guard.sessions.contains_key(session_id),
            "failed first persistence must not leave in-memory-only DKG material"
        );
        assert!(guard.sessions[retired_session]
            .interactive
            .consumed_attempt_markers
            .contains(&consumed_marker));
        assert_eq!(guard.sessions.len(), 2);
    }

    let result = persist_distributed_dkg_key_package(request)
        .expect("retry installs and persists the distributed DKG package");
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let guard = state().expect("engine state").lock().expect("engine lock");
    let session = guard
        .sessions
        .get(session_id)
        .expect("reloaded DKG session");
    assert_eq!(session.dkg.result.as_ref(), Some(&result));
    assert!(session
        .dkg
        .key_packages
        .as_ref()
        .is_some_and(|packages| packages.contains_key(&1)));
    assert_eq!(guard.sessions.len(), 2);
    assert!(guard.sessions.contains_key("distributed-dkg-active-owner"));
    assert!(!guard.sessions.contains_key(retired_session));
    drop(guard);

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn distributed_dkg_pre_replace_rollback_preserves_existing_seats() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("distributed_dkg_existing_seat_rollback");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-distributed-dkg-existing-seat-rollback";
    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(29);
    let request_for = |participant_identifier| crate::api::PersistDistributedDkgKeyPackageRequest {
        session_id: session_id.to_string(),
        participant_identifier,
        threshold: 2,
        participant_count: 3,
        key_package: native_key_packages
            .get(&participant_identifier)
            .expect("local seat")
            .clone(),
        public_key_package: native_public.clone(),
    };

    let baseline = persist_distributed_dkg_key_package(request_for(1))
        .expect("persist baseline distributed-DKG seat");
    let baseline_key_package = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        guard.sessions[session_id]
            .dkg
            .key_packages
            .as_ref()
            .expect("key packages")[&1]
            .serialize()
            .expect("serialize baseline key package")
    };

    // Replacing the same seat before a failed write must restore the prior
    // secret package rather than dropping the session's only local seat.
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    persist_distributed_dkg_key_package(request_for(1))
        .expect_err("same-seat persist fault must restore the existing package");
    clear_persist_fault_injection_for_tests();
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = &guard.sessions[session_id];
        assert_eq!(session.dkg.result.as_ref(), Some(&baseline));
        assert_eq!(
            session
                .dkg
                .key_packages
                .as_ref()
                .expect("restored key packages")[&1]
                .serialize()
                .expect("serialize restored key package"),
            baseline_key_package
        );
    }

    // Adding a sibling seat before a failed write must leave the prior seat set
    // untouched; a subsequent healthy retry can then add it normally.
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    persist_distributed_dkg_key_package(request_for(2))
        .expect_err("sibling-seat persist fault must roll back only the new seat");
    clear_persist_fault_injection_for_tests();
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let packages = guard.sessions[session_id]
            .dkg
            .key_packages
            .as_ref()
            .expect("baseline key packages");
        assert!(packages.contains_key(&1));
        assert!(!packages.contains_key(&2));
    }
    persist_distributed_dkg_key_package(request_for(2))
        .expect("healthy retry adds the sibling seat");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

// The op rejects a key package whose own identifier does not match the claimed
// participant, and refuses to install a DIFFERENT DKG's key group over a session
// that already holds one.
#[test]
fn persist_distributed_dkg_key_package_rejects_mismatched_and_conflicting_inputs() {
    let _guard = lock_test_state();
    reset_for_tests();

    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(7);

    let mismatch =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "session-persist-mismatch".to_string(),
            participant_identifier: 2, // the key package below is seat 1's
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: native_public.clone(),
        })
        .expect_err("identifier mismatch must be rejected");
    assert!(
        matches!(mismatch, EngineError::Validation(_)),
        "got {mismatch:?}"
    );

    // A threshold that disagrees with the key package's embedded min_signers (the
    // material is 2-of-3) must be rejected at persist, not burned at share release.
    let threshold_mismatch =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "session-persist-threshold-mismatch".to_string(),
            participant_identifier: 1,
            threshold: 3,
            participant_count: 3,
            key_package: native_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: native_public.clone(),
        })
        .expect_err("a threshold disagreeing with the key package must be rejected");
    assert!(
        matches!(threshold_mismatch, EngineError::Validation(_)),
        "got {threshold_mismatch:?}"
    );

    let session_id = "session-persist-conflict".to_string();
    persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
        session_id: session_id.clone(),
        participant_identifier: 1,
        threshold: 2,
        participant_count: 3,
        key_package: native_key_packages.get(&1).expect("seat 1").clone(),
        public_key_package: native_public.clone(),
    })
    .expect("first DKG persists");

    let (other_public, other_key_packages) = sample_distributed_dkg_native_material(9);
    let conflict =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: session_id.clone(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: other_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: other_public,
        })
        .expect_err("a different key group for the same session must conflict");
    assert!(
        matches!(conflict, EngineError::SessionConflict { .. }),
        "got {conflict:?}"
    );
}

// The op writes durable signing material, so it enforces the DKG admission policy
// over the participant set DERIVED from the public key package: a group that
// includes a non-allowlisted participant is rejected before anything is stored.
#[test]
fn persist_distributed_dkg_key_package_enforces_admission_policy() {
    let _guard = lock_test_state();
    reset_for_tests();

    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV, "true");
    std::env::set_var(TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV, "1,2");

    // The group's members are 1, 2, 3 (from the public key package); member 3 is
    // not on the allowlist, so persistence must be refused.
    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(7);
    let err =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "session-persist-admission".to_string(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: native_public,
        })
        .expect_err("a group with a non-allowlisted participant must be rejected");

    let EngineError::AdmissionPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert_eq!(reason_code, "participant_identifier_not_allowlisted");

    // Restore the global policy env so later tests see the default configuration
    // (reset_for_tests does not clear the admission overrides).
    clear_state_storage_policy_overrides();
}

// A distributed DKG whose group includes an auto-quarantined operator must be
// refused before persistence, exactly as the dealer run_dkg refuses it.
#[test]
fn persist_distributed_dkg_key_package_rejects_quarantined_participant() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV, "true");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV, "2");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV, "1");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV, "2");

    // Operator 3, a member of the group below (members 1,2,3), is auto-quarantined.
    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        guard.quarantined_operator_identifiers.insert(3);
    }

    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(7);
    let err =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "session-persist-quarantine".to_string(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: native_public,
        })
        .expect_err("a group including a quarantined operator must be rejected");
    assert!(
        matches!(err, EngineError::QuarantinePolicyRejected { ref reason_code, .. }
            if reason_code == "operator_auto_quarantined"),
        "unexpected error: {err:?}"
    );

    clear_state_storage_policy_overrides();
}

// The op writes signing material to durable state, so it must enforce the same
// provenance gate run_dkg and the interactive path do: an unattested runtime
// cannot install distributed-DKG signing material.
#[test]
fn persist_distributed_dkg_key_package_rejects_when_provenance_gate_requires_attestation() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
    std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, "sigstore-main");
    std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(7);
    let err =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "session-persist-provenance".to_string(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: native_public,
        })
        .expect_err("expected provenance gate rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert_eq!(reason_code, "missing_attestation_status");

    clear_state_storage_policy_overrides();
}

// A key package whose SECRET signing share does not derive to its (public)
// verifying share must be rejected: it would open interactive attempts and burn
// them, producing shares that never verify. Crafted with seat 1's identity and
// verifying share (so it matches the public package) but seat 2's signing share.
#[test]
fn persist_distributed_dkg_key_package_rejects_signing_share_not_deriving_to_public() {
    let _guard = lock_test_state();
    reset_for_tests();

    let identifiers = [1_u16, 2, 3]
        .iter()
        .map(|m| participant_identifier_to_frost_identifier(*m).expect("frost id"))
        .collect::<Vec<_>>();
    let rng = ZeroizingChaCha20Rng::from_seed([7_u8; 32]);
    let (shares, public_key_package) = frost::keys::generate_with_dealer(
        3,
        2,
        frost::keys::IdentifierList::Custom(&identifiers),
        rng,
    )
    .expect("generate_with_dealer");
    let is_even_y = public_key_package.has_even_y();
    let public_key_package = public_key_package.into_even_y(Some(is_even_y));
    let native_public =
        native_public_key_package_from_frost(&public_key_package).expect("native public");

    let key_package_of = |member: u16| {
        let id = participant_identifier_to_frost_identifier(member).expect("frost id");
        frost::keys::KeyPackage::try_from(shares.get(&id).expect("share").clone())
            .expect("key package")
            .into_even_y(Some(is_even_y))
    };
    let key_package_1 = key_package_of(1);
    let key_package_2 = key_package_of(2);

    // Seat 1's identity + verifying share, but seat 2's signing share.
    let corrupt = frost::keys::KeyPackage::new(
        *key_package_1.identifier(),
        *key_package_2.signing_share(),
        *key_package_1.verifying_share(),
        *key_package_1.verifying_key(),
        *key_package_1.min_signers(),
    );
    let corrupt_data = corrupt.serialize().expect("serialize corrupt key package");

    let err =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "session-persist-share-mismatch".to_string(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: crate::api::NativeFrostKeyPackage {
                identifier: frost_identifier_to_go_string(*key_package_1.identifier()),
                data_hex: hex::encode(corrupt_data).into(),
            },
            public_key_package: native_public,
        })
        .expect_err("a signing share not deriving to its verifying share must be rejected");
    assert!(matches!(err, EngineError::Validation(_)), "got {err:?}");
}

// A second seat of the SAME session (same group key) must carry the SAME public
// key package. A package with the same group verifying key but a different
// verifying-shares map must be rejected on accumulate, or later signing would use
// public material inconsistent with the newly stored key.
#[test]
fn persist_distributed_dkg_key_package_rejects_mismatched_public_package_on_accumulate() {
    let _guard = lock_test_state();
    reset_for_tests();

    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(7);
    let session_id = "session-persist-accumulate-mismatch".to_string();

    persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
        session_id: session_id.clone(),
        participant_identifier: 1,
        threshold: 2,
        participant_count: 3,
        key_package: native_key_packages.get(&1).expect("seat 1").clone(),
        public_key_package: native_public.clone(),
    })
    .expect("first seat persists");

    // Same group verifying key (so the same key group), but a different shares map:
    // give seat 3 seat 2's verifying share. Seat 1's own share is unchanged, so its
    // key package still validates - only the accumulate public-package check fails.
    let mut tampered = native_public.clone();
    let id2 = frost_identifier_to_go_string(
        participant_identifier_to_frost_identifier(2).expect("frost id"),
    );
    let id3 = frost_identifier_to_go_string(
        participant_identifier_to_frost_identifier(3).expect("frost id"),
    );
    let share2 = tampered
        .verifying_shares
        .get(&id2)
        .expect("share 2")
        .clone();
    tampered.verifying_shares.insert(id3, share2);

    let err =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: session_id.clone(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: tampered,
        })
        .expect_err("a mismatched public package for the same session must be rejected");
    assert!(matches!(err, EngineError::Validation(_)), "got {err:?}");
}

#[test]
fn provenance_gate_rejects_runtime_prerelease_for_release_minimum() {
    let runtime_version = parse_version_triplet("1.2.3-rc1").expect("runtime parse");
    let minimum_version = parse_version_triplet("1.2.3").expect("minimum parse");
    assert!(!runtime_satisfies_minimum_version(
        runtime_version,
        minimum_version
    ));

    let runtime_version = parse_version_triplet("1.2.3").expect("runtime parse");
    let minimum_version = parse_version_triplet("1.2.3-rc1").expect("minimum parse");
    assert!(runtime_satisfies_minimum_version(
        runtime_version,
        minimum_version
    ));
}

fn taproot_prevout_script_hex() -> String {
    format!("5120{}", "33".repeat(32))
}

fn build_policy_test_request(session_id: &str) -> BuildTaprootTxRequest {
    BuildTaprootTxRequest {
        session_id: session_id.to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: 10_000,
            script_pubkey_hex: taproot_prevout_script_hex(),
        }],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "22".repeat(32)),
            value_sats: 9_000,
        }],
        script_tree_hex: None,
    }
}

#[test]
fn build_taproot_tx_rejects_invalid_or_non_p2tr_prevout_scripts() {
    let _guard = lock_test_state();
    reset_for_tests();

    for (case, script_pubkey_hex, expected_detail) in [
        ("empty", "", "not a P2TR prevout"),
        ("non-hex", "zz", "invalid input script_pubkey_hex"),
        ("malformed", "4c", "invalid input script_pubkey_hex"),
        (
            "non-p2tr",
            "00141111111111111111111111111111111111111111",
            "not a P2TR prevout",
        ),
    ] {
        let mut request = build_policy_test_request(&format!("session-prevout-{case}"));
        request.inputs[0].script_pubkey_hex = script_pubkey_hex.to_string();

        let error = build_taproot_tx(request)
            .expect_err("an invalid or non-P2TR prevout script must fail closed");
        let EngineError::Validation(detail) = error else {
            panic!("unexpected error variant: {error:?}");
        };
        assert!(
            detail.contains(expected_detail),
            "case [{case}] returned unexpected validation detail: {detail}"
        );
    }
}

#[test]
fn signing_policy_firewall_rejects_every_malformed_cached_artifact_shape() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr");
    configure_required_signing_policy_limits_for_tests();

    let session_id = "session-invalid-policy-artifact";
    let valid = build_taproot_tx(build_policy_test_request(session_id))
        .expect("build a valid policy artifact");
    let message = valid.taproot_key_spend_sighashes_hex[0].clone();

    let mut cases = Vec::new();

    let mut wrong_session = valid.clone();
    wrong_session.session_id = "different-session".to_string();
    cases.push(("wrong session", wrong_session));

    let mut non_hex_tx = valid.clone();
    non_hex_tx.tx_hex = "zz".to_string();
    cases.push(("non-hex transaction", non_hex_tx));

    let mut invalid_tx = valid.clone();
    invalid_tx.tx_hex = "00".to_string();
    cases.push(("invalid transaction", invalid_tx));

    let mut legacy_empty_sighashes = valid.clone();
    legacy_empty_sighashes
        .taproot_key_spend_sighashes_hex
        .clear();
    cases.push(("pre-ABI-3 empty sighash list", legacy_empty_sighashes));

    let mut non_hex_sighash = valid.clone();
    non_hex_sighash.taproot_key_spend_sighashes_hex[0] = "zz".to_string();
    cases.push(("non-hex sighash", non_hex_sighash));

    let mut short_sighash = valid;
    short_sighash.taproot_key_spend_sighashes_hex[0] = "00".to_string();
    cases.push(("short sighash", short_sighash));

    for (case, artifact) in cases {
        let error = enforce_signing_message_binding_to_policy_checked_build_tx(
            session_id,
            &message,
            None,
            Some(&artifact),
            None,
        )
        .expect_err("a malformed policy artifact must fail closed");
        assert!(
            matches!(error, EngineError::SigningPolicyRejected { ref reason_code, .. }
                if reason_code == "invalid_policy_checked_build_tx_artifact"),
            "case [{case}] returned unexpected error: {error:?}"
        );
    }

    let metrics = hardening_metrics();
    assert_eq!(metrics.build_taproot_tx_policy_reject_total, 6);
    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_records_ordered_bip341_key_spend_sighashes() {
    let _guard = lock_test_state();
    reset_for_tests();

    let request = BuildTaprootTxRequest {
        session_id: "session-bip341-policy-messages".to_string(),
        inputs: vec![
            crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: 10_000,
                script_pubkey_hex: taproot_prevout_script_hex(),
            },
            crate::api::TxInput {
                txid_hex: "22".repeat(32),
                vout: 1,
                value_sats: 11_000,
                script_pubkey_hex: format!("5120{}", "44".repeat(32)),
            },
        ],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "55".repeat(32)),
            value_sats: 20_000,
        }],
        script_tree_hex: None,
    };
    let result = build_taproot_tx(request.clone()).expect("build policy transaction");

    assert_eq!(result.taproot_key_spend_sighashes_hex.len(), 2);
    assert_ne!(
        result.taproot_key_spend_sighashes_hex[0], result.taproot_key_spend_sighashes_hex[1],
        "BIP-341 commits to the input index"
    );
    assert!(result
        .taproot_key_spend_sighashes_hex
        .iter()
        .all(|sighash| sighash.len() == 64 && hex::decode(sighash).is_ok()));

    let tx_bytes = hex::decode(&result.tx_hex).expect("transaction hex");
    let tx: Transaction = deserialize(&tx_bytes).expect("transaction decode");
    assert_eq!(
        tx.version,
        Version::ONE,
        "BuildTaprootTx must match the Go host's canonical transaction version"
    );
    let prevouts = request
        .inputs
        .iter()
        .map(|input| TxOut {
            value: Amount::from_sat(input.value_sats),
            script_pubkey: ScriptBuf::from_bytes(
                hex::decode(&input.script_pubkey_hex).expect("prevout script hex"),
            ),
        })
        .collect::<Vec<_>>();
    let mut cache = SighashCache::new(&tx);
    for (input_index, recorded) in result.taproot_key_spend_sighashes_hex.iter().enumerate() {
        let expected = cache
            .taproot_key_spend_signature_hash(
                input_index,
                &Prevouts::All(&prevouts),
                TapSighashType::Default,
            )
            .expect("BIP-341 sighash");
        assert_eq!(recorded, &hex::encode(expected.to_byte_array()));
    }
    assert!(
        !result
            .taproot_key_spend_sighashes_hex
            .contains(&hash_hex(&tx_bytes)),
        "SHA256(unsigned_tx) is not a BIP-341 signing message"
    );
}

#[test]
fn signing_policy_firewall_is_enforced_in_production_by_default() {
    let _guard = lock_test_state();
    reset_for_tests();

    // Development without the opt-in flag: not enforced (unchanged default).
    std::env::remove_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV);
    assert!(
        !signing_policy_firewall_enforced(),
        "firewall must stay opt-in outside production"
    );

    // Production: always enforced, no flag required (mirrors the provenance gate).
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    assert!(
        signing_policy_firewall_enforced(),
        "firewall must be force-enabled in production"
    );

    reset_for_tests();
}

#[test]
fn signing_policy_firewall_config_uses_builtin_defaults_in_production() {
    let _guard = lock_test_state();
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);

    // No explicit firewall policy env -> conservative built-in defaults, so a
    // production signer boots without shipping full policy config.
    for env in [
        TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV,
        TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV,
        TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV,
        TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV,
    ] {
        std::env::remove_var(env);
    }

    let config = load_signing_policy_firewall_config()
        .expect("firewall config loads with built-in defaults")
        .expect("firewall is enforced in production");

    let expected_classes: std::collections::HashSet<String> = DEFAULT_ALLOWED_SCRIPT_CLASSES
        .iter()
        .map(|class| class.to_string())
        .collect();
    assert_eq!(config.allowed_script_classes, expected_classes);
    // "other" (non-standard) is not in the default allowlist -> fails closed.
    assert!(!config.allowed_script_classes.contains("other"));
    assert_eq!(config.max_output_count, DEFAULT_MAX_OUTPUT_COUNT);
    assert_eq!(config.max_output_value_sats, BITCOIN_MAX_MONEY_SATS);
    assert_eq!(config.max_total_output_value_sats, BITCOIN_MAX_MONEY_SATS);

    reset_for_tests();
}

#[test]
fn build_taproot_tx_signing_policy_firewall_rejects_disallowed_script_class() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let err = build_taproot_tx(build_policy_test_request(
        "session-signing-policy-script-class-reject",
    ))
    .expect_err("expected signing policy rejection");

    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "script_class_not_allowlisted");

    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_signing_policy_firewall_rejects_excess_output_count() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr");
    std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV, "1");
    std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV, "100000000");
    std::env::set_var(
        TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV,
        "2100000000000000",
    );

    let mut request = build_policy_test_request("session-signing-policy-output-count-reject");
    request.inputs[0].value_sats = 20_000;
    request.outputs.push(crate::api::TxOutput {
        script_pubkey_hex: format!("5120{}", "33".repeat(32)),
        value_sats: 9_000,
    });

    let err =
        build_taproot_tx(request).expect_err("expected signing policy output count rejection");

    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "output_count_exceeds_policy_limit");

    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_signing_policy_firewall_rejects_excess_single_output_value() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV, "64");
    std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV, "5000");
    std::env::set_var(
        TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV,
        "2100000000000000",
    );

    let err = build_taproot_tx(build_policy_test_request(
        "session-signing-policy-single-output-value-reject",
    ))
    .expect_err("expected signing policy single output value rejection");

    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "single_output_value_exceeds_policy_limit");

    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_signing_policy_firewall_rejects_excess_total_output_value() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_COUNT_ENV, "64");
    std::env::set_var(TBTC_SIGNER_POLICY_MAX_OUTPUT_VALUE_SATS_ENV, "100000000");
    std::env::set_var(TBTC_SIGNER_POLICY_MAX_TOTAL_OUTPUT_VALUE_SATS_ENV, "5000");

    let err = build_taproot_tx(build_policy_test_request(
        "session-signing-policy-total-output-value-reject",
    ))
    .expect_err("expected signing policy total output value rejection");

    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "total_output_value_exceeds_policy_limit");

    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_rejects_total_input_value_above_bitcoin_max_money() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("build_taproot_tx_max_input_total");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let request = BuildTaprootTxRequest {
        session_id: "session-build-tx-max-input-total".to_string(),
        inputs: vec![
            crate::api::TxInput {
                txid_hex: "11".repeat(32),
                vout: 0,
                value_sats: BITCOIN_MAX_MONEY_SATS,
                script_pubkey_hex: taproot_prevout_script_hex(),
            },
            crate::api::TxInput {
                txid_hex: "22".repeat(32),
                vout: 0,
                value_sats: 1,
                script_pubkey_hex: taproot_prevout_script_hex(),
            },
        ],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "33".repeat(32)),
            value_sats: 1,
        }],
        script_tree_hex: None,
    };

    let err = build_taproot_tx(request).expect_err("expected max money rejection");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert!(
        message.contains("input value_sats total") && message.contains("exceeds Bitcoin max money"),
        "unexpected validation message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_rejects_total_output_value_above_bitcoin_max_money() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("build_taproot_tx_max_output_total");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let request = BuildTaprootTxRequest {
        session_id: "session-build-tx-max-output-total".to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: BITCOIN_MAX_MONEY_SATS,
            script_pubkey_hex: taproot_prevout_script_hex(),
        }],
        outputs: vec![
            crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "22".repeat(32)),
                value_sats: BITCOIN_MAX_MONEY_SATS / 2 + 1,
            },
            crate::api::TxOutput {
                script_pubkey_hex: format!("5120{}", "33".repeat(32)),
                value_sats: BITCOIN_MAX_MONEY_SATS / 2 + 1,
            },
        ],
        script_tree_hex: None,
    };

    let err = build_taproot_tx(request).expect_err("expected max money rejection");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert!(
        message.contains("output value_sats total")
            && message.contains("exceeds Bitcoin max money"),
        "unexpected validation message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_signing_policy_firewall_rejects_outside_utc_window() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();
    let current_hour = current_utc_hour();
    let start_hour = (current_hour + 1) % 24;
    let end_hour = (current_hour + 2) % 24;
    std::env::set_var(
        TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV,
        start_hour.to_string(),
    );
    std::env::set_var(
        TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV,
        end_hour.to_string(),
    );

    let err = build_taproot_tx(build_policy_test_request(
        "session-signing-policy-utc-window-reject",
    ))
    .expect_err("expected signing policy UTC window rejection");

    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "request_outside_allowed_utc_window");

    clear_state_storage_policy_overrides();
}

#[test]
fn signing_policy_firewall_rejects_equal_utc_window_bounds() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    // A degenerate equal-bounds window (start == end) must be rejected at load
    // time. utc_hour_in_window treats start == end as "always in window", so
    // accepting it would silently disable the time-of-day control (fail-open).
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_UTC_START_HOUR_ENV, "12");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_UTC_END_HOUR_ENV, "12");

    let err = load_signing_policy_firewall_config()
        .expect_err("equal-bounds UTC window must be rejected at load time");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert!(
        message.contains("must differ"),
        "unexpected validation message: {message}"
    );

    clear_state_storage_policy_overrides();
}

#[test]
fn hardening_metrics_tracks_policy_rejections() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let _ = build_taproot_tx(build_policy_test_request(
        "session-hardening-metrics-policy-reject",
    ));

    let metrics = hardening_metrics();
    assert_eq!(metrics.build_taproot_tx_calls_total, 1);
    assert_eq!(metrics.build_taproot_tx_policy_reject_total, 1);
    assert_eq!(metrics.build_taproot_tx_success_total, 0);

    clear_state_storage_policy_overrides();
}

#[test]
fn hardening_metrics_count_calls_before_provenance_gate_rejection() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
    std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, "sigstore-main");
    std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

    // build_taproot_tx is counted the moment it is called, BEFORE the
    // provenance gate rejects it: the call counter increments but the
    // success counter does not.
    let build_tx_err = build_taproot_tx(BuildTaprootTxRequest {
        session_id: "session-metrics-provenance-build-tx".to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: 10_000,
            script_pubkey_hex: taproot_prevout_script_hex(),
        }],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("0014{}", "33".repeat(20)),
            value_sats: 9_000,
        }],
        script_tree_hex: None,
    })
    .expect_err("expected build_taproot_tx provenance gate rejection");
    assert!(matches!(
        build_tx_err,
        EngineError::ProvenanceGateRejected { .. }
    ));

    let metrics = hardening_metrics();
    assert_eq!(metrics.build_taproot_tx_calls_total, 1);
    assert_eq!(metrics.build_taproot_tx_success_total, 0);
    assert_eq!(metrics.refresh_shares_calls_total, 0);
    assert_eq!(metrics.refresh_shares_success_total, 0);

    clear_state_storage_policy_overrides();
}

#[test]
fn refresh_shares_fails_closed_without_mutating_wallet_state() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("refresh_shares_fail_closed");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-refresh-fail-closed";
    ensure_interactive_dkg_session(session_id, "refresh-fail-closed-key-group");
    let persisted_state_before = std::fs::read(&state_path).expect("read baseline state");
    let (key_packages_before, public_key_package_before, dkg_result_before) = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get(session_id).expect("wallet session");
        (
            session.dkg.key_packages.clone(),
            session.dkg.public_key_package.clone(),
            session.dkg.result.clone(),
        )
    };

    let error = refresh_shares(RefreshSharesRequest {
        session_id: session_id.to_string(),
        current_shares: vec![crate::api::ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "aaaa".to_string(),
        }],
    })
    .expect_err("RefreshShares must reject until a cryptographic protocol is implemented");
    assert!(matches!(
        error,
        EngineError::CryptographicRefreshNotSupported { ref session_id }
            if session_id == "session-refresh-fail-closed"
    ));

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get(session_id).expect("wallet session");
        assert_eq!(guard.refresh_epoch_counter, 0);
        assert!(session.lifecycle.refresh_request_fingerprint.is_none());
        assert!(session.lifecycle.refresh_result.is_none());
        assert!(session.lifecycle.refresh_history.is_empty());
        assert_eq!(session.lifecycle.refresh_count, 0);
        assert_eq!(session.dkg.key_packages, key_packages_before);
        assert_eq!(session.dkg.public_key_package, public_key_package_before);
        assert_eq!(session.dkg.result, dkg_result_before);
    }
    assert_eq!(
        std::fs::read(&state_path).expect("read state after rejection"),
        persisted_state_before,
    );

    let metrics = hardening_metrics();
    assert_eq!(metrics.refresh_shares_calls_total, 1);
    assert_eq!(metrics.refresh_shares_success_total, 0);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn first_refresh_deadline_survives_restart_and_becomes_overdue() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("first_refresh_deadline");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV, "60");

    let overdue_session_id = "session-first-refresh-overdue";
    let fresh_session_id = "session-first-refresh-fresh";
    ensure_interactive_dkg_session(overdue_session_id, "first-refresh-overdue-key-group");
    ensure_interactive_dkg_session(fresh_session_id, "first-refresh-fresh-key-group");

    let created_at_unix = now_unix().saturating_sub(600);
    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        guard
            .sessions
            .get_mut(overdue_session_id)
            .expect("overdue wallet session")
            .dkg
            .result
            .as_mut()
            .expect("DKG result")
            .created_at_unix = created_at_unix;
        persist_engine_state_to_storage(&guard).expect("persist DKG creation anchors");
    }

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    let status = refresh_cadence_status(RefreshCadenceStatusRequest {
        session_id: overdue_session_id.to_string(),
    })
    .expect("refresh cadence status");
    assert_eq!(status.refresh_count, 0);
    assert_eq!(status.last_refresh_epoch, 0);
    assert_eq!(status.next_refresh_due_unix, created_at_unix + 60);
    assert!(status.overdue);

    let fresh_status = refresh_cadence_status(RefreshCadenceStatusRequest {
        session_id: fresh_session_id.to_string(),
    })
    .expect("fresh refresh cadence status");
    assert!(!fresh_status.overdue);
    assert_eq!(hardening_metrics().refresh_cadence_overdue_sessions, 1);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn legacy_synthetic_refresh_metadata_cannot_postpone_cadence_or_claim_continuity() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("legacy_synthetic_refresh_metadata");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV, "60");

    let session_id = "session-legacy-synthetic-refresh";
    let key_group = "legacy-synthetic-refresh-key-group";
    ensure_interactive_dkg_session(session_id, key_group);
    let created_at_unix = now_unix().saturating_sub(600);
    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get_mut(session_id).expect("wallet session");
        session
            .dkg
            .result
            .as_mut()
            .expect("DKG result")
            .created_at_unix = created_at_unix;
        session.lifecycle.refresh_request_fingerprint =
            Some("legacy-synthetic-request".to_string());
        session.lifecycle.refresh_result = Some(RefreshSharesResult {
            session_id: session_id.to_string(),
            refresh_epoch: 1,
            new_shares: vec![crate::api::ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "synthetic-hash".to_string(),
            }],
        });
        session.lifecycle.refresh_history = vec![RefreshHistoryRecord {
            refresh_epoch: 1,
            refreshed_at_unix: now_unix(),
            share_count: 1,
            key_group: Some(key_group.to_string()),
            request_fingerprint: Some("legacy-synthetic-request".to_string()),
        }];
        session.lifecycle.refresh_count = 1;
        guard.refresh_epoch_counter = 1;
        persist_engine_state_to_storage(&guard).expect("persist legacy synthetic metadata");
    }

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    let status = refresh_cadence_status(RefreshCadenceStatusRequest {
        session_id: session_id.to_string(),
    })
    .expect("refresh cadence status");
    assert_eq!(status.refresh_count, 0);
    assert_eq!(status.last_refresh_epoch, 0);
    assert_eq!(status.next_refresh_due_unix, created_at_unix + 60);
    assert!(status.overdue);
    assert!(!status.continuity_preserved);
    assert_eq!(
        status.continuity_reference_key_group.as_deref(),
        Some(key_group)
    );
    assert_eq!(hardening_metrics().refresh_cadence_overdue_sessions, 1);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn unanchored_legacy_refresh_session_is_immediately_overdue_after_restart() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("unanchored_legacy_refresh");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV, "60");

    let session_id = "session-unanchored-legacy-refresh";
    let plain_session_id = "session-unanchored-without-refresh-artifacts";
    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.entry(session_id.to_string()).or_default();
        assert!(session.dkg.result.is_none());
        session.lifecycle.refresh_request_fingerprint =
            Some("legacy-refresh-only-request".to_string());
        session.lifecycle.refresh_result = Some(RefreshSharesResult {
            session_id: session_id.to_string(),
            refresh_epoch: 1,
            new_shares: vec![crate::api::ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "aa".repeat(32),
            }],
        });
        session.lifecycle.refresh_history = vec![RefreshHistoryRecord {
            refresh_epoch: 1,
            refreshed_at_unix: now_unix(),
            share_count: 1,
            key_group: None,
            request_fingerprint: Some("legacy-refresh-only-request".to_string()),
        }];
        session.lifecycle.refresh_count = 1;
        guard.refresh_epoch_counter = 1;
        guard
            .sessions
            .entry(plain_session_id.to_string())
            .or_default();
        persist_engine_state_to_storage(&guard).expect("persist refresh-only legacy session");
    }

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    let status = refresh_cadence_status(RefreshCadenceStatusRequest {
        session_id: session_id.to_string(),
    })
    .expect("refresh cadence status");
    assert_eq!(status.refresh_count, 0);
    assert_eq!(status.last_refresh_epoch, 0);
    assert_eq!(status.next_refresh_due_unix, 0);
    assert!(status.overdue);
    assert!(!status.continuity_preserved);
    assert!(status.continuity_reference_key_group.is_none());

    let plain_status = refresh_cadence_status(RefreshCadenceStatusRequest {
        session_id: plain_session_id.to_string(),
    })
    .expect("plain session cadence status");
    assert!(!plain_status.overdue);
    assert!(plain_status.continuity_preserved);
    assert_eq!(hardening_metrics().refresh_cadence_overdue_sessions, 1);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn refresh_cadence_overdue_sentinel_survives_clock_rollback() {
    assert!(refresh_cadence_is_overdue(0, 0));
    assert!(refresh_cadence_is_overdue(101, 100));
    assert!(!refresh_cadence_is_overdue(100, 100));
    assert!(!refresh_cadence_is_overdue(99, 100));
}

#[test]
fn differential_fuzzing_reports_no_unresolved_critical_divergence() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let result = run_differential_fuzzing(DifferentialFuzzRequest {
        seed: 0xD1FF_2026_0302_0001,
        case_count: 64,
    })
    .expect("run differential fuzzing");
    assert_eq!(result.case_count, 64);
    assert_eq!(result.critical_divergence_count, 0);
    assert!(!result.unresolved_critical_divergence);

    let metrics = hardening_metrics();
    assert!(metrics.differential_fuzz_runs_total >= 1);
    assert_eq!(metrics.differential_fuzz_critical_divergence_total, 0);
}

#[test]
fn canary_promotion_and_rollback_controls_persist_across_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("canary_rollout_controls");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let initial_status = canary_rollout_status().expect("canary rollout status");
    assert_eq!(initial_status.current_percent, 10);
    assert!(!initial_status.promotion_gate_passed);
    assert_eq!(initial_status.recommended_next_percent, None);

    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    let ready_10 = canary_rollout_status().expect("10% stage has fresh evidence");
    assert!(ready_10.promotion_gate_passed);
    assert_eq!(ready_10.recommended_next_percent, Some(50));

    let promoted_50 =
        promote_canary(PromoteCanaryRequest { target_percent: 50 }).expect("promote canary to 50%");
    assert_eq!(promoted_50.from_percent, 10);
    assert_eq!(promoted_50.to_percent, 50);

    let after_50 = canary_rollout_status().expect("promotion resets stage evidence");
    assert!(!after_50.promotion_gate_passed);
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    let promoted_100 = promote_canary(PromoteCanaryRequest {
        target_percent: 100,
    })
    .expect("promote canary to 100%");
    assert_eq!(promoted_100.from_percent, 50);
    assert_eq!(promoted_100.to_percent, 100);

    let rolled_back = rollback_canary(RollbackCanaryRequest {
        reason: "slo regression drill".to_string(),
    })
    .expect("rollback canary");
    assert_eq!(rolled_back.from_percent, 100);
    assert_eq!(rolled_back.to_percent, 50);

    reload_state_from_storage_for_tests();
    let post_reload_status = canary_rollout_status().expect("canary rollout status after reload");
    assert_eq!(post_reload_status.current_percent, 50);
    assert_eq!(post_reload_status.previous_percent, 50);

    let metrics = hardening_metrics();
    assert!(metrics.canary_promotions_total >= 2);
    assert!(metrics.canary_rollbacks_total >= 1);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn completed_canary_rollback_retries_are_idempotent() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("completed_canary_rollback_retry");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    promote_canary(PromoteCanaryRequest { target_percent: 50 })
        .expect("promote canary before rollback");
    let request = RollbackCanaryRequest {
        reason: "lost rollback response".to_string(),
    };
    let completed = rollback_canary(request.clone()).expect("complete canary rollback");
    assert_eq!(completed.from_percent, 50);
    assert_eq!(completed.to_percent, 10);

    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    let status_before_retry = canary_rollout_status().expect("canary status before rollback retry");
    assert!(status_before_retry.promotion_gate_passed);
    let rollback_count_before_retry = hardening_metrics().canary_rollbacks_total;

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let first_retry = rollback_canary(request.clone())
        .expect("completed rollback retry must not attempt persistence");
    let second_retry =
        rollback_canary(request).expect("repeated completed rollback retry is stable");
    clear_persist_fault_injection_for_tests();

    assert_eq!(first_retry.from_percent, 10);
    assert_eq!(first_retry.to_percent, 10);
    assert_eq!(first_retry.config_version, completed.config_version);
    assert_eq!(
        first_retry.rolled_back_at_unix,
        completed.rolled_back_at_unix
    );
    assert_eq!(second_retry, first_retry);
    assert_eq!(
        canary_rollout_status().expect("canary status after rollback retries"),
        status_before_retry,
        "rollback retries must not mutate rollout state or reset promotion evidence"
    );
    assert_eq!(
        hardening_metrics().canary_rollbacks_total,
        rollback_count_before_retry,
        "rollback retries must not be counted again"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn fresh_canary_rollback_noop_does_not_require_a_state_parent_directory() {
    let _guard = lock_test_state();
    clear_state_storage_policy_overrides();

    let missing_parent = std::env::temp_dir().join(format!(
        "frost_tbtc_missing_state_parent_{}",
        std::process::id()
    ));
    let state_path = missing_parent.join("state.json");
    cleanup_test_state_artifacts(&state_path);
    let _ = std::fs::remove_dir(&missing_parent);
    std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &state_path);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    let _ = std::fs::remove_dir(&missing_parent);
    assert!(!missing_parent.exists());

    let status_before = canary_rollout_status().expect("fresh canary rollout status");
    let directory_syncs_before = state_file_parent_directory_syncs_for_tests();
    let rollback = rollback_canary(RollbackCanaryRequest {
        reason: "fresh state no-op".to_string(),
    })
    .expect("fresh-state rollback no-op");

    assert_eq!(rollback.from_percent, 10);
    assert_eq!(rollback.to_percent, 10);
    assert_eq!(rollback.config_version, status_before.config_version);
    assert_eq!(
        canary_rollout_status().expect("fresh canary status after no-op"),
        status_before
    );
    assert_eq!(
        state_file_parent_directory_syncs_for_tests(),
        directory_syncs_before,
        "an absent state file must not trigger a parent-directory sync"
    );
    assert!(
        !missing_parent.exists(),
        "a fresh-state no-op must not create persistence directories"
    );

    std::env::remove_var(TBTC_SIGNER_STATE_PATH_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    let _ = std::fs::remove_dir(&missing_parent);
    clear_state_storage_policy_overrides();
}

#[test]
fn completed_canary_rollback_retry_after_restart_repairs_directory_durability() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("completed_canary_rollback_restart_retry");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    promote_canary(PromoteCanaryRequest { target_percent: 50 })
        .expect("promote canary before rollback");
    let request = RollbackCanaryRequest {
        reason: "lost rollback response across restart".to_string(),
    };
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let rollback_error = rollback_canary(request.clone())
        .expect_err("post-rename rollback failure must remain unacknowledged");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        rollback_error,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(pending_canary_rollback_result(&request.reason).is_some());

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    assert!(
        pending_canary_rollback_result(&request.reason).is_none(),
        "the process-local pending marker must be absent after restart"
    );
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    let status_before_retry =
        canary_rollout_status().expect("reloaded rollback state before retry");
    assert_eq!(status_before_retry.current_percent, 10);
    assert_eq!(status_before_retry.previous_percent, 10);
    assert_eq!(status_before_retry.config_version, 3);
    assert!(status_before_retry.promotion_gate_passed);
    let rollback_count_before_retry = hardening_metrics().canary_rollbacks_total;
    let persisted_bytes_before_retry =
        std::fs::read(&state_path).expect("read replacement state before retry");
    let directory_syncs_before_retry = state_file_parent_directory_syncs_for_tests();

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let retry_result = rollback_canary(request);
    clear_persist_fault_injection_for_tests();
    let retry = retry_result.expect("state-based retry repairs parent-directory durability");

    assert_eq!(retry.from_percent, 10);
    assert_eq!(retry.to_percent, 10);
    assert_eq!(retry.config_version, status_before_retry.config_version);
    assert_eq!(
        retry.rolled_back_at_unix,
        status_before_retry.last_action_unix
    );
    assert_eq!(
        state_file_parent_directory_syncs_for_tests(),
        directory_syncs_before_retry.saturating_add(1),
        "the retry must sync the existing state file's parent directory"
    );
    assert_eq!(
        std::fs::read(&state_path).expect("read state after retry"),
        persisted_bytes_before_retry,
        "the directory durability repair must not rewrite the state file"
    );
    assert_eq!(
        canary_rollout_status().expect("canary status after restarted rollback retry"),
        status_before_retry,
        "the retry must not mutate rollout state or reset promotion evidence"
    );
    assert_eq!(
        hardening_metrics().canary_rollbacks_total,
        rollback_count_before_retry,
        "the retry must not count another rollback"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn emergency_rekey_persist_failure_rolls_back_and_retry_is_durable() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("emergency_rekey_persist_retry");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-emergency-rekey-persist-retry";
    ensure_interactive_dkg_session(session_id, "emergency-rekey-persist-key-group");
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        persist_engine_state_to_storage(&guard).expect("persist baseline wallet session");
    }

    let request = TriggerEmergencyRekeyRequest {
        session_id: session_id.to_string(),
        reason: "compromise containment".to_string(),
    };
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let err = trigger_emergency_rekey(request.clone())
        .expect_err("injected persist fault must fail emergency rekey");
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(err, EngineError::Internal(ref message) if message.contains("injected persist fault")),
        "unexpected error: {err:?}"
    );

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        assert!(
            guard
                .sessions
                .get(session_id)
                .expect("wallet session")
                .lifecycle
                .emergency_rekey_event
                .is_none(),
            "a failed persist must not strand an in-memory-only kill switch"
        );
    }

    trigger_emergency_rekey(request).expect("retry persists emergency rekey");
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let event = guard
            .sessions
            .get(session_id)
            .expect("reloaded wallet session")
            .lifecycle
            .emergency_rekey_event
            .as_ref()
            .expect("durable emergency rekey event");
        assert_eq!(event.reason, "compromise containment");
    }

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn emergency_rekey_different_reason_retry_repairs_pending_persistence() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("emergency_rekey_pending_different_reason");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-emergency-rekey-pending-different-reason";
    ensure_interactive_dkg_session(session_id, "emergency-rekey-different-reason-key-group");
    let original_request = TriggerEmergencyRekeyRequest {
        session_id: session_id.to_string(),
        reason: "key compromise".to_string(),
    };
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    trigger_emergency_rekey(original_request)
        .expect_err("post-replacement fault leaves a pending emergency rekey");
    clear_persist_fault_injection_for_tests();
    assert!(pending_emergency_rekey_operation(session_id).is_some());

    // A healthy full-state write makes the state durable, but must not erase the
    // original operation's cached retry result.
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        persist_engine_state_to_storage(&guard).expect("unrelated full-state write");
    }
    assert!(pending_emergency_rekey_operation(session_id).is_some());

    let changed_reason_request = TriggerEmergencyRekeyRequest {
        session_id: session_id.to_string(),
        reason: "key-compromise".to_string(),
    };
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let retry_error = trigger_emergency_rekey(changed_reason_request.clone())
        .expect_err("different-reason retry must attempt the pending durability repair");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        retry_error,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(pending_emergency_rekey_operation(session_id).is_some());

    let immutable_error = trigger_emergency_rekey(changed_reason_request)
        .expect_err("after repair, a changed reason remains an immutable-event conflict");
    assert!(matches!(
        immutable_error,
        EngineError::Validation(ref message) if message.contains("already triggered")
    ));
    assert!(pending_emergency_rekey_operation(session_id).is_none());

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let guard = state().expect("engine state").lock().expect("engine lock");
    assert_eq!(
        guard.sessions[session_id]
            .lifecycle
            .emergency_rekey_event
            .as_ref()
            .expect("durable rekey event")
            .reason,
        "key compromise"
    );
    drop(guard);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn emergency_rekey_post_replace_state_survives_immediate_process_restart() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("emergency_rekey_post_replace_restart");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-emergency-rekey-post-replace-restart";
    ensure_interactive_dkg_session(session_id, "emergency-rekey-restart-key-group");
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    trigger_emergency_rekey(TriggerEmergencyRekeyRequest {
        session_id: session_id.to_string(),
        reason: "restart-window containment".to_string(),
    })
    .expect_err("post-replacement fault must report failure before restart");
    clear_persist_fault_injection_for_tests();

    // Simulate process death before an in-process retry can flush the pending
    // registry. On filesystems where rename completed, the replacement image is
    // loaded and the kill switch remains active.
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let guard = state().expect("engine state").lock().expect("engine lock");
    assert_eq!(
        guard.sessions[session_id]
            .lifecycle
            .emergency_rekey_event
            .as_ref()
            .expect("post-replacement event survives restart")
            .reason,
        "restart-window containment"
    );
    drop(guard);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn canary_promotion_persist_failure_rolls_back_and_retry_is_durable() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("canary_promotion_persist_retry");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);

    let request = PromoteCanaryRequest { target_percent: 50 };
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let err = promote_canary(request.clone())
        .expect_err("injected persist fault must fail canary promotion");
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(err, EngineError::Internal(ref message) if message.contains("injected persist fault")),
        "unexpected error: {err:?}"
    );

    let rolled_back_status = canary_rollout_status().expect("canary status after failed persist");
    assert_eq!(rolled_back_status.current_percent, 10);
    assert_eq!(rolled_back_status.previous_percent, 10);
    assert_eq!(rolled_back_status.config_version, 1);

    let promoted = promote_canary(request).expect("retry persists canary promotion");
    assert_eq!(promoted.from_percent, 10);
    assert_eq!(promoted.to_percent, 50);
    assert_eq!(promoted.config_version, 2);

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let reloaded_status = canary_rollout_status().expect("reloaded canary status");
    assert_eq!(reloaded_status.current_percent, 50);
    assert_eq!(reloaded_status.previous_percent, 10);
    assert_eq!(reloaded_status.config_version, 2);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn canary_rollback_persist_failure_rolls_back_and_retry_is_durable() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("canary_rollback_persist_retry");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    promote_canary(PromoteCanaryRequest { target_percent: 50 })
        .expect("persist baseline canary promotion");
    let request = RollbackCanaryRequest {
        reason: "rollback persist drill".to_string(),
    };
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let err = rollback_canary(request.clone())
        .expect_err("injected persist fault must fail canary rollback");
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(err, EngineError::Internal(ref message) if message.contains("injected persist fault")),
        "unexpected error: {err:?}"
    );

    let rolled_back_status = canary_rollout_status().expect("canary status after failed rollback");
    assert_eq!(rolled_back_status.current_percent, 50);
    assert_eq!(rolled_back_status.previous_percent, 10);
    assert_eq!(rolled_back_status.config_version, 2);

    let rollback = rollback_canary(request).expect("retry persists canary rollback");
    assert_eq!(rollback.from_percent, 50);
    assert_eq!(rollback.to_percent, 10);
    assert_eq!(rollback.config_version, 3);

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let reloaded_status = canary_rollout_status().expect("reloaded rollback status");
    assert_eq!(reloaded_status.current_percent, 10);
    assert_eq!(reloaded_status.previous_percent, 10);
    assert_eq!(reloaded_status.config_version, 3);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn canary_post_rename_recovery_preserves_evidence_and_records_transitions() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("canary_post_rename_recovery_bookkeeping");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let baseline_metrics = hardening_metrics();
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    let promotion_request = PromoteCanaryRequest { target_percent: 50 };
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    promote_canary(promotion_request.clone())
        .expect_err("post-rename fault must leave the promotion pending");
    clear_persist_fault_injection_for_tests();

    assert!(pending_canary_promotion_result(50).is_some());
    assert_eq!(
        hardening_metrics().canary_promotions_total,
        baseline_metrics.canary_promotions_total,
        "an unconfirmed transition must not be counted yet"
    );
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    assert!(
        canary_rollout_status()
            .expect("active 50% stage status before recovery")
            .promotion_gate_passed,
        "operations after the rename must count toward the active 50% stage"
    );

    let promotion =
        promote_canary(promotion_request).expect("matching retry repairs promotion durability");
    assert_eq!(promotion.from_percent, 10);
    assert_eq!(promotion.to_percent, 50);
    assert!(pending_canary_promotion_result(50).is_none());
    assert!(
        canary_rollout_status()
            .expect("active 50% stage status after recovery")
            .promotion_gate_passed,
        "confirming durability must preserve current-stage evidence"
    );
    assert_eq!(
        hardening_metrics().canary_promotions_total,
        baseline_metrics.canary_promotions_total.saturating_add(1),
        "recovery must record the completed promotion exactly once"
    );

    let rollback_request = RollbackCanaryRequest {
        reason: "post-rename recovery bookkeeping".to_string(),
    };
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    rollback_canary(rollback_request.clone())
        .expect_err("post-rename fault must leave the rollback pending");
    clear_persist_fault_injection_for_tests();

    assert!(pending_canary_rollback_result("post-rename recovery bookkeeping").is_some());
    assert_eq!(
        hardening_metrics().canary_rollbacks_total,
        baseline_metrics.canary_rollbacks_total,
        "an unconfirmed rollback must not be counted yet"
    );
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    assert!(
        canary_rollout_status()
            .expect("active rolled-back stage status before recovery")
            .promotion_gate_passed,
        "operations after the rename must count toward the rolled-back stage"
    );

    let rollback =
        rollback_canary(rollback_request).expect("matching retry repairs rollback durability");
    assert_eq!(rollback.from_percent, 50);
    assert_eq!(rollback.to_percent, 10);
    assert!(pending_canary_rollback_result("post-rename recovery bookkeeping").is_none());
    assert!(
        canary_rollout_status()
            .expect("active rolled-back stage status after recovery")
            .promotion_gate_passed,
        "confirming rollback durability must preserve current-stage evidence"
    );
    let recovered_metrics = hardening_metrics();
    assert_eq!(
        recovered_metrics.canary_promotions_total,
        baseline_metrics.canary_promotions_total.saturating_add(1)
    );
    assert_eq!(
        recovered_metrics.canary_rollbacks_total,
        baseline_metrics.canary_rollbacks_total.saturating_add(1),
        "recovery must record the completed rollback exactly once"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn lifecycle_post_rename_persist_failures_remain_fail_closed_and_retry_durably() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("lifecycle_post_rename_retry");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let rekey_session = "session-emergency-rekey-post-rename";
    ensure_interactive_dkg_session(rekey_session, "emergency-rekey-post-rename-key-group");
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        persist_engine_state_to_storage(&guard).expect("persist baseline wallet session");
    }
    let rekey_request = TriggerEmergencyRekeyRequest {
        session_id: rekey_session.to_string(),
        reason: "post-rename containment".to_string(),
    };
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let rekey_error = trigger_emergency_rekey(rekey_request.clone())
        .expect_err("post-rename fault must report emergency rekey failure");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        rekey_error,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(pending_emergency_rekey_result(rekey_session, "post-rename containment").is_some());
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        assert!(
            guard.sessions[rekey_session]
                .lifecycle
                .emergency_rekey_event
                .is_some(),
            "a replaced state file keeps the in-memory kill switch fail closed"
        );
    }
    let rekey_result =
        trigger_emergency_rekey(rekey_request).expect("same rekey retry flushes pending state");
    assert_eq!(rekey_result.session_id, rekey_session);
    assert!(pending_emergency_rekey_result(rekey_session, "post-rename containment").is_none());

    let promotion_request = PromoteCanaryRequest { target_percent: 50 };
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let promotion_error = promote_canary(promotion_request.clone())
        .expect_err("post-rename fault must report promotion failure");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        promotion_error,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(pending_canary_promotion_result(50).is_some());
    let pending_promotion_status = canary_rollout_status().expect("pending promotion status");
    assert_eq!(pending_promotion_status.current_percent, 50);
    assert_eq!(pending_promotion_status.config_version, 2);
    assert!(!pending_promotion_status.promotion_gate_passed);
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    promote_canary(promotion_request.clone())
        .expect_err("a failed pending promotion flush must not report success");
    clear_persist_fault_injection_for_tests();
    assert!(pending_canary_promotion_result(50).is_some());
    let promotion_result =
        promote_canary(promotion_request).expect("same promotion retry flushes pending state");
    assert_eq!(promotion_result.from_percent, 10);
    assert_eq!(promotion_result.to_percent, 50);
    assert_eq!(promotion_result.config_version, 2);
    assert!(pending_canary_promotion_result(50).is_none());

    let rollback_request = RollbackCanaryRequest {
        reason: "post-rename rollback".to_string(),
    };
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let rollback_error = rollback_canary(rollback_request.clone())
        .expect_err("post-rename fault must report rollback failure");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        rollback_error,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(pending_canary_rollback_result("post-rename rollback").is_some());
    let pending_rollback_status = canary_rollout_status().expect("pending rollback status");
    assert_eq!(pending_rollback_status.current_percent, 10);
    assert_eq!(pending_rollback_status.config_version, 3);
    assert!(!pending_rollback_status.promotion_gate_passed);
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    rollback_canary(rollback_request.clone())
        .expect_err("a failed pending rollback flush must not report success");
    clear_persist_fault_injection_for_tests();
    assert!(pending_canary_rollback_result("post-rename rollback").is_some());
    let rollback_result =
        rollback_canary(rollback_request).expect("same rollback retry flushes pending state");
    assert_eq!(rollback_result.from_percent, 50);
    assert_eq!(rollback_result.to_percent, 10);
    assert_eq!(rollback_result.config_version, 3);
    assert!(pending_canary_rollback_result("post-rename rollback").is_none());

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let guard = state().expect("engine state").lock().expect("engine lock");
    assert!(guard.sessions[rekey_session]
        .lifecycle
        .emergency_rekey_event
        .is_some());
    assert_eq!(guard.canary_rollout.current_percent, 10);
    assert_eq!(guard.canary_rollout.config_version, 3);
    drop(guard);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn canary_promotion_halts_when_policy_reject_rate_exceeds_gate() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV, "1");
    record_hardening_operation_latency(HardeningOperation::InteractiveRound1, 1);
    record_hardening_operation_latency(HardeningOperation::InteractiveRound2, 1);
    record_hardening_operation_latency(HardeningOperation::InteractiveAggregate, 1);

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let rejected = build_taproot_tx(build_policy_test_request("session-canary-gate-fail"))
        .expect_err("expected policy rejection");
    let EngineError::SigningPolicyRejected { reason_code, .. } = rejected else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "script_class_not_allowlisted");

    std::env::set_var(TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV, "0");
    let err = promote_canary(PromoteCanaryRequest { target_percent: 50 })
        .expect_err("expected canary gate rejection");
    let EngineError::LifecyclePolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "canary_slo_gate_failed");
}

#[test]
fn canary_promotion_halts_when_interactive_latency_exceeds_gate() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("canary_interactive_latency_gate");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV, "1");
    std::env::set_var(TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND1_P95_MS_ENV, "10");
    std::env::set_var(TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND2_P95_MS_ENV, "10");
    std::env::set_var(
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_AGGREGATE_P95_MS_ENV,
        "20",
    );
    record_hardening_operation_latency(HardeningOperation::InteractiveRound1, 11);
    record_hardening_operation_latency(HardeningOperation::InteractiveRound2, 12);
    record_hardening_operation_latency(HardeningOperation::InteractiveAggregate, 21);
    record_canary_policy_outcome(false);

    let failures = canary_promotion_gate_failures();
    assert_eq!(
        failures,
        vec![
            "interactive_round1 p95 latency [11ms] exceeds canary gate [10ms]",
            "interactive_round2 p95 latency [12ms] exceeds canary gate [10ms]",
            "interactive_aggregate p95 latency [21ms] exceeds canary gate [20ms]",
        ]
    );

    let err = promote_canary(PromoteCanaryRequest { target_percent: 50 })
        .expect_err("interactive signing latency must block canary promotion");
    let EngineError::LifecyclePolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "canary_slo_gate_failed");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn canary_interactive_knobs_override_legacy_threshold_aliases() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV, "10");
    std::env::set_var(TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS_ENV, "20");
    assert_eq!(canary_max_interactive_round1_p95_ms(), 10);
    assert_eq!(canary_max_interactive_round2_p95_ms(), 10);
    assert_eq!(canary_max_interactive_aggregate_p95_ms(), 20);

    std::env::set_var(TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND1_P95_MS_ENV, "11");
    std::env::set_var(TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND2_P95_MS_ENV, "12");
    std::env::set_var(
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_AGGREGATE_P95_MS_ENV,
        "21",
    );
    assert_eq!(canary_max_interactive_round1_p95_ms(), 11);
    assert_eq!(canary_max_interactive_round2_p95_ms(), 12);
    assert_eq!(canary_max_interactive_aggregate_p95_ms(), 21);

    // A malformed or non-positive explicit knob must not shadow a valid
    // legacy alias. Operators can therefore recover from a bad new-name
    // value without silently falling back to the much looser built-in
    // latency threshold.
    std::env::set_var(
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND1_P95_MS_ENV,
        "not-a-number",
    );
    std::env::set_var(TBTC_SIGNER_CANARY_MAX_INTERACTIVE_ROUND2_P95_MS_ENV, "0");
    std::env::set_var(
        TBTC_SIGNER_CANARY_MAX_INTERACTIVE_AGGREGATE_P95_MS_ENV,
        "not-a-number",
    );
    assert_eq!(canary_max_interactive_round1_p95_ms(), 10);
    assert_eq!(canary_max_interactive_round2_p95_ms(), 10);
    assert_eq!(canary_max_interactive_aggregate_p95_ms(), 20);

    std::env::set_var(TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV, "0");
    assert_eq!(canary_min_samples(), TBTC_SIGNER_DEFAULT_CANARY_MIN_SAMPLES);
    std::env::set_var(TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV, "256");
    assert_eq!(canary_min_samples(), 256);

    std::env::set_var(TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV, "257");
    assert_eq!(canary_min_samples(), TBTC_SIGNER_MAX_CANARY_MIN_SAMPLES);
    std::env::set_var(TBTC_SIGNER_CANARY_MIN_POLICY_SAMPLES_ENV, "0");
    assert_eq!(
        canary_min_policy_samples(),
        TBTC_SIGNER_MAX_CANARY_MIN_SAMPLES,
        "a zero policy minimum retains the interactive fallback",
    );
    std::env::set_var(TBTC_SIGNER_CANARY_MIN_POLICY_SAMPLES_ENV, "257");
    assert_eq!(
        canary_min_policy_samples(),
        TBTC_SIGNER_MAX_CANARY_MIN_SAMPLES
    );
    std::env::set_var(TBTC_SIGNER_CANARY_MAX_SAMPLE_AGE_SECONDS_ENV, "604801");
    assert_eq!(
        canary_max_sample_age_seconds(),
        TBTC_SIGNER_MAX_CANARY_SAMPLE_AGE_SECONDS
    );
}

#[test]
fn canary_policy_evidence_minimum_is_independent_from_interactive_minimum() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV, "2");
    assert_eq!(
        canary_min_policy_samples(),
        2,
        "an absent policy minimum must preserve the prior fail-closed minimum",
    );
    std::env::set_var(TBTC_SIGNER_CANARY_MIN_POLICY_SAMPLES_ENV, "1");

    record_hardening_operation_latency(HardeningOperation::InteractiveRound1, 1);
    record_hardening_operation_latency(HardeningOperation::InteractiveRound2, 1);
    record_hardening_operation_latency(HardeningOperation::InteractiveAggregate, 1);
    record_canary_policy_outcome(false);

    assert_eq!(
        canary_promotion_gate_failures(),
        vec![
            "interactive_round1 fresh successful samples [1] below canary minimum [2]",
            "interactive_round2 fresh successful samples [1] below canary minimum [2]",
            "interactive_aggregate fresh successful samples [1] below canary minimum [2]",
        ],
        "the lower policy minimum must not weaken interactive evidence",
    );

    record_hardening_operation_latency(HardeningOperation::InteractiveRound1, 1);
    record_hardening_operation_latency(HardeningOperation::InteractiveRound2, 1);
    record_hardening_operation_latency(HardeningOperation::InteractiveAggregate, 1);
    assert!(
        canary_promotion_gate_failures().is_empty(),
        "one policy outcome should remain sufficient after interactive evidence reaches its own minimum",
    );
}

#[test]
fn canary_evidence_freshness_fails_closed_when_clock_precedes_observation() {
    let evaluated_at = Instant::now();
    let observed_at = evaluated_at + Duration::from_secs(1);

    let mut latency = HardeningLatencyTracker::default();
    latency.record_at(7, observed_at);
    assert_eq!(latency.fresh_sample_count(evaluated_at, 60), 0);
    assert_eq!(latency.fresh_p95_ms(evaluated_at, 60), 0);

    let mut policy = HardeningPolicyOutcomeTracker::default();
    policy.record_at(false, observed_at);
    assert_eq!(policy.fresh_snapshot(evaluated_at, 60), (0, 0));
}

#[test]
fn canary_promotion_requires_fresh_minimum_evidence_after_restart() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("canary_restart_evidence_gate");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let empty = canary_rollout_status().expect("empty-evidence status");
    assert!(!empty.promotion_gate_passed);
    assert_eq!(empty.gate_failures, canary_missing_evidence_gate_failures());

    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    promote_canary(PromoteCanaryRequest { target_percent: 50 })
        .expect("fresh minimum evidence promotes to 50%");
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);
    assert!(
        canary_rollout_status()
            .expect("50% stage evidence")
            .promotion_gate_passed
    );

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let restarted = canary_rollout_status().expect("status after process restart");
    assert_eq!(restarted.current_percent, 50);
    assert!(!restarted.promotion_gate_passed);
    assert_eq!(
        restarted.gate_failures,
        canary_missing_evidence_gate_failures()
    );
    let error = promote_canary(PromoteCanaryRequest {
        target_percent: 100,
    })
    .expect_err("persisted rollout state must not promote on empty post-restart telemetry");
    assert!(matches!(
        error,
        EngineError::LifecyclePolicyRejected { ref reason_code, .. }
            if reason_code == "canary_slo_gate_failed"
    ));

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn concurrent_canary_promotions_cannot_reuse_prior_stage_evidence() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);

    struct CanaryPromotionLockReleaseGuard;
    impl Drop for CanaryPromotionLockReleaseGuard {
        fn drop(&mut self) {
            release_canary_promotion_lock_for_tests();
        }
    }

    arm_canary_promotion_lock_hold_for_tests();
    let release_guard = CanaryPromotionLockReleaseGuard;
    let promote_50 =
        std::thread::spawn(|| promote_canary(PromoteCanaryRequest { target_percent: 50 }));

    let deadline = Instant::now() + Duration::from_secs(5);
    while !canary_promotion_lock_held_for_tests() {
        assert!(
            Instant::now() < deadline,
            "50% promotion did not acquire the rollout-state lock"
        );
        std::thread::yield_now();
    }

    let promote_100 = std::thread::spawn(|| {
        promote_canary(PromoteCanaryRequest {
            target_percent: 100,
        })
    });
    while canary_promotion_lock_attempts_for_tests() < 2 {
        assert!(
            Instant::now() < deadline,
            "100% promotion did not reach the rollout-state lock"
        );
        std::thread::yield_now();
    }

    release_canary_promotion_lock_for_tests();
    drop(release_guard);
    let first = promote_50
        .join()
        .expect("50% promotion thread")
        .expect("50% promotion succeeds with stage-10 evidence");
    assert_eq!(first.to_percent, 50);

    let second = promote_100
        .join()
        .expect("100% promotion thread")
        .expect_err("100% promotion must wait for fresh stage-50 evidence");
    assert!(matches!(
        second,
        EngineError::LifecyclePolicyRejected { ref reason_code, .. }
            if reason_code == "canary_slo_gate_failed"
    ));
    let status = canary_rollout_status().expect("rollout status after concurrent requests");
    assert_eq!(status.current_percent, 50);
    assert!(!status.promotion_gate_passed);
}

#[test]
fn canary_promotion_ignores_samples_older_than_the_configured_window() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    seed_canary_promotion_evidence_for_tests(1, 1, 1, 0);

    std::env::set_var(TBTC_SIGNER_CANARY_MAX_SAMPLE_AGE_SECONDS_ENV, "1");
    let stale_at = Instant::now()
        .checked_sub(Duration::from_secs(2))
        .expect("monotonic clock must support a two-second test offset");
    {
        let mut telemetry = hardening_telemetry_state()
            .lock()
            .expect("hardening telemetry lock");
        for sample in &mut telemetry.canary_interactive_round1_latency.samples {
            sample.observed_at = stale_at;
        }
        for sample in &mut telemetry.canary_interactive_round2_latency.samples {
            sample.observed_at = stale_at;
        }
        for sample in &mut telemetry.canary_interactive_aggregate_latency.samples {
            sample.observed_at = stale_at;
        }
        for sample in &mut telemetry.canary_policy_outcomes.samples {
            sample.observed_at = stale_at;
        }
    }

    let metrics = hardening_metrics();
    assert_eq!(
        metrics.interactive_round1_latency_samples,
        canary_min_samples()
    );
    assert_eq!(
        metrics.interactive_round2_latency_samples,
        canary_min_samples()
    );
    assert_eq!(
        metrics.interactive_aggregate_latency_samples,
        canary_min_samples()
    );
    assert_eq!(metrics.interactive_round1_latency_p95_ms, 1);
    assert_eq!(metrics.interactive_round2_latency_p95_ms, 1);
    assert_eq!(metrics.interactive_aggregate_latency_p95_ms, 1);
    assert_eq!(
        canary_promotion_gate_failures(),
        canary_missing_evidence_gate_failures()
    );
}

#[test]
fn canary_evidence_reset_preserves_abi_latency_metrics() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_CANARY_MIN_SAMPLES_ENV, "1");

    seed_canary_promotion_evidence_for_tests(7, 8, 9, 0);
    let before = hardening_metrics();
    assert!(canary_promotion_gate_failures().is_empty());

    reset_canary_promotion_evidence();

    let after = hardening_metrics();
    assert_eq!(after.interactive_round1_latency_samples, 1);
    assert_eq!(after.interactive_round1_latency_p95_ms, 7);
    assert_eq!(after.interactive_round2_latency_samples, 1);
    assert_eq!(after.interactive_round2_latency_p95_ms, 8);
    assert_eq!(after.interactive_aggregate_latency_samples, 1);
    assert_eq!(after.interactive_aggregate_latency_p95_ms, 9);
    assert_eq!(
        (
            after.interactive_round1_latency_samples,
            after.interactive_round1_latency_p95_ms,
            after.interactive_round2_latency_samples,
            after.interactive_round2_latency_p95_ms,
            after.interactive_aggregate_latency_samples,
            after.interactive_aggregate_latency_p95_ms,
        ),
        (
            before.interactive_round1_latency_samples,
            before.interactive_round1_latency_p95_ms,
            before.interactive_round2_latency_samples,
            before.interactive_round2_latency_p95_ms,
            before.interactive_aggregate_latency_samples,
            before.interactive_aggregate_latency_p95_ms,
        ),
        "rollout evidence reset must not change established ABI-3 metrics",
    );
    assert_eq!(
        canary_promotion_gate_failures(),
        canary_missing_evidence_gate_failures(),
        "the separate promotion window must still reset fail closed",
    );
}

#[test]
fn canary_promotion_ignores_interactive_latency_from_the_prior_stage() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    // Model an operation that began in the current rollout stage, completed
    // while promotion reset the evidence window, and dropped its success guard
    // only after the new stage began collecting samples.
    let mut operation =
        HardeningOperationLatencyGuard::success_only(HardeningOperation::InteractiveRound1);
    reset_canary_promotion_evidence();
    operation.mark_success();
    drop(operation);

    assert_eq!(
        hardening_metrics().interactive_round1_latency_samples,
        1,
        "the completed call remains visible through the ABI-3 rolling metric",
    );
    let telemetry = hardening_telemetry_state()
        .lock()
        .expect("hardening telemetry lock");
    assert_eq!(
        telemetry.canary_interactive_round1_latency.sample_count(),
        0,
        "a completion from the prior stage must not enter new-stage evidence",
    );
}

#[test]
fn idempotent_build_replays_do_not_dilute_canary_policy_outcomes() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let request = build_policy_test_request("session-canary-policy-cache-dilution");
    build_taproot_tx(request.clone()).expect("first artifact decision");
    for _ in 0..HARDENING_LATENCY_SAMPLE_WINDOW {
        build_taproot_tx(request.clone()).expect("idempotent build replay");
    }

    let telemetry = hardening_telemetry_state()
        .lock()
        .expect("hardening telemetry lock");
    let (sample_count, rejected_count) = telemetry
        .canary_policy_outcomes
        .fresh_snapshot(Instant::now(), canary_max_sample_age_seconds());
    assert_eq!(sample_count, 1);
    assert_eq!(rejected_count, 0);
}

#[test]
fn emergency_rekey_blocks_finalize_and_build_taproot_tx_for_session() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-emergency-rekey-finalize";
    ensure_interactive_dkg_session(session_id, "emergency-rekey-key-group");
    trigger_emergency_rekey(TriggerEmergencyRekeyRequest {
        session_id: session_id.to_string(),
        reason: "compromise containment".to_string(),
    })
    .expect("trigger emergency rekey");

    let build_err = build_taproot_tx(build_policy_test_request(session_id))
        .expect_err("expected build tx emergency rekey rejection");
    let EngineError::LifecyclePolicyRejected { reason_code, .. } = build_err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "emergency_rekey_required");
}

#[test]
fn build_taproot_tx_rate_limiter_uses_token_bucket_refill() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    std::env::set_var(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV, "2");
    configure_required_signing_policy_limits_for_tests();

    {
        let mut limiter = build_tx_rate_limiter_state()
            .lock()
            .expect("build tx rate limiter lock");
        limiter.last_refill_unix = now_unix().saturating_sub(1);
        limiter.token_microunits = 0;
        limiter.configured_rate_limit_per_minute = 2;
    }

    let rejected = build_taproot_tx(build_policy_test_request("session-rate-limited-initial"))
        .expect_err("expected rate-limit rejection with sub-token refill");
    let EngineError::SigningPolicyRejected { reason_code, .. } = rejected else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "rate_limit_per_minute_exceeded");

    {
        let mut limiter = build_tx_rate_limiter_state()
            .lock()
            .expect("build tx rate limiter lock");
        limiter.last_refill_unix = now_unix().saturating_sub(30);
        limiter.token_microunits = 0;
        limiter.configured_rate_limit_per_minute = 2;
    }

    let allowed = build_taproot_tx(build_policy_test_request("session-rate-limited-refill"));
    assert!(allowed.is_ok(), "expected one token after 30s refill");

    let rejected_again =
        build_taproot_tx(build_policy_test_request("session-rate-limited-followup"))
            .expect_err("expected immediate follow-up rejection without full refill");
    let EngineError::SigningPolicyRejected { reason_code, .. } = rejected_again else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "rate_limit_per_minute_exceeded");

    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_cache_hit_rechecks_signing_policy_firewall() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let request = build_policy_test_request("session-build-tx-cache-policy-recheck");

    let first_result = build_taproot_tx(request.clone()).expect("first build tx");
    assert!(!first_result.tx_hex.is_empty());

    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
    let err = build_taproot_tx(request).expect_err("expected cache-hit firewall policy rejection");
    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "script_class_not_allowlisted");

    let metrics = hardening_metrics();
    assert_eq!(metrics.build_taproot_tx_calls_total, 2);
    assert_eq!(metrics.build_taproot_tx_success_total, 1);
    assert_eq!(metrics.build_taproot_tx_policy_reject_total, 1);

    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_cached_retry_does_not_charge_rate_limit() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    std::env::set_var(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV, "1");
    configure_required_signing_policy_limits_for_tests();

    let request = build_policy_test_request("session-build-tx-cache-rate-limit");

    let first_result = build_taproot_tx(request.clone()).expect("first build tx");
    assert!(!first_result.tx_hex.is_empty());

    {
        let mut limiter = build_tx_rate_limiter_state()
            .lock()
            .expect("build tx rate limiter lock");
        limiter.last_refill_unix = now_unix();
        limiter.token_microunits = 0;
        limiter.configured_rate_limit_per_minute = 1;
    }

    let retry_result = build_taproot_tx(request).expect("cached retry must not rate-limit");
    assert_eq!(first_result, retry_result);

    let rejected = build_taproot_tx(build_policy_test_request("session-build-tx-rate-limit-new"))
        .expect_err("new build tx should still be rate-limited");
    let EngineError::SigningPolicyRejected { reason_code, .. } = rejected else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "rate_limit_per_minute_exceeded");

    clear_state_storage_policy_overrides();
}

#[cfg(unix)]
fn wait_for_file(path: &Path, timeout: Duration) -> bool {
    let start = Instant::now();
    while start.elapsed() < timeout {
        if path.exists() {
            return true;
        }
        thread::sleep(Duration::from_millis(50));
    }
    path.exists()
}

#[cfg(unix)]
struct LockHelperProcessGuard {
    child: Option<std::process::Child>,
    release_path: PathBuf,
}

#[cfg(unix)]
impl LockHelperProcessGuard {
    fn new(child: std::process::Child, release_path: PathBuf) -> Self {
        Self {
            child: Some(child),
            release_path,
        }
    }

    fn signal_release(&self) {
        let _ = std::fs::write(&self.release_path, b"release");
    }

    fn wait_for_success(mut self) {
        self.signal_release();
        let mut child = self.child.take().expect("helper child should be present");
        let child_status = child.wait().expect("wait for lock helper process");
        assert!(
            child_status.success(),
            "lock helper process failed with status: {child_status}"
        );
    }
}

#[cfg(unix)]
impl Drop for LockHelperProcessGuard {
    fn drop(&mut self) {
        self.signal_release();

        let Some(mut child) = self.child.take() else {
            return;
        };

        let start = Instant::now();
        while start.elapsed() < Duration::from_secs(2) {
            match child.try_wait() {
                Ok(Some(_)) => return,
                Ok(None) => thread::sleep(Duration::from_millis(50)),
                Err(_) => break,
            }
        }

        let _ = child.kill();
        let _ = child.wait();
    }
}

fn build_attempt_context(
    session_id: &str,
    message_hex: &str,
    attempt_number: u32,
    coordinator_identifier: u16,
    included_participants: Vec<u16>,
) -> AttemptContext {
    let canonical_included_participants =
        canonicalize_included_participants(&included_participants)
            .expect("canonical included participants");
    let message_bytes = hex::decode(message_hex).expect("message hex");
    let message_digest_hex = hash_hex(&message_bytes);
    let included_participants_fingerprint =
        roast_included_participants_fingerprint_hex(&canonical_included_participants)
            .expect("included participants fingerprint");
    let attempt_id = roast_attempt_id_hex(
        session_id,
        &message_digest_hex,
        attempt_number,
        coordinator_identifier,
        &included_participants_fingerprint,
    )
    .expect("attempt id");

    AttemptContext {
        attempt_number,
        coordinator_identifier,
        included_participants,
        included_participants_fingerprint,
        attempt_id,
    }
}

// Resolves the key group the engine will use when validating attempt
// contexts for `session_id`: the session's dealer-DKG key group when
// the session exists, otherwise a deterministic per-session stand-in
// for tests that exercise `validate_attempt_context` directly without
// creating a session. Construction and validation must use the same
// resolver or the RFC-21 Annex A seed derivation diverges.
fn attempt_context_key_group_for_tests(session_id: &str) -> String {
    if let Ok(state) = state() {
        if let Ok(guard) = state.lock() {
            if let Some(key_group) = guard
                .sessions
                .get(session_id)
                .and_then(|session| session.dkg.result.as_ref())
                .map(|dkg| dkg.key_group.clone())
            {
                return key_group;
            }
        }
    }

    format!("test-key-group:{session_id}")
}

fn build_deterministic_attempt_context(
    session_id: &str,
    message_hex: &str,
    attempt_number: u32,
    included_participants: Vec<u16>,
) -> AttemptContext {
    let canonical_included_participants =
        canonicalize_included_participants(&included_participants)
            .expect("canonical included participants");
    let message_bytes = hex::decode(message_hex).expect("message hex");
    let key_group = attempt_context_key_group_for_tests(session_id);
    // RFC-21 seed input: the padded raw message, not the SHA256
    // transcript digest -- mirroring the Go layer's derivation.
    let attempt_seed = roast_attempt_shuffle_seed(
        &key_group,
        session_id,
        &rfc21_message_digest(&message_bytes).expect("rfc21 message digest"),
    )
    .expect("attempt shuffle seed");
    assert!(
        attempt_number >= 1,
        "attempt_number is the 1-based wire encoding",
    );
    let coordinator_identifier = select_coordinator_identifier(
        &canonical_included_participants,
        attempt_seed,
        attempt_number - 1,
    )
    .expect("deterministic coordinator");

    build_attempt_context(
        session_id,
        message_hex,
        attempt_number,
        coordinator_identifier,
        included_participants,
    )
}

#[test]
fn roast_attempt_context_hash_vectors_match_expected_values() {
    let included_participants_fingerprint = roast_included_participants_fingerprint_hex(&[1, 3, 5])
        .expect("included participants fingerprint");
    assert_eq!(
        included_participants_fingerprint,
        "0c9258935f0a30c065befcd746cb1564e9f3c91936c0f0f1c78853fa2d6713dc"
    );

    let attempt_id = roast_attempt_id_hex(
        "vector-session-1",
        "5f78c33274e43fa9de5659265c1d917e25c03722dcb0b8d27db8d5feaa813953",
        7,
        3,
        &included_participants_fingerprint,
    )
    .expect("attempt id");
    assert_eq!(
        attempt_id,
        "dbc7a4df9bc3ef8dee3a9f5a47ff519e22e8d6f9b0461dd415077176e4e6ee95"
    );
}

#[test]
fn derive_interactive_attempt_context_matches_standalone_derivations() {
    let _guard = lock_test_state(); // hermetic env: development profile, provenance gate off
    let session_id = "derive-session-1";
    let key_group = "derive-key-group";
    let message_hex = "77".repeat(32); // 32-byte signing digest
    let message_bytes = hex::decode(&message_hex).expect("message hex");
    let threshold = 2u16;
    let attempt_number = 3u32; // 1-based wire
    let included = vec![5u16, 1, 3]; // unsorted -> exercises canonical (ascending) ordering

    let result = derive_interactive_attempt_context(DeriveInteractiveAttemptContextRequest {
        session_id: session_id.to_string(),
        message_hex: message_hex.clone(),
        key_group: key_group.to_string(),
        threshold,
        attempt_number,
        included_participants: included.clone(),
    })
    .expect("derivation should succeed");

    let canonical = canonicalize_included_participants(&included).expect("canonical");
    assert_eq!(canonical, vec![1, 3, 5]);
    assert_eq!(result.attempt_context.included_participants, canonical);
    assert_eq!(result.attempt_context.attempt_number, attempt_number);

    // Coordinator matches the standalone shuffle selection (0-based attempt).
    let seed = roast_attempt_shuffle_seed(
        key_group,
        session_id,
        &rfc21_message_digest(&message_bytes).expect("rfc21 digest"),
    )
    .expect("seed");
    let expected_coordinator =
        select_coordinator_identifier(&canonical, seed, attempt_number - 1).expect("coordinator");
    assert_eq!(
        result.attempt_context.coordinator_identifier,
        expected_coordinator
    );

    // Fingerprint + attempt_id match the standalone domain-separated derivations.
    let expected_fingerprint =
        roast_included_participants_fingerprint_hex(&canonical).expect("fingerprint");
    assert_eq!(
        result.attempt_context.included_participants_fingerprint,
        expected_fingerprint
    );
    let expected_attempt_id = roast_attempt_id_hex(
        session_id,
        &hash_hex(&message_bytes),
        attempt_number,
        expected_coordinator,
        &expected_fingerprint,
    )
    .expect("attempt id");
    assert_eq!(result.attempt_context.attempt_id, expected_attempt_id);

    // The derived context is accepted by the same strict validator open runs.
    validate_attempt_context(
        session_id,
        key_group,
        &message_bytes,
        &hash_hex(&message_bytes),
        threshold,
        Some(&result.attempt_context),
        true,
    )
    .expect("derived context must satisfy strict validation");

    // One FROST identifier per participant, canonical order + encoding.
    assert_eq!(result.frost_identifiers.len(), canonical.len());
    for (entry, participant) in result.frost_identifiers.iter().zip(canonical.iter()) {
        assert_eq!(entry.participant_identifier, *participant);
        let expected = frost_identifier_to_go_string(
            participant_identifier_to_frost_identifier(*participant).expect("frost id"),
        );
        assert_eq!(entry.frost_identifier, expected);
    }
}

#[test]
fn derive_interactive_attempt_context_is_deterministic() {
    let _guard = lock_test_state();
    let request = DeriveInteractiveAttemptContextRequest {
        session_id: "s".to_string(),
        message_hex: "ab".repeat(32),
        key_group: "kg".to_string(),
        threshold: 2,
        attempt_number: 1,
        included_participants: vec![1, 2, 3],
    };
    let first = derive_interactive_attempt_context(request.clone()).expect("first");
    let second = derive_interactive_attempt_context(request).expect("second");
    assert_eq!(first, second);
}

#[test]
fn derive_interactive_attempt_context_rejects_invalid_inputs() {
    let _guard = lock_test_state();
    let base = DeriveInteractiveAttemptContextRequest {
        session_id: "s".to_string(),
        message_hex: "cd".repeat(32),
        key_group: "kg".to_string(),
        threshold: 2,
        attempt_number: 1,
        included_participants: vec![1, 2, 3],
    };

    let mut empty_message = base.clone();
    empty_message.message_hex = String::new();
    assert!(derive_interactive_attempt_context(empty_message).is_err());

    // session_id is validated (and hashed into attempt_id), so an empty/malformed
    // one must fail here exactly as interactive_session_open's validate_session_id
    // would reject it.
    let mut empty_session = base.clone();
    empty_session.session_id = String::new();
    assert!(derive_interactive_attempt_context(empty_session).is_err());

    let mut zero_attempt = base.clone();
    zero_attempt.attempt_number = 0;
    assert!(derive_interactive_attempt_context(zero_attempt).is_err());

    // threshold == 0 is vacuously >= len, but interactive_session_open rejects
    // it, so the helper must too rather than hand back a context open refuses.
    let mut zero_threshold = base.clone();
    zero_threshold.threshold = 0;
    assert!(derive_interactive_attempt_context(zero_threshold).is_err());

    let mut threshold_too_large = base.clone();
    threshold_too_large.threshold = 5;
    threshold_too_large.included_participants = vec![1, 2];
    assert!(derive_interactive_attempt_context(threshold_too_large).is_err());

    let mut duplicate_participant = base.clone();
    duplicate_participant.included_participants = vec![1, 2, 2];
    assert!(derive_interactive_attempt_context(duplicate_participant).is_err());

    let mut no_participants = base;
    no_participants.included_participants = vec![];
    assert!(derive_interactive_attempt_context(no_participants).is_err());
}

#[test]
fn derive_interactive_attempt_context_enforces_provenance_gate() {
    let _guard = lock_test_state();
    // Enable the provenance gate with no attestation configured: the helper must
    // fail closed exactly like interactive_session_open's front door, never
    // returning a derived context on an unattested engine.
    std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");

    let result = derive_interactive_attempt_context(DeriveInteractiveAttemptContextRequest {
        session_id: "s".to_string(),
        message_hex: "ef".repeat(32),
        key_group: "kg".to_string(),
        threshold: 2,
        attempt_number: 1,
        included_participants: vec![1, 2, 3],
    });
    assert!(matches!(
        result,
        Err(EngineError::ProvenanceGateRejected { .. })
    ));
}

#[test]
fn formal_verification_roast_attempt_context_shared_vectors_match_expected_values() {
    let vector_suite = load_attempt_context_vector_suite();
    assert_eq!(vector_suite.schema_version, "roast-attempt-context-v1");
    assert_eq!(
        vector_suite.hash_domains.included_participants_fingerprint,
        ROAST_INCLUDED_PARTICIPANTS_FINGERPRINT_DOMAIN
    );
    assert_eq!(
        vector_suite.hash_domains.attempt_id,
        ROAST_ATTEMPT_ID_DOMAIN
    );
    assert!(
        !vector_suite.vectors.is_empty(),
        "expected at least one shared attempt-context vector"
    );

    for vector in vector_suite.vectors {
        let canonical_participants =
            canonicalize_included_participants(&vector.included_participants)
                .expect("vector participants should canonicalize");
        let included_participants_fingerprint =
            roast_included_participants_fingerprint_hex(&canonical_participants)
                .expect("included participants fingerprint");
        assert_eq!(
            included_participants_fingerprint,
            vector
                .expected_included_participants_fingerprint
                .to_ascii_lowercase(),
            "included participants fingerprint mismatch for vector [{}]",
            vector.id
        );

        let attempt_id = roast_attempt_id_hex(
            &vector.session_id,
            &vector.message_digest_hex.to_ascii_lowercase(),
            vector.attempt_number,
            vector.coordinator_identifier,
            &included_participants_fingerprint,
        )
        .expect("attempt id");
        assert_eq!(
            attempt_id,
            vector.expected_attempt_id.to_ascii_lowercase(),
            "attempt id mismatch for vector [{}]",
            vector.id
        );
    }
}

fn participant_set_strategy() -> impl Strategy<Value = Vec<u16>> {
    prop::collection::btree_set(1_u16..=1024_u16, 2..=16)
        .prop_map(|participants| participants.into_iter().collect())
}

proptest! {
    #![proptest_config(ProptestConfig::with_cases(64))]

    #[test]
    fn formal_verification_attempt_context_is_stable_under_participant_permutations(
        session_suffix in any::<u32>(),
        attempt_number in 1_u32..=16_u32,
        participants in participant_set_strategy(),
        // RFC-21 attempt contexts only bind 32-byte signing digests
        // (rfc21_message_digest rejects longer messages), so the
        // strategy stays within that bound.
        message_bytes in prop::collection::vec(any::<u8>(), 1..=32),
    ) {
        let session_id = format!("formal-attempt-session-{session_suffix}");
        let message_hex = hex::encode(message_bytes);
        let mut reversed_participants = participants.clone();
        reversed_participants.reverse();

        let canonical_attempt_context = build_deterministic_attempt_context(
            &session_id,
            &message_hex,
            attempt_number,
            participants.clone(),
        );
        let permuted_attempt_context = build_deterministic_attempt_context(
            &session_id,
            &message_hex,
            attempt_number,
            reversed_participants,
        );

        prop_assert_eq!(
            &canonical_attempt_context.included_participants_fingerprint,
            &permuted_attempt_context.included_participants_fingerprint
        );
        prop_assert_eq!(
            &canonical_attempt_context.attempt_id,
            &permuted_attempt_context.attempt_id
        );

        let validation_message_bytes =
            hex::decode(&message_hex).expect("message hex should decode for validation");
        let message_digest_hex = hash_hex(&validation_message_bytes);
        let validated_participants = validate_attempt_context(
            &session_id,
            &attempt_context_key_group_for_tests(&session_id),
            &validation_message_bytes,
            &message_digest_hex,
            2,
            Some(&permuted_attempt_context),
            true,
        )
        .expect("attempt context should validate")
        .expect("validated attempt context should return canonical participants");

        let mut expected_canonical_participants = participants;
        expected_canonical_participants.sort_unstable();
        prop_assert_eq!(validated_participants, expected_canonical_participants);
    }

    #[test]
    fn formal_verification_attempt_context_rejects_tampered_attempt_id(
        session_suffix in any::<u32>(),
        attempt_number in 1_u32..=16_u32,
        participants in participant_set_strategy(),
        // RFC-21 attempt contexts only bind 32-byte signing digests
        // (rfc21_message_digest rejects longer messages), so the
        // strategy stays within that bound.
        message_bytes in prop::collection::vec(any::<u8>(), 1..=32),
    ) {
        let session_id = format!("formal-attempt-tamper-session-{session_suffix}");
        let message_hex = hex::encode(message_bytes);

        let mut tampered_attempt_context = build_deterministic_attempt_context(
            &session_id,
            &message_hex,
            attempt_number,
            participants,
        );
        tampered_attempt_context.attempt_id = "11".repeat(32);

        let validation_message_bytes =
            hex::decode(&message_hex).expect("message hex should decode for validation");
        let message_digest_hex = hash_hex(&validation_message_bytes);
        let err = validate_attempt_context(
            &session_id,
            &attempt_context_key_group_for_tests(&session_id),
            &validation_message_bytes,
            &message_digest_hex,
            2,
            Some(&tampered_attempt_context),
            true,
        )
        .expect_err("tampered attempt id must be rejected");
        prop_assert!(matches!(
            err,
            EngineError::Validation(message)
            if message.contains("attempt_context.attempt_id")
        ));
    }

    #[test]
    fn formal_verification_encrypted_state_envelope_fails_closed_on_key_id_mismatch(
        refresh_epoch_counter in any::<u64>(),
        mismatched_key_id_suffix in any::<u16>(),
    ) {
        let _guard = lock_test_state();
        std::env::set_var(
            TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
            TEST_STATE_ENCRYPTION_KEY_HEX,
        );

        let persisted = PersistedEngineState {
            schema_version: PERSISTED_STATE_SCHEMA_VERSION,
            sessions: HashMap::new(),
            refresh_epoch_counter,
            operator_fault_scores: BTreeMap::new(),
            quarantined_operator_identifiers: vec![],
            canary_rollout: CanaryRolloutState::default(),
        };
        let key_material =
            state_encryption_key_material().expect("state encryption key material");
        let encoded = encode_encrypted_state_envelope(&persisted, &key_material)
            .expect("state envelope encode");
        let envelope: PersistedEncryptedEngineStateEnvelope =
            serde_json::from_slice(encoded.as_ref()).expect("state envelope decode");

        let decoded = decode_encrypted_state_envelope(envelope.clone())
            .expect("untampered envelope should decode");
        prop_assert_eq!(decoded.schema_version, persisted.schema_version);
        prop_assert_eq!(decoded.refresh_epoch_counter, persisted.refresh_epoch_counter);
        prop_assert_eq!(decoded.sessions.len(), persisted.sessions.len());

        let mut tampered_envelope = envelope;
        tampered_envelope.key_id = format!(
            "{}-{}",
            TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX, mismatched_key_id_suffix
        );
        let err = decode_encrypted_state_envelope(tampered_envelope)
            .expect_err("tampered key_id must fail closed");
        prop_assert!(matches!(
            err,
            EngineError::Internal(message)
            if message.contains("state key identifier mismatch")
        ));
    }
}

struct RoastStrictModeGuard {
    previous_value: Option<String>,
}

impl RoastStrictModeGuard {
    fn set(value: Option<&str>) -> Self {
        let previous_value = std::env::var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV).ok();
        match value {
            Some(value) => std::env::set_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV, value),
            None => std::env::remove_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV),
        }

        Self { previous_value }
    }
}

impl Drop for RoastStrictModeGuard {
    fn drop(&mut self) {
        match &self.previous_value {
            Some(value) => std::env::set_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV, value),
            None => std::env::remove_var(TBTC_SIGNER_ENABLE_ROAST_STRICT_ENV),
        }
    }
}

struct SignerProfileGuard {
    previous_value: Option<String>,
}

impl SignerProfileGuard {
    fn set(value: Option<&str>) -> Self {
        let previous_value = std::env::var(TBTC_SIGNER_PROFILE_ENV).ok();
        match value {
            Some(value) => std::env::set_var(TBTC_SIGNER_PROFILE_ENV, value),
            None => std::env::remove_var(TBTC_SIGNER_PROFILE_ENV),
        }

        Self { previous_value }
    }

    fn production() -> Self {
        Self::set(Some(TBTC_SIGNER_PROFILE_PRODUCTION))
    }
}

impl Drop for SignerProfileGuard {
    fn drop(&mut self) {
        match &self.previous_value {
            Some(value) => std::env::set_var(TBTC_SIGNER_PROFILE_ENV, value),
            None => std::env::remove_var(TBTC_SIGNER_PROFILE_ENV),
        }
    }
}

#[test]
#[cfg(unix)]
#[ignore]
fn state_file_lock_contention_helper() {
    if std::env::var("TBTC_SIGNER_LOCK_HELPER").ok().as_deref() != Some("1") {
        return;
    }

    let state_path = active_state_file_path().expect("resolve helper state path");
    let _lock = StateFileLock::acquire(&state_path).expect("acquire helper lock");

    let ready_path =
        std::env::var("TBTC_SIGNER_LOCK_READY_PATH").expect("helper ready path env should be set");
    std::fs::write(&ready_path, b"ready").expect("write helper ready file");

    let release_path = std::env::var("TBTC_SIGNER_LOCK_RELEASE_PATH")
        .expect("helper release path env should be set");
    assert!(
        wait_for_file(Path::new(&release_path), Duration::from_secs(20)),
        "timed out waiting for helper release signal"
    );
}

#[test]
fn production_profile_forces_roast_strict_mode_without_env_flag() {
    let _guard = lock_test_state();
    reset_for_tests();

    {
        let _signer_profile = SignerProfileGuard::production();
        let _roast_strict_mode = RoastStrictModeGuard::set(Some("false"));
        assert!(
            roast_strict_mode_enabled(),
            "production profile must force ROAST strict mode regardless of env flag",
        );
    }

    let _roast_strict_mode = RoastStrictModeGuard::set(Some("false"));
    assert!(
        !roast_strict_mode_enabled(),
        "development profile must honor the disabled strict-mode env flag",
    );
}

#[test]
fn taproot_tweak_matches_cross_repo_deposit_fixture() {
    let internal_key =
        hex::decode("022336f65004d8f122f1fe947ebd009a8b4add3a0d937356d568e30f7fcc2e4008")
            .expect("decode compressed internal key");
    let verifying_key =
        frost::VerifyingKey::deserialize(&internal_key).expect("deserialize verifying key");
    let public_key_package = frost::keys::PublicKeyPackage::new(
        BTreeMap::<frost::Identifier, frost::keys::VerifyingShare>::new(),
        verifying_key,
        Some(1),
    );

    let merkle_root =
        hex::decode("3d6f9a2fea1de0a6c260d1fbc0343c9b2ed84307e6a7231139b78438448ee8c0")
            .expect("decode taproot merkle root");
    let tweaked_public_key = public_key_package
        .tweak(Some(merkle_root.as_slice()))
        .verifying_key()
        .serialize()
        .expect("serialize tweaked verifying key");

    assert_eq!(
        hex::encode(&tweaked_public_key[1..]),
        "90e7ce2b6cd476b7a1c2c7f6585c3fd0eae4379a508e981ed422b3e28b9ae8c2"
    );
}

#[test]
fn deterministic_seed_disambiguates_embedded_zero_bytes() {
    let parts_a = [b"\xaa\x00".as_slice(), b"\x01".as_slice()];
    let parts_b = [b"\xaa".as_slice(), b"\x00\x01".as_slice()];

    assert_ne!(deterministic_seed(&parts_a), deterministic_seed(&parts_b));
}

#[test]
fn persisted_engine_state_rejects_session_registry_over_limit() {
    let _guard = lock_test_state();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

    let mut sessions = HashMap::new();
    sessions.insert("session-a".to_string(), persisted_session_state_fixture());
    sessions.insert("session-b".to_string(), persisted_session_state_fixture());
    sessions.insert("session-c".to_string(), persisted_session_state_fixture());

    let persisted = PersistedEngineState {
        schema_version: PERSISTED_STATE_SCHEMA_VERSION,
        sessions,
        refresh_epoch_counter: 0,
        operator_fault_scores: BTreeMap::new(),
        quarantined_operator_identifiers: vec![],
        canary_rollout: CanaryRolloutState::default(),
    };

    let err = match EngineState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "persisted session registry size [3] exceeds max [2]");

    clear_state_storage_policy_overrides();
}

#[test]
fn persisted_engine_state_compacts_migrated_idle_entries_to_legacy_total_bound() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("migrated_idle_total_bound_rewrite");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

    let mut wallet = persisted_session_state_fixture();
    wallet.dkg_result = Some(DkgResult {
        session_id: "wallet".to_string(),
        key_group: "wallet-key-group".to_string(),
        participant_count: 3,
        threshold: 2,
        created_at_unix: 1,
    });
    let mut consumed_message = persisted_session_state_fixture();
    consumed_message.bound_key_group = Some("wallet-key-group".to_string());
    consumed_message.consumed_interactive_attempt_markers =
        vec![interactive_consumed_marker(&"11".repeat(32), 1)];
    consumed_message.authorized_interactive_aggregate_markers = vec!["22".repeat(32)];
    let mut aborted_message = persisted_session_state_fixture();
    aborted_message.bound_key_group = Some("wallet-key-group".to_string());
    aborted_message.build_tx_request_fingerprint = Some("policy-fingerprint".to_string());

    let persisted = PersistedEngineState {
        schema_version: PERSISTED_STATE_SCHEMA_VERSION,
        sessions: HashMap::from([
            ("wallet".to_string(), wallet),
            ("consumed-message".to_string(), consumed_message),
            ("aborted-message".to_string(), aborted_message),
        ]),
        refresh_epoch_counter: 0,
        operator_fault_scores: BTreeMap::new(),
        quarantined_operator_identifiers: vec![],
        canary_rollout: CanaryRolloutState::default(),
    };

    let loaded = EngineState::try_from(persisted.clone())
        .expect("idle per-message entries migrate and compact to the shared total budget");
    assert_eq!(loaded.sessions.len(), 2);
    assert_eq!(active_session_count(&loaded.sessions), 1);
    assert_eq!(retired_interactive_session_count(&loaded.sessions), 1);
    assert!(loaded.sessions["wallet"]
        .capacity_pins
        .retired_interactive_at_unix
        .is_none());
    assert!(!loaded.sessions.contains_key("aborted-message"));
    assert!(loaded.sessions["consumed-message"]
        .capacity_pins
        .retired_interactive_at_unix
        .is_some());

    let encoded = PersistedEngineState::try_from(&loaded).expect("compacted state encodes");
    assert!(
        encoded.sessions.len() <= 2,
        "the immediately previous schema-1 reader enforces this total bound"
    );

    // Exercise the real load path with a current encrypted envelope emitted by
    // the flawed intermediate writer. Startup must replace the oversized file,
    // not merely compact its in-memory copy, so an immediate rollback can read it.
    let key_material = state_encryption_key_material().expect("test state key");
    let oversized_envelope =
        encode_encrypted_state_envelope(&persisted, &key_material).expect("oversized envelope");
    std::fs::write(&state_path, oversized_envelope.as_slice())
        .expect("write intermediate oversized state");
    let reloaded = load_engine_state_from_storage().expect("load compacts and rewrites state");
    assert_eq!(reloaded.sessions.len(), 2);

    let rewritten_bytes = std::fs::read(&state_path).expect("read rewritten state");
    let rewritten = match decode_persisted_state_storage_format(&rewritten_bytes)
        .expect("decode rewritten state")
    {
        PersistedStateStorageFormat::EncryptedEnvelope { persisted, .. } => persisted,
        PersistedStateStorageFormat::LegacyPlaintext(_) => {
            panic!("rewrite must retain the encrypted envelope")
        }
    };
    assert_eq!(rewritten.sessions.len(), 2);

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn persisted_engine_state_rejects_duplicate_dkg_key_group_owners() {
    let mut owner_a = persisted_session_state_fixture();
    owner_a.dkg_result = Some(DkgResult {
        session_id: "persisted-wallet-a".to_string(),
        key_group: "duplicate-wallet-key-group".to_string(),
        participant_count: 3,
        threshold: 2,
        created_at_unix: 1,
    });
    let mut owner_b = persisted_session_state_fixture();
    owner_b.dkg_result = Some(DkgResult {
        session_id: "persisted-wallet-b".to_string(),
        key_group: "duplicate-wallet-key-group".to_string(),
        participant_count: 3,
        threshold: 2,
        created_at_unix: 1,
    });

    let persisted = PersistedEngineState {
        schema_version: PERSISTED_STATE_SCHEMA_VERSION,
        sessions: HashMap::from([
            ("persisted-wallet-a".to_string(), owner_a),
            ("persisted-wallet-b".to_string(), owner_b),
        ]),
        refresh_epoch_counter: 0,
        operator_fault_scores: BTreeMap::new(),
        quarantined_operator_identifiers: vec![],
        canary_rollout: CanaryRolloutState::default(),
    };

    let err = EngineState::try_from(persisted)
        .err()
        .expect("duplicate persisted key_group owners must fail closed");
    expect_internal_error_contains(err, "duplicate persisted DKG key_group");
}

#[test]
fn persisted_session_state_round_trip_preserves_bound_key_group() {
    // A cross-session signing session has no dkg_result, so bound_key_group is the only
    // durable link back to the wallet DKG. It MUST survive a persist/reload: otherwise an
    // InteractiveAggregate/verify_share that runs after a restart (past a member's Round2,
    // where the live state is already gone) would resolve neither dkg_result nor
    // bound_key_group and return DkgNotReady, stranding the collected shares.
    let session = SessionState {
        dkg: DkgSessionState::default(),
        signing: LegacySigningSessionState::default(),
        interactive: InteractiveSessionState {
            bound_key_group: Some("wallet-key-group".to_string()),
            ..Default::default()
        },
        lifecycle: LifecycleState::default(),
        capacity_pins: OperationalState::default(),
        audit: AuditTrail::default(),
    };
    let persisted = PersistedSessionState::try_from(&session).expect("serialize");
    assert_eq!(
        persisted.bound_key_group.as_deref(),
        Some("wallet-key-group")
    );
    let restored = SessionState::try_from(persisted).expect("deserialize");
    assert_eq!(
        restored.interactive.bound_key_group.as_deref(),
        Some("wallet-key-group"),
        "bound_key_group must survive persist/reload for cross-session signing"
    );
}

#[test]
fn persisted_session_state_rejects_retired_interactive_on_non_per_message_session() {
    // Cross-field retirement invariant: a persisted retired interactive session
    // must satisfy per_message_interactive_session() - i.e. be bound to a wallet
    // key group AND carry no DKG material. A retired entry on a non-per-message
    // session (e.g. one that owns a dkg_result, or that has no bound_key_group)
    // would corrupt the load path because the engine treats retired entries as
    // "idle per-message tombstones that have already released the wallet DKG
    // slot". The TryFrom<PersistedSessionState> impl enforces this check at the
    // end of the conversion; this test pins the invariant so a future migration
    // cannot silently drop or invert it. See docs/specs/frost-signer-sessionstate-
    // grouping.md "Risks" - this exact check is the highest-risk seam in the
    // SessionState grouping split, and is the only invariant in the new code
    // that spans three of the six substructures at once.
    let mut persisted = persisted_session_state_fixture();
    // Some(1) is positive (Some(0) is its own dedicated rejection earlier in the
    // TryFrom body) and obviously synthetic. No bound_key_group means the
    // session is not per-message-interactive, so the invariant must fire.
    persisted.retired_interactive_at_unix = Some(1);
    persisted.bound_key_group = None;

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!(
            "expected decode rejection: a retired interactive session without a \
             per-message role must not round-trip back into SessionState"
        ),
        Err(err) => err,
    };
    expect_internal_error_contains(
        err,
        "persisted retired interactive session must have the per-message role",
    );
}

// Pins the OperationalState::retired_interactive_at_unix round-trip end-to-end.
// Of the three OperationalState fields, only this one persists: the two siblings
// (heartbeat_rate_limiter, aggregate_eviction_pin) are deliberately transient
// and reset on restart. The grouping split makes an easy mistake - wholesale-
// defaulting OperationalState in either TryFrom direction - that would silently
// drop the timestamp on every restart and break idle-session admission. The
// cross-field invariant above requires the session to be per-message-interactive
// for the retired timestamp to survive, so this SessionState carries a
// bound_key_group.
#[test]
fn persisted_session_state_round_trip_preserves_capacity_pins_retired_interactive_at_unix() {
    let retired_at: u64 = 1_700_000_000;
    let session = SessionState {
        interactive: InteractiveSessionState {
            bound_key_group: Some("wallet-key-group".to_string()),
            ..Default::default()
        },
        capacity_pins: OperationalState {
            retired_interactive_at_unix: Some(retired_at),
            ..Default::default()
        },
        ..Default::default()
    };
    let persisted = PersistedSessionState::try_from(&session).expect("serialize");
    assert_eq!(
        persisted.retired_interactive_at_unix,
        Some(retired_at),
        "retired_interactive_at_unix must serialize into PersistedSessionState"
    );
    let restored = SessionState::try_from(persisted).expect("deserialize");
    assert_eq!(
        restored.capacity_pins.retired_interactive_at_unix,
        Some(retired_at),
        "retired_interactive_at_unix must round-trip through persistence"
    );
}

#[test]
fn interactive_open_cross_session_respects_the_session_cap() {
    // A fresh RoastSessionID per message must not let Open grow the session registry
    // past TBTC_SIGNER_MAX_SESSIONS: otherwise the cross-session path could build an
    // over-limit registry that the reload path (see the test above) then rejects,
    // stranding the node's persisted state. Open enforces the SAME total-session cap as
    // every other session-creating path; a reopen of an existing session stays exempt.
    let _guard = lock_test_state();
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "1");

    let wallet_session = "wallet-dkg-session-cap";
    let key_group = "cross-session-cap-key-group";
    let message = [0x23u8; 32];
    let included = [1u16, 2];

    // The wallet DKG session fills the cap (1 of 1).
    ensure_interactive_dkg_session(wallet_session, key_group);

    // A distinct signing session would be a SECOND session -> rejected by the cap,
    // BEFORE any per-signing state is installed.
    let attempt_context =
        interactive_test_attempt_context("roast-over-cap", key_group, &message, &included, 1);
    let err = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: "roast-over-cap".to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
    .expect_err("a new cross-session Open at the session cap must be rejected");
    assert!(
        matches!(err, EngineError::Internal(ref m) if m.contains("reached max")),
        "unexpected error: {err:?}"
    );
    // The over-cap session must NOT have been created.
    {
        let guard = state().expect("state").lock().expect("lock");
        assert!(
            !guard.sessions.contains_key("roast-over-cap"),
            "a capped-out Open must not create the session"
        );
    }

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
}

#[test]
fn interactive_open_refuses_to_rebind_a_live_session_to_a_different_key_group() {
    // A per-signing session is keyed by RoastSessionID (message/root/start-block), NOT
    // key_group, so two wallets can collide on one session id. While a member is
    // mid-signing under one wallet key, an Open for a DIFFERENT key group on the same
    // session id must be REJECTED - not silently rebind bound_key_group and make the
    // live member's Round2/Aggregate resolve the wrong wallet material.
    let _guard = lock_test_state();
    reset_for_tests();

    let wallet_a = "wallet-dkg-a-rebind";
    let wallet_b = "wallet-dkg-b-rebind";
    let key_group_a = "key-group-a-rebind";
    let key_group_b = "key-group-b-rebind";
    let shared_session = "roast-collision-session";
    let message = [0x24u8; 32];
    let included = [1u16, 2];

    ensure_interactive_dkg_session(wallet_a, key_group_a);
    ensure_interactive_dkg_session(wallet_b, key_group_b);

    // Member 1 opens under wallet A on the shared session and runs Round1 (a LIVE entry).
    let ctx_a =
        interactive_test_attempt_context(shared_session, key_group_a, &message, &included, 1);
    let opened_a = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: shared_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group_a.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: ctx_a,
    })
    .expect("wallet A opens on the shared session");
    interactive_round1(InteractiveRound1Request {
        session_id: shared_session.to_string(),
        attempt_id: opened_a.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("wallet A round 1 leaves a live entry");

    // Wallet B tries to open the SAME session id while A is live -> rejected.
    let ctx_b =
        interactive_test_attempt_context(shared_session, key_group_b, &message, &included, 1);
    let err = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: shared_session.to_string(),
        member_identifier: 2,
        message_hex: hex::encode(message),
        key_group: key_group_b.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: ctx_b,
    })
    .expect_err("a different key group must not rebind a live session");
    assert!(
        matches!(err, EngineError::SessionConflict { .. }),
        "unexpected error: {err:?}"
    );

    // Wallet A's binding and live entry are intact - not corrupted by B's attempt.
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(shared_session).expect("shared session");
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some(key_group_a)
        );
        assert!(
            session
                .interactive
                .interactive_signing
                .values()
                .any(|members| members.contains_key(&1)),
            "wallet A's live member entry must survive B's rejected open"
        );
    }
}

#[test]
fn interactive_open_refuses_to_bind_through_another_wallets_dkg_session() {
    // If request.session_id names wallet A's (idle) DKG session but the request's
    // key_group is wallet B, Open must NOT install B into A's session: with dkg_result
    // A present, later Round2/Aggregate/verify_share would resolve A's material while
    // signing B's share (wrong wallet, bypassing B's rekey/finalization gates). A
    // session belongs to ONE key group for its lifetime, so the mismatch is rejected -
    // even with no live members (dkg_result establishes the binding).
    let _guard = lock_test_state();
    reset_for_tests();

    let wallet_a = "wallet-a-dkg-bind";
    let wallet_b = "wallet-b-dkg-bind";
    let key_group_a = "key-group-a-dkg-bind";
    let key_group_b = "key-group-b-dkg-bind";
    let message = [0x25u8; 32];
    let included = [1u16, 2];

    // wallet_a is an IDLE DKG session (dkg_result A, no live interactive entries);
    // wallet_b provides key_group B's material so its wallet resolution succeeds.
    ensure_interactive_dkg_session(wallet_a, key_group_a);
    ensure_interactive_dkg_session(wallet_b, key_group_b);

    // Open wallet A's DKG session id but for key_group B -> rejected.
    let ctx = interactive_test_attempt_context(wallet_a, key_group_b, &message, &included, 1);
    let err = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: wallet_a.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group_b.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: ctx,
    })
    .expect_err("binding through another wallet's DKG session must be rejected");
    assert!(
        matches!(err, EngineError::SessionConflict { .. }),
        "unexpected error: {err:?}"
    );

    // Wallet A's DKG session is untouched: no B binding installed.
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(wallet_a).expect("wallet A session");
        assert_eq!(
            session
                .dkg
                .result
                .as_ref()
                .map(|dkg| dkg.key_group.as_str()),
            Some(key_group_a)
        );
        assert!(
            session.interactive.bound_key_group.is_none(),
            "no cross-wallet binding may be installed on wallet A's session"
        );
    }
}

#[test]
fn max_sessions_limit_env_parser_is_strict_positive() {
    let _guard = lock_test_state();
    clear_state_storage_policy_overrides();

    assert_eq!(max_sessions_limit(), TBTC_SIGNER_DEFAULT_MAX_SESSIONS);

    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "not-a-number");
    assert_eq!(max_sessions_limit(), TBTC_SIGNER_DEFAULT_MAX_SESSIONS);

    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "0");
    assert_eq!(max_sessions_limit(), TBTC_SIGNER_DEFAULT_MAX_SESSIONS);

    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "-1");
    assert_eq!(max_sessions_limit(), TBTC_SIGNER_DEFAULT_MAX_SESSIONS);

    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "   7   ");
    assert_eq!(max_sessions_limit(), 7);

    clear_state_storage_policy_overrides();
}

#[test]
fn roast_coordinator_timeout_ms_env_parser_is_strict_bounds() {
    let _guard = lock_test_state();
    clear_state_storage_policy_overrides();

    assert_eq!(
        roast_coordinator_timeout_ms(),
        TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
    );

    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "not-a-number");
    assert_eq!(
        roast_coordinator_timeout_ms(),
        TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
    );

    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "0");
    assert_eq!(
        roast_coordinator_timeout_ms(),
        TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
    );

    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "999");
    assert_eq!(
        roast_coordinator_timeout_ms(),
        TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
    );

    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "300001");
    assert_eq!(
        roast_coordinator_timeout_ms(),
        TBTC_SIGNER_DEFAULT_ROAST_COORDINATOR_TIMEOUT_MS
    );

    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, " 45000 ");
    assert_eq!(roast_coordinator_timeout_ms(), 45_000);

    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_rejects_new_session_when_session_registry_is_at_capacity() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("build_taproot_tx_session_capacity");
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "1");

    let first_request = BuildTaprootTxRequest {
        session_id: "session-build-tx-capacity-a".to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: 10_000,
            script_pubkey_hex: taproot_prevout_script_hex(),
        }],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "22".repeat(32)),
            value_sats: 8_000,
        }],
        script_tree_hex: None,
    };
    build_taproot_tx(first_request.clone()).expect("first build tx");
    build_taproot_tx(first_request).expect("idempotent build tx at capacity");

    let second_request = BuildTaprootTxRequest {
        session_id: "session-build-tx-capacity-b".to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "33".repeat(32),
            vout: 0,
            value_sats: 10_000,
            script_pubkey_hex: taproot_prevout_script_hex(),
        }],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "44".repeat(32)),
            value_sats: 8_000,
        }],
        script_tree_hex: None,
    };
    let err = build_taproot_tx(second_request).expect_err("expected session cap rejection");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("session registry size [1] reached max [1]"),
        "unexpected internal message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn per_message_session_retirement_preserves_wallet_routing_and_retry_state() {
    let _guard = lock_test_state();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "5");

    let mut engine_state = EngineState::default();
    engine_state.sessions.insert(
        "wallet".to_string(),
        SessionState {
            dkg: DkgSessionState {
                result: Some(DkgResult {
                    session_id: "wallet".to_string(),
                    key_group: "wallet-key-group".to_string(),
                    participant_count: 3,
                    threshold: 2,
                    created_at_unix: now_unix(),
                }),
                ..Default::default()
            },
            signing: LegacySigningSessionState::default(),
            interactive: InteractiveSessionState::default(),
            lifecycle: LifecycleState::default(),
            capacity_pins: OperationalState::default(),
            audit: AuditTrail::default(),
        },
    );
    engine_state.sessions.insert(
        "open-only".to_string(),
        SessionState {
            dkg: DkgSessionState::default(),
            signing: LegacySigningSessionState::default(),
            interactive: InteractiveSessionState {
                bound_key_group: Some("wallet-key-group".to_string()),
                ..Default::default()
            },
            lifecycle: LifecycleState::default(),
            capacity_pins: OperationalState::default(),
            audit: AuditTrail::default(),
        },
    );
    engine_state.sessions.insert(
        "consumed".to_string(),
        SessionState {
            dkg: DkgSessionState::default(),
            signing: LegacySigningSessionState::default(),
            interactive: InteractiveSessionState {
                bound_key_group: Some("wallet-key-group".to_string()),
                consumed_attempt_markers: HashSet::from([interactive_consumed_marker(
                    &"11".repeat(32),
                    1,
                )]),
                authorized_aggregate_markers: HashSet::from(["22".repeat(32)]),
                ..Default::default()
            },
            lifecycle: LifecycleState::default(),
            capacity_pins: OperationalState::default(),
            audit: AuditTrail::default(),
        },
    );
    engine_state.sessions.insert(
        "completed".to_string(),
        SessionState {
            dkg: DkgSessionState::default(),
            signing: LegacySigningSessionState::default(),
            interactive: InteractiveSessionState {
                bound_key_group: Some("wallet-key-group".to_string()),
                aggregated_attempt_markers: HashSet::from([format!(
                    "{}@{}@keypath",
                    "33".repeat(32),
                    "44".repeat(32)
                )]),
                ..Default::default()
            },
            lifecycle: LifecycleState::default(),
            capacity_pins: OperationalState::default(),
            audit: AuditTrail::default(),
        },
    );
    engine_state.sessions.insert(
        "retry-policy".to_string(),
        SessionState {
            dkg: DkgSessionState::default(),
            signing: LegacySigningSessionState {
                build_tx_request_fingerprint: Some("policy-fingerprint".to_string()),
                ..Default::default()
            },
            interactive: InteractiveSessionState {
                bound_key_group: Some("wallet-key-group".to_string()),
                ..Default::default()
            },
            lifecycle: LifecycleState::default(),
            capacity_pins: OperationalState::default(),
            audit: AuditTrail::default(),
        },
    );

    assert_eq!(retire_idle_per_message_sessions(&mut engine_state, None), 4);
    assert_eq!(active_session_count(&engine_state.sessions), 1);
    assert_eq!(retired_interactive_session_count(&engine_state.sessions), 4);
    assert!(engine_state.sessions["wallet"]
        .capacity_pins
        .retired_interactive_at_unix
        .is_none());
    assert_eq!(
        engine_state.sessions["retry-policy"]
            .signing
            .build_tx_request_fingerprint,
        Some("policy-fingerprint".to_string())
    );
    assert!(engine_state.sessions["consumed"]
        .interactive
        .authorized_aggregate_markers
        .contains(&"22".repeat(32)));

    reactivate_retired_per_message_session(&mut engine_state, "retry-policy")
        .expect("retired retry state reactivates");
    assert_eq!(active_session_count(&engine_state.sessions), 2);
    assert!(engine_state.sessions["retry-policy"]
        .capacity_pins
        .retired_interactive_at_unix
        .is_none());
    assert_eq!(
        engine_state.sessions["retry-policy"]
            .signing
            .build_tx_request_fingerprint,
        Some("policy-fingerprint".to_string())
    );

    clear_state_storage_policy_overrides();
}

#[test]
fn retired_per_message_sessions_share_the_total_bound_and_yield_to_admission() {
    let _guard = lock_test_state();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

    let mut engine_state = EngineState::default();
    for (session_id, retired_at) in [("oldest", 1), ("middle", 2), ("newest", 3)] {
        engine_state.sessions.insert(
            session_id.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some("wallet-key-group".to_string()),
                    consumed_attempt_markers: HashSet::from([interactive_consumed_marker(
                        &hash_hex(session_id.as_bytes()),
                        1,
                    )]),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState {
                    retired_interactive_at_unix: Some(retired_at),
                    ..Default::default()
                },
                audit: AuditTrail::default(),
            },
        );
    }

    assert_eq!(
        compact_retired_per_message_sessions(&mut engine_state, Some("newest")).len(),
        1
    );
    assert!(!engine_state.sessions.contains_key("oldest"));
    assert!(engine_state.sessions.contains_key("middle"));
    assert!(engine_state.sessions.contains_key("newest"));
    assert_eq!(active_session_count(&engine_state.sessions), 0);
    assert_eq!(retired_interactive_session_count(&engine_state.sessions), 2);
    let reserved = ensure_session_insert_capacity(&mut engine_state, "fresh-active")
        .expect("bounded retirement cannot block active admission");
    assert_eq!(
        reserved
            .iter()
            .map(|(session_id, _)| session_id.as_str())
            .collect::<Vec<_>>(),
        vec!["middle"]
    );
    engine_state
        .sessions
        .insert("fresh-active".to_string(), SessionState::default());
    assert_eq!(engine_state.sessions.len(), 2);
    assert!(engine_state.sessions.contains_key("newest"));
    assert!(engine_state.sessions.contains_key("fresh-active"));

    clear_state_storage_policy_overrides();
}

#[test]
fn session_slot_reservation_preserves_pinned_and_persistence_pending_tombstones() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "1");

    let retired_session = "protected-retired-session";
    let aggregated_marker = interactive_aggregated_marker(
        &hash_hex(b"protected-attempt"),
        &hash_hex(b"protected-message"),
        None,
    );
    let mut engine_state = EngineState::default();
    engine_state.sessions.insert(
        retired_session.to_string(),
        SessionState {
            dkg: DkgSessionState::default(),
            signing: LegacySigningSessionState::default(),
            interactive: InteractiveSessionState {
                bound_key_group: Some("protected-retired-key".to_string()),
                aggregated_attempt_markers: HashSet::from([aggregated_marker.clone()]),
                ..Default::default()
            },
            lifecycle: LifecycleState::default(),
            capacity_pins: OperationalState {
                retired_interactive_at_unix: Some(1),
                ..Default::default()
            },
            audit: AuditTrail::default(),
        },
    );

    let aggregate_pin = Arc::clone(
        &engine_state.sessions[retired_session]
            .capacity_pins
            .aggregate_eviction_pin,
    );
    let pinned_error = match ensure_session_insert_capacity(&mut engine_state, "new-active") {
        Ok(_) => panic!("an in-flight Aggregate pin must block eviction"),
        Err(error) => error,
    };
    assert!(matches!(pinned_error, EngineError::Internal(_)));
    assert!(engine_state.sessions.contains_key(retired_session));
    drop(aggregate_pin);

    let pending = PersistencePendingOperation::InteractiveAggregate {
        session_id: retired_session.to_string(),
        aggregated_marker,
    };
    mark_persistence_pending(pending.clone());
    let pending_error = match ensure_session_insert_capacity(&mut engine_state, "new-active") {
        Ok(_) => panic!("an uncovered persistence marker must block eviction"),
        Err(error) => error,
    };
    assert!(matches!(pending_error, EngineError::Internal(_)));
    assert!(engine_state.sessions.contains_key(retired_session));
    clear_persistence_pending_operation(&pending);

    let removed = ensure_session_insert_capacity(&mut engine_state, "new-active")
        .expect("the slot becomes available after both protections release");
    assert_eq!(removed.len(), 1);
    assert_eq!(removed[0].0, retired_session);
    assert!(engine_state.sessions.is_empty());

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    clear_state_storage_policy_overrides();
}

#[test]
fn idle_per_message_session_stays_active_while_marker_persistence_is_pending() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "pending-idle-per-message";
    let aggregated_marker = interactive_aggregated_marker(
        &hash_hex(b"pending-idle-attempt"),
        &hash_hex(b"pending-idle-message"),
        None,
    );
    let pending_operation = PersistencePendingOperation::InteractiveAggregate {
        session_id: session_id.to_string(),
        aggregated_marker: aggregated_marker.clone(),
    };
    let mut engine_state = EngineState::default();
    engine_state.sessions.insert(
        session_id.to_string(),
        SessionState {
            dkg: DkgSessionState::default(),
            signing: LegacySigningSessionState::default(),
            interactive: InteractiveSessionState {
                bound_key_group: Some("pending-idle-key-group".to_string()),
                aggregated_attempt_markers: HashSet::from([aggregated_marker]),
                ..Default::default()
            },
            lifecycle: LifecycleState::default(),
            capacity_pins: OperationalState::default(),
            audit: AuditTrail::default(),
        },
    );
    mark_persistence_pending(pending_operation.clone());

    assert_eq!(retire_idle_per_message_sessions(&mut engine_state, None), 0);
    assert!(engine_state.sessions[session_id]
        .capacity_pins
        .retired_interactive_at_unix
        .is_none());

    clear_persistence_pending_operation(&pending_operation);
    assert_eq!(retire_idle_per_message_sessions(&mut engine_state, None), 1);
    assert!(engine_state.sessions[session_id]
        .capacity_pins
        .retired_interactive_at_unix
        .is_some());
}

#[test]
fn persisted_session_state_rejects_empty_consumed_attempt_id() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_attempt_ids = vec!["".to_string()];

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "persisted consumed attempt ID must be non-empty");
}

#[test]
fn persisted_session_state_rejects_duplicate_consumed_attempt_id() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_attempt_ids = vec!["attempt-a".to_string(), "attempt-a".to_string()];

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "duplicate persisted consumed attempt ID [attempt-a]");
}

#[test]
fn persisted_session_state_rejects_empty_consumed_sign_round_id() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_sign_round_ids = vec!["".to_string()];

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "persisted consumed sign round ID must be non-empty");
}

#[test]
fn persisted_session_state_rejects_duplicate_consumed_sign_round_id() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_sign_round_ids = vec!["round-a".to_string(), "round-a".to_string()];

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "duplicate persisted consumed sign round ID [round-a]");
}

#[test]
fn persisted_session_state_rejects_empty_consumed_finalize_round_id() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_finalize_round_ids = vec!["".to_string()];

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(
        err,
        "persisted consumed finalize round ID must be non-empty",
    );
}

#[test]
fn persisted_session_state_rejects_duplicate_consumed_finalize_round_id() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_finalize_round_ids = vec!["round-b".to_string(), "round-b".to_string()];

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(
        err,
        "duplicate persisted consumed finalize round ID [round-b]",
    );
}

#[test]
fn persisted_session_state_rejects_empty_consumed_finalize_request_fingerprint() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_finalize_request_fingerprints = vec!["".to_string()];

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(
        err,
        "persisted consumed finalize request fingerprint must be non-empty",
    );
}

#[test]
fn persisted_session_state_rejects_duplicate_consumed_finalize_request_fingerprint() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_finalize_request_fingerprints = vec!["fp-1".to_string(), "fp-1".to_string()];

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(
        err,
        "duplicate persisted consumed finalize request fingerprint [fp-1]",
    );
}

#[test]
fn persisted_session_state_rejects_consumed_attempt_registry_over_limit() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_attempt_ids = (0..=TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION)
        .map(|idx| format!("attempt-{idx}"))
        .collect();

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "persisted consumed_attempt_ids registry size");
}

#[test]
fn persisted_session_state_rejects_consumed_sign_round_registry_over_limit() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_sign_round_ids = (0..=TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION)
        .map(|idx| format!("round-{idx}"))
        .collect();

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "persisted consumed_sign_round_ids registry size");
}

#[test]
fn persisted_session_state_rejects_consumed_finalize_round_registry_over_limit() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_finalize_round_ids = (0
        ..=TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION)
        .map(|idx| format!("round-{idx}"))
        .collect();

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "persisted consumed_finalize_round_ids registry size");
}

#[test]
fn persisted_session_state_rejects_consumed_finalize_request_registry_over_limit() {
    let mut persisted = persisted_session_state_fixture();
    persisted.consumed_finalize_request_fingerprints = (0
        ..=TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION)
        .map(|idx| format!("fp-{idx}"))
        .collect();

    let err = match SessionState::try_from(persisted) {
        Ok(_) => panic!("expected decode rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(
        err,
        "persisted consumed_finalize_request_fingerprints registry size",
    );
}

#[test]
fn state_lock_path_is_bound_and_rejects_in_process_path_switch() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("state_lock_path_binding");
    let alternate_state_path = std::env::temp_dir().join(format!(
        "frost_tbtc_engine_state_state_lock_path_binding_alt_{}.json",
        std::process::id()
    ));
    cleanup_test_state_artifacts(&alternate_state_path);
    reset_for_tests();

    persist_state_for_key_provider_test("session-lock-path-initial")
        .expect("initial state persist");

    std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &alternate_state_path);

    let err = persist_state_for_key_provider_test("session-lock-path-switch")
        .expect_err("expected path switch rejection");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("refusing to switch"),
        "unexpected lock path switch error: {message}"
    );

    std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &state_path);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    cleanup_test_state_artifacts(&alternate_state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn restart_reload_recovers_persisted_state_across_operation_types() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("restart_reload_integration");
    reset_for_tests();

    // Operation type 1: distributed-DKG key-package persistence.
    let (native_public, native_key_packages) = sample_distributed_dkg_native_material(9);
    let persist_result =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "session-restart-dkg".to_string(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: native_public.clone(),
        })
        .expect("persist distributed dkg key package");

    // Operation type 2: taproot transaction building.
    let build_request = BuildTaprootTxRequest {
        session_id: "session-restart-buildtx".to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: 10_000,
            script_pubkey_hex: taproot_prevout_script_hex(),
        }],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "22".repeat(32)),
            value_sats: 9_000,
        }],
        script_tree_hex: None,
    };
    let build_result = build_taproot_tx(build_request.clone()).expect("build taproot tx");

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        assert!(guard.sessions.contains_key("session-restart-dkg"));
        assert!(guard.sessions.contains_key("session-restart-buildtx"));
    }

    // The persisted DKG session survives the restart: a sibling seat
    // accumulates into the reloaded session and shares its key group.
    let persist_sibling =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "session-restart-dkg".to_string(),
            participant_identifier: 2,
            threshold: 2,
            participant_count: 3,
            key_package: native_key_packages.get(&2).expect("seat 2").clone(),
            public_key_package: native_public.clone(),
        })
        .expect("persist sibling seat after reload");
    assert_eq!(persist_sibling.key_group, persist_result.key_group);

    // Idempotent retries of the persisted operations return the same result.
    let build_retry_result = build_taproot_tx(build_request).expect("retry build taproot tx");
    assert_eq!(build_result, build_retry_result);

    // A brand-new operation on a fresh session works post-restart.
    let (new_public, new_key_packages) = sample_distributed_dkg_native_material(11);
    let new_session_result =
        persist_distributed_dkg_key_package(crate::api::PersistDistributedDkgKeyPackageRequest {
            session_id: "session-restart-new".to_string(),
            participant_identifier: 1,
            threshold: 2,
            participant_count: 3,
            key_package: new_key_packages.get(&1).expect("seat 1").clone(),
            public_key_package: new_public,
        })
        .expect("post-restart persist");
    assert!(!new_session_result.key_group.is_empty());

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn first_engine_state_load_is_serialized_across_callers() {
    let engine_state = std::sync::Arc::new(OnceLock::<Mutex<EngineState>>::new());
    let initialization_lock = std::sync::Arc::new(Mutex::new(()));
    let initial_miss_barrier = std::sync::Arc::new(std::sync::Barrier::new(2));
    let load_count = std::sync::Arc::new(std::sync::atomic::AtomicUsize::new(0));

    let callers = (0..2)
        .map(|_| {
            let engine_state = std::sync::Arc::clone(&engine_state);
            let initialization_lock = std::sync::Arc::clone(&initialization_lock);
            let initial_miss_barrier = std::sync::Arc::clone(&initial_miss_barrier);
            let load_count = std::sync::Arc::clone(&load_count);

            std::thread::spawn(move || {
                let initialized = initialize_engine_state_with_loader(
                    &engine_state,
                    &initialization_lock,
                    || {
                        // Both callers must pass the optimistic OnceLock check
                        // before either may enter the serialized loader path.
                        initial_miss_barrier.wait();
                    },
                    || {
                        let invocation =
                            load_count.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                        Ok(EngineState {
                            refresh_epoch_counter: invocation as u64 + 41,
                            ..EngineState::default()
                        })
                    },
                )
                .expect("concurrent initialization");

                let state_pointer = std::ptr::from_ref(initialized) as usize;
                let refresh_epoch_counter = initialized
                    .lock()
                    .expect("initialized engine state lock")
                    .refresh_epoch_counter;
                (state_pointer, refresh_epoch_counter)
            })
        })
        .collect::<Vec<_>>();

    let results = callers
        .into_iter()
        .map(|caller| caller.join().expect("initialization caller"))
        .collect::<Vec<_>>();

    assert_eq!(
        load_count.load(std::sync::atomic::Ordering::SeqCst),
        1,
        "only one first caller may load or migrate persistent state"
    );
    assert_eq!(results[0].0, results[1].0);
    assert_eq!(results[0].1, 41);
    assert_eq!(results[1].1, 41);
}

#[test]
fn failed_engine_state_load_remains_retryable() {
    let engine_state = OnceLock::<Mutex<EngineState>>::new();
    let initialization_lock = Mutex::new(());
    let load_count = std::sync::atomic::AtomicUsize::new(0);

    let first_error = match initialize_engine_state_with_loader(
        &engine_state,
        &initialization_lock,
        || {},
        || {
            load_count.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
            Err(EngineError::Internal(
                "intentional first-load failure".to_string(),
            ))
        },
    ) {
        Ok(_) => panic!("failed loader must not initialize engine state"),
        Err(error) => error,
    };
    expect_internal_error_contains(first_error, "intentional first-load failure");
    assert!(
        engine_state.get().is_none(),
        "a fallible load must leave the OnceLock unset"
    );

    let initialized = initialize_engine_state_with_loader(
        &engine_state,
        &initialization_lock,
        || {},
        || {
            load_count.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
            Ok(EngineState {
                refresh_epoch_counter: 73,
                ..EngineState::default()
            })
        },
    )
    .expect("retry after failed state load");

    assert_eq!(load_count.load(std::sync::atomic::Ordering::SeqCst), 2);
    assert_eq!(
        initialized
            .lock()
            .expect("initialized engine state lock")
            .refresh_epoch_counter,
        73
    );
}

#[test]
#[cfg(unix)]
fn state_lock_rejects_multi_process_contention() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("state_lock_multi_process_contention");
    let ready_path = std::env::temp_dir().join(format!(
        "frost_tbtc_lock_ready_{}_{}.flag",
        std::process::id(),
        now_unix()
    ));
    let release_path = std::env::temp_dir().join(format!(
        "frost_tbtc_lock_release_{}_{}.flag",
        std::process::id(),
        now_unix()
    ));
    let _ = std::fs::remove_file(&ready_path);
    let _ = std::fs::remove_file(&release_path);
    reset_for_tests();

    if let Ok(mut lock_slot) = state_file_lock_slot().lock() {
        *lock_slot = None;
    }

    let child = Command::new(std::env::current_exe().expect("current test binary path"))
        .arg("--exact")
        .arg("engine::tests::state_file_lock_contention_helper")
        .arg("--ignored")
        .arg("--nocapture")
        .env(TBTC_SIGNER_STATE_PATH_ENV, &state_path)
        .env("TBTC_SIGNER_LOCK_HELPER", "1")
        .env("TBTC_SIGNER_LOCK_READY_PATH", &ready_path)
        .env("TBTC_SIGNER_LOCK_RELEASE_PATH", &release_path)
        .spawn()
        .expect("spawn lock holder helper process");
    let helper_guard = LockHelperProcessGuard::new(child, release_path.clone());

    assert!(
        wait_for_file(&ready_path, Duration::from_secs(10)),
        "helper did not report lock acquisition"
    );

    let err = match ensure_state_file_lock() {
        Ok(_) => panic!("expected lock contention error"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "signer state lock already held by another process");

    helper_guard.wait_for_success();

    let _ = std::fs::remove_file(&ready_path);
    let _ = std::fs::remove_file(&release_path);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
#[cfg(unix)]
fn persisted_state_file_uses_owner_only_permissions() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("state_file_permissions");
    reset_for_tests();

    persist_state_for_key_provider_test("session-state-file-permissions")
        .expect("persist signer state");

    let mode = std::fs::metadata(&state_path)
        .expect("state file metadata")
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(
        mode, 0o600,
        "state file should be owner read/write only, got mode {mode:o}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn build_taproot_tx_idempotency_persists_across_storage_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("build_taproot_tx_idempotency");
    reset_for_tests();

    let request = BuildTaprootTxRequest {
        session_id: "session-build-tx".to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: 10_000,
            script_pubkey_hex: taproot_prevout_script_hex(),
        }],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "22".repeat(32)),
            value_sats: 9_000,
        }],
        script_tree_hex: None,
    };

    let first_result = build_taproot_tx(request.clone()).expect("first build tx");
    assert!(!first_result.tx_hex.is_empty());

    reload_state_from_storage_for_tests();
    let second_result = build_taproot_tx(request).expect("persisted build tx retry");
    assert_eq!(first_result, second_result);

    let conflict_request = BuildTaprootTxRequest {
        session_id: "session-build-tx".to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: 10_000,
            script_pubkey_hex: taproot_prevout_script_hex(),
        }],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "22".repeat(32)),
            value_sats: 8_000,
        }],
        script_tree_hex: None,
    };

    let err = build_taproot_tx(conflict_request).expect_err("expected build tx conflict");
    assert!(matches!(err, EngineError::SessionConflict { .. }));

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn corrupt_state_file_fails_closed_by_default() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("corrupt_state_fail_closed");
    reset_for_tests();

    std::fs::write(&state_path, b"{invalid-state").expect("write corrupt state file");

    let err = match load_engine_state_from_storage() {
        Ok(_) => panic!("expected corruption failure"),
        Err(err) => err,
    };
    assert!(matches!(err, EngineError::Internal(_)));

    let err_message = err.to_string();
    assert!(err_message.contains("refusing to continue with corrupted signer state file"));
    assert!(err_message.contains(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV));
    assert!(state_path.exists());

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn state_file_parent_directory_normalizes_only_bare_paths() {
    assert_eq!(
        state_file_parent_directory(Path::new("state.json")),
        Some(Path::new("."))
    );

    let nested_state_path = Path::new("nested/state.json");
    assert_eq!(
        state_file_parent_directory(nested_state_path),
        nested_state_path.parent()
    );

    let absolute_state_path = std::env::temp_dir().join("nested").join("state.json");
    assert!(absolute_state_path.is_absolute());
    assert_eq!(
        state_file_parent_directory(&absolute_state_path),
        absolute_state_path.parent()
    );
}

#[test]
fn bare_state_path_persists_and_syncs_current_directory() {
    let _guard = lock_test_state();
    let state_path = PathBuf::from(format!(
        "frost_tbtc_engine_state_bare_persist_{}.json",
        std::process::id()
    ));
    assert_eq!(state_path.parent(), Some(Path::new("")));

    clear_state_storage_policy_overrides();
    cleanup_test_state_artifacts(&state_path);
    std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &state_path);
    reset_for_tests();

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        guard.refresh_epoch_counter = 41;
        persist_engine_state_to_storage(&guard)
            .expect("persist state through a one-component path");
    }

    assert!(state_path.exists(), "bare state path should be replaced");
    let loaded = load_engine_state_from_storage().expect("load persisted bare-path state");
    assert_eq!(loaded.refresh_epoch_counter, 41);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn bare_state_path_corruption_quarantine_enumerates_current_directory() {
    let _guard = lock_test_state();
    let state_path = PathBuf::from(format!(
        "frost_tbtc_engine_state_bare_corrupt_{}.json",
        std::process::id()
    ));
    assert_eq!(state_path.parent(), Some(Path::new("")));

    clear_state_storage_policy_overrides();
    cleanup_test_state_artifacts(&state_path);
    std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &state_path);
    reset_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
    );
    std::fs::write(&state_path, b"{invalid-bare-state")
        .expect("write corrupt one-component state path");

    let loaded = load_engine_state_from_storage().expect("quarantine corrupt bare-path state");
    assert!(loaded.sessions.is_empty());
    assert!(!state_path.exists());

    let backups = sorted_corrupted_state_backups(&state_path).expect("enumerate bare-path backups");
    assert_eq!(backups.len(), 1);
    assert_eq!(
        std::fs::read(&backups[0]).expect("read quarantined bare-path state"),
        b"{invalid-bare-state"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn empty_state_file_fails_closed_by_default() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("empty_state_fail_closed");
    reset_for_tests();

    std::fs::write(&state_path, b"").expect("truncate state file to zero bytes");

    let err = match load_engine_state_from_storage() {
        Ok(_) => panic!("empty state must be corruption"),
        Err(err) => err,
    };
    assert!(matches!(err, EngineError::Internal(_)));
    let err_message = err.to_string();
    assert!(err_message.contains("exists but is empty"));
    assert!(err_message.contains("refusing to continue with corrupted signer state file"));
    assert!(err_message.contains(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV));
    assert_eq!(
        std::fs::metadata(&state_path)
            .expect("empty state file remains")
            .len(),
        0
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[cfg(unix)]
#[test]
fn dangling_state_file_symlink_fails_closed() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("dangling_state_symlink");
    reset_for_tests();

    let missing_target = state_path.with_extension("missing-target");
    let _ = std::fs::remove_file(&missing_target);
    std::fs::remove_file(&state_path).expect("remove initialized state file");
    std::os::unix::fs::symlink(&missing_target, &state_path)
        .expect("create dangling state symlink");

    let err = match load_engine_state_from_storage() {
        Ok(_) => panic!("a dangling state symlink must not initialize clean state"),
        Err(err) => err,
    };
    assert!(
        matches!(err, EngineError::Internal(ref message) if message.contains("failed to read signer state file")),
        "unexpected error: {err:?}"
    );
    assert!(
        std::fs::symlink_metadata(&state_path)
            .expect("dangling symlink metadata")
            .file_type()
            .is_symlink(),
        "the failed load must not replace or reinterpret the dangling symlink"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    let _ = std::fs::remove_file(&missing_target);
    clear_state_storage_policy_overrides();
}

#[test]
fn corrupt_state_file_quarantines_and_resets_when_enabled() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("corrupt_state_quarantine_reset");
    reset_for_tests();

    std::env::set_var(
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
    );
    std::fs::write(&state_path, b"{invalid-state").expect("write corrupt state file");

    let loaded = load_engine_state_from_storage().expect("recover from corrupted state file");
    assert!(loaded.sessions.is_empty());
    assert_eq!(loaded.refresh_epoch_counter, 0);
    assert!(!state_path.exists());

    let backups =
        sorted_corrupted_state_backups(&state_path).expect("list corrupted state backups");
    assert_eq!(backups.len(), 1);
    let backup_contents = std::fs::read(&backups[0]).expect("read backup file contents");
    assert_eq!(backup_contents, b"{invalid-state");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn empty_state_file_quarantines_and_resets_when_enabled() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("empty_state_quarantine_reset");
    reset_for_tests();

    std::env::set_var(
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
    );
    std::fs::write(&state_path, b"").expect("truncate state file to zero bytes");

    let loaded = load_engine_state_from_storage().expect("quarantine empty state file");
    assert!(loaded.sessions.is_empty());
    assert_eq!(loaded.refresh_epoch_counter, 0);
    assert!(!state_path.exists());

    let backups =
        sorted_corrupted_state_backups(&state_path).expect("list corrupted state backups");
    assert_eq!(backups.len(), 1);
    assert_eq!(
        std::fs::metadata(&backups[0])
            .expect("empty backup exists")
            .len(),
        0
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

// The plaintext-acceptance path is debug-only (legacy_plaintext_state_permitted
// gates on cfg!(debug_assertions)), so this rollback-path test is too; in a
// release build the bytes are always refused before schema validation is reached.
#[cfg(debug_assertions)]
#[test]
fn schema_mismatch_state_file_fails_closed_by_default() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("schema_mismatch_fail_closed");
    reset_for_tests();

    let unsupported_schema_version = if PERSISTED_STATE_SCHEMA_VERSION == u16::MAX {
        0
    } else {
        PERSISTED_STATE_SCHEMA_VERSION + 1
    };
    let persisted = PersistedEngineState {
        schema_version: unsupported_schema_version,
        sessions: HashMap::new(),
        refresh_epoch_counter: 0,
        operator_fault_scores: BTreeMap::new(),
        quarantined_operator_identifiers: vec![],
        canary_rollout: CanaryRolloutState::default(),
    };
    let persisted_bytes = serde_json::to_vec(&persisted).expect("encode mismatched schema");
    std::fs::write(&state_path, &persisted_bytes).expect("write mismatched schema state file");

    // Schema validation runs only after the plaintext gate, so opt into the
    // legacy plaintext rollback path (development profile + flag) to reach it.
    std::env::set_var(TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV, "true");
    let err = match load_engine_state_from_storage() {
        Ok(_) => panic!("expected schema mismatch failure"),
        Err(err) => err,
    };
    std::env::remove_var(TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV);
    assert!(matches!(err, EngineError::Internal(_)));

    let err_message = err.to_string();
    assert!(err_message.contains("failed to validate signer state file"));
    assert!(err_message.contains("unsupported signer state schema version"));
    assert!(err_message.contains(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV));
    assert!(state_path.exists());

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[cfg(debug_assertions)] // plaintext rollback path is debug-only; see legacy_plaintext_state_permitted
#[test]
fn schema_mismatch_state_file_quarantines_and_resets_when_enabled() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("schema_mismatch_quarantine_reset");
    reset_for_tests();

    std::env::set_var(
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
    );

    let unsupported_schema_version = if PERSISTED_STATE_SCHEMA_VERSION == u16::MAX {
        0
    } else {
        PERSISTED_STATE_SCHEMA_VERSION + 1
    };
    let persisted = PersistedEngineState {
        schema_version: unsupported_schema_version,
        sessions: HashMap::new(),
        refresh_epoch_counter: 0,
        operator_fault_scores: BTreeMap::new(),
        quarantined_operator_identifiers: vec![],
        canary_rollout: CanaryRolloutState::default(),
    };
    let persisted_bytes = serde_json::to_vec(&persisted).expect("encode mismatched schema");
    std::fs::write(&state_path, &persisted_bytes).expect("write mismatched schema state file");

    // Reach schema validation (the test's intent) by opting into the plaintext
    // rollback path; otherwise the plaintext gate refuses the bytes before the
    // schema check and the quarantine-and-reset would fire for the wrong reason.
    std::env::set_var(TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV, "true");
    let loaded = load_engine_state_from_storage().expect("recover from schema mismatch state");
    std::env::remove_var(TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV);
    assert!(loaded.sessions.is_empty());
    assert_eq!(loaded.refresh_epoch_counter, 0);
    assert!(!state_path.exists());

    let backups =
        sorted_corrupted_state_backups(&state_path).expect("list corrupted state backups");
    assert_eq!(backups.len(), 1);
    let backup_contents = std::fs::read(&backups[0]).expect("read backup file contents");
    assert_eq!(backup_contents, persisted_bytes);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn corrupt_state_backup_retention_evicts_old_backups() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("corrupt_state_retention");
    reset_for_tests();

    std::env::set_var(
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
    );
    std::env::set_var(TBTC_SIGNER_STATE_CORRUPT_BACKUP_LIMIT_ENV, "2");

    for seed in 0..4 {
        std::fs::write(&state_path, format!("{{invalid-state-{seed}"))
            .expect("write corrupt state");
        let loaded =
            load_engine_state_from_storage().expect("recover from corrupt state iteration");
        assert!(loaded.sessions.is_empty());
    }

    let backups =
        sorted_corrupted_state_backups(&state_path).expect("list corrupted state backups");
    assert_eq!(backups.len(), 2);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[cfg(debug_assertions)] // plaintext rollback path is debug-only; see legacy_plaintext_state_permitted
#[test]
fn legacy_plaintext_state_migrates_to_encrypted_envelope_on_load() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("legacy_plaintext_migration");
    reset_for_tests();

    let mut sessions = HashMap::new();
    sessions.insert(
        "legacy-session".to_string(),
        persisted_session_state_fixture(),
    );
    let plaintext_state = PersistedEngineState {
        schema_version: PERSISTED_STATE_SCHEMA_VERSION,
        sessions,
        refresh_epoch_counter: 7,
        operator_fault_scores: BTreeMap::new(),
        quarantined_operator_identifiers: vec![],
        canary_rollout: CanaryRolloutState::default(),
    };
    let plaintext_bytes = serde_json::to_vec(&plaintext_state).expect("encode plaintext state");
    std::fs::write(&state_path, &plaintext_bytes).expect("write plaintext state file");

    // Without the opt-in rollback flag the unauthenticated plaintext is refused
    // (fail-closed), even in a non-production profile.
    assert!(
        load_engine_state_from_storage().is_err(),
        "plaintext signer state must be rejected without the rollback opt-in"
    );

    // Plaintext load is an opt-in emergency-rollback path: development profile
    // (selected by reset_for_tests) + this flag.
    std::env::set_var(TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV, "true");
    let loaded = load_engine_state_from_storage().expect("load and migrate legacy plaintext");
    std::env::remove_var(TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV);
    assert_eq!(loaded.sessions.len(), 1);
    assert_eq!(loaded.refresh_epoch_counter, 7);

    let migrated_bytes = std::fs::read(&state_path).expect("read migrated state file");
    let envelope: PersistedEncryptedEngineStateEnvelope =
        serde_json::from_slice(&migrated_bytes).expect("decode migrated encrypted envelope");
    assert_eq!(
        envelope.schema_version,
        PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION
    );
    assert!(!envelope.ciphertext.is_empty());

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn legacy_v2_encrypted_state_rewrites_with_current_key_id() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("encrypted_state_v2_legacy_key_id");
    reset_for_tests();

    let persisted_state = PersistedEngineState {
        schema_version: PERSISTED_STATE_SCHEMA_VERSION,
        sessions: HashMap::new(),
        refresh_epoch_counter: 11,
        operator_fault_scores: BTreeMap::new(),
        quarantined_operator_identifiers: vec![],
        canary_rollout: CanaryRolloutState::default(),
    };
    let mut plaintext =
        serde_json::to_vec(&persisted_state).expect("encode persisted state fixture");
    let key_material = state_encryption_key_material().expect("load test state key");
    let cipher =
        XChaCha20Poly1305::new_from_slice(&key_material.key[..]).expect("initialize test cipher");
    let nonce_bytes = [7u8; TBTC_SIGNER_STATE_ENVELOPE_NONCE_BYTES];
    let nonce = XNonce::from_slice(&nonce_bytes);
    let mut ciphertext_and_tag = cipher
        .encrypt(nonce, plaintext.as_ref())
        .expect("encrypt legacy v2 envelope fixture");
    plaintext.zeroize();
    let mut authentication_tag = ciphertext_and_tag
        .split_off(ciphertext_and_tag.len() - TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES);
    let envelope = PersistedEncryptedEngineStateEnvelope {
        schema_version: PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION_V2,
        encryption_algorithm: TBTC_SIGNER_STATE_ENCRYPTION_ALGORITHM_XCHACHA20POLY1305.to_string(),
        key_provider: TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT.to_string(),
        key_id: TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX.to_string(),
        nonce: hex::encode(nonce_bytes),
        ciphertext: hex::encode(&ciphertext_and_tag),
        authentication_tag: hex::encode(&authentication_tag),
    };
    ciphertext_and_tag.zeroize();
    authentication_tag.zeroize();
    std::fs::write(
        &state_path,
        serde_json::to_vec(&envelope).expect("encode legacy v2 envelope"),
    )
    .expect("write legacy v2 envelope");

    let loaded = load_engine_state_from_storage().expect("load legacy v2 envelope");
    assert_eq!(loaded.refresh_epoch_counter, 11);

    let rewritten_bytes = std::fs::read(&state_path).expect("read rewritten envelope");
    let rewritten: PersistedEncryptedEngineStateEnvelope =
        serde_json::from_slice(&rewritten_bytes).expect("decode rewritten envelope");
    assert_eq!(
        rewritten.schema_version,
        PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION
    );
    assert!(rewritten.key_id.starts_with("sha256:"));
    assert_ne!(rewritten.key_id, TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn env_key_provider_is_rejected_in_production_profile() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("production_rejects_env_provider");
    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    configure_valid_provenance_attestation_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT,
    );

    let err = persist_state_for_key_provider_test("session-production-rejects-env-provider")
        .expect_err("production profile should reject env provider");
    expect_internal_error_contains(err, "is not allowed in profile [production]");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn production_profile_rejects_implicit_temp_state_path() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::remove_var(TBTC_SIGNER_STATE_PATH_ENV);
    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    configure_valid_provenance_attestation_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        format!("printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
    );

    let err = persist_state_for_key_provider_test("session-production-rejects-implicit-state-path")
        .expect_err("production profile should reject implicit state path");
    expect_internal_error_contains(
        err,
        "refusing to use the implicit temp-dir signer state path",
    );

    reset_for_tests();
    clear_state_storage_policy_overrides();
}

#[test]
fn unknown_state_key_provider_is_rejected() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("unknown_state_key_provider");
    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_STATE_KEY_PROVIDER_ENV, "hsm");

    let err = persist_state_for_key_provider_test("session-unknown-state-key-provider")
        .expect_err("unsupported state key provider should fail closed");
    expect_internal_error_contains(err, "unsupported state key provider");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn command_key_provider_rejects_non_zero_exit() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("production_command_provider_non_zero_exit");
    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    configure_valid_provenance_attestation_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV, "exit 17");

    let err =
        persist_state_for_key_provider_test("session-production-command-provider-non-zero-exit")
            .expect_err("non-zero command exit should fail closed");
    expect_internal_error_contains(err, "exited with non-zero status");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn command_key_provider_rejects_bad_output() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("production_command_provider_bad_output");
    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    configure_valid_provenance_attestation_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        "printf 'zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz\\n'",
    );

    let err = persist_state_for_key_provider_test("session-production-command-provider-bad-output")
        .expect_err("bad command output should fail closed");
    expect_internal_error_contains(err, "must be valid hex");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn command_key_provider_drains_large_stderr_without_deadlock() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("production_command_provider_large_stderr");
    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    configure_valid_provenance_attestation_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV, "2");
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        format!(
            "dd if=/dev/zero bs=70000 count=1 1>&2 2>/dev/null; printf '{}\\n'",
            TEST_STATE_ENCRYPTION_KEY_HEX
        ),
    );

    persist_state_for_key_provider_test("session-production-command-provider-large-stderr")
        .expect("large stderr from state key command should not deadlock");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn command_key_provider_times_out_fail_closed() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("production_command_provider_timeout");
    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    configure_valid_provenance_attestation_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV, "1");
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        format!("sleep 2; printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
    );

    let err = persist_state_for_key_provider_test("session-production-command-provider-timeout")
        .expect_err("state key command timeout should fail closed");
    expect_internal_error_contains(err, "timed out");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
#[cfg(unix)]
fn command_key_provider_times_out_when_background_descendant_keeps_pipe_open() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("production_command_provider_background_pipe");
    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    configure_valid_provenance_attestation_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_TIMEOUT_SECS_ENV, "1");
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        format!("sleep 5 & printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
    );

    let started_at = Instant::now();
    let err =
        persist_state_for_key_provider_test("session-production-command-provider-background-pipe")
            .expect_err("state key command pipe timeout should fail closed");
    assert!(
        started_at.elapsed() < Duration::from_secs(4),
        "state key command should not wait for background descendant pipe EOF"
    );
    expect_internal_error_contains(err, "timed out");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn command_key_provider_survives_restart_with_stable_key() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("production_command_provider");
    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);
    configure_valid_provenance_attestation_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        format!("printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
    );

    persist_state_for_key_provider_test("session-production-command-provider")
        .expect("seed encrypted state with command provider");

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    {
        let state = state().expect("engine state should initialize");
        let guard = state.lock().expect("engine lock");
        assert!(guard
            .sessions
            .contains_key("session-production-command-provider"));
    }

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

// --- init-time signer config (frost_tbtc_init_signer_config) ---

/// Clears the installed config on drop so a panicking test cannot leak an
/// installed snapshot into unrelated tests that expect env-fallback mode.
struct InstalledConfigClearGuard;

impl Drop for InstalledConfigClearGuard {
    fn drop(&mut self) {
        clear_installed_signer_config_for_tests();
    }
}

#[test]
fn init_signer_config_overrides_environment_for_covered_knobs() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "120000");
    assert_eq!(roast_coordinator_timeout_ms(), 120_000);
    assert_eq!(
        heartbeat_rate_limit_per_minute().unwrap(),
        TBTC_SIGNER_DEFAULT_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE
    );

    let result = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        roast_coordinator_timeout_ms: Some(60_000),
        policy_heartbeat_rate_limit_per_minute: Some(7),
        canary_max_interactive_round1_p95_ms: Some(101),
        canary_max_interactive_round2_p95_ms: Some(202),
        canary_max_interactive_aggregate_p95_ms: Some(303),
        canary_min_samples: Some(42),
        canary_min_policy_samples: Some(7),
        canary_max_sample_age_seconds: Some(900),
        ..InitSignerConfigRequest::default()
    })
    .expect("install config");
    assert!(result.installed);
    assert!(!result.idempotent);
    assert_eq!(result.configured_key_count, 9);

    assert_eq!(roast_coordinator_timeout_ms(), 60_000);
    assert_eq!(heartbeat_rate_limit_per_minute().unwrap(), 7);
    assert_eq!(canary_max_interactive_round1_p95_ms(), 101);
    assert_eq!(canary_max_interactive_round2_p95_ms(), 202);
    assert_eq!(canary_max_interactive_aggregate_p95_ms(), 303);
    assert_eq!(canary_min_samples(), 42);
    assert_eq!(canary_min_policy_samples(), 7);
    assert_eq!(canary_max_sample_age_seconds(), 900);
    std::env::remove_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV);
}

#[test]
fn init_signer_config_rejects_zero_heartbeat_rate_limit_without_installing() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let error = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        policy_heartbeat_rate_limit_per_minute: Some(0),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("a zero heartbeat rate limit must fail init");
    assert!(
        error
            .to_string()
            .contains(TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV),
        "unexpected error: {error}"
    );

    // Failed candidate validation must not install a snapshot.
    std::env::set_var(TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV, "9");
    assert_eq!(heartbeat_rate_limit_per_minute().unwrap(), 9);
    std::env::remove_var(TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV);
}

#[test]
fn init_signer_config_ignores_environment_wholesale_for_unset_fields() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    // A valid env override that would normally win...
    std::env::set_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV, "120");
    assert_eq!(refresh_cadence_seconds(), 120);

    // ...is ignored once a config is installed, even though the installed
    // config does not set that field: absent field = built-in default.
    init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        roast_coordinator_timeout_ms: Some(60_000),
        ..InitSignerConfigRequest::default()
    })
    .expect("install config");

    assert_eq!(
        refresh_cadence_seconds(),
        TBTC_SIGNER_DEFAULT_REFRESH_CADENCE_SECONDS
    );
    std::env::remove_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV);
}

#[test]
fn init_signer_config_is_idempotent_for_identical_request_and_rejects_conflicts() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let request = InitSignerConfigRequest {
        profile: Some("development".to_string()),
        max_sessions: Some(64),
        ..InitSignerConfigRequest::default()
    };
    let first = init_signer_config(request.clone()).expect("first install");
    assert!(!first.idempotent);

    let second = init_signer_config(request).expect("identical re-init");
    assert!(second.idempotent);
    assert_eq!(second.config_fingerprint, first.config_fingerprint);

    let conflict = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        max_sessions: Some(128),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("conflicting re-init must be rejected");
    let message = conflict.to_string();
    assert!(
        message.contains("conflicting re-initialization rejected"),
        "unexpected error: {message}"
    );
}

#[test]
fn init_signer_config_rejects_invalid_profile_without_installing() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let error = init_signer_config(InitSignerConfigRequest {
        profile: Some("staging".to_string()),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("invalid profile must be rejected");
    assert!(error.to_string().contains("profile"), "{error}");

    // Nothing installed: environment reads still apply.
    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "120000");
    assert_eq!(roast_coordinator_timeout_ms(), 120_000);
    std::env::remove_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV);
}

#[test]
fn init_signer_config_rolls_back_install_when_policy_validation_fails() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    // Firewall enforcement on with an INVALID policy (a UTC start hour without a
    // matching end hour) -> the loader rejects and the install must roll back.
    // Absent firewall knobs no longer trip this: the loader now falls back to
    // conservative built-in defaults, so only an explicitly-invalid value fails.
    let error = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        enforce_signing_policy_firewall: Some(true),
        policy_allowed_utc_start_hour: Some(8),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("invalid firewall policy must fail the init");
    assert!(
        error.to_string().contains("must be configured together"),
        "unexpected error: {error}"
    );

    // Rolled back: env fallback is live again.
    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "120000");
    assert_eq!(roast_coordinator_timeout_ms(), 120_000);
    std::env::remove_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV);
}

#[test]
fn init_signer_config_validates_complete_admission_policy_at_install() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let result = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        enforce_admission_policy: Some(true),
        admission_min_participants: Some(3),
        admission_min_threshold: Some(2),
        ..InitSignerConfigRequest::default()
    })
    .expect("complete admission policy installs");
    assert_eq!(result.configured_key_count, 4);

    let config = load_admission_policy_config()
        .expect("load admission policy")
        .expect("admission policy enforced");
    assert_eq!(config.min_participants, 3);
    assert_eq!(config.min_threshold, 2);
}

#[test]
fn init_signer_config_keeps_state_encryption_key_on_environment_channel() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    // reset_for_tests points the env key at TEST_STATE_ENCRYPTION_KEY_HEX.
    // Installing a config that selects the env provider but (by design)
    // cannot carry the key itself must still resolve the key from the real
    // environment.
    init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        state_key_provider: Some("env".to_string()),
        ..InitSignerConfigRequest::default()
    })
    .expect("install config");

    let material = state_encryption_key_material().expect("key material resolves from env");
    assert_eq!(
        material.key_provider,
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT
    );
}

#[test]
fn init_signer_config_production_profile_forces_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let (test_trust_root, test_payload, test_signature) = build_signed_provenance_attestation(
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        TBTC_SIGNER_RUNTIME_VERSION,
        Some(now_unix() + 3600),
    );

    // lock_test_state pins the env profile to development; the installed
    // config must override it wholesale.
    init_signer_config(InitSignerConfigRequest {
        profile: Some("production".to_string()),
        state_path: Some(
            std::env::temp_dir()
                .join("frost_init_config_prod_profile_state.json")
                .to_string_lossy()
                .into_owned(),
        ),
        state_key_provider: Some("command".to_string()),
        state_key_command: Some("/nonexistent/key-helper-never-run-at-init".to_string()),
        provenance_attestation_status: Some("approved".to_string()),
        provenance_trust_root: Some(test_trust_root.clone()),
        provenance_attestation_payload: Some(test_payload.clone()),
        provenance_attestation_signature_hex: Some(test_signature.clone()),
        min_approved_version: Some(TBTC_SIGNER_RUNTIME_VERSION.to_string()),
        ..InitSignerConfigRequest::default()
    })
    .expect("install config");

    assert!(signer_profile_is_production());
    assert!(roast_strict_mode_enabled());
}

#[test]
fn reset_for_tests_clears_installed_signer_config() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        roast_coordinator_timeout_ms: Some(60_000),
        ..InitSignerConfigRequest::default()
    })
    .expect("install config");
    assert_eq!(roast_coordinator_timeout_ms(), 60_000);

    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "120000");
    assert_eq!(roast_coordinator_timeout_ms(), 120_000);
    std::env::remove_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV);
}

#[test]
fn init_signer_config_request_rejects_unknown_fields() {
    // The misspelled field name below is intentional: the request type is
    // `#[serde(deny_unknown_fields)]`, so a non-existent key must fail the
    // parse. A correctly-spelled `policy_max_output_count` would either be
    // accepted (if the field exists) or fail for the wrong reason, defeating
    // the test.
    let parsed: Result<InitSignerConfigRequest, _> =
        serde_json::from_str(r#"{"polciy_max_output_count": 1}"#);
    assert!(parsed.is_err(), "typo'd field names must fail the parse");
}

#[test]
fn init_signer_config_canonicalizes_list_and_bool_encodings() {
    let values = config_values_from_request(&InitSignerConfigRequest {
        enable_auto_quarantine: Some(false),
        auto_quarantine_dao_allowlist_identifiers: Some(vec![3, 1, 2, 2]),
        policy_allowed_script_classes: Some(vec!["P2TR".to_string(), "p2wpkh".to_string()]),
        permit_plaintext_state_rollback: Some(true),
        ..InitSignerConfigRequest::default()
    })
    .expect("convert request");

    assert_eq!(
        values
            .get(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV)
            .map(String::as_str),
        Some("false")
    );
    assert_eq!(
        values
            .get(TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV)
            .map(String::as_str),
        Some("1,2,3")
    );
    // Raw values are inserted; the existing loader normalizes case exactly as
    // it does for environment values.
    assert_eq!(
        values
            .get(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV)
            .map(String::as_str),
        Some("P2TR,p2wpkh")
    );
    // The plaintext rollback opt-in is reachable via init-time config, not only
    // the process environment.
    assert_eq!(
        values
            .get(TBTC_SIGNER_PERMIT_PLAINTEXT_STATE_ROLLBACK_ENV)
            .map(String::as_str),
        Some("true")
    );

    let empty_list = config_values_from_request(&InitSignerConfigRequest {
        admission_required_identifiers: Some(Vec::new()),
        ..InitSignerConfigRequest::default()
    });
    assert!(
        empty_list.is_err(),
        "empty identifier list must be rejected"
    );
}

#[test]
fn init_signer_config_rejects_production_config_without_state_path() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    // Explicit production AND production-by-omission (the wholesale default
    // when the profile field is unset) must both fail the init when no
    // state_path is configured, instead of installing and then failing at
    // the first state access.
    for request in [
        InitSignerConfigRequest {
            profile: Some("production".to_string()),
            ..InitSignerConfigRequest::default()
        },
        InitSignerConfigRequest {
            roast_coordinator_timeout_ms: Some(60_000),
            ..InitSignerConfigRequest::default()
        },
    ] {
        let error = init_signer_config(request)
            .expect_err("production config without state_path must fail the init");
        assert!(
            error
                .to_string()
                .contains("refusing to use the implicit temp-dir signer state path"),
            "unexpected error: {error}"
        );
    }

    // Nothing installed: environment reads still apply.
    std::env::set_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV, "120000");
    assert_eq!(roast_coordinator_timeout_ms(), 120_000);
    std::env::remove_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV);
}

#[test]
fn init_signer_config_rejects_production_config_defaulting_to_env_key_provider() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    // Wholesale semantics: leaving state_key_provider unset in the config
    // defaults to the env provider even if the environment exported
    // "command" - and production forbids the env provider. This must fail
    // the init, not the first state access.
    std::env::set_var(TBTC_SIGNER_STATE_KEY_PROVIDER_ENV, "command");
    let error = init_signer_config(InitSignerConfigRequest {
        profile: Some("production".to_string()),
        state_path: Some("/var/lib/tbtc/signer-state.json".to_string()),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("production config defaulting to the env key provider must fail the init");
    assert!(
        error.to_string().contains("is not allowed in profile"),
        "unexpected error: {error}"
    );
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_PROVIDER_ENV);
}

#[test]
fn init_signer_config_rejects_command_key_provider_without_command() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let error = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        state_key_provider: Some("command".to_string()),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("command key provider without a command must fail the init");
    assert!(
        error
            .to_string()
            .contains("missing required state key command env"),
        "unexpected error: {error}"
    );
}

#[test]
fn init_signer_config_rejects_unknown_state_key_provider() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let error = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        state_key_provider: Some("kms".to_string()),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("unknown key provider must fail the init");
    assert!(
        error.to_string().contains("unsupported state key provider"),
        "unexpected error: {error}"
    );
}

#[test]
fn init_signer_config_validates_command_key_provider_without_executing_it() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let (test_trust_root, test_payload, test_signature) = build_signed_provenance_attestation(
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        TBTC_SIGNER_RUNTIME_VERSION,
        Some(now_unix() + 3600),
    );

    // The command path deliberately points at a binary that cannot succeed:
    // if init executed the key command, this install would fail. Structural
    // validation must accept it without running it.
    let result = init_signer_config(InitSignerConfigRequest {
        profile: Some("production".to_string()),
        state_path: Some("/var/lib/tbtc/signer-state.json".to_string()),
        state_key_provider: Some("command".to_string()),
        state_key_command: Some("/nonexistent/key-helper-never-run-at-init".to_string()),
        provenance_attestation_status: Some("approved".to_string()),
        provenance_trust_root: Some(test_trust_root.clone()),
        provenance_attestation_payload: Some(test_payload.clone()),
        provenance_attestation_signature_hex: Some(test_signature.clone()),
        min_approved_version: Some(TBTC_SIGNER_RUNTIME_VERSION.to_string()),
        ..InitSignerConfigRequest::default()
    })
    .expect("structurally valid production config installs without running the key command");
    assert!(result.installed);
}

#[test]
fn init_signer_config_rejects_production_config_without_provenance_attestation() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    // Production forces the provenance gate; a production config that is
    // otherwise complete but carries no attestation set is unusable for
    // every protected operation and must fail the init, not the first call.
    let error = init_signer_config(InitSignerConfigRequest {
        profile: Some("production".to_string()),
        state_path: Some("/var/lib/tbtc/signer-state.json".to_string()),
        state_key_provider: Some("command".to_string()),
        state_key_command: Some("/nonexistent/key-helper-never-run-at-init".to_string()),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("production config without provenance attestation must fail the init");
    assert!(
        error
            .to_string()
            .contains(TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV),
        "unexpected error: {error}"
    );
}

#[test]
fn init_signer_config_rejects_enforced_gate_with_unparseable_trust_root() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let (_, test_payload, test_signature) = build_signed_provenance_attestation(
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        TBTC_SIGNER_RUNTIME_VERSION,
        Some(now_unix() + 3600),
    );

    let error = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        enforce_provenance_gate: Some(true),
        provenance_attestation_status: Some("approved".to_string()),
        provenance_trust_root: Some("not-a-pubkey".to_string()),
        provenance_attestation_payload: Some(test_payload),
        provenance_attestation_signature_hex: Some(test_signature),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("enforced gate with unparseable trust root must fail the init");
    assert!(
        error
            .to_string()
            .to_ascii_lowercase()
            .contains("trust_root"),
        "unexpected error: {error}"
    );
}

#[test]
fn init_signer_config_installs_production_config_with_valid_provenance() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let (test_trust_root, test_payload, test_signature) = build_signed_provenance_attestation(
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
        TBTC_SIGNER_RUNTIME_VERSION,
        Some(now_unix() + 3600),
    );

    let result = init_signer_config(InitSignerConfigRequest {
        profile: Some("production".to_string()),
        state_path: Some("/var/lib/tbtc/signer-state.json".to_string()),
        state_key_provider: Some("command".to_string()),
        state_key_command: Some("/nonexistent/key-helper-never-run-at-init".to_string()),
        provenance_attestation_status: Some("approved".to_string()),
        provenance_trust_root: Some(test_trust_root),
        provenance_attestation_payload: Some(test_payload),
        provenance_attestation_signature_hex: Some(test_signature),
        min_approved_version: Some(TBTC_SIGNER_RUNTIME_VERSION.to_string()),
        ..InitSignerConfigRequest::default()
    })
    .expect("complete production config installs");
    assert!(result.installed);
    assert!(signer_profile_is_production());
    assert!(provenance_gate_enforced());
}

// --- Phase 7.1: hardened interactive signing session ---
//
// These tests pin the frozen-spec contracts (sections 4-5 of
// docs/phase-7-interactive-session-spec-freeze.md): engine-held nonce
// custody, Round2 verification (a)-(f) including the own-commitment
// framing defense, verify-before-consume, consumption-before-release
// marker durability, and abort/expiry/capacity semantics.

fn interactive_test_key_packages() -> BTreeMap<u16, crate::api::NativeFrostKeyPackage> {
    let fixture = deterministic_interactive_dkg_fixture(0);
    fixture
        .part3_requests
        .into_iter()
        .map(|(id, request)| {
            (
                id,
                dkg_part3(request)
                    .expect("DKG part3 for fixture")
                    .key_package,
            )
        })
        .collect()
}

// Seed a session with DKG state (threshold 2, members 1..3) from the
// deterministic fixture, so the interactive path can resolve key
// material from engine state - the request never carries it. Idempotent
// per session_id. Returns the fixture's native key packages for tests
// that also drive the stateless primitive (e.g. the non-interactive
// member in an aggregation).
fn ensure_interactive_dkg_session(
    session_id: &str,
    key_group: &str,
) -> BTreeMap<u16, crate::api::NativeFrostKeyPackage> {
    let fixture = deterministic_interactive_dkg_fixture(0);
    let mut native = BTreeMap::new();
    let mut public_key_package_native = None;
    for (id, request) in fixture.part3_requests {
        let result = dkg_part3(request).expect("DKG part3 for fixture");
        if public_key_package_native.is_none() {
            public_key_package_native = Some(result.public_key_package.clone());
        }
        native.insert(id, result.key_package);
    }
    let public_key_package_native =
        public_key_package_native.expect("fixture has at least one participant");

    let mut guard = state().expect("engine state").lock().expect("engine lock");
    let session = guard.sessions.entry(session_id.to_string()).or_default();
    if session.dkg.result.is_none() {
        let mut frost_key_packages = BTreeMap::new();
        for (id, key_package) in &native {
            let deserialized = frost::keys::KeyPackage::deserialize(
                &hex::decode(key_package.data_hex.expose_secret())
                    .expect("fixture key package hex decodes"),
            )
            .expect("fixture key package deserializes");
            frost_key_packages.insert(*id, deserialized);
        }
        let public_key_package =
            native_public_key_package_to_frost("interactive-dkg-seed", &public_key_package_native)
                .expect("fixture public key package converts");
        session.dkg.result = Some(DkgResult {
            session_id: session_id.to_string(),
            key_group: key_group.to_string(),
            participant_count: native.len() as u16,
            threshold: 2,
            created_at_unix: now_unix(),
        });
        session.dkg.key_packages = Some(frost_key_packages);
        session.dkg.public_key_package = Some(public_key_package);
    }

    native
}

fn interactive_test_attempt_context(
    session_id: &str,
    key_group: &str,
    message_bytes: &[u8],
    included_participants: &[u16],
    wire_attempt_number: u32,
) -> AttemptContext {
    let shuffle_seed = roast_attempt_shuffle_seed(
        key_group,
        session_id,
        &rfc21_message_digest(message_bytes).expect("rfc21 message digest"),
    )
    .expect("shuffle seed");
    let coordinator =
        select_coordinator_identifier(included_participants, shuffle_seed, wire_attempt_number - 1)
            .expect("coordinator selects");
    let fingerprint = roast_included_participants_fingerprint_hex(included_participants)
        .expect("included participants fingerprint");
    let attempt_id = roast_attempt_id_hex(
        session_id,
        &hash_hex(message_bytes),
        wire_attempt_number,
        coordinator,
        &fingerprint,
    )
    .expect("attempt id");

    AttemptContext {
        attempt_number: wire_attempt_number,
        coordinator_identifier: coordinator,
        included_participants: included_participants.to_vec(),
        included_participants_fingerprint: fingerprint,
        attempt_id,
    }
}

fn heartbeat_message_for_test(nonce: u64) -> [u8; 16] {
    let mut message = [0xff; 16];
    message[8..].copy_from_slice(&nonce.to_be_bytes());
    message
}

fn heartbeat_signing_message_for_test(heartbeat_message: &[u8]) -> [u8; 32] {
    let first_digest = Sha256::digest(heartbeat_message);
    Sha256::digest(first_digest).into()
}

fn heartbeat_signing_intent_for_test(message: &[u8]) -> InteractiveSigningIntent {
    InteractiveSigningIntent::Heartbeat {
        message_hex: hex::encode(message),
    }
}

#[allow(clippy::too_many_arguments)]
fn open_interactive_for_test(
    session_id: &str,
    key_group: &str,
    message_bytes: &[u8],
    included_participants: &[u16],
    wire_attempt_number: u32,
    member_identifier: u16,
    threshold: u16,
) -> Result<InteractiveSessionOpenResult, EngineError> {
    // Key material is resolved from the session's DKG state, never the
    // request, so seed that state first (idempotent).
    ensure_interactive_dkg_session(session_id, key_group);
    let attempt_context = interactive_test_attempt_context(
        session_id,
        key_group,
        message_bytes,
        included_participants,
        wire_attempt_number,
    );
    interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier,
        message_hex: hex::encode(message_bytes),
        key_group: key_group.to_string(),
        threshold,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
}

fn interactive_package_for_test(
    message_bytes: &[u8],
    commitments: Vec<NativeFrostCommitment>,
) -> String {
    new_signing_package(NewSigningPackageRequest {
        message_hex: hex::encode(message_bytes),
        commitments,
    })
    .expect("signing package builds")
    .signing_package_hex
}

fn interactive_last_activity_at_for_test(session_id: &str, member_identifier: u16) -> Instant {
    let guard = state().expect("state").lock().expect("lock");
    let session = guard.sessions.get(session_id).expect("session exists");
    session
        .interactive
        .interactive_signing
        .values()
        .find_map(|members| members.get(&member_identifier))
        .expect("member has live interactive entry")
        .last_activity_at
}

/// Returns the live interactive signing state for a member, if present.
fn live_member(session: &SessionState, member_identifier: u16) -> Option<&InteractiveSigningState> {
    session
        .interactive
        .interactive_signing
        .values()
        .find_map(|members| members.get(&member_identifier))
}

fn stateless_package_and_shares_for_test(
    message_bytes: &[u8],
    signer_ids: &[u16],
    key_packages: &BTreeMap<u16, crate::api::NativeFrostKeyPackage>,
) -> (String, Vec<NativeFrostSignatureShare>) {
    let generated = signer_ids
        .iter()
        .map(|signer_id| {
            let key_package = &key_packages[signer_id];
            let nonces = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
                key_package_identifier: key_package.identifier.clone(),
                key_package_hex: key_package.data_hex.clone(),
            })
            .expect("stateless signer nonces");
            (*signer_id, nonces)
        })
        .collect::<Vec<_>>();
    let signing_package_hex = interactive_package_for_test(
        message_bytes,
        generated
            .iter()
            .map(|(_, generated)| generated.commitment.clone())
            .collect(),
    );
    let signature_shares = generated
        .into_iter()
        .map(|(signer_id, generated)| {
            let key_package = &key_packages[&signer_id];
            sign_share(SignShareRequest {
                signing_package_hex: signing_package_hex.clone(),
                nonces_hex: generated.nonces_hex,
                key_package_identifier: key_package.identifier.clone(),
                key_package_hex: key_package.data_hex.clone(),
            })
            .expect("stateless signature share")
            .signature_share
        })
        .collect();
    (signing_package_hex, signature_shares)
}

// Regression for the deferred state-key resolution: a Round2 whose persist
// fails because the state-key command fails must NOT leave the consumption
// marker set (which would burn the attempt in-process). The key is resolved
// before the marker insert, so the failure returns cleanly with the nonces
// still live, and a retry after the key recovers still releases the share.
#[test]
fn interactive_round2_state_key_failure_does_not_burn_attempt() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-round2-key-failure";
    let key_group = "interactive-round2-key-failure-group";
    let message = [0x42u8; 32];
    let included = [1u16, 2];
    let ttl_seconds = interactive_session_ttl_seconds();
    let margin_seconds = ttl_seconds / 4;
    assert!(
        margin_seconds > 0,
        "the configured interactive TTL must provide a synthetic test margin"
    );

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("interactive session opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("interactive round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 stateless nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex.clone(),
            },
            member2.commitment.clone(),
        ],
    );
    let rejected_signing_package_hex = interactive_package_for_test(
        &[0x43u8; 32],
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );

    // Advance well within the TTL. Invalid traffic must not refresh this live
    // handle, while the later retry-preserving key failure must refresh it at
    // failure completion.
    let prior_activity = interactive_last_activity_at_for_test(session_id, 1);
    advance_interactive_clock_for_tests(ttl_seconds / 2);

    let rejected = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: rejected_signing_package_hex,
    })
    .expect_err("a wrong-message Round2 package must be rejected");
    assert!(matches!(rejected, EngineError::Validation(_)));
    assert_eq!(
        interactive_last_activity_at_for_test(session_id, 1),
        prior_activity,
        "invalid Round2 traffic must not refresh activity"
    );

    // Make the state-key command fail, then attempt Round2.
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV, "exit 7");

    let round2_started_at = interactive_now();
    let err = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect_err("round 2 must fail when the state-key command fails");
    let failure_returned_at = interactive_now();
    assert!(matches!(err, EngineError::Internal(_)), "got {err:?}");

    // The consumption marker must NOT be set: the failed Round2 must not burn
    // the attempt.
    let refreshed_activity = {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            !session
                .interactive
                .consumed_attempt_markers
                .contains(&interactive_consumed_marker(&opened.attempt_id, 1)),
            "a Round2 that failed at the state-key step must not leave a consumption marker"
        );
        let interactive = session
            .interactive
            .interactive_signing
            .values()
            .find_map(|members| members.get(&1))
            .expect("key failure leaves the nonce handle retryable");
        assert!(interactive.round1.is_some(), "Round1 nonces remain live");
        interactive.last_activity_at
    };
    assert!(
        refreshed_activity > prior_activity,
        "validated retry-preserving Round2 failure must advance activity"
    );
    assert!(
        refreshed_activity >= round2_started_at && refreshed_activity <= failure_returned_at,
        "Round2 failure activity must fall within the failed call"
    );

    // Model substantial but sub-TTL inactivity from the failed call without
    // sleeping, then restore a working key. The same attempt must release its
    // share rather than being swept on retry.
    advance_interactive_clock_for_tests(margin_seconds);
    assert!(
        interactive_now().saturating_duration_since(refreshed_activity)
            < Duration::from_secs(ttl_seconds)
    );
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_COMMAND_ENV);
    std::env::remove_var(TBTC_SIGNER_STATE_KEY_PROVIDER_ENV);

    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("round 2 retry must succeed after the key recovers");
    assert_eq!(round2.attempt_id, opened.attempt_id);
}

#[test]
fn interactive_session_full_round_trip_aggregates_bip340() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-e2e";
    let key_group = "interactive-e2e-key-group";
    let message = [0x42u8; 32];
    let included = [1u16, 2];

    // Member 1 signs through the hardened session API; member 2 signs
    // through the stateless primitive. The shares must interoperate:
    // the session layer changes custody, not cryptography.
    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("interactive session opens");
    assert!(!opened.idempotent);

    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("interactive round 1");

    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 stateless nonces");

    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex.clone(),
            },
            member2.commitment.clone(),
        ],
    );

    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("interactive round 2 releases the share");
    assert_eq!(round2.attempt_id, opened.attempt_id);

    // A completed Round2 frees the live session state immediately: the
    // resident key package + message must not linger to the TTL sweep,
    // and the capacity slot must be returned. Only the durable marker
    // remains.
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            session.interactive.interactive_signing.is_empty(),
            "completed Round2 must free the live interactive session state"
        );
        assert!(
            session
                .interactive
                .consumed_attempt_markers
                .contains(&interactive_consumed_marker(&opened.attempt_id, 1)),
            "the durable consumption marker must remain after Round2"
        );
    }

    let member2_share = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 stateless share");

    let public_key_package = dkg_part3(
        deterministic_interactive_dkg_fixture(0)
            .part3_requests
            .remove(&1)
            .expect("fixture participant 1"),
    )
    .expect("public key package")
    .public_key_package;

    let aggregate = aggregate(AggregateRequest {
        signing_package_hex,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            member2_share.signature_share,
        ],
        public_key_package: public_key_package.clone(),
    })
    .expect("aggregate");

    let signature_bytes = hex::decode(aggregate.signature_hex).expect("signature hex");
    let signature = SchnorrSignature::from_slice(&signature_bytes).expect("BIP340 signature");
    let public_key_bytes = hex::decode(public_key_package.verifying_key).expect("key hex");
    let public_key = XOnlyPublicKey::from_slice(&public_key_bytes).expect("x-only key");
    Secp256k1::verification_only()
        .verify_schnorr(&signature, &SecpMessage::from_digest(message), &public_key)
        .expect("interactive + stateless shares aggregate to a valid BIP-340 signature");
}

#[test]
fn interactive_signs_across_sessions_by_key_group() {
    // PRODUCTION SHAPE: a wallet's DKG material is persisted under its DKG session,
    // but interactive ROAST signing runs under a DIFFERENT, per-message session (the
    // RoastSessionID). The engine must resolve the wallet key by key_group so signing
    // under a distinct session still works - otherwise distributed-DKG wallets, which
    // are signable ONLY via the interactive path, could never sign. The single-session
    // tests miss this because they persist and sign under one id.
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_cross_session_compaction");
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "3");

    let key_packages = interactive_test_key_packages();
    let wallet_session = "wallet-dkg-session";
    let signing_session = "roast-signing-session";
    let key_group = "cross-session-key-group";
    let message = [0x5au8; 32];
    let included = [1u16, 2];

    // The DKG material lives ONLY under the wallet (DKG) session.
    ensure_interactive_dkg_session(wallet_session, key_group);

    // All signing runs under a DISTINCT session id, with the attempt context derived
    // from THAT signing session (coordinator/attempt id bind to the RoastSessionID,
    // unchanged by this fix).
    let open_under_signing_session = |member: u16| {
        let attempt_context =
            interactive_test_attempt_context(signing_session, key_group, &message, &included, 1);
        interactive_session_open(InteractiveSessionOpenRequest {
            session_id: signing_session.to_string(),
            member_identifier: member,
            message_hex: hex::encode(message),
            key_group: key_group.to_string(),
            threshold: 2,
            taproot_merkle_root_hex: None,
            signing_intent: None,
            attempt_context,
        })
        .unwrap_or_else(|e| panic!("member {member} opens under the signing session: {e:?}"))
    };
    let opened1 = open_under_signing_session(1);
    let opened2 = open_under_signing_session(2);
    assert_eq!(opened1.attempt_id, opened2.attempt_id);

    // The wallet material must NOT be copied into the signing session (no secret
    // duplication): the signing session holds only per-signing state, bound to the
    // wallet key by key_group.
    {
        let guard = state().expect("state").lock().expect("lock");
        let signing = guard
            .sessions
            .get(signing_session)
            .expect("signing session created on open");
        assert!(
            signing.dkg.key_packages.is_none() && signing.dkg.result.is_none(),
            "signing session must not hold a copy of the wallet DKG material"
        );
        assert_eq!(
            signing.interactive.bound_key_group.as_deref(),
            Some(key_group),
            "signing session is bound to the wallet key it signs for"
        );
    }

    let round1_m1 = interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: opened1.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1 under the signing session");
    let round1_m2 = interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: opened2.attempt_id.clone(),
        member_identifier: 2,
    })
    .expect("member 2 round 1 under the signing session");

    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1_m1.commitments_hex.clone(),
            },
            NativeFrostCommitment {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: round1_m2.commitments_hex.clone(),
            },
        ],
    );

    let round2_m1 = interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened1.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("member 1 round 2 under the signing session");
    let round2_m2 = interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened2.attempt_id.clone(),
        member_identifier: 2,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("member 2 round 2 under the signing session");

    // Non-coordinator signers stop after Round2. The now-idle outer entry must
    // leave the active budget immediately, while its exact package
    // authorization remains durable for a delayed coordinator Aggregate.
    let next_session = "next-roast-signing-session";
    build_taproot_tx(build_policy_test_request(next_session))
        .expect("a new message uses the active slot freed after Round2");
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        assert_eq!(active_session_count(&guard.sessions), 2);
        assert_eq!(retired_interactive_session_count(&guard.sessions), 1);
        assert!(guard.sessions[signing_session]
            .capacity_pins
            .retired_interactive_at_unix
            .is_some());
        assert_eq!(
            guard.sessions[signing_session]
                .interactive
                .authorized_aggregate_markers
                .len(),
            1
        );
    }

    // interactive_aggregate resolves the group public key by key_group from the wallet
    // session and produces a valid BIP-340 signature over the distinct signing session.
    let aggregated = interactive_aggregate(InteractiveAggregateRequest {
        session_id: signing_session.to_string(),
        attempt_id: opened1.attempt_id.clone(),
        signing_package_hex: signing_package_hex.clone(),
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2_m1.signature_share_hex,
            },
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: round2_m2.signature_share_hex,
            },
        ],
        taproot_merkle_root_hex: None,
    })
    .expect("interactive aggregate resolves wallet material by key_group across sessions");

    let public_key_package = dkg_part3(
        deterministic_interactive_dkg_fixture(0)
            .part3_requests
            .remove(&1)
            .expect("fixture participant 1"),
    )
    .expect("public key package")
    .public_key_package;

    let signature_bytes = hex::decode(aggregated.signature_hex).expect("signature hex");
    let signature = SchnorrSignature::from_slice(&signature_bytes).expect("BIP340 signature");
    let public_key_bytes = hex::decode(public_key_package.verifying_key).expect("key hex");
    let public_key = XOnlyPublicKey::from_slice(&public_key_bytes).expect("x-only key");
    Secp256k1::verification_only()
        .verify_schnorr(&signature, &SecpMessage::from_digest(message), &public_key)
        .expect("cross-session interactive signing produces a valid BIP-340 signature");

    // With spare room in the shared total budget, the completed per-message
    // entry retains wallet routing, exact Aggregate authorization, and typed
    // replay markers across restart while the next message is admitted.
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        assert_eq!(guard.sessions.len(), 3);
        assert!(guard.sessions.contains_key(wallet_session));
        assert!(guard.sessions.contains_key(signing_session));
        assert!(guard.sessions.contains_key(next_session));
        assert!(guard.sessions[signing_session]
            .capacity_pins
            .retired_interactive_at_unix
            .is_some());
        assert_eq!(
            guard.sessions[signing_session]
                .interactive
                .authorized_aggregate_markers
                .len(),
            1
        );
        assert_eq!(active_session_count(&guard.sessions), 2);
        assert_eq!(retired_interactive_session_count(&guard.sessions), 1);
    }

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn per_message_abort_and_expiry_retire_without_losing_policy_artifacts() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_cross_session_abort_expiry_retirement");
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "4");

    let wallet_session = "retirement-wallet";
    let key_group = "retirement-wallet-key-group";
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);

    let open = |session_id: &str, message: &[u8; 32]| {
        interactive_session_open(InteractiveSessionOpenRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: hex::encode(message),
            key_group: key_group.to_string(),
            threshold: 2,
            taproot_merkle_root_hex: None,
            signing_intent: None,
            attempt_context: interactive_test_attempt_context(
                session_id, key_group, message, &included, 1,
            ),
        })
    };

    let aborted_session = "retired-aborted-message";
    let aborted_build_request = build_policy_test_request(aborted_session);
    build_taproot_tx(aborted_build_request.clone()).expect("aborted flow builds policy artifact");
    let aborted_open = open(aborted_session, &[0x71; 32]).expect("aborted flow opens");
    interactive_round1(InteractiveRound1Request {
        session_id: aborted_session.to_string(),
        attempt_id: aborted_open.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("aborted flow round 1");
    let aborted = interactive_session_abort(InteractiveSessionAbortRequest {
        session_id: aborted_session.to_string(),
        attempt_id: Some(aborted_open.attempt_id),
    })
    .expect("abort retires the idle per-message entry");
    assert!(aborted.aborted);

    // The successful Abort itself is the durability boundary. Restart before
    // any unrelated writer can accidentally flush the in-memory binding and
    // retirement metadata; the Build-only shell must not return as an unbound
    // active entry that consumes the shared session budget.
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = &guard.sessions[aborted_session];
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some(key_group)
        );
        assert!(session.capacity_pins.retired_interactive_at_unix.is_some());
        assert!(session.signing.build_tx_request_fingerprint.is_some());
        assert!(session.signing.tx_result.is_some());
        assert!(session.interactive.interactive_signing.is_empty());
    }
    build_taproot_tx(aborted_build_request)
        .expect("retired abort entry retains its BuildTaprootTx cache");

    let expired_session = "retired-expired-message";
    build_taproot_tx(build_policy_test_request(expired_session))
        .expect("expired flow builds policy artifact");
    let expired_open = open(expired_session, &[0x72; 32]).expect("expired flow opens");
    interactive_round1(InteractiveRound1Request {
        session_id: expired_session.to_string(),
        attempt_id: expired_open.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("expired flow round 1");
    advance_interactive_clock_for_tests(interactive_session_ttl_seconds().saturating_add(1));
    let expired = interactive_round1(InteractiveRound1Request {
        session_id: expired_session.to_string(),
        attempt_id: expired_open.attempt_id,
        member_identifier: 1,
    })
    .expect_err("Round1 sweeps and rejects the expired flow");
    assert!(matches!(expired, EngineError::SessionNotFound { .. }));

    // Sweep-triggered expiry has Abort semantics and must be durable without a
    // later Build/Round2 write accidentally closing the crash window.
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = &guard.sessions[expired_session];
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some(key_group)
        );
        assert!(session.capacity_pins.retired_interactive_at_unix.is_some());
        assert!(session.signing.build_tx_request_fingerprint.is_some());
        assert!(session.signing.tx_result.is_some());
        assert!(session.interactive.interactive_signing.is_empty());
    }

    let next_session = "retirement-next-message";
    build_taproot_tx(build_policy_test_request(next_session))
        .expect("retired abort and expiry entries do not exhaust active admission");
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        assert_eq!(active_session_count(&guard.sessions), 2);
        assert_eq!(retired_interactive_session_count(&guard.sessions), 2);
        for retired_session in [aborted_session, expired_session] {
            let session = &guard.sessions[retired_session];
            assert!(session.capacity_pins.retired_interactive_at_unix.is_some());
            assert!(session.signing.build_tx_request_fingerprint.is_some());
            assert!(session.signing.tx_result.is_some());
            assert!(session.interactive.interactive_signing.is_empty());
        }
        assert!(guard.sessions.contains_key(wallet_session));
        assert!(guard.sessions.contains_key(next_session));
    }

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn first_open_persists_per_message_binding_before_restart() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_first_open_binding_restart");
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

    let wallet_session = "first-open-wallet";
    let signing_session = "first-open-message";
    let next_session = "first-open-next-message";
    let key_group = "first-open-key-group";
    let message = [0x79u8; 32];
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);
    build_taproot_tx(build_policy_test_request(signing_session))
        .expect("Build persists the initially unbound per-message shell");

    interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &message,
            &included,
            1,
        ),
    })
    .expect("the first Open durably binds the Build shell");

    // No Round1, Abort, expiry, or unrelated writer closes this window: Open
    // itself must be the durability boundary for the session's per-message role.
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = &guard.sessions[signing_session];
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some(key_group)
        );
        assert!(session.capacity_pins.retired_interactive_at_unix.is_some());
        assert!(session.interactive.interactive_signing.is_empty());
        assert!(session.signing.build_tx_request_fingerprint.is_some());
        assert!(session.signing.tx_result.is_some());
    }

    build_taproot_tx(build_policy_test_request(next_session))
        .expect("the restarted Open shell yields its retired registry slot");
    {
        let guard = state().expect("state").lock().expect("lock");
        assert!(guard.sessions.contains_key(wallet_session));
        assert!(guard.sessions.contains_key(next_session));
        assert!(!guard.sessions.contains_key(signing_session));
        assert_eq!(guard.sessions.len(), 2);
    }

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn first_open_binding_persist_failures_are_transactional_and_repairable() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_first_open_binding_faults");
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "3");

    let wallet_session = "first-open-fault-wallet";
    let pre_replace_session = "first-open-fault-pre";
    let post_replace_session = "first-open-fault-post";
    let retired_session = "first-open-fault-retired";
    let key_group = "first-open-fault-key-group";
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);
    let retired_marker = interactive_consumed_marker(&hash_hex(b"retired-attempt"), 1);
    {
        let mut guard = state().expect("state").lock().expect("lock");
        guard.sessions.insert(
            retired_session.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some(key_group.to_string()),
                    consumed_attempt_markers: HashSet::from([retired_marker.clone()]),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState {
                    retired_interactive_at_unix: Some(1),
                    ..Default::default()
                },
                audit: AuditTrail::default(),
            },
        );
    }
    build_taproot_tx(build_policy_test_request(post_replace_session))
        .expect("persist baseline wallet, retired tombstone, and Build shell");

    let request_for = |session_id: &str, message: [u8; 32]| InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            session_id, key_group, &message, &included, 1,
        ),
    };

    let pre_replace_request = request_for(pre_replace_session, [0x81; 32]);
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let pre_replace = interactive_session_open(pre_replace_request.clone())
        .expect_err("a pre-replacement binding fault must roll Open back");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        pre_replace,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    {
        let guard = state().expect("state").lock().expect("lock");
        assert!(!guard.sessions.contains_key(pre_replace_session));
        let restored = &guard.sessions[retired_session];
        assert_eq!(restored.capacity_pins.retired_interactive_at_unix, Some(1));
        assert!(restored
            .interactive
            .consumed_attempt_markers
            .contains(&retired_marker));
        assert_eq!(guard.sessions.len(), 3);
    }
    interactive_session_open(pre_replace_request)
        .expect("a healthy retry evicts the restored tombstone and opens");
    {
        let guard = state().expect("state").lock().expect("lock");
        assert!(!guard.sessions.contains_key(retired_session));
        assert!(guard.sessions[pre_replace_session]
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&1)),);
        assert_eq!(guard.sessions.len(), 3);
    }

    let post_replace_request = request_for(post_replace_session, [0x82; 32]);
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let post_replace = interactive_session_open(post_replace_request.clone())
        .expect_err("a post-replacement binding fault reports uncertain durability");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        post_replace,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = &guard.sessions[post_replace_session];
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some(key_group)
        );
        assert!(session.capacity_pins.retired_interactive_at_unix.is_some());
        assert!(session.interactive.interactive_signing.is_empty());
    }
    assert!(interactive_state_persistence_pending());

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    interactive_session_open(post_replace_request.clone())
        .expect_err("retry must repair the uncertain binding before reopening");
    clear_persist_fault_injection_for_tests();
    assert!(interactive_state_persistence_pending());
    interactive_session_open(post_replace_request)
        .expect("a healthy retry repairs, reactivates, and opens the session");
    assert!(!interactive_state_persistence_pending());
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = &guard.sessions[post_replace_session];
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some(key_group)
        );
        assert!(session.capacity_pins.retired_interactive_at_unix.is_none());
        assert!(session
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&1)));
        assert_eq!(guard.sessions.len(), 3);
    }

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn partial_member_expiry_persists_binding_before_restart() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_partial_expiry_binding_restart");
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

    let wallet_session = "partial-expiry-wallet";
    let signing_session = "partial-expiry-message";
    let next_session = "partial-expiry-next-message";
    let key_group = "partial-expiry-key-group";
    let message = [0x7au8; 32];
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);
    build_taproot_tx(build_policy_test_request(signing_session))
        .expect("Build persists the initially unbound per-message shell");

    let open = |member_identifier| {
        interactive_session_open(InteractiveSessionOpenRequest {
            session_id: signing_session.to_string(),
            member_identifier,
            message_hex: hex::encode(message),
            key_group: key_group.to_string(),
            threshold: 2,
            taproot_merkle_root_hex: None,
            signing_intent: None,
            attempt_context: interactive_test_attempt_context(
                signing_session,
                key_group,
                &message,
                &included,
                1,
            ),
        })
    };
    let member_1 = open(1).expect("member 1 opens");
    let member_2 = open(2).expect("member 2 opens");
    assert_eq!(member_1.attempt_id, member_2.attempt_id);
    for member_identifier in included {
        interactive_round1(InteractiveRound1Request {
            session_id: signing_session.to_string(),
            attempt_id: member_1.attempt_id.clone(),
            member_identifier,
        })
        .expect("both members create nonce state");
    }

    let ttl_seconds = interactive_session_ttl_seconds();
    advance_interactive_clock_for_tests(ttl_seconds / 2);
    interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: member_1.attempt_id.clone(),
        member_identifier: 2,
    })
    .expect("member 2 activity refreshes independently");
    advance_interactive_clock_for_tests(ttl_seconds / 2 + 1);
    let expired = interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: member_1.attempt_id,
        member_identifier: 1,
    })
    .expect_err("the expired member is removed before Round1 lookup");
    assert!(matches!(expired, EngineError::SessionNotFound { .. }));
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = &guard.sessions[signing_session];
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some(key_group)
        );
        assert!(session.capacity_pins.retired_interactive_at_unix.is_none());
        assert!(!session
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&1)));
        assert!(session
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&2)));
    }

    // The partial sweep is itself a durability boundary. Although member 2 is
    // still live in memory, live nonces intentionally disappear at restart;
    // the persisted binding lets load classify the shell as retired instead
    // of restoring the old unbound Build entry as an active capacity leak.
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = &guard.sessions[signing_session];
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some(key_group)
        );
        assert!(session.capacity_pins.retired_interactive_at_unix.is_some());
        assert!(session.interactive.interactive_signing.is_empty());
        assert!(session.signing.build_tx_request_fingerprint.is_some());
        assert!(session.signing.tx_result.is_some());
    }

    build_taproot_tx(build_policy_test_request(next_session))
        .expect("retired partial-expiry shell yields its shared registry slot");
    {
        let guard = state().expect("state").lock().expect("lock");
        assert!(guard.sessions.contains_key(wallet_session));
        assert!(guard.sessions.contains_key(next_session));
        assert!(!guard.sessions.contains_key(signing_session));
        assert_eq!(guard.sessions.len(), 2);
    }

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_round2_pre_replace_failure_restores_staged_retirement() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("round2_retirement_compaction_rollback");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "3");

    let key_packages = interactive_test_key_packages();
    let wallet_session = "wallet-round2-compaction-rollback";
    let signing_session = "roast-round2-compaction-rollback";
    let key_group = "round2-compaction-rollback-key-group";
    let message = [0x73u8; 32];
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);
    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        for (session_id, retired_at) in [("retired-oldest", 1), ("retired-newer", 2)] {
            guard.sessions.insert(
                session_id.to_string(),
                SessionState {
                    dkg: DkgSessionState::default(),
                    signing: LegacySigningSessionState::default(),
                    interactive: InteractiveSessionState {
                        bound_key_group: Some(key_group.to_string()),
                        ..Default::default()
                    },
                    lifecycle: LifecycleState::default(),
                    capacity_pins: OperationalState {
                        retired_interactive_at_unix: Some(retired_at),
                        ..Default::default()
                    },
                    audit: AuditTrail::default(),
                },
            );
        }
        persist_engine_state_to_storage(&guard).expect("persist initial retired tier");
    }

    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &message,
            &included,
            1,
        ),
    })
    .expect("signing session opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 commitments");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );
    let round2_request = InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    };

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let faulted = interactive_round2(round2_request.clone())
        .expect_err("the injected pre-replacement fault must fail Round2");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        faulted,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    {
        let guard = state().expect("state").lock().expect("engine lock");
        assert!(!guard.sessions.contains_key("retired-oldest"));
        assert!(guard.sessions.contains_key("retired-newer"));
        assert_eq!(retired_interactive_session_count(&guard.sessions), 1);
        let signing = &guard.sessions[signing_session];
        assert!(signing.capacity_pins.retired_interactive_at_unix.is_none());
        assert!(signing
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&1)));
        assert!(!interactive_attempt_consumed(
            &signing.interactive.consumed_attempt_markers,
            &opened.attempt_id,
            1,
        ));
    }

    interactive_round2(round2_request)
        .expect("the same live nonces remain usable once persistence recovers");
    {
        let guard = state().expect("state").lock().expect("engine lock");
        assert!(!guard.sessions.contains_key("retired-oldest"));
        assert!(guard.sessions.contains_key("retired-newer"));
        assert!(guard.sessions[signing_session]
            .capacity_pins
            .retired_interactive_at_unix
            .is_some());
        assert_eq!(retired_interactive_session_count(&guard.sessions), 2);
    }

    clear_state_storage_policy_overrides();
    cleanup_test_state_artifacts(&state_path);
}

#[test]
fn interactive_aggregate_pins_a_retired_session_while_the_engine_lock_is_released() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_aggregate_retirement_pin");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "3");

    let wallet_session = "wallet-aggregate-retirement-pin";
    let signing_session = "roast-aggregate-retirement-pin";
    let filler_session = "retired-aggregate-pin-filler";
    let newcomer_session = "retired-aggregate-pin-newcomer";
    let key_group = "aggregate-retirement-pin-key-group";
    let message = [0x75u8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(wallet_session, key_group);

    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &message,
            &included,
            1,
        ),
    })
    .expect("per-message signing session opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment.clone(),
        ],
    );
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("member 1 round 2");
    let member2_share = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 share");
    let aggregate_request = InteractiveAggregateRequest {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id,
        signing_package_hex,
        signature_shares: vec![
            NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            member2_share.signature_share,
        ],
        taproot_merkle_root_hex: None,
    };

    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        let target = guard
            .sessions
            .get_mut(signing_session)
            .expect("Round2 retains the retired signing tombstone");
        assert!(target.capacity_pins.retired_interactive_at_unix.is_some());
        target.capacity_pins.retired_interactive_at_unix = Some(1);
        guard.sessions.insert(
            filler_session.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some(key_group.to_string()),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState {
                    retired_interactive_at_unix: Some(2),
                    ..Default::default()
                },
                audit: AuditTrail::default(),
            },
        );
        persist_engine_state_to_storage(&guard).expect("persist full retired tier");
    }

    struct AggregateReleaseGuard(
        Option<std::thread::JoinHandle<Result<InteractiveAggregateResult, EngineError>>>,
    );
    impl Drop for AggregateReleaseGuard {
        fn drop(&mut self) {
            release_interactive_aggregate_unlock_for_tests();
            if let Some(handle) = self.0.take() {
                let _ = handle.join();
            }
        }
    }

    arm_interactive_aggregate_unlock_hold_for_tests();
    let aggregate = std::thread::spawn(move || interactive_aggregate(aggregate_request));
    let mut release_guard = AggregateReleaseGuard(Some(aggregate));
    let deadline = std::time::Instant::now() + std::time::Duration::from_secs(5);
    while !interactive_aggregate_unlock_held_for_tests() {
        assert!(
            std::time::Instant::now() < deadline,
            "Aggregate did not reach its unlocked cryptographic section"
        );
        std::thread::yield_now();
    }

    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        assert_eq!(
            Arc::strong_count(
                &guard.sessions[signing_session]
                    .capacity_pins
                    .aggregate_eviction_pin
            ),
            2,
            "the in-flight Aggregate must hold the transient eviction pin"
        );
        guard.sessions.insert(
            newcomer_session.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some(key_group.to_string()),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState {
                    retired_interactive_at_unix: Some(3),
                    ..Default::default()
                },
                audit: AuditTrail::default(),
            },
        );
        let removed = compact_retired_per_message_sessions(&mut guard, Some(newcomer_session));
        assert_eq!(
            removed
                .iter()
                .map(|(id, _)| id.as_str())
                .collect::<Vec<_>>(),
            vec![filler_session],
            "compaction must skip the older but in-flight Aggregate target"
        );
        assert!(guard.sessions.contains_key(signing_session));
    }

    release_interactive_aggregate_unlock_for_tests();
    let aggregate = release_guard
        .0
        .take()
        .expect("aggregate thread handle")
        .join()
        .expect("aggregate thread does not panic")
        .expect("aggregate succeeds after concurrent retirement compaction");
    drop(release_guard);
    assert_eq!(aggregate.session_id, signing_session);
    {
        let guard = state().expect("state").lock().expect("engine lock");
        let target = &guard.sessions[signing_session];
        assert_eq!(
            Arc::strong_count(&target.capacity_pins.aggregate_eviction_pin),
            1,
            "the transient eviction pin releases after Aggregate completes"
        );
        assert_eq!(target.interactive.aggregated_attempt_markers.len(), 1);
    }

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn retired_compaction_preserves_pending_marker_sessions_until_snapshot_covers_them() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("retired_pending_marker_compaction");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "3");

    let key_packages = interactive_test_key_packages();
    let wallet_session = "wallet-retired-pending-marker";
    let round2_session = "a-round2-pending-marker";
    let aggregate_session = "b-aggregate-pending-marker";
    let evictable_session = "c-evictable-retired-marker";
    let key_group = "retired-pending-marker-key-group";
    let message = [0x74u8; 32];
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);

    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: round2_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            round2_session,
            key_group,
            &message,
            &included,
            1,
        ),
    })
    .expect("pending-marker session opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: round2_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("pending-marker member round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("pending-marker member 2 commitments");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );
    let round2_request = InteractiveRound2Request {
        session_id: round2_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    };
    let consumed_marker = interactive_consumed_marker(&opened.attempt_id, 1);

    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let faulted = interactive_round2(round2_request.clone())
        .expect_err("the injected post-replacement fault must leave a pending marker");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        faulted,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(interactive_round2_persistence_pending(
        round2_session,
        &consumed_marker
    ));

    let aggregated_marker = interactive_aggregated_marker(
        &hash_hex(b"aggregate-attempt"),
        &hash_hex(b"aggregate-message"),
        None,
    );
    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        guard
            .sessions
            .get_mut(round2_session)
            .expect("Round2 pending session exists")
            .capacity_pins
            .retired_interactive_at_unix = Some(1);
        guard.sessions.insert(
            aggregate_session.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some(key_group.to_string()),
                    aggregated_attempt_markers: HashSet::from([aggregated_marker.clone()]),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState {
                    retired_interactive_at_unix: Some(2),
                    ..Default::default()
                },
                audit: AuditTrail::default(),
            },
        );
        guard.sessions.insert(
            evictable_session.to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState {
                    bound_key_group: Some(key_group.to_string()),
                    ..Default::default()
                },
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState {
                    retired_interactive_at_unix: Some(3),
                    ..Default::default()
                },
                audit: AuditTrail::default(),
            },
        );
    }
    mark_persistence_pending(PersistencePendingOperation::InteractiveAggregate {
        session_id: aggregate_session.to_string(),
        aggregated_marker: aggregated_marker.clone(),
    });
    assert!(interactive_aggregate_persistence_pending(
        aggregate_session,
        &aggregated_marker
    ));

    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        let removed = compact_retired_per_message_sessions(&mut guard, None);
        assert_eq!(removed.len(), 1);
        assert_eq!(removed[0].0, evictable_session);
        assert!(guard.sessions.contains_key(round2_session));
        assert!(guard.sessions.contains_key(aggregate_session));
        persist_engine_state_to_storage(&guard)
            .expect("a successful snapshot covers both protected markers");
    }
    assert!(!interactive_round2_persistence_pending(
        round2_session,
        &consumed_marker
    ));
    assert!(!interactive_aggregate_persistence_pending(
        aggregate_session,
        &aggregated_marker
    ));

    let uncovered_session = "missing-pending-marker-session";
    let uncovered_marker = interactive_consumed_marker(&hash_hex(b"missing-attempt"), 1);
    let uncovered_operation = PersistencePendingOperation::InteractiveRound2 {
        session_id: uncovered_session.to_string(),
        consumed_marker: uncovered_marker.clone(),
    };
    mark_persistence_pending(uncovered_operation.clone());
    {
        let guard = state().expect("state").lock().expect("engine lock");
        persist_engine_state_to_storage(&guard)
            .expect("an unrelated snapshot can succeed without the missing marker");
    }
    assert!(
        interactive_round2_persistence_pending(uncovered_session, &uncovered_marker),
        "a snapshot that omits the exact marker must not clear its repair obligation"
    );
    clear_persistence_pending_operation(&uncovered_operation);

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("engine lock");
        assert!(guard.sessions[round2_session]
            .interactive
            .consumed_attempt_markers
            .contains(&consumed_marker));
        assert!(guard.sessions[aggregate_session]
            .interactive
            .aggregated_attempt_markers
            .contains(&aggregated_marker));
    }
    let replay = interactive_round2(round2_request)
        .expect_err("the covered Round2 marker remains fail-closed after restart");
    assert!(matches!(replay, EngineError::ConsumedNonceReplay { .. }));

    clear_state_storage_policy_overrides();
    cleanup_test_state_artifacts(&state_path);
}

#[test]
fn interactive_multi_seat_two_members_one_process_aggregate_bip340() {
    // Multi-seat: ONE process drives TWO local members through the interactive
    // session API for the SAME session and attempt - the case the pre-multi-seat
    // engine rejected with SessionConflict. Both produce real shares; member 1's
    // Round2 must NOT disturb member 2's live entry, and member 1's consumed
    // marker must NOT block member 2 for the same attempt. The two interactive
    // shares aggregate to a valid BIP-340 signature.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-multi-seat";
    let key_group = "interactive-multi-seat-key-group";
    let message = [0x77u8; 32];
    let included = [1u16, 2];

    // Both seats open the SAME session + attempt. With the per-member map this
    // succeeds for both (was SessionConflict for the second seat).
    let opened1 = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("member 1 opens");
    let opened2 = open_interactive_for_test(session_id, key_group, &message, &included, 1, 2, 2)
        .expect("member 2 opens the same attempt (multi-seat)");
    assert_eq!(
        opened1.attempt_id, opened2.attempt_id,
        "both local seats sign the same attempt"
    );

    // Independent Round1 per member.
    let round1_m1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened1.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1");
    let round1_m2 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened2.attempt_id.clone(),
        member_identifier: 2,
    })
    .expect("member 2 round 1");

    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1_m1.commitments_hex.clone(),
            },
            NativeFrostCommitment {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: round1_m2.commitments_hex.clone(),
            },
        ],
    );

    // Member 1's Round2 releases its share and frees ONLY its own entry.
    let round2_m1 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened1.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("member 1 round 2");

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            !session
                .interactive
                .interactive_signing
                .values()
                .any(|members| members.contains_key(&1)),
            "member 1's entry is freed after its Round2"
        );
        assert!(
            session
                .interactive
                .interactive_signing
                .values()
                .any(|members| members.contains_key(&2)),
            "member 2's entry stays live - a sibling seat's Round2 must not free it"
        );
        assert!(
            session
                .interactive
                .consumed_attempt_markers
                .contains(&interactive_consumed_marker(&opened1.attempt_id, 1)),
            "member 1's consumed marker is written"
        );
        assert!(
            !session
                .interactive
                .consumed_attempt_markers
                .contains(&interactive_consumed_marker(&opened2.attempt_id, 2)),
            "member 2's marker is NOT written by member 1's Round2"
        );
    }

    // Member 2's Round2 is NOT blocked by member 1's same-attempt marker.
    let round2_m2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened2.attempt_id.clone(),
        member_identifier: 2,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("member 2 round 2 is independent of member 1's consumed marker");

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            session.interactive.interactive_signing.is_empty(),
            "both members' entries are freed after their Round2s"
        );
    }

    // The two interactive shares aggregate to a valid BIP-340 signature: real
    // multi-seat interactive signing, end to end in one process.
    let public_key_package = dkg_part3(
        deterministic_interactive_dkg_fixture(0)
            .part3_requests
            .remove(&1)
            .expect("fixture participant 1"),
    )
    .expect("public key package")
    .public_key_package;

    let aggregate = aggregate(AggregateRequest {
        signing_package_hex,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2_m1.signature_share_hex,
            },
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: round2_m2.signature_share_hex,
            },
        ],
        public_key_package: public_key_package.clone(),
    })
    .expect("aggregate");

    let signature_bytes = hex::decode(aggregate.signature_hex).expect("signature hex");
    let signature = SchnorrSignature::from_slice(&signature_bytes).expect("BIP340 signature");
    let public_key_bytes = hex::decode(public_key_package.verifying_key).expect("key hex");
    let public_key = XOnlyPublicKey::from_slice(&public_key_bytes).expect("x-only key");
    Secp256k1::verification_only()
        .verify_schnorr(&signature, &SecpMessage::from_digest(message), &public_key)
        .expect("two interactive multi-seat shares aggregate to a valid BIP-340 signature");
}

#[test]
fn interactive_round2_refused_after_aggregate_for_unsigned_sibling() {
    // Multi-seat: an attempt completes (interactive_aggregate) with one threshold
    // subset {1,2} while a third local seat is open but never signed - so it has NO
    // per-member consumed marker. Round2 must still refuse to release seat 3's share
    // for the finished attempt: completion is final (recovery is a fresh attempt),
    // and otherwise seat 3's share could combine with a signer's into a SECOND valid
    // signature over the same message. Also exercises interactive_aggregate over two
    // interactive multi-seat shares.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-multi-seat-completed";
    let key_group = "interactive-multi-seat-completed-key-group";
    let message = [0x55u8; 32];
    let included = [1u16, 2, 3];

    // Three local seats open the same attempt.
    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("member 1 opens");
    open_interactive_for_test(session_id, key_group, &message, &included, 1, 2, 2)
        .expect("member 2 opens");
    open_interactive_for_test(session_id, key_group, &message, &included, 1, 3, 2)
        .expect("member 3 opens");

    let c1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1");
    let c2 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 2,
    })
    .expect("member 2 round 1");
    // Seat 3 opens + Round1s the same attempt but is NOT in the {1,2} signing subset.
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 3,
    })
    .expect("member 3 round 1");

    // Complete the attempt with the {1,2} subset.
    let package_12 = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: c1.commitments_hex.clone(),
            },
            NativeFrostCommitment {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: c2.commitments_hex.clone(),
            },
        ],
    );
    let share1 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: package_12.clone(),
    })
    .expect("member 1 round 2");
    let share2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 2,
        signing_package_hex: package_12.clone(),
    })
    .expect("member 2 round 2");
    interactive_aggregate(InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex: package_12,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: share1.signature_share_hex,
            },
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: share2.signature_share_hex,
            },
        ],
        taproot_merkle_root_hex: None,
    })
    .expect("interactive aggregate completes the attempt");

    // The completion marker is MESSAGE-BOUND (attempt_id@digest), not the bare
    // attempt_id - so it cannot be set for this attempt id via an aggregate over a
    // different message (which would otherwise preempt this attempt's live Round2).
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            session
                .interactive
                .aggregated_attempt_markers
                .iter()
                .any(|marker| marker.starts_with(&format!("{}@", opened.attempt_id))),
            "completion marker binds attempt_id to the aggregated message digest"
        );
        assert!(
            !session
                .interactive
                .aggregated_attempt_markers
                .contains(&opened.attempt_id),
            "the bare (unbound) attempt_id marker is not written"
        );
    }

    // Aggregation proactively frees the LOCAL non-signing sibling (seat 3): it never
    // calls Round2, so its entry must not linger to the TTL sweep.
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            !session
                .interactive
                .interactive_signing
                .values()
                .any(|members| members.contains_key(&3)),
            "the non-signing sibling is freed when the attempt aggregates"
        );
    }

    // If seat 3 RE-OPENS the finalized attempt and tries to release a share (in a
    // {1,3} subset it would otherwise be valid for), the completion gate refuses it:
    // the bound marker makes the attempt/message/root final.
    open_interactive_for_test(session_id, key_group, &message, &included, 1, 3, 2)
        .expect("seat 3 re-opens the finalized attempt");
    let c3_reopened = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 3,
    })
    .expect("seat 3 round 1 after re-open");
    let package_13 = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: c1.commitments_hex.clone(),
            },
            NativeFrostCommitment {
                identifier: key_packages[&3].identifier.clone(),
                data_hex: c3_reopened.commitments_hex.clone(),
            },
        ],
    );
    let refused = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 3,
        signing_package_hex: package_13,
    })
    .expect_err("round 2 must refuse a share for an already-aggregated attempt");
    assert!(
        matches!(
            refused,
            EngineError::InteractiveAttemptAlreadyAggregated { .. }
        ),
        "unexpected error: {refused:?}"
    );

    // The refused sibling's now-dead entry is freed (its nonces zeroized) rather
    // than lingering against the live-member cap until the TTL sweep.
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            !session
                .interactive
                .interactive_signing
                .values()
                .any(|members| members.contains_key(&3)),
            "seat 3's dead entry is freed when Round2 is refused for the finalized attempt"
        );
    }
}

#[test]
fn interactive_open_advances_only_the_opening_member_attempt() {
    // Per-member live attempt: seat 1 advancing to a newer attempt replaces ONLY its
    // own entry (with fresh nonce state); a sibling seat on an older attempt is
    // untouched - seats advance independently, exactly as separate processes would.
    // A stale re-open is rejected for the member that advanced, but an idempotent
    // re-open is accepted for a sibling still on that attempt.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-multi-seat-advance-key-group";
    let session_id = "interactive-multi-seat-advance";
    let message = [0x33u8; 32];
    let included = [1u16, 2];

    // Seat 1 and seat 2 open attempt 1; seat 1 takes its round-1 nonces.
    let a1_m1 = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("member 1 opens attempt 1");
    open_interactive_for_test(session_id, key_group, &message, &included, 1, 2, 2)
        .expect("member 2 opens attempt 1");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: a1_m1.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1 on attempt 1");

    // Seat 1 advances to attempt 2; only its entry is replaced.
    let a2_m1 = open_interactive_for_test(session_id, key_group, &message, &included, 2, 1, 2)
        .expect("member 1 opens attempt 2");
    assert_ne!(a2_m1.attempt_id, a1_m1.attempt_id);

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert_eq!(
            session
                .interactive
                .interactive_signing
                .get(&a2_m1.attempt_id)
                .and_then(|members| members.get(&1))
                .expect("member 1 has live entry on attempt 2")
                .attempt_context
                .attempt_id,
            a2_m1.attempt_id,
            "seat 1 advanced to attempt 2"
        );
        assert!(
            session
                .interactive
                .interactive_signing
                .get(&a2_m1.attempt_id)
                .and_then(|members| members.get(&1))
                .expect("member 1 has live entry on attempt 2")
                .round1
                .is_none(),
            "seat 1's attempt-2 entry starts fresh (old round-1 nonces replaced)"
        );
        assert_eq!(
            session
                .interactive
                .interactive_signing
                .get(&a1_m1.attempt_id)
                .and_then(|members| members.get(&2))
                .expect("member 2 has live entry on attempt 1")
                .attempt_context
                .attempt_id,
            a1_m1.attempt_id,
            "seat 2's attempt-1 entry is untouched by seat 1's advance"
        );
    }

    // A stale re-open of attempt 1 is rejected for seat 1 (it advanced) ...
    let stale = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect_err("seat 1 cannot roll back to attempt 1");
    assert!(
        matches!(stale, EngineError::Validation(_)),
        "unexpected error: {stale:?}"
    );
    // ... but seat 2 re-opening attempt 1 is idempotent (it never advanced).
    let m2_reopen = open_interactive_for_test(session_id, key_group, &message, &included, 1, 2, 2)
        .expect("seat 2 idempotent re-open of its live attempt");
    assert!(m2_reopen.idempotent);
}

#[test]
fn interactive_honors_legacy_bare_aggregate_completion_marker() {
    // Backward compat: a completion persisted by the pre-binding engine is the BARE
    // attempt_id (not attempt_id@digest). After an upgrade it must still finalize the
    // attempt fail-closed - the Round2 completion gate refuses a fresh share for it.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-legacy-aggregate-marker";
    let key_group = "interactive-test-key-group";
    let message = [0x66u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("member 1 opens");
    let c1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1");

    // Simulate a completion persisted by the pre-binding engine: the BARE attempt_id.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        guard
            .sessions
            .get_mut(session_id)
            .expect("session")
            .interactive
            .aggregated_attempt_markers
            .insert(opened.attempt_id.clone());
    }

    // Round2 must treat the bare legacy marker as a completed attempt and fail closed
    // before any share is released.
    let package = interactive_package_for_test(
        &message,
        vec![NativeFrostCommitment {
            identifier: key_packages[&1].identifier.clone(),
            data_hex: c1.commitments_hex.clone(),
        }],
    );
    let refused = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: package,
    })
    .expect_err("a legacy bare completion marker must finalize the attempt");
    assert!(
        matches!(
            refused,
            EngineError::InteractiveAttemptAlreadyAggregated { .. }
        ),
        "unexpected error: {refused:?}"
    );
}

#[test]
fn interactive_round2_completion_marker_binds_taproot_root() {
    // The completion marker binds the taproot root: a completion recorded for one
    // root must NOT finalize the same attempt/message for a member opened with a
    // different root (the signature differs per tweak), else Round2 for the live root
    // is wrongly preempted.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-taproot-root-binding";
    let key_group = "interactive-test-key-group";
    let message = [0x44u8; 32];
    let included = [1u16, 2];

    // Member 1 opens key-path (no taproot root).
    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("member 1 opens key-path");
    let c1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1");

    let digest = hash_hex(&message);
    let package = interactive_package_for_test(
        &message,
        vec![NativeFrostCommitment {
            identifier: key_packages[&1].identifier.clone(),
            data_hex: c1.commitments_hex.clone(),
        }],
    );

    // A completion recorded for a DIFFERENT taproot root must not finalize this
    // member's key-path attempt: Round2 gets past the completion gate (and then fails
    // on the deliberately sub-threshold package, not as already-aggregated).
    {
        let mut guard = state().expect("state").lock().expect("lock");
        guard
            .sessions
            .get_mut(session_id)
            .expect("session")
            .interactive
            .aggregated_attempt_markers
            .insert(interactive_aggregated_marker(
                &opened.attempt_id,
                &digest,
                Some(&[0x22u8; 32]),
            ));
    }
    let not_preempted = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: package.clone(),
    });
    assert!(
        !matches!(
            not_preempted,
            Err(EngineError::InteractiveAttemptAlreadyAggregated { .. })
        ),
        "a different-root completion must not finalize this attempt: {not_preempted:?}"
    );

    // A completion for THIS member's root (key-path) does finalize it.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        guard
            .sessions
            .get_mut(session_id)
            .expect("session")
            .interactive
            .aggregated_attempt_markers
            .insert(interactive_aggregated_marker(
                &opened.attempt_id,
                &digest,
                None,
            ));
    }
    let preempted = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: package,
    })
    .expect_err("a same-root completion finalizes the attempt");
    assert!(
        matches!(
            preempted,
            EngineError::InteractiveAttemptAlreadyAggregated { .. }
        ),
        "unexpected error: {preempted:?}"
    );
}

#[test]
fn interactive_aggregate_rejects_mismatched_message_without_cleanup() {
    // Aggregate authorization binds the canonical signing package, including
    // its message. A valid package for another message cannot create completion
    // state or delete the authorized attempt's live nonce state.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-cleanup-message-binding";
    let key_group = "interactive-test-key-group";
    let message_a = [0x88u8; 32];
    let message_b = [0x99u8; 32];
    let included = [1u16, 2];

    // A live interactive attempt over message A (attempt_id derives from message A).
    let opened = open_interactive_for_test(session_id, key_group, &message_a, &included, 1, 1, 2)
        .expect("member 1 opens message-A attempt");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1 (message A)");

    // A valid aggregate over a DIFFERENT message B, submitted under message A's
    // attempt id - via stateless shares, so it does not touch the live attempt.
    let m1_b = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
    })
    .expect("stateless nonces 1");
    let m2_b = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("stateless nonces 2");
    let package_b = interactive_package_for_test(
        &message_b,
        vec![m1_b.commitment.clone(), m2_b.commitment.clone()],
    );
    let share1_b = sign_share(SignShareRequest {
        signing_package_hex: package_b.clone(),
        nonces_hex: m1_b.nonces_hex,
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
    })
    .expect("stateless share 1 over B");
    let share2_b = sign_share(SignShareRequest {
        signing_package_hex: package_b.clone(),
        nonces_hex: m2_b.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("stateless share 2 over B");
    let err = interactive_aggregate(InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex: package_b,
        signature_shares: vec![share1_b.signature_share, share2_b.signature_share],
        taproot_merkle_root_hex: None,
    })
    .expect_err("aggregate over message B must not use message A's attempt id");
    assert!(
        matches!(err, EngineError::Validation(ref message) if message.contains("not authorized")),
        "unexpected error: {err:?}"
    );

    // The live message-A seat must survive: the cleanup is message-bound.
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            session
                .interactive
                .interactive_signing
                .values()
                .any(|members| members.contains_key(&1)),
            "a mismatched-message aggregate must not delete the live message-A seat"
        );
        assert!(session.interactive.aggregated_attempt_markers.is_empty());
    }
}

#[test]
fn interactive_capacity_counts_new_members_not_replacements() {
    // The live-member cap counts member ENTRIES: a new member takes a slot, but a
    // same-member replacement (re-open on a newer attempt) reuses its own slot.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-test-key-group";
    let message = [0xc4u8; 32];
    let included = [1u16, 2];
    let session_id = "interactive-cap-multiseat";

    std::env::set_var(TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV, "1");
    let outcome = (|| -> Result<(), EngineError> {
        // Member 1 takes the one live-member slot.
        open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)?;

        // Member 1 advancing to a NEWER attempt replaces its own entry - no new slot,
        // so it succeeds even at capacity 1 (a replacement, not an idempotent reopen).
        let advanced =
            open_interactive_for_test(session_id, key_group, &message, &included, 2, 1, 2)?;
        assert!(
            !advanced.idempotent,
            "a newer attempt is a replacement, not idempotent"
        );

        // A DIFFERENT member is a new entry, so it trips the cap and fails closed.
        let at_capacity =
            open_interactive_for_test(session_id, key_group, &message, &included, 2, 2, 2)
                .expect_err("a new member must trip the live-member cap");
        assert!(
            matches!(at_capacity, EngineError::Internal(ref m)
                if m.contains("live interactive member count")),
            "unexpected error: {at_capacity:?}"
        );
        Ok(())
    })();
    std::env::remove_var(TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV);
    outcome.expect("capacity new-vs-replacement lifecycle");
}

#[test]
fn interactive_abort_by_attempt_removes_all_members_on_that_attempt() {
    // Abort with an attempt_id filter is session-level over the member map: it removes
    // EVERY local seat on that attempt, while a sibling seat on a different attempt
    // survives.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-test-key-group";
    let message = [0xa4u8; 32];
    let included = [1u16, 2, 3];
    let session_id = "interactive-abort-multiseat";

    // Members 1 and 2 on attempt 1; member 3 on attempt 2 (a different attempt id).
    let opened1 = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("member 1 opens attempt 1");
    open_interactive_for_test(session_id, key_group, &message, &included, 1, 2, 2)
        .expect("member 2 opens attempt 1");
    open_interactive_for_test(session_id, key_group, &message, &included, 2, 3, 2)
        .expect("member 3 opens attempt 2");

    // Abort attempt 1: removes BOTH members on it; member 3 (attempt 2) is untouched.
    let result = interactive_session_abort(InteractiveSessionAbortRequest {
        session_id: session_id.to_string(),
        attempt_id: Some(opened1.attempt_id.clone()),
    })
    .expect("abort attempt 1");
    assert!(result.aborted, "abort removed live state");

    let guard = state().expect("state").lock().expect("lock");
    let session = guard.sessions.get(session_id).expect("session exists");
    assert!(
        !session
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&1)),
        "member 1 (attempt 1) is aborted"
    );
    assert!(
        !session
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&2)),
        "member 2 (attempt 1) is aborted"
    );
    assert!(
        session
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&3)),
        "member 3 (attempt 2) survives the attempt-1 abort"
    );
}

#[test]
fn interactive_round1_is_idempotent_until_consumed() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-round1-idempotent";
    let key_group = "interactive-test-key-group";
    let message = [0x21u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");

    let first = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    assert_eq!(hardening_metrics().interactive_round1_latency_samples, 1);
    assert_eq!(
        hardening_telemetry_state()
            .lock()
            .expect("hardening telemetry lock")
            .canary_interactive_round1_latency
            .sample_count(),
        1,
    );
    let second = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("repeat round 1");
    assert_eq!(
        first.commitments_hex, second.commitments_hex,
        "round 1 must be idempotent until the nonces are consumed"
    );
    assert_eq!(
        hardening_metrics().interactive_round1_latency_samples,
        2,
        "the ABI-3 rolling metric includes the idempotent call"
    );
    assert_eq!(
        hardening_telemetry_state()
            .lock()
            .expect("hardening telemetry lock")
            .canary_interactive_round1_latency
            .sample_count(),
        1,
        "an idempotent replay must not dilute the promotion latency window",
    );

    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: first.commitments_hex.clone(),
            },
            member2.commitment,
        ],
    );
    interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("round 2 consumes");

    let replay = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect_err("round 1 after consumption must fail closed");
    assert!(
        matches!(replay, EngineError::ConsumedNonceReplay { .. }),
        "unexpected error: {replay:?}"
    );
    assert_eq!(replay.code(), "consumed_nonce_replay");
    assert_eq!(
        hardening_metrics().interactive_round1_latency_samples,
        3,
        "the ABI-3 rolling metric includes the rejected call"
    );
    assert_eq!(
        hardening_telemetry_state()
            .lock()
            .expect("hardening telemetry lock")
            .canary_interactive_round1_latency
            .sample_count(),
        1,
        "a fail-fast rejection must not enter the successful promotion window",
    );
}

#[test]
fn interactive_round2_rejects_substituted_own_commitment_then_accepts_corrected() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-framing-defense";
    let key_group = "interactive-test-key-group";
    let message = [0x33u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");

    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");

    // A malicious coordinator substitutes member 1's commitment with a
    // different (validly formed) commitment for the same key package.
    // Without the own-commitment check the member would sign with its
    // true nonces over a package misrepresenting its commitment - the
    // share then fails verification at aggregation and becomes false
    // blame evidence against an honest member.
    let substituted = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
    })
    .expect("substituted commitment");
    assert_ne!(
        substituted.commitment.data_hex, round1.commitments_hex,
        "fixture sanity: substituted commitment differs"
    );

    let framed_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: substituted.commitment.data_hex,
            },
            member2.commitment.clone(),
        ],
    );
    let framed = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: framed_package_hex,
    })
    .expect_err("substituted own commitment must be rejected");
    assert!(
        matches!(framed, EngineError::Validation(ref message)
            if message.contains("does not match its round-1 output")),
        "unexpected error: {framed:?}"
    );

    // Verify-before-consume: the rejected package must NOT have burned
    // the nonces; the honest package still succeeds.
    let honest_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex.clone(),
            },
            member2.commitment,
        ],
    );
    interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id,
        member_identifier: 1,
        signing_package_hex: honest_package_hex,
    })
    .expect("honest package succeeds after the framed one was rejected");
}

#[test]
fn interactive_round2_package_shape_rejections() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let key_group = "interactive-test-key-group";
    let message = [0x44u8; 32];

    // Session A: included {1,2} - outside-set and message-mismatch.
    let session_a = "interactive-shape-a";
    let opened_a = open_interactive_for_test(session_a, key_group, &message, &[1, 2], 1, 1, 2)
        .expect("session A opens");
    let round1_a = interactive_round1(InteractiveRound1Request {
        session_id: session_a.to_string(),
        attempt_id: opened_a.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("session A round 1");

    let member3 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&3].identifier.clone(),
        key_package_hex: key_packages[&3].data_hex.clone(),
    })
    .expect("member 3 nonces");

    let outside_set_package = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1_a.commitments_hex.clone(),
            },
            member3.commitment.clone(),
        ],
    );
    let outside = interactive_round2(InteractiveRound2Request {
        session_id: session_a.to_string(),
        attempt_id: opened_a.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: outside_set_package,
    })
    .expect_err("participant outside the included set must be rejected");
    assert!(
        matches!(outside, EngineError::Validation(ref m) if m.contains("included set")),
        "unexpected error: {outside:?}"
    );

    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let wrong_message = [0x55u8; 32];
    let wrong_message_package = interactive_package_for_test(
        &wrong_message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1_a.commitments_hex.clone(),
            },
            member2.commitment.clone(),
        ],
    );
    let mismatch = interactive_round2(InteractiveRound2Request {
        session_id: session_a.to_string(),
        attempt_id: opened_a.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: wrong_message_package,
    })
    .expect_err("package over a different message must be rejected");
    assert!(
        matches!(mismatch, EngineError::Validation(ref m) if m.contains("message")),
        "unexpected error: {mismatch:?}"
    );

    // Session B: included {1,2,3}, threshold 2 - size and self-missing.
    let session_b = "interactive-shape-b";
    let opened_b = open_interactive_for_test(session_b, key_group, &message, &[1, 2, 3], 1, 1, 2)
        .expect("session B opens");
    let round1_b = interactive_round1(InteractiveRound1Request {
        session_id: session_b.to_string(),
        attempt_id: opened_b.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("session B round 1");

    let oversized_package = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1_b.commitments_hex.clone(),
            },
            member2.commitment.clone(),
            member3.commitment.clone(),
        ],
    );
    let oversized = interactive_round2(InteractiveRound2Request {
        session_id: session_b.to_string(),
        attempt_id: opened_b.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: oversized_package,
    })
    .expect_err("more than exactly-threshold commitments must be rejected");
    assert!(
        matches!(oversized, EngineError::Validation(ref m) if m.contains("exactly threshold")),
        "unexpected error: {oversized:?}"
    );

    let self_missing_package =
        interactive_package_for_test(&message, vec![member2.commitment, member3.commitment]);
    let self_missing = interactive_round2(InteractiveRound2Request {
        session_id: session_b.to_string(),
        attempt_id: opened_b.attempt_id,
        member_identifier: 1,
        signing_package_hex: self_missing_package,
    })
    .expect_err("a package excluding this member must be rejected");
    assert!(
        matches!(self_missing, EngineError::Validation(ref m)
            if m.contains("does not include this member")),
        "unexpected error: {self_missing:?}"
    );
}

#[test]
fn interactive_consumption_marker_survives_restart() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-restart-marker";
    let key_group = "interactive-test-key-group";
    let message = [0x61u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );
    interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("round 2 consumes");

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    // The durable marker must reject the consumed attempt across a
    // restart at every entry point, even though the live interactive
    // state (and its nonces) did not survive by construction.
    let reopen = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect_err("reopening a consumed attempt after restart must fail closed");
    assert!(
        matches!(reopen, EngineError::ConsumedNonceReplay { .. }),
        "unexpected error: {reopen:?}"
    );

    // A fresh attempt for the same session proceeds: the marker is
    // attempt-scoped, not session-scoped.
    let second_attempt =
        open_interactive_for_test(session_id, key_group, &message, &included, 2, 1, 2)
            .expect("a new attempt opens after restart");
    let round2_without_round1 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: second_attempt.attempt_id,
        member_identifier: 1,
        signing_package_hex: "00".repeat(8),
    })
    .expect_err("round 2 without round 1 must fail");
    assert!(
        matches!(
            round2_without_round1,
            EngineError::Validation(_) | EngineError::SignRoundNotStarted { .. }
        ),
        "unexpected error: {round2_without_round1:?}"
    );
}

#[test]
fn interactive_round2_persist_fault_leaves_nonces_live() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-persist-fault";
    let key_group = "interactive-test-key-group";
    let message = [0x71u8; 32];
    let included = [1u16, 2];
    let ttl_seconds = interactive_session_ttl_seconds();
    let margin_seconds = ttl_seconds / 4;
    assert!(margin_seconds > 0);

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex.clone(),
            },
            member2.commitment,
        ],
    );
    let prior_activity = interactive_last_activity_at_for_test(session_id, 1);
    advance_interactive_clock_for_tests(ttl_seconds / 2);

    // Consumption-before-release: if the durable marker cannot be
    // persisted, NO share leaves the engine and the nonces stay live.
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let fault_started_at = interactive_now();
    let faulted = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect_err("injected persist fault must fail round 2");
    let fault_returned_at = interactive_now();
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(faulted, EngineError::Internal(ref m) if m.contains("injected persist fault")),
        "unexpected error: {faulted:?}"
    );

    let refreshed_activity = {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            !session
                .interactive
                .consumed_attempt_markers
                .contains(&interactive_consumed_marker(&opened.attempt_id, 1)),
            "a failed persist must roll the consumption marker back"
        );
        let interactive = session
            .interactive
            .interactive_signing
            .values()
            .find_map(|members| members.get(&1))
            .expect("pre-replacement failure leaves the nonce retryable");
        assert!(interactive.round1.is_some(), "Round1 nonces remain live");
        interactive.last_activity_at
    };
    assert!(
        refreshed_activity > prior_activity,
        "pre-replacement failure completion must advance activity"
    );
    assert!(
        refreshed_activity >= fault_started_at && refreshed_activity <= fault_returned_at,
        "persistence-failure activity must fall within the failed call"
    );

    // The same attempt completes once persistence recovers - the
    // nonces were never consumed by the failed call.
    advance_interactive_clock_for_tests(margin_seconds);
    assert!(
        interactive_now().saturating_duration_since(refreshed_activity)
            < Duration::from_secs(ttl_seconds)
    );
    interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("round 2 succeeds after the persist fault clears");

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            session
                .interactive
                .consumed_attempt_markers
                .contains(&interactive_consumed_marker(&opened.attempt_id, 1)),
            "successful round 2 must leave the durable marker"
        );
    }
}

#[test]
fn interactive_round2_post_rename_persist_failure_consumes_attempt_and_retry_flushes() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_round2_post_rename");
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-round2-post-rename";
    let key_group = "interactive-test-key-group";
    let message = [0x72u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );
    let round2_request = InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    };
    let consumed_marker = interactive_consumed_marker(&opened.attempt_id, 1);

    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let faulted = interactive_round2(round2_request.clone())
        .expect_err("post-rename persist fault must release no share");
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(faulted, EngineError::Internal(ref message) if message.contains("injected persist fault")),
        "unexpected error: {faulted:?}"
    );

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(session
            .interactive
            .consumed_attempt_markers
            .contains(&consumed_marker));
        assert!(
            !session
                .interactive
                .interactive_signing
                .values()
                .any(|members| members.contains_key(&1)),
            "post-rename failure must destroy the live nonce-bearing member state"
        );
    }
    assert!(interactive_round2_persistence_pending(
        session_id,
        &consumed_marker
    ));

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let failed_flush = interactive_round2(round2_request.clone())
        .expect_err("a failed pending-marker flush must not reach the replay gate");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        failed_flush,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(interactive_round2_persistence_pending(
        session_id,
        &consumed_marker
    ));

    // Crash before any successful in-process repair. The pending registry is
    // memory-only and disappears; the replacement image must carry the marker.
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        assert!(guard.sessions[session_id]
            .interactive
            .consumed_attempt_markers
            .contains(&consumed_marker));
        assert!(guard.sessions[session_id]
            .interactive
            .interactive_signing
            .is_empty());
    }

    let retry = interactive_round2(round2_request)
        .expect_err("restart retry rejects the durable consumed attempt");
    assert!(
        matches!(retry, EngineError::ConsumedNonceReplay { .. }),
        "unexpected retry error: {retry:?}"
    );
    assert!(!interactive_round2_persistence_pending(
        session_id,
        &consumed_marker
    ));

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_open_idempotency_conflict_and_replacement() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-open-lifecycle";
    let key_group = "interactive-test-key-group";
    let message = [0x81u8; 32];
    let included = [1u16, 2];

    let first = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    assert!(!first.idempotent);

    let repeat = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("identical reopen is idempotent");
    assert!(repeat.idempotent);
    assert_eq!(repeat.attempt_id, first.attempt_id);

    // Same attempt, different request: conflicting reopen fails closed.
    let attempt_context =
        interactive_test_attempt_context(session_id, key_group, &message, &included, 1);
    let conflicting = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: Some(
            "1111111111111111111111111111111111111111111111111111111111111111".to_string(),
        ),
        signing_intent: None,
        attempt_context,
    })
    .expect_err("conflicting reopen of a live attempt must fail closed");
    assert!(
        matches!(conflicting, EngineError::SessionConflict { .. }),
        "unexpected error: {conflicting:?}"
    );

    // Round 1 for attempt 1, then open attempt 2: the retry loop has
    // moved on, so the newer attempt implicitly aborts the older one
    // and its nonces.
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: first.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1 for attempt 1");
    let second = open_interactive_for_test(session_id, key_group, &message, &included, 2, 1, 2)
        .expect("a newer attempt replaces the live one");
    assert_ne!(second.attempt_id, first.attempt_id);

    let stale = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: first.attempt_id,
        member_identifier: 1,
    })
    .expect_err("the replaced attempt must no longer be live");
    assert!(
        matches!(stale, EngineError::Validation(ref m) if m.contains("does not match")),
        "unexpected error: {stale:?}"
    );
}

#[test]
fn interactive_abort_destroys_nonces_and_is_idempotent() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-abort";
    let key_group = "interactive-test-key-group";
    let message = [0x91u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");

    let aborted = interactive_session_abort(InteractiveSessionAbortRequest {
        session_id: session_id.to_string(),
        attempt_id: Some(opened.attempt_id.clone()),
    })
    .expect("abort");
    assert!(aborted.aborted);

    let again = interactive_session_abort(InteractiveSessionAbortRequest {
        session_id: session_id.to_string(),
        attempt_id: Some(opened.attempt_id.clone()),
    })
    .expect("abort is idempotent");
    assert!(!again.aborted);

    let dead = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect_err("an aborted attempt must not serve round 1");
    assert!(
        matches!(dead, EngineError::SessionNotFound { .. }),
        "unexpected error: {dead:?}"
    );

    // Abort destroyed the nonces WITHOUT a consumption marker: the
    // attempt was never consumed, so reopening it is allowed and gets
    // FRESH nonces (the old ones are gone forever).
    let reopened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("an aborted (never consumed) attempt may reopen");
    assert_eq!(reopened.attempt_id, opened.attempt_id);
}

#[test]
fn interactive_abort_persist_failures_are_retryable_before_replace_and_fail_closed_after() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_abort_persist_durability");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let wallet_session = "interactive-abort-persist-wallet";
    let key_group = "interactive-abort-persist-key-group";
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);

    let open_round1 = |session_id: &str, message: [u8; 32]| {
        let opened = interactive_session_open(InteractiveSessionOpenRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: hex::encode(message),
            key_group: key_group.to_string(),
            threshold: 2,
            taproot_merkle_root_hex: None,
            signing_intent: None,
            attempt_context: interactive_test_attempt_context(
                session_id, key_group, &message, &included, 1,
            ),
        })
        .expect("per-message session opens");
        interactive_round1(InteractiveRound1Request {
            session_id: session_id.to_string(),
            attempt_id: opened.attempt_id.clone(),
            member_identifier: 1,
        })
        .expect("per-message session reaches Round1");
        opened
    };

    let retryable_session = "interactive-abort-pre-replace";
    let retryable = open_round1(retryable_session, [0xa2; 32]);
    let retryable_request = InteractiveSessionAbortRequest {
        session_id: retryable_session.to_string(),
        attempt_id: Some(retryable.attempt_id),
    };
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let pre_replace = interactive_session_abort(retryable_request.clone())
        .expect_err("a pre-replacement Abort fault must not consume live nonces");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        pre_replace,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    {
        let guard = state().expect("state").lock().expect("engine lock");
        let session = &guard.sessions[retryable_session];
        assert!(session
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&1)),);
        assert!(session.capacity_pins.retired_interactive_at_unix.is_none());
    }
    assert!(
        interactive_session_abort(retryable_request)
            .expect("Abort retry persists and succeeds")
            .aborted
    );

    let fail_closed_session = "interactive-abort-post-replace";
    let fail_closed = open_round1(fail_closed_session, [0xa3; 32]);
    let fail_closed_request = InteractiveSessionAbortRequest {
        session_id: fail_closed_session.to_string(),
        attempt_id: Some(fail_closed.attempt_id),
    };
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let post_replace = interactive_session_abort(fail_closed_request.clone())
        .expect_err("a post-replacement Abort fault reports uncertain directory durability");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        post_replace,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    {
        let guard = state().expect("state").lock().expect("engine lock");
        let session = &guard.sessions[fail_closed_session];
        assert!(session.interactive.interactive_signing.is_empty());
        assert!(session.capacity_pins.retired_interactive_at_unix.is_some());
    }
    assert!(interactive_state_persistence_pending());

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    interactive_session_abort(fail_closed_request.clone())
        .expect_err("an idempotent retry must attempt the pending durability repair");
    clear_persist_fault_injection_for_tests();
    assert!(interactive_state_persistence_pending());
    let repaired = interactive_session_abort(fail_closed_request)
        .expect("a healthy idempotent retry repairs Abort durability");
    assert!(!repaired.aborted);
    assert!(!interactive_state_persistence_pending());

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("engine lock");
        let session = &guard.sessions[fail_closed_session];
        assert_eq!(
            session.interactive.bound_key_group.as_deref(),
            Some(key_group)
        );
        assert!(session.capacity_pins.retired_interactive_at_unix.is_some());
        assert!(session.interactive.interactive_signing.is_empty());
    }

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn reset_for_tests_clears_interactive_clock_offset() {
    let _guard = lock_test_state();
    reset_for_tests();

    let baseline = interactive_now();
    advance_interactive_clock_for_tests(3_600);
    let advanced = interactive_now();
    assert!(advanced.saturating_duration_since(baseline) >= Duration::from_secs(3_600));

    reset_for_tests();
    assert!(
        interactive_now() < advanced,
        "engine reset must clear the test-only interactive clock offset"
    );
}

#[test]
fn interactive_session_ttl_expiry_has_abort_semantics() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-ttl";
    let key_group = "interactive-test-key-group";
    let message = [0xa1u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");

    // Age the session past the TTL directly; the next entry point's
    // lazy sweep must destroy the nonces with abort semantics.
    advance_interactive_clock_for_tests(interactive_session_ttl_seconds().saturating_add(1));

    let expired = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect_err("an expired attempt must not serve round 1");
    assert!(
        matches!(expired, EngineError::SessionNotFound { .. }),
        "unexpected error: {expired:?}"
    );

    // Expiry, like abort, leaves no consumption marker: the attempt
    // never released a share, so reopening is allowed.
    open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("an expired (never consumed) attempt may reopen");
}

#[test]
fn interactive_inactivity_ttl_refreshes_on_idempotent_open() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-ttl-idempotent-open";
    let key_group = "interactive-test-key-group";
    let message = [0xa2u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let ttl_seconds = interactive_session_ttl_seconds();
    let recent_margin_seconds = ttl_seconds / 4;
    assert!(
        recent_margin_seconds > 0,
        "the configured interactive TTL must provide a synthetic test margin"
    );
    let prior_activity = interactive_last_activity_at_for_test(session_id, 1);

    // Advance by half the TTL without sleeping. The exact retry is legitimate
    // activity and must advance the monotonic timestamp.
    advance_interactive_clock_for_tests(ttl_seconds / 2);

    let retry_started_at = interactive_now();
    let retry = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("an exact Open retry remains live");
    assert!(retry.idempotent);
    let refreshed_activity = {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        live_member(session, 1)
            .expect("member has live interactive entry")
            .last_activity_at
    };
    assert!(
        refreshed_activity > prior_activity,
        "an idempotent Open must advance the member's activity instant"
    );
    assert!(
        refreshed_activity >= retry_started_at,
        "the refreshed activity instant must belong to the retry"
    );

    // Model substantial but still sub-TTL inactivity from the observed retry
    // instant. This stays far from the expiry boundary and requires no sleep.
    advance_interactive_clock_for_tests(recent_margin_seconds);
    assert!(
        interactive_now().saturating_duration_since(refreshed_activity)
            < Duration::from_secs(ttl_seconds),
        "the synthetic activity must remain comfortably inside the TTL"
    );

    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id,
        member_identifier: 1,
    })
    .expect("recent idempotent Open activity keeps the attempt live");
}

#[test]
fn interactive_inactivity_ttl_refreshes_fresh_round1_per_member_before_round2() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-ttl-round1-round2";
    let key_group = "interactive-test-key-group";
    let message = [0xa3u8; 32];
    let included = [1u16, 2];
    let key_packages = interactive_test_key_packages();
    let ttl_seconds = interactive_session_ttl_seconds();
    let margin_seconds = ttl_seconds / 4;
    assert!(
        margin_seconds > 0,
        "the configured interactive TTL must provide a synthetic test margin"
    );

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("member 1 opens");
    open_interactive_for_test(session_id, key_group, &message, &included, 1, 2, 2)
        .expect("member 2 opens");
    let round1_member_2 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 2,
    })
    .expect("member 2 round 1");

    // Member 1 has not run Round1 yet. Advance it well within the TTL, then
    // prove rejected traffic cannot refresh its original Open activity.
    let prior_activity = interactive_last_activity_at_for_test(session_id, 1);
    advance_interactive_clock_for_tests(ttl_seconds / 2);
    let rejected_attempt_id = hash_hex(b"rejected-ttl-round1-attempt");
    assert_ne!(rejected_attempt_id, opened.attempt_id);
    let rejected = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: rejected_attempt_id,
        member_identifier: 1,
    })
    .expect_err("a wrong-attempt Round1 must be rejected");
    assert_eq!(
        interactive_last_activity_at_for_test(session_id, 1),
        prior_activity,
        "rejected traffic must not refresh the member's activity instant"
    );
    assert!(matches!(rejected, EngineError::Validation(_)));

    // A first, successful Round1 must advance the monotonic activity instant.
    let round1_started_at = interactive_now();
    let round1_member_1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 fresh round 1");
    let refreshed_activity = interactive_last_activity_at_for_test(session_id, 1);
    assert!(
        refreshed_activity > prior_activity,
        "fresh Round1 must advance the member's activity instant"
    );
    assert!(
        refreshed_activity >= round1_started_at,
        "the refreshed activity instant must belong to fresh Round1"
    );

    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1_member_1.commitments_hex,
            },
            NativeFrostCommitment {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: round1_member_2.commitments_hex,
            },
        ],
    );

    // Advance far enough that member 2, idle since offset zero, is beyond the
    // TTL while member 1's fresh Round1 remains comfortably live.
    advance_interactive_clock_for_tests(ttl_seconds / 2 + margin_seconds);
    let synthetic_now = interactive_now();
    let idle_activity = interactive_last_activity_at_for_test(session_id, 2);
    assert!(
        synthetic_now.saturating_duration_since(refreshed_activity)
            < Duration::from_secs(ttl_seconds)
    );
    assert!(
        synthetic_now.saturating_duration_since(idle_activity) > Duration::from_secs(ttl_seconds)
    );

    interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id,
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("recent Round1 activity keeps member 1 live through Round2");

    let guard = state().expect("state").lock().expect("lock");
    let session = guard.sessions.get(session_id).expect("session remains");
    assert!(
        !session
            .interactive
            .interactive_signing
            .values()
            .any(|members| members.contains_key(&2)),
        "the genuinely idle sibling expires on the same sweep"
    );
}

#[test]
fn interactive_live_session_capacity_fails_closed() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-test-key-group";
    let message = [0xb1u8; 32];
    let included = [1u16, 2];

    std::env::set_var(TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV, "1");

    let outcome = (|| -> Result<(), EngineError> {
        open_interactive_for_test("interactive-cap-a", key_group, &message, &included, 1, 1, 2)?;

        let at_capacity =
            open_interactive_for_test("interactive-cap-b", key_group, &message, &included, 1, 1, 2)
                .expect_err("the live-session cap must fail closed");
        assert!(
            matches!(at_capacity, EngineError::Internal(ref m)
                if m.contains("live interactive member count")),
            "unexpected error: {at_capacity:?}"
        );

        // An idempotent reopen of the live session does not trip the cap.
        let reopen = open_interactive_for_test(
            "interactive-cap-a",
            key_group,
            &message,
            &included,
            1,
            1,
            2,
        )?;
        assert!(reopen.idempotent);

        // Aborting frees the slot.
        interactive_session_abort(InteractiveSessionAbortRequest {
            session_id: "interactive-cap-a".to_string(),
            attempt_id: None,
        })?;
        open_interactive_for_test("interactive-cap-b", key_group, &message, &included, 1, 1, 2)?;
        Ok(())
    })();

    std::env::remove_var(TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV);
    outcome.expect("capacity lifecycle");
}

#[test]
fn interactive_open_signing_policy_firewall_rejects_without_policy_checked_build_tx() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    // The critical security assertion: with the firewall enabled, a
    // fresh interactive session with no prior policy-checked
    // build_taproot_tx must NOT be able to open and sign an arbitrary
    // message. It fails closed at the same gate the coarse path uses.
    let outcome = open_interactive_for_test(
        "interactive-firewall-no-build-tx",
        "interactive-firewall-key-group",
        &[0xc1u8; 32],
        &[1u16, 2],
        1,
        1,
        2,
    );

    std::env::remove_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV);
    clear_state_storage_policy_overrides();

    let err = outcome.expect_err("interactive open must fail closed under the firewall");
    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert_eq!(reason_code, "missing_policy_checked_build_tx");
}

#[test]
fn interactive_heartbeat_intent_opens_and_releases_round2_share_under_firewall() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV, "1");

    let wallet_session = "wallet-heartbeat-intent-success";
    let signing_session = "roast-heartbeat-intent-success";
    let key_group = "heartbeat-intent-success-key-group";
    let included = [1u16, 2];
    let heartbeat_message = heartbeat_message_for_test(1);
    let signing_message = heartbeat_signing_message_for_test(&heartbeat_message);
    let key_packages = ensure_interactive_dkg_session(wallet_session, key_group);

    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(signing_message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: Some(heartbeat_signing_intent_for_test(&heartbeat_message)),
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &signing_message,
            &included,
            1,
        ),
    })
    .expect("a valid heartbeat intent authorizes Open without a transaction artifact");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("heartbeat round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 heartbeat commitments");
    let signing_package_hex = interactive_package_for_test(
        &signing_message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );

    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id,
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("Round2 rechecks and accepts the stored heartbeat intent");
    assert!(!round2.signature_share_hex.is_empty());

    let guard = state().expect("state").lock().expect("engine lock");
    let session = guard
        .sessions
        .get(signing_session)
        .expect("heartbeat signing session");
    assert!(session.signing.tx_result.is_none());
    assert!(session.interactive.interactive_signing.is_empty());
    drop(guard);

    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_heartbeat_capacity_rejection_does_not_consume_wallet_token() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("heartbeat_capacity_preflight");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");
    std::env::set_var(TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV, "1");

    let wallet_session = "wallet-heartbeat-capacity-preflight";
    let key_group = "heartbeat-capacity-preflight-key-group";
    let signing_session = "roast-heartbeat-capacity-preflight";
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);
    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        guard.sessions.insert(
            "active-capacity-filler".to_string(),
            SessionState::default(),
        );
        assert_eq!(active_session_count(&guard.sessions), 2);
    }

    let heartbeat_message = heartbeat_message_for_test(100);
    let signing_message = heartbeat_signing_message_for_test(&heartbeat_message);
    let request = InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(signing_message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: Some(heartbeat_signing_intent_for_test(&heartbeat_message)),
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &signing_message,
            &included,
            1,
        ),
    };

    let rejected = interactive_session_open(request.clone())
        .expect_err("a full active tier must reject a fresh heartbeat session");
    assert!(matches!(
        rejected,
        EngineError::Internal(ref message) if message.contains("active session registry size")
    ));
    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        let limiter = &guard.sessions[wallet_session]
            .capacity_pins
            .heartbeat_rate_limiter;
        assert_eq!(limiter.last_refill_unix, 0);
        assert_eq!(limiter.token_microunits, 0);
        assert_eq!(limiter.configured_rate_limit_per_minute, 0);
        guard.sessions.remove("active-capacity-filler");
    }

    interactive_session_open(request)
        .expect("the capacity-rejected call must leave the wallet's token available");
    {
        let guard = state().expect("state").lock().expect("engine lock");
        let limiter = &guard.sessions[wallet_session]
            .capacity_pins
            .heartbeat_rate_limiter;
        assert_eq!(limiter.configured_rate_limit_per_minute, 1);
        assert_eq!(limiter.token_microunits, 0);
    }

    clear_state_storage_policy_overrides();
    cleanup_test_state_artifacts(&state_path);
}

#[test]
fn interactive_heartbeat_rate_limit_is_per_wallet_and_retry_safe() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("heartbeat_rate_limit");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    // Heartbeat authorization is independently rate-limited even when the
    // transaction policy firewall is disabled. Keep the BuildTaprootTx bucket at
    // one token too so the final assertion proves the two budgets are disjoint.
    std::env::set_var(TBTC_SIGNER_POLICY_HEARTBEAT_RATE_LIMIT_PER_MINUTE_ENV, "1");
    std::env::set_var(TBTC_SIGNER_POLICY_RATE_LIMIT_PER_MINUTE_ENV, "1");

    let wallet_a_session = "wallet-heartbeat-rate-limit-a";
    let wallet_a_key_group = "heartbeat-rate-limit-key-group-a";
    let wallet_b_session = "wallet-heartbeat-rate-limit-b";
    let wallet_b_key_group = "heartbeat-rate-limit-key-group-b";
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_a_session, wallet_a_key_group);
    ensure_interactive_dkg_session(wallet_b_session, wallet_b_key_group);

    let heartbeat_open_request = |session_id: &str, key_group: &str, nonce: u64| {
        let heartbeat_message = heartbeat_message_for_test(nonce);
        let signing_message = heartbeat_signing_message_for_test(&heartbeat_message);
        InteractiveSessionOpenRequest {
            session_id: session_id.to_string(),
            member_identifier: 1,
            message_hex: hex::encode(signing_message),
            key_group: key_group.to_string(),
            threshold: 2,
            taproot_merkle_root_hex: None,
            signing_intent: Some(heartbeat_signing_intent_for_test(&heartbeat_message)),
            attempt_context: interactive_test_attempt_context(
                session_id,
                key_group,
                &signing_message,
                &included,
                1,
            ),
        }
    };

    let first_wallet_a_request =
        heartbeat_open_request("roast-heartbeat-rate-limit-a-1", wallet_a_key_group, 10);
    let first_wallet_a = interactive_session_open(first_wallet_a_request.clone())
        .expect("the first wallet-A heartbeat Open has one token");
    assert!(!first_wallet_a.idempotent);

    let wallet_a_retry = interactive_session_open(first_wallet_a_request)
        .expect("an exact Open retry must not charge another heartbeat token");
    assert!(wallet_a_retry.idempotent);

    let wallet_a_limited = interactive_session_open(heartbeat_open_request(
        "roast-heartbeat-rate-limit-a-2",
        wallet_a_key_group,
        11,
    ))
    .expect_err("a fresh wallet-A heartbeat Open must exhaust its one-token budget");
    assert!(matches!(
        wallet_a_limited,
        EngineError::SigningPolicyRejected { ref reason_code, .. }
            if reason_code == "heartbeat_rate_limit_per_minute_exceeded"
    ));
    let metrics_after_limit = hardening_metrics();
    assert_eq!(metrics_after_limit.heartbeat_signing_policy_reject_total, 1);
    assert_eq!(metrics_after_limit.build_taproot_tx_policy_reject_total, 0);

    let wallet_b = interactive_session_open(heartbeat_open_request(
        "roast-heartbeat-rate-limit-b-1",
        wallet_b_key_group,
        12,
    ))
    .expect("wallet B must have an independent heartbeat budget");
    assert!(!wallet_b.idempotent);

    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr");
    configure_required_signing_policy_limits_for_tests();
    build_taproot_tx(build_policy_test_request(
        "session-heartbeat-rate-limit-build-tx-budget",
    ))
    .expect("heartbeat Opens must not consume the BuildTaprootTx token bucket");
    let metrics_after_build = hardening_metrics();
    assert_eq!(metrics_after_build.heartbeat_signing_policy_reject_total, 1);
    assert_eq!(metrics_after_build.build_taproot_tx_policy_reject_total, 0);

    clear_state_storage_policy_overrides();
    cleanup_test_state_artifacts(&state_path);
}

#[test]
fn interactive_heartbeat_intent_rejects_malformed_ambiguous_or_tweaked_requests() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr");
    configure_required_signing_policy_limits_for_tests();

    let wallet_session = "wallet-heartbeat-intent-negative";
    let key_group = "heartbeat-intent-negative-key-group";
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);
    let heartbeat_message = heartbeat_message_for_test(2);
    let signing_message = heartbeat_signing_message_for_test(&heartbeat_message);

    let mut wrong_prefix = heartbeat_message;
    wrong_prefix[0] = 0;
    let mismatched_message = heartbeat_message_for_test(3);
    let cases = vec![
        (
            "non-hex",
            InteractiveSigningIntent::Heartbeat {
                message_hex: "zz".repeat(16),
            },
            None,
            "invalid_heartbeat_signing_intent",
        ),
        (
            "wrong-prefix",
            heartbeat_signing_intent_for_test(&wrong_prefix),
            None,
            "invalid_heartbeat_signing_intent",
        ),
        (
            "short-message",
            heartbeat_signing_intent_for_test(&heartbeat_message[..15]),
            None,
            "invalid_heartbeat_signing_intent",
        ),
        (
            "digest-mismatch",
            heartbeat_signing_intent_for_test(&mismatched_message),
            None,
            "heartbeat_signing_message_mismatch",
        ),
        (
            "taproot-root",
            heartbeat_signing_intent_for_test(&heartbeat_message),
            Some("11".repeat(32)),
            "invalid_heartbeat_signing_intent",
        ),
    ];

    for (case, signing_intent, taproot_merkle_root_hex, expected_reason) in cases {
        let signing_session = format!("roast-heartbeat-intent-{case}");
        let error = interactive_session_open(InteractiveSessionOpenRequest {
            session_id: signing_session.clone(),
            member_identifier: 1,
            message_hex: hex::encode(signing_message),
            key_group: key_group.to_string(),
            threshold: 2,
            taproot_merkle_root_hex,
            signing_intent: Some(signing_intent),
            attempt_context: interactive_test_attempt_context(
                &signing_session,
                key_group,
                &signing_message,
                &included,
                1,
            ),
        })
        .expect_err("invalid heartbeat intent must fail closed at Open");
        assert!(
            matches!(error, EngineError::SigningPolicyRejected { ref reason_code, .. }
                if reason_code == expected_reason),
            "case [{case}] returned unexpected error: {error:?}"
        );
    }

    let ambiguous_session = "roast-heartbeat-intent-ambiguous";
    build_taproot_tx(build_policy_test_request(ambiguous_session))
        .expect("seed a transaction artifact on the ambiguous session");
    let ambiguous = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: ambiguous_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(signing_message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: Some(heartbeat_signing_intent_for_test(&heartbeat_message)),
        attempt_context: interactive_test_attempt_context(
            ambiguous_session,
            key_group,
            &signing_message,
            &included,
            1,
        ),
    })
    .expect_err("transaction and heartbeat authorizations must not coexist");
    assert!(matches!(
        ambiguous,
        EngineError::SigningPolicyRejected { ref reason_code, .. }
            if reason_code == "ambiguous_signing_policy_artifact"
    ));

    let metrics = hardening_metrics();
    assert_eq!(metrics.heartbeat_signing_policy_reject_total, 6);
    assert_eq!(metrics.build_taproot_tx_policy_reject_total, 0);

    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_round2_rechecks_stored_heartbeat_intent_before_share_release() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");

    let wallet_session = "wallet-heartbeat-intent-round2-recheck";
    let signing_session = "roast-heartbeat-intent-round2-recheck";
    let key_group = "heartbeat-intent-round2-recheck-key-group";
    let included = [1u16, 2];
    let heartbeat_message = heartbeat_message_for_test(4);
    let signing_message = heartbeat_signing_message_for_test(&heartbeat_message);
    let key_packages = ensure_interactive_dkg_session(wallet_session, key_group);

    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(signing_message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: Some(heartbeat_signing_intent_for_test(&heartbeat_message)),
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &signing_message,
            &included,
            1,
        ),
    })
    .expect("valid heartbeat Open");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("heartbeat round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 heartbeat commitments");
    let signing_package_hex = interactive_package_for_test(
        &signing_message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );

    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        guard
            .sessions
            .get_mut(signing_session)
            .expect("heartbeat signing session")
            .interactive
            .interactive_signing
            .get_mut(&opened.attempt_id)
            .expect("live heartbeat attempt scope")
            .get_mut(&1)
            .expect("live heartbeat attempt member")
            .signing_intent = Some(heartbeat_signing_intent_for_test(
            &heartbeat_message_for_test(5),
        ));
    }

    let error = interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    })
    .expect_err("Round2 must recheck the stored heartbeat intent");
    assert!(matches!(
        error,
        EngineError::SigningPolicyRejected { ref reason_code, .. }
            if reason_code == "heartbeat_signing_message_mismatch"
    ));
    let guard = state().expect("state").lock().expect("engine lock");
    let session = &guard.sessions[signing_session];
    assert!(
        session.interactive.interactive_signing[&opened.attempt_id][&1]
            .round1
            .is_some()
    );
    assert!(!interactive_attempt_consumed(
        &session.interactive.consumed_attempt_markers,
        &opened.attempt_id,
        1,
    ));
    drop(guard);

    let metrics = hardening_metrics();
    assert_eq!(metrics.heartbeat_signing_policy_reject_total, 1);
    assert_eq!(metrics.build_taproot_tx_policy_reject_total, 0);

    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_open_cross_session_respects_wallet_emergency_rekey() {
    let _guard = lock_test_state();
    reset_for_tests();

    let wallet_session = "wallet-rekeyed-before-cross-session-open";
    let signing_session = "roast-blocked-before-open";
    let key_group = "cross-session-open-rekey-key-group";
    let message = [0xc2u8; 32];
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);
    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        guard
            .sessions
            .get_mut(wallet_session)
            .expect("wallet session")
            .lifecycle
            .emergency_rekey_event = Some(EmergencyRekeyEvent {
            reason: "wallet compromised before Open".to_string(),
            triggered_at_unix: now_unix(),
        });
    }

    let error = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &message,
            &included,
            1,
        ),
    })
    .expect_err("a wallet emergency rekey must block a distinct signing session at Open");
    assert!(
        matches!(error, EngineError::LifecyclePolicyRejected { ref reason_code, .. }
            if reason_code == "emergency_rekey_required"),
        "unexpected error: {error:?}"
    );

    let guard = state().expect("state").lock().expect("engine lock");
    assert!(
        !guard.sessions.contains_key(signing_session),
        "Open must reject before allocating or burning a signing-session nonce"
    );
}

#[test]
fn interactive_open_uses_signing_session_bip341_artifact_and_current_policy() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr");
    configure_required_signing_policy_limits_for_tests();

    let wallet_session = "wallet-policy-artifact-owner";
    let signing_session = "roast-policy-artifact-signing";
    let key_group = "policy-artifact-key-group";
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);

    let tx_result = build_taproot_tx(build_policy_test_request(signing_session))
        .expect("BuildTaprootTx stores the artifact on the signing session");
    reload_state_from_storage_for_tests();
    let signing_message =
        hex::decode(&tx_result.taproot_key_spend_sighashes_hex[0]).expect("BIP-341 sighash hex");
    let open_request = InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(&signing_message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &signing_message,
            &included,
            1,
        ),
    };
    interactive_session_open(open_request.clone())
        .expect("cross-session Open uses the signing session's Build artifact");

    let unsigned_tx_hash = hash_hex(&hex::decode(&tx_result.tx_hex).expect("tx hex"));
    let old_binding = enforce_signing_message_binding_to_policy_checked_build_tx(
        signing_session,
        &unsigned_tx_hash,
        None,
        Some(&tx_result),
        None,
    )
    .expect_err("SHA256(unsigned_tx) must not authorize a BIP-341 signature");
    assert!(matches!(
        old_binding,
        EngineError::SigningPolicyRejected { ref reason_code, .. }
            if reason_code == "signing_message_not_bound_to_policy_checked_build_tx"
    ));

    // The artifact was accepted under p2tr, but every Open rechecks the active
    // non-rate policy before returning even for an otherwise idempotent retry.
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
    let tightened = interactive_session_open(open_request)
        .expect_err("a stricter active policy must reject the cached transaction");
    assert!(matches!(
        tightened,
        EngineError::SigningPolicyRejected { ref reason_code, .. }
            if reason_code == "script_class_not_allowlisted"
    ));

    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_open_does_not_use_wallet_session_transaction_artifact() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr");
    configure_required_signing_policy_limits_for_tests();

    let wallet_session = "wallet-with-wrong-scope-artifact";
    let signing_session = "roast-without-own-artifact";
    let key_group = "wrong-scope-artifact-key-group";
    let included = [1u16, 2];
    ensure_interactive_dkg_session(wallet_session, key_group);
    let wallet_tx = build_taproot_tx(build_policy_test_request(wallet_session))
        .expect("wallet-scoped Build artifact");
    let message =
        hex::decode(&wallet_tx.taproot_key_spend_sighashes_hex[0]).expect("BIP-341 sighash hex");

    let err = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(&message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &message,
            &included,
            1,
        ),
    })
    .expect_err("a wallet-session artifact must not authorize a fresh signing flow");
    assert!(matches!(
        err,
        EngineError::SigningPolicyRejected { ref reason_code, .. }
            if reason_code == "missing_policy_checked_build_tx"
    ));

    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_round2_writes_a_consumed_marker_readable_by_the_previous_schema1_binary() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_round2_rollback_marker");
    reset_for_tests();

    let session_id = "interactive-round2-rollback-marker";
    let key_group = "interactive-test-key-group";
    let message = [0xe2u8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);
    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("interactive attempt opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );
    interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("round 2 persists consumption before releasing the share");

    let previous_schema1_marker = format!("m1@{}", opened.attempt_id);
    assert_eq!(
        interactive_consumed_marker(&opened.attempt_id, 1),
        previous_schema1_marker,
        "schema-1 state must keep the marker representation understood by the prior binary"
    );

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let guard = state().expect("state").lock().expect("lock");
    let markers = &guard.sessions[session_id]
        .interactive
        .consumed_attempt_markers;
    let previous_binary_reports_consumed =
        markers.contains(&previous_schema1_marker) || markers.contains(&opened.attempt_id);
    assert!(
        previous_binary_reports_consumed,
        "the immediately previous schema-1 reader must fail closed after rollback"
    );
    drop(guard);

    let transitional_v2 = HashSet::from([format!("{previous_schema1_marker}@v2")]);
    assert!(
        interactive_attempt_consumed(&transitional_v2, &opened.attempt_id, 1),
        "current readers retain fail-closed support for transitional @v2 markers"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_consumed_marker_is_case_insensitive() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-attempt-id-casing";
    let key_group = "interactive-test-key-group";
    let message = [0xe3u8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    // Build the canonical (lowercase) attempt context, consume it, then
    // retry the SAME logical attempt with the attempt_id upper-cased.
    // validate_attempt_context accepts the hash fields case-
    // insensitively, so a raw-keyed marker would miss and re-sign;
    // the canonical keying must reject it as consumed.
    let canonical = interactive_test_attempt_context(session_id, key_group, &message, &included, 1);
    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: canonical.clone(),
    })
    .expect("canonical open");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );
    // Round2 with an UPPER-cased attempt_id must still consume the
    // canonical attempt (proves round entry points canonicalize).
    interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.to_ascii_uppercase(),
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("round 2 under an upper-cased attempt_id consumes the canonical attempt");

    // Reopen the SAME attempt with an upper-cased attempt_id: the
    // consumed marker must catch it.
    let mut recased_context = canonical;
    recased_context.attempt_id = recased_context.attempt_id.to_ascii_uppercase();
    let replay = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: recased_context,
    })
    .expect_err("a re-cased consumed attempt must fail closed");
    assert!(
        matches!(replay, EngineError::ConsumedNonceReplay { .. }),
        "unexpected error: {replay:?}"
    );
}

#[test]
fn interactive_abort_sweeps_expired_sessions() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-test-key-group";
    let message = [0xf4u8; 32];
    let included = [1u16, 2];

    // Open a live attempt on session A, then age it past the TTL.
    let opened = open_interactive_for_test(
        "interactive-abort-sweep-a",
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect("session A opens");
    interactive_round1(InteractiveRound1Request {
        session_id: "interactive-abort-sweep-a".to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    advance_interactive_clock_for_tests(interactive_session_ttl_seconds().saturating_add(1));

    // An abort for a DIFFERENT session is the only post-expiry traffic;
    // it must still sweep session A's expired nonces (the TTL guarantee
    // holds regardless of which entry point takes the lock).
    interactive_session_abort(InteractiveSessionAbortRequest {
        session_id: "interactive-abort-sweep-other".to_string(),
        attempt_id: None,
    })
    .expect("abort for an unrelated session");

    // The sweep clears session A's expired live attempt (and its
    // nonces) even though the only post-expiry traffic was an abort for
    // an unrelated session. The session itself is retained - it rides
    // DKG state that persists for future signing.
    let guard = state().expect("state").lock().expect("lock");
    let session = guard
        .sessions
        .get("interactive-abort-sweep-a")
        .expect("session A (DKG state) is retained");
    assert!(
        session.interactive.interactive_signing.is_empty(),
        "an abort elsewhere must still sweep an expired interactive attempt"
    );
}

#[test]
fn interactive_open_rejected_on_session_lifecycle_states() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-test-key-group";
    let message = [0x17u8; 32];
    let included = [1u16, 2];

    // A session under an emergency rekey must refuse interactive opens,
    // exactly as start_sign_round does.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        guard.sessions.insert(
            "interactive-lifecycle-rekey".to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState::default(),
                interactive: InteractiveSessionState::default(),
                lifecycle: LifecycleState {
                    emergency_rekey_event: Some(EmergencyRekeyEvent {
                        reason: "test rekey".to_string(),
                        triggered_at_unix: now_unix(),
                    }),
                    ..Default::default()
                },
                capacity_pins: OperationalState::default(),
                audit: AuditTrail::default(),
            },
        );
    }
    let rekey = open_interactive_for_test(
        "interactive-lifecycle-rekey",
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect_err("an emergency-rekey session must refuse interactive open");
    assert!(
        matches!(rekey, EngineError::LifecyclePolicyRejected { ref reason_code, .. }
            if reason_code == "emergency_rekey_required"),
        "unexpected error: {rekey:?}"
    );

    // A terminally finalized session must refuse interactive opens.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        guard.sessions.insert(
            "interactive-lifecycle-finalized".to_string(),
            SessionState {
                dkg: DkgSessionState::default(),
                signing: LegacySigningSessionState {
                    finalize_request_fingerprint: Some("already-finalized".to_string()),
                    ..Default::default()
                },
                interactive: InteractiveSessionState::default(),
                lifecycle: LifecycleState::default(),
                capacity_pins: OperationalState::default(),
                audit: AuditTrail::default(),
            },
        );
    }
    let finalized = open_interactive_for_test(
        "interactive-lifecycle-finalized",
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect_err("a finalized session must refuse interactive open");
    assert!(
        matches!(finalized, EngineError::SessionFinalized { .. }),
        "unexpected error: {finalized:?}"
    );
}

#[test]
fn interactive_open_rejected_for_quarantined_member_honors_dao_allowlist() {
    let _guard = lock_test_state();
    reset_for_tests();

    std::env::set_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV, "true");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV, "2");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV, "1");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV, "2");

    // Member 1 is auto-quarantined.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        guard.quarantined_operator_identifiers.insert(1);
    }

    let key_group = "interactive-test-key-group";
    let message = [0x18u8; 32];
    let included = [1u16, 2];

    let outcome = (|| -> Result<(), EngineError> {
        let quarantined = open_interactive_for_test(
            "interactive-quarantine",
            key_group,
            &message,
            &included,
            1,
            1,
            2,
        )
        .expect_err("a quarantined member must not open an interactive session");
        assert!(
            matches!(quarantined, EngineError::QuarantinePolicyRejected { ref reason_code, .. }
                if reason_code == "operator_auto_quarantined"),
            "unexpected error: {quarantined:?}"
        );

        // A DAO allowlist override restores the member's ability to sign.
        std::env::set_var(
            TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV,
            "1",
        );
        let allowlisted = open_interactive_for_test(
            "interactive-quarantine-allowlisted",
            key_group,
            &message,
            &included,
            1,
            1,
            2,
        )?;
        assert!(!allowlisted.idempotent);
        Ok(())
    })();

    std::env::remove_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV);

    outcome.expect("quarantine gate lifecycle");
}

#[test]
fn interactive_round2_rechecks_gates_at_share_release() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let key_group = "interactive-test-key-group";
    let message = [0x19u8; 32];
    let included = [1u16, 2];

    // Open + Round1 normally (gates pass at Open), build the package,
    // THEN record an emergency rekey before Round2. The share must not
    // leave the engine: Round2 re-evaluates the gates at release time.
    let session_id = "interactive-toctou-rekey";
    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );

    // Kill switch recorded AFTER Open/Round1.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get_mut(session_id).expect("session exists");
        session.lifecycle.emergency_rekey_event = Some(EmergencyRekeyEvent {
            reason: "post-open rekey".to_string(),
            triggered_at_unix: now_unix(),
        });
    }

    let blocked = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect_err("a post-open emergency rekey must block the Round2 share");
    assert!(
        matches!(blocked, EngineError::LifecyclePolicyRejected { ref reason_code, .. }
            if reason_code == "emergency_rekey_required"),
        "unexpected error: {blocked:?}"
    );

    // The block at release time must be fail-closed WITHOUT consuming
    // the nonces: no marker was written (verify-before-consume applies
    // to the gate recheck too), so clearing the kill switch lets the
    // same attempt complete. This proves the recheck rejects before
    // consumption rather than after.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get_mut(session_id).expect("session exists");
        assert!(
            !session
                .interactive
                .consumed_attempt_markers
                .contains(&interactive_consumed_marker(&opened.attempt_id, 1)),
            "a gate rejection must not consume the attempt"
        );
        session.lifecycle.emergency_rekey_event = None;
    }

    interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id,
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("the same attempt completes once the kill switch clears");
}

#[test]
fn interactive_round2_rechecks_signing_session_transaction_against_current_policy() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr");
    configure_required_signing_policy_limits_for_tests();

    let wallet_session = "wallet-round2-policy-owner";
    let signing_session = "roast-round2-policy-signing";
    let key_group = "round2-policy-key-group";
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(wallet_session, key_group);
    let tx_result = build_taproot_tx(build_policy_test_request(signing_session))
        .expect("signing-session Build artifact");
    let message =
        hex::decode(&tx_result.taproot_key_spend_sighashes_hex[0]).expect("BIP-341 sighash hex");

    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(&message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            signing_session,
            key_group,
            &message,
            &included,
            1,
        ),
    })
    .expect("Open accepts current policy");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );

    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2wpkh");
    let blocked = interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect_err("Round2 must reject a transaction disallowed by current policy");
    assert!(matches!(
        blocked,
        EngineError::SigningPolicyRejected { ref reason_code, .. }
            if reason_code == "script_class_not_allowlisted"
    ));
    {
        let guard = state().expect("state").lock().expect("lock");
        let signing = guard
            .sessions
            .get(signing_session)
            .expect("signing session");
        assert!(!interactive_attempt_consumed(
            &signing.interactive.consumed_attempt_markers,
            &opened.attempt_id,
            1,
        ));
    }

    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr");
    interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id,
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("same live nonces complete once policy allows the transaction");

    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_round2_rechecks_gates_at_share_release_across_sessions() {
    // The cross-session counterpart of the above: the emergency-rekey kill switch is
    // recorded on the WALLET (DKG) session, but signing runs under a DISTINCT
    // per-message session. Round2 must STILL block the share - the wallet-level gate
    // has to be resolved by key_group, not read from the (empty) per-signing session.
    // This is the exact fail-open the state-split risked for distributed-DKG wallets,
    // whose only signing path is interactive.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let wallet_session = "wallet-dkg-session-rekey";
    let signing_session = "roast-signing-session-rekey";
    let key_group = "cross-session-rekey-key-group";
    let message = [0x21u8; 32];
    let included = [1u16, 2];

    // DKG material lives ONLY under the wallet session.
    ensure_interactive_dkg_session(wallet_session, key_group);

    // Open + Round1 under the distinct signing session (gates clear at Open).
    let attempt_context =
        interactive_test_attempt_context(signing_session, key_group, &message, &included, 1);
    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
    .expect("opens under the signing session");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1 under the signing session");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );

    // Kill switch recorded on the WALLET session AFTER Open/Round1 - NOT on the
    // signing session the operator is driving.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        let session = guard
            .sessions
            .get_mut(wallet_session)
            .expect("wallet session exists");
        session.lifecycle.emergency_rekey_event = Some(EmergencyRekeyEvent {
            reason: "post-open rekey on the wallet session".to_string(),
            triggered_at_unix: now_unix(),
        });
    }

    // Round2 under the signing session MUST block - the wallet-level rekey gate is
    // resolved by key_group from the wallet session, not the empty signing session.
    let blocked = interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect_err("a wallet-session emergency rekey must block a cross-session Round2 share");
    assert!(
        matches!(blocked, EngineError::LifecyclePolicyRejected { ref reason_code, .. }
            if reason_code == "emergency_rekey_required"),
        "unexpected error: {blocked:?}"
    );

    // The rejection must be fail-closed WITHOUT consuming the nonce. Clear the wallet
    // event, then prove that the same reader also honors an event stored directly on
    // the per-signing session.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        assert!(
            !guard
                .sessions
                .get(signing_session)
                .expect("signing session exists")
                .interactive
                .consumed_attempt_markers
                .contains(&interactive_consumed_marker(&opened.attempt_id, 1)),
            "a cross-session gate rejection must not consume the attempt"
        );
        guard
            .sessions
            .get_mut(wallet_session)
            .expect("wallet session exists")
            .lifecycle
            .emergency_rekey_event = None;
        let signing = guard
            .sessions
            .get_mut(signing_session)
            .expect("signing session exists");
        signing.lifecycle.emergency_rekey_event = Some(EmergencyRekeyEvent {
            reason: "post-open rekey on the signing session".to_string(),
            triggered_at_unix: now_unix(),
        });
    }

    let signing_session_blocked = interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect_err("a signing-session emergency rekey must also block Round2");
    assert!(
        matches!(signing_session_blocked, EngineError::LifecyclePolicyRejected { ref reason_code, .. }
            if reason_code == "emergency_rekey_required"),
        "unexpected error: {signing_session_blocked:?}"
    );

    {
        let mut guard = state().expect("state").lock().expect("lock");
        let signing = guard
            .sessions
            .get_mut(signing_session)
            .expect("signing session exists");
        assert!(
            !signing
                .interactive
                .consumed_attempt_markers
                .contains(&interactive_consumed_marker(&opened.attempt_id, 1)),
            "a signing-session gate rejection must not consume the attempt"
        );
        signing.lifecycle.emergency_rekey_event = None;
    }

    interactive_round2(InteractiveRound2Request {
        session_id: signing_session.to_string(),
        attempt_id: opened.attempt_id,
        member_identifier: 1,
        signing_package_hex,
    })
    .expect("the cross-session attempt completes once both kill switches clear");
}

#[test]
fn interactive_open_rejects_signing_session_rekey_before_wallet_binding() {
    let _guard = lock_test_state();
    reset_for_tests();

    let wallet_session = "wallet-dkg-session-pre-open-rekey";
    let signing_session = "roast-signing-session-pre-open-rekey";
    let key_group = "pre-open-rekey-key-group";
    let included = [1u16, 2];

    ensure_interactive_dkg_session(wallet_session, key_group);
    let tx_result = build_taproot_tx(build_policy_test_request(signing_session))
        .expect("BuildTaprootTx creates the per-signing session");
    let message = hex::decode(&tx_result.taproot_key_spend_sighashes_hex[0])
        .expect("BIP-341 sighash decodes");

    {
        let guard = state().expect("state").lock().expect("lock");
        let signing = guard
            .sessions
            .get(signing_session)
            .expect("BuildTaprootTx signing session exists");
        assert!(
            signing.interactive.bound_key_group.is_none(),
            "BuildTaprootTx must precede the Open wallet binding"
        );
    }

    let rekey = trigger_emergency_rekey(TriggerEmergencyRekeyRequest {
        session_id: signing_session.to_string(),
        reason: "compromise detected before Open".to_string(),
    })
    .expect("emergency rekey triggers on the unbound signing session");
    assert_eq!(
        rekey.session_id, signing_session,
        "without a wallet binding, the event remains on the signing session"
    );

    let attempt_context =
        interactive_test_attempt_context(signing_session, key_group, &message, &included, 1);
    let blocked = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
    .expect_err("the signing-session emergency rekey must block Open");
    assert!(
        matches!(blocked, EngineError::LifecyclePolicyRejected { ref reason_code, .. }
            if reason_code == "emergency_rekey_required"),
        "unexpected error: {blocked:?}"
    );
}

#[test]
fn trigger_emergency_rekey_on_signing_session_records_on_wallet_session() {
    // Defense in depth (writer side): emergency rekey is a WALLET-level kill switch,
    // and interactive Round2 resolves it from the wallet session by key_group. If a
    // caller triggers it on a per-signing session (a distinct RoastSessionID bound to a
    // wallet key), the event must land on the WALLET session - where a reader looks -
    // not on the ephemeral signing session. This makes the writer and reader keying
    // impossible to diverge.
    let _guard = lock_test_state();
    reset_for_tests();

    let wallet_session = "wallet-dkg-session-rekey-writer";
    let signing_session = "roast-signing-session-rekey-writer";
    let key_group = "cross-session-rekey-writer-key-group";
    let message = [0x22u8; 32];
    let included = [1u16, 2];

    ensure_interactive_dkg_session(wallet_session, key_group);

    // Open under the distinct signing session so it is bound to the wallet key.
    let attempt_context =
        interactive_test_attempt_context(signing_session, key_group, &message, &included, 1);
    interactive_session_open(InteractiveSessionOpenRequest {
        session_id: signing_session.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
    .expect("opens under the signing session");

    // Trigger the kill switch on the SIGNING session id.
    let result = trigger_emergency_rekey(TriggerEmergencyRekeyRequest {
        session_id: signing_session.to_string(),
        reason: "compromise detected".to_string(),
    })
    .expect("emergency rekey triggers");

    // It must have been recorded on the resolved WALLET session, where Round2 reads it.
    assert_eq!(
        result.session_id, wallet_session,
        "the rekey must be recorded on the resolved wallet session"
    );
    let guard = state().expect("state").lock().expect("lock");
    assert!(
        guard
            .sessions
            .get(wallet_session)
            .expect("wallet session")
            .lifecycle
            .emergency_rekey_event
            .is_some(),
        "the wallet session must hold the kill switch"
    );
    assert!(
        guard
            .sessions
            .get(signing_session)
            .expect("signing session")
            .lifecycle
            .emergency_rekey_event
            .is_none(),
        "the ephemeral signing session must NOT hold the kill switch"
    );
}

#[test]
fn interactive_open_rejects_threshold_below_key_package_min_signers() {
    let _guard = lock_test_state();
    reset_for_tests();

    // The fixture key packages are min_signers = 2. A request threshold
    // of 3 must be rejected at Open: otherwise Round2 would accept a
    // 3-commitment package, persist the marker, and only then have
    // frost::round2::sign fail on the count - burning the nonce for a
    // validation error.
    let mismatch = open_interactive_for_test(
        "interactive-threshold-mismatch",
        "interactive-test-key-group",
        &[0x1au8; 32],
        &[1u16, 2, 3],
        1,
        1,
        3,
    )
    .expect_err("a threshold below the key package min_signers must be rejected");
    assert!(
        matches!(mismatch, EngineError::Validation(ref m)
            if m.contains("does not match the DKG threshold")),
        "unexpected error: {mismatch:?}"
    );

    // The matching threshold (2) opens.
    open_interactive_for_test(
        "interactive-threshold-match",
        "interactive-test-key-group",
        &[0x1au8; 32],
        &[1u16, 2],
        1,
        1,
        2,
    )
    .expect("the key-package-matching threshold opens");
}

#[test]
fn interactive_open_requires_an_existing_dkg_session() {
    let _guard = lock_test_state();
    reset_for_tests();

    // Key material is resolved from engine DKG state by key_group, never the
    // request, so an interactive open when NO wallet key exists for that
    // key_group fails closed with DkgNotReady - the interactive path never
    // signs with caller-supplied material. (It MAY create a per-signing session
    // bound to an EXISTING wallet key, but only then.)
    let attempt_context = interactive_test_attempt_context(
        "interactive-no-dkg",
        "interactive-test-key-group",
        &[0x1bu8; 32],
        &[1u16, 2],
        1,
    );
    let err = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: "interactive-no-dkg".to_string(),
        member_identifier: 1,
        message_hex: hex::encode([0x1bu8; 32]),
        key_group: "interactive-test-key-group".to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
    .expect_err("interactive open with no wallet key for the key_group must fail closed");
    assert!(
        matches!(err, EngineError::DkgNotReady { .. }),
        "unexpected error: {err:?}"
    );

    // A member not in the session's DKG group is rejected even once DKG
    // exists (the group has members 1..3, so member 4 is absent).
    ensure_interactive_dkg_session("interactive-dkg-present", "interactive-test-key-group");
    let absent_member = interactive_test_attempt_context(
        "interactive-dkg-present",
        "interactive-test-key-group",
        &[0x1bu8; 32],
        &[1u16, 2],
        1,
    );
    let absent = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: "interactive-dkg-present".to_string(),
        member_identifier: 4,
        message_hex: hex::encode([0x1bu8; 32]),
        key_group: "interactive-test-key-group".to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: absent_member,
    })
    .expect_err("a non-DKG-participant member must be rejected");
    assert!(
        matches!(absent, EngineError::Validation(ref m)
            if m.contains("not a DKG participant")
                || m.contains("included_participants")),
        "unexpected error: {absent:?}"
    );
}

#[test]
fn interactive_round2_rejects_quarantined_co_signer_in_package() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-round2-quarantined-cosigner";
    let key_group = "interactive-test-key-group";
    let message = [0x1cu8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    std::env::set_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV, "true");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV, "2");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV, "1");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV, "2");

    let outcome = (|| -> Result<(), EngineError> {
        // This member (1) opens and runs round 1 while no one is
        // quarantined; the co-signer (2) is quarantined afterward.
        let opened =
            open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)?;
        let round1 = interactive_round1(InteractiveRound1Request {
            session_id: session_id.to_string(),
            attempt_id: opened.attempt_id.clone(),
            member_identifier: 1,
        })?;
        let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
            key_package_identifier: key_packages[&2].identifier.clone(),
            key_package_hex: key_packages[&2].data_hex.clone(),
        })?;
        let signing_package_hex = interactive_package_for_test(
            &message,
            vec![
                NativeFrostCommitment {
                    identifier: key_packages[&1].identifier.clone(),
                    data_hex: round1.commitments_hex,
                },
                member2.commitment,
            ],
        );

        // Quarantine the co-signer (member 2) after round 1.
        {
            let mut guard = state().expect("state").lock().expect("lock");
            guard.quarantined_operator_identifiers.insert(2);
        }

        // Round2 must refuse: this node will not contribute a share to a
        // package whose subset includes a quarantined co-signer, even
        // though this node (member 1) is not itself quarantined.
        let blocked = interactive_round2(InteractiveRound2Request {
            session_id: session_id.to_string(),
            attempt_id: opened.attempt_id.clone(),
            member_identifier: 1,
            signing_package_hex,
        })
        .expect_err("a quarantined co-signer in the package must block the share");
        assert!(
            matches!(blocked, EngineError::QuarantinePolicyRejected { ref reason_code, .. }
                if reason_code == "operator_auto_quarantined"),
            "unexpected error: {blocked:?}"
        );

        // Fail-closed without consuming: clearing the quarantine lets the
        // same attempt complete (the rejection preceded consumption).
        {
            let mut guard = state().expect("state").lock().expect("lock");
            assert!(
                !guard
                    .sessions
                    .get(session_id)
                    .expect("session")
                    .interactive
                    .consumed_attempt_markers
                    .contains(&interactive_consumed_marker(&opened.attempt_id, 1)),
                "a quarantine rejection must not consume the attempt"
            );
            guard.quarantined_operator_identifiers.remove(&2);
        }
        Ok(())
    })();

    std::env::remove_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV);
    outcome.expect("round2 co-signer quarantine lifecycle");
}

#[test]
fn interactive_open_rejects_phantom_included_participant() {
    let _guard = lock_test_state();
    reset_for_tests();

    // The session's DKG group is members 1..3. An attempt context whose
    // included set names a phantom id (99) must be rejected even though
    // the local member (1) is a real participant - otherwise a caller
    // could bias the RFC-21 coordinator/attempt derivation with
    // non-participants.
    let session_id = "interactive-phantom-included";
    let key_group = "interactive-test-key-group";
    let message = [0x1du8; 32];
    ensure_interactive_dkg_session(session_id, key_group);

    let attempt_context =
        interactive_test_attempt_context(session_id, key_group, &message, &[1u16, 99], 1);
    let err = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
    .expect_err("a phantom included participant must be rejected");
    assert!(
        matches!(err, EngineError::Validation(ref m)
            if m.contains("not a DKG participant for this session")),
        "unexpected error: {err:?}"
    );
}

#[test]
fn interactive_aggregate_allows_an_elected_coordinator_outside_the_signing_subset() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-nonsigning-coordinator";
    let key_group = "interactive-test-key-group";
    let message = [0x49u8; 32];
    let included = [1u16, 2, 3];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);
    let attempt_context =
        interactive_test_attempt_context(session_id, key_group, &message, &included, 1);
    let coordinator = attempt_context.coordinator_identifier;
    let signing_subset = included
        .iter()
        .copied()
        .filter(|member| *member != coordinator)
        .collect::<Vec<_>>();
    assert_eq!(signing_subset.len(), 2);

    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: coordinator,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
    .expect("elected coordinator opens the attempt");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: coordinator,
    })
    .expect("elected coordinator joins commitment collection");

    let (signing_package_hex, signature_shares) =
        stateless_package_and_shares_for_test(&message, &signing_subset, &key_packages);
    let aggregate_request = InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex,
        signature_shares,
        taproot_merkle_root_hex: None,
    };
    let aggregate = interactive_aggregate(aggregate_request.clone())
        .expect("an elected coordinator need not be one of the first threshold responders");
    assert_eq!(aggregate.attempt_id, opened.attempt_id);

    let guard = state().expect("state").lock().expect("lock");
    let session = &guard.sessions[session_id];
    assert!(
        !interactive_attempt_consumed(
            &session.interactive.consumed_attempt_markers,
            &opened.attempt_id,
            coordinator,
        ),
        "the coordinator did not release a share"
    );
    assert!(
        session.interactive.interactive_signing.is_empty(),
        "successful aggregation retires the coordinator's unused nonce handle"
    );
    assert_eq!(
        session.interactive.aggregated_attempt_markers.len(),
        1,
        "the successful attempt consumes one completion slot"
    );
    drop(guard);

    // The coordinator-only fallback cannot bind an inner FROST package to an
    // attempt id because the package carries no such field. Its durable
    // package identity must therefore reject the same valid package/share set
    // under a fresh canonical coordinator attempt, including after restart.
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let next_attempt_number = (2..=32)
        .find(|attempt_number| {
            interactive_test_attempt_context(
                session_id,
                key_group,
                &message,
                &included,
                *attempt_number,
            )
            .coordinator_identifier
                == coordinator
        })
        .expect("coordinator recurs within bounded RFC-21 rotation");
    let next_context = interactive_test_attempt_context(
        session_id,
        key_group,
        &message,
        &included,
        next_attempt_number,
    );
    let reopened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: coordinator,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context: next_context,
    })
    .expect("fresh canonical coordinator attempt opens after restart");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: reopened.attempt_id.clone(),
        member_identifier: coordinator,
    })
    .expect("fresh coordinator attempt reaches live authorization");

    let mut replay = aggregate_request;
    replay.attempt_id = reopened.attempt_id;
    let error = interactive_aggregate(replay)
        .expect_err("a completed package cannot consume a second attempt marker");
    assert!(
        matches!(error, EngineError::Validation(ref message) if message.contains("already aggregated")),
        "unexpected replay error: {error:?}"
    );
    let guard = state().expect("state").lock().expect("lock");
    assert_eq!(
        guard.sessions[session_id]
            .interactive
            .aggregated_attempt_markers
            .len(),
        1,
        "cross-attempt package replay must not amplify persistent completion state"
    );
}

#[test]
fn interactive_aggregate_does_not_authorize_an_omitted_noncoordinator_observer() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-omitted-observer";
    let key_group = "interactive-test-key-group";
    let message = [0x48u8; 32];
    let included = [1u16, 2, 3];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);
    let attempt_context =
        interactive_test_attempt_context(session_id, key_group, &message, &included, 1);
    let observer = included
        .iter()
        .copied()
        .find(|member| *member != attempt_context.coordinator_identifier)
        .expect("included set has a non-coordinator");
    let signing_subset = included
        .iter()
        .copied()
        .filter(|member| *member != observer)
        .collect::<Vec<_>>();

    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: observer,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
    .expect("observer opens the attempt");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: observer,
    })
    .expect("observer produces a commitment before the first-t subset freezes");
    let (signing_package_hex, signature_shares) =
        stateless_package_and_shares_for_test(&message, &signing_subset, &key_packages);

    let error = interactive_aggregate(InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id,
        signing_package_hex,
        signature_shares,
        taproot_merkle_root_hex: None,
    })
    .expect_err("an omitted observer cannot claim coordinator authorization");
    assert!(
        matches!(error, EngineError::Validation(ref message) if message.contains("not authorized")),
        "unexpected error: {error:?}"
    );
}

#[test]
fn interactive_aggregate_produces_and_self_verifies_bip340() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-aggregate-e2e";
    let key_group = "interactive-test-key-group";
    let message = [0x4au8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    // Member 1 signs through the hardened session API; member 2 through
    // the stateless primitive. Both shares feed the coordinator's
    // InteractiveAggregate, which resolves the verifying shares from the
    // session's own DKG state.
    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment.clone(),
        ],
    );
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("round 2 share");
    let member2_share = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 share");

    let aggregate = interactive_aggregate(InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            member2_share.signature_share,
        ],
        taproot_merkle_root_hex: None,
    })
    .expect("interactive aggregate");
    assert_eq!(aggregate.attempt_id, opened.attempt_id);

    // The engine already self-verified; re-verify here against the DKG
    // group key to pin the round trip.
    let public_key_package = {
        let guard = state().expect("state").lock().expect("lock");
        guard
            .sessions
            .get(session_id)
            .expect("session")
            .dkg
            .public_key_package
            .clone()
            .expect("public key package")
    };
    let verifying_key_bytes = public_key_package.verifying_key().serialize().expect("vk");
    let signature_bytes = hex::decode(aggregate.signature_hex).expect("sig hex");
    let signature = SchnorrSignature::from_slice(&signature_bytes).expect("BIP340 signature");
    let public_key = XOnlyPublicKey::from_slice(&verifying_key_bytes[1..]).expect("x-only key");
    Secp256k1::verification_only()
        .verify_schnorr(&signature, &SecpMessage::from_digest(message), &public_key)
        .expect("interactive aggregate yields a valid BIP-340 signature");
}

#[test]
fn interactive_aggregate_rejects_repeat_aggregate_of_completed_attempt() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-aggregate-repeat";
    let key_group = "interactive-test-key-group";
    let message = [0x4eu8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment.clone(),
        ],
    );
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("round 2 share");
    let member2_share = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 share");

    let aggregate_request = InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            member2_share.signature_share,
        ],
        taproot_merkle_root_hex: None,
    };

    // First aggregate completes the attempt. The wire remains case-insensitive:
    // a canonical 64-hex id is normalized before lookup and response emission.
    let mut recased_request = aggregate_request.clone();
    recased_request.attempt_id = recased_request.attempt_id.to_ascii_uppercase();
    let aggregate = interactive_aggregate(recased_request).expect("first interactive aggregate");
    assert_eq!(aggregate.attempt_id, opened.attempt_id);

    // A valid package/share set cannot be replayed under a different, merely
    // well-formed id.
    let mut unbound_request = aggregate_request.clone();
    unbound_request.attempt_id = "aa".repeat(32);
    assert_ne!(unbound_request.attempt_id, opened.attempt_id);
    let err = interactive_aggregate(unbound_request)
        .expect_err("an unbound aggregate attempt id must be rejected");
    assert!(
        matches!(err, EngineError::Validation(ref message) if message.contains("not authorized")),
        "unexpected error: {err:?}"
    );

    // Merely opening the next canonical attempt (and even producing fresh
    // Round1 commitments) must not authorize the prior attempt's package. The
    // old ID-only binding allowed this loop to add one completion marker per
    // Open until the 128-entry cap blocked legitimate work.
    let reopened = open_interactive_for_test(session_id, key_group, &message, &included, 2, 1, 2)
        .expect("next canonical attempt opens");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: reopened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("next canonical attempt produces fresh commitments");
    let mut rebound_replay = aggregate_request.clone();
    rebound_replay.attempt_id = reopened.attempt_id;
    let err = interactive_aggregate(rebound_replay)
        .expect_err("a fresh Open must not authorize an old signing package");
    assert!(
        matches!(err, EngineError::Validation(ref message) if message.contains("not authorized")),
        "unexpected error: {err:?}"
    );
    {
        let guard = state().expect("state").lock().expect("lock");
        assert_eq!(
            guard.sessions[session_id]
                .interactive
                .aggregated_attempt_markers
                .len(),
            1,
            "a replay under a freshly opened canonical id must not consume marker capacity"
        );
    }

    // Re-aggregating a completed attempt is rejected by the durable completion
    // marker rather than recomputed (re-aggregation is not a recovery path; a
    // lost signature is recovered with a fresh attempt). Phase 7.2b design
    // section 6.
    let err = interactive_aggregate(aggregate_request)
        .expect_err("re-aggregating a completed attempt must be rejected");
    assert!(
        matches!(
            err,
            EngineError::InteractiveAttemptAlreadyAggregated { ref attempt_id, .. }
                if *attempt_id == opened.attempt_id
        ),
        "unexpected error: {err:?}"
    );
    assert_eq!(err.code(), "interactive_attempt_already_aggregated");
}

// The interactive_aggregate result must be deterministic over the PUBLIC
// inputs (signing_package, signature_shares, verifying shares from the
// session's DKG public key package): a fresh stateless re-aggregation from
// the same wire data must yield the byte-identical signature. This pins the
// invariant any future performance path that elides re-aggregation when the
// public data is unchanged (cf. F-08 review note) relies on - such a path
// would be unsound if the engine path applied a tweak the stateless primitive
// does not, or vice versa.
#[test]
fn interactive_aggregate_returns_same_signature_for_same_completed_session() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-aggregate-determinism";
    let key_group = "interactive-test-key-group";
    let message = [0x4fu8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    // End-to-end signing flow: member 1 through the session API, member 2
    // through the stateless primitive. Mirrors
    // interactive_aggregate_produces_and_self_verifies_bip340.
    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment.clone(),
        ],
    );
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("round 2 share");
    let member2_share = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 share");

    // Engine path: resolves verifying shares from the session's DKG state
    // and emits a single canonical signature for the attempt.
    let engine_aggregate = interactive_aggregate(InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex: signing_package_hex.clone(),
        signature_shares: vec![
            NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex.clone(),
            },
            member2_share.signature_share.clone(),
        ],
        taproot_merkle_root_hex: None,
    })
    .expect("interactive aggregate");

    // Fresh stateless re-aggregation of the SAME public inputs. If the
    // engine result diverges from this, any caller that caches "aggregate
    // result for these public inputs" would hand back a stale signature.
    // Engine state holds the frost public key package (not the native wire
    // form the stateless helper consumes). Convert before re-aggregating so
    // the byte-for-byte comparison proves the engine path applies no tweak
    // the stateless primitive does not.
    let frost_pkg = {
        let guard = state().expect("state").lock().expect("lock");
        guard
            .sessions
            .get(session_id)
            .expect("session")
            .dkg
            .public_key_package
            .clone()
            .expect("public key package")
    };
    let native_public_key_package =
        native_public_key_package_from_frost(&frost_pkg).expect("convert to native");
    let stateless_aggregate = aggregate(AggregateRequest {
        signing_package_hex,
        signature_shares: vec![
            NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            member2_share.signature_share,
        ],
        public_key_package: native_public_key_package,
    })
    .expect("stateless re-aggregate");

    assert_eq!(
        engine_aggregate.signature_hex, stateless_aggregate.signature_hex,
        "engine aggregate must be deterministic over public inputs (signing_package + signature_shares + public_key_package)"
    );
}

#[test]
fn interactive_aggregate_rejects_noncanonical_attempt_ids() {
    let _guard = lock_test_state();
    reset_for_tests();

    // Completion markers persist attempt_id verbatim as part of their key. The
    // canonical derivation is a SHA-256 digest, so reject empty, oversized, and
    // non-hex ids before decoding the other aggregate inputs or touching state.
    for attempt_id in [
        String::new(),
        "a".repeat(63),
        "a".repeat(65),
        format!("0x{}", "aa".repeat(31)),
        "zz".repeat(32),
    ] {
        let err = interactive_aggregate(InteractiveAggregateRequest {
            session_id: "interactive-aggregate-invalid-attempt".to_string(),
            attempt_id,
            signing_package_hex: String::new(),
            signature_shares: vec![],
            taproot_merkle_root_hex: None,
        })
        .expect_err("a noncanonical attempt_id must be rejected");
        assert!(
            matches!(err, EngineError::Validation(ref message)
                if message.contains("exactly 64 hexadecimal characters")),
            "unexpected error: {err:?}"
        );
    }
}

#[test]
fn interactive_aggregate_completion_marker_survives_process_restart() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_aggregate_marker_restart");
    reset_for_tests();

    let session_id = "interactive-aggregate-marker-restart";
    let key_group = "interactive-test-key-group";
    let message = [0x4fu8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment.clone(),
        ],
    );
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("round 2 share");
    let member2_share = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 share");

    let aggregate_request = InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            member2_share.signature_share,
        ],
        taproot_merkle_root_hex: None,
    };
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let faulted = interactive_aggregate(aggregate_request.clone())
        .expect_err("a pre-replacement persistence fault must roll Aggregate state back");
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(faulted, EngineError::Internal(ref message) if message.contains("injected persist fault")),
        "unexpected fault: {faulted:?}"
    );
    interactive_aggregate(aggregate_request.clone())
        .expect("the exact authorization and package remain retryable after rollback");

    // The completion marker is the only durable interactive artifact (live
    // nonce state is gone after restart by construction). It must survive a
    // reload so a replayed aggregate is still rejected - this also exercises
    // the marker's persistence round-trip (serialize + reload validation).
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    let err = interactive_aggregate(aggregate_request)
        .expect_err("a completed attempt must stay completed across restart");
    assert!(
        matches!(err, EngineError::InteractiveAttemptAlreadyAggregated { .. }),
        "unexpected error: {err:?}"
    );
    assert_eq!(err.code(), "interactive_attempt_already_aggregated");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_aggregate_post_rename_persist_failure_finalizes_attempt_and_retry_flushes() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_aggregate_post_rename");
    reset_for_tests();

    let session_id = "interactive-aggregate-post-rename";
    let key_group = "interactive-test-key-group";
    let message = [0x50u8; 32];
    let included = [1u16, 2, 3];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("member 1 opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1");

    let sibling = open_interactive_for_test(session_id, key_group, &message, &included, 1, 3, 2)
        .expect("unsigned sibling opens");
    assert_eq!(sibling.attempt_id, opened.attempt_id);
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: sibling.attempt_id,
        member_identifier: 3,
    })
    .expect("unsigned sibling creates live nonces");

    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment.clone(),
        ],
    );
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("member 1 round 2 share");
    let member2_share = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 share");

    let aggregate_request = InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            member2_share.signature_share,
        ],
        taproot_merkle_root_hex: None,
    };
    let aggregated_marker =
        interactive_aggregated_marker(&opened.attempt_id, &hash_hex(&message), None);

    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let faulted = interactive_aggregate(aggregate_request.clone())
        .expect_err("post-rename persist fault must report aggregate failure");
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(faulted, EngineError::Internal(ref text) if text.contains("injected persist fault")),
        "unexpected error: {faulted:?}"
    );

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(session
            .interactive
            .aggregated_attempt_markers
            .contains(&aggregated_marker));
        assert!(
            !session
                .interactive
                .interactive_signing
                .contains_key(&opened.attempt_id),
            "a retained completion marker must destroy an unsigned sibling's live nonces"
        );
    }
    assert!(interactive_aggregate_persistence_pending(
        session_id,
        &aggregated_marker
    ));

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let failed_flush = interactive_aggregate(aggregate_request.clone())
        .expect_err("a failed pending completion flush must not reach the completion gate");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        failed_flush,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(interactive_aggregate_persistence_pending(
        session_id,
        &aggregated_marker
    ));

    // Crash before a successful in-process repair. Reload must retain the
    // completion marker and cannot resurrect the unsigned sibling's nonces.
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        assert!(guard.sessions[session_id]
            .interactive
            .aggregated_attempt_markers
            .contains(&aggregated_marker));
        assert!(guard.sessions[session_id]
            .interactive
            .interactive_signing
            .is_empty());
    }

    let retry = interactive_aggregate(aggregate_request)
        .expect_err("restart retry rejects the durable completed attempt");
    assert!(
        matches!(
            retry,
            EngineError::InteractiveAttemptAlreadyAggregated { .. }
        ),
        "unexpected retry error: {retry:?}"
    );
    assert!(!interactive_aggregate_persistence_pending(
        session_id,
        &aggregated_marker
    ));

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_aggregate_post_rename_repair_retires_session_and_releases_capacity() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_aggregate_post_rename");
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

    let wallet_session_id = "interactive-aggregate-post-rename-wallet";
    let session_id = "interactive-aggregate-post-rename";
    let next_session_id = "interactive-aggregate-post-rename-next";
    let key_group = "interactive-test-key-group";
    let message = [0x50u8; 32];
    let included = [1u16, 2, 3];
    let key_packages = ensure_interactive_dkg_session(wallet_session_id, key_group);
    build_taproot_tx(build_policy_test_request(session_id))
        .expect("Build persists the cross-session policy shell");

    let open_member = |member_identifier| {
        interactive_session_open(InteractiveSessionOpenRequest {
            session_id: session_id.to_string(),
            member_identifier,
            message_hex: hex::encode(message),
            key_group: key_group.to_string(),
            threshold: 2,
            taproot_merkle_root_hex: None,
            signing_intent: None,
            attempt_context: interactive_test_attempt_context(
                session_id, key_group, &message, &included, 1,
            ),
        })
    };
    let opened = open_member(1).expect("member 1 opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1");

    let sibling = open_member(3).expect("unsigned sibling opens");
    assert_eq!(sibling.attempt_id, opened.attempt_id);
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: sibling.attempt_id,
        member_identifier: 3,
    })
    .expect("unsigned sibling creates live nonces");

    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment.clone(),
        ],
    );
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("member 1 round 2 share");
    let member2_share = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 share");

    let aggregate_request = InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            member2_share.signature_share,
        ],
        taproot_merkle_root_hex: None,
    };
    let aggregated_marker =
        interactive_aggregated_marker(&opened.attempt_id, &hash_hex(&message), None);

    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let faulted = interactive_aggregate(aggregate_request.clone())
        .expect_err("post-rename persist fault must report aggregate failure");
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(faulted, EngineError::Internal(ref text) if text.contains("injected persist fault")),
        "unexpected error: {faulted:?}"
    );

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(session
            .interactive
            .aggregated_attempt_markers
            .contains(&aggregated_marker));
        assert!(
            !session
                .interactive
                .interactive_signing
                .contains_key(&opened.attempt_id),
            "a retained completion marker must destroy an unsigned sibling's live nonces"
        );
    }
    assert!(interactive_aggregate_persistence_pending(
        session_id,
        &aggregated_marker
    ));

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let failed_flush = interactive_aggregate(aggregate_request.clone())
        .expect_err("a failed pending completion flush must not reach the completion gate");
    clear_persist_fault_injection_for_tests();
    assert!(matches!(
        failed_flush,
        EngineError::Internal(ref message) if message.contains("injected persist fault")
    ));
    assert!(interactive_aggregate_persistence_pending(
        session_id,
        &aggregated_marker
    ));

    // A different successful full-state writer is allowed to cover and clear
    // the Aggregate repair. Retirement must already be staged so this unrelated
    // snapshot cannot clear pending while leaving an active, nonce-free shell.
    build_taproot_tx(build_policy_test_request(wallet_session_id))
        .expect("an unrelated wallet Build persists the full engine snapshot");
    assert!(!interactive_aggregate_persistence_pending(
        session_id,
        &aggregated_marker
    ));

    // With pending already cleared by the unrelated writer, the exact retry
    // still rejects the completed attempt and must not duplicate its marker.
    let retry = interactive_aggregate(aggregate_request)
        .expect_err("in-process retry rejects the completed attempt");
    assert!(
        matches!(
            retry,
            EngineError::InteractiveAttemptAlreadyAggregated { .. }
        ),
        "unexpected retry error: {retry:?}"
    );
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = &guard.sessions[session_id];
        assert!(session
            .interactive
            .aggregated_attempt_markers
            .contains(&aggregated_marker));
        assert_eq!(session.interactive.aggregated_attempt_markers.len(), 1);
        assert!(session.interactive.interactive_signing.is_empty());
        assert!(
            session.capacity_pins.retired_interactive_at_unix.is_some(),
            "repair must retire the completed per-message session immediately"
        );
        assert_eq!(active_session_count(&guard.sessions), 1);
    }
    build_taproot_tx(build_policy_test_request(next_session_id))
        .expect("the repaired Aggregate session yields its shared registry slot");
    {
        let guard = state().expect("state").lock().expect("lock");
        assert!(guard.sessions.contains_key(wallet_session_id));
        assert!(guard.sessions.contains_key(next_session_id));
        assert!(!guard.sessions.contains_key(session_id));
        assert_eq!(guard.sessions.len(), 2);
    }

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_aggregate_rejects_invalid_share_fail_closed() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-aggregate-blame";
    let key_group = "interactive-test-key-group";
    let message = [0x4bu8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("round 2 share");

    // Member 2 contributes a structurally valid but WRONG share (a fresh
    // share over a different signing package). Aggregation must fail with
    // attributable blame naming member 2, not an opaque error.
    let bogus_member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("bogus member 2 nonces");
    let other_message = [0x4cu8; 32];
    let other_package = interactive_package_for_test(
        &other_message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: bogus_member2.commitment.data_hex.clone(),
            },
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: bogus_member2.commitment.data_hex,
            },
        ],
    );
    let bogus_share = sign_share(SignShareRequest {
        signing_package_hex: other_package,
        nonces_hex: bogus_member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("bogus member 2 share");

    let err = interactive_aggregate(InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            bogus_share.signature_share,
        ],
        taproot_merkle_root_hex: None,
    })
    .expect_err("an invalid share must fail aggregation closed");
    // 7.2b-3: the aggregate now fails closed WITH attributable CANDIDATE blame.
    // Member 2 submitted a structurally valid share over a different package, so
    // its share fails verification against the group's verifying material and is
    // named a candidate culprit (its u16 Go member id); member 1's honest share
    // is not. The engine surfaces candidates only - envelope-bound adjudication
    // is the Go host's job (frozen Phase 7.2b spec, section 6).
    let candidate_culprits = match err {
        EngineError::AggregateShareVerificationFailed {
            candidate_culprits, ..
        } => candidate_culprits,
        other => panic!("expected AggregateShareVerificationFailed, got {other:?}"),
    };
    assert_eq!(
        candidate_culprits,
        vec![2],
        "only the cheating member 2 must be named: {candidate_culprits:?}"
    );
    let guard = state().expect("state").lock().expect("lock");
    assert_eq!(
        Arc::strong_count(
            &guard.sessions[session_id]
                .capacity_pins
                .aggregate_eviction_pin
        ),
        1,
        "an Aggregate crypto error must release its transient eviction pin"
    );
}

#[test]
fn frost_identifier_to_u16_inverts_participant_mapping() {
    // The culprit list reports u16 Go member identifiers, so the inverse of
    // participant_identifier_to_frost_identifier must round-trip - including
    // across the low/high byte boundary (255 -> 256).
    for id in [1u16, 2, 3, 255, 256, 65535] {
        let identifier = participant_identifier_to_frost_identifier(id).expect("identifier");
        assert_eq!(frost_identifier_to_u16(identifier), Some(id), "id {id}");
    }
}

#[test]
fn interactive_aggregate_names_all_invalid_share_culprits() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-aggregate-multi-blame";
    let key_group = "interactive-test-key-group";
    let message = [0x5au8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    // Both members of the threshold-2 signing subset cheat: each signs a
    // DIFFERENT package, so both shares fail verification against the
    // authoritative package. Aggregation must name BOTH (AllCheaters), not just
    // the first cheater. (The signing package carries exactly `threshold`
    // commitments, so a multi-culprit case needs every subset member to cheat.)
    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");

    let real1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 live authorization commitment");
    let real2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: real1.commitments_hex,
            },
            real2.commitment.clone(),
        ],
    );

    // Each member signs a different (2-party) package over another message, so
    // both shares fail verification against the authoritative package.
    let other_message = [0x5bu8; 32];
    let bogus1 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
    })
    .expect("bogus member 1 nonces");
    let bogus1_package = interactive_package_for_test(
        &other_message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: bogus1.commitment.data_hex.clone(),
            },
            NativeFrostCommitment {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: bogus1.commitment.data_hex.clone(),
            },
        ],
    );
    let bogus1_share = sign_share(SignShareRequest {
        signing_package_hex: bogus1_package,
        nonces_hex: bogus1.nonces_hex,
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
    })
    .expect("bogus member 1 share");
    let bogus2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("bogus member 2 nonces");
    let bogus2_package = interactive_package_for_test(
        &other_message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: bogus2.commitment.data_hex.clone(),
            },
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: bogus2.commitment.data_hex.clone(),
            },
        ],
    );
    let bogus2_share = sign_share(SignShareRequest {
        signing_package_hex: bogus2_package,
        nonces_hex: bogus2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("bogus member 2 share");

    let err = interactive_aggregate(InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex,
        signature_shares: vec![bogus1_share.signature_share, bogus2_share.signature_share],
        taproot_merkle_root_hex: None,
    })
    .expect_err("two invalid shares must fail aggregation closed");

    let mut candidate_culprits = match err {
        EngineError::AggregateShareVerificationFailed {
            candidate_culprits, ..
        } => candidate_culprits,
        other => panic!("expected AggregateShareVerificationFailed, got {other:?}"),
    };
    candidate_culprits.sort_unstable();
    // AllCheaters, not FirstCheater: BOTH cheating members are named.
    assert_eq!(candidate_culprits, vec![1, 2], "{candidate_culprits:?}");
}

#[test]
fn interactive_aggregate_sweeps_expired_sessions() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-test-key-group";
    let message = [0x4du8; 32];
    let included = [1u16, 2];

    // An interactive attempt is opened + round-1'd on session A, then
    // aged past the TTL.
    let key_packages = ensure_interactive_dkg_session("interactive-aggregate-sweep-a", key_group);
    let opened = open_interactive_for_test(
        "interactive-aggregate-sweep-a",
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect("session A opens");
    interactive_round1(InteractiveRound1Request {
        session_id: "interactive-aggregate-sweep-a".to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    advance_interactive_clock_for_tests(interactive_session_ttl_seconds().saturating_add(1));

    // A parseable threshold-sized package + share so the aggregate call
    // reaches the lock and the sweep (it then fails on the missing
    // target session). The package needs `threshold` (2) commitments for
    // sign_share to produce a share.
    let member1 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
    })
    .expect("member 1 nonces");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let parseable_package = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: member1.commitment.data_hex.clone(),
            },
            member2.commitment,
        ],
    );
    let parseable_share = sign_share(SignShareRequest {
        signing_package_hex: parseable_package.clone(),
        nonces_hex: member1.nonces_hex,
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
    })
    .expect("member 1 share");

    // Aggregate against a session that does not exist: the inputs parse,
    // so the call reaches the lock and the sweep before failing with
    // SessionNotFound.
    let err = interactive_aggregate(InteractiveAggregateRequest {
        session_id: "interactive-aggregate-sweep-missing".to_string(),
        attempt_id: "00".repeat(32),
        signing_package_hex: parseable_package,
        signature_shares: vec![parseable_share.signature_share],
        taproot_merkle_root_hex: None,
    })
    .expect_err("aggregate against a missing session fails closed");
    assert!(
        matches!(err, EngineError::SessionNotFound { .. }),
        "unexpected error: {err:?}"
    );

    // The aggregate call's sweep must have cleared session A's expired
    // nonce handle even though the call targeted a different session.
    let guard = state().expect("state").lock().expect("lock");
    let session_a = guard
        .sessions
        .get("interactive-aggregate-sweep-a")
        .expect("session A (DKG state) retained");
    assert!(
        session_a.interactive.interactive_signing.is_empty(),
        "an aggregate call must sweep expired interactive state in other sessions"
    );
}

#[test]
fn lock_test_state_recovers_from_a_poisoned_mutex() {
    // A test that panics while holding the test lock must not cascade
    // into every subsequent test. Poison the lock from a child thread,
    // then confirm lock_test_state still hands out the guard.
    //
    // The poisoning thread prints an "intentional poison" panic message
    // (plus a backtrace note) to stderr - this is expected test output,
    // not a failure. It is deliberately not suppressed with a panic
    // hook: the hook is process-global, so silencing it here could
    // swallow the message of a genuinely-failing test running in
    // parallel.
    let poisoner = std::thread::spawn(|| {
        let _guard = lock_test_state();
        panic!("intentional poison to exercise lock recovery");
    });
    assert!(
        poisoner.join().is_err(),
        "poisoner thread was expected to panic"
    );

    // Recovers the guard despite the poison (would panic before the fix).
    let _guard = lock_test_state();
}

#[test]
fn establish_clean_signer_test_env_clears_leaked_toggles() {
    let _guard = lock_test_state();

    // Simulate a prior test that set toggle vars and "panicked" before
    // its own cleanup. The next lock acquisition's baseline reset must
    // remove them.
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV, "true");
    std::env::set_var(TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV, "1");

    establish_clean_signer_test_env();

    assert!(std::env::var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV).is_err());
    assert!(std::env::var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV).is_err());
    assert!(std::env::var(TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV).is_err());

    // The baseline the engine needs is re-established.
    assert_eq!(
        std::env::var(TBTC_SIGNER_PROFILE_ENV).as_deref(),
        Ok(TBTC_SIGNER_PROFILE_DEVELOPMENT)
    );
    assert_eq!(
        std::env::var(TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV).as_deref(),
        Ok(TEST_STATE_ENCRYPTION_KEY_HEX)
    );
}

// Persisted structs hold serialized signing-share material in
// `SecretString` (`Zeroizing<String>`) fields. `Debug` MUST NOT
// render that material: any future log/panic that `{:?}`-formats one
// of these structs would otherwise spill key shares or the signing
// message. Guards both the top-level field and the nested key-package
// vec.
#[test]
fn persisted_secret_structs_redact_debug_output() {
    let secret_key_package = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef";
    let secret_message = "cafebabecafebabecafebabecafebabe";

    let key_package = PersistedKeyPackage {
        identifier: 1,
        key_package_hex: Zeroizing::new(secret_key_package.to_string()),
    };
    let rendered = format!("{key_package:?}");
    assert!(
        !rendered.contains(secret_key_package),
        "PersistedKeyPackage Debug leaked key share material: {rendered}"
    );

    let mut session = persisted_session_state_fixture();
    session.sign_message_hex = Some(Zeroizing::new(secret_message.to_string()));
    session.dkg_key_packages = Some(vec![key_package]);
    let rendered = format!("{session:?}");
    assert!(
        !rendered.contains(secret_message),
        "PersistedSessionState Debug leaked sign message material: {rendered}"
    );
    assert!(
        !rendered.contains(secret_key_package),
        "PersistedSessionState Debug leaked nested key share material: {rendered}"
    );
}

// The open-request fingerprint serializes message_hex verbatim, so a
// retry of an identical open that only differs in message_hex casing
// must still be recognized as idempotent (the engine accepts hex
// case-insensitively elsewhere). Without canonicalization it would be
// rejected as a SessionConflict.
#[test]
fn interactive_session_open_is_idempotent_across_message_hex_casing() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-msg-hex-casing";
    let key_group = "interactive-msg-hex-casing-key-group";
    let message = [0x42u8; 32];
    let included = [1u16, 2];

    let first = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("first interactive open succeeds");
    assert!(
        !first.idempotent,
        "a fresh open must not be reported as idempotent"
    );

    // Reopen with message_hex upper-cased; every other field (including
    // the attempt context, which derives from the decoded bytes) is
    // identical. This must be idempotent, not a SessionConflict.
    ensure_interactive_dkg_session(session_id, key_group);
    let attempt_context =
        interactive_test_attempt_context(session_id, key_group, &message, &included, 1);
    let reopened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message).to_uppercase(),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: None,
        signing_intent: None,
        attempt_context,
    })
    .expect("re-cased reopen of an identical attempt must be accepted");
    assert!(
        reopened.idempotent,
        "re-cased message_hex reopen of an identical attempt must be idempotent"
    );
}

// interactive_session_abort_success_total must count only aborts that
// actually destroyed live state. No-op calls (unknown session, or an
// attempt_id filter that matched nothing) still bump calls_total but
// must not inflate the success counter.
#[test]
fn interactive_session_abort_success_metric_counts_only_real_aborts() {
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-abort-metric";
    let key_group = "interactive-test-key-group";
    let message = [0x73u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");

    // No-op: unknown session.
    let noop = interactive_session_abort(InteractiveSessionAbortRequest {
        session_id: "no-such-session".to_string(),
        attempt_id: None,
    })
    .expect("no-op abort still returns Ok");
    assert!(!noop.aborted);

    // No-op: right session, attempt_id filter that matches nothing.
    let noop_filter = interactive_session_abort(InteractiveSessionAbortRequest {
        session_id: session_id.to_string(),
        attempt_id: Some("deadbeef".to_string()),
    })
    .expect("filtered no-op abort returns Ok");
    assert!(!noop_filter.aborted);

    let after_noops = hardening_metrics();
    assert_eq!(
        after_noops.interactive_session_abort_calls_total, 2,
        "every abort entry, including no-ops, increments calls_total"
    );
    assert_eq!(
        after_noops.interactive_session_abort_success_total, 0,
        "no-op aborts must not increment success_total"
    );

    // Real abort of the live attempt.
    let aborted = interactive_session_abort(InteractiveSessionAbortRequest {
        session_id: session_id.to_string(),
        attempt_id: Some(opened.attempt_id.clone()),
    })
    .expect("real abort");
    assert!(aborted.aborted);

    let after_real = hardening_metrics();
    assert_eq!(after_real.interactive_session_abort_calls_total, 3);
    assert_eq!(
        after_real.interactive_session_abort_success_total, 1,
        "a real abort increments success_total exactly once"
    );
}

// An absent optional request field must be omitted from the serialized
// form (not emitted as null), matching every other Option field in the
// API surface, and a payload that omits it must still deserialize.
#[test]
fn build_taproot_tx_request_omits_absent_script_tree_hex() {
    let request = BuildTaprootTxRequest {
        session_id: "s".to_string(),
        inputs: vec![],
        outputs: vec![],
        script_tree_hex: None,
    };
    let json = serde_json::to_string(&request).expect("serialize");
    assert!(
        !json.contains("script_tree_hex"),
        "absent script_tree_hex must be omitted, not serialized as null: {json}"
    );

    let round_trip: BuildTaprootTxRequest =
        serde_json::from_str(r#"{"session_id":"s","inputs":[],"outputs":[]}"#)
            .expect("a payload omitting script_tree_hex must deserialize");
    assert!(round_trip.script_tree_hex.is_none());
}

#[test]
fn verify_signature_share_verdicts_match_aggregate_and_handle_edges() {
    use crate::api::ShareVerificationVerdict;

    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "verify-share-session";
    let key_group = "interactive-test-key-group";
    let message = [0x77u8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let member2_nonces = member2.nonces_hex.clone();
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );
    // Member 1's valid share (engine round2); member 2's valid share (stateless).
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("round 2 share");
    let member2_valid = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member2_nonces,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 valid share");

    // Member 2's INVALID share: validly signed over a DIFFERENT package.
    let bogus_member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("bogus member 2 nonces");
    let other_message = [0x88u8; 32];
    let other_package = interactive_package_for_test(
        &other_message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: bogus_member2.commitment.data_hex.clone(),
            },
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: bogus_member2.commitment.data_hex,
            },
        ],
    );
    let bogus_share = sign_share(SignShareRequest {
        signing_package_hex: other_package,
        nonces_hex: bogus_member2.nonces_hex,
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("bogus member 2 share");

    let verdict = |share_hex: String, member: u16| {
        verify_signature_share(crate::api::VerifySignatureShareRequest {
            session_id: session_id.to_string(),
            signing_package_hex: signing_package_hex.clone(),
            signature_share_hex: share_hex,
            member_identifier: member,
            taproot_merkle_root_hex: None,
        })
        .expect("verify")
        .verdict
    };

    // Valid shares verify Valid; the wrong-package share is Invalid.
    assert_eq!(
        verdict(round2.signature_share_hex.clone(), 1),
        ShareVerificationVerdict::Valid
    );
    assert_eq!(
        verdict(member2_valid.signature_share.data_hex.clone(), 2),
        ShareVerificationVerdict::Valid
    );
    assert_eq!(
        verdict(bogus_share.signature_share.data_hex.clone(), 2),
        ShareVerificationVerdict::Invalid
    );

    // Undecodable member share bytes, WITH established context (member 2 is in
    // this group, session ready) -> Invalid (self-incriminating member fault).
    assert_eq!(
        verdict("ee".to_string(), 2),
        ShareVerificationVerdict::Invalid
    );
    // A member with no verifying share in the group -> Indeterminate.
    assert_eq!(
        verdict(round2.signature_share_hex.clone(), 9),
        ShareVerificationVerdict::Indeterminate
    );
    // Ordering contract: undecodable share bytes for a member NOT in the group
    // are Indeterminate, NOT Invalid - the share is only judged once session /
    // DKG / membership are established, so blame never precedes that context.
    assert_eq!(
        verdict("ee".to_string(), 9),
        ShareVerificationVerdict::Indeterminate
    );
    // Package-membership contract: member 3 is in the GROUP (threshold 2 of {1,2,3})
    // but OMITTED from this attempt's package (commitments {1,2}). The package
    // omission is coordinator/context input, so neither a decodable share NOR
    // undecodable bytes may blame member 3 - both are Indeterminate, never Invalid.
    assert_eq!(
        verdict(round2.signature_share_hex.clone(), 3),
        ShareVerificationVerdict::Indeterminate
    );
    assert_eq!(
        verdict("ee".to_string(), 3),
        ShareVerificationVerdict::Indeterminate
    );
    // Undecodable signing package (coordinator input) -> Indeterminate.
    assert_eq!(
        verify_signature_share(crate::api::VerifySignatureShareRequest {
            session_id: session_id.to_string(),
            signing_package_hex: "ee".to_string(),
            signature_share_hex: round2.signature_share_hex.clone(),
            member_identifier: 1,
            taproot_merkle_root_hex: None,
        })
        .expect("verify")
        .verdict,
        ShareVerificationVerdict::Indeterminate
    );
    // Unknown session -> Indeterminate.
    assert_eq!(
        verify_signature_share(crate::api::VerifySignatureShareRequest {
            session_id: "no-such-session".to_string(),
            signing_package_hex: signing_package_hex.clone(),
            signature_share_hex: round2.signature_share_hex.clone(),
            member_identifier: 1,
            taproot_merkle_root_hex: None,
        })
        .expect("verify")
        .verdict,
        ShareVerificationVerdict::Indeterminate
    );
    // Ordering contract: even undecodable share bytes for an unknown session are
    // Indeterminate (session context is resolved before the share is judged).
    assert_eq!(
        verify_signature_share(crate::api::VerifySignatureShareRequest {
            session_id: "no-such-session".to_string(),
            signing_package_hex: signing_package_hex.clone(),
            signature_share_hex: "ee".to_string(),
            member_identifier: 1,
            taproot_merkle_root_hex: None,
        })
        .expect("verify")
        .verdict,
        ShareVerificationVerdict::Indeterminate
    );
    // Malformed taproot root (coordinator/wallet context) -> Indeterminate,
    // returned in-band, NOT escaped to the error channel (verdict contract).
    assert_eq!(
        verify_signature_share(crate::api::VerifySignatureShareRequest {
            session_id: session_id.to_string(),
            signing_package_hex: signing_package_hex.clone(),
            signature_share_hex: round2.signature_share_hex.clone(),
            member_identifier: 1,
            taproot_merkle_root_hex: Some("not-hex".to_string()),
        })
        .expect("malformed taproot root must not error out-of-band")
        .verdict,
        ShareVerificationVerdict::Indeterminate
    );

    // Equivalence guard: aggregate's AllCheaters verdict over [member 1 valid,
    // member 2 bogus] must name exactly the share verify_signature_share calls
    // Invalid (member 2), and not the one it calls Valid (member 1).
    let err = interactive_aggregate(InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex: signing_package_hex.clone(),
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex.clone(),
            },
            bogus_share.signature_share,
        ],
        taproot_merkle_root_hex: None,
    })
    .expect_err("an invalid share must fail aggregation closed");
    let candidate_culprits = match err {
        EngineError::AggregateShareVerificationFailed {
            candidate_culprits, ..
        } => candidate_culprits,
        other => panic!("expected AggregateShareVerificationFailed, got {other:?}"),
    };
    assert_eq!(
        candidate_culprits,
        vec![2],
        "aggregate's culprit verdict must match verify_signature_share: {candidate_culprits:?}"
    );
}

// Script-path (tweaked-root) companion to the equivalence test above: the
// None-root case pins parity on the untweaked path; this pins it on the taproot
// tweak path, where the even-Y/tweak machinery is exercised. Shares are produced
// with sign_with_tweak (== sign under a taproot-tweaked key package, the
// production taproot signing path), and verify_signature_share / aggregate are
// driven with Some(root). The clinching assertion is that the SAME tweaked share
// is Invalid under None: the root must be materially applied, not ignored.
#[test]
fn verify_signature_share_tweaked_root_matches_aggregate() {
    use crate::api::ShareVerificationVerdict;

    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "verify-share-tweaked-session";
    let key_group = "interactive-test-key-group";
    let message = [0x55u8; 32];
    let included = [1u16, 2];
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    let taproot_merkle_root = [0x11u8; 32];
    let taproot_merkle_root_hex = hex::encode(taproot_merkle_root);
    let opened = interactive_session_open(InteractiveSessionOpenRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: hex::encode(message),
        key_group: key_group.to_string(),
        threshold: 2,
        taproot_merkle_root_hex: Some(taproot_merkle_root_hex.clone()),
        signing_intent: None,
        attempt_context: interactive_test_attempt_context(
            session_id, key_group, &message, &included, 1,
        ),
    })
    .expect("opens with the tweaked root");
    let member1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 interactive nonces");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: member1.commitments_hex.clone(),
            },
            member2.commitment.clone(),
        ],
    );

    // Produce a TWEAKED round-2 share: sign_with_tweak signs under a
    // taproot-tweaked key package, exactly the production taproot signing path.
    let sign_tweaked = |key_package_hex: &str, nonces_hex: &str, package_hex: &str| -> String {
        let key_package = frost::keys::KeyPackage::deserialize(
            &hex::decode(key_package_hex).expect("key package hex"),
        )
        .expect("key package");
        let nonces = frost::round1::SigningNonces::deserialize(
            &hex::decode(nonces_hex).expect("nonces hex"),
        )
        .expect("nonces");
        let package =
            frost::SigningPackage::deserialize(&hex::decode(package_hex).expect("package hex"))
                .expect("signing package");
        let share = frost::round2::sign_with_tweak(
            &package,
            &nonces,
            &key_package,
            Some(taproot_merkle_root.as_slice()),
        )
        .expect("sign_with_tweak");
        hex::encode(share.serialize())
    };

    let share1 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("member 1 tweaked Round2 share")
    .signature_share_hex;
    let share2 = sign_tweaked(
        key_packages[&2].data_hex.expose_secret(),
        &member2.nonces_hex,
        &signing_package_hex,
    );

    let verdict = |share_hex: String, member: u16, root: Option<String>| {
        verify_signature_share(crate::api::VerifySignatureShareRequest {
            session_id: session_id.to_string(),
            signing_package_hex: signing_package_hex.clone(),
            signature_share_hex: share_hex,
            member_identifier: member,
            taproot_merkle_root_hex: root,
        })
        .expect("verify")
        .verdict
    };

    // Each tweaked share verifies Valid UNDER THE ROOT...
    assert_eq!(
        verdict(share1.clone(), 1, Some(taproot_merkle_root_hex.clone())),
        ShareVerificationVerdict::Valid
    );
    assert_eq!(
        verdict(share2.clone(), 2, Some(taproot_merkle_root_hex.clone())),
        ShareVerificationVerdict::Valid
    );
    // ...and the SAME tweaked share is Invalid WITHOUT the root: the taproot
    // tweak is materially applied to the verifying material, not ignored. (This
    // is what makes the Some(root) Valid verdicts meaningful.)
    assert_eq!(
        verdict(share1.clone(), 1, None),
        ShareVerificationVerdict::Invalid
    );

    // A bogus tweaked share for member 2: validly signed (with the root) over a
    // DIFFERENT package, so it fails verification against this package.
    let other_message = [0x66u8; 32];
    let bogus_member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("bogus member 2 nonces");
    let other_package_hex = interactive_package_for_test(
        &other_message,
        vec![
            bogus_member2.commitment.clone(),
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: member1.commitments_hex,
            },
        ],
    );
    let bogus_share2 = sign_tweaked(
        key_packages[&2].data_hex.expose_secret(),
        &bogus_member2.nonces_hex,
        &other_package_hex,
    );
    assert_eq!(
        verdict(
            bogus_share2.clone(),
            2,
            Some(taproot_merkle_root_hex.clone())
        ),
        ShareVerificationVerdict::Invalid
    );

    // Equivalence on the TWEAKED path: aggregate's AllCheaters verdict over
    // [member 1 valid, member 2 bogus] with the same root must name exactly the
    // share verify_signature_share calls Invalid (member 2), and not member 1.
    let err = interactive_aggregate(InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        signing_package_hex: signing_package_hex.clone(),
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: share1.clone(),
            },
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&2].identifier.clone(),
                data_hex: bogus_share2,
            },
        ],
        taproot_merkle_root_hex: Some(taproot_merkle_root_hex),
    })
    .expect_err("an invalid tweaked share must fail aggregation closed");
    let candidate_culprits = match err {
        EngineError::AggregateShareVerificationFailed {
            candidate_culprits, ..
        } => candidate_culprits,
        other => panic!("expected AggregateShareVerificationFailed, got {other:?}"),
    };
    assert_eq!(
        candidate_culprits,
        vec![2],
        "tweaked aggregate culprit must match verify_signature_share: {candidate_culprits:?}"
    );
}

// Migrated from the deleted run_dkg_rejects_when_signed_attestation_status_mismatches_env:
// the coarse dealer run_dkg was removed, so drive the PRESERVED shared provenance gate
// directly. A validly signed attestation whose payload status disagrees with the required
// env status must be rejected with reason_code "attestation_status_mismatch".
#[test]
fn enforce_provenance_gate_rejects_signed_attestation_status_mismatch() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let (trust_root, attestation_payload, attestation_signature_hex) =
        build_signed_provenance_attestation(
            "pending",
            TBTC_SIGNER_RUNTIME_VERSION,
            Some(now_unix().saturating_add(300)),
        );

    std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
    );
    std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
        &attestation_payload,
    );
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
        &attestation_signature_hex,
    );
    std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

    let err = enforce_provenance_gate().expect_err("expected status mismatch rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert_eq!(reason_code, "attestation_status_mismatch");

    clear_state_storage_policy_overrides();
}

// Migrated from the deleted run_dkg_rejects_when_signed_attestation_runtime_version_mismatch:
// a validly signed, status-APPROVED attestation whose runtime_version disagrees with the
// build's TBTC_SIGNER_RUNTIME_VERSION must be rejected with "runtime_version_not_attested".
#[test]
fn enforce_provenance_gate_rejects_signed_attestation_runtime_version_mismatch() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let (trust_root, attestation_payload, attestation_signature_hex) =
        build_signed_provenance_attestation(
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
            "99.99.99",
            Some(now_unix().saturating_add(300)),
        );

    std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
    );
    std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, &trust_root);
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
        &attestation_payload,
    );
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
        &attestation_signature_hex,
    );
    std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

    let err = enforce_provenance_gate().expect_err("expected runtime version mismatch rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert_eq!(reason_code, "runtime_version_not_attested");

    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_two_attempts_coexist_same_session() {
    // Test that two attempts can coexist in the same session for different members
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-two-attempts-test";
    let session_id = "interactive-session-id";
    let message = [0xcu8; 32];
    let included = [1u16, 2, 3];
    let threshold = 2u16;
    let key_packages = ensure_interactive_dkg_session(session_id, key_group);

    // Open attempt A for member 1
    let opened_a1 =
        open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, threshold)
            .expect("member 1 opens attempt A");

    // Open attempt B for member 2
    let opened_b2 =
        open_interactive_for_test(session_id, key_group, &message, &included, 2, 2, threshold)
            .expect("member 2 opens attempt B");

    assert_ne!(
        opened_a1.attempt_id, opened_b2.attempt_id,
        "attempt IDs must differ"
    );

    // Attempt B's member 2 creates live Round1 nonces, independent of attempt A.
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened_b2.attempt_id.clone(),
        member_identifier: 2,
    })
    .expect("member 2 round 1 in attempt B");

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");

        // Verify both attempt scopes exist
        assert!(session
            .interactive
            .interactive_signing
            .contains_key(&opened_a1.attempt_id));
        assert!(session
            .interactive
            .interactive_signing
            .contains_key(&opened_b2.attempt_id));

        // Verify member 1 is in attempt A scope
        assert!(session
            .interactive
            .interactive_signing
            .get(&opened_a1.attempt_id)
            .expect("attempt A scope exists")
            .contains_key(&1));

        // Verify member 2 is in attempt B scope
        assert!(session
            .interactive
            .interactive_signing
            .get(&opened_b2.attempt_id)
            .expect("attempt B scope exists")
            .contains_key(&2));

        // Verify member counts per attempt
        let attempt_a_members = session
            .interactive
            .interactive_signing
            .get(&opened_a1.attempt_id)
            .expect("attempt A scope exists")
            .len();
        let attempt_b_members = session
            .interactive
            .interactive_signing
            .get(&opened_b2.attempt_id)
            .expect("attempt B scope exists")
            .len();
        assert_eq!(
            attempt_a_members, 1,
            "attempt A should have exactly 1 member"
        );
        assert_eq!(
            attempt_b_members, 1,
            "attempt B should have exactly 1 member"
        );
    }

    // Complete attempt A via round1 -> round2 -> aggregate, using an ad-hoc
    // (non-interactive) second signer so attempt B's live state is untouched.
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened_a1.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1 in attempt A");
    let member3 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&3].identifier.clone(),
        key_package_hex: key_packages[&3].data_hex.clone(),
    })
    .expect("member 3 ad-hoc commitments");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member3.commitment.clone(),
        ],
    );
    let round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened_a1.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect("member 1 round 2 share");
    let member3_share = sign_share(SignShareRequest {
        signing_package_hex: signing_package_hex.clone(),
        nonces_hex: member3.nonces_hex,
        key_package_identifier: key_packages[&3].identifier.clone(),
        key_package_hex: key_packages[&3].data_hex.clone(),
    })
    .expect("member 3 ad-hoc share");
    let aggregate_request = InteractiveAggregateRequest {
        session_id: session_id.to_string(),
        attempt_id: opened_a1.attempt_id.clone(),
        signing_package_hex,
        signature_shares: vec![
            crate::api::NativeFrostSignatureShare {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round2.signature_share_hex,
            },
            member3_share.signature_share,
        ],
        taproot_merkle_root_hex: None,
    };
    interactive_aggregate(aggregate_request).expect("aggregate succeeds");

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");

        // Verify attempt A scope is cleared (aggregated)
        assert!(!session
            .interactive
            .interactive_signing
            .contains_key(&opened_a1.attempt_id));

        // Verify attempt B scope still exists with member 2's Round1 nonces live
        assert!(session
            .interactive
            .interactive_signing
            .contains_key(&opened_b2.attempt_id));
        let attempt_b_scope = session
            .interactive
            .interactive_signing
            .get(&opened_b2.attempt_id)
            .expect("attempt B scope still exists");
        assert!(
            attempt_b_scope.contains_key(&2),
            "member 2 still in attempt B scope"
        );

        // Verify member 2's Round1 nonces are still live (not zeroized)
        let member_b_state = attempt_b_scope.get(&2).expect("member 2 state exists");
        assert!(
            member_b_state.round1.is_some(),
            "member 2's Round1 nonces still live"
        );
    }
}

#[test]
fn interactive_concurrent_attempt_cap_enforced() {
    // Test that the n-t+1 concurrent attempt cap is enforced
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-cap-test";
    let session_id = "interactive-cap-session";
    let message = [0xdu8; 32];
    // The DKG fixture is threshold=2; n=3, t=2 so cap = n - t + 1 = 3 - 2 + 1 = 2
    let included = [1u16, 2, 3];
    let threshold = 2u16;
    let cap: usize = 2; // n - t + 1

    // Open cap number of attempts (should succeed)
    let mut opened_attempts = Vec::new();
    for i in 0..cap {
        let member_identifier = (i + 1) as u16;
        let wire_attempt_number = (i + 1) as u32;
        let opened = open_interactive_for_test(
            session_id,
            key_group,
            &message,
            &included,
            wire_attempt_number,
            member_identifier,
            threshold,
        )
        .unwrap_or_else(|e| panic!("member {member_identifier} should open attempt: {e:?}"));
        opened_attempts.push(opened);
    }

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");

        // Verify exactly cap attempts exist
        assert_eq!(
            session.interactive.interactive_signing.len(),
            cap,
            "should have exactly cap attempt scopes"
        );

        // Verify each attempt has exactly one member
        for (i, opened) in opened_attempts.iter().enumerate() {
            let member_identifier = (i + 1) as u16;
            let attempt_scope = session
                .interactive
                .interactive_signing
                .get(&opened.attempt_id)
                .unwrap_or_else(|| panic!("attempt {} scope exists", i + 1));
            assert_eq!(
                attempt_scope.len(),
                1,
                "attempt should have exactly one member"
            );
            assert!(
                attempt_scope.contains_key(&member_identifier),
                "member {member_identifier} should be in attempt"
            );
        }
    }

    // Attempt to open one more attempt (should fail due to cap)
    let overflow_member_identifier = (cap + 1) as u16;
    let overflow_wire_attempt_number = (cap + 1) as u32;
    let overflow_attempt = open_interactive_for_test(
        session_id,
        key_group,
        &message,
        &included,
        overflow_wire_attempt_number,
        overflow_member_identifier,
        threshold,
    );
    assert!(
        matches!(
            &overflow_attempt,
            Err(EngineError::SigningPolicyRejected {
                session_id: bound_session_id,
                reason_code: bound_reason_code,
                detail: bound_detail,
            }) if bound_session_id == session_id
                && bound_reason_code == "concurrent_attempt_cap_exceeded"
                && bound_detail.contains(&format!(
                    "session [{}] cannot open new attempt",
                    session_id
                ))
        ),
        "overflow attempt should be rejected with concurrent_attempt_cap_exceeded, got {overflow_attempt:?}"
    );

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");

        // Verify still only cap attempts exist (overflow attempt not added)
        assert_eq!(
            session.interactive.interactive_signing.len(),
            cap,
            "overflow attempt should not increase attempt count"
        );
    }
}

#[test]
fn interactive_round2_durable_markers_persist_before_share_release() {
    // Regression test for the Round2 durable-marker-ordering fix.
    //
    // `interactive_round2` MUST insert the consumed-attempt and aggregate-
    // authorization markers BEFORE persisting, so that any failure path
    // that hits the persisted state file (a post-rename directory-sync
    // fault, a process restart, a crash) still finds the markers on disk
    // and a retry is rejected as a replay. The install path removes the
    // member's nonce-bearing entry in the same step, so a non-durable
    // marker would silently allow a second share to be released for an
    // already-consumed attempt - which is exactly the silent handoff
    // break this fix is supposed to prevent.
    //
    // The test exercises the post-rename persist-failure path
    // (state_file_replaced = true) and then simulates a process restart
    // to force a reload from disk. If a future change regresses the
    // ordering back to "insert markers after persist", the markers are
    // in memory only, the saved state file does not contain them, the
    // restart reload sees an empty marker set, and the retry below is
    // NOT rejected as a replay.
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("interactive_round2_marker_ordering");
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-round2-marker-ordering";
    let key_group = "interactive-test-key-group";
    let message = [0x73u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("opens");
    let round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
    let member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 nonces");
    let signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: round1.commitments_hex,
            },
            member2.commitment,
        ],
    );
    let round2_request = InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex,
    };
    let consumed_marker = interactive_consumed_marker(&opened.attempt_id, 1);

    // Post-rename persist fault: the temp file is renamed onto the
    // state-file path (state_file_replaced = true), but the parent
    // directory sync fails. This is the durability boundary the
    // marker ordering protects: the markers in memory at the moment of
    // persist are exactly the markers in the saved state file.
    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let faulted = interactive_round2(round2_request.clone())
        .expect_err("post-rename persist fault must release no share");
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(faulted, EngineError::Internal(ref m) if m.contains("injected persist fault")),
        "unexpected error: {faulted:?}"
    );

    // Crash before any successful in-process repair. The pending registry
    // is memory-only and disappears; the saved state file MUST carry
    // the marker, otherwise the markers were only in memory and a real
    // restart would lose them (a future regression that inserts markers
    // AFTER persist would land here).
    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            session
                .interactive
                .consumed_attempt_markers
                .contains(&consumed_marker),
            "consumed-attempt marker must survive a process restart when the \
             post-rename persist replaced the state file (markers are inserted \
             BEFORE persist, so they ride the same write that fails the directory sync)"
        );
        assert!(
            session.interactive.interactive_signing.is_empty(),
            "post-rename failure destroys the live nonce-bearing state"
        );
    }

    // A retry must be rejected as a consumed-nonce replay because the
    // marker is on disk. If the marker were lost across restart, the
    // retry would proceed and could re-release a share.
    let retry = interactive_round2(round2_request)
        .expect_err("restart retry rejects the durable consumed attempt");
    assert!(
        matches!(retry, EngineError::ConsumedNonceReplay { .. }),
        "unexpected retry error: {retry:?}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn interactive_round1_reports_attempt_id_mismatch_for_superseded_member() {
    // Regression test for the stale-attempt-id lookup fix.
    //
    // `interactive_state_for_attempt_mut` MUST distinguish "member has no
    // live attempt at all" from "member's live attempt moved to a
    // different attempt_id". The nested-map restructure makes the outer
    // key the attempt_id, so a lookup that just did
    // `session.interactive.interactive_signing.get_mut(attempt_id)?` would return
    // SessionNotFound for a member who has a live entry under a DIFFERENT
    // attempt_id - masking the fact that the member is actively signing,
    // just on a newer attempt. The fix adds a fallback scan of the other
    // live attempt scopes for this member, and returns a Validation
    // error specifically identifying the attempt-id MISMATCH (both the
    // stale id passed in and the current live id on the member).
    let _guard = lock_test_state();
    reset_for_tests();

    let session_id = "interactive-round1-supersede-mismatch";
    let key_group = "interactive-test-key-group";
    let message = [0x74u8; 32];
    let included = [1u16, 2];

    let first = open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, 2)
        .expect("member 1 opens attempt 1");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: first.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1 on attempt 1");

    let second = open_interactive_for_test(session_id, key_group, &message, &included, 2, 1, 2)
        .expect("member 1 advances to attempt 2");
    assert_ne!(first.attempt_id, second.attempt_id);

    // Round1 with the stale attempt_id must report a Validation error
    // that specifically identifies the attempt-id MISMATCH (both the
    // stale id we passed and the live id this member now occupies),
    // not a generic SessionNotFound. Without the fix, the function
    // would do session.interactive.interactive_signing.get_mut(stale_id)? and
    // return SessionNotFound on the None.
    let stale = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: first.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect_err("stale attempt id must be rejected");
    match &stale {
        EngineError::Validation(message) => {
            assert!(
                message.contains(&first.attempt_id)
                    && message.contains(&second.attempt_id)
                    && message.contains("does not match"),
                "expected a specific attempt-id mismatch error mentioning \
                 both stale id [{}] and live id [{}], got {:?}",
                first.attempt_id,
                second.attempt_id,
                message,
            );
        }
        other => panic!("expected EngineError::Validation for attempt-id mismatch, got {other:?}"),
    }
    // The Round2 path shares the same lookup. A Round2 with the stale
    // attempt_id must report the same specific mismatch error, not a
    // generic not-found (the Round2 entry point also gates on this).
    // The signing package only needs to deserialize cleanly - the
    // stale attempt_id lookup fails BEFORE any package verification.
    let key_packages = interactive_test_key_packages();
    let second_round1 = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: second.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("member 1 round 1 on attempt 2 to seed a valid signing package");
    let second_member2 = generate_nonces_and_commitments(GenerateNoncesAndCommitmentsRequest {
        key_package_identifier: key_packages[&2].identifier.clone(),
        key_package_hex: key_packages[&2].data_hex.clone(),
    })
    .expect("member 2 stateless commitments for the live attempt");
    let live_signing_package_hex = interactive_package_for_test(
        &message,
        vec![
            NativeFrostCommitment {
                identifier: key_packages[&1].identifier.clone(),
                data_hex: second_round1.commitments_hex,
            },
            second_member2.commitment,
        ],
    );
    let stale_round2 = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: first.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: live_signing_package_hex,
    })
    .expect_err("stale attempt id on Round2 must be rejected");
    match &stale_round2 {
        EngineError::Validation(message) => {
            assert!(
                message.contains(&first.attempt_id)
                    && message.contains(&second.attempt_id)
                    && message.contains("does not match"),
                "Round2 expected attempt-id mismatch mentioning both stale id \
                 [{}] and live id [{}], got {:?}",
                first.attempt_id,
                second.attempt_id,
                message,
            );
        }
        other => panic!(
            "expected EngineError::Validation for attempt-id mismatch on \
             Round2, got {other:?}"
        ),
    }

    // A truly-absent member (no live entry at all) gets a different
    // error: SessionNotFound. The fix must distinguish this from
    // "the member has a live entry, just under a different attempt_id".
    let absent = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: first.attempt_id.clone(),
        member_identifier: 7, // not opened at all
    })
    .expect_err("a member with no live entry must be SessionNotFound");
    assert!(
        matches!(absent, EngineError::SessionNotFound { .. }),
        "absent member lookup must be SessionNotFound (not the supersede \
         error), got {absent:?}"
    );
}

#[test]
fn interactive_cap_does_not_double_count_member_vacating_own_attempt_scope() {
    // Regression test for the cap-accounting vacating path.
    //
    // When a member advances to a NEW attempt under a fresh attempt_id,
    // the prior scope they vacate must NOT count against the n-t+1
    // cap (the install path drops it because empty scopes are pruned
    // immediately). The Open cap check excludes the scope this member
    // would vacate entirely, so the net effect is one attempt in / one
    // attempt out, not two attempts added. Without that exclusion the
    // Open would reject the advance at the cap, blocking the retry
    // loop entirely.
    let _guard = lock_test_state();
    reset_for_tests();

    let key_group = "interactive-cap-vacate-key-group";
    let session_id = "interactive-cap-vacate";
    let message = [0x75u8; 32];
    // DKG fixture is threshold=2; cap = 3 - 2 + 1 = 2.
    let included = [1u16, 2, 3];
    let threshold = 2u16;
    // Fill the cap: member 1 opens attempt A (wire attempt 1),
    // member 2 opens attempt B (wire attempt 2 - different attempt_id
    // so they hold distinct concurrent attempts under the n-t+1 cap).
    let opened_a =
        open_interactive_for_test(session_id, key_group, &message, &included, 1, 1, threshold)
            .expect("member 1 opens attempt A");
    let opened_b =
        open_interactive_for_test(session_id, key_group, &message, &included, 2, 2, threshold)
            .expect("member 2 opens attempt B");
    assert_ne!(
        opened_a.attempt_id, opened_b.attempt_id,
        "attempts must differ"
    );
    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert_eq!(
            session.interactive.interactive_signing.len(),
            2,
            "two attempt scopes are at the cap"
        );
    }

    // Member 3 cannot open a 3rd attempt - cap reached.
    let overflow =
        open_interactive_for_test(session_id, key_group, &message, &included, 3, 3, threshold)
            .expect_err("opening a 3rd attempt while at cap must fail");
    match &overflow {
        EngineError::SigningPolicyRejected { reason_code, .. } => assert_eq!(
            reason_code, "concurrent_attempt_cap_exceeded",
            "expected concurrent_attempt_cap_exceeded, got {overflow:?}"
        ),
        other => panic!("expected SigningPolicyRejected for cap overflow, got {other:?}"),
    }

    // Member 1 advances to a NEW attempt under a fresh attempt_id
    // (wire attempt number 2). The prior scope they vacate is dropped
    // because empty scopes are pruned immediately; the cap check
    // excludes the scope this member would vacate, so the net is one
    // in / one out and the open succeeds. Without that exclusion, the
    let opened_c =
        open_interactive_for_test(session_id, key_group, &message, &included, 3, 1, threshold)
            .expect(
                "member 1 advancing to a new attempt must not double-count: \
                 the vacated prior scope is excluded from the cap",
            );
    assert_ne!(
        opened_c.attempt_id, opened_a.attempt_id,
        "the advancing member's NEW attempt must have a fresh attempt_id"
    );
    assert_ne!(
        opened_c.attempt_id, opened_b.attempt_id,
        "the advancing member's NEW attempt must not collide with member 2's attempt"
    );

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        // The vacated scope is gone (member 1 was the sole member there).
        assert!(
            !session
                .interactive
                .interactive_signing
                .contains_key(&opened_a.attempt_id),
            "the vacated attempt scope is pruned immediately"
        );
        // Member 2's scope on attempt B is untouched by member 1's advance.
        assert!(
            session
                .interactive
                .interactive_signing
                .contains_key(&opened_b.attempt_id),
            "sibling member's scope is untouched by the advance"
        );
        // Member 1 is now under the new attempt C.
        assert!(
            session
                .interactive
                .interactive_signing
                .contains_key(&opened_c.attempt_id),
            "the new attempt scope holds the advancing member"
        );
        let new_scope = session
            .interactive
            .interactive_signing
            .get(&opened_c.attempt_id)
            .expect("new attempt scope exists");
        assert!(new_scope.contains_key(&1), "member 1 is on the new attempt");
        // Cap is still 2 (B and C), not 3.
        assert_eq!(
            session.interactive.interactive_signing.len(),
            2,
            "the cap is n-t+1 = 2, not n-t = 1; the vacated scope did not double-count"
        );
    }
}

// -- DKGPart1 input validation regression tests ----------------------------
//
// The validation hardening PR added `min_signers >= 2` to `frost_ops::dkg_part1`
// before any downstream use of the input. These tests pin that branch so a
// future edit that drops or weakens the cap cannot silently regress it.

#[test]
fn dkg_part1_rejects_min_signers_below_two() {
    let _guard = lock_test_state();
    reset_for_tests();

    let request = DkgPart1Request {
        participant_identifier: participant_identifier_to_frost_identifier(1)
            .map(frost_identifier_to_go_string)
            .expect("u16::1 to frost identifier"),
        max_signers: 3,
        min_signers: 1,
    };

    let err = dkg_part1(request).expect_err("min_signers < 2 must be rejected");
    let EngineError::Validation(detail) = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert!(
        detail.contains("min_signers must be at least 2"),
        "unexpected rejection detail: {detail}"
    );
}

#[test]
fn dkg_part1_rejects_min_signers_zero() {
    let _guard = lock_test_state();
    reset_for_tests();

    let request = DkgPart1Request {
        participant_identifier: participant_identifier_to_frost_identifier(1)
            .map(frost_identifier_to_go_string)
            .expect("u16::1 to frost identifier"),
        max_signers: 3,
        min_signers: 0,
    };

    let err = dkg_part1(request).expect_err("min_signers = 0 must be rejected");
    let EngineError::Validation(detail) = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert!(
        detail.contains("min_signers must be at least 2"),
        "unexpected rejection detail: {detail}"
    );
}

#[test]
fn dkg_part1_rejects_min_signers_greater_than_max_signers() {
    let _guard = lock_test_state();
    reset_for_tests();

    let request = DkgPart1Request {
        participant_identifier: participant_identifier_to_frost_identifier(1)
            .map(frost_identifier_to_go_string)
            .expect("u16::1 to frost identifier"),
        max_signers: 3,
        min_signers: 4,
    };

    let err = dkg_part1(request).expect_err("min_signers > max_signers must be rejected");
    let EngineError::Validation(detail) = err else {
        panic!("unexpected error variant: {err:?}");
    };
    assert!(
        detail.contains("min_signers exceeds max_signers"),
        "unexpected rejection detail: {detail}"
    );
}

#[test]
fn dkg_part1_accepts_min_signers_at_threshold() {
    let _guard = lock_test_state();
    reset_for_tests();

    let expected_identifier = participant_identifier_to_frost_identifier(1)
        .map(frost_identifier_to_go_string)
        .expect("u16::1 to frost identifier");
    let request = DkgPart1Request {
        participant_identifier: expected_identifier.clone(),
        max_signers: 3,
        min_signers: 2,
    };

    let result = dkg_part1(request).expect("min_signers = 2 must be accepted");
    assert!(!result.secret_package_hex.expose_secret().is_empty());
    assert_eq!(result.package.identifier, expected_identifier);
}
