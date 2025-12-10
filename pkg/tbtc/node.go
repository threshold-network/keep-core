package tbtc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"

	"go.uber.org/zap"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/announcer"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity"
	"github.com/keep-network/keep-core/pkg/tecdsa/signing"
)

const (
	// signingAttemptsLimit determines the maximum number of signing attempts
	// that can be performed for the given message being subject of signing.
	//
	// The value of `5` should be enough to produce the signature even with
	// `2` malicious members in a signing group of `100` members. To produce
	// the signature, `51` members must be selected out of the honest `98`.
	// The probability of successful signing in that case is:
	// `P = (98 choose 51) / (100 choose 51) = ~0.24` which means we need
	// `5` attempts on the worst case.
	//
	// A greater limit does not necessarily make sense. Presence of more than
	// `2` malicious members in the signing group has a very small probability.
	// Moreover, the signature must be produced in the reasonable time.
	// That being said, the value `5` seems to be reasonable trade-off.
	signingAttemptsLimit = 5

	// walletClosureConfirmationBlocks determines the period used when waiting
	// for the wallet closure confirmation. This period ensures the wallet has
	// been definitely closed and the closing transaction will not be removed by
	// a chain reorganization.
	walletClosureConfirmationBlocks = 32
)

// TODO: Unit tests for `node.go`.

// node represents the current state of an ECDSA node.
type node struct {
	groupParameters *GroupParameters

	chain           Chain
	btcChain        bitcoin.Chain
	netProvider     net.Provider
	walletRegistry  *walletRegistry
	metricsRecorder *clientinfo.MetricsRecorder

	// walletDispatcher ensures only one action is executed by a wallet at
	// a time. All possible activities of a created wallet must be represented
	// by appropriate actions dispatched through this component.
	walletDispatcher *walletDispatcher

	// protocolLatch makes sure no expensive number generator operations are
	// running when signing or generating a wallet key are executed. The
	// protocolLatch is used by dkgExecutor and signingExecutor.
	protocolLatch *generator.ProtocolLatch

	// dkgExecutor encapsulates the logic of distributed key generation.
	//
	// dkgExecutor MUST NOT be used outside this struct.
	dkgExecutor *dkgExecutor

	// heartbeatFailureCounter stores the counters of consecutive heartbeat
	// failures for each wallet.
	heartbeatFailureCounter *heartbeatFailureCounter

	inactivityClaimExecutorMutex sync.Mutex
	// inactivityClaimExecutors is the cache holding inactivity claim executors
	// for specific wallets. The cache key is the uncompressed public key
	// (with 04 prefix) of the wallet.
	// inactivityClaimExecutor encapsulates the logic of handling inactivity
	// claim signing and submitting.
	//
	// inactivityClaimExecutors MUST NOT be used outside this struct. Please use
	// wallet actions and walletDispatcher to execute an action on an existing
	// wallet.
	inactivityClaimExecutors map[string]*inactivityClaimExecutor

	signingExecutorsMutex sync.Mutex
	// signingExecutors is the cache holding signing executors for specific wallets.
	// The cache key is the uncompressed public key (with 04 prefix) of the wallet.
	// signingExecutor encapsulates the generic logic of signing messages.
	//
	// signingExecutors MUST NOT be used outside this struct. Please use
	// wallet actions and walletDispatcher to execute an action on an existing
	// wallet.
	signingExecutors map[string]*signingExecutor

	coordinationExecutorsMutex sync.Mutex
	// coordinationExecutors is the cache holding coordination executors for
	// specific wallets. The cache key is the uncompressed public key
	// (with 04 prefix) of the wallet. The coordinationExecutor encapsulates the
	// logic of the wallet coordination procedure.
	//
	// coordinationExecutors MUST NOT be used outside this struct.
	coordinationExecutors map[string]*coordinationExecutor

	// proposalGenerator is the implementation of the coordination proposal
	// generator used by the node.
	proposalGenerator CoordinationProposalGenerator
}

