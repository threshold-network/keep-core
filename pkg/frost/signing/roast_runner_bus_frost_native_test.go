//go:build frost_native

package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func testRunnerMessage(t RunnerMessageType, sender group.MemberIndex, payload []byte) RunnerMessage {
	return RunnerMessage{
		Type:    t,
		Sender:  sender,
		Attempt: [attempt.MessageDigestLength]byte{0x42},
		Payload: payload,
	}
}

// recvOrFail reads one message from a stream without blocking the test forever.
func recvOrFail(t *testing.T, stream <-chan RunnerMessage, what string) RunnerMessage {
	t.Helper()
	select {
	case msg := <-stream:
		return msg
	default:
		t.Fatalf("expected a %s message, stream was empty", what)
		return RunnerMessage{}
	}
}

func TestInProcessRunnerBus_BroadcastsToAllSubscribersOnTypedStream(t *testing.T) {
	bus := NewInProcessRunnerBus(8)
	a := bus.Subscribe()
	b := bus.Subscribe()

	bus.Broadcast(testRunnerMessage(RunnerMsgSigningPackage, 1, []byte{0xde, 0xad}))

	for name, sub := range map[string]*RunnerBusSubscriber{"a": a, "b": b} {
		msg := recvOrFail(t, sub.SigningPackages(), "signing package")
		if msg.Sender != 1 || string(msg.Payload) != string([]byte{0xde, 0xad}) {
			t.Fatalf("subscriber %s got wrong message: %+v", name, msg)
		}
		// It must NOT also appear on another typed stream.
		select {
		case other := <-sub.Commitments():
			t.Fatalf("subscriber %s leaked a package onto the commitments stream: %+v", name, other)
		default:
		}
	}
}

func TestInProcessRunnerBus_DedupsExactRetransmission(t *testing.T) {
	bus := NewInProcessRunnerBus(8)
	sub := bus.Subscribe()

	msg := testRunnerMessage(RunnerMsgShareSubmission, 2, []byte{0x01, 0x02})
	bus.Broadcast(msg)
	bus.Broadcast(msg) // byte-identical retransmission

	recvOrFail(t, sub.Shares(), "share")
	select {
	case dup := <-sub.Shares():
		t.Fatalf("exact retransmission must be deduped, got a second delivery: %+v", dup)
	default:
	}
}

// The critical property: a body-DIFFERENT message from the same sender must NOT
// be suppressed - for packages and shares it is equivocation evidence the
// collector has to see. Deduping by (attempt, sender) would silently drop it.
func TestInProcessRunnerBus_DeliversBodyDifferentDuplicateFromSameSender(t *testing.T) {
	bus := NewInProcessRunnerBus(8)
	sub := bus.Subscribe()

	bus.Broadcast(testRunnerMessage(RunnerMsgSigningPackage, 1, []byte{0xaa}))
	bus.Broadcast(testRunnerMessage(RunnerMsgSigningPackage, 1, []byte{0xbb})) // same sender, different body

	first := recvOrFail(t, sub.SigningPackages(), "first package")
	second := recvOrFail(t, sub.SigningPackages(), "second (equivocating) package")
	if string(first.Payload) == string(second.Payload) {
		t.Fatal("expected two distinct package bodies to be delivered")
	}
	got := map[string]bool{string(first.Payload): true, string(second.Payload): true}
	if !got[string([]byte{0xaa})] || !got[string([]byte{0xbb})] {
		t.Fatalf("expected both 0xaa and 0xbb bodies delivered, got: %v", got)
	}
}

func TestInProcessRunnerBus_LateSubscriberMissesPastMessages(t *testing.T) {
	bus := NewInProcessRunnerBus(8)
	bus.Broadcast(testRunnerMessage(RunnerMsgCommitments, 1, []byte{0x01}))

	late := bus.Subscribe()
	select {
	case msg := <-late.Commitments():
		t.Fatalf("a late subscriber must not receive past messages, got: %+v", msg)
	default:
	}
}

// Each delivered message must own its payload bytes: mutating the broadcaster's
// slice after Broadcast, or one subscriber's received slice, must not change
// another subscriber's view - otherwise the body the bus hashed for dedup could
// differ from the body delivered, destroying equivocation evidence.
func TestInProcessRunnerBus_OwnsDeliveredPayload(t *testing.T) {
	bus := NewInProcessRunnerBus(8)
	a := bus.Subscribe()
	b := bus.Subscribe()

	payload := []byte{0xaa, 0xbb}
	bus.Broadcast(testRunnerMessage(RunnerMsgSigningPackage, 1, payload))

	// Mutate the broadcaster's original slice after Broadcast returns.
	payload[0] = 0xff
	msgA := recvOrFail(t, a.SigningPackages(), "package (a)")
	if msgA.Payload[0] != 0xaa {
		t.Fatalf("subscriber a saw the broadcaster's post-broadcast mutation: %x", msgA.Payload)
	}

	// Mutating a's received payload must not corrupt b's.
	msgA.Payload[0] = 0xee
	msgB := recvOrFail(t, b.SigningPackages(), "package (b)")
	if msgB.Payload[0] != 0xaa {
		t.Fatalf("subscriber b's payload was corrupted by a's mutation: %x", msgB.Payload)
	}
}
