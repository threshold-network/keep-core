package tbtcpg

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/ipfs/go-log/v2"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// advanceBlockCounter is a block counter that behaves like a real one once
// the next block is produced: waiting for a height strictly above the
// highest height already reached succeeds and moves the head to it, while
// waiting for a height at or below the head fails, because that block has
// already passed. It records every height it was asked for so a test can
// pin the exact window the caller waits through.
type advanceBlockCounter struct {
	head    uint64
	heights []uint64
}

func (c *advanceBlockCounter) WaitForBlockHeight(blockNumber uint64) error {
	if blockNumber <= c.head {
		return &pastBlockError{blockNumber: blockNumber, head: c.head}
	}
	c.heights = append(c.heights, blockNumber)
	c.head = blockNumber
	return nil
}

type pastBlockError struct {
	blockNumber uint64
	head        uint64
}

func (e *pastBlockError) Error() string {
	return "block height [" + big.NewInt(int64(e.blockNumber)).String() +
		"] is not above the current head [" +
		big.NewInt(int64(e.head)).String() + "]"
}

func (c *advanceBlockCounter) BlockHeightWaiter(blockNumber uint64) (
	<-chan uint64,
	error,
) {
	panic("unsupported")
}

func (c *advanceBlockCounter) CurrentBlock() (uint64, error) {
	return c.head, nil
}

func (c *advanceBlockCounter) WatchBlocks(ctx context.Context) <-chan uint64 {
	panic("unsupported")
}

// TestWaitForReservationReanchorRequestMined_CoversWholeWaitWindow pins the
// bounds of the post-submit wait loop: it starts at the block after the
// head and waits through exactly reservationReanchorRequestWaitBlocks
// blocks, ending at head + reservationReanchorRequestWaitBlocks inclusive.
// A loop that starts at the head (or before it) re-reads blocks that have
// already passed, and a loop that stops one block early stops watching one
// block before the request can no longer be mined inside the window.
func TestWaitForReservationReanchorRequestMined_CoversWholeWaitWindow(t *testing.T) {
	const head = uint64(1000)

	chain := NewLocalChain()
	key := big.NewInt(9101)
	chain.SetReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: [20]byte{0x01},
		State:               tbtc.ReservationStateActive,
		RequestNonce:        7,
	})
	counter := &advanceBlockCounter{head: head}
	chain.SetBlockCounter(counter)

	task := NewReservationReanchorTask(chain, NewLocalBitcoinChain())

	_, err := task.waitForReservationReanchorRequestMined(
		log.Logger("keep-tbtcpg-test"),
		key,
		[20]byte{0x02},
		// The generation the caller requested is the one the chain
		// already carries: the submitted transaction is not mined, so
		// the nonce never advances and the loop runs to its end.
		7,
	)
	if err == nil || !strings.Contains(err.Error(), "not confirmed within") {
		t.Fatalf(
			"expected the wait to end with a not-confirmed error after "+
				"the whole window, got [%v]",
			err,
		)
	}

	wantHeights := []uint64{head + 1, head + 2, head + 3, head + 4, head + 5, head + 6}
	if reservationReanchorRequestWaitBlocks != uint64(len(wantHeights)) {
		t.Fatalf(
			"the recorded window assumes a %d-block wait, the constant is [%d]",
			len(wantHeights),
			reservationReanchorRequestWaitBlocks,
		)
	}
	if len(counter.heights) != len(wantHeights) {
		t.Fatalf(
			"expected %d waited block heights, got %v",
			len(wantHeights),
			counter.heights,
		)
	}
	for i, want := range wantHeights {
		if counter.heights[i] != want {
			t.Fatalf(
				"waited block heights %v, expected %v",
				counter.heights,
				wantHeights,
			)
		}
	}
}

// TestWaitForReservationReanchorRequestMined_IgnoresCurrentGeneration
// pins that the wait loop only accepts a generation that advanced past the
// pre-request nonce. A reservation that is still at the pre-request nonce
// carries the generation that was current when the request was sent -- for
// a reservation that came back from a timed-out generation that is the
// timed-out action itself. Treating a non-advancing generation as the
// requested one reports the stale action as mined and skips the rest of
// the window.
func TestWaitForReservationReanchorRequestMined_IgnoresCurrentGeneration(t *testing.T) {
	chain := NewLocalChain()
	key := big.NewInt(9102)
	// The reservation is back to Active at nonce 7 after its nonce-7
	// generation timed out; the just-submitted request has not been
	// mined, so the chain still reports nonce 7 and its timed-out action.
	chain.SetReservation(key, &tbtc.Reservation{
		WalletPublicKeyHash: [20]byte{0x01},
		State:               tbtc.ReservationStateActive,
		RequestNonce:        7,
	})
	chain.SetReservationAction(key, 7, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStateTimedOut,
		TargetWalletPublicKeyHash: [20]byte{0x02},
	})
	counter := &advanceBlockCounter{head: 2000}
	chain.SetBlockCounter(counter)

	task := NewReservationReanchorTask(chain, NewLocalBitcoinChain())

	reservation, err := task.waitForReservationReanchorRequestMined(
		log.Logger("keep-tbtcpg-test"),
		key,
		[20]byte{0x02},
		7,
	)
	if reservation != nil {
		t.Fatalf("expected no mined reservation, got %+v", reservation)
	}
	if err == nil {
		t.Fatal("expected the wait to time out over the whole window")
	}
	if strings.Contains(err.Error(), "different action") {
		t.Fatalf(
			"the loop treated the pre-request generation as the mined "+
				"request: [%v]",
			err,
		)
	}
	if !strings.Contains(err.Error(), "not confirmed within") {
		t.Fatalf(
			"expected a not-confirmed error after the whole window, got [%v]",
			err,
		)
	}
	if len(counter.heights) != int(reservationReanchorRequestWaitBlocks) {
		t.Fatalf(
			"expected the whole %d-block window to be waited, got %v",
			reservationReanchorRequestWaitBlocks,
			counter.heights,
		)
	}
}

