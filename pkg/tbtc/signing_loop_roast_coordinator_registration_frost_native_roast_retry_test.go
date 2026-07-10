//go:build frost_native && frost_roast_retry

package tbtc

import (
	"encoding/json"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/signing"
)

// TestRegistrationKeyGroupIDForSigner_NormalizesMaterialForms is the Codex P2
// regression: registrationKeyGroupIDForSigner must accept the same native
// signer-material forms Request.NativeSignerMaterial() accepts for signing --
// crucially the VALUE form a custom SignerMaterialResolver may return -- so a
// wallet with valid native material is not silently dropped to legacy for ROAST.
func TestRegistrationKeyGroupIDForSigner_NormalizesMaterialForms(t *testing.T) {
	payload, err := json.Marshal(&signing.NativeTBTCSignerMaterialPayload{
		KeyGroup: "kg-normalize",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	valueMaterial := signing.NativeSignerMaterial{
		Format:  signing.NativeSignerMaterialFormatFrostTBTCSignerV1,
		Payload: payload,
	}

	// VALUE form -- the regression: previously rejected by the pointer-only assertion.
	kg, err := registrationKeyGroupIDForSigner(&signer{signerMaterial: valueMaterial})
	if err != nil {
		t.Fatalf("value-form native material must be accepted: %v", err)
	}
	if kg != "kg-normalize" {
		t.Fatalf("value-form key group = %q, want kg-normalize", kg)
	}

	// POINTER form still works.
	kg, err = registrationKeyGroupIDForSigner(&signer{signerMaterial: &valueMaterial})
	if err != nil {
		t.Fatalf("pointer-form native material must be accepted: %v", err)
	}
	if kg != "kg-normalize" {
		t.Fatalf("pointer-form key group = %q, want kg-normalize", kg)
	}

	// Non-native material (e.g. legacy []byte / UniFFIv1) has no FROST key-group
	// handle and is correctly rejected -> the wallet stays on the legacy path.
	if _, err := registrationKeyGroupIDForSigner(&signer{signerMaterial: []byte{0x01, 0x02}}); err == nil {
		t.Fatal("non-native signer material must be rejected")
	}
}
