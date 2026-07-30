//go:build frost_native

package tbtc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/chain"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type orphanedDKGSnapshotTestChain struct {
	state      DKGState
	registered map[[32]byte]bool
	points     []FrostPreSignFinality
}

func (chain *orphanedDKGSnapshotTestChain) FrostDKGRetirementSnapshot(
	_ context.Context,
	point FrostPreSignFinality,
	walletIDs [][32]byte,
) (*FrostDKGRetirementSnapshot, error) {
	chain.points = append(chain.points, point)
	registered := make(map[[32]byte]bool, len(walletIDs))
	for _, walletID := range walletIDs {
		registered[walletID] = chain.registered[walletID]
	}
	return &FrostDKGRetirementSnapshot{
		Point:             point,
		State:             chain.state,
		RegisteredWallets: registered,
	}, nil
}

type orphanedDKGRetirementTestEngine struct {
	retired []string
}

func (engine *orphanedDKGRetirementTestEngine) RetireDistributedDKGKeyPackages(
	keyGroup string,
) error {
	engine.retired = append(engine.retired, keyGroup)
	return nil
}

func TestFrostOrphanedDKGReconcilerRetiresNativeOnlyOrphan(t *testing.T) {
	walletID := [32]byte{1}
	const keyGroup = "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	engine := &orphanedDKGRetirementTestEngine{}
	snapshotChain := &orphanedDKGSnapshotTestChain{
		state:      Idle,
		registered: make(map[[32]byte]bool),
	}
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain:  snapshotChain,
		walletRegistry: orphanedDKGTestWalletRegistry(t, 90),
		anchorAdmission: &frostNativeSignerAnchorAdmissionController{
			readHeadroom: func(
				context.Context,
			) (frostNativeSignerAnchorCapacity, error) {
				return frostNativeSignerAnchorCapacity{
					Revisions: FrostNativeSignerAnchorRotationWarningHeadroom + 10,
					Generations: FrostNativeSignerAnchorRotationWarningHeadroom +
						10,
				}, nil
			},
		},
		retirementEngine: engine,
		readInventory: func() (
			*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
			error,
		) {
			return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
				Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
					WalletID:         walletID,
					KeyGroup:         keyGroup,
					ParticipantCount: 3,
				}},
			}, nil
		},
	}

	target := orphanedDKGTestPoint()
	if err := reconciler.reconcile(
		context.Background(),
		target,
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(snapshotChain.points) != 1 || snapshotChain.points[0] != target {
		t.Fatalf(
			"retirement was not pinned to the journal point: [%+v]",
			snapshotChain.points,
		)
	}
	if len(engine.retired) != 1 || engine.retired[0] != keyGroup {
		t.Fatalf("unexpected retired key groups: [%v]", engine.retired)
	}
	if reconciler.anchorAdmission.reserved !=
		(frostNativeSignerAnchorCapacity{}) {
		t.Fatalf(
			"retirement reservation was not released: [%+v]",
			reconciler.anchorAdmission.reserved,
		)
	}
}

func TestFrostOrphanedDKGReconcilerPreservesCanonicalAndRegistered(t *testing.T) {
	canonicalWalletID := [32]byte{1}
	registeredWalletID := [32]byte{2}
	engine := &orphanedDKGRetirementTestEngine{}
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain: &orphanedDKGSnapshotTestChain{
			state:      Idle,
			registered: map[[32]byte]bool{registeredWalletID: true},
		},
		walletRegistry:   orphanedDKGTestWalletRegistry(t, 90),
		anchorAdmission:  &frostNativeSignerAnchorAdmissionController{},
		retirementEngine: engine,
		readInventory: func() (
			*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
			error,
		) {
			return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
				Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{
					{
						WalletID: canonicalWalletID,
						KeyGroup: "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
					},
					{
						WalletID: registeredWalletID,
						KeyGroup: "0379be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
					},
				},
			}, nil
		},
	}

	if err := reconciler.reconcile(
		context.Background(),
		orphanedDKGTestPoint(),
		map[[32]byte]struct{}{canonicalWalletID: {}},
	); err != nil {
		t.Fatal(err)
	}
	if len(engine.retired) != 0 {
		t.Fatalf("canonical or registered key groups were retired: [%v]", engine.retired)
	}
}

