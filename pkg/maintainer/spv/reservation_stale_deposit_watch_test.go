package spv

import (
	"fmt"
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

	// Inject the acceptance (nonce 1) action with a deadline well below
	// `now`.
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
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
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
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
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
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
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
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

func TestReservationStaleDepositWatcher_IsReservedDepositChainError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	spvChain.isReservedDepositErr = fmt.Errorf("rpc unavailable")

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	err := watcher.CheckStaleReservedDeposit(reservationDepositKey(0xB010), 5_000)
	if err == nil {
		t.Fatal("expected error when IsReservedDeposit fails, got nil")
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(notifier.calls))
	}
}

func TestReservationStaleDepositWatcher_ReservedDepositWalletChainError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB011)
	spvChain.setReservedDeposit(key, walletPKH(), true)
	spvChain.reservedDepositWalletErr = fmt.Errorf("rpc unavailable")

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err == nil {
		t.Fatal("expected error when ReservedDepositWallet fails, got nil")
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(notifier.calls))
	}
}

// TestReservationStaleDepositWatcher_GetWalletChainError exercises the
// error passthrough without any special chain-double wiring: the deposit's
// assigned wallet is never registered via setWallet, so GetWallet fails
// with its natural "no wallet for given PKH" error exactly as a real chain
// would if the wallet were somehow unresolvable.
func TestReservationStaleDepositWatcher_GetWalletChainError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB012)
	spvChain.setReservedDeposit(key, walletPKH(), true)
	// No spvChain.setWallet call: GetWallet errors naturally.

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err == nil {
		t.Fatal("expected error when GetWallet fails, got nil")
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(notifier.calls))
	}
}

// TestReservationStaleDepositWatcher_GetReservationActionChainError
// exercises the error passthrough the same way: the acceptance action
// (nonce 1) is never installed via setReservationAction, so
// GetReservationAction fails with its natural "no action for given
// reservation/nonce" error.
func TestReservationStaleDepositWatcher_GetReservationActionChainError(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB013)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	// No spvChain.setReservationAction call: GetReservationAction errors
	// naturally.

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err == nil {
		t.Fatal("expected error when GetReservationAction fails, got nil")
	}

	if len(notifier.calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(notifier.calls))
	}
}

// TestReservationStaleDepositWatcher_NotifierError verifies that, unlike
// the stranding watcher (which continues past a notifier failure because
// it processes a batch of reservations per call), the stale-deposit
// watcher propagates a NotifyStaleReservedDeposit failure to its single
// caller: CheckStaleReservedDeposit checks exactly one deposit per call, so
// there is nothing else to "continue" to.
func TestReservationStaleDepositWatcher_NotifierError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB014)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})

	notifier := StaleReservedDepositNotifierFunc(func(*big.Int) error {
		return fmt.Errorf("notifier unavailable")
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err == nil {
		t.Fatal("expected error when the notifier fails, got nil")
	}
}

// TestReservationStaleDepositWatcher_ExactTimeoutBoundaryDoesNotNotify
// covers the `now == action.TimeoutAt` boundary explicitly: the watcher's
// condition is `now <= action.TimeoutAt` (must NOT notify), so equality
// must defer exactly like "before the deadline" does. Existing tests only
// exercise now < TimeoutAt and now > TimeoutAt.
func TestReservationStaleDepositWatcher_ExactTimeoutBoundaryDoesNotNotify(t *testing.T) {
	spvChain := newLocalChain()
	notifier := &recordingStaleNotifier{}

	key := reservationDepositKey(0xB015)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 5_000,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, notifier)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(notifier.calls) != 0 {
		t.Fatalf(
			"now == action.TimeoutAt must not notify, got %d calls",
			len(notifier.calls),
		)
	}
}

func TestReservationStaleDepositWatcher_NilNotifierError(t *testing.T) {
	spvChain := newLocalChain()
	watcher := NewReservationStaleDepositWatcher(spvChain, nil)

	key := reservationDepositKey(0xB016)

	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err == nil {
		t.Fatal("expected error for nil notifier via CheckStaleReservedDeposit, got nil")
	}
	if err := watcher.OnDepositRevealed(key, 5_000); err == nil {
		t.Fatal("expected error for nil notifier via OnDepositRevealed, got nil")
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
