package signing

import (
	"sync/atomic"

	"github.com/keep-network/keep-core/pkg/clientinfo"
)

// Interactive-signing observability counters (RFC-21 Phase 7.3, handoff §6.3).
//
// The interactive signing runner surfaces outcomes only via return values - it emits
// no logs or metrics of its own - so before the coordinated tECDSA->FROST flip there is
// no operator-visible signal for "is interactive signing working, and is anything
// failing closed?". These process-wide cumulative counters add that signal:
//
//   - success/failure track whether the interactive path, once driven, produces
//     signatures or fails on a committed attempt;
//   - coarseFallbackRefused is the FLIP-SAFETY signal: it increments whenever the
//     no-coarse-fallback mode (KEEP_CORE_FROST_INTERACTIVE_SIGNING_ONLY) terminally
//     refuses a would-be coarse/legacy signing. During a staged flip a misconfigured
//     node - interactive-only on but interactive signing not actually running - shows
//     up here instead of silently failing every signing attempt.
//
// They are process-wide rather than per-session (operators want "how many today?"),
// follow the roast_retry_metrics.go pattern, are emitted in every build, and stay at
// zero until the gated interactive path actually runs - so they are inert by default.
var (
	interactiveSigningSuccessEvents atomic.Uint64
	interactiveSigningFailureEvents atomic.Uint64
	coarseFallbackRefusedEvents     atomic.Uint64
)

// interactiveSigningMetricsApplication is the clientinfo application-label prefix; the
// registry concatenates it with each per-source name, so the final labels look like
// "frost_interactive_signing_success_total".
const interactiveSigningMetricsApplication = "frost_interactive_signing"

const (
	interactiveSigningSuccessMetricName       = "success_total"
	interactiveSigningFailureMetricName       = "failure_total"
	interactiveSigningCoarseRefusedMetricName = "coarse_fallback_refused_total"
)

// RegisterInteractiveSigningMetrics registers the cumulative interactive-signing
// counters with the supplied clientinfo registry. Operators call this from the node's
// startup sequence (alongside RegisterRoastRetryMetrics) so the counters appear in the
// Prometheus scrape. A nil registry is a no-op.
func RegisterInteractiveSigningMetrics(registry *clientinfo.Registry) {
	if registry == nil {
		return
	}
	registry.ObserveApplicationSource(
		interactiveSigningMetricsApplication,
		map[string]clientinfo.Source{
			interactiveSigningSuccessMetricName: func() float64 {
				return float64(interactiveSigningSuccessEvents.Load())
			},
			interactiveSigningFailureMetricName: func() float64 {
				return float64(interactiveSigningFailureEvents.Load())
			},
			interactiveSigningCoarseRefusedMetricName: func() float64 {
				return float64(coarseFallbackRefusedEvents.Load())
			},
		},
	)
}

// recordInteractiveSigningSuccess marks one interactive signing attempt that produced a
// signature.
func recordInteractiveSigningSuccess() {
	interactiveSigningSuccessEvents.Add(1)
}

// recordInteractiveSigningFailure marks one committed interactive signing attempt that
// failed (a drive error - not the inactive fall-through, which produces no signature
// and no error).
func recordInteractiveSigningFailure() {
	interactiveSigningFailureEvents.Add(1)
}

// recordCoarseFallbackRefused marks one terminal no-coarse-fallback refusal
// (interactive-only mode declined to sign via the retired coarse/legacy path).
func recordCoarseFallbackRefused() {
	coarseFallbackRefusedEvents.Add(1)
}

// resetInteractiveSigningMetricsForTest clears the cumulative counters. Exposed only for
// the package's own tests; not a production helper.
func resetInteractiveSigningMetricsForTest() {
	interactiveSigningSuccessEvents.Store(0)
	interactiveSigningFailureEvents.Store(0)
	coarseFallbackRefusedEvents.Store(0)
}
