//go:build frost_native

package signing

import "fmt"

// BuildNativeTBTCSignerTaprootTx routes a BuildTaprootTx request through the
// currently-registered coarse tbtc-signer engine.
func BuildNativeTBTCSignerTaprootTx(
	sessionID string,
	inputs []NativeTBTCSignerTxInput,
	outputs []NativeTBTCSignerTxOutput,
	scriptTreeHex *string,
) (*NativeTBTCSignerTxResult, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is empty")
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("inputs are empty")
	}

	if len(outputs) == 0 {
		return nil, fmt.Errorf("outputs are empty")
	}

	nativeEngine := currentNativeTBTCSignerEngine()
	if nativeEngine == nil {
		return nil, fmt.Errorf(
			"%w: native tbtc-signer engine is unavailable",
			ErrNativeCryptographyUnavailable,
		)
	}

	return nativeEngine.BuildTaprootTx(sessionID, inputs, outputs, scriptTreeHex)
}
