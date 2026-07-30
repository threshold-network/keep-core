package tbtc

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

const frostNativeSignerStateWitnessMaximumPages = 16

func completeFrostInteractiveSigningReadiness(
	interactivePathReady bool,
	interactiveOnly bool,
	nativeExecutionAvailable bool,
	nativeBackendSelected bool,
) bool {
	return interactivePathReady && interactiveOnly &&
		nativeExecutionAvailable && nativeBackendSelected
}

type frostNativeSignerInventoryReader func() (
	*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
	error,
)

type frostNativeSignerStateWitnessProofReader func(
	*frostsigning.NativeTBTCSignerStateWitnessProofRequest,
) (*frostsigning.NativeTBTCSignerStateWitnessProof, error)

type frostNativeSignerStateAnchorTrustHeadReader func() (
	*frostsigning.NativeTBTCSignerStateAnchorTrustHead,
	error,
)

type frostNativeSignerInventoryExpectation struct {
	WalletID         [32]byte
	KeyGroup         string
	Threshold        uint16
	ParticipantCount uint16
	ShareEpoch       uint64
	ParticipantSeats []uint16
}

type frostNativeSignerInventorySnapshot struct {
	Schema                        string
	StoreFingerprint              [32]byte
	StateGeneration               uint64
	StateCommitment               [32]byte
	PreviousStateCommitment       [32]byte
	StateImageDigest              [32]byte
	InventoryCommitment           [32]byte
	WalletCount                   uint64
	KeyPackageCount               uint64
	ExternalRollbackAnchorBound   bool
	TrustCertificateSequence      uint64
	TrustCertificateDigest        [32]byte
	AnchorServiceEpoch            uint64
	CertifiedFloorRevision        uint64
	CertifiedFloorGeneration      uint64
	CurrentAnchorRevision         uint64
	RestartableRevisionHeadroom   uint64
	RestartableGenerationHeadroom uint64
	AnchorRotationWarning         bool
}

type frostNativeSignerInventoryBinding struct {
	storeBinding      *frostDurableSessionStoreBinding
	anchorBinding     *frostNativeSignerAnchorBinding
	readInventory     frostNativeSignerInventoryReader
	readTrustHead     frostNativeSignerStateAnchorTrustHeadReader
	expectedTrustHead frostsigning.NativeTBTCSignerStateAnchorTrustHead

	mutex sync.Mutex
}

func newFrostNativeSignerInventoryBinding(
	storeBinding *frostDurableSessionStoreBinding,
	anchorBinding *frostNativeSignerAnchorBinding,
	readInventory frostNativeSignerInventoryReader,
	readTrustHead frostNativeSignerStateAnchorTrustHeadReader,
	expectedTrustHead *frostsigning.NativeTBTCSignerStateAnchorTrustHead,
) (*frostNativeSignerInventoryBinding, error) {
	if storeBinding == nil || anchorBinding == nil || readInventory == nil ||
		readTrustHead == nil || expectedTrustHead == nil ||
		expectedTrustHead.CertificateSequence == 0 ||
		expectedTrustHead.CertificateDigest == [32]byte{} {
		return nil, fmt.Errorf("FROST native signer inventory dependencies are incomplete")
	}
	if _, err := storeBinding.verify(); err != nil {
		return nil, fmt.Errorf("FROST native signer inventory store is not bound: [%w]", err)
	}
	return &frostNativeSignerInventoryBinding{
		storeBinding:      storeBinding,
		anchorBinding:     anchorBinding,
		readInventory:     readInventory,
		readTrustHead:     readTrustHead,
		expectedTrustHead: *expectedTrustHead,
	}, nil
}

