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

// Regression for the review finding on the seed-unification change:
// the engine must seed the coordinator shuffle from the padded raw
// message it receives (the Go layer's messageDigestFromBigInt
// output), NOT from its internal SHA256 transcript digest. This test
// reimplements the Go-side derivation inline -- independently of the
// engine helpers -- and proves the resulting context is accepted
// through the real strict-mode StartSignRound call path.
#[test]
fn start_sign_round_accepts_go_derived_attempt_context_in_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-go-style-attempt-context";
    // A 32-byte signing digest, as production always supplies (the
    // engine message IS the digest).
    let message_hex = "5f78c33274e43fa9de5659265c1d917e25c03722dcb0b8d27db8d5feaa813953";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    // --- Go-side derivation, reimplemented inline ---
    // attempt.DeriveAttemptSeed(keyGroupBytes, sessionID, digest):
    // SHA256 over the raw concatenation, digest = the 32 message
    // bytes themselves.
    let message_bytes = hex::decode(message_hex).expect("message decodes");
    let mut hasher = Sha256::new();
    hasher.update(dkg_result.key_group.as_bytes());
    hasher.update(session_id.as_bytes());
    hasher.update(&message_bytes);
    let go_attempt_seed = hasher.finalize();
    // foldAttemptSeed: first 8 bytes, big-endian, reinterpreted i64.
    let mut go_seed_bytes = [0_u8; 8];
    go_seed_bytes.copy_from_slice(&go_attempt_seed[..8]);
    let go_shuffle_seed = i64::from_be_bytes(go_seed_bytes);
    // SelectCoordinator(included, seed, attemptNumber): 0-based
    // RFC-21 attempt number; first logical attempt = 0.
    let included_participants = vec![1_u16, 2];
    let go_coordinator = select_coordinator_identifier(&included_participants, go_shuffle_seed, 0)
        .expect("go-style coordinator");

    // --- FFI wire encoding of the same logical attempt ---
    // wire attempt_number = RFC-21 AttemptNumber + 1; attempt_id is
    // engine-defined over the SHA256 transcript digest.
    let wire_attempt_number = 1_u32;
    let engine_message_digest_hex = hash_hex(&message_bytes);
    let fingerprint =
        roast_included_participants_fingerprint_hex(&included_participants).expect("fingerprint");
    let attempt_id = roast_attempt_id_hex(
        session_id,
        &engine_message_digest_hex,
        wire_attempt_number,
        go_coordinator,
        &fingerprint,
    )
    .expect("attempt id");

    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(included_participants.clone()),
        attempt_context: Some(AttemptContext {
            attempt_number: wire_attempt_number,
            coordinator_identifier: go_coordinator,
            included_participants,
            included_participants_fingerprint: fingerprint,
            attempt_id,
        }),
        attempt_transition_evidence: None,
    })
    .expect("strict StartSignRound must accept the Go-derived attempt context");
    assert_eq!(round_state.session_id, session_id);
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
                    package_hex: hex::encode(package.serialize().expect("round2 package")),
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
                secret_package_hex: hex::encode(secret_package_bytes),
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

fn seeded_round_state(session_id: &str) -> RoundState {
    let run_dkg_request = RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let start_request = StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };

    start_sign_round(start_request).expect("start sign round")
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
    std::env::remove_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV);
    std::env::remove_var(TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV);
    std::env::remove_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_START_SIGN_ROUND_P95_MS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_FINALIZE_SIGN_ROUND_P95_MS_ENV);
    std::env::remove_var(TBTC_SIGNER_CANARY_MAX_POLICY_REJECT_RATE_BPS_ENV);
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
        emergency_rekey_event: None,
        consumed_interactive_attempt_markers: vec![],
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

fn state_mutation_request(session_id: &str) -> RefreshSharesRequest {
    RefreshSharesRequest {
        session_id: session_id.to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "1111".to_string(),
        }],
    }
}

fn mutate_state_for_key_provider_test(
    session_id: &str,
) -> Result<RefreshSharesResult, EngineError> {
    refresh_shares(state_mutation_request(session_id))
}

#[test]
fn run_dkg_rejects_bootstrap_dealer_dkg_in_production_profile() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_PROFILE_ENV, TBTC_SIGNER_PROFILE_PRODUCTION);

    let err = run_dkg(RunDkgRequest {
        session_id: "session-production-bootstrap-dkg".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("production profile should reject bootstrap dealer DKG");

    let EngineError::LifecyclePolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "bootstrap_dealer_dkg_disabled_in_production");
}

#[test]
fn run_dkg_rejects_bootstrap_dealer_dkg_when_profile_is_missing_or_empty() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    for profile_value in [None, Some(" ")] {
        match profile_value {
            Some(value) => std::env::set_var(TBTC_SIGNER_PROFILE_ENV, value),
            None => std::env::remove_var(TBTC_SIGNER_PROFILE_ENV),
        }

        let err = run_dkg(RunDkgRequest {
            session_id: format!(
                "session-default-production-bootstrap-dkg-{}",
                profile_value.unwrap_or("missing").trim()
            ),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
            dkg_seed_hex: None,
        })
        .expect_err("missing/empty profile should reject bootstrap dealer DKG");

        let EngineError::LifecyclePolicyRejected { reason_code, .. } = err else {
            panic!("unexpected error variant");
        };
        assert_eq!(reason_code, "bootstrap_dealer_dkg_disabled_in_production");
    }
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
fn run_dkg_rejects_malformed_seed_as_validation_input() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    for (index, seed_hex, expected_message) in [
        (1, "not-hex", "dkg_seed_hex must be valid hex"),
        (2, "0102", "dkg_seed_hex decoded to [2] bytes, expected 32"),
    ] {
        let err = run_dkg(RunDkgRequest {
            session_id: format!("session-malformed-dkg-seed-{index}"),
            participants: vec![
                crate::api::DkgParticipant {
                    identifier: 1,
                    public_key_hex: "02aa".to_string(),
                },
                crate::api::DkgParticipant {
                    identifier: 2,
                    public_key_hex: "02bb".to_string(),
                },
            ],
            threshold: 2,
            dkg_seed_hex: Some(seed_hex.to_string()),
        })
        .expect_err("malformed DKG seed should be rejected");

        let EngineError::Validation(message) = err else {
            panic!("unexpected error variant");
        };
        assert!(
            message.contains(expected_message),
            "unexpected validation message: {message}"
        );
    }
}

#[test]
fn run_dkg_rejects_when_provenance_gate_requires_attestation() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
    std::env::set_var(TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV, "sigstore-main");
    std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-gate-missing-attestation".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected provenance gate rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "missing_attestation_status");

    clear_state_storage_policy_overrides();
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

#[test]
fn run_dkg_accepts_valid_signed_provenance_attestation() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let (trust_root, attestation_payload, attestation_signature_hex) =
        build_signed_provenance_attestation(
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
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

    let result = run_dkg(RunDkgRequest {
        session_id: "session-provenance-signed-attestation-accept".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    });
    assert!(result.is_ok(), "expected signed attestation acceptance");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_when_provenance_attestation_signature_missing() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let (trust_root, attestation_payload, _) = build_signed_provenance_attestation(
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
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
    std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-signed-attestation-missing-signature".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected missing signature rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "missing_attestation_signature");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_when_provenance_attestation_signature_invalid() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let (trust_root, attestation_payload, mut attestation_signature_hex) =
        build_signed_provenance_attestation(
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
            TBTC_SIGNER_RUNTIME_VERSION,
            Some(now_unix().saturating_add(300)),
        );
    let replacement = if attestation_signature_hex.ends_with('0') {
        "1"
    } else {
        "0"
    };
    attestation_signature_hex.replace_range(
        attestation_signature_hex.len() - 1..attestation_signature_hex.len(),
        replacement,
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

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-signed-attestation-invalid-signature".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected signature verification rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "attestation_signature_verification_failed");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_when_provenance_attestation_expired() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let (trust_root, attestation_payload, attestation_signature_hex) =
        build_signed_provenance_attestation(
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
            TBTC_SIGNER_RUNTIME_VERSION,
            Some(now_unix().saturating_sub(1)),
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

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-signed-attestation-expired".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected attestation expiry rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "attestation_expired");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_when_provenance_attestation_missing_expiry() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let (trust_root, attestation_payload, attestation_signature_hex) =
        build_signed_provenance_attestation(
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
            TBTC_SIGNER_RUNTIME_VERSION,
            None,
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

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-signed-attestation-missing-expiry".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected attestation missing expiry rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "missing_attestation_expiry");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_when_provenance_attestation_expiry_too_far_in_future() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let (trust_root, attestation_payload, attestation_signature_hex) =
        build_signed_provenance_attestation(
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
            TBTC_SIGNER_RUNTIME_VERSION,
            Some(now_unix().saturating_add(TBTC_SIGNER_PROVENANCE_MAX_ATTESTATION_TTL_SECONDS) + 1),
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

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-expiry-too-far".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected attestation expiry too far rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "attestation_expiry_too_far_in_future");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_when_provenance_trust_root_mismatches_signature_key() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let (_trust_root, attestation_payload, attestation_signature_hex) =
        build_signed_provenance_attestation(
            TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
            TBTC_SIGNER_RUNTIME_VERSION,
            Some(now_unix().saturating_add(300)),
        );

    let secp = Secp256k1::new();
    let wrong_secret_key =
        bitcoin::secp256k1::SecretKey::from_slice(&[0x22; 32]).expect("secret key");
    let wrong_keypair = bitcoin::secp256k1::Keypair::from_secret_key(&secp, &wrong_secret_key);
    let (wrong_trust_root, _) = XOnlyPublicKey::from_keypair(&wrong_keypair);

    std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
    );
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV,
        wrong_trust_root.to_string(),
    );
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV,
        &attestation_payload,
    );
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
        &attestation_signature_hex,
    );
    std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-wrong-trust-root".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected trust-root mismatch rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "attestation_signature_verification_failed");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_when_signed_attestation_runtime_version_mismatch() {
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

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-runtime-version-mismatch".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected runtime version mismatch rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "runtime_version_not_attested");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_when_signed_attestation_status_mismatches_env() {
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

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-status-mismatch".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected status mismatch rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "attestation_status_mismatch");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_invalid_curve_point_trust_root() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_PROVENANCE_GATE_ENV, "true");
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_STATUS_ENV,
        TBTC_SIGNER_REQUIRED_ATTESTATION_STATUS_APPROVED,
    );
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_TRUST_ROOT_ENV,
        "0000000000000000000000000000000000000000000000000000000000000000",
    );
    std::env::set_var(TBTC_SIGNER_PROVENANCE_ATTESTATION_PAYLOAD_ENV, "{}");
    std::env::set_var(
        TBTC_SIGNER_PROVENANCE_ATTESTATION_SIGNATURE_HEX_ENV,
        "aa".repeat(64),
    );
    std::env::set_var(TBTC_SIGNER_MIN_APPROVED_VERSION_ENV, "0.1.0");

    let err = run_dkg(RunDkgRequest {
        session_id: "session-provenance-invalid-curve-point-trust-root".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected invalid trust root rejection");

    let EngineError::ProvenanceGateRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "invalid_trust_root_format");

    clear_state_storage_policy_overrides();
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

#[test]
fn run_dkg_rejects_session_id_with_disallowed_characters() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let err = run_dkg(RunDkgRequest {
        session_id: "session-log\ninject".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected session_id validation rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("session_id contains disallowed characters"),
        "unexpected validation message: {message}"
    );

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_non_allowlisted_participant_under_admission_policy() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV, "true");
    std::env::set_var(TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV, "1,2");

    let err = run_dkg(RunDkgRequest {
        session_id: "session-admission-allowlist-reject".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected admission policy rejection");

    let EngineError::AdmissionPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "participant_identifier_not_allowlisted");

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_maps_admission_policy_config_error_to_rejection_with_counter() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV, "true");
    std::env::set_var(TBTC_SIGNER_ADMISSION_MIN_PARTICIPANTS_ENV, "not-a-number");

    let err = run_dkg(RunDkgRequest {
        session_id: "session-admission-invalid-policy-config".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected admission policy config rejection");

    let EngineError::AdmissionPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "invalid_policy_configuration");

    let metrics = hardening_metrics();
    assert_eq!(metrics.run_dkg_calls_total, 1);
    assert_eq!(metrics.run_dkg_admission_reject_total, 1);
    assert_eq!(metrics.run_dkg_success_total, 0);

    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_rejects_empty_admission_allowlist_as_invalid_config() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    std::env::set_var(TBTC_SIGNER_ENFORCE_ADMISSION_POLICY_ENV, "true");
    std::env::set_var(TBTC_SIGNER_ADMISSION_ALLOWLIST_IDENTIFIERS_ENV, "");

    let err = run_dkg(RunDkgRequest {
        session_id: "session-admission-empty-allowlist".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected admission policy config rejection");

    let EngineError::AdmissionPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "invalid_policy_configuration");

    let metrics = hardening_metrics();
    assert_eq!(metrics.run_dkg_calls_total, 1);
    assert_eq!(metrics.run_dkg_admission_reject_total, 1);

    clear_state_storage_policy_overrides();
}

fn build_policy_test_request(session_id: &str) -> BuildTaprootTxRequest {
    BuildTaprootTxRequest {
        session_id: session_id.to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: 10_000,
        }],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "22".repeat(32)),
            value_sats: 9_000,
        }],
        script_tree_hex: None,
    }
}

fn policy_bound_message_hex_from_tx_result(tx_result: &TransactionResult) -> String {
    let tx_bytes = hex::decode(&tx_result.tx_hex).expect("tx hex");
    hash_hex(&tx_bytes)
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
            },
            crate::api::TxInput {
                txid_hex: "22".repeat(32),
                vout: 0,
                value_sats: 1,
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

    let run_dkg_err = run_dkg(RunDkgRequest {
        session_id: "session-metrics-provenance-run-dkg".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected run_dkg provenance gate rejection");
    assert!(matches!(
        run_dkg_err,
        EngineError::ProvenanceGateRejected { .. }
    ));

    let build_tx_err = build_taproot_tx(BuildTaprootTxRequest {
        session_id: "session-metrics-provenance-build-tx".to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: 10_000,
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

    let finalize_err = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: "session-metrics-provenance-finalize".to_string(),
            taproot_merkle_root_hex: None,
            round_contributions: vec![],
            attempt_context: None,
        },
        true,
    )
    .expect_err("expected finalize_sign_round provenance gate rejection");
    assert!(matches!(
        finalize_err,
        EngineError::ProvenanceGateRejected { .. }
    ));

    let metrics = hardening_metrics();
    assert_eq!(metrics.run_dkg_calls_total, 1);
    assert_eq!(metrics.start_sign_round_calls_total, 0);
    assert_eq!(metrics.build_taproot_tx_calls_total, 1);
    assert_eq!(metrics.finalize_sign_round_calls_total, 1);
    assert_eq!(metrics.refresh_shares_calls_total, 0);
    assert_eq!(metrics.run_dkg_success_total, 0);
    assert_eq!(metrics.start_sign_round_success_total, 0);
    assert_eq!(metrics.build_taproot_tx_success_total, 0);
    assert_eq!(metrics.finalize_sign_round_success_total, 0);
    assert_eq!(metrics.refresh_shares_success_total, 0);

    clear_state_storage_policy_overrides();
}

#[test]
fn hardening_metrics_track_start_sign_round_and_refresh_shares_counters() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: "session-metrics-start-refresh".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let _ = start_sign_round(StartSignRoundRequest {
        session_id: "session-metrics-start-refresh".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect("start sign round");

    let _ = refresh_shares(RefreshSharesRequest {
        session_id: "session-metrics-refresh-only".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "aaaa".to_string(),
        }],
    })
    .expect("refresh shares");

    let metrics = hardening_metrics();
    assert_eq!(metrics.start_sign_round_calls_total, 1);
    assert_eq!(metrics.start_sign_round_success_total, 1);
    assert_eq!(metrics.refresh_shares_calls_total, 1);
    assert_eq!(metrics.refresh_shares_success_total, 1);

    clear_state_storage_policy_overrides();
}

#[test]
fn roast_transcript_audit_and_verify_blame_proof_roundtrip() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-transcript-audit-roundtrip";
    let message_hex = "deadbeef";
    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2, 3]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round attempt 1");

    let mut transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
        reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
        excluded_member_identifiers: vec![3],
        invalid_share_proof_fingerprint: Some("ab".repeat(32)),
    });

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect("start sign round attempt 2");

    let audit = roast_transcript_audit(crate::api::TranscriptAuditRequest {
        session_id: session_id.to_string(),
    })
    .expect("transcript audit");
    assert_eq!(audit.transition_count, 1);
    assert_eq!(audit.records.len(), 1);
    let record = &audit.records[0];
    assert_eq!(record.from_attempt_number, 1);
    assert_eq!(record.to_attempt_number, 2);
    assert_eq!(record.reason, ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF);
    assert_eq!(record.excluded_member_identifiers, vec![3]);
    assert!(!record.transcript_hash.is_empty());

    let verified = verify_blame_proof(crate::api::VerifyBlameProofRequest {
        session_id: session_id.to_string(),
        from_attempt_number: 1,
        accused_member_identifier: 3,
        reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
        invalid_share_proof_fingerprint: Some("ab".repeat(32)),
    })
    .expect("verify blame proof");
    assert!(verified.verified);
    assert_eq!(
        verified.transcript_hash,
        Some(record.transcript_hash.clone())
    );

    let not_verified = verify_blame_proof(crate::api::VerifyBlameProofRequest {
        session_id: session_id.to_string(),
        from_attempt_number: 1,
        accused_member_identifier: 2,
        reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
        invalid_share_proof_fingerprint: Some("ab".repeat(32)),
    })
    .expect("verify blame proof mismatch");
    assert!(!not_verified.verified);

    let metrics = hardening_metrics();
    assert_eq!(metrics.roast_transcript_audit_calls_total, 1);
    assert_eq!(metrics.roast_transcript_audit_success_total, 1);
    assert_eq!(metrics.verify_blame_proof_calls_total, 2);
    assert_eq!(metrics.verify_blame_proof_success_total, 1);

    clear_state_storage_policy_overrides();
}

