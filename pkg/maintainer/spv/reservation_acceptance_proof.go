package spv

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// SubmitReservationAcceptanceProof prepares the reservation acceptance proof for
// the given transaction and submits it to the on-chain contract.
func SubmitReservationAcceptanceProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
	spvChain Chain,
) error {
	return submitReservationAcceptanceProof(
		transactionHash,
		requiredConfirmations,
		btcChain,
		spvChain,
		bitcoin.AssembleSpvProof,
		getGlobalMetricsRecorder(),
	)
}

func submitReservationAcceptanceProof(
	transactionHash bitcoin.Hash,
	requiredConfirmations uint,
	btcChain bitcoin.Chain,
	spvChain Chain,
	spvProofAssembler spvProofAssembler,
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
	},
) error {
	if requiredConfirmations == 0 {
		return fmt.Errorf("provided required confirmations count must be greater than 0")
	}

	// This is a stub for the SPV proof side, pending final integration.
	return nil
}