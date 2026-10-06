package spv

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// The G2 chain-time tests below pin that reservation deadline decisions are
// judged on the chain head's block time, not on the node's wall clock. The
// node clock is kept only for the renotify backoff and notifiedAt stamps,
// and an unreadable head time skips the decision (no notify, no evict).

// TestG2L7_TimedOutGenerationKeptWhileChainHeadInsideLateWindow pins that
// the late-settlement window of a TimedOut acceptance generation is judged
// on the chain head: the fixture's generation stays settleable through
// timeoutAt (1000) + termSeconds (100) = 1100 in chain time, while the
// node's wall clock is billions of seconds past that bound.
func TestG2L7_TimedOutGenerationKeptWhileChainHeadInsideLateWindow(t *testing.T) {
	fixture := newTimedOutAcceptanceFixture(t, encodingP2WPKH)
	fixture.spvChain.setLatestBlockTimestamp(time.Unix(1050, 0))

	if err := proveReservationAcceptanceActions(
		fixture.state,
		fixture.config,
		fixture.spvChain,
		fixture.spvChain,
		fixture.btcChain,
		newProofInfoCache(),
		nil,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key := reservationEventKey(fixture.reservationKey, fixture.requestNonce)
	if _, exists := fixture.state.pendingAcceptanceEvents[key]; !exists {
		t.Error(
			"expected the generation to stay tracked while the chain head " +
				"is inside the late window, regardless of the node clock",
		)
	}
	if fixture.submissions != 1 {
		t.Errorf(
			"expected exactly one proof submission inside the late window, got %d",
			fixture.submissions,
		)
	}
}

// TestG2L7_UnreadableHeadTimeDoesNotEvictTimedOutGeneration pins that when
// the chain head time cannot be read the late-window eviction is skipped:
// the generation is kept even though the node clock is far past the bound.
func TestG2L7_UnreadableHeadTimeDoesNotEvictTimedOutGeneration(t *testing.T) {
	fixture := newTimedOutAcceptanceFixture(t, encodingP2WPKH)
	fixture.spvChain.latestBlockTimestampErr = errors.New("head unavailable")

	if err := proveReservationAcceptanceActions(
		fixture.state,
		fixture.config,
		fixture.spvChain,
		fixture.spvChain,
		fixture.btcChain,
		newProofInfoCache(),
		nil,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key := reservationEventKey(fixture.reservationKey, fixture.requestNonce)
	if _, exists := fixture.state.pendingAcceptanceEvents[key]; !exists {
		t.Error("expected the generation to be kept when the head time is unreadable")
	}
}

// newG2L11Watcher returns an action-timeout watcher tracking one Pending
// reanchor generation with timeoutAt 1000, and the node clock the watcher
// reads from nodeNow.
func newG2L11Watcher(
	t *testing.T,
	nodeNow *uint32,
) (*localChain, *ReservationActionTimeoutWatcher) {
	t.Helper()

	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet := walletPKH()
	key := reservationKey(0x6001)
	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            key,
		RequestNonce:              1,
		SourceWalletPublicKeyHash: wallet,
		BlockNumber:               500,
	})
	seededReservation(
		t,
		spvChain,
		key,
		wallet,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  1000,
			},
		},
		1,
	)

	ratw := NewReservationActionTimeoutWatcher(spvChain, time.Minute, common.Address{}, nil, 0)
	ratw.nowFn = func() uint32 { return *nodeNow }

	return spvChain, ratw
}

// TestG2L11_NotifyGateJudgedOnChainTime pins that the action-timeout notify
// decision compares the action's timeoutAt with the chain head time and not
// with the node clock. A head time past timeoutAt plus the maximum stagger
// offset (600s) always opens the gate.
func TestG2L11_NotifyGateJudgedOnChainTime(t *testing.T) {
	t.Run("node clock past timeoutAt but chain head before it does not notify", func(t *testing.T) {
		nodeNow := uint32(1_000_000)
		spvChain, ratw := newG2L11Watcher(t, &nodeNow)
		spvChain.setLatestBlockTimestamp(time.Unix(500, 0))

		if err := ratw.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
			t.Fatalf("expected no notification before timeoutAt in chain time, got %d", len(calls))
		}
		item, tracked := ratw.pendingActions[actionEventKey(reservationKey(0x6001), 1)]
		if !tracked {
			t.Fatal("expected the action to stay tracked")
		}
		if item.notifiedAt != 0 {
			t.Errorf("expected no notifiedAt stamp, got %d", item.notifiedAt)
		}
	})

	t.Run("chain head past timeoutAt with node clock before it notifies", func(t *testing.T) {
		nodeNow := uint32(10)
		spvChain, ratw := newG2L11Watcher(t, &nodeNow)
		spvChain.setLatestBlockTimestamp(time.Unix(1000+601, 0))

		if err := ratw.pollPendingActions(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
			t.Fatalf("expected one notification past timeoutAt in chain time, got %d", len(calls))
		}
	})
}

