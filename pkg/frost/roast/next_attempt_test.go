package roast

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// nextAttemptFixture builds a previous AttemptContext and an
// associated TransitionMessage for the NextAttempt-policy tests.
// Members 1..5 included; no excluded; no parking; threshold 3 (so
// ExclusionAccuserQuorum(5, 3) = 3). By default every member submits
// a snapshot with no evidence entries. The overflows/rejects/conflicts
// maps are keyed by observer, then by accused member.
type nextAttemptFixture struct {
	included          []group.MemberIndex
	excluded          []group.MemberIndex
	parked            []group.MemberIndex
	overflows         map[group.MemberIndex]map[group.MemberIndex]uint
	rejects           map[group.MemberIndex]map[group.MemberIndex]uint
	conflicts         map[group.MemberIndex]map[group.MemberIndex]uint
	bundleSenders     []group.MemberIndex // override default = included
	attemptNumber     uint32
	dkgGroupPublicKey []byte
	threshold         uint
	sessionID         string
	messageDigest     [attempt.MessageDigestLength]byte
}

func newNextAttemptFixture() *nextAttemptFixture {
	return &nextAttemptFixture{
		included:          []group.MemberIndex{1, 2, 3, 4, 5},
		excluded:          nil,
		parked:            nil,
		overflows:         map[group.MemberIndex]map[group.MemberIndex]uint{},
		rejects:           map[group.MemberIndex]map[group.MemberIndex]uint{},
		conflicts:         map[group.MemberIndex]map[group.MemberIndex]uint{},
		bundleSenders:     nil,
		attemptNumber:     0,
		dkgGroupPublicKey: []byte{0x01, 0x02, 0x03},
		threshold:         3,
		sessionID:         "session-next-attempt",
		messageDigest:     [attempt.MessageDigestLength]byte{0x42},
	}
}

func (f *nextAttemptFixture) prev(t *testing.T) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContextWithParking(
		f.sessionID,
		"key-group-next-attempt",
		f.dkgGroupPublicKey,
		f.messageDigest,
		f.attemptNumber,
		f.included,
		f.excluded,
		f.parked,
	)
	if err != nil {
		t.Fatalf("fixture prev: %v", err)
	}
	return ctx
}

func (f *nextAttemptFixture) bundle(t *testing.T) *TransitionMessage {
	t.Helper()
	prev := f.prev(t)
	prevHash := prev.Hash()
	senders := f.bundleSenders
	if senders == nil {
		senders = append([]group.MemberIndex{}, f.included...)
	}
	bundle := make([]LocalEvidenceSnapshot, 0, len(senders))
	for _, s := range senders {
		snap := LocalEvidenceSnapshot{
			SenderIDValue:      uint32(s),
			AttemptContextHash: append([]byte{}, prevHash[:]...),
		}
		if entries, ok := f.overflows[s]; ok {
			ov := make([]OverflowEntry, 0, len(entries))
			for sender, count := range entries {
				ov = append(ov, OverflowEntry{Sender: sender, Count: count})
			}
			snap.Overflows = sortedOverflowEntries(ov)
		}
		if entries, ok := f.rejects[s]; ok {
			rj := make([]RejectEntry, 0, len(entries))
			for sender, count := range entries {
				rj = append(rj, RejectEntry{
					Sender: sender,
					Reason: "validation_gate_rejected",
					Count:  count,
				})
			}
			sortRejectEntriesForTest(rj)
			snap.Rejects = rj
		}
		if entries, ok := f.conflicts[s]; ok {
			cf := make([]ConflictEntry, 0, len(entries))
			for sender, count := range entries {
				cf = append(cf, ConflictEntry{Sender: sender, Count: count})
			}
			sortConflictEntriesForTest(cf)
			snap.Conflicts = cf
		}
		bundle = append(bundle, snap)
	}
	return &TransitionMessage{
		AttemptContextHash: append([]byte{}, prevHash[:]...),
		CoordinatorIDValue: 1,
		Bundle:             bundle,
	}
}

