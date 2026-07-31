package tbtc

import (
	"context"
	"fmt"
	"time"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

const frostDKGRecoveryRetryInterval = 15 * time.Second

type frostDKGExecutor func(
	context.Context,
	*node,
	FrostDKGChain,
	*FrostDKGStartedEvent,
	[]group.MemberIndex,
	*GroupSelectionResult,
) bool

func initializeFrostDKGCoordinator(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
) {
	if frostChain == nil || !frostChain.FrostWalletRegistryAvailable() {
		return
	}

	frostDeduplicator := newDeduplicator()

	_ = frostChain.OnBridgeNewWalletRequested(
		func(event *BridgeNewWalletRequestedEvent) {
			logger.Infof(
				"observed Bridge NewWalletRequested event at block [%v]; "+
					"waiting for FROST DkgStarted seed callback",
				event.BlockNumber,
			)
		},
	)

	_ = frostChain.OnFrostDKGStarted(func(event *FrostDKGStartedEvent) {
		go handleFrostDKGStarted(
			ctx,
			node,
			frostChain,
			frostDeduplicator,
			event,
			true,
		)
	})

	_ = frostChain.OnFrostDKGResultSubmitted(
		func(event *FrostDKGResultSubmittedEvent) {
			go handleFrostDKGResultSubmitted(
				ctx,
				node,
				frostChain,
				frostDeduplicator,
				event,
			)
		},
	)

	go recoverFrostDKGCoordinatorState(ctx, node, frostChain, frostDeduplicator)
}

func handleFrostDKGStarted(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	deduplicator *deduplicator,
	event *FrostDKGStartedEvent,
	waitForConfirmation bool,
) bool {
	return handleFrostDKGStartedWithExecutor(
		ctx,
		node,
		frostChain,
		deduplicator,
		event,
		waitForConfirmation,
		executeFrostDKGIfPossible,
	)
}

func handleFrostDKGStartedWithExecutor(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	deduplicator *deduplicator,
	event *FrostDKGStartedEvent,
	waitForConfirmation bool,
	execute frostDKGExecutor,
) (completed bool) {
	lease, ok := deduplicator.beginDKGStarted(event.Seed)
	if !ok {
		logger.Infof(
			"FROST DKG started event with seed [0x%x] is already completed or "+
				"being processed",
			event.Seed,
		)
		return deduplicator.dkgSeedCache.Has(event.Seed.Text(16))
	}
	defer func() { lease.finish(completed) }()

	if waitForConfirmation {
		confirmationBlock := event.BlockNumber + dkgStartedConfirmationBlocks
		logger.Infof(
			"observed FROST DKG started event with seed [0x%x] and "+
				"starting block [%v]; waiting for block [%v] to confirm",
			event.Seed,
			event.BlockNumber,
			confirmationBlock,
		)

		if err := node.waitForBlockHeight(ctx, confirmationBlock); err != nil {
			logger.Errorf("failed to confirm FROST DKG started event: [%v]", err)
			return false
		}
		if ctx.Err() != nil {
			logger.Errorf("stopping FROST DKG started event handling: [%v]", ctx.Err())
			return false
		}
	}

	dkgState, err := frostChain.GetFrostDKGState()
	if err != nil {
		logger.Errorf("failed to check FROST DKG state: [%v]", err)
		return false
	}
	if dkgState != AwaitingResult {
		logger.Infof(
			"FROST DKG started event with seed [0x%x] and starting "+
				"block [%v] was not confirmed",
			event.Seed,
			event.BlockNumber,
		)
		return true
	}

	startBlock := uint64(0)
	if event.BlockNumber > dkgStartedConfirmationBlocks {
		startBlock = event.BlockNumber - dkgStartedConfirmationBlocks
	}

	pastEvents, err := frostChain.PastFrostDKGStartedEvents(
		&FrostDKGStartedEventFilter{
			StartBlock: startBlock,
		},
	)
	if err != nil {
		logger.Errorf("failed to get past FROST DKG started events: [%v]", err)
		return false
	}
	if len(pastEvents) == 0 {
		logger.Errorf("no past FROST DKG started events")
		return false
	}

	lastEvent := pastEvents[len(pastEvents)-1]
	if lastEvent.Seed.Cmp(event.Seed) != 0 {
		logger.Infof(
			"FROST DKG started event with seed [0x%x] was superseded by seed "+
				"[0x%x]; transferring handling to the latest event",
			event.Seed,
			lastEvent.Seed,
		)

		// Mark the triggering event as terminal before re-entering so stale
		// subscription replays remain suppressed. The recursive handler must
		// acquire a fresh lease for the resolved seed before it performs any
		// membership or DKG work. If another handler already owns or completed
		// that seed, beginDKGStarted suppresses this path; if it previously
		// failed and released its lease, this path safely becomes the retry.
		completed = true
		lease.finish(completed)
		return handleFrostDKGStartedWithExecutor(
			ctx,
			node,
			frostChain,
			deduplicator,
			lastEvent,
			waitForConfirmation,
			execute,
		)
	}

	memberIndexes, groupSelectionResult, err := localFrostMembership(
		node,
		frostChain,
	)
	if err != nil {
		logger.Errorf("failed to resolve FROST DKG membership: [%v]", err)
		return false
	}

	if len(memberIndexes) == 0 {
		logger.Infof(
			"FROST DKG with seed [0x%x] at block [%v] selected a group "+
				"that does not include this operator; monitoring only",
			lastEvent.Seed,
			lastEvent.BlockNumber,
		)
		return true
	}

	return execute(
		ctx,
		node,
		frostChain,
		lastEvent,
		memberIndexes,
		groupSelectionResult,
	)
}

func recoverFrostDKGCoordinatorState(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	deduplicator *deduplicator,
) {
	recoverFrostDKGCoordinatorStateWithRetryInterval(
		ctx,
		node,
		frostChain,
		deduplicator,
		frostDKGRecoveryRetryInterval,
	)
}

func recoverFrostDKGCoordinatorStateWithRetryInterval(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	deduplicator *deduplicator,
	retryInterval time.Duration,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if recoverFrostDKGCoordinatorStateOnce(
			ctx,
			node,
			frostChain,
			deduplicator,
		) {
			return
		}

		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
		}
	}
}

