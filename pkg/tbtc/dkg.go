package tbtc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"golang.org/x/exp/maps"

	"go.uber.org/zap"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/generator"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/announcer"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa/dkg"
)

const (
	// dkgStartedConfirmationBlocks determines the block length of the
	// confirmation period that is preserved after a DKG start. Once the period
	// elapses, the DKG state is checked to confirm the protocol can be started.
	dkgStartedConfirmationBlocks = 20
	// dkgResultSubmissionDelayStep determines the delay step in blocks that
	// is used to calculate the submission delay period that should be respected
	// by the given member to avoid all members submitting the same DKG result
	// at the same time.
	dkgResultSubmissionDelayStepBlocks = 3
	// dkgResultApprovalDelayStepBlocks determines the delay step in blocks
	// that is used to calculate the approval delay period that should be
	// respected by the given member to avoid all members approving the same
	// DKG result at the same time.
	dkgResultApprovalDelayStepBlocks = 15
	// dkgResultChallengeConfirmationBlocks determines the block length of
	// the confirmation period that is preserved after a DKG result challenge
	// submission. Once the period elapses, the DKG state is checked to confirm
	// the challenge was accepted successfully.
	dkgResultChallengeConfirmationBlocks = 20
	// dkgAttemptsLimit determines the maximum number of attempts to execute
	// the DKG protocol. If the limit is reached, the protocol execution is
	// aborted.
	dkgAttemptsLimit = 1
)

// dkgExecutor is a component responsible for the full execution of ECDSA
// Distributed Key Generation: determining members selected to the signing
// group, executing off-chain protocol, and publishing the result to the chain.
type dkgExecutor struct {
	groupParameters *GroupParameters

	operatorIDFn    func() (chain.OperatorID, error)
	operatorAddress chain.Address

	chain          Chain
	netProvider    net.Provider
	walletRegistry *walletRegistry
	protocolLatch  *generator.ProtocolLatch

	// waitForBlockFn is a function used to wait for the given block.
	waitForBlockFn waitForBlockFn

	tecdsaExecutor *dkg.Executor

	// metricsRecorder is optional and used for recording performance metrics
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}
}

// newDkgExecutor creates a new instance of dkgExecutor struct. There should
// be only one instance of dkgExecutor.
func newDkgExecutor(
	groupParameters *GroupParameters,
	operatorIDFn func() (chain.OperatorID, error),
	operatorAddress chain.Address,
	chain Chain,
	netProvider net.Provider,
	walletRegistry *walletRegistry,
	protocolLatch *generator.ProtocolLatch,
	config Config,
	workPersistence persistence.BasicHandle,
	scheduler *generator.Scheduler,
	waitForBlockFn waitForBlockFn,
) *dkgExecutor {
	tecdsaExecutor := dkg.NewExecutor(
		logger,
		scheduler,
		workPersistence,
		config.PreParamsPoolSize,
		config.PreParamsGenerationTimeout,
		config.PreParamsGenerationDelay,
		config.PreParamsGenerationConcurrency,
		config.KeyGenerationConcurrency,
	)

	return &dkgExecutor{
		groupParameters: groupParameters,
		operatorIDFn:    operatorIDFn,
		operatorAddress: operatorAddress,
		chain:           chain,
		netProvider:     netProvider,
		walletRegistry:  walletRegistry,
		protocolLatch:   protocolLatch,
		tecdsaExecutor:  tecdsaExecutor,
		waitForBlockFn:  waitForBlockFn,
	}
}

// setMetricsRecorder sets the metrics recorder for the DKG executor.
func (de *dkgExecutor) setMetricsRecorder(recorder interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	de.metricsRecorder = recorder
}

// preParamsCount returns the current count of the ECDSA DKG pre-parameters.
func (de *dkgExecutor) preParamsCount() int {
	return de.tecdsaExecutor.PreParamsCount()
}

