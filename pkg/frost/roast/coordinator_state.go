package roast

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
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
// signed-evidence aggregation, transition-message construction, and
// -- in Phase 3.4 -- the NextAttempt policy.
//
// Phase 3.1 introduced BeginAttempt, State, and SelectedCoordinator.
// Phase 3.3 (this commit) adds RecordEvidence, AggregateBundle, and
// VerifyBundle.
// Phase 3.4 will add NextAttempt.
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
	// RecordEvidence stores a peer's signed LocalEvidenceSnapshot
	// against the named attempt. The snapshot is validated for
	// structural correctness, its OperatorSignature is verified
	// against the configured SignatureVerifier, and its
	// AttemptContextHash is checked to match the handle's bound
	// context. First-write-wins / equal-or-reject semantics apply:
	// a peer that re-submits the same byte-identical snapshot is
	// idempotent; a peer that mutates its snapshot returns an error
	// without overwriting the originally accepted one.
	RecordEvidence(handle AttemptHandle, snapshot *LocalEvidenceSnapshot) error
	// AggregateBundle is called by the elected coordinator's node
	// to produce a TransitionMessage from the accumulated evidence
	// snapshots. The bundle is sorted ascending by SenderID, signed
	// with the coordinator's Signer, and the attempt state is
	// transitioned to AttemptStateAggregating then
	// AttemptStateTransitioned.
	//
	// Returns ErrNotAggregator if the caller is not the elected
	// coordinator for the attempt (the Coordinator's selfMember
	// must equal SelectedCoordinator(handle)).
	AggregateBundle(handle AttemptHandle) (*TransitionMessage, error)
	// VerifyBundle is called by every receiver of a
	// TransitionMessage. It validates the structural invariants of
	// the bundle, verifies the coordinator-level signature against
	// the attempt's elected coordinator, verifies each contained
	// snapshot's operator signature, and -- if the receiver has
	// already submitted its own snapshot via RecordEvidence with
	// the local Signer applied -- verifies that the receiver's own
	// snapshot is present and byte-identical in the bundle
	// (censorship detection).
	//
	// Returns ErrCensorshipDetected when the receiver's own
	// submitted snapshot is missing or mutated. Returns
	// ErrSignatureInvalid when any signature fails verification.
	VerifyBundle(handle AttemptHandle, msg *TransitionMessage) error
	// NextAttempt computes the deterministic next AttemptContext
	// from a verified TransitionMessage. Callers MUST call
	// VerifyBundle before NextAttempt; NextAttempt does not
	// re-verify signatures.
	//
	// threshold is the FROST signing threshold t for the key group;
	// it is constant across attempts within a session. A threshold
	// of zero disables the infeasibility check (test seam).
	//
	// dkgGroupPublicKey is the DKG-validated group public key from
	// the FFI signer material (RFC-21 Decision 2). It is passed
	// here so two honest signers derive the same AttemptSeed for
	// the next attempt.
	//
	// Returns ErrAttemptInfeasible when the next IncludedSet would
	// drop below threshold.
	NextAttempt(
		handle AttemptHandle,
		bundle *TransitionMessage,
		threshold uint,
		dkgGroupPublicKey []byte,
	) (attempt.AttemptContext, error)
}

// ErrNotAggregator is returned by AggregateBundle when the caller
// is not the elected coordinator for the named attempt.
var ErrNotAggregator = errors.New(
	"coordinator: caller is not the elected coordinator for this attempt",
)

// ErrAttemptStateInvalid is returned when an operation is requested
// against an attempt in a state that does not permit it (e.g.
// AggregateBundle on an attempt already transitioned, or
// RecordEvidence on an attempt past Collecting).
var ErrAttemptStateInvalid = errors.New("coordinator: attempt state does not permit operation")

// ErrAttemptContextMismatch is returned when a snapshot's
// AttemptContextHash does not match the handle's bound context.
var ErrAttemptContextMismatch = errors.New("coordinator: snapshot attempt context hash does not match attempt")

// ErrSnapshotConflict is returned by RecordEvidence when a peer
// re-submits a snapshot whose canonical bytes differ from the
// previously-accepted snapshot for that peer in this attempt. The
// originally accepted snapshot is retained; the new submission is
// rejected (first-write-wins).
var ErrSnapshotConflict = errors.New("coordinator: snapshot conflicts with previously recorded one (first-write-wins)")

