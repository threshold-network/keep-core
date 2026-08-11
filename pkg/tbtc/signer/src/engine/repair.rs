//! Offline-authorized repair of one lost FROST signing share.
//!
//! This module implements the small secp256k1 repair arithmetic locally so
//! complete Delta/Sigma scalar sets never enter Copy-typed upstream repair
//! wrappers or cross the FFI boundary. It also supplies the protocol boundary:
//! a signed, expiring context and transport roster; exact helper-set and
//! endpoint binding; public-package commitment checks; and an atomic install
//! into the descriptor-bound signer store.

use super::*;

use ed25519_dalek::{Signature, VerifyingKey};
use hkdf::Hkdf;
use k256::{
    ecdh::{diffie_hellman, EphemeralSecret},
    elliptic_curve::{sec1::ToEncodedPoint, PrimeField},
    PublicKey as RepairPublicKey, Scalar as RepairScalar, SecretKey as RepairSecretKey,
};
use zeroize::ZeroizeOnDrop;

pub(crate) const TBTC_SIGNER_SHARE_REPAIR_AUTHORIZATION_SCHEMA: &str =
    "tbtc-frost-share-repair-authorization/v1";
pub(crate) const TBTC_SIGNER_SHARE_REPAIR_TRANSPORT_ROSTER_SCHEMA: &str =
    "tbtc-frost-share-repair-transport-roster/v1";
pub(crate) const TBTC_SIGNER_SHARE_REPAIR_INSTALL_RESULT_SCHEMA: &str =
    "tbtc-frost-share-repair-install-result/v1";

const SHARE_REPAIR_AUTHORIZATION_DOMAIN: &[u8] = b"tbtc-frost-share-repair-authorization/v1\0";
const SHARE_REPAIR_TRANSPORT_ROSTER_DOMAIN: &[u8] =
    b"tbtc-frost-share-repair-transport-roster/v1\0";
const SHARE_REPAIR_TRANSPORT_SECRET_DERIVATION_DOMAIN: &[u8] =
    b"tbtc-frost-share-repair-transport-secret/v1\0";
// A signed authorization and transport roster define one replayable Part1
// plaintext transcript. This derivation is frozen for transport v1. Any
// algorithm or domain change must also bump the transport/AAD/ABI version and
// require a uniform launch; otherwise two signer versions could disagree about
// a sender/recipient slot while accepting the same recovery bundle. The frozen
// known-answer test below catches accidental drift within v1.
const SHARE_REPAIR_PART1_DELTA_DERIVATION_DOMAIN: &[u8] =
    b"tbtc-frost-share-repair-part1-delta/v1\0";
const SHARE_REPAIR_MAX_AUTHORIZATION_LIFETIME_SECONDS: u64 = 24 * 60 * 60;
const SHARE_REPAIR_TRANSPORT_VERSION: u8 = 1;
const SHARE_REPAIR_TRANSPORT_KDF_DOMAIN: &[u8] = b"tbtc-frost-share-repair-kdf/v1\0";
const SHARE_REPAIR_TRANSPORT_AAD_DOMAIN: &[u8] = b"tbtc-frost-share-repair-aad/v1\0";
const SHARE_REPAIR_TRANSPORT_PUBLIC_KEY_BYTES: usize = 33;
const SHARE_REPAIR_TRANSPORT_NONCE_BYTES: usize = 24;
const SHARE_REPAIR_TRANSPORT_SCALAR_BYTES: usize = 32;
const SHARE_REPAIR_TRANSPORT_TAG_BYTES: usize = 16;
pub(crate) const SHARE_REPAIR_TRANSPORT_PAYLOAD_BYTES: usize =
    SHARE_REPAIR_TRANSPORT_PUBLIC_KEY_BYTES
        + SHARE_REPAIR_TRANSPORT_NONCE_BYTES
        + SHARE_REPAIR_TRANSPORT_SCALAR_BYTES
        + SHARE_REPAIR_TRANSPORT_TAG_BYTES;
const SHARE_REPAIR_MAX_LIVE_TRANSPORT_SESSIONS: usize = 256;

/// The pinned hkdf crate does not zeroize its PRK/HMAC state. Wrap the
/// short-lived context and wipe its complete in-place representation after
/// expansion. It owns no external allocation and has no Drop implementation
/// in the pinned 0.12.4 release.
struct ZeroizingHkdfSha256(Hkdf<Sha256>);

impl ZeroizingHkdfSha256 {
    fn new(salt: &[u8], input_key_material: &[u8]) -> Self {
        Self(Hkdf::<Sha256>::new(Some(salt), input_key_material))
    }

    fn expand(&self, info: &[u8], output: &mut [u8]) -> Result<(), EngineError> {
        self.0
            .expand(info, output)
            .map_err(|_| EngineError::Internal("share-repair HKDF expansion failed".to_string()))
    }
}

impl Drop for ZeroizingHkdfSha256 {
    fn drop(&mut self) {
        // SAFETY: hkdf 0.12.4's Hkdf<Sha256> is an owned, fixed-size value with
        // no Drop implementation or external allocation. It is never used
        // after this wrapper's Drop starts. Volatile zeroization prevents the
        // compiler from eliding the wipe of its secret PRK/HMAC state.
        unsafe {
            std::slice::from_raw_parts_mut(
                (&mut self.0 as *mut Hkdf<Sha256>).cast::<u8>(),
                std::mem::size_of::<Hkdf<Sha256>>(),
            )
            .zeroize();
        }
    }
}

#[derive(Clone, Copy, Eq, Hash, PartialEq)]
pub(crate) struct ShareRepairTransportSessionKey {
    authorization_digest: [u8; 32],
    participant_identifier: u16,
}

pub(crate) struct ShareRepairTransportSession {
    secret_key: RepairSecretKey,
    expires_at_unix: u64,
    /// Set on first use after the offline authority signs a roster. A live
    /// native key cannot be reused under a second roster for the same
    /// authorization and seat.
    transport_roster_digest: Option<[u8; 32]>,
}

/// A repair scalar with a single, non-Copy, byte-backed owner whose storage is
/// wiped on every success and error path. k256's arithmetic scalar must itself
/// be Copy, so it exists only in narrowly scoped `Zeroizing` temporaries;
/// persistent repair values are never represented by a Copy scalar type.
struct SecretRepairScalar(Zeroizing<[u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]>);

impl std::fmt::Debug for SecretRepairScalar {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.write_str("<redacted repair scalar>")
    }
}

impl Zeroize for SecretRepairScalar {
    fn zeroize(&mut self) {
        self.0.zeroize();
    }
}

impl ZeroizeOnDrop for SecretRepairScalar {}

impl Drop for SecretRepairScalar {
    fn drop(&mut self) {
        self.zeroize();
    }
}

impl SecretRepairScalar {
    fn zero() -> Self {
        Self(Zeroizing::new([0u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]))
    }

    fn deserialize(bytes: &[u8]) -> Result<Self, EngineError> {
        if bytes.len() != SHARE_REPAIR_TRANSPORT_SCALAR_BYTES {
            return Err(EngineError::Validation(
                "repair scalar must contain exactly 32 bytes".to_string(),
            ));
        }
        let mut representation = Zeroizing::new(k256::FieldBytes::default());
        representation.copy_from_slice(bytes);
        let _scalar = Zeroizing::new(
            Option::<RepairScalar>::from(RepairScalar::from_repr(*representation)).ok_or_else(
                || EngineError::Validation("repair scalar is not canonical".to_string()),
            )?,
        );
        let mut owned = Zeroizing::new([0u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]);
        owned.copy_from_slice(bytes);
        Ok(Self(owned))
    }

    fn serialize(&self) -> Zeroizing<[u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]> {
        let mut bytes = Zeroizing::new([0u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]);
        bytes.copy_from_slice(self.0.as_ref());
        bytes
    }

    fn to_scalar(&self) -> Zeroizing<RepairScalar> {
        let mut representation = Zeroizing::new(k256::FieldBytes::default());
        representation.copy_from_slice(self.0.as_ref());
        Zeroizing::new(
            Option::<RepairScalar>::from(RepairScalar::from_repr(*representation))
                .expect("SecretRepairScalar invariant: stored bytes are canonical"),
        )
    }

    fn from_scalar(scalar: &RepairScalar) -> Self {
        let representation = Zeroizing::new(scalar.to_repr());
        let mut bytes = Zeroizing::new([0u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]);
        bytes.copy_from_slice(representation.as_slice());
        Self(bytes)
    }

    fn add_assign(&mut self, other: &Self) {
        let left = self.to_scalar();
        let right = other.to_scalar();
        let result = Zeroizing::new(*left + *right);
        let replacement = Self::from_scalar(&result);
        self.zeroize();
        self.0.copy_from_slice(replacement.0.as_ref());
    }

    fn subtract_assign(&mut self, other: &Self) {
        let left = self.to_scalar();
        let right = other.to_scalar();
        let result = Zeroizing::new(*left - *right);
        let replacement = Self::from_scalar(&result);
        self.zeroize();
        self.0.copy_from_slice(replacement.0.as_ref());
    }

    fn multiply_public(&self, public: RepairScalar) -> Self {
        let secret = self.to_scalar();
        let product = Zeroizing::new(*secret * public);
        Self::from_scalar(&product)
    }

    fn verifying_share_encoding(
        &self,
        operation: &str,
    ) -> Result<[u8; SHARE_REPAIR_TRANSPORT_PUBLIC_KEY_BYTES], EngineError> {
        let secret = self.to_scalar();
        let public = (k256::ProjectivePoint::GENERATOR * *secret).to_affine();
        let encoded = public.to_encoded_point(true);
        let mut bytes = [0u8; SHARE_REPAIR_TRANSPORT_PUBLIC_KEY_BYTES];
        if encoded.as_bytes().len() != bytes.len() {
            return Err(validation_error(
                operation,
                "reconstructed signing share derives the identity element",
            ));
        }
        bytes.copy_from_slice(encoded.as_bytes());
        Ok(bytes)
    }
}

#[derive(Clone, Copy)]
enum ShareRepairEnvelopeKind {
    Delta = 1,
    Sigma = 2,
}

#[derive(Clone, Copy)]
enum ShareRepairTransportRole {
    Helper = 1,
    Target = 2,
}

struct ShareRepairEnvelopeContext<'a> {
    kind: ShareRepairEnvelopeKind,
    authorization_digest: [u8; 32],
    transport_roster_digest: [u8; 32],
    sender_identifier: u16,
    recipient_identifier: u16,
    ephemeral_public_key: &'a [u8],
    sender_public_key: &'a [u8],
    recipient_public_key: &'a [u8],
}

impl ShareRepairEnvelopeKind {
    fn label(self) -> &'static str {
        match self {
            Self::Delta => "delta",
            Self::Sigma => "sigma",
        }
    }
}

#[cfg(test)]
static TEST_SHARE_REPAIR_AUTHORITY: OnceLock<Mutex<Option<[u8; 32]>>> = OnceLock::new();

#[cfg(test)]
pub(crate) fn set_share_repair_authority_for_tests(public_key: Option<[u8; 32]>) {
    *TEST_SHARE_REPAIR_AUTHORITY
        .get_or_init(|| Mutex::new(None))
        .lock()
        .expect("share-repair test authority lock") = public_key;
}

#[derive(Clone)]
struct ValidatedShareRepairAuthorization {
    digest: [u8; 32],
    wallet_id: [u8; 32],
    compressed_key_group: [u8; 33],
    public_key_package_commitment: [u8; 32],
    target_identifier: frost::Identifier,
    helper_identifiers: Vec<frost::Identifier>,
    new_store_fingerprint: [u8; 32],
}

