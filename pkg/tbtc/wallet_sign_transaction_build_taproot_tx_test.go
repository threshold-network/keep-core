package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/tecdsa"
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
		&bitcoin.Transaction{
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
					Value:           999,
					PublicKeyScript: scriptPubKey,
				},
			},
			Locktime: 0,
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

func TestEvaluateNativeUnsignedTransactionForSigning_ObservationalModeLogsWarningOnStructuralDivergence(
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
		&bitcoin.Transaction{
			Version: 1,
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
		&bitcoin.Transaction{
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
					Value:           999,
					PublicKeyScript: scriptPubKey,
				},
			},
			Locktime: 0,
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
		nativeTransaction,
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

func TestEvaluateNativeUnsignedTransactionForSigning_SubstitutionModeRejectsStructuralDivergence(
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

	expectedTransaction := &bitcoin.Transaction{
		Version: 1,
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

	nativeUnsignedTx, err := evaluateNativeUnsignedTransactionForSigning(
		logger,
		nativeTxHex,
		expectedTransaction,
		true,
	)
	if err == nil {
		t.Fatal("expected substitution-mode structural divergence error")
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

func TestWalletTransactionExecutor_SignTransaction_SubstitutesNativeUnsignedTransactionWhenGateEnabled(
	t *testing.T,
) {
	privateKey, unsignedTx, nativeUnsignedTxHex, nativeUnsignedTx := buildTaprootTxSubstitutionFixture(t)

	originalBuildTaprootTxViaNativeSignerFn := buildTaprootTxViaNativeSignerFn
	originalSigningSubstitutionEnabledFn := nativeBuildTaprootTxSigningSubstitutionEnabledFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = originalBuildTaprootTxViaNativeSignerFn
		nativeBuildTaprootTxSigningSubstitutionEnabledFn = originalSigningSubstitutionEnabledFn
	})

	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return nativeUnsignedTxHex, nil
	}
	nativeBuildTaprootTxSigningSubstitutionEnabledFn = func() bool {
		return true
	}

	wte := &walletTransactionExecutor{
		executingWallet: wallet{
			publicKey: &privateKey.PublicKey,
		},
		signingExecutor: &deterministicECDSASigningExecutorForBuildTaprootTxSubstitution{
			privateKey: privateKey,
		},
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}

	logger := &warningCaptureLogger{}

	tx, err := wte.signTransaction(logger, unsignedTx, 0, 0)
	if err != nil {
		t.Fatalf("unexpected signTransaction error: [%v]", err)
	}

	if tx.Version != nativeUnsignedTx.Version {
		t.Fatalf(
			"unexpected substituted transaction version\nexpected: [%v]\nactual:   [%v]",
			nativeUnsignedTx.Version,
			tx.Version,
		)
	}

	if tx.Locktime != nativeUnsignedTx.Locktime {
		t.Fatalf(
			"unexpected substituted transaction locktime\nexpected: [%v]\nactual:   [%v]",
			nativeUnsignedTx.Locktime,
			tx.Locktime,
		)
	}

	if len(tx.Inputs) != len(nativeUnsignedTx.Inputs) {
		t.Fatalf(
			"unexpected substituted input count\nexpected: [%v]\nactual:   [%v]",
			len(nativeUnsignedTx.Inputs),
			len(tx.Inputs),
		)
	}

	if tx.Inputs[0].Outpoint.TransactionHash != nativeUnsignedTx.Inputs[0].Outpoint.TransactionHash {
		t.Fatalf(
			"unexpected substituted input txid\nexpected: [%v]\nactual:   [%v]",
			nativeUnsignedTx.Inputs[0].Outpoint.TransactionHash,
			tx.Inputs[0].Outpoint.TransactionHash,
		)
	}

	if tx.Inputs[0].Outpoint.OutputIndex != nativeUnsignedTx.Inputs[0].Outpoint.OutputIndex {
		t.Fatalf(
			"unexpected substituted input vout\nexpected: [%v]\nactual:   [%v]",
			nativeUnsignedTx.Inputs[0].Outpoint.OutputIndex,
			tx.Inputs[0].Outpoint.OutputIndex,
		)
	}

	if tx.Inputs[0].Sequence != nativeUnsignedTx.Inputs[0].Sequence {
		t.Fatalf(
			"unexpected substituted input sequence\nexpected: [%v]\nactual:   [%v]",
			nativeUnsignedTx.Inputs[0].Sequence,
			tx.Inputs[0].Sequence,
		)
	}

	if len(tx.Inputs[0].SignatureScript) == 0 {
		t.Fatal("expected signature script to be populated after signing")
	}

	if len(tx.Outputs) != len(nativeUnsignedTx.Outputs) {
		t.Fatalf(
			"unexpected substituted output count\nexpected: [%v]\nactual:   [%v]",
			len(nativeUnsignedTx.Outputs),
			len(tx.Outputs),
		)
	}

	if tx.Outputs[0].Value != nativeUnsignedTx.Outputs[0].Value {
		t.Fatalf(
			"unexpected substituted output value\nexpected: [%v]\nactual:   [%v]",
			nativeUnsignedTx.Outputs[0].Value,
			tx.Outputs[0].Value,
		)
	}

	if !bytes.Equal(
		tx.Outputs[0].PublicKeyScript,
		nativeUnsignedTx.Outputs[0].PublicKeyScript,
	) {
		t.Fatalf(
			"unexpected substituted output script\nexpected: [%x]\nactual:   [%x]",
			nativeUnsignedTx.Outputs[0].PublicKeyScript,
			tx.Outputs[0].PublicKeyScript,
		)
	}

	if len(logger.warningMessages) != 0 {
		t.Fatalf("unexpected warning logs: [%v]", logger.warningMessages)
	}

	if !containsLoggedMessage(
		logger.infoMessages,
		"substituted Go unsigned transaction with native tbtc-signer BuildTaprootTx output",
	) {
		t.Fatalf("expected substitution info log, got: [%v]", logger.infoMessages)
	}
}

func TestWalletTransactionExecutor_SignTransaction_DoesNotSubstituteWhenGateDisabled(
	t *testing.T,
) {
	privateKey, unsignedTx, nativeUnsignedTxHex, _ := buildTaprootTxSubstitutionFixture(t)

	originalBuildTaprootTxViaNativeSignerFn := buildTaprootTxViaNativeSignerFn
	originalSigningSubstitutionEnabledFn := nativeBuildTaprootTxSigningSubstitutionEnabledFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = originalBuildTaprootTxViaNativeSignerFn
		nativeBuildTaprootTxSigningSubstitutionEnabledFn = originalSigningSubstitutionEnabledFn
	})

	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return nativeUnsignedTxHex, nil
	}
	nativeBuildTaprootTxSigningSubstitutionEnabledFn = func() bool {
		return false
	}

	wte := &walletTransactionExecutor{
		executingWallet: wallet{
			publicKey: &privateKey.PublicKey,
		},
		signingExecutor: &deterministicECDSASigningExecutorForBuildTaprootTxSubstitution{
			privateKey: privateKey,
		},
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}

	logger := &warningCaptureLogger{}

	tx, err := wte.signTransaction(logger, unsignedTx, 0, 0)
	if err != nil {
		t.Fatalf("unexpected signTransaction error: [%v]", err)
	}

	if tx.Version != 1 {
		t.Fatalf(
			"unexpected non-substituted transaction version\nexpected: [1]\nactual:   [%v]",
			tx.Version,
		)
	}

	if tx.Locktime != 0 {
		t.Fatalf(
			"unexpected non-substituted transaction locktime\nexpected: [0]\nactual:   [%v]",
			tx.Locktime,
		)
	}

	if tx.Inputs[0].Sequence != 0xffffffff {
		t.Fatalf(
			"unexpected non-substituted input sequence\nexpected: [4294967295]\nactual:   [%v]",
			tx.Inputs[0].Sequence,
		)
	}

	if len(logger.warningMessages) != 0 {
		t.Fatalf("unexpected warning logs: [%v]", logger.warningMessages)
	}

	if containsLoggedMessage(
		logger.infoMessages,
		"substituted Go unsigned transaction with native tbtc-signer BuildTaprootTx output",
	) {
		t.Fatalf("did not expect substitution info log when gate disabled: [%v]", logger.infoMessages)
	}
}

