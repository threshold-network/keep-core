// Package cache provides a time cache implementation safe for concurrent use
// without the need of additional locking.
package cache

import (
	"container/list"
	"sync"
	"time"

	"github.com/ipfs/go-log"
)

var logger = log.Logger("keep-cache")

// TimeCache provides a time cache safe for concurrent use by
// multiple goroutines without additional locking or coordination.
type TimeCache struct {
	// all items in the cache in the order they were added
	// most recent items are on the front of the indexer;
	// it is used to optimize cache sweeping
	indexer *list.List
	// item in the cache with the timestamp it's been added
	// to the cache the last time
	cache map[string]time.Time
	// the timespan after which entry in the cache is considered
	// as outdated and can be removed from the cache
	timespan time.Duration
	mutex    sync.RWMutex
}

// NewTimeCache creates a new cache instance with provided timespan.
func NewTimeCache(timespan time.Duration) *TimeCache {
	return &TimeCache{
		indexer:  list.New(),
		cache:    make(map[string]time.Time),
		timespan: timespan,
	}
}

// Add adds an entry to the cache. Returns `true` if entry was not present in
// the cache and was successfully added into it. Returns `false` if
// entry is already in the cache. This method is synchronized.
func (tc *TimeCache) Add(item string) bool {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, ok := tc.cache[item]
	if ok {
		return false
	}

	tc.sweep()

	tc.cache[item] = time.Now()
	tc.indexer.PushFront(item)
	return true
}

// Has checks presence of an entry in the cache. Returns `true` if entry is
// present and `false` otherwise.
func (tc *TimeCache) Has(item string) bool {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	_, ok := tc.cache[item]
	return ok
}

// Sweep removes old entries. That is those for which caching timespan has
// passed.
func (tc *TimeCache) Sweep() {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	tc.sweep()
}

func (tc *TimeCache) sweep() {
	for {
		back := tc.indexer.Back()
		if back == nil {
			break
		}

		item := back.Value.(string)
		itemTime, ok := tc.cache[item]
		if !ok {
			logger.Errorf(
				"inconsistent cache state - expected item [%v] is not present",
				item,
			)
			break
		}

		if time.Since(itemTime) > tc.timespan {
			tc.indexer.Remove(back)
			delete(tc.cache, item)
		} else {
			break
		}
	}
}
