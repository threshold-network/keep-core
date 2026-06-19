//go:build frost_roast_retry

package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func newTestContextForSubmit(t *testing.T, sessionID string) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		sessionID,
		"key-group-submit",
		[]byte{0xAA},
		[attempt.MessageDigestLength]byte{0x42},
		0,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)
	if err != nil {
		t.Fatalf("ctx: %v", err)
	}
	return ctx
}

// TestSubmitSnapshotIfActive_NilRecorderIsNoOp guards the cheap nil guard: the
// receive loop falls back to a nil/NoOp recorder when ROAST retry is inactive, and
// submit must not panic or stash.
func TestSubmitSnapshotIfActive_NilRecorderIsNoOp(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	submitSnapshotIfActive("session-nil", 1, nil)

	if PendingEvidenceStashedForTest("session-nil", 1) {
		t.Fatal("nil recorder must not stash")
	}
}

// TestSubmitSnapshotIfActive_NoOpWhenSessionUnbound asserts that without a
// session-handle binding (the orchestration layer has not run, or the default
// build), submit stashes nothing -- there is no attempt to attribute evidence to.
func TestSubmitSnapshotIfActive_NoOpWhenSessionUnbound(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	recorder := attempt.NewBoundedRecorder()
	recorder.RecordOverflow(7)
	submitSnapshotIfActive("session-with-no-binding", 1, recorder)

	if PendingEvidenceStashedForTest("session-with-no-binding", 1) {
		t.Fatal("expected no stash when session unbound")
	}
}

// TestSubmitSnapshotIfActive_NoOpWhenRecorderEmpty asserts a bound attempt whose
// recorder captured zero events stashes nothing: the exchange still broadcasts an
// empty proof-of-attendance snapshot, so skipping the stash does not silence-park
// the seat.
func TestSubmitSnapshotIfActive_NoOpWhenRecorderEmpty(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	const selfMember group.MemberIndex = 1
	ctx := newTestContextForSubmit(t, "session-empty")
	SetCurrentAttemptHandleForSession("session-empty", selfMember, roast.AttemptHandle{}, ctx)

	recorder := attempt.NewBoundedRecorder()
	submitSnapshotIfActive("session-empty", selfMember, recorder)

	if PendingEvidenceStashedForTest("session-empty", selfMember) {
		t.Fatal("expected no stash for an empty snapshot")
	}
}

// TestSubmitSnapshotIfActive_StashesEvidenceWhenBoundAndPopulated is the core
// blame-bridge wiring (RFC-21 Phase 7.3 PR2b-2 step 2): a bound attempt with
// captured evidence stashes the RAW evidence keyed by the attempt's
// (RoastSessionID==ctx.SessionID, member, attemptHash==ctx.Hash()) so the
// transition exchange's BroadcastForcedSnapshot can carry it into the bundle.
func TestSubmitSnapshotIfActive_StashesEvidenceWhenBoundAndPopulated(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	const selfMember group.MemberIndex = 1
	ctx := newTestContextForSubmit(t, "session-real")
	SetCurrentAttemptHandleForSession("session-real", selfMember, roast.AttemptHandle{}, ctx)

	recorder := attempt.NewBoundedRecorder()
	recorder.RecordOverflow(3)
	recorder.RecordOverflow(3)
	recorder.RecordOverflow(5)
	submitSnapshotIfActive("session-real", selfMember, recorder)

	evidence, ok := takePendingEvidence(ctx.SessionID, selfMember, ctx.Hash())
	if !ok {
		t.Fatal("expected stashed evidence after a populated submit")
	}
	// 2 distinct senders observed (3 twice, 5 once).
	if len(evidence.Overflows) != 2 {
		t.Fatalf("expected 2 overflow senders stashed; got %d", len(evidence.Overflows))
	}
	if evidence.Overflows[3] != 2 || evidence.Overflows[5] != 1 {
		t.Fatalf("stashed overflow counts wrong: %+v", evidence.Overflows)
	}
	// take consumes: a second take finds nothing.
	if _, ok := takePendingEvidence(ctx.SessionID, selfMember, ctx.Hash()); ok {
		t.Fatal("take must consume the stash entry")
	}
}

