package spv

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationActionTimeoutLookBackBlocks bounds the pending-action-request event
// scan performed on the very first pass, before an incremental cursor
// exists. Mirrors reservationProofLookBackBlocks in reservation_proof_loop.go:
// 30 days at 12s/block.
const reservationActionTimeoutLookBackBlocks = uint64(216000)

// actionTimeoutRenotifyInterval bounds how often a still-Pending action
// generation is re-offered to NotifyReservationActionTimeout once one
// attempt has already been made. Mirrors DefaultIdleBackOffTime's 10
// minute convention in config.go; long enough to avoid resubmitting on
// every one-minute poll tick while a normal transaction confirms, short
// enough that a dropped or reverted notification is retried well within
// an action's timeout-to-slashing window rather than silently stalling
// for the process lifetime.
const actionTimeoutRenotifyInterval = 10 * time.Minute

// ReservationActionTimeoutWatcher observes the reservation action set and
// notifies the Bridge when a pending action's on-chain deadline has elapsed
// without an SPV proof being submitted.
//
// The Bridge uses a per-action timeout window (snapshotted in the
// ReservationAction record at nonce creation time) to bound the lag between
// action creation and SPV proof submission. When the deadline passes without
// a proof, the SPV maintainer is no longer eligible to settle the action
// and the Bridge must be told to update the action to ReservationActionStateTimedOut.
// This triggers Bridge-side sweeping (e.g. fee slashing in m2+ and the
// fallback to owner late-settlement) and ensures the state machine can move
// forward.
//
// The Bridge exposes two distinct timeout entry points depending on the
// action generation's type: NotifyReservationAcceptanceTimedOut for
// Acceptance-type actions, and NotifyReservationActionTimeout (which
// takes a wallet member IDs slice the router ignores for Reanchor
// timeouts) for Reanchor-type actions. checkReservationActionTimeout
// branches on action.ActionType to call the correct entry point; an
// unexpected action type (Redemption and Dissolution are out of m1
// scope) is logged and skipped rather than causing an ill-formed call.
//
// Reanchor timeouts are the permissionless path for a wallet the
// operator no longer locally tracks as open (e.g. Closed/Terminated and
// archived out of the wallet registry cache): NotifyReservationActionTimeout
// is called with an empty member IDs slice unconditionally, requiring no
// cooperation from the dead wallet.
type ReservationActionTimeoutWatcher struct {
	spvChain Chain
	// nowFn returns the current UNIX timestamp the watcher treats as "now"
	// for `now > timeoutAt` comparisons. Tests override it to drive the
	// deadline forward; production wires it to time.Now in UTC.
	nowFn func() uint32
	// interval is how often the background poll loop re-checks pending
	// actions. Production always drives the watcher through Run, started
	// as a background goroutine by WireReservationWatchers with the fixed
	// one-minute DefaultReservationActionTimeoutPollInterval; there is no
	// other production integration path. interval must be positive
	// whenever Run is used; tests that call CheckReservationActionTimeouts
	// directly, without starting Run, may leave it zero.
	interval time.Duration

	acceptanceLastScannedBlock uint64
	reanchorLastScannedBlock   uint64

	// pendingActions tracks still-pending reservation actions discovered from
	// acceptance and re-anchor request events across successive poll passes.
	pendingActions map[string]*pendingAction
}

type pendingAction struct {
	reservationKey *big.Int
	requestNonce   uint64
	// notifiedAt is the UNIX timestamp of the last successful
	// NotifyReservationAcceptanceTimedOut or NotifyReservationActionTimeout
	// call for this action generation, or 0 if neither has ever
	// succeeded. It is NOT treated as proof the notification landed: a
	// submitted-but-dropped or reverted transaction still leaves the
	// on-chain action Pending, so pollPendingActions re-attempts the
	// notification once actionTimeoutRenotifyInterval has elapsed since
	// notifiedAt rather than treating one local send as permanent
	// evidence of success. Eviction still happens only once
	// action.State actually leaves Pending, which is the real on-chain
	// evidence the notification took effect.
	notifiedAt uint32
}

// actionEventKey identifies one reservation action generation.
func actionEventKey(reservationKey *big.Int, requestNonce uint64) string {
	return fmt.Sprintf("%s#%d", reservationKey.String(), requestNonce)
}

