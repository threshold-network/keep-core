package tbtc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/protocol/announcer"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
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

// signingAttemptSeed computes a deterministic seed used for retry and
// coordinator selection for a given signed message.
func signingAttemptSeed(message *big.Int) int64 {
	// Compute the 8-byte seed needed for the random retry algorithm. We take
	// the first 8 bytes of the hash of the signed message. This allows us to
	// not care in this piece of the code about the length of the message and
	// how this message is proposed.
	messageSha256 := sha256.Sum256(message.Bytes())
	return int64(binary.BigEndian.Uint64(messageSha256[:8]))
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

	// roastSessionID is the STABLE ROAST session id (no attempt number) keying
	// the ROAST transition-record registry + participant-selector lookup across
	// this signing's attempts. Empty in deployments that do not drive ROAST.
	roastSessionID string

	signingGroupMemberIndex group.MemberIndex
	signingGroupOperators   chain.Addresses

	groupParameters *GroupParameters

	announcer signingAnnouncer

	attemptCounter    uint
	attemptStartBlock uint64
	attemptSeed       int64

	// roastAttemptNumber is the 0-based COMMITTED ROAST attempt index: it advances
	// only when an iteration reaches selection/observe (a committed attempt), NOT
	// on the block-timing/announcement/minority-readiness skips that tick
	// attemptCounter. The ROAST transition chain keys off it (observe context,
	// freshness, consume) so a skipped loop iteration does not break the
	// consecutive-transition chain across honest nodes (RFC-21 Phase 7.3 PR2b-1b).
	roastAttemptNumber uint

	doneCheck signingDoneCheckStrategy

	// participantSelector dispatches qualified-operator selection.
	// Default: legacy retry shuffle. Phase 7 may install a
	// ROAST-driven implementation behind the frost_roast_retry
	// build tag once AggregateBundle production is wired upstream.
	participantSelector signingParticipantSelector

	// transitionController, when non-nil, owns the session-scoped ROAST
	// transition machinery for this signer (RFC-21 Phase 7.3 PR2b-1b): observing
	// each attempt so the signer holds a handle to verify the attempt's
	// transition bundle and run NextAttempt for participant selection. nil in the
	// default build and in deployments that do not drive ROAST retry, in which
	// case the loop runs no transition steps.
	transitionController roastTransitionController

	// attemptOutcomeReporter, when non-nil, receives the terminal outcome
	// of every network-wide signing attempt this loop observes (RFC-21
	// Annex B implied-f liveness alerting). An outcome is reported when an
	// attempt reaches a terminal disposition: minority readiness after a
	// completed announcement, a failed protocol run, a failed done-check
	// exchange, or success. Mechanical iterations that never sample the
	// group - block-timing skips, local announcement errors, context
	// cancellation, and a members-selection error (which terminates the
	// whole loop, not the attempt) - are deliberately not reported, so
	// the rate feeding the Annex B sampling model is not diluted by
	// local noise.
	attemptOutcomeReporter func(success bool)
}

func newSigningRetryLoop(
	logger log.StandardLogger,
	message *big.Int,
	roastSessionID string,
	initialStartBlock uint64,
	signingGroupMemberIndex group.MemberIndex,
	signingGroupOperators chain.Addresses,
	groupParameters *GroupParameters,
	announcer signingAnnouncer,
	doneCheck signingDoneCheckStrategy,
) *signingRetryLoop {
	return &signingRetryLoop{
		logger:                  logger,
		message:                 message,
		roastSessionID:          roastSessionID,
		signingGroupMemberIndex: signingGroupMemberIndex,
		signingGroupOperators:   signingGroupOperators,
		groupParameters:         groupParameters,
		announcer:               announcer,
		attemptCounter:          0,
		attemptStartBlock:       initialStartBlock,
		attemptSeed:             signingAttemptSeed(message),
		doneCheck:               doneCheck,
		participantSelector:     defaultSigningParticipantSelector(),
	}
}

// setAttemptOutcomeReporter installs the attempt-outcome reporter. See the
// attemptOutcomeReporter field for the reporting contract.
func (srl *signingRetryLoop) setAttemptOutcomeReporter(
	reporter func(success bool),
) {
	srl.attemptOutcomeReporter = reporter
}

// setTransitionController installs the ROAST transition controller. A nil
// controller leaves the loop running no transition steps (the default).
func (srl *signingRetryLoop) setTransitionController(
	controller roastTransitionController,
) {
	srl.transitionController = controller
}

func (srl *signingRetryLoop) reportAttemptOutcome(success bool) {
	if srl.attemptOutcomeReporter != nil {
		srl.attemptOutcomeReporter(success)
	}
}

