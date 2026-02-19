package frost

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcutil"
)

const (
	// OutputKeySize is the byte length of a Taproot x-only output key.
	OutputKeySize = 32
	// SignatureComponentSize is the byte length of each Schnorr signature part.
	SignatureComponentSize = 32
)

// OutputKey is a Taproot x-only output key used by BIP-340/341.
type OutputKey [OutputKeySize]byte

// WalletPublicKeyHashCompatibilityAlias computes the 20-byte compatibility
// alias from a Taproot output key:
// HASH160(0x02 || xOnlyOutputKey).
func WalletPublicKeyHashCompatibilityAlias(outputKey OutputKey) [20]byte {
	serialized := make([]byte, 0, 1+OutputKeySize)
	serialized = append(serialized, byte(0x02))
	serialized = append(serialized, outputKey[:]...)

	hash := btcutil.Hash160(serialized)

	var result [20]byte
	copy(result[:], hash)

	return result
}

// Signature is a 64-byte BIP-340 Schnorr signature split into its two
// 32-byte components: R (x-coordinate nonce commitment) and S (scalar).
type Signature struct {
	R [SignatureComponentSize]byte
	S [SignatureComponentSize]byte
}

// Serialize concatenates signature components into a 64-byte value.
func (s *Signature) Serialize() [2 * SignatureComponentSize]byte {
	var result [2 * SignatureComponentSize]byte
	copy(result[0:SignatureComponentSize], s.R[:])
	copy(result[SignatureComponentSize:], s.S[:])
	return result
}

// String returns a hex representation useful in logs.
func (s *Signature) String() string {
	serialized := s.Serialize()
	return fmt.Sprintf("R: 0x%s, S: 0x%s",
		hex.EncodeToString(serialized[0:SignatureComponentSize]),
		hex.EncodeToString(serialized[SignatureComponentSize:]),
	)
}
