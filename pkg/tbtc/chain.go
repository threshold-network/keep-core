package tbtc

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity"
	"github.com/keep-network/keep-core/pkg/sortition"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tecdsa/dkg"
)

type DKGState int

var ErrWalletNotFound = errors.New("wallet not found")

const (
	Idle DKGState = iota
	AwaitingSeed
	AwaitingResult
	Challenge
)

// GroupSelectionChain defines the subset of the TBTC chain interface that
// pertains to the group selection activities.
type GroupSelectionChain interface {
	// SelectGroup returns the group members selected for the current group
	// selection. The function returns an error if the chain's state does not
	// allow for group selection at the moment.
	SelectGroup() (*GroupSelectionResult, error)
}

// GroupSelectionResult represents a group selection result, i.e. operators
// selected to perform the DKG protocol. The result consists of two slices
// of equal length holding the chain.OperatorID and chain.Address for each
// selected operator.
type GroupSelectionResult struct {
	OperatorsIDs       chain.OperatorIDs
	OperatorsAddresses chain.Addresses
}

// DistributedKeyGenerationChain defines the subset of the TBTC chain
// interface that pertains specifically to group formation's distributed key
// generation process.
type DistributedKeyGenerationChain interface {
	// OnDKGStarted registers a callback that is invoked when an on-chain
	// notification of the DKG process start is seen.
	OnDKGStarted(
		func(event *DKGStartedEvent),
	) subscription.EventSubscription

	// PastDKGStartedEvents fetches past DKG started events according to the
	// provided filter or unfiltered if the filter is nil. Returned events
	// are sorted by the block number in the ascending order, i.e. the latest
	// event is at the end of the slice.
	PastDKGStartedEvents(
		filter *DKGStartedEventFilter,
	) ([]*DKGStartedEvent, error)

	// OnDKGResultSubmitted registers a callback that is invoked when an on-chain
	// notification of the DKG result submission is seen.
	OnDKGResultSubmitted(
		func(event *DKGResultSubmittedEvent),
	) subscription.EventSubscription

	// OnDKGResultChallenged registers a callback that is invoked when an
	// on-chain notification of the DKG result challenge is seen.
	OnDKGResultChallenged(
		func(event *DKGResultChallengedEvent),
	) subscription.EventSubscription

	// OnDKGResultApproved registers a callback that is invoked when an on-chain
	// notification of the DKG result approval is seen.
	OnDKGResultApproved(
		func(event *DKGResultApprovedEvent),
	) subscription.EventSubscription

	// AssembleDKGResult assembles the DKG chain result according to the rules
	// expected by the given chain.
	AssembleDKGResult(
		submitterMemberIndex group.MemberIndex,
		groupPublicKey *ecdsa.PublicKey,
		operatingMembersIndexes []group.MemberIndex,
		misbehavedMembersIndexes []group.MemberIndex,
		signatures map[group.MemberIndex][]byte,
		groupSelectionResult *GroupSelectionResult,
	) (*DKGChainResult, error)

	// SubmitDKGResult submits the DKG result to the chain.
	SubmitDKGResult(dkgResult *DKGChainResult) error

	// GetDKGState returns the current state of the DKG procedure.
	GetDKGState() (DKGState, error)

	// CalculateDKGResultSignatureHash calculates a 32-byte hash that is used
	// to produce a signature supporting the given groupPublicKey computed
	// as result of the given DKG process. The misbehavedMembersIndexes parameter
	// should contain indexes of members that were considered as misbehaved
	// during the DKG process. The startBlock argument is the block at which
	// the given DKG process started.
	CalculateDKGResultSignatureHash(
		groupPublicKey *ecdsa.PublicKey,
		misbehavedMembersIndexes []group.MemberIndex,
		startBlock uint64,
	) (dkg.ResultSignatureHash, error)

	// IsDKGResultValid checks whether the submitted DKG result is valid from
	// the on-chain contract standpoint.
	IsDKGResultValid(dkgResult *DKGChainResult) (bool, error)

	// ChallengeDKGResult challenges the submitted DKG result.
	ChallengeDKGResult(dkgResult *DKGChainResult) error

	// ApproveDKGResult approves the submitted DKG result.
	ApproveDKGResult(dkgResult *DKGChainResult) error

	// DKGParameters gets the current value of DKG-specific control parameters.
	DKGParameters() (*DKGParameters, error)
}

