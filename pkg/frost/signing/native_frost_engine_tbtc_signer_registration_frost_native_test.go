//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

func TestRegisterBuildTaggedTBTCSignerEngine(t *testing.T) {
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)

	err := registerBuildTaggedNativeFROSTSigningEngine()
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	engine := currentNativeTBTCSignerEngine()
	if engine == nil {
		t.Fatal("expected native tbtc-signer engine registration")
	}

	_, err = engine.StartSignRound(
		"session-1",
		1,
		[]byte("message"),
		"key-group",
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected unavailable tbtc-signer bridge error")
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"expected native cryptography unavailable error: [%v], got [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("unexpected bridge error: [%v]", err)
	}

	_, err = engine.BuildTaprootTx(
		"session-1",
		[]NativeTBTCSignerTxInput{
			{TxIDHex: "11", Vout: 0, ValueSats: 1},
		},
		[]NativeTBTCSignerTxOutput{
			{ScriptPubKeyHex: "0014", ValueSats: 1},
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected unavailable tbtc-signer build-tx bridge error")
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"expected native cryptography unavailable error: [%v], got [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	versionedEngine, ok := engine.(interface {
		Version() (string, error)
	})
	if !ok {
		t.Fatal("expected versioned native tbtc-signer engine")
	}

	_, err = versionedEngine.Version()
	if err == nil {
		t.Fatal("expected unavailable tbtc-signer version bridge error")
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"expected native cryptography unavailable error: [%v], got [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}
}

func TestBuildTaggedTBTCSignerInteractiveFROSTBridge_WithLinkedSigner(t *testing.T) {
	t.Setenv("TBTC_SIGNER_PROFILE", "development")
	t.Setenv("TBTC_SIGNER_ENFORCE_PROVENANCE_GATE", "false")

	engine := &buildTaggedTBTCSignerEngine{}
	participantIDs := []byte{1, 2, 3}
	participantIdentifiers := make(map[byte]string, len(participantIDs))
	for _, participantID := range participantIDs {
		participantIdentifiers[participantID] = buildTaggedTBTCSignerTestIdentifier(
			participantID,
		)
	}

	part1Results := make(map[byte]*NativeFROSTDKGPart1Result, len(participantIDs))
	for _, participantID := range participantIDs {
		result, err := engine.Part1(
			participantIdentifiers[participantID],
			3,
			2,
		)
		if err != nil {
			if errors.Is(err, ErrNativeCryptographyUnavailable) {
				t.Skip("linked tbtc-signer FFI symbols unavailable")
			}
			t.Fatalf("unexpected DKG part1 error: [%v]", err)
		}
		if result.Package.Identifier != participantIdentifiers[participantID] {
			t.Fatalf("unexpected DKG part1 identifier: [%s]", result.Package.Identifier)
		}
		part1Results[participantID] = result
	}

	part2Results := make(map[byte]*NativeFROSTDKGPart2Result, len(participantIDs))
	for _, participantID := range participantIDs {
		round1Packages := make([]*NativeFROSTDKGRound1Package, 0, 2)
		for _, otherParticipantID := range participantIDs {
			if otherParticipantID == participantID {
				continue
			}
			round1Packages = append(
				round1Packages,
				part1Results[otherParticipantID].Package,
			)
		}

		result, err := engine.Part2(
			part1Results[participantID].SecretPackage,
			round1Packages,
		)
		if err != nil {
			t.Fatalf("unexpected DKG part2 error: [%v]", err)
		}
		if len(result.Packages) != 2 {
			t.Fatalf("unexpected DKG part2 package count: [%d]", len(result.Packages))
		}
		part2Results[participantID] = result
	}

	part3Results := make(map[byte]*NativeFROSTDKGResult, len(participantIDs))
	for _, participantID := range participantIDs {
		round1Packages := make([]*NativeFROSTDKGRound1Package, 0, 2)
		for _, otherParticipantID := range participantIDs {
			if otherParticipantID == participantID {
				continue
			}
			round1Packages = append(
				round1Packages,
				part1Results[otherParticipantID].Package,
			)
		}

		round2Packages := make([]*NativeFROSTDKGRound2Package, 0, 2)
		for _, senderParticipantID := range participantIDs {
			if senderParticipantID == participantID {
				continue
			}
			var packageForRecipient *NativeFROSTDKGRound2Package
			for _, pkg := range part2Results[senderParticipantID].Packages {
				if pkg.Identifier == participantIdentifiers[participantID] {
					packageForRecipient = pkg
					break
				}
			}
			if packageForRecipient == nil {
				t.Fatalf(
					"missing DKG round2 package from [%d] to [%d]",
					senderParticipantID,
					participantID,
				)
			}
			copied := *packageForRecipient
			copied.SenderIdentifier = participantIdentifiers[senderParticipantID]
			round2Packages = append(round2Packages, &copied)
		}

		result, err := engine.Part3(
			part2Results[participantID].SecretPackage,
			round1Packages,
			round2Packages,
		)
		if err != nil {
			t.Fatalf("unexpected DKG part3 error: [%v]", err)
		}
		if result.KeyPackage.Identifier != participantIdentifiers[participantID] {
			t.Fatalf("unexpected DKG key package identifier")
		}
		if len(result.PublicKeyPackage.VerifyingKey) != 64 {
			t.Fatalf(
				"unexpected DKG x-only verifying key length: [%d]",
				len(result.PublicKeyPackage.VerifyingKey),
			)
		}
		if len(result.PublicKeyPackage.VerifyingShares) != 3 {
			t.Fatalf(
				"unexpected DKG verifying share count: [%d]",
				len(result.PublicKeyPackage.VerifyingShares),
			)
		}
		part3Results[participantID] = result
	}

	verifyingKey := part3Results[1].PublicKeyPackage.VerifyingKey
	for _, participantID := range participantIDs {
		if part3Results[participantID].PublicKeyPackage.VerifyingKey != verifyingKey {
			t.Fatal("DKG participants produced different group verifying keys")
		}
	}

	signingParticipants := []byte{1, 2}
	commitments := make([]nativeFROSTCommitment, 0, len(signingParticipants))
	noncesByParticipant := make(map[byte][]byte, len(signingParticipants))
	for _, participantID := range signingParticipants {
		nonces, commitmentIdentifier, commitmentData, err :=
			engine.GenerateNoncesAndCommitments(
				part3Results[participantID].KeyPackage.Identifier,
				part3Results[participantID].KeyPackage.Data,
			)
		if err != nil {
			t.Fatalf("unexpected nonce generation error: [%v]", err)
		}
		commitments = append(commitments, nativeFROSTCommitment{
			Identifier: commitmentIdentifier,
			Data:       commitmentData,
		})
		noncesByParticipant[participantID] = nonces
	}

	message := bytesOf(0x42, 32)
	signingPackage, err := engine.NewSigningPackage(message, commitments)
	if err != nil {
		t.Fatalf("unexpected signing package error: [%v]", err)
	}

	signatureShares := make(
		[]nativeFROSTSignatureShare,
		0,
		len(signingParticipants),
	)
	for _, participantID := range signingParticipants {
		signatureShareIdentifier, signatureShareData, err := engine.Sign(
			signingPackage,
			noncesByParticipant[participantID],
			part3Results[participantID].KeyPackage.Identifier,
			part3Results[participantID].KeyPackage.Data,
		)
		if err != nil {
			t.Fatalf("unexpected signature share error: [%v]", err)
		}
		signatureShares = append(signatureShares, nativeFROSTSignatureShare{
			Identifier: signatureShareIdentifier,
			Data:       signatureShareData,
		})
	}

	signatureBytes, err := engine.Aggregate(
		signingPackage,
		signatureShares,
		part3Results[1].PublicKeyPackage,
	)
	if err != nil {
		t.Fatalf("unexpected aggregate error: [%v]", err)
	}
	if len(signatureBytes) != 64 {
		t.Fatalf("unexpected aggregate signature length: [%d]", len(signatureBytes))
	}

	publicKeyBytes, err := hex.DecodeString(verifyingKey)
	if err != nil {
		t.Fatalf("cannot decode verifying key: [%v]", err)
	}
	publicKey, err := schnorr.ParsePubKey(publicKeyBytes)
	if err != nil {
		t.Fatalf("cannot parse verifying key: [%v]", err)
	}
	signature, err := schnorr.ParseSignature(signatureBytes)
	if err != nil {
		t.Fatalf("cannot parse aggregate signature: [%v]", err)
	}
	if !signature.Verify(message, publicKey) {
		t.Fatal("aggregate signature does not verify under DKG x-only key")
	}
}

