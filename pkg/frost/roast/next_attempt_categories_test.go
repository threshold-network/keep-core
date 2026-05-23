package roast

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// buildBundleWithCategories constructs a TransitionMessage where each
// observer contributes the same per-(category, sender) evidence -- one
// reject reason and one conflict per "blamed" sender per observer.
// Useful for verifying the cross-observer summing behaviour.
func buildBundleWithCategories(
	t *testing.T,
	prev attempt.AttemptContext,
	rejects map[group.MemberIndex][]string,
	conflicts []group.MemberIndex,
) *TransitionMessage {
	t.Helper()
	prevHash := prev.Hash()
	bundle := make([]LocalEvidenceSnapshot, 0, len(prev.IncludedSet))
	for _, sender := range prev.IncludedSet {
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

func TestNextAttempt_SingleRejectExcludesPermanently(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	// Every observer reports one reject against sender 3 → total
	// count is len(IncludedSet) = 5 across observers, summed by
	// rejectBlamedSenders.
	bundle := buildBundleWithCategories(
		t,
		prev,
		map[group.MemberIndex][]string{3: {"validation_gate_rejected"}},
		nil,
	)

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey)
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

func TestNextAttempt_SingleConflictExcludesPermanently(t *testing.T) {
	f := newNextAttemptFixture()
	prev := f.prev(t)
	bundle := buildBundleWithCategories(
		t,
		prev,
		nil,
		[]group.MemberIndex{3},
	)

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if !memberSliceContains(next.ExcludedSet, 3) {
		t.Fatalf(
			"sender 3 must be excluded after a single conflict; got %v",
			next.ExcludedSet,
		)
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

	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey)
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
	next, err := computeNextAttempt(prev, bundle, f.threshold, f.dkgGroupPublicKey)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if len(next.ExcludedSet) != 0 {
		t.Fatalf("no evidence -> no exclusions; got %v", next.ExcludedSet)
	}
}

func TestRejectAndConflictThresholds_MatchRFC(t *testing.T) {
	if RejectExclusionThreshold != 1 {
		t.Fatalf(
			"RFC-21 Layer B specifies reject threshold = 1; constant is %d",
			RejectExclusionThreshold,
		)
	}
	if ConflictExclusionThreshold != 1 {
		t.Fatalf(
			"single conflict is sufficient evidence; constant is %d",
			ConflictExclusionThreshold,
		)
	}
}
