//go:build frost_native

package signing

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func newTestRequestWithUnsupportedUniFFIV2Material(t *testing.T, attemptNumber uint) *NativeExecutionFFISigningRequest {
	t.Helper()
	const hexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	payload, _ := json.Marshal(&nativeFROSTUniFFIV2SignerMaterial{
		KeyPackage: &NativeFROSTKeyPackage{
			Identifier: "id-1",
			Data:       []byte{0x01},
		},
		PublicKeyPackage: &NativeFROSTPublicKeyPackage{
			VerifyingKey: hexKey,
		},
	})
	return &NativeExecutionFFISigningRequest{
		Message:     new(big.Int).SetBytes([]byte{0xab, 0xcd}),
		SessionID:   "session-test",
		MemberIndex: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostUniFFIV2,
			Payload: payload,
		},
		Attempt: &Attempt{
			Number:                 attemptNumber,
			CoordinatorMemberIndex: 1,
			IncludedMembersIndexes: []group.MemberIndex{1, 2, 3, 4, 5},
			ExcludedMembersIndexes: nil,
		},
	}
}

func newTestRequestWithTBTCSignerV1Material(t *testing.T, attemptNumber uint) *NativeExecutionFFISigningRequest {
	t.Helper()
	payload, _ := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup: "tbtc-group-A",
	})
	return &NativeExecutionFFISigningRequest{
		Message:     new(big.Int).SetBytes([]byte{0xab, 0xcd}),
		SessionID:   "session-test",
		MemberIndex: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: payload,
		},
		Attempt: &Attempt{
			Number:                 attemptNumber,
			CoordinatorMemberIndex: 1,
			IncludedMembersIndexes: []group.MemberIndex{1, 2, 3},
			ExcludedMembersIndexes: nil,
		},
	}
}

func TestBuildAttemptContextFromRequest_TBTCSignerV1_KeyGroupIDIsRawIdentifier(t *testing.T) {
	req := newTestRequestWithTBTCSignerV1Material(t, 1)
	ctx, err := BuildAttemptContextFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.KeyGroupID != "tbtc-group-A" {
		t.Fatalf(
			"key group id: got %q, want %q",
			ctx.KeyGroupID,
			"tbtc-group-A",
		)
	}
}

func TestBuildAttemptContextFromRequest_UnsupportedUniFFIV2Rejected(t *testing.T) {
	req := newTestRequestWithUnsupportedUniFFIV2Material(t, 1)
	_, err := BuildAttemptContextFromRequest(req)
	if !errors.Is(err, ErrUnsupportedSignerMaterialFormat) {
		t.Fatalf("expected ErrUnsupportedSignerMaterialFormat, got %v", err)
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error should mention unsupported format; got %v", err)
	}
}

func TestBuildAttemptContextFromRequest_RejectsNilRequest(t *testing.T) {
	_, err := BuildAttemptContextFromRequest(nil)
	if !errors.Is(err, ErrAttemptContextConstruction) {
		t.Fatalf("expected ErrAttemptContextConstruction, got %v", err)
	}
}

func TestBuildAttemptContextFromRequest_RejectsNilMessage(t *testing.T) {
	req := newTestRequestWithTBTCSignerV1Material(t, 1)
	req.Message = nil
	_, err := BuildAttemptContextFromRequest(req)
	if err == nil {
		t.Fatal("expected error for nil message")
	}
	if !strings.Contains(err.Error(), "message is nil") {
		t.Fatalf("error must mention nil message; got %v", err)
	}
}

func TestBuildAttemptContextFromRequest_RejectsNilSignerMaterial(t *testing.T) {
	req := newTestRequestWithTBTCSignerV1Material(t, 1)
	req.SignerMaterial = nil
	_, err := BuildAttemptContextFromRequest(req)
	if err == nil {
		t.Fatal("expected error for nil signer material")
	}
	if !strings.Contains(err.Error(), "signer material is nil") {
		t.Fatalf("error must mention nil signer material; got %v", err)
	}
}

func TestBuildAttemptContextFromRequest_RejectsNilAttempt(t *testing.T) {
	req := newTestRequestWithTBTCSignerV1Material(t, 1)
	req.Attempt = nil
	_, err := BuildAttemptContextFromRequest(req)
	if err == nil {
		t.Fatal("expected error for nil attempt metadata")
	}
}

