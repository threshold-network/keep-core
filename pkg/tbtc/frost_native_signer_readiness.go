package tbtc

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"

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

// FrostNativeSignerStateWitnessAnchor is an independently durable accepted
// tip of the native signer's append-only state-witness chain. It is dynamic
// state and deliberately is not folded into the immutable activation manifest
// or stable store identity.
type FrostNativeSignerStateWitnessAnchor struct {
	StoreFingerprint [32]byte
	Generation       uint64
	Commitment       [32]byte
}

// FrostNativeSignerStateWitnessAnchorSource supplies the last state witness
// accepted by the independent activation/watchtower system. Its backing store
// must be outside the signer state directory and must atomically persist an
// accepted handshake before treating that handshake as active.
//
// A local signer file cannot implement this interface safely: coordinated
// rollback of that file and the signer state would otherwise be invisible.
type FrostNativeSignerStateWitnessAnchorSource interface {
	ReadFrostNativeSignerStateWitnessAnchor(
		context.Context,
	) (*FrostNativeSignerStateWitnessAnchor, error)
}

type frostNativeSignerInventoryReader func() (
	*frostsigning.NativeTBTCSignerRetainedKeyPackageInventory,
	error,
)

type frostNativeSignerStateWitnessProofReader func(
	*frostsigning.NativeTBTCSignerStateWitnessProofRequest,
) (*frostsigning.NativeTBTCSignerStateWitnessProof, error)

type frostNativeSignerInventoryExpectation struct {
	WalletID         [32]byte
	Threshold        uint16
	ParticipantCount uint16
	ShareEpoch       uint64
	ParticipantSeats []uint16
}

type frostNativeSignerInventorySnapshot struct {
	Schema                  string
	StoreFingerprint        [32]byte
	StateGeneration         uint64
	StateCommitment         [32]byte
	PreviousStateCommitment [32]byte
	StateImageDigest        [32]byte
	InventoryCommitment     [32]byte
	WalletCount             uint64
	KeyPackageCount         uint64
}

type frostNativeSignerInventoryBinding struct {
	storeBinding  *frostDurableSessionStoreBinding
	anchorSource  FrostNativeSignerStateWitnessAnchorSource
	readInventory frostNativeSignerInventoryReader
	readProof     frostNativeSignerStateWitnessProofReader

	mutex    sync.Mutex
	baseline *FrostNativeSignerStateWitnessAnchor
}

