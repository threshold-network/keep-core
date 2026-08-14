package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// signingDoneReceiveBuffer is a buffer for messages received from the broadcast
// channel needed when the signing done's consumer is temporarily too slow to
// handle them. Keep in mind that although we expect only 51 done messages,
// it may happen that the check receives retransmissions of messages from
// the signing protocol and before they are filtered out as not interesting for
// the done check, they are buffered in the channel.
const signingDoneReceiveBuffer = 512

// signingDoneCheckInterval determines a frequency of checking if all conditions
// to consider the signing as done are met, in waitUntilAllDone.
const signingDoneCheckInterval = 100 * time.Millisecond

// errWaitDoneTimedOut is returned by waitUntilAllDone if it did not receive
// valid done checks from all members on time.
var errWaitDoneTimedOut = fmt.Errorf("cannot receive signing done messages on time")

// signingDoneMessage is a message used to signal a successful signature
// calculation across all signing group members.
type signingDoneMessage struct {
	senderID      group.MemberIndex
	message       *big.Int
	attemptNumber uint64
	signature     *frost.Signature
	endBlock      uint64
}

func (sdm *signingDoneMessage) Type() string {
	return "tbtc/signing_done_message"
}

// signingDoneCheck is a component that is responsible for signaling a
// successful signature calculation across all signing group members.
type signingDoneCheck struct {
	groupSize           int
	honestThreshold     int
	broadcastChannel    net.BroadcastChannel
	membershipValidator *group.MembershipValidator

	receiveCtx       context.Context
	cancelReceiveCtx context.CancelFunc
	// attemptMemberCount is len(attemptMembersIndexes) for the live attempt - the
	// number of confirmations the legacy (non-oversized) rule waits for.
	attemptMemberCount int
	// oversized is true when the attempt's included set is larger than the honest
	// threshold: the RFC-21 Phase 7.3 t-of-included case, where the attempt is
	// signed by a t-subset so the non-subset / offline members never report done
	// and the legacy all-members rule would hang. It selects the
	// quorum-by-signature completion rule. When false (included == threshold, the
	// pre-oversizing selector output and the whole coarse path) the legacy rule is
	// used UNCHANGED.
	oversized bool
	// attemptTimeoutBlock is the deterministic block the attempt concludes by
	// (announcementEndBlock + signingAttemptMaximumProtocolBlocks). On the oversized
	// path it is returned as the result end block instead of a network-order-
	// dependent max over done messages, so every honest node feeds signBatch the
	// same next-signature start block (signingStartBlock = prev endBlock + interlude).
	attemptTimeoutBlock uint64
	doneSigners         map[group.MemberIndex]*signingDoneMessage
	doneSignersMutex    sync.RWMutex
}

func newSigningDoneCheck(
	groupSize int,
	honestThreshold int,
	broadcastChannel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
) *signingDoneCheck {
	return &signingDoneCheck{
		groupSize:           groupSize,
		honestThreshold:     honestThreshold,
		broadcastChannel:    broadcastChannel,
		membershipValidator: membershipValidator,
	}
}

// listen runs the signing done check listening routine. This function listens
// for incoming signing done checks from members participating in the given
// signing attempt. Messages are filtered out based on the attempt number. Only
// one message for the given attempt can be sent by the given signing group
// member. This function should be called before the signing attempt starts to
// ensure signing done messages are getting received as early as possible. This
// is especially important when the current member is the slowest one with
// executing the signing.
func (sdc *signingDoneCheck) listen(
	ctx context.Context,
	message *big.Int,
	attemptNumber uint64,
	attemptTimeoutBlock uint64,
	attemptMembersIndexes []group.MemberIndex,
) {
	// Use a separate context for the message receiver as the receiver and the
	// consuming goroutine are closed when the `waitUntilAllDone` completes its
	// work. Leaving a dangling receiver without the message processing loop
	// causes warnings on the channel level.
	sdc.receiveCtx, sdc.cancelReceiveCtx = context.WithCancel(ctx)

	sdc.doneSignersMutex.Lock()
	sdc.attemptMemberCount = len(attemptMembersIndexes)
	// An included set larger than the honest threshold is the RFC-21 Phase 7.3
	// t-of-included case: the attempt is signed by a t-subset and the non-subset /
	// offline members never report done, so the all-members rule would hang. The
	// oversized path concludes on a quorum of >= honestThreshold matching
	// signatures instead, with a deterministic end block. included == threshold
	// (today's selector output and the whole coarse path) keeps the legacy rule
	// byte-for-byte, so behavior is unchanged until participant selection oversizes
	// the set.
	sdc.oversized = len(attemptMembersIndexes) > sdc.honestThreshold
	sdc.attemptTimeoutBlock = attemptTimeoutBlock
	sdc.doneSigners = make(map[group.MemberIndex]*signingDoneMessage)
	sdc.doneSignersMutex.Unlock()

	messagesChan := make(chan net.Message, signingDoneReceiveBuffer)
	sdc.broadcastChannel.Recv(sdc.receiveCtx, func(message net.Message) {
		messagesChan <- message
	})

	go func() {
		for {
			select {
			case netMessage := <-messagesChan:
				doneMessage, ok := netMessage.Payload().(*signingDoneMessage)
				if !ok {
					continue
				}

				if !sdc.isValidDoneMessage(
					doneMessage,
					netMessage.SenderPublicKey(),
					message,
					attemptNumber,
					attemptTimeoutBlock,
				) {
					continue
				}

				if !sdc.recordDoneMessage(doneMessage) {
					continue
				}

			case <-sdc.receiveCtx.Done():
				return
			}
		}
	}()
}

