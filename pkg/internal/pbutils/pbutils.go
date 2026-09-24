// Package pbutils provides helper utilities for working with protobuf objects.
// These utilities are mostly aimed at testing.
package pbutils

import (
	"math/big"

	"github.com/btcsuite/btcd/btcec"
	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	fuzz "github.com/google/gofuzz"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
	"github.com/keep-network/keep-core/pkg/internal/pb"
)

// RoundTrip takes a marshaler and unmarshaler, marshals the marshaler, and then
// unmarshals the result into the unmarshaler. If either procedure errors out,
// it returns an error; otherwise it returns nil and the unmarshaler is left
// with the results of the round-trip.
//
// This is a utility meant to facilitate tests that verify round-trip marshaling
// of objects with custom protobuf marshaling.
func RoundTrip(
	marshaler pb.Marshaler,
	unmarshaler pb.Unmarshaler,
) error {
	bytes, err := marshaler.Marshal()
	if err != nil {
		return err
	}

	err = unmarshaler.Unmarshal(bytes)
	if err != nil {
		return err
	}

	return nil
}

// AssertUnmarshalDoesNotPanic feeds the given unmarshaler 100 random byte
// slices and fails the test only if one of them panics.
//
// It deliberately discards the unmarshal error: random bytes are almost never a
// valid message, so requiring a nil error would fail constantly. That makes
// this a crash check, not a correctness check. It cannot detect an unmarshaler
// that silently accepts a malformed message, so a caller needing that guarantee
// must assert it separately.
func AssertUnmarshalDoesNotPanic(unmarshaler pb.Unmarshaler) {
	for i := 0; i < 100; i++ {
		var messageBytes []byte

		f := fuzz.New().NilChance(0.01).NumElements(0, 512)
		f.Fuzz(&messageBytes)

		_ = unmarshaler.Unmarshal(messageBytes)
	}
}

// FuzzFuncs returns custom fuzzing functions set.
func FuzzFuncs() []interface{} {
	return []interface{}{
		fuzzBigInt(),
		fuzzEphemeralPublicKey(),
		fuzzEphemeralPrivateKey(),
		fuzzG1(),
		fuzzG2(),
	}
}

func fuzzBigInt() func(*big.Int, fuzz.Continue) {
	return func(int *big.Int, c fuzz.Continue) {
		var abs []big.Word

		c.Fuzz(&abs)

		int.SetBits(abs)
	}
}

func fuzzEphemeralPublicKey() func(*ephemeral.PublicKey, fuzz.Continue) {
	return func(key *ephemeral.PublicKey, c fuzz.Continue) {
		var scalar big.Int

		c.Fuzz(&scalar)

		// Derive the point from a scalar instead of fuzzing X and Y directly.
		// Arbitrary X and Y are almost never a point on the curve, and their
		// compressed form is not 33 bytes, so ParsePubKey rejects it and no
		// round-trip through Marshal/Unmarshal can succeed.
		curve := btcec.S256()
		x, y := curve.ScalarBaseMult(normalizeScalar(&scalar).Bytes())

		key.Curve = curve
		key.X = x
		key.Y = y
	}
}

func fuzzEphemeralPrivateKey() func(*ephemeral.PrivateKey, fuzz.Continue) {
	return func(key *ephemeral.PrivateKey, c fuzz.Continue) {
		var scalar big.Int

		c.Fuzz(&scalar)

		// Derive both halves from one scalar so the key pair is internally
		// consistent. Fuzzing the public key and D independently produces a
		// private key whose D does not generate its own public point, which
		// IsKeyMatching rejects and which makes any ECDH round trip meaningless.
		curve := btcec.S256()
		priv, _ := btcec.PrivKeyFromBytes(curve, normalizeScalar(&scalar).Bytes())

		*key = ephemeral.PrivateKey(*priv)
	}
}

// normalizeScalar reduces a fuzzed value into [1, N-1], the range of valid
// secp256k1 private scalars, excluding zero so the derived point is never the
// point at infinity.
func normalizeScalar(scalar *big.Int) *big.Int {
	curve := btcec.S256()

	normalized := new(big.Int).Mod(scalar, new(big.Int).Sub(curve.N, big.NewInt(1)))

	return normalized.Add(normalized, big.NewInt(1))
}

func fuzzG1() func(*bn256.G1, fuzz.Continue) {
	return func(g1 *bn256.G1, c fuzz.Continue) {
		var k big.Int

		c.Fuzz(&k)

		g1.ScalarBaseMult(&k)
	}
}

func fuzzG2() func(*bn256.G2, fuzz.Continue) {
	return func(g2 *bn256.G2, c fuzz.Continue) {
		var k big.Int

		c.Fuzz(&k)

		// trim k to reasonable number of bytes to prevent long execution
		if len(k.Bytes()) > 64 {
			k = *new(big.Int).SetBytes(k.Bytes()[:64])
		}

		g2.ScalarBaseMult(&k)
	}
}
