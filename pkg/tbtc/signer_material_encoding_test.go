package tbtc

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/google/gofuzz"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/internal/pbutils"
	"github.com/keep-network/keep-core/pkg/tbtc/gen/pb"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	"google.golang.org/protobuf/proto"
)

func TestMarshalSignerMaterialForPersistence_LegacyPrivateKeyShare(t *testing.T) {
	signer := createMockSigner(t)

	encoded, err := marshalSignerMaterialForPersistence(
		signer.privateKeyShare,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected marshal error: [%v]", err)
	}

	_, isNative, err := decodeNativeSignerMaterialFromPersistence(encoded)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}

	if isNative {
		t.Fatal("expected legacy private key share encoding")
	}

	decoded := &tecdsa.PrivateKeyShare{}
	if err := decoded.Unmarshal(encoded); err != nil {
		t.Fatalf("unexpected legacy unmarshal error: [%v]", err)
	}
}

func TestMarshalSignerMaterialForPersistence_NativeSignerMaterial(t *testing.T) {
	payload := []byte{0xaa, 0xbb, 0xcc}
	encoded, err := marshalSignerMaterialForPersistence(
		&frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: payload,
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected marshal error: [%v]", err)
	}

	decoded, isNative, err := decodeNativeSignerMaterialFromPersistence(encoded)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}

	if !isNative {
		t.Fatal("expected native signer material envelope")
	}

	if decoded == nil {
		t.Fatal("expected native signer material")
	}

	if decoded.Format != frostsigning.NativeSignerMaterialFormatFrostUniFFIV1 {
		t.Fatalf(
			"unexpected decoded format\nexpected: [%v]\nactual:   [%v]",
			frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
			decoded.Format,
		)
	}

	if !bytes.Equal(decoded.Payload, payload) {
		t.Fatalf(
			"unexpected decoded payload\nexpected: [%x]\nactual:   [%x]",
			payload,
			decoded.Payload,
		)
	}
}

func TestUnmarshalSignerMaterialFromPersistence_NativeEnvelope(t *testing.T) {
	encoded, err := encodeNativeSignerMaterialForPersistence(
		frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
		[]byte{0x10, 0x20},
	)
	if err != nil {
		t.Fatalf("unexpected encode error: [%v]", err)
	}

	decoded, err := unmarshalSignerMaterialFromPersistence(encoded)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: [%v]", err)
	}

	if decoded.privateKeyShare != nil {
		t.Fatal("expected nil private key share for native signer material")
	}

	nativeSignerMaterial, ok := decoded.signerMaterial.(*frostsigning.NativeSignerMaterial)
	if !ok {
		t.Fatalf(
			"unexpected signer material type\nexpected: [%T]\nactual:   [%T]",
			&frostsigning.NativeSignerMaterial{},
			decoded.signerMaterial,
		)
	}

	if nativeSignerMaterial.Format != frostsigning.NativeSignerMaterialFormatFrostUniFFIV1 {
		t.Fatalf(
			"unexpected signer material format\nexpected: [%v]\nactual:   [%v]",
			frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
			nativeSignerMaterial.Format,
		)
	}
}

func TestUnmarshalSignerMaterialFromPersistence_CorruptedNativeEnvelope(t *testing.T) {
	encoded, err := encodeNativeSignerMaterialForPersistence(
		frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
		[]byte{0x10, 0x20},
	)
	if err != nil {
		t.Fatalf("unexpected encode error: [%v]", err)
	}

	encoded = encoded[:len(encoded)-1]

	_, err = unmarshalSignerMaterialFromPersistence(encoded)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}

	if !strings.Contains(err.Error(), "signer material payload length exceeds payload") {
		t.Fatalf(
			"unexpected unmarshal error\nexpected substring: [%s]\nactual:             [%v]",
			"signer material payload length exceeds payload",
			err,
		)
	}
}

func TestMarshalSignerMaterialForPersistence_UnsupportedType(t *testing.T) {
	_, err := marshalSignerMaterialForPersistence(struct{}{}, nil)
	if err == nil {
		t.Fatal("expected marshal error")
	}

	if !strings.Contains(err.Error(), "unsupported signer material type") {
		t.Fatalf(
			"unexpected marshal error\nexpected substring: [%s]\nactual:             [%v]",
			"unsupported signer material type",
			err,
		)
	}
}

func TestSignerMarshalling_NativeSignerMaterialRoundtrip(t *testing.T) {
	legacySigner := createMockSigner(t)
	marshaled := &signer{
		wallet:                  legacySigner.wallet,
		signingGroupMemberIndex: legacySigner.signingGroupMemberIndex,
		signerMaterial: &frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: []byte{0x44, 0x55, 0x66},
		},
	}
	unmarshaled := &signer{}

	if err := pbutils.RoundTrip(marshaled, unmarshaled); err != nil {
		t.Fatal(err)
	}

	if unmarshaled.privateKeyShare != nil {
		t.Fatal("expected nil private key share for native signer material")
	}

	if !reflect.DeepEqual(marshaled.wallet, unmarshaled.wallet) {
		t.Fatalf(
			"unexpected wallet state after roundtrip\nexpected: [%+v]\nactual:   [%+v]",
			marshaled.wallet,
			unmarshaled.wallet,
		)
	}

	if marshaled.signingGroupMemberIndex != unmarshaled.signingGroupMemberIndex {
		t.Fatalf(
			"unexpected signer member index\nexpected: [%v]\nactual:   [%v]",
			marshaled.signingGroupMemberIndex,
			unmarshaled.signingGroupMemberIndex,
		)
	}

	nativeSignerMaterial, ok := unmarshaled.signerMaterial.(*frostsigning.NativeSignerMaterial)
	if !ok {
		t.Fatalf(
			"unexpected signer material type\nexpected: [%T]\nactual:   [%T]",
			&frostsigning.NativeSignerMaterial{},
			unmarshaled.signerMaterial,
		)
	}

	if nativeSignerMaterial.Format != frostsigning.NativeSignerMaterialFormatFrostUniFFIV1 {
		t.Fatalf(
			"unexpected signer material format\nexpected: [%v]\nactual:   [%v]",
			frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
			nativeSignerMaterial.Format,
		)
	}

	if !bytes.Equal(nativeSignerMaterial.Payload, []byte{0x44, 0x55, 0x66}) {
		t.Fatalf(
			"unexpected signer material payload\nexpected: [%x]\nactual:   [%x]",
			[]byte{0x44, 0x55, 0x66},
			nativeSignerMaterial.Payload,
		)
	}
}

func TestSignerMarshalling_LegacyEncodingDoesNotUseNativeEnvelope(t *testing.T) {
	signer := createMockSigner(t)

	encodedSigner, err := signer.Marshal()
	if err != nil {
		t.Fatalf("unexpected marshal error: [%v]", err)
	}

	pbSigner := &pb.Signer{}
	if err := proto.Unmarshal(encodedSigner, pbSigner); err != nil {
		t.Fatalf("unexpected proto unmarshal error: [%v]", err)
	}

	if bytes.HasPrefix(pbSigner.PrivateKeyShare, signerMaterialEnvelopePrefix) {
		t.Fatal("expected legacy signer encoding without native envelope")
	}
}

func TestFuzzDecodeNativeSignerMaterialFromPersistence(t *testing.T) {
	for i := 0; i < 10; i++ {
		var data []byte
		fuzz.New().NilChance(0.1).NumElements(0, 256).Fuzz(&data)

		_, _, _ = decodeNativeSignerMaterialFromPersistence(data)
	}
}
