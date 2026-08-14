//go:build !(frost_native && frost_tbtc_signer && cgo)

package signing

import (
	"errors"
	"testing"
)

func TestTriggerNativeTBTCSignerEmergencyRekeyFailsClosedWithoutBridge(
	t *testing.T,
) {
	rekey, err := TriggerNativeTBTCSignerEmergencyRekey("session", "compromise")
	if rekey != nil || !errors.Is(err, ErrNativeCryptographyUnavailable) {
		t.Fatalf("expected unavailable emergency rekey, got [%+v] [%v]", rekey, err)
	}
}
