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
	member group.MemberIndex
	stream chan shareRepairMessage
	mutex  sync.Mutex
	seen   map[[32]byte]struct{}
}

func (subscriber *shareRepairBusSubscriber) deliver(
	message shareRepairMessage,
	seenBound int,
) {
	if message.Recipient != 0 && message.Recipient != subscriber.member {
		return
	}
	hash := message.contentHash()
	delivered := message
	delivered.EphemeralPublicKey = append([]byte(nil), message.EphemeralPublicKey...)
	delivered.Payload = append([]byte(nil), message.Payload...)
	subscriber.mutex.Lock()
	defer subscriber.mutex.Unlock()
	if _, exists := subscriber.seen[hash]; exists {
		return
	}
	select {
	case subscriber.stream <- delivered:
		if seenBound > 0 && len(subscriber.seen) >= seenBound {
			subscriber.seen = make(map[[32]byte]struct{})
		}
		subscriber.seen[hash] = struct{}{}
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
		member: member,
		stream: make(chan shareRepairMessage, bus.bufferSize),
		seen:   make(map[[32]byte]struct{}),
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
	shareRepairMaximumPublicPayload         = 256 * 1024
)

type shareRepairTransportMessage struct {
	message shareRepairMessage
}

func (*shareRepairTransportMessage) Type() string { return shareRepairTransportType }

// Marshal encodes type(1) || sender(4) || recipient(4) || context(32) ||
// ephemeral-key-length(2) || ephemeral-key || payload.
func (message *shareRepairTransportMessage) Marshal() ([]byte, error) {
	value := message.message
	if value.Type < shareRepairAnnouncementMessage ||
		value.Type > shareRepairCompletionMessage ||
		value.Sender == 0 || value.ContextDigest == [32]byte{} ||
		len(value.Payload) > shareRepairMaximumPublicPayload {
		return nil, fmt.Errorf("share-repair transport message is invalid")
	}
	switch value.Type {
	case shareRepairAnnouncementMessage:
		if value.Recipient != 0 ||
			len(value.EphemeralPublicKey) != shareRepairEphemeralPublicKeyLength ||
			len(value.Payload) != 0 {
			return nil, fmt.Errorf("share-repair announcement shape is invalid")
		}
	case shareRepairInstalledMessage:
		if value.Recipient != 0 || len(value.EphemeralPublicKey) != 0 ||
			len(value.Payload) == 0 || len(value.Payload) > shareRepairMaximumSecretPayload {
			return nil, fmt.Errorf("share-repair installed receipt shape is invalid")
		}
	case shareRepairPublicPackageMessage:
		if value.Recipient != 0 || len(value.EphemeralPublicKey) != 0 || len(value.Payload) == 0 {
			return nil, fmt.Errorf("share-repair public-package message shape is invalid")
		}
	case shareRepairInstalledAcknowledgementMessage:
		if value.Recipient == 0 || len(value.EphemeralPublicKey) != 0 ||
			len(value.Payload) != sha256.Size {
			return nil, fmt.Errorf("share-repair installed acknowledgement shape is invalid")
		}
	case shareRepairCompletionMessage:
		if value.Recipient != 0 || len(value.EphemeralPublicKey) != 0 ||
			len(value.Payload) != sha256.Size {
			return nil, fmt.Errorf("share-repair completion shape is invalid")
		}
	case shareRepairDeltaMessage, shareRepairSigmaMessage:
		if value.Recipient == 0 || len(value.EphemeralPublicKey) != 0 ||
			len(value.Payload) != shareRepairEncryptedScalarPayloadLength {
			return nil, fmt.Errorf("share-repair secret message shape is invalid")
		}
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
	if len(data) < 43+ephemeralLength ||
		len(data)-(43+ephemeralLength) > shareRepairMaximumPublicPayload {
		return fmt.Errorf("share-repair ephemeral key is truncated")
	}
	value := shareRepairMessage{
		Type:               messageType,
		Sender:             group.MemberIndex(rawSender),
		Recipient:          group.MemberIndex(rawRecipient),
		ContextDigest:      contextDigest,
		EphemeralPublicKey: append([]byte(nil), data[43:43+ephemeralLength]...),
		Payload:            append([]byte(nil), data[43+ephemeralLength:]...),
	}
	switch messageType {
	case shareRepairAnnouncementMessage:
		if rawRecipient != 0 ||
			ephemeralLength != shareRepairEphemeralPublicKeyLength ||
			len(value.Payload) != 0 {
			return fmt.Errorf("share-repair announcement shape is invalid")
		}
	case shareRepairInstalledMessage:
		if rawRecipient != 0 || ephemeralLength != 0 || len(value.Payload) == 0 ||
			len(value.Payload) > shareRepairMaximumSecretPayload {
			return fmt.Errorf("share-repair installed receipt shape is invalid")
		}
	case shareRepairPublicPackageMessage:
		if rawRecipient != 0 || ephemeralLength != 0 || len(value.Payload) == 0 {
			return fmt.Errorf("share-repair public-package message shape is invalid")
		}
	case shareRepairInstalledAcknowledgementMessage:
		if rawRecipient == 0 || ephemeralLength != 0 || len(value.Payload) != sha256.Size {
			return fmt.Errorf("share-repair installed acknowledgement shape is invalid")
		}
	case shareRepairCompletionMessage:
		if rawRecipient != 0 || ephemeralLength != 0 || len(value.Payload) != sha256.Size {
			return fmt.Errorf("share-repair completion shape is invalid")
		}
	case shareRepairDeltaMessage, shareRepairSigmaMessage:
		if rawRecipient == 0 || ephemeralLength != 0 || len(value.Payload) == 0 ||
			len(value.Payload) != shareRepairEncryptedScalarPayloadLength {
			return fmt.Errorf("share-repair secret message shape is invalid")
		}
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
) (shareRepairBus, error) {
	if ctx == nil || channel == nil || membershipValidator == nil ||
		len(participants) == 0 {
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
	}, nil
}

func (bus *broadcastChannelShareRepairBus) Subscribe(
	member group.MemberIndex,
) <-chan shareRepairMessage {
	subscriber := &shareRepairBusSubscriber{
		member: member,
		stream: make(chan shareRepairMessage, 1024),
		seen:   make(map[[32]byte]struct{}),
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
