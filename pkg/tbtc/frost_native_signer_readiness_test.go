package tbtc

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type testFrostNativeSignerStateWitnessAnchorStore struct {
	record *FrostNativeSignerStateWitnessAnchorRecord
	err    error
}

func (store *testFrostNativeSignerStateWitnessAnchorStore) ReadFrostNativeSignerStateWitnessAnchor(
	context.Context,
) (*FrostNativeSignerStateWitnessAnchorRecord, error) {
	if store.err != nil {
		return nil, store.err
	}
	if store.record == nil {
		return nil, nil
	}
	result := *store.record
	return &result, nil
}

func (store *testFrostNativeSignerStateWitnessAnchorStore) CompareAndSwapFrostNativeSignerStateWitnessAnchor(
	context.Context,
	FrostNativeSignerStateWitnessCheckpoint,
	FrostNativeSignerStateWitnessCheckpoint,
	[]frostsigning.NativeTBTCSignerStateWitnessProofEntry,
) (*FrostNativeSignerStateWitnessAnchorCASResult, error) {
	return nil, fmt.Errorf("unexpected test anchor CAS")
}

func (store *testFrostNativeSignerStateWitnessAnchorStore) ReadFrostNativeSignerStateWitnessAnchorHistory(
	context.Context,
	FrostNativeSignerStateWitnessAnchorReference,
) (*FrostNativeSignerStateWitnessAnchorHistory, error) {
	return nil, fmt.Errorf("unexpected test anchor history read")
}

func testFrostNativeSignerInventoryAnchorBinding(
	storeFingerprint [32]byte,
	checkpoint FrostNativeSignerStateWitnessCheckpoint,
) (
	*frostNativeSignerAnchorBinding,
	*testFrostNativeSignerStateWitnessAnchorStore,
) {
	bindingHash := [32]byte{0xa1}
	eventRoot := [32]byte{0xa2}
	acknowledgementDigest := [32]byte{0xa3}
	tip := &frostsigning.NativeTBTCSignerStateWitnessTip{
		Schema:                      frostsigning.NativeTBTCSignerStateWitnessTipSchema,
		StoreFingerprint:            checkpoint.StoreFingerprint,
		Generation:                  checkpoint.Generation,
		PreviousStateCommitment:     checkpoint.PreviousStateCommitment,
		StateImageDigest:            checkpoint.StateImageDigest,
		StateCommitment:             checkpoint.StateCommitment,
		WitnessBaseGeneration:       checkpoint.Generation,
		WitnessBaseCommitment:       checkpoint.StateCommitment,
		AnchorBindingHash:           bindingHash,
		AnchorServiceEpoch:          1,
		AnchorRevision:              1,
		AnchorEventRoot:             eventRoot,
		AnchorAcknowledgementDigest: acknowledgementDigest,
	}
	store := &testFrostNativeSignerStateWitnessAnchorStore{
		record: &FrostNativeSignerStateWitnessAnchorRecord{
			Checkpoint:            checkpoint,
			BindingHash:           bindingHash,
			AcknowledgementDigest: acknowledgementDigest,
			OperationID:           [32]byte{0xa4},
			TransitionDigest:      [32]byte{0xa5},
			ServiceEpoch:          1,
			Revision:              1,
			EventRoot:             eventRoot,
			AcknowledgementJSON:   []byte(`{"test":"ack"}`),
		},
	}
	return &frostNativeSignerAnchorBinding{
		store:       store,
		identity:    FrostNativeSignerAnchorIdentity{SignerStoreFingerprint: storeFingerprint},
		bindingHash: bindingHash,
		floor: FrostNativeSignerStateWitnessAnchorReference{
			ServiceEpoch: 1,
			Revision:     1,
			Checkpoint:   checkpoint,
		},
		readTip: func() (*frostsigning.NativeTBTCSignerStateWitnessTip, error) {
			result := *tip
			return &result, nil
		},
	}, store
}

