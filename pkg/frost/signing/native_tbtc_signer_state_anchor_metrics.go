package signing

import (
	"sync/atomic"

	"github.com/keep-network/keep-core/pkg/clientinfo"
)

// Native tBTC signer state-anchor observability.
//
// Two independent conditions stop this node from signing without stopping the
// process, and until now neither of them was scrapeable:
//
//   - the state-anchor barrier latches terminally poisoned, after which every
//     request-taking native signer call is refused until the process is
//     restarted. The node keeps running, keeps its wallet seats, and keeps
//     attesting healthy on the activation handshake; it simply never produces
//     another signature. In a permissioned FROST set several members can be in
//     that state at once while every one of them attests healthy, and the only
//     visible symptom is a wallet that stops reaching its signing threshold
//     with no node reporting a cause.
//
//   - the certified restart windows drain. Revision and generation headroom
//     shrink as anchored work commits and are only replenished by the offline
//     rotation ceremony, so an operator needs to watch them fall in order to
//     schedule that ceremony before admission starts refusing work outright.
//     Both numbers existed only as JSON on the activation-handshake endpoint,
//     whose validator requires a loopback host, so nothing off-box could read
//     them.
//
// These sources follow roast_retry_metrics.go and
// roast_interactive_signing_metrics.go: package-level state, one
// Register*Metrics helper called once from the node startup sequence, and no
// gating on whether FROST is actually active. The gating point matters and is
// the same reasoning tbtc.go already records for the ROAST counters: a source
// that is registered only once the gated path runs is a source that reports
// nothing during exactly the window an operator is trying to observe.
//
// Every source here is a plain atomic load. That is a hard requirement rather
// than a preference: clientinfo drives each source from its own goroutine on a
// fixed tick, and the accessors that could answer these questions from live
// state cannot be called from there. NativeTBTCSignerStateAnchorPoisoned reads
// the barrier's lock-free mirror for that reason, and the headroom pair is
// mirrored here rather than recomputed because computing it requires the
// anchor binding mutex - held across a remote CAS for the whole duration of a
// signing commit - plus an authenticated read of the remote anchor service. A
// scrape must not stall behind a signing operation and must not put a network
// call on a timer, so the last value observed by the paths that already
// compute it is the correct thing to publish.
var (
	// nativeTBTCSignerStateAnchorHeadroomSignal mirrors the most recent
	// restartable headroom pair. It is a single pointer rather than two
	// counters so a scrape can never pair a fresh revision count with a stale
	// generation count; the two are only meaningful together, because
	// admission refuses work as soon as EITHER dimension runs out.
	nativeTBTCSignerStateAnchorHeadroomSignal atomic.Pointer[nativeTBTCSignerStateAnchorHeadroomRecord]

	// nativeTBTCSignerStateAnchorHeadroomObservations counts how many times
	// the headroom mirror has been refreshed. It is what makes the two
	// headroom gauges interpretable: a node that has never run an anchored
	// workflow - or is not a FROST node at all - publishes zero headroom
	// simply because nothing has ever reported any, and that is
	// indistinguishable from genuinely exhausted windows unless this counter
	// is consulted. Alerts on the headroom gauges MUST require
	// headroom_observations_total > 0, and its rate is also the staleness
	// signal for the two gauges.
	nativeTBTCSignerStateAnchorHeadroomObservations atomic.Uint64

	// The two consumption counters below are the burn-rate numerator. They are
	// deliberately monotonic totals rather than levels: the headroom gauges
	// already report a level, but that level resets at every rotation, so a
	// rate cannot be taken across one. These do not reset, so
	// rate(generations_consumed_total) divided by the rate of an admitted-work
	// counter yields real burn per unit of work, across rotations.
	//
	// That ratio is the measurement the anchor's capacity planning currently
	// lacks. Fault-free burn is only bounded - between k+3 and 3k+2 durable
	// generations per signed input, because a call whose sweep prologue mutates
	// state advances more than one generation - and every window-sizing and
	// rotation-cadence figure derived so far is a code reading against that
	// range rather than an observation of it.
	nativeTBTCSignerStateAnchorGenerationsConsumed atomic.Uint64
	nativeTBTCSignerStateAnchorRevisionsConsumed   atomic.Uint64
)

