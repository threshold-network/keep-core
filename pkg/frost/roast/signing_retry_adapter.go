package roast

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// MemberToParticipantResolver maps a ROAST group.MemberIndex to the
// participant-identifier type the legacy signing-retry path uses
// (typically chain.Address in keep-core production flows, but the
// interface is intentionally generic in T so pkg/frost/roast does
// not import any caller-side type).
//
// Implementations are wallet-scoped: each FROST signing flow
// constructs a resolver from its existing wallet/group state at the
// call site and passes it to EvaluateRoastRetryForSigning or
// SigningRetryAdapter.
type MemberToParticipantResolver[T any] interface {
	// For returns the participant identifier corresponding to the
	// given member index. Returns an error if the member is unknown
	// to the resolver (out-of-range index, evicted member, etc.).
	For(member group.MemberIndex) (T, error)
}

// EvaluateRoastRetryForSigning bridges the ROAST coordinator state
// machine with the legacy signing-retry shape. Given the previous
// attempt's handle and a verified TransitionMessage, it computes
// the next attempt's IncludedSet, converts each member index to its
// resolver-supplied participant identifier, and returns both the
// participant list and the full AttemptContext.
//
// Callers MUST call Coordinator.VerifyBundle on bundle before
// passing it to this function; the bundle is the load-bearing
// authoritative input to NextAttempt and an unverified bundle would
// silently fracture multi-instance agreement.
//
// Returns ErrAttemptInfeasible directly when the next attempt's
// included set would drop below threshold; the caller must
// propagate that to the session manager rather than swallow it.
// See RFC-21 Phase-5 Resolved Decision on infeasibility.
//
// The function is generic in T so it can be used with chain.Address
// in production keep-core flows and with simple test types
// (strings, ints) in unit tests.
func EvaluateRoastRetryForSigning[T any](
	coord Coordinator,
	handle AttemptHandle,
	bundle *TransitionMessage,
	threshold uint,
	dkgGroupPublicKey []byte,
	resolver MemberToParticipantResolver[T],
) ([]T, attempt.AttemptContext, error) {
	if coord == nil {
		return nil, attempt.AttemptContext{}, fmt.Errorf(
			"roast retry adapter: coordinator is nil",
		)
	}
	if resolver == nil {
		var zero T
		_ = zero
		return nil, attempt.AttemptContext{}, fmt.Errorf(
			"roast retry adapter: resolver is nil",
		)
	}
	nextCtx, err := coord.NextAttempt(handle, bundle, threshold, dkgGroupPublicKey)
	if err != nil {
		return nil, attempt.AttemptContext{}, err
	}
	participants := make([]T, 0, len(nextCtx.IncludedSet))
	for _, m := range nextCtx.IncludedSet {
		t, err := resolver.For(m)
		if err != nil {
			return nil, attempt.AttemptContext{}, fmt.Errorf(
				"roast retry adapter: resolver failed for member %d: %w",
				m,
				err,
			)
		}
		participants = append(participants, t)
	}
	return participants, nextCtx, nil
}

// SigningRetryAdapter binds the inputs to EvaluateRoastRetryForSigning
// onto a struct so call sites can hold the configuration once and
// call EvaluateRetryParticipantsForSigning (legacy-shaped) per
// retry. Phase 6 migrates call sites to either the function or the
// struct -- whichever fits the existing call shape.
type SigningRetryAdapter[T any] struct {
	Coordinator       Coordinator
	Handle            AttemptHandle
	Bundle            *TransitionMessage
	Threshold         uint
	DkgGroupPublicKey []byte
	Resolver          MemberToParticipantResolver[T]
}

// EvaluateRetryParticipantsForSigning matches the shape of the
// legacy helper in pkg/protocol/retry so call sites can adopt the
// adapter without changing their function-call surface. The legacy
// signature's parameters (groupMembers, seed, retryCount,
// retryParticipantsCount) are ignored: the AttemptContext bound to
// the handle is the source of truth for next-attempt selection.
//
// Returns the next IncludedSet's participants and any error from
// NextAttempt (typically ErrAttemptInfeasible).
func (a SigningRetryAdapter[T]) EvaluateRetryParticipantsForSigning(
	_ []T,
	_ int64,
	_ uint,
	_ uint,
) ([]T, error) {
	participants, _, err := EvaluateRoastRetryForSigning(
		a.Coordinator,
		a.Handle,
		a.Bundle,
		a.Threshold,
		a.DkgGroupPublicKey,
		a.Resolver,
	)
	return participants, err
}

// NextAttemptContext returns the AttemptContext the adapter would
// transition to. Useful when callers need both the participant
// list and the context (e.g. to re-bind session orchestration to
// the new attempt's handle).
func (a SigningRetryAdapter[T]) NextAttemptContext() (
	attempt.AttemptContext, error,
) {
	_, ctx, err := EvaluateRoastRetryForSigning(
		a.Coordinator,
		a.Handle,
		a.Bundle,
		a.Threshold,
		a.DkgGroupPublicKey,
		a.Resolver,
	)
	return ctx, err
}
