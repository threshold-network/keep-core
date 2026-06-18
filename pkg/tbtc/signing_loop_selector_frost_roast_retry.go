//go:build frost_roast_retry

package tbtc

import (
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// roastSigningParticipantSelector is installed as the default participant
// selector in the frost_roast_retry build. In RFC-21 Phase 7.3 PR2a it
// delegates to the legacy retry shuffle: the cross-attempt ROAST transition
// record is now PRODUCED and stored (the data foundation this PR lays), but
// CONSUMING it to drive participant selection is deferred to PR2b.
//
// Consuming a purely-local record here would FRACTURE the signing group: only
// the elected coordinator's cleanup produces a record locally, so on a retry it
// would take the ROAST branch while every peer (no local record) fell back to
// legacy -- yielding divergent IncludedSets across honest nodes. PR2b adds the
// snapshot/transition bus exchange so every signer holds the record, AND selects
// at the member level (the legacy address-based path loses ROAST's per-member
// decision under multi-seat operators and partial readiness). Until then this
// selector is observationally identical to the legacy one.
type roastSigningParticipantSelector struct {
	legacy legacySigningParticipantSelector
}

// defaultSigningParticipantSelector in the frost_roast_retry build returns the
// ROAST selector (which, in PR2a, delegates to legacy -- see the type doc).
func defaultSigningParticipantSelector() signingParticipantSelector {
	return roastSigningParticipantSelector{}
}

// Select delegates to the legacy retry shuffle in PR2a. The sessionID +
// memberIndex parameters are threaded through the interface now so PR2b can wire
// distributed, member-level consumption of the transition record without
// touching the call site again.
func (s roastSigningParticipantSelector) Select(
	members []chain.Address,
	seed int64,
	retryCount uint,
	honestThreshold uint,
	sessionID string,
	memberIndex group.MemberIndex,
) ([]chain.Address, error) {
	return s.legacy.Select(
		members, seed, retryCount, honestThreshold, sessionID, memberIndex,
	)
}
