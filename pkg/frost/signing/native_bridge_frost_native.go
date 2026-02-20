//go:build frost_native

package signing

import (
	"context"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

// buildTaggedNativeExecutionBridge is a transitional native bridge registered
// for frost_native builds.
//
// Until a real FFI-backed bridge is linked, this bridge delegates to the
// legacy signing backend while still surfacing native-bridge availability.
type buildTaggedNativeExecutionBridge struct {
	delegate ExecutionBackend
}

func newBuildTaggedNativeExecutionBridge() NativeExecutionBridge {
	return &buildTaggedNativeExecutionBridge{
		delegate: newLegacyExecutionBackend(),
	}
}

func (btneb *buildTaggedNativeExecutionBridge) IsAvailable() bool {
	return btneb.delegate != nil
}

func (btneb *buildTaggedNativeExecutionBridge) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	if btneb.delegate == nil {
		return nil, ErrNativeCryptographyUnavailable
	}

	return btneb.delegate.Execute(ctx, logger, request)
}

func (btneb *buildTaggedNativeExecutionBridge) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	if btneb.delegate == nil {
		return
	}

	btneb.delegate.RegisterUnmarshallers(channel)
}
