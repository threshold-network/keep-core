package tbtc

// handleWalletClosedEvent runs the deduplication-guarded handling of a
// WalletClosed event. It claims the event, runs handle, and then either confirms
// the deduplication entry once the wallet has actually been archived (so
// subsequent deliveries of the same event are ignored as duplicates) or releases
// the claim when handle returns an error, so a later redelivery of the same
// event can retry the archival.
//
// handle performs the actual wallet-closure archival and returns a non-nil error
// when archival did not complete (e.g. a transient RPC error while waiting for
// closure confirmation, or a failed archive write). Releasing the claim on such
// a failure is what prevents a closed wallet from remaining signable through
// fresh getSigningExecutor lookups until process restart: the deduplication
// entry is recorded as completed only after the wallet is removed from the local
// registry.
func handleWalletClosedEvent(
	deduplicator *deduplicator,
	event *WalletClosedEvent,
	handle func(event *WalletClosedEvent) error,
) {
	if ok := deduplicator.notifyWalletClosed(
		event.WalletID,
	); !ok {
		logger.Warnf(
			"Wallet closure for wallet with ID [0x%x] at block [%v] "+
				"has been already processed",
			event.WalletID,
			event.BlockNumber,
		)
		return
	}

	logger.Infof(
		"Wallet with ID [0x%x] has been closed at block [%v]; "+
			"proceeding with handling wallet closure",
		event.WalletID,
		event.BlockNumber,
	)

	if err := handle(event); err != nil {
		// Archival did not complete (e.g. a transient RPC error while
		// waiting for closure confirmation, or a failed archive write).
		// Release the deduplication claim so a later redelivery of the
		// same event retries the archival instead of being dropped as
		// an already-processed duplicate, which would otherwise leave
		// the closed wallet signable in the local registry.
		logger.Errorf(
			"Failure while handling wallet closure with ID [0x%x]; "+
				"allowing retry on event redelivery: [%v]",
			event.WalletID,
			err,
		)
		deduplicator.abortWalletClosed(event.WalletID)
		return
	}

	deduplicator.confirmWalletClosed(event.WalletID)
}
