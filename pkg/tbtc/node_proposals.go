package tbtc

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/bitcoin"

	"go.uber.org/zap"
)

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
		n.transactionMonitor,
	)

	// Wire metrics recorder if available
	if n.performanceMetrics != nil {
		action.setMetricsRecorder(n.performanceMetrics)
	}

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
		n.transactionMonitor,
	)

	// Wire metrics recorder if available
	if n.performanceMetrics != nil {
		action.setMetricsRecorder(n.performanceMetrics)
	}

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
		n.transactionMonitor,
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
		n.transactionMonitor,
	)

	err = n.walletDispatcher.dispatch(action)
	if err != nil {
		walletActionLogger.Errorf("cannot dispatch wallet action: [%v]", err)
		return
	}

	walletActionLogger.Infof("wallet action dispatched successfully")
}
