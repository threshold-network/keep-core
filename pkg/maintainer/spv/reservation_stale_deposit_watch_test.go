package spv

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/go-test/deep"
)

// reservationDepositKey returns a big.Int constructed from a uint64 to act
// as a reserved-deposit identifier in the stale-deposit watcher tests.
func reservationDepositKey(low uint64) *big.Int {
	return new(big.Int).SetUint64(low)
}

// reservationActionTimeout is the fixed action timeout used by the tests.
// It is large enough to keep the timeout ordering robust against any
// timestamp arithmetic in the watcher.
const reservationActionTimeout uint32 = 3600

func seedPastDepositRevealedEvent(
	t *testing.T,
	spvChain *localChain,
	wallet [20]byte,
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
	currentBlock uint64,
) {
	t.Helper()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)

	startBlock := uint64(0)
	if currentBlock > reservationDefaultLookBackBlocks {
		startBlock = currentBlock - reservationDefaultLookBackBlocks
	}
	endBlock := currentBlock
	if err := spvChain.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          startBlock,
			EndBlock:            &endBlock,
			WalletPublicKeyHash: [][20]byte{wallet},
		},
		&tbtc.DepositRevealedEvent{
			FundingTxHash:       fundingTxHash,
			FundingOutputIndex:  fundingOutputIndex,
			WalletPublicKeyHash: wallet,
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestReservationStaleDepositWatcher_NonReservedDepositIsSkipped(t *testing.T) {
	spvChain := newLocalChain()

	// Deposit is NOT booked as reserved.
	spvChain.setReservedDeposit(reservationDepositKey(0xB001), walletPKH(), false)

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(reservationDepositKey(0xB001), 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionDrop {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionDrop, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("non-reserved deposit must not notify, got %d calls", len(calls))
	}
}

// TestReservationStaleDepositWatcher_LiveWalletIsKeptNotDropped verifies
// that a reserved deposit assigned to a Live wallet does not notify and
// resolves to Keep, not Drop: the wallet may still transition away from
// Live (e.g. MovingFunds/Closing/Terminated) before anchoring, and the
// poller's forward-only scan cursor means a deposit dropped here could
// never re-enter tracking to be caught later.
func TestReservationStaleDepositWatcher_LiveWalletIsKeptNotDropped(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB002)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateLive,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 10_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("live wallet must not trigger stale notification, got %d calls", len(calls))
	}
}

func TestReservationStaleDepositWatcher_NotifiesAfterTimeout(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB003)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
	})

	// Inject the acceptance (nonce 1) action with a deadline well below
	// `now`.
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	// now (5_000) > action.TimeoutAt (100). NotifyStaleReservedDeposit
	// is submitted, but the resolution is Keep, not Notified: submission
	// alone is not proof the transaction mined (see
	// TestReservationStaleDepositWatcher_NotifiedConfirmedOnRecheckIsRetired
	// for the confirmation path).
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	calls := spvChain.getSubmittedStaleReservedDeposits()
	if len(calls) != 1 {
		t.Fatalf("expected one stale notification, got %d", len(calls))
	}
	if diff := deep.Equal(key, calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

func TestReservationStaleDepositWatcher_DoesNotNotifyBeforeTimeout(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB004)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
	})

	// Action has a deadline of 10_000; we ask the watcher to evaluate at
	// now=5_000, which is before the deadline.
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 10_000,
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("action not yet timed out; expected zero notifications, got %d", len(calls))
	}
}

func TestReservationStaleDepositWatcher_SettledActionIsSkipped(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB005)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
	})

	// Action is already settled (no longer pending). The watcher must skip
	// the stale notification even though the wall clock has passed the
	// deadline.
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateSettled,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionDrop {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionDrop, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("settled action must skip stale notification, got %d calls", len(calls))
	}
}

func TestReservationStaleDepositWatcher_ZeroWalletSkips(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB006)
	spvChain.setReservedDeposit(key, [20]byte{}, true)

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionDrop {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionDrop, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("zero-wallet deposit must skip, got %d calls", len(calls))
	}
}

