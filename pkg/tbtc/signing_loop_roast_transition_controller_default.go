//go:build !(frost_native && frost_roast_retry)

package tbtc

import (
	"context"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost/signing"
)

// newRoastTransitionController returns nil in builds without both the
// frost_native and frost_roast_retry tags: there is no ROAST transition
// machinery to drive, so the signing loop skips every transition step and uses
// the legacy retry shuffle. The nil controller is the same signal the active
// build uses for a nil request template.
func newRoastTransitionController(
	_ context.Context,
	_ log.StandardLogger,
	_ *signing.Request,
	_ waitForBlockFn,
) roastTransitionController {
	return nil
}
