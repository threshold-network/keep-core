//go:build frost_native

package tbtc

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type frostOrphanedDKGReconciler struct {
	snapshotChain    frostDKGRetirementSnapshotChain
	walletRegistry   *walletRegistry
	anchorAdmission  *frostNativeSignerAnchorAdmissionController
	retirementEngine frostsigning.NativeTBTCSignerDistributedDKGRetirementEngine
	readInventory    func() (*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory, error)
	readCurrentBlock func() (uint64, error)
}

type frostDKGRetirementCandidate struct {
	walletID            [32]byte
	keyGroup            string
	participantCount    uint16
	walletPublicKeyHash [20]byte
	hasLocalSession     bool
	hasNativeInventory  bool
}

func newFrostOrphanedDKGReconciler(
	chain Chain,
	walletRegistry *walletRegistry,
	anchorAdmission *frostNativeSignerAnchorAdmissionController,
) (
	frostOrphanedDKGReconcilerFunc,
	error,
) {
	if chain == nil || walletRegistry == nil || anchorAdmission == nil {
		return nil, fmt.Errorf(
			"orphaned FROST DKG reconciliation dependencies are incomplete",
		)
	}
	snapshotChain, ok := chain.(frostDKGRetirementSnapshotChain)
	if !ok {
		return nil, fmt.Errorf(
			"chain does not expose exact finalized FROST DKG snapshots for orphan reconciliation",
		)
	}
	retirementEngine, ok := frostsigning.CurrentNativeTBTCSignerEngine().(frostsigning.NativeTBTCSignerDistributedDKGRetirementEngine)
	if !ok {
		return nil, fmt.Errorf(
			"native signer does not support durable distributed-DKG retirement",
		)
	}
	blockCounter, err := chain.BlockCounter()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot initialize orphaned FROST DKG migration boundary: [%w]",
			err,
		)
	}
	reconciler := &frostOrphanedDKGReconciler{
		snapshotChain:    snapshotChain,
		walletRegistry:   walletRegistry,
		anchorAdmission:  anchorAdmission,
		retirementEngine: retirementEngine,
		readInventory: frostsigning.
			ReadNativeTBTCSignerRetainedKeyPackageInventory,
		readCurrentBlock: blockCounter.CurrentBlock,
	}
	return reconciler.reconcile, nil
}

