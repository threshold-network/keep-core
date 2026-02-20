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
	// ErrNativeExecutionBackendNotImplemented is returned when native backend
	// can be selected but does not provide a cryptographic execution engine yet.
	ErrNativeExecutionBackendNotImplemented = fmt.Errorf(
		"native FROST signing backend is not implemented",
	)

	executionBackendMutex  sync.RWMutex
	executionBackend       ExecutionBackend = newLegacyExecutionBackend()
	nativeExecutionAdapter NativeExecutionAdapter
)

// LegacyExecutionBackendName is a stable identifier of the transitional
// legacy tECDSA bridge backend.
const LegacyExecutionBackendName = legacyExecutionBackendName

// NativeExecutionBackendName is a stable identifier of the native FROST
// execution backend.
const NativeExecutionBackendName = nativeExecutionBackendName

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
//   - "native", "ffi": native FROST backend (requires registered native adapter)
func SetExecutionBackendByName(name string) error {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "legacy", legacyExecutionBackendName:
		ResetExecutionBackend()
		return nil
	case "native", "ffi":
		nativeBackend, err := currentNativeExecutionBackend()
		if err != nil {
			return err
		}

		return SetExecutionBackend(nativeBackend)
	default:
		return fmt.Errorf("unknown FROST signing backend: [%s]", name)
	}
}

// RegisterNativeExecutionAdapter sets a native adapter used by the
// native FROST execution backend.
func RegisterNativeExecutionAdapter(adapter NativeExecutionAdapter) error {
	if adapter == nil {
		return fmt.Errorf("native execution adapter is nil")
	}

	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeExecutionAdapter = adapter

	return nil
}

// UnregisterNativeExecutionAdapter clears the native adapter registration.
func UnregisterNativeExecutionAdapter() {
	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeExecutionAdapter = nil
}

func currentNativeExecutionBackend() (ExecutionBackend, error) {
	executionBackendMutex.RLock()
	adapter := nativeExecutionAdapter
	executionBackendMutex.RUnlock()

	if adapter == nil {
		return nil, fmt.Errorf(
			"%w: no native execution adapter registered",
			ErrNativeExecutionBackendUnavailable,
		)
	}

	backend, err := newNativeExecutionBackend(adapter)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: [%v]",
			ErrNativeExecutionBackendUnavailable,
			err,
		)
	}

	return backend, nil
}
