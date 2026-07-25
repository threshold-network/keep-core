package tbtc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type journalHistorySource struct {
	identity    FrostRetainedGroupHistoryIdentity
	checkpoint  FrostPreSignFinality
	head        FrostPreSignFinality
	descriptor  [32]byte
	mutations   []FrostRetainedGroupMutation
	complete    bool
	emptyAtFrom bool
	points      map[uint64][32]byte
	operators   map[chain.Address]chain.OperatorID
	verifyErr   error
}

func (jhs *journalHistorySource) Identity(
	context.Context,
) (FrostRetainedGroupHistoryIdentity, error) {
	return jhs.identity, nil
}

func (jhs *journalHistorySource) FinalizedHead(
	context.Context,
) (FrostPreSignFinality, error) {
	return jhs.head, nil
}

func (jhs *journalHistorySource) VerifyPoint(
	_ context.Context,
	point FrostPreSignFinality,
) error {
	if jhs.verifyErr != nil {
		return jhs.verifyErr
	}
	if expected, ok := jhs.points[point.BlockNumber]; !ok || expected != point.BlockHash {
		return fmt.Errorf("point is not canonical")
	}
	return nil
}

func (jhs *journalHistorySource) ReadCompleteHistory(
	_ context.Context,
	from FrostPreSignFinality,
	to FrostPreSignFinality,
) (*FrostRetainedGroupHistory, error) {
	mutations := make([]FrostRetainedGroupMutation, 0)
	for _, mutation := range jhs.mutations {
		if mutation.Point.BlockNumber <= to.BlockNumber {
			mutations = append(mutations, mutation)
		}
	}
	return &FrostRetainedGroupHistory{
		From:              from,
		To:                to,
		Mutations:         cloneFrostRetainedGroupMutations(mutations),
		Complete:          jhs.complete,
		EmptyAtFrom:       jhs.emptyAtFrom,
		DescriptorSetHash: jhs.descriptor,
	}, nil
}

func (jhs *journalHistorySource) ResolveOperatorID(
	_ context.Context,
	address chain.Address,
	_ FrostPreSignFinality,
) (chain.OperatorID, error) {
	operatorID := jhs.operators[address]
	if operatorID == 0 {
		return 0, fmt.Errorf("unknown operator")
	}
	return operatorID, nil
}

type journalTestFixture struct {
	manifest      FrostRetainedGroupCanonicalJournalManifest
	quarantine    FrostRetainedGroupQuarantineJournalManifest
	manifestHash  [32]byte
	checkpoint    FrostPreSignFinality
	target        FrostPreSignFinality
	later         FrostPreSignFinality
	walletID      [32]byte
	walletPKH     [20]byte
	operatorIDs   []uint32
	operatorAddrs []chain.Address
	localOperator chain.Address
	registry      *walletRegistry
	source        *journalHistorySource
	admission     FrostRetainedGroupMutation
}

