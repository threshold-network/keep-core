//go:build !frost_roast_retry

package signing

import (
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
)

// SetCurrentAttemptHandleForSession is a no-op in the default build:
// the receive loops will never find a handle for any session, so the
// snapshot submission path is dormant. The build-tagged
// implementation does the real registration.
func SetCurrentAttemptHandleForSession(
	_ string,
	_ roast.AttemptHandle,
	_ attempt.AttemptContext,
) {
}

// ClearCurrentAttemptHandleForSession is a no-op in the default
// build.
func ClearCurrentAttemptHandleForSession(_ string) {}

// ResetSessionHandleRegistryForTest is a no-op in the default
// build.
func ResetSessionHandleRegistryForTest() {}

// StartSessionHandleSweeper is a no-op in the default build: with
// no real registry there is nothing to sweep.
func StartSessionHandleSweeper() {}

// currentAttemptHandleForCollect always returns ok=false in the
// default build, so submitSnapshotIfActive exits without attempting
// the RecordEvidence call.
func currentAttemptHandleForCollect(
	_ string,
) (roast.AttemptHandle, attempt.AttemptContext, bool) {
	return roast.AttemptHandle{}, attempt.AttemptContext{}, false
}

// CurrentAttemptHandleForSession is the exported alias for
// callers outside the package (e.g. the ROAST-driven signing
// selector in pkg/tbtc). In the default build it is a no-op that
// always returns ok=false.
func CurrentAttemptHandleForSession(
	sessionID string,
) (roast.AttemptHandle, attempt.AttemptContext, bool) {
	return currentAttemptHandleForCollect(sessionID)
}
