//go:build !frost_native

package tbtc

import (
	"encoding/json"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

func signingMaterialUsesSchnorrSignatures(signingMaterial any) bool {
	switch material := signingMaterial.(type) {
	case *tecdsa.PrivateKeyShare:
		return false
	case *frostsigning.NativeSignerMaterial:
		return nativeSignerMaterialUsesSchnorrSignaturesDefault(material)
	case frostsigning.NativeSignerMaterial:
		return nativeSignerMaterialUsesSchnorrSignaturesDefault(&material)
	default:
		return true
	}
}

func nativeSignerMaterialUsesSchnorrSignaturesDefault(
	material *frostsigning.NativeSignerMaterial,
) bool {
	if material == nil {
		return true
	}

	switch material.Format {
	case frostsigning.NativeSignerMaterialFormatFrostUniFFIV1:
		return false
	case frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1:
		var payload frostsigning.NativeTBTCSignerMaterialPayload
		if err := json.Unmarshal(material.Payload, &payload); err != nil {
			return true
		}

		return payload.KeyGroupSource !=
			frostsigning.NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey
	default:
		return true
	}
}
