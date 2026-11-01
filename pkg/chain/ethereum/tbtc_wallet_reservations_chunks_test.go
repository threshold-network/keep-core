package ethereum

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	ethereumutil "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestScanWalletReservationEventsInChunks pins how WalletReservations
// queries the wallet's reservation events: the range is walked in
// tbtc.DepositRevealLookupChunkBlocks-wide chunks, and every event found is
// returned. A single eth_getLogs over the wallet's whole lifetime is
// rejected by providers that cap the query range, which fails the re-anchor
// pass for every wallet on such a provider.
func TestScanWalletReservationEventsInChunks(t *testing.T) {
	walletPKH := [20]byte{0x42}
	chunk := tbtc.DepositRevealLookupChunkBlocks

	acceptance := func(key int64) *tbtc.ReservationAcceptanceRequestedEvent {
		return &tbtc.ReservationAcceptanceRequestedEvent{
			ReservationKey:      big.NewInt(key),
			WalletPublicKeyHash: walletPKH,
		}
	}
	reanchor := func(key int64) *tbtc.ReservationReanchorRequestedEvent {
		return &tbtc.ReservationReanchorRequestedEvent{
			ReservationKey:            big.NewInt(key),
			TargetWalletPublicKeyHash: walletPKH,
		}
	}

	// A fake fetch callback records the ranges it was asked for and
	// returns one event per range, so the merged result also shows how
	// many chunks were walked.
	type rangeCall struct {
		startBlock uint64
		endBlock   uint64
	}

	t.Run("range inside one chunk is a single query", func(t *testing.T) {
		var calls []rangeCall
		acceptanceEvents, reanchorEvents, err := scanWalletReservationEventsInChunks(
			walletPKH,
			500,
			900,
			func(startBlock, endBlock uint64) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
				calls = append(calls, rangeCall{startBlock, endBlock})
				return []*tbtc.ReservationAcceptanceRequestedEvent{acceptance(1)}, nil
			},
			func(startBlock, endBlock uint64) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
				return []*tbtc.ReservationReanchorRequestedEvent{reanchor(2)}, nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expectedCalls := []rangeCall{{500, 900}}
		if len(calls) != 1 || calls[0] != expectedCalls[0] {
			t.Errorf(
				"expected calls %+v, actual %+v",
				expectedCalls,
				calls,
			)
		}
		if len(acceptanceEvents) != 1 || acceptanceEvents[0].ReservationKey.Int64() != 1 {
			t.Errorf("unexpected acceptance events: %+v", acceptanceEvents)
		}
		if len(reanchorEvents) != 1 || reanchorEvents[0].ReservationKey.Int64() != 2 {
			t.Errorf("unexpected reanchor events: %+v", reanchorEvents)
		}
	})

	t.Run("range wider than one chunk is walked chunk by chunk", func(t *testing.T) {
		startBlock := uint64(1000)
		endBlock := startBlock + 2*chunk + 5

		var acceptanceCalls []rangeCall
		var reanchorCalls []rangeCall
		acceptanceEvents, reanchorEvents, err := scanWalletReservationEventsInChunks(
			walletPKH,
			startBlock,
			endBlock,
			func(rangeStartBlock, rangeEndBlock uint64) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
				acceptanceCalls = append(
					acceptanceCalls,
					rangeCall{rangeStartBlock, rangeEndBlock},
				)
				return []*tbtc.ReservationAcceptanceRequestedEvent{
					acceptance(int64(len(acceptanceCalls))),
				}, nil
			},
			func(rangeStartBlock, rangeEndBlock uint64) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
				reanchorCalls = append(
					reanchorCalls,
					rangeCall{rangeStartBlock, rangeEndBlock},
				)
				return []*tbtc.ReservationReanchorRequestedEvent{
					reanchor(int64(len(reanchorCalls))),
				}, nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedCalls := []rangeCall{
			{startBlock, startBlock + chunk - 1},
			{startBlock + chunk, startBlock + 2*chunk - 1},
			{startBlock + 2*chunk, endBlock},
		}
		for name, calls := range map[string][]rangeCall{
			"acceptance": acceptanceCalls,
			"reanchor":   reanchorCalls,
		} {
			if len(calls) != len(expectedCalls) {
				t.Fatalf(
					"%s: expected %d calls, actual %d (%+v)",
					name,
					len(expectedCalls),
					len(calls),
					calls,
				)
			}
			for i := range expectedCalls {
				if calls[i] != expectedCalls[i] {
					t.Errorf(
						"%s: call %d: expected range %+v, actual %+v",
						name,
						i,
						expectedCalls[i],
						calls[i],
					)
				}
			}
		}

		// Every chunk's events are merged, in ascending block order.
		if len(acceptanceEvents) != len(expectedCalls) {
			t.Fatalf(
				"expected %d merged acceptance events, actual %d",
				len(expectedCalls),
				len(acceptanceEvents),
			)
		}
		for i := range expectedCalls {
			if got := acceptanceEvents[i].ReservationKey.Int64(); got != int64(i+1) {
				t.Errorf(
					"acceptance event %d: expected key %d, got %d",
					i,
					i+1,
					got,
				)
			}
		}
		if len(reanchorEvents) != len(expectedCalls) {
			t.Fatalf(
				"expected %d merged reanchor events, actual %d",
				len(expectedCalls),
				len(reanchorEvents),
			)
		}
	})

	t.Run("a chunk exactly one chunk wide starts the next at the following block", func(t *testing.T) {
		startBlock := uint64(10)
		endBlock := startBlock + chunk - 1

		var calls []rangeCall
		_, _, err := scanWalletReservationEventsInChunks(
			walletPKH,
			startBlock,
			endBlock,
			func(rangeStartBlock, rangeEndBlock uint64) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
				calls = append(calls, rangeCall{rangeStartBlock, rangeEndBlock})
				return nil, nil
			},
			func(rangeStartBlock, rangeEndBlock uint64) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
				return nil, nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calls) != 1 || calls[0] != (rangeCall{startBlock, endBlock}) {
			t.Errorf("expected a single full-width query, actual %+v", calls)
		}
	})

	t.Run("an inverted range queries nothing", func(t *testing.T) {
		var calls int
		acceptanceEvents, reanchorEvents, err := scanWalletReservationEventsInChunks(
			walletPKH,
			900,
			500,
			func(uint64, uint64) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
				calls++
				return nil, nil
			},
			func(uint64, uint64) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
				calls++
				return nil, nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 0 {
			t.Errorf("expected no queries for an inverted range, got %d", calls)
		}
		if acceptanceEvents != nil || reanchorEvents != nil {
			t.Errorf(
				"expected no events for an inverted range, got %d and %d",
				len(acceptanceEvents),
				len(reanchorEvents),
			)
		}
	})

	t.Run("a range ending at the uint64 maximum terminates", func(t *testing.T) {
		startBlock := uint64(math.MaxUint64 - 5)

		var calls []rangeCall
		_, _, err := scanWalletReservationEventsInChunks(
			walletPKH,
			startBlock,
			math.MaxUint64,
			func(rangeStartBlock, rangeEndBlock uint64) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
				calls = append(calls, rangeCall{rangeStartBlock, rangeEndBlock})
				return nil, nil
			},
			func(uint64, uint64) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
				return nil, nil
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calls) != 1 || calls[0] != (rangeCall{startBlock, math.MaxUint64}) {
			t.Errorf(
				"expected one query over [%d..%d], actual %+v",
				startBlock,
				uint64(math.MaxUint64),
				calls,
			)
		}
	})

	t.Run("a failing chunk fails the scan and names the range", func(t *testing.T) {
		const startBlock = uint64(100)
		const endBlock = uint64(100 + 2*tbtc.DepositRevealLookupChunkBlocks)

		var acceptanceCalls int
		acceptanceEvents, reanchorEvents, err := scanWalletReservationEventsInChunks(
			walletPKH,
			startBlock,
			endBlock,
			func(rangeStartBlock, rangeEndBlock uint64) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
				acceptanceCalls++
				if acceptanceCalls == 2 {
					return nil, fmt.Errorf("simulated provider range cap")
				}
				return []*tbtc.ReservationAcceptanceRequestedEvent{acceptance(1)}, nil
			},
			func(uint64, uint64) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
				return nil, nil
			},
		)
		if err == nil {
			t.Fatal("expected the failing chunk to fail the scan")
		}
		// The failing chunk's range is reported, so the provider that
		// rejected it is identifiable from the error alone.
		failingRange := fmt.Sprintf(
			"[%d..%d]",
			startBlock+tbtc.DepositRevealLookupChunkBlocks,
			startBlock+2*tbtc.DepositRevealLookupChunkBlocks-1,
		)
		if !strings.Contains(err.Error(), failingRange) {
			t.Errorf(
				"expected the error to name the failing range %s, got [%v]",
				failingRange,
				err,
			)
		}
		if acceptanceEvents != nil || reanchorEvents != nil {
			t.Errorf(
				"expected no partial events to be returned, got %d and %d",
				len(acceptanceEvents),
				len(reanchorEvents),
			)
		}
	})

	t.Run("a failing reanchor chunk fails the scan", func(t *testing.T) {
		_, _, err := scanWalletReservationEventsInChunks(
			walletPKH,
			1,
			2,
			func(uint64, uint64) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
				return nil, nil
			},
			func(uint64, uint64) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
				return nil, fmt.Errorf("simulated transient RPC failure")
			},
		)
		if err == nil {
			t.Fatal("expected the failing reanchor query to fail the scan")
		}
		if !strings.Contains(err.Error(), "reservation reanchor events") {
			t.Errorf(
				"expected the error to name the reanchor query, got [%v]",
				err,
			)
		}
	})
}

