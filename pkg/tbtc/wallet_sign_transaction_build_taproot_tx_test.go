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
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)

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
		executingWallet: generateWallet(big.NewInt(111)),
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
	unsignedTx, _ := buildTaprootKeyPathUnsignedTxForTest(t)

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
		executingWallet: generateWallet(big.NewInt(111)),
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

// The native tbtc-signer BuildTaprootTx parity/substitution path is gated on the
// transaction being all-Taproot. The substitution LOGIC itself (observational
// logging, divergence rejection, matching-IO acceptance) is covered directly by
// the TestEvaluateNativeUnsignedTransactionForSigning_* tests; these two tests
// cover the signTransaction gate: skip for legacy, invoke for Taproot.
func TestWalletTransactionExecutor_SignTransaction_SkipsNativeBuildForLegacyTransaction(
	t *testing.T,
) {
	privateKey, unsignedTx, _, _ := buildTaprootTxSubstitutionFixture(t)

	originalBuildTaprootTxViaNativeSignerFn := buildTaprootTxViaNativeSignerFn
	originalSigningSubstitutionEnabledFn := nativeBuildTaprootTxSigningSubstitutionEnabledFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = originalBuildTaprootTxViaNativeSignerFn
		nativeBuildTaprootTxSigningSubstitutionEnabledFn = originalSigningSubstitutionEnabledFn
	})

	nativeBuildCalled := false
	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		nativeBuildCalled = true
		return "", nil
	}
	// Even with substitution enabled, the native Taproot builder must not run for
	// a legacy (non-Taproot) transaction.
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

	if nativeBuildCalled {
		t.Fatal(
			"native BuildTaprootTx must not be invoked for a legacy (non-Taproot) transaction",
		)
	}
	if len(tx.Inputs[0].SignatureScript) == 0 {
		t.Fatal("expected the legacy transaction to be signed via the Go path")
	}
	if containsLoggedMessage(
		logger.infoMessages,
		"substituted Go unsigned transaction with native tbtc-signer BuildTaprootTx output",
	) {
		t.Fatal("must not substitute a native transaction for a legacy transaction")
	}
}

