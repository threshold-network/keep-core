package spv

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/go-test/deep"
)

// recordingActionTimeoutMembers is a test double for the
// tbtc.WalletMembersResolver interface. It returns the operator IDs configured
// at construction time and records the wallet PKHs it was asked to resolve.
type recordingActionTimeoutMembers struct {
	walletIDs map[[20]byte][]uint32
	calls     [][20]byte
	errByPKH  map[[20]byte]error
}

func (r *recordingActionTimeoutMembers) ResolveWalletMembers(
	walletPublicKeyHash [20]byte,
) ([]uint32, error) {
	r.calls = append(r.calls, walletPublicKeyHash)
	if err, ok := r.errByPKH[walletPublicKeyHash]; ok {
		return nil, err
	}
	return r.walletIDs[walletPublicKeyHash], nil
}

// seededReservation installs a reservation and (optionally) a list of
// action generations under spvChain for use in the action-timeout watcher
// tests. Helper reduces per-test noise. actions[0] is stored as generation
// nonce 1, actions[1] as nonce 2, etc., matching the 1-based action
// generation convention (a reservation has no generation 0; the first
// ever-requested action is nonce 1).
func seededReservation(
	t *testing.T,
	spvChain *localChain,
	key *big.Int,
	wallet [20]byte,
	actions []*tbtc.ReservationAction,
	requestNonce uint64,
) {
	t.Helper()
	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: wallet,
		RequestNonce:        requestNonce,
	})
	for i, action := range actions {
		spvChain.setReservationAction(key, uint64(i)+1, action)
	}
}

func TestReservationActionTimeoutWatcher_NotifiesTimedOutPendingAction(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC001)
	members := []uint32{11, 22, 33}

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: members},
	}
	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := spvChain.getSubmittedReservationActionTimeouts()
	if len(calls) != 1 {
		t.Fatalf("expected one timeout notification, got %d", len(calls))
	}
	if diff := deep.Equal(key, calls[0].reservationKey); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
	if diff := deep.Equal(members, calls[0].walletMembersIDs); diff != nil {
		t.Errorf("unexpected notified members: %v", diff)
	}
	// The resolver must be consulted exactly once per Check call, not per
	// nonce, because the Bridge requires the member IDs to be consistent
	// across all notifications emitted in response to a single reservation.
	if len(resolver.calls) != 1 {
		t.Errorf("expected resolver to be called once, got %d", len(resolver.calls))
	}
}

func TestReservationActionTimeoutWatcher_DoesNotNotifyBeforeTimeout(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC002)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {1, 2}},
	}
	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 10_000,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf(
			"action not yet timed out; expected zero notifications, got %d",
			len(calls),
		)
	}
}

func TestReservationActionTimeoutWatcher_IgnoresSettledOlderGeneration(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC003)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {1, 2}},
	}
	// Generation 1 (an old re-anchor, say) is already Settled; generation 2
	// is the current pending generation and is past its deadline. The
	// watcher must inspect only the current generation (RequestNonce = 2)
	// and notify for it - this is the fix for the bug where an older
	// walk-from-zero implementation stopped at the first non-pending
	// generation and never reached the real timed-out one.
	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStateSettled,
				TimeoutAt: 100,
			},
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100,
			},
		},
		2,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf(
			"current generation is pending and past deadline; expected one "+
				"notification, got %d",
			len(calls),
		)
	}
}

func TestReservationActionTimeoutWatcher_NotifiesCurrentGenerationOnly(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC004)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {7, 8, 9}},
	}
	// Generation 1 is still pending and NOT past its deadline; generation 2
	// is the current pending generation and IS past its deadline. Only
	// generation 2 (RequestNonce) is ever inspected, so exactly one
	// notification fires regardless of generation 1's state.
	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 10_000,
			},
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100,
			},
		},
		2,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf(
			"expected exactly one notification for the current generation, got %d",
			len(calls),
		)
	}
}

