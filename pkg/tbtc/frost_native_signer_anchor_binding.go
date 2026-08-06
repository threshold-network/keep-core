package tbtc

import (
	"context"
	"fmt"
	"sync"
	"time"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

type frostNativeSignerStateWitnessTipReader func() (
	*frostsigning.NativeTBTCSignerStateWitnessTip,
	error,
)

type frostNativeSignerStateWitnessAcknowledger func(
	[]byte,
) (*frostsigning.NativeTBTCSignerStateWitnessCheckpointAcknowledgementResult, error)

type frostNativeSignerStateWitnessRecoverer func(
	[]byte,
) (*frostsigning.NativeTBTCSignerStateWitnessCheckpointRecoveryResult, error)

// frostNativeSignerAnchorBinding is the only production adapter between the
// central FFI output barrier and the authenticated remote checkpoint service.
// Its callbacks use only guard-bypassing readback/proof/acknowledgement symbols;
// they must never re-enter a request-taking signer operation while the global
// signer barrier lock is held.
type frostNativeSignerAnchorBinding struct {
	store                  FrostNativeSignerStateWitnessAnchorStore
	identity               FrostNativeSignerAnchorIdentity
	bindingHash            [32]byte
	floor                  FrostNativeSignerStateWitnessAnchorReference
	floorPreviousEventRoot [32]byte

	readTip     frostNativeSignerStateWitnessTipReader
	readProof   frostNativeSignerStateWitnessProofReader
	acknowledge frostNativeSignerStateWitnessAcknowledger
	recover     frostNativeSignerStateWitnessRecoverer
	now         func() time.Time

	mutex sync.Mutex
}

func newFrostNativeSignerAnchorBinding(
	store FrostNativeSignerStateWitnessAnchorStore,
	manifest FrostNativeSignerAnchorManifest,
	floor FrostNativeSignerStateWitnessAnchorReference,
	floorPreviousEventRoot [32]byte,
	readTip frostNativeSignerStateWitnessTipReader,
	readProof frostNativeSignerStateWitnessProofReader,
	acknowledge frostNativeSignerStateWitnessAcknowledger,
	recover frostNativeSignerStateWitnessRecoverer,
) (*frostNativeSignerAnchorBinding, error) {
	if store == nil || readTip == nil || readProof == nil ||
		acknowledge == nil || recover == nil {
		return nil, fmt.Errorf("FROST native signer anchor dependencies are incomplete")
	}
	identity := manifest.Identity
	if identity.StreamID != ComputeFrostNativeSignerAnchorStreamID(identity) {
		return nil, fmt.Errorf("FROST native signer anchor stream identity is invalid")
	}
	if floor.ServiceEpoch == 0 || floor.Revision == 0 ||
		floor.EventRoot == [32]byte{} ||
		floor.AcknowledgementDigest == [32]byte{} ||
		floor.Checkpoint.StoreFingerprint !=
			identity.SignerStoreFingerprint ||
		manifest.WitnessMaximumRecords != identity.WitnessMaximumRecords ||
		manifest.WitnessRotationThresholdRecords !=
			identity.WitnessRotationThresholdRecords {
		return nil, fmt.Errorf("FROST native signer certified floor is invalid")
	}
	computedFloorCommitment :=
		frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
			floor.Checkpoint.StoreFingerprint,
			floor.Checkpoint.Generation,
			floor.Checkpoint.PreviousStateCommitment,
			floor.Checkpoint.StateImageDigest,
		)
	if floor.Checkpoint.Generation == 0 ||
		computedFloorCommitment != floor.Checkpoint.StateCommitment {
		return nil, fmt.Errorf(
			"FROST native signer certified checkpoint is invalid",
		)
	}
	bindingHash := ComputeFrostNativeSignerAnchorBindingHash(identity)
	if bindingHash == [32]byte{} {
		return nil, fmt.Errorf("FROST native signer anchor binding hash is zero")
	}
	return &frostNativeSignerAnchorBinding{
		store:                  store,
		identity:               identity,
		bindingHash:            bindingHash,
		floor:                  floor,
		floorPreviousEventRoot: floorPreviousEventRoot,
		readTip:                readTip,
		readProof:              readProof,
		acknowledge:            acknowledge,
		recover:                recover,
		now:                    time.Now,
	}, nil
}

