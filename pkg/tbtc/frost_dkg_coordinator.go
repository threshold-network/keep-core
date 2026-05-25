package tbtc

import (
	"context"
	"fmt"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

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
) {
	if ok := deduplicator.notifyDKGStarted(event.Seed); !ok {
		logger.Infof(
			"FROST DKG started event with seed [0x%x] has already been processed",
			event.Seed,
		)
		return
	}

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
			return
		}
	}

	dkgState, err := frostChain.GetFrostDKGState()
	if err != nil {
		logger.Errorf("failed to check FROST DKG state: [%v]", err)
		return
	}
	if dkgState != AwaitingResult {
		logger.Infof(
			"FROST DKG started event with seed [0x%x] and starting "+
				"block [%v] was not confirmed",
			event.Seed,
			event.BlockNumber,
		)
		return
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
		return
	}
	if len(pastEvents) == 0 {
		logger.Errorf("no past FROST DKG started events")
		return
	}

	lastEvent := pastEvents[len(pastEvents)-1]
	memberIndexes, groupSelectionResult, err := localFrostMembership(
		node,
		frostChain,
	)
	if err != nil {
		logger.Errorf("failed to resolve FROST DKG membership: [%v]", err)
		return
	}

	if len(memberIndexes) == 0 {
		logger.Infof(
			"FROST DKG with seed [0x%x] at block [%v] selected a group "+
				"that does not include this operator; monitoring only",
			lastEvent.Seed,
			lastEvent.BlockNumber,
		)
		return
	}

	executeFrostDKGIfPossible(
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
	state, err := frostChain.GetFrostDKGState()
	if err != nil {
		logger.Errorf("failed to recover FROST DKG state: [%v]", err)
		return
	}

	switch state {
	case AwaitingResult:
		startBlock, err := frostDKGRecoveryStartBlock(node, frostChain)
		if err != nil {
			logger.Errorf("failed to resolve FROST DKG recovery start block: [%v]", err)
			return
		}

		events, err := frostChain.PastFrostDKGStartedEvents(
			&FrostDKGStartedEventFilter{StartBlock: startBlock},
		)
		if err != nil {
			logger.Errorf("failed to recover past FROST DKG started events: [%v]", err)
			return
		}
		if len(events) == 0 {
			logger.Warnf("FROST DKG state is AwaitingResult but no DkgStarted event was found")
			return
		}

		handleFrostDKGStarted(
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
			return
		}

		events, err := frostChain.PastFrostDKGResultSubmittedEvents(
			&FrostDKGResultSubmittedEventFilter{StartBlock: startBlock},
		)
		if err != nil {
			logger.Errorf("failed to recover past FROST DKG result submissions: [%v]", err)
			return
		}
		if len(events) == 0 {
			logger.Warnf("FROST DKG state is Challenge but no result submission was found")
			return
		}

		handleFrostDKGResultSubmitted(
			ctx,
			node,
			frostChain,
			deduplicator,
			events[len(events)-1],
		)
	}
}

func handleFrostDKGResultSubmitted(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	deduplicator *deduplicator,
	event *FrostDKGResultSubmittedEvent,
) {
	if ok := deduplicator.notifyDKGResultSubmitted(
		event.Seed,
		event.ResultHash,
		event.BlockNumber,
	); !ok {
		logger.Infof(
			"FROST DKG result with hash [0x%x] for seed [0x%x] at block [%v] "+
				"has already been processed",
			event.ResultHash,
			event.Seed,
			event.BlockNumber,
		)
		return
	}

	valid, reason, err := frostChain.IsFrostDKGResultValid(event.Result)
	if err != nil {
		logger.Errorf(
			"failed to validate FROST DKG result [0x%x]: [%v]",
			event.ResultHash,
			err,
		)
		return
	}

	if !valid {
		logger.Warnf(
			"challenging invalid FROST DKG result [0x%x]: [%s]",
			event.ResultHash,
			reason,
		)
		if err := frostChain.ChallengeFrostDKGResult(event.Result); err != nil {
			logger.Errorf(
				"failed to challenge FROST DKG result [0x%x]: [%v]",
				event.ResultHash,
				err,
			)
		}
		return
	}

	memberIndexes, _, err := localFrostMembership(node, frostChain)
	if err != nil {
		logger.Errorf("failed to resolve local FROST DKG membership: [%v]", err)
		return
	}
	if len(memberIndexes) == 0 {
		logger.Infof(
			"FROST DKG result [0x%x] is valid; this operator is not in the "+
				"selected group and will not approve",
			event.ResultHash,
		)
		return
	}

	params, err := frostChain.FrostDKGParameters()
	if err != nil {
		logger.Errorf("failed to get FROST DKG parameters: [%v]", err)
		return
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

func frostDKGRecoveryStartBlock(
	node *node,
	frostChain FrostDKGChain,
) (uint64, error) {
	blockCounter, err := node.chain.BlockCounter()
	if err != nil {
		return 0, err
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return 0, err
	}

	params, err := frostChain.FrostDKGParameters()
	if err != nil {
		return 0, err
	}

	lookBackBlocks, err := frostDKGRecoveryLookBackBlocks(
		params,
		node.groupParameters,
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
