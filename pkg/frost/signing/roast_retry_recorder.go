package signing

import (
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// roastRetryRecorderForCollect returns the EvidenceRecorder a FROST
// receive loop should use for its current call.
//
// When the package-level ROAST-retry registry is empty (default
// build, or no caller has invoked RegisterRoastRetryCoordinator),
// the receive loops fall back to attempt.NoOpRecorder() so receive
// semantics match Phase 2 exactly: overflow events are discarded
// without observable effect.
//
// When the registry has a coordinator, the function returns a fresh
// attempt.NewBoundedRecorder(). Each call returns a NEW recorder so
// per-collect evidence does not leak across calls. The caller is
// responsible for capturing the returned recorder if it intends to
// inspect Snapshot() at end-of-collect; in Phase 4.2 we only wire
// the call sites to use the registry. PR 4.3 captures the recorder
// reference and submits its snapshot via Coordinator.RecordEvidence.
//
// This helper is intentionally not build-tagged: it delegates to
// RegisteredRoastRetryCoordinator (which IS build-tagged via the
// roast_retry_registration_* files), so the default-build path
// always sees an empty registry and returns NoOp without paying any
// coordinator-construction cost.
func roastRetryRecorderForCollect() attempt.EvidenceRecorder {
	if _, ok := RegisteredRoastRetryCoordinator(); !ok {
		return attempt.NoOpRecorder()
	}
	return attempt.NewBoundedRecorder()
}
