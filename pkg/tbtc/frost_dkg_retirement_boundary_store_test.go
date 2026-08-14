package tbtc

import (
	"encoding/json"
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

func TestFrostDKGAttemptBoundaryAcceptsCanonicalReinclusion(t *testing.T) {
	persistenceHandle := &mockPersistenceHandle{}
	registry := &walletRegistry{
		walletCache:                  make(map[string]*walletCacheValue),
		walletStorage:                newWalletStorage(persistenceHandle),
		frostDKGRetirementBoundaries: make(map[string]uint64),
	}
	seed := big.NewInt(100)
	if err := registry.recordFrostDKGAttempt(seed, 101); err != nil {
		t.Fatal(err)
	}
	if err := registry.recordFrostDKGAttempt(seed, 102); err != nil {
		t.Fatalf("canonical re-inclusion was rejected: [%v]", err)
	}
	if err := registry.recordFrostDKGAttempt(seed, 102); err != nil {
		t.Fatal(err)
	}
	if len(persistenceHandle.saved) != 2 {
		t.Fatalf(
			"unexpected persisted re-inclusion boundaries: [%d]",
			len(persistenceHandle.saved),
		)
	}
	_, latestStartBlock, hasAttempt, err :=
		registry.frostDKGRetirementMaterialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !hasAttempt || latestStartBlock != 102 {
		t.Fatalf(
			"re-included attempt did not advance the retirement boundary: latest=%d present=%t",
			latestStartBlock,
			hasAttempt,
		)
	}

	reopened, err := newWalletRegistry(
		persistenceHandle,
		Connect().CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, latestStartBlock, hasAttempt, err =
		reopened.frostDKGRetirementMaterialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !hasAttempt || latestStartBlock != 102 {
		t.Fatal("re-included attempt boundaries did not survive restart")
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

func TestFrostDKGAttemptBoundaryLoadsLegacyRecord(t *testing.T) {
	seed, err := canonicalFrostDKGAttemptSeed(big.NewInt(300))
	if err != nil {
		t.Fatal(err)
	}
	record := frostDKGRetirementBoundaryRecord{
		Schema:     frostDKGAttemptLegacySchema,
		Seed:       seed,
		StartBlock: 301,
		Checksum:   frostDKGLegacyAttemptChecksum(seed, 301),
	}
	encoded, err := json.Marshal(&record)
	if err != nil {
		t.Fatal(err)
	}
	persistenceHandle := &mockPersistenceHandle{
		saved: []persistence.DataDescriptor{&mockDescriptor{
			name:      frostDKGLegacyAttemptFile(seed),
			directory: frostDKGAttemptDirectory,
			content:   encoded,
		}},
	}

	registry, err := newWalletRegistry(
		persistenceHandle,
		Connect().CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, latestStartBlock, hasAttempt, err :=
		registry.frostDKGRetirementMaterialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !hasAttempt || latestStartBlock != 301 {
		t.Fatal("legacy attempt boundary was not recovered")
	}

	// A canonical re-inclusion writes a distinct v2 boundary next to the
	// legacy record and advances the high-water mark.
	if err := registry.recordFrostDKGAttempt(big.NewInt(300), 302); err != nil {
		t.Fatal(err)
	}
	reopened, err := newWalletRegistry(
		persistenceHandle,
		Connect().CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, latestStartBlock, hasAttempt, err =
		reopened.frostDKGRetirementMaterialSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !hasAttempt || latestStartBlock != 302 {
		t.Fatal("legacy record prevented canonical re-inclusion")
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
		walletCache:                  make(map[string]*walletCacheValue),
		walletStorage:                newWalletStorage(handle),
		frostDKGRetirementBoundaries: make(map[string]uint64),
	}

	err := registry.recordFrostDKGAttempt(big.NewInt(100), 101)
	if err == nil || !strings.Contains(
		err.Error(),
		errFrostDKGAttemptPersistenceTest.Error(),
	) {
		t.Fatalf("attempt persistence failure was ignored: [%v]", err)
	}
	if len(registry.frostDKGRetirementBoundaries) != 0 ||
		registry.revision != 0 {
		t.Fatal("failed attempt persistence changed registry state")
	}
}
