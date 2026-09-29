package spv

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// chunkTestEvent is the event type fetchPastEventsInChunks is driven with in
// tests, standing in for the reservation event types the helper serves.
type chunkTestEvent struct {
	BlockNumber uint64
}

// recordedFetchRange captures one chunked fetch call's inclusive block range.
type recordedFetchRange struct {
	start uint64
	end   uint64
}

// rangeRecordingFetcher records every chunk range it is asked to fetch and
// returns, from a fixed complete event set, the events whose block number
// falls inside that range, so tests can assert on both the returned events
// and the fetch plan.
type rangeRecordingFetcher struct {
	events  []chunkTestEvent
	ranges  []recordedFetchRange
	failOn  int // 1-based index of the chunk to fail on; 0 = never
	failErr error
}

func (f *rangeRecordingFetcher) fetch(
	chunkStart, chunkEnd uint64,
) ([]chunkTestEvent, error) {
	f.ranges = append(f.ranges, recordedFetchRange{chunkStart, chunkEnd})
	if f.failOn == len(f.ranges) {
		return nil, fmt.Errorf("chunk error [%v]", f.failErr)
	}

	var chunk []chunkTestEvent
	for _, event := range f.events {
		if event.BlockNumber >= chunkStart && event.BlockNumber <= chunkEnd {
			chunk = append(chunk, event)
		}
	}
	return chunk, nil
}

// assertContiguousCoverage verifies that the recorded fetch ranges are
// contiguous, non-overlapping, each within the chunk bound, and jointly
// cover [start, end] without gaps.
func assertContiguousCoverage(
	t *testing.T,
	ranges []recordedFetchRange,
	start, end uint64,
) {
	t.Helper()
	for i, r := range ranges {
		if r.end < r.start {
			t.Errorf("range %d is inverted: %v", i, r)
		}
		if r.end-r.start+1 > reservationEventScanChunkSize {
			t.Errorf(
				"range %d spans %d blocks, above the chunk bound of %d",
				i,
				r.end-r.start+1,
				reservationEventScanChunkSize,
			)
		}
		if i == 0 && r.start != start {
			t.Errorf("first range starts at %d, expected %d", r.start, start)
		}
		if i > 0 && r.start != ranges[i-1].end+1 {
			t.Errorf(
				"range %d starts at %d, expected %d after the previous range",
				i,
				r.start,
				ranges[i-1].end+1,
			)
		}
	}
	if ranges[len(ranges)-1].end != end {
		t.Errorf(
			"last range ends at %d, expected %d",
			ranges[len(ranges)-1].end,
			end,
		)
	}
}

// assertEachEventReturnedOnce verifies that every event block number appears
// exactly once in the returned events and nothing else was returned.
func assertEachEventReturnedOnce(
	t *testing.T,
	got []chunkTestEvent,
	expected []chunkTestEvent,
) {
	t.Helper()
	if len(got) != len(expected) {
		t.Fatalf("expected %d events, got %d", len(expected), len(got))
	}
	seen := make(map[uint64]int, len(got))
	for _, e := range got {
		seen[e.BlockNumber]++
	}
	for _, e := range expected {
		if seen[e.BlockNumber] != 1 {
			t.Errorf(
				"event at block %d returned %d times, expected exactly once",
				e.BlockNumber,
				seen[e.BlockNumber],
			)
		}
	}
}