func buildTaggedTBTCSignerTestIdentifier(memberIndex byte) string {
	identifier := make([]byte, 32)
	identifier[0] = memberIndex
	return fmt.Sprintf("%q", hex.EncodeToString(identifier))
}

func bytesOf(value byte, length int) []byte {
	bytes := make([]byte, length)
	for i := range bytes {
		bytes[i] = value
	}
	return bytes
}

func TestBuildTaggedTBTCSignerResultStatusError_Unavailable(t *testing.T) {
	err := buildTaggedTBTCSignerResultStatusError(
		"BuildTaprootTx",
		buildTaggedTBTCSignerUnavailableStatusCode,
		nil,
	)
	if err == nil {
		t.Fatal("expected unavailable error")
	}

	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"expected native cryptography unavailable error: [%v], got [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf(
			"did not expect native bridge operation failed error: [%v]",
			err,
		)
	}
}

func TestBuildTaggedTBTCSignerResultStatusError_BridgeOperationFailure(t *testing.T) {
	err := buildTaggedTBTCSignerResultStatusError(
		"BuildTaprootTx",
		2,
		[]byte(`{"code":"validation","message":"invalid input"}`),
	)
	if err == nil {
		t.Fatal("expected bridge operation failure error")
	}

	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf(
			"expected native bridge operation failed error: [%v], got [%v]",
			ErrNativeBridgeOperationFailed,
			err,
		)
	}

	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"did not expect native cryptography unavailable error: [%v]",
			err,
		)
	}

	if !strings.Contains(err.Error(), "validation: invalid input") {
		t.Fatalf("unexpected bridge operation error: [%v]", err)
	}
}

func TestBuildTaggedTBTCSignerResultStatusError_BridgeOperationFailure_InvalidPayload(
	t *testing.T,
) {
	err := buildTaggedTBTCSignerResultStatusError(
		"BuildTaprootTx",
		2,
		[]byte("{invalid-json"),
	)
	if err == nil {
		t.Fatal("expected bridge operation failure error")
	}

	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf(
			"expected native bridge operation failed error: [%v], got [%v]",
			ErrNativeBridgeOperationFailed,
			err,
		)
	}

	if !strings.Contains(err.Error(), "cannot decode error payload") {
		t.Fatalf("unexpected bridge operation error: [%v]", err)
	}
}

func TestBuildTaggedTBTCSignerResultStatusError_BridgeOperationFailure_FallbackPayload(
	t *testing.T,
) {
	err := buildTaggedTBTCSignerResultStatusError(
		"BuildTaprootTx",
		2,
		[]byte(`{"code":"internal_error","message":"failed to encode error"}`),
	)
	if err == nil {
		t.Fatal("expected bridge operation failure error")
	}

	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf(
			"expected native bridge operation failed error: [%v], got [%v]",
			ErrNativeBridgeOperationFailed,
			err,
		)
	}

	if !strings.Contains(err.Error(), "internal_error: failed to encode error") {
		t.Fatalf("unexpected bridge operation error: [%v]", err)
	}
}

