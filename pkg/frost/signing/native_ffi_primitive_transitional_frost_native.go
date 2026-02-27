//go:build frost_native

package signing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/protocol/group"
	"github.com/keep-network/keep-core/pkg/tecdsa"
	legacySigning "github.com/keep-network/keep-core/pkg/tecdsa/signing"
)

func defaultNativeExecutionFFISigningPrimitiveProviderForBuild() (
	NativeExecutionFFISigningPrimitive,
	error,
) {
	if err := registerBuildTaggedNativeFROSTSigningEngine(); err != nil {
		return nil, err
	}

	return &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}, nil
}

// buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive is a
// transitional primitive that executes native two-round FROST when
// `frost-uniffi-v2` signer material is provided, and preserves legacy bridge
// execution for `frost-uniffi-v1` payloads. `frost-tbtc-signer-v1` uses the
// coarse signing flow for bootstrap engine versions and falls back to legacy
// signing for unsupported or failed coarse-path executions.
type buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive struct{}

const buildTaggedTBTCSignerVersionPrefix = "tbtc-signer/"
const buildTaggedTBTCSignerBootstrapVersionPrerelease = "bootstrap"
const buildTaggedTBTCSignerSyntheticContributionDomain = "tbtc-signer-bootstrap-contribution-v1"
const buildTaggedTBTCSignerMessageTypePrefix = "frost_signing/native_tbtc_signer/"
const buildTaggedTBTCSignerConsumedAttemptReplayErrorFragment = "already consumed for sign attempt"

type nativeTBTCSignerVersionedEngine interface {
	Version() (string, error)
}

type buildTaggedTBTCSignerRoundContributionMessage struct {
	SenderIDValue          uint32 `json:"senderID"`
	SessionIDValue         string `json:"sessionID"`
	ContributionIdentifier uint16 `json:"contributionIdentifier"`
	ContributionData       []byte `json:"contributionData"`
}

func (bttsrcm *buildTaggedTBTCSignerRoundContributionMessage) SenderID() group.MemberIndex {
	return group.MemberIndex(bttsrcm.SenderIDValue)
}

func (bttsrcm *buildTaggedTBTCSignerRoundContributionMessage) SessionID() string {
	return bttsrcm.SessionIDValue
}

func (bttsrcm *buildTaggedTBTCSignerRoundContributionMessage) Type() string {
	return buildTaggedTBTCSignerMessageTypePrefix + "round_contribution"
}

func (bttsrcm *buildTaggedTBTCSignerRoundContributionMessage) Marshal() ([]byte, error) {
	return json.Marshal(bttsrcm)
}

func (bttsrcm *buildTaggedTBTCSignerRoundContributionMessage) Unmarshal(data []byte) error {
	if err := json.Unmarshal(data, bttsrcm); err != nil {
		return err
	}

	if bttsrcm.SenderID() == 0 {
		return fmt.Errorf("sender ID is zero")
	}

	if bttsrcm.SessionID() == "" {
		return fmt.Errorf("session ID is empty")
	}

	if bttsrcm.ContributionIdentifier == 0 {
		return fmt.Errorf("contribution identifier is zero")
	}

	if len(bttsrcm.ContributionData) == 0 {
		return fmt.Errorf("contribution data is empty")
	}

	return nil
}

func (btlcnnefsp *buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive) Sign(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
) (*frost.Signature, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if request.Message == nil {
		return nil, fmt.Errorf("request message is nil")
	}

	if request.SignerMaterial == nil {
		return nil, fmt.Errorf(
			"%w: signer material is nil",
			ErrNativeCryptographyUnavailable,
		)
	}

	switch request.SignerMaterial.Format {
	case NativeSignerMaterialFormatFrostUniFFIV2:
		nativeSignerMaterial, err := decodeNativeFROSTUniFFIV2SignerMaterial(
			request.SignerMaterial,
		)
		if err != nil {
			return nil, err
		}

		return executeNativeFROSTSigning(
			ctx,
			logger,
			request,
			currentNativeFROSTSigningEngine(),
			nativeSignerMaterial,
		)

	case NativeSignerMaterialFormatFrostUniFFIV1:
		return btlcnnefsp.signWithLegacyTECDSABridge(ctx, logger, request)

	case NativeSignerMaterialFormatFrostTBTCSignerV1:
		return btlcnnefsp.signWithTBTCSignerCoarseEngine(ctx, logger, request)

	default:
		return nil, fmt.Errorf(
			"%w: unsupported signer material format: [%s]",
			ErrNativeCryptographyUnavailable,
			request.SignerMaterial.Format,
		)
	}
}