func newNode(
	groupParameters *GroupParameters,
	chain Chain,
	btcChain bitcoin.Chain,
	netProvider net.Provider,
	keyStorePersistance persistence.ProtectedHandle,
	workPersistence persistence.BasicHandle,
	scheduler *generator.Scheduler,
	proposalGenerator CoordinationProposalGenerator,
	config Config,
) (*node, error) {
	walletRegistry, err := newWalletRegistry(
		keyStorePersistance,
		chain.CalculateWalletID,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create wallet registry: [%v]", err)
	}

	latch := generator.NewProtocolLatch()
	scheduler.RegisterProtocol(latch)

	node := &node{
		groupParameters:          groupParameters,
		chain:                    chain,
		btcChain:                 btcChain,
		netProvider:              netProvider,
		walletRegistry:           walletRegistry,
		walletDispatcher:         newWalletDispatcher(),
		protocolLatch:            latch,
		heartbeatFailureCounter:  newHeartbeatFailureCounter(),
		signingExecutors:         make(map[string]*signingExecutor),
		inactivityClaimExecutors: make(map[string]*inactivityClaimExecutor),
		coordinationExecutors:    make(map[string]*coordinationExecutor),
		proposalGenerator:        proposalGenerator,
	}

	// Archive any wallets that might have been closed or terminated while the
	// client was turned off.
	err = node.archiveClosedWallets()
	if err != nil {
		return nil, fmt.Errorf("cannot archive closed wallets: [%v]", err)
	}

	// Only the operator address is known at this point and can be pre-fetched.
	// The operator ID must be determined later as the operator may not be in
	// the sortition pool yet.
	operatorAddress, err := node.operatorAddress()
	if err != nil {
		return nil, fmt.Errorf("cannot get node's operator address: [%v]", err)
	}

	// TODO: This chicken and egg problem should be solved when
	// waitForBlockHeight becomes a part of BlockHeightWaiter interface.
	node.dkgExecutor = newDkgExecutor(
		node.groupParameters,
		node.operatorID,
		operatorAddress,
		chain,
		netProvider,
		walletRegistry,
		latch,
		config,
		workPersistence,
		scheduler,
		node.waitForBlockHeight,
		nil, // metricsRecorder will be set after node creation
	)

	return node, nil
}

// operatorAddress returns the node's operator address.
func (n *node) operatorAddress() (chain.Address, error) {
	_, operatorPublicKey, err := n.chain.OperatorKeyPair()
	if err != nil {
		return "", fmt.Errorf("failed to get operator public key: [%v]", err)
	}

	operatorAddress, err := n.chain.Signing().PublicKeyToAddress(operatorPublicKey)
	if err != nil {
		return "", fmt.Errorf(
			"failed to convert operator public key to address: [%v]",
			err,
		)
	}

	return operatorAddress, nil
}

// operatorAddress returns the node's operator ID.
func (n *node) operatorID() (chain.OperatorID, error) {
	operatorAddress, err := n.operatorAddress()
	if err != nil {
		return 0, fmt.Errorf("failed to get operator address: [%v]", err)
	}

	operatorID, err := n.chain.GetOperatorID(operatorAddress)
	if err != nil {
		return 0, fmt.Errorf("failed to get operator ID: [%v]", err)
	}

	return operatorID, nil
}

// joinDKGIfEligible takes a seed value and undergoes the process of the
// distributed key generation if this node's operator proves to be eligible for
// the group generated by that seed. This is an interactive on-chain process,
// and joinDKGIfEligible can block for an extended period of time while it
// completes the on-chain operation. The execution can be delayed by an
// arbitrary number of blocks using the delayBlocks argument. This allows
// confirming the state on-chain - e.g. wait for the required number of
// confirming blocks - before executing the off-chain action.
func (n *node) joinDKGIfEligible(
	seed *big.Int,
	startBlock uint64,
	delayBlocks uint64,
) {
	n.dkgExecutor.executeDkgIfEligible(seed, startBlock, delayBlocks)
}

// validateDKG performs the submitted DKG result validation process.
// If the result is not valid, this function submits an on-chain result
// challenge. If the result is valid and the given node was involved in the DKG,
// this function schedules an on-chain approve that is submitted once the
// challenge period elapses.
func (n *node) validateDKG(
	seed *big.Int,
	submissionBlock uint64,
	result *DKGChainResult,
	resultHash [32]byte,
) {
	n.dkgExecutor.executeDkgValidation(seed, submissionBlock, result, resultHash)
}

