package tbtc

import (
	"github.com/keep-network/keep-core/pkg/chain"
)

// signingParticipantSelector picks the set of operators qualified for
// a signing attempt. The legacy implementation is the pseudo-random
// retry shuffle in pkg/frost/retry; the RFC-21 Phase-6 migration
// introduces this interface so an alternate ROAST-driven
// implementation can be installed behind the frost_roast_retry build
// tag without touching the call site.
//
// PR 6.4 ships the dispatcher with only the legacy implementation
// installed; Phase 7 wires the ROAST-driven implementation along
// with the supporting AggregateBundle production at the executor-
// adapter layer. Until Phase 7, behaviour is byte-identical to
// pre-RFC-21 retry semantics.
type signingParticipantSelector interface {
	// Select returns the set of operators qualified to participate
	// in the given signing attempt. members is the set of operators
	// whose ready signal was received for this attempt. seed is the
	// per-message retry seed; retryCount is 0-based (i.e. 0 for the
	// first retry). honestThreshold is the group's signing
	// threshold.
	Select(
		members []chain.Address,
		seed int64,
		retryCount uint,
		honestThreshold uint,
		sessionID string,
	) ([]chain.Address, error)
}

// defaultSigningParticipantSelector returns the legacy implementation
// installed by every Phase-6 build (default + frost_roast_retry).
// Phase 7 will install a ROAST-driven implementation in a follow-up
// PR that also wires AggregateBundle production.
func defaultSigningParticipantSelector() signingParticipantSelector {
	return legacySigningParticipantSelector{}
}
