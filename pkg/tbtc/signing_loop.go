package tbtc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"math/rand"
	"sort"

	"github.com/keep-network/keep-core/pkg/protocol/announcer"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa/retry"
	"golang.org/x/exp/slices"
)

const (
	// signingAttemptAnnouncementDelayBlocks determines the duration of the
	// announcement phase delay that is preserved before starting the
	// announcement phase.
	signingAttemptAnnouncementDelayBlocks = 1
	// signingAttemptAnnouncementActiveBlocks determines the duration of the
	// announcement phase that is performed at the beginning of each signing
	// attempt.
	signingAttemptAnnouncementActiveBlocks = 5
	// signingAttemptProtocolBlocks determines the maximum block duration of the
	// actual protocol computations.
	signingAttemptMaximumProtocolBlocks = 30
	// signingAttemptCoolDownBlocks determines the duration of the cool down
	// period that is preserved between subsequent signing attempts.
	signingAttemptCoolDownBlocks = 5
)

// signingAttemptMaximumBlocks returns the maximum block duration of a single
// signing attempt.
func signingAttemptMaximumBlocks() uint {
	return signingAttemptAnnouncementDelayBlocks +
		signingAttemptAnnouncementActiveBlocks +
		signingAttemptMaximumProtocolBlocks +
		signingAttemptCoolDownBlocks
}

// signingAnnouncer represents a component responsible for exchanging readiness
// announcements for the given signing attempt of the given message.
type signingAnnouncer interface {
	Announce(
		ctx context.Context,
		memberIndex group.MemberIndex,
		sessionID string,
	) ([]group.MemberIndex, error)
}

// signingDoneCheckStrategy is a strategy that determines the way of signaling
// a successful signature calculation across all signing group members.
type signingDoneCheckStrategy interface {
	listen(
		ctx context.Context,
		message *big.Int,
		attemptNumber uint64,
		attemptTimeoutBlock uint64,
		attemptMembersIndexes []group.MemberIndex,
	)

	signalDone(
		ctx context.Context,
		memberIndex group.MemberIndex,
		message *big.Int,
		attemptNumber uint64,
		result *signing.Result,
		endBlock uint64,
	) error

	waitUntilAllDone(ctx context.Context) (*signing.Result, uint64, error)
}

// signingRetryLoop is a struct that encapsulates the signing retry logic.
type signingRetryLoop struct {
	logger log.StandardLogger

	message *big.Int

	signingGroupMemberIndex group.MemberIndex
	signingGroupOperators   chain.Addresses

	groupParameters *GroupParameters

	announcer signingAnnouncer

	attemptCounter    uint
	attemptStartBlock uint64
	attemptSeed       int64

	doneCheck signingDoneCheckStrategy
}

func newSigningRetryLoop(
	logger log.StandardLogger,
	message *big.Int,
	initialStartBlock uint64,
	signingGroupMemberIndex group.MemberIndex,
	signingGroupOperators chain.Addresses,
	groupParameters *GroupParameters,
	announcer signingAnnouncer,
	doneCheck signingDoneCheckStrategy,
) *signingRetryLoop {
	// Compute the 8-byte seed needed for the random retry algorithm. We take
	// the first 8 bytes of the hash of the signed message. This allows us to
	// not care in this piece of the code about the length of the message and
	// how this message is proposed.
	messageSha256 := sha256.Sum256(message.Bytes())
	attemptSeed := int64(binary.BigEndian.Uint64(messageSha256[:8]))

	return &signingRetryLoop{
		logger:                  logger,
		message:                 message,
		signingGroupMemberIndex: signingGroupMemberIndex,
		signingGroupOperators:   signingGroupOperators,
		groupParameters:         groupParameters,
		announcer:               announcer,
		attemptCounter:          0,
		attemptStartBlock:       initialStartBlock,
		attemptSeed:             attemptSeed,
		doneCheck:               doneCheck,
	}
}

// signingAttemptParams represents parameters of a signing attempt.
type signingAttemptParams struct {
	number                 uint
	startBlock             uint64
	timeoutBlock           uint64
	excludedMembersIndexes []group.MemberIndex
}

// signingAttemptFn represents a function performing a signing attempt.
type signingAttemptFn func(*signingAttemptParams) (*signing.Result, uint64, error)

// signingActivityReport holds information about the activity of the signing
// group members during the signing process.
type signingActivityReport struct {
	activeMembers   []group.MemberIndex
	inactiveMembers []group.MemberIndex
}

