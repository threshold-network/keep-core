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

func TestReservationStaleDepositWatcher_NonReservedDepositIsSkipped(t *testing.T) {
	spvChain := newLocalChain()

	// Deposit is NOT booked as reserved.
	spvChain.setReservedDeposit(reservationDepositKey(0xB001), walletPKH(), false)

	watcher := NewReservationStaleDepositWatcher(spvChain)
	if err := watcher.CheckStaleReservedDeposit(reservationDepositKey(0xB001), 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("non-reserved deposit must not notify, got %d calls", len(calls))
	}
}

func TestReservationStaleDepositWatcher_LiveWalletDoesNotNotify(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB002)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateLive,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain)
	if err := watcher.CheckStaleReservedDeposit(key, 10_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
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

	// Action is already settled (no longer pending). The watcher must skip
	// the stale notification even though the wall clock has passed the
	// deadline.
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateSettled,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("zero-wallet deposit must skip, got %d calls", len(calls))
	}
}

func TestReservationStaleDepositWatcher_OnDepositRevealedDelegates(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB007)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain)
	if err := watcher.OnDepositRevealed(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(calls))
	}
}

func TestReservationStaleDepositWatcher_NilDepositKeyError(t *testing.T) {
	spvChain := newLocalChain()

	watcher := NewReservationStaleDepositWatcher(spvChain)
	if err := watcher.CheckStaleReservedDeposit(nil, 5_000); err == nil {
		t.Fatal("expected error for nil deposit key, got nil")
	}
}

func TestReservationStaleDepositWatcher_IsReservedDepositChainError(t *testing.T) {
	spvChain := newLocalChain()

	spvChain.isReservedDepositErr = fmt.Errorf("rpc unavailable")

	watcher := NewReservationStaleDepositWatcher(spvChain)
	err := watcher.CheckStaleReservedDeposit(reservationDepositKey(0xB010), 5_000)
	if err == nil {
		t.Fatal("expected error when IsReservedDeposit fails, got nil")
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
	err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err == nil {
		t.Fatal("expected error when ReservedDepositWallet fails, got nil")
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(calls))
	}
}

// TestReservationStaleDepositWatcher_GetWalletChainError exercises the
// error passthrough without any special chain-double wiring: the deposit's
// assigned wallet is never registered via setWallet, so GetWallet fails
// with its natural "no wallet for given PKH" error exactly as a real chain
// would if the wallet were somehow unresolvable.
func TestReservationStaleDepositWatcher_GetWalletChainError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB012)
	spvChain.setReservedDeposit(key, walletPKH(), true)
	// No spvChain.setWallet call: GetWallet errors naturally.

	watcher := NewReservationStaleDepositWatcher(spvChain)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err == nil {
		t.Fatal("expected error when GetWallet fails, got nil")
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(calls))
	}
}

// TestReservationStaleDepositWatcher_NoActionRequestedYetPropagatesWithoutMatchingEvent
// covers the "no acceptance action generation exists yet" branch
// (GetReservationAction returns State == ReservationActionStateUnknown, the
// zero value a Solidity mapping read returns for a never-requested nonce)
// when no matching DepositRevealed event has been seeded either: the
// watcher cannot derive a staleness deadline from nothing, so it must
// still surface an error rather than silently notifying or skipping.
func TestReservationStaleDepositWatcher_NoActionRequestedYetPropagatesWithoutMatchingEvent(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB013)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	// No spvChain.setReservationAction call: GetReservationAction returns
	// the zero-value action (State == ReservationActionStateUnknown), not
	// an error - this drives the watcher into the reveal-timestamp
	// derivation branch. No DepositRevealed event is seeded either, so
	// that branch cannot resolve and must itself return an error.

	watcher := NewReservationStaleDepositWatcher(spvChain)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err == nil {
		t.Fatal("expected error when no matching deposit revealed event exists, got nil")
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected no notifications on chain error, got %d", len(calls))
	}
}

// TestReservationStaleDepositWatcher_NoActionRequestedYetNotifiesFromRevealTimestamp
// is the real P1 fix under test: a reserved deposit whose wallet never
// became Live, so its acceptance action generation was never requested
// on-chain (GetReservationAction returns the zero-value
// ReservationActionStateUnknown, not an error and not a Pending action the
// old code path required to compute a deadline). The watcher must derive
// the staleness deadline from the deposit's own reveal timestamp
// (DepositRevealed event -> DepositChainRequest.RevealedAt) plus
// ReservationActionTimeout, and notify once that derived deadline has
// passed - exactly the scenario the watcher exists to catch, which the
// pre-fix code silently skipped forever.
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
	// No setReservationAction: no acceptance was ever requested on-chain.

	if err := spvChain.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
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
	spvChain.setDepositRequest(fundingTxHash, fundingOutputIndex, &tbtc.DepositChainRequest{
		RevealedAt: time.Unix(1_000, 0),
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout, // 3600
	})

	watcher := NewReservationStaleDepositWatcher(spvChain)
	// Derived deadline = RevealedAt (1_000) + ReservationActionTimeout
	// (3600) = 4_600. now = 10_000 > 4_600, so the deposit is stale.
	if err := watcher.CheckStaleReservedDeposit(key, 10_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
// deadline, asserting the watcher correctly defers rather than notifying
// early.
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

	if err := spvChain.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{
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
	spvChain.setDepositRequest(fundingTxHash, fundingOutputIndex, &tbtc.DepositChainRequest{
		RevealedAt: time.Unix(1_000, 0),
	})
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationActionTimeout: reservationActionTimeout, // 3600
	})

	watcher := NewReservationStaleDepositWatcher(spvChain)
	// Derived deadline = 1_000 + 3600 = 4_600. now = 2_000 < 4_600.
	if err := watcher.CheckStaleReservedDeposit(key, 2_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"derived deadline not yet reached; expected zero notifications, got %d",
			len(calls),
		)
	}
}

// TestReservationStaleDepositWatcher_NotifierError verifies that, unlike
// the stranding watcher (which continues past a notify failure because it
// processes a batch of reservations per call), the stale-deposit watcher
// propagates a NotifyStaleReservedDeposit failure to its single caller:
// CheckStaleReservedDeposit checks exactly one deposit per call, so there
// is nothing else to "continue" to.
func TestReservationStaleDepositWatcher_NotifierError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB014)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})
	spvChain.notifyStaleReservedDepositErr = fmt.Errorf("notifier unavailable")

	watcher := NewReservationStaleDepositWatcher(spvChain)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err == nil {
		t.Fatal("expected error when the notifier fails, got nil")
	}
}

// TestReservationStaleDepositWatcher_ExactTimeoutBoundaryDoesNotNotify
// covers the `now == action.TimeoutAt` boundary explicitly: the watcher's
// condition is `now <= action.TimeoutAt` (must NOT notify), so equality
// must defer exactly like "before the deadline" does. Existing tests only
// exercise now < TimeoutAt and now > TimeoutAt.
func TestReservationStaleDepositWatcher_ExactTimeoutBoundaryDoesNotNotify(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB015)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 5_000,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain)
	if err := watcher.CheckStaleReservedDeposit(key, 5_000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"now == action.TimeoutAt must not notify, got %d calls",
			len(calls),
		)
	}
}
