package tbtc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/keep-network/keep-core/pkg/protocol/announcer"
	"math/big"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/protocol/retry"
	"github.com/keep-network/keep-core/pkg/tecdsa/dkg"
	"golang.org/x/exp/slices"
)

const (
	// dkgAttemptAnnouncementDelayBlocks determines the duration of the
	// announcement phase delay that is preserved before starting the
	// announcement phase.
	dkgAttemptAnnouncementDelayBlocks = 1
	// dkgAttemptAnnouncementActiveBlocks determines the duration of the
	// announcement phase that is performed at the beginning of each DKG
	// attempt.
	dkgAttemptAnnouncementActiveBlocks = 10
	// dkgAttemptProtocolBlocks determines the maximum block duration of the
	// actual protocol computations.
	dkgAttemptMaximumProtocolBlocks = 200
	// dkgAttemptCoolDownBlocks determines the duration of the cool down
	// period that is preserved between subsequent DKG attempts.
	dkgAttemptCoolDownBlocks = 5
)

// dkgAttemptMaximumBlocks returns the maximum block duration of a single
// DKG attempt.
func dkgAttemptMaximumBlocks() uint {
	return dkgAttemptAnnouncementDelayBlocks +
		dkgAttemptAnnouncementActiveBlocks +
		dkgAttemptMaximumProtocolBlocks +
		dkgAttemptCoolDownBlocks
}

// dkgAnnouncer represents a component responsible for exchanging readiness
// announcements for the given DKG attempt for the given seed.
type dkgAnnouncer interface {
	Announce(
		ctx context.Context,
		memberIndex group.MemberIndex,
		sessionID string,
	) ([]group.MemberIndex, error)
}

// dkgRetryLoop is a struct that encapsulates the DKG retry logic.
type dkgRetryLoop struct {
	logger log.StandardLogger

	// seed is the original seed for DKG.
	// Used for the announcement. It never changes.
	seed *big.Int

	memberIndex       group.MemberIndex
	selectedOperators chain.Addresses

	groupParameters *GroupParameters

	announcer dkgAnnouncer

	attemptCounter    uint
	attemptStartBlock uint64
	// attemptSeed is a 8-byte seed obtained from the original seed.
	// Used for the random operator selection. It never changes.
	attemptSeed        int64
	attemptDelayBlocks uint64

	attemptsLimit uint
}

func newDkgRetryLoop(
	logger log.StandardLogger,
	seed *big.Int,
	initialStartBlock uint64,
	memberIndex group.MemberIndex,
	selectedOperators chain.Addresses,
	groupParameters *GroupParameters,
	announcer dkgAnnouncer,
	attemptsLimit uint,
) *dkgRetryLoop {
	// Compute the 8-byte seed needed for the random retry algorithm. We take
	// the first 8 bytes of the hash of the DKG seed. This allows us to not
	// care in this piece of the code about the length of the seed and how this
	// seed is proposed.
	seedSha256 := sha256.Sum256(seed.Bytes())
	attemptSeed := int64(binary.BigEndian.Uint64(seedSha256[:8]))

	return &dkgRetryLoop{
		logger:             logger,
		seed:               seed,
		memberIndex:        memberIndex,
		selectedOperators:  selectedOperators,
		groupParameters:    groupParameters,
		announcer:          announcer,
		attemptCounter:     0,
		attemptStartBlock:  initialStartBlock,
		attemptSeed:        attemptSeed,
		attemptDelayBlocks: 5,
		attemptsLimit:      attemptsLimit,
	}
}

// dkgAttemptParams represents parameters of a DKG attempt.
type dkgAttemptParams struct {
	number                 uint
	startBlock             uint64
	timeoutBlock           uint64
	excludedMembersIndexes []group.MemberIndex
}

// dkgAttemptFn represents a function performing a DKG attempt.
type dkgAttemptFn func(*dkgAttemptParams) (*dkg.Result, error)

