package spv

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/operator"
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
			expectedResolution: StaleDepositResolutionKeep,
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
			watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
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

// mockSigning is a minimal chain.Signing fake exposing a fixed operator
// address. WireReservationWatchers only ever calls Address() on it (to
// derive reservationOperatorStaggerOffset); the remaining methods exist
// only to satisfy the interface and are never exercised by these tests.
type mockSigning struct {
	address chain.Address
}

func (m mockSigning) Address() chain.Address { return m.address }
func (m mockSigning) PublicKey() []byte      { return nil }
func (m mockSigning) Sign(message []byte) ([]byte, error) {
	return nil, nil
}
func (m mockSigning) Verify(message []byte, signature []byte) (bool, error) {
	return false, nil
}
func (m mockSigning) VerifyWithPublicKey(
	message []byte,
	signature []byte,
	publicKey []byte,
) (bool, error) {
	return false, nil
}
func (m mockSigning) PublicKeyToAddress(publicKey *operator.PublicKey) (chain.Address, error) {
	return "", nil
}
func (m mockSigning) PublicKeyBytesToAddress(publicKey []byte) chain.Address { return "" }

func (m *mockWalletClosedChain) Signing() chain.Signing {
	return mockSigning{address: chain.Address("0x0000000000000000000000000000000000000001")}
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

// TestReservationSelfCheckMisconfigured covers
// reservationSelfCheckMisconfigured, the pure function backing
// WireReservationWatchers's misconfiguration self-check. The self-check
// itself now runs asynchronously and reports a confirmed
// misconfiguration via reservationWiringLogger.Errorf from a background
// goroutine (see reservation_wiring.go); this test instead covers the
// extracted pure decision function directly, independent of that
// goroutine's asynchronous timing and logging side effect.
func TestReservationSelfCheckMisconfigured(t *testing.T) {
	definitiveZero := scanResult{count: 0, scanOK: true}
	definitiveActivity := scanResult{count: 1, scanOK: true}
	unreliable := scanResult{count: 0, scanOK: false}

	tests := map[string]struct {
		pairedFlagEnabled                         bool
		registration, staleDeposit, actionTimeout scanResult
		expectMisconfigured                       bool
	}{
		"paired flag enabled always skips regardless of scan results": {
			pairedFlagEnabled: true,
			registration:      definitiveZero,
			staleDeposit:      definitiveZero,
			actionTimeout:     definitiveZero,
		},
		"all three scans definitively found zero activity is misconfigured": {
			pairedFlagEnabled:   false,
			registration:        definitiveZero,
			staleDeposit:        definitiveZero,
			actionTimeout:       definitiveZero,
			expectMisconfigured: true,
		},
		"unreliable registration scan is not misconfigured": {
			pairedFlagEnabled: false,
			registration:      unreliable,
			staleDeposit:      definitiveZero,
			actionTimeout:     definitiveZero,
		},
		"unreliable stale-deposit scan is not misconfigured": {
			pairedFlagEnabled: false,
			registration:      definitiveZero,
			staleDeposit:      unreliable,
			actionTimeout:     definitiveZero,
		},
		"unreliable action-timeout scan is not misconfigured": {
			pairedFlagEnabled: false,
			registration:      definitiveZero,
			staleDeposit:      definitiveZero,
			actionTimeout:     unreliable,
		},
		"nonzero registration activity is not misconfigured": {
			pairedFlagEnabled: false,
			registration:      definitiveActivity,
			staleDeposit:      definitiveZero,
			actionTimeout:     definitiveZero,
		},
		"nonzero stale-deposit activity is not misconfigured": {
			pairedFlagEnabled: false,
			registration:      definitiveZero,
			staleDeposit:      definitiveActivity,
			actionTimeout:     definitiveZero,
		},
		"nonzero action-timeout activity is not misconfigured": {
			pairedFlagEnabled: false,
			registration:      definitiveZero,
			staleDeposit:      definitiveZero,
			actionTimeout:     definitiveActivity,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			actual := reservationSelfCheckMisconfigured(
				test.pairedFlagEnabled,
				test.registration,
				test.staleDeposit,
				test.actionTimeout,
			)
			if actual != test.expectMisconfigured {
				t.Fatalf(
					"expected misconfigured=%v, got %v",
					test.expectMisconfigured,
					actual,
				)
			}
		})
	}
}