func TestFrostOrphanedDKGReconcilerPreservesAwaitingResultMaterial(t *testing.T) {
	walletID := [32]byte{3}
	engine := &orphanedDKGRetirementTestEngine{}
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain: &orphanedDKGSnapshotTestChain{
			state:      AwaitingResult,
			registered: make(map[[32]byte]bool),
		},
		walletRegistry:   orphanedDKGTestWalletRegistry(t, 90),
		anchorAdmission:  &frostNativeSignerAnchorAdmissionController{},
		retirementEngine: engine,
		readInventory: func() (
			*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
			error,
		) {
			return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
				Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
					WalletID: walletID,
					KeyGroup: "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
				}},
			}, nil
		},
	}

	if err := reconciler.reconcile(
		context.Background(),
		orphanedDKGTestPoint(),
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(engine.retired) != 0 {
		t.Fatalf("AwaitingResult DKG material was retired: [%v]", engine.retired)
	}
}

func TestFrostOrphanedDKGReconcilerPreservesMaterialDuringChallenge(
	t *testing.T,
) {
	walletID := orphanedDKGTestWalletID(t)
	const keyGroup = "0379be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	engine := &orphanedDKGRetirementTestEngine{}
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain: &orphanedDKGSnapshotTestChain{
			state:      Challenge,
			registered: make(map[[32]byte]bool),
		},
		walletRegistry:   orphanedDKGTestWalletRegistry(t, 90),
		anchorAdmission:  orphanedDKGTestAnchorAdmission(),
		retirementEngine: engine,
		readInventory: func() (
			*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
			error,
		) {
			return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
				Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
					WalletID:         walletID,
					KeyGroup:         keyGroup,
					ParticipantCount: 3,
				}},
			}, nil
		},
	}

	if err := reconciler.reconcile(
		context.Background(),
		orphanedDKGTestPoint(),
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(engine.retired) != 0 {
		t.Fatalf("unresolved DKG material was retired: [%v]", engine.retired)
	}
}

func TestFrostOrphanedDKGReconcilerPreservesEveryKeyEncodingDuringChallenge(
	t *testing.T,
) {
	walletID := orphanedDKGTestWalletID(t)
	testCases := map[string]string{
		"odd compressed key": "0379be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
		"legacy x-only key":  "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
	}

	for name, keyGroup := range testCases {
		t.Run(name, func(t *testing.T) {
			engine := &orphanedDKGRetirementTestEngine{}
			reconciler := &frostOrphanedDKGReconciler{
				snapshotChain: &orphanedDKGSnapshotTestChain{
					state:      Challenge,
					registered: make(map[[32]byte]bool),
				},
				walletRegistry:   orphanedDKGTestWalletRegistry(t, 90),
				anchorAdmission:  orphanedDKGTestAnchorAdmission(),
				retirementEngine: engine,
				readInventory: func() (
					*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
					error,
				) {
					return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
						Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
							WalletID:         walletID,
							KeyGroup:         keyGroup,
							ParticipantCount: 3,
						}},
					}, nil
				},
			}

			if err := reconciler.reconcile(
				context.Background(),
				orphanedDKGTestPoint(),
				map[[32]byte]struct{}{},
			); err != nil {
				t.Fatal(err)
			}
			if len(engine.retired) != 0 {
				t.Fatalf(
					"pending DKG material [%s] was retired: [%v]",
					keyGroup,
					engine.retired,
				)
			}
		})
	}
}

func TestFrostOrphanedDKGReconcilerPreservesMaterialBeforeAttemptFinality(
	t *testing.T,
) {
	walletID := [32]byte{4}
	engine := &orphanedDKGRetirementTestEngine{}
	snapshotChain := &orphanedDKGSnapshotTestChain{
		state:      Idle,
		registered: make(map[[32]byte]bool),
	}
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain:    snapshotChain,
		walletRegistry:   orphanedDKGTestWalletRegistry(t, 101),
		anchorAdmission:  orphanedDKGTestAnchorAdmission(),
		retirementEngine: engine,
		readInventory: func() (
			*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
			error,
		) {
			return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
				Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
					WalletID: walletID,
					KeyGroup: "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
				}},
			}, nil
		},
	}

	if err := reconciler.reconcile(
		context.Background(),
		orphanedDKGTestPoint(),
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(snapshotChain.points) != 0 {
		t.Fatalf(
			"pre-attempt finalized point triggered a retirement snapshot read: [%+v]",
			snapshotChain.points,
		)
	}
	if len(engine.retired) != 0 {
		t.Fatalf(
			"in-flight DKG material was retired from a pre-attempt snapshot: [%v]",
			engine.retired,
		)
	}
}

