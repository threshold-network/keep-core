//go:build frost_native && !(frost_uniffi_sdk && cgo) && !(frost_tbtc_signer && cgo)

package signing

func registerBuildTaggedNativeFROSTSigningEngine() error {
	return nil
}