// signingAttemptParams represents parameters of a signing attempt.
type signingAttemptParams struct {
	// number is the attempt number the active signing keys its AttemptContext,
	// coordinator election, and session id off. Under active ROAST retry this is
	// the COMMITTED roast attempt number (roastAttemptNumber+1), so the active
	// signing context matches the observe/transition context the loop built for
	// the same committed attempt; otherwise it is the block-paced attemptCounter
	// (legacy, unchanged). RFC-21 Phase 7.3 PR2b-1b.
	number                 uint
	startBlock             uint64
	timeoutBlock           uint64
	excludedMembersIndexes []group.MemberIndex
	// transientlyParkedMembersIndexes are the members the prior ROAST transition
	// parked for THIS attempt only. The active signing stamps them onto its
	// AttemptContext so it is byte-identical to the observe context (which carries
	// the same parking); empty for the legacy path.
	transientlyParkedMembersIndexes []group.MemberIndex
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
			srl.reportAttemptOutcome(false)
			continue
		}

		// RFC-21 Phase 7.3 PR2b-1b: if this seat fell behind the group's committed
		// ROAST attempt chain -- its listener received a transition for an attempt
		// it never observed because it skipped a window peers committed -- selecting
		// now would produce a divergent included set (the fracture class). Fail
		// closed before selection; the outer layer retries the whole signing from a
		// fresh election. The check is deterministic per seat (a nil controller or
		// inactive ROAST is never lost-sync).
		if srl.transitionController != nil && srl.transitionController.HasLostSync() {
			return nil, fmt.Errorf(
				"cannot select members for attempt [%v]: lost ROAST sync "+
					"(received a transition for an unobserved attempt); fail closed",
				srl.attemptCounter,
			)
		}

		includedMembersIndexes, excludedMembersIndexes, transientlyParkedMembersIndexes, err := srl.performMembersSelection(
			readyMembersIndexes,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot select members for attempt [%v]: [%w]",
				srl.attemptCounter,
				err,
			)
		}

		// committedRoastAttemptNumber is this committed attempt's 0-based ROAST
		// index, captured BEFORE the advance below so both the observe context and
		// the active signing context key off the same value (the observe Attempt is
		// stamped Number = committedRoastAttemptNumber+1, so the active attempt must
		// use the same number to stay byte-identical).
		committedRoastAttemptNumber := srl.roastAttemptNumber

		// RFC-21 Phase 7.3 PR2b-1b: record a local observe binding for this
		// committed ROAST attempt BEFORE the skip branch, so every local seat --
		// including one excluded from this attempt -- holds a handle to verify the
		// attempt's transition bundle and run NextAttempt for the next attempt's
		// selection. The observe context carries the transiently-parked set, so a
		// one-attempt park is reinstated next time rather than made permanent.
		if srl.transitionController != nil {
			srl.transitionController.BeginObservedAttempt(
				committedRoastAttemptNumber,
				includedMembersIndexes,
				excludedMembersIndexes,
				transientlyParkedMembersIndexes,
			)
		}
		// This iteration reached selection/observe, so it COMMITTED to a ROAST
		// attempt: advance the committed ROAST attempt number (the block-timing,
		// announcement, and minority-readiness skips above did not). Every seat --
		// included or excluded -- advances identically, keeping the transition
		// chain consecutive across honest nodes.
		srl.roastAttemptNumber++

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

			// RFC-21 Phase 7.3 PR2b-1b: when ROAST retry is runtime-active, the
			// active signing attempt must key off the COMMITTED roast attempt index
			// (+ parking) so its AttemptContext is byte-identical to the
			// observe/transition context this seat built for the same committed
			// attempt above. Keyed off attemptCounter, the two would diverge after a
			// pre-selection skip or whenever a member is parked, binding signing
			// messages and transition bundles to different context hashes. ROAST
			// inactive keeps the block-paced attemptCounter (legacy, unchanged); the
			// gate is the deterministic, group-wide RoastRetryActive predicate.
			activeAttemptNumber := srl.attemptCounter
			if signing.RoastRetryActive() {
				activeAttemptNumber = committedRoastAttemptNumber + 1
			}

			result, endBlock, err := signingAttemptFn(&signingAttemptParams{
				number:                          activeAttemptNumber,
				startBlock:                      announcementEndBlock,
				timeoutBlock:                    timeoutBlock,
				excludedMembersIndexes:          excludedMembersIndexes,
				transientlyParkedMembersIndexes: transientlyParkedMembersIndexes,
			})
			if err != nil {
				srl.logger.Warnf(
					"[member:%v] failed attempt [%v]: [%v]; "+
						"starting next attempt",
					srl.signingGroupMemberIndex,
					srl.attemptCounter,
					err,
				)
				// RFC-21 Phase 7.3 PR2b-1b: this seat committed to and failed the
				// attempt, so drive the transition exchange (forced snapshot, and
				// the elected coordinator's aggregation) for the next attempt's
				// selection. Inert until the selector consumes records (C3).
				if srl.transitionController != nil {
					srl.transitionController.OnAttemptFailed(
						srl.attemptCounter,
						timeoutBlock,
					)
				}
				srl.reportAttemptOutcome(false)
				continue
			}

			// RFC-21 Phase 7.3 PR2b-1b: the protocol round succeeded locally (a valid
			// signature aggregated), so clear this seat's observe binding for the
			// attempt -- no failure transition may be synthesized or stored for a
			// succeeded attempt. If the done-check below then fails, the next attempt
			// finds no fresh transition and fails closed (the honest outcome) instead
			// of consuming a peer's failure transition for an attempt this seat won.
			if srl.transitionController != nil {
				srl.transitionController.OnAttemptSucceeded()
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
				// A failed done signal is a local fault, but this loop
				// abandons the attempt here, so from this node's sampler
				// the attempt did not complete; the baseline calibration
				// absorbs this as benign noise.
				srl.reportAttemptOutcome(false)
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
			srl.reportAttemptOutcome(false)
			continue
		}

		activityReport := &signingActivityReport{
			activeMembers:   readyMembersIndexes,
			inactiveMembers: unreadyMembersIndexes,
		}

		srl.reportAttemptOutcome(true)

		return &signingRetryLoopResult{
			result:              result,
			activityReport:      activityReport,
			latestEndBlock:      latestEndBlock,
			attemptTimeoutBlock: timeoutBlock,
		}, nil
	}
}

