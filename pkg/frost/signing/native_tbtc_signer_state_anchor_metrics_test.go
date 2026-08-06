package signing

import (
	"errors"
	"testing"
)

// poisonGlobalNativeTBTCSignerStateAnchorBarrierForTest latches the package
// barrier the way a real failure would, through the one function that is
// allowed to do it, so the test observes the same mirror production callers
// observe rather than a value it wrote itself.
func poisonGlobalNativeTBTCSignerStateAnchorBarrierForTest(cause error) {
	barrier := &globalNativeTBTCSignerStateAnchorBarrier
	barrier.mutex.Lock()
	defer barrier.mutex.Unlock()
	recordNativeTBTCSignerStateAnchorPoisoning(barrier, cause)
}

func nativeTBTCSignerStateAnchorMetricSourceForTest(
	t *testing.T,
	name string,
) float64 {
	t.Helper()
	sources := nativeTBTCSignerStateAnchorMetricSources()
	source, ok := sources[name]
	if !ok {
		t.Fatalf("metric source [%s] is not registered", name)
	}
	return source()
}

// The poisoned gauge is the whole point of the lane: a poisoned node keeps
// running and keeps attesting healthy, so the only way an operator learns it
// has stopped signing is a scrapeable signal that tracks the barrier's latched
// state.
func TestNativeTBTCSignerStateAnchorMetrics_PoisonedGaugeTracksTheBarrier(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorBarrierForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorBarrierForTest)

	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorPoisonedMetricName,
	); got != 0 {
		t.Fatalf("poisoned gauge on a healthy barrier: got %v want 0", got)
	}

	poisonGlobalNativeTBTCSignerStateAnchorBarrierForTest(
		errors.New("anchor forked its certified floor"),
	)

	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorPoisonedMetricName,
	); got != 1 {
		t.Fatalf("poisoned gauge on a poisoned barrier: got %v want 1", got)
	}
}

// The headroom gauges must report the last published pair, and the
// observations counter must distinguish "nothing has ever reported headroom"
// from "the certified windows are exhausted" - both of which read as zero on
// the gauges themselves.
func TestNativeTBTCSignerStateAnchorMetrics_HeadroomGaugesMirrorTheLastReading(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorMetricsForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorMetricsForTest)

	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorHeadroomObservationsMetricName,
	); got != 0 {
		t.Fatalf("observations before any reading: got %v want 0", got)
	}
	if _, _, observed :=
		NativeTBTCSignerStateAnchorRestartableHeadroom(); observed {
		t.Fatal("headroom reported as observed before any reading")
	}

	RecordNativeTBTCSignerStateAnchorRestartableHeadroom(4000, 3900, false, 5)
	RecordNativeTBTCSignerStateAnchorRestartableHeadroom(1234, 567, false, 5)

	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorRevisionHeadroomMetricName,
	); got != 1234 {
		t.Fatalf("revision headroom gauge: got %v want 1234", got)
	}
	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorGenerationHeadroomMetricName,
	); got != 567 {
		t.Fatalf("generation headroom gauge: got %v want 567", got)
	}
	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorHeadroomObservationsMetricName,
	); got != 2 {
		t.Fatalf("observations after two readings: got %v want 2", got)
	}

	revisions, generations, observed :=
		NativeTBTCSignerStateAnchorRestartableHeadroom()
	if !observed || revisions != 1234 || generations != 567 {
		t.Fatalf(
			"accessor: got (%d, %d, %v) want (1234, 567, true)",
			revisions,
			generations,
			observed,
		)
	}
}