func sortedOverflowEntries(in []OverflowEntry) []OverflowEntry {
	out := append([]OverflowEntry{}, in...)
	// insertion sort; small slices.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Sender < out[j-1].Sender; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func sortConflictEntriesForTest(entries []ConflictEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Sender < entries[j-1].Sender; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

func TestNextAttempt_NoEvidenceProducesIdenticalIncludedSet(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	bundle := f.bundle(t)
	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSlicesEqual(next.IncludedSet, prev.IncludedSet) {
		t.Fatalf(
			"included set changed unexpectedly: prev=%v next=%v",
			prev.IncludedSet, next.IncludedSet,
		)
	}
	if len(next.ExcludedSet) != 0 {
		t.Fatalf("excluded set should be empty, got %v", next.ExcludedSet)
	}
	if len(next.TransientlyParked) != 0 {
		t.Fatalf("parking set should be empty, got %v", next.TransientlyParked)
	}
	if next.AttemptNumber != prev.AttemptNumber+1 {
		t.Fatalf(
			"attempt number not incremented: got %d, want %d",
			next.AttemptNumber, prev.AttemptNumber+1,
		)
	}
}

func TestNextAttempt_EstablishedOverflowParksTransiently(t *testing.T) {
	f := newNextAttemptFixture()
	// Members 2..5 all report overflow against sender 3: 4 distinct
	// accusers >= quorum (3). Transport blame is unverifiable in
	// principle, so even an established overflow accusation parks
	// transiently and never permanently excludes.
	for observer := group.MemberIndex(2); observer <= 5; observer++ {
		f.overflows[observer] = map[group.MemberIndex]uint{3: 1}
	}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf("sender 3 should be parked; got included %v", next.IncludedSet)
	}
	if !memberSliceContains(next.TransientlyParked, 3) {
		t.Fatalf(
			"sender 3 should appear in parked set; got %v",
			next.TransientlyParked,
		)
	}
	if memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf(
			"overflow must never permanently exclude; got excluded %v",
			next.ExcludedSet,
		)
	}
}

func TestNextAttempt_EstablishedOverflowParkIsTransient(t *testing.T) {
	f := newNextAttemptFixture()
	// Attempt N: quorum overflow accusation against member 3 parks it.
	for observer := group.MemberIndex(2); observer <= 5; observer++ {
		f.overflows[observer] = map[group.MemberIndex]uint{3: 1}
	}
	attemptN1, err := computeNextAttempt(
		f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{},
	)
	if err != nil {
		t.Fatalf("N -> N+1: %v", err)
	}
	if !memberSliceContains(attemptN1.TransientlyParked, 3) {
		t.Fatalf("N+1 must park member 3; got %v", attemptN1.TransientlyParked)
	}

	// Attempt N+1: parked member 3 cannot submit; no new accusations.
	attemptN1Hash := attemptN1.Hash()
	bundleN1 := &TransitionMessage{
		AttemptContextHash: append([]byte{}, attemptN1Hash[:]...),
		CoordinatorIDValue: 1,
		Bundle: []LocalEvidenceSnapshot{
			{SenderIDValue: 1, AttemptContextHash: append([]byte{}, attemptN1Hash[:]...)},
			{SenderIDValue: 2, AttemptContextHash: append([]byte{}, attemptN1Hash[:]...)},
			{SenderIDValue: 4, AttemptContextHash: append([]byte{}, attemptN1Hash[:]...)},
			{SenderIDValue: 5, AttemptContextHash: append([]byte{}, attemptN1Hash[:]...)},
		},
	}
	attemptN2, err := computeNextAttempt(
		attemptN1, bundleN1, f.threshold, f.dkgGroupPublicKey, fakeVerifier{},
	)
	if err != nil {
		t.Fatalf("N+1 -> N+2: %v", err)
	}
	if !memberSliceContains(attemptN2.IncludedSet, 3) {
		t.Fatalf(
			"N+2 must reinstate overflow-parked member 3; got included %v",
			attemptN2.IncludedSet,
		)
	}
}

func TestNextAttempt_SubQuorumOverflowHasNoEffect(t *testing.T) {
	f := newNextAttemptFixture()
	// Two observers (< quorum 3) report overflow against sender 3,
	// with large claimed counts. Counts must not be summed into
	// blame: an accusation below the accuser quorum is ignored.
	f.overflows[2] = map[group.MemberIndex]uint{3: 100}
	f.overflows[4] = map[group.MemberIndex]uint{3: 100}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf("sender 3 should remain included; got %v", next.IncludedSet)
	}
	if memberSliceContains(next.TransientlyParked, 3) {
		t.Fatal("sub-quorum overflow must not park")
	}
	if memberSliceContains(next.ExcludedSet, 3) {
		t.Fatal("sub-quorum overflow must not exclude")
	}
}

