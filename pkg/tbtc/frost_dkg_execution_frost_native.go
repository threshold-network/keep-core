//go:build frost_native

package tbtc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/btcsuite/btcd/btcec/v2"
	"go.uber.org/zap"

	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/frost/registry"
	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/net"
	protocolannouncer "github.com/keep-network/keep-core/pkg/protocol/announcer"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
)

const frostDKGResultSubmissionDelayStepBlocks = 30

var errFrostDKGSubmissionOutcomeUncertain = errors.New(
	"FROST DKG submission outcome is uncertain",
)

func executeFrostDKGIfPossible(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	event *FrostDKGStartedEvent,
	memberIndexes []group.MemberIndex,
	groupSelectionResult *GroupSelectionResult,
) bool {
	nativeTBTCSignerEngine := frostsigning.CurrentNativeTBTCSignerEngine()
	if nativeTBTCSignerEngine == nil {
		logger.Infof(
			"FROST DKG with seed [0x%x] selected this operator as member "+
				"indexes [%v], but no native tbtc-signer engine is registered",
			event.Seed,
			memberIndexes,
		)
		return false
	}
	if _, ok := nativeTBTCSignerEngine.(frostsigning.NativeTBTCSignerDistributedDKGRetirementEngine); !ok {
		logger.Errorf(
			"FROST DKG with seed [0x%x] selected this operator, but the native "+
				"tbtc-signer engine cannot durably retire partially persisted DKG packages",
			event.Seed,
		)
		return false
	}

	// Distributed DKG produces signing material usable ONLY via the complete
	// interactive ROAST path. The interactive audit flag alone is insufficient:
	// without the ROAST readiness opt-in or a build containing the transition
	// producer, orchestration takes its static fallback and reaches the removed
	// coarse primitive. Refuse to run rather than create an unsignable wallet.
	if !frostsigning.InteractiveSigningReady() {
		logger.Errorf(
			"FROST DKG with seed [0x%x] selected this operator, but the distributed "+
				"DKG requires the complete interactive ROAST signing path (%s=true, "+
				"%s=true, a frost_native+frost_roast_retry build, and a registered "+
				"interactive engine); refusing to run to avoid creating a wallet that "+
				"cannot sign",
			event.Seed,
			frostsigning.InteractiveSigningOptInEnvVar,
			frostsigning.RoastRetryReadinessOptInEnvVar,
		)
		return false
	}

	membershipValidator := group.NewMembershipValidator(
		logger,
		groupSelectionResult.OperatorsAddresses,
		node.chain.Signing(),
	)

	channelName := fmt.Sprintf("%s-frost-dkg-%s", ProtocolName, event.Seed.Text(16))
	channel, err := node.netProvider.BroadcastChannelFor(channelName)
	if err != nil {
		logger.Errorf("failed to get FROST DKG broadcast channel: [%v]", err)
		return false
	}

	registerFrostDKGResultSigningUnmarshaller(channel)
	protocolannouncer.RegisterUnmarshaller(channel)

	if err := channel.SetFilter(membershipValidator.IsInGroup); err != nil {
		logger.Errorf("failed to set FROST DKG broadcast channel filter: [%v]", err)
		return false
	}

	params, err := frostChain.FrostDKGParameters()
	if err != nil {
		logger.Errorf("failed to get FROST DKG parameters: [%v]", err)
		return false
	}
	if params == nil {
		logger.Errorf("FROST DKG parameters are nil")
		return false
	}

	signatureThreshold, err := frostDKGSignatureThreshold(
		node.frostGroupParameters,
	)
	if err != nil {
		logger.Errorf("invalid FROST DKG group parameters: [%v]", err)
		return false
	}

	fullMembers := frostFullMembers(groupSelectionResult)
	dkgTimeoutBlock := event.BlockNumber + params.SubmissionTimeoutBlocks

	go func() {
		dkgLogger := logger.With(
			zap.String("seed", fmt.Sprintf("0x%x", event.Seed)),
			zap.String("memberIndexes", fmt.Sprintf("%v", memberIndexes)),
		)

		node.protocolLatch.Lock()
		defer node.protocolLatch.Unlock()

		dkgCtx, cancelDkgCtx := withCancelOnBlock(
			ctx,
			dkgTimeoutBlock,
			node.waitForBlockHeight,
		)
		defer cancelDkgCtx()

		sessionID := fmt.Sprintf("%s-%s", channelName, "attempt-1")

		var dkgPrebuffer *frostsigning.DKGMessagePrebuffer
		announceReadiness := func() (
			[]group.MemberIndex,
			registry.MisbehavedMemberIndices,
			error,
		) {
			// Capture DKG round messages off the channel BEFORE the
			// readiness barrier below. The reservation is deliberately
			// acquired before this callback, so no local seat can announce
			// readiness unless worst-case anchor capacity is already held.
			//
			// announceFrostDKGReadiness releases every peer once the
			// quorum announces, but a node installs its DKG receiver only
			// later, inside executeDistributedFrostDKG. A peer released
			// ahead of a slower node can broadcast round-1 before that
			// node is receiving; the transport would drop it (no
			// subscriber) and not retransmit, stalling the DKG. The
			// prebuffer catches those messages so they are replayed once
			// the receiver is up.
			dkgPrebuffer = frostsigning.StartDKGMessagePrebuffer(
				dkgCtx,
				channel,
			)
			return announceFrostDKGReadiness(
				dkgCtx,
				node,
				channel,
				membershipValidator,
				fmt.Sprintf("%v-%v", ProtocolName, "frost-dkg"),
				sessionID,
				memberIndexes,
				len(groupSelectionResult.OperatorsIDs),
			)
		}
		readinessAdmission, err := reserveAndAnnounceFrostDKGReadiness(
			dkgCtx,
			node.frostNativeSignerAnchorAdmission,
			memberIndexes,
			announceReadiness,
		)
		if err != nil {
			dkgLogger.Errorf("FROST DKG admission or readiness failed: [%v]", err)
			return
		}
		defer readinessAdmission.anchorReservation.Release()
		activeMemberIndexes := readinessAdmission.activeMemberIndexes
		misbehavedMembersIndices :=
			readinessAdmission.misbehavedMembersIndices

		localActiveMemberIndexes := localActiveFrostMemberIndexes(
			memberIndexes,
			activeMemberIndexes,
		)
		submitterMemberIndex := lowestLocalActiveMemberIndex(
			localActiveMemberIndexes,
			activeMemberIndexes,
		)
		if submitterMemberIndex == 0 {
			dkgLogger.Infof(
				"skipping FROST DKG result assembly; no local member "+
					"index is active in [%v]",
				activeMemberIndexes,
			)
			return
		}
		tbtcSignerMemberIndexes, err := finalFrostDKGMemberIndexes(
			activeMemberIndexes,
			groupSelectionResult,
			node.frostGroupParameters,
		)
		if err != nil {
			dkgLogger.Errorf("failed to resolve final FROST DKG member indexes: [%v]", err)
			return
		}

		executionResult, err := executeDistributedFrostDKG(
			dkgCtx,
			nativeTBTCSignerEngine,
			node,
			channel,
			activeMemberIndexes,
			tbtcSignerMemberIndexes,
			localActiveMemberIndexes,
			groupSelectionResult,
			signatureThreshold,
			sessionID,
			dkgPrebuffer,
		)
		if err != nil {
			dkgLogger.Errorf("FROST DKG execution failed: [%v]", err)
			return
		}
		// From this point every local key package is durable. Do not retire it
		// from any error exit below using latest-state DKG or registration reads:
		// replicas can observe those values at different chain points. Only
		// reconciliation rooted in finalized retained history may prove that the
		// attempt resolved without this wallet and retire the material.

		for _, localMemberIndex := range localActiveMemberIndexes {
			if err := registerFrostSignerWithMaterial(
				node,
				executionResult.outputKey,
				executionResult.signerMaterial,
				localMemberIndex,
				activeMemberIndexes,
				groupSelectionResult,
			); err != nil {
				dkgLogger.Errorf("failed to register FROST signer: [%v]", err)
				return
			}
		}

		unsignedResult, err := registry.AssembleResult(
			uint64(submitterMemberIndex),
			executionResult.outputKey,
			fullMembers,
			misbehavedMembersIndices,
			nil,
			nil,
		)
		if err != nil {
			dkgLogger.Errorf("failed to assemble unsigned FROST DKG result: [%v]", err)
			return
		}

		signedResult, err := signAndCollectFrostDKGResultSignatures(
			dkgCtx,
			node,
			frostChain,
			channel,
			membershipValidator,
			sessionID,
			event.Seed,
			localActiveMemberIndexes,
			activeMemberIndexes,
			groupSelectionResult,
			unsignedResult,
		)
		if err != nil {
			dkgLogger.Errorf("failed to collect FROST DKG result signatures: [%v]", err)
			return
		}

		valid, reason, err := frostChain.IsFrostDKGResultValid(signedResult)
		if err != nil {
			dkgLogger.Errorf("failed to pre-validate FROST DKG result: [%v]", err)
			return
		}
		if !valid {
			dkgLogger.Errorf("assembled FROST DKG result is invalid: [%s]", reason)
			return
		}

		if err := submitFrostDKGResultWithDelay(
			dkgCtx,
			node,
			frostChain,
			submitterMemberIndex,
			activeMemberIndexes,
			signedResult,
		); err != nil {
			dkgLogger.Errorf("failed to submit FROST DKG result: [%v]", err)
			return
		}
	}()

	return true
}

