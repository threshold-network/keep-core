package clientinfo

import "testing"

type gaugeCaptureRecorder struct {
	values map[string]float64
}

func newGaugeCaptureRecorder() *gaugeCaptureRecorder {
	return &gaugeCaptureRecorder{values: make(map[string]float64)}
}

func (gcr *gaugeCaptureRecorder) SetGauge(name string, value float64) {
	gcr.values[name] = value
}

func recordOutcomes(
	tracker *SigningAttemptLivenessTracker,
	successes int,
	failures int,
) {
	for i := 0; i < failures; i++ {
		tracker.RecordAttemptOutcome(false)
	}
	for i := 0; i < successes; i++ {
		tracker.RecordAttemptOutcome(true)
	}
}

func assertGauge(
	t *testing.T,
	recorder *gaugeCaptureRecorder,
	name string,
	expected float64,
) {
	t.Helper()

	actual, exists := recorder.values[name]
	if !exists {
		t.Errorf("gauge [%v] was never set", name)
		return
	}
	if actual != expected {
		t.Errorf(
			"unexpected value of gauge [%v]\nexpected: [%v]\nactual:   [%v]",
			name,
			expected,
			actual,
		)
	}
}

func TestSigningAttemptLivenessTracker_BelowMinimumSamples_NoAlert(t *testing.T) {
	recorder := newGaugeCaptureRecorder()
	tracker := NewSigningAttemptLivenessTracker(recorder)

	// One short of the minimum sample count: even a zero success rate must
	// not fire the alert yet.
	recordOutcomes(tracker, 0, SigningLivenessMinimumSamples-1)

	assertGauge(t, recorder, MetricSigningAttemptRollingSuccessRate, 0)
	assertGauge(
		t,
		recorder,
		MetricSigningAttemptRollingSampleCount,
		float64(SigningLivenessMinimumSamples-1),
	)
	assertGauge(t, recorder, MetricSigningAttemptImpliedFAlert, 0)
}

func TestSigningAttemptLivenessTracker_AlertFiresBelowThreshold(t *testing.T) {
	recorder := newGaugeCaptureRecorder()
	tracker := NewSigningAttemptLivenessTracker(recorder)

	// 6 successes of 50 = 0.12, below the 0.14 threshold with a full
	// window: the implied-f alert fires.
	recordOutcomes(tracker, 6, 44)

	assertGauge(t, recorder, MetricSigningAttemptRollingSuccessRate, 0.12)
	assertGauge(t, recorder, MetricSigningAttemptRollingSampleCount, 50)
	assertGauge(t, recorder, MetricSigningAttemptImpliedFAlert, 1)
}

func TestSigningAttemptLivenessTracker_ThresholdBoundary_NoAlert(t *testing.T) {
	recorder := newGaugeCaptureRecorder()
	tracker := NewSigningAttemptLivenessTracker(recorder)

	// 7 successes of 50 = 0.14 exactly: the alert requires the rate to be
	// strictly below the threshold, so it must not fire. (IEEE 754
	// division is correctly rounded, so 7.0/50.0 and the 0.14 literal are
	// the same float64.)
	recordOutcomes(tracker, 7, 43)

	assertGauge(t, recorder, MetricSigningAttemptRollingSuccessRate, 0.14)
	assertGauge(t, recorder, MetricSigningAttemptRollingSampleCount, 50)
	assertGauge(t, recorder, MetricSigningAttemptImpliedFAlert, 0)
}

func TestSigningAttemptLivenessTracker_WindowRollover(t *testing.T) {
	recorder := newGaugeCaptureRecorder()
	tracker := NewSigningAttemptLivenessTracker(recorder)

	// A full window of failures fires the alert...
	recordOutcomes(tracker, 0, SigningLivenessWindowSize)
	assertGauge(t, recorder, MetricSigningAttemptImpliedFAlert, 1)

	// ...and a subsequent full window of successes evicts every failure:
	// the rate recovers to 1, the sample count stays capped at the window
	// size, and the alert clears.
	recordOutcomes(tracker, SigningLivenessWindowSize, 0)
	assertGauge(t, recorder, MetricSigningAttemptRollingSuccessRate, 1)
	assertGauge(
		t,
		recorder,
		MetricSigningAttemptRollingSampleCount,
		float64(SigningLivenessWindowSize),
	)
	assertGauge(t, recorder, MetricSigningAttemptImpliedFAlert, 0)
}