// recoverFrostDKGCoordinatorStateOnce returns true when recovery reached a
// terminal on-chain state or successfully handed the active state to its
// handler. It returns false for retryable reads and handler failures.
func recoverFrostDKGCoordinatorStateOnce(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	deduplicator *deduplicator,
) bool {
	state, err := frostChain.GetFrostDKGState()
	if err != nil {
		logger.Errorf("failed to recover FROST DKG state: [%v]", err)
		return false
	}

	switch state {
	case AwaitingResult:
		startBlock, err := frostDKGRecoveryStartBlock(node, frostChain)
		if err != nil {
			logger.Errorf("failed to resolve FROST DKG recovery start block: [%v]", err)
			return false
		}

		events, err := frostChain.PastFrostDKGStartedEvents(
			&FrostDKGStartedEventFilter{StartBlock: startBlock},
		)
		if err != nil {
			logger.Errorf("failed to recover past FROST DKG started events: [%v]", err)
			return false
		}
		if len(events) == 0 {
			logger.Warnf("FROST DKG state is AwaitingResult but no DkgStarted event was found")
			return false
		}

		return handleFrostDKGStarted(
			ctx,
			node,
			frostChain,
			deduplicator,
			events[len(events)-1],
			false,
		)

	case Challenge:
		startBlock, err := frostDKGRecoveryStartBlock(node, frostChain)
		if err != nil {
			logger.Errorf("failed to resolve FROST DKG recovery start block: [%v]", err)
			return false
		}

		events, err := frostChain.PastFrostDKGResultSubmittedEvents(
			&FrostDKGResultSubmittedEventFilter{StartBlock: startBlock},
		)
		if err != nil {
			logger.Errorf("failed to recover past FROST DKG result submissions: [%v]", err)
			return false
		}
		if len(events) == 0 {
			logger.Warnf("FROST DKG state is Challenge but no result submission was found")
			return false
		}

		return handleFrostDKGResultSubmitted(
			ctx,
			node,
			frostChain,
			deduplicator,
			events[len(events)-1],
		)
	}

	return true
}