// reconcileStartup forces Rust state loading/migration through the cheap tip
// symbol, then authenticates the complete independent service history from the
// offline floor to a twice-read stable target. A local tip ahead of that target
// may be caught up with a bounded Rust proof; a remote tip ahead of/divergent
// from local is a rollback/fork failure.
func (binding *frostNativeSignerAnchorBinding) reconcileStartup(
	ctx context.Context,
) (*frostsigning.NativeTBTCSignerStateWitnessTip, error) {
	if binding == nil || ctx == nil {
		return nil, fmt.Errorf("FROST native signer anchor startup context is invalid")
	}
	binding.mutex.Lock()
	defer binding.mutex.Unlock()

	local, err := binding.readTip()
	if err != nil {
		return nil, fmt.Errorf("cannot read startup native signer state tip: %w", err)
	}
	if err := binding.validateLocalTip(local); err != nil {
		return nil, err
	}
	floor := binding.manifestFloorReference()
	history, err :=
		binding.store.ReadFrostNativeSignerStateWitnessAnchorHistory(ctx, floor)
	if err != nil {
		// In particular, an absent stream remains a hard failure. The online
		// signer is never authorized to create its own rollback boundary.
		return nil, fmt.Errorf(
			"cannot authenticate startup native signer anchor history: %w",
			err,
		)
	}
	serviceCommitments, remote, previousRemote, err :=
		binding.validateStartupHistory(history)
	if err != nil {
		return nil, err
	}

	localCheckpoint := frostNativeSignerCheckpointFromTip(*local)
	switch {
	case localCheckpoint.Generation < remote.Checkpoint.Generation:
		return nil, fmt.Errorf(
			"startup native signer local state is behind the authenticated remote anchor",
		)
	case localCheckpoint.Generation == remote.Checkpoint.Generation &&
		localCheckpoint != remote.Checkpoint:
		return nil, fmt.Errorf(
			"startup native signer local state forks the authenticated remote anchor",
		)
	case localCheckpoint == remote.Checkpoint:
		if err := binding.validateLocalHistorySplice(
			*local,
			serviceCommitments,
		); err != nil {
			return nil, err
		}
		if binding.localTipMatchesRemoteRecord(*local, remote) {
			copy := *local
			return &copy, nil
		}
		if binding.localTipHasNoAnchor(*local) {
			return binding.recoverAcknowledgementLocked(*local, remote)
		}
		if previousRemote == nil ||
			!binding.localTipHasAnchorReference(*local, *previousRemote) {
			return nil, fmt.Errorf(
				"local native signer anchor metadata is neither absent, exact, nor the authenticated immediately preceding revision",
			)
		}
		return binding.recoverAcknowledgementLocked(*local, remote)
	default:
		// The local state advanced durably before its remote CAS completed.
		// Its anchor metadata must still be the exact authenticated remote
		// target observed immediately before that operation.
		headroom, err := binding.restartableRevisionHeadroom(
			remote.ServiceEpoch,
			remote.Revision,
		)
		if err != nil {
			return nil, err
		}
		if headroom == 0 {
			return nil, fmt.Errorf(
				"startup native signer local-ahead state cannot cross the certified-floor history bound; offline anchor rotation is required",
			)
		}
		if _, err := binding.restartableGenerationHeadroom(
			localCheckpoint.Generation,
		); err != nil {
			return nil, fmt.Errorf(
				"startup native signer local-ahead state exceeds the certified-floor proof bound: %w",
				err,
			)
		}
		if !binding.localTipHasAnchorReference(
			*local,
			frostNativeSignerAnchorReferenceFromRecord(remote),
		) {
			return nil, fmt.Errorf(
				"startup native signer local-ahead state is not based on the exact remote anchor",
			)
		}
		proof, err := binding.collectProofLocked(
			remote.Checkpoint,
			localCheckpoint,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot prove startup local-ahead native signer state: %w",
				err,
			)
		}
		result, err :=
			binding.store.CompareAndSwapFrostNativeSignerStateWitnessAnchor(
				ctx,
				remote.Checkpoint,
				localCheckpoint,
				proof,
			)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot commit startup local-ahead native signer state: %w",
				err,
			)
		}
		if result == nil ||
			result.Acknowledgement.Checkpoint != localCheckpoint ||
			result.Acknowledgement.BindingHash != binding.bindingHash {
			return nil, fmt.Errorf(
				"startup native signer anchor CAS acknowledged a different binding or checkpoint",
			)
		}
		return binding.installCASResultLocked(*local, result)
	}
}

