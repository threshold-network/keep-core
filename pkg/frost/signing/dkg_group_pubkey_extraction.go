//go:build frost_native

package signing

import (
	"errors"
	"fmt"
)

// ErrUnsupportedSignerMaterialFormat is returned by
// ExtractDkgGroupPublicKeyFromMaterial when the material's Format
// field names a signer-material variant the helper cannot extract a DKG group
// public key from. The current implementation accepts FrostTBTCSignerV1;
// FrostUniFFIV1 is rejected because the legacy bridge format does not expose
// the group key, and unsupported FrostUniFFIV2 material is rejected because it
// cannot support Taproot-tweaked deposit sweep signatures.
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
//   - FrostTBTCSignerV1: decode payload as NativeTBTCSignerMaterialPayload;
//     return the raw bytes of the KeyGroup identifier. The tbtc-signer
//     engine treats KeyGroup as the canonical handle for the FROST
//     key group; every honest signer running the same tbtc-signer
//     build agrees on its bytes.
//
//   - FrostUniFFIV1 and FrostUniFFIV2: return
//     ErrUnsupportedSignerMaterialFormat. V1 material is the legacy bridge
//     format that does not carry the group key in a form Phase 6 can extract.
//     V2 material is unsupported in favor of FrostTBTCSignerV1.
//
// Callers MUST use the returned bytes only as the
// DkgGroupPublicKey input to attempt.DeriveAttemptSeed; the bytes
// are not interchangeable across format boundaries. Production signing groups
// must use FrostTBTCSignerV1 material.
func ExtractDkgGroupPublicKeyFromMaterial(
	signerMaterial *NativeSignerMaterial,
) ([]byte, error) {
	if signerMaterial == nil {
		return nil, fmt.Errorf(
			"dkg group public key: signer material is nil",
		)
	}
	switch signerMaterial.Format {
	case NativeSignerMaterialFormatFrostTBTCSignerV1:
		return extractDkgGroupPublicKeyFromTBTCSignerV1(signerMaterial)
	case NativeSignerMaterialFormatFrostUniFFIV1:
		return nil, fmt.Errorf(
			"%w: %s (migrate to %s before enabling ROAST retry)",
			ErrUnsupportedSignerMaterialFormat,
			signerMaterial.Format,
			NativeSignerMaterialFormatFrostTBTCSignerV1,
		)
	case NativeSignerMaterialFormatFrostUniFFIV2:
		return nil, fmt.Errorf(
			"%w: %s is unsupported; use %s",
			ErrUnsupportedSignerMaterialFormat,
			signerMaterial.Format,
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