func TestBuildAttemptContextFromRequest_RejectsZeroAttemptNumber(t *testing.T) {
	req := newTestRequestWithTBTCSignerV1Material(t, 0)
	_, err := BuildAttemptContextFromRequest(req)
	if err == nil {
		t.Fatal("expected error for zero attempt number")
	}
	if !strings.Contains(err.Error(), "Attempt.Number is zero") {
		t.Fatalf("error must mention zero attempt; got %v", err)
	}
}

func TestBuildAttemptContextFromRequest_PropagatesExtractionErrors(t *testing.T) {
	req := newTestRequestWithTBTCSignerV1Material(t, 1)
	req.SignerMaterial = &NativeSignerMaterial{
		Format:  NativeSignerMaterialFormatFrostUniFFIV1,
		Payload: []byte("{}"),
	}
	_, err := BuildAttemptContextFromRequest(req)
	if !errors.Is(err, ErrUnsupportedSignerMaterialFormat) {
		t.Fatalf("expected ErrUnsupportedSignerMaterialFormat, got %v", err)
	}
	if !errors.Is(err, ErrAttemptContextConstruction) {
		t.Fatalf("expected ErrAttemptContextConstruction wrapper, got %v", err)
	}
}

func TestBuildAttemptContextFromRequest_AttemptNumberIsZeroBased(t *testing.T) {
	cases := []struct {
		legacyNumber      uint
		expectedZeroBased uint32
	}{
		{1, 0},
		{2, 1},
		{5, 4},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("legacy=%d", tc.legacyNumber), func(t *testing.T) {
			req := newTestRequestWithTBTCSignerV1Material(t, tc.legacyNumber)
			ctx, err := BuildAttemptContextFromRequest(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ctx.AttemptNumber != tc.expectedZeroBased {
				t.Fatalf(
					"got attempt number %d, want %d (legacy 1-based input %d)",
					ctx.AttemptNumber, tc.expectedZeroBased, tc.legacyNumber,
				)
			}
		})
	}
}

func TestMessageDigestFromBigInt_PadsShortBigInts(t *testing.T) {
	bi := new(big.Int).SetBytes([]byte{0x01, 0x02})
	digest, err := messageDigestFromBigInt(bi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [attempt.MessageDigestLength]byte{}
	want[30] = 0x01
	want[31] = 0x02
	if digest != want {
		t.Fatalf("padding wrong: got %x, want %x", digest, want)
	}
}

func TestMessageDigestFromBigInt_RejectsLongBigInts(t *testing.T) {
	bi := new(big.Int).SetBytes(make([]byte, 33))
	bi.SetBit(bi, 264, 1) // 33-byte length
	_, err := messageDigestFromBigInt(bi)
	if err == nil {
		t.Fatal("expected error for over-long message")
	}
}

func TestBuildAttemptContextFromRequest_DeterministicAcrossInvocations(t *testing.T) {
	req := newTestRequestWithTBTCSignerV1Material(t, 1)
	a, err := BuildAttemptContextFromRequest(req)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	b, err := BuildAttemptContextFromRequest(req)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if a.Hash() != b.Hash() {
		t.Fatalf(
			"two calls with same request produced different hashes: %x vs %x",
			a.Hash(), b.Hash(),
		)
	}
}

func TestBuildAttemptContextFromRequest_HashChangesWhenMessageDigestChanges(t *testing.T) {
	req := newTestRequestWithTBTCSignerV1Material(t, 1)
	a, _ := BuildAttemptContextFromRequest(req)
	req.Message = new(big.Int).SetBytes([]byte{0x99, 0x88, 0x77})
	b, _ := BuildAttemptContextFromRequest(req)
	if a.Hash() == b.Hash() {
		t.Fatal("hash must change when message digest changes")
	}
}

func TestBuildAttemptContextFromRequest_HashChangesWhenIncludedSetChanges(t *testing.T) {
	req := newTestRequestWithTBTCSignerV1Material(t, 1)
	a, _ := BuildAttemptContextFromRequest(req)
	req.Attempt.IncludedMembersIndexes = []group.MemberIndex{1, 2, 4}
	b, _ := BuildAttemptContextFromRequest(req)
	if a.Hash() == b.Hash() {
		t.Fatal("hash must change when included set changes")
	}
}

// Sanity check that the message digest padding produces the same
// bytes as a direct SHA-256 (just a smoke test on the constants).
func TestMessageDigestFromBigInt_SmokeTestSha256Length(t *testing.T) {
	if attempt.MessageDigestLength != sha256.Size {
		t.Fatalf(
			"AttemptContext digest length %d != SHA-256 size %d",
			attempt.MessageDigestLength, sha256.Size,
		)
	}
}
