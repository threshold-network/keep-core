package tbtcpg_test

import (
	"math"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestReservationAcceptanceTask_AmountAtInt64Max pins the inclusivity of the
// int64-fit guard on a reserved deposit's amount. The candidate is built
// with the deposit amount as an int64 UTXO value, so an amount of exactly
// math.MaxInt64 still fits and must be proposed; a guard that rejects the
// equality drops a deposit the assembly step could still carry.
func TestReservationAcceptanceTask_AmountAtInt64Max(t *testing.T) {
	walletPublicKeyHash := [20]byte{
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
		0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
		0xaa, 0xaa, 0xaa, 0xaa,
	}

	const currentBlock = uint64(300000)
	head := time.Unix(1700000000, 0)

	ralc := newBoundaryTestChain(t, walletPublicKeyHash, currentBlock, nil)
	ralc.SetLatestBlockTimestamp(head)
	btcChain := tbtcpg.NewLocalBitcoinChain()

	opts := defaultPendingDepositOptions(currentBlock)
	opts.amount = uint64(math.MaxInt64)
	opts.revealedAt = head.Add(-3 * time.Hour)
	opts.timeoutAt = uint32(head.Unix()) + 10000
	deposit := addPendingDeposit(t, ralc, btcChain, walletPublicKeyHash, opts)

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, walletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, deposit, opts.requestNonce)
}