struct ValidatedShareRepairTransportEndpoint {
    store_fingerprint: [u8; 32],
    public_key: RepairPublicKey,
}

struct ValidatedShareRepairTransportRoster {
    digest: [u8; 32],
    endpoints: BTreeMap<u16, ValidatedShareRepairTransportEndpoint>,
}

fn validation_error(operation: &str, detail: impl std::fmt::Display) -> EngineError {
    EngineError::Validation(format!("{operation}: {detail}"))
}

fn share_repair_transport_session_key(
    authorization_digest: [u8; 32],
    participant_identifier: u16,
) -> ShareRepairTransportSessionKey {
    ShareRepairTransportSessionKey {
        authorization_digest,
        participant_identifier,
    }
}

fn derive_share_repair_transport_secret(
    authorization_digest: [u8; 32],
    participant_identifier: u16,
    role: ShareRepairTransportRole,
    store_fingerprint: [u8; 32],
) -> Result<RepairSecretKey, EngineError> {
    let key_material = state_encryption_key_material()?;
    let key_provider = key_material.key_provider.as_bytes();
    let key_id = key_material.key_id.as_bytes();
    let key_provider_length = u16::try_from(key_provider.len()).map_err(|_| {
        EngineError::Internal("state-key provider name exceeds u16::MAX".to_string())
    })?;
    let key_id_length = u16::try_from(key_id.len())
        .map_err(|_| EngineError::Internal("state-key identifier exceeds u16::MAX".to_string()))?;
    let kdf = ZeroizingHkdfSha256::new(
        SHARE_REPAIR_TRANSPORT_SECRET_DERIVATION_DOMAIN,
        key_material.key.as_ref(),
    );
    for counter in 0..=u32::MAX {
        let mut info = Vec::with_capacity(
            SHARE_REPAIR_TRANSPORT_SECRET_DERIVATION_DOMAIN.len()
                + authorization_digest.len()
                + 2
                + 1
                + store_fingerprint.len()
                + 2
                + key_provider.len()
                + 2
                + key_id.len()
                + 4,
        );
        info.extend_from_slice(SHARE_REPAIR_TRANSPORT_SECRET_DERIVATION_DOMAIN);
        info.extend_from_slice(&authorization_digest);
        info.extend_from_slice(&participant_identifier.to_be_bytes());
        info.push(role as u8);
        info.extend_from_slice(&store_fingerprint);
        info.extend_from_slice(&key_provider_length.to_be_bytes());
        info.extend_from_slice(key_provider);
        info.extend_from_slice(&key_id_length.to_be_bytes());
        info.extend_from_slice(key_id);
        info.extend_from_slice(&counter.to_be_bytes());
        let mut candidate = Zeroizing::new([0u8; 32]);
        kdf.expand(&info, candidate.as_mut())?;
        if let Ok(secret_key) = RepairSecretKey::from_slice(candidate.as_ref()) {
            return Ok(secret_key);
        }
    }
    Err(EngineError::Internal(
        "share-repair transport key derivation exhausted its counter".to_string(),
    ))
}

fn derive_share_repair_part1_delta(
    transport_secret: &RepairSecretKey,
    authorization_digest: [u8; 32],
    transport_roster_digest: [u8; 32],
    sender_identifier: u16,
    recipient_identifier: u16,
) -> Result<SecretRepairScalar, EngineError> {
    // The transport secret is itself authorization-, role-, store-, provider-,
    // and state-root-bound. Keying the per-slot PRF with it makes an exact
    // signed bundle replay the same plaintext row after Finish/restart while a
    // root or roster change produces an independent transcript. Each slot is
    // sampled independently so code changes cannot shift a sequential DRBG and
    // silently change every later recipient.
    let transport_secret_bytes = Zeroizing::new(transport_secret.to_bytes());
    let kdf = ZeroizingHkdfSha256::new(
        SHARE_REPAIR_PART1_DELTA_DERIVATION_DOMAIN,
        transport_secret_bytes.as_slice(),
    );
    for counter in 0..=u32::MAX {
        let mut info = Zeroizing::new(Vec::with_capacity(
            SHARE_REPAIR_PART1_DELTA_DERIVATION_DOMAIN.len()
                + 1
                + authorization_digest.len()
                + transport_roster_digest.len()
                + 2
                + 2
                + 4,
        ));
        info.extend_from_slice(SHARE_REPAIR_PART1_DELTA_DERIVATION_DOMAIN);
        info.push(SHARE_REPAIR_TRANSPORT_VERSION);
        info.extend_from_slice(&authorization_digest);
        info.extend_from_slice(&transport_roster_digest);
        info.extend_from_slice(&sender_identifier.to_be_bytes());
        info.extend_from_slice(&recipient_identifier.to_be_bytes());
        info.extend_from_slice(&counter.to_be_bytes());
        let mut candidate = Zeroizing::new([0u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]);
        kdf.expand(info.as_slice(), candidate.as_mut())?;
        if let Ok(delta) = SecretRepairScalar::deserialize(candidate.as_ref()) {
            return Ok(delta);
        }
    }
    Err(EngineError::Internal(
        "share-repair Part1 delta derivation exhausted its counter".to_string(),
    ))
}

fn ensure_current_share_repair_transport_key(
    operation: &str,
    expected_public_key: &RepairPublicKey,
    authorization_digest: [u8; 32],
    participant_identifier: u16,
    role: ShareRepairTransportRole,
    store_fingerprint: [u8; 32],
) -> Result<(), EngineError> {
    // Provider commands must never run while ENGINE_STATE is locked. Re-resolve
    // after the cryptographic work and before releasing any ciphertext or
    // performing Install's irreversible persistence. This is the operation's
    // fail-closed key-generation linearization point: if the provider/root
    // changed while work was in flight, its result is discarded. Production
    // provider rotation is coupled to signer restart.
    let current = derive_share_repair_transport_secret(
        authorization_digest,
        participant_identifier,
        role,
        store_fingerprint,
    )?;
    if current.public_key() != *expected_public_key {
        return Err(validation_error(
            operation,
            "native repair transport key changed while the operation was in flight",
        ));
    }
    Ok(())
}

fn require_authorized_repair_participant(
    operation: &str,
    authorization: &ShareRepairAuthorization,
    participant_identifier: u16,
) -> Result<(), EngineError> {
    if participant_identifier != authorization.target_identifier
        && authorization
            .helper_identifiers
            .binary_search(&participant_identifier)
            .is_err()
    {
        return Err(validation_error(
            operation,
            "participant_identifier is not in the authorized repair set",
        ));
    }
    Ok(())
}

fn canonical_repair_public_key(
    operation: &str,
    field: &str,
    value: &str,
) -> Result<RepairPublicKey, EngineError> {
    if value.len() != SHARE_REPAIR_TRANSPORT_PUBLIC_KEY_BYTES * 2
        || value.bytes().any(|byte| byte.is_ascii_uppercase())
    {
        return Err(validation_error(
            operation,
            format!("{field} must be canonical lowercase 33-byte compressed SEC1 hex"),
        ));
    }
    let bytes = hex::decode(value)
        .map_err(|error| validation_error(operation, format!("invalid {field}: {error}")))?;
    let public_key = RepairPublicKey::from_sec1_bytes(&bytes)
        .map_err(|_| validation_error(operation, format!("invalid {field}")))?;
    if public_key.to_encoded_point(true).as_bytes() != bytes {
        return Err(validation_error(
            operation,
            format!("{field} is not canonical compressed SEC1"),
        ));
    }
    Ok(public_key)
}

fn share_repair_envelope_binding(
    domain: &[u8],
    context: &ShareRepairEnvelopeContext<'_>,
) -> Vec<u8> {
    let mut binding = Vec::with_capacity(
        domain.len()
            + 1
            + 1
            + context.authorization_digest.len()
            + context.transport_roster_digest.len()
            + 2
            + 2
            + context.ephemeral_public_key.len()
            + context.sender_public_key.len()
            + context.recipient_public_key.len(),
    );
    binding.extend_from_slice(domain);
    binding.push(SHARE_REPAIR_TRANSPORT_VERSION);
    binding.push(context.kind as u8);
    binding.extend_from_slice(&context.authorization_digest);
    binding.extend_from_slice(&context.transport_roster_digest);
    binding.extend_from_slice(&context.sender_identifier.to_be_bytes());
    binding.extend_from_slice(&context.recipient_identifier.to_be_bytes());
    binding.extend_from_slice(context.ephemeral_public_key);
    binding.extend_from_slice(context.sender_public_key);
    binding.extend_from_slice(context.recipient_public_key);
    binding
}

fn share_repair_envelope_key(
    ephemeral_shared_secret: &k256::ecdh::SharedSecret,
    authenticated_sender_shared_secret: &k256::ecdh::SharedSecret,
    context: &ShareRepairEnvelopeContext<'_>,
) -> Result<Zeroizing<[u8; 32]>, EngineError> {
    let info = share_repair_envelope_binding(SHARE_REPAIR_TRANSPORT_KDF_DOMAIN, context);
    let mut input_key_material = Zeroizing::new([0u8; 64]);
    input_key_material[..32].copy_from_slice(ephemeral_shared_secret.raw_secret_bytes().as_ref());
    input_key_material[32..].copy_from_slice(
        authenticated_sender_shared_secret
            .raw_secret_bytes()
            .as_ref(),
    );
    let kdf = ZeroizingHkdfSha256::new(
        SHARE_REPAIR_TRANSPORT_KDF_DOMAIN,
        input_key_material.as_ref(),
    );
    let mut key = Zeroizing::new([0u8; 32]);
    kdf.expand(&info, key.as_mut())?;
    Ok(key)
}

#[allow(clippy::too_many_arguments)]
fn encrypt_repair_scalar(
    scalar: &SecretRepairScalar,
    sender_secret_key: &RepairSecretKey,
    recipient_public_key: &RepairPublicKey,
    kind: ShareRepairEnvelopeKind,
    context_digest: [u8; 32],
    transport_roster_digest: [u8; 32],
    sender_identifier: u16,
    recipient_identifier: u16,
    rng: &mut (impl RngCore + CryptoRng),
) -> Result<String, EngineError> {
    let ephemeral_secret = EphemeralSecret::random(rng);
    let ephemeral_public_key = ephemeral_secret.public_key();
    let ephemeral_public_key_bytes = ephemeral_public_key.to_encoded_point(true);
    let sender_public_key = sender_secret_key.public_key();
    let sender_public_key_bytes = sender_public_key.to_encoded_point(true);
    let recipient_public_key_bytes = recipient_public_key.to_encoded_point(true);
    let ephemeral_shared_secret = ephemeral_secret.diffie_hellman(recipient_public_key);
    let sender_secret_scalar = Zeroizing::new(sender_secret_key.to_nonzero_scalar());
    let authenticated_sender_shared_secret =
        diffie_hellman(&*sender_secret_scalar, recipient_public_key.as_affine());
    let envelope_context = ShareRepairEnvelopeContext {
        kind,
        authorization_digest: context_digest,
        transport_roster_digest,
        sender_identifier,
        recipient_identifier,
        ephemeral_public_key: ephemeral_public_key_bytes.as_bytes(),
        sender_public_key: sender_public_key_bytes.as_bytes(),
        recipient_public_key: recipient_public_key_bytes.as_bytes(),
    };
    let key = share_repair_envelope_key(
        &ephemeral_shared_secret,
        &authenticated_sender_shared_secret,
        &envelope_context,
    )?;
    let cipher = XChaCha20Poly1305::new_from_slice(key.as_ref()).map_err(|_| {
        EngineError::Internal("share-repair transport cipher initialization failed".to_string())
    })?;
    let mut nonce = [0u8; SHARE_REPAIR_TRANSPORT_NONCE_BYTES];
    rng.fill_bytes(&mut nonce);
    let aad = share_repair_envelope_binding(SHARE_REPAIR_TRANSPORT_AAD_DOMAIN, &envelope_context);
    let plaintext = scalar.serialize();
    let ciphertext = cipher
        .encrypt(
            XNonce::from_slice(&nonce),
            Payload {
                msg: plaintext.as_ref(),
                aad: &aad,
            },
        )
        .map_err(|_| EngineError::Internal("share-repair encryption failed".to_string()))?;
    let mut payload = Vec::with_capacity(SHARE_REPAIR_TRANSPORT_PAYLOAD_BYTES);
    payload.extend_from_slice(ephemeral_public_key_bytes.as_bytes());
    payload.extend_from_slice(&nonce);
    payload.extend_from_slice(&ciphertext);
    if payload.len() != SHARE_REPAIR_TRANSPORT_PAYLOAD_BYTES {
        return Err(EngineError::Internal(
            "share-repair cipher returned an unexpected payload length".to_string(),
        ));
    }
    Ok(hex::encode(payload))
}