func TestBuildTaggedTBTCSignerRunDKGRequestPayload(t *testing.T) {
	payload, err := buildTaggedTBTCSignerRunDKGRequestPayload(
		"session-1",
		[]NativeTBTCSignerDKGParticipant{
			{
				Identifier:   1,
				PublicKeyHex: "02aa",
			},
			{
				Identifier:   2,
				PublicKeyHex: "02bb",
			},
		},
		2,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerRunDKGRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.SessionID != "session-1" {
		t.Fatalf(
			"unexpected session id\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			request.SessionID,
		)
	}

	if request.Threshold != 2 {
		t.Fatalf(
			"unexpected threshold\nexpected: [%v]\nactual:   [%v]",
			2,
			request.Threshold,
		)
	}

	if len(request.Participants) != 2 {
		t.Fatalf(
			"unexpected participants count\nexpected: [%v]\nactual:   [%v]",
			2,
			len(request.Participants),
		)
	}

	if request.Participants[0].Identifier != 1 {
		t.Fatalf(
			"unexpected participant identifier\nexpected: [%v]\nactual:   [%v]",
			1,
			request.Participants[0].Identifier,
		)
	}

	if request.Participants[0].PublicKeyHex != "02aa" {
		t.Fatalf(
			"unexpected participant public key hex\nexpected: [%v]\nactual:   [%v]",
			"02aa",
			request.Participants[0].PublicKeyHex,
		)
	}

	if request.DKGSeedHex != nil {
		t.Fatalf("unexpected DKG seed hex: [%v]", *request.DKGSeedHex)
	}
}

func TestBuildTaggedTBTCSignerRunDKGRequestPayloadWithSeed(t *testing.T) {
	payload, err := buildTaggedTBTCSignerRunDKGRequestPayloadWithSeed(
		"session-1",
		[]NativeTBTCSignerDKGParticipant{
			{
				Identifier:   1,
				PublicKeyHex: "02aa",
			},
			{
				Identifier:   2,
				PublicKeyHex: "02bb",
			},
		},
		2,
		"0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20",
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerRunDKGRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.DKGSeedHex == nil {
		t.Fatal("expected DKG seed hex")
	}
	if *request.DKGSeedHex !=
		"0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20" {
		t.Fatalf("unexpected DKG seed hex: [%v]", *request.DKGSeedHex)
	}
}

func TestBuildTaggedTBTCSignerRunDKGRequestPayload_RejectsInvalidInput(t *testing.T) {
	testCases := []struct {
		name         string
		sessionID    string
		participants []NativeTBTCSignerDKGParticipant
		threshold    uint16
	}{
		{
			name:         "empty session id",
			sessionID:    "",
			participants: []NativeTBTCSignerDKGParticipant{{Identifier: 1, PublicKeyHex: "02aa"}},
			threshold:    2,
		},
		{
			name:         "empty participants",
			sessionID:    "session-1",
			participants: nil,
			threshold:    2,
		},
		{
			name:      "zero threshold",
			sessionID: "session-1",
			participants: []NativeTBTCSignerDKGParticipant{
				{Identifier: 1, PublicKeyHex: "02aa"},
			},
			threshold: 0,
		},
		{
			name:      "participant zero identifier",
			sessionID: "session-1",
			participants: []NativeTBTCSignerDKGParticipant{
				{Identifier: 0, PublicKeyHex: "02aa"},
			},
			threshold: 1,
		},
		{
			name:      "participant empty public key hex",
			sessionID: "session-1",
			participants: []NativeTBTCSignerDKGParticipant{
				{Identifier: 1, PublicKeyHex: ""},
			},
			threshold: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTaggedTBTCSignerRunDKGRequestPayload(
				tc.sessionID,
				tc.participants,
				tc.threshold,
			)
			if err == nil {
				t.Fatal("expected payload build error")
			}

			if !errors.Is(err, ErrNativeBridgeOperationFailed) {
				t.Fatalf(
					"expected native bridge operation failed error: [%v], got [%v]",
					ErrNativeBridgeOperationFailed,
					err,
				)
			}

			if errors.Is(err, ErrNativeCryptographyUnavailable) {
				t.Fatalf(
					"did not expect native cryptography unavailable error: [%v]",
					err,
				)
			}
		})
	}
}

func TestDecodeBuildTaggedTBTCSignerRunDKGResponse(t *testing.T) {
	result, err := decodeBuildTaggedTBTCSignerRunDKGResponse(
		[]byte(
			`{"session_id":"session-1","key_group":"group-1","participant_count":3,"threshold":2,"created_at_unix":123456789}`,
		),
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}

	if result.SessionID != "session-1" {
		t.Fatalf(
			"unexpected session id\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			result.SessionID,
		)
	}

	if result.KeyGroup != "group-1" {
		t.Fatalf(
			"unexpected key group\nexpected: [%v]\nactual:   [%v]",
			"group-1",
			result.KeyGroup,
		)
	}

	if result.ParticipantCount != 3 {
		t.Fatalf(
			"unexpected participant count\nexpected: [%v]\nactual:   [%v]",
			3,
			result.ParticipantCount,
		)
	}

	if result.Threshold != 2 {
		t.Fatalf(
			"unexpected threshold\nexpected: [%v]\nactual:   [%v]",
			2,
			result.Threshold,
		)
	}

	if result.CreatedAtUnix != 123456789 {
		t.Fatalf(
			"unexpected created-at unix\nexpected: [%v]\nactual:   [%v]",
			123456789,
			result.CreatedAtUnix,
		)
	}
}

func TestBuildTaggedTBTCSignerStartSignRoundRequestPayload(t *testing.T) {
	payload, err := buildTaggedTBTCSignerStartSignRoundRequestPayload(
		"session-1",
		3,
		[]byte{0xab, 0xcd},
		"key-group-1",
		[]uint16{1, 2, 3},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerStartSignRoundRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.SessionID != "session-1" {
		t.Fatalf(
			"unexpected session id\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			request.SessionID,
		)
	}

	if request.MessageHex != "abcd" {
		t.Fatalf(
			"unexpected message hex\nexpected: [%v]\nactual:   [%v]",
			"abcd",
			request.MessageHex,
		)
	}

	if request.KeyGroup != "key-group-1" {
		t.Fatalf(
			"unexpected key group\nexpected: [%v]\nactual:   [%v]",
			"key-group-1",
			request.KeyGroup,
		)
	}

	if request.MemberIdentifier != 3 {
		t.Fatalf(
			"unexpected member identifier\nexpected: [%v]\nactual:   [%v]",
			3,
			request.MemberIdentifier,
		)
	}

	if len(request.SigningParticipants) != 3 {
		t.Fatalf(
			"unexpected signing participants count\nexpected: [%v]\nactual:   [%v]",
			3,
			len(request.SigningParticipants),
		)
	}

	expectedSigningParticipants := []uint16{1, 2, 3}
	for i := range expectedSigningParticipants {
		if request.SigningParticipants[i] != expectedSigningParticipants[i] {
			t.Fatalf(
				"unexpected signing participant at index [%d]\nexpected: [%v]\nactual:   [%v]",
				i,
				expectedSigningParticipants[i],
				request.SigningParticipants[i],
			)
		}
	}
}

func TestBuildTaggedTBTCSignerStartSignRoundRequestPayload_TaprootMerkleRoot(
	t *testing.T,
) {
	var taprootMerkleRoot [32]byte
	taprootMerkleRoot[0] = 0xab
	taprootMerkleRoot[31] = 0xcd

	payload, err := buildTaggedTBTCSignerStartSignRoundRequestPayload(
		"session-1",
		3,
		[]byte{0xab, 0xcd},
		"key-group-1",
		[]uint16{1, 2, 3},
		&taprootMerkleRoot,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerStartSignRoundRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.TaprootMerkleRootHex == nil {
		t.Fatal("expected taproot merkle root")
	}

	expectedTaprootMerkleRootHex := hex.EncodeToString(taprootMerkleRoot[:])
	if *request.TaprootMerkleRootHex != expectedTaprootMerkleRootHex {
		t.Fatalf(
			"unexpected taproot merkle root\nexpected: [%v]\nactual:   [%v]",
			expectedTaprootMerkleRootHex,
			*request.TaprootMerkleRootHex,
		)
	}
}

func TestBuildTaggedTBTCSignerStartSignRoundRequestPayload_EmptySessionID(t *testing.T) {
	_, err := buildTaggedTBTCSignerStartSignRoundRequestPayload(
		"",
		1,
		[]byte{0xab},
		"key-group-1",
		nil,
		nil,
	)
	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf(
			"expected native bridge operation failed error: [%v], got [%v]",
			ErrNativeBridgeOperationFailed,
			err,
		)
	}

	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"did not expect native cryptography unavailable error: [%v]",
			err,
		)
	}
}