// NewReservationActionTimeoutWatcher constructs a watcher bound to the
// given chain and poll interval.
//
// pollInterval must be positive whenever Run is used to drive the
// background loop. Production always uses Run, via
// WireReservationWatchers; pollInterval only needs to be non-zero for
// that path, not for tests that drive the watcher directly through
// CheckReservationActionTimeouts.
func NewReservationActionTimeoutWatcher(
	spvChain Chain,
	pollInterval time.Duration,
) *ReservationActionTimeoutWatcher {
	return &ReservationActionTimeoutWatcher{
		spvChain:       spvChain,
		nowFn:          defaultActionTimeoutNowFn,
		interval:       pollInterval,
		pendingActions: make(map[string]*pendingAction),
	}
}

// defaultActionTimeoutNowFn returns time.Now() as a uint32 UNIX timestamp.
// Kept separate from the struct to allow tests to swap it deterministically.
func defaultActionTimeoutNowFn() uint32 {
	return uint32(time.Now().Unix())
}

// nextScanRange calculates the start and current block numbers for the next
// event scan. On the first scan (lastScannedBlock == 0), the scan window is
// bounded by reservationActionTimeoutLookBackBlocks. On subsequent scans, it
// resumes from lastScannedBlock + 1.
func (ratw *ReservationActionTimeoutWatcher) nextScanRange(
	lastScannedBlock uint64,
) (startBlock uint64, currentBlock uint64, err error) {
	blockCounter, err := ratw.spvChain.BlockCounter()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get block counter: [%v]", err)
	}

	currentBlock, err = blockCounter.CurrentBlock()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get current block: [%v]", err)
	}

	if lastScannedBlock == 0 {
		if currentBlock > reservationActionTimeoutLookBackBlocks {
			startBlock = currentBlock - reservationActionTimeoutLookBackBlocks
		} else {
			startBlock = 0
		}
	} else {
		startBlock = lastScannedBlock + 1
	}

	return startBlock, currentBlock, nil
}

// Run starts the background poll loop. It returns when ctx is done or when
// a fatal configuration error is detected.
//
// Each iteration discovers new reservation acceptance and re-anchor action
// request events incrementally, updates the tracked pending actions set,
// removes actions that are no longer pending, and calls
// CheckReservationActionTimeouts on any overdue pending action.
func (ratw *ReservationActionTimeoutWatcher) Run(ctx context.Context) error {
	if ratw.interval <= 0 {
		return fmt.Errorf(
			"action-timeout watcher requires a positive poll interval",
		)
	}

	ticker := time.NewTicker(ratw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		if err := ratw.pollPendingActions(); err != nil {
			logger.Errorf("action-timeout watcher poll failed: [%v]", err)
		}
	}
}