func (btlcnnefsp *buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive) signWithTBTCSignerCoarseEngine(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
) (*frost.Signature, error) {
	payload, err := decodeBuildTaggedTBTCSignerMaterialPayload(request.SignerMaterial)
	if err != nil {
		return nil, err
	}

	legacyPrivateKeyShare, err := decodeBuildTaggedTBTCSignerLegacyPrivateKeyShare(payload)
	if err != nil {
		return nil, err
	}

	nativeEngine := currentNativeTBTCSignerEngine()
	if nativeEngine == nil {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			"native tbtc-signer engine is unavailable",
			payload.KeyGroupSource,
		)
	}

	includedMembersSet, includedMembersIndexes, err := includedMembersFromRequest(request)
	if err != nil {
		if errors.Is(err, ErrInvalidSigningAttemptPolicy) {
			return nil, fmt.Errorf(
				"%w: invalid tbtc-signer signing attempt policy: %w",
				ErrNativeBridgeOperationFailed,
				err,
			)
		}

		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			fmt.Sprintf("cannot determine included members: [%v]", err),
			payload.KeyGroupSource,
		)
	}

	dkgParticipants, dkgThreshold, err := buildTaggedTBTCSignerRunDKGInputsForIncludedMembers(
		request,
		includedMembersIndexes,
	)
	if err != nil {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			fmt.Sprintf("cannot prepare tbtc-signer RunDKG request: [%v]", err),
			payload.KeyGroupSource,
		)
	}

	dkgResult, err := nativeEngine.RunDKG(
		request.SessionID,
		dkgParticipants,
		dkgThreshold,
	)
	if err != nil {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			fmt.Sprintf("tbtc-signer RunDKG failed: [%v]", err),
			payload.KeyGroupSource,
		)
	}

	if dkgResult == nil {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			"tbtc-signer RunDKG returned nil result",
			payload.KeyGroupSource,
		)
	}

	if dkgResult.KeyGroup == "" {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			"tbtc-signer RunDKG returned empty key group",
			payload.KeyGroupSource,
		)
	}

	keyGroupForRound, keyGroupSubstituted, err := buildTaggedTBTCSignerRoundKeyGroup(
		payload,
		dkgResult,
	)
	if err != nil {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			err.Error(),
			payload.KeyGroupSource,
		)
	}

	if keyGroupSubstituted && logger != nil {
		logger.Debugf(
			"substituting scaffold key group from payload source [%s]: payload [%s] -> RunDKG [%s]",
			payload.KeyGroupSource,
			payload.KeyGroup,
			dkgResult.KeyGroup,
		)
	}

	versionedEngine, isVersioned := nativeEngine.(nativeTBTCSignerVersionedEngine)
	if !isVersioned {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			"tbtc-signer version API is unavailable; coarse round scaffold skipped",
			payload.KeyGroupSource,
		)
	}

	engineVersion, err := versionedEngine.Version()
	if err != nil {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			fmt.Sprintf(
				"cannot query tbtc-signer version; coarse round scaffold skipped: [%v]",
				err,
			),
			payload.KeyGroupSource,
		)
	}

	if !isBuildTaggedTBTCSignerBootstrapVersion(engineVersion) {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			fmt.Sprintf(
				"tbtc-signer version [%s] is not bootstrap; coarse round scaffold skipped",
				engineVersion,
			),
			payload.KeyGroupSource,
		)
	}

	coarseSignatureBytes, err := executeBuildTaggedTBTCSignerBootstrapCoarseRoundWithSignature(
		ctx,
		request,
		keyGroupForRound,
		nativeEngine,
		includedMembersSet,
		includedMembersIndexes,
	)
	if err != nil {
		if isBuildTaggedTBTCSignerConsumedAttemptReplayError(err) {
			return nil, fmt.Errorf(
				"%w: consumed tbtc-signer attempt replay: %w: %v",
				ErrNativeBridgeOperationFailed,
				ErrConsumedSigningAttemptReplay,
				err,
			)
		}

		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			fmt.Sprintf("tbtc-signer bootstrap coarse round failed: [%v]", err),
			payload.KeyGroupSource,
		)
	}

	coarseSignature, err := decodeBuildTaggedTBTCSignerSignature(coarseSignatureBytes)
	if err != nil {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			fmt.Sprintf("cannot decode tbtc-signer coarse signature: [%v]", err),
			payload.KeyGroupSource,
		)
	}

	if logger != nil {
		logger.Debugf(
			"validated tbtc-signer key-group contract via RunDKG and bootstrap coarse round; returning coarse signature",
		)
	}

	emitNativeTBTCSignerCoarseSignatureEvent(
		NativeTBTCSignerCoarseSignatureEvent{
			SessionID:      request.SessionID,
			KeyGroupSource: payload.KeyGroupSource,
			EngineVersion:  engineVersion,
		},
	)

	return coarseSignature, nil
}

