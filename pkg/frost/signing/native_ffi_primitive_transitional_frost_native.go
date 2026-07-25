//go:build frost_native

package signing

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
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
	if err := installConfiguredTBTCSignerInitConfig(); err != nil {
		return nil, err
	}

	if err := registerBuildTaggedNativeFROSTSigningEngine(); err != nil {
		return nil, err
	}

	return &buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive{}, nil
}

// installConfiguredTBTCSignerInitConfig installs the tbtc-signer init-time
// configuration when TBTC_SIGNER_INIT_CONFIG_PATH points at a JSON config
// file. The operator setting the path is an explicit demand for config-mode
// operation, so every failure - unreadable file, validation rejection, or a
// loaded signer library that predates frost_tbtc_init_signer_config - fails
// the FROST-native engine registration, and the demand enforcement at the
// end of registration (enforceNativeInitConfigDemand) then terminates the
// process: a node that cannot honor its demanded config must not run
// half-alive on the legacy bridge. The failure is logged at error level
// here with the config-mode context before the fatal exit so the cause is
// on record. With the path unset this is a no-op and the signer reads
// TBTC_SIGNER_* from the process environment (the transitional path), where
// registration failures keep the safe-by-default degrade posture.
func installConfiguredTBTCSignerInitConfig() error {
	configPath := strings.TrimSpace(os.Getenv(TBTCSignerInitConfigPathEnv))
	if configPath == "" {
		return nil
	}

	configJSON, err := readSecureNativeTBTCSignerInitConfig(configPath)
	if err != nil {
		err = fmt.Errorf(
			"read tbtc-signer init config [%s]: %w",
			configPath, err,
		)
		registrationLogger.Errorf(
			"tbtc-signer init config installation failed; FROST-native "+
				"engine registration fails closed and the process will "+
				"terminate at the end of registration (config-mode demand "+
				"unmet): [%v]",
			err,
		)
		return err
	}
	defer zeroBytes(configJSON)

	result, err := InstallNativeTBTCSignerConfig(configJSON)
	if err != nil {
		err = fmt.Errorf(
			"install tbtc-signer init config from [%s]: %w",
			configPath, err,
		)
		registrationLogger.Errorf(
			"tbtc-signer init config installation failed; FROST-native "+
				"engine registration fails closed and the process will "+
				"terminate at the end of registration (config-mode demand "+
				"unmet): [%v]",
			err,
		)
		return err
	}
	if err := recordNativeTBTCSignerInstalledStateAnchorConfig(
		configJSON,
		result.ConfigFingerprint,
	); err != nil {
		err = fmt.Errorf(
			"bind installed tbtc-signer anchor config from [%s]: %w",
			configPath,
			err,
		)
		registrationLogger.Errorf(
			"tbtc-signer anchor config binding failed; FROST-native engine "+
				"registration fails closed: [%v]",
			err,
		)
		return err
	}

	registrationLogger.Infof(
		"installed tbtc-signer init config from [%s]: fingerprint [%s], "+
			"configured keys [%d], idempotent [%v]",
		configPath,
		result.ConfigFingerprint,
		result.ConfiguredKeyCount,
		result.Idempotent,
	)
	return nil
}

// buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive is a
// transitional primitive that preserves legacy tECDSA bridge execution for
// `frost-uniffi-v1` payloads. The transitional coarse-FROST signing path for
// `frost-tbtc-signer-v1` material has been removed: that material is signed via
// the interactive ROAST path (driven by the executor adapter), not by this
// inner primitive, which now refuses it terminally. Unsupported
// `frost-uniffi-v2` material is rejected explicitly because it cannot produce
// Taproot-tweaked signatures; accepting it would allow new deposits to a
// wallet that cannot sweep them.
type buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive struct{}

const buildTaggedTBTCSignerVersionPrefix = "tbtc-signer/"
const buildTaggedTBTCSignerBootstrapVersionPrerelease = "bootstrap"

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
		return nil, fmt.Errorf(
			"%w: unsupported UniFFI FROST signer material format [%s]; it cannot sweep Taproot deposits; use [%s]",
			ErrUnsupportedSignerMaterialFormat,
			NativeSignerMaterialFormatFrostUniFFIV2,
			NativeSignerMaterialFormatFrostTBTCSignerV1,
		)

	case NativeSignerMaterialFormatFrostUniFFIV1:
		return btlcnnefsp.signWithLegacyTECDSABridge(ctx, logger, request)

	case NativeSignerMaterialFormatFrostTBTCSignerV1:
		// The transitional coarse-FROST signing path has been removed. FROST
		// `frost-tbtc-signer-v1` material must be signed via the interactive
		// ROAST path (enabled with KEEP_CORE_FROST_INTERACTIVE_SIGNING_ENABLED
		// and driven by the executor adapter); this inner primitive no longer
		// produces coarse signatures for it. Fail TERMINAL so the tBTC
		// signingRetryLoop aborts immediately instead of retrying a
		// deterministic configuration failure until timeout.
		return nil, fmt.Errorf(
			"%w: coarse-FROST signing was removed; [%s] signer material must be "+
				"signed via the interactive path (%s), not this transitional primitive",
			ErrTerminalSigningFailure,
			NativeSignerMaterialFormatFrostTBTCSignerV1,
			InteractiveSigningOptInEnvVar,
		)

	default:
		return nil, fmt.Errorf(
			"%w: unsupported signer material format: [%s]",
			ErrNativeCryptographyUnavailable,
			request.SignerMaterial.Format,
		)
	}
}

func buildTaggedTBTCSignerDKGPlaceholderPublicKeyHex(identifier uint16) string {
	// Transitional placeholder until canonical member public keys are available
	// in the native signing request path.
	return fmt.Sprintf("02%04x", identifier)
}

// NativeTBTCSignerDKGPlaceholderPublicKeyHex returns the transitional
// placeholder public key used by tbtc-signer dealer-DKG requests.
func NativeTBTCSignerDKGPlaceholderPublicKeyHex(identifier uint16) string {
	return buildTaggedTBTCSignerDKGPlaceholderPublicKeyHex(identifier)
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
	if request.TaprootMerkleRoot != nil {
		return nil, fmt.Errorf(
			"%w: taproot tweaked signing requires native FROST signer support",
			ErrNativeCryptographyUnavailable,
		)
	}

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

func (btlcnnefsp *buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive) RegisterUnmarshallers(
	channel net.BroadcastChannel,
) {
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

// decodeBuildTaggedTBTCSignerSignature decodes and canonicality-checks a
// BIP-340 signature produced by the native FROST signer. It is shared with the
// interactive ROAST signing drive (the go-forward path), which aggregates the
// signature bytes engine-side and decodes them here.
func decodeBuildTaggedTBTCSignerSignature(signature []byte) (*frost.Signature, error) {
	if len(signature) == 0 {
		return nil, fmt.Errorf("signature is empty")
	}

	// Unmarshal validates length and splits the wire value into R/S. The
	// tbtc-signer material carries a key-group handle rather than the x-only
	// output key, so this layer can only enforce canonical Schnorr encoding.
	// Key-bound verification happens downstream when the wallet output key is
	// available.
	result := &frost.Signature{}
	if err := result.Unmarshal(signature); err != nil {
		return nil, fmt.Errorf("invalid frost signature bytes: [%w]", err)
	}

	serialized := result.Serialize()
	if _, err := schnorr.ParseSignature(serialized[:]); err != nil {
		return nil, fmt.Errorf("non-canonical BIP-340 signature bytes: [%w]", err)
	}

	return result, nil
}
