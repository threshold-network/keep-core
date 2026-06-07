//go:build frost_native

package tbtc

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"

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

func executeFrostDKGIfPossible(
	ctx context.Context,
	node *node,
	frostChain FrostDKGChain,
	event *FrostDKGStartedEvent,
	memberIndexes []group.MemberIndex,
	groupSelectionResult *GroupSelectionResult,
) {
	nativeTBTCSignerEngine := frostsigning.CurrentNativeTBTCSignerEngine()
	if nativeTBTCSignerEngine == nil {
		logger.Infof(
			"FROST DKG with seed [0x%x] selected this operator as member "+
				"indexes [%v], but no native tbtc-signer engine is registered",
			event.Seed,
			memberIndexes,
		)
		return
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
		return
	}

	registerFrostDKGResultSigningUnmarshaller(channel)
	protocolannouncer.RegisterUnmarshaller(channel)

	if err := channel.SetFilter(membershipValidator.IsInGroup); err != nil {
		logger.Errorf("failed to set FROST DKG broadcast channel filter: [%v]", err)
		return
	}

	params, err := frostChain.FrostDKGParameters()
	if err != nil {
		logger.Errorf("failed to get FROST DKG parameters: [%v]", err)
		return
	}

	signatureThreshold, err := frostDKGSignatureThreshold(node.groupParameters)
	if err != nil {
		logger.Errorf("invalid FROST DKG group parameters: [%v]", err)
		return
	}

	fullMembers := frostFullMembers(groupSelectionResult)
	dkgTimeoutBlock := event.BlockNumber + params.SubmissionTimeoutBlocks

	for _, currentMemberIndex := range memberIndexes {
		memberIndex := currentMemberIndex

		go func() {
			dkgLogger := logger.With(
				zap.String("seed", fmt.Sprintf("0x%x", event.Seed)),
				zap.Uint8("memberIndex", uint8(memberIndex)),
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
			activeMemberIndexes, misbehavedMembersIndices, err :=
				announceFrostDKGReadiness(
					dkgCtx,
					node,
					channel,
					membershipValidator,
					fmt.Sprintf("%v-%v", ProtocolName, "frost-dkg"),
					sessionID,
					memberIndex,
					len(groupSelectionResult.OperatorsIDs),
				)
			if err != nil {
				dkgLogger.Errorf("FROST DKG readiness announcement failed: [%v]", err)
				return
			}

			submitterMemberIndex := lowestLocalActiveMemberIndex(
				memberIndexes,
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
				node.groupParameters,
			)
			if err != nil {
				dkgLogger.Errorf("failed to resolve final FROST DKG member indexes: [%v]", err)
				return
			}

			executionResult, err := executeFrostDKG(
				nativeTBTCSignerEngine,
				event,
				tbtcSignerMemberIndexes,
				signatureThreshold,
				sessionID,
			)
			if err != nil {
				dkgLogger.Errorf("FROST DKG execution failed: [%v]", err)
				return
			}

			if err := registerFrostSignerWithMaterial(
				node,
				executionResult.outputKey,
				executionResult.signerMaterial,
				memberIndex,
				activeMemberIndexes,
				groupSelectionResult,
			); err != nil {
				dkgLogger.Errorf("failed to register FROST signer: [%v]", err)
				return
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
				memberIndex,
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

			if memberIndex != submitterMemberIndex {
				dkgLogger.Infof(
					"skipping FROST DKG result submission; member [%d] is "+
						"the designated local submitter",
					submitterMemberIndex,
				)
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
	}
}

type frostDKGExecutionResult struct {
	outputKey      frost.OutputKey
	signerMaterial *frostsigning.NativeSignerMaterial
}

func executeFrostDKG(
	nativeTBTCSignerEngine frostsigning.NativeTBTCSignerEngine,
	event *FrostDKGStartedEvent,
	dkgMemberIndexes []group.MemberIndex,
	signatureThreshold int,
	sessionID string,
) (*frostDKGExecutionResult, error) {
	if nativeTBTCSignerEngine != nil {
		return executeTBTCSignerFROSTDKG(
			nativeTBTCSignerEngine,
			event,
			dkgMemberIndexes,
			signatureThreshold,
			sessionID,
		)
	}

	return nil, fmt.Errorf("native tbtc-signer engine is unavailable")
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

func executeTBTCSignerFROSTDKG(
	nativeEngine frostsigning.NativeTBTCSignerEngine,
	event *FrostDKGStartedEvent,
	dkgMemberIndexes []group.MemberIndex,
	signatureThreshold int,
	sessionID string,
) (*frostDKGExecutionResult, error) {
	if nativeEngine == nil {
		return nil, fmt.Errorf("native tbtc-signer engine is unavailable")
	}

	seededEngine, ok := nativeEngine.(frostsigning.NativeTBTCSignerSeededDKGEngine)
	if !ok {
		return nil, fmt.Errorf("native tbtc-signer engine does not support seeded DKG")
	}

	dkgSeedHex, err := frostDKGSeedHex(event.Seed)
	if err != nil {
		return nil, err
	}

	participants, err := nativeTBTCSignerDKGParticipants(dkgMemberIndexes)
	if err != nil {
		return nil, err
	}

	if signatureThreshold <= 0 || signatureThreshold > int(^uint16(0)) {
		return nil, fmt.Errorf(
			"invalid tbtc-signer DKG threshold [%d]",
			signatureThreshold,
		)
	}

	dkgResult, err := seededEngine.RunDKGWithSeed(
		sessionID,
		participants,
		uint16(signatureThreshold),
		dkgSeedHex,
	)
	if err != nil {
		return nil, fmt.Errorf("tbtc-signer RunDKG failed: [%w]", err)
	}

	outputKey, err := outputKeyFromTBTCSignerDKGResult(dkgResult)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(frostsigning.NativeTBTCSignerMaterialPayload{
		KeyGroup:         dkgResult.KeyGroup,
		TaprootOutputKey: hex.EncodeToString(outputKey[:]),
		KeyGroupSource:   frostsigning.NativeTBTCSignerKeyGroupSourceDKGPersisted,
		DKGSeedHex:       dkgSeedHex,
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

func frostDKGSeedHex(seed *big.Int) (string, error) {
	if seed == nil {
		return "", fmt.Errorf("FROST DKG seed is nil")
	}
	if seed.Sign() < 0 || len(seed.Bytes()) > frost.OutputKeySize {
		return "", fmt.Errorf("FROST DKG seed must fit in %d bytes", frost.OutputKeySize)
	}

	seedBytes := make([]byte, frost.OutputKeySize)
	seed.FillBytes(seedBytes)

	return hex.EncodeToString(seedBytes), nil
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
	memberIndex group.MemberIndex,
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
	activeMemberIndexes, err := announcer.Announce(
		announceCtx,
		memberIndex,
		sessionID,
	)
	if err != nil {
		return nil, nil, err
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	if len(activeMemberIndexes) < node.groupParameters.GroupQuorum {
		return nil, nil, fmt.Errorf(
			"FROST DKG readiness quorum not reached: [%d] active members, quorum [%d]",
			len(activeMemberIndexes),
			node.groupParameters.GroupQuorum,
		)
	}

	return activeMemberIndexes,
		frostMisbehavedMemberIndices(groupSize, activeMemberIndexes),
		nil
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
			node.groupParameters,
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
		logger.Infof(
			"skipping FROST DKG result submission by member [%d]; current state is [%v]",
			memberIndex,
			state,
		)
		return nil
	}

	return frostChain.SubmitFrostDKGResult(result)
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
