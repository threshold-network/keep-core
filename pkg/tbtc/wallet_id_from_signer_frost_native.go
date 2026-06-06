//go:build frost_native

package tbtc

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func calculateWalletIDForSigner(
	signer *signer,
	calculateLegacyWalletID func(*ecdsa.PublicKey) ([32]byte, error),
) ([32]byte, error) {
	if signer == nil {
		return [32]byte{}, fmt.Errorf("signer is nil")
	}

	walletID, isFrostWallet, err := frostWalletIDFromSigner(signer)
	if err != nil {
		return [32]byte{}, err
	}
	if isFrostWallet {
		return walletID, nil
	}

	return calculateLegacyWalletID(signer.wallet.publicKey)
}

func frostWalletIDFromSigner(signer *signer) ([32]byte, bool, error) {
	material, ok := nativeSignerMaterialFromSigner(signer)
	if !ok {
		return [32]byte{}, false, nil
	}

	if material.Format != frostsigning.NativeSignerMaterialFormatFrostUniFFIV2 &&
		material.Format != frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1 {
		return [32]byte{}, false, nil
	}

	if material.Format == frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1 {
		var payload frostsigning.NativeTBTCSignerMaterialPayload
		if err := json.Unmarshal(material.Payload, &payload); err != nil {
			return [32]byte{}, false, fmt.Errorf(
				"cannot decode FrostTBTCSignerV1 signer material: [%w]",
				err,
			)
		}

		if payload.KeyGroupSource ==
			frostsigning.NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey {
			return [32]byte{}, false, nil
		}
	}

	xOnlyOutputKey, err := frostsigning.ExtractTaprootOutputKeyFromMaterial(
		material,
	)
	if err != nil {
		return [32]byte{}, true, err
	}
	if len(xOnlyOutputKey) != 32 {
		return [32]byte{}, true, fmt.Errorf(
			"FROST DKG output key length [%d] is not 32",
			len(xOnlyOutputKey),
		)
	}

	var walletID [32]byte
	copy(walletID[:], xOnlyOutputKey)

	return walletID, true, nil
}

func nativeSignerMaterialFromSigner(
	signer *signer,
) (*frostsigning.NativeSignerMaterial, bool) {
	if signer == nil {
		return nil, false
	}

	switch material := signer.signingMaterial().(type) {
	case *frostsigning.NativeSignerMaterial:
		if material == nil {
			return nil, false
		}

		return material, true
	case frostsigning.NativeSignerMaterial:
		return &material, true
	default:
		return nil, false
	}
}
