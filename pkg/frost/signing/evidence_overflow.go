//go:build frost_native

package signing

import (
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// senderIndexedMessage is the minimal contract a protocol message must
// satisfy for enqueueOrRecordOverflow to handle it: the message must
// expose its sender so the recorder can attribute overflow events to a
// specific member.
type senderIndexedMessage interface {
	SenderID() group.MemberIndex
}

// enqueueOrRecordOverflow attempts to enqueue payload onto target. If
// the channel is full, the overflow is recorded against the payload's
// sender on the supplied recorder instead. Returns true if the payload
// was enqueued, false if the overflow was recorded.
//
// This is a standalone, directly unit-testable select-or-record body. It is NOT
// on the production receive path: RunnerBusSubscriber.deliverNonBlocking owns
// dedup and payload copying under one lock, so it records overflow through its
// own per-attempt recorder (see recordOverflowLocked) rather than through this
// generic helper. Kept because the real-crypto overflow-park test drives it
// against a genuinely full bounded channel, which pins the record-on-full
// contract independently of the bus.
func enqueueOrRecordOverflow[T senderIndexedMessage](
	payload T,
	target chan<- T,
	recorder attempt.EvidenceRecorder,
) bool {
	select {
	case target <- payload:
		return true
	default:
		recorder.RecordOverflow(payload.SenderID())
		return false
	}
}
