package tbtc

import (
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-common/pkg/cache"
)

const (
	testDKGSeedCachePeriod       = 1 * time.Second
	testDKGResultHashCachePeriod = 1 * time.Second
)

func TestNotifyDKGStarted(t *testing.T) {
	deduplicator := deduplicator{
		dkgSeedCache: cache.NewTimeCache(testDKGSeedCachePeriod),
	}

	seed1 := big.NewInt(100)
	seed2 := big.NewInt(200)

	// Add the first seed.
	canJoinDKG := deduplicator.notifyDKGStarted(seed1)
	if !canJoinDKG {
		t.Fatal("should be allowed to join DKG")
	}

	// Add the second seed.
	canJoinDKG = deduplicator.notifyDKGStarted(seed2)
	if !canJoinDKG {
		t.Fatal("should be allowed to join DKG")
	}

	// Add the first seed before caching period elapses.
	canJoinDKG = deduplicator.notifyDKGStarted(seed1)
	if canJoinDKG {
		t.Fatal("should not be allowed to join DKG")
	}

	// Wait until caching period elapses.
	time.Sleep(testDKGSeedCachePeriod)

	// Add the first seed again.
	canJoinDKG = deduplicator.notifyDKGStarted(seed1)
	if !canJoinDKG {
		t.Fatal("should be allowed to join DKG")
	}
}

func TestDKGStartedDeduplicationLease(t *testing.T) {
	deduplicator := newDeduplicator()
	seed := big.NewInt(100)

	lease, ok := deduplicator.beginDKGStarted(seed)
	if !ok {
		t.Fatal("expected first handler to acquire the event")
	}

	if _, ok := deduplicator.beginDKGStarted(seed); ok {
		t.Fatal("expected in-progress duplicate to be suppressed")
	}

	lease.finish(false)
	lease, ok = deduplicator.beginDKGStarted(seed)
	if !ok {
		t.Fatal("expected released event to be retryable")
	}

	lease.finish(true)
	if _, ok := deduplicator.beginDKGStarted(seed); ok {
		t.Fatal("expected completed event to be suppressed")
	}
}

func TestNotifyDKGResultSubmitted(t *testing.T) {
	deduplicator := deduplicator{
		dkgResultHashCache: cache.NewTimeCache(testDKGResultHashCachePeriod),
	}

	hash1Bytes, err := hex.DecodeString("92327ddff69a2b8c7ae787c5d590a2f14586089e6339e942d56e82aa42052cd9")
	if err != nil {
		t.Fatal(err)
	}
	var hash1 [32]byte
	copy(hash1[:], hash1Bytes)

	hash2Bytes, err := hex.DecodeString("23c0062913c4614bdff07f94475ceb4c585df53f71611776c3521ed8f8785913")
	if err != nil {
		t.Fatal(err)
	}
	var hash2 [32]byte
	copy(hash2[:], hash2Bytes)

	// Add the original parameters.
	canProcess := deduplicator.notifyDKGResultSubmitted(big.NewInt(100), hash1, 500)
	if !canProcess {
		t.Fatal("should be allowed to process")
	}

	// Add with different seed.
	canProcess = deduplicator.notifyDKGResultSubmitted(big.NewInt(101), hash1, 500)
	if !canProcess {
		t.Fatal("should be allowed to process")
	}

	// Add with different result hash.
	canProcess = deduplicator.notifyDKGResultSubmitted(big.NewInt(100), hash2, 500)
	if !canProcess {
		t.Fatal("should be allowed to process")
	}

	// Add with different result block.
	canProcess = deduplicator.notifyDKGResultSubmitted(big.NewInt(100), hash1, 501)
	if !canProcess {
		t.Fatal("should be allowed to process")
	}

	// Add with all different parameters.
	canProcess = deduplicator.notifyDKGResultSubmitted(big.NewInt(101), hash2, 501)
	if !canProcess {
		t.Fatal("should be allowed to process")
	}

	// Add the original parameters before caching period elapses.
	canProcess = deduplicator.notifyDKGResultSubmitted(big.NewInt(100), hash1, 500)
	if canProcess {
		t.Fatal("should not be allowed to process")
	}

	// Wait until caching period elapses.
	time.Sleep(testDKGResultHashCachePeriod)

	// Add the original parameters again.
	canProcess = deduplicator.notifyDKGResultSubmitted(big.NewInt(100), hash1, 500)
	if !canProcess {
		t.Fatal("should be allowed to process")
	}
}
