package spv

import (
	"fmt"
	"math/big"
	"testing"
	"time"

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
	if currentBlock > staleDepositRevealScanLookBackBlocks {
		startBlock = currentBlock - staleDepositRevealScanLookBackBlocks
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
	// now (5_000) > action.TimeoutAt (100).
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionNotified {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionNotified, res)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

func TestReservationStaleDepositWatcher_AlreadyNotifiedReturnsNotified(t *testing.T) {
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionNotified {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionNotified, res)
	}

	// Second check for already notified deposit returns Notified without resubmitting.
	res, err = watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionNotified {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionNotified, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf("expected exactly one notification, got %d", len(calls))
	}
}

func TestReservationStaleDepositWatcher_NilDepositKeyError(t *testing.T) {
	spvChain := newLocalChain()

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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
	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
	// now = 5_000 > nonce 2's TimeoutAt (200).
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionNotified {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionNotified, res)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
	// Derived deadline = RevealedAt (1_000) + ReservationActionTimeout
	// (3600) = 4_600. now = 10_000 > 4_600, so the deposit is stale.
	res, err := watcher.CheckStaleReservedDeposit(key, 10_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionNotified {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionNotified, res)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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

	watcher := NewReservationStaleDepositWatcher(spvChain)
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
