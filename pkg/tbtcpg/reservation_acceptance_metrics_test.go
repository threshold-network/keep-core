package tbtcpg

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
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

	// The fee-observability gauges are published alongside the
	// occupancy gauges on the same pass; the embedded LocalChain
	// reports no debt and no reserve by default, so their presence
	// (not their value) is the assertion here.
	if _, ok := recorder.calls["reservation_vault_fee_debt_sat"]; !ok {
		t.Error(
			"expected reservation_vault_fee_debt_sat gauge to be recorded",
		)
	}
	if _, ok := recorder.calls["reservation_vault_fee_reserve_tbtc_base_units"]; !ok {
		t.Error(
			"expected reservation_vault_fee_reserve_tbtc_base_units gauge to be recorded",
		)
	}
}

// feeObservabilityChain wraps LocalChain to configure the two
// ReservationVault fee-observability methods the acceptance task
// publishes, mirroring the activeReservationsCapChain override pattern.
type feeObservabilityChain struct {
	*LocalChain

	feeDebtSat    uint64
	feeDebtErr    error
	feeReserve    *big.Int
	feeReserveErr error
}

func (c *feeObservabilityChain) ReservationVaultFeeDebtSat(
	reservationVault chain.Address,
) (uint64, error) {
	return c.feeDebtSat, c.feeDebtErr
}

func (c *feeObservabilityChain) ReservationVaultFeeReserveTbtcBaseUnits(
	reservationVault chain.Address,
) (*big.Int, error) {
	return c.feeReserve, c.feeReserveErr
}

// TestReservationAcceptanceTask_RecordsVaultFeeGauges asserts the two
// ReservationVault fee-observability gauges are published with the
// chain's values: fee debt in satoshi, fee reserve in TBTC base
// units (the raw big.Int value, 1e18 per whole TBTC).
func TestReservationAcceptanceTask_RecordsVaultFeeGauges(t *testing.T) {
	lc := NewLocalChain()
	btcChain := NewLocalBitcoinChain()

	feeChain := &feeObservabilityChain{
		LocalChain: lc,
		feeDebtSat: 12345,
		// 2 TBTC in base units (2 x 1e18).
		feeReserve: new(big.Int).Lsh(big.NewInt(2), 18),
	}

	task := NewReservationAcceptanceTask(feeChain, btcChain)

	// Configure the wallet/vault/event fixture on the underlying
	// LocalChain (the feeObservabilityChain embeds it), then drive an
	// empty scan through the real task path.
	runFeeGaugeFixture(t, lc)
	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	recorder := newFakeMetricsRecorder()
	task.setMetricsRecorder(recorder)

	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, ok := recorder.calls["reservation_vault_fee_debt_sat"]; !ok {
		t.Error("expected reservation_vault_fee_debt_sat gauge to be recorded")
	} else if got != 12345 {
		t.Errorf("expected reservation_vault_fee_debt_sat = 12345, got %v", got)
	}

	// Compute the expected value the same way production does (the
	// task's big.Int -> float64 conversion), so the assertion tracks
	// the exact value the recorder would have received.
	wantReserve, _ := new(big.Int).Lsh(big.NewInt(2), 18).Float64()
	if got, ok := recorder.calls["reservation_vault_fee_reserve_tbtc_base_units"]; !ok {
		t.Error("expected reservation_vault_fee_reserve_tbtc_base_units gauge to be recorded")
	} else if got != wantReserve {
		t.Errorf(
			"expected reservation_vault_fee_reserve_tbtc_base_units = %v, got %v",
			wantReserve,
			got,
		)
	}
}

