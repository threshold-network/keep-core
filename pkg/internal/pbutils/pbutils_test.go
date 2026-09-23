package pbutils

import (
	"testing"

	fuzz "github.com/google/gofuzz"

	"github.com/keep-network/keep-core/pkg/crypto/ephemeral"
)

// TestFuzzedEphemeralPublicKeyIsSerializable checks that every public key the
// fuzzer produces survives a Marshal/Unmarshal round trip.
//
// This is the contract the round-trip marshaling tests across pkg/beacon and
// pkg/tecdsa rely on: they fuzz a message, round-trip it, and assert no error.
// A generator that emits points which are not on the curve makes those tests
// unable to pass, which is how this was previously masked by a discarded error.
func TestFuzzedEphemeralPublicKeyIsSerializable(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(FuzzFuncs()...)

	for i := 0; i < 100; i++ {
		var key ephemeral.PublicKey

		f.Fuzz(&key)

		marshaled := key.Marshal()
		if len(marshaled) != 33 {
			t.Fatalf(
				"unexpected compressed public key length\n"+
					"expected: [33]\n"+
					"actual:   [%d]",
				len(marshaled),
			)
		}

		unmarshaled, err := ephemeral.UnmarshalPublicKey(marshaled)
		if err != nil {
			t.Fatal(err)
		}

		if key.X.Cmp(unmarshaled.X) != 0 || key.Y.Cmp(unmarshaled.Y) != 0 {
			t.Fatalf(
				"public key does not round-trip\n"+
					"expected: [%v, %v]\n"+
					"actual:   [%v, %v]",
				key.X,
				key.Y,
				unmarshaled.X,
				unmarshaled.Y,
			)
		}
	}
}

// TestFuzzedEphemeralPrivateKeyIsConsistent checks that the fuzzer produces
// private keys whose D actually generates their own public point.
//
// The generator used to fuzz the public key and D independently, so the two
// halves disagreed. Nothing round-trips an ephemeral private key today, which
// is why that never surfaced - but IsKeyMatching would reject such a key and
// any ECDH derived from it is meaningless, so the next test to touch one would
// have inherited the same silent-garbage problem the round-trip repairs exposed
// elsewhere.
func TestFuzzedEphemeralPrivateKeyIsConsistent(t *testing.T) {
	f := fuzz.New().NilChance(0).Funcs(FuzzFuncs()...)

	for i := 0; i < 100; i++ {
		var key ephemeral.PrivateKey

		f.Fuzz(&key)

		publicKey := (*ephemeral.PublicKey)(&key.PublicKey)
		if !publicKey.IsKeyMatching(&key) {
			t.Fatal("fuzzed private key does not match its own public key")
		}

		marshaled := publicKey.Marshal()
		if _, err := ephemeral.UnmarshalPublicKey(marshaled); err != nil {
			t.Fatal(err)
		}
	}
}
