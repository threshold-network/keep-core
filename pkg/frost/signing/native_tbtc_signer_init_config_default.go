//go:build !(frost_native && frost_tbtc_signer && cgo)

package signing

import "fmt"

// InstallNativeTBTCSignerConfig is unavailable in builds without the
// tbtc-signer cgo bridge; see the frost_native && frost_tbtc_signer && cgo
// variant for the real implementation and contract.
func InstallNativeTBTCSignerConfig(
	_ []byte,
) (*NativeTBTCSignerInitConfigResult, error) {
	return nil, fmt.Errorf(
		"%w: tbtc-signer bridge operation [InitSignerConfig] is unavailable in this build",
		ErrNativeCryptographyUnavailable,
	)
}
