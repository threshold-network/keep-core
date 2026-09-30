package tbtc

import (
	"fmt"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
)

// rangeDepositRevealedEvents answers PastDepositRevealedEvents the way an
// Ethereum node does: every registered event whose block lies in
// [StartBlock, EndBlock] and whose wallet matches the filter. It rejects
// unbounded queries and queries wider than DepositRevealLookupChunkBlocks,
// so a lookup that stops chunking fails loudly instead of silently
// passing against a lenient fake. It records every queried range.
type rangeDepositRevealedEvents struct {
	events  []*DepositRevealedEvent
	queries [][2]uint64
}

func (r *rangeDepositRevealedEvents) PastDepositRevealedEvents(
	filter *DepositRevealedEventFilter,
) ([]*DepositRevealedEvent, error) {
	if filter == nil || filter.EndBlock == nil {
		return nil, fmt.Errorf("unbounded deposit revealed events query")
	}
	if filter.StartBlock > *filter.EndBlock {
		return nil, fmt.Errorf(
			"inverted block range [%d, %d]",
			filter.StartBlock,
			*filter.EndBlock,
		)
	}
	if *filter.EndBlock-filter.StartBlock+1 > DepositRevealLookupChunkBlocks {
		return nil, fmt.Errorf(
			"block range [%d, %d] wider than one chunk",
			filter.StartBlock,
			*filter.EndBlock,
		)
	}
	r.queries = append(r.queries, [2]uint64{filter.StartBlock, *filter.EndBlock})

	var result []*DepositRevealedEvent
	for _, event := range r.events {
		if event.BlockNumber < filter.StartBlock ||
			event.BlockNumber > *filter.EndBlock {
			continue
		}
		walletMatch := len(filter.WalletPublicKeyHash) == 0
		for _, wallet := range filter.WalletPublicKeyHash {
			if wallet == event.WalletPublicKeyHash {
				walletMatch = true
			}
		}
		if walletMatch {
			result = append(result, event)
		}
	}
	return result, nil
}

// rangeRevealChain is a localChain whose DepositRevealed events are served
// by rangeDepositRevealedEvents.
type rangeRevealChain struct {
	*localChain
	reveals *rangeDepositRevealedEvents
}

func newRangeRevealChain(events ...*DepositRevealedEvent) *rangeRevealChain {
	return &rangeRevealChain{
		localChain: Connect(),
		reveals:    &rangeDepositRevealedEvents{events: events},
	}
}

func (c *rangeRevealChain) PastDepositRevealedEvents(
	filter *DepositRevealedEventFilter,
) ([]*DepositRevealedEvent, error) {
	return c.reveals.PastDepositRevealedEvents(filter)
}

func matchFundingTxHash(hash bitcoin.Hash) func(*DepositRevealedEvent) bool {
	return func(event *DepositRevealedEvent) bool {
		return event.FundingTxHash == hash
	}
}

