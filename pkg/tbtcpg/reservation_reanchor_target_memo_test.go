package tbtcpg

import (
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/tbtc"
)

// amountReadCountingChain counts the wallet reservation amount reads the
// target search issues, so a test can tell a pass that rescans every
// candidate from one that reuses what it already learned.
type amountReadCountingChain struct {
	*LocalChain

	amountReads int
}

func (c *amountReadCountingChain) WalletReservationsAmount(
	walletPublicKeyHash [20]byte,
) (uint64, error) {
	c.amountReads++
	return c.LocalChain.WalletReservationsAmount(walletPublicKeyHash)
}

// seedReservedAmount gives walletPublicKeyHash the given number of
// anchor reservations whose values add up to total, so the wallet's
// reservation count and reserved amount are both set.
func seedReservedAmount(
	t *testing.T,
	lc *LocalChain,
	walletPublicKeyHash [20]byte,
	firstKey uint64,
	count int,
	total uint64,
) {
	t.Helper()

	perReservation := total / uint64(count)
	keys := make([]*big.Int, 0, count)
	for i := range count {
		value := perReservation
		if i == count-1 {
			value = total - perReservation*uint64(count-1)
		}
		key := new(big.Int).SetUint64(firstKey + uint64(i))
		lc.SetReservation(key, &tbtc.Reservation{
			WalletPublicKeyHash: walletPublicKeyHash,
			AnchorUtxo: &bitcoin.UnspentTransactionOutput{
				Outpoint: &bitcoin.TransactionOutpoint{},
				Value:    int64(value),
			},
			State: tbtc.ReservationStateActive,
		})
		keys = append(keys, key)
	}
	lc.SetWalletReservations(walletPublicKeyHash, keys)
}

// TestReanchorTargetSearch_NoHeadroomMemoAppliesToItsOwnAnchor pins the
// anchor boundary of the pass-level no-headroom memo. When no candidate has
// amount headroom for an anchor value, the search records that value and
// short-circuits every later request at or above it, because headroom only
// shrinks as the anchor grows. Two things follow, and both are pinned here:
//
//   - a later, smaller anchor must still be searched -- a pass that lets one
//     heavy reservation starve a lighter one loses an entire re-anchor it
//     could have made; and
//   - a repeat of the recorded anchor value must short-circuit itself, not
//     rescan every candidate, because the answer cannot have improved.
func TestReanchorTargetSearch_NoHeadroomMemoAppliesToItsOwnAnchor(t *testing.T) {
	sourceWalletPublicKeyHash := [20]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	targetOne := [20]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	targetTwo := [20]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}

	const amountCap = uint64(300000)

	lc := &amountReadCountingChain{LocalChain: NewLocalChain()}
	lc.SetWallet(sourceWalletPublicKeyHash, &tbtc.WalletChainData{State: tbtc.StateMovingFunds})
	lc.SetWallet(targetOne, &tbtc.WalletChainData{State: tbtc.StateLive})
	lc.SetWallet(targetTwo, &tbtc.WalletChainData{State: tbtc.StateLive})
	lc.SetLiveWalletsCount(2)
	blockCounter := NewMockBlockCounter()
	blockCounter.SetCurrentBlock(1000)
	lc.SetBlockCounter(blockCounter)

	// Both targets hold the same reserved amount, so neither has room for
	// an anchor above the remaining 50000 of the amount cap, and both have
	// count room.
	seedReservedAmount(t, lc.LocalChain, targetOne, 8000, 3, 250000)
	seedReservedAmount(t, lc.LocalChain, targetTwo, 8100, 2, 250000)

	task := NewReservationReanchorTask(lc, NewLocalBitcoinChain())
	search := task.newReanchorTargetSearch(
		logger,
		sourceWalletPublicKeyHash,
		5,
		amountCap,
		time.Unix(1000, 0),
		0,
		// An ordinary third-party request: no privileged fallback, so a
		// no-headroom answer stays a no-headroom answer.
		false,
	)
	search.candidates = [][20]byte{targetOne, targetTwo}
	search.createdAt[targetOne] = time.Unix(0, 0)
	search.createdAt[targetTwo] = time.Unix(0, 0)
	// The candidate set is supplied directly, so the search's own
	// registration scans are already done for this pass.
	search.cachedChecked = true
	search.recentScanned = true
	search.fullScanned = true

	// 150000 does not fit under either target's amount headroom.
	if _, err := search.find(150000); err != errNoLiveTargetWallet {
		t.Fatalf("expected no target for a 150000 anchor, got [%v]", err)
	}
	readsAfterFirstFind := lc.amountReads
	if readsAfterFirstFind != 2 {
		t.Fatalf(
			"expected both candidates to be examined once, got %d amount "+
				"reads",
			readsAfterFirstFind,
		)
	}

	// A lighter anchor still fits: a memo at 150000 must not answer for it.
	picked, err := search.find(50000)
	if err != nil {
		t.Fatalf("expected a target for a 50000 anchor, got [%v]", err)
	}
	if picked != targetOne {
		t.Fatalf(
			"expected the first candidate [%x] to be picked for a small "+
				"anchor, got [%x]",
			targetOne,
			picked,
		)
	}

	readsAfterSmallFind := lc.amountReads

	// No candidate had headroom for 150000, so a repeat of that anchor must
	// be answered from the memo rather than by rescanning both candidates.
	if _, err := search.find(150000); err != errNoLiveTargetWallet {
		t.Fatalf("expected no target for a repeated 150000 anchor, got [%v]", err)
	}
	if lc.amountReads != readsAfterSmallFind {
		t.Fatalf(
			"expected the repeated anchor to be answered without a rescan, "+
				"got %d extra amount reads",
			lc.amountReads-readsAfterSmallFind,
		)
	}
}

// TestReservationReanchorTask_EvictCachedTargetKeepsOtherWallets pins that
// eviction is scoped to the named wallet. A target that lost its headroom
// or reverted on capacity must be dropped from the cache, but a different
// cached target is still the best-known destination and must survive the
// eviction; dropping it sends the next pass back to a full registration
// scan and can pick a different, worse target.
func TestReservationReanchorTask_EvictCachedTargetKeepsOtherWallets(t *testing.T) {
	sourceWalletPublicKeyHash := [20]byte{4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}
	cached := [20]byte{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5}
	reverted := [20]byte{6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6}

	task := NewReservationReanchorTask(NewLocalChain(), NewLocalBitcoinChain())

	task.setCachedTargetWallet(cached)
	task.evictCachedTargetWallet(reverted)

	got, ok := task.cachedTargetWallet(sourceWalletPublicKeyHash)
	if !ok || got != cached {
		t.Fatalf(
			"expected the cached target [%x] to survive the eviction of a "+
				"different wallet [%x], got [%x] cached=%v",
			cached,
			reverted,
			got,
			ok,
		)
	}

	// Evicting the cached wallet itself still clears it.
	task.evictCachedTargetWallet(cached)
	if _, ok := task.cachedTargetWallet(sourceWalletPublicKeyHash); ok {
		t.Fatal("expected the cached target to be cleared by its own eviction")
	}
}