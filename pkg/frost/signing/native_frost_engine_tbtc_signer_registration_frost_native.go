//go:build frost_native && frost_tbtc_signer && cgo && !frost_uniffi_sdk

package signing

import "fmt"

type buildTaggedTBTCSignerEngine struct{}

func registerBuildTaggedNativeFROSTSigningEngine() error {
	return RegisterNativeTBTCSignerEngine(&buildTaggedTBTCSignerEngine{})
}

func (bttse *buildTaggedTBTCSignerEngine) StartSignRound(
	sessionID string,
	message []byte,
	keyGroup string,
) (*NativeTBTCSignerRoundState, error) {
	return nil, buildTaggedTBTCSignerBridgeNotImplementedError("StartSignRound")
}

func (bttse *buildTaggedTBTCSignerEngine) FinalizeSignRound(
	sessionID string,
	roundContributions []NativeTBTCSignerRoundContribution,
) ([]byte, error) {
	return nil, buildTaggedTBTCSignerBridgeNotImplementedError("FinalizeSignRound")
}

func buildTaggedTBTCSignerBridgeNotImplementedError(operation string) error {
	return fmt.Errorf(
		"tbtc-signer bridge operation [%v] is not implemented",
		operation,
	)
}
