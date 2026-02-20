package signing

import (
	"context"
	"errors"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

var (
	// ErrNativeCryptographyUnavailable indicates that native FROST
	// cryptographic execution is not linked in the current build.
	//
	// The frost_native adapter handles this condition by falling back to the
	// legacy bridge backend.
	ErrNativeCryptographyUnavailable = errors.New(
		"native FROST cryptographic execution is unavailable",
	)
)

// nativeExecutionBridge defines a native cryptographic execution entrypoint
// used by the frost_native adapter.
//
// The current implementation returns ErrNativeCryptographyUnavailable. Future
// FFI-backed integrations should provide an available bridge implementation.
type nativeExecutionBridge interface {
	IsAvailable() bool
	Execute(
		ctx context.Context,
		logger log.StandardLogger,
		request *Request,
	) (*Result, error)
	RegisterUnmarshallers(channel net.BroadcastChannel)
}

func newNativeExecutionBridge() nativeExecutionBridge {
	return &unlinkedNativeExecutionBridge{}
}

type unlinkedNativeExecutionBridge struct{}

func (uneb *unlinkedNativeExecutionBridge) IsAvailable() bool {
	return false
}

func (uneb *unlinkedNativeExecutionBridge) Execute(
	ctx context.Context,
	logger log.StandardLogger,
	request *Request,
) (*Result, error) {
	return nil, ErrNativeCryptographyUnavailable
}

func (uneb *unlinkedNativeExecutionBridge) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
}