type frostDKGReadinessAdmission struct {
	anchorReservation        *frostNativeSignerAnchorRevisionReservation
	activeMemberIndexes      []group.MemberIndex
	misbehavedMembersIndices registry.MisbehavedMemberIndices
}

func reserveAndAnnounceFrostDKGReadiness(
	ctx context.Context,
	anchorAdmission *frostNativeSignerAnchorAdmissionController,
	localMemberIndexes []group.MemberIndex,
	announce func() (
		[]group.MemberIndex,
		registry.MisbehavedMemberIndices,
		error,
	),
) (
	*frostDKGReadinessAdmission,
	error,
) {
	if anchorAdmission == nil {
		return nil, fmt.Errorf(
			"native signer anchor admission is unavailable",
		)
	}
	if announce == nil {
		return nil, fmt.Errorf(
			"FROST DKG readiness announcement is unavailable",
		)
	}

	// Every selected local seat may become active once readiness is announced.
	// Reserve that worst-case persistence cost before invoking the announcer.
	anchorReservation, err := anchorAdmission.reserveDKG(
		ctx,
		uint64(len(localMemberIndexes)),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"native signer anchor admission failed: [%w]",
			err,
		)
	}

	activeMemberIndexes, misbehavedMembersIndices, err := announce()
	if err != nil {
		anchorReservation.Release()
		return nil, fmt.Errorf(
			"readiness announcement failed: [%w]",
			err,
		)
	}

	return &frostDKGReadinessAdmission{
		anchorReservation:        anchorReservation,
		activeMemberIndexes:      activeMemberIndexes,
		misbehavedMembersIndices: misbehavedMembersIndices,
	}, nil
}

