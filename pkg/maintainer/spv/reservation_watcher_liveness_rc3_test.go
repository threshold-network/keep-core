package spv

// This file covers the block-range shape of the wallet-registration lookup
// behind the stranding recheck path: the lookup reaches back to the wallet's
// registration block, so it must walk each look-back window in bounded
// chunks rather than issuing one eth_getLogs per window.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// cappedEventRangeChain rejects any PastNewWalletRegisteredEvents query whose
// range is unbounded or wider than maxSpan, the way a provider with an
// eth_getLogs range cap does, and counts the queries it accepted.
type cappedEventRangeChain struct {
	*localChain
	maxSpan uint64
	queries int
}

func (c *cappedEventRangeChain) PastNewWalletRegisteredEvents(
	filter *tbtc.NewWalletRegisteredEventFilter,
) ([]*tbtc.NewWalletRegisteredEvent, error) {
	if filter == nil {
		return nil, errors.New("unfiltered registration event query")
	}
	if filter.EndBlock == nil {
		return nil, fmt.Errorf(
			"eth_getLogs rejected: unbounded block range from [%d]",
			filter.StartBlock,
		)
	}
	if span := *filter.EndBlock - filter.StartBlock + 1; span > c.maxSpan {
		return nil, fmt.Errorf(
			"eth_getLogs rejected: block range [%d..%d] spans %d blocks, "+
				"the provider caps it at %d",
			filter.StartBlock,
			*filter.EndBlock,
			span,
			c.maxSpan,
		)
	}
	c.queries++
	return c.localChain.PastNewWalletRegisteredEvents(filter)
}

// TestResolveWalletPublicKeyHashChunksRegistrationScan pins that resolving a
// closed wallet's public key hash walks its doubled look-back window in
// bounded chunks. The lookup has to reach back to the wallet's registration
// block, which grows the window by doubling; issuing that window as one
// eth_getLogs is rejected outright by a provider that caps the range, and the
// rejection costs the wallet its stranding check.
func TestResolveWalletPublicKeyHashChunksRegistrationScan(t *testing.T) {
	wallet := walletPKHAt(0x64)
	walletID := [32]byte{0x64}

	base := newLocalChain()
	blockCounter := newMockBlockCounter()
	// The registration is far outside the first look-back window, so the
	// lookup has to double it - and each window still has to be chunked.
	blockCounter.SetCurrentBlock(2_000_000)
	base.setBlockCounter(blockCounter)

	base.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       walletID,
		WalletPublicKeyHash: wallet,
		BlockNumber:         1000,
	})

	chain := &cappedEventRangeChain{
		localChain: base,
		maxSpan:    reservationEventScanSpan(),
	}

	resolved, err := resolveWalletPublicKeyHash(chain, walletID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != wallet {
		t.Fatalf("expected wallet [0x%x], got [0x%x]", wallet, resolved)
	}
	if chain.queries == 0 {
		t.Fatal("expected at least one registration query")
	}
}

// TestResolveWalletPublicKeyHashHonoursConfiguredSpan pins that the
// registration lookup behind the stranding path uses the operator-configured
// eth_getLogs span, not a span of its own. An operator behind a provider with
// a cap tighter than the default lowers
// tbtc.ReservationEventScanMaxBlockSpan; a lookup that kept issuing
// default-sized chunks was rejected by that provider, so the wallet ID never
// resolved and the wallet stayed queued with no stranding coverage.
func TestResolveWalletPublicKeyHashHonoursConfiguredSpan(t *testing.T) {
	const configuredSpan = uint64(250)

	previousSpan := tbtc.ReservationEventScanMaxBlockSpan
	tbtc.ReservationEventScanMaxBlockSpan = configuredSpan
	defer func() { tbtc.ReservationEventScanMaxBlockSpan = previousSpan }()

	wallet := walletPKHAt(0x65)
	walletID := [32]byte{0x65}

	base := newLocalChain()
	blockCounter := newMockBlockCounter()
	blockCounter.SetCurrentBlock(2_000_000)
	base.setBlockCounter(blockCounter)

	base.addNewWalletRegisteredEvent(&tbtc.NewWalletRegisteredEvent{
		EcdsaWalletID:       walletID,
		WalletPublicKeyHash: wallet,
		BlockNumber:         1000,
	})

	// The provider enforces exactly the configured cap, so any chunk wider
	// than the override is rejected the way a real provider would.
	chain := &cappedEventRangeChain{
		localChain: base,
		maxSpan:    configuredSpan,
	}

	resolved, err := resolveWalletPublicKeyHash(chain, walletID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != wallet {
		t.Fatalf("expected wallet [0x%x], got [0x%x]", wallet, resolved)
	}
	if chain.queries == 0 {
		t.Fatal("expected at least one registration query")
	}
}