func isBuildTaggedTBTCSignerConsumedAttemptReplayError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "attempt_id") &&
		strings.Contains(message, buildTaggedTBTCSignerConsumedAttemptReplayErrorFragment)
}

func buildTaggedTBTCSignerRunDKGInputs(
	request *NativeExecutionFFISigningRequest,
) ([]NativeTBTCSignerDKGParticipant, uint16, error) {
	_, includedMembersIndexes, err := includedMembersFromRequest(request)
	if err != nil {
		return nil, 0, err
	}

	return buildTaggedTBTCSignerRunDKGInputsForIncludedMembers(
		request,
		includedMembersIndexes,
	)
}

func buildTaggedTBTCSignerRunDKGInputsForIncludedMembers(
	request *NativeExecutionFFISigningRequest,
	includedMembersIndexes []group.MemberIndex,
) ([]NativeTBTCSignerDKGParticipant, uint16, error) {
	if request == nil {
		return nil, 0, fmt.Errorf("request is nil")
	}

	if len(includedMembersIndexes) < 2 {
		return nil, 0, fmt.Errorf("insufficient included members for DKG")
	}

	threshold := request.DishonestThreshold + 1
	if threshold < 2 {
		return nil, 0, fmt.Errorf("derived threshold is below minimum: [%v]", threshold)
	}

	if threshold > len(includedMembersIndexes) {
		return nil, 0, fmt.Errorf(
			"derived threshold exceeds included members count: [%v] > [%v]",
			threshold,
			len(includedMembersIndexes),
		)
	}

	participants := make([]NativeTBTCSignerDKGParticipant, 0, len(includedMembersIndexes))
	for _, memberIndex := range includedMembersIndexes {
		if memberIndex == 0 {
			return nil, 0, fmt.Errorf("included member index is zero")
		}

		identifier := uint16(memberIndex)
		participants = append(
			participants,
			NativeTBTCSignerDKGParticipant{
				Identifier:   identifier,
				PublicKeyHex: buildTaggedTBTCSignerDKGPlaceholderPublicKeyHex(identifier),
			},
		)
	}

	return participants, uint16(threshold), nil
}

func buildTaggedTBTCSignerDKGPlaceholderPublicKeyHex(identifier uint16) string {
	// Transitional placeholder until canonical member public keys are available
	// in the native signing request path.
	return fmt.Sprintf("02%04x", identifier)
}

func buildTaggedTBTCSignerRoundKeyGroup(
	payload *NativeTBTCSignerMaterialPayload,
	dkgResult *NativeTBTCSignerDKGResult,
) (string, bool, error) {
	if payload == nil {
		return "", false, fmt.Errorf("tbtc-signer payload is nil")
	}

	if dkgResult == nil {
		return "", false, fmt.Errorf("tbtc-signer RunDKG result is nil")
	}

	if dkgResult.KeyGroup == "" {
		return "", false, fmt.Errorf("tbtc-signer RunDKG key group is empty")
	}

	if payload.KeyGroup == dkgResult.KeyGroup {
		return payload.KeyGroup, false, nil
	}

	if payload.KeyGroupSource == NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey {
		// Scaffold compatibility: legacy-wallet-pubkey key groups are
		// placeholder-only and expected to diverge from coarse RunDKG output.
		return dkgResult.KeyGroup, true, nil
	}

	return "", false, fmt.Errorf("tbtc-signer key group does not match RunDKG result")
}