// TestFindDepositRevealedEvent_ChunkBoundaries pins the chunked scan: the
// range is covered newest chunk first without gaps or overlaps, an event
// on either edge of a chunk is found, and the scan stops at the first
// match instead of reading older chunks.
func TestFindDepositRevealedEvent_ChunkBoundaries(t *testing.T) {
	wallet := [20]byte{0x01}
	target := bitcoin.Hash{0xaa}

	tests := map[string]struct {
		eventBlock      uint64
		expectedQueries [][2]uint64
	}{
		"newest block of the newest chunk": {
			eventBlock:      50000,
			expectedQueries: [][2]uint64{{40001, 50000}},
		},
		"oldest block of the newest chunk": {
			eventBlock:      40001,
			expectedQueries: [][2]uint64{{40001, 50000}},
		},
		"newest block of the second chunk": {
			eventBlock: 40000,
			expectedQueries: [][2]uint64{
				{40001, 50000},
				{30001, 40000},
			},
		},
		"range start inside a partial last chunk": {
			eventBlock: 25000,
			expectedQueries: [][2]uint64{
				{40001, 50000},
				{30001, 40000},
				{25000, 30000},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			source := &rangeDepositRevealedEvents{
				events: []*DepositRevealedEvent{{
					BlockNumber:         test.eventBlock,
					WalletPublicKeyHash: wallet,
					FundingTxHash:       target,
				}},
			}

			event, err := FindDepositRevealedEvent(
				source,
				wallet,
				25000,
				50000,
				matchFundingTxHash(target),
			)
			if err != nil {
				t.Fatal(err)
			}
			if event == nil || event.BlockNumber != test.eventBlock {
				t.Fatalf("expected the event at block %d, got %+v", test.eventBlock, event)
			}
			if fmt.Sprint(source.queries) != fmt.Sprint(test.expectedQueries) {
				t.Fatalf(
					"unexpected queried ranges\nexpected: %v\nactual:   %v",
					test.expectedQueries,
					source.queries,
				)
			}
		})
	}

	t.Run("not found covers the whole range once", func(t *testing.T) {
		source := &rangeDepositRevealedEvents{}
		event, err := FindDepositRevealedEvent(
			source,
			wallet,
			25000,
			50000,
			matchFundingTxHash(target),
		)
		if err != nil || event != nil {
			t.Fatalf("expected no event and no error, got %+v, %v", event, err)
		}
		expected := [][2]uint64{{40001, 50000}, {30001, 40000}, {25000, 30000}}
		if fmt.Sprint(source.queries) != fmt.Sprint(expected) {
			t.Fatalf("expected queries %v, got %v", expected, source.queries)
		}
	})

	t.Run("range starting at block zero does not underflow", func(t *testing.T) {
		source := &rangeDepositRevealedEvents{}
		if _, err := FindDepositRevealedEvent(
			source,
			wallet,
			0,
			12000,
			matchFundingTxHash(target),
		); err != nil {
			t.Fatal(err)
		}
		expected := [][2]uint64{{2001, 12000}, {0, 2000}}
		if fmt.Sprint(source.queries) != fmt.Sprint(expected) {
			t.Fatalf("expected queries %v, got %v", expected, source.queries)
		}
	})
}

// TestFindDepositRevealedEventByRevealTime pins the reveal-time lookup a
// signer uses for an anchor proposal: a reveal far older than any fixed
// look-back window is found from its on-chain timestamp, block-time drift
// inside the margin is tolerated, and a reveal outside the first window
// but inside the widened one is found by the single retry.
func TestFindDepositRevealedEventByRevealTime(t *testing.T) {
	wallet := [20]byte{0x01}
	target := bitcoin.Hash{0xaa}
	now := time.Now()
	currentBlock := uint64(10_000_000)
	// 300000 blocks (about 41 days at 12 s) before the current block,
	// older than the 216000-block window the lookup replaced. The
	// margin for this distance is 5% of 300000 = 15000 blocks.
	revealedAt := now.Add(-300000 * 12 * time.Second)
	estimatedBlock := currentBlock - 300000

	tests := map[string]struct {
		eventBlock uint64
		expectHit  bool
	}{
		"exact estimate":                  {eventBlock: estimatedBlock, expectHit: true},
		"drift inside the first window":   {eventBlock: estimatedBlock + 14000, expectHit: true},
		"later block found by widening":   {eventBlock: estimatedBlock + 40000, expectHit: true},
		"earlier block found by widening": {eventBlock: estimatedBlock - 40000, expectHit: true},
		"outside the widened window":      {eventBlock: estimatedBlock - 70000, expectHit: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			source := &rangeDepositRevealedEvents{
				events: []*DepositRevealedEvent{{
					BlockNumber:         test.eventBlock,
					WalletPublicKeyHash: wallet,
					FundingTxHash:       target,
				}},
			}

			event, err := FindDepositRevealedEventByRevealTime(
				source,
				wallet,
				revealedAt,
				now,
				currentBlock,
				0,
				matchFundingTxHash(target),
			)
			if err != nil {
				t.Fatal(err)
			}
			if test.expectHit && (event == nil || event.BlockNumber != test.eventBlock) {
				t.Fatalf("expected the event at block %d, got %+v", test.eventBlock, event)
			}
			if !test.expectHit && event != nil {
				t.Fatalf("expected no event, got %+v", event)
			}
			// No block may be queried twice: the retry scans only the
			// two outer strips of the widened window.
			for i, a := range source.queries {
				for _, b := range source.queries[i+1:] {
					if a[0] <= b[1] && b[0] <= a[1] {
						t.Fatalf(
							"ranges %v and %v overlap: %v",
							a,
							b,
							source.queries,
						)
					}
				}
			}
		})
	}
}
