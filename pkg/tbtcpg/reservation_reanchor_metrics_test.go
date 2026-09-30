package tbtcpg

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// scanningLocalChain wraps LocalChain with a PastNewWalletRegisteredEvents
// call counter so a cache-vs-scan test can observe how many registration
// scans the re-anchor target search actually triggered.
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

// TestReservationReanchorTask_TargetSearch_CachesAcrossRuns is a
// regression test for the target-wallet cache: once a re-anchor target
// wallet has been selected for a source wallet, the next Run pass must
// reuse the cached target -- validating only that one wallet via
// GetWallet -- instead of repeating the O(W) GetWallet-per-registration
// scan. A cached target whose count headroom has since dropped must be
// evicted so that pass performs a fresh scan, and that fresh scan must
// pick the most recently registered Live wallet that still has room.
func TestReservationReanchorTask_TargetSearch_CachesAcrossRuns(t *testing.T) {
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

	task := NewReservationReanchorTask(lc, NewLocalBitcoinChain())

	// Each pass gets its own search, as each Run does.
	find := func() ([20]byte, error) {
		return task.newReanchorTargetSearch(logger, walletA, 5, 100000000).find(100000)
	}

	// First pass: no cache, must scan the registration events.
	got, err := find()
	if err != nil {
		t.Fatalf("first pass: unexpected error: %v", err)
	}
	if got != walletC {
		t.Fatalf("first pass: expected target wallet C [%x], got [%x]", walletC, got)
	}
	if lc.calls != 1 {
		t.Fatalf(
			"first pass should have triggered exactly 1 registration-event "+
				"scan, got %d",
			lc.calls,
		)
	}

	// Second pass: cache hit. No new registration-event scan.
	got, err = find()
	if err != nil {
		t.Fatalf("second pass (cache hit): unexpected error: %v", err)
	}
	if got != walletC {
		t.Fatalf(
			"second pass: expected the cached target wallet C [%x], got [%x]",
			walletC,
			got,
		)
	}
	if lc.calls != 1 {
		t.Fatalf(
			"cache hit must not trigger another registration-event scan; "+
				"observed %d total scans after 2 passes (want 1)",
			lc.calls,
		)
	}

	// Third pass: cached target C now has no count headroom (count =
	// cap), which forces eviction and a fresh scan. The fresh scan walks
	// C and B newest-first; C was already read this pass and is
	// excluded, B has headroom -- so B wins.
	walletKeys := make([]*big.Int, 5)
	for i := range walletKeys {
		walletKeys[i] = big.NewInt(int64(1000 + i))
	}
	lc.SetWalletReservations(walletC, walletKeys)
	got, err = find()
	if err != nil {
		t.Fatalf("third pass (evict + rescan): unexpected error: %v", err)
	}
	if got != walletB {
		t.Fatalf(
			"third pass: expected the fresh-scan target wallet B [%x] "+
				"after the cached C lost headroom, got [%x]",
			walletB,
			got,
		)
	}
	if lc.calls != 2 {
		t.Fatalf(
			"eviction must trigger a fresh scan; observed %d total "+
				"scans after 3 passes (want 2)",
			lc.calls,
		)
	}
}

// targetSearchCountingChain counts the chain reads the re-anchor target
// search makes.
type targetSearchCountingChain struct {
	*LocalChain

	registrationScans int
	walletReads       map[[20]byte]int
	countReads        map[[20]byte]int
}

func (c *targetSearchCountingChain) PastNewWalletRegisteredEvents(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	c.registrationScans++
	return c.LocalChain.PastNewWalletRegisteredEvents(filter)
}

func (c *targetSearchCountingChain) GetWallet(
	walletPublicKeyHash [20]byte,
) (*tbtc.WalletChainData, error) {
	c.walletReads[walletPublicKeyHash]++
	return c.LocalChain.GetWallet(walletPublicKeyHash)
}

func (c *targetSearchCountingChain) WalletReservationsCount(
	walletPublicKeyHash [20]byte,
) (uint32, error) {
	c.countReads[walletPublicKeyHash]++
	return c.LocalChain.WalletReservationsCount(walletPublicKeyHash)
}

