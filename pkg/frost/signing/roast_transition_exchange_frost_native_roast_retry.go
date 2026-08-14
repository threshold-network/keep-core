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
//   - BroadcastForcedSnapshot: a participating seat publishes its proof-of-
//     attendance snapshot -- carrying the real evidence the coarse receive loop
//     stashed for the attempt, if any -- so it appears in the aggregated bundle
//     and is not silence-parked by NextAttempt. It records its OWN snapshot before
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
	// Likewise drop any stashed coarse-path evidence this seat captured but never
	// broadcast -- a succeeded attempt never reaches BroadcastForcedSnapshot, which
	// is what consumes the stash (RFC-21 Phase 7.3 PR2b-2 step 2).
	defer clearPendingEvidenceForSession(e.roastSessionID, e.member)
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
	if err != nil || elected != e.member {
		// Only the elected coordinator collects snapshots for aggregation. Compare
		// against THIS exchange's seat (e.member), not deps.SelfMember: under
		// PR2b-1.5 multi-seat, deps is per-seat so the two agree, but e.member is
		// the unambiguous seat identity and avoids relying on the deps binding.
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
		//
		// The marker is scoped to ONE attempt: the retry loop consumes it at its
		// next selection check (see ConsumeLostSync), so a bundle for a bogus hash
		// costs the receiving seat a single attempt rather than the whole signing
		// session. A seat that is genuinely behind keeps receiving such bundles and
		// keeps skipping, which is self-correcting; an attacker must keep spamming,
		// attributably, once per attempt to sustain any denial. Under the
		// PERMISSIONED operator set that residual is accepted: the triggering seat is
		// operator-authenticated (logged below), the action is liveness-only (never an
		// unsafe or divergent signature), and a misbehaving operator is
		// governance-removable. REVISIT before any move to a PERMISSIONLESS operator
		// set, where anonymous costless spam would warrant f+1 snapshot-corroboration
		// (or resync) instead.
		if e.markLostSync() {
			e.logger.Warnf(
				"roast transition exchange: seat %d entered lost-sync from an "+
					"unobserved-attempt bundle sent by seat %d (attempt context "+
					"hash %x); failing closed before next selection",
				e.member, msg.Sender, hash,
			)
		}
		return
	}
	e.verifyAndStore(bundle)
}

// markLostSync records that this seat received a transition for an attempt it
// never observed -- it fell behind the group's committed ROAST attempt chain.
// It returns true only on the first transition into lost-sync, so the caller can
// attribute the triggering bundle exactly once (the listener may keep receiving
// such bundles -- including a spammer's -- while lost-sync stays latched).
func (e *RoastTransitionExchange) markLostSync() bool {
	return e.lostSync.CompareAndSwap(false, true)
}

// ConsumeLostSync reports whether this seat fell behind the group's committed
// ROAST attempt chain (it received a transition bundle for an attempt it never
// observed) AND clears the marker, so each recorded lost-sync event is charged to
// exactly one attempt.
//
// Consuming rather than latching is what bounds the blast radius. The triggering
// bundle cannot be cryptographically verified here -- VerifyBundle needs a local
// observe handle this seat never created -- so any authenticated member can
// produce one. A session-wide latch would let a single unverifiable message end
// the whole signing session; consuming per attempt costs one attempt and forces an
// attacker to keep re-sending, attributably, to sustain a denial.
func (e *RoastTransitionExchange) ConsumeLostSync() bool {
	return e.lostSync.CompareAndSwap(true, false)
}

// BroadcastForcedSnapshot publishes this seat's proof-of-attendance snapshot for
// the attempt, recording it locally BEFORE the broadcast so the censorship check
// on the returned bundle is meaningful. A no-op when the seat has no observe
// binding for the attempt or signing fails.
//
// RFC-21 Phase 7.3 PR2b-2 step 2 (the blame bridge): the snapshot carries the REAL
// evidence the coarse receive loop captured for this attempt, if any. The coarse
// path stashes its recorder snapshot keyed by the same (RoastSessionID, member,
// attemptHash); this consumes it so the seat's broadcast -- and therefore the
// elected coordinator's aggregated bundle -- carries real rejects/overflows/
// conflicts and NextAttempt's f+1 tally can fire. When the stash is empty (the
// seat observed nothing) the snapshot is the empty proof-of-attendance one, which
// must still be broadcast so the seat is not silence-parked.
func (e *RoastTransitionExchange) BroadcastForcedSnapshot(
	attemptHash [attempt.MessageDigestLength]byte,
) {
	binding, ok := observedAttempt(e.roastSessionID, e.member, attemptHash)
	if !ok {
		return
	}
	// takePendingEvidence returns the zero Evidence + nil proofs on a miss, which
	// NewLocalEvidenceSnapshot renders as the empty proof-of-attendance snapshot --
	// still broadcast so the seat is not silence-parked. When present, the snapshot
	// carries the coarse path's evidence and/or the interactive path's
	// coordinator-equivocation proofs (RFC-21 Phase 7.3 PR2b-2 step 2b); the
	// constructor sorts + owns the proofs and the single signing happens below.
	evidence, proofs, _ := takePendingEvidence(e.roastSessionID, e.member, attemptHash)
	// RFC-21 Layer A / M4: fold in the permanent inbound drops the receive loop
	// recorded for this attempt. Merged here rather than stashed at the drop site
	// because stashPendingEvidence replaces the evidence field wholesale, which
	// would clobber the share-blame rejects written above.
	if overflows := e.sub.TakeOverflowEvidence(attemptHash); len(overflows) > 0 {
		if evidence.Overflows == nil {
			evidence.Overflows = make(map[group.MemberIndex]uint, len(overflows))
		}
		for sender, count := range overflows {
			evidence.Overflows[sender] += count
		}
	}
	snapshot := roast.NewLocalEvidenceSnapshot(e.member, attemptHash, evidence, proofs...)
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
