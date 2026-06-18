package signing

// Static-vs-runtime error taxonomy (RFC-21 Phase 6 — Resolved Decision).
//
// The orchestration layer in this file participates in a load-bearing
// decision that prevents split-brain group fracture in the ROAST retry
// path. Errors returned through the orchestration boundary are
// classified into one of two categories, and the consumer (the
// signing-loop dispatcher) routes them accordingly:
//
//   STATIC errors  -> safe to fall back to the legacy retry path.
//                     Every honest signer observes the same node-local
//                     configuration state (registry population, build
//                     tags) at the same startup, so a fallback decision
//                     is deterministic across the group. No participant
//                     fork can arise from a static-error fallback.
//                     Sentinel: ErrNoRoastRetryCoordinatorRegistered.
//                     Detected via errors.Is in
//                     signing_loop_roast_dispatcher.go.
//
//   RUNTIME errors -> HARD FAIL. No fallback. Any error that arises
//                     from per-attempt protocol state (BeginAttempt
//                     internals, AttemptContext binding mismatches,
//                     transition-bundle verification failures, etc.)
//                     can be observed by some participants and not
//                     others within the same attempt. Falling back to
//                     legacy under those conditions would leave some
//                     operators running the new code path and others
//                     running legacy on the same attempt -- the canonical
//                     definition of split-brain fracture. The
//                     orchestration layer therefore returns these as
//                     bare (non-sentinel) errors that the dispatcher
//                     treats as terminal.
//
// The classification is enforced at this file's boundary: any error
// surfaced from this package that is intended to permit fallback MUST
// be the ErrNoRoastRetryCoordinatorRegistered sentinel (or wrap it for
// errors.Is matching). Wrapping ANY runtime error in the sentinel is a
// safety regression that re-enables split-brain risk; PR reviewers
// should reject it.
//
// Background: this decision was redirected during Phase 5/6 review.
// The earlier design had Coordinator.BeginAttempt failures fall back to
// the legacy retry path on the assumption that BeginAttempt was a
// cheap idempotent setup. Review identified that BeginAttempt mutates
// per-attempt state (session bindings, evidence recorder) and can fail
// from races with concurrent receives or from peer-supplied protocol
// messages -- both of which produce non-deterministic per-participant
// outcomes. The taxonomy was tightened so only true configuration
// errors are fallback-eligible.

import (
	"errors"
	"fmt"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// ErrNoRoastRetryCoordinatorRegistered is returned by
// BeginOrchestrationForSession when the package-level ROAST-retry
// registry has not been populated by a caller. The error is the
// "static configuration" class per the RFC-21 Phase-6 Resolved
// Decision on orchestration error taxonomy: it is safe to fall
// back to the legacy retry path because every honest signer
// observes the same registry state at the same node startup, so
// the fallback decision is deterministic across the group.
//
// Use errors.Is to detect.
var ErrNoRoastRetryCoordinatorRegistered = errors.New(
	"roast orchestration: no coordinator registered",
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
//
// RFC-21 Phase 7.3 PR2b-1b retired the cleanup's transition-record
// production (the transition exchange is now the sole producer), so
// this no longer takes the DKG group public key.
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
			"%w: caller should fall back to legacy behaviour",
			ErrNoRoastRetryCoordinatorRegistered,
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
		// RFC-21 Phase 7.3 PR2b-1b: the cleanup ONLY clears the per-attempt
		// handle binding. It no longer produces a transition record -- the
		// session-scoped transition exchange (the observe binding + forced-
		// snapshot aggregation + bundle distribution) is now the SOLE producer,
		// keyed by the observe handle. Producing here too would let this drive
		// handle's (empty) aggregation write a SECOND, possibly divergent record
		// for the same (sessionID, member) the exchange owns -- a fracture risk.
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
