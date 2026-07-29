//go:build frost_native

package tbtc

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	"github.com/keep-network/keep-core/pkg/frost/registry"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type frostPendingDKGDescriptor struct {
	walletID         [32]byte
	participantCount uint16
	preserveAll      bool
}

type frostOrphanedDKGReconciler struct {
	dkgChain          FrostDKGChain
	registrationChain frostWalletRegistrationChain
	walletRegistry    *walletRegistry
	anchorAdmission   *frostNativeSignerAnchorAdmissionController
	retirementEngine  frostsigning.NativeTBTCSignerDistributedDKGRetirementEngine
	readInventory     func() (*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory, error)
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
	func(context.Context, map[[32]byte]struct{}) error,
	error,
) {
	if chain == nil || walletRegistry == nil || anchorAdmission == nil {
		return nil, fmt.Errorf(
			"orphaned FROST DKG reconciliation dependencies are incomplete",
		)
	}
	dkgChain, ok := chain.(FrostDKGChain)
	if !ok {
		return nil, fmt.Errorf(
			"chain does not expose FROST DKG state for orphan reconciliation",
		)
	}
	registrationChain, ok := chain.(frostWalletRegistrationChain)
	if !ok {
		return nil, fmt.Errorf(
			"chain does not expose FROST wallet registration for orphan reconciliation",
		)
	}
	retirementEngine, ok := frostsigning.CurrentNativeTBTCSignerEngine().(frostsigning.NativeTBTCSignerDistributedDKGRetirementEngine)
	if !ok {
		return nil, fmt.Errorf(
			"native signer does not support durable distributed-DKG retirement",
		)
	}
	reconciler := &frostOrphanedDKGReconciler{
		dkgChain:          dkgChain,
		registrationChain: registrationChain,
		walletRegistry:    walletRegistry,
		anchorAdmission:   anchorAdmission,
		retirementEngine:  retirementEngine,
		readInventory: frostsigning.
			ReadNativeTBTCSignerRetainedKeyPackageInventory,
	}
	return reconciler.reconcile, nil
}

func (reconciler *frostOrphanedDKGReconciler) reconcile(
	ctx context.Context,
	canonicalWallets map[[32]byte]struct{},
) error {
	if reconciler == nil || ctx == nil || canonicalWallets == nil ||
		reconciler.readInventory == nil {
		return fmt.Errorf("orphaned FROST DKG reconciliation is not configured")
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
	sessions, err := reconciler.walletRegistry.frostLocalSessionSnapshot()
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

	var pendingRead bool
	var pending *frostPendingDKGDescriptor
	retirements := make([]*frostDKGRetirementCandidate, 0)
	for _, walletID := range walletIDs {
		candidate := candidatesByWallet[walletID]
		if _, canonical := canonicalWallets[walletID]; canonical {
			continue
		}
		registered, err :=
			reconciler.registrationChain.IsFrostWalletRegistered(walletID)
		if err != nil {
			return fmt.Errorf(
				"cannot check FROST registration for wallet [0x%x]: [%w]",
				walletID,
				err,
			)
		}
		if registered {
			continue
		}
		if !pendingRead {
			pending, err = reconciler.currentPendingDescriptor()
			if err != nil {
				return err
			}
			pendingRead = true
		}
		if pending != nil {
			if pending.preserveAll ||
				(pending.walletID == candidate.walletID &&
					pending.participantCount == candidate.participantCount) {
				continue
			}
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

func (reconciler *frostOrphanedDKGReconciler) currentPendingDescriptor() (
	*frostPendingDKGDescriptor,
	error,
) {
	state, err := reconciler.dkgChain.GetFrostDKGState()
	if err != nil {
		return nil, fmt.Errorf("cannot read current FROST DKG state: [%w]", err)
	}
	if state == AwaitingResult {
		// A package may have been persisted by the current DKG, or its result
		// transaction may have been accepted despite an ambiguous RPC error.
		// Without an on-chain output key there is no exact identity to compare,
		// so preserve noncanonical packages until Challenge or timeout makes the
		// outcome provable. This can delay readiness but cannot release signing.
		return &frostPendingDKGDescriptor{preserveAll: true}, nil
	}
	if state != Challenge {
		return nil, nil
	}
	events, err := reconciler.dkgChain.PastFrostDKGResultSubmittedEvents(nil)
	if err != nil {
		return nil, fmt.Errorf("cannot read pending FROST DKG result: [%w]", err)
	}
	var latest *FrostDKGResultSubmittedEvent
	for _, event := range events {
		if event == nil || event.Result == nil {
			continue
		}
		if latest == nil || event.BlockNumber >= latest.BlockNumber {
			latest = event
		}
	}
	if latest == nil {
		return nil, fmt.Errorf(
			"FROST DKG state is Challenge but no submitted result is available",
		)
	}
	valid, _, err := reconciler.dkgChain.IsFrostDKGResultValid(latest.Result)
	if err != nil {
		return nil, fmt.Errorf("cannot validate pending FROST DKG result: [%w]", err)
	}
	if !valid {
		// A successfully challenged invalid result returns this same DKG
		// attempt to AwaitingResult, where another member may submit the valid
		// result backed by the packages already persisted by this node. The
		// invalid submission gives us no trustworthy wallet identity to match,
		// so retain every noncanonical package until the attempt is resolved.
		return &frostPendingDKGDescriptor{preserveAll: true}, nil
	}
	activeMembers, err := registry.ActiveMembersFromMisbehaved(
		latest.Result.Members,
		latest.Result.MisbehavedMembersIndices,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid pending FROST DKG members: [%w]", err)
	}
	if len(activeMembers) > int(^uint16(0)) {
		return nil, fmt.Errorf("pending FROST DKG participant count overflows")
	}
	return &frostPendingDKGDescriptor{
		walletID:         [32]byte(latest.Result.XOnlyOutputKey),
		participantCount: uint16(len(activeMembers)),
	}, nil
}