func TestFrostOrphanedDKGReconcilerBackfillsLegacyInventoryBoundary(
	t *testing.T,
) {
	walletID := [32]byte{5}
	const keyGroup = "0379be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
	engine := &orphanedDKGRetirementTestEngine{}
	snapshotChain := &orphanedDKGSnapshotTestChain{
		state:      Idle,
		registered: make(map[[32]byte]bool),
	}
	persistenceHandle := &mockPersistenceHandle{}
	walletRegistry := &walletRegistry{
		walletCache:                  make(map[string]*walletCacheValue),
		walletStorage:                newWalletStorage(persistenceHandle),
		frostDKGRetirementBoundaries: make(map[string]uint64),
	}
	currentBlockReads := 0
	operationOrder := make([]string, 0)
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain:    snapshotChain,
		walletRegistry:   walletRegistry,
		anchorAdmission:  orphanedDKGTestAnchorAdmission(),
		retirementEngine: engine,
		readCurrentBlock: func() (uint64, error) {
			currentBlockReads++
			operationOrder = append(operationOrder, "head")
			return 120, nil
		},
		readInventory: func() (
			*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
			error,
		) {
			operationOrder = append(operationOrder, "inventory")
			return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
				Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
					WalletID: walletID,
					KeyGroup: keyGroup,
				}},
			}, nil
		},
	}

	if err := reconciler.reconcile(
		context.Background(),
		orphanedDKGTestPoint(),
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(snapshotChain.points) != 0 || len(engine.retired) != 0 {
		t.Fatal("legacy material was evaluated before migration finality")
	}
	if currentBlockReads != 1 || len(persistenceHandle.saved) != 1 {
		t.Fatalf(
			"legacy migration boundary was not persisted exactly once: reads=%d saves=%d",
			currentBlockReads,
			len(persistenceHandle.saved),
		)
	}
	if len(operationOrder) != 2 ||
		operationOrder[0] != "inventory" ||
		operationOrder[1] != "head" {
		t.Fatalf(
			"migration boundary was not observed after inventory: [%v]",
			operationOrder,
		)
	}

	reopened, err := newWalletRegistry(
		persistenceHandle,
		Connect().CalculateWalletID,
	)
	if err != nil {
		t.Fatal(err)
	}
	reconciler.walletRegistry = reopened
	beforeBoundary := FrostPreSignFinality{
		BlockNumber: 119,
		BlockHash:   [32]byte{0xbb},
	}
	if err := reconciler.reconcile(
		context.Background(),
		beforeBoundary,
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(snapshotChain.points) != 0 || len(engine.retired) != 0 {
		t.Fatal("legacy material was evaluated before the migration boundary")
	}

	atBoundary := FrostPreSignFinality{
		BlockNumber: 120,
		BlockHash:   [32]byte{0xcc},
	}
	if err := reconciler.reconcile(
		context.Background(),
		atBoundary,
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(snapshotChain.points) != 0 || len(engine.retired) != 0 {
		t.Fatal("legacy material was evaluated at the migration boundary")
	}

	afterBoundary := FrostPreSignFinality{
		BlockNumber: 121,
		BlockHash:   [32]byte{0xdd},
	}
	if err := reconciler.reconcile(
		context.Background(),
		afterBoundary,
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(snapshotChain.points) != 1 ||
		snapshotChain.points[0] != afterBoundary ||
		len(engine.retired) != 1 ||
		engine.retired[0] != keyGroup {
		t.Fatalf(
			"legacy orphan was not retired after migration finality: points=%+v retired=%v",
			snapshotChain.points,
			engine.retired,
		)
	}
	if currentBlockReads != 1 {
		t.Fatalf("migration head was read again: [%d]", currentBlockReads)
	}
}

// orphanedDKGOrderedTestEngine and orphanedDKGOrderedTestPersistence share one
// operation log so a test can pin the ORDER of the two durable mutations a
// retirement performs. Native key packages must go first: a crash between them
// leaves a state a later pass can finish (no native material, session still
// present), whereas archiving first would destroy the identity proving which
// native key group belongs to the orphan.
type orphanedDKGOrderedTestEngine struct {
	operations *[]string
}

func (engine *orphanedDKGOrderedTestEngine) RetireDistributedDKGKeyPackages(
	keyGroup string,
) error {
	*engine.operations = append(*engine.operations, "retire:"+keyGroup)
	return nil
}

type orphanedDKGOrderedTestPersistence struct {
	*mockPersistenceHandle
	operations *[]string
}

func (persistence *orphanedDKGOrderedTestPersistence) Archive(
	directory string,
) error {
	*persistence.operations = append(*persistence.operations, "archive")
	return persistence.mockPersistenceHandle.Archive(directory)
}

// orphanedDKGTestSessionRegistry builds a wallet registry holding ONE real local
// FROST session, so the reconciler's local-session paths run against the same
// snapshot production takes rather than an empty wallet cache.
func orphanedDKGTestSessionRegistry(
	t *testing.T,
	attemptStartBlock uint64,
	keyGroup string,
	operatorCount int,
	persistenceHandle persistence.ProtectedHandle,
) *walletRegistry {
	t.Helper()
	publicKey := frostBindingWalletPublicKey()
	walletPublicKeyHash := [20]byte{0x51}
	operators := make([]chain.Address, 0, operatorCount)
	for index := 0; index < operatorCount; index++ {
		operators = append(
			operators,
			chain.Address(fmt.Sprintf("0x%040d", index+1)),
		)
	}

	registry := orphanedDKGTestWalletRegistry(t, attemptStartBlock)
	registry.walletStorage = newWalletStorage(persistenceHandle)
	registry.retainedFrostKeyGroups = make(map[[32]byte]string)
	registry.walletCache[getWalletStorageKey(publicKey)] =
		orphanedDKGTestSession(t, publicKey, walletPublicKeyHash, keyGroup, operators)

	return registry
}

func orphanedDKGTestSession(
	t *testing.T,
	publicKey *ecdsa.PublicKey,
	walletPublicKeyHash [20]byte,
	keyGroup string,
	operators []chain.Address,
) *walletCacheValue {
	t.Helper()
	return &walletCacheValue{
		walletID:            frostBindingWalletID(t),
		walletPublicKeyHash: walletPublicKeyHash,
		signers: []*signer{{
			wallet: wallet{
				publicKey:             publicKey,
				signingGroupOperators: operators,
			},
			signingGroupMemberIndex: 1,
			signerMaterial:          frostBindingSignerMaterial(t, keyGroup),
		}},
	}
}

func TestFrostOrphanedDKGReconcilerRetiresNativeMaterialBeforeArchiving(
	t *testing.T,
) {
	operations := make([]string, 0)
	persistenceHandle := &orphanedDKGOrderedTestPersistence{
		mockPersistenceHandle: &mockPersistenceHandle{},
		operations:            &operations,
	}
	registry := orphanedDKGTestSessionRegistry(
		t,
		90,
		frostBindingEvenKey,
		3,
		persistenceHandle,
	)
	walletID := frostBindingWalletID(t)
	snapshotChain := &orphanedDKGSnapshotTestChain{
		state:      Idle,
		registered: make(map[[32]byte]bool),
	}
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain:    snapshotChain,
		walletRegistry:   registry,
		anchorAdmission:  orphanedDKGTestAnchorAdmission(),
		retirementEngine: &orphanedDKGOrderedTestEngine{operations: &operations},
		readInventory: func() (
			*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
			error,
		) {
			return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
				Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
					WalletID:         walletID,
					KeyGroup:         frostBindingEvenKey,
					ParticipantCount: 3,
				}},
			}, nil
		},
	}

	if err := reconciler.reconcile(
		context.Background(),
		orphanedDKGTestPoint(),
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(operations) != 2 ||
		operations[0] != "retire:"+frostBindingEvenKey ||
		operations[1] != "archive" {
		t.Fatalf(
			"orphan was not retired natively before archiving: [%v]",
			operations,
		)
	}
	if len(registry.walletCache) != 0 {
		t.Fatalf(
			"orphaned local session survived reconciliation: [%d]",
			len(registry.walletCache),
		)
	}
	if registry.retainedFrostKeyGroups[walletID] != frostBindingEvenKey {
		t.Fatal("archived orphan did not retain its exact key-group binding")
	}
	if reconciler.anchorAdmission.reserved !=
		(frostNativeSignerAnchorCapacity{}) {
		t.Fatalf(
			"retirement reservation was not released: [%+v]",
			reconciler.anchorAdmission.reserved,
		)
	}
}