func (binding *frostNativeSignerInventoryBinding) verify(
	ctx context.Context,
	expected []frostNativeSignerInventoryExpectation,
) (*frostNativeSignerInventorySnapshot, error) {
	if binding == nil {
		return nil, fmt.Errorf("FROST native signer inventory binding is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("FROST native signer inventory context is nil")
	}
	binding.mutex.Lock()
	defer binding.mutex.Unlock()

	storeFingerprint, err := binding.storeBinding.verify()
	if err != nil {
		return nil, fmt.Errorf("FROST native signer store binding failed: [%w]", err)
	}
	trustHead, err := binding.readTrustHead()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read native signer state-anchor trust head: [%w]",
			err,
		)
	}
	if trustHead == nil || *trustHead != binding.expectedTrustHead {
		return nil, fmt.Errorf(
			"native signer state-anchor trust head differs from the startup-certified head",
		)
	}
	inventory, err := binding.readInventory()
	if err != nil {
		return nil, fmt.Errorf("cannot read native retained key-package inventory: [%w]", err)
	}
	if inventory == nil || inventory.StoreFingerprint != storeFingerprint {
		return nil, fmt.Errorf("native retained key-package inventory is absent or belongs to another store")
	}
	target := FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        inventory.StoreFingerprint,
		Generation:              inventory.StateGeneration,
		PreviousStateCommitment: inventory.PreviousStateCommitment,
		StateImageDigest:        inventory.StateImageDigest,
		StateCommitment:         inventory.StateCommitment,
	}
	localTip, err := binding.anchorBinding.readTip()
	if err != nil {
		return nil, fmt.Errorf("cannot read native signer state tip: [%w]", err)
	}
	if localTip == nil || frostNativeSignerCheckpointFromTip(*localTip) != target {
		return nil, fmt.Errorf(
			"native signer inventory and state-witness tip identify different checkpoints",
		)
	}
	if trustHead.BindingHash != localTip.AnchorBindingHash ||
		trustHead.ServiceEpoch != localTip.AnchorServiceEpoch ||
		trustHead.CertifiedFloor.ServiceEpoch != binding.anchorBinding.floor.ServiceEpoch ||
		trustHead.CertifiedFloor.Revision != binding.anchorBinding.floor.Revision ||
		trustHead.CertifiedFloor.EventRoot != binding.anchorBinding.floor.EventRoot ||
		trustHead.CertifiedFloor.AcknowledgementDigest !=
			binding.anchorBinding.floor.AcknowledgementDigest ||
		frostNativeSignerCheckpointFromTrustHead(
			trustHead.CertifiedFloor.Checkpoint,
		) != binding.anchorBinding.floor.Checkpoint {
		return nil, fmt.Errorf(
			"native signer trust head, certified floor, and current anchor are inconsistent",
		)
	}
	if err := binding.anchorBinding.VerifyNativeTBTCSignerStateTip(
		ctx,
		*localTip,
	); err != nil {
		return nil, fmt.Errorf(
			"native signer state is not bound to the authenticated external anchor: [%w]",
			err,
		)
	}
	revisionHeadroom, err := binding.anchorBinding.restartableRevisionHeadroom(
		localTip.AnchorServiceEpoch,
		localTip.AnchorRevision,
	)
	if err != nil {
		return nil, err
	}
	generationHeadroom, err :=
		binding.anchorBinding.restartableGenerationHeadroom(
			localTip.Generation,
		)
	if err != nil {
		return nil, err
	}
	if err := verifyFrostNativeSignerInventoryEntries(inventory.Entries, expected); err != nil {
		return nil, err
	}

	keyPackageCount := uint64(0)
	for _, entry := range inventory.Entries {
		keyPackageCount += uint64(len(entry.KeyPackages))
	}
	return &frostNativeSignerInventorySnapshot{
		Schema:                        inventory.Schema,
		StoreFingerprint:              inventory.StoreFingerprint,
		StateGeneration:               inventory.StateGeneration,
		StateCommitment:               inventory.StateCommitment,
		PreviousStateCommitment:       inventory.PreviousStateCommitment,
		StateImageDigest:              inventory.StateImageDigest,
		InventoryCommitment:           inventory.InventoryCommitment,
		WalletCount:                   uint64(len(inventory.Entries)),
		KeyPackageCount:               keyPackageCount,
		ExternalRollbackAnchorBound:   true,
		TrustCertificateSequence:      trustHead.CertificateSequence,
		TrustCertificateDigest:        trustHead.CertificateDigest,
		AnchorServiceEpoch:            localTip.AnchorServiceEpoch,
		CertifiedFloorRevision:        trustHead.CertifiedFloor.Revision,
		CertifiedFloorGeneration:      trustHead.CertifiedFloor.Checkpoint.Generation,
		CurrentAnchorRevision:         localTip.AnchorRevision,
		RestartableRevisionHeadroom:   revisionHeadroom,
		RestartableGenerationHeadroom: generationHeadroom,
		AnchorRotationWarning: frostNativeSignerAnchorRotationWarning(
			minFrostNativeSignerAnchorHeadroom(
				revisionHeadroom,
				generationHeadroom,
			),
		),
	}, nil
}