func newJournalTestFixture(t *testing.T) *journalTestFixture {
	t.Helper()
	checkpoint := FrostPreSignFinality{BlockNumber: 1, BlockHash: [32]byte{0x01}}
	target := FrostPreSignFinality{BlockNumber: 10, BlockHash: [32]byte{0x0a}}
	later := FrostPreSignFinality{BlockNumber: 20, BlockHash: [32]byte{0x14}}
	operatorIDs := make([]uint32, 51)
	operatorAddrs := make([]chain.Address, 51)
	operators := make(map[chain.Address]chain.OperatorID, 51)
	for index := range operatorIDs {
		operatorIDs[index] = uint32(index + 1)
		operatorAddrs[index] = chain.Address(fmt.Sprintf("operator-%02d", index+1))
		operators[operatorAddrs[index]] = chain.OperatorID(index + 1)
	}
	walletID := [32]byte{0x91}
	walletPKH := [20]byte{0x92}
	localOperator := operatorAddrs[6]
	publicKey := &ecdsa.PublicKey{Curve: elliptic.P256()}
	signerMaterialPayload, err := json.Marshal(
		frostsigning.NativeTBTCSignerMaterialPayload{
			KeyGroup:       fmt.Sprintf("%x", walletID),
			KeyGroupSource: frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	localSigner := &signer{
		wallet: wallet{
			publicKey:             publicKey,
			signingGroupOperators: append([]chain.Address{}, operatorAddrs...),
		},
		signingGroupMemberIndex: group.MemberIndex(7),
		signerMaterial: &frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: signerMaterialPayload,
		},
	}
	registry := &walletRegistry{
		walletCache: map[string]*walletCacheValue{
			"wallet": {
				walletPublicKeyHash: walletPKH,
				walletID:            walletID,
				signers:             []*signer{localSigner},
			},
		},
	}
	manifest := FrostRetainedGroupCanonicalJournalManifest{
		StoreID:                   "journal-store-uuid",
		StoreFingerprint:          [32]byte{0x31},
		ClusterFingerprint:        [32]byte{0x32},
		Checkpoint:                checkpoint,
		DescriptorSetHash:         [32]byte{0x33},
		SourceTrustDomainID:       "independent-journal-source",
		SourceEndpointFingerprint: [32]byte{0x34},
		SourceOperatorFingerprint: [32]byte{0x35},
		MinimumGeneration:         1,
	}
	quarantine := FrostRetainedGroupQuarantineJournalManifest{
		ProtocolID:         [32]byte{0x41},
		StoreID:            "quarantine-store-uuid",
		StoreFingerprint:   [32]byte{0x42},
		ClusterFingerprint: [32]byte{0x43},
	}
	source := &journalHistorySource{
		identity: FrostRetainedGroupHistoryIdentity{
			TrustDomainID:       manifest.SourceTrustDomainID,
			EndpointFingerprint: manifest.SourceEndpointFingerprint,
			OperatorFingerprint: manifest.SourceOperatorFingerprint,
		},
		checkpoint:  checkpoint,
		head:        FrostPreSignFinality{BlockNumber: 100, BlockHash: [32]byte{0x64}},
		descriptor:  manifest.DescriptorSetHash,
		complete:    true,
		emptyAtFrom: true,
		points: map[uint64][32]byte{
			1:   checkpoint.BlockHash,
			10:  target.BlockHash,
			20:  later.BlockHash,
			100: {0x64},
		},
		operators: operators,
	}
	admission := FrostRetainedGroupMutation{
		Point: FrostRetainedGroupEventPoint{
			BlockNumber:      2,
			BlockHash:        [32]byte{0x02},
			TransactionHash:  [32]byte{0xa2},
			TransactionIndex: 1,
			LogIndex:         5,
		},
		Kind:                    FrostRetainedGroupAdmissionMutation,
		WalletID:                walletID,
		WalletPublicKeyHash:     walletPKH,
		OperatorIDs:             append([]uint32{}, operatorIDs...),
		RetainedGroupHash:       [32]byte{0x93},
		CreationPoint:           FrostRetainedGroupEventPoint{BlockNumber: 2, BlockHash: [32]byte{0x02}, TransactionHash: [32]byte{0xa2}, TransactionIndex: 1, LogIndex: 4},
		BridgeRegistrationPoint: FrostRetainedGroupEventPoint{BlockNumber: 2, BlockHash: [32]byte{0x02}, TransactionHash: [32]byte{0xa2}, TransactionIndex: 1, LogIndex: 5},
	}
	source.mutations = []FrostRetainedGroupMutation{admission}
	return &journalTestFixture{
		manifest:      manifest,
		quarantine:    quarantine,
		manifestHash:  [32]byte{0x42},
		checkpoint:    checkpoint,
		target:        target,
		later:         later,
		walletID:      walletID,
		walletPKH:     walletPKH,
		operatorIDs:   operatorIDs,
		operatorAddrs: operatorAddrs,
		localOperator: localOperator,
		registry:      registry,
		source:        source,
		admission:     admission,
	}
}

func TestFrostLocalSessionSnapshotBindsExactSignerMaterial(t *testing.T) {
	fixture := newJournalTestFixture(t)
	sessions, err := fixture.registry.frostLocalSessionSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	expectedKeyGroup := fmt.Sprintf("%x", fixture.walletID)
	if len(sessions) != 1 || sessions[0].WalletID != fixture.walletID ||
		sessions[0].KeyGroup != expectedKeyGroup {
		t.Fatalf("unexpected local FROST session snapshot: [%+v]", sessions)
	}

	mismatchedPayload, err := json.Marshal(
		frostsigning.NativeTBTCSignerMaterialPayload{
			KeyGroup:       strings.Repeat("22", 32),
			KeyGroupSource: frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	fixture.registry.walletCache["wallet"].signers[0].signerMaterial =
		&frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: mismatchedPayload,
		}
	if _, err := fixture.registry.frostLocalSessionSnapshot(); err == nil ||
		!strings.Contains(err.Error(), "does not identify its wallet") {
		t.Fatalf("mismatched local key-group material was accepted: [%v]", err)
	}
}

func (fixture *journalTestFixture) openJournal(
	t *testing.T,
	directory string,
) *frostRetainedGroupJournal {
	t.Helper()
	journal, err := newFrostRetainedGroupJournal(
		directory,
		fixture.manifestHash,
		fixture.manifest,
		fixture.quarantine,
		fixture.source,
		fixture.registry,
		fixture.localOperator,
	)
	if err != nil {
		t.Fatal(err)
	}
	return journal
}

func TestFrostRetainedGroupJournal_ReconcilesAndRejectsRewrittenHistory(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := fixture.openJournal(t, directory)
	defer journal.close()
	snapshot, err := journal.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Complete || snapshot.SnapshotGeneration != 1 ||
		snapshot.WalletCount != 1 || snapshot.LocalSessionCount != 1 ||
		snapshot.QuarantineCount != 0 || snapshot.InventoryRoot == [32]byte{} {
		t.Fatalf("unexpected journal snapshot: %+v", snapshot)
	}

	fixture.source.mutations[0].OperatorIDs[0] = 51
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "rewrote, omitted, or reordered") {
		t.Fatalf("expected canonical prefix rewrite failure, got [%v]", err)
	}
	fixture.source.mutations[0] = fixture.admission
	fixture.source.complete = false
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected incomplete-history failure, got [%v]", err)
	}
	fixture.source.complete = true
	fixture.source.emptyAtFrom = false
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected nonempty-checkpoint failure, got [%v]", err)
	}
}

