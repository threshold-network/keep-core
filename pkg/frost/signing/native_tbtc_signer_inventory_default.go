//go:build !(frost_native && frost_tbtc_signer && cgo)

package signing

import "fmt"

func ReadNativeTBTCSignerRetainedKeyPackageInventory() (
	*NativeTBTCSignerRetainedKeyPackageInventory,
	error,
) {
	return nil, fmt.Errorf(
		"%w: tbtc-signer bridge operation [RetainedKeyPackageInventory] is unavailable in this build",
		ErrNativeCryptographyUnavailable,
	)
}

func ReadNativeTBTCSignerStateWitnessProof(
	request *NativeTBTCSignerStateWitnessProofRequest,
) (*NativeTBTCSignerStateWitnessProof, error) {
	return nil, fmt.Errorf(
		"%w: tbtc-signer bridge operation [StateWitnessProof] is unavailable in this build",
		ErrNativeCryptographyUnavailable,
	)
}
