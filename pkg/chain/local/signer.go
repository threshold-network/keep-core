package local

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"fmt"
	"math/big"
)

// Signer provides functions to sign a message and verify a signature
// using the chain-specific signature format. It also provides functions for
// convertion of a public key to address.
type Signer struct {
	operatorKey *ecdsa.PrivateKey
}

type ecdsaSignature struct {
	R, S *big.Int
}

// NewSigner creates a new Signer instance for the provided private key.
func NewSigner(privateKey *ecdsa.PrivateKey) *Signer {
	return &Signer{privateKey}
}

// PublicKey returns byte representation of a public key for the private key
// signer was created with.
func (ls *Signer) PublicKey() []byte {
	publicKey := ls.operatorKey.PublicKey
	return elliptic.Marshal(publicKey.Curve, publicKey.X, publicKey.Y)
}

// Sign signs the provided message.
func (ls *Signer) Sign(message []byte) ([]byte, error) {
	hash := sha256.Sum256(message)

	r, s, err := ecdsa.Sign(rand.Reader, ls.operatorKey, hash[:])
	if err != nil {
		return nil, err
	}

	return asn1.Marshal(ecdsaSignature{r, s})
}

// Verify verifies the provided message against a signature using the key
// Signer was created with.
func (ls *Signer) Verify(message []byte, signature []byte) (bool, error) {
	return verifySignature(message, signature, &ls.operatorKey.PublicKey)
}

// VerifyWithPublicKey verifies the provided message against a signature and
// public key.
func (ls *Signer) VerifyWithPublicKey(
	message []byte,
	signature []byte,
	publicKey []byte,
) (bool, error) {
	unmarshalledPubKey, err := unmarshalPublicKey(
		publicKey,
		ls.operatorKey.Curve,
	)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal public key: [%v]", err)
	}

	return verifySignature(message, signature, unmarshalledPubKey)
}

func verifySignature(
	message []byte,
	signature []byte,
	publicKey *ecdsa.PublicKey,
) (bool, error) {
	hash := sha256.Sum256(message)

	sig := &ecdsaSignature{}
	_, err := asn1.Unmarshal(signature, sig)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal signature: [%v]", err)
	}

	return ecdsa.Verify(publicKey, hash[:], sig.R, sig.S), nil
}

func unmarshalPublicKey(
	bytes []byte,
	curve elliptic.Curve,
) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(curve, bytes)
	if x == nil {
		return nil, fmt.Errorf(
			"invalid public key bytes",
		)
	}
	ecdsaPublicKey := &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
	return (*ecdsa.PublicKey)(ecdsaPublicKey), nil
}

// PublicKeyToAddress transforms the provided ECDSA public key into chain
// address represented in bytes.
func (ls *Signer) PublicKeyToAddress(publicKey ecdsa.PublicKey) []byte {
	return elliptic.Marshal(publicKey.Curve, publicKey.X, publicKey.Y)
}

// PublicKeyBytesToAddress transforms the provided ECDSA public key in a bytes
// format into chain address represented in bytes.
func (ls *Signer) PublicKeyBytesToAddress(publicKey []byte) []byte {
	return publicKey
}