func TestFrostRetainedGroupJournal_IntegratesCommittedOrphanBatchExactlyOnce(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := fixture.openJournal(t, directory)
	journal.persistFailureHook = func(stage string) error {
		if stage != "after-batch-before-state" {
			t.Fatalf("unexpected failure stage [%s]", stage)
		}
		return fmt.Errorf("simulated crash")
	}
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "simulated crash") {
		t.Fatalf("expected simulated crash, got [%v]", err)
	}
	if err := journal.close(); err != nil {
		t.Fatal(err)
	}

	restarted := fixture.openJournal(t, directory)
	defer restarted.close()
	snapshot, err := restarted.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SnapshotGeneration != 1 || restarted.state.BatchSequence != 1 ||
		len(restarted.mutations) != 1 {
		t.Fatalf("orphan batch was not integrated exactly once: %+v", restarted.state)
	}
}

func TestFrostRetainedGroupJournal_QuarantineAndAuthenticatedLiftAreIndependent(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	quarantine := FrostRetainedGroupMutation{
		Point: FrostRetainedGroupEventPoint{
			BlockNumber:      5,
			BlockHash:        [32]byte{0x05},
			TransactionHash:  [32]byte{0xa5},
			TransactionIndex: 1,
			LogIndex:         1,
		},
		Kind:         FrostRetainedGroupRecoveryRequiredMutation,
		WalletID:     fixture.walletID,
		QuarantineID: [32]byte{0x51},
		EvidenceHash: [32]byte{0x52},
		Reason:       "manual recovery is required",
	}
	fixture.source.mutations = append(fixture.source.mutations, quarantine)
	journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
	defer journal.close()
	first, err := journal.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	if first.SnapshotGeneration != 1 || first.QuarantineGeneration != 1 ||
		first.QuarantineCount != 1 {
		t.Fatalf("unexpected quarantined snapshot: %+v", first)
	}
	if journal.directory == journal.quarantineDirectory {
		t.Fatal("canonical and quarantine journals share a physical store")
	}
	if len(journal.mutations) != 1 ||
		len(journal.quarantineMutations) != 1 ||
		isFrostRetainedGroupQuarantineMutation(journal.mutations[0].Kind) ||
		!isFrostRetainedGroupQuarantineMutation(journal.quarantineMutations[0].Kind) {
		t.Fatal("canonical and quarantine mutations were not durably partitioned")
	}
	for _, metadataPath := range []string{
		filepath.Join(journal.directory, frostRetainedGroupJournalMetadataFile),
		filepath.Join(journal.quarantineDirectory, frostRetainedGroupJournalMetadataFile),
	} {
		if info, err := os.Lstat(metadataPath); err != nil ||
			!info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatalf("independent journal metadata is not durable and private: [%s] [%v]", metadataPath, err)
		}
	}
	lift := FrostRetainedGroupMutation{
		Point: FrostRetainedGroupEventPoint{
			BlockNumber:      15,
			BlockHash:        [32]byte{0x0f},
			TransactionHash:  [32]byte{0xaf},
			TransactionIndex: 2,
			LogIndex:         3,
		},
		Kind:               FrostRetainedGroupQuarantineLiftMutation,
		WalletID:           fixture.walletID,
		QuarantineID:       quarantine.QuarantineID,
		AuthenticationHash: [32]byte{0x53},
	}
	fixture.source.mutations = append(fixture.source.mutations, lift)
	second, err := journal.reconcile(context.Background(), fixture.later)
	if err != nil {
		t.Fatal(err)
	}
	if second.SnapshotGeneration != 1 || second.QuarantineGeneration != 2 ||
		second.QuarantineCount != 0 || second.QuarantineRoot == first.QuarantineRoot {
		t.Fatalf("unexpected lifted snapshot: %+v", second)
	}
}