func TestWalletTransactionExecutor_SignTransaction_RejectsNativeUnsignedTransactionDivergenceWhenGateEnabled(
	t *testing.T,
) {
	privateKey, unsignedTx, _, nativeUnsignedTx := buildTaprootTxSubstitutionFixture(t)

	divergingNativeUnsignedTx := *nativeUnsignedTx
	divergingOutputs := make(
		[]*bitcoin.TransactionOutput,
		len(nativeUnsignedTx.Outputs),
	)
	for i, output := range nativeUnsignedTx.Outputs {
		if output == nil {
			t.Fatalf("native fixture output [%d] is nil", i)
		}

		clonedOutput := *output
		divergingOutputs[i] = &clonedOutput
	}
	divergingNativeUnsignedTx.Outputs = divergingOutputs
	divergingNativeUnsignedTx.Outputs[0].Value = nativeUnsignedTx.Outputs[0].Value - 1
	divergingNativeUnsignedTxHex := hex.EncodeToString(
		divergingNativeUnsignedTx.Serialize(bitcoin.Standard),
	)

	originalBuildTaprootTxViaNativeSignerFn := buildTaprootTxViaNativeSignerFn
	originalSigningSubstitutionEnabledFn := nativeBuildTaprootTxSigningSubstitutionEnabledFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = originalBuildTaprootTxViaNativeSignerFn
		nativeBuildTaprootTxSigningSubstitutionEnabledFn = originalSigningSubstitutionEnabledFn
	})

	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return divergingNativeUnsignedTxHex, nil
	}
	nativeBuildTaprootTxSigningSubstitutionEnabledFn = func() bool {
		return true
	}

	wte := &walletTransactionExecutor{
		executingWallet: wallet{
			publicKey: &privateKey.PublicKey,
		},
		signingExecutor: &deterministicECDSASigningExecutorForBuildTaprootTxSubstitution{
			privateKey: privateKey,
		},
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}

	logger := &warningCaptureLogger{}

	tx, err := wte.signTransaction(logger, unsignedTx, 0, 0)
	if err == nil {
		t.Fatal("expected signTransaction divergence error")
	}

	if tx != nil {
		t.Fatal("expected no signed transaction on substitution divergence")
	}

	if !strings.Contains(err.Error(), "diverges") {
		t.Fatalf("unexpected signTransaction divergence error: [%v]", err)
	}

	if len(logger.warningMessages) != 0 {
		t.Fatalf("unexpected warning logs in substitution mode: [%v]", logger.warningMessages)
	}
}