type frostDKGExecutionResult struct {
	outputKey      frost.OutputKey
	signerMaterial *frostsigning.NativeSignerMaterial
}

// executeDistributedFrostDKG runs a real distributed FROST DKG for this node's
// local seats over the wallet broadcast channel, persists each seat's key package
// as signing material, and returns the shared group output key plus the signer
// material (the same for every local seat; the differing secret key packages live
// in the engine's per-seat session store). It is the only DKG path: the
// transitional trusted-dealer seeded DKG has been removed with the coarse path.
func executeDistributedFrostDKG(
	dkgCtx context.Context,
	nativeEngine frostsigning.NativeTBTCSignerEngine,
	node *node,
	channel net.BroadcastChannel,
	activeMemberIndexes []group.MemberIndex,
	tbtcSignerMemberIndexes []group.MemberIndex,
	localActiveMemberIndexes []group.MemberIndex,
	groupSelectionResult *GroupSelectionResult,
	signatureThreshold int,
	sessionID string,
	prebuffer *frostsigning.DKGMessagePrebuffer,
) (*frostDKGExecutionResult, error) {
	if nativeEngine == nil {
		return nil, fmt.Errorf("native tbtc-signer engine is unavailable")
	}
	distributedEngine, ok := nativeEngine.(frostsigning.NativeTBTCSignerDistributedDKGEngine)
	if !ok {
		return nil, fmt.Errorf("native tbtc-signer engine does not support distributed DKG")
	}
	if signatureThreshold <= 0 || signatureThreshold > int(^uint16(0)) {
		return nil, fmt.Errorf("invalid tbtc-signer DKG threshold [%d]", signatureThreshold)
	}

	// Canonical FROST identifiers over the FULL participant set (the final compact
	// DKG member space), matching what the persist op and the signing path expect.
	identifierByID := make(map[group.MemberIndex]string, len(tbtcSignerMemberIndexes))
	for _, memberIndex := range tbtcSignerMemberIndexes {
		identifierByID[memberIndex] = frostsigning.CanonicalFROSTIdentifier(uint16(memberIndex))
	}

	// Remap this node's local seats from the ORIGINAL sortition space to the FINAL
	// compact DKG member space the runner and persist op operate in (the same
	// mapping registerFrostSignerWithMaterial uses). finalSigningGroup sorts its
	// operating-members argument in place, so pass a copy.
	finalSigningGroupOperators, finalSigningGroupMembersIndexes, err := finalSigningGroup(
		groupSelectionResult.OperatorsAddresses,
		append([]group.MemberIndex{}, activeMemberIndexes...),
		node.frostGroupParameters,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve the final signing group: [%v]", err)
	}
	localDKGMemberIndexes := make([]group.MemberIndex, 0, len(localActiveMemberIndexes))
	for _, localSeat := range localActiveMemberIndexes {
		finalSeat, ok := finalSigningGroupMembersIndexes[localSeat]
		if !ok {
			return nil, fmt.Errorf("local seat [%v] is missing from the final signing group", localSeat)
		}
		localDKGMemberIndexes = append(localDKGMemberIndexes, finalSeat)
	}

	// The runner broadcasts FINAL compact member indexes as the transport sender,
	// so the DKG bus must authenticate them against a validator indexed by the SAME
	// final space - not the original sortition-ordered membership (which would
	// reject shifted seats when readiness compacts the active set, stalling the
	// DKG). finalSigningGroupOperators[i] is the operator for final member i+1.
	finalMembershipValidator := group.NewMembershipValidator(
		logger,
		finalSigningGroupOperators,
		node.chain.Signing(),
	)

	// Each seat's per-DKG round-2 sealing key is a fresh ephemeral generated inside
	// the orchestration (not this node's operator key), so operator-key material
	// never reaches the runner; the operator key stays bound to the channel, which
	// authenticates every seat's round-1 broadcast (and the ephemeral key riding
	// in it) via finalMembershipValidator.
	persistBySeat, err := frostsigning.RunDistributedDKGForSeats(
		dkgCtx,
		logger,
		channel,
		finalMembershipValidator,
		distributedEngine,
		sessionID,
		tbtcSignerMemberIndexes,
		localDKGMemberIndexes,
		identifierByID,
		uint16(signatureThreshold),
		prebuffer,
	)
	if err != nil {
		if retirementErr := retirePersistedFrostDKGKeyGroups(
			nativeEngine,
			persistBySeat,
		); retirementErr != nil {
			return nil, fmt.Errorf("%w; %v", err, retirementErr)
		}
		return nil, err
	}

	// Every local seat shares the same group key; build the output key + material
	// once from any persisted result.
	var persisted *frostsigning.NativeTBTCSignerDKGResult
	for _, seatResult := range persistBySeat {
		persisted = seatResult
		break
	}
	if persisted == nil {
		return nil, fmt.Errorf("distributed DKG produced no persisted result")
	}

	outputKey, err := outputKeyFromTBTCSignerDKGResult(persisted)
	if err != nil {
		return nil, err
	}

	// A populated participant list + threshold are required for dkg-persisted
	// signer material; there is no dealer seed for a distributed DKG.
	participants, err := nativeTBTCSignerDKGParticipants(tbtcSignerMemberIndexes)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(frostsigning.NativeTBTCSignerMaterialPayload{
		KeyGroup:         persisted.KeyGroup,
		TaprootOutputKey: hex.EncodeToString(outputKey[:]),
		KeyGroupSource:   frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
		DKGParticipants:  participants,
		DKGThreshold:     uint16(signatureThreshold),
	})
	if err != nil {
		return nil, fmt.Errorf("cannot marshal tbtc-signer material: [%w]", err)
	}

	return &frostDKGExecutionResult{
		outputKey: outputKey,
		signerMaterial: &frostsigning.NativeSignerMaterial{
			Format:  frostsigning.NativeSignerMaterialFormatFrostTBTCSignerV1,
			Payload: payload,
		},
	}, nil
}

