//go:build !(frost_native && frost_tbtc_signer && cgo)

package signing

import "fmt"

// ReadNativeTBTCSignerDurableStoreIdentity is unavailable without the native
// tbtc-signer bridge. Production FROST activation treats this as fatal; it
// must never fall back to a configured path or config fingerprint.
func ReadNativeTBTCSignerDurableStoreIdentity() (
	*NativeTBTCSignerDurableStoreIdentity,
	error,
) {
	return nil, fmt.Errorf(
		"%w: tbtc-signer bridge operation [DurableStoreIdentity] is unavailable in this build",
		ErrNativeCryptographyUnavailable,
	)
}
