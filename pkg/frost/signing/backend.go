package signing

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

// ExecutionBackend represents a pluggable backend used by the FROST signing
// runtime. This enables seamless replacement of the transitional legacy engine
// with a native FROST/FFI-backed implementation.
type ExecutionBackend interface {
	Name() string
	Execute(
		ctx context.Context,
		logger log.StandardLogger,
		request *Request,
	) (*Result, error)
	RegisterUnmarshallers(channel net.BroadcastChannel)
}

var (
	executionBackendMutex sync.RWMutex
	executionBackend      ExecutionBackend = newLegacyExecutionBackend()
)

func currentExecutionBackend() ExecutionBackend {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return executionBackend
}

// SetExecutionBackend sets a runtime execution backend.
func SetExecutionBackend(backend ExecutionBackend) error {
	if backend == nil {
		return fmt.Errorf("execution backend is nil")
	}

	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	executionBackend = backend
	return nil
}

// ResetExecutionBackend restores the default transitional legacy backend.
func ResetExecutionBackend() {
	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	executionBackend = newLegacyExecutionBackend()
}

// CurrentExecutionBackendName returns the active backend name.
func CurrentExecutionBackendName() string {
	return currentExecutionBackend().Name()
}
