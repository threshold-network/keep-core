package tecdsa

import (
	"crypto/ecdsa"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/btcsuite/btcd/btcec"
)

// Curve is the curve implementation used across the tecdsa package.
//
// IMPORTANT NOTE: The elliptic.UnmarshalCompressed function does not work
// as expected for this curve and always produces nil point coordinates.
// This is because the elliptic.UnmarshalCompressed always execute the
// y² = x³ - 3x + b equation to compute the y coordinate while the actual
// equation of the Curve (secp256k1) is y² = x³ + 7, i.e. the `a` parameter
// is 0 not -3.
var Curve = tss.S256()

// PrivateKeyShare represents a private key share used to produce tECDSA
// signatures. Private key shares are generated as result of the tECDSA
// distributed key generation (DKG) process.
type PrivateKeyShare struct {
	data keygen.LocalPartySaveData
}

// NewPrivateKeyShare constructs a new instance of the tECDSA public key
// share based on the DKG result.
func NewPrivateKeyShare(data keygen.LocalPartySaveData) *PrivateKeyShare {
	return &PrivateKeyShare{data}
}

// PublicKey returns the ECDSA public key corresponding to the given tECDSA
// private key share.
//
// The key is expressed on the btcec secp256k1 curve object that keep-core
// uses when it parses wallet keys from the chain and from storage
// (pkg/crypto/secp256k1), not on tss-lib's own curve object. Both describe
// secp256k1, but ecdsa.PublicKey.Equal and reflect.DeepEqual also compare the
// curve object, so keys for the same wallet must share one.
func (pks *PrivateKeyShare) PublicKey() *ecdsa.PublicKey {
	curve := pks.data.ECDSAPub.Curve()
	if curve.Params().Name == Curve.Params().Name {
		curve = btcec.S256()
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     pks.data.ECDSAPub.X(),
		Y:     pks.data.ECDSAPub.Y(),
	}
}

// Data returns the internal data of the private key share.
func (pks *PrivateKeyShare) Data() keygen.LocalPartySaveData {
	return pks.data
}