// signalDone broadcasts the signing done check along with information necessary
// to attribute the result to the given signing attempt.
func (sdc *signingDoneCheck) signalDone(
	ctx context.Context,
	memberIndex group.MemberIndex,
	message *big.Int,
	attemptNumber uint64,
	result *signing.Result,
	endBlock uint64,
) error {
	return sdc.broadcastChannel.Send(ctx, &signingDoneMessage{
		senderID:      memberIndex,
		message:       message,
		attemptNumber: attemptNumber,
		signature:     result.Signature,
		endBlock:      endBlock,
	}, net.BackoffRetransmissionStrategy)
}

// waitUntilAllDone blocks until the attempt's completion rule is met or the
// passed context is done. On success it returns the agreed signature and a
// deterministic end block (the same value on every honest node): on the legacy
// path the block at which the slowest attempt member completed, on the oversized
// path the attempt timeout block. It returns errWaitDoneTimedOut if the rule is
// not met on time, and a non-nil error on a fatal divergence (legacy path only).
func (sdc *signingDoneCheck) waitUntilAllDone(ctx context.Context) (
	*signing.Result,
	uint64,
	error,
) {
	defer sdc.cancelReceiveCtx()

	ticker := time.NewTicker(signingDoneCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, 0, errWaitDoneTimedOut

		case <-ticker.C:
			result, endBlock, concluded, err := sdc.evaluateDone()
			if err != nil {
				return nil, 0, err
			}
			if concluded {
				return result, endBlock, nil
			}
		}
	}
}

// evaluateDone snapshots the done checks collected so far and applies the
// attempt's completion rule. It returns (result, endBlock, true, nil) once the
// attempt can conclude, (nil, 0, false, nil) while still waiting, or
// (nil, 0, false, err) on a fatal divergence. The legacy (non-oversized) rule
// and the oversized t-of-included rule are kept fully separate so the legacy /
// coarse path is byte-for-byte unchanged.
func (sdc *signingDoneCheck) evaluateDone() (*signing.Result, uint64, bool, error) {
	sdc.doneSignersMutex.RLock()
	oversized := sdc.oversized
	attemptMemberCount := sdc.attemptMemberCount
	attemptTimeoutBlock := sdc.attemptTimeoutBlock
	honestThreshold := sdc.honestThreshold
	doneSigners := make([]*signingDoneMessage, 0, len(sdc.doneSigners))
	for _, doneMessage := range sdc.doneSigners {
		doneSigners = append(doneSigners, doneMessage.clone())
	}
	sdc.doneSignersMutex.RUnlock()

	if oversized {
		return concludeOversizedDone(doneSigners, honestThreshold, attemptTimeoutBlock)
	}
	return concludeLegacyDone(doneSigners, attemptMemberCount)
}

