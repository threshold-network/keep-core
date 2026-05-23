package signing

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
		// RFC-21 Phase 7.1: if this node is the elected
		// coordinator and the attempt is still in the Collecting
		// state at cleanup time (i.e. it did not succeed via
		// signature aggregation), produce the TransitionMessage
		// and stash it in the per-session bundle registry. Phase
		// 7.2's ROAST signingParticipantSelector consumes the
		// stashed bundle to compute the next attempt's
		// IncludedSet via EvaluateRoastRetryForSigning.
		//
		// Failures are best-effort and silent: a panic in the
		// deferred cleanup is materially worse than a missing
		// transition bundle (the next attempt's selector falls
		// back to the legacy retry shuffle), so we swallow errors
		// rather than propagate them.
		maybeProduceTransitionBundle(sessionID, handle, deps)
		ClearCurrentAttemptHandleForSession(sessionID)
	}
	return handle, cleanup, nil
}

// maybeProduceTransitionBundle attempts to call AggregateBundle on
// the registered Coordinator when (a) the local node is the
// elected coordinator for the attempt and (b) the attempt has not
// already transitioned. The result is stashed via
// RecordTransitionBundleForSession (a no-op in default build); on
// any error path the function returns silently because cleanup
// must not break the signing-flow contract.
//
// In the default build this still compiles because
// RecordTransitionBundleForSession is a no-op stub; calls to
// roast.Coordinator methods compile because pkg/frost/roast is
// not build-tagged.
func maybeProduceTransitionBundle(
	sessionID string,
	handle roast.AttemptHandle,
	deps RoastRetryDeps,
) {
	if deps.Coordinator == nil {
		return
	}
	if deps.SelfMember == 0 {
		// Without a known self-member, we cannot determine
		// whether to aggregate. Skip.
		return
	}
	elected, err := deps.Coordinator.SelectedCoordinator(handle)
	if err != nil {
		return
	}
	if elected != group.MemberIndex(deps.SelfMember) {
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
	RecordTransitionBundleForSession(sessionID, bundle)
}

// EndOrchestrationForSession is a convenience for callers that
// did not capture the cleanup function from
// BeginOrchestrationForSession (e.g. callers that pass session
// ownership across function boundaries). It is equivalent to
// invoking the cleanup function returned by Begin.
func EndOrchestrationForSession(sessionID string) {
	ClearCurrentAttemptHandleForSession(sessionID)
}
