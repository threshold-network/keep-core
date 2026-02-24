package tbtc

import (
	"bytes"
	"reflect"
	"testing"
)

func assertSignerEquivalent(
	t *testing.T,
	name string,
	expected *signer,
	actual *signer,
) {
	t.Helper()

	if expected == nil {
		if actual != nil {
			t.Fatalf("%s should be nil", name)
		}
		return
	}

	if actual == nil {
		t.Fatalf("%s is nil", name)
	}

	if !expected.wallet.publicKey.Equal(actual.wallet.publicKey) {
		t.Fatalf("%s has unexpected wallet public key", name)
	}

	if !reflect.DeepEqual(
		expected.wallet.signingGroupOperators,
		actual.wallet.signingGroupOperators,
	) {
		t.Fatalf(
			"%s has unexpected signing group operators\nexpected: [%v]\nactual:   [%v]",
			name,
			expected.wallet.signingGroupOperators,
			actual.wallet.signingGroupOperators,
		)
	}

	if expected.signingGroupMemberIndex != actual.signingGroupMemberIndex {
		t.Fatalf(
			"%s has unexpected member index\nexpected: [%v]\nactual:   [%v]",
			name,
			expected.signingGroupMemberIndex,
			actual.signingGroupMemberIndex,
		)
	}

	if expected.privateKeyShare == nil {
		if actual.privateKeyShare != nil {
			t.Fatalf("%s should have nil private key share", name)
		}
		return
	}

	if actual.privateKeyShare == nil {
		t.Fatalf("%s has nil private key share", name)
	}

	expectedPrivateKeyShare, err := expected.privateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("cannot marshal expected private key share for %s: [%v]", name, err)
	}

	actualPrivateKeyShare, err := actual.privateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("cannot marshal actual private key share for %s: [%v]", name, err)
	}

	if !bytes.Equal(expectedPrivateKeyShare, actualPrivateKeyShare) {
		t.Fatalf(
			"%s has unexpected private key share\nexpected: [%x]\nactual:   [%x]",
			name,
			expectedPrivateKeyShare,
			actualPrivateKeyShare,
		)
	}
}