// ErrUnknownAttempt indicates an AttemptHandle does not correspond to
// any attempt tracked by this Coordinator. Either the handle was
// minted by a different coordinator instance, or the attempt has
// been pruned.
var ErrUnknownAttempt = errors.New("coordinator: unknown attempt handle")

// NewInMemoryCoordinator returns a Coordinator that tracks attempts
// in-process with no operator-key signing wired in (NoOpSigner +
// NoOpSignatureVerifier). Suitable for tests that exercise only the
// structural state-machine surface; bundle verification will accept
// any signature.
//
// Production Phase-4 callers should use
// NewInMemoryCoordinatorWithSigning to inject the node's real
// operator-key signer and the network's member-key-resolving
// verifier.
func NewInMemoryCoordinator() Coordinator {
	return NewInMemoryCoordinatorWithSigning(
		0,
		NoOpSigner(),
		NoOpSignatureVerifier(),
	)
}

// NewInMemoryCoordinatorWithSigning returns an in-memory Coordinator
// bound to the node's own member index, the node's operator-key
// Signer, and a SignatureVerifier capable of resolving every member's
// operator key. selfMember = 0 disables the censorship-detection
// check in VerifyBundle (Phase 3.3 default for unit tests; Phase 4
// always supplies a non-zero value).
func NewInMemoryCoordinatorWithSigning(
	selfMember group.MemberIndex,
	signer Signer,
	verifier SignatureVerifier,
) Coordinator {
	return &inMemoryCoordinator{
		attempts:   map[uint64]*attemptRecord{},
		selfMember: selfMember,
		signer:     signer,
		verifier:   verifier,
	}
}

type attemptRecord struct {
	handle         AttemptHandle
	context        attempt.AttemptContext
	coordinator    group.MemberIndex
	state          AttemptState
	snapshots      map[group.MemberIndex]*LocalEvidenceSnapshot
	selfSubmission *LocalEvidenceSnapshot
}

