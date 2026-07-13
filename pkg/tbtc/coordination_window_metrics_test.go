package tbtc

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/clientinfo"
)

// noopMetrics satisfies clientinfo.PerformanceMetricsRecorder with no side effects.
type noopMetrics struct{}

func (n *noopMetrics) IncrementCounter(name string, value float64) {}
func (n *noopMetrics) RecordDuration(name string, d time.Duration) {}
func (n *noopMetrics) SetGauge(name string, value float64)         {}
func (n *noopMetrics) GetCounterValue(name string) float64         { return 0 }
func (n *noopMetrics) GetGaugeValue(name string) float64           { return 0 }

var _ clientinfo.PerformanceMetricsRecorder = (*noopMetrics)(nil)

func newTestWindowMetrics(max uint64) *coordinationWindowMetrics {
	return newCoordinationWindowMetrics(&noopMetrics{}, max)
}

// --- faultMessage ---

func TestFaultMessage_LeaderIdleness(t *testing.T) {
	msg := faultMessage(FaultLeaderIdleness, "0xabc")
	if msg == "" {
		t.Error("expected non-empty fault message for LeaderIdleness")
	}
}

func TestFaultMessage_LeaderMistake(t *testing.T) {
	msg := faultMessage(FaultLeaderMistake, "0xabc")
	if msg == "" {
		t.Error("expected non-empty fault message for LeaderMistake")
	}
}

func TestFaultMessage_LeaderImpersonation(t *testing.T) {
	msg := faultMessage(FaultLeaderImpersonation, "0xabc")
	if msg == "" {
		t.Error("expected non-empty fault message for LeaderImpersonation")
	}
}

func TestFaultMessage_Unknown(t *testing.T) {
	msg := faultMessage(FaultUnknown, "0xabc")
	if msg == "" {
		t.Error("expected non-empty fault message for FaultUnknown")
	}
}

// --- recordWindowStart / recordWindowEnd lifecycle ---

func TestCoordinationWindowMetrics_RecordWindowLifecycle(t *testing.T) {
	cwm := newTestWindowMetrics(10)
	window := newCoordinationWindow(900)

	cwm.recordWindowStart(window)

	wm, ok := cwm.GetWindowMetrics(window.index())
	if !ok {
		t.Fatal("window should exist after recordWindowStart")
	}
	if wm.StartTime.IsZero() {
		t.Error("StartTime should be set after recordWindowStart")
	}
	if !wm.EndTime.IsZero() {
		t.Error("EndTime should not be set before recordWindowEnd")
	}

	cwm.recordWindowEnd(window)

	wm, ok = cwm.GetWindowMetrics(window.index())
	if !ok {
		t.Fatal("window should still exist after recordWindowEnd")
	}
	if wm.EndTime.IsZero() {
		t.Error("EndTime should be set after recordWindowEnd")
	}
	if wm.Duration == 0 {
		t.Error("Duration should be set after recordWindowEnd")
	}
}

func TestCoordinationWindowMetrics_RecordWindowEnd_Idempotent(t *testing.T) {
	cwm := newTestWindowMetrics(10)
	window := newCoordinationWindow(900)

	cwm.recordWindowStart(window)
	cwm.recordWindowEnd(window)

	wm, _ := cwm.GetWindowMetrics(window.index())
	firstEndTime := wm.EndTime

	// Second call should be a no-op (EndTime guard).
	cwm.recordWindowEnd(window)
	wm, _ = cwm.GetWindowMetrics(window.index())
	if wm.EndTime != firstEndTime {
		t.Error("second recordWindowEnd should not overwrite EndTime")
	}
}

func TestCoordinationWindowMetrics_RecordWindowEnd_NoWindow(t *testing.T) {
	cwm := newTestWindowMetrics(10)
	window := newCoordinationWindow(900)

	// recordWindowEnd without prior recordWindowStart should not panic.
	cwm.recordWindowEnd(window)
}

// --- GetWindowMetrics ---

func TestCoordinationWindowMetrics_GetWindowMetrics_Missing(t *testing.T) {
	cwm := newTestWindowMetrics(10)

	_, ok := cwm.GetWindowMetrics(9999)
	if ok {
		t.Error("expected false for unknown window index")
	}
}