#[allow(clippy::too_many_arguments)]
fn decrypt_repair_scalar(
    recipient_secret_key: &RepairSecretKey,
    sender_public_key: &RepairPublicKey,
    payload_hex: &str,
    kind: ShareRepairEnvelopeKind,
    context_digest: [u8; 32],
    transport_roster_digest: [u8; 32],
    sender_identifier: u16,
    recipient_identifier: u16,
) -> Result<SecretRepairScalar, EngineError> {
    let operation = match kind {
        ShareRepairEnvelopeKind::Delta => "share_repair_part2",
        ShareRepairEnvelopeKind::Sigma => "install_repaired_share",
    };
    if payload_hex.len() != SHARE_REPAIR_TRANSPORT_PAYLOAD_BYTES * 2
        || payload_hex.bytes().any(|byte| byte.is_ascii_uppercase())
    {
        return Err(validation_error(
            operation,
            format!(
                "{} payload must be canonical lowercase {}-byte hex",
                kind.label(),
                SHARE_REPAIR_TRANSPORT_PAYLOAD_BYTES
            ),
        ));
    }
    let payload = hex::decode(payload_hex)
        .map_err(|_| validation_error(operation, format!("invalid {} payload", kind.label())))?;
    let (ephemeral_public_key_bytes, remaining) =
        payload.split_at(SHARE_REPAIR_TRANSPORT_PUBLIC_KEY_BYTES);
    let (nonce, ciphertext) = remaining.split_at(SHARE_REPAIR_TRANSPORT_NONCE_BYTES);
    let ephemeral_public_key = RepairPublicKey::from_sec1_bytes(ephemeral_public_key_bytes)
        .map_err(|_| {
            validation_error(
                operation,
                format!("invalid {} ephemeral public key", kind.label()),
            )
        })?;
    if ephemeral_public_key.to_encoded_point(true).as_bytes() != ephemeral_public_key_bytes {
        return Err(validation_error(
            operation,
            format!("non-canonical {} ephemeral public key", kind.label()),
        ));
    }
    let recipient_public_key = recipient_secret_key.public_key();
    let sender_public_key_bytes = sender_public_key.to_encoded_point(true);
    let recipient_public_key_bytes = recipient_public_key.to_encoded_point(true);
    let secret_scalar = Zeroizing::new(recipient_secret_key.to_nonzero_scalar());
    let ephemeral_shared_secret = diffie_hellman(&*secret_scalar, ephemeral_public_key.as_affine());
    let authenticated_sender_shared_secret =
        diffie_hellman(&*secret_scalar, sender_public_key.as_affine());
    let envelope_context = ShareRepairEnvelopeContext {
        kind,
        authorization_digest: context_digest,
        transport_roster_digest,
        sender_identifier,
        recipient_identifier,
        ephemeral_public_key: ephemeral_public_key_bytes,
        sender_public_key: sender_public_key_bytes.as_bytes(),
        recipient_public_key: recipient_public_key_bytes.as_bytes(),
    };
    let key = share_repair_envelope_key(
        &ephemeral_shared_secret,
        &authenticated_sender_shared_secret,
        &envelope_context,
    )?;
    let cipher = XChaCha20Poly1305::new_from_slice(key.as_ref()).map_err(|_| {
        EngineError::Internal("share-repair transport cipher initialization failed".to_string())
    })?;
    let aad = share_repair_envelope_binding(SHARE_REPAIR_TRANSPORT_AAD_DOMAIN, &envelope_context);
    let plaintext = Zeroizing::new(
        cipher
            .decrypt(
                XNonce::from_slice(nonce),
                Payload {
                    msg: ciphertext,
                    aad: &aad,
                },
            )
            .map_err(|_| {
                validation_error(
                    operation,
                    format!("{} payload authentication failed", kind.label()),
                )
            })?,
    );
    SecretRepairScalar::deserialize(plaintext.as_ref()).map_err(|_| {
        validation_error(
            operation,
            format!("{} payload contains an invalid scalar", kind.label()),
        )
    })
}

#[cfg(test)]
pub(crate) fn share_repair_delta_plaintext_for_tests(
    authorization: &ShareRepairAuthorization,
    transport_roster: &ShareRepairTransportRoster,
    delta: &ShareRepairDelta,
) -> Result<Zeroizing<[u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]>, EngineError> {
    const OP: &str = "share_repair_delta_plaintext_for_tests";
    let validated = validate_share_repair_authorization(OP, authorization, true)?;
    let transport_roster =
        validate_share_repair_transport_roster(OP, authorization, &validated, transport_roster)?;
    if delta.context_digest != bytes32_hex(validated.digest)
        || authorization
            .helper_identifiers
            .binary_search(&delta.sender_identifier)
            .is_err()
        || authorization
            .helper_identifiers
            .binary_search(&delta.recipient_identifier)
            .is_err()
    {
        return Err(validation_error(OP, "delta has invalid routing bindings"));
    }
    let local_store_fingerprint = durable_store_identity()?.fingerprint;
    let recipient_endpoint = transport_roster
        .endpoints
        .get(&delta.recipient_identifier)
        .ok_or_else(|| validation_error(OP, "recipient endpoint is missing"))?;
    if recipient_endpoint.store_fingerprint != local_store_fingerprint {
        return Err(validation_error(
            OP,
            "recipient endpoint does not use the active test store",
        ));
    }
    let recipient_secret = derive_share_repair_transport_secret(
        validated.digest,
        delta.recipient_identifier,
        ShareRepairTransportRole::Helper,
        local_store_fingerprint,
    )?;
    if recipient_secret.public_key() != recipient_endpoint.public_key {
        return Err(validation_error(
            OP,
            "recipient endpoint does not match its derived test key",
        ));
    }
    let sender_endpoint = transport_roster
        .endpoints
        .get(&delta.sender_identifier)
        .ok_or_else(|| validation_error(OP, "sender endpoint is missing"))?;
    Ok(decrypt_repair_scalar(
        &recipient_secret,
        &sender_endpoint.public_key,
        &delta.payload_hex,
        ShareRepairEnvelopeKind::Delta,
        validated.digest,
        transport_roster.digest,
        delta.sender_identifier,
        delta.recipient_identifier,
    )?
    .serialize())
}

#[cfg(test)]
pub(crate) fn corrupt_share_repair_delta_plaintext_for_tests(
    authorization: &ShareRepairAuthorization,
    transport_roster: &ShareRepairTransportRoster,
    delta: &ShareRepairDelta,
) -> Result<ShareRepairDelta, EngineError> {
    const OP: &str = "corrupt_share_repair_delta_plaintext_for_tests";
    let plaintext = share_repair_delta_plaintext_for_tests(authorization, transport_roster, delta)?;
    let mut altered = SecretRepairScalar::deserialize(plaintext.as_ref())?;
    let mut one_bytes = Zeroizing::new([0u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]);
    one_bytes[SHARE_REPAIR_TRANSPORT_SCALAR_BYTES - 1] = 1;
    let one = SecretRepairScalar::deserialize(one_bytes.as_ref())?;
    altered.add_assign(&one);

    let validated = validate_share_repair_authorization(OP, authorization, true)?;
    let transport_roster =
        validate_share_repair_transport_roster(OP, authorization, &validated, transport_roster)?;
    let local_store_fingerprint = durable_store_identity()?.fingerprint;
    let sender_endpoint = transport_roster
        .endpoints
        .get(&delta.sender_identifier)
        .ok_or_else(|| validation_error(OP, "sender endpoint is missing"))?;
    if sender_endpoint.store_fingerprint != local_store_fingerprint {
        return Err(validation_error(
            OP,
            "sender endpoint does not use the active test store",
        ));
    }
    let sender_secret = derive_share_repair_transport_secret(
        validated.digest,
        delta.sender_identifier,
        ShareRepairTransportRole::Helper,
        local_store_fingerprint,
    )?;
    if sender_secret.public_key() != sender_endpoint.public_key {
        return Err(validation_error(
            OP,
            "sender endpoint does not match its derived test key",
        ));
    }
    let recipient_endpoint = transport_roster
        .endpoints
        .get(&delta.recipient_identifier)
        .ok_or_else(|| validation_error(OP, "recipient endpoint is missing"))?;
    let mut rng = zeroizing_rng_from_os();
    Ok(ShareRepairDelta {
        context_digest: delta.context_digest.clone(),
        sender_identifier: delta.sender_identifier,
        recipient_identifier: delta.recipient_identifier,
        payload_hex: encrypt_repair_scalar(
            &altered,
            &sender_secret,
            &recipient_endpoint.public_key,
            ShareRepairEnvelopeKind::Delta,
            validated.digest,
            transport_roster.digest,
            delta.sender_identifier,
            delta.recipient_identifier,
            &mut rng,
        )?,
    })
}

fn write_length_prefixed(digest: &mut Sha256, value: &[u8]) -> Result<(), EngineError> {
    let length = u32::try_from(value.len()).map_err(|_| {
        EngineError::Validation(
            "share-repair authorization contains a field longer than u32::MAX".to_string(),
        )
    })?;
    digest.update(length.to_be_bytes());
    digest.update(value);
    Ok(())
}

fn require_nonzero_bytes32(value: [u8; 32], label: &str) -> Result<[u8; 32], EngineError> {
    if value == [0u8; 32] {
        return Err(EngineError::Validation(format!("{label} must not be zero")));
    }
    Ok(value)
}

#[allow(clippy::too_many_arguments)]
fn share_repair_authorization_signing_digest(
    authorization: &ShareRepairAuthorization,
    wallet_id: [u8; 32],
    compressed_key_group: [u8; 33],
    public_key_package_commitment: [u8; 32],
    old_store_fingerprint: [u8; 32],
    new_store_fingerprint: [u8; 32],
    nonce: [u8; 32],
) -> Result<[u8; 32], EngineError> {
    let mut transcript = Sha256::new();
    transcript.update(SHARE_REPAIR_AUTHORIZATION_DOMAIN);
    write_length_prefixed(&mut transcript, authorization.session_id.as_bytes())?;
    transcript.update(wallet_id);
    transcript.update(compressed_key_group);
    transcript.update(public_key_package_commitment);
    transcript.update(authorization.target_identifier.to_be_bytes());
    transcript.update(
        u16::try_from(authorization.helper_identifiers.len())
            .map_err(|_| {
                EngineError::Validation(
                    "share-repair helper count does not fit the signing transcript".to_string(),
                )
            })?
            .to_be_bytes(),
    );
    for helper in &authorization.helper_identifiers {
        transcript.update(helper.to_be_bytes());
    }
    transcript.update(authorization.threshold.to_be_bytes());
    transcript.update(authorization.participant_count.to_be_bytes());
    transcript.update(old_store_fingerprint);
    transcript.update(new_store_fingerprint);
    transcript.update(authorization.recovery_epoch.to_be_bytes());
    transcript.update(authorization.issued_at_unix.to_be_bytes());
    transcript.update(authorization.not_before_unix.to_be_bytes());
    transcript.update(authorization.expires_at_unix.to_be_bytes());
    transcript.update(nonce);
    Ok(transcript.finalize().into())
}

