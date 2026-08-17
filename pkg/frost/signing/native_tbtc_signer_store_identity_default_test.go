//go:build !(frost_native && frost_tbtc_signer && cgo)

package signing

import (
	"errors"
	"testing"
)

func TestReadNativeTBTCSignerDurableStoreIdentityFailsClosedWithoutBridge(
	t *testing.T,
) {
	identity, err := ReadNativeTBTCSignerDurableStoreIdentity()
	if identity != nil || !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf("expected unavailable identity readback, got [%+v] [%v]", identity, err)
	}
}
