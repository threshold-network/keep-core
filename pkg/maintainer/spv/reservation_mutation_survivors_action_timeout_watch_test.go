package spv

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationActionTimeoutWatcher_MutationSurvivors_ZeroIntervalRejected
// pins the poll-interval guard: a zero interval must be rejected before
// time.NewTicker is reached, because NewTicker(0) panics and would take
// the watcher goroutine (and, through the wiring restart loop, the whole
// reservation watcher set) down instead of reporting a configuration
// error.
func TestReservationActionTimeoutWatcher_MutationSurvivors_ZeroIntervalRejected(t *testing.T) {
	ratw := NewReservationActionTimeoutWatcher(
		newLocalChain(),
		0,
		common.Address{},
		nil,
		0,
		nil,
	)

	if err := ratw.Run(context.Background()); err == nil {
		t.Fatal("expected Run to reject a zero poll interval")
	}
}

// TestReservationActionTimeoutWatcher_MutationSurvivors_RenotifyIntervalBoundary
// pins the renotify backoff comparison at its exact boundary: a pending
// action whose renotify interval has elapsed down to the second must be
// retried, because the interval is a minimum backoff, not a strict
// threshold that needs one extra second.
func TestReservationActionTimeoutWatcher_MutationSurvivors_RenotifyIntervalBoundary(
	t *testing.T,
) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	key := reservationKey(0x5101)
	seededReservation(
		t,
		spvChain,
		key,
		walletPKH(),
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		time.Minute,
		common.Address{},
		nil,
		0,
		nil,
	)

	const notifiedAt uint32 = 5_000
	ratw.pendingActions[actionEventKey(key, 1)] = &pendingAction{
		reservationKey: key,
		requestNonce:   1,
		notifiedAt:     notifiedAt,
	}

	interval := uint32(actionTimeoutRenotifyInterval.Seconds())
	ratw.nowFn = func() uint32 { return notifiedAt + interval }
	spvChain.setLatestBlockTimestamp(time.Unix(100, 0))

	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf(
			"expected a single retry notification once the renotify interval "+
				"had exactly elapsed, got %d submissions",
			len(calls),
		)
	}
}

// TestReservationActionTimeoutWatcher_MutationSurvivors_FirstAttemptStaggered
// pins the stagger gate to the first notify attempt: a generation that has
// never been notified must have its deadline shifted by the deterministic
// per-operator stagger offset, so at the bare timeoutAt it is still held
// back. Applying the offset only to retries would make every operator's
// first attempt collide in the same tick, which is the collision the
// stagger exists to prevent.
func TestReservationActionTimeoutWatcher_MutationSurvivors_FirstAttemptStaggered(
	t *testing.T,
) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	operatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000a1")

	// Pick a reservation key whose stagger offset is non-zero for this
	// operator, so the first-attempt deadline is strictly past timeoutAt.
	// The search is bounded: a mutation that disables staggering would
	// otherwise spin here forever instead of failing the test.
	var key = reservationKey(0x5102)
	var offset uint32
	var found bool
	for i := uint64(0); i < 64 && !found; i++ {
		candidate := reservationKey(0x5102 + i)
		candidateOffset := reservationOperatorStaggerOffset(
			operatorAddress,
			actionEventKey(candidate, 1),
			actionTimeoutRenotifyInterval,
		)
		if candidateOffset != 0 {
			key, offset, found = candidate, candidateOffset, true
		}
	}
	if !found {
		t.Fatal("expected a non-zero stagger offset within the first 64 keys")
	}

	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            key,
		RequestNonce:              1,
		SourceWalletPublicKeyHash: walletPKH(),
		BlockNumber:               500,
	})
	seededReservation(
		t,
		spvChain,
		key,
		walletPKH(),
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		time.Minute,
		operatorAddress,
		nil,
		0,
		nil,
	)
	ratw.nowFn = func() uint32 { return 1_000 }
	// The chain head sits exactly on the un-staggered deadline; the
	// stagger offset must still hold the first attempt back.
	spvChain.setLatestBlockTimestamp(time.Unix(100, 0))

	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf(
			"expected the first notify attempt to be staggered past the bare "+
				"timeoutAt (offset %ds), got %d submissions",
			offset,
			len(calls),
		)
	}
}

// TestReservationActionTimeoutWatcher_MutationSurvivors_NotifiesAtStaggeredDeadline
// pins the deadline comparison in the poll pass as inclusive: an already
// notified generation (no stagger on retries) whose timeoutAt equals the
// chain head must be notified, mirroring the contract's
// block.timestamp >= timeoutAt gate.
func TestReservationActionTimeoutWatcher_MutationSurvivors_NotifiesAtStaggeredDeadline(
	t *testing.T,
) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	key := reservationKey(0x5103)
	seededReservation(
		t,
		spvChain,
		key,
		walletPKH(),
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		time.Minute,
		common.Address{},
		nil,
		0,
		nil,
	)

	const notifiedAt uint32 = 1_000
	ratw.pendingActions[actionEventKey(key, 1)] = &pendingAction{
		reservationKey: key,
		requestNonce:   1,
		notifiedAt:     notifiedAt,
	}
	// Well past the renotify backoff, so this tick reaches the deadline
	// comparison; notifiedAt is set, so no stagger offset is applied.
	ratw.nowFn = func() uint32 {
		return notifiedAt + uint32(actionTimeoutRenotifyInterval.Seconds()) + 1
	}
	// Chain head exactly on timeoutAt.
	spvChain.setLatestBlockTimestamp(time.Unix(100, 0))

	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf(
			"expected a notification at the exact timeoutAt in chain time, "+
				"got %d submissions",
			len(calls),
		)
	}
}

// TestReservationActionTimeoutWatcher_MutationSurvivors_NilPreloadReadsAction
// pins the preload guard: a caller with no preloaded action (nil) must fall
// through to the chain read. Treating a nil preload as reusable would
// dereference it and panic the poll pass.
func TestReservationActionTimeoutWatcher_MutationSurvivors_NilPreloadReadsAction(
	t *testing.T,
) {
	spvChain := newLocalChain()

	key := reservationKey(0x5104)
	seededReservation(
		t,
		spvChain,
		key,
		walletPKH(),
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeAcceptance,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		time.Minute,
		common.Address{},
		nil,
		0,
		nil,
	)

	notified, err := ratw.checkReservationActionTimeout(key, 100, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error for a nil preload: %v", err)
	}
	if !notified {
		t.Fatal("expected a nil preload to fall back to the chain read and notify")
	}
	if calls := spvChain.getSubmittedAcceptanceTimeouts(); len(calls) != 1 {
		t.Fatalf("expected exactly one acceptance timeout notification, got %d", len(calls))
	}
}
