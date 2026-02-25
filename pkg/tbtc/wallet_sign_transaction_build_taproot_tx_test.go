package tbtc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func TestWalletTransactionExecutor_SignTransaction_ReturnsBuildTaprootTxError(
	t *testing.T,
) {
	original := buildTaprootTxViaNativeSignerFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = original
	})

	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return "", errors.New("build tx failed")
	}

	wte := &walletTransactionExecutor{}

	_, err := wte.signTransaction(nil, nil, 0, 0)
	if err == nil {
		t.Fatal("expected signTransaction error")
	}

	if !strings.Contains(err.Error(), "native tbtc-signer") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

func TestNativeUnsignedTransactionIODiverges_MatchingIO(t *testing.T) {
	txHashBytes := make([]byte, bitcoin.HashByteLength)
	for i := range txHashBytes {
		txHashBytes[i] = byte(i + 1)
	}

	txHash, err := bitcoin.NewHash(txHashBytes, bitcoin.InternalByteOrder)
	if err != nil {
		t.Fatalf("cannot build tx hash: [%v]", err)
	}

	scriptPubKey := mustDecodeHex(t, "0014deadbeef")
	nativeTransaction := &bitcoin.Transaction{
		Version: 2,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: txHash,
					OutputIndex:     7,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           1000,
				PublicKeyScript: scriptPubKey,
			},
		},
		Locktime: 0,
	}

	nativeTxHex := hex.EncodeToString(nativeTransaction.Serialize(bitcoin.Standard))

	expectedInputs := []bitcoin.UnsignedTransactionInput{
		{
			TxIDHex:   txHash.Hex(bitcoin.ReversedByteOrder),
			Vout:      7,
			ValueSats: 1234,
		},
	}
	expectedOutputs := []bitcoin.UnsignedTransactionOutput{
		{
			ScriptPubKeyHex: "0014deadbeef",
			ValueSats:       1000,
		},
	}

	diverges, err := nativeUnsignedTransactionIODiverges(
		nativeTxHex,
		expectedInputs,
		expectedOutputs,
	)
	if err != nil {
		t.Fatalf("unexpected comparison error: [%v]", err)
	}

	if diverges {
		t.Fatal("expected matching unsigned transaction I/O")
	}
}

func TestNativeUnsignedTransactionIODiverges_MismatchedIO(t *testing.T) {
	txHashBytes := make([]byte, bitcoin.HashByteLength)
	for i := range txHashBytes {
		txHashBytes[i] = byte(i + 1)
	}

	txHash, err := bitcoin.NewHash(txHashBytes, bitcoin.InternalByteOrder)
	if err != nil {
		t.Fatalf("cannot build tx hash: [%v]", err)
	}

	scriptPubKey := mustDecodeHex(t, "0014deadbeef")
	nativeTransaction := &bitcoin.Transaction{
		Version: 2,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: txHash,
					OutputIndex:     7,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           1000,
				PublicKeyScript: scriptPubKey,
			},
		},
		Locktime: 0,
	}

	nativeTxHex := hex.EncodeToString(nativeTransaction.Serialize(bitcoin.Standard))

	expectedInputs := []bitcoin.UnsignedTransactionInput{
		{
			TxIDHex:   txHash.Hex(bitcoin.ReversedByteOrder),
			Vout:      7,
			ValueSats: 1234,
		},
	}
	expectedOutputs := []bitcoin.UnsignedTransactionOutput{
		{
			ScriptPubKeyHex: "0014deadbeef",
			ValueSats:       999,
		},
	}

	diverges, err := nativeUnsignedTransactionIODiverges(
		nativeTxHex,
		expectedInputs,
		expectedOutputs,
	)
	if err != nil {
		t.Fatalf("unexpected comparison error: [%v]", err)
	}

	if !diverges {
		t.Fatal("expected unsigned transaction I/O divergence")
	}
}

