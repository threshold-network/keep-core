//go:build frost_native

package signing

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/keep-network/keep-core/pkg/frost"
)

// ErrUnsupportedSignerMaterialFormat is returned by
// ExtractDkgGroupPublicKeyFromMaterial when the material's Format
// field names a signer-material variant the helper cannot extract
// a DKG group public key from. The current implementation accepts
// FrostUniFFIV2 and FrostTBTCSignerV1; FrostUniFFIV1 is rejected
// because the legacy bridge format does not expose the group key.
//
// Per RFC-21 Phase-6 Resolved Decision: the Phase 7 manifest flip
// is gated on verified migration off V1 across production signers,
// so this error class is expected to disappear by the time ROAST
// retry ships unconditionally.
var ErrUnsupportedSignerMaterialFormat = errors.New(
	"dkg group public key: unsupported signer-material format for extraction",
)

// ExtractDkgGroupPublicKeyFromMaterial returns the DKG-validated
// group public key from the supplied NativeSignerMaterial in the
// canonical byte representation that attempt.DeriveAttemptSeed
// consumes. Two honest signers feeding the same material into this
// helper produce byte-identical outputs.
//
// Format handling:
//
//   - FrostUniFFIV2: decode payload as nativeFROSTUniFFIV2SignerMaterial;
//     hex-decode PublicKeyPackage.VerifyingKey. This is the x-only
//     output key produced by the native FROST DKG.
//
//   - FrostTBTCSignerV1: decode payload as NativeTBTCSignerMaterialPayload;
//     return the raw bytes of the KeyGroup identifier. The tbtc-signer
//     engine treats KeyGroup as the canonical handle for the FROST
//     key group; every honest signer running the same tbtc-signer
//     build agrees on its bytes.
//
//   - FrostUniFFIV1: returns ErrUnsupportedSignerMaterialFormat.
//     V1 material is the legacy bridge format that does not carry
//     the group public key in a form Phase 6 can extract.
//
// Callers MUST use the returned bytes only as the
// DkgGroupPublicKey input to attempt.DeriveAttemptSeed; the bytes
// are not interchangeable across format boundaries (a UniFFIV2 key
// and a TBTCSignerV1 key for the "same" logical group produce
// different bytes -- they are different formats). Production
// signing groups must run on a single uniform format.
func ExtractDkgGroupPublicKeyFromMaterial(
	signerMaterial *NativeSignerMaterial,
) ([]byte, error) {
	if signerMaterial == nil {
		return nil, fmt.Errorf(
			"dkg group public key: signer material is nil",
		)
	}
	switch signerMaterial.Format {
	case NativeSignerMaterialFormatFrostUniFFIV2:
		return extractDkgGroupPublicKeyFromUniFFIV2(signerMaterial)
	case NativeSignerMaterialFormatFrostTBTCSignerV1:
		return extractDkgGroupPublicKeyFromTBTCSignerV1(signerMaterial)
	case NativeSignerMaterialFormatFrostUniFFIV1:
		return nil, fmt.Errorf(
			"%w: %s (migrate to %s or %s before enabling ROAST retry)",
			ErrUnsupportedSignerMaterialFormat,
			signerMaterial.Format,
			NativeSignerMaterialFormatFrostUniFFIV2,
			NativeSignerMaterialFormatFrostTBTCSignerV1,
		)
	default:
		return nil, fmt.Errorf(
			"%w: unknown format %q",
			ErrUnsupportedSignerMaterialFormat,
			signerMaterial.Format,
		)
	}
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
	case NativeSignerMaterialFormatFrostUniFFIV2:
		return extractDkgGroupPublicKeyFromUniFFIV2(signerMaterial)
	case NativeSignerMaterialFormatFrostTBTCSignerV1:
		return extractTaprootOutputKeyFromTBTCSignerV1(signerMaterial)
	default:
		return nil, fmt.Errorf(
			"taproot output key: unsupported signer-material format [%s]",
			signerMaterial.Format,
		)
	}
}

func extractDkgGroupPublicKeyFromUniFFIV2(
	signerMaterial *NativeSignerMaterial,
) ([]byte, error) {
	decoded, err := decodeNativeFROSTUniFFIV2SignerMaterial(signerMaterial)
	if err != nil {
		return nil, fmt.Errorf(
			"dkg group public key: decode FrostUniFFIV2: %w",
			err,
		)
	}
	if decoded.PublicKeyPackage == nil {
		return nil, fmt.Errorf(
			"dkg group public key: FrostUniFFIV2 public key package is nil",
		)
	}
	verifyingKey := decoded.PublicKeyPackage.VerifyingKey
	if verifyingKey == "" {
		return nil, fmt.Errorf(
			"dkg group public key: FrostUniFFIV2 verifying key is empty",
		)
	}
	raw, err := hex.DecodeString(verifyingKey)
	if err != nil {
		return nil, fmt.Errorf(
			"dkg group public key: FrostUniFFIV2 verifying key is not hex: %w",
			err,
		)
	}
	if len(raw) != frost.OutputKeySize {
		return nil, fmt.Errorf(
			"dkg group public key: FrostUniFFIV2 verifying key must be %d bytes, got %d",
			frost.OutputKeySize,
			len(raw),
		)
	}
	return raw, nil
}

func extractDkgGroupPublicKeyFromTBTCSignerV1(
	signerMaterial *NativeSignerMaterial,
) ([]byte, error) {
	payload, err := decodeBuildTaggedTBTCSignerMaterialPayload(signerMaterial)
	if err != nil {
		return nil, fmt.Errorf(
			"dkg group public key: decode FrostTBTCSignerV1: %w",
			err,
		)
	}
	if payload.KeyGroup == "" {
		return nil, fmt.Errorf(
			"dkg group public key: FrostTBTCSignerV1 key group is empty",
		)
	}
	return []byte(payload.KeyGroup), nil
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
// secp256k1 key-group handle, while older test material may already carry the
// x-only key.
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
