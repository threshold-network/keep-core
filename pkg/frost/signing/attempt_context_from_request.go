//go:build frost_native

package signing

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// ErrAttemptContextConstruction is the sentinel error class returned
// by BuildAttemptContextFromRequest for any failure during
// construction. Callers can match with errors.Is to distinguish
// it from runtime ROAST errors.
var ErrAttemptContextConstruction = errors.New(
	"attempt context: construction failed",
)

// BuildAttemptContextFromRequest converts a
// NativeExecutionFFISigningRequest into an attempt.AttemptContext
// suitable for Coordinator.BeginAttempt. The conversion:
//
//   - SessionID, AttemptNumber, IncludedSet, ExcludedSet come from
//     the request and its Attempt sub-struct directly.
//   - TransientlyParked is empty: the existing Attempt struct does
//     not carry parking info. Phase-7+ orchestration that drives
//     multi-attempt sessions will need to thread parking metadata
//     through; Phase 6 only handles attempt-zero shape.
//   - MessageDigest is the request.Message bytes left-padded with
//     zeros to 32 bytes, then truncated if longer. In BIP-340
//     production, request.Message is already a 32-byte digest of
//     the tagged payload, so padding is a no-op.
//   - DkgGroupPublicKey is extracted via
//     ExtractDkgGroupPublicKeyFromMaterial.
//   - KeyGroupID is derived from the raw FrostTBTCSignerV1 KeyGroup string
//     identifier, which is already a canonical per-group handle.
//   - AttemptSeed = SHA256(DkgGroupPublicKey || SessionID ||
//     MessageDigest) per RFC-21 Decision 2.
//
// Critically, the FFI signer material is decoded *first* so any
// extraction failure is surfaced before the AttemptContext is
// constructed. This enforces the ordering Gemini flagged in the
// Phase-6 design review: AttemptContext must never be built from
// undecoded material because the seed derivation would silently
// fail.
//
// Returns ErrAttemptContextConstruction-wrapped errors for any
// failure during the construction. Returns ErrUnsupportedSignerMaterialFormat
// (via errors.Is) when the material's format is not extractable
// (e.g. FrostUniFFIV1 or unsupported FrostUniFFIV2 today).
func BuildAttemptContextFromRequest(
	request *NativeExecutionFFISigningRequest,
) (attempt.AttemptContext, error) {
	if request == nil {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: request is nil",
			ErrAttemptContextConstruction,
		)
	}
	if request.Message == nil {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: request message is nil",
			ErrAttemptContextConstruction,
		)
	}
	if request.SignerMaterial == nil {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: signer material is nil",
			ErrAttemptContextConstruction,
		)
	}
	if request.Attempt == nil {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: attempt metadata is nil",
			ErrAttemptContextConstruction,
		)
	}

	// Strict ordering: extract DKG group public key (which decodes
	// the signer material) BEFORE deriving the context. A failure
	// here propagates directly without leaving a half-built
	// context.
	dkgPub, err := ExtractDkgGroupPublicKeyFromMaterial(request.SignerMaterial)
	if err != nil {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: %w",
			ErrAttemptContextConstruction,
			err,
		)
	}

	keyGroupID, err := deriveKeyGroupID(request.SignerMaterial, dkgPub)
	if err != nil {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: %w",
			ErrAttemptContextConstruction,
			err,
		)
	}

	digest, err := messageDigestFromBigInt(request.Message)
	if err != nil {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: %w",
			ErrAttemptContextConstruction,
			err,
		)
	}

	// AttemptNumber on the keep-core Attempt struct is 1-based
	// (1 = first attempt). RFC-21's AttemptContext.AttemptNumber is
	// 0-based. Convert by subtracting 1 (Attempt.Number must be
	// >= 1).
	if request.Attempt.Number == 0 {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: request.Attempt.Number is zero (must be >= 1)",
			ErrAttemptContextConstruction,
		)
	}
	attemptNumber := uint32(request.Attempt.Number - 1)

	// Prefer the STABLE ROAST session id so ctx.SessionID -- and everything
	// keyed off it (the orchestration handle + transition-record registries,
	// the selector lookup, the interactive engine session) -- is stable across
	// attempts; the per-attempt SessionID would make the next attempt's selector
	// unable to find the previous attempt's transition record. Fall back to
	// SessionID when the caller does not drive ROAST orchestration.
	roastSessionID := request.RoastSessionID
	if roastSessionID == "" {
		roastSessionID = request.SessionID
	}

	// RFC-21 Phase 7.3 PR2b-1b: carry transient parking. Attempt.Excluded is the
	// full "not participating now" set (permanent-excluded plus transiently-
	// parked); split it so the AttemptContext distinguishes them, or NextAttempt
	// would treat a one-attempt park as a permanent exclusion and never reinstate
	// the member.
	transientlyParked := request.Attempt.TransientlyParkedMembersIndexes
	permanentExcluded := membersDifference(
		request.Attempt.ExcludedMembersIndexes,
		transientlyParked,
	)
	ctx, err := attempt.NewAttemptContextWithParking(
		roastSessionID,
		keyGroupID,
		dkgPub,
		digest,
		attemptNumber,
		request.Attempt.IncludedMembersIndexes,
		permanentExcluded,
		transientlyParked,
	)
	if err != nil {
		return attempt.AttemptContext{}, fmt.Errorf(
			"%w: %w",
			ErrAttemptContextConstruction,
			err,
		)
	}
	return ctx, nil
}

