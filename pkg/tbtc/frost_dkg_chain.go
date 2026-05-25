package tbtc

import (
	"math/big"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/registry"
	"github.com/keep-network/keep-core/pkg/subscription"
)

// FrostDKGChain defines the FROST wallet-registry chain surface. It is kept
// separate from the legacy ECDSA DKG chain so the existing coordinator remains
// unchanged until FROST creation is explicitly enabled.
type FrostDKGChain interface {
	FrostWalletRegistryAvailable() bool

	OnBridgeNewWalletRequested(
		func(event *BridgeNewWalletRequestedEvent),
	) subscription.EventSubscription

	OnFrostDKGStarted(
		func(event *FrostDKGStartedEvent),
	) subscription.EventSubscription
	PastFrostDKGStartedEvents(
		filter *DKGStartedEventFilter,
	) ([]*FrostDKGStartedEvent, error)

	OnFrostDKGResultSubmitted(
		func(event *FrostDKGResultSubmittedEvent),
	) subscription.EventSubscription
	PastFrostDKGResultSubmittedEvents(
		filter *DKGStartedEventFilter,
	) ([]*FrostDKGResultSubmittedEvent, error)
	OnFrostDKGResultChallenged(
		func(event *FrostDKGResultChallengedEvent),
	) subscription.EventSubscription
	OnFrostDKGResultApproved(
		func(event *FrostDKGResultApprovedEvent),
	) subscription.EventSubscription

	SelectFrostGroup() (*GroupSelectionResult, error)
	GetFrostDKGState() (DKGState, error)
	IsFrostDKGResultValid(result *registry.Result) (bool, string, error)
	CalculateFrostDKGResultDigest(
		seed *big.Int,
		result *registry.Result,
	) ([32]byte, error)
	SubmitFrostDKGResult(result *registry.Result) error
	ChallengeFrostDKGResult(result *registry.Result) error
	ApproveFrostDKGResult(result *registry.Result) error
	FrostDKGParameters() (*DKGParameters, error)
}

// BridgeNewWalletRequestedEvent represents Bridge.NewWalletRequested.
type BridgeNewWalletRequestedEvent struct {
	BlockNumber uint64
}

// FrostDKGStartedEvent represents the FrostWalletRegistry.DkgStarted event.
type FrostDKGStartedEvent struct {
	Seed        *big.Int
	BlockNumber uint64
}

// FrostDKGResultSubmittedEvent represents a FROST DKG result submission.
type FrostDKGResultSubmittedEvent struct {
	Seed        *big.Int
	ResultHash  DKGChainResultHash
	Result      *registry.Result
	BlockNumber uint64
}

// FrostDKGResultChallengedEvent represents a successful FROST DKG challenge.
type FrostDKGResultChallengedEvent struct {
	ResultHash  DKGChainResultHash
	Challenger  chain.Address
	Reason      string
	BlockNumber uint64
}

// FrostDKGResultApprovedEvent represents a FROST DKG result approval.
type FrostDKGResultApprovedEvent struct {
	ResultHash  DKGChainResultHash
	Approver    chain.Address
	BlockNumber uint64
}