func frostNativeSignerAnchorRotationWarning(headroom uint64) bool {
	return headroom <= FrostNativeSignerAnchorRotationWarningHeadroom
}

func minFrostNativeSignerAnchorHeadroom(
	revisionHeadroom uint64,
	generationHeadroom uint64,
) uint64 {
	if revisionHeadroom < generationHeadroom {
		return revisionHeadroom
	}
	return generationHeadroom
}

func frostNativeSignerCheckpointFromTrustHead(
	checkpoint frostsigning.NativeTBTCSignerStateAnchorCheckpoint,
) FrostNativeSignerStateWitnessCheckpoint {
	return FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        checkpoint.StoreFingerprint,
		Generation:              checkpoint.Generation,
		PreviousStateCommitment: checkpoint.PreviousStateCommitment,
		StateImageDigest:        checkpoint.StateImageDigest,
		StateCommitment:         checkpoint.StateCommitment,
	}
}

func verifyFrostNativeSignerInventoryEntries(
	actual []frostsigning.NativeTBTCSignerRetainedKeyGroup,
	expected []frostNativeSignerInventoryExpectation,
) error {
	expectedCopy := append([]frostNativeSignerInventoryExpectation{}, expected...)
	for index := range expectedCopy {
		expectedCopy[index].ParticipantSeats = append(
			[]uint16{},
			expectedCopy[index].ParticipantSeats...,
		)
		sort.Slice(expectedCopy[index].ParticipantSeats, func(i, j int) bool {
			return expectedCopy[index].ParticipantSeats[i] < expectedCopy[index].ParticipantSeats[j]
		})
	}
	sort.Slice(expectedCopy, func(i, j int) bool {
		return bytes.Compare(expectedCopy[i].WalletID[:], expectedCopy[j].WalletID[:]) < 0
	})
	if len(actual) != len(expectedCopy) {
		return fmt.Errorf(
			"native retained key-group count [%d] differs from canonical local membership count [%d]",
			len(actual), len(expectedCopy),
		)
	}
	for index := range expectedCopy {
		actualEntry := actual[index]
		expectedEntry := expectedCopy[index]
		if expectedEntry.KeyGroup == "" {
			return fmt.Errorf(
				"canonical local signer key-group binding is empty",
			)
		}
		if actualEntry.KeyGroup != expectedEntry.KeyGroup {
			return fmt.Errorf(
				"native retained key group differs from exact local signer material",
			)
		}
		if actualEntry.WalletID != expectedEntry.WalletID ||
			actualEntry.Threshold != expectedEntry.Threshold ||
			actualEntry.ParticipantCount != expectedEntry.ParticipantCount ||
			actualEntry.ShareEpoch != expectedEntry.ShareEpoch ||
			len(actualEntry.KeyPackages) != len(expectedEntry.ParticipantSeats) {
			return fmt.Errorf("native retained key group differs from canonical wallet membership or epoch")
		}
		for packageIndex, expectedSeat := range expectedEntry.ParticipantSeats {
			if actualEntry.KeyPackages[packageIndex].ParticipantSeat != expectedSeat {
				return fmt.Errorf("native retained key-package seats differ from canonical local seats")
			}
		}
	}
	return nil
}

type frostProductionSignerReadinessSnapshot struct {
	Journal                 *frostRetainedGroupJournalSnapshot
	Inventory               *frostNativeSignerInventorySnapshot
	InteractiveSigningReady bool

	inventoryExpectations []frostNativeSignerInventoryExpectation
	registryRevision      uint64
}

type frostProductionSignerReadinessVerifier interface {
	verifyFrostProductionSignerReadiness(
		context.Context,
		FrostPreSignFinality,
	) (*frostProductionSignerReadinessSnapshot, error)
	verifyFrostProductionSignerReadinessUnchanged(
		context.Context,
		*frostProductionSignerReadinessSnapshot,
	) error
}

