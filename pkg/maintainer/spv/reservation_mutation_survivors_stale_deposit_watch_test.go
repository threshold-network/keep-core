package spv

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationStaleDepositWatcher_MutationSurvivors_NoReadsAtExactDeadline
// pins the refund-deadline gate: until the deadline has STRICTLY passed in
// chain time the contract rejects the notification, so a chain head
// exactly on the deadline must stay in-memory - no chain reads, no
// notification.
func TestReservationStaleDepositWatcher_MutationSurvivors_NoReadsAtExactDeadline(
	t *testing.T,
) {
	inner := newLocalChain()
	spvChain := &staleReadCountingChain{Chain: inner}

	key := reservationDepositKey(0xB100)
	inner.setReservedDeposit(key, walletPKH(), true)
	inner.setLatestBlockTimestamp(time.Unix(10_000, 0))

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)

	res, err := watcher.CheckStaleReservedDeposit(key, 10_000, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if spvChain.reservedDepositWalletCalls != 0 ||
		spvChain.getReservationCalls != 0 ||
		spvChain.getReservationActionCalls != 0 {
		t.Fatalf(
			"expected zero chain reads at the exact refund deadline, got "+
				"wallet=%d reservation=%d action=%d",
			spvChain.reservedDepositWalletCalls,
			spvChain.getReservationCalls,
			spvChain.getReservationActionCalls,
		)
	}
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected no notification at the exact deadline, got %d", len(calls))
	}
}

// TestReservationStaleDepositWatcher_MutationSurvivors_RenotifiesAtExactBackoffBoundary
// pins the renotify backoff as a minimum: a submitted notification whose
// record has still not cleared once the interval has elapsed down to the
// second must be retried.
func TestReservationStaleDepositWatcher_MutationSurvivors_RenotifiesAtExactBackoffBoundary(
	t *testing.T,
) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB101)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})
	spvChain.setLatestBlockTimestamp(time.Unix(5_000, 0))

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)

	const notifiedAt uint32 = 5_000
	if res, err := watcher.CheckStaleReservedDeposit(key, 100, notifiedAt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf("expected the first notification, got %d", len(calls))
	}

	interval := uint32(actionTimeoutRenotifyInterval.Seconds())
	if _, err := watcher.CheckStaleReservedDeposit(key, 100, notifiedAt+interval); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 2 {
		t.Fatalf(
			"expected a retry notification once the renotify interval had "+
				"exactly elapsed, got %d total submissions",
			len(calls),
		)
	}
}

// TestReservationStaleDepositWatcher_MutationSurvivors_FirstAttemptStaggerBoundary
// pins the first-attempt stagger gate at its exact boundary: the guard is
// inclusive (keep while the head time has not passed deadline + offset),
// so a head time landing exactly on deadline + offset still holds the
// first attempt back.
func TestReservationStaleDepositWatcher_MutationSurvivors_FirstAttemptStaggerBoundary(
	t *testing.T,
) {
	spvChain := newLocalChain()

	operatorAddress := common.HexToAddress("0x00000000000000000000000000000000000000b2")

	// Pick a deposit key whose stagger offset is non-zero for this
	// operator, so the boundary is strictly past the bare deadline. The
	// search is bounded: a mutation that disables staggering would
	// otherwise spin here forever instead of failing the test.
	key := reservationDepositKey(0xB102)
	var offset uint32
	var found bool
	for i := uint64(0); i < 64 && !found; i++ {
		candidate := reservationDepositKey(0xB102 + i)
		candidateOffset := reservationOperatorStaggerOffset(
			operatorAddress,
			candidate.String(),
			actionTimeoutRenotifyInterval,
		)
		if candidateOffset != 0 {
			key, offset, found = candidate, candidateOffset, true
		}
	}
	if !found {
		t.Fatal("expected a non-zero stagger offset within the first 64 deposit keys")
	}

	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})
	const deadline uint32 = 100
	spvChain.setLatestBlockTimestamp(
		time.Unix(int64(deadline+offset), 0),
	)

	watcher := NewReservationStaleDepositWatcher(spvChain, operatorAddress, 0)

	res, err := watcher.CheckStaleReservedDeposit(key, deadline, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"expected the first attempt to stay staggered at deadline+offset "+
				"(offset %ds), got %d submissions",
			offset,
			len(calls),
		)
	}
	if _, attempted := watcher.attempted[key.String()]; attempted {
		t.Fatal("expected the held-back first attempt not to be recorded as attempted")
	}
}

