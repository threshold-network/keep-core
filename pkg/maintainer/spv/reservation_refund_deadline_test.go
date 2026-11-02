package spv

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// us12RefundDeadline is the refund deadline the US-12 row's Bitcoin
// refund spend used: the reveal-time RefundLocktime's little-endian
// value (the very bytes the deposit script carries and that
// OP_CHECKLOCKTIMEVERIFY compares the spending transaction's nLockTime
// against). The same number is what the Bridge snapshots on-chain at
// reveal and what the stale-deposit watcher compares the chain head's
// block time against.
const us12RefundDeadline uint32 = 1642773600

// us12RefundLocktimeBytes is the RefundLocktime the US-12 reveal
// carried, byte for byte: the 4-byte little-endian encoding of
// us12RefundDeadline.
var us12RefundLocktimeBytes = locktimeForDeadline(us12RefundDeadline)

// us12StaleDepositKey is the reserved deposit's key in these tests.
//
// The key is chosen so that the watcher's first-notification stagger
// offset (reservationOperatorStaggerOffset, bounded by
// actionTimeoutRenotifyInterval) is ZERO for the zero operator address
// used here. With a zero offset the deadline comparison alone decides
// when the first notification may be submitted, so the tests pin the
// deadline rule itself rather than the stagger. The offset is read
// from the production helper in the tests that need it, so a change to
// the stagger's derivation would be caught rather than silently
// absorbed.
var us12StaleDepositKey = reservationDepositKey(0xB053)

// us12StaleDepositSetup installs a reserved deposit whose action
// generation has timed out, so the deposit is eligible for the stale
// notification once its refund deadline has passed.
func us12StaleDepositSetup(
	spvChain *localChain,
	key *big.Int,
) {
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{
		State: tbtc.StateUnknown,
	})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: us12RefundDeadline,
	})
}

// TestReservedDepositRefund_StaleNotificationFollowsTheRefundDeadline
// is the keep-core half of the US-12 row: after the depositor's refund
// spend has consumed the funding output, the on-chain bookkeeping
// follows only when a separate, eligible notifyStaleReservedDeposit
// transaction is submitted - it does not run automatically.
//
// The test walks the chain head across the very deadline the STAGE A
// Bitcoin refund spend used and asserts:
//
//  1. at the deadline the watcher does NOT notify, and it does not even
//     read the deposit's on-chain record - the deadline gate is
//     in-memory only (the contract's rule is block.timestamp >
//     refundDeadline, so the deadline itself is not yet past);
//  2. one second later it DOES notify, exactly once, for this deposit;
//  3. a second check at the same head time does not notify again - the
//     notification is a one-shot transaction, not a per-tick stream.
//
// The on-chain `pendingReservedDeposits` counter decrement itself is
// NOT observable from keep-core: the counter lives in the Bridge
// contract and only the contract's own transaction decrements it. What
// is observable here is the client-side effect the row requires: the
// notification is submitted, exactly once, and only after the deadline.
func TestReservedDepositRefund_StaleNotificationFollowsTheRefundDeadline(
	t *testing.T,
) {
	inner := newLocalChain()
	spvChain := &staleReadCountingChain{Chain: inner}

	key := us12StaleDepositKey
	us12StaleDepositSetup(inner, key)

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)

	// The chain head is exactly at the refund deadline. The contract
	// requires block.timestamp > refundDeadline, so the notification
	// must not fire yet.
	inner.setLatestBlockTimestamp(time.Unix(int64(us12RefundDeadline), 0))

	resolution, err := watcher.CheckStaleReservedDeposit(
		key,
		us12RefundDeadline,
		us12RefundDeadline,
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if resolution != StaleDepositResolutionKeep {
		t.Fatalf(
			"expected resolution %v at the refund deadline, got %v",
			StaleDepositResolutionKeep,
			resolution,
		)
	}
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"the notification must not fire at the refund deadline, got %d calls",
			len(calls),
		)
	}
	if spvChain.reservedDepositWalletCalls != 0 ||
		spvChain.getReservationCalls != 0 ||
		spvChain.getReservationActionCalls != 0 {
		t.Fatalf(
			"expected zero chain reads at the refund deadline, got "+
				"wallet=%d reservation=%d action=%d",
			spvChain.reservedDepositWalletCalls,
			spvChain.getReservationCalls,
			spvChain.getReservationActionCalls,
		)
	}

	// One second past the deadline the refund spend is spendable and
	// the notification is eligible.
	inner.setLatestBlockTimestamp(
		time.Unix(int64(us12RefundDeadline)+1, 0),
	)

	resolution, err = watcher.CheckStaleReservedDeposit(
		key,
		us12RefundDeadline,
		us12RefundDeadline+1,
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if resolution != StaleDepositResolutionKeep {
		t.Fatalf(
			"expected resolution %v past the refund deadline, got %v",
			StaleDepositResolutionKeep,
			resolution,
		)
	}

	calls := inner.getSubmittedStaleReservedDeposits()
	if len(calls) != 1 {
		t.Fatalf(
			"expected exactly one stale notification past the refund deadline, got %d",
			len(calls),
		)
	}
	if calls[0].Cmp(key) != 0 {
		t.Fatalf(
			"expected the notification for deposit [%v], got [%v]",
			key,
			calls[0],
		)
	}

	// A further check at the same head time must not submit a second
	// notification: the notification is a transaction, and the record
	// stays tracked until it clears on-chain.
	resolution, err = watcher.CheckStaleReservedDeposit(
		key,
		us12RefundDeadline,
		us12RefundDeadline+1,
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if resolution != StaleDepositResolutionKeep {
		t.Fatalf(
			"expected resolution %v on the repeated check, got %v",
			StaleDepositResolutionKeep,
			resolution,
		)
	}
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf(
			"expected still exactly one stale notification, got %d",
			len(calls),
		)
	}
}

