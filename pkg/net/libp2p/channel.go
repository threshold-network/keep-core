package libp2p

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/keep-network/keep-core/pkg/operator"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/gen/pb"
	"github.com/keep-network/keep-core/pkg/net/internal"
	"github.com/keep-network/keep-core/pkg/net/retransmission"
)

var (
	subscriptionWorkers = 32
	messageWorkers      = runtime.NumCPU()
)

const (
	incomingMessageThrottle = 4096
	messageHandlerThrottle  = 512
)

type validator interface {
	RegisterTopicValidator(
		topic string,
		val interface{},
		opts ...pubsub.ValidatorOpt,
	) error

	UnregisterTopicValidator(topic string) error
}

type publisher interface {
	Publish(ctx context.Context, data []byte, opts ...pubsub.PubOpt) error
}

type channel struct {
	// channel-scoped atomic counter for sequence numbers
	//
	// Must be declared at the top of the struct!
	// See: https://golang.org/pkg/sync/atomic/#pkg-note-BUG
	counter uint64

	name string

	clientIdentity *identity
	peerStore      peerstore.Peerstore

	validatorMutex sync.Mutex
	validator      validator

	publisherMutex sync.Mutex
	publisher      publisher

	subscription         *pubsub.Subscription
	incomingMessageQueue chan *pubsub.Message

	messageHandlersMutex sync.Mutex
	messageHandlers      []*messageHandler

	unmarshalersMutex  sync.Mutex
	unmarshalersByType map[string]func() net.TaggedUnmarshaler

	retransmissionTicker *retransmission.Ticker

	// metricsRecorder is optional and used for recording performance metrics
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}
}

type messageHandler struct {
	ctx     context.Context
	channel chan net.Message
}

func (c *channel) nextSeqno() uint64 {
	return atomic.AddUint64(&c.counter, 1)
}

func (c *channel) Name() string {
	return c.name
}

func (c *channel) Send(
	ctx context.Context,
	message net.TaggedMarshaler,
	retransmissionStrategy ...net.RetransmissionStrategy,
) error {
	messageProto, err := c.messageProto(message)
	if err != nil {
		return err
	}

	messageProto.SequenceNumber = c.nextSeqno()

	doSend := func() error {
		return c.publish(messageProto)
	}

	var strategy net.RetransmissionStrategy
	switch len(retransmissionStrategy) {
	case 1:
		strategy = retransmissionStrategy[0]
	default:
		strategy = net.StandardRetransmissionStrategy
	}

	retransmission.ScheduleRetransmissions(
		ctx,
		logger,
		c.retransmissionTicker,
		doSend,
		retransmission.WithStrategy(strategy),
	)

	return doSend()
}

// TODO: The broadcast channel Recv function expects the message handler as
// a callback. However, in possibly all cases all the handler is doing is
// writing to a buffered channel. The broadcast channel has no idea what is
// happening with the receiver. If it takes too long to receive a message, the
// receiver's handler goroutine piping messages from `messageHandler.channel`
// to `handleWithRetransmissions` gets blocked. Instead of using a callback, we
// could explore using a channel. We would avoid unnecessary hop and the
// broadcast channel could be dropping messages if the receiving channel is full.
//
// See https://github.com/keep-network/keep-core/issues/3420
func (c *channel) Recv(ctx context.Context, handler func(m net.Message)) {
	messageHandler := &messageHandler{
		ctx:     ctx,
		channel: make(chan net.Message, messageHandlerThrottle),
	}

	c.messageHandlersMutex.Lock()
	c.messageHandlers = append(c.messageHandlers, messageHandler)
	c.messageHandlersMutex.Unlock()

	handleWithRetransmissions := retransmission.WithRetransmissionSupport(handler)

	// A separate goroutine controls the lifecycle of the handler. The message
	// handler is removed from the channel if the context is done. This logic is
	// placed in a separate goroutine because the call to
	// `handleWithRetransmission` is blocking and we do not want to wait with
	// removing the handler if that call blocks for a longer period of time,
	// for example, when the underlying buffered channel is full.
	go func() {
		<-ctx.Done()
		logger.Debug("context is done; removing message handler")
		c.removeHandler(messageHandler)
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case msg := <-messageHandler.channel:
				// Go language specification says that if one or more of the
				// communications in the select statement can proceed, a single
				// one that will proceed is chosen via a uniform pseudo-random
				// selection.
				// Thus, it can happen this communication is called when ctx is
				// already done. Since we guarantee in the network channel API
				// that handler is not called after ctx is done (client code
				// could e.g. perform come cleanup), we need to double-check
				// the context state here.
				if messageHandler.ctx.Err() != nil {
					continue
				}

				// TODO: If this function blocks forever, this entire goroutine
				// will be blocked forever. We should consider using a channel
				// instead of a callback receiver.
				//
				// See https://github.com/keep-network/keep-core/issues/3420
				handleWithRetransmissions(msg)
			}
		}
	}()
}

