package spv

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/go-test/deep"
)

// recordingActionTimeoutMembers is a test double for the
// WalletMembersResolver interface. It returns the operator IDs configured
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

	resolver := &recordingActionTimeoutMembers{
		errByPKH: map[[20]byte]error{wallet: errors.New("oops")},
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
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err == nil {
		t.Fatal("expected error from resolver, got nil")
	}
	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
		t.Fatalf("no notifications should fire on resolver error, got %d", len(calls))
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

func TestReservationActionTimeoutWatcher_RunLoop(t *testing.T) {
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{},
	}

	pollInterval := 10 * time.Millisecond
	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		resolver,
		pollInterval,
	)
	ratw.nowFn = func() uint32 { return 100 }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Tick 1: register wallet 1, within the first scan window.
	wallet1 := [20]byte{1}
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		WalletPublicKeyHash: wallet1,
		BlockNumber:         500,
	})
	spvChain.setWallet(wallet1, &tbtc.WalletChainData{State: tbtc.StateLive})

	errChan := make(chan error, 1)
	go func() {
		errChan <- ratw.Run(ctx)
	}()

	// Wait for tick 1 to run discoverWallets and pick up wallet 1.
	time.Sleep(50 * time.Millisecond)

	// Tick 2: register wallet 2 (block number past the tick-1 cursor, so
	// the incremental scan actually picks it up) and close wallet 1, which
	// must be evicted from knownWallets on this pass.
	blockCounter.SetCurrentBlock(2000)
	wallet2 := [20]byte{2}
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		WalletPublicKeyHash: wallet2,
		BlockNumber:         1500,
	})
	spvChain.setWallet(wallet2, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.setWallet(wallet1, &tbtc.WalletChainData{State: tbtc.StateClosed})

	// Wait for tick 2 to observe the new wallet and the eviction.
	time.Sleep(50 * time.Millisecond)

	cancel()
	if err := <-errChan; err != nil {
		t.Errorf("Run returned error: %v", err)
	}

	// Assertions run only after Run has fully returned, so knownWallets is
	// no longer being mutated concurrently by the background goroutine.
	if _, ok := ratw.knownWallets[wallet1]; ok {
		t.Errorf("wallet 1 should have been evicted after being observed Closed")
	}
	if _, ok := ratw.knownWallets[wallet2]; !ok {
		t.Errorf("wallet 2 should have been discovered")
	}
}