// executeDkgIfEligible is the main function of dkgExecutor. It performs the
// full execution of ECDSA Distributed Key Generation: determining members
// selected to the signing group, executing off-chain protocol, and publishing
// the result to the chain. The execution can be delayed by an arbitrary number
// of blocks using the delayBlocks argument. This allows confirming the state
// on-chain - e.g. wait for the required number of confirming blocks - before
// executing the off-chain action.
func (de *dkgExecutor) executeDkgIfEligible(
	seed *big.Int,
	startBlock uint64,
	delayBlocks uint64,
) {
	dkgLogger := logger.With(
		zap.String("seed", fmt.Sprintf("0x%x", seed)),
	)

	dkgLogger.Info("checking eligibility for DKG")
	memberIndexes, groupSelectionResult, err := de.checkEligibility(
		dkgLogger,
	)
	if err != nil {
		dkgLogger.Errorf("could not check eligibility for DKG: [%v]", err)
		return
	}

	if membersCount := len(memberIndexes); membersCount > 0 {
		if preParamsCount := de.tecdsaExecutor.PreParamsCount(); membersCount > preParamsCount {
			dkgLogger.Infof(
				"cannot join DKG as pre-parameters pool size is "+
					"too small; [%v] pre-parameters are required but "+
					"only [%v] available",
				membersCount,
				preParamsCount,
			)
			return
		}

		dkgLogger.Infof(
			"joining DKG and controlling [%v] group members",
			membersCount,
		)

		if de.metricsRecorder != nil {
			de.metricsRecorder.IncrementCounter("dkg_joined_total", float64(membersCount))
		}

		de.generateSigningGroup(
			dkgLogger,
			seed,
			memberIndexes,
			groupSelectionResult,
			startBlock,
			delayBlocks,
		)
	} else {
		dkgLogger.Infof("not eligible for DKG")
	}
}

// checkEligibility performs on-chain group selection and returns two pieces
// of information:
//   - Indexes of members selected to the signing group and controlled by this
//     operator. The indexes are in range [1, `groupSize`]. The slice is nil if
//     none of the selected signing group members is controlled by this operator.
//   - Group selection result holding chain.OperatorID and chain.Address for
//     operators selected to the signing group. There are always `groupSize`
//     selected operators.
func (de *dkgExecutor) checkEligibility(
	dkgLogger log.StandardLogger,
) ([]uint8, *GroupSelectionResult, error) {
	groupSelectionResult, err := de.chain.SelectGroup()
	if err != nil {
		return nil, nil, fmt.Errorf("selecting group not possible: [%v]", err)
	}

	dkgLogger.Infof(
		"selected group members (seats) for DKG: [%s]",
		groupSelectionResult.OperatorsAddresses,
	)

	dkgLogger.Infof(
		"distinct operators participating in DKG: [%s]",
		maps.Keys(groupSelectionResult.OperatorsAddresses.Set()),
	)

	if len(groupSelectionResult.OperatorsAddresses) > de.groupParameters.GroupSize {
		return nil, nil, fmt.Errorf(
			"group size larger than supported: [%v]",
			len(groupSelectionResult.OperatorsAddresses),
		)
	}

	indexes := make([]uint8, 0)
	for index, operator := range groupSelectionResult.OperatorsAddresses {
		// See if we are amongst those chosen
		if operator == de.operatorAddress {
			// The group member index should be in range [1, groupSize] so we
			// need to add 1.
			indexes = append(indexes, uint8(index)+1)
		}
	}

	return indexes, groupSelectionResult, nil
}

// setupBroadcastChannel creates and initializes broadcast channel for the
// current DKG execution. It is a temporary channel named after the seed and
// the protocol name.
func (de *dkgExecutor) setupBroadcastChannel(
	seed *big.Int,
	membershipValidator *group.MembershipValidator,
) (net.BroadcastChannel, error) {
	// Create temporary broadcast channel name for DKG using the
	// group selection seed with the protocol name as prefix.
	channelName := fmt.Sprintf("%s-%s", ProtocolName, seed.Text(16))

	broadcastChannel, err := de.netProvider.BroadcastChannelFor(channelName)
	if err != nil {
		return nil, fmt.Errorf("failed to get broadcast channel: [%v]", err)
	}

	dkg.RegisterUnmarshallers(broadcastChannel)
	announcer.RegisterUnmarshaller(broadcastChannel)

	err = broadcastChannel.SetFilter(membershipValidator.IsInGroup)
	if err != nil {
		return nil, fmt.Errorf(
			"could not set filter for channel [%v]: [%v]",
			broadcastChannel.Name(),
			err,
		)
	}

	return broadcastChannel, nil
}

