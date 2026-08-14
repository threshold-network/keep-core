package tbtc

import (
	"bytes"
	"fmt"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

func (binding *frostNativeSignerAnchorBinding) manifestFloorReference() FrostNativeSignerStateWitnessAnchorReference {
	return binding.floor
}

// validateStartupHistory independently rechecks the semantic history returned
// by the authenticated client. The client verifies both signed wrappers and
// every embedded acknowledgement; this layer proves that those signed events
// are the exact state-transition chain authorized by the offline floor.
func (binding *frostNativeSignerAnchorBinding) validateStartupHistory(
	history *FrostNativeSignerStateWitnessAnchorHistory,
) (
	map[uint64][32]byte,
	*FrostNativeSignerStateWitnessAnchorRecord,
	*FrostNativeSignerStateWitnessAnchorReference,
	error,
) {
	floor := binding.manifestFloorReference()
	if history == nil || history.FinalRead == nil ||
		history.Floor != floor {
		return nil, nil, nil, fmt.Errorf(
			"startup native signer anchor history does not begin at the exact offline floor",
		)
	}
	if err := validateFrostNativeSignerAnchorHistoryBounds(
		history.Floor,
		history.Target,
		binding.identity.SignerStoreFingerprint,
	); err != nil {
		return nil, nil, nil, fmt.Errorf(
			"startup native signer anchor history bounds are invalid: %w",
			err,
		)
	}
	remote := history.FinalRead
	if err := binding.validateRemoteRecord(remote); err != nil {
		return nil, nil, nil, err
	}
	if frostNativeSignerAnchorReferenceFromRecord(remote) != history.Target {
		return nil, nil, nil, fmt.Errorf(
			"startup native signer final Read differs from the authenticated history target",
		)
	}
	nowUnixMillis := binding.now().UnixMilli()
	if nowUnixMillis < 0 || len(remote.ReadRecoveryJSON) == 0 ||
		remote.ReadRecoveryExpires <= uint64(nowUnixMillis) {
		return nil, nil, nil, fmt.Errorf(
			"startup native signer final Read recovery certificate is absent or expired",
		)
	}
	if history.Target.Revision-history.Floor.Revision != uint64(len(history.Events)) {
		return nil, nil, nil, fmt.Errorf(
			"startup native signer anchor history event count is discontinuous",
		)
	}

	serviceCommitments := map[uint64][32]byte{
		floor.Checkpoint.Generation: floor.Checkpoint.StateCommitment,
	}
	current := floor
	var targetPrevious *FrostNativeSignerStateWitnessAnchorReference
	totalProofEntries := 0
	for index := range history.Events {
		event := &history.Events[index]
		acknowledgement := &event.Acknowledgement
		if acknowledgement.BindingHash != binding.bindingHash ||
			acknowledgement.RequestDigest == [32]byte{} ||
			acknowledgement.Nonce == [32]byte{} ||
			acknowledgement.ServiceEpoch != floor.ServiceEpoch ||
			current.Revision == ^uint64(0) ||
			acknowledgement.Revision != current.Revision+1 ||
			acknowledgement.PreviousEventRoot != current.EventRoot ||
			acknowledgement.OperationID == [32]byte{} ||
			acknowledgement.TransitionDigest == [32]byte{} ||
			len(acknowledgement.ExactAcknowledgement) == 0 {
			return nil, nil, nil, fmt.Errorf(
				"startup native signer history acknowledgement [%d] is incomplete or discontinuous",
				index,
			)
		}
		if acknowledgement.Status != "applied" &&
			acknowledgement.Status != "already-applied" {
			return nil, nil, nil, fmt.Errorf(
				"startup native signer history acknowledgement [%d] has an invalid status",
				index,
			)
		}
		if computeFrostNativeSignerAnchorEventRoot(*acknowledgement) !=
			acknowledgement.EventRoot {
			return nil, nil, nil, fmt.Errorf(
				"startup native signer history acknowledgement [%d] has an invalid event root",
				index,
			)
		}
		computedAcknowledgementDigest :=
			computeFrostNativeSignerCheckpointAcknowledgementDigest(
				acknowledgement.SigningDigest,
				acknowledgement.Signature,
				binding.identity.OnlineKeyHash,
			)
		if computedAcknowledgementDigest !=
			acknowledgement.AcknowledgementDigest {
			return nil, nil, nil, fmt.Errorf(
				"startup native signer history acknowledgement [%d] has an invalid digest",
				index,
			)
		}
		totalProofEntries += len(event.WitnessProof)
		if totalProofEntries > FrostNativeSignerAnchorMaximumHistoryProofEntries {
			return nil, nil, nil, fmt.Errorf(
				"startup native signer history proof exceeds its aggregate bound",
			)
		}
		if err := validateFrostNativeSignerAnchorTransition(
			current.Checkpoint,
			acknowledgement.Checkpoint,
			event.WitnessProof,
			binding.identity.SignerStoreFingerprint,
		); err != nil {
			return nil, nil, nil, fmt.Errorf(
				"startup native signer history transition [%d] is invalid: %w",
				index,
				err,
			)
		}
		expectedTransitionDigest :=
			computeFrostNativeSignerAnchorTransitionDigest(
				binding.identity,
				acknowledgement.OperationID,
				current.Checkpoint,
				acknowledgement.Checkpoint,
				event.WitnessProof,
			)
		if acknowledgement.TransitionDigest != expectedTransitionDigest {
			return nil, nil, nil, fmt.Errorf(
				"startup native signer history transition [%d] digest mismatch",
				index,
			)
		}
		for _, entry := range event.WitnessProof {
			if _, exists := serviceCommitments[entry.Generation]; exists {
				return nil, nil, nil, fmt.Errorf(
					"startup native signer history repeats state generation [%d]",
					entry.Generation,
				)
			}
			serviceCommitments[entry.Generation] = entry.StateCommitment
		}
		previous := current
		targetPrevious = &previous
		current = frostNativeSignerAnchorReferenceFromAcknowledgement(
			acknowledgement,
		)
	}
	if current != history.Target {
		return nil, nil, nil, fmt.Errorf(
			"startup native signer anchor history does not reach its exact target",
		)
	}
	if len(history.Events) > 0 {
		last := &history.Events[len(history.Events)-1].Acknowledgement
		if !binding.remoteRecordMatchesAcknowledgement(remote, last) {
			return nil, nil, nil, fmt.Errorf(
				"startup native signer final Read differs from the final history acknowledgement",
			)
		}
	}
	return serviceCommitments, remote, targetPrevious, nil
}

// validateLocalHistorySplice proves the local witness from a commitment that
// also appears in the independently authenticated service chain. If Rust has
// pruned pre-base records, the service history supplies floor→base and Rust
// supplies base→tip; otherwise Rust proves floor→tip directly.
func (binding *frostNativeSignerAnchorBinding) validateLocalHistorySplice(
	local frostsigning.NativeTBTCSignerStateWitnessTip,
	serviceCommitments map[uint64][32]byte,
) error {
	floor := binding.floor.Checkpoint
	var splice FrostNativeSignerStateWitnessCheckpoint
	if local.WitnessBaseGeneration <= floor.Generation {
		splice = floor
	} else {
		serviceCommitment, ok :=
			serviceCommitments[local.WitnessBaseGeneration]
		if !ok || serviceCommitment != local.WitnessBaseCommitment {
			return fmt.Errorf(
				"local native signer witness base is absent from the authenticated service history",
			)
		}
		splice = FrostNativeSignerStateWitnessCheckpoint{
			StoreFingerprint: binding.identity.SignerStoreFingerprint,
			Generation:       local.WitnessBaseGeneration,
			StateCommitment:  local.WitnessBaseCommitment,
		}
	}
	localCheckpoint := frostNativeSignerCheckpointFromTip(local)
	if splice.Generation == localCheckpoint.Generation {
		if splice.StateCommitment != localCheckpoint.StateCommitment {
			return fmt.Errorf(
				"local native signer witness splice commitment differs at equal generation",
			)
		}
		return nil
	}
	if splice.Generation > localCheckpoint.Generation {
		return fmt.Errorf(
			"local native signer witness splice is ahead of the local tip",
		)
	}
	if _, err := binding.collectProofLocked(splice, localCheckpoint); err != nil {
		return fmt.Errorf(
			"cannot prove local native signer witness ancestry from the authenticated service chain: %w",
			err,
		)
	}
	return nil
}

func frostNativeSignerAnchorReferenceFromRecord(
	record *FrostNativeSignerStateWitnessAnchorRecord,
) FrostNativeSignerStateWitnessAnchorReference {
	if record == nil {
		return FrostNativeSignerStateWitnessAnchorReference{}
	}
	return FrostNativeSignerStateWitnessAnchorReference{
		ServiceEpoch:          record.ServiceEpoch,
		Revision:              record.Revision,
		EventRoot:             record.EventRoot,
		AcknowledgementDigest: record.AcknowledgementDigest,
		Checkpoint:            record.Checkpoint,
	}
}

func (binding *frostNativeSignerAnchorBinding) localTipHasAnchorReference(
	local frostsigning.NativeTBTCSignerStateWitnessTip,
	reference FrostNativeSignerStateWitnessAnchorReference,
) bool {
	return local.AnchorBindingHash == binding.bindingHash &&
		local.AnchorServiceEpoch == reference.ServiceEpoch &&
		local.AnchorRevision == reference.Revision &&
		local.AnchorEventRoot == reference.EventRoot &&
		local.AnchorAcknowledgementDigest == reference.AcknowledgementDigest
}

func (binding *frostNativeSignerAnchorBinding) localTipHasNoAnchor(
	local frostsigning.NativeTBTCSignerStateWitnessTip,
) bool {
	return local.AnchorBindingHash == [32]byte{} &&
		local.AnchorServiceEpoch == 0 &&
		local.AnchorRevision == 0 &&
		local.AnchorEventRoot == [32]byte{} &&
		local.AnchorAcknowledgementDigest == [32]byte{}
}

func (binding *frostNativeSignerAnchorBinding) remoteRecordMatchesAcknowledgement(
	record *FrostNativeSignerStateWitnessAnchorRecord,
	acknowledgement *FrostNativeSignerCheckpointAcknowledgement,
) bool {
	return record != nil && acknowledgement != nil &&
		record.Checkpoint == acknowledgement.Checkpoint &&
		record.BindingHash == acknowledgement.BindingHash &&
		record.AcknowledgementDigest ==
			acknowledgement.AcknowledgementDigest &&
		record.OperationID == acknowledgement.OperationID &&
		record.TransitionDigest == acknowledgement.TransitionDigest &&
		record.ServiceEpoch == acknowledgement.ServiceEpoch &&
		record.Revision == acknowledgement.Revision &&
		record.PreviousEventRoot == acknowledgement.PreviousEventRoot &&
		record.EventRoot == acknowledgement.EventRoot &&
		bytes.Equal(
			record.AcknowledgementJSON,
			acknowledgement.ExactAcknowledgement,
		)
}
