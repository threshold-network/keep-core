package spv

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestResolveWalletPublicKeyHash covers the three branches of
// resolveWalletPublicKeyHash: a matching NewWalletRegistered event found, no
// matching event found, and a chain read error.
func TestResolveWalletPublicKeyHash(t *testing.T) {
	walletID := [32]byte{0x01, 0x02, 0x03}
	expectedPKH := [20]byte{0xAA, 0xBB, 0xCC}

	t.Run("found", func(t *testing.T) {
		spvChain := newLocalChain()
		spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
			EcdsaWalletID:       walletID,
			WalletPublicKeyHash: expectedPKH,
		})

		pkh, err := resolveWalletPublicKeyHash(spvChain, walletID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pkh != expectedPKH {
			t.Errorf(
				"unexpected public key hash\nexpected: %x\nactual:   %x",
				expectedPKH,
				pkh,
			)
		}
	})

	t.Run("not found", func(t *testing.T) {
		spvChain := newLocalChain()
		// No matching event registered for walletID; a different wallet's
		// event exists to confirm the filter, not just an empty set, drives
		// the not-found path.
		spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
			EcdsaWalletID:       [32]byte{0x99},
			WalletPublicKeyHash: expectedPKH,
		})

		_, err := resolveWalletPublicKeyHash(spvChain, walletID)
		if err == nil {
			t.Fatal("expected error for missing wallet registration event")
		}
	})

	t.Run("chain error", func(t *testing.T) {
		spvChain := newLocalChain()
		spvChain.setPastNewWalletRegisteredEventsErr(
			fmt.Errorf("rpc unavailable"),
		)

		_, err := resolveWalletPublicKeyHash(spvChain, walletID)
		if err == nil {
			t.Fatal("expected chain error to propagate")
		}
	})

	t.Run("duplicate event delivery uses the latest match", func(t *testing.T) {
		spvChain := newLocalChain()
		staleePKH := [20]byte{0x11}
		spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
			EcdsaWalletID:       walletID,
			WalletPublicKeyHash: staleePKH,
		})
		spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
			EcdsaWalletID:       walletID,
			WalletPublicKeyHash: expectedPKH,
		})

		pkh, err := resolveWalletPublicKeyHash(spvChain, walletID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pkh != expectedPKH {
			t.Errorf(
				"expected the latest matching event to win\nexpected: %x\nactual:   %x",
				expectedPKH,
				pkh,
			)
		}
	})
}