// generateSigningGroup executes off-chain protocol for each member controlled
// by the current operator and upon successful execution of the protocol
// publishes the result to the chain. The execution can be delayed by an
// arbitrary number of blocks using the delayBlocks argument. This allows
// confirming the state on-chain - e.g. wait for the required number of
// confirming blocks - before executing the off-chain action. Note that the
// startBlock represents the block at which DKG started on-chain. This is
// important for the result submission.
func (de *dkgExecutor) generateSigningGroup(
	dkgLogger *zap.SugaredLogger,
	seed *big.Int,
	memberIndexes []uint8,
	groupSelectionResult *GroupSelectionResult,
	startBlock uint64,
	delayBlocks uint64,
) {
	membershipValidator := group.NewMembershipValidator(
		dkgLogger,
		groupSelectionResult.OperatorsAddresses,
		de.chain.Signing(),
	)

	broadcastChannel, err := de.setupBroadcastChannel(seed, membershipValidator)
	if err != nil {
		dkgLogger.Errorf("could not set up a broadcast channel: [%v]", err)
		return
	}

	dkgParameters, err := de.chain.DKGParameters()
	if err != nil {
		dkgLogger.Errorf("cannot get DKG parameters: [%v]", err)
		return
	}

	dkgTimeoutBlock := startBlock + dkgParameters.SubmissionTimeoutBlocks

	for _, index := range memberIndexes {
		// Capture the member index for the goroutine.
		memberIndex := index

		go func() {
			dkgStartTime := time.Now()
			de.protocolLatch.Lock()
			defer de.protocolLatch.Unlock()

			ctx, cancelCtx := withCancelOnBlock(
				context.Background(),
				dkgTimeoutBlock,
				de.waitForBlockFn,
			)
			defer cancelCtx()

			// TODO: This subscription has to be updated once we implement
			//       re-submitting DKG result to the chain after a challenge.
			//       See https://github.com/keep-network/keep-core/issues/3450
			subscription := de.chain.OnDKGResultSubmitted(
				func(event *DKGResultSubmittedEvent) {
					defer cancelCtx()

					dkgLogger.Infof(
						"[member:%v] DKG result with group public "+
							"key [0x%x] and result hash [0x%x] submitted "+
							"at block [%v] by member [%v]",
						memberIndex,
						event.Result.GroupPublicKey,
						event.ResultHash,
						event.BlockNumber,
						event.Result.SubmitterMemberIndex,
					)
				})
			defer subscription.Unsubscribe()

			announcer := announcer.New(
				fmt.Sprintf("%v-%v", ProtocolName, "dkg"),
				broadcastChannel,
				membershipValidator,
			)

			retryLoop := newDkgRetryLoop(
				dkgLogger,
				seed,
				startBlock+delayBlocks,
				memberIndex,
				groupSelectionResult.OperatorsAddresses,
				de.groupParameters,
				announcer,
				dkgAttemptsLimit,
			)

			result, err := retryLoop.start(
				ctx,
				de.waitForBlockFn,
				func(attempt *dkgAttemptParams) (*dkg.Result, error) {
					dkgAttemptLogger := dkgLogger.With(
						zap.Uint("attempt", attempt.number),
						zap.Uint64("attemptStartBlock", attempt.startBlock),
						zap.Uint64("attemptTimeoutBlock", attempt.timeoutBlock),
					)

					dkgAttemptLogger.Infof(
						"[member:%v] scheduled dkg attempt "+
							"with [%v] group members (excluded: [%v])",
						memberIndex,
						de.groupParameters.GroupSize-len(attempt.excludedMembersIndexes),
						attempt.excludedMembersIndexes,
					)

					// Set up the attempt timeout signal.
					attemptCtx, _ := withCancelOnBlock(
						ctx,
						attempt.timeoutBlock,
						de.waitForBlockFn,
					)

					// sessionID must be different for each attempt.
					sessionID := fmt.Sprintf(
						"%v-%v",
						seed.Text(16),
						attempt.number,
					)

					result, err := de.tecdsaExecutor.Execute(
						attemptCtx,
						dkgAttemptLogger,
						seed,
						sessionID,
						memberIndex,
						de.groupParameters.GroupSize,
						de.groupParameters.DishonestThreshold(),
						attempt.excludedMembersIndexes,
						broadcastChannel,
						membershipValidator,
					)
					if err != nil {
						dkgAttemptLogger.Errorf(
							"[member:%v] dkg attempt failed: [%v]",
							memberIndex,
							err,
						)

						return nil, err
					}

					return result, nil
				},
			)
			if err != nil {
				if de.metricsRecorder != nil {
					de.metricsRecorder.IncrementCounter("dkg_failed_total", 1)
					de.metricsRecorder.RecordDuration("dkg_duration_seconds", time.Since(dkgStartTime))
				}
				if errors.Is(err, context.Canceled) {
					dkgLogger.Infof(
						"[member:%v] DKG is no longer awaiting the result; "+
							"aborting DKG protocol execution",
						memberIndex,
					)
					return
				}

				dkgLogger.Errorf(
					"[member:%v] failed to execute DKG: [%v]",
					memberIndex,
					err,
				)
				return
			}

			signer, err := de.registerSigner(
				result,
				memberIndex,
				groupSelectionResult.OperatorsAddresses,
			)
			if err != nil {
				dkgLogger.Errorf(
					"[member:%v] failed to register signing group member: [%v]",
					memberIndex,
					err,
				)
			}

			dkgLogger.Infof("registered %s", signer)

			// Record successful DKG completion
			if de.metricsRecorder != nil {
				de.metricsRecorder.RecordDuration("dkg_duration_seconds", time.Since(dkgStartTime))
			}

			err = de.publishDkgResult(
				ctx,
				dkgLogger,
				seed,
				memberIndex,
				broadcastChannel,
				membershipValidator,
				result,
				groupSelectionResult,
				startBlock,
			)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					dkgLogger.Infof(
						"[member:%v] DKG is no longer awaiting the result; "+
							"aborting DKG result publication",
						memberIndex,
					)
					return
				}

				dkgLogger.Errorf(
					"[member:%v] DKG result publication failed [%v]",
					memberIndex,
					err,
				)
				return
			}
		}()
	}
}

