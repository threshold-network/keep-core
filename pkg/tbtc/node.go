package tbtc

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-common/pkg/chain/ethereum"

	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
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
	ethereumNetwork ethereum.Network
	groupParameters *GroupParameters

	chain          Chain
	btcChain       bitcoin.Chain
	netProvider    net.Provider
	walletRegistry *walletRegistry

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

	// performanceMetrics is optional and used for recording performance metrics
	performanceMetrics interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}

	// windowMetricsTracker tracks detailed metrics for individual coordination windows
	windowMetricsTracker *coordinationWindowMetrics

	// transactionMonitor watches broadcast wallet transactions and alerts on
	// ones that remain unconfirmed long enough to be considered stuck.
	transactionMonitor *transactionMonitor
}

func newNode(
	ethereumNetwork ethereum.Network,
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
		ethereumNetwork:          ethereumNetwork,
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
		transactionMonitor:       newTransactionMonitor(btcChain),
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
	)

	return node, nil
}

// setPerformanceMetrics sets the performance metrics recorder for the node
// and wires it into components that support metrics.
func (n *node) setPerformanceMetrics(metrics interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	n.performanceMetrics = metrics

	// Initialize window metrics tracker with performance metrics
	// Keep metrics for the last 100 windows (approximately 25 hours at 900 blocks per window)
	if perfMetrics, ok := metrics.(clientinfo.PerformanceMetricsRecorder); ok {
		n.windowMetricsTracker = newCoordinationWindowMetrics(perfMetrics, 100)

		if n.transactionMonitor != nil {
			n.transactionMonitor.setMetricsRecorder(perfMetrics)
		}
	}

	if n.walletDispatcher != nil {
		n.walletDispatcher.setMetricsRecorder(metrics)
	}
	if n.dkgExecutor != nil {
		n.dkgExecutor.setMetricsRecorder(metrics)
	}

	// Wire redemption metrics to proposal generator if it supports it
	// This uses a type assertion to check if proposalGenerator is a *ProposalGenerator
	// from the tbtcpg package. We can't import tbtcpg here to avoid circular dependencies,
	// so we use an interface check instead.
	if pg, ok := n.proposalGenerator.(interface {
		SetRedemptionMetricsRecorder(recorder interface {
			SetGauge(name string, value float64)
		})
	}); ok {
		pg.SetRedemptionMetricsRecorder(metrics)
	}

	// Wire reservation metrics to proposal generator if it supports it,
	// mirroring the redemption wiring above. A no-op on non-reservation
	// deployments since SetReservationMetricsRecorder finds no reservation
	// tasks in that case.
	if pg, ok := n.proposalGenerator.(interface {
		SetReservationMetricsRecorder(recorder interface {
			SetGauge(name string, value float64)
		})
	}); ok {
		pg.SetReservationMetricsRecorder(metrics)
	}

	// Update metrics recorder for all cached coordination executors
	// This is important because executors may be created before metrics are set
	n.coordinationExecutorsMutex.Lock()
	for _, executor := range n.coordinationExecutors {
		executor.setMetricsRecorder(metrics)
	}
	n.coordinationExecutorsMutex.Unlock()
}

// GetCoordinationWindowsSummary returns a summary of coordination window metrics.
// Returns nil if the window metrics tracker is not initialized.
func (n *node) GetCoordinationWindowsSummary() *WindowMetricsSummary {
	if n.windowMetricsTracker == nil {
		return nil
	}
	summary := n.windowMetricsTracker.GetSummary()
	return &summary
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

func (n *node) ResolveWalletMembers(walletPublicKeyHash [20]byte) ([]uint32, error) {
	wallet, found := n.walletRegistry.getWalletByPublicKeyHash(walletPublicKeyHash)
	if !found {
		return nil, fmt.Errorf("wallet not found")
	}

	operatorIDs := make([]uint32, len(wallet.signingGroupOperators))
	for i, operatorAddress := range wallet.signingGroupOperators {
		operatorID, err := n.chain.GetOperatorID(operatorAddress)
		if err != nil {
			return nil, err
		}
		operatorIDs[i] = uint32(operatorID)
	}
	return operatorIDs, nil
}
