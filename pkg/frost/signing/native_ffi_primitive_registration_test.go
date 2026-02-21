package signing

import (
	"strings"
	"testing"
)

func TestRegisterNativeExecutionFFISigningPrimitiveProviderForBuild_Nil(
	t *testing.T,
) {
	err := RegisterNativeExecutionFFISigningPrimitiveProviderForBuild(nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(
		err.Error(),
		"native execution FFI signing primitive provider is nil",
	) {
		t.Fatalf(
			"unexpected error\nexpected substring: [%s]\nactual:             [%v]",
			"native execution FFI signing primitive provider is nil",
			err,
		)
	}
}

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