#[test]
fn roast_transcript_audit_records_persist_across_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("transcript_audit_persist_reload");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-transcript-audit-persist";
    let message_hex = "deadbeef";
    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2, 3]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round attempt 1");

    let mut transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
        reason: ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT.to_string(),
        excluded_member_identifiers: vec![],
        invalid_share_proof_fingerprint: None,
    });
    let attempt_two =
        build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2, 3]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2, 3]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect("start sign round attempt 2");

    reload_state_from_storage_for_tests();

    let audit = roast_transcript_audit(crate::api::TranscriptAuditRequest {
        session_id: session_id.to_string(),
    })
    .expect("transcript audit after reload");
    assert_eq!(audit.transition_count, 1);
    assert_eq!(audit.records.len(), 1);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn auto_quarantine_enforces_threshold_and_honors_dao_allowlist_override() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    std::env::set_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV, "true");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV, "2");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV, "1");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV, "2");

    let session_id = "session-auto-quarantine-threshold";
    let message_hex = "deadbeef";
    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2, 3]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round attempt 1");

    let mut transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
        reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
        excluded_member_identifiers: vec![3],
        invalid_share_proof_fingerprint: Some("cd".repeat(32)),
    });

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect("start sign round attempt 2");

    let status = quarantine_status(crate::api::QuarantineStatusRequest {
        operator_identifier: 3,
    })
    .expect("quarantine status");
    assert!(status.auto_quarantine_enabled);
    assert_eq!(status.fault_score, 2);
    assert_eq!(status.quarantine_threshold, 2);
    assert!(status.quarantined);
    assert!(!status.dao_override_allowlisted);

    let err = run_dkg(RunDkgRequest {
        session_id: "session-auto-quarantine-rejected".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected auto-quarantine rejection");
    let EngineError::QuarantinePolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "operator_auto_quarantined");

    std::env::set_var(
        TBTC_SIGNER_AUTO_QUARANTINE_DAO_ALLOWLIST_IDENTIFIERS_ENV,
        "3",
    );
    let allowlisted_status = quarantine_status(crate::api::QuarantineStatusRequest {
        operator_identifier: 3,
    })
    .expect("allowlisted quarantine status");
    assert!(allowlisted_status.dao_override_allowlisted);
    assert!(!allowlisted_status.quarantined);

    let _allowlisted_dkg = run_dkg(RunDkgRequest {
        session_id: "session-auto-quarantine-allowlisted".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("allowlisted operator should bypass quarantine rejection");

    let metrics = hardening_metrics();
    assert!(metrics.auto_quarantine_fault_events_total >= 1);
    assert!(metrics.auto_quarantine_enforcements_total >= 1);
    assert!(metrics.quarantined_operator_count >= 1);

    clear_state_storage_policy_overrides();
}

#[test]
fn auto_quarantine_persists_across_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("auto_quarantine_persist_reload");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    std::env::set_var(TBTC_SIGNER_ENABLE_AUTO_QUARANTINE_ENV, "true");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_FAULT_THRESHOLD_ENV, "2");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_TIMEOUT_PENALTY_ENV, "1");
    std::env::set_var(TBTC_SIGNER_AUTO_QUARANTINE_INVALID_SHARE_PENALTY_ENV, "2");

    let session_id = "session-auto-quarantine-persist-reload";
    let message_hex = "deadbeef";
    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2, 3]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round attempt 1");

    let mut transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
        reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
        excluded_member_identifiers: vec![3],
        invalid_share_proof_fingerprint: Some("ef".repeat(32)),
    });

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect("start sign round attempt 2");

    let status_before_reload = quarantine_status(crate::api::QuarantineStatusRequest {
        operator_identifier: 3,
    })
    .expect("quarantine status before reload");
    assert!(status_before_reload.quarantined);
    assert_eq!(status_before_reload.fault_score, 2);

    reload_state_from_storage_for_tests();

    let status_after_reload = quarantine_status(crate::api::QuarantineStatusRequest {
        operator_identifier: 3,
    })
    .expect("quarantine status after reload");
    assert!(status_after_reload.quarantined);
    assert_eq!(status_after_reload.fault_score, 2);

    let err = run_dkg(RunDkgRequest {
        session_id: "session-auto-quarantine-persist-reload-reject".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect_err("expected quarantine rejection after reload");
    let EngineError::QuarantinePolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "operator_auto_quarantined");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn refresh_cadence_status_tracks_overdue_and_emergency_rekey_persistence() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("refresh_cadence_status");
    reset_for_tests();
    clear_state_storage_policy_overrides();
    std::env::set_var(TBTC_SIGNER_REFRESH_CADENCE_SECONDS_ENV, "60");

    let session_id = "session-refresh-cadence";
    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let refresh_result = refresh_shares(RefreshSharesRequest {
        session_id: session_id.to_string(),
        current_shares: vec![
            ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "11".repeat(16),
            },
            ShareMaterial {
                identifier: 2,
                encrypted_share_hex: "22".repeat(16),
            },
        ],
    })
    .expect("refresh shares");
    let initial_status = refresh_cadence_status(RefreshCadenceStatusRequest {
        session_id: session_id.to_string(),
    })
    .expect("refresh cadence status");
    assert_eq!(initial_status.refresh_count, 1);
    assert_eq!(
        initial_status.last_refresh_epoch,
        refresh_result.refresh_epoch
    );
    assert_eq!(
        initial_status.continuity_reference_key_group,
        Some(dkg_result.key_group)
    );
    assert!(initial_status.continuity_preserved);
    assert!(!initial_status.overdue);
    assert!(!initial_status.emergency_rekey_required);

    {
        let state = state().expect("state initialization");
        let mut guard = state.lock().expect("engine lock");
        let session = guard.sessions.get_mut(session_id).expect("session state");
        let refresh_record = session
            .refresh_history
            .last_mut()
            .expect("refresh history entry");
        refresh_record.refreshed_at_unix = refresh_record.refreshed_at_unix.saturating_sub(600);
        persist_engine_state_to_storage(&guard).expect("persist stale refresh history");
    }

    let stale_status = refresh_cadence_status(RefreshCadenceStatusRequest {
        session_id: session_id.to_string(),
    })
    .expect("stale refresh cadence status");
    assert!(stale_status.overdue);

    trigger_emergency_rekey(TriggerEmergencyRekeyRequest {
        session_id: session_id.to_string(),
        reason: "key compromise drill".to_string(),
    })
    .expect("trigger emergency rekey");
    reload_state_from_storage_for_tests();

    let post_rekey_status = refresh_cadence_status(RefreshCadenceStatusRequest {
        session_id: session_id.to_string(),
    })
    .expect("refresh cadence status after rekey");
    assert!(post_rekey_status.emergency_rekey_required);
    assert_eq!(
        post_rekey_status.emergency_rekey_reason,
        Some("key compromise drill".to_string())
    );

    let start_err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: post_rekey_status
            .continuity_reference_key_group
            .expect("continuity reference key group"),
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect_err("expected start sign round emergency rekey rejection");
    let EngineError::LifecyclePolicyRejected { reason_code, .. } = start_err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "emergency_rekey_required");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
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
    assert_eq!(initial_status.recommended_next_percent, Some(50));

    let promoted_50 =
        promote_canary(PromoteCanaryRequest { target_percent: 50 }).expect("promote canary to 50%");
    assert_eq!(promoted_50.from_percent, 10);
    assert_eq!(promoted_50.to_percent, 50);

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
fn canary_promotion_halts_when_policy_reject_rate_exceeds_gate() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

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
fn emergency_rekey_blocks_finalize_and_build_taproot_tx_for_session() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let round_state = seeded_round_state("session-emergency-rekey-finalize");
    trigger_emergency_rekey(TriggerEmergencyRekeyRequest {
        session_id: round_state.session_id.clone(),
        reason: "compromise containment".to_string(),
    })
    .expect("trigger emergency rekey");

    let finalize_err = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: round_state.session_id.clone(),
            taproot_merkle_root_hex: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
            attempt_context: None,
        },
        true,
    )
    .expect_err("expected finalize emergency rekey rejection");
    let EngineError::LifecyclePolicyRejected { reason_code, .. } = finalize_err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "emergency_rekey_required");

    let build_err = build_taproot_tx(build_policy_test_request(&round_state.session_id))
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

#[test]
fn start_sign_round_signing_policy_firewall_rejects_without_policy_checked_build_tx() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-signing-policy-start-missing-build-tx";
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect_err("expected signing policy reject without build tx binding");
    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "missing_policy_checked_build_tx");

    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_signing_policy_firewall_rejects_message_not_bound_to_build_tx() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-signing-policy-start-message-mismatch";
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    build_taproot_tx(build_policy_test_request(session_id)).expect("build tx");

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect_err("expected signing policy reject for message mismatch");
    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(
        reason_code,
        "signing_message_not_bound_to_policy_checked_build_tx"
    );

    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_signing_policy_firewall_accepts_policy_bound_message() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-signing-policy-start-bound-message";
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let tx_result = build_taproot_tx(build_policy_test_request(session_id)).expect("build tx");
    let message_hex = policy_bound_message_hex_from_tx_result(&tx_result);

    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex,
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect("expected start_sign_round allow for policy-bound message");
    assert_eq!(round_state.session_id, session_id);

    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_signing_policy_firewall_rejects_missing_policy_checked_build_tx() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-signing-policy-finalize-missing-build-tx";
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let tx_result = build_taproot_tx(build_policy_test_request(session_id)).expect("build tx");
    let message_hex = policy_bound_message_hex_from_tx_result(&tx_result);
    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex,
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect("start sign round");

    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut(session_id)
            .expect("session should exist");
        session.tx_result = None;
        session.build_tx_request_fingerprint = None;
    }

    let err = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: session_id.to_string(),
            taproot_merkle_root_hex: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
            attempt_context: None,
        },
        true,
    )
    .expect_err("expected finalize reject without policy-checked build tx");
    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(reason_code, "missing_policy_checked_build_tx");

    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_signing_policy_firewall_rejects_message_mismatch_after_tx_result_swap() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-signing-policy-finalize-tx-result-swap";
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let tx_result = build_taproot_tx(build_policy_test_request(session_id)).expect("build tx");
    let message_hex = policy_bound_message_hex_from_tx_result(&tx_result);
    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex,
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect("start sign round");

    {
        let mut guard = state().expect("state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut(session_id)
            .expect("session should exist");
        session.tx_result = Some(TransactionResult {
            session_id: session_id.to_string(),
            tx_hex: "00".to_string(),
        });
    }

    let err = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: session_id.to_string(),
            taproot_merkle_root_hex: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
            attempt_context: None,
        },
        true,
    )
    .expect_err("expected finalize reject for tx_result swap");
    let EngineError::SigningPolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(
        reason_code,
        "signing_message_not_bound_to_policy_checked_build_tx"
    );

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
                .and_then(|session| session.dkg_result.as_ref())
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

