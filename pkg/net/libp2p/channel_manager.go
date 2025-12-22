package libp2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/retransmission"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubtc "github.com/libp2p/go-libp2p-pubsub/timecache"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

const (
	libp2pPeerOutboundQueueSize = 256
	libp2pValidationQueueSize   = 4096
	// libp2pSeenMessagesTTL is the time-to-live used for pubsub seen messages
	// cache. Once a message is received and validated, pubsub re-broadcasts it
	// to other peers and puts it into the seen messages cache. This way,
	// subsequent arrivals of the same message are not re-broadcasted
	// unnecessarily. This mechanism is important for the network to avoid
	// excessive message flooding. The default value used by libp2p is 2 minutes.
	// However, Keep client messaging sessions are quite time-consuming so,
	// we use a longer TTL to reduce flooding risk even further. Worth noting
	// that this time cannot be too long as the cache may grow excessively and
	// impact memory consumption.
	libp2pSeenMessagesTTL = 5 * time.Minute
)

type channelManager struct {
	ctx context.Context

	identity  *identity
	peerStore peerstore.Peerstore

	channelsMutex sync.Mutex
	channels      map[string]*channel

	pubsub *pubsub.PubSub

	retransmissionTicker *retransmission.Ticker

	forwardersMutex sync.Mutex
	forwarders      map[string]pubsub.RelayCancelFunc

	topicsMutex sync.Mutex
	topics      map[string]*pubsub.Topic

	// metricsRecorder is optional and used for recording performance metrics
	metricsRecorder interface {
		IncrementCounter(name string, value float64)
		SetGauge(name string, value float64)
		RecordDuration(name string, duration time.Duration)
	}
}

func newChannelManager(
	ctx context.Context,
	identity *identity,
	p2phost host.Host,
	retransmissionTicker *retransmission.Ticker,
) (*channelManager, error) {
	floodsub, err := pubsub.NewFloodSub(
		ctx,
		p2phost,
		pubsub.WithMessageAuthor(identity.id),
		pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
		pubsub.WithPeerOutboundQueueSize(libp2pPeerOutboundQueueSize),
		pubsub.WithValidateQueueSize(libp2pValidationQueueSize),
		pubsub.WithSeenMessagesStrategy(pubsubtc.Strategy_LastSeen),
		pubsub.WithSeenMessagesTTL(libp2pSeenMessagesTTL),
	)
	if err != nil {
		return nil, err
	}
	return &channelManager{
		channels:             make(map[string]*channel),
		pubsub:               floodsub,
		peerStore:            p2phost.Peerstore(),
		identity:             identity,
		ctx:                  ctx,
		retransmissionTicker: retransmissionTicker,
		forwarders:           make(map[string]pubsub.RelayCancelFunc),
		topics:               make(map[string]*pubsub.Topic),
	}, nil
}

func (cm *channelManager) getChannel(name string) (*channel, error) {
	var (
		channel *channel
		exists  bool
		err     error
	)

	cm.channelsMutex.Lock()
	channel, exists = cm.channels[name]
	cm.channelsMutex.Unlock()

	if !exists {
		// Ensure we update our cache of known channels
		cm.channelsMutex.Lock()
		defer cm.channelsMutex.Unlock()

		channel, exists = cm.channels[name]
		if exists {
			return channel, nil
		}

		channel, err = cm.newChannel(name)
		if err != nil {
			return nil, err
		}

		cm.channels[name] = channel
		// Wire metrics recorder into channel if available
		if cm.metricsRecorder != nil {
			channel.setMetricsRecorder(cm.metricsRecorder)
		}
	}

	return channel, nil
}

// setMetricsRecorder sets the metrics recorder for the channel manager
// and wires it into existing channels.
func (cm *channelManager) setMetricsRecorder(recorder interface {
	IncrementCounter(name string, value float64)
	SetGauge(name string, value float64)
	RecordDuration(name string, duration time.Duration)
}) {
	cm.metricsRecorder = recorder
	// Wire metrics into existing channels
	cm.channelsMutex.Lock()
	defer cm.channelsMutex.Unlock()
	for _, channel := range cm.channels {
		channel.setMetricsRecorder(recorder)
	}
}

func (cm *channelManager) newChannel(name string) (*channel, error) {
	topic, err := cm.getTopic(name)
	if err != nil {
		return nil, fmt.Errorf(
			"could not get topic [%v] handle: [%v]",
			name,
			err,
		)
	}

	subscription, err := topic.Subscribe()
	if err != nil {
		return nil, fmt.Errorf(
			"could not subscribe topic [%v]: [%v]",
			name,
			err,
		)
	}

	channel := &channel{
		name:                 name,
		clientIdentity:       cm.identity,
		peerStore:            cm.peerStore,
		validator:            cm.pubsub,
		publisher:            topic,
		subscription:         subscription,
		incomingMessageQueue: make(chan *pubsub.Message, incomingMessageThrottle),
		messageHandlers:      make([]*messageHandler, 0),
		unmarshalersByType:   make(map[string]func() net.TaggedUnmarshaler),
		retransmissionTicker: cm.retransmissionTicker,
	}

	go channel.handleMessages(cm.ctx)

	return channel, nil
}

func (cm *channelManager) newForwarder(name string, ttl time.Duration) error {
	cm.forwardersMutex.Lock()
	defer cm.forwardersMutex.Unlock()

	if _, ok := cm.forwarders[name]; !ok {
		topic, err := cm.getTopic(name)
		if err != nil {
			return fmt.Errorf(
				"could not get topic [%v] handle: [%v]",
				name,
				err,
			)
		}

		cancelFn, err := topic.Relay()
		if err != nil {
			return fmt.Errorf(
				"could not enable relay for topic [%v]: [%v]",
				name,
				err,
			)
		}

		go func() {
			ctx, cancelCtx := context.WithTimeout(cm.ctx, ttl)
			defer cancelCtx()

			<-ctx.Done()
			cm.shutdownForwarder(name)
		}()

		cm.forwarders[name] = cancelFn
	}

	return nil
}

func (cm *channelManager) shutdownForwarder(name string) {
	cm.forwardersMutex.Lock()
	defer cm.forwardersMutex.Unlock()

	logger.Infof("shutting down message forwarder for channel: [%v]", name)

	cancelFn, ok := cm.forwarders[name]

	if !ok {
		return
	}

	cancelFn()
	delete(cm.forwarders, name)
}

func (cm *channelManager) getTopic(name string) (*pubsub.Topic, error) {
	var (
		topic  *pubsub.Topic
		exists bool
		err    error
	)

	cm.topicsMutex.Lock()
	topic, exists = cm.topics[name]
	cm.topicsMutex.Unlock()

	if !exists {
		// Ensure we update our cache of known topics.
		cm.topicsMutex.Lock()
		defer cm.topicsMutex.Unlock()

		topic, exists = cm.topics[name]
		if exists {
			return topic, nil
		}

		topic, err = cm.pubsub.Join(name)
		if err != nil {
			return nil, err
		}

		cm.topics[name] = topic
	}

	return topic, nil
}
