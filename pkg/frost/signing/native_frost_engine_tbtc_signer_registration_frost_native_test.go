//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRegisterBuildTaggedTBTCSignerEngine(t *testing.T) {
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(func() {
		UnregisterNativeTBTCSignerEngine()
	})

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

func TestBuildTaggedTBTCSignerStartSignRoundRequestPayload_EmptySessionID(t *testing.T) {
	_, err := buildTaggedTBTCSignerStartSignRoundRequestPayload(
		"",
		1,
		[]byte{0xab},
		"key-group-1",
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
