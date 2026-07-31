package tbtc

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
)

// TestHandleDKGStartedEvent_RetriesAfterTransientFailure verifies the
// TOB-TBTCACEXT-72 fix at the callback level: when the first delivery of a
// DKGStarted event fails after the deduplication claim (e.g. a block-confirmation
// wait failure before the local DKG join is dispatched), an identical redelivery
// must reach the handling path again instead of being dropped as an
// already-processed duplicate. Once handling reaches a terminal state, a further
// redelivery must be ignored as a duplicate.
func TestHandleDKGStartedEvent_RetriesAfterTransientFailure(t *testing.T) {
	deduplicator := newDeduplicator()

	event := &DKGStartedEvent{
		Seed:        big.NewInt(100),
		BlockNumber: 500,
	}

	var handlingCalls int
	handle := func(*DKGStartedEvent) error {
		handlingCalls++
		if handlingCalls == 1 {
			// Simulate a transient early return (e.g. a failed block-height
			// confirmation) before the local DKG join could be dispatched.
			return fmt.Errorf("transient DKG started handling failure")
		}
		return nil
	}

	// First delivery: the event is claimed, handling fails, and the claim is
	// released so the event stays retryable.
	handleDKGStartedEvent(deduplicator, event, handle)

	// Redelivery of the identical event must reach the handling path again
	// rather than being dropped as an already-processed duplicate; otherwise the
	// operator would never join the new group.
	handleDKGStartedEvent(deduplicator, event, handle)

	testutils.AssertIntsEqual(
		t,
		"handling calls after a failed delivery and its retry",
		2,
		handlingCalls,
	)

	// The retry succeeded and confirmed the event, so a further redelivery must
	// be ignored as a duplicate and must not reach the handling path again.
	handleDKGStartedEvent(deduplicator, event, handle)

	testutils.AssertIntsEqual(
		t,
		"handling calls after the event was confirmed handled",
		2,
		handlingCalls,
	)
}

// TestHandleDKGStartedEvent_IgnoresConcurrentDuplicate verifies that while a
// DKGStarted event is being handled, a concurrent duplicate delivery of the same
// event is ignored rather than starting a second handling.
func TestHandleDKGStartedEvent_IgnoresConcurrentDuplicate(t *testing.T) {
	deduplicator := newDeduplicator()

	event := &DKGStartedEvent{
		Seed:        big.NewInt(200),
		BlockNumber: 700,
	}

	var nestedCalls int
	handle := func(*DKGStartedEvent) error {
		// While this handling is in progress, a duplicate delivery must be
		// dropped and must not enter the handler a second time.
		handleDKGStartedEvent(
			deduplicator,
			event,
			func(*DKGStartedEvent) error {
				nestedCalls++
				return nil
			},
		)
		return nil
	}

	handleDKGStartedEvent(deduplicator, event, handle)

	testutils.AssertIntsEqual(
		t,
		"handler calls triggered by an in-progress duplicate",
		0,
		nestedCalls,
	)
}
