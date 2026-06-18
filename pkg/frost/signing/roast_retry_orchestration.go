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
	"github.com/keep-network/keep-core/pkg/protocol/group"
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
func BeginOrchestrationForSession(
	sessionID string,
	ctx attempt.AttemptContext,
	member group.MemberIndex,
	dkgGroupPublicKey []byte,
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
		// RFC-21 Phase 7.1/7.3: if this seat is the elected
		// coordinator and the attempt is still Collecting at
		// cleanup time (i.e. it did not succeed via signature
		// aggregation), produce the TransitionMessage and stash a
		// full RoastTransitionRecord (bundle + this attempt's
		// handle/context + the DKG group public key) keyed by
		// (sessionID, member). The next attempt's ROAST
		// signingParticipantSelector consumes it to compute the
		// IncludedSet via EvaluateRoastRetryForSigning. The record
		// carries the handle/context so the selector does not race
		// the ClearCurrentAttemptHandleForSession below, and the
		// DKG key so NextAttempt can derive a valid next context.
		//
		// Failures are best-effort and silent: a panic in the
		// deferred cleanup is materially worse than a missing
		// transition record (the next attempt's selector falls
		// back to the legacy retry shuffle), so we swallow errors
		// rather than propagate them.
		// The transition record is keyed by the STABLE ctx.SessionID
		// (== RoastSessionID) so the next attempt's selector finds it; the
		// handle registry above stays keyed by the attempt-specific sessionID.
		maybeProduceTransitionRecord(ctx.SessionID, member, handle, ctx, dkgGroupPublicKey, deps)
		ClearCurrentAttemptHandleForSession(sessionID)
	}
	return handle, cleanup, nil
}

// maybeProduceTransitionRecord attempts to call AggregateBundle on
// the registered Coordinator when (a) this seat (member) is the
// elected coordinator for the attempt and (b) the attempt has not
// already transitioned, then stashes a full RoastTransitionRecord
// keyed by (sessionID, member) via RecordRoastTransition (a no-op
// in the default build). On any error path the function returns
// silently because cleanup must not break the signing-flow
// contract.
//
// The elected check uses the per-seat member (not deps.SelfMember)
// so a multi-seat operator's elected-coordinator seat -- whichever
// it is -- is the one that produces and stores the record under its
// own member. Non-coordinator seats produce nothing here; they
// receive the bundle via the (Phase 7.3 PR2b) snapshot/transition
// bus exchange.
//
// In the default build this still compiles because
// RecordRoastTransition is a no-op stub; calls to roast.Coordinator
// methods compile because pkg/frost/roast is not build-tagged.
func maybeProduceTransitionRecord(
	roastSessionID string,
	member group.MemberIndex,
	handle roast.AttemptHandle,
	ctx attempt.AttemptContext,
	dkgGroupPublicKey []byte,
	deps RoastRetryDeps,
) {
	if deps.Coordinator == nil || member == 0 {
		return
	}
	elected, err := deps.Coordinator.SelectedCoordinator(handle)
	if err != nil {
		return
	}
	if elected != member {
		return
	}
	state, err := deps.Coordinator.State(handle)
	if err != nil {
		return
	}
	if state != roast.AttemptStateCollecting {
		// Already transitioned or succeeded -- nothing to do.
		return
	}
	bundle, err := deps.Coordinator.AggregateBundle(handle)
	if err != nil {
		// Best-effort; the next attempt's selector will fall
		// back to the legacy retry shuffle.
		return
	}
	RecordRoastTransition(roastSessionID, member, RoastTransitionRecord{
		Bundle:            bundle,
		PreviousHandle:    handle,
		PreviousContext:   ctx,
		DkgGroupPublicKey: dkgGroupPublicKey,
	})
}

// EndOrchestrationForSession is a convenience for callers that
// did not capture the cleanup function from
// BeginOrchestrationForSession (e.g. callers that pass session
// ownership across function boundaries). It is equivalent to
// invoking the cleanup function returned by Begin.
func EndOrchestrationForSession(sessionID string) {
	ClearCurrentAttemptHandleForSession(sessionID)
}
