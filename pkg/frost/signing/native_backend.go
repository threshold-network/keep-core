package signing

import (
	"context"
	"fmt"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

const nativeExecutionBackendName = "native-frost-ffi"

// NativeExecutionAdapter is a transitional hook for wiring a future native
// FROST signing implementation (for example, cgo/FFI-backed).
type NativeExecutionAdapter interface {
	Execute(
		ctx context.Context,
		logger log.StandardLogger,
		request *Request,
	) (*Result, error)
	RegisterUnmarshallers(channel net.BroadcastChannel)
}

type nativeExecutionBackend struct {
	adapter NativeExecutionAdapter
}

func newNativeExecutionBackend(
	adapter NativeExecutionAdapter,
) (*nativeExecutionBackend, error) {
	if adapter == nil {
		return nil, fmt.Errorf("native execution adapter is nil")
	}

	return &nativeExecutionBackend{
		adapter: adapter,
	}, nil
}

func (neb *nativeExecutionBackend) Name() string {
	return nativeExecutionBackendName
}

func (neb *nativeExecutionBackend) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	return neb.adapter.Execute(ctx, logger, request)
}

func (neb *nativeExecutionBackend) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	neb.adapter.RegisterUnmarshallers(channel)
}
