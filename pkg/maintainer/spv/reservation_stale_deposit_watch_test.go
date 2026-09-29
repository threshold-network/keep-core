package spv

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/go-test/deep"
)

// reservationDepositKey returns a big.Int constructed from a uint64 to act
// as a reserved-deposit identifier in the stale-deposit watcher tests.
func reservationDepositKey(low uint64) *big.Int {
	return new(big.Int).SetUint64(low)
}

// reservationActionTimeout is the fixed action timeout used by some tests
// to seed ReservationParameters. It plays no role in the stale-deposit
// watcher's own staleness derivation any more (see D-2/C-3): it only
// matters for tests that rely on pollTick's vault matching against
// ReservationParameters.
const reservationActionTimeout uint32 = 3600

// locktimeForDeadline returns the 4-byte little-endian Bitcoin refund
// locktime that reverseUint32 decodes back to deadline, so tests can seed
// a DepositRevealed event's RefundLocktime for an exact, known refund
// deadline.
func locktimeForDeadline(deadline uint32) [4]byte {
	var locktime [4]byte
	binary.LittleEndian.PutUint32(locktime[:], deadline)
	return locktime
}

// seedPastDepositRevealedEvent installs a DepositRevealed event for the
// given wallet/funding outpoint at refundLocktime, discoverable by
// snapshotRefundDeadline's wallet-keyed event scan.
func seedPastDepositRevealedEvent(
	t *testing.T,
	spvChain *localChain,
	wallet [20]byte,
	fundingTxHash bitcoin.Hash,
	fundingOutputIndex uint32,
	currentBlock uint64,
	refundLocktime [4]byte,
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
			RefundLocktime:      refundLocktime,
		},
	); err != nil {
		t.Fatal(err)
	}
}

// seedStaleDeadline pre-populates the watcher's refund-deadline memo
// directly, standing in for a pollTick discovery pass: production
// snapshots the deadline from the DepositRevealed event at discovery
// time, so most of these tests - which exercise CheckStaleReservedDeposit
// directly - do not need to also seed a matching reveal event.
func seedStaleDeadline(
	watcher *ReservationStaleDepositWatcher,
	depositKey *big.Int,
	deadline uint32,
) {
	watcher.refundDeadlineMemo[depositKey.String()] = deadline
}

// TestReverseUint32 pins the exact byte order the Bridge derives from a
// reveal-time RefundLocktime. Deposit.sol documents the event field as
// "the deposit refund locktime as 4-byte LE", and the on-chain deadline
// is BTCUtils.reverseUint32(uint32(refundLocktime)): Solidity reads the
// bytes4 big-endian, then byte-swaps the result, which together is a
// plain little-endian read of the original 4 bytes. So the little-
// endian byte array for 4584 (0x11E8) - [0xE8, 0x11, 0x00, 0x00] -
// must decode back to 4584. The earlier expected value of 4600 was an
// arithmetic slip: 4600 is 0x11F8, whose little-endian bytes are
// [0xF8, 0x11, 0x00, 0x00], a different input. A big-endian read of the
// bytes above would give 0xE8110000, so this also guards against a
// byte-swapped or reversed-endianness decode.
func TestReverseUint32(t *testing.T) {
	got := reverseUint32([4]byte{0xE8, 0x11, 0x00, 0x00})
	if got != 4584 {
		t.Fatalf("expected 4584, got %d", got)
	}
}

func TestReservationStaleDepositWatcher_NonReservedDepositIsSkipped(t *testing.T) {
	spvChain := newLocalChain()

	// Deposit is NOT booked as reserved: the fake mirrors the contract
	// and reports a zero wallet field for it (see chain_test.go's
	// ReservedDepositWallet).
	spvChain.setReservedDeposit(reservationDepositKey(0xB001), walletPKH(), false)

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
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

// TestReservationStaleDepositWatcher_LiveWalletNotifiedAfterDeadline is a
// regression test for C-3 (case 1): a reserved deposit assigned to a Live
// wallet that never accepts it must still be notified once its refund
// deadline has passed and its action generation is no longer Pending -
// the old code treated a Live wallet as permanently exempt and never
// notified it.
func TestReservationStaleDepositWatcher_LiveWalletNotifiedAfterDeadline(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB002)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateLive,
	})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	seedStaleDeadline(watcher, key, 100)

	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	calls := spvChain.getSubmittedStaleReservedDeposits()
	if len(calls) != 1 {
		t.Fatalf("expected one stale notification for the Live-wallet deposit, got %d", len(calls))
	}
	if diff := deep.Equal(key, calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

// TestReservationStaleDepositWatcher_TimedOutActionNotifiedAfterDeadline is
// a regression test for C-3 (case 2): a deposit whose acceptance
// generation timed out before its later refund deadline must still be
// notified once that deadline passes - the old code dropped the deposit
// as soon as it observed the TimedOut action, well before the on-chain
// refund deadline had elapsed.
func TestReservationStaleDepositWatcher_TimedOutActionNotifiedAfterDeadline(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB003)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateMovingFunds,
	})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	seedStaleDeadline(watcher, key, 100)

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