// TestFrostOrphanedDKGReconcilerArchivesSessionWithoutNativeMaterial covers the
// crash-recovery half-state: a previous pass retired the native key group and
// died before archiving the Go session. The repeat pass must finish the archive
// and must NOT ask the engine to retire material that is already gone.
func TestFrostOrphanedDKGReconcilerArchivesSessionWithoutNativeMaterial(
	t *testing.T,
) {
	operations := make([]string, 0)
	persistenceHandle := &orphanedDKGOrderedTestPersistence{
		mockPersistenceHandle: &mockPersistenceHandle{},
		operations:            &operations,
	}
	registry := orphanedDKGTestSessionRegistry(
		t,
		90,
		frostBindingEvenKey,
		3,
		persistenceHandle,
	)
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain: &orphanedDKGSnapshotTestChain{
			state:      Idle,
			registered: make(map[[32]byte]bool),
		},
		walletRegistry:   registry,
		anchorAdmission:  orphanedDKGTestAnchorAdmission(),
		retirementEngine: &orphanedDKGOrderedTestEngine{operations: &operations},
		readInventory: func() (
			*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
			error,
		) {
			return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
				Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{},
			}, nil
		},
	}

	if err := reconciler.reconcile(
		context.Background(),
		orphanedDKGTestPoint(),
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(operations) != 1 || operations[0] != "archive" {
		t.Fatalf(
			"half-retired orphan was not archived exactly once: [%v]",
			operations,
		)
	}
	if len(registry.walletCache) != 0 {
		t.Fatal("half-retired orphan kept its local session")
	}
	// No native inventory means nothing to charge the anchor for.
	if reconciler.anchorAdmission.reserved !=
		(frostNativeSignerAnchorCapacity{}) {
		t.Fatalf(
			"session-only retirement charged anchor capacity: [%+v]",
			reconciler.anchorAdmission.reserved,
		)
	}
}