// InactivityClaimedEvent represents an inactivity claimed event. It is emitted
// after a submitted inactivity claim lands on the chain.
type InactivityClaimedEvent struct {
	WalletID    [32]byte
	Nonce       *big.Int
	Notifier    chain.Address
	BlockNumber uint64
}

// InactivityClaim represents an inactivity claim submitted to the chain.
type InactivityClaim struct {
	WalletID               [32]byte
	InactiveMembersIndices []group.MemberIndex
	HeartbeatFailed        bool
	Signatures             []byte
	SigningMembersIndices  []group.MemberIndex
}

type InactivityClaimChain interface {
	// OnInactivityClaimed registers a callback that is invoked when an on-chain
	// notification of the inactivity claim submission is seen.
	OnInactivityClaimed(
		func(event *InactivityClaimedEvent),
	) subscription.EventSubscription

	// AssembleInactivityClaim assembles the inactivity chain claim according to
	// the rules expected by the given chain.
	AssembleInactivityClaim(
		walletID [32]byte,
		inactiveMembersIndices []group.MemberIndex,
		signatures map[group.MemberIndex][]byte,
		heartbeatFailed bool,
	) (*InactivityClaim, error)

	// SubmitInactivityClaim submits the inactivity claim to the chain.
	SubmitInactivityClaim(
		claim *InactivityClaim,
		nonce *big.Int,
		groupMembers []uint32,
	) error

	// CalculateInactivityClaimHash calculates hash for the given inactivity
	// claim.
	CalculateInactivityClaimHash(claim *inactivity.ClaimPreimage) (
		inactivity.ClaimHash,
		error,
	)

	// GetInactivityClaimNonce returns inactivity claim nonce for the given
	// wallet.
	GetInactivityClaimNonce(walletID [32]byte) (*big.Int, error)
}

// DKGChainResultHash represents a hash of the DKGChainResult. The algorithm
// used is specific to the chain.
type DKGChainResultHash [32]byte

// DKGChainResult represents a DKG result submitted to the chain.
type DKGChainResult struct {
	SubmitterMemberIndex     group.MemberIndex
	GroupPublicKey           []byte
	MisbehavedMembersIndexes []group.MemberIndex
	Signatures               []byte
	SigningMembersIndexes    []group.MemberIndex
	Members                  chain.OperatorIDs
	MembersHash              [32]byte
}

// DKGStartedEvent represents a DKG start event.
type DKGStartedEvent struct {
	Seed        *big.Int
	BlockNumber uint64
}

// DKGStartedEventFilter is a component allowing to filter DKGStartedEvent.
type DKGStartedEventFilter struct {
	StartBlock uint64
	EndBlock   *uint64
	Seed       []*big.Int
}

// DKGResultSubmittedEvent represents a DKG result submission event. It is
// emitted after a submitted DKG result lands on the chain.
type DKGResultSubmittedEvent struct {
	Seed        *big.Int
	ResultHash  DKGChainResultHash
	Result      *DKGChainResult
	BlockNumber uint64
}

// DKGResultChallengedEvent represents a DKG result challenge event. It is
// emitted after a submitted DKG result is challenged as an invalid result.
type DKGResultChallengedEvent struct {
	ResultHash  DKGChainResultHash
	Challenger  chain.Address
	Reason      string
	BlockNumber uint64
}

// DKGResultApprovedEvent represents a DKG result approval event. It is
// emitted after a submitted DKG result is approved as a valid result.
type DKGResultApprovedEvent struct {
	ResultHash  DKGChainResultHash
	Approver    chain.Address
	BlockNumber uint64
}

// DKGParameters contains values of DKG-specific control parameters.
type DKGParameters struct {
	SubmissionTimeoutBlocks       uint64
	ChallengePeriodBlocks         uint64
	ApprovePrecedencePeriodBlocks uint64
}

// WalletClosedEvent represents a wallet closed event. It is emitted when the
// wallet is closed in the wallet registry.
type WalletClosedEvent struct {
	WalletID    [32]byte
	BlockNumber uint64
}