fn enforce_share_repair_authorization_time(
    authorization: &ShareRepairAuthorization,
) -> Result<(), EngineError> {
    let now = now_unix();
    if now == 0 {
        return Err(EngineError::Internal(
            "share-repair authorization: system clock is before UNIX epoch".to_string(),
        ));
    }
    if now < authorization.not_before_unix {
        return Err(EngineError::Validation(format!(
            "share-repair authorization is not valid before [{}]",
            authorization.not_before_unix
        )));
    }
    if now >= authorization.expires_at_unix {
        return Err(EngineError::Validation(format!(
            "share-repair authorization expired at [{}]",
            authorization.expires_at_unix
        )));
    }
    Ok(())
}

fn enforce_share_repair_preparation_time(
    authorization: &ShareRepairAuthorization,
) -> Result<(), EngineError> {
    let now = now_unix();
    if now == 0 {
        return Err(EngineError::Internal(
            "share-repair preparation: system clock is before UNIX epoch".to_string(),
        ));
    }
    if now < authorization.issued_at_unix {
        return Err(validation_error(
            "begin_share_repair_session",
            format!(
                "share-repair authorization is not issued until [{}]",
                authorization.issued_at_unix
            ),
        ));
    }
    if now >= authorization.expires_at_unix {
        return Err(validation_error(
            "begin_share_repair_session",
            format!(
                "share-repair authorization expired at [{}]",
                authorization.expires_at_unix
            ),
        ));
    }
    Ok(())
}

fn configured_share_repair_authority_key(operation: &str) -> Result<[u8; 32], EngineError> {
    #[cfg(test)]
    let test_authority = TEST_SHARE_REPAIR_AUTHORITY
        .get_or_init(|| Mutex::new(None))
        .lock()
        .expect("share-repair test authority lock")
        .as_ref()
        .copied();
    #[cfg(not(test))]
    let test_authority: Option<[u8; 32]> = None;
    if let Some(test_authority) = test_authority {
        return Ok(test_authority);
    }
    let configuration = configured_state_anchor()?.ok_or_else(|| {
        validation_error(
            operation,
            "state-anchor trust configuration is required for share repair",
        )
    })?;
    Ok(configuration
        .trust
        .ok_or_else(|| {
            validation_error(
                operation,
                "offline-authority trust configuration is required for share repair",
            )
        })?
        .offline_authority_public_key)
}

fn validate_share_repair_authorization(
    operation: &str,
    authorization: &ShareRepairAuthorization,
    enforce_time: bool,
) -> Result<ValidatedShareRepairAuthorization, EngineError> {
    if authorization.schema != TBTC_SIGNER_SHARE_REPAIR_AUTHORIZATION_SCHEMA {
        return Err(validation_error(
            operation,
            "unsupported share-repair authorization schema",
        ));
    }
    validate_session_id(&authorization.session_id)?;
    if authorization.threshold < 2
        || authorization.participant_count < authorization.threshold
        || authorization.participant_count > 100
    {
        return Err(validation_error(
            operation,
            format!(
                "threshold [{}] must be between 2 and participant_count [{}], with at most 100 participants",
                authorization.threshold, authorization.participant_count
            ),
        ));
    }
    if authorization.helper_identifiers.len() != authorization.threshold as usize {
        return Err(validation_error(
            operation,
            format!(
                "helper_identifiers must contain exactly threshold [{}] members",
                authorization.threshold
            ),
        ));
    }
    if authorization.target_identifier == 0
        || authorization.target_identifier > authorization.participant_count
    {
        return Err(validation_error(
            operation,
            "target_identifier is outside the participant set",
        ));
    }
    let mut previous_helper = 0u16;
    for helper in &authorization.helper_identifiers {
        if *helper == 0 || *helper > authorization.participant_count {
            return Err(validation_error(
                operation,
                "helper identifier is outside the participant set",
            ));
        }
        if *helper <= previous_helper {
            return Err(validation_error(
                operation,
                "helper_identifiers must be distinct and strictly ascending",
            ));
        }
        if *helper == authorization.target_identifier {
            return Err(validation_error(
                operation,
                "target_identifier must not be a helper",
            ));
        }
        previous_helper = *helper;
    }
    if authorization.recovery_epoch == 0 {
        return Err(validation_error(
            operation,
            "recovery_epoch must be non-zero",
        ));
    }
    if authorization.issued_at_unix > authorization.not_before_unix
        || authorization.not_before_unix >= authorization.expires_at_unix
    {
        return Err(validation_error(
            operation,
            "authorization timestamps are not ordered",
        ));
    }
    let lifetime = authorization
        .expires_at_unix
        .checked_sub(authorization.issued_at_unix)
        .ok_or_else(|| validation_error(operation, "authorization lifetime underflow"))?;
    if lifetime > SHARE_REPAIR_MAX_AUTHORIZATION_LIFETIME_SECONDS {
        return Err(validation_error(
            operation,
            format!(
                "authorization lifetime exceeds [{}] seconds",
                SHARE_REPAIR_MAX_AUTHORIZATION_LIFETIME_SECONDS
            ),
        ));
    }

    let wallet_id = require_nonzero_bytes32(
        parse_canonical_bytes32(&authorization.wallet_id, "wallet_id")?,
        "wallet_id",
    )?;
    let public_key_package_commitment = require_nonzero_bytes32(
        parse_canonical_bytes32(
            &authorization.public_key_package_commitment,
            "public_key_package_commitment",
        )?,
        "public_key_package_commitment",
    )?;
    let old_store_fingerprint = require_nonzero_bytes32(
        parse_canonical_bytes32(
            &authorization.old_store_fingerprint,
            "old_store_fingerprint",
        )?,
        "old_store_fingerprint",
    )?;
    let new_store_fingerprint = require_nonzero_bytes32(
        parse_canonical_bytes32(
            &authorization.new_store_fingerprint,
            "new_store_fingerprint",
        )?,
        "new_store_fingerprint",
    )?;
    if old_store_fingerprint == new_store_fingerprint {
        return Err(validation_error(
            operation,
            "old_store_fingerprint and new_store_fingerprint must differ",
        ));
    }
    let nonce = require_nonzero_bytes32(
        parse_canonical_bytes32(&authorization.nonce, "nonce")?,
        "nonce",
    )?;

    let (derived_wallet_id, compressed_key_group) =
        super::inventory::parse_key_group(&authorization.key_group).map_err(|error| {
            validation_error(
                operation,
                format!("key_group is not canonical compressed SEC1: {error}"),
            )
        })?;
    if derived_wallet_id != wallet_id {
        return Err(validation_error(
            operation,
            "wallet_id does not match key_group",
        ));
    }

    let target_identifier =
        participant_identifier_to_frost_identifier(authorization.target_identifier)?;
    let helper_identifiers = authorization
        .helper_identifiers
        .iter()
        .copied()
        .map(participant_identifier_to_frost_identifier)
        .collect::<Result<Vec<_>, _>>()?;

    let digest = share_repair_authorization_signing_digest(
        authorization,
        wallet_id,
        compressed_key_group,
        public_key_package_commitment,
        old_store_fingerprint,
        new_store_fingerprint,
        nonce,
    )?;

    let authority_public_key = configured_share_repair_authority_key(operation)?;
    let signature = parse_canonical_signature(&authorization.signature_hex)?;
    let verifying_key = VerifyingKey::from_bytes(&authority_public_key).map_err(|error| {
        EngineError::Internal(format!(
            "configured share-repair authority key is invalid: {error}"
        ))
    })?;
    verifying_key
        .verify_strict(&digest, &Signature::from_bytes(&signature))
        .map_err(|_| validation_error(operation, "authorization signature is invalid"))?;

    if enforce_time {
        enforce_share_repair_authorization_time(authorization)?;
    }

    Ok(ValidatedShareRepairAuthorization {
        digest,
        wallet_id,
        compressed_key_group,
        public_key_package_commitment,
        target_identifier,
        helper_identifiers,
        new_store_fingerprint,
    })
}

#[cfg(test)]
pub(crate) fn share_repair_authorization_digest_for_tests(
    authorization: &ShareRepairAuthorization,
) -> Result<[u8; 32], EngineError> {
    let wallet_id = parse_canonical_bytes32(&authorization.wallet_id, "wallet_id")?;
    let (_, compressed_key_group) = super::inventory::parse_key_group(&authorization.key_group)?;
    let public_key_package_commitment = parse_canonical_bytes32(
        &authorization.public_key_package_commitment,
        "public_key_package_commitment",
    )?;
    let old_store_fingerprint = parse_canonical_bytes32(
        &authorization.old_store_fingerprint,
        "old_store_fingerprint",
    )?;
    let new_store_fingerprint = parse_canonical_bytes32(
        &authorization.new_store_fingerprint,
        "new_store_fingerprint",
    )?;
    let nonce = parse_canonical_bytes32(&authorization.nonce, "nonce")?;
    share_repair_authorization_signing_digest(
        authorization,
        wallet_id,
        compressed_key_group,
        public_key_package_commitment,
        old_store_fingerprint,
        new_store_fingerprint,
        nonce,
    )
}

fn share_repair_transport_roster_signing_digest(
    authorization_digest: [u8; 32],
    participant_public_keys: &[(u16, [u8; 32], [u8; SHARE_REPAIR_TRANSPORT_PUBLIC_KEY_BYTES])],
) -> Result<[u8; 32], EngineError> {
    let mut digest = Sha256::new();
    digest.update(SHARE_REPAIR_TRANSPORT_ROSTER_DOMAIN);
    digest.update(authorization_digest);
    digest.update(
        u16::try_from(participant_public_keys.len())
            .map_err(|_| {
                EngineError::Validation(
                    "share-repair transport roster participant count exceeds u16::MAX".to_string(),
                )
            })?
            .to_be_bytes(),
    );
    for (participant_identifier, store_fingerprint, public_key) in participant_public_keys {
        digest.update(participant_identifier.to_be_bytes());
        digest.update(store_fingerprint);
        digest.update(public_key);
    }
    Ok(digest.finalize().into())
}

fn validate_share_repair_transport_roster_shape(
    operation: &str,
    authorization: &ShareRepairAuthorization,
    authorization_digest: [u8; 32],
    roster: &ShareRepairTransportRoster,
) -> Result<
    (
        [u8; 32],
        BTreeMap<u16, ValidatedShareRepairTransportEndpoint>,
    ),
    EngineError,
