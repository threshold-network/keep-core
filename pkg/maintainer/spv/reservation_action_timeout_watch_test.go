package spv

import (
	"errors"
	"math/big"
	"testing"

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
		1,
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
		1,
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

func TestReservationActionTimeoutWatcher_IgnoresSettledOlderGeneration(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}

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

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf(
			"current generation is pending and past deadline; expected one "+
				"notification, got %d",
			len(notifier.calls),
		)
	}
}

func TestReservationActionTimeoutWatcher_NotifiesCurrentGenerationOnly(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}

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

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf(
			"expected exactly one notification for the current generation, got %d",
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
		1,
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
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	if err := watcher.CheckReservationActionTimeouts(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(captured) != 1 {
		t.Fatalf("expected one captured notification, got %d", len(captured))
	}
}

func TestReservationActionTimeoutWatcher_NotifierErrorPropagates(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingActionTimeoutNotifier{}
	errFromNotifier := errors.New("downstream")

	wallet := walletPKH()
	key := reservationKey(0xC00A)

	resolver := &recordingActionTimeoutMembers{
		walletIDs: map[[20]byte][]uint32{wallet: {1, 2, 3}},
	}
	// The current generation is pending and past its deadline, but the
	// notifier fails. With only one generation ever inspected per Check
	// call, the failure must surface as an error from
	// CheckReservationActionTimeouts (not be silently swallowed), so a
	// poll-loop caller logs and retries on the next tick instead of
	// wrongly treating it as settled.
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
		},
		1,
	)

	watcher := NewReservationActionTimeoutWatcher(spvChain, notifier, resolver, 0)
	err := watcher.CheckReservationActionTimeouts(key, 5_000)
	if err == nil {
		t.Fatal("expected the notifier error to propagate, got nil")
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("expected exactly one notification attempt, got %d", len(notifier.calls))
	}
}
