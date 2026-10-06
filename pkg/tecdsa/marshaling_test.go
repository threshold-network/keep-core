package tecdsa

import (
	"crypto/elliptic"
	"math/big"
	"reflect"
	"testing"

	fuzz "github.com/google/gofuzz"

	"github.com/bnb-chain/tss-lib/crypto"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
)

func TestPrivateKeyShareMarshalling(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	privateKeyShare := NewPrivateKeyShare(testData[0])

	unmarshaled := &PrivateKeyShare{}

	if err := pbutils.RoundTrip(privateKeyShare, unmarshaled); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(privateKeyShare, unmarshaled) {
		t.Fatal("unexpected content of unmarshaled private key share")
	}
}

func TestPrivateKeyShareMarshalling_NonTECDSAKey(t *testing.T) {
	testData, err := tecdsatest.LoadPrivateKeyShareTestFixtures(1)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	privateKeyShare := NewPrivateKeyShare(testData[0])

	p256 := elliptic.P256()

	ecPoint, err := crypto.NewECPoint(p256, p256.Params().Gx, p256.Params().Gy)
	if err != nil {
		t.Fatal(err)
	}

	// Use a non-secp256k1 based key to cause the expected failure.
	privateKeyShare.data.ECDSAPub = ecPoint

	_, err = privateKeyShare.Marshal()

	testutils.AssertErrorsSame(t, ErrIncompatiblePublicKey, err)
}

func TestSignature_MarshalingRoundtrip(t *testing.T) {
	signature := &Signature{
		R:          big.NewInt(100),
		S:          big.NewInt(200),
		RecoveryID: 2,
	}

	unmarshaled := &Signature{}

	if err := pbutils.RoundTrip(signature, unmarshaled); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(signature, unmarshaled) {
		t.Fatal("unexpected content of unmarshaled signature")
	}
}

func TestFuzzSignature_MarshalingRoundtrip(t *testing.T) {
	for i := 0; i < 10; i++ {
		var (
			r          big.Int
			s          big.Int
			recoveryID int8
		)

		f := fuzz.New().NilChance(0.1).
			NumElements(0, 512).
			Funcs(pbutils.FuzzFuncs()...)

		f.Fuzz(&r)
		f.Fuzz(&s)
		f.Fuzz(&recoveryID)

		signature := &Signature{
			R:          &r,
			S:          &s,
			RecoveryID: recoveryID,
		}

		if err := pbutils.RoundTrip(signature, &Signature{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFuzzSignature_Unmarshaler(t *testing.T) {
	pbutils.AssertUnmarshalDoesNotPanic(&Signature{})
}
