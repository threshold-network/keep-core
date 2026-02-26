//go:build frost_native && frost_tbtc_signer && cgo

package tbtc

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func buildTaprootTxViaNativeSigner(
	unsignedTx *bitcoin.TransactionBuilder,
) (string, error) {
	if unsignedTx == nil {
		return "", fmt.Errorf("unsigned transaction builder is nil")
	}

	inputs, outputs, err := unsignedTx.UnsignedTransactionIO()
	if err != nil {
		return "", fmt.Errorf("cannot extract unsigned transaction I/O: [%w]", err)
	}

	nativeInputs := make([]frostsigning.NativeTBTCSignerTxInput, 0, len(inputs))
	for _, input := range inputs {
		nativeInputs = append(
			nativeInputs,
			frostsigning.NativeTBTCSignerTxInput{
				TxIDHex:   input.TxIDHex,
				Vout:      input.Vout,
				ValueSats: input.ValueSats,
			},
		)
	}

	nativeOutputs := make([]frostsigning.NativeTBTCSignerTxOutput, 0, len(outputs))
	for _, output := range outputs {
		nativeOutputs = append(
			nativeOutputs,
			frostsigning.NativeTBTCSignerTxOutput{
				ScriptPubKeyHex: output.ScriptPubKeyHex,
				ValueSats:       output.ValueSats,
			},
		)
	}

	sessionID := buildTaprootTxSessionID(inputs, outputs)

	result, err := frostsigning.BuildNativeTBTCSignerTaprootTx(
		sessionID,
		nativeInputs,
		nativeOutputs,
		nil,
	)
	if err != nil {
		// Keep legacy fallback behavior for the observational BuildTaprootTx
		// phase when native bridge support is unavailable.
		if errors.Is(err, frostsigning.ErrNativeCryptographyUnavailable) {
			return "", nil
		}

		return "", err
	}

	if result == nil {
		return "", fmt.Errorf("native tbtc-signer returned nil BuildTaprootTx result")
	}

	if result.SessionID != sessionID {
		return "", fmt.Errorf(
			"native tbtc-signer BuildTaprootTx returned unexpected session ID: [%v] != [%v]",
			result.SessionID,
			sessionID,
		)
	}

	if result.TxHex == "" {
		return "", fmt.Errorf("native tbtc-signer BuildTaprootTx returned empty tx hex")
	}

	return result.TxHex, nil
}

func buildTaprootTxSessionID(
	inputs []bitcoin.UnsignedTransactionInput,
	outputs []bitcoin.UnsignedTransactionOutput,
) string {
	// Session ID is deterministically derived from Go-side transaction I/O using
	// encoding/json. Rust currently treats this session_id as opaque.
	// If input/output schema changes in a future migration phase, update this
	// derivation intentionally to avoid silent cross-version session ID drift.
	sessionPayload, err := json.Marshal(struct {
		Inputs  []bitcoin.UnsignedTransactionInput  `json:"inputs"`
		Outputs []bitcoin.UnsignedTransactionOutput `json:"outputs"`
	}{
		Inputs:  inputs,
		Outputs: outputs,
	})
	if err != nil {
		return fmt.Sprintf("buildtx-fallback-%d-%d", len(inputs), len(outputs))
	}

	digest := sha256.Sum256(sessionPayload)
	return fmt.Sprintf("buildtx-%x", digest[:])
}
