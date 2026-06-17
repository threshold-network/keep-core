//go:build frost_native

package signing

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// This file implements the production RunnerBus over keep-core's pkg/net
// BroadcastChannel, per the 2026-06-17 Codex+Gemini transport consult. It is a
// THIN adapter: it does NOT create or own the channel (the wallet signing
// channel is created and membership-filtered by pkg/tbtc) - it is constructed
// from the channel + the group MembershipValidator the existing signing.Request
// already carries.
//
// It honors the runner's two transport-contract assumptions:
//
//  1. RunnerMessage.Sender is the AUTHENTICATED seat. The wire body carries a
//     CLAIMED sender_id; the adapter accepts it only after IsValidMembership
//     confirms the claimed seat belongs to the message's authenticated operator
//     public key, then sets Sender from it. It NEVER trusts a sender field
//     inside Payload. (An operator can hold multiple seats - MembershipValidator
//     maps an address to a SET of positions - so a public key does not resolve
//     to a single member index; the claimed-and-validated seat is the only sound
//     binding.)
//  2. Delivery never blocks an honest broadcaster. The single Recv handler
//     demuxes into bounded per-subscriber streams with non-blocking sends
//     (dropping the newest on overflow); late/excess traffic is the blame path's
//     concern, and pkg/net retransmits a genuinely-needed message anyway.

// runnerTransportType maps each RunnerMessageType to the distinct pkg/net
// message Type() string the BroadcastChannel dispatches on.
var runnerTransportType = map[RunnerMessageType]string{
	RunnerMsgCommitments:      "frost/roast_runner/commitments",
	RunnerMsgSigningPackage:   "frost/roast_runner/signing_package",
	RunnerMsgShareSubmission:  "frost/roast_runner/share_submission",
	RunnerMsgEvidenceSnapshot: "frost/roast_runner/evidence_snapshot",
	RunnerMsgTransitionBundle: "frost/roast_runner/transition_bundle",
}

// runnerTransportMessage is the wire envelope for one RunnerMessage. The five
// runner stream types share this body and are distinguished by the Type()
// string (set per registered unmarshaler), matching the RegisterUnmarshallers
// convention. The body carries the CLAIMED sender seat, the attempt context
// hash, and the opaque runner payload.
type runnerTransportMessage struct {
	messageType RunnerMessageType
	sender      group.MemberIndex
	attempt     [attemptContextHashLength]byte
	payload     []byte
}

// attemptContextHashLength is the fixed wire length of the attempt context hash
// (a SHA-256 digest). It equals attempt.MessageDigestLength; redeclared as a
// constant here to keep the fixed-size prefix framing self-documenting.
const attemptContextHashLength = sha256.Size

// Type returns the pkg/net dispatch tag for this message's runner type.
func (m *runnerTransportMessage) Type() string {
	return runnerTransportType[m.messageType]
}

// Marshal encodes the body as: sender_id (uint32 big-endian) || attempt_context
// _hash (32 bytes) || payload (remaining bytes). The fixed-size prefix makes the
// boundary unambiguous, so the variable-length payload needs no length prefix.
func (m *runnerTransportMessage) Marshal() ([]byte, error) {
	if m.sender == 0 {
		return nil, fmt.Errorf("runner transport: sender is zero")
	}
	out := make([]byte, 4+attemptContextHashLength+len(m.payload))
	binary.BigEndian.PutUint32(out[0:4], uint32(m.sender))
	copy(out[4:4+attemptContextHashLength], m.attempt[:])
	copy(out[4+attemptContextHashLength:], m.payload)
	return out, nil
}