// TestCheckStaleReservedDeposit_Resolution covers the resolution outcomes of
// CheckStaleReservedDeposit used by the poller to decide pending-set retention:
// a deposit still reserved with unreached timeout, or reserved with a live
// wallet, must be kept (a live wallet can still transition away from Live
// before anchoring, so the poller must keep re-evaluating it); non-reserved
// deposits or deposits with settled actions must be dropped; and timed-out
// deposits must be notified and evicted.
func TestCheckStaleReservedDeposit_Resolution(t *testing.T) {
	tests := map[string]struct {
		isReserved         bool
		walletState        tbtc.WalletState
		actionState        tbtc.ReservationActionState
		timeoutAt          uint32
		now                uint32
		expectedResolution StaleDepositResolution
	}{
		"not reserved": {
			isReserved:         false,
			walletState:        tbtc.StateMovingFunds,
			actionState:        tbtc.ReservationActionStatePending,
			timeoutAt:          100,
			now:                1000,
			expectedResolution: StaleDepositResolutionDrop,
		},
		// A Live wallet is not yet stale: it may still transition away from
		// Live (e.g. MovingFunds/Closing/Terminated) before anchoring, so
		// the deposit is kept in tracking rather than dropped (see
		// CheckStaleReservedDeposit's Live-wallet branch).
		"reserved, wallet live": {
			isReserved:         true,
			walletState:        tbtc.StateLive,
			actionState:        tbtc.ReservationActionStatePending,
			timeoutAt:          100,
			now:                1000,
			expectedResolution: StaleDepositResolutionKeep,
		},
		"reserved, action settled": {
			isReserved:         true,
			walletState:        tbtc.StateMovingFunds,
			actionState:        tbtc.ReservationActionStateSettled,
			timeoutAt:          100,
			now:                1000,
			expectedResolution: StaleDepositResolutionDrop,
		},
		"reserved, timeout not yet reached": {
			isReserved:         true,
			walletState:        tbtc.StateMovingFunds,
			actionState:        tbtc.ReservationActionStatePending,
			timeoutAt:          5000,
			now:                1000,
			expectedResolution: StaleDepositResolutionKeep,
		},
		"reserved, timeout passed and notified": {
			isReserved:         true,
			walletState:        tbtc.StateMovingFunds,
			actionState:        tbtc.ReservationActionStatePending,
			timeoutAt:          100,
			now:                1000,
			expectedResolution: StaleDepositResolutionNotified,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			spvChain := newLocalChain()
			depositKey := reservationDepositKey(0xCC01)
			wallet := walletPKH()
			spvChain.setReservedDeposit(depositKey, wallet, test.isReserved)
			spvChain.setWallet(wallet, &tbtc.WalletChainData{
				State: test.walletState,
			})
			spvChain.setReservation(depositKey, &tbtc.Reservation{
				RequestNonce: 1,
			})
			spvChain.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
				State:     test.actionState,
				TimeoutAt: test.timeoutAt,
			})
			spvChain.setReservationParameters(&tbtc.ReservationParameters{
				ReservationActionTimeout: 3600,
			})
			watcher := NewReservationStaleDepositWatcher(spvChain)
			resolution, err := watcher.CheckStaleReservedDeposit(depositKey, test.now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resolution != test.expectedResolution {
				t.Errorf(
					"unexpected resolution\nexpected: %v\nactual:   %v",
					test.expectedResolution,
					resolution,
				)
			}
		})
	}
}

type mockWalletClosedChain struct {
	onWalletClosedHandler func(event *tbtc.WalletClosedEvent)
}

func (m *mockWalletClosedChain) OnWalletClosed(
	handler func(event *tbtc.WalletClosedEvent),
) subscription.EventSubscription {
	m.onWalletClosedHandler = handler
	return subscription.NewEventSubscription(func() {})
}

