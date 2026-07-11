package tbtc

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/keep-network/keep-common/pkg/cache"
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

	mutex          sync.Mutex
	inProgress     map[string]struct{}
	pendingReplays map[string]struct{}
}

func newDeduplicator() *deduplicator {
	return &deduplicator{
		dkgSeedCache:       cache.NewTimeCache(DKGSeedCachePeriod),
		dkgResultHashCache: cache.NewTimeCache(DKGResultHashCachePeriod),
		walletClosedCache:  cache.NewTimeCache(WalletClosedCachePeriod),
		inProgress:         make(map[string]struct{}),
		pendingReplays:     make(map[string]struct{}),
	}
}

// deduplicationLease suppresses concurrent handling of an event without
// marking it permanently handled up front. Completing the lease moves the key
// to the long-lived cache; releasing it after an error lets a subscription or
// recovery replay retry the work.
type deduplicationLease struct {
	owner         *deduplicator
	cache         *cache.TimeCache
	cacheKey      string
	inProgressKey string
}

func (d *deduplicator) beginDKGStarted(
	seed *big.Int,
) (*deduplicationLease, bool) {
	cacheKey := seed.Text(16)
	return d.begin(
		d.dkgSeedCache,
		cacheKey,
		"dkg-started:"+cacheKey,
		false,
	)
}

func (d *deduplicator) beginDKGResultSubmitted(
	seed *big.Int,
	resultHash DKGChainResultHash,
	block uint64,
) (*deduplicationLease, bool) {
	cacheKey := dkgResultSubmittedCacheKey(seed, resultHash, block)
	return d.begin(
		d.dkgResultHashCache,
		cacheKey,
		"dkg-result-submitted:"+cacheKey,
		false,
	)
}

func (d *deduplicator) beginWalletClosed(
	walletScheme WalletScheme,
	walletID [32]byte,
) (*deduplicationLease, bool) {
	cacheKey := walletClosedCacheKey(walletScheme, walletID)
	return d.begin(
		d.walletClosedCache,
		cacheKey,
		"wallet-closed:"+cacheKey,
		true,
	)
}

func (d *deduplicator) begin(
	completedCache *cache.TimeCache,
	cacheKey string,
	inProgressKey string,
	preserveConcurrentReplay bool,
) (*deduplicationLease, bool) {
	completedCache.Sweep()

	d.mutex.Lock()
	defer d.mutex.Unlock()

	if completedCache.Has(cacheKey) {
		return nil, false
	}
	if _, ok := d.inProgress[inProgressKey]; ok {
		if preserveConcurrentReplay {
			if d.pendingReplays == nil {
				d.pendingReplays = make(map[string]struct{})
			}
			d.pendingReplays[inProgressKey] = struct{}{}
		}
		return nil, false
	}
	if d.inProgress == nil {
		d.inProgress = make(map[string]struct{})
	}

	d.inProgress[inProgressKey] = struct{}{}
	return &deduplicationLease{
		owner:         d,
		cache:         completedCache,
		cacheKey:      cacheKey,
		inProgressKey: inProgressKey,
	}, true
}

// finish releases the lease. When handling failed and a concurrent replay was
// recorded, it keeps the lease active and asks the owner to retry immediately.
func (dl *deduplicationLease) finish(completed bool) bool {
	dl.owner.mutex.Lock()
	defer dl.owner.mutex.Unlock()

	if _, ok := dl.owner.inProgress[dl.inProgressKey]; !ok {
		return false
	}

	if completed {
		delete(dl.owner.inProgress, dl.inProgressKey)
		delete(dl.owner.pendingReplays, dl.inProgressKey)
		dl.cache.Add(dl.cacheKey)
		return false
	}

	if _, ok := dl.owner.pendingReplays[dl.inProgressKey]; ok {
		delete(dl.owner.pendingReplays, dl.inProgressKey)
		return true
	}

	delete(dl.owner.inProgress, dl.inProgressKey)
	return false
}

func (dl *deduplicationLease) release() {
	dl.owner.mutex.Lock()
	defer dl.owner.mutex.Unlock()

	delete(dl.owner.inProgress, dl.inProgressKey)
	delete(dl.owner.pendingReplays, dl.inProgressKey)
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

	// Add performs the presence check and insertion atomically.
	return d.dkgSeedCache.Add(cacheKey)
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

	cacheKey := dkgResultSubmittedCacheKey(
		newDKGResultSeed,
		newDKGResultHash,
		newDKGResultBlock,
	)

	// Add performs the presence check and insertion atomically.
	return d.dkgResultHashCache.Add(cacheKey)
}

func dkgResultSubmittedCacheKey(
	seed *big.Int,
	resultHash DKGChainResultHash,
	block uint64,
) string {
	return seed.Text(16) +
		hex.EncodeToString(resultHash[:]) +
		strconv.FormatUint(block, 10)
}

func walletClosedCacheKey(
	walletScheme WalletScheme,
	walletID [32]byte,
) string {
	return strconv.Itoa(int(normalizeWalletScheme(walletScheme))) + ":" +
		hex.EncodeToString(walletID[:])
}