// membersDifference returns the members in `all` not present in `remove`,
// preserving the order of `all`. Used to split an attempt's "not participating"
// set into permanently-excluded vs transiently-parked.
func membersDifference(all, remove []group.MemberIndex) []group.MemberIndex {
	if len(remove) == 0 {
		// Return a fresh slice, never the caller's backing array: the result
		// becomes permanentExcluded on the context-build path and must not alias
		// request.Attempt.ExcludedMembersIndexes (shared via the request template).
		return append([]group.MemberIndex(nil), all...)
	}
	removeSet := make(map[group.MemberIndex]bool, len(remove))
	for _, m := range remove {
		removeSet[m] = true
	}
	out := make([]group.MemberIndex, 0, len(all))
	for _, m := range all {
		if !removeSet[m] {
			out = append(out, m)
		}
	}
	return out
}

// KeyGroupIDFromSignerMaterial returns the canonical FROST key-group handle for the
// given native signer material -- the exact string BuildAttemptContextFromRequest
// stores as AttemptContext.KeyGroupID. ROAST-retry wiring uses it to scope the
// coordinator registry by wallet key group at the registration and lookup sites that
// hold signer material rather than a fully-built AttemptContext (the interactive
// drive registration and the transition-controller). Returns an error for material
// whose format has no derivable key-group handle.
func KeyGroupIDFromSignerMaterial(signerMaterial *NativeSignerMaterial) (string, error) {
	if signerMaterial == nil {
		return "", fmt.Errorf("key group id: signer material is nil")
	}
	// deriveKeyGroupID ignores the DKG public key for the only supported format
	// (FrostTBTCSignerV1, whose KeyGroup string is the handle); pass nil rather than
	// re-extracting it, and let deriveKeyGroupID reject unsupported formats.
	return deriveKeyGroupID(signerMaterial, nil)
}

// deriveKeyGroupID computes the AttemptContext KeyGroupID field
// from the signer material plus the already-extracted DKG group
// public key. The derivation is format-aware:
//
//   - FrostTBTCSignerV1: the raw KeyGroup string from the tbtc-
//     signer material. That string is the canonical handle.
//
// Returns an error for unknown formats; the caller will already
// have rejected unsupported formats via ExtractDkgGroupPublicKeyFromMaterial,
// so reaching the default arm here is an internal consistency
// error.
func deriveKeyGroupID(
	signerMaterial *NativeSignerMaterial,
	dkgPub []byte,
) (string, error) {
	switch signerMaterial.Format {
	case NativeSignerMaterialFormatFrostTBTCSignerV1:
		payload, err := decodeBuildTaggedTBTCSignerMaterialPayload(signerMaterial)
		if err != nil {
			return "", fmt.Errorf("derive key group id: %w", err)
		}
		return payload.KeyGroup, nil
	default:
		return "", fmt.Errorf(
			"derive key group id: cannot derive id from format %q",
			signerMaterial.Format,
		)
	}
}

// messageDigestFromBigInt converts a *big.Int message to the
// 32-byte digest shape AttemptContext expects. Big-int values
// shorter than 32 bytes are left-padded with zeros (big.Int.Bytes
// strips leading zeros). Values longer than 32 bytes return an
// error -- a real digest never exceeds 32 bytes for SHA-256.
func messageDigestFromBigInt(
	message *big.Int,
) ([attempt.MessageDigestLength]byte, error) {
	var out [attempt.MessageDigestLength]byte
	if message == nil {
		return out, fmt.Errorf("message is nil")
	}
	bz := message.Bytes()
	if len(bz) > attempt.MessageDigestLength {
		return out, fmt.Errorf(
			"message digest length %d exceeds expected %d",
			len(bz),
			attempt.MessageDigestLength,
		)
	}
	// Left-pad with zeros: big.Int.Bytes strips leading zeros, so a
	// 32-byte digest with a leading zero byte returns a 31-byte
	// slice. Copy into the tail of `out` to restore canonical
	// alignment.
	copy(out[attempt.MessageDigestLength-len(bz):], bz)
	return out, nil
}
