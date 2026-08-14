//go:build frost_roast_retry

package signing

import (
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// RoastTransitionRegistryTTL is how long a (session, member) transition
// record is retained before the background sweeper evicts it. Matches the
// session-handle TTL: a record's usefulness to retry-driven participant
// selection expires when the session it describes is itself archived.
const RoastTransitionRegistryTTL = SessionHandleBindingTTL

// roastTransitionKey scopes a transition record to one local signer (member)
// within a ROAST session. A multi-seat operator runs one concurrent signer per
// seat sharing one roastSessionID; keying by session alone would collide (the
// #4081 multi-seat handle class). Each seat reads and writes its own record.
type roastTransitionKey struct {
	sessionID string
	member    group.MemberIndex
}

type roastTransitionEntry struct {
	record    RoastTransitionRecord
	createdAt time.Time
}

var (
	roastTransitionRegistryMu sync.RWMutex
	roastTransitionRegistry   = map[roastTransitionKey]roastTransitionEntry{}
)

// RecordRoastTransition stores the most recent transition record produced for
// (sessionID, member). A later call for the same key overwrites the earlier
// record. A nil Bundle is ignored: a record without a bundle is useless to the
// selector (it has nothing to drive NextAttempt with).
func RecordRoastTransition(
	sessionID string,
	member group.MemberIndex,
	record RoastTransitionRecord,
) {
	if record.Bundle == nil {
		return
	}
	roastTransitionRegistryMu.Lock()
	defer roastTransitionRegistryMu.Unlock()
	roastTransitionRegistry[roastTransitionKey{sessionID, member}] = roastTransitionEntry{
		record:    record,
		createdAt: time.Now(),
	}
}

// RoastTransitionForSession returns the most recent transition record for
// (sessionID, member), plus a presence flag. The ROAST selector treats
// (zero, false) as "no record; fall back to legacy".
func RoastTransitionForSession(
	sessionID string,
	member group.MemberIndex,
) (RoastTransitionRecord, bool) {
	roastTransitionRegistryMu.RLock()
	defer roastTransitionRegistryMu.RUnlock()
	entry, ok := roastTransitionRegistry[roastTransitionKey{sessionID, member}]
	if !ok {
		return RoastTransitionRecord{}, false
	}
	return entry.record, true
}

// ClearRoastTransitionForSession removes the record for (sessionID, member).
// Called when a session terminates.
func ClearRoastTransitionForSession(sessionID string, member group.MemberIndex) {
	roastTransitionRegistryMu.Lock()
	defer roastTransitionRegistryMu.Unlock()
	delete(roastTransitionRegistry, roastTransitionKey{sessionID, member})
}

// ResetRoastTransitionRegistryForTest clears every record. Test-only seam.
func ResetRoastTransitionRegistryForTest() {
	roastTransitionRegistryMu.Lock()
	defer roastTransitionRegistryMu.Unlock()
	roastTransitionRegistry = map[roastTransitionKey]roastTransitionEntry{}
}

// evictStaleRoastTransitions sweeps the registry and removes entries older than
// maxAge. Exposed at the package level so tests can invoke it directly with
// small maxAge values and so the session-handle sweeper can share one
// background goroutine.
func evictStaleRoastTransitions(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	roastTransitionRegistryMu.Lock()
	defer roastTransitionRegistryMu.Unlock()
	evicted := 0
	for key, entry := range roastTransitionRegistry {
		if entry.createdAt.Before(cutoff) {
			delete(roastTransitionRegistry, key)
			evicted++
		}
	}
	return evicted
}