func TestWalletTransactionExecutor_SignTransaction_RejectsNativeUnsignedTransactionStructuralDivergenceWhenGateEnabled(
	t *testing.T,
) {
	privateKey, unsignedTx, _, nativeUnsignedTx := buildTaprootTxSubstitutionFixture(t)

	divergingNativeUnsignedTx := *nativeUnsignedTx
	divergingInputs := make(
		[]*bitcoin.TransactionInput,
		len(nativeUnsignedTx.Inputs),
	)
	for i, input := range nativeUnsignedTx.Inputs {
		if input == nil {
			t.Fatalf("native fixture input [%d] is nil", i)
		}

		clonedInput := *input
		divergingInputs[i] = &clonedInput
	}
	divergingNativeUnsignedTx.Inputs = divergingInputs
	divergingNativeUnsignedTx.Version = nativeUnsignedTx.Version + 1
	divergingNativeUnsignedTx.Locktime = nativeUnsignedTx.Locktime + 1
	divergingNativeUnsignedTx.Inputs[0].Sequence = nativeUnsignedTx.Inputs[0].Sequence - 1
	divergingNativeUnsignedTxHex := hex.EncodeToString(
		divergingNativeUnsignedTx.Serialize(bitcoin.Standard),
	)

	originalBuildTaprootTxViaNativeSignerFn := buildTaprootTxViaNativeSignerFn
	originalSigningSubstitutionEnabledFn := nativeBuildTaprootTxSigningSubstitutionEnabledFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = originalBuildTaprootTxViaNativeSignerFn
		nativeBuildTaprootTxSigningSubstitutionEnabledFn = originalSigningSubstitutionEnabledFn
	})

	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return divergingNativeUnsignedTxHex, nil
	}
	nativeBuildTaprootTxSigningSubstitutionEnabledFn = func() bool {
		return true
	}

	wte := &walletTransactionExecutor{
		executingWallet: wallet{
			publicKey: &privateKey.PublicKey,
		},
		signingExecutor: &deterministicECDSASigningExecutorForBuildTaprootTxSubstitution{
			privateKey: privateKey,
		},
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}

	logger := &warningCaptureLogger{}

	tx, err := wte.signTransaction(logger, unsignedTx, 0, 0)
	if err == nil {
		t.Fatal("expected signTransaction structural divergence error")
	}

	if tx != nil {
		t.Fatal("expected no signed transaction on substitution structural divergence")
	}

	if !strings.Contains(err.Error(), "diverges") {
		t.Fatalf("unexpected signTransaction divergence error: [%v]", err)
	}

	if len(logger.warningMessages) != 0 {
		t.Fatalf("unexpected warning logs in substitution mode: [%v]", logger.warningMessages)
	}
}

