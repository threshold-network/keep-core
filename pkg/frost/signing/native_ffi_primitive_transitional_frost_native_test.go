//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/internal/tecdsatest"
	"github.com/keep-network/keep-core/pkg/net/local"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

type mockBuildTaggedTBTCSignerEngine struct {
	runDKGCalled       bool
	runDKGSessionID    string
	runDKGParticipants []NativeTBTCSignerDKGParticipant
	runDKGThreshold    uint16
	runDKGResult       *NativeTBTCSignerDKGResult
	runDKGErr          error
	runDKGFn           func(
		sessionID string,
		participants []NativeTBTCSignerDKGParticipant,
		threshold uint16,
	) (*NativeTBTCSignerDKGResult, error)
	version                  string
	versionErr               error
	startCalled              bool
	startSessionID           string
	startMemberID            uint16
	startMessage             []byte
	startKeyGroup            string
	startSigningParticipants []uint16
	startSignRoundFn         func(
		sessionID string,
		memberIdentifier uint16,
		message []byte,
		keyGroup string,
		signingParticipants []uint16,
		taprootMerkleRoot *[32]byte,
	) (*NativeTBTCSignerRoundState, error)
	startTaprootMerkleRoot    *[32]byte
	startRoundState           *NativeTBTCSignerRoundState
	startErr                  error
	finalizeCalled            bool
	finalizeSessionID         string
	finalizeTaprootMerkleRoot *[32]byte
	finalizeInputs            []NativeTBTCSignerRoundContribution
	finalizeSignature         []byte
	finalizeErr               error
}

func (mbttse *mockBuildTaggedTBTCSignerEngine) RunDKG(
	sessionID string,
	participants []NativeTBTCSignerDKGParticipant,
	threshold uint16,
) (*NativeTBTCSignerDKGResult, error) {
	mbttse.runDKGCalled = true
	mbttse.runDKGSessionID = sessionID
	mbttse.runDKGParticipants = append(
		[]NativeTBTCSignerDKGParticipant{},
		participants...,
	)
	mbttse.runDKGThreshold = threshold

	if mbttse.runDKGErr != nil {
		return nil, mbttse.runDKGErr
	}

	if mbttse.runDKGFn != nil {
		return mbttse.runDKGFn(sessionID, participants, threshold)
	}

	if mbttse.runDKGResult != nil {
		return mbttse.runDKGResult, nil
	}

	return &NativeTBTCSignerDKGResult{
		SessionID:        sessionID,
		KeyGroup:         "group-1",
		ParticipantCount: uint16(len(participants)),
		Threshold:        threshold,
		CreatedAtUnix:    1,
	}, nil
}

func (mbttse *mockBuildTaggedTBTCSignerEngine) Version() (string, error) {
	if mbttse.versionErr != nil {
		return "", mbttse.versionErr
	}

	return mbttse.version, nil
}

func (mbttse *mockBuildTaggedTBTCSignerEngine) StartSignRound(
	sessionID string,
	memberIdentifier uint16,
	message []byte,
	keyGroup string,
	signingParticipants []uint16,
	taprootMerkleRoot *[32]byte,
) (*NativeTBTCSignerRoundState, error) {
	mbttse.startCalled = true
	mbttse.startSessionID = sessionID
	mbttse.startMemberID = memberIdentifier
	mbttse.startMessage = append([]byte{}, message...)
	mbttse.startKeyGroup = keyGroup
	mbttse.startSigningParticipants = append([]uint16{}, signingParticipants...)
	mbttse.startTaprootMerkleRoot = cloneTestTaprootMerkleRoot(taprootMerkleRoot)

	if mbttse.startErr != nil {
		return nil, mbttse.startErr
	}

	if mbttse.startSignRoundFn != nil {
		return mbttse.startSignRoundFn(
			sessionID,
			memberIdentifier,
			message,
			keyGroup,
			signingParticipants,
			taprootMerkleRoot,
		)
	}

	if mbttse.startRoundState != nil {
		if len(mbttse.startRoundState.SigningParticipants) == 0 {
			mbttse.startRoundState.SigningParticipants = append(
				[]uint16{},
				signingParticipants...,
			)
		}

		return mbttse.startRoundState, nil
	}

	return &NativeTBTCSignerRoundState{
		SessionID:             sessionID,
		RoundID:               "round-1",
		RequiredContributions: 2,
		MessageDigestHex:      "00",
		SigningParticipants:   append([]uint16{}, signingParticipants...),
	}, nil
}

func (mbttse *mockBuildTaggedTBTCSignerEngine) FinalizeSignRound(
	sessionID string,
	roundContributions []NativeTBTCSignerRoundContribution,
	taprootMerkleRoot *[32]byte,
) ([]byte, error) {
	mbttse.finalizeCalled = true
	mbttse.finalizeSessionID = sessionID
	mbttse.finalizeTaprootMerkleRoot = cloneTestTaprootMerkleRoot(taprootMerkleRoot)
	mbttse.finalizeInputs = append(
		[]NativeTBTCSignerRoundContribution{},
		roundContributions...,
	)

	if mbttse.finalizeErr != nil {
		return nil, mbttse.finalizeErr
	}

	if len(mbttse.finalizeSignature) > 0 {
		return append([]byte{}, mbttse.finalizeSignature...), nil
	}

	return []byte{0xaa}, nil
}

func (mbttse *mockBuildTaggedTBTCSignerEngine) BuildTaprootTx(
	sessionID string,
	inputs []NativeTBTCSignerTxInput,
	outputs []NativeTBTCSignerTxOutput,
	scriptTreeHex *string,
) (*NativeTBTCSignerTxResult, error) {
	return nil, errors.New("not implemented")
}

func cloneTestTaprootMerkleRoot(taprootMerkleRoot *[32]byte) *[32]byte {
	if taprootMerkleRoot == nil {
		return nil
	}

	result := new([32]byte)
	copy(result[:], taprootMerkleRoot[:])

	return result
}

type deterministicBuildTaggedTBTCSignerBootstrapRoundEngine struct {
	roundState    *NativeTBTCSignerRoundState
	finalizeMutex sync.Mutex
	finalizeCalls int
	finalizeInput []NativeTBTCSignerRoundContribution
}

func (dbttsbre *deterministicBuildTaggedTBTCSignerBootstrapRoundEngine) RunDKG(
	sessionID string,
	participants []NativeTBTCSignerDKGParticipant,
	threshold uint16,
) (*NativeTBTCSignerDKGResult, error) {
	return &NativeTBTCSignerDKGResult{
		SessionID:        sessionID,
		KeyGroup:         "group-1",
		ParticipantCount: uint16(len(participants)),
		Threshold:        threshold,
		CreatedAtUnix:    1,
	}, nil
}

func (dbttsbre *deterministicBuildTaggedTBTCSignerBootstrapRoundEngine) StartSignRound(
	sessionID string,
	memberIdentifier uint16,
	_ []byte,
	_ string,
	signingParticipants []uint16,
	_ *[32]byte,
) (*NativeTBTCSignerRoundState, error) {
	if dbttsbre.roundState != nil {
		if dbttsbre.roundState.OwnContribution == nil {
			dbttsbre.roundState.OwnContribution = &NativeTBTCSignerRoundContribution{
				Identifier: memberIdentifier,
				Data:       []byte{byte(memberIdentifier), 0xab},
			}
		}

		if len(dbttsbre.roundState.SigningParticipants) == 0 {
			dbttsbre.roundState.SigningParticipants = append(
				[]uint16{},
				signingParticipants...,
			)
		}

		return dbttsbre.roundState, nil
	}

	if len(signingParticipants) == 0 {
		signingParticipants = []uint16{memberIdentifier}
	}

	return &NativeTBTCSignerRoundState{
		SessionID:             sessionID,
		RoundID:               "round-1",
		RequiredContributions: 2,
		MessageDigestHex:      "00",
		SigningParticipants:   append([]uint16{}, signingParticipants...),
		OwnContribution: &NativeTBTCSignerRoundContribution{
			Identifier: memberIdentifier,
			Data:       []byte{byte(memberIdentifier), 0xab},
		},
	}, nil
}

func (dbttsbre *deterministicBuildTaggedTBTCSignerBootstrapRoundEngine) FinalizeSignRound(
	_ string,
	roundContributions []NativeTBTCSignerRoundContribution,
	_ *[32]byte,
) ([]byte, error) {
	dbttsbre.finalizeMutex.Lock()
	defer dbttsbre.finalizeMutex.Unlock()

	dbttsbre.finalizeCalls++
	dbttsbre.finalizeInput = append(
		[]NativeTBTCSignerRoundContribution{},
		roundContributions...,
	)

	return []byte{0xaa}, nil
}

func (dbttsbre *deterministicBuildTaggedTBTCSignerBootstrapRoundEngine) BuildTaprootTx(
	sessionID string,
	inputs []NativeTBTCSignerTxInput,
	outputs []NativeTBTCSignerTxOutput,
	scriptTreeHex *string,
) (*NativeTBTCSignerTxResult, error) {
	return nil, errors.New("not implemented")
}

func (dbttsbre *deterministicBuildTaggedTBTCSignerBootstrapRoundEngine) finalizeInputs() []NativeTBTCSignerRoundContribution {
	dbttsbre.finalizeMutex.Lock()
	defer dbttsbre.finalizeMutex.Unlock()

	return append([]NativeTBTCSignerRoundContribution{}, dbttsbre.finalizeInput...)
}

