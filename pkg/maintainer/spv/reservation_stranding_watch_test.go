package spv

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/go-test/deep"
)

// reservationKey constructs an in-memory reservation key with the given low
// 16 bytes set. The watcher only consults `GetReservation` via the local
// chain's [16]byte map key, so compact test keys avoid accidental collisions
// across test cases.
func reservationKey(low uint64) *big.Int {
	return new(big.Int).SetUint64(low)
}

// walletPKH is a deterministic wallet public key hash used in the stranding
// tests. Tests that need a different wallet use walletPKHAt.
func walletPKH() [20]byte {
	var out [20]byte
	out[19] = 0x42
	return out
}

// walletPKHAt returns a wallet PKH with the trailing byte set to byte b. It
// exists to make multi-wallet tests readable.
func walletPKHAt(b byte) [20]byte {
	var out [20]byte
	out[19] = b
	return out
}

func TestReservationStrandingWatcher_NoReservations(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStrandingNotifier{}

	watcher := NewReservationStrandingWatcher(spvChain, notifier)
	if watcher == nil {
		t.Fatal("expected non-nil watcher")
	}

	if err := watcher.CheckReservationStrandingForWallet(walletPKH()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf(
			"expected no notifications, got %d",
			len(notifier.calls),
		)
	}
}

func TestReservationStrandingWatcher_NotifiesActiveReservation(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStrandingNotifier{}

	wallet := walletPKH()
	key := reservationKey(0xAA01)

	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	watcher := NewReservationStrandingWatcher(spvChain, notifier)
	if err := watcher.CheckReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(notifier.calls))
	}
	if diff := deep.Equal(key, notifier.calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

func TestReservationStrandingWatcher_NotifiesClosedReservation(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStrandingNotifier{}

	wallet := walletPKH()
	key := reservationKey(0xAA02)

	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateClosed,
	})

	watcher := NewReservationStrandingWatcher(spvChain, notifier)
	if err := watcher.CheckReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(notifier.calls))
	}
}

func TestReservationStrandingWatcher_SkipsPendingReservation(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStrandingNotifier{}

	wallet := walletPKH()
	key := reservationKey(0xAA03)

	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActionPending,
	})

	watcher := NewReservationStrandingWatcher(spvChain, notifier)
	if err := watcher.CheckReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf(
			"expected pending reservation to defer to action-timeout, "+
				"got %d notifications",
			len(notifier.calls),
		)
	}
}

func TestReservationStrandingWatcher_MultipleReservations(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStrandingNotifier{}

	wallet := walletPKH()
	active := reservationKey(0xAA10)
	closed := reservationKey(0xAA11)
	pending := reservationKey(0xAA12)
	stranded := reservationKey(0xAA13)

	spvChain.setWalletReservations(
		wallet,
		[]*big.Int{active, closed, pending, stranded},
	)
	spvChain.setReservation(active, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})
	spvChain.setReservation(closed, &tbtc.Reservation{
		State: tbtc.ReservationStateClosed,
	})
	spvChain.setReservation(pending, &tbtc.Reservation{
		State: tbtc.ReservationStateActionPending,
	})
	spvChain.setReservation(stranded, &tbtc.Reservation{
		State: tbtc.ReservationStateStranded,
	})

	watcher := NewReservationStrandingWatcher(spvChain, notifier)
	if err := watcher.CheckReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The watcher must notify for all reservations that are not in
	// ActionPending. ReservationStateStranded is the natural re-notify case
	// (the Bridge dedupes; the watcher does not).
	if len(notifier.calls) != 3 {
		t.Fatalf(
			"expected three notifications (active+closed+stranded), "+
				"got %d: %v",
			len(notifier.calls),
			notifier.calls,
		)
	}
}

func TestReservationStrandingWatcher_UnknownReservationIsSkipped(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStrandingNotifier{}

	wallet := walletPKH()
	staleKey := reservationKey(0xAA20)
	freshKey := reservationKey(0xAA21)

	// walletReservations references staleKey but the chain has no record of
	// it. freshKey is properly recorded.
	spvChain.setWalletReservations(
		wallet,
		[]*big.Int{staleKey, freshKey},
	)
	spvChain.setReservation(freshKey, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	watcher := NewReservationStrandingWatcher(spvChain, notifier)
	if err := watcher.CheckReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("expected one notification (freshKey), got %d", len(notifier.calls))
	}
	if diff := deep.Equal(freshKey, notifier.calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

func TestReservationStrandingWatcher_WalletChainError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStrandingNotifier{}

	// No walletReservations entry: WalletReservations returns (nil, nil) for
	// unknown wallets; the watcher iterates over a nil slice and exits
	// cleanly, so we expect no error here. Reserve the case for wallets that
	// have an entry which the chain then refuses to enumerate.
	wallet := walletPKH()
	spvChain.setWalletReservations(wallet, nil)

	watcher := NewReservationStrandingWatcher(spvChain, notifier)
	if err := watcher.CheckReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error for empty wallet: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf(
			"expected zero notifications on empty wallet list, got %d",
			len(notifier.calls),
		)
	}
}

func TestReservationStrandingWatcher_NotifierFuncAdapter(t *testing.T) {
	var captured []*big.Int
	notifier := ReservationStrandingNotifierFunc(func(key *big.Int) error {
		captured = append(captured, key)
		return nil
	})

	spvChain := newLocalChain()
	wallet := walletPKHAt(0x01)
	key := reservationKey(0xAA30)
	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	watcher := NewReservationStrandingWatcher(spvChain, notifier)
	if err := watcher.CheckReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(captured) != 1 {
		t.Fatalf("expected one captured key, got %d", len(captured))
	}
}

// recordingStrandingNotifier is a test double that captures every
// NotifyReservationStranded call. It is used to assert the watcher fires
// the expected notifications in the expected order.
type recordingStrandingNotifier struct {
	calls []*big.Int
}

func (r *recordingStrandingNotifier) NotifyReservationStranded(
	reservationKey *big.Int,
) error {
	r.calls = append(r.calls, reservationKey)
	return nil
}