func TestReservationActionTimeoutWatcher_SkipsReservationWithoutWallet(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationKey(0xC005)
	// No wallet PKH assigned.

	resolver := &recordingActionTimeoutMembers{}
	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: [20]byte{},
		RequestNonce:        0,
	})

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf("zero-wallet reservation must skip, got %d notifications", len(calls))
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("resolver must not be called for zero-wallet reservation, got %d calls", len(resolver.calls))
	}
}

func TestReservationActionTimeoutWatcher_MembersResolverError(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC006)

	// In production, node.ResolveWalletMembers errors ("wallet not found") for
	// wallets the local operator is not a signing member of. The watcher must
	// treat this as an expected non-membership condition and skip cleanly without
	// returning an error or notifying.
	resolver := &recordingActionTimeoutMembers{
		errByPKH: map[[20]byte]error{wallet: errors.New("wallet not found")},
	}
	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("expected nil error on resolver non-member error, got: %v", err)
	}
	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf("no notifications should fire on resolver error, got %d", len(calls))
	}
}

func TestReservationActionTimeoutWatcher_MembersResolverEmpty(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC007)

	// An empty member set must also skip cleanly without emitting a notification.
	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {}},
	}
	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("expected nil error on empty member set, got: %v", err)
	}
	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf("no notifications should fire on empty member set, got %d", len(calls))
	}
}

func TestReservationActionTimeoutWatcher_NilResolverError(t *testing.T) {
	spvChain := newLocalChain()

	watcher := NewReservationActionTimeoutWatcher(spvChain, nil, 0)
	if err := watcher.CheckReservationActionTimeouts(reservationKey(0xC008), 5_000); err == nil {
		t.Fatal("expected error for nil resolver, got nil")
	}
}

func TestReservationActionTimeoutWatcher_NilKeyError(t *testing.T) {
	spvChain := newLocalChain()
	resolver := &recordingActionTimeoutMembers{}

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(nil, 5_000); err == nil {
		t.Fatal("expected error for nil reservation key, got nil")
	}
}

func TestReservationActionTimeoutWatcher_SkipsWalletZeroBranch(t *testing.T) {
	// Isolates the wallet-zero skip branch from the RequestNonce == 0 skip
	// branch: RequestNonce is nonzero (a real action generation exists) but
	// WalletPublicKeyHash is zero, so the reservation exists yet has no
	// wallet assigned. This must skip via the wallet-zero check, not be
	// short-circuited by the (separate) RequestNonce == 0 check that an
	// earlier version of this test file conflated by zeroing both fields
	// together.
	spvChain := newLocalChain()
	resolver := &recordingActionTimeoutMembers{}

	key := reservationKey(0xC00B)
	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: [20]byte{},
		RequestNonce:        1,
	})

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf("zero-wallet reservation must skip, got %d notifications", len(calls))
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("resolver must not be called for zero-wallet reservation, got %d calls", len(resolver.calls))
	}
}

func TestReservationActionTimeoutWatcher_NotifierErrorPropagates(t *testing.T) {
	spvChain := newLocalChain()
	errFromNotifier := errors.New("downstream")

	wallet := walletPKH()
	key := reservationKey(0xC00A)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {1, 2, 3}},
	}
	// The current generation is pending and past its deadline, but the
	// Bridge notify call fails. With only one generation ever inspected per
	// Check call, the failure must surface as an error from
	// CheckReservationActionTimeouts (not be silently swallowed), so a
	// poll-loop caller logs and retries on the next tick instead of
	// wrongly treating it as settled.
	spvChain.notifyReservationActionTimeoutErr = errFromNotifier
	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, 0)
	err := watcher.CheckReservationActionTimeouts(key, 5_000)
	if err == nil {
		t.Fatal("expected the notifier error to propagate, got nil")
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf("expected exactly one notification attempt, got %d", len(calls))
	}
}