func (binding *frostNativeSignerAnchorBinding) VerifyNativeTBTCSignerStateTip(
	ctx context.Context,
	local frostsigning.NativeTBTCSignerStateWitnessTip,
) error {
	if binding == nil || ctx == nil {
		return fmt.Errorf("FROST native signer anchor verification context is invalid")
	}
	binding.mutex.Lock()
	defer binding.mutex.Unlock()
	if err := binding.validateLocalTip(&local); err != nil {
		return err
	}
	remote, err := binding.store.ReadFrostNativeSignerStateWitnessAnchor(ctx)
	if err != nil {
		return fmt.Errorf("cannot read native signer remote anchor: %w", err)
	}
	if err := binding.validateRemoteRecord(remote); err != nil {
		return err
	}
	if !binding.localTipMatchesRemoteRecord(local, remote) {
		return fmt.Errorf(
			"local native signer state tip differs from the authenticated remote anchor",
		)
	}
	return nil
}

func (binding *frostNativeSignerAnchorBinding) CommitNativeTBTCSignerStateTransition(
	ctx context.Context,
	operation string,
	expected frostsigning.NativeTBTCSignerStateWitnessTip,
	candidate frostsigning.NativeTBTCSignerStateWitnessTip,
) (*frostsigning.NativeTBTCSignerStateWitnessTip, error) {
	if binding == nil || ctx == nil {
		return nil, fmt.Errorf("FROST native signer anchor commit context is invalid")
	}
	if operation == "" {
		return nil, fmt.Errorf("FROST native signer operation is empty")
	}
	binding.mutex.Lock()
	defer binding.mutex.Unlock()
	if err := binding.validateLocalTip(&expected); err != nil {
		return nil, fmt.Errorf("invalid expected native signer tip: %w", err)
	}
	if err := binding.validateLocalTip(&candidate); err != nil {
		return nil, fmt.Errorf("invalid candidate native signer tip: %w", err)
	}
	headroom, err := binding.restartableRevisionHeadroom(
		expected.AnchorServiceEpoch,
		expected.AnchorRevision,
	)
	if err != nil {
		return nil, err
	}
	if headroom == 0 {
		return nil, fmt.Errorf(
			"native signer anchor certified-floor history bound is exhausted; offline anchor rotation is required",
		)
	}
	if _, err := binding.restartableGenerationHeadroom(
		candidate.Generation,
	); err != nil {
		return nil, fmt.Errorf(
			"native signer candidate exceeds the certified-floor proof bound: %w",
			err,
		)
	}
	expectedCheckpoint := frostNativeSignerCheckpointFromTip(expected)
	candidateCheckpoint := frostNativeSignerCheckpointFromTip(candidate)
	proof, err := binding.collectProofLocked(
		expectedCheckpoint,
		candidateCheckpoint,
	)
	if err != nil {
		return nil, err
	}
	result, err :=
		binding.store.CompareAndSwapFrostNativeSignerStateWitnessAnchor(
			ctx,
			expectedCheckpoint,
			candidateCheckpoint,
			proof,
		)
	if err != nil {
		return nil, err
	}
	if result == nil ||
		result.Acknowledgement.Checkpoint != candidateCheckpoint ||
		result.Acknowledgement.BindingHash != binding.bindingHash {
		return nil, fmt.Errorf(
			"native signer anchor CAS acknowledged a different binding or checkpoint",
		)
	}
	return binding.installCASResultLocked(candidate, result)
}