// TestReservationStaleDepositWatcher_MutationSurvivors_SingleBlockScanRange
// pins the empty-range guard on the reveal scan: a scan range of exactly
// one block is a real scan. Skipping it would drop the reveal that lands
// on the one-block window between two ticks' cursors.
func TestReservationStaleDepositWatcher_MutationSurvivors_SingleBlockScanRange(
	t *testing.T,
) {
	spvChain := newLocalChain()

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	// reservationScanRange stops reservationEventScanConfirmationBlocks
	// behind the tip, so the confirmed end block is 1000-12 = 988. A
	// cursor of 987 makes the next tick cover exactly [988..988].
	endBlock := currentBlock - reservationEventScanConfirmationBlocks
	fundingTxHash := bitcoin.Hash{0xb3}
	if err := spvChain.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{StartBlock: endBlock, EndBlock: &endBlock},
		&tbtc.DepositRevealedEvent{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: 0,
			Vault:              &vault,
			BlockNumber:        endBlock,
			RefundLocktime:     locktimeForDeadline(50_000),
		},
	); err != nil {
		t.Fatal(err)
	}

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	watcher.lastSeenBlock = endBlock - 1

	tracked, ok := watcher.pollTick(1_000)
	if !ok {
		t.Fatal("expected the tick's scans to succeed")
	}
	if tracked != 1 {
		t.Fatalf(
			"expected the reveal in the single-block scan range to be "+
				"tracked, got %d",
			tracked,
		)
	}
}

// TestReservationStaleDepositWatcher_MutationSurvivors_BudgetAppliesAtExactDeadline
// pins the per-tick budget gate as inclusive: a tracked deposit whose
// refund deadline equals the chain head is already due its check, so it
// consumes budget and, once the budget is spent, the tick stops and
// records the resume cursor instead of walking the whole tracked set.
func TestReservationStaleDepositWatcher_MutationSurvivors_BudgetAppliesAtExactDeadline(
	t *testing.T,
) {
	spvChain := newLocalChain()

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)
	spvChain.setReservationParameters(&tbtc.ReservationParameters{})

	const deadline uint32 = 10_000
	spvChain.setLatestBlockTimestamp(time.Unix(int64(deadline), 0))

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	// The confirmed tip is the end of the scan window; a cursor there
	// makes the reveal scan empty, leaving the budget logic as the only
	// thing this tick exercises.
	watcher.lastSeenBlock = currentBlock

	count := reservationStaleDepositMaxChecksPerTick + 1
	for i := range count {
		key := reservationDepositKey(uint64(0xB200 + i))
		watcher.pending[key.String()] = &trackedStaleDeposit{
			depositKey:     key,
			refundDeadline: deadline,
		}
	}

	tracked, ok := watcher.pollTick(deadline)
	if !ok {
		t.Fatal("expected the tick's scans to succeed")
	}
	if tracked != count {
		t.Fatalf("expected all %d deposits to stay tracked, got %d", count, tracked)
	}

	if watcher.checkCursor == "" {
		t.Fatal(
			"expected the budget to stop the tick after " +
				"reservationStaleDepositMaxChecksPerTick deposits due at " +
				"the exact deadline and record a resume cursor",
		)
	}

	keys := make([]string, 0, count)
	for key := range watcher.pending {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if want := keys[reservationStaleDepositMaxChecksPerTick]; watcher.checkCursor != want {
		t.Fatalf(
			"expected the resume cursor to be the first deposit past the "+
				"budget (%s), got %s",
			want,
			watcher.checkCursor,
		)
	}
}

