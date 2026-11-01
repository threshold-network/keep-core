package spv

// This file covers the liveness of the deferred stranding recheck queue:
// one drain at the head of a poll tick re-examines a bounded number of
// wallets, so a backlog cannot push the deadline-critical action-timeout
// batch behind an unbounded number of full reservation scans.

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// strandingRecheckQueued reports whether wallet is waiting on the watcher's
// stranding recheck queue.
func strandingRecheckQueued(
	ratw *ReservationActionTimeoutWatcher,
	wallet [20]byte,
) bool {
	if ratw == nil {
		return false
	}

	ratw.strandingRecheckMutex.Lock()
	defer ratw.strandingRecheckMutex.Unlock()

	if ratw.strandingRecheckQueue == nil {
		return false
	}

	return ratw.strandingRecheckQueue.has(strandingRecheckTarget{
		walletPublicKeyHash: wallet,
	})
}

// strandingRecheckWalletIDQueued reports whether the given ECDSA wallet ID is
// waiting on the watcher's stranding recheck queue.
func strandingRecheckWalletIDQueued(
	ratw *ReservationActionTimeoutWatcher,
	walletID [32]byte,
) bool {
	if ratw == nil {
		return false
	}

	ratw.strandingRecheckMutex.Lock()
	defer ratw.strandingRecheckMutex.Unlock()

	if ratw.strandingRecheckQueue == nil {
		return false
	}

	return ratw.strandingRecheckQueue.has(strandingRecheckTarget{
		walletID: walletID,
	})
}

// TestDrainStrandingRechecksIsBoundedPerTick pins the per-tick budget of the
// stranding recheck drain. Each recheck is a GetWallet plus a full
// WalletReservations scan plus a GetReservation per candidate key, and the
// drain runs at the head of the poll tick, ahead of the deadline-critical
// action-timeout batch. Draining the whole queue in one tick made a tick's
// cost grow with the backlog and held the timeout notices behind it; the
// budget keeps the tick's cost fixed and leaves the remaining wallets queued
// for later ticks.
func TestDrainStrandingRechecksIsBoundedPerTick(t *testing.T) {
	const walletCount = 2 * reservationStrandingRecheckMaxWalletsPerTick

	base := newLocalChain()

	// Live wallets with no reservations: each is read and settles at once,
	// so a recheck neither notifies nor re-queues, and the only thing the
	// assertions below can observe is how many wallets one drain touched.
	wallets := make([][20]byte, 0, walletCount)
	for i := range walletCount {
		wallet := walletPKHAt(byte(0x70 + i))
		wallets = append(wallets, wallet)
		base.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateLive})
	}

	chain := &walletCountingChain{localChain: base}

	ratw := NewReservationActionTimeoutWatcher(
		chain,
		0,
		common.Address{},
		newReservationStrandingWatcher(chain),
		0,
		nil,
	)
	for _, wallet := range wallets {
		ratw.ScheduleStrandingRecheck(wallet)
	}

	ratw.drainStrandingRechecks()

	if chain.walletFetches != reservationStrandingRecheckMaxWalletsPerTick {
		t.Fatalf(
			"expected one drain to re-check %d wallets, got %d",
			reservationStrandingRecheckMaxWalletsPerTick,
			chain.walletFetches,
		)
	}

	queued := 0
	for _, wallet := range wallets {
		if strandingRecheckQueued(ratw, wallet) {
			queued++
		}
	}
	if expected := walletCount - reservationStrandingRecheckMaxWalletsPerTick; queued != expected {
		t.Fatalf(
			"expected %d wallets to stay queued after one bounded drain, got %d",
			expected,
			queued,
		)
	}

	// The queue drains oldest-first, so the next tick takes the wallets the
	// first one left behind and no wallet is starved.
	ratw.drainStrandingRechecks()

	if expected := 2 * reservationStrandingRecheckMaxWalletsPerTick; chain.walletFetches != expected {
		t.Fatalf(
			"expected two drains to re-check %d wallets, got %d",
			expected,
			chain.walletFetches,
		)
	}
	for _, wallet := range wallets {
		if strandingRecheckQueued(ratw, wallet) {
			t.Fatalf(
				"expected wallet [0x%x] to have been drained after two ticks",
				wallet,
			)
		}
	}
}

// TestDrainStrandingRechecksChecksARequeuedWalletOncePerDrain pins that the
// per-tick budget counts distinct wallets, not attempts. A recheck that fails
// at the GetWallet read re-queues its wallet, and the wallet was previously
// popped again inside the same drain: one wallet behind a flapping provider
// then consumed the whole budget, with a retry pause between every attempt,
// while every other queued wallet waited for the next tick.
func TestDrainStrandingRechecksChecksARequeuedWalletOncePerDrain(t *testing.T) {
	setReservationStrandingRetryDelay(0)
	defer setReservationStrandingRetryDelay(DefaultReservationStrandingRetryDelay)

	// A wallet with no chain entry fails every GetWallet, which is the
	// observable shape of a transient read failure on a recheck.
	base := newLocalChain()
	chain := &walletCountingChain{localChain: base}
	wallet := walletPKHAt(0x33)

	ratw := NewReservationActionTimeoutWatcher(
		chain,
		0,
		common.Address{},
		newReservationStrandingWatcher(chain),
		0,
		nil,
	)
	ratw.ScheduleStrandingRecheck(wallet)

	ratw.drainStrandingRechecks()

	if chain.walletFetches != 1 {
		t.Fatalf(
			"expected one drain to check a failing wallet once, got %d reads",
			chain.walletFetches,
		)
	}
	if !strandingRecheckQueued(ratw, wallet) {
		t.Fatal("expected a failed recheck to leave the wallet queued")
	}

	// The retry is deferred, not dropped: the next tick re-examines it.
	ratw.drainStrandingRechecks()

	if chain.walletFetches != 2 {
		t.Fatalf(
			"expected a second drain to check the wallet once more, got %d reads",
			chain.walletFetches,
		)
	}
}