func isBuildTaggedTBTCSignerBootstrapVersion(version string) bool {
	version = strings.TrimSpace(version)
	if !strings.HasPrefix(version, buildTaggedTBTCSignerVersionPrefix) {
		return false
	}

	version = strings.TrimPrefix(version, buildTaggedTBTCSignerVersionPrefix)
	coreVersion, prerelease, hasPrerelease := strings.Cut(version, "-")
	if !hasPrerelease {
		return false
	}

	if prerelease != buildTaggedTBTCSignerBootstrapVersionPrerelease &&
		!strings.HasPrefix(
			prerelease,
			buildTaggedTBTCSignerBootstrapVersionPrerelease+".",
		) {
		return false
	}

	coreSegments := strings.Split(coreVersion, ".")
	if len(coreSegments) != 3 {
		return false
	}

	// Bootstrap scaffold must be enabled only on 0.x.y pre-release builds.
	if coreSegments[0] != "0" {
		return false
	}

	for _, segment := range coreSegments {
		if segment == "" {
			return false
		}

		for _, character := range segment {
			if character < '0' || character > '9' {
				return false
			}
		}
	}

	return true
}

func executeBuildTaggedTBTCSignerBootstrapCoarseRound(
	ctx context.Context,
	request *NativeExecutionFFISigningRequest,
	keyGroup string,
	nativeEngine NativeTBTCSignerEngine,
	includedMembersSet map[group.MemberIndex]struct{},
	includedMembersIndexes []group.MemberIndex,
) error {
	_, err := executeBuildTaggedTBTCSignerBootstrapCoarseRoundWithSignature(
		ctx,
		request,
		keyGroup,
		nativeEngine,
		includedMembersSet,
		includedMembersIndexes,
	)

	return err
}

