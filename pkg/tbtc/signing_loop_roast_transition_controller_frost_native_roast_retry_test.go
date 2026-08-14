//go:build frost_native && frost_roast_retry

package tbtc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/frost/signing"
)

// fakeExchange records the produce-side calls the controller drives.
type fakeExchange struct {
	mu             sync.Mutex
	broadcastCalls [][32]byte
	aggregateCalls [][32]byte
	aggregated     chan struct{}
	lostSync       bool
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

func (f *fakeExchange) ConsumeLostSync() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.lostSync {
		return false
	}
	f.lostSync = false

	return true
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

// TestRoastTransitionController_ConsumeLostSyncDelegatesToExchange asserts
// ConsumeLostSync reflects the exchange's lost-sync state, charges it to exactly
// one attempt, and is false when no exchange is installed (ROAST retry inactive ->
// observe-only -> no listener -> never lost sync).
func TestRoastTransitionController_ConsumeLostSyncDelegatesToExchange(t *testing.T) {
	exchange := &fakeExchange{}
	controller := &roastTransitionControllerImpl{
		ctx:      context.Background(),
		logger:   &testutils.MockLogger{},
		exchange: exchange,
	}
	if controller.ConsumeLostSync() {
		t.Fatal("expected not lost sync initially")
	}
	exchange.mu.Lock()
	exchange.lostSync = true
	exchange.mu.Unlock()
	if !controller.ConsumeLostSync() {
		t.Fatal("expected ConsumeLostSync to reflect the exchange's lost-sync state")
	}
	// The blast-radius bound: one recorded lost-sync event is charged to one
	// attempt. A second read must be false, so the retry loop skips a single
	// attempt rather than every remaining attempt in the session.
	if controller.ConsumeLostSync() {
		t.Fatal(
			"expected lost sync to be consumed by the first read, so it costs " +
				"one attempt rather than the whole session",
		)
	}

	noExchange := &roastTransitionControllerImpl{
		ctx:    context.Background(),
		logger: &testutils.MockLogger{},
	}
	if noExchange.ConsumeLostSync() {
		t.Fatal("a controller without an exchange must never report lost sync")
	}
}

// TestRoastTransitionController_OnAttemptSucceededZeroHashIsNoOp asserts that
// without a stored attempt hash (a static-fallback observe) the success hook
// clears nothing.
func TestRoastTransitionController_OnAttemptSucceededZeroHashIsNoOp(t *testing.T) {
	signing.ResetObservedAttemptRegistryForTest()
	t.Cleanup(signing.ResetObservedAttemptRegistryForTest)

	controller := &roastTransitionControllerImpl{
		ctx:    context.Background(),
		logger: &testutils.MockLogger{},
		requestTemplate: &signing.Request{
			RoastSessionID: "ctrl-success-session",
			MemberIndex:    1,
		},
		// currentAttemptHash is the zero value.
	}
	controller.OnAttemptSucceeded() // must not panic
	if signing.ObservedAttemptStoredForTest("ctrl-success-session", 1) {
		t.Fatal("zero attempt hash must not interact with the registry")
	}
}
