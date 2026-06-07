//go:build frost_native

package signing

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
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
// transitional primitive that preserves legacy bridge execution for
// `frost-uniffi-v1` payloads. `frost-tbtc-signer-v1` uses the coarse signing
// flow for bootstrap engine versions and falls back to legacy signing for
// unsupported or failed coarse-path executions. Unsupported
// `frost-uniffi-v2` material is rejected explicitly because it cannot produce
// Taproot-tweaked signatures; accepting it would allow new deposits to a
// wallet that cannot sweep them.
type buildTaggedLegacyCompatibleNativeExecutionFFISigningPrimitive struct{}

const buildTaggedTBTCSignerVersionPrefix = "tbtc-signer/"
const buildTaggedTBTCSignerBootstrapVersionPrerelease = "bootstrap"
const buildTaggedTBTCSignerSyntheticContributionDomain = "tbtc-signer-bootstrap-contribution-v1"
const buildTaggedTBTCSignerMessageTypePrefix = "frost_signing/native_tbtc_signer/"
const buildTaggedTBTCSignerConsumedAttemptReplayErrorFragment = "already consumed for sign attempt"

// buildTaggedTBTCSignerConsumedAttemptReplayErrorCode is the structured Rust
// `ErrorResponse.code` value emitted by tbtc-signer when an `attempt_id` is
// reused after consumption. Preferred over substring matching on the message
// because the code is contract-stable: see `EngineError::code()` in the
// `tbtc-signer` crate.
const buildTaggedTBTCSignerConsumedAttemptReplayErrorCode = "consumed_attempt_replay"

// buildTaggedTBTCSignerLegacyValidationErrorCode is the structured code
// emitted by tbtc-signer builds that pre-date the dedicated replay variant.
// Those builds route the replay path through `EngineError::Validation`, so
// the code on the wire is `validation_error` and the substring check on the
// message is the only signal callers have. Once the rolling upgrade is past
// the minimum-supported signer version, this code can be retired.
const buildTaggedTBTCSignerLegacyValidationErrorCode = "validation_error"

type nativeTBTCSignerVersionedEngine interface {
	Version() (string, error)
}

type buildTaggedTBTCSignerRoundContributionMessage struct {
	SenderIDValue          uint32 `json:"senderID"`
	SessionIDValue         string `json:"sessionID"`
	ContributionIdentifier uint16 `json:"contributionIdentifier"`
	ContributionData       []byte `json:"contributionData"`
	// AttemptContextHash -- see nativeFROSTRoundOneCommitmentMessage
	// for the RFC-21 Phase 1 migration contract.
	AttemptContextHash []byte `json:"attemptContextHash,omitempty"`
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

	if err := validateAttemptContextHashField(
		bttsrcm.AttemptContextHash,
	); err != nil {
		return err
	}

	return nil
}

func (bttsrcm *buildTaggedTBTCSignerRoundContributionMessage) SetAttemptContextHash(
	hash [AttemptContextHashFieldLength]byte,
) {
	bttsrcm.AttemptContextHash = attemptContextHashFieldFromArray(hash)
}

