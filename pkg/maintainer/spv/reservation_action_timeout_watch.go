package spv

import (
	"context"
	"fmt"
	"math/big"
	"sort"
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

// reservationActionTimeoutMaxChecksPerTick bounds the number of tracked
// pending actions whose GetReservationAction chain read pollPendingActions
// issues on a single poll tick. Actions beyond this cap on a given tick are
// picked up on a later tick via actionCheckCursor, so a single tick's RPC
// volume stays bounded regardless of how many actions are tracked - unlike
// the previous unbounded one-read-per-tracked-action pass. No reusable
// multicall/batch-read helper exists elsewhere in this codebase for this
// chain-read pattern (checked pkg/chain), and adding one to the shared
// Chain interface is out of scope here, so this is the self-contained
// partial fix; a true batched on-chain read remains a follow-up.
const reservationActionTimeoutMaxChecksPerTick = 50

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
// Acceptance-type actions, and NotifyReservationActionTimeout for
// Reanchor-type actions. checkReservationActionTimeout branches on
// action.ActionType to call the correct entry point; an unexpected action
// type (Redemption and Dissolution are out of m1 scope) is logged and
// skipped rather than causing an ill-formed call.
//
// Reanchor timeouts are the permissionless path for a wallet the
// operator no longer locally tracks as open (e.g. Closed/Terminated and
// archived out of the wallet registry cache): NotifyReservationActionTimeout
// requires no cooperation from the dead wallet.
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

	// actionCheckCursor is the lexically-sorted pendingActions key
	// nextActionCheckBatch resumes from on the next poll tick, so a large
	// tracked-action set is checked in bounded
	// reservationActionTimeoutMaxChecksPerTick-sized slices across
	// successive ticks instead of all at once. Empty means resume from the
	// beginning (also true once a rotation has covered every key).
	actionCheckCursor string

	// strandingWatcher, when non-nil, lets checkReservationActionTimeout
	// immediately re-examine a Reanchor-type action's reservation for
	// stranding right after a successful NotifyReservationActionTimeout
	// call. ReservationRouter.sol's notifyReservationActionTimeout restores
	// the reservation to Active under its current (possibly now-dead)
	// source wallet, and the only trigger that would otherwise prompt a
	// re-examination of that wallet - the stranding watcher's one-shot
	// OnWalletClosed subscription - has already fired and been consumed by
	// the time the reservation reappears as Active, so nothing else will
	// ever notice it is stranded. Left nil (the default until
	// SetStrandingWatcher is called), recheckStrandingAfterActionTimeout is
	// a no-op, exactly matching pre-fix behavior.
	strandingWatcher *reservationStrandingWatcher
}

