package ephemeral

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
)

// ErrInvalidPublicKey is returned by UnmarshalPublicKey when the given bytes
// do not decode to a valid point on the curve. Wrapped into the returned
// error via %w so callers up the stack (including retry-policy code) can
// classify the failure with errors.Is regardless of the underlying decoder's
// error type.
var ErrInvalidPublicKey = errors.New("invalid ephemeral public key")

// PrivateKey is an ephemeral private elliptic curve key.
type PrivateKey btcec.PrivateKey

// PublicKey is an ephemeral public elliptic curve key.
type PublicKey btcec.PublicKey

// KeyPair represents the generated ephemeral elliptic curve private and public
// key pair
type KeyPair struct {
	PrivateKey *PrivateKey
	PublicKey  *PublicKey
}

func curve() *btcec.KoblitzCurve {
	return btcec.S256()
}

// GenerateKeyPair generates a pair of public and private elliptic curve
// ephemeral key that can be used as an input for ECDH.
func GenerateKeyPair() (*KeyPair, error) {
	ecdsaKey, err := btcec.NewPrivateKey(curve())
	if err != nil {
		return nil, fmt.Errorf(
			"could not generate new ephemeral keypair: [%v]",
			err,
		)
	}

	return &KeyPair{
		(*PrivateKey)(ecdsaKey),
		(*PublicKey)(&ecdsaKey.PublicKey),
	}, nil
}

// IsKeyMatching verifies if private key is valid for given public key.
// It checks if public key equals `g^privateKey`, where `g` is a base point of
// the curve.
func (pk *PublicKey) IsKeyMatching(privateKey *PrivateKey) bool {
	expectedX, expectedY := curve().ScalarBaseMult(privateKey.Marshal())
	return expectedX.Cmp(pk.X) == 0 && expectedY.Cmp(pk.Y) == 0
}

// UnmarshalPrivateKey turns a slice of bytes into a `PrivateKey`.
func UnmarshalPrivateKey(bytes []byte) *PrivateKey {
	priv, _ := btcec.PrivKeyFromBytes(curve(), bytes)
	return (*PrivateKey)(priv)
}

// UnmarshalPublicKey turns a slice of bytes into a `PublicKey`.
func UnmarshalPublicKey(bytes []byte) (*PublicKey, error) {
	pubKey, err := btcec.ParsePubKey(bytes, curve())
	if err != nil {
		return nil, fmt.Errorf("%w: [%w]", ErrInvalidPublicKey, err)
	}

	return (*PublicKey)(pubKey), nil
}

// Marshal turns a `PrivateKey` into a slice of bytes.
func (pk *PrivateKey) Marshal() []byte {
	return (*btcec.PrivateKey)(pk).Serialize()
}

// Marshal turns a `PublicKey` into a slice of bytes.
func (pk *PublicKey) Marshal() []byte {
	return (*btcec.PublicKey)(pk).SerializeCompressed()
}