func (bttsrcm *buildTaggedTBTCSignerRoundContributionMessage) GetAttemptContextHash() (
	[AttemptContextHashFieldLength]byte, bool,
) {
	return attemptContextHashFieldToArray(bttsrcm.AttemptContextHash)
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
		return nil, fmt.Errorf(
			"%w: unsupported UniFFI FROST signer material format [%s]; it cannot sweep Taproot deposits; use [%s]",
			ErrUnsupportedSignerMaterialFormat,
			NativeSignerMaterialFormatFrostUniFFIV2,
			NativeSignerMaterialFormatFrostTBTCSignerV1,
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

	// Scaffold persistence-vs-execution gate. The resolver in #3959 refuses to
	// BUILD scaffold-era signer material without the env opt-in, but material
	// persisted from a previous opted-in session can still drive this signing
	// path on later runs after the operator has unset the flag. Refuse to
	// enter the FFI scaffold path (which feeds placeholder participant
	// pubkeys into RunDKG) when the payload is scaffold-era and the operator
	// has not actively opted in for this process. The check is per-call (not
	// cached) so flipping the env back unset recovers fail-closed behavior
	// without a restart, matching the contract documented on
	// AcceptScaffoldKeyGroupEnvVar.
	if payload.KeyGroupSource == NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey &&
		!AcceptScaffoldKeyGroupEnabled() {
		return nil, fmt.Errorf(
			"%w: refusing to drive the tbtc-signer FFI signing path with "+
				"scaffold-era %q signer material; set %s=true to opt in for "+
				"local/CI use only, never in production",
			ErrNativeCryptographyUnavailable,
			NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
			AcceptScaffoldKeyGroupEnvVar,
		)
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

	dkgParticipants, dkgThreshold, err := buildTaggedTBTCSignerRunDKGInputsForPayload(
		payload,
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

	dkgResult, err := runNativeTBTCSignerDKG(
		nativeEngine,
		request.SessionID,
		dkgParticipants,
		dkgThreshold,
		payload.DKGSeedHex,
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

	// Prefer the structured `code` field from the FFI error envelope when it
	// is reachable through the error chain. The Rust signer's
	// `EngineError::code()` value `"consumed_attempt_replay"` is a
	// contract-stable identifier; this check survives any cosmetic rewording
	// of the human-readable message on either side.
	//
	// Older signer builds emit `validation_error` for the replay path with
	// the legacy wording in the message. For those, fall through to the
	// substring check restricted to the structured message field so a
	// `validation_error` carrying an unrelated error chain string cannot be
	// mistaken for a replay. Any other recognized code is authoritative.
	var structured *buildTaggedTBTCSignerStructuredError
	if errors.As(err, &structured) && structured.Code != "" {
		switch structured.Code {
		case buildTaggedTBTCSignerConsumedAttemptReplayErrorCode:
			return true
		case buildTaggedTBTCSignerLegacyValidationErrorCode:
			return messageMatchesLegacyConsumedAttemptReplay(structured.Message)
		default:
			return false
		}
	}

	// No structured code reachable — the error chain pre-dates the FFI
	// envelope. The legacy wording is preserved by the current tbtc-signer
	// release so this branch continues to work during the rolling upgrade
	// window. Match on the whole rendered string for maximum compatibility.
	return messageMatchesLegacyConsumedAttemptReplay(err.Error())
}

func messageMatchesLegacyConsumedAttemptReplay(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "attempt_id") &&
		strings.Contains(lower, buildTaggedTBTCSignerConsumedAttemptReplayErrorFragment)
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

func buildTaggedTBTCSignerRunDKGInputsForPayload(
	payload *NativeTBTCSignerMaterialPayload,
	request *NativeExecutionFFISigningRequest,
	includedMembersIndexes []group.MemberIndex,
) ([]NativeTBTCSignerDKGParticipant, uint16, error) {
	if payload != nil &&
		payload.KeyGroupSource == NativeTBTCSignerKeyGroupSourceDKGPersisted {
		if len(payload.DKGParticipants) < 2 {
			return nil, 0, fmt.Errorf(
				"persisted tbtc-signer DKG participants are insufficient",
			)
		}
		if payload.DKGThreshold == 0 {
			return nil, 0, fmt.Errorf(
				"persisted tbtc-signer DKG threshold is zero",
			)
		}
		if int(payload.DKGThreshold) > len(payload.DKGParticipants) {
			return nil, 0, fmt.Errorf(
				"persisted tbtc-signer DKG threshold exceeds participant count: [%v] > [%v]",
				payload.DKGThreshold,
				len(payload.DKGParticipants),
			)
		}

		participants := make(
			[]NativeTBTCSignerDKGParticipant,
			len(payload.DKGParticipants),
		)
		copy(participants, payload.DKGParticipants)

		return participants, payload.DKGThreshold, nil
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

// NativeTBTCSignerDKGPlaceholderPublicKeyHex returns the transitional
// placeholder public key used by tbtc-signer dealer-DKG requests.
func NativeTBTCSignerDKGPlaceholderPublicKeyHex(identifier uint16) string {
	return buildTaggedTBTCSignerDKGPlaceholderPublicKeyHex(identifier)
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
		// Refuse the substitution by default so a production deployment that
		// somehow ended up with placeholder material does not silently route
		// signing through whatever key group the Rust side happens to return.
		// The operator must explicitly opt into the scaffold path via
		// AcceptScaffoldKeyGroupEnvVar; the env-var check is per-call (not
		// cached) so flipping it off recovers fail-closed behavior without a
		// restart.
		if !AcceptScaffoldKeyGroupEnabled() {
			return "", false, fmt.Errorf(
				"tbtc-signer key group source %q is scaffold-era placeholder "+
					"material and may not be silently substituted with the "+
					"RunDKG output; set %s=true to opt in for local/CI use "+
					"only, never in production",
				NativeTBTCSignerKeyGroupSourceLegacyWalletPubKey,
				AcceptScaffoldKeyGroupEnvVar,
			)
		}
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

	messageDigest, err := messageDigestFromBigInt(request.Message)
	if err != nil {
		return nil, fmt.Errorf("invalid request message digest: [%v]", err)
	}
	messageBytes := make([]byte, len(messageDigest))
	copy(messageBytes, messageDigest[:])

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
		request.TaprootMerkleRoot,
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
		request.TaprootMerkleRoot,
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
	setMessageAttemptContextHashIfBound(roundContributionMessage, request.SessionID)

	if err := request.Channel.Send(
		ctx,
		roundContributionMessage,
		net.BackoffRetransmissionStrategy,
	); err != nil {
		return nil, fmt.Errorf("cannot send round contribution message: [%w]", err)
	}

	// RFC-21 Phase 4.2/4.3: recorder comes from the roast-retry
	// registry; deferred submission pushes the snapshot into
	// Coordinator.RecordEvidence at end-of-collect. NoOp fallback
	// when nothing is registered preserves Phase 2 receive
	// semantics.
	contributionsRecorder := roastRetryRecorderForCollect()
	defer submitSnapshotIfActive(request.SessionID, contributionsRecorder)
	peerMessages, err := collectBuildTaggedTBTCSignerRoundContributionMessages(
		ctx,
		request,
		includedMembersSet,
		includedMembersIndexes,
		contributionsRecorder,
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
	evidence attempt.EvidenceRecorder,
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
			evidence.RecordReject(payload.SenderID(), "validation_gate_rejected")
			return
		}

		if err := verifyMessageAttemptContextHash(payload, request.SessionID); err != nil {
			evidence.RecordReject(payload.SenderID(), "attempt_context_hash_mismatch")
			return
		}

		_ = enqueueOrRecordOverflow(payload, messageChan, evidence)
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
			// First-write-wins / equal-or-reject. A peer that retransmits the
			// same contribution is idempotent; a peer that mutates its own
			// contribution after the first send is a ROAST evidence concern
			// and must not be allowed to overwrite the persisted view.
			senderID := message.SenderID()
			if existing, ok := receivedMessages[senderID]; ok {
				if !buildTaggedTBTCSignerRoundContributionMessagesEqual(
					existing,
					message,
				) {
					evidence.RecordConflict(senderID)
					protocolLogger.Warnf(
						"dropping conflicting tbtc-signer round contribution "+
							"from sender [%d]; first-write-wins keeps the "+
							"originally accepted contribution",
						senderID,
					)
				}
				continue
			}
			receivedMessages[senderID] = message
		}
	}

	return receivedMessages, nil
}

func buildTaggedTBTCSignerRoundContributionMessagesEqual(
	left, right *buildTaggedTBTCSignerRoundContributionMessage,
) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.SenderIDValue == right.SenderIDValue &&
		left.SessionIDValue == right.SessionIDValue &&
		left.ContributionIdentifier == right.ContributionIdentifier &&
		bytes.Equal(left.ContributionData, right.ContributionData) &&
		bytes.Equal(left.AttemptContextHash, right.AttemptContextHash)
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

func runNativeTBTCSignerDKG(
	nativeEngine NativeTBTCSignerEngine,
	sessionID string,
	participants []NativeTBTCSignerDKGParticipant,
	threshold uint16,
	dkgSeedHex string,
) (*NativeTBTCSignerDKGResult, error) {
	if dkgSeedHex == "" {
		return nativeEngine.RunDKG(sessionID, participants, threshold)
	}

	seededEngine, ok := nativeEngine.(NativeTBTCSignerSeededDKGEngine)
	if !ok {
		return nil, fmt.Errorf(
			"native tbtc-signer engine does not support seeded RunDKG",
		)
	}

	return seededEngine.RunDKGWithSeed(
		sessionID,
		participants,
		threshold,
		dkgSeedHex,
	)
}