// SetStrandingWatcher wires an optional reservationStrandingWatcher into the
// action-timeout watcher (see the strandingWatcher field doc for why). It is
// a post-construction setter rather than a NewReservationActionTimeoutWatcher
// parameter so existing call sites keep compiling unchanged; production
// wiring (WireReservationWatchers in reservation_wiring.go) must call this
// once after constructing both watchers to pick up the fix.
func (ratw *ReservationActionTimeoutWatcher) SetStrandingWatcher(
	strandingWatcher *reservationStrandingWatcher,
) {
	ratw.strandingWatcher = strandingWatcher
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

	// 3. Re-check a bounded batch of tracked actions and remove entries
	// that are no longer pending. Each checked action costs one serial
	// GetReservationAction RPC; nextActionCheckBatch caps how many are
	// issued this tick (reservationActionTimeoutMaxChecksPerTick) and
	// rotates through the full tracked set across successive ticks, so
	// per-tick RPC count no longer scales unboundedly with the number of
	// tracked actions. See reservationActionTimeoutMaxChecksPerTick's doc
	// for why a full multicall-style batch read is not used instead.
	for _, key := range ratw.nextActionCheckBatch() {
		item, ok := ratw.pendingActions[key]
		if !ok {
			// Evicted since the batch was built (e.g. by an earlier
			// eviction this same tick, or theoretically by a concurrent
			// caller); nothing left to check.
			continue
		}
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
			// to land before resubmitting a timeout notification on
			// every poll tick. If the prior attempt's transaction was
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

// nextActionCheckBatch returns up to reservationActionTimeoutMaxChecksPerTick
// pendingActions keys to check this tick, resuming from
// ratw.actionCheckCursor and rotating through a lexically sorted view of the
// tracked key set so every action eventually gets checked across successive
// ticks even when the tracked count exceeds the per-tick cap. Sorting gives
// the rotation a deterministic, reproducible order; map iteration order
// would otherwise make which actions get deferred to a later tick
// unpredictable from one run to the next. Persists the key immediately
// after the last one returned as the next cursor, or "" once the returned
// batch reaches the end of the sorted set (so the next tick starts over from
// the beginning) or covers every tracked key.
func (ratw *ReservationActionTimeoutWatcher) nextActionCheckBatch() []string {
	if len(ratw.pendingActions) == 0 {
		return nil
	}

	keys := make([]string, 0, len(ratw.pendingActions))
	for key := range ratw.pendingActions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	start := 0
	if ratw.actionCheckCursor != "" {
		start = sort.SearchStrings(keys, ratw.actionCheckCursor)
	}

	batchSize := min(len(keys), reservationActionTimeoutMaxChecksPerTick)
	batch := make([]string, batchSize)
	for i := range batch {
		batch[i] = keys[(start+i)%len(keys)]
	}

	ratw.actionCheckCursor = ""
	if batchSize < len(keys) {
		ratw.actionCheckCursor = keys[(start+batchSize)%len(keys)]
	}

	return batch
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
		// The action-timeout watcher is the permissionless path that must
		// not depend on cooperation from the (possibly dead) custodying
		// wallet.
		if err := ratw.spvChain.NotifyReservationActionTimeout(
			reservationKey,
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

		ratw.recheckStrandingAfterActionTimeout(reservationKey)
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

// recheckStrandingAfterActionTimeout re-examines reservationKey for
// stranding immediately after a successful Reanchor-type
// NotifyReservationActionTimeout call. ReservationRouter.sol's
// notifyReservationActionTimeout restores the reservation to Active under
// its current wallet as part of that call; if that wallet is already
// Closed/Terminated, the anchor is stranded on Bitcoin but reads Active
// on-chain indefinitely, because the stranding watcher's only trigger - the
// wallet's one-shot OnWalletClosed event - already fired (and was consumed)
// before this timeout notification landed. A nil strandingWatcher (the
// default until SetStrandingWatcher is wired in) makes this a no-op; see
// the strandingWatcher field doc.
func (ratw *ReservationActionTimeoutWatcher) recheckStrandingAfterActionTimeout(
	reservationKey *big.Int,
) {
	if ratw.strandingWatcher == nil {
		return
	}

	reservation, err := ratw.spvChain.GetReservation(reservationKey)
	if err != nil {
		logger.Errorf(
			"failed to re-resolve reservation [%v] for immediate "+
				"stranding re-check after action timeout notification: [%v]",
			reservationKey,
			err,
		)
		return
	}

	walletPublicKeyHash := reservation.WalletPublicKeyHash
	if walletPublicKeyHash == ([20]byte{}) {
		return
	}

	wallet, err := ratw.spvChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		logger.Errorf(
			"failed to fetch wallet [0x%x] state for immediate stranding "+
				"re-check of reservation [%v] after action timeout "+
				"notification: [%v]",
			walletPublicKeyHash,
			reservationKey,
			err,
		)
		return
	}

	if wallet.State != tbtc.StateClosed && wallet.State != tbtc.StateTerminated {
		return
	}

	if err := ratw.strandingWatcher.checkReservationStrandingForWallet(
		walletPublicKeyHash,
	); err != nil {
		logger.Errorf(
			"immediate stranding re-check after action timeout "+
				"notification failed for wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}
}