// TestReservationAcceptanceTask_VaultFeeReadErrorKeepsGauges asserts
// a fee-read error is logged and the gauges keep their previous
// (registered zero) value instead of being published with a garbage
// default, while proposal generation itself still completes.
func TestReservationAcceptanceTask_VaultFeeReadErrorKeepsGauges(t *testing.T) {
	lc := NewLocalChain()
	btcChain := NewLocalBitcoinChain()

	feeChain := &feeObservabilityChain{
		LocalChain:    lc,
		feeDebtErr:    errors.New("in-kind fee debt read failed"),
		feeReserveErr: errors.New("fee reserve read failed"),
	}

	runFeeGaugeFixture(t, lc)

	task := NewReservationAcceptanceTask(feeChain, btcChain)
	recorder := newFakeMetricsRecorder()
	task.setMetricsRecorder(recorder)

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("fee read errors must not abort proposal generation: %v", err)
	}
	if proposal != nil || shouldExecute {
		t.Errorf("expected no proposal, got shouldExecute=%v", shouldExecute)
	}

	if _, ok := recorder.calls["reservation_vault_fee_debt_sat"]; ok {
		t.Error(
			"reservation_vault_fee_debt_sat must not be published on read error",
		)
	}
	if _, ok := recorder.calls["reservation_vault_fee_reserve_tbtc_base_units"]; ok {
		t.Error(
			"reservation_vault_fee_reserve_tbtc_base_units must not be published on read error",
		)
	}

	// The occupancy gauges are published unconditionally and are
	// unaffected by the fee-read errors.
	if _, ok := recorder.calls["wallet_reservations_count"]; !ok {
		t.Error("expected wallet_reservations_count gauge to still be recorded")
	}
}

// TestReservationAcceptanceTask_ZeroVaultAddressZeroesFeeGauges asserts
// an unconfigured (zero-address) reservation vault short-circuits
// findReservationAcceptanceCandidates before any fee-gauge read chain
// call (the wrapper's nonzero fee values below are tripwires:
// publishing them would mean the fee methods ran on the unconfigured
// path), while the two fee gauges are still published as zero: a
// stale nonzero reading carried over from an earlier pass with a
// configured vault is indistinguishable from a real fee debt.
func TestReservationAcceptanceTask_ZeroVaultAddressZeroesFeeGauges(t *testing.T) {
	lc := NewLocalChain()
	btcChain := NewLocalBitcoinChain()

	feeChain := &feeObservabilityChain{
		LocalChain: lc,
		feeDebtSat: 12345,
		// 2 TBTC in base units (2 x 1e18).
		feeReserve: new(big.Int).Lsh(big.NewInt(2), 18),
	}

	runZeroVaultFixture(t, lc)

	task := NewReservationAcceptanceTask(feeChain, btcChain)
	recorder := newFakeMetricsRecorder()
	task.setMetricsRecorder(recorder)

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proposal != nil || shouldExecute {
		t.Errorf("expected no proposal, got shouldExecute=%v", shouldExecute)
	}

	if len(recorder.calls) != 2 {
		t.Errorf(
			"expected only the two zeroed fee gauges for an unconfigured "+
				"vault, got %v",
			recorder.calls,
		)
	}
	if got, ok := recorder.calls["reservation_vault_fee_debt_sat"]; !ok {
		t.Error("expected reservation_vault_fee_debt_sat gauge to be recorded")
	} else if got != 0 {
		t.Errorf("expected reservation_vault_fee_debt_sat = 0, got %v", got)
	}
	if got, ok := recorder.calls["reservation_vault_fee_reserve_tbtc_base_units"]; !ok {
		t.Error("expected reservation_vault_fee_reserve_tbtc_base_units gauge to be recorded")
	} else if got != 0 {
		t.Errorf("expected reservation_vault_fee_reserve_tbtc_base_units = 0, got %v", got)
	}
}