fn build_attempt_transition_evidence_from_active_session(
    session_id: &str,
) -> AttemptTransitionEvidence {
    let guard = state()
        .expect("engine state")
        .lock()
        .expect("engine state lock");
    let session = guard
        .sessions
        .get(session_id)
        .expect("session should exist for transition evidence");
    let active_attempt_context = session
        .active_attempt_context
        .as_ref()
        .expect("active attempt context should exist");
    let round_state = session
        .round_state
        .as_ref()
        .expect("round state should exist for transition evidence");
    let sign_request_fingerprint = session
        .sign_request_fingerprint
        .as_ref()
        .expect("sign request fingerprint should exist");

    AttemptTransitionEvidence {
        from_attempt_number: active_attempt_context.attempt_number,
        from_attempt_id: active_attempt_context.attempt_id.clone(),
        from_coordinator_identifier: active_attempt_context.coordinator_identifier,
        previous_round_id: round_state.round_id.clone(),
        previous_sign_request_fingerprint: sign_request_fingerprint.clone(),
        exclusion_evidence: Some(AttemptExclusionEvidence {
            reason: ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT.to_string(),
            excluded_member_identifiers: vec![],
            invalid_share_proof_fingerprint: None,
        }),
    }
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
        let encoded =
            encode_encrypted_state_envelope(&persisted).expect("state envelope encode");
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

#[test]
fn formal_verification_derive_round_id_binds_attempt_id_case_insensitive_component() {
    let request_session_id = "round-id-attempt-case-session";
    let key_group = "key-group";
    let message_hex = "deadbeef";
    let signing_participants_fingerprint = "participants-fingerprint";

    let lowercase_attempt_context = AttemptContext {
        attempt_number: 1,
        coordinator_identifier: 1,
        included_participants: vec![1, 2],
        included_participants_fingerprint: "aa".repeat(32),
        attempt_id: "ab".repeat(32),
    };
    let uppercase_attempt_context = AttemptContext {
        attempt_id: lowercase_attempt_context.attempt_id.to_ascii_uppercase(),
        ..lowercase_attempt_context.clone()
    };

    let round_id_lowercase_attempt = derive_round_id(
        request_session_id,
        key_group,
        message_hex,
        None,
        signing_participants_fingerprint,
        Some(&lowercase_attempt_context),
    );
    let round_id_uppercase_attempt = derive_round_id(
        request_session_id,
        key_group,
        message_hex,
        None,
        signing_participants_fingerprint,
        Some(&uppercase_attempt_context),
    );
    assert_eq!(round_id_lowercase_attempt, round_id_uppercase_attempt);

    let different_attempt_context = AttemptContext {
        attempt_id: "cd".repeat(32),
        ..lowercase_attempt_context.clone()
    };
    let round_id_different_attempt = derive_round_id(
        request_session_id,
        key_group,
        message_hex,
        None,
        signing_participants_fingerprint,
        Some(&different_attempt_context),
    );
    assert_ne!(round_id_lowercase_attempt, round_id_different_attempt);

    let round_id_without_attempt = derive_round_id(
        request_session_id,
        key_group,
        message_hex,
        None,
        signing_participants_fingerprint,
        None,
    );
    assert_ne!(round_id_lowercase_attempt, round_id_without_attempt);
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

    fn enable() -> Self {
        Self::set(Some("true"))
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
fn start_sign_round_rejects_missing_attempt_context_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: "session-roast-strict-start-missing-attempt-context".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let err = start_sign_round(StartSignRoundRequest {
        session_id: "session-roast-strict-start-missing-attempt-context".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect_err("expected attempt context validation");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("attempt_context is required"),
        "unexpected validation message: {message}"
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
fn start_sign_round_rejects_transitional_signing_in_production_profile() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("production_rejects_transitional_signing");
    reset_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        format!("printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
    );

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: "session-production-rejects-transitional".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("seed non-production dkg");

    // RAII guards restore the prior env on Drop so a panic or early return
    // does not leak production-profile state into subsequent tests.
    //
    // This is the state-smuggling scenario: the dealer session above was
    // created under the development profile, and the process now runs as
    // production. The deterministic-nonce signing entry point itself must
    // reject, even with the strict-mode env flag explicitly disabled.
    configure_valid_provenance_attestation_for_tests();
    let _signer_profile = SignerProfileGuard::production();
    let _roast_strict_mode = RoastStrictModeGuard::set(Some("false"));

    let err = start_sign_round(StartSignRoundRequest {
        session_id: "session-production-rejects-transitional".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect_err("production profile should reject transitional signing");

    let EngineError::LifecyclePolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(
        reason_code,
        "transitional_deterministic_signing_disabled_in_production"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_rejects_transitional_signing_in_production_profile() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("production_rejects_transitional_finalize");
    reset_for_tests();
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV,
        TBTC_SIGNER_STATE_KEY_PROVIDER_COMMAND,
    );
    std::env::set_var(
        TBTC_SIGNER_STATE_KEY_COMMAND_ENV,
        format!("printf '{}\\n'", TEST_STATE_ENCRYPTION_KEY_HEX),
    );

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: "session-production-rejects-transitional-finalize".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("seed non-production dkg");

    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: "session-production-rejects-transitional-finalize".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect("start sign round under development profile");

    // A round started under the development profile must not be
    // finalizable by a production-profile process either; the gate fires
    // before any round state is consumed.
    configure_valid_provenance_attestation_for_tests();
    let _signer_profile = SignerProfileGuard::production();

    let err = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: "session-production-rejects-transitional-finalize".to_string(),
            taproot_merkle_root_hex: None,
            attempt_context: None,
            round_contributions: vec![round_state.own_contribution.clone()],
        },
        false,
    )
    .expect_err("production profile should reject transitional finalize");

    let EngineError::LifecyclePolicyRejected { reason_code, .. } = err else {
        panic!("unexpected error variant");
    };
    assert_eq!(
        reason_code,
        "transitional_deterministic_signing_disabled_in_production"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_accepts_valid_attempt_context_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-start-valid-attempt-context";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect("start sign round");

    assert_eq!(round_state.required_contributions, 2);
}

#[test]
fn start_sign_round_rejects_invalid_attempt_context_fingerprint_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-start-invalid-attempt-context-fingerprint";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let mut attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    attempt_context.included_participants_fingerprint = "00".repeat(32);

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect_err("expected attempt context fingerprint validation");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("included_participants_fingerprint"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn start_sign_round_rejects_invalid_attempt_context_attempt_id_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-start-invalid-attempt-id";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let mut attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    attempt_context.attempt_id = "11".repeat(32);

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect_err("expected attempt context attempt-id validation");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("attempt_context.attempt_id"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn start_sign_round_rejects_attempt_number_zero_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-start-attempt-number-zero";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let mut attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    attempt_context.attempt_number = 0;

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect_err("expected attempt number validation");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("attempt_context.attempt_number"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn start_sign_round_rejects_zero_coordinator_identifier_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-start-coordinator-zero";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let mut attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    attempt_context.coordinator_identifier = 0;

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect_err("expected coordinator identifier validation");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("attempt_context.coordinator_identifier"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn start_sign_round_rejects_nondeterministic_coordinator_identifier_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-start-coordinator-nondeterministic";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let deterministic_attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let mismatched_coordinator_identifier =
        if deterministic_attempt_context.coordinator_identifier == 1 {
            2
        } else {
            1
        };
    let invalid_attempt_context = build_attempt_context(
        session_id,
        message_hex,
        1,
        mismatched_coordinator_identifier,
        vec![1, 2],
    );

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(invalid_attempt_context),
        attempt_transition_evidence: None,
    })
    .expect_err("expected deterministic coordinator validation");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("deterministic coordinator"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn start_sign_round_rejects_sub_threshold_attempt_participants_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-start-sub-threshold-attempt-participants";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_context = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1]);

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect_err("expected attempt participants threshold validation");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("at least threshold members"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn start_sign_round_rejects_duplicate_attempt_participants_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-start-duplicate-attempt-participants";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_context = AttemptContext {
        attempt_number: 1,
        coordinator_identifier: 1,
        included_participants: vec![1, 1, 2],
        included_participants_fingerprint: "00".repeat(32),
        attempt_id: "11".repeat(32),
    };

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect_err("expected duplicate attempt participant validation");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("duplicate identifier"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn start_sign_round_accepts_hex_case_variant_attempt_context_idempotent_retry() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-start-case-variant-idempotency";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let mut uppercase_attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
    uppercase_attempt_context.included_participants_fingerprint = uppercase_attempt_context
        .included_participants_fingerprint
        .to_ascii_uppercase();
    uppercase_attempt_context.attempt_id =
        uppercase_attempt_context.attempt_id.to_ascii_uppercase();

    let first_round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(uppercase_attempt_context),
        attempt_transition_evidence: None,
    })
    .expect("first start sign round");

    let lowercase_attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let second_round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![2, 1]),
        attempt_context: Some(lowercase_attempt_context),
        attempt_transition_evidence: None,
    })
    .expect("second start sign round retry");

    assert_eq!(first_round_state, second_round_state);
}

#[test]
fn finalize_sign_round_rejects_missing_attempt_context_in_roast_strict_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-strict-finalize-missing-attempt-context";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect("start sign round");

    let err = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: session_id.to_string(),
            taproot_merkle_root_hex: None,
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
        },
        true,
    )
    .expect_err("expected attempt context validation");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("attempt_context is required"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn finalize_sign_round_accepts_missing_attempt_context_when_not_strict_with_active_attempt_context()
{
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-roast-phase2-nonstrict-finalize-missing-attempt-context";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect("start sign round");

    let signature_result = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: session_id.to_string(),
            taproot_merkle_root_hex: None,
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
        },
        true,
    )
    .expect("finalize without attempt context in non-strict mode");

    assert_eq!(signature_result.round_id, round_state.round_id);
    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_accepts_missing_attempt_context_after_reload_when_not_strict() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("phase2_nonstrict_finalize_missing_after_reload");
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-roast-phase2-nonstrict-finalize-reload";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect("start sign round");

    reload_state_from_storage_for_tests();

    let signature_result = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: session_id.to_string(),
            taproot_merkle_root_hex: None,
            attempt_context: None,
            round_contributions: vec![
                RoundContribution {
                    identifier: 1,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
                },
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
        },
        true,
    )
    .expect("finalize without attempt context after reload in non-strict mode");

    assert_eq!(signature_result.round_id, round_state.round_id);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_returns_session_conflict_for_attempt_context_presence_mismatch_in_non_strict_mode(
) {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "session-roast-phase2-nonstrict-start-presence-mismatch";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    })
    .expect("start sign round with attempt context");

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    })
    .expect_err("expected session conflict on payload mismatch");

    assert!(matches!(err, EngineError::SessionConflict { .. }));
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_rejects_stale_attempt_number_against_active_attempt_context() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase2-stale-start-attempt";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 2");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect_err("expected stale attempt rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("stale"),
        "expected stale-attempt validation message, got: {message}"
    );
}

#[test]
fn start_sign_round_rejects_future_attempt_number_without_transition_authorization() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase2-future-start-attempt";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: None,
    })
    .expect_err("expected future attempt rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("attempt_transition_evidence"),
        "expected future-attempt validation message, got: {message}"
    );
}

#[test]
fn start_sign_round_allows_next_attempt_with_valid_transition_evidence() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase2-transition-evidence-valid";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let round_state_one = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let round_state_two = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect("start sign round for authorized attempt 2");

    assert_ne!(round_state_one.round_id, round_state_two.round_id);
    let transition_telemetry = round_state_two
        .attempt_transition_telemetry
        .expect("attempt transition telemetry");
    assert_eq!(transition_telemetry.from_attempt_number, 1);
    assert_eq!(transition_telemetry.to_attempt_number, 2);
    assert_eq!(
        transition_telemetry.reason,
        ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT
    );
    assert!(transition_telemetry.excluded_member_identifiers.is_empty());

    let stale_attempt = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(stale_attempt),
        attempt_transition_evidence: None,
    })
    .expect_err("expected stale rejection after authorized advancement");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("stale"),
        "expected stale-attempt validation message, got: {message}"
    );
}

#[test]
fn start_sign_round_allows_member_reuse_after_transition_without_resending_evidence() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-transition-reuse-without-evidence";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let transitioned_round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two.clone()),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect("start sign round for authorized attempt 2");

    let reused_round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 2,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: None,
    })
    .expect("reuse active attempt without transition evidence");

    assert_eq!(
        transitioned_round_state.round_id,
        reused_round_state.round_id
    );
    assert_eq!(transitioned_round_state.required_contributions, 2);
    assert_eq!(reused_round_state.required_contributions, 2);
    assert_eq!(transitioned_round_state.own_contribution.identifier, 1);
    assert_eq!(reused_round_state.own_contribution.identifier, 2);
    assert_ne!(
        transitioned_round_state
            .own_contribution
            .signature_share_hex,
        reused_round_state.own_contribution.signature_share_hex
    );
}

#[test]
fn start_sign_round_allows_next_attempt_with_valid_transition_evidence_after_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("phase2_transition_evidence_valid_reload");
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase2-transition-evidence-valid-reload";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let round_state_one = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    reload_state_from_storage_for_tests();

    let transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let round_state_two = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect("start sign round for authorized attempt 2 after reload");

    assert_ne!(round_state_one.round_id, round_state_two.round_id);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_rejects_stale_attempt_after_authorized_transition_across_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("phase2_transition_stale_after_reload");
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase2-transition-stale-after-reload";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one.clone()),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect("start sign round for authorized attempt 2");

    reload_state_from_storage_for_tests();

    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect_err("expected stale attempt rejection after reload");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("stale"),
        "expected stale-attempt validation message, got: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_rejects_next_attempt_with_invalid_transition_evidence() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase2-transition-evidence-invalid";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let mut invalid_transition_evidence =
        build_attempt_transition_evidence_from_active_session(session_id);
    invalid_transition_evidence.previous_round_id = "invalid-round-id".to_string();

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(invalid_transition_evidence),
    })
    .expect_err("expected invalid transition evidence rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("previous_round_id"),
        "expected transition-evidence previous_round_id validation message, got: {message}"
    );
}

#[test]
fn start_sign_round_rejects_far_future_attempt_even_with_transition_evidence() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase2-transition-evidence-far-future";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    let attempt_three = build_deterministic_attempt_context(session_id, message_hex, 3, vec![1, 2]);
    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_three),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect_err("expected far-future attempt rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("ahead of active attempt_number"),
        "expected far-future validation message, got: {message}"
    );
}

#[test]
fn start_sign_round_rejects_next_attempt_without_exclusion_evidence() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase4-transition-missing-exclusion-evidence";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let mut transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    transition_evidence.exclusion_evidence = None;

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect_err("expected missing exclusion evidence rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("exclusion_evidence"),
        "expected exclusion-evidence validation message, got: {message}"
    );
}

#[test]
fn start_sign_round_rejects_timeout_reason_with_invalid_share_fingerprint() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase4-timeout-reason-fingerprint-rejection";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let mut transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
        reason: ROAST_EXCLUSION_REASON_COORDINATOR_TIMEOUT.to_string(),
        excluded_member_identifiers: vec![],
        invalid_share_proof_fingerprint: Some("ab".repeat(32)),
    });

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect_err("expected timeout-reason proof fingerprint rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("must be omitted"),
        "expected timeout-reason proof-fingerprint validation message, got: {message}"
    );
}

#[test]
fn start_sign_round_accepts_invalid_share_proof_exclusion_evidence() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase4-invalid-share-proof-evidence-valid";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2, 3]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let mut transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
        reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
        excluded_member_identifiers: vec![3],
        invalid_share_proof_fingerprint: Some("ab".repeat(32)),
    });

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let round_state_two = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect("start sign round for attempt 2 with invalid-share-proof evidence");

    assert_eq!(round_state_two.required_contributions, 2);
    let transition_telemetry = round_state_two
        .attempt_transition_telemetry
        .expect("attempt transition telemetry");
    assert_eq!(transition_telemetry.from_attempt_number, 1);
    assert_eq!(transition_telemetry.to_attempt_number, 2);
    assert_eq!(
        transition_telemetry.reason,
        ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF
    );
    assert_eq!(transition_telemetry.excluded_member_identifiers, vec![3]);
}

#[test]
fn start_sign_round_rejects_invalid_share_proof_without_fingerprint() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase4-invalid-share-proof-fingerprint-required";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2, 3]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let mut transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
        reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
        excluded_member_identifiers: vec![3],
        invalid_share_proof_fingerprint: None,
    });

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect_err("expected invalid-share-proof fingerprint required rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("invalid_share_proof_fingerprint is required"),
        "expected invalid-share-proof fingerprint-required message, got: {message}"
    );
}

#[test]
fn start_sign_round_rejects_invalid_share_proof_with_empty_fingerprint() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase4-invalid-share-proof-empty-fingerprint";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let attempt_one =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2, 3]);
    start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2, 3]),
        attempt_context: Some(attempt_one),
        attempt_transition_evidence: None,
    })
    .expect("start sign round for attempt 1");

    let mut transition_evidence = build_attempt_transition_evidence_from_active_session(session_id);
    transition_evidence.exclusion_evidence = Some(AttemptExclusionEvidence {
        reason: ROAST_EXCLUSION_REASON_INVALID_SHARE_PROOF.to_string(),
        excluded_member_identifiers: vec![3],
        invalid_share_proof_fingerprint: Some("   ".to_string()),
    });

    let attempt_two = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let err = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_two),
        attempt_transition_evidence: Some(transition_evidence),
    })
    .expect_err("expected invalid-share-proof empty-fingerprint rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("must be non-empty valid hex"),
        "expected invalid-share-proof empty-fingerprint message, got: {message}"
    );
}

#[test]
fn finalize_sign_round_rejects_coordinator_mismatch_against_active_attempt_context() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase2-finalize-coordinator-mismatch";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let start_attempt = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(start_attempt),
        attempt_transition_evidence: None,
    })
    .expect("start sign round");

    // Pick the member that is provably not the deterministic
    // coordinator so the test stays valid under any seed derivation.
    let deterministic_attempt =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let mismatched_coordinator = if deterministic_attempt.coordinator_identifier == 1 {
        2
    } else {
        1
    };
    let mismatched_attempt = build_attempt_context(
        session_id,
        message_hex,
        1,
        mismatched_coordinator,
        vec![1, 2],
    );
    let err = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: session_id.to_string(),
            taproot_merkle_root_hex: None,
            attempt_context: Some(mismatched_attempt),
            round_contributions: vec![
                round_state.own_contribution.clone(),
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
        },
        true,
    )
    .expect_err("expected coordinator mismatch rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("coordinator_identifier"),
        "expected coordinator mismatch validation message, got: {message}"
    );
}

#[test]
fn finalize_sign_round_rejects_stale_attempt_number_against_active_attempt_context() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-roast-phase2-finalize-stale-attempt";
    let message_hex = "deadbeef";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let start_attempt = build_deterministic_attempt_context(session_id, message_hex, 2, vec![1, 2]);
    let round_state = start_sign_round(StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(start_attempt),
        attempt_transition_evidence: None,
    })
    .expect("start sign round");

    let stale_attempt = build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    let err = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: session_id.to_string(),
            taproot_merkle_root_hex: None,
            attempt_context: Some(stale_attempt),
            round_contributions: vec![
                round_state.own_contribution.clone(),
                RoundContribution {
                    identifier: 2,
                    signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
                },
            ],
        },
        true,
    )
    .expect_err("expected stale attempt rejection");

    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("stale"),
        "expected stale-attempt validation message, got: {message}"
    );
}

#[test]
fn finalize_rejects_bootstrap_synthetic_contributions_outside_bootstrap_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let round_state = seeded_round_state("session-synthetic-rejected");

    let request = FinalizeSignRoundRequest {
        session_id: "session-synthetic-rejected".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };

    let err = finalize_sign_round(request, false).expect_err("expected synthetic rejection");
    assert!(matches!(
        err,
        EngineError::SyntheticContributionRejected { .. }
    ));
}

#[test]
fn finalize_accepts_bootstrap_synthetic_contributions_in_bootstrap_mode() {
    let _guard = lock_test_state();
    reset_for_tests();
    let round_state = seeded_round_state("session-synthetic-accepted");

    let request = FinalizeSignRoundRequest {
        session_id: "session-synthetic-accepted".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };

    let result =
        finalize_sign_round(request, true).expect("expected bootstrap synthetic acceptance");
    assert_eq!(result.round_id, round_state.round_id);
}