func cloneFrostProductionSignerReadinessSnapshot(
	snapshot *frostProductionSignerReadinessSnapshot,
) *frostProductionSignerReadinessSnapshot {
	if snapshot == nil {
		return nil
	}
	result := *snapshot
	if snapshot.Journal != nil {
		journal := *snapshot.Journal
		journal.ActiveQuarantines = append(
			[]frostRetainedGroupActiveQuarantine{},
			snapshot.Journal.ActiveQuarantines...,
		)
		result.Journal = &journal
	}
	if snapshot.Inventory != nil {
		inventory := *snapshot.Inventory
		result.Inventory = &inventory
	}
	result.inventoryExpectations = make(
		[]frostNativeSignerInventoryExpectation,
		len(snapshot.inventoryExpectations),
	)
	for i, expectation := range snapshot.inventoryExpectations {
		result.inventoryExpectations[i] = expectation
		result.inventoryExpectations[i].ParticipantSeats = append(
			[]uint16{},
			expectation.ParticipantSeats...,
		)
	}
	return &result
}

type frostProductionSignerReadiness struct {
	interactiveSigningReady func() bool
	journal                 *frostRetainedGroupJournal
	inventoryBinding        *frostNativeSignerInventoryBinding
}

func newFrostProductionSignerReadiness(
	interactiveSigningReady func() bool,
	journal *frostRetainedGroupJournal,
	inventoryBinding *frostNativeSignerInventoryBinding,
) (*frostProductionSignerReadiness, error) {
	if interactiveSigningReady == nil || journal == nil || inventoryBinding == nil {
		return nil, fmt.Errorf("FROST production signer readiness dependencies are incomplete")
	}
	return &frostProductionSignerReadiness{
		interactiveSigningReady: interactiveSigningReady,
		journal:                 journal,
		inventoryBinding:        inventoryBinding,
	}, nil
}

func (readiness *frostProductionSignerReadiness) verifyFrostProductionSignerReadiness(
	ctx context.Context,
	point FrostPreSignFinality,
) (*frostProductionSignerReadinessSnapshot, error) {
	if readiness == nil || readiness.interactiveSigningReady == nil ||
		!readiness.interactiveSigningReady() {
		return nil, fmt.Errorf("interactive FROST signing engine is not ready")
	}
	journalSnapshot, err := readiness.journal.reconcile(ctx, point)
	if err != nil {
		return nil, fmt.Errorf("cannot reconcile canonical FROST retained groups: [%w]", err)
	}
	expected, registryRevision, err :=
		readiness.journal.nativeSignerInventoryExpectations(ctx, point)
	if err != nil {
		return nil, err
	}
	inventory, err := readiness.verifyStableFrostProductionSignerInventory(
		ctx,
		expected,
	)
	if err != nil {
		return nil, err
	}
	if !readiness.journal.walletRegistry.frostReadinessRevisionMatches(
		registryRevision,
	) {
		return nil, fmt.Errorf(
			"local FROST signer registry changed during readiness reconciliation",
		)
	}
	if !readiness.interactiveSigningReady() {
		return nil, fmt.Errorf("interactive FROST signing engine became unavailable during reconciliation")
	}
	return &frostProductionSignerReadinessSnapshot{
		Journal:                 journalSnapshot,
		Inventory:               inventory,
		InteractiveSigningReady: true,
		inventoryExpectations:   expected,
		registryRevision:        registryRevision,
	}, nil
}

// verifyFrostProductionSignerReadinessUnchanged revalidates cached readiness
// and discards the live native signer inventory it read. Callers that export
// native signer facts into a signed statement must use
// revalidateFrostProductionSignerReadinessInventory instead and publish the
// returned live snapshot: the cached one is only pinned on the strict fields.
func (readiness *frostProductionSignerReadiness) verifyFrostProductionSignerReadinessUnchanged(
	ctx context.Context,
	expected *frostProductionSignerReadinessSnapshot,
) error {
	_, err := readiness.revalidateFrostProductionSignerReadinessInventory(
		ctx,
		expected,
	)
	return err
}