// Unmarshal decodes a body produced by Marshal. messageType is preset by the
// registered unmarshaler (one per Type() string), so it is not carried on the
// wire.
func (m *runnerTransportMessage) Unmarshal(data []byte) error {
	const prefix = 4 + attemptContextHashLength
	if len(data) < prefix {
		return fmt.Errorf(
			"runner transport: message length [%d] shorter than the %d-byte header",
			len(data), prefix,
		)
	}
	// Validate the raw 4-byte seat BEFORE narrowing to group.MemberIndex (uint8).
	// A truncating cast would wrap an out-of-range claim (e.g. 259 -> 3) and let
	// it pass IsValidMembership for whoever holds the wrapped seat - and round-1
	// commitments carry no inner signature to reject it later. Reject any
	// non-canonical seat at the decode boundary.
	rawSender := binary.BigEndian.Uint32(data[0:4])
	if rawSender == 0 || rawSender > uint32(group.MaxMemberIndex) {
		return fmt.Errorf(
			"runner transport: sender id [%d] out of range [1, %d]",
			rawSender, group.MaxMemberIndex,
		)
	}
	m.sender = group.MemberIndex(rawSender)
	copy(m.attempt[:], data[4:prefix])
	m.payload = append([]byte(nil), data[prefix:]...)
	return nil
}

// registerRunnerTransportUnmarshalers registers one unmarshaler per runner
// stream type, each presetting messageType so Type() and the demux know the
// stream without a wire type tag.
func registerRunnerTransportUnmarshalers(channel net.BroadcastChannel) {
	for messageType := range runnerTransportType {
		mt := messageType
		channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
			return &runnerTransportMessage{messageType: mt}
		})
	}
}

const (
	// defaultRunnerBusStreamBuffer bounds each per-subscriber stream. Sized to
	// comfortably hold one attempt's worth of one type from every member, with
	// headroom; overflow drops the newest (late/excess is the blame path's
	// concern).
	defaultRunnerBusStreamBuffer = 256
	// defaultRunnerBusSeenBound caps the per-subscriber dedup set so a peer
	// flooding body-different messages cannot grow it without bound. On overflow
	// the set resets (coarse but bounded); a re-delivered byte-identical message
	// is harmless because pkg/net already dedups retransmissions and the
	// collector is idempotent.
	defaultRunnerBusSeenBound = 4096
)

// broadcastChannelRunnerBus is the production RunnerBus over a net.BroadcastChannel.
type broadcastChannelRunnerBus struct {
	ctx                 context.Context
	logger              log.StandardLogger
	channel             net.BroadcastChannel
	membershipValidator *group.MembershipValidator
	streamBuffer        int
	seenBound           int

	mu          sync.Mutex
	subscribers []*RunnerBusSubscriber
}

// NewBroadcastChannelRunnerBus returns a RunnerBus over the given wallet signing
// broadcast channel. It registers the runner message unmarshalers and installs
// a receive handler for the lifetime of ctx; cancel ctx (e.g. at session end)
// to stop receiving. The channel and membershipValidator are the ones the
// existing signing.Request already carries; this adapter does not create them.
func NewBroadcastChannelRunnerBus(
	ctx context.Context,
	logger log.StandardLogger,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
) (RunnerBus, error) {
	if ctx == nil {
		return nil, fmt.Errorf("runner bus: context is nil")
	}
	if channel == nil {
		return nil, fmt.Errorf("runner bus: broadcast channel is nil")
	}
	if membershipValidator == nil {
		return nil, fmt.Errorf("runner bus: membership validator is nil")
	}
	if logger == nil {
		logger = log.Logger("frost-roast-runner-bus")
	}

	b := &broadcastChannelRunnerBus{
		ctx:                 ctx,
		logger:              logger,
		channel:             channel,
		membershipValidator: membershipValidator,
		streamBuffer:        defaultRunnerBusStreamBuffer,
		seenBound:           defaultRunnerBusSeenBound,
	}

	registerRunnerTransportUnmarshalers(channel)
	channel.Recv(ctx, b.handleMessage)

	return b, nil
}

// Subscribe registers a receiver and returns its typed streams. Subscribing
// before any Broadcast is the caller's responsibility (the runner subscribes in
// its constructor).
func (b *broadcastChannelRunnerBus) Subscribe() *RunnerBusSubscriber {
	s := &RunnerBusSubscriber{
		commitments:       make(chan RunnerMessage, b.streamBuffer),
		signingPackages:   make(chan RunnerMessage, b.streamBuffer),
		shares:            make(chan RunnerMessage, b.streamBuffer),
		evidenceSnapshots: make(chan RunnerMessage, b.streamBuffer),
		transitionBundles: make(chan RunnerMessage, b.streamBuffer),
		seen:              make(map[[sha256.Size]byte]struct{}),
	}

	b.mu.Lock()
	b.subscribers = append(b.subscribers, s)
	b.mu.Unlock()

	return s
}

