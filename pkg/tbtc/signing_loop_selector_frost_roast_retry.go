//go:build frost_roast_retry

package tbtc

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// roastSigningParticipantSelector consumes the per-session
// TransitionMessage registry populated by Phase 7.1's bundle
// production. When a bundle is available for the session, it
// invokes EvaluateRoastRetryForSigning to compute the next
// attempt's IncludedSet from the verified evidence. When no bundle
// is available -- typically the first attempt of a session, or
// when the elected coordinator has not yet produced a transition
// message for the current message -- it falls back to the legacy
// retry shuffle.
//
// The selector is installed as defaultSigningParticipantSelector
// when the binary is built with the frost_roast_retry tag and the
// operator opts in via KEEP_CORE_FROST_ROAST_RETRY_ENABLED.
type roastSigningParticipantSelector struct {
	legacy legacySigningParticipantSelector
}

// defaultSigningParticipantSelector in the frost_roast_retry build
// returns the ROAST-driven selector. Its Select method internally
// dispatches to the bundle-based path when a TransitionMessage is
// available and falls back to the legacy shuffle otherwise, so a
// node that has not yet produced any bundles is observationally
// identical to a legacy-only deployment.
func defaultSigningParticipantSelector() signingParticipantSelector {
	return roastSigningParticipantSelector{}
}

// Select chooses the next attempt's qualified operators. When a
// TransitionMessage is present for sessionID, the selector calls
// EvaluateRoastRetryForSigning with a per-call closure resolver
// that maps group.MemberIndex to chain.Address using the supplied
// members slice. When no bundle is present, the selector falls
// back to the legacy retry shuffle.
func (s roastSigningParticipantSelector) Select(
	members []chain.Address,
	seed int64,
	retryCount uint,
	honestThreshold uint,
	sessionID string,
) ([]chain.Address, error) {
	bundle, ok := signing.TransitionBundleForSession(sessionID)
	if !ok || bundle == nil {
		return s.legacy.Select(
			members, seed, retryCount, honestThreshold, sessionID,
		)
	}
	deps, registryOK := signing.RegisteredRoastRetryCoordinator()
	if !registryOK || deps.Coordinator == nil {
		// Should not happen in practice (the bundle was produced
		// by a registered coordinator) but defend against the
		// race anyway.
		return s.legacy.Select(
			members, seed, retryCount, honestThreshold, sessionID,
		)
	}

	// Look up the AttemptHandle bound to this session. The handle
	// identifies the attempt whose bundle we are now consuming;
	// NextAttempt is invoked against it to derive the next
	// AttemptContext's IncludedSet.
	handle, _, handleOK := signing.CurrentAttemptHandleForSession(sessionID)
	if !handleOK {
		return s.legacy.Select(
			members, seed, retryCount, honestThreshold, sessionID,
		)
	}

	resolver := membersResolver(members)
	addresses, _, err := roast.EvaluateRoastRetryForSigning[chain.Address](
		deps.Coordinator,
		handle,
		bundle,
		honestThreshold,
		nil, // DKG public key is recomputed inside Coordinator.NextAttempt; passing nil is acceptable when the bundle's attempt context carries the seed binding.
		resolver,
	)
	if err != nil {
		// Hard-fail per RFC-21 Phase-6 error taxonomy:
		// EvaluateRoastRetryForSigning surfaces
		// ErrAttemptInfeasible (session structurally failed) or
		// resolver errors. Neither is safe to silently fall back
		// to legacy, because honest signers would all observe the
		// same outcome from the same verified bundle. Surface to
		// the caller so the session can be terminated cleanly.
		return nil, fmt.Errorf(
			"roast signing participant selector: %w",
			err,
		)
	}
	return addresses, nil
}

// membersResolver is the per-call closure that maps
// group.MemberIndex to chain.Address using the supplied slice.
// Member indices are 1-based (per the FROST group convention) and
// the address at index 0 of `members` corresponds to member index
// 1.
type membersResolver []chain.Address

func (m membersResolver) For(member group.MemberIndex) (chain.Address, error) {
	if member == 0 {
		return chain.Address(""), fmt.Errorf(
			"member resolver: zero member index",
		)
	}
	idx := int(member) - 1
	if idx >= len(m) {
		return chain.Address(""), fmt.Errorf(
			"member resolver: member index %d exceeds members slice length %d",
			member, len(m),
		)
	}
	return m[idx], nil
}
