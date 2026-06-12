//go:build !(frost_native && frost_tbtc_signer && cgo)

package signing

import (
	"errors"
	"testing"
)

func TestInstallNativeTBTCSignerConfig_UnavailableWithoutBridge(t *testing.T) {
	result, err := InstallNativeTBTCSignerConfig([]byte(`{}`))
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf("expected ErrNativeCryptographyUnavailable, got %v", err)
	}
}
