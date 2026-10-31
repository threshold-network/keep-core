package spv

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
	ethereum "github.com/keep-network/keep-core/pkg/chain/ethereumutil"
)

// TestProveReservationAcceptanceActions_ReorgWithinConfirmationLag
// verifies that an acceptance event which falls within the
// confirmation lag (12 blocks behind the tip) and is therefore not
// yet scanned, will be re-seen by the next pass if a shallow reorg
// replaces the block containing it. This demonstrates the
// "no lost event" property for reorgs within the lag window.
// Negative control: if the lag constant were smaller, the event would
// have been scanned on the first pass and a subsequent reorg would
// lose it (the test would fail).
func TestProveReservationAcceptanceActions_ReorgWithinConfirmationLag(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newReservationProofBitcoinChain()
	blockCounter := newMockBlockCounter()
	// First pass: current block 1000, confirmed tip = 988 (lag = 12).
	// An event at block 995 is ABOVE the confirmed tip (988), so it
	// is NOT scanned on the first pass.
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(12345)
	const requestNonce = 1

	// Event at block 995: inside the confirmation lag (988 < 995 < 1000)
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        requestNonce,
		WalletPublicKeyHash: [20]byte{1},
		DepositAmount:       150000,
		BlockNumber:         995,
	})
	// Action is Pending.
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: [20]byte{1},
			TimeoutAt:                 2000,
			TermSeconds:               100,
		},
	)

	scanState := newReservationProofScanState()
	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}

	// First pass: the event at 995 is NOT scanned (endBlock = 988, startBlock = 1, range 1..988).
	// The scan covers blocks 1..988, so the event at 995 is in the unconfirmed zone.
	key := reservationEventKey(reservationKey, requestNonce)
	cache := newProofInfoCache()
	if err := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
		cache,
		nil,
	); err != nil {
		t.Fatalf("unexpected error on first pass: %v", err)
	}

	// Event should NOT be tracked yet (it was in the unconfirmed zone).
	if _, exists := scanState.pendingAcceptanceEvents[key]; exists {
		t.Fatal("expected event at block 995 to be above the confirmed tip and not scanned on first pass")
	}

	// Now simulate a shallow reorg: the chain head moves to 1010, but the
	// block at 995 is replaced (the event disappears from that block).
	// The new canonical chain has a new block at 995 (or 1000+).
	// Advance head so that the previously-unconfirmed block 995 becomes
	// part of the confirmed tip.
	blockCounter.SetCurrentBlock(1010)
	spvChain.setBlockCounter(blockCounter)

	// Remove the event from the old block (simulating reorg) and add it
	// at the new canonical block 1000 (which is now within the confirmed
	// tip: 1010 - 12 = 998).
	spvChain.removeReservationAcceptanceRequestedEvent(reservationKey, requestNonce, 995)
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        requestNonce,
		WalletPublicKeyHash: [20]byte{1},
		DepositAmount:       150000,
		BlockNumber:         1000,
	})

	// Second pass: confirmed tip is now 1010 - 12 = 998. The event at
	// block 1000 is still ABOVE the confirmed tip (1000 > 998), so it
	// is still not scanned. We need to advance further.
	blockCounter.SetCurrentBlock(1020)
	spvChain.setBlockCounter(blockCounter)

	// Third pass: confirmed tip = 1020 - 12 = 1008. Now the event at
	// 1000 IS within the scan range (start = 1, end = 1008).
	cache = newProofInfoCache()
	if err := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
		cache,
		nil,
	); err != nil {
		t.Fatalf("unexpected error on third pass: %v", err)
	}

	// The event should now be tracked exactly once (no double-processing).
	if _, exists := scanState.pendingAcceptanceEvents[key]; !exists {
		t.Fatal("expected event to be tracked after the reorg")
	}

	// Cursor should have advanced to the confirmed tip.
	if scanState.acceptanceLastScannedBlock != 1008 {
		t.Errorf("expected cursor at 1008, got %d", scanState.acceptanceLastScannedBlock)
	}
}