func TestNextAttempt_SilentMemberIsParkedTransiently(t *testing.T) {
	f := newNextAttemptFixture()
	// Only members 1, 2, 4, 5 submit; member 3 is silent.
	f.bundleSenders = []group.MemberIndex{1, 2, 4, 5}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.IncludedSet, 3) {
		t.Fatal("silent sender 3 must not appear in next IncludedSet")
	}
	if !memberSliceContains(next.TransientlyParked, 3) {
		t.Fatalf("silent sender 3 must appear in next TransientlyParked; got %v", next.TransientlyParked)
	}
	if memberSliceContains(next.ExcludedSet, 3) {
		t.Fatal("silent sender 3 must not be permanently excluded")
	}
}

func TestNextAttempt_PreviouslyParkedAreReinstated(t *testing.T) {
	f := newNextAttemptFixture()
	// Previous attempt: members 1, 2, 4, 5 included; member 3 parked.
	f.included = []group.MemberIndex{1, 2, 4, 5}
	f.parked = []group.MemberIndex{3}
	// Bundle: only the included set submits (parked cannot).
	f.bundleSenders = []group.MemberIndex{1, 2, 4, 5}

	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf(
			"previously parked member 3 must be reinstated; got included %v",
			next.IncludedSet,
		)
	}
	if memberSliceContains(next.TransientlyParked, 3) {
		t.Fatal("member 3 must not be re-parked")
	}
	if memberSliceContains(next.ExcludedSet, 3) {
		t.Fatal("member 3 must not be excluded")
	}
}

func TestNextAttempt_ParkingIsStrictlyTransient_NoEscalation(t *testing.T) {
	// Demonstrate the full cycle: park, skip one attempt, reinstate.
	// Attempt N: member 3 is silent.
	// Attempt N+1: member 3 is parked, did not submit.
	// Attempt N+2: member 3 is reinstated.
	f := newNextAttemptFixture()
	f.bundleSenders = []group.MemberIndex{1, 2, 4, 5}
	prev := f.prev(t)
	bundle := f.bundle(t)
	attemptN1, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("N -> N+1: %v", err)
	}
	if !memberSliceContains(attemptN1.TransientlyParked, 3) {
		t.Fatalf("N+1 must park member 3; got %v", attemptN1.TransientlyParked)
	}
	if memberSliceContains(attemptN1.IncludedSet, 3) {
		t.Fatal("member 3 must not be in N+1 IncludedSet (parked this attempt)")
	}

	// Now compute attempt N+2 from a bundle where parked member 3
	// could not submit (legitimately), and members 1, 2, 4, 5 did
	// submit.
	attemptN1Hash := attemptN1.Hash()
	bundleN1 := &TransitionMessage{
		AttemptContextHash: append([]byte{}, attemptN1Hash[:]...),
		CoordinatorIDValue: 1,
		Bundle: []LocalEvidenceSnapshot{
			{SenderIDValue: 1, AttemptContextHash: append([]byte{}, attemptN1Hash[:]...)},
			{SenderIDValue: 2, AttemptContextHash: append([]byte{}, attemptN1Hash[:]...)},
			{SenderIDValue: 4, AttemptContextHash: append([]byte{}, attemptN1Hash[:]...)},
			{SenderIDValue: 5, AttemptContextHash: append([]byte{}, attemptN1Hash[:]...)},
		},
	}
	attemptN2, err := computeNextAttempt(attemptN1, bundleN1, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("N+1 -> N+2: %v", err)
	}
	if !memberSliceContains(attemptN2.IncludedSet, 3) {
		t.Fatalf(
			"N+2 must reinstate member 3; got included %v",
			attemptN2.IncludedSet,
		)
	}
	if memberSliceContains(attemptN2.TransientlyParked, 3) {
		t.Fatal("N+2 must not re-park member 3")
	}
	if memberSliceContains(attemptN2.ExcludedSet, 3) {
		t.Fatal("N+2 must not permanently exclude member 3")
	}
}

