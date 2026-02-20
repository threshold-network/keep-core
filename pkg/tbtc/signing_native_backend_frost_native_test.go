//go:build frost_native

package tbtc

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"testing"

	"github.com/ipfs/go-log/v2"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/net"
)

type noopNativeExecutionFFIExecutor struct{}

func (nnefe *noopNativeExecutionFFIExecutor) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *frostsigning.Request,
) (*frostsigning.Result, error) {
	return nil, nil
}

func (nnefe *noopNativeExecutionFFIExecutor) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
}

func TestConfigureFrostSigningBackend_FFIStrictConfigured_BuildAdapter(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	err := frostsigning.RegisterNativeExecutionFFIExecutor(
		&noopNativeExecutionFFIExecutor{},
	)
	if err != nil {
		t.Fatalf("unexpected native FFI executor registration error: [%v]", err)
	}
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
	t.Cleanup(frostsigning.UnregisterNativeExecutionBridge)
	t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

	err = configureFrostSigningBackend(Config{FrostSigningBackend: "ffi"})
	if err != nil {
		t.Fatalf("unexpected strict ffi backend configuration error: [%v]", err)
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.NativeExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.NativeExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}
}

func TestConfigureFrostSigningBackend_FFIStrictUnavailable_NoBridge(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	// Remove build-registered bridge and executor to exercise strict ffi
	// configuration when no native cryptography path is available.
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
	t.Cleanup(frostsigning.UnregisterNativeExecutionBridge)
	t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

	err := configureFrostSigningBackend(Config{FrostSigningBackend: "ffi"})
	if err == nil {
		t.Fatal("expected strict ffi backend configuration error")
	}

	if !errors.Is(err, frostsigning.ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected strict ffi backend error\nexpected: [%v]\nactual:   [%v]",
			frostsigning.ErrNativeExecutionBackendUnavailable,
			err,
		)
	}

	if !errors.Is(err, frostsigning.ErrNativeCryptographyUnavailable) {
		t.Fatalf(
			"unexpected strict ffi native-availability error\nexpected: [%v]\nactual:   [%v]",
			frostsigning.ErrNativeCryptographyUnavailable,
			err,
		)
	}
}

func TestSigningExecutor_Sign_NativeBackend(t *testing.T) {
	executor := setupSigningExecutor(t)

	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	frostsigning.UnregisterNativeExecutionBridge()
	frostsigning.UnregisterNativeExecutionFFIExecutor()
	frostsigning.RegisterNativeExecutionAdapterForBuild()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
	t.Cleanup(frostsigning.UnregisterNativeExecutionBridge)
	t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

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

	// Transitional path note:
	// The current native-tag adapter delegates to legacy tECDSA signing.
	// Switch this verification to Schnorr/BIP-340 once native FROST crypto
	// execution is linked.
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