func TestFrostRetainedGroupJournal_IntegratesQuarantineOrphanBatchExactlyOnce(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	fixture.source.mutations = append(
		fixture.source.mutations,
		FrostRetainedGroupMutation{
			Point: FrostRetainedGroupEventPoint{
				BlockNumber:      5,
				BlockHash:        [32]byte{0x05},
				TransactionHash:  [32]byte{0xa5},
				TransactionIndex: 1,
				LogIndex:         1,
			},
			Kind:         FrostRetainedGroupQuarantineMutation,
			WalletID:     fixture.walletID,
			QuarantineID: [32]byte{0x61},
			EvidenceHash: [32]byte{0x62},
			Reason:       "independent quarantine crash test",
		},
	)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := fixture.openJournal(t, directory)
	journal.persistFailureHook = func(stage string) error {
		switch stage {
		case "after-batch-before-state":
			return nil
		case "after-quarantine-batch-before-state":
			return fmt.Errorf("simulated quarantine crash")
		default:
			t.Fatalf("unexpected failure stage [%s]", stage)
			return nil
		}
	}
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "simulated quarantine crash") {
		t.Fatalf("expected simulated quarantine crash, got [%v]", err)
	}
	if err := journal.close(); err != nil {
		t.Fatal(err)
	}

	restarted := fixture.openJournal(t, directory)
	defer restarted.close()
	snapshot, err := restarted.reconcile(context.Background(), fixture.target)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.QuarantineGeneration != 1 || snapshot.QuarantineCount != 1 ||
		restarted.quarantineState.BatchSequence != 1 ||
		len(restarted.quarantineMutations) != 1 {
		t.Fatalf("quarantine orphan batch was not integrated exactly once: %+v", restarted.quarantineState)
	}
}

