package clientinfo

import (
	"context"
	"testing"
)

// TestReservationProofMetricsRegistered tests that the reservation proof
// counters and the liveness gauge are registered upfront so they appear in
// the metrics endpoint before any proof round runs.
//
// The SPV maintainer's reservation proof loop emits IncrementCounter calls
// for these counters and a SetGauge call for the liveness gauge. A missing
// upfront registration makes PerformanceMetrics.IncrementCounter and
// PerformanceMetrics.SetGauge silently drop the value, so an operator would
// see an always-zero liveness gauge and could not tell a working proof loop
// from a stalled one.
func TestReservationProofMetricsRegistered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := newRegistry(ctx)
	pm := NewPerformanceMetrics(ctx, registry, true)

	counters := []string{
		MetricSpvReservationProofsSubmittedTotal,
		MetricSpvReservationProofsSucceededTotal,
		MetricSpvReservationProofsFailedTotal,
		MetricSpvReservationProofsRetriedTotal,
	}

	for _, counterName := range counters {
		pm.countersMutex.RLock()
		_, exists := pm.counters[counterName]
		pm.countersMutex.RUnlock()
		if !exists {
			t.Errorf("counter %s should be registered upfront", counterName)
			continue
		}

		assertCounterExportedInRegistry(t, registry, counterName)

		if value := pm.GetCounterValue(counterName); value != 0 {
			t.Errorf("counter %s should start at 0, got %v", counterName, value)
		}

		pm.IncrementCounter(counterName, 1)
		if value := pm.GetCounterValue(counterName); value != 1 {
			t.Errorf("counter %s should increment to 1, got %v", counterName, value)
		}
	}

	// The liveness gauge must be registered too, and SetGauge must update
	// it rather than silently no-op.
	pm.gaugesMutex.RLock()
	_, exists := pm.gauges[MetricSpvReservationProofLastSuccessTimestamp]
	pm.gaugesMutex.RUnlock()
	if !exists {
		t.Errorf(
			"gauge %s should be registered upfront",
			MetricSpvReservationProofLastSuccessTimestamp,
		)
	}

	if value := pm.GetGaugeValue(MetricSpvReservationProofLastSuccessTimestamp); value != 0 {
		t.Errorf(
			"gauge %s should start at 0, got %v",
			MetricSpvReservationProofLastSuccessTimestamp,
			value,
		)
	}

	pm.SetGauge(MetricSpvReservationProofLastSuccessTimestamp, 1234)
	if value := pm.GetGaugeValue(MetricSpvReservationProofLastSuccessTimestamp); value != 1234 {
		t.Errorf(
			"gauge %s = %v, want 1234",
			MetricSpvReservationProofLastSuccessTimestamp,
			value,
		)
	}
}

// TestReservationProofGaugeNotRegisteredWhenReservationsDisabled verifies
// that the reservation proof liveness gauge is absent (not just zero) when
// the reservations feature is disabled. The counters stay registered either
// way, like the wallet-action counters, so a dashboard that queries them
// does not break when the feature is off.
func TestReservationProofGaugeNotRegisteredWhenReservationsDisabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := newRegistry(ctx)
	pm := NewPerformanceMetrics(ctx, registry, false)

	pm.gaugesMutex.RLock()
	_, exists := pm.gauges[MetricSpvReservationProofLastSuccessTimestamp]
	pm.gaugesMutex.RUnlock()

	if exists {
		t.Errorf(
			"gauge %s should not be registered when reservations are disabled",
			MetricSpvReservationProofLastSuccessTimestamp,
		)
	}
}