// BridgeChain defines the subset of the TBTC chain interface that pertains
// specifically to the tBTC Bridge operations.
type BridgeChain interface {
	// CalculateWalletID calculates the wallet's ECDSA ID based on the provided
	// wallet public key.
	CalculateWalletID(walletPublicKey *ecdsa.PublicKey) ([32]byte, error)

	// IsWalletRegistered checks whether the given wallet is registered in the
	// ECDSA wallet registry.
	IsWalletRegistered(EcdsaWalletID [32]byte) (bool, error)

	// GetWallet gets the on-chain data for the given wallet. Returns an error
	// if the wallet was not found.
	GetWallet(walletPublicKeyHash [20]byte) (*WalletChainData, error)

	// OnWalletClosed registers a callback that is invoked when an on-chain
	// notification of the wallet closed is seen. The notification occurs when
	// the wallet is closed or terminated.
	OnWalletClosed(
		func(event *WalletClosedEvent),
	) subscription.EventSubscription

	// ComputeMainUtxoHash computes the hash of the provided main UTXO
	// according to the on-chain Bridge rules.
	ComputeMainUtxoHash(mainUtxo *bitcoin.UnspentTransactionOutput) [32]byte

	// PastDepositRevealedEvents fetches past deposit reveal events according
	// to the provided filter or unfiltered if the filter is nil. Returned
	// events are sorted by the block number in the ascending order, i.e. the
	// latest event is at the end of the slice.
	PastDepositRevealedEvents(
		filter *DepositRevealedEventFilter,
	) ([]*DepositRevealedEvent, error)

	// GetPendingRedemptionRequest gets the on-chain pending redemption request
	// for the given wallet public key hash and redeemer output script.
	// The returned bool value indicates whether the request was found or not.
	GetPendingRedemptionRequest(
		walletPublicKeyHash [20]byte,
		redeemerOutputScript bitcoin.Script,
	) (*RedemptionRequest, bool, error)

	// GetDepositRequest gets the on-chain deposit request for the given
	// funding transaction hash and output index. The returned bool value
	// indicates whether the request was found or not.
	GetDepositRequest(
		fundingTxHash bitcoin.Hash,
		fundingOutputIndex uint32,
	) (*DepositChainRequest, bool, error)

	// GetMovedFundsSweepRequest gets the on-chain moved funds sweep request for
	// the given moving funds transaction hash and output index.
	// The returned bool value indicates whether the request was found or not.
	GetMovedFundsSweepRequest(
		movingFundsTxHash bitcoin.Hash,
		movingFundsTxOutpointIndex uint32,
	) (*MovedFundsSweepRequest, bool, error)

	// GetMovingFundsParameters gets the current value of parameters relevant
	// for the moving funds process.
	GetMovingFundsParameters() (
		txMaxTotalFee uint64,
		dustThreshold uint64,
		timeoutResetDelay uint32,
		timeout uint32,
		timeoutSlashingAmount *big.Int,
		timeoutNotifierRewardMultiplier uint32,
		commitmentGasOffset uint16,
		sweepTxMaxTotalFee uint64,
		sweepTimeout uint32,
		sweepTimeoutSlashingAmount *big.Int,
		sweepTimeoutNotifierRewardMultiplier uint32,
		err error,
	)

	// PastMovingFundsCommitmentSubmittedEvents fetches past moving funds
	// commitment submitted events according to the provided filter or
	// unfiltered if the filter is nil. Returned events are sorted by the block
	// number in the ascending order, i.e. the latest event is at the end of the
	// slice.
	PastMovingFundsCommitmentSubmittedEvents(
		filter *MovingFundsCommitmentSubmittedEventFilter,
	) ([]*MovingFundsCommitmentSubmittedEvent, error)
}

// NewWalletRegisteredEvent represents a new wallet registered event.
type NewWalletRegisteredEvent struct {
	EcdsaWalletID       [32]byte
	WalletPublicKeyHash [20]byte
	BlockNumber         uint64
}

// NewWalletRegisteredEventFilter is a component allowing to filter NewWalletRegisteredEvent.
type NewWalletRegisteredEventFilter struct {
	StartBlock          uint64
	EndBlock            *uint64
	EcdsaWalletID       [][32]byte
	WalletPublicKeyHash [][20]byte
}