func TestApplyFrostRetainedGroupMutations_EnforcesLifecycleAndRegistryClosure(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	state := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		CurrentPoint: fixture.checkpoint,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	closing := lifecycleMutation(fixture, 3, 1, FrostRetainedGroupClosingMutation, [32]byte{0xb3})
	closed := lifecycleMutation(fixture, 4, 1, FrostRetainedGroupClosedMutation, [32]byte{0xb4})
	registryClosed := lifecycleMutation(fixture, 4, 2, FrostRetainedGroupRegistryClosureMutation, [32]byte{0xb4})
	if err := applyFrostRetainedGroupMutations(
		&state,
		[]FrostRetainedGroupMutation{fixture.admission, closing, closed, registryClosed},
	); err != nil {
		t.Fatal(err)
	}
	if state.SnapshotGeneration != 4 || !state.Wallets[0].RegistryClosed ||
		state.Wallets[0].Lifecycle != FrostRetainedGroupClosed {
		t.Fatalf("unexpected lifecycle state: %+v", state)
	}

	invalid := frostRetainedGroupJournalState{
		Schema:       frostRetainedGroupJournalStateSchema,
		CurrentPoint: fixture.checkpoint,
		Wallets:      []frostRetainedGroupWalletState{},
	}
	directClose := lifecycleMutation(fixture, 3, 1, FrostRetainedGroupClosedMutation, [32]byte{0xc3})
	if err := applyFrostRetainedGroupMutations(
		&invalid,
		[]FrostRetainedGroupMutation{fixture.admission, directClose},
	); err == nil {
		t.Fatal("expected Live -> Closed transition to fail")
	}
}

func lifecycleMutation(
	fixture *journalTestFixture,
	block uint64,
	logIndex uint32,
	kind FrostRetainedGroupMutationKind,
	transactionHash [32]byte,
) FrostRetainedGroupMutation {
	return FrostRetainedGroupMutation{
		Point: FrostRetainedGroupEventPoint{
			BlockNumber:      block,
			BlockHash:        [32]byte{byte(block)},
			TransactionHash:  transactionHash,
			TransactionIndex: 1,
			LogIndex:         logIndex,
		},
		Kind:                kind,
		WalletID:            fixture.walletID,
		WalletPublicKeyHash: fixture.walletPKH,
	}
}

func TestFrostRetainedGroupJournal_RejectsIdentityMismatchAndConcurrentOwner(
	t *testing.T,
) {
	fixture := newJournalTestFixture(t)
	directory := filepath.Join(t.TempDir(), "journal")
	journal := fixture.openJournal(t, directory)
	defer journal.close()
	if _, err := newFrostRetainedGroupJournal(
		directory,
		fixture.manifestHash,
		fixture.manifest,
		fixture.quarantine,
		fixture.source,
		fixture.registry,
		fixture.localOperator,
	); err == nil || !strings.Contains(err.Error(), "already owned") {
		t.Fatalf("expected exclusive-lock failure, got [%v]", err)
	}

	otherDirectory := filepath.Join(t.TempDir(), "identity")
	fixture.source.identity.EndpointFingerprint = [32]byte{0xff}
	if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
		!strings.Contains(err.Error(), "identity differs") {
		t.Fatalf("expected runtime identity mismatch, got [%v]", err)
	}
	if _, err := newFrostRetainedGroupJournal(
		otherDirectory,
		fixture.manifestHash,
		fixture.manifest,
		fixture.quarantine,
		fixture.source,
		fixture.registry,
		fixture.localOperator,
	); err == nil || !strings.Contains(err.Error(), "identity differs") {
		t.Fatalf("expected identity mismatch, got [%v]", err)
	}
}

func TestFrostRetainedGroupJournal_RejectsSymlinkEntry(t *testing.T) {
	fixture := newJournalTestFixture(t)
	directory := filepath.Join(t.TempDir(), "journal")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "target"), filepath.Join(directory, "evil")); err != nil {
		t.Fatal(err)
	}
	if _, err := newFrostRetainedGroupJournal(
		directory,
		fixture.manifestHash,
		fixture.manifest,
		fixture.quarantine,
		fixture.source,
		fixture.registry,
		fixture.localOperator,
	); err == nil || !strings.Contains(err.Error(), "unsafe entry") {
		t.Fatalf("expected symlink rejection, got [%v]", err)
	}
}