func (c *channel) removeHandler(handler *messageHandler) {
	c.messageHandlersMutex.Lock()
	defer c.messageHandlersMutex.Unlock()

	for i, h := range c.messageHandlers {
		if h.channel == handler.channel {
			c.messageHandlers[i] = c.messageHandlers[len(c.messageHandlers)-1]
			c.messageHandlers = c.messageHandlers[:len(c.messageHandlers)-1]
			break
		}
	}
}

func (c *channel) SetUnmarshaler(unmarshaler func() net.TaggedUnmarshaler) {
	tpe := unmarshaler().Type()

	c.unmarshalersMutex.Lock()
	defer c.unmarshalersMutex.Unlock()

	c.unmarshalersByType[tpe] = unmarshaler
}

func (c *channel) messageProto(
	message net.TaggedMarshaler,
) (*pb.BroadcastNetworkMessage, error) {
	payloadBytes, err := message.Marshal()
	if err != nil {
		return nil, err
	}

	senderIdentityBytes, err := c.clientIdentity.Marshal()
	if err != nil {
		return nil, err
	}

	return &pb.BroadcastNetworkMessage{
		Payload: payloadBytes,
		Sender:  senderIdentityBytes,
		Type:    []byte(message.Type()),
	}, nil
}

func (c *channel) publish(message *pb.BroadcastNetworkMessage) error {
	messageBytes, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	c.publisherMutex.Lock()
	defer c.publisherMutex.Unlock()

	publishErr := c.publisher.Publish(context.TODO(), messageBytes)
	if publishErr == nil && c.metricsRecorder != nil {
		c.metricsRecorder.IncrementCounter("message_broadcast_total", 1)
	}
	return publishErr
}

func (c *channel) handleMessages(ctx context.Context) {
	logger.Debugf("creating [%v] subscription workers", subscriptionWorkers)
	for i := 0; i < subscriptionWorkers; i++ {
		go c.subscriptionWorker(ctx)
	}

	logger.Debugf("creating [%v] message workers", messageWorkers)
	for i := 0; i < messageWorkers; i++ {
		go c.incomingMessageWorker(ctx)
	}
}

func (c *channel) subscriptionWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.subscription.Cancel()
			return
		default:
			message, err := c.subscription.Next(ctx)
			if err != nil {
				logger.Error(err)
				continue
			}

			select {
			case c.incomingMessageQueue <- message:
			default:
				logger.Warnf("message workers are too slow; dropping message")
			}
		}
	}
}

func (c *channel) incomingMessageWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.incomingMessageQueue:
			if c.metricsRecorder != nil {
				c.metricsRecorder.IncrementCounter("message_received_total", 1)
			}
			if err := c.processPubsubMessage(msg); err != nil {
				logger.Error(err)
			}
		}
	}
}

func (c *channel) processPubsubMessage(pubsubMessage *pubsub.Message) error {
	var messageProto pb.BroadcastNetworkMessage
	if err := proto.Unmarshal(pubsubMessage.Data, &messageProto); err != nil {
		return err
	}

	return c.processContainerMessage(pubsubMessage.GetFrom(), &messageProto)
}

