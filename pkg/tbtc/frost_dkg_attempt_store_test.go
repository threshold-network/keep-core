package tbtc

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/keep-network/keep-common/pkg/persistence"
)

func TestFrostDKGAttemptBoundarySurvivesRestart(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	registry, err := newWalletRegistry(
		persistenceHandle,
		Connect().CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}
	seed := big.NewInt(100)
	if err := registry.recordFrostDKGAttempt(seed, 101); err != nil {
		t.Fatal(err)
	}
	if len(persistenceHandle.saved) != 1 {
		t.Fatalf(
			"unexpected persisted attempt count: [%d]",
			len(persistenceHandle.saved),
		)
	}

	// Event recovery can deliver the same DkgStarted event repeatedly. The
	// durable boundary is idempotent and does not create ambiguous records.
	if err := registry.recordFrostDKGAttempt(seed, 101); err != nil {
		t.Fatal(err)
	}
	if len(persistenceHandle.saved) != 1 {
		t.Fatalf(
			"idempotent attempt was persisted again: [%d]",
			len(persistenceHandle.saved),
		)
	}

	reopened, err := newWalletRegistry(
		persistenceHandle,
		Connect().CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}
	sessions, latestStartBlock, hasAttempt, err :=
		reopened.frostDKGRetirementMaterialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 || !hasAttempt || latestStartBlock != 101 {
		t.Fatalf(
			"durable attempt boundary was not recovered: sessions=%d latest=%d present=%t",
			len(sessions),
			latestStartBlock,
			hasAttempt,
		)
	}
}

func TestFrostDKGAttemptBoundaryRejectsConflictingReplay(t *testing.T) {
	registry := &walletRegistry{
		walletCache:                make(map[string]*walletCacheValue),
		walletStorage:              newWalletStorage(&mockPersistenceHandle{}),
		frostDKGAttemptStartBlocks: make(map[string]uint64),
	}
	seed := big.NewInt(100)
	if err := registry.recordFrostDKGAttempt(seed, 101); err != nil {
		t.Fatal(err)
	}
	err := registry.recordFrostDKGAttempt(seed, 102)
	if err == nil || !strings.Contains(err.Error(), "conflicting start blocks") {
		t.Fatalf("conflicting attempt replay was accepted: [%v]", err)
	}
}

func TestFrostDKGAttemptBoundarySurvivesRealPersistenceRestart(t *testing.T) {
	storagePath := t.TempDir()
	handle, err := persistence.NewProtectedDiskHandle(storagePath)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := newWalletRegistry(handle, Connect().CalculateWalletID)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.recordFrostDKGAttempt(big.NewInt(200), 201); err != nil {
		t.Fatal(err)
	}

	reopenedHandle, err := persistence.NewProtectedDiskHandle(storagePath)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := newWalletRegistry(
		reopenedHandle,
		Connect().CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, latestStartBlock, hasAttempt, err :=
		reopened.frostDKGRetirementMaterialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !hasAttempt || latestStartBlock != 201 {
		t.Fatalf(
			"real persistence lost the attempt boundary: latest=%d present=%t",
			latestStartBlock,
			hasAttempt,
		)
	}
}

type failingFrostDKGAttemptPersistence struct {
	*mockPersistenceHandle
}

func (persistence *failingFrostDKGAttemptPersistence) Save(
	data []byte,
	directory string,
	name string,
) error {
	if directory == frostDKGAttemptDirectory {
		return errFrostDKGAttemptPersistenceTest
	}
	return persistence.mockPersistenceHandle.Save(data, directory, name)
}

var errFrostDKGAttemptPersistenceTest = errors.New(
	"injected FROST DKG attempt persistence failure",
)

func TestFrostDKGAttemptBoundaryPersistenceFailureIsNotAdmitted(t *testing.T) {
	handle := &failingFrostDKGAttemptPersistence{
		mockPersistenceHandle: &mockPersistenceHandle{},
	}
	registry := &walletRegistry{
		walletCache:                make(map[string]*walletCacheValue),
		walletStorage:              newWalletStorage(handle),
		frostDKGAttemptStartBlocks: make(map[string]uint64),
	}

	err := registry.recordFrostDKGAttempt(big.NewInt(100), 101)
	if err == nil || !strings.Contains(
		err.Error(),
		errFrostDKGAttemptPersistenceTest.Error(),
	) {
		t.Fatalf("attempt persistence failure was ignored: [%v]", err)
	}
	if len(registry.frostDKGAttemptStartBlocks) != 0 ||
		registry.revision != 0 {
		t.Fatal("failed attempt persistence changed registry state")
	}
}