func TestPersistFrostRetainedGroupEnvelopeAt_RestrictsFilesToJournal(
	t *testing.T,
) {
	root := t.TempDir()
	directory := filepath.Join(root, "journal")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(root, "outside.json")
	outsideContents := []byte("must not change")
	if err := os.WriteFile(outsidePath, outsideContents, 0600); err != nil {
		t.Fatal(err)
	}

	invalidNames := []string{
		"../outside.json",
		filepath.Join("nested", frostRetainedGroupJournalStateFile),
		outsidePath,
		".",
		frostRetainedGroupJournalLockFile,
		"batch-1.json",
		"batch-00000000000000000000.json",
		"batch-00000000000000000001.json/../../outside.json",
		frostRetainedGroupJournalStateFile + "\x00",
	}
	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := persistFrostRetainedGroupEnvelopeAt(
				directory,
				name,
				map[string]uint64{"generation": 1},
				true,
			)
			if err == nil {
				t.Fatalf("expected unsafe journal file name [%q] to be rejected", name)
			}
			actual, readErr := os.ReadFile(outsidePath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(actual, outsideContents) {
				t.Fatalf("unsafe journal file name [%q] modified an outside file", name)
			}
		})
	}
	if err := persistFrostRetainedGroupEnvelopeAt(
		".",
		frostRetainedGroupJournalStateFile,
		map[string]uint64{"generation": 1},
		true,
	); err == nil {
		t.Fatal("expected a noncanonical journal directory to be rejected")
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected paths left files in the journal directory: [%v]", entries)
	}
}

func TestPersistFrostRetainedGroupEnvelopeAt_PreservesInternalFiles(
	t *testing.T,
) {
	directory := filepath.Join(t.TempDir(), "journal")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	type payload struct {
		Generation uint64 `json:"generation"`
	}

	tests := []struct {
		name    string
		replace bool
		value   uint64
	}{
		{frostRetainedGroupJournalMetadataFile, false, 1},
		{frostRetainedGroupJournalStateFile, true, 2},
		{frostRetainedGroupBatchFileName(1), false, 3},
	}
	for _, test := range tests {
		if err := persistFrostRetainedGroupEnvelopeAt(
			directory,
			test.name,
			payload{Generation: test.value},
			test.replace,
		); err != nil {
			t.Fatalf("cannot persist legitimate journal file [%s]: [%v]", test.name, err)
		}
		var actual payload
		if err := readFrostRetainedGroupEnvelopeAt(
			directory,
			test.name,
			&actual,
		); err != nil {
			t.Fatalf("cannot read legitimate journal file [%s]: [%v]", test.name, err)
		}
		if actual.Generation != test.value {
			t.Fatalf("unexpected journal file [%s] payload: [%+v]", test.name, actual)
		}
		info, err := os.Lstat(filepath.Join(directory, test.name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatalf("legitimate journal file [%s] is not private and regular", test.name)
		}
	}

	if err := persistFrostRetainedGroupEnvelopeAt(
		directory,
		frostRetainedGroupJournalMetadataFile,
		payload{Generation: 4},
		false,
	); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected immutable journal replacement to fail, got [%v]", err)
	}
	if err := persistFrostRetainedGroupEnvelopeAt(
		directory,
		frostRetainedGroupJournalStateFile,
		payload{Generation: 5},
		true,
	); err != nil {
		t.Fatal(err)
	}
	var replaced payload
	if err := readFrostRetainedGroupEnvelopeAt(
		directory,
		frostRetainedGroupJournalStateFile,
		&replaced,
	); err != nil {
		t.Fatal(err)
	}
	if replaced.Generation != 5 {
		t.Fatalf("replaceable journal state was not updated: [%+v]", replaced)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), frostRetainedGroupJournalTempSuffix) {
			t.Fatalf("successful persistence left temporary file [%s]", entry.Name())
		}
	}
}

