package tbtcpg_test

import (
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// TestReservationAcceptanceTask_RequestScanChunks pins the chunk boundaries
// of the acceptance request-event scan. Providers cap the block range of one
// log query at DepositRevealLookupChunkBlocks, so a window wider than that
// is scanned in chunks of exactly that size, and the remainder of the window
// is its own chunk.
//
// The window here is exactly two chunks wide, which is the only width where
// the chunk cap is decided on an exact multiple: a cap that treats "the rest
// of the window equals one chunk" as "the rest fits in one chunk plus one"
// asks for one block too many in the second query and the provider rejects
// it.
func TestReservationAcceptanceTask_RequestScanChunks(t *testing.T) {
	walletPublicKeyHash := [20]byte{
		0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88,
		0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88,
		0x88, 0x88, 0x88, 0x88,
	}

	chunk := tbtc.DepositRevealLookupChunkBlocks
	// The scan's window is the action timeout in blocks plus the request
	// look-back margin in blocks. A one-second block time and a 12800
	// second action timeout put the window at 12800 + 7200 blocks, so the
	// chain head at 2 * chunk blocks makes the window exactly the whole
	// chain and exactly two chunks wide.
	const currentBlock = uint64(20000)
	if 2*chunk != currentBlock {
		t.Fatalf("this test assumes a window of %d blocks", currentBlock)
	}

	ralc := newBoundaryTestChain(
		t,
		walletPublicKeyHash,
		currentBlock,
		func(ralc *reservationAcceptanceLocalChain) {
			ralc.SetAverageBlockTime(1 * time.Second)
			ralc.SetReservationParameters(tbtc.ReservationParameters{
				ReservationVault:          testReservationVaultAddress,
				ReservationMinAmount:      1000,
				ReservationTxMaxFee:       5000,
				MaxReservationsPerWallet:  5,
				ReservationMaxTotalAmount: 100000000,
				ReservationTermSeconds:    testReservationTermSeconds,
				ReservationActionTimeout:  uint32(currentBlock - 7200),
			})
		},
	)
	btcChain := tbtcpg.NewLocalBitcoinChain()

	proposal, shouldExecute, err := runTask(t, ralc, btcChain, walletPublicKeyHash)
	expectNoProposal(t, proposal, shouldExecute, err)

	wantRanges := [][2]uint64{
		{0, chunk - 1},
		{chunk, 2*chunk - 1},
		{2 * chunk, 2 * chunk},
	}
	if len(ralc.acceptanceEventFilters) != len(wantRanges) {
		t.Fatalf(
			"expected %d chunk queries, got %d: %+v",
			len(wantRanges),
			len(ralc.acceptanceEventFilters),
			ralc.acceptanceEventFilters,
		)
	}
	for i, want := range wantRanges {
		filter := ralc.acceptanceEventFilters[i]
		if filter.StartBlock != want[0] {
			t.Fatalf(
				"chunk %d: expected StartBlock [%d], got [%d]",
				i,
				want[0],
				filter.StartBlock,
			)
		}
		if filter.EndBlock == nil {
			t.Fatalf("chunk %d: expected a bounded query", i)
		}
		if *filter.EndBlock != want[1] {
			t.Fatalf(
				"chunk %d: expected EndBlock [%d], got [%d]",
				i,
				want[1],
				*filter.EndBlock,
			)
		}
		if width := *filter.EndBlock - filter.StartBlock + 1; width > chunk {
			t.Fatalf(
				"chunk %d covers %d blocks, more than the %d-block query cap",
				i,
				width,
				chunk,
			)
		}
	}
}