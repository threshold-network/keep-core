package spv

// This file covers the liveness of the stranding pipeline: a transient
// failure anywhere on it keeps the wallet queued for another pass instead of
// dropping it. The wallet-close event that would otherwise bring the wallet
// back is one-shot and never replayed, so a drop here is permanent.

import (
	"context"
	"errors"
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	ethereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestStrandingNotifyFailureKeepsWalletQueued pins that a failed
// NotifyReservationStranded call flags the wallet for another pass, and that
// the flag survives into the deferred recheck queue. Only the transport
// failed: the reservation is still Active and still custodied by a Closed
// wallet, so the notification has to be attempted again. Dropping the flag
// left the reservation unnotified for good, with its capacity pinned,
// because nothing else ever revisits a wallet whose close event has already
// been consumed.
func TestStrandingNotifyFailureKeepsWalletQueued(t *testing.T) {
	spvChain := newLocalChain()
	wallet := walletPKHAt(0x61)
	key := reservationKey(0xD101)

	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateClosed})
	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: wallet,
		State:               tbtc.ReservationStateActive,
	})
	notifyErr := errors.New("rpc unavailable")
	spvChain.notifyReservationStrandedErrByKey = map[string]error{
		key.String(): notifyErr,
	}

	strandingWatcher := newReservationStrandingWatcher(spvChain)
	strandingWatcher.retryDelay = 0

	checked, recheck, err := strandingWatcher.checkStrandingForWalletState(
		wallet,
		tbtc.StateClosed,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !checked {
		t.Fatal("expected a Closed wallet to be checked")
	}
	if !recheck {
		t.Fatal(
			"expected the failed stranding notification to flag the wallet " +
				"for another pass",
		)
	}

	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		0,
		common.Address{},
		strandingWatcher,
		0,
		nil,
	)
	ratw.ScheduleStrandingRecheck(wallet)

	ratw.drainStrandingRechecks()
	if !strandingRecheckQueued(ratw, wallet) {
		t.Fatal(
			"expected the wallet whose notification failed to stay queued " +
				"for the next drain",
		)
	}
	if notified := spvChain.getSubmittedReservationStrandedKeys(); len(notified) != 0 {
		t.Fatalf("expected no successful notification yet, got %v", notified)
	}

	// The transport recovers: the next drain retries the notification.
	spvChain.notifyReservationStrandedErrByKey = nil
	ratw.drainStrandingRechecks()

	notified := spvChain.getSubmittedReservationStrandedKeys()
	if len(notified) != 1 || notified[0].Cmp(key) != 0 {
		t.Fatalf(
			"expected the retried notification to land for [%v], got %v",
			key,
			notified,
		)
	}
	if strandingRecheckQueued(ratw, wallet) {
		t.Fatal("expected the settled wallet to leave the recheck queue")
	}
}

// failingReservationsChain fails WalletReservations while its flag is set,
// modelling an RPC that is down for the length of the startup scan and
// recovers afterwards.
type failingReservationsChain struct {
	*localChain
	fails atomic.Bool
}

func (c *failingReservationsChain) WalletReservations(
	walletPublicKeyHash [20]byte,
) ([]*big.Int, error) {
	if c.fails.Load() {
		return nil, errors.New("transient RPC failure")
	}
	return c.localChain.WalletReservations(walletPublicKeyHash)
}

