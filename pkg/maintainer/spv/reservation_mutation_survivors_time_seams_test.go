package spv

import (
	"context"
	"math/big"
	"testing"
	"time"

	ethereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationKeeperObserver_MutationSurvivors_WalletCacheRefreshAtInterval
// pins the wallet-cache refresh interval as half-open: an entry last read
// exactly walletCacheRefreshInterval ago is re-read on this pass, and one
// read a nanosecond more recently than that is not. Reading the interval as
// inclusive defers the boundary entry's refresh by a whole pass, so a wallet
// whose state changed right at the boundary keeps feeding the risk gauges
// from stale data for one tick. The clock is injected because a live one
// cannot be placed on the boundary at nanosecond resolution.
func TestReservationKeeperObserver_MutationSurvivors_WalletCacheRefreshAtInterval(
	t *testing.T,
) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	counting := &walletCountingChain{localChain: spvChain}
	observer := newObserverForTest(spvChain, 0, recorder)
	observer.spvChain = counting

	now := time.Unix(1_700_000_000, 0)
	observer.nowFn = func() time.Time { return now }

	atBoundary := [20]byte{0x71}
	justInside := [20]byte{0x72}
	spvChain.setWallet(atBoundary, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.setWallet(justInside, &tbtc.WalletChainData{State: tbtc.StateLive})
	observer.cachedWallets[atBoundary] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateLive},
		fetchedAt: now.Add(-walletCacheRefreshInterval),
	}
	observer.cachedWallets[justInside] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateLive},
		fetchedAt: now.Add(-walletCacheRefreshInterval + time.Nanosecond),
	}

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}

	if got := counting.walletFetches; got != 1 {
		t.Fatalf(
			"wallet reads at the epoch-sized interval boundary = %d, want 1 "+
				"(the entry aged exactly walletCacheRefreshInterval must be "+
				"re-read; the one a nanosecond younger must not)",
			got,
		)
	}
	if got := observer.cachedWallets[atBoundary].fetchedAt; !got.Equal(now) {
		t.Errorf(
			"refreshed entry stamped at %v, want the injected clock's %v",
			got,
			now,
		)
	}
	if got := observer.cachedWallets[justInside].fetchedAt; got.Equal(now) {
		t.Errorf(
			"entry younger than the interval was stamped at the pass time "+
				"(%v), it must be skipped",
			got,
		)
	}
}

// TestWireReservationWatchers_MutationSurvivors_StrandingSecondChanceRetryScheduled
// pins the scheduling condition on the stranding startup scan's second-chance
// retry: a wallet whose GetWallet call failed on all three startup-scan
// attempts must still be checked once the retry delay elapses, because the
// live close-event subscription cannot replay a close that already happened.
// Inverting the condition (spawning the retry only for an empty unresolved
// list) leaves that wallet's stranded reservations unnotified forever.
func TestWireReservationWatchers_MutationSurvivors_StrandingSecondChanceRetryScheduled(
	t *testing.T,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	base := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(5000)
	base.setBlockCounter(blockCounter)

	wallet := walletPKHAt(0x22)
	reservationKey := reservationKey(0xC200)

	base.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       [32]byte{0x22},
		WalletPublicKeyHash: wallet,
	})
	base.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateClosed})
	base.setWalletReservations(wallet, []*big.Int{reservationKey})
	base.setReservation(reservationKey, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	// Exactly the three startup-scan attempts fail, so the wallet lands in
	// unresolvedWallets and the second-chance read is the first one to
	// succeed.
	chain := &transientGetWalletChain{localChain: base, wallet: wallet}
	chain.failuresLeft.Store(3)

	setReservationStrandingStartupScanRetryDelay(5 * time.Millisecond)
	defer setReservationStrandingStartupScanRetryDelay(
		DefaultReservationStrandingStartupScanRetryDelay,
	)

	if err := WireReservationWatchers(
		ctx,
		&mockWalletClosedChain{},
		chain,
		true,
		nil,
		ethereum.Unknown,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		notified := base.getSubmittedReservationStrandedKeys()
		if len(notified) > 0 {
			if len(notified) != 1 {
				t.Fatalf(
					"expected exactly one stranding notification from the "+
						"second-chance pass, got %d",
					len(notified),
				)
			}
			if notified[0].Cmp(reservationKey) != 0 {
				t.Errorf("unexpected notified key: %v", notified[0])
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"the second-chance retry pass never checked the wallet whose " +
					"startup reads all failed: no stranding notification within " +
					"5s of wiring, so its stranded reservations stay unnotified",
			)
		}
		time.Sleep(time.Millisecond)
	}
}
