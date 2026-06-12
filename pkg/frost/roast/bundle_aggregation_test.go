package roast

import (
	"bytes"
	"errors"
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// pickNonCoordinatorMember returns the first member of `set` that is
// not equal to `elected`. Fatals if no such member exists. Used by
// receiver-side tests that need a member distinct from the
// aggregator.
func pickNonCoordinatorMember(
	t *testing.T,
	set []group.MemberIndex,
	elected group.MemberIndex,
) group.MemberIndex {
	t.Helper()
	for _, m := range set {
		if m != elected {
			return m
		}
	}
	t.Fatalf("no non-coordinator member available; set=%v elected=%d", set, elected)
	return 0
}

// signSnapshotForTest mints a fakeSigner signature on a snapshot and
// stores it on the snapshot's OperatorSignature field. Returns the
// snapshot for chaining.
func signSnapshotForTest(
	t *testing.T,
	snap *LocalEvidenceSnapshot,
) *LocalEvidenceSnapshot {
	t.Helper()
	signer := &fakeSigner{id: snap.SenderID()}
	payload, err := snap.SignableBytes()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	sig, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	snap.OperatorSignature = sig
	return snap
}

// newSignedCoordinatorForMember returns an inMemoryCoordinator wired
// for the named member to act as self -- meaning AggregateBundle is
// only callable when that member is the elected coordinator for the
// attempt under test.
func newSignedCoordinatorForMember(
	self group.MemberIndex,
) *inMemoryCoordinator {
	return NewInMemoryCoordinatorWithSigning(
		self,
		&fakeSigner{id: self},
		fakeVerifier{},
	).(*inMemoryCoordinator)
}

func TestRecordEvidence_RejectsNilSnapshot(t *testing.T) {
	c := newSignedCoordinatorForMember(0)
	handle, err := c.BeginAttempt(newTestContext(t))
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := c.RecordEvidence(handle, nil); err == nil {
		t.Fatal("expected nil snapshot error")
	}
}

func TestRecordEvidence_RejectsUnknownHandle(t *testing.T) {
	c := newSignedCoordinatorForMember(0)
	snap := signSnapshotForTest(t, NewLocalEvidenceSnapshot(1, pinnedContextHash, attempt.Evidence{}))
	bogus := AttemptHandle{id: 999}
	err := c.RecordEvidence(bogus, snap)
	if !errors.Is(err, ErrUnknownAttempt) {
		t.Fatalf("expected ErrUnknownAttempt, got %v", err)
	}
}

func TestRecordEvidence_RejectsContextHashMismatch(t *testing.T) {
	c := newSignedCoordinatorForMember(0)
	handle, err := c.BeginAttempt(newTestContext(t))
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	// Build a snapshot bound to a *different* context hash.
	wrongHash := [attempt.MessageDigestLength]byte{0xff}
	snap := signSnapshotForTest(t, NewLocalEvidenceSnapshot(1, wrongHash, attempt.Evidence{}))
	if err := c.RecordEvidence(handle, snap); !errors.Is(err, ErrAttemptContextMismatch) {
		t.Fatalf("expected ErrAttemptContextMismatch, got %v", err)
	}
}

func TestRecordEvidence_RejectsBadSignature(t *testing.T) {
	c := newSignedCoordinatorForMember(0)
	ctx := newTestContext(t)
	handle, err := c.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	snap := NewLocalEvidenceSnapshot(1, ctx.Hash(), attempt.Evidence{})
	snap.OperatorSignature = []byte{0xff, 0xee}
	err = c.RecordEvidence(handle, snap)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestRecordEvidence_AcceptsValidSnapshotAndIsIdempotent(t *testing.T) {
	c := newSignedCoordinatorForMember(0)
	ctx := newTestContext(t)
	handle, err := c.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	snap := signSnapshotForTest(
		t,
		NewLocalEvidenceSnapshot(1, ctx.Hash(), attempt.Evidence{}),
	)
	if err := c.RecordEvidence(handle, snap); err != nil {
		t.Fatalf("first record: %v", err)
	}
	// Identical re-submission must be idempotent.
	if err := c.RecordEvidence(handle, snap); err != nil {
		t.Fatalf("idempotent re-record: %v", err)
	}
}

func TestRecordEvidence_RejectsConflict(t *testing.T) {
	c := newSignedCoordinatorForMember(0)
	ctx := newTestContext(t)
	handle, err := c.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	first := signSnapshotForTest(
		t,
		NewLocalEvidenceSnapshot(1, ctx.Hash(), attempt.Evidence{}),
	)
	if err := c.RecordEvidence(handle, first); err != nil {
		t.Fatalf("first record: %v", err)
	}
	// Same sender, different evidence -> conflict.
	conflicting := signSnapshotForTest(
		t,
		NewLocalEvidenceSnapshot(1, ctx.Hash(), attempt.Evidence{
			Overflows: map[group.MemberIndex]uint{5: 3},
		}),
	)
	if err := c.RecordEvidence(handle, conflicting); !errors.Is(err, ErrSnapshotConflict) {
		t.Fatalf("expected ErrSnapshotConflict, got %v", err)
	}
}

func TestRecordEvidence_TracksSelfSubmission(t *testing.T) {
	const self group.MemberIndex = 3
	c := newSignedCoordinatorForMember(self)
	ctx := newTestContext(t)
	handle, err := c.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	selfSnap := signSnapshotForTest(
		t,
		NewLocalEvidenceSnapshot(self, ctx.Hash(), attempt.Evidence{}),
	)
	if err := c.RecordEvidence(handle, selfSnap); err != nil {
		t.Fatalf("record self: %v", err)
	}
	record := c.attempts[handle.id]
	if record.selfSubmission == nil {
		t.Fatal("expected selfSubmission to be set")
	}
	if record.selfSubmission.SenderID() != self {
		t.Fatalf("self submission member mismatch: got %d", record.selfSubmission.SenderID())
	}
}

func TestAggregateBundle_RejectsNonAggregator(t *testing.T) {
	// Two coordinator instances, both begin the same attempt. Only
	// the elected one can aggregate. We force the election by
	// building a context where SelectCoordinator will pick member 1.
	c := NewInMemoryCoordinatorWithSigning(99, &fakeSigner{id: 99}, fakeVerifier{}).(*inMemoryCoordinator)
	handle, err := c.BeginAttempt(newTestContext(t))
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	// member 99 is not in the IncludedSet, so it cannot be the
	// elected coordinator.
	_, err = c.AggregateBundle(handle)
	if !errors.Is(err, ErrNotAggregator) {
		t.Fatalf("expected ErrNotAggregator, got %v", err)
	}
}

func TestAggregateBundle_BuildsSignedBundle(t *testing.T) {
	// Pick the elected coordinator: run BeginAttempt once with a
	// throwaway coordinator instance to discover the elected member,
	// then build a real coordinator bound to that self.
	scratch := NewInMemoryCoordinator().(*inMemoryCoordinator)
	ctx := newTestContext(t)
	h0, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(h0)

	c := newSignedCoordinatorForMember(elected)
	handle, err := c.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	// Record snapshots from every included member.
	for _, m := range ctx.IncludedSet {
		snap := signSnapshotForTest(
			t,
			NewLocalEvidenceSnapshot(m, ctx.Hash(), attempt.Evidence{}),
		)
		if err := c.RecordEvidence(handle, snap); err != nil {
			t.Fatalf("record %d: %v", m, err)
		}
	}
	bundle, err := c.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(bundle.Bundle) != len(ctx.IncludedSet) {
		t.Fatalf(
			"bundle size: got %d want %d",
			len(bundle.Bundle), len(ctx.IncludedSet),
		)
	}
	for i := 1; i < len(bundle.Bundle); i++ {
		if bundle.Bundle[i].SenderIDValue <= bundle.Bundle[i-1].SenderIDValue {
			t.Fatalf("bundle not sorted ascending at %d", i)
		}
	}
	if bundle.CoordinatorID() != elected {
		t.Fatalf("bundle coordinator id %d != elected %d", bundle.CoordinatorID(), elected)
	}
	if len(bundle.CoordinatorSignature) == 0 {
		t.Fatal("expected coordinator signature to be populated")
	}
	state, _ := c.State(handle)
	if state != AttemptStateTransitioned {
		t.Fatalf("expected state Transitioned, got %v", state)
	}
}

func TestAggregateBundle_ProducesDeterministicBundleAcrossOrderings(t *testing.T) {
	// Two coordinators aggregate the same evidence in different
	// arrival orders. The resulting bundles must be byte-identical
	// after JSON marshal.
	scratch := NewInMemoryCoordinator().(*inMemoryCoordinator)
	ctx := newTestContext(t)
	h0, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(h0)

	make := func(
		t *testing.T,
		recordOrder []group.MemberIndex,
	) []byte {
		t.Helper()
		c := newSignedCoordinatorForMember(elected)
		handle, err := c.BeginAttempt(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		for _, m := range recordOrder {
			snap := signSnapshotForTest(
				t,
				NewLocalEvidenceSnapshot(m, ctx.Hash(), attempt.Evidence{}),
			)
			if err := c.RecordEvidence(handle, snap); err != nil {
				t.Fatalf("record %d: %v", m, err)
			}
		}
		bundle, err := c.AggregateBundle(handle)
		if err != nil {
			t.Fatalf("aggregate: %v", err)
		}
		data, err := bundle.Marshal()
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return data
	}
	ordering1 := []group.MemberIndex{1, 2, 3, 4, 5}
	ordering2 := []group.MemberIndex{5, 3, 1, 4, 2}
	a := make(t, ordering1)
	b := make(t, ordering2)
	if !bytes.Equal(a, b) {
		t.Fatalf(
			"identical evidence in different arrival order produced "+
				"different bundles:\n a=%s\n b=%s",
			string(a), string(b),
		)
	}
}

func TestVerifyBundle_AcceptsValidBundle(t *testing.T) {
	scratch := NewInMemoryCoordinator().(*inMemoryCoordinator)
	ctx := newTestContext(t)
	h0, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(h0)

	aggregator := newSignedCoordinatorForMember(elected)
	handle, err := aggregator.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("aggregator begin: %v", err)
	}
	for _, m := range ctx.IncludedSet {
		snap := signSnapshotForTest(
			t,
			NewLocalEvidenceSnapshot(m, ctx.Hash(), attempt.Evidence{}),
		)
		if err := aggregator.RecordEvidence(handle, snap); err != nil {
			t.Fatalf("record %d: %v", m, err)
		}
	}
	bundle, err := aggregator.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	// Receiver: a different coordinator instance bound to a
	// non-coordinator member that has not submitted its own snapshot.
	// The receiver must accept the bundle.
	receiverID := pickNonCoordinatorMember(t, ctx.IncludedSet, elected)
	receiver := NewInMemoryCoordinatorWithSigning(
		receiverID,
		&fakeSigner{id: receiverID},
		fakeVerifier{},
	).(*inMemoryCoordinator)
	rh, err := receiver.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("receiver begin: %v", err)
	}
	if err := receiver.VerifyBundle(rh, bundle); err != nil {
		t.Fatalf("expected verify success, got %v", err)
	}
}

func TestVerifyBundle_DetectsCensorship(t *testing.T) {
	scratch := NewInMemoryCoordinator().(*inMemoryCoordinator)
	ctx := newTestContext(t)
	h0, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(h0)

	aggregator := newSignedCoordinatorForMember(elected)
	handle, err := aggregator.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("agg begin: %v", err)
	}
	// Record snapshots from every member EXCEPT receiverID.
	receiverID := pickNonCoordinatorMember(t, ctx.IncludedSet, elected)
	for _, m := range ctx.IncludedSet {
		if m == receiverID {
			continue
		}
		snap := signSnapshotForTest(
			t,
			NewLocalEvidenceSnapshot(m, ctx.Hash(), attempt.Evidence{}),
		)
		if err := aggregator.RecordEvidence(handle, snap); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	bundle, err := aggregator.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	// Receiver: bound to receiverID, has submitted its own snapshot,
	// but the coordinator chose to censor it.
	receiver := NewInMemoryCoordinatorWithSigning(
		receiverID,
		&fakeSigner{id: receiverID},
		fakeVerifier{},
	).(*inMemoryCoordinator)
	rh, err := receiver.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("receiver begin: %v", err)
	}
	selfSnap := signSnapshotForTest(
		t,
		NewLocalEvidenceSnapshot(receiverID, ctx.Hash(), attempt.Evidence{}),
	)
	if err := receiver.RecordEvidence(rh, selfSnap); err != nil {
		t.Fatalf("receiver record self: %v", err)
	}
	err = receiver.VerifyBundle(rh, bundle)
	if !errors.Is(err, ErrCensorshipDetected) {
		t.Fatalf("expected ErrCensorshipDetected, got %v", err)
	}
}

