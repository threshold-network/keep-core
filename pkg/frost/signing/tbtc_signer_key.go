package signing

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/keep-network/keep-core/pkg/frost"
)

// KeyGroupIDFromSignerMaterial returns the exact FROST key-group handle stored
// in native signer material.
func KeyGroupIDFromSignerMaterial(
	signerMaterial *NativeSignerMaterial,
) (string, error) {
	if signerMaterial == nil {
		return "", fmt.Errorf("key group id: signer material is nil")
	}
	payload, err := decodeBuildTaggedTBTCSignerMaterialPayload(signerMaterial)
	if err != nil {
		return "", fmt.Errorf("key group id: %w", err)
	}
	return payload.KeyGroup, nil
}

// ExtractTaprootOutputKeyFromMaterial returns the 32-byte x-only Taproot
// output key committed to by native FROST signer material.
func ExtractTaprootOutputKeyFromMaterial(
	signerMaterial *NativeSignerMaterial,
) ([]byte, error) {
	if signerMaterial == nil {
		return nil, fmt.Errorf("taproot output key: signer material is nil")
	}

	switch signerMaterial.Format {
	case NativeSignerMaterialFormatFrostTBTCSignerV1:
		return extractTaprootOutputKeyFromTBTCSignerV1(signerMaterial)
	default:
		return nil, fmt.Errorf(
			"taproot output key: unsupported signer-material format [%s]",
			signerMaterial.Format,
		)
	}
}

func extractTaprootOutputKeyFromTBTCSignerV1(
	signerMaterial *NativeSignerMaterial,
) ([]byte, error) {
	payload, err := decodeBuildTaggedTBTCSignerMaterialPayload(signerMaterial)
	if err != nil {
		return nil, fmt.Errorf(
			"taproot output key: decode FrostTBTCSignerV1: %w",
			err,
		)
	}
	if payload.KeyGroupSource != NativeTBTCSignerKeyGroupSourceDKGPersisted {
		return nil, fmt.Errorf(
			"taproot output key: FrostTBTCSignerV1 key group source [%s] is not [%s]",
			payload.KeyGroupSource,
			NativeTBTCSignerKeyGroupSourceDKGPersisted,
		)
	}

	outputKeyHex := payload.TaprootOutputKey
	if outputKeyHex == "" {
		outputKeyHex = payload.KeyGroup
	}

	outputKey, err := TaprootOutputKeyFromTBTCSignerKey(outputKeyHex)
	if err != nil {
		return nil, fmt.Errorf(
			"taproot output key: FrostTBTCSignerV1 key material is invalid: %w",
			err,
		)
	}

	return outputKey, nil
}

// TaprootOutputKeyFromTBTCSignerKey converts tbtc-signer key material to the
// x-only BIP-340 output key committed to by P2TR wallet scripts. Current
// tbtc-signer DKG results expose the group verifying key as a compressed
// secp256k1 key-group handle, while older persisted material may already carry
// the x-only key.
func TaprootOutputKeyFromTBTCSignerKey(keyHex string) ([]byte, error) {
	raw, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}

	switch len(raw) {
	case frost.OutputKeySize:
		return raw, nil
	case 1 + frost.OutputKeySize:
		publicKey, err := btcec.ParsePubKey(raw)
		if err != nil {
			return nil, err
		}
		return publicKey.X().FillBytes(make([]byte, frost.OutputKeySize)), nil
	default:
		return nil, fmt.Errorf(
			"must be %d-byte x-only or %d-byte compressed key, got %d bytes",
			frost.OutputKeySize,
			1+frost.OutputKeySize,
			len(raw),
		)
	}
}