func (binding *frostNativeSignerAnchorBinding) collectProofLocked(
	expected FrostNativeSignerStateWitnessCheckpoint,
	candidate FrostNativeSignerStateWitnessCheckpoint,
) ([]frostsigning.NativeTBTCSignerStateWitnessProofEntry, error) {
	if candidate.Generation <= expected.Generation ||
		candidate.StoreFingerprint != expected.StoreFingerprint {
		return nil, fmt.Errorf("native signer anchor proof bounds are invalid")
	}
	capacity := FrostNativeSignerAnchorMaximumProofEntries
	generationDelta := candidate.Generation - expected.Generation
	if generationDelta < uint64(capacity) {
		capacity = int(generationDelta)
	}
	result := make(
		[]frostsigning.NativeTBTCSignerStateWitnessProofEntry,
		0,
		capacity,
	)
	cursorGeneration := expected.Generation
	cursorCommitment := expected.StateCommitment
	for page := 0; page < frostNativeSignerStateWitnessMaximumPages; page++ {
		request := &frostsigning.NativeTBTCSignerStateWitnessProofRequest{
			Schema:             frostsigning.NativeTBTCSignerStateWitnessProofRequestSchema,
			StoreFingerprint:   candidate.StoreFingerprint,
			AncestorGeneration: cursorGeneration,
			AncestorCommitment: cursorCommitment,
			TargetGeneration:   candidate.Generation,
			TargetCommitment:   candidate.StateCommitment,
			MaximumEntries:     frostsigning.NativeTBTCSignerStateWitnessProofMaximumEntries,
		}
		proof, err := binding.readProof(request)
		if err != nil {
			return nil, fmt.Errorf("cannot read native signer state-witness proof: %w", err)
		}
		if proof == nil || proof.Schema != frostsigning.NativeTBTCSignerStateWitnessProofSchema ||
			proof.StoreFingerprint != request.StoreFingerprint ||
			proof.AncestorGeneration != request.AncestorGeneration ||
			proof.AncestorCommitment != request.AncestorCommitment ||
			proof.TargetGeneration != request.TargetGeneration ||
			proof.TargetCommitment != request.TargetCommitment ||
			len(proof.Entries) == 0 ||
			len(proof.Entries) > int(request.MaximumEntries) {
			return nil, fmt.Errorf(
				"native signer returned a proof for different or empty ancestry bounds",
			)
		}
		result = append(result, proof.Entries...)
		if len(result) > FrostNativeSignerAnchorMaximumProofEntries {
			return nil, fmt.Errorf(
				"native signer state-witness ancestry exceeds the bounded proof window",
			)
		}
		last := proof.Entries[len(proof.Entries)-1]
		cursorGeneration = last.Generation
		cursorCommitment = last.StateCommitment
		if proof.Complete {
			if cursorGeneration != candidate.Generation ||
				cursorCommitment != candidate.StateCommitment ||
				last.PreviousStateCommitment != candidate.PreviousStateCommitment ||
				last.StateImageDigest != candidate.StateImageDigest {
				return nil, fmt.Errorf(
					"complete native signer proof does not reach the exact candidate checkpoint",
				)
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf(
		"native signer state-witness ancestry exceeds the bounded proof window",
	)
}

func (binding *frostNativeSignerAnchorBinding) installAcknowledgementLocked(
	candidate frostsigning.NativeTBTCSignerStateWitnessTip,
	record *FrostNativeSignerStateWitnessAnchorRecord,
) (*frostsigning.NativeTBTCSignerStateWitnessTip, error) {
	if err := binding.validateRemoteRecord(record); err != nil {
		return nil, err
	}
	if record.Checkpoint != frostNativeSignerCheckpointFromTip(candidate) {
		return nil, fmt.Errorf(
			"native signer acknowledgement does not identify the candidate checkpoint",
		)
	}
	nowUnixMillis := binding.now().UnixMilli()
	if nowUnixMillis < 0 ||
		record.AcknowledgementExpires <= uint64(nowUnixMillis) {
		return nil, fmt.Errorf(
			"native signer acknowledgement expired before it could be installed",
		)
	}
	result, err := binding.acknowledge(record.AcknowledgementJSON)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot install signed native signer checkpoint acknowledgement: %w",
			err,
		)
	}
	if result == nil ||
		result.StoreFingerprint != candidate.StoreFingerprint ||
		result.Generation != candidate.Generation ||
		result.StateCommitment != candidate.StateCommitment ||
		result.AnchorServiceEpoch != record.ServiceEpoch ||
		result.AnchorServiceRevision != record.Revision ||
		result.AnchorEventRoot != record.EventRoot ||
		result.AnchorAcknowledgementDigest != record.AcknowledgementDigest {
		return nil, fmt.Errorf(
			"native signer acknowledgement readback differs from the signed remote record",
		)
	}
	baseUnchanged := result.WitnessBaseGeneration ==
		candidate.WitnessBaseGeneration &&
		result.WitnessBaseCommitment == candidate.WitnessBaseCommitment
	baseRotatedToCandidate := result.WitnessBaseGeneration ==
		candidate.Generation &&
		result.WitnessBaseCommitment == candidate.StateCommitment
	if !baseUnchanged && !baseRotatedToCandidate {
		return nil, fmt.Errorf(
			"native signer acknowledgement rotated to an unauthenticated witness base",
		)
	}
	expected := candidate
	expected.WitnessBaseGeneration = result.WitnessBaseGeneration
	expected.WitnessBaseCommitment = result.WitnessBaseCommitment
	expected.AnchorBindingHash = binding.bindingHash
	expected.AnchorServiceEpoch = record.ServiceEpoch
	expected.AnchorRevision = record.Revision
	expected.AnchorEventRoot = record.EventRoot
	expected.AnchorAcknowledgementDigest = record.AcknowledgementDigest
	readback, err := binding.readTip()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read native signer tip after acknowledgement install: %w",
			err,
		)
	}
	if readback == nil || *readback != expected {
		return nil, fmt.Errorf(
			"native signer did not durably install the exact signed acknowledgement",
		)
	}
	binding.publishRestartableHeadroomLocked(readback)
	return readback, nil
}