> {
    if roster.schema != TBTC_SIGNER_SHARE_REPAIR_TRANSPORT_ROSTER_SCHEMA {
        return Err(validation_error(
            operation,
            "unsupported share-repair transport-roster schema",
        ));
    }
    let claimed_authorization_digest = parse_canonical_bytes32(
        &roster.authorization_digest,
        "transport_roster.authorization_digest",
    )?;
    if claimed_authorization_digest != authorization_digest {
        return Err(validation_error(
            operation,
            "transport roster names a different authorization digest",
        ));
    }
    let expected_count = authorization.helper_identifiers.len() + 1;
    if roster.participant_public_keys.len() != expected_count {
        return Err(validation_error(
            operation,
            "transport roster must contain the exact helper set followed by the target",
        ));
    }
    let expected_identifiers = authorization
        .helper_identifiers
        .iter()
        .copied()
        .chain(std::iter::once(authorization.target_identifier));
    let target_store_fingerprint = parse_canonical_bytes32(
        &authorization.new_store_fingerprint,
        "new_store_fingerprint",
    )?;
    let mut endpoints = BTreeMap::new();
    let mut unique_public_keys = HashSet::new();
    let mut transcript_public_keys = Vec::with_capacity(expected_count);
    for (index, (endpoint, expected_identifier)) in roster
        .participant_public_keys
        .iter()
        .zip(expected_identifiers)
        .enumerate()
    {
        if endpoint.participant_identifier != expected_identifier {
            return Err(validation_error(
                operation,
                format!(
                    "transport_roster.participant_public_keys[{index}] has the wrong participant"
                ),
            ));
        }
        let store_fingerprint = parse_canonical_bytes32(
            &endpoint.store_fingerprint,
            &format!("transport_roster.participant_public_keys[{index}].store_fingerprint"),
        )?;
        if expected_identifier == authorization.target_identifier
            && store_fingerprint != target_store_fingerprint
        {
            return Err(validation_error(
                operation,
                "target transport-roster store fingerprint does not match new_store_fingerprint",
            ));
        }
        let public_key = canonical_repair_public_key(
            operation,
            &format!("transport_roster.participant_public_keys[{index}].public_key_hex"),
            &endpoint.public_key_hex,
        )?;
        let encoded = public_key.to_encoded_point(true);
        let mut public_key_bytes = [0u8; SHARE_REPAIR_TRANSPORT_PUBLIC_KEY_BYTES];
        public_key_bytes.copy_from_slice(encoded.as_bytes());
        if !unique_public_keys.insert(public_key_bytes) {
            return Err(validation_error(
                operation,
                "transport roster contains duplicate public keys",
            ));
        }
        endpoints.insert(
            expected_identifier,
            ValidatedShareRepairTransportEndpoint {
                store_fingerprint,
                public_key,
            },
        );
        transcript_public_keys.push((expected_identifier, store_fingerprint, public_key_bytes));
    }
    let signing_digest = share_repair_transport_roster_signing_digest(
        authorization_digest,
        &transcript_public_keys,
    )?;
    Ok((signing_digest, endpoints))
}

fn validate_share_repair_transport_roster(
    operation: &str,
    authorization: &ShareRepairAuthorization,
    validated: &ValidatedShareRepairAuthorization,
    roster: &ShareRepairTransportRoster,
) -> Result<ValidatedShareRepairTransportRoster, EngineError> {
    // The caller has already revalidated the original authorization's exact
    // wall-clock window. Binding this roster to its digest inherits the same
    // issued/not-before/expiry limits without a second, divergent clock.
    let (signing_digest, endpoints) = validate_share_repair_transport_roster_shape(
        operation,
        authorization,
        validated.digest,
        roster,
    )?;
    let signature = parse_canonical_signature(&roster.signature_hex)?;
    let authority_public_key = configured_share_repair_authority_key(operation)?;
    let verifying_key = VerifyingKey::from_bytes(&authority_public_key).map_err(|error| {
        EngineError::Internal(format!(
            "configured share-repair authority key is invalid: {error}"
        ))
    })?;
    verifying_key
        .verify_strict(&signing_digest, &Signature::from_bytes(&signature))
        .map_err(|_| validation_error(operation, "transport-roster signature is invalid"))?;
    Ok(ValidatedShareRepairTransportRoster {
        digest: signing_digest,
        endpoints,
    })
}

#[cfg(test)]
pub(crate) fn share_repair_transport_roster_digest_for_tests(
    authorization: &ShareRepairAuthorization,
    roster: &ShareRepairTransportRoster,
) -> Result<[u8; 32], EngineError> {
    let authorization_digest = share_repair_authorization_digest_for_tests(authorization)?;
    let (signing_digest, _) = validate_share_repair_transport_roster_shape(
        "share_repair_transport_roster_digest_for_tests",
        authorization,
        authorization_digest,
        roster,
    )?;
    Ok(signing_digest)
}

pub(crate) fn begin_share_repair_session(
    request: BeginShareRepairSessionRequest,
) -> Result<BeginShareRepairSessionResult, EngineError> {
    const OP: &str = "begin_share_repair_session";
    enforce_provenance_gate()?;
    let validated = validate_share_repair_authorization(OP, &request.authorization, false)?;
    enforce_share_repair_preparation_time(&request.authorization)?;
    require_authorized_repair_participant(
        OP,
        &request.authorization,
        request.participant_identifier,
    )?;
    let store_fingerprint = durable_store_identity()?.fingerprint;
    let role = if request.participant_identifier == request.authorization.target_identifier {
        if store_fingerprint != validated.new_store_fingerprint {
            return Err(validation_error(
                OP,
                "target repair session must be opened in the authorized new durable store",
            ));
        }
        ShareRepairTransportRole::Target
    } else {
        // A helper transport key is available only in a native signer that
        // demonstrably retains that helper's authorized long-lived share.
        let _ = load_helper_material(
            OP,
            &request.authorization,
            &validated,
            request.participant_identifier,
        )?;
        ShareRepairTransportRole::Helper
    };
    let session_key =
        share_repair_transport_session_key(validated.digest, request.participant_identifier);
    // Cache only a live copy. Deterministic derivation keeps the public key
    // stable across Finish/process restart so an offline authority can sign
    // the roster; rotating the state root or durable store intentionally
    // invalidates that roster.
    let derived_secret = derive_share_repair_transport_secret(
        validated.digest,
        request.participant_identifier,
        role,
        store_fingerprint,
    )?;
    let derived_public = derived_secret.public_key();
    let now = now_unix();
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    guard
        .share_repair_sessions
        .retain(|_, session| session.expires_at_unix > now);
    if let Some(session) = guard.share_repair_sessions.get(&session_key) {
        if session.secret_key.public_key() != derived_public {
            return Err(validation_error(
                OP,
                "cached native repair session conflicts with the derived store-bound key",
            ));
        }
    } else {
        if guard.share_repair_sessions.len() >= SHARE_REPAIR_MAX_LIVE_TRANSPORT_SESSIONS {
            return Err(validation_error(
                OP,
                "live native repair-session limit reached",
            ));
        }
        guard.share_repair_sessions.insert(
            session_key,
            ShareRepairTransportSession {
                secret_key: derived_secret,
                expires_at_unix: request.authorization.expires_at_unix,
                transport_roster_digest: None,
            },
        );
    }
    let session = guard
        .share_repair_sessions
        .get(&session_key)
        .ok_or_else(|| EngineError::Internal(format!("{OP}: native repair session disappeared")))?;
    let public_key = session.secret_key.public_key().to_encoded_point(true);
    let transport_public_key_hex = hex::encode(public_key.as_bytes());
    drop(guard);
    ensure_current_share_repair_transport_key(
        OP,
        &derived_public,
        validated.digest,
        request.participant_identifier,
        role,
        store_fingerprint,
    )?;
    Ok(BeginShareRepairSessionResult {
        context_digest: bytes32_hex(validated.digest),
        participant_identifier: request.participant_identifier,
        store_fingerprint: bytes32_hex(store_fingerprint),
        transport_public_key_hex,
    })
}

pub(crate) fn finish_share_repair_session(
    request: FinishShareRepairSessionRequest,
) -> Result<FinishShareRepairSessionResult, EngineError> {
    const OP: &str = "finish_share_repair_session";
    enforce_provenance_gate()?;
    // Cleanup deliberately verifies the signed context but ignores its wall
    // clock window so a deadline race cannot strand a native private key.
    let validated = validate_share_repair_authorization(OP, &request.authorization, false)?;
    require_authorized_repair_participant(
        OP,
        &request.authorization,
        request.participant_identifier,
    )?;
    let session_key =
        share_repair_transport_session_key(validated.digest, request.participant_identifier);
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    // Report the postcondition rather than whether this particular call found
    // the entry, making cleanup retries genuine successful no-ops. Finish
    // wipes only the live cache; a valid signed authorization can rederive the
    // deterministic key until its expiry boundary.
    guard.share_repair_sessions.remove(&session_key);
    Ok(FinishShareRepairSessionResult {
        context_digest: bytes32_hex(validated.digest),
        participant_identifier: request.participant_identifier,
        finished: true,
    })
}

fn with_share_repair_transport_secret<T>(
    operation: &str,
    authorization_digest: [u8; 32],
    transport_roster_digest: [u8; 32],
    participant_identifier: u16,
    current_root_bound_secret: &RepairSecretKey,
    expected_public_key: &RepairPublicKey,
    use_secret: impl FnOnce(&RepairSecretKey) -> Result<T, EngineError>,
) -> Result<T, EngineError> {
    let now = now_unix();
    let session_key =
        share_repair_transport_session_key(authorization_digest, participant_identifier);
    let mut guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    guard
        .share_repair_sessions
        .retain(|_, session| session.expires_at_unix > now);
    let session = guard
        .share_repair_sessions
        .get_mut(&session_key)
        .ok_or_else(|| {
            validation_error(
                operation,
                format!("no live native repair session for participant [{participant_identifier}]"),
            )
        })?;
    if session.secret_key.public_key() != current_root_bound_secret.public_key() {
        return Err(validation_error(
            operation,
            "cached native repair session does not match the current state-root-bound key",
        ));
    }
    if current_root_bound_secret.public_key() != *expected_public_key {
        return Err(validation_error(
            operation,
            "local native repair session does not match the signed transport roster",
        ));
    }
    match session.transport_roster_digest {
        Some(bound_digest) if bound_digest != transport_roster_digest => {
            return Err(validation_error(
                operation,
                "local native repair session is already bound to a different signed transport roster",
            ));
        }
        Some(_) => {}
        None => session.transport_roster_digest = Some(transport_roster_digest),
    }
    use_secret(&session.secret_key)
}