// signingRetryLoopResult represents the result of the signing retry loop.
type signingRetryLoopResult struct {
	// result is the outcome of the signing process.
	result *signing.Result
	// activityReport holds information about the activity of the signing
	// group members during the signing process.
	activityReport *signingActivityReport
	// latestEndBlock is the block at which the slowest signer of the successful
	// signing attempt completed signature computation. This block is also
	// the common end block accepted by all other members of the signing group.
	latestEndBlock uint64
	// attemptTimeoutBlock is the block at which the successful attempt times
	// out.
	attemptTimeoutBlock uint64
}

// start begins the signing retry loop using the given signing attempt function.
// The retry loop terminates when the signing result is produced or the ctx
// parameter is done, whatever comes first. The signing result is produced
// only if all signers who participated in signing confirmed they are done
// by sending a valid `signingDoneMessage` during the signing done check phase.
func (srl *signingRetryLoop) start(
	ctx context.Context,
	waitForBlockFn waitForBlockFn,
	getCurrentBlockFn getCurrentBlockFn,
	signingAttemptFn signingAttemptFn,
) (*signingRetryLoopResult, error) {
	for {
		srl.attemptCounter++

		// Check the loop stop signal.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// In order to start attempts >1 in the right place, we need to
		// determine how many blocks were taken by previous attempts. We assume
		// the worst case that each attempt failed at the end of the signing
		// protocol.
		//
		// That said, we need to increment the previous attempt start
		// block by the number of blocks equal to the protocol duration and
		// by some additional delay blocks. We need a small cool down in
		// order to mitigate all corner cases where the actual attempt duration
		// was slightly longer than the expected duration determined by the
		// signingAttemptMaximumProtocolBlocks constant.
		//
		// For example, the attempt may fail at the end of the protocol but the
		// error is returned after some time and more blocks than expected are
		// mined in the meantime.
		if srl.attemptCounter > 1 {
			srl.attemptStartBlock = srl.attemptStartBlock +
				uint64(signingAttemptMaximumBlocks())
		}

		srl.logger.Infof(
			"[member:%v] waiting for attempt [%v] start signal",
			srl.signingGroupMemberIndex,
			srl.attemptCounter,
		)

		announcementStartBlock := srl.attemptStartBlock + signingAttemptAnnouncementDelayBlocks
		announcementEndBlock := announcementStartBlock + signingAttemptAnnouncementActiveBlocks

		currentBlock, err := getCurrentBlockFn()
		if err != nil {
			srl.logger.Errorf(
				"[member:%v] failed to get the current block for attempt [%v]: "+
					"[%v]; starting next attempt",
				srl.signingGroupMemberIndex,
				srl.attemptCounter,
				err,
			)
			continue
		}

		if announcementEndBlock <= currentBlock {
			srl.logger.Infof(
				"[member:%v] skipping attempt [%v]; the current block is [%v] "+
					"and the end block [%v] for the announcement phase is in the past",
				srl.signingGroupMemberIndex,
				srl.attemptCounter,
				currentBlock,
				announcementEndBlock,
			)
			continue
		}

		err = waitForBlockFn(ctx, announcementStartBlock)
		if err != nil {
			srl.logger.Errorf(
				"[member:%v] failed waiting for announcement start "+
					"block [%v] for attempt [%v]: [%v]; starting next attempt",
				srl.signingGroupMemberIndex,
				announcementStartBlock,
				srl.attemptCounter,
				err,
			)
			continue
		}

		// Set up the announcement phase stop signal.
		announceCtx, _ := withCancelOnBlock(ctx, announcementEndBlock, waitForBlockFn)

		srl.logger.Infof(
			"[member:%v] starting announcement phase for attempt [%v]",
			srl.signingGroupMemberIndex,
			srl.attemptCounter,
		)

		readyMembersIndexes, err := srl.announcer.Announce(
			announceCtx,
			srl.signingGroupMemberIndex,
			fmt.Sprintf("%v-%v", srl.message, srl.attemptCounter),
		)
		if err != nil {
			srl.logger.Warnf(
				"[member:%v] announcement for attempt [%v] "+
					"failed: [%v]; starting next attempt",
				srl.signingGroupMemberIndex,
				srl.attemptCounter,
				err,
			)
			continue
		}

		unreadyMembersIndexes := announcer.UnreadyMembers(
			readyMembersIndexes,
			len(srl.signingGroupOperators),
		)

		// Check the loop stop signal again. The announcement took some time
		// and the context may be done now.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if len(readyMembersIndexes) >= srl.groupParameters.HonestThreshold {
			srl.logger.Infof(
				"[member:%v] completed announcement phase for attempt [%v] "+
					"with honest majority of [%v] members ready to sign; "+
					"following members are not ready: [%v]",
				srl.signingGroupMemberIndex,
				srl.attemptCounter,
				len(readyMembersIndexes),
				unreadyMembersIndexes,
			)
		} else {
			srl.logger.Warnf(
				"[member:%v] completed announcement phase for attempt [%v] "+
					"with minority of [%v] members ready to sign; "+
					"following members are not ready: [%v]; "+
					"moving to the next attempt",
				srl.signingGroupMemberIndex,
				srl.attemptCounter,
				len(readyMembersIndexes),
				unreadyMembersIndexes,
			)
			continue
		}

		excludedMembersIndexes, err := srl.performMembersSelection(
			readyMembersIndexes,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot select members for attempt [%v]: [%w]",
				srl.attemptCounter,
				err,
			)
		}

		includedMembersIndexes := make([]group.MemberIndex, 0)
		for i := range srl.signingGroupOperators {
			memberIndex := group.MemberIndex(i + 1)
			if !slices.Contains(excludedMembersIndexes, memberIndex) {
				includedMembersIndexes = append(includedMembersIndexes, memberIndex)
			}
		}

		attemptSkipped := slices.Contains(
			excludedMembersIndexes,
			srl.signingGroupMemberIndex,
		)

		timeoutBlock := announcementEndBlock + signingAttemptMaximumProtocolBlocks

		// doneCheckTimeoutCtx is active until the timeout even if the protocol
		// completed successfully earlier. This is needed to ensure all protocol
		// participants have a chance to receive signingDoneMessage.
		doneCheckTimeoutCtx, _ := withCancelOnBlock(ctx, timeoutBlock, waitForBlockFn)

		srl.doneCheck.listen(
			doneCheckTimeoutCtx,
			srl.message,
			uint64(srl.attemptCounter),
			timeoutBlock,
			includedMembersIndexes,
		)

		if !attemptSkipped {
			srl.logger.Infof(
				"[member:%v] eligible for attempt [%v]",
				srl.signingGroupMemberIndex,
				srl.attemptCounter,
			)

			result, endBlock, err := signingAttemptFn(&signingAttemptParams{
				number:                 srl.attemptCounter,
				startBlock:             announcementEndBlock,
				timeoutBlock:           timeoutBlock,
				excludedMembersIndexes: excludedMembersIndexes,
			})
			if err != nil {
				srl.logger.Warnf(
					"[member:%v] failed attempt [%v]: [%v]; "+
						"starting next attempt",
					srl.signingGroupMemberIndex,
					srl.attemptCounter,
					err,
				)
				continue
			}

			srl.logger.Infof(
				"[member:%v] exchanging signing done checks for attempt [%v]",
				srl.signingGroupMemberIndex,
				srl.attemptCounter,
			)

			err = srl.doneCheck.signalDone(
				doneCheckTimeoutCtx,
				srl.signingGroupMemberIndex,
				srl.message,
				uint64(srl.attemptCounter),
				result,
				endBlock,
			)
			if err != nil {
				srl.logger.Warnf(
					"[member:%v] cannot send signing done signal "+
						"for attempt [%v]: [%v]; starting next attempt",
					srl.signingGroupMemberIndex,
					srl.attemptCounter,
					err,
				)
				continue
			}
		} else {
			srl.logger.Infof(
				"[member:%v] not eligible for attempt [%v]; "+
					"listening for signing done checks",
				srl.signingGroupMemberIndex,
				srl.attemptCounter,
			)
		}

		result, latestEndBlock, err := srl.doneCheck.waitUntilAllDone(doneCheckTimeoutCtx)
		if err != nil {
			srl.logger.Warnf(
				"[member:%v] cannot wait for signing done "+
					"checks for attempt [%v]: [%v]; starting next attempt",
				srl.signingGroupMemberIndex,
				srl.attemptCounter,
				err,
			)
			continue
		}

		activityReport := &signingActivityReport{
			activeMembers:   readyMembersIndexes,
			inactiveMembers: unreadyMembersIndexes,
		}

		return &signingRetryLoopResult{
			result:              result,
			activityReport:      activityReport,
			latestEndBlock:      latestEndBlock,
			attemptTimeoutBlock: timeoutBlock,
		}, nil
	}
}