// TestSubmitSnapshotIfActive_StashesRejectOnlySnapshot guards the all-categories
// emptiness test: a snapshot carrying ONLY reject evidence -- no overflow, no
// conflict -- must still be stashed. A validation-blamable Reject populates
// Evidence.Rejects without any Overflow, and NextAttempt's exclusion path consumes
// snapshot.Rejects; an overflow-only emptiness check would starve the blame
// pipeline.
func TestSubmitSnapshotIfActive_StashesRejectOnlySnapshot(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	const selfMember group.MemberIndex = 1
	ctx := newTestContextForSubmit(t, "session-reject-only")
	SetCurrentAttemptHandleForSession("session-reject-only", selfMember, roast.AttemptHandle{}, ctx)

	recorder := attempt.NewBoundedRecorder()
	recorder.RecordReject(2, "attempt_context_hash_mismatch")
	submitSnapshotIfActive("session-reject-only", selfMember, recorder)

	evidence, ok := takePendingEvidence(ctx.SessionID, selfMember, ctx.Hash())
	if !ok {
		t.Fatal("reject-only snapshot must be stashed")
	}
	if len(evidence.Rejects[2]) == 0 {
		t.Fatalf("stashed evidence must carry the reject for sender 2; got %+v", evidence.Rejects)
	}
}

// TestSubmitSnapshotIfActive_MultiSeatStashesPerSeat asserts the PR2b-2
// member-aware path: with two local seats bound to the SAME attempt (same
// RoastSessionID + attemptHash), each seat stashes its OWN evidence under its own
// member key -- neither overwrites the other (the member-keying that fixes the
// sibling-collision hazard).
func TestSubmitSnapshotIfActive_MultiSeatStashesPerSeat(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	ResetPendingEvidenceRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetPendingEvidenceRegistryForTest)

	ctx := newTestContextForSubmit(t, "session-ms")
	SetCurrentAttemptHandleForSession("session-ms", 1, roast.AttemptHandle{}, ctx)
	SetCurrentAttemptHandleForSession("session-ms", 2, roast.AttemptHandle{}, ctx)

	rec1 := attempt.NewBoundedRecorder()
	rec1.RecordOverflow(3)
	rec2 := attempt.NewBoundedRecorder()
	rec2.RecordOverflow(4)

	submitSnapshotIfActive("session-ms", 1, rec1)
	submitSnapshotIfActive("session-ms", 2, rec2)

	ev1, ok1 := takePendingEvidence(ctx.SessionID, 1, ctx.Hash())
	ev2, ok2 := takePendingEvidence(ctx.SessionID, 2, ctx.Hash())
	if !ok1 || !ok2 {
		t.Fatalf("both seats must stash their own evidence; got ok1=%v ok2=%v", ok1, ok2)
	}
	// Seat 1 observed sender 3 only; seat 2 observed sender 4 only -- no bleed.
	if ev1.Overflows[3] != 1 || ev1.Overflows[4] != 0 {
		t.Fatalf("seat 1 stash must isolate its own evidence; got %+v", ev1.Overflows)
	}
	if ev2.Overflows[4] != 1 || ev2.Overflows[3] != 0 {
		t.Fatalf("seat 2 stash must isolate its own evidence; got %+v", ev2.Overflows)
	}
}

func TestSetCurrentAttemptHandleForSession_LaterBindingOverwrites(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctxA := newTestContextForSubmit(t, "session-overwrite")
	ctxB, _ := attempt.NewAttemptContext(
		"session-overwrite", "key-group-submit", []byte{0xAA},
		[attempt.MessageDigestLength]byte{0x42}, 1,
		[]group.MemberIndex{1, 2, 3, 4, 5}, nil,
	)
	h1 := roast.AttemptHandle{}
	h2 := roast.AttemptHandle{}

	SetCurrentAttemptHandleForSession("session-overwrite", 1, h1, ctxA)
	gotHandle, gotCtx, ok := currentAttemptHandleForCollect("session-overwrite", 1)
	if !ok {
		t.Fatal("expected binding after first Set")
	}
	if gotHandle != h1 {
		t.Fatal("first binding handle mismatch")
	}
	if gotCtx.AttemptNumber != ctxA.AttemptNumber {
		t.Fatal("first binding context mismatch")
	}

	SetCurrentAttemptHandleForSession("session-overwrite", 1, h2, ctxB)
	_, gotCtx2, ok := currentAttemptHandleForCollect("session-overwrite", 1)
	if !ok {
		t.Fatal("expected binding after second Set")
	}
	if gotCtx2.AttemptNumber != ctxB.AttemptNumber {
		t.Fatal("second binding context did not overwrite first")
	}
}

func TestClearCurrentAttemptHandleForSession_RemovesBinding(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newTestContextForSubmit(t, "session-clear")
	SetCurrentAttemptHandleForSession("session-clear", 1, roast.AttemptHandle{}, ctx)
	if _, _, ok := currentAttemptHandleForCollect("session-clear", 1); !ok {
		t.Fatal("setup: binding must exist")
	}
	ClearCurrentAttemptHandleForSession("session-clear", 1)
	if _, _, ok := currentAttemptHandleForCollect("session-clear", 1); ok {
		t.Fatal("binding must be cleared")
	}
}