func TestNextAttempt_OriginalSignerSetPreservedAcrossTransitions(t *testing.T) {
	f := newNextAttemptFixture()
	f.bundleSenders = []group.MemberIndex{1, 2, 4, 5} // 3 silent
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	originalSize := len(f.included)
	nextSize := len(next.IncludedSet) + len(next.ExcludedSet) + len(next.TransientlyParked)
	if nextSize != originalSize {
		t.Fatalf(
			"original signer set size not preserved: %d vs %d",
			nextSize, originalSize,
		)
	}
}

func TestNextAttempt_PolicyIsDeterministic(t *testing.T) {
	f := newNextAttemptFixture()
	f.bundleSenders = []group.MemberIndex{1, 2, 4, 5}
	f.overflows[2] = map[group.MemberIndex]uint{1: 2}
	f.overflows[5] = map[group.MemberIndex]uint{1: 2}
	a, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("first compute: %v", err)
	}
	b, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("second compute: %v", err)
	}
	if a.Hash() != b.Hash() {
		t.Fatalf("same inputs produced different next-attempt hashes")
	}
}

func TestNextAttempt_InfeasibilityWhenPermanentExclusionsBelowThreshold(t *testing.T) {
	f := newNextAttemptFixture()
	f.threshold = 5 // n-of-n: the accuser quorum is 1, so one reject establishes.
	// Permanently exclude member 3 via an established reject accusation. Only 4
	// non-excluded members remain, below the threshold of 5, and a permanently
	// excluded member is never reinstated -- so the session is genuinely
	// infeasible.
	f.rejects[1] = map[group.MemberIndex]uint{3: 1}
	_, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if !errors.Is(err, ErrAttemptInfeasible) {
		t.Fatalf("expected ErrAttemptInfeasible, got %v", err)
	}
}

func TestNextAttempt_TransientSilenceBelowThresholdDoesNotPermanentlyFail(t *testing.T) {
	f := newNextAttemptFixture()
	f.threshold = 5 // Require all 5 members.
	// Silently lose 2 members -> the next attempt's included set drops to 3
	// (below threshold), but NO member is permanently excluded. The session
	// must NOT be declared infeasible: the silenced members are transiently
	// parked and a later attempt reinstates them.
	f.bundleSenders = []group.MemberIndex{1, 2, 3}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("transient silence must not permanently fail the session, got %v", err)
	}
	if !memberSliceContains(next.TransientlyParked, 4) ||
		!memberSliceContains(next.TransientlyParked, 5) {
		t.Fatalf(
			"silenced members 4 and 5 must be transiently parked; got parked %v",
			next.TransientlyParked,
		)
	}
	if len(next.ExcludedSet) != 0 {
		t.Fatalf(
			"transient silence must not permanently exclude; got excluded %v",
			next.ExcludedSet,
		)
	}
}

func TestNextAttempt_TransientSilenceRecoversAcrossTwoAttempts(t *testing.T) {
	f := newNextAttemptFixture()
	f.threshold = 5

	// Attempt N -> N+1: members 4 and 5 are silent. They are parked (not
	// excluded) and the next attempt's included set is {1,2,3} (sub-threshold),
	// but the session is not failed.
	f.bundleSenders = []group.MemberIndex{1, 2, 3}
	attemptN1, err := computeNextAttempt(
		f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{},
	)
	if err != nil {
		t.Fatalf("attempt N+1: transient silence must not fail, got %v", err)
	}
	if !memberSliceContains(attemptN1.TransientlyParked, 4) ||
		!memberSliceContains(attemptN1.TransientlyParked, 5) {
		t.Fatalf(
			"attempt N+1: members 4 and 5 must be parked; got %v",
			attemptN1.TransientlyParked,
		)
	}

	// Attempt N+1 -> N+2: the previously-parked members are reinstated, so the
	// included set returns to all five and the session recovers without
	// intervention. The bundle defaults to N+1's included set {1,2,3}, which all
	// respond.
	g := newNextAttemptFixture()
	g.threshold = 5
	g.included = attemptN1.IncludedSet
	g.excluded = attemptN1.ExcludedSet
	g.parked = attemptN1.TransientlyParked
	g.attemptNumber = attemptN1.AttemptNumber
	attemptN2, err := computeNextAttempt(
		g.prev(t), g.bundle(t), g.threshold, g.dkgGroupPublicKey, fakeVerifier{},
	)
	if err != nil {
		t.Fatalf("attempt N+2: %v", err)
	}
	for _, m := range []group.MemberIndex{1, 2, 3, 4, 5} {
		if !memberSliceContains(attemptN2.IncludedSet, m) {
			t.Fatalf(
				"attempt N+2 must reinstate all members; missing %d, got included %v",
				m, attemptN2.IncludedSet,
			)
		}
	}
	if len(attemptN2.TransientlyParked) != 0 {
		t.Fatalf(
			"attempt N+2 should have no parked members; got %v",
			attemptN2.TransientlyParked,
		)
	}
}

