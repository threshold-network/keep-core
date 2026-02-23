package tbtc

// tbtcSignerMaterialPayload is the persisted signer-material payload for
// `frost-tbtc-signer-v1`.
type tbtcSignerMaterialPayload struct {
	KeyGroup                 string `json:"keyGroup"`
	LegacyPrivateKeyShareHex string `json:"legacyPrivateKeyShareHex,omitempty"`
}