func handleFrostDKGResultSubmitted(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	deduplicator *deduplicator,
	event *FrostDKGResultSubmittedEvent,
) (completed bool) {
	lease, ok := deduplicator.beginDKGResultSubmitted(
		event.Seed,
		event.ResultHash,
		event.BlockNumber,
	)
	if !ok {
		logger.Infof(
			"FROST DKG result with hash [0x%x] for seed [0x%x] at block [%v] "+
				"is already completed or being processed",
			event.ResultHash,
			event.Seed,
			event.BlockNumber,
		)
		return deduplicator.dkgResultHashCache.Has(
			dkgResultSubmittedCacheKey(
				event.Seed,
				event.ResultHash,
				event.BlockNumber,
			),
		)
	}
	defer func() { lease.finish(completed) }()

	valid, reason, err := frostChain.IsFrostDKGResultValid(event.Result)
	if err != nil {
		logger.Errorf(
			"failed to validate FROST DKG result [0x%x]: [%v]",
			event.ResultHash,
			err,
		)
		return false
	}

	if !valid {
		logger.Warnf(
			"challenging invalid FROST DKG result [0x%x]: [%s]",
			event.ResultHash,
			reason,
		)
		return challengeInvalidFrostDKGResult(ctx, node, frostChain, event)
	}

	memberIndexes, _, err := localFrostMembership(node, frostChain)
	if err != nil {
		logger.Errorf("failed to resolve local FROST DKG membership: [%v]", err)
		return false
	}
	if len(memberIndexes) == 0 {
		logger.Infof(
			"FROST DKG result [0x%x] is valid; this operator is not in the "+
				"selected group and will not approve",
			event.ResultHash,
		)
		return true
	}

	params, err := frostChain.FrostDKGParameters()
	if err != nil {
		logger.Errorf("failed to get FROST DKG parameters: [%v]", err)
		return false
	}
	if params == nil {
		logger.Errorf("FROST DKG parameters are nil")
		return false
	}
	if ctx.Err() != nil {
		logger.Errorf("stopping FROST DKG result handling: [%v]", ctx.Err())
		return false
	}

	challengePeriodEndBlock := event.BlockNumber + params.ChallengePeriodBlocks
	approvePrecedencePeriodStartBlock := challengePeriodEndBlock + 1
	approvePeriodStartBlock := approvePrecedencePeriodStartBlock +
		params.ApprovePrecedencePeriodBlocks

	for _, currentMemberIndex := range memberIndexes {
		memberIndex := currentMemberIndex
		var approvalBlock uint64
		if uint64(memberIndex) == event.Result.SubmitterMemberIndex {
			approvalBlock = approvePrecedencePeriodStartBlock
		} else {
			approvalBlock = approvePeriodStartBlock +
				uint64(memberIndex-1)*dkgResultApprovalDelayStepBlocks
		}

		go scheduleFrostDKGResultApproval(
			ctx,
			node,
			frostChain,
			event,
			memberIndex,
			approvalBlock,
		)
	}
	return true
}

func challengeInvalidFrostDKGResult(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	event *FrostDKGResultSubmittedEvent,
) bool {
	for attempt := uint64(1); ; attempt++ {
		select {
		case <-ctx.Done():
			logger.Errorf(
				"stopping FROST DKG challenge confirmation: [%v]",
				ctx.Err(),
			)
			return false
		default:
		}

		state, err := frostChain.GetFrostDKGState()
		if err != nil {
			logger.Errorf("failed to check FROST DKG state before challenge: [%v]", err)
			return false
		}
		if state != Challenge {
			logger.Infof(
				"invalid FROST DKG result [0x%x] challenged successfully",
				event.ResultHash,
			)
			return true
		}

		if err := frostChain.ChallengeFrostDKGResult(event.Result); err != nil {
			state, stateErr := frostChain.GetFrostDKGState()
			if stateErr == nil && state != Challenge {
				logger.Infof(
					"invalid FROST DKG result [0x%x] was challenged by another "+
						"operator",
					event.ResultHash,
				)
				return true
			}

			logger.Errorf(
				"failed to challenge FROST DKG result [0x%x]: [%v]",
				event.ResultHash,
				err,
			)
			if stateErr != nil {
				logger.Errorf(
					"failed to check FROST DKG state after challenge error: [%v]",
					stateErr,
				)
			}
			return false
		}

		currentBlock, err := currentFrostDKGBlock(node)
		if err != nil {
			logger.Errorf(
				"failed to get current block after FROST DKG challenge: [%v]",
				err,
			)
			return false
		}

		confirmationBlock := currentBlock + dkgResultChallengeConfirmationBlocks
		logger.Infof(
			"challenging invalid FROST DKG result [0x%x], attempt [%v]; "+
				"waiting for block [%v] to confirm DKG state",
			event.ResultHash,
			attempt,
			confirmationBlock,
		)

		if err := node.waitForBlockHeight(ctx, confirmationBlock); err != nil {
			logger.Errorf(
				"failed to wait for FROST DKG challenge confirmation: [%v]",
				err,
			)
			return false
		}
		if ctx.Err() != nil {
			logger.Errorf(
				"stopping FROST DKG challenge confirmation: [%v]",
				ctx.Err(),
			)
			return false
		}

		state, err = frostChain.GetFrostDKGState()
		if err != nil {
			logger.Errorf("failed to check FROST DKG state after challenge: [%v]", err)
			return false
		}
		if state != Challenge {
			logger.Infof(
				"invalid FROST DKG result [0x%x] challenged successfully",
				event.ResultHash,
			)
			return true
		}

		logger.Infof(
			"invalid FROST DKG result [0x%x] still not challenged; retrying",
			event.ResultHash,
		)
	}
}

