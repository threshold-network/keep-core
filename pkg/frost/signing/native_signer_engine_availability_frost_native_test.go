//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"errors"
	"sync"
	"testing"
)

// TestNativeExecutionAvailable verifies that NativeExecutionAvailable requires
// both a registered native signer engine and a passing libfrost_tbtc ABI probe -
// not just the Go wrapper pointer.
//
// The optional-link dev profile compiles this tag set without linking
// libfrost_tbtc (the bridge tolerates it via dlsym). There the ABI probe reports
// unavailable, so the "available" subtest is skipped - native execution is
// genuinely unavailable. The other subtests hold regardless.
func TestNativeExecutionAvailable(t *testing.T) {
	t.Cleanup(func() {
		UnregisterNativeTBTCSignerEngine()
		resetTBTCSignerABIOnceForTest()
	})

	t.Run("no engine registered -> unavailable", func(t *testing.T) {
		UnregisterNativeTBTCSignerEngine()
		resetTBTCSignerABIOnceForTest()
		if NativeExecutionAvailable() {
			t.Fatal("expected false with no native signer engine registered")
		}
	})

	t.Run("engine registered and linked lib ABI-compatible -> available", func(t *testing.T) {
		UnregisterNativeTBTCSignerEngine()
		if err := RegisterNativeTBTCSignerEngine(&mockNativeTBTCSignerEngine{}); err != nil {
			t.Fatalf("failed to register native signer engine: [%v]", err)
		}
		t.Cleanup(UnregisterNativeTBTCSignerEngine)

		resetTBTCSignerABIOnceForTest() // probe the real linked lib
		if ensureTBTCSignerABICompatible() != nil {
			t.Skip("libfrost_tbtc not linked in this build profile; native execution is genuinely unavailable")
		}
		if !NativeExecutionAvailable() {
			t.Fatal("expected true with the engine registered and the linked lib ABI-compatible")
		}
	})

	t.Run("engine registered but ABI probe fails -> unavailable", func(t *testing.T) {
		UnregisterNativeTBTCSignerEngine()
		if err := RegisterNativeTBTCSignerEngine(&mockNativeTBTCSignerEngine{}); err != nil {
			t.Fatalf("failed to register native signer engine: [%v]", err)
		}
		t.Cleanup(func() {
			UnregisterNativeTBTCSignerEngine()
			resetTBTCSignerABIOnceForTest()
		})

		// Force the ABI probe to report failure regardless of the build profile.
		tbtcSignerABIOnce = sync.Once{}
		tbtcSignerABIErr = errors.New("forced: libfrost_tbtc ABI probe failed")
		tbtcSignerABIOnce.Do(func() {}) // mark the once done without overwriting the forced verdict

		if NativeExecutionAvailable() {
			t.Fatal("expected false when the native signer ABI probe fails despite a registered engine")
		}
	})
}
