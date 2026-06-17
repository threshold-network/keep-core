//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"encoding/binary"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// fakeNetMessage is a minimal net.Message for exercising the adapter's receive
// path (sender authentication + demux) without standing up a full network. The
// authenticated author key is SenderPublicKey(); Payload() is the unmarshaled
// runner transport message the channel would hand the handler.
type fakeNetMessage struct {
	senderPublicKey []byte
	payload         interface{}
}

func (m fakeNetMessage) TransportSenderID() net.TransportIdentifier { return nil }
func (m fakeNetMessage) SenderPublicKey() []byte                    { return m.senderPublicKey }
func (m fakeNetMessage) Payload() interface{}                       { return m.payload }
func (m fakeNetMessage) Seqno() uint64                              { return 0 }
func (m fakeNetMessage) Type() string {
	if w, ok := m.payload.(*runnerTransportMessage); ok {
		return w.Type()
	}
	return ""
}

// runnerBusAuthFixture builds a three-seat group with a MULTI-SEAT operator
// (operator A holds seats 1 and 3; operator B holds seat 2) and a bus wired to
// the resulting MembershipValidator. It returns the bus plus each operator's
// authenticated public-key bytes and an outsider's key (not in the group).
type runnerBusAuthFixture struct {
	bus        *broadcastChannelRunnerBus
	operatorA  []byte // seats 1 and 3
	operatorB  []byte // seat 2
	outsider   []byte // not selected
	streamSize int
}

func newRunnerBusAuthFixture(t *testing.T, streamSize int) runnerBusAuthFixture {
	t.Helper()
	signing := local_v1.Connect(3, 3).Signing()

	key := func() []byte {
		_, publicKey, err := operator.GenerateKeyPair(local_v1.DefaultCurve)
		if err != nil {
			t.Fatalf("generate operator key: %v", err)
		}
		return operator.MarshalUncompressed(publicKey)
	}
	operatorA, operatorB, outsider := key(), key(), key()

	addrA := signing.PublicKeyBytesToAddress(operatorA)
	addrB := signing.PublicKeyBytesToAddress(operatorB)
	// Ordered seats: 1 -> A, 2 -> B, 3 -> A (operator A is multi-seat).
	validator := group.NewMembershipValidator(
		&testutils.MockLogger{},
		[]chain.Address{addrA, addrB, addrA},
		signing,
	)

	bus := &broadcastChannelRunnerBus{
		logger:              &testutils.MockLogger{},
		membershipValidator: validator,
		streamBuffer:        streamSize,
		seenBound:           defaultRunnerBusSeenBound,
	}
	return runnerBusAuthFixture{
		bus:        bus,
		operatorA:  operatorA,
		operatorB:  operatorB,
		outsider:   outsider,
		streamSize: streamSize,
	}
}

func shareMessage(sender group.MemberIndex, authorPublicKey []byte, payload string) fakeNetMessage {
	return fakeNetMessage{
		senderPublicKey: authorPublicKey,
		payload: &runnerTransportMessage{
			messageType: RunnerMsgShareSubmission,
			sender:      sender,
			attempt:     [attemptContextHashLength]byte{0x42},
			payload:     []byte(payload),
		},
	}
}

func TestBroadcastChannelRunnerBus_AuthenticatedMessageDemuxed(t *testing.T) {
	f := newRunnerBusAuthFixture(t, 8)
	sub := f.bus.Subscribe()

	// Operator A authentically sends as seat 1 (a seat it holds).
	f.bus.handleMessage(shareMessage(1, f.operatorA, "share-from-1"))

	select {
	case msg := <-sub.Shares():
		if msg.Sender != 1 {
			t.Fatalf("unexpected sender: [%d]", msg.Sender)
		}
		if string(msg.Payload) != "share-from-1" {
			t.Fatalf("unexpected payload: [%q]", msg.Payload)
		}
		if msg.Type != RunnerMsgShareSubmission {
			t.Fatalf("unexpected type: [%v]", msg.Type)
		}
	default:
		t.Fatal("expected an authenticated message to be delivered")
	}
}