func executeBuildTaggedTBTCSignerBootstrapCoarseRoundWithSignature(
	ctx context.Context,
	request *NativeExecutionFFISigningRequest,
	keyGroup string,
	nativeEngine NativeTBTCSignerEngine,
	includedMembersSet map[group.MemberIndex]struct{},
	includedMembersIndexes []group.MemberIndex,
) ([]byte, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if request.Message == nil {
		return nil, fmt.Errorf("request message is nil")
	}

	if nativeEngine == nil {
		return nil, fmt.Errorf("native tbtc-signer engine is nil")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if includedMembersSet == nil || len(includedMembersIndexes) == 0 {
		var err error
		includedMembersSet, includedMembersIndexes, err = includedMembersFromRequest(request)
		if err != nil {
			return nil, fmt.Errorf("cannot determine included members: [%w]", err)
		}
	}

	if _, ok := includedMembersSet[request.MemberIndex]; !ok {
		return nil, fmt.Errorf(
			"member [%v] not included in tbtc-signer signing attempt",
			request.MemberIndex,
		)
	}

	messageBytes := request.Message.Bytes()
	if len(messageBytes) == 0 {
		messageBytes = []byte{0}
	}

	if request.MemberIndex == 0 {
		return nil, fmt.Errorf("request member index is zero")
	}

	signingParticipants, err := buildTaggedTBTCSignerSigningParticipants(
		includedMembersIndexes,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot derive signing participants: [%w]", err)
	}

	roundState, err := nativeEngine.StartSignRound(
		request.SessionID,
		uint16(request.MemberIndex),
		messageBytes,
		keyGroup,
		signingParticipants,
	)
	if err != nil {
		return nil, fmt.Errorf("start sign round failed: [%w]", err)
	}

	if roundState == nil {
		return nil, fmt.Errorf("start sign round returned nil state")
	}

	if roundState.RequiredContributions == 0 {
		return nil, fmt.Errorf("start sign round required contributions are zero")
	}

	if len(signingParticipants) > 0 {
		if len(roundState.SigningParticipants) != len(signingParticipants) {
			return nil, fmt.Errorf(
				"start sign round returned unexpected signing participants count: [%v] != [%v]",
				len(roundState.SigningParticipants),
				len(signingParticipants),
			)
		}

		for i := range signingParticipants {
			if roundState.SigningParticipants[i] != signingParticipants[i] {
				return nil, fmt.Errorf(
					"start sign round returned unexpected signing participant at index [%d]: [%v] != [%v]",
					i,
					roundState.SigningParticipants[i],
					signingParticipants[i],
				)
			}
		}
	}

	roundContributions, err := buildTaggedTBTCSignerRoundContributions(
		ctx,
		request,
		roundState,
		includedMembersSet,
		includedMembersIndexes,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot collect round contributions: [%w]", err)
	}

	if len(roundContributions) < int(roundState.RequiredContributions) {
		return nil, fmt.Errorf(
			"insufficient round contributions: [%v] < [%v]",
			len(roundContributions),
			roundState.RequiredContributions,
		)
	}

	signature, err := nativeEngine.FinalizeSignRound(
		request.SessionID,
		roundContributions,
	)
	if err != nil {
		return nil, fmt.Errorf("finalize sign round failed: [%w]", err)
	}

	if len(signature) == 0 {
		return nil, fmt.Errorf("finalize sign round returned empty signature")
	}

	return signature, nil
}

func decodeBuildTaggedTBTCSignerSignature(signature []byte) (*frost.Signature, error) {
	if len(signature) == 0 {
		return nil, fmt.Errorf("signature is empty")
	}

	// Unmarshal validates signature wire format (length + split into R/S) only.
	// Cryptographic validity is enforced by downstream Schnorr verification at
	// submission time.
	result := &frost.Signature{}
	if err := result.Unmarshal(signature); err != nil {
		return nil, fmt.Errorf("invalid frost signature bytes: [%w]", err)
	}

	return result, nil
}

func buildTaggedTBTCSignerSigningParticipants(
	includedMembersIndexes []group.MemberIndex,
) ([]uint16, error) {
	if len(includedMembersIndexes) == 0 {
		return nil, fmt.Errorf("included members are empty")
	}

	signingParticipants := make([]uint16, 0, len(includedMembersIndexes))
	seenParticipants := make(map[uint16]struct{}, len(includedMembersIndexes))

	for _, memberIndex := range includedMembersIndexes {
		if memberIndex == 0 {
			return nil, fmt.Errorf("included member index is zero")
		}

		participant := uint16(memberIndex)
		if _, ok := seenParticipants[participant]; ok {
			return nil, fmt.Errorf("duplicate included member index: [%v]", memberIndex)
		}

		seenParticipants[participant] = struct{}{}
		signingParticipants = append(signingParticipants, participant)
	}

	return signingParticipants, nil
}

func buildTaggedTBTCSignerRoundContributions(
	ctx context.Context,
	request *NativeExecutionFFISigningRequest,
	roundState *NativeTBTCSignerRoundState,
	includedMembersSet map[group.MemberIndex]struct{},
	includedMembersIndexes []group.MemberIndex,
) ([]NativeTBTCSignerRoundContribution, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if request.Channel == nil {
		// Compatibility path for unit tests that do not attach a broadcast
		// channel. Runtime signer flows provide a channel and use contribution
		// exchange with peers.
		return buildTaggedTBTCSignerSyntheticRoundContributions(
			roundState,
			includedMembersIndexes,
		)
	}

	ownContribution, err := buildTaggedTBTCSignerOwnRoundContribution(
		request,
		roundState,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot build own round contribution: [%w]", err)
	}

	roundContributionMessage := &buildTaggedTBTCSignerRoundContributionMessage{
		SenderIDValue:          uint32(request.MemberIndex),
		SessionIDValue:         request.SessionID,
		ContributionIdentifier: ownContribution.Identifier,
		ContributionData:       append([]byte{}, ownContribution.Data...),
	}

	if err := request.Channel.Send(
		ctx,
		roundContributionMessage,
		net.BackoffRetransmissionStrategy,
	); err != nil {
		return nil, fmt.Errorf("cannot send round contribution message: [%w]", err)
	}

	peerMessages, err := collectBuildTaggedTBTCSignerRoundContributionMessages(
		ctx,
		request,
		includedMembersSet,
		includedMembersIndexes,
	)
	if err != nil {
		return nil, err
	}

	contributionsBySender := map[group.MemberIndex]NativeTBTCSignerRoundContribution{
		request.MemberIndex: ownContribution,
	}

	for senderID, message := range peerMessages {
		contributionsBySender[senderID] = NativeTBTCSignerRoundContribution{
			Identifier: message.ContributionIdentifier,
			Data:       append([]byte{}, message.ContributionData...),
		}
	}

	orderedContributions := make(
		[]NativeTBTCSignerRoundContribution,
		0,
		len(includedMembersIndexes),
	)
	for _, memberIndex := range includedMembersIndexes {
		contribution, ok := contributionsBySender[memberIndex]
		if !ok {
			return nil, fmt.Errorf("missing contribution from member [%v]", memberIndex)
		}

		orderedContributions = append(orderedContributions, contribution)
	}

	return orderedContributions, nil
}

func buildTaggedTBTCSignerOwnRoundContribution(
	request *NativeExecutionFFISigningRequest,
	roundState *NativeTBTCSignerRoundState,
) (NativeTBTCSignerRoundContribution, error) {
	if request == nil {
		return NativeTBTCSignerRoundContribution{}, fmt.Errorf("request is nil")
	}

	if request.MemberIndex == 0 {
		return NativeTBTCSignerRoundContribution{}, fmt.Errorf("request member index is zero")
	}

	if roundState != nil && roundState.OwnContribution != nil {
		ownContribution := roundState.OwnContribution
		if ownContribution.Identifier == 0 {
			return NativeTBTCSignerRoundContribution{}, fmt.Errorf(
				"round state own contribution identifier is zero",
			)
		}

		if len(ownContribution.Data) == 0 {
			return NativeTBTCSignerRoundContribution{}, fmt.Errorf(
				"round state own contribution data is empty",
			)
		}

		if ownContribution.Identifier != uint16(request.MemberIndex) {
			return NativeTBTCSignerRoundContribution{}, fmt.Errorf(
				"round state own contribution identifier [%v] does not match member index [%v]",
				ownContribution.Identifier,
				request.MemberIndex,
			)
		}

		return NativeTBTCSignerRoundContribution{
			Identifier: ownContribution.Identifier,
			Data:       append([]byte{}, ownContribution.Data...),
		}, nil
	}

	ownContributions, err := buildTaggedTBTCSignerSyntheticRoundContributions(
		roundState,
		[]group.MemberIndex{request.MemberIndex},
	)
	if err != nil {
		return NativeTBTCSignerRoundContribution{}, err
	}

	if len(ownContributions) != 1 {
		return NativeTBTCSignerRoundContribution{}, fmt.Errorf(
			"unexpected own contribution count: [%v]",
			len(ownContributions),
		)
	}

	return ownContributions[0], nil
}

func collectBuildTaggedTBTCSignerRoundContributionMessages(
	ctx context.Context,
	request *NativeExecutionFFISigningRequest,
	includedMembersSet map[group.MemberIndex]struct{},
	includedMembersIndexes []group.MemberIndex,
) (map[group.MemberIndex]*buildTaggedTBTCSignerRoundContributionMessage, error) {
	expectedMessagesCount := len(includedMembersIndexes) - 1
	if expectedMessagesCount <= 0 {
		return map[group.MemberIndex]*buildTaggedTBTCSignerRoundContributionMessage{}, nil
	}

	recvCtx, cancelRecvCtx := context.WithCancel(ctx)
	defer cancelRecvCtx()

	messageChan := make(
		chan *buildTaggedTBTCSignerRoundContributionMessage,
		expectedMessagesCount*4+1,
	)

	request.Channel.Recv(recvCtx, func(message net.Message) {
		payload, ok := message.Payload().(*buildTaggedTBTCSignerRoundContributionMessage)
		if !ok {
			return
		}

		if !shouldAcceptNativeFROSTMessage(
			request,
			includedMembersSet,
			payload.SenderID(),
			payload.SessionID(),
			message.SenderPublicKey(),
		) {
			return
		}

		select {
		case messageChan <- payload:
		default:
		}
	})

	receivedMessages := make(
		map[group.MemberIndex]*buildTaggedTBTCSignerRoundContributionMessage,
	)
	for len(receivedMessages) < expectedMessagesCount {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"tbtc-signer round contribution collection interrupted: [%w]",
				ctx.Err(),
			)

		case message := <-messageChan:
			receivedMessages[message.SenderID()] = message
		}
	}

	return receivedMessages, nil
}

