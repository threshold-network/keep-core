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
	broadcastChannel    net.BroadcastChannel
	membershipValidator *group.MembershipValidator

	receiveCtx           context.Context
	cancelReceiveCtx     context.CancelFunc
	expectedSignersCount int
	doneSigners          map[group.MemberIndex]*signingDoneMessage
	doneSignersMutex     sync.Mutex
}

func newSigningDoneCheck(
	groupSize int,
	broadcastChannel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
) *signingDoneCheck {
	return &signingDoneCheck{
		groupSize:           groupSize,
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

	messagesChan := make(chan net.Message, signingDoneReceiveBuffer)
	sdc.broadcastChannel.Recv(sdc.receiveCtx, func(message net.Message) {
		messagesChan <- message
	})

	sdc.expectedSignersCount = len(attemptMembersIndexes)
	sdc.doneSigners = make(map[group.MemberIndex]*signingDoneMessage)

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

				sdc.doneSignersMutex.Lock()
				sdc.doneSigners[doneMessage.senderID] = doneMessage
				sdc.doneSignersMutex.Unlock()

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

// waitUntilAllDone blocks until it receives all the required done checks from
// members or until the passed context is done. In the first case, it returns
// the signature computed by the signing members and the block at which the
// slowest signer completed the signature computation process. If the expected
// done checks are not received on time, the function returns an error. If at
// least one signature is different from others, the function returns an error.
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
			if sdc.expectedSignersCount == len(sdc.doneSigners) {
				var signature *frost.Signature
				var latestEndBlock uint64

				for _, doneMessage := range sdc.doneSigners {
					if signature == nil {
						signature = doneMessage.signature
					} else {
						if !signature.Equals(doneMessage.signature) {
							return nil, 0, fmt.Errorf(
								"not matching signatures detected: [%v] and [%v]",
								signature,
								doneMessage.signature,
							)
						}
					}

					if doneMessage.endBlock > latestEndBlock {
						latestEndBlock = doneMessage.endBlock
					}
				}

				return &signing.Result{Signature: signature}, latestEndBlock, nil
			}
		}
	}
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
	_, signerDone := sdc.doneSigners[doneMessage.senderID]
	if signerDone {
		// only one done message allowed
		return false
	}

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
