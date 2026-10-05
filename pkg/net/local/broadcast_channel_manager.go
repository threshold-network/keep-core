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

var broadcastSessionsMutex sync.Mutex
var broadcastSessions map[string]*broadcastSession

// broadcastSession groups the channels and retransmission tickers created
// under one name during a single use of that name, ending at
// ReleaseBroadcastChannel. Retransmissions deliver through the session
// they were scheduled in, so an in-flight retransmission of a released
// session can never reach a later session's channels under the same
// name, even when the release and the re-creation race a tick that is
// already firing.
type broadcastSession struct {
	mu       sync.Mutex
	channels []*localChannel
	cancels  []context.CancelFunc
}

// broadcast delivers message to every channel currently registered in
// this session.
func (s *broadcastSession) broadcast(message net.Message) {
	s.mu.Lock()
	targetChannels := make([]*localChannel, len(s.channels))
	copy(targetChannels, s.channels)
	s.mu.Unlock()

	for _, targetChannel := range targetChannels {
		targetChannel.deliver(message)
	}
}

// getBroadcastChannel returns a BroadcastChannel designed to mediate between local
// participants. It delivers all messages sent to the channel through its
// receive channels. RecvChan on a LocalChannel creates a new receive channel
// that is returned to the caller, so that all receive channels can receive
// the message.
func getBroadcastChannel(
	name string,
	operatorPublicKey *operator.PublicKey,
) net.BroadcastChannel {
	broadcastSessionsMutex.Lock()
	defer broadcastSessionsMutex.Unlock()

	if broadcastSessions == nil {
		broadcastSessions = make(map[string]*broadcastSession)
	}

	session := broadcastSessions[name]
	if session == nil {
		session = &broadcastSession{
			channels: make([]*localChannel, 0),
			cancels:  make([]context.CancelFunc, 0),
		}
		broadcastSessions[name] = session
	}

	tickerCtx, cancelTicker := context.WithCancel(context.Background())

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
		session: session,
	}

	session.mu.Lock()
	session.cancels = append(session.cancels, cancelTicker)
	session.channels = append(session.channels, channel)
	session.mu.Unlock()

	return channel
}

// ReleaseBroadcastChannel stops every outstanding retransmission ticker
// registered under name and closes name's session, so that retransmissions
// already in flight when the release happens only ever reach this
// session's own, already emptied channel list and can never reach a later
// invocation reusing name, whose session starts from an empty channel
// list. Callers that create broadcast channels in tests should call this
// from t.Cleanup, passing the same name they created the channel(s) under.
func ReleaseBroadcastChannel(name string) {
	broadcastSessionsMutex.Lock()
	session := broadcastSessions[name]
	if session != nil {
		delete(broadcastSessions, name)
	}
	broadcastSessionsMutex.Unlock()

	if session == nil {
		return
	}

	session.mu.Lock()
	cancels := session.cancels
	session.cancels = nil
	session.channels = nil
	session.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}
