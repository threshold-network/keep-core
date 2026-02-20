package signing

import (
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestRequest_LegacyPrivateKeyShare_FromDeprecatedField(t *testing.T) {
	expected := new(tecdsa.PrivateKeyShare)

	request := &Request{
		PrivateKeyShare: expected,
	}

	actual, err := request.LegacyPrivateKeyShare()
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if actual != expected {
		t.Fatalf(
			"unexpected private key share\nexpected: [%v]\nactual:   [%v]",
			expected,
			actual,
		)
	}
}

func TestRequest_LegacyPrivateKeyShare_FromSignerMaterial(t *testing.T) {
	expected := new(tecdsa.PrivateKeyShare)

	request := &Request{
		SignerMaterial: expected,
	}

	actual, err := request.LegacyPrivateKeyShare()
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if actual != expected {
		t.Fatalf(
			"unexpected private key share\nexpected: [%v]\nactual:   [%v]",
			expected,
			actual,
		)
	}
}

func TestRequest_LegacyPrivateKeyShare_NilRequest(t *testing.T) {
	_, err := (*Request)(nil).LegacyPrivateKeyShare()
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "request is nil") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"request is nil",
			err,
		)
	}
}

func TestRequest_LegacyPrivateKeyShare_NilMaterial(t *testing.T) {
	_, err := (&Request{}).LegacyPrivateKeyShare()
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "legacy private key share is nil") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"legacy private key share is nil",
			err,
		)
	}
}

func TestRequest_LegacyPrivateKeyShare_WrongMaterialType(t *testing.T) {
	request := &Request{
		SignerMaterial: "invalid",
	}

	_, err := request.LegacyPrivateKeyShare()
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "legacy signing material has wrong type") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"legacy signing material has wrong type",
			err,
		)
	}
}

func TestRequest_LegacyPrivateKeyShare_NilTypedMaterial(t *testing.T) {
	var typedNil *tecdsa.PrivateKeyShare

	request := &Request{
		SignerMaterial: typedNil,
	}

	_, err := request.LegacyPrivateKeyShare()
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "legacy private key share is nil") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"legacy private key share is nil",
			err,
		)
	}
}
