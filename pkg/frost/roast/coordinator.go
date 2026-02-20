package roast

import (
	"fmt"
	"math/rand"
	"sort"

	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// SelectCoordinator deterministically picks a coordinator from the included
// members set for a given attempt.
//
// Selection is pseudo-random but stable across all participants that use the
// same attempt seed and attempt number.
func SelectCoordinator(
	includedMembersIndexes []group.MemberIndex,
	attemptSeed int64,
	attemptNumber uint,
) (group.MemberIndex, error) {
	if len(includedMembersIndexes) == 0 {
		return 0, fmt.Errorf("cannot select coordinator from empty member set")
	}

	members := make([]group.MemberIndex, len(includedMembersIndexes))
	copy(members, includedMembersIndexes)

	// Sort first to make sure selection result is independent from input order.
	sort.Slice(members, func(i, j int) bool {
		return members[i] < members[j]
	})

	// #nosec G404 (insecure random number source (rand))
	// Coordinator shuffling needs deterministic, not cryptographic randomness.
	rng := rand.New(rand.NewSource(attemptSeed + int64(attemptNumber)))
	rng.Shuffle(len(members), func(i, j int) {
		members[i], members[j] = members[j], members[i]
	})

	return members[0], nil
}
