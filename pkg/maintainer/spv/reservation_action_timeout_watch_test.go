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

	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
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
	// Reanchor timeouts must not depend on wallet member resolution: the
	// router ignores the member-IDs parameter for this action type, so an
	// empty slice is passed unconditionally.
	if len(calls[0].walletMembersIDs) != 0 {
		t.Errorf(
			"expected empty member IDs for a Reanchor timeout, got %v",
			calls[0].walletMembersIDs,
		)
	}
	if acceptanceCalls := spvChain.getSubmittedAcceptanceTimeouts(); len(acceptanceCalls) != 0 {
		t.Errorf(
			"Reanchor timeout must not call the Acceptance entry point, got %d calls",
			len(acceptanceCalls),
		)
	}
}

// TestReservationActionTimeoutWatcher_NotifiesAcceptanceTimeoutViaDedicatedEntryPoint
// verifies that an Acceptance-type pending action is reported through
// NotifyReservationAcceptanceTimedOut, the Bridge's dedicated Acceptance
// entry point, and never through NotifyReservationActionTimeout (which is
// Reanchor-only and hard-reverts for Acceptance actions).
func TestReservationActionTimeoutWatcher_NotifiesAcceptanceTimeoutViaDedicatedEntryPoint(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC00C)

	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeAcceptance,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	acceptanceCalls := spvChain.getSubmittedAcceptanceTimeouts()
	if len(acceptanceCalls) != 1 {
		t.Fatalf("expected one acceptance-timeout notification, got %d", len(acceptanceCalls))
	}
	if diff := deep.Equal(key, acceptanceCalls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Errorf(
			"acceptance timeout must not call the Reanchor-only entry point, got %d calls",
			len(calls),
		)
	}
}

// TestReservationActionTimeoutWatcher_SkipsUnrecognizedActionType verifies
// the defensive default branch: Redemption and Dissolution are m2+ scope
// and should never reach a Pending, timed-out state in m1, but an
// unrecognized ActionType must be logged and skipped rather than causing
// an ill-formed Bridge call or a panic.
func TestReservationActionTimeoutWatcher_SkipsUnrecognizedActionType(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC00D)

	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeRedemption,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Errorf(
			"unrecognized action type must not call NotifyReservationActionTimeout, got %d calls",
			len(calls),
		)
	}
	if calls := spvChain.getSubmittedAcceptanceTimeouts(); len(calls) != 0 {
		t.Errorf(
			"unrecognized action type must not call NotifyReservationAcceptanceTimedOut, got %d calls",
			len(calls),
		)
	}
}

func TestReservationActionTimeoutWatcher_DoesNotNotifyBeforeTimeout(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC002)

	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  10_000,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
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

	// Only the current generation (RequestNonce = 2) is eligible; an
	// older non-pending generation must not stop the lookup.
	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStateSettled,
				TimeoutAt:  100,
			},
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		2,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
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
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  10_000,
			},
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		2,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
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

	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: [20]byte{},
		RequestNonce:        0,
	})

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf("zero-wallet reservation must skip, got %d notifications", len(calls))
	}
}

// TestReservationActionTimeoutWatcher_ReanchorNotifiesUnconditionally
// verifies that a Reanchor timeout is the permissionless path for a
// wallet the operator no longer locally tracks as open (e.g.
// Closed/Terminated and archived out of the wallet registry cache), so it
// must notify unconditionally with an empty member IDs slice regardless
// of any relationship between the operator and the custodying wallet -
// there is no members resolver in this watcher at all.
func TestReservationActionTimeoutWatcher_ReanchorNotifiesUnconditionally(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKHAt(0xEE)
	key := reservationKey(0xC006)

	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	calls := spvChain.getSubmittedReservationActionTimeouts()
	if len(calls) != 1 {
		t.Fatalf(
			"expected the Reanchor timeout to notify unconditionally, got %d calls",
			len(calls),
		)
	}
	if len(calls[0].walletMembersIDs) != 0 {
		t.Errorf("expected empty member IDs, got %v", calls[0].walletMembersIDs)
	}
}

func TestReservationActionTimeoutWatcher_NilKeyError(t *testing.T) {
	spvChain := newLocalChain()

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
	if err := watcher.CheckReservationActionTimeouts(nil, 5_000); err == nil {
		t.Fatal("expected error for nil reservation key, got nil")
	}
}

func TestReservationActionTimeoutWatcher_SkipsWalletZeroBranch(t *testing.T) {
	// WalletPublicKeyHash is zero while RequestNonce remains nonzero,
	// isolating the wallet-zero guard.
	spvChain := newLocalChain()

	key := reservationKey(0xC00B)
	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: [20]byte{},
		RequestNonce:        1,
	})

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf("zero-wallet reservation must skip, got %d notifications", len(calls))
	}
}

func TestReservationActionTimeoutWatcher_NotifierErrorPropagates(t *testing.T) {
	spvChain := newLocalChain()
	errFromNotifier := errors.New("downstream")

	wallet := walletPKH()
	key := reservationKey(0xC00A)

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
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
	err := watcher.CheckReservationActionTimeouts(key, 5_000)
	if err == nil {
		t.Fatal("expected the notifier error to propagate, got nil")
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf("expected exactly one notification attempt, got %d", len(calls))
	}
}

