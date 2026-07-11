package tbtc

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/sortition"
)

const (
	// heartbeatTotalProposalValidityBlocks determines the total wallet
	// heartbeat proposal validity time expressed in blocks. In other words,
	// this is the worst-case time for a wallet heartbeat during which the
	// wallet is busy and cannot take another actions. It includes the total
	// duration needed to perform both signing the heartbeat message and
	// optionally notifying about operator inactivity if the heartbeat failed.
	// The value of 600 blocks is roughly 2 hours, assuming 12 seconds per block.
	heartbeatTotalProposalValidityBlocks = 600
	// heartbeatInactivityClaimValidityBlocks determines the duration that needs
	// to be preserved for the optional notification about operator inactivity
	// that follows a failed heartbeat signing.
	heartbeatInactivityClaimValidityBlocks = 300
	// heartbeatTimeoutSafetyMarginBlocks determines the duration of the safety
	// margin that must be preserved between the timeout of operator inactivity
	// notification and the timeout of the entire heartbeat action. This safety
	// margin prevents against the case where signing completes too late and
	// another action has been already requested by the coordinator. The value
	// of 25 blocks is roughly 5 minutes, assuming 12 seconds per block.
	heartbeatTimeoutSafetyMarginBlocks = 25
	// heartbeatSigningMinimumActiveOperators determines the minimum number of
	// active members during signing for a heartbeat to be considered valid.
	heartbeatSigningMinimumActiveMembers = 70
	// heartbeatConsecutiveFailuresThreshold determines the number of consecutive
	// heartbeat failures required to trigger inactivity operator notification.
	heartbeatConsecutiveFailureThreshold = 3
)

type HeartbeatProposal struct {
	Message [16]byte
}

func (hp *HeartbeatProposal) ActionType() WalletActionType {
	return ActionHeartbeat
}

func (hp *HeartbeatProposal) ValidityBlocks() uint64 {
	return heartbeatTotalProposalValidityBlocks
}

// heartbeatSigningExecutor is an interface meant to decouple the specific
// implementation of the signing executor from the heartbeat action.
type heartbeatSigningExecutor interface {
	sign(
		ctx context.Context,
		message *big.Int,
		startBlock uint64,
	) (*frost.Signature, *signingActivityReport, uint64, error)
}

// heartbeatInactivityClaimExecutor is an interface meant to decouple the
// specific implementation of the inactivity claim executor from the heartbeat
// action.
type heartbeatInactivityClaimExecutor interface {
	claimInactivity(
		ctx context.Context,
		inactiveMembersIndexes []group.MemberIndex,
		heartbeatFailed bool,
		sessionID *big.Int,
	) error
}

// walletSortitionChainProvider supplies sortition views pinned to one wallet
// scheme. It prevents a process-wide FROST configuration switch from changing
// the authorization checks performed for legacy ECDSA wallets during the
// migration drain.
type walletSortitionChainProvider interface {
	LegacyECDSASortitionChain() sortition.Chain
	FrostSortitionChain() (sortition.Chain, bool)
}

// heartbeatAction is a walletAction implementation handling heartbeat requests
// from the wallet coordinator.
type heartbeatAction struct {
	logger log.StandardLogger
	chain  Chain

	executingWallet      wallet
	signingExecutor      heartbeatSigningExecutor
	minimumActiveMembers int

	proposal       *HeartbeatProposal
	failureCounter *heartbeatFailureCounter

	inactivityClaimExecutor heartbeatInactivityClaimExecutor

	startBlock  uint64
	expiryBlock uint64

	waitForBlockFn waitForBlockFn
}

func newHeartbeatAction(
	logger log.StandardLogger,
	chain Chain,
	executingWallet wallet,
	signingExecutor heartbeatSigningExecutor,
	minimumActiveMembers int,
	proposal *HeartbeatProposal,
	failureCounter *heartbeatFailureCounter,
	inactivityClaimExecutor heartbeatInactivityClaimExecutor,
	startBlock uint64,
	expiryBlock uint64,
	waitForBlockFn waitForBlockFn,
) *heartbeatAction {
	return &heartbeatAction{
		logger:                  logger,
		chain:                   chain,
		executingWallet:         executingWallet,
		signingExecutor:         signingExecutor,
		minimumActiveMembers:    minimumActiveMembers,
		proposal:                proposal,
		failureCounter:          failureCounter,
		inactivityClaimExecutor: inactivityClaimExecutor,
		startBlock:              startBlock,
		expiryBlock:             expiryBlock,
		waitForBlockFn:          waitForBlockFn,
	}
}