#[test]
fn finalize_aggregates_real_contributions_outside_bootstrap_mode() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-real-finalize".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-real-finalize".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request.clone()).expect("start sign round");
    let signing_participants = round_state
        .signing_participants
        .clone()
        .expect("round signing participants");

    let (dkg_key_packages, dkg_public_key_package, sign_message_bytes) = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get(&start_request.session_id)
            .expect("session state");

        (
            session.dkg_key_packages.clone().expect("dkg key packages"),
            session
                .dkg_public_key_package
                .clone()
                .expect("dkg public key package"),
            session
                .sign_message_bytes
                .clone()
                .expect("sign message bytes"),
        )
    };

    let member_two_request = StartSignRoundRequest {
        member_identifier: 2,
        attempt_transition_evidence: None,
        ..start_request
    };
    let member_two_contribution = build_real_signature_share_contribution(
        &dkg_key_packages,
        &dkg_public_key_package,
        &signing_participants,
        &member_two_request,
        &round_state.round_id,
        &hex::decode(&member_two_request.message_hex).expect("message decode"),
        None,
    )
    .expect("member two contribution");
    let member_three_request = StartSignRoundRequest {
        member_identifier: 3,
        attempt_transition_evidence: None,
        ..member_two_request.clone()
    };
    let member_three_contribution = build_real_signature_share_contribution(
        &dkg_key_packages,
        &dkg_public_key_package,
        &signing_participants,
        &member_three_request,
        &round_state.round_id,
        &hex::decode(&member_three_request.message_hex).expect("message decode"),
        None,
    )
    .expect("member three contribution");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-real-finalize".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            round_state.own_contribution.clone(),
            member_two_contribution,
            member_three_contribution,
        ],
    };

    let first_result = finalize_sign_round(finalize_request.clone(), false).expect("finalize");
    let second_result = finalize_sign_round(finalize_request, false).expect("finalize retry");

    assert_eq!(first_result, second_result);
    assert_eq!(first_result.round_id, round_state.round_id);
    let signature_bytes = hex::decode(&first_result.signature_hex).expect("signature decode");
    assert_eq!(signature_bytes.len(), 64);
    let signature = frost::Signature::deserialize(&signature_bytes).expect("signature parse");
    let exported_key_group_bytes =
        hex::decode(&dkg_result.key_group).expect("decode exported key group");
    let exported_verifying_key = frost::VerifyingKey::deserialize(&exported_key_group_bytes)
        .expect("deserialize exported key group");
    assert_eq!(
        dkg_result.key_group,
        hex::encode(
            dkg_public_key_package
                .verifying_key()
                .serialize()
                .expect("serialize DKG verifying key")
        )
    );
    dkg_public_key_package
        .verifying_key()
        .verify(&sign_message_bytes, &signature)
        .expect("signature verification");
    exported_verifying_key
        .verify(&sign_message_bytes, &signature)
        .expect("signature verifies under exported key group");
    assert!(
        dkg_public_key_package
            .clone()
            .tweak::<&[u8]>(None)
            .verifying_key()
            .verify(&sign_message_bytes, &signature)
            .is_err(),
        "no-root signature must not verify under an additional BIP-86 empty-root tweak"
    );
}

#[test]
fn finalize_aggregates_real_taproot_tweaked_contributions() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-real-taproot-tweak".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let taproot_merkle_root_hex =
        "37a57b86de2819d2b72a173df46238a7ad295ea1485d3b40e9415daa82b4fdcb";
    let taproot_merkle_root_bytes =
        hex::decode(taproot_merkle_root_hex).expect("taproot merkle root");
    let mut taproot_merkle_root = [0_u8; 32];
    taproot_merkle_root.copy_from_slice(&taproot_merkle_root_bytes);

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-real-taproot-tweak".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: Some(taproot_merkle_root_hex.to_string()),
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request.clone()).expect("start sign round");
    assert_eq!(
        round_state.taproot_merkle_root_hex.as_deref(),
        Some(taproot_merkle_root_hex)
    );
    let signing_participants = round_state
        .signing_participants
        .clone()
        .expect("round signing participants");

    let (dkg_key_packages, dkg_public_key_package, sign_message_bytes) = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get(&start_request.session_id)
            .expect("session state");

        (
            session.dkg_key_packages.clone().expect("dkg key packages"),
            session
                .dkg_public_key_package
                .clone()
                .expect("dkg public key package"),
            session
                .sign_message_bytes
                .clone()
                .expect("sign message bytes"),
        )
    };

    let member_two_request = StartSignRoundRequest {
        member_identifier: 2,
        attempt_transition_evidence: None,
        ..start_request.clone()
    };
    let member_two_contribution = build_real_signature_share_contribution(
        &dkg_key_packages,
        &dkg_public_key_package,
        &signing_participants,
        &member_two_request,
        &round_state.round_id,
        &hex::decode(&member_two_request.message_hex).expect("message decode"),
        Some(&taproot_merkle_root),
    )
    .expect("member two contribution");
    let member_three_request = StartSignRoundRequest {
        member_identifier: 3,
        attempt_transition_evidence: None,
        ..member_two_request.clone()
    };
    let member_three_contribution = build_real_signature_share_contribution(
        &dkg_key_packages,
        &dkg_public_key_package,
        &signing_participants,
        &member_three_request,
        &round_state.round_id,
        &hex::decode(&member_three_request.message_hex).expect("message decode"),
        Some(&taproot_merkle_root),
    )
    .expect("member three contribution");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-real-taproot-tweak".to_string(),
        taproot_merkle_root_hex: Some(taproot_merkle_root_hex.to_string()),
        attempt_context: None,
        round_contributions: vec![
            round_state.own_contribution.clone(),
            member_two_contribution,
            member_three_contribution,
        ],
    };

    let result = finalize_sign_round(finalize_request, false).expect("finalize");

    assert_eq!(result.round_id, round_state.round_id);
    let signature_bytes = hex::decode(&result.signature_hex).expect("signature decode");
    assert_eq!(signature_bytes.len(), 64);
    let signature = frost::Signature::deserialize(&signature_bytes).expect("signature parse");
    let exported_key_group_bytes =
        hex::decode(&dkg_result.key_group).expect("decode exported key group");
    let exported_verifying_key = frost::VerifyingKey::deserialize(&exported_key_group_bytes)
        .expect("deserialize exported key group");
    let exported_public_key_package = frost::keys::PublicKeyPackage::new(
        BTreeMap::<frost::Identifier, frost::keys::VerifyingShare>::new(),
        exported_verifying_key,
        Some(dkg_result.threshold),
    );
    assert_eq!(
        dkg_result.key_group,
        hex::encode(
            dkg_public_key_package
                .verifying_key()
                .serialize()
                .expect("serialize DKG verifying key")
        )
    );
    let tweaked_public_key_package = dkg_public_key_package
        .clone()
        .tweak(Some(taproot_merkle_root.as_slice()));
    tweaked_public_key_package
        .verifying_key()
        .verify(&sign_message_bytes, &signature)
        .expect("tweaked signature verification");
    exported_public_key_package
        .tweak(Some(taproot_merkle_root.as_slice()))
        .verifying_key()
        .verify(&sign_message_bytes, &signature)
        .expect("tweaked signature verifies under exported key group");
    assert!(
        dkg_public_key_package
            .verifying_key()
            .verify(&sign_message_bytes, &signature)
            .is_err(),
        "tweaked signature must not verify under the untweaked key"
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
fn finalize_aggregates_real_threshold_subset_outside_bootstrap_mode() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-real-threshold-subset".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-real-threshold-subset".to_string(),
        member_identifier: 1,
        message_hex: "cafef00d".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request.clone()).expect("start sign round");
    let signing_participants = round_state
        .signing_participants
        .clone()
        .expect("round signing participants");

    let (dkg_key_packages, dkg_public_key_package, sign_message_bytes) = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get(&start_request.session_id)
            .expect("session state");

        (
            session.dkg_key_packages.clone().expect("dkg key packages"),
            session
                .dkg_public_key_package
                .clone()
                .expect("dkg public key package"),
            session
                .sign_message_bytes
                .clone()
                .expect("sign message bytes"),
        )
    };

    let member_two_request = StartSignRoundRequest {
        member_identifier: 2,
        attempt_transition_evidence: None,
        ..start_request
    };
    let member_two_contribution = build_real_signature_share_contribution(
        &dkg_key_packages,
        &dkg_public_key_package,
        &signing_participants,
        &member_two_request,
        &round_state.round_id,
        &hex::decode(&member_two_request.message_hex).expect("message decode"),
        None,
    )
    .expect("member two contribution");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-real-threshold-subset".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            round_state.own_contribution.clone(),
            member_two_contribution,
        ],
    };

    let first_result = finalize_sign_round(finalize_request.clone(), false).expect("finalize");
    let second_result = finalize_sign_round(finalize_request, false).expect("finalize retry");

    assert_eq!(first_result, second_result);
    assert_eq!(first_result.round_id, round_state.round_id);
    let signature_bytes = hex::decode(&first_result.signature_hex).expect("signature decode");
    assert_eq!(signature_bytes.len(), 64);
    let signature = frost::Signature::deserialize(&signature_bytes).expect("signature parse");
    dkg_public_key_package
        .verifying_key()
        .verify(&sign_message_bytes, &signature)
        .expect("signature verification");
}

#[test]
fn start_sign_round_allows_distinct_members_for_same_active_round() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-real-multi-member-process".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-real-multi-member-process".to_string(),
        member_identifier: 1,
        message_hex: "baddcafe".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let first_round_state =
        start_sign_round(start_request.clone()).expect("first member start sign round");

    let second_round_state = start_sign_round(StartSignRoundRequest {
        member_identifier: 2,
        ..start_request.clone()
    })
    .expect("second member start sign round");

    assert_eq!(first_round_state.session_id, second_round_state.session_id);
    assert_eq!(first_round_state.round_id, second_round_state.round_id);
    assert_eq!(first_round_state.required_contributions, 2);
    assert_eq!(second_round_state.required_contributions, 2);
    assert_eq!(first_round_state.own_contribution.identifier, 1);
    assert_eq!(second_round_state.own_contribution.identifier, 2);
    assert_ne!(
        first_round_state.own_contribution.signature_share_hex,
        second_round_state.own_contribution.signature_share_hex
    );

    let (dkg_public_key_package, sign_message_bytes) = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get(&start_request.session_id)
            .expect("session state");

        (
            session
                .dkg_public_key_package
                .clone()
                .expect("dkg public key package"),
            session
                .sign_message_bytes
                .clone()
                .expect("sign message bytes"),
        )
    };

    let finalize_request = FinalizeSignRoundRequest {
        session_id: start_request.session_id,
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            first_round_state.own_contribution,
            second_round_state.own_contribution,
        ],
    };

    let result = finalize_sign_round(finalize_request, false).expect("finalize");

    assert_eq!(result.round_id, first_round_state.round_id);
    let signature_bytes = hex::decode(&result.signature_hex).expect("signature decode");
    let signature = frost::Signature::deserialize(&signature_bytes).expect("signature parse");
    dkg_public_key_package
        .verifying_key()
        .verify(&sign_message_bytes, &signature)
        .expect("signature verification");
}

#[test]
fn start_sign_round_allows_taproot_threshold_subset_members_for_same_active_round() {
    let _guard = lock_test_state();
    reset_for_tests();

    let participants = (1_u16..=100)
        .map(|identifier| crate::api::DkgParticipant {
            identifier,
            public_key_hex: format!("02{identifier:02x}"),
        })
        .collect::<Vec<_>>();
    let signing_participants = vec![
        2, 3, 4, 8, 11, 13, 14, 17, 19, 21, 22, 25, 27, 29, 30, 31, 32, 33, 35, 37, 38, 39, 42, 44,
        45, 48, 50, 51, 52, 53, 57, 58, 60, 61, 63, 64, 65, 67, 68, 73, 76, 77, 80, 81, 84, 86, 87,
        88, 90, 94, 96,
    ];
    let taproot_merkle_root_hex =
        "37a57b86de2819d2b72a173df46238a7ad295ea1485d3b40e9415daa82b4fdcb";

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: "session-real-taproot-multi-member-process".to_string(),
        participants,
        threshold: 51,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let first_request = StartSignRoundRequest {
        session_id: "session-real-taproot-multi-member-process".to_string(),
        member_identifier: 86,
        message_hex: "ac692bb7fddf3f7e1e050a83cf3ffb6e8e69888ce980281aa39da169525750ef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: Some(taproot_merkle_root_hex.to_string()),
        signing_participants: Some(signing_participants.clone()),
        attempt_context: None,
        attempt_transition_evidence: None,
    };

    let first_round_state =
        start_sign_round(first_request.clone()).expect("first member start sign round");
    assert_eq!(first_round_state.required_contributions, 51);
    assert_eq!(
        first_round_state.signing_participants.as_deref(),
        Some(signing_participants.as_slice())
    );

    let mut contributions = vec![first_round_state.own_contribution.clone()];
    for member_identifier in [76_u16, 39, 53, 3] {
        let round_state = start_sign_round(StartSignRoundRequest {
            member_identifier,
            ..first_request.clone()
        })
        .expect("next member start sign round");

        assert_eq!(round_state.session_id, first_round_state.session_id);
        assert_eq!(round_state.round_id, first_round_state.round_id);
        assert_eq!(round_state.required_contributions, 51);
        assert_eq!(round_state.own_contribution.identifier, member_identifier);
        contributions.push(round_state.own_contribution);
    }

    let (dkg_key_packages, dkg_public_key_package, sign_message_bytes) = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get(&first_request.session_id)
            .expect("session state");

        (
            session.dkg_key_packages.clone().expect("dkg key packages"),
            session
                .dkg_public_key_package
                .clone()
                .expect("dkg public key package"),
            session
                .sign_message_bytes
                .clone()
                .expect("sign message bytes"),
        )
    };
    let taproot_merkle_root_bytes =
        hex::decode(taproot_merkle_root_hex).expect("taproot merkle root");
    let mut taproot_merkle_root = [0_u8; 32];
    taproot_merkle_root.copy_from_slice(&taproot_merkle_root_bytes);

    for member_identifier in signing_participants
        .iter()
        .copied()
        .filter(|identifier| ![86_u16, 76, 39, 53, 3].contains(identifier))
        .take(46)
    {
        let member_request = StartSignRoundRequest {
            member_identifier,
            ..first_request.clone()
        };
        contributions.push(
            build_real_signature_share_contribution(
                &dkg_key_packages,
                &dkg_public_key_package,
                signing_participants.as_slice(),
                &member_request,
                &first_round_state.round_id,
                &sign_message_bytes,
                Some(&taproot_merkle_root),
            )
            .expect("additional contribution"),
        );
    }
    assert_eq!(contributions.len(), 51);

    let result = finalize_sign_round(
        FinalizeSignRoundRequest {
            session_id: first_request.session_id,
            taproot_merkle_root_hex: Some(taproot_merkle_root_hex.to_string()),
            attempt_context: None,
            round_contributions: contributions,
        },
        false,
    )
    .expect("finalize");

    assert_eq!(result.round_id, first_round_state.round_id);
    let signature_bytes = hex::decode(&result.signature_hex).expect("signature decode");
    let signature = frost::Signature::deserialize(&signature_bytes).expect("signature parse");
    let tweaked_public_key_package = dkg_public_key_package
        .clone()
        .tweak(Some(taproot_merkle_root.as_slice()));
    tweaked_public_key_package
        .verifying_key()
        .verify(&sign_message_bytes, &signature)
        .expect("tweaked signature verification");
}

