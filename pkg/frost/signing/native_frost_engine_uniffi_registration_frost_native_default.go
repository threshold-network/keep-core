//go:build frost_native && !(frost_uniffi_sdk && cgo)

package signing

func registerBuildTaggedNativeFROSTSigningEngine() error {
	return nil
}