// TestReservationStaleDepositWatcher_NotifiedConfirmedOnRecheckIsRetired
// verifies Finding 6's confirm-before-terminal contract: submitting
// NotifyStaleReservedDeposit resolves Keep, not Notified, and only a
// LATER call that observes the reservation actually reporting
// ReservationStateClosed on-chain retires the deposit as Notified. This
// matters because NotifyStaleReservedDeposit's generated chain binding
// returns as soon as the transaction is submitted, not once it mines,
// so treating submission itself as terminal would let a
// dropped-or-reverted transaction silently and permanently lose the
// deposit (the poller's DepositRevealed scan is forward-only and would
// never re-discover it).
func TestReservationStaleDepositWatcher_NotifiedConfirmedOnRecheckIsRetired(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB007)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})

	// First tick crosses the deadline: NotifyStaleReservedDeposit is
	// submitted, but the resolution is Keep - submission alone is not
	// proof the notification landed.
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf("expected exactly one notification submitted, got %d", len(calls))
	}

	// The notify transaction mines: the reservation is released back to
	// the default sweep path, observable as ReservationStateClosed.
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
		State:        tbtc.ReservationStateClosed,
	})

	// A later tick observes the confirmed on-chain state and only now
	// retires the deposit.
	res, err = watcher.CheckStaleReservedDeposit(key, 5_060)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionNotified {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionNotified, res)
	}

	// No renotification was submitted while confirming.
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf("expected still exactly one notification submitted, got %d", len(calls))
	}
}

// TestReservationStaleDepositWatcher_NotifiedNotConfirmedIsRetried covers
// the other half of Finding 6's contract: if a later tick's re-check
// does NOT observe ReservationStateClosed, the deposit must stay tracked
// (resolution Keep, never Drop or Notified) rather than being silently
// and permanently lost, and once actionTimeoutRenotifyInterval elapses
// without confirmation the notification is retried.
func TestReservationStaleDepositWatcher_NotifiedNotConfirmedIsRetried(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB017)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})

	// First tick: submit the notification, awaiting confirmation.
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	// A tick shortly after, still within the renotify grace window, must
	// neither resubmit nor evict: the reservation is still Pending (the
	// notify transaction has not been observed to take effect), so the
	// deposit stays tracked rather than being lost.
	res, err = watcher.CheckStaleReservedDeposit(key, 5_060)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf("expected no renotification within the grace window, got %d calls", len(calls))
	}

	// The reservation never actually left Pending, meaning the prior
	// notification was dropped or reverted. Once
	// actionTimeoutRenotifyInterval elapses, the watcher must retry
	// rather than treat the deposit as permanently resolved or lose it.
	renotifyAfter := 5_000 + uint32(actionTimeoutRenotifyInterval.Seconds()) + 1
	res, err = watcher.CheckStaleReservedDeposit(key, renotifyAfter)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 2 {
		t.Fatalf(
			"expected the notification to be retried once the grace window elapsed, got %d calls",
			len(calls),
		)
	}
}

// TestReservationStaleDepositWatcher_RenotifyBackoffSurvivesNowBeforeNotifiedAt
// is a regression test for an unguarded uint32 subtraction: now is a
// caller-supplied, not-guaranteed-monotonic tick token (see
// getWalletForTick's doc comment), so a tick whose now is smaller than the
// deposit's recorded notifiedAt must not underflow now-notifiedAt to a huge
// value and treat the backoff window as already elapsed - that would
// resubmit NotifyStaleReservedDeposit immediately instead of waiting out
// actionTimeoutRenotifyInterval.
func TestReservationStaleDepositWatcher_RenotifyBackoffSurvivesNowBeforeNotifiedAt(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB018)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})

	// First tick: submit the notification, recording notifiedAt = 5_000.
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	// A later call with now < notifiedAt (e.g. a clock adjustment, or an
	// out-of-order tick). Without the now < notifiedAt guard,
	// now-notifiedAt underflows to approximately 2^32 and is never less
	// than the renotify interval, so the code falls through and
	// resubmits immediately. The guard must keep this call in the
	// backoff window instead.
	res, err = watcher.CheckStaleReservedDeposit(key, 4_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf(
			"expected now < notifiedAt to be treated as still within the "+
				"backoff window (no resubmission), got %d total submissions",
			len(calls),
		)
	}
}

