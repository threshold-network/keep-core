package local

import (
	"context"
	"github.com/keep-network/keep-core/pkg/operator"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/retransmission"
)

// RetransmissionTick determines the frequency of the retransmissions done
// by the local broadcast channel implementation.
const RetransmissionTick = 50 * time.Millisecond

var broadcastChannelsMutex sync.Mutex
var broadcastChannels map[string][]*localChannel
var broadcastChannelCancels map[string][]context.CancelFunc

// getBroadcastChannel returns a BroadcastChannel designed to mediate between local
// participants. It delivers all messages sent to the channel through its
// receive channels. RecvChan on a LocalChannel creates a new receive channel
// that is returned to the caller, so that all receive channels can receive
// the message.
func getBroadcastChannel(
	name string,
	operatorPublicKey *operator.PublicKey,
) net.BroadcastChannel {
	broadcastChannelsMutex.Lock()
	defer broadcastChannelsMutex.Unlock()
	if broadcastChannels == nil {
		broadcastChannels = make(map[string][]*localChannel)
	}
	if broadcastChannelCancels == nil {
		broadcastChannelCancels = make(map[string][]context.CancelFunc)
	}

	_, exists := broadcastChannels[name]
	if !exists {
		broadcastChannels[name] = make([]*localChannel, 0)
		broadcastChannelCancels[name] = make([]context.CancelFunc, 0)
	}

	tickerCtx, cancelTicker := context.WithCancel(context.Background())
	broadcastChannelCancels[name] = append(broadcastChannelCancels[name], cancelTicker)

	identifier := randomLocalIdentifier()
	channel := &localChannel{
		name:                 name,
		identifier:           &identifier,
		operatorPublicKey:    operatorPublicKey,
		messageHandlersMutex: sync.Mutex{},
		messageHandlers:      make([]*messageHandler, 0),
		unmarshalersMutex:    sync.Mutex{},
		unmarshalersByType:   make(map[string]func() net.TaggedUnmarshaler, 0),
		retransmissionTicker: retransmission.NewTimeTicker(
			tickerCtx, RetransmissionTick,
		),
	}
	broadcastChannels[name] = append(broadcastChannels[name], channel)

	return channel
}

func broadcastMessage(name string, message net.Message) error {
	broadcastChannelsMutex.Lock()
	targetChannels := broadcastChannels[name]
	broadcastChannelsMutex.Unlock()

	for _, targetChannel := range targetChannels {
		targetChannel.deliver(message)
	}

	return nil
}

// ReleaseBroadcastChannel cancels every outstanding retransmission ticker
// registered under name and removes name's entry from the registry, so a
// later invocation reusing name starts from an empty registry regardless of
// whether an earlier invocation's leader was still retransmitting. Callers
// that create broadcast channels in tests should call this from t.Cleanup,
// passing the same name they created the channel(s) under.
func ReleaseBroadcastChannel(name string) {
	broadcastChannelsMutex.Lock()
	defer broadcastChannelsMutex.Unlock()

	for _, cancel := range broadcastChannelCancels[name] {
		cancel()
	}

	delete(broadcastChannels, name)
	delete(broadcastChannelCancels, name)
}