// performMembersSelection runs the member selection process whose result
// is a list of members' indexes that should be excluded by the client
// for the given signing attempt.
//
// The member selection process is done based on the list of ready members
// provided as the readyMembersIndexes argument. This list is used twice:
//
// First, the algorithm determining the qualified operators set uses the
// ready members list to build an input consisting of only active operators.
// This way we guarantee that the qualified operators set contains only
// ready and active operators that will actually take part in the signing
// attempt.
//
// Second, the ready members list is used to determine a list of excluded
// members. The excluded members list is built using the qualified operators
// set. The algorithm that determines the qualified operators set does not
// care about an exact mapping between operators and controlled members but
// relies on the members count solely. That means the information about
// readiness of specific members controlled by the given operators is not
// included in the resulting qualified operators set. In order to properly
// decide about inclusion or exclusion of specific members of a given
// qualified operator, we must take the ready members list into account.
func (srl *signingRetryLoop) performMembersSelection(
	readyMembersIndexes []group.MemberIndex,
) ([]group.MemberIndex, error) {
	qualifiedOperatorsSet, err := srl.qualifiedOperatorsSet(readyMembersIndexes)
	if err != nil {
		return nil, fmt.Errorf("cannot get qualified operators: [%w]", err)
	}

	// Exclude all members controlled by the operators that were not
	// qualified for the current attempt.
	return srl.excludedMembersIndexes(
		qualifiedOperatorsSet,
		readyMembersIndexes,
	), nil
}

