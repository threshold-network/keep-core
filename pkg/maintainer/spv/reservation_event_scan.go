package spv

import (
	"fmt"
	"math"
)

// reservationEventScanChunkSize bounds the inclusive block range of a
// single Past*Events call in the reservation startup catch-up scans. The
// startup scan can stretch from the network's reservation activation
// block to the current tip, which on a long-running network is millions
// of blocks; a single getLogs across that span is not usable. 10,000
// blocks keeps every individual call within the eth_getLogs range cap
// enforced by many common third-party RPC providers, so a provider with
// that cap still makes progress chunk by chunk instead of the whole
// walk stalling on the first rejected range (an aborted chunk discards
// the earlier chunks' progress).
//
// Steady-state incremental scans (cursor + 1 to currentBlock) are far
// smaller than this chunk size and continue to issue a single call.
const reservationEventScanChunkSize uint64 = 10000

// reservationStartupScanStartBlock returns the first block the
// reservation startup catch-up scan covers, together with a flag
// indicating whether a scan is warranted at all.
//
// activationBlock is the network's reservation activation block height
// (see tbtc.ReservationsActivationBlock). When it equals math.MaxUint64
// the network has no reservations-activation entry and the feature is
// inactive for that network; the startup scan is skipped entirely - no
// reservation events can exist when reservations are inactive, and
// silently scanning an arbitrary window of unrelated history would just
// burn RPC quota.
//
// Otherwise the scan starts at activationBlock. If activation is still
// in the future, startBlock is greater than the current tip; the chunk
// helper treats that inverted range as a no-op. Keeping the future
// activation block intact also prevents callers from scanning
// pre-activation history.
//
// The boolean is false only for an inactive network, where the caller
// should not attempt a scan at all.
func reservationStartupScanStartBlock(activationBlock uint64) (uint64, bool) {
	if activationBlock == math.MaxUint64 {
		return 0, false
	}
	return activationBlock, true
}

// fetchPastEventsInChunks walks the inclusive block range
// [startBlock, endBlock] in chunks of at most
// reservationEventScanChunkSize blocks, calling fetch for each chunk
// and concatenating the returned events in ascending block order. A
// fetch error stops the walk and is returned wrapped with the chunk
// range that triggered it. An inverted range (startBlock > endBlock)
// is a no-op and yields a nil slice without calling fetch, matching
// the conventions of the Past*Events methods on the spv chain
// implementations and giving callers a single conditional-free call
// pattern: compute the range, hand it to this helper, done.
//
// The generic parameter T lets a single implementation serve all three
// reservation Past*Events fetches (acceptance requested, reanchor
// requested, deposit revealed) without per-type boilerplate. The
// caller builds a closure that constructs the event-type-specific
// filter, so the helper does not need to know the filter shape.
func fetchPastEventsInChunks[T any](
	fetch func(startBlock, endBlock uint64) ([]T, error),
	startBlock, endBlock uint64,
) ([]T, error) {
	if startBlock > endBlock {
		return nil, nil
	}

	var all []T
	for chunkStart := startBlock; ; {
		chunkEnd := chunkStart + reservationEventScanChunkSize - 1
		if chunkEnd > endBlock {
			chunkEnd = endBlock
		}

		chunk, err := fetch(chunkStart, chunkEnd)
		if err != nil {
			return nil, fmt.Errorf(
				"fetching past events in chunk [%d..%d]: [%w]",
				chunkStart, chunkEnd, err,
			)
		}
		all = append(all, chunk...)

		if chunkEnd == endBlock {
			break
		}
		chunkStart = chunkEnd + 1
	}
	return all, nil
}
