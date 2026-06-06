package tbtcpg

import (
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// Chain represents the interface that the wallet maintainer module expects
// to interact with the anchoring blockchain on.
type Chain interface {
	tbtc.BridgeChain

	// PastNewWalletRegisteredEvents fetches past new wallet registered events
	// according to the provided filter or unfiltered if the filter is nil. Returned
	// events are sorted by the block number in the ascending order, i.e. the
	// latest event is at the end of the slice.
	PastNewWalletRegisteredEvents(
		filter *tbtc.NewWalletRegisteredEventFilter,
	) ([]*tbtc.NewWalletRegisteredEvent, error)

	// GetWalletParameters gets the current value of parameters relevant to
	// wallet.
	GetWalletParameters() (
		creationPeriod uint32,
		creationMinBtcBalance uint64,
		creationMaxBtcBalance uint64,
		closureMinBtcBalance uint64,
		maxAge uint32,
		maxBtcTransfer uint64,
		closingPeriod uint32,
		err error,
	)

	// GetLiveWalletsCount gets the current count of live wallets.
	GetLiveWalletsCount() (uint32, error)

	// BuildDepositKey calculates a deposit key for the given funding transaction
	// which is a unique identifier for a deposit on-chain.
	BuildDepositKey(fundingTxHash bitcoin.Hash, fundingOutputIndex uint32) *big.Int

	// GetDepositParameters gets the current value of parameters relevant
	// for the depositing process.
	GetDepositParameters() (
		dustThreshold uint64,
		treasuryFeeDivisor uint64,
		txMaxFee uint64,
		revealAheadPeriod uint32,
		err error,
	)

	// PastRedemptionRequestedEvents fetches past redemption requested events according
	// to the provided filter or unfiltered if the filter is nil. Returned
	// events are sorted by the block number in the ascending order, i.e. the
	// latest event is at the end of the slice.
	PastRedemptionRequestedEvents(
		filter *tbtc.RedemptionRequestedEventFilter,
	) ([]*tbtc.RedemptionRequestedEvent, error)

	// BuildRedemptionKey calculates a redemption key for the given redemption
	// request which is an identifier for a redemption at the given time
	// on-chain.
	BuildRedemptionKey(
		walletPublicKeyHash [20]byte,
		redeemerOutputScript bitcoin.Script,
	) (*big.Int, error)

	// GetRedemptionParameters gets the current value of parameters relevant
	// for the redemption process.
	GetRedemptionParameters() (
		dustThreshold uint64,
		treasuryFeeDivisor uint64,
		txMaxFee uint64,
		txMaxTotalFee uint64,
		timeout uint32,
		timeoutSlashingAmount *big.Int,
		timeoutNotifierRewardMultiplier uint32,
		err error,
	)

	// GetRedemptionMaxSize gets the maximum number of redemption requests that
	// can be a part of a redemption sweep proposal.
	GetRedemptionMaxSize() (uint16, error)

	// GetRedemptionRequestMinAge get the  minimum time that must elapse since
	// the redemption request creation before a request becomes eligible for
	// a processing.
	GetRedemptionRequestMinAge() (uint32, error)

	// ValidateDepositSweepProposal validates the given deposit sweep proposal
	// against the chain. It requires some additional data about the deposits
	// that must be fetched externally. Returns an error if the proposal is
	// not valid or nil otherwise.
	ValidateDepositSweepProposal(
		walletPublicKeyHash [20]byte,
		proposal *tbtc.DepositSweepProposal,
		depositsExtraInfo []struct {
			*tbtc.Deposit
			FundingTx *bitcoin.Transaction
		},
	) error

	// ValidateTaprootDepositSweepProposal validates the given Taproot deposit
	// sweep proposal against the chain. It requires some additional data about
	// the deposits that must be fetched externally. Returns an error if the
	// proposal is not valid or nil otherwise.
	ValidateTaprootDepositSweepProposal(
		walletPublicKeyHash [20]byte,
		proposal *tbtc.DepositSweepProposal,
		depositsExtraInfo []struct {
			*tbtc.Deposit
			FundingTx *bitcoin.Transaction
		},
	) error

	// ValidateRedemptionProposal validates the given redemption proposal
	// against the chain. Returns an error if the proposal is not valid or
	// nil otherwise.
	ValidateRedemptionProposal(
		walletPublicKeyHash [20]byte,
		proposal *tbtc.RedemptionProposal,
	) error

	// GetDepositSweepMaxSize gets the maximum number of deposits that can
	// be part of a deposit sweep proposal.
	GetDepositSweepMaxSize() (uint16, error)

	BlockCounter() (chain.BlockCounter, error)

	AverageBlockTime() time.Duration

	// GetOperatorID returns the operator ID for the given operator address.
	GetOperatorID(operatorAddress chain.Address) (chain.OperatorID, error)

	// ValidateHeartbeatProposal validates the given heartbeat proposal
	// against the chain. Returns an error if the proposal is not valid or
	// nil otherwise.
	ValidateHeartbeatProposal(
		walletPublicKeyHash [20]byte,
		proposal *tbtc.HeartbeatProposal,
	) error

	// PastMovingFundsCommitmentSubmittedEvents fetches past moving funds
	// commitment submitted events according to the provided filter or
	// unfiltered if the filter is nil. Returned events are sorted by the block
	// number in the ascending order, i.e. the latest event is at the end of the
	// slice.
	PastMovingFundsCommitmentSubmittedEvents(
		filter *tbtc.MovingFundsCommitmentSubmittedEventFilter,
	) ([]*tbtc.MovingFundsCommitmentSubmittedEvent, error)

	// PastMovingFundsCompletedEvents fetches past moving funds completed events
	// according to the provided filter or unfiltered if the filter is nil.
	// Returned events are sorted by the block number in the ascending order,
	// i.e. the latest event is at the end of the slice.
	PastMovingFundsCompletedEvents(
		filter *tbtc.MovingFundsCompletedEventFilter,
	) ([]*tbtc.MovingFundsCompletedEvent, error)

	// ValidateMovingFundsProposal validates the given moving funds proposal
	// against the chain. Returns an error if the proposal is not valid or
	// nil otherwise.
	ValidateMovingFundsProposal(
		walletPublicKeyHash [20]byte,
		mainUTXO *bitcoin.UnspentTransactionOutput,
		proposal *tbtc.MovingFundsProposal,
	) error

	// Submits the moving funds target wallets commitment.
	SubmitMovingFundsCommitment(
		walletPublicKeyHash [20]byte,
		walletMainUTXO bitcoin.UnspentTransactionOutput,
		walletMembersIDs []uint32,
		walletMemberIndex uint32,
		targetWallets [][20]byte,
	) error

	// ValidateMovedFundsSweepProposal validates the given moved funds sweep
	// proposal against the chain. Returns an error if the proposal is not valid
	// or nil otherwise.
	ValidateMovedFundsSweepProposal(
		walletPublicKeyHash [20]byte,
		proposal *tbtc.MovedFundsSweepProposal,
	) error

	// Computes the moving funds commitment hash from the provided public key
	// hashes of target wallets.
	ComputeMovingFundsCommitmentHash(targetWallets [][20]byte) [32]byte

	// GetRedemptionDelay returns the processing delay for the given redemption.
	GetRedemptionDelay(
		walletPublicKeyHash [20]byte,
		redeemerOutputScript bitcoin.Script,
	) (time.Duration, error)

	// GetDepositMinAge get the minimum time that must elapse since
	// the deposit reveal before a deposit becomes eligible for
	// a processing.
	GetDepositMinAge() (uint32, error)
}
