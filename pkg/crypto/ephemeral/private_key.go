package ephemeral

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec"
)

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
		return nil, fmt.Errorf("could not parse ephemeral public key: [%v]", err)
	}

	return (*PublicKey)(pubKey), nil
}

// Marshal turns a `PrivateKey` into a slice of bytes.
func (pk *PrivateKey) Marshal() []byte {
	return (*btcec.PrivateKey)(pk).Serialize()
}

// Zero best-effort scrubs the private key's secret scalar in place, so a
// short-lived key (a one-time or per-attempt ephemeral) does not linger in the
// heap - and thus a core dump or swap file - after its single use. big.Int does
// not expose zeroization, so this wipes the backing words of D directly. Callers
// that hold an ephemeral key should Zero() it as soon as they are done with it.
func (pk *PrivateKey) Zero() {
	if pk == nil {
		return
	}
	d := (*btcec.PrivateKey)(pk).D
	if d == nil {
		return
	}
	words := d.Bits()
	for i := range words {
		words[i] = 0
	}
	d.SetInt64(0)
}

// Marshal turns a `PublicKey` into a slice of bytes.
func (pk *PublicKey) Marshal() []byte {
	return (*btcec.PublicKey)(pk).SerializeCompressed()
}