func buildTaggedTBTCSignerValidTestSignature(seed byte) []byte {
	signature := make([]byte, 64)
	for i := range signature {
		signature[i] = seed + byte(i)
	}

	return signature
}

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
		&NativeTBTCSignerMaterialPayload{
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
		payload     *NativeTBTCSignerMaterialPayload
		expectError bool
	}{
		{
			name:        "nil payload",
			payload:     nil,
			expectError: false,
		},
		{
			name:        "empty legacy private key share",
			payload:     &NativeTBTCSignerMaterialPayload{},
			expectError: false,
		},
		{
			name: "invalid hex",
			payload: &NativeTBTCSignerMaterialPayload{
				LegacyPrivateKeyShareHex: "zz",
			},
			expectError: true,
		},
		{
			name: "invalid private key share payload",
			payload: &NativeTBTCSignerMaterialPayload{
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

func TestBuildTaggedTBTCSignerRunDKGInputs(t *testing.T) {
	participants, threshold, err := buildTaggedTBTCSignerRunDKGInputs(
		&NativeExecutionFFISigningRequest{
			GroupSize:          5,
			DishonestThreshold: 2,
			Attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 1,
				IncludedMembersIndexes: []group.MemberIndex{1, 3, 5},
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected RunDKG inputs error: [%v]", err)
	}

	if threshold != 3 {
		t.Fatalf(
			"unexpected threshold\nexpected: [%v]\nactual:   [%v]",
			3,
			threshold,
		)
	}

	if len(participants) != 3 {
		t.Fatalf(
			"unexpected participants count\nexpected: [%v]\nactual:   [%v]",
			3,
			len(participants),
		)
	}

	expectedIdentifiers := []uint16{1, 3, 5}
	expectedPublicKeys := []string{"020001", "020003", "020005"}

	for i := range participants {
		if participants[i].Identifier != expectedIdentifiers[i] {
			t.Fatalf(
				"unexpected participant identifier at index [%d]\nexpected: [%v]\nactual:   [%v]",
				i,
				expectedIdentifiers[i],
				participants[i].Identifier,
			)
		}

		if participants[i].PublicKeyHex != expectedPublicKeys[i] {
			t.Fatalf(
				"unexpected participant public key at index [%d]\nexpected: [%v]\nactual:   [%v]",
				i,
				expectedPublicKeys[i],
				participants[i].PublicKeyHex,
			)
		}
	}
}

func TestBuildTaggedTBTCSignerRunDKGInputsForPayload_UsesPersistedDKGInputs(
	t *testing.T,
) {
	persistedParticipants := []NativeTBTCSignerDKGParticipant{
		{Identifier: 1, PublicKeyHex: "020001"},
		{Identifier: 2, PublicKeyHex: "020002"},
		{Identifier: 3, PublicKeyHex: "020003"},
	}

	participants, threshold, err := buildTaggedTBTCSignerRunDKGInputsForPayload(
		&NativeTBTCSignerMaterialPayload{
			KeyGroupSource:  NativeTBTCSignerKeyGroupSourceDKGPersisted,
			DKGParticipants: persistedParticipants,
			DKGThreshold:    2,
		},
		&NativeExecutionFFISigningRequest{
			GroupSize:          3,
			DishonestThreshold: 1,
			Attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 1,
				IncludedMembersIndexes: []group.MemberIndex{1, 3},
			},
		},
		[]group.MemberIndex{1, 3},
	)
	if err != nil {
		t.Fatalf("unexpected RunDKG inputs error: [%v]", err)
	}

	if threshold != 2 {
		t.Fatalf("unexpected threshold: [%v]", threshold)
	}
	if len(participants) != len(persistedParticipants) {
		t.Fatalf(
			"unexpected participants count\nexpected: [%v]\nactual:   [%v]",
			len(persistedParticipants),
			len(participants),
		)
	}
	for i := range participants {
		if participants[i] != persistedParticipants[i] {
			t.Fatalf(
				"unexpected participant at index [%d]\nexpected: [%+v]\nactual:   [%+v]",
				i,
				persistedParticipants[i],
				participants[i],
			)
		}
	}
}

func TestBuildTaggedTBTCSignerRunDKGInputs_RejectsInvalidRequest(t *testing.T) {
	testCases := []struct {
		name    string
		request *NativeExecutionFFISigningRequest
	}{
		{
			name: "zero group size",
			request: &NativeExecutionFFISigningRequest{
				GroupSize:          0,
				DishonestThreshold: 1,
			},
		},
		{
			name: "derived threshold exceeds participants",
			request: &NativeExecutionFFISigningRequest{
				GroupSize:          2,
				DishonestThreshold: 2,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := buildTaggedTBTCSignerRunDKGInputs(tc.request)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestIncludedMembersFromRequest_RejectsInvalidAttemptPolicy(t *testing.T) {
	testCases := []struct {
		name        string
		request     *NativeExecutionFFISigningRequest
		errFragment string
	}{
		{
			name: "zero attempt number",
			request: &NativeExecutionFFISigningRequest{
				GroupSize: 3,
				Attempt: &Attempt{
					Number:                 0,
					CoordinatorMemberIndex: 1,
					IncludedMembersIndexes: []group.MemberIndex{1, 2},
				},
			},
			errFragment: "attempt number is zero",
		},
		{
			name: "zero coordinator",
			request: &NativeExecutionFFISigningRequest{
				GroupSize: 3,
				Attempt: &Attempt{
					Number:                 1,
					CoordinatorMemberIndex: 0,
					IncludedMembersIndexes: []group.MemberIndex{1, 2},
				},
			},
			errFragment: "attempt coordinator member index is zero",
		},
		{
			name: "coordinator not included",
			request: &NativeExecutionFFISigningRequest{
				GroupSize: 3,
				Attempt: &Attempt{
					Number:                 1,
					CoordinatorMemberIndex: 3,
					IncludedMembersIndexes: []group.MemberIndex{1, 2},
				},
			},
			errFragment: "attempt coordinator [3] is not included",
		},
		{
			name: "member both included and excluded",
			request: &NativeExecutionFFISigningRequest{
				GroupSize: 3,
				Attempt: &Attempt{
					Number:                 1,
					CoordinatorMemberIndex: 1,
					IncludedMembersIndexes: []group.MemberIndex{1, 2},
					ExcludedMembersIndexes: []group.MemberIndex{2},
				},
			},
			errFragment: "member [2] is both included and excluded in attempt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := includedMembersFromRequest(tc.request)
			if err == nil {
				t.Fatal("expected error")
			}

			if !strings.Contains(err.Error(), tc.errFragment) {
				t.Fatalf(
					"unexpected error\nexpected to contain: [%v]\nactual:              [%v]",
					tc.errFragment,
					err,
				)
			}
		})
	}
}

func TestBuildTaggedTBTCSignerSyntheticRoundContributions(t *testing.T) {
	roundState := &NativeTBTCSignerRoundState{
		SessionID:        "session-1",
		RoundID:          "round-1",
		MessageDigestHex: "aabbccdd",
	}

	contributionsFirst, err := buildTaggedTBTCSignerSyntheticRoundContributions(
		roundState,
		[]group.MemberIndex{1, 2, 3},
	)
	if err != nil {
		t.Fatalf("unexpected synthetic contribution error: [%v]", err)
	}

	contributionsSecond, err := buildTaggedTBTCSignerSyntheticRoundContributions(
		roundState,
		[]group.MemberIndex{1, 2, 3},
	)
	if err != nil {
		t.Fatalf("unexpected synthetic contribution error: [%v]", err)
	}

	if len(contributionsFirst) != 3 {
		t.Fatalf(
			"unexpected contribution count\nexpected: [%v]\nactual:   [%v]",
			3,
			len(contributionsFirst),
		)
	}

	expectedIdentifiers := []uint16{1, 2, 3}
	for i, contribution := range contributionsFirst {
		if contribution.Identifier != expectedIdentifiers[i] {
			t.Fatalf(
				"unexpected contribution identifier at index [%d]\nexpected: [%v]\nactual:   [%v]",
				i,
				expectedIdentifiers[i],
				contribution.Identifier,
			)
		}

		if len(contribution.Data) != 32 {
			t.Fatalf(
				"unexpected contribution size at index [%d]\nexpected: [%v]\nactual:   [%v]",
				i,
				32,
				len(contribution.Data),
			)
		}

		if !bytes.Equal(contribution.Data, contributionsSecond[i].Data) {
			t.Fatalf("expected deterministic contribution at index [%d]", i)
		}
	}

	roundStateChanged := &NativeTBTCSignerRoundState{
		SessionID:        "session-1",
		RoundID:          "round-2",
		MessageDigestHex: "aabbccdd",
	}
	contributionsChanged, err := buildTaggedTBTCSignerSyntheticRoundContributions(
		roundStateChanged,
		[]group.MemberIndex{1, 2, 3},
	)
	if err != nil {
		t.Fatalf("unexpected synthetic contribution error: [%v]", err)
	}

	if bytes.Equal(contributionsFirst[0].Data, contributionsChanged[0].Data) {
		t.Fatal("expected contribution data to change when round metadata changes")
	}
}

func TestBuildTaggedTBTCSignerSyntheticRoundContributions_RejectsInvalidInput(t *testing.T) {
	testCases := []struct {
		name       string
		roundState *NativeTBTCSignerRoundState
		members    []group.MemberIndex
	}{
		{
			name:       "nil round state",
			roundState: nil,
			members:    []group.MemberIndex{1, 2},
		},
		{
			name: "empty session id",
			roundState: &NativeTBTCSignerRoundState{
				SessionID:        "",
				RoundID:          "round-1",
				MessageDigestHex: "aa",
			},
			members: []group.MemberIndex{1, 2},
		},
		{
			name: "empty round id",
			roundState: &NativeTBTCSignerRoundState{
				SessionID:        "session-1",
				RoundID:          "",
				MessageDigestHex: "aa",
			},
			members: []group.MemberIndex{1, 2},
		},
		{
			name: "empty message digest",
			roundState: &NativeTBTCSignerRoundState{
				SessionID:        "session-1",
				RoundID:          "round-1",
				MessageDigestHex: "",
			},
			members: []group.MemberIndex{1, 2},
		},
		{
			name: "zero member index",
			roundState: &NativeTBTCSignerRoundState{
				SessionID:        "session-1",
				RoundID:          "round-1",
				MessageDigestHex: "aa",
			},
			members: []group.MemberIndex{0, 2},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTaggedTBTCSignerSyntheticRoundContributions(
				tc.roundState,
				tc.members,
			)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestExecuteBuildTaggedTBTCSignerBootstrapCoarseRound_ExchangesContributionsOverChannel(
	t *testing.T,
) {
	provider := local.Connect()
	channel, err := provider.BroadcastChannelFor("tbtc-signer-bootstrap-round-plumbing-test")
	if err != nil {
		t.Fatalf("failed creating broadcast channel: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}
	primitive.RegisterUnmarshallers(channel)

	engineByMember := map[group.MemberIndex]*deterministicBuildTaggedTBTCSignerBootstrapRoundEngine{
		1: &deterministicBuildTaggedTBTCSignerBootstrapRoundEngine{
			roundState: &NativeTBTCSignerRoundState{
				SessionID:             "session-1",
				RoundID:               "round-1",
				RequiredContributions: 2,
				MessageDigestHex:      "0011",
				OwnContribution: &NativeTBTCSignerRoundContribution{
					Identifier: 1,
					Data:       []byte{0x11, 0x01},
				},
			},
		},
		2: &deterministicBuildTaggedTBTCSignerBootstrapRoundEngine{
			roundState: &NativeTBTCSignerRoundState{
				SessionID:             "session-1",
				RoundID:               "round-1",
				RequiredContributions: 2,
				MessageDigestHex:      "0011",
				OwnContribution: &NativeTBTCSignerRoundContribution{
					Identifier: 2,
					Data:       []byte{0x22, 0x02},
				},
			},
		},
	}

	requestByMember := map[group.MemberIndex]*NativeExecutionFFISigningRequest{
		1: {
			Message:            big.NewInt(123),
			SessionID:          "session-1",
			MemberIndex:        1,
			GroupSize:          2,
			DishonestThreshold: 1,
			Channel:            channel,
			Attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 1,
				IncludedMembersIndexes: []group.MemberIndex{1, 2},
			},
		},
		2: {
			Message:            big.NewInt(123),
			SessionID:          "session-1",
			MemberIndex:        2,
			GroupSize:          2,
			DishonestThreshold: 1,
			Channel:            channel,
			Attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 1,
				IncludedMembersIndexes: []group.MemberIndex{1, 2},
			},
		},
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCtx()

	var wg sync.WaitGroup
	signingErrors := make(chan error, len(requestByMember))

	for memberIndex, request := range requestByMember {
		engine := engineByMember[memberIndex]
		wg.Add(1)

		go func(
			signingRequest *NativeExecutionFFISigningRequest,
			signingEngine NativeTBTCSignerEngine,
		) {
			defer wg.Done()

			signingErrors <- executeBuildTaggedTBTCSignerBootstrapCoarseRound(
				ctx,
				signingRequest,
				"group-1",
				signingEngine,
				nil,
				nil,
			)
		}(request, engine)
	}

	wg.Wait()
	close(signingErrors)

	for signingErr := range signingErrors {
		if signingErr != nil {
			t.Fatalf("unexpected signing error: [%v]", signingErr)
		}
	}

	for memberIndex, engine := range engineByMember {
		finalizeInputs := engine.finalizeInputs()
		if len(finalizeInputs) != 2 {
			t.Fatalf(
				"unexpected finalize input count for member [%v]\nexpected: [%v]\nactual:   [%v]",
				memberIndex,
				2,
				len(finalizeInputs),
			)
		}

		if finalizeInputs[0].Identifier != 1 || finalizeInputs[1].Identifier != 2 {
			t.Fatalf(
				"unexpected finalize identifiers for member [%v]\nexpected: [1 2]\nactual:   [%v %v]",
				memberIndex,
				finalizeInputs[0].Identifier,
				finalizeInputs[1].Identifier,
			)
		}

		if len(finalizeInputs[0].Data) == 0 || len(finalizeInputs[1].Data) == 0 {
			t.Fatalf("expected non-empty finalize contribution data for member [%v]", memberIndex)
		}

		if !bytes.Equal(finalizeInputs[0].Data, []byte{0x11, 0x01}) {
			t.Fatalf(
				"unexpected contribution data for identifier 1, member [%v]\nexpected: [%x]\nactual:   [%x]",
				memberIndex,
				[]byte{0x11, 0x01},
				finalizeInputs[0].Data,
			)
		}

		if !bytes.Equal(finalizeInputs[1].Data, []byte{0x22, 0x02}) {
			t.Fatalf(
				"unexpected contribution data for identifier 2, member [%v]\nexpected: [%x]\nactual:   [%x]",
				memberIndex,
				[]byte{0x22, 0x02},
				finalizeInputs[1].Data,
			)
		}
	}
}

func TestExecuteBuildTaggedTBTCSignerBootstrapCoarseRound_UsesThresholdCohortOverFullGroup(
	t *testing.T,
) {
	provider := local.Connect()
	channel, err := provider.BroadcastChannelFor("tbtc-signer-bootstrap-round-threshold-cohort-test")
	if err != nil {
		t.Fatalf("failed creating broadcast channel: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}
	primitive.RegisterUnmarshallers(channel)

	engineByMember := map[group.MemberIndex]*mockBuildTaggedTBTCSignerEngine{
		1: {
			startRoundState: &NativeTBTCSignerRoundState{
				SessionID:             "session-threshold",
				RoundID:               "round-threshold",
				RequiredContributions: 2,
				MessageDigestHex:      "0011",
				SigningParticipants:   []uint16{1, 3},
				OwnContribution: &NativeTBTCSignerRoundContribution{
					Identifier: 1,
					Data:       []byte{0x11, 0x01},
				},
			},
		},
		3: {
			startRoundState: &NativeTBTCSignerRoundState{
				SessionID:             "session-threshold",
				RoundID:               "round-threshold",
				RequiredContributions: 2,
				MessageDigestHex:      "0011",
				SigningParticipants:   []uint16{1, 3},
				OwnContribution: &NativeTBTCSignerRoundContribution{
					Identifier: 3,
					Data:       []byte{0x33, 0x03},
				},
			},
		},
	}

	requestByMember := map[group.MemberIndex]*NativeExecutionFFISigningRequest{
		1: {
			Message:            big.NewInt(123),
			SessionID:          "session-threshold",
			MemberIndex:        1,
			GroupSize:          3,
			DishonestThreshold: 1,
			Channel:            channel,
			Attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 1,
				IncludedMembersIndexes: []group.MemberIndex{1, 3},
			},
		},
		3: {
			Message:            big.NewInt(123),
			SessionID:          "session-threshold",
			MemberIndex:        3,
			GroupSize:          3,
			DishonestThreshold: 1,
			Channel:            channel,
			Attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 1,
				IncludedMembersIndexes: []group.MemberIndex{1, 3},
			},
		},
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelCtx()

	var wg sync.WaitGroup
	signingErrors := make(chan error, len(requestByMember))

	for memberIndex, request := range requestByMember {
		engine := engineByMember[memberIndex]
		wg.Add(1)

		go func(
			signingRequest *NativeExecutionFFISigningRequest,
			signingEngine NativeTBTCSignerEngine,
		) {
			defer wg.Done()

			signingErrors <- executeBuildTaggedTBTCSignerBootstrapCoarseRound(
				ctx,
				signingRequest,
				"group-1",
				signingEngine,
				nil,
				nil,
			)
		}(request, engine)
	}

	wg.Wait()
	close(signingErrors)

	for signingErr := range signingErrors {
		if signingErr != nil {
			t.Fatalf("unexpected signing error: [%v]", signingErr)
		}
	}

	expectedSigningParticipants := []uint16{1, 3}
	for memberIndex, engine := range engineByMember {
		if !reflect.DeepEqual(engine.startSigningParticipants, expectedSigningParticipants) {
			t.Fatalf(
				"unexpected StartSignRound signing participants for member [%v]\nexpected: [%v]\nactual:   [%v]",
				memberIndex,
				expectedSigningParticipants,
				engine.startSigningParticipants,
			)
		}

		if len(engine.finalizeInputs) != 2 {
			t.Fatalf(
				"unexpected finalize input count for member [%v]\nexpected: [%v]\nactual:   [%v]",
				memberIndex,
				2,
				len(engine.finalizeInputs),
			)
		}

		if engine.finalizeInputs[0].Identifier != 1 || engine.finalizeInputs[1].Identifier != 3 {
			t.Fatalf(
				"unexpected finalize identifiers for member [%v]\nexpected: [1 3]\nactual:   [%v %v]",
				memberIndex,
				engine.finalizeInputs[0].Identifier,
				engine.finalizeInputs[1].Identifier,
			)
		}
	}
}

func TestExecuteBuildTaggedTBTCSignerBootstrapCoarseRound_FailsWhenRoundStateSigningParticipantsMismatch(
	t *testing.T,
) {
	request := &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          2,
		DishonestThreshold: 1,
		Attempt: &Attempt{
			Number:                 1,
			CoordinatorMemberIndex: 1,
			IncludedMembersIndexes: []group.MemberIndex{1, 2},
		},
	}

	engine := &mockBuildTaggedTBTCSignerEngine{
		startRoundState: &NativeTBTCSignerRoundState{
			SessionID:             "session-1",
			RoundID:               "round-1",
			RequiredContributions: 2,
			MessageDigestHex:      "0011",
			SigningParticipants:   []uint16{1, 3},
			OwnContribution: &NativeTBTCSignerRoundContribution{
				Identifier: 1,
				Data:       []byte{0x11, 0x01},
			},
		},
	}

	err := executeBuildTaggedTBTCSignerBootstrapCoarseRound(
		context.Background(),
		request,
		"group-1",
		engine,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}

	expectedErrFragment := "start sign round returned unexpected signing participant"
	if !strings.Contains(err.Error(), expectedErrFragment) {
		t.Fatalf(
			"unexpected error\nexpected to contain: [%v]\nactual:              [%v]",
			expectedErrFragment,
			err,
		)
	}
}

func TestExecuteBuildTaggedTBTCSignerBootstrapCoarseRound_FailsWhenRoundStateSigningParticipantsMissing(
	t *testing.T,
) {
	request := &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          2,
		DishonestThreshold: 1,
		Attempt: &Attempt{
			Number:                 1,
			CoordinatorMemberIndex: 1,
			IncludedMembersIndexes: []group.MemberIndex{1, 2},
		},
	}

	engine := &mockBuildTaggedTBTCSignerEngine{
		startSignRoundFn: func(
			sessionID string,
			memberIdentifier uint16,
			message []byte,
			keyGroup string,
			signingParticipants []uint16,
			_ *[32]byte,
		) (*NativeTBTCSignerRoundState, error) {
			return &NativeTBTCSignerRoundState{
				SessionID:             sessionID,
				RoundID:               "round-1",
				RequiredContributions: 2,
				MessageDigestHex:      "0011",
				OwnContribution: &NativeTBTCSignerRoundContribution{
					Identifier: memberIdentifier,
					Data:       []byte{0x11, 0x01},
				},
			}, nil
		},
	}

	err := executeBuildTaggedTBTCSignerBootstrapCoarseRound(
		context.Background(),
		request,
		"group-1",
		engine,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected error")
	}

	expectedErrFragment := "start sign round returned unexpected signing participants count"
	if !strings.Contains(err.Error(), expectedErrFragment) {
		t.Fatalf(
			"unexpected error\nexpected to contain: [%v]\nactual:              [%v]",
			expectedErrFragment,
			err,
		)
	}
}

func TestBuildTaggedTBTCSignerRoundKeyGroup(t *testing.T) {
	testCases := []struct {
		name                 string
		payload              *NativeTBTCSignerMaterialPayload
		dkgResult            *NativeTBTCSignerDKGResult
		acceptScaffoldOptIn  bool
		expected             string
		substituted          bool
		expectError          bool
		expectScaffoldRefuse bool
	}{
		{
			name: "exact match",
			payload: &NativeTBTCSignerMaterialPayload{
				KeyGroup: "group-1",
			},
			dkgResult: &NativeTBTCSignerDKGResult{
				KeyGroup: "group-1",
			},
			expected:    "group-1",
			substituted: false,
		},
		{
			name: "legacy source mismatch refused without opt-in",
			payload: &NativeTBTCSignerMaterialPayload{
				KeyGroup:       "legacy-group",
				KeyGroupSource: NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
			},
			dkgResult: &NativeTBTCSignerDKGResult{
				KeyGroup: "dkg-group",
			},
			expectError:          true,
			expectScaffoldRefuse: true,
		},
		{
			name: "legacy source mismatch uses dkg key group with opt-in",
			payload: &NativeTBTCSignerMaterialPayload{
				KeyGroup:       "legacy-group",
				KeyGroupSource: NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
			},
			dkgResult: &NativeTBTCSignerDKGResult{
				KeyGroup: "dkg-group",
			},
			acceptScaffoldOptIn: true,
			expected:            "dkg-group",
			substituted:         true,
		},
		{
			name: "non-legacy source mismatch rejects",
			payload: &NativeTBTCSignerMaterialPayload{
				KeyGroup:       "legacy-group",
				KeyGroupSource: "dkg-persisted",
			},
			dkgResult: &NativeTBTCSignerDKGResult{
				KeyGroup: "dkg-group",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.acceptScaffoldOptIn {
				t.Setenv(AcceptScaffoldKeyGroupEnvVar, "true")
			} else {
				// Force the env to "" so a stray external value from a
				// containing process cannot suppress the scaffold refusal
				// during this test case.
				t.Setenv(AcceptScaffoldKeyGroupEnvVar, "")
			}

			actual, substituted, err := buildTaggedTBTCSignerRoundKeyGroup(tc.payload, tc.dkgResult)
			if tc.expectError {
				if err == nil {
					t.Fatal("expected error")
				}

				if tc.expectScaffoldRefuse &&
					!strings.Contains(err.Error(), AcceptScaffoldKeyGroupEnvVar) {
					t.Fatalf(
						"expected scaffold-refusal error referencing %s; got: [%v]",
						AcceptScaffoldKeyGroupEnvVar,
						err,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: [%v]", err)
			}

			if actual != tc.expected {
				t.Fatalf(
					"unexpected key group\nexpected: [%v]\nactual:   [%v]",
					tc.expected,
					actual,
				)
			}

			if substituted != tc.substituted {
				t.Fatalf(
					"unexpected substitution flag\nexpected: [%v]\nactual:   [%v]",
					tc.substituted,
					substituted,
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
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
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

	if !engine.runDKGCalled {
		t.Fatal("expected RunDKG call in tbtc-signer path")
	}

	if engine.runDKGSessionID != "session-1" {
		t.Fatalf(
			"unexpected RunDKG session ID\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			engine.runDKGSessionID,
		)
	}

	if engine.runDKGThreshold != 2 {
		t.Fatalf(
			"unexpected RunDKG threshold\nexpected: [%v]\nactual:   [%v]",
			2,
			engine.runDKGThreshold,
		)
	}

	if len(engine.runDKGParticipants) != 3 {
		t.Fatalf(
			"unexpected RunDKG participants count\nexpected: [%v]\nactual:   [%v]",
			3,
			len(engine.runDKGParticipants),
		)
	}

	if engine.startCalled {
		t.Fatal("did not expect StartSignRound call for non-bootstrap tbtc-signer version")
	}

	if engine.finalizeCalled {
		t.Fatal("did not expect FinalizeSignRound call for non-bootstrap tbtc-signer version")
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_BootstrapVersion(
	t *testing.T,
) {
	engine := &mockBuildTaggedTBTCSignerEngine{
		version:           "tbtc-signer/0.1.0-bootstrap",
		finalizeSignature: buildTaggedTBTCSignerValidTestSignature(0x11),
	}
	UnregisterNativeTBTCSignerEngine()
	UnregisterNativeTBTCSignerFallbackObserver()
	UnregisterNativeTBTCSignerCoarseSignatureObserver()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)
	t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)
	t.Cleanup(UnregisterNativeTBTCSignerCoarseSignatureObserver)

	err := RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	var observedEvents []NativeTBTCSignerFallbackEvent
	err = RegisterNativeTBTCSignerFallbackObserver(
		func(event NativeTBTCSignerFallbackEvent) {
			observedEvents = append(observedEvents, event)
		},
	)
	if err != nil {
		t.Fatalf("unexpected observer registration error: [%v]", err)
	}

	var observedCoarseSignatureEvents []NativeTBTCSignerCoarseSignatureEvent
	err = RegisterNativeTBTCSignerCoarseSignatureObserver(
		func(event NativeTBTCSignerCoarseSignatureEvent) {
			observedCoarseSignatureEvents = append(
				observedCoarseSignatureEvents,
				event,
			)
		},
	)
	if err != nil {
		t.Fatalf("unexpected coarse observer registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	signature, err := primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(`{"keyGroup":"group-1"}`),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if signature == nil {
		t.Fatal("expected signature")
	}

	marshaledSignature, err := signature.Marshal()
	if err != nil {
		t.Fatalf("cannot marshal signature: [%v]", err)
	}

	expectedSignature := buildTaggedTBTCSignerValidTestSignature(0x11)
	if !bytes.Equal(marshaledSignature, expectedSignature) {
		t.Fatalf(
			"unexpected signature bytes\nexpected: [%x]\nactual:   [%x]",
			expectedSignature,
			marshaledSignature,
		)
	}

	if !engine.runDKGCalled {
		t.Fatal("expected RunDKG call in bootstrap tbtc-signer path")
	}

	if !engine.startCalled {
		t.Fatal("expected StartSignRound call in bootstrap tbtc-signer path")
	}

	if engine.startSessionID != "session-1" {
		t.Fatalf(
			"unexpected StartSignRound session ID\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			engine.startSessionID,
		)
	}

	if engine.startMemberID != 1 {
		t.Fatalf(
			"unexpected StartSignRound member identifier\nexpected: [%v]\nactual:   [%v]",
			1,
			engine.startMemberID,
		)
	}

	if engine.startKeyGroup != "group-1" {
		t.Fatalf(
			"unexpected StartSignRound key group\nexpected: [%v]\nactual:   [%v]",
			"group-1",
			engine.startKeyGroup,
		)
	}

	expectedSigningParticipants := []uint16{1, 2, 3}
	if !reflect.DeepEqual(engine.startSigningParticipants, expectedSigningParticipants) {
		t.Fatalf(
			"unexpected StartSignRound signing participants\nexpected: [%v]\nactual:   [%v]",
			expectedSigningParticipants,
			engine.startSigningParticipants,
		)
	}

	if !engine.finalizeCalled {
		t.Fatal("expected FinalizeSignRound call in bootstrap tbtc-signer path")
	}

	if len(observedEvents) != 0 {
		t.Fatalf(
			"did not expect fallback events\nactual: [%v]",
			observedEvents,
		)
	}

	if len(observedCoarseSignatureEvents) != 1 {
		t.Fatalf(
			"unexpected coarse signature event count\nexpected: [%d]\nactual:   [%d]",
			1,
			len(observedCoarseSignatureEvents),
		)
	}

	if observedCoarseSignatureEvents[0].SessionID != "session-1" {
		t.Fatalf(
			"unexpected coarse signature session ID\nexpected: [%s]\nactual:   [%s]",
			"session-1",
			observedCoarseSignatureEvents[0].SessionID,
		)
	}

	if observedCoarseSignatureEvents[0].EngineVersion != "tbtc-signer/0.1.0-bootstrap" {
		t.Fatalf(
			"unexpected coarse signature engine version\nexpected: [%s]\nactual:   [%s]",
			"tbtc-signer/0.1.0-bootstrap",
			observedCoarseSignatureEvents[0].EngineVersion,
		)
	}

	if engine.finalizeSessionID != "session-1" {
		t.Fatalf(
			"unexpected FinalizeSignRound session ID\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			engine.finalizeSessionID,
		)
	}

	if len(engine.finalizeInputs) != 3 {
		t.Fatalf(
			"unexpected FinalizeSignRound contributions count\nexpected: [%v]\nactual:   [%v]",
			3,
			len(engine.finalizeInputs),
		)
	}

	expectedIdentifiers := []uint16{1, 2, 3}
	for i, contribution := range engine.finalizeInputs {
		if contribution.Identifier != expectedIdentifiers[i] {
			t.Fatalf(
				"unexpected contribution identifier at index [%d]\nexpected: [%v]\nactual:   [%v]",
				i,
				expectedIdentifiers[i],
				contribution.Identifier,
			)
		}

		if len(contribution.Data) == 0 {
			t.Fatalf("expected non-empty contribution data at index [%d]", i)
		}
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_BootstrapVersion_TaprootMerkleRoot(
	t *testing.T,
) {
	engine := &mockBuildTaggedTBTCSignerEngine{
		version:           "tbtc-signer/0.1.0-bootstrap",
		finalizeSignature: buildTaggedTBTCSignerValidTestSignature(0x22),
	}
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)

	err := RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	var taprootMerkleRoot [32]byte
	taprootMerkleRoot[0] = 0xab
	taprootMerkleRoot[31] = 0xcd

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	signature, err := primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
		TaprootMerkleRoot:  &taprootMerkleRoot,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(`{"keyGroup":"group-1"}`),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if signature == nil {
		t.Fatal("expected signature")
	}

	if engine.startTaprootMerkleRoot == nil {
		t.Fatal("expected StartSignRound taproot merkle root")
	}
	if !bytes.Equal(engine.startTaprootMerkleRoot[:], taprootMerkleRoot[:]) {
		t.Fatalf(
			"unexpected StartSignRound taproot merkle root\nexpected: [%x]\nactual:   [%x]",
			taprootMerkleRoot,
			*engine.startTaprootMerkleRoot,
		)
	}

	if engine.finalizeTaprootMerkleRoot == nil {
		t.Fatal("expected FinalizeSignRound taproot merkle root")
	}
	if !bytes.Equal(engine.finalizeTaprootMerkleRoot[:], taprootMerkleRoot[:]) {
		t.Fatalf(
			"unexpected FinalizeSignRound taproot merkle root\nexpected: [%x]\nactual:   [%x]",
			taprootMerkleRoot,
			*engine.finalizeTaprootMerkleRoot,
		)
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_BootstrapVersion_InvalidCoarseSignatureFallsBack(
	t *testing.T,
) {
	t.Setenv(AcceptScaffoldKeyGroupEnvVar, "true")

	engine := &mockBuildTaggedTBTCSignerEngine{
		version:           "tbtc-signer/0.1.0-bootstrap",
		finalizeSignature: []byte{0xaa},
	}
	UnregisterNativeTBTCSignerEngine()
	UnregisterNativeTBTCSignerFallbackObserver()
	UnregisterNativeTBTCSignerCoarseSignatureObserver()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)
	t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)
	t.Cleanup(UnregisterNativeTBTCSignerCoarseSignatureObserver)

	err := RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	var observedEvents []NativeTBTCSignerFallbackEvent
	err = RegisterNativeTBTCSignerFallbackObserver(
		func(event NativeTBTCSignerFallbackEvent) {
			observedEvents = append(observedEvents, event)
		},
	)
	if err != nil {
		t.Fatalf("unexpected observer registration error: [%v]", err)
	}

	var observedCoarseSignatureEvents []NativeTBTCSignerCoarseSignatureEvent
	err = RegisterNativeTBTCSignerCoarseSignatureObserver(
		func(event NativeTBTCSignerCoarseSignatureEvent) {
			observedCoarseSignatureEvents = append(
				observedCoarseSignatureEvents,
				event,
			)
		},
	)
	if err != nil {
		t.Fatalf("unexpected coarse observer registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	_, err = primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(`{"keyGroup":"group-1","keyGroupSource":"legacy-wallet-pubkey"}`),
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

	if !engine.runDKGCalled {
		t.Fatal("expected RunDKG call in bootstrap path")
	}

	if !engine.startCalled {
		t.Fatal("expected StartSignRound call in bootstrap path")
	}

	if !engine.finalizeCalled {
		t.Fatal("expected FinalizeSignRound call in bootstrap path")
	}

	if len(observedEvents) != 1 {
		t.Fatalf(
			"unexpected fallback event count\nexpected: [%d]\nactual:   [%d]",
			1,
			len(observedEvents),
		)
	}

	if !strings.Contains(
		observedEvents[0].Reason,
		"cannot decode tbtc-signer coarse signature",
	) {
		t.Fatalf(
			"expected fallback reason to include decode failure\nactual: [%s]",
			observedEvents[0].Reason,
		)
	}

	if len(observedCoarseSignatureEvents) != 0 {
		t.Fatalf(
			"did not expect coarse signature events\nactual: [%v]",
			observedCoarseSignatureEvents,
		)
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_RefusesScaffoldMaterialWithoutOptIn(
	t *testing.T,
) {
	// Closes the persistence-vs-execution gap: even when the native
	// tbtc-signer engine is registered (scaffold material on disk from a
	// previous opted-in session is enough to reach this point), the FFI
	// signing path must refuse to feed RunDKG placeholder participant
	// pubkeys without an active operator opt-in. Force the env var off so
	// any value inherited from the test runner's containing process cannot
	// suppress the refusal.
	t.Setenv(AcceptScaffoldKeyGroupEnvVar, "")

	engine := &mockBuildTaggedTBTCSignerEngine{
		version: "tbtc-signer/0.1.0-bootstrap",
		runDKGResult: &NativeTBTCSignerDKGResult{
			SessionID:        "session-scaffold-refused",
			KeyGroup:         "group-from-dkg",
			ParticipantCount: 3,
			Threshold:        2,
			CreatedAtUnix:    1,
		},
		finalizeSignature: buildTaggedTBTCSignerValidTestSignature(0x22),
	}
	UnregisterNativeTBTCSignerEngine()
	UnregisterNativeTBTCSignerCoarseSignatureObserver()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)
	t.Cleanup(UnregisterNativeTBTCSignerCoarseSignatureObserver)

	if err := RegisterNativeTBTCSignerEngine(engine); err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	signature, err := primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-scaffold-refused",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format: NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(
				`{"keyGroup":"legacy-wallet-derived","keyGroupSource":"legacy-wallet-pubkey"}`,
			),
		},
	})
	if err == nil {
		t.Fatal("expected scaffold-refusal error from FFI signing path")
	}
	if signature != nil {
		t.Fatal("expected nil signature when scaffold path is refused")
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"expected ErrNativeCryptographyUnavailable wrap; got: [%v]",
			err,
		)
	}
	if !strings.Contains(err.Error(), AcceptScaffoldKeyGroupEnvVar) {
		t.Fatalf(
			"expected scaffold-refusal error to reference %s; got: [%v]",
			AcceptScaffoldKeyGroupEnvVar,
			err,
		)
	}
	if !strings.Contains(err.Error(), NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey) {
		t.Fatalf(
			"expected scaffold-refusal error to reference %s; got: [%v]",
			NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
			err,
		)
	}

	if engine.runDKGCalled {
		t.Fatal(
			"RunDKG must not be called when the scaffold opt-in flag is unset; " +
				"refusing before the placeholder participant pubkeys are built " +
				"is the whole point of the fence",
		)
	}
	if engine.startCalled {
		t.Fatal("StartSignRound must not be called when the scaffold path is refused")
	}
	if engine.finalizeCalled {
		t.Fatal(
			"FinalizeSignRound must not be called when the scaffold path is refused",
		)
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_BootstrapVersion_LegacyKeyGroupSourceUsesRunDKGResult(
	t *testing.T,
) {
	// Scaffold-era path: legacy-wallet-pubkey signer material is refused by
	// default; the operator opt-in via AcceptScaffoldKeyGroupEnvVar is what
	// lets this test exercise the substitution. Production deployments must
	// never set this.
	t.Setenv(AcceptScaffoldKeyGroupEnvVar, "true")

	engine := &mockBuildTaggedTBTCSignerEngine{
		version: "tbtc-signer/0.1.0-bootstrap",
		runDKGResult: &NativeTBTCSignerDKGResult{
			SessionID:        "session-1",
			KeyGroup:         "group-from-dkg",
			ParticipantCount: 3,
			Threshold:        2,
			CreatedAtUnix:    1,
		},
		finalizeSignature: buildTaggedTBTCSignerValidTestSignature(0x22),
	}
	UnregisterNativeTBTCSignerEngine()
	UnregisterNativeTBTCSignerCoarseSignatureObserver()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)
	t.Cleanup(UnregisterNativeTBTCSignerCoarseSignatureObserver)

	err := RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	var observedCoarseSignatureEvents []NativeTBTCSignerCoarseSignatureEvent
	err = RegisterNativeTBTCSignerCoarseSignatureObserver(
		func(event NativeTBTCSignerCoarseSignatureEvent) {
			observedCoarseSignatureEvents = append(
				observedCoarseSignatureEvents,
				event,
			)
		},
	)
	if err != nil {
		t.Fatalf("unexpected coarse observer registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	signature, err := primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format: NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(
				`{"keyGroup":"legacy-wallet-derived","keyGroupSource":"legacy-wallet-pubkey"}`,
			),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	if signature == nil {
		t.Fatal("expected signature")
	}

	marshaledSignature, err := signature.Marshal()
	if err != nil {
		t.Fatalf("cannot marshal signature: [%v]", err)
	}

	expectedSignature := buildTaggedTBTCSignerValidTestSignature(0x22)
	if !bytes.Equal(marshaledSignature, expectedSignature) {
		t.Fatalf(
			"unexpected signature bytes\nexpected: [%x]\nactual:   [%x]",
			expectedSignature,
			marshaledSignature,
		)
	}

	if !engine.startCalled {
		t.Fatal("expected StartSignRound call in bootstrap path")
	}

	if engine.startKeyGroup != "group-from-dkg" {
		t.Fatalf(
			"unexpected StartSignRound key group\nexpected: [%v]\nactual:   [%v]",
			"group-from-dkg",
			engine.startKeyGroup,
		)
	}

	expectedSigningParticipants := []uint16{1, 2, 3}
	if !reflect.DeepEqual(engine.startSigningParticipants, expectedSigningParticipants) {
		t.Fatalf(
			"unexpected StartSignRound signing participants\nexpected: [%v]\nactual:   [%v]",
			expectedSigningParticipants,
			engine.startSigningParticipants,
		)
	}

	if !engine.finalizeCalled {
		t.Fatal("expected FinalizeSignRound call in bootstrap path")
	}

	if len(observedCoarseSignatureEvents) != 1 {
		t.Fatalf(
			"unexpected coarse signature event count\nexpected: [%d]\nactual:   [%d]",
			1,
			len(observedCoarseSignatureEvents),
		)
	}

	if observedCoarseSignatureEvents[0].KeyGroupSource != "legacy-wallet-pubkey" {
		t.Fatalf(
			"unexpected coarse signature key group source\nexpected: [%s]\nactual:   [%s]",
			"legacy-wallet-pubkey",
			observedCoarseSignatureEvents[0].KeyGroupSource,
		)
	}

	if observedCoarseSignatureEvents[0].EngineVersion != "tbtc-signer/0.1.0-bootstrap" {
		t.Fatalf(
			"unexpected coarse signature engine version\nexpected: [%s]\nactual:   [%s]",
			"tbtc-signer/0.1.0-bootstrap",
			observedCoarseSignatureEvents[0].EngineVersion,
		)
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_BootstrapVersion_KeyGroupMismatchNonLegacySourceSkipsCoarseRound(
	t *testing.T,
) {
	engine := &mockBuildTaggedTBTCSignerEngine{
		version: "tbtc-signer/0.1.0-bootstrap",
		runDKGResult: &NativeTBTCSignerDKGResult{
			SessionID:        "session-1",
			KeyGroup:         "group-from-dkg",
			ParticipantCount: 3,
			Threshold:        2,
			CreatedAtUnix:    1,
		},
	}
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)

	err := RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	_, err = primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format: NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(
				`{"keyGroup":"legacy-wallet-derived","keyGroupSource":"dkg-persisted"}`,
			),
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

	if engine.startCalled {
		t.Fatal("did not expect StartSignRound call for non-legacy key-group mismatch")
	}

	if engine.finalizeCalled {
		t.Fatal("did not expect FinalizeSignRound call for non-legacy key-group mismatch")
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_NoEngineNoLegacyShare(
	t *testing.T,
) {
	// Scaffold-era signing path requires explicit operator opt-in; this test
	// exercises the engine-unavailable + no-legacy-share branch which lives
	// past the scaffold fence.
	t.Setenv(AcceptScaffoldKeyGroupEnvVar, "true")

	UnregisterNativeTBTCSignerEngine()
	UnregisterNativeTBTCSignerFallbackObserver()
	UnregisterNativeTBTCSignerCoarseSignatureObserver()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)
	t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)
	t.Cleanup(UnregisterNativeTBTCSignerCoarseSignatureObserver)

	var observedEvents []NativeTBTCSignerFallbackEvent
	err := RegisterNativeTBTCSignerFallbackObserver(
		func(event NativeTBTCSignerFallbackEvent) {
			observedEvents = append(observedEvents, event)
		},
	)
	if err != nil {
		t.Fatalf("unexpected observer registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	_, err = primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message:   big.NewInt(123),
		SessionID: "session-1",
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(`{"keyGroup":"group-1","keyGroupSource":"legacy-wallet-pubkey"}`),
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

	if len(observedEvents) != 1 {
		t.Fatalf(
			"unexpected fallback event count\nexpected: [%d]\nactual:   [%d]",
			1,
			len(observedEvents),
		)
	}

	event := observedEvents[0]
	if event.SessionID != "session-1" {
		t.Fatalf(
			"unexpected fallback session ID\nexpected: [%s]\nactual:   [%s]",
			"session-1",
			event.SessionID,
		)
	}

	if event.KeyGroupSource != "legacy-wallet-pubkey" {
		t.Fatalf(
			"unexpected fallback key group source\nexpected: [%s]\nactual:   [%s]",
			"legacy-wallet-pubkey",
			event.KeyGroupSource,
		)
	}

	if event.LegacyPrivateKeyShareExists {
		t.Fatal("expected fallback event without legacy private key share")
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_AttemptVariationRunDKGConflictFallsBack(
	t *testing.T,
) {
	t.Setenv(AcceptScaffoldKeyGroupEnvVar, "true")

	UnregisterNativeTBTCSignerEngine()
	UnregisterNativeTBTCSignerFallbackObserver()
	UnregisterNativeTBTCSignerCoarseSignatureObserver()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)
	t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)
	t.Cleanup(UnregisterNativeTBTCSignerCoarseSignatureObserver)

	var firstParticipants []NativeTBTCSignerDKGParticipant
	engine := &mockBuildTaggedTBTCSignerEngine{
		version: "tbtc-signer/0.1.0",
		runDKGFn: func(
			sessionID string,
			participants []NativeTBTCSignerDKGParticipant,
			threshold uint16,
		) (*NativeTBTCSignerDKGResult, error) {
			if firstParticipants == nil {
				firstParticipants = append(
					[]NativeTBTCSignerDKGParticipant{},
					participants...,
				)

				return &NativeTBTCSignerDKGResult{
					SessionID:        sessionID,
					KeyGroup:         "group-1",
					ParticipantCount: uint16(len(participants)),
					Threshold:        threshold,
					CreatedAtUnix:    1,
				}, nil
			}

			if !reflect.DeepEqual(participants, firstParticipants) {
				return nil, errors.New("session_conflict")
			}

			return &NativeTBTCSignerDKGResult{
				SessionID:        sessionID,
				KeyGroup:         "group-1",
				ParticipantCount: uint16(len(participants)),
				Threshold:        threshold,
				CreatedAtUnix:    1,
			}, nil
		},
	}

	err := RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	var observedEvents []NativeTBTCSignerFallbackEvent
	err = RegisterNativeTBTCSignerFallbackObserver(
		func(event NativeTBTCSignerFallbackEvent) {
			observedEvents = append(observedEvents, event)
		},
	)
	if err != nil {
		t.Fatalf("unexpected observer registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	baseRequest := &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(`{"keyGroup":"group-1","keyGroupSource":"legacy-wallet-pubkey"}`),
		},
	}

	_, err = primitive.Sign(nil, nil, baseRequest)
	if err == nil {
		t.Fatal("expected first signing error due to legacy fallback without private key share")
	}
	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected first signing error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	secondRequest := *baseRequest
	secondRequest.Attempt = &Attempt{
		Number:                 2,
		CoordinatorMemberIndex: 1,
		ExcludedMembersIndexes: []group.MemberIndex{3},
	}

	_, err = primitive.Sign(nil, nil, &secondRequest)
	if err == nil {
		t.Fatal("expected second signing error due to legacy fallback without private key share")
	}
	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected second signing error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if len(observedEvents) != 2 {
		t.Fatalf(
			"unexpected fallback event count\nexpected: [%d]\nactual:   [%d]",
			2,
			len(observedEvents),
		)
	}

	if !strings.Contains(observedEvents[1].Reason, "session_conflict") {
		t.Fatalf(
			"expected second fallback reason to include session_conflict\nactual: [%s]",
			observedEvents[1].Reason,
		)
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_BootstrapVersion_AttemptVariationStartSignRoundConflictFallsBack(
	t *testing.T,
) {
	t.Setenv(AcceptScaffoldKeyGroupEnvVar, "true")

	UnregisterNativeTBTCSignerEngine()
	UnregisterNativeTBTCSignerFallbackObserver()
	UnregisterNativeTBTCSignerCoarseSignatureObserver()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)
	t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)
	t.Cleanup(UnregisterNativeTBTCSignerCoarseSignatureObserver)

	var firstSigningParticipants []uint16
	var observedSigningParticipants [][]uint16

	engine := &mockBuildTaggedTBTCSignerEngine{
		version:           "tbtc-signer/0.1.0-bootstrap",
		finalizeSignature: buildTaggedTBTCSignerValidTestSignature(0x44),
		runDKGFn: func(
			sessionID string,
			participants []NativeTBTCSignerDKGParticipant,
			threshold uint16,
		) (*NativeTBTCSignerDKGResult, error) {
			return &NativeTBTCSignerDKGResult{
				SessionID:        sessionID,
				KeyGroup:         "group-1",
				ParticipantCount: uint16(len(participants)),
				Threshold:        threshold,
				CreatedAtUnix:    1,
			}, nil
		},
		startSignRoundFn: func(
			sessionID string,
			_ uint16,
			_ []byte,
			_ string,
			signingParticipants []uint16,
			_ *[32]byte,
		) (*NativeTBTCSignerRoundState, error) {
			observedSigningParticipants = append(
				observedSigningParticipants,
				append([]uint16{}, signingParticipants...),
			)

			if firstSigningParticipants == nil {
				firstSigningParticipants = append(
					[]uint16{},
					signingParticipants...,
				)
			} else if !reflect.DeepEqual(signingParticipants, firstSigningParticipants) {
				return nil, errors.New("session_conflict")
			}

			return &NativeTBTCSignerRoundState{
				SessionID:             sessionID,
				RoundID:               "round-1",
				RequiredContributions: 2,
				MessageDigestHex:      "00",
				SigningParticipants: append(
					[]uint16{},
					signingParticipants...,
				),
			}, nil
		},
	}

	err := RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	var observedEvents []NativeTBTCSignerFallbackEvent
	err = RegisterNativeTBTCSignerFallbackObserver(
		func(event NativeTBTCSignerFallbackEvent) {
			observedEvents = append(observedEvents, event)
		},
	)
	if err != nil {
		t.Fatalf("unexpected observer registration error: [%v]", err)
	}

	var observedCoarseSignatureEvents []NativeTBTCSignerCoarseSignatureEvent
	err = RegisterNativeTBTCSignerCoarseSignatureObserver(
		func(event NativeTBTCSignerCoarseSignatureEvent) {
			observedCoarseSignatureEvents = append(
				observedCoarseSignatureEvents,
				event,
			)
		},
	)
	if err != nil {
		t.Fatalf("unexpected coarse observer registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	baseRequest := &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: []byte(`{"keyGroup":"group-1","keyGroupSource":"legacy-wallet-pubkey"}`),
		},
	}

	firstSignature, err := primitive.Sign(nil, nil, baseRequest)
	if err != nil {
		t.Fatalf("unexpected first signing error: [%v]", err)
	}
	if firstSignature == nil {
		t.Fatal("expected first signature")
	}

	secondRequest := *baseRequest
	secondRequest.Attempt = &Attempt{
		Number:                 2,
		CoordinatorMemberIndex: 1,
		ExcludedMembersIndexes: []group.MemberIndex{2},
	}

	_, err = primitive.Sign(nil, nil, &secondRequest)
	if err == nil {
		t.Fatal("expected second signing error due to legacy fallback without private key share")
	}
	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected second signing error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if len(observedSigningParticipants) != 2 {
		t.Fatalf(
			"unexpected StartSignRound call count\nexpected: [%d]\nactual:   [%d]",
			2,
			len(observedSigningParticipants),
		)
	}

	expectedFirstParticipants := []uint16{1, 2, 3}
	if !reflect.DeepEqual(observedSigningParticipants[0], expectedFirstParticipants) {
		t.Fatalf(
			"unexpected first StartSignRound signing participants\nexpected: [%v]\nactual:   [%v]",
			expectedFirstParticipants,
			observedSigningParticipants[0],
		)
	}

	expectedSecondParticipants := []uint16{1, 3}
	if !reflect.DeepEqual(observedSigningParticipants[1], expectedSecondParticipants) {
		t.Fatalf(
			"unexpected second StartSignRound signing participants\nexpected: [%v]\nactual:   [%v]",
			expectedSecondParticipants,
			observedSigningParticipants[1],
		)
	}

	if len(observedEvents) != 1 {
		t.Fatalf(
			"unexpected fallback event count\nexpected: [%d]\nactual:   [%d]",
			1,
			len(observedEvents),
		)
	}

	if !strings.Contains(observedEvents[0].Reason, "session_conflict") {
		t.Fatalf(
			"expected fallback reason to include session_conflict\nactual: [%s]",
			observedEvents[0].Reason,
		)
	}

	if len(observedCoarseSignatureEvents) != 1 {
		t.Fatalf(
			"unexpected coarse signature event count\nexpected: [%d]\nactual:   [%d]",
			1,
			len(observedCoarseSignatureEvents),
		)
	}
}

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_InvalidAttemptPolicy_DoesNotFallback(
	t *testing.T,
) {
	// Scaffold-era signing path requires explicit operator opt-in; this test
	// exercises the FFI flow's invalid-attempt-policy branch, which lives
	// past the scaffold fence.
	t.Setenv(AcceptScaffoldKeyGroupEnvVar, "true")

	fixtures, err := tecdsatest.LoadPrivateKeyShareTestFixtures(3)
	if err != nil {
		t.Fatalf("failed loading key share fixtures: [%v]", err)
	}

	privateKeyShare := tecdsa.NewPrivateKeyShare(fixtures[0])
	privateKeySharePayload, err := privateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("failed marshaling private key share: [%v]", err)
	}

	signerMaterialPayload, err := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup:                 "group-1",
		KeyGroupSource:           NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
		LegacyPrivateKeyShareHex: hex.EncodeToString(privateKeySharePayload),
	})
	if err != nil {
		t.Fatalf("cannot marshal signer material payload: [%v]", err)
	}

	testCases := []struct {
		name    string
		attempt *Attempt
	}{
		{
			name: "zero attempt number",
			attempt: &Attempt{
				Number:                 0,
				CoordinatorMemberIndex: 1,
				IncludedMembersIndexes: []group.MemberIndex{1, 2},
			},
		},
		{
			name: "zero coordinator",
			attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 0,
				IncludedMembersIndexes: []group.MemberIndex{1, 2},
			},
		},
		{
			name: "coordinator not included",
			attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 3,
				IncludedMembersIndexes: []group.MemberIndex{1, 2},
			},
		},
		{
			name: "included members empty after exclusions",
			attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 1,
				ExcludedMembersIndexes: []group.MemberIndex{1, 2, 3},
			},
		},
		{
			name: "member included and excluded",
			attempt: &Attempt{
				Number:                 1,
				CoordinatorMemberIndex: 2,
				IncludedMembersIndexes: []group.MemberIndex{1, 2},
				ExcludedMembersIndexes: []group.MemberIndex{2},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			engine := &mockBuildTaggedTBTCSignerEngine{
				version: "tbtc-signer/0.1.0-bootstrap",
			}
			UnregisterNativeTBTCSignerEngine()
			UnregisterNativeTBTCSignerFallbackObserver()
			t.Cleanup(UnregisterNativeTBTCSignerEngine)
			t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)

			err := RegisterNativeTBTCSignerEngine(engine)
			if err != nil {
				t.Fatalf("unexpected registration error: [%v]", err)
			}

			var observedEvents []NativeTBTCSignerFallbackEvent
			err = RegisterNativeTBTCSignerFallbackObserver(
				func(event NativeTBTCSignerFallbackEvent) {
					observedEvents = append(observedEvents, event)
				},
			)
			if err != nil {
				t.Fatalf("unexpected observer registration error: [%v]", err)
			}

			primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

			_, err = primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
				Message:            big.NewInt(123),
				SessionID:          "session-1",
				MemberIndex:        1,
				GroupSize:          3,
				DishonestThreshold: 1,
				SignerMaterial: &NativeSignerMaterial{
					Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
					Payload: signerMaterialPayload,
				},
				Attempt: tc.attempt,
			})
			if err == nil {
				t.Fatal("expected error")
			}

			if !errors.Is(err, ErrNativeBridgeOperationFailed) {
				t.Fatalf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]",
					ErrNativeBridgeOperationFailed,
					err,
				)
			}

			if !errors.Is(err, ErrInvalidSigningAttemptPolicy) {
				t.Fatalf(
					"unexpected error\nexpected: [%v]\nactual:   [%v]",
					ErrInvalidSigningAttemptPolicy,
					err,
				)
			}

			if errors.Is(err, ErrNativeCryptographyUnavailable) {
				t.Fatalf(
					"unexpected error\nexpected not to include: [%v]\nactual:                 [%v]",
					ErrNativeCryptographyUnavailable,
					err,
				)
			}

			if engine.runDKGCalled {
				t.Fatal("did not expect RunDKG call for invalid attempt policy")
			}

			if len(observedEvents) != 0 {
				t.Fatalf(
					"did not expect fallback events\nactual: [%v]",
					observedEvents,
				)
			}
		})
	}
}

func TestIsBuildTaggedTBTCSignerConsumedAttemptReplayError(t *testing.T) {
	// Locking-mutex-free unit-coverage for the replay detector. Each case
	// constructs an error in the shape that flows out of the FFI bridge today
	// and asserts the detector's decision.
	cases := []struct {
		name  string
		err   error
		match bool
	}{
		{
			name:  "nil error is not a replay",
			err:   nil,
			match: false,
		},
		{
			name: "structured code wins over message wording",
			err: fmt.Errorf(
				"%w: tbtc-signer bridge operation [StartSignRound] failed: [%w]",
				ErrNativeBridgeOperationFailed,
				&buildTaggedTBTCSignerStructuredError{
					Code:    buildTaggedTBTCSignerConsumedAttemptReplayErrorCode,
					Message: "rust message wording is not load-bearing here",
				},
			),
			match: true,
		},
		{
			name: "structured but different code does not match",
			err: fmt.Errorf(
				"%w: tbtc-signer bridge operation [StartSignRound] failed: [%w]",
				ErrNativeBridgeOperationFailed,
				&buildTaggedTBTCSignerStructuredError{
					Code:    "session_conflict",
					Message: "attempt_id [x] already consumed for sign attempt in session [y]",
				},
			),
			match: false,
		},
		{
			name: "legacy substring still matches when code is missing",
			err: fmt.Errorf(
				"%w: tbtc-signer bridge operation [StartSignRound] failed: [%w]",
				ErrNativeBridgeOperationFailed,
				&buildTaggedTBTCSignerStructuredError{
					Code:    "",
					Message: "attempt_id [att-1] already consumed for sign attempt in session [sess-1]",
				},
			),
			match: true,
		},
		{
			// Pre-dedicated-variant signer builds route the replay path
			// through Validation, so the code on the wire is
			// validation_error and only the message identifies replay.
			name: "validation_error code with legacy wording is a replay",
			err: fmt.Errorf(
				"%w: tbtc-signer bridge operation [StartSignRound] failed: [%w]",
				ErrNativeBridgeOperationFailed,
				&buildTaggedTBTCSignerStructuredError{
					Code:    "validation_error",
					Message: "attempt_id [att-1] already consumed for sign attempt in session [sess-1]",
				},
			),
			match: true,
		},
		{
			// A validation_error that is NOT the replay path must not be
			// flagged as a replay even if surrounding error chain noise
			// happens to mention attempt_id elsewhere.
			name: "validation_error without legacy wording is not a replay",
			err: fmt.Errorf(
				"%w: tbtc-signer bridge operation [StartSignRound] failed: [%w]",
				ErrNativeBridgeOperationFailed,
				&buildTaggedTBTCSignerStructuredError{
					Code:    "validation_error",
					Message: "session_id is empty",
				},
			),
			match: false,
		},
		{
			name: "legacy substring still matches when error is a plain wrapper",
			err: fmt.Errorf(
				"native FROST bridge operation failed: tbtc-signer bridge " +
					"operation [StartSignRound] failed: [validation_error: " +
					"attempt_id [att-1] already consumed for sign attempt in " +
					"session [sess-1]]",
			),
			match: true,
		},
		{
			name: "unrelated error is not a replay",
			err: fmt.Errorf(
				"%w: tbtc-signer bridge operation [StartSignRound] failed: [%w]",
				ErrNativeBridgeOperationFailed,
				&buildTaggedTBTCSignerStructuredError{
					Code:    "validation_error",
					Message: "session_id is empty",
				},
			),
			match: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBuildTaggedTBTCSignerConsumedAttemptReplayError(tc.err); got != tc.match {
				t.Fatalf(
					"detector returned [%v]; expected [%v] for error [%v]",
					got,
					tc.match,
					tc.err,
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

func TestBuildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive_Sign_TBTCSignerPath_ConsumedAttemptReplay_DoesNotFallback(
	t *testing.T,
) {
	// Scaffold-era signing path requires explicit operator opt-in; this test
	// exercises the FFI flow's consumed-attempt-replay branch, which lives
	// past the scaffold fence.
	t.Setenv(AcceptScaffoldKeyGroupEnvVar, "true")

	fixtures, err := tecdsatest.LoadPrivateKeyShareTestFixtures(3)
	if err != nil {
		t.Fatalf("failed loading key share fixtures: [%v]", err)
	}

	privateKeyShare := tecdsa.NewPrivateKeyShare(fixtures[0])
	privateKeySharePayload, err := privateKeyShare.Marshal()
	if err != nil {
		t.Fatalf("failed marshaling private key share: [%v]", err)
	}

	signerMaterialPayload, err := json.Marshal(&NativeTBTCSignerMaterialPayload{
		KeyGroup:                 "group-1",
		KeyGroupSource:           NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
		LegacyPrivateKeyShareHex: hex.EncodeToString(privateKeySharePayload),
	})
	if err != nil {
		t.Fatalf("cannot marshal signer material payload: [%v]", err)
	}

	engine := &mockBuildTaggedTBTCSignerEngine{
		version: "tbtc-signer/0.1.0-bootstrap",
		startErr: errors.New(
			"validation: attempt_id [11] already consumed for sign attempt in session [session-1]",
		),
	}
	UnregisterNativeTBTCSignerEngine()
	UnregisterNativeTBTCSignerFallbackObserver()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)
	t.Cleanup(UnregisterNativeTBTCSignerFallbackObserver)

	err = RegisterNativeTBTCSignerEngine(engine)
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	var observedEvents []NativeTBTCSignerFallbackEvent
	err = RegisterNativeTBTCSignerFallbackObserver(
		func(event NativeTBTCSignerFallbackEvent) {
			observedEvents = append(observedEvents, event)
		},
	)
	if err != nil {
		t.Fatalf("unexpected observer registration error: [%v]", err)
	}

	primitive := &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}

	_, err = primitive.Sign(nil, nil, &NativeExecutionFFISigningRequest{
		Message:            big.NewInt(123),
		SessionID:          "session-1",
		MemberIndex:        1,
		GroupSize:          3,
		DishonestThreshold: 1,
		SignerMaterial: &NativeSignerMaterial{
			Format:  NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: signerMaterialPayload,
		},
		Attempt: &Attempt{
			Number:                 1,
			CoordinatorMemberIndex: 1,
			IncludedMembersIndexes: []group.MemberIndex{1, 2},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			ErrNativeBridgeOperationFailed,
			err,
		)
	}

	if !errors.Is(err, ErrConsumedSigningAttemptReplay) {
		t.Fatalf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			ErrConsumedSigningAttemptReplay,
			err,
		)
	}

	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected error\nexpected not to include: [%v]\nactual:                 [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if !engine.runDKGCalled {
		t.Fatal("expected RunDKG call before consumed-attempt replay rejection")
	}

	if len(observedEvents) != 0 {
		t.Fatalf(
			"did not expect fallback events\nactual: [%v]",
			observedEvents,
		)
	}

	if !strings.Contains(err.Error(), "already consumed for sign attempt") {
		t.Fatalf(
			"expected replay fragment in error message\nactual: [%v]",
			err,
		)
	}
}
