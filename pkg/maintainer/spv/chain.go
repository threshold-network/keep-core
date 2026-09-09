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

	// SubmitReservationAcceptanceProof submits an SPV proof for the given
	// reservation acceptance action generation. The call is restricted to
	// the SPV maintainer registered against the Bridge.
	SubmitReservationAcceptanceProof(
		txInfo *tbtc.BitcoinTxInfo,
		proof *tbtc.BitcoinTxProof,
		reservationKey *big.Int,
		requestNonce uint64,
	) error

	// SubmitReservationReanchorProof submits an SPV proof for the given
	// reservation re-anchor action generation. The call is restricted to
	// the SPV maintainer registered against the Bridge.
	SubmitReservationReanchorProof(
		txInfo *tbtc.BitcoinTxInfo,
		proof *tbtc.BitcoinTxProof,
		reservationKey *big.Int,
		requestNonce uint64,
	) error

	// NotifyReservationActionTimeout notifies the Bridge that the timeout
	// for the given Reanchor-type reservation action generation has
	// elapsed without the SPV proof being submitted.
	NotifyReservationActionTimeout(reservationKey *big.Int) error

	// NotifyReservationAcceptanceTimedOut notifies the Bridge that the
	// acceptance-type action timeout has elapsed for the given reservation
	// without the SPV proof being submitted.
	NotifyReservationAcceptanceTimedOut(reservationKey *big.Int) error

	// NotifyStaleReservedDeposit notifies the Bridge that the given reserved
	// deposit's wallet did not anchor it within the reservation-action
	// timeout and should be released back to the default sweeping path.
	NotifyStaleReservedDeposit(depositKey *big.Int) error

	// NotifyReservationStranded notifies the Bridge that the wallet
	// custodying the given reservation has been closed or terminated and
	// the anchor is therefore stranded.
	NotifyReservationStranded(reservationKey *big.Int) error

	// WalletTerminationCause returns the on-chain reason the given wallet
	// was most recently terminated, inferred from the most recent of the
	// three pre-termination timeout events (MovingFundsTimedOut,
	// MovedFundsSweepTimedOut, FraudChallengeDefeatTimedOut) found for it.
	// Returns tbtc.WalletTerminationCauseUnknown (with a nil error) if none
	// of the three events can be found for the wallet.
	WalletTerminationCause(walletPublicKeyHash [20]byte) (tbtc.WalletTerminationCause, error)

	// GetReservation returns the on-chain reservation record. An absent key is represented by ReservationStateUnknown; errors report chain-call or conversion failures.
	GetReservation(reservationKey *big.Int) (*tbtc.Reservation, error)

	// GetReservationAction returns the nonce-bound on-chain action record. An absent generation is represented by ReservationActionStateUnknown; errors report chain-call or conversion failures.
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

	// PastReservationAcceptanceRequestedEvents fetches past
	// ReservationAcceptanceRequested events according to the provided filter
	// or unfiltered if the filter is nil. Returned events are sorted by the
	// block number in the ascending order.
	PastReservationAcceptanceRequestedEvents(
		filter *tbtc.ReservationAcceptanceRequestedEventFilter,
	) ([]*tbtc.ReservationAcceptanceRequestedEvent, error)
	// PastReservationReanchorRequestedEvents fetches past
	// ReservationReanchorRequested events according to the provided filter
	// or unfiltered if the filter is nil. Returned events are sorted by the
	// block number in the ascending order.
	PastReservationReanchorRequestedEvents(
		filter *tbtc.ReservationReanchorRequestedEventFilter,
	) ([]*tbtc.ReservationReanchorRequestedEvent, error)

	// ReservationByAnchorUtxo returns the reservation key whose anchor
	// outpoint is the given Bitcoin transaction output, or a zero value if
	// no reservation is anchored there.
	ReservationByAnchorUtxo(
		anchorTxHash [32]byte,
		anchorTxOutputIndex uint32,
	) (*big.Int, error)
	// PastNewWalletRegisteredEvents fetches past NewWalletRegistered events
	// according to the provided filter or unfiltered if the filter is nil.
	// Returned events are sorted by the block number in the ascending order.
	PastNewWalletRegisteredEvents(
		filter *tbtc.NewWalletRegisteredEventFilter,
	) ([]*tbtc.NewWalletRegisteredEvent, error)

	// BuildDepositKey calculates the key used by the Bridge to store a
	// deposit request, which is a unique identifier for a deposit on-chain.
	BuildDepositKey(fundingTxHash bitcoin.Hash, fundingOutputIndex uint32) *big.Int
}