fn validate_public_key_package(
    operation: &str,
    authorization: &ShareRepairAuthorization,
    validated: &ValidatedShareRepairAuthorization,
    public_key_package: &NativeFrostPublicKeyPackage,
) -> Result<(frost::keys::PublicKeyPackage, frost::keys::PublicKeyPackage), EngineError> {
    let stored_shape = native_public_key_package_to_frost(operation, public_key_package)?;
    if stored_shape.max_signers() != authorization.participant_count {
        return Err(validation_error(
            operation,
            format!(
                "public key package has [{}] participants; expected [{}]",
                stored_shape.max_signers(),
                authorization.participant_count
            ),
        ));
    }
    for identifier in stored_shape.verifying_shares().keys() {
        let participant = frost_identifier_to_u16(*identifier).ok_or_else(|| {
            validation_error(
                operation,
                "public key package contains a non-canonical participant identifier",
            )
        })?;
        if participant == 0 || participant > authorization.participant_count {
            return Err(validation_error(
                operation,
                "public key package identifier is outside the participant set",
            ));
        }
    }
    if !stored_shape
        .verifying_shares()
        .contains_key(&validated.target_identifier)
        || validated
            .helper_identifiers
            .iter()
            .any(|helper| !stored_shape.verifying_shares().contains_key(helper))
    {
        return Err(validation_error(
            operation,
            "public key package does not contain the authorized target and helper set",
        ));
    }

    let serialized_group_key = stored_shape.verifying_key().serialize().map_err(|error| {
        EngineError::Internal(format!(
            "{operation}: failed to serialize public verifying key: {error}"
        ))
    })?;
    if serialized_group_key.as_slice() != validated.compressed_key_group {
        return Err(validation_error(
            operation,
            "public key package verifying key does not match key_group",
        ));
    }
    let serialized_public_package = stored_shape.serialize().map_err(|error| {
        EngineError::Internal(format!(
            "{operation}: failed to serialize public key package: {error}"
        ))
    })?;
    let commitment = public_key_package_commitment(
        &validated.wallet_id,
        &authorization.key_group,
        authorization.threshold,
        authorization.participant_count,
        0,
        &serialized_public_package,
    );
    if commitment != validated.public_key_package_commitment {
        return Err(validation_error(
            operation,
            "public key package does not match its authorized commitment",
        ));
    }

    let repair_shape = frost::keys::PublicKeyPackage::new(
        stored_shape.verifying_shares().clone(),
        *stored_shape.verifying_key(),
        Some(authorization.threshold),
    );
    Ok((stored_shape, repair_shape))
}

fn load_helper_material(
    operation: &str,
    authorization: &ShareRepairAuthorization,
    validated: &ValidatedShareRepairAuthorization,
    helper_identifier: u16,
) -> Result<(frost::keys::KeyPackage, frost::keys::PublicKeyPackage), EngineError> {
    if authorization
        .helper_identifiers
        .binary_search(&helper_identifier)
        .is_err()
    {
        return Err(validation_error(
            operation,
            "helper_identifier is not in the authorized helper set",
        ));
    }
    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let session = guard
        .sessions
        .get(&authorization.session_id)
        .ok_or_else(|| EngineError::SessionNotFound {
            session_id: authorization.session_id.clone(),
        })?;
    let dkg_result = session
        .dkg_result
        .as_ref()
        .ok_or_else(|| EngineError::DkgNotReady {
            session_id: authorization.session_id.clone(),
        })?;
    if dkg_result.key_group != authorization.key_group
        || dkg_result.threshold != authorization.threshold
        || dkg_result.participant_count != authorization.participant_count
        || session.dkg_share_epoch != 0
    {
        return Err(validation_error(
            operation,
            "authorization does not match the retained DKG session",
        ));
    }
    let stored_public = session.dkg_public_key_package.clone().ok_or_else(|| {
        EngineError::Internal(format!(
            "{operation}: retained DKG session has no public key package"
        ))
    })?;
    let native_public = native_public_key_package_from_frost(&stored_public)?;
    let (validated_stored_public, _) =
        validate_public_key_package(operation, authorization, validated, &native_public)?;
    if validated_stored_public != stored_public {
        return Err(EngineError::Internal(format!(
            "{operation}: retained public key package failed canonical round trip"
        )));
    }
    let key_package = session
        .dkg_key_packages
        .as_ref()
        .and_then(|packages| packages.get(&helper_identifier))
        .cloned()
        .ok_or_else(|| {
            validation_error(
                operation,
                format!("local store has no key package for helper [{helper_identifier}]"),
            )
        })?;
    let frost_helper = participant_identifier_to_frost_identifier(helper_identifier)?;
    if *key_package.identifier() != frost_helper
        || *key_package.min_signers() != authorization.threshold
        || key_package.verifying_key() != stored_public.verifying_key()
        || stored_public.verifying_shares().get(&frost_helper)
            != Some(key_package.verifying_share())
    {
        return Err(EngineError::Internal(format!(
            "{operation}: retained helper key package is inconsistent with its DKG session"
        )));
    }
    Ok((key_package, stored_public))
}

fn share_repair_lagrange_coefficient(
    operation: &str,
    helper_identifiers: &[u16],
    helper_identifier: u16,
    target_identifier: u16,
) -> Result<RepairScalar, EngineError> {
    let helper = RepairScalar::from(u64::from(helper_identifier));
    let target = RepairScalar::from(u64::from(target_identifier));
    let mut numerator = RepairScalar::ONE;
    let mut denominator = RepairScalar::ONE;
    let mut found = false;
    for candidate_identifier in helper_identifiers {
        if *candidate_identifier == helper_identifier {
            found = true;
            continue;
        }
        let candidate = RepairScalar::from(u64::from(*candidate_identifier));
        numerator *= target - candidate;
        denominator *= helper - candidate;
    }
    if !found {
        return Err(validation_error(
            operation,
            "helper_identifier is not in the authorized helper set",
        ));
    }
    let inverse = Option::<RepairScalar>::from(denominator.invert()).ok_or_else(|| {
        validation_error(operation, "authorized helper identifiers are not distinct")
    })?;
    Ok(numerator * inverse)
}

pub(crate) fn share_repair_part1(
    request: ShareRepairPart1Request,
) -> Result<ShareRepairPart1Result, EngineError> {
    const OP: &str = "share_repair_part1";
    enforce_provenance_gate()?;
    let validated = validate_share_repair_authorization(OP, &request.authorization, true)?;
    let (key_package, stored_public_key_package) = load_helper_material(
        OP,
        &request.authorization,
        &validated,
        request.helper_identifier,
    )?;
    let transport_roster = validate_share_repair_transport_roster(
        OP,
        &request.authorization,
        &validated,
        &request.transport_roster,
    )?;
    let local_store_fingerprint = durable_store_identity()?.fingerprint;
    let local_transport_endpoint = transport_roster
        .endpoints
        .get(&request.helper_identifier)
        .ok_or_else(|| EngineError::Internal(format!("{OP}: helper transport key disappeared")))?;
    if local_transport_endpoint.store_fingerprint != local_store_fingerprint {
        return Err(validation_error(
            OP,
            "helper transport-roster store fingerprint does not match the active durable store",
        ));
    }
    // Re-resolve the active state-key provider outside the engine-state lock.
    // A provider/root rotation invalidates a cached session instead of letting
    // old native key material bridge the rotation boundary.
    let current_transport_secret = derive_share_repair_transport_secret(
        validated.digest,
        request.helper_identifier,
        ShareRepairTransportRole::Helper,
        local_store_fingerprint,
    )?;
    with_share_repair_transport_secret(
        OP,
        validated.digest,
        transport_roster.digest,
        request.helper_identifier,
        &current_transport_secret,
        &local_transport_endpoint.public_key,
        |_| Ok(()),
    )?;

    let signing_share_bytes = Zeroizing::new(key_package.signing_share().serialize());
    let signing_share =
        SecretRepairScalar::deserialize(signing_share_bytes.as_ref()).map_err(|error| {
            validation_error(OP, format!("invalid retained signing share: {error}"))
        })?;
    let lagrange = share_repair_lagrange_coefficient(
        OP,
        &request.authorization.helper_identifiers,
        request.helper_identifier,
        request.authorization.target_identifier,
    )?;
    let mut weighted_share = signing_share.multiply_public(lagrange);
    // Plaintext delta slots are deterministic for this exact signed bundle so
    // a helper restart cannot mix a new row with peers' still-retransmitted old
    // rows. The ECIES envelopes below deliberately retain fresh OS randomness;
    // authenticated alternate encodings of a slot decrypt to this same value.
    let mut envelope_rng = zeroizing_rng_from_os();
    let context_digest = bytes32_hex(validated.digest);
    let mut result_deltas = Vec::with_capacity(request.authorization.helper_identifiers.len());
    let mut running_sum = SecretRepairScalar::zero();
    let last_index = request
        .authorization
        .helper_identifiers
        .len()
        .checked_sub(1)
        .ok_or_else(|| {
            EngineError::Internal(format!("{OP}: authorized helper set is unexpectedly empty"))
        })?;
    for (index, recipient_identifier) in request.authorization.helper_identifiers.iter().enumerate()
    {
        let endpoint = transport_roster
            .endpoints
            .get(recipient_identifier)
            .ok_or_else(|| {
                EngineError::Internal(format!("{OP}: recipient transport key disappeared"))
            })?;
        let delta = if index == last_index {
            weighted_share.subtract_assign(&running_sum);
            &weighted_share
        } else {
            let random_delta = derive_share_repair_part1_delta(
                &current_transport_secret,
                validated.digest,
                transport_roster.digest,
                request.helper_identifier,
                *recipient_identifier,
            )?;
            running_sum.add_assign(&random_delta);
            result_deltas.push(ShareRepairDelta {
                context_digest: context_digest.clone(),
                sender_identifier: request.helper_identifier,
                recipient_identifier: *recipient_identifier,
                payload_hex: encrypt_repair_scalar(
                    &random_delta,
                    &current_transport_secret,
                    &endpoint.public_key,
                    ShareRepairEnvelopeKind::Delta,
                    validated.digest,
                    transport_roster.digest,
                    request.helper_identifier,
                    *recipient_identifier,
                    &mut envelope_rng,
                )?,
            });
            continue;
        };
        result_deltas.push(ShareRepairDelta {
            context_digest: context_digest.clone(),
            sender_identifier: request.helper_identifier,
            recipient_identifier: *recipient_identifier,
            payload_hex: encrypt_repair_scalar(
                delta,
                &current_transport_secret,
                &endpoint.public_key,
                ShareRepairEnvelopeKind::Delta,
                validated.digest,
                transport_roster.digest,
                request.helper_identifier,
                *recipient_identifier,
                &mut envelope_rng,
            )?,
        });
    }
    ensure_current_share_repair_transport_key(
        OP,
        &current_transport_secret.public_key(),
        validated.digest,
        request.helper_identifier,
        ShareRepairTransportRole::Helper,
        local_store_fingerprint,
    )?;
    Ok(ShareRepairPart1Result {
        context_digest,
        helper_identifier: request.helper_identifier,
        public_key_package: native_public_key_package_from_frost(&stored_public_key_package)?,
        deltas: result_deltas,
    })
}