#[test]
fn deterministic_round_nonce_and_commitment_binds_full_transcript() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-nonce-transcript-bound".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    run_dkg(run_dkg_request).expect("run dkg");

    let other_session_request = RunDkgRequest {
        session_id: "session-nonce-transcript-bound-other".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    run_dkg(other_session_request).expect("run other dkg");

    let fetch_session_material = |session_id: &str| {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get(session_id).expect("session state");

        (
            session
                .dkg_key_packages
                .as_ref()
                .expect("dkg key packages")
                .get(&1)
                .expect("key package")
                .clone(),
            session
                .dkg_public_key_package
                .clone()
                .expect("dkg public key package"),
        )
    };
    let (key_package, public_key_package) =
        fetch_session_material("session-nonce-transcript-bound");
    let (_, other_public_key_package) =
        fetch_session_material("session-nonce-transcript-bound-other");

    let public_key_package_bytes = public_key_package
        .serialize()
        .expect("public key package bytes");
    let other_public_key_package_bytes = other_public_key_package
        .serialize()
        .expect("other public key package bytes");

    // F1 regression: a package sharing the baseline's GROUP verifying
    // key but differing in a non-target participant's verifying share
    // (members 2 and 3 swapped). The target is member 1, so the old
    // group-key-only binding produced an identical seed here even
    // though every member re-derives member 2's commitment from this
    // share -- the silent nonce-reuse-under-a-different-challenge case.
    let identifier_two = participant_identifier_to_frost_identifier(2).expect("identifier 2");
    let identifier_three = participant_identifier_to_frost_identifier(3).expect("identifier 3");
    let mut perturbed_verifying_shares = public_key_package.verifying_shares().clone();
    let share_two = *perturbed_verifying_shares
        .get(&identifier_two)
        .expect("verifying share 2");
    let share_three = *perturbed_verifying_shares
        .get(&identifier_three)
        .expect("verifying share 3");
    perturbed_verifying_shares.insert(identifier_two, share_three);
    perturbed_verifying_shares.insert(identifier_three, share_two);
    let perturbed_share_package = frost::keys::PublicKeyPackage::new(
        perturbed_verifying_shares,
        *public_key_package.verifying_key(),
        None,
    );
    assert_eq!(
        perturbed_share_package.verifying_key(),
        public_key_package.verifying_key(),
        "perturbed package must keep the baseline group verifying key",
    );
    let perturbed_share_package_bytes = perturbed_share_package
        .serialize()
        .expect("perturbed share package bytes");

    let message_one = hex::decode("deadbeef").expect("message one decode");
    let message_two = hex::decode("cafebabe").expect("message two decode");
    let taproot_merkle_root = [0x42_u8; 32];
    let baseline_participants: Vec<u16> = vec![1, 2];
    let wider_participants: Vec<u16> = vec![1, 2, 3];

    let baseline_binding = RoundNonceBinding {
        session_id: "session-nonce-transcript-bound",
        round_id: "fixed-round-id",
        public_key_package_bytes: &public_key_package_bytes,
        message_bytes: &message_one,
        taproot_merkle_root: None,
        signing_participants: &baseline_participants,
        participant_identifier: 1,
    };

    let (_, baseline_commitments) =
        build_deterministic_round_nonce_and_commitment(&key_package, &baseline_binding);
    let (_, retry_commitments) =
        build_deterministic_round_nonce_and_commitment(&key_package, &baseline_binding);
    assert_eq!(
        baseline_commitments, retry_commitments,
        "identical binding inputs must re-derive identical commitments",
    );

    // Each transcript-affecting input must independently change the nonce.
    let variant_bindings = [
        RoundNonceBinding {
            message_bytes: &message_two,
            ..baseline_binding
        },
        RoundNonceBinding {
            taproot_merkle_root: Some(&taproot_merkle_root),
            ..baseline_binding
        },
        RoundNonceBinding {
            signing_participants: &wider_participants,
            ..baseline_binding
        },
        RoundNonceBinding {
            public_key_package_bytes: &other_public_key_package_bytes,
            ..baseline_binding
        },
        // Same group key, one non-target verifying share changed.
        RoundNonceBinding {
            public_key_package_bytes: &perturbed_share_package_bytes,
            ..baseline_binding
        },
        RoundNonceBinding {
            session_id: "session-nonce-transcript-bound-other",
            ..baseline_binding
        },
        RoundNonceBinding {
            round_id: "other-round-id",
            ..baseline_binding
        },
        RoundNonceBinding {
            participant_identifier: 2,
            ..baseline_binding
        },
    ];
    for (variant_index, variant_binding) in variant_bindings.iter().enumerate() {
        let (_, variant_commitments) =
            build_deterministic_round_nonce_and_commitment(&key_package, variant_binding);
        assert_ne!(
            baseline_commitments, variant_commitments,
            "binding variant [{variant_index}] must change the derived commitment",
        );
    }
}

#[test]
fn deterministic_seed_disambiguates_embedded_zero_bytes() {
    let parts_a = [b"\xaa\x00".as_slice(), b"\x01".as_slice()];
    let parts_b = [b"\xaa".as_slice(), b"\x00\x01".as_slice()];

    assert_ne!(deterministic_seed(&parts_a), deterministic_seed(&parts_b));
}

#[test]
fn finalize_rejects_tampered_session_message_bytes() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-message-tamper".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-message-tamper".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request.clone()).expect("start sign round");
    let signing_participants = round_state
        .signing_participants
        .clone()
        .expect("round signing participants");

    let (dkg_key_packages, dkg_public_key_package) = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get(&start_request.session_id)
            .expect("session state");

        (
            session.dkg_key_packages.clone().expect("dkg key packages"),
            session
                .dkg_public_key_package
                .clone()
                .expect("dkg public key package"),
        )
    };

    let member_two_request = StartSignRoundRequest {
        member_identifier: 2,
        attempt_transition_evidence: None,
        ..start_request.clone()
    };
    let member_two_contribution = build_real_signature_share_contribution(
        &dkg_key_packages,
        &dkg_public_key_package,
        &signing_participants,
        &member_two_request,
        &round_state.round_id,
        &hex::decode(&member_two_request.message_hex).expect("message decode"),
        None,
    )
    .expect("member two contribution");

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut(&start_request.session_id)
            .expect("session state");

        session.sign_message_bytes = Some(Zeroizing::new(
            hex::decode("cafebabe").expect("tamper decode"),
        ));
    }

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-message-tamper".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            round_state.own_contribution.clone(),
            member_two_contribution,
        ],
    };

    let err = finalize_sign_round(finalize_request, false).expect_err("expected failure");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };

    assert!(
        message.contains("failed to aggregate signature shares"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn finalize_rejects_real_contributor_set_mismatch_with_explicit_error() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-real-contributor-set-mismatch".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-real-contributor-set-mismatch".to_string(),
        member_identifier: 1,
        message_hex: "b16b00b5".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request.clone()).expect("start sign round");
    let signing_participants = round_state
        .signing_participants
        .clone()
        .expect("round signing participants");

    let (dkg_key_packages, dkg_public_key_package) = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get(&start_request.session_id)
            .expect("session state");

        (
            session.dkg_key_packages.clone().expect("dkg key packages"),
            session
                .dkg_public_key_package
                .clone()
                .expect("dkg public key package"),
        )
    };

    let member_two_request = StartSignRoundRequest {
        member_identifier: 2,
        attempt_transition_evidence: None,
        ..start_request
    };
    let member_two_contribution = build_real_signature_share_contribution(
        &dkg_key_packages,
        &dkg_public_key_package,
        &signing_participants,
        &member_two_request,
        &round_state.round_id,
        &hex::decode(&member_two_request.message_hex).expect("message decode"),
        None,
    )
    .expect("member two contribution");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-real-contributor-set-mismatch".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            round_state.own_contribution.clone(),
            member_two_contribution,
        ],
    };

    let err = finalize_sign_round(finalize_request, false).expect_err("expected mismatch");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };

    assert!(
        message.contains(
            "round contribution identifiers must match signing participants for real finalize"
        ),
        "unexpected validation message: {message}"
    );
    assert!(
        message.contains("[1, 2, 3]"),
        "expected identifier set in message: {message}"
    );
    assert!(
        message.contains("[1, 2]"),
        "expected contributor set in message: {message}"
    );
}

#[test]
fn finalize_rejects_real_contribution_identifier_outside_signing_cohort() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-real-outside-signing-cohort".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-real-outside-signing-cohort".to_string(),
        member_identifier: 1,
        message_hex: "facefeed".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-real-outside-signing-cohort".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            round_state.own_contribution,
            RoundContribution {
                identifier: 3,
                signature_share_hex: "abcd".to_string(),
            },
        ],
    };

    let err = finalize_sign_round(finalize_request, false).expect_err("expected rejection");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("round contribution identifier [3] is not in signing participant set"),
        "unexpected validation message: {message}"
    );
}