func TestReservationStaleDepositWatcher_NilDepositKeyError(t *testing.T) {
	spvChain := newLocalChain()

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(nil, 5_000)
	if err == nil {
		t.Fatal("expected error for nil deposit key, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}
}

func TestReservationStaleDepositWatcher_IsReservedDepositChainError(t *testing.T) {
	spvChain := newLocalChain()

	spvChain.isReservedDepositErr = fmt.Errorf("rpc unavailable")

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(reservationDepositKey(0xB010), 5_000)
	if err == nil {
		t.Fatal("expected error when IsReservedDeposit fails, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(calls))
	}
}

func TestReservationStaleDepositWatcher_ReservedDepositWalletChainError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB011)
	spvChain.setReservedDeposit(key, walletPKH(), true)
	spvChain.reservedDepositWalletErr = fmt.Errorf("rpc unavailable")

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err == nil {
		t.Fatal("expected error when ReservedDepositWallet fails, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(calls))
	}
}

func TestReservationStaleDepositWatcher_GetWalletChainError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB012)
	spvChain.setReservedDeposit(key, walletPKH(), true)
	// No spvChain.setWallet call: GetWallet errors naturally.

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err == nil {
		t.Fatal("expected error when GetWallet fails, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(calls))
	}
}

func TestReservationStaleDepositWatcher_GetReservationChainError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB016)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	// No spvChain.setReservation: GetReservation returns an error.

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err == nil {
		t.Fatal("expected error when GetReservation fails, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(calls))
	}
}