func finalFrostDKGMemberIndexes(
	activeMemberIndexes []group.MemberIndex,
	groupSelectionResult *GroupSelectionResult,
	groupParameters *GroupParameters,
) ([]group.MemberIndex, error) {
	if groupSelectionResult == nil {
		return nil, fmt.Errorf("group selection result is nil")
	}
	if groupParameters == nil {
		return nil, fmt.Errorf("group parameters are nil")
	}

	operatingMembersIndexes := append([]group.MemberIndex{}, activeMemberIndexes...)
	_, finalSigningGroupMembersIndexes, err := finalSigningGroup(
		groupSelectionResult.OperatorsAddresses,
		operatingMembersIndexes,
		groupParameters,
	)
	if err != nil {
		return nil, err
	}

	dkgMemberIndexes := make(
		[]group.MemberIndex,
		0,
		len(operatingMembersIndexes),
	)
	for _, activeMemberIndex := range operatingMembersIndexes {
		finalMemberIndex, ok :=
			finalSigningGroupMembersIndexes[activeMemberIndex]
		if !ok {
			return nil, fmt.Errorf(
				"active member [%d] is missing final FROST DKG member index",
				activeMemberIndex,
			)
		}

		dkgMemberIndexes = append(dkgMemberIndexes, finalMemberIndex)
	}

	return dkgMemberIndexes, nil
}