#[test]
fn run_dkg_conflict_persists_across_storage_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("run_dkg_conflict_persists");
    reset_for_tests();

    let request_a = RunDkgRequest {
        session_id: "session-persisted-conflict".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let mut request_b = request_a.clone();
    request_b.participants.push(crate::api::DkgParticipant {
        identifier: 3,
        public_key_hex: "02cc".to_string(),
    });

    run_dkg(request_a).expect("initial run dkg");
    reload_state_from_storage_for_tests();

    let err = run_dkg(request_b).expect_err("expected persisted session conflict");
    assert!(matches!(err, EngineError::SessionConflict { .. }));

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
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
fn run_dkg_rejects_new_session_when_session_registry_is_at_capacity() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("run_dkg_session_capacity");
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "1");

    let request_a = RunDkgRequest {
        session_id: "session-capacity-a".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    run_dkg(request_a.clone()).expect("initial run dkg");
    run_dkg(request_a).expect("idempotent run dkg at capacity");

    let request_b = RunDkgRequest {
        session_id: "session-capacity-b".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "03aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "03bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let err = run_dkg(request_b).expect_err("expected session cap rejection");
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
fn run_dkg_uses_secret_entropy_for_new_sessions_and_cache_for_retries() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("run_dkg_secret_entropy");
    reset_for_tests();

    let request_a = RunDkgRequest {
        session_id: "session-secret-entropy-a".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let mut request_b = request_a.clone();
    request_b.session_id = "session-secret-entropy-b".to_string();

    let result_a = run_dkg(request_a.clone()).expect("run dkg a");
    let retry_a = run_dkg(request_a).expect("retry dkg a");
    let result_b = run_dkg(request_b).expect("run dkg b");

    assert_eq!(result_a, retry_a);
    assert_ne!(
            result_a.key_group, result_b.key_group,
            "new sessions with the same public DKG request shape must not derive dealer entropy from public request data"
        );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn run_dkg_retry_is_participant_order_insensitive() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("run_dkg_participant_order_retry");
    reset_for_tests();

    let request = RunDkgRequest {
        session_id: "session-dkg-participant-order-retry".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let mut retry_request = request.clone();
    retry_request.participants.reverse();

    let first_result = run_dkg(request).expect("initial DKG");
    let retry_result = run_dkg(retry_request).expect("equivalent DKG retry");

    assert_eq!(first_result, retry_result);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
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
fn refresh_shares_rejects_new_session_when_session_registry_is_at_capacity() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("refresh_session_capacity");
    reset_for_tests();
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "1");

    let first_request = RefreshSharesRequest {
        session_id: "session-refresh-capacity-a".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "aa11".to_string(),
        }],
    };
    refresh_shares(first_request.clone()).expect("first refresh");
    refresh_shares(first_request).expect("idempotent refresh at capacity");

    let second_request = RefreshSharesRequest {
        session_id: "session-refresh-capacity-b".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "bb22".to_string(),
        }],
    };
    let err = refresh_shares(second_request).expect_err("expected session cap rejection");
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
fn refresh_shares_retry_is_share_order_insensitive() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("refresh_share_order_retry");
    reset_for_tests();

    let request = RefreshSharesRequest {
        session_id: "session-refresh-share-order-retry".to_string(),
        current_shares: vec![
            ShareMaterial {
                identifier: 3,
                encrypted_share_hex: "cccc".to_string(),
            },
            ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "aaaa".to_string(),
            },
            ShareMaterial {
                identifier: 2,
                encrypted_share_hex: "bbbb".to_string(),
            },
        ],
    };
    let mut retry_request = request.clone();
    retry_request.current_shares.reverse();

    let first_result = refresh_shares(request).expect("initial refresh");
    let retry_result = refresh_shares(retry_request).expect("equivalent refresh retry");

    assert_eq!(first_result, retry_result);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn refresh_shares_rejects_duplicate_current_share_identifiers() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("refresh_duplicate_share_identifier");
    reset_for_tests();

    let err = refresh_shares(RefreshSharesRequest {
        session_id: "session-refresh-duplicate-share-id".to_string(),
        current_shares: vec![
            ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "aaaa".to_string(),
            },
            ShareMaterial {
                identifier: 1,
                encrypted_share_hex: "bbbb".to_string(),
            },
        ],
    })
    .expect_err("expected duplicate share identifier rejection");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("current_shares contains duplicate identifier [1]"),
        "unexpected validation message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn refresh_shares_rejects_zero_current_share_identifier() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("refresh_zero_share_identifier");
    reset_for_tests();

    let err = refresh_shares(RefreshSharesRequest {
        session_id: "session-refresh-zero-share-id".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 0,
            encrypted_share_hex: "aaaa".to_string(),
        }],
    })
    .expect_err("expected zero share identifier rejection");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("current_shares identifiers must be non-zero"),
        "unexpected validation message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn sign_round_and_finalize_idempotency_persist_across_storage_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("sign_finalize_idempotency");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-persisted-idempotency".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let start_request = StartSignRoundRequest {
        session_id: "session-persisted-idempotency".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let first_round_state = start_sign_round(start_request.clone()).expect("start sign round");

    reload_state_from_storage_for_tests();
    let second_round_state = start_sign_round(start_request).expect("persisted start retry");
    assert_eq!(first_round_state, second_round_state);

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-persisted-idempotency".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&first_round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&first_round_state, 2),
            },
        ],
    };

    let first_signature =
        finalize_sign_round(finalize_request.clone(), true).expect("initial finalize");
    reload_state_from_storage_for_tests();
    let second_signature =
        finalize_sign_round(finalize_request, true).expect("persisted finalize retry");
    assert_eq!(first_signature, second_signature);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_accepts_persisted_legacy_member_bound_fingerprint() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("sign_legacy_member_fingerprint");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-legacy-member-fingerprint".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let start_request = StartSignRoundRequest {
        session_id: "session-legacy-member-fingerprint".to_string(),
        member_identifier: 1,
        message_hex: "baddcafe".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let first_round_state = start_sign_round(start_request.clone()).expect("start sign round");

    let canonical_fingerprint =
        start_sign_round_request_fingerprint(&start_request, 0).expect("canonical fingerprint");
    let legacy_member_fingerprint =
        start_sign_round_request_fingerprint(&start_request, start_request.member_identifier)
            .expect("legacy member fingerprint");
    assert_ne!(canonical_fingerprint, legacy_member_fingerprint);

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut(&start_request.session_id)
            .expect("session state");
        assert_eq!(
            session.sign_request_fingerprint.as_deref(),
            Some(canonical_fingerprint.as_str())
        );
        session.sign_request_fingerprint = Some(legacy_member_fingerprint.clone());
        persist_engine_state_to_storage(&guard).expect("persist legacy fingerprint");
    }

    reload_state_from_storage_for_tests();
    let retry_round_state =
        start_sign_round(start_request.clone()).expect("legacy fingerprint retry");
    assert_eq!(first_round_state, retry_round_state);

    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get(&start_request.session_id)
            .expect("session state");
        assert_eq!(
            session.sign_request_fingerprint.as_deref(),
            Some(canonical_fingerprint.as_str())
        );
    }

    let second_member_round_state = start_sign_round(StartSignRoundRequest {
        member_identifier: 2,
        ..start_request.clone()
    })
    .expect("second member after fingerprint migration");
    assert_eq!(
        first_round_state.round_id,
        second_member_round_state.round_id
    );
    assert_eq!(second_member_round_state.own_contribution.identifier, 2);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
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
fn start_sign_round_rejects_consumed_round_id_when_sign_cache_is_missing() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("sign_round_consumed_nonce_enforcement");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-sign-round-consumed-nonce".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let start_request = StartSignRoundRequest {
        session_id: "session-sign-round-consumed-nonce".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let first_round_state = start_sign_round(start_request.clone()).expect("start sign round");

    reload_state_from_storage_for_tests();

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut("session-sign-round-consumed-nonce")
            .expect("session state");
        assert!(session
            .consumed_sign_round_ids
            .contains(&first_round_state.round_id));
        session.sign_request_fingerprint = None;
        session.sign_message_bytes = None;
        session.round_state = None;
        persist_engine_state_to_storage(&guard).expect("persist tampered sign cache state");
    }

    reload_state_from_storage_for_tests();
    let err = start_sign_round(start_request).expect_err("expected consumed round rejection");
    let EngineError::ConsumedRoundReplay {
        round_id,
        session_id: _,
    } = err
    else {
        panic!("unexpected error variant");
    };
    assert_eq!(round_id, first_round_state.round_id);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_replay_guard_survives_process_restart_with_sign_cache_loss() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("sign_round_consumed_nonce_restart_replay");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-sign-round-consumed-nonce-restart".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let start_request = StartSignRoundRequest {
        session_id: "session-sign-round-consumed-nonce-restart".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let first_round_state = start_sign_round(start_request.clone()).expect("start sign round");

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut("session-sign-round-consumed-nonce-restart")
            .expect("session state");
        assert!(session
            .consumed_sign_round_ids
            .contains(&first_round_state.round_id));
        session.sign_request_fingerprint = None;
        session.sign_message_bytes = None;
        session.round_state = None;
        persist_engine_state_to_storage(&guard).expect("persist tampered sign cache state");
    }

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let err = start_sign_round(start_request).expect_err("expected consumed round rejection");
    let EngineError::ConsumedRoundReplay {
        round_id,
        session_id: _,
    } = err
    else {
        panic!("unexpected error variant");
    };
    assert_eq!(round_id, first_round_state.round_id);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_rejects_consumed_attempt_id_when_sign_cache_is_missing() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("sign_round_consumed_attempt_enforcement");
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-sign-round-consumed-attempt";
    let message_hex = "deadbeef";
    let run_dkg_request = RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
    let expected_attempt_id = attempt_context.attempt_id.clone();
    let start_request = StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    };
    start_sign_round(start_request.clone()).expect("start sign round");

    reload_state_from_storage_for_tests();

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get_mut(session_id).expect("session state");
        assert!(session.consumed_attempt_ids.contains(&expected_attempt_id));
        session.sign_request_fingerprint = None;
        session.sign_message_bytes = None;
        session.round_state = None;
        persist_engine_state_to_storage(&guard).expect("persist tampered sign cache state");
    }

    reload_state_from_storage_for_tests();
    let err = start_sign_round(start_request).expect_err("expected consumed attempt-id rejection");
    let EngineError::ConsumedAttemptReplay {
        attempt_id,
        session_id: _,
    } = err
    else {
        panic!("unexpected error variant");
    };
    assert_eq!(attempt_id, expected_attempt_id);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_attempt_replay_guard_survives_process_restart_with_sign_cache_loss() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("sign_round_consumed_attempt_restart_replay");
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-sign-round-consumed-attempt-restart";
    let message_hex = "deadbeef";
    let run_dkg_request = RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
    let expected_attempt_id = attempt_context.attempt_id.clone();
    let start_request = StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    };
    start_sign_round(start_request.clone()).expect("start sign round");

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get_mut(session_id).expect("session state");
        assert!(session.consumed_attempt_ids.contains(&expected_attempt_id));
        session.sign_request_fingerprint = None;
        session.sign_message_bytes = None;
        session.round_state = None;
        persist_engine_state_to_storage(&guard).expect("persist tampered sign cache state");
    }

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let err = start_sign_round(start_request).expect_err("expected consumed attempt-id rejection");
    let EngineError::ConsumedAttemptReplay {
        attempt_id,
        session_id: _,
    } = err
    else {
        panic!("unexpected error variant");
    };
    assert_eq!(attempt_id, expected_attempt_id);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn persist_fault_after_temp_sync_before_rename_preserves_previous_state_on_restart() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("persist_fault_before_rename");
    reset_for_tests();

    let existing_request = RunDkgRequest {
        session_id: "session-persist-fault-existing".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    run_dkg(existing_request).expect("seed existing persisted session");

    let failed_request = RunDkgRequest {
        session_id: "session-persist-fault-before-rename".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "03aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "03bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let err = run_dkg(failed_request).expect_err("expected injected persist failure");
    clear_persist_fault_injection_for_tests();

    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("injected persist fault at [after_temp_sync_before_rename]"),
        "unexpected persist fault message: {message}"
    );
    assert!(
        !state_path
            .with_extension(format!("tmp-{}", std::process::id()))
            .exists(),
        "persist temp state file should be cleaned up on failure"
    );

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        assert!(guard
            .sessions
            .contains_key("session-persist-fault-existing"));
        assert!(!guard
            .sessions
            .contains_key("session-persist-fault-before-rename"));
    }

    run_dkg(RunDkgRequest {
        session_id: "session-persist-fault-recovery".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "04aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "04bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("post-fault recovery run dkg");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_rejects_when_consumed_sign_round_registry_is_at_capacity() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("sign_round_consumed_capacity");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-sign-round-consumed-capacity".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut("session-sign-round-consumed-capacity")
            .expect("session state");

        for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
            session
                .consumed_sign_round_ids
                .insert(format!("preused-round-{idx}"));
        }
        persist_engine_state_to_storage(&guard).expect("persist prefilled consumed sign rounds");
    }

    let start_request = StartSignRoundRequest {
        session_id: "session-sign-round-consumed-capacity".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let err = start_sign_round(start_request).expect_err("expected capacity rejection");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("consumed_sign_round_ids registry size"),
        "unexpected internal message: {message}"
    );
    assert!(
        message.contains("reached max"),
        "unexpected internal message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_rejects_when_consumed_sign_round_registry_is_at_capacity_with_attempt_context()
{
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("sign_round_consumed_capacity_attempt_context");
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-sign-round-consumed-capacity-attempt-context";
    let message_hex = "deadbeef";
    let run_dkg_request = RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get_mut(session_id).expect("session state");

        for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
            session
                .consumed_sign_round_ids
                .insert(format!("preused-round-{idx}"));
        }
        persist_engine_state_to_storage(&guard).expect("persist prefilled consumed sign rounds");
    }

    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
    let start_request = StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    };
    let err = start_sign_round(start_request).expect_err("expected capacity rejection");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("consumed_sign_round_ids registry size"),
        "unexpected internal message: {message}"
    );
    assert!(
        message.contains("reached max"),
        "unexpected internal message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_rejects_when_consumed_attempt_registry_is_at_capacity_with_attempt_context() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("sign_round_consumed_attempt_capacity");
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-sign-round-consumed-attempt-capacity";
    let message_hex = "deadbeef";
    let run_dkg_request = RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get_mut(session_id).expect("session state");

        for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
            session
                .consumed_attempt_ids
                .insert(format!("preused-attempt-{idx}"));
        }
        persist_engine_state_to_storage(&guard).expect("persist prefilled consumed attempt IDs");
    }

    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
    let start_request = StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context),
        attempt_transition_evidence: None,
    };
    let err = start_sign_round(start_request).expect_err("expected capacity rejection");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("consumed_attempt_ids registry size"),
        "unexpected internal message: {message}"
    );
    assert!(
        message.contains("reached max"),
        "unexpected internal message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_rejects_consumed_round_id_when_finalize_cache_is_missing() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("finalize_consumed_round_enforcement");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-consumed-round".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-consumed-round".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-consumed-round".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };
    finalize_sign_round(finalize_request.clone(), true).expect("first finalize");

    reload_state_from_storage_for_tests();

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut("session-finalize-consumed-round")
            .expect("session state");
        assert!(session
            .consumed_finalize_round_ids
            .contains(&round_state.round_id));
        session.finalize_request_fingerprint = None;
        session.signature_result = None;
        session.round_state = Some(round_state.clone());
        persist_engine_state_to_storage(&guard).expect("persist tampered finalize cache state");
    }

    let round_only_replay_request = FinalizeSignRoundRequest {
        session_id: finalize_request.session_id.clone(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: format!(
                    "{}00",
                    bootstrap_synthetic_share_hex(&round_state, 1)
                ),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };

    reload_state_from_storage_for_tests();
    let err = finalize_sign_round(round_only_replay_request, true)
        .expect_err("expected consumed round-id rejection");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("already consumed for finalize"),
        "unexpected validation message: {message}"
    );
    assert!(
        message.contains(&round_state.round_id),
        "unexpected validation message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn persist_fault_after_rename_before_directory_sync_keeps_state_loadable_after_restart() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("persist_fault_after_rename");
    reset_for_tests();

    let existing_request = RunDkgRequest {
        session_id: "session-persist-fault-existing-after-rename".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    run_dkg(existing_request).expect("seed existing persisted session");

    let renamed_request = RunDkgRequest {
        session_id: "session-persist-fault-after-rename".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "03aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "03bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    set_persist_fault_injection_for_tests(
        PersistFaultInjectionPoint::AfterRenameBeforeDirectorySync,
    );
    let err = run_dkg(renamed_request.clone()).expect_err("expected injected persist failure");
    clear_persist_fault_injection_for_tests();

    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("injected persist fault at [after_rename_before_directory_sync]"),
        "unexpected persist fault message: {message}"
    );
    assert!(
        !state_path
            .with_extension(format!("tmp-{}", std::process::id()))
            .exists(),
        "persist temp state file should not remain after post-rename failure"
    );

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        assert!(guard
            .sessions
            .contains_key("session-persist-fault-existing-after-rename"));
        assert!(guard
            .sessions
            .contains_key("session-persist-fault-after-rename"));
    }

    let retry_result = run_dkg(renamed_request).expect("retry request after reload");
    assert_eq!(
        retry_result.session_id,
        "session-persist-fault-after-rename"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_rejects_when_consumed_request_registry_is_at_capacity() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("finalize_consumed_request_capacity");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-consumed-request-capacity".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-consumed-request-capacity".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut("session-finalize-consumed-request-capacity")
            .expect("session state");

        for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
            session
                .consumed_finalize_request_fingerprints
                .insert(format!("prefilled-fingerprint-{idx}"));
        }
        persist_engine_state_to_storage(&guard)
            .expect("persist prefilled consumed finalize request fingerprints");
    }

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-consumed-request-capacity".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };
    let err = finalize_sign_round(finalize_request, true).expect_err("expected capacity rejection");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("consumed_finalize_request_fingerprints registry size"),
        "unexpected internal message: {message}"
    );
    assert!(
        message.contains("reached max"),
        "unexpected internal message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_rejects_when_consumed_request_registry_is_at_capacity_with_attempt_context()
{
    let _guard = lock_test_state();
    let state_path =
        configure_test_state_path("finalize_consumed_request_capacity_attempt_context");
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-finalize-consumed-request-capacity-attempt-context";
    let message_hex = "deadbeef";
    let run_dkg_request = RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let mut uppercase_attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![1, 2]);
    uppercase_attempt_context.included_participants_fingerprint = uppercase_attempt_context
        .included_participants_fingerprint
        .to_ascii_uppercase();
    uppercase_attempt_context.attempt_id =
        uppercase_attempt_context.attempt_id.to_ascii_uppercase();

    let start_request = StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(uppercase_attempt_context.clone()),
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get_mut(session_id).expect("session state");

        for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
            session
                .consumed_finalize_request_fingerprints
                .insert(format!("prefilled-fingerprint-{idx}"));
        }
        persist_engine_state_to_storage(&guard)
            .expect("persist prefilled consumed finalize request fingerprints");
    }

    let finalize_request = FinalizeSignRoundRequest {
        session_id: session_id.to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: Some(uppercase_attempt_context),
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };
    let err = finalize_sign_round(finalize_request, true).expect_err("expected capacity rejection");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("consumed_finalize_request_fingerprints registry size"),
        "unexpected internal message: {message}"
    );
    assert!(
        message.contains("reached max"),
        "unexpected internal message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_rejects_when_consumed_round_registry_is_at_capacity() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("finalize_consumed_round_capacity");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-consumed-round-capacity".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-consumed-round-capacity".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut("session-finalize-consumed-round-capacity")
            .expect("session state");

        for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
            session
                .consumed_finalize_round_ids
                .insert(format!("prefilled-round-{idx}"));
        }
        persist_engine_state_to_storage(&guard)
            .expect("persist prefilled consumed finalize round IDs");
    }

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-consumed-round-capacity".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };
    let err = finalize_sign_round(finalize_request, true).expect_err("expected capacity rejection");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("consumed_finalize_round_ids registry size"),
        "unexpected internal message: {message}"
    );
    assert!(
        message.contains("reached max"),
        "unexpected internal message: {message}"
    );

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get("session-finalize-consumed-round-capacity")
            .expect("session state");
        assert!(session.finalize_request_fingerprint.is_none());
        assert!(session.signature_result.is_none());
    }

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_rejects_when_consumed_round_registry_is_at_capacity_with_attempt_context() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("finalize_consumed_round_capacity_attempt_context");
    reset_for_tests();
    let _roast_strict_mode = RoastStrictModeGuard::enable();

    let session_id = "session-finalize-consumed-round-capacity-attempt-context";
    let message_hex = "deadbeef";
    let run_dkg_request = RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let attempt_context =
        build_deterministic_attempt_context(session_id, message_hex, 1, vec![2, 1]);
    let start_request = StartSignRoundRequest {
        session_id: session_id.to_string(),
        member_identifier: 1,
        message_hex: message_hex.to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![1, 2]),
        attempt_context: Some(attempt_context.clone()),
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get_mut(session_id).expect("session state");

        for idx in 0..TBTC_SIGNER_MAX_CONSUMED_REGISTRY_ENTRIES_PER_SESSION {
            session
                .consumed_finalize_round_ids
                .insert(format!("prefilled-round-{idx}"));
        }
        persist_engine_state_to_storage(&guard)
            .expect("persist prefilled consumed finalize round IDs");
    }

    let finalize_request = FinalizeSignRoundRequest {
        session_id: session_id.to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: Some(attempt_context),
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };
    let err = finalize_sign_round(finalize_request, true).expect_err("expected capacity rejection");
    let EngineError::Internal(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("consumed_finalize_round_ids registry size"),
        "unexpected internal message: {message}"
    );
    assert!(
        message.contains("reached max"),
        "unexpected internal message: {message}"
    );

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard.sessions.get(session_id).expect("session state");
        assert!(session.finalize_request_fingerprint.is_none());
        assert!(session.signature_result.is_none());
    }

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_rejects_consumed_request_fingerprint_when_round_state_missing() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("finalize_consumed_request_fingerprint");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-consumed-request-fingerprint".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-consumed-request-fingerprint".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-consumed-request-fingerprint".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };
    let mut canonical_contributions = finalize_request.round_contributions.clone();
    canonical_contributions.sort_unstable_by(|left, right| {
        left.identifier
            .cmp(&right.identifier)
            .then_with(|| left.signature_share_hex.cmp(&right.signature_share_hex))
    });
    let expected_request_fingerprint = fingerprint(&FinalizeSignRoundRequest {
        session_id: finalize_request.session_id.clone(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: canonical_contributions,
    })
    .expect("finalize request fingerprint");

    finalize_sign_round(finalize_request.clone(), true).expect("first finalize");
    reload_state_from_storage_for_tests();

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut("session-finalize-consumed-request-fingerprint")
            .expect("session state");
        assert!(session
            .consumed_finalize_request_fingerprints
            .contains(&expected_request_fingerprint));
        assert!(session.round_state.is_none());
        session.finalize_request_fingerprint = None;
        session.signature_result = None;
        persist_engine_state_to_storage(&guard)
            .expect("persist tampered finalize request cache state");
    }

    reload_state_from_storage_for_tests();
    let err = finalize_sign_round(finalize_request, true)
        .expect_err("expected consumed request fingerprint rejection");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("finalize request fingerprint"),
        "unexpected validation message: {message}"
    );
    assert!(
        message.contains("already consumed"),
        "unexpected validation message: {message}"
    );
    assert!(
        message.contains(&expected_request_fingerprint),
        "unexpected validation message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn finalize_sign_round_replay_guard_survives_process_restart_with_finalize_cache_loss() {
    let _guard = lock_test_state();
    let state_path =
        configure_test_state_path("finalize_consumed_request_fingerprint_restart_replay");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-consumed-request-fingerprint-restart".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-consumed-request-fingerprint-restart".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-consumed-request-fingerprint-restart".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };
    let mut canonical_contributions = finalize_request.round_contributions.clone();
    canonical_contributions.sort_unstable_by(|left, right| {
        left.identifier
            .cmp(&right.identifier)
            .then_with(|| left.signature_share_hex.cmp(&right.signature_share_hex))
    });
    let expected_request_fingerprint = fingerprint(&FinalizeSignRoundRequest {
        session_id: finalize_request.session_id.clone(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: canonical_contributions,
    })
    .expect("finalize request fingerprint");

    finalize_sign_round(finalize_request.clone(), true).expect("first finalize");

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    {
        let mut guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get_mut("session-finalize-consumed-request-fingerprint-restart")
            .expect("session state");
        assert!(session
            .consumed_finalize_request_fingerprints
            .contains(&expected_request_fingerprint));
        assert!(session.round_state.is_none());
        session.finalize_request_fingerprint = None;
        session.signature_result = None;
        persist_engine_state_to_storage(&guard)
            .expect("persist tampered finalize request cache state");
    }

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();
    let err = finalize_sign_round(finalize_request, true)
        .expect_err("expected consumed request fingerprint rejection");
    let EngineError::Validation(message) = err else {
        panic!("unexpected error variant");
    };
    assert!(
        message.contains("finalize request fingerprint"),
        "unexpected validation message: {message}"
    );
    assert!(
        message.contains("already consumed"),
        "unexpected validation message: {message}"
    );
    assert!(
        message.contains(&expected_request_fingerprint),
        "unexpected validation message: {message}"
    );

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn start_sign_round_accepts_reordered_participant_idempotent_retry() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-start-round-reordered-idempotency".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let first_request = StartSignRoundRequest {
        session_id: "session-start-round-reordered-idempotency".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![3, 1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let first_round_state = start_sign_round(first_request).expect("first start sign round");
    let consumed_round_ids_after_first = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get("session-start-round-reordered-idempotency")
            .expect("session state");
        session.consumed_sign_round_ids.clone()
    };
    assert_eq!(consumed_round_ids_after_first.len(), 1);
    assert!(consumed_round_ids_after_first.contains(&first_round_state.round_id));

    let second_request = StartSignRoundRequest {
        session_id: "session-start-round-reordered-idempotency".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![2, 3, 1]),
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let second_round_state =
        start_sign_round(second_request).expect("second start sign round retry");

    assert_eq!(first_round_state, second_round_state);
    let consumed_round_ids_after_second = {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get("session-start-round-reordered-idempotency")
            .expect("session state");
        session.consumed_sign_round_ids.clone()
    };
    assert_eq!(
        consumed_round_ids_after_first,
        consumed_round_ids_after_second
    );
}

#[test]
fn start_sign_round_rejects_materially_different_retry_after_canonicalization() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-start-round-canonicalization-conflict".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 3,
                public_key_hex: "02cc".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let first_request = StartSignRoundRequest {
        session_id: "session-start-round-canonicalization-conflict".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group.clone(),
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![3, 1, 2]),
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    start_sign_round(first_request).expect("first start sign round");

    let second_request = StartSignRoundRequest {
        session_id: "session-start-round-canonicalization-conflict".to_string(),
        member_identifier: 1,
        message_hex: "cafebabe".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: Some(vec![2, 3, 1]),
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let err = start_sign_round(second_request).expect_err("expected session conflict");
    assert!(matches!(err, EngineError::SessionConflict { .. }));
}

#[test]
fn finalize_sign_round_accepts_reordered_contribution_idempotent_retry() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-reordered-idempotency".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-reordered-idempotency".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    let first_finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-reordered-idempotency".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };

    let second_finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-reordered-idempotency".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
        ],
    };

    let first_signature =
        finalize_sign_round(first_finalize_request, true).expect("first finalize");
    let second_signature =
        finalize_sign_round(second_finalize_request, true).expect("second finalize retry");

    assert_eq!(first_signature, second_signature);
}

