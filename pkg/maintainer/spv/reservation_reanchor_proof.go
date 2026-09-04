package spv

import (
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ProofTypeReservationReanchor is the value passed to
// SubmitReservationProof as proofType for a reservation re-anchor SPV proof.
// The numeric value mirrors the on-chain ReservationProofType enum (3 =
// Reanchor).
const ProofTypeReservationReanchor uint8 = 3

// SubmitReservationReanchorProof drives the SPV proof submission for a
// reservation re-anchor action generation. The caller (the reservation
// proof loop) supplies the (reservationKey, requestNonce)
// pair of the on-chain action generation it is proving, plus the Bitcoin
// transaction hash of the re-anchor transaction already signed and
// broadcast by the wallet coordinator. The proof is fetched from btcChain,
// the re-anchor transaction is rebuilt locally to extract the anchor UTXO
// and target wallet, and the proof is submitted directly to the Bridge via
// the SPV maintainer's SubmitReservationProof entry point (not via
// MaintainerProxy: reservations are not reimbursed).
//
// requiredConfirmations must be > 0; the SPV maintainer relies on it to
// assemble the proof.
func SubmitReservationReanchorProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	reservationKey *big.Int,
	requestNonce uint64,
	btcChain bitcoin.Chain,
	spvChain Chain,
) error {
	return submitReservationReanchorProof(
		transactionHash,
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		bitcoin.AssembleSpvProof,
		getMetricsRecorder(),
	)
}

func submitReservationReanchorProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	reservationKey *big.Int,
	requestNonce uint64,
	btcChain bitcoin.Chain,
	spvChain Chain,
	spvProofAssembler spvProofAssembler,
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
	},
) error {
	return submitReservationActionProof(
		transactionHash,
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		spvProofAssembler,
		metricsRecorder,
		ProofTypeReservationReanchor,
		"reservation_reanchor_proof",
		tbtc.ReservationActionTypeReanchor,
		"re-anchor",
	)
}

// spentOutputAsUtxo fetches the single previous output spent by transaction's
// sole input and returns it as an UnspentTransactionOutput. Shared by
// parseReservationTransaction, which parses a 1-input-1-output reservation
// transaction and needs the spent outpoint's value to build the SPV proof's
// main UTXO.
func spentOutputAsUtxo(
	btcChain bitcoin.Chain,
	transaction *bitcoin.Transaction,
) (*bitcoin.UnspentTransactionOutput, error) {
	if len(transaction.Inputs) != 1 {
		return nil, fmt.Errorf(
			"reservation transaction must have exactly one input",
		)
	}

	spentOutpoint := transaction.Inputs[0].Outpoint

	previousTransaction, err := btcChain.GetTransaction(
		spentOutpoint.TransactionHash,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot fetch previous transaction: [%v]",
			err,
		)
	}

	if int(spentOutpoint.OutputIndex) >= len(previousTransaction.Outputs) {
		return nil, fmt.Errorf(
			"spent output index [%v] out of bounds for previous "+
				"transaction with [%v] outputs",
			spentOutpoint.OutputIndex,
			len(previousTransaction.Outputs),
		)
	}

	spentOutput := previousTransaction.Outputs[spentOutpoint.OutputIndex]

	return &bitcoin.UnspentTransactionOutput{
		Outpoint: spentOutpoint,
		Value:    spentOutput.Value,
	}, nil
}

// parseReservationTransaction parses the single input and single output
// of a reservation transaction and returns the UTXO that was spent and
// the target wallet's public key hash from the new output script.
func parseReservationTransaction(
	btcChain bitcoin.Chain,
	transaction *bitcoin.Transaction,
	txType string,
) (*bitcoin.UnspentTransactionOutput, [20]byte, error) {
	utxo, err := spentOutputAsUtxo(btcChain, transaction)
	if err != nil {
		return nil, [20]byte{}, err
	}

	if len(transaction.Outputs) != 1 {
		return nil, [20]byte{}, fmt.Errorf(
			"reservation %v transaction must have exactly one output",
			txType,
		)
	}

	publicKeyHash, err := bitcoin.ExtractPublicKeyHash(
		transaction.Outputs[0].PublicKeyScript,
	)
	if err != nil {
		return nil, [20]byte{}, fmt.Errorf(
			"cannot extract %v public key hash: [%v]",
			txType,
			err,
		)
	}

	return utxo, publicKeyHash, nil
}