func TestBuildTaggedTBTCSignerStartSignRoundRequestPayload_ZeroMemberID(t *testing.T) {
	_, err := buildTaggedTBTCSignerStartSignRoundRequestPayload(
		"session-1",
		0,
		[]byte{0xab},
		"key-group-1",
		nil,
		nil,
	)
	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf(
			"expected native bridge operation failed error: [%v], got [%v]",
			ErrNativeBridgeOperationFailed,
			err,
		)
	}

	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"did not expect native cryptography unavailable error: [%v]",
			err,
		)
	}
}

func TestBuildTaggedTBTCSignerFinalizeSignRoundRequestPayload(t *testing.T) {
	payload, err := buildTaggedTBTCSignerFinalizeSignRoundRequestPayload(
		"session-1",
		[]NativeTBTCSignerRoundContribution{
			{
				Identifier: 7,
				Data:       []byte{0xde, 0xad},
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerFinalizeSignRoundRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.SessionID != "session-1" {
		t.Fatalf(
			"unexpected session id\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			request.SessionID,
		)
	}

	if len(request.RoundContributions) != 1 {
		t.Fatalf(
			"unexpected contribution count\nexpected: [%v]\nactual:   [%v]",
			1,
			len(request.RoundContributions),
		)
	}

	if request.RoundContributions[0].Identifier != 7 {
		t.Fatalf(
			"unexpected contribution identifier\nexpected: [%v]\nactual:   [%v]",
			7,
			request.RoundContributions[0].Identifier,
		)
	}

	if request.RoundContributions[0].SignatureShareHex != "dead" {
		t.Fatalf(
			"unexpected contribution signature share\nexpected: [%v]\nactual:   [%v]",
			"dead",
			request.RoundContributions[0].SignatureShareHex,
		)
	}
}

func TestBuildTaggedTBTCSignerFinalizeSignRoundRequestPayload_TaprootMerkleRoot(
	t *testing.T,
) {
	var taprootMerkleRoot [32]byte
	taprootMerkleRoot[0] = 0xab
	taprootMerkleRoot[31] = 0xcd

	payload, err := buildTaggedTBTCSignerFinalizeSignRoundRequestPayload(
		"session-1",
		[]NativeTBTCSignerRoundContribution{
			{
				Identifier: 7,
				Data:       []byte{0xde, 0xad},
			},
		},
		&taprootMerkleRoot,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerFinalizeSignRoundRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.TaprootMerkleRootHex == nil {
		t.Fatal("expected taproot merkle root")
	}

	expectedTaprootMerkleRootHex := hex.EncodeToString(taprootMerkleRoot[:])
	if *request.TaprootMerkleRootHex != expectedTaprootMerkleRootHex {
		t.Fatalf(
			"unexpected taproot merkle root\nexpected: [%v]\nactual:   [%v]",
			expectedTaprootMerkleRootHex,
			*request.TaprootMerkleRootHex,
		)
	}
}

func TestDecodeBuildTaggedTBTCSignerStartSignRoundResponse(t *testing.T) {
	roundState, err := decodeBuildTaggedTBTCSignerStartSignRoundResponse(
		[]byte(
			`{"session_id":"session-1","round_id":"round-1","required_contributions":2,"message_digest_hex":"abcd","signing_participants":[1,2,3],"own_contribution":{"identifier":3,"signature_share_hex":"deadbeef"}}`,
		),
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}

	if roundState.SessionID != "session-1" {
		t.Fatalf(
			"unexpected session id\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			roundState.SessionID,
		)
	}

	if roundState.RoundID != "round-1" {
		t.Fatalf(
			"unexpected round id\nexpected: [%v]\nactual:   [%v]",
			"round-1",
			roundState.RoundID,
		)
	}

	if roundState.RequiredContributions != 2 {
		t.Fatalf(
			"unexpected required contributions\nexpected: [%v]\nactual:   [%v]",
			2,
			roundState.RequiredContributions,
		)
	}

	if roundState.MessageDigestHex != "abcd" {
		t.Fatalf(
			"unexpected message digest hex\nexpected: [%v]\nactual:   [%v]",
			"abcd",
			roundState.MessageDigestHex,
		)
	}

	if len(roundState.SigningParticipants) != 3 {
		t.Fatalf(
			"unexpected signing participants count\nexpected: [%v]\nactual:   [%v]",
			3,
			len(roundState.SigningParticipants),
		)
	}

	if roundState.OwnContribution == nil {
		t.Fatal("expected own contribution in round state response")
	}

	if roundState.OwnContribution.Identifier != 3 {
		t.Fatalf(
			"unexpected own contribution identifier\nexpected: [%v]\nactual:   [%v]",
			3,
			roundState.OwnContribution.Identifier,
		)
	}

	expectedOwnContributionData := []byte{0xde, 0xad, 0xbe, 0xef}
	if len(roundState.OwnContribution.Data) != len(expectedOwnContributionData) {
		t.Fatalf(
			"unexpected own contribution data length\nexpected: [%v]\nactual:   [%v]",
			len(expectedOwnContributionData),
			len(roundState.OwnContribution.Data),
		)
	}

	for i := range roundState.OwnContribution.Data {
		if roundState.OwnContribution.Data[i] != expectedOwnContributionData[i] {
			t.Fatalf(
				"unexpected own contribution byte at index [%d]\nexpected: [%x]\nactual:   [%x]",
				i,
				expectedOwnContributionData[i],
				roundState.OwnContribution.Data[i],
			)
		}
	}
}

func TestDecodeBuildTaggedTBTCSignerStartSignRoundResponse_RejectsZeroSigningParticipant(
	t *testing.T,
) {
	_, err := decodeBuildTaggedTBTCSignerStartSignRoundResponse(
		[]byte(
			`{"session_id":"session-1","round_id":"round-1","required_contributions":2,"message_digest_hex":"abcd","signing_participants":[1,0,3],"own_contribution":{"identifier":3,"signature_share_hex":"deadbeef"}}`,
		),
	)
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

	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"did not expect native cryptography unavailable error: [%v]",
			err,
		)
	}
}

func TestDecodeBuildTaggedTBTCSignerStartSignRoundResponse_RejectsDuplicateSigningParticipant(
	t *testing.T,
) {
	_, err := decodeBuildTaggedTBTCSignerStartSignRoundResponse(
		[]byte(
			`{"session_id":"session-1","round_id":"round-1","required_contributions":2,"message_digest_hex":"abcd","signing_participants":[1,2,2],"own_contribution":{"identifier":3,"signature_share_hex":"deadbeef"}}`,
		),
	)
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

	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"did not expect native cryptography unavailable error: [%v]",
			err,
		)
	}
}

