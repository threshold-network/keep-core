// Package cache provides a time cache implementation safe for concurrent use
// without the need of additional locking.
package cache

import (
	"container/list"
	"sync"
	"time"
)

type genericCacheEntry[T any] struct {
	value     T
	timestamp time.Time
}

// GenericTimeCache provides a generic time cache safe for concurrent use by
// multiple goroutines without additional locking or coordination.
// The implementation is based on the simple TimeCache.
type GenericTimeCache[T any] struct {
	// all keys in the cache in the order they were added
	// most recent keys are on the front of the indexer;
	// it is used to optimize cache sweeping
	indexer *list.List
	// key in the cache with the value and timestamp it's been added
	// to the cache the last time
	cache map[string]*genericCacheEntry[T]
	// the timespan after which entry in the cache is considered
	// as outdated and can be removed from the cache
	timespan time.Duration
	mutex    sync.RWMutex
}

// NewGenericTimeCache creates a new generic cache instance with provided timespan.
func NewGenericTimeCache[T any](timespan time.Duration) *GenericTimeCache[T] {
	return &GenericTimeCache[T]{
		indexer:  list.New(),
		cache:    make(map[string]*genericCacheEntry[T]),
		timespan: timespan,
	}
}

// Add adds an entry to the cache. Returns `true` if entry was not present in
// the cache and was successfully added into it. Returns `false` if
// entry is already in the cache. This method is synchronized.
func (tc *GenericTimeCache[T]) Add(key string, value T) bool {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	_, ok := tc.cache[key]
	if ok {
		return false
	}

	tc.sweep()

	tc.cache[key] = &genericCacheEntry[T]{
		value:     value,
		timestamp: time.Now(),
	}
	tc.indexer.PushFront(key)
	return true
}

// Get gets an entry from the cache. Boolean flag is `true` if entry is
// present and `false` otherwise.
func (tc *GenericTimeCache[T]) Get(key string) (T, bool) {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()

	entry, ok := tc.cache[key]
	if !ok {
		var zeroValue T
		return zeroValue, ok
	}

	return entry.value, ok
}

// Sweep removes old entries. That is those for which caching timespan has
// passed.
func (tc *GenericTimeCache[T]) Sweep() {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	tc.sweep()
}

func (tc *GenericTimeCache[T]) sweep() {
	for {
		back := tc.indexer.Back()
		if back == nil {
			break
		}

		key := back.Value.(string)
		entry, ok := tc.cache[key]
		if !ok {
			logger.Errorf(
				"inconsistent cache state - expected key [%v] is not present",
				key,
			)
			break
		}

		if time.Since(entry.timestamp) > tc.timespan {
			tc.indexer.Remove(back)
			delete(tc.cache, key)
		} else {
			break
		}
	}
}