type inMemoryCoordinator struct {
	mu         sync.Mutex
	nextID     atomic.Uint64
	attempts   map[uint64]*attemptRecord
	selfMember group.MemberIndex
	signer     Signer
	verifier   SignatureVerifier
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
		snapshots:   map[group.MemberIndex]*LocalEvidenceSnapshot{},
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

func (c *inMemoryCoordinator) RecordEvidence(
	handle AttemptHandle,
	snapshot *LocalEvidenceSnapshot,
) error {
	if snapshot == nil {
		return errors.New("coordinator: snapshot is nil")
	}
	if err := snapshot.Validate(); err != nil {
		return fmt.Errorf("coordinator: snapshot invalid: %w", err)
	}
	if err := verifySnapshotSignature(c.verifier, snapshot); err != nil {
		return fmt.Errorf("coordinator: %w", err)
	}

	// Emit any equivocation evidence AFTER c.mu is released: a registered
	// observer is host telemetry (possibly a blocking write) and must not
	// run while the coordinator state mutex is held. The evidence value is
	// fully materialized (bytes copied) under the lock; emission only reads
	// that copy. Registered before the unlock defer so it runs after it.
	var pendingEvidence *EquivocationEvidence
	defer func() {
		if pendingEvidence != nil {
			emitEquivocationEvidence(*pendingEvidence)
		}
	}()

	c.mu.Lock()
	defer c.mu.Unlock()
	record, ok := c.attempts[handle.id]
	if !ok {
		return ErrUnknownAttempt
	}
	if record.state != AttemptStateCollecting {
		return fmt.Errorf(
			"%w: state is %v, want %v",
			ErrAttemptStateInvalid,
			record.state,
			AttemptStateCollecting,
		)
	}
	if !bytes.Equal(
		snapshot.AttemptContextHash,
		record.handle.contextHash[:],
	) {
		return ErrAttemptContextMismatch
	}

	if existing, present := record.snapshots[snapshot.SenderID()]; present {
		existingBytes, err := existing.SignableBytes()
		if err != nil {
			return fmt.Errorf("coordinator: existing signable bytes: %w", err)
		}
		newBytes, err := snapshot.SignableBytes()
		if err != nil {
			return fmt.Errorf("coordinator: new signable bytes: %w", err)
		}
		if !bytes.Equal(existingBytes, newBytes) ||
			!bytes.Equal(existing.OperatorSignature, snapshot.OperatorSignature) {
			pendingEvidence = &EquivocationEvidence{
				Kind:                EquivocationKindSnapshotConflict,
				AttemptContextHash:  append([]byte(nil), snapshot.AttemptContextHash...),
				Sender:              snapshot.SenderID(),
				ExistingEnvelope:    snapshotEnvelopeForEvidence(existing),
				ConflictingEnvelope: snapshotEnvelopeForEvidence(snapshot),
			}
			return ErrSnapshotConflict
		}
		// Identical re-submission: idempotent no-op.
		return nil
	}
	record.snapshots[snapshot.SenderID()] = snapshot
	if c.selfMember != 0 && snapshot.SenderID() == c.selfMember {
		record.selfSubmission = snapshot
	}
	return nil
}

func (c *inMemoryCoordinator) AggregateBundle(
	handle AttemptHandle,
) (*TransitionMessage, error) {
	c.mu.Lock()
	record, ok := c.attempts[handle.id]
	if !ok {
		c.mu.Unlock()
		return nil, ErrUnknownAttempt
	}
	if c.selfMember == 0 || record.coordinator != c.selfMember {
		c.mu.Unlock()
		return nil, ErrNotAggregator
	}
	if record.state != AttemptStateCollecting {
		c.mu.Unlock()
		return nil, fmt.Errorf(
			"%w: state is %v, want %v",
			ErrAttemptStateInvalid,
			record.state,
			AttemptStateCollecting,
		)
	}

	senders := make([]group.MemberIndex, 0, len(record.snapshots))
	for s := range record.snapshots {
		senders = append(senders, s)
	}
	sort.Slice(senders, func(i, j int) bool { return senders[i] < senders[j] })

	bundle := make([]LocalEvidenceSnapshot, 0, len(senders))
	for _, s := range senders {
		bundle = append(bundle, *record.snapshots[s])
	}

	record.state = AttemptStateAggregating
	hash := record.handle.contextHash
	coord := record.coordinator
	c.mu.Unlock()

	msg := &TransitionMessage{
		AttemptContextHash: append([]byte{}, hash[:]...),
		CoordinatorIDValue: uint32(coord),
		Bundle:             bundle,
	}
	payload, err := msg.SignableBytes()
	if err != nil {
		c.markTransitionedLocked(handle.id)
		return nil, fmt.Errorf("coordinator: bundle signable bytes: %w", err)
	}
	sig, err := c.signer.Sign(payload)
	if err != nil {
		c.markTransitionedLocked(handle.id)
		return nil, fmt.Errorf("coordinator: sign bundle: %w", err)
	}
	msg.CoordinatorSignature = sig
	if err := msg.Validate(); err != nil {
		c.markTransitionedLocked(handle.id)
		return nil, fmt.Errorf("coordinator: aggregated bundle invalid: %w", err)
	}
	c.markTransitionedLocked(handle.id)
	return msg, nil
}

func (c *inMemoryCoordinator) markTransitionedLocked(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if record, ok := c.attempts[id]; ok {
		record.state = AttemptStateTransitioned
	}
}

func (c *inMemoryCoordinator) VerifyBundle(
	handle AttemptHandle,
	msg *TransitionMessage,
) error {
	if msg == nil {
		return errors.New("coordinator: transition message is nil")
	}
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("coordinator: transition message invalid: %w", err)
	}

	c.mu.Lock()
	record, ok := c.attempts[handle.id]
	if !ok {
		c.mu.Unlock()
		return ErrUnknownAttempt
	}
	expectedCoordinator := record.coordinator
	expectedHash := record.handle.contextHash
	selfSubmission := record.selfSubmission
	c.mu.Unlock()

	if !bytes.Equal(msg.AttemptContextHash, expectedHash[:]) {
		return ErrAttemptContextMismatch
	}
	if err := verifyBundleSignature(c.verifier, msg, expectedCoordinator); err != nil {
		return fmt.Errorf("coordinator: %w", err)
	}
	for i := range msg.Bundle {
		if err := verifySnapshotSignature(c.verifier, &msg.Bundle[i]); err != nil {
			return fmt.Errorf("coordinator: bundle[%d]: %w", i, err)
		}
	}
	if err := verifyOwnObservationsPresent(msg, c.selfMember, selfSubmission); err != nil {
		return err
	}
	return nil
}
