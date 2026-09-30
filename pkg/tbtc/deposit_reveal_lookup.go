package tbtc

import (
	"fmt"
	"time"
)

// DepositRevealLookupChunkBlocks is the maximum block range a single
// PastDepositRevealedEvents call made by FindDepositRevealedEvent spans.
// Many Ethereum providers reject or truncate log queries over wider
// ranges.
const DepositRevealLookupChunkBlocks = uint64(10000)

// DepositRevealLookupDefaultBlockTime is the block time
// FindDepositRevealedEventByRevealTime assumes when the caller does not
// know one: Ethereum's post-merge slot time.
const DepositRevealLookupDefaultBlockTime = 12 * time.Second

// depositRevealLookupMinMarginBlocks is the smallest padding
// FindDepositRevealedEventByRevealTime applies on each side of the
// estimated reveal block.
const depositRevealLookupMinMarginBlocks = uint64(1000)

// depositRevealLookupWidenFactor is how much wider than the first window
// the single retry window of FindDepositRevealedEventByRevealTime is.
const depositRevealLookupWidenFactor = uint64(4)

// DepositRevealedEventSource is the chain surface the deposit reveal
// lookup helpers need.
type DepositRevealedEventSource interface {
	PastDepositRevealedEvents(
		filter *DepositRevealedEventFilter,
	) ([]*DepositRevealedEvent, error)
}

// FindDepositRevealedEvent scans the DepositRevealed events of the given
// wallet in the block range [fromBlock, toBlock] and returns the first
// event for which match returns true, or nil if none does. The range is
// scanned newest chunk first, at most DepositRevealLookupChunkBlocks per
// call, and the scan stops at the first match.
func FindDepositRevealedEvent(
	source DepositRevealedEventSource,
	walletPublicKeyHash [20]byte,
	fromBlock uint64,
	toBlock uint64,
	match func(event *DepositRevealedEvent) bool,
) (*DepositRevealedEvent, error) {
	if fromBlock > toBlock {
		return nil, nil
	}

	chunkEnd := toBlock
	for {
		chunkStart := fromBlock
		if chunkEnd-fromBlock >= DepositRevealLookupChunkBlocks {
			chunkStart = chunkEnd - DepositRevealLookupChunkBlocks + 1
		}

		end := chunkEnd
		events, err := source.PastDepositRevealedEvents(
			&DepositRevealedEventFilter{
				StartBlock:          chunkStart,
				EndBlock:            &end,
				WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot get deposit revealed events in blocks [%d, %d]: [%w]",
				chunkStart,
				chunkEnd,
				err,
			)
		}

		for _, event := range events {
			if match(event) {
				return event, nil
			}
		}

		if chunkStart == fromBlock {
			return nil, nil
		}
		chunkEnd = chunkStart - 1
	}
}

// FindDepositRevealedEventByRevealTime returns the DepositRevealed event
// of the given wallet for which match returns true, locating it from the
// deposit's on-chain reveal timestamp instead of scanning the wallet's
// whole reveal history. The reveal block is estimated as
// currentBlock - (now - revealedAt) / averageBlockTime and padded on each
// side by 5% of the estimated distance (at least
// depositRevealLookupMinMarginBlocks). If the event is not in that window,
// the two outer strips of a window depositRevealLookupWidenFactor times
// wider are scanned once before giving up with a nil event. A
// non-positive averageBlockTime is replaced by
// DepositRevealLookupDefaultBlockTime.
func FindDepositRevealedEventByRevealTime(
	source DepositRevealedEventSource,
	walletPublicKeyHash [20]byte,
	revealedAt time.Time,
	now time.Time,
	currentBlock uint64,
	averageBlockTime time.Duration,
	match func(event *DepositRevealedEvent) bool,
) (*DepositRevealedEvent, error) {
	if averageBlockTime <= 0 {
		averageBlockTime = DepositRevealLookupDefaultBlockTime
	}

	elapsed := now.Sub(revealedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	blocksAgo := uint64(elapsed / averageBlockTime)

	estimatedBlock := uint64(0)
	if currentBlock > blocksAgo {
		estimatedBlock = currentBlock - blocksAgo
	}

	margin := blocksAgo / 20
	if margin < depositRevealLookupMinMarginBlocks {
		margin = depositRevealLookupMinMarginBlocks
	}

	window := func(margin uint64) (uint64, uint64) {
		from := uint64(0)
		if estimatedBlock > margin {
			from = estimatedBlock - margin
		}
		to := estimatedBlock + margin
		if to > currentBlock {
			to = currentBlock
		}
		return from, to
	}

	from, to := window(margin)
	event, err := FindDepositRevealedEvent(
		source,
		walletPublicKeyHash,
		from,
		to,
		match,
	)
	if err != nil || event != nil {
		return event, err
	}

	wideFrom, wideTo := window(margin * depositRevealLookupWidenFactor)
	if to < wideTo {
		event, err = FindDepositRevealedEvent(
			source,
			walletPublicKeyHash,
			to+1,
			wideTo,
			match,
		)
		if err != nil || event != nil {
			return event, err
		}
	}
	if wideFrom < from {
		return FindDepositRevealedEvent(
			source,
			walletPublicKeyHash,
			wideFrom,
			from-1,
			match,
		)
	}

	return nil, nil
}
