// Package secp256k1 provides helpers for encoding and decoding secp256k1
// public keys.
package secp256k1

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
)

const uncompressedPublicKeyPrefix = byte(0x04)

// isSecp256k1 checks that the given curve parameters describe the secp256k1
// curve. The comparison uses the field prime and group order instead of the
// curve name so that keys held with any secp256k1 implementation are
// accepted, regardless of how that implementation labels itself.
func isSecp256k1(params *elliptic.CurveParams) bool {
	s256 := btcec.S256().Params()
	return params != nil &&
		params.P != nil &&
		params.N != nil &&
		params.P.Cmp(s256.P) == 0 &&
		params.N.Cmp(s256.N) == 0
}

// Marshal converts a secp256k1 public key to the 65-byte uncompressed form
// specified in SEC 1, Version 2.0, Section 2.3.3.
func Marshal(publicKey *ecdsa.PublicKey) []byte {
	curve := btcec.S256()
	if publicKey == nil ||
		publicKey.Curve == nil ||
		publicKey.X == nil ||
		publicKey.Y == nil ||
		!isSecp256k1(publicKey.Curve.Params()) ||
		publicKey.X.Sign() < 0 ||
		publicKey.Y.Sign() < 0 ||
		publicKey.X.Cmp(curve.Params().P) >= 0 ||
		publicKey.Y.Cmp(curve.Params().P) >= 0 ||
		!curve.IsOnCurve(publicKey.X, publicKey.Y) {
		panic("invalid secp256k1 public key")
	}

	return (*btcec.PublicKey)(publicKey).SerializeUncompressed()
}

// Unmarshal converts a 65-byte uncompressed SEC 1 encoding to a secp256k1
// public key.
func Unmarshal(bytes []byte) (*ecdsa.PublicKey, error) {
	if len(bytes) != btcec.PubKeyBytesLenUncompressed {
		return nil, fmt.Errorf(
			"invalid uncompressed public key length: [%v]",
			len(bytes),
		)
	}

	if bytes[0] != uncompressedPublicKeyPrefix {
		return nil, fmt.Errorf(
			"invalid uncompressed public key prefix: [0x%x]",
			bytes[0],
		)
	}

	publicKey, err := btcec.ParsePubKey(bytes, btcec.S256())
	if err != nil {
		return nil, fmt.Errorf("invalid secp256k1 public key: [%w]", err)
	}

	return publicKey.ToECDSA(), nil
}
