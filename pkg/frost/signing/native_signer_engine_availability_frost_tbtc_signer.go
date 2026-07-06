//go:build frost_native && frost_tbtc_signer && cgo

package signing

// nativeSignerEngineAvailable reports whether real native FROST signing is
// actually usable in this build/runtime: the build-tagged signer engine is
// registered AND the linked libfrost_tbtc responds to the FFI ABI probe.
//
// The Go wrapper (buildTaggedTBTCSignerEngine) is registered by this build even
// when libfrost_tbtc is absent or lacks the required ABI symbol - it only
// discovers that later, via dlsym/ABI preflight, when an operation runs. So the
// wrapper pointer alone (currentNativeTBTCSignerEngine != nil) is not a reliable
// availability signal. ensureTBTCSignerABICompatible runs the same preflight
// probe the wrapper uses at operation time (dlsym of frost_tbtc_abi_version plus
// a version-compatibility check, cached via sync.Once so it is cheap to call);
// it returns nil only for a present, ABI-compatible library. Requiring both
// makes the FROST startup guard fail fast when the signer library is not truly
// loadable, instead of the node starting and failing DKG/signing at runtime.
func nativeSignerEngineAvailable() bool {
	return currentNativeTBTCSignerEngine() != nil &&
		ensureTBTCSignerABICompatible() == nil
}