// TestReservationReanchorTask_TargetSearch_ReadsOncePerRun pins that when
// no Live wallet has headroom, a Run pass with several reservations
// searches for a target once: one bounded and one unbounded registration
// scan, one state read per wallet, one count read for a count-capped
// wallet, and no capacity reads at all for a reservation whose anchor is
// at least one already found to fit nowhere. Saturated caps are exactly when this
// search repeats, so the pass must not repeat it per reservation.
func TestReservationReanchorTask_TargetSearch_ReadsOncePerRun(t *testing.T) {
	source := [20]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	countFull := [20]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	amountFull := [20]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}
	oldCountFull := [20]byte{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}

	lc := NewLocalChain()
	chain := &targetSearchCountingChain{
		LocalChain:  lc,
		walletReads: make(map[[20]byte]int),
		countReads:  make(map[[20]byte]int),
	}

	// A current block past the look-back window makes the bounded scan
	// start above zero, so the unbounded fallback also runs.
	blockCounter := NewMockBlockCounter()
	blockCounter.SetCurrentBlock(300000)
	lc.SetBlockCounter(blockCounter)
	recent := &tbtc.NewWalletRegisteredEventFilter{
		StartBlock: 300000 - ReservationReanchorLookBackBlocks,
	}
	all := &tbtc.NewWalletRegisteredEventFilter{StartBlock: 0}
	for _, registration := range []struct {
		filter *tbtc.NewWalletRegisteredEventFilter
		wallet [20]byte
	}{
		{recent, countFull},
		{recent, amountFull},
		{all, oldCountFull},
		{all, countFull},
		{all, amountFull},
	} {
		if err := lc.AddPastNewWalletRegisteredEvent(
			registration.filter,
			&tbtc.NewWalletRegisteredEvent{WalletPublicKeyHash: registration.wallet},
		); err != nil {
			t.Fatal(err)
		}
	}

	lc.SetWallet(source, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	for _, wallet := range [][20]byte{countFull, amountFull, oldCountFull} {
		lc.SetWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateLive})
	}
	lc.SetLiveWalletsCount(3)
	lc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationTxMaxFee:      10000,
		MaxReservationsPerWallet: 2,
		ReservationActionTimeout: 86400,
	})
	lc.SetReservationCaps(300000, 0)

	reservation := func(key int64, wallet [20]byte, value int64) *big.Int {
		reservationKey := big.NewInt(key)
		lc.SetReservation(reservationKey, &tbtc.Reservation{
			WalletPublicKeyHash: wallet,
			AnchorUtxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{
					TransactionHash: bitcoin.Hash{byte(key)},
				},
				Value: value,
			},
			State: tbtc.ReservationStateActive,
		})
		return reservationKey
	}
	// countFull and oldCountFull are at the count cap; amountFull has
	// count room but 250000 reserved, so no anchor of 200000 or more fits.
	lc.SetWalletReservations(countFull, []*big.Int{
		reservation(11, countFull, 1), reservation(12, countFull, 1),
	})
	lc.SetWalletReservations(oldCountFull, []*big.Int{
		reservation(13, oldCountFull, 1), reservation(14, oldCountFull, 1),
	})
	lc.SetWalletReservations(amountFull, []*big.Int{
		reservation(15, amountFull, 250000),
	})
	// The 300000 anchor is ruled out by the 200000 one without any read;
	// the smaller 100000 anchor re-checks only amountFull, since the
	// count-capped wallets are ruled out for every anchor.
	lc.SetWalletReservations(source, []*big.Int{
		reservation(21, source, 200000),
		reservation(22, source, 300000),
		reservation(23, source, 100000),
	})

	task := NewReservationReanchorTask(chain, NewLocalBitcoinChain())
	proposal, ok, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: source,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || proposal != nil {
		t.Fatalf("expected no proposal, got ok=%v proposal=%v", ok, proposal)
	}

	if chain.registrationScans != 2 {
		t.Errorf(
			"expected one bounded and one unbounded registration scan, got %d",
			chain.registrationScans,
		)
	}
	expectedCountReads := map[[20]byte]int{
		countFull:    1,
		oldCountFull: 1,
		amountFull:   2,
	}
	for wallet, expected := range expectedCountReads {
		if got := chain.walletReads[wallet]; got != 1 {
			t.Errorf("expected wallet [%x] state to be read once, got %d", wallet, got)
		}
		if got := chain.countReads[wallet]; got != expected {
			t.Errorf(
				"expected wallet [%x] count to be read %d times, got %d",
				wallet,
				expected,
				got,
			)
		}
	}
	if got := len(lc.GetReservationReanchorRequestAttempts()); got != 0 {
		t.Errorf("expected no re-anchor request, got %d", got)
	}
}
