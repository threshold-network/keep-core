package pbutils

import (
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/btcec"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
)

// FuzzEphemeralKeyRoundTrip exercises marshal and unmarshal of ephemeral public
// and private keys across the whole scalar domain, including the boundaries at 1
// and N-1 that a fixed-count generator loop is unlikely to reach.
//
// Scope, stated precisely because it is easy to overstate: this target builds its
// keys directly from a scalar, so it does NOT exercise fuzzEphemeralPublicKey or
// fuzzEphemeralPrivateKey, and it would NOT fail if either were regressed to
// fuzzing coordinates independently again. The guards against that are
// TestFuzzedEphemeralPublicKeyIsSerializable and
// TestFuzzedEphemeralPrivateKeyIsConsistent, which do call FuzzFuncs.
//
// What this target does guard is the shared normalizeScalar helper, which the
// production generators also use, and the marshaling itself against any scalar
// the engine can reach.
func FuzzEphemeralKeyRoundTrip(f *testing.F) {
	curve := btcec.S256()

	// Seed 1: Empty and all-zero byte slices (normalize to scalar 1).
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add(make([]byte, 32))

	// Seed 2: Small inputs producing small in-range scalars.
	f.Add([]byte{0x01})
	f.Add([]byte{0x02})
	f.Add(big.NewInt(42).Bytes())

	// Seed 3: Large inputs near the top of the accepted domain [1, N-1].
	// N - 2 and N - 1 test boundaries near the curve order.
	topMinus2 := new(big.Int).Sub(curve.N, big.NewInt(2))
	topMinus1 := new(big.Int).Sub(curve.N, big.NewInt(1))
	f.Add(topMinus2.Bytes())
	f.Add(topMinus1.Bytes())

	f.Fuzz(func(t *testing.T, data []byte) {
		scalar := normalizeScalar(new(big.Int).SetBytes(data))

		// 1. Ephemeral Public Key Round Trip
		x, y := curve.ScalarBaseMult(scalar.Bytes())
		originalPubKey := &ephemeral.PublicKey{
			Curve: curve,
			X:     x,
			Y:     y,
		}

		marshaledPubKey := originalPubKey.Marshal()
		if len(marshaledPubKey) != 33 {
			t.Fatalf(
				"unexpected marshaled public key length: expected 33, got %d",
				len(marshaledPubKey),
			)
		}

		unmarshaledPubKey, err := ephemeral.UnmarshalPublicKey(marshaledPubKey)
		if err != nil {
			t.Fatalf(
				"failed to unmarshal valid ephemeral public key: %v",
				err,
			)
		}

		// Explicit field comparison on (X, Y) coordinates using big.Int.Cmp
		// to verify mathematical equality and format clear mismatch diagnostics.
		if originalPubKey.X.Cmp(unmarshaledPubKey.X) != 0 ||
			originalPubKey.Y.Cmp(unmarshaledPubKey.Y) != 0 {
			t.Fatalf(
				"public key round-trip inequality\n"+
					"original:    (X: %s, Y: %s)\n"+
					"unmarshaled: (X: %s, Y: %s)",
				originalPubKey.X.String(),
				originalPubKey.Y.String(),
				unmarshaledPubKey.X.String(),
				unmarshaledPubKey.Y.String(),
			)
		}

		// 2. Ephemeral Private Key Consistency and Round Trip
		priv, _ := btcec.PrivKeyFromBytes(curve, scalar.Bytes())
		originalPrivKey := (*ephemeral.PrivateKey)(priv)

		pubFromPriv := (*ephemeral.PublicKey)(&originalPrivKey.PublicKey)
		if !pubFromPriv.IsKeyMatching(originalPrivKey) {
			t.Fatalf(
				"private key D does not match its public key\n"+
					"D: (D: %s)\n"+
					"pub: (X: %s, Y: %s)",
				originalPrivKey.D.String(),
				pubFromPriv.X.String(),
				pubFromPriv.Y.String(),
			)
		}

		marshaledPrivKey := originalPrivKey.Marshal()
		unmarshaledPrivKey := ephemeral.UnmarshalPrivateKey(marshaledPrivKey)
		if unmarshaledPrivKey == nil {
			t.Fatalf("failed to unmarshal valid ephemeral private key")
		}

		// Explicit field comparison on scalar D and public coordinates (X, Y).
		if originalPrivKey.D.Cmp(unmarshaledPrivKey.D) != 0 ||
			originalPrivKey.X.Cmp(unmarshaledPrivKey.X) != 0 ||
			originalPrivKey.Y.Cmp(unmarshaledPrivKey.Y) != 0 {
			t.Fatalf(
				"private key round-trip inequality\n"+
					"original:    (D: %s, X: %s, Y: %s)\n"+
					"unmarshaled: (D: %s, X: %s, Y: %s)",
				originalPrivKey.D.String(),
				originalPrivKey.X.String(),
				originalPrivKey.Y.String(),
				unmarshaledPrivKey.D.String(),
				unmarshaledPrivKey.X.String(),
				unmarshaledPrivKey.Y.String(),
			)
		}
	})
}
