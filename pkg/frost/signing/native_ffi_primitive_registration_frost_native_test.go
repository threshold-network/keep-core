//go:build frost_native

package signing

import (
	"errors"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost"
)

func TestRegisterNativeExecutionFFISigningPrimitiveForBuild_UsesProvider(
	t *testing.T,
) {
	UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild()
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	err := RegisterNativeExecutionFFISigningPrimitiveProviderForBuild(
		func() (NativeExecutionFFISigningPrimitive, error) {
			return &mockNativeExecutionFFISigningPrimitive{
				signature: &frost.Signature{},
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected provider registration error: [%v]", err)
	}

	RegisterNativeExecutionFFISigningPrimitiveForBuild()

	if currentNativeExecutionFFIExecutor() == nil {
		t.Fatal("expected FFI executor registration from build provider")
	}
}

func TestRegisterNativeExecutionFFISigningPrimitiveForBuild_ProviderErrorPanics(
	t *testing.T,
) {
	UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild()
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)

	expectedErr := errors.New("provider error")

	err := RegisterNativeExecutionFFISigningPrimitiveProviderForBuild(
		func() (NativeExecutionFFISigningPrimitive, error) {
			return nil, expectedErr
		},
	)
	if err != nil {
		t.Fatalf("unexpected provider registration error: [%v]", err)
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic")
		}

		recoveredError, ok := recovered.(string)
		if !ok {
			t.Fatalf("unexpected panic type: [%T]", recovered)
		}

		if !strings.Contains(recoveredError, expectedErr.Error()) {
			t.Fatalf(
				"unexpected panic value\nexpected substring: [%s]\nactual:             [%v]",
				expectedErr.Error(),
				recovered,
			)
		}
	}()

	RegisterNativeExecutionFFISigningPrimitiveForBuild()
}