// getSigningExecutor gets the signing executor responsible for executing
// signing related to a specific wallet whose part is controlled by this node.
// The second boolean return value indicates whether the node controls at least
// one signer for the given wallet.
func (n *node) getSigningExecutor(
	walletPublicKey *ecdsa.PublicKey,
) (*signingExecutor, bool, error) {
	n.signingExecutorsMutex.Lock()
	defer n.signingExecutorsMutex.Unlock()

	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		return nil, false, fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	executorKey := hex.EncodeToString(walletPublicKeyBytes)

	if executor, exists := n.signingExecutors[executorKey]; exists {
		return executor, true, nil
	}

	executorLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
	)

	signers := n.walletRegistry.getSigners(walletPublicKey)
	if len(signers) == 0 {
		// This is not an error because the node simply does not control
		// the given wallet.
		return nil, false, nil
	}

	// All signers belong to one wallet. Take that wallet from the
	// first signer.
	wallet := signers[0].wallet

	channelName := fmt.Sprintf(
		"%s-%s",
		ProtocolName,
		hex.EncodeToString(walletPublicKeyBytes),
	)

	broadcastChannel, err := n.netProvider.BroadcastChannelFor(channelName)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get broadcast channel: [%v]", err)
	}

	signing.RegisterUnmarshallers(broadcastChannel)
	announcer.RegisterUnmarshaller(broadcastChannel)
	broadcastChannel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &signingDoneMessage{}
	})

	membershipValidator := group.NewMembershipValidator(
		executorLogger,
		wallet.signingGroupOperators,
		n.chain.Signing(),
	)

	err = broadcastChannel.SetFilter(membershipValidator.IsInGroup)
	if err != nil {
		return nil, false, fmt.Errorf(
			"could not set filter for channel [%v]: [%v]",
			broadcastChannel.Name(),
			err,
		)
	}

	executorLogger.Infof(
		"signing executor created; controlling [%v] signers",
		len(signers),
	)

	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return nil, false, fmt.Errorf(
			"could not get block counter: [%v]",
			err,
		)
	}

	executor := newSigningExecutor(
		signers,
		broadcastChannel,
		membershipValidator,
		n.groupParameters,
		n.protocolLatch,
		n.getCurrentBlockHeight,
		n.waitForBlockHeight,
		signingAttemptsLimit,
		n.metricsRecorder,
	)

	n.signingExecutors[executorKey] = executor

	return executor, true, nil
}

// getCoordinationExecutor gets the coordination executor responsible for
// executing coordination related to a specific wallet whose part is controlled
// by this node. The second boolean return value indicates whether the node
// controls at least one signer for the given wallet.
func (n *node) getCoordinationExecutor(
	walletPublicKey *ecdsa.PublicKey,
) (*coordinationExecutor, bool, error) {
	n.coordinationExecutorsMutex.Lock()
	defer n.coordinationExecutorsMutex.Unlock()

	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		return nil, false, fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	executorKey := hex.EncodeToString(walletPublicKeyBytes)

	if executor, exists := n.coordinationExecutors[executorKey]; exists {
		return executor, true, nil
	}

	executorLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
	)

	signers := n.walletRegistry.getSigners(walletPublicKey)
	if len(signers) == 0 {
		// This is not an error because the node simply does not control
		// the given wallet.
		return nil, false, nil
	}

	// All signers belong to one wallet. Take that wallet from the
	// first signer.
	wallet := signers[0].wallet

	channelName := fmt.Sprintf(
		"%s-%s-coordination",
		ProtocolName,
		hex.EncodeToString(walletPublicKeyBytes),
	)

	broadcastChannel, err := n.netProvider.BroadcastChannelFor(channelName)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get broadcast channel: [%v]", err)
	}

	broadcastChannel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &coordinationMessage{}
	})

	membershipValidator := group.NewMembershipValidator(
		executorLogger,
		wallet.signingGroupOperators,
		n.chain.Signing(),
	)

	err = broadcastChannel.SetFilter(membershipValidator.IsInGroup)
	if err != nil {
		return nil, false, fmt.Errorf(
			"could not set filter for channel [%v]: [%v]",
			broadcastChannel.Name(),
			err,
		)
	}

	// The coordination executor does not need access to signers' key material.
	// It is enough to pass only their member indexes.
	membersIndexes := make([]group.MemberIndex, len(signers))
	for i, s := range signers {
		membersIndexes[i] = s.signingGroupMemberIndex
	}

	operatorAddress, err := n.operatorAddress()
	if err != nil {
		return nil, false, fmt.Errorf("failed to get operator address: [%v]", err)
	}

	executor := newCoordinationExecutor(
		n.chain,
		wallet,
		membersIndexes,
		operatorAddress,
		n.proposalGenerator,
		broadcastChannel,
		membershipValidator,
		n.protocolLatch,
		n.waitForBlockHeight,
	)

	n.coordinationExecutors[executorKey] = executor

	executorLogger.Infof(
		"coordination executor created; controlling [%v] signers",
		len(signers),
	)

	return executor, true, nil
}