// TestReservedDepositRefund_StaleNotificationIsStrictlyAfterTheDeadline
// pins the STRICTNESS of the deadline comparison - the contract's rule
// is block.timestamp > refundDeadline, so the deadline second itself is
// not yet past.
//
// The strictness is observable on the RETRY path rather than on the
// first attempt: the first attempt is additionally gated by the
// per-operator stagger (see reservationOperatorStaggerOffset), which
// already keeps the notification behind the deadline, so a first
// attempt cannot distinguish a strict comparison from a non-strict one.
// Once the first notification has been submitted, the retry path is
// gated only by the deadline comparison and the renotify backoff, and
// the chain head may legitimately sit exactly at the deadline while the
// node clock has already moved past the renotify interval - the chain
// head time and the node clock are different clocks.
//
// The test therefore submits the first notification past the deadline,
// lets the renotify interval elapse on the node clock, and then checks
// again with the chain head exactly AT the deadline. A watcher that
// compared the deadline non-strictly would submit a second
// notification there - one that the contract would reject with "Deposit
// refund deadline has not elapsed".
func TestReservedDepositRefund_StaleNotificationIsStrictlyAfterTheDeadline(
	t *testing.T,
) {
	inner := newLocalChain()

	key := us12StaleDepositKey
	us12StaleDepositSetup(inner, key)

	watcher := NewReservationStaleDepositWatcher(inner, common.Address{}, 0)

	// First attempt: the chain head is past the deadline and past the
	// (zero) stagger offset, so the notification is submitted.
	inner.setLatestBlockTimestamp(
		time.Unix(int64(us12RefundDeadline)+1, 0),
	)
	if _, err := watcher.CheckStaleReservedDeposit(
		key,
		us12RefundDeadline,
		us12RefundDeadline+1,
	); err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf(
			"expected exactly one stale notification for the first attempt, got %d",
			len(calls),
		)
	}

	// The renotify interval elapses on the node clock, while the chain
	// head sits exactly AT the refund deadline: the chain head has not
	// caught up with the node clock yet.
	renotifyAfter := us12RefundDeadline + 1 +
		uint32(actionTimeoutRenotifyInterval.Seconds()) + 1
	inner.setLatestBlockTimestamp(time.Unix(int64(us12RefundDeadline), 0))

	resolution, err := watcher.CheckStaleReservedDeposit(
		key,
		us12RefundDeadline,
		renotifyAfter,
	)
	if err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if resolution != StaleDepositResolutionKeep {
		t.Fatalf(
			"expected resolution %v at the refund deadline, got %v",
			StaleDepositResolutionKeep,
			resolution,
		)
	}
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf(
			"expected still exactly one stale notification: the chain head "+
				"is exactly at the refund deadline, so the notification "+
				"is not eligible, got %d calls",
			len(calls),
		)
	}

	// One second later - the chain head is now strictly past the
	// deadline - the retry becomes eligible again.
	inner.setLatestBlockTimestamp(
		time.Unix(int64(us12RefundDeadline)+1, 0),
	)
	if _, err := watcher.CheckStaleReservedDeposit(
		key,
		us12RefundDeadline,
		renotifyAfter,
	); err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 2 {
		t.Fatalf(
			"expected the notification to be retried once the chain head "+
				"passed the deadline, got %d calls",
			len(calls),
		)
	}
}

