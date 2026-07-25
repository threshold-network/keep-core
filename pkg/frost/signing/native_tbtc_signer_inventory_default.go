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

func ReadNativeTBTCSignerStateWitnessTip() (
	*NativeTBTCSignerStateWitnessTip,
	error,
) {
	return nil, fmt.Errorf(
		"%w: tbtc-signer bridge operation [StateWitnessTip] is unavailable in this build",
		ErrNativeCryptographyUnavailable,
	)
}

func AcknowledgeNativeTBTCSignerStateWitnessCheckpoint(
	signedAcknowledgementJSON []byte,
) (*NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult, error) {
	return nil, fmt.Errorf(
		"%w: tbtc-signer bridge operation [AcknowledgeStateWitnessCheckpoint] is unavailable in this build",
		ErrNativeCryptographyUnavailable,
	)
}

func RecoverNativeTBTCSignerStateWitnessCheckpoint(
	exactReadResponseJSON []byte,
) (*NativeTBTCSignerStateWitnessCheckpointRecoveryResult, error) {
	return nil, fmt.Errorf(
		"%w: tbtc-signer bridge operation [RecoverStateWitnessCheckpoint] is unavailable in this build",
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