// TestProveReservationAcceptanceActions_DeepReorg_CursorDoesNotRollBack
// is a characterization test documenting the current behavior for a
// deep reorg (deeper than the 12-block confirmation lag). The cursor
// only moves forward; it does NOT roll back. An event that was
// already scanned and then disappears in a deep reorg is NOT
// re-fetched, and the proof loop continues tracking it based on its
// on-chain action state (which will eventually be evicted when the
// action state becomes Unknown or Settled). This is the documented
// residual risk (TM-20: a reorg deeper than the window is the
// documented assumption). The test pins this behavior explicitly so
// any future cursor-rollback logic would fail this test.
func TestProveReservationAcceptanceActions_DeepReorg_CursorDoesNotRollBack(t *testing.T) {
	spvChain := newLocalChain()
	btcChain := newReservationProofBitcoinChain()

	blockCounter := newMockBlockCounter()
	// First pass: current block 1000, confirmed tip = 988.
	blockCounter.SetCurrentBlock(1000)
	spvChain.setBlockCounter(blockCounter)

	reservationKey := big.NewInt(67890)
	const requestNonce = 1

	// Event at block 500 — well within the confirmed range.
	spvChain.addReservationAcceptanceRequestedEvent(&tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      reservationKey,
		RequestNonce:        requestNonce,
		WalletPublicKeyHash: [20]byte{1},
		DepositAmount:       150000,
		BlockNumber:         500,
	})
	spvChain.setReservationAction(
		reservationKey,
		requestNonce,
		&tbtc.ReservationAction{
			State:                     tbtc.ReservationActionStatePending,
			ActionType:                tbtc.ReservationActionTypeAcceptance,
			TargetWalletPublicKeyHash: [20]byte{1},
			TimeoutAt:                 2000,
			TermSeconds:               100,
		},
	)

	scanState := newReservationProofScanState()
	config := Config{
		TransactionLimit: 100,
		MaxProofHeaders:  DefaultMaxProofHeaders,
		EthereumNetwork:  ethereum.Developer,
	}
	key := reservationEventKey(reservationKey, requestNonce)

	// First pass: event at block 500 IS scanned (range 1..988).
	cache := newProofInfoCache()
	if err := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
		cache,
		nil,
	); err != nil {
		t.Fatalf("unexpected error on first pass: %v", err)
	}

	if _, exists := scanState.pendingAcceptanceEvents[key]; !exists {
		t.Fatal("expected event at block 500 to be tracked on first pass")
	}
	if scanState.acceptanceLastScannedBlock != 988 {
		t.Fatalf("expected cursor at 988, got %d", scanState.acceptanceLastScannedBlock)
	}

	// Now simulate a DEEP reorg: the chain head drops below the cursor.
	// Current block was 1000 (cursor at 988). Reorg removes blocks down to 950.
	// The block at 500 is REMOVED from the canonical chain (the event's
	// block is now orphaned). The new head is 950.
	blockCounter.SetCurrentBlock(950)
	spvChain.setBlockCounter(blockCounter)

	// Remove the event from the chain (it was in the orphaned block).
	spvChain.removeReservationAcceptanceRequestedEvent(reservationKey, requestNonce, 500)

	// Second pass: confirmed tip = 950 - 12 = 938. startBlock = 989, endBlock = 938.
	// This is an INVERTED range, so fetchPastEventsInChunks is a no-op.
	// The cursor does NOT roll back (stays at 988).
	cache = newProofInfoCache()
	err := proveReservationAcceptanceActions(
		scanState,
		config,
		spvChain,
		spvChain,
		btcChain,
		cache,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}

	// The cursor should NOT have rolled back (stays at 988).
	if scanState.acceptanceLastScannedBlock != 988 {
		t.Errorf("expected cursor to stay at 988 (no rollback), got %d", scanState.acceptanceLastScannedBlock)
	}

	// The event is still tracked in memory (the proof loop keeps
	// tracked events across passes regardless of whether the block
	// was reorged out). The on-chain action state will eventually
	// be re-read and the event evicted when the action state is
	// no longer settleable. The key point: the cursor does not roll
	// back, and no double-processing occurs.
	if _, exists := scanState.pendingAcceptanceEvents[key]; !exists {
		t.Fatal("expected event to remain tracked in memory even after deep reorg")
	}

	// The scan range for this pass was inverted (989..938), so no
	// new events were fetched. This is the documented behavior:
	// deep reorgs beyond the lag leave the cursor ahead of the
	// canonical chain and the scan is a no-op.
}

// TestReservationScanRange_DeepReorg_InvertedRangeIsNoOp verifies
// that when the chain head drops below the cursor, the scan range
// is inverted (startBlock > endBlock) and fetchPastEventsInChunks
// performs no work (a no-op). The cursor does NOT roll back; it
// simply stays ahead of the canonical chain. This is the mechanism
// that prevents cursor rollback.
func TestReservationScanRange_DeepReorg_InvertedRangeIsNoOp(t *testing.T) {
	// Cursor at 988, new head at 950 (deep reorg).
	// endBlock = 950 - 12 = 938.
	// startBlock = 988 + 1 = 989.
	// 989 > 938 → inverted range.
	start, end, scan := reservationScanRange(988, 0, 950)

	// reservationScanRange returns scan=true even for an inverted
	// range; the no-op happens in fetchPastEventsInChunks which
	// checks startBlock > endBlock.
	if !scan {
		t.Fatalf("expected scan = true for inverted range (no-op in fetch)")
	}
	if start != 989 {
		t.Errorf("expected start = 989, got %d", start)
	}
	if end != 938 {
		t.Errorf("expected end = 938, got %d", end)
	}
	if start <= end {
		t.Errorf("expected inverted range (start > end), got start=%d end=%d", start, end)
	}

	// Now verify fetchPastEventsInChunks is a no-op for the
	// inverted range: it must not call the fetch function at all.
	fetchCalled := false
	err := fetchPastEventsInChunks(
		func(startBlock, endBlock uint64) ([]uint64, error) {
			fetchCalled = true
			return nil, nil
		},
		start,
		end,
		func(events []uint64, chunkEnd uint64) {
			t.Errorf("onChunk called unexpectedly with chunkEnd=%d", chunkEnd)
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetchCalled {
		t.Fatal("expected fetch to not be called for inverted range (no-op)")
	}

	// Negative control: if the cursor were rolled back to the
	// activation block (e.g. startBlock = activationBlock), the
	// range would be valid and fetchPastEventsInChunks would call
	// the fetch function. This test asserts the current no-rollback
	// behavior.
}