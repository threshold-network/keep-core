package tbtc

import (
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

// TestVerifyFrostSigningBackend covers the startup guard that prevents a
// FROST-enabled node from running on the legacy signing backend (which cannot
// sign native FROST wallets and would fail every signature), while leaving nodes
// with FROST disabled unaffected.
func TestVerifyFrostSigningBackend(t *testing.T) {
	t.Run("FROST disabled: legacy backend is accepted", func(t *testing.T) {
		// A node whose chain satisfies FrostDKGChain but has no FROST wallet
		// registry configured must keep working on the default legacy backend.
		frostsigning.ResetExecutionBackend()
		t.Cleanup(frostsigning.ResetExecutionBackend)

		if err := verifyFrostSigningBackend(false); err != nil {
			t.Fatalf("expected no error when FROST is disabled, got [%v]", err)
		}
	})

	t.Run("FROST enabled: legacy backend is rejected", func(t *testing.T) {
		// The default backend is the transitional legacy backend.
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

	t.Run("FROST enabled: native backend is accepted", func(t *testing.T) {
		frostsigning.ResetExecutionBackend()
		frostsigning.UnregisterNativeExecutionAdapter()
		t.Cleanup(frostsigning.ResetExecutionBackend)
		t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

		if err := frostsigning.RegisterNativeExecutionAdapter(
			&noopNativeExecutionAdapter{},
		); err != nil {
			t.Fatalf("failed to register native execution adapter: [%v]", err)
		}
		if err := frostsigning.SetExecutionBackendByName("native"); err != nil {
			t.Fatalf("failed to select the native backend: [%v]", err)
		}

		if err := verifyFrostSigningBackend(true); err != nil {
			t.Fatalf("expected no error for the native backend, got [%v]", err)
		}
	})

	t.Run("FROST enabled: native selected but unavailable is rejected", func(t *testing.T) {
		// The fallback-allowed "native" mode selects a native backend name
		// without verifying availability, so an unavailable native engine must
		// still be rejected rather than silently falling back to legacy.
		frostsigning.ResetExecutionBackend()
		frostsigning.UnregisterNativeExecutionAdapter()
		t.Cleanup(frostsigning.ResetExecutionBackend)
		t.Cleanup(frostsigning.UnregisterNativeExecutionAdapter)

		if err := frostsigning.RegisterNativeExecutionAdapter(
			&unavailableNativeExecutionAdapter{},
		); err != nil {
			t.Fatalf("failed to register native execution adapter: [%v]", err)
		}
		if err := frostsigning.SetExecutionBackendByName("native"); err != nil {
			t.Fatalf("failed to select the native backend: [%v]", err)
		}

		err := verifyFrostSigningBackend(true)
		if err == nil {
			t.Fatal("expected an error when native execution is unavailable, got nil")
		}
	})
}

// unavailableNativeExecutionAdapter is a native adapter that registers
// successfully but reports native execution as unavailable, modelling a build
// where the native tbtc-signer engine is not linked in.
type unavailableNativeExecutionAdapter struct {
	noopNativeExecutionAdapter
}

func (u *unavailableNativeExecutionAdapter) NativeExecutionAvailable() bool {
	return false
}
