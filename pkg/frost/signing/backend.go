package signing

import (
	"context"
	"fmt"
	"strings"
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
	// ErrNativeExecutionBackendUnavailable is returned when native backend is
	// requested but not linked in the current build.
	ErrNativeExecutionBackendUnavailable = fmt.Errorf(
		"native FROST signing backend is unavailable in this build",
	)

	executionBackendMutex sync.RWMutex
	executionBackend      ExecutionBackend = newLegacyExecutionBackend()
)

// LegacyExecutionBackendName is a stable identifier of the transitional
// legacy tECDSA bridge backend.
const LegacyExecutionBackendName = legacyExecutionBackendName

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

// SetExecutionBackendByName configures the runtime backend by a stable name.
//
// Supported values:
//   - "", "legacy", "legacy-tecdsa-bridge": transitional legacy bridge backend
//   - "native", "ffi": reserved for native FROST backend (currently unavailable)
func SetExecutionBackendByName(name string) error {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "legacy", legacyExecutionBackendName:
		ResetExecutionBackend()
		return nil
	case "native", "ffi":
		return ErrNativeExecutionBackendUnavailable
	default:
		return fmt.Errorf("unknown FROST signing backend: [%s]", name)
	}
}
