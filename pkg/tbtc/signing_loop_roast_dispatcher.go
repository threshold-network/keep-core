package tbtc

import (
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// signingParticipantSelector picks the set of members included in a
// signing attempt. The legacy implementation is the pseudo-random
// retry shuffle in pkg/protocol/retry; the RFC-21 Phase-6 migration
// introduces this interface so an alternate ROAST-driven
// implementation can be installed behind the frost_roast_retry build
// tag without touching the call site.
//
// PR 6.4 ships the dispatcher with only the legacy implementation
// installed; Phase 7 wires the ROAST-driven implementation along
// with the supporting AggregateBundle production at the executor-
// adapter layer. Until the ROAST selector consumes its transition
// record (RFC-21 Phase 7.3 PR2b), behaviour is byte-identical to
// pre-RFC-21 retry semantics.
//
// RFC-21 Phase 7.3 PR2b-1a made selection MEMBER-LEVEL: Select returns
// the exact included member indices, not a qualified-operator address
// set the caller re-maps. The legacy path computes the indices
// internally (operator selection + member mapping + surplus trim);
// the ROAST path (PR2b) returns the transition's IncludedSet directly.
// This removes the multi-seat precision loss of the old address-based
// path, where one ready seat qualified ALL of an operator's seats --
// including ones ROAST means to park or exclude.
type signingParticipantSelector interface {
	// Select returns the participant selection for the given attempt.
	// readyMembersIndexes is the set of members whose ready signal was
	// received this attempt; signingGroupOperators is the full
	// member->operator roster (index i is member i+1), used by the
	// legacy path to map qualified operators back to members. seed is
	// the per-message retry seed; retryCount is the 0-based LEGACY retry
	// counter (the block-paced loop counter, for the legacy shuffle).
	// roastAttemptNumber is the 0-based COMMITTED ROAST attempt index
	// (advanced only by observed attempts), which the ROAST selector
	// keys its freshness/consume off so block-timing skips do not break
	// the transition chain. honestThreshold is the group's signing
	// threshold. sessionID is the STABLE ROAST session id and
	// memberIndex is the local signer's member. keyGroupID is THIS
	// wallet's FROST key-group handle; the ROAST selector uses it to
	// scope the ROAST-vs-legacy activation decision PER WALLET so a
	// registration-skipped wallet stays on legacy even when a sibling
	// wallet is ROAST-active (empty for legacy/non-native material, and
	// ignored by the legacy selector).
	Select(
		readyMembersIndexes []group.MemberIndex,
		signingGroupOperators chain.Addresses,
		seed int64,
		retryCount uint,
		roastAttemptNumber uint,
		honestThreshold uint,
		sessionID string,
		memberIndex group.MemberIndex,
		keyGroupID string,
	) (participantSelection, error)
}

// participantSelection is one attempt's selection result: the member-level
// included set, plus the members this attempt parked for THIS attempt ONLY. The
// parked set is a subset of the complement of the included set; the loop carries it
// so the attempt after this one reinstates them -- without it a one-attempt
// (transient) park becomes a permanent exclusion (RFC-21 Phase 7.3 PR2b-1b).
//
// Both selectors populate it. The ROAST selector carries forward what the prior
// transition parked; the legacy selector parks every ready member its seeded
// qualification shuffle did not include (see blamelessDrops). Announcement-silent
// members are in neither set -- they are a real liveness fault and stay excluded.
type participantSelection struct {
	includedMembersIndexes          []group.MemberIndex
	transientlyParkedMembersIndexes []group.MemberIndex
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