func (ha *heartbeatAction) execute() error {
	// Do not execute the heartbeat action if the operator is unstaking.
	isUnstaking, err := ha.isOperatorUnstaking()
	if err != nil {
		return fmt.Errorf(
			"failed to check if the operator is unstaking [%v]",
			err,
		)
	}

	if isUnstaking {
		ha.logger.Warn(
			"quitting the heartbeat action without signing because the " +
				"operator is unstaking",
		)
		return nil
	}

	walletPublicKey := ha.wallet().publicKey
	walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		return fmt.Errorf("failed to unmarshal wallet public key: [%v]", err)
	}

	walletKey := hex.EncodeToString(walletPublicKeyBytes)

	err = ha.chain.ValidateHeartbeatProposal(walletPublicKeyHash, ha.proposal)
	if err != nil {
		return fmt.Errorf("heartbeat proposal is invalid: [%v]", err)
	}

	messageBytes := bitcoin.ComputeHash(ha.proposal.Message[:])
	messageToSign := new(big.Int).SetBytes(messageBytes[:])

	// Just in case. This should never happen.
	if ha.expiryBlock < heartbeatInactivityClaimValidityBlocks {
		return fmt.Errorf("invalid proposal expiry block")
	}

	heartbeatSigningCtx, cancelHeartbeatSigningCtx := withCancelOnBlock(
		context.Background(),
		ha.expiryBlock-heartbeatInactivityClaimValidityBlocks,
		ha.waitForBlockFn,
	)
	defer cancelHeartbeatSigningCtx()

	signature, activityReport, _, err := ha.signingExecutor.sign(
		heartbeatSigningCtx,
		messageToSign,
		ha.startBlock,
	)
	if err != nil {
		// Do not count this error as heartbeat inactivity failure. If the
		// process returned an error here, that likely means the group signing
		// threshold was not met. In such a case, the inactivity claim does not
		// have a chance for success anyway (it needs the group threshold to
		// be met as well).
		return fmt.Errorf("heartbeat signing process errored out: [%v]", err)
	}

	// If the number of active members during signing was enough, we can
	// consider the heartbeat procedure as successful.
	activeMembersCount := len(activityReport.activeMembers)
	if activeMembersCount >= ha.minimumActiveMembers {
		ha.logger.Infof(
			"heartbeat generated signature [%s] for message [0x%x]",
			signature,
			ha.proposal.Message[:],
		)

		// Reset the counter of consecutive heartbeat inactivity failures.
		ha.failureCounter.reset(walletKey)

		return nil
	}

	// If the number of active members during signing was not enough, we
	// must consider the heartbeat procedure as an inactivity failure.
	ha.logger.Warnf(
		"heartbeat generated signature but minimum activity "+
			"threshold was not met ([%d/%d] members participated); "+
			"counting it as inactivity failure",
		activeMembersCount,
		ha.minimumActiveMembers,
	)

	// Increment the heartbeat inactivity failure counter.
	ha.failureCounter.increment(walletKey)

	// If the number of consecutive heartbeat inactivity failures does not
	// exceed the threshold, do not issue an inactivity claim yet.
	if ha.failureCounter.get(walletKey) < heartbeatConsecutiveFailureThreshold {
		ha.logger.Warnf(
			"not issuing an inactivity claim yet; current consecutive"+
				"heartbeat inactivity failure count is [%d/%d]",
			ha.failureCounter.get(walletKey),
			heartbeatConsecutiveFailureThreshold,
		)
		return nil
	}

	// This should not happen but check it just in case as inactivity claim
	// requires a non-empty inactive members set.
	if len(activityReport.inactiveMembers) == 0 {
		return fmt.Errorf(
			"inactivity claim aborted due to an undetermined set of inactive members",
		)
	}

	heartbeatInactivityCtx, cancelHeartbeatInactivityCtx := withCancelOnBlock(
		context.Background(),
		ha.expiryBlock-heartbeatTimeoutSafetyMarginBlocks,
		ha.waitForBlockFn,
	)
	defer cancelHeartbeatInactivityCtx()

	// The value of consecutive heartbeat inactivity failures exceeds the threshold.
	// Proceed with operator inactivity claim.
	err = ha.inactivityClaimExecutor.claimInactivity(
		heartbeatInactivityCtx,
		// It's safe to consider unstaking members as inactive members in the claim.
		// Inactive members are set ineligible for on-chain rewards for a certain
		// period of time. This is a desired outcome for unstaking members as well.
		activityReport.inactiveMembers,
		true,
		messageToSign,
	)
	if err != nil {
		return fmt.Errorf(
			"error while notifying about operator inactivity [%v]]",
			err,
		)
	}

	return nil
}