// revalidateFrostProductionSignerReadinessInventory revalidates cached
// readiness and returns the live native signer inventory it read.
//
// The returned snapshot is not the cached one. Everything
// verifyFrostNativeSignerInventoryUnchanged pins strictly - store identity,
// retained key material, trust head, certified floor and the anchor rotation
// warning - is guaranteed equal to the cached value or this returns an error.
// The state checkpoint, the anchor revision and both restartable headrooms are
// only held to a monotone advance, so on those the returned live values and
// the cached ones can legitimately differ.
func (readiness *frostProductionSignerReadiness) revalidateFrostProductionSignerReadinessInventory(
	ctx context.Context,
	expected *frostProductionSignerReadinessSnapshot,
) (*frostNativeSignerInventorySnapshot, error) {
	if readiness == nil || readiness.interactiveSigningReady == nil ||
		readiness.journal == nil || readiness.inventoryBinding == nil ||
		ctx == nil || expected == nil || expected.Journal == nil ||
		expected.Inventory == nil ||
		!expected.InteractiveSigningReady {
		return nil, fmt.Errorf(
			"cached FROST production signer readiness is incomplete",
		)
	}
	if !readiness.interactiveSigningReady() {
		return nil, fmt.Errorf("interactive FROST signing engine is not ready")
	}
	if err := readiness.verifyFrostRetainedGroupJournalStampUnchanged(
		ctx,
		expected.Journal,
	); err != nil {
		return nil, err
	}
	if !readiness.journal.walletRegistry.frostReadinessRevisionMatches(
		expected.registryRevision,
	) {
		return nil, fmt.Errorf(
			"local FROST signer registry changed since readiness reconciliation",
		)
	}
	inventory, err := readiness.verifyStableFrostProductionSignerInventory(
		ctx,
		expected.inventoryExpectations,
	)
	if err != nil {
		return nil, err
	}
	if err := verifyFrostNativeSignerInventoryUnchanged(
		expected.Inventory,
		inventory,
	); err != nil {
		return nil, err
	}
	if !readiness.journal.walletRegistry.frostReadinessRevisionMatches(
		expected.registryRevision,
	) {
		return nil, fmt.Errorf(
			"local FROST signer registry changed during readiness revalidation",
		)
	}
	if !readiness.interactiveSigningReady() {
		return nil, fmt.Errorf(
			"interactive FROST signing engine became unavailable during readiness revalidation",
		)
	}
	if err := readiness.verifyFrostRetainedGroupJournalStampUnchanged(
		ctx,
		expected.Journal,
	); err != nil {
		return nil, err
	}
	return inventory, nil
}

// verifyFrostNativeSignerInventoryUnchanged separates the native signer facts
// that must be byte-identical for the whole authorized signing window from the
// ones the window itself legitimately advances.
//
// The authorized window is exactly what mutates the Rust store: the anchor
// admission controller reserves the durable cost of every anchored call up
// front, the engine persists a consumption marker before releasing each share,
// and the inventory endpoint reports the live tip. Requiring the whole snapshot
// to compare equal therefore turns each session's own persistence into
// "readiness changed", and because the pre-sign authorization monitor latches
// the first revalidation error permanently, that aborts the very signing it is
// guarding.
//
// Identity, trust and key material stay strictly pinned. The state checkpoint,
// the anchor revision and the derived restartable headrooms may only advance
// monotonically, and only while the anchor stays out of its rotation-warning
// band - the same threshold below which the admission controller refuses new
// work. A rollback, an equal-generation fork, a different store or trust head,
// a changed key-package commitment, or an advance that consumes the warning
// headroom all still fail closed.
func verifyFrostNativeSignerInventoryUnchanged(
	expected *frostNativeSignerInventorySnapshot,
	actual *frostNativeSignerInventorySnapshot,
) error {
	if expected == nil || actual == nil {
		return fmt.Errorf("native signer readiness snapshot is nil")
	}
	if actual.Schema != expected.Schema ||
		actual.StoreFingerprint != expected.StoreFingerprint ||
		actual.InventoryCommitment != expected.InventoryCommitment ||
		actual.WalletCount != expected.WalletCount ||
		actual.KeyPackageCount != expected.KeyPackageCount ||
		actual.ExternalRollbackAnchorBound !=
			expected.ExternalRollbackAnchorBound ||
		actual.TrustCertificateSequence != expected.TrustCertificateSequence ||
		actual.TrustCertificateDigest != expected.TrustCertificateDigest ||
		actual.AnchorServiceEpoch != expected.AnchorServiceEpoch ||
		actual.CertifiedFloorRevision != expected.CertifiedFloorRevision ||
		actual.CertifiedFloorGeneration != expected.CertifiedFloorGeneration ||
		actual.AnchorRotationWarning != expected.AnchorRotationWarning {
		return fmt.Errorf(
			"native signer identity, trust head, or retained key material changed since readiness reconciliation",
		)
	}
	if actual.StateGeneration < expected.StateGeneration ||
		actual.CurrentAnchorRevision < expected.CurrentAnchorRevision ||
		actual.RestartableGenerationHeadroom >
			expected.RestartableGenerationHeadroom ||
		actual.RestartableRevisionHeadroom >
			expected.RestartableRevisionHeadroom {
		return fmt.Errorf(
			"native signer state or anchor revision rolled back since readiness reconciliation",
		)
	}
	// A durable advance always moves the state commitment chain forward, so an
	// unchanged generation carrying a different commitment is a fork, not the
	// session's own progress.
	if actual.StateGeneration == expected.StateGeneration &&
		(actual.StateCommitment != expected.StateCommitment ||
			actual.PreviousStateCommitment != expected.PreviousStateCommitment ||
			actual.StateImageDigest != expected.StateImageDigest) {
		return fmt.Errorf(
			"native signer state forked at an unchanged generation since readiness reconciliation",
		)
	}
	return nil
}