func (binding *frostNativeSignerAnchorBinding) installCASResultLocked(
	candidate frostsigning.NativeTBTCSignerStateWitnessTip,
	result *FrostNativeSignerStateWitnessAnchorCASResult,
) (*frostsigning.NativeTBTCSignerStateWitnessTip, error) {
	if result == nil {
		return nil, fmt.Errorf("native signer anchor CAS result is nil")
	}
	record := frostNativeSignerAnchorRecord(&result.Acknowledgement)
	if len(record.ReadRecoveryJSON) != 0 {
		return binding.recoverAcknowledgementLocked(candidate, record)
	}
	return binding.installAcknowledgementLocked(candidate, record)
}

func (binding *frostNativeSignerAnchorBinding) recoverAcknowledgementLocked(
	candidate frostsigning.NativeTBTCSignerStateWitnessTip,
	record *FrostNativeSignerStateWitnessAnchorRecord,
) (*frostsigning.NativeTBTCSignerStateWitnessTip, error) {
	if err := binding.validateRemoteRecord(record); err != nil {
		return nil, err
	}
	if record.Checkpoint != frostNativeSignerCheckpointFromTip(candidate) ||
		len(record.ReadRecoveryJSON) == 0 {
		return nil, fmt.Errorf(
			"native signer recovery certificate does not identify the local checkpoint",
		)
	}
	nowUnixMillis := binding.now().UnixMilli()
	if nowUnixMillis < 0 ||
		record.ReadRecoveryExpires <= uint64(nowUnixMillis) {
		return nil, fmt.Errorf(
			"native signer recovery certificate expired before it could be installed",
		)
	}
	result, err := binding.recover(record.ReadRecoveryJSON)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot recover signed native signer checkpoint acknowledgement: %w",
			err,
		)
	}
	if result == nil || !result.Recovered ||
		result.StoreFingerprint != candidate.StoreFingerprint ||
		result.Generation != candidate.Generation ||
		result.StateCommitment != candidate.StateCommitment ||
		result.AnchorServiceEpoch != record.ServiceEpoch ||
		result.AnchorServiceRevision != record.Revision ||
		result.AnchorEventRoot != record.EventRoot ||
		result.AnchorAcknowledgementDigest != record.AcknowledgementDigest {
		return nil, fmt.Errorf(
			"native signer recovery readback differs from the signed remote record",
		)
	}
	baseUnchanged := result.WitnessBaseGeneration ==
		candidate.WitnessBaseGeneration &&
		result.WitnessBaseCommitment == candidate.WitnessBaseCommitment
	baseRotatedToCandidate := result.WitnessBaseGeneration ==
		candidate.Generation &&
		result.WitnessBaseCommitment == candidate.StateCommitment
	if !baseUnchanged && !baseRotatedToCandidate {
		return nil, fmt.Errorf(
			"native signer recovery rotated to an unauthenticated witness base",
		)
	}
	expected := candidate
	expected.WitnessBaseGeneration = result.WitnessBaseGeneration
	expected.WitnessBaseCommitment = result.WitnessBaseCommitment
	expected.AnchorBindingHash = binding.bindingHash
	expected.AnchorServiceEpoch = record.ServiceEpoch
	expected.AnchorRevision = record.Revision
	expected.AnchorEventRoot = record.EventRoot
	expected.AnchorAcknowledgementDigest = record.AcknowledgementDigest
	readback, err := binding.readTip()
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read native signer tip after recovery: %w",
			err,
		)
	}
	if readback == nil || *readback != expected {
		return nil, fmt.Errorf(
			"native signer did not durably recover the exact signed acknowledgement",
		)
	}
	return readback, nil
}

