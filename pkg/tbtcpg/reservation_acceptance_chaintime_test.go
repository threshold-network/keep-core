package tbtcpg_test

import (
	"errors"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestReservationAcceptanceTask_MinAgeGateUsesChainTime verifies that the
// minimum-age gate uses the chain head timestamp. When the host clock runs
// ahead of the chain, the old code's wall-clock min-age check matures a
// deposit the chain would not yet accept, causing a proposal the validator
// rejects. The new code's chain-time check correctly skips such a deposit.
//
// G1-S2.
func TestReservationAcceptanceTask_MinAgeGateUsesChainTime(t *testing.T) {
	walletPublicKeyHash := [20]byte{
		0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10,
		0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10, 0x10,
		0x10, 0x10, 0x10, 0x10,
	}

	t.Run("fast host clock: no early action (old code proposes, validator rejects)", func(t *testing.T) {
		// Chain head is 2 hours behind the host wall clock.
		// The deposit was revealed at chainHead - 1h and the minimum age is
		// 2h, so it matures at chainHead + 1h.
		// On chain: chainHead < chainHead + 1h -> not mature.
		// On the host clock: wall = chainHead + 2h > chainHead + 1h -> mature.
		// Base code (wall clock) treats the deposit as mature, proposes, and
		// the validator rejects it; chain-time code skips it.
		wall := time.Now()
		chainHead := wall.Add(-2 * time.Hour)

		btcChain := tbtcpg.NewLocalBitcoinChain()
		ralc := newBoundaryTestChain(t, walletPublicKeyHash, 300000, nil)
		ralc.SetLatestBlockTimestamp(chainHead)

		opts := defaultPendingDepositOptions(300000)
		// Event and action timeouts far in the future: both margin gates pass.
		opts.timeoutAt = uint32(wall.Add(24 * time.Hour).Unix())
		// Deposit revealed 1 hour before chain head (3 hours before wall).
		opts.revealedAt = chainHead.Add(-time.Hour)
		addPendingDeposit(t, ralc, btcChain, walletPublicKeyHash, opts)

		proposal, shouldExecute, err := runTask(t, ralc, btcChain, walletPublicKeyHash)
		// New code: min-age gate skips (chainHead not after revealedAt + minAge).
		expectNoProposal(t, proposal, shouldExecute, err)

		// Old code: wall > revealedAt + 2h (wall > wall + 1h) → skips min-age gate,
		// reads reservation/action, proposes → validator (chain time) rejects →
		// validateCalls >= 1. New code skips at min-age → validateCalls = 0.
		// Asserting validateCalls == 0 fails on old code.
		if ralc.validateCalls != 0 {
			t.Fatalf("expected no validation calls, got %d", ralc.validateCalls)
		}
	})
}

// failingTimestampAcceptanceChain wraps reservationAcceptanceLocalChain and
// makes GetLatestBlockTimestamp fail, modelling a node that cannot reach
// the Ethereum chain.
type failingTimestampAcceptanceChain struct {
	*reservationAcceptanceLocalChain
	err error
}

func (f *failingTimestampAcceptanceChain) GetLatestBlockTimestamp() (time.Time, error) {
	return time.Time{}, f.err
}

// TestReservationAcceptanceTask_ChainTimeReadFailureSkipsPass verifies that
// a failure to read the chain head timestamp causes the entire pass to fail
// closed, returning an error so the coordinator retries.
func TestReservationAcceptanceTask_ChainTimeReadFailureSkipsPass(t *testing.T) {
	walletPublicKeyHash := [20]byte{
		0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
		0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
		0x20, 0x20, 0x20, 0x20,
	}

	failingChain := &failingTimestampAcceptanceChain{
		reservationAcceptanceLocalChain: newReservationAcceptanceLocalChain(),
		err:                             errors.New("chain head timestamp unavailable"),
	}
	failingChain.SetReservationParameters(defaultTestReservationParameters())
	failingChain.SetDepositMinAge(testDepositMinAgeSeconds)
	failingChain.SetAverageBlockTime(12 * time.Second)

	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(300000)
	failingChain.SetBlockCounter(blockCounter)

	failingChain.SetWallet(walletPublicKeyHash, &tbtc.WalletChainData{
		State: tbtc.StateLive,
	})

	btcChain := tbtcpg.NewLocalBitcoinChain()
	task := tbtcpg.NewReservationAcceptanceTask(failingChain, btcChain)

	proposal, shouldExecute, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	})
	// On read error, Run must return an error (fail closed, retry next pass).
	if err == nil {
		t.Fatalf("expected an error on chain head read failure, got nil")
	}
	if shouldExecute || proposal != nil {
		t.Fatalf("expected no proposal on head read failure, got shouldExecute=%v proposal=%v",
			shouldExecute, proposal)
	}
}