// frostRetainedGroupJournalStampRetryInterval bounds how often a readiness
// revalidation retries the canonical journal mutex while another workflow
// holds it.
const frostRetainedGroupJournalStampRetryInterval = 5 * time.Millisecond

// lockFrostRetainedGroupJournalStamp waits for the canonical journal mutex
// under the caller's context instead of treating contention as a failure. The
// mutex is held for an entire reconcile, including paginated reads against the
// independent history source, so multi-second holds are routine and any other
// wallet's authorization - or the activation-handshake exporter's background
// reconciliation - can hold it. Contention is not evidence that anything
// changed, and the pre-sign authorization monitor latches the first
// revalidation error permanently, so returning one here would cancel a
// perfectly valid signing session. Only a genuine stamp change, or the
// caller's own context ending, fails.
func lockFrostRetainedGroupJournalStamp(
	ctx context.Context,
	journal *frostRetainedGroupJournal,
) error {
	if journal.mutex.TryLock() {
		return nil
	}
	ticker := time.NewTicker(frostRetainedGroupJournalStampRetryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(
				"canonical FROST retained-group journal stayed busy until the readiness revalidation deadline: [%w]",
				ctx.Err(),
			)
		case <-ticker.C:
			if journal.mutex.TryLock() {
				return nil
			}
		}
	}
}

