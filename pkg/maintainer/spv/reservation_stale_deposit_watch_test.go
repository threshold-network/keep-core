package spv

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/go-test/deep"
)

// reservationDepositKey returns a big.Int constructed from a uint64 to act
// as a reserved-deposit identifier in the stale-deposit watcher tests.
func reservationDepositKey(low uint64) *big.Int {
	return new(big.Int).SetUint64(low)
}

// reservationActionTimeout is the fixed action timeout used by the tests.
// It is large enough to keep the timeout ordering robust against any
// timestamp arithmetic in the watcher.
const reservationActionTimeout uint32 = 3600

func TestReservationStaleDepositWatcher_NonReservedDepositIsSkipped(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	// Deposit is NOT booked as reserved.
	spvChain.setReservedDeposit(reservationDepositKey(0xB001), walletPKH(), false)

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(reservationDepositKey(0xB001), 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("non-reserved deposit must not notify, got %d calls", len(notifier.calls))
	}
}

func TestReservationStaleDepositWatcher_LiveWalletDoesNotNotify(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB002)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateLive,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(key, 10_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("live wallet must not trigger stale notification, got %d calls", len(notifier.calls))
	}
}

func TestReservationStaleDepositWatcher_NotifiesAfterTimeout(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB003)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})

	// Inject the acceptance (nonce 0) action with a deadline well below
	// `now`.
	spvChain.setReservationAction(key, 0, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	// now (5_000) > action.TimeoutAt (100).
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("expected one stale notification, got %d", len(notifier.calls))
	}
	if diff := deep.Equal(key, notifier.calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

func TestReservationStaleDepositWatcher_DoesNotNotifyBeforeTimeout(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB004)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})

	// Action has a deadline of 10_000; we ask the watcher to evaluate at
	// now=5_000, which is before the deadline.
	spvChain.setReservationAction(key, 0, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 10_000,
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("action not yet timed out; expected zero notifications, got %d", len(notifier.calls))
	}
}

func TestReservationStaleDepositWatcher_SettledActionIsSkipped(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB005)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})

	// Action is already settled (no longer pending). The watcher must skip
	// the stale notification even though the wall clock has passed the
	// deadline.
	spvChain.setReservationAction(key, 0, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateSettled,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("settled action must skip stale notification, got %d calls", len(notifier.calls))
	}
}

func TestReservationStaleDepositWatcher_ZeroWalletSkips(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB006)
	spvChain.setReservedDeposit(key, [20]byte{}, true)

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("zero-wallet deposit must skip, got %d calls", len(notifier.calls))
	}
}

func TestReservationStaleDepositWatcher_OnDepositRevealedDelegates(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB007)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservationAction(key, 0, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.OnDepositRevealed(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(notifier.calls))
	}
}

func TestReservationStaleDepositWatcher_NilDepositKeyError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(nil, 5_000); err == nil {
		t.Fatal("expected error for nil deposit key, got nil")
	}
}

// recordingStaleNotifier is a test double that captures every
// NotifyStaleReservedDeposit call.
type recordingStaleNotifier struct {
	calls []*big.Int
}

func (r *recordingStaleNotifier) NotifyStaleReservedDeposit(
	depositKey *big.Int,
) error {
	r.calls = append(r.calls, depositKey)
	return nil
}
