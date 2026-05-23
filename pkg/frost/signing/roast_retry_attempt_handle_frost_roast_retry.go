//go:build frost_roast_retry

package signing

import (
	"sync"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// sessionAttemptBinding records the current attempt's handle and
// context for a session. The orchestration layer (Phase 5+) sets
// the binding via SetCurrentAttemptHandleForSession before driving
// the round-one / round-two / contribution receive loops; the
// receive loops read it at end-of-collect to know which attempt to
// submit their evidence snapshot against.
type sessionAttemptBinding struct {
	handle  roast.AttemptHandle
	context attempt.AttemptContext
}

var (
	sessionAttemptBindingMu sync.RWMutex
	sessionAttemptBindings  = map[string]sessionAttemptBinding{}
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
func SetCurrentAttemptHandleForSession(
	sessionID string,
	handle roast.AttemptHandle,
	ctx attempt.AttemptContext,
) {
	sessionAttemptBindingMu.Lock()
	defer sessionAttemptBindingMu.Unlock()
	sessionAttemptBindings[sessionID] = sessionAttemptBinding{
		handle:  handle,
		context: ctx,
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

// ResetSessionHandleRegistryForTest clears every binding. Exposed
// only for tests; not for production code paths.
func ResetSessionHandleRegistryForTest() {
	sessionAttemptBindingMu.Lock()
	defer sessionAttemptBindingMu.Unlock()
	sessionAttemptBindings = map[string]sessionAttemptBinding{}
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
