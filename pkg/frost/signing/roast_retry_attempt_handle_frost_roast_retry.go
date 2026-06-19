//go:build frost_roast_retry

package signing

import (
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// SessionHandleBindingTTL is the maximum age the eviction sweep
// tolerates for a sessionAttemptBinding before treating it as
// orphaned. The two-hour default is documented in RFC-21's
// Resolved decisions section: long enough that no real signing
// session reaches it, short enough that a leaked binding cannot
// accumulate across days of node uptime.
const SessionHandleBindingTTL = 2 * time.Hour

// SessionHandleSweepInterval is how often the background sweeper
// goroutine wakes up to evict stale bindings. Coarse-grained on
// purpose: the sweep is a defence-in-depth backstop, not a tight
// liveness mechanism. 15 minutes balances responsiveness against
// goroutine churn.
const SessionHandleSweepInterval = 15 * time.Minute

// sessionAttemptBinding records the current attempt's handle and
// context for a session. The orchestration layer (Phase 5+) sets
// the binding via SetCurrentAttemptHandleForSession before driving
// the round-one / round-two / contribution receive loops; the
// receive loops read it at end-of-collect to know which attempt to
// submit their evidence snapshot against.
//
// createdAt is the wall-clock time at which the binding was last
// (re)set. The background sweeper evicts bindings older than
// SessionHandleBindingTTL.
type sessionAttemptBinding struct {
	handle    roast.AttemptHandle
	context   attempt.AttemptContext
	createdAt time.Time
}

var (
	sessionAttemptBindingMu sync.RWMutex
	sessionAttemptBindings  = map[string]sessionAttemptBinding{}

	sweeperOnce sync.Once
	sweeperStop chan struct{}
)

// SetCurrentAttemptHandleForSession records the in-flight attempt
// handle and context for the named session. Callers in the
// orchestration layer (Phase 5+) invoke this immediately after
// Coordinator.BeginAttempt so receive loops can correlate their
// captured evidence with the right attempt.
//
// Later calls for the same session overwrite earlier ones (this is
// the documented behaviour: a session whose attempt has transitioned
// re-binds to the new attempt's handle).
//
// The binding's createdAt is set to the current wall-clock time so
// the background sweeper can evict it if Clear is never called
// (panic before the deferred clear, etc.).
func SetCurrentAttemptHandleForSession(
	sessionID string,
	handle roast.AttemptHandle,
	ctx attempt.AttemptContext,
) {
	sessionAttemptBindingMu.Lock()
	defer sessionAttemptBindingMu.Unlock()
	sessionAttemptBindings[sessionID] = sessionAttemptBinding{
		handle:    handle,
		context:   ctx,
		createdAt: time.Now(),
	}
}

// ClearCurrentAttemptHandleForSession removes any binding for the
// named session. Callers invoke this when a session terminates so
// the registry does not grow unbounded.
func ClearCurrentAttemptHandleForSession(sessionID string) {
	sessionAttemptBindingMu.Lock()
	defer sessionAttemptBindingMu.Unlock()
	delete(sessionAttemptBindings, sessionID)
}

// ResetSessionHandleRegistryForTest clears every binding and stops
// the background sweeper if one is running. Exposed only for
// tests; not for production code paths.
func ResetSessionHandleRegistryForTest() {
	sessionAttemptBindingMu.Lock()
	defer sessionAttemptBindingMu.Unlock()
	sessionAttemptBindings = map[string]sessionAttemptBinding{}
	if sweeperStop != nil {
		close(sweeperStop)
		sweeperStop = nil
		sweeperOnce = sync.Once{}
	}
}

// StartSessionHandleSweeper launches the background goroutine that
// evicts sessionAttemptBindings older than SessionHandleBindingTTL.
// Idempotent via sync.Once: the first caller starts the sweeper;
// subsequent calls are no-ops. The sweeper runs for the lifetime of
// the process (until ResetSessionHandleRegistryForTest stops it,
// which only tests do).
//
// Phase 5.2 starts the sweeper from RegisterRoastRetryCoordinator
// so the defence-in-depth backstop is active whenever orchestration
// could plausibly run.
func StartSessionHandleSweeper() {
	sweeperOnce.Do(func() {
		sessionAttemptBindingMu.Lock()
		sweeperStop = make(chan struct{})
		stop := sweeperStop
		sessionAttemptBindingMu.Unlock()
		go sessionHandleSweepLoop(stop)
	})
}

func sessionHandleSweepLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(SessionHandleSweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			evictStaleSessionHandleBindings(SessionHandleBindingTTL)
			// Defense-in-depth backstop for the cross-attempt registries
			// (RFC-21 Phase 7.3 PR2b-1b): observe bindings are normally cleared
			// at session end and transition records are overwritten per attempt,
			// but a session that ends abnormally could orphan either -- sweep
			// anything past the TTL.
			evictStaleObservedAttempts(ObservedAttemptRegistryTTL)
			evictStaleRoastTransitions(RoastTransitionRegistryTTL)
		}
	}
}

// evictStaleSessionHandleBindings sweeps the binding map and
// removes entries older than maxAge. Exposed at the package level
// so tests can invoke it directly with small maxAge values without
// waiting for the sweeper ticker.
func evictStaleSessionHandleBindings(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	sessionAttemptBindingMu.Lock()
	defer sessionAttemptBindingMu.Unlock()
	evicted := 0
	for sessionID, binding := range sessionAttemptBindings {
		if binding.createdAt.Before(cutoff) {
			delete(sessionAttemptBindings, sessionID)
			evicted++
		}
	}
	return evicted
}

// currentAttemptHandleForCollect reads the binding the orchestration
// layer set for this session. Returns (zero, zero, false) when no
// binding exists -- the typical Phase-4 state, where no orchestration
// is wired yet. The submit helper takes ok=false as the signal to
// skip the RecordEvidence call.
func currentAttemptHandleForCollect(
	sessionID string,
) (roast.AttemptHandle, attempt.AttemptContext, bool) {
	sessionAttemptBindingMu.RLock()
	defer sessionAttemptBindingMu.RUnlock()
	binding, ok := sessionAttemptBindings[sessionID]
	if !ok {
		return roast.AttemptHandle{}, attempt.AttemptContext{}, false
	}
	return binding.handle, binding.context, true
}

// CurrentAttemptHandleForSession is the exported alias for callers
// outside the package (e.g. the ROAST-driven signing selector in
// pkg/tbtc). It is identical to currentAttemptHandleForCollect.
func CurrentAttemptHandleForSession(
	sessionID string,
) (roast.AttemptHandle, attempt.AttemptContext, bool) {
	return currentAttemptHandleForCollect(sessionID)
}
