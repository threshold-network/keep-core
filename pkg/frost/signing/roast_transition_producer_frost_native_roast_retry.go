//go:build frost_native && frost_roast_retry

package signing

// roastTransitionProducerAvailable reports whether this build contains the ROAST
// transition PRODUCER -- the observe step, the transition exchange, and the
// elected-coordinator aggregation that create transition records. The producer
// lives behind frost_native && frost_roast_retry, so it is present here.
//
// RoastRetryActive and the participant selector gate on it: without a producer no
// transition record can ever exist, so treating ROAST retry as active would
// fail-close every retry (roastAttemptNumber > 0 finds no record) instead of using
// the uniform legacy shuffle. RFC-21 Phase 7.3 PR2b-1b (Codex P2-1).
func roastTransitionProducerAvailable() bool {
	return true
}
