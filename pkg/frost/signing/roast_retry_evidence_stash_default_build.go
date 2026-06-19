//go:build !frost_roast_retry

package signing

import (
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// stashPendingEvidence is a no-op in the default build: with no ROAST-retry
// orchestration there is never a session-handle binding, so submitSnapshotIfActive
// returns before reaching this call. The build-tagged implementation does the real
// stashing the transition exchange's BroadcastForcedSnapshot consumes.
func stashPendingEvidence(
	_ string,
	_ group.MemberIndex,
	_ [attempt.MessageDigestLength]byte,
	_ attempt.Evidence,
) {
}

// stashPendingCoordinatorProofs is a no-op in the default build, mirroring
// stashPendingEvidence: the interactive drive path (frost_native) calls it, but
// with no ROAST-retry orchestration there is no transition exchange to consume the
// proofs. The build-tagged implementation does the real stashing.
func stashPendingCoordinatorProofs(
	_ string,
	_ group.MemberIndex,
	_ [attempt.MessageDigestLength]byte,
	_ [][]byte,
) {
}
