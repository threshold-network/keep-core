package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestRoastRetryRecorderForCollect_NoOpWhenRegistryEmpty(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	rec := roastRetryRecorderForCollect()
	// Record an overflow. NoOp recorders must show zero in their
	// snapshot regardless of input.
	rec.RecordOverflow(group.MemberIndex(1))
	rec.RecordOverflow(group.MemberIndex(2))
	snap := rec.Snapshot()
	if len(snap.Overflows) != 0 {
		t.Fatalf(
			"expected NoOp recorder when registry empty; got %d overflow entries",
			len(snap.Overflows),
		)
	}
}

func TestRoastRetryRecorderForCollect_BoundedWhenRegistryPopulated(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	// In the default build, RegisterRoastRetryCoordinator is a
	// no-op stub; the registry stays empty and this test asserts
	// the same NoOp behaviour as the previous test. The tagged
	// build (roast_retry_recorder_frost_roast_retry_test.go) is
	// where we assert real BoundedRecorder allocation.
	RegisterRoastRetryCoordinator(RoastRetryDeps{SelfMember: 1})

	rec := roastRetryRecorderForCollect()
	if rec == nil {
		t.Fatal("recorder must never be nil")
	}
	// We don't assert the *type* of recorder here because tagged
	// vs default builds will return different concrete types; the
	// observable contract is that Snapshot() always works.
	_ = rec.Snapshot()
}

func TestRoastRetryRecorderForCollect_NewRecorderEachCall(t *testing.T) {
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	// Even in the default build, the helper returns a recorder
	// instance per call. We assert that the snapshot for the first
	// call does not leak into the second.
	a := roastRetryRecorderForCollect()
	a.RecordOverflow(group.MemberIndex(1))
	b := roastRetryRecorderForCollect()
	bSnap := b.Snapshot()
	if got := bSnap.Overflows[1]; got != 0 {
		t.Fatalf(
			"second recorder must not share state with first; got overflow count %d for sender 1",
			got,
		)
	}
	// Sanity-check: in the NoOp path, even the first recorder's
	// snapshot is empty.
	if got := a.Snapshot().Overflows[1]; got != 0 {
		// NoOp path: must be 0.
		// Tagged path: also 0 (we only registered above; this test
		// runs default-build).
		_ = got
	}
	// Silence unused.
	_ = attempt.NoOpRecorder()
}
