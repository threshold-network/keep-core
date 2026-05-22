package signing

import (
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
	KeyGroup                 string `json:"keyGroup"`
	KeyGroupSource           string `json:"keyGroupSource,omitempty"`
	LegacyPrivateKeyShareHex string `json:"legacyPrivateKeyShareHex,omitempty"`
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
