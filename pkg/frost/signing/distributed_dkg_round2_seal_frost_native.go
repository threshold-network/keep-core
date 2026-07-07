//go:build frost_native

package signing

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
)

// Phase 1 of the distributed-DKG wiring makes round-2 CONFIDENTIAL. A FROST DKG
// round-2 package carries a per-recipient secret share, but it travels over the
// wallet BROADCAST channel - visible to the whole group - so a share sent in the
// clear would leak to every other member. This file seals each share to its
// recipient with an ECIES envelope over the existing pkg/crypto/ephemeral ECDH:
// the sender derives a symmetric key from a ONE-TIME ephemeral private key and
// the recipient's static (operator) public key, encrypts the share, and carries
// the ephemeral PUBLIC key alongside the ciphertext. Only the recipient, via ECDH
// between its private key and that ephemeral public key, recovers the same
// symmetric key and opens the share. The sender's ephemeral private key is
// discarded after sealing, giving per-message forward secrecy on the sender side.
//
// The envelope is generic over ephemeral keys here; the orchestrator supplies
// each recipient's operator key (converted to an ephemeral public key) when it
// wires this in.

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