// registerSigner determines the final signing group shape and persists the
// generated signer with a unique key share. Note that the final group members
// may differ from the ones returned by the sortition pool if there was any
// misbehavior or inactivities during the key generation.
func (de *dkgExecutor) registerSigner(
	result *dkg.Result,
	memberIndex group.MemberIndex,
	selectedSigningGroupOperators chain.Addresses,
) (*signer, error) {
	// Final signing group may differ from the original DKG
	// group outputted by the sortition protocol. One need to
	// determine the final signing group based on the selected
	// group members who behaved correctly during DKG protocol.
	operatingMemberIndexes := result.Group.OperatingMemberIndexes()
	finalSigningGroupOperators, finalSigningGroupMembersIndexes, err :=
		finalSigningGroup(
			selectedSigningGroupOperators,
			operatingMemberIndexes,
			de.groupParameters,
		)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve final signing group members")
	}

	// Just like the final and original group may differ, the
	// member index used during the DKG protocol may differ
	// from the final signing group member index as well.
	// We need to remap it.
	finalSigningGroupMemberIndex, ok :=
		finalSigningGroupMembersIndexes[memberIndex]
	if !ok {
		return nil, fmt.Errorf("failed to resolve final signing " +
			"group member index",
		)
	}

	signer := newSigner(
		result.PrivateKeyShare.PublicKey(),
		finalSigningGroupOperators,
		finalSigningGroupMemberIndex,
		result.PrivateKeyShare,
	)

	err = de.walletRegistry.registerSigner(signer)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to register %s: [%v]",
			signer,
			err,
		)
	}

	return signer, nil
}

// publishDkgResult performs the DKG result publication process.
func (de *dkgExecutor) publishDkgResult(
	ctx context.Context,
	dkgLogger log.StandardLogger,
	seed *big.Int,
	memberIndex group.MemberIndex,
	broadcastChannel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	dkgResult *dkg.Result,
	groupSelectionResult *GroupSelectionResult,
	startBlock uint64,
) error {
	return dkg.Publish(
		ctx,
		dkgLogger,
		seed.Text(16),
		memberIndex,
		broadcastChannel,
		membershipValidator,
		newDkgResultSigner(de.chain, startBlock),
		newDkgResultSubmitter(
			dkgLogger,
			de.chain,
			de.groupParameters,
			groupSelectionResult,
			de.waitForBlockFn,
		),
		dkgResult,
	)
}

