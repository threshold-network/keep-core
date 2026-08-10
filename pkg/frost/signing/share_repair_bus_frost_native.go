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
	shareRepairAnnouncementMessage  shareRepairMessageType = 1
	shareRepairDeltaMessage         shareRepairMessageType = 2
	shareRepairSigmaMessage         shareRepairMessageType = 3
	shareRepairInstalledMessage     shareRepairMessageType = 4
	shareRepairPublicPackageMessage shareRepairMessageType = 5
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
	Broadcast(shareRepairMessage)
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

func (bus *inProcessShareRepairBus) Broadcast(message shareRepairMessage) {
	bus.mutex.Lock()
	subscribers := append([]*shareRepairBusSubscriber(nil), bus.subscribers...)
	bus.mutex.Unlock()
	for _, subscriber := range subscribers {
		subscriber.deliver(message, 4096)
	}
}

const shareRepairTransportType = "frost/share_repair/v1"

const (
	shareRepairEphemeralPublicKeyLength = 33
	shareRepairMaximumSecretPayload     = 4 * 1024
	shareRepairMaximumPublicPayload     = 256 * 1024
)

type shareRepairTransportMessage struct {
	message shareRepairMessage
}

func (*shareRepairTransportMessage) Type() string { return shareRepairTransportType }

// Marshal encodes type(1) || sender(4) || recipient(4) || context(32) ||
// ephemeral-key-length(2) || ephemeral-key || payload.
func (message *shareRepairTransportMessage) Marshal() ([]byte, error) {
	value := message.message
	if value.Type < shareRepairAnnouncementMessage || value.Type > shareRepairPublicPackageMessage ||
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
	default:
		if value.Recipient == 0 || len(value.EphemeralPublicKey) != 0 ||
			len(value.Payload) == 0 || len(value.Payload) > shareRepairMaximumSecretPayload {
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
	if messageType < shareRepairAnnouncementMessage || messageType > shareRepairPublicPackageMessage ||
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
	default:
		if rawRecipient == 0 || ephemeralLength != 0 || len(value.Payload) == 0 ||
			len(value.Payload) > shareRepairMaximumSecretPayload {
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
	mutex               sync.Mutex
	subscribers         []*shareRepairBusSubscriber
	startOnce           sync.Once
}

func newBroadcastChannelShareRepairBus(
	ctx context.Context,
	logger log.StandardLogger,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
) (shareRepairBus, error) {
	if ctx == nil || channel == nil || membershipValidator == nil {
		return nil, fmt.Errorf("share-repair bus dependencies are incomplete")
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

func (bus *broadcastChannelShareRepairBus) Broadcast(message shareRepairMessage) {
	// Deliver locally first. A channel implementation may not echo the sender;
	// the content hash suppresses a later network echo.
	bus.deliver(message)
	if err := bus.channel.Send(
		bus.ctx,
		&shareRepairTransportMessage{message: message},
	); err != nil {
		bus.logger.Warnf("share-repair bus send failed: [%v]", err)
	}
}

func (bus *broadcastChannelShareRepairBus) handleMessage(message net.Message) {
	wire, ok := message.Payload().(*shareRepairTransportMessage)
	if !ok {
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