#[test]
fn finalize_sign_round_rejects_materially_different_retry_after_canonicalization() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-canonicalization-conflict".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");

    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-canonicalization-conflict".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    let first_finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-canonicalization-conflict".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };
    finalize_sign_round(first_finalize_request, true).expect("first finalize");

    let second_finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-canonicalization-conflict".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
            RoundContribution {
                identifier: 1,
                signature_share_hex: format!(
                    "00{}",
                    bootstrap_synthetic_share_hex(&round_state, 1)
                ),
            },
        ],
    };
    let err = finalize_sign_round(second_finalize_request, true).expect_err("expected conflict");
    assert!(matches!(err, EngineError::SessionConflict { .. }));
}

#[test]
fn refresh_epoch_counter_persists_across_storage_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("refresh_epoch_counter");
    reset_for_tests();

    let first_result = refresh_shares(RefreshSharesRequest {
        session_id: "session-persisted-refresh-1".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "aaaa".to_string(),
        }],
    })
    .expect("first refresh");
    assert_eq!(first_result.refresh_epoch, 1);

    reload_state_from_storage_for_tests();

    let second_result = refresh_shares(RefreshSharesRequest {
        session_id: "session-persisted-refresh-2".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "bbbb".to_string(),
        }],
    })
    .expect("second refresh");
    assert_eq!(second_result.refresh_epoch, 2);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
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

    refresh_shares(RefreshSharesRequest {
        session_id: "session-lock-path-initial".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "aaaa".to_string(),
        }],
    })
    .expect("initial refresh");

    std::env::set_var(TBTC_SIGNER_STATE_PATH_ENV, &alternate_state_path);

    let err = refresh_shares(RefreshSharesRequest {
        session_id: "session-lock-path-switch".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "bbbb".to_string(),
        }],
    })
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

    let dkg_request = RunDkgRequest {
        session_id: "session-restart-dkg".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let dkg_result = run_dkg(dkg_request.clone()).expect("run dkg");

    let build_request = BuildTaprootTxRequest {
        session_id: "session-restart-buildtx".to_string(),
        inputs: vec![crate::api::TxInput {
            txid_hex: "11".repeat(32),
            vout: 0,
            value_sats: 10_000,
        }],
        outputs: vec![crate::api::TxOutput {
            script_pubkey_hex: format!("5120{}", "22".repeat(32)),
            value_sats: 9_000,
        }],
        script_tree_hex: None,
    };
    let build_result = build_taproot_tx(build_request.clone()).expect("build taproot tx");

    let refresh_request = RefreshSharesRequest {
        session_id: "session-restart-refresh".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "abba".to_string(),
        }],
    };
    let refresh_result = refresh_shares(refresh_request.clone()).expect("refresh shares");

    let finalize_dkg_request = RunDkgRequest {
        session_id: "session-restart-finalize".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "03aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "03bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };
    let finalize_dkg_result = run_dkg(finalize_dkg_request).expect("run finalize dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-restart-finalize".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: finalize_dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request).expect("start sign round");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-restart-finalize".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };
    let finalize_result =
        finalize_sign_round(finalize_request.clone(), true).expect("finalize sign round");

    simulate_process_restart_for_tests();
    reload_state_from_storage_for_tests();

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        assert!(guard.sessions.contains_key("session-restart-dkg"));
        assert!(guard.sessions.contains_key("session-restart-buildtx"));
        assert!(guard.sessions.contains_key("session-restart-refresh"));
        assert!(guard.sessions.contains_key("session-restart-finalize"));
    }

    let dkg_retry_result = run_dkg(dkg_request).expect("retry run dkg");
    assert_eq!(dkg_result, dkg_retry_result);

    let build_retry_result = build_taproot_tx(build_request).expect("retry build taproot tx");
    assert_eq!(build_result, build_retry_result);

    let refresh_retry_result = refresh_shares(refresh_request).expect("retry refresh shares");
    assert_eq!(refresh_result, refresh_retry_result);

    let finalize_retry_result =
        finalize_sign_round(finalize_request, true).expect("retry finalize sign round");
    assert_eq!(finalize_result, finalize_retry_result);

    let new_session_result = run_dkg(RunDkgRequest {
        session_id: "session-restart-new".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "04aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "04bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("post-restart run dkg");
    assert!(!new_session_result.key_group.is_empty());

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
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

    refresh_shares(RefreshSharesRequest {
        session_id: "session-state-file-permissions".to_string(),
        current_shares: vec![ShareMaterial {
            identifier: 1,
            encrypted_share_hex: "aaaa".to_string(),
        }],
    })
    .expect("persist state via refresh");

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
fn finalize_clears_signing_material_and_rejects_sign_round_restart() {
    let _guard = lock_test_state();
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-clears-signing-material".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-clears-signing-material".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request.clone()).expect("start sign round");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-clears-signing-material".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };

    let first_result = finalize_sign_round(finalize_request.clone(), true).expect("finalize");

    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get("session-finalize-clears-signing-material")
            .expect("session state");

        assert!(session.finalize_request_fingerprint.is_some());
        assert!(session.signature_result.is_some());
        assert!(session.dkg_key_packages.is_none());
        assert!(session.dkg_public_key_package.is_none());
        assert!(session.sign_request_fingerprint.is_none());
        assert!(session.sign_message_bytes.is_none());
        assert!(session.round_state.is_none());
    }

    let second_result =
        finalize_sign_round(finalize_request, true).expect("finalize idempotent retry");
    assert_eq!(first_result, second_result);

    let err = start_sign_round(start_request).expect_err("start sign round should fail");
    assert!(matches!(err, EngineError::SessionFinalized { .. }));
}

