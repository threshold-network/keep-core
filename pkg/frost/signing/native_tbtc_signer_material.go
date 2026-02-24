package signing

const (
	// NativeSignerMaterialFormatFrostTBTCSignerV1 carries signer material for
	// tbtc-signer coarse session APIs.
	NativeSignerMaterialFormatFrostTBTCSignerV1 = "frost-tbtc-signer-v1"
	// NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey marks scaffold-era
	// key-group derivation from the legacy wallet public key.
	NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey = "legacy-wallet-pubkey"
)

// NativeTBTCSignerMaterialPayload is the signer-material payload schema for
// `frost-tbtc-signer-v1`.
type NativeTBTCSignerMaterialPayload struct {
	KeyGroup                 string `json:"keyGroup"`
	KeyGroupSource           string `json:"keyGroupSource,omitempty"`
	LegacyPrivateKeyShareHex string `json:"legacyPrivateKeyShareHex,omitempty"`
}