// TestReservationHistoryExistsInChunks_MutationSurvivors_SingleBlockRange
// pins that a single-block range where startBlock == endBlock is queried
// rather than being skipped as empty. A boundary mutation from > to >=
// treats startBlock == endBlock as an empty range and returns false without
// querying the block, dropping events occurring on that single block.
func TestReservationHistoryExistsInChunks_MutationSurvivors_SingleBlockRange(t *testing.T) {
	const block = 100
	fetchCalled := false

	found, err := reservationHistoryExistsInChunks(
		block,
		block,
		func(chunkStartBlock, chunkEndBlock uint64) ([]int, error) {
			fetchCalled = true
			if chunkStartBlock != block || chunkEndBlock != block {
				t.Fatalf("expected query on [%d..%d], got [%d..%d]", block, block, chunkStartBlock, chunkEndBlock)
			}
			return []int{1}, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fetchCalled {
		t.Fatal("expected fetch to be called for single-block range startBlock == endBlock")
	}
	if !found {
		t.Fatal("expected event on single block to be found")
	}
}

// TestReservationHistoryExistsInChunks_MutationSurvivors_ContiguousChunkProgression
// pins that successive chunks progress strictly contiguously without overlapping
// the previous chunk's end block (chunkStartBlock = chunkEndBlock + 1). An arithmetic
// mutation from + 1 to - 1 sets chunkStartBlock = chunkEndBlock - 1, causing redundant
// overlapping queries and corrupting chunk boundaries.
func TestReservationHistoryExistsInChunks_MutationSurvivors_ContiguousChunkProgression(t *testing.T) {
	originalSpan := tbtc.ReservationEventScanMaxBlockSpan
	tbtc.ReservationEventScanMaxBlockSpan = 100
	t.Cleanup(func() { tbtc.ReservationEventScanMaxBlockSpan = originalSpan })

	const start = 1000
	const end = 1250 // produces 3 chunks: [1000..1099], [1100..1199], [1200..1250]

	var queries [][2]uint64

	found, err := reservationHistoryExistsInChunks(
		start,
		end,
		func(chunkStartBlock, chunkEndBlock uint64) ([]string, error) {
			queries = append(queries, [2]uint64{chunkStartBlock, chunkEndBlock})
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected no events found")
	}

	expected := [][2]uint64{
		{1000, 1099},
		{1100, 1199},
		{1200, 1250},
	}
	if len(queries) != len(expected) {
		t.Fatalf("expected %d queries, got %d: %v", len(expected), len(queries), queries)
	}
	for i, exp := range expected {
		if queries[i] != exp {
			t.Fatalf("query %d mismatch: expected %v, got %v", i, exp, queries[i])
		}
	}
}

// TestReservationHistoryExistsInChunks_MutationSurvivors_SingleBlockChunkSpan
// pins that a chunk span of one block yields one query per block. With a
// span of one, chunkEndBlock equals chunkStartBlock, so a boundary mutation
// from < to <= on the wrap guard widens every chunk to the whole remaining
// range and breaks the provider cap the span exists to honour.
func TestReservationHistoryExistsInChunks_MutationSurvivors_SingleBlockChunkSpan(t *testing.T) {
	originalSpan := tbtc.ReservationEventScanMaxBlockSpan
	tbtc.ReservationEventScanMaxBlockSpan = 1
	t.Cleanup(func() { tbtc.ReservationEventScanMaxBlockSpan = originalSpan })

	var queries [][2]uint64

	found, err := reservationHistoryExistsInChunks(
		500,
		502,
		func(chunkStartBlock, chunkEndBlock uint64) ([]string, error) {
			queries = append(queries, [2]uint64{chunkStartBlock, chunkEndBlock})
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Fatal("expected no events found")
	}

	expected := [][2]uint64{{500, 500}, {501, 501}, {502, 502}}
	if len(queries) != len(expected) {
		t.Fatalf("expected %d queries, got %d: %v", len(expected), len(queries), queries)
	}
	for i, exp := range expected {
		if queries[i] != exp {
			t.Fatalf("query %d mismatch: expected %v, got %v", i, exp, queries[i])
		}
	}
}
