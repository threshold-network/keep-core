package tbtc

import (
	"context"
	"errors"
	"testing"

	"github.com/ipfs/go-log/v2"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/net"
)

type noopNativeExecutionAdapter struct{}

func (nnea *noopNativeExecutionAdapter) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *frostsigning.Request,
) (*frostsigning.Result, error) {
	return nil, nil
}

func (nnea *noopNativeExecutionAdapter) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
}

func TestConfigureFrostSigningBackend_Default(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

	err := configureFrostSigningBackend(Config{})
	if err != nil {
		t.Fatalf("unexpected config error: [%v]", err)
	}

	if frostsigning.CurrentExecutionBackendName() != frostsigning.LegacyExecutionBackendName {
		t.Fatalf(
			"unexpected backend name\nexpected: [%s]\nactual:   [%s]",
			frostsigning.LegacyExecutionBackendName,
			frostsigning.CurrentExecutionBackendName(),
		)
	}
}

func TestConfigureFrostSigningBackend_NativeUnavailable(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

	err := configureFrostSigningBackend(Config{FrostSigningBackend: "native"})
	if err == nil {
		t.Fatal("expected native backend config error")
	}

	if !errors.Is(err, frostsigning.ErrNativeExecutionBackendUnavailable) {
		t.Fatalf(
			"unexpected error\nexpected: [%v]\nactual:   [%v]",
			frostsigning.ErrNativeExecutionBackendUnavailable,
			err,
		)
	}
}

func TestConfigureFrostSigningBackend_NativeRegistered(t *testing.T) {
	frostsigning.ResetExecutionBackend()
	frostsigning.UnregisterNativeExecutionAdapter()
	t.Cleanup(frostsigning.ResetExecutionBackend)
	t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

	err := frostsigning.RegisterNativeExecutionAdapter(&noopNativeExecutionAdapter{})
	if err != nil {
		t.Fatalf("unexpected native adapter registration error: [%v]", err)
	}

	err = configureFrostSigningBackend(Config{FrostSigningBackend: "native"})
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
}