// TestFrostOrphanedDKGReconcilerRejectsDisagreeingMaterial pins the fail-closed
// check guarding destructive reconciliation: if the native inventory and the Go
// session describe the same wallet differently, the reconciler cannot know which
// key group a retirement would destroy, so it must refuse to retire anything.
func TestFrostOrphanedDKGReconcilerRejectsDisagreeingMaterial(t *testing.T) {
	walletID := frostBindingWalletID(t)
	testCases := map[string]struct {
		inventoryKeyGroup         string
		inventoryParticipantCount uint16
	}{
		"key group parity disagrees": {
			inventoryKeyGroup:         frostBindingOddKey,
			inventoryParticipantCount: 3,
		},
		"participant count disagrees": {
			inventoryKeyGroup:         frostBindingEvenKey,
			inventoryParticipantCount: 4,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			operations := make([]string, 0)
			persistenceHandle := &orphanedDKGOrderedTestPersistence{
				mockPersistenceHandle: &mockPersistenceHandle{},
				operations:            &operations,
			}
			registry := orphanedDKGTestSessionRegistry(
				t,
				90,
				frostBindingEvenKey,
				3,
				persistenceHandle,
			)
			snapshotChain := &orphanedDKGSnapshotTestChain{
				state:      Idle,
				registered: make(map[[32]byte]bool),
			}
			reconciler := &frostOrphanedDKGReconciler{
				snapshotChain:   snapshotChain,
				walletRegistry:  registry,
				anchorAdmission: orphanedDKGTestAnchorAdmission(),
				retirementEngine: &orphanedDKGOrderedTestEngine{
					operations: &operations,
				},
				readInventory: func() (
					*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
					error,
				) {
					return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
						Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{{
							WalletID:         walletID,
							KeyGroup:         test.inventoryKeyGroup,
							ParticipantCount: test.inventoryParticipantCount,
						}},
					}, nil
				},
			}

			err := reconciler.reconcile(
				context.Background(),
				orphanedDKGTestPoint(),
				map[[32]byte]struct{}{},
			)
			if err == nil ||
				!strings.Contains(err.Error(), "FROST DKG material disagree") {
				t.Fatalf("disagreeing DKG material was accepted: [%v]", err)
			}
			if len(operations) != 0 || len(snapshotChain.points) != 0 {
				t.Fatalf(
					"disagreeing DKG material was acted on: [%v]",
					operations,
				)
			}
			if len(registry.walletCache) != 1 {
				t.Fatal("disagreeing DKG material lost its local session")
			}
		})
	}
}

