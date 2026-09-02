package spv

import (
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
	return submitReservationActionProof(
		transactionHash,
		requiredConfirmations,
		reservationKey,
		requestNonce,
		btcChain,
		spvChain,
		spvProofAssembler,
		metricsRecorder,
		ProofTypeReservationAcceptance,
		"reservation_acceptance_proof",
		tbtc.ReservationActionTypeAcceptance,
		"acceptance",
	)
}