// stalledHeadSubscription models a new-head subscription that delivers no
// notifications: a subscription that is still open but silent, which is what
// leaves the block counter's cached height behind the node.
type stalledHeadSubscription struct {
	err chan error
}

func (s *stalledHeadSubscription) Unsubscribe() {}

func (s *stalledHeadSubscription) Err() <-chan error { return s.err }

// stalledHeadChainReader feeds a block counter a startup height and then
// never delivers a new head, so CurrentBlock keeps returning that height
// while ordinary RPC calls see a later one.
type stalledHeadChainReader struct {
	startupHeight uint64
}

func (r *stalledHeadChainReader) HeaderByNumber(
	context.Context,
	*big.Int,
) (*ethereumutil.Header, error) {
	return &ethereumutil.Header{Number: new(big.Int).SetUint64(r.startupHeight)}, nil
}

func (r *stalledHeadChainReader) SubscribeNewHead(
	context.Context,
	chan<- *ethereumutil.Header,
) (ethereumutil.Subscription, error) {
	return &stalledHeadSubscription{err: make(chan error)}, nil
}

// TestWalletReservationScanEndBlock pins where the wallet reservation scan's
// end block comes from: the node itself, read over RPC, never the
// subscription-cached height. The custody count and every other reservation
// read go to the node's current state, so a scan bounded by a lagging cache
// would come back empty - with no error - for a wallet that still custodies
// reservations, and the re-anchor and stranding consumers treat an empty key
// set as a wallet with nothing left to move.
func TestWalletReservationScanEndBlock(t *testing.T) {
	const cachedStartupHeight = uint64(1000)
	const nodeHeight = uint64(1200)

	newChainWithStaleCounter := func(t *testing.T) *TbtcChain {
		t.Helper()

		// The counter is real, built over a reader whose head
		// subscription never delivers, so its cached height stays at the
		// startup value.
		counter, err := ethereumutil.CreateBlockCounter(
			&stalledHeadChainReader{startupHeight: cachedStartupHeight},
		)
		if err != nil {
			t.Fatalf("cannot create the block counter: [%v]", err)
		}

		return &TbtcChain{baseChain: &baseChain{
			client: &latestHeaderClient{
				getHeader: func(
					context.Context,
					*big.Int,
				) (*types.Header, error) {
					return &types.Header{Number: new(big.Int).SetUint64(nodeHeight)}, nil
				},
			},
			blockCounter: counter,
		}}
	}

	t.Run("scan end comes from the node, not the stale cached height", func(t *testing.T) {
		tbtcChain := newChainWithStaleCounter(t)

		// Precondition: the cached height really is behind the node.
		cachedHeight, err := tbtcChain.blockCounter.CurrentBlock()
		if err != nil {
			t.Fatalf("cannot read the cached height: [%v]", err)
		}
		if cachedHeight != cachedStartupHeight {
			t.Fatalf(
				"fixture precondition failed: cached height [%d], want the "+
					"stale startup height [%d]",
				cachedHeight,
				cachedStartupHeight,
			)
		}

		endBlock, err := tbtcChain.walletReservationScanEndBlock()
		if err != nil {
			t.Fatalf("unexpected error: [%v]", err)
		}
		if endBlock != nodeHeight {
			t.Fatalf(
				"expected the scan to end at the node's block [%d], got [%d]",
				nodeHeight,
				endBlock,
			)
		}
	})

	t.Run("node read failure fails the scan instead of using the cache", func(t *testing.T) {
		tbtcChain := newChainWithStaleCounter(t)
		tbtcChain.client = &latestHeaderClient{
			getHeader: func(context.Context, *big.Int) (*types.Header, error) {
				return nil, fmt.Errorf("simulated RPC failure")
			},
		}

		if _, err := tbtcChain.walletReservationScanEndBlock(); err == nil {
			t.Fatal(
				"expected the scan to fail when the node's tip cannot be " +
					"read, not to fall back to the cached height",
			)
		}
	})
}
