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
	if n.frostPreSignAuthorizationBackend != nil {
		localMemberIndexes := make([]group.MemberIndex, 0, len(signers))
		for _, signer := range signers {
			localMemberIndexes = append(
				localMemberIndexes,
				signer.signingGroupMemberIndex,
			)
		}
		gate, err := newThresholdFrostPreSignAuthorizationGate(
			n.frostPreSignAuthorizationBackend,
			n.frostPreSignActivationProfile,
			n.frostDurableSessionStoreBinding,
			n.frostProductionSignerReadiness,
			n.frostNativeSignerAnchorAdmission,
			n.chain.Signing(),
			broadcastChannel,
			membershipValidator,
			wallet,
			localMemberIndexes,
		)
		if err != nil {
			return nil, false, fmt.Errorf(
				"cannot create FROST pre-sign authorization gate: [%w]",
				err,
			)
		}
		executor.preSignAuthorizationGate = gate
		executor.broadcastOutbox = n.bitcoinBroadcastOutbox

		// The seat ceiling is a property of this node's seat count in this
		// wallet and of protocol constants, so it is decided the moment the
		// wallet is formed and never changes for the wallet's whole life. The
		// gate is the only thing that reports it otherwise, and only inside the
		// error of an authorization it has already refused, so an operator over
		// the ceiling would learn about it the first time a deposit sweep is
		// proposed - which may be weeks after the seats were awarded, and is
		// the worst possible moment. Say it once, here, at the same place the
		// executor announces how many signers it controls.
		//
		// No production node is expected to trip this now that admission
		// reserves one input at a time: a wallet's whole hundred-seat set fits
		// the certified windows for one input. It stays because the numbers it
		// reads are protocol constants, and a change to any of them must
		// announce itself rather than quietly excluding an operator again.
		//
		// The gate's own deduplicated, validated seat set is used rather than
		// localMemberIndexes because that is exactly what reservePreSign
		// charges for; a duplicated index in the registry would otherwise make
		// this warn about a seat count admission never sees.
		//
		// Gated on Schnorr material because getSigningExecutor builds a gate
		// for every wallet on a FROST-enabled node, including the legacy ECDSA
		// wallets still draining, and walletTransactionExecutor only routes
		// through the gate when the wallet signs with Schnorr. Warning about a
		// seat ceiling that cannot apply to an ECDSA wallet would be noise.
		if executor.usesSchnorrSignatures() {
			if warning, exceeded := frostPreSignLocalSeatCeilingWarning(
				uint64(len(gate.localMemberIndexes)),
				gate.maximumAttempts,
			); exceeded {
				executorLogger.Warnf("%s", warning)
			}
		}
	}

	// Wire metrics recorder if available
	if n.performanceMetrics != nil {
		executor.setMetricsRecorder(n.performanceMetrics)
	}

	n.signingExecutors[executorKey] = executor

	return executor, true, nil
}

// frostPreSignLocalSeatCeilingWarning states, at wallet-executor construction
// time, that this node's seat count in a FROST wallet is above the count anchor
// admission can serve for a single pre-sign transaction input. It returns the
// warning text and whether the ceiling is exceeded at all.
//
// The exclusion this reports is whole-node, not per-seat.
// thresholdFrostPreSignAuthorizationGate charges reservePreSign for its
// complete local seat set in one reservation, so a set one seat over the
// ceiling fails the reservation and the node contributes nothing at all - every
// one of its seats is lost to the wallet's signing threshold, not just the
// surplus one. That is worth stating explicitly because the natural reading of
// "a ceiling of N" is that N seats still sign, and they do not.
//
// It no longer takes a batch size, and there is no longer a second lever to
// offer. Anchor admission reserves one input at a time and a batch signs its
// inputs sequentially, so one input costs the same in a one-input batch as in a
// full twenty-one input sweep: an operator over this ceiling cannot get under
// it by proposing smaller sweeps, only by shedding seats. Under the current
// constants nothing is over it - a wallet's entire hundred-seat set fits inside
// the certified windows for one input - so this is a guard against a constant
// change rather than a warning any production node is expected to see.
//
// Kept as a pure function of the two numbers so it can be exercised without
// standing up a wallet, a gate, or a native build.
func frostPreSignLocalSeatCeilingWarning(
	localSeatCount uint64,
	maximumSigningAttempts uint64,
) (string, bool) {
	// Mirrors the gate: an unset limit means the package default, and charging
	// the ceiling scan a different attempt count than admission uses would make
	// this warn about a ceiling that does not exist.
	if maximumSigningAttempts == 0 {
		maximumSigningAttempts = signingAttemptsLimit
	}

	admissibleSeats := frostPreSignMaximumAdmissibleLocalSeatCount(
		maximumSigningAttempts,
	)
	if admissibleSeats > 0 && localSeatCount <= admissibleSeats {
		return "", false
	}
	if admissibleSeats == 0 {
		// Not this node's problem to fix: no seat count at all can serve even a
		// single input under the current windows, so shedding seats would not
		// help and only a protocol-level change would.
		return fmt.Sprintf(
			"this node holds [%d] of this FROST wallet's seats, and no local "+
				"seat count can sign a single pre-sign transaction input within "+
				"the certified anchor restart windows; every deposit sweep on this "+
				"wallet will be refused for every member, and only enlarging those "+
				"windows or lowering the signing-attempt limit [%d] changes that",
			localSeatCount,
			maximumSigningAttempts,
		), true
	}

	return fmt.Sprintf(
		"this node holds [%d] of this FROST wallet's seats, above the [%d]-seat "+
			"ceiling for a single pre-sign transaction input; anchor admission "+
			"refuses the node as a whole rather than the surplus seats, so all "+
			"[%d] of its seats are excluded from every deposit sweep for this "+
			"wallet's whole life and are lost to the wallet's signing threshold. "+
			"Batch size is not a lever - admission reserves one input at a time "+
			"and a sweep signs its inputs sequentially - so shed seats down to "+
			"[%d]",
		localSeatCount,
		admissibleSeats,
		localSeatCount,
		admissibleSeats,
	), true
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
	executor.suppressHeartbeat = signingMaterialUsesSchnorrSignatures(
		signers[0].signingMaterial(),
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