func (readiness *frostProductionSignerReadiness) verifyFrostRetainedGroupJournalStampUnchanged(
	ctx context.Context,
	expected *frostRetainedGroupJournalSnapshot,
) error {
	if readiness == nil || readiness.journal == nil || expected == nil ||
		ctx == nil {
		return fmt.Errorf(
			"cached FROST retained-group journal readiness is incomplete",
		)
	}
	journal := readiness.journal
	if err := lockFrostRetainedGroupJournalStamp(ctx, journal); err != nil {
		return err
	}
	defer journal.mutex.Unlock()

	if !expected.Complete ||
		expected.Schema != frostRetainedGroupJournalSnapshotSchema ||
		journal.closed ||
		journal.metadata.Schema != frostRetainedGroupJournalMetadataSchema ||
		journal.metadata.BindingHash != expected.BindingHash ||
		journal.metadata.StoreID != expected.StoreID ||
		journal.metadata.StoreFingerprint != expected.StoreFingerprint ||
		journal.metadata.ClusterFingerprint != expected.ClusterFingerprint ||
		journal.state.Schema != frostRetainedGroupJournalStateSchema ||
		journal.state.BindingHash != expected.BindingHash ||
		journal.state.CurrentPoint != expected.CurrentPoint ||
		journal.state.SnapshotGeneration != expected.SnapshotGeneration ||
		journal.state.BatchRoot != expected.BatchRoot ||
		journal.state.InventoryRoot != expected.InventoryRoot ||
		journal.quarantineMetadata.Schema !=
			frostRetainedGroupQuarantineMetadataSchema ||
		journal.quarantineMetadata.BindingHash != expected.BindingHash ||
		journal.quarantineMetadata.ProtocolID != expected.QuarantineProtocolID ||
		journal.quarantineMetadata.StoreID != expected.QuarantineStoreID ||
		journal.quarantineMetadata.StoreFingerprint !=
			expected.QuarantineStoreFingerprint ||
		journal.quarantineMetadata.ClusterFingerprint !=
			expected.QuarantineClusterFingerprint ||
		journal.quarantineMinimumGeneration !=
			expected.QuarantineMinimumGeneration ||
		journal.quarantineState.Schema !=
			frostRetainedGroupQuarantineStateSchema ||
		journal.quarantineState.BindingHash != expected.BindingHash ||
		journal.quarantineState.CurrentPoint != expected.CurrentPoint ||
		journal.quarantineState.Generation != expected.QuarantineGeneration ||
		journal.quarantineState.Root != expected.QuarantineRoot ||
		journal.quarantineState.ActiveRoot != expected.QuarantineActiveRoot ||
		journal.quarantineState.TombstoneRoot !=
			expected.QuarantineTombstoneRoot ||
		frostRetainedGroupActiveQuarantineCount(journal.quarantineState) !=
			expected.QuarantineCount ||
		uint64(len(journal.quarantineState.Tombstones)) !=
			expected.QuarantineTombstoneCount ||
		journal.checkpointPolicy.MinimumSequence !=
			expected.CheckpointMinimumSequence ||
		journal.checkpointPolicy.PredecessorHash !=
			expected.CheckpointPredecessorHash ||
		journal.checkpointState.Schema !=
			frostRetainedGroupCheckpointStateSchema ||
		journal.checkpointState.BindingHash != expected.BindingHash ||
		journal.checkpointState.Point != expected.CurrentPoint ||
		journal.checkpointState.Sequence != expected.CheckpointSequence ||
		journal.checkpointState.CertificateHash !=
			expected.CheckpointCertificateHash ||
		journal.checkpointState.HistoryRoot !=
			expected.CheckpointHistoryRoot ||
		journal.checkpointState.CanonicalGeneration !=
			expected.SnapshotGeneration ||
		journal.checkpointState.CanonicalInventoryRoot !=
			expected.InventoryRoot ||
		journal.checkpointState.QuarantineGeneration !=
			expected.QuarantineGeneration ||
		journal.checkpointState.QuarantineEventRoot !=
			expected.QuarantineRoot ||
		journal.checkpointState.QuarantineActiveRoot !=
			expected.QuarantineActiveRoot ||
		journal.checkpointState.QuarantineTombstoneRoot !=
			expected.QuarantineTombstoneRoot {
		return fmt.Errorf(
			"canonical, quarantine, or checkpoint FROST retained-group journal changed since readiness reconciliation",
		)
	}
	return nil
}

func (readiness *frostProductionSignerReadiness) verifyStableFrostProductionSignerInventory(
	ctx context.Context,
	expected []frostNativeSignerInventoryExpectation,
) (*frostNativeSignerInventorySnapshot, error) {
	firstInventory, err := readiness.inventoryBinding.verify(ctx, expected)
	if err != nil {
		return nil, err
	}
	secondInventory, err := readiness.inventoryBinding.verify(ctx, expected)
	if err != nil {
		return nil, err
	}
	// The two reads are not adjacent: each one verifies the state tip against
	// the authenticated external anchor, which is a full round trip to the
	// anchor service. An authorized signing session persisting its own
	// consumption marker routinely lands in that window, so the same
	// advance-versus-change discipline the cached revalidation uses has to
	// apply here too. Comparing the whole snapshot by value instead would let
	// a session's own durable advance fail its guard - and the pre-sign
	// authorization monitor latches the first such error permanently.
	if err := verifyFrostNativeSignerInventoryUnchanged(
		firstInventory,
		secondInventory,
	); err != nil {
		return nil, fmt.Errorf(
			"native signer state changed during readiness verification: [%w]",
			err,
		)
	}
	if err := validateFrostNativeSignerAnchorReadinessHeadroom(
		secondInventory,
	); err != nil {
		return nil, err
	}
	return secondInventory, nil
}

