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

func TestRegisterNativeExecutionFFISigningPrimitiveForBuild_UsesDefaultProvider(
	t *testing.T,
) {
	UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild()
	UnregisterNativeExecutionFFIExecutor()
	// Under the cgo build the default provider's registration installs the
	// interactive signing provider as a side effect, so reset it here too -
	// symmetric with the FFI executor/provider - both before (clean slate) and in
	// cleanup. Otherwise a later test asserting no interactive provider is set
	// (TestRegisterInteractiveSigningEngineProvider) fails under -shuffle or a
	// focused -run. Resetting is a no-op on builds that register no provider.
	ResetInteractiveSigningEngineProviderForTest()
	t.Cleanup(UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)
	t.Cleanup(ResetInteractiveSigningEngineProviderForTest)

	RegisterNativeExecutionFFISigningPrimitiveForBuild()

	if currentNativeExecutionFFIExecutor() == nil {
		t.Fatal("expected FFI executor registration from default build provider")
	}
}

func TestRegisterNativeExecutionFFISigningPrimitiveForBuild_ProviderErrorIsRecordedNotPanicked(
	t *testing.T,
) {
	UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild()
	UnregisterNativeExecutionFFIExecutor()
	t.Cleanup(UnregisterNativeExecutionFFISigningPrimitiveProviderForBuild)
	t.Cleanup(UnregisterNativeExecutionFFIExecutor)
	t.Cleanup(func() { setLastRegistrationError(nil) })

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
		if recovered := recover(); recovered != nil {
			t.Fatalf(
				"registration must not panic; recovered: [%v]",
				recovered,
			)
		}
	}()

	// Pre-condition: the registration error slot is clear before invoking the
	// helper, so any non-nil error after the call is from this attempt.
	setLastRegistrationError(nil)

	RegisterNativeExecutionFFISigningPrimitiveForBuild()

	registered := LastNativeRegistrationError()
	if registered == nil {
		t.Fatal("expected LastNativeRegistrationError to surface the provider error")
	}
	if !strings.Contains(registered.Error(), expectedErr.Error()) {
		t.Fatalf(
			"LastNativeRegistrationError missing expected substring\nexpected: [%s]\nactual:   [%v]",
			expectedErr.Error(),
			registered,
		)
	}

	if currentNativeExecutionFFIExecutor() != nil {
		t.Fatal(
			"FFI executor must not be registered when the provider returned an error",
		)
	}
}
