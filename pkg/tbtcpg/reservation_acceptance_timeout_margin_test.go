package tbtcpg_test

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestReservationAcceptanceTask_TimeoutMarginBoundaries pins the two
// timeout safety margin gates of the acceptance path on their own edges.
//
// WalletProposalValidator signs a pending acceptance generation only while
// block.timestamp < TimeoutAt - REQUEST_TIMEOUT_SAFETY_MARGIN. The pass
// applies that rule twice: once on the request event's own TimeoutAt,
// before any reservation read, and once on the generation's action
// TimeoutAt. Both must skip at the equality point, because at
// block.timestamp == TimeoutAt - margin the signing window has closed.
func TestReservationAcceptanceTask_TimeoutMarginBoundaries(t *testing.T) {
	walletPublicKeyHash := [20]byte{
		0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
		0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
		0x77, 0x77, 0x77, 0x77,
	}

	const currentBlock = uint64(300000)
	head := time.Unix(1700000000, 0)
	margin := uint32(7200)

	newFixture := func(t *testing.T) (
		*reservationAcceptanceLocalChain,
		*tbtcpg.LocalBitcoinChain,
		*pendingDeposit,
	) {
		t.Helper()

		ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)
		ralc.SetLatestBlockTimestamp(head)
		btcChain := tbtcpg.NewLocalBitcoinChain()

		opts := defaultPendingDepositOptions(currentBlock)
		opts.revealedAt = head.Add(-3 * time.Hour)
		// The generation's action expires exactly at the margin edge.
		opts.timeoutAt = uint32(head.Unix()) + margin
		deposit := addPendingDeposit(t, ralc, btcChain, walletPublicKeyHash, opts)

		return ralc, btcChain, deposit
	}

	t.Run("request event timeout at the margin edge is skipped before any read", func(t *testing.T) {
		ralc, btcChain, deposit := newFixture(t)

		// The event carries the margin-edge timeout, the generation's
		// action a later one, so only the event gate can skip it.
		ralc.acceptanceEvents[len(ralc.acceptanceEvents)-1].TimeoutAt =
			uint32(head.Unix()) + margin
		ralc.SetReservationAction(deposit.depositKey, 1, &tbtc.ReservationAction{
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			State:                     tbtc.ReservationActionStatePending,
			TargetWalletPublicKeyHash: walletPublicKeyHash,
			TxMaxFee:                  5000,
			MinAmount:                 1000,
			TermSeconds:               testReservationTermSeconds,
			TimeoutAt:                 uint32(head.Unix()) + 10000,
			Amount:                    2000000,
		})

		proposal, shouldExecute, err := runTask(t, ralc, btcChain, walletPublicKeyHash)
		expectNoProposal(t, proposal, shouldExecute, err)

		if ralc.getReservationCalls != 0 {
			t.Fatalf(
				"expected the event margin gate to skip the deposit before "+
					"reading its reservation, got %d reads",
				ralc.getReservationCalls,
			)
		}
		if ralc.validateCalls != 0 {
			t.Fatalf(
				"expected no validation for a deposit at the event margin "+
					"edge, got %d calls",
				ralc.validateCalls,
			)
		}
	})

	t.Run("generation action timeout at the margin edge is skipped", func(t *testing.T) {
		ralc, btcChain, _ := newFixture(t)

		// The event carries a later timeout, the generation's action the
		// margin edge, so only the action gate can skip it.
		ralc.acceptanceEvents[len(ralc.acceptanceEvents)-1].TimeoutAt =
			uint32(head.Unix()) + 10000

		proposal, shouldExecute, err := runTask(t, ralc, btcChain, walletPublicKeyHash)
		expectNoProposal(t, proposal, shouldExecute, err)

		if ralc.getReservationCalls == 0 {
			t.Fatal("expected the deposit's reservation to be read")
		}
		if ralc.validateCalls != 0 {
			t.Fatalf(
				"expected the action margin gate to skip the generation "+
					"before on-chain validation, got %d calls",
				ralc.validateCalls,
			)
		}
	})
}