// start begins the DKG retry loop using the given DKG attempt function.
// The retry loop terminates when the DKG result is produced or the ctx
// parameter is done, whatever comes first.
func (drl *dkgRetryLoop) start(
	ctx context.Context,
	waitForBlockFn waitForBlockFn,
	dkgAttemptFn dkgAttemptFn,
) (*dkg.Result, error) {
	for {
		drl.attemptCounter++

		if drl.attemptsLimit != 0 && drl.attemptCounter > drl.attemptsLimit {
			return nil, fmt.Errorf(
				"reached the limit of attempts [%v]",
				drl.attemptsLimit,
			)
		}

		// In order to start attempts >1 in the right place, we need to
		// determine how many blocks were taken by previous attempts. We assume
		// the worst case that each attempt failed at the end of the DKG
		// protocol.
		//
		// That said, we need to increment the previous attempt start
		// block by the number of blocks equal to the protocol duration and
		// by some additional delay blocks. We need a small cool down in
		// order to mitigate all corner cases where the actual attempt duration
		// was slightly longer than the expected duration determined by the
		// dkgAttemptMaximumProtocolBlocks constant.
		//
		// For example, the attempt may fail at the end of the protocol but the
		// error is returned after some time and more blocks than expected are
		// mined in the meantime.
		if drl.attemptCounter > 1 {
			drl.attemptStartBlock = drl.attemptStartBlock +
				uint64(dkgAttemptMaximumBlocks())
		}

		announcementStartBlock := drl.attemptStartBlock + dkgAttemptAnnouncementDelayBlocks
		err := waitForBlockFn(ctx, announcementStartBlock)
		if err != nil {
			return nil, fmt.Errorf(
				"failed waiting for announcement start block [%v] "+
					"for attempt [%v]: [%v]",
				announcementStartBlock,
				drl.attemptCounter,
				err,
			)
		}

		// Set up the announcement phase stop signal.
		announceCtx, cancelAnnounceCtx := context.WithCancel(ctx)
		announcementEndBlock := announcementStartBlock + dkgAttemptAnnouncementActiveBlocks
		go func() {
			defer cancelAnnounceCtx()

			if err := waitForBlockFn(ctx, announcementEndBlock); err != nil {
				drl.logger.Errorf(
					"[member:%v] failed waiting for announcement end "+
						"block [%v] for attempt [%v]: [%v]",
					drl.memberIndex,
					announcementEndBlock,
					drl.attemptCounter,
					err,
				)
			}
		}()

		drl.logger.Infof(
			"[member:%v] starting announcement phase for attempt [%v]",
			drl.memberIndex,
			drl.attemptCounter,
		)

		readyMembersIndexes, err := drl.announcer.Announce(
			announceCtx,
			drl.memberIndex,
			fmt.Sprintf("%v-%v", drl.seed, drl.attemptCounter),
		)
		cancelAnnounceCtx()
		if err != nil {
			drl.logger.Warnf(
				"[member:%v] announcement for attempt [%v] "+
					"failed: [%v]; starting next attempt",
				drl.memberIndex,
				drl.attemptCounter,
				err,
			)
			continue
		}

		unreadyMembersIndexes := announcer.UnreadyMembers(
			readyMembersIndexes,
			drl.groupParameters.GroupSize,
		)

		// Check the loop stop signal.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if len(readyMembersIndexes) >= drl.groupParameters.GroupQuorum {
			drl.logger.Infof(
				"[member:%v] completed announcement phase for attempt [%v] "+
					"with quorum of [%v] members ready to perform DKG; "+
					"following members are not ready: [%v]",
				drl.memberIndex,
				drl.attemptCounter,
				len(readyMembersIndexes),
				unreadyMembersIndexes,
			)
		} else {
			drl.logger.Warnf(
				"[member:%v] completed announcement phase for attempt [%v] "+
					"with non-quorum of [%v] members ready to perform DKG; "+
					"following members are not ready: [%v]; "+
					"moving to the next attempt",
				drl.memberIndex,
				drl.attemptCounter,
				len(readyMembersIndexes),
				unreadyMembersIndexes,
			)
			continue
		}

		excludedMembersIndexes, err := drl.performMembersSelection(
			readyMembersIndexes,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot select members for attempt [%v]: [%w]",
				drl.attemptCounter,
				err,
			)
		}

		attemptSkipped := slices.Contains(
			excludedMembersIndexes,
			drl.memberIndex,
		)

		timeoutBlock := announcementEndBlock + dkgAttemptMaximumProtocolBlocks

		var result *dkg.Result
		var attemptErr error

		if !attemptSkipped {
			result, attemptErr = dkgAttemptFn(&dkgAttemptParams{
				number:                 drl.attemptCounter,
				startBlock:             announcementEndBlock,
				timeoutBlock:           timeoutBlock,
				excludedMembersIndexes: excludedMembersIndexes,
			})
		} else {
			drl.logger.Infof(
				"[member:%v] attempt [%v] skipped",
				drl.memberIndex,
				drl.attemptCounter,
			)
		}

		if attemptSkipped || attemptErr != nil {
			continue
		}

		return result, nil
	}
}

// performMembersSelection runs the member selection process whose result
// is a list of members' indexes that should be excluded by the client
// for the given DKG attempt.
//
// The member selection process is done based on the list of ready members
// provided as the readyMembersIndexes argument. This list is used twice:
//
// First, the algorithm determining the qualified operators set uses the
// ready members list to build an input consisting of only active operators.
// This way we guarantee that the qualified operators set contains only
// ready and active operators that will actually take part in the DKG
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
func (drl *dkgRetryLoop) performMembersSelection(
	readyMembersIndexes []group.MemberIndex,
) ([]group.MemberIndex, error) {
	qualifiedOperatorsSet, err := drl.qualifiedOperatorsSet(readyMembersIndexes)
	if err != nil {
		return nil, fmt.Errorf("cannot get qualified operators: [%w]", err)
	}

	excludedMembersIndexes := make([]group.MemberIndex, 0)
	for i, operator := range drl.selectedOperators {
		memberIndex := group.MemberIndex(i + 1)

		included := qualifiedOperatorsSet[operator] &&
			slices.Contains(readyMembersIndexes, memberIndex)

		if !included {
			excludedMembersIndexes = append(
				excludedMembersIndexes,
				memberIndex,
			)
		}
	}

	return excludedMembersIndexes, nil
}

// qualifiedOperatorsSet returns a set of operators qualified to participate
// in the given DKG attempt. The set of qualified operators is taken from the
// set of active operators who announced readiness through their controlled DKG
// group members.
func (drl *dkgRetryLoop) qualifiedOperatorsSet(
	readyMembersIndexes []group.MemberIndex,
) (map[chain.Address]bool, error) {
	var readyOperators chain.Addresses
	for _, memberIndex := range readyMembersIndexes {
		readyOperators = append(
			readyOperators,
			drl.selectedOperators[memberIndex-1],
		)
	}

	// For the first attempt, just return the operators who announced readiness.
	// Otherwise, randomly exclude operators from the ready operators set.
	if drl.attemptCounter == 1 {
		return readyOperators.Set(), nil
	}

	// The retry algorithm expects that we count retries from 0. Since
	// the first invocation of the algorithm will be for `attemptCounter == 1`
	// we need to subtract one while determining the number of the given retry.
	retryCount := drl.attemptCounter - 1

	qualifiedOperators, err := retry.EvaluateRetryParticipantsForKeyGeneration(
		readyOperators,
		drl.attemptSeed,
		retryCount,
		uint(drl.groupParameters.GroupQuorum),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"random operator selection failed: [%w]",
			err,
		)
	}

	return chain.Addresses(qualifiedOperators).Set(), nil
}
