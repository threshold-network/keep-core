package roast

import (
	"bytes"
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func captureEquivocationEvidence(t *testing.T) *[]EquivocationEvidence {
	t.Helper()
	captured := &[]EquivocationEvidence{}
	if err := RegisterEquivocationEvidenceObserver(
		func(evidence EquivocationEvidence) {
			*captured = append(*captured, evidence)
		},
	); err != nil {
		t.Fatalf("register observer: %v", err)
	}
	t.Cleanup(UnregisterEquivocationEvidenceObserver)
	return captured
}

func TestSnapshotConflict_RetainsBothSignedEnvelopes(t *testing.T) {
	captured := captureEquivocationEvidence(t)

	c := NewInMemoryCoordinator().(*inMemoryCoordinator)
	ctx := newTestContext(t)
	handle, err := c.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}

	first := signSnapshotForTest(
		t,
		NewLocalEvidenceSnapshot(3, ctx.Hash(), attempt.Evidence{
			Overflows: map[group.MemberIndex]uint{1: 1},
		}),
	)
	if err := c.RecordEvidence(handle, first); err != nil {
		t.Fatalf("record first: %v", err)
	}

	// The same sender equivocates: a different signed snapshot for the
	// same attempt.
	conflicting := signSnapshotForTest(
		t,
		NewLocalEvidenceSnapshot(3, ctx.Hash(), attempt.Evidence{
			Overflows: map[group.MemberIndex]uint{1: 2},
		}),
	)
	if err := c.RecordEvidence(handle, conflicting); !errors.Is(err, ErrSnapshotConflict) {
		t.Fatalf("expected ErrSnapshotConflict, got %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 evidence event, got %d", len(*captured))
	}
	evidence := (*captured)[0]
	if evidence.Kind != EquivocationKindSnapshotConflict {
		t.Fatalf("kind = %q", evidence.Kind)
	}
	if evidence.Sender != 3 {
		t.Fatalf("sender = %d", evidence.Sender)
	}
	wantExisting, _ := first.Marshal()
	wantConflicting, _ := conflicting.Marshal()
	if !bytes.Equal(evidence.ExistingEnvelope, wantExisting) {
		t.Fatal("existing envelope bytes must match the first submission verbatim")
	}
	if !bytes.Equal(evidence.ConflictingEnvelope, wantConflicting) {
		t.Fatal("conflicting envelope bytes must match the re-submission verbatim")
	}

	// Idempotent identical re-submission must NOT emit evidence.
	if err := c.RecordEvidence(handle, first); err != nil {
		t.Fatalf("identical re-submission should be a no-op: %v", err)
	}
	if len(*captured) != 1 {
		t.Fatalf("identical re-submission emitted evidence: %d events", len(*captured))
	}
}

func TestOwnSnapshotMutatedInBundle_RetainsBothSignedEnvelopes(t *testing.T) {
	captured := captureEquivocationEvidence(t)

	selfSubmission := signSnapshotForTest(
		t,
		NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{}),
	)

	mutated := *selfSubmission
	mutated.OperatorSignature = bytes.Repeat([]byte{0xff}, 64)
	// Fresh caches: the mutated copy is a distinct signed object.
	mutated.signedBody = nil
	mutated.wireEnvelope = nil

	bundle := &TransitionMessage{
		AttemptContextHash: append([]byte{}, pinnedContextHash[:]...),
		Bundle:             []LocalEvidenceSnapshot{mutated},
	}
	if err := verifyOwnObservationsPresent(bundle, 7, selfSubmission); !errors.Is(err, ErrCensorshipDetected) {
		t.Fatalf("expected ErrCensorshipDetected, got %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 evidence event, got %d", len(*captured))
	}
	evidence := (*captured)[0]
	if evidence.Kind != EquivocationKindOwnSnapshotMutatedInBundle {
		t.Fatalf("kind = %q", evidence.Kind)
	}
	wantSelf, _ := selfSubmission.Marshal()
	if !bytes.Equal(evidence.ExistingEnvelope, wantSelf) {
		t.Fatal("existing envelope must be the self submission verbatim")
	}
	if len(evidence.ConflictingEnvelope) == 0 {
		t.Fatal("conflicting envelope must carry the bundled snapshot")
	}
}

func TestOwnSnapshotMissingFromBundle_RetainsSelfEnvelope(t *testing.T) {
	captured := captureEquivocationEvidence(t)

	selfSubmission := signSnapshotForTest(
		t,
		NewLocalEvidenceSnapshot(7, pinnedContextHash, attempt.Evidence{}),
	)
	bundle := &TransitionMessage{
		AttemptContextHash: append([]byte{}, pinnedContextHash[:]...),
		Bundle:             []LocalEvidenceSnapshot{},
	}
	if err := verifyOwnObservationsPresent(bundle, 7, selfSubmission); !errors.Is(err, ErrCensorshipDetected) {
		t.Fatalf("expected ErrCensorshipDetected, got %v", err)
	}

	if len(*captured) != 1 {
		t.Fatalf("expected 1 evidence event, got %d", len(*captured))
	}
	evidence := (*captured)[0]
	if evidence.Kind != EquivocationKindOwnSnapshotMissingFromBundle {
		t.Fatalf("kind = %q", evidence.Kind)
	}
	wantSelf, _ := selfSubmission.Marshal()
	if !bytes.Equal(evidence.ExistingEnvelope, wantSelf) {
		t.Fatal("existing envelope must be the self submission verbatim")
	}
	if evidence.ConflictingEnvelope != nil {
		t.Fatal("missing-snapshot evidence has no conflicting envelope")
	}
}
