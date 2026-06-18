//go:build !frost_roast_retry

package signing

import "github.com/keep-network/keep-core/pkg/protocol/group"

// RecordRoastTransition is a no-op in the default build: the per-(session,
// member) transition-record registry is not active without the
// frost_roast_retry tag. The signing-loop ROAST selector (only compiled into
// the frost_roast_retry build) reads this registry; in the default build the
// legacy retry shuffle is always used.
func RecordRoastTransition(_ string, _ group.MemberIndex, _ RoastTransitionRecord) {}

// RoastTransitionForSession returns (zero, false) in the default build,
// signalling to callers "no ROAST record; use the legacy retry shuffle".
func RoastTransitionForSession(_ string, _ group.MemberIndex) (RoastTransitionRecord, bool) {
	return RoastTransitionRecord{}, false
}

// ClearRoastTransitionForSession is a no-op in the default build.
func ClearRoastTransitionForSession(_ string, _ group.MemberIndex) {}

// ResetRoastTransitionRegistryForTest is a no-op in the default build.
func ResetRoastTransitionRegistryForTest() {}
