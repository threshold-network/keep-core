//go:build frost_native && frost_roast_retry

package tbtc

import (
	"context"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// roastTransitionControllerImpl is the active (frost_native && frost_roast_retry)
// transition controller. It is constructed once per local signer with the static
// signing-request material; each BeginObservedAttempt fills the per-attempt
// metadata and delegates to signing.ObserveAttemptForTransition.
//
// ctx is the signer's session-lifetime loop context; C2 uses it (with the
// channel/validator carried on requestTemplate) for the bundle exchange. C1 does
// not read it yet.
type roastTransitionControllerImpl struct {
	ctx             context.Context
	logger          log.StandardLogger
	requestTemplate *signing.Request
}

// newRoastTransitionController builds the active controller. requestTemplate
// carries the static (non-per-attempt) request material; a nil template yields a
// nil controller (the loop then skips transition steps).
func newRoastTransitionController(
	ctx context.Context,
	logger log.StandardLogger,
	requestTemplate *signing.Request,
) roastTransitionController {
	if requestTemplate == nil {
		return nil
	}
	if logger == nil {
		logger = log.Logger("keep-tbtc-roast-transition")
	}
	return &roastTransitionControllerImpl{
		ctx:             ctx,
		logger:          logger,
		requestTemplate: requestTemplate,
	}
}

func (c *roastTransitionControllerImpl) BeginObservedAttempt(
	attemptNumber uint,
	includedMembersIndexes []group.MemberIndex,
	excludedMembersIndexes []group.MemberIndex,
) {
	// Shallow-copy the static template and stamp this attempt's metadata. The
	// copy keeps each attempt's request independent without rebuilding the
	// material every call.
	request := *c.requestTemplate
	request.Attempt = &signing.Attempt{
		Number:                 attemptNumber,
		IncludedMembersIndexes: includedMembersIndexes,
		ExcludedMembersIndexes: excludedMembersIndexes,
	}

	if err := signing.ObserveAttemptForTransition(&request); err != nil {
		c.logger.Warnf(
			"[member:%v] roast transition: observe attempt [%v] failed: [%v]",
			request.MemberIndex,
			attemptNumber,
			err,
		)
	}
}
