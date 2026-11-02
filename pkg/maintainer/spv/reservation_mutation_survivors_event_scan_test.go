package spv

import (
	"testing"
)

// TestFetchPastEventsInChunks_SingleBlockRange pins the empty-range
// guard: a range of exactly one block is not empty. Treating it as
// inverted would silently skip the block at the scan boundary, which is
// the block a cursor advance lands on when activation or the confirmed
// tip moves by one.
func TestFetchPastEventsInChunks_SingleBlockRange(t *testing.T) {
	const block = uint64(1234)

	fetches := 0
	chunkEnds := make([]uint64, 0, 1)

	err := fetchPastEventsInChunks(
		func(startBlock, endBlock uint64) ([]uint64, error) {
			fetches++
			if startBlock != block || endBlock != block {
				t.Fatalf(
					"expected the single block [%d] to be fetched, got [%d..%d]",
					block,
					startBlock,
					endBlock,
				)
			}
			return []uint64{block}, nil
		},
		block,
		block,
		func(events []uint64, chunkEnd uint64) {
			chunkEnds = append(chunkEnds, chunkEnd)
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetches != 1 {
		t.Fatalf("expected exactly one fetch for a single-block range, got %d", fetches)
	}
	if len(chunkEnds) != 1 || chunkEnds[0] != block {
		t.Fatalf("expected one delivered chunk ending at %d, got %v", block, chunkEnds)
	}
}