// getInactivityClaimExecutor gets the inactivity claim executor responsible for
// executing inactivity claim signing and submission related to a specific
// wallet whose part is controlled by this node. The second boolean return value
// indicates whether the node controls at least one signer for the given wallet.
func (n *node) getInactivityClaimExecutor(
	walletPublicKey *ecdsa.PublicKey,
) (*inactivityClaimExecutor, bool, error) {
	n.inactivityClaimExecutorMutex.Lock()
	defer n.inactivityClaimExecutorMutex.Unlock()

	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		return nil, false, fmt.Errorf("cannot marshal wallet public key: [%v]", err)
	}

	executorKey := hex.EncodeToString(walletPublicKeyBytes)

	if executor, exists := n.inactivityClaimExecutors[executorKey]; exists {
		return executor, true, nil
	}

	executorLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
	)

	signers := n.walletRegistry.getSigners(walletPublicKey)
	if len(signers) == 0 {
		// This is not an error because the node simply does not control
		// the given wallet.
		return nil, false, nil
	}

	// All signers belong to one wallet. Take that wallet from the first signer.
	wallet := signers[0].wallet

	channelName := fmt.Sprintf(
		"%s-%s-inactivity",
		ProtocolName,
		hex.EncodeToString(walletPublicKeyBytes),
	)

	broadcastChannel, err := n.netProvider.BroadcastChannelFor(channelName)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get broadcast channel: [%v]", err)
	}

	inactivity.RegisterUnmarshallers(broadcastChannel)

	membershipValidator := group.NewMembershipValidator(
		executorLogger,
		wallet.signingGroupOperators,
		n.chain.Signing(),
	)

	err = broadcastChannel.SetFilter(membershipValidator.IsInGroup)
	if err != nil {
		return nil, false, fmt.Errorf(
			"could not set filter for channel [%v]: [%v]",
			broadcastChannel.Name(),
			err,
		)
	}

	executorLogger.Infof(
		"inactivity executor created; controlling [%v] signers",
		len(signers),
	)

	executor := newInactivityClaimExecutor(
		n.chain,
		signers,
		broadcastChannel,
		membershipValidator,
		n.groupParameters,
		n.protocolLatch,
		n.waitForBlockHeight,
	)

	n.inactivityClaimExecutors[executorKey] = executor

	return executor, true, nil
}

// handleHeartbeatProposal handles an incoming heartbeat proposal by
// orchestrating and dispatching an appropriate wallet action.
func (n *node) handleHeartbeatProposal(
	wallet wallet,
	proposal *HeartbeatProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet [0x%x]; "+
				"ignoring the received heartbeat request",
			walletPublicKeyBytes,
		)
		return
	}

	inactivityClaimExecutor, ok, err := n.getInactivityClaimExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get inactivity claim executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getInactivityClaimExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet [0x%x]; "+
				"ignoring the received heartbeat request",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the heartbeat action for wallet [0x%x]; "+
			"20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionHeartbeat.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newHeartbeatAction(
		walletActionLogger,
		n.chain,
		wallet,
		signingExecutor,
		proposal,
		n.heartbeatFailureCounter,
		inactivityClaimExecutor,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// handleDepositSweepProposal handles an incoming deposit sweep proposal by