func newFrostNativeSignerInventoryBinding(
	storeBinding *frostDurableSessionStoreBinding,
	anchorSource FrostNativeSignerStateWitnessAnchorSource,
	readInventory frostNativeSignerInventoryReader,
	readProof frostNativeSignerStateWitnessProofReader,
) (*frostNativeSignerInventoryBinding, error) {
	if storeBinding == nil || anchorSource == nil || readInventory == nil || readProof == nil {
		return nil, fmt.Errorf("FROST native signer inventory dependencies are incomplete")
	}
	if _, err := storeBinding.verify(); err != nil {
		return nil, fmt.Errorf("FROST native signer inventory store is not bound: [%w]", err)
	}
	return &frostNativeSignerInventoryBinding{
		storeBinding:  storeBinding,
		anchorSource:  anchorSource,
		readInventory: readInventory,
		readProof:     readProof,
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
	anchor, err := binding.anchorSource.ReadFrostNativeSignerStateWitnessAnchor(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read independent native signer state-witness anchor: [%w]", err)
	}
	if err := validateFrostNativeSignerStateWitnessAnchor(anchor, storeFingerprint); err != nil {
		return nil, err
	}

	inventory, err := binding.readInventory()
	if err != nil {
		return nil, fmt.Errorf("cannot read native retained key-package inventory: [%w]", err)
	}
	if inventory == nil || inventory.StoreFingerprint != storeFingerprint {
		return nil, fmt.Errorf("native retained key-package inventory is absent or belongs to another store")
	}
	target := FrostNativeSignerStateWitnessAnchor{
		StoreFingerprint: inventory.StoreFingerprint,
		Generation:       inventory.StateGeneration,
		Commitment:       inventory.StateCommitment,
	}
	if err := binding.verifyAncestry(anchor, &target); err != nil {
		return nil, fmt.Errorf("native signer state does not descend from the independent anchor: [%w]", err)
	}
	if binding.baseline != nil {
		if err := binding.verifyAncestry(binding.baseline, &target); err != nil {
			return nil, fmt.Errorf("native signer state does not descend from the process baseline: [%w]", err)
		}
	}
	if err := verifyFrostNativeSignerInventoryEntries(inventory.Entries, expected); err != nil {
		return nil, err
	}

	binding.baseline = &FrostNativeSignerStateWitnessAnchor{
		StoreFingerprint: target.StoreFingerprint,
		Generation:       target.Generation,
		Commitment:       target.Commitment,
	}
	keyPackageCount := uint64(0)
	for _, entry := range inventory.Entries {
		keyPackageCount += uint64(len(entry.KeyPackages))
	}
	return &frostNativeSignerInventorySnapshot{
		Schema:                  inventory.Schema,
		StoreFingerprint:        inventory.StoreFingerprint,
		StateGeneration:         inventory.StateGeneration,
		StateCommitment:         inventory.StateCommitment,
		PreviousStateCommitment: inventory.PreviousStateCommitment,
		StateImageDigest:        inventory.StateImageDigest,
		InventoryCommitment:     inventory.InventoryCommitment,
		WalletCount:             uint64(len(inventory.Entries)),
		KeyPackageCount:         keyPackageCount,
	}, nil
}

func validateFrostNativeSignerStateWitnessAnchor(
	anchor *FrostNativeSignerStateWitnessAnchor,
	storeFingerprint [32]byte,
) error {
	if anchor == nil || anchor.StoreFingerprint != storeFingerprint ||
		anchor.Generation == 0 || anchor.Commitment == [32]byte{} {
		return fmt.Errorf("independent native signer state-witness anchor is invalid")
	}
	return nil
}

func (binding *frostNativeSignerInventoryBinding) verifyAncestry(
	ancestor *FrostNativeSignerStateWitnessAnchor,
	target *FrostNativeSignerStateWitnessAnchor,
) error {
	if ancestor == nil || target == nil ||
		ancestor.StoreFingerprint != target.StoreFingerprint ||
		ancestor.Generation == 0 || target.Generation == 0 ||
		ancestor.Commitment == [32]byte{} || target.Commitment == [32]byte{} ||
		target.Generation < ancestor.Generation {
		return fmt.Errorf("state-witness ancestry bounds are invalid")
	}
	if target.Generation == ancestor.Generation && target.Commitment != ancestor.Commitment {
		return fmt.Errorf("equal state-witness generations have different commitments")
	}

	cursorGeneration := ancestor.Generation
	cursorCommitment := ancestor.Commitment
	for page := 0; page < frostNativeSignerStateWitnessMaximumPages; page++ {
		request := &frostsigning.NativeTBTCSignerStateWitnessProofRequest{
			Schema:             frostsigning.NativeTBTCSignerStateWitnessProofRequestSchema,
			StoreFingerprint:   target.StoreFingerprint,
			AncestorGeneration: cursorGeneration,
			AncestorCommitment: cursorCommitment,
			TargetGeneration:   target.Generation,
			TargetCommitment:   target.Commitment,
			MaximumEntries:     frostsigning.NativeTBTCSignerStateWitnessProofMaximumEntries,
		}
		proof, err := binding.readProof(request)
		if err != nil {
			return fmt.Errorf("cannot read native signer state-witness proof: [%w]", err)
		}
		if proof == nil || proof.Schema != frostsigning.NativeTBTCSignerStateWitnessProofSchema ||
			proof.StoreFingerprint != request.StoreFingerprint ||
			proof.AncestorGeneration != request.AncestorGeneration ||
			proof.AncestorCommitment != request.AncestorCommitment ||
			proof.TargetGeneration != request.TargetGeneration ||
			proof.TargetCommitment != request.TargetCommitment ||
			len(proof.Entries) > int(request.MaximumEntries) {
			return fmt.Errorf("native signer returned a proof for different ancestry bounds")
		}
		if proof.Complete {
			return nil
		}
		if len(proof.Entries) == 0 {
			return fmt.Errorf("native signer state-witness proof made no progress")
		}
		last := proof.Entries[len(proof.Entries)-1]
		cursorGeneration = last.Generation
		cursorCommitment = last.StateCommitment
	}
	return fmt.Errorf(
		"native signer state-witness ancestry exceeds the bounded proof window [%d]",
		frostNativeSignerStateWitnessMaximumPages*int(frostsigning.NativeTBTCSignerStateWitnessProofMaximumEntries),
	)
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
}

type frostProductionSignerReadinessVerifier interface {
	verifyFrostProductionSignerReadiness(
		context.Context,
		FrostPreSignFinality,
	) (*frostProductionSignerReadinessSnapshot, error)
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
	expected, err := readiness.journal.nativeSignerInventoryExpectations(ctx, point)
	if err != nil {
		return nil, err
	}
	firstInventory, err := readiness.inventoryBinding.verify(ctx, expected)
	if err != nil {
		return nil, err
	}
	secondInventory, err := readiness.inventoryBinding.verify(ctx, expected)
	if err != nil {
		return nil, err
	}
	if *firstInventory != *secondInventory {
		return nil, fmt.Errorf("native signer state changed during readiness reconciliation")
	}
	if !readiness.interactiveSigningReady() {
		return nil, fmt.Errorf("interactive FROST signing engine became unavailable during reconciliation")
	}
	return &frostProductionSignerReadinessSnapshot{
		Journal:                 journalSnapshot,
		Inventory:               secondInventory,
		InteractiveSigningReady: true,
	}, nil
}

func (journal *frostRetainedGroupJournal) nativeSignerInventoryExpectations(
	ctx context.Context,
	point FrostPreSignFinality,
) ([]frostNativeSignerInventoryExpectation, error) {
	if journal == nil || ctx == nil {
		return nil, fmt.Errorf("FROST retained-group journal expectation context is invalid")
	}
	localOperatorID, err := journal.source.ResolveOperatorID(ctx, journal.operatorAddress, point)
	if err != nil || localOperatorID == 0 {
		return nil, fmt.Errorf("cannot resolve local operator for native signer inventory: [%w]", err)
	}

	journal.mutex.Lock()
	defer journal.mutex.Unlock()
	if journal.closed || journal.state.CurrentPoint != point {
		return nil, fmt.Errorf("canonical retained-group journal moved during native signer reconciliation")
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
		expected = append(expected, frostNativeSignerInventoryExpectation{
			WalletID:         wallet.WalletID,
			Threshold:        frostPreSignAuthorizationThreshold,
			ParticipantCount: uint16(len(wallet.OperatorIDs)),
			ShareEpoch:       0,
			ParticipantSeats: seats,
		})
	}
	return expected, nil
}