func TestDecodeBuildTaggedTBTCSignerStartSignRoundResponse_RejectsZeroOwnContributionIdentifier(
	t *testing.T,
) {
	_, err := decodeBuildTaggedTBTCSignerStartSignRoundResponse(
		[]byte(
			`{"session_id":"session-1","round_id":"round-1","required_contributions":2,"message_digest_hex":"abcd","signing_participants":[1,2,3],"own_contribution":{"identifier":0,"signature_share_hex":"deadbeef"}}`,
		),
	)
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

	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"did not expect native cryptography unavailable error: [%v]",
			err,
		)
	}
}

func TestDecodeBuildTaggedTBTCSignerFinalizeSignRoundResponse(t *testing.T) {
	signature, err := decodeBuildTaggedTBTCSignerFinalizeSignRoundResponse(
		[]byte(`{"session_id":"session-1","round_id":"round-1","signature_hex":"deadbeef"}`),
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}

	expectedSignature := []byte{0xde, 0xad, 0xbe, 0xef}
	if len(signature) != len(expectedSignature) {
		t.Fatalf(
			"unexpected signature length\nexpected: [%v]\nactual:   [%v]",
			len(expectedSignature),
			len(signature),
		)
	}

	for i := range signature {
		if signature[i] != expectedSignature[i] {
			t.Fatalf(
				"unexpected signature byte at index [%d]\nexpected: [%x]\nactual:   [%x]",
				i,
				expectedSignature[i],
				signature[i],
			)
		}
	}
}

func TestBuildTaggedTBTCSignerBuildTaprootTxRequestPayload(t *testing.T) {
	scriptTreeHex := "deadbeef"

	payload, err := buildTaggedTBTCSignerBuildTaprootTxRequestPayload(
		"session-buildtx-1",
		[]NativeTBTCSignerTxInput{
			{
				TxIDHex:   strings.Repeat("11", 32),
				Vout:      3,
				ValueSats: 1000,
			},
		},
		[]NativeTBTCSignerTxOutput{
			{
				ScriptPubKeyHex: "0014deadbeef",
				ValueSats:       900,
			},
		},
		&scriptTreeHex,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerBuildTaprootTxRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.SessionID != "session-buildtx-1" {
		t.Fatalf(
			"unexpected session id\nexpected: [%v]\nactual:   [%v]",
			"session-buildtx-1",
			request.SessionID,
		)
	}

	if len(request.Inputs) != 1 {
		t.Fatalf(
			"unexpected input count\nexpected: [%v]\nactual:   [%v]",
			1,
			len(request.Inputs),
		)
	}

	if request.Inputs[0].TxIDHex != strings.Repeat("11", 32) {
		t.Fatalf(
			"unexpected input txid\nexpected: [%v]\nactual:   [%v]",
			strings.Repeat("11", 32),
			request.Inputs[0].TxIDHex,
		)
	}

	if len(request.Outputs) != 1 {
		t.Fatalf(
			"unexpected output count\nexpected: [%v]\nactual:   [%v]",
			1,
			len(request.Outputs),
		)
	}

	if request.Outputs[0].ScriptPubKeyHex != "0014deadbeef" {
		t.Fatalf(
			"unexpected output script pubkey\nexpected: [%v]\nactual:   [%v]",
			"0014deadbeef",
			request.Outputs[0].ScriptPubKeyHex,
		)
	}

	if request.ScriptTreeHex == nil || *request.ScriptTreeHex != scriptTreeHex {
		t.Fatal("expected script tree hex to be present and preserved")
	}
}

func TestBuildTaggedTBTCSignerBuildTaprootTxRequestPayload_RejectsInvalidInput(
	t *testing.T,
) {
	scriptTreeHex := ""

	testCases := []struct {
		name          string
		sessionID     string
		inputs        []NativeTBTCSignerTxInput
		outputs       []NativeTBTCSignerTxOutput
		scriptTreeHex *string
	}{
		{
			name:      "empty session id",
			sessionID: "",
			inputs: []NativeTBTCSignerTxInput{
				{TxIDHex: strings.Repeat("11", 32), Vout: 0, ValueSats: 1},
			},
			outputs: []NativeTBTCSignerTxOutput{
				{ScriptPubKeyHex: "0014aa", ValueSats: 1},
			},
		},
		{
			name:      "empty inputs",
			sessionID: "session-1",
			inputs:    nil,
			outputs: []NativeTBTCSignerTxOutput{
				{ScriptPubKeyHex: "0014aa", ValueSats: 1},
			},
		},
		{
			name:      "empty outputs",
			sessionID: "session-1",
			inputs: []NativeTBTCSignerTxInput{
				{TxIDHex: strings.Repeat("11", 32), Vout: 0, ValueSats: 1},
			},
			outputs: nil,
		},
		{
			name:      "input txid empty",
			sessionID: "session-1",
			inputs: []NativeTBTCSignerTxInput{
				{TxIDHex: "", Vout: 0, ValueSats: 1},
			},
			outputs: []NativeTBTCSignerTxOutput{
				{ScriptPubKeyHex: "0014aa", ValueSats: 1},
			},
		},
		{
			name:      "output script empty",
			sessionID: "session-1",
			inputs: []NativeTBTCSignerTxInput{
				{TxIDHex: strings.Repeat("11", 32), Vout: 0, ValueSats: 1},
			},
			outputs: []NativeTBTCSignerTxOutput{
				{ScriptPubKeyHex: "", ValueSats: 1},
			},
		},
		{
			name:      "script tree empty string",
			sessionID: "session-1",
			inputs: []NativeTBTCSignerTxInput{
				{TxIDHex: strings.Repeat("11", 32), Vout: 0, ValueSats: 1},
			},
			outputs: []NativeTBTCSignerTxOutput{
				{ScriptPubKeyHex: "0014aa", ValueSats: 1},
			},
			scriptTreeHex: &scriptTreeHex,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildTaggedTBTCSignerBuildTaprootTxRequestPayload(
				tc.sessionID,
				tc.inputs,
				tc.outputs,
				tc.scriptTreeHex,
			)
			if err == nil {
				t.Fatal("expected payload build error")
			}

			if !errors.Is(err, ErrNativeBridgeOperationFailed) {
				t.Fatalf(
					"expected native bridge operation failed error: [%v], got [%v]",
					ErrNativeBridgeOperationFailed,
					err,
				)
			}

			if errors.Is(err, ErrNativeCryptographyUnavailable) {
				t.Fatalf(
					"did not expect native cryptography unavailable error: [%v]",
					err,
				)
			}
		})
	}
}

