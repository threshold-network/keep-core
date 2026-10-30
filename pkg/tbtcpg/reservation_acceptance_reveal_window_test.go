package tbtcpg_test

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestReservationAcceptanceTask_RevealWindowCoversOneTerm pins the width of
// the deposit reveal look-back window. A reserved deposit's reveal can be as
// old as the acceptance generation's own snapshotted custody term -- the
// deposit is revealed, then reserved, then accepted, and the term is the
// bound on how long it can sit reserved -- so the window is one term plus a
// margin, and a reveal inside one term of the request block must be found.
//
// Shrinking the window below one term hides reveals the generation is still
// allowed to consume, and the acceptance never happens for a deposit the
// contract would accept.
func TestReservationAcceptanceTask_RevealWindowCoversOneTerm(t *testing.T) {
	walletPublicKeyHash := [20]byte{
		0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99,
		0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99, 0x99,
		0x99, 0x99, 0x99, 0x99,
	}

	const currentBlock = uint64(1000000)
	const blockTime = 12 * time.Second

	// One snapshotted term in blocks, and a reveal well inside it.
	termBlocks := uint64(
		(time.Duration(testReservationTermSeconds) * time.Second) / blockTime,
	)
	const requestBlock = currentBlock - 10
	revealBlock := requestBlock - termBlocks + 100

	head := time.Unix(1700000000, 0)

	ralc := newBoundaryTestChain(
		t,
		walletPublicKeyHash,
		currentBlock,
		func(ralc *reservationAcceptanceLocalChain) {
			ralc.SetAverageBlockTime(blockTime)
			ralc.SetLatestBlockTimestamp(head)
		},
	)
	btcChain := tbtcpg.NewLocalBitcoinChain()

	opts := defaultPendingDepositOptions(currentBlock)
	opts.revealedAt = head.Add(-3 * time.Hour)
	opts.revealBlock = revealBlock
	opts.requestBlock = requestBlock
	opts.timeoutAt = uint32(head.Unix()) + 10000
	deposit := addPendingDeposit(t, ralc, btcChain, walletPublicKeyHash, opts)

	// The reveal sits inside one snapshotted term of the request, so it is
	// inside the window and the deposit must be proposed.
	proposal, shouldExecute, err := runTask(t, ralc, btcChain, walletPublicKeyHash)
	expectProposalFor(t, proposal, shouldExecute, err, deposit, opts.requestNonce)
}