// TestReservationAcceptanceTask_UnconfiguredVaultZeroesFeeGauges is a
// regression test for the stale-gauge leak when the reservation vault
// is cleared: a pass with a configured vault publishes nonzero fee
// gauges, and a later pass with the vault unconfigured must reset both
// gauges to zero rather than leave the last nonzero readings
// published forever.
func TestReservationAcceptanceTask_UnconfiguredVaultZeroesFeeGauges(t *testing.T) {
	lc := NewLocalChain()
	btcChain := NewLocalBitcoinChain()

	task := NewReservationAcceptanceTask(lc, btcChain)
	recorder := newFakeMetricsRecorder()
	task.setMetricsRecorder(recorder)

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	// Pass 1: configured vault, nonzero fee gauges published.
	runFeeGaugeFixture(t, lc)
	lc.SetReservationVaultFeeDebtSat(12345, nil)
	// 2 TBTC in base units (2 x 1e18).
	lc.SetReservationVaultFeeReserveBalance(
		new(big.Int).Lsh(big.NewInt(2), 18),
		nil,
	)

	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("pass 1: unexpected error: %v", err)
	}
	if proposal != nil || shouldExecute {
		t.Fatalf("pass 1: expected no proposal, got shouldExecute=%v", shouldExecute)
	}
	if got := recorder.calls["reservation_vault_fee_debt_sat"]; got != 12345 {
		t.Fatalf("pass 1: expected fee-debt gauge 12345, got %v", got)
	}
	if got := recorder.calls["reservation_vault_fee_reserve_tbtc_base_units"]; got == 0 {
		t.Fatalf("pass 1: expected a nonzero fee-reserve gauge, got 0")
	}

	// Pass 2: the vault address is cleared (zero params -> ""
	// vault), so the unconfigured path must zero both gauges again
	// instead of holding pass 1's readings.
	lc.SetReservationParameters(tbtc.ReservationParameters{})

	proposal, shouldExecute, err = task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	if err != nil {
		t.Fatalf("pass 2: unexpected error: %v", err)
	}
	if proposal != nil || shouldExecute {
		t.Fatalf("pass 2: expected no proposal, got shouldExecute=%v", shouldExecute)
	}
	if got := recorder.calls["reservation_vault_fee_debt_sat"]; got != 0 {
		t.Errorf(
			"pass 2: expected the fee-debt gauge zeroed for an "+
				"unconfigured vault, got %v",
			got,
		)
	}
	if got := recorder.calls["reservation_vault_fee_reserve_tbtc_base_units"]; got != 0 {
		t.Errorf(
			"pass 2: expected the fee-reserve gauge zeroed for an "+
				"unconfigured vault, got %v",
			got,
		)
	}
}

// runFeeGaugeFixture configures a LocalChain with a live wallet, a
// configured (non-zero) reservation vault and a block counter. The
// embedded LocalChain reports no acceptance requests, so Run completes
// with no candidate. Shared by the fee-gauge tests.
func runFeeGaugeFixture(t *testing.T, lc *LocalChain) {
	t.Helper()

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
}

// runZeroVaultFixture configures a LocalChain with a live wallet and a
// zero-address reservation vault, so
// findReservationAcceptanceCandidates returns at the "reservation
// vault not configured" gate before any chain read (including the
// fee reads) or any gauge publication.
func runZeroVaultFixture(t *testing.T, lc *LocalChain) {
	t.Helper()

	walletPublicKeyHash := [20]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

	lc.SetReservationParameters(tbtc.ReservationParameters{
		ReservationVault: chain.Address(
			"0x0000000000000000000000000000000000000000",
		),
	})
	lc.SetWallet(walletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateLive})
}

// --- fetchReservationAcceptanceFundingTxs pipeline tests --------------
//
// fetchReservationAcceptanceFundingTxs is exercised directly, rather than
// through Run(), so these tests can hold each candidate's Bitcoin RPCs
// under precise, deterministic control (artificial concurrency tracking,
// artificial per-candidate latency, and a context that is never
// resolved) without depending on findReservationAcceptanceCandidates'
// unrelated eligibility gates.

// concurrencyTrackingBTCChain wraps LocalBitcoinChain and records, across
// every GetTransaction call, the maximum number that were ever in flight
// at once. GetTransactionConfirmations is left unmodified: it errors
// "transaction not found" for every candidate here (none is registered
// via SetTransactionConfirmations), so no candidate ever becomes eligible
// and the pipeline is forced to walk every one of them -- exactly what
// this test needs to observe the concurrency cap across the whole
// candidate set, uncomplicated by the pipeline's own early-stop
// behavior.
type concurrencyTrackingBTCChain struct {
	*LocalBitcoinChain

	mutex       sync.Mutex
	inFlight    int
	maxInFlight int
}

func (c *concurrencyTrackingBTCChain) GetTransaction(
	transactionHash bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	c.mutex.Lock()
	c.inFlight++
	if c.inFlight > c.maxInFlight {
		c.maxInFlight = c.inFlight
	}
	c.mutex.Unlock()

	// Long enough that sibling goroutines reliably pile up to the worker
	// cap before any of them returns, without making the test slow.
	time.Sleep(20 * time.Millisecond)

	c.mutex.Lock()
	c.inFlight--
	c.mutex.Unlock()

	return &bitcoin.Transaction{}, nil
}

