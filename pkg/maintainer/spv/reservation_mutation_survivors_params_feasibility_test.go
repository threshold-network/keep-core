package spv

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationParamsInfeasibleRevealWindowBoundary pins the
// reveal-ahead feasibility rule at its exact boundary: a reveal-ahead
// period equal to the reservation term plus the refund safety margin is
// still feasible (the refund window and the reveal window meet, they are
// not disjoint), so it must raise neither the gauge nor the error log.
func TestReservationParamsInfeasibleRevealWindowBoundary(t *testing.T) {
	spvChain, ratw, recorder := newFeasibilityCheckWatcher(t)

	const term = 12 * 60 * 60
	spvChain.setReservationParameters(
		&tbtc.ReservationParameters{
			ReservationTermSeconds: term,
		},
	)
	spvChain.setDepositParameters(tbtc.DepositParameters{
		RevealAheadPeriod: term + reservationDepositRefundSafetyMarginSeconds,
	})

	entries := captureSpvLogs(t, func() {
		if err := ratw.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if got := recorder.Gauge(clientinfo.MetricReservationParamsInfeasible); got != 0 {
		t.Errorf(
			"expected the reveal-ahead period equal to the term plus the "+
				"safety margin to stay feasible, got gauge %v",
			got,
		)
	}
	if errors := errorEntryCount(entries, "make reserved deposits impossible"); errors != 0 {
		t.Errorf(
			"expected no infeasibility error at the exact boundary, got %d",
			errors,
		)
	}
}

// TestSetReservationParamsInfeasibleGauge_TypedNilPerformanceMetrics guards
// the feasibility gauge against the recorder shape a direct caller can
// hand in: a *clientinfo.PerformanceMetrics pointer boxed into the
// MetricsRecorder interface. The interface value is not nil while the
// pointer is, so without the guard the SetGauge call dereferences a nil
// pointer and panics inside the watcher poll pass.
func TestSetReservationParamsInfeasibleGauge_TypedNilPerformanceMetrics(
	t *testing.T,
) {
	t.Run("typed nil *clientinfo.PerformanceMetrics does not panic", func(t *testing.T) {
		var typedNilMetrics *clientinfo.PerformanceMetrics
		var recorder MetricsRecorder = typedNilMetrics
		setReservationParamsInfeasibleGauge(recorder, 1) // must not panic
	})

	t.Run("untyped nil recorder does not panic", func(t *testing.T) {
		setReservationParamsInfeasibleGauge(nil, 1)
	})

	t.Run("a live recorder still sets the gauge", func(t *testing.T) {
		recorder := &recordingMetricsRecorder{counters: make(map[string]float64)}
		setReservationParamsInfeasibleGauge(recorder, 1)
		if got := recorder.Gauge(clientinfo.MetricReservationParamsInfeasible); got != 1 {
			t.Fatalf("expected the infeasible gauge to be 1, got %v", got)
		}
	})
}