// TestReservationActionTimeoutWatcher_StalePreloadNonceMismatchFallsBackToFreshRead
// verifies that a preloadedAction fetched at a since-superseded nonce
// must not be trusted just because it is non-nil. The reservation's
// on-chain RequestNonce has advanced to 2 (a fresh, Pending, overdue
// generation) but the caller passes a stale preload captured at nonce 1
// (Settled - if wrongly trusted, it would skip instead of notifying); the
// nonce mismatch must force a fresh GetReservationAction read that finds
// the real, current generation and notifies it.
func TestReservationActionTimeoutWatcher_StalePreloadNonceMismatchFallsBackToFreshRead(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC00E)

	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStateSettled,
				TimeoutAt:  100,
			},
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		2,
	)

	stalePreload := &tbtc.ReservationAction{
		ActionType: tbtc.ReservationActionTypeReanchor,
		State:      tbtc.ReservationActionStateSettled,
		TimeoutAt:  100,
	}

	watcher := NewReservationActionTimeoutWatcher(spvChain, 0)
	notified, err := watcher.checkReservationActionTimeout(key, 5_000, stalePreload, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !notified {
		t.Fatal(
			"expected the nonce mismatch to trigger a fresh read and " +
				"notify the current generation",
		)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf("expected exactly one notification for the fresh generation, got %d", len(calls))
	}
}

// TestReservationActionTimeoutWatcher_PollPendingActions_SkipsNotifiedAtStampOnSkip
// verifies that pollPendingActions stamps item.notifiedAt only when
// checkReservationActionTimeout reports notified=true, never merely
// because it returned a nil error (such as when an unrecognized
// ActionType is skipped without calling the Bridge).
func TestReservationActionTimeoutWatcher_PollPendingActions_SkipsNotifiedAtStampOnSkip(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet1 := walletPKH()

	ratw := NewReservationActionTimeoutWatcher(spvChain, time.Minute)
	ratw.nowFn = func() uint32 { return 500 }

	key1 := reservationKey(0x3001)
	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            key1,
		RequestNonce:              1,
		SourceWalletPublicKeyHash: wallet1,
		BlockNumber:               500,
	})
	seededReservation(
		t,
		spvChain,
		key1,
		wallet1,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeRedemption,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100, // Timed out (now=500 > 100)
			},
		},
		1,
	)

	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf("unrecognized action type must never notify, got %d calls", len(calls))
	}
	if calls := spvChain.getSubmittedAcceptanceTimeouts(); len(calls) != 0 {
		t.Fatalf("unrecognized action type must never notify, got %d calls", len(calls))
	}

	key1EventKey := actionEventKey(key1, 1)
	item, ok := ratw.pendingActions[key1EventKey]
	if !ok {
		t.Fatalf("key1 should remain tracked in pendingActions (still Pending on-chain)")
	}
	if item.notifiedAt != 0 {
		t.Errorf(
			"notifiedAt must stay 0 when no notification was actually sent, got %d",
			item.notifiedAt,
		)
	}
}

// TestReservationActionTimeoutWatcher_PollPendingActions_RetainsEntryAcrossLoadFailures
// verifies that consecutive GetReservationAction poll-pass failures never
// evict a tracked entry - only an actually-observed on-chain State
// transition away from Pending may remove it, ensuring temporary RPC
// outages do not take actions out of timeout coverage.
func TestReservationActionTimeoutWatcher_PollPendingActions_RetainsEntryAcrossLoadFailures(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet1 := walletPKH()

	ratw := NewReservationActionTimeoutWatcher(spvChain, time.Minute)
	currentNow := uint32(500)
	ratw.nowFn = func() uint32 { return currentNow }

	key1 := reservationKey(0x4001)
	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            key1,
		RequestNonce:              1,
		SourceWalletPublicKeyHash: wallet1,
		BlockNumber:               500,
	})
	seededReservation(
		t,
		spvChain,
		key1,
		wallet1,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	// First tick discovers key1 via the reanchor event; its first load
	// attempt fails immediately.
	spvChain.getReservationActionErr = errors.New("transient RPC failure")
	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key1EventKey := actionEventKey(key1, 1)
	if _, ok := ratw.pendingActions[key1EventKey]; !ok {
		t.Fatalf("key1 should be tracked after discovery, even though its first load failed")
	}

	// Several more consecutive failures must never evict the entry.
	for i := range 5 {
		if err := ratw.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error on retry %d: %v", i, err)
		}
		if _, ok := ratw.pendingActions[key1EventKey]; !ok {
			t.Fatalf(
				"key1 should remain tracked despite %d consecutive load "+
					"failures; it must only be evicted once its on-chain "+
					"state is actually observed to have left Pending",
				i+1,
			)
		}
	}

	// Recovery: once reads succeed again, the entry is still there to
	// be checked and notified on the very next tick - there is no
	// backoff window to wait out.
	spvChain.getReservationActionErr = nil
	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf("expected exactly one notification after recovery, got %d", len(calls))
	}
}

func TestReservationActionTimeoutWatcher_NextScanRange(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	spvChain.setBlockCounter(blockCounter)

	watcher := NewReservationActionTimeoutWatcher(spvChain, time.Minute)

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

	pollInterval := 10 * time.Millisecond
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
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
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100, // Timed out (now=500 > 100)
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
		ActionType: tbtc.ReservationActionTypeReanchor,
		State:      tbtc.ReservationActionStateSettled,
		TimeoutAt:  100,
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
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStateSettled,
				TimeoutAt:  100,
			},
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  200, // Timed out (now=500 > 200)
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
// verifies that a successful notification remains tracked while the
// action stays Pending and suppresses duplicate calls within the
// re-notification interval.
func TestReservationActionTimeoutWatcher_RunLoop_DoesNotRenotifyWhilePending(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet1 := walletPKH()

	pollInterval := 10 * time.Millisecond
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
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
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100, // Timed out (now=500 > 100)
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

	pollInterval := 10 * time.Millisecond
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
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
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
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

	pollInterval := 10 * time.Millisecond
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
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
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
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
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
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