func TestFrostRetainedGroupJournal_RejectsCorruptOrPublicStoreFiles(t *testing.T) {
	t.Run("quarantine batch checksum", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		fixture.source.mutations = append(
			fixture.source.mutations,
			FrostRetainedGroupMutation{
				Point: FrostRetainedGroupEventPoint{
					BlockNumber:      5,
					BlockHash:        [32]byte{0x05},
					TransactionHash:  [32]byte{0xa5},
					TransactionIndex: 1,
					LogIndex:         1,
				},
				Kind:         FrostRetainedGroupQuarantineMutation,
				WalletID:     fixture.walletID,
				QuarantineID: [32]byte{0x71},
				EvidenceHash: [32]byte{0x72},
				Reason:       "checksum test",
			},
		)
		directory := filepath.Join(t.TempDir(), "journal")
		journal := fixture.openJournal(t, directory)
		if _, err := journal.reconcile(context.Background(), fixture.target); err != nil {
			t.Fatal(err)
		}
		quarantineBatchPath := filepath.Join(
			journal.quarantineDirectory,
			frostRetainedGroupBatchFileName(1),
		)
		if err := journal.close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(quarantineBatchPath)
		if err != nil {
			t.Fatal(err)
		}
		envelope := frostRetainedGroupEnvelope{}
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatal(err)
		}
		envelope.Checksum[0] ^= 0xff
		data, err = json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(quarantineBatchPath, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := newFrostRetainedGroupJournal(
			directory,
			fixture.manifestHash,
			fixture.manifest,
			fixture.quarantine,
			fixture.source,
			fixture.registry,
			fixture.localOperator,
		); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("expected quarantine checksum failure, got [%v]", err)
		}
	})

	t.Run("canonical metadata permissions", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		directory := filepath.Join(t.TempDir(), "journal")
		journal := fixture.openJournal(t, directory)
		metadataPath := filepath.Join(journal.directory, frostRetainedGroupJournalMetadataFile)
		if err := journal.close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(metadataPath, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := newFrostRetainedGroupJournal(
			directory,
			fixture.manifestHash,
			fixture.manifest,
			fixture.quarantine,
			fixture.source,
			fixture.registry,
			fixture.localOperator,
		); err == nil {
			t.Fatal("expected public canonical metadata to be rejected")
		}
	})
}

func TestFrostRetainedGroupJournal_RejectsLocalSessionReconciliationDrift(
	t *testing.T,
) {
	t.Run("missing controlled group", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		fixture.registry.walletCache = make(map[string]*walletCacheValue)
		journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
		defer journal.close()
		if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
			!strings.Contains(err.Error(), "presence differs") {
			t.Fatalf("expected missing local-session failure, got [%v]", err)
		}
	})

	t.Run("nonmember local group", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		fixture.source.mutations[0].OperatorIDs[6] = 52
		journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
		defer journal.close()
		if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
			!strings.Contains(err.Error(), "presence differs") {
			t.Fatalf("expected nonmember local-session failure, got [%v]", err)
		}
	})

	t.Run("terminal group retained locally", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		fixture.source.mutations = append(
			fixture.source.mutations,
			lifecycleMutation(fixture, 3, 1, FrostRetainedGroupClosingMutation, [32]byte{0xb3}),
			lifecycleMutation(fixture, 4, 1, FrostRetainedGroupClosedMutation, [32]byte{0xb4}),
			lifecycleMutation(fixture, 4, 2, FrostRetainedGroupRegistryClosureMutation, [32]byte{0xb4}),
		)
		journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
		defer journal.close()
		if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
			!strings.Contains(err.Error(), "terminal FROST retained group") {
			t.Fatalf("expected terminal local-session failure, got [%v]", err)
		}
	})

	t.Run("operator ordering mismatch", func(t *testing.T) {
		fixture := newJournalTestFixture(t)
		signer := fixture.registry.walletCache["wallet"].signers[0]
		signer.wallet.signingGroupOperators[0], signer.wallet.signingGroupOperators[1] =
			signer.wallet.signingGroupOperators[1], signer.wallet.signingGroupOperators[0]
		journal := fixture.openJournal(t, filepath.Join(t.TempDir(), "journal"))
		defer journal.close()
		if _, err := journal.reconcile(context.Background(), fixture.target); err == nil ||
			!strings.Contains(err.Error(), "operator ordering differs") {
			t.Fatalf("expected operator-ordering failure, got [%v]", err)
		}
	})
}