func nativeTBTCSignerDKGParticipants(
	activeMemberIndexes []group.MemberIndex,
) ([]frostsigning.NativeTBTCSignerDKGParticipant, error) {
	participants := make(
		[]frostsigning.NativeTBTCSignerDKGParticipant,
		0,
		len(activeMemberIndexes),
	)

	for _, memberIndex := range activeMemberIndexes {
		if memberIndex == 0 {
			return nil, fmt.Errorf(
				"invalid tbtc-signer DKG member index [%d]",
				memberIndex,
			)
		}

		identifier := uint16(memberIndex)
		participants = append(
			participants,
			frostsigning.NativeTBTCSignerDKGParticipant{
				Identifier: identifier,
				PublicKeyHex: frostsigning.
					NativeTBTCSignerDKGPlaceholderPublicKeyHex(identifier),
			},
		)
	}

	return participants, nil
}

func outputKeyFromTBTCSignerDKGResult(
	dkgResult *frostsigning.NativeTBTCSignerDKGResult,
) (frost.OutputKey, error) {
	if dkgResult == nil {
		return frost.OutputKey{}, fmt.Errorf("tbtc-signer DKG result is nil")
	}
	if dkgResult.KeyGroup == "" {
		return frost.OutputKey{}, fmt.Errorf("tbtc-signer DKG key group is empty")
	}

	outputKeyBytes, err := frostsigning.TaprootOutputKeyFromTBTCSignerKey(
		dkgResult.KeyGroup,
	)
	if err != nil {
		return frost.OutputKey{}, fmt.Errorf(
			"cannot derive tbtc-signer DKG Taproot output key: [%w]",
			err,
		)
	}
	if len(outputKeyBytes) != frost.OutputKeySize {
		return frost.OutputKey{}, fmt.Errorf(
			"unexpected tbtc-signer DKG output key length [%d]",
			len(outputKeyBytes),
		)
	}

	var outputKey frost.OutputKey
	copy(outputKey[:], outputKeyBytes)

	return outputKey, nil
}