// TestFetchReservationAcceptanceFundingTxs_ConcurrencyCap is a regression
// test asserting that the funding-transaction lookup pipeline never runs
// more than reservationAcceptanceFundingTxLookupWorkers GetTransaction
// calls at once, and that it actually reaches that cap given enough
// pending candidates -- the concurrency property the PR-4324 review's
// validation pass confirmed was already race-free, but which had no test
// coverage of its own.
func TestFetchReservationAcceptanceFundingTxs_ConcurrencyCap(t *testing.T) {
	btcChain := &concurrencyTrackingBTCChain{LocalBitcoinChain: NewLocalBitcoinChain()}
	task := NewReservationAcceptanceTask(nil, btcChain)

	candidateCount := reservationAcceptanceFundingTxLookupWorkers * 3
	candidates := make([]*reservationAcceptanceFundingTxCandidate, candidateCount)
	for i := range candidates {
		candidates[i] = &reservationAcceptanceFundingTxCandidate{
			event:      &tbtc.DepositRevealedEvent{FundingTxHash: bitcoin.Hash{byte(i)}},
			depositKey: big.NewInt(int64(i)),
		}
	}

	pipeline := task.fetchReservationAcceptanceFundingTxs(candidates)
	for i := range candidates {
		pipeline.next(i)
	}

	if btcChain.maxInFlight > reservationAcceptanceFundingTxLookupWorkers {
		t.Errorf(
			"expected at most %d concurrent GetTransaction calls, observed %d",
			reservationAcceptanceFundingTxLookupWorkers,
			btcChain.maxInFlight,
		)
	}
	if btcChain.maxInFlight != reservationAcceptanceFundingTxLookupWorkers {
		t.Errorf(
			"expected the pipeline to actually reach the worker cap of %d "+
				"concurrent calls with %d pending candidates, observed only %d",
			reservationAcceptanceFundingTxLookupWorkers,
			candidateCount,
			btcChain.maxInFlight,
		)
	}
}

// timeoutBTCChain wraps LocalBitcoinChain, returning a valid funding
// transaction for GetTransaction unconditionally (so timeout behavior is
// isolated to GetTransactionConfirmations) but blocking
// GetTransactionConfirmations on blockedHash until the caller's context
// is done, so an unhealthy backend's per-candidate lookup can be
// simulated deterministically instead of by luck; every other hash falls
// through to the embedded LocalBitcoinChain so a sibling candidate can
// still succeed normally in the same test.
type timeoutBTCChain struct {
	*LocalBitcoinChain
	blockedHash bitcoin.Hash
}

func (c *timeoutBTCChain) GetTransaction(
	bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	return &bitcoin.Transaction{}, nil
}

