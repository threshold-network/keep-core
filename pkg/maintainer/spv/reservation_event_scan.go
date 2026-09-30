package spv

import (
	"fmt"
	"math"
)

// reservationEventScanChunkSize bounds the inclusive block range of a
// single Past*Events call in the reservation event scans. The first scan
// can stretch from the network's reservation activation block to the
// current tip, which on a long-running network is millions of blocks; a
// single getLogs across that span is not usable. 10,000 blocks keeps
// every individual call within the eth_getLogs range cap enforced by
// many common third-party RPC providers, so a provider with that cap
// still makes progress chunk by chunk.
//
// Steady-state incremental scans (cursor + 1 to the confirmed tip) are
// far smaller than this chunk size and issue a single call.
const reservationEventScanChunkSize uint64 = 10000

// reservationEventScanConfirmationBlocks is how far behind the current
// chain tip every reservation event scan stops. A scan cursor never
// moves past a block that could still be replaced by a short reorg, so
// an event that only appears in a replacement block is still picked up
// by a later scan. Every reservation event starts a deadline measured in
// hours or days, so discovering it a dozen blocks late costs nothing.
const reservationEventScanConfirmationBlocks uint64 = 12

// reservationScanRange returns the inclusive block range
// [startBlock, endBlock] the next reservation event scan covers, given
// the scan's cursor (the last block already scanned, 0 before the first
// scan), the network's reservation activation block (see
// tbtc.ReservationsActivationBlock) and the current chain tip.
//
// endBlock trails currentBlock by reservationEventScanConfirmationBlocks.
// The first scan starts at activationBlock, so a process started long
// after activation still finds every event emitted since then; later
// scans start one block past the cursor. When activation is still ahead
// of endBlock the range is inverted, fetchPastEventsInChunks treats it
// as a no-op and the cursor stays 0, so the next scan starts at
// activation again.
//
// scan is false only for the first scan on a network with no activation
// entry (math.MaxUint64): reservations are inactive there, so no
// catch-up scan runs and the caller should move its cursor straight to
// endBlock. Later scans on such a network cover only new blocks.
func reservationScanRange(
	lastScannedBlock uint64,
	activationBlock uint64,
	currentBlock uint64,
) (startBlock uint64, endBlock uint64, scan bool) {
	if currentBlock > reservationEventScanConfirmationBlocks {
		endBlock = currentBlock - reservationEventScanConfirmationBlocks
	}

	if lastScannedBlock != 0 {
		return lastScannedBlock + 1, endBlock, true
	}

	if activationBlock == math.MaxUint64 {
		return 0, endBlock, false
	}

	return activationBlock, endBlock, true
}

// nextReservationScanRange reads the current block from spvChain and
// returns reservationScanRange for the given cursor and activation block.
func nextReservationScanRange(
	spvChain Chain,
	lastScannedBlock uint64,
	activationBlock uint64,
) (startBlock uint64, endBlock uint64, scan bool, err error) {
	blockCounter, err := spvChain.BlockCounter()
	if err != nil {
		return 0, 0, false, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return 0, 0, false, fmt.Errorf("failed to get current block: [%v]", err)
	}

	startBlock, endBlock, scan = reservationScanRange(
		lastScannedBlock,
		activationBlock,
		currentBlock,
	)
	return startBlock, endBlock, scan, nil
}

// fetchPastEventsInChunks walks the inclusive block range
// [startBlock, endBlock] in chunks of at most
// reservationEventScanChunkSize blocks, in ascending order. After each
// chunk is fetched it calls onChunk with that chunk's events and last
// block, so the caller can merge the events and advance its cursor
// chunk by chunk: a fetch error stops the walk, but the chunks before it
// are kept and the next scan resumes after the last one delivered. The
// error is returned wrapped with the failing chunk range. An inverted
// range (startBlock > endBlock) is a no-op that calls neither fetch nor
// onChunk.
//
// The caller builds a fetch closure that constructs the
// event-type-specific filter, so one implementation serves every
// reservation Past*Events call.
func fetchPastEventsInChunks[T any](
	fetch func(startBlock, endBlock uint64) ([]T, error),
	startBlock, endBlock uint64,
	onChunk func(events []T, chunkEnd uint64),
) error {
	if startBlock > endBlock {
		return nil
	}

	for chunkStart := startBlock; ; {
		chunkEnd := chunkStart + reservationEventScanChunkSize - 1
		if chunkEnd > endBlock || chunkEnd < chunkStart {
			chunkEnd = endBlock
		}

		chunk, err := fetch(chunkStart, chunkEnd)
		if err != nil {
			return fmt.Errorf(
				"fetching past events in chunk [%d..%d]: [%w]",
				chunkStart, chunkEnd, err,
			)
		}
		onChunk(chunk, chunkEnd)

		if chunkEnd == endBlock {
			return nil
		}
		chunkStart = chunkEnd + 1
	}
}
