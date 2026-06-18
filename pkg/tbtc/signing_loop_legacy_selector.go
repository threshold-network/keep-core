package tbtc

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/retry"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// legacySigningParticipantSelector is the pre-RFC-21 implementation:
// it calls the pseudo-random retry shuffle in pkg/frost/retry.
// Kept as the canonical fallback through Phase 6; Phase 7 may
// remove it once the ROAST-driven retry path is fully wired and
// the readiness manifest flips.
//
// The legacy code is *intentionally retained* through Phase 6 to
// preserve the operational rollback path: if a deployment toggles
// the readiness env var off, this implementation is what the
// dispatcher falls back to.
type legacySigningParticipantSelector struct{}

func (legacySigningParticipantSelector) Select(
	members []chain.Address,
	seed int64,
	retryCount uint,
	honestThreshold uint,
	_ string,
	_ group.MemberIndex,
) ([]chain.Address, error) {
	qualifiedOperators, err := retry.EvaluateRetryParticipantsForSigning(
		members,
		seed,
		retryCount,
		honestThreshold,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"legacy participant selector: random operator selection failed: %w",
			err,
		)
	}
	return qualifiedOperators, nil
}
