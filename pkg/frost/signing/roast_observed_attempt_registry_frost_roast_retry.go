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
	// observedAttemptHistory records that (sessionID, member) observed an attempt
	// hash, retained even after the per-attempt binding is consumed or cleared so
	// the transition listener can tell a bundle for an attempt this seat NEVER
	// observed (it skipped a window peers committed -> lost ROAST sync -> fail
	// closed) apart from a duplicate bundle for one it already consumed (a benign
	// retransmit). It shares the binding's lifetime: cleared at session end and
	// swept by the same TTL.
	observedAttemptHistory = map[observedAttemptKey]time.Time{}
)

// recordObservedAttempt stores the observe binding for (sessionID, member,
// attemptHash) and marks the attempt as observed in the history. A later call for
// the same key overwrites the earlier binding (a re-observed attempt re-binds
// against the latest handle).
func recordObservedAttempt(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
	binding observedAttemptBinding,
) {
	observedAttemptRegistryMu.Lock()
	defer observedAttemptRegistryMu.Unlock()
	now := time.Now()
	key := observedAttemptKey{sessionID, member, attemptHash}
	observedAttemptRegistry[key] = observedAttemptEntry{binding: binding, createdAt: now}
	observedAttemptHistory[key] = now
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

// attemptEverObserved reports whether (sessionID, member) observed the given
// attempt hash at any point this session -- including an attempt whose per-attempt
// binding has since been consumed or cleared. The transition listener uses it to
// tell a bundle for a NEVER-observed attempt (this seat skipped a window peers
// committed -> lost ROAST sync) apart from a duplicate bundle for an
// already-consumed attempt (a benign retransmit that must NOT trip lost sync).
func attemptEverObserved(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
) bool {
	observedAttemptRegistryMu.RLock()
	defer observedAttemptRegistryMu.RUnlock()
	_, ok := observedAttemptHistory[observedAttemptKey{sessionID, member, attemptHash}]
	return ok
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

// ClearObservedAttemptOnLocalSuccess clears the observe binding for an attempt
// this seat completed successfully (a valid signature aggregated on its drive
// handle). The observe handle otherwise stays in a collecting state, so an elected
// coordinator could still aggregate a failure bundle, and a peer's failure bundle
// would be stored as a transition record, for an attempt that actually SUCCEEDED.
// Clearing the binding -- the observed-history marker remains, so a later bundle
// for it is treated as a benign retransmit, not lost sync -- means no failure
// transition is synthesized or stored for the succeeded attempt, so a subsequent
// done-check failure fails closed (no fresh record) instead of consuming a
// dishonest failure transition (RFC-21 Phase 7.3 PR2b-1b B3). Exported so the
// pkg/tbtc transition controller can call it on local success.
func ClearObservedAttemptOnLocalSuccess(
	sessionID string,
	member group.MemberIndex,
	attemptHash [attempt.MessageDigestLength]byte,
) {
	clearObservedAttempt(sessionID, member, attemptHash)
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
	for key := range observedAttemptHistory {
		if key.sessionID == sessionID && key.member == member {
			delete(observedAttemptHistory, key)
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
	observedAttemptHistory = map[observedAttemptKey]time.Time{}
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
	// Sweep stale observed-history markers on the same cutoff so a long-abandoned
	// session's lost-sync markers do not linger past their backstop TTL.
	for key, observedAt := range observedAttemptHistory {
		if observedAt.Before(cutoff) {
			delete(observedAttemptHistory, key)
		}
	}
	return evicted
}