// TestReservedDepositRefund_StaleNotificationUsesTheRevealedLocktime
// pins the bridge between the two halves of the row: the refund
// deadline the watcher gates on is decoded from the very RefundLocktime
// bytes the Bitcoin refund script carries. The reveal event's
// RefundLocktime is byte-reversed (see reverseUint32) into the UNIX
// deadline, so the number the watcher compares the chain head against
// and the nLockTime the refund spend must carry are the same value.
//
// The test seeds the reveal event's RefundLocktime with the US-12
// locktime bytes and drives the watcher through its poll loop, so the
// deadline comes from the event rather than from a hand-passed
// argument.
func TestReservedDepositRefund_StaleNotificationUsesTheRevealedLocktime(
	t *testing.T,
) {
	inner := newLocalChain()

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	fundingTxHash := bitcoin.Hash{0x09}
	fundingOutputIndex := uint32(0)
	endBlock := currentBlock - reservationEventScanConfirmationBlocks
	if err := inner.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{StartBlock: 0, EndBlock: &endBlock},
		&tbtc.DepositRevealedEvent{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: fundingOutputIndex,
			Vault:              &vault,
			// The reveal's RefundLocktime, byte for byte as the US-12
			// reveal carried it.
			RefundLocktime: us12RefundLocktimeBytes,
		},
	); err != nil {
		t.Fatal(err)
	}
	depositKey := inner.BuildDepositKey(fundingTxHash, fundingOutputIndex)

	// The reserved deposit's record is still open: the wallet field is
	// non-zero, so the deposit is still pending on-chain.
	wallet := walletPKHAt(0x91)
	inner.setReservedDeposit(depositKey, wallet, true)
	inner.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	inner.setReservation(depositKey, &tbtc.Reservation{RequestNonce: 1})
	inner.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(inner, common.Address{}, 0)

	// The watcher's FIRST notify attempt for a deposit is staggered by
	// a deterministic per-operator offset (see
	// reservationOperatorStaggerOffset), so the head time that opens
	// the deadline gate for this deposit's first attempt is the
	// deadline plus that offset. The offset is read from the
	// production helper, not hard-coded.
	stagger := reservationOperatorStaggerOffset(
		common.Address{},
		depositKey.String(),
		actionTimeoutRenotifyInterval,
	)

	// The chain head is exactly at the decoded deadline: the reveal is
	// tracked, and the check stays in-memory - no notification.
	inner.setLatestBlockTimestamp(time.Unix(int64(us12RefundDeadline), 0))
	trackedCount, ok := watcher.pollTick(us12RefundDeadline)
	if !ok {
		t.Fatal("expected pollTick to report a definitively successful scan")
	}
	if trackedCount != 1 {
		t.Fatalf(
			"expected the reveal to be tracked on discovery, got %d tracked",
			trackedCount,
		)
	}
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"the notification must not fire at the decoded deadline, got %d calls",
			len(calls),
		)
	}

	// One second past the deadline plus the stagger offset the
	// notification fires.
	notifyAt := us12RefundDeadline + stagger + 1
	inner.setLatestBlockTimestamp(time.Unix(int64(notifyAt), 0))
	trackedCount, ok = watcher.pollTick(notifyAt)
	if !ok {
		t.Fatal("expected pollTick to report a definitively successful scan")
	}
	if trackedCount != 1 {
		t.Fatalf(
			"expected the deposit to stay tracked awaiting the record to clear, got %d",
			trackedCount,
		)
	}

	calls := inner.getSubmittedStaleReservedDeposits()
	if len(calls) != 1 {
		t.Fatalf(
			"expected exactly one stale notification past the decoded deadline, got %d",
			len(calls),
		)
	}
	if calls[0].Cmp(depositKey) != 0 {
		t.Fatalf(
			"expected the notification for deposit [%v], got [%v]",
			depositKey,
			calls[0],
		)
	}
}

