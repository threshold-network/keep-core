package entry

import (
	"fmt"
	"math/big"

	"github.com/ipfs/go-log/v2"

	beaconchain "github.com/keep-network/keep-core/pkg/beacon/chain"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type relayEntrySubmitter struct {
	logger       log.StandardLogger
	chain        beaconchain.Interface
	blockCounter chain.BlockCounter

	index group.MemberIndex
}

// submitRelayEntry submits the provided relay entry data to the chain.
// Group members tries to submit in the order specified by their indexes.
// Group member with index 1 tries to submit as the first one, group member 2
// tries to submit after a few blocks if member 1 did not submit and so on.
// Relay entry submit process starts at block height defined by startBlockheight
// parameter.
func (res *relayEntrySubmitter) submitRelayEntry(
	newEntry []byte,
	groupPublicKey []byte,
	startBlockHeight uint64,
	relayEntrySubmittedChannel <-chan uint64,
	relayEntryTimeoutChannel <-chan uint64,
) error {
	config := res.chain.GetConfig()

	// TODO: we should eventually check if entry has been already submitted
	// but we may skip this check for V1.

	// Wait until the current member is eligible to submit the entry.
	eligibleToSubmitWaiter, err := res.waitForSubmissionEligibility(
		newEntry,
		startBlockHeight,
		config.GroupSize,
		config.ResultPublicationBlockStep,
	)
	if err != nil {
		return fmt.Errorf("wait for eligibility failure: [%v]", err)
	}

	for {
		select {
		case blockNumber := <-eligibleToSubmitWaiter:
			res.logger.Infof(
				"[member:%v] submitting relay entry [0x%x] on "+
					"behalf of group [0x%x] at block [%v]",
				res.index,
				newEntry,
				groupPublicKey,
				blockNumber,
			)

			err := res.chain.SubmitRelayEntry(newEntry)
			if err != nil {
				isEntryInProgress, statusCheckErr := res.chain.IsEntryInProgress()
				if statusCheckErr != nil {
					res.logger.Errorf(
						"[member:%v] could not check entry status "+
							"after relay entry submission error: [%v]; "+
							"original error will be returned",
						res.index,
						statusCheckErr,
					)
					return err
				}

				// Check if we failed because someone else submitted in the
				// meantime or because something wrong happened with
				// our transaction.
				if !isEntryInProgress {
					res.logger.Infof(
						"[member:%v] relay entry already submitted",
						res.index,
					)
					return nil
				}

				res.logger.Errorf(
					"[member:%v] could not submit relay entry: [%v]",
					res.index,
					err,
				)
				return err
			}

			res.logger.Infof(
				"[member:%v] successfully submitted relay entry "+
					"transaction to the mempool at block [%v]",
				res.index,
				blockNumber,
			)

			// Relay entry submission is fire and forget. Submitting member
			// should not quit the submitter loop after firing the submission
			// but is still monitoring for relay entry submission confirmation
			// or timeout
		case blockNumber := <-relayEntrySubmittedChannel:
			res.logger.Infof(
				"[member:%v] leaving submitter; "+
					"relay entry submitted at block [%v]",
				res.index,
				blockNumber,
			)
			return nil
		case blockNumber := <-relayEntryTimeoutChannel:
			return fmt.Errorf(
				"relay entry timed out at block [%v]",
				blockNumber,
			)
		}
	}
}

// waitForSubmissionEligibility waits until the current member is eligible to
// submit entry to the blockchain. First member is eligible to submit straight
// away, each following member is eligible after pre-defined block step.
func (res *relayEntrySubmitter) waitForSubmissionEligibility(
	entry []byte,
	startBlockHeight uint64,
	groupSize int,
	blockStep uint64,
) (<-chan uint64, error) {
	if groupSize <= 0 {
		return nil, fmt.Errorf("invalid group size: [%v]", groupSize)
	}

	// First submitter index is calculated as entry % groupSize and gives
	// an index from range [0, groupSize-1].
	firstSubmitterMemberIndex := new(big.Int).Mod(
		new(big.Int).SetBytes(entry),
		big.NewInt(int64(groupSize)),
	).Uint64()

	submissionQueueIndex := calculateSubmissionQueueIndex(
		uint64(res.index),
		firstSubmitterMemberIndex,
		uint64(groupSize),
	)

	// Calculate the block wait time based on the position in the submission
	// queue. For example, the member at the first position (index `0`)
	// waits `0 * blockStep = 0` blocks, the member at the second position
	// (index `1`) waits `1 * blockStep` blocks, and so on.
	blockWaitTime := submissionQueueIndex * blockStep

	eligibleBlockHeight := startBlockHeight + blockWaitTime
	res.logger.Infof(
		"[member:%v] waiting for block [%v] to submit",
		res.index,
		eligibleBlockHeight,
	)

	waiter, err := res.blockCounter.BlockHeightWaiter(eligibleBlockHeight)
	if err != nil {
		return nil, fmt.Errorf("block height waiter failure: [%v]", err)
	}

	return waiter, err
}

// calculateSubmissionQueueIndex calculates the position in the submission
// queue for the given member. The submission queue consists of the
// firstSubmitterMemberIndex followed by subsequent member indexes according to
// the modulus groupSize. The submission queue position for the given member
// can be computed as:
// - if memberIndex >= firstSubmitterMemberIndex: memberIndex - firstSubmitterMemberIndex
// - otherwise: memberIndex + groupSize - firstSubmitterMemberIndex
//
// For example, for `groupSize = 5` and `firstSubmitterMemberIndex = 2`, the
// submission queue is [2, 3, 4, 0, 1]. We compute the submission queue
// position for each member as:
// - member 0: 0 + 5 - 2 = 3
// - member 1: 1 + 5 - 2 = 4
// - member 2:     2 - 2 = 0
// - member 3:     3 - 2 = 1
// - member 4:     4 - 2 = 2
func calculateSubmissionQueueIndex(
	memberIndex uint64,
	firstSubmitterMemberIndex uint64,
	groupSize uint64,
) uint64 {
	if memberIndex >= firstSubmitterMemberIndex {
		return memberIndex - firstSubmitterMemberIndex
	} else {
		return memberIndex + groupSize - firstSubmitterMemberIndex
	}
}