func TestEvaluateNativeUnsignedTransactionForSigning_ObservationalModeLogsWarning(
	t *testing.T,
) {
	logger := &warningCaptureLogger{}

	txHashBytes := make([]byte, bitcoin.HashByteLength)
	for i := range txHashBytes {
		txHashBytes[i] = byte(i + 1)
	}

	txHash, err := bitcoin.NewHash(txHashBytes, bitcoin.InternalByteOrder)
	if err != nil {
		t.Fatalf("cannot build tx hash: [%v]", err)
	}

	scriptPubKey := mustDecodeHex(t, "0014deadbeef")
	nativeTransaction := &bitcoin.Transaction{
		Version: 2,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: txHash,
					OutputIndex:     7,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           1000,
				PublicKeyScript: scriptPubKey,
			},
		},
		Locktime: 0,
	}

	nativeTxHex := hex.EncodeToString(nativeTransaction.Serialize(bitcoin.Standard))

	nativeUnsignedTx, err := evaluateNativeUnsignedTransactionForSigning(
		logger,
		nativeTxHex,
		[]bitcoin.UnsignedTransactionInput{
			{
				TxIDHex:   txHash.Hex(bitcoin.ReversedByteOrder),
				Vout:      7,
				ValueSats: 1234,
			},
		},
		[]bitcoin.UnsignedTransactionOutput{
			{
				ScriptPubKeyHex: "0014deadbeef",
				ValueSats:       999,
			},
		},
		false,
	)
	if err != nil {
		t.Fatalf("unexpected evaluation error: [%v]", err)
	}

	if nativeUnsignedTx != nil {
		t.Fatal("did not expect native transaction substitution in observational mode")
	}

	if len(logger.warningMessages) != 1 {
		t.Fatalf(
			"unexpected warning message count\nexpected: [%v]\nactual:   [%v]",
			1,
			len(logger.warningMessages),
		)
	}

	if !strings.Contains(logger.warningMessages[0], "diverges") {
		t.Fatalf("unexpected warning message: [%v]", logger.warningMessages[0])
	}
}

func TestEvaluateNativeUnsignedTransactionForSigning_SubstitutionModeRejectsDivergence(
	t *testing.T,
) {
	logger := &warningCaptureLogger{}

	txHashBytes := make([]byte, bitcoin.HashByteLength)
	for i := range txHashBytes {
		txHashBytes[i] = byte(i + 1)
	}

	txHash, err := bitcoin.NewHash(txHashBytes, bitcoin.InternalByteOrder)
	if err != nil {
		t.Fatalf("cannot build tx hash: [%v]", err)
	}

	scriptPubKey := mustDecodeHex(t, "0014deadbeef")
	nativeTransaction := &bitcoin.Transaction{
		Version: 2,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: txHash,
					OutputIndex:     7,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           1000,
				PublicKeyScript: scriptPubKey,
			},
		},
		Locktime: 0,
	}

	nativeTxHex := hex.EncodeToString(nativeTransaction.Serialize(bitcoin.Standard))

	nativeUnsignedTx, err := evaluateNativeUnsignedTransactionForSigning(
		logger,
		nativeTxHex,
		[]bitcoin.UnsignedTransactionInput{
			{
				TxIDHex:   txHash.Hex(bitcoin.ReversedByteOrder),
				Vout:      7,
				ValueSats: 1234,
			},
		},
		[]bitcoin.UnsignedTransactionOutput{
			{
				ScriptPubKeyHex: "0014deadbeef",
				ValueSats:       999,
			},
		},
		true,
	)
	if err == nil {
		t.Fatal("expected substitution-mode divergence error")
	}

	if !strings.Contains(err.Error(), "diverges") {
		t.Fatalf("unexpected substitution-mode error: [%v]", err)
	}

	if nativeUnsignedTx != nil {
		t.Fatal("did not expect native transaction on divergence")
	}

	if len(logger.warningMessages) != 0 {
		t.Fatalf("unexpected warnings in substitution mode: [%v]", logger.warningMessages)
	}
}

