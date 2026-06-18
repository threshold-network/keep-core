//go:build frost_native && frost_roast_retry

package tbtc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
)

// fakeExchange records the produce-side calls the controller drives.
type fakeExchange struct {
	mu             sync.Mutex
	broadcastCalls [][32]byte
	aggregateCalls [][32]byte
	aggregated     chan struct{}
}

func (f *fakeExchange) BroadcastForcedSnapshot(hash [32]byte) {
	f.mu.Lock()
	f.broadcastCalls = append(f.broadcastCalls, hash)
	f.mu.Unlock()
}

func (f *fakeExchange) AggregateAndBroadcast(hash [32]byte) {
	f.mu.Lock()
	f.aggregateCalls = append(f.aggregateCalls, hash)
	f.mu.Unlock()
	if f.aggregated != nil {
		f.aggregated <- struct{}{}
	}
}

func (f *fakeExchange) broadcasts() [][32]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][32]byte(nil), f.broadcastCalls...)
}

func (f *fakeExchange) aggregates() [][32]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][32]byte(nil), f.aggregateCalls...)
}

// TestRoastTransitionController_OnAttemptFailedDrivesExchange asserts a failed
// attempt synchronously broadcasts the forced snapshot and, after the snapshot
// deadline, aggregates + broadcasts the bundle -- both for the attempt's hash.
func TestRoastTransitionController_OnAttemptFailedDrivesExchange(t *testing.T) {
	hash := [32]byte{0x01, 0x02, 0x03}
	exchange := &fakeExchange{aggregated: make(chan struct{}, 1)}

	deadlineReached := make(chan uint64, 1)
	controller := &roastTransitionControllerImpl{
		ctx:      context.Background(),
		logger:   &testutils.MockLogger{},
		exchange: exchange,
		waitForBlockFn: func(_ context.Context, block uint64) error {
			deadlineReached <- block
			return nil
		},
		currentAttemptHash: hash,
	}

	controller.OnAttemptFailed(1, 100)

	// The forced snapshot broadcast is synchronous.
	if got := exchange.broadcasts(); len(got) != 1 || got[0] != hash {
		t.Fatalf("expected one forced-snapshot broadcast for the attempt hash, got %v", got)
	}

	// The deadline goroutine waits on the snapshot deadline (timeout + cooldown),
	// then aggregates.
	select {
	case block := <-deadlineReached:
		if block != 100+signingAttemptCoolDownBlocks {
			t.Fatalf(
				"expected aggregation to wait until snapshot deadline %d, got %d",
				100+signingAttemptCoolDownBlocks, block,
			)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the deadline goroutine to wait for a block")
	}

	select {
	case <-exchange.aggregated:
	case <-time.After(2 * time.Second):
		t.Fatal("expected AggregateAndBroadcast to run after the deadline")
	}
	if got := exchange.aggregates(); len(got) != 1 || got[0] != hash {
		t.Fatalf("expected one aggregate for the attempt hash, got %v", got)
	}
}

// TestRoastTransitionController_OnAttemptFailedNoExchangeIsSafe asserts a
// controller with no exchange (ROAST retry inactive) ignores a failed attempt.
func TestRoastTransitionController_OnAttemptFailedNoExchangeIsSafe(t *testing.T) {
	controller := &roastTransitionControllerImpl{
		ctx:                context.Background(),
		logger:             &testutils.MockLogger{},
		currentAttemptHash: [32]byte{0x01},
	}
	controller.OnAttemptFailed(1, 100) // must not panic
}

// TestRoastTransitionController_OnAttemptFailedZeroHashIsNoOp asserts that
// without a stored attempt hash (a static-fallback observe) the exchange is not
// driven.
func TestRoastTransitionController_OnAttemptFailedZeroHashIsNoOp(t *testing.T) {
	exchange := &fakeExchange{aggregated: make(chan struct{}, 1)}
	controller := &roastTransitionControllerImpl{
		ctx:            context.Background(),
		logger:         &testutils.MockLogger{},
		exchange:       exchange,
		waitForBlockFn: func(context.Context, uint64) error { return nil },
		// currentAttemptHash is the zero value.
	}
	controller.OnAttemptFailed(1, 100)

	if got := exchange.broadcasts(); len(got) != 0 {
		t.Fatalf("zero attempt hash must not broadcast a snapshot, got %v", got)
	}
	// Give any erroneously-spawned goroutine a chance to fire.
	select {
	case <-exchange.aggregated:
		t.Fatal("zero attempt hash must not aggregate")
	case <-time.After(100 * time.Millisecond):
	}
}
