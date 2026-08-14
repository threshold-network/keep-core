package tbtc

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/announcer"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/inactivity"
	"github.com/keep-network/keep-core/pkg/tecdsa/signing"
)

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

	// Register a ROAST-retry coordinator for each local seat of this wallet so
	// the interactive FROST signing path can run for it. This must happen before
	// the executor is used: the executor entry's BeginOrchestrationForSession
	// looks the coordinator up per member, and an absent coordinator makes the
	// interactive drive fall through (fail-closed under interactive-only mode).
	// No-op unless built with frost_native && frost_roast_retry.
	registerRoastRetryCoordinatorForSeats(n, signers)

	blockCounter, err := n.chain.BlockCounter()
	if err != nil {
		return nil, false, fmt.Errorf(
			"could not get block counter: [%v]",
			err,
		)
	}

	walletGroupParameters, err := n.groupParametersForSigners(signers)
	if err != nil {
		return nil, false, err
	}

	executor := newSigningExecutor(
		signers,
		broadcastChannel,
		membershipValidator,
		walletGroupParameters,
		n.protocolLatch,
		blockCounter.CurrentBlock,
		n.waitForBlockHeight,
		signingAttemptsLimit,
	)

	// Wire metrics recorder if available
	if n.performanceMetrics != nil {
		executor.setMetricsRecorder(n.performanceMetrics)
	}

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
		// Ensure metrics recorder is set if metrics are available
		// (executor may have been created before metrics were initialized)
		if n.performanceMetrics != nil {
			executor.setMetricsRecorder(n.performanceMetrics)
		}
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

	// Wire metrics recorder if available
	if n.performanceMetrics != nil {
		executor.setMetricsRecorder(n.performanceMetrics)
	}

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

	walletGroupParameters, err := n.groupParametersForSigners(signers)
	if err != nil {
		return nil, false, err
	}

	executor := newInactivityClaimExecutor(
		n.chain,
		signers,
		broadcastChannel,
		membershipValidator,
		walletGroupParameters,
		n.protocolLatch,
		n.waitForBlockHeight,
	)

	n.inactivityClaimExecutors[executorKey] = executor

	return executor, true, nil
}

// groupParametersForSigners selects parameters from the registry that created
// the wallet. Native Schnorr signer material denotes a FROST wallet; legacy
// material denotes an ECDSA wallet. All signers in this slice belong to the
// same wallet, so the first signer is sufficient to identify the scheme.
func (n *node) groupParametersForSigners(
	signers []*signer,
) (*GroupParameters, error) {
	if len(signers) == 0 {
		return nil, fmt.Errorf("cannot select group parameters without signers")
	}

	if signingMaterialUsesSchnorrSignatures(signers[0].signingMaterial()) {
		if n.frostGroupParameters == nil {
			return nil, fmt.Errorf("FROST group parameters are not configured")
		}

		return n.frostGroupParameters, nil
	}

	if n.groupParameters == nil {
		return nil, fmt.Errorf("ECDSA group parameters are not configured")
	}

	return n.groupParameters, nil
}