func announceFrostDKGReadiness(
	ctx context.Context,
	node *node,
	channel net.BroadcastChannel,
	membershipValidator *group.MembershipValidator,
	protocolID string,
	sessionID string,
	memberIndexes []group.MemberIndex,
	groupSize int,
) (
	[]group.MemberIndex,
	registry.MisbehavedMemberIndices,
	error,
) {
	blockCounter, err := node.chain.BlockCounter()
	if err != nil {
		return nil, nil, err
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return nil, nil, err
	}

	announcementEndBlock := currentBlock + dkgAttemptAnnouncementActiveBlocks
	announceCtx, cancelAnnounceCtx := withCancelOnBlock(
		ctx,
		announcementEndBlock,
		node.waitForBlockHeight,
	)
	defer cancelAnnounceCtx()

	announcer := protocolannouncer.New(protocolID, channel, membershipValidator)
	activeMemberIndexes, err := announcer.AnnounceMany(
		announceCtx,
		memberIndexes,
		sessionID,
	)
	if err != nil {
		return nil, nil, err
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	if len(activeMemberIndexes) < node.frostGroupParameters.GroupQuorum {
		return nil, nil, fmt.Errorf(
			"FROST DKG readiness quorum not reached: [%d] active members, quorum [%d]",
			len(activeMemberIndexes),
			node.frostGroupParameters.GroupQuorum,
		)
	}

	return activeMemberIndexes,
		frostMisbehavedMemberIndices(groupSize, activeMemberIndexes),
		nil
}

func localActiveFrostMemberIndexes(
	localMemberIndexes []group.MemberIndex,
	activeMemberIndexes []group.MemberIndex,
) []group.MemberIndex {
	activeMembersSet := make(
		map[group.MemberIndex]struct{},
		len(activeMemberIndexes),
	)
	for _, activeMemberIndex := range activeMemberIndexes {
		activeMembersSet[activeMemberIndex] = struct{}{}
	}

	localActiveMemberIndexes := make(
		[]group.MemberIndex,
		0,
		len(localMemberIndexes),
	)
	for _, localMemberIndex := range localMemberIndexes {
		if _, ok := activeMembersSet[localMemberIndex]; ok {
			localActiveMemberIndexes = append(
				localActiveMemberIndexes,
				localMemberIndex,
			)
		}
	}

	return localActiveMemberIndexes
}

func registerFrostSignerWithMaterial(
	node *node,
	outputKey frost.OutputKey,
	signerMaterial *frostsigning.NativeSignerMaterial,
	memberIndex group.MemberIndex,
	activeMemberIndexes []group.MemberIndex,
	groupSelectionResult *GroupSelectionResult,
) error {
	if signerMaterial == nil {
		return fmt.Errorf("FROST signer material is nil")
	}

	walletPublicKey, err := frostOutputKeyToECDSAPublicKey(outputKey)
	if err != nil {
		return err
	}

	finalSigningGroupOperators, finalSigningGroupMembersIndexes, err :=
		finalSigningGroup(
			groupSelectionResult.OperatorsAddresses,
			append([]group.MemberIndex{}, activeMemberIndexes...),
			node.frostGroupParameters,
		)
	if err != nil {
		return fmt.Errorf("failed to resolve final FROST signing group members: [%w]", err)
	}

	finalSigningGroupMemberIndex, ok :=
		finalSigningGroupMembersIndexes[memberIndex]
	if !ok {
		return fmt.Errorf("failed to resolve final FROST signing group member index")
	}

	return node.walletRegistry.registerSigner(newSigner(
		walletPublicKey,
		finalSigningGroupOperators,
		finalSigningGroupMemberIndex,
		nil,
		signerMaterial,
	))
}

func submitFrostDKGResultWithDelay(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	memberIndex group.MemberIndex,
	activeMemberIndexes []group.MemberIndex,
	result *registry.Result,
) error {
	rank := -1
	for i, activeMemberIndex := range activeMemberIndexes {
		if activeMemberIndex == memberIndex {
			rank = i
			break
		}
	}
	if rank < 0 {
		return fmt.Errorf(
			"FROST DKG submitter member [%d] is not in active members [%v]",
			memberIndex,
			activeMemberIndexes,
		)
	}

	blockCounter, err := node.chain.BlockCounter()
	if err != nil {
		return err
	}

	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return err
	}

	submissionBlock := currentBlock +
		uint64(rank)*frostDKGResultSubmissionDelayStepBlocks
	if err := node.waitForBlockHeight(ctx, submissionBlock); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	state, err := frostChain.GetFrostDKGState()
	if err != nil {
		return err
	}
	if state != AwaitingResult {
		matches, matchErr := currentFrostDKGResultMatches(frostChain, result)
		if matchErr != nil {
			return fmt.Errorf(
				"cannot verify the result that advanced FROST DKG state to [%v]: [%w]",
				state,
				matchErr,
			)
		}
		if matches {
			return nil
		}
		return fmt.Errorf(
			"FROST DKG state advanced to [%v] without this wallet result",
			state,
		)
	}

	if err := frostChain.SubmitFrostDKGResult(result); err != nil {
		matches, matchErr := currentFrostDKGResultMatches(frostChain, result)
		if matchErr == nil && matches {
			return nil
		}
		if matchErr != nil {
			return fmt.Errorf(
				"%w: submission failed [%v] and its canonical outcome "+
					"could not be verified: [%w]",
				errFrostDKGSubmissionOutcomeUncertain,
				err,
				matchErr,
			)
		}
		stateAfterSubmission, stateErr := frostChain.GetFrostDKGState()
		if stateErr != nil || stateAfterSubmission == AwaitingResult {
			return fmt.Errorf(
				"%w: submission failed: [%v]",
				errFrostDKGSubmissionOutcomeUncertain,
				err,
			)
		}
		return err
	}
	return nil
}

