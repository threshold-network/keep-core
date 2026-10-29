package spv

import (
	"errors"
	"math/big"
	"testing"

	ethereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reanchorChunkFailingChain wraps localChain and fails the re-anchor
// event fetch for any chunk starting at failChunkStart, so a test can drive
// a re-anchor scan that breaks part-way through the activation-to-tip walk.
type reanchorChunkFailingChain struct {
	*localChain

	failChunkStart *uint64
}

func (c *reanchorChunkFailingChain) PastReservationReanchorRequestedEvents(
	filter *tbtc.ReservationReanchorRequestedEventFilter,
) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
	if c.failChunkStart != nil && filter.StartBlock == *c.failChunkStart {
		return nil, errors.New("simulated provider failure")
	}
	return c.localChain.PastReservationReanchorRequestedEvents(filter)
}

// TestProveReservationReanchorActions_ChunkedScanKeepsEarlierProgress pins
// that the re-anchor event scan saves its progress per chunk: every chunk of
// the activation-to-tip walk is scanned, and a chunk that fails leaves the
// cursor at the end of the last good chunk, so the next pass resumes after
// it instead of restarting the whole walk or losing the events of the
// chunks that already succeeded.
//
// The acceptance scan's equivalent rule is covered by
// TestProveReservationAcceptanceActions_ChunkedScan, but the re-anchor scan
// keeps its own cursor (state.reanchorLastScannedBlock) and its own chunk
// callback, and no test drove the re-anchor side of the walk. A mutation
// that advances the re-anchor cursor to the end of the whole range as soon
// as the first chunk arrives would therefore have gone unnoticed: the
// failed chunk's events would be skipped for good.
func TestProveReservationReanchorActions_ChunkedScanKeepsEarlierProgress(t *testing.T) {
	const currentBlock = 2*reservationEventScanChunkSize + 500
	confirmedTip := uint64(currentBlock) - reservationEventScanConfirmationBlocks

	newFixture := func() (
		*localChain,
		*reanchorChunkFailingChain,
		*reservationProofScanState,
		Config,
	) {
		inner := newLocalChain()
		blockCounter := newMockBlockCounter()
		blockCounter.SetCurrentBlock(currentBlock)
		inner.setBlockCounter(blockCounter)
		return inner,
			&reanchorChunkFailingChain{localChain: inner},
			newReservationProofScanState(),
			Config{
				TransactionLimit: 100,
				MaxProofHeaders:  DefaultMaxProofHeaders,
				EthereumNetwork:  ethereum.Developer,
			}
	}

	// addEvent tracks a Pending re-anchor generation requested at
	// blockNumber. Its source wallet has no transactions, so nothing is
	// proved and the generation simply stays tracked.
	addEvent := func(inner *localChain, key int64, blockNumber uint64) string {
		reservationKey := big.NewInt(key)
		inner.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
			ReservationKey:            reservationKey,
			RequestNonce:              1,
			SourceWalletPublicKeyHash: [20]byte{byte(key)},
			TargetWalletPublicKeyHash: [20]byte{byte(key) + 1},
			TxMaxFee:                  1000,
			BlockNumber:               blockNumber,
		})
		inner.setReservationAction(reservationKey, 1, &tbtc.ReservationAction{
			State:      tbtc.ReservationActionStatePending,
			ActionType: tbtc.ReservationActionTypeReanchor,
		})
		return reservationEventKey(reservationKey, 1)
	}

	prove := func(
		chain *reanchorChunkFailingChain,
		state *reservationProofScanState,
		config Config,
	) error {
		return proveReservationReanchorActions(
			state,
			config,
			chain,
			chain,
			newReservationProofBitcoinChain(),
			newProofInfoCache(),
			nil,
		)
	}

	t.Run("every chunk is scanned", func(t *testing.T) {
		inner, chain, state, config := newFixture()
		firstChunkKey := addEvent(inner, 1, 100)
		secondChunkKey := addEvent(inner, 2, reservationEventScanChunkSize+100)

		if err := prove(chain, state, config); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, key := range []string{firstChunkKey, secondChunkKey} {
			if _, ok := state.pendingReanchorEvents[key]; !ok {
				t.Errorf("expected event [%s] to be tracked", key)
			}
		}
		if state.reanchorLastScannedBlock != confirmedTip {
			t.Errorf("expected the cursor at %d, got %d", confirmedTip, state.reanchorLastScannedBlock)
		}
	})

	t.Run("a failing chunk keeps the earlier chunks' progress", func(t *testing.T) {
		inner, chain, state, config := newFixture()
		firstChunkKey := addEvent(inner, 1, 100)
		secondChunkKey := addEvent(inner, 2, reservationEventScanChunkSize+100)

		failStart := reservationEventScanChunkSize
		chain.failChunkStart = &failStart
		if err := prove(chain, state, config); err == nil {
			t.Fatal("expected the failing chunk to be reported")
		}

		if _, ok := state.pendingReanchorEvents[firstChunkKey]; !ok {
			t.Error("expected the first chunk's event to be tracked despite the later failure")
		}
		if _, ok := state.pendingReanchorEvents[secondChunkKey]; ok {
			t.Error("expected the failed chunk's event not to be tracked yet")
		}

		// The cursor must sit at the end of the last good chunk, so the
		// next pass resumes after it. A cursor that jumped to the end of
		// the whole range would lose the failed chunk's events for good.
		if state.reanchorLastScannedBlock != reservationEventScanChunkSize-1 {
			t.Errorf(
				"expected the cursor at the end of the first chunk %d, got %d",
				reservationEventScanChunkSize-1,
				state.reanchorLastScannedBlock,
			)
		}

		chain.failChunkStart = nil
		if err := prove(chain, state, config); err != nil {
			t.Fatalf("unexpected error on the resumed pass: %v", err)
		}
		if _, ok := state.pendingReanchorEvents[secondChunkKey]; !ok {
			t.Error("expected the resumed pass to find the second chunk's event")
		}
		if state.reanchorLastScannedBlock != confirmedTip {
			t.Errorf("expected the cursor at %d, got %d", confirmedTip, state.reanchorLastScannedBlock)
		}
	})
}