func (ha *heartbeatAction) wallet() wallet {
	return ha.executingWallet
}

func (ha *heartbeatAction) actionType() WalletActionType {
	return ActionHeartbeat
}

func (ha *heartbeatAction) isOperatorUnstaking() (bool, error) {
	stakingChain, err := ha.sortitionChain()
	if err != nil {
		return false, err
	}

	stakingProvider, isRegistered, err := stakingChain.OperatorToStakingProvider()
	if err != nil {
		return false, fmt.Errorf(
			"failed to get staking provider for operator [%v]",
			err,
		)
	}

	if !isRegistered {
		return false, fmt.Errorf("staking provider not registered for operator")
	}

	// Eligible stake is defined as the currently authorized stake minus the
	// pending authorization decrease.
	eligibleStake, err := stakingChain.EligibleStake(stakingProvider)
	if err != nil {
		return false, fmt.Errorf(
			"failed to check eligible stake for operator [%v]",
			err,
		)
	}

	// The operator is considered unstaking if their eligible stake is `0`.
	return eligibleStake.Cmp(big.NewInt(0)) == 0, nil
}

func (ha *heartbeatAction) sortitionChain() (sortition.Chain, error) {
	provider, ok := ha.chain.(walletSortitionChainProvider)
	if !ok {
		// Backward compatibility for non-Ethereum chains and test chains that
		// expose only one registry.
		return ha.chain, nil
	}

	walletPublicKeyHash := bitcoin.PublicKeyHash(ha.executingWallet.publicKey)
	walletData, err := ha.chain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get wallet data for authorization check [%v]",
			err,
		)
	}

	walletScheme, _, err := walletSchemeAndRegistryID(walletData)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to resolve wallet scheme for authorization check [%v]",
			err,
		)
	}

	switch walletScheme {
	case WalletSchemeECDSA:
		return provider.LegacyECDSASortitionChain(), nil
	case WalletSchemeFROST:
		frostChain, configured := provider.FrostSortitionChain()
		if !configured || frostChain == nil {
			return nil, fmt.Errorf("FROST sortition chain is not configured")
		}

		return frostChain, nil
	default:
		return nil, fmt.Errorf("unsupported wallet scheme [%v]", walletScheme)
	}
}

// heartbeatFailureCounter holds counters keeping track of consecutive
// heartbeat failures. Each wallet has a separate counter. The key used in
// the map is the uncompressed public key (with 04 prefix) of the wallet.
type heartbeatFailureCounter struct {
	mutex    sync.Mutex
	counters map[string]uint
}

func newHeartbeatFailureCounter() *heartbeatFailureCounter {
	return &heartbeatFailureCounter{
		counters: make(map[string]uint),
	}
}

func (hfc *heartbeatFailureCounter) increment(walletPublicKey string) {
	hfc.mutex.Lock()
	defer hfc.mutex.Unlock()

	hfc.counters[walletPublicKey]++

}

func (hfc *heartbeatFailureCounter) reset(walletPublicKey string) {
	hfc.mutex.Lock()
	defer hfc.mutex.Unlock()

	hfc.counters[walletPublicKey] = 0
}

func (hfc *heartbeatFailureCounter) get(walletPublicKey string) uint {
	hfc.mutex.Lock()
	defer hfc.mutex.Unlock()

	return hfc.counters[walletPublicKey]
}
