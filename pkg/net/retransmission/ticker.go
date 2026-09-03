package retransmission

import (
	"context"
	"sync"
	"time"
)

// Ticker controls the frequency of retransmissions.
type Ticker struct {
	ticks <-chan uint64

	handlersMutex sync.Mutex
	handlers      map[uint64]*handler
	nextHandlerId uint64
}

type handler struct {
	ctx context.Context
	fn  func()
}

// NewTicker creates and starts a new Ticker for the provided channel.
// For each item read from the channel, new tick is triggered. All handlers
// are unregistered and ticker is stopped when the provided channel gets closed.
func NewTicker(ticks <-chan uint64) *Ticker {
	ticker := &Ticker{
		ticks:    ticks,
		handlers: make(map[uint64]*handler),
	}

	go ticker.start()

	return ticker
}

// NewTimeTicker is a convenience function allowing to create time-based
// retransmission.Ticker for the provided duration. When the provided context is
// done, all handlers are unregistered and retransmission.Ticker is stopped.
func NewTimeTicker(ctx context.Context, duration time.Duration) *Ticker {
	ticks := make(chan uint64)
	timeTicker := time.NewTicker(duration)

	// pipe ticks from time ticker
	go func() {
		for {
			select {
			case tick := <-timeTicker.C:
				ticks <- uint64(tick.Unix())

			case <-ctx.Done():
				timeTicker.Stop()
				close(ticks)
				return
			}
		}
	}()

	return NewTicker(ticks)
}

func (t *Ticker) start() {
	for range t.ticks {
		t.handlersMutex.Lock()

		for id, handler := range t.handlers {
			if handler.ctx.Err() != nil {
				delete(t.handlers, id)
				continue
			}

			handler.fn()
		}

		t.handlersMutex.Unlock()
	}

	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	clear(t.handlers)
}

func (t *Ticker) onTick(ctx context.Context, fn func()) {
	t.handlersMutex.Lock()
	t.nextHandlerId++
	t.handlers[t.nextHandlerId] = &handler{ctx, fn}
	t.handlersMutex.Unlock()
}