func TestEvaluateNativeUnsignedTransactionForSigning_SubstitutionModeAcceptsMatchingIO(
	t *testing.T,
) {
	logger := &warningCaptureLogger{}

	txHashBytes := make([]byte, bitcoin.HashByteLength)
	for i := range txHashBytes {
		txHashBytes[i] = byte(i + 1)
	}

	txHash, err := bitcoin.NewHash(txHashBytes, bitcoin.InternalByteOrder)
	if err != nil {
		t.Fatalf("cannot build tx hash: [%v]", err)
	}

	scriptPubKey := mustDecodeHex(t, "0014deadbeef")
	nativeTransaction := &bitcoin.Transaction{
		Version: 2,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: txHash,
					OutputIndex:     7,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           1000,
				PublicKeyScript: scriptPubKey,
			},
		},
		Locktime: 0,
	}

	nativeTxHex := hex.EncodeToString(nativeTransaction.Serialize(bitcoin.Standard))

	nativeUnsignedTx, err := evaluateNativeUnsignedTransactionForSigning(
		logger,
		nativeTxHex,
		[]bitcoin.UnsignedTransactionInput{
			{
				TxIDHex:   txHash.Hex(bitcoin.ReversedByteOrder),
				Vout:      7,
				ValueSats: 1234,
			},
		},
		[]bitcoin.UnsignedTransactionOutput{
			{
				ScriptPubKeyHex: "0014deadbeef",
				ValueSats:       1000,
			},
		},
		true,
	)
	if err != nil {
		t.Fatalf("unexpected substitution-mode evaluation error: [%v]", err)
	}

	if nativeUnsignedTx == nil {
		t.Fatal("expected native transaction substitution candidate")
	}

	if len(logger.warningMessages) != 0 {
		t.Fatalf("unexpected warnings in substitution mode: [%v]", logger.warningMessages)
	}
}

func TestNativeBuildTaprootTxSigningSubstitutionEnabled(t *testing.T) {
	testCases := []struct {
		name     string
		envValue string
		expected bool
	}{
		{name: "unset", envValue: "", expected: false},
		{name: "true", envValue: "true", expected: true},
		{name: "TRUE", envValue: "TRUE", expected: true},
		{name: "one", envValue: "1", expected: true},
		{name: "yes", envValue: "yes", expected: true},
		{name: "on", envValue: "on", expected: true},
		{name: "false", envValue: "false", expected: false},
		{name: "zero", envValue: "0", expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(nativeBuildTaprootTxSigningSubstitutionEnvVar, tc.envValue)

			actual := nativeBuildTaprootTxSigningSubstitutionEnabled()
			if actual != tc.expected {
				t.Fatalf(
					"unexpected flag state\nexpected: [%v]\nactual:   [%v]",
					tc.expected,
					actual,
				)
			}
		})
	}
}

func mustDecodeHex(t *testing.T, value string) []byte {
	result, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("cannot decode hex: [%v]", err)
	}

	return result
}

type warningCaptureLogger struct {
	warningMessages []string
}

func (wcl *warningCaptureLogger) Debug(args ...interface{}) {}

func (wcl *warningCaptureLogger) Debugf(format string, args ...interface{}) {}

func (wcl *warningCaptureLogger) Error(args ...interface{}) {}

func (wcl *warningCaptureLogger) Errorf(format string, args ...interface{}) {}

func (wcl *warningCaptureLogger) Fatal(args ...interface{}) {}

func (wcl *warningCaptureLogger) Fatalf(format string, args ...interface{}) {}

func (wcl *warningCaptureLogger) Info(args ...interface{}) {}

func (wcl *warningCaptureLogger) Infof(format string, args ...interface{}) {}

func (wcl *warningCaptureLogger) Panic(args ...interface{}) {}

func (wcl *warningCaptureLogger) Panicf(format string, args ...interface{}) {}

func (wcl *warningCaptureLogger) Warn(args ...interface{}) {}

func (wcl *warningCaptureLogger) Warnf(format string, args ...interface{}) {
	wcl.warningMessages = append(
		wcl.warningMessages,
		fmt.Sprintf(format, args...),
	)
}
