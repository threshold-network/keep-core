package ephemeral

import (
	"reflect"
	"testing"
)

func TestPrivateKeyZero(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	allZero := func(b []byte) bool {
		for _, x := range b {
			if x != 0 {
				return false
			}
		}
		return true
	}

	if allZero(keyPair.PrivateKey.Marshal()) {
		t.Fatal("a freshly generated private key must not be all-zero")
	}

	keyPair.PrivateKey.Zero()
	if scrubbed := keyPair.PrivateKey.Marshal(); !allZero(scrubbed) {
		t.Fatalf("Zero() must scrub the secret scalar; got [%x]", scrubbed)
	}

	// Zero is nil-safe.
	var nilKey *PrivateKey
	nilKey.Zero()
}

func TestMarshalUnmarshalPublicKey(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	marshalled := keyPair.PublicKey.Marshal()

	unmarshalled, err := UnmarshalPublicKey(marshalled)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(unmarshalled, keyPair.PublicKey) {
		t.Fatal("unmarshalled public key does not match the original one")
	}
}

func TestMarshalUnmarshalPrivateKey(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	marshalled := keyPair.PrivateKey.Marshal()
	unmarshalled := UnmarshalPrivateKey(marshalled)

	if !reflect.DeepEqual(unmarshalled, keyPair.PrivateKey) {
		t.Fatal("unmarshalled private key does not match the original one")
	}
}

func TestIsKeyMatching(t *testing.T) {
	keyPair1, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	keyPair2, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	ok := keyPair1.PublicKey.IsKeyMatching(keyPair1.PrivateKey)
	if !ok {
		t.Fatal("private key does not match the public key")
	}

	ok = keyPair1.PublicKey.IsKeyMatching(keyPair2.PrivateKey)
	if ok {
		t.Fatal("private key matches wrong public key")
	}
}
