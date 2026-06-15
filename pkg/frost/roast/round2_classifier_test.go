package roast

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// fakeShareVerifier is a configurable Round2ShareVerifier for the classifier
// tests. Members without a configured verdict default to ShareValid (no blame);
// the calls slice records every member actually re-verified, so tests can assert
// that divergent-only and absent candidates are never handed to the verifier.
type fakeShareVerifier struct {
	verdicts map[group.MemberIndex]ShareVerificationResult
	calls    *[]group.MemberIndex
}

func (f fakeShareVerifier) VerifyRetainedShare(
	_ []byte,
	_ []byte,
	submitter group.MemberIndex,
) ShareVerificationResult {
	if f.calls != nil {
		*f.calls = append(*f.calls, submitter)
	}
	if r, ok := f.verdicts[submitter]; ok {
		return r
	}
	return ShareValid
}

// recordAcceptedShare records an authoritative-package-bound (accepted) share for
// submitter on an attempt already set up by recordTestPackage.
func recordAcceptedShare(t *testing.T, c *Round2Collector, submitter group.MemberIndex, pkgHash []byte) {
	t.Helper()
	if err := c.RecordShareSubmission(signedTestShareSubmission(t, submitter, pkgHash)); err != nil {
		t.Fatalf("record accepted share for %d: %v", submitter, err)
	}
}

func TestClassifyCandidateCulprits_InvalidAcceptedShareEmitsReject(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	elected := group.MemberIndex(testShareCoordinatorID)
	pkgHash := recordTestPackage(t, c, elected)
	recordAcceptedShare(t, c, 3, pkgHash)

	// ShareInvalid covers both a mathematically invalid share and undecodable
	// share bytes the member operator-signed: either is self-incriminating.
	verifier := fakeShareVerifier{verdicts: map[group.MemberIndex]ShareVerificationResult{3: ShareInvalid}}
	rejects, err := c.ClassifyCandidateCulprits(pinnedContextHash[:], []group.MemberIndex{3}, verifier)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	want := []RejectEntry{{Sender: 3, Reason: candidateCulpritRejectReason, Count: 1}}
	if !reflect.DeepEqual(rejects, want) {
		t.Fatalf("want %+v, got %+v", want, rejects)
	}
}

func TestClassifyCandidateCulprits_ValidAcceptedShareEmitsNothing(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	elected := group.MemberIndex(testShareCoordinatorID)
	pkgHash := recordTestPackage(t, c, elected)
	recordAcceptedShare(t, c, 3, pkgHash)

	// The engine flagged 3, but THIS observer's retained share re-verifies valid
	// against the package it accepted: not self-incriminating -> no accusation.
	verifier := fakeShareVerifier{verdicts: map[group.MemberIndex]ShareVerificationResult{3: ShareValid}}
	rejects, err := c.ClassifyCandidateCulprits(pinnedContextHash[:], []group.MemberIndex{3}, verifier)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(rejects) != 0 {
		t.Fatalf("a valid-but-flagged candidate must not be blamed, got %+v", rejects)
	}
}

func TestClassifyCandidateCulprits_IndeterminateEmitsNothing(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	elected := group.MemberIndex(testShareCoordinatorID)
	pkgHash := recordTestPackage(t, c, elected)
	recordAcceptedShare(t, c, 3, pkgHash)

	// An indeterminate re-verification (not the member's fault) must fail closed
	// against blame - distinct from ShareValid, but likewise emits nothing.
	verifier := fakeShareVerifier{verdicts: map[group.MemberIndex]ShareVerificationResult{3: ShareIndeterminate}}
	rejects, err := c.ClassifyCandidateCulprits(pinnedContextHash[:], []group.MemberIndex{3}, verifier)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(rejects) != 0 {
		t.Fatalf("indeterminate verification must not blame, got %+v", rejects)
	}
}

func TestClassifyCandidateCulprits_DivergentShareIsNeutral(t *testing.T) {
	_ = captureEquivocationEvidence(t)
	c := NewRound2Collector(fakeVerifier{})
	elected := group.MemberIndex(testShareCoordinatorID)
	_ = recordTestPackage(t, c, elected)

	// Member 3 has ONLY a divergent share (binds a non-authoritative package).
	wrong := bytes.Repeat([]byte{0x11}, SigningPackageHashLength)
	if err := c.RecordShareSubmission(signedTestShareSubmission(t, 3, wrong)); !errors.Is(err, ErrShareRetainedNotAccepted) {
		t.Fatalf("setup divergent share: want ErrShareRetainedNotAccepted, got %v", err)
	}

	calls := []group.MemberIndex{}
	verifier := fakeShareVerifier{
		// Would blame 3 if consulted - but a divergent-only candidate must be
		// skipped BEFORE re-verification.
		verdicts: map[group.MemberIndex]ShareVerificationResult{3: ShareInvalid},
		calls:    &calls,
	}
	rejects, err := c.ClassifyCandidateCulprits(pinnedContextHash[:], []group.MemberIndex{3}, verifier)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(rejects) != 0 {
		t.Fatalf("a divergent-only candidate must stay neutral, got %+v", rejects)
	}
	if len(calls) != 0 {
		t.Fatalf("the verifier must not be consulted for a divergent-only candidate, got %v", calls)
	}
}

