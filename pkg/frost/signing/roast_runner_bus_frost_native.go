//go:build frost_native

package signing

import (
	"crypto/sha256"
	"sync"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// RunnerMessageType tags a broadcast interactive-signing message so subscribers
// can route it to the right typed receive stream.
type RunnerMessageType int

const (
	// RunnerMsgCommitments carries a member's round-1 commitments.
	RunnerMsgCommitments RunnerMessageType = iota
	// RunnerMsgSigningPackage carries the coordinator's signed SigningPackage.
	RunnerMsgSigningPackage
	// RunnerMsgShareSubmission carries a member's signed ShareSubmission.
	RunnerMsgShareSubmission
	// RunnerMsgEvidenceSnapshot carries a member's signed LocalEvidenceSnapshot.
	RunnerMsgEvidenceSnapshot
	// RunnerMsgTransitionBundle carries the coordinator's TransitionMessage.
	RunnerMsgTransitionBundle
)

// RunnerMessage is one broadcast message on the interactive-signing bus. Payload
// is the serialized content (commitments bytes, SigningPackage/ShareSubmission/
// LocalEvidenceSnapshot/TransitionMessage envelope bytes); the bus treats it as
// opaque and never interprets it.
type RunnerMessage struct {
	Type    RunnerMessageType
	Sender  group.MemberIndex
	Attempt [attempt.MessageDigestLength]byte
	Payload []byte
}

// contentHash is the full-message identity used for retransmission dedup. It
// covers EVERY field including the payload, so two messages from the same sender
// with different bodies hash differently and are both delivered - critical
// because for signing packages and shares a body-different duplicate IS
// equivocation evidence the collector must see. Only byte-identical
// retransmissions collide and are suppressed.
func (m RunnerMessage) contentHash() [sha256.Size]byte {
	h := sha256.New()
	h.Write([]byte{byte(m.Type), byte(m.Sender)})
	h.Write(m.Attempt[:])
	h.Write(m.Payload)
	var out [sha256.Size]byte
	copy(out[:], h.Sum(nil))
	return out
}

// RunnerBus is the interactive-signing broadcast mesh. Production wraps pkg/net;
// the in-process implementation here drives the runner's deterministic unit
// tests without real networking.
type RunnerBus interface {
	// Broadcast delivers msg to every subscriber. Byte-identical retransmissions
	// are deduplicated per subscriber; body-different messages from the same
	// sender are NEVER suppressed (they are equivocation evidence). The caller
	// records its OWN produced messages into its collector/coordinator directly
	// and must not rely on receiving its own broadcast back.
	Broadcast(msg RunnerMessage)
	// Subscribe registers a receiver and returns its typed streams. The harness
	// wires every node up front (a subscriber does not receive messages
	// broadcast before it subscribed).
	Subscribe() *RunnerBusSubscriber
}

// RunnerBusSubscriber exposes one node's typed receive streams plus a
// per-subscriber dedup set keyed by full message content.
type RunnerBusSubscriber struct {
	commitments       chan RunnerMessage
	signingPackages   chan RunnerMessage
	shares            chan RunnerMessage
	evidenceSnapshots chan RunnerMessage
	transitionBundles chan RunnerMessage

	mu   sync.Mutex
	seen map[[sha256.Size]byte]struct{}
	// overflowRecorders holds one evidence recorder per attempt, created lazily on
	// the first overflow for that attempt and removed when the attempt's snapshot
	// claims it (TakeOverflowEvidence). Recording is gated on a registered
	// ROAST-retry coordinator, so this stays empty and free in builds where retry
	// is inactive. Guarded by mu.
	overflowRecorders map[[attempt.MessageDigestLength]byte]attempt.EvidenceRecorder
}

// Commitments returns the round-1 commitments stream.
func (s *RunnerBusSubscriber) Commitments() <-chan RunnerMessage { return s.commitments }

// SigningPackages returns the coordinator signing-package stream.
func (s *RunnerBusSubscriber) SigningPackages() <-chan RunnerMessage { return s.signingPackages }

// Shares returns the share-submission stream.
func (s *RunnerBusSubscriber) Shares() <-chan RunnerMessage { return s.shares }

// EvidenceSnapshots returns the evidence-snapshot stream.
func (s *RunnerBusSubscriber) EvidenceSnapshots() <-chan RunnerMessage { return s.evidenceSnapshots }

// TransitionBundles returns the transition-bundle stream.
func (s *RunnerBusSubscriber) TransitionBundles() <-chan RunnerMessage { return s.transitionBundles }

// overflowAttemptBound caps how many distinct attempts the subscriber tracks
// overflow evidence for at once. Snapshots normally claim an attempt's evidence
// promptly, so this only bounds the pathological case where evidence is recorded
// for attempts whose snapshot never runs; it mirrors the dedup set's reset policy.
const overflowAttemptBound = 64

// recordOverflowLocked notes a permanent inbound drop against the message's
// sender for the message's attempt (RFC-21 Layer A / M4). The caller must hold
// s.mu.
//
// Recording is skipped entirely when no ROAST-retry coordinator is registered:
// without one there is no transition that could act on the evidence, so this is
// free in the default and non-retry builds. Per-sender counts are saturated by
// the bounded recorder's own quota, so a flooding peer cannot inflate the
// snapshot.
func (s *RunnerBusSubscriber) recordOverflowLocked(msg RunnerMessage) {
	if _, ok := RegisteredRoastRetryCoordinator(); !ok {
		return
	}

	recorder, ok := s.overflowRecorders[msg.Attempt]
	if !ok {
		if len(s.overflowRecorders) >= overflowAttemptBound {
			// Unclaimed evidence for stale attempts: drop it wholesale rather than
			// grow without bound. Matches how the dedup set resets at its bound.
			s.overflowRecorders = nil
		}
		if s.overflowRecorders == nil {
			s.overflowRecorders = make(
				map[[attempt.MessageDigestLength]byte]attempt.EvidenceRecorder,
				1,
			)
		}
		recorder = attempt.NewBoundedRecorder()
		s.overflowRecorders[msg.Attempt] = recorder
	}

	recorder.RecordOverflow(msg.Sender)
}

// TakeOverflowEvidence removes and returns the per-sender overflow counts
// recorded for the given attempt, or nil when none were recorded. The seat's
// forced-snapshot path merges these into the evidence it broadcasts so the
// coordinator's f+1 tally can see permanent inbound drops.
func (s *RunnerBusSubscriber) TakeOverflowEvidence(
	attemptHash [attempt.MessageDigestLength]byte,
) map[group.MemberIndex]uint {
	s.mu.Lock()
	defer s.mu.Unlock()

	recorder, ok := s.overflowRecorders[attemptHash]
	if !ok {
		return nil
	}
	delete(s.overflowRecorders, attemptHash)

	overflows := recorder.Snapshot().Overflows
	if len(overflows) == 0 {
		return nil
	}

	return overflows
}

func (s *RunnerBusSubscriber) streamFor(t RunnerMessageType) chan RunnerMessage {
	switch t {
	case RunnerMsgCommitments:
		return s.commitments
	case RunnerMsgSigningPackage:
		return s.signingPackages
	case RunnerMsgShareSubmission:
		return s.shares
	case RunnerMsgEvidenceSnapshot:
		return s.evidenceSnapshots
	case RunnerMsgTransitionBundle:
		return s.transitionBundles
	default:
		return nil
	}
}

func (s *RunnerBusSubscriber) deliver(hash [sha256.Size]byte, msg RunnerMessage) {
	s.mu.Lock()
	if _, dup := s.seen[hash]; dup {
		s.mu.Unlock()
		return
	}
	s.seen[hash] = struct{}{}
	s.mu.Unlock()

	if stream := s.streamFor(msg.Type); stream != nil {
		// Own the payload bytes per delivery: RunnerMessage.Payload is a slice,
		// so without this every queued message would alias one backing array.
		// The broadcaster mutating/reusing it after Broadcast returns, or one
		// receiver mutating what it read, would then change another subscriber's
		// view - and the body the bus hashed for dedup could differ from the body
		// delivered, silently destroying the equivocation evidence this bus
		// exists to preserve. The dedup hash was computed from these same bytes,
		// so the copy is byte-identical and consistent with it.
		delivered := msg
		delivered.Payload = append([]byte(nil), msg.Payload...)
		stream <- delivered
	}
}

// inProcessRunnerBus is the deterministic in-process RunnerBus for runner unit
// tests. Streams are buffered (bufferSize per type per subscriber); a Broadcast
// blocks only if a subscriber's buffer for that type is full, so the harness
// sizes the buffer to the expected message volume.
type inProcessRunnerBus struct {
	mu          sync.Mutex
	subscribers []*RunnerBusSubscriber
	bufferSize  int
}

// NewInProcessRunnerBus returns an in-process bus with per-stream buffers of the
// given size.
func NewInProcessRunnerBus(bufferSize int) RunnerBus {
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &inProcessRunnerBus{bufferSize: bufferSize}
}

func (b *inProcessRunnerBus) Subscribe() *RunnerBusSubscriber {
	s := &RunnerBusSubscriber{
		commitments:       make(chan RunnerMessage, b.bufferSize),
		signingPackages:   make(chan RunnerMessage, b.bufferSize),
		shares:            make(chan RunnerMessage, b.bufferSize),
		evidenceSnapshots: make(chan RunnerMessage, b.bufferSize),
		transitionBundles: make(chan RunnerMessage, b.bufferSize),
		seen:              map[[sha256.Size]byte]struct{}{},
	}
	b.mu.Lock()
	b.subscribers = append(b.subscribers, s)
	b.mu.Unlock()
	return s
}

func (b *inProcessRunnerBus) Broadcast(msg RunnerMessage) {
	hash := msg.contentHash()
	b.mu.Lock()
	subscribers := append([]*RunnerBusSubscriber(nil), b.subscribers...)
	b.mu.Unlock()
	// Deliver outside the bus lock so a slow/full subscriber stream cannot block
	// other subscribers' registration; each subscriber guards its own dedup set.
	for _, s := range subscribers {
		s.deliver(hash, msg)
	}
}
