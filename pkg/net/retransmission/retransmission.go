// Package retransmission implements a simple retransmission mechanism for
// network messages based on their sequence number. Retransmitting message
// several times for the lifetime of the given phase helps to improve message
// delivery rate for senders and receivers who are not perfectly synced on time.
package retransmission

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-core/pkg/net"
)

// maxCachedMessageIDs bounds the retransmission deduplication window. It is
// sized to accommodate the expected number of concurrent protocol participants
// multiplied by the retransmission count per phase, with headroom for bursts.
const maxCachedMessageIDs = 10000

// RetransmitFn represents a retransmission routine.
type RetransmitFn func() error

// ScheduleRetransmissions uses the given Strategy to decide whether to call
// the provided RetransmitFn for every new tick received from the provided
// Ticker for the entire lifetime of the Context. The RetransmitFn function has
// to guarantee that every call from this function sends a message with the
// same sequence number.
func ScheduleRetransmissions(
	ctx context.Context,
	logger log.StandardLogger,
	ticker *Ticker,
	retransmit RetransmitFn,
	strategy Strategy,
) {
	go func() {
		ticker.onTick(ctx, func() {
			go func() {
				if err := strategy.Tick(retransmit); err != nil {
					logger.Errorf("could not retransmit message: [%v]", err)
				}
			}()
		})
	}()
}

// WithRetransmissionSupport takes the standard network message handler and
// enhances it with functionality allowing to handle retransmissions.
// The returned handler filters out retransmissions and calls the delegate
// handler only if the received message is not a retransmission or if it is
// a retransmission but it has not been seen within the last
// maxCachedMessageIDs unique message IDs. Messages older than that window may
// be re-processed if their IDs have been evicted from the cache.
// The returned handler is thread-safe.
//
// Retransmissions are identified by sender transport ID and message sequence
// number. Two messages with the same sender ID and sequence number are
// considered the same. Handler can not be reused between channels if sequence
// number of message is local for channel.
func WithRetransmissionSupport(delegate func(m net.Message)) func(m net.Message) {
	return withRetransmissionSupport(delegate, maxCachedMessageIDs)
}

func withRetransmissionSupport(
	delegate func(m net.Message),
	maxCacheSize int,
) func(m net.Message) {
	if maxCacheSize <= 0 {
		maxCacheSize = 1
	}

	mutex := &sync.Mutex{}
	cache := make(map[string]struct{})
	// Fixed-size ring buffer recording insertion order so eviction is O(1)
	// and the backing array never grows beyond maxCacheSize. A slice-shift
	// approach (cache = cache[1:]) would slide the window through the backing
	// array and force periodic reallocations, overshooting the configured
	// bound by roughly 2x in steady state.
	ring := make([]string, maxCacheSize)
	head := 0
	size := 0

	return func(message net.Message) {
		messageID := fmt.Sprintf(
			"%v-%v",
			message.TransportSenderID().String(),
			message.Seqno(),
		)

		// Hold the mutex only while consulting and updating the bounded
		// cache. The delegate is invoked outside the critical section so a
		// slow or re-entrant handler cannot stall other receivers.
		seen := func() bool {
			mutex.Lock()
			defer mutex.Unlock()

			if _, ok := cache[messageID]; ok {
				return true
			}

			if size == maxCacheSize {
				delete(cache, ring[head])
				ring[head] = messageID
				head = (head + 1) % maxCacheSize
			} else {
				ring[(head+size)%maxCacheSize] = messageID
				size++
			}
			cache[messageID] = struct{}{}
			return false
		}()

		if !seen {
			delegate(message)
		}
	}
}