#[test]
fn finalize_purge_persists_across_storage_reload() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("finalize_purge_persist_reload");
    reset_for_tests();

    let run_dkg_request = RunDkgRequest {
        session_id: "session-finalize-purge-persist-reload".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    let dkg_result = run_dkg(run_dkg_request).expect("run dkg");
    let start_request = StartSignRoundRequest {
        session_id: "session-finalize-purge-persist-reload".to_string(),
        member_identifier: 1,
        message_hex: "deadbeef".to_string(),
        key_group: dkg_result.key_group,
        taproot_merkle_root_hex: None,
        signing_participants: None,
        attempt_context: None,
        attempt_transition_evidence: None,
    };
    let round_state = start_sign_round(start_request.clone()).expect("start sign round");

    let finalize_request = FinalizeSignRoundRequest {
        session_id: "session-finalize-purge-persist-reload".to_string(),
        taproot_merkle_root_hex: None,
        attempt_context: None,
        round_contributions: vec![
            RoundContribution {
                identifier: 1,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 1),
            },
            RoundContribution {
                identifier: 2,
                signature_share_hex: bootstrap_synthetic_share_hex(&round_state, 2),
            },
        ],
    };

    let first_result = finalize_sign_round(finalize_request.clone(), true).expect("finalize");

    reload_state_from_storage_for_tests();
    {
        let guard = state().expect("engine state").lock().expect("engine lock");
        let session = guard
            .sessions
            .get("session-finalize-purge-persist-reload")
            .expect("session state");

        assert!(session.finalize_request_fingerprint.is_some());
        assert!(session.signature_result.is_some());
        assert!(session.dkg_key_packages.is_none());
        assert!(session.dkg_public_key_package.is_none());
        assert!(session.sign_request_fingerprint.is_none());
        assert!(session.sign_message_bytes.is_none());
        assert!(session.round_state.is_none());
    }

    let second_result =
        finalize_sign_round(finalize_request, true).expect("persisted finalize retry");
    assert_eq!(first_result, second_result);

    let err = start_sign_round(start_request).expect_err("start sign round should fail");
    assert!(matches!(err, EngineError::SessionFinalized { .. }));

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
fn truncated_state_file_fails_closed_by_default() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("truncated_state_fail_closed");
    reset_for_tests();

    run_dkg(RunDkgRequest {
        session_id: "session-truncated-state-fail-closed".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("seed persisted state");

    let persisted_bytes = std::fs::read(&state_path).expect("read persisted state file");
    assert!(
        persisted_bytes.len() > 1,
        "persisted state should be larger than one byte"
    );
    std::fs::write(&state_path, &persisted_bytes[..persisted_bytes.len() - 1])
        .expect("write truncated state file");

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
fn truncated_state_file_quarantines_and_resets_when_enabled() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("truncated_state_quarantine_reset");
    reset_for_tests();

    std::env::set_var(
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV,
        TBTC_SIGNER_STATE_CORRUPTION_POLICY_QUARANTINE_AND_RESET,
    );

    run_dkg(RunDkgRequest {
        session_id: "session-truncated-state-quarantine-reset".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("seed persisted state");

    let persisted_bytes = std::fs::read(&state_path).expect("read persisted state file");
    assert!(
        persisted_bytes.len() > 1,
        "persisted state should be larger than one byte"
    );
    let truncated_bytes = persisted_bytes[..persisted_bytes.len() - 1].to_vec();
    std::fs::write(&state_path, &truncated_bytes).expect("write truncated state file");

    let loaded = load_engine_state_from_storage().expect("recover from truncated state file");
    assert!(loaded.sessions.is_empty());
    assert_eq!(loaded.refresh_epoch_counter, 0);
    assert!(!state_path.exists());

    let backups =
        sorted_corrupted_state_backups(&state_path).expect("list corrupted state backups");
    assert_eq!(backups.len(), 1);
    let backup_contents = std::fs::read(&backups[0]).expect("read backup file contents");
    assert_eq!(backup_contents, truncated_bytes);

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

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

    let err = match load_engine_state_from_storage() {
        Ok(_) => panic!("expected schema mismatch failure"),
        Err(err) => err,
    };
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

    let loaded = load_engine_state_from_storage().expect("recover from schema mismatch state");
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

#[test]
fn persisted_state_is_encrypted_envelope() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("encrypted_envelope_persist");
    reset_for_tests();

    run_dkg(RunDkgRequest {
        session_id: "session-encrypted-envelope".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("seed persisted encrypted state");

    let persisted_bytes = std::fs::read(&state_path).expect("read persisted state file");
    let envelope: PersistedEncryptedEngineStateEnvelope =
        serde_json::from_slice(&persisted_bytes).expect("decode encrypted envelope");
    assert_eq!(
        envelope.schema_version,
        PERSISTED_STATE_ENVELOPE_SCHEMA_VERSION
    );
    assert_eq!(
        envelope.encryption_algorithm,
        TBTC_SIGNER_STATE_ENCRYPTION_ALGORITHM_XCHACHA20POLY1305
    );
    assert_eq!(
        envelope.key_provider,
        TBTC_SIGNER_STATE_KEY_PROVIDER_ENV_DEFAULT
    );
    assert!(envelope.key_id.starts_with("sha256:"));
    assert_eq!(
        envelope.authentication_tag.len(),
        TBTC_SIGNER_STATE_ENVELOPE_AUTH_TAG_BYTES * 2
    );
    assert!(!envelope.ciphertext.is_empty());

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

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

    let loaded = load_engine_state_from_storage().expect("load and migrate legacy plaintext");
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
fn encrypted_state_load_fails_closed_when_key_missing() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("encrypted_state_missing_key");
    reset_for_tests();

    run_dkg(RunDkgRequest {
        session_id: "session-encrypted-state-missing-key".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("seed encrypted state file");

    std::env::remove_var(TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV);
    let err = match load_engine_state_from_storage() {
        Ok(_) => panic!("expected encrypted state load failure"),
        Err(err) => err,
    };
    let err_message = err.to_string();
    assert!(err_message.contains("missing required state encryption key env"));
    assert!(err_message.contains(TBTC_SIGNER_STATE_CORRUPTION_POLICY_ENV));
    assert!(state_path.exists());

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn encrypted_state_load_rejects_tampered_legacy_key_id_format() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("encrypted_state_legacy_key_id");
    reset_for_tests();

    let session_id = "session-encrypted-state-legacy-key-id";
    run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("seed encrypted state file");

    let persisted_bytes = std::fs::read(&state_path).expect("read persisted state file");
    let mut envelope: PersistedEncryptedEngineStateEnvelope =
        serde_json::from_slice(&persisted_bytes).expect("decode encrypted envelope");
    envelope.key_id = TBTC_SIGNER_STATE_KEY_ID_LEGACY_ENV_HEX.to_string();
    let mutated_bytes = serde_json::to_vec(&envelope).expect("encode legacy key_id envelope");
    std::fs::write(&state_path, mutated_bytes).expect("write legacy key_id envelope");

    let err = match load_engine_state_from_storage() {
        Ok(_) => panic!("tampered legacy key_id envelope should fail closed"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "state key identifier mismatch");

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

    let err = mutate_state_for_key_provider_test("session-production-rejects-env-provider")
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

    let err = mutate_state_for_key_provider_test("session-production-rejects-implicit-state-path")
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

    let err = mutate_state_for_key_provider_test("session-unknown-state-key-provider")
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
        mutate_state_for_key_provider_test("session-production-command-provider-non-zero-exit")
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

    let err = mutate_state_for_key_provider_test("session-production-command-provider-bad-output")
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

    mutate_state_for_key_provider_test("session-production-command-provider-large-stderr")
        .expect("large stderr from state key command should not deadlock");

    reset_for_tests();
    cleanup_test_state_artifacts(&state_path);
    clear_state_storage_policy_overrides();
}

#[test]
fn encrypted_state_load_rejects_mismatched_key_id() {
    let _guard = lock_test_state();
    let state_path = configure_test_state_path("encrypted_state_mismatched_key_id");
    reset_for_tests();

    run_dkg(RunDkgRequest {
        session_id: "session-encrypted-state-mismatched-key-id".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("seed encrypted state file");

    std::env::set_var(
        TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
        "2222222222222222222222222222222222222222222222222222222222222222",
    );
    let err = match load_engine_state_from_storage() {
        Ok(_) => panic!("expected key_id mismatch rejection"),
        Err(err) => err,
    };
    expect_internal_error_contains(err, "state key identifier mismatch");

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

    let err = mutate_state_for_key_provider_test("session-production-command-provider-timeout")
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
        mutate_state_for_key_provider_test("session-production-command-provider-background-pipe")
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

    mutate_state_for_key_provider_test("session-production-command-provider")
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

    let result = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        roast_coordinator_timeout_ms: Some(60_000),
        ..InitSignerConfigRequest::default()
    })
    .expect("install config");
    assert!(result.installed);
    assert!(!result.idempotent);
    assert_eq!(result.configured_key_count, 2);

    assert_eq!(roast_coordinator_timeout_ms(), 60_000);
    std::env::remove_var(TBTC_SIGNER_ROAST_COORDINATOR_TIMEOUT_MS_ENV);
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

    // Firewall enforcement on, but the required allowed-script-classes knob
    // is absent from the same config (and, wholesale semantics, the
    // environment cannot supply it) -> the loader rejects and the install
    // must roll back. (Admission knobs would NOT trip this: that loader
    // falls back to defaults for absent values.)
    let error = init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        enforce_signing_policy_firewall: Some(true),
        ..InitSignerConfigRequest::default()
    })
    .expect_err("incomplete firewall policy must fail the init");
    assert!(
        error.to_string().contains("missing required env"),
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
fn init_signer_config_state_path_is_honored_end_to_end() {
    let _guard = lock_test_state();
    reset_for_tests();
    let _clear = InstalledConfigClearGuard;
    clear_state_storage_policy_overrides();

    let state_path = std::env::temp_dir().join(format!(
        "frost_init_config_e2e_state_{}.json",
        std::process::id()
    ));
    let _ = fs::remove_file(&state_path);

    init_signer_config(InitSignerConfigRequest {
        profile: Some("development".to_string()),
        state_path: Some(state_path.to_string_lossy().into_owned()),
        ..InitSignerConfigRequest::default()
    })
    .expect("install config");

    let dkg_request = RunDkgRequest {
        session_id: "session-init-config-e2e".to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    };

    // The state-file lock was already bound to the default path by the
    // pre-install persist in reset_for_tests, and the engine refuses to
    // switch state paths in-process: installing a config after state has
    // been touched fails loudly instead of splitting state across paths.
    let error = run_dkg(dkg_request.clone())
        .expect_err("state-path switch after first state access must be refused");
    assert!(
        error.to_string().contains("refusing to switch"),
        "unexpected error: {error}"
    );

    // A fresh process that installs the config before touching state binds
    // the lock at the configured path and persists there.
    simulate_process_restart_for_tests();
    run_dkg(dkg_request).expect("run dkg under installed config after restart");

    assert!(
        state_path.exists(),
        "engine state must persist at the config-provided path"
    );

    reset_for_tests();
    let _ = fs::remove_file(&state_path);
    let _ = fs::remove_file(state_path.with_extension("json.lock"));
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

#[allow(clippy::too_many_arguments)]
fn open_interactive_for_test(
    key_packages: &BTreeMap<u16, crate::api::NativeFrostKeyPackage>,
    session_id: &str,
    key_group: &str,
    message_bytes: &[u8],
    included_participants: &[u16],
    wire_attempt_number: u32,
    member_identifier: u16,
    threshold: u16,
) -> Result<InteractiveSessionOpenResult, EngineError> {
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
        attempt_context,
        key_package_identifier: key_packages[&member_identifier].identifier.clone(),
        key_package_hex: key_packages[&member_identifier].data_hex.clone(),
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
    let opened = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
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
            session.interactive_signing.is_none(),
            "completed Round2 must free the live interactive session state"
        );
        assert!(
            session
                .consumed_interactive_attempt_markers
                .contains(&opened.attempt_id),
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
fn interactive_round1_is_idempotent_until_consumed() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-round1-idempotent";
    let key_group = "interactive-test-key-group";
    let message = [0x21u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect("opens");

    let first = interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");
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

    let opened = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
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
    let opened_a = open_interactive_for_test(
        &key_packages,
        session_a,
        key_group,
        &message,
        &[1, 2],
        1,
        1,
        2,
    )
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
    let opened_b = open_interactive_for_test(
        &key_packages,
        session_b,
        key_group,
        &message,
        &[1, 2, 3],
        1,
        1,
        2,
    )
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

    let opened = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
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
    let reopen = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect_err("reopening a consumed attempt after restart must fail closed");
    assert!(
        matches!(reopen, EngineError::ConsumedNonceReplay { .. }),
        "unexpected error: {reopen:?}"
    );

    // A fresh attempt for the same session proceeds: the marker is
    // attempt-scoped, not session-scoped.
    let second_attempt = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        2,
        1,
        2,
    )
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

    let opened = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
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

    // Consumption-before-release: if the durable marker cannot be
    // persisted, NO share leaves the engine and the nonces stay live.
    set_persist_fault_injection_for_tests(PersistFaultInjectionPoint::AfterTempSyncBeforeRename);
    let faulted = interactive_round2(InteractiveRound2Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
        signing_package_hex: signing_package_hex.clone(),
    })
    .expect_err("injected persist fault must fail round 2");
    clear_persist_fault_injection_for_tests();
    assert!(
        matches!(faulted, EngineError::Internal(ref m) if m.contains("injected persist fault")),
        "unexpected error: {faulted:?}"
    );

    {
        let guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get(session_id).expect("session exists");
        assert!(
            !session
                .consumed_interactive_attempt_markers
                .contains(&opened.attempt_id),
            "a failed persist must roll the consumption marker back"
        );
    }

    // The same attempt completes once persistence recovers - the
    // nonces were never consumed by the failed call.
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
                .consumed_interactive_attempt_markers
                .contains(&opened.attempt_id),
            "successful round 2 must leave the durable marker"
        );
    }
}

#[test]
fn interactive_open_idempotency_conflict_and_replacement() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-open-lifecycle";
    let key_group = "interactive-test-key-group";
    let message = [0x81u8; 32];
    let included = [1u16, 2];

    let first = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect("opens");
    assert!(!first.idempotent);

    let repeat = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
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
        attempt_context,
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
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
    let second = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        2,
        1,
        2,
    )
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

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-abort";
    let key_group = "interactive-test-key-group";
    let message = [0x91u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
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
    let reopened = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect("an aborted (never consumed) attempt may reopen");
    assert_eq!(reopened.attempt_id, opened.attempt_id);
}

#[test]
fn interactive_session_ttl_expiry_has_abort_semantics() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-ttl";
    let key_group = "interactive-test-key-group";
    let message = [0xa1u8; 32];
    let included = [1u16, 2];

    let opened = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect("opens");
    interactive_round1(InteractiveRound1Request {
        session_id: session_id.to_string(),
        attempt_id: opened.attempt_id.clone(),
        member_identifier: 1,
    })
    .expect("round 1");

    // Age the session past the TTL directly; the next entry point's
    // lazy sweep must destroy the nonces with abort semantics.
    {
        let mut guard = state().expect("state").lock().expect("lock");
        let session = guard.sessions.get_mut(session_id).expect("session exists");
        let interactive = session
            .interactive_signing
            .as_mut()
            .expect("live interactive state");
        interactive.opened_at_unix = interactive
            .opened_at_unix
            .saturating_sub(interactive_session_ttl_seconds() + 1);
    }

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
    open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
    .expect("an expired (never consumed) attempt may reopen");
}

#[test]
fn interactive_live_session_capacity_fails_closed() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let key_group = "interactive-test-key-group";
    let message = [0xb1u8; 32];
    let included = [1u16, 2];

    std::env::set_var(TBTC_SIGNER_MAX_LIVE_INTERACTIVE_SESSIONS_ENV, "1");

    let outcome = (|| -> Result<(), EngineError> {
        open_interactive_for_test(
            &key_packages,
            "interactive-cap-a",
            key_group,
            &message,
            &included,
            1,
            1,
            2,
        )?;

        let at_capacity = open_interactive_for_test(
            &key_packages,
            "interactive-cap-b",
            key_group,
            &message,
            &included,
            1,
            1,
            2,
        )
        .expect_err("the live-session cap must fail closed");
        assert!(
            matches!(at_capacity, EngineError::Internal(ref m)
                if m.contains("live interactive session count")),
            "unexpected error: {at_capacity:?}"
        );

        // A capacity rejection for a brand-new session_id must NOT
        // leave an empty SessionState behind (it would otherwise
        // accumulate against the global session cap and could starve
        // DKG).
        {
            let guard = state().expect("state").lock().expect("lock");
            assert!(
                !guard.sessions.contains_key("interactive-cap-b"),
                "a rejected interactive open must not insert an empty session"
            );
        }

        // An idempotent reopen of the live session does not trip the cap.
        let reopen = open_interactive_for_test(
            &key_packages,
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
        open_interactive_for_test(
            &key_packages,
            "interactive-cap-b",
            key_group,
            &message,
            &included,
            1,
            1,
            2,
        )?;
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
    let key_packages = interactive_test_key_packages();
    let outcome = open_interactive_for_test(
        &key_packages,
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
fn interactive_open_signing_policy_firewall_binds_message_to_build_tx() {
    let _guard = lock_test_state();
    reset_for_tests();
    clear_state_storage_policy_overrides();

    let session_id = "interactive-firewall-bound";
    std::env::set_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV, "true");
    std::env::set_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV, "p2tr,p2wpkh");
    configure_required_signing_policy_limits_for_tests();

    let dkg_result = run_dkg(RunDkgRequest {
        session_id: session_id.to_string(),
        participants: vec![
            crate::api::DkgParticipant {
                identifier: 1,
                public_key_hex: "02aa".to_string(),
            },
            crate::api::DkgParticipant {
                identifier: 2,
                public_key_hex: "02bb".to_string(),
            },
        ],
        threshold: 2,
        dkg_seed_hex: None,
    })
    .expect("run dkg");

    let tx_result = build_taproot_tx(build_policy_test_request(session_id)).expect("build tx");
    let bound_message_hex = policy_bound_message_hex_from_tx_result(&tx_result);
    let bound_message = hex::decode(&bound_message_hex).expect("bound message decodes");
    let key_packages = interactive_test_key_packages();

    let outcome = (|| -> Result<(), EngineError> {
        // A message NOT bound to the policy-checked tx is rejected even
        // for an otherwise-valid attempt context.
        let unbound = open_interactive_for_test(
            &key_packages,
            session_id,
            &dkg_result.key_group,
            &[0xd2u8; 32],
            &[1u16, 2],
            1,
            1,
            2,
        )
        .expect_err("an unbound message must be rejected under the firewall");
        assert!(
            matches!(unbound, EngineError::SigningPolicyRejected { ref reason_code, .. }
                if reason_code == "signing_message_not_bound_to_policy_checked_build_tx"),
            "unexpected error: {unbound:?}"
        );

        // The policy-bound message opens successfully: enforcement is
        // real, not always-reject.
        let opened = open_interactive_for_test(
            &key_packages,
            session_id,
            &dkg_result.key_group,
            &bound_message,
            &[1u16, 2],
            1,
            1,
            2,
        )?;
        assert!(!opened.idempotent);
        Ok(())
    })();

    std::env::remove_var(TBTC_SIGNER_ENFORCE_SIGNING_POLICY_FIREWALL_ENV);
    std::env::remove_var(TBTC_SIGNER_POLICY_ALLOWED_SCRIPT_CLASSES_ENV);
    clear_state_storage_policy_overrides();

    outcome.expect("policy-bound interactive open lifecycle");
}

#[test]
fn interactive_consumed_marker_is_case_insensitive() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
    let session_id = "interactive-attempt-id-casing";
    let key_group = "interactive-test-key-group";
    let message = [0xe3u8; 32];
    let included = [1u16, 2];

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
        attempt_context: canonical.clone(),
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
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
        attempt_context: recased_context,
        key_package_identifier: key_packages[&1].identifier.clone(),
        key_package_hex: key_packages[&1].data_hex.clone(),
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

    let key_packages = interactive_test_key_packages();
    let key_group = "interactive-test-key-group";
    let message = [0xf4u8; 32];
    let included = [1u16, 2];

    // Open a live attempt on session A, then age it past the TTL.
    let opened = open_interactive_for_test(
        &key_packages,
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
    {
        let mut guard = state().expect("state").lock().expect("lock");
        let session = guard
            .sessions
            .get_mut("interactive-abort-sweep-a")
            .expect("session A exists");
        let interactive = session
            .interactive_signing
            .as_mut()
            .expect("live interactive state");
        interactive.opened_at_unix = interactive
            .opened_at_unix
            .saturating_sub(interactive_session_ttl_seconds() + 1);
    }

    // An abort for a DIFFERENT session is the only post-expiry traffic;
    // it must still sweep session A's expired nonces (the TTL guarantee
    // holds regardless of which entry point takes the lock).
    interactive_session_abort(InteractiveSessionAbortRequest {
        session_id: "interactive-abort-sweep-other".to_string(),
        attempt_id: None,
    })
    .expect("abort for an unrelated session");

    // Session A held only its (now-expired) interactive attempt, so the
    // sweep must remove the whole entry, not just clear the live state -
    // otherwise empty sessions accumulate against TBTC_SIGNER_MAX_SESSIONS.
    let guard = state().expect("state").lock().expect("lock");
    assert!(
        !guard.sessions.contains_key("interactive-abort-sweep-a"),
        "an abort must sweep AND drop an otherwise-empty expired session"
    );
}

#[test]
fn interactive_open_rejected_on_session_lifecycle_states() {
    let _guard = lock_test_state();
    reset_for_tests();

    let key_packages = interactive_test_key_packages();
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
                emergency_rekey_event: Some(EmergencyRekeyEvent {
                    reason: "test rekey".to_string(),
                    triggered_at_unix: now_unix(),
                }),
                ..Default::default()
            },
        );
    }
    let rekey = open_interactive_for_test(
        &key_packages,
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
                finalize_request_fingerprint: Some("already-finalized".to_string()),
                ..Default::default()
            },
        );
    }
    let finalized = open_interactive_for_test(
        &key_packages,
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

    let key_packages = interactive_test_key_packages();
    let key_group = "interactive-test-key-group";
    let message = [0x18u8; 32];
    let included = [1u16, 2];

    let outcome = (|| -> Result<(), EngineError> {
        let quarantined = open_interactive_for_test(
            &key_packages,
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
            &key_packages,
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
    let opened = open_interactive_for_test(
        &key_packages,
        session_id,
        key_group,
        &message,
        &included,
        1,
        1,
        2,
    )
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
        session.emergency_rekey_event = Some(EmergencyRekeyEvent {
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
                .consumed_interactive_attempt_markers
                .contains(&opened.attempt_id),
            "a gate rejection must not consume the attempt"
        );
        session.emergency_rekey_event = None;
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
fn interactive_open_rejects_threshold_below_key_package_min_signers() {
    let _guard = lock_test_state();
    reset_for_tests();

    // The fixture key packages are min_signers = 2. A request threshold
    // of 3 must be rejected at Open: otherwise Round2 would accept a
    // 3-commitment package, persist the marker, and only then have
    // frost::round2::sign fail on the count - burning the nonce for a
    // validation error.
    let key_packages = interactive_test_key_packages();
    let mismatch = open_interactive_for_test(
        &key_packages,
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
            if m.contains("does not match the key package min_signers")),
        "unexpected error: {mismatch:?}"
    );

    // The matching threshold (2) opens.
    open_interactive_for_test(
        &key_packages,
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
fn interactive_open_abort_churn_does_not_exhaust_session_registry() {
    let _guard = lock_test_state();
    reset_for_tests();

    // A tiny global session cap: if open-then-abort left empty session
    // entries behind, this churn would fill the registry and then reject
    // a fresh open. The disposal on abort must keep the registry clear.
    std::env::set_var(TBTC_SIGNER_MAX_SESSIONS_ENV, "2");

    let key_packages = interactive_test_key_packages();
    let key_group = "interactive-test-key-group";
    let message = [0x1bu8; 32];
    let included = [1u16, 2];

    let outcome = (|| -> Result<(), EngineError> {
        for cycle in 0..16 {
            let session_id = format!("interactive-churn-{cycle}");
            open_interactive_for_test(
                &key_packages,
                &session_id,
                key_group,
                &message,
                &included,
                1,
                1,
                2,
            )?;
            interactive_session_abort(InteractiveSessionAbortRequest {
                session_id: session_id.clone(),
                attempt_id: None,
            })?;
        }
        // The registry is clear, so the global cap still has room.
        let guard = state().expect("state").lock().expect("lock");
        assert!(
            guard.sessions.is_empty(),
            "open-then-abort churn must not accumulate session entries: {} present",
            guard.sessions.len()
        );
        Ok(())
    })();

    std::env::remove_var(TBTC_SIGNER_MAX_SESSIONS_ENV);
    outcome.expect("session churn stays bounded");
}