func TestVerifyBundle_DetectsCoordinatorSignatureForgery(t *testing.T) {
	scratch := NewInMemoryCoordinator().(*inMemoryCoordinator)
	ctx := newTestContext(t)
	h0, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(h0)

	aggregator := newSignedCoordinatorForMember(elected)
	handle, _ := aggregator.BeginAttempt(ctx)
	for _, m := range ctx.IncludedSet {
		_ = aggregator.RecordEvidence(handle, signSnapshotForTest(
			t,
			NewLocalEvidenceSnapshot(m, ctx.Hash(), attempt.Evidence{}),
		))
	}
	bundle, _ := aggregator.AggregateBundle(handle)
	// Tamper: re-sign the bundle as a different (non-elected) member.
	const wrongSigner group.MemberIndex = 99
	bundle.CoordinatorIDValue = uint32(wrongSigner)
	payload, _ := bundle.SignableBytes()
	forged, _ := (&fakeSigner{id: wrongSigner}).Sign(payload)
	bundle.CoordinatorSignature = forged

	receiver := NewInMemoryCoordinatorWithSigning(
		7,
		&fakeSigner{id: 7},
		fakeVerifier{},
	).(*inMemoryCoordinator)
	rh, _ := receiver.BeginAttempt(ctx)
	err := receiver.VerifyBundle(rh, bundle)
	if err == nil {
		t.Fatal("expected verification failure")
	}
}

