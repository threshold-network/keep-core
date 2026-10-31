package spv

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestA6_ClosingWalletDoesNotStrandBeforeClosingPeriodEnds is the A6 branch
// test for the Closing state's closing-period term.
//
// The client's Closing check ignored the closing period before the fix: it
// notified as soon as the reservation was dissolution-eligible, and the
// Bridge reverted the notification because the wallet's closing period had
// not elapsed. This test fails without the fix.
//
// The reservation here is dissolution-eligible (term ended at 1000, head
// time 2000) but the wallet's closing period runs to 4600, so the
// notification must not be sent.
func TestA6_ClosingWalletDoesNotStrandBeforeClosingPeriodEnds(t *testing.T) {
	lc := newLocalChain()
	wallet := walletPKHAt(0x05)
	key := reservationKey(0xA605)

	headTime := time.Unix(2000, 0)
	lc.setLatestBlockTimestamp(headTime)
	lc.setWalletParameters(tbtc.WalletParameters{
		ClosingPeriod: A6_TEST_CLOSING_PERIOD,
	})

	// The wallet started closing at 1000, so its closing period ends at
	// 1000 + 3600 = 4600, well past the head time of 2000.
	a6InstallWallet(lc, wallet, tbtc.StateClosing, time.Unix(1000, 0))
	a6InstallReservation(lc, wallet, key, 1000)

	watcher := newReservationStrandingWatcher(lc)
	checked, recheck, err := watcher.checkStrandingForWalletState(
		wallet,
		tbtc.StateClosing,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !checked {
		t.Fatal("expected the Closing state to be checked")
	}
	if !recheck {
		t.Error("expected a recheck for a wallet still in its closing period")
	}

	calls := lc.getSubmittedReservationStrandedKeys()
	if len(calls) != 0 {
		t.Fatalf(
			"expected no notifications before the closing period ends, got %d",
			len(calls),
		)
	}
}

// TestA6_ClosingWalletStrandsAfterClosingPeriodEnds is the A6 branch test
// for the Closing state's positive side: once both the dissolution-eligible
// time and the closing period have passed, the notification must be sent.
//
// It fails without the fix if the Closing branch never notifies.
func TestA6_ClosingWalletStrandsAfterClosingPeriodEnds(t *testing.T) {
	lc := newLocalChain()
	wallet := walletPKHAt(0x06)
	key := reservationKey(0xA606)

	headTime := time.Unix(5000, 0)
	lc.setLatestBlockTimestamp(headTime)
	lc.setWalletParameters(tbtc.WalletParameters{
		ClosingPeriod: A6_TEST_CLOSING_PERIOD,
	})

	// The wallet started closing at 1000, so its closing period ends at
	// 4600, before the head time of 5000.
	a6InstallWallet(lc, wallet, tbtc.StateClosing, time.Unix(1000, 0))
	a6InstallReservation(lc, wallet, key, 1000)

	watcher := newReservationStrandingWatcher(lc)
	checked, recheck, err := watcher.checkStrandingForWalletState(
		wallet,
		tbtc.StateClosing,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !checked {
		t.Fatal("expected the Closing state to be checked")
	}
	if recheck {
		t.Error("expected no recheck for a settled Closing wallet")
	}

	calls := lc.getSubmittedReservationStrandedKeys()
	if len(calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(calls))
	}
	if calls[0].Cmp(key) != 0 {
		t.Errorf("unexpected notified key: %v", calls[0])
	}
}

// TestA6_ClosingWalletDoesNotStrandBeforeDissolutionEligible is the A6
// branch test for the Closing state's dissolution-eligible term: a Closing
// wallet whose reservation is still within its term must not be notified.
//
// It fails without the fix if the Closing branch notifies regardless of the
// term.
func TestA6_ClosingWalletDoesNotStrandBeforeDissolutionEligible(t *testing.T) {
	lc := newLocalChain()
	wallet := walletPKHAt(0x07)
	key := reservationKey(0xA607)

	headTime := time.Unix(2000, 0)
	lc.setLatestBlockTimestamp(headTime)
	lc.setWalletParameters(tbtc.WalletParameters{
		ClosingPeriod: A6_TEST_CLOSING_PERIOD,
	})

	// The reservation's term has not ended, which alone must block the
	// notification.
	a6InstallWallet(lc, wallet, tbtc.StateClosing, time.Unix(0, 0))
	a6InstallReservation(lc, wallet, key, 3000)

	watcher := newReservationStrandingWatcher(lc)
	_, recheck, err := watcher.checkStrandingForWalletState(
		wallet,
		tbtc.StateClosing,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recheck {
		t.Error("expected a recheck for a reservation still within its term")
	}

	calls := lc.getSubmittedReservationStrandedKeys()
	if len(calls) != 0 {
		t.Fatalf("expected no notifications, got %d", len(calls))
	}
}
