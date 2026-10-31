package spv

import (
	"math/big"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// SubmitReservationAcceptanceProof drives the SPV proof submission for a
// reservation acceptance action generation. The caller (the reservation
// proof loop) supplies the (reservationKey, requestNonce) pair of the
// on-chain action generation it is proving, plus the Bitcoin transaction
// hash of the anchor transaction already signed and broadcast by the wallet
// coordinator. The proof is fetched from btcChain, the anchor transaction
// is rebuilt locally to extract the deposit UTXO that was anchored, and the
// proof is submitted directly to the Bridge via the SPV maintainer's
// SubmitReservationAcceptanceProof entry point (not via MaintainerProxy:
// reservations are not reimbursed).
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
	metricsRecorder MetricsRecorder,
) error {
	return submitReservationAcceptanceProof(
		transactionHash,
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		bitcoin.AssembleSpvProof,
		metricsRecorder,
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
	metricsRecorder MetricsRecorder,
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
		spvChain.SubmitReservationAcceptanceProof,
		"reservation_acceptance_proof",
		tbtc.ReservationActionTypeAcceptance,
		"acceptance",
	)
}