func (binding *frostNativeSignerAnchorBinding) validateLocalTip(
	tip *frostsigning.NativeTBTCSignerStateWitnessTip,
) error {
	if tip == nil || tip.Schema != frostsigning.NativeTBTCSignerStateWitnessTipSchema ||
		tip.StoreFingerprint != binding.identity.SignerStoreFingerprint ||
		tip.Generation == 0 || tip.StateCommitment == [32]byte{} ||
		tip.WitnessBaseGeneration == 0 ||
		tip.WitnessBaseGeneration > tip.Generation ||
		tip.WitnessBaseCommitment == [32]byte{} ||
		(tip.WitnessBaseGeneration == tip.Generation &&
			tip.WitnessBaseCommitment != tip.StateCommitment) {
		return fmt.Errorf("native signer state-witness tip is invalid or belongs to another store")
	}
	computed := frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
		tip.StoreFingerprint,
		tip.Generation,
		tip.PreviousStateCommitment,
		tip.StateImageDigest,
	)
	if computed != tip.StateCommitment {
		return fmt.Errorf("native signer state-witness tip commitment is invalid")
	}
	hasAnchor := tip.AnchorBindingHash != [32]byte{}
	if hasAnchor != (tip.AnchorServiceEpoch != 0) ||
		hasAnchor != (tip.AnchorRevision != 0) ||
		hasAnchor != (tip.AnchorEventRoot != [32]byte{}) ||
		hasAnchor != (tip.AnchorAcknowledgementDigest != [32]byte{}) {
		return fmt.Errorf("native signer state-witness anchor metadata is partial")
	}
	return nil
}

func (binding *frostNativeSignerAnchorBinding) validateRemoteRecord(
	record *FrostNativeSignerStateWitnessAnchorRecord,
) error {
	if record == nil ||
		record.BindingHash != binding.bindingHash ||
		record.Checkpoint.StoreFingerprint != binding.identity.SignerStoreFingerprint ||
		record.Checkpoint.Generation == 0 ||
		record.Checkpoint.StateCommitment == [32]byte{} ||
		record.OperationID == [32]byte{} ||
		record.TransitionDigest == [32]byte{} ||
		record.ServiceEpoch != binding.floor.ServiceEpoch ||
		record.Revision < binding.floor.Revision ||
		record.Revision-binding.floor.Revision >
			FrostNativeSignerAnchorMaximumHistoryEvents ||
		record.Checkpoint.Generation < binding.floor.Checkpoint.Generation ||
		record.Checkpoint.Generation-binding.floor.Checkpoint.Generation >
			FrostNativeSignerAnchorMaximumHistoryProofEntries ||
		record.EventRoot == [32]byte{} ||
		record.AcknowledgementDigest == [32]byte{} ||
		len(record.AcknowledgementJSON) == 0 {
		return fmt.Errorf("authenticated native signer anchor record is incomplete")
	}
	computed := frostsigning.ComputeNativeTBTCSignerStateWitnessCommitment(
		record.Checkpoint.StoreFingerprint,
		record.Checkpoint.Generation,
		record.Checkpoint.PreviousStateCommitment,
		record.Checkpoint.StateImageDigest,
	)
	if computed != record.Checkpoint.StateCommitment {
		return fmt.Errorf("authenticated native signer anchor checkpoint is invalid")
	}
	isCertifiedFloor := record.ServiceEpoch == binding.floor.ServiceEpoch &&
		record.Revision == binding.floor.Revision &&
		record.EventRoot == binding.floor.EventRoot &&
		record.AcknowledgementDigest == binding.floor.AcknowledgementDigest &&
		record.Checkpoint == binding.floor.Checkpoint
	if (isCertifiedFloor &&
		record.PreviousEventRoot != binding.floorPreviousEventRoot) ||
		(!isCertifiedFloor &&
			((record.Revision == 1 &&
				record.PreviousEventRoot != [32]byte{}) ||
				(record.Revision > 1 &&
					record.PreviousEventRoot == [32]byte{}))) {
		return fmt.Errorf(
			"authenticated native signer anchor event predecessor is invalid",
		)
	}
	return nil
}

