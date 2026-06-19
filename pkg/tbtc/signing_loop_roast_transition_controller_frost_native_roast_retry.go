//go:build frost_native && frost_roast_retry

package tbtc

import (
	"context"

	"github.com/ipfs/go-log/v2"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// roastTransitionExchange is the produce-side surface the controller drives on a
// failed attempt. The production implementation is *signing.RoastTransitionExchange;
// the controller tests inject a fake.
type roastTransitionExchange interface {
	BroadcastForcedSnapshot(attemptHash [32]byte)
	AggregateAndBroadcast(attemptHash [32]byte)
	HasLostSync() bool
}

// roastTransitionControllerImpl is the active (frost_native && frost_roast_retry)
// transition controller. Constructed once per local signer, it observes each
// attempt (C1) and, on a failed attempt, drives the transition exchange (C2): it
// publishes this seat's forced proof-of-attendance snapshot and, after the
// collection window, has the elected coordinator aggregate + broadcast the
// bundle.
//
// BeginObservedAttempt and OnAttemptFailed are called only from the signer's
// single retry-loop goroutine, sequentially within an attempt, so currentAttemptHash
// needs no lock; the deadline goroutine captures the hash by value.
type roastTransitionControllerImpl struct {
	ctx             context.Context
	logger          log.StandardLogger
	requestTemplate *signing.Request
	waitForBlockFn  waitForBlockFn
	// exchange is nil when ROAST retry is inactive (readiness opted out, no
	// coordinator, or no usable channel); the controller then observes only.
	exchange roastTransitionExchange

	currentAttemptHash [32]byte
}

// newRoastTransitionController builds the active controller. requestTemplate
// carries the static (non-per-attempt) request material; a nil template yields a
// nil controller (the loop then skips transition steps).
func newRoastTransitionController(
	ctx context.Context,
	logger log.StandardLogger,
	requestTemplate *signing.Request,
	waitForBlockFn waitForBlockFn,
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
		waitForBlockFn:  waitForBlockFn,
		exchange:        newRoastTransitionExchangeForRequest(ctx, logger, requestTemplate),
	}
}

// newRoastTransitionExchangeForRequest builds the session-scoped transition
// exchange when ROAST retry is active: readiness opt-in on, a coordinator
// registered, and a usable wallet channel. It returns nil otherwise, so the
// controller observes only (C1) and drives no exchange -- the same set of
// deterministic static conditions ObserveAttemptForTransition gates on.
func newRoastTransitionExchangeForRequest(
	ctx context.Context,
	logger log.StandardLogger,
	template *signing.Request,
) roastTransitionExchange {
	// RFC-21 Phase 7.3 PR2b-1.5: gate + fetch deps for THIS seat, so a multi-seat
	// operator's exchange uses the coordinator bound to template.MemberIndex (the
	// elected-but-not-process-default seat can then collect + aggregate).
	if !signing.RoastRetryActiveForMember(template.MemberIndex) {
		return nil
	}
	deps, ok := signing.RegisteredRoastRetryCoordinatorForMember(template.MemberIndex)
	if !ok || deps.Coordinator == nil {
		return nil
	}
	if template.Channel == nil || template.MembershipValidator == nil {
		return nil
	}
	bus, err := signing.NewBroadcastChannelRunnerBus(
		ctx, logger, template.Channel, template.MembershipValidator,
	)
	if err != nil {
		logger.Warnf("roast transition: build transport bus: [%v]", err)
		return nil
	}
	return signing.NewRoastTransitionExchange(
		ctx, logger, bus, deps, template.RoastSessionID, template.MemberIndex,
	)
}