func TestNextAttempt_ThresholdZeroDisablesInfeasibilityCheck(t *testing.T) {
	f := newNextAttemptFixture()
	f.threshold = 0
	// All members silent; without the infeasibility check, the next
	// attempt has zero included members. This is documented as a
	// test seam, not a production state.
	f.bundleSenders = []group.MemberIndex{}
	// We need at least one entry in the bundle for TransitionMessage
	// to be valid. Add a no-op snapshot from member 1 even though
	// they're "silent" by the policy's view. The policy only looks
	// at bundle senders that intersect prev.IncludedSet, which all
	// of them do here. So instead let's leave member 1 in the
	// bundle alone and silent the rest.
	f.bundleSenders = []group.MemberIndex{1}
	// IncludedSet would become {1}; for threshold=0 that's still
	// permitted.
	_, err := computeNextAttempt(f.prev(t), f.bundle(t), 0, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("expected success with threshold=0, got %v", err)
	}
}

func TestNextAttempt_SingleObserverCountMagnitudeIsNotBlame(t *testing.T) {
	f := newNextAttemptFixture()
	// One observer fabricating arbitrarily large counts in every
	// category is still a single accuser, far below the quorum (3):
	// the accused member must be completely unaffected. This is the
	// counted-blame fabrication vector the quorum policy closes.
	f.overflows[5] = map[group.MemberIndex]uint{3: 1000}
	f.rejects[5] = map[group.MemberIndex]uint{3: 1000}
	f.conflicts[5] = map[group.MemberIndex]uint{3: 1000}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf("sender 3 must remain included; got %v", next.IncludedSet)
	}
	if memberSliceContains(next.TransientlyParked, 3) {
		t.Fatal("fabricated counts must not park")
	}
	if memberSliceContains(next.ExcludedSet, 3) {
		t.Fatal("fabricated counts must not exclude")
	}
}

func TestNextAttempt_QuorumBoundaryForRejectAndConflict(t *testing.T) {
	// n=5, t=3 => f = 2 byzantine tolerated => quorum = 3 accusers.
	for _, tc := range []struct {
		name     string
		category func(f *nextAttemptFixture) map[group.MemberIndex]map[group.MemberIndex]uint
	}{
		{
			name: "rejects",
			category: func(f *nextAttemptFixture) map[group.MemberIndex]map[group.MemberIndex]uint {
				return f.rejects
			},
		},
		{
			name: "conflicts",
			category: func(f *nextAttemptFixture) map[group.MemberIndex]map[group.MemberIndex]uint {
				return f.conflicts
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// f accusers (= 2, the max byzantine count): no action.
			fixtureAtF := newNextAttemptFixture()
			tc.category(fixtureAtF)[1] = map[group.MemberIndex]uint{3: 1}
			tc.category(fixtureAtF)[2] = map[group.MemberIndex]uint{3: 1}
			next, err := computeNextAttempt(
				fixtureAtF.prev(t),
				fixtureAtF.bundle(t),
				fixtureAtF.threshold,
				fixtureAtF.dkgGroupPublicKey,
				fakeVerifier{},
			)
			if err != nil {
				t.Fatalf("compute at f accusers: %v", err)
			}
			if !memberSliceContains(next.IncludedSet, 3) {
				t.Fatalf(
					"f accusers must not move sender 3; got included %v",
					next.IncludedSet,
				)
			}

			// f+1 accusers (= 3): permanent exclusion.
			fixtureAtQuorum := newNextAttemptFixture()
			tc.category(fixtureAtQuorum)[1] = map[group.MemberIndex]uint{3: 1}
			tc.category(fixtureAtQuorum)[2] = map[group.MemberIndex]uint{3: 1}
			tc.category(fixtureAtQuorum)[4] = map[group.MemberIndex]uint{3: 1}
			next, err = computeNextAttempt(
				fixtureAtQuorum.prev(t),
				fixtureAtQuorum.bundle(t),
				fixtureAtQuorum.threshold,
				fixtureAtQuorum.dkgGroupPublicKey,
				fakeVerifier{},
			)
			if err != nil {
				t.Fatalf("compute at quorum: %v", err)
			}
			if !memberSliceContains(next.ExcludedSet, 3) {
				t.Fatalf(
					"quorum accusers must exclude sender 3; got %v",
					next.ExcludedSet,
				)
			}
			if memberSliceContains(next.IncludedSet, 3) {
				t.Fatal("excluded sender 3 must not remain included")
			}
		})
	}
}

