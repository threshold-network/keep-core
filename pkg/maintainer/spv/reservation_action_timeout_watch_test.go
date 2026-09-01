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

// recordingActionTimeoutNotifier captures every
// NotifyReservationActionTimeout call for assertion in tests.
type recordingActionTimeoutNotifier struct {
	calls []*submittedReservationActionTimeout
	err   error
}

func (r *recordingActionTimeoutNotifier) NotifyReservationActionTimeout(
	reservationKey *big.Int,
	walletMembersIDs []uint32,
) error {
	r.calls = append(r.calls, &submittedReservationActionTimeout{
		reservationKey:   reservationKey,
		walletMembersIDs: walletMembersIDs,
	})
	return r.err
}

// seededReservation installs a reservation and (optionally) a list of
// action generations under spvChain for use in the action-timeout watcher
// tests. Helper reduces per-test noise.
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
	for nonce, action := range actions {
		spvChain.setReservationAction(key, uint64(nonce), action)
	}
}

func TestReservationActionTimeoutWatcher_NotifiesTimedOutPendingAction(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}

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
		0,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("expected one timeout notification, got %d", len(notifier.calls))
	}
	if diff := deep.Equal(key, notifier.calls[0].reservationKey); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
	if diff := deep.Equal(members, notifier.calls[0].walletMembersIDs); diff != nil {
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
	notifier := &recordingActionTimeoutNotifier{}

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
		0,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf(
			"action not yet timed out; expected zero notifications, got %d",
			len(notifier.calls),
		)
	}
}

func TestReservationActionTimeoutWatcher_StopsAtFirstNonPending(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}

	wallet := walletPKH()
	key := reservationKey(0xC003)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {1, 2}},
	}
	// Nonce 0 is settled, nonce 1 is the latest pending and past deadline.
	// The walker must stop at nonce 0 without notifying.
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
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf(
			"first action is settled; walker must stop, got %d notifications",
			len(notifier.calls),
		)
	}
}

func TestReservationActionTimeoutWatcher_NotifiesLatestNonce(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}

	wallet := walletPKH()
	key := reservationKey(0xC004)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {7, 8, 9}},
	}
	// Nonce 0 pending past its deadline, nonce 1 pending past its deadline.
	// Both must be notified (defensive walker continues past the first).
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
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 200,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 2 {
		t.Fatalf(
			"expected two notifications (nonce 0 and 1), got %d",
			len(notifier.calls),
		)
	}
}

func TestReservationActionTimeoutWatcher_SkipsReservationWithoutWallet(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}

	key := reservationKey(0xC005)
	// No wallet PKH assigned.

	resolver := &recordingActionTimeoutMembers{}
	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: [20]byte{},
		RequestNonce:        0,
	})

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("zero-wallet reservation must skip, got %d notifications", len(notifier.calls))
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("resolver must not be called for zero-wallet reservation, got %d calls", len(resolver.calls))
	}
}

func TestReservationActionTimeoutWatcher_MembersResolverError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}

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
		0,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err == nil {
		t.Fatal("expected error from resolver, got nil")
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("no notifications should fire on resolver error, got %d", len(notifier.calls))
	}
}

func TestReservationActionTimeoutWatcher_NilNotifierError(t *testing.T) {
	spvChain := newLocalChain()
	resolver := &recordingActionTimeoutMembers{}

	watcher := NewReservationActionTimeoutWatcher(spvChain, nil, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(reservationKey(0xC007), 5_000); err == nil {
		t.Fatal("expected error for nil notifier, got nil")
	}
}

func TestReservationActionTimeoutWatcher_NilResolverError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, nil, 0)
	if err := watcher.CheckReservationActionTimeouts(reservationKey(0xC008), 5_000); err == nil {
		t.Fatal("expected error for nil resolver, got nil")
	}
}

func TestReservationActionTimeoutWatcher_NilKeyError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}
	resolver := &recordingActionTimeoutMembers{}

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(nil, 5_000); err == nil {
		t.Fatal("expected error for nil reservation key, got nil")
	}
}

func TestReservationActionTimeoutWatcher_NotifierFuncAdapter(t *testing.T) {
	var captured []*submittedReservationActionTimeout
	notifier := ReservationActionTimeoutNotifierFunc(func(
		reservationKey *big.Int,
		walletMembersIDs []uint32,
	) error {
		captured = append(captured, &submittedReservationActionTimeout{
			reservationKey:   reservationKey,
			walletMembersIDs: walletMembersIDs,
		})
		return nil
	})

	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xC009)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {42}},
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
		0,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(captured) != 1 {
		t.Fatalf("expected one captured notification, got %d", len(captured))
	}
}