func TestReservationActionTimeoutWatcher_NextScanRange(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	spvChain.setBlockCounter(blockCounter)

	resolver := &recordingActionTimeoutMembers{}
	watcher := NewReservationActionTimeoutWatcher(spvChain, resolver, time.Minute)

	// Case 1: First scan (lastScannedBlock == 0) and currentBlock > lookback.
	blockCounter.SetCurrentBlock(300_000)
	startBlock, currentBlock, err := watcher.nextScanRange(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedStart := uint64(300_000) - reservationActionTimeoutLookBackBlocks
	if startBlock != expectedStart {
		t.Errorf("expected start block %d, got %d", expectedStart, startBlock)
	}
	if currentBlock != 300_000 {
		t.Errorf("expected current block 300000, got %d", currentBlock)
	}

	// Case 2: First scan (lastScannedBlock == 0) and currentBlock <= lookback.
	blockCounter.SetCurrentBlock(100_000)
	startBlock, currentBlock, err = watcher.nextScanRange(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startBlock != 0 {
		t.Errorf("expected start block 0, got %d", startBlock)
	}
	if currentBlock != 100_000 {
		t.Errorf("expected current block 100000, got %d", currentBlock)
	}

	// Case 3: Subsequent scan (lastScannedBlock > 0).
	blockCounter.SetCurrentBlock(500_000)
	startBlock, currentBlock, err = watcher.nextScanRange(450_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startBlock != 450_001 {
		t.Errorf("expected start block 450001, got %d", startBlock)
	}
	if currentBlock != 500_000 {
		t.Errorf("expected current block 500000, got %d", currentBlock)
	}
}

func TestReservationActionTimeoutWatcher_RunLoop_IncrementalTracking(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet1 := walletPKH()
	members := []uint32{1, 2, 3}
	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet1: members},
	}

	pollInterval := 10 * time.Millisecond
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		resolver,
		pollInterval,
	)
	ratw.nowFn = func() uint32 { return 500 }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Tick 1: acceptance event for key1 (nonce 1).
	key1 := reservationKey(0x1001)
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      key1,
		RequestNonce:        1,
		WalletPublicKeyHash: wallet1,
		BlockNumber:         500,
	})
	seededReservation(
		t,
		spvChain,
		key1,
		wallet1,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100, // Timed out (now=500 > 100)
			},
		},
		1,
	)

	errChan := make(chan error, 1)
	go func() {
		errChan <- ratw.Run(ctx)
	}()

	// Wait for tick 1 to process key1.
	time.Sleep(50 * time.Millisecond)

	// Verify key1 was notified.
	calls := spvChain.getSubmittedReservationActionTimeouts()
	if len(calls) != 1 {
		t.Fatalf("expected 1 notification after tick 1, got %d", len(calls))
	}
	if diff := deep.Equal(key1, calls[0].reservationKey); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}

	// Tick 2: key1 is now settled (no longer pending), and a reanchor event
	// arrives for key2 (nonce 2) at block 1500.
	spvChain.setReservationAction(key1, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateSettled,
		TimeoutAt: 100,
	})

	blockCounter.SetCurrentBlock(2000)
	key2 := reservationKey(0x1002)
	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            key2,
		RequestNonce:              2,
		SourceWalletPublicKeyHash: wallet1,
		BlockNumber:               1500,
	})
	seededReservation(
		t,
		spvChain,
		key2,
		wallet1,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStateSettled,
				TimeoutAt: 100,
			},
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 200, // Timed out (now=500 > 200)
			},
		},
		2,
	)

	// Wait for tick 2 to process key2 and evict key1.
	time.Sleep(50 * time.Millisecond)

	cancel()
	if err := <-errChan; err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	// Total timeout notifications should now be 2 (key1 on tick 1, key2 on tick 2).
	calls = spvChain.getSubmittedReservationActionTimeouts()
	if len(calls) != 2 {
		t.Fatalf("expected 2 notifications after tick 2, got %d", len(calls))
	}
	if diff := deep.Equal(key2, calls[1].reservationKey); diff != nil {
		t.Errorf("unexpected second notified key: %v", diff)
	}

	// key1 should have been evicted from pendingActions because it became Settled.
	key1EventKey := actionEventKey(key1, 1)
	if _, ok := ratw.pendingActions[key1EventKey]; ok {
		t.Errorf("key1 should have been evicted from pendingActions once Settled")
	}
}

