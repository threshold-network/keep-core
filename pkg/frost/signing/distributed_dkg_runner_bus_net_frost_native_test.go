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

func dkgRound1Msg(sender group.MemberIndex, authorKey []byte, session, payload string) fakeDKGNetMessage {
	return fakeDKGNetMessage{
		senderPublicKey: authorKey,
		payload: &dkgTransportMessage{
			messageType: dkgRound1Message,
			sender:      sender,
			session:     session,
			payload:     []byte(payload),
		},
	}
}

func dkgRound2Msg(sender, recipient group.MemberIndex, authorKey []byte, session, payload string) fakeDKGNetMessage {
	return fakeDKGNetMessage{
		senderPublicKey: authorKey,
		payload: &dkgTransportMessage{
			messageType: dkgRound2Message,
			sender:      sender,
			recipient:   recipient,
			session:     session,
			payload:     []byte(payload),
		},
	}
}

func TestBroadcastChannelDKGBus_AuthenticatedMessagesDemuxed(t *testing.T) {
	f := newDKGBusAuthFixture(t, 8)
	sub := f.bus.Subscribe()

	// Operator A authentically sends a round-1 broadcast as seat 1 and a round-2
	// message as seat 3 (both seats it holds).
	f.bus.handleMessage(dkgRound1Msg(1, f.operatorA, "sess", "round1-from-1"))
	f.bus.handleMessage(dkgRound2Msg(3, 2, f.operatorA, "sess", "round2-from-3"))

	select {
	case msg := <-sub.round1:
		if msg.Sender != 1 || msg.Session != "sess" || string(msg.Payload) != "round1-from-1" || msg.Type != dkgRound1Message {
			t.Fatalf("unexpected round-1 delivery: %+v", msg)
		}
	default:
		t.Fatal("expected the authenticated round-1 message on the round-1 stream")
	}
	select {
	case msg := <-sub.round2:
		if msg.Sender != 3 || msg.Recipient != 2 || string(msg.Payload) != "round2-from-3" || msg.Type != dkgRound2Message {
			t.Fatalf("unexpected round-2 delivery: %+v", msg)
		}
	default:
		t.Fatal("expected the authenticated round-2 message on the round-2 stream")
	}
}

func TestBroadcastChannelDKGBus_RejectsSpoofedSeat(t *testing.T) {
	f := newDKGBusAuthFixture(t, 8)
	sub := f.bus.Subscribe()

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
	sub := f.bus.Subscribe()

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
	sub := f.bus.Subscribe()

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

func TestDKGTransportMessage_MarshalRoundTrip(t *testing.T) {
	original := &dkgTransportMessage{
		messageType: dkgRound2Message,
		sender:      7,
		recipient:   3,
		session:     "wallet-seed-0xabcd-attempt-2",
		payload:     []byte("sealed-round-2-share-bytes"),
	}
	encoded, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	decoded := &dkgTransportMessage{messageType: dkgRound2Message}
	if err := decoded.Unmarshal(encoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.sender != original.sender || decoded.recipient != original.recipient ||
		decoded.session != original.session || !bytes.Equal(decoded.payload, original.payload) {
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
