//go:build frost_native && frost_roast_retry

package signing

import (
	"bytes"
	"context"
	"sync/atomic"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// RoastTransitionExchange runs one signer's session-scoped ROAST transition
// exchange over a RunnerBus (RFC-21 Phase 7.3 PR2b-1b C2). It owns the receive
// side (a listener goroutine for the session lifetime) and exposes the produce
// side the block-timed controller drives:
//
//   - listen: peers' evidence snapshots are recorded against the elected
//     coordinator's local observe handle (only the elected seat collects, for
//     aggregation); peers' transition bundles are verified against this seat's
//     own observe handle and, when valid, stored as the next-attempt record.
//   - BroadcastForcedSnapshot: a participating seat publishes a forced (empty)
//     proof-of-attendance snapshot so it appears in the aggregated bundle and is
//     not silence-parked by NextAttempt. It records its OWN snapshot before
//     broadcasting, so VerifyBundle's censorship check is meaningful.
//   - AggregateAndBroadcast: the elected seat aggregates the collected snapshots
//     into a coordinator-signed bundle, stores it locally via the same
//     verify+store path receivers use, and broadcasts it. A no-op on a seat that
//     is not the elected coordinator (AggregateBundle returns ErrNotAggregator).
//
// Every binding lookup resolves the per-attempt observe handle the C1 observe
// step stored, keyed by (roastSessionID, member, attemptContextHash). The
// exchange never trusts an unsigned outer bus field over the signed body: it
// resolves bindings and roles from the snapshot/bundle's own AttemptContextHash.
type RoastTransitionExchange struct {
	ctx            context.Context
	logger         log.StandardLogger
	bus            RunnerBus
	deps           RoastRetryDeps
	roastSessionID string
	member         group.MemberIndex
	sub            *RunnerBusSubscriber
	// lostSync is set by the listener when a transition bundle arrives for an
	// attempt this seat never observed (it skipped a window peers committed). The
	// retry loop reads it via the controller from its own goroutine, hence atomic.
	lostSync atomic.Bool
}

// NewRoastTransitionExchange constructs the exchange and starts its listener for
// the lifetime of ctx (cancel ctx -- e.g. at session end -- to stop it). It
// subscribes to the bus before returning so no peer message broadcast after
// construction is missed.
func NewRoastTransitionExchange(
	ctx context.Context,
	logger log.StandardLogger,
	bus RunnerBus,
	deps RoastRetryDeps,
	roastSessionID string,
	member group.MemberIndex,
) *RoastTransitionExchange {
	if logger == nil {
		logger = log.Logger("keep-frost-roast-transition-exchange")
	}
	e := &RoastTransitionExchange{
		ctx:            ctx,
		logger:         logger,
		bus:            bus,
		deps:           deps,
		roastSessionID: roastSessionID,
		member:         member,
		sub:            bus.Subscribe(),
	}
	go e.listen()
	return e
}

func (e *RoastTransitionExchange) listen() {
	// On session end (ctx done), drop any observe bindings this seat did not
	// consume per-attempt -- e.g. a signing whose attempts all succeeded never
	// produced a transition record to clear, so its bindings would otherwise
	// linger until the TTL sweep.
	defer clearObservedAttemptsForSession(e.roastSessionID, e.member)
	for {
		select {
		case <-e.ctx.Done():
			return
		case msg := <-e.sub.EvidenceSnapshots():
			e.onSnapshot(msg)
		case msg := <-e.sub.TransitionBundles():
			e.onBundle(msg)
		}
	}
}

// onSnapshot records a peer's evidence snapshot against this seat's observe
// handle for the attempt, but ONLY when this seat is the attempt's elected
// coordinator -- it is the only seat that aggregates, so other seats need not
// collect. The snapshot's signed AttemptContextHash (not the unsigned bus field)
// resolves the binding, and the authenticated bus sender must match the claimed
// submitter.
func (e *RoastTransitionExchange) onSnapshot(msg RunnerMessage) {
	snapshot := &roast.LocalEvidenceSnapshot{}
	if err := snapshot.Unmarshal(msg.Payload); err != nil {
		return
	}
	if snapshot.SenderID() != msg.Sender {
		// A seat embedding another member's id: drop, do not let it fill that
		// member's slot.
		return
	}
	hash := snapshot.AttemptContextHashArray()
	binding, ok := observedAttempt(e.roastSessionID, e.member, hash)
	if !ok {
		return
	}
	elected, err := e.deps.Coordinator.SelectedCoordinator(binding.handle)
	if err != nil || elected != group.MemberIndex(e.deps.SelfMember) {
		// Only the elected coordinator collects snapshots for aggregation.
		return
	}
	if err := e.deps.Coordinator.RecordEvidence(binding.handle, snapshot); err != nil {
		e.logger.Warnf(
			"roast transition: record peer snapshot from [%d] failed: [%v]",
			msg.Sender, err,
		)
	}
}

// onBundle verifies a received transition bundle against this seat's observe
// handle and, when valid, stores the next-attempt record. The bundle's claimed
// coordinator must equal the authenticated bus sender (hygiene; VerifyBundle then
// authenticates the coordinator signature cryptographically).
func (e *RoastTransitionExchange) onBundle(msg RunnerMessage) {
	bundle := &roast.TransitionMessage{}
	if err := bundle.Unmarshal(msg.Payload); err != nil {
		return
	}
	if bundle.CoordinatorID() != msg.Sender {
		return
	}
	hash := bundle.AttemptContextHashArray()
	if !attemptEverObserved(e.roastSessionID, e.member, hash) {
		// A bundle for an attempt this seat NEVER observed: peers committed an
		// attempt this seat skipped (its local block schedule moved past the
		// announcement window while peers stayed in it), so this seat is behind the
		// group's committed ROAST attempt chain. Flag lost sync; the retry loop
		// fails closed before its next selection rather than select a divergent
		// included set (the fracture class).
		//
		// The bundle is authenticated (the bus binds msg.Sender to an operator seat
		// and the claimed coordinator must equal it) but UNVERIFIABLE here:
		// VerifyBundle needs the local observe handle this seat never created. An
		// authenticated member could thus broadcast a bundle for a bogus hash to
		// force a peer's fail-closed -- an insider-liveness surface that the blame
		// bridge addresses (PR2b-2) and that stays within 1b's accepted
		// fail-closed-terminate liveness regression (a single bad actor can already
		// kill a round by withholding a bundle).
		e.markLostSync()
		return
	}
	e.verifyAndStore(bundle)
}

// markLostSync records that this seat received a transition for an attempt it
// never observed -- it fell behind the group's committed ROAST attempt chain.
func (e *RoastTransitionExchange) markLostSync() {
	e.lostSync.Store(true)
}

// HasLostSync reports whether this seat fell behind the group's committed ROAST
// attempt chain (it received a transition for an attempt it never observed). The
// retry loop checks it before selection and fails closed when true.
func (e *RoastTransitionExchange) HasLostSync() bool {
	return e.lostSync.Load()
}

// BroadcastForcedSnapshot publishes this seat's forced (empty) proof-of-attendance
// snapshot for the attempt, recording it locally BEFORE the broadcast so the
// censorship check on the returned bundle is meaningful. A no-op when the seat
// has no observe binding for the attempt or signing fails.
func (e *RoastTransitionExchange) BroadcastForcedSnapshot(
	attemptHash [attempt.MessageDigestLength]byte,
) {
	binding, ok := observedAttempt(e.roastSessionID, e.member, attemptHash)
	if !ok {
		return
	}
	snapshot := roast.NewLocalEvidenceSnapshot(e.member, attemptHash, attempt.Evidence{})
	payload, err := snapshot.SignableBytes()
	if err != nil {
		e.logger.Warnf("roast transition: forced snapshot signable bytes: [%v]", err)
		return
	}
	signature, err := e.deps.Signer.Sign(payload)
	if err != nil {
		e.logger.Warnf("roast transition: sign forced snapshot: [%v]", err)
		return
	}
	snapshot.OperatorSignature = signature

	// Marshal first so the wire bytes are fixed, then record OWN before
	// broadcasting: the elected coordinator must have it for aggregation, and
	// recording-before-broadcast makes VerifyBundle's self-submission censorship
	// check meaningful -- the recorded snapshot is byte-identical to the
	// broadcast one and to the copy peers aggregate.
	envelope, err := snapshot.Marshal()
	if err != nil {
		e.logger.Warnf("roast transition: marshal forced snapshot: [%v]", err)
		return
	}
	if err := e.deps.Coordinator.RecordEvidence(binding.handle, snapshot); err != nil {
		e.logger.Warnf("roast transition: record own snapshot failed: [%v]", err)
		return
	}
	e.bus.Broadcast(RunnerMessage{
		Type:    RunnerMsgEvidenceSnapshot,
		Sender:  e.member,
		Attempt: attemptHash,
		Payload: envelope,
	})
}

// AggregateAndBroadcast aggregates the collected snapshots into a
// coordinator-signed bundle, stores it locally via the same verify+store path
// receivers use, and broadcasts it. A no-op on any seat that is not the attempt's
// elected coordinator (AggregateBundle returns ErrNotAggregator) or that has no
// observe binding.
func (e *RoastTransitionExchange) AggregateAndBroadcast(
	attemptHash [attempt.MessageDigestLength]byte,
) {
	binding, ok := observedAttempt(e.roastSessionID, e.member, attemptHash)
	if !ok {
		return
	}
	bundle, err := e.deps.Coordinator.AggregateBundle(binding.handle)
	if err != nil {
		// Not the elected coordinator, already transitioned, or an empty bundle:
		// nothing to broadcast. Non-elected seats reach here harmlessly.
		return
	}

	// Store our own record the same way receivers do, then broadcast (the bus
	// does not echo our own message back).
	e.verifyAndStore(bundle)

	envelope, err := bundle.Marshal()
	if err != nil {
		e.logger.Warnf("roast transition: marshal bundle: [%v]", err)
		return
	}
	e.bus.Broadcast(RunnerMessage{
		Type:    RunnerMsgTransitionBundle,
		Sender:  e.member,
		Attempt: attemptHash,
		Payload: envelope,
	})
}

// verifyAndStore is the single verify+store path both receivers (onBundle) and
// the producing coordinator (AggregateAndBroadcast) use, so a stored record is
// always a verified one. It resolves this seat's observe binding from the
// bundle's signed AttemptContextHash, verifies the bundle against that handle,
// stores the next-attempt record, and clears the consumed observe binding.
func (e *RoastTransitionExchange) verifyAndStore(bundle *roast.TransitionMessage) {
	hash := bundle.AttemptContextHashArray()
	binding, ok := observedAttempt(e.roastSessionID, e.member, hash)
	if !ok {
		return
	}
	if err := e.deps.Coordinator.VerifyBundle(binding.handle, bundle); err != nil {
		e.logger.Warnf("roast transition: verify bundle failed: [%v]", err)
		return
	}
	// Defensive: the bundle's hash must match the binding's bound context hash
	// (VerifyBundle binds to the handle, but keep the record self-consistent).
	boundHash := binding.context.Hash()
	if !bytes.Equal(hash[:], boundHash[:]) {
		return
	}
	RecordRoastTransition(e.roastSessionID, e.member, RoastTransitionRecord{
		Bundle:            bundle,
		PreviousHandle:    binding.handle,
		PreviousContext:   binding.context,
		DkgGroupPublicKey: binding.dkgGroupPublicKey,
	})
	clearObservedAttempt(e.roastSessionID, e.member, hash)
}
