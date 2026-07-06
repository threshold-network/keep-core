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
func TestNativeExecutionAvailable(t *testing.T) {
	t.Cleanup(func() {
		UnregisterNativeTBTCSignerEngine()
		resetTBTCSignerABIOnceForTest()
	})

	// 1. No engine registered -> unavailable.
	UnregisterNativeTBTCSignerEngine()
	resetTBTCSignerABIOnceForTest()
	if NativeExecutionAvailable() {
		t.Fatal("expected false with no native signer engine registered")
	}

	// 2. Engine registered and the linked lib passes the ABI probe -> available.
	if err := RegisterNativeTBTCSignerEngine(&mockNativeTBTCSignerEngine{}); err != nil {
		t.Fatalf("failed to register native signer engine: [%v]", err)
	}
	resetTBTCSignerABIOnceForTest() // re-probe the real linked lib (compatible in this build)
	if !NativeExecutionAvailable() {
		t.Fatal("expected true with the engine registered and the linked lib ABI-compatible")
	}

	// 3. Engine registered but the ABI probe fails (lib missing/incompatible) ->
	// unavailable, even though the Go wrapper pointer is non-nil.
	tbtcSignerABIOnce = sync.Once{}
	tbtcSignerABIErr = errors.New("forced: libfrost_tbtc ABI probe failed")
	tbtcSignerABIOnce.Do(func() {}) // mark the once done without overwriting the forced verdict
	if NativeExecutionAvailable() {
		t.Fatal("expected false when the native signer ABI probe fails despite a registered engine")
	}
}