// qualifiedOperatorsSet returns a set of operators qualified to participate
// in the given signing attempt. The set of qualified operators is taken
// from the set of active operators who announced readiness through
// their controlled signing group members.
func (srl *signingRetryLoop) qualifiedOperatorsSet(
	readyMembersIndexes []group.MemberIndex,
) (map[chain.Address]bool, error) {
	// The retry algorithm expects that we count retries from 0. Since
	// the first invocation of the algorithm will be for `attemptCounter == 1`
	// we need to subtract one while determining the number of the given retry.
	retryCount := srl.attemptCounter - 1

	var readySigningGroupOperators []chain.Address
	for _, memberIndex := range readyMembersIndexes {
		readySigningGroupOperators = append(
			readySigningGroupOperators,
			srl.signingGroupOperators[memberIndex-1],
		)
	}

	qualifiedOperators, err := retry.EvaluateRetryParticipantsForSigning(
		readySigningGroupOperators,
		srl.attemptSeed,
		retryCount,
		uint(srl.groupParameters.HonestThreshold),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"random operator selection failed: [%w]",
			err,
		)
	}

	return chain.Addresses(qualifiedOperators).Set(), nil
}

// excludedMembersIndexes returns a list of excluded members' indexes for
// the given qualified operators set.
func (srl *signingRetryLoop) excludedMembersIndexes(
	qualifiedOperatorsSet map[chain.Address]bool,
	readyMembersIndexes []group.MemberIndex,
) []group.MemberIndex {
	includedMembersIndexes := make([]group.MemberIndex, 0)
	excludedMembersIndexes := make([]group.MemberIndex, 0)
	for i, operator := range srl.signingGroupOperators {
		memberIndex := group.MemberIndex(i + 1)

		if qualifiedOperatorsSet[operator] &&
			slices.Contains(readyMembersIndexes, memberIndex) {
			includedMembersIndexes = append(
				includedMembersIndexes,
				memberIndex,
			)
		} else {
			excludedMembersIndexes = append(
				excludedMembersIndexes,
				memberIndex,
			)
		}
	}

	// Make sure we always use just the smallest required count of
	// signing members for performance reasons
	if len(includedMembersIndexes) > srl.groupParameters.HonestThreshold {
		// #nosec G404 (insecure random number source (rand))
		// Shuffling does not require secure randomness.
		rng := rand.New(rand.NewSource(
			srl.attemptSeed + int64(srl.attemptCounter),
		))
		// Sort in ascending order just in case.
		sort.Slice(includedMembersIndexes, func(i, j int) bool {
			return includedMembersIndexes[i] < includedMembersIndexes[j]
		})
		// Shuffle the included members slice to randomize the
		// selection of additionally excluded members.
		rng.Shuffle(len(includedMembersIndexes), func(i, j int) {
			includedMembersIndexes[i], includedMembersIndexes[j] =
				includedMembersIndexes[j], includedMembersIndexes[i]
		})
		// Get the surplus of included members and add them to
		// the excluded members list.
		excludedMembersIndexes = append(
			excludedMembersIndexes,
			includedMembersIndexes[srl.groupParameters.HonestThreshold:]...,
		)
		// Sort the resulting excluded members list in ascending order.
		sort.Slice(excludedMembersIndexes, func(i, j int) bool {
			return excludedMembersIndexes[i] < excludedMembersIndexes[j]
		})
	}

	return excludedMembersIndexes
}