func (binding *frostNativeSignerAnchorBinding) restartableRevisionHeadroom(
	serviceEpoch uint64,
	revision uint64,
) (uint64, error) {
	if binding == nil || serviceEpoch != binding.floor.ServiceEpoch ||
		revision < binding.floor.Revision {
		return 0, fmt.Errorf(
			"native signer anchor reference is outside its certified service-epoch floor",
		)
	}
	distance := revision - binding.floor.Revision
	if distance > FrostNativeSignerAnchorMaximumHistoryEvents {
		return 0, fmt.Errorf(
			"native signer anchor reference exceeds the restartable certified-floor history bound",
		)
	}
	return FrostNativeSignerAnchorMaximumHistoryEvents - distance, nil
}

func (binding *frostNativeSignerAnchorBinding) restartableGenerationHeadroom(
	generation uint64,
) (uint64, error) {
	if binding == nil ||
		generation < binding.floor.Checkpoint.Generation {
		return 0, fmt.Errorf(
			"native signer state generation is below its certified checkpoint floor",
		)
	}
	distance := generation - binding.floor.Checkpoint.Generation
	if distance > FrostNativeSignerAnchorMaximumHistoryProofEntries {
		return 0, fmt.Errorf(
			"native signer state generation exceeds the restartable certified-floor proof bound",
		)
	}
	return FrostNativeSignerAnchorMaximumHistoryProofEntries - distance, nil
}

// publishRestartableHeadroomLocked mirrors the headroom carried by a tip the
// anchor has just acknowledged.
//
// Without this the mirror refreshes only when a full readiness reconciliation
// succeeds, so a node that is signing steadily and a node that has stalled
// look identical at the scrape until the next reconciliation - and pre-sign
// authorization caches readiness per finality window, so an entire authorized
// batch can burn window invisibly inside one window. Every successful
// compare-and-swap already holds an authenticated tip, which is precisely when
// these numbers are both fresh and free.
//
// The rotation warning is recomputed from the seat count that travelled with
// the previous publication, because this path has authenticated headroom but
// no inventory. Seat count only changes on DKG and retirement, both of which
// force a full reconciliation that republishes it.
//
// Failure to compute either half leaves the previous mirror value standing
// rather than publishing a partial pair: the two are only meaningful together,
// and a stale pair is more honest than a half-fresh one. The caller must hold
// the binding mutex.
func (binding *frostNativeSignerAnchorBinding) publishRestartableHeadroomLocked(
	tip *frostsigning.NativeTBTCSignerStateWitnessTip,
) {
	if binding == nil || tip == nil {
		return
	}
	revisionHeadroom, err := binding.restartableRevisionHeadroom(
		tip.AnchorServiceEpoch,
		tip.AnchorRevision,
	)
	if err != nil {
		return
	}
	generationHeadroom, err := binding.restartableGenerationHeadroom(
		tip.Generation,
	)
	if err != nil {
		return
	}
	seats, _ := frostsigning.NativeTBTCSignerStateAnchorLargestLocalSeatCount()
	frostsigning.RecordNativeTBTCSignerStateAnchorRestartableHeadroom(
		revisionHeadroom,
		generationHeadroom,
		frostNativeSignerAnchorWorkloadRotationWarning(
			revisionHeadroom,
			generationHeadroom,
			seats,
		),
		seats,
	)
}

func (binding *frostNativeSignerAnchorBinding) localTipMatchesRemoteRecord(
	local frostsigning.NativeTBTCSignerStateWitnessTip,
	remote *FrostNativeSignerStateWitnessAnchorRecord,
) bool {
	return remote != nil &&
		frostNativeSignerCheckpointFromTip(local) == remote.Checkpoint &&
		local.AnchorBindingHash == binding.bindingHash &&
		local.AnchorServiceEpoch == remote.ServiceEpoch &&
		local.AnchorRevision == remote.Revision &&
		local.AnchorEventRoot == remote.EventRoot &&
		local.AnchorAcknowledgementDigest == remote.AcknowledgementDigest
}

func frostNativeSignerCheckpointFromTip(
	tip frostsigning.NativeTBTCSignerStateWitnessTip,
) FrostNativeSignerStateWitnessCheckpoint {
	return FrostNativeSignerStateWitnessCheckpoint{
		StoreFingerprint:        tip.StoreFingerprint,
		Generation:              tip.Generation,
		PreviousStateCommitment: tip.PreviousStateCommitment,
		StateImageDigest:        tip.StateImageDigest,
		StateCommitment:         tip.StateCommitment,
	}
}
