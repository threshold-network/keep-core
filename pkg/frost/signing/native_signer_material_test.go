package signing

import (
	"bytes"
	"strings"
	"testing"
)

func TestRequest_NativeSignerMaterial_FromPointer(t *testing.T) {
	input := &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV1,
		Payload: []byte{0x01, 0x02, 0x03},
	}

	request := &Request{
		SignerMaterial: input,
	}

	result, err := request.NativeSignerMaterial()
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if result == input {
		t.Fatal("expected a clone of native signer material")
	}

	if result.Format != input.Format {
		t.Fatalf(
			"unexpected signer material format\nexpected: [%v]\nactual:   [%v]",
			input.Format,
			result.Format,
		)
	}

	if !bytes.Equal(result.Payload, input.Payload) {
		t.Fatalf(
			"unexpected signer material payload\nexpected: [%x]\nactual:   [%x]",
			input.Payload,
			result.Payload,
		)
	}
}

func TestRequest_NativeSignerMaterial_FromValue(t *testing.T) {
	request := &Request{
		SignerMaterial: NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: []byte{0xaa, 0xbb},
		},
	}

	result, err := request.NativeSignerMaterial()
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if result.Format != NativeSignerMaterialFormatFrostUniFFIV1 {
		t.Fatalf(
			"unexpected signer material format\nexpected: [%v]\nactual:   [%v]",
			NativeSignerMaterialFormatFrostUniFFIV1,
			result.Format,
		)
	}
}

func TestRequest_NativeSignerMaterial_FromBytesUsesDefaultFormat(t *testing.T) {
	request := &Request{
		SignerMaterial: []byte{0x10, 0x20},
	}

	result, err := request.NativeSignerMaterial()
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if result.Format != NativeSignerMaterialFormatFrostUniFFIV1 {
		t.Fatalf(
			"unexpected signer material format\nexpected: [%v]\nactual:   [%v]",
			NativeSignerMaterialFormatFrostUniFFIV1,
			result.Format,
		)
	}
}

func TestRequest_NativeSignerMaterial_NilRequest(t *testing.T) {
	_, err := (*Request)(nil).NativeSignerMaterial()
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

func TestRequest_NativeSignerMaterial_NilMaterial(t *testing.T) {
	_, err := (&Request{}).NativeSignerMaterial()
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "native signer material is nil") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"native signer material is nil",
			err,
		)
	}
}

func TestRequest_NativeSignerMaterial_WrongType(t *testing.T) {
	request := &Request{
		SignerMaterial: "invalid",
	}

	_, err := request.NativeSignerMaterial()
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "native signer material has wrong type") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"native signer material has wrong type",
			err,
		)
	}
}

func TestRequest_NativeSignerMaterial_ValidationFailure(t *testing.T) {
	request := &Request{
		SignerMaterial: NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: []byte{},
		},
	}

	_, err := request.NativeSignerMaterial()
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "native signer material payload is empty") {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"native signer material payload is empty",
			err,
		)
	}
}
