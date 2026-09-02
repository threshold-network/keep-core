package spv

import (
	"context"
	"fmt"
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

// TestIsPendingStaleDepositResolved covers the three reachable outcomes of
// the eviction predicate: a deposit still reserved on a non-Live wallet must
// stay pending (false), a deposit no longer reserved (released or swept)
// must be evicted (true), and a still-reserved deposit whose wallet reached
// StateLive must be evicted (true).
func TestIsPendingStaleDepositResolved(t *testing.T) {
	tests := map[string]struct {
		isReserved       bool
		walletState      tbtc.WalletState
		expectedResolved bool
	}{
		"still reserved, wallet not live": {
			isReserved:       true,
			walletState:      tbtc.StateMovingFunds,
			expectedResolved: false,
		},
		"released (no longer reserved)": {
			isReserved:       false,
			walletState:      tbtc.StateMovingFunds,
			expectedResolved: true,
		},
		"swept (no longer reserved), wallet already live": {
			isReserved:       false,
			walletState:      tbtc.StateLive,
			expectedResolved: true,
		},
		"still reserved, wallet now live": {
			isReserved:       true,
			walletState:      tbtc.StateLive,
			expectedResolved: true,
		},
		"still reserved, wallet closing": {
			isReserved:       true,
			walletState:      tbtc.StateClosing,
			expectedResolved: false,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			resolved := isPendingStaleDepositResolved(
				test.isReserved,
				test.walletState,
			)
			if resolved != test.expectedResolved {
				t.Errorf(
					"unexpected resolved value\nexpected: %v\nactual:   %v",
					test.expectedResolved,
					resolved,
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

// stubTbtcChain implements only the tbtc.Chain methods
// WireReservationWatchers's synchronous startup path actually calls
// (OnWalletClosed, to register the stranding watcher's live subscription).
// Every other tbtc.Chain method is unreachable from that synchronous path
// within this test's lifetime and is left to the embedded nil interface,
// which would panic if ever invoked - an intentional signal that the test
// has started exercising a code path it does not yet stub.
type stubTbtcChain struct {
	tbtc.Chain
}

func (s *stubTbtcChain) OnWalletClosed(
	handler func(event *tbtc.WalletClosedEvent),
) subscription.EventSubscription {
	return subscription.NewEventSubscription(func() {})
}

func TestWireReservationWatchers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tbtcChain := &stubTbtcChain{}
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	resolver := &mockWalletMembersResolver{
		resolveFn: func(walletPublicKeyHash [20]byte) ([]uint32, error) {
			return []uint32{1, 2, 3}, nil
		},
	}

	if err := WireReservationWatchers(ctx, tbtcChain, spvChain, resolver); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
