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

// This file implements the production DKGBus over keep-core's pkg/net
// BroadcastChannel - the transport the distributed-DKG orchestrator runs on in a
// real deployment (the wallet DKG broadcast channel, already opened and
// membership-filtered by pkg/tbtc). It mirrors the ROAST runner bus
// (roast_runner_bus_net_frost_native.go): it authenticates a message's CLAIMED
// sender against the message's authenticated operator public key BEFORE
// delivering, and demuxes into bounded per-subscriber streams with non-blocking
// sends so a slow or flooding peer never blocks an honest broadcaster.
//
// Round-1 messages are broadcast (Recipient 0); round-2 messages are addressed
// (Recipient set) but still delivered to every subscriber - the orchestrator
// keeps only the ones addressed to it, and only it can OPEN a share sealed to it.
// Confidentiality of round-2 shares comes from the seal envelope, not the
// transport; the transport provides sender authentication and the session
// discriminator the orchestrator's collectors require.

// dkgTransportType maps each DKG message type to the pkg/net Type() string the
// BroadcastChannel dispatches on.
var dkgTransportType = map[dkgMessageType]string{
	dkgRound1Message: "frost/dkg_runner/round1",
	dkgRound2Message: "frost/dkg_runner/round2",
}

// dkgTransportMessage is the wire envelope for one dkgMessage. The two DKG
// message types share this body and are distinguished by Type() (set per
// registered unmarshaler). The body carries the CLAIMED sender seat, the
// addressed recipient (0 for a round-1 broadcast), the attempt session, and the
// opaque round payload; the sender is authenticated by the receive handler.
type dkgTransportMessage struct {
	messageType dkgMessageType
	sender      group.MemberIndex
	recipient   group.MemberIndex
	session     string
	payload     []byte
}

// Type returns the pkg/net dispatch tag for this message's DKG round type.
func (m *dkgTransportMessage) Type() string { return dkgTransportType[m.messageType] }

// Marshal encodes the body as: sender(4 BE) || recipient(4 BE) ||
// session_len(2 BE) || session || payload. The fixed-size prefix plus the
// length-prefixed session make the payload boundary unambiguous.
func (m *dkgTransportMessage) Marshal() ([]byte, error) {
	if m.sender == 0 {
		return nil, fmt.Errorf("dkg transport: sender is zero")
	}
	if len(m.session) > 0xffff {
		return nil, fmt.Errorf("dkg transport: session length [%d] exceeds the 16-bit cap", len(m.session))
	}
	out := make([]byte, 10+len(m.session)+len(m.payload))
	binary.BigEndian.PutUint32(out[0:4], uint32(m.sender))
	binary.BigEndian.PutUint32(out[4:8], uint32(m.recipient))
	binary.BigEndian.PutUint16(out[8:10], uint16(len(m.session)))
	copy(out[10:10+len(m.session)], m.session)
	copy(out[10+len(m.session):], m.payload)
	return out, nil
}

// Unmarshal decodes a body produced by Marshal. messageType is preset by the
// registered unmarshaler (one per Type() string), so it is not carried on wire.
// It rejects a non-canonical sender/recipient at the decode boundary before the
// truncating cast to group.MemberIndex (uint8).
func (m *dkgTransportMessage) Unmarshal(data []byte) error {
	const prefix = 10
	if len(data) < prefix {
		return fmt.Errorf(
			"dkg transport: message length [%d] shorter than the %d-byte header",
			len(data), prefix,
		)
	}
	rawSender := binary.BigEndian.Uint32(data[0:4])
	if rawSender == 0 || rawSender > uint32(group.MaxMemberIndex) {
		return fmt.Errorf(
			"dkg transport: sender id [%d] out of range [1, %d]", rawSender, group.MaxMemberIndex,
		)
	}
	rawRecipient := binary.BigEndian.Uint32(data[4:8])
	if rawRecipient > uint32(group.MaxMemberIndex) {
		return fmt.Errorf(
			"dkg transport: recipient id [%d] out of range [0, %d]", rawRecipient, group.MaxMemberIndex,
		)
	}
	sessionLen := int(binary.BigEndian.Uint16(data[8:10]))
	if len(data) < prefix+sessionLen {
		return fmt.Errorf("dkg transport: message truncated in the session field")
	}
	m.sender = group.MemberIndex(rawSender)
	m.recipient = group.MemberIndex(rawRecipient)
	m.session = string(data[prefix : prefix+sessionLen])
	m.payload = append([]byte(nil), data[prefix+sessionLen:]...)
	return nil
}

