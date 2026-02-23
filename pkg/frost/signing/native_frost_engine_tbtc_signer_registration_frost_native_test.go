//go:build frost_native && frost_tbtc_signer && cgo && !frost_uniffi_sdk

package signing

import (
	"strings"
	"testing"
)

func TestRegisterBuildTaggedTBTCSignerEngine(t *testing.T) {
	UnregisterNativeTBTCSignerEngine()
	t.Cleanup(func() {
		UnregisterNativeTBTCSignerEngine()
	})

	err := registerBuildTaggedNativeFROSTSigningEngine()
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	engine := currentNativeTBTCSignerEngine()
	if engine == nil {
		t.Fatal("expected native tbtc-signer engine registration")
	}

	_, err = engine.StartSignRound(
		"session-1",
		[]byte("message"),
		"key-group",
	)
	if err == nil {
		t.Fatal("expected not-implemented tbtc-signer bridge error")
	}

	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("unexpected bridge error: [%v]", err)
	}
}
