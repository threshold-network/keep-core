package tbtc

import (
	"crypto/ecdsa"
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
// selected to perform the group key generation protocol. The result consists of
// two slices of equal length holding the chain.OperatorID and chain.Address for
// each selected operator.
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

	// BuildDepositKey calculates a deposit key for the given funding
	// transaction hash and output index. Mirrors tbtcpg.Chain's identical
	// method - the reservation anchor wallet action needs it to derive the
	// m1 reservation key (reservationKey == depositKey) without depending
	// on the tbtcpg package.
	BuildDepositKey(fundingTxHash bitcoin.Hash, fundingOutputIndex uint32) *big.Int

	// GetMovedFundsSweepRequest gets the on-chain moved funds sweep request for
	// the given moving funds transaction hash and output index.
	// The returned bool value indicates whether the request was found or not.
	GetMovedFundsSweepRequest(
		movingFundsTxHash bitcoin.Hash,
		movingFundsTxOutpointIndex uint32,
	) (*MovedFundsSweepRequest, bool, error)

	// GetMovingFundsParameters gets the current value of parameters relevant
	// for the moving funds process.
	GetMovingFundsParameters() (MovingFundsParameters, error)

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
type WalletChainData struct {
	EcdsaWalletID                          [32]byte
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

	// ValidateReservationAnchorProposal validates the given reservation
	// anchor proposal against the chain. Returns an error if the proposal
	// is not valid or nil otherwise.
	ValidateReservationAnchorProposal(
		walletPublicKeyHash [20]byte,
		proposal *ReservationAnchorProposal,
		depositExtraInfo struct {
			*Deposit
			FundingTx *bitcoin.Transaction
		},
	) error

	// ValidateReservedRedemptionProposal validates the given reserved
	// redemption proposal against the chain. Returns an error if the
	// proposal is not valid or nil otherwise.
	ValidateReservedRedemptionProposal(
		walletPublicKeyHash [20]byte,
		proposal *ReservedRedemptionProposal,
	) error

	// ValidateReservationReanchorProposal validates the given reservation
	// re-anchor proposal against the chain. Returns an error if the
	// proposal is not valid or nil otherwise.
	ValidateReservationReanchorProposal(
		sourceWalletPublicKeyHash [20]byte,
		proposal *ReservationReanchorProposal,
	) error

	// ValidateReservationDissolutionProposal validates the given reservation
	// dissolution proposal against the chain. Returns an error if the
	// proposal is not valid or nil otherwise.
	ValidateReservationDissolutionProposal(
		walletPublicKeyHash [20]byte,
		proposal *ReservationDissolutionProposal,
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
	ReservationChain
}

// ReservationChain defines the subset of the TBTC chain interface that pertains
// specifically to UTXO reservation Bridge operations. The reservation state
// machine is implemented behind Bridge.fallback's delegatecall to the
// ReservationRouter contract; the binding is constructed against the Bridge
// address, so every read, write, and log subscription on this interface
// routes through the Bridge's storage rather than the router's empty
// standalone storage.
type ReservationChain interface {
	// RequestReservationAcceptance requests a reservation acceptance action
	// generation for the given reservation. The reservation must be in a
	// state that allows acceptance; the operator-side guard is enforced at
	// the chain layer.
	RequestReservationAcceptance(
		reservationKey *big.Int,
		walletPublicKeyHash [20]byte,
	) error

	// RequestReservationReanchor requests a reservation re-anchor action
	// generation for the given reservation, targeting the given wallet.
	RequestReservationReanchor(
		reservationKey *big.Int,
		targetWalletPublicKeyHash [20]byte,
	) error

	// SubmitReservationProof submits an SPV proof for the given reservation
	// action generation. proofType selects between Acceptance, Redemption,
	// Reanchor, and Dissolution proofs; m1 invokes only Acceptance (1) and
	// Reanchor (3). The call is restricted to the SPV maintainer registered
	// against the Bridge.
	SubmitReservationProof(
		proofType uint8,
		txInfo *BitcoinTxInfo,
		proof *BitcoinTxProof,
		mainUtxo *BitcoinTxUTXO,
		reservationKey *big.Int,
		requestNonce uint64,
	) error

	// NotifyReservationActionTimeout notifies the Bridge that the timeout
	// for the given reservation action generation has elapsed without the
	// SPV proof being submitted. The walletMembersIDs carry the operator
	// IDs of the wallet that was authorized for the action; they are used
	// to slash the wallet in m2-era records (no-op in m1).
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
	// the anchor is therefore stranded. This is the m1 path that closes
	// reservations whose wallet is no longer live.
	NotifyReservationStranded(reservationKey *big.Int) error

	// GetReservation gets the on-chain reservation record for the given
	// reservation key. Returns an error if the reservation was not found.
	GetReservation(reservationKey *big.Int) (*Reservation, error)

	// GetReservationAction gets the on-chain action record for the given
	// reservation key and request nonce. Returns an error if the action
	// generation was not found.
	GetReservationAction(
		reservationKey *big.Int,
		requestNonce uint64,
	) (*ReservationAction, error)

	// ReservationParameters gets the current on-chain values of the Bridge
	// reservation parameters.
	ReservationParameters() (*ReservationParameters, error)

	// ReservationCaps returns the cap parameters that gate reservation
	// acceptance: the maximum aggregate satoshi amount a single wallet may
	// custody across all of its reservations, and the maximum satoshi
	// amount any single reservation may anchor.
	ReservationCaps() (maxReservationsAmountPerWallet uint64, reservationMaxSingleAmount uint64, err error)

	// WalletReservationsAmount returns the aggregate satoshi amount
	// currently anchored by the given wallet across all of its
	// reservations.
	WalletReservationsAmount(walletPublicKeyHash [20]byte) (uint64, error)

	// WalletReservationsCount returns the number of reservations currently
	// custodied by the given wallet.
	WalletReservationsCount(walletPublicKeyHash [20]byte) (uint32, error)

	// WalletReservations returns the reservation keys for all reservations
	// currently custodied by the given wallet.
	WalletReservations(walletPublicKeyHash [20]byte) ([]*big.Int, error)

	// ReservationByAnchorUtxo returns the reservation key whose anchor
	// outpoint is the given Bitcoin transaction output, or an empty value
	// if no reservation is anchored there.
	ReservationByAnchorUtxo(
		anchorTxHash [32]byte,
		anchorTxOutputIndex uint32,
	) (*big.Int, error)

	// ReservedDepositWallet returns the wallet public key hash to which
	// the given reserved deposit was revealed. Returns the zero hash if the
	// deposit is not a reserved deposit.
	ReservedDepositWallet(depositKey *big.Int) ([20]byte, error)

	// PendingReservedDeposits returns the number of reserved deposits that
	// have been revealed to the Bridge but not yet accepted by a wallet.
	// The value is consumed by the vault-repoint and pre-acceptance paths
	// to gate new deposits.
	PendingReservedDeposits() (uint64, error)

	// ActiveReservationsCount returns the current count of active
	// reservations across all wallets and the cap on that count.
	ActiveReservationsCount() (count uint32, maxActive uint32, err error)

	// ReservationRouter returns the address of the ReservationRouter
	// contract as stored on the Bridge. This is the one place where the
	// chain handle reads a router address value rather than binding a call
	// to it: the router holds its own empty storage and only ever executes
	// via Bridge.fallback's delegatecall, so any actual reservation call
	// goes through the Bridge binding.
	ReservationRouter() (chain.Address, error)

	// IsReservedDeposit returns true if the given deposit was revealed
	// with the reservation vault address and is therefore a reservation
	// rather than a default deposit.
	IsReservedDeposit(depositKey *big.Int) (bool, error)

	// OnReservationAcceptanceRequested registers a callback that is invoked
	// when an on-chain ReservationAcceptanceRequested event is seen.
	OnReservationAcceptanceRequested(
		handler func(event *ReservationAcceptanceRequestedEvent),
	) subscription.EventSubscription

	// PastReservationAcceptanceRequestedEvents fetches past
	// ReservationAcceptanceRequested events according to the provided
	// filter or unfiltered if the filter is nil. Returned events are sorted
	// by the block number in the ascending order, i.e. the latest event is
	// at the end of the slice.
	PastReservationAcceptanceRequestedEvents(
		filter *ReservationAcceptanceRequestedEventFilter,
	) ([]*ReservationAcceptanceRequestedEvent, error)

	// OnReservationAccepted registers a callback that is invoked when an
	// on-chain ReservationAccepted event is seen.
	OnReservationAccepted(
		handler func(event *ReservationAcceptedEvent),
	) subscription.EventSubscription

	// PastReservationAcceptedEvents fetches past ReservationAccepted events
	// according to the provided filter or unfiltered if the filter is nil.
	PastReservationAcceptedEvents(
		filter *ReservationAcceptedEventFilter,
	) ([]*ReservationAcceptedEvent, error)

	// OnReservationReanchorRequested registers a callback that is invoked
	// when an on-chain ReservationReanchorRequested event is seen.
	OnReservationReanchorRequested(
		handler func(event *ReservationReanchorRequestedEvent),
	) subscription.EventSubscription

	// PastReservationReanchorRequestedEvents fetches past
	// ReservationReanchorRequested events according to the provided filter
	// or unfiltered if the filter is nil.
	PastReservationReanchorRequestedEvents(
		filter *ReservationReanchorRequestedEventFilter,
	) ([]*ReservationReanchorRequestedEvent, error)

	// OnReservationReanchored registers a callback that is invoked when an
	// on-chain ReservationReanchored event is seen.
	OnReservationReanchored(
		handler func(event *ReservationReanchoredEvent),
	) subscription.EventSubscription

	// PastReservationReanchoredEvents fetches past ReservationReanchored
	// events according to the provided filter or unfiltered if the filter
	// is nil.
	PastReservationReanchoredEvents(
		filter *ReservationReanchoredEventFilter,
	) ([]*ReservationReanchoredEvent, error)

	// OnReservationActionTimedOut registers a callback that is invoked
	// when an on-chain ReservationActionTimedOut event is seen. The
	// timeout watcher fires the notification that triggers this event.
	OnReservationActionTimedOut(
		handler func(event *ReservationActionTimedOutEvent),
	) subscription.EventSubscription

	// PastReservationActionTimedOutEvents fetches past
	// ReservationActionTimedOut events according to the provided filter or
	// unfiltered if the filter is nil.
	PastReservationActionTimedOutEvents(
		filter *ReservationActionTimedOutEventFilter,
	) ([]*ReservationActionTimedOutEvent, error)

	// OnReservationActionSuperseded registers a callback that is invoked
	// when an on-chain ReservationActionSuperseded event is seen.
	OnReservationActionSuperseded(
		handler func(event *ReservationActionSupersededEvent),
	) subscription.EventSubscription

	// OnReservationLateSettled registers a callback that is invoked when
	// an on-chain ReservationLateSettled event is seen.
	OnReservationLateSettled(
		handler func(event *ReservationLateSettledEvent),
	) subscription.EventSubscription

	// OnReservationRetryCreditMinted registers a callback that is invoked
	// when an on-chain ReservationRetryCreditMinted event is seen. m1
	// records no such events because the on-chain mint path is unreachable
	// on m1-era records; the subscription is still wired for forward
	// compatibility with m2.
	OnReservationRetryCreditMinted(
		handler func(event *ReservationRetryCreditMintedEvent),
	) subscription.EventSubscription

	// OnReservedDepositMarkedStale registers a callback that is invoked
	// when an on-chain ReservedDepositMarkedStale event is seen.
	OnReservedDepositMarkedStale(
		handler func(event *ReservedDepositMarkedStaleEvent),
	) subscription.EventSubscription

	// OnReservationStranded registers a callback that is invoked when an
	// on-chain ReservationStranded event is seen. Stranding is the m1
	// close path for reservations whose custodying wallet has been closed
	// or terminated.
	OnReservationStranded(
		handler func(event *ReservationStrandedEvent),
	) subscription.EventSubscription

	// OnReservationParametersUpdated registers a callback that is invoked
	// when an on-chain ReservationParametersUpdated event is seen.
	OnReservationParametersUpdated(
		handler func(event *ReservationParametersUpdatedEvent),
	) subscription.EventSubscription

	// OnReservationVaultUpdated registers a callback that is invoked when
	// an on-chain ReservationVaultUpdated event is seen.
	OnReservationVaultUpdated(
		handler func(event *ReservationVaultUpdatedEvent),
	) subscription.EventSubscription

	// OnReservationCapsUpdated registers a callback that is invoked when
	// an on-chain ReservationCapsUpdated event is seen.
	OnReservationCapsUpdated(
		handler func(event *ReservationCapsUpdatedEvent),
	) subscription.EventSubscription
}

// BitcoinTxInfo represents the on-chain BitcoinTx.Info struct used by
// reservation proof submissions.
type BitcoinTxInfo struct {
	Version      [4]byte
	InputVector  []byte
	OutputVector []byte
	Locktime     [4]byte
}

// BitcoinTxProof represents the on-chain BitcoinTx.Proof struct used by
// reservation proof submissions.
type BitcoinTxProof struct {
	MerkleProof      []byte
	TxIndexInBlock   *big.Int
	BitcoinHeaders   []byte
	CoinbasePreimage [32]byte
	CoinbaseProof    []byte
}

// BitcoinTxUTXO represents the on-chain BitcoinTx.UTXO struct used by
// reservation proof submissions.
type BitcoinTxUTXO struct {
	TxHash        [32]byte
	TxOutputIndex uint32
	TxOutputValue uint64
}

// ReservationAcceptanceRequestedEvent represents a reservation acceptance
// requested event.
type ReservationAcceptanceRequestedEvent struct {
	ReservationKey      *big.Int
	RequestNonce        uint64
	WalletPublicKeyHash [20]byte
	DepositAmount       uint64
	TxMaxFee            uint64
	TimeoutAt           uint32
	BlockNumber         uint64
}

// ReservationAcceptanceRequestedEventFilter is a component allowing to filter
// ReservationAcceptanceRequestedEvent.
type ReservationAcceptanceRequestedEventFilter struct {
	StartBlock          uint64
	EndBlock            *uint64
	ReservationKey      []*big.Int
	WalletPublicKeyHash [][20]byte
}

// ReservationAcceptedEvent represents a reservation accepted event.
type ReservationAcceptedEvent struct {
	ReservationKey      *big.Int
	RequestNonce        uint64
	WalletPublicKeyHash [20]byte
	Owner               chain.Address
	AnchorTxHash        [32]byte
	AnchorAmount        uint64
	ExpiresAt           uint32
	BlockNumber         uint64
}

// ReservationAcceptedEventFilter is a component allowing to filter
// ReservationAcceptedEvent.
type ReservationAcceptedEventFilter struct {
	StartBlock          uint64
	EndBlock            *uint64
	ReservationKey      []*big.Int
	WalletPublicKeyHash [][20]byte
	Owner               []chain.Address
}

// ReservationReanchorRequestedEvent represents a reservation re-anchor
// requested event.
type ReservationReanchorRequestedEvent struct {
	ReservationKey            *big.Int
	RequestNonce              uint64
	SourceWalletPublicKeyHash [20]byte
	TargetWalletPublicKeyHash [20]byte
	TxMaxFee                  uint64
	BlockNumber               uint64
}

// ReservationReanchorRequestedEventFilter is a component allowing to filter
// ReservationReanchorRequestedEvent.
type ReservationReanchorRequestedEventFilter struct {
	StartBlock                uint64
	EndBlock                  *uint64
	ReservationKey            []*big.Int
	SourceWalletPublicKeyHash [][20]byte
	TargetWalletPublicKeyHash [][20]byte
}

// ReservationReanchoredEvent represents a reservation re-anchored event.
type ReservationReanchoredEvent struct {
	ReservationKey         *big.Int
	RequestNonce           uint64
	NewWalletPublicKeyHash [20]byte
	NewAnchorTxHash        [32]byte
	NewAnchorAmount        uint64
	BlockNumber            uint64
}

// ReservationReanchoredEventFilter is a component allowing to filter
// ReservationReanchoredEvent.
type ReservationReanchoredEventFilter struct {
	StartBlock             uint64
	EndBlock               *uint64
	ReservationKey         []*big.Int
	NewWalletPublicKeyHash [][20]byte
}

// ReservationActionTimedOutEvent represents a reservation action timed out
// event.
type ReservationActionTimedOutEvent struct {
	ReservationKey *big.Int
	RequestNonce   uint64
	ActionType     ReservationActionType
	BlockNumber    uint64
}

// ReservationActionTimedOutEventFilter is a component allowing to filter
// ReservationActionTimedOutEvent.
type ReservationActionTimedOutEventFilter struct {
	StartBlock     uint64
	EndBlock       *uint64
	ReservationKey []*big.Int
}

// ReservationActionSupersededEvent represents a reservation action superseded
// event.
type ReservationActionSupersededEvent struct {
	ReservationKey *big.Int
	RequestNonce   uint64
	BlockNumber    uint64
}

// ReservationLateSettledEvent represents a reservation late-settled event.
type ReservationLateSettledEvent struct {
	ReservationKey *big.Int
	RequestNonce   uint64
	ActionType     ReservationActionType
	BlockNumber    uint64
}

// ReservationRetryCreditMintedEvent represents a reservation retry credit
// minted event.
type ReservationRetryCreditMintedEvent struct {
	ReservationKey *big.Int
	BlockNumber    uint64
}

// ReservedDepositMarkedStaleEvent represents a reserved deposit marked stale
// event.
type ReservedDepositMarkedStaleEvent struct {
	DepositKey  *big.Int
	BlockNumber uint64
}

// ReservationStrandedEvent represents a reservation stranded event.
type ReservationStrandedEvent struct {
	ReservationKey      *big.Int
	WalletPublicKeyHash [20]byte
	Owner               chain.Address
	AnchorAmount        uint64
	BlockNumber         uint64
}

// ReservationParametersUpdatedEvent represents a reservation parameters
// updated event.
type ReservationParametersUpdatedEvent struct {
	ReservationMinAmount            uint64
	ReservationTxMaxFee             uint64
	ReservationTermSeconds          uint32
	ReservationDissolutionDelay     uint32
	ReservationMaxTotalAmount       uint64
	MaxReservationsPerWallet        uint32
	ReservationActionTimeout        uint32
	ReservationRenewalWindowSeconds uint32
	BlockNumber                     uint64
}

// ReservationVaultUpdatedEvent represents a reservation vault updated event.
type ReservationVaultUpdatedEvent struct {
	ReservationVault chain.Address
	BlockNumber      uint64
}

// ReservationCapsUpdatedEvent represents a reservation caps updated event.
type ReservationCapsUpdatedEvent struct {
	MaxReservationsAmountPerWallet uint64
	ReservationMaxSingleAmount     uint64
	MaxActiveReservations          uint32
	BlockNumber                    uint64
}