// TestReservationStaleDepositWatcher_PendingActionNotNotifiedAfterDeadline
// pins the notification-eligibility rule (C-3): even after the refund
// deadline has passed, a still-Pending action generation must not be
// notified - the contract would revert with "Acceptance authorization
// pending" - so the deposit stays tracked rather than triggering an
// avoidable reverting transaction.
func TestReservationStaleDepositWatcher_PendingActionNotNotifiedAfterDeadline(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB004)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 10_000,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	seedStaleDeadline(watcher, key, 100)

	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"pending action past the deadline must not notify, got %d calls",
			len(calls),
		)
	}
}

// staleReadCountingChain wraps a Chain and counts the per-deposit chain
// reads CheckStaleReservedDeposit issues past the deadline gate
// (IsReservedDeposit, ReservedDepositWallet, GetReservation,
// GetReservationAction), delegating every other method to the embedded
// Chain. It is used to verify I-2/D-2's "no chain reads before the
// deadline" invariant, extended to cover IsReservedDeposit (I-3): the
// deferred reservation confirmation pollTick's vault-match discovery no
// longer performs at discovery time.
type staleReadCountingChain struct {
	Chain
	isReservedDepositCalls     int
	reservedDepositWalletCalls int
	getReservationCalls        int
	getReservationActionCalls  int
}

func (c *staleReadCountingChain) IsReservedDeposit(
	depositKey *big.Int,
) (bool, error) {
	c.isReservedDepositCalls++
	return c.Chain.IsReservedDeposit(depositKey)
}

func (c *staleReadCountingChain) ReservedDepositWallet(
	depositKey *big.Int,
) ([20]byte, error) {
	c.reservedDepositWalletCalls++
	return c.Chain.ReservedDepositWallet(depositKey)
}

func (c *staleReadCountingChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	c.getReservationCalls++
	return c.Chain.GetReservation(reservationKey)
}

func (c *staleReadCountingChain) GetReservationAction(
	reservationKey *big.Int,
	requestNonce uint64,
) (*tbtc.ReservationAction, error) {
	c.getReservationActionCalls++
	return c.Chain.GetReservationAction(reservationKey, requestNonce)
}

// TestReservationStaleDepositWatcher_NoChainReadsBeforeDeadline is a
// regression test for I-2/D-2: before the snapshotted refund deadline
// has passed, CheckStaleReservedDeposit must not issue any per-deposit
// chain read at all (the check is in-memory only). Once the deadline has
// passed, the check does read on-chain state.
func TestReservationStaleDepositWatcher_NoChainReadsBeforeDeadline(t *testing.T) {
	inner := newLocalChain()
	spvChain := &staleReadCountingChain{Chain: inner}

	key := reservationDepositKey(0xB005)
	wallet := walletPKH()
	inner.setReservedDeposit(key, wallet, true)
	inner.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	inner.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	inner.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	// deadline 10_000 keeps the 5_000 pre-deadline check below it; the
	// post-deadline check uses 10_601, which is comfortably past
	// deadline + the maximum possible first-attempt stagger offset
	// (bounded by actionTimeoutRenotifyInterval, 600s) so the stagger
	// gate (see the identical comment in
	// TestReservationActionTimeoutWatcher_RunLoop_IncrementalTracking)
	// never masks the notification this test asserts.
	seedStaleDeadline(watcher, key, 10_000)

	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if spvChain.isReservedDepositCalls != 0 ||
		spvChain.reservedDepositWalletCalls != 0 ||
		spvChain.getReservationCalls != 0 ||
		spvChain.getReservationActionCalls != 0 {
		t.Fatalf(
			"expected zero chain reads before the deadline, got "+
				"isReserved=%d wallet=%d reservation=%d action=%d",
			spvChain.isReservedDepositCalls,
			spvChain.reservedDepositWalletCalls,
			spvChain.getReservationCalls,
			spvChain.getReservationActionCalls,
		)
	}

	res, err = watcher.CheckStaleReservedDeposit(key, 10_601)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if spvChain.isReservedDepositCalls == 0 ||
		spvChain.reservedDepositWalletCalls == 0 ||
		spvChain.getReservationCalls == 0 ||
		spvChain.getReservationActionCalls == 0 {
		t.Fatalf(
			"expected chain reads past the deadline, got isReserved=%d "+
				"wallet=%d reservation=%d action=%d",
			spvChain.isReservedDepositCalls,
			spvChain.reservedDepositWalletCalls,
			spvChain.getReservationCalls,
			spvChain.getReservationActionCalls,
		)
	}
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf("expected one stale notification, got %d", len(calls))
	}
}

