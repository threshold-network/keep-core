package tbtc

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"time"

	"github.com/keep-network/keep-core/pkg/cache"
)

const (
	// DKGSeedCachePeriod is the time period the cache maintains
	// the DKG seed corresponding to a DKG instance.
	DKGSeedCachePeriod = 7 * 24 * time.Hour
	// DKGResultHashCachePeriod is the time period the cache maintains
	// the given DKG result hash.
	DKGResultHashCachePeriod = 7 * 24 * time.Hour
	// WalletClosedCachePeriod is the time period the cache maintains the ID of
	// a closed wallet.
	WalletClosedCachePeriod = 7 * 24 * time.Hour
)

// deduplicator decides whether the given event should be handled by the
// client or not.
//
// Event subscription may emit the same event two or more times. The same event
// can be emitted right after it's been emitted for the first time. The same
// event can also be emitted a long time after it's been emitted for the first
// time. It is the deduplicator's responsibility to decide whether the given
// event is a duplicate and should be ignored or if it is not a duplicate and
// should be handled.
//
// Those events are supported:
// - DKG started
// - DKG result submitted
// - Wallet closed
type deduplicator struct {
	dkgSeedCache       *cache.TimeCache
	dkgResultHashCache *cache.TimeCache
	walletClosedCache  *cache.TimeCache
}

func newDeduplicator() *deduplicator {
	return &deduplicator{
		dkgSeedCache:       cache.NewTimeCache(DKGSeedCachePeriod),
		dkgResultHashCache: cache.NewTimeCache(DKGResultHashCachePeriod),
		walletClosedCache:  cache.NewTimeCache(WalletClosedCachePeriod),
	}
}

// notifyDKGStarted notifies the client wants to start the distributed key
// generation upon receiving an event. It returns boolean indicating whether the
// client should proceed with the execution or ignore the event as a duplicate.
func (d *deduplicator) notifyDKGStarted(
	newDKGSeed *big.Int,
) bool {
	d.dkgSeedCache.Sweep()

	// The cache key is the hexadecimal representation of the seed.
	cacheKey := newDKGSeed.Text(16)
	// If the key is not in the cache, that means the seed was not handled
	// yet and the client should proceed with the execution.
	if !d.dkgSeedCache.Has(cacheKey) {
		d.dkgSeedCache.Add(cacheKey)
		return true
	}

	// Otherwise, the DKG seed is a duplicate and the client should not proceed
	// with the execution.
	return false
}

// notifyDKGResultSubmitted notifies the client wants to start some actions
// upon the DKG result submission. It returns boolean indicating whether the
// client should proceed with the actions or ignore the event as a duplicate.
func (d *deduplicator) notifyDKGResultSubmitted(
	newDKGResultSeed *big.Int,
	newDKGResultHash DKGChainResultHash,
	newDKGResultBlock uint64,
) bool {
	d.dkgResultHashCache.Sweep()

	cacheKey := newDKGResultSeed.Text(16) +
		hex.EncodeToString(newDKGResultHash[:]) +
		strconv.Itoa(int(newDKGResultBlock))

	// If the key is not in the cache, that means the result was not handled
	// yet and the client should proceed with the execution.
	if !d.dkgResultHashCache.Has(cacheKey) {
		d.dkgResultHashCache.Add(cacheKey)
		return true
	}

	// Otherwise, the DKG result is a duplicate and the client should not
	// proceed with the execution.
	return false
}

func (d *deduplicator) notifyWalletClosed(
	WalletID [32]byte,
) bool {
	d.walletClosedCache.Sweep()

	// Use wallet ID converted to string as the cache key.
	cacheKey := hex.EncodeToString(WalletID[:])

	// If the key is not in the cache, that means the wallet closure was not
	// handled yet and the client should proceed with the execution.
	if !d.walletClosedCache.Has(cacheKey) {
		d.walletClosedCache.Add(cacheKey)
		return true
	}

	// Otherwise, the wallet closure is a duplicate and the client should not
	// proceed with the execution.
	return false
}
