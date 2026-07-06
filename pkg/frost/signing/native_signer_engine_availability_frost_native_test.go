//go:build frost_native

package signing

import "testing"

// TestNativeExecutionAvailable verifies that NativeExecutionAvailable tracks the
// real native tbtc-signer engine registration, not the transitional wrappers.
func TestNativeExecutionAvailable(t *testing.T) {
	// Start from a clean slate: no engine registered.
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(UnregisterNativeTBTCSignerEngine)

	if NativeExecutionAvailable() {
		t.Fatal("expected NativeExecutionAvailable() to be false with no native signer engine registered")
	}

	if err := RegisterNativeTBTCSignerEngine(&mockNativeTBTCSignerEngine{}); err != nil {
		t.Fatalf("failed to register native signer engine: [%v]", err)
	}
	if !NativeExecutionAvailable() {
		t.Fatal("expected NativeExecutionAvailable() to be true once the native signer engine is registered")
	}
}