// TestReservationStaleDepositWatcher_GetReservationActionChainError_DoesNotNotifyEvenPastDeadline
// verifies that a transient RPC error on GetReservationAction is
// propagated as an error and MUST NOT fall through to the reveal-timestamp
// staleness path or trigger a premature stale deposit notification.
func TestReservationStaleDepositWatcher_GetReservationActionChainError_DoesNotNotifyEvenPastDeadline(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	fundingOutputIndex := uint32(0)

	key := spvChain.BuildDepositKey(fundingTxHash, fundingOutputIndex)
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
	})
	// Seed deposit revealed event and request with a deadline in the past:
	// RevealedAt (1_000) + Timeout (3600) = 4_600.
	seedPastDepositRevealedEvent(t, spvChain, wallet, fundingTxHash, fundingOutputIndex, 0)
	spvChain.setDepositRequest(fundingTxHash, fundingOutputIndex, &tbtc.DepositChainRequest{
		RevealedAt: time.Unix(1_000, 0),
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	// GetReservationAction is NOT seeded, so it returns an error ("no action for given reservation/nonce").
	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	// now = 10_000 is well past the 4_600 reveal-derived deadline. A
	// transient RPC error must be surfaced as an error and must not be
	// conflated with an "action generation not yet created" Unknown state,
	// which would fall through to the reveal fallback and submit a
	// premature stale deposit notification.
	res, err := watcher.CheckStaleReservedDeposit(key, 10_000)
	if err == nil {
		t.Fatal("expected error on transient GetReservationAction RPC failure, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"transient RPC error must NOT trigger stale deposit notification, got %d calls",
			len(calls),
		)
	}
}

// TestReservationStaleDepositWatcher_AdvancingNonceEvaluatesActiveGeneration
// verifies that the watcher reads reservation.RequestNonce via
// GetReservation rather than assuming a hardcoded nonce = 1. If nonce 1
// timed out and a retry advanced the nonce to 2, the watcher must evaluate
// nonce 2's action generation, the current one.
func TestReservationStaleDepositWatcher_AdvancingNonceEvaluatesActiveGeneration(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB020)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	// Reservation has advanced to nonce 2 (e.g. after retry).
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 2,
	})

	// Nonce 1 is TimedOut (stale generation).
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})
	// Nonce 2 is Pending with a timeout in the past relative to now (5_000).
	spvChain.setReservationAction(key, 2, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 200,
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	// now = 5_000 > nonce 2's TimeoutAt (200).
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	calls := spvChain.getSubmittedStaleReservedDeposits()
	if len(calls) != 1 {
		t.Fatalf("expected one stale notification for nonce 2 timeout, got %d", len(calls))
	}
	if diff := deep.Equal(key, calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

// TestReservationStaleDepositWatcher_NoActionRequestedYetPropagatesWithoutMatchingEvent
// covers the "no acceptance action generation exists yet" branch
// (reservation.RequestNonce == 0 or action.State == ReservationActionStateUnknown)
// when no matching DepositRevealed event has been seeded either: the
// watcher cannot derive a staleness deadline from nothing, so it must
func TestReservationStaleDepositWatcher_NoActionRequestedYetPropagatesWithoutMatchingEvent(t *testing.T) {
	spvChain := newLocalChain()
	spvChain.setBlockCounter(newMockBlockCounter())

	key := reservationDepositKey(0xB013)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 0,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err == nil {
		t.Fatal("expected error when no matching deposit revealed event exists, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(calls))
	}
}

// TestReservationStaleDepositWatcher_NoActionRequestedYetNotifiesFromRevealTimestamp
// tests the reveal-timestamp derivation: a reserved deposit whose wallet never
// became Live, so its acceptance action generation was never requested
// on-chain (RequestNonce == 0). The watcher must derive the staleness deadline
// from the deposit's own reveal timestamp plus ReservationActionTimeout,
// and notify once that derived deadline has passed.
func TestReservationStaleDepositWatcher_NoActionRequestedYetNotifiesFromRevealTimestamp(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	fundingOutputIndex := uint32(0)

	key := spvChain.BuildDepositKey(fundingTxHash, fundingOutputIndex)
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 0,
	})

	seedPastDepositRevealedEvent(t, spvChain, wallet, fundingTxHash, fundingOutputIndex, 0)
	spvChain.setDepositRequest(fundingTxHash, fundingOutputIndex, &tbtc.DepositChainRequest{
		RevealedAt: time.Unix(1_000, 0),
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout, // 3600
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	// Derived deadline = RevealedAt (1_000) + ReservationActionTimeout
	// (3600) = 4_600. now = 10_000 > 4_600, so the deposit is stale.
	res, err := watcher.CheckStaleReservedDeposit(key, 10_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	calls := spvChain.getSubmittedStaleReservedDeposits()
	if len(calls) != 1 {
		t.Fatalf("expected one stale notification, got %d", len(calls))
	}
	if diff := deep.Equal(key, calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

// TestReservationStaleDepositWatcher_NoActionRequestedYetDoesNotNotifyBeforeDerivedDeadline
// mirrors the notifying case above but asks at a `now` before the derived
// deadline, asserting the watcher correctly defers rather than notifying early.
func TestReservationStaleDepositWatcher_NoActionRequestedYetDoesNotNotifyBeforeDerivedDeadline(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	fundingTxHash, err := bitcoin.NewHashFromString(
		"7cff663e3e08847a5579913f6a66bc6c01f5f48c6ae1783be77418ed188021e6",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	fundingOutputIndex := uint32(1)

	key := spvChain.BuildDepositKey(fundingTxHash, fundingOutputIndex)
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 0,
	})

	seedPastDepositRevealedEvent(t, spvChain, wallet, fundingTxHash, fundingOutputIndex, 0)
	spvChain.setDepositRequest(fundingTxHash, fundingOutputIndex, &tbtc.DepositChainRequest{
		RevealedAt: time.Unix(1_000, 0),
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout, // 3600
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	// Derived deadline = 1_000 + 3600 = 4_600. now = 2_000 < 4_600.
	res, err := watcher.CheckStaleReservedDeposit(key, 2_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"derived deadline not yet reached; expected zero notifications, got %d",
			len(calls),
		)
	}
}

// TestReservationStaleDepositWatcher_NotifierError verifies that the
// stale-deposit watcher propagates a NotifyStaleReservedDeposit failure.
func TestReservationStaleDepositWatcher_NotifierError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB014)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})
	spvChain.notifyStaleReservedDepositErr = fmt.Errorf("notifier unavailable")

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err == nil {
		t.Fatal("expected error when the notifier fails, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}
}

// TestReservationStaleDepositWatcher_ExactTimeoutBoundaryDoesNotNotify
// covers the `now == action.TimeoutAt` boundary explicitly: the watcher's
// condition is `now <= action.TimeoutAt` (must NOT notify), so equality
// must defer exactly like "before the deadline" does.
func TestReservationStaleDepositWatcher_ExactTimeoutBoundaryDoesNotNotify(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB015)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{
		RequestNonce: 1,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 5_000,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"now == action.TimeoutAt must not notify, got %d calls",
			len(calls),
		)
	}
}

// walletCallCountingChain wraps a Chain and counts GetWallet invocations,
// delegating every other method to the embedded Chain. It is used to
// assert that CheckStaleReservedDeposit deduplicates wallet-state reads
// within a single poll tick (calls sharing the same `now`) instead of
// issuing one GetWallet call per deposit.
type walletCallCountingChain struct {
	Chain
	walletCallCount int
}

func (w *walletCallCountingChain) GetWallet(
	walletPublicKeyHash [20]byte,
) (*tbtc.WalletChainData, error) {
	w.walletCallCount++
	return w.Chain.GetWallet(walletPublicKeyHash)
}

// TestReservationStaleDepositWatcher_DedupesWalletFetchWithinTick verifies
// that N deposits assigned to the same wallet, checked with the identical
// `now` value (as the poller does for every deposit within one poll
// tick), result in exactly one GetWallet call rather than one per
// deposit.
func TestReservationStaleDepositWatcher_DedupesWalletFetchWithinTick(t *testing.T) {
	inner := newLocalChain()
	spvChain := &walletCallCountingChain{Chain: inner}

	wallet := walletPKH()
	inner.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	const depositCount = 3
	keys := make([]*big.Int, depositCount)
	for i := range depositCount {
		key := reservationDepositKey(0xC000 + uint64(i))
		keys[i] = key
		inner.setReservedDeposit(key, wallet, true)
		inner.setReservation(key, &tbtc.Reservation{
			RequestNonce: 1,
		})
		// now (5_000) < action.TimeoutAt (10_000): every deposit resolves
		// to Keep, so the loop below checks all three deposits without
		// any being forgotten, isolating the assertion to the wallet
		// fetch count.
		inner.setReservationAction(key, 1, &tbtc.ReservationAction{
			State:     tbtc.ReservationActionStatePending,
			TimeoutAt: 10_000,
		})
	}

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})

	const now = uint32(5_000)
	for _, key := range keys {
		res, err := watcher.CheckStaleReservedDeposit(key, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != StaleDepositResolutionKeep {
			t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
		}
	}

	if spvChain.walletCallCount != 1 {
		t.Fatalf(
			"expected exactly one GetWallet call for %d deposits sharing "+
				"a wallet within one poll tick, got %d",
			depositCount,
			spvChain.walletCallCount,
		)
	}
}

// TestReservationStaleDepositWatcher_WalletFetchRefreshesAcrossTicks
// verifies that the wallet-state cache is scoped to a single poll tick,
// not permanent: a new `now` value (signaling the next tick) must trigger
// a fresh GetWallet call rather than reusing a previous tick's cached
// wallet state indefinitely.
func TestReservationStaleDepositWatcher_WalletFetchRefreshesAcrossTicks(t *testing.T) {
	inner := newLocalChain()
	spvChain := &walletCallCountingChain{Chain: inner}

	wallet := walletPKH()
	inner.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	key := reservationDepositKey(0xC100)
	inner.setReservedDeposit(key, wallet, true)
	inner.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	inner.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 10_000,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})

	if _, err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := watcher.CheckStaleReservedDeposit(key, 5_001); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spvChain.walletCallCount != 2 {
		t.Fatalf(
			"expected one GetWallet call per distinct poll tick, got %d",
			spvChain.walletCallCount,
		)
	}
}

