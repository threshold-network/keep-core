package tbtcpg

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

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
