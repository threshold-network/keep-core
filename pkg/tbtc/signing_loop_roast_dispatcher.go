package tbtc

import (
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
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
	// threshold. sessionID is the STABLE ROAST session id and
	// memberIndex is the local signer's member; together they key the
	// per-(session, member) transition record the ROAST selector
	// consumes (a multi-seat operator runs one signer per seat, each
	// with its own record).
	Select(
		members []chain.Address,
		seed int64,
		retryCount uint,
		honestThreshold uint,
		sessionID string,
		memberIndex group.MemberIndex,
	) ([]chain.Address, error)
}

// defaultSigningParticipantSelector returns the build-default
// implementation. Default build: the legacy retry shuffle. Tagged
// build (frost_roast_retry, Phase 7.2): a ROAST-driven selector
// that consults the per-session TransitionMessage registry and
// falls back to the legacy selector when no bundle is available.
//
// Defined in build-tagged sibling files
// (signing_loop_selector_*.go) so the right implementation is
// chosen at compile time without runtime branching.