// TestWireReservationWatchers_DrivesRealNotifications_NotJustWiringSuccess
// verifies that WireReservationWatchers actually drives the stale-deposit
// and action-timeout watchers to submit real Bridge notifications when
// on-chain data is already overdue at wiring time, via each watcher's
// initial poll (staleDepositWatcher.pollTick and
// actionTimeoutWatcher.pollPendingActions, both run inside backgrounded
// goroutines before the Run loops start - see WireReservationWatchers) -
// not merely that the wiring call itself returns nil. Because those
// initial passes now run asynchronously rather than synchronously before
// WireReservationWatchers returns, the test polls for the expected
// notifications instead of asserting on them immediately. The stranding
// watcher's equivalent startup-scan behavior is already covered by
// TestWireReservationWatchers_StartupCatchUpScan_TransientErrorsDoNotAbort.
func TestWireReservationWatchers_DrivesRealNotifications_NotJustWiringSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walletClosedChain := &mockWalletClosedChain{}
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	// Seed an overdue Reanchor action generation: the action-timeout
	// watcher's backgrounded initial pollPendingActions pass must
	// discover and notify it. TimeoutAt is far in the past relative to
	// the real wall-clock time.Now() WireReservationWatchers uses, so
	// the FIRST-attempt stagger offset (bounded by
	// actionTimeoutRenotifyInterval, 600s) can never mask it.
	actionWallet := walletPKHAt(0x60)
	actionKey := reservationKey(0xAA01)
	spvChain.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            actionKey,
		RequestNonce:              1,
		SourceWalletPublicKeyHash: actionWallet,
		BlockNumber:               500,
	})
	seededReservation(
		t,
		spvChain,
		actionKey,
		actionWallet,
		[]*tbtc.ReservationAction{
			{
				ActionType: tbtc.ReservationActionTypeReanchor,
				State:      tbtc.ReservationActionStatePending,
				TimeoutAt:  100,
			},
		},
		1,
	)

	// Seed an overdue reserved deposit: the stale-deposit watcher's
	// backgrounded initial pollTick pass must discover and notify it.
	depositWallet := walletPKHAt(0x61)
	vault := chain.Address("0xVault")
	spvChain.setReservationParameters(&tbtc.ReservationParameters{
		ReservationVault:         vault,
		ReservationActionTimeout: reservationActionTimeout,
	})
	fundingTxHash := bitcoin.Hash{0x02}
	fundingOutputIndex := uint32(0)
	// currentBlock (1000) does not exceed staleDepositRevealScanLookBackBlocks,
	// so pollTick's own startBlock computation stays 0 - mirror that
	// here rather than subtracting the lookback bound directly (which
	// would underflow uint64 for a currentBlock this small).
	startBlock := uint64(0)
	endBlock := uint64(1000)
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
	spvChain.setReservedDeposit(depositKey, depositWallet, true)
	spvChain.setWallet(depositWallet, &tbtc.WalletChainData{State: tbtc.StateUnknown})
	spvChain.setReservation(depositKey, &tbtc.Reservation{RequestNonce: 1})
	spvChain.setReservationAction(depositKey, 1, &tbtc.ReservationAction{
		State:     tbtc.ReservationActionStatePending,
		TimeoutAt: 100,
	})

	if err := WireReservationWatchers(ctx, walletClosedChain, spvChain, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForReservationWiringCondition(
		t,
		500*time.Millisecond,
		func() bool {
			return len(spvChain.getSubmittedReservationActionTimeouts()) == 1 &&
				len(spvChain.getSubmittedStaleReservedDeposits()) == 1
		},
	)

	if calls := spvChain.getSubmittedReservationActionTimeouts(); len(calls) != 1 {
		t.Fatalf(
			"expected WireReservationWatchers's backgrounded initial poll "+
				"to drive a real NotifyReservationActionTimeout call, got %d",
			len(calls),
		)
	}
	if calls := spvChain.getSubmittedStaleReservedDeposits(); len(calls) != 1 {
		t.Fatalf(
			"expected WireReservationWatchers's backgrounded initial poll "+
				"to drive a real NotifyStaleReservedDeposit call, got %d",
			len(calls),
		)
	}
}

