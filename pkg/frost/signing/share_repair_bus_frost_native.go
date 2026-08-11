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

type shareRepairMessageType uint8

const (
	shareRepairAnnouncementMessage             shareRepairMessageType = 1
	shareRepairDeltaMessage                    shareRepairMessageType = 2
	shareRepairSigmaMessage                    shareRepairMessageType = 3
	shareRepairInstalledMessage                shareRepairMessageType = 4
	shareRepairPublicPackageMessage            shareRepairMessageType = 5
	shareRepairInstalledAcknowledgementMessage shareRepairMessageType = 6
	shareRepairCompletionMessage               shareRepairMessageType = 7
)

type shareRepairMessage struct {
	Type               shareRepairMessageType
	Sender             group.MemberIndex
	Recipient          group.MemberIndex
	ContextDigest      [32]byte
	EphemeralPublicKey []byte
	Payload            []byte
}

func (message shareRepairMessage) contentHash() [32]byte {
	hasher := sha256.New()
	hasher.Write([]byte{byte(message.Type), byte(message.Sender), byte(message.Recipient)})
	hasher.Write(message.ContextDigest[:])
	hasher.Write(message.EphemeralPublicKey)
	hasher.Write(message.Payload)
	result := [32]byte{}
	copy(result[:], hasher.Sum(nil))
	return result
}

type shareRepairBus interface {
	Subscribe(group.MemberIndex) <-chan shareRepairMessage
	Start()
	// Broadcast delivers the message and returns a function that stops its
	// network retransmissions without canceling any other protocol message.
	Broadcast(shareRepairMessage) context.CancelFunc
}

type shareRepairBusSubscriber struct {
	member                group.MemberIndex
	stream                chan shareRepairMessage
	mutex                 sync.Mutex
	seen                  map[[32]byte]struct{}
	acceptedBytes         int
	acceptedBytesBySender map[group.MemberIndex]int
}

func (subscriber *shareRepairBusSubscriber) deliver(
	message shareRepairMessage,
	seenBound int,
) {
	if message.Recipient != 0 && message.Recipient != subscriber.member {
		return
	}
	if err := validateShareRepairMessage(message); err != nil {
		return
	}
	hash := message.contentHash()
	messageBytes := shareRepairMessageRetainedBytes(message)
	subscriber.mutex.Lock()
	defer subscriber.mutex.Unlock()
	if subscriber.seen == nil {
		subscriber.seen = make(map[[32]byte]struct{})
	}
	if _, exists := subscriber.seen[hash]; exists {
		return
	}
	if messageBytes > shareRepairMaximumSessionBytesPerSender-
		subscriber.acceptedBytesBySender[message.Sender] ||
		messageBytes > shareRepairMaximumSessionBytes-subscriber.acceptedBytes {
		return
	}
	// Check capacity before retaining attacker-controlled slices. Deliver is
	// serialized per subscriber, so no other producer can fill this stream
	// between this check and the non-blocking send below.
	if len(subscriber.stream) >= cap(subscriber.stream) {
		return
	}
	delivered := message
	delivered.EphemeralPublicKey = append([]byte(nil), message.EphemeralPublicKey...)
	delivered.Payload = append([]byte(nil), message.Payload...)
	select {
	case subscriber.stream <- delivered:
		if seenBound > 0 && len(subscriber.seen) >= seenBound {
			subscriber.seen = make(map[[32]byte]struct{})
		}
		subscriber.seen[hash] = struct{}{}
		if subscriber.acceptedBytesBySender == nil {
			subscriber.acceptedBytesBySender = make(map[group.MemberIndex]int)
		}
		subscriber.acceptedBytes += messageBytes
		subscriber.acceptedBytesBySender[message.Sender] += messageBytes
	default:
		// Honest traffic is O(threshold); a full stream means flooding. Drop the
		// newest and let the bounded recovery context time out fail-closed.
	}
}

type inProcessShareRepairBus struct {
	mutex       sync.Mutex
	subscribers []*shareRepairBusSubscriber
	bufferSize  int
}

