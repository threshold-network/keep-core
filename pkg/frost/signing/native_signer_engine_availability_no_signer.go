//go:build frost_native && (!frost_tbtc_signer || !cgo)

package signing

// nativeSignerEngineAvailable is false on frost_native builds that are not linked
// with the native tbtc-signer (frost_tbtc_signer && cgo): the signer engine is
// never registered and there is no libfrost_tbtc to probe, so native FROST
// signing is unavailable.
func nativeSignerEngineAvailable() bool {
	return false
}
