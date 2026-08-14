package signing

import (
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// submitSnapshotIfActive is invoked at end-of-collect to capture the receive
// loop's accumulated evidence for the ROAST blame pipeline. member is the local
// seat whose receive loop is submitting (request.MemberIndex). The path is fully
// member-aware (RFC-21 Phase 7.3 PR2b-2): a multi-seat operator's sibling seats
// each capture their own evidence against their own attempt binding, so they never
// mis-attribute or collide.
//
// RFC-21 Phase 7.3 PR2b-2 step 2 (the blame bridge): the captured evidence is
// STASHED keyed by the attempt's (RoastSessionID, member, attemptHash), NOT
// recorded against the drive handle. The drive handle is never aggregated -- the
// transition exchange is the sole bundle producer and aggregates the OBSERVE
// handle -- so a RecordEvidence here was a write-only dead end. Stashing instead
// lets the exchange's BroadcastForcedSnapshot build + sign ONE snapshot carrying
// this evidence, so the elected coordinator's AggregateBundle includes it and
// NextAttempt's f+1 accuser tally can finally fire.
//
// The function is a no-op when any of the following holds:
//
//   - no session-handle binding exists for (sessionID, member): the default build
//     (where currentAttemptHandleForCollect always returns ok=false), or a state
//     where the orchestration layer that calls SetCurrentAttemptHandleForSession
//     has not run;
//   - the recorder is nil / a NoOp (no events were captured);
//   - the captured evidence is empty across all three categories: the exchange
//     still broadcasts an empty proof-of-attendance snapshot for the attempt, so
//     skipping the stash here does not silence-park the seat.
//
// Capturing must never break the receive loop's primary signing behaviour, so the
// function returns silently on every skip condition.
func submitSnapshotIfActive(
	sessionID string,
	member group.MemberIndex,
	recorder attempt.EvidenceRecorder,
) {
	if recorder == nil {
		return
	}
	// The drive binding (set by BeginOrchestrationForSession) signals this seat is
	// driving a ROAST attempt and carries its AttemptContext. ctx.SessionID is the
	// STABLE RoastSessionID and ctx.Hash() the attempt hash -- the
	// namespace-independent coordinate the transition exchange keys its observe
	// binding (and this stash) by, so the broadcast resolves the same entry.
	_, ctx, ok := currentAttemptHandleForCollect(sessionID, member)
	if !ok {
		return
	}
	evidence := recorder.Snapshot()
	if len(evidence.Overflows) == 0 &&
		len(evidence.Rejects) == 0 &&
		len(evidence.Conflicts) == 0 {
		// Truly nothing observed worth carrying. The emptiness test MUST consider
		// all three categories, not just overflows: a validation-blamable Reject
		// (e.g. an attempt-context-hash mismatch) or a first-write-wins Conflict
		// populates Rejects/Conflicts WITHOUT any Overflow, and NextAttempt's
		// exclusion path consumes snapshot.Rejects (next_attempt.go). Dropping a
		// reject/conflict-only snapshot here would silently starve the blame
		// pipeline of exactly the validation evidence it needs.
		return
	}
	stashPendingEvidence(ctx.SessionID, member, ctx.Hash(), evidence)
}