// HeartbeatRequestedEvent represents a Bridge heartbeat request event.
type HeartbeatRequestedEvent struct {
	WalletPublicKey []byte
	Messages        []*big.Int
	BlockNumber     uint64
}

// DepositRevealedEvent represents a deposit reveal event.
//
// The Vault field is nil if the deposit does not target any vault on-chain.
type DepositRevealedEvent struct {
	FundingTxHash       bitcoin.Hash
	FundingOutputIndex  uint32
	Depositor           chain.Address
	Amount              uint64
	BlindingFactor      [8]byte
	WalletPublicKeyHash [20]byte
	RefundPublicKeyHash [20]byte
	RefundLocktime      [4]byte
	Vault               *chain.Address
	BlockNumber         uint64
}

func (dre *DepositRevealedEvent) unpack(extraData *[32]byte) *Deposit {
	return &Deposit{
		Utxo: &bitcoin.UnspentTransactionOutput{
			Outpoint: &bitcoin.TransactionOutpoint{
				TransactionHash: dre.FundingTxHash,
				OutputIndex:     dre.FundingOutputIndex,
			},
			Value: int64(dre.Amount),
		},
		Depositor:           dre.Depositor,
		BlindingFactor:      dre.BlindingFactor,
		WalletPublicKeyHash: dre.WalletPublicKeyHash,
		RefundPublicKeyHash: dre.RefundPublicKeyHash,
		RefundLocktime:      dre.RefundLocktime,
		Vault:               dre.Vault,
		ExtraData:           extraData,
	}
}

func (dre *DepositRevealedEvent) GetWalletPublicKeyHash() [20]byte {
	return dre.WalletPublicKeyHash
}

// DepositRevealedEventFilter is a component allowing to filter DepositRevealedEvent.
type DepositRevealedEventFilter struct {
	StartBlock          uint64
	EndBlock            *uint64
	Depositor           []chain.Address
	WalletPublicKeyHash [][20]byte
}

// DepositChainRequest represents a deposit request stored on-chain.
// This is a deposit revealed to the Bridge and recorded on-chain. There is no
// guarantee this deposit actually happened on the Bitcoin side.
//
// The Vault field is nil if the deposit does not target any vault on-chain.
type DepositChainRequest struct {
	Depositor   chain.Address
	Amount      uint64
	RevealedAt  time.Time
	Vault       *chain.Address
	TreasuryFee uint64
	SweptAt     time.Time
	ExtraData   *[32]byte
}

// WalletChainData represents wallet data stored on-chain.
//
// EcdsaWalletID and MembersIDsHash are sourced from the wallet registry.
// When the registry is unavailable during a fault-isolated GetWallet call,
// these fields contain their zero values. Consumers that require registry
// data (e.g. signer approval certificate computation) must guard against
// zero values -- see ErrMissingWalletID and ErrMissingMembersIDsHash.
type WalletChainData struct {
	EcdsaWalletID                          [32]byte
	MembersIDsHash                         [32]byte
	MainUtxoHash                           [32]byte
	PendingRedemptionsValue                uint64
	CreatedAt                              time.Time
	MovingFundsRequestedAt                 time.Time
	ClosingStartedAt                       time.Time
	PendingMovedFundsSweepRequestsCount    uint32
	State                                  WalletState
	MovingFundsTargetWalletsCommitmentHash [32]byte
}

// WalletProposalValidatorChain defines the subset of the TBTC chain interface
// that pertains specifically to the tBTC wallet proposal validator.
type WalletProposalValidatorChain interface {
	// ValidateDepositSweepProposal validates the given deposit sweep proposal
	// against the chain. It requires some additional data about the deposits
	// that must be fetched externally. Returns an error if the proposal is
	// not valid or nil otherwise.
	ValidateDepositSweepProposal(
		walletPublicKeyHash [20]byte,
		proposal *DepositSweepProposal,
		depositsExtraInfo []struct {
			*Deposit
			FundingTx *bitcoin.Transaction
		},
	) error

	// ValidateRedemptionProposal validates the given redemption proposal
	// against the chain. Returns an error if the proposal is not valid or
	// nil otherwise.
	ValidateRedemptionProposal(
		walletPublicKeyHash [20]byte,
		proposal *RedemptionProposal,
	) error

	// ValidateHeartbeatProposal validates the given heartbeat proposal
	// against the chain. Returns an error if the proposal is not valid or
	// nil otherwise.
	ValidateHeartbeatProposal(
		walletPublicKeyHash [20]byte,
		proposal *HeartbeatProposal,
	) error

	// ValidateMovingFundsProposal validates the given moving funds proposal
	// against the chain. Returns an error if the proposal is not valid or
	// nil otherwise.
	ValidateMovingFundsProposal(
		walletPublicKeyHash [20]byte,
		mainUTXO *bitcoin.UnspentTransactionOutput,
		proposal *MovingFundsProposal,
	) error

	// ValidateMovedFundsSweepProposal validates the given moved funds sweep
	// proposal against the chain. Returns an error if the proposal is not valid
	// or nil otherwise.
	ValidateMovedFundsSweepProposal(
		walletPublicKeyHash [20]byte,
		proposal *MovedFundsSweepProposal,
	) error
}

