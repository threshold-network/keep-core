use bitcoin::{
    consensus::{deserialize, encode::serialize},
    hashes::{sha256, Hash},
    secp256k1::{
        schnorr::Signature as SchnorrSignature, Message as SecpMessage, Secp256k1, XOnlyPublicKey,
    },
    sighash::{Prevouts, SighashCache, TapSighashType},
    Amount, ScriptBuf, Transaction, TxOut,
};
use serde::Deserialize;

const SIGHASH_DEFAULT: u8 = 0;
const SIGHASH_ALL: u8 = 1;
const WITNESS_ERROR_INVALID_LENGTH: &str = "invalid-length";
const WITNESS_ERROR_UNSUPPORTED_SIGHASH: &str = "unsupported-sighash";

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct P2trSignatureFraudVectors {
    name: String,
    cases: Vec<VectorCase>,
    #[serde(default)]
    negative_witness_cases: Vec<NegativeWitnessCase>,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct VectorCase {
    id: String,
    #[serde(rename = "walletIDHex")]
    wallet_id_hex: String,
    wallet_p2tr_script_pub_key_hex: String,
    unsigned_transaction_hex: String,
    signed_input_index: usize,
    prevouts: Vec<Prevout>,
    outputs: Vec<Output>,
    sighash_type: u8,
    expected_bip341_sighash_hex: String,
    bip340_signature_hex: String,
    witness_signature_hex: String,
    expected_draft_challenge_identity_hex: String,
    expected_bridge_challenge_identity_hex: String,
    expected_verify: bool,
    #[serde(default)]
    negative_verification_cases: Vec<NegativeVerificationCase>,
    #[serde(default)]
    negative_sighash_cases: Vec<NegativeSighashCase>,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Prevout {
    txid_hex: String,
    vout: u32,
    value_sats: u64,
    script_pub_key_hex: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Output {
    value_sats: u64,
    script_pub_key_hex: String,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NegativeVerificationCase {
    id: String,
    #[serde(rename = "walletIDHex")]
    wallet_id_hex: Option<String>,
    bip341_sighash_hex: Option<String>,
    bip340_signature_hex: Option<String>,
    expected_verify: bool,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NegativeSighashCase {
    id: String,
    unsigned_transaction_hex: Option<String>,
    prevouts: Option<Vec<Prevout>>,
    outputs: Option<Vec<Output>>,
    expected_verify: bool,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NegativeWitnessCase {
    id: String,
    base_case_id: String,
    witness_signature_hex: String,
    expected_error: String,
}

fn decode_hex<const N: usize>(hex_value: &str, context: &str) -> [u8; N] {
    let bytes = hex::decode(hex_value).unwrap_or_else(|e| panic!("{context}: invalid hex: {e}"));
    bytes.try_into().unwrap_or_else(|bytes: Vec<u8>| {
        panic!("{context}: expected {N} bytes, got {}", bytes.len())
    })
}

fn decode_vec(hex_value: &str, context: &str) -> Vec<u8> {
    hex::decode(hex_value).unwrap_or_else(|e| panic!("{context}: invalid hex: {e}"))
}

fn tap_sighash_type(raw: u8, context: &str) -> TapSighashType {
    match raw {
        SIGHASH_DEFAULT => TapSighashType::Default,
        SIGHASH_ALL => TapSighashType::All,
        _ => panic!("{context}: unsupported Taproot sighash type {raw}"),
    }
}

fn parse_unsigned_transaction(case: &VectorCase) -> Transaction {
    let tx_bytes = decode_vec(&case.unsigned_transaction_hex, &case.id);
    deserialize(&tx_bytes).unwrap_or_else(|e| panic!("{}: transaction decode failed: {e}", case.id))
}

fn validate_prevout_metadata(case: &VectorCase, transaction: &Transaction) {
    assert_eq!(
        case.prevouts.len(),
        transaction.input.len(),
        "{}: prevout count must match transaction input count",
        case.id
    );

    for (index, (prevout, input)) in case
        .prevouts
        .iter()
        .zip(transaction.input.iter())
        .enumerate()
    {
        assert_eq!(
            prevout.txid_hex,
            input.previous_output.txid.to_string(),
            "{}: prevout {index} txid mismatch",
            case.id
        );
        assert_eq!(
            prevout.vout, input.previous_output.vout,
            "{}: prevout {index} vout mismatch",
            case.id
        );
    }
}

fn validate_outputs(case: &VectorCase, transaction: &Transaction) {
    assert_eq!(
        case.outputs.len(),
        transaction.output.len(),
        "{}: output count must match transaction output count",
        case.id
    );

    for (index, (expected, actual)) in case
        .outputs
        .iter()
        .zip(transaction.output.iter())
        .enumerate()
    {
        assert_eq!(
            expected.value_sats,
            actual.value.to_sat(),
            "{}: output {index} value mismatch",
            case.id
        );
        assert_eq!(
            decode_vec(
                &expected.script_pub_key_hex,
                &format!("{} output {index}", case.id)
            ),
            actual.script_pubkey.as_bytes(),
            "{}: output {index} scriptPubKey mismatch",
            case.id
        );
    }
}

fn prevout_txouts(case: &VectorCase) -> Vec<TxOut> {
    case.prevouts
        .iter()
        .enumerate()
        .map(|(index, prevout)| TxOut {
            value: Amount::from_sat(prevout.value_sats),
            script_pubkey: ScriptBuf::from_bytes(decode_vec(
                &prevout.script_pub_key_hex,
                &format!("{} prevout {index}", case.id),
            )),
        })
        .collect()
}

fn compute_bip341_key_path_sighash(case: &VectorCase) -> [u8; 32] {
    let transaction = parse_unsigned_transaction(case);
    validate_prevout_metadata(case, &transaction);
    validate_outputs(case, &transaction);

    let prevouts = prevout_txouts(case);
    let sighash = SighashCache::new(&transaction)
        .taproot_key_spend_signature_hash(
            case.signed_input_index,
            &Prevouts::All(&prevouts),
            tap_sighash_type(case.sighash_type, &case.id),
        )
        .unwrap_or_else(|e| panic!("{}: Taproot sighash failed: {e}", case.id));

    sighash.to_byte_array()
}

fn encode_compact_size(value: usize) -> Vec<u8> {
    if value < 0xfd {
        return vec![value as u8];
    }
    if value <= 0xffff {
        let mut bytes = vec![0xfd];
        bytes.extend_from_slice(&(value as u16).to_le_bytes());
        return bytes;
    }
    if value <= 0xffff_ffff {
        let mut bytes = vec![0xfe];
        bytes.extend_from_slice(&(value as u32).to_le_bytes());
        return bytes;
    }

    let mut bytes = vec![0xff];
    bytes.extend_from_slice(&(value as u64).to_le_bytes());
    bytes
}

fn push_len_prefixed(preimage: &mut Vec<u8>, bytes: &[u8]) {
    preimage.extend_from_slice(&encode_compact_size(bytes.len()));
    preimage.extend_from_slice(bytes);
}

fn derive_draft_challenge_identity(
    case: &VectorCase,
    sighash: [u8; 32],
    signature: &[u8],
) -> [u8; 32] {
    let mut preimage = Vec::new();
    preimage.extend_from_slice(b"tbtc-p2tr-signature-fraud-challenge-v0");
    preimage.extend_from_slice(&decode_vec(&case.wallet_id_hex, &case.id));
    preimage.extend_from_slice(&sighash);
    preimage.extend_from_slice(signature);
    preimage.push(case.sighash_type);
    let input_index = u32::try_from(case.signed_input_index)
        .unwrap_or_else(|_| panic!("{}: signed input index exceeds u32", case.id));
    preimage.extend_from_slice(&input_index.to_le_bytes());

    let tx_bytes = decode_vec(&case.unsigned_transaction_hex, &case.id);
    push_len_prefixed(&mut preimage, &tx_bytes);
    preimage.extend_from_slice(&encode_compact_size(case.prevouts.len()));

    for (index, prevout) in case.prevouts.iter().enumerate() {
        preimage.extend_from_slice(&decode_vec(
            &prevout.txid_hex,
            &format!("{} prevout {index} txid", case.id),
        ));
        preimage.extend_from_slice(&prevout.vout.to_le_bytes());
        preimage.extend_from_slice(&prevout.value_sats.to_le_bytes());
        push_len_prefixed(
            &mut preimage,
            &decode_vec(
                &prevout.script_pub_key_hex,
                &format!("{} prevout {index} script", case.id),
            ),
        );
    }

    sha256::Hash::hash(&preimage).to_byte_array()
}

fn derive_bridge_challenge_identity(
    case: &VectorCase,
    sighash: [u8; 32],
    signature: &[u8],
) -> [u8; 32] {
    let transaction = parse_unsigned_transaction(case);
    let mut preimage = Vec::new();
    preimage.extend_from_slice(b"tbtc-p2tr-signature-fraud-bridge-challenge-v0");
    preimage.extend_from_slice(&decode_vec(&case.wallet_id_hex, &case.id));
    preimage.extend_from_slice(&sighash);
    preimage.extend_from_slice(signature);
    preimage.push(case.sighash_type);
    let input_index = u32::try_from(case.signed_input_index)
        .unwrap_or_else(|_| panic!("{}: signed input index exceeds u32", case.id));
    preimage.extend_from_slice(&input_index.to_le_bytes());
    preimage.extend_from_slice(&serialize(&transaction.version));
    preimage.extend_from_slice(&serialize(&transaction.lock_time));

    preimage.extend_from_slice(&encode_compact_size(transaction.input.len()));
    for input in &transaction.input {
        preimage.extend_from_slice(&serialize(&input.previous_output));
        preimage.extend_from_slice(&serialize(&input.sequence));
    }

    let prevouts = prevout_txouts(case);
    preimage.extend_from_slice(&encode_compact_size(prevouts.len()));
    for prevout in &prevouts {
        preimage.extend_from_slice(&serialize(prevout));
    }

    preimage.extend_from_slice(&encode_compact_size(transaction.output.len()));
    for output in &transaction.output {
        preimage.extend_from_slice(&serialize(output));
    }

    sha256::Hash::hash(&preimage).to_byte_array()
}

fn verify_bip340(message: [u8; 32], wallet_id: &[u8], signature: &[u8]) -> bool {
    let Ok(public_key) = XOnlyPublicKey::from_slice(wallet_id) else {
        return false;
    };
    let Ok(signature) = SchnorrSignature::from_slice(signature) else {
        return false;
    };
    let Ok(message) = SecpMessage::from_digest_slice(&message) else {
        return false;
    };

    Secp256k1::verification_only()
        .verify_schnorr(&signature, &message, &public_key)
        .is_ok()
}

fn parse_witness_signature(
    witness_signature_hex: &str,
    context: &str,
) -> Result<(Vec<u8>, u8), &'static str> {
    let witness_signature = decode_vec(witness_signature_hex, context);

    if witness_signature.len() == 64 {
        return Ok((witness_signature, SIGHASH_DEFAULT));
    }

    if witness_signature.len() != 65 {
        return Err(WITNESS_ERROR_INVALID_LENGTH);
    }

    let sighash_type = witness_signature[64];
    if sighash_type == SIGHASH_DEFAULT {
        return Err(WITNESS_ERROR_UNSUPPORTED_SIGHASH);
    }
    if sighash_type != SIGHASH_ALL {
        return Err(WITNESS_ERROR_UNSUPPORTED_SIGHASH);
    }

    Ok((witness_signature[..64].to_vec(), sighash_type))
}

fn mutate_last_byte_hex(value: &str) -> String {
    let replacement = if value.ends_with("00") { "01" } else { "00" };
    format!("{}{}", &value[..value.len() - 2], replacement)
}

fn with_negative_sighash_case(base: &VectorCase, negative: &NegativeSighashCase) -> VectorCase {
    let mut mutated = base.clone();
    mutated.id = format!("{}/{}", base.id, negative.id);
    if let Some(unsigned_transaction_hex) = &negative.unsigned_transaction_hex {
        mutated.unsigned_transaction_hex = unsigned_transaction_hex.clone();
    }
    if let Some(prevouts) = &negative.prevouts {
        mutated.prevouts = prevouts.clone();
    }
    if let Some(outputs) = &negative.outputs {
        mutated.outputs = outputs.clone();
    }
    mutated
}

#[test]
fn formal_verification_p2tr_signature_fraud_vectors_match_bitcoin_crate() {
    let vectors_path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../docs/frost-migration/test-vectors/p2tr-signature-fraud-v0.json");
    let vectors_bytes =
        std::fs::read(&vectors_path).unwrap_or_else(|e| panic!("read {vectors_path:?}: {e}"));
    let vectors: P2trSignatureFraudVectors =
        serde_json::from_slice(&vectors_bytes).expect("P2TR signature-fraud vectors decode");

    assert_eq!(vectors.name, "p2tr-signature-fraud-v0");

    let mut verified = 0usize;
    let mut challenge_identities = std::collections::HashSet::new();
    let mut bridge_challenge_identities = std::collections::HashSet::new();
    let mut sighash_types = std::collections::HashSet::new();
    let mut witness_sighash_types = std::collections::HashSet::new();
    let case_ids: std::collections::HashSet<_> =
        vectors.cases.iter().map(|case| case.id.as_str()).collect();
    for case in &vectors.cases {
        let wallet_id = decode_vec(&case.wallet_id_hex, &case.id);
        let mut expected_wallet_script = vec![0x51, 0x20];
        expected_wallet_script.extend_from_slice(&wallet_id);
        assert_eq!(
            expected_wallet_script,
            decode_vec(&case.wallet_p2tr_script_pub_key_hex, &case.id),
            "{}: wallet script must be OP_1 x-only wallet ID",
            case.id
        );

        let actual_sighash = compute_bip341_key_path_sighash(case);
        sighash_types.insert(case.sighash_type);
        let expected_sighash = decode_hex::<32>(&case.expected_bip341_sighash_hex, &case.id);
        assert_eq!(
            expected_sighash, actual_sighash,
            "{}: sighash mismatch",
            case.id
        );

        let signature = decode_vec(&case.bip340_signature_hex, &case.id);
        let (witness_signature, witness_sighash_type) =
            parse_witness_signature(&case.witness_signature_hex, &case.id)
                .unwrap_or_else(|e| panic!("{}: witness signature rejected with {e}", case.id));
        assert_eq!(
            signature, witness_signature,
            "{}: witness signature does not match BIP-340 signature",
            case.id
        );
        assert_eq!(
            case.sighash_type, witness_sighash_type,
            "{}: witness sighash type mismatch",
            case.id
        );
        witness_sighash_types.insert(witness_sighash_type);

        assert_eq!(
            case.expected_verify,
            verify_bip340(actual_sighash, &wallet_id, &signature),
            "{}: BIP-340 verification mismatch",
            case.id
        );
        verified += 1;

        let challenge_identity = derive_draft_challenge_identity(case, actual_sighash, &signature);
        assert_eq!(
            decode_hex::<32>(&case.expected_draft_challenge_identity_hex, &case.id),
            challenge_identity,
            "{}: draft challenge identity mismatch",
            case.id
        );
        assert!(
            challenge_identities.insert(challenge_identity),
            "{}: duplicate draft challenge identity",
            case.id
        );

        let bridge_challenge_identity =
            derive_bridge_challenge_identity(case, actual_sighash, &signature);
        assert_eq!(
            decode_hex::<32>(&case.expected_bridge_challenge_identity_hex, &case.id),
            bridge_challenge_identity,
            "{}: Bridge challenge identity mismatch",
            case.id
        );
        assert!(
            bridge_challenge_identities.insert(bridge_challenge_identity),
            "{}: duplicate Bridge challenge identity",
            case.id
        );

        if case.id == "bip341-keypath-sighash-default-single-input" {
            let mut wrong_wallet_case = case.clone();
            wrong_wallet_case.wallet_id_hex = mutate_last_byte_hex(&case.wallet_id_hex);
            assert_ne!(
                challenge_identity,
                derive_draft_challenge_identity(&wrong_wallet_case, actual_sighash, &signature),
                "{}: draft challenge identity must commit to wallet ID",
                case.id
            );

            let wrong_sighash = decode_hex::<32>(
                &mutate_last_byte_hex(&case.expected_bip341_sighash_hex),
                &case.id,
            );
            assert_ne!(
                challenge_identity,
                derive_draft_challenge_identity(case, wrong_sighash, &signature),
                "{}: draft challenge identity must commit to sighash",
                case.id
            );

            let wrong_signature =
                decode_vec(&mutate_last_byte_hex(&case.bip340_signature_hex), &case.id);
            assert_ne!(
                challenge_identity,
                derive_draft_challenge_identity(case, actual_sighash, &wrong_signature),
                "{}: draft challenge identity must commit to signature",
                case.id
            );

            let mut wrong_sighash_type_case = case.clone();
            wrong_sighash_type_case.sighash_type = if case.sighash_type == SIGHASH_DEFAULT {
                SIGHASH_ALL
            } else {
                SIGHASH_DEFAULT
            };
            assert_ne!(
                challenge_identity,
                derive_draft_challenge_identity(
                    &wrong_sighash_type_case,
                    actual_sighash,
                    &signature
                ),
                "{}: draft challenge identity must commit to sighash type",
                case.id
            );

            let mut wrong_transaction_case = case.clone();
            wrong_transaction_case.unsigned_transaction_hex =
                mutate_last_byte_hex(&case.unsigned_transaction_hex);
            assert_ne!(
                challenge_identity,
                derive_draft_challenge_identity(
                    &wrong_transaction_case,
                    actual_sighash,
                    &signature
                ),
                "{}: draft challenge identity must commit to raw transaction",
                case.id
            );
        }

        for negative in &case.negative_verification_cases {
            let negative_wallet_id = negative
                .wallet_id_hex
                .as_ref()
                .map(|value| decode_vec(value, &format!("{}/{}", case.id, negative.id)))
                .unwrap_or_else(|| wallet_id.clone());
            let negative_message = negative
                .bip341_sighash_hex
                .as_ref()
                .map(|value| decode_hex::<32>(value, &format!("{}/{}", case.id, negative.id)))
                .unwrap_or(actual_sighash);
            let negative_signature = negative
                .bip340_signature_hex
                .as_ref()
                .map(|value| decode_vec(value, &format!("{}/{}", case.id, negative.id)))
                .unwrap_or_else(|| signature.clone());

            assert_eq!(
                negative.expected_verify,
                verify_bip340(negative_message, &negative_wallet_id, &negative_signature),
                "{}/{}: negative BIP-340 verification mismatch",
                case.id,
                negative.id
            );
            verified += 1;
        }

        for negative in &case.negative_sighash_cases {
            let negative_case = with_negative_sighash_case(case, negative);
            let negative_sighash = compute_bip341_key_path_sighash(&negative_case);
            assert_ne!(
                actual_sighash, negative_sighash,
                "{}: negative sighash did not change",
                negative_case.id
            );
            assert_eq!(
                negative.expected_verify,
                verify_bip340(negative_sighash, &wallet_id, &signature),
                "{}: negative sighash verification mismatch",
                negative_case.id
            );
            verified += 1;
        }
    }

    let mut negative_witnesses = 0usize;
    for negative in &vectors.negative_witness_cases {
        assert!(
            case_ids.contains(negative.base_case_id.as_str()),
            "{}: unknown baseCaseId",
            negative.id
        );

        let actual_error = parse_witness_signature(&negative.witness_signature_hex, &negative.id)
            .expect_err("negative witness signature was accepted");
        assert_eq!(
            negative.expected_error, actual_error,
            "{}: negative witness parser error mismatch",
            negative.id
        );
        negative_witnesses += 1;
    }

    assert_eq!(verified, 32);
    assert!(
        sighash_types.contains(&SIGHASH_DEFAULT),
        "missing required SIGHASH_DEFAULT vector"
    );
    assert!(
        sighash_types.contains(&SIGHASH_ALL),
        "missing required SIGHASH_ALL vector"
    );
    assert!(
        witness_sighash_types.contains(&SIGHASH_DEFAULT),
        "missing required SIGHASH_DEFAULT witness vector"
    );
    assert!(
        witness_sighash_types.contains(&SIGHASH_ALL),
        "missing required SIGHASH_ALL witness vector"
    );
    assert_eq!(negative_witnesses, 4);
}
