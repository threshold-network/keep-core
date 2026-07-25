package tbtc

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type testFrostNativeSignerStateWitnessAnchorSource struct {
	anchor *FrostNativeSignerStateWitnessAnchor
	err    error
}

func (source *testFrostNativeSignerStateWitnessAnchorSource) ReadFrostNativeSignerStateWitnessAnchor(
	context.Context,
) (*FrostNativeSignerStateWitnessAnchor, error) {
	if source.err != nil {
		return nil, source.err
	}
	if source.anchor == nil {
		return nil, nil
	}
	result := *source.anchor
	return &result, nil
}

func TestCompleteFrostInteractiveSigningReadiness_RequiresEveryRuntimeGate(
	t *testing.T,
) {
	tests := []struct {
		name                 string
		interactivePathReady bool
		interactiveOnly      bool
		nativeExecution      bool
		nativeBackend        bool
		expected             bool
	}{
		{"all absent", false, false, false, false, false},
		{"engine and flags absent", false, true, false, true, false},
		{"interactive opt-ins absent", false, true, true, true, false},
		{"interactive-only absent", true, false, true, true, false},
		{"native engine absent", true, true, false, true, false},
		{"native backend absent", true, true, true, false, false},
		{"complete interactive path", true, true, true, true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := completeFrostInteractiveSigningReadiness(
				test.interactivePathReady,
				test.interactiveOnly,
				test.nativeExecution,
				test.nativeBackend,
			)
			if actual != test.expected {
				t.Fatalf("unexpected readiness: got [%v], want [%v]", actual, test.expected)
			}
		})
	}
}

func TestVerifyFrostNativeSignerInventoryEntriesRejectsMismatch(t *testing.T) {
	walletID := [32]byte{0x21}
	valid := []frostsigning.NativeTBTCSignerRetainedKeyGroup{
		{
			WalletID:                   walletID,
			KeyGroup:                   hex.EncodeToString(walletID[:]),
			Threshold:                  51,
			ParticipantCount:           100,
			ShareEpoch:                 0,
			PublicKeyPackageCommitment: [32]byte{0x22},
			KeyPackages: []frostsigning.NativeTBTCSignerRetainedKeyPackage{
				{ParticipantSeat: 4, KeyPackageCommitment: [32]byte{0x23}},
			},
		},
	}
	expected := []frostNativeSignerInventoryExpectation{
		{
			WalletID:         walletID,
			KeyGroup:         hex.EncodeToString(walletID[:]),
			Threshold:        51,
			ParticipantCount: 100,
			ShareEpoch:       0,
			ParticipantSeats: []uint16{4},
		},
	}
	if err := verifyFrostNativeSignerInventoryEntries(valid, expected); err != nil {
		t.Fatalf("valid native inventory was rejected: [%v]", err)
	}

	tests := map[string]func([]frostsigning.NativeTBTCSignerRetainedKeyGroup) []frostsigning.NativeTBTCSignerRetainedKeyGroup{
		"missing group": func([]frostsigning.NativeTBTCSignerRetainedKeyGroup) []frostsigning.NativeTBTCSignerRetainedKeyGroup {
			return nil
		},
		"wrong wallet": func(entries []frostsigning.NativeTBTCSignerRetainedKeyGroup) []frostsigning.NativeTBTCSignerRetainedKeyGroup {
			entries[0].WalletID[0] ^= 0xff
			return entries
		},
		"wrong key group": func(entries []frostsigning.NativeTBTCSignerRetainedKeyGroup) []frostsigning.NativeTBTCSignerRetainedKeyGroup {
			entries[0].KeyGroup = strings.Repeat("09", 32)
			return entries
		},
		"wrong participant seat": func(entries []frostsigning.NativeTBTCSignerRetainedKeyGroup) []frostsigning.NativeTBTCSignerRetainedKeyGroup {
			entries[0].KeyPackages[0].ParticipantSeat = 5
			return entries
		},
		"stale share epoch": func(entries []frostsigning.NativeTBTCSignerRetainedKeyGroup) []frostsigning.NativeTBTCSignerRetainedKeyGroup {
			entries[0].ShareEpoch = 1
			return entries
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			entries := append([]frostsigning.NativeTBTCSignerRetainedKeyGroup{}, valid...)
			entries[0].KeyPackages = append(
				[]frostsigning.NativeTBTCSignerRetainedKeyPackage{},
				valid[0].KeyPackages...,
			)
			if err := verifyFrostNativeSignerInventoryEntries(mutate(entries), expected); err == nil {
				t.Fatal("mismatched native inventory was accepted")
			}
		})
	}
}