// pollPendingActions scans for newly requested reservation actions, updates the
// pendingActions map, evicts actions that are no longer pending, and checks
// overdue actions for timeout.
func (ratw *ReservationActionTimeoutWatcher) pollPendingActions() error {
	// 1. Scan new ReservationAcceptanceRequestedEvents
	acceptanceStartBlock, acceptanceCurrentBlock, err := ratw.nextScanRange(
		ratw.acceptanceLastScannedBlock,
	)
	if err != nil {
		return fmt.Errorf("failed to get acceptance scan range: [%v]", err)
	}

	acceptanceEvents, err := ratw.spvChain.PastReservationAcceptanceRequestedEvents(
		&tbtc.ReservationAcceptanceRequestedEventFilter{
			StartBlock: acceptanceStartBlock,
			EndBlock:   &acceptanceCurrentBlock,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get past reservation acceptance requested events: [%v]",
			err,
		)
	}

	for _, event := range acceptanceEvents {
		key := actionEventKey(event.ReservationKey, event.RequestNonce)
		ratw.pendingActions[key] = &pendingAction{
			reservationKey: event.ReservationKey,
			requestNonce:   event.RequestNonce,
		}
	}
	ratw.acceptanceLastScannedBlock = acceptanceCurrentBlock

	// 2. Scan new ReservationReanchorRequestedEvents
	reanchorStartBlock, reanchorCurrentBlock, err := ratw.nextScanRange(
		ratw.reanchorLastScannedBlock,
	)
	if err != nil {
		return fmt.Errorf("failed to get reanchor scan range: [%v]", err)
	}

	reanchorEvents, err := ratw.spvChain.PastReservationReanchorRequestedEvents(
		&tbtc.ReservationReanchorRequestedEventFilter{
			StartBlock: reanchorStartBlock,
			EndBlock:   &reanchorCurrentBlock,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get past reservation reanchor requested events: [%v]",
			err,
		)
	}

	for _, event := range reanchorEvents {
		key := actionEventKey(event.ReservationKey, event.RequestNonce)
		ratw.pendingActions[key] = &pendingAction{
			reservationKey: event.ReservationKey,
			requestNonce:   event.RequestNonce,
		}
	}
	ratw.reanchorLastScannedBlock = reanchorCurrentBlock

	now := ratw.nowFn()

	// 3. Re-check each tracked action and remove entries that are no longer
	// pending. Each tracked action costs one serial GetReservationAction RPC
	// per poll tick; no multicall-style batching helper exists elsewhere in
	// this codebase for this chain-read pattern (checked pkg/chain), so
	// per-tick RPC count scales linearly with the number of tracked actions.
	for key, item := range ratw.pendingActions {
		action, err := ratw.spvChain.GetReservationAction(
			item.reservationKey,
			item.requestNonce,
		)
		if err != nil {
			logger.Errorf(
				"failed to load reservation action [%v]/%d: [%v]",
				item.reservationKey,
				item.requestNonce,
				err,
			)
			continue
		}

		if action.State != tbtc.ReservationActionStatePending {
			delete(ratw.pendingActions, key)
			continue
		}

		if item.notifiedAt != 0 &&
			now-item.notifiedAt < uint32(actionTimeoutRenotifyInterval.Seconds()) {
			// A timeout notification was attempted recently for this
			// action generation while it remains Pending; give it time
			// to land before resubmitting NotifyReservationActionTimeout
			// on every poll tick. If the prior attempt's transaction was
			// dropped or reverted, the action is still Pending once
			// actionTimeoutRenotifyInterval elapses and this branch is
			// skipped, so the next tick retries below.
			continue
		}

		if now > action.TimeoutAt {
			notified, err := ratw.checkReservationActionTimeout(
				item.reservationKey,
				now,
				action,
				item.requestNonce,
			)
			if err != nil {
				logger.Errorf(
					"action-timeout watcher failed to check reservation [%v]: [%v]",
					item.reservationKey,
					err,
				)
			}
			if notified {
				item.notifiedAt = now
			}
		}
	}

	return nil
}

// CheckReservationActionTimeouts inspects the current action generation of a
// single reservation and notifies the Bridge if it is Pending and its
// TimeoutAt has elapsed. The caller controls the iteration; the watcher
// does not background-loop on its own (see Run for the poll-driven caller).
//
// Parameters:
//
//   - reservationKey: the reservation identifier used by the Bridge's
//     ReservationRouter.
//   - now: a UNIX timestamp used to compare against TimeoutAt. Tests pass
//     an explicit value; production passes time.Now().Unix() cast to uint32.
//
// The function resolves the custodying wallet, then inspects only the
// action generation at reservation.RequestNonce. By the Bridge
// invariant, only the most-recent action generation can be Pending -
// older nonces have already settled, timed out, or been superseded -
// so a single lookup suffices; no walk from nonce 0 is needed. A
// RequestNonce of 0 means no action generation has ever been requested
// against the reservation, so there is nothing to check.
func (ratw *ReservationActionTimeoutWatcher) CheckReservationActionTimeouts(
	reservationKey *big.Int,
	now uint32,
) error {
	_, err := ratw.checkReservationActionTimeout(reservationKey, now, nil, 0)
	return err
}

