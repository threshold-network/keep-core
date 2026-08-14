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
	// ErrNativeBridgeOperationFailed indicates that native cryptographic
	// execution is available but a bridge operation returned a non-success
	// status. This error should not trigger availability fallback.
	ErrNativeBridgeOperationFailed = errors.New(
		"native FROST bridge operation failed",
	)
)

// NativeExecutionBridge defines a native cryptographic execution entrypoint
// used by the frost_native adapter.
//
// The current implementation returns ErrNativeCryptographyUnavailable. Future
// FFI-backed integrations should provide an available bridge implementation.
type NativeExecutionBridge interface {
	IsAvailable() bool
	Execute(
		ctx context.Context,
		logger log.StandardLogger,
		request *Request,
	) (*Result, error)
	RegisterUnmarshallers(channel net.BroadcastChannel)
}

// RegisterNativeExecutionBridge registers a native execution bridge for
// frost_native adapter routing.
func RegisterNativeExecutionBridge(bridge NativeExecutionBridge) error {
	if bridge == nil {
		return errors.New("native execution bridge is nil")
	}

	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	registeredNativeExecBridge = bridge

	return nil
}

// UnregisterNativeExecutionBridge clears the registered native execution
// bridge.
func UnregisterNativeExecutionBridge() {
	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	registeredNativeExecBridge = nil
}

func currentNativeExecutionBridge() NativeExecutionBridge {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return registeredNativeExecBridge
}

func newNativeExecutionBridge() NativeExecutionBridge {
	bridge := currentNativeExecutionBridge()
	if bridge != nil {
		return bridge
	}

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
