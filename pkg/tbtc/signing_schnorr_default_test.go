//go:build !frost_native

package tbtc

import (
	"encoding/json"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func TestSigningMaterialUsesSchnorrSignatures_Default(t *testing.T) {
	tbtcSignerPayload := func(t *testing.T, keyGroupSource string) []byte {
		t.Helper()

		payload, err := json.Marshal(frostsigning.NativeTBTCSignerMaterialPayload{
			KeyGroup:       "key-group",
			KeyGroupSource: keyGroupSource,
		})
		if err != nil {
			t.Fatalf("cannot marshal tbtc-signer payload: [%v]", err)
		}

		return payload
	}

	tests := map[string]struct {
		material      any
		expectSchnorr bool
	}{
		"legacy tecdsa private key share": {
			material:      &tecdsa.PrivateKeyShare{},
			expectSchnorr: false,
		},
		"legacy frost uniffi v1 material": {
			material: &frostsigning.NativeSignerMaterial{
				Format:  frostsigning.NativeSignerMaterialFormatFrostUniFFIV1,
				Payload: []byte{0x01},
			},
			expectSchnorr: false,
		},
		"legacy tbtc-signer scaffold material": {
			material: &frostsigning.NativeSignerMaterial{
				Format: frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
				Payload: tbtcSignerPayload(
					t,
					frostsigning.NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
				),
			},
			expectSchnorr: false,
		},
		"native tbtc-signer material": {
			material: &frostsigning.NativeSignerMaterial{
				Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
				Payload: tbtcSignerPayload(t, "dkg-persisted"),
			},
			expectSchnorr: true,
		},
		"malformed tbtc-signer material": {
			material: &frostsigning.NativeSignerMaterial{
				Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
				Payload: []byte("not-json"),
			},
			expectSchnorr: true,
		},
		"unknown native material": {
			material: &frostsigning.NativeSignerMaterial{
				Format:  "unknown",
				Payload: []byte{0x01},
			},
			expectSchnorr: true,
		},
		"unknown material": {
			material:      struct{}{},
			expectSchnorr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			actual := signingMaterialUsesSchnorrSignatures(test.material)
			if actual != test.expectSchnorr {
				t.Fatalf(
					"unexpected Schnorr classification\nexpected: [%v]\nactual:   [%v]",
					test.expectSchnorr,
					actual,
				)
			}
		})
	}
}