func (c *roastTransitionControllerImpl) BeginObservedAttempt(
	roastAttemptNumber uint,
	includedMembersIndexes []group.MemberIndex,
	excludedMembersIndexes []group.MemberIndex,
	transientlyParkedMembersIndexes []group.MemberIndex,
) {
	// Shallow-copy the static template and stamp this attempt's metadata.
	// Attempt.Number is 1-based; BuildAttemptContextFromRequest maps it to the
	// 0-based AttemptContext.AttemptNumber == roastAttemptNumber, so the observe
	// context and the transition-record freshness chain key off the committed
	// ROAST attempt index, not the block-paced loop counter.
	request := *c.requestTemplate
	request.Attempt = &signing.Attempt{
		Number:                          roastAttemptNumber + 1,
		IncludedMembersIndexes:          includedMembersIndexes,
		ExcludedMembersIndexes:          excludedMembersIndexes,
		TransientlyParkedMembersIndexes: transientlyParkedMembersIndexes,
	}

	hash, err := signing.ObserveAttemptForTransition(&request)
	if err != nil {
		c.logger.Warnf(
			"[member:%v] roast transition: observe roast attempt [%v] failed: [%v]",
			request.MemberIndex, roastAttemptNumber, err,
		)
	}
	// Retain the attempt hash so OnAttemptFailed can drive the exchange against
	// this attempt's observe binding. Zero on a static-fallback observe, which
	// makes OnAttemptFailed a no-op.
	c.currentAttemptHash = hash
}

func (c *roastTransitionControllerImpl) OnAttemptFailed(
	attemptNumber uint,
	timeoutBlock uint64,
) {
	if c.exchange == nil {
		return
	}
	hash := c.currentAttemptHash
	if hash == ([32]byte{}) {
		// No observe binding for this attempt (static fallback) -- nothing to do.
		return
	}

	// The caller reaches the failed-attempt path only when it participated (a
	// skipped seat never runs the attempt), so publish its forced
	// proof-of-attendance snapshot.
	c.exchange.BroadcastForcedSnapshot(hash)

	// The elected coordinator aggregates + broadcasts the bundle once the snapshot
	// collection window closes. Run it OFF the retry-loop goroutine so the loop
	// proceeds on its own block schedule; AggregateAndBroadcast is a no-op on a
	// seat that is not the elected coordinator. snapshotDeadline aligns with the
	// cooldown between attempts, so a produced bundle reaches peers before the next
	// attempt's selection.
	snapshotDeadline := timeoutBlock + signingAttemptCoolDownBlocks
	ctx := c.ctx
	waitForBlock := c.waitForBlockFn
	exchange := c.exchange
	go func() {
		if waitForBlock != nil {
			if err := waitForBlock(ctx, snapshotDeadline); err != nil {
				return
			}
		}
		exchange.AggregateAndBroadcast(hash)
	}()
}

func (c *roastTransitionControllerImpl) OnAttemptSucceeded() {
	hash := c.currentAttemptHash
	if hash == ([32]byte{}) {
		// No observe binding for this attempt (static fallback) -- nothing to clear.
		return
	}
	// Clear the observe binding for the succeeded attempt so the observe handle no
	// longer collects: the elected coordinator cannot aggregate a failure bundle,
	// and a peer's failure bundle is not stored, for an attempt this seat won. The
	// observed-history marker remains, so a late bundle for it is a benign
	// retransmit rather than lost sync.
	signing.ClearObservedAttemptOnLocalSuccess(
		c.requestTemplate.RoastSessionID,
		c.requestTemplate.MemberIndex,
		hash,
	)
	// Also drop any coarse-path evidence stashed for this attempt: a succeeded
	// attempt never reaches OnAttemptFailed -> BroadcastForcedSnapshot (which is
	// what consumes the stash), so without this the entry would leak until the TTL
	// sweep (RFC-21 Phase 7.3 PR2b-2 step 2, the blame bridge).
	signing.ClearPendingEvidenceOnLocalSuccess(
		c.requestTemplate.RoastSessionID,
		c.requestTemplate.MemberIndex,
		hash,
	)
}

func (c *roastTransitionControllerImpl) HasLostSync() bool {
	// nil when ROAST retry is inactive (the controller only observes): never lost
	// sync, since no listener runs to receive a peer's transition.
	return c.exchange != nil && c.exchange.HasLostSync()
}