// reservationParamsCallCountingChain wraps a Chain and counts
// ReservationParameters invocations, delegating every other method to the
// embedded Chain. It is used to assert that deriveTimeoutFromReveal
// deduplicates the governance-parameter fetch within a single poll tick
// (calls sharing the same `now`) instead of issuing one fetch per
// deposit.
type reservationParamsCallCountingChain struct {
	Chain
	reservationParamsCallCount int
}

func (w *reservationParamsCallCountingChain) ReservationParameters() (
	*tbtc.ReservationParameters,
	error,
) {
	w.reservationParamsCallCount++
	return w.Chain.ReservationParameters()
}

// TestReservationStaleDepositWatcher_DedupesReservationParametersFetchWithinTick
// verifies that N deposits with no reservation action recorded yet
// (RequestNonce == 0, so each routes through deriveTimeoutFromReveal's
// reveal-timestamp fallback), checked with the identical `now` value as
// the poller does for every deposit within one poll tick, result in
// exactly one ReservationParameters call rather than one per deposit.
func TestReservationStaleDepositWatcher_DedupesReservationParametersFetchWithinTick(t *testing.T) {
	inner := newLocalChain()
	spvChain := &reservationParamsCallCountingChain{Chain: inner}

	wallet := walletPKH()
	inner.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	fundingTxHash, err := bitcoin.NewHashFromString(
		"585b6699f42291d1a9d0776b75f04c295ea203f83504349db11e94fdae7d1b2c",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}

	const depositCount = 3
	const currentBlock = uint64(0)
	keys := make([]*big.Int, depositCount)
	for i := range depositCount {
		fundingOutputIndex := uint32(i)
		key := inner.BuildDepositKey(fundingTxHash, fundingOutputIndex)
		keys[i] = key

		inner.setReservedDeposit(key, wallet, true)
		inner.setReservation(key, &tbtc.Reservation{RequestNonce: 0})

		seedPastDepositRevealedEvent(t, inner, wallet, fundingTxHash, fundingOutputIndex, currentBlock)
		inner.setDepositRequest(fundingTxHash, fundingOutputIndex, &tbtc.DepositChainRequest{
			RevealedAt: time.Unix(1_000, 0),
		})
	}

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})

	// Derived deadline = 1_000 + 3600 = 4_600. now = 2_000 < 4_600, so
	// every deposit resolves to Keep without triggering forgetDeposit,
	// isolating the assertion to the parameter fetch count.
	const now = uint32(2_000)
	for _, key := range keys {
		res, err := watcher.CheckStaleReservedDeposit(key, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != StaleDepositResolutionKeep {
			t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
		}
	}

	if spvChain.reservationParamsCallCount != 1 {
		t.Fatalf(
			"expected exactly one ReservationParameters call for %d "+
				"deposits sharing a poll tick, got %d",
			depositCount,
			spvChain.reservationParamsCallCount,
		)
	}
}

