package operator

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
)

// Curve represents an elliptic curve name that denotes the curve used by
// the operator keys.
type Curve int

// Enum that lists all the elliptic curves that can be used with the operator
// keys.
const (
	Secp256k1 Curve = iota
)

// Holds the bit size of the supported elliptic curves. All curves specified by
// the Curve enum MUST have the bit size specified here.
var curveBitSizes = map[Curve]int{
	Secp256k1: 256,
}

// ParseCurve takes a curve name as string and parses it to a specific
// operator.Curve enum instance.
func ParseCurve(value string) (Curve, error) {
	switch value {
	case "secp256k1":
		return Secp256k1, nil
	}

	return -1, fmt.Errorf("unknown curve: [%v]", value)
}

// String returns the string representation of a given operator.Curve
// enum instance.
func (c Curve) String() string {
	switch c {
	case Secp256k1:
		return "secp256k1"
	default:
		panic("unknown curve")
	}
}

// PublicKey represents a public key that identifies the node operator.
// Actually this is an ECDSA public key that is not tied to any particular
// elliptic curve implementation and holds just the curve name instead.
// That trait makes the operator key flexible enough to be accepted by chain
// and network packages that can convert it to the chain and network key
// respectively. Chain and network keys should use their own implementations
// of the curve held by the operator key.
type PublicKey struct {
	Curve Curve
	X, Y  *big.Int
}

// String returns the hexadecimal representation of the public key in the
// compressed form prefixed with 02 or 03 byte.
func (pk *PublicKey) String() string {
	return hex.EncodeToString(MarshalCompressed(pk))
}

// PrivateKey represents an operator private key corresponding to an
// operator public key.
type PrivateKey struct {
	PublicKey
	D *big.Int
}

// GenerateKeyPair generates an operator key pair using the provided elliptic
// curve instance. The provided curve instance's name must resolve to one of
// the supported curves determined by the operator.Curve enum.
func GenerateKeyPair(
	curveInstance elliptic.Curve,
) (*PrivateKey, *PublicKey, error) {
	curve, err := ParseCurve(curveInstance.Params().Name)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse curve name: [%v]", err)
	}

	generated, err := ecdsa.GenerateKey(curveInstance, rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot generate key: [%v]", err)
	}

	publicKey := PublicKey{
		Curve: curve,
		X:     generated.X,
		Y:     generated.Y,
	}

	privateKey := PrivateKey{
		PublicKey: publicKey,
		D:         generated.D,
	}

	return &privateKey, &publicKey, nil
}

// MarshalUncompressed marshals the given public key to the 65-byte uncompressed
// form. This implementation is based on the elliptic.Marshal function.
func MarshalUncompressed(publicKey *PublicKey) []byte {
	curveBitSize, exists := curveBitSizes[publicKey.Curve]
	if !exists {
		// Should not happen but just in case.
		panic("missing curve bit size information")
	}

	byteLength := (curveBitSize + 7) / 8

	uncompressed := make([]byte, 1+2*byteLength)
	uncompressed[0] = 4 // uncompressed point

	publicKey.X.FillBytes(uncompressed[1 : 1+byteLength])
	publicKey.Y.FillBytes(uncompressed[1+byteLength : 1+2*byteLength])

	return uncompressed
}

// MarshalCompressed marshals the given public key to the 33-byte compressed
// form. This implementation is based on the elliptic.MarshalCompressed
// function.
func MarshalCompressed(publicKey *PublicKey) []byte {
	curveBitSize, exists := curveBitSizes[publicKey.Curve]
	if !exists {
		// Should not happen but just in case.
		panic("missing curve bit size information")
	}

	byteLength := (curveBitSize + 7) / 8
	compressed := make([]byte, 1+byteLength)
	compressed[0] = byte(publicKey.Y.Bit(0)&1) | 2
	publicKey.X.FillBytes(compressed[1:])

	return compressed
}
