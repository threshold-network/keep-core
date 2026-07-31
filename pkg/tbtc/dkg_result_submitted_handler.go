package tbtc

// handleDKGResultSubmittedEvent runs the deduplication-guarded handling of a
// DKGResultSubmitted event. It claims the event, runs handle, and then either
// confirms the deduplication entry once handling reaches a terminal state (so
// subsequent deliveries of the same event are ignored as duplicates) or releases
// the claim when handle returns an error, so a later redelivery of the same
// event can retry.
//
// handle performs the actual DKG result validation and returns a non-nil error
// when validation did not reach a terminal state (e.g. a transient RPC error
// before the result could be challenged or its approval scheduled). Releasing
// the claim on such a failure is what prevents the node from being silently
// removed from the challenger set for the rest of the challenge window: the
// deduplication entry is recorded as completed only after handling succeeds.
func handleDKGResultSubmittedEvent(
	deduplicator *deduplicator,
	event *DKGResultSubmittedEvent,
	handle func(event *DKGResultSubmittedEvent) error,
) {
	if ok := deduplicator.notifyDKGResultSubmitted(
		event.Seed,
		event.ResultHash,
		event.BlockNumber,
	); !ok {
		logger.Warnf(
			"Result with hash [0x%x] for DKG with seed [0x%x] "+
				"and starting block [%v] has been already processed",
			event.ResultHash,
			event.Seed,
			event.BlockNumber,
		)
		return
	}

	logger.Infof(
		"Result with hash [0x%x] for DKG with seed [0x%x] "+
			"submitted at block [%v]",
		event.ResultHash,
		event.Seed,
		event.BlockNumber,
	)

	if err := handle(event); err != nil {
		// Validation did not reach a terminal state (e.g. a transient
		// RPC error before the result could be challenged or its
		// approval scheduled). Release the deduplication claim so a
		// later redelivery of the same event retries validation instead
		// of being dropped as an already-processed duplicate.
		logger.Warnf(
			"DKG result validation for result with hash [0x%x] and "+
				"seed [0x%x] did not complete; allowing retry on event "+
				"redelivery: [%v]",
			event.ResultHash,
			event.Seed,
			err,
		)
		deduplicator.abortDKGResultSubmitted(
			event.Seed,
			event.ResultHash,
			event.BlockNumber,
		)
		return
	}

	deduplicator.confirmDKGResultSubmitted(
		event.Seed,
		event.ResultHash,
		event.BlockNumber,
	)
}
