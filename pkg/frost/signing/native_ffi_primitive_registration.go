package signing

import (
	"fmt"
	"sync"

	"github.com/ipfs/go-log/v2"
)

var (
	registrationLogger    = log.Logger("keep-frost-signing-registration")
	registrationErrorMu   sync.RWMutex
	lastRegistrationError error
)

func setLastRegistrationError(err error) {
	registrationErrorMu.Lock()
	defer registrationErrorMu.Unlock()
	lastRegistrationError = err
}

// LastNativeRegistrationError returns the most recent error observed while
// registering build-tagged native FROST execution adapters or FFI signing
// primitives. It is nil when the most recent registration attempt succeeded
// or when no registration has been attempted yet. Callers that want to fail
// startup on a registration error should check this after invoking
// `RegisterNativeExecutionAdapterForBuild` rather than relying on the
// previously panicking registration helpers themselves.
func LastNativeRegistrationError() error {
	registrationErrorMu.RLock()
	defer registrationErrorMu.RUnlock()
	return lastRegistrationError
}

// NativeExecutionFFISigningPrimitiveProviderForBuild produces a native FFI
// signing primitive for the current build/runtime flavor.
type NativeExecutionFFISigningPrimitiveProviderForBuild func() (
	NativeExecutionFFISigningPrimitive,
	error,
)

// RegisterNativeExecutionFFISigningPrimitiveProviderForBuild registers
// build-scoped primitive provider used by
// RegisterNativeExecutionFFISigningPrimitiveForBuild.
func RegisterNativeExecutionFFISigningPrimitiveProviderForBuild(
	provider NativeExecutionFFISigningPrimitiveProviderForBuild,
) error {
	if provider == nil {
		return fmt.Errorf("native execution FFI signing primitive provider is nil")
	}

	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeExecutionFFISigningPrimitiveProviderForBuild = provider

	return nil
}

// UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild clears
// build-scoped primitive provider registration.
func UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild() {
	executionBackendMutex.Lock()
	defer executionBackendMutex.Unlock()

	nativeExecutionFFISigningPrimitiveProviderForBuild = nil
}

func currentNativeExecutionFFISigningPrimitiveProviderForBuild() NativeExecutionFFISigningPrimitiveProviderForBuild {
	executionBackendMutex.RLock()
	defer executionBackendMutex.RUnlock()

	return nativeExecutionFFISigningPrimitiveProviderForBuild
}

// RegisterNativeExecutionFFISigningPrimitiveForBuild attempts to register
// build-flavor native FFI signing primitive bindings.
//
// On default builds, this is a no-op.
// On `frost_native` builds, this can be wired to a concrete primitive.
//
// Registration errors are surfaced via `LastNativeRegistrationError()` rather
// than panicking, so a transient FFI lookup failure at init time does not
// crash the binary. Downstream code in `pkg/frost/signing/backend.go` already
// handles the absence of a registered native adapter through
// `ErrNativeCryptographyUnavailable`, so the legacy execution backend remains
// the safe-by-default path even when this registration fails.
func RegisterNativeExecutionFFISigningPrimitiveForBuild() {
	err := registerNativeExecutionFFISigningPrimitiveForBuild()
	if err != nil {
		registrationLogger.Warnf(
			"failed to register build-tagged native FFI signing primitive: [%v]; "+
				"the native execution backend will report unavailable and callers "+
				"that selected the legacy or native-with-fallback backend will "+
				"continue using the legacy bridge",
			err,
		)
		setLastRegistrationError(fmt.Errorf(
			"failed to register build-tagged native FFI signing primitive: [%w]",
			err,
		))
		return
	}

	setLastRegistrationError(nil)
}
