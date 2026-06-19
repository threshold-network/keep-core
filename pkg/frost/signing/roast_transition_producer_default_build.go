//go:build !(frost_native && frost_roast_retry)

package signing

// roastTransitionProducerAvailable reports whether this build contains the ROAST
// transition producer (observe + exchange + elected-coordinator aggregation). That
// producer requires BOTH frost_native and frost_roast_retry; this build lacks at
// least one, so no producer exists and the function reports false.
//
// In particular a frost_roast_retry && !frost_native build has the participant
// selector and the coordinator registry but NO producer, so RoastRetryActive
// reports inactive and the selector falls back to the uniform legacy shuffle rather
// than fail-closing every retry against records that can never be created. RFC-21
// Phase 7.3 PR2b-1b (Codex P2-1).
func roastTransitionProducerAvailable() bool {
	return false
}
