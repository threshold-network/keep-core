package tbtcpg

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// scanningLocalChain wraps LocalChain with a PastNewWalletRegisteredEvents
// call counter so a cache-vs-scan test can observe how many registration
// scans findTargetWallet actually triggered.
type scanningLocalChain struct {
	*LocalChain

	calls int
}

func (c *scanningLocalChain) PastNewWalletRegisteredEvents(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	c.calls++
	return c.LocalChain.PastNewWalletRegisteredEvents(filter)
}

// TestReservationReanchorTask_RecordsLiveWalletsCountGauge is a regression
// test for the same saturation-monitoring gap in the re-anchor task: Run
// already fetches GetLiveWalletsCount but never exposed it as a metric.
func TestReservationReanchorTask_RecordsLiveWalletsCountGauge(t *testing.T) {
	lc := NewLocalChain()
	btcChain := NewLocalBitcoinChain()

	sourceWalletPublicKeyHash := [20]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	targetWalletPublicKeyHash := [20]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}

	blockCounter := NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	lc.SetBlockCounter(blockCounter)

	if err := lc.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: targetWalletPublicKeyHash},
	); err != nil {
		t.Fatal(err)
	}

	lc.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	lc.SetWallet(targetWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	// Run fetches ReservationParameters before the reservation loop
	// (for the per-wallet count headroom check), so the test must
	// install at least a TxMaxFee here.
	lc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      100000,
		MaxReservationsPerWallet: 5,
	})
	// A reservation exists but has no anchor UTXO, so the wallet is left
	// with nothing eligible to re-anchor. This test only cares that the
	// live_wallets_count gauge fires before that failure, matching Run's
	// actual call order.
	reservationKey := big.NewInt(1)
	lc.SetWalletReservations(sourceWalletPublicKeyHash, []*big.Int{reservationKey})
	lc.SetReservation(reservationKey, &tbtc.Reservation{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
		State:               tbtc.ReservationStateActive,
	})
	lc.SetLiveWalletsCount(3)

	task := NewReservationReanchorTask(lc, btcChain)
	recorder := newFakeMetricsRecorder()
	task.setMetricsRecorder(recorder)

	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: sourceWalletPublicKeyHash,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, ok := recorder.calls["live_wallets_count"]; !ok {
		t.Error("expected live_wallets_count gauge to be recorded")
	} else if got != 3 {
		t.Errorf("expected live_wallets_count = 3, got %v", got)
	}
}

// TestReservationReanchorTask_FindTargetWallet_CachesAcrossRuns is a
// regression test for the target-wallet cache added to findTargetWallet:
// once a re-anchor target wallet has been selected for a source wallet,
// a subsequent call to findTargetWallet for the same source wallet must
// reuse the cached target -- validating only that one wallet via
// GetWallet -- instead of repeating the O(W) GetWallet-per-registration
// scan. A cached target whose headroom has since dropped must be
// evicted so the next call performs a fresh scan, and that fresh scan
// must pick the most recently registered Live wallet that still has
// room.
func TestReservationReanchorTask_FindTargetWallet_CachesAcrossRuns(t *testing.T) {
	walletA := [20]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	walletB := [20]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	walletC := [20]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}

	lc := &scanningLocalChain{LocalChain: NewLocalChain()}

	blockCounter := NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	lc.SetBlockCounter(blockCounter)

	// Register B at block 100, then C at block 200 (C is newest).
	if err := lc.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: walletB},
	); err != nil {
		t.Fatal(err)
	}
	if err := lc.AddPastNewWalletRegisteredEvent(
		&tbtc.NewWalletRegisteredEventFilter{StartBlock: 0},
		&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: walletC},
	); err != nil {
		t.Fatal(err)
	}

	lc.SetWallet(walletB, &tbtc.WalletChainData{State: tbtc.StateLive})
	lc.SetWallet(walletC, &tbtc.WalletChainData{State: tbtc.StateLive})

	lc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      100000,
		MaxReservationsPerWallet: 5,
	})

	task := NewReservationReanchorTask(lc, NewLocalBitcoinChain())

	params, err := task.chain.ReservationParameters()
	if err != nil {
		t.Fatalf("ReservationParameters: %v", err)
	}

	// First call: no cache, must scan the registration events.
	got, err := task.findTargetWallet(
		logger,
		walletA,
		100000,
		params.MaxReservationsPerWallet,
		100000000,
		map[[20]byte]bool{},
	)
	if err != nil {
		t.Fatalf("first findTargetWallet: unexpected error: %v", err)
	}
	if got != walletC {
		t.Fatalf("first call: expected target wallet C [%x], got [%x]", walletC, got)
	}
	if lc.calls != 1 {
		t.Fatalf(
			"first call should have triggered exactly 1 registration-event "+
				"scan, got %d",
			lc.calls,
		)
	}

	// Second call: cache hit. No new registration-event scan.
	got, err = task.findTargetWallet(
		logger,
		walletA,
		100000,
		params.MaxReservationsPerWallet,
		100000000,
		map[[20]byte]bool{},
	)
	if err != nil {
		t.Fatalf("second findTargetWallet (cache hit): unexpected error: %v", err)
	}
	if got != walletC {
		t.Fatalf(
			"second call: expected the cached target wallet C [%x], got [%x]",
			walletC,
			got,
		)
	}
	if lc.calls != 1 {
		t.Fatalf(
			"cache hit must not trigger another registration-event scan; "+
				"observed %d total scans after 2 calls (want 1)",
			lc.calls,
		)
	}

	// Third call: cached target C now has no headroom (count = cap),
	// which forces eviction and a fresh scan. The fresh scan walks B
	// and C newest-first; C is excluded by the caller-supplied
	// triedTargets map, B has headroom -- so B wins.
	walletKeys := make([]*big.Int, 5)
	for i := range walletKeys {
		walletKeys[i] = big.NewInt(int64(1000 + i))
	}
	lc.SetWalletReservations(walletC, walletKeys)
	got, err = task.findTargetWallet(
		logger,
		walletA,
		100000,
		params.MaxReservationsPerWallet,
		100000000,
		map[[20]byte]bool{walletC: true},
	)
	if err != nil {
		t.Fatalf("third findTargetWallet (evict + rescan): unexpected error: %v", err)
	}
	if got != walletB {
		t.Fatalf(
			"third call: expected the fresh-scan target wallet B [%x] "+
				"after the cached C lost headroom, got [%x]",
			walletB,
			got,
		)
	}
	if lc.calls != 2 {
		t.Fatalf(
			"eviction must trigger a fresh scan; observed %d total "+
				"scans after 3 calls (want 2)",
			lc.calls,
		)
	}
}
