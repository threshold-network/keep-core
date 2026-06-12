package clientinfo

import "sync"

// RFC-21 Annex B codifies the liveness envelope of the serial signing
// retry loop for the production wallet shape (n = 100, t = 51): an attempt
// succeeds only when the drawn t-subset contains no byzantine member, so
// the per-attempt success probability against f byzantine-but-announcing
// members is
//
//	f=1: 0.490    f=2: 0.238    f=3: 0.114    f=5: 0.025
//
// and the deployed signingAttemptsLimit = 5 rationale assumes f <= 2. The
// annex requires alerting "when observed attempt failure rates imply f >= 3
// behaviour" as the interim measure until Phase-7 t-of-included finalize
// removes the per-attempt veto. The defaults below implement that rule.
const (
	// SigningLivenessWindowSize is the number of most recent attempt
	// outcomes the rolling window holds.
	SigningLivenessWindowSize = 50

	// SigningLivenessMinimumSamples is the minimum number of recorded
	// outcomes before the implied-f alert may fire. Below this the rate
	// gauge is still exported but the alert stays at 0.
	SigningLivenessMinimumSamples = 50

	// SigningLivenessAlertSuccessRateThreshold is the rolling per-attempt
	// success rate below which the implied-f alert fires. 0.14 sits between
	// the f=2 (0.238) and f=3 (0.114) expectations. For independent
	// 50-attempt windows (normal approximation of the Annex B i.i.d.
	// model): a true f=2 fleet false-alarms on ~4% of windows, a true f=3
	// fleet alerts on ~64%, f>=4 on ~97%, and the staggered-silence
	// adversary profile (deterministic attempt failure) alerts with
	// certainty. Two caveats for tuning against the Phase 5 baseline
	// calibration worksheet: benign failures (network flakes, restarts)
	// depress the observed rate below the model, so a measured benign
	// failure rate may justify raising the threshold; and the window
	// slides per attempt, so consecutive evaluations are correlated -
	// operators should alert on a sustained value, not a single scrape.
	SigningLivenessAlertSuccessRateThreshold = 0.14
)

// Signing-attempt liveness gauges. The rate and sample-count gauges expose
// the raw rolling window so operators can apply their own thresholds; the
// alert gauge encodes the documented Annex B default (1 = observed attempt
// failure rates imply f >= 3 under the sampling model).
const (
	MetricSigningAttemptRollingSuccessRate = "signing_attempt_rolling_success_rate"
	MetricSigningAttemptRollingSampleCount = "signing_attempt_rolling_sample_count"
	MetricSigningAttemptImpliedFAlert      = "signing_attempt_implied_f_alert"
)

// SigningLivenessGaugeSetter is the minimal recorder surface the tracker
// needs.
type SigningLivenessGaugeSetter interface {
	SetGauge(name string, value float64)
}

// SigningAttemptLivenessTracker keeps a rolling window of signing-attempt
// outcomes and exports the Annex B implied-f liveness gauges. Outcomes are
// node-level aggregates: a node serving several wallets interleaves their
// attempts in one window, which keeps gauge cardinality flat at the cost of
// diluting a single compromised wallet's signal among healthy ones - if any
// served wallet's group exhibits f >= 3 behaviour, its operations still
// drag the aggregate rate toward the alert threshold.
type SigningAttemptLivenessTracker struct {
	mutex    sync.Mutex
	recorder SigningLivenessGaugeSetter

	window       []bool
	next         int
	filled       int
	successCount int

	minimumSamples int
	alertThreshold float64
}

// NewSigningAttemptLivenessTracker creates a tracker with the documented
// Annex B defaults, exporting through the given recorder.
func NewSigningAttemptLivenessTracker(
	recorder SigningLivenessGaugeSetter,
) *SigningAttemptLivenessTracker {
	return &SigningAttemptLivenessTracker{
		recorder:       recorder,
		window:         make([]bool, SigningLivenessWindowSize),
		minimumSamples: SigningLivenessMinimumSamples,
		alertThreshold: SigningLivenessAlertSuccessRateThreshold,
	}
}

// RecordAttemptOutcome records the terminal outcome of one network-wide
// signing attempt and refreshes the exported gauges.
func (salt *SigningAttemptLivenessTracker) RecordAttemptOutcome(success bool) {
	salt.mutex.Lock()

	if salt.filled == len(salt.window) {
		if salt.window[salt.next] {
			salt.successCount--
		}
	} else {
		salt.filled++
	}

	salt.window[salt.next] = success
	if success {
		salt.successCount++
	}
	salt.next = (salt.next + 1) % len(salt.window)

	samples := salt.filled
	successRate := float64(salt.successCount) / float64(samples)
	alert := samples >= salt.minimumSamples &&
		successRate < salt.alertThreshold

	salt.mutex.Unlock()

	salt.recorder.SetGauge(MetricSigningAttemptRollingSuccessRate, successRate)
	salt.recorder.SetGauge(
		MetricSigningAttemptRollingSampleCount,
		float64(samples),
	)

	alertValue := 0.0
	if alert {
		alertValue = 1.0
	}
	salt.recorder.SetGauge(MetricSigningAttemptImpliedFAlert, alertValue)
}
