package tbtc

import (
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

// TestVerifyFrostSigningBackendForFrost covers the startup guard that prevents a
// FROST-enabled node from running on the legacy signing backend (which cannot
// sign native FROST wallets and would fail every signature).
func TestVerifyFrostSigningBackendForFrost(t *testing.T) {
	t.Run("legacy backend is rejected", func(t *testing.T) {
		// The default backend is the transitional legacy backend.
		frostsigning.ResetExecutionBackend()
		t.Cleanup(frostsigning.ResetExecutionBackend)

		err := verifyFrostSigningBackendForFrost()
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

	t.Run("native backend is accepted", func(t *testing.T) {
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

		if err := verifyFrostSigningBackendForFrost(); err != nil {
			t.Fatalf("expected no error for the native backend, got [%v]", err)
		}
	})
}
