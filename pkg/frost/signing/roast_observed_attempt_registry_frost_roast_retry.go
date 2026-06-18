//go:build frost_roast_retry

package signing

import (
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// ObservedAttemptRegistryTTL is how long an observe binding is retained before
// the background sweeper evicts it. Matches the session-handle TTL: an observe
// binding is useless once the session it tracks is archived.
const ObservedAttemptRegistryTTL = SessionHandleBindingTTL

// observedAttemptKey scopes one local seat's observe binding to a specific
// attempt of a ROAST session. RFC-21 Phase 7.3 PR2b-1b has every local signer --
// including ones excluded from the attempt -- BeginAttempt locally so it holds a
// coordinator-instance-local handle to verify a transition bundle and run
// NextAttempt. The attempt hash discriminates the per-attempt bindings of one
// (session, member); the transition listener looks a binding up by the hash the
// incoming bundle carries.
type observedAttemptKey struct {
	sessionID   string
	member      group.MemberIndex
	attemptHash [attempt.MessageDigestLength]byte
}

// observedAttemptBinding is the per-attempt observe state a receiver needs to
// verify the attempt's transition bundle and compute the next attempt: the local
// handle (its own coordinator instance produced it), the bound attempt context
// (NextAttempt reads it as the previous context), and the DKG group public key
// (NextAttempt derives the next seed from it).
type observedAttemptBinding struct {
	handle            roast.AttemptHandle
	context           attempt.AttemptContext
	dkgGroupPublicKey []byte
}

type observedAttemptEntry struct {
	binding   observedAttemptBinding
	createdAt time.Time
}

var (
	observedAttemptRegistryMu sync.RWMutex
	observedAttemptRegistry   = map[observedAttemptKey]observedAttemptEntry{}
)

// recordObservedAttempt stores the observe binding for (sessionID, member,
// attemptHash). A later call for the same key overwrites the earlier binding
// (a re-observed attempt re-binds against the latest handle).
func recordObservedAttempt(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
	binding observedAttemptBinding,
) {
	observedAttemptRegistryMu.Lock()
	defer observedAttemptRegistryMu.Unlock()
	observedAttemptRegistry[observedAttemptKey{sessionID, member, attemptHash}] =
		observedAttemptEntry{binding: binding, createdAt: time.Now()}
}

// observedAttempt returns the observe binding for (sessionID, member,
// attemptHash) plus a presence flag.
func observedAttempt(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
) (observedAttemptBinding, bool) {
	observedAttemptRegistryMu.RLock()
	defer observedAttemptRegistryMu.RUnlock()
	entry, ok := observedAttemptRegistry[observedAttemptKey{sessionID, member, attemptHash}]
	if !ok {
		return observedAttemptBinding{}, false
	}
	return entry.binding, true
}

// clearObservedAttempt removes the observe binding for (sessionID, member,
// attemptHash). Called once a verified transition record is stored for the
// attempt, on success, or at session end.
func clearObservedAttempt(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
) {
	observedAttemptRegistryMu.Lock()
	defer observedAttemptRegistryMu.Unlock()
	delete(observedAttemptRegistry, observedAttemptKey{sessionID, member, attemptHash})
}

// clearObservedAttemptsForSession removes every observe binding for
// (sessionID, member), regardless of attempt hash. The transition exchange calls
// it when the session ends (its listener context is done), so a signing whose
// attempts succeeded -- and therefore never produced a transition record to clear
// per-attempt via clearObservedAttempt -- does not leave its observe bindings
// behind.
func clearObservedAttemptsForSession(sessionID string, member group.MemberIndex) {
	observedAttemptRegistryMu.Lock()
	defer observedAttemptRegistryMu.Unlock()
	for key := range observedAttemptRegistry {
		if key.sessionID == sessionID && key.member == member {
			delete(observedAttemptRegistry, key)
		}
	}
}

// ObservedAttemptStoredForTest reports whether any observe binding exists for
// (sessionID, member), regardless of attempt hash. Exported test seam so
// downstream-package tests can assert the controller stored a binding without
// reaching into the unexported registry.
func ObservedAttemptStoredForTest(sessionID string, member group.MemberIndex) bool {
	observedAttemptRegistryMu.RLock()
	defer observedAttemptRegistryMu.RUnlock()
	for key := range observedAttemptRegistry {
		if key.sessionID == sessionID && key.member == member {
			return true
		}
	}
	return false
}

// ResetObservedAttemptRegistryForTest clears every observe binding. Test-only
// seam.
func ResetObservedAttemptRegistryForTest() {
	observedAttemptRegistryMu.Lock()
	defer observedAttemptRegistryMu.Unlock()
	observedAttemptRegistry = map[observedAttemptKey]observedAttemptEntry{}
}

// evictStaleObservedAttempts sweeps the registry and removes bindings older than
// maxAge. Exposed at the package level so tests can invoke it directly with
// small maxAge values and so the session-handle sweeper can share one background
// goroutine.
func evictStaleObservedAttempts(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	observedAttemptRegistryMu.Lock()
	defer observedAttemptRegistryMu.Unlock()
	evicted := 0
	for key, entry := range observedAttemptRegistry {
		if entry.createdAt.Before(cutoff) {
			delete(observedAttemptRegistry, key)
			evicted++
		}
	}
	return evicted
}