// TestWireReservationWatchers_StartupScanKeepsUnsettledWalletQueued pins
// that the startup catch-up scan hands a wallet whose check it could not
// complete to the action-timeout watcher's recheck queue. The scan runs once
// per process start and the wallet's close event has already been emitted,
// so a wallet dropped here is never revisited: its stranded reservations
// stay unnotified and their capacity stays pinned for the process lifetime.
func TestWireReservationWatchers_StartupScanKeepsUnsettledWalletQueued(
	t *testing.T,
) {
	originalInterval := getReservationActionTimeoutPollInterval()
	setReservationActionTimeoutPollInterval(25 * time.Millisecond)
	defer func() { setReservationActionTimeoutPollInterval(originalInterval) }()

	originalRetryDelay := getReservationStrandingRetryDelay()
	setReservationStrandingRetryDelay(time.Millisecond)
	defer func() { setReservationStrandingRetryDelay(originalRetryDelay) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	base := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(5000)
	base.setBlockCounter(blockCounter)

	wallet := walletPKHAt(0x62)
	key := reservationKey(0xD102)

	base.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       [32]byte{0x62},
		WalletPublicKeyHash: wallet,
		BlockNumber:         4000,
	})
	base.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateClosed})
	base.setWalletReservations(wallet, []*big.Int{key})
	base.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: wallet,
		State:               tbtc.ReservationStateActive,
	})

	chain := &failingReservationsChain{localChain: base}
	chain.fails.Store(true)

	if err := WireReservationWatchers(
		ctx,
		&mockWalletClosedChain{},
		chain,
		true,
		nil,
		ethereum.Developer,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The reservation read recovers after the startup scan gave up on it.
	chain.fails.Store(false)

	waitForReservationWiringCondition(
		t,
		2*time.Second,
		func() bool {
			return len(base.getSubmittedReservationStrandedKeys()) == 1
		},
	)

	stranded := base.getSubmittedReservationStrandedKeys()
	if stranded[0].Cmp(key) != 0 {
		t.Fatalf("expected stranded reservation [%v], got %v", key, stranded)
	}
}

// TestWalletClosedHandlerResolveFailureKeepsWalletQueued pins that a closed
// wallet whose ECDSA wallet ID could not be resolved to a public key hash is
// queued by ID instead of dropped, and that the drain resolves it once the
// registration read recovers. The close event is one-shot: without the
// queued ID the wallet would leave stranding coverage entirely, and every
// reservation it custodied would stay unnotified.
func TestWalletClosedHandlerResolveFailureKeepsWalletQueued(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(5000)
	spvChain.setBlockCounter(blockCounter)

	wallet := walletPKHAt(0x63)
	walletID := [32]byte{0x63}
	key := reservationKey(0xD103)

	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       walletID,
		WalletPublicKeyHash: wallet,
		BlockNumber:         4000,
	})
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateClosed})
	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: wallet,
		State:               tbtc.ReservationStateActive,
	})

	strandingWatcher := newReservationStrandingWatcher(spvChain)
	strandingWatcher.retryDelay = time.Millisecond

	ratw := NewReservationActionTimeoutWatcher(
		spvChain,
		0,
		common.Address{},
		strandingWatcher,
		0,
		nil,
	)

	walletClosedChain := &mockWalletClosedChain{}
	subscribeReservationWalletClosed(
		ctx,
		walletClosedChain,
		spvChain,
		strandingWatcher,
		ratw,
	)

	// The registration read is down while the close event arrives.
	spvChain.pastNewWalletRegisteredEventsErr = errors.New("rpc unavailable")
	walletClosedChain.onWalletClosedHandler(&tbtc.WalletClosedEvent{
		WalletID: walletID,
	})

	waitForReservationWiringCondition(
		t,
		2*time.Second,
		func() bool { return strandingRecheckWalletIDQueued(ratw, walletID) },
	)

	if notified := spvChain.getSubmittedReservationStrandedKeys(); len(notified) != 0 {
		t.Fatalf(
			"expected no notification while the wallet ID is unresolved, got %v",
			notified,
		)
	}

	// The registration read recovers: the drain resolves the ID and runs
	// the stranding check for the wallet it names.
	spvChain.pastNewWalletRegisteredEventsErr = nil
	ratw.drainStrandingRechecks()

	notified := spvChain.getSubmittedReservationStrandedKeys()
	if len(notified) != 1 || notified[0].Cmp(key) != 0 {
		t.Fatalf(
			"expected the queued wallet ID to resolve and notify [%v], got %v",
			key,
			notified,
		)
	}
	if strandingRecheckWalletIDQueued(ratw, walletID) {
		t.Fatal("expected the resolved wallet ID to leave the recheck queue")
	}
}