func TestCoordinationWindowMetrics_GetWindowMetrics_DeepCopy(t *testing.T) {
	cwm := newTestWindowMetrics(10)
	window := newCoordinationWindow(900)

	cwm.recordWindowStart(window)

	wm, ok := cwm.GetWindowMetrics(window.index())
	if !ok {
		t.Fatal("window not found")
	}

	// Mutate the returned copy.
	wm.WalletsCoordinated = 999
	wm.Leaders["external_mutation"] = 1

	// Internal state should be unaffected.
	internal, _ := cwm.GetWindowMetrics(window.index())
	if internal.WalletsCoordinated == 999 {
		t.Error("mutation of returned copy should not affect stored metrics")
	}
	if _, exists := internal.Leaders["external_mutation"]; exists {
		t.Error("map mutation of returned copy should not affect stored metrics")
	}
}

// --- GetSummary ---

func TestCoordinationWindowMetrics_GetSummary_Empty(t *testing.T) {
	cwm := newTestWindowMetrics(10)

	summary := cwm.GetSummary()

	testutils.AssertIntsEqual(t, "TotalWindows", 0, int(summary.TotalWindows))
	testutils.AssertIntsEqual(t, "TotalWalletsCoordinated", 0, int(summary.TotalWalletsCoordinated))
	testutils.AssertIntsEqual(t, "TotalFaults", 0, int(summary.TotalFaults))
}

func TestCoordinationWindowMetrics_GetSummary_AggregatesMultipleWindows(t *testing.T) {
	cwm := newTestWindowMetrics(10)

	leader := chain.Address("0xleader")

	for i := uint64(1); i <= 3; i++ {
		window := newCoordinationWindow(i * 900)
		cwm.recordWalletCoordination(
			window,
			[20]byte{byte(i)},
			leader,
			"Heartbeat",
			true,
			10*time.Millisecond,
			nil,
			nil,
		)
	}

	summary := cwm.GetSummary()

	testutils.AssertIntsEqual(t, "TotalWindows", 3, int(summary.TotalWindows))
	testutils.AssertIntsEqual(t, "TotalWalletsCoordinated", 3, int(summary.TotalWalletsCoordinated))
	testutils.AssertIntsEqual(t, "TotalWalletsSuccessful", 3, int(summary.TotalWalletsSuccessful))
	testutils.AssertIntsEqual(t, "TotalFaults", 0, int(summary.TotalFaults))
}

// --- GetRecentWindows ---

func TestCoordinationWindowMetrics_GetRecentWindows_Order(t *testing.T) {
	cwm := newTestWindowMetrics(10)
	leader := chain.Address("0xleader")

	for i := uint64(1); i <= 5; i++ {
		window := newCoordinationWindow(i * 900)
		cwm.recordWalletCoordination(window, [20]byte{}, leader, "Heartbeat", true, 0, nil, nil)
	}

	windows := cwm.GetRecentWindows(3)

	if len(windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(windows))
	}
	// Most recent first.
	if windows[0].WindowIndex <= windows[1].WindowIndex {
		t.Error("GetRecentWindows should return most recent window first")
	}
}

func TestCoordinationWindowMetrics_GetRecentWindows_LimitHigherThanCount(t *testing.T) {
	cwm := newTestWindowMetrics(10)
	leader := chain.Address("0xleader")

	window := newCoordinationWindow(900)
	cwm.recordWalletCoordination(window, [20]byte{}, leader, "Heartbeat", true, 0, nil, nil)

	windows := cwm.GetRecentWindows(100)
	if len(windows) != 1 {
		t.Errorf("expected 1 window, got %d", len(windows))
	}
}

// --- cleanupOldWindows (via maxWindowsToTrack) ---

func TestCoordinationWindowMetrics_CleanupOldWindows(t *testing.T) {
	const max = 3
	cwm := newTestWindowMetrics(max)
	leader := chain.Address("0xleader")

	// Insert 5 windows; only the 3 most recent should survive.
	for i := uint64(1); i <= 5; i++ {
		window := newCoordinationWindow(i * 900)
		cwm.recordWalletCoordination(window, [20]byte{}, leader, "Heartbeat", true, 0, nil, nil)
	}

	summary := cwm.GetSummary()
	if summary.TotalWindows > max {
		t.Errorf("expected at most %d windows after cleanup, got %d", max, summary.TotalWindows)
	}

	// The oldest windows (index 1, 2) should have been evicted.
	for _, i := range []uint64{1, 2} {
		if _, ok := cwm.GetWindowMetrics(i); ok {
			t.Errorf("window index %d should have been evicted", i)
		}
	}
}