func buildTaggedTBTCSignerSyntheticRoundContributions(
	roundState *NativeTBTCSignerRoundState,
	includedMembersIndexes []group.MemberIndex,
) ([]NativeTBTCSignerRoundContribution, error) {
	if roundState == nil {
		return nil, fmt.Errorf("round state is nil")
	}

	if roundState.SessionID == "" {
		return nil, fmt.Errorf("round state session ID is empty")
	}

	if roundState.RoundID == "" {
		return nil, fmt.Errorf("round state round ID is empty")
	}

	if roundState.MessageDigestHex == "" {
		return nil, fmt.Errorf("round state message digest is empty")
	}

	contributions := make(
		[]NativeTBTCSignerRoundContribution,
		0,
		len(includedMembersIndexes),
	)

	for _, memberIndex := range includedMembersIndexes {
		if memberIndex == 0 {
			return nil, fmt.Errorf("included member index is zero")
		}

		identifier := uint16(memberIndex)
		seed := fmt.Sprintf(
			"%s:%s:%s:%s:%d",
			buildTaggedTBTCSignerSyntheticContributionDomain,
			roundState.SessionID,
			roundState.RoundID,
			roundState.MessageDigestHex,
			identifier,
		)
		shareDigest := sha256.Sum256([]byte(seed))

		contributions = append(
			contributions,
			NativeTBTCSignerRoundContribution{
				Identifier: identifier,
				Data:       append([]byte{}, shareDigest[:]...),
			},
		)
	}

	return contributions, nil
}