func scheduleFrostDKGResultApproval(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	event *FrostDKGResultSubmittedEvent,
	memberIndex group.MemberIndex,
	approvalBlock uint64,
) {
	logger.Infof(
		"FROST DKG result [0x%x] is valid; member [%d] scheduling approval "+
			"at block [%v]",
		event.ResultHash,
		memberIndex,
		approvalBlock,
	)

	if err := node.waitForBlockHeight(ctx, approvalBlock); err != nil {
		logger.Errorf(
			"member [%d] failed to wait for FROST DKG approval block [%v]: [%v]",
			memberIndex,
			approvalBlock,
			err,
		)
		return
	}
	if ctx.Err() != nil {
		logger.Errorf(
			"stopping FROST DKG result approval for member [%d]: [%v]",
			memberIndex,
			ctx.Err(),
		)
		return
	}

	state, err := frostChain.GetFrostDKGState()
	if err != nil {
		logger.Errorf("failed to check FROST DKG state before approval: [%v]", err)
		return
	}
	if state != Challenge {
		logger.Infof(
			"skipping FROST DKG result [0x%x] approval; current state is [%v]",
			event.ResultHash,
			state,
		)
		return
	}

	valid, reason, err := frostChain.IsFrostDKGResultValid(event.Result)
	if err != nil {
		logger.Errorf(
			"failed to revalidate FROST DKG result [0x%x] before approval: [%v]",
			event.ResultHash,
			err,
		)
		return
	}
	if !valid {
		logger.Errorf(
			"FROST DKG result [0x%x] became invalid before approval: [%s]",
			event.ResultHash,
			reason,
		)
		return
	}

	if err := frostChain.ApproveFrostDKGResult(event.Result); err != nil {
		logger.Errorf(
			"member [%d] failed to approve FROST DKG result [0x%x]: [%v]",
			memberIndex,
			event.ResultHash,
			err,
		)
	}
}

func localFrostMembership(
	node *node,
	frostChain FrostDKGChain,
) ([]group.MemberIndex, *GroupSelectionResult, error) {
	operatorAddress, err := node.operatorAddress()
	if err != nil {
		return nil, nil, err
	}

	groupSelectionResult, err := frostChain.SelectFrostGroup()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to select FROST group: [%v]", err)
	}

	memberIndexes := make([]group.MemberIndex, 0)
	for i, selectedOperatorAddress := range groupSelectionResult.OperatorsAddresses {
		if selectedOperatorAddress == operatorAddress {
			memberIndexes = append(memberIndexes, group.MemberIndex(i+1))
		}
	}

	return memberIndexes, groupSelectionResult, nil
}

func currentFrostDKGBlock(node *node) (uint64, error) {
	blockCounter, err := node.chain.BlockCounter()
	if err != nil {
		return 0, err
	}

	return blockCounter.CurrentBlock()
}

func frostDKGRecoveryStartBlock(
	node *node,
	frostChain FrostDKGChain,
) (uint64, error) {
	currentBlock, err := currentFrostDKGBlock(node)
	if err != nil {
		return 0, err
	}

	params, err := frostChain.FrostDKGParameters()
	if err != nil {
		return 0, err
	}

	lookBackBlocks, err := frostDKGRecoveryLookBackBlocks(
		params,
		node.frostGroupParameters,
	)
	if err != nil {
		return 0, err
	}

	return boundedFrostDKGRecoveryStartBlock(currentBlock, lookBackBlocks), nil
}

func frostDKGRecoveryLookBackBlocks(
	params *DKGParameters,
	groupParameters *GroupParameters,
) (uint64, error) {
	if params == nil {
		return 0, fmt.Errorf("FROST DKG parameters are nil")
	}
	if groupParameters == nil {
		return 0, fmt.Errorf("group parameters are nil")
	}

	// Bound cold-start eth_getLogs by the live on-chain timing parameters while
	// still covering the full lifecycle that may require local action after a
	// restart: result submission, challenge, submitter precedence, and delayed
	// approval fallback across the full group.
	return params.SubmissionTimeoutBlocks +
			params.ChallengePeriodBlocks +
			params.ApprovePrecedencePeriodBlocks +
			uint64(groupParameters.GroupSize)*dkgResultApprovalDelayStepBlocks +
			dkgStartedConfirmationBlocks,
		nil
}

func boundedFrostDKGRecoveryStartBlock(
	currentBlock uint64,
	lookBackBlocks uint64,
) uint64 {
	if currentBlock <= lookBackBlocks {
		return 0
	}

	return currentBlock - lookBackBlocks
}