// nativeTBTCSignerStateAnchorHeadroomRecord is the immutable snapshot stored
// in the mirror. Values are replaced, never mutated in place.
type nativeTBTCSignerStateAnchorHeadroomRecord struct {
	revisions   uint64
	generations uint64
	// rotationWarning travels with the headroom pair rather than being derived
	// at scrape time because it is not a function of the pair alone: the
	// workload term needs the node's largest local seat count, which only the
	// readiness inventory knows. Publishing it here is what makes the warning
	// alertable without an auditor challenge.
	rotationWarning bool
	// largestLocalSeatCount is retained so a publisher that has fresh headroom
	// but no fresh inventory - the anchor commit path - can recompute the
	// warning against the last seat count observed. Seat count only changes on
	// DKG and retirement, both of which force a full readiness reconciliation
	// that republishes it.
	largestLocalSeatCount uint64
}

// nativeTBTCSignerStateAnchorMetricsApplication is the clientinfo
// application-label prefix; the registry concatenates it with each per-source
// name, so the final labels look like "frost_native_signer_anchor_poisoned".
const nativeTBTCSignerStateAnchorMetricsApplication = "frost_native_signer_anchor"

const (
	nativeTBTCSignerStateAnchorPoisonedMetricName             = "poisoned"
	nativeTBTCSignerStateAnchorRevisionHeadroomMetricName     = "restartable_revision_headroom"
	nativeTBTCSignerStateAnchorGenerationHeadroomMetricName   = "restartable_generation_headroom"
	nativeTBTCSignerStateAnchorHeadroomObservationsMetricName = "headroom_observations_total"
	nativeTBTCSignerStateAnchorRotationWarningMetricName      = "rotation_warning"
	nativeTBTCSignerStateAnchorGenerationsConsumedMetricName  = "generations_consumed_total"
	nativeTBTCSignerStateAnchorRevisionsConsumedMetricName    = "revisions_consumed_total"
)

// RegisterNativeTBTCSignerStateAnchorMetrics registers the state-anchor health
// and capacity sources with the supplied clientinfo registry. Operators call
// this from the node's startup sequence, alongside RegisterRoastRetryMetrics
// and RegisterInteractiveSigningMetrics. A nil registry is a no-op.
//
// The poisoned source is authoritative the moment it is registered: it reads
// the barrier mirror directly, so it reports the real state on any node in any
// build without anything else having to be wired. The two headroom sources
// report whatever RecordNativeTBTCSignerStateAnchorRestartableHeadroom last
// published, and read zero until then - see the observations counter above.
func RegisterNativeTBTCSignerStateAnchorMetrics(registry *clientinfo.Registry) {
	if registry == nil {
		return
	}
	registry.ObserveApplicationSource(
		nativeTBTCSignerStateAnchorMetricsApplication,
		nativeTBTCSignerStateAnchorMetricSources(),
	)
}

// nativeTBTCSignerStateAnchorMetricSources builds the exact source set
// RegisterNativeTBTCSignerStateAnchorMetrics hands to the registry. It is
// separate so the published names and the values behind them can be asserted
// without standing up a Prometheus registry - a source that reads the wrong
// state is indistinguishable from a correct one until something reads it.
func nativeTBTCSignerStateAnchorMetricSources() map[string]clientinfo.Source {
	return map[string]clientinfo.Source{
		nativeTBTCSignerStateAnchorPoisonedMetricName: func() float64 {
			if NativeTBTCSignerStateAnchorPoisoned() != nil {
				return 1
			}
			return 0
		},
		nativeTBTCSignerStateAnchorRevisionHeadroomMetricName: func() float64 {
			record := nativeTBTCSignerStateAnchorHeadroomSignal.Load()
			if record == nil {
				return 0
			}
			return float64(record.revisions)
		},
		nativeTBTCSignerStateAnchorGenerationHeadroomMetricName: func() float64 {
			record := nativeTBTCSignerStateAnchorHeadroomSignal.Load()
			if record == nil {
				return 0
			}
			return float64(record.generations)
		},
		nativeTBTCSignerStateAnchorHeadroomObservationsMetricName: func() float64 {
			return float64(
				nativeTBTCSignerStateAnchorHeadroomObservations.Load(),
			)
		},
		// Reports 0 both when no rotation is due and when nothing has ever
		// published, exactly as the headroom gauges do. Alerts on this gauge
		// carry the same obligation: require headroom_observations_total > 0,
		// or a node that has never run an anchored workflow reads the same as
		// a healthy one.
		nativeTBTCSignerStateAnchorRotationWarningMetricName: func() float64 {
			record := nativeTBTCSignerStateAnchorHeadroomSignal.Load()
			if record == nil || !record.rotationWarning {
				return 0
			}
			return 1
		},
		nativeTBTCSignerStateAnchorGenerationsConsumedMetricName: func() float64 {
			return float64(
				nativeTBTCSignerStateAnchorGenerationsConsumed.Load(),
			)
		},
		nativeTBTCSignerStateAnchorRevisionsConsumedMetricName: func() float64 {
			return float64(
				nativeTBTCSignerStateAnchorRevisionsConsumed.Load(),
			)
		},
	}
}