func validateFrostNativeSignerAnchorReadinessHeadroom(
	inventory *frostNativeSignerInventorySnapshot,
) error {
	if inventory == nil ||
		inventory.CurrentAnchorRevision < inventory.CertifiedFloorRevision ||
		inventory.CurrentAnchorRevision-inventory.CertifiedFloorRevision+
			inventory.RestartableRevisionHeadroom !=
			FrostNativeSignerAnchorMaximumHistoryEvents ||
		inventory.StateGeneration <
			inventory.CertifiedFloorGeneration ||
		inventory.StateGeneration-
			inventory.CertifiedFloorGeneration+
			inventory.RestartableGenerationHeadroom !=
			FrostNativeSignerAnchorMaximumHistoryProofEntries ||
		inventory.AnchorRotationWarning !=
			frostNativeSignerAnchorRotationWarning(
				minFrostNativeSignerAnchorHeadroom(
					inventory.RestartableRevisionHeadroom,
					inventory.RestartableGenerationHeadroom,
				),
			) {
		return fmt.Errorf(
			"native signer certified anchor revision or generation headroom is inconsistent",
		)
	}
	if inventory.RestartableRevisionHeadroom == 0 ||
		inventory.RestartableGenerationHeadroom == 0 {
		return fmt.Errorf(
			"native signer certified anchor revision or generation window is exhausted; offline anchor rotation is required",
		)
	}
	return nil
}

func (journal *frostRetainedGroupJournal) nativeSignerInventoryExpectations(
	ctx context.Context,
	point FrostPreSignFinality,
) ([]frostNativeSignerInventoryExpectation, uint64, error) {
	if journal == nil || ctx == nil {
		return nil, 0, fmt.Errorf("FROST retained-group journal expectation context is invalid")
	}
	localOperatorID, err := journal.source.ResolveOperatorID(ctx, journal.operatorAddress, point)
	if err != nil || localOperatorID == 0 {
		return nil, 0, fmt.Errorf("cannot resolve local operator for native signer inventory: [%w]", err)
	}
	sessions, retainedKeyGroups, registryRevision, err :=
		journal.walletRegistry.frostReadinessMaterialSnapshot()
	if err != nil {
		return nil, 0, fmt.Errorf("cannot resolve local FROST signer material: [%w]", err)
	}
	sessionsByWallet := make(map[[32]byte]frostLocalSessionSnapshot, len(sessions))
	for _, session := range sessions {
		if _, exists := sessionsByWallet[session.WalletID]; exists {
			return nil, 0, fmt.Errorf("duplicate FROST local session wallet ID")
		}
		sessionsByWallet[session.WalletID] = session
	}

	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	if journal.closed || journal.state.CurrentPoint != point {
		return nil, 0, fmt.Errorf("canonical retained-group journal moved during native signer reconciliation")
	}
	expected := make([]frostNativeSignerInventoryExpectation, 0)
	for _, wallet := range journal.state.Wallets {
		seats := make([]uint16, 0)
		for index, operatorID := range wallet.OperatorIDs {
			if chain.OperatorID(operatorID) == localOperatorID {
				seats = append(seats, uint16(index+1))
			}
		}
		if len(seats) == 0 {
			continue
		}
		keyGroup, hasBinding := retainedKeyGroups[wallet.WalletID]
		if !hasBinding || keyGroup == "" {
			return nil, 0, fmt.Errorf(
				"canonical FROST retained group has no durable local key-group binding",
			)
		}
		session, hasSession := sessionsByWallet[wallet.WalletID]
		if wallet.Lifecycle.terminal() {
			if hasSession {
				return nil, 0, fmt.Errorf(
					"canonical terminal FROST retained group still has an active local session",
				)
			}
		} else {
			if !hasSession || session.KeyGroup == "" {
				return nil, 0, fmt.Errorf(
					"canonical live FROST retained group has no resolved local key-group handle",
				)
			}
			if session.KeyGroup != keyGroup {
				return nil, 0, fmt.Errorf(
					"active FROST key-group handle differs from durable local binding",
				)
			}
		}
		expected = append(expected, frostNativeSignerInventoryExpectation{
			WalletID:         wallet.WalletID,
			KeyGroup:         keyGroup,
			Threshold:        frostPreSignAuthorizationThreshold,
			ParticipantCount: uint16(len(wallet.OperatorIDs)),
			ShareEpoch:       0,
			ParticipantSeats: seats,
		})
	}
	return expected, registryRevision, nil
}
