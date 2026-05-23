package signing

import (
	"sync/atomic"

	"github.com/keep-network/keep-core/pkg/clientinfo"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// roastRetryEvidenceCounters holds cumulative event counts across
// the entire process lifetime. They are bumped whenever a
// metrics-emitting recorder records an event. Exposed to keep-
// core's clientinfo registry via RegisterRoastRetryMetrics, which
// operators invoke at process startup.
//
// The counters are intentionally process-wide rather than per-
// session: operators want to see "how many overflow events did
// the node observe today?" rather than "what was the count for
// the third attempt of session 0x1234?". Per-attempt detail is
// already visible in the TransitionMessage payload.
var (
	roastRetryOverflowEvents atomic.Uint64
	roastRetryRejectEvents   atomic.Uint64
	roastRetryConflictEvents atomic.Uint64
)

// Application label prefix used by RegisterRoastRetryMetrics when
// registering with clientinfo.Registry.ObserveApplicationSource.
// The registry concatenates this with each per-source name, so the
// final metric labels look like "frost_roast_retry_overflow_events_total".
const roastRetryMetricsApplication = "frost_roast_retry"

const (
	overflowEventsMetricName = "overflow_events_total"
	rejectEventsMetricName   = "reject_events_total"
	conflictEventsMetricName = "conflict_events_total"
)

// RegisterRoastRetryMetrics registers the cumulative ROAST-retry
// evidence counters with the supplied clientinfo registry.
// Operators call this from the node's startup sequence so the
// counters appear in the Prometheus scrape alongside the other
// keep-core metrics.
//
// The metrics are emitted in every build but only increment when
// the receive loops actually call into the metrics-emitting
// recorder, which happens only when the ROAST-retry registry is
// populated (i.e. the operator has opted in). In default builds
// the counters stay at zero.
func RegisterRoastRetryMetrics(registry *clientinfo.Registry) {
	if registry == nil {
		return
	}
	registry.ObserveApplicationSource(
		roastRetryMetricsApplication,
		map[string]clientinfo.Source{
			overflowEventsMetricName: func() float64 {
				return float64(roastRetryOverflowEvents.Load())
			},
			rejectEventsMetricName: func() float64 {
				return float64(roastRetryRejectEvents.Load())
			},
			conflictEventsMetricName: func() float64 {
				return float64(roastRetryConflictEvents.Load())
			},
		},
	)
}

// metricsEmittingRecorder wraps an attempt.EvidenceRecorder with
// the process-wide cumulative counters declared above. Each
// Record*-class method bumps the matching counter and then
// delegates to the inner recorder so the per-attempt bounded
// snapshot still reflects the event for the NextAttempt policy.
//
// Use newMetricsEmittingRecorder to construct; do not instantiate
// directly.
type metricsEmittingRecorder struct {
	inner attempt.EvidenceRecorder
}

func newMetricsEmittingRecorder(
	inner attempt.EvidenceRecorder,
) attempt.EvidenceRecorder {
	if inner == nil {
		return attempt.NoOpRecorder()
	}
	return &metricsEmittingRecorder{inner: inner}
}

func (m *metricsEmittingRecorder) RecordOverflow(sender group.MemberIndex) {
	roastRetryOverflowEvents.Add(1)
	m.inner.RecordOverflow(sender)
}

func (m *metricsEmittingRecorder) RecordReject(
	sender group.MemberIndex,
	reason string,
) {
	roastRetryRejectEvents.Add(1)
	m.inner.RecordReject(sender, reason)
}

func (m *metricsEmittingRecorder) RecordConflict(sender group.MemberIndex) {
	roastRetryConflictEvents.Add(1)
	m.inner.RecordConflict(sender)
}

func (m *metricsEmittingRecorder) Snapshot() attempt.Evidence {
	return m.inner.Snapshot()
}

// resetRoastRetryMetricsForTest clears the cumulative counters.
// Exposed only for the package's own tests; not a production
// helper.
func resetRoastRetryMetricsForTest() {
	roastRetryOverflowEvents.Store(0)
	roastRetryRejectEvents.Store(0)
	roastRetryConflictEvents.Store(0)
}
