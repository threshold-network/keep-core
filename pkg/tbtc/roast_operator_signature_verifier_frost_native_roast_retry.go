//go:build frost_native && frost_roast_retry

package tbtc

import (
	"encoding/binary"
	"fmt"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// The ROAST retry/blame layer authenticates evidence with operator-key
// signatures: a signer signs its own LocalEvidenceSnapshot and the elected
// coordinator signs the assembled TransitionMessage bundle (see
// pkg/frost/roast/signature.go). The coordinator state machine treats a
// signature as OPAQUE bytes and delegates all interpretation to the
// SignatureVerifier, which owns the member-index -> operator-key mapping.
//
// The node only knows each seat's operator ADDRESS (wallet.signingGroupOperators),
// not its public key, and chain.Signing.VerifyWithPublicKey needs the public key.
// So the signature the Signer emits is a self-describing ENVELOPE that carries the
// signing operator's public key alongside the raw operator-key signature. The
// Verifier then, exactly like the FROST DKG-result-signing path
// (frost_dkg_result_signing.go): (1) binds the carried public key to the claimed
// member's seat via PublicKeyBytesToAddress == signingGroupOperators[member-1],
// and (2) checks the raw signature against that public key. Both must hold, so a
// valid signature under the wrong seat's key -- or a key that is not seated at the
// claimed member -- is rejected.
//
// This envelope is a private contract between operatorKeyRoastSigner and
// memberKeyedRoastSignatureVerifier; it must be adopted by the whole operator set
// at once (a peer still emitting bare signatures would be rejected by an upgraded
// verifier during a retry). The happy path never verifies evidence signatures, so
// the switch is inert until an actual ROAST retry occurs.

const (
	// roastSignatureEnvelopeVersion is the leading byte of every envelope. It lets
	// the format evolve (e.g. a future compressed-key layout) while letting an
	// upgraded verifier reject an unrecognized layout deterministically instead of
	// misparsing it.
	roastSignatureEnvelopeVersion byte = 1

	// roastSignatureEnvelopeHeaderLen is the fixed prefix length:
	// version(1) + publicKeyLen(2, big-endian).
	roastSignatureEnvelopeHeaderLen = 3
)

// encodeRoastSignatureEnvelope packs an operator public key and a raw operator-key
// signature into the wire form the verifier expects:
//
//	[version:1][publicKeyLen: uint16 big-endian][publicKey][signature]
//
// Both parts must be non-empty: an empty public key cannot be bound to a seat and
// an empty signature never verifies, so refusing them here surfaces a signer bug
// at sign time rather than as an opaque verification failure on the retry path.
func encodeRoastSignatureEnvelope(publicKey, signature []byte) ([]byte, error) {
	if len(publicKey) == 0 {
		return nil, fmt.Errorf("roast signature envelope: empty public key")
	}
	if len(signature) == 0 {
		return nil, fmt.Errorf("roast signature envelope: empty signature")
	}
	if len(publicKey) > 0xFFFF {
		return nil, fmt.Errorf(
			"roast signature envelope: public key too long: %d bytes",
			len(publicKey),
		)
	}

	out := make([]byte, roastSignatureEnvelopeHeaderLen+len(publicKey)+len(signature))
	out[0] = roastSignatureEnvelopeVersion
	binary.BigEndian.PutUint16(out[1:3], uint16(len(publicKey)))
	copy(out[roastSignatureEnvelopeHeaderLen:], publicKey)
	copy(out[roastSignatureEnvelopeHeaderLen+len(publicKey):], signature)
	return out, nil
}

// decodeRoastSignatureEnvelope reverses encodeRoastSignatureEnvelope. It fails
// closed on any malformed input (wrong version, truncated header, length that
// overruns the buffer, or an empty signature remainder) so a corrupt or
// wrong-format envelope becomes a verification error rather than a panic or a
// silently-accepted signature.
func decodeRoastSignatureEnvelope(envelope []byte) (publicKey, signature []byte, err error) {
	if len(envelope) < roastSignatureEnvelopeHeaderLen {
		return nil, nil, fmt.Errorf(
			"roast signature envelope: too short: %d bytes", len(envelope),
		)
	}
	if envelope[0] != roastSignatureEnvelopeVersion {
		return nil, nil, fmt.Errorf(
			"roast signature envelope: unsupported version %d", envelope[0],
		)
	}
	pubKeyLen := int(binary.BigEndian.Uint16(envelope[1:3]))
	sigStart := roastSignatureEnvelopeHeaderLen + pubKeyLen
	if pubKeyLen == 0 || sigStart >= len(envelope) {
		return nil, nil, fmt.Errorf(
			"roast signature envelope: declared public key length %d inconsistent with envelope size %d",
			pubKeyLen, len(envelope),
		)
	}
	return envelope[roastSignatureEnvelopeHeaderLen:sigStart], envelope[sigStart:], nil
}

// memberKeyedRoastSignatureVerifier is the production roast.SignatureVerifier. It
// verifies that a signature attributed to a signing-group seat was produced by the
// operator seated at that member index, using the wallet's seat -> operator-address
// list and the chain's operator-key verification primitives.
//
// operatorAddresses is 0-indexed by seat: operatorAddresses[i] is the operator at
// 1-based member index i+1, matching wallet.signingGroupOperators. Verify is safe
// for concurrent use: it only reads immutable state and calls concurrency-safe
// chain.Signing methods.
type memberKeyedRoastSignatureVerifier struct {
	signing           chain.Signing
	operatorAddresses []chain.Address
}

// Verify implements roast.SignatureVerifier. It returns nil only when the envelope
// decodes, the carried public key is seated at the claimed member index, and the
// raw signature verifies against that public key over payload. Every failure path
// returns a descriptive error; the caller (verifySnapshotSignature /
// verifyBundleSignature) wraps it with roast.ErrSignatureInvalid.
func (v memberKeyedRoastSignatureVerifier) Verify(
	payload []byte,
	signature []byte,
	signer group.MemberIndex,
) error {
	if signer < 1 || int(signer) > len(v.operatorAddresses) {
		return fmt.Errorf(
			"member index %d out of range [1, %d]",
			signer, len(v.operatorAddresses),
		)
	}

	publicKey, rawSignature, err := decodeRoastSignatureEnvelope(signature)
	if err != nil {
		return fmt.Errorf("member %d: %w", signer, err)
	}

	// Bind the carried public key to the claimed seat BEFORE trusting the
	// signature: a signature can be perfectly valid under some key yet be
	// attributed to a member that key does not occupy. Deriving the address from
	// the key and matching it to this seat's operator is what ties the signature
	// to the member index the coordinator asked about.
	expectedOperator := v.operatorAddresses[signer-1]
	actualOperator := v.signing.PublicKeyBytesToAddress(publicKey)
	if actualOperator != expectedOperator {
		return fmt.Errorf(
			"member %d: envelope public key maps to operator %s, seat holds %s",
			signer, actualOperator, expectedOperator,
		)
	}

	valid, err := v.signing.VerifyWithPublicKey(payload, rawSignature, publicKey)
	if err != nil {
		return fmt.Errorf("member %d: signature verification error: %w", signer, err)
	}
	if !valid {
		return fmt.Errorf("member %d: signature does not verify against seat operator key", signer)
	}
	return nil
}