func newInProcessShareRepairBus(bufferSize int) shareRepairBus {
	if bufferSize < 1 {
		bufferSize = 1
	}
	return &inProcessShareRepairBus{bufferSize: bufferSize}
}

func (bus *inProcessShareRepairBus) Subscribe(member group.MemberIndex) <-chan shareRepairMessage {
	subscriber := &shareRepairBusSubscriber{
		member:                member,
		stream:                make(chan shareRepairMessage, bus.bufferSize),
		seen:                  make(map[[32]byte]struct{}),
		acceptedBytesBySender: make(map[group.MemberIndex]int),
	}
	bus.mutex.Lock()
	bus.subscribers = append(bus.subscribers, subscriber)
	bus.mutex.Unlock()
	return subscriber.stream
}

func (*inProcessShareRepairBus) Start() {}

func (bus *inProcessShareRepairBus) Broadcast(
	message shareRepairMessage,
) context.CancelFunc {
	bus.mutex.Lock()
	subscribers := append([]*shareRepairBusSubscriber(nil), bus.subscribers...)
	bus.mutex.Unlock()
	for _, subscriber := range subscribers {
		subscriber.deliver(message, 4096)
	}
	return func() {}
}

const shareRepairTransportType = "frost/share_repair/v1"

const (
	shareRepairEphemeralPublicKeyLength = 33
	// A native repair envelope is compressed ephemeral SEC1 (33), XChaCha20
	// nonce (24), encrypted scalar (32), and Poly1305 tag (16). Keeping this
	// exact at the transport boundary prevents a stale or custom engine from
	// putting plaintext scalars on the wire.
	shareRepairEncryptedScalarPayloadLength = 33 + 24 + 32 + 16
	shareRepairMaximumSecretPayload         = 4 * 1024
	// A 100-seat native public-key package currently serializes below 14 KiB.
	// Sixteen KiB leaves format headroom without allowing one frame to dominate
	// the maintenance process's receive queues.
	shareRepairMaximumPublicPayload = 16 * 1024
	// One honest helper contributes one maximum public package, one encrypted
	// scalar to a given local seat, its announcement, and at most one receipt
	// acknowledgement. Twenty KiB covers that traffic with room to spare; the
	// aggregate cap covers the complete 100-seat authorization.
	shareRepairMaximumSessionBytesPerSender = 20 * 1024
	shareRepairMaximumSessionBytes          = 2 * 1024 * 1024
	shareRepairSubscriberStreamBuffer       = 1024
)

func shareRepairMessageRetainedBytes(message shareRepairMessage) int {
	return len(message.EphemeralPublicKey) + len(message.Payload)
}

func validateShareRepairMessage(message shareRepairMessage) error {
	if message.Type < shareRepairAnnouncementMessage ||
		message.Type > shareRepairCompletionMessage ||
		message.Sender == 0 || message.ContextDigest == [32]byte{} {
		return fmt.Errorf("share-repair transport message is invalid")
	}
	return validateShareRepairMessageShape(
		message.Type,
		message.Recipient,
		len(message.EphemeralPublicKey),
		len(message.Payload),
	)
}

func validateShareRepairMessageShape(
	messageType shareRepairMessageType,
	recipient group.MemberIndex,
	ephemeralLength int,
	payloadLength int,
) error {
	switch messageType {
	case shareRepairAnnouncementMessage:
		if recipient != 0 ||
			ephemeralLength != shareRepairEphemeralPublicKeyLength ||
			payloadLength != 0 {
			return fmt.Errorf("share-repair announcement shape is invalid")
		}
	case shareRepairInstalledMessage:
		if recipient != 0 || ephemeralLength != 0 || payloadLength == 0 ||
			payloadLength > shareRepairMaximumSecretPayload {
			return fmt.Errorf("share-repair installed receipt shape is invalid")
		}
	case shareRepairPublicPackageMessage:
		if recipient != 0 || ephemeralLength != 0 || payloadLength == 0 ||
			payloadLength > shareRepairMaximumPublicPayload {
			return fmt.Errorf("share-repair public-package message shape is invalid")
		}
	case shareRepairInstalledAcknowledgementMessage:
		if recipient == 0 || ephemeralLength != 0 || payloadLength != sha256.Size {
			return fmt.Errorf("share-repair installed acknowledgement shape is invalid")
		}
	case shareRepairCompletionMessage:
		if recipient != 0 || ephemeralLength != 0 || payloadLength != sha256.Size {
			return fmt.Errorf("share-repair completion shape is invalid")
		}
	case shareRepairDeltaMessage, shareRepairSigmaMessage:
		if recipient == 0 || ephemeralLength != 0 ||
			payloadLength != shareRepairEncryptedScalarPayloadLength {
			return fmt.Errorf("share-repair secret message shape is invalid")
		}
	default:
		return fmt.Errorf("share-repair transport message is invalid")
	}
	return nil
}

