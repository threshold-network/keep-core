//go:build frost_native

package signing

import "fmt"

func registerNativeExecutionFFISigningPrimitiveForBuild() error {
	provider := currentNativeExecutionFFISigningPrimitiveProviderForBuild()
	if provider == nil {
		provider = defaultNativeExecutionFFISigningPrimitiveProviderForBuild
	}

	primitive, err := provider()
	if err != nil {
		return err
	}

	if primitive == nil {
		return fmt.Errorf("native execution FFI signing primitive is nil")
	}

	return RegisterNativeExecutionFFISigningPrimitive(primitive)
}