func (btlcnnefsp *buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive) signWithLegacyTECDSABridge(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
) (*frost.Signature, error) {
	privateKeyShare, err := decodeBuildTaggedLegacyPrivateKeyShare(
		request.SignerMaterial,
	)
	if err != nil {
		return nil, err
	}

	return btlcnnefsp.signWithLegacyPrivateKeyShare(
		ctx,
		logger,
		request,
		privateKeyShare,
	)
}

func (btlcnnefsp *buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive) signWithLegacyPrivateKeyShare(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
	privateKeyShare *tecdsa.PrivateKeyShare,
) (*frost.Signature, error) {
	if privateKeyShare == nil {
		return nil, fmt.Errorf("legacy private key share is nil")
	}

	excludedMembersIndexes := []group.MemberIndex{}
	if request.Attempt != nil {
		excludedMembersIndexes = request.Attempt.ExcludedMembersIndexes
	}

	legacyResult, err := legacySigning.Execute(
		ctx,
		logger,
		request.Message,
		request.SessionID,
		request.MemberIndex,
		privateKeyShare,
		request.GroupSize,
		request.DishonestThreshold,
		excludedMembersIndexes,
		request.Channel,
		request.MembershipValidator,
	)
	if err != nil {
		return nil, err
	}

	return FromTECDSASignature(legacyResult.Signature)
}

func (btlcnnefsp *buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive) fallbackTBTCSignerLegacySigning(
	ctx context.Context,
	logger log.StandardLogger,
	request *NativeExecutionFFISigningRequest,
	legacyPrivateKeyShare *tecdsa.PrivateKeyShare,
	reason string,
	keyGroupSource string,
) (*frost.Signature, error) {
	emitNativeTBTCSignerFallbackEvent(
		NativeTBTCSignerFallbackEvent{
			SessionID:                   request.SessionID,
			Reason:                      reason,
			KeyGroupSource:              keyGroupSource,
			LegacyPrivateKeyShareExists: legacyPrivateKeyShare != nil,
		},
	)

	if legacyPrivateKeyShare == nil {
		return nil, fmt.Errorf("%w: %s", ErrNativeCryptographyUnavailable, reason)
	}

	if logger != nil {
		logger.Warnf(
			"falling back to legacy tECDSA signer path for tbtc-signer payload: [%s]",
			reason,
		)
	}

	return btlcnnefsp.signWithLegacyPrivateKeyShare(
		ctx,
		logger,
		request,
		legacyPrivateKeyShare,
	)
}