// checkReservationActionTimeout is the shared implementation behind
// CheckReservationActionTimeouts. When preloadedAction is non-nil and
// preloadedActionNonce matches the reservation's freshly-read
// RequestNonce, it is used in place of a second GetReservationAction
// RPC: pollPendingActions already loads the action for
// (reservationKey, requestNonce) once per poll pass to decide whether
// the entry is overdue, and by the Bridge invariant documented above
// that loaded action is the same one reservation.RequestNonce resolves
// to whenever its state is still Pending, so re-fetching it here would
// be a redundant RPC for the exact same value. A nonce mismatch means
// the preload was fetched against a since-superseded action generation,
// so the fresh nonce is used to re-fetch instead of trusting the stale
// preload.
//
// The returned bool is true only immediately after a Bridge timeout
// notification call (NotifyReservationAcceptanceTimedOut or
// NotifyReservationActionTimeout) itself succeeds; it is false on every
// skip path and on any error, letting callers distinguish "nothing
// needed to happen" from "a notification was actually sent".
func (ratw *ReservationActionTimeoutWatcher) checkReservationActionTimeout(
	reservationKey *big.Int,
	now uint32,
	preloadedAction *tbtc.ReservationAction,
	preloadedActionNonce uint64,
) (bool, error) {
	if reservationKey == nil {
		return false, fmt.Errorf("reservation key must not be nil")
	}

	reservation, err := ratw.spvChain.GetReservation(reservationKey)
	if err != nil {
		return false, fmt.Errorf(
			"failed to load reservation [%v]: [%v]",
			reservationKey,
			err,
		)
	}

	if reservation.RequestNonce == 0 {
		// No action generation has ever been requested; nothing pending.
		return false, nil
	}

	walletPublicKeyHash := reservation.WalletPublicKeyHash
	if walletPublicKeyHash == ([20]byte{}) {
		logger.Debugf("reservation [%v] has no wallet; skipping", reservationKey)
		return false, nil
	}

	nonce := reservation.RequestNonce

	action := preloadedAction
	if action == nil || nonce != preloadedActionNonce {
		action, err = ratw.spvChain.GetReservationAction(reservationKey, nonce)
		if err != nil {
			return false, fmt.Errorf(
				"failed to load action for reservation [%v] at nonce %d: [%v]",
				reservationKey,
				nonce,
				err,
			)
		}
	}

	if action.State != tbtc.ReservationActionStatePending {
		logger.Debugf(
			"reservation [%v] action nonce %d state=%s; not pending, "+
				"nothing to time out",
			reservationKey,
			nonce,
			action.State,
		)
		return false, nil
	}

	if now <= action.TimeoutAt {
		logger.Debugf(
			"reservation [%v] action nonce %d timeout at [%d] "+
				"not yet reached (now=%d); skipping",
			reservationKey,
			nonce,
			action.TimeoutAt,
			now,
		)
		return false, nil
	}

	switch action.ActionType {
	case tbtc.ReservationActionTypeAcceptance:
		if err := ratw.spvChain.NotifyReservationAcceptanceTimedOut(
			reservationKey,
		); err != nil {
			return false, fmt.Errorf(
				"failed to notify acceptance timeout for "+
					"reservation [%v] nonce %d: [%v]",
				reservationKey,
				nonce,
				err,
			)
		}

		logger.Infof(
			"notified acceptance timeout for reservation [%v] nonce %d "+
				"(timeout=%d)",
			reservationKey,
			nonce,
			action.TimeoutAt,
		)

		return true, nil
	case tbtc.ReservationActionTypeReanchor:
		// The router ignores the member-IDs parameter for Reanchor-type
		// timeouts, and the action-timeout watcher is the permissionless
		// path that must not depend on cooperation from the (possibly
		// dead) custodying wallet, so an empty slice is passed
		// unconditionally rather than resolving wallet members.
		if err := ratw.spvChain.NotifyReservationActionTimeout(
			reservationKey,
			[]uint32{},
		); err != nil {
			return false, fmt.Errorf(
				"failed to notify action timeout for "+
					"reservation [%v] nonce %d: [%v]",
				reservationKey,
				nonce,
				err,
			)
		}

		logger.Infof(
			"notified action timeout for reservation [%v] nonce %d "+
				"(timeout=%d)",
			reservationKey,
			nonce,
			action.TimeoutAt,
		)

		return true, nil
	default:
		// Redemption and Dissolution are m2+ scope and should never
		// reach a Pending, timed-out state here; handle defensively
		// rather than assuming only Acceptance and Reanchor exist.
		logger.Warnf(
			"reservation [%v] action nonce %d has unexpected action "+
				"type %s; skipping timeout notification",
			reservationKey,
			nonce,
			action.ActionType,
		)

		return false, nil
	}
}
