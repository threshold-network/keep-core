//go:build !frost_roast_retry

package signing

import "github.com/keep-network/keep-core/pkg/frost/roast"

// RecordTransitionBundleForSession is a no-op in the default build:
// the per-session bundle registry is not active without the
// frost_roast_retry tag. The signing-loop ROAST selector (when
// installed via Phase 7's build) reads this registry to consume
// the most recent TransitionMessage for a message.
func RecordTransitionBundleForSession(_ string, _ *roast.TransitionMessage) {}

// TransitionBundleForSession returns (nil, false) in the default
// build, signalling to callers that no ROAST bundle is available
// and the legacy retry shuffle should be used.
func TransitionBundleForSession(_ string) (*roast.TransitionMessage, bool) {
	return nil, false
}

// ClearTransitionBundleForSession is a no-op in the default build.
func ClearTransitionBundleForSession(_ string) {}

// ResetTransitionBundleRegistryForTest is a no-op in the default
// build.
func ResetTransitionBundleRegistryForTest() {}