func (c *channel) processContainerMessage(
	proposedSender peer.ID,
	message *pb.BroadcastNetworkMessage,
) error {
	// The protocol type is on the envelope; let's pull that type
	// from our map of unmarshallers.
	unmarshaled, err := c.getUnmarshalingContainerByType(string(message.Type))
	if err != nil {
		return err
	}

	if err := unmarshaled.Unmarshal(message.GetPayload()); err != nil {
		return err
	}

	// Construct an identifier from the sender.
	senderIdentifier := &identity{}
	if err := senderIdentifier.Unmarshal(message.Sender); err != nil {
		return err
	}

	// Ensure the sender wasn't tampered by:
	//     Test that the proposed sender (outer layer) matches the
	//     sender identifier we grab from the message (inner layer).
	if proposedSender != senderIdentifier.id {
		return fmt.Errorf(
			"outer layer sender [%v] does not match inner layer sender [%v]",
			proposedSender,
			senderIdentifier,
		)
	}

	operatorPublicKey, err := networkPublicKeyToOperatorPublicKey(senderIdentifier.pubKey)
	if err != nil {
		return fmt.Errorf(
			"sender [%v] with key [%v] is not of correct type",
			senderIdentifier.id,
			senderIdentifier.pubKey,
		)
	}

	operatorPublicKeyBytes := operator.MarshalUncompressed(operatorPublicKey)

	netMessage := internal.BasicMessage(
		senderIdentifier.id,
		unmarshaled,
		string(message.Type),
		operatorPublicKeyBytes,
		message.SequenceNumber,
	)

	c.deliver(netMessage)

	return nil
}

func (c *channel) getUnmarshalingContainerByType(messageType string) (net.TaggedUnmarshaler, error) {
	c.unmarshalersMutex.Lock()
	defer c.unmarshalersMutex.Unlock()

	unmarshaler, found := c.unmarshalersByType[messageType]
	if !found {
		return nil, fmt.Errorf(
			"couldn't find unmarshaler for type [%s]",
			messageType,
		)
	}

	return unmarshaler(), nil
}

func (c *channel) deliver(message net.Message) {
	c.messageHandlersMutex.Lock()
	snapshot := make([]*messageHandler, len(c.messageHandlers))
	copy(snapshot, c.messageHandlers)
	c.messageHandlersMutex.Unlock()

	for _, handler := range snapshot {
		select {
		case handler.channel <- message:
		default:
			logger.Warnf("message handler is too slow; dropping message")
		}
	}
}

func (c *channel) SetFilter(filter net.BroadcastChannelFilter) error {
	c.validatorMutex.Lock()
	defer c.validatorMutex.Unlock()

	err := c.validator.UnregisterTopicValidator(c.name)
	if err != nil {
		// That error can occur when the filter is set for the first time
		// and no prior filter exists.
		logger.Debugf(
			"could not unregister topic validator for channel [%v]: [%v]",
			c.name,
			err,
		)
	}

	return c.validator.RegisterTopicValidator(c.name, createTopicValidator(filter))
}

func createTopicValidator(filter net.BroadcastChannelFilter) pubsub.Validator {
	return func(_ context.Context, _ peer.ID, message *pubsub.Message) bool {
		authorPublicKey, err := extractPublicKey(message.GetFrom())
		if err != nil {
			logger.Warnf(
				"could not retrieve message author public key: [%v]",
				err,
			)
			return false
		}
		return filter(authorPublicKey)
	}
}

func extractPublicKey(peer peer.ID) (*operator.PublicKey, error) {
	publicKey, err := peer.ExtractPublicKey()
	if err != nil {
		return nil, err
	}

	return networkPublicKeyToOperatorPublicKey(publicKey)
}

// setMetricsRecorder sets the metrics recorder for the channel and starts
// periodic queue size monitoring.
func (c *channel) setMetricsRecorder(recorder interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	c.metricsRecorder = recorder
	// Start periodic queue size monitoring
	if recorder != nil {
		go c.monitorQueueSizes(recorder)
	}
}

// monitorQueueSizes periodically records queue sizes as metrics.
func (c *channel) monitorQueueSizes(recorder interface {
	SetGauge(name string, value float64)
}) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Stop monitoring when channel is closed (we'll use a simple ticker for now)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Record incoming message queue size
			queueSize := float64(len(c.incomingMessageQueue))
			recorder.SetGauge("incoming_message_queue_size", queueSize)

			// Record message handler queue sizes
			c.messageHandlersMutex.Lock()
			for i, handler := range c.messageHandlers {
				handlerQueueSize := float64(len(handler.channel))
				recorder.SetGauge("message_handler_queue_size", handlerQueueSize)
				_ = i // avoid unused variable
			}
			c.messageHandlersMutex.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
