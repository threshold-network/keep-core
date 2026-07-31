package tbtc

import (
	"fmt"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
)

// TestHandleWalletClosedEvent_RetriesAfterArchivalFailure verifies the
// TOB-TBTCACEXT-57 fix at the callback level: when the first delivery of a
// WalletClosed event fails during archival, an identical redelivery must reach
// the archival handler again instead of being dropped as an already-processed
// duplicate. Once archival succeeds, a further redelivery must be ignored as a
// duplicate.
func TestHandleWalletClosedEvent_RetriesAfterArchivalFailure(t *testing.T) {
	deduplicator := newDeduplicator()

	event := &WalletClosedEvent{
		WalletID:    [32]byte{0x01, 0x02, 0x03},
		BlockNumber: 1000,
	}

	var archivalCalls int
	handle := func(*WalletClosedEvent) error {
		archivalCalls++
		if archivalCalls == 1 {
			// Simulate a transient failure while waiting for closure
			// confirmation before the wallet could be archived.
			return fmt.Errorf("transient archival failure")
		}
		return nil
	}

	// First delivery: the event is claimed, archival fails, and the claim is
	// released so the event stays retryable.
	handleWalletClosedEvent(deduplicator, event, handle)

	// Redelivery of the identical event must reach the archival handler again
	// rather than being dropped as an already-processed duplicate; otherwise the
	// closed wallet would remain signable in the local registry.
	handleWalletClosedEvent(deduplicator, event, handle)

	testutils.AssertIntsEqual(
		t,
		"archival calls after a failed delivery and its retry",
		2,
		archivalCalls,
	)

	// The retry succeeded and confirmed the event, so a further redelivery must
	// be ignored as a duplicate and must not reach the archival handler again.
	handleWalletClosedEvent(deduplicator, event, handle)

	testutils.AssertIntsEqual(
		t,
		"archival calls after the event was confirmed handled",
		2,
		archivalCalls,
	)
}

// TestHandleWalletClosedEvent_IgnoresConcurrentDuplicate verifies that while a
// WalletClosed event is being handled, a concurrent duplicate delivery of the
// same event is ignored rather than starting a second archival.
func TestHandleWalletClosedEvent_IgnoresConcurrentDuplicate(t *testing.T) {
	deduplicator := newDeduplicator()

	event := &WalletClosedEvent{
		WalletID:    [32]byte{0x0a, 0x0b, 0x0c},
		BlockNumber: 2000,
	}

	var nestedCalls int
	handle := func(*WalletClosedEvent) error {
		// While this archival is in progress, a duplicate delivery must be
		// dropped and must not enter the handler a second time.
		handleWalletClosedEvent(
			deduplicator,
			event,
			func(*WalletClosedEvent) error {
				nestedCalls++
				return nil
			},
		)
		return nil
	}

	handleWalletClosedEvent(deduplicator, event, handle)

	testutils.AssertIntsEqual(
		t,
		"handler calls triggered by an in-progress duplicate",
		0,
		nestedCalls,
	)
}