pub(crate) fn share_repair_part2(
    request: ShareRepairPart2Request,
) -> Result<ShareRepairPart2Result, EngineError> {
    const OP: &str = "share_repair_part2";
    enforce_provenance_gate()?;
    let validated = validate_share_repair_authorization(OP, &request.authorization, true)?;
    // Loading the selected helper's key is a possession/admission check even
    // though Part2 itself only sums incoming scalars.
    let _ = load_helper_material(
        OP,
        &request.authorization,
        &validated,
        request.helper_identifier,
    )?;
    if request.deltas.len() != request.authorization.helper_identifiers.len() {
        return Err(validation_error(
            OP,
            "deltas must contain exactly one value from every authorized helper",
        ));
    }
    let context_digest = bytes32_hex(validated.digest);
    let transport_roster = validate_share_repair_transport_roster(
        OP,
        &request.authorization,
        &validated,
        &request.transport_roster,
    )?;
    let target_transport_endpoint = transport_roster
        .endpoints
        .get(&request.authorization.target_identifier)
        .ok_or_else(|| EngineError::Internal(format!("{OP}: target transport key disappeared")))?;
    let local_transport_endpoint = transport_roster
        .endpoints
        .get(&request.helper_identifier)
        .ok_or_else(|| EngineError::Internal(format!("{OP}: helper transport key disappeared")))?;
    let local_store_fingerprint = durable_store_identity()?.fingerprint;
    if local_transport_endpoint.store_fingerprint != local_store_fingerprint {
        return Err(validation_error(
            OP,
            "helper transport-roster store fingerprint does not match the active durable store",
        ));
    }
    let current_transport_secret = derive_share_repair_transport_secret(
        validated.digest,
        request.helper_identifier,
        ShareRepairTransportRole::Helper,
        local_store_fingerprint,
    )?;
    let sigma = with_share_repair_transport_secret(
        OP,
        validated.digest,
        transport_roster.digest,
        request.helper_identifier,
        &current_transport_secret,
        &local_transport_endpoint.public_key,
        |recipient_secret_key| {
            let mut accumulator = SecretRepairScalar::zero();
            for (index, (delta, expected_sender)) in request
                .deltas
                .iter()
                .zip(request.authorization.helper_identifiers.iter())
                .enumerate()
            {
                if delta.context_digest != context_digest
                    || delta.sender_identifier != *expected_sender
                    || delta.recipient_identifier != request.helper_identifier
                {
                    return Err(validation_error(
                        OP,
                        format!("delta [{index}] has the wrong context, sender, or recipient"),
                    ));
                }
                let decoded = decrypt_repair_scalar(
                    recipient_secret_key,
                    &transport_roster
                        .endpoints
                        .get(expected_sender)
                        .ok_or_else(|| {
                            EngineError::Internal(format!("{OP}: sender transport key disappeared"))
                        })?
                        .public_key,
                    &delta.payload_hex,
                    ShareRepairEnvelopeKind::Delta,
                    validated.digest,
                    transport_roster.digest,
                    *expected_sender,
                    request.helper_identifier,
                )?;
                accumulator.add_assign(&decoded);
            }
            Ok(accumulator)
        },
    )?;
    let mut rng = zeroizing_rng_from_os();
    let sigma_payload_hex = encrypt_repair_scalar(
        &sigma,
        &current_transport_secret,
        &target_transport_endpoint.public_key,
        ShareRepairEnvelopeKind::Sigma,
        validated.digest,
        transport_roster.digest,
        request.helper_identifier,
        request.authorization.target_identifier,
        &mut rng,
    )?;
    ensure_current_share_repair_transport_key(
        OP,
        &current_transport_secret.public_key(),
        validated.digest,
        request.helper_identifier,
        ShareRepairTransportRole::Helper,
        local_store_fingerprint,
    )?;
    Ok(ShareRepairPart2Result {
        context_digest: context_digest.clone(),
        sigma: ShareRepairSigma {
            context_digest,
            helper_identifier: request.helper_identifier,
            payload_hex: sigma_payload_hex,
        },
    })
}

fn exact_installed_repair(
    authorization: &ShareRepairAuthorization,
    validated: &ValidatedShareRepairAuthorization,
    public_key_package: &frost::keys::PublicKeyPackage,
) -> Result<Option<DkgResult>, EngineError> {
    let guard = state()?
        .lock()
        .map_err(|_| EngineError::Internal("engine lock poisoned".to_string()))?;
    let Some(session) = guard.sessions.get(&authorization.session_id) else {
        return Ok(None);
    };
    let Some(recovered) = session
        .recovered_seats
        .get(&authorization.target_identifier)
    else {
        return Ok(None);
    };
    if recovered.recovery_epoch != authorization.recovery_epoch
        || recovered.authorization_digest != validated.digest
        || recovered.active_store_fingerprint != validated.new_store_fingerprint
    {
        return Ok(None);
    }
    let result = session.dkg_result.as_ref().ok_or_else(|| {
        EngineError::Internal("recovered seat has no retained DKG result".to_string())
    })?;
    let key_package = session
        .dkg_key_packages
        .as_ref()
        .and_then(|packages| packages.get(&authorization.target_identifier))
        .ok_or_else(|| {
            EngineError::Internal("recovered seat has no retained key package".to_string())
        })?;
    if result.key_group != authorization.key_group
        || result.threshold != authorization.threshold
        || result.participant_count != authorization.participant_count
        || session.dkg_public_key_package.as_ref() != Some(public_key_package)
        || *key_package.identifier() != validated.target_identifier
        || public_key_package
            .verifying_shares()
            .get(&validated.target_identifier)
            != Some(key_package.verifying_share())
    {
        return Err(EngineError::Internal(
            "recovered-seat metadata is inconsistent with retained key material".to_string(),
        ));
    }
    Ok(Some(result.clone()))
}

fn install_result(
    authorization: &ShareRepairAuthorization,
    validated: &ValidatedShareRepairAuthorization,
    result: DkgResult,
    idempotent: bool,
) -> InstallRepairedShareResult {
    InstallRepairedShareResult {
        schema: TBTC_SIGNER_SHARE_REPAIR_INSTALL_RESULT_SCHEMA.to_string(),
        session_id: result.session_id,
        key_group: result.key_group,
        target_identifier: authorization.target_identifier,
        recovery_epoch: authorization.recovery_epoch,
        authorization_digest: bytes32_hex(validated.digest),
        active_store_fingerprint: bytes32_hex(validated.new_store_fingerprint),
        idempotent,
    }
}

pub(crate) fn install_repaired_share(
    request: InstallRepairedShareRequest,
) -> Result<InstallRepairedShareResult, EngineError> {
    const OP: &str = "install_repaired_share";
    enforce_provenance_gate()?;
    // Verify the static certificate first. The endpoint recognizes an exact
    // already-committed replay after expiry as defense in depth; normal
    // recovery from an uncertain external-anchor outcome is process restart
    // plus authenticated startup reconciliation. Expiry still gates every
    // initial installation and all helper-side secret generation.
    let validated = validate_share_repair_authorization(OP, &request.authorization, false)?;
    let (stored_public, repair_public) = validate_public_key_package(
        OP,
        &request.authorization,
        &validated,
        &request.public_key_package,
    )?;
    let current_store_fingerprint = durable_store_identity()?.fingerprint;
    if current_store_fingerprint != validated.new_store_fingerprint {
        return Err(validation_error(
            OP,
            "authorization does not name the active durable store",
        ));
    }
    if let Some(result) =
        exact_installed_repair(&request.authorization, &validated, &stored_public)?
    {
        return Ok(install_result(
            &request.authorization,
            &validated,
            result,
            true,
        ));
    }
    enforce_share_repair_authorization_time(&request.authorization)?;
    let transport_roster = validate_share_repair_transport_roster(
        OP,
        &request.authorization,
        &validated,
        &request.transport_roster,
    )?;
    let local_transport_endpoint = transport_roster
        .endpoints
        .get(&request.authorization.target_identifier)
        .ok_or_else(|| EngineError::Internal(format!("{OP}: target transport key disappeared")))?;
    if local_transport_endpoint.store_fingerprint != current_store_fingerprint {
        return Err(validation_error(
            OP,
            "target transport-roster store fingerprint does not match the active durable store",
        ));
    }
    let current_transport_secret = derive_share_repair_transport_secret(
        validated.digest,
        request.authorization.target_identifier,
        ShareRepairTransportRole::Target,
        current_store_fingerprint,
    )?;

    if request.sigmas.len() != request.authorization.helper_identifiers.len() {
        return Err(validation_error(
            OP,
            "sigmas must contain exactly one value from every authorized helper",
        ));
    }
    let context_digest = bytes32_hex(validated.digest);
    let repaired_share = with_share_repair_transport_secret(
        OP,
        validated.digest,
        transport_roster.digest,
        request.authorization.target_identifier,
        &current_transport_secret,
        &local_transport_endpoint.public_key,
        |recipient_secret_key| {
            let mut accumulator = SecretRepairScalar::zero();
            for (index, (sigma, expected_helper)) in request
                .sigmas
                .iter()
                .zip(request.authorization.helper_identifiers.iter())
                .enumerate()
            {
                if sigma.context_digest != context_digest
                    || sigma.helper_identifier != *expected_helper
                {
                    return Err(validation_error(
                        OP,
                        format!("sigma [{index}] has the wrong context or helper"),
                    ));
                }
                let decoded = decrypt_repair_scalar(
                    recipient_secret_key,
                    &transport_roster
                        .endpoints
                        .get(expected_helper)
                        .ok_or_else(|| {
                            EngineError::Internal(format!("{OP}: helper transport key disappeared"))
                        })?
                        .public_key,
                    &sigma.payload_hex,
                    ShareRepairEnvelopeKind::Sigma,
                    validated.digest,
                    transport_roster.digest,
                    *expected_helper,
                    request.authorization.target_identifier,
                )?;
                accumulator.add_assign(&decoded);
            }
            Ok(accumulator)
        },
    )?;
    ensure_current_share_repair_transport_key(
        OP,
        &current_transport_secret.public_key(),
        validated.digest,
        request.authorization.target_identifier,
        ShareRepairTransportRole::Target,
        current_store_fingerprint,
    )?;
    let expected_verifying_share = stored_public
        .verifying_shares()
        .get(&validated.target_identifier)
        .ok_or_else(|| {
            EngineError::Internal(format!("{OP}: target verifying share disappeared"))
        })?;
    let expected_verifying_share_bytes = expected_verifying_share.serialize().map_err(|error| {
        EngineError::Internal(format!(
            "{OP}: failed to serialize target verifying share: {error}"
        ))
    })?;
    if repaired_share.verifying_share_encoding(OP)?.as_slice()
        != expected_verifying_share_bytes.as_slice()
    {
        return Err(validation_error(
            OP,
            "reconstructed share does not match the authorized public key package",
        ));
    }

    let repaired_share_bytes = repaired_share.serialize();
    let mut signing_share =
        frost::keys::SigningShare::deserialize(repaired_share_bytes.as_ref())
            .map_err(|error| validation_error(OP, format!("invalid repaired share: {error}")))?;
    let key_package = frost::keys::KeyPackage::new(
        validated.target_identifier,
        signing_share,
        *expected_verifying_share,
        *repair_public.verifying_key(),
        request.authorization.threshold,
    );
    // SigningShare is an upstream Copy type. Wipe the named source copy after
    // KeyPackage construction; the package owns its own ZeroizeOnDrop copy.
    signing_share.zeroize();
    if key_package.verifying_share() != expected_verifying_share
        || key_package.verifying_key() != stored_public.verifying_key()
        || *key_package.min_signers() != request.authorization.threshold
    {
        return Err(validation_error(
            OP,
            "reconstructed share does not match the authorized public key package",
        ));
    }
    let mut key_package_bytes = key_package.serialize().map_err(|error| {
        EngineError::Internal(format!(
            "{OP}: failed to serialize repaired key package: {error}"
        ))
    })?;
    let key_package_hex = SecretHex::new(hex::encode(&key_package_bytes));
    key_package_bytes.zeroize();
    let persistence_request = PersistDistributedDkgKeyPackageRequest {
        session_id: request.authorization.session_id.clone(),
        participant_identifier: request.authorization.target_identifier,
        threshold: request.authorization.threshold,
        participant_count: request.authorization.participant_count,
        key_package: NativeFrostKeyPackage {
            identifier: frost_identifier_to_go_string(validated.target_identifier),
            data_hex: key_package_hex,
        },
        public_key_package: request.public_key_package,
    };
    let outcome = persist_repaired_dkg_key_package(
        persistence_request,
        RecoveredSeatState {
            participant_identifier: request.authorization.target_identifier,
            recovery_epoch: request.authorization.recovery_epoch,
            authorization_digest: validated.digest,
            active_store_fingerprint: validated.new_store_fingerprint,
        },
    )?;
    Ok(install_result(
        &request.authorization,
        &validated,
        outcome.result,
        outcome.idempotent,
    ))
}

#[cfg(test)]
mod repair_secret_tests {
    use super::*;