// executeDkgValidation performs the submitted DKG result validation process.
// If the result is not valid, this function submits an on-chain result
// challenge. If the result is valid and the given node was involved in the DKG,
// this function schedules an on-chain approve that is submitted once the
// challenge period elapses.
func (de *dkgExecutor) executeDkgValidation(
	seed *big.Int,
	submissionBlock uint64,
	result *DKGChainResult,
	resultHash [32]byte,
) {
	dkgLogger := logger.With(
		zap.String("seed", fmt.Sprintf("0x%x", seed)),
		zap.String("groupPublicKey", fmt.Sprintf("0x%x", result.GroupPublicKey)),
		zap.String("resultHash", fmt.Sprintf("0x%x", resultHash)),
	)

	dkgLogger.Infof("starting DKG result validation")

	if de.metricsRecorder != nil {
		de.metricsRecorder.IncrementCounter("dkg_validation_total", 1)
	}

	isValid, err := de.chain.IsDKGResultValid(result)
	if err != nil {
		dkgLogger.Errorf("cannot validate DKG result: [%v]", err)
		return
	}

	if !isValid {
		dkgLogger.Infof("DKG result is invalid")

		i := uint64(0)

		// Challenges are done along with DKG state confirmations. This is
		// needed to handle chain reorgs that may wipe out the block holding
		// the challenge transaction. The state check done upon the confirmation
		// block makes sure the submitted challenge changed the DKG state
		// as expected. If the DKG state was not changed, the challenge is
		// re-submitted.
		for {
			i++

			err = de.chain.ChallengeDKGResult(result)
			if err != nil {
				dkgLogger.Errorf(
					"cannot challenge invalid DKG result: [%v]",
					err,
				)
				return
			}

			if de.metricsRecorder != nil {
				de.metricsRecorder.IncrementCounter("dkg_challenges_submitted_total", 1)
			}

			confirmationBlock := submissionBlock +
				(i * dkgResultChallengeConfirmationBlocks)

			dkgLogger.Infof(
				"challenging invalid DKG result; waiting for "+
					"block [%v] to confirm DKG state",
				confirmationBlock,
			)

			err := de.waitForBlockFn(context.Background(), confirmationBlock)
			if err != nil {
				dkgLogger.Errorf(
					"error while waiting for challenge confirmation: [%v]",
					err,
				)
				return
			}

			state, err := de.chain.GetDKGState()
			if err != nil {
				dkgLogger.Errorf("cannot check DKG state: [%v]", err)
				return
			}

			if state != Challenge {
				dkgLogger.Infof(
					"invalid DKG result challenged successfully",
				)
				return
			}

			dkgLogger.Infof(
				"invalid DKG result still not challenged; retrying",
			)
		}
	}

	dkgLogger.Infof("DKG result is valid")

	operatorID, err := de.operatorIDFn()
	if err != nil {
		dkgLogger.Errorf("cannot get node's operator ID: [%v]", err)
		return
	}

	// Determine the member indexes controlled by this node's operator.
	memberIndexes := make([]group.MemberIndex, 0)
	for index, memberOperatorID := range result.Members {
		if memberOperatorID == operatorID {
			// The group member index should be in range [1, groupSize] so we
			// need to add 1.
			memberIndexes = append(memberIndexes, group.MemberIndex(index+1))
		}
	}

	if len(memberIndexes) == 0 {
		dkgLogger.Infof(
			"not eligible for DKG result approval; my operator "+
				"ID [%v] is not among DKG participants [%v]",
			operatorID,
			result.Members,
		)
		return
	}

	dkgLogger.Infof("scheduling DKG result approval")

	parameters, err := de.chain.DKGParameters()
	if err != nil {
		dkgLogger.Errorf("cannot get current DKG parameters: [%v]", err)
		return
	}

	// The challenge period starts at the result submission block and lasts
	// for challengePeriodBlocks.
	challengePeriodEndBlock := submissionBlock + parameters.ChallengePeriodBlocks
	// The approval is possible one block after the challenge period end.
	// The result submitter has precedence for approvePrecedencePeriodBlocks.
	approvePrecedencePeriodStartBlock := challengePeriodEndBlock + 1
	// Everyone else can approve once the precedence period ends.
	approvePeriodStartBlock := approvePrecedencePeriodStartBlock +
		parameters.ApprovePrecedencePeriodBlocks

	for _, currentMemberIndex := range memberIndexes {
		go func(memberIndex group.MemberIndex) {
			var approveBlock uint64

			if memberIndex == result.SubmitterMemberIndex {
				// The submitter can approve earlier, during the precedence
				// period.
				approveBlock = approvePrecedencePeriodStartBlock
			} else {
				// Everyone else must approve after the precedence period ends.
				// Each member preserves a delay according to their index
				// to avoid simultaneous approval.
				delayBlocks := uint64(memberIndex-1) * dkgResultApprovalDelayStepBlocks
				approveBlock = approvePeriodStartBlock + delayBlocks
			}

			dkgLogger.Infof(
				"[member:%v] waiting for block [%v] to approve DKG result",
				memberIndex,
				approveBlock,
			)

			ctx, cancelCtx := context.WithCancel(context.Background())
			defer cancelCtx()

			subscription := de.chain.OnDKGResultApproved(
				func(event *DKGResultApprovedEvent) {
					cancelCtx()
				},
			)
			defer subscription.Unsubscribe()

			err := de.waitForBlockFn(ctx, approveBlock)
			if err != nil {
				dkgLogger.Errorf(
					"[member:%v] error while waiting for DKG result "+
						"approve block: [%v]",
					memberIndex,
					err,
				)
				return
			}

			// If the context got cancelled that means the result was approved
			// by someone else.
			if ctx.Err() != nil {
				dkgLogger.Infof(
					"[member:%v] DKG result approved by someone else",
					memberIndex,
				)
				return
			}

			err = de.chain.ApproveDKGResult(result)
			if err != nil {
				dkgLogger.Errorf(
					"[member:%v] cannot approve DKG result: [%v]",
					memberIndex,
					err,
				)
				return
			}

			if de.metricsRecorder != nil {
				de.metricsRecorder.IncrementCounter("dkg_approvals_submitted_total", 1)
			}

			dkgLogger.Infof("[member:%v] approving DKG result", memberIndex)
		}(currentMemberIndex)
	}
}

