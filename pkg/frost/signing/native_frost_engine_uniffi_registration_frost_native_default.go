//go:build frost_native && !(frost_tbtc_signer && cgo)

package signing

func registerBuildTaggedNativeFROSTSigningEngine() error {
	return nil
}
