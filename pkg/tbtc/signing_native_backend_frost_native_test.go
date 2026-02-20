//go:build frost_native

package tbtc

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func TestSigningExecutor_Sign_NativeBackend(t *testing.T) {
	executor := setupSigningExecutor(t)

	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

	err := configureFrostSigningBackend(Config{FrostSigningBackend: "native"})
	if err != nil {
		t.Fatalf("unexpected native backend config error: [%v]", err)
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	message := big.NewInt(100)
	startBlock := uint64(0)

	signature, _, endBlock, err := executor.sign(ctx, message, startBlock)
	if err != nil {
		t.Fatalf("unexpected native backend signing error: [%v]", err)
	}

	walletPublicKey := executor.wallet().publicKey
	if !ecdsa.Verify(
		walletPublicKey,
		message.Bytes(),
		new(big.Int).SetBytes(signature.R[:]),
		new(big.Int).SetBytes(signature.S[:]),
	) {
		t.Fatalf("invalid signature: [%+v]", signature)
	}

	if endBlock <= startBlock {
		t.Fatal("wrong end block")
	}
}
