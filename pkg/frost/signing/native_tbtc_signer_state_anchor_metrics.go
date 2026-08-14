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
)

// nativeTBTCSignerStateAnchorHeadroomRecord is the immutable snapshot stored
// in the mirror. Values are replaced, never mutated in place.
type nativeTBTCSignerStateAnchorHeadroomRecord struct {
	revisions   uint64
	generations uint64
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
	}
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
) {
	nativeTBTCSignerStateAnchorHeadroomSignal.Store(
		&nativeTBTCSignerStateAnchorHeadroomRecord{
			revisions:   revisions,
			generations: generations,
		},
	)
	nativeTBTCSignerStateAnchorHeadroomObservations.Add(1)
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