func TestDecodeBuildTaggedTBTCSignerBuildTaprootTxResponse(t *testing.T) {
	result, err := decodeBuildTaggedTBTCSignerBuildTaprootTxResponse(
		[]byte(`{"session_id":"session-buildtx-1","tx_hex":"deadbeef"}`),
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}

	if result.SessionID != "session-buildtx-1" {
		t.Fatalf(
			"unexpected session id\nexpected: [%v]\nactual:   [%v]",
			"session-buildtx-1",
			result.SessionID,
		)
	}

	if result.TxHex != "deadbeef" {
		t.Fatalf(
			"unexpected tx hex\nexpected: [%v]\nactual:   [%v]",
			"deadbeef",
			result.TxHex,
		)
	}
}

func TestBuildTaggedTBTCSignerVerifySignatureShareRequestPayload(t *testing.T) {
	payload, err := buildTaggedTBTCSignerVerifySignatureShareRequestPayload(
		"session-1",
		[]byte{0xde, 0xad},
		[]byte{0xbe, 0xef},
		2,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerVerifySignatureShareRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.SessionID != "session-1" {
		t.Fatalf(
			"unexpected session id\nexpected: [%v]\nactual:   [%v]",
			"session-1",
			request.SessionID,
		)
	}
	if request.SigningPackageHex != "dead" {
		t.Fatalf(
			"unexpected signing package hex\nexpected: [%v]\nactual:   [%v]",
			"dead",
			request.SigningPackageHex,
		)
	}
	if request.SignatureShareHex != "beef" {
		t.Fatalf(
			"unexpected signature share hex\nexpected: [%v]\nactual:   [%v]",
			"beef",
			request.SignatureShareHex,
		)
	}
	if request.MemberIdentifier != 2 {
		t.Fatalf(
			"unexpected member identifier\nexpected: [%v]\nactual:   [%v]",
			2,
			request.MemberIdentifier,
		)
	}
	if request.TaprootMerkleRootHex != nil {
		t.Fatalf(
			"expected omitted taproot merkle root, got: [%v]",
			*request.TaprootMerkleRootHex,
		)
	}
}

func TestBuildTaggedTBTCSignerVerifySignatureShareRequestPayload_TaprootMerkleRoot(
	t *testing.T,
) {
	var taprootMerkleRoot [32]byte
	taprootMerkleRoot[0] = 0xab
	taprootMerkleRoot[31] = 0xcd

	payload, err := buildTaggedTBTCSignerVerifySignatureShareRequestPayload(
		"session-1",
		[]byte{0xde, 0xad},
		[]byte{0xbe, 0xef},
		2,
		&taprootMerkleRoot,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerVerifySignatureShareRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.TaprootMerkleRootHex == nil {
		t.Fatal("expected taproot merkle root")
	}
	expectedTaprootMerkleRootHex := hex.EncodeToString(taprootMerkleRoot[:])
	if *request.TaprootMerkleRootHex != expectedTaprootMerkleRootHex {
		t.Fatalf(
			"unexpected taproot merkle root hex\nexpected: [%v]\nactual:   [%v]",
			expectedTaprootMerkleRootHex,
			*request.TaprootMerkleRootHex,
		)
	}
}

func TestBuildTaggedTBTCSignerVerifySignatureShareRequestPayload_EmptySessionID(
	t *testing.T,
) {
	_, err := buildTaggedTBTCSignerVerifySignatureShareRequestPayload(
		"",
		[]byte{0xde, 0xad},
		[]byte{0xbe, 0xef},
		2,
		nil,
	)
	if err == nil {
		t.Fatal("expected an empty session id to be rejected")
	}
	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf("expected ErrNativeBridgeOperationFailed, got: [%v]", err)
	}
}

// The verify-share request builder must NOT reject empty/short package or share
// bytes: those are the SUBJECT of the engine's verdict. A member who submits
// empty/garbage inner FROST bytes must reach the engine (which returns
// `invalid` -> blame); a bridge-side rejection would surface as an FFI error the
// Go host maps to ShareIndeterminate, letting the cheater dodge blame. This test
// pins that the builder passes such bytes through as empty/short hex.
func TestBuildTaggedTBTCSignerVerifySignatureShareRequestPayload_PassesThroughEmptyBlameSubjectBytes(
	t *testing.T,
) {
	payload, err := buildTaggedTBTCSignerVerifySignatureShareRequestPayload(
		"session-1",
		nil,
		nil,
		2,
		nil,
	)
	if err != nil {
		t.Fatalf("empty package/share bytes must pass through, got error: [%v]", err)
	}

	var request buildTaggedTBTCSignerVerifySignatureShareRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if request.SigningPackageHex != "" {
		t.Fatalf("expected empty signing package hex, got: [%q]", request.SigningPackageHex)
	}
	if request.SignatureShareHex != "" {
		t.Fatalf("expected empty signature share hex, got: [%q]", request.SignatureShareHex)
	}
}

func TestDecodeBuildTaggedTBTCSignerVerifySignatureShareResponse(t *testing.T) {
	tests := map[string]struct {
		verdict  string
		expected NativeShareVerificationVerdict
	}{
		"valid":         {verdict: "valid", expected: NativeShareVerdictValid},
		"invalid":       {verdict: "invalid", expected: NativeShareVerdictInvalid},
		"indeterminate": {verdict: "indeterminate", expected: NativeShareVerdictIndeterminate},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			payload := []byte(fmt.Sprintf(`{"verdict":%q}`, test.verdict))
			verdict, err := decodeBuildTaggedTBTCSignerVerifySignatureShareResponse(payload)
			if err != nil {
				t.Fatalf("unexpected decode error: [%v]", err)
			}
			if verdict != test.expected {
				t.Fatalf(
					"unexpected verdict\nexpected: [%v]\nactual:   [%v]",
					test.expected,
					verdict,
				)
			}
		})
	}
}

