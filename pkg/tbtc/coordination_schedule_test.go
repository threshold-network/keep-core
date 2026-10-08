package tbtc

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"testing"
	"time"
)

// assumedBlockTime converts the wallet actions' broadcast timeouts into
// Ethereum blocks. Post-merge Ethereum produces a block every 12 seconds.
const assumedBlockTime = 12 * time.Second

// maxBusyBlocks returns the number of blocks, counted from the coordination
// block, during which a wallet action proposed in that window can keep the
// wallet dispatcher busy. The action starts at the end of the coordination
// window, signs until its signing deadline (proposal expiry minus the
// safety margin), and then broadcasts for up to broadcastTimeout.
func maxBusyBlocks(
	validityBlocks uint64,
	signingSafetyMarginBlocks uint64,
	broadcastTimeout time.Duration,
) uint64 {
	return coordinationDurationBlocks +
		validityBlocks -
		signingSafetyMarginBlocks +
		uint64(broadcastTimeout/assumedBlockTime)
}

// TestDepositSweepLeadPeriodCoversSweepBusyTime pins the timing assumption
// behind depositSweepLeadPeriodWindows. A deposit sweep may keep the wallet
// busy past the next window, so a redemption proposed there can be dropped
// with errWalletBusy. The last Redemption-led window before the next
// DepositSweep-led window must start after the sweep's busy time ends, or
// slow sweeps can starve redemptions. Every other action must finish within
// its own window so it never takes the next window's slot.
//
// If this test fails, a proposal validity, signing margin or broadcast
// timeout was raised, and depositSweepLeadPeriodWindows must be raised too.
func TestDepositSweepLeadPeriodCoversSweepBusyTime(t *testing.T) {
	if depositSweepLeadPeriodWindows < 2 {
		t.Fatalf(
			"depositSweepLeadPeriodWindows must leave at least one "+
				"Redemption-led window, got %d",
			depositSweepLeadPeriodWindows,
		)
	}

	sweepBusy := maxBusyBlocks(
		depositSweepProposalValidityBlocks,
		depositSweepSigningTimeoutSafetyMarginBlocks,
		depositSweepBroadcastTimeout,
	)
	lastRedemptionLedWindowStart :=
		uint64(depositSweepLeadPeriodWindows-1) * coordinationFrequencyBlocks

	if sweepBusy >= lastRedemptionLedWindowStart {
		t.Errorf(
			"deposit sweep can keep the wallet busy for %d blocks, "+
				"but the last Redemption-led window before the next "+
				"sweep starts %d blocks after the sweep's window",
			sweepBusy,
			lastRedemptionLedWindowStart,
		)
	}

	otherActions := map[WalletActionType]uint64{
		ActionRedemption: maxBusyBlocks(
			redemptionProposalValidityBlocks,
			redemptionSigningTimeoutSafetyMarginBlocks,
			redemptionBroadcastTimeout,
		),
		ActionMovedFundsSweep: maxBusyBlocks(
			movedFundsSweepProposalValidityBlocks,
			movedFundsSweepSigningTimeoutSafetyMarginBlocks,
			movedFundsSweepBroadcastTimeout,
		),
		ActionMovingFunds: maxBusyBlocks(
			movingFundsProposalValidityBlocks,
			movingFundsSigningTimeoutSafetyMarginBlocks,
			movingFundsBroadcastTimeout,
		),
		ActionReservationAnchor: maxBusyBlocks(
			reservationAnchorProposalValidityBlocks,
			reservationActionSigningTimeoutSafetyMarginBlocks,
			reservationActionBroadcastTimeout,
		),
		ActionReservationReanchor: maxBusyBlocks(
			reservationReanchorProposalValidityBlocks,
			reservationActionSigningTimeoutSafetyMarginBlocks,
			reservationActionBroadcastTimeout,
		),
		ActionHeartbeat: coordinationDurationBlocks +
			heartbeatTotalProposalValidityBlocks,
	}

	for action, busy := range otherActions {
		if busy >= coordinationFrequencyBlocks {
			t.Errorf(
				"%s can keep the wallet busy for %d blocks, past the "+
					"next coordination window at %d blocks",
				action,
				busy,
				coordinationFrequencyBlocks,
			)
		}
	}
}