// orchestrating and dispatching an appropriate wallet action.
func (n *node) handleDepositSweepProposal(
	wallet wallet,
	proposal *DepositSweepProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet [0x%x]; "+
				"ignoring the received deposit sweep proposal",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the deposit sweep action for wallet [0x%x]; "+
			"20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionDepositSweep.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newDepositSweepAction(
		walletActionLogger,
		n.chain,
		n.btcChain,
		wallet,
		signingExecutor,
		proposal,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// handleRedemptionProposal handles an incoming redemption proposal by
// orchestrating and dispatching an appropriate wallet action.
func (n *node) handleRedemptionProposal(
	wallet wallet,
	proposal *RedemptionProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet [0x%x]; "+
				"ignoring the received redemption proposal",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the redemption action for wallet [0x%x]; "+
			"20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionRedemption.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newRedemptionAction(
		walletActionLogger,
		n.chain,
		n.btcChain,
		wallet,
		signingExecutor,
		proposal,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// handleMovingFundsProposal handles an incoming moving funds proposal by
// orchestrating and dispatching an appropriate wallet action.
func (n *node) handleMovingFundsProposal(
	wallet wallet,
	proposal *MovingFundsProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet PKH [0x%x]; "+
				"ignoring the received moving funds proposal",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the moving funds action for wallet [0x%x]; "+
			"20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionMovingFunds.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newMovingFundsAction(
		walletActionLogger,
		n.chain,
		n.btcChain,
		wallet,
		signingExecutor,
		proposal,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// handleMovedFundsSweepProposal handles an incoming moved funds sweep proposal
// by orchestrating and dispatching an appropriate wallet action.
func (n *node) handleMovedFundsSweepProposal(
	wallet wallet,
	proposal *MovedFundsSweepProposal,
	startBlock uint64,
	expiryBlock uint64,
) {
	walletPublicKeyBytes, err := marshalPublicKey(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return
	}

	signingExecutor, ok, err := n.getSigningExecutor(wallet.publicKey)
	if err != nil {
		logger.Errorf("cannot get signing executor: [%v]", err)
		return
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry using their
	// public key hash. However, we are doing it just in case. The API
	// contract of getSigningExecutor may change one day.
	if !ok {
		logger.Infof(
			"node does not control signers of wallet PKH [0x%x]; "+
				"ignoring the received moved funds sweep proposal",
			walletPublicKeyBytes,
		)
		return
	}

	logger.Infof(
		"starting orchestration of the moved funds sweep action for wallet "+
			"[0x%x]; 20-byte public key hash of that wallet is [0x%x]",
		walletPublicKeyBytes,
		bitcoin.PublicKeyHash(wallet.publicKey),
	)

	walletActionLogger := logger.With(
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
		zap.String("action", ActionMovedFundsSweep.String()),
		zap.Uint64("startBlock", startBlock),
		zap.Uint64("expiryBlock", expiryBlock),
	)
	walletActionLogger.Infof("dispatching wallet action")

	action := newMovedFundsSweepAction(
		walletActionLogger,
		n.chain,
		n.btcChain,
		wallet,
		signingExecutor,
		proposal,
		startBlock,
		expiryBlock,
		n.waitForBlockHeight,
	)

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}

// coordinationLayerSettings represents settings for the coordination layer.
type coordinationLayerSettings struct {
	// executeCoordinationProcedureFn is a function executing the coordination
	// procedure for the given wallet and coordination window.
	executeCoordinationProcedureFn func(
		node *node,
		window *coordinationWindow,
		walletPublicKey *ecdsa.PublicKey,
	) (*coordinationResult, bool)

	// processCoordinationResultFn is a function processing the given
	// coordination result.
	processCoordinationResultFn func(
		node *node,
		result *coordinationResult,
	)
}

// runCoordinationLayer starts the coordination layer of the node. It is
// responsible for detecting new coordination windows, running coordination
// procedures for all wallets controlled by the node, and processing
// coordination results.
func (n *node) runCoordinationLayer(
	ctx context.Context,
	settings ...*coordinationLayerSettings,
) error {
	// Resolve settings for the coordination layer.
	var cls *coordinationLayerSettings
	switch len(settings) {
	case 1:
		cls = settings[0]
	default:
		cls = &coordinationLayerSettings{
			executeCoordinationProcedureFn: executeCoordinationProcedure,
			processCoordinationResultFn:    processCoordinationResult,
		}
	}

	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return fmt.Errorf("cannot get block counter: [%w]", err)
	}

	coordinationResultChan := make(chan *coordinationResult)

	// Prepare a callback function that will be called every time a new
	// coordination window is detected.
	onWindowFn := func(window *coordinationWindow) {
		// Fetch all wallets controlled by the node. It is important to
		// get the wallets every time the window is triggered as the
		// node may have started controlling a new wallet in the meantime.
		walletsPublicKeys := n.walletRegistry.getWalletsPublicKeys()

		for _, currentWalletPublicKey := range walletsPublicKeys {
			// Run an independent coordination procedure for the given wallet
			// in a separate goroutine. The coordination result will be sent
			// to the coordination result channel.
			go func(walletPublicKey *ecdsa.PublicKey) {
				result, ok := cls.executeCoordinationProcedureFn(
					n,
					window,
					walletPublicKey,
				)
				if ok {
					coordinationResultChan <- result
				}
			}(currentWalletPublicKey)
		}
	}

	// Start the coordination windows watcher.
	go watchCoordinationWindows(
		ctx,
		blockCounter.WatchBlocks,
		onWindowFn,
	)

	// Start the coordination result processor.
	go func() {
		for {
			select {
			case result := <-coordinationResultChan:
				go cls.processCoordinationResultFn(n, result)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// executeCoordinationProcedure executes the coordination procedure for the
// given wallet and coordination window.
func executeCoordinationProcedure(
	node *node,
	window *coordinationWindow,
	walletPublicKey *ecdsa.PublicKey,
) (*coordinationResult, bool) {
	walletPublicKeyBytes, err := marshalPublicKey(walletPublicKey)
	if err != nil {
		logger.Errorf("cannot marshal wallet public key: [%v]", err)
		return nil, false
	}

	procedureLogger := logger.With(
		zap.Uint64("coordinationBlock", window.coordinationBlock),
		zap.String("wallet", fmt.Sprintf("0x%x", walletPublicKeyBytes)),
	)

	procedureLogger.Infof("starting coordination procedure")

	executor, ok, err := node.getCoordinationExecutor(walletPublicKey)
	if err != nil {
		procedureLogger.Errorf("cannot get coordination executor: [%v]", err)
		return nil, false
	}
	// This check is actually redundant. We know the node controls some
	// wallet signers as we just got the wallet from the registry.
	// However, we are doing it just in case. The API contract of
	// getWalletsPublicKeys and/or getCoordinationExecutor may change one day.
	if !ok {
		procedureLogger.Infof("node does not control signers of this wallet")
		return nil, false
	}

	result, err := executor.coordinate(window)
	if err != nil {
		procedureLogger.Errorf("coordination procedure failed: [%v]", err)
		return nil, false
	}

	procedureLogger.Infof(
		"coordination procedure finished successfully with result [%s]",
		result,
	)

	return result, true
}

// processCoordinationResult processes the given coordination result.
func processCoordinationResult(node *node, result *coordinationResult) {
	logger.Infof("processing coordination result [%s]", result)

	// TODO: In the future, create coordination faults cache and
	//       record faults from the processed results there.

	proposedAction := result.proposal.ActionType()

	if proposedAction == ActionNoop {
		// No-op proposal cannot be processed so return early to avoid
		// panicking on the ValidityBlocks call.
		return
	}

	startBlock := result.window.endBlock()
	expiryBlock := startBlock + result.proposal.ValidityBlocks()

	switch proposedAction {
	case ActionHeartbeat:
		if proposal, ok := result.proposal.(*HeartbeatProposal); ok {
			node.handleHeartbeatProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	case ActionDepositSweep:
		if proposal, ok := result.proposal.(*DepositSweepProposal); ok {
			node.handleDepositSweepProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	case ActionRedemption:
		if proposal, ok := result.proposal.(*RedemptionProposal); ok {
			node.handleRedemptionProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	case ActionMovingFunds:
		if proposal, ok := result.proposal.(*MovingFundsProposal); ok {
			node.handleMovingFundsProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	case ActionMovedFundsSweep:
		if proposal, ok := result.proposal.(*MovedFundsSweepProposal); ok {
			node.handleMovedFundsSweepProposal(
				result.wallet,
				proposal,
				startBlock,
				expiryBlock,
			)
		}
	default:
		logger.Errorf("no handler for coordination result [%s]", result)
	}
}

// archiveClosedWallets archives closed or terminated wallets.
func (n *node) archiveClosedWallets() error {
	// Get all the wallets controlled by the node.
	walletPublicKeys := n.walletRegistry.getWalletsPublicKeys()

	for _, walletPublicKey := range walletPublicKeys {
		walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)

		walletID, err := n.chain.CalculateWalletID(walletPublicKey)
		if err != nil {
			return fmt.Errorf(
				"could not calculate wallet ID for wallet with public key "+
					"hash [0x%x]: [%v]",
				walletPublicKeyHash,
				err,
			)
		}

		isRegistered, err := n.chain.IsWalletRegistered(walletID)
		if err != nil {
			return fmt.Errorf(
				"could not check if wallet is registered for wallet with ID "+
					"[0x%x]: [%v]",
				walletPublicKeyHash,
				err,
			)
		}

		if !isRegistered {
			// If the wallet is no longer registered it means the wallet has
			// been closed or terminated.
			err := n.walletRegistry.archiveWallet(walletPublicKeyHash)
			if err != nil {
				return fmt.Errorf(
					"could not archive wallet with public key hash [0x%x]: [%v]",
					walletPublicKeyHash,
					err,
				)
			}

			logger.Infof(
				"successfully archived wallet with ID [0x%x] and public key "+
					"hash [0x%x]",
				walletID,
				walletPublicKeyHash,
			)
		}
	}

	return nil
}

// handleWalletClosure handles the wallet termination or closing process.
func (n *node) handleWalletClosure(walletID [32]byte) error {
	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return fmt.Errorf("error getting block counter [%w]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return fmt.Errorf("error getting current block [%w]", err)
	}

	// To verify there was no chain reorg and the wallet is really closed check
	// if it is registered. Both terminated and closed wallets are removed
	// from the ECDSA registry.
	stateCheck := func() (bool, error) {
		isRegistered, err := n.chain.IsWalletRegistered(walletID)
		if err != nil {
			return false, err
		}

		return !isRegistered, nil
	}

	// Wait a significant number of blocks to make sure the transaction has not
	// been reverted for some reason, e.g. due to a chain reorganization.
	result, err := ethereum.WaitForBlockConfirmations(
		blockCounter,
		currentBlock,
		walletClosureConfirmationBlocks,
		stateCheck,
	)
	if err != nil {
		return fmt.Errorf(
			"error while waiting for wallet closure confirmation [%w]",
			err,
		)
	}

	if !result {
		return fmt.Errorf("wallet closure not confirmed")
	}

	wallet, ok := n.walletRegistry.getWalletByID(walletID)
	if !ok {
		// Wallet was not found in the registry. The wallet is not controlled by
		// this node.
		logger.Infof(
			"node does not control wallet with ID [0x%x]; quitting wallet "+
				"archiving",
			walletID,
		)
		return nil
	}

	walletPublicKeyHash := bitcoin.PublicKeyHash(wallet.publicKey)

	err = n.walletRegistry.archiveWallet(walletPublicKeyHash)
	if err != nil {
		return fmt.Errorf("failed to archive the wallet: [%v]", err)
	}

	logger.Infof(
		"successfully archived wallet with wallet ID [0x%x] and public key "+
			"hash [0x%x]",
		walletID,
		walletPublicKeyHash,
	)

	return nil
}

// waitForBlockFn represents a function blocking the execution until the given
// block height.
type waitForBlockFn func(context.Context, uint64) error

// getCurrentBlockFn represents a function returning the current block height.
type getCurrentBlockFn func() (uint64, error)

// TODO: this should become a part of BlockHeightWaiter interface.
func (n *node) waitForBlockHeight(ctx context.Context, blockHeight uint64) error {
	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return err
	}

	wait, err := blockCounter.BlockHeightWaiter(blockHeight)
	if err != nil {
		return err
	}

	select {
	case <-wait:
	case <-ctx.Done():
	}

	return nil
}

// withCancelOnBlock returns a copy of the given ctx that is automatically
// cancelled on the given block or when the parent ctx is done. Note that the
// context can be cancelled earlier if the waitForBlockFn returns an error.
func withCancelOnBlock(
	ctx context.Context,
	block uint64,
	waitForBlockFn waitForBlockFn,
) (context.Context, context.CancelFunc) {
	blockCtx, cancelBlockCtx := context.WithCancel(ctx)

	go func() {
		defer cancelBlockCtx()

		err := waitForBlockFn(ctx, block)
		if err != nil {
			logger.Errorf(
				"failed to wait for block [%v]; "+
					"context cancelled earlier than expected",
				err,
			)
		}
	}()

	return blockCtx, cancelBlockCtx
}
