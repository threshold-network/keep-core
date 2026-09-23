package pbutils

import (
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/btcec"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
)

// FuzzEphemeralKeyRoundTrip tests the round-trip serialization and internal
// consistency of ephemeral public and private keys derived from fuzzed scalars.
//
// This target catches two generator bugs that previously broke round trips:
//  1. fuzzEphemeralPublicKey used to fuzz X and Y coordinates independently as
//     arbitrary big.Ints, producing points not on the secp256k1 curve that failed
//     UnmarshalPublicKey (ParsePubKey). Deriving (X, Y) via ScalarBaseMult from
//     a scalar reduced into [1, N-1] ensures valid curve points that round-trip.
//  2. fuzzEphemeralPrivateKey used to fuzz the public key and scalar D independently,
//     yielding inconsistent keypairs where D did not generate the public point.
//     Deriving both halves from the same normalized scalar via PrivKeyFromBytes
//     ensures consistency verified by IsKeyMatching and ECDH compatibility.
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
