//go:build frost_roast_retry

package tbtc

import (
	"errors"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// roastSigningParticipantSelector is installed as the default participant
// selector in the frost_roast_retry build. RFC-21 Phase 7.3 PR2b-1b C3 activates
// it: on a retry under active ROAST retry it consumes this seat's stored
// transition record to select the next attempt's included set at the member
// level; otherwise it uses the legacy retry shuffle.
//
// The transition record is produced by the (C1/C2) observe + bus exchange: every
// signer holds the verified record for the prior attempt, so consuming it here
// no longer fractures the group the way a purely-local record would have. The
// crucial discipline is FAIL-CLOSED: when a committed ROAST attempt expected a
// transition that did not arrive, this selector returns the error (terminating
// the retry loop) rather than falling back to legacy -- mixed ROAST/legacy
// selection across honest nodes is the fracture class.
type roastSigningParticipantSelector struct {
	legacy legacySigningParticipantSelector
}

// defaultSigningParticipantSelector in the frost_roast_retry build returns the
// ROAST selector.
func defaultSigningParticipantSelector() signingParticipantSelector {
	return roastSigningParticipantSelector{}
}

// Select consumes the stored transition record when ROAST retry drives this
// attempt, and otherwise uses the legacy shuffle. See
// signing.ConsumeRoastTransitionForSelection for the three-way contract
// (consume / uniform legacy fallback / fail closed).
func (s roastSigningParticipantSelector) Select(
	readyMembersIndexes []group.MemberIndex,
	signingGroupOperators chain.Addresses,
	seed int64,
	retryCount uint,
	roastAttemptNumber uint,
	honestThreshold uint,
	sessionID string,
	memberIndex group.MemberIndex,
) (participantSelection, error) {
	included, parked, err := signing.ConsumeRoastTransitionForSelection(
		sessionID,
		memberIndex,
		roastAttemptNumber,
		honestThreshold,
	)
	if err == nil {
		return participantSelection{
			includedMembersIndexes:          included,
			transientlyParkedMembersIndexes: parked,
		}, nil
	}

	// Initial ROAST attempt or ROAST retry inactive: a uniform legacy fallback
	// every honest node makes identically.
	if errors.Is(err, signing.ErrRoastSelectionFallBackToLegacy) {
		return s.legacy.Select(
			readyMembersIndexes,
			signingGroupOperators,
			seed,
			retryCount,
			roastAttemptNumber,
			honestThreshold,
			sessionID,
			memberIndex,
		)
	}

	// A committed ROAST attempt expected a transition that did not arrive (or
	// NextAttempt was infeasible). FAIL CLOSED: surfacing the error terminates the
	// retry loop, and the outer layer retries the whole signing. Falling back to
	// legacy here -- while peers that DID receive the transition select from it --
	// would split the signing group into divergent included sets.
	return participantSelection{}, err
}