// TestReservationStaleDepositWatcher_MutationSurvivors_NotifiedAtEqualitySkipsBudget
// pins the notified-at lower bound of the budget backoff as inclusive: a
// deposit whose notifiedAt equals the tick's now is inside the backoff
// window (the subtraction is exactly zero), so the tick must skip it
// without spending the per-tick budget or reading the chain.
func TestReservationStaleDepositWatcher_MutationSurvivors_NotifiedAtEqualitySkipsBudget(
	t *testing.T,
) {
	inner := newLocalChain()
	spvChain := &staleReadCountingChain{Chain: inner}

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)
	inner.setReservationParameters(&tbtc.ReservationParameters{})
	inner.setLatestBlockTimestamp(time.Unix(10_001, 0))

	const now uint32 = 5_000
	const deadline uint32 = 10_000

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	watcher.lastSeenBlock = currentBlock

	key := reservationDepositKey(0xB300)
	watcher.pending[key.String()] = &trackedStaleDeposit{
		depositKey:     key,
		refundDeadline: deadline,
	}
	watcher.notifiedAt[key.String()] = now

	if _, ok := watcher.pollTick(now); !ok {
		t.Fatal("expected the tick's scans to succeed")
	}

	if spvChain.reservedDepositWalletCalls != 0 {
		t.Fatalf(
			"expected a deposit notified at exactly the tick's now to stay "+
				"inside the backoff, got %d wallet reads",
			spvChain.reservedDepositWalletCalls,
		)
	}
	if watcher.checkCursor != "" {
		t.Fatalf(
			"expected the skipped deposit not to consume the per-tick budget, got cursor %q",
			watcher.checkCursor,
		)
	}
}

// TestReservationStaleDepositWatcher_MutationSurvivors_RenotifiesAtExactBudgetBoundary
// pins the budget-loop renotify comparison at its exact boundary: once the
// interval has elapsed down to the second the deposit is eligible for a
// retry, so the budget loop must let it through to the check that
// resubmits the notification.
func TestReservationStaleDepositWatcher_MutationSurvivors_RenotifiesAtExactBudgetBoundary(
	t *testing.T,
) {
	spvChain := newLocalChain()

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)
	spvChain.setReservationParameters(&tbtc.ReservationParameters{})
	// Past the deadline and past the largest possible first-attempt
	// stagger offset, so only the renotify backoff decides the outcome.
	spvChain.setLatestBlockTimestamp(time.Unix(10_601, 0))

	const deadline uint32 = 10_000
	const notifiedAt uint32 = 5_000
	now := notifiedAt + uint32(actionTimeoutRenotifyInterval.Seconds())

	key := reservationDepositKey(0xB301)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	watcher.lastSeenBlock = currentBlock
	watcher.pending[key.String()] = &trackedStaleDeposit{
		depositKey:     key,
		refundDeadline: deadline,
	}
	watcher.notifiedAt[key.String()] = notifiedAt

	if _, ok := watcher.pollTick(now); !ok {
		t.Fatal("expected the tick's scans to succeed")
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf(
			"expected a retry notification once the renotify interval had "+
				"exactly elapsed, got %d submissions",
			len(calls),
		)
	}
}

// TestReservationStaleDepositWatcher_MutationSurvivors_ZeroIntervalRejected
// pins the poll-interval guard: a zero interval must be rejected before
// time.NewTicker is reached, because NewTicker(0) panics and would take
// the watcher goroutine down instead of reporting a configuration error.
func TestReservationStaleDepositWatcher_MutationSurvivors_ZeroIntervalRejected(
	t *testing.T,
) {
	watcher := NewReservationStaleDepositWatcher(
		newLocalChain(),
		common.Address{},
		0,
	)

	if err := watcher.Run(context.Background(), 0); err == nil {
		t.Fatal("expected Run to reject a zero poll interval")
	}
}