// performMembersSelection runs the member selection process for the given
// signing attempt, returning the included member indices (the members that
// participate), the excluded ones (everyone else), and the transiently-parked
// subset, each sorted ascending.
//
// Selection is dispatched through participantSelector (RFC-21 Phase 6.4). As of
// Phase 7.3 PR2b-1a it is MEMBER-LEVEL; PR2b-1b's ROAST implementation returns
// the transition's IncludedSet + TransientlyParked directly (keyed by the
// COMMITTED roastAttemptNumber, not the block-paced attemptCounter). The excluded
// set is the complement of the included set over the whole signing group, so the
// two partition [1, groupSize]; the parked set is the subset of the excluded set
// that the next attempt reinstates.
func (srl *signingRetryLoop) performMembersSelection(
	readyMembersIndexes []group.MemberIndex,
) ([]group.MemberIndex, []group.MemberIndex, []group.MemberIndex, error) {
	// The legacy retry algorithm counts retries from 0. The first invocation is
	// for attemptCounter == 1, so the legacy retry count is attemptCounter - 1.
	// The ROAST path instead keys off the committed roastAttemptNumber.
	retryCount := srl.attemptCounter - 1

	selection, err := srl.participantSelector.Select(
		readyMembersIndexes,
		srl.signingGroupOperators,
		srl.attemptSeed,
		retryCount,
		srl.roastAttemptNumber,
		uint(srl.groupParameters.HonestThreshold),
		srl.roastSessionID,
		srl.signingGroupMemberIndex,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(
			"participant selection failed: [%w]",
			err,
		)
	}

	excludedMembersIndexes := membersComplement(
		selection.includedMembersIndexes,
		len(srl.signingGroupOperators),
	)

	return selection.includedMembersIndexes,
		excludedMembersIndexes,
		selection.transientlyParkedMembersIndexes,
		nil
}

// membersComplement returns the member indices in [1, groupSize] that are NOT
// in the included set, sorted ascending. Together with the included set it
// partitions the whole signing group.
func membersComplement(
	includedMembersIndexes []group.MemberIndex,
	groupSize int,
) []group.MemberIndex {
	includedSet := make(map[group.MemberIndex]bool, len(includedMembersIndexes))
	for _, memberIndex := range includedMembersIndexes {
		includedSet[memberIndex] = true
	}

	excludedMembersIndexes := make([]group.MemberIndex, 0, groupSize)
	for i := 0; i < groupSize; i++ {
		memberIndex := group.MemberIndex(i + 1)
		if !includedSet[memberIndex] {
			excludedMembersIndexes = append(excludedMembersIndexes, memberIndex)
		}
	}

	return excludedMembersIndexes
}
