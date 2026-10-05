package local

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/keep-network/keep-core/pkg/operator"

	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/net/internal"
	"github.com/keep-network/keep-core/pkg/net/retransmission"
)

const messageHandlerThrottle = 256

type messageHandler struct {
	ctx     context.Context
	channel chan net.Message
}

type localChannel struct {
	counter              uint64
	name                 string
	identifier           net.TransportIdentifier
	operatorPublicKey    *operator.PublicKey
	messageHandlersMutex sync.Mutex
	messageHandlers      []*messageHandler
	unmarshalersMutex    sync.Mutex
	unmarshalersByType   map[string]func() net.TaggedUnmarshaler
	retransmissionTicker *retransmission.Ticker
	session              *broadcastSession
}

func (lc *localChannel) nextSeqno() uint64 {
	return atomic.AddUint64(&lc.counter, 1)
}

func (lc *localChannel) Name() string {
	return lc.name
}

func (lc *localChannel) Send(
	ctx context.Context,
	message net.TaggedMarshaler,
	retransmissionStrategy ...net.RetransmissionStrategy,
) error {
	bytes, err := message.Marshal()
	if err != nil {
		return err
	}

	unmarshaler, found := lc.unmarshalersByType[string(message.Type())]
	if !found {
		return fmt.Errorf("couldn't find unmarshaler for type %s", string(message.Type()))
	}

	unmarshaled := unmarshaler()
	err = unmarshaled.Unmarshal(bytes)
	if err != nil {
		return err
	}

	operatorPublicKeyBytes := operator.MarshalUncompressed(lc.operatorPublicKey)

	netMessage := internal.BasicMessage(
		lc.identifier,
		unmarshaled,
		message.Type(),
		operatorPublicKeyBytes,
		lc.nextSeqno(),
	)

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
		lc.retransmissionTicker,
		func() error {
			lc.session.broadcast(netMessage)
			return nil
		},
		retransmission.WithStrategy(strategy),
	)

	lc.session.broadcast(netMessage)
	return nil
}

func (lc *localChannel) deliver(message net.Message) {
	lc.messageHandlersMutex.Lock()
	snapshot := make([]*messageHandler, len(lc.messageHandlers))
	copy(snapshot, lc.messageHandlers)
	lc.messageHandlersMutex.Unlock()

	for _, handler := range snapshot {
		select {
		case handler.channel <- message:
		default:
			logger.Warnf("handler too slow, dropping message")
		}
	}
}

func (lc *localChannel) Recv(ctx context.Context, handler func(m net.Message)) {
	messageHandler := &messageHandler{
		ctx:     ctx,
		channel: make(chan net.Message, messageHandlerThrottle),
	}

	lc.messageHandlersMutex.Lock()
	lc.messageHandlers = append(lc.messageHandlers, messageHandler)
	lc.messageHandlersMutex.Unlock()

	handleWithRetransmissions := retransmission.WithRetransmissionSupport(handler)

	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Debug("context is done, removing handler")
				lc.removeHandler(messageHandler)
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

				handleWithRetransmissions(msg)
			}
		}
	}()
}

func (lc *localChannel) removeHandler(handler *messageHandler) {
	lc.messageHandlersMutex.Lock()
	defer lc.messageHandlersMutex.Unlock()

	for i, h := range lc.messageHandlers {
		if h.channel == handler.channel {
			lc.messageHandlers[i] = lc.messageHandlers[len(lc.messageHandlers)-1]
			lc.messageHandlers = lc.messageHandlers[:len(lc.messageHandlers)-1]
			break
		}
	}
}

func (lc *localChannel) SetUnmarshaler(unmarshaler func() net.TaggedUnmarshaler) {
	tpe := unmarshaler().Type()

	lc.unmarshalersMutex.Lock()
	defer lc.unmarshalersMutex.Unlock()

	lc.unmarshalersByType[tpe] = unmarshaler
}

func (lc *localChannel) SetFilter(filter net.BroadcastChannelFilter) error {
	return nil // no-op
}