func TestFrostNativeSignerInventoryBindingRequiresAnchoredDescendant(t *testing.T) {
	storeBinding := testFrostDurableSessionStoreBinding(t)
	storeFingerprint, err := storeBinding.verify()
	if err != nil {
		t.Fatal(err)
	}
	ancestor := [32]byte{0x41}
	stateImage := [32]byte{0x42}
	target := frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		2,
		ancestor,
		stateImage,
	)
	walletID := [32]byte{0x43}
	entries := []frostsigning.NativeTBTCSignerRetainedKeyGroup{
		{
			WalletID:                   walletID,
			KeyGroup:                   hex.EncodeToString(walletID[:]),
			Threshold:                  51,
			ParticipantCount:           100,
			PublicKeyPackageCommitment: [32]byte{0x44},
			KeyPackages: []frostsigning.NativeTBTCSignerRetainedKeyPackage{
				{ParticipantSeat: 2, KeyPackageCommitment: [32]byte{0x45}},
			},
		},
	}
	inventory := &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
		Schema:                  frostsigning.NativeTBTCSignerRetainedKeyPackageInventorySchema,
		StoreFingerprint:        storeFingerprint,
		StateGeneration:         2,
		StateCommitment:         target,
		PreviousStateCommitment: ancestor,
		StateImageDigest:        stateImage,
		InventoryCommitment: frostsigning.ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment(
			entries,
		),
		Entries: entries,
	}
	anchorSource := &testFrostNativeSignerStateWitnessAnchorSource{
		anchor: &FrostNativeSignerStateWitnessAnchor{
			StoreFingerprint: storeFingerprint,
			Generation:       1,
			Commitment:       ancestor,
		},
	}
	proofReader := func(
		request *frostsigning.NativeTBTCSignerStateWitnessProofRequest,
	) (*frostsigning.NativeTBTCSignerStateWitnessProof, error) {
		proof := &frostsigning.NativeTBTCSignerStateWitnessProof{
			Schema:             frostsigning.NativeTBTCSignerStateWitnessProofSchema,
			StoreFingerprint:   request.StoreFingerprint,
			AncestorGeneration: request.AncestorGeneration,
			AncestorCommitment: request.AncestorCommitment,
			TargetGeneration:   request.TargetGeneration,
			TargetCommitment:   request.TargetCommitment,
			Complete:           true,
		}
		if request.AncestorGeneration == request.TargetGeneration {
			return proof, nil
		}
		if request.AncestorGeneration != 1 || request.AncestorCommitment != ancestor {
			return nil, fmt.Errorf("unknown ancestor")
		}
		proof.Entries = []frostsigning.NativeTBTCSignerStateWitnessProofEntry{
			{
				Generation:              2,
				PreviousStateCommitment: ancestor,
				StateCommitment:         target,
				StateImageDigest:        stateImage,
			},
		}
		return proof, nil
	}
	binding, err := newFrostNativeSignerInventoryBinding(
		storeBinding,
		anchorSource,
		func() (*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory, error) {
			return inventory, nil
		},
		proofReader,
	)
	if err != nil {
		t.Fatal(err)
	}
	expected := []frostNativeSignerInventoryExpectation{
		{
			WalletID:         walletID,
			KeyGroup:         hex.EncodeToString(walletID[:]),
			Threshold:        51,
			ParticipantCount: 100,
			ParticipantSeats: []uint16{2},
		},
	}
	if _, err := binding.verify(context.Background(), expected); err != nil {
		t.Fatalf("anchored descendant inventory was rejected: [%v]", err)
	}

	anchorSource.anchor = &FrostNativeSignerStateWitnessAnchor{
		StoreFingerprint: storeFingerprint,
		Generation:       2,
		Commitment:       [32]byte{0xff},
	}
	if _, err := binding.verify(context.Background(), expected); err == nil ||
		!strings.Contains(err.Error(), "equal state-witness generations") {
		t.Fatalf("equal-generation rollback fork was accepted: [%v]", err)
	}

	anchorSource.anchor = &FrostNativeSignerStateWitnessAnchor{
		StoreFingerprint: storeFingerprint,
		Generation:       3,
		Commitment:       [32]byte{0xfe},
	}
	if _, err := binding.verify(context.Background(), expected); err == nil ||
		!strings.Contains(err.Error(), "ancestry bounds") {
		t.Fatalf("lower-generation rolled-back signer state was accepted: [%v]", err)
	}
}