func testFrostNativeSignerInventoryTrustHead(
	binding *frostNativeSignerAnchorBinding,
) *frostsigning.NativeTBTCSignerStateAnchorTrustHead {
	floor := binding.floor
	return &frostsigning.NativeTBTCSignerStateAnchorTrustHead{
		Schema:                          frostsigning.NativeTBTCSignerStateAnchorTrustHeadSchema,
		CertificateSequence:             1,
		CertificateDigest:               [32]byte{0xb1},
		ActivationManifestSequence:      1,
		ActivationManifestHash:          [32]byte{0xb2},
		BindingHash:                     binding.bindingHash,
		ResponsePublicKeySPKISHA256:     [32]byte{0xb3},
		OfflineAuthoritySPKISHA256:      [32]byte{0xb4},
		ServiceEpoch:                    floor.ServiceEpoch,
		WitnessMaximumRecords:           4096,
		WitnessRotationThresholdRecords: 1024,
		CertifiedFloor: frostsigning.NativeTBTCSignerStateAnchorTrustReference{
			ServiceEpoch:          floor.ServiceEpoch,
			Revision:              floor.Revision,
			EventRoot:             floor.EventRoot,
			AcknowledgementDigest: floor.AcknowledgementDigest,
			Checkpoint: frostsigning.NativeTBTCSignerStateAnchorCheckpoint{
				StoreFingerprint:        floor.Checkpoint.StoreFingerprint,
				Generation:              floor.Checkpoint.Generation,
				PreviousStateCommitment: floor.Checkpoint.PreviousStateCommitment,
				StateImageDigest:        floor.Checkpoint.StateImageDigest,
				StateCommitment:         floor.Checkpoint.StateCommitment,
			},
		},
	}
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

func TestValidateFrostNativeSignerAnchorReadinessHeadroomFreezesAtBound(
	t *testing.T,
) {
	inventory := &frostNativeSignerInventorySnapshot{
		CertifiedFloorRevision: 1,
		CurrentAnchorRevision: 1 +
			FrostNativeSignerAnchorMaximumHistoryEvents - 1,
		CertifiedFloorGeneration:      1,
		StateGeneration:               1,
		RestartableRevisionHeadroom:   1,
		RestartableGenerationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
		AnchorRotationWarning:         true,
	}
	if err := validateFrostNativeSignerAnchorReadinessHeadroom(
		inventory,
	); err != nil {
		t.Fatalf("last usable revision was rejected: %v", err)
	}
	inventory.CurrentAnchorRevision++
	inventory.RestartableRevisionHeadroom = 0
	if err := validateFrostNativeSignerAnchorReadinessHeadroom(
		inventory,
	); err == nil || !strings.Contains(err.Error(), "rotation is required") {
		t.Fatalf("exhausted revision window remained readiness-valid: %v", err)
	}
	inventory.CurrentAnchorRevision++
	if err := validateFrostNativeSignerAnchorReadinessHeadroom(
		inventory,
	); err == nil || !strings.Contains(err.Error(), "inconsistent") {
		t.Fatalf("beyond-bound readiness snapshot was accepted: %v", err)
	}
}

func TestValidateFrostNativeSignerAnchorReadinessHeadroomTracksGenerationBound(
	t *testing.T,
) {
	inventory := &frostNativeSignerInventorySnapshot{
		CertifiedFloorRevision:        1,
		CurrentAnchorRevision:         1,
		RestartableRevisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
		CertifiedFloorGeneration:      1,
		StateGeneration:               FrostNativeSignerAnchorMaximumHistoryProofEntries,
		RestartableGenerationHeadroom: 1,
		AnchorRotationWarning:         true,
	}
	if err := validateFrostNativeSignerAnchorReadinessHeadroom(
		inventory,
	); err != nil {
		t.Fatalf("last usable generation was rejected: %v", err)
	}
	inventory.StateGeneration++
	inventory.RestartableGenerationHeadroom = 0
	if err := validateFrostNativeSignerAnchorReadinessHeadroom(
		inventory,
	); err == nil || !strings.Contains(err.Error(), "rotation is required") {
		t.Fatalf("exhausted generation window remained readiness-valid: %v", err)
	}
	inventory.StateGeneration++
	if err := validateFrostNativeSignerAnchorReadinessHeadroom(
		inventory,
	); err == nil || !strings.Contains(err.Error(), "inconsistent") {
		t.Fatalf("beyond-bound generation snapshot was accepted: %v", err)
	}
}

func TestValidateFrostNativeSignerAnchorReadinessHeadroomWarningUsesMinimum(
	t *testing.T,
) {
	inventory := &frostNativeSignerInventorySnapshot{
		CertifiedFloorRevision:      1,
		CurrentAnchorRevision:       1,
		RestartableRevisionHeadroom: FrostNativeSignerAnchorMaximumHistoryEvents,
		CertifiedFloorGeneration:    1,
	}
	for _, test := range []struct {
		headroom uint64
		warning  bool
	}{
		{FrostNativeSignerAnchorRotationWarningHeadroom, true},
		{FrostNativeSignerAnchorRotationWarningHeadroom + 1, false},
	} {
		inventory.RestartableGenerationHeadroom = test.headroom
		inventory.StateGeneration = 1 +
			FrostNativeSignerAnchorMaximumHistoryProofEntries -
			test.headroom
		inventory.AnchorRotationWarning = test.warning
		if err := validateFrostNativeSignerAnchorReadinessHeadroom(
			inventory,
		); err != nil {
			t.Fatalf(
				"generation warning boundary [%d] was rejected: %v",
				test.headroom,
				err,
			)
		}
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

func TestFrostNativeSignerInventoryBindingRequiresExactAuthenticatedAnchor(t *testing.T) {
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
	checkpoint := FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              2,
		PreviousStateCommitment: ancestor,
		StateImageDigest:        stateImage,
		StateCommitment:         target,
	}
	anchorBinding, anchorStore :=
		testFrostNativeSignerInventoryAnchorBinding(storeFingerprint, checkpoint)
	trustHead := testFrostNativeSignerInventoryTrustHead(anchorBinding)
	binding, err := newFrostNativeSignerInventoryBinding(
		storeBinding,
		anchorBinding,
		func() (*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory, error) {
			return inventory, nil
		},
		func() (*frostsigning.NativeTBTCSignerStateAnchorTrustHead, error) {
			copy := *trustHead
			return &copy, nil
		},
		trustHead,
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
	snapshot, err := binding.verify(context.Background(), expected)
	if err != nil {
		t.Fatalf("anchored descendant inventory was rejected: [%v]", err)
	}
	if snapshot.AnchorServiceEpoch != 1 ||
		snapshot.CertifiedFloorRevision != 1 ||
		snapshot.CertifiedFloorGeneration != 2 ||
		snapshot.CurrentAnchorRevision != 1 ||
		snapshot.RestartableRevisionHeadroom !=
			FrostNativeSignerAnchorMaximumHistoryEvents ||
		snapshot.RestartableGenerationHeadroom !=
			FrostNativeSignerAnchorMaximumHistoryProofEntries ||
		snapshot.AnchorRotationWarning {
		t.Fatalf("unexpected native anchor readiness headroom: %+v", snapshot)
	}

	warningTip, err := anchorBinding.readTip()
	if err != nil {
		t.Fatal(err)
	}
	baselineTip := *warningTip
	baselineRecord := *anchorStore.record
	warningTip.AnchorRevision =
		anchorBinding.floor.Revision +
			FrostNativeSignerAnchorMaximumHistoryEvents - 1
	warningTip.AnchorEventRoot = [32]byte{0xd1}
	warningTip.AnchorAcknowledgementDigest = [32]byte{0xd2}
	anchorBinding.readTip = func() (
		*frostsigning.NativeTBTCSignerStateWitnessTip,
		error,
	) {
		copy := *warningTip
		return &copy, nil
	}
	anchorStore.record.Revision = warningTip.AnchorRevision
	anchorStore.record.PreviousEventRoot = [32]byte{0xd0}
	anchorStore.record.EventRoot = warningTip.AnchorEventRoot
	anchorStore.record.AcknowledgementDigest =
		warningTip.AnchorAcknowledgementDigest
	warningSnapshot, err := binding.verify(context.Background(), expected)
	if err != nil {
		t.Fatalf("warning-headroom inventory was rejected: %v", err)
	}
	if warningSnapshot.RestartableRevisionHeadroom != 1 ||
		!warningSnapshot.AnchorRotationWarning {
		t.Fatalf(
			"revision exhaustion warning was not surfaced: %+v",
			warningSnapshot,
		)
	}

	// Restore the exact baseline before the fork checks below.
	*warningTip = baselineTip
	*anchorStore.record = baselineRecord

	forkImage := [32]byte{0xff}
	anchorStore.record.Checkpoint = FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              2,
		PreviousStateCommitment: ancestor,
		StateImageDigest:        forkImage,
		StateCommitment: frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			storeFingerprint,
			2,
			ancestor,
			forkImage,
		),
	}
	if _, err := binding.verify(context.Background(), expected); err == nil ||
		!strings.Contains(err.Error(), "differs from the authenticated remote anchor") {
		t.Fatalf("equal-generation rollback fork was accepted: [%v]", err)
	}

	aheadImage := [32]byte{0xfe}
	anchorStore.record.Checkpoint = FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              3,
		PreviousStateCommitment: target,
		StateImageDigest:        aheadImage,
		StateCommitment: frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			storeFingerprint,
			3,
			target,
			aheadImage,
		),
	}
	if _, err := binding.verify(context.Background(), expected); err == nil ||
		!strings.Contains(err.Error(), "differs from the authenticated remote anchor") {
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
	previousStateCommitment := [32]byte{0x71}
	stateImageDigest := [32]byte{0x72}
	stateCommitment := frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		1,
		previousStateCommitment,
		stateImageDigest,
	)
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
		Schema:                  frostsigning.NativeTBTCSignerRetainedKeyPackageInventorySchema,
		StoreFingerprint:        storeFingerprint,
		StateGeneration:         1,
		StateCommitment:         stateCommitment,
		PreviousStateCommitment: previousStateCommitment,
		StateImageDigest:        stateImageDigest,
		InventoryCommitment:     frostsigning.ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment(entries),
		Entries:                 entries,
	}
	checkpoint := FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              1,
		PreviousStateCommitment: previousStateCommitment,
		StateImageDigest:        stateImageDigest,
		StateCommitment:         stateCommitment,
	}
	anchorBinding, _ :=
		testFrostNativeSignerInventoryAnchorBinding(storeFingerprint, checkpoint)
	trustHead := testFrostNativeSignerInventoryTrustHead(anchorBinding)
	inventoryReads := 0
	inventoryBinding, err := newFrostNativeSignerInventoryBinding(
		storeBinding,
		anchorBinding,
		func() (*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory, error) {
			inventoryReads++
			if inventoryReads == 2 {
				fixture.registry.mutex.Lock()
				fixture.registry.revision++
				fixture.registry.mutex.Unlock()
			}
			return inventory, nil
		},
		func() (*frostsigning.NativeTBTCSignerStateAnchorTrustHead, error) {
			copy := *trustHead
			return &copy, nil
		},
		trustHead,
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

func TestFrostProductionSignerReadinessRejectsChangedJournalStamp(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	journalDirectory := t.TempDir()
	if err := os.Chmod(journalDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	journal := fixture.openJournal(t, journalDirectory)
	expected, err := journal.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	readiness := &frostProductionSignerReadiness{journal: journal}
	if err := readiness.verifyFrostRetainedGroupJournalStampUnchanged(
		context.Background(),
		expected,
	); err != nil {
		t.Fatalf("matching retained-group journal stamp was rejected: [%v]", err)
	}

	journal.quarantineState.Generation++
	if err := readiness.verifyFrostRetainedGroupJournalStampUnchanged(
		context.Background(),
		expected,
	); err == nil || !strings.Contains(err.Error(), "journal changed") {
		t.Fatalf("advanced quarantine journal stamp was accepted: [%v]", err)
	}
	journal.quarantineState.Generation--
}

func TestFrostProductionSignerReadinessWaitsOutBusyJournalStamp(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	journalDirectory := t.TempDir()
	if err := os.Chmod(journalDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	journal := fixture.openJournal(t, journalDirectory)
	expected, err := journal.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	readiness := &frostProductionSignerReadiness{journal: journal}

	// Another workflow holding the journal is contention, not a readiness
	// change. The monitor latches the first revalidation error permanently, so
	// the stamp check must wait the holder out instead of reporting one.
	journal.mutex.Lock()
	released := make(chan struct{})
	go func() {
		defer close(released)
		time.Sleep(50 * time.Millisecond)
		journal.mutex.Unlock()
	}()
	if err := readiness.verifyFrostRetainedGroupJournalStampUnchanged(
		context.Background(),
		expected,
	); err != nil {
		t.Fatalf("busy retained-group journal was not waited out: [%v]", err)
	}
	<-released

	// Only the caller's own deadline ends the wait.
	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	deadlineContext, cancel := context.WithTimeout(
		context.Background(),
		20*time.Millisecond,
	)
	defer cancel()
	if err := readiness.verifyFrostRetainedGroupJournalStampUnchanged(
		deadlineContext,
		expected,
	); err == nil || !strings.Contains(err.Error(), "stayed busy") {
		t.Fatalf("busy journal ignored the revalidation deadline: [%v]", err)
	}
}

// TestFrostProductionSignerReadinessAcceptsAuthorizedGenerationAdvance pins the
// asymmetry the cached revalidation path depends on: the authorized signing
// window durably advances the native store it is guarding, so its own advance
// must not read as a readiness change, while a rollback still must.
func TestFrostProductionSignerReadinessAcceptsAuthorizedGenerationAdvance(
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
	previousStateCommitment := [32]byte{0x71}
	stateImageDigest := [32]byte{0x72}
	stateCommitment := frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		1,
		previousStateCommitment,
		stateImageDigest,
	)
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
		Schema:                  frostsigning.NativeTBTCSignerRetainedKeyPackageInventorySchema,
		StoreFingerprint:        storeFingerprint,
		StateGeneration:         1,
		StateCommitment:         stateCommitment,
		PreviousStateCommitment: previousStateCommitment,
		StateImageDigest:        stateImageDigest,
		InventoryCommitment:     frostsigning.ComputeNativeTBTCSignerRetainedKeyPackageInventoryCommitment(entries),
		Entries:                 entries,
	}
	checkpoint := FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              1,
		PreviousStateCommitment: previousStateCommitment,
		StateImageDigest:        stateImageDigest,
		StateCommitment:         stateCommitment,
	}
	anchorBinding, anchorStore :=
		testFrostNativeSignerInventoryAnchorBinding(storeFingerprint, checkpoint)
	trustHead := testFrostNativeSignerInventoryTrustHead(anchorBinding)
	inventoryBinding, err := newFrostNativeSignerInventoryBinding(
		storeBinding,
		anchorBinding,
		func() (*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory, error) {
			return inventory, nil
		},
		func() (*frostsigning.NativeTBTCSignerStateAnchorTrustHead, error) {
			copy := *trustHead
			return &copy, nil
		},
		trustHead,
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
	snapshot, err := readiness.verifyFrostProductionSignerReadiness(
		context.Background(),
		fixture.target,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := readiness.verifyFrostProductionSignerReadinessUnchanged(
		context.Background(),
		snapshot,
	); err != nil {
		t.Fatalf("unmutated native signer state was rejected: [%v]", err)
	}

	// Advance the durable state exactly as one anchored interactive call does:
	// the consumption marker persists a new generation and the process output
	// barrier advances the anchor revision once.
	baselineTip, err := anchorBinding.readTip()
	if err != nil {
		t.Fatal(err)
	}
	baselineRecord := *anchorStore.record
	advancedImage := [32]byte{0x73}
	advancedCommitment := frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
		storeFingerprint,
		2,
		stateCommitment,
		advancedImage,
	)
	advancedCheckpoint := FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        storeFingerprint,
		Generation:              2,
		PreviousStateCommitment: stateCommitment,
		StateImageDigest:        advancedImage,
		StateCommitment:         advancedCommitment,
	}
	advancedEventRoot := [32]byte{0x74}
	advancedTip := *baselineTip
	advancedTip.Generation = advancedCheckpoint.Generation
	advancedTip.PreviousStateCommitment = advancedCheckpoint.PreviousStateCommitment
	advancedTip.StateImageDigest = advancedCheckpoint.StateImageDigest
	advancedTip.StateCommitment = advancedCheckpoint.StateCommitment
	advancedTip.AnchorRevision = baselineTip.AnchorRevision + 1
	advancedTip.AnchorEventRoot = advancedEventRoot
	anchorBinding.readTip = func() (
		*frostsigning.NativeTBTCSignerStateWitnessTip,
		error,
	) {
		copy := advancedTip
		return &copy, nil
	}
	anchorStore.record.Checkpoint = advancedCheckpoint
	anchorStore.record.Revision = advancedTip.AnchorRevision
	anchorStore.record.PreviousEventRoot = baselineRecord.EventRoot
	anchorStore.record.EventRoot = advancedEventRoot
	inventory.StateGeneration = advancedCheckpoint.Generation
	inventory.PreviousStateCommitment = advancedCheckpoint.PreviousStateCommitment
	inventory.StateImageDigest = advancedCheckpoint.StateImageDigest
	inventory.StateCommitment = advancedCheckpoint.StateCommitment

	if err := readiness.verifyFrostProductionSignerReadinessUnchanged(
		context.Background(),
		snapshot,
	); err != nil {
		t.Fatalf(
			"the signing session's own authorized durable advance aborted revalidation: [%v]",
			err,
		)
	}

	// A rolled-back native store is still a fatal readiness change.
	anchorBinding.readTip = func() (
		*frostsigning.NativeTBTCSignerStateWitnessTip,
		error,
	) {
		copy := *baselineTip
		return &copy, nil
	}
	*anchorStore.record = baselineRecord
	inventory.StateGeneration = checkpoint.Generation
	inventory.PreviousStateCommitment = checkpoint.PreviousStateCommitment
	inventory.StateImageDigest = checkpoint.StateImageDigest
	inventory.StateCommitment = checkpoint.StateCommitment
	snapshot.Inventory.StateGeneration = advancedCheckpoint.Generation
	snapshot.Inventory.StateCommitment = advancedCheckpoint.StateCommitment
	snapshot.Inventory.PreviousStateCommitment =
		advancedCheckpoint.PreviousStateCommitment
	snapshot.Inventory.StateImageDigest = advancedCheckpoint.StateImageDigest
	snapshot.Inventory.CurrentAnchorRevision = advancedTip.AnchorRevision
	snapshot.Inventory.RestartableRevisionHeadroom--
	snapshot.Inventory.RestartableGenerationHeadroom--
	if err := readiness.verifyFrostProductionSignerReadinessUnchanged(
		context.Background(),
		snapshot,
	); err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("rolled-back native signer state was accepted: [%v]", err)
	}
}