type shareRepairTransportMessage struct {
	message shareRepairMessage
}

func (*shareRepairTransportMessage) Type() string { return shareRepairTransportType }

// Marshal encodes type(1) || sender(4) || recipient(4) || context(32) ||
// ephemeral-key-length(2) || ephemeral-key || payload.
func (message *shareRepairTransportMessage) Marshal() ([]byte, error) {
	value := message.message
	if err := validateShareRepairMessage(value); err != nil {
		return nil, err
	}
	result := make([]byte, 43+len(value.EphemeralPublicKey)+len(value.Payload))
	result[0] = byte(value.Type)
	binary.BigEndian.PutUint32(result[1:5], uint32(value.Sender))
	binary.BigEndian.PutUint32(result[5:9], uint32(value.Recipient))
	copy(result[9:41], value.ContextDigest[:])
	binary.BigEndian.PutUint16(result[41:43], uint16(len(value.EphemeralPublicKey)))
	offset := 43
	offset += copy(result[offset:], value.EphemeralPublicKey)
	copy(result[offset:], value.Payload)
	return result, nil
}

func (message *shareRepairTransportMessage) Unmarshal(data []byte) error {
	if len(data) < 43 {
		return fmt.Errorf("share-repair transport message is truncated")
	}
	messageType := shareRepairMessageType(data[0])
	rawSender := binary.BigEndian.Uint32(data[1:5])
	rawRecipient := binary.BigEndian.Uint32(data[5:9])
	if messageType < shareRepairAnnouncementMessage ||
		messageType > shareRepairCompletionMessage ||
		rawSender == 0 || rawSender > uint32(group.MaxMemberIndex) ||
		rawRecipient > uint32(group.MaxMemberIndex) {
		return fmt.Errorf("share-repair transport header is invalid")
	}
	contextDigest := [32]byte{}
	copy(contextDigest[:], data[9:41])
	if contextDigest == [32]byte{} {
		return fmt.Errorf("share-repair context digest is zero")
	}
	ephemeralLength := int(binary.BigEndian.Uint16(data[41:43]))
	if len(data) < 43+ephemeralLength {
		return fmt.Errorf("share-repair ephemeral key is truncated")
	}
	payloadLength := len(data) - (43 + ephemeralLength)
	if err := validateShareRepairMessageShape(
		messageType,
		group.MemberIndex(rawRecipient),
		ephemeralLength,
		payloadLength,
	); err != nil {
		return err
	}
	value := shareRepairMessage{
		Type:               messageType,
		Sender:             group.MemberIndex(rawSender),
		Recipient:          group.MemberIndex(rawRecipient),
		ContextDigest:      contextDigest,
		EphemeralPublicKey: append([]byte(nil), data[43:43+ephemeralLength]...),
		Payload:            append([]byte(nil), data[43+ephemeralLength:]...),
	}
	message.message = value
	return nil
}

type broadcastChannelShareRepairBus struct {
	ctx                 context.Context
	logger              log.StandardLogger
	channel             net.BroadcastChannel
	membershipValidator *group.MembershipValidator
	participants        map[group.MemberIndex]struct{}
	expectedContext     [32]byte
	mutex               sync.Mutex
	subscribers         []*shareRepairBusSubscriber
	startOnce           sync.Once
}

