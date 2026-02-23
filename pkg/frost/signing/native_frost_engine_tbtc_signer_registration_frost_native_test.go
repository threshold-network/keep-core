//go:build frost_native && frost_tbtc_signer && cgo && !frost_uniffi_sdk

package signing

import (
	"strings"
	"testing"
)

func TestRegisterBuildTaggedTBTCSignerNativeFROSTSigningEngine(t *testing.T) {
	UnregisterNativeFROSTSigningEngine()
	t.Cleanup(func() {
		UnregisterNativeFROSTSigningEngine()
	})

	err := registerBuildTaggedNativeFROSTSigningEngine()
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}

	engine := currentNativeFROSTSigningEngine()
	if engine == nil {
		t.Fatal("expected native FROST signing engine registration")
	}

	_, _, err = engine.GenerateNoncesAndCommitments(&NativeFROSTKeyPackage{
		Identifier: "participant-1",
		Data:       []byte{1, 2, 3},
	})
	if err == nil {
		t.Fatal("expected not-implemented tbtc-signer bridge error")
	}

	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("unexpected bridge error: [%v]", err)
	}
}
