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
var broadcastChannelCancels []context.CancelFunc

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

	_, exists := broadcastChannels[name]
	if !exists {
		broadcastChannels[name] = make([]*localChannel, 0)
	}

	tickerCtx, cancelTicker := context.WithCancel(context.Background())
	broadcastChannelCancels = append(broadcastChannelCancels, cancelTicker)

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

// ResetForTesting clears every registered broadcast channel and cancels
// every outstanding retransmission ticker's context, stopping it. It exists
// because getBroadcastChannel's registry is append-only and process-global:
// without an explicit reset, a channel created by one test keeps
// retransmitting forever (its ticker context was never otherwise cancelled)
// and stays registered under its name for the lifetime of the test binary,
// so a later test - or a repeated -count=N invocation of the same test -
// that reuses that name would receive the earlier invocation's stale,
// still-retransmitting messages alongside its own. Callers that create
// broadcast channels in tests should call this from t.Cleanup so each test
// invocation starts from an empty registry.
func ResetForTesting() {
	broadcastChannelsMutex.Lock()
	defer broadcastChannelsMutex.Unlock()

	for _, cancel := range broadcastChannelCancels {
		cancel()
	}

	broadcastChannels = nil
	broadcastChannelCancels = nil
}
