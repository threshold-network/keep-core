package spv

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestA6_ClosingWalletStrandsAtClosingPeriodEnd pins the closing-period
// gate at its exact boundary: the Bridge compares block.timestamp against
// closingStartedAt + closingPeriod with >= , so a chain head exactly at
// the closing-period end already satisfies the wallet's exclusive
// re-anchor window and the notification must be sent. Treating the
// boundary as "still inside the closing period" delays the stranding
// notification by a full recheck cycle for no reason.
func TestA6_ClosingWalletStrandsAtClosingPeriodEnd(t *testing.T) {
	lc := newLocalChain()
	wallet := walletPKHAt(0x08)
	key := reservationKey(0xA608)

	// The wallet started closing at 1000 with a closing period of 3600,
	// so the closing period ends at 4600; the head time is exactly there.
	headTime := time.Unix(4600, 0)
	lc.setLatestBlockTimestamp(headTime)
	lc.setWalletParameters(tbtc.WalletParameters{
		ClosingPeriod: A6_TEST_CLOSING_PERIOD,
	})

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
		t.Error(
			"expected no recheck once the chain head reached the " +
				"closing-period end exactly",
		)
	}

	calls := lc.getSubmittedReservationStrandedKeys()
	if len(calls) != 1 {
		t.Fatalf(
			"expected one notification at the exact closing-period end, got %d",
			len(calls),
		)
	}
	if calls[0].Cmp(key) != 0 {
		t.Errorf("unexpected notified key: %v", calls[0])
	}
}