func TestBroadcastChannelRunnerBus_RejectsSpoofedSeat(t *testing.T) {
	f := newRunnerBusAuthFixture(t, 8)
	sub := f.bus.Subscribe()

	// Operator B (seat 2) claims seat 1 - a seat its key was NOT selected to.
	f.bus.handleMessage(shareMessage(1, f.operatorB, "spoofed"))
	// An outsider (no seat) claims seat 1.
	f.bus.handleMessage(shareMessage(1, f.outsider, "outsider"))

	select {
	case msg := <-sub.Shares():
		t.Fatalf("expected spoofed-seat messages to be dropped, got sender [%d]", msg.Sender)
	default:
	}
}

func TestBroadcastChannelRunnerBus_MultiSeatOperator(t *testing.T) {
	f := newRunnerBusAuthFixture(t, 8)
	sub := f.bus.Subscribe()

	// Operator A holds seats 1 AND 3: it may authentically send as either, but
	// NOT as seat 2 (operator B's).
	f.bus.handleMessage(shareMessage(3, f.operatorA, "share-from-3"))
	f.bus.handleMessage(shareMessage(2, f.operatorA, "claiming-Bs-seat"))

	got := map[group.MemberIndex]string{}
	for {
		select {
		case msg := <-sub.Shares():
			got[msg.Sender] = string(msg.Payload)
			continue
		default:
		}
		break
	}
	if len(got) != 1 || got[3] != "share-from-3" {
		t.Fatalf("expected only seat 3 delivered, got %v", got)
	}
}

func TestBroadcastChannelRunnerBus_DedupsByteIdentical(t *testing.T) {
	f := newRunnerBusAuthFixture(t, 8)
	sub := f.bus.Subscribe()

	msg := shareMessage(1, f.operatorA, "same-body")
	f.bus.handleMessage(msg)
	f.bus.handleMessage(msg) // a retransmission of the identical content

	count := 0
	for {
		select {
		case <-sub.Shares():
			count++
			continue
		default:
		}
		break
	}
	if count != 1 {
		t.Fatalf("expected one delivery for byte-identical messages, got [%d]", count)
	}
}

func TestRunnerTransportMessage_RejectsOutOfRangeSender(t *testing.T) {
	frame := make([]byte, 4+attemptContextHashLength+2)
	// A seat beyond the valid range must be rejected at decode, BEFORE the uint8
	// narrowing - else it would wrap (e.g. 256+3 -> 3) and pass authentication.
	binary.BigEndian.PutUint32(frame[0:4], uint32(group.MaxMemberIndex)+1)
	if err := (&runnerTransportMessage{}).Unmarshal(frame); err == nil {
		t.Fatal("expected an out-of-range sender id to be rejected")
	}
	binary.BigEndian.PutUint32(frame[0:4], 256+3) // wraps to seat 3 if truncated
	if err := (&runnerTransportMessage{}).Unmarshal(frame); err == nil {
		t.Fatal("expected a wrapping sender id to be rejected before truncation")
	}
}

// A message dropped because a stream was full must NOT be recorded as seen: a
// drop must not poison the dedup set against a later re-delivery of the same
// content. (Standard pkg/net retransmissions are filtered upstream and would not
// re-reach the handler, so the real protection against losing an honest message
// is the buffer sizing; this guards only the dedup bookkeeping for any
// non-retransmit re-delivery.)
func TestBroadcastChannelRunnerBus_DropDoesNotPoisonDedup(t *testing.T) {
	f := newRunnerBusAuthFixture(t, 1) // buffer of one
	sub := f.bus.Subscribe()

	first := shareMessage(1, f.operatorA, "first")
	second := shareMessage(1, f.operatorA, "second") // distinct body, distinct hash

	f.bus.handleMessage(first)  // fills the single-slot buffer
	f.bus.handleMessage(second) // buffer full -> dropped (must stay un-seen)

	if got := <-sub.Shares(); string(got.Payload) != "first" {
		t.Fatalf("expected 'first' drained, got %q", got.Payload)
	}

	// A re-delivery of the dropped content, now that the buffer has room, is
	// accepted (not suppressed as a duplicate).
	f.bus.handleMessage(second)
	select {
	case got := <-sub.Shares():
		if string(got.Payload) != "second" {
			t.Fatalf("expected 'second' on re-delivery, got %q", got.Payload)
		}
	default:
		t.Fatal("a message dropped on overflow must not be suppressed on a later re-delivery")
	}
}

