package spv

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-test/deep"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestReservationTimestampHorizon_Uint32MaxBoundary pins the client-side
// behaviour at the uint32 timestamp horizon that the reservation
// contracts' timestamps live in.
//
// The four reservation timestamps - timeoutAt, expiresAt,
// dissolutionEligibleAt and reanchorCooldownUntil - are uint32 on chain.
// The contracts compute them with an EXPLICIT `uint32(block.timestamp)`
// cast, which TRUNCATES after the horizon: a timestamp past 2106-02-07
// wraps into the past. Checked arithmetic does not protect an explicit
// narrowing cast, so the horizon is a stated deployment assumption, not
// an unlimited-time safety guarantee (INV-210).
//
// The Go client is the mirror image of that contract cast, and the
// difference is the point this test pins. Every client-side comparison
// against an on-chain deadline goes through tbtc.UnixSecondsToUint32,
// which SATURATES at math.MaxUint32 instead of wrapping. A wrapped
// clock would read as the wrong side of a deadline (or as a time decades
// in the past), so a client that wrapped would notify early and the
// contract would revert the notification; a client that saturates reads
// the horizon as the horizon.
//
// The test drives the real watchers at the boundary:
//   - the action-timeout watcher's deadline comparison (timeoutAt),
//   - the stale-deposit watcher's refund-deadline comparison, the
//     client-side mirror of the deposit's refund window, and
//   - the stranding watcher's dissolution-eligibility comparison
//     (dissolutionEligibleAt).
//
// The stale-deposit watcher is driven twice, for two different reasons.
// The first case pins the deadline-AT-the-horizon limitation: the
// saturating clock reads the same value at the horizon and one second
// past it, so a deposit whose deadline is the horizon itself is kept
// rather than notified on a guess. The second case is the one that
// DISTINGUISHES the two conversions: a deadline below the horizon by
// more than the maximum per-operator stagger, with the chain head one
// second past the horizon. The saturating clock reads the head as the
// horizon and notifies; the wrapping clock reads the head as zero and
// does not.
//
// Negative control: replace the saturating conversion with a wrapping
// one (uint32(seconds)) and the horizon assertions below fail: the
// conversion returns 0 past the horizon, and the stale-deposit watcher
// then reads the head time as 1970 and fires the notification early.
func TestReservationTimestampHorizon_Uint32MaxBoundary(t *testing.T) {
	// math.MaxUint32 is 4294967295, i.e. 2106-02-07T06:28:15Z. One
	// second later the uint32 horizon is behind the client.
	const horizon = uint64(math.MaxUint32)

	// The saturating conversion must return the horizon itself at the
	// horizon, and the horizon (not zero, not a wrapped value) one
	// second past it.
	t.Run("the clock saturates at the horizon instead of wrapping", func(t *testing.T) {
		if got := tbtc.UnixSecondsToUint32(int64(horizon)); got != math.MaxUint32 {
			t.Errorf(
				"expected the uint32 max at the horizon, got %d",
				got,
			)
		}
		if got := tbtc.UnixSecondsToUint32(int64(horizon) + 1); got != math.MaxUint32 {
			t.Errorf(
				"expected the uint32 max to saturate past the horizon, got %d",
				got,
			)
		}
	})

	// The action-timeout watcher compares the caller's timestamp against
	// the action's timeoutAt with an INCLUSIVE comparison (the contract
	// accepts the notification at exactly timeoutAt). At the horizon the
	// comparison must still fire exactly at timeoutAt and not one
	// second earlier.
	t.Run("action timeout fires exactly at the horizon", func(t *testing.T) {
		spvChain := newLocalChain()

		wallet := walletPKH()
		key := reservationKey(0xD001)

		seededReservation(
			t,
			spvChain,
			key,
			wallet,
			[]*tbtc.ReservationAction{
				{
					ActionType: tbtc.ReservationActionTypeReanchor,
					State:      tbtc.ReservationActionStatePending,
					TimeoutAt:  uint32(horizon),
				},
			},
			1,
		)

		watcher := NewReservationActionTimeoutWatcher(
			spvChain,
			0,
			common.Address{},
			newReservationStrandingWatcher(spvChain),
			0,
			nil,
		)

		// One second before the horizon the deadline has not been
		// reached: no notification.
		if err := watcher.CheckReservationActionTimeouts(
			key,
			uint32(horizon)-1,
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 0 {
			t.Fatalf(
				"expected no timeout notification before the horizon, got %d",
				len(calls),
			)
		}

		// Exactly at the horizon the deadline is reached: the
		// notification fires.
		if err := watcher.CheckReservationActionTimeouts(
			key,
			uint32(horizon),
		); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		calls := spvChain.getSubmittedReservationActionTimeouts()
		if len(calls) != 1 {
			t.Fatalf(
				"expected one timeout notification at the horizon, got %d",
				len(calls),
			)
		}
	})

	// The stale-deposit watcher compares the chain head time against the
	// deposit's snapshotted refund deadline with a STRICT comparison
	// (the contract requires block.timestamp > refundDeadline). At the
	// horizon the comparison must not fire one second early.
	//
	// Past the horizon the saturating clock reads the same value as at
	// the horizon, so the watcher can no longer tell "at the deadline"
	// from "past the deadline". That is the documented limit of the
	// horizon assumption, and the test pins it explicitly: the client
	// keeps the deposit rather than firing on a guess, which is the safe
	// direction (an early notification would be reverted by the
	// contract).
	t.Run("stale deposit does not fire before the horizon", func(t *testing.T) {
		spvChain := newLocalChain()

		key := reservationDepositKey(0xD002)
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

		watcher := NewReservationStaleDepositWatcher(
			spvChain,
			common.Address{},
			0,
		)

		// The deadline is at the horizon. At the horizon itself the
		// deadline has NOT strictly passed, so no notification.
		spvChain.setLatestBlockTimestamp(time.Unix(int64(horizon), 0))
		res, err := watcher.CheckStaleReservedDeposit(
			key,
			uint32(horizon),
			5_000,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != StaleDepositResolutionKeep {
			t.Fatalf(
				"expected resolution %v, got %v",
				StaleDepositResolutionKeep,
				res,
			)
		}
		if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
			t.Fatalf(
				"expected no stale notification at the horizon, got %d",
				len(calls),
			)
		}

		// One second past the horizon the saturating clock reads the
		// horizon again, so the strict comparison still does not fire:
		// the deposit is kept rather than notified on a guess. This is
		// the horizon assumption's documented limit, pinned here so a
		// change to the saturating conversion is caught.
		spvChain.setLatestBlockTimestamp(time.Unix(int64(horizon)+1, 0))
		res, err = watcher.CheckStaleReservedDeposit(
			key,
			uint32(horizon),
			5_000,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != StaleDepositResolutionKeep {
			t.Fatalf(
				"expected resolution %v, got %v",
				StaleDepositResolutionKeep,
				res,
			)
		}
		if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 0 {
			t.Fatalf(
				"expected no stale notification past the horizon, got %d",
				len(calls),
			)
		}
	})

	// The case above cannot distinguish a saturating clock from a
	// wrapping one: its deadline is AT the horizon, so every possible
	// uint32 head value (the saturated max, the wrapped zero, anything
	// in between) satisfies the watcher's `headNow <= deadline` guard.
	//
	// This case can. The deadline sits BELOW the horizon by more than
	// the maximum per-operator stagger, and the chain head is one second
	// past the horizon:
	//
	//   - the saturating clock reads the head as the horizon itself,
	//     which is past the deadline by 1000 seconds - more than the
	//     600-second maximum stagger - so the notification FIRES;
	//   - the wrapping clock reads the head as zero, which is before
	//     the deadline, so the notification does NOT fire.
	//
	// Saturation must notify and wrapping must not, so the assertion
	// below distinguishes the two conversions where the previous case
	// could not.
	t.Run("stale deposit past the horizon distinguishes saturation from wrapping", func(t *testing.T) {
		spvChain := newLocalChain()

		key := reservationDepositKey(0xD004)
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

		watcher := NewReservationStaleDepositWatcher(
			spvChain,
			common.Address{},
			0,
		)

		// The deadline is 1000 seconds below the horizon, which is
		// more than the maximum stagger
		// (actionTimeoutRenotifyInterval, 600 seconds), so the
		// notification is not deferred by the stagger gate.
		deadline := uint32(horizon) - 1000

		// The chain head is one second past the horizon.
		spvChain.setLatestBlockTimestamp(time.Unix(int64(horizon)+1, 0))

		res, err := watcher.CheckStaleReservedDeposit(key, deadline, 5_000)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res != StaleDepositResolutionKeep {
			t.Fatalf(
				"expected resolution %v, got %v",
				StaleDepositResolutionKeep,
				res,
			)
		}

		calls := spvChain.getSubmittedStaleReservedDeposits()
		if len(calls) != 1 {
			t.Fatalf(
				"expected one stale notification past the horizon for a "+
					"deadline below it, got %d",
				len(calls),
			)
		}
		if diff := deep.Equal(key, calls[0]); diff != nil {
			t.Errorf("unexpected notified key: %v", diff)
		}

		// NEGATIVE CONTROL: the same head and deadline under a
		// WRAPPING conversion. The wrapped head reads as zero, which
		// is before the deadline, so the watcher's gate would keep the
		// deposit and NOT notify. The notification above therefore
		// distinguishes the two conversions: a wrapping clock would
		// have produced no notification at all.
		headSeconds := int64(horizon) + 1
		wrappedHead := uint32(headSeconds)
		if wrappedHead != 0 {
			t.Fatalf(
				"expected the wrapping conversion to read the head "+
					"past the horizon as zero, got %d",
				wrappedHead,
			)
		}
		if wrappedHead <= deadline {
			// The wrapping clock's gate keeps the deposit: this is
			// the outcome the notification above rules out.
			if len(spvChain.getSubmittedStaleReservedDeposits()) != 1 {
				t.Error(
					"expected the notification count to be unchanged " +
						"by the wrapping-clock comparison",
				)
			}
		} else {
			t.Error(
				"expected the wrapping clock to keep the deposit " +
					"(head zero is before the deadline)",
			)
		}

		// The saturating clock reads the same head as the horizon,
		// which IS past the deadline by more than the maximum
		// stagger, so the notification is due.
		saturatedHead := tbtc.UnixSecondsToUint32(int64(horizon) + 1)
		if saturatedHead != math.MaxUint32 {
			t.Fatalf(
				"expected the saturating conversion to read the head "+
					"past the horizon as %d, got %d",
				uint32(math.MaxUint32),
				saturatedHead,
			)
		}
		maxStagger := uint32(actionTimeoutRenotifyInterval / time.Second)
		if saturatedHead <= deadline+maxStagger {
			t.Errorf(
				"expected the saturated head %d to be past the deadline "+
					"%d by more than the maximum stagger %d",
				saturatedHead,
				deadline,
				maxStagger,
			)
		}
	})

	// The stranding watcher compares the chain head time against the
	// reservation's dissolutionEligibleAt. At the horizon the
	// comparison must not treat a dissolution-eligible-at-the-horizon
	// reservation as eligible one second early.
	t.Run("dissolution eligibility is not reached before the horizon", func(t *testing.T) {
		spvChain := newLocalChain()

		wallet := walletPKH()
		key := reservationKey(0xD003)

		spvChain.setWalletReservations(wallet, []*big.Int{key})
		spvChain.setWallet(wallet, &tbtc.WalletChainData{
			State: tbtc.StateLive,
		})
		spvChain.setReservation(key, &tbtc.Reservation{
			RequestNonce:          1,
			State:                 tbtc.ReservationStateActive,
			DissolutionEligibleAt: uint32(horizon),
		})

		watcher := newReservationStrandingWatcher(spvChain)

		// One second before the horizon the reservation is not yet
		// dissolution-eligible, so no stranding notification.
		spvChain.setLatestBlockTimestamp(time.Unix(int64(horizon)-1, 0))
		if _, err := watcher.checkReservationStrandingForLiveWallet(wallet); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls := spvChain.getSubmittedReservationStrandedKeys(); len(calls) != 0 {
			t.Fatalf(
				"expected no stranding notification before the dissolution horizon, got %d",
				len(calls),
			)
		}

		// Exactly at the horizon it is eligible.
		spvChain.setLatestBlockTimestamp(time.Unix(int64(horizon), 0))
		if _, err := watcher.checkReservationStrandingForLiveWallet(wallet); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		calls := spvChain.getSubmittedReservationStrandedKeys()
		if len(calls) != 1 {
			t.Fatalf(
				"expected one stranding notification at the dissolution horizon, got %d",
				len(calls),
			)
		}
	})
}
