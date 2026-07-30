//go:build frost_native

package tbtc

import (
	"context"
	"encoding/hex"
	"testing"

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
		snapshotChain: snapshotChain,
		walletRegistry: &walletRegistry{
			walletCache: make(map[string]*walletCacheValue),
		},
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
		walletRegistry: &walletRegistry{
			walletCache: make(map[string]*walletCacheValue),
		},
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
		walletRegistry: &walletRegistry{
			walletCache: make(map[string]*walletCacheValue),
		},
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
		walletRegistry: &walletRegistry{
			walletCache: make(map[string]*walletCacheValue),
		},
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
				walletRegistry: &walletRegistry{
					walletCache: make(map[string]*walletCacheValue),
				},
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
