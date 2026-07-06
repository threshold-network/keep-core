//go:build !frost_native

package signing

// nativeSignerEngineAvailable is always false on non-frost_native builds: the
// native tbtc-signer FROST engine is not compiled in, so native FROST signing
// is unavailable.
func nativeSignerEngineAvailable() bool {
	return false
}
