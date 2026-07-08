//go:build frost_native

package signing

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/operator"
)

// Phase 1 of the distributed-DKG wiring makes round-2 CONFIDENTIAL. A FROST DKG
// round-2 package carries a per-recipient secret share, but it travels over the
// wallet BROADCAST channel - visible to the whole group - so a share sent in the
// clear would leak to every other member. This file seals each share to its
// recipient with an ECIES envelope over the existing pkg/crypto/ephemeral ECDH:
// the sender derives a symmetric key from a ONE-TIME ephemeral private key and
// the recipient's public key, encrypts the share, and carries the ephemeral
// PUBLIC key alongside the ciphertext. Only the recipient, via ECDH between its
// private key and that ephemeral public key, recovers the same symmetric key and
// opens the share. The sender's ephemeral private key is discarded after sealing,
// giving per-message forward secrecy on the sender side.
//
// The envelope is generic over ephemeral keys. The orchestrator supplies each
// recipient's per-DKG EPHEMERAL public key (learned from its authenticated
// round-1 broadcast), so BOTH sides use ephemeral keys discarded after the DKG,
// giving two-sided forward secrecy. (operatorPublicKeyToEphemeral converts an
// operator key to this type too, still used by the seal's own round-trip test.)
//
// SECURITY BOUNDARY: the envelope provides CONFIDENTIALITY ONLY. It does not
// authenticate the sealer, bind to a session/attempt, or prevent replay. Those
// come from (a) the sender-authenticated transport it runs over - the wallet
// broadcast channel authenticates the claimed sender against its operator key
// via the group MembershipValidator - and (b) FROST DKG Part3, which
// cryptographically verifies each round-2 share against the sender's round-1
// commitment (so a garbage or misdirected share fails the DKG into the existing
// retry/blame path, not a signing breach). It MUST NOT be used over an
// unauthenticated transport.

// sealedRound2Share is the confidential on-wire form of a round-2 share: a
// one-time ephemeral public key bound to the AES-ECDH ciphertext of the share.
type sealedRound2Share struct {
	EphemeralPublicKey []byte `json:"ephemeralPublicKey"`
	Ciphertext         []byte `json:"ciphertext"`
}

// sealRound2Share encrypts a round-2 share to the recipient's public key,
// producing a self-describing envelope any member can route but only the
// recipient can open.
func sealRound2Share(share []byte, recipient *ephemeral.PublicKey) (*sealedRound2Share, error) {
	if recipient == nil {
		return nil, fmt.Errorf("distributed dkg: nil recipient key for round-2 seal")
	}
	oneTime, err := ephemeral.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("distributed dkg: generate ephemeral key: %w", err)
	}
	ciphertext, err := oneTime.PrivateKey.Ecdh(recipient).Encrypt(share)
	if err != nil {
		return nil, fmt.Errorf("distributed dkg: seal round-2 share: %w", err)
	}
	return &sealedRound2Share{
		EphemeralPublicKey: oneTime.PublicKey.Marshal(),
		Ciphertext:         ciphertext,
	}, nil
}

// openRound2Share recovers a round-2 share sealed to us, using our static
// (operator) private key and the ephemeral public key carried in the envelope.
// A share sealed to another member, or a tampered envelope, fails to open (the
// underlying box is authenticated) rather than yielding a wrong share.
func openRound2Share(sealed *sealedRound2Share, self *ephemeral.PrivateKey) ([]byte, error) {
	if sealed == nil {
		return nil, fmt.Errorf("distributed dkg: nil sealed round-2 share")
	}
	if self == nil {
		return nil, fmt.Errorf("distributed dkg: nil private key for round-2 open")
	}
	oneTimePublic, err := ephemeral.UnmarshalPublicKey(sealed.EphemeralPublicKey)
	if err != nil {
		return nil, fmt.Errorf("distributed dkg: parse ephemeral public key: %w", err)
	}
	share, err := self.Ecdh(oneTimePublic).Decrypt(sealed.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("distributed dkg: open round-2 share: %w", err)
	}
	return share, nil
}

// operatorPublicKeyToEphemeral converts a member's operator public key into the
// ephemeral (btcec) public key the seal envelope encrypts to. Operator keys are
// secp256k1 (the ephemeral package's only curve), so the uncompressed operator
// key parses directly; a non-secp256k1 key is rejected rather than silently
// mis-parsed.
func operatorPublicKeyToEphemeral(publicKey *operator.PublicKey) (*ephemeral.PublicKey, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("distributed dkg: nil operator public key")
	}
	if publicKey.Curve != operator.Secp256k1 {
		return nil, fmt.Errorf(
			"distributed dkg: operator key curve [%v] is not secp256k1", publicKey.Curve,
		)
	}
	return ephemeral.UnmarshalPublicKey(operator.MarshalUncompressed(publicKey))
}

// operatorPrivateKeyToEphemeral converts this node's operator private key into
// the ephemeral private key used to open round-2 shares sealed to us.
func operatorPrivateKeyToEphemeral(privateKey *operator.PrivateKey) (*ephemeral.PrivateKey, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("distributed dkg: nil operator private key")
	}
	if privateKey.Curve != operator.Secp256k1 {
		return nil, fmt.Errorf(
			"distributed dkg: operator key curve [%v] is not secp256k1", privateKey.Curve,
		)
	}
	scalar := make([]byte, 32)
	privateKey.D.FillBytes(scalar)
	// UnmarshalPrivateKey copies the scalar into the returned key, so scrub this
	// raw copy of the long-lived operator secret on return.
	defer zeroBytes(scalar)
	return ephemeral.UnmarshalPrivateKey(scalar), nil
}
