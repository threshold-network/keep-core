package roast

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// AttemptState is the phase an attempt is in within the Coordinator
// state machine. The lifecycle is monotonic:
//
//	AttemptStatePending -> AttemptStateCollecting -> AttemptStateAggregating
//	    -> {AttemptStateSucceeded, AttemptStateTransitioned}
//
// AttemptStateSucceeded means the attempt produced a final signature.
// AttemptStateTransitioned means the attempt timed out or hit an
// unrecoverable reject and the coordinator emitted a
// TransitionMessage that drives the next attempt's context. Phase 3.1
// (this file) introduces the state surface only; later phases drive
// the transitions.
type AttemptState uint8

const (
	// AttemptStatePending is the zero value -- not a real state, used
	// only as the default-initialised "unknown" sentinel returned with
	// ErrUnknownAttempt.
	AttemptStatePending AttemptState = iota
	// AttemptStateCollecting -- the attempt has been started, the
	// included set is fixed, and the coordinator is accepting signed
	// evidence snapshots from peers.
	AttemptStateCollecting
	// AttemptStateAggregating -- the coordinator has stopped
	// accepting evidence and is building the TransitionMessage
	// bundle.
	AttemptStateAggregating
	// AttemptStateSucceeded -- the attempt produced a final
	// signature; no transition message is needed.
	AttemptStateSucceeded
	// AttemptStateTransitioned -- the attempt timed out or failed
	// and the coordinator has emitted a TransitionMessage; the next
	// attempt's context can now be computed by NextAttempt.
	AttemptStateTransitioned
)

func (s AttemptState) String() string {
	switch s {
	case AttemptStatePending:
		return "pending"
	case AttemptStateCollecting:
		return "collecting"
	case AttemptStateAggregating:
		return "aggregating"
	case AttemptStateSucceeded:
		return "succeeded"
	case AttemptStateTransitioned:
		return "transitioned"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(s))
	}
}

// AttemptHandle is the opaque per-attempt identity returned by
// Coordinator.BeginAttempt. Handles are not interchangeable across
// coordinator instances: a handle minted by coordinator A cannot be
// passed to coordinator B. Callers must not mutate handles directly.
type AttemptHandle struct {
	id          uint64
	contextHash [attempt.MessageDigestLength]byte
}

// ContextHash returns the canonical AttemptContext.Hash() value bound
// to this handle. Useful for cross-checking a handle against a
// context after the fact.
func (h AttemptHandle) ContextHash() [attempt.MessageDigestLength]byte {
	return h.contextHash
}

// Coordinator is the ROAST coordinator state machine introduced by
// RFC-21 Phase 3. It owns per-attempt state, the deterministic
// participant selection (via the existing SelectCoordinator helper),
// and -- in later Phase-3 PRs -- signed-evidence aggregation,
// transition-message construction, and the NextAttempt policy.
//
// Phase 3.1 (this file) introduces only:
//   - BeginAttempt: initialise tracking for a new attempt.
//   - State: read the current AttemptState for a handle.
//   - SelectedCoordinator: report the member elected as coordinator
//     for the attempt.
//
// Phase 3.2 adds the TransitionMessage / LocalEvidenceSnapshot types.
// Phase 3.3 adds AggregateBundle and VerifyBundle. Phase 3.4 adds the
// NextAttempt policy function.
//
// Implementations must be safe for concurrent calls from multiple
// goroutines; production keep-core code paths are network-driven.
type Coordinator interface {
	// BeginAttempt initialises tracking for a new attempt with the
	// given context. It selects the attempt's coordinator
	// deterministically from ctx.IncludedSet via SelectCoordinator
	// (with the legacy int64 seed produced by foldAttemptSeed) and
	// stores the result on the returned handle.
	BeginAttempt(ctx attempt.AttemptContext) (AttemptHandle, error)
	// State returns the current AttemptState for the given handle.
	// Returns ErrUnknownAttempt if the handle was not produced by
	// this Coordinator instance.
	State(handle AttemptHandle) (AttemptState, error)
	// SelectedCoordinator returns the member elected as coordinator
	// for the attempt identified by the handle. Returns
	// ErrUnknownAttempt if the handle is not tracked.
	SelectedCoordinator(handle AttemptHandle) (group.MemberIndex, error)
}

// ErrUnknownAttempt indicates an AttemptHandle does not correspond to
// any attempt tracked by this Coordinator. Either the handle was
// minted by a different coordinator instance, or the attempt has
// been pruned.
var ErrUnknownAttempt = errors.New("coordinator: unknown attempt handle")

// NewInMemoryCoordinator returns a Coordinator that tracks attempts
// in-process. Phase 3 production paths use this implementation.
// Later phases may add persistent variants once persistence is
// designed (RFC-21 Open question on signer restart).
func NewInMemoryCoordinator() Coordinator {
	return &inMemoryCoordinator{
		attempts: map[uint64]*attemptRecord{},
	}
}

type attemptRecord struct {
	handle      AttemptHandle
	context     attempt.AttemptContext
	coordinator group.MemberIndex
	state       AttemptState
}

type inMemoryCoordinator struct {
	mu       sync.Mutex
	nextID   atomic.Uint64
	attempts map[uint64]*attemptRecord
}

func (c *inMemoryCoordinator) BeginAttempt(
	ctx attempt.AttemptContext,
) (AttemptHandle, error) {
	if len(ctx.IncludedSet) == 0 {
		return AttemptHandle{}, fmt.Errorf(
			"coordinator: cannot begin attempt with empty included set",
		)
	}
	coord, err := SelectCoordinator(
		ctx.IncludedSet,
		foldAttemptSeed(ctx.AttemptSeed),
		uint(ctx.AttemptNumber),
	)
	if err != nil {
		return AttemptHandle{}, fmt.Errorf(
			"coordinator: selection failed: %w",
			err,
		)
	}
	handle := AttemptHandle{
		id:          c.nextID.Add(1),
		contextHash: ctx.Hash(),
	}
	record := &attemptRecord{
		handle:      handle,
		context:     ctx,
		coordinator: coord,
		state:       AttemptStateCollecting,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attempts[handle.id] = record
	return handle, nil
}

func (c *inMemoryCoordinator) State(
	handle AttemptHandle,
) (AttemptState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	record, ok := c.attempts[handle.id]
	if !ok {
		return AttemptStatePending, ErrUnknownAttempt
	}
	return record.state, nil
}

func (c *inMemoryCoordinator) SelectedCoordinator(
	handle AttemptHandle,
) (group.MemberIndex, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	record, ok := c.attempts[handle.id]
	if !ok {
		return 0, ErrUnknownAttempt
	}
	return record.coordinator, nil
}
