//go:build frost_native

package signing

import (
	"bytes"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/chain/local_v1"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// fakeDKGNetMessage is a minimal net.Message for exercising the DKG bus receive
// path (sender authentication + demux) without standing up a real network.
type fakeDKGNetMessage struct {
	senderPublicKey []byte
	payload         interface{}
}

func (m fakeDKGNetMessage) TransportSenderID() net.TransportIdentifier { return nil }
func (m fakeDKGNetMessage) SenderPublicKey() []byte                    { return m.senderPublicKey }
func (m fakeDKGNetMessage) Payload() interface{}                       { return m.payload }
func (m fakeDKGNetMessage) Seqno() uint64                              { return 0 }
func (m fakeDKGNetMessage) Type() string {
	if w, ok := m.payload.(*dkgTransportMessage); ok {
		return w.Type()
	}
	return ""
}

// dkgBusAuthFixture builds a three-seat group with a MULTI-SEAT operator
// (operator A holds seats 1 and 3; operator B holds seat 2) and a bus wired to
// the resulting MembershipValidator.
type dkgBusAuthFixture struct {
	bus       *broadcastChannelDKGBus
	operatorA []byte // seats 1 and 3
	operatorB []byte // seat 2
	outsider  []byte // not selected
}

func newDKGBusAuthFixture(t *testing.T, streamSize int) dkgBusAuthFixture {
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

	bus := &broadcastChannelDKGBus{
		logger:              &testutils.MockLogger{},
		membershipValidator: validator,
		streamBuffer:        streamSize,
		seenBound:           defaultDKGBusSeenBound,
	}
	return dkgBusAuthFixture{bus: bus, operatorA: operatorA, operatorB: operatorB, outsider: outsider}
}

// dkgRound1Msg builds a round-1 message authored (signed) by authorKey. For test
// simplicity the sender's on-wire ephemeral sealing key is set to authorKey too;
// TestBroadcastChannelDKGBus_CarriesWireEphemeralKey exercises the case where they
// differ.
func dkgRound1Msg(sender group.MemberIndex, authorKey []byte, session, payload string) fakeDKGNetMessage {
	return fakeDKGNetMessage{
		senderPublicKey: authorKey,
		payload: &dkgTransportMessage{
			messageType:        dkgRound1Message,
			sender:             sender,
			session:            session,
			ephemeralPublicKey: authorKey,
			payload:            []byte(payload),
		},
	}
}

func dkgRound2Msg(sender, recipient group.MemberIndex, authorKey []byte, session, payload string) fakeDKGNetMessage {
	return fakeDKGNetMessage{
		senderPublicKey: authorKey,
		payload: &dkgTransportMessage{
			messageType:        dkgRound2Message,
			sender:             sender,
			recipient:          recipient,
			session:            session,
			ephemeralPublicKey: authorKey,
			payload:            []byte(payload),
		},
	}
}

func TestBroadcastChannelDKGBus_AuthenticatedMessagesDemuxed(t *testing.T) {
	f := newDKGBusAuthFixture(t, 8)
	// Subscribe as seat 2 so it receives the round-2 message addressed to it.
	sub := f.bus.Subscribe(2)

	// Operator A authentically sends a round-1 broadcast as seat 1 and a round-2
	// message as seat 3 (both seats it holds), the latter addressed to seat 2.
	f.bus.handleMessage(dkgRound1Msg(1, f.operatorA, "sess", "round1-from-1"))
	f.bus.handleMessage(dkgRound2Msg(3, 2, f.operatorA, "sess", "round2-from-3"))

	select {
	case msg := <-sub.round1:
		if msg.Sender != 1 || msg.Session != "sess" || string(msg.Payload) != "round1-from-1" || msg.Type != dkgRound1Message {
			t.Fatalf("unexpected round-1 delivery: %+v", msg)
		}
		// The delivered SenderPublicKey is the sender's ephemeral sealing key
		// carried on the wire (here equal to the author key).
		if !bytes.Equal(msg.SenderPublicKey, f.operatorA) {
			t.Fatalf("round-1 SenderPublicKey must be the wire ephemeral key")
		}
	default:
		t.Fatal("expected the authenticated round-1 message on the round-1 stream")
	}
	select {
	case msg := <-sub.round2:
		if msg.Sender != 3 || msg.Recipient != 2 || string(msg.Payload) != "round2-from-3" || msg.Type != dkgRound2Message {
			t.Fatalf("unexpected round-2 delivery: %+v", msg)
		}
		if !bytes.Equal(msg.SenderPublicKey, f.operatorA) {
			t.Fatalf("round-2 SenderPublicKey must be the wire ephemeral key")
		}
	default:
		t.Fatal("expected the authenticated round-2 message on the round-2 stream")
	}
}

// TestBroadcastChannelDKGBus_CarriesWireEphemeralKey proves the delivered
// SenderPublicKey (peers' round-2 sealing key) is the sender's EPHEMERAL key
// carried on the wire - NOT the operator key the message was authenticated
// against. So a seat seals to a key that never exists at rest (forward secrecy),
// while the seat itself is still bound to its operator for authentication.
func TestBroadcastChannelDKGBus_CarriesWireEphemeralKey(t *testing.T) {
	f := newDKGBusAuthFixture(t, 8)
	sub := f.bus.Subscribe(1)

	// Operator A (seat 1) authenticates the message, but the on-wire ephemeral
	// sealing key is DISTINCT from the operator key.
	ephemeralKey := []byte("distinct-ephemeral-round2-sealing-key")
	msg := dkgRound1Msg(1, f.operatorA, "sess", "round1")
	msg.payload.(*dkgTransportMessage).ephemeralPublicKey = ephemeralKey
	f.bus.handleMessage(msg)

	select {
	case delivered := <-sub.round1:
		if bytes.Equal(delivered.SenderPublicKey, f.operatorA) {
			t.Fatal("delivered sealing key must be the wire ephemeral, not the operator key")
		}
		if !bytes.Equal(delivered.SenderPublicKey, ephemeralKey) {
			t.Fatal("delivered sealing key must equal the sender's wire ephemeral key")
		}
	default:
		t.Fatal("expected the round-1 message on the round-1 stream")
	}
}

// TestBroadcastChannelDKGBus_ReplayDeliversAndDedups covers the prebuffer rescue path:
// a round-1 message captured before Start (by the prebuffer, from before the readiness
// barrier) is delivered when Replayed, and Replaying it again - as if it also arrived
// live - is deduplicated by content, so a member never sees the same round-1 twice.
func TestBroadcastChannelDKGBus_ReplayDeliversAndDedups(t *testing.T) {
	f := newDKGBusAuthFixture(t, 8)
	sub := f.bus.Subscribe(1)

	msg := dkgRound1Msg(1, f.operatorA, "sess", "prebuffered-round1")
	f.bus.Replay([]net.Message{msg})

	select {
	case delivered := <-sub.round1:
		if string(delivered.Payload) != "prebuffered-round1" {
			t.Fatalf("unexpected replayed payload: %q", delivered.Payload)
		}
	default:
		t.Fatal("a replayed round-1 message must be delivered")
	}

	// The same message arriving live (or replayed again) must NOT redeliver.
	f.bus.Replay([]net.Message{msg})
	select {
	case dup := <-sub.round1:
		t.Fatalf("a duplicate replay must be deduped, got: %q", dup.Payload)
	default:
	}
}

func TestBroadcastChannelDKGBus_RejectsSpoofedSeat(t *testing.T) {
	f := newDKGBusAuthFixture(t, 8)
	sub := f.bus.Subscribe(1)

	// Operator B (seat 2) claims seat 1; an outsider claims seat 1; operator A
	// claims seat 2 (operator B's). All must be dropped.
	f.bus.handleMessage(dkgRound1Msg(1, f.operatorB, "sess", "spoofed"))
	f.bus.handleMessage(dkgRound1Msg(1, f.outsider, "sess", "outsider"))
	f.bus.handleMessage(dkgRound1Msg(2, f.operatorA, "sess", "claiming-Bs-seat"))

	select {
	case msg := <-sub.round1:
		t.Fatalf("expected spoofed-seat messages to be dropped, got sender [%d]", msg.Sender)
	default:
	}
}

func TestBroadcastChannelDKGBus_MultiSeatOperator(t *testing.T) {
	f := newDKGBusAuthFixture(t, 8)
	sub := f.bus.Subscribe(1)

	// Operator A holds seats 1 AND 3: it may authentically send as either.
	f.bus.handleMessage(dkgRound1Msg(1, f.operatorA, "sess", "from-1"))
	f.bus.handleMessage(dkgRound1Msg(3, f.operatorA, "sess", "from-3"))

	got := map[group.MemberIndex]string{}
	for {
		select {
		case msg := <-sub.round1:
			got[msg.Sender] = string(msg.Payload)
			continue
		default:
		}
		break
	}
	if len(got) != 2 || got[1] != "from-1" || got[3] != "from-3" {
		t.Fatalf("expected both of operator A's seats delivered, got %v", got)
	}
}

func TestBroadcastChannelDKGBus_DedupsByteIdentical(t *testing.T) {
	f := newDKGBusAuthFixture(t, 8)
	sub := f.bus.Subscribe(1)

	msg := dkgRound1Msg(1, f.operatorA, "sess", "same-body")
	f.bus.handleMessage(msg)
	f.bus.handleMessage(msg) // an identical retransmission

	count := 0
	for {
		select {
		case <-sub.round1:
			count++
			continue
		default:
		}
		break
	}
	if count != 1 {
		t.Fatalf("byte-identical retransmission must be deduped to a single delivery, got %d", count)
	}
}

func TestBroadcastChannelDKGBus_Round2DeliveredOnlyToRecipient(t *testing.T) {
	f := newDKGBusAuthFixture(t, 8)
	// This subscriber is seat 1; a round-2 message addressed to seat 2 must NOT
	// reach it (that would be the O(n^2) fan-out the recipient filter prevents).
	sub := f.bus.Subscribe(1)

	f.bus.handleMessage(dkgRound2Msg(3, 2, f.operatorA, "sess", "for-seat-2"))

	select {
	case msg := <-sub.round2:
		t.Fatalf("seat 1 received a round-2 message addressed to seat 2: %+v", msg)
	default:
	}
}

func TestDKGTransportMessage_MarshalRoundTrip(t *testing.T) {
	original := &dkgTransportMessage{
		messageType:        dkgRound1Message,
		sender:             7,
		recipient:          3,
		session:            "wallet-seed-0xabcd-attempt-2",
		ephemeralPublicKey: []byte("per-dkg-ephemeral-sealing-pubkey"),
		payload:            []byte("sealed-round-2-share-bytes"),
	}
	encoded, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &dkgTransportMessage{messageType: dkgRound1Message}
	if err := decoded.Unmarshal(encoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.sender != original.sender || decoded.recipient != original.recipient ||
		decoded.session != original.session ||
		!bytes.Equal(decoded.ephemeralPublicKey, original.ephemeralPublicKey) ||
		!bytes.Equal(decoded.payload, original.payload) {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestDKGTransportMessage_UnmarshalRejectsMalformed(t *testing.T) {
	// Too short for the header.
	if err := (&dkgTransportMessage{}).Unmarshal([]byte{0x00, 0x01}); err == nil {
		t.Fatal("expected an error for a truncated header")
	}
	// Out-of-range sender (0).
	zeroSender := make([]byte, 10)
	if err := (&dkgTransportMessage{}).Unmarshal(zeroSender); err == nil {
		t.Fatal("expected an error for a zero sender")
	}
	// Session length longer than the remaining bytes.
	badSession := make([]byte, 10)
	badSession[3] = 1  // sender = 1
	badSession[9] = 20 // session_len = 20, but no session bytes follow
	if err := (&dkgTransportMessage{}).Unmarshal(badSession); err == nil {
		t.Fatal("expected an error for a truncated session field")
	}
}