func (reconciler *frostOrphanedDKGReconciler) reconcile(
	ctx context.Context,
	target FrostPreSignFinality,
	canonicalWallets map[[32]byte]struct{},
) error {
	if reconciler == nil || ctx == nil || canonicalWallets == nil ||
		reconciler.snapshotChain == nil || reconciler.readInventory == nil {
		return fmt.Errorf("orphaned FROST DKG reconciliation is not configured")
	}
	if target.BlockNumber == 0 || target.BlockHash == [32]byte{} {
		return fmt.Errorf("orphaned FROST DKG reconciliation point is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	inventory, err := reconciler.readInventory()
	if err != nil {
		return fmt.Errorf("cannot read native key-package inventory: [%w]", err)
	}
	if inventory == nil {
		return fmt.Errorf("native key-package inventory is nil")
	}
	sessions, latestRetirementBoundary, hasRetirementBoundary, err :=
		reconciler.walletRegistry.frostDKGRetirementMaterialSnapshot()
	if err != nil {
		return err
	}

	candidatesByWallet := make(map[[32]byte]*frostDKGRetirementCandidate)
	for _, entry := range inventory.Entries {
		candidatesByWallet[entry.WalletID] = &frostDKGRetirementCandidate{
			walletID:           entry.WalletID,
			keyGroup:           entry.KeyGroup,
			participantCount:   entry.ParticipantCount,
			hasNativeInventory: true,
		}
	}
	for _, session := range sessions {
		candidate, exists := candidatesByWallet[session.WalletID]
		if !exists {
			candidate = &frostDKGRetirementCandidate{
				walletID: session.WalletID,
			}
			candidatesByWallet[session.WalletID] = candidate
		}
		if candidate.hasLocalSession {
			return fmt.Errorf("duplicate local FROST DKG wallet session")
		}
		if candidate.hasNativeInventory &&
			(candidate.keyGroup != session.KeyGroup ||
				candidate.participantCount != uint16(len(session.OperatorAddresses))) {
			return fmt.Errorf(
				"native and Go FROST DKG material disagree for wallet [0x%x]",
				session.WalletID,
			)
		}
		candidate.keyGroup = session.KeyGroup
		candidate.participantCount = uint16(len(session.OperatorAddresses))
		candidate.walletPublicKeyHash = session.WalletPublicKeyHash
		candidate.hasLocalSession = true
	}

	walletIDs := make([][32]byte, 0, len(candidatesByWallet))
	for walletID := range candidatesByWallet {
		walletIDs = append(walletIDs, walletID)
	}
	sort.Slice(walletIDs, func(i, j int) bool {
		return bytes.Compare(walletIDs[i][:], walletIDs[j][:]) < 0
	})

	noncanonicalWalletIDs := make([][32]byte, 0, len(walletIDs))
	for _, walletID := range walletIDs {
		if _, canonical := canonicalWallets[walletID]; !canonical {
			noncanonicalWalletIDs = append(noncanonicalWalletIDs, walletID)
		}
	}
	if len(noncanonicalWalletIDs) == 0 {
		return nil
	}

	// Native package persistence is ordered after the durable attempt marker.
	// Packages created by older binaries have no marker, so observe and
	// persist the latest canonical head after reading their inventory. A later
	// finalized point covering that head can prove those pre-existing
	// packages' attempts are no longer hidden beyond the journal cursor.
	if !hasRetirementBoundary {
		if reconciler.readCurrentBlock == nil {
			return fmt.Errorf(
				"FROST DKG migration boundary source is unavailable",
			)
		}
		migrationBoundary, err := reconciler.readCurrentBlock()
		if err != nil {
			return fmt.Errorf(
				"cannot read FROST DKG migration boundary: [%w]",
				err,
			)
		}
		if migrationBoundary < target.BlockNumber {
			return fmt.Errorf(
				"FROST DKG migration boundary is behind finalized history",
			)
		}
		if err := reconciler.walletRegistry.recordFrostDKGMigrationBoundary(
			migrationBoundary,
		); err != nil {
			return err
		}
		return nil
	}

	// Require finality to advance beyond the boundary, not merely equal it.
	// This also makes the one-time migration fail closed if reconciliation is
	// repeated at the same finalized point immediately after persisting it.
	if target.BlockNumber <= latestRetirementBoundary {
		return nil
	}

	snapshot, err := reconciler.snapshotChain.FrostDKGRetirementSnapshot(
		ctx,
		target,
		noncanonicalWalletIDs,
	)
	if err != nil {
		return fmt.Errorf(
			"cannot read finalized FROST DKG retirement snapshot: [%w]",
			err,
		)
	}
	if snapshot == nil || snapshot.Point != target ||
		snapshot.State < Idle || snapshot.State > Challenge ||
		len(snapshot.RegisteredWallets) != len(noncanonicalWalletIDs) {
		return fmt.Errorf("finalized FROST DKG retirement snapshot is incomplete")
	}

	unresolvedDKG := snapshot.State == AwaitingResult ||
		snapshot.State == Challenge
	retirements := make([]*frostDKGRetirementCandidate, 0)
	for _, walletID := range noncanonicalWalletIDs {
		candidate := candidatesByWallet[walletID]
		registered, present := snapshot.RegisteredWallets[walletID]
		if !present {
			return fmt.Errorf(
				"finalized FROST DKG retirement snapshot omits wallet [0x%x]",
				walletID,
			)
		}
		if registered || unresolvedDKG {
			continue
		}
		retirements = append(retirements, candidate)
	}

	var nativeRetirementCount uint64
	for _, candidate := range retirements {
		if candidate.hasNativeInventory {
			nativeRetirementCount++
		}
	}
	var reservation *frostNativeSignerAnchorRevisionReservation
	if nativeRetirementCount > 0 {
		reservation, err = reconciler.anchorAdmission.reserveDKGRetirement(
			ctx,
			nativeRetirementCount,
		)
		if err != nil {
			return err
		}
		defer reservation.Release()
	}

	for _, candidate := range retirements {
		if err := ctx.Err(); err != nil {
			return err
		}
		if candidate.hasNativeInventory {
			if err := reconciler.retirementEngine.
				RetireDistributedDKGKeyPackages(candidate.keyGroup); err != nil {
				return fmt.Errorf(
					"cannot retire native key group [%s]: [%w]",
					candidate.keyGroup,
					err,
				)
			}
		}
		if candidate.hasLocalSession {
			if err := reconciler.walletRegistry.archiveWallet(
				candidate.walletPublicKeyHash,
			); err != nil {
				return fmt.Errorf(
					"cannot archive orphaned FROST wallet [0x%x]: [%w]",
					candidate.walletID,
					err,
				)
			}
		}
	}
	return nil
}