func TestFrostRetainedGroupInventoryRoot_IsOrderIndependent(t *testing.T) {
	state := frostRetainedGroupJournalState{
		CurrentPoint: FrostPreSignFinality{
			BlockNumber: 120,
			BlockHash:   repeatedJournalTestBytes32(0x12),
		},
		SnapshotGeneration: 9,
		Wallets: []frostRetainedGroupWalletState{
			journalTestInventoryWallet(0x03, 100, FrostRetainedGroupClosed),
			journalTestInventoryWallet(0x01, 51, FrostRetainedGroupLive),
			journalTestInventoryWallet(0x02, 73, FrostRetainedGroupClosing),
		},
	}
	root, count, minimum, maximum, err := frostRetainedGroupInventoryRoot(state)
	if err != nil {
		t.Fatal(err)
	}
	state.Wallets[0], state.Wallets[2] = state.Wallets[2], state.Wallets[0]
	reordered, _, _, _, err := frostRetainedGroupInventoryRoot(state)
	if err != nil {
		t.Fatal(err)
	}
	// Fixed vector produced by computeP2TRFrostWalletGroupInventory at runtime
	// commit cb39161d6 for the same three entries and snapshot point.
	expected := [32]byte{
		0x9d, 0xd9, 0xca, 0x84, 0xc5, 0x20, 0x8c, 0x6b,
		0x73, 0x62, 0xe3, 0x72, 0xb7, 0x94, 0xfc, 0x93,
		0xbc, 0x88, 0xc1, 0x61, 0x8e, 0xa1, 0x0f, 0x74,
		0xe4, 0x13, 0x51, 0xeb, 0x9a, 0x54, 0xfb, 0xe3,
	}
	if root != expected || root != reordered || count != 3 || minimum != 51 || maximum != 100 {
		t.Fatalf("unexpected inventory commitment [%x]", root)
	}
}

func repeatedJournalTestBytes32(value byte) [32]byte {
	result := [32]byte{}
	for index := range result {
		result[index] = value
	}
	return result
}

func journalTestInventoryWallet(
	walletByte byte,
	groupSize int,
	lifecycle FrostRetainedGroupLifecycle,
) frostRetainedGroupWalletState {
	retainedGroupHash := repeatedJournalTestBytes32(0xab)
	retainedGroupHash[len(retainedGroupHash)-1] = walletByte
	creation := FrostRetainedGroupEventPoint{
		BlockNumber:      20,
		BlockHash:        repeatedJournalTestBytes32(0x20),
		TransactionHash:  repeatedJournalTestBytes32(0x21),
		TransactionIndex: 1,
		LogIndex:         4,
	}
	registration := creation
	registration.LogIndex = 5
	lifecyclePoint := registration
	if lifecycle != FrostRetainedGroupLive {
		lifecyclePoint = FrostRetainedGroupEventPoint{
			BlockNumber:      80,
			BlockHash:        repeatedJournalTestBytes32(0x80),
			TransactionHash:  repeatedJournalTestBytes32(0x81),
			TransactionIndex: 1,
			LogIndex:         2,
		}
	}
	wallet := frostRetainedGroupWalletState{
		WalletID:                repeatedJournalTestBytes32(walletByte),
		OperatorIDs:             make([]uint32, groupSize),
		RetainedGroupHash:       retainedGroupHash,
		Lifecycle:               lifecycle,
		CreationPoint:           creation,
		BridgeRegistrationPoint: registration,
		LifecyclePoint:          lifecyclePoint,
		LastBridgePoint:         lifecyclePoint,
	}
	if lifecycle.terminal() {
		wallet.RegistryClosed = true
		wallet.RegistryClosurePoint = lifecyclePoint
		wallet.RegistryClosurePoint.LogIndex = 3
	}
	return wallet
}
