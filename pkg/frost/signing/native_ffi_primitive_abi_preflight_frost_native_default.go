//go:build frost_native && (!frost_tbtc_signer || !cgo)

package signing

// tbtcSignerABIIncompatibility is a no-op in builds without the linked cgo engine: there
// is no real libfrost_tbtc to be incompatible with. The cgo-tagged variant
// (native_ffi_primitive_abi_preflight_frost_native_tbtc_signer.go) does the real check.
func tbtcSignerABIIncompatibility() error { return nil }