func (btlcnnefsp *buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
	registerBuildTaggedTBTCSignerUnmarshallers(channel)
	registerNativeFROSTSigningUnmarshallers(channel)
	legacySigning.RegisterUnmarshallers(channel)
}

func registerBuildTaggedTBTCSignerUnmarshallers(channel net.BroadcastChannel) {
	channel.SetUnmarshaler(func() net.TaggedUnmarshaler {
		return &buildTaggedTBTCSignerRoundContributionMessage{}
	})
}

func decodeBuildTaggedLegacyPrivateKeyShare(
	signerMaterial *NativeSignerMaterial,
) (*tecdsa.PrivateKeyShare, error) {
	if signerMaterial == nil {
		return nil, fmt.Errorf(
			"%w: signer material is nil",
			ErrNativeCryptographyUnavailable,
		)
	}

	if signerMaterial.Format != NativeSignerMaterialFormatFrostUniFFIV1 {
		return nil, fmt.Errorf(
			"%w: unsupported signer material format: [%s]",
			ErrNativeCryptographyUnavailable,
			signerMaterial.Format,
		)
	}

	if len(signerMaterial.Payload) == 0 {
		return nil, fmt.Errorf(
			"%w: signer material payload is empty",
			ErrNativeCryptographyUnavailable,
		)
	}

	privateKeyShare := &tecdsa.PrivateKeyShare{}
	if err := privateKeyShare.Unmarshal(signerMaterial.Payload); err != nil {
		return nil, fmt.Errorf(
			"%w: cannot unmarshal signer material payload: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	return privateKeyShare, nil
}

func decodeBuildTaggedTBTCSignerMaterialPayload(
	signerMaterial *NativeSignerMaterial,
) (*NativeTBTCSignerMaterialPayload, error) {
	if signerMaterial == nil {
		return nil, fmt.Errorf(
			"%w: signer material is nil",
			ErrNativeCryptographyUnavailable,
		)
	}

	if signerMaterial.Format != NativeSignerMaterialFormatFrostTBTCSignerV1 {
		return nil, fmt.Errorf(
			"%w: unsupported signer material format: [%s]",
			ErrNativeCryptographyUnavailable,
			signerMaterial.Format,
		)
	}

	if len(signerMaterial.Payload) == 0 {
		return nil, fmt.Errorf(
			"%w: signer material payload is empty",
			ErrNativeCryptographyUnavailable,
		)
	}

	var payload NativeTBTCSignerMaterialPayload
	if err := json.Unmarshal(signerMaterial.Payload, &payload); err != nil {
		return nil, fmt.Errorf(
			"%w: cannot unmarshal tbtc-signer payload: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if payload.KeyGroup == "" {
		return nil, fmt.Errorf(
			"%w: tbtc-signer key group is empty",
			ErrNativeCryptographyUnavailable,
		)
	}

	return &payload, nil
}

func decodeBuildTaggedTBTCSignerKeyGroup(
	signerMaterial *NativeSignerMaterial,
) (string, error) {
	payload, err := decodeBuildTaggedTBTCSignerMaterialPayload(signerMaterial)
	if err != nil {
		return "", err
	}

	return payload.KeyGroup, nil
}

func decodeBuildTaggedTBTCSignerLegacyPrivateKeyShare(
	payload *NativeTBTCSignerMaterialPayload,
) (*tecdsa.PrivateKeyShare, error) {
	if payload == nil || payload.LegacyPrivateKeyShareHex == "" {
		return nil, nil
	}

	legacyPrivateKeySharePayload, err := hex.DecodeString(payload.LegacyPrivateKeyShareHex)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: cannot decode tbtc-signer legacy private key share: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	privateKeyShare := &tecdsa.PrivateKeyShare{}
	if err := privateKeyShare.Unmarshal(legacyPrivateKeySharePayload); err != nil {
		return nil, fmt.Errorf(
			"%w: cannot unmarshal tbtc-signer legacy private key share: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	return privateKeyShare, nil
}
