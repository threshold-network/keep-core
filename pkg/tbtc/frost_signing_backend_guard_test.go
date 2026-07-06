//go:build frost_native && frost_tbtc_signer && cgo

package tbtc

import (
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

// TestVerifyFrostSigningBackend covers the startup guard that prevents a
// FROST-enabled node from starting unless the real native FROST signer engine is
// linked and usable. The legacy backend cannot sign native FROST wallets, and a
// frost_native build not linked with the native signer still registers
// transitional FFI executor / legacy-delegate wrappers - so a non-legacy backend
// name alone is not sufficient. Nodes with FROST disabled are unaffected.
//
// This file is tagged frost_native && frost_tbtc_signer && cgo because it needs
// to add/drop the real native signer engine to exercise the availability paths.
func TestVerifyFrostSigningBackend(t *testing.T) {
	// Restore the build-tagged native registrations (adapter, FFI executor,
	// signer engine) after the suite so other tests see a linked engine.
	t.Cleanup(func() {
		frostsigning.ResetExecutionBackend()
		frostsigning.RegisterNativeExecutionAdapterForBuild()
	})

	t.Run("FROST disabled: any backend is accepted", func(t *testing.T) {
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

	t.Run("FROST enabled: native with the linked signer engine is accepted", func(t *testing.T) {
		frostsigning.ResetExecutionBackend()
		// Register the build's native adapter + FFI + signer engine.
		frostsigning.RegisterNativeExecutionAdapterForBuild()
		t.Cleanup(func() {
			frostsigning.ResetExecutionBackend()
			frostsigning.RegisterNativeExecutionAdapterForBuild()
		})

		if err := frostsigning.SetExecutionBackendByName("native"); err != nil {
			t.Fatalf("failed to select the native backend: [%v]", err)
		}
		// In the optional-link dev profile libfrost_tbtc is not linked, so native
		// execution is genuinely unavailable and the guard correctly rejects it;
		// skip the positive assertion there.
		if !frostsigning.NativeExecutionAvailable() {
			t.Skip("native signer library not linked in this build profile; native execution is genuinely unavailable")
		}
		if err := verifyFrostSigningBackend(true); err != nil {
			t.Fatalf("expected no error with the native signer engine linked, got [%v]", err)
		}
	})

	t.Run("FROST enabled: native without a linked signer engine is rejected", func(t *testing.T) {
		frostsigning.ResetExecutionBackend()
		frostsigning.RegisterNativeExecutionAdapterForBuild()
		// Drop only the real signer engine, leaving the transitional wrappers -
		// this models a frost_native build not linked with the native signer.
		frostsigning.UnregisterNativeTBTCSignerEngine()
		t.Cleanup(func() {
			frostsigning.ResetExecutionBackend()
			frostsigning.RegisterNativeExecutionAdapterForBuild()
		})

		if err := frostsigning.SetExecutionBackendByName("native"); err != nil {
			t.Fatalf("failed to select the native backend: [%v]", err)
		}
		err := verifyFrostSigningBackend(true)
		if err == nil {
			t.Fatal("expected an error when the native signer engine is not linked, got nil")
		}
	})
}
