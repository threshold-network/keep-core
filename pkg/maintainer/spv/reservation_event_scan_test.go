package spv

import (
	"errors"
	"fmt"
	"math"
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

// collectChunks runs fetchPastEventsInChunks against f and returns every
// event delivered to onChunk together with the chunkEnd of each delivery,
// in delivery order.
func collectChunks(
	f *rangeRecordingFetcher,
	startBlock, endBlock uint64,
) ([]chunkTestEvent, []uint64, error) {
	var events []chunkTestEvent
	var chunkEnds []uint64
	err := fetchPastEventsInChunks(
		f.fetch,
		startBlock,
		endBlock,
		func(chunk []chunkTestEvent, chunkEnd uint64) {
			events = append(events, chunk...)
			chunkEnds = append(chunkEnds, chunkEnd)
		},
	)
	return events, chunkEnds, err
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

	got, chunkEnds, err := collectChunks(f, 0, endBlock)
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
		if chunkEnds[i] != expectedRanges[i].end {
			t.Errorf(
				"chunk %d delivered chunkEnd %d, expected %d",
				i,
				chunkEnds[i],
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

	got, _, err := collectChunks(f, 0, reservationEventScanChunkSize-1)
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
// is a no-op: neither fetch nor onChunk is called, so a caller's cursor
// does not move.
func TestFetchPastEventsInChunks_InvertedRange(t *testing.T) {
	f := &rangeRecordingFetcher{
		events: []chunkTestEvent{{BlockNumber: 4}},
	}

	got, chunkEnds, err := collectChunks(f, 10, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil || chunkEnds != nil {
		t.Errorf("expected no deliveries for an inverted range, got %v / %v", got, chunkEnds)
	}
	if len(f.ranges) != 0 {
		t.Errorf("expected no chunk fetches for an inverted range, got %v", f.ranges)
	}
}

// TestFetchPastEventsInChunks_ErrorInSecondChunk verifies that an error on
// the second chunk is returned, wrapped with the range that failed, and
// that the first chunk's events were already delivered, so a caller keeps
// that progress and resumes after the first chunk instead of restarting
// the whole walk.
func TestFetchPastEventsInChunks_ErrorInSecondChunk(t *testing.T) {
	f := &rangeRecordingFetcher{
		events: []chunkTestEvent{
			{BlockNumber: 100},
			{BlockNumber: 10000},
		},
		failOn:  2,
		failErr: errors.New("provider rejected the range"),
	}

	got, chunkEnds, err := collectChunks(f, 0, 2*reservationEventScanChunkSize-1)
	if err == nil {
		t.Fatal("expected an error from the second chunk, got nil")
	}
	assertEachEventReturnedOnce(t, got, []chunkTestEvent{{BlockNumber: 100}})
	if len(chunkEnds) != 1 || chunkEnds[0] != reservationEventScanChunkSize-1 {
		t.Errorf(
			"expected only the first chunk [..%d] to be delivered, got %v",
			reservationEventScanChunkSize-1,
			chunkEnds,
		)
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

// TestReservationScanRange covers the scan-range policy shared by the proof
// loop, the stale-deposit watcher and the action-timeout watcher. The
// first scan starts at the activation block wherever it sits relative to
// the tip - an old activation is not cut off by any lookback window,
// because an action that timed out but was never notified still holds
// capacity - an unknown network skips the first scan, and every range
// stops reservationEventScanConfirmationBlocks behind the tip.
func TestReservationScanRange(t *testing.T) {
	const lag = reservationEventScanConfirmationBlocks

	tests := map[string]struct {
		lastScannedBlock uint64
		activationBlock  uint64
		currentBlock     uint64
		expectedStart    uint64
		expectedEnd      uint64
		expectedScan     bool
	}{
		"first scan, activation at genesis (Developer)": {
			activationBlock: 0,
			currentBlock:    1000,
			expectedStart:   0,
			expectedEnd:     1000 - lag,
			expectedScan:    true,
		},
		"first scan, activation inside the last 30 days": {
			activationBlock: 250_000,
			currentBlock:    300_000,
			expectedStart:   250_000,
			expectedEnd:     300_000 - lag,
			expectedScan:    true,
		},
		"first scan, activation older than 30 days": {
			activationBlock: 1_000,
			currentBlock:    1_000_000,
			expectedStart:   1_000,
			expectedEnd:     1_000_000 - lag,
			expectedScan:    true,
		},
		"first scan, activation above the tip": {
			activationBlock: 500_000,
			currentBlock:    300_000,
			expectedStart:   500_000,
			expectedEnd:     300_000 - lag,
			expectedScan:    true,
		},
		"first scan, unknown network": {
			activationBlock: math.MaxUint64,
			currentBlock:    300_000,
			expectedStart:   0,
			expectedEnd:     300_000 - lag,
			expectedScan:    false,
		},
		"later scan starts one block past the cursor": {
			lastScannedBlock: 450_000,
			activationBlock:  0,
			currentBlock:     500_000,
			expectedStart:    450_001,
			expectedEnd:      500_000 - lag,
			expectedScan:     true,
		},
		"later scan on an unknown network covers new blocks": {
			lastScannedBlock: 450_000,
			activationBlock:  math.MaxUint64,
			currentBlock:     500_000,
			expectedStart:    450_001,
			expectedEnd:      500_000 - lag,
			expectedScan:     true,
		},
		"tip within the confirmation depth": {
			activationBlock: 0,
			currentBlock:    lag,
			expectedStart:   0,
			expectedEnd:     0,
			expectedScan:    true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			start, end, scan := reservationScanRange(
				test.lastScannedBlock,
				test.activationBlock,
				test.currentBlock,
			)
			if scan != test.expectedScan {
				t.Errorf("unexpected scan: expected %v, got %v", test.expectedScan, scan)
			}
			if start != test.expectedStart {
				t.Errorf("unexpected start: expected %d, got %d", test.expectedStart, start)
			}
			if end != test.expectedEnd {
				t.Errorf("unexpected end: expected %d, got %d", test.expectedEnd, end)
			}
		})
	}
}
