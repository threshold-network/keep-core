//go:build frost_native

package signing

import (
	"context"
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
// execution for `frost-uniffi-v1` payloads. `frost-tbtc-signer-v1` is reserved
// for coarse session engine integration and currently returns a scaffold error.
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
	keyGroup, err := decodeBuildTaggedTBTCSignerKeyGroup(request.SignerMaterial)
	if err != nil {
		return nil, err
	}

	engine := currentNativeTBTCSignerEngine()
	if engine == nil {
		return nil, fmt.Errorf(
			"%w: native tbtc-signer engine is unavailable",
			ErrNativeCryptographyUnavailable,
		)
	}

	_, err = engine.StartSignRound(
		request.SessionID,
		request.Message.Bytes(),
		keyGroup,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: tbtc-signer StartSignRound failed: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	// The coarse-session finalize flow is intentionally deferred until keep-core
	// transport/orchestration is migrated from round-level message exchange.
	return nil, fmt.Errorf(
		"%w: tbtc-signer coarse session finalize flow is not wired",
		ErrNativeCryptographyUnavailable,
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

type buildTaggedTBTCSignerMaterialPayload struct {
	KeyGroup string `json:"keyGroup"`
}

func decodeBuildTaggedTBTCSignerKeyGroup(
	signerMaterial *NativeSignerMaterial,
) (string, error) {
	if signerMaterial == nil {
		return "", fmt.Errorf(
			"%w: signer material is nil",
			ErrNativeCryptographyUnavailable,
		)
	}

	if signerMaterial.Format != NativeSignerMaterialFormatFrostTBTCSignerV1 {
		return "", fmt.Errorf(
			"%w: unsupported signer material format: [%s]",
			ErrNativeCryptographyUnavailable,
			signerMaterial.Format,
		)
	}

	if len(signerMaterial.Payload) == 0 {
		return "", fmt.Errorf(
			"%w: signer material payload is empty",
			ErrNativeCryptographyUnavailable,
		)
	}

	var payload buildTaggedTBTCSignerMaterialPayload
	if err := json.Unmarshal(signerMaterial.Payload, &payload); err != nil {
		return "", fmt.Errorf(
			"%w: cannot unmarshal tbtc-signer payload: [%v]",
			ErrNativeCryptographyUnavailable,
			err,
		)
	}

	if payload.KeyGroup == "" {
		return "", fmt.Errorf(
			"%w: tbtc-signer key group is empty",
			ErrNativeCryptographyUnavailable,
		)
	}

	return payload.KeyGroup, nil
}
