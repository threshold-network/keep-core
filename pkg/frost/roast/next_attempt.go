package roast

import (
	"errors"
	"fmt"
	"sort"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// OverflowExclusionThreshold is the per-sender overflow-count
// threshold above which the NextAttempt policy permanently excludes
// the sender (transport-blamable). Matches the constant documented in
// RFC-21 Layer B.
const OverflowExclusionThreshold uint = 4

// ErrAttemptInfeasible is returned by NextAttempt when the next
// attempt's IncludedSet would drop below the signing threshold t and
// the session can no longer make progress with the original signer
// set. Callers must surface this to the application layer: the
// session is permanently failed.
var ErrAttemptInfeasible = errors.New(
	"coordinator: next attempt is infeasible -- included set below threshold",
)

// NextAttempt computes the deterministic next attempt context from a
// verified TransitionMessage. It is a pure function of
// (previous AttemptContext, bundle, threshold): two honest signers
// fed the same inputs produce byte-identical outputs, so the
// signing-group state machine remains in agreement across the
// network.
//
// Callers MUST call VerifyBundle on the message before passing it to
// NextAttempt. NextAttempt does not re-run the signature checks; it
// assumes the bundle is verified and only applies the policy.
//
// The policy (RFC-21 Layer B):
//
//  1. Permanent exclusion (transport-blamable): a sender whose total
//     overflow count across the bundle is at least
//     OverflowExclusionThreshold is added to ExcludedSet forever.
//
//  2. Permanent exclusion (validation-blamable): senders with
//     confirmed non-transport reject events. Phase 3.4 does not yet
//     track reject events, so this is a no-op; the hook is in place
//     for a later phase.
//
//  3. Silence parking (strictly transient): a sender in the
//     previous attempt's IncludedSet that does not appear in the
//     bundle, and is not permanently excluded, is added to the next
//     attempt's TransientlyParked set. The attempt after that
//     automatically reinstates them, so a falsely-silenced honest
//     peer recovers without intervention.
//
//  4. Reinstatement: members in the previous attempt's
//     TransientlyParked set automatically rejoin the next attempt's
//     IncludedSet (unless they are now permanently excluded for
//     another reason).
//
//  5. Infeasibility: if the next attempt's IncludedSet would have
//     fewer than threshold members, return ErrAttemptInfeasible.
//
// threshold is the FROST signing threshold t for the key group; it
// is constant across attempts within a session. A threshold of zero
// disables the infeasibility check (useful in tests that exercise
// the policy independently from threshold semantics).
//
// The caller is responsible for supplying the DKG group public key
// from the same source the previous attempt used (the FFI signer
// material, per RFC-21 Decision 2); a different source would
// silently desynchronise the seed derivation.
func (c *inMemoryCoordinator) NextAttempt(
	handle AttemptHandle,
	bundle *TransitionMessage,
	threshold uint,
	dkgGroupPublicKey []byte,
) (attempt.AttemptContext, error) {
	if bundle == nil {
		return attempt.AttemptContext{}, errors.New(
			"coordinator: cannot compute next attempt from nil bundle",
		)
	}
	c.mu.Lock()
	record, ok := c.attempts[handle.id]
	if !ok {
		c.mu.Unlock()
		return attempt.AttemptContext{}, ErrUnknownAttempt
	}
	prev := record.context
	c.mu.Unlock()

	return computeNextAttempt(prev, bundle, threshold, dkgGroupPublicKey)
}

// computeNextAttempt is the pure-function policy core: it takes the
// previous AttemptContext, a verified bundle, and the signing
// threshold, and returns the next AttemptContext. Factored out from
// NextAttempt so the policy is independently unit-testable without a
// Coordinator instance.
func computeNextAttempt(
	prev attempt.AttemptContext,
	bundle *TransitionMessage,
	threshold uint,
	dkgGroupPublicKey []byte,
) (attempt.AttemptContext, error) {
	// (1) Permanent exclusion from overflow evidence.
	overflowBlamed := overflowBlamedSenders(bundle, OverflowExclusionThreshold)

	// (2) Reject blame -- Phase 3.4 has no reject category to read.
	// rejectBlamed := <future>

	// Merge into permanent exclusion.
	exclSet := newMemberSet()
	exclSet.addAll(prev.ExcludedSet)
	exclSet.addAll(overflowBlamed)

	// (3) Silence parking: senders in prev.IncludedSet but not in
	// bundle, that we are not now permanently excluding.
	bundleSenders := bundleSenderSet(bundle)
	parkSet := newMemberSet()
	for _, m := range prev.IncludedSet {
		if bundleSenders.contains(m) {
			continue
		}
		if exclSet.contains(m) {
			continue
		}
		parkSet.add(m)
	}

	// (4) Original signer set persists across transitions as
	//     IncludedSet ∪ ExcludedSet ∪ TransientlyParked. Reinstate
	//     previously parked members by re-including them
	//     (unless newly permanently excluded -- which they cannot be,
	//     since they could not have submitted overflow evidence
	//     this attempt).
	original := newMemberSet()
	original.addAll(prev.IncludedSet)
	original.addAll(prev.ExcludedSet)
	original.addAll(prev.TransientlyParked)

	included := original.sorted()
	included = filterOut(included, exclSet)
	included = filterOut(included, parkSet)

	// (5) Infeasibility check.
	if threshold > 0 && uint(len(included)) < threshold {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: %d eligible, threshold %d",
			ErrAttemptInfeasible,
			len(included),
			threshold,
		)
	}

	// Convert ExcludedSet to its canonical (sorted, deduped) slice.
	nextExcluded := exclSet.sorted()
	nextParked := parkSet.sorted()

	next, err := attempt.NewAttemptContextWithParking(
		prev.SessionID,
		prev.KeyGroupID,
		dkgGroupPublicKey,
		prev.MessageDigest,
		prev.AttemptNumber+1,
		included,
		nextExcluded,
		nextParked,
	)
	if err != nil {
		return attempt.AttemptContext{}, fmt.Errorf(
			"coordinator: next attempt construction: %w",
			err,
		)
	}
	return next, nil
}

