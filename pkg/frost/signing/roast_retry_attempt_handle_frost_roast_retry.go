//go:build frost_roast_retry

package signing

import (
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
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

// sessionMemberKey identifies a binding by the signing session AND the
// local member seat it belongs to. RFC-21 Phase 7.3 PR2b-2 re-keyed the
// registry from sessionID alone to (sessionID, member): a multi-seat
// operator runs one receive loop per local seat, and each seat mints its
// own attempt handle from its own coordinator. Keying by sessionID alone
// let sibling seats collide -- the later Set overwrote the earlier seat's
// handle (mis-attributing its evidence) and one seat's cleanup deleted the
// shared binding out from under the survivor (silently disabling the
// survivor's inbound attempt-context-hash enforcement). The member
// component isolates each seat's binding so neither happens.
type sessionMemberKey struct {
	sessionID string
	member    group.MemberIndex
}

// sessionAttemptBinding records the current attempt's handle and
// context for a (session, member). The orchestration layer (Phase 5+)
// sets the binding via SetCurrentAttemptHandleForSession before driving
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
	sessionAttemptBindings  = map[sessionMemberKey]sessionAttemptBinding{}

	sweeperOnce sync.Once
	sweeperStop chan struct{}
)

// SetCurrentAttemptHandleForSession records the in-flight attempt
// handle and context for the named session and local member seat.
// Callers in the orchestration layer (Phase 5+) invoke this immediately
// after Coordinator.BeginAttempt so receive loops can correlate their
// captured evidence with the right attempt.
//
// The binding is keyed by (sessionID, member): a multi-seat operator's
// sibling seats each record under their own member so neither overwrites
// the other (RFC-21 Phase 7.3 PR2b-2).
//
// Later calls for the same (session, member) overwrite earlier ones (this
// is the documented behaviour: a session whose attempt has transitioned
// re-binds to the new attempt's handle). Begin/cleanup are scoped to a
// single Execute and the signingRetryLoop drives attempts strictly
// sequentially per session, so the overwrite never races a live earlier
// attempt's binding.
//
// The binding's createdAt is set to the current wall-clock time so
// the background sweeper can evict it if Clear is never called
// (panic before the deferred clear, etc.).
func SetCurrentAttemptHandleForSession(
	sessionID string,
	member group.MemberIndex,
	handle roast.AttemptHandle,
	ctx attempt.AttemptContext,
) {
	sessionAttemptBindingMu.Lock()
	defer sessionAttemptBindingMu.Unlock()
	sessionAttemptBindings[sessionMemberKey{sessionID, member}] = sessionAttemptBinding{
		handle:    handle,
		context:   ctx,
		createdAt: time.Now(),
	}
}

// ClearCurrentAttemptHandleForSession removes any binding for the
// named session and member. Callers invoke this when a session
// terminates so the registry does not grow unbounded. Because the
// binding is member-keyed, a seat only clears its OWN binding -- a
// sibling seat's binding for the same session is untouched.
func ClearCurrentAttemptHandleForSession(sessionID string, member group.MemberIndex) {
	sessionAttemptBindingMu.Lock()
	defer sessionAttemptBindingMu.Unlock()
	delete(sessionAttemptBindings, sessionMemberKey{sessionID, member})
}

// ResetSessionHandleRegistryForTest clears every binding and stops
// the background sweeper if one is running. Exposed only for
// tests; not for production code paths.
func ResetSessionHandleRegistryForTest() {
	sessionAttemptBindingMu.Lock()
	defer sessionAttemptBindingMu.Unlock()
	sessionAttemptBindings = map[sessionMemberKey]sessionAttemptBinding{}
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
			// Stashed coarse-path evidence is normally consumed by
			// BroadcastForcedSnapshot or cleared on success/session-end (RFC-21
			// Phase 7.3 PR2b-2 step 2); sweep any orphaned by an abnormal end.
			evictStalePendingEvidence(PendingEvidenceRegistryTTL)
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
	for key, binding := range sessionAttemptBindings {
		if binding.createdAt.Before(cutoff) {
			delete(sessionAttemptBindings, key)
			evicted++
		}
	}
	return evicted
}

// currentAttemptHandleForCollect reads the binding the orchestration
// layer set for this (session, member). Returns (zero, zero, false) when
// no binding exists -- the typical Phase-4 state, where no orchestration
// is wired yet. The submit helper takes ok=false as the signal to
// skip the RecordEvidence call.
func currentAttemptHandleForCollect(
	sessionID string,
	member group.MemberIndex,
) (roast.AttemptHandle, attempt.AttemptContext, bool) {
	sessionAttemptBindingMu.RLock()
	defer sessionAttemptBindingMu.RUnlock()
	binding, ok := sessionAttemptBindings[sessionMemberKey{sessionID, member}]
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
	member group.MemberIndex,
) (roast.AttemptHandle, attempt.AttemptContext, bool) {
	return currentAttemptHandleForCollect(sessionID, member)
}
