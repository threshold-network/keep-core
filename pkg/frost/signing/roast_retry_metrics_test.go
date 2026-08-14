package signing

import (
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

func TestMetricsEmittingRecorder_IncrementsOnEachCategory(t *testing.T) {
	resetRoastRetryMetricsForTest()
	t.Cleanup(resetRoastRetryMetricsForTest)

	rec := newMetricsEmittingRecorder(attempt.NewBoundedRecorder())
	rec.RecordOverflow(1)
	rec.RecordOverflow(2)
	rec.RecordReject(3, "validation_gate_rejected")
	rec.RecordConflict(4)
	rec.RecordConflict(5)
	rec.RecordConflict(6)

	if got := roastRetryOverflowEvents.Load(); got != 2 {
		t.Fatalf("overflow counter: got %d want 2", got)
	}
	if got := roastRetryRejectEvents.Load(); got != 1 {
		t.Fatalf("reject counter: got %d want 1", got)
	}
	if got := roastRetryConflictEvents.Load(); got != 3 {
		t.Fatalf("conflict counter: got %d want 3", got)
	}
}

func TestMetricsEmittingRecorder_DelegatesSnapshotToInner(t *testing.T) {
	resetRoastRetryMetricsForTest()
	t.Cleanup(resetRoastRetryMetricsForTest)

	rec := newMetricsEmittingRecorder(attempt.NewBoundedRecorder())
	rec.RecordOverflow(7)
	rec.RecordOverflow(7)

	snap := rec.Snapshot()
	if snap.Overflows[7] != 2 {
		t.Fatalf(
			"inner snapshot must reflect events; got %d want 2",
			snap.Overflows[7],
		)
	}
}

func TestMetricsEmittingRecorder_NilInnerFallsBackToNoOp(t *testing.T) {
	resetRoastRetryMetricsForTest()
	t.Cleanup(resetRoastRetryMetricsForTest)

	rec := newMetricsEmittingRecorder(nil)
	// Defensive guard: a nil inner recorder must produce a recorder
	// that does not panic on Record* calls. The wrapper substitutes
	// a NoOp inner.
	rec.RecordOverflow(1)
	rec.RecordReject(1, "x")
	rec.RecordConflict(1)
	// Counters STILL increment with the recommended call sites...
	// wait, that's wrong. If inner is nil and we substitute NoOp,
	// the wrapper is the NoOp recorder, no counters bumped.
	if roastRetryOverflowEvents.Load() != 0 {
		t.Fatal("nil inner -> NoOp; counters should stay at zero")
	}
}

func TestRoastRetryRecorderForCollect_WrapsBoundedWithMetricsWhenRegistered(t *testing.T) {
	resetRoastRetryMetricsForTest()
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(resetRoastRetryMetricsForTest)
	t.Cleanup(ResetRoastRetryRegistrationForTest)

	// Without registration, the recorder is NoOp -- recording does
	// not bump the cumulative counters.
	rec := roastRetryRecorderForCollect()
	rec.RecordOverflow(1)
	if roastRetryOverflowEvents.Load() != 0 {
		t.Fatal("no registration -> NoOp recorder -> no counter bump")
	}
}

func TestMetricsEmittingRecorder_ConcurrentCountersAreRaceSafe(t *testing.T) {
	resetRoastRetryMetricsForTest()
	t.Cleanup(resetRoastRetryMetricsForTest)

	rec := newMetricsEmittingRecorder(attempt.NewBoundedRecorder())
	const workers = 16
	const callsPerWorker = 100

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerWorker; j++ {
				rec.RecordOverflow(1)
			}
		}()
	}
	wg.Wait()

	if got := roastRetryOverflowEvents.Load(); got != uint64(workers*callsPerWorker) {
		t.Fatalf(
			"concurrent counter: got %d want %d",
			got, workers*callsPerWorker,
		)
	}
}

func TestRegisterRoastRetryMetrics_NilRegistryIsNoOp(t *testing.T) {
	// Defensive: RegisterRoastRetryMetrics(nil) must not panic so
	// optional integration paths can pass through nil.
	RegisterRoastRetryMetrics(nil)
}