// TestWireReservationWatchers_DrainsStrandingRecheckThroughRealWiring
// verifies that WireReservationWatchers - not a directly-constructed
// ReservationActionTimeoutWatcher, as every existing cross-watcher
// recheck test in reservation_action_timeout_watch_test.go uses -
// actually wires the real strandingWatcher into
// NewReservationActionTimeoutWatcher's constructor argument, end to
// end: a successful Reanchor-timeout notification defers its wallet to
// a stranding recheck (see
// ReservationActionTimeoutWatcher.strandingRecheckWallets), and that
// recheck - drained on the action-timeout watcher's own next poll tick,
// not the wallet's one-shot OnWalletClosed subscription, which has
// already fired and been consumed by this point (see
// strandingRecheckWallets's doc comment) - must notify the reservation
// stranded. reservationActionTimeoutPollInterval is temporarily
// shrunk so the test does not wait on the real one-minute default.
func TestWireReservationWatchers_DrainsStrandingRecheckThroughRealWiring(t *testing.T) {
	originalInterval := reservationActionTimeoutPollInterval
	reservationActionTimeoutPollInterval = 50 * time.Millisecond
	defer func() { reservationActionTimeoutPollInterval = originalInterval }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walletClosedChain := &mockWalletClosedChain{}
	spvChain := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	wallet := walletPKHAt(0x70)
	key := reservationKey(0xBB01)

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
				TimeoutAt:  100,
			},
		},
		1,
	)

	// The wallet already reads Closed, and the reservation already
	// reads Active under it (as ReservationRouter.sol's
	// notifyReservationActionTimeout will restore it to be, once
	// mined), mirroring the production race the deferred recheck exists
	// to catch: the wallet's one-shot OnWalletClosed subscription has
	// already fired and been consumed before the timeout notification
	// below lands, so only the action-timeout watcher's own next poll
	// tick can notice this wallet is stranded (see
	// TestReservationActionTimeoutWatcher_RecheckStrandingAfterActionTimeout_NotifiesWhenWalletClosed
	// in reservation_action_timeout_watch_test.go for the equivalent
	// directly-constructed-watcher scenario this test proves through
	// the real wiring path instead).
	spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateClosed})
	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: wallet,
		RequestNonce:        1,
		State:               tbtc.ReservationStateActive,
	})

	if err := WireReservationWatchers(ctx, walletClosedChain, spvChain, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForReservationWiringCondition(
		t,
		500*time.Millisecond,
		func() bool {
			return len(spvChain.getSubmittedReservationActionTimeouts()) == 1
		},
	)

	waitForReservationWiringCondition(
		t,
		2*time.Second,
		func() bool {
			return len(spvChain.getSubmittedReservationStrandedKeys()) == 1
		},
	)

	stranded := spvChain.getSubmittedReservationStrandedKeys()
	if len(stranded) != 1 || stranded[0].Cmp(key) != 0 {
		t.Fatalf(
			"expected WireReservationWatchers's own action-timeout "+
				"watcher to drain the deferred stranding recheck for "+
				"reservation [%v] through its real strandingWatcher "+
				"wiring, got %v",
			key,
			stranded,
		)
	}
}

