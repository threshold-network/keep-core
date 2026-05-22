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
// This is the shared select-or-record body that replaces the three
// inline select { default } drop sites in the FROST/tbtc-signer
// receive loops. Pulling it out lets the recorder integration be unit-
// tested directly without spinning up a network channel.
//
// Phase 2 callers pass attempt.NoOpRecorder(), so behaviour is
// observably unchanged from before RFC-21 wiring. A coordinator-aware
// caller in a later phase injects a real recorder.
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