// Broadcast publishes msg to the channel. It is fire-and-forget: pkg/net handles
// retransmission, so a Send error is logged (not surfaced) and the runner
// records its OWN produced messages directly rather than relying on self-echo.
func (b *broadcastChannelRunnerBus) Broadcast(msg RunnerMessage) {
	wire := &runnerTransportMessage{
		messageType: msg.Type,
		sender:      msg.Sender,
		attempt:     msg.Attempt,
		payload:     msg.Payload,
	}
	if err := b.channel.Send(b.ctx, wire); err != nil {
		b.logger.Warnf("runner bus: failed to broadcast [%s] message: [%v]", wire.Type(), err)
	}
}

// handleMessage is the single Recv handler. It authenticates the claimed sender
// seat against the message's authenticated operator public key, then demuxes the
// message into every subscriber's typed stream (non-blocking, deduped).
func (b *broadcastChannelRunnerBus) handleMessage(m net.Message) {
	wire, ok := m.Payload().(*runnerTransportMessage)
	if !ok {
		// A message of an unregistered/foreign type, or a decode failure.
		return
	}

	// Bind the CLAIMED seat to the AUTHENTICATED operator public key. An operator
	// may hold several seats, so this validates membership of the specific
	// claimed seat rather than resolving the key to one index. A spoofed seat (a
	// seat the sender's key was not selected to) is dropped here, before the
	// runner ever sees it.
	if !b.membershipValidator.IsValidMembership(wire.sender, m.SenderPublicKey()) {
		b.logger.Warnf(
			"runner bus: dropping [%s] message claiming unauthenticated seat [%d]",
			wire.Type(), wire.sender,
		)
		return
	}

	msg := RunnerMessage{
		Type:    wire.messageType,
		Sender:  wire.sender,
		Attempt: wire.attempt,
		Payload: wire.payload,
	}
	hash := msg.contentHash()

	b.mu.Lock()
	subscribers := append([]*RunnerBusSubscriber(nil), b.subscribers...)
	b.mu.Unlock()

	for _, s := range subscribers {
		s.deliverNonBlocking(hash, msg, b.seenBound)
	}
}

// deliverNonBlocking routes msg into the matching typed stream WITHOUT blocking:
// on a full stream it drops the newest message (earlier, useful ones stay
// queued). It dedups by full content hash per subscriber - byte-identical
// retransmissions are suppressed, while body-different messages (equivocation
// evidence) are delivered while the buffer permits. seenBound caps the dedup
// set; on overflow it resets (bounded - a rare re-delivery is harmless given
// pkg/net's own retransmission dedup and the collector's idempotent recording).
func (s *RunnerBusSubscriber) deliverNonBlocking(
	hash [sha256.Size]byte,
	msg RunnerMessage,
	seenBound int,
) {
	stream := s.streamFor(msg.Type)
	if stream == nil {
		return
	}
	// Own the payload bytes per delivery (matches the in-process bus): the
	// receive path may reuse the backing array, and a receiver must not be able
	// to mutate another subscriber's view.
	delivered := msg
	delivered.Payload = append([]byte(nil), msg.Payload...)

	// Dedup-check, enqueue, and record-seen under one lock. The non-blocking
	// send never blocks (select/default), so holding the lock across it is safe.
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.seen[hash]; dup {
		return
	}
	select {
	case stream <- delivered:
		// Record as seen ONLY after a successful enqueue. A message dropped on
		// overflow is left un-seen so the pkg/net retransmission of it can still be
		// delivered once the stream drains - marking it here would permanently
		// suppress a message the runner never actually received.
		if seenBound > 0 && len(s.seen) >= seenBound {
			s.seen = make(map[[sha256.Size]byte]struct{})
		}
		s.seen[hash] = struct{}{}
	default:
		// Stream full: drop the newest, leave it un-seen for a future retransmit.
	}
}