func TestDecodeBuildTaggedTBTCSignerVerifySignatureShareResponse_UnrecognizedVerdict(
	t *testing.T,
) {
	verdict, err := decodeBuildTaggedTBTCSignerVerifySignatureShareResponse(
		[]byte(`{"verdict":"maybe"}`),
	)
	if err == nil {
		t.Fatal("expected an unrecognized verdict to be rejected")
	}
	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf("expected ErrNativeBridgeOperationFailed, got: [%v]", err)
	}
	// An unrecognized verdict must fail closed to the safe Indeterminate, never
	// to a blame verdict, even though the caller is expected to check the error.
	if verdict != NativeShareVerdictIndeterminate {
		t.Fatalf(
			"expected Indeterminate on error\nexpected: [%v]\nactual:   [%v]",
			NativeShareVerdictIndeterminate,
			verdict,
		)
	}
}

func TestDecodeBuildTaggedTBTCSignerVerifySignatureShareResponse_MalformedJSON(
	t *testing.T,
) {
	verdict, err := decodeBuildTaggedTBTCSignerVerifySignatureShareResponse(
		[]byte("not json"),
	)
	if err == nil {
		t.Fatal("expected malformed JSON to be rejected")
	}
	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf("expected ErrNativeBridgeOperationFailed, got: [%v]", err)
	}
	// The other decoder error path (a json.Unmarshal failure) must also fail
	// closed to the safe Indeterminate, never to a blame verdict.
	if verdict != NativeShareVerdictIndeterminate {
		t.Fatalf(
			"expected Indeterminate on error\nexpected: [%v]\nactual:   [%v]",
			NativeShareVerdictIndeterminate,
			verdict,
		)
	}
}

func testInteractiveAttemptContext() NativeInteractiveAttemptContext {
	return NativeInteractiveAttemptContext{
		AttemptNumber:                   3,
		CoordinatorIdentifier:           2,
		IncludedParticipants:            []uint16{1, 2, 3},
		IncludedParticipantsFingerprint: "fingerprint-abc",
		AttemptID:                       "attempt-1",
	}
}

func TestBuildTaggedTBTCSignerInteractiveSessionOpenRequestPayload(t *testing.T) {
	payload, err := buildTaggedTBTCSignerInteractiveSessionOpenRequestPayload(
		"session-1",
		2,
		[]byte{0xab, 0xcd},
		"key-group-1",
		2,
		nil,
		testInteractiveAttemptContext(),
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerInteractiveSessionOpenRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}

	if request.SessionID != "session-1" {
		t.Fatalf("unexpected session id: [%s]", request.SessionID)
	}
	if request.MemberIdentifier != 2 {
		t.Fatalf("unexpected member id: [%d]", request.MemberIdentifier)
	}
	if request.MessageHex != "abcd" {
		t.Fatalf("unexpected message hex: [%s]", request.MessageHex)
	}
	if request.KeyGroup != "key-group-1" {
		t.Fatalf("unexpected key group: [%s]", request.KeyGroup)
	}
	if request.Threshold != 2 {
		t.Fatalf("unexpected threshold: [%d]", request.Threshold)
	}
	if request.TaprootMerkleRootHex != nil {
		t.Fatalf("expected omitted taproot root, got: [%v]", *request.TaprootMerkleRootHex)
	}
	// Wire attempt_number is 1-based: the RFC-21 0-based 3 serializes as 4.
	if request.AttemptContext.AttemptNumber != 4 ||
		request.AttemptContext.CoordinatorIdentifier != 2 ||
		request.AttemptContext.IncludedParticipantsFingerprint != "fingerprint-abc" ||
		request.AttemptContext.AttemptID != "attempt-1" {
		t.Fatalf("unexpected attempt context: [%+v]", request.AttemptContext)
	}
	if len(request.AttemptContext.IncludedParticipants) != 3 ||
		request.AttemptContext.IncludedParticipants[0] != 1 ||
		request.AttemptContext.IncludedParticipants[2] != 3 {
		t.Fatalf("unexpected included participants: [%v]", request.AttemptContext.IncludedParticipants)
	}
}

// The RFC-21 0-based first attempt (AttemptNumber 0) must serialize as the
// engine's 1-based wire attempt_number 1 - the engine rejects 0, so passing it
// through unchanged would fail InteractiveSessionOpen before round 1.
func TestBuildTaggedTBTCSignerInteractiveSessionOpenRequestPayload_FirstAttemptIsOneBasedOnWire(t *testing.T) {
	ctx := testInteractiveAttemptContext()
	ctx.AttemptNumber = 0 // RFC-21 first attempt

	payload, err := buildTaggedTBTCSignerInteractiveSessionOpenRequestPayload(
		"session-1", 2, []byte{0xab}, "key-group-1", 2, nil, ctx,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerInteractiveSessionOpenRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if request.AttemptContext.AttemptNumber != 1 {
		t.Fatalf(
			"expected RFC-21 attempt 0 to serialize as wire attempt_number 1, got [%d]",
			request.AttemptContext.AttemptNumber,
		)
	}
}

func TestBuildTaggedTBTCSignerInteractiveSessionOpenRequestPayload_TaprootMerkleRoot(t *testing.T) {
	var root [32]byte
	root[0] = 0xab
	root[31] = 0xcd

	payload, err := buildTaggedTBTCSignerInteractiveSessionOpenRequestPayload(
		"session-1", 2, []byte{0xab}, "key-group-1", 2, &root, testInteractiveAttemptContext(),
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerInteractiveSessionOpenRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if request.TaprootMerkleRootHex == nil {
		t.Fatal("expected taproot merkle root")
	}
	if *request.TaprootMerkleRootHex != hex.EncodeToString(root[:]) {
		t.Fatalf("unexpected taproot root hex: [%s]", *request.TaprootMerkleRootHex)
	}
}

func TestBuildTaggedTBTCSignerInteractiveSessionOpenRequestPayload_RejectsInvalidInput(t *testing.T) {
	ctx := testInteractiveAttemptContext()
	emptyCtx := NativeInteractiveAttemptContext{}
	noParticipantsCtx := testInteractiveAttemptContext()
	noParticipantsCtx.IncludedParticipants = nil

	tests := map[string]struct {
		sessionID string
		member    uint16
		message   []byte
		keyGroup  string
		threshold uint16
		ctx       NativeInteractiveAttemptContext
	}{
		"empty session":            {"", 2, []byte{0xab}, "kg", 2, ctx},
		"zero member":              {"s", 0, []byte{0xab}, "kg", 2, ctx},
		"empty message":            {"s", 2, nil, "kg", 2, ctx},
		"empty key group":          {"s", 2, []byte{0xab}, "", 2, ctx},
		"zero threshold":           {"s", 2, []byte{0xab}, "kg", 0, ctx},
		"empty attempt context":    {"s", 2, []byte{0xab}, "kg", 2, emptyCtx},
		"no included participants": {"s", 2, []byte{0xab}, "kg", 2, noParticipantsCtx},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := buildTaggedTBTCSignerInteractiveSessionOpenRequestPayload(
				test.sessionID, test.member, test.message, test.keyGroup, test.threshold, nil, test.ctx,
			)
			if err == nil {
				t.Fatal("expected invalid input to be rejected")
			}
			if !errors.Is(err, ErrNativeBridgeOperationFailed) {
				t.Fatalf("expected ErrNativeBridgeOperationFailed, got: [%v]", err)
			}
		})
	}
}

