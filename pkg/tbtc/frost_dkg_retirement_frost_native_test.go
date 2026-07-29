//go:build frost_native

package tbtc

import (
	"context"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type orphanedDKGRegistrationTestChain struct {
	registered map[[32]byte]bool
}

func (chain *orphanedDKGRegistrationTestChain) IsFrostWalletRegistered(
	walletID [32]byte,
) (bool, error) {
	return chain.registered[walletID], nil
}

type orphanedDKGStateTestChain struct {
	FrostDKGChain
	state DKGState
}

func (chain *orphanedDKGStateTestChain) GetFrostDKGState() (
	DKGState,
	error,
) {
	return chain.state, nil
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
	reconciler := &frostOrphanedDKGReconciler{
		dkgChain: &orphanedDKGStateTestChain{state: Idle},
		registrationChain: &orphanedDKGRegistrationTestChain{
			registered: make(map[[32]byte]bool),
		},
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

	if err := reconciler.reconcile(
		context.Background(),
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
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
		dkgChain: &orphanedDKGStateTestChain{state: Idle},
		registrationChain: &orphanedDKGRegistrationTestChain{
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
		dkgChain: &orphanedDKGStateTestChain{state: AwaitingResult},
		registrationChain: &orphanedDKGRegistrationTestChain{
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
		map[[32]byte]struct{}{},
	); err != nil {
		t.Fatal(err)
	}
	if len(engine.retired) != 0 {
		t.Fatalf("AwaitingResult DKG material was retired: [%v]", engine.retired)
	}
}
