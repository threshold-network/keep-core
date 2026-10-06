package ephemeral

import (
	"crypto/sha256"

	"github.com/btcsuite/btcd/btcec"
	"github.com/keep-network/keep-core/pkg/encryption"
)

// SymmetricEcdhKey is an ephemeral Elliptic Curve key created with
// Diffie-Hellman key exchange and implementing `SymmetricKey` interface.
type SymmetricEcdhKey struct {
	box encryption.Box
}

// Ecdh performs Elliptic Curve Diffie-Hellman operation between public and
// private key. The returned value is `SymmetricEcdhKey` that can be used
// for encryption and decryption.
func (pk *PrivateKey) Ecdh(publicKey *PublicKey) *SymmetricEcdhKey {
	shared := btcec.GenerateSharedSecret(
		(*btcec.PrivateKey)(pk),
		(*btcec.PublicKey)(publicKey),
	)

	return &SymmetricEcdhKey{
		box: encryption.NewBox(sha256.Sum256(shared)),
	}
}

// Encrypt plaintext.
func (sek *SymmetricEcdhKey) Encrypt(plaintext []byte) ([]byte, error) {
	return sek.box.Encrypt(plaintext)
}

// Decrypt ciphertext.
func (sek *SymmetricEcdhKey) Decrypt(ciphertext []byte) (plaintext []byte, err error) {
	return sek.box.Decrypt(ciphertext)
}