func registerDKGTransportUnmarshalers(channel net.BroadcastChannel) {
	for messageType := range dkgTransportType {
		mt := messageType
		channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
			return &dkgTransportMessage{messageType: mt}
		})
	}
}

const (
	// defaultDKGBusStreamBuffer bounds each per-subscriber stream. Because round-2
	// messages are delivered only to their addressed recipient, a subscriber's
	// honest per-stream volume is O(n): round-1 is one per member and round-2 is
	// one per OTHER member (those addressed to this seat). 1024 sits well above the
	// largest tBTC group (MaxMemberIndex 255), so honest operation never overflows;
	// only a flooding peer can, which degrades the attempt to a retry.
	defaultDKGBusStreamBuffer = 1024
	// defaultDKGBusSeenBound caps the per-subscriber dedup set so a peer flooding
	// distinct messages cannot grow it without bound; on overflow it resets.
	defaultDKGBusSeenBound = 4096
)

// broadcastChannelDKGBus is the production DKGBus over a net.BroadcastChannel.
type broadcastChannelDKGBus struct {
	ctx                 context.Context
	logger              log.StandardLogger
	channel             net.BroadcastChannel
	membershipValidator *group.MembershipValidator
	streamBuffer        int
	seenBound           int

	mu          sync.Mutex
	subscribers []*dkgBusSubscriber
}

// NewBroadcastChannelDKGBus returns a DKGBus over the given wallet DKG broadcast
// channel. It registers the DKG message unmarshalers and installs a receive
// handler for the lifetime of ctx; cancel ctx (e.g. at DKG conclusion) to stop
// receiving. The channel and membershipValidator are the ones pkg/tbtc already
// carries for the DKG; this adapter does not create them.
func NewBroadcastChannelDKGBus(
	ctx context.Context,
	logger log.StandardLogger,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
) (DKGBus, error) {
	if ctx == nil {
		return nil, fmt.Errorf("dkg bus: context is nil")
	}
	if channel == nil {
		return nil, fmt.Errorf("dkg bus: broadcast channel is nil")
	}
	if membershipValidator == nil {
		return nil, fmt.Errorf("dkg bus: membership validator is nil")
	}
	if logger == nil {
		logger = log.Logger("frost-distributed-dkg-bus")
	}

	b := &broadcastChannelDKGBus{
		ctx:                 ctx,
		logger:              logger,
		channel:             channel,
		membershipValidator: membershipValidator,
		streamBuffer:        defaultDKGBusStreamBuffer,
		seenBound:           defaultDKGBusSeenBound,
	}

	registerDKGTransportUnmarshalers(channel)
	channel.Recv(ctx, b.handleMessage)

	return b, nil
}

// Subscribe registers a receiver and returns its typed streams. The orchestrator
// subscribes in its constructor, before broadcasting.
func (b *broadcastChannelDKGBus) Subscribe(member group.MemberIndex) *dkgBusSubscriber {
	s := &dkgBusSubscriber{
		member: member,
		round1: make(chan dkgMessage, b.streamBuffer),
		round2: make(chan dkgMessage, b.streamBuffer),
		seen:   make(map[[32]byte]struct{}),
	}
	b.mu.Lock()
	b.subscribers = append(b.subscribers, s)
	b.mu.Unlock()
	return s
}

