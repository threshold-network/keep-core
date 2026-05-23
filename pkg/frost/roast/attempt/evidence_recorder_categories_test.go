package attempt

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestBoundedRecorder_RecordReject_AccumulatesByReason(t *testing.T) {
	rec := NewBoundedRecorder()
	rec.RecordReject(1, "validation_gate_rejected")
	rec.RecordReject(1, "validation_gate_rejected")
	rec.RecordReject(1, "some_other_reason")

	snap := rec.Snapshot()
	entries := snap.Rejects[1]
	if len(entries) != 2 {
		t.Fatalf("expected 2 reject reasons, got %d", len(entries))
	}
	counts := map[string]uint{}
	for _, e := range entries {
		counts[e.Reason] = e.Count
	}
	if counts["validation_gate_rejected"] != 2 {
		t.Fatalf("validation_gate_rejected count: got %d want 2", counts["validation_gate_rejected"])
	}
	if counts["some_other_reason"] != 1 {
		t.Fatalf("some_other_reason count: got %d want 1", counts["some_other_reason"])
	}
}

func TestBoundedRecorder_RecordReject_PerReasonQuota(t *testing.T) {
	rec := NewBoundedRecorderWithQuotas(8, 3, 4)
	for i := 0; i < 10; i++ {
		rec.RecordReject(1, "spam")
	}
	snap := rec.Snapshot()
	got := snap.Rejects[1][0].Count
	if got != 3 {
		t.Fatalf("reject quota not enforced: got %d, want 3", got)
	}
}

func TestBoundedRecorder_RecordReject_PerReasonQuotasIndependent(t *testing.T) {
	// A peer cannot saturate one reason to mask another -- each
	// reason has its own quota counter.
	rec := NewBoundedRecorderWithQuotas(8, 2, 4)
	for i := 0; i < 10; i++ {
		rec.RecordReject(1, "reason-A")
	}
	rec.RecordReject(1, "reason-B")
	snap := rec.Snapshot()
	counts := map[string]uint{}
	for _, e := range snap.Rejects[1] {
		counts[e.Reason] = e.Count
	}
	if counts["reason-A"] != 2 {
		t.Fatalf("reason-A saturated at: got %d want 2", counts["reason-A"])
	}
	if counts["reason-B"] != 1 {
		t.Fatalf("reason-B counted independently: got %d want 1", counts["reason-B"])
	}
}

func TestBoundedRecorder_RecordConflict_AccumulatesAndSaturates(t *testing.T) {
	rec := NewBoundedRecorderWithQuotas(8, 8, 2)
	rec.RecordConflict(7)
	rec.RecordConflict(7)
	rec.RecordConflict(7)
	rec.RecordConflict(7)
	snap := rec.Snapshot()
	if got := snap.Conflicts[7]; got != 2 {
		t.Fatalf("conflict count saturated at quota; got %d want 2", got)
	}
}

func TestBoundedRecorder_AllCategoriesPresentInSnapshot(t *testing.T) {
	rec := NewBoundedRecorder()
	rec.RecordOverflow(1)
	rec.RecordReject(2, "validation_gate_rejected")
	rec.RecordConflict(3)
	snap := rec.Snapshot()
	if snap.Overflows[1] == 0 {
		t.Fatal("overflow not recorded")
	}
	if len(snap.Rejects[2]) == 0 {
		t.Fatal("reject not recorded")
	}
	if snap.Conflicts[3] == 0 {
		t.Fatal("conflict not recorded")
	}
}

func TestNoOpRecorder_AllCategoriesInert(t *testing.T) {
	rec := NoOpRecorder()
	for i := 0; i < 100; i++ {
		rec.RecordOverflow(group.MemberIndex(i % 5))
		rec.RecordReject(group.MemberIndex(i%5), "spam")
		rec.RecordConflict(group.MemberIndex(i % 5))
	}
	snap := rec.Snapshot()
	if len(snap.Overflows) != 0 || len(snap.Rejects) != 0 || len(snap.Conflicts) != 0 {
		t.Fatalf("NoOp recorder must report empty snapshot; got %+v", snap)
	}
}

func TestRejectAndConflictQuotaConstants_MatchRFC(t *testing.T) {
	if RejectQuotaDefault != 8 {
		t.Fatalf("RFC-21 specifies reject quota = 8; constant is %d", RejectQuotaDefault)
	}
	if ConflictQuotaDefault != 4 {
		t.Fatalf("RFC-21 specifies conflict quota = 4; constant is %d", ConflictQuotaDefault)
	}
}