func TestReservationActionTimeoutWatcher_NotifiesOncePerQualifyingNonce(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}
	errFromNotifier := errors.New("downstream")

	wallet := walletPKH()
	key := reservationKey(0xC00A)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {1, 2, 3}},
	}
	// Two pending actions past their timeouts; the notifier fails for the
	// first and succeeds for the second; the walker must continue past the
	// first failure (defensive coverage).
	notifier.err = errFromNotifier
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
			{
				State:     tbtc.ReservationActionStatePending,
				TimeoutAt: 200,
			},
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error from the watcher itself: %v", err)
	}

	// Both attempts are recorded even though the first returned an error:
	// the walker never silently drops notifications.
	if len(notifier.calls) != 2 {
		t.Fatalf("expected two recorded notification attempts, got %d", len(notifier.calls))
	}
}

// TestReservationActionTimeoutWatcher_WatchWallet_Deduplicates verifies that
// registering the same wallet more than once does not grow the watched set.
func TestReservationActionTimeoutWatcher_WatchWallet_Deduplicates(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}
	resolver := &recordingActionTimeoutMembers{}

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)

	wallet := walletPKH()
	watcher.WatchWallet(wallet)
	watcher.WatchWallet(wallet)

	wallets := watcher.watchedWalletsSnapshot()
	if len(wallets) != 1 {
		t.Fatalf("expected 1 watched wallet after duplicate registration, got %d", len(wallets))
	}
	if wallets[0] != wallet {
		t.Errorf("unexpected watched wallet: got %x, want %x", wallets[0], wallet)
	}
}

// TestReservationActionTimeoutWatcher_Run_NilNotifierError verifies Run
// fails its precondition check synchronously (does not block on ctx) when
// the notifier is nil.
func TestReservationActionTimeoutWatcher_Run_NilNotifierError(t *testing.T) {
	spvChain := newLocalChain()
	resolver := &recordingActionTimeoutMembers{}

	watcher := NewReservationActionTimeoutWatcher(spvChain, nil, resolver, time.Millisecond)
	if err := watcher.Run(context.Background()); err == nil {
		t.Fatal("expected error for nil notifier, got nil")
	}
}

// TestReservationActionTimeoutWatcher_Run_NilResolverError mirrors the nil
// notifier case for the members resolver precondition.
func TestReservationActionTimeoutWatcher_Run_NilResolverError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, nil, time.Millisecond)
	if err := watcher.Run(context.Background()); err == nil {
		t.Fatal("expected error for nil resolver, got nil")
	}
}

// TestReservationActionTimeoutWatcher_Run_ZeroIntervalError verifies Run
// refuses to start its poll loop with a non-positive interval, matching the
// documented NewReservationActionTimeoutWatcher contract (a zero interval
// means "synchronous CheckReservationActionTimeouts only").
func TestReservationActionTimeoutWatcher_Run_ZeroIntervalError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}
	resolver := &recordingActionTimeoutMembers{}

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.Run(context.Background()); err == nil {
		t.Fatal("expected error for zero poll interval, got nil")
	}
}

// TestReservationActionTimeoutWatcher_Run_ChecksWatchedWalletsAndStopsOnCancel
// is the end-to-end coverage for the polling loop this task adds: it
// verifies Run (a) immediately checks every wallet registered via
// WatchWallet without waiting a full interval first, (b) notifies the
// Bridge for a reservation whose pending action has timed out, and (c)
// returns promptly once ctx is canceled rather than running forever.
func TestReservationActionTimeoutWatcher_Run_ChecksWatchedWalletsAndStopsOnCancel(t *testing.T) {
	spvChain := newLocalChain()

	notified := make(chan *big.Int, 4)
	notifier := ReservationActionTimeoutNotifierFunc(func(
		reservationKey *big.Int,
		walletMembersIDs []uint32,
	) error {
		notified <- reservationKey
		return nil
	})

	wallet := walletPKH()
	key := reservationKey(0xC00B)
	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {7}},
	}

	// now() is fixed far past the seeded action's TimeoutAt so the very
	// first poll iteration (which runs immediately, before any ticker
	// fires) already finds a timed-out action.
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
		0,
	)
	spvChain.setWalletReservations(wallet, []*big.Int{key})

	watcher := NewReservationActionTimeoutWatcher(
		spvChain,
		notifier,
		resolver,
		time.Millisecond,
	)
	watcher.nowFn = func() uint32 { return 5_000 }
	watcher.WatchWallet(wallet)

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() {
		runErr <- watcher.Run(ctx)
	}()

	select {
	case notifiedKey := <-notified:
		if notifiedKey.Cmp(key) != 0 {
			t.Errorf("unexpected notified reservation key: got %v, want %v", notifiedKey, key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to notify the timed-out action")
	}

	cancel()

	select {
	case err := <-runErr:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled from Run, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to return after ctx cancellation")
	}
}
