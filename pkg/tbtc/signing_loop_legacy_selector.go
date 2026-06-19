package tbtc

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/retry"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// legacySigningParticipantSelector is the pre-RFC-21 implementation:
// it calls the pseudo-random retry shuffle in pkg/frost/retry and maps
// the resulting qualified operators back to the included member
// indices.
//
// Kept as the canonical fallback through Phase 6; Phase 7 may
// remove it once the ROAST-driven retry path is fully wired and
// the readiness manifest flips.
//
// The legacy code is *intentionally retained* through Phase 6 to
// preserve the operational rollback path: if a deployment toggles
// the readiness env var off, this implementation is what the
// dispatcher falls back to.
//
// RFC-21 Phase 7.3 PR2b-1a moved the member-index mapping and the
// surplus trim from the signing-loop INTO this selector so selection
// is member-level (see signingParticipantSelector). The computation is
// byte-identical to the pre-RFC-21 signing_loop.excludedMembersIndexes:
// the same operator selection, the same per-member inclusion rule, and
// the same seeded surplus shuffle (seed offset by the attempt counter,
// which is retryCount+1), just producing the included set directly
// instead of the excluded complement.
type legacySigningParticipantSelector struct{}

func (legacySigningParticipantSelector) Select(
	readyMembersIndexes []group.MemberIndex,
	signingGroupOperators chain.Addresses,
	seed int64,
	retryCount uint,
	_ uint, // roastAttemptNumber: legacy diversifies by retryCount, not the ROAST counter
	honestThreshold uint,
	_ string,
	_ group.MemberIndex,
) (participantSelection, error) {
	// Build the input the retry shuffle expects: one operator address
	// per ready member (an operator controlling k ready members appears
	// k times, matching the pre-RFC-21 input to the algorithm).
	readyOperators := make([]chain.Address, 0, len(readyMembersIndexes))
	for _, memberIndex := range readyMembersIndexes {
		readyOperators = append(
			readyOperators,
			signingGroupOperators[memberIndex-1],
		)
	}

	qualifiedOperators, err := retry.EvaluateRetryParticipantsForSigning(
		readyOperators,
		seed,
		retryCount,
		honestThreshold,
	)
	if err != nil {
		return participantSelection{}, fmt.Errorf(
			"legacy participant selector: random operator selection failed: %w",
			err,
		)
	}
	qualifiedOperatorsSet := chain.Addresses(qualifiedOperators).Set()

	readyMembersSet := make(map[group.MemberIndex]bool, len(readyMembersIndexes))
	for _, memberIndex := range readyMembersIndexes {
		readyMembersSet[memberIndex] = true
	}

	// Include a member iff its operator is qualified and the member
	// itself announced readiness. This is the per-member inclusion rule
	// the old signing_loop.excludedMembersIndexes applied; doing it here
	// keeps the ROAST-excluded seat of a multi-seat qualified operator
	// from being pulled back in (the precision the address-based path
	// lost).
	includedMembersIndexes := make([]group.MemberIndex, 0, len(signingGroupOperators))
	for i, operator := range signingGroupOperators {
		memberIndex := group.MemberIndex(i + 1)
		if qualifiedOperatorsSet[operator] && readyMembersSet[memberIndex] {
			includedMembersIndexes = append(includedMembersIndexes, memberIndex)
		}
	}

	// Trim to the smallest required count of signing members for
	// performance, dropping the surplus by a seeded shuffle. The shuffle
	// seed matches the pre-RFC-21 loop: seed + attemptCounter, and
	// attemptCounter == retryCount + 1.
	if len(includedMembersIndexes) > int(honestThreshold) {
		// #nosec G404 (insecure random number source (rand))
		// Shuffling does not require secure randomness.
		rng := rand.New(rand.NewSource(seed + int64(retryCount) + 1))
		// Sort in ascending order first so the shuffle (and thus the
		// retained set) is independent of the iteration order above.
		sort.Slice(includedMembersIndexes, func(i, j int) bool {
			return includedMembersIndexes[i] < includedMembersIndexes[j]
		})
		rng.Shuffle(len(includedMembersIndexes), func(i, j int) {
			includedMembersIndexes[i], includedMembersIndexes[j] =
				includedMembersIndexes[j], includedMembersIndexes[i]
		})
		includedMembersIndexes = includedMembersIndexes[:honestThreshold]
	}

	// Return the included set in canonical ascending order (matches the
	// pre-RFC-21 AttemptInfo.IncludedMembersIndexes, which was the
	// ascending complement of the sorted excluded set).
	sort.Slice(includedMembersIndexes, func(i, j int) bool {
		return includedMembersIndexes[i] < includedMembersIndexes[j]
	})
	// The legacy path never parks: parking is a ROAST-transition concept.
	return participantSelection{includedMembersIndexes: includedMembersIndexes}, nil
}