func TestDecodeBuildTaggedTBTCSignerInteractiveSessionOpenResponse(t *testing.T) {
	result, err := decodeBuildTaggedTBTCSignerInteractiveSessionOpenResponse(
		[]byte(`{"session_id":"session-1","attempt_id":"attempt-1","idempotent":true}`),
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}
	if result.SessionID != "session-1" || result.AttemptID != "attempt-1" || !result.Idempotent {
		t.Fatalf("unexpected result: [%+v]", result)
	}
}

func TestBuildTaggedTBTCSignerInteractiveRound1RequestPayload(t *testing.T) {
	payload, err := buildTaggedTBTCSignerInteractiveRound1RequestPayload("session-1", "attempt-1", 2)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}
	var request buildTaggedTBTCSignerInteractiveRound1Request
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if request.SessionID != "session-1" || request.AttemptID != "attempt-1" || request.MemberIdentifier != 2 {
		t.Fatalf("unexpected request: [%+v]", request)
	}

	if _, err := buildTaggedTBTCSignerInteractiveRound1RequestPayload("", "attempt-1", 2); err == nil {
		t.Fatal("expected empty session id to be rejected")
	}
}

func TestDecodeBuildTaggedTBTCSignerInteractiveRound1Response(t *testing.T) {
	commitments, err := decodeBuildTaggedTBTCSignerInteractiveRound1Response(
		[]byte(`{"commitments_hex":"dead"}`),
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}
	if hex.EncodeToString(commitments) != "dead" {
		t.Fatalf("unexpected commitments: [%x]", commitments)
	}
}

func TestBuildTaggedTBTCSignerInteractiveRound2RequestPayload(t *testing.T) {
	payload, err := buildTaggedTBTCSignerInteractiveRound2RequestPayload(
		"session-1", "attempt-1", 2, []byte{0xbe, 0xef},
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}
	var request buildTaggedTBTCSignerInteractiveRound2Request
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if request.SigningPackageHex != "beef" {
		t.Fatalf("unexpected signing package hex: [%s]", request.SigningPackageHex)
	}

	if _, err := buildTaggedTBTCSignerInteractiveRound2RequestPayload("session-1", "attempt-1", 2, nil); err == nil {
		t.Fatal("expected empty signing package to be rejected")
	}
}

func TestDecodeBuildTaggedTBTCSignerInteractiveRound2Response(t *testing.T) {
	share, err := decodeBuildTaggedTBTCSignerInteractiveRound2Response(
		[]byte(`{"session_id":"s","attempt_id":"a","signature_share_hex":"cafe"}`),
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}
	if hex.EncodeToString(share) != "cafe" {
		t.Fatalf("unexpected signature share: [%x]", share)
	}
}

func TestBuildTaggedTBTCSignerInteractiveSessionAbortRequestPayload(t *testing.T) {
	attemptID := "attempt-1"
	payload, err := buildTaggedTBTCSignerInteractiveSessionAbortRequestPayload("session-1", &attemptID)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}
	var request buildTaggedTBTCSignerInteractiveSessionAbortRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if request.AttemptID == nil || *request.AttemptID != "attempt-1" {
		t.Fatalf("unexpected attempt id: [%v]", request.AttemptID)
	}

	// A nil attempt id (abort whatever is live) is valid and omitted on the wire.
	payloadNil, err := buildTaggedTBTCSignerInteractiveSessionAbortRequestPayload("session-1", nil)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}
	var requestNil buildTaggedTBTCSignerInteractiveSessionAbortRequest
	if err := json.Unmarshal(payloadNil, &requestNil); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if requestNil.AttemptID != nil {
		t.Fatalf("expected omitted attempt id, got: [%v]", *requestNil.AttemptID)
	}

	if _, err := buildTaggedTBTCSignerInteractiveSessionAbortRequestPayload("", nil); err == nil {
		t.Fatal("expected empty session id to be rejected")
	}
}

func TestDecodeBuildTaggedTBTCSignerInteractiveSessionAbortResponse(t *testing.T) {
	result, err := decodeBuildTaggedTBTCSignerInteractiveSessionAbortResponse(
		[]byte(`{"session_id":"session-1","aborted":true}`),
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}
	if result.SessionID != "session-1" || !result.Aborted {
		t.Fatalf("unexpected result: [%+v]", result)
	}
}

// Every interactive decoder must reject a malformed payload rather than return a
// zero-valued result, and the open decoder must also reject a structurally
// valid response missing the session/attempt ids.
func TestDecodeBuildTaggedTBTCSignerInteractiveResponses_RejectMalformed(t *testing.T) {
	malformed := []byte("not json")

	if _, err := decodeBuildTaggedTBTCSignerInteractiveSessionOpenResponse(malformed); err == nil {
		t.Fatal("open: expected malformed JSON to be rejected")
	}
	if _, err := decodeBuildTaggedTBTCSignerInteractiveRound1Response(malformed); err == nil {
		t.Fatal("round1: expected malformed JSON to be rejected")
	}
	if _, err := decodeBuildTaggedTBTCSignerInteractiveRound2Response(malformed); err == nil {
		t.Fatal("round2: expected malformed JSON to be rejected")
	}
	if _, err := decodeBuildTaggedTBTCSignerInteractiveSessionAbortResponse(malformed); err == nil {
		t.Fatal("abort: expected malformed JSON to be rejected")
	}

	if _, err := decodeBuildTaggedTBTCSignerInteractiveSessionOpenResponse([]byte(`{}`)); err == nil {
		t.Fatal("open: expected a response missing session/attempt ids to be rejected")
	}
}
