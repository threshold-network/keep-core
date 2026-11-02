package spv

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// A6 covers the stranding watcher's wallet-state branches. The Bridge
// accepts exactly four wallet states in notifyReservationStranded
// (Reservation.sol at tbtc-v2 44d9c9c6a):
//
//	Terminated  -> strand outright
//	Closed      -> strand outright
//	Live        -> strand once block.timestamp >= dissolutionEligibleAt
//	Closing     -> strand once block.timestamp >= dissolutionEligibleAt
//	               AND block.timestamp >= closingStartedAt + walletClosingPeriod
//
// The client must notify exactly when the Bridge would accept the
// notification: a notification before the Bridge's own gate reverts, and a
// missing notification leaves the position pinned forever.

// A6_TEST_CLOSING_PERIOD is the wallet closing period the tests install.
const A6_TEST_CLOSING_PERIOD uint32 = 3600

// a6InstallWallet installs a wallet in the given state with the given
// closing start time.
func a6InstallWallet(
	lc *localChain,
	wallet [20]byte,
	state tbtc.WalletState,
	closingStartedAt time.Time,
) {
	lc.setWallet(wallet, &tbtc.WalletChainData{
		State:            state,
		ClosingStartedAt: closingStartedAt,
	})
}

// a6InstallReservation installs an Active reservation whose term became
// dissolution-eligible at the given timestamp.
func a6InstallReservation(
	lc *localChain,
	wallet [20]byte,
	key *big.Int,
	dissolutionEligibleAt uint32,
) {
	lc.setWalletReservations(wallet, []*big.Int{key})
	lc.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash:   wallet,
		State:                 tbtc.ReservationStateActive,
		DissolutionEligibleAt: dissolutionEligibleAt,
	})
}

// TestA6_ClosedWalletStrandsReservation is the A6 branch test for the
// Closed state: a Closed wallet strands its Active reservations outright,
// with no dissolution-eligible gate.
//
// It fails without the fix if the Closed branch is broken.
func TestA6_ClosedWalletStrandsReservation(t *testing.T) {
	lc := newLocalChain()
	wallet := walletPKHAt(0x01)
	key := reservationKey(0xA601)

	a6InstallWallet(lc, wallet, tbtc.StateClosed, time.Unix(0, 0))
	// A Closed wallet strands regardless of the term, so the reservation
	// is deliberately not dissolution-eligible.
	a6InstallReservation(lc, wallet, key, 0)

	watcher := newReservationStrandingWatcher(lc)
	if _, err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := lc.getSubmittedReservationStrandedKeys()
	if len(calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(calls))
	}
	if calls[0].Cmp(key) != 0 {
		t.Errorf("unexpected notified key: %v", calls[0])
	}
}

// TestA6_TerminatedWalletStrandsReservation is the A6 branch test for the
// Terminated state: a Terminated wallet strands its Active reservations
// outright, with no dissolution-eligible gate.
//
// It fails without the fix if the Terminated branch is broken.
func TestA6_TerminatedWalletStrandsReservation(t *testing.T) {
	lc := newLocalChain()
	wallet := walletPKHAt(0x02)
	key := reservationKey(0xA602)

	a6InstallWallet(lc, wallet, tbtc.StateTerminated, time.Unix(0, 0))
	a6InstallReservation(lc, wallet, key, 0)

	watcher := newReservationStrandingWatcher(lc)
	if _, err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := lc.getSubmittedReservationStrandedKeys()
	if len(calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(calls))
	}
	if calls[0].Cmp(key) != 0 {
		t.Errorf("unexpected notified key: %v", calls[0])
	}
}

// TestA6_LiveWalletStrandsDissolutionEligibleReservation is the A6 branch
// test for the Live state: a Live wallet strands an Active reservation
// once the reservation's term has passed its dissolution-eligible time.
//
// The Live branch did not exist before the fix: the dispatcher's default
// case skipped a Live wallet entirely, so this test fails without it.
func TestA6_LiveWalletStrandsDissolutionEligibleReservation(t *testing.T) {
	lc := newLocalChain()
	wallet := walletPKHAt(0x03)
	key := reservationKey(0xA603)

	headTime := time.Unix(2000, 0)
	lc.setLatestBlockTimestamp(headTime)

	a6InstallWallet(lc, wallet, tbtc.StateLive, time.Unix(0, 0))
	// The reservation's term ended at 1000 and the head time is 2000, so
	// the reservation is dissolution-eligible and the Live wallet may
	// strand it.
	a6InstallReservation(lc, wallet, key, 1000)

	watcher := newReservationStrandingWatcher(lc)
	checked, recheck, err := watcher.checkStrandingForWalletState(
		wallet,
		tbtc.StateLive,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !checked {
		t.Fatal("expected the Live state to be checked")
	}
	if recheck {
		t.Error("expected no recheck for a settled Live wallet")
	}

	calls := lc.getSubmittedReservationStrandedKeys()
	if len(calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(calls))
	}
	if calls[0].Cmp(key) != 0 {
		t.Errorf("unexpected notified key: %v", calls[0])
	}
}

// TestA6_LiveWalletDoesNotStrandReservationBeforeDissolutionEligible is the
// A6 branch test for the Live state's negative side: a Live wallet whose
// reservation is still within its term must not be notified, because the
// Bridge would revert the notification.
//
// It fails without the fix if the Live branch notifies regardless of the
// term.
func TestA6_LiveWalletDoesNotStrandReservationBeforeDissolutionEligible(t *testing.T) {
	lc := newLocalChain()
	wallet := walletPKHAt(0x04)
	key := reservationKey(0xA604)

	headTime := time.Unix(2000, 0)
	lc.setLatestBlockTimestamp(headTime)

	a6InstallWallet(lc, wallet, tbtc.StateLive, time.Unix(0, 0))
	// The reservation's term ends at 3000, so it is not yet
	// dissolution-eligible at the head time of 2000.
	a6InstallReservation(lc, wallet, key, 3000)

	watcher := newReservationStrandingWatcher(lc)
	checked, recheck, err := watcher.checkStrandingForWalletState(
		wallet,
		tbtc.StateLive,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !checked {
		t.Fatal("expected the Live state to be checked")
	}
	if !recheck {
		t.Error("expected a recheck for a wallet whose term has not ended")
	}

	calls := lc.getSubmittedReservationStrandedKeys()
	if len(calls) != 0 {
		t.Fatalf("expected no notifications, got %d", len(calls))
	}
}

// TestA6_MovingFundsWalletIsNotChecked pins that a wallet in a state the
// Bridge does not accept is left alone: MovingFunds is not one of the four
// admissible states in notifyReservationStranded, so the client must not
// even read its reservations.
//
// It fails without the fix if the dispatcher starts checking a state the
// Bridge would reject.
func TestA6_MovingFundsWalletIsNotChecked(t *testing.T) {
	lc := newLocalChain()
	wallet := walletPKHAt(0x08)
	key := reservationKey(0xA608)

	a6InstallWallet(lc, wallet, tbtc.StateMovingFunds, time.Unix(0, 0))
	a6InstallReservation(lc, wallet, key, 0)

	watcher := newReservationStrandingWatcher(lc)
	checked, _, err := watcher.checkStrandingForWalletState(
		wallet,
		tbtc.StateMovingFunds,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checked {
		t.Error("expected the MovingFunds state to not be checked")
	}

	calls := lc.getSubmittedReservationStrandedKeys()
	if len(calls) != 0 {
		t.Fatalf("expected no notifications, got %d", len(calls))
	}
}
