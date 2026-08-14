//go:build frost_native

package signing

import (
	"bytes"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestDecodeBuildTaggedTBTCSignerSignatureRejectsNonCanonicalBIP340(
	t *testing.T,
) {
	_, err := decodeBuildTaggedTBTCSignerSignature(bytes.Repeat([]byte{0xff}, 64))
	if err == nil {
		t.Fatal("expected non-canonical BIP-340 signature bytes to be rejected")
	}
	if !strings.Contains(err.Error(), "non-canonical BIP-340 signature bytes") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

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

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_RejectsUnsupportedUniFFIV2Material(
	t *testing.T,
) {
	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	_, err := primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message: big.NewInt(123),
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV2,
			Payload: []byte{0x01},
		},
	})
	if err == nil {
		t.Fatal("expected unsupported material error")
	}
	if !errors.Is(err, ErrUnsupportedSignerMaterialFormat) {
		t.Fatalf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			ErrUnsupportedSignerMaterialFormat,
			err,
		)
	}
	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unsupported signer material should not be reported as unavailable native cryptography: [%v]",
			err,
		)
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_RefusesTBTCSignerV1MaterialTerminally(
	t *testing.T,
) {
	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	_, err := primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message: big.NewInt(123),
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(`{"keyGroup":"kg"}`),
		},
	})
	if err == nil {
		t.Fatal("expected refusal of frost-tbtc-signer-v1 material")
	}
	// Coarse-FROST signing was removed: this inner primitive must refuse
	// frost-tbtc-signer-v1 material (it is signed via the interactive path), and
	// the refusal must be TERMINAL so the tBTC signing retry loop aborts instead
	// of retrying a deterministic configuration failure until timeout.
	if !errors.Is(err, ErrTerminalSigningFailure) {
		t.Fatalf(
			"expected ErrTerminalSigningFailure\nactual: [%v]",
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

func TestIsBuildTaggedTBTCSignerBootstrapVersion(t *testing.T) {
	testCases := []struct {
		name     string
		version  string
		expected bool
	}{
		{
			name:     "valid exact bootstrap",
			version:  "tbtc-signer/0.1.0-bootstrap",
			expected: true,
		},
		{
			name:     "valid bootstrap dotted suffix",
			version:  "tbtc-signer/0.1.0-bootstrap.1",
			expected: true,
		},
		{
			name:     "invalid non-bootstrap prerelease",
			version:  "tbtc-signer/0.1.0-post-bootstrap",
			expected: false,
		},
		{
			name:     "invalid major version one",
			version:  "tbtc-signer/1.0.0-bootstrap",
			expected: false,
		},
		{
			name:     "invalid missing prerelease",
			version:  "tbtc-signer/0.1.0",
			expected: false,
		},
		{
			name:     "invalid malformed core semver",
			version:  "tbtc-signer/0.1-bootstrap",
			expected: false,
		},
		{
			name:     "invalid prefix",
			version:  "other/0.1.0-bootstrap",
			expected: false,
		},
		{
			name:     "invalid uppercase bootstrap token",
			version:  "tbtc-signer/0.1.0-Bootstrap",
			expected: false,
		},
		{
			name:     "invalid substring trap",
			version:  "tbtc-signer/0.1.0-post-bootstrap-cleanup",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isBuildTaggedTBTCSignerBootstrapVersion(tc.version)
			if actual != tc.expected {
				t.Fatalf(
					"unexpected bootstrap version classification\nversion:  [%s]\nexpected: [%v]\nactual:   [%v]",
					tc.version,
					tc.expected,
					actual,
				)
			}
		})
	}
}

func TestBuildTaggedTBTCSignerErrorPayload(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		code    string
		// Substring expected in the rendered Message. Empty means we don't
		// assert beyond Code presence.
		messageSubstring string
	}{
		{
			name:             "decodes structured envelope",
			payload:          []byte(`{"code":"consumed_attempt_replay","message":"attempt_id [a] already consumed"}`),
			code:             "consumed_attempt_replay",
			messageSubstring: "already consumed",
		},
		{
			name:             "legacy validation_error code is preserved",
			payload:          []byte(`{"code":"validation_error","message":"session_id is empty"}`),
			code:             "validation_error",
			messageSubstring: "session_id is empty",
		},
		{
			name:             "message-only payload leaves Code empty",
			payload:          []byte(`{"message":"opaque message"}`),
			code:             "",
			messageSubstring: "opaque message",
		},
		{
			name:             "completely empty envelope surfaces the raw payload",
			payload:          []byte(`{}`),
			code:             "",
			messageSubstring: "empty error payload",
		},
		{
			name:             "non-JSON payload is reported as a decode failure",
			payload:          []byte(`not json`),
			code:             "",
			messageSubstring: "cannot decode error payload",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			structured := buildTaggedTBTCSignerErrorPayload(tc.payload)
			if structured == nil {
				t.Fatal("expected non-nil structured error")
			}
			if structured.Code != tc.code {
				t.Fatalf(
					"unexpected Code\nexpected: [%s]\nactual:   [%s]",
					tc.code,
					structured.Code,
				)
			}
			if tc.messageSubstring != "" &&
				!strings.Contains(structured.Message, tc.messageSubstring) {
				t.Fatalf(
					"Message missing expected substring\nexpected substring: [%s]\nactual:             [%s]",
					tc.messageSubstring,
					structured.Message,
				)
			}
		})
	}
}