func (c *timeoutBTCChain) GetTransactionConfirmations(
	ctx context.Context,
	transactionHash bitcoin.Hash,
) (uint, error) {
	if transactionHash == c.blockedHash {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	return c.LocalBitcoinChain.GetTransactionConfirmations(ctx, transactionHash)
}

// TestFetchReservationAcceptanceFundingTxs_PerCandidateTimeout is a
// regression test asserting that a candidate whose GetTransactionConfirmations
// call exceeds the per-candidate timeout is reported as a normal,
// non-fatal confirmationsErr on that candidate's own lookup result --
// never a panic, and never an error that aborts the whole pipeline -- and
// that every other candidate's lookup still completes normally.
func TestFetchReservationAcceptanceFundingTxs_PerCandidateTimeout(t *testing.T) {
	timedOutHash := bitcoin.Hash{1}
	btcChain := &timeoutBTCChain{LocalBitcoinChain: NewLocalBitcoinChain(), blockedHash: timedOutHash}
	task := NewReservationAcceptanceTask(nil, btcChain)
	// Shrink the production 30s default so the test can actually wait out
	// a real timeout firing.
	task.fundingTxLookupTimeout = 20 * time.Millisecond

	okHash := bitcoin.Hash{2}
	btcChain.SetTransactionConfirmations(okHash, 12)

	candidates := []*reservationAcceptanceFundingTxCandidate{
		{
			event:      &tbtc.DepositRevealedEvent{FundingTxHash: timedOutHash},
			depositKey: big.NewInt(1),
		},
		{
			event:      &tbtc.DepositRevealedEvent{FundingTxHash: okHash},
			depositKey: big.NewInt(2),
		},
	}

	pipeline := task.fetchReservationAcceptanceFundingTxs(candidates)

	timedOut := pipeline.next(0)
	if timedOut.fundingTxErr != nil {
		t.Fatalf("expected funding tx lookup to succeed, got: %v", timedOut.fundingTxErr)
	}
	if timedOut.confirmationsErr == nil {
		t.Fatal("expected the timed-out candidate to carry a non-nil confirmationsErr")
	}
	if !errors.Is(timedOut.confirmationsErr, context.DeadlineExceeded) {
		t.Errorf(
			"expected confirmationsErr to be context.DeadlineExceeded, got: %v",
			timedOut.confirmationsErr,
		)
	}

	ok := pipeline.next(1)
	if ok.confirmationsErr != nil {
		t.Fatalf(
			"expected the second candidate's lookup to succeed despite "+
				"the first timing out, got: %v",
			ok.confirmationsErr,
		)
	}
	if ok.confirmations != 12 {
		t.Errorf("expected confirmations = 12, got %d", ok.confirmations)
	}
}

// latencyBTCChain wraps LocalBitcoinChain, sleeping a configured per-hash
// delay in GetTransaction before delegating to the embedded
// implementation, so a candidate's fetch latency can be controlled
// independently of its position in the candidate slice.
type latencyBTCChain struct {
	*LocalBitcoinChain
	delays map[bitcoin.Hash]time.Duration
}

func (c *latencyBTCChain) GetTransaction(
	transactionHash bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	if d, ok := c.delays[transactionHash]; ok {
		time.Sleep(d)
	}
	return c.LocalBitcoinChain.GetTransaction(transactionHash)
}

// TestFetchReservationAcceptanceFundingTxs_OldestFirstOrderingPreserved is
// a regression test asserting that next(i) always returns candidate i's
// own lookup result, even when a later (in candidate order)
// candidate's fetch races ahead and completes first -- the property
// Run's in-order candidate loop
// depends on to return the correct candidate regardless of which
// underlying network call happens to finish first.
func TestFetchReservationAcceptanceFundingTxs_OldestFirstOrderingPreserved(t *testing.T) {
	oldestHash := bitcoin.Hash{1}
	newestHash := bitcoin.Hash{2}

	btcChain := &latencyBTCChain{
		LocalBitcoinChain: NewLocalBitcoinChain(),
		delays: map[bitcoin.Hash]time.Duration{
			// The oldest (index 0) candidate is deliberately slow so the
			// newest (index 1) candidate's fetch -- with no delay --
			// completes first in wall-clock time.
			oldestHash: 50 * time.Millisecond,
		},
	}
	btcChain.SetTransaction(oldestHash, &bitcoin.Transaction{})
	btcChain.SetTransaction(newestHash, &bitcoin.Transaction{})
	// Distinct confirmation counts double as a marker proving next(i)
	// returns index i's own result rather than whichever fetch happened
	// to finish first.
	btcChain.SetTransactionConfirmations(oldestHash, 111)
	btcChain.SetTransactionConfirmations(newestHash, 222)

	task := NewReservationAcceptanceTask(nil, btcChain)
	candidates := []*reservationAcceptanceFundingTxCandidate{
		{event: &tbtc.DepositRevealedEvent{FundingTxHash: oldestHash}, depositKey: big.NewInt(1)},
		{event: &tbtc.DepositRevealedEvent{FundingTxHash: newestHash}, depositKey: big.NewInt(2)},
	}

	pipeline := task.fetchReservationAcceptanceFundingTxs(candidates)

	oldest := pipeline.next(0)
	if oldest.confirmations != 111 {
		t.Errorf(
			"expected index 0 to carry the oldest candidate's confirmations (111), got %d",
			oldest.confirmations,
		)
	}

	newest := pipeline.next(1)
	if newest.confirmations != 222 {
		t.Errorf(
			"expected index 1 to carry the newest candidate's confirmations (222), got %d",
			newest.confirmations,
		)
	}
}

// countingBTCChain wraps LocalBitcoinChain, counting GetTransaction calls
// and returning a fixed valid transaction for any hash. Every hash other
// than fastHash sleeps slowDelay before returning, so those candidates'
// dispatch slots are still held (not yet released back to the pipeline's
// dispatcher) by the time the test's assertions run -- fastHash returns
// immediately so the test can observe its own result without waiting.
type countingBTCChain struct {
	*LocalBitcoinChain

	fastHash  bitcoin.Hash
	slowDelay time.Duration

	mutex sync.Mutex
	calls int
}

func (c *countingBTCChain) GetTransaction(
	transactionHash bitcoin.Hash,
) (*bitcoin.Transaction, error) {
	c.mutex.Lock()
	c.calls++
	c.mutex.Unlock()

	if transactionHash != c.fastHash {
		time.Sleep(c.slowDelay)
	}
	return &bitcoin.Transaction{}, nil
}

func (c *countingBTCChain) callCount() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.calls
}

