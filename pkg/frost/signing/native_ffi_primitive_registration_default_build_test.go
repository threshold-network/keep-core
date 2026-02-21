//go:build !frost_native

package signing

import "testing"

func TestRegisterNativeExecutionFFISigningPrimitiveForBuild_DefaultBuildNoop(
	t *testing.T,
) {
	UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild()
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	RegisterNativeExecutionFFISigningPrimitiveForBuild()

	if currentNativeExecutionFFIExecutor() != nil {
		t.Fatal("expected no FFI executor registration on default build")
	}
}
