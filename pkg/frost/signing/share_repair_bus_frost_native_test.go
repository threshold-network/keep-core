//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type contextRecordingShareRepairChannel struct {
	immediateRecvBroadcastChannel
	sendContexts []context.Context
}

func TestShareRepairPublicPackageTransportPayloadCap(t *testing.T) {
	message := shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        1,
		ContextDigest: [32]byte{0x44},
		Payload:       bytes.Repeat([]byte{0x55}, shareRepairMaximumPublicPayload),
	}
	wire, err := (&shareRepairTransportMessage{message: message}).Marshal()
	if err != nil {
		t.Fatalf("public package at the cap was rejected: %v", err)
	}
	decoded := &shareRepairTransportMessage{}
	if err := decoded.Unmarshal(wire); err != nil {
		t.Fatalf("public package at the cap failed decoding: %v", err)
	}
	if len(decoded.message.Payload) != shareRepairMaximumPublicPayload {
		t.Fatalf("decoded public package length is [%d]", len(decoded.message.Payload))
	}

	overCap := message
	overCap.Payload = append(append([]byte(nil), message.Payload...), 0x56)
	if _, err := (&shareRepairTransportMessage{message: overCap}).Marshal(); err == nil {
		t.Fatal("public package above the cap was marshaled")
	}
	// Append directly so the receive boundary is tested independently from
	// Marshal. Unmarshal must reject the shape before assigning copied slices.
	overCapWire := append(append([]byte(nil), wire...), 0x56)
	rejected := &shareRepairTransportMessage{}
	if err := rejected.Unmarshal(overCapWire); err == nil {
		t.Fatal("public package above the cap was decoded")
	}
	if rejected.message.Payload != nil || rejected.message.EphemeralPublicKey != nil {
		t.Fatal("rejected public package populated retained message slices")
	}
}

