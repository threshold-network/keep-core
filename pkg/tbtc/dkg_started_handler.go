package tbtc

// handleDKGStartedEvent runs the deduplication-guarded handling of a DKGStarted
// event. It claims the event, runs handle, and then either confirms the
// deduplication entry once handling reaches a terminal state (so subsequent
// deliveries of the same event are ignored as duplicates) or releases the claim
// when handle returns an error, so a later redelivery of the same event can
// retry.
//
// handle performs the local DKG-start handling and returns a nil error once the
// event has been terminally handled (the local DKG join was dispatched, or the
// event turned out to be unconfirmed) and a non-nil error when a transient early
// return (e.g. a block-confirmation wait failure, a DKG-state check error, or a
// past-event lookup error) prevented handling from completing. Releasing the
// claim on such a failure is what lets a redelivered event retry so the operator
// still joins the new group: the deduplication entry is recorded as completed
// only after handling succeeds.
func handleDKGStartedEvent(
	deduplicator *deduplicator,
	event *DKGStartedEvent,
	handle func(event *DKGStartedEvent) error,
) {
	if ok := deduplicator.notifyDKGStarted(
		event.Seed,
	); !ok {
		logger.Infof(
			"DKG started event with seed [0x%x] has been "+
				"already processed",
			event.Seed,
		)
		return
	}

	if err := handle(event); err != nil {
		// Handling did not reach a terminal state (e.g. a block-confirmation
		// wait failure, a DKG-state check error, or a past-event lookup error
		// before the local DKG join could be dispatched). Release the
		// deduplication claim so a later redelivery of the same event retries
		// the handling instead of being dropped as an already-processed
		// duplicate, which would otherwise leave the operator out of the new
		// group.
		logger.Warnf(
			"handling of DKG started event with seed [0x%x] did "+
				"not complete; allowing retry on event redelivery: [%v]",
			event.Seed,
			err,
		)
		deduplicator.abortDKGStarted(event.Seed)
		return
	}

	deduplicator.confirmDKGStarted(event.Seed)
}
