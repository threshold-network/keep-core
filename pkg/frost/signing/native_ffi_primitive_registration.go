package signing

import "fmt"

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
func RegisterNativeExecutionFFISigningPrimitiveForBuild() {
	err := registerNativeExecutionFFISigningPrimitiveForBuild()
	if err != nil {
		panic(fmt.Sprintf(
			"failed to register build-tagged native FFI signing primitive: [%v]",
			err,
		))
	}
}
