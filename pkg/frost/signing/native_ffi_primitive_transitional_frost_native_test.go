//go:build frost_native

package signing

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

type mockBuildTaggedTBTCSignerEngine struct {
	startCalled bool
	sessionID   string
	message     []byte
	keyGroup    string
}

func (mbttse *mockBuildTaggedTBTCSignerEngine) StartSignRound(
	sessionID string,
	message []byte,
	keyGroup string,
) (*NativeTBTCSignerRoundState, error) {
	mbttse.startCalled = true
	mbttse.sessionID = sessionID
	mbttse.message = append([]byte{}, message...)
	mbttse.keyGroup = keyGroup

	return &NativeTBTCSignerRoundState{
		SessionID:             sessionID,
		RoundID:               "round-1",
		RequiredContributions: 2,
		MessageDigestHex:      "00",
	}, nil
}

func (mbttse *mockBuildTaggedTBTCSignerEngine) FinalizeSignRound(
	sessionID string,
	roundContributions []NativeTBTCSignerRoundContribution,
) ([]byte, error) {
	return nil, fmt.Errorf("not used")
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

func TestDecodeBuildTaggedTBTCSignerKeyGroup(t *testing.T) {
	keyGroup, err := decodeBuildTaggedTBTCSignerKeyGroup(&NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: []byte(`{"keyGroup":"group-1"}`),
	})
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}

	if keyGroup != "group-1" {
		t.Fatalf(
			"unexpected key group\nexpected: [%v]\nactual:   [%v]",
			"group-1",
			keyGroup,
		)
	}
}

func TestDecodeBuildTaggedTBTCSignerKeyGroup_RejectsInvalidMaterial(
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
				Payload: []byte(`{"keyGroup":"group-1"}`),
			},
		},
		{
			name: "empty payload",
			signerMaterial: &NativeSignerMaterial{
				Format: NativeSignerMaterialFormatFrostTBTCSignerV1,
			},
		},
		{
			name: "invalid payload",
			signerMaterial: &NativeSignerMaterial{
				Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
				Payload: []byte(`{"keyGroup":`),
			},
		},
		{
			name: "empty key group",
			signerMaterial: &NativeSignerMaterial{
				Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
				Payload: []byte(`{"keyGroup":""}`),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeBuildTaggedTBTCSignerKeyGroup(tc.signerMaterial)
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

func TestDecodeBuildTaggedTBTCSignerLegacyPrivateKeyShare(t *testing.T) {
	fixtures, err := tecdsatest.LoadPrivateKeyShareTestFixtures(5)
	if err != nil {
		t.Fatalf("failed loading key share fixtures: [%v]", err)
	}

	expectedPrivateKeyShare := tecdsa.NewPrivateKeyShare(fixtures[0])
	expectedPayload, err := expectedPrivateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("failed marshaling private key share: [%v]", err)
	}

	decodedPrivateKeyShare, err := decodeBuildTaggedTBTCSignerLegacyPrivateKeyShare(
		&buildTaggedTBTCSignerMaterialPayload{
			KeyGroup:                 "group-1",
			LegacyPrivateKeyShareHex: hex.EncodeToString(expectedPayload),
		},
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}

	if decodedPrivateKeyShare == nil {
		t.Fatal("expected decoded private key share")
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

func TestDecodeBuildTaggedTBTCSignerLegacyPrivateKeyShare_RejectsInvalidPayload(
	t *testing.T,
) {
	testCases := []struct {
		name        string
		payload     *buildTaggedTBTCSignerMaterialPayload
		expectError bool
	}{
		{
			name:        "nil payload",
			payload:     nil,
			expectError: false,
		},
		{
			name:        "empty legacy private key share",
			payload:     &buildTaggedTBTCSignerMaterialPayload{},
			expectError: false,
		},
		{
			name: "invalid hex",
			payload: &buildTaggedTBTCSignerMaterialPayload{
				LegacyPrivateKeyShareHex: "zz",
			},
			expectError: true,
		},
		{
			name: "invalid private key share payload",
			payload: &buildTaggedTBTCSignerMaterialPayload{
				LegacyPrivateKeyShareHex: hex.EncodeToString(big.NewInt(123).Bytes()),
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := decodeBuildTaggedTBTCSignerLegacyPrivateKeyShare(tc.payload)

			if tc.expectError {
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

				return
			}

			if err != nil {
				t.Fatalf("expected nil error, got: [%v]", err)
			}

			if decoded != nil {
				t.Fatalf("expected nil decoded private key share, got: [%v]", decoded)
			}
		})
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath(
	t *testing.T,
) {
	engine := &mockBuildTaggedTBTCSignerEngine{}
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)

	err := RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	_, err = primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message:   big.NewInt(123),
		SessionID: "session-1",
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(`{"keyGroup":"group-1"}`),
		},
	})
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

	if !engine.startCalled {
		t.Fatal("expected StartSignRound call")
	}

	if engine.sessionID != "session-1" {
		t.Fatalf(
			"unexpected session ID\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			engine.sessionID,
		)
	}

	if engine.keyGroup != "group-1" {
		t.Fatalf(
			"unexpected key group\nexpected: [%v]\nactual:   [%v]",
			"group-1",
			engine.keyGroup,
		)
	}
}