// TestCoordinationSchedule_SlowSweepsDoNotStarveRedemptions drives the
// post-activation checklist through the real walletDispatcher. A pending
// redemption and sweepable deposits exist on every window, so a
// first-with-work leader (as in tbtcpg.ProposalGenerator.Generate) picks the
// checklist's first action. Every deposit sweep runs until its latest
// possible end, which models multi-input sweeps that use up their signing
// budget. Each window's proposal is dispatched at the window's coordination
// block, the earliest point at which followers can receive it.
//
// Both actions must execute at least once in every
// depositSweepLeadPeriodWindows consecutive windows. With Redemption always
// first, deposit sweeps never execute. With DepositSweep leading every other
// window, every Redemption-led window finds the wallet busy with the
// previous sweep, and redemptions never execute.
func TestCoordinationSchedule_SlowSweepsDoNotStarveRedemptions(t *testing.T) {
	const numWindows = 30

	sweepBusy := maxBusyBlocks(
		depositSweepProposalValidityBlocks,
		depositSweepSigningTimeoutSafetyMarginBlocks,
		depositSweepBroadcastTimeout,
	)
	redemptionBusy := maxBusyBlocks(
		redemptionProposalValidityBlocks,
		redemptionSigningTimeoutSafetyMarginBlocks,
		redemptionBroadcastTimeout,
	)

	executor := &coordinationExecutor{}
	dispatcher := newWalletDispatcher()
	testWallet := generateWallet(big.NewInt(100))

	// running is the action currently held by the dispatcher, if any.
	var running *scheduledWalletAction

	lastExecuted := map[WalletActionType]uint64{}

	// Window 27290 is past DepositSweepEveryWindowActivationBlock.
	firstWindow := uint64(27290)

	for i := uint64(0); i < numWindows; i++ {
		window := newCoordinationWindow(
			(firstWindow + i) * coordinationFrequencyBlocks,
		)
		dispatchBlock := window.coordinationBlock

		if running != nil && running.busyUntilBlock <= dispatchBlock {
			running.finish(t, dispatcher)
			running = nil
		}

		seed := sha256.Sum256(
			big.NewInt(int64(window.coordinationBlock) + 2).Bytes(),
		)
		checklist := executor.getActionsChecklist(
			window.index(),
			seed,
			window.coordinationBlock,
		)

		picked := checklist[0]

		busy := redemptionBusy
		if picked == ActionDepositSweep {
			busy = sweepBusy
		}

		action := newScheduledWalletAction(
			testWallet,
			picked,
			window.coordinationBlock+busy,
		)

		err := dispatcher.dispatch(action)
		switch {
		case err == nil:
			running = action
			lastExecuted[picked] = window.index()
		case err == errWalletBusy:
			// The proposal is dropped, as in the real handlers.
		default:
			t.Fatalf("window %d: unexpected dispatch error: %v", window.index(), err)
		}

		for _, tracked := range []WalletActionType{
			ActionDepositSweep,
			ActionRedemption,
		} {
			last, ok := lastExecuted[tracked]
			if !ok {
				last = firstWindow - 1
			}

			if window.index()-last >= depositSweepLeadPeriodWindows {
				t.Errorf(
					"window %d: %s has not executed for %d windows",
					window.index(),
					tracked,
					window.index()-last,
				)
			}
		}
	}

	if running != nil {
		running.finish(t, dispatcher)
	}
}

// scheduledWalletAction is a wallet action that keeps the dispatcher busy
// until the test releases it.
type scheduledWalletAction struct {
	actionWallet   wallet
	action         WalletActionType
	busyUntilBlock uint64
	release        chan struct{}
}

func newScheduledWalletAction(
	actionWallet wallet,
	action WalletActionType,
	busyUntilBlock uint64,
) *scheduledWalletAction {
	return &scheduledWalletAction{
		actionWallet:   actionWallet,
		action:         action,
		busyUntilBlock: busyUntilBlock,
		release:        make(chan struct{}),
	}
}

func (swa *scheduledWalletAction) execute() error {
	<-swa.release
	return nil
}

func (swa *scheduledWalletAction) wallet() wallet {
	return swa.actionWallet
}

func (swa *scheduledWalletAction) actionType() WalletActionType {
	return swa.action
}

// finish releases the action and waits until the dispatcher frees the
// wallet.
func (swa *scheduledWalletAction) finish(
	t *testing.T,
	dispatcher *walletDispatcher,
) {
	t.Helper()

	close(swa.release)

	key, err := marshalPublicKey(swa.actionWallet.publicKey)
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		dispatcher.actionsMutex.Lock()
		_, busy := dispatcher.actions[hex.EncodeToString(key)]
		dispatcher.actionsMutex.Unlock()

		if !busy {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("dispatcher did not free the wallet after the action finished")
}
