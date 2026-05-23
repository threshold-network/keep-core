//go:build frost_roast_retry

package signing

import (
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast"
)

// TransitionBundleRegistryTTL is how long a session's most recent
// TransitionMessage is retained before the background sweeper
// evicts it. Matches the session-handle TTL: a bundle's usefulness
// to retry-driven participant selection expires when the session
// it describes is itself archived.
const TransitionBundleRegistryTTL = SessionHandleBindingTTL

// sessionBundleEntry pairs a TransitionMessage with the wall-clock
// time at which it was recorded so the sweeper can evict stale
// entries.
type sessionBundleEntry struct {
	bundle    *roast.TransitionMessage
	createdAt time.Time
}

var (
	sessionBundleRegistryMu sync.RWMutex
	sessionBundleRegistry   = map[string]sessionBundleEntry{}
)

// RecordTransitionBundleForSession stores the most recent
// TransitionMessage produced by the elected coordinator for the
// named session. The bundle is later consumed by the ROAST-driven
// signingParticipantSelector to compute the next attempt's
// IncludedSet via EvaluateRoastRetryForSigning.
//
// A later call for the same session overwrites the earlier bundle
// -- the registry tracks only the most recent transition.
func RecordTransitionBundleForSession(
	sessionID string,
	bundle *roast.TransitionMessage,
) {
	if bundle == nil {
		return
	}
	sessionBundleRegistryMu.Lock()
	defer sessionBundleRegistryMu.Unlock()
	sessionBundleRegistry[sessionID] = sessionBundleEntry{
		bundle:    bundle,
		createdAt: time.Now(),
	}
}

// TransitionBundleForSession returns the most recent transition
// message for the named session, plus a presence flag. Callers
// (the ROAST selector) treat (nil, false) as "no bundle; fall back
// to legacy".
func TransitionBundleForSession(
	sessionID string,
) (*roast.TransitionMessage, bool) {
	sessionBundleRegistryMu.RLock()
	defer sessionBundleRegistryMu.RUnlock()
	entry, ok := sessionBundleRegistry[sessionID]
	if !ok {
		return nil, false
	}
	return entry.bundle, true
}

// ClearTransitionBundleForSession removes any bundle for the named
// session. Called when a session terminates.
func ClearTransitionBundleForSession(sessionID string) {
	sessionBundleRegistryMu.Lock()
	defer sessionBundleRegistryMu.Unlock()
	delete(sessionBundleRegistry, sessionID)
}

// ResetTransitionBundleRegistryForTest clears every bundle. Test-
// only seam.
func ResetTransitionBundleRegistryForTest() {
	sessionBundleRegistryMu.Lock()
	defer sessionBundleRegistryMu.Unlock()
	sessionBundleRegistry = map[string]sessionBundleEntry{}
}

// evictStaleTransitionBundles sweeps the registry and removes
// entries older than maxAge. Exposed at the package level so
// tests can invoke it directly with small maxAge values. The
// production sweeper invokes it from sessionHandleSweepLoop
// (Phase 5.2) so the bundle and handle registries share a single
// background goroutine.
func evictStaleTransitionBundles(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	sessionBundleRegistryMu.Lock()
	defer sessionBundleRegistryMu.Unlock()
	evicted := 0
	for sessionID, entry := range sessionBundleRegistry {
		if entry.createdAt.Before(cutoff) {
			delete(sessionBundleRegistry, sessionID)
			evicted++
		}
	}
	return evicted
}