// TestFetchPastEventsInChunks_ChunkBoundaries drives a range spanning three
// chunks and asserts that events sitting on the chunk boundaries - the last
// block of chunk 1, the first block of chunk 2, and the later chunk 2/3
// boundary - are each returned exactly once, and that the fetch ranges are
// contiguous, non-overlapping, within the chunk bound, and jointly cover
// the full range.
func TestFetchPastEventsInChunks_ChunkBoundaries(t *testing.T) {
	const (
		chunkSize = reservationEventScanChunkSize
		lastOf1   = chunkSize - 1 // last block of chunk 1
		firstOf2  = chunkSize     // first block of chunk 2
		lastOf2   = 2*chunkSize - 1
		firstOf3  = 2 * chunkSize
		endBlock  = 3*chunkSize - 1
	)

	f := &rangeRecordingFetcher{
		events: []chunkTestEvent{
			{BlockNumber: lastOf1},
			{BlockNumber: firstOf2},
			{BlockNumber: lastOf2},
			{BlockNumber: firstOf3},
		},
	}

	got, err := fetchPastEventsInChunks(f.fetch, 0, endBlock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEachEventReturnedOnce(t, got, f.events)
	assertContiguousCoverage(t, f.ranges, 0, endBlock)

	expectedRanges := []recordedFetchRange{
		{start: 0, end: lastOf1},
		{start: firstOf2, end: lastOf2},
		{start: firstOf3, end: endBlock},
	}
	if len(f.ranges) != len(expectedRanges) {
		t.Fatalf("expected %d chunk fetches, got %d: %v",
			len(expectedRanges), len(f.ranges), f.ranges)
	}
	for i, r := range f.ranges {
		if r != expectedRanges[i] {
			t.Errorf(
				"chunk %d fetched [%d..%d], expected [%d..%d]",
				i,
				r.start,
				r.end,
				expectedRanges[i].start,
				expectedRanges[i].end,
			)
		}
	}
}

// TestFetchPastEventsInChunks_SingleChunk drives a range within one chunk
// and asserts that exactly one fetch covering the full range is issued and
// that boundary events are each returned once.
func TestFetchPastEventsInChunks_SingleChunk(t *testing.T) {
	f := &rangeRecordingFetcher{
		events: []chunkTestEvent{
			{BlockNumber: 0},
			{BlockNumber: reservationEventScanChunkSize - 1},
		},
	}

	got, err := fetchPastEventsInChunks(
		f.fetch,
		0,
		reservationEventScanChunkSize-1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEachEventReturnedOnce(t, got, f.events)
	assertContiguousCoverage(t, f.ranges, 0, reservationEventScanChunkSize-1)

	if len(f.ranges) != 1 {
		t.Fatalf("expected a single chunk fetch, got %d: %v", len(f.ranges), f.ranges)
	}
	if f.ranges[0].start != 0 || f.ranges[0].end != reservationEventScanChunkSize-1 {
		t.Errorf("expected chunk [0..%d], got %v", reservationEventScanChunkSize-1, f.ranges[0])
	}
}

// TestFetchPastEventsInChunks_InvertedRange verifies that an inverted range
// is a no-op: no fetch is issued and an empty result is returned.
func TestFetchPastEventsInChunks_InvertedRange(t *testing.T) {
	f := &rangeRecordingFetcher{
		events: []chunkTestEvent{{BlockNumber: 4}},
	}

	got, err := fetchPastEventsInChunks(f.fetch, 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected no events for an inverted range, got %v", got)
	}
	if len(f.ranges) != 0 {
		t.Errorf("expected no chunk fetches for an inverted range, got %v", f.ranges)
	}
}

// TestFetchPastEventsInChunks_ErrorInSecondChunk verifies that an error on
// the second chunk is returned, wrapped with the range that failed, and
// that no partial result is returned.
func TestFetchPastEventsInChunks_ErrorInSecondChunk(t *testing.T) {
	f := &rangeRecordingFetcher{
		events: []chunkTestEvent{
			{BlockNumber: 100},
			{BlockNumber: 10000},
		},
		failOn:  2,
		failErr: errors.New("provider rejected the range"),
	}

	got, err := fetchPastEventsInChunks(
		f.fetch,
		0,
		2*reservationEventScanChunkSize-1,
	)
	if err == nil {
		t.Fatal("expected an error from the second chunk, got nil")
	}
	if got != nil {
		t.Errorf("expected no partial result, got %v events", len(got))
	}
	expectedRange := fmt.Sprintf(
		"[%d..%d]",
		reservationEventScanChunkSize,
		2*reservationEventScanChunkSize-1,
	)
	if !strings.Contains(err.Error(), "fetching past events in chunk "+expectedRange) {
		t.Errorf(
			"expected the error to be wrapped with the failing chunk range %s, got: %v",
			expectedRange,
			err,
		)
	}
	if len(f.ranges) != 2 {
		t.Fatalf("expected two chunk fetches (second failed), got %v", f.ranges)
	}
	assertContiguousCoverage(
		t,
		f.ranges,
		0,
		2*reservationEventScanChunkSize-1,
	)
}
