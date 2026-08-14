package roast

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// buildBundleWithCategories constructs a TransitionMessage where
// every member of prev.IncludedSet acts as an observer contributing
// the same per-(category, sender) evidence -- one reject reason and
// one conflict per "blamed" sender per observer. With the full
// included set accusing, every accusation is established (accusers =
// n >= quorum), so these bundles exercise the established-blame
// paths.
func buildBundleWithCategories(
	t *testing.T,
	prev attempt.AttemptContext,
	rejects map[group.MemberIndex][]string,
	conflicts []group.MemberIndex,
) *TransitionMessage {
	t.Helper()
	return buildBundleWithCategoriesFromAccusers(
		t, prev, prev.IncludedSet, rejects, conflicts,
	)
}

// buildBundleWithCategoriesFromAccusers is buildBundleWithCategories
// with an explicit accuser subset: every member of prev.IncludedSet
// submits a snapshot, but only the listed accusers carry the
// reject/conflict evidence. Lets tests pin the accuser-quorum
// boundary precisely.
func buildBundleWithCategoriesFromAccusers(
	t *testing.T,
	prev attempt.AttemptContext,
	accusers []group.MemberIndex,
	rejects map[group.MemberIndex][]string,
	conflicts []group.MemberIndex,
) *TransitionMessage {
	t.Helper()
	accuserSet := newMemberSet()
	accuserSet.addAll(accusers)
	prevHash := prev.Hash()
	bundle := make([]LocalEvidenceSnapshot, 0, len(prev.IncludedSet))
	for _, sender := range prev.IncludedSet {
		if !accuserSet.contains(sender) {
			bundle = append(bundle, LocalEvidenceSnapshot{
				SenderIDValue:      uint32(sender),
				AttemptContextHash: append([]byte{}, prevHash[:]...),
			})
			continue
		}
		snap := LocalEvidenceSnapshot{
			SenderIDValue:      uint32(sender),
			AttemptContextHash: append([]byte{}, prevHash[:]...),
		}
		var rejectEntries []RejectEntry
		for blamedSender, reasons := range rejects {
			for _, r := range reasons {
				rejectEntries = append(rejectEntries, RejectEntry{
					Sender: blamedSender,
					Reason: r,
					Count:  1,
				})
			}
		}
		sortRejectEntriesForTest(rejectEntries)
		if len(rejectEntries) > 0 {
			snap.Rejects = rejectEntries
		}
		var conflictEntries []ConflictEntry
		for _, blamedSender := range conflicts {
			conflictEntries = append(conflictEntries, ConflictEntry{
				Sender: blamedSender,
				Count:  1,
			})
		}
		if len(conflictEntries) > 0 {
			snap.Conflicts = conflictEntries
		}
		bundle = append(bundle, snap)
	}
	return &TransitionMessage{
		AttemptContextHash: append([]byte{}, prevHash[:]...),
		CoordinatorIDValue: 1,
		Bundle:             bundle,
	}
}

func sortRejectEntriesForTest(entries []RejectEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && (entries[j].Sender < entries[j-1].Sender ||
			(entries[j].Sender == entries[j-1].Sender && entries[j].Reason < entries[j-1].Reason)); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

func TestNextAttempt_EstablishedRejectExcludesPermanently(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	// Every observer reports one reject against sender 3: five
	// distinct accusers >= quorum (3), so the accusation is
	// established and the exclusion is permanent.
	bundle := buildBundleWithCategories(
		t,
		prev,
		map[group.MemberIndex][]string{3: {"validation_gate_rejected"}},
		nil,
	)

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf("sender 3 must be excluded; got %v", next.ExcludedSet)
	}
	if memberSliceContains(next.IncludedSet, 3) {
		t.Fatal("sender 3 must not be in next IncludedSet")
	}
}

func TestNextAttempt_EstablishedConflictExcludesPermanently(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	bundle := buildBundleWithCategories(
		t,
		prev,
		nil,
		[]group.MemberIndex{3},
	)

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf(
			"sender 3 must be excluded on an established conflict; got %v",
			next.ExcludedSet,
		)
	}
}

func TestNextAttempt_SubQuorumRejectAndConflictDoNotExclude(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	// Only two accusers (= f, the tolerated byzantine count) report
	// the reject and the conflict: below the quorum (3), so no
	// action -- the accused stays included and is not parked.
	bundle := buildBundleWithCategoriesFromAccusers(
		t,
		prev,
		[]group.MemberIndex{1, 2},
		map[group.MemberIndex][]string{3: {"validation_gate_rejected"}},
		[]group.MemberIndex{3},
	)

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf(
			"sub-quorum accusations must not exclude; got %v",
			next.ExcludedSet,
		)
	}
	if memberSliceContains(next.TransientlyParked, 3) {
		t.Fatalf(
			"sub-quorum accusations must not park; got %v",
			next.TransientlyParked,
		)
	}
	if !memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf("sender 3 must remain included; got %v", next.IncludedSet)
	}
}

func TestNextAttempt_MultipleRejectReasonsAreOneAccuser(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	// Two observers each report three distinct reject reasons against
	// sender 3. Reasons must not multiply accusers: 2 accusers <
	// quorum (3), no action. Under summed counting this would have
	// been 6 >= 1 and an immediate permanent exclusion.
	bundle := buildBundleWithCategoriesFromAccusers(
		t,
		prev,
		[]group.MemberIndex{1, 2},
		map[group.MemberIndex][]string{
			3: {
				"validation_gate_rejected",
				"share_verification_failed",
				"commitment_malformed",
			},
		},
		nil,
	)

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf(
			"multiple reasons from few observers must not exclude; got %v",
			next.ExcludedSet,
		)
	}
	if !memberSliceContains(next.IncludedSet, 3) {
		t.Fatalf("sender 3 must remain included; got %v", next.IncludedSet)
	}
}

func TestNextAttempt_RejectAndConflictBothExclude(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	bundle := buildBundleWithCategories(
		t,
		prev,
		map[group.MemberIndex][]string{2: {"validation_gate_rejected"}},
		[]group.MemberIndex{4},
	)

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.ExcludedSet, 2) {
		t.Fatalf("sender 2 (reject) must be excluded; got %v", next.ExcludedSet)
	}
	if !memberSliceContains(next.ExcludedSet, 4) {
		t.Fatalf("sender 4 (conflict) must be excluded; got %v", next.ExcludedSet)
	}
}

func TestNextAttempt_EmptyRejectsAndConflicts_DoNotExclude(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	bundle := buildBundleWithCategories(t, prev, nil, nil)
	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if len(next.ExcludedSet) != 0 {
		t.Fatalf("no evidence -> no exclusions; got %v", next.ExcludedSet)
	}
}

func TestNextAttempt_ExactQuorumOfHonestObserversExcludes(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	// Exactly quorum (3 = f+1) accusers: established. This is the
	// boundary at which at least one accuser is guaranteed honest
	// under the t-of-n assumption.
	bundle := buildBundleWithCategoriesFromAccusers(
		t,
		prev,
		[]group.MemberIndex{1, 2, 4},
		map[group.MemberIndex][]string{3: {"validation_gate_rejected"}},
		nil,
	)

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey, fakeVerifier{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf(
			"exact quorum must establish the exclusion; got %v",
			next.ExcludedSet,
		)
	}
}
