// Deterministic round-nonce binding (round-nonce-v3 transcript seed).
//
// DELETION COMMITTED (decision 2026-06-12; see the Decision Log in
// docs/roast-phase-5-security-rollout-gates.md): this deterministic
// transitional path is dev/staging-only (production-gated) and will be
// deleted once the interactive production path is validated end to end.
// Until then the transitional signing flow is FROZEN - do not add new
// transcript inputs to it: each one must also extend RoundNonceBinding
// below, and an omission is a key-extraction-class bug (see the v3
// history in the struct docs).

use super::*;

/// Inputs that bind a deterministic transitional round-1 nonce.
///
/// Nonce-reuse safety invariant: a deterministic nonce may only repeat when
/// the entire FROST transcript it signs over repeats. Every value that can
/// change that transcript — anything entering the binding factor, the
/// challenge, the Lagrange interpolation set, or selecting the key material —
/// MUST be a field here and MUST feed `deterministic_seed` below. Binding a
/// value only through `round_id` is not sufficient: `round_id` is a
/// registry/idempotency handle whose derivation schema may evolve, and nonce
/// safety must not depend on that schema or on consumed-round registry
/// integrity (durable state can be rolled back, restored, or replicated).
/// If a new transcript input is added to the transitional signing flow (as
/// the Taproot tweak once was), it must be added here in the same change.
///
/// Note on the key material: the *group* verifying key alone is NOT enough.
/// In this transitional flow every member re-derives ALL participants'
/// round-1 commitments from the held key packages, so each *other*
/// participant's verifying share enters the commitment list, hence this
/// member's binding factor and challenge. Two key packages can share a
/// group verifying key while differing in an individual verifying share
/// (any threshold t ≥ 3 admits two polynomials with the same f(0) and the
/// same target share but a different non-target share). Binding only the
/// group key would let a rolled-back/cloned state present an identical seed
/// under a *different* challenge — the exact reuse this struct exists to
/// preclude — so the field below binds the full `PublicKeyPackage`
/// (group key AND every verifying share).
#[derive(Clone, Copy)]
pub(crate) struct RoundNonceBinding<'a> {
    pub(crate) session_id: &'a str,
    pub(crate) round_id: &'a str,
    /// Serialized full `PublicKeyPackage` — the group verifying key AND
    /// every participant's verifying share. Binds the nonce to the concrete
    /// key material that determines the whole commitment list (every other
    /// participant's commitment feeds this member's binding factor and
    /// challenge), not just to the group key or the session label.
    pub(crate) public_key_package_bytes: &'a [u8],
    pub(crate) message_bytes: &'a [u8],
    /// Taproot tweak applied at round 2; tweaking changes the challenge.
    pub(crate) taproot_merkle_root: Option<&'a [u8; 32]>,
    /// Canonical (sorted, deduplicated) signing set; determines the
    /// commitment list and the Lagrange coefficients.
    pub(crate) signing_participants: &'a [u16],
    pub(crate) participant_identifier: u16,
}

pub(crate) fn build_deterministic_round_nonce_and_commitment(
    key_package: &frost::keys::KeyPackage,
    binding: &RoundNonceBinding<'_>,
) -> (
    frost::round1::SigningNonces,
    frost::round1::SigningCommitments,
) {
    let mut signing_participants_bytes = Vec::with_capacity(binding.signing_participants.len() * 2);
    for signing_participant in binding.signing_participants {
        signing_participants_bytes.extend_from_slice(&signing_participant.to_be_bytes());
    }
    let (taproot_tweak_tag, taproot_tweak_bytes): (&[u8], &[u8]) = match binding.taproot_merkle_root
    {
        Some(taproot_merkle_root) => (b"taproot-tweak", taproot_merkle_root.as_slice()),
        None => (b"no-taproot-tweak", &[]),
    };

    let mut signing_share_bytes = key_package.signing_share().serialize();
    // Domain v3: v2 widened the set beyond (session, round, message,
    // participant); v3 widens the key-material binding from the group
    // verifying key alone to the full PublicKeyPackage (every verifying
    // share), closing the case where two key packages share a group key
    // but differ in a non-target share. See `RoundNonceBinding`.
    //
    // Encoding note: the participants set serializes big-endian while
    // `participant_identifier` keeps the v1 little-endian encoding. The
    // mix is harmless -- both are fixed-width and `deterministic_seed`
    // length-frames every part -- but it is part of the derived value:
    // changing either encoding changes derived commitments fleet-wide
    // and requires a new domain (`round-nonce-v4`), never an in-place
    // edit.
    let mut nonce_seed = deterministic_seed(&[
        b"round-nonce-v3",
        &signing_share_bytes,
        binding.public_key_package_bytes,
        binding.session_id.as_bytes(),
        binding.round_id.as_bytes(),
        binding.message_bytes,
        taproot_tweak_tag,
        taproot_tweak_bytes,
        &signing_participants_bytes,
        &binding.participant_identifier.to_le_bytes(),
    ]);
    signing_share_bytes.zeroize();
    let mut nonce_rng = ZeroizingChaCha20Rng::from_seed(nonce_seed);
    nonce_seed.zeroize();

    frost::round1::commit(key_package.signing_share(), &mut nonce_rng)
}