// TestReservationActionTimeoutWatcher_RunLoop_DoesNotRenotifyWhilePending
// covers the dedup guarantee TestReservationActionTimeoutWatcher_RunLoop_IncrementalTracking
// does not: it never flips the tracked action's state away from Pending,
// so eviction-on-settlement cannot be what is suppressing repeat
// notifications. Two poll windows elapse (several 10ms ticks each) while
// the action stays Pending and past its deadline; the notifier must still
// show exactly one call, and the entry must still be present in
// pendingActions (not evicted) after both windows. Both assertions depend
// on Finding A's fix: the prior unconditional
// delete-after-successful-check would also happen to leave the call count
// at one, but only because it deletes the entry outright on tick 1 - it
// would fail the "still tracked" assertion below.
func TestReservationActionTimeoutWatcher_RunLoop_DoesNotRenotifyWhilePending(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet1 := walletPKH()
	members := []uint32{1, 2, 3}
	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet1: members},
	}

	pollInterval := 10 * time.Millisecond
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		resolver,
		pollInterval,
	)
	ratw.nowFn = func() uint32 { return 500 }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	key1 := reservationKey(0x2001)
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      key1,
		RequestNonce:        1,
		WalletPublicKeyHash: wallet1,
		BlockNumber:         500,
	})
	seededReservation(
		t,
		spvChain,
		key1,
		wallet1,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100, // Timed out (now=500 > 100)
			},
		},
		1,
	)

	errChan := make(chan error, 1)
	go func() {
		errChan <- ratw.Run(ctx)
	}()

	// Tick window 1: several poll ticks fire while key1 is Pending and
	// overdue.
	time.Sleep(50 * time.Millisecond)

	calls := spvChain.getSubmittedReservationActionTimeouts()
	if len(calls) != 1 {
		t.Fatalf("expected 1 notification after tick window 1, got %d", len(calls))
	}
	if diff := deep.Equal(key1, calls[0].reservationKey); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}

	key1EventKey := actionEventKey(key1, 1)
	item, ok := ratw.pendingActions[key1EventKey]
	if !ok {
		t.Fatalf("key1 should remain tracked in pendingActions while still Pending")
	}
	if item.notifiedAt == 0 {
		t.Errorf("expected key1's pendingActions entry to record a notifiedAt timestamp")
	}

	// Tick window 2: key1's action generation is left untouched - still
	// Pending, still past TimeoutAt. Several more poll ticks fire.
	time.Sleep(50 * time.Millisecond)

	cancel()
	if err := <-errChan; err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	calls = spvChain.getSubmittedReservationActionTimeouts()
	if len(calls) != 1 {
		t.Fatalf(
			"expected notifier to still show exactly 1 call for key1 after "+
				"tick window 2 (not 2), got %d",
			len(calls),
		)
	}

	if _, ok := ratw.pendingActions[key1EventKey]; !ok {
		t.Errorf(
			"key1 should still be tracked in pendingActions after tick " +
				"window 2: it never left Pending on-chain, so only " +
				"state-driven eviction - not a delete-on-notify-success - " +
				"may remove it",
		)
	}
}