// concludeLegacyDone is the pre-7.3 rule, UNCHANGED: conclude once every attempt
// member confirmed, require all signatures equal, and return the max end block.
// The attemptMemberCount > 0 guard rejects the pre-listen state (no attempt
// configured) so an empty done set is never read as success.
func concludeLegacyDone(
	doneSigners []*signingDoneMessage,
	attemptMemberCount int,
) (*signing.Result, uint64, bool, error) {
	if attemptMemberCount == 0 || len(doneSigners) != attemptMemberCount {
		return nil, 0, false, nil
	}

	var signature *frost.Signature
	var latestEndBlock uint64
	for _, doneMessage := range doneSigners {
		if signature == nil {
			signature = doneMessage.signature
		} else if !signature.Equals(doneMessage.signature) {
			return nil, 0, false, fmt.Errorf(
				"not matching signatures detected: [%v] and [%v]",
				signature,
				doneMessage.signature,
			)
		}

		if doneMessage.endBlock > latestEndBlock {
			latestEndBlock = doneMessage.endBlock
		}
	}

	return &signing.Result{Signature: signature}, latestEndBlock, true, nil
}

// concludeOversizedDone is the RFC-21 Phase 7.3 t-of-included rule: bucket the
// done checks by signature (one done message per sender, so each member is in
// exactly one bucket) and conclude once a bucket holds >= honestThreshold
// distinct senders - the minimum that proves a valid threshold signature.
// Minority buckets (divergent or adversarial signatures) are IGNORED, never
// fatal, so a single bad done message cannot fracture the group. The end block
// is the deterministic attempt timeout block, not a network-order-dependent max,
// so every honest node returns the same value for batch scheduling.
func concludeOversizedDone(
	doneSigners []*signingDoneMessage,
	honestThreshold int,
	attemptTimeoutBlock uint64,
) (*signing.Result, uint64, bool, error) {
	if honestThreshold <= 0 {
		return nil, 0, false, nil
	}

	bucketSig := map[string]*frost.Signature{}
	bucketCount := map[string]int{}
	for _, doneMessage := range doneSigners {
		serialized := doneMessage.signature.Serialize()
		key := string(serialized[:])
		if _, ok := bucketSig[key]; !ok {
			bucketSig[key] = doneMessage.signature
		}
		bucketCount[key]++
	}

	var quorumSig *frost.Signature
	quorums := 0
	for key, count := range bucketCount {
		if count >= honestThreshold {
			quorums++
			quorumSig = bucketSig[key]
		}
	}

	// Exactly one >= t bucket is the only reachable outcome under honest majority
	// (honestThreshold > groupSize/2 means two disjoint >= t buckets cannot
	// coexist). It carries the one valid signature and concludes deterministically.
	// quorums == 0 keeps waiting; quorums > 1 is unreachable and intentionally NOT
	// concluded (the attempt fails via the ctx timeout rather than picking a bucket
	// nondeterministically - a bare done-message split is not coordinator blame).
	if quorums == 1 {
		return &signing.Result{Signature: quorumSig}, attemptTimeoutBlock, true, nil
	}
	return nil, 0, false, nil
}

// isValidDoneMessage validates the given signingDoneMessage in the context
// of the given signing attempt.
func (sdc *signingDoneCheck) isValidDoneMessage(
	doneMessage *signingDoneMessage,
	senderPublicKey []byte,
	message *big.Int,
	attemptNumber uint64,
	attemptTimeoutBlock uint64,
) bool {
	if !sdc.membershipValidator.IsValidMembership(
		doneMessage.senderID,
		senderPublicKey,
	) {
		return false
	}

	if doneMessage.message.Cmp(message) != 0 {
		return false
	}

	if doneMessage.attemptNumber != attemptNumber {
		return false
	}

	if doneMessage.endBlock > attemptTimeoutBlock {
		return false
	}

	if doneMessage.signature == nil {
		return false
	}

	return true
}

func (sdc *signingDoneCheck) recordDoneMessage(
	doneMessage *signingDoneMessage,
) bool {
	sdc.doneSignersMutex.Lock()
	defer sdc.doneSignersMutex.Unlock()

	if _, signerDone := sdc.doneSigners[doneMessage.senderID]; signerDone {
		// Only one done message is allowed for the given signer.
		return false
	}

	sdc.doneSigners[doneMessage.senderID] = doneMessage.clone()
	return true
}

func (sdm *signingDoneMessage) clone() *signingDoneMessage {
	if sdm == nil {
		return nil
	}

	result := &signingDoneMessage{
		senderID:      sdm.senderID,
		attemptNumber: sdm.attemptNumber,
		endBlock:      sdm.endBlock,
	}

	if sdm.message != nil {
		result.message = new(big.Int).Set(sdm.message)
	}

	if sdm.signature != nil {
		signatureCopy := *sdm.signature
		result.signature = &signatureCopy
	}

	return result
}