func TestWalletTransactionExecutor_SignTransaction_SubstitutesNativeBuildForTaprootTransaction(
	t *testing.T,
) {
	unsignedTx, privateKey := buildTaprootKeyPathUnsignedTxForTest(t)

	// The native builder returns a transaction structurally identical to the
	// Go-built one, so substitution mode accepts it and substitutes -- this
	// exercises the ReplaceUnsignedTransaction call and the substitution info
	// log in the real signTransaction caller. Returning a non-empty hex also
	// proves the gate invoked the native build for an all-Taproot transaction.
	nativeUnsignedTxHex := hex.EncodeToString(
		unsignedTx.UnsignedTransaction().Serialize(bitcoin.Standard),
	)

	originalBuildTaprootTxViaNativeSignerFn := buildTaprootTxViaNativeSignerFn
	originalSigningSubstitutionEnabledFn := nativeBuildTaprootTxSigningSubstitutionEnabledFn
	t.Cleanup(func() {
		buildTaprootTxViaNativeSignerFn = originalBuildTaprootTxViaNativeSignerFn
		nativeBuildTaprootTxSigningSubstitutionEnabledFn = originalSigningSubstitutionEnabledFn
	})

	nativeBuildCalled := false
	buildTaprootTxViaNativeSignerFn = func(
		unsignedTx *bitcoin.TransactionBuilder,
	) (string, error) {
		nativeBuildCalled = true
		return nativeUnsignedTxHex, nil
	}
	nativeBuildTaprootTxSigningSubstitutionEnabledFn = func() bool {
		return true
	}

	wte := &walletTransactionExecutor{
		executingWallet: generateWallet(big.NewInt(111)),
		signingExecutor: &deterministicSchnorrSigningExecutorForTaproot{
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

	if !nativeBuildCalled {
		t.Fatal(
			"native BuildTaprootTx must be invoked for an all-Taproot transaction",
		)
	}
	if !containsLoggedMessage(
		logger.infoMessages,
		"substituted Go unsigned transaction with native tbtc-signer BuildTaprootTx output",
	) {
		t.Fatalf("expected the substitution info log, got: [%v]", logger.infoMessages)
	}
	if len(tx.Inputs) != 1 || len(tx.Inputs[0].Witness) == 0 {
		t.Fatal("expected the substituted transaction to be signed with a taproot witness")
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

func TestWalletTransactionExecutor_SignTransaction_AppliesTweakedTaprootKeyPathSignatures(
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

	internalPrivateKeyBytes := mustDecodeHex(
		t,
		"0101010101010101010101010101010101010101010101010101010101010101",
	)
	_, internalPublicKey := btcec2.PrivKeyFromBytes(internalPrivateKeyBytes)

	var internalKey [32]byte
	copy(internalKey[:], schnorr.SerializePubKey(internalPublicKey))

	refundLeaf := bitcoin.Script(mustDecodeHex(
		t,
		"76a9140102030405060708090a0b0c0d0e0f101112131488ac",
	))
	merkleRoot, err := bitcoin.TaprootLeafHash(refundLeaf)
	if err != nil {
		t.Fatalf("cannot compute taproot leaf hash: [%v]", err)
	}

	taprootOutputKey, err := bitcoin.TaprootOutputKey(internalKey, &merkleRoot)
	if err != nil {
		t.Fatalf("cannot derive taproot output key: [%v]", err)
	}

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
					TransactionHash: bitcoin.Hash{0x10},
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
	if err := unsignedTx.AddTaprootKeyPathInputWithMerkleRoot(
		&bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: fundingTransaction.Hash(),
				OutputIndex:     0,
			},
			Value: 100000,
		},
		internalKey,
		merkleRoot,
	); err != nil {
		t.Fatalf("cannot add tweaked taproot input: [%v]", err)
	}
	unsignedTx.AddOutput(&bitcoin.TransactionOutput{
		Value:           90000,
		PublicKeyScript: outputScript,
	})

	tweakedPrivateKeyBytes := mustDecodeHex(
		t,
		"6ba56a44ff544e35d38fd126659aa68b2c4677a7ebbf7464ad2e9d86c18e1149",
	)
	tweakedPrivateKey, _ := btcec2.PrivKeyFromBytes(tweakedPrivateKeyBytes)

	signingExecutor := &taprootMerkleRootRecordingSchnorrSigningExecutor{
		privateKey: tweakedPrivateKey,
	}

	wte := &walletTransactionExecutor{
		executingWallet: generateWallet(big.NewInt(111)),
		signingExecutor: signingExecutor,
		waitForBlockFn: func(ctx context.Context, block uint64) error {
			return nil
		},
	}

	tx, err := wte.signTransaction(&warningCaptureLogger{}, unsignedTx, 0, 0)
	if err != nil {
		t.Fatalf("unexpected signTransaction error: [%v]", err)
	}

	if signingExecutor.signBatchCalled {
		t.Fatal("ordinary signBatch must not be called for tweaked taproot input")
	}

	if len(signingExecutor.taprootMerkleRoots) != 1 {
		t.Fatalf(
			"unexpected taproot merkle root count\nexpected: [%d]\nactual:   [%d]",
			1,
			len(signingExecutor.taprootMerkleRoots),
		)
	}
	if signingExecutor.taprootMerkleRoots[0] == nil {
		t.Fatal("expected taproot merkle root")
	}
	if !bytes.Equal(signingExecutor.taprootMerkleRoots[0][:], merkleRoot[:]) {
		t.Fatalf(
			"unexpected taproot merkle root\nexpected: [%x]\nactual:   [%x]",
			merkleRoot,
			*signingExecutor.taprootMerkleRoots[0],
		)
	}

	if len(tx.Inputs) != 1 {
		t.Fatalf("unexpected input count: [%d]", len(tx.Inputs))
	}
	if len(tx.Inputs[0].Witness) != 1 {
		t.Fatalf("unexpected taproot witness: [%x]", tx.Inputs[0].Witness)
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

// buildTaprootKeyPathUnsignedTxForTest builds an all-Taproot-key-path unsigned
// transaction (and returns the key controlling its single input) for exercising
// the native BuildTaprootTx gate / signing path.
func buildTaprootKeyPathUnsignedTxForTest(
	t *testing.T,
) (*bitcoin.TransactionBuilder, *btcec2.PrivateKey) {
	t.Helper()

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
						0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
						0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
						0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27,
						0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f,
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

	return unsignedTx, privateKey
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

type taprootMerkleRootRecordingSchnorrSigningExecutor struct {
	privateKey         *btcec2.PrivateKey
	signBatchCalled    bool
	taprootMerkleRoots []*[32]byte
}

func (tmrrsse *taprootMerkleRootRecordingSchnorrSigningExecutor) signBatch(
	ctx context.Context,
	messages []*big.Int,
	startBlock uint64,
) ([]*frost.Signature, error) {
	tmrrsse.signBatchCalled = true
	return nil, errors.New("unexpected signBatch invocation")
}

func (tmrrsse *taprootMerkleRootRecordingSchnorrSigningExecutor) signBatchWithTaprootMerkleRoots(
	ctx context.Context,
	messages []*big.Int,
	taprootMerkleRoots []*[32]byte,
	startBlock uint64,
) ([]*frost.Signature, error) {
	tmrrsse.taprootMerkleRoots = make([]*[32]byte, len(taprootMerkleRoots))
	for i, taprootMerkleRoot := range taprootMerkleRoots {
		if taprootMerkleRoot == nil {
			continue
		}

		tmrrsse.taprootMerkleRoots[i] = new([32]byte)
		copy(tmrrsse.taprootMerkleRoots[i][:], taprootMerkleRoot[:])
	}

	signatures := make([]*frost.Signature, 0, len(messages))

	for _, message := range messages {
		signature, err := schnorr.Sign(
			tmrrsse.privateKey,
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

func (tmrrsse *taprootMerkleRootRecordingSchnorrSigningExecutor) usesSchnorrSignatures() bool {
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