// TestReservedDepositRefund_StaleNotificationIsNotAutomatic pins the
// row's "it does not run automatically" clause on the client side: the
// refund spend itself is a Bitcoin transaction that keep-core neither
// observes nor reacts to, and the on-chain cleanup happens only when
// the watcher's check runs. Nothing in the watcher's construction or in
// the poll tick submits the notification on its own account: the
// notification is submitted by CheckStaleReservedDeposit, which the
// caller (the poll loop) invokes.
//
// The test therefore asserts that a freshly constructed watcher holds
// no notification state, and that the notification appears only after
// an explicit check - not before.
func TestReservedDepositRefund_StaleNotificationIsNotAutomatic(t *testing.T) {
	inner := newLocalChain()

	const currentBlock = uint64(1000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	inner.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	fundingTxHash := bitcoin.Hash{0x0A}
	fundingOutputIndex := uint32(0)
	endBlock := currentBlock - reservationEventScanConfirmationBlocks
	if err := inner.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{StartBlock: 0, EndBlock: &endBlock},
		&tbtc.DepositRevealedEvent{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: fundingOutputIndex,
			Vault:              &vault,
			RefundLocktime:     us12RefundLocktimeBytes,
		},
	); err != nil {
		t.Fatal(err)
	}
	depositKey := inner.BuildDepositKey(fundingTxHash, fundingOutputIndex)

	wallet := walletPKHAt(0x92)
	inner.setReservedDeposit(depositKey, wallet, true)
	inner.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	inner.setReservation(depositKey, &tbtc.Reservation{RequestNonce: 1})
	inner.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(inner, common.Address{}, 0)

	// The chain head is well past the deadline (and past the first
	// attempt's stagger offset) from the very first tick, yet nothing
	// is submitted until a check runs.
	stagger := reservationOperatorStaggerOffset(
		common.Address{},
		depositKey.String(),
		actionTimeoutRenotifyInterval,
	)
	notifyAt := us12RefundDeadline + stagger + 1

	inner.setLatestBlockTimestamp(time.Unix(int64(notifyAt), 0))
	if calls := inner.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
		t.Fatalf(
			"expected no notification before any check runs, got %d calls",
			len(calls),
		)
	}

	if trackedCount, ok := watcher.pollTick(notifyAt); !ok {
		t.Fatal("expected pollTick to report a definitively successful scan")
	} else if trackedCount != 1 {
		t.Fatalf("expected the reveal to be tracked, got %d", trackedCount)
	}

	calls := inner.getSubmittedStaleReservedDeposits()
	if len(calls) != 1 {
		t.Fatalf(
			"expected exactly one notification once the check ran, got %d",
			len(calls),
		)
	}
	if calls[0].Cmp(depositKey) != 0 {
		t.Fatalf(
			"expected the notification for deposit [%v], got [%v]",
			depositKey,
			calls[0],
		)
	}
}

// TestReservedDepositRefund_StaleNotificationIsPerDeposit pins that the
// notification is keyed on the deposit key: a different deposit key in
// the same watcher gets its own notification, and the refunded deposit's
// own notification is not re-submitted for the other key. This keeps the
// "exactly once" assertion in the tests above unambiguous.
func TestReservedDepositRefund_StaleNotificationIsPerDeposit(t *testing.T) {
	spvChain := newLocalChain()

	key := reservationDepositKey(0xB053)
	otherKey := reservationDepositKey(0xB104)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setReservedDeposit(otherKey, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})
	spvChain.setReservation(otherKey, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(otherKey, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
	spvChain.setLatestBlockTimestamp(time.Unix(int64(us12RefundDeadline)+1, 0))

	if _, err := watcher.CheckStaleReservedDeposit(
		key,
		us12RefundDeadline,
		us12RefundDeadline+1,
	); err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}
	if _, err := watcher.CheckStaleReservedDeposit(
		otherKey,
		us12RefundDeadline,
		us12RefundDeadline+1,
	); err != nil {
		t.Fatalf("unexpected error: [%v]", err)
	}

	calls := spvChain.getSubmittedStaleReservedDeposits()
	if len(calls) != 2 {
		t.Fatalf("expected one notification per deposit, got %d", len(calls))
	}

	notified := make(map[string]bool)
	for _, call := range calls {
		notified[call.String()] = true
	}
	for _, expected := range []*big.Int{key, otherKey} {
		if !notified[expected.String()] {
			t.Fatalf(
				"expected a notification for deposit [%v], got %v",
				expected,
				calls,
			)
		}
	}
}