    fn assert_zeroizing_drop<T: Zeroize + ZeroizeOnDrop>() {}
    fn assert_zeroize_on_drop<T: ZeroizeOnDrop>() {}

    #[test]
    fn repair_scalar_has_owned_zeroizing_lifetime() {
        assert_zeroizing_drop::<SecretRepairScalar>();
        assert_zeroize_on_drop::<RepairSecretKey>();
        assert!(std::mem::needs_drop::<SecretRepairScalar>());
        assert!(std::mem::needs_drop::<RepairSecretKey>());
        // This is the safety premise for ZeroizingHkdfSha256's complete
        // in-place wipe. A dependency upgrade that adds an owned allocation or
        // Drop implementation must fail this focused audit guard.
        assert!(!std::mem::needs_drop::<Hkdf<Sha256>>());
        assert!(std::mem::needs_drop::<ZeroizingHkdfSha256>());

        let mut bytes = [0u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES];
        bytes[SHARE_REPAIR_TRANSPORT_SCALAR_BYTES - 1] = 7;
        let mut scalar = SecretRepairScalar::deserialize(&bytes).expect("canonical scalar");
        assert_eq!(scalar.serialize().as_ref(), bytes);
        scalar.zeroize();
        assert_eq!(
            scalar.serialize().as_ref(),
            [0u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES]
        );
    }

    #[test]
    fn repair_transport_derivation_is_stable_and_domain_separated() {
        let _guard = lock_test_state();
        reset_for_tests();

        let authorization_digest = [0x71; 32];
        let store_fingerprint = [0x72; 32];
        let helper = derive_share_repair_transport_secret(
            authorization_digest,
            7,
            ShareRepairTransportRole::Helper,
            store_fingerprint,
        )
        .expect("derive helper transport secret");
        let repeated = derive_share_repair_transport_secret(
            authorization_digest,
            7,
            ShareRepairTransportRole::Helper,
            store_fingerprint,
        )
        .expect("repeat helper transport derivation");
        assert_eq!(helper.public_key(), repeated.public_key());

        let target_role = derive_share_repair_transport_secret(
            authorization_digest,
            7,
            ShareRepairTransportRole::Target,
            store_fingerprint,
        )
        .expect("derive target-role transport secret");
        let other_participant = derive_share_repair_transport_secret(
            authorization_digest,
            8,
            ShareRepairTransportRole::Helper,
            store_fingerprint,
        )
        .expect("derive other-participant transport secret");
        let other_authorization = derive_share_repair_transport_secret(
            [0x73; 32],
            7,
            ShareRepairTransportRole::Helper,
            store_fingerprint,
        )
        .expect("derive other-authorization transport secret");
        let other_store = derive_share_repair_transport_secret(
            authorization_digest,
            7,
            ShareRepairTransportRole::Helper,
            [0x74; 32],
        )
        .expect("derive other-store transport secret");
        for separated in [
            target_role.public_key(),
            other_participant.public_key(),
            other_authorization.public_key(),
            other_store.public_key(),
        ] {
            assert_ne!(helper.public_key(), separated);
        }

        std::env::set_var(
            TBTC_SIGNER_STATE_ENCRYPTION_KEY_HEX_ENV,
            hex::encode([0x75; 32]),
        );
        let rotated_state_root = derive_share_repair_transport_secret(
            authorization_digest,
            7,
            ShareRepairTransportRole::Helper,
            store_fingerprint,
        )
        .expect("derive under rotated state root");
        assert_ne!(helper.public_key(), rotated_state_root.public_key());
        assert!(ensure_current_share_repair_transport_key(
            "repair-transport-rotation-test",
            &helper.public_key(),
            authorization_digest,
            7,
            ShareRepairTransportRole::Helper,
            store_fingerprint,
        )
        .is_err());
        ensure_current_share_repair_transport_key(
            "repair-transport-rotation-test",
            &rotated_state_root.public_key(),
            authorization_digest,
            7,
            ShareRepairTransportRole::Helper,
            store_fingerprint,
        )
        .expect("current root-bound key passes the post-operation gate");

        reset_for_tests();
    }

    #[test]
    fn part1_delta_derivation_is_replay_stable_and_context_separated() {
        let transport_secret =
            RepairSecretKey::from_slice(&[0x61; 32]).expect("fixed transport secret");
        let authorization_digest = [0x31; 32];
        let transport_roster_digest = [0x41; 32];
        let first = derive_share_repair_part1_delta(
            &transport_secret,
            authorization_digest,
            transport_roster_digest,
            1,
            2,
        )
        .expect("derive first replay delta");
        // Deriving an unrelated slot in between must not advance shared state
        // or change this slot's result.
        let _other_slot = derive_share_repair_part1_delta(
            &transport_secret,
            authorization_digest,
            transport_roster_digest,
            1,
            3,
        )
        .expect("derive other replay slot");
        let replay = derive_share_repair_part1_delta(
            &transport_secret,
            authorization_digest,
            transport_roster_digest,
            1,
            2,
        )
        .expect("rederive replay delta");
        let first_bytes = first.serialize();
        assert_eq!(
            "6bdeabf1aabfd58ae11dc11d7b3c685f6d13397cbabd1670507a3067f80ff0a6",
            hex::encode(&first_bytes[..])
        );
        assert_eq!(&first_bytes[..], &replay.serialize()[..]);

        for separated in [
            derive_share_repair_part1_delta(
                &transport_secret,
                [0x32; 32],
                transport_roster_digest,
                1,
                2,
            ),
            derive_share_repair_part1_delta(
                &transport_secret,
                authorization_digest,
                [0x42; 32],
                1,
                2,
            ),
            derive_share_repair_part1_delta(
                &transport_secret,
                authorization_digest,
                transport_roster_digest,
                2,
                2,
            ),
            derive_share_repair_part1_delta(
                &transport_secret,
                authorization_digest,
                transport_roster_digest,
                1,
                3,
            ),
        ] {
            let separated = separated.expect("derive separated replay delta");
            assert_ne!(&first_bytes[..], &separated.serialize()[..]);
        }
        let other_transport_secret =
            RepairSecretKey::from_slice(&[0x62; 32]).expect("other transport secret");
        let other_key = derive_share_repair_part1_delta(
            &other_transport_secret,
            authorization_digest,
            transport_roster_digest,
            1,
            2,
        )
        .expect("derive other-key replay delta");
        assert_ne!(&first_bytes[..], &other_key.serialize()[..]);
    }

    #[test]
    fn repair_envelope_authenticates_every_routing_binding() {
        let mut rng = zeroizing_rng_from_os();
        let sender_secret = RepairSecretKey::random(&mut rng);
        let sender_public = sender_secret.public_key();
        let recipient_secret = RepairSecretKey::random(&mut rng);
        let recipient_public = recipient_secret.public_key();
        let mut scalar_bytes = [0u8; SHARE_REPAIR_TRANSPORT_SCALAR_BYTES];
        scalar_bytes[SHARE_REPAIR_TRANSPORT_SCALAR_BYTES - 1] = 9;
        let scalar = SecretRepairScalar::deserialize(&scalar_bytes).expect("canonical scalar");
        let context_digest = [0x31; 32];
        let transport_roster_digest = [0x41; 32];
        let payload = encrypt_repair_scalar(
            &scalar,
            &sender_secret,
            &recipient_public,
            ShareRepairEnvelopeKind::Delta,
            context_digest,
            transport_roster_digest,
            1,
            2,
            &mut rng,
        )
        .expect("encrypt scalar");
        assert_eq!(payload.len(), SHARE_REPAIR_TRANSPORT_PAYLOAD_BYTES * 2);
        assert!(!payload.contains(&hex::encode(scalar_bytes)));
        let alternate_encoding = encrypt_repair_scalar(
            &scalar,
            &sender_secret,
            &recipient_public,
            ShareRepairEnvelopeKind::Delta,
            context_digest,
            transport_roster_digest,
            1,
            2,
            &mut rng,
        )
        .expect("encrypt alternate scalar encoding");
        assert_ne!(payload, alternate_encoding);

        let decrypted = decrypt_repair_scalar(
            &recipient_secret,
            &sender_public,
            &payload,
            ShareRepairEnvelopeKind::Delta,
            context_digest,
            transport_roster_digest,
            1,
            2,
        )
        .expect("decrypt scalar");
        assert_eq!(decrypted.serialize().as_ref(), scalar_bytes);
        let alternate_decrypted = decrypt_repair_scalar(
            &recipient_secret,
            &sender_public,
            &alternate_encoding,
            ShareRepairEnvelopeKind::Delta,
            context_digest,
            transport_roster_digest,
            1,
            2,
        )
        .expect("decrypt alternate scalar encoding");
        assert_eq!(alternate_decrypted.serialize().as_ref(), scalar_bytes);

        for invalid in [
            decrypt_repair_scalar(
                &recipient_secret,
                &sender_public,
                &payload,
                ShareRepairEnvelopeKind::Sigma,
                context_digest,
                transport_roster_digest,
                1,
                2,
            ),
            decrypt_repair_scalar(
                &recipient_secret,
                &sender_public,
                &payload,
                ShareRepairEnvelopeKind::Delta,
                [0x32; 32],
                transport_roster_digest,
                1,
                2,
            ),
            decrypt_repair_scalar(
                &recipient_secret,
                &sender_public,
                &payload,
                ShareRepairEnvelopeKind::Delta,
                context_digest,
                [0x42; 32],
                1,
                2,
            ),
            decrypt_repair_scalar(
                &recipient_secret,
                &sender_public,
                &payload,
                ShareRepairEnvelopeKind::Delta,
                context_digest,
                transport_roster_digest,
                3,
                2,
            ),
            decrypt_repair_scalar(
                &recipient_secret,
                &sender_public,
                &payload,
                ShareRepairEnvelopeKind::Delta,
                context_digest,
                transport_roster_digest,
                1,
                3,
            ),
        ] {
            assert!(invalid.is_err());
        }

        let wrong_recipient = RepairSecretKey::random(&mut rng);
        assert!(decrypt_repair_scalar(
            &wrong_recipient,
            &sender_public,
            &payload,
            ShareRepairEnvelopeKind::Delta,
            context_digest,
            transport_roster_digest,
            1,
            2,
        )
        .is_err());

        let wrong_sender = RepairSecretKey::random(&mut rng).public_key();
        assert!(decrypt_repair_scalar(
            &recipient_secret,
            &wrong_sender,
            &payload,
            ShareRepairEnvelopeKind::Delta,
            context_digest,
            transport_roster_digest,
            1,
            2,
        )
        .is_err());

        let forger_secret = RepairSecretKey::random(&mut rng);
        let forged_payload = encrypt_repair_scalar(
            &scalar,
            &forger_secret,
            &recipient_public,
            ShareRepairEnvelopeKind::Delta,
            context_digest,
            transport_roster_digest,
            1,
            2,
            &mut rng,
        )
        .expect("construct payload under non-roster sender key");
        assert!(decrypt_repair_scalar(
            &recipient_secret,
            &sender_public,
            &forged_payload,
            ShareRepairEnvelopeKind::Delta,
            context_digest,
            transport_roster_digest,
            1,
            2,
        )
        .is_err());

        let mut corrupted_payload = hex::decode(&payload).expect("payload hex");
        let last = corrupted_payload
            .last_mut()
            .expect("non-empty encrypted payload");
        *last ^= 1;
        assert!(decrypt_repair_scalar(
            &recipient_secret,
            &sender_public,
            &hex::encode(corrupted_payload),
            ShareRepairEnvelopeKind::Delta,
            context_digest,
            transport_roster_digest,
            1,
            2,
        )
        .is_err());
    }
}
