package tbtcpg

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// fakeMetricsRecorder captures SetGauge calls for assertion, distinguishing
// "never called" from "called with a zero value" - the pre-registered gauge
// default in the real PerformanceMetrics would make a value-only assertion
// pass even if the wiring were silently dropped.
type fakeMetricsRecorder struct {
	calls map[string]float64
}

func newFakeMetricsRecorder() *fakeMetricsRecorder {
	return &fakeMetricsRecorder{calls: make(map[string]float64)}
}

func (f *fakeMetricsRecorder) SetGauge(name string, value float64) {
	f.calls[name] = value
}

// TestReservationAcceptanceTask_RecordsSaturationGauges is a regression
// test for the M-clientinfo saturation-monitoring gap: findDeposits already
// fetches wallet_reservations_count, active_reservations_count, and
// max_active_reservations from the chain, but nothing exposed them as
// metrics, so an operator could not see reservation capacity approaching
// its cap before acceptances silently stopped.
func TestReservationAcceptanceTask_RecordsSaturationGauges(t *testing.T) {
	lc := NewLocalChain()
	btcChain := NewLocalBitcoinChain()

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	lc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationVault: chain.Address(
			"0xReservationVaultAddress1234567890abcdef12345678",
		),
		ReservationMinAmount:      1000,
		ReservationTxMaxFee:       5000,
		MaxReservationsPerWallet:  5,
		ReservationMaxTotalAmount: 100000000,
	})
	lc.SetWallet(walletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
	lc.SetDepositMinAge(3600)

	blockCounter := NewMockBlockCounter()
	blockCounter.SetCurrentBlock(300000)
	lc.SetBlockCounter(blockCounter)

	// Non-zero wallet reservations count so the assertion below can
	// distinguish a real wired value from a coincidental zero default.
	lc.SetWalletReservations(walletPublicKeyHash, []*big.Int{big.NewInt(1), big.NewInt(2)})

	// Run scans PastDepositRevealedEvents with a filter bounded by
	// ReservationAcceptanceLookBackBlocks; register an empty match so the
	// call succeeds and Run proceeds to (correctly) report no candidate,
	// rather than erroring on an unregistered filter.
	currentBlock := uint64(300000)
	filterStartBlock := currentBlock - ReservationAcceptanceLookBackBlocks
	if err := lc.AddPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          filterStartBlock,
			EndBlock:            &currentBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
		&tbtc.DepositRevealedEvent{
			// Targets a different vault so it is filtered out immediately
			// without needing a matching deposit request/funding tx.
			BlockNumber:         filterStartBlock,
			WalletPublicKeyHash: walletPublicKeyHash,
			Vault: &[]chain.Address{chain.Address(
				"0xOtherVaultAddress1234567890abcdef123456789012",
			)}[0],
		},
	); err != nil {
		t.Fatal(err)
	}

	task := NewReservationAcceptanceTask(lc, btcChain)
	recorder := newFakeMetricsRecorder()
	task.setMetricsRecorder(recorder)

	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, ok := recorder.calls["wallet_reservations_count"]; !ok {
		t.Error("expected wallet_reservations_count gauge to be recorded")
	} else if got != 2 {
		t.Errorf("expected wallet_reservations_count = 2, got %v", got)
	}

	if _, ok := recorder.calls["active_reservations_count"]; !ok {
		t.Error("expected active_reservations_count gauge to be recorded")
	}
	if _, ok := recorder.calls["max_active_reservations"]; !ok {
		t.Error("expected max_active_reservations gauge to be recorded")
	}
}
