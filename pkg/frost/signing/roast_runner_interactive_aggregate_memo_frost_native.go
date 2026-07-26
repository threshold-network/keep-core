//go:build frost_native

package signing

import (
	"fmt"
	"strings"
	"sync"
)

type interactiveAggregateEntry struct {
	once      sync.Once
	signature []byte
	err       error
	owner     *InteractiveAggregateMemoSession
}

// InteractiveAggregateMemoSession owns every per-attempt aggregate result for
// one outer tBTC signing operation. The tBTC executor releases it only after
// all local-seat goroutines have joined, so no wall-clock eviction can cause a
// straggling seat to repeat the native Aggregate call.
type InteractiveAggregateMemoSession struct {
	sessionID string
	release   sync.Once
}

var (
	interactiveAggregateMemoMu       sync.Mutex
	interactiveAggregateMemo         = map[string]*interactiveAggregateEntry{}
	interactiveAggregateMemoSessions = map[string]*InteractiveAggregateMemoSession{}
)

// BeginInteractiveAggregateMemoSession binds aggregate memo lifetime to one
// outer signing operation. A duplicate live session is rejected rather than
// sharing state across independent operation owners.
func BeginInteractiveAggregateMemoSession(
	sessionID string,
) (*InteractiveAggregateMemoSession, error) {
	if sessionID == "" || strings.Contains(sessionID, "|") {
		return nil, fmt.Errorf("interactive aggregate memo session ID is invalid")
	}

	interactiveAggregateMemoMu.Lock()
	defer interactiveAggregateMemoMu.Unlock()

	if _, exists := interactiveAggregateMemoSessions[sessionID]; exists {
		return nil, fmt.Errorf(
			"interactive aggregate memo session [%s] is already active",
			sessionID,
		)
	}
	prefix := sessionID + "|"
	for key := range interactiveAggregateMemo {
		if strings.HasPrefix(key, prefix) {
			return nil, fmt.Errorf(
				"interactive aggregate memo session [%s] has unowned stale entries",
				sessionID,
			)
		}
	}

	session := &InteractiveAggregateMemoSession{sessionID: sessionID}
	interactiveAggregateMemoSessions[sessionID] = session
	return session, nil
}

// Release deletes exactly the entries owned by this session generation. The
// pointer identity guard prevents a delayed stale cleanup from deleting a
// newer operation that happens to reuse the same textual session ID.
func (session *InteractiveAggregateMemoSession) Release() {
	if session == nil {
		return
	}
	session.release.Do(func() {
		releaseInteractiveAggregateMemoSession(session)
	})
}

func releaseInteractiveAggregateMemoSession(
	session *InteractiveAggregateMemoSession,
) {
	if session == nil {
		return
	}

	interactiveAggregateMemoMu.Lock()
	defer interactiveAggregateMemoMu.Unlock()

	if interactiveAggregateMemoSessions[session.sessionID] != session {
		return
	}
	delete(interactiveAggregateMemoSessions, session.sessionID)
	prefix := session.sessionID + "|"
	for key, entry := range interactiveAggregateMemo {
		if strings.HasPrefix(key, prefix) && entry.owner == session {
			delete(interactiveAggregateMemo, key)
		}
	}
}

// aggregateInteractiveOnce runs aggregate AT MOST ONCE per key for the active
// outer session and returns the same (signature, error) to every caller sharing
// that key.
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
	sessionID, _, hasAttemptSeparator := strings.Cut(key, "|")
	owner := interactiveAggregateMemoSessions[sessionID]
	entry, ok := interactiveAggregateMemo[key]
	if !ok {
		entry = &interactiveAggregateEntry{owner: owner}
		interactiveAggregateMemo[key] = entry
	} else if hasAttemptSeparator && entry.owner != owner {
		interactiveAggregateMemoMu.Unlock()
		return nil, fmt.Errorf(
			"interactive aggregate memo key [%s] belongs to another session owner",
			key,
		)
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
	interactiveAggregateMemoSessions =
		map[string]*InteractiveAggregateMemoSession{}
	interactiveAggregateMemoMu.Unlock()
}