func TestVerifyBundle_DetectsSnapshotSignatureForgery(t *testing.T) {
	scratch := NewInMemoryCoordinator().(*inMemoryCoordinator)
	ctx := newTestContext(t)
	h0, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(h0)

	aggregator := newSignedCoordinatorForMember(elected)
	handle, _ := aggregator.BeginAttempt(ctx)
	for _, m := range ctx.IncludedSet {
		_ = aggregator.RecordEvidence(handle, signSnapshotForTest(
			t,
			NewLocalEvidenceSnapshot(m, ctx.Hash(), attempt.Evidence{}),
		))
	}
	bundle, _ := aggregator.AggregateBundle(handle)

	// Tamper: replace one snapshot's signature with garbage. The
	// bundle's coordinator signature still validates (since the
	// canonical bundle bytes include the snapshot signature, an
	// integrated bundle would have detected the change at the
	// coordinator-signature layer). For this test we re-sign the
	// bundle with the new garbage signature so the bundle-level
	// signature appears valid but the snapshot signature does not.
	bundle.Bundle[0].OperatorSignature = []byte{0xde, 0xad}
	payload, _ := bundle.SignableBytes()
	resign, _ := (&fakeSigner{id: elected}).Sign(payload)
	bundle.CoordinatorSignature = resign

	receiver := NewInMemoryCoordinatorWithSigning(
		7,
		&fakeSigner{id: 7},
		fakeVerifier{},
	).(*inMemoryCoordinator)
	rh, _ := receiver.BeginAttempt(ctx)
	err := receiver.VerifyBundle(rh, bundle)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifyBundle_RejectsAttemptContextMismatch(t *testing.T) {
	scratch := NewInMemoryCoordinator().(*inMemoryCoordinator)
	ctx := newTestContext(t)
	h0, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(h0)

	aggregator := newSignedCoordinatorForMember(elected)
	handle, _ := aggregator.BeginAttempt(ctx)
	for _, m := range ctx.IncludedSet {
		_ = aggregator.RecordEvidence(handle, signSnapshotForTest(
			t,
			NewLocalEvidenceSnapshot(m, ctx.Hash(), attempt.Evidence{}),
		))
	}
	bundle, _ := aggregator.AggregateBundle(handle)

	receiver := NewInMemoryCoordinatorWithSigning(
		7,
		&fakeSigner{id: 7},
		fakeVerifier{},
	).(*inMemoryCoordinator)

	// Receiver begins a different attempt context.
	wrongCtx, _ := attempt.NewAttemptContext(
		"different-session",
		"key-group-test",
		[]byte{0xab, 0xcd, 0xef},
		[attempt.MessageDigestLength]byte{0x42},
		0,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)
	rh, _ := receiver.BeginAttempt(wrongCtx)
	err := receiver.VerifyBundle(rh, bundle)
	if !errors.Is(err, ErrAttemptContextMismatch) {
		t.Fatalf("expected ErrAttemptContextMismatch, got %v", err)
	}
}

func TestVerifyBundle_RejectsNilMessage(t *testing.T) {
	c := newSignedCoordinatorForMember(7)
	handle, _ := c.BeginAttempt(newTestContext(t))
	if err := c.VerifyBundle(handle, nil); err == nil {
		t.Fatal("expected error for nil message")
	}
}

func TestVerifyBundle_RejectsUnknownAttempt(t *testing.T) {
	c := newSignedCoordinatorForMember(7)
	bundle := buildValidTransitionMessage()
	bogus := AttemptHandle{id: 999}
	if err := c.VerifyBundle(bogus, bundle); !errors.Is(err, ErrUnknownAttempt) {
		t.Fatalf("expected ErrUnknownAttempt, got %v", err)
	}
}

func TestCoordinator_ConcurrentRecordAndVerifyAreRaceSafe(t *testing.T) {
	scratch := NewInMemoryCoordinator().(*inMemoryCoordinator)
	ctx := newTestContext(t)
	h0, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(h0)

	aggregator := newSignedCoordinatorForMember(elected)
	handle, _ := aggregator.BeginAttempt(ctx)
	var wg sync.WaitGroup
	for _, m := range ctx.IncludedSet {
		wg.Add(1)
		mLocal := m
		go func() {
			defer wg.Done()
			snap := signSnapshotForTest(t, NewLocalEvidenceSnapshot(mLocal, ctx.Hash(), attempt.Evidence{}))
			if err := aggregator.RecordEvidence(handle, snap); err != nil {
				t.Errorf("concurrent record %d: %v", mLocal, err)
			}
		}()
	}
	wg.Wait()
	bundle, err := aggregator.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate after concurrent records: %v", err)
	}
	if len(bundle.Bundle) != len(ctx.IncludedSet) {
		t.Fatalf(
			"bundle size after concurrent records: got %d want %d",
			len(bundle.Bundle), len(ctx.IncludedSet),
		)
	}
}
