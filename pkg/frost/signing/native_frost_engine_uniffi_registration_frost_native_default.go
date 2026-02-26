//go:build frost_native && !(frost_tbtc_signer && cgo) && !(frost_uniffi_sdk && cgo && frost_uniffi_legacy)

package signing

func registerBuildTaggedNativeFROSTSigningEngine() error {
	return nil
}