func TestNextAttempt_CrossCategoryAccusationsDoNotSum(t *testing.T) {
	f := newNextAttemptFixture()
	// Two reject accusers plus two different conflict accusers against
	// sender 3: each category is below the quorum (3) on its own.
	// Categories must be tallied independently -- four observers
	// spread across categories are not an established accusation.
	f.rejects[1] = map[group.MemberIndex]uint{3: 1}
	f.rejects[2] = map[group.MemberIndex]uint{3: 1}
	f.conflicts[4] = map[group.MemberIndex]uint{3: 1}
	f.conflicts[5] = map[group.MemberIndex]uint{3: 1}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf(
			"cross-category sub-quorum claims must not exclude; got %v",
			next.ExcludedSet,
		)
	}
	if !memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf("sender 3 must remain included; got %v", next.IncludedSet)
	}
}

func TestNextAttempt_FabricatedBlameCannotGrindHonestMembers(t *testing.T) {
	// Regression for the counted-blame grinding vector: a single
	// byzantine member fabricates maximal evidence against every
	// honest member on every attempt. Under the quorum policy the
	// honest members must never be excluded or parked, and the
	// session must never become infeasible.
	f := newNextAttemptFixture()
	byzantine := group.MemberIndex(5)
	honest := []group.MemberIndex{1, 2, 3, 4}

	prev := f.prev(t)
	for attemptIndex := 0; attemptIndex < 6; attemptIndex++ {
		accusations := map[group.MemberIndex]uint{}
		for _, target := range honest {
			accusations[target] = 1000
		}
		prevHash := prev.Hash()
		bundle := make([]LocalEvidenceSnapshot, 0, len(prev.IncludedSet))
		for _, sender := range prev.IncludedSet {
			snap := LocalEvidenceSnapshot{
				SenderIDValue:      uint32(sender),
				AttemptContextHash: append([]byte{}, prevHash[:]...),
			}
			if sender == byzantine {
				overflows := make([]OverflowEntry, 0, len(honest))
				rejects := make([]RejectEntry, 0, len(honest))
				conflicts := make([]ConflictEntry, 0, len(honest))
				for _, target := range honest {
					overflows = append(overflows, OverflowEntry{
						Sender: target, Count: accusations[target],
					})
					rejects = append(rejects, RejectEntry{
						Sender: target,
						Reason: "validation_gate_rejected",
						Count:  accusations[target],
					})
					conflicts = append(conflicts, ConflictEntry{
						Sender: target, Count: accusations[target],
					})
				}
				snap.Overflows = overflows
				snap.Rejects = rejects
				snap.Conflicts = conflicts
			}
			bundle = append(bundle, snap)
		}
		msg := &TransitionMessage{
			AttemptContextHash: append([]byte{}, prevHash[:]...),
			CoordinatorIDValue: 1,
			Bundle:             bundle,
		}

		next, err := computeNextAttempt(prev, msg, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
		if err != nil {
			t.Fatalf("attempt %d: %v", attemptIndex, err)
		}
		for _, member := range honest {
			if !memberSliceContains(next.IncludedSet, member) {
				t.Fatalf(
					"attempt %d: honest member %d ground out; included %v, excluded %v, parked %v",
					attemptIndex,
					member,
					next.IncludedSet,
					next.ExcludedSet,
					next.TransientlyParked,
				)
			}
		}
		prev = next
	}
}

func TestNextAttempt_NonCredibleAccusersAreIgnored(t *testing.T) {
	f := newNextAttemptFixture()
	// Members 1..5 included; member 6 is outside the original signer
	// set and member 7 likewise. Their snapshots appear in the bundle
	// (a byzantine coordinator could aggregate anything), each
	// accusing sender 3 of conflicts, alongside two credible
	// accusers. Total claims = 4, but credible accusers = 2 < quorum:
	// no action.
	f.bundleSenders = []group.MemberIndex{1, 2, 3, 4, 5, 6, 7}
	f.conflicts[1] = map[group.MemberIndex]uint{3: 1}
	f.conflicts[2] = map[group.MemberIndex]uint{3: 1}
	f.conflicts[6] = map[group.MemberIndex]uint{3: 1}
	f.conflicts[7] = map[group.MemberIndex]uint{3: 1}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf(
			"non-credible accusers must not contribute to exclusion; got %v",
			next.ExcludedSet,
		)
	}
	if !memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf("sender 3 must remain included; got %v", next.IncludedSet)
	}
}

