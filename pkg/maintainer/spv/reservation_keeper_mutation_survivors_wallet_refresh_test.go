package spv

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationKeeperObserver_RefreshReadsAndStamps checks the wallet
// refresh read/stamp semantics: a cache entry last read past the
// refresh interval and a never-read entry are both re-read on a pass, a
// recently read one is skipped, and a successful pass stamps the entry
// so the next pass within the interval re-reads nothing.
func TestReservationKeeperObserver_RefreshReadsAndStamps(t *testing.T) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	counting := &walletCountingChain{localChain: spvChain}
	observer := newObserverForTest(spvChain, 0, recorder)
	observer.spvChain = counting

	a := [20]byte{1}
	b := [20]byte{2}
	c := [20]byte{3}
	d := [20]byte{4}
	now := time.Now()
	spvChain.setWallet(a, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.setWallet(b, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.setWallet(c, &tbtc.WalletChainData{State: tbtc.StateClosing})
	spvChain.setWallet(d, &tbtc.WalletChainData{State: tbtc.StateClosing})
	// a sits just past the 10-minute interval; d sits between the 9- and
	// 10-minute interval variants; c is recently read; b is never read.
	observer.cachedWallets[a] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateLive},
		fetchedAt: now.Add(-10*time.Minute - 30*time.Second),
	}
	observer.cachedWallets[b] = &cachedWalletState{}
	observer.cachedWallets[c] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateClosing},
		fetchedAt: now.Add(-30 * time.Second),
	}
	observer.cachedWallets[d] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateClosing},
		fetchedAt: now.Add(-9*time.Minute - 30*time.Second),
	}

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("first observation pass failed: [%v]", err)
	}
	if got := counting.walletFetches; got != 2 {
		t.Fatalf(
			"wallet reads on the first pass = %d, want 2 "+
				"(the over-interval and the never-read entries)",
			got,
		)
	}

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("second observation pass failed: [%v]", err)
	}
	if got := counting.walletFetches; got != 2 {
		t.Errorf(
			"wallet reads after the second pass = %d, want 2 "+
				"(every entry was stamped by the first pass)",
			got,
		)
	}
}

// TestReservationKeeperObserver_RefreshSkipsThenFetches checks that the
// refresh loop keeps walking after a skipped entry: an over-interval
// entry that follows a recently read one must still be re-read, not
// skipped because the loop stopped at the skip.
func TestReservationKeeperObserver_RefreshSkipsThenFetches(t *testing.T) {
	spvChain, _, recorder, _, _ := observerFixture(t)
	spvChain.setWalletParameters(tbtc.WalletParameters{})
	counting := &walletCountingChain{localChain: spvChain}
	observer := newObserverForTest(spvChain, 0, recorder)
	observer.spvChain = counting

	a := [20]byte{1}
	b := [20]byte{2}
	now := time.Now()
	spvChain.setWallet(a, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.setWallet(b, &tbtc.WalletChainData{State: tbtc.StateClosing})
	// a is recently read: skipped. b is over the interval: must still
	// be read on the same pass, even though it follows the skipped
	// entry in the map walk.
	observer.cachedWallets[a] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateLive},
		fetchedAt: now.Add(-30 * time.Second),
	}
	observer.cachedWallets[b] = &cachedWalletState{
		data:      &tbtc.WalletChainData{State: tbtc.StateClosing},
		fetchedAt: now.Add(-11 * time.Minute),
	}

	if err := observer.ObservationPass(); err != nil {
		t.Fatalf("observation pass failed: [%v]", err)
	}
	if got := counting.walletFetches; got != 1 {
		t.Errorf(
			"wallet reads = %d, want 1 (the over-interval entry, after "+
				"the skipped one)",
			got,
		)
	}
}
