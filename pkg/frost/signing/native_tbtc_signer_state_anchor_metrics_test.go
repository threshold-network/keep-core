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

	RecordNativeTBTCSignerStateAnchorRestartableHeadroom(4000, 3900)
	RecordNativeTBTCSignerStateAnchorRestartableHeadroom(1234, 567)

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

	RecordNativeTBTCSignerStateAnchorRestartableHeadroom(0, 0)

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
	} {
		if _, ok := sources[name]; !ok {
			t.Errorf("metric source [%s] is not registered", name)
		}
	}
	if len(sources) != 4 {
		t.Errorf("registered source count: got %d want 4", len(sources))
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
