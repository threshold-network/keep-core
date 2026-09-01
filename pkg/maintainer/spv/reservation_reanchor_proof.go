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
		getGlobalMetricsRecorder(),
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
		parseReservationReanchorTransactionInput,
	)
}

// spentOutputAsUtxo fetches the single previous output spent by transaction's
// sole input and returns it as an UnspentTransactionOutput. Shared by
// parseReservationAcceptanceTransactionInput and
// parseReservationReanchorTransactionInput, both of which parse a
// 1-input-1-output reservation transaction and need the spent outpoint's
// value to build the SPV proof's main UTXO.
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

// parseReservationReanchorTransactionInput parses the single input and
// single output of a reservation re-anchor transaction and returns the
// anchor UTXO that was spent and the target wallet's public key hash from
// the new anchor output script.
func parseReservationReanchorTransactionInput(
	btcChain bitcoin.Chain,
	transaction *bitcoin.Transaction,
) (*bitcoin.UnspentTransactionOutput, [20]byte, error) {
	anchorUtxo, err := spentOutputAsUtxo(btcChain, transaction)
	if err != nil {
		return nil, [20]byte{}, err
	}

	if len(transaction.Outputs) != 1 {
		return nil, [20]byte{}, fmt.Errorf(
			"reservation re-anchor transaction must have exactly one output",
		)
	}

	targetWalletPublicKeyHash, err := bitcoin.ExtractPublicKeyHash(
		transaction.Outputs[0].PublicKeyScript,
	)
	if err != nil {
		return nil, [20]byte{}, fmt.Errorf(
			"cannot extract target wallet public key hash: [%v]",
			err,
		)
	}

	return anchorUtxo, targetWalletPublicKeyHash, nil
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

// buildReservationProofMainUtxo packages the spent anchor UTXO into the
// BitcoinTxUTXO structure expected by SubmitReservationProof.
func buildReservationProofMainUtxo(
	anchorUtxo *bitcoin.UnspentTransactionOutput,
) *tbtc.BitcoinTxUTXO {
	var (
		txHash     [32]byte
		txOutIndex uint32
		txOutValue uint64
	)

	if anchorUtxo.Outpoint != nil {
		txHash = anchorUtxo.Outpoint.TransactionHash
		txOutIndex = anchorUtxo.Outpoint.OutputIndex
	}
	if anchorUtxo.Value < 0 {
		txOutValue = 0
	} else {
		txOutValue = uint64(anchorUtxo.Value)
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
	inputParser func(
		btcChain bitcoin.Chain,
		transaction *bitcoin.Transaction,
	) (*bitcoin.UnspentTransactionOutput, [20]byte, error),
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
		return fmt.Errorf("reservation key is required")
	}
	if requestNonce == 0 {
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

	anchorUtxo, pkh, err := inputParser(btcChain, transaction)
	if err != nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("error while parsing reservation transaction inputs: [%v]", err)
	}

	action, err := spvChain.GetReservationAction(reservationKey, requestNonce)
	if err != nil {
		return fmt.Errorf("cannot fetch reservation action generation: [%v]", err)
	}

	// Fix 6: Check PKH match
	if pkh != action.TargetWalletPublicKeyHash {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_failed_total", 1)
		}
		return fmt.Errorf("target wallet public key hash mismatch")
	}

	if action.ActionType != expectedActionType {
		return fmt.Errorf("reservation action generation is not expected type")
	}

	if action.State != tbtc.ReservationActionStatePending {
		return fmt.Errorf("reservation action generation is not pending")
	}

	txInfo := buildReservationProofTxInfo(transaction)
	txProof := buildReservationProofTxProof(proof)
	mainUtxo := buildReservationProofMainUtxo(anchorUtxo)

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
		metricsRecorder.IncrementCounter(metricsPrefix+"_submissions_success_total", 1)
	}

	return nil
}