func newBroadcastChannelShareRepairBus(
	ctx context.Context,
	logger log.StandardLogger,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	participants map[group.MemberIndex]struct{},
	expectedContext [32]byte,
) (shareRepairBus, error) {
	if ctx == nil || channel == nil || membershipValidator == nil ||
		len(participants) == 0 || expectedContext == [32]byte{} {
		return nil, fmt.Errorf("share-repair bus dependencies are incomplete")
	}
	participantCopy := make(map[group.MemberIndex]struct{}, len(participants))
	for participant := range participants {
		if participant == 0 || participant > group.MaxMemberIndex {
			return nil, fmt.Errorf("share-repair participant [%d] is invalid", participant)
		}
		participantCopy[participant] = struct{}{}
	}
	if logger == nil {
		logger = log.Logger("frost-share-repair-bus")
	}
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &shareRepairTransportMessage{}
	})
	return &broadcastChannelShareRepairBus{
		ctx:                 ctx,
		logger:              logger,
		channel:             channel,
		membershipValidator: membershipValidator,
		participants:        participantCopy,
		expectedContext:     expectedContext,
	}, nil
}

func (bus *broadcastChannelShareRepairBus) Subscribe(
	member group.MemberIndex,
) <-chan shareRepairMessage {
	subscriber := &shareRepairBusSubscriber{
		member:                member,
		stream:                make(chan shareRepairMessage, shareRepairSubscriberStreamBuffer),
		seen:                  make(map[[32]byte]struct{}),
		acceptedBytesBySender: make(map[group.MemberIndex]int),
	}
	bus.mutex.Lock()
	bus.subscribers = append(bus.subscribers, subscriber)
	bus.mutex.Unlock()
	return subscriber.stream
}

func (bus *broadcastChannelShareRepairBus) Start() {
	bus.startOnce.Do(func() { bus.channel.Recv(bus.ctx, bus.handleMessage) })
}

func (bus *broadcastChannelShareRepairBus) deliver(message shareRepairMessage) {
	if message.ContextDigest != bus.expectedContext {
		return
	}
	bus.mutex.Lock()
	subscribers := append([]*shareRepairBusSubscriber(nil), bus.subscribers...)
	bus.mutex.Unlock()
	for _, subscriber := range subscribers {
		subscriber.deliver(message, 4096)
	}
}

func (bus *broadcastChannelShareRepairBus) Broadcast(
	message shareRepairMessage,
) context.CancelFunc {
	sendContext, cancel := context.WithCancel(bus.ctx)
	// Deliver locally first. A channel implementation may not echo the sender;
	// the content hash suppresses a later network echo.
	bus.deliver(message)
	if err := bus.channel.Send(
		sendContext,
		&shareRepairTransportMessage{message: message},
	); err != nil {
		bus.logger.Warnf("share-repair bus send failed: [%v]", err)
	}
	return cancel
}

func (bus *broadcastChannelShareRepairBus) handleMessage(message net.Message) {
	wire, ok := message.Payload().(*shareRepairTransportMessage)
	if !ok {
		return
	}
	if wire.message.ContextDigest != bus.expectedContext {
		return
	}
	// Full wallet membership is broader than the exact helper/target set named
	// by this recovery authorization. Drop an authenticated but unauthorized
	// wallet seat before it can consume subscriber or pre-rendezvous capacity.
	if _, participant := bus.participants[wire.message.Sender]; !participant {
		// This path is attacker-controlled by any authenticated wallet seat not
		// named in the repair certificate. Drop silently so admission control
		// cannot be repurposed into warning-log amplification.
		return
	}
	if !bus.membershipValidator.IsValidMembership(
		wire.message.Sender,
		message.SenderPublicKey(),
	) {
		bus.logger.Warnf(
			"share-repair bus dropped unauthenticated seat [%d]",
			wire.message.Sender,
		)
		return
	}
	bus.deliver(wire.message)
}