// TestProveReservationReanchorActions_ScanErrorKeepsProvingTrackedGenerations
// pins the second half of the rule for the re-anchor scan: a scan error does
// not skip the generations that are already tracked. The scan error is
// reported after the pass has done its work, so the tracked generations are
// still visited and the error surfaces as a scan error.
//
// The acceptance scan's equivalent rule is covered by
// TestProveReservationAcceptanceActions_ChunkedScan and the per-item
// aggregation tests; the re-anchor scan had no equivalent.
func TestProveReservationReanchorActions_ScanErrorKeepsProvingTrackedGenerations(t *testing.T) {
	const currentBlock = 2*reservationEventScanChunkSize + 500
	confirmedTip := uint64(currentBlock) - reservationEventScanConfirmationBlocks

	inner := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(currentBlock)
	inner.setBlockCounter(blockCounter)

	chain := &reanchorChunkFailingChain{localChain: inner}
	state := newReservationProofScanState()
	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}

	// A generation tracked by an earlier pass, before the scan started to
	// fail. It must still be visited by the failing pass.
	reservationKey := big.NewInt(7)
	inner.addReservationReanchorRequestedEvent(&tbtc.ReservationReanchorRequestedEvent{
		ReservationKey:            reservationKey,
		RequestNonce:              1,
		SourceWalletPublicKeyHash: [20]byte{7},
		TargetWalletPublicKeyHash: [20]byte{8},
		TxMaxFee:                  1000,
		BlockNumber:               100,
	})
	inner.setReservationAction(reservationKey, 1, &tbtc.ReservationAction{
		State:      tbtc.ReservationActionStatePending,
		ActionType: tbtc.ReservationActionTypeReanchor,
	})

	// The first pass scans the whole range successfully and tracks the
	// generation.
	if err := proveReservationReanchorActions(
		state,
		config,
		chain,
		chain,
		newReservationProofBitcoinChain(),
		newProofInfoCache(),
		nil,
	); err != nil {
		t.Fatalf("unexpected error on the first pass: %v", err)
	}

	key := reservationEventKey(reservationKey, 1)
	if _, ok := state.pendingReanchorEvents[key]; !ok {
		t.Fatal("expected the first pass to track the generation")
	}

	// The next pass's scan fails on its first chunk. The tracked generation
	// must still be visited: the scan error is reported after the pass has
	// done its work, not before.
	//
	// The tip advances first, so the second pass has a fresh range to scan
	// and the failing chunk is the first chunk of that range.
	advancedCounter := newMockBlockCounter()
	advancedCounter.SetCurrentBlock(currentBlock + 2*reservationEventScanChunkSize)
	inner.setBlockCounter(advancedCounter)

	failStart := confirmedTip + 1
	chain.failChunkStart = &failStart

	err := proveReservationReanchorActions(
		state,
		config,
		chain,
		chain,
		newReservationProofBitcoinChain(),
		newProofInfoCache(),
		nil,
	)
	if err == nil {
		t.Fatal("expected the failing scan to be reported")
	}
	var chunkErr *chunkScanError
	if !errors.As(err, &chunkErr) {
		t.Fatalf("expected a chunk scan error, got %T: %v", err, err)
	}

	// The tracked generation must survive the scan error: it is still
	// settleable and the next pass has to retry it.
	if _, ok := state.pendingReanchorEvents[key]; !ok {
		t.Error("expected the tracked generation to survive the scan error")
	}
}