// buildReservationProofTxInfo serializes the relevant parts of the
// transaction into the BitcoinTxInfo structure expected by
// SubmitReservationProof.
func buildReservationProofTxInfo(
	transaction *bitcoin.Transaction,
) *tbtc.BitcoinTxInfo {
	return &tbtc.BitcoinTxInfo{
		Version:      transaction.SerializeVersion(),
		InputVector:  transaction.SerializeInputs(),
		OutputVector: transaction.SerializeOutputs(),
		Locktime:     transaction.SerializeLocktime(),
	}
}

// buildReservationProofTxProof converts a bitcoin.SpvProof into the
// BitcoinTxProof structure expected by SubmitReservationProof.
func buildReservationProofTxProof(
	proof *bitcoin.SpvProof,
) *tbtc.BitcoinTxProof {
	txIndexInBlock := big.NewInt(int64(proof.TxIndexInBlock))

	return &tbtc.BitcoinTxProof{
		MerkleProof:      proof.MerkleProof,
		TxIndexInBlock:   txIndexInBlock,
		BitcoinHeaders:   proof.BitcoinHeaders,
		CoinbasePreimage: proof.CoinbasePreimage,
		CoinbaseProof:    proof.CoinbaseProof,
	}
}

// buildReservationProofMainUtxo packages the spent deposit or anchor UTXO
// into the BitcoinTxUTXO structure expected by SubmitReservationProof.
//
// IMPORTANT: The mainUtxo parameter is INERT IN MILESTONE 1. Per
// ReservationRouter.sol's devdoc on the tbtc-v2 reservations-upgrade branch:
// "Unused in milestone 1; Dissolution proofs are rejected by the underlying
// library. Reserved for milestone 2." The underlying ReservationProofs.sol
// library has zero references to mainUtxo. The current value passed is the
// spent deposit/anchor outpoint, which is harmless for m1 but a future
// milestone-2 activation MUST revisit what value is actually correct here.
// Do NOT change this value without updating the corresponding test assertion.
func buildReservationProofMainUtxo(
	spentUtxo *bitcoin.UnspentTransactionOutput,
) *tbtc.BitcoinTxUTXO {
	var (
		txHash     [32]byte
		txOutIndex uint32
		txOutValue uint64
	)

	if spentUtxo.Outpoint != nil {
		txHash = spentUtxo.Outpoint.TransactionHash
		txOutIndex = spentUtxo.Outpoint.OutputIndex
	}
	if spentUtxo.Value < 0 {
		txOutValue = 0
	} else {
		txOutValue = uint64(spentUtxo.Value)
	}

	return &tbtc.BitcoinTxUTXO{
		TxHash:        txHash,
		TxOutputIndex: txOutIndex,
		TxOutputValue: txOutValue,
	}
}

func submitReservationActionProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	reservationKey *big.Int,
	requestNonce uint64,
	btcChain bitcoin.Chain,
	spvChain Chain,
	spvProofAssembler spvProofAssembler,
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
	},
	proofType uint8,
	metricsPrefix string,
	expectedActionType tbtc.ReservationActionType,
	txType string,
) error {
	if metricsRecorder != nil {
		metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_total", 1)
	}

	if requiredConfirmations == 0 {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("provided required confirmations count must be greater than 0")
	}
	if reservationKey == nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("reservation key is required")
	}
	if requestNonce == 0 {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("request nonce must be > 0")
	}

	transaction, proof, err := spvProofAssembler(
		transactionHash,
		requiredConfirmations,
		btcChain,
	)
	if err != nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("failed to assemble transaction spv proof: [%v]", err)
	}

	utxo, pkh, err := parseReservationTransaction(btcChain, transaction, txType)
	if err != nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("error while parsing reservation transaction inputs: [%v]", err)
	}

	action, err := spvChain.GetReservationAction(reservationKey, requestNonce)
	if err != nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("cannot fetch reservation action generation: [%v]", err)
	}

	// The action snapshot is the on-chain authorization for the destination, so verify the PKH match.
	if pkh != action.TargetWalletPublicKeyHash {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("target wallet public key hash mismatch")
	}

	if action.ActionType != expectedActionType {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("reservation action generation is not expected type")
	}

	if action.State != tbtc.ReservationActionStatePending {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("reservation action generation is not pending")
	}

	txInfo := buildReservationProofTxInfo(transaction)
	txProof := buildReservationProofTxProof(proof)
	mainUtxo := buildReservationProofMainUtxo(utxo)

	if err := spvChain.SubmitReservationProof(
		proofType,
		txInfo,
		txProof,
		mainUtxo,
		reservationKey,
		requestNonce,
	); err != nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("failed to submit reservation proof: [%v]", err)
	}

	if metricsRecorder != nil {
		metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_succeeded_total", 1)
	}

	return nil
}
