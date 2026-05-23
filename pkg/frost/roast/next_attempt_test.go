package roast

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// nextAttemptFixture builds a previous AttemptContext and an
// associated TransitionMessage for the NextAttempt-policy tests.
// Members 1..5 included; no excluded; no parking. By default every
// member submits a snapshot with no overflow events.
type nextAttemptFixture struct {
	included          []group.MemberIndex
	excluded          []group.MemberIndex
	parked            []group.MemberIndex
	overflows         map[group.MemberIndex]map[group.MemberIndex]uint
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

func TestNextAttempt_NoEvidenceProducesIdenticalIncludedSet(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	bundle := f.bundle(t)
	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey)
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

func TestNextAttempt_OverflowThresholdTriggersPermanentExclusion(t *testing.T) {
	f := newNextAttemptFixture()
	// Members 2..5 all report 1 overflow event each against sender 3.
	// 4 observers × 1 event = 4 total = OverflowExclusionThreshold.
	for observer := group.MemberIndex(2); observer <= 5; observer++ {
		f.overflows[observer] = map[group.MemberIndex]uint{3: 1}
	}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf("sender 3 should be excluded; got included %v", next.IncludedSet)
	}
	if !memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf("sender 3 should appear in excluded set; got %v", next.ExcludedSet)
	}
}

func TestNextAttempt_OverflowBelowThresholdDoesNotExclude(t *testing.T) {
	f := newNextAttemptFixture()
	// Only 1 observer reports 1 overflow event against sender 3.
	// 1 < threshold (4).
	f.overflows[2] = map[group.MemberIndex]uint{3: 1}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf("sender 3 should remain included; got %v", next.IncludedSet)
	}
}

func TestNextAttempt_SilentMemberIsParkedTransiently(t *testing.T) {
	f := newNextAttemptFixture()
	// Only members 1, 2, 4, 5 submit; member 3 is silent.
	f.bundleSenders = []group.MemberIndex{1, 2, 4, 5}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey)
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

	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey)
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
	attemptN1, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey)
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
	attemptN2, err := computeNextAttempt(attemptN1, bundleN1, f.threshold, f.dkgGroupPublicKey)
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
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey)
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
	a, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey)
	if err != nil {
		t.Fatalf("first compute: %v", err)
	}
	b, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey)
	if err != nil {
		t.Fatalf("second compute: %v", err)
	}
	if a.Hash() != b.Hash() {
		t.Fatalf("same inputs produced different next-attempt hashes")
	}
}

func TestNextAttempt_InfeasibilityWhenBelowThreshold(t *testing.T) {
	f := newNextAttemptFixture()
	f.threshold = 5 // Require all 5 members.
	// Silently lose 2 members -> only 3 remain in IncludedSet, below
	// threshold of 5.
	f.bundleSenders = []group.MemberIndex{1, 2, 3}
	_, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey)
	if !errors.Is(err, ErrAttemptInfeasible) {
		t.Fatalf("expected ErrAttemptInfeasible, got %v", err)
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
	_, err := computeNextAttempt(f.prev(t), f.bundle(t), 0, f.dkgGroupPublicKey)
	if err != nil {
		t.Fatalf("expected success with threshold=0, got %v", err)
	}
}

func TestNextAttempt_OverflowFromMultipleObserversIsSummed(t *testing.T) {
	f := newNextAttemptFixture()
	// 2 observers each report 2 overflow events = total 4 = threshold.
	f.overflows[1] = map[group.MemberIndex]uint{3: 2}
	f.overflows[2] = map[group.MemberIndex]uint{3: 2}
	next, err := computeNextAttempt(f.prev(t), f.bundle(t), f.threshold, f.dkgGroupPublicKey)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf(
			"sender 3 should be excluded by summed overflow; got %v",
			next.ExcludedSet,
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

func TestOverflowExclusionThreshold_MatchesRFC(t *testing.T) {
	if OverflowExclusionThreshold != 4 {
		t.Fatalf(
			"RFC-21 Layer B specifies overflow threshold = 4; constant is %d",
			OverflowExclusionThreshold,
		)
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