func TestWireReservationWatchers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walletClosedChain := &mockWalletClosedChain{}
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	if err := WireReservationWatchers(ctx, walletClosedChain, spvChain, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWireReservationWatchers_NilParameters(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walletClosedChain := &mockWalletClosedChain{}
	spvChain := newLocalChain()

	t.Run("nil wallet closed chain", func(t *testing.T) {
		err := WireReservationWatchers(ctx, nil, spvChain, true)
		if err == nil {
			t.Fatal("expected error for nil wallet closed chain")
		}
	})

	t.Run("nil spv chain", func(t *testing.T) {
		err := WireReservationWatchers(ctx, walletClosedChain, nil, true)
		if err == nil {
			t.Fatal("expected error for nil spv chain")
		}
	})
}

// TestWireReservationWatchers_StartupCatchUpScan_TransientErrorsDoNotAbort
// verifies that the stranding watcher's startup catch-up scan tolerates a
// transient chain-read failure against one wallet (e.g. GetWallet
// returning an error): the scan continues to the remaining wallets rather
// than aborting client startup, correctly notifying Closed and Terminated
// wallets' stranded reservations while skipping Live ones.
func TestWireReservationWatchers_StartupCatchUpScan_TransientErrorsDoNotAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walletClosedChain := &mockWalletClosedChain{}
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(5000)
	spvChain.setBlockCounter(blockCounter)

	walletTransientError := walletPKHAt(0x01)
	walletClosed := walletPKHAt(0x02)
	walletLive := walletPKHAt(0x03)
	walletTerminated := walletPKHAt(0x04)

	resKeyClosed := reservationKey(0xDD02)
	resKeyLive := reservationKey(0xDD03)
	resKeyTerminated := reservationKey(0xDD04)

	// Register all 4 wallets in NewWalletRegisteredEvents.
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       [32]byte{0x01},
		WalletPublicKeyHash: walletTransientError,
	})
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       [32]byte{0x02},
		WalletPublicKeyHash: walletClosed,
	})
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       [32]byte{0x03},
		WalletPublicKeyHash: walletLive,
	})
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       [32]byte{0x04},
		WalletPublicKeyHash: walletTerminated,
	})

	// walletTransientError is NOT added to spvChain.wallets, so GetWallet
	// returns "no wallet for given PKH" simulating a transient RPC error.

	// walletClosed is Closed with an Active reservation.
	spvChain.setWallet(walletClosed, &tbtc.WalletChainData{
		State: tbtc.StateClosed,
	})
	spvChain.setWalletReservations(walletClosed, []*big.Int{resKeyClosed})
	spvChain.setReservation(resKeyClosed, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	// walletLive is Live with an Active reservation (must be skipped).
	spvChain.setWallet(walletLive, &tbtc.WalletChainData{
		State: tbtc.StateLive,
	})
	spvChain.setWalletReservations(walletLive, []*big.Int{resKeyLive})
	spvChain.setReservation(resKeyLive, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	// walletTerminated is Terminated with an Active reservation.
	spvChain.setWallet(walletTerminated, &tbtc.WalletChainData{
		State: tbtc.StateTerminated,
	})
	spvChain.setWalletReservations(walletTerminated, []*big.Int{resKeyTerminated})
	spvChain.setReservation(resKeyTerminated, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	// WireReservationWatchers must succeed without returning an error despite
	// walletTransientError failing GetWallet.
	err := WireReservationWatchers(ctx, walletClosedChain, spvChain, true)
	if err != nil {
		t.Fatalf("expected WireReservationWatchers to succeed despite transient wallet error: %v", err)
	}

	// Verify that the stranded active reservations for walletClosed and
	// walletTerminated were both notified, while walletLive was skipped.
	notifiedKeys := spvChain.getSubmittedReservationStrandedKeys()
	if len(notifiedKeys) != 2 {
		t.Fatalf("expected 2 notified stranded keys, got %d: %v", len(notifiedKeys), notifiedKeys)
	}

	foundClosed := false
	foundTerminated := false
	for _, k := range notifiedKeys {
		if k.Cmp(resKeyClosed) == 0 {
			foundClosed = true
		}
		if k.Cmp(resKeyTerminated) == 0 {
			foundTerminated = true
		}
		if k.Cmp(resKeyLive) == 0 {
			t.Errorf("live wallet reservation was unexpectedly notified as stranded")
		}
	}

	if !foundClosed {
		t.Errorf("expected closed wallet reservation [%v] to be notified", resKeyClosed)
	}
	if !foundTerminated {
		t.Errorf("expected terminated wallet reservation [%v] to be notified", resKeyTerminated)
	}
}

// TestWireReservationWatchers_SelfCheckHardError verifies the
// misconfiguration self-check: when the caller could not confirm the
// paired process's LeaderDutiesEnabled flag AND all three watchers found
// zero reservation activity on-chain, WireReservationWatchers returns a
// hard error instead of only logging a warning.
func TestWireReservationWatchers_SelfCheckHardError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walletClosedChain := &mockWalletClosedChain{}
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	err := WireReservationWatchers(ctx, walletClosedChain, spvChain, false)
	if err == nil {
		t.Fatal(
			"expected a hard error when the paired flag could not be " +
				"confirmed and all three watchers found zero activity",
		)
	}
}

// TestWireReservationWatchers_SelfCheckSkippedWhenActivityFound verifies
// that finding any reservation activity - here, a single wallet
// registration, even one that never becomes Closed/Terminated - is enough
// to suppress the self-check's hard error, since the zero-activity signal
// alone is required to corroborate the disabled paired flag.
func TestWireReservationWatchers_SelfCheckSkippedWhenActivityFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walletClosedChain := &mockWalletClosedChain{}
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet := walletPKHAt(0x30)
	spvChain.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       [32]byte{0x30},
		WalletPublicKeyHash: wallet,
	})
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateLive})

	if err := WireReservationWatchers(ctx, walletClosedChain, spvChain, false); err != nil {
		t.Fatalf("expected no error when reservation activity is found, got: %v", err)
	}
}