func buildTaprootTxSubstitutionFixture(
	t *testing.T,
) (
	*ecdsa.PrivateKey,
	*bitcoin.TransactionBuilder,
	string,
	*bitcoin.Transaction,
) {
	privateKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: tecdsa.Curve,
		},
		D: big.NewInt(111),
	}
	privateKey.PublicKey.X, privateKey.PublicKey.Y = tecdsa.Curve.ScalarBaseMult(
		privateKey.D.Bytes(),
	)

	pubKeyHash := [20]byte{}
	for i := range pubKeyHash {
		pubKeyHash[i] = byte(i + 1)
	}

	lockingScript, err := bitcoin.PayToPublicKeyHash(pubKeyHash)
	if err != nil {
		t.Fatalf("cannot create locking script: [%v]", err)
	}

	localBitcoinChain := newLocalBitcoinChain()

	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs:  []*bitcoin.TransactionInput{},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           10000,
				PublicKeyScript: lockingScript,
			},
		},
		Locktime: 0,
	}

	if err := localBitcoinChain.BroadcastTransaction(fundingTransaction); err != nil {
		t.Fatalf("cannot broadcast funding transaction: [%v]", err)
	}

	unsignedTx := bitcoin.NewTransactionBuilder(localBitcoinChain)
	if err := unsignedTx.AddPublicKeyHashInput(
		&bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTransaction.Hash(),
				OutputIndex:     0,
			},
			Value: 10000,
		},
	); err != nil {
		t.Fatalf("cannot add unsigned input: [%v]", err)
	}

	replacementOutputScript := mustDecodeHex(t, "0014deadbeef")
	unsignedTx.AddOutput(
		&bitcoin.TransactionOutput{
			Value:           9000,
			PublicKeyScript: replacementOutputScript,
		},
	)

	nativeUnsignedTx := &bitcoin.Transaction{
		Version:  1,
		Locktime: 0,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: fundingTransaction.Hash(),
					OutputIndex:     0,
				},
				Sequence: 0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           9000,
				PublicKeyScript: replacementOutputScript,
			},
		},
	}

	return privateKey,
		unsignedTx,
		hex.EncodeToString(nativeUnsignedTx.Serialize(bitcoin.Standard)),
		nativeUnsignedTx
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
	infoMessages    []string
}

func (wcl *warningCaptureLogger) Debug(args ...interface{}) {}

func (wcl *warningCaptureLogger) Debugf(format string, args ...interface{}) {}

func (wcl *warningCaptureLogger) Error(args ...interface{}) {}

func (wcl *warningCaptureLogger) Errorf(format string, args ...interface{}) {}

func (wcl *warningCaptureLogger) Fatal(args ...interface{}) {}

func (wcl *warningCaptureLogger) Fatalf(format string, args ...interface{}) {}

func (wcl *warningCaptureLogger) Info(args ...interface{}) {
	wcl.infoMessages = append(wcl.infoMessages, fmt.Sprint(args...))
}

func (wcl *warningCaptureLogger) Infof(format string, args ...interface{}) {
	wcl.infoMessages = append(wcl.infoMessages, fmt.Sprintf(format, args...))
}

func (wcl *warningCaptureLogger) Panic(args ...interface{}) {}

func (wcl *warningCaptureLogger) Panicf(format string, args ...interface{}) {}

func (wcl *warningCaptureLogger) Warn(args ...interface{}) {}

func (wcl *warningCaptureLogger) Warnf(format string, args ...interface{}) {
	wcl.warningMessages = append(
		wcl.warningMessages,
		fmt.Sprintf(format, args...),
	)
}

func containsLoggedMessage(messages []string, substring string) bool {
	for _, message := range messages {
		if strings.Contains(message, substring) {
			return true
		}
	}

	return false
}

type deterministicECDSASigningExecutorForBuildTaprootTxSubstitution struct {
	privateKey *ecdsa.PrivateKey
}

func (desefbts *deterministicECDSASigningExecutorForBuildTaprootTxSubstitution) signBatch(
	ctx context.Context,
	messages []*big.Int,
	startBlock uint64,
) ([]*frost.Signature, error) {
	signatures := make([]*frost.Signature, 0, len(messages))

	for _, message := range messages {
		r, s, err := ecdsa.Sign(
			rand.Reader,
			desefbts.privateKey,
			message.Bytes(),
		)
		if err != nil {
			return nil, err
		}

		signature := &frost.Signature{}
		rBytes := r.Bytes()
		copy(signature.R[len(signature.R)-len(rBytes):], rBytes)
		sBytes := s.Bytes()
		copy(signature.S[len(signature.S)-len(sBytes):], sBytes)

		signatures = append(signatures, signature)
	}

	return signatures, nil
}
