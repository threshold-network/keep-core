//go:build frost_native && frost_tbtc_signer && cgo && !frost_uniffi_sdk

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
		[]byte("message"),
		"key-group",
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
}

func TestBuildTaggedTBTCSignerStartSignRoundRequestPayload(t *testing.T) {
	payload, err := buildTaggedTBTCSignerStartSignRoundRequestPayload(
		"session-1",
		[]byte{0xab, 0xcd},
		"key-group-1",
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
}

func TestBuildTaggedTBTCSignerStartSignRoundRequestPayload_EmptySessionID(t *testing.T) {
	_, err := buildTaggedTBTCSignerStartSignRoundRequestPayload(
		"",
		[]byte{0xab},
		"key-group-1",
	)
	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"expected native cryptography unavailable error: [%v], got [%v]",
			ErrNativeCryptographyUnavailable,
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
			`{"session_id":"session-1","round_id":"round-1","required_contributions":2,"message_digest_hex":"abcd"}`,
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
