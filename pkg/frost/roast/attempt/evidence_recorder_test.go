package attempt

import (
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestNoOpRecorder_IsObservablyInert(t *testing.T) {
	rec := NoOpRecorder()
	for i := 0; i < 1000; i++ {
		rec.RecordOverflow(group.MemberIndex(i%5 + 1))
	}
	snap := rec.Snapshot()
	if len(snap.Overflows) != 0 {
		t.Fatalf(
			"NoOp recorder must report zero overflows; got %d entries",
			len(snap.Overflows),
		)
	}
}

func TestBoundedRecorder_CountsOverflowsBySender(t *testing.T) {
	rec := NewBoundedRecorder()
	rec.RecordOverflow(1)
	rec.RecordOverflow(2)
	rec.RecordOverflow(1)

	snap := rec.Snapshot()
	if got := snap.Overflows[1]; got != 2 {
		t.Fatalf("sender 1 overflow count: got %d want 2", got)
	}
	if got := snap.Overflows[2]; got != 1 {
		t.Fatalf("sender 2 overflow count: got %d want 1", got)
	}
	if _, ok := snap.Overflows[3]; ok {
		t.Fatal("sender 3 should have no entry")
	}
}

func TestBoundedRecorder_SaturatesAtQuota(t *testing.T) {
	const quota uint = 4
	rec := NewBoundedRecorderWithQuota(quota)

	for i := uint(0); i < quota+10; i++ {
		rec.RecordOverflow(1)
	}
	snap := rec.Snapshot()
	if got := snap.Overflows[1]; got != quota {
		t.Fatalf(
			"overflow count must saturate at quota %d; got %d",
			quota, got,
		)
	}
}

func TestBoundedRecorder_DefaultQuotaIs8(t *testing.T) {
	rec := NewBoundedRecorder()
	for i := 0; i < 100; i++ {
		rec.RecordOverflow(1)
	}
	if got := rec.Snapshot().Overflows[1]; got != OverflowQuotaDefault {
		t.Fatalf(
			"default quota mismatch; got %d want %d",
			got, OverflowQuotaDefault,
		)
	}
	if OverflowQuotaDefault != 8 {
		t.Fatalf(
			"RFC-21 Layer A specifies overflow quota = 8; constant is %d",
			OverflowQuotaDefault,
		)
	}
}

func TestBoundedRecorder_SnapshotIsDeepCopy(t *testing.T) {
	rec := NewBoundedRecorder()
	rec.RecordOverflow(1)
	rec.RecordOverflow(1)

	snap := rec.Snapshot()
	snap.Overflows[1] = 999
	snap.Overflows[42] = 7

	freshSnap := rec.Snapshot()
	if got := freshSnap.Overflows[1]; got != 2 {
		t.Fatalf(
			"snapshot mutation leaked into recorder state: got %d want 2",
			got,
		)
	}
	if _, ok := freshSnap.Overflows[42]; ok {
		t.Fatal("snapshot mutation leaked a new key into recorder state")
	}
}

func TestBoundedRecorder_ConcurrentRecordersAreRaceSafe(t *testing.T) {
	const (
		recordersPerSender = 8
		sendersN           = 16
		recordsPerRecorder = 200
	)
	rec := NewBoundedRecorderWithQuota(uint(recordersPerSender * recordsPerRecorder * 10))

	var wg sync.WaitGroup
	for senderIdx := 1; senderIdx <= sendersN; senderIdx++ {
		sender := group.MemberIndex(senderIdx)
		for w := 0; w < recordersPerSender; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for n := 0; n < recordsPerRecorder; n++ {
					rec.RecordOverflow(sender)
				}
			}()
		}
	}
	wg.Wait()

	snap := rec.Snapshot()
	for senderIdx := 1; senderIdx <= sendersN; senderIdx++ {
		want := uint(recordersPerSender * recordsPerRecorder)
		if got := snap.Overflows[group.MemberIndex(senderIdx)]; got != want {
			t.Fatalf(
				"sender %d concurrent count: got %d want %d",
				senderIdx, got, want,
			)
		}
	}
}

func TestNoOpRecorder_DistinctInstancesShareSemantics(t *testing.T) {
	a := NoOpRecorder()
	b := NoOpRecorder()
	a.RecordOverflow(1)
	b.RecordOverflow(2)
	if len(a.Snapshot().Overflows) != 0 || len(b.Snapshot().Overflows) != 0 {
		t.Fatal("NoOp instances must not retain state")
	}
}