func TestNextAttempt_AccusationsAgainstNonOriginalMembersIgnored(t *testing.T) {
	f := newNextAttemptFixture()
	// Quorum-many accusers blame member 9, which is not part of the
	// original signer set. The policy must not act on it: the next
	// context's excluded/parked sets stay within the original set.
	f.conflicts[1] = map[group.MemberIndex]uint{9: 1}
	f.conflicts[2] = map[group.MemberIndex]uint{9: 1}
	f.conflicts[4] = map[group.MemberIndex]uint{9: 1}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.ExcludedSet, 9) {
		t.Fatalf(
			"non-original member must not enter excluded set; got %v",
			next.ExcludedSet,
		)
	}
	if memberSliceContains(next.TransientlyParked, 9) {
		t.Fatalf(
			"non-original member must not enter parked set; got %v",
			next.TransientlyParked,
		)
	}
}

func TestNextAttempt_NilBundleRejected(t *testing.T) {
	c := newSignedCoordinatorForMember(0)
	handle, _ := c.BeginAttempt(newTestContext(t))
	_, err := c.NextAttempt(handle, nil, 3, []byte{0x01})
	if err == nil {
		t.Fatal("expected error for nil bundle")
	}
}

func TestNextAttempt_UnknownHandleRejected(t *testing.T) {
	c := newSignedCoordinatorForMember(0)
	bogus := AttemptHandle{id: 999}
	_, err := c.NextAttempt(bogus, &TransitionMessage{}, 3, []byte{0x01})
	if !errors.Is(err, ErrUnknownAttempt) {
		t.Fatalf("expected ErrUnknownAttempt, got %v", err)
	}
}

func TestExclusionAccuserQuorum_MatchesRFC(t *testing.T) {
	// Production shape: n=100, t=51 => f = 49 => quorum 50. The 49
	// worst-case byzantine members can never fabricate a quorum; the
	// 51 honest members establish any fault they all observe.
	if got := ExclusionAccuserQuorum(100, 51); got != 50 {
		t.Fatalf("quorum(100, 51) = %d, want 50", got)
	}
	// Unit-test shape used by the fixtures in this file.
	if got := ExclusionAccuserQuorum(5, 3); got != 3 {
		t.Fatalf("quorum(5, 3) = %d, want 3", got)
	}
	// Zero threshold (policy-only test seam): quorum is deliberately
	// unreachable so no accusation-driven action can occur.
	if got := ExclusionAccuserQuorum(5, 0); got != 6 {
		t.Fatalf("quorum(5, 0) = %d, want unreachable 6", got)
	}
	// Degenerate threshold larger than the group: also unreachable.
	if got := ExclusionAccuserQuorum(3, 5); got != 4 {
		t.Fatalf("quorum(3, 5) = %d, want unreachable 4", got)
	}
}

func memberSliceContains(slice []group.MemberIndex, target group.MemberIndex) bool {
	for _, m := range slice {
		if m == target {
			return true
		}
	}
	return false
}

func memberSlicesEqual(a, b []group.MemberIndex) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
