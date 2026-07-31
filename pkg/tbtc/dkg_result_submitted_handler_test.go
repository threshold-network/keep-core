package tbtc

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/internal/testutils"
)

// TestHandleDKGResultSubmittedEvent_RetriesAfterTransientFailure verifies the
// TOB-TBTCACEXT-56 fix at the callback level: when the first delivery of a
// DKGResultSubmitted event fails transiently during validation, an identical
// redelivery must reach validation again instead of being dropped as an
// already-processed duplicate. Once validation reaches a terminal state, a
// further redelivery must be ignored as a duplicate.
func TestHandleDKGResultSubmittedEvent_RetriesAfterTransientFailure(t *testing.T) {
	deduplicator := newDeduplicator()

	var resultHash DKGChainResultHash
	copy(resultHash[:], []byte{0x01, 0x02, 0x03})
	event := &DKGResultSubmittedEvent{
		Seed:        big.NewInt(100),
		ResultHash:  resultHash,
		BlockNumber: 500,
	}

	var validationCalls int
	handle := func(*DKGResultSubmittedEvent) error {
		validationCalls++
		if validationCalls == 1 {
			// Simulate a transient RPC error before the result could be
			// challenged or its approval scheduled.
			return fmt.Errorf("transient validation failure")
		}
		return nil
	}

	// First delivery: the event is claimed, validation fails, and the claim is
	// released so the event stays retryable.
	handleDKGResultSubmittedEvent(deduplicator, event, handle)

	// Redelivery of the identical event must reach validation again rather than
	// being dropped as an already-processed duplicate.
	handleDKGResultSubmittedEvent(deduplicator, event, handle)

	testutils.AssertIntsEqual(
		t,
		"validation calls after a failed delivery and its retry",
		2,
		validationCalls,
	)

	// The retry succeeded and confirmed the event, so a further redelivery must
	// be ignored as a duplicate and must not reach validation again.
	handleDKGResultSubmittedEvent(deduplicator, event, handle)

	testutils.AssertIntsEqual(
		t,
		"validation calls after the event was confirmed handled",
		2,
		validationCalls,
	)
}

// TestHandleDKGResultSubmittedEvent_IgnoresConcurrentDuplicate verifies that
// while a DKGResultSubmitted event is being handled, a concurrent duplicate
// delivery of the same event is ignored rather than starting a second handling.
func TestHandleDKGResultSubmittedEvent_IgnoresConcurrentDuplicate(t *testing.T) {
	deduplicator := newDeduplicator()

	var resultHash DKGChainResultHash
	copy(resultHash[:], []byte{0x0a, 0x0b, 0x0c})
	event := &DKGResultSubmittedEvent{
		Seed:        big.NewInt(200),
		ResultHash:  resultHash,
		BlockNumber: 700,
	}

	var nestedCalls int
	handle := func(*DKGResultSubmittedEvent) error {
		// While this handling is in progress, a duplicate delivery must be
		// dropped and must not enter the handler a second time.
		handleDKGResultSubmittedEvent(
			deduplicator,
			event,
			func(*DKGResultSubmittedEvent) error {
				nestedCalls++
				return nil
			},
		)
		return nil
	}

	handleDKGResultSubmittedEvent(deduplicator, event, handle)

	testutils.AssertIntsEqual(
		t,
		"handler calls triggered by an in-progress duplicate",
		0,
		nestedCalls,
	)
}
