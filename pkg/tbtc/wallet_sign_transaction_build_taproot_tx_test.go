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

	btcec2 "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/frost"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestWalletTransactionExecutor_SignTransaction_ReturnsBuildTaprootTxError(
	t *testing.T,
) {
	privateKey, unsignedTx, _, _ := buildTaprootTxSubstitutionFixture(t)

	original := buildTaprootTxViaNativeSignerFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = original
	})

	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return "", errors.New("build tx failed")
	}

	wte := &walletTransactionExecutor{
		executingWallet: wallet{
			publicKey: &privateKey.PublicKey,
		},
		signingExecutor: &unexpectedSigningExecutorForBuildTaprootTxError{},
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}
	logger := &warningCaptureLogger{}

	_, err := wte.signTransaction(logger, unsignedTx, 0, 0)
	if err == nil {
		t.Fatal("expected signTransaction error")
	}

	if !strings.Contains(err.Error(), "native tbtc-signer") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

func TestWalletTransactionExecutor_SignTransaction_PropagatesBuildTaprootTxBridgeOperationError(
	t *testing.T,
) {
	privateKey, unsignedTx, _, _ := buildTaprootTxSubstitutionFixture(t)

	original := buildTaprootTxViaNativeSignerFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = original
	})

	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return "", fmt.Errorf(
			"%w: operation failed",
			frostsigning.ErrNativeBridgeOperationFailed,
		)
	}

	wte := &walletTransactionExecutor{
		executingWallet: wallet{
			publicKey: &privateKey.PublicKey,
		},
		signingExecutor: &unexpectedSigningExecutorForBuildTaprootTxError{},
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}
	logger := &warningCaptureLogger{}

	_, err := wte.signTransaction(logger, unsignedTx, 0, 0)
	if err == nil {
		t.Fatal("expected signTransaction error")
	}

	if !errors.Is(err, frostsigning.ErrNativeBridgeOperationFailed) {
		t.Fatalf(
			"expected bridge operation failure error: [%v], got [%v]",
			frostsigning.ErrNativeBridgeOperationFailed,
			err,
		)
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

	if !strings.Contains(logger.warningMessages[0], "output value mismatch") {
		t.Fatalf("missing divergence detail in warning: [%v]", logger.warningMessages[0])
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

	if !strings.Contains(logger.warningMessages[0], "version mismatch") {
		t.Fatalf("missing divergence detail in warning: [%v]", logger.warningMessages[0])
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

	if !strings.Contains(err.Error(), "output value mismatch") {
		t.Fatalf("missing divergence detail in substitution error: [%v]", err)
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

	if !strings.Contains(err.Error(), "version mismatch") {
		t.Fatalf("missing divergence detail in substitution error: [%v]", err)
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

	if !strings.Contains(err.Error(), "output value mismatch") {
		t.Fatalf("missing divergence detail in signTransaction error: [%v]", err)
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

	if !strings.Contains(err.Error(), "version mismatch") {
		t.Fatalf("missing divergence detail in signTransaction error: [%v]", err)
	}

	if len(logger.warningMessages) != 0 {
		t.Fatalf("unexpected warning logs in substitution mode: [%v]", logger.warningMessages)
	}
}

func TestWalletTransactionExecutor_SignTransaction_AppliesTaprootKeyPathSignatures(
	t *testing.T,
) {
	originalBuildTaprootTxViaNativeSignerFn := buildTaprootTxViaNativeSignerFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = originalBuildTaprootTxViaNativeSignerFn
	})
	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return "", nil
	}

	privateKeyBytes := mustDecodeHex(
		t,
		"0101010101010101010101010101010101010101010101010101010101010101",
	)
	privateKey, publicKey := btcec2.PrivKeyFromBytes(privateKeyBytes)

	var taprootOutputKey [32]byte
	copy(taprootOutputKey[:], schnorr.SerializePubKey(publicKey))

	inputScript, err := bitcoin.PayToTaproot(taprootOutputKey)
	if err != nil {
		t.Fatalf("cannot create taproot input script: [%v]", err)
	}

	var outputPublicKeyHash [20]byte
	copy(
		outputPublicKeyHash[:],
		mustDecodeHex(t, "0202020202020202020202020202020202020202"),
	)
	outputScript, err := bitcoin.PayToWitnessPublicKeyHash(outputPublicKeyHash)
	if err != nil {
		t.Fatalf("cannot create output script: [%v]", err)
	}

	localBitcoinChain := newLocalBitcoinChain()
	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{
						0x10, 0x11, 0x12, 0x13,
						0x14, 0x15, 0x16, 0x17,
						0x18, 0x19, 0x1a, 0x1b,
						0x1c, 0x1d, 0x1e, 0x1f,
						0x20, 0x21, 0x22, 0x23,
						0x24, 0x25, 0x26, 0x27,
						0x28, 0x29, 0x2a, 0x2b,
						0x2c, 0x2d, 0x2e, 0x2f,
					},
					OutputIndex: 0,
				},
				SignatureScript: []byte{0x51},
				Sequence:        0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: inputScript,
			},
		},
		Locktime: 0,
	}
	if err := localBitcoinChain.BroadcastTransaction(fundingTransaction); err != nil {
		t.Fatalf("cannot broadcast funding transaction: [%v]", err)
	}

	unsignedTx := bitcoin.NewTransactionBuilder(localBitcoinChain)
	if err := unsignedTx.AddTaprootKeyPathInput(
		&bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTransaction.Hash(),
				OutputIndex:     0,
			},
			Value: 100000,
		},
	); err != nil {
		t.Fatalf("cannot add taproot input: [%v]", err)
	}
	unsignedTx.AddOutput(&bitcoin.TransactionOutput{
		Value:           90000,
		PublicKeyScript: outputScript,
	})

	wte := &walletTransactionExecutor{
		executingWallet: generateWallet(big.NewInt(111)),
		signingExecutor: &deterministicSchnorrSigningExecutorForTaproot{
			privateKey: privateKey,
		},
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}

	tx, err := wte.signTransaction(&warningCaptureLogger{}, unsignedTx, 0, 0)
	if err != nil {
		t.Fatalf("unexpected signTransaction error: [%v]", err)
	}

	expectedSignature := mustDecodeHex(
		t,
		"5e847a0c22486f3b89ff80edd5afaf4be550aa411a0a7e28cff19d2b5924d77102bbf9a0a51100f4fdfc8435d0e8ff0f61dfdeccd464b78c553b1b4414ac0877",
	)

	if len(tx.Inputs) != 1 {
		t.Fatalf("unexpected input count: [%d]", len(tx.Inputs))
	}
	if len(tx.Inputs[0].Witness) != 1 {
		t.Fatalf("unexpected taproot witness: [%x]", tx.Inputs[0].Witness)
	}
	if !bytes.Equal(expectedSignature, tx.Inputs[0].Witness[0]) {
		t.Fatalf(
			"unexpected taproot witness signature\nexpected: [%x]\nactual:   [%x]",
			expectedSignature,
			tx.Inputs[0].Witness[0],
		)
	}
	if len(tx.Inputs[0].SignatureScript) != 0 {
		t.Fatalf(
			"unexpected signature script for taproot input: [%x]",
			tx.Inputs[0].SignatureScript,
		)
	}
}

