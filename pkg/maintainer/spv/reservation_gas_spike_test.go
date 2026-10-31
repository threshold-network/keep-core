package spv

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestProveReservationAcceptanceActions_SubmissionFailureKeepsRecordUntilWindowCloses
// injects a persistent submission failure into the proof loop and asserts
// the documented outcome: the generation stays tracked (no auto-clear)
// while the late-proof window is open, and is evicted once the chain
// head time passes timeoutAt + termSeconds. The failure does not clear
// the on-chain action record; the action remains in its current state
// (Pending/TimedOut) until the window closes, at which point the client
// stops tracking it.
func TestProveReservationAcceptanceActions_SubmissionFailureKeepsRecordUntilWindowCloses(t *testing.T) {
	// Use the existing timedOutAcceptanceFixture which provides a
	// fully configured TimedOut acceptance generation with a
	// confirmed anchor transaction, and whose submit hook we can
	// override to inject failures.
	fixture := newTimedOutAcceptanceFixture(t, encodingP2WPKH)

	// The fixture has TimeoutAt=1000, TermSeconds=100 → late window
	// closes at chain head time 1100.
	key := reservationEventKey(fixture.reservationKey, fixture.requestNonce)

	// Inject a persistent submission failure (simulating a gas spike
	// that makes every submission attempt fail).
	attemptCount := 0
	fixture.spvChain.submitReservationAcceptanceProofHook = func(
		*tbtc.BitcoinTxInfo,
		*tbtc.BitcoinTxProof,
		*big.Int,
		uint64,
	) error {
		attemptCount++
		return context.DeadlineExceeded
	}

	// Run passes while head time is before the window close (1100).
	// The generation should remain tracked (no auto-clear).
	for i := 0; i < 5; i++ {
		fixture.spvChain.setLatestBlockTimestamp(time.Unix(1050, 0))
		cache := newProofInfoCache()
		err := proveReservationAcceptanceActions(
			fixture.state,
			fixture.config,
			fixture.spvChain,
			fixture.spvChain,
			fixture.btcChain,
			cache,
			nil,
		)
		// The pass returns an error (item error), but the generation
		// is NOT evicted.
		if err == nil {
			t.Fatalf("pass %d: expected an error from the failing submission", i)
		}
		if _, exists := fixture.state.pendingAcceptanceEvents[key]; !exists {
			t.Fatalf("pass %d: expected generation to remain tracked", i)
		}
		// Verify the failure is recorded as an item error (retryable),
		// not a chain-wide abort.
		var roundErr reservationRoundErrors
		if !errors.As(err, &roundErr) {
			t.Fatalf("pass %d: expected retryable per-item error, got %v", i, err)
		}
	}

	// At this point, 5 submission attempts were made and all failed.
	if attemptCount != 5 {
		t.Errorf("expected 5 submission attempts, got %d", attemptCount)
	}

	// Now advance the chain head time PAST the late-proof window
	// (timeoutAt + TermSeconds = 1000 + 100 = 1100).
	fixture.spvChain.setLatestBlockTimestamp(time.Unix(1101, 0))

	// The next pass should evict the generation (window closed).
	cache := newProofInfoCache()
	err := proveReservationAcceptanceActions(
		fixture.state,
		fixture.config,
		fixture.spvChain,
		fixture.spvChain,
		fixture.btcChain,
		cache,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error after window close: %v", err)
	}

	// The generation should now be evicted (no longer tracked).
	if _, exists := fixture.state.pendingAcceptanceEvents[key]; exists {
		t.Fatal("expected generation to be evicted after window close")
	}

	// The on-chain action record is still TimedOut (no auto-clear).
	// The failure did not clear the on-chain state.
	action, err := fixture.spvChain.GetReservationAction(fixture.reservationKey, fixture.requestNonce)
	if err != nil {
		t.Fatalf("failed to read on-chain action: %v", err)
	}
	if action.State != tbtc.ReservationActionStateTimedOut {
		t.Errorf("expected on-chain action to remain TimedOut (no auto-clear), got %v", action.State)
	}

	// Negative control: with a successful submission, the generation
	// is NOT evicted while the window is open, and is submitted exactly
	// once. This verifies the test isn't vacuous.
	attemptCount = 0
	fixture.spvChain.submitReservationAcceptanceProofHook = func(
		*tbtc.BitcoinTxInfo,
		*tbtc.BitcoinTxProof,
		*big.Int,
		uint64,
	) error {
		attemptCount++
		return nil
	}

	// Reset to a fresh state with a Pending action (the fixture's
	// action is TimedOut; we need a Pending action for the successful
	// submission path).
	fixture.spvChain.setReservationAction(
		fixture.reservationKey,
		fixture.requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: fixture.walletPKH,
			TimeoutAt:                 1000,
			TermSeconds:               100,
			MinAmount:                 1000,
		},
	)

	// Fresh scan state to avoid carrying over the evicted event.
	state := newReservationProofScanState()
	fixture.spvChain.setLatestBlockTimestamp(time.Unix(1050, 0))
	cache = newProofInfoCache()
	if err := proveReservationAcceptanceActions(
		state,
		fixture.config,
		fixture.spvChain,
		fixture.spvChain,
		fixture.btcChain,
		cache,
		nil,
	); err != nil {
		t.Fatalf("unexpected error with successful submission: %v", err)
	}

	// The generation should be tracked and the submission happened once.
	if attemptCount != 1 {
		t.Errorf("expected 1 successful submission, got %d", attemptCount)
	}
	if _, exists := state.pendingAcceptanceEvents[key]; !exists {
		t.Fatal("expected generation to remain tracked after successful submission")
	}
}