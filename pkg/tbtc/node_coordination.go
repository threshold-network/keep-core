package tbtc

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"

	"go.uber.org/zap"
)

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
	// Start the background monitor that alerts on stuck (long-unconfirmed)
	// wallet transactions.
	if n.transactionMonitor != nil {
		go n.transactionMonitor.run(ctx)
	}

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

	// Track the previous window to record its end when a new one starts
	// Use a mutex to safely access from multiple goroutines
	var previousWindowMu sync.Mutex
	var previousWindow *coordinationWindow

	// Prepare a callback function that will be called every time a new
	// coordination window is detected.
	onWindowFn := func(window *coordinationWindow) {
		previousWindowMu.Lock()
		// Record end of previous window if it exists
		if previousWindow != nil && n.windowMetricsTracker != nil {
			n.windowMetricsTracker.recordWindowEnd(previousWindow)
		}
		previousWindowMu.Unlock()

		// Track coordination window detection
		if n.performanceMetrics != nil {
			n.performanceMetrics.IncrementCounter(clientinfo.MetricCoordinationWindowsDetectedTotal, 1)
		}

		// Record window start in detailed metrics tracker
		if n.windowMetricsTracker != nil {
			n.windowMetricsTracker.recordWindowStart(window)
		}

		previousWindowMu.Lock()
		previousWindow = window
		previousWindowMu.Unlock()

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

	// Start a cleanup goroutine to record the end time of the last window on shutdown
	go func() {
		<-ctx.Done()
		// Record end time for the active window if it exists and hasn't been ended yet
		previousWindowMu.Lock()
		if previousWindow != nil && n.windowMetricsTracker != nil {
			n.windowMetricsTracker.recordWindowEnd(previousWindow)
		}
		previousWindowMu.Unlock()
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

	startTime := time.Now()
	result, err := executor.coordinate(window)
	duration := time.Since(startTime)

	if err != nil {
		procedureLogger.Errorf("coordination procedure failed: [%v]", err)
		// Metrics are already recorded in executor.coordinate() for failures

		// Record window metrics for failed coordination
		if node.windowMetricsTracker != nil {
			walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
			// Extract leader and faults from partial result if available
			// (e.g., when follower routine fails, we know who the leader was)
			leader := chain.Address("")
			var faults []*coordinationFault
			if result != nil {
				leader = result.leader
				faults = result.faults
			}
			node.windowMetricsTracker.recordWalletCoordination(
				window,
				walletPublicKeyHash,
				leader,
				"",
				false,
				duration,
				faults,
				err, // capture the error message
			)
		}
		return nil, false
	}

	procedureLogger.Infof(
		"coordination procedure finished successfully with result [%s]",
		result,
	)

	// Metrics are already recorded in executor.coordinate() for successful executions

	// Record window metrics for successful coordination
	if node.windowMetricsTracker != nil {
		walletPublicKeyHash := bitcoin.PublicKeyHash(walletPublicKey)
		actionType := ""
		if result.proposal != nil {
			actionType = result.proposal.ActionType().String()
		}
		node.windowMetricsTracker.recordWalletCoordination(
			window,
			walletPublicKeyHash,
			result.leader,
			actionType,
			true,
			duration,
			result.faults,
			nil, // no error on success
		)
	}

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
