package tbtc

import (
	"errors"
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