func currentFrostDKGResultMatches(
	frostChain FrostDKGChain,
	expected *registry.Result,
) (bool, error) {
	if frostChain == nil || expected == nil {
		return false, fmt.Errorf("FROST DKG result comparison dependencies are nil")
	}
	state, err := frostChain.GetFrostDKGState()
	if err != nil {
		return false, err
	}
	if state != Challenge {
		return false, nil
	}
	events, err := frostChain.PastFrostDKGResultSubmittedEvents(nil)
	if err != nil {
		return false, err
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
	if latest == nil || !sameFrostDKGWalletResult(latest.Result, expected) {
		return false, nil
	}
	valid, _, err := frostChain.IsFrostDKGResultValid(latest.Result)
	if err != nil {
		return false, err
	}
	return valid, nil
}

func sameFrostDKGWalletResult(first, second *registry.Result) bool {
	return first != nil &&
		second != nil &&
		first.XOnlyOutputKey == second.XOnlyOutputKey &&
		first.MembersHash == second.MembersHash &&
		slices.Equal(first.Members, second.Members) &&
		slices.Equal(
			first.MisbehavedMembersIndices,
			second.MisbehavedMembersIndices,
		)
}

func retirePersistedFrostDKGKeyGroups(
	nativeEngine frostsigning.NativeTBTCSignerEngine,
	persistBySeat map[group.MemberIndex]*frostsigning.NativeTBTCSignerDKGResult,
) error {
	keyGroupSet := make(map[string]struct{})
	for _, persisted := range persistBySeat {
		if persisted == nil || persisted.KeyGroup == "" {
			continue
		}
		keyGroupSet[persisted.KeyGroup] = struct{}{}
	}
	if len(keyGroupSet) == 0 {
		return nil
	}

	retirementEngine, ok :=
		nativeEngine.(frostsigning.NativeTBTCSignerDistributedDKGRetirementEngine)
	if !ok {
		return fmt.Errorf(
			"native signer cannot retire partially persisted DKG material",
		)
	}

	keyGroups := make([]string, 0, len(keyGroupSet))
	for keyGroup := range keyGroupSet {
		keyGroups = append(keyGroups, keyGroup)
	}
	slices.Sort(keyGroups)

	retirementErrors := make([]error, 0)
	for _, keyGroup := range keyGroups {
		if err := retirementEngine.
			RetireDistributedDKGKeyPackages(keyGroup); err != nil {
			retirementErrors = append(
				retirementErrors,
				fmt.Errorf(
					"cannot retire partially persisted DKG key group [%s]: [%w]",
					keyGroup,
					err,
				),
			)
		}
	}
	return errors.Join(retirementErrors...)
}

func frostOutputKeyToECDSAPublicKey(
	outputKey frost.OutputKey,
) (*ecdsa.PublicKey, error) {
	compressed := make([]byte, 0, 1+frost.OutputKeySize)
	compressed = append(compressed, byte(0x02))
	compressed = append(compressed, outputKey[:]...)

	publicKey, err := btcec.ParsePubKey(compressed)
	if err != nil {
		return nil, fmt.Errorf("cannot lift x-only FROST output key: [%w]", err)
	}

	return &ecdsa.PublicKey{
		Curve: tecdsa.Curve,
		X:     publicKey.X(),
		Y:     publicKey.Y(),
	}, nil
}

func frostFullMembers(
	groupSelectionResult *GroupSelectionResult,
) registry.FullMembers {
	members := make(registry.FullMembers, len(groupSelectionResult.OperatorsIDs))
	for i, operatorID := range groupSelectionResult.OperatorsIDs {
		members[i] = uint32(operatorID)
	}

	return members
}

func lowestLocalActiveMemberIndex(
	localMemberIndexes []group.MemberIndex,
	activeMemberIndexes []group.MemberIndex,
) group.MemberIndex {
	activeMembersSet := make(
		map[group.MemberIndex]struct{},
		len(activeMemberIndexes),
	)
	for _, activeMemberIndex := range activeMemberIndexes {
		activeMembersSet[activeMemberIndex] = struct{}{}
	}

	var lowest group.MemberIndex
	for _, localMemberIndex := range localMemberIndexes {
		if _, ok := activeMembersSet[localMemberIndex]; !ok {
			continue
		}

		if lowest == 0 || localMemberIndex < lowest {
			lowest = localMemberIndex
		}
	}

	return lowest
}

func frostMisbehavedMemberIndices(
	groupSize int,
	activeMemberIndexes []group.MemberIndex,
) registry.MisbehavedMemberIndices {
	activeMembersSet := make(map[group.MemberIndex]struct{}, len(activeMemberIndexes))
	for _, memberIndex := range activeMemberIndexes {
		activeMembersSet[memberIndex] = struct{}{}
	}

	misbehavedMembersIndices := make(registry.MisbehavedMemberIndices, 0)
	for i := 1; i <= groupSize; i++ {
		memberIndex := group.MemberIndex(i)
		if _, ok := activeMembersSet[memberIndex]; ok {
			continue
		}

		misbehavedMembersIndices = append(
			misbehavedMembersIndices,
			uint8(memberIndex),
		)
	}

	return misbehavedMembersIndices
}
