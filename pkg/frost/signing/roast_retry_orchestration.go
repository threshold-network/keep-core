package signing

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// BeginOrchestrationForSession encapsulates the per-session
// BeginAttempt + binding-population step the RFC-21 Phase 5
// orchestration layer performs. Callers in the layer above the
// FROST signing primitive invoke it at session start; the returned
// cleanup function is the matching unbinding step the caller
// defers.
//
// Phase 5.2 ships the helper; Phase 6 wires production call sites
// to invoke it (and to feed the AttemptContext from the resolver
// adapter, etc.).
//
// When the ROAST-retry registry is empty (default build, no caller
// has registered a coordinator), the helper returns an error so
// the caller can fall back to legacy behaviour. The two-arg
// "shape" -- (handle, cleanup, error) -- forces the caller to
// handle the absence of a coordinator explicitly rather than
// silently dropping the orchestration.
func BeginOrchestrationForSession(
	sessionID string,
	ctx attempt.AttemptContext,
) (roast.AttemptHandle, func(), error) {
	if err := EnsureRoastRetryReadinessOptIn(); err != nil {
		return roast.AttemptHandle{}, nil, fmt.Errorf(
			"roast orchestration: %w",
			err,
		)
	}
	deps, ok := RegisteredRoastRetryCoordinator()
	if !ok {
		return roast.AttemptHandle{}, nil, fmt.Errorf(
			"roast orchestration: no coordinator registered; caller should fall back to legacy behaviour",
		)
	}
	if deps.Coordinator == nil {
		return roast.AttemptHandle{}, nil, fmt.Errorf(
			"roast orchestration: registered RoastRetryDeps has nil Coordinator",
		)
	}
	handle, err := deps.Coordinator.BeginAttempt(ctx)
	if err != nil {
		return roast.AttemptHandle{}, nil, fmt.Errorf(
			"roast orchestration: begin attempt for session %q: %w",
			sessionID,
			err,
		)
	}
	SetCurrentAttemptHandleForSession(sessionID, handle, ctx)
	cleanup := func() {
		ClearCurrentAttemptHandleForSession(sessionID)
	}
	return handle, cleanup, nil
}

// EndOrchestrationForSession is a convenience for callers that
// did not capture the cleanup function from
// BeginOrchestrationForSession (e.g. callers that pass session
// ownership across function boundaries). It is equivalent to
// invoking the cleanup function returned by Begin.
func EndOrchestrationForSession(sessionID string) {
	ClearCurrentAttemptHandleForSession(sessionID)
}
