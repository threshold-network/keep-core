package tbtcpg_test

// This file covers the block-range shape of the wallet reservation history
// scan behind the below-dust notification: the scan reaches back to block 0
// and must walk the range in bounded chunks rather than issuing one
// eth_getLogs over the wallet's whole lifetime, which providers that cap the
// query range reject.

import (
	"fmt"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
	"github.com/keep-network/keep-core/pkg/tbtcpg"
)

// cappedHistoryRangeChain rejects reservation history queries whose block
// range is unbounded or wider than maxSpan, the way a provider that caps the
// eth_getLogs range does, and counts the queries it accepted.
type cappedHistoryRangeChain struct {
	*historySourceChain

	maxSpan uint64
	queries int
}

func (c *cappedHistoryRangeChain) checkRange(
	startBlock uint64,
	endBlock *uint64,
) error {
	if endBlock == nil {
		return fmt.Errorf(
			"eth_getLogs rejected: unbounded block range from [%d]",
			startBlock,
		)
	}
	if span := *endBlock - startBlock + 1; span > c.maxSpan {
		return fmt.Errorf(
			"eth_getLogs rejected: blocks [%d..%d] span %d, the provider "+
				"caps it at %d",
			startBlock,
			*endBlock,
			span,
			c.maxSpan,
		)
	}

	c.queries++

	return nil
}

func (c *cappedHistoryRangeChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	if filter == nil {
		return nil, fmt.Errorf("unfiltered acceptance request query")
	}
	if err := c.checkRange(filter.StartBlock, filter.EndBlock); err != nil {
		return nil, err
	}

	return c.historySourceChain.PastReservationAcceptanceRequestedEvents(filter)
}

func (c *cappedHistoryRangeChain) PastReservationReanchorRequestedEvents(
	filter *tbtc.ReservationReanchorRequestedEventFilter,
) ([]*tbtc.ReservationReanchorRequestedEvent, error) {
	if filter == nil {
		return nil, fmt.Errorf("unfiltered re-anchor request query")
	}
	if err := c.checkRange(filter.StartBlock, filter.EndBlock); err != nil {
		return nil, err
	}

	return c.historySourceChain.PastReservationReanchorRequestedEvents(filter)
}

// TestReservationReanchorTask_HistoryScanChunksBlockRange pins that the
// reservation history scan that gates the below-dust notification walks the
// wallet's whole history in bounded chunks. The scan is unbounded by design
// (a wallet's reservation history can predate any look-back window), so the
// only thing that keeps it inside a provider's eth_getLogs cap is the
// chunking; an unbounded query is rejected outright, the history read fails,
// and the wallet is never notified - the wallet stays in MovingFunds for
// good.
func TestReservationReanchorTask_HistoryScanChunksBlockRange(t *testing.T) {
	const blockTip = 250_000

	walletPublicKeyHash := hexToByte20("51b6c3de2b4e0c6b3f0d4e59c8a2f7f2e0b45c15")

	base, btcChain := newHistorySourceFixture(t, walletPublicKeyHash)
	base.markReservationTouched(walletPublicKeyHash)
	blockCounter := tbtcpg.NewMockBlockCounter()
	blockCounter.SetCurrentBlock(blockTip)
	base.SetBlockCounter(blockCounter)

	originalSpan := tbtc.ReservationEventScanMaxBlockSpan
	tbtc.ReservationEventScanMaxBlockSpan = 10_000
	t.Cleanup(func() { tbtc.ReservationEventScanMaxBlockSpan = originalSpan })

	chain := &cappedHistoryRangeChain{
		historySourceChain: base,
		maxSpan:            tbtc.ReservationEventScanMaxBlockSpan,
	}

	task := tbtcpg.NewReservationReanchorTask(chain, btcChain)
	if _, _, err := task.Run(&tbtc.CoordinationProposalRequest{
		WalletPublicKeyHash: walletPublicKeyHash,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if notifications := chain.GetBelowDustNotifications(); len(notifications) != 1 {
		t.Fatalf(
			"expected 1 below-dust notification for a wallet whose history "+
				"scan stays inside the provider cap, got %d",
			len(notifications),
		)
	}
	if chain.queries == 0 {
		t.Fatal("expected the history scan to issue at least one query")
	}
}
