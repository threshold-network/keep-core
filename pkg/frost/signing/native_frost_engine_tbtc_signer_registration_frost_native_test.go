//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRegisterBuildTaggedTBTCSignerEngine(t *testing.T) {
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)
	t.Cleanup(ResetInteractiveSigningEngineProviderForTest)

	err := registerBuildTaggedNativeFROSTSigningEngine()
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	engine := currentNativeTBTCSignerEngine()
	if engine == nil {
		t.Fatal("expected native tbtc-signer engine registration")
	}

	// RFC-21 Phase 7.3: the same registration installs the cgo engine as the
	// interactive signing provider, so the executor can obtain a real engine for
	// the gated interactive ROAST path. The provider is a factory returning a
	// fresh cgo bridge handle; the path itself stays dormant behind the default-off
	// KEEP_CORE_FROST_INTERACTIVE_SIGNING_ENABLED opt-in.
	interactive := registeredInteractiveSigningEngine()
	if interactive == nil {
		t.Fatal("expected the interactive signing provider to be registered")
	}
	if _, ok := interactive.(*buildTaggedTBTCSignerEngine); !ok {
		t.Fatalf(
			"interactive provider returned %T, want *buildTaggedTBTCSignerEngine",
			interactive,
		)
	}

	// The fail-closed contract asserted below - every engine operation returns
	// ErrNativeCryptographyUnavailable - only holds when libfrost_tbtc is NOT linked:
	// the cgo bridge is compiled (this file's build tag) but the frost_tbtc_* symbols
	// are not resolvable via dlsym. Under the frost-cgo-integration gate the lib IS
	// linked, so native crypto is available and BuildTaprootTx instead reaches the real
	// signer; asserting an unavailable error there would be a
	// false failure. Probe the linked lib with the same check the ABI preflight uses -
	// it keeps ErrNativeCryptographyUnavailable in the chain iff the lib is absent - and
	// skip the fail-closed assertions with a reason when the lib is present. The
	// registration wiring above still runs under both builds, and the linked-lib crypto
	// path is covered by the TestRealCgoInteractiveSigning* suite.
	if abiErr := assertTBTCSignerABICompatible(); !errors.Is(
		abiErr, ErrNativeCryptographyUnavailable,
	) {
		t.Skipf(
			"libfrost_tbtc linked (native crypto available; ABI probe: %v); the "+
				"fail-closed-unavailable path this test asserts is not exercisable with "+
				"the lib present",
			abiErr,
		)
	}

	_, err = engine.BuildTaprootTx(
		"session-1",
		[]NativeTBTCSignerTxInput{
			{
				TxIDHex:         "11",
				Vout:            0,
				ValueSats:       1,
				ScriptPubKeyHex: "5120" + strings.Repeat("22", 32),
			},
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

func TestBuildTaggedTBTCSignerBuildTaprootTxRequestPayload(t *testing.T) {
	scriptTreeHex := "deadbeef"

	payload, err := buildTaggedTBTCSignerBuildTaprootTxRequestPayload(
		"session-buildtx-1",
		[]NativeTBTCSignerTxInput{
			{
				TxIDHex:         strings.Repeat("11", 32),
				Vout:            3,
				ValueSats:       1000,
				ScriptPubKeyHex: "5120" + strings.Repeat("22", 32),
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
	if request.Inputs[0].ScriptPubKeyHex != "5120"+strings.Repeat("22", 32) {
		t.Fatalf(
			"unexpected input script pubkey\nexpected: [%v]\nactual:   [%v]",
			"5120"+strings.Repeat("22", 32),
			request.Inputs[0].ScriptPubKeyHex,
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
				{TxIDHex: strings.Repeat("11", 32), Vout: 0, ValueSats: 1, ScriptPubKeyHex: "5120" + strings.Repeat("22", 32)},
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
				{TxIDHex: strings.Repeat("11", 32), Vout: 0, ValueSats: 1, ScriptPubKeyHex: "5120" + strings.Repeat("22", 32)},
			},
			outputs: nil,
		},
		{
			name:      "input txid empty",
			sessionID: "session-1",
			inputs: []NativeTBTCSignerTxInput{
				{TxIDHex: "", Vout: 0, ValueSats: 1, ScriptPubKeyHex: "5120" + strings.Repeat("22", 32)},
			},
			outputs: []NativeTBTCSignerTxOutput{
				{ScriptPubKeyHex: "0014aa", ValueSats: 1},
			},
		},
		{
			name:      "input script empty",
			sessionID: "session-1",
			inputs: []NativeTBTCSignerTxInput{
				{TxIDHex: strings.Repeat("11", 32), Vout: 0, ValueSats: 1},
			},
			outputs: []NativeTBTCSignerTxOutput{
				{ScriptPubKeyHex: "0014aa", ValueSats: 1},
			},
		},
		{
			name:      "output script empty",
			sessionID: "session-1",
			inputs: []NativeTBTCSignerTxInput{
				{TxIDHex: strings.Repeat("11", 32), Vout: 0, ValueSats: 1, ScriptPubKeyHex: "5120" + strings.Repeat("22", 32)},
			},
			outputs: []NativeTBTCSignerTxOutput{
				{ScriptPubKeyHex: "", ValueSats: 1},
			},
		},
		{
			name:      "script tree empty string",
			sessionID: "session-1",
			inputs: []NativeTBTCSignerTxInput{
				{TxIDHex: strings.Repeat("11", 32), Vout: 0, ValueSats: 1, ScriptPubKeyHex: "5120" + strings.Repeat("22", 32)},
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
		[]byte(`{"session_id":"session-buildtx-1","tx_hex":"deadbeef","taproot_key_spend_sighashes_hex":["1111111111111111111111111111111111111111111111111111111111111111"]}`),
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
	if len(result.TaprootKeySpendSighashesHex) != 1 ||
		result.TaprootKeySpendSighashesHex[0] != strings.Repeat("11", 32) {
		t.Fatalf(
			"unexpected taproot key-spend sighashes: [%v]",
			result.TaprootKeySpendSighashesHex,
		)
	}
}

func TestDecodeBuildTaggedTBTCSignerBuildTaprootTxResponseRejectsMissingOrInvalidSighash(
	t *testing.T,
) {
	for name, payload := range map[string]string{
		"missing": `{"session_id":"session-buildtx-1","tx_hex":"deadbeef"}`,
		"short":   `{"session_id":"session-buildtx-1","tx_hex":"deadbeef","taproot_key_spend_sighashes_hex":["11"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := decodeBuildTaggedTBTCSignerBuildTaprootTxResponse([]byte(payload))
			if err == nil {
				t.Fatal("missing or malformed BIP-341 sighash must be rejected")
			}
			if !errors.Is(err, ErrNativeBridgeOperationFailed) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
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
	if request.SigningIntent != nil {
		t.Fatalf("generic signing must omit signing_intent, got: [%+v]", request.SigningIntent)
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
		"session-1", 2, []byte{0xab}, "key-group-1", 2, nil, nil, ctx,
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
		"session-1", 2, []byte{0xab}, "key-group-1", 2, &root, nil, testInteractiveAttemptContext(),
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

func TestBuildTaggedTBTCSignerInteractiveSessionOpenRequestPayload_HeartbeatIntent(t *testing.T) {
	heartbeatMessage := [16]byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	}
	payload, err := buildTaggedTBTCSignerInteractiveSessionOpenRequestPayload(
		"session-1",
		2,
		[]byte{0xab},
		"key-group-1",
		2,
		nil,
		NewHeartbeatSigningIntent(heartbeatMessage),
		testInteractiveAttemptContext(),
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerInteractiveSessionOpenRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if request.SigningIntent == nil {
		t.Fatal("heartbeat signing must include signing_intent")
	}
	if request.SigningIntent.Type != "heartbeat" {
		t.Fatalf("unexpected signing intent type: [%s]", request.SigningIntent.Type)
	}
	wantMessageHex := "ffffffffffffffff0001020304050607"
	if request.SigningIntent.MessageHex != wantMessageHex {
		t.Fatalf(
			"unexpected heartbeat intent message: got [%s], want [%s]",
			request.SigningIntent.MessageHex,
			wantMessageHex,
		)
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
				test.sessionID, test.member, test.message, test.keyGroup, test.threshold, nil, nil, test.ctx,
			)
			if err == nil {
				t.Fatal("expected invalid input to be rejected")
			}
			if !errors.Is(err, ErrNativeBridgeOperationFailed) {
				t.Fatalf("expected ErrNativeBridgeOperationFailed, got: [%v]", err)
			}
		})
	}

	_, err := buildTaggedTBTCSignerInteractiveSessionOpenRequestPayload(
		"s", 2, []byte{0xab}, "kg", 2, nil, &SigningIntent{}, ctx,
	)
	if err == nil {
		t.Fatal("an unsupported signing intent must fail closed")
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

func TestBuildTaggedTBTCSignerInteractiveAggregateRequestPayload(t *testing.T) {
	shares := []nativeFROSTSignatureShare{
		{Identifier: "id-1", Data: []byte{0xaa}},
		{Identifier: "id-2", Data: []byte{0xbb}},
	}

	payload, err := buildTaggedTBTCSignerInteractiveAggregateRequestPayload(
		"session-1", "attempt-1", []byte{0xde, 0xad}, shares, nil,
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerInteractiveAggregateRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if request.SessionID != "session-1" || request.AttemptID != "attempt-1" {
		t.Fatalf("unexpected session/attempt: [%+v]", request)
	}
	if request.SigningPackageHex != "dead" {
		t.Fatalf("unexpected signing package hex: [%s]", request.SigningPackageHex)
	}
	if len(request.SignatureShares) != 2 ||
		request.SignatureShares[0].Identifier != "id-1" ||
		request.SignatureShares[0].DataHex != "aa" ||
		request.SignatureShares[1].DataHex != "bb" {
		t.Fatalf("unexpected signature shares: [%+v]", request.SignatureShares)
	}

	rejections := map[string]struct {
		sessionID string
		attemptID string
		pkg       []byte
		shares    []nativeFROSTSignatureShare
	}{
		"empty session":         {"", "a", []byte{0x1}, shares},
		"empty attempt":         {"s", "", []byte{0x1}, shares},
		"empty signing package": {"s", "a", nil, shares},
		"no shares":             {"s", "a", []byte{0x1}, nil},
		"share missing data":    {"s", "a", []byte{0x1}, []nativeFROSTSignatureShare{{Identifier: "id-1"}}},
	}
	for name, r := range rejections {
		t.Run(name, func(t *testing.T) {
			if _, err := buildTaggedTBTCSignerInteractiveAggregateRequestPayload(
				r.sessionID, r.attemptID, r.pkg, r.shares, nil,
			); err == nil {
				t.Fatal("expected invalid input to be rejected")
			}
		})
	}
}

func TestDecodeBuildTaggedTBTCSignerInteractiveAggregateResponse(t *testing.T) {
	signature, err := decodeBuildTaggedTBTCSignerInteractiveAggregateResponse(
		[]byte(`{"session_id":"s","attempt_id":"a","signature_hex":"cafe"}`),
	)
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}
	if hex.EncodeToString(signature) != "cafe" {
		t.Fatalf("unexpected signature: [%x]", signature)
	}

	if _, err := decodeBuildTaggedTBTCSignerInteractiveAggregateResponse([]byte("not json")); err == nil {
		t.Fatal("expected malformed JSON to be rejected")
	}
}

// The aggregate_share_verification_failed error must surface as the typed error
// carrying the candidate culprits (so the Go host can adjudicate envelope-bound
// blame over them), with the session/attempt filled from the caller's request.
func TestInterpretInteractiveAggregateError_ShareVerificationFailure(t *testing.T) {
	structured := &buildTaggedTBTCSignerStructuredError{
		Code:              interactiveAggregateShareVerificationFailedCode,
		Message:           "shares failed verification",
		CandidateCulprits: []uint16{2, 3},
	}
	// Wrap exactly as the bridge call helper does (double %w).
	wrapped := fmt.Errorf(
		"%w: tbtc-signer bridge operation [InteractiveAggregate] failed: [%w]",
		ErrNativeBridgeOperationFailed,
		structured,
	)

	err := interpretInteractiveAggregateError("session-1", "attempt-1", wrapped)

	var aggErr *InteractiveAggregateShareVerificationError
	if !errors.As(err, &aggErr) {
		t.Fatalf("expected InteractiveAggregateShareVerificationError, got: [%v]", err)
	}
	if aggErr.SessionID != "session-1" || aggErr.AttemptID != "attempt-1" {
		t.Fatalf("unexpected session/attempt: [%+v]", aggErr)
	}
	if len(aggErr.CandidateCulprits) != 2 ||
		aggErr.CandidateCulprits[0] != 2 ||
		aggErr.CandidateCulprits[1] != 3 {
		t.Fatalf("unexpected candidate culprits: [%v]", aggErr.CandidateCulprits)
	}
}

func TestInterpretInteractiveAggregateError_OtherErrorPassesThrough(t *testing.T) {
	structured := &buildTaggedTBTCSignerStructuredError{Code: "some_other_error", Message: "boom"}
	wrapped := fmt.Errorf(
		"%w: tbtc-signer bridge operation [InteractiveAggregate] failed: [%w]",
		ErrNativeBridgeOperationFailed,
		structured,
	)

	err := interpretInteractiveAggregateError("s", "a", wrapped)

	var aggErr *InteractiveAggregateShareVerificationError
	if errors.As(err, &aggErr) {
		t.Fatal("a non-share-verification error must not become the typed culprit error")
	}
	if !errors.Is(err, ErrNativeBridgeOperationFailed) {
		t.Fatalf("expected the original wrapped error to pass through, got: [%v]", err)
	}
}

func TestBuildTaggedTBTCSignerErrorPayload_CandidateCulprits(t *testing.T) {
	structured := buildTaggedTBTCSignerErrorPayload([]byte(
		`{"code":"aggregate_share_verification_failed","message":"x","candidate_culprits":[2,3]}`,
	))
	if structured.Code != interactiveAggregateShareVerificationFailedCode {
		t.Fatalf("unexpected code: [%s]", structured.Code)
	}
	if len(structured.CandidateCulprits) != 2 ||
		structured.CandidateCulprits[0] != 2 ||
		structured.CandidateCulprits[1] != 3 {
		t.Fatalf("unexpected candidate culprits: [%v]", structured.CandidateCulprits)
	}

	// A non-culprit error decodes with an empty culprit list.
	plain := buildTaggedTBTCSignerErrorPayload([]byte(`{"code":"validation_error","message":"x"}`))
	if len(plain.CandidateCulprits) != 0 {
		t.Fatalf("expected no culprits for a non-culprit error, got: [%v]", plain.CandidateCulprits)
	}
}

func TestBuildTaggedTBTCSignerDeriveInteractiveAttemptContextRequestPayload(t *testing.T) {
	payload, err := buildTaggedTBTCSignerDeriveInteractiveAttemptContextRequestPayload(
		"session-1",
		[]byte{0xab, 0xcd},
		"key-group-1",
		2,
		3, // RFC-21 0-based attempt
		[]uint16{1, 2, 3},
	)
	if err != nil {
		t.Fatalf("unexpected payload build error: [%v]", err)
	}

	var request buildTaggedTBTCSignerDeriveInteractiveAttemptContextRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatalf("cannot decode request payload: [%v]", err)
	}
	if request.SessionID != "session-1" {
		t.Fatalf("unexpected session id: [%s]", request.SessionID)
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
	// Wire attempt_number is 1-based: the RFC-21 0-based 3 serializes as 4.
	if request.AttemptNumber != 4 {
		t.Fatalf("unexpected wire attempt number: [%d]", request.AttemptNumber)
	}
	if len(request.IncludedParticipants) != 3 ||
		request.IncludedParticipants[0] != 1 ||
		request.IncludedParticipants[2] != 3 {
		t.Fatalf("unexpected included participants: [%v]", request.IncludedParticipants)
	}
}

func TestBuildTaggedTBTCSignerDeriveInteractiveAttemptContextRequestPayload_RejectsInvalidInput(t *testing.T) {
	tests := map[string]struct {
		sessionID    string
		message      []byte
		keyGroup     string
		threshold    uint16
		participants []uint16
	}{
		"empty session":      {"", []byte{0x01}, "kg", 2, []uint16{1, 2}},
		"empty message":      {"s", nil, "kg", 2, []uint16{1, 2}},
		"empty key group":    {"s", []byte{0x01}, "", 2, []uint16{1, 2}},
		"zero threshold":     {"s", []byte{0x01}, "kg", 0, []uint16{1, 2}},
		"empty participants": {"s", []byte{0x01}, "kg", 2, nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := buildTaggedTBTCSignerDeriveInteractiveAttemptContextRequestPayload(
				tc.sessionID, tc.message, tc.keyGroup, tc.threshold, 0, tc.participants,
			); err == nil {
				t.Fatal("expected invalid input to be rejected")
			}
		})
	}
}

func TestDecodeBuildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse(t *testing.T) {
	result, err := decodeBuildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse([]byte(`{
		"attempt_context": {
			"attempt_number": 4,
			"coordinator_identifier": 2,
			"included_participants": [1, 2, 3],
			"included_participants_fingerprint": "deadbeef",
			"attempt_id": "attempt-xyz"
		},
		"frost_identifiers": [
			{"participant_identifier": 1, "frost_identifier": "id-1"},
			{"participant_identifier": 2, "frost_identifier": "id-2"},
			{"participant_identifier": 3, "frost_identifier": "id-3"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected decode error: [%v]", err)
	}
	// Wire 1-based attempt_number 4 decodes to the RFC-21 0-based 3.
	if result.AttemptContext.AttemptNumber != 3 {
		t.Fatalf("unexpected attempt number: [%d]", result.AttemptContext.AttemptNumber)
	}
	if result.AttemptContext.CoordinatorIdentifier != 2 {
		t.Fatalf("unexpected coordinator: [%d]", result.AttemptContext.CoordinatorIdentifier)
	}
	if result.AttemptContext.IncludedParticipantsFingerprint != "deadbeef" {
		t.Fatalf("unexpected fingerprint: [%s]", result.AttemptContext.IncludedParticipantsFingerprint)
	}
	if result.AttemptContext.AttemptID != "attempt-xyz" {
		t.Fatalf("unexpected attempt id: [%s]", result.AttemptContext.AttemptID)
	}
	if len(result.AttemptContext.IncludedParticipants) != 3 ||
		result.AttemptContext.IncludedParticipants[2] != 3 {
		t.Fatalf("unexpected included participants: [%v]", result.AttemptContext.IncludedParticipants)
	}
	if len(result.FrostIdentifiers) != 3 ||
		result.FrostIdentifiers[0].ParticipantIdentifier != 1 ||
		result.FrostIdentifiers[0].FrostIdentifier != "id-1" ||
		result.FrostIdentifiers[2].FrostIdentifier != "id-3" {
		t.Fatalf("unexpected frost identifiers: [%+v]", result.FrostIdentifiers)
	}

	// Malformed JSON and a zero (impossible 1-based) wire attempt number reject.
	if _, err := decodeBuildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse([]byte(`{`)); err == nil {
		t.Fatal("expected malformed payload to be rejected")
	}
	if _, err := decodeBuildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse(
		[]byte(`{"attempt_context":{"attempt_number":0,"coordinator_identifier":2,"included_participants":[1,2],"included_participants_fingerprint":"ab","attempt_id":"a"}}`),
	); err == nil {
		t.Fatal("expected zero wire attempt number to be rejected")
	}
	// One identifier per participant is required: 3 participants, 2 identifiers.
	if _, err := decodeBuildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse(
		[]byte(`{"attempt_context":{"attempt_number":4,"coordinator_identifier":2,"included_participants":[1,2,3],"included_participants_fingerprint":"ab","attempt_id":"a"},"frost_identifiers":[{"participant_identifier":1,"frost_identifier":"id-1"},{"participant_identifier":2,"frost_identifier":"id-2"}]}`),
	); err == nil {
		t.Fatal("expected frost-identifier/participant count mismatch to be rejected")
	}
	// A matching count but mismatched participant correspondence is rejected
	// (participant 3 appears at the position participant 2 is expected).
	if _, err := decodeBuildTaggedTBTCSignerDeriveInteractiveAttemptContextResponse(
		[]byte(`{"attempt_context":{"attempt_number":4,"coordinator_identifier":2,"included_participants":[1,2,3],"included_participants_fingerprint":"ab","attempt_id":"a"},"frost_identifiers":[{"participant_identifier":1,"frost_identifier":"id-1"},{"participant_identifier":3,"frost_identifier":"id-3"},{"participant_identifier":2,"frost_identifier":"id-2"}]}`),
	); err == nil {
		t.Fatal("expected mismatched participant correspondence to be rejected")
	}
}