// TestReservationStaleDepositWatcher_CompletionDetectedByClearedWalletField
// is a regression test for the C-3 completion signal: the deposit is
// retired only once ReservedDepositWallet reads back zero - not on
// submission alone - and the retirement resolution reflects whether this
// watcher's own notification was outstanding.
func TestReservationStaleDepositWatcher_CompletionDetectedByClearedWalletField(t *testing.T) {
	t.Run("our outstanding notification is confirmed", func(t *testing.T) {
		spvChain := newLocalChain()

		key := reservationDepositKey(0xB006)
		wallet := walletPKH()
		spvChain.setReservedDeposit(key, wallet, true)
		spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
		spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
		spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
			State:     tbtc.ReservationActionStateTimedOut,
			TimeoutAt: 100,
		})

		watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
		seedStaleDeadline(watcher, key, 100)

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

		// The notify transaction mines: the record's wallet field clears.
		spvChain.setReservedDeposit(key, [20]byte{}, true)

		res, err = watcher.CheckStaleReservedDeposit(key, 5_060)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != StaleDepositResolutionNotified {
			t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionNotified, res)
		}
		if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
			t.Fatalf("expected still exactly one notification submitted, got %d", len(calls))
		}
	})

	t.Run("cleared without an outstanding notification resolves Drop", func(t *testing.T) {
		spvChain := newLocalChain()

		key := reservationDepositKey(0xB007)
		// isReserved true but the wallet field is already zero: released
		// by another operator, an acceptance, or governance before this
		// watcher ever attempted a notification.
		spvChain.setReservedDeposit(key, [20]byte{}, true)

		watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
		seedStaleDeadline(watcher, key, 100)

		res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != StaleDepositResolutionDrop {
			t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionDrop, res)
		}
		if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
			t.Fatalf("expected zero notifications, got %d", len(calls))
		}
	})
}

// TestReservationStaleDepositWatcher_NotifiedNotConfirmedIsRetried covers
// the retry half of the completion contract: if a later tick's re-check
// does NOT observe the wallet field cleared, the deposit must stay
// tracked (resolution Keep, never Drop or Notified) rather than being
// silently and permanently lost, and once actionTimeoutRenotifyInterval
// elapses without confirmation the notification is retried.
func TestReservationStaleDepositWatcher_NotifiedNotConfirmedIsRetried(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB017)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	seedStaleDeadline(watcher, key, 100)

	// First tick: submit the notification, awaiting confirmation.
	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	// A tick shortly after, still within the renotify grace window, must
	// neither resubmit nor evict: the wallet field is still non-zero
	// (the notify transaction has not been observed to take effect), so
	// the deposit stays tracked rather than being lost.
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

	// The wallet field never actually cleared, meaning the prior
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
// caller-supplied, not-guaranteed-monotonic tick token, so a tick whose
// now is smaller than the deposit's recorded notifiedAt must not
// underflow now-notifiedAt to a huge value and treat the backoff window
// as already elapsed - that would resubmit NotifyStaleReservedDeposit
// immediately instead of waiting out actionTimeoutRenotifyInterval.
func TestReservationStaleDepositWatcher_RenotifyBackoffSurvivesNowBeforeNotifiedAt(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB018)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	seedStaleDeadline(watcher, key, 100)

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

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	res, err := watcher.CheckStaleReservedDeposit(nil, 5_000)
	if err == nil {
		t.Fatal("expected error for nil deposit key, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}
}

func TestReservationStaleDepositWatcher_ReservedDepositWalletChainError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB011)
	spvChain.setReservedDeposit(key, walletPKH(), true)
	spvChain.reservedDepositWalletErr = fmt.Errorf("rpc unavailable")

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	// No memo seeded: the deadline snapshot's own ReservedDepositWallet
	// read fails first.
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