// overflowBlamedSenders returns the senders whose total overflow
// count across every snapshot in the bundle is at least the
// supplied threshold. Counts are summed (not averaged) so a sender
// hitting the threshold from one observer alone is sufficient.
func overflowBlamedSenders(
	bundle *TransitionMessage,
	threshold uint,
) []group.MemberIndex {
	counts := map[group.MemberIndex]uint{}
	for i := range bundle.Bundle {
		for _, entry := range bundle.Bundle[i].Overflows {
			counts[entry.Sender] += entry.Count
		}
	}
	out := make([]group.MemberIndex, 0)
	for sender, count := range counts {
		if count >= threshold {
			out = append(out, sender)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// bundleSenderSet returns the set of senders that submitted a
// snapshot to the bundle.
func bundleSenderSet(bundle *TransitionMessage) *memberSet {
	out := newMemberSet()
	for i := range bundle.Bundle {
		out.add(bundle.Bundle[i].SenderID())
	}
	return out
}

// memberSet is a small helper for set arithmetic over
// group.MemberIndex. Sufficient for the small (≤256) sizes the
// coordinator deals with.
type memberSet struct {
	m map[group.MemberIndex]struct{}
}

func newMemberSet() *memberSet {
	return &memberSet{m: map[group.MemberIndex]struct{}{}}
}

func (s *memberSet) add(member group.MemberIndex) { s.m[member] = struct{}{} }
func (s *memberSet) contains(m group.MemberIndex) bool {
	_, ok := s.m[m]
	return ok
}

func (s *memberSet) addAll(members []group.MemberIndex) {
	for _, m := range members {
		s.add(m)
	}
}

func (s *memberSet) sorted() []group.MemberIndex {
	out := make([]group.MemberIndex, 0, len(s.m))
	for m := range s.m {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// filterOut returns members not in the excluded set, preserving
// input order.
func filterOut(
	members []group.MemberIndex,
	excluded *memberSet,
) []group.MemberIndex {
	out := make([]group.MemberIndex, 0, len(members))
	for _, m := range members {
		if !excluded.contains(m) {
			out = append(out, m)
		}
	}
	return out
}