// waitForReservationWiringCondition polls cond every 5ms until it
// reports true or timeout elapses, then fails the test. It exists
// because WireReservationWatchers's initial watcher passes now run
// inside backgrounded goroutines (see reservation_wiring.go) rather
// than synchronously before WireReservationWatchers returns, so tests
// asserting on their side effects can no longer rely on a synchronous
// return to guarantee those passes have already completed.
func waitForReservationWiringCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition was not met before timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// blockingActionScanChain wraps a *localChain, blocking inside
// PastReservationAcceptanceRequestedEvents - the first chain read
// actionTimeoutWatcher.pollPendingActions performs - until unblock is
// closed. It embeds *localChain so every other Chain method delegates
// to the normal fake; only this one read is intercepted, letting
// TestWireReservationWatchers_ReturnsBeforeInitialPassesComplete prove
// WireReservationWatchers returns before the action-timeout watcher's
// backgrounded initial pass completes.
type blockingActionScanChain struct {
	*localChain
	unblock chan struct{}
}

func (b *blockingActionScanChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	<-b.unblock
	return b.localChain.PastReservationAcceptanceRequestedEvents(filter)
}

// TestWireReservationWatchers_ReturnsBeforeInitialPassesComplete proves
// that WireReservationWatchers returns before the stale-deposit and
// action-timeout watchers' backgrounded initial passes complete, rather
// than blocking on them synchronously: it wires against a chain double
// that blocks the action-timeout watcher's first chain read
// indefinitely, and asserts WireReservationWatchers still returns well
// within a short timeout.
func TestWireReservationWatchers_ReturnsBeforeInitialPassesComplete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	walletClosedChain := &mockWalletClosedChain{}
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)

	unblock := make(chan struct{})
	defer close(unblock)
	spvChain := &blockingActionScanChain{localChain: newLocalChain(), unblock: unblock}
	spvChain.setBlockCounter(blockCounter)

	done := make(chan error, 1)
	go func() {
		done <- WireReservationWatchers(ctx, walletClosedChain, spvChain, true)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal(
			"expected WireReservationWatchers to return promptly without " +
				"blocking on its backgrounded initial watcher passes, but " +
				"it had not returned after 500ms while the action-timeout " +
				"watcher's initial scan remained blocked",
		)
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

	startBlock := currentBlock - reservationDefaultLookBackBlocks
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

	watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})

	trackedCount, ok := watcher.pollTick(1000)
	if !ok {
		t.Fatal("expected pollTick to report a definitively successful scan")
	}
	if trackedCount != 1 {
		t.Fatalf("expected 1 tracked deposit after the first tick, got %d", trackedCount)
	}
	if len(watcher.pending) != 0 {
		t.Fatalf("expected the live-wallet deposit to be parked, not left pending: %v", watcher.pending)
	}
	if len(watcher.parked) != 1 {
		t.Fatalf("expected 1 parked deposit, got %d: %v", len(watcher.parked), watcher.parked)
	}
}

// TestRunStaleDepositParkedReconcile covers parkedReconcile's two
// outcomes on a parked deposit: reactivation into the pending set once
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

		watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
		key := depositKey.String()
		watcher.parked[key] = depositKey

		watcher.parkedReconcile(1000)

		if len(watcher.parked) != 0 {
			t.Fatalf("expected the deposit to leave the parked set, got %v", watcher.parked)
		}
		if _, ok := watcher.pending[key]; !ok {
			t.Fatalf("expected the deposit to be reactivated into the pending set, got %v", watcher.pending)
		}
	})

	t.Run("evicts once resolved", func(t *testing.T) {
		spvChain := newLocalChain()

		depositKey := reservationDepositKey(0xB011)
		wallet := walletPKHAt(0x23)
		// Not reserved: resolves Drop regardless of wallet state.
		spvChain.setReservedDeposit(depositKey, wallet, false)
		spvChain.setWallet(wallet, &tbtc.WalletChainData{State: tbtc.StateLive})

		watcher := NewReservationStaleDepositWatcher(spvChain, common.Address{})
		key := depositKey.String()
		watcher.parked[key] = depositKey

		watcher.parkedReconcile(1000)

		if len(watcher.parked) != 0 {
			t.Fatalf("expected the resolved deposit to be evicted from the parked set, got %v", watcher.parked)
		}
		if len(watcher.pending) != 0 {
			t.Fatalf("expected the resolved deposit not to reappear in pending, got %v", watcher.pending)
		}
	})
}