func TestReservationStaleDepositWatcher_GetReservationChainError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB016)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	// No spvChain.setReservation: GetReservation returns an error.

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	seedStaleDeadline(watcher, key, 100)

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
// propagated as an error and MUST NOT trigger a premature stale deposit
// notification.
func TestReservationStaleDepositWatcher_GetReservationActionChainError_DoesNotNotifyEvenPastDeadline(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB013)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	// GetReservationAction is NOT seeded, so it returns an error.

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	seedStaleDeadline(watcher, key, 100)

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

// TestReservationStaleDepositWatcher_AdvancingNonceEvaluatesCurrentGenerationState
// verifies that the watcher reads reservation.RequestNonce via
// GetReservation rather than assuming a hardcoded nonce, and evaluates
// THAT generation's state - not an older one. Nonce 1 is seeded Pending
// (which would wrongly suppress the notification if read); nonce 2, the
// reservation's actual current generation, is TimedOut.
func TestReservationStaleDepositWatcher_AdvancingNonceEvaluatesCurrentGenerationState(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB020)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 2})

	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})
	spvChain.setReservationAction(key, 2, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 200,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	seedStaleDeadline(watcher, key, 100)

	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}

	calls := spvChain.getSubmittedStaleReservedDeposits()
	if len(calls) != 1 {
		t.Fatalf("expected one stale notification reflecting nonce 2's state, got %d", len(calls))
	}
	if diff := deep.Equal(key, calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

// TestReservationStaleDepositWatcher_RefundDeadlineFromRevealEvent covers
// the deadline snapshot's derivation path (a deposit checked without a
// pre-populated memo, i.e. not yet discovered by pollTick), and the
// exact byte-reversal (D-2): the deadline decoded from RefundLocktime
// must be honored precisely, both for "not yet reached" and "notify".
func TestReservationStaleDepositWatcher_RefundDeadlineFromRevealEvent(t *testing.T) {
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
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 0})

	seedPastDepositRevealedEvent(
		t, spvChain, wallet, fundingTxHash, fundingOutputIndex, 0,
		locktimeForDeadline(4_600),
	)

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)

	// now == the exact snapshotted deadline: the contract's gate is
	// strictly-greater-than, so this must still defer.
	res, err := watcher.CheckStaleReservedDeposit(key, 4_600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != StaleDepositResolutionKeep {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionKeep, res)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf("expected zero notifications at the exact deadline, got %d", len(calls))
	}

	// now strictly past the deadline, with no requested action
	// generation (RequestNonce == 0): notification-eligible. now must
	// also clear the first-attempt stagger window: deadline (4_600)
	// plus the maximum possible reservationOperatorStaggerOffset
	// (bounded by actionTimeoutRenotifyInterval, 600s), so 5_201.
	res, err = watcher.CheckStaleReservedDeposit(key, 5_201)
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

// TestReservationStaleDepositWatcher_NoMatchingRevealEventPropagatesError
// covers the derivation miss path: no DepositRevealed event can be found
// for the deposit within the catch-up window, so the deadline cannot be
// snapshotted and the check must propagate an error rather than guess.
func TestReservationStaleDepositWatcher_NoMatchingRevealEventPropagatesError(t *testing.T) {
	spvChain := newLocalChain()
	spvChain.setBlockCounter(newMockBlockCounter())

	key := reservationDepositKey(0xB013)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	// No seedPastDepositRevealedEvent call: the scan finds nothing.

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
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

func TestReservationStaleDepositWatcher_NotifierError(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB014)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})
	spvChain.notifyStaleReservedDepositErr = fmt.Errorf("notifier unavailable")

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	seedStaleDeadline(watcher, key, 100)

	res, err := watcher.CheckStaleReservedDeposit(key, 5_000)
	if err == nil {
		t.Fatal("expected error when the notifier fails, got nil")
	}
	if res != StaleDepositResolutionUnknown {
		t.Fatalf("expected resolution %v, got %v", StaleDepositResolutionUnknown, res)
	}
}