func TestVerifyFrostNativeSignerInventoryUnchangedSeparatesAdvanceFromChange(
	t *testing.T,
) {
	baseline := func() *frostNativeSignerInventorySnapshot {
		return &frostNativeSignerInventorySnapshot{
			Schema:                        "inventory/v1",
			StoreFingerprint:              [32]byte{0x01},
			StateGeneration:               10,
			StateCommitment:               [32]byte{0x02},
			PreviousStateCommitment:       [32]byte{0x03},
			StateImageDigest:              [32]byte{0x04},
			InventoryCommitment:           [32]byte{0x05},
			WalletCount:                   1,
			KeyPackageCount:               1,
			ExternalRollbackAnchorBound:   true,
			TrustCertificateSequence:      1,
			TrustCertificateDigest:        [32]byte{0x06},
			AnchorServiceEpoch:            1,
			CertifiedFloorRevision:        1,
			CertifiedFloorGeneration:      1,
			CurrentAnchorRevision:         5,
			RestartableRevisionHeadroom:   4092,
			RestartableGenerationHeadroom: 4087,
		}
	}
	tests := map[string]struct {
		mutate   func(*frostNativeSignerInventorySnapshot)
		accepted bool
		message  string
	}{
		"unchanged": {
			mutate:   func(*frostNativeSignerInventorySnapshot) {},
			accepted: true,
		},
		"authorized durable advance": {
			mutate: func(actual *frostNativeSignerInventorySnapshot) {
				actual.StateGeneration++
				actual.PreviousStateCommitment = actual.StateCommitment
				actual.StateCommitment = [32]byte{0x12}
				actual.StateImageDigest = [32]byte{0x14}
				actual.CurrentAnchorRevision++
				actual.RestartableRevisionHeadroom--
				actual.RestartableGenerationHeadroom--
			},
			accepted: true,
		},
		"generation rollback": {
			mutate: func(actual *frostNativeSignerInventorySnapshot) {
				actual.StateGeneration--
				actual.RestartableGenerationHeadroom++
			},
			message: "rolled back",
		},
		"anchor revision rollback": {
			mutate: func(actual *frostNativeSignerInventorySnapshot) {
				actual.CurrentAnchorRevision--
				actual.RestartableRevisionHeadroom++
			},
			message: "rolled back",
		},
		"fork at unchanged generation": {
			mutate: func(actual *frostNativeSignerInventorySnapshot) {
				actual.StateCommitment = [32]byte{0x22}
			},
			message: "forked at an unchanged generation",
		},
		"another durable store": {
			mutate: func(actual *frostNativeSignerInventorySnapshot) {
				actual.StoreFingerprint = [32]byte{0x31}
			},
			message: "identity, trust head, or retained key material changed",
		},
		"replaced key material": {
			mutate: func(actual *frostNativeSignerInventorySnapshot) {
				actual.InventoryCommitment = [32]byte{0x32}
			},
			message: "identity, trust head, or retained key material changed",
		},
		"rotated trust certificate": {
			mutate: func(actual *frostNativeSignerInventorySnapshot) {
				actual.TrustCertificateSequence++
			},
			message: "identity, trust head, or retained key material changed",
		},
		"advance into the rotation warning band": {
			mutate: func(actual *frostNativeSignerInventorySnapshot) {
				actual.StateGeneration +=
					actual.RestartableGenerationHeadroom -
						FrostNativeSignerAnchorRotationWarningHeadroom
				actual.RestartableGenerationHeadroom =
					FrostNativeSignerAnchorRotationWarningHeadroom
				actual.PreviousStateCommitment = actual.StateCommitment
				actual.StateCommitment = [32]byte{0x42}
				actual.AnchorRotationWarning = true
			},
			message: "identity, trust head, or retained key material changed",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			expected := baseline()
			actual := baseline()
			test.mutate(actual)
			err := verifyFrostNativeSignerInventoryUnchanged(expected, actual)
			if test.accepted {
				if err != nil {
					t.Fatalf("legitimate native signer state was rejected: [%v]", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("native signer state change was accepted: [%v]", err)
			}
		})
	}
}
