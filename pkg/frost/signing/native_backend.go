package signing

import (
	"context"
	"errors"
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

	result, err := neb.adapter.Execute(ctx, logger, request)
	if err != nil &&
		InteractiveSigningOnlyEnabled() &&
		errors.Is(err, ErrNativeCryptographyUnavailable) &&
		!errors.Is(err, ErrTerminalSigningFailure) {
		// Interactive-only mode (coarse-path retirement): the native interactive path
		// could not produce a signature and every coarse/legacy fallback is suppressed,
		// so an outer refusal surfaces as a bare ErrNativeCryptographyUnavailable.
		// Promote it to TERMINAL so the tBTC signingRetryLoop aborts immediately rather
		// than retrying a deterministic configuration failure until timeout.
		return nil, fmt.Errorf(
			"%w: interactive-only signing mode (%s) and the native interactive path is "+
				"unavailable; refusing the coarse fallback: %v",
			ErrTerminalSigningFailure, InteractiveSigningOnlyEnvVar, err,
		)
	}
	return result, err
}

func (neb *nativeExecutionBackend) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	neb.adapter.RegisterUnmarshallers(channel)
}