// TestReservationStaleDepositWatcher_ZeroWalletSkips verifies that a
// deposit whose reserved-deposit record is already cleared (wallet field
// zero) when the deadline snapshot is attempted resolves Drop without
// any notification.
func TestReservationStaleDepositWatcher_ZeroWalletSkips(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB006)
	spvChain.setReservedDeposit(key, [20]byte{}, true)

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
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

// staleDepositEventCountingChain wraps a Chain and counts calls to
// PastDepositRevealedEvents and IsReservedDeposit, delegating every
// other method to the embedded Chain. It is used to verify pollTick's
// activation-block-aware startup scan (I-1) and its deferral of the
// IsReservedDeposit read to after a tracked deposit's refund deadline
// (I-3).
type staleDepositEventCountingChain struct {
	Chain
	pastDepositRevealedEventsCalls int
	isReservedDepositCalls         int
}

func (c *staleDepositEventCountingChain) PastDepositRevealedEvents(
	filter *tbtc.DepositRevealedEventFilter,
) ([]*tbtc.DepositRevealedEvent, error) {
	c.pastDepositRevealedEventsCalls++
	return c.Chain.PastDepositRevealedEvents(filter)
}

func (c *staleDepositEventCountingChain) IsReservedDeposit(
	depositKey *big.Int,
) (bool, error) {
	c.isReservedDepositCalls++
	return c.Chain.IsReservedDeposit(depositKey)
}

// TestRunStaleDepositPollTick_TracksVaultMatchWithoutImmediateIsReservedDeposit
// is a regression test for I-3: a newly observed reveal whose vault
// matches the reservation vault must be tracked directly from the event
// (vault match against ReservationParameters().ReservationVault, read
// once per tick) without an immediate IsReservedDeposit chain read. That
// read is deferred until the tracked deposit's snapshotted refund
// deadline has passed. On the old behavior (IsReservedDeposit called at
// discovery time inside pollTick) this test fails: isReservedDepositCalls
// would already be 1 after the very first, pre-deadline tick.
func TestRunStaleDepositPollTick_TracksVaultMatchWithoutImmediateIsReservedDeposit(t *testing.T) {
	inner := newLocalChain()
	spvChain := &staleDepositEventCountingChain{Chain: inner}

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	fundingTxHash := bitcoin.Hash{0x05}
	fundingOutputIndex := uint32(0)
	endBlock := currentBlock
	if err := inner.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{StartBlock: 0, EndBlock: &endBlock},
		&tbtc.DepositRevealedEvent{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: fundingOutputIndex,
			Vault:              &vault,
			// Deadline far in the future: the first tick's check below
			// must stay entirely in-memory.
			RefundLocktime: locktimeForDeadline(50_000),
		},
	); err != nil {
		t.Fatal(err)
	}
	depositKey := inner.BuildDepositKey(fundingTxHash, fundingOutputIndex)

	// Reserved deposit record so the eventual post-deadline check does
	// not immediately drop it as never-reserved, isolating the
	// pre-deadline assertion below from that branch.
	wallet := walletPKHAt(0x77)
	inner.setReservedDeposit(depositKey, wallet, true)
	inner.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	inner.setReservation(depositKey, &tbtc.Reservation{RequestNonce: 1})
	inner.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)

	// First tick, now (1_000) is far before the deadline (50_000): the
	// reveal is discovered and tracked, but IsReservedDeposit must not
	// be called yet.
	trackedCount, ok := watcher.pollTick(1_000)
	if !ok {
		t.Fatal("expected pollTick to report a definitively successful scan")
	}
	if trackedCount != 1 {
		t.Fatalf("expected the reveal to be tracked on discovery, got %d", trackedCount)
	}
	if spvChain.isReservedDepositCalls != 0 {
		t.Fatalf(
			"expected zero IsReservedDeposit calls before the deadline, got %d",
			spvChain.isReservedDepositCalls,
		)
	}

	// Second tick, now (60_000) is past the deadline: the deferred
	// IsReservedDeposit read fires exactly once as part of the
	// post-deadline check.
	trackedCount, ok = watcher.pollTick(60_000)
	if !ok {
		t.Fatal("expected pollTick to report a definitively successful scan")
	}
	if trackedCount != 1 {
		t.Fatalf("expected the deposit to stay tracked awaiting the record to clear, got %d", trackedCount)
	}
	if spvChain.isReservedDepositCalls != 1 {
		t.Fatalf(
			"expected exactly one IsReservedDeposit call past the deadline, got %d",
			spvChain.isReservedDepositCalls,
		)
	}
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf("expected one stale notification, got %d", len(calls))
	}
}