// TestG2L11_UnreadableHeadTimeNeitherNotifiesNorEvicts pins that an
// unreadable head time skips the notify decision and evicts nothing: a
// generation that was already notified stays tracked and is not renotified
// even though the node clock is far past the renotify backoff.
func TestG2L11_UnreadableHeadTimeNeitherNotifiesNorEvicts(t *testing.T) {
	nodeNow := uint32(1_000_000)
	spvChain, ratw := newG2L11Watcher(t, &nodeNow)

	// First pass with a readable head far past the deadline: notifies.
	spvChain.setLatestBlockTimestamp(time.Unix(1000+601, 0))
	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf("expected one notification on the first pass, got %d", len(calls))
	}

	// The node clock moves past the renotify backoff while the head time
	// becomes unreadable: nothing may be notified or evicted.
	nodeNow += uint32(actionTimeoutRenotifyInterval.Seconds()) + 1
	spvChain.latestBlockTimestampErr = errors.New("head unavailable")
	if err := ratw.pollPendingActions(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf("expected no further notification with an unreadable head, got %d total", len(calls))
	}
	if _, tracked := ratw.pendingActions[actionEventKey(reservationKey(0x6001), 1)]; !tracked {
		t.Error("expected the action to stay tracked with an unreadable head")
	}
}

// newG2L13Deposit seeds a reserved deposit whose action generation is
// TimedOut (so the notification is eligible once the refund deadline has
// passed) and returns its key.
func newG2L13Deposit(spvChain *localChain) *big.Int {
	key := reservationDepositKey(0xB013)
	wallet := walletPKH()
	spvChain.setReservedDeposit(key, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	spvChain.setReservation(key, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(key, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStateTimedOut,
		TimeoutAt: 100,
	})

	return key
}

// TestG2L13_RefundDeadlineGateJudgedOnChainTime pins that the stale-deposit
// refund-deadline gate compares the deposit's refund deadline with the chain
// head time and not with the node clock passed as now.
func TestG2L13_RefundDeadlineGateJudgedOnChainTime(t *testing.T) {
	const refundDeadline = uint32(1000)

	t.Run("node clock past the deadline but chain head before it does not notify", func(t *testing.T) {
		spvChain := newLocalChain()
		key := newG2L13Deposit(spvChain)
		spvChain.setLatestBlockTimestamp(time.Unix(500, 0))

		watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
		resolution, err := watcher.CheckStaleReservedDeposit(key, refundDeadline, 1_000_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolution != StaleDepositResolutionKeep {
			t.Errorf("expected resolution %v, got %v", StaleDepositResolutionKeep, resolution)
		}
		if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
			t.Fatalf("expected no notification before the deadline in chain time, got %d", len(calls))
		}
	})

	t.Run("chain head past the deadline with node clock before it notifies", func(t *testing.T) {
		spvChain := newLocalChain()
		key := newG2L13Deposit(spvChain)
		spvChain.setLatestBlockTimestamp(time.Unix(int64(refundDeadline)+601, 0))

		watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
		resolution, err := watcher.CheckStaleReservedDeposit(key, refundDeadline, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolution != StaleDepositResolutionKeep {
			t.Errorf("expected resolution %v, got %v", StaleDepositResolutionKeep, resolution)
		}
		if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
			t.Fatalf("expected one notification past the deadline in chain time, got %d", len(calls))
		}
	})
}

// TestG2L13_UnreadableHeadTimeNeitherNotifiesNorEvicts pins that an
// unreadable head time skips the refund-deadline decision: an eligible
// deposit is not notified, and a deposit whose record is not reserved is not
// retired, even though the node clock is far past the deadline.
func TestG2L13_UnreadableHeadTimeNeitherNotifiesNorEvicts(t *testing.T) {
	t.Run("eligible deposit is not notified", func(t *testing.T) {
		spvChain := newLocalChain()
		key := newG2L13Deposit(spvChain)
		spvChain.latestBlockTimestampErr = errors.New("head unavailable")

		watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
		resolution, err := watcher.CheckStaleReservedDeposit(key, 1000, 1_000_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolution != StaleDepositResolutionKeep {
			t.Errorf("expected resolution %v, got %v", StaleDepositResolutionKeep, resolution)
		}
		if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
			t.Fatalf("expected no notification with an unreadable head, got %d", len(calls))
		}
	})

	t.Run("unreserved deposit is not retired", func(t *testing.T) {
		spvChain := newLocalChain()
		key := reservationDepositKey(0xB014)
		spvChain.latestBlockTimestampErr = errors.New("head unavailable")

		watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{}, 0)
		resolution, err := watcher.CheckStaleReservedDeposit(key, 1000, 1_000_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolution != StaleDepositResolutionKeep {
			t.Errorf(
				"expected the deposit to be kept (resolution %v), got %v",
				StaleDepositResolutionKeep,
				resolution,
			)
		}
	})
}