func TestFrostOrphanedDKGReconcilerRejectsDuplicateLocalSession(t *testing.T) {
	operations := make([]string, 0)
	persistenceHandle := &orphanedDKGOrderedTestPersistence{
		mockPersistenceHandle: &mockPersistenceHandle{},
		operations:            &operations,
	}
	registry := orphanedDKGTestSessionRegistry(
		t,
		90,
		frostBindingEvenKey,
		3,
		persistenceHandle,
	)
	// A second cache entry resolving to the SAME wallet ID: reconciliation
	// cannot tell which session a retirement would archive, so it must stop.
	registry.walletCache["duplicate-frost-session"] =
		orphanedDKGTestSession(
			t,
			frostBindingWalletPublicKey(),
			[20]byte{0x52},
			frostBindingEvenKey,
			[]chain.Address{"0x01", "0x02", "0x03"},
		)
	snapshotChain := &orphanedDKGSnapshotTestChain{
		state:      Idle,
		registered: make(map[[32]byte]bool),
	}
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain:    snapshotChain,
		walletRegistry:   registry,
		anchorAdmission:  orphanedDKGTestAnchorAdmission(),
		retirementEngine: &orphanedDKGOrderedTestEngine{operations: &operations},
		readInventory: func() (
			*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
			error,
		) {
			return &frostsigning.NativeTBTCSignerRetainedKeyPackageInventory{
				Entries: []frostsigning.NativeTBTCSignerRetainedKeyGroup{},
			}, nil
		},
	}

	err := reconciler.reconcile(
		context.Background(),
		orphanedDKGTestPoint(),
		map[[32]byte]struct{}{},
	)
	if err == nil ||
		!strings.Contains(err.Error(), "duplicate local FROST DKG wallet session") {
		t.Fatalf("duplicate local FROST session was accepted: [%v]", err)
	}
	if len(operations) != 0 || len(snapshotChain.points) != 0 {
		t.Fatalf("duplicate local FROST session was acted on: [%v]", operations)
	}
	if len(registry.walletCache) != 2 {
		t.Fatal("duplicate local FROST sessions were mutated")
	}
}

func orphanedDKGTestWalletID(t *testing.T) [32]byte {
	t.Helper()
	decoded, err := hex.DecodeString(
		"79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
	)
	if err != nil {
		t.Fatal(err)
	}
	var walletID [32]byte
	copy(walletID[:], decoded)
	return walletID
}

func orphanedDKGTestWalletRegistry(
	t *testing.T,
	attemptStartBlock uint64,
) *walletRegistry {
	t.Helper()
	seed, err := canonicalFrostDKGAttemptSeed(big.NewInt(100))
	if err != nil {
		t.Fatal(err)
	}
	return &walletRegistry{
		walletCache: make(map[string]*walletCacheValue),
		frostDKGRetirementBoundaries: map[string]uint64{
			frostDKGRetirementBoundaryIdentity(
				frostDKGRetirementBoundaryKindAttempt,
				seed,
				attemptStartBlock,
			): attemptStartBlock,
		},
	}
}

func orphanedDKGTestPoint() FrostPreSignFinality {
	return FrostPreSignFinality{
		BlockNumber: 100,
		BlockHash:   [32]byte{0xaa},
	}
}

func orphanedDKGTestAnchorAdmission() *frostNativeSignerAnchorAdmissionController {
	return &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			return frostNativeSignerAnchorCapacity{
				Revisions: FrostNativeSignerAnchorRotationWarningHeadroom + 10,
				Generations: FrostNativeSignerAnchorRotationWarningHeadroom +
					10,
			}, nil
		},
	}
}