// --- recordWalletCoordination ---

func TestCoordinationWindowMetrics_RecordWalletCoordination_Success(t *testing.T) {
	cwm := newTestWindowMetrics(10)
	window := newCoordinationWindow(900)
	leader := chain.Address("0xleader")

	cwm.recordWalletCoordination(
		window, [20]byte{1}, leader, "DepositSweep", true, 50*time.Millisecond, nil, nil,
	)

	wm, ok := cwm.GetWindowMetrics(window.index())
	if !ok {
		t.Fatal("window should exist after recordWalletCoordination")
	}
	testutils.AssertIntsEqual(t, "WalletsCoordinated", 1, int(wm.WalletsCoordinated))
	testutils.AssertIntsEqual(t, "WalletsSuccessful", 1, int(wm.WalletsSuccessful))
	testutils.AssertIntsEqual(t, "WalletsFailed", 0, int(wm.WalletsFailed))
	if wm.ActionTypes["DepositSweep"] != 1 {
		t.Error("expected DepositSweep action type to be recorded")
	}
}

func TestCoordinationWindowMetrics_RecordWalletCoordination_Failure(t *testing.T) {
	cwm := newTestWindowMetrics(10)
	window := newCoordinationWindow(900)
	leader := chain.Address("0xleader")
	coordErr := fmt.Errorf("proposal rejected")

	cwm.recordWalletCoordination(
		window, [20]byte{2}, leader, "Redemption", false, 0, nil, coordErr,
	)

	wm, _ := cwm.GetWindowMetrics(window.index())
	testutils.AssertIntsEqual(t, "WalletsFailed", 1, int(wm.WalletsFailed))
	testutils.AssertIntsEqual(t, "WalletsSuccessful", 0, int(wm.WalletsSuccessful))

	if len(wm.WalletCoordinationDetails) == 0 || wm.WalletCoordinationDetails[0].ErrorMessage == "" {
		t.Error("expected error message in coordination detail")
	}
}

func TestCoordinationWindowMetrics_RecordWalletCoordination_Faults(t *testing.T) {
	cwm := newTestWindowMetrics(10)
	window := newCoordinationWindow(900)
	leader := chain.Address("0xleader")
	culprit := chain.Address("0xbad")

	faults := []*coordinationFault{
		{culprit: culprit, faultType: FaultLeaderIdleness},
	}

	cwm.recordWalletCoordination(
		window, [20]byte{3}, leader, "Heartbeat", false, 0, faults, nil,
	)

	wm, _ := cwm.GetWindowMetrics(window.index())
	testutils.AssertIntsEqual(t, "TotalFaults", 1, int(wm.TotalFaults))

	if wm.FaultsByCulprit[culprit.String()] != 1 {
		t.Errorf("expected fault to be attributed to culprit %s", culprit)
	}

	details := wm.WalletCoordinationDetails
	if len(details) == 0 || len(details[0].Faults) == 0 {
		t.Fatal("expected fault details in wallet coordination detail")
	}
	if details[0].Faults[0].Message == "" {
		t.Error("expected non-empty fault message in detail")
	}
}

// --- concurrent safety ---

func TestCoordinationWindowMetrics_Concurrent(t *testing.T) {
	cwm := newTestWindowMetrics(20)
	leader := chain.Address("0xleader")

	var wg sync.WaitGroup
	for i := uint64(1); i <= 10; i++ {
		wg.Add(1)
		go func(i uint64) {
			defer wg.Done()
			window := newCoordinationWindow(i * 900)
			cwm.recordWindowStart(window)
			cwm.recordWalletCoordination(window, [20]byte{byte(i)}, leader, "Heartbeat", true, 0, nil, nil)
			cwm.recordWindowEnd(window)
		}(i)
	}
	wg.Wait()

	// All reads should complete without data races.
	_ = cwm.GetSummary()
	_ = cwm.GetRecentWindows(5)
}