// RedemptionRequestedEvent represents a redemption requested event.
type RedemptionRequestedEvent struct {
	WalletPublicKeyHash  [20]byte
	RedeemerOutputScript bitcoin.Script
	Redeemer             chain.Address
	RequestedAmount      uint64
	TreasuryFee          uint64
	TxMaxFee             uint64
	BlockNumber          uint64
}

func (rre *RedemptionRequestedEvent) GetWalletPublicKeyHash() [20]byte {
	return rre.WalletPublicKeyHash
}

// RedemptionRequestedEventFilter is a component allowing to filter RedemptionRequestedEvent.
type RedemptionRequestedEventFilter struct {
	StartBlock          uint64
	EndBlock            *uint64
	WalletPublicKeyHash [][20]byte
	Redeemer            []chain.Address
}

// MovingFundsCommitmentSubmittedEvent represents a moving funds commitment submitted event.
type MovingFundsCommitmentSubmittedEvent struct {
	WalletPublicKeyHash [20]byte
	TargetWallets       [][20]byte
	Submitter           chain.Address
	BlockNumber         uint64
}

func (mfcse *MovingFundsCommitmentSubmittedEvent) GetWalletPublicKeyHash() [20]byte {
	return mfcse.WalletPublicKeyHash
}

func (mfcse *MovingFundsCommitmentSubmittedEvent) GetTargetWallets() [][20]byte {
	return mfcse.TargetWallets
}

// MovingFundsCommitmentSubmittedEventFilter is a component allowing to filter MovingFundsCommitmentSubmittedEvent.
type MovingFundsCommitmentSubmittedEventFilter struct {
	StartBlock          uint64
	EndBlock            *uint64
	WalletPublicKeyHash [][20]byte
}

// MovingFundsCompletedEvent represents a moving funds completed event.
type MovingFundsCompletedEvent struct {
	WalletPublicKeyHash [20]byte
	MovingFundsTxHash   bitcoin.Hash
	BlockNumber         uint64
}

// MovingFundsCompletedEventFilter is a component allowing to filter MovingFundsCompletedEvent.
type MovingFundsCompletedEventFilter struct {
	StartBlock          uint64
	EndBlock            *uint64
	WalletPublicKeyHash [][20]byte
}

// Chain represents the interface that the TBTC module expects to interact
// with the anchoring blockchain on.
type Chain interface {
	// BlockCounter returns the chain's block counter.
	BlockCounter() (chain.BlockCounter, error)
	// Signing returns the chain's signer.
	Signing() chain.Signing
	// OperatorKeyPair returns the key pair of the operator assigned to this
	// chain handle.
	OperatorKeyPair() (*operator.PrivateKey, *operator.PublicKey, error)
	// GetBlockNumberByTimestamp gets the block number for the given timestamp.
	// In the best case, the block with the exact same timestamp is returned.
	// If the aforementioned is not possible, it tries to return the closest
	// possible block.
	GetBlockNumberByTimestamp(timestamp uint64) (uint64, error)
	// GetBlockHashByNumber gets the block hash for the given block number.
	GetBlockHashByNumber(blockNumber uint64) ([32]byte, error)

	sortition.Chain
	GroupSelectionChain
	DistributedKeyGenerationChain
	InactivityClaimChain
	BridgeChain
	WalletProposalValidatorChain
}
