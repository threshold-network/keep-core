package spv

import (
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ProofTypeReservationAcceptance is the value passed to
// SubmitReservationProof as proofType for a reservation acceptance SPV
// proof. The numeric value mirrors the on-chain ReservationProofType enum
// (1 = Acceptance).
const ProofTypeReservationAcceptance uint8 = 1

// SubmitReservationAcceptanceProof drives the SPV proof submission for a
// reservation acceptance action generation. The caller (the reservation
// proof loop) supplies the (reservationKey, requestNonce) pair of the
// on-chain action generation it is proving, plus the Bitcoin transaction
// hash of the anchor transaction already signed and broadcast by the wallet
// coordinator. The proof is fetched from btcChain, the anchor transaction
// is rebuilt locally to extract the deposit UTXO that was anchored, and the
// proof is submitted directly to the Bridge via the SPV maintainer's
// SubmitReservationProof entry point (not via MaintainerProxy: reservations
// are not reimbursed).
//
// requiredConfirmations must be > 0; the SPV maintainer relies on it to
// assemble the proof.
func SubmitReservationAcceptanceProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	reservationKey *big.Int,
	requestNonce uint64,
	btcChain bitcoin.Chain,
	spvChain Chain,
) error {
	return submitReservationAcceptanceProof(
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

func submitReservationAcceptanceProof(
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
	if metricsRecorder != nil {
		metricsRecorder.IncrementCounter(
			"reservation_acceptance_proof_submissions_total",
			1,
		)
	}

	if requiredConfirmations == 0 {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(
				"reservation_acceptance_proof_submissions_failed_total",
				1,
			)
		}
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
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(
				"reservation_acceptance_proof_submissions_failed_total",
				1,
			)
		}
		return fmt.Errorf(
			"failed to assemble transaction spv proof: [%v]",
			err,
		)
	}

	depositUtxo, err := parseReservationAcceptanceTransactionInput(
		btcChain,
		transaction,
	)
	if err != nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(
				"reservation_acceptance_proof_submissions_failed_total",
				1,
			)
		}
		return fmt.Errorf(
			"error while parsing reservation acceptance transaction "+
				"inputs: [%v]",
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

	if action.ActionType != tbtc.ReservationActionTypeAcceptance {
		return fmt.Errorf(
			"reservation action generation is not an acceptance (got %v)",
			action.ActionType,
		)
	}

	if action.State != tbtc.ReservationActionStatePending {
		return fmt.Errorf(
			"reservation acceptance action generation is not pending "+
				"(state=%v)",
			action.State,
		)
	}

	txInfo := buildReservationProofTxInfo(transaction)
	txProof := buildReservationProofTxProof(proof)
	mainUtxo := buildReservationProofMainUtxo(depositUtxo)

	if err := spvChain.SubmitReservationProof(
		ProofTypeReservationAcceptance,
		txInfo,
		txProof,
		mainUtxo,
		reservationKey,
		requestNonce,
	); err != nil {
		if metricsRecorder != nil {
			metricsRecorder.IncrementCounter(
				"reservation_acceptance_proof_submissions_failed_total",
				1,
			)
		}
		return fmt.Errorf(
			"failed to submit reservation acceptance proof: [%v]",
			err,
		)
	}

	if metricsRecorder != nil {
		metricsRecorder.IncrementCounter(
			"reservation_acceptance_proof_submissions_success_total",
			1,
		)
	}

	return nil
}

// parseReservationAcceptanceTransactionInput parses the single input of a
// reservation acceptance (anchor) transaction and returns the deposit UTXO
// that was anchored. Mirrors parseReservationReanchorTransactionInput in
// reservation_reanchor_proof.go.
func parseReservationAcceptanceTransactionInput(
	btcChain bitcoin.Chain,
	transaction *bitcoin.Transaction,
) (*bitcoin.UnspentTransactionOutput, error) {
	if len(transaction.Inputs) != 1 {
		return nil, fmt.Errorf(
			"reservation acceptance transaction must have exactly one input",
		)
	}

	if len(transaction.Outputs) != 1 {
		return nil, fmt.Errorf(
			"reservation acceptance transaction must have exactly one output",
		)
	}

	input := transaction.Inputs[0]

	inputTx, err := btcChain.GetTransaction(input.Outpoint.TransactionHash)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get input transaction data: [%v]",
			err,
		)
	}

	if int(input.Outpoint.OutputIndex) >= len(inputTx.Outputs) {
		return nil, fmt.Errorf(
			"input outpoint index [%d] out of range for transaction [%d] "+
				"outputs",
			input.Outpoint.OutputIndex,
			len(inputTx.Outputs),
		)
	}

	spentOutput := inputTx.Outputs[input.Outpoint.OutputIndex]

	depositUtxo := &bitcoin.UnspentTransactionOutput{
		Outpoint: input.Outpoint,
		Value:    spentOutput.Value,
	}

	return depositUtxo, nil
}
