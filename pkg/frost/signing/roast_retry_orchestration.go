package signing

// Static-vs-runtime error taxonomy (RFC-21 Phase 6 — Resolved Decision).
//
// The orchestration layer in this file participates in a load-bearing
// decision that prevents split-brain group fracture in the ROAST retry
// path. Errors returned through the orchestration boundary are
// classified into one of three categories, and the consumer (the
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
//   RUNTIME errors -> NO FALLBACK, RETRY THE NEXT ATTEMPT. Any error
//                     that arises from per-attempt protocol state
//                     (BeginAttempt internals, AttemptContext binding
//                     mismatches, transition-bundle verification
//                     failures, etc.) can be observed by some
//                     participants and not others within the same
//                     attempt. Falling back to legacy under those
//                     conditions would leave some operators running the
//                     new code path and others running legacy on the
//                     same attempt -- the canonical definition of
//                     split-brain fracture. The orchestration layer
//                     therefore returns these as bare (non-sentinel)
//                     errors; the signingRetryLoop does NOT fall back to
//                     coarse, but it DOES retry on the next attempt
//                     because the fault may be transient and clear.
//
//   TERMINAL errors -> ABORT THE RETRY LOOP. A STATIC condition that no
//                     future attempt can resolve, e.g. a partially-
//                     registered multi-seat operator: THIS seat has no
//                     ROAST coordinator while a sibling seat is ROAST-
//                     active, so this seat can neither drive ROAST nor
//                     safely fall back to legacy (that would fracture
//                     the attempt). (PR2b-2 member-keyed the handle
//                     binding, so a FULLY-registered multi-seat operator
//                     is no longer terminal -- it proceeds per-seat.)
//                     Unlike a RUNTIME error, retrying is FUTILE: every attempt
//                     re-derives the same static outcome, so the loop
//                     would spin until timeout AND synthesize garbage
//                     failed-attempt transitions (OnAttemptFailed).
//                     Coarse fallback is also unsafe (interactive<->coarse
//                     mixing fractures the group), so terminating is the
//                     only non-fracturing option. The orchestration layer
//                     wraps ErrTerminalSigningFailure; the signingRetryLoop
//                     matches it via errors.Is and exits immediately
//                     (return nil, err) BEFORE the retry/transition
//                     machinery.
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

// ErrTerminalSigningFailure classifies an orchestration error as TERMINAL: a
// static condition no future attempt can resolve, so the signingRetryLoop must
// ABORT the loop (return nil, err) rather than retry the next attempt. It is the
// third disposition in the taxonomy above. Orchestration code wraps it
// (fmt.Errorf("%w: ...", ErrTerminalSigningFailure)) and the loop matches it via
// errors.Is. It is distinct from ErrNoRoastRetryCoordinatorRegistered (STATIC,
// coarse-fallback) and from bare RUNTIME errors (no fallback, but retried): a
// TERMINAL error is futile to retry and unsafe to coarse-fall-back, so the only
// non-fracturing disposition is to stop.
//
// Current sole producer: BeginOrchestrationForSession, for a partially-registered
// multi-seat operator -- this seat has no registered coordinator while a sibling
// seat is ROAST-active. (A fully-registered multi-seat operator is no longer
// terminal as of PR2b-2's member-keyed handle binding.)
//
// Use errors.Is to detect.
var ErrTerminalSigningFailure = errors.New(
	"terminal signing failure",
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
	member group.MemberIndex,
	ctx attempt.AttemptContext,
) (roast.AttemptHandle, func(), error) {
	if err := EnsureRoastRetryReadinessOptIn(); err != nil {
		return roast.AttemptHandle{}, nil, fmt.Errorf(
			"roast orchestration: %w",
			err,
		)
	}
	// RFC-21 Phase 7.3 PR2b-1.5: mint the handle from THIS seat's coordinator, so a
	// multi-seat operator's elected seat aggregates with its own binding. PR2b-2
	// member-keyed the handle binding below (SetCurrentAttemptHandleForSession), so a
	// fully-registered multi-seat operator now proceeds per-seat with isolated
	// bindings -- the PR2b-1.5 `count > 1` fail-closed guard that stood here is gone.
	deps, ok := RegisteredRoastRetryCoordinatorForKeyGroupMember(ctx.KeyGroupID, member)
	if !ok {
		// THIS seat has no registered coordinator for THIS wallet's key group. Whether
		// that means legacy fallback or fail-closed hinges on whether ROAST is active
		// for THIS wallet, which is group-uniform for a key group (every honest node in
		// the wallet's group sees the same per-key-group registration state). The count
		// is scoped to ctx.KeyGroupID so a second, independently-configured wallet on
		// this node never perturbs the decision:
		//   - count == 0: no seat of THIS wallet is registered, so ROAST is inactive
		//     for this wallet -- a uniform condition every honest node decides
		//     identically -> safe legacy fallback (the static sentinel).
		//   - count > 0: a sibling seat of THIS wallet IS ROAST-active (a partially-
		//     registered operator). Advertising the legacy fallback for THIS seat while
		//     the sibling drives bound ROAST would fracture the attempt (some seats
		//     bound, one seat legacy-shuffle) -> fail CLOSED (terminal). A seat with no
		//     coordinator cannot participate in ROAST regardless of key shape, so this
		//     fail-closed survives PR2b-2 (only the fully-registered multi-seat guard
		//     was retired).
		if registeredRoastRetryMemberCount(ctx.KeyGroupID) == 0 {
			return roast.AttemptHandle{}, nil, fmt.Errorf(
				"%w: caller should fall back to legacy behaviour",
				ErrNoRoastRetryCoordinatorRegistered,
			)
		}
		return roast.AttemptHandle{}, nil, fmt.Errorf(
			"%w: seat %d has no registered coordinator while a sibling "+
				"seat is ROAST-active; fail closed",
			ErrTerminalSigningFailure,
			member,
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
	// RFC-21 Phase 7.3 PR2b-2: bind per (sessionID, member). A multi-seat
	// operator's sibling seats each bind their own handle here, so neither
	// overwrites the other and each seat's cleanup clears only its own binding.
	SetCurrentAttemptHandleForSession(sessionID, member, handle, ctx)
	cleanup := func() {
		// RFC-21 Phase 7.3 PR2b-1b: the cleanup ONLY clears the per-attempt
		// handle binding. It no longer produces a transition record -- the
		// session-scoped transition exchange (the observe binding + forced-
		// snapshot aggregation + bundle distribution) is now the SOLE producer,
		// keyed by the observe handle. Producing here too would let this drive
		// handle's (empty) aggregation write a SECOND, possibly divergent record
		// for the same (sessionID, member) the exchange owns -- a fracture risk.
		ClearCurrentAttemptHandleForSession(sessionID, member)
	}
	return handle, cleanup, nil
}

// EndOrchestrationForSession is a convenience for callers that
// did not capture the cleanup function from
// BeginOrchestrationForSession (e.g. callers that pass session
// ownership across function boundaries). It is equivalent to
// invoking the cleanup function returned by Begin, so it clears the
// binding for this (session, member) seat.
func EndOrchestrationForSession(sessionID string, member group.MemberIndex) {
	ClearCurrentAttemptHandleForSession(sessionID, member)
}