// Broadcast publishes msg to the channel; pkg/net handles retransmission, so a
// Send error is logged, not surfaced.
func (b *broadcastChannelDKGBus) Broadcast(msg dkgMessage) {
	wire := &dkgTransportMessage{
		messageType: msg.Type,
		sender:      msg.Sender,
		recipient:   msg.Recipient,
		session:     msg.Session,
		payload:     msg.Payload,
	}
	if err := b.channel.Send(b.ctx, wire); err != nil {
		b.logger.Warnf("dkg bus: failed to broadcast [%s] message: [%v]", wire.Type(), err)
	}
}

// handleMessage is the single Recv handler. It authenticates the claimed sender
// seat against the message's authenticated operator public key, then demuxes the
// message into every subscriber's typed stream (non-blocking, deduped).
func (b *broadcastChannelDKGBus) handleMessage(m net.Message) {
	wire, ok := m.Payload().(*dkgTransportMessage)
	if !ok {
		return
	}
	// Bind the CLAIMED seat to the AUTHENTICATED operator public key. A spoofed
	// seat (one the sender's key was not selected to) is dropped here, before the
	// orchestrator ever sees it.
	if !b.membershipValidator.IsValidMembership(wire.sender, m.SenderPublicKey()) {
		b.logger.Warnf(
			"dkg bus: dropping [%s] message claiming unauthenticated seat [%d]",
			wire.Type(), wire.sender,
		)
		return
	}

	msg := dkgMessage{
		Type:    wire.messageType,
		Session: wire.session,
		Sender:  wire.sender,
		// The AUTHENTICATED operator public key, not any value claimed on the wire:
		// peers learn each other's round-2 sealing key from this, so it must be the
		// key membership was validated against.
		SenderPublicKey: m.SenderPublicKey(),
		Recipient:       wire.recipient,
		Payload:         wire.payload,
	}
	hash := msg.contentHash()

	b.mu.Lock()
	subscribers := append([]*dkgBusSubscriber(nil), b.subscribers...)
	b.mu.Unlock()

	for _, s := range subscribers {
		// A round-2 message is ADDRESSED: deliver it only to its recipient's
		// subscriber, so a subscriber buffers O(n) round-2 messages (those for it),
		// not the O(n^2) whole-group fan-out that would overflow the stream at
		// mainnet group sizes. Round-1 messages are broadcast to every subscriber.
		if msg.Type == dkgRound2Message && s.member != msg.Recipient {
			continue
		}
		s.deliverNonBlocking(hash, msg, b.seenBound)
	}
}

// contentHash is the full-message identity used for retransmission dedup; it
// covers every field so a body-different message hashes differently.
func (m dkgMessage) contentHash() [32]byte {
	h := sha256.New()
	h.Write([]byte{byte(m.Type), byte(m.Sender), byte(m.Recipient)})
	h.Write([]byte(m.Session))
	h.Write(m.Payload)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func (s *dkgBusSubscriber) streamFor(t dkgMessageType) chan dkgMessage {
	switch t {
	case dkgRound1Message:
		return s.round1
	case dkgRound2Message:
		return s.round2
	default:
		return nil
	}
}

// deliverNonBlocking routes msg into the matching typed stream WITHOUT blocking:
// on a full stream it drops the newest (earlier, useful messages stay queued).
// It dedups by full content hash per subscriber; seenBound caps the dedup set.
func (s *dkgBusSubscriber) deliverNonBlocking(hash [32]byte, msg dkgMessage, seenBound int) {
	stream := s.streamFor(msg.Type)
	if stream == nil {
		return
	}
	// Own the payload bytes per delivery so no receiver can mutate another's view.
	delivered := msg
	delivered.Payload = append([]byte(nil), msg.Payload...)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.seen[hash]; dup {
		return
	}
	select {
	case stream <- delivered:
		if seenBound > 0 && len(s.seen) >= seenBound {
			s.seen = make(map[[32]byte]struct{})
		}
		s.seen[hash] = struct{}{}
	default:
		// Stream full: drop the newest (a flood -> retry), leaving it un-seen.
	}
}