// finalSigningGroup takes three parameters:
//   - selectedOperators: Contains addresses of all selected operators. Slice
//     length equals to the groupSize. Each element with index N corresponds
//     to the group member with ID N+1.
//   - operatingMembersIndexes: Contains group members indexes that were neither
//     disqualified nor marked as inactive. Slice length is lesser than or equal
//     to the groupSize.
//   - chainConfig: The tBTC chain's configuration
//
// Using those parameters, this function transforms the selectedOperators
// slice into another slice that contains addresses of all operators
// that were neither disqualified nor marked as inactive. This way, the
// resulting slice has only addresses of properly operating operators
// who form the resulting group.
//
// Apart from that, this function returns a map that holds the final signing
// group members indexes that should be used by particular members who behaved
// correctly during the DKG protocol execution. The key of this map is the
// member index used during DKG protocol and the value is the new member
// index that should be used in the context of the final signing group.
//
// Example:
// selectedOperators: [0xAA, 0xBB, 0xCC, 0xDD, 0xEE]
// operatingMembersIndexes: [5, 1, 3]
// finalOperators: [0xAA, 0xCC, 0xEE]
// finalMembersIndexes: [1:1, 3:2, 5:3]
//
// Please see docs of IdentityConverter from pkg/tecdsa/common for more
// information about shifting indexes.
func finalSigningGroup(
	selectedOperators []chain.Address,
	operatingMembersIndexes []group.MemberIndex,
	groupParameters *GroupParameters,
) (
	[]chain.Address,
	map[group.MemberIndex]group.MemberIndex,
	error,
) {
	if len(selectedOperators) != groupParameters.GroupSize ||
		len(operatingMembersIndexes) < groupParameters.GroupQuorum {
		return nil, nil, fmt.Errorf("invalid input parameters")
	}

	sort.Slice(operatingMembersIndexes, func(i, j int) bool {
		return operatingMembersIndexes[i] < operatingMembersIndexes[j]
	})

	finalOperators := make(
		[]chain.Address,
		len(operatingMembersIndexes),
	)
	finalMembersIndexes := make(
		map[group.MemberIndex]group.MemberIndex,
		len(operatingMembersIndexes),
	)

	for i, operatingMemberID := range operatingMembersIndexes {
		finalOperators[i] = selectedOperators[operatingMemberID-1]
		finalMembersIndexes[operatingMemberID] = group.MemberIndex(i + 1)
	}

	return finalOperators, finalMembersIndexes, nil
}
