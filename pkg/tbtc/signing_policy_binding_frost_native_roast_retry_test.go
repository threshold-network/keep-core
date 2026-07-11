//go:build frost_native && frost_roast_retry

package tbtc

import (
	"errors"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

func TestBindTaprootPolicyArtifactForSigningUsesStableRoastSession(t *testing.T) {
	original := bindTaprootTxViaNativeSignerFn
	defer func() { bindTaprootTxViaNativeSignerFn = original }()

	message := new(big.Int).SetBytes([]byte{0x11, 0x22, 0x33})
	merkleRoot := &[32]byte{0x44}
	startBlock := uint64(12345)
	keyGroupID := "policy-binding-key-group"
	unsignedTx := bitcoin.NewTransactionBuilder(nil)
	inputIndex := 2

	var capturedSessionID string
	bindTaprootTxViaNativeSignerFn = func(
		sessionID string,
		capturedTx *bitcoin.TransactionBuilder,
		capturedInputIndex int,
		capturedMessage *big.Int,
	) error {
		capturedSessionID = sessionID
		if capturedTx != unsignedTx {
			t.Fatal("binding received a different transaction builder")
		}
		if capturedInputIndex != inputIndex {
			t.Fatalf("binding input index = %d, want %d", capturedInputIndex, inputIndex)
		}
		if capturedMessage.Cmp(message) != 0 {
			t.Fatalf("binding message = %x, want %x", capturedMessage, message)
		}
		return nil
	}

	sessionID, err := bindTaprootPolicyArtifactForSigning(
		message,
		merkleRoot,
		startBlock,
		keyGroupID,
		unsignedTx,
		inputIndex,
	)
	if err != nil {
		t.Fatalf("unexpected binding error: %v", err)
	}

	expectedSessionID := roastSessionID(message, merkleRoot, startBlock, keyGroupID)
	if sessionID != expectedSessionID || capturedSessionID != expectedSessionID {
		t.Fatalf(
			"policy artifact session mismatch: returned=%q captured=%q expected=%q",
			sessionID,
			capturedSessionID,
			expectedSessionID,
		)
	}
}

func TestBindTaprootPolicyArtifactForSigningPropagatesFailure(t *testing.T) {
	original := bindTaprootTxViaNativeSignerFn
	defer func() { bindTaprootTxViaNativeSignerFn = original }()

	expectedErr := errors.New("policy artifact rejected")
	bindTaprootTxViaNativeSignerFn = func(
		string,
		*bitcoin.TransactionBuilder,
		int,
		*big.Int,
	) error {
		return expectedErr
	}

	sessionID, err := bindTaprootPolicyArtifactForSigning(
		big.NewInt(1),
		nil,
		1,
		"policy-binding-key-group",
		bitcoin.NewTransactionBuilder(nil),
		0,
	)
	if sessionID != "" {
		t.Fatalf("failed binding returned session ID %q", sessionID)
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("binding error = %v, want %v", err, expectedErr)
	}
}

func TestBindTaprootPolicyArtifactForSigningRejectsMissingKeyGroup(t *testing.T) {
	called := false
	original := bindTaprootTxViaNativeSignerFn
	bindTaprootTxViaNativeSignerFn = func(
		string,
		*bitcoin.TransactionBuilder,
		int,
		*big.Int,
	) error {
		called = true
		return nil
	}
	defer func() { bindTaprootTxViaNativeSignerFn = original }()

	_, err := bindTaprootPolicyArtifactForSigning(
		big.NewInt(1),
		nil,
		1,
		"",
		bitcoin.NewTransactionBuilder(nil),
		0,
	)
	if err == nil {
		t.Fatal("transaction policy binding without a key group must fail closed")
	}
	if called {
		t.Fatal("native binding must not run with an empty key group")
	}
}
