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

	// inProgressMutex guards the inProgress set below.
	inProgressMutex sync.Mutex
	// inProgress tracks event keys that have been claimed for handling but
	// whose handling has not yet completed successfully. Keeping claimed but
	// unfinished events here (rather than immediately in the completed caches
	// above) lets concurrent duplicate deliveries be ignored while still
	// allowing a later redelivery to retry the handling if the current attempt
	// fails. An event key is moved into its completed cache only once its
	// handler finishes successfully.
	inProgress map[string]bool
}

func newDeduplicator() *deduplicator {
	return &deduplicator{
		dkgSeedCache:       cache.NewTimeCache(DKGSeedCachePeriod),
		dkgResultHashCache: cache.NewTimeCache(DKGResultHashCachePeriod),
		walletClosedCache:  cache.NewTimeCache(WalletClosedCachePeriod),
		inProgress:         make(map[string]bool),
	}
}

// claim attempts to reserve the given event, identified by key, for handling.
// It returns true when the caller should proceed with handling the event and
// false when the event has already been handled successfully (it lives in
// completedCache) or is currently being handled by another goroutine (it lives
// in the inProgress set).
//
// A successful claim must be paired with exactly one later call: markCompleted
// once the handler finishes successfully, or release if the handler fails.
// Releasing a failed claim (instead of marking it completed) is what lets a
// later redelivery of the same event retry the handling rather than being
// silently dropped as a duplicate.
func (d *deduplicator) claim(completedCache *cache.TimeCache, key string) bool {
	d.inProgressMutex.Lock()
	defer d.inProgressMutex.Unlock()

	// Drop expired entries so a long-past event can be handled again.
	completedCache.Sweep()

	if completedCache.Has(key) {
		return false
	}

	if d.inProgress[key] {
		return false
	}

	d.inProgress[key] = true
	return true
}

// markCompleted records the claimed event, identified by key, as successfully
// handled and releases the in-progress claim. Subsequent deliveries of the same
// event are treated as duplicates until the completedCache entry expires.
func (d *deduplicator) markCompleted(completedCache *cache.TimeCache, key string) {
	d.inProgressMutex.Lock()
	defer d.inProgressMutex.Unlock()

	completedCache.Add(key)
	delete(d.inProgress, key)
}

// release drops the in-progress claim for the given event, identified by key,
// without recording it as completed. It is called when handling fails so a
// later redelivery of the same event can be retried instead of being dropped
// as a duplicate.
func (d *deduplicator) release(key string) {
	d.inProgressMutex.Lock()
	defer d.inProgressMutex.Unlock()

	delete(d.inProgress, key)
}

// dkgStartedKey builds the deduplication key identifying a DKG started event.
// The key is the hexadecimal representation of the seed.
func dkgStartedKey(newDKGSeed *big.Int) string {
	return newDKGSeed.Text(16)
}

// notifyDKGStarted notifies the client wants to start the distributed key
// generation upon receiving an event. It returns boolean indicating whether the
// client should proceed with the execution or ignore the event as a duplicate.
//
// A successful claim must be released with confirmDKGStarted once the event has
// been terminally handled (the local DKG join was started, or the event turned
// out to be unconfirmed) or with abortDKGStarted if handling did not complete,
// so a later redelivery of the same event can retry.
func (d *deduplicator) notifyDKGStarted(
	newDKGSeed *big.Int,
) bool {
	return d.claim(d.dkgSeedCache, dkgStartedKey(newDKGSeed))
}

// confirmDKGStarted marks the given DKG started event as successfully handled
// so subsequent deliveries of the same event are ignored as duplicates.
func (d *deduplicator) confirmDKGStarted(newDKGSeed *big.Int) {
	d.markCompleted(d.dkgSeedCache, dkgStartedKey(newDKGSeed))
}

// abortDKGStarted releases the in-progress claim for the given DKG started
// event without marking it handled, so a later redelivery of the same event can
// retry the handling.
func (d *deduplicator) abortDKGStarted(newDKGSeed *big.Int) {
	d.release(dkgStartedKey(newDKGSeed))
}

// dkgResultSubmittedKey builds the deduplication key identifying a DKG result
// submission event.
func dkgResultSubmittedKey(
	newDKGResultSeed *big.Int,
	newDKGResultHash DKGChainResultHash,
	newDKGResultBlock uint64,
) string {
	return newDKGResultSeed.Text(16) +
		hex.EncodeToString(newDKGResultHash[:]) +
		strconv.Itoa(int(newDKGResultBlock))
}

// notifyDKGResultSubmitted notifies the client wants to start some actions
// upon the DKG result submission. It returns boolean indicating whether the
// client should proceed with the actions or ignore the event as a duplicate.
//
// A successful claim must be released with confirmDKGResultSubmitted once the
// result has been validated (challenged if invalid, or its approval scheduled
// if valid) or with abortDKGResultSubmitted if validation did not reach a
// terminal state, so a later redelivery of the same event can retry.
func (d *deduplicator) notifyDKGResultSubmitted(
	newDKGResultSeed *big.Int,
	newDKGResultHash DKGChainResultHash,
	newDKGResultBlock uint64,
) bool {
	return d.claim(
		d.dkgResultHashCache,
		dkgResultSubmittedKey(newDKGResultSeed, newDKGResultHash, newDKGResultBlock),
	)
}

// confirmDKGResultSubmitted marks the given DKG result submission as
// successfully handled so subsequent deliveries of the same event are ignored
// as duplicates.
func (d *deduplicator) confirmDKGResultSubmitted(
	newDKGResultSeed *big.Int,
	newDKGResultHash DKGChainResultHash,
	newDKGResultBlock uint64,
) {
	d.markCompleted(
		d.dkgResultHashCache,
		dkgResultSubmittedKey(newDKGResultSeed, newDKGResultHash, newDKGResultBlock),
	)
}

// abortDKGResultSubmitted releases the in-progress claim for the given DKG
// result submission without marking it handled, so a later redelivery of the
// same event can retry the validation.
func (d *deduplicator) abortDKGResultSubmitted(
	newDKGResultSeed *big.Int,
	newDKGResultHash DKGChainResultHash,
	newDKGResultBlock uint64,
) {
	d.release(
		dkgResultSubmittedKey(newDKGResultSeed, newDKGResultHash, newDKGResultBlock),
	)
}

// walletClosedKey builds the deduplication key identifying a wallet closure
// event.
func walletClosedKey(walletID [32]byte) string {
	return hex.EncodeToString(walletID[:])
}

// notifyWalletClosed notifies the client wants to handle a wallet closure upon
// receiving an event. It returns a boolean indicating whether the client should
// proceed with the handling or ignore the event as a duplicate.
//
// A successful claim must be released with confirmWalletClosed once the wallet
// has actually been archived, or with abortWalletClosed if archival did not
// complete, so a later redelivery of the same event can retry.
func (d *deduplicator) notifyWalletClosed(
	walletID [32]byte,
) bool {
	return d.claim(d.walletClosedCache, walletClosedKey(walletID))
}

// confirmWalletClosed marks the given wallet closure as successfully handled so
// subsequent deliveries of the same event are ignored as duplicates.
func (d *deduplicator) confirmWalletClosed(walletID [32]byte) {
	d.markCompleted(d.walletClosedCache, walletClosedKey(walletID))
}

// abortWalletClosed releases the in-progress claim for the given wallet closure
// without marking it handled, so a later redelivery of the same event can retry
// the archival.
func (d *deduplicator) abortWalletClosed(walletID [32]byte) {
	d.release(walletClosedKey(walletID))
}
