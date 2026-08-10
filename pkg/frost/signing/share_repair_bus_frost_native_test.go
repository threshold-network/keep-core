//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type contextRecordingShareRepairChannel struct {
	immediateRecvBroadcastChannel
	sendContexts []context.Context
}

func (channel *contextRecordingShareRepairChannel) Send(
	ctx context.Context,
	_ net.TaggedMarshaler,
	_ ...net.RetransmissionStrategy,
) error {
	channel.sendContexts = append(channel.sendContexts, ctx)
	return nil
}

func TestShareRepairTransportRejectsMalformedFrames(t *testing.T) {
	valid := shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             1,
		ContextDigest:      [32]byte{0x01},
		EphemeralPublicKey: bytes.Repeat([]byte{0x02}, 33),
	}
	wire, err := (&shareRepairTransportMessage{message: valid}).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded := &shareRepairTransportMessage{}
	if err := decoded.Unmarshal(wire); err != nil || decoded.message.Sender != 1 {
		t.Fatalf("valid share-repair frame failed round trip: %v", err)
	}

	mutations := map[string]func([]byte) []byte{
		"unknown type": func(value []byte) []byte {
			value[0] = 0xff
			return value
		},
		"zero sender": func(value []byte) []byte {
			for index := 1; index < 5; index++ {
				value[index] = 0
			}
			return value
		},
		"zero context": func(value []byte) []byte {
			for index := 9; index < 41; index++ {
				value[index] = 0
			}
			return value
		},
		"truncated ephemeral key": func(value []byte) []byte {
			return value[:len(value)-1]
		},
		"announcement recipient": func(value []byte) []byte {
			value[8] = 2
			return value
		},
		"announcement payload": func(value []byte) []byte {
			return append(value, 0x01)
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := mutate(append([]byte(nil), wire...))
			if err := (&shareRepairTransportMessage{}).Unmarshal(candidate); err == nil {
				t.Fatal("malformed share-repair frame was accepted")
			}
		})
	}
}

func TestShareRepairInstalledAcknowledgementTransportShape(t *testing.T) {
	message := shareRepairMessage{
		Type:          shareRepairInstalledAcknowledgementMessage,
		Sender:        1,
		Recipient:     3,
		ContextDigest: [32]byte{0x44},
		Payload:       bytes.Repeat([]byte{0x55}, sha256.Size),
	}
	wire, err := (&shareRepairTransportMessage{message: message}).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decoded := &shareRepairTransportMessage{}
	if err := decoded.Unmarshal(wire); err != nil {
		t.Fatal(err)
	}
	if decoded.message.Type != message.Type ||
		decoded.message.Sender != message.Sender ||
		decoded.message.Recipient != message.Recipient ||
		decoded.message.ContextDigest != message.ContextDigest ||
		!bytes.Equal(decoded.message.Payload, message.Payload) {
		t.Fatalf("installed acknowledgement changed across round trip: %+v", decoded.message)
	}

	invalidRecipient := message
	invalidRecipient.Recipient = 0
	if _, err := (&shareRepairTransportMessage{message: invalidRecipient}).Marshal(); err == nil {
		t.Fatal("installed acknowledgement with zero recipient was accepted")
	}
	invalidDigest := message
	invalidDigest.Payload = invalidDigest.Payload[:sha256.Size-1]
	if _, err := (&shareRepairTransportMessage{message: invalidDigest}).Marshal(); err == nil {
		t.Fatal("installed acknowledgement with truncated receipt digest was accepted")
	}

	completion := message
	completion.Type = shareRepairCompletionMessage
	completion.Sender = message.Recipient
	completion.Recipient = 0
	completionWire, err := (&shareRepairTransportMessage{message: completion}).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	decodedCompletion := &shareRepairTransportMessage{}
	if err := decodedCompletion.Unmarshal(completionWire); err != nil {
		t.Fatal(err)
	}
	if decodedCompletion.message.Type != shareRepairCompletionMessage ||
		decodedCompletion.message.Sender != completion.Sender ||
		decodedCompletion.message.Recipient != 0 ||
		!bytes.Equal(decodedCompletion.message.Payload, completion.Payload) {
		t.Fatalf("share-repair completion changed across round trip: %+v", decodedCompletion.message)
	}
	invalidCompletion := completion
	invalidCompletion.Recipient = message.Sender
	if _, err := (&shareRepairTransportMessage{message: invalidCompletion}).Marshal(); err == nil {
		t.Fatal("share-repair completion with a recipient was accepted")
	}
}

func TestShareRepairBusCancelsRetransmissionsPerMessage(t *testing.T) {
	fixture := newRunnerBusAuthFixture(t, 8)
	parentContext, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	channel := &contextRecordingShareRepairChannel{}
	bus, err := newBroadcastChannelShareRepairBus(
		parentContext,
		&testutils.MockLogger{},
		channel,
		fixture.validator,
	)
	if err != nil {
		t.Fatal(err)
	}
	message := shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             1,
		ContextDigest:      [32]byte{0x44},
		EphemeralPublicKey: bytes.Repeat([]byte{0x55}, shareRepairEphemeralPublicKeyLength),
	}
	cancelFirst := bus.Broadcast(message)
	message.Sender = 2
	cancelSecond := bus.Broadcast(message)
	defer cancelSecond()
	if len(channel.sendContexts) != 2 {
		t.Fatalf("expected two per-message send contexts, got [%d]", len(channel.sendContexts))
	}
	if channel.sendContexts[0] == channel.sendContexts[1] {
		t.Fatal("share-repair messages reused one retransmission context")
	}
	cancelFirst()
	select {
	case <-channel.sendContexts[0].Done():
	default:
		t.Fatal("canceling a broadcast did not stop its retransmission context")
	}
	select {
	case <-channel.sendContexts[1].Done():
		t.Fatal("canceling one broadcast stopped a different message")
	default:
	}
	if parentContext.Err() != nil {
		t.Fatal("canceling a broadcast canceled the maintenance context")
	}
}

func TestShareRepairBusAuthenticatesSenderAndSuppressesReplay(t *testing.T) {
	fixture := newRunnerBusAuthFixture(t, 8)
	channel := &immediateRecvBroadcastChannel{}
	busInterface, err := newBroadcastChannelShareRepairBus(
		context.Background(),
		&testutils.MockLogger{},
		channel,
		fixture.validator,
	)
	if err != nil {
		t.Fatal(err)
	}
	bus := busInterface.(*broadcastChannelShareRepairBus)
	stream := bus.Subscribe(group.MemberIndex(2))
	message := shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             1,
		ContextDigest:      [32]byte{0x44},
		EphemeralPublicKey: bytes.Repeat([]byte{0x03}, 33),
	}
	wire := &shareRepairTransportMessage{message: message}
	bus.handleMessage(fakeNetMessage{senderPublicKey: fixture.operatorB, payload: wire})
	select {
	case <-stream:
		t.Fatal("claimed sender authenticated by the wrong operator was delivered")
	default:
	}

	authenticated := fakeNetMessage{senderPublicKey: fixture.operatorA, payload: wire}
	bus.handleMessage(authenticated)
	bus.handleMessage(authenticated)
	select {
	case received := <-stream:
		if received.Sender != 1 || received.ContextDigest != message.ContextDigest {
			t.Fatalf("unexpected authenticated share-repair message: %+v", received)
		}
	default:
		t.Fatal("authenticated share-repair message was not delivered")
	}
	select {
	case <-stream:
		t.Fatal("replayed share-repair message was delivered twice")
	default:
	}
}
