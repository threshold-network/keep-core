package tbtc

import (
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

// TestVerifyFrostSigningBackend covers the startup guard that prevents a
// FROST-enabled node from starting unless the real native FROST signing engine
// is usable. The legacy backend cannot sign native FROST wallets, and the
// fallback-allowed "native" mode can select a native backend name without a real
// FFI engine present - both must fail fast rather than at signing time. Nodes
// with FROST disabled are unaffected.
//
// noopNativeExecutionAdapter (from node_signing_backend_test.go) satisfies both
// the native adapter and the FFI executor interfaces, so it is reused as a
// stand-in native FFI engine.
func TestVerifyFrostSigningBackend(t *testing.T) {
	t.Run("FROST disabled: legacy backend is accepted", func(t *testing.T) {
		frostsigning.ResetExecutionBackend()
		t.Cleanup(frostsigning.ResetExecutionBackend)

		if err := verifyFrostSigningBackend(false); err != nil {
			t.Fatalf("expected no error when FROST is disabled, got [%v]", err)
		}
	})

	t.Run("FROST enabled: legacy backend is rejected", func(t *testing.T) {
		frostsigning.ResetExecutionBackend()
		t.Cleanup(frostsigning.ResetExecutionBackend)

		err := verifyFrostSigningBackend(true)
		if err == nil {
			t.Fatal("expected an error for the legacy signing backend, got nil")
		}
		if !strings.Contains(err.Error(), "frostSigningBackend") {
			t.Errorf(
				"error should point at the tbtc.frostSigningBackend config: [%v]",
				err,
			)
		}
	})

	t.Run("FROST enabled: native with a registered FFI engine is accepted", func(t *testing.T) {
		frostsigning.ResetExecutionBackend()
		frostsigning.UnregisterNativeExecutionAdapter()
		frostsigning.UnregisterNativeExecutionFFIExecutor()
		t.Cleanup(frostsigning.ResetExecutionBackend)
		t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
		t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

		if err := frostsigning.RegisterNativeExecutionAdapter(
			&noopNativeExecutionAdapter{},
		); err != nil {
			t.Fatalf("failed to register native execution adapter: [%v]", err)
		}
		// The strict availability signal is a registered native FFI executor.
		if err := frostsigning.RegisterNativeExecutionFFIExecutor(
			&noopNativeExecutionAdapter{},
		); err != nil {
			t.Fatalf("failed to register native FFI executor: [%v]", err)
		}
		if err := frostsigning.SetExecutionBackendByName("native"); err != nil {
			t.Fatalf("failed to select the native backend: [%v]", err)
		}

		if err := verifyFrostSigningBackend(true); err != nil {
			t.Fatalf("expected no error when the native FFI engine is registered, got [%v]", err)
		}
	})

	t.Run("FROST enabled: native without an FFI engine is rejected", func(t *testing.T) {
		// The fallback-allowed "native" mode selects a native backend name even
		// with no real FFI engine registered; the guard must still reject it,
		// because signing would fall back to the legacy bridge and fail on native
		// FROST material.
		frostsigning.ResetExecutionBackend()
		frostsigning.UnregisterNativeExecutionAdapter()
		frostsigning.UnregisterNativeExecutionFFIExecutor()
		t.Cleanup(frostsigning.ResetExecutionBackend)
		t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)
		t.Cleanup(frostsigning.UnregisterNativeExecutionFFIExecutor)

		if err := frostsigning.RegisterNativeExecutionAdapter(
			&noopNativeExecutionAdapter{},
		); err != nil {
			t.Fatalf("failed to register native execution adapter: [%v]", err)
		}
		if err := frostsigning.SetExecutionBackendByName("native"); err != nil {
			t.Fatalf("failed to select the native backend: [%v]", err)
		}

		err := verifyFrostSigningBackend(true)
		if err == nil {
			t.Fatal("expected an error when no native FFI engine is registered, got nil")
		}
	})
}
