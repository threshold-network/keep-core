package event

import (
	"encoding/hex"
	"github.com/keep-network/keep-core/pkg/cache"
	"math/big"
	"testing"
	"time"
)

const testDKGSeedCachePeriod = 1 * time.Second

func TestNotifyDKGStarted(t *testing.T) {
	chain := &testChain{
		currentRequestStartBlockValue:    nil,
		currentRequestPreviousEntryValue: []byte{},
	}

	deduplicator := &Deduplicator{
		chain:        chain,
		dkgSeedCache: cache.NewTimeCache(testDKGSeedCachePeriod),
	}

	seed1 := big.NewInt(100)
	seed2 := big.NewInt(200)

	// Add the first seed.
	canJoinDKG := deduplicator.NotifyDKGStarted(seed1)
	if !canJoinDKG {
		t.Fatal("should be allowed to join DKG")
	}

	// Add the second seed.
	canJoinDKG = deduplicator.NotifyDKGStarted(seed2)
	if !canJoinDKG {
		t.Fatal("should be allowed to join DKG")
	}

	// Add the first seed before caching period elapses.
	canJoinDKG = deduplicator.NotifyDKGStarted(seed1)
	if canJoinDKG {
		t.Fatal("should not be allowed to join DKG")
	}

	// Wait until caching period elapses.
	time.Sleep(testDKGSeedCachePeriod)

	// Add the first seed again.
	canJoinDKG = deduplicator.NotifyDKGStarted(seed1)
	if !canJoinDKG {
		t.Fatal("should be allowed to join DKG")
	}
}

func TestStartRelayEntry_NoPriorRelayEntries(t *testing.T) {
	chain := &testChain{
		currentRequestStartBlockValue:    nil,
		currentRequestPreviousEntryValue: []byte{},
	}

	deduplicator := NewDeduplicator(
		chain,
	)

	canGenerate, err := deduplicator.NotifyRelayEntryStarted(
		5,
		"entry",
	)
	if err != nil {
		t.Fatal(err)
	}

	if !canGenerate {
		t.Fatal("should be allowed to start relay entry")
	}
}

func TestStartRelayEntry_LowerStartBlock(t *testing.T) {
	chain := &testChain{
		currentRequestStartBlockValue:    nil,
		currentRequestPreviousEntryValue: []byte{},
	}

	deduplicator := NewDeduplicator(
		chain,
	)

	_, err := deduplicator.NotifyRelayEntryStarted(100, "01")
	if err != nil {
		t.Fatal(err)
	}

	canGenerate, err := deduplicator.NotifyRelayEntryStarted(
		5,
		"02",
	)
	if err != nil {
		t.Fatal(err)
	}

	if canGenerate {
		t.Fatal("should not be allowed to start relay entry")
	}
}

func TestStartRelayEntry_HigherStartBlock_DifferentPreviousEntry(t *testing.T) {
	chain := &testChain{
		currentRequestStartBlockValue:    nil,
		currentRequestPreviousEntryValue: []byte{},
	}

	deduplicator := NewDeduplicator(
		chain,
	)

	_, err := deduplicator.NotifyRelayEntryStarted(100, "01")
	if err != nil {
		t.Fatal(err)
	}

	canGenerate, err := deduplicator.NotifyRelayEntryStarted(
		101,
		"02",
	)
	if err != nil {
		t.Fatal(err)
	}

	if !canGenerate {
		t.Fatal("should be allowed to start relay entry")
	}
}

func TestStartRelayEntry_HigherStartBlock_SamePreviousEntry_ConfirmedOnChain(t *testing.T) {
	bytes, err := hex.DecodeString("01")
	if err != nil {
		t.Fatal(err)
	}

	chain := &testChain{
		currentRequestStartBlockValue:    big.NewInt(101),
		currentRequestPreviousEntryValue: bytes,
	}

	deduplicator := NewDeduplicator(
		chain,
	)

	_, err = deduplicator.NotifyRelayEntryStarted(100, "01")
	if err != nil {
		t.Fatal(err)
	}

	canGenerate, err := deduplicator.NotifyRelayEntryStarted(
		101,
		"01",
	)
	if err != nil {
		t.Fatal(err)
	}

	if !canGenerate {
		t.Fatal("should be allowed to start relay entry")
	}
}

func TestStartRelayEntry_HigherStartBlock_SamePreviousEntry_PreviousEntryNotConfirmedOnChain(t *testing.T) {
	bytes, err := hex.DecodeString("02")
	if err != nil {
		t.Fatal(err)
	}

	chain := &testChain{
		currentRequestStartBlockValue:    big.NewInt(101),
		currentRequestPreviousEntryValue: bytes,
	}

	deduplicator := NewDeduplicator(
		chain,
	)

	_, err = deduplicator.NotifyRelayEntryStarted(100, "01")
	if err != nil {
		t.Fatal(err)
	}

	canGenerate, err := deduplicator.NotifyRelayEntryStarted(
		101,
		"01",
	)
	if err != nil {
		t.Fatal(err)
	}

	if canGenerate {
		t.Fatal("should not be allowed to start relay entry")
	}
}

func TestStartRelayEntry_HigherStartBlock_SamePreviousEntry_StartBlockNotConfirmedOnChain(t *testing.T) {
	bytes, err := hex.DecodeString("01")
	if err != nil {
		t.Fatal(err)
	}

	chain := &testChain{
		currentRequestStartBlockValue:    big.NewInt(100),
		currentRequestPreviousEntryValue: bytes,
	}

	deduplicator := NewDeduplicator(
		chain,
	)

	_, err = deduplicator.NotifyRelayEntryStarted(100, "01")
	if err != nil {
		t.Fatal(err)
	}

	canGenerate, err := deduplicator.NotifyRelayEntryStarted(
		101,
		"01",
	)
	if err != nil {
		t.Fatal(err)
	}

	if canGenerate {
		t.Fatal("should not be allowed to start relay entry")
	}
}

type testChain struct {
	currentRequestStartBlockValue    *big.Int
	currentRequestPreviousEntryValue []byte
}

func (tc *testChain) CurrentRequestStartBlock() (*big.Int, error) {
	return tc.currentRequestStartBlockValue, nil
}

func (tc *testChain) CurrentRequestPreviousEntry() ([]byte, error) {
	return tc.currentRequestPreviousEntryValue, nil
}