// TestRunStaleDepositPollTick_FirstScanStartsAtActivationBlock is a
// regression test for I-1: the stale-deposit watcher's first reveal
// scan must start at the network's reservation activation block instead
// of a fixed 30-day lookback. currentBlock - reservationDefaultLookBackBlocks
// is 300_000 - 216_000 = 84_000, so a reveal at block 50_001 (inside the
// activation-based window, outside the old lookback window) is missed
// by the pre-fix 30-day-lookback scan and found by the activation-based
// one.
func TestRunStaleDepositPollTick_FirstScanStartsAtActivationBlock(t *testing.T) {
	inner := newLocalChain()

	const currentBlock = uint64(300_000)
	const activationBlock = uint64(50_000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	fundingTxHash := bitcoin.Hash{0x06}
	fundingOutputIndex := uint32(0)
	// Seeded at block 50_001: inside [activationBlock, currentBlock],
	// outside the old head-30d window [84_001, 300_000].
	eventEnd := currentBlock
	if err := inner.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{StartBlock: 0, EndBlock: &eventEnd},
		&tbtc.DepositRevealedEvent{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: fundingOutputIndex,
			Vault:              &vault,
			BlockNumber:        activationBlock + 1,
			RefundLocktime:     locktimeForDeadline(5_000_000),
		},
	); err != nil {
		t.Fatal(err)
	}

	watcher := NewReservationStaleDepositWatcher(inner, common.Address{}, activationBlock)

	trackedCount, ok := watcher.pollTick(1_000)
	if !ok {
		t.Fatal("expected pollTick to report a definitively successful scan")
	}
	if trackedCount != 1 {
		t.Fatalf(
			"expected the reveal at block %d (inside the activation-based "+
				"window) to be discovered on the first tick, got %d tracked",
			activationBlock+1,
			trackedCount,
		)
	}
}

// TestRunStaleDepositPollTick_UnknownNetworkSkipsStartupScan is a
// regression test for I-1: on a network with no reservation-activation
// entry (tbtc.ReservationsActivationBlock's math.MaxUint64 sentinel),
// the first reveal scan must be skipped entirely rather than falling
// back to a 30-day lookback - no PastDepositRevealedEvents call at all,
// and the tracked set stays empty even though a reveal exists on-chain
// within what would have been the old lookback window.
func TestRunStaleDepositPollTick_UnknownNetworkSkipsStartupScan(t *testing.T) {
	inner := newLocalChain()
	spvChain := &staleDepositEventCountingChain{Chain: inner}

	const currentBlock = uint64(300_000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	fundingTxHash := bitcoin.Hash{0x07}
	fundingOutputIndex := uint32(0)
	eventEnd := currentBlock
	if err := inner.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{StartBlock: 0, EndBlock: &eventEnd},
		&tbtc.DepositRevealedEvent{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: fundingOutputIndex,
			Vault:              &vault,
			BlockNumber:        currentBlock - 216_000 + 1,
			RefundLocktime:     locktimeForDeadline(5_000_000),
		},
	); err != nil {
		t.Fatal(err)
	}

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, math.MaxUint64)

	trackedCount, ok := watcher.pollTick(1_000)
	if !ok {
		t.Fatal("expected pollTick to report a definitively successful scan")
	}
	if trackedCount != 0 {
		t.Fatalf("expected an inactive network to skip the startup scan entirely, got %d tracked", trackedCount)
	}
	if spvChain.pastDepositRevealedEventsCalls != 0 {
		t.Fatalf(
			"expected zero PastDepositRevealedEvents calls on the startup "+
				"tick of a network without an activation entry, got %d",
			spvChain.pastDepositRevealedEventsCalls,
		)
	}
}