func TestFrostRetainedGroupJournalNativeInventoryExpectationsIncludeTerminalGroups(
	t *testing.T,
) {
	point := FrostPreSignFinality{BlockNumber: 17, BlockHash: [32]byte{0x61}}
	operatorIDs := make([]uint32, 51)
	for index := range operatorIDs {
		operatorIDs[index] = 2
	}
	operatorIDs[1] = 1
	operatorIDs[2] = 1
	terminalWalletID := [32]byte{0x62}
	terminalKeyGroup := hex.EncodeToString(terminalWalletID[:])
	journal := &frostRetainedGroupJournal{
		source: &testFrostRetainedGroupHistorySource{},
		walletRegistry: &walletRegistry{
			walletCache: make(map[string]*walletCacheValue),
			retainedFrostKeyGroups: map[[32]byte]string{
				terminalWalletID: terminalKeyGroup,
			},
		},
		operatorAddress: "0x01",
		state: frostRetainedGroupJournalState{
			CurrentPoint: point,
			Wallets: []frostRetainedGroupWalletState{
				{
					WalletID:    [32]byte{0x62},
					OperatorIDs: operatorIDs,
					Lifecycle:   FrostRetainedGroupClosed,
				},
				{
					WalletID:    [32]byte{0x63},
					OperatorIDs: []uint32{4, 5},
					Lifecycle:   FrostRetainedGroupLive,
				},
			},
		},
	}
	expected, _, err := journal.nativeSignerInventoryExpectations(context.Background(), point)
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) != 1 || expected[0].WalletID != terminalWalletID ||
		expected[0].KeyGroup != terminalKeyGroup ||
		expected[0].ParticipantCount != 51 || expected[0].ShareEpoch != 0 ||
		len(expected[0].ParticipantSeats) != 2 ||
		expected[0].ParticipantSeats[0] != 2 || expected[0].ParticipantSeats[1] != 3 {
		t.Fatalf("unexpected native inventory expectations: %+v", expected)
	}

	journal.walletRegistry.retainedFrostKeyGroups = nil
	if _, _, err := journal.nativeSignerInventoryExpectations(
		context.Background(),
		point,
	); err == nil || !strings.Contains(err.Error(), "no durable local key-group binding") {
		t.Fatalf("terminal group without durable key-group binding was accepted: [%v]", err)
	}
}

func TestFrostProductionSignerReadinessRejectsConcurrentRegistryChange(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	keyGroup := hex.EncodeToString(fixture.walletID[:])
	fixture.registry.retainedFrostKeyGroups = map[[32]byte]string{
		fixture.walletID: keyGroup,
	}
	journalDirectory := t.TempDir()
	if err := os.Chmod(journalDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	journal := fixture.openJournal(t, journalDirectory)

	storeBinding := testFrostDurableSessionStoreBinding(t)
	storeFingerprint, err := storeBinding.verify()
	if err != nil {
		t.Fatal(err)
	}
	stateCommitment := [32]byte{0x71}
	entries := []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
		WalletID:         fixture.walletID,
		KeyGroup:         keyGroup,
		Threshold:        frostPreSignAuthorizationThreshold,
		ParticipantCount: uint16(len(fixture.operatorIDs)),
		KeyPackages: []frostsigning.NativeTBTCSignerRetainedKeyPackage{{
			ParticipantSeat: 7,
		}},
	}}
	inventory := &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
		Schema:              frostsigning.NativeTBTCSignerRetainedKeyPackageInventorySchema,
		StoreFingerprint:    storeFingerprint,
		StateGeneration:     1,
		StateCommitment:     stateCommitment,
		InventoryCommitment: frostsigning.ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment(entries),
		Entries:             entries,
	}
	anchorSource := &testFrostNativeSignerStateWitnessAnchorSource{
		anchor: &FrostNativeSignerStateWitnessAnchor{
			StoreFingerprint: storeFingerprint,
			Generation:       1,
			Commitment:       stateCommitment,
		},
	}
	inventoryReads := 0
	inventoryBinding, err := newFrostNativeSignerInventoryBinding(
		storeBinding,
		anchorSource,
		func() (*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory, error) {
			inventoryReads++
			if inventoryReads == 2 {
				fixture.registry.mutex.Lock()
				fixture.registry.revision++
				fixture.registry.mutex.Unlock()
			}
			return inventory, nil
		},
		func(
			request *frostsigning.NativeTBTCSignerStateWitnessProofRequest,
		) (*frostsigning.NativeTBTCSignerStateWitnessProof, error) {
			return &frostsigning.NativeTBTCSignerStateWitnessProof{
				Schema:             frostsigning.NativeTBTCSignerStateWitnessProofSchema,
				StoreFingerprint:   request.StoreFingerprint,
				AncestorGeneration: request.AncestorGeneration,
				AncestorCommitment: request.AncestorCommitment,
				TargetGeneration:   request.TargetGeneration,
				TargetCommitment:   request.TargetCommitment,
				Complete:           true,
			}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	readiness, err := newFrostProductionSignerReadiness(
		func() bool { return true },
		journal,
		inventoryBinding,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = readiness.verifyFrostProductionSignerReadiness(
		context.Background(),
		fixture.target,
	)
	if err == nil || !strings.Contains(err.Error(), "registry changed") {
		t.Fatalf("concurrent local registry change was accepted: [%v]", err)
	}
}