func TestClassifyCandidateCulprits_AbsentCandidateIsNothing(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	elected := group.MemberIndex(testShareCoordinatorID)
	pkgHash := recordTestPackage(t, c, elected)
	recordAcceptedShare(t, c, 3, pkgHash)

	calls := []group.MemberIndex{}
	verifier := fakeShareVerifier{verdicts: map[group.MemberIndex]ShareVerificationResult{5: ShareInvalid}, calls: &calls}
	// 5 is in the included set but never submitted a share to this observer.
	rejects, err := c.ClassifyCandidateCulprits(pinnedContextHash[:], []group.MemberIndex{5}, verifier)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(rejects) != 0 {
		t.Fatalf("a candidate with no retained share must not be blamed, got %+v", rejects)
	}
	if len(calls) != 0 {
		t.Fatalf("the verifier must not be consulted for an absent candidate, got %v", calls)
	}
}

func TestClassifyCandidateCulprits_MultipleSortedAndDeduplicated(t *testing.T) {
	c := NewRound2Collector(fakeVerifier{})
	elected := group.MemberIndex(testShareCoordinatorID)
	pkgHash := recordTestPackage(t, c, elected)
	// All three included members submitted accepted shares.
	recordAcceptedShare(t, c, 3, pkgHash)
	recordAcceptedShare(t, c, 5, pkgHash)
	recordAcceptedShare(t, c, 7, pkgHash)

	// 3 and 7 re-verify invalid; 5 re-verifies valid (not blamed).
	verifier := fakeShareVerifier{verdicts: map[group.MemberIndex]ShareVerificationResult{
		3: ShareInvalid,
		5: ShareValid,
		7: ShareInvalid,
	}}
	// Candidates arrive unsorted and duplicated.
	rejects, err := c.ClassifyCandidateCulprits(
		pinnedContextHash[:],
		[]group.MemberIndex{7, 5, 3, 7, 3},
		verifier,
	)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	want := []RejectEntry{
		{Sender: 3, Reason: candidateCulpritRejectReason, Count: 1},
		{Sender: 7, Reason: candidateCulpritRejectReason, Count: 1},
	}
	if !reflect.DeepEqual(rejects, want) {
		t.Fatalf("want deduplicated, ascending rejects %+v, got %+v", want, rejects)
	}
}

func TestClassifyCandidateCulprits_Errors(t *testing.T) {
	elected := group.MemberIndex(testShareCoordinatorID)

	t.Run("nil verifier", func(t *testing.T) {
		c := NewRound2Collector(fakeVerifier{})
		_ = recordTestPackage(t, c, elected)
		if _, err := c.ClassifyCandidateCulprits(pinnedContextHash[:], []group.MemberIndex{3}, nil); err == nil {
			t.Fatal("a nil verifier must be rejected, not panic")
		}
	})

	t.Run("unknown attempt", func(t *testing.T) {
		c := NewRound2Collector(fakeVerifier{})
		_, err := c.ClassifyCandidateCulprits(pinnedContextHash[:], []group.MemberIndex{3}, fakeShareVerifier{})
		if !errors.Is(err, ErrRound2UnknownAttempt) {
			t.Fatalf("want ErrRound2UnknownAttempt, got %v", err)
		}
	})

	t.Run("no signing package recorded", func(t *testing.T) {
		c := NewRound2Collector(fakeVerifier{})
		if err := c.BeginAttempt(pinnedContextHash[:], elected, testIncludedSet()); err != nil {
			t.Fatalf("begin: %v", err)
		}
		_, err := c.ClassifyCandidateCulprits(pinnedContextHash[:], []group.MemberIndex{3}, fakeShareVerifier{})
		if !errors.Is(err, ErrRound2NoSigningPackage) {
			t.Fatalf("want ErrRound2NoSigningPackage, got %v", err)
		}
	})

	t.Run("no candidates yields no rejects", func(t *testing.T) {
		c := NewRound2Collector(fakeVerifier{})
		_ = recordTestPackage(t, c, elected)
		rejects, err := c.ClassifyCandidateCulprits(pinnedContextHash[:], nil, fakeShareVerifier{})
		if err != nil {
			t.Fatalf("classify: %v", err)
		}
		if len(rejects) != 0 {
			t.Fatalf("no candidates must yield no rejects, got %+v", rejects)
		}
	})
}
