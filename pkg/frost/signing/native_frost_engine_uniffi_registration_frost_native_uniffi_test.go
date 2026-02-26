//go:build frost_native && frost_uniffi_sdk && cgo && frost_uniffi_legacy

package signing

import "testing"

func TestRegisterBuildTaggedNativeFROSTSigningEngine_UniFFILegacyNoop(
	t *testing.T,
) {
	err := registerBuildTaggedNativeFROSTSigningEngine()
	if err != nil {
		t.Fatalf("unexpected registration error: [%v]", err)
	}
}