// TestRetryReservationStrandingStartupScan covers
// retryReservationStrandingStartupScan's three outcomes: a wallet that
// becomes fetchable during the retry delay is checked and, if
// stranded, notified; a wallet that remains unfetchable is skipped without
// error; and a context canceled before the delay elapses aborts before any
// wallet is processed.
func TestRetryReservationStrandingStartupScan(t *testing.T) {
	t.Run("resolves once the wallet becomes fetchable", func(t *testing.T) {
		spvChain := newLocalChain()
		strandingWatcher := newReservationStrandingWatcher(spvChain)

		wallet := walletPKHAt(0x50)
		resKey := reservationKey(0xFF01)
		spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateClosed})
		spvChain.setWalletReservations(wallet, []*big.Int{resKey})
		spvChain.setReservation(resKey, &tbtc.Reservation{State: tbtc.ReservationStateActive})

		retryReservationStrandingStartupScan(
			context.Background(),
			spvChain,
			strandingWatcher,
			[][20]byte{wallet},
			0,
		)

		notifiedKeys := spvChain.getSubmittedReservationStrandedKeys()
		if len(notifiedKeys) != 1 || notifiedKeys[0].Cmp(resKey) != 0 {
			t.Fatalf(
				"expected the retry pass to notify reservation [%v] as stranded, got %v",
				resKey,
				notifiedKeys,
			)
		}
	})

	t.Run("still-unresolved wallet is skipped without error", func(t *testing.T) {
		spvChain := newLocalChain()
		strandingWatcher := newReservationStrandingWatcher(spvChain)

		retryReservationStrandingStartupScan(
			context.Background(),
			spvChain,
			strandingWatcher,
			[][20]byte{walletPKHAt(0x51)},
			0,
		)

		if notified := spvChain.getSubmittedReservationStrandedKeys(); len(notified) != 0 {
			t.Fatalf("expected no notifications for a still-unresolved wallet, got %v", notified)
		}
	})

	t.Run("context cancellation before the delay elapses aborts without processing", func(t *testing.T) {
		spvChain := newLocalChain()
		strandingWatcher := newReservationStrandingWatcher(spvChain)

		wallet := walletPKHAt(0x52)
		resKey := reservationKey(0xFF02)
		spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateClosed})
		spvChain.setWalletReservations(wallet, []*big.Int{resKey})
		spvChain.setReservation(resKey, &tbtc.Reservation{State: tbtc.ReservationStateActive})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		retryReservationStrandingStartupScan(
			ctx,
			spvChain,
			strandingWatcher,
			[][20]byte{wallet},
			time.Hour,
		)

		if notified := spvChain.getSubmittedReservationStrandedKeys(); len(notified) != 0 {
			t.Fatalf("expected a canceled context to abort before any notification, got %v", notified)
		}
	})
}