// TestReservationActionTimeoutWatcher_RunLoop_RenotifiesAfterBackoffWindow
// proves the retry path the notifiedAt/actionTimeoutRenotifyInterval
// mechanism exists for: if the first NotifyReservationActionTimeout
// transaction is dropped or reverted, the action stays Pending on-chain
// forever, and the watcher must eventually try again rather than leaving
// item.notifiedAt as permanent (but false) evidence of success. This
// drives nowFn forward past actionTimeoutRenotifyInterval between two
// poll ticks and asserts a second notification call is submitted.
func TestReservationActionTimeoutWatcher_RunLoop_RenotifiesAfterBackoffWindow(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet1 := walletPKH()
	members := []uint32{1, 2, 3}
	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet1: members},
	}

	pollInterval := 10 * time.Millisecond
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		resolver,
		pollInterval,
	)

	var currentNow uint32 = 500
	var nowMutex sync.Mutex
	ratw.nowFn = func() uint32 {
		nowMutex.Lock()
		defer nowMutex.Unlock()
		return currentNow
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	key1 := reservationKey(0x2101)
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      key1,
		RequestNonce:        1,
		WalletPublicKeyHash: wallet1,
		BlockNumber:         500,
	})
	// TimeoutAt stays fixed and far in the past relative to every "now"
	// value used below, so the action is overdue for the whole test.
	seededReservation(
		t,
		spvChain,
		key1,
		wallet1,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100,
			},
		},
		1,
	)

	errChan := make(chan error, 1)
	go func() {
		errChan <- ratw.Run(ctx)
	}()

	// First window: exactly one notification while notifiedAt is 0.
	time.Sleep(50 * time.Millisecond)
	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf("expected 1 notification before the backoff window, got %d", len(calls))
	}

	// Advance "now" past actionTimeoutRenotifyInterval. The on-chain
	// action is left untouched (still Pending, still overdue) - exactly
	// the dropped/reverted-notification scenario this mechanism exists
	// to recover from.
	nowMutex.Lock()
	currentNow = 500 + uint32(actionTimeoutRenotifyInterval.Seconds()) + 1
	nowMutex.Unlock()

	// Second window: the backoff has elapsed, so a retry notification
	// must be submitted.
	time.Sleep(50 * time.Millisecond)

	cancel()
	if err := <-errChan; err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	calls := spvChain.getSubmittedReservationActionTimeouts()
	if len(calls) != 2 {
		t.Fatalf(
			"expected a retry notification after the backoff window "+
				"elapsed (2 total calls), got %d",
			len(calls),
		)
	}
	for _, call := range calls {
		if diff := deep.Equal(key1, call.reservationKey); diff != nil {
			t.Errorf("unexpected notified key: %v", diff)
		}
	}
}

func TestReservationActionTimeoutWatcher_RunLoop_BoundedFirstScan(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	// Set current block high enough that lookback applies.
	currentBlock := uint64(500_000)
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)

	wallet1 := walletPKH()
	members := []uint32{1, 2, 3}
	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet1: members},
	}

	pollInterval := 10 * time.Millisecond
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		resolver,
		pollInterval,
	)
	ratw.nowFn = func() uint32 { return 500 }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Event 1 is old: block 100,000 (before startBlock = 500,000 - 216,000 = 284,000).
	oldKey := reservationKey(0x9001)
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      oldKey,
		RequestNonce:        1,
		WalletPublicKeyHash: wallet1,
		BlockNumber:         100_000,
	})
	seededReservation(
		t,
		spvChain,
		oldKey,
		wallet1,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100,
			},
		},
		1,
	)

	// Event 2 is within lookback: block 300,000.
	recentKey := reservationKey(0x9002)
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      recentKey,
		RequestNonce:        1,
		WalletPublicKeyHash: wallet1,
		BlockNumber:         300_000,
	})
	seededReservation(
		t,
		spvChain,
		recentKey,
		wallet1,
		[]*tbtc.ReservationAction{
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 100,
			},
		},
		1,
	)

	errChan := make(chan error, 1)
	go func() {
		errChan <- ratw.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	if err := <-errChan; err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	// Only recentKey should have been discovered and notified.
	calls := spvChain.getSubmittedReservationActionTimeouts()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 notification (recent event only), got %d", len(calls))
	}
	if diff := deep.Equal(recentKey, calls[0].reservationKey); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}

	// oldKey should not be tracked in pendingActions.
	oldEventKey := actionEventKey(oldKey, 1)
	if _, ok := ratw.pendingActions[oldEventKey]; ok {
		t.Errorf("oldKey should not have been discovered by bounded initial scan")
	}
}
