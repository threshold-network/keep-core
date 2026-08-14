//go:build frost_native && frost_tbtc_signer && cgo

package signing

import (
	"errors"
	"testing"
)

// TestTBTCSignerABIPreflight_CompatibleAgainstLinkedLib asserts the linked libfrost_tbtc
// reports an FFI contract version this build accepts. With no/old lib (the abi symbol is
// absent), assertTBTCSignerABICompatible keeps ErrNativeCryptographyUnavailable in the
// chain, so the test skips - matching the rest of the cgo suite. Against a current lib it
// must be compatible; an incompatibility here is a real, fail-loud finding.
func TestTBTCSignerABIPreflight_CompatibleAgainstLinkedLib(t *testing.T) {
	resetTBTCSignerABIOnceForTest()
	t.Cleanup(resetTBTCSignerABIOnceForTest)

	err := assertTBTCSignerABICompatible()
	if errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Skip("libfrost_tbtc not linked or predates frost_tbtc_abi_version")
	}
	if err != nil {
		t.Fatalf("linked libfrost_tbtc must be ABI-compatible with this build: %v", err)
	}

	// ensure caches the same verdict.
	if err := ensureTBTCSignerABICompatible(); err != nil {
		t.Fatalf("cached preflight verdict diverged: %v", err)
	}
}
