package ethereum

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

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
