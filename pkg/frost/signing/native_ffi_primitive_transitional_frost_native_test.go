//go:build frost_native

package signing

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_ValidatesRequest(
	t *testing.T,
) {
	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	_, err := primitive.Sign(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if err.Error() != "request is nil" {
		t.Fatalf(
			"unexpected error\nexpected: [%s]\nactual:   [%v]",
			"request is nil",
			err,
		)
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_ValidatesMessage(
	t *testing.T,
) {
	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	_, err := primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: []byte{0x01},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if err.Error() != "request message is nil" {
		t.Fatalf(
			"unexpected error\nexpected: [%s]\nactual:   [%v]",
			"request message is nil",
			err,
		)
	}
}

func TestDecodeBuildTaggedLegacyPrivateKeyShare(t *testing.T) {
	fixtures, err := tecdsatest.LoadPrivateKeyShareTestFixtures(5)
	if err != nil {
		t.Fatalf("failed loading key share fixtures: [%v]", err)
	}

	expectedPrivateKeyShare := tecdsa.NewPrivateKeyShare(fixtures[0])
	expectedPayload, err := expectedPrivateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("failed marshaling private key share: [%v]", err)
	}

	decodedPrivateKeyShare, err := decodeBuildTaggedLegacyPrivateKeyShare(
		&NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV1,
			Payload: expectedPayload,
		},
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}

	actualPayload, err := decodedPrivateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("failed marshaling decoded private key share: [%v]", err)
	}

	if !bytes.Equal(expectedPayload, actualPayload) {
		t.Fatalf(
			"unexpected decoded private key share\nexpected: [%x]\nactual:   [%x]",
			expectedPayload,
			actualPayload,
		)
	}
}

func TestDecodeBuildTaggedLegacyPrivateKeyShare_RejectsInvalidMaterial(
	t *testing.T,
) {
	testCases := []struct {
		name           string
		signerMaterial *NativeSignerMaterial
	}{
		{
			name:           "nil signer material",
			signerMaterial: nil,
		},
		{
			name: "unsupported format",
			signerMaterial: &NativeSignerMaterial{
				Format:  "other",
				Payload: []byte{0x01},
			},
		},
		{
			name: "empty payload",
			signerMaterial: &NativeSignerMaterial{
				Format: NativeSignerMaterialFormatFrostUniFFIV1,
			},
		},
		{
			name: "invalid payload",
			signerMaterial: &NativeSignerMaterial{
				Format:  NativeSignerMaterialFormatFrostUniFFIV1,
				Payload: big.NewInt(123).Bytes(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeBuildTaggedLegacyPrivateKeyShare(tc.signerMaterial)
			if err == nil {
				t.Fatal("expected error")
			}

			if !errors.Is(err, ErrNativeCryptographyUnavailable) {
				t.Fatalf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]",
					ErrNativeCryptographyUnavailable,
					err,
				)
			}
		})
	}
}