func TestWalletTransactionExecutor_SignTransaction_RejectsMixedTaprootAndLegacyInputsBeforeSigning(
	t *testing.T,
) {
	originalBuildTaprootTxViaNativeSignerFn := buildTaprootTxViaNativeSignerFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = originalBuildTaprootTxViaNativeSignerFn
	})
	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return "", nil
	}

	var taprootOutputKey [32]byte
	copy(
		taprootOutputKey[:],
		mustDecodeHex(
			t,
			"1b84c5567b126440995d3ed5aaba0565d71e1834604819ff9c17f5e9d5dd078f",
		),
	)
	taprootInputScript, err := bitcoin.PayToTaproot(taprootOutputKey)
	if err != nil {
		t.Fatalf("cannot create taproot input script: [%v]", err)
	}

	var witnessPublicKeyHash [20]byte
	copy(
		witnessPublicKeyHash[:],
		mustDecodeHex(t, "0202020202020202020202020202020202020202"),
	)
	witnessInputScript, err := bitcoin.PayToWitnessPublicKeyHash(
		witnessPublicKeyHash,
	)
	if err != nil {
		t.Fatalf("cannot create witness input script: [%v]", err)
	}

	localBitcoinChain := newLocalBitcoinChain()
	taprootFundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x01},
					OutputIndex:     0,
				},
				SignatureScript: []byte{0x51},
				Sequence:        0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: taprootInputScript,
			},
		},
		Locktime: 0,
	}
	if err := localBitcoinChain.BroadcastTransaction(
		taprootFundingTransaction,
	); err != nil {
		t.Fatalf("cannot broadcast taproot funding transaction: [%v]", err)
	}

	legacyFundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x02},
					OutputIndex:     0,
				},
				SignatureScript: []byte{0x51},
				Sequence:        0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           50000,
				PublicKeyScript: witnessInputScript,
			},
		},
		Locktime: 0,
	}
	if err := localBitcoinChain.BroadcastTransaction(
		legacyFundingTransaction,
	); err != nil {
		t.Fatalf("cannot broadcast legacy funding transaction: [%v]", err)
	}

	unsignedTx := bitcoin.NewTransactionBuilder(localBitcoinChain)
	if err := unsignedTx.AddTaprootKeyPathInput(
		&bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: taprootFundingTransaction.Hash(),
				OutputIndex:     0,
			},
			Value: 100000,
		},
	); err != nil {
		t.Fatalf("cannot add taproot input: [%v]", err)
	}
	if err := unsignedTx.AddPublicKeyHashInput(
		&bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: legacyFundingTransaction.Hash(),
				OutputIndex:     0,
			},
			Value: 50000,
		},
	); err != nil {
		t.Fatalf("cannot add legacy input: [%v]", err)
	}
	unsignedTx.AddOutput(&bitcoin.TransactionOutput{
		Value:           140000,
		PublicKeyScript: witnessInputScript,
	})

	wte := &walletTransactionExecutor{
		executingWallet: generateWallet(big.NewInt(111)),
		signingExecutor: &unexpectedSigningExecutorForBuildTaprootTxError{},
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}

	tx, err := wte.signTransaction(&warningCaptureLogger{}, unsignedTx, 0, 0)
	if err == nil {
		t.Fatal("expected mixed taproot and legacy signing error")
	}
	if tx != nil {
		t.Fatal("expected no signed transaction")
	}
	if !strings.Contains(err.Error(), "mixed taproot and legacy inputs") {
		t.Fatalf("unexpected error: [%v]", err)
	}
}