// recordNativeTBTCSignerStateAnchorConsumption adds one anchored operation's
// durable cost to the burn-rate totals: the generations its native call
// advanced, and the revision its compare-and-swap spent.
//
// Called only after the acknowledgement has been validated and durably read
// back, so it counts work the anchor actually witnessed rather than work
// attempted. An operation that advanced no generation spends no revision
// either - the barrier skips the CAS when the tip is unchanged - and is
// counted as neither.
func recordNativeTBTCSignerStateAnchorConsumption(
	generations uint64,
	revisions uint64,
) {
	if generations > 0 {
		nativeTBTCSignerStateAnchorGenerationsConsumed.Add(generations)
	}
	if revisions > 0 {
		nativeTBTCSignerStateAnchorRevisionsConsumed.Add(revisions)
	}
}

// NativeTBTCSignerStateAnchorConsumption returns the burn-rate totals. Exposed
// so a caller can assert on what the counters will report without reaching
// into package state.
func NativeTBTCSignerStateAnchorConsumption() (
	generations uint64,
	revisions uint64,
) {
	return nativeTBTCSignerStateAnchorGenerationsConsumed.Load(),
		nativeTBTCSignerStateAnchorRevisionsConsumed.Load()
}

// RecordNativeTBTCSignerStateAnchorRestartableHeadroom publishes the
// restartable revision and generation headroom last computed by a caller that
// already had to compute it. It is deliberately a dumb setter: it performs no
// I/O, takes no lock, and never fails, so it is safe to call from inside a
// path that is holding the anchor or admission mutex, which is exactly where
// these numbers become available.
//
// Callers must pass an authenticated pair. This is an observability mirror and
// nothing reads it back to make a decision, so a wrong value here misleads an
// operator but cannot admit work; even so, publishing an unauthenticated
// reading would defeat the purpose of the gauge.
func RecordNativeTBTCSignerStateAnchorRestartableHeadroom(
	revisions uint64,
	generations uint64,
	rotationWarning bool,
	largestLocalSeatCount uint64,
) {
	nativeTBTCSignerStateAnchorHeadroomSignal.Store(
		&nativeTBTCSignerStateAnchorHeadroomRecord{
			revisions:             revisions,
			generations:           generations,
			rotationWarning:       rotationWarning,
			largestLocalSeatCount: largestLocalSeatCount,
		},
	)
	nativeTBTCSignerStateAnchorHeadroomObservations.Add(1)
}

// NativeTBTCSignerStateAnchorLargestLocalSeatCount returns the seat count that
// travelled with the last published headroom, and whether anything has
// published one.
//
// It exists for the anchor commit path, which has authenticated headroom on
// every successful compare-and-swap but no inventory of its own. Without this
// the commit path could publish headroom but not the warning derived from it,
// which is how the gauge ends up refreshing only on readiness reconciliation -
// the exact staleness this mirror is meant to remove.
func NativeTBTCSignerStateAnchorLargestLocalSeatCount() (
	seats uint64,
	observed bool,
) {
	record := nativeTBTCSignerStateAnchorHeadroomSignal.Load()
	if record == nil {
		return 0, false
	}
	return record.largestLocalSeatCount, true
}

// NativeTBTCSignerStateAnchorRotationWarning returns the mirrored rotation
// warning and whether anything has ever published one.
func NativeTBTCSignerStateAnchorRotationWarning() (warning bool, observed bool) {
	record := nativeTBTCSignerStateAnchorHeadroomSignal.Load()
	if record == nil {
		return false, false
	}
	return record.rotationWarning, true
}

// NativeTBTCSignerStateAnchorRestartableHeadroom returns the mirrored headroom
// pair and whether anything has ever published one. Exposed so a caller can
// assert on what the gauges will report without reaching into package state.
func NativeTBTCSignerStateAnchorRestartableHeadroom() (
	revisions uint64,
	generations uint64,
	observed bool,
) {
	record := nativeTBTCSignerStateAnchorHeadroomSignal.Load()
	if record == nil {
		return 0, 0, false
	}
	return record.revisions, record.generations, true
}

// resetNativeTBTCSignerStateAnchorMetricsForTest clears the headroom mirror
// and its observation counter. Exposed only for the package's own tests; not a
// production helper. The poisoned mirror is owned by the barrier and is not
// touched here.
func resetNativeTBTCSignerStateAnchorMetricsForTest() {
	nativeTBTCSignerStateAnchorHeadroomSignal.Store(nil)
	nativeTBTCSignerStateAnchorHeadroomObservations.Store(0)
}
