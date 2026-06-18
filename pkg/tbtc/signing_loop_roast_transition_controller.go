package tbtc

import "github.com/keep-network/keep-core/pkg/protocol/group"

// roastTransitionController owns the session-scoped ROAST transition machinery
// for one local signer across a signing's attempts (RFC-21 Phase 7.3 PR2b-1b).
//
// The signing retry loop holds one per signer. A nil controller -- the default
// build, or a deployment that does not drive ROAST retry -- makes the loop skip
// every transition step, so behaviour is the legacy retry shuffle.
//
// PR2b-1b lands in three commits:
//   - C1 (this commit) wires the observe step: every attempt, every local seat
//     (including ones it is excluded from) records a local handle binding so it
//     can later verify a transition bundle and run NextAttempt. INERT -- the
//     bindings are produced but not consumed yet.
//   - C2 adds the failed-attempt transition exchange (forced snapshots ->
//     coordinator aggregation -> bundle distribution).
//   - C3 adds member-level consumption + fail-closed selection (the activation).
type roastTransitionController interface {
	// BeginObservedAttempt records a local observe binding for the attempt so the
	// signer can later verify the attempt's transition bundle and compute the next
	// attempt's included set. Called for EVERY attempt, including ones this signer
	// is excluded from (an excluded seat may be reinstated by NextAttempt, so it
	// must track the transition too). Best-effort: a failure is logged, never
	// propagated to the signing flow.
	BeginObservedAttempt(
		roastAttemptNumber uint,
		includedMembersIndexes []group.MemberIndex,
		excludedMembersIndexes []group.MemberIndex,
		transientlyParkedMembersIndexes []group.MemberIndex,
	)
	// OnAttemptFailed signals that a committed attempt this seat participated in
	// failed, so the transition exchange should run: publish this seat's forced
	// proof-of-attendance snapshot and, on the elected coordinator, aggregate +
	// broadcast the transition bundle once the snapshot collection window
	// (derived from timeoutBlock) closes. Best-effort and non-blocking: the
	// aggregation runs off the retry-loop goroutine.
	OnAttemptFailed(attemptNumber uint, timeoutBlock uint64)
}