func TestShareRepairPublicPackageProductionScale100SeatCap(t *testing.T) {
	verifyingShares := make(map[string]string, 100)
	for identifier := 1; identifier <= 100; identifier++ {
		// Rust's bridge representation intentionally carries the 32-byte FROST
		// identifier as a JSON-string-wrapped hex string. Preserve the quotes
		// here so JSON map-key escaping is included in the launch-gate size.
		wireIdentifier := fmt.Sprintf("\"%064x\"", identifier)
		verifyingShares[wireIdentifier] = "03" + strings.Repeat("f", 64)
	}
	publicPackage, err := json.Marshal(&NativeFROSTPublicKeyPackage{
		VerifyingShares: verifyingShares,
		VerifyingKey:    strings.Repeat("f", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(publicPackage) >= shareRepairMaximumPublicPayload {
		t.Fatalf(
			"100-seat native public package is [%d] bytes, cap is [%d]",
			len(publicPackage),
			shareRepairMaximumPublicPayload,
		)
	}
	wire, err := (&shareRepairTransportMessage{message: shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        1,
		ContextDigest: [32]byte{0x44},
		Payload:       publicPackage,
	}}).Marshal()
	if err != nil {
		t.Fatalf("100-seat native public package exceeded transport shape: %v", err)
	}
	decoded := &shareRepairTransportMessage{}
	if err := decoded.Unmarshal(wire); err != nil {
		t.Fatalf("100-seat native public package failed transport decoding: %v", err)
	}
	if !bytes.Equal(decoded.message.Payload, publicPackage) {
		t.Fatal("100-seat native public package changed across transport")
	}
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
		map[group.MemberIndex]struct{}{1: {}, 2: {}},
		[32]byte{0x44},
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
		map[group.MemberIndex]struct{}{1: {}, 2: {}},
		[32]byte{0x44},
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

	wrongContext := message
	wrongContext.ContextDigest = [32]byte{0x45}
	bus.handleMessage(fakeNetMessage{
		senderPublicKey: fixture.operatorA,
		payload:         &shareRepairTransportMessage{message: wrongContext},
	})
	select {
	case <-stream:
		t.Fatal("wrong-context share-repair message was delivered")
	default:
	}
	bus.subscribers[0].mutex.Lock()
	if bus.subscribers[0].acceptedBytes != 0 {
		bus.subscribers[0].mutex.Unlock()
		t.Fatal("wrong-context message consumed subscriber budget")
	}
	bus.subscribers[0].mutex.Unlock()

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

func TestShareRepairBusRejectsAuthenticatedNonparticipantBeforeDelivery(t *testing.T) {
	fixture := newRunnerBusAuthFixture(t, 8)
	busInterface, err := newBroadcastChannelShareRepairBus(
		context.Background(),
		&testutils.MockLogger{},
		&immediateRecvBroadcastChannel{},
		fixture.validator,
		map[group.MemberIndex]struct{}{1: {}, 2: {}},
		[32]byte{0x44},
	)
	if err != nil {
		t.Fatal(err)
	}
	bus := busInterface.(*broadcastChannelShareRepairBus)
	stream := bus.Subscribe(group.MemberIndex(2))

	// Operator A legitimately owns both seat 1 and seat 3 in the wallet, but
	// this repair authorization names only seats 1 and 2. Seat 3 must be
	// rejected before it consumes subscriber capacity.
	nonparticipant := &shareRepairTransportMessage{message: shareRepairMessage{
		Type:          shareRepairDeltaMessage,
		Sender:        3,
		Recipient:     2,
		ContextDigest: [32]byte{0x44},
		Payload:       bytes.Repeat([]byte{0x31}, shareRepairEncryptedScalarPayloadLength),
	}}
	bus.handleMessage(fakeNetMessage{
		senderPublicKey: fixture.operatorA,
		payload:         nonparticipant,
	})
	select {
	case <-stream:
		t.Fatal("authenticated nonparticipant frame was delivered")
	default:
	}

	// Early phase frames from an exact participant remain valid while another
	// subscriber may still be collecting announcements.
	participant := &shareRepairTransportMessage{message: shareRepairMessage{
		Type:          shareRepairDeltaMessage,
		Sender:        1,
		Recipient:     2,
		ContextDigest: [32]byte{0x44},
		Payload:       bytes.Repeat([]byte{0x32}, shareRepairEncryptedScalarPayloadLength),
	}}
	bus.handleMessage(fakeNetMessage{
		senderPublicKey: fixture.operatorA,
		payload:         participant,
	})
	select {
	case received := <-stream:
		if received.Sender != 1 || received.Type != shareRepairDeltaMessage {
			t.Fatalf("unexpected participant frame: %+v", received)
		}
	default:
		t.Fatal("authorized early phase frame was not delivered")
	}
}

func TestShareRepairSubscriberPerSenderByteBudget(t *testing.T) {
	subscriber := &shareRepairBusSubscriber{
		member:                1,
		stream:                make(chan shareRepairMessage, 4),
		seen:                  make(map[[32]byte]struct{}),
		acceptedBytesBySender: make(map[group.MemberIndex]int),
	}
	contextDigest := [32]byte{0x44}
	publicPackage := shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        2,
		ContextDigest: contextDigest,
		Payload:       bytes.Repeat([]byte{0x41}, shareRepairMaximumPublicPayload),
	}
	installed := shareRepairMessage{
		Type:          shareRepairInstalledMessage,
		Sender:        2,
		ContextDigest: contextDigest,
		Payload:       bytes.Repeat([]byte{0x42}, shareRepairMaximumSecretPayload),
	}
	subscriber.deliver(publicPackage, 4096)
	subscriber.deliver(installed, 4096)
	if subscriber.acceptedBytes != shareRepairMaximumSessionBytesPerSender ||
		subscriber.acceptedBytesBySender[2] != shareRepairMaximumSessionBytesPerSender ||
		len(subscriber.stream) != 2 {
		t.Fatalf(
			"subscriber accepted [%d]/[%d] bytes and [%d] messages",
			subscriber.acceptedBytes,
			subscriber.acceptedBytesBySender[2],
			len(subscriber.stream),
		)
	}
	subscriber.deliver(shareRepairMessage{
		Type:          shareRepairCompletionMessage,
		Sender:        2,
		ContextDigest: contextDigest,
		Payload:       bytes.Repeat([]byte{0x43}, sha256.Size),
	}, 4096)
	if subscriber.acceptedBytes != shareRepairMaximumSessionBytesPerSender ||
		len(subscriber.stream) != 2 {
		t.Fatal("over-budget sender consumed subscriber capacity")
	}
}

func TestShareRepairSubscriberTotalByteBudget(t *testing.T) {
	subscriber := &shareRepairBusSubscriber{
		member:                200,
		stream:                make(chan shareRepairMessage, 256),
		seen:                  make(map[[32]byte]struct{}),
		acceptedBytesBySender: make(map[group.MemberIndex]int),
	}
	contextDigest := [32]byte{0x44}
	for rawSender := 1; rawSender <= 102; rawSender++ {
		sender := group.MemberIndex(rawSender)
		subscriber.deliver(shareRepairMessage{
			Type:          shareRepairPublicPackageMessage,
			Sender:        sender,
			ContextDigest: contextDigest,
			Payload:       bytes.Repeat([]byte{byte(sender)}, shareRepairMaximumPublicPayload),
		}, 4096)
		subscriber.deliver(shareRepairMessage{
			Type:          shareRepairInstalledMessage,
			Sender:        sender,
			ContextDigest: contextDigest,
			Payload:       bytes.Repeat([]byte{byte(sender)}, shareRepairMaximumSecretPayload),
		}, 4096)
	}
	remaining := shareRepairMaximumSessionBytes - subscriber.acceptedBytes
	if remaining <= 0 || remaining > shareRepairMaximumPublicPayload {
		t.Fatalf("unexpected remaining total budget [%d]", remaining)
	}
	subscriber.deliver(shareRepairMessage{
		Type:          shareRepairPublicPackageMessage,
		Sender:        103,
		ContextDigest: contextDigest,
		Payload:       bytes.Repeat([]byte{0x67}, remaining),
	}, 4096)
	if subscriber.acceptedBytes != shareRepairMaximumSessionBytes {
		t.Fatalf("subscriber accepted [%d] total bytes", subscriber.acceptedBytes)
	}
	acceptedMessages := len(subscriber.stream)
	subscriber.deliver(shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             104,
		ContextDigest:      contextDigest,
		EphemeralPublicKey: bytes.Repeat([]byte{0x02}, shareRepairEphemeralPublicKeyLength),
	}, 4096)
	if subscriber.acceptedBytes != shareRepairMaximumSessionBytes ||
		len(subscriber.stream) != acceptedMessages {
		t.Fatal("message above the total subscriber budget was retained")
	}
}

func TestShareRepairSubscriberDuplicateAndFullStreamDoNotCharge(t *testing.T) {
	subscriber := &shareRepairBusSubscriber{
		member:                3,
		stream:                make(chan shareRepairMessage, 1),
		seen:                  make(map[[32]byte]struct{}),
		acceptedBytesBySender: make(map[group.MemberIndex]int),
	}
	first := shareRepairMessage{
		Type:               shareRepairAnnouncementMessage,
		Sender:             1,
		ContextDigest:      [32]byte{0x44},
		EphemeralPublicKey: bytes.Repeat([]byte{0x02}, shareRepairEphemeralPublicKeyLength),
	}
	second := first
	second.Sender = 2
	second.EphemeralPublicKey = bytes.Repeat(
		[]byte{0x03},
		shareRepairEphemeralPublicKeyLength,
	)
	subscriber.deliver(first, 4096)
	subscriber.deliver(first, 4096)
	subscriber.deliver(second, 4096)
	if subscriber.acceptedBytes != shareRepairEphemeralPublicKeyLength ||
		subscriber.acceptedBytesBySender[2] != 0 || len(subscriber.stream) != 1 {
		t.Fatal("duplicate or full-stream delivery consumed subscriber budget")
	}
	if _, seen := subscriber.seen[second.contentHash()]; seen {
		t.Fatal("full-stream delivery was marked as seen")
	}
	<-subscriber.stream
	subscriber.deliver(second, 4096)
	if subscriber.acceptedBytes != 2*shareRepairEphemeralPublicKeyLength ||
		subscriber.acceptedBytesBySender[2] != shareRepairEphemeralPublicKeyLength ||
		len(subscriber.stream) != 1 {
		t.Fatal("previously full-stream delivery could not be retried")
	}
}

func TestShareRepairSubscriberConcurrentDeliveryIsRaceSafe(t *testing.T) {
	const senderCount = 64
	const messagesPerSender = 8
	subscriber := &shareRepairBusSubscriber{
		member:                100,
		stream:                make(chan shareRepairMessage, senderCount*messagesPerSender),
		seen:                  make(map[[32]byte]struct{}),
		acceptedBytesBySender: make(map[group.MemberIndex]int),
	}
	var waitGroup sync.WaitGroup
	for rawSender := 1; rawSender <= senderCount; rawSender++ {
		sender := group.MemberIndex(rawSender)
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for sequence := 0; sequence < messagesPerSender; sequence++ {
				payload := make([]byte, sha256.Size)
				payload[0] = byte(sender)
				payload[1] = byte(sequence)
				subscriber.deliver(shareRepairMessage{
					Type:          shareRepairCompletionMessage,
					Sender:        sender,
					ContextDigest: [32]byte{0x44},
					Payload:       payload,
				}, 4096)
			}
		}()
	}
	waitGroup.Wait()
	expectedMessages := senderCount * messagesPerSender
	expectedBytes := expectedMessages * sha256.Size
	if len(subscriber.stream) != expectedMessages ||
		subscriber.acceptedBytes != expectedBytes {
		t.Fatalf(
			"concurrent delivery retained [%d] messages and [%d] bytes",
			len(subscriber.stream),
			subscriber.acceptedBytes,
		)
	}
}