// TestRunStaleDepositPollTick_LiveWalletIsParked verifies that a reserved
// deposit discovered by the poll tick is moved to the parked set, not the
// actively-polled pending set, once its assigned wallet is observed Live.
func TestRunStaleDepositPollTick_LiveWalletIsParked(t *testing.T) {
	spvChain := newLocalChain()

	const currentBlock = uint64(300000)
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	spvChain.setBlockCounter(blockCounter)

	vault := chain.Address("0xVault")
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})

	startBlock := currentBlock - reservationStaleDepositLookBackBlocks
	endBlock := currentBlock
	fundingTxHash := bitcoin.Hash{0x01}
	fundingOutputIndex := uint32(0)

	if err := spvChain.addPastDepositRevealedEvent(
		&tbtc.DepositRevealedEventFilter{StartBlock: startBlock + 1, EndBlock: &endBlock},
		&tbtc.DepositRevealedEvent{
			FundingTxHash:      fundingTxHash,
			FundingOutputIndex: fundingOutputIndex,
			Vault:              &vault,
		},
	); err != nil {
		t.Fatal(err)
	}

	depositKey := spvChain.BuildDepositKey(fundingTxHash, fundingOutputIndex)
	wallet := walletPKHAt(0x21)
	spvChain.setReservedDeposit(depositKey, wallet, true)
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateLive})
	spvChain.setReservation(depositKey, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 5000,
	})

	watcher := NewReservationStaleDepositWatcher(spvChain)
	state := newStaleDepositPollState()

	trackedCount := runStaleDepositPollTick(spvChain, watcher, state, 1000)
	if trackedCount != 1 {
		t.Fatalf("expected 1 tracked deposit after the first tick, got %d", trackedCount)
	}
	if len(state.pending) != 0 {
		t.Fatalf("expected the live-wallet deposit to be parked, not left pending: %v", state.pending)
	}
	if len(state.parked) != 1 {
		t.Fatalf("expected 1 parked deposit, got %d: %v", len(state.parked), state.parked)
	}
}

// TestRunStaleDepositParkedReconcile covers runStaleDepositParkedReconcile's
// two outcomes on a parked deposit: reactivation into the pending set once
// its wallet is no longer Live, and eviction once it resolves Drop.
func TestRunStaleDepositParkedReconcile(t *testing.T) {
	t.Run("reactivates into pending once the wallet leaves live", func(t *testing.T) {
		spvChain := newLocalChain()

		depositKey := reservationDepositKey(0xB010)
		wallet := walletPKHAt(0x22)
		spvChain.setReservedDeposit(depositKey, wallet, true)
		spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
		spvChain.setReservation(depositKey, &tbtc.Reservation{RequestNonce: 1})
		spvChain.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
			State:     tbtc.ReservationActionStatePending,
			TimeoutAt: 5000,
		})
		spvChain.setReservationParameters(&tbtc.ReservationParameters{
			ReservationActionTimeout: reservationActionTimeout,
		})

		watcher := NewReservationStaleDepositWatcher(spvChain)
		state := newStaleDepositPollState()
		key := depositKey.String()
		state.parked[key] = depositKey

		runStaleDepositParkedReconcile(spvChain, watcher, state, 1000)

		if len(state.parked) != 0 {
			t.Fatalf("expected the deposit to leave the parked set, got %v", state.parked)
		}
		if _, ok := state.pending[key]; !ok {
			t.Fatalf("expected the deposit to be reactivated into the pending set, got %v", state.pending)
		}
	})

	t.Run("evicts once resolved", func(t *testing.T) {
		spvChain := newLocalChain()

		depositKey := reservationDepositKey(0xB011)
		wallet := walletPKHAt(0x23)
		// Not reserved: resolves Drop regardless of wallet state.
		spvChain.setReservedDeposit(depositKey, wallet, false)
		spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateLive})

		watcher := NewReservationStaleDepositWatcher(spvChain)
		state := newStaleDepositPollState()
		key := depositKey.String()
		state.parked[key] = depositKey

		runStaleDepositParkedReconcile(spvChain, watcher, state, 1000)

		if len(state.parked) != 0 {
			t.Fatalf("expected the resolved deposit to be evicted from the parked set, got %v", state.parked)
		}
		if len(state.pending) != 0 {
			t.Fatalf("expected the resolved deposit not to reappear in pending, got %v", state.pending)
		}
	})
}
