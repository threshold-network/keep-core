package spv

import (
	"context"
	"fmt"
	"math/big"
	"testing"

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

type mockWalletMembersResolver struct {
	resolveFn func(walletPublicKeyHash [20]byte) ([]uint32, error)
}

func (m *mockWalletMembersResolver) ResolveWalletMembers(walletPublicKeyHash [20]byte) ([]uint32, error) {
	return m.resolveFn(walletPublicKeyHash)
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

	resolver := &mockWalletMembersResolver{
		resolveFn: func(walletPublicKeyHash [20]byte) ([]uint32, error) {
			return []uint32{1, 2, 3}, nil
		},
	}

	if err := WireReservationWatchers(ctx, walletClosedChain, spvChain, resolver); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWireReservationWatchers_NilParameters(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walletClosedChain := &mockWalletClosedChain{}
	spvChain := newLocalChain()
	resolver := &mockWalletMembersResolver{
		resolveFn: func(walletPublicKeyHash [20]byte) ([]uint32, error) {
			return []uint32{1}, nil
		},
	}

	t.Run("nil wallet closed chain", func(t *testing.T) {
		err := WireReservationWatchers(ctx, nil, spvChain, resolver)
		if err == nil {
			t.Fatal("expected error for nil wallet closed chain")
		}
	})

	t.Run("nil spv chain", func(t *testing.T) {
		err := WireReservationWatchers(ctx, walletClosedChain, nil, resolver)
		if err == nil {
			t.Fatal("expected error for nil spv chain")
		}
	})

	t.Run("nil wallet members resolver", func(t *testing.T) {
		err := WireReservationWatchers(ctx, walletClosedChain, spvChain, nil)
		if err == nil {
			t.Fatal("expected error for nil wallet members resolver")
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

	resolver := &mockWalletMembersResolver{
		resolveFn: func(walletPublicKeyHash [20]byte) ([]uint32, error) {
			return []uint32{1, 2, 3}, nil
		},
	}

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
	err := WireReservationWatchers(ctx, walletClosedChain, spvChain, resolver)
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
