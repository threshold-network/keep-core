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
// reservation re-anchor action generation. The caller (typically the
// reservation re-anchor watcher) supplies the (reservationKey, requestNonce)
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
) error {
	if requiredConfirmations == 0 {
		return fmt.Errorf(
			"provided required confirmations count must be greater than 0",
		)
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
		return fmt.Errorf(
			"failed to assemble transaction spv proof: [%v]",
			err,
		)
	}

	anchorUtxo, _, err := parseReservationReanchorTransactionInput(
		btcChain,
		transaction,
	)
	if err != nil {
		return fmt.Errorf(
			"error while parsing reservation re-anchor transaction inputs: [%v]",
			err,
		)
	}

	action, err := spvChain.GetReservationAction(reservationKey, requestNonce)
	if err != nil {
		return fmt.Errorf(
			"cannot fetch reservation action generation: [%v]",
			err,
		)
	}

	if action.ActionType != tbtc.ReservationActionTypeReanchor {
		return fmt.Errorf(
			"reservation action generation is not a re-anchor (got %v)",
			action.ActionType,
		)
	}

	if action.State != tbtc.ReservationActionStatePending {
		return fmt.Errorf(
			"reservation re-anchor action generation is not pending (state=%v)",
			action.State,
		)
	}

	txInfo := buildReservationProofTxInfo(transaction)
	txProof := buildReservationProofTxProof(proof)
	mainUtxo := buildReservationProofMainUtxo(anchorUtxo)

	if err := spvChain.SubmitReservationProof(
		ProofTypeReservationReanchor,
		txInfo,
		txProof,
		mainUtxo,
		reservationKey,
		requestNonce,
	); err != nil {
		return fmt.Errorf(
			"failed to submit reservation re-anchor proof: [%v]",
			err,
		)
	}

	return nil
}

// parseReservationReanchorTransactionInput parses the single input and
// single output of a reservation re-anchor transaction and returns the
// anchor UTXO that was spent and the target wallet's public key hash from
// the new anchor output script.
func parseReservationReanchorTransactionInput(
	btcChain bitcoin.Chain,
	transaction *bitcoin.Transaction,
) (*bitcoin.UnspentTransactionOutput, [20]byte, error) {
	if len(transaction.Inputs) != 1 {
		return nil, [20]byte{}, fmt.Errorf(
			"reservation re-anchor transaction must have exactly one input",
		)
	}

	if len(transaction.Outputs) != 1 {
		return nil, [20]byte{}, fmt.Errorf(
			"reservation re-anchor transaction must have exactly one output",
		)
	}

	input := transaction.Inputs[0]

	inputTx, err := btcChain.GetTransaction(input.Outpoint.TransactionHash)
	if err != nil {
		return nil, [20]byte{}, fmt.Errorf(
			"cannot get input transaction data: [%v]",
			err,
		)
	}

	if int(input.Outpoint.OutputIndex) >= len(inputTx.Outputs) {
		return nil, [20]byte{}, fmt.Errorf(
			"input outpoint index [%d] out of range for transaction [%d] "+
				"outputs",
			input.Outpoint.OutputIndex,
			len(inputTx.Outputs),
		)
	}

	spentOutput := inputTx.Outputs[input.Outpoint.OutputIndex]

	anchorUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: input.Outpoint,
		Value:    spentOutput.Value,
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
