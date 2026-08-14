package signing

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	// NativeSignerMaterialFormatFrostTBTCSignerV1 carries signer material for
	// tbtc-signer coarse session APIs.
	NativeSignerMaterialFormatFrostTBTCSignerV1 = "frost-tbtc-signer-v1"
	// NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey marks scaffold-era
	// key-group derivation from the legacy wallet public key. Material built
	// with this source is placeholder data, not the output of a real FROST DKG
	// run, and is refused by default at signing time. See
	// `AcceptScaffoldKeyGroupEnvVar` for the opt-in escape hatch.
	NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey = "legacy-wallet-pubkey"
	// NativeTBTCSignerKeyGroupSourceDKGPersisted marks key-group material
	// produced by a FROST wallet DKG and persisted for later signing.
	NativeTBTCSignerKeyGroupSourceDKGPersisted = "dkg-persisted"

	// AcceptScaffoldKeyGroupEnvVar is the operator-facing opt-in that allows
	// the FROST tbtc-signer FFI path to accept signer material whose
	// `KeyGroupSource` is `legacy-wallet-pubkey`. Production deployments must
	// not set this; it exists for local dev, CI, and integration rehearsals
	// where a real DKG hand-off is not yet wired.
	AcceptScaffoldKeyGroupEnvVar = "KEEP_CORE_FROST_TBTC_SIGNER_ACCEPT_SCAFFOLD_KEY_GROUP"
)

// NativeTBTCSignerMaterialPayload is the signer-material payload schema for
// `frost-tbtc-signer-v1`.
type NativeTBTCSignerMaterialPayload struct {
	KeyGroup                 string                           `json:"keyGroup"`
	TaprootOutputKey         string                           `json:"taprootOutputKey,omitempty"`
	KeyGroupSource           string                           `json:"keyGroupSource,omitempty"`
	DKGSeedHex               string                           `json:"dkgSeedHex,omitempty"`
	DKGParticipants          []NativeTBTCSignerDKGParticipant `json:"dkgParticipants,omitempty"`
	DKGThreshold             uint16                           `json:"dkgThreshold,omitempty"`
	LegacyPrivateKeyShareHex string                           `json:"legacyPrivateKeyShareHex,omitempty"`
}

// NativeTBTCSignerDKGParticipant identifies a DKG participant for coarse
// tbtc-signer RunDKG operation.
type NativeTBTCSignerDKGParticipant struct {
	Identifier   uint16 `json:"identifier"`
	PublicKeyHex string `json:"publicKeyHex"`
}

func decodeBuildTaggedTBTCSignerMaterialPayload(
	signerMaterial *NativeSignerMaterial,
) (*NativeTBTCSignerMaterialPayload, error) {
	if signerMaterial == nil {
		return nil, fmt.Errorf(
			"%w: signer material is nil",
			ErrNativeCryptographyUnavailable,
		)
	}

	if signerMaterial.Format != NativeSignerMaterialFormatFrostTBTCSignerV1 {
		return nil, fmt.Errorf(
			"%w: unsupported signer material format: [%s]",
			ErrNativeCryptographyUnavailable,
			signerMaterial.Format,
		)
	}

	if len(signerMaterial.Payload) == 0 {
		return nil, fmt.Errorf(
			"%w: signer material payload is empty",
			ErrNativeCryptographyUnavailable,
		)
	}

	var payload NativeTBTCSignerMaterialPayload
	if err := json.Unmarshal(signerMaterial.Payload, &payload); err != nil {
		return nil, fmt.Errorf(
			"%w: cannot unmarshal tbtc-signer payload: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if payload.KeyGroup == "" {
		return nil, fmt.Errorf(
			"%w: tbtc-signer key group is empty",
			ErrNativeCryptographyUnavailable,
		)
	}

	return &payload, nil
}

// AcceptScaffoldKeyGroupEnabled reports whether the operator has opted into
// accepting scaffold-era (legacy-wallet-pubkey) key-group material. Without
// this, the signer material resolver and the FFI signing primitive both
// refuse legacy material rather than silently signing with placeholder
// cryptographic context.
//
// The env var is parsed identically to the bootstrap-mode flag in
// `pkg/frost/signing/backend.go`: case-insensitive `1`, `true`, `yes`, or
// `on`. Anything else (including missing/empty) is treated as disabled, so
// the safe-by-default behavior is to refuse.
func AcceptScaffoldKeyGroupEnabled() bool {
	raw, ok := os.LookupEnv(AcceptScaffoldKeyGroupEnvVar)
	if !ok {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
