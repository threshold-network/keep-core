//go:build frost_native

package signing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
// execution for `frost-uniffi-v1` payloads. `frost-tbtc-signer-v1` currently
// routes through a temporary legacy fallback until coarse session finalize flow
// is wired end-to-end.
type buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive struct{}

const buildTaggedTBTCSignerBootstrapVersionToken = "bootstrap"
const buildTaggedTBTCSignerSyntheticContributionDomain = "tbtc-signer-bootstrap-contribution-v1"

type nativeTBTCSignerVersionedEngine interface {
	Version() (string, error)
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

	dkgParticipants, dkgThreshold, err := buildTaggedTBTCSignerRunDKGInputs(request)
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
			"tbtc-signer RunDKG failed",
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

	keyGroupForRound, err := buildTaggedTBTCSignerRoundKeyGroup(
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
			"cannot query tbtc-signer version; coarse round scaffold skipped",
			payload.KeyGroupSource,
		)
	}

	if !strings.Contains(
		strings.ToLower(engineVersion),
		buildTaggedTBTCSignerBootstrapVersionToken,
	) {
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

	if err := executeBuildTaggedTBTCSignerBootstrapCoarseRound(
		request,
		keyGroupForRound,
		nativeEngine,
	); err != nil {
		return btlcnnefsp.fallbackTBTCSignerLegacySigning(
			ctx,
			logger,
			request,
			legacyPrivateKeyShare,
			"tbtc-signer bootstrap coarse round failed",
			payload.KeyGroupSource,
		)
	}

	if logger != nil {
		logger.Debugf(
			"validated tbtc-signer key-group contract via RunDKG and bootstrap coarse round; using legacy fallback until signature cutover",
		)
	}

	return btlcnnefsp.fallbackTBTCSignerLegacySigning(
		ctx,
		logger,
		request,
		legacyPrivateKeyShare,
		"tbtc-signer bootstrap coarse round completed; using legacy fallback during migration",
		payload.KeyGroupSource,
	)
}

func buildTaggedTBTCSignerRunDKGInputs(
	request *NativeExecutionFFISigningRequest,
) ([]NativeTBTCSignerDKGParticipant, uint16, error) {
	_, includedMembersIndexes, err := includedMembersFromRequest(request)
	if err != nil {
		return nil, 0, err
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
) (string, error) {
	if payload == nil {
		return "", fmt.Errorf("tbtc-signer payload is nil")
	}

	if dkgResult == nil {
		return "", fmt.Errorf("tbtc-signer RunDKG result is nil")
	}

	if dkgResult.KeyGroup == "" {
		return "", fmt.Errorf("tbtc-signer RunDKG key group is empty")
	}

	if payload.KeyGroup == dkgResult.KeyGroup {
		return payload.KeyGroup, nil
	}

	if payload.KeyGroupSource == NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey {
		// Scaffold compatibility: legacy-wallet-pubkey key groups are
		// placeholder-only and expected to diverge from coarse RunDKG output.
		return dkgResult.KeyGroup, nil
	}

	return "", fmt.Errorf("tbtc-signer key group does not match RunDKG result")
}

func executeBuildTaggedTBTCSignerBootstrapCoarseRound(
	request *NativeExecutionFFISigningRequest,
	keyGroup string,
	nativeEngine NativeTBTCSignerEngine,
) error {
	if request == nil {
		return fmt.Errorf("request is nil")
	}

	if request.Message == nil {
		return fmt.Errorf("request message is nil")
	}

	if nativeEngine == nil {
		return fmt.Errorf("native tbtc-signer engine is nil")
	}

	messageBytes := request.Message.Bytes()
	if len(messageBytes) == 0 {
		messageBytes = []byte{0}
	}

	roundState, err := nativeEngine.StartSignRound(
		request.SessionID,
		messageBytes,
		keyGroup,
	)
	if err != nil {
		return fmt.Errorf("start sign round failed: [%w]", err)
	}

	if roundState == nil {
		return fmt.Errorf("start sign round returned nil state")
	}

	if roundState.RequiredContributions == 0 {
		return fmt.Errorf("start sign round required contributions are zero")
	}

	_, includedMembersIndexes, err := includedMembersFromRequest(request)
	if err != nil {
		return fmt.Errorf("cannot determine included members: [%w]", err)
	}

	roundContributions, err := buildTaggedTBTCSignerSyntheticRoundContributions(
		roundState,
		includedMembersIndexes,
	)
	if err != nil {
		return fmt.Errorf("cannot build synthetic round contributions: [%w]", err)
	}

	if len(roundContributions) < int(roundState.RequiredContributions) {
		return fmt.Errorf(
			"insufficient synthetic round contributions: [%v] < [%v]",
			len(roundContributions),
			roundState.RequiredContributions,
		)
	}

	signature, err := nativeEngine.FinalizeSignRound(
		request.SessionID,
		roundContributions,
	)
	if err != nil {
		return fmt.Errorf("finalize sign round failed: [%w]", err)
	}

	if len(signature) == 0 {
		return fmt.Errorf("finalize sign round returned empty signature")
	}

	return nil
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
	registerNativeFROSTSigningUnmarshallers(channel)
	legacySigning.RegisterUnmarshallers(channel)
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
