//go:build frost_native && frost_roast_retry

package signing

import (
	"crypto/sha256"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// newOverflowTestSubscriber returns a subscriber whose streams are all full, so
// every delivery takes deliverNonBlocking's drop arm.
func newOverflowTestSubscriber(t *testing.T) *RunnerBusSubscriber {
	t.Helper()

	// Capacity-zero channels are always full for a non-blocking send.
	s := &RunnerBusSubscriber{
		commitments:       make(chan RunnerMessage),
		signingPackages:   make(chan RunnerMessage),
		shares:            make(chan RunnerMessage),
		evidenceSnapshots: make(chan RunnerMessage),
		transitionBundles: make(chan RunnerMessage),
		seen:              map[[sha256.Size]byte]struct{}{},
	}

	return s
}

func overflowTestMessage(
	sender group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
	payload byte,
) RunnerMessage {
	return RunnerMessage{
		Type:    RunnerMsgShareSubmission,
		Sender:  sender,
		Attempt: attemptHash,
		Payload: []byte{payload},
	}
}

// TestDeliverNonBlockingRecordsOverflowEvidence pins RFC-21 Layer A / M4: a
// permanent inbound drop must be attributable to its sender, not silent. Before
// this wiring the drop arm carried only a comment, and the helper written for the
// job had no production caller, so a flooding peer could force honest messages to
// be discarded with nothing recorded anywhere.
func TestDeliverNonBlockingRecordsOverflowEvidence(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	RegisterRoastRetryCoordinator(RoastRetryDeps{SelfMember: 1})

	s := newOverflowTestSubscriber(t)
	attemptHash := [attempt.MessageDigestLength]byte{0xAA}

	// Distinct payloads so the content-hash dedup does not swallow the second and
	// third deliveries before they reach the drop arm.
	for i, sender := range []group.MemberIndex{2, 3, 2} {
		msg := overflowTestMessage(sender, attemptHash, byte(i))
		s.deliverNonBlocking(sha256.Sum256(msg.Payload), msg, 0)
	}

	overflows := s.TakeOverflowEvidence(attemptHash)
	if len(overflows) != 2 {
		t.Fatalf("expected overflow evidence for 2 senders, got [%v]", overflows)
	}
	if overflows[2] != 2 {
		t.Fatalf("expected 2 overflows recorded for member 2, got [%v]", overflows[2])
	}
	if overflows[3] != 1 {
		t.Fatalf("expected 1 overflow recorded for member 3, got [%v]", overflows[3])
	}

	// Evidence is claimed exactly once: the snapshot that carries it consumes it,
	// so a later attempt cannot re-broadcast a stale accusation.
	if again := s.TakeOverflowEvidence(attemptHash); again != nil {
		t.Fatalf("expected evidence to be consumed by the first take, got [%v]", again)
	}
}

// TestDeliverNonBlockingScopesOverflowPerAttempt checks the evidence is keyed to
// the attempt it happened in. Sharing one bucket across attempts would let a drop
// from a concluded attempt accuse a member in a later one.
func TestDeliverNonBlockingScopesOverflowPerAttempt(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	RegisterRoastRetryCoordinator(RoastRetryDeps{SelfMember: 1})

	s := newOverflowTestSubscriber(t)
	first := [attempt.MessageDigestLength]byte{0x01}
	second := [attempt.MessageDigestLength]byte{0x02}

	msgFirst := overflowTestMessage(4, first, 0x01)
	s.deliverNonBlocking(sha256.Sum256(msgFirst.Payload), msgFirst, 0)

	if got := s.TakeOverflowEvidence(second); got != nil {
		t.Fatalf("attempt [%x] must not see attempt [%x]'s evidence, got [%v]", second, first, got)
	}
	if got := s.TakeOverflowEvidence(first); got[4] != 1 {
		t.Fatalf("expected 1 overflow for member 4 in its own attempt, got [%v]", got)
	}
}

// TestDeliverNonBlockingSkipsOverflowWithoutCoordinator keeps the recording free
// when no ROAST-retry coordinator is registered: without a transition there is
// nothing that could act on the evidence, so builds that do not run retry must
// not pay for it or accumulate state.
func TestDeliverNonBlockingSkipsOverflowWithoutCoordinator(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	s := newOverflowTestSubscriber(t)
	attemptHash := [attempt.MessageDigestLength]byte{0xBB}

	msg := overflowTestMessage(5, attemptHash, 0x01)
	s.deliverNonBlocking(sha256.Sum256(msg.Payload), msg, 0)

	if got := s.TakeOverflowEvidence(attemptHash); got != nil {
		t.Fatalf("expected no evidence without a registered coordinator, got [%v]", got)
	}
	if len(s.overflowRecorders) != 0 {
		t.Fatalf(
			"expected no per-attempt state without a registered coordinator, got [%d] entries",
			len(s.overflowRecorders),
		)
	}
}
