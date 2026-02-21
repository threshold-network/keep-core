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

type nativeExecutionAvailabilityReporter interface {
	NativeExecutionAvailable() bool
}

var (
	// ErrNativeExecutionBackendUnavailable is returned when native backend is
	// requested but not linked in the current build.
	ErrNativeExecutionBackendUnavailable = fmt.Errorf(
		"native FROST signing backend is unavailable in this build",
	)

	// executionBackend, nativeExecutionAdapter, registeredNativeExecBridge, and
	// nativeExecutionFFIExecutor are process-global runtime state. Tests
	// mutating this state must run sequentially; do not use t.Parallel in such
	// tests.
	executionBackendMutex      sync.RWMutex
	executionBackend           ExecutionBackend = newLegacyExecutionBackend()
	nativeExecutionAdapter     NativeExecutionAdapter
	registeredNativeExecBridge NativeExecutionBridge
	nativeExecutionFFIExecutor NativeExecutionFFIExecutor
	nativeExecutionMode        = nativeExecutionModeFallbackAllowed
)

// LegacyExecutionBackendName is a stable identifier of the transitional
// legacy tECDSA bridge backend.
const LegacyExecutionBackendName = legacyExecutionBackendName

// NativeExecutionBackendName is a stable identifier of the native FROST
// execution backend.
const NativeExecutionBackendName = nativeExecutionBackendName

type nativeExecutionModeValue uint8

const (
	// nativeExecutionModeFallbackAllowed means the native adapter may fall back
	// to transitional legacy execution when native cryptography is unavailable.
	nativeExecutionModeFallbackAllowed nativeExecutionModeValue = iota
	// nativeExecutionModeStrict requires native cryptographic execution and
	// does not allow fallback to transitional legacy execution.
	nativeExecutionModeStrict
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
	nativeExecutionMode = nativeExecutionModeFallbackAllowed
}

// CurrentExecutionBackendName returns the active backend name.
func CurrentExecutionBackendName() string {
	return currentExecutionBackend().Name()
}

// SetExecutionBackendByName configures the runtime backend by a stable name.
//
// Supported values:
//   - "", "legacy", "legacy-tecdsa-bridge": transitional legacy bridge backend
//   - "native": native route with transitional fallback to legacy when native
//     cryptography is unavailable
//   - "ffi": strict native route; no fallback to legacy execution
func SetExecutionBackendByName(name string) error {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "legacy", legacyExecutionBackendName:
		ResetExecutionBackend()
		return nil
	case "native":
		previousMode := currentNativeExecutionMode()
		setNativeExecutionMode(nativeExecutionModeFallbackAllowed)

		nativeBackend, err := currentNativeExecutionBackend()
		if err != nil {
			setNativeExecutionMode(previousMode)
			return err
		}

		if err := SetExecutionBackend(nativeBackend); err != nil {
			setNativeExecutionMode(previousMode)
			return err
		}

		return nil
	case "ffi":
		previousMode := currentNativeExecutionMode()
		setNativeExecutionMode(nativeExecutionModeStrict)

		nativeBackend, err := currentNativeExecutionBackend()
		if err != nil {
			setNativeExecutionMode(previousMode)
			return err
		}

		if err := SetExecutionBackend(nativeBackend); err != nil {
			setNativeExecutionMode(previousMode)
			return err
		}

		return nil
	default:
		return fmt.Errorf("unknown FROST signing backend: [%s]", name)
	}
}

func currentNativeExecutionMode() nativeExecutionModeValue {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return nativeExecutionMode
}

func setNativeExecutionMode(mode nativeExecutionModeValue) {
	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeExecutionMode = mode
}

func nativeExecutionFallbackAllowed() bool {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return nativeExecutionMode == nativeExecutionModeFallbackAllowed
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

// RegisterNativeExecutionAdapterForBuild attempts to register the native
// adapter provided by the current build flavor.
//
// On default builds, this is a no-op.
// On `frost_native` builds, this registers the tagged native adapter.
func RegisterNativeExecutionAdapterForBuild() {
	registerNativeExecutionAdapterForBuild()
	RegisterNativeExecutionFFISigningPrimitiveForBuild()
}

func currentNativeExecutionBackend() (ExecutionBackend, error) {
	executionBackendMutex.RLock()
	adapter := nativeExecutionAdapter
	mode := nativeExecutionMode
	executionBackendMutex.RUnlock()

	if adapter == nil {
		return nil, fmt.Errorf(
			"%w: no native execution adapter registered",
			ErrNativeExecutionBackendUnavailable,
		)
	}

	if mode == nativeExecutionModeStrict {
		if reporter, ok := adapter.(nativeExecutionAvailabilityReporter); ok {
			if !reporter.NativeExecutionAvailable() {
				return nil, fmt.Errorf(
					"%w: %w",
					ErrNativeExecutionBackendUnavailable,
					ErrNativeCryptographyUnavailable,
				)
			}
		}
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
