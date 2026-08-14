package signing

import (
	"context"
	"errors"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
)

// NativeExecutionFFIExecutor is a bridge to the native/FFI signing engine.
// This executor is intended to run FROST-native cryptographic execution.
type NativeExecutionFFIExecutor interface {
	Execute(
		ctx context.Context,
		logger log.StandardLogger,
		request *Request,
	) (*Result, error)
	RegisterUnmarshallers(channel net.BroadcastChannel)
}

// RegisterNativeExecutionFFIExecutor registers a native FFI executor used by
// build-tagged bridges.
func RegisterNativeExecutionFFIExecutor(executor NativeExecutionFFIExecutor) error {
	if executor == nil {
		return errors.New("native execution FFI executor is nil")
	}

	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeExecutionFFIExecutor = executor

	return nil
}

// UnregisterNativeExecutionFFIExecutor clears the native FFI executor
// registration.
func UnregisterNativeExecutionFFIExecutor() {
	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeExecutionFFIExecutor = nil
}

func currentNativeExecutionFFIExecutor() NativeExecutionFFIExecutor {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return nativeExecutionFFIExecutor
}