// TestReservationStaleDepositWatcher_ReservationParametersFetchRefreshesAcrossTicks
// verifies that the governance-parameter cache is scoped to a single poll
// tick, not permanent: a new `now` value (signaling the next tick) must
// trigger a fresh ReservationParameters call rather than reusing a
// previous tick's cached value indefinitely.
func TestReservationStaleDepositWatcher_ReservationParametersFetchRefreshesAcrossTicks(t *testing.T) {
	inner := newLocalChain()
	spvChain := &reservationParamsCallCountingChain{Chain: inner}

	wallet := walletPKH()
	inner.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout,
	})

	fundingTxHash, err := bitcoin.NewHashFromString(
		"7cff663e3e08847a5579913f6a66bc6c01f5f48c6ae1783be77418ed188021e6",
		bitcoin.InternalByteOrder,
	)
	if err != nil {
		t.Fatal(err)
	}
	fundingOutputIndex := uint32(0)

	key := inner.BuildDepositKey(fundingTxHash, fundingOutputIndex)
	inner.setReservedDeposit(key, wallet, true)
	inner.setReservation(key, &tbtc.Reservation{RequestNonce: 0})

	seedPastDepositRevealedEvent(t, inner, wallet, fundingTxHash, fundingOutputIndex, 0)
	inner.setDepositRequest(fundingTxHash, fundingOutputIndex, &tbtc.DepositChainRequest{
		RevealedAt: time.Unix(1_000, 0),
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})

	// Derived deadline = 1_000 + 3600 = 4_600; both ticks ask before the
	// deadline so the deposit stays Keep and the memo is never
	// invalidated by a resolved-deposit forgetDeposit call.
	if _, err := watcher.CheckStaleReservedDeposit(key, 2_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := watcher.CheckStaleReservedDeposit(key, 2_001); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spvChain.reservationParamsCallCount != 2 {
		t.Fatalf(
			"expected one ReservationParameters call per distinct poll tick, got %d",
			spvChain.reservationParamsCallCount,
		)
	}
}
