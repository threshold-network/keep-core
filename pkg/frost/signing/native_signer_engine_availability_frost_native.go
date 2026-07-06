//go:build frost_native

package signing

// nativeSignerEngineAvailable reports whether the real native tbtc-signer FROST
// engine is registered. The engine is registered only by
// frost_native && frost_tbtc_signer && cgo builds (see
// native_frost_engine_tbtc_signer_registration_frost_native.go), so on a
// frost_native build that is not linked with the native signer this is nil even
// though the transitional FFI executor/legacy-delegate wrappers are registered.
// This is the same engine the transitional signing primitive requires before it
// will execute natively rather than fall back to the legacy bridge.
func nativeSignerEngineAvailable() bool {
	return currentNativeTBTCSignerEngine() != nil
}
