package spv

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

type Chain interface {
	// SubmitDepositSweepProofWithReimbursement submits the deposit sweep proof
	// via MaintainerProxy. It is used to prove the deposit sweep Bitcoin
	// transaction and update depositors' balances. The caller is reimbursed.
	SubmitDepositSweepProofWithReimbursement(
		transaction *bitcoin.Transaction,
		proof *bitcoin.SpvProof,
		mainUTXO bitcoin.UnspentTransactionOutput,
		vault common.Address,
	) error

	// GetDepositRequest gets the on-chain deposit request for the given
	// funding transaction hash and output index. The returned bool value
	// indicates whether the request was found or not.
	GetDepositRequest(
		fundingTxHash bitcoin.Hash,
		fundingOutputIndex uint32,
	) (*tbtc.DepositChainRequest, bool, error)

	GetWallet(
		walletPublicKeyHash [20]byte,
	) (*tbtc.WalletChainData, error)

	// ComputeMainUtxoHash computes the hash of the provided main UTXO
	// according to the on-chain Bridge rules.
	ComputeMainUtxoHash(mainUtxo *bitcoin.UnspentTransactionOutput) [32]byte

	// TxProofDifficultyFactor returns the number of confirmations on the
	// Bitcoin chain required to successfully evaluate an SPV proof.
	TxProofDifficultyFactor() (*big.Int, error)

	// BlockCounter returns the chain's block counter.
	BlockCounter() (chain.BlockCounter, error)

	// GetPendingRedemptionRequest gets the on-chain pending redemption request
	// for the given wallet public key hash and redeemer output script.
	// The returned bool value indicates whether the request was found or not.
	GetPendingRedemptionRequest(
		walletPublicKeyHash [20]byte,
		redeemerOutputScript bitcoin.Script,
	) (*tbtc.RedemptionRequest, bool, error)

	// GetMovedFundsSweepRequest gets the on-chain moved funds sweep request for
	// the given moving funds transaction hash and output index.
	// The returned bool value indicates whether the request was found or not.
	GetMovedFundsSweepRequest(
		movingFundsTxHash bitcoin.Hash,
		movingFundsTxOutpointIndex uint32,
	) (*tbtc.MovedFundsSweepRequest, bool, error)

	// SubmitRedemptionProofWithReimbursement submits the redemption proof
	// via MaintainerProxy. The caller is reimbursed.
	SubmitRedemptionProofWithReimbursement(
		transaction *bitcoin.Transaction,
		proof *bitcoin.SpvProof,
		mainUTXO bitcoin.UnspentTransactionOutput,
		walletPublicKeyHash [20]byte,
	) error

	// SubmitMovingFundsProofWithReimbursement submits the moving funds proof
	// via MaintainerProxy. The caller is reimbursed.
	SubmitMovingFundsProofWithReimbursement(
		transaction *bitcoin.Transaction,
		proof *bitcoin.SpvProof,
		mainUTXO bitcoin.UnspentTransactionOutput,
		walletPublicKeyHash [20]byte,
	) error

	// SubmitMovedFundsSweepProofWithReimbursement submits the moved funds sweep
	//  proof via MaintainerProxy. The caller is reimbursed.
	SubmitMovedFundsSweepProofWithReimbursement(
		transaction *bitcoin.Transaction,
		proof *bitcoin.SpvProof,
		mainUTXO bitcoin.UnspentTransactionOutput,
	) error

	// SubmitReservationProof submits an SPV proof for the given reservation
	// action generation. proofType selects between Acceptance, Redemption,
	// Reanchor, and Dissolution proofs; m1 invokes only Acceptance (1) and
	// Reanchor (3). The call is restricted to the SPV maintainer registered
	// against the Bridge.
	SubmitReservationProof(
		proofType uint8,
		txInfo *tbtc.BitcoinTxInfo,
		proof *tbtc.BitcoinTxProof,
		mainUtxo *tbtc.BitcoinTxUTXO,
		reservationKey *big.Int,
		requestNonce uint64,
	) error

	// NotifyReservationActionTimeout notifies the Bridge that the timeout
	// for the given reservation action generation has elapsed without the
	// SPV proof being submitted. The walletMembersIDs carry the operator
	// IDs of the wallet that was authorized for the action.
	NotifyReservationActionTimeout(
		reservationKey *big.Int,
		walletMembersIDs []uint32,
	) error

	// NotifyStaleReservedDeposit notifies the Bridge that the given reserved
	// deposit's wallet did not anchor it within the reservation-action
	// timeout and should be released back to the default sweeping path.
	NotifyStaleReservedDeposit(depositKey *big.Int) error

	// NotifyReservationStranded notifies the Bridge that the wallet
	// custodying the given reservation has been closed or terminated and
	// the anchor is therefore stranded.
	NotifyReservationStranded(reservationKey *big.Int) error

	// GetReservation gets the on-chain reservation record for the given
	// reservation key. Returns an error if the reservation was not found.
	GetReservation(reservationKey *big.Int) (*tbtc.Reservation, error)

	// GetReservationAction gets the on-chain action record for the given
	// reservation key and request nonce. Returns an error if the action
	// generation was not found.
	GetReservationAction(
		reservationKey *big.Int,
		requestNonce uint64,
	) (*tbtc.ReservationAction, error)

	// ReservationParameters gets the current on-chain values of the Bridge
	// reservation parameters.
	ReservationParameters() (*tbtc.ReservationParameters, error)

	// WalletReservations returns the reservation keys for all reservations
	// currently custodied by the given wallet.
	WalletReservations(walletPublicKeyHash [20]byte) ([]*big.Int, error)

	// Reservations returns the on-chain reservation request record for the
	// given reservation key. Mirrors the ReservationRouter.reservations
	// view verbatim.
	Reservations(reservationKey *big.Int) (*tbtc.ReservationRequest, error)

	// ReservationActions returns the on-chain reservation action record
	// for the given reservation key and request nonce. Mirrors the
	// ReservationRouter.reservationActions view verbatim.
	ReservationActions(
		reservationKey *big.Int,
		requestNonce uint64,
	) (*tbtc.ReservationActionRecord, error)

	// IsReservedDeposit returns true if the given deposit was revealed
	// with the reservation vault address and is therefore a reservation
	// rather than a default deposit.
	IsReservedDeposit(depositKey *big.Int) (bool, error)

	// ReservedDepositWallet returns the wallet public key hash to which the
	// given reserved deposit was revealed. Returns the zero hash if the
	// deposit is not a reserved deposit.
	ReservedDepositWallet(depositKey *big.Int) ([20]byte, error)

	// PastDepositRevealedEvents fetches past deposit reveal events according
	// to the provided filter or unfiltered if the filter is nil. Returned
	// events are sorted by the block number in the ascending order, i.e. the
	// latest event is at the end of the slice.
	PastDepositRevealedEvents(
		filter *tbtc.DepositRevealedEventFilter,
	) ([]*tbtc.DepositRevealedEvent, error)

	// PastRedemptionRequestedEvents fetches past redemption requested events according
	// to the provided filter or unfiltered if the filter is nil. Returned
	// events are sorted by the block number in the ascending order, i.e. the
	// latest event is at the end of the slice.
	PastRedemptionRequestedEvents(
		filter *tbtc.RedemptionRequestedEventFilter,
	) ([]*tbtc.RedemptionRequestedEvent, error)

	// PastMovingFundsCommitmentSubmittedEvents fetches past moving funds
	// commitment submitted events according to the provided filter or
	// unfiltered if the filter is nil. Returned events are sorted by the block
	// number in the ascending order, i.e. the latest event is at the end of the
	// slice.
	PastMovingFundsCommitmentSubmittedEvents(
		filter *tbtc.MovingFundsCommitmentSubmittedEventFilter,
	) ([]*tbtc.MovingFundsCommitmentSubmittedEvent, error)

	// PastReservationAcceptedEvents fetches past ReservationAccepted events
	// according to the provided filter or unfiltered if the filter is nil.
	// Returned events are sorted by the block number in the ascending order.
	PastReservationAcceptedEvents(
		filter *tbtc.ReservationAcceptedEventFilter,
	) ([]*tbtc.ReservationAcceptedEvent, error)

	// PastReservationReanchoredEvents fetches past ReservationReanchored
	// events according to the provided filter or unfiltered if the filter is
	// nil. Returned events are sorted by the block number in the ascending order.
	PastReservationReanchoredEvents(
		filter *tbtc.ReservationReanchoredEventFilter,
	) ([]*tbtc.ReservationReanchoredEvent, error)

	// PastReservationActionTimedOutEvents fetches past ReservationActionTimedOut
	// events according to the provided filter or unfiltered if the filter is nil.
	// Returned events are sorted by the block number in the ascending order.
	PastReservationActionTimedOutEvents(
		filter *tbtc.ReservationActionTimedOutEventFilter,
	) ([]*tbtc.ReservationActionTimedOutEvent, error)
}
