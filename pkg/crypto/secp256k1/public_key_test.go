package secp256k1

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/btcec"
)

func TestPublicKeyRoundtrip(t *testing.T) {
	expected, err := hex.DecodeString(
		"0479be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798" +
			"483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8",
	)
	if err != nil {
		t.Fatal(err)
	}

	curve := btcec.S256()
	publicKey := &ecdsa.PublicKey{
		Curve: curve,
		X:     curve.Params().Gx,
		Y:     curve.Params().Gy,
	}

	marshaled := Marshal(publicKey)
	if hex.EncodeToString(marshaled) != hex.EncodeToString(expected) {
		t.Fatalf(
			"unexpected public key encoding\nexpected: %x\nactual:   %x",
			expected,
			marshaled,
		)
	}

	unmarshaled, err := Unmarshal(marshaled)
	if err != nil {
		t.Fatal(err)
	}

	if unmarshaled.Curve.Params().Name != publicKey.Curve.Params().Name ||
		unmarshaled.X.Cmp(publicKey.X) != 0 ||
		unmarshaled.Y.Cmp(publicKey.Y) != 0 {
		t.Fatal("unexpected public key after roundtrip")
	}
}

func TestUnmarshalMalformedPublicKey(t *testing.T) {
	compressed, err := hex.DecodeString(
		"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
	)
	if err != nil {
		t.Fatal(err)
	}

	hybrid, err := hex.DecodeString(
		"0679be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798" +
			"483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8",
	)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string][]byte{
		"empty":               nil,
		"compressed":          compressed,
		"hybrid":              hybrid,
		"invalid prefix":      append([]byte{0x05}, make([]byte, 64)...),
		"point off the curve": append([]byte{0x04}, make([]byte, 64)...),
	}

	for testName, bytes := range tests {
		t.Run(testName, func(t *testing.T) {
			if _, err := Unmarshal(bytes); err == nil {
				t.Fatal("expected an error for malformed public key")
			}
		})
	}
}

func TestMarshalAcceptsRelabeledSecp256k1Curve(t *testing.T) {
	// A secp256k1 key may be held with a curve implementation that labels
	// itself differently than btcec (e.g. a plain elliptic.CurveParams
	// copy). Such keys must marshal successfully because the curve is
	// identified by its parameters, not by its name.
	relabeled := *btcec.S256().Params()
	relabeled.Name = "relabeled-secp256k1"

	publicKey := &ecdsa.PublicKey{
		Curve: &relabeled,
		X:     new(big.Int).Set(btcec.S256().Params().Gx),
		Y:     new(big.Int).Set(btcec.S256().Params().Gy),
	}

	marshaled := Marshal(publicKey)

	if len(marshaled) != btcec.PubKeyBytesLenUncompressed {
		t.Fatalf("unexpected marshaled length: [%v]", len(marshaled))
	}

	unmarshaled, err := Unmarshal(marshaled)
	if err != nil {
		t.Fatal(err)
	}

	if unmarshaled.X.Cmp(publicKey.X) != 0 ||
		unmarshaled.Y.Cmp(publicKey.Y) != 0 {
		t.Fatal("unexpected public key after roundtrip")
	}
}

func TestMarshalRejectsInvalidPublicKey(t *testing.T) {
	p256 := elliptic.P256()
	secp256k1 := btcec.S256()
	tests := map[string]*ecdsa.PublicKey{
		"nil": nil,
		"non-secp256k1 curve": {
			Curve: p256,
			X:     p256.Params().Gx,
			Y:     p256.Params().Gy,
		},
		"point off the curve": {
			Curve: secp256k1,
			X:     new(big.Int),
			Y:     new(big.Int),
		},
		"negative x coordinate": {
			Curve: secp256k1,
			X:     big.NewInt(-1),
			Y:     new(big.Int).Set(secp256k1.Params().Gy),
		},
		"negative y coordinate": {
			Curve: secp256k1,
			X:     new(big.Int).Set(secp256k1.Params().Gx),
			Y:     big.NewInt(-1),
		},
		"oversized x coordinate": {
			Curve: secp256k1,
			X: new(big.Int).Add(
				new(big.Int).Set(secp256k1.Params().P),
				big.NewInt(1),
			),
			Y: new(big.Int).Set(secp256k1.Params().Gy),
		},
		"oversized y coordinate": {
			Curve: secp256k1,
			X:     new(big.Int).Set(secp256k1.Params().Gx),
			Y: new(big.Int).Add(
				new(big.Int).Set(secp256k1.Params().P),
				big.NewInt(1),
			),
		},
		"shifted generator coordinates": {
			Curve: secp256k1,
			X: new(big.Int).Lsh(
				new(big.Int).Set(secp256k1.Params().Gx),
				8,
			),
			Y: new(big.Int).Lsh(
				new(big.Int).Set(secp256k1.Params().Gy),
				8,
			),
		},
	}

	for testName, publicKey := range tests {
		t.Run(testName, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected a panic for an invalid public key")
				}
			}()

			Marshal(publicKey)
		})
	}
}