func TestWalletTransactionExecutor_SignTransaction_RejectsSchnorrForLegacyInputsBeforeSigning(
	t *testing.T,
) {
	originalBuildTaprootTxViaNativeSignerFn := buildTaprootTxViaNativeSignerFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = originalBuildTaprootTxViaNativeSignerFn
	})
	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		return "", nil
	}

	var publicKeyHash [20]byte
	copy(
		publicKeyHash[:],
		mustDecodeHex(t, "0202020202020202020202020202020202020202"),
	)
	inputScript, err := bitcoin.PayToWitnessPublicKeyHash(publicKeyHash)
	if err != nil {
		t.Fatalf("cannot create witness input script: [%v]", err)
	}

	localBitcoinChain := newLocalBitcoinChain()
	fundingTransaction := &bitcoin.Transaction{
		Version: 1,
		Inputs: []*bitcoin.TransactionInput{
			{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{0x03},
					OutputIndex:     0,
				},
				SignatureScript: []byte{0x51},
				Sequence:        0xffffffff,
			},
		},
		Outputs: []*bitcoin.TransactionOutput{
			{
				Value:           100000,
				PublicKeyScript: inputScript,
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
			Value: 100000,
		},
	); err != nil {
		t.Fatalf("cannot add legacy input: [%v]", err)
	}
	unsignedTx.AddOutput(&bitcoin.TransactionOutput{
		Value:           90000,
		PublicKeyScript: inputScript,
	})

	wte := &walletTransactionExecutor{
		executingWallet: generateWallet(big.NewInt(111)),
		signingExecutor: &deterministicSchnorrSigningExecutorForTaproot{},
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}

	tx, err := wte.signTransaction(&warningCaptureLogger{}, unsignedTx, 0, 0)
	if err == nil {
		t.Fatal("expected schnorr non-taproot signing error")
	}
	if tx != nil {
		t.Fatal("expected no signed transaction")
	}
	if !strings.Contains(err.Error(), "non-taproot transaction inputs") {
		t.Fatalf("unexpected error: [%v]", err)
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

type deterministicSchnorrSigningExecutorForTaproot struct {
	privateKey *btcec2.PrivateKey
}

func (dsseft *deterministicSchnorrSigningExecutorForTaproot) signBatch(
	ctx context.Context,
	messages []*big.Int,
	startBlock uint64,
) ([]*frost.Signature, error) {
	signatures := make([]*frost.Signature, 0, len(messages))

	for _, message := range messages {
		signature, err := schnorr.Sign(
			dsseft.privateKey,
			message.FillBytes(make([]byte, 32)),
		)
		if err != nil {
			return nil, err
		}

		serialized := signature.Serialize()
		frostSignature := &frost.Signature{}
		copy(frostSignature.R[:], serialized[:32])
		copy(frostSignature.S[:], serialized[32:])

		signatures = append(signatures, frostSignature)
	}

	return signatures, nil
}

func (dsseft *deterministicSchnorrSigningExecutorForTaproot) usesSchnorrSignatures() bool {
	return true
}

type unexpectedSigningExecutorForBuildTaprootTxError struct{}

func (usefbte *unexpectedSigningExecutorForBuildTaprootTxError) signBatch(
	ctx context.Context,
	messages []*big.Int,
	startBlock uint64,
) ([]*frost.Signature, error) {
	return nil, errors.New("unexpected signBatch invocation")
}
