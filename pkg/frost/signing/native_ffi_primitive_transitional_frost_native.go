//go:build frost_native

package signing

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

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

	// Do not start coarse native sessions until finalize flow is wired. Calling
	// StartSignRound without finalize would orphan signer-engine state.
	if currentNativeTBTCSignerEngine() != nil && logger != nil {
		logger.Warnf(
			"native tbtc-signer engine is registered but coarse finalize flow is not wired; using legacy fallback",
		)
	}

	// The coarse-session flow is intentionally deferred until keep-core
	// orchestration is migrated from round-level message exchange. Use a Go-side
	// legacy fallback while this migration is in progress.
	return btlcnnefsp.fallbackTBTCSignerLegacySigning(
		ctx,
		logger,
		request,
		legacyPrivateKeyShare,
		fmt.Sprintf(
			"tbtc-signer coarse session flow is not wired (keyGroupSource=%s)",
			payload.KeyGroupSource,
		),
		payload.KeyGroupSource,
	)
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