// Exhausted windows must be reportable as genuinely zero, not swallowed as
// "never observed": that is precisely the reading an operator alerts on.
func TestNativeTBTCSignerStateAnchorMetrics_ExhaustedHeadroomIsObserved(
	t *testing.T,
) {
	resetNativeTBTCSignerStateAnchorMetricsForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorMetricsForTest)

	RecordNativeTBTCSignerStateAnchorRestartableHeadroom(0, 0, true, 5)

	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorHeadroomObservationsMetricName,
	); got != 1 {
		t.Fatalf("observations after an exhausted reading: got %v want 1", got)
	}
	if _, _, observed :=
		NativeTBTCSignerStateAnchorRestartableHeadroom(); !observed {
		t.Fatal("an exhausted reading must still count as observed")
	}
}

func TestNativeTBTCSignerStateAnchorMetrics_RegisteredSourceNames(t *testing.T) {
	sources := nativeTBTCSignerStateAnchorMetricSources()
	for _, name := range []string{
		nativeTBTCSignerStateAnchorPoisonedMetricName,
		nativeTBTCSignerStateAnchorRevisionHeadroomMetricName,
		nativeTBTCSignerStateAnchorGenerationHeadroomMetricName,
		nativeTBTCSignerStateAnchorHeadroomObservationsMetricName,
		nativeTBTCSignerStateAnchorRotationWarningMetricName,
	} {
		if _, ok := sources[name]; !ok {
			t.Errorf("metric source [%s] is not registered", name)
		}
	}
	if len(sources) != 5 {
		t.Errorf("registered source count: got %d want 5", len(sources))
	}
	if nativeTBTCSignerStateAnchorMetricsApplication !=
		"frost_native_signer_anchor" {
		t.Errorf(
			"application prefix changed to [%s]; scrape labels and any "+
				"operator alerts built on them move with it",
			nativeTBTCSignerStateAnchorMetricsApplication,
		)
	}
}

func TestRegisterNativeTBTCSignerStateAnchorMetrics_NilRegistryIsNoOp(
	t *testing.T,
) {
	RegisterNativeTBTCSignerStateAnchorMetrics(nil) // must not panic
}

// The rotation warning must be alertable from a scrape. It cannot be derived
// from the two headroom gauges alone, because the workload term needs the
// node's seat count - so it travels with the pair and gets its own gauge.
func TestNativeTBTCSignerStateAnchorMetrics_RotationWarningGauge(t *testing.T) {
	resetNativeTBTCSignerStateAnchorMetricsForTest()
	t.Cleanup(resetNativeTBTCSignerStateAnchorMetricsForTest)

	// Never published reads 0, exactly as the headroom gauges do; an alert
	// must qualify on the observations counter to tell the two apart.
	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorRotationWarningMetricName,
	); got != 0 {
		t.Fatalf("rotation warning before any reading: got %v want 0", got)
	}
	if warning, observed :=
		NativeTBTCSignerStateAnchorRotationWarning(); warning || observed {
		t.Fatal("rotation warning reported as observed before any reading")
	}

	RecordNativeTBTCSignerStateAnchorRestartableHeadroom(4000, 3900, false, 7)
	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorRotationWarningMetricName,
	); got != 0 {
		t.Fatalf("rotation warning while healthy: got %v want 0", got)
	}

	RecordNativeTBTCSignerStateAnchorRestartableHeadroom(200, 180, true, 7)
	if got := nativeTBTCSignerStateAnchorMetricSourceForTest(
		t,
		nativeTBTCSignerStateAnchorRotationWarningMetricName,
	); got != 1 {
		t.Fatalf("rotation warning when due: got %v want 1", got)
	}
	if warning, observed :=
		NativeTBTCSignerStateAnchorRotationWarning(); !warning || !observed {
		t.Fatalf("accessor: got (%v, %v) want (true, true)", warning, observed)
	}

	// The seat count rides along so a publisher holding fresh headroom but no
	// inventory - the anchor commit path - can still recompute the warning.
	if seats, observed :=
		NativeTBTCSignerStateAnchorLargestLocalSeatCount(); !observed ||
		seats != 7 {
		t.Fatalf("seat count: got (%d, %v) want (7, true)", seats, observed)
	}
}