// TestFetchReservationAcceptanceFundingTxs_StopsAfterEarlyMatch is a
// regression test for the eager-fetch cost the PR-4324 review flagged: a
// caller that only needs the first candidate's result and then
// calls stop() must not cause every remaining candidate's Bitcoin RPCs to
// be dispatched, even with a full maxReservationAcceptanceCandidatesPerRun
// backlog pending. Before the fix, this scenario issued a GetTransaction
// call for every one of the 50 candidates; after it, dispatch stops at
// roughly the worker cap.
func TestFetchReservationAcceptanceFundingTxs_StopsAfterEarlyMatch(t *testing.T) {
	candidateCount := maxReservationAcceptanceCandidatesPerRun
	fastHash := bitcoin.Hash{0}

	btcChain := &countingBTCChain{
		LocalBitcoinChain: NewLocalBitcoinChain(),
		fastHash:          fastHash,
		// Long enough that every candidate but the fast (index 0) one is
		// still sleeping -- and so has not yet released its dispatch slot
		// -- by the time next(0) returns and stop() runs below, so the
		// dispatcher cannot race ahead and start a second batch before it
		// observes stop().
		slowDelay: 200 * time.Millisecond,
	}
	task := NewReservationAcceptanceTask(nil, btcChain)

	candidates := make([]*reservationAcceptanceFundingTxCandidate, candidateCount)
	candidates[0] = &reservationAcceptanceFundingTxCandidate{
		event:      &tbtc.DepositRevealedEvent{FundingTxHash: fastHash},
		depositKey: big.NewInt(0),
	}
	for i := 1; i < candidateCount; i++ {
		candidates[i] = &reservationAcceptanceFundingTxCandidate{
			event:      &tbtc.DepositRevealedEvent{FundingTxHash: bitcoin.Hash{byte(i)}},
			depositKey: big.NewInt(int64(i)),
		}
	}

	pipeline := task.fetchReservationAcceptanceFundingTxs(candidates)
	pipeline.next(0) // the caller determined the oldest candidate is a full match
	pipeline.stop()

	// callCount is incremented at the start of GetTransaction, before its
	// artificial delay, so a short settle here just covers the scheduling
	// gap between a goroutine being launched and actually running --
	// nowhere near long enough for a slow candidate to finish and free a
	// slot the (already-stopped) dispatcher could reuse.
	time.Sleep(20 * time.Millisecond)

	if got := btcChain.callCount(); got >= candidateCount {
		t.Errorf(
			"expected stop() to prevent every one of the %d pending "+
				"candidates from being fetched, but observed %d GetTransaction calls",
			candidateCount,
			got,
		)
	}
	// Allow modest scheduling slack above the exact worker cap: index 0's
	// own slot release races (harmlessly) against the dispatcher noticing
	// stop(), so an occasional one-off extra dispatch is expected, not a
	// bug.
	if maxExpected := reservationAcceptanceFundingTxLookupWorkers * 2; btcChain.callCount() > maxExpected {
		t.Errorf(
			"expected roughly the worker cap of %d GetTransaction calls "+
				"after an early stop, observed %d",
			reservationAcceptanceFundingTxLookupWorkers,
			btcChain.callCount(),
		)
	}
}
