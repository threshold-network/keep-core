//go:build frost_native

package signing

import (
	"sync"
	"time"
)

// interactiveAggregateMemoTTL bounds how long a memoized aggregate result is
// retained. It only needs to outlive the window in which a single signing
// attempt's local seats race to aggregate (bounded by the attempt timeout, tens
// of seconds), so a few minutes is safely conservative while keeping the memo
// from growing over the node's lifetime.
const interactiveAggregateMemoTTL = 5 * time.Minute

type interactiveAggregateEntry struct {
	once      sync.Once
	signature []byte
	err       error
}

var (
	interactiveAggregateMemoMu sync.Mutex
	interactiveAggregateMemo   = map[string]*interactiveAggregateEntry{}
)

// aggregateInteractiveOnce runs aggregate AT MOST ONCE per key within this
// process and returns the same (signature, error) to every caller sharing that
// key.
//
// Why it exists: a multi-seat operator runs one interactiveSigningRunner
// goroutine per LOCAL seat, and they all drive the SAME per-process interactive
// engine session for the wallet's key group. Step 9 of the runner has every
// participating member aggregate — correct across SEPARATE processes, where each
// aggregates against its own session. But two local seats in ONE process hit the
// engine's per-attempt anti-replay marker: the first InteractiveAggregate
// succeeds and the second fails with InteractiveAttemptAlreadyAggregated even
// though the deterministic signature was already produced. Aggregation is a
// public, deterministic operation over the same signing package and the same
// subset shares, so returning the first execution's result to the sibling seats
// is correct and lets the coordinator seat obtain the signature regardless of
// which local seat computed it first.
//
// The key must uniquely identify one aggregation: the caller uses
// sessionID + attemptID, which the engine's marker is derived from (attempt id +
// message digest, with the message fixed for the attempt and the session pinned
// to a single message + taproot root at open time).
func aggregateInteractiveOnce(
	key string,
	aggregate func() ([]byte, error),
) ([]byte, error) {
	interactiveAggregateMemoMu.Lock()
	entry, ok := interactiveAggregateMemo[key]
	if !ok {
		entry = &interactiveAggregateEntry{}
		interactiveAggregateMemo[key] = entry
		// Self-evict well after the attempt's signing window so the memo does not
		// grow unbounded. Concurrent local seats share the entry via sync.Once long
		// before this fires; a straggler that arrives after eviction simply
		// re-aggregates (and, if the engine already consumed the marker, fails its
		// own attempt into the existing retry path — never a wrong signature).
		time.AfterFunc(interactiveAggregateMemoTTL, func() {
			interactiveAggregateMemoMu.Lock()
			delete(interactiveAggregateMemo, key)
			interactiveAggregateMemoMu.Unlock()
		})
	}
	interactiveAggregateMemoMu.Unlock()

	entry.once.Do(func() {
		entry.signature, entry.err = aggregate()
	})
	return entry.signature, entry.err
}

// ResetInteractiveAggregateMemoForTest clears the process-wide aggregate memo.
// Tests that run multiple runners in a single process (each simulating a
// separate operator, which in production would be its own process with its own
// memo) must reset between cases so a memoized result does not leak across tests
// or suppress a case's own engine aggregate call. Not for production use.
func ResetInteractiveAggregateMemoForTest() {
	interactiveAggregateMemoMu.Lock()
	interactiveAggregateMemo = map[string]*interactiveAggregateEntry{}
	interactiveAggregateMemoMu.Unlock()
}
