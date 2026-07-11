//go:build frost_native && frost_tbtc_signer && cgo

package tbtc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

var buildTaprootTxForSessionFn = buildTaprootTxForSession

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

	result, err := buildTaprootTxForSessionFn(
		buildTaprootTxSessionID(inputs, outputs),
		unsignedTx,
	)
	if errors.Is(err, frostsigning.ErrNativeCryptographyUnavailable) {
		// This legacy parity call is observational unless substitution is
		// explicitly enabled, so preserve its no-native fallback. The mandatory
		// per-signing policy binding below deliberately does not suppress this
		// error.
		return "", nil
	}
	if err != nil || result == nil {
		return "", err
	}

	return result.TxHex, nil
}

func buildTaprootTxForSession(
	sessionID string,
	unsignedTx *bitcoin.TransactionBuilder,
) (*frostsigning.NativeTBTCSignerTxResult, error) {
	if unsignedTx == nil {
		return nil, fmt.Errorf("unsigned transaction builder is nil")
	}

	inputs, outputs, err := unsignedTx.UnsignedTransactionIO()
	if err != nil {
		return nil, fmt.Errorf("cannot extract unsigned transaction I/O: [%w]", err)
	}

	nativeInputs := make([]frostsigning.NativeTBTCSignerTxInput, 0, len(inputs))
	for _, input := range inputs {
		nativeInputs = append(
			nativeInputs,
			frostsigning.NativeTBTCSignerTxInput{
				TxIDHex:         input.TxIDHex,
				Vout:            input.Vout,
				ValueSats:       input.ValueSats,
				ScriptPubKeyHex: input.ScriptPubKeyHex,
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

	result, err := frostsigning.BuildNativeTBTCSignerTaprootTx(
		sessionID,
		nativeInputs,
		nativeOutputs,
		nil,
	)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("native tbtc-signer returned nil BuildTaprootTx result")
	}

	if result.SessionID != sessionID {
		return nil, fmt.Errorf(
			"native tbtc-signer BuildTaprootTx returned unexpected session ID: [%v] != [%v]",
			result.SessionID,
			sessionID,
		)
	}

	if result.TxHex == "" {
		return nil, fmt.Errorf("native tbtc-signer BuildTaprootTx returned empty tx hex")
	}

	return result, nil
}

// bindTaprootTxViaNativeSigner creates the policy artifact in the SAME stable
// ROAST session InteractiveSessionOpen will use, then proves the Rust builder
// authorized the exact transaction and input sighash the Go host is about to
// sign.
func bindTaprootTxViaNativeSigner(
	sessionID string,
	unsignedTx *bitcoin.TransactionBuilder,
	inputIndex int,
	expectedSighash *big.Int,
) error {
	if expectedSighash == nil {
		return fmt.Errorf("expected sighash is nil")
	}

	result, err := buildTaprootTxForSessionFn(sessionID, unsignedTx)
	if err != nil {
		return err
	}

	expectedTxHex := hex.EncodeToString(unsignedTx.UnsignedTransaction().Serialize())
	if result.TxHex != expectedTxHex {
		return fmt.Errorf(
			"native tbtc-signer BuildTaprootTx returned a different unsigned transaction",
		)
	}
	if inputIndex < 0 || inputIndex >= len(result.TaprootKeySpendSighashesHex) {
		return fmt.Errorf(
			"native tbtc-signer BuildTaprootTx returned [%d] sighashes; input index [%d] is unavailable",
			len(result.TaprootKeySpendSighashesHex),
			inputIndex,
		)
	}

	sighash, err := hex.DecodeString(result.TaprootKeySpendSighashesHex[inputIndex])
	if err != nil || len(sighash) != 32 {
		return fmt.Errorf(
			"native tbtc-signer BuildTaprootTx returned invalid sighash for input [%d]",
			inputIndex,
		)
	}
	if new(big.Int).SetBytes(sighash).Cmp(expectedSighash) != 0 {
		return fmt.Errorf(
			"native tbtc-signer BuildTaprootTx sighash for input [%d] does not match the Go signing message",
			inputIndex,
		)
	}

	return nil
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