func TestBroadcastChannelRunnerBus_DropsWhenStreamFullWithoutBlocking(t *testing.T) {
	f := newRunnerBusAuthFixture(t, 2) // tiny stream buffer
	sub := f.bus.Subscribe()

	// Deliver more distinct (body-different) shares than the buffer holds. Each
	// must not block; the excess is dropped (newest).
	for i := 0; i < 5; i++ {
		f.bus.handleMessage(shareMessage(1, f.operatorA, string(rune('a'+i))))
	}

	count := 0
	for {
		select {
		case <-sub.Shares():
			count++
			continue
		default:
		}
		break
	}
	if count != 2 {
		t.Fatalf("expected the stream bounded to 2, got [%d] (and Broadcast must not have blocked)", count)
	}
}

func TestRunnerTransportMessage_RoundTrip(t *testing.T) {
	original := &runnerTransportMessage{
		messageType: RunnerMsgSigningPackage,
		sender:      3,
		attempt:     [attemptContextHashLength]byte{0x42, 0x99, 0xff},
		payload:     []byte("signed-signing-package-envelope"),
	}

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The registered unmarshaler presets messageType (it is not on the wire).
	decoded := &runnerTransportMessage{messageType: RunnerMsgSigningPackage}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.sender != original.sender {
		t.Fatalf("sender: got [%d], want [%d]", decoded.sender, original.sender)
	}
	if decoded.attempt != original.attempt {
		t.Fatalf("attempt mismatch: got [%x], want [%x]", decoded.attempt, original.attempt)
	}
	if !bytes.Equal(decoded.payload, original.payload) {
		t.Fatalf("payload mismatch: got [%q], want [%q]", decoded.payload, original.payload)
	}
	if decoded.Type() != "frost/roast_runner/signing_package" {
		t.Fatalf("unexpected wire type: [%s]", decoded.Type())
	}
}

func TestRunnerTransportMessage_EmptyPayloadRoundTrips(t *testing.T) {
	original := &runnerTransportMessage{messageType: RunnerMsgCommitments, sender: 1}
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &runnerTransportMessage{messageType: RunnerMsgCommitments}
	if err := decoded.Unmarshal(data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.sender != 1 || len(decoded.payload) != 0 {
		t.Fatalf("unexpected decode: sender [%d], payload len [%d]", decoded.sender, len(decoded.payload))
	}
}

func TestRunnerTransportMessage_RejectsInvalid(t *testing.T) {
	// Zero sender on marshal.
	if _, err := (&runnerTransportMessage{messageType: RunnerMsgCommitments, sender: 0}).Marshal(); err == nil {
		t.Fatal("expected marshal to reject a zero sender")
	}
	// Shorter than the fixed header.
	if err := (&runnerTransportMessage{}).Unmarshal([]byte{0x00, 0x01}); err == nil {
		t.Fatal("expected unmarshal to reject a short message")
	}
	// Exactly the header but a zero sender (all-zero prefix).
	if err := (&runnerTransportMessage{}).Unmarshal(make([]byte, 4+attemptContextHashLength)); err == nil {
		t.Fatal("expected unmarshal to reject a zero sender")
	}
}

func TestRunnerTransportType_CoversEveryStream(t *testing.T) {
	// Every type the subscriber demuxes must have a distinct wire type string,
	// else BroadcastChannel cannot dispatch it.
	types := []RunnerMessageType{
		RunnerMsgCommitments,
		RunnerMsgSigningPackage,
		RunnerMsgShareSubmission,
		RunnerMsgEvidenceSnapshot,
		RunnerMsgTransitionBundle,
	}
	seen := map[string]struct{}{}
	for _, mt := range types {
		s := runnerTransportType[mt]
		if s == "" {
			t.Fatalf("runner message type [%v] has no wire type string", mt)
		}
		if _, dup := seen[s]; dup {
			t.Fatalf("wire type string [%s] is not distinct", s)
		}
		seen[s] = struct{}{}
	}
}

func TestNewBroadcastChannelRunnerBus_RejectsNilDependencies(t *testing.T) {
	ctx := context.Background()
	// nil context
	if _, err := NewBroadcastChannelRunnerBus(nil, nil, nil, nil); err == nil { //nolint:staticcheck
		t.Fatal("expected nil context to be rejected")
	}
	// nil channel (other deps non-nil-checked after)
	if _, err := NewBroadcastChannelRunnerBus(ctx, nil, nil, nil); err == nil {
		t.Fatal("expected nil channel to be rejected")
	}
}
