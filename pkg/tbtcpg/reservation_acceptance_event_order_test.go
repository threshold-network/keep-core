package tbtcpg

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// seededAcceptanceEventsChain returns the acceptance request events a test
// seeds for any block range, standing in for a node that serves the whole
// scan window.
type seededAcceptanceEventsChain struct {
	*LocalChain

	events  []*tbtc.ReservationAcceptanceRequestedEvent
	filters []tbtc.ReservationAcceptanceRequestedEventFilter
}

func (c *seededAcceptanceEventsChain) PastReservationAcceptanceRequestedEvents(
	filter *tbtc.ReservationAcceptanceRequestedEventFilter,
) ([]*tbtc.ReservationAcceptanceRequestedEvent, error) {
	if filter != nil {
		c.filters = append(c.filters, *filter)
	}
	return c.events, nil
}

// acceptanceRequestEvents runs the request-event scan over the seeded
// events once and returns the collected result.
func acceptanceRequestEvents(
	t *testing.T,
	walletPublicKeyHash [20]byte,
	seeded []*tbtc.ReservationAcceptanceRequestedEvent,
) []*tbtc.ReservationAcceptanceRequestedEvent {
	t.Helper()

	lc := NewLocalChain()
	chain := &seededAcceptanceEventsChain{LocalChain: lc, events: seeded}
	blockCounter := NewMockBlockCounter()
	blockCounter.SetCurrentBlock(300000)
	lc.SetBlockCounter(blockCounter)

	task := NewReservationAcceptanceTask(chain, NewLocalBitcoinChain())
	events, err := task.reservationAcceptanceRequestedEvents(
		walletPublicKeyHash,
		300000,
		86400,
		12*time.Second,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return events
}

// withNonce returns an acceptance request event for the given reservation
// key and nonce, differing in timeout and block so a test can tell which
// copy of the generation survived the scan.
func withNonce(
	walletPublicKeyHash [20]byte,
	reservationKey int64,
	nonce uint64,
	timeoutAt uint32,
	blockNumber uint64,
) *tbtc.ReservationAcceptanceRequestedEvent {
	return &tbtc.ReservationAcceptanceRequestedEvent{
		ReservationKey:      big.NewInt(reservationKey),
		RequestNonce:        nonce,
		WalletPublicKeyHash: walletPublicKeyHash,
		TimeoutAt:           timeoutAt,
		BlockNumber:         blockNumber,
	}
}

// TestReservationAcceptanceRequestedEvents_KeepsHighestNonce pins the
// per-reservation dedupe: a generation can be requested more than once
// inside one scan window, and only the highest nonce is the current
// generation. Keeping a lower nonce hands the pass a stale generation that
// the chain would refuse.
func TestReservationAcceptanceRequestedEvents_KeepsHighestNonce(t *testing.T) {
	walletPublicKeyHash := [20]byte{0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31, 0x31}

	events := acceptanceRequestEvents(t, walletPublicKeyHash, []*tbtc.ReservationAcceptanceRequestedEvent{
		// The newer generation is seen first, the stale one second, so a
		// comparison that accepts the second on equality keeps the stale
		// generation.
		withNonce(walletPublicKeyHash, 41, 5, 2000, 120),
		withNonce(walletPublicKeyHash, 41, 3, 1000, 100),
	})

	if len(events) != 1 {
		t.Fatalf("expected 1 generation per reservation key, got %d", len(events))
	}
	if events[0].RequestNonce != 5 {
		t.Fatalf(
			"expected the highest nonce [5] to survive the dedupe, got [%d]",
			events[0].RequestNonce,
		)
	}
	if events[0].TimeoutAt != 2000 {
		t.Fatalf(
			"expected the surviving generation's timeout [2000], got [%d]",
			events[0].TimeoutAt,
		)
	}
}

// TestReservationAcceptanceRequestedEvents_NonceTieKeepsFirstSeen pins the
// dedupe on a nonce tie: two events carrying the same nonce are the same
// generation reported twice, and the first one seen is the one the scan
// keeps. Replacing it on equality makes the surviving copy depend on log
// order, which is not stable across providers.
func TestReservationAcceptanceRequestedEvents_NonceTieKeepsFirstSeen(t *testing.T) {
	walletPublicKeyHash := [20]byte{0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32}

	events := acceptanceRequestEvents(t, walletPublicKeyHash, []*tbtc.ReservationAcceptanceRequestedEvent{
		withNonce(walletPublicKeyHash, 42, 7, 2000, 120),
		withNonce(walletPublicKeyHash, 42, 7, 1000, 100),
	})

	if len(events) != 1 {
		t.Fatalf("expected 1 generation per reservation key, got %d", len(events))
	}
	if events[0].TimeoutAt != 2000 || events[0].BlockNumber != 120 {
		t.Fatalf(
			"expected the first-seen generation (timeout 2000, block 120) "+
				"to survive a nonce tie, got timeout [%d], block [%d]",
			events[0].TimeoutAt,
			events[0].BlockNumber,
		)
	}
}

// TestReservationAcceptanceRequestedEvents_OrderIsDeterministic pins the
// scan's total order: oldest timeout first, then lowest block, then lowest
// reservation key. The event set is collected from a map, so the order the
// sort receives is arbitrary; the order it produces must not depend on it,
// or the pass proposes a different deposit from one run to the next.
//
// The last two comparisons are only reached when the earlier keys tie, and
// they are strictly weaker than their enclosing != guards, so the boundary
// between < and <= on either of them cannot be observed: a pair of events
// that reaches the block comparison has different timeouts, and a pair that
// reaches the key comparison has equal timeouts and equal blocks. Only the
// guards and the direction of each comparison are pinned here.
func TestReservationAcceptanceRequestedEvents_OrderIsDeterministic(t *testing.T) {
	walletPublicKeyHash := [20]byte{0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33, 0x33}

	const runs = 100

	t.Run("timeout tie orders by block", func(t *testing.T) {
		// Reservation keys descend as blocks ascend, so a scan that orders
		// by key instead of by block is visible as a reversed result.
		seeded := []*tbtc.ReservationAcceptanceRequestedEvent{
			withNonce(walletPublicKeyHash, 3, 1, 5000, 10),
			withNonce(walletPublicKeyHash, 2, 1, 5000, 20),
			withNonce(walletPublicKeyHash, 1, 1, 5000, 30),
		}

		for range runs {
			events := acceptanceRequestEvents(t, walletPublicKeyHash, seeded)
			if len(events) != len(seeded) {
				t.Fatalf("expected %d generations, got %d", len(seeded), len(events))
			}
			for i, wantBlock := range []uint64{10, 20, 30} {
				if events[i].BlockNumber != wantBlock {
					t.Fatalf(
						"expected the scan to be ordered by block [%d %d %d], "+
							"got blocks [%d %d %d]",
						10, 20, 30,
						events[0].BlockNumber,
						events[1].BlockNumber,
						events[2].BlockNumber,
					)
				}
			}
		}
	})

	t.Run("block tie orders by reservation key", func(t *testing.T) {
		seeded := []*tbtc.ReservationAcceptanceRequestedEvent{
			withNonce(walletPublicKeyHash, 9, 1, 5000, 10),
			withNonce(walletPublicKeyHash, 4, 1, 5000, 10),
		}

		for range runs {
			events := acceptanceRequestEvents(t, walletPublicKeyHash, seeded)
			if len(events) != len(seeded) {
				t.Fatalf("expected %d generations, got %d", len(seeded), len(events))
			}
			if events[0].ReservationKey.Int64() != 4 ||
				events[1].ReservationKey.Int64() != 9 {
				t.Fatalf(
					"expected the scan to be ordered by reservation key (4 "+
						"then 9) when timeout and block tie, got (%d then %d)",
					events[0].ReservationKey.Int64(),
					events[1].ReservationKey.Int64(),
				)
			}
		}
	})
}