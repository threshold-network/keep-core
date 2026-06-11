package roast

import (
	"errors"
	"fmt"
	"sort"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// ExclusionAccuserQuorum returns the number of distinct accusing
// observers required before the NextAttempt policy treats an evidence
// category as group-established against an accused member.
//
// The bundle's evidence entries are observer-signed *claims*, not
// self-incriminating proofs: nothing in an OverflowEntry, RejectEntry,
// or ConflictEntry lets a third party re-check that the accused
// actually misbehaved. A policy that permanently excludes on
// unverifiable claims inverts ROAST's robustness guarantee -- a single
// byzantine observer could fabricate evidence against honest members
// and grind the included set toward ErrAttemptInfeasible.
//
// Under the protocol's own trust assumption (at least threshold of the
// original groupSize members honest), at most
// f = groupSize - threshold members are byzantine. An accusation
// corroborated by at least f+1 distinct observers therefore contains
// at least one honest accuser and can be acted on as established.
// Below the quorum the policy takes no action on the accusation.
//
// A real fault is observed by every honest member processing the
// broadcast (contributions are broadcast, not point-to-point), so
// established faults reach the quorum naturally: with the production
// shape (n=100, t=51) a real fault gathers up to 51 honest accusers
// against a quorum of 50, while the 49 worst-case byzantine members
// can never reach 50 by fabrication.
//
// A zero threshold (used by policy-only tests) or a threshold larger
// than the group yields groupSize+1 -- deliberately unreachable, so no
// accusation-driven action can occur without a real threshold.
func ExclusionAccuserQuorum(groupSize, threshold uint) uint {
	if threshold == 0 || threshold > groupSize {
		return groupSize + 1
	}
	return groupSize - threshold + 1
}

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
// The policy (RFC-21 Layer B, verifiable-blame revision):
//
//  1. Accusation tallying: each bundle snapshot is one observer's
//     claim set. Only observers in the previous attempt's IncludedSet
//     are credible (they are the members that participated), each
//     observer counts at most once per accused member per category
//     regardless of the claimed Count, and only accusations against
//     members of the original signer set are tallied. An accusation
//     is *established* when at least
//     ExclusionAccuserQuorum(originalGroupSize, threshold) distinct
//     observers make it; below the quorum it is ignored, because a
//     bare counter is not verifiable evidence (see
//     ExclusionAccuserQuorum).
//
//  2. Permanent exclusion (validation- or equivocation-blamable):
//     members with an established reject accusation or an established
//     conflict accusation are added to ExcludedSet forever. The
//     categories are tallied independently -- reject and conflict
//     claims against the same member do not sum.
//
//  3. Transient parking (transport-blamable): members with an
//     established overflow accusation are added to the next attempt's
//     TransientlyParked set. Transport pressure is observable only at
//     the transport layer and can never be made self-incriminating,
//     so overflow can park -- cost one attempt of liveness -- but
//     never permanently exclude.
//
//  4. Silence parking (strictly transient): a member in the previous
//     attempt's IncludedSet that does not appear in the bundle, and
//     is not permanently excluded, is added to the next attempt's
//     TransientlyParked set. The attempt after that automatically
//     reinstates them, so a falsely-silenced honest peer recovers
//     without intervention.
//
//  5. Reinstatement: members in the previous attempt's
//     TransientlyParked set automatically rejoin the next attempt's
//     IncludedSet (unless they are now permanently excluded).
//
//  6. Infeasibility: if the next attempt's IncludedSet would have
//     fewer than threshold members, return ErrAttemptInfeasible.
//
// Verifiability roadmap: permanent exclusion on a *single* piece of
// evidence becomes sound once the wire format carries
// self-incriminating proof -- for conflicts, the accused's own two
// operator-signed payloads with identical (attempt, sender) and
// different bytes; for rejects, the accused's operator-signed
// contribution plus a deterministic validation failure any member can
// re-run. When those land, the per-category quorum gate can be
// relaxed to proof-verified entries. Until then the quorum is the
// hard gate that keeps fabricated blame from compounding.
//
// threshold is the FROST signing threshold t for the key group; it
// is constant across attempts within a session. A threshold of zero
// disables the infeasibility check and (via ExclusionAccuserQuorum)
// all accusation-driven action -- useful in tests that exercise the
// silence/parking mechanics independently from threshold semantics.
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
	// Original signer set persists across transitions as
	// IncludedSet ∪ ExcludedSet ∪ TransientlyParked. Its size (not
	// the shrinking IncludedSet) anchors the accuser quorum so the
	// bar cannot be lowered by excluding members first.
	original := newMemberSet()
	original.addAll(prev.IncludedSet)
	original.addAll(prev.ExcludedSet)
	original.addAll(prev.TransientlyParked)

	quorum := ExclusionAccuserQuorum(uint(original.size()), threshold)

	credibleObservers := newMemberSet()
	credibleObservers.addAll(prev.IncludedSet)

	// (1)+(2) Established reject/conflict accusations: permanent.
	rejectBlamed := establishedAccused(
		bundle, credibleObservers, original, quorum, snapshotRejectAccusations,
	)
	conflictBlamed := establishedAccused(
		bundle, credibleObservers, original, quorum, snapshotConflictAccusations,
	)

	// (1)+(3) Established overflow accusations: transient parking only.
	overflowParked := establishedAccused(
		bundle, credibleObservers, original, quorum, snapshotOverflowAccusations,
	)

	// Merge into permanent exclusion.
	exclSet := newMemberSet()
	exclSet.addAll(prev.ExcludedSet)
	exclSet.addAll(rejectBlamed)
	exclSet.addAll(conflictBlamed)

	// (3)+(4) Parking: established overflow accusations plus senders
	// in prev.IncludedSet missing from the bundle -- in both cases
	// only when not permanently excluded (exclusion wins).
	bundleSenders := bundleSenderSet(bundle)
	parkSet := newMemberSet()
	for _, m := range overflowParked {
		if exclSet.contains(m) {
			continue
		}
		parkSet.add(m)
	}
	for _, m := range prev.IncludedSet {
		if bundleSenders.contains(m) {
			continue
		}
		if exclSet.contains(m) {
			continue
		}
		parkSet.add(m)
	}

	// (5) Reinstate previously parked members by re-including them
	// (unless newly permanently excluded or re-parked).
	included := original.sorted()
	included = filterOut(included, exclSet)
	included = filterOut(included, parkSet)

	// (6) Infeasibility check.
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

// snapshotAccusations extracts the members one observer's snapshot
// accuses in a single evidence category. Implementations must return
// each accused member at most once per snapshot.
type snapshotAccusations func(snapshot *LocalEvidenceSnapshot) []group.MemberIndex

// snapshotOverflowAccusations returns the members the snapshot accuses
// of transport overflow. The claimed Count magnitude is intentionally
// ignored beyond existence: an observer is one accuser regardless of
// how large a counter it reports.
func snapshotOverflowAccusations(
	snapshot *LocalEvidenceSnapshot,
) []group.MemberIndex {
	out := make([]group.MemberIndex, 0, len(snapshot.Overflows))
	for _, entry := range snapshot.Overflows {
		if entry.Count == 0 {
			continue
		}
		out = append(out, entry.Sender)
	}
	return out
}

// snapshotRejectAccusations returns the members the snapshot accuses
// of validation rejects, deduplicated across reasons: one observer
// reporting three reject reasons against the same member is still a
// single accuser for that member.
func snapshotRejectAccusations(
	snapshot *LocalEvidenceSnapshot,
) []group.MemberIndex {
	seen := newMemberSet()
	out := make([]group.MemberIndex, 0, len(snapshot.Rejects))
	for _, entry := range snapshot.Rejects {
		if entry.Count == 0 {
			continue
		}
		if seen.contains(entry.Sender) {
			continue
		}
		seen.add(entry.Sender)
		out = append(out, entry.Sender)
	}
	return out
}

// snapshotConflictAccusations returns the members the snapshot accuses
// of first-write-wins conflicts.
func snapshotConflictAccusations(
	snapshot *LocalEvidenceSnapshot,
) []group.MemberIndex {
	out := make([]group.MemberIndex, 0, len(snapshot.Conflicts))
	for _, entry := range snapshot.Conflicts {
		if entry.Count == 0 {
			continue
		}
		out = append(out, entry.Sender)
	}
	return out
}

// establishedAccused tallies one evidence category across the bundle
// by *distinct credible accuser* and returns the deterministically
// sorted accused members whose accuser count meets the quorum.
//
// Bundle validity (one snapshot per sender, ascending) is enforced by
// TransitionMessage.Validate; the memberSet additionally makes
// re-counting harmless.
func establishedAccused(
	bundle *TransitionMessage,
	credibleObservers *memberSet,
	original *memberSet,
	quorum uint,
	accusations snapshotAccusations,
) []group.MemberIndex {
	accusersByAccused := map[group.MemberIndex]*memberSet{}
	for i := range bundle.Bundle {
		snapshot := &bundle.Bundle[i]
		observer := snapshot.SenderID()
		if !credibleObservers.contains(observer) {
			continue
		}
		for _, accused := range accusations(snapshot) {
			if !original.contains(accused) {
				continue
			}
			accusers, ok := accusersByAccused[accused]
			if !ok {
				accusers = newMemberSet()
				accusersByAccused[accused] = accusers
			}
			accusers.add(observer)
		}
	}

	counts := map[group.MemberIndex]uint{}
	for accused, accusers := range accusersByAccused {
		counts[accused] = uint(accusers.size())
	}
	return blamedSenders(counts, quorum)
}

// blamedSenders extracts the deterministically-sorted list of
// senders whose accumulated count meets the threshold. Factored
// out so the category helpers share the same canonicalisation.
func blamedSenders(
	counts map[group.MemberIndex]uint,
	threshold uint,
) []group.MemberIndex {
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

func (s *memberSet) size() int { return len(s.m) }

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
