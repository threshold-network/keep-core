package spv

import (
	"errors"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/clientinfo"
)

// newOverdueActionWatcher returns an action-timeout watcher with one
// Pending reanchor generation (timeoutAt 1000, see newG2L11Watcher) that
// reports through a fake metrics recorder.
func newOverdueActionWatcher(
	t *testing.T,
	nodeNow *uint32,
) (*localChain, *ReservationActionTimeoutWatcher, *fakeMetricsRecorder) {
	t.Helper()

	spvChain, ratw := newG2L11Watcher(t, nodeNow)
	recorder := newFakeMetricsRecorder()
	ratw.metricsRecorder = recorder

	return spvChain, ratw, recorder
}

// TestG4F6_OverdueActionRaisesAlert pins the client-side alert: a tracked
// action that is still Pending past its timeoutAt in chain time, with no
// proof submitted, increments
// clientinfo.MetricSpvReservationActionOverdueTotal at the notify decision.
func TestG4F6_OverdueActionRaisesAlert(t *testing.T) {
	nodeNow := uint32(10)
	spvChain, ratw, recorder := newOverdueActionWatcher(t, &nodeNow)
	// Past timeoutAt (1000) plus the maximum stagger offset; the node clock
	// is deliberately far behind, so only the chain head time can trigger
	// the alert.
	spvChain.setLatestBlockTimestamp(time.Unix(1000+601, 0))

	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := recorder.counters[clientinfo.MetricSpvReservationActionOverdueTotal]; got != 1 {
		t.Errorf("expected the overdue counter to be 1, got %v", got)
	}
	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Errorf("expected the timeout notice to still be submitted, got %d", len(calls))
	}
}

// TestG4F6_NoAlertBeforeDeadlineOrWithUnreadableHead pins that the alert
// stays silent while the action is not yet overdue in chain time, and when
// the head time cannot be read.
func TestG4F6_NoAlertBeforeDeadlineOrWithUnreadableHead(t *testing.T) {
	t.Run("chain head before timeoutAt", func(t *testing.T) {
		// A node clock far past the deadline must not trigger the alert.
		nodeNow := uint32(1_000_000)
		spvChain, ratw, recorder := newOverdueActionWatcher(t, &nodeNow)
		spvChain.setLatestBlockTimestamp(time.Unix(500, 0))

		if err := ratw.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := recorder.counters[clientinfo.MetricSpvReservationActionOverdueTotal]; got != 0 {
			t.Errorf("expected no overdue alert before timeoutAt, got %v", got)
		}
	})

	t.Run("unreadable chain head", func(t *testing.T) {
		nodeNow := uint32(1_000_000)
		spvChain, ratw, recorder := newOverdueActionWatcher(t, &nodeNow)
		spvChain.latestBlockTimestampErr = errors.New("head unavailable")

		if err := ratw.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := recorder.counters[clientinfo.MetricSpvReservationActionOverdueTotal]; got != 0 {
			t.Errorf("expected no overdue alert with an unreadable head, got %v", got)
		}
	})
}

// TestG4F6_AlertRepeatsOncePerRenotifyInterval pins that an action that
// stays Pending is reported again only when the renotify backoff lets the
// next notify decision through, not on every poll tick.
func TestG4F6_AlertRepeatsOncePerRenotifyInterval(t *testing.T) {
	nodeNow := uint32(1_000_000)
	spvChain, ratw, recorder := newOverdueActionWatcher(t, &nodeNow)
	spvChain.setLatestBlockTimestamp(time.Unix(1000+601, 0))

	poll := func() {
		t.Helper()
		if err := ratw.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	overdue := func() float64 {
		return recorder.counters[clientinfo.MetricSpvReservationActionOverdueTotal]
	}

	poll()
	if got := overdue(); got != 1 {
		t.Fatalf("expected one alert on the first decision, got %v", got)
	}

	// Still inside the renotify backoff: no new decision, no new alert.
	nodeNow += 60
	poll()
	if got := overdue(); got != 1 {
		t.Fatalf("expected no alert inside the renotify backoff, got %v", got)
	}

	// Backoff elapsed and the action is still Pending: reported again.
	nodeNow += uint32(actionTimeoutRenotifyInterval.Seconds())
	poll()
	if got := overdue(); got != 2 {
		t.Fatalf("expected a second alert after the renotify interval, got %v", got)
	}
}
