package spv

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

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
	// defer a Reanchor-type action's reservation to a stranding recheck
	// after a successful NotifyReservationActionTimeout call (see
	// strandingRecheckWallets below and drainStrandingRechecks).
	// ReservationRouter.sol's notifyReservationActionTimeout restores
	// the reservation to Active under its current (possibly now-dead)
	// source wallet, and the only trigger that would otherwise prompt a
	// re-examination of that wallet - the stranding watcher's one-shot
	// OnWalletClosed subscription - has already fired and been consumed by
	// the time the reservation reappears as Active, so nothing else will
	// ever notice it is stranded. It is a required
	// NewReservationActionTimeoutWatcher parameter; nil is a legitimate,
	// explicit "no stranding watcher" configuration (checkReservationActionTimeout
	// never populates strandingRecheckWallets when this is nil), but the
	// parameter can never be silently forgotten the way a
	// post-construction setter could be.
	strandingWatcher *reservationStrandingWatcher

	// strandingRecheckWallets accumulates the wallet public key hashes
	// whose reservation(s) had a Reanchor-type action-timeout
	// notification submitted since the last drain, deferred here rather
	// than rechecked synchronously at notify time: NotifyReservationActionTimeout's
	// generated chain binding returns as soon as the transaction is
	// submitted, not once it mines (mining is handled by a background
	// ForceMining goroutine elsewhere), so a synchronous GetWallet
	// re-check at that point would read pre-mining state and the
	// State != Active guard in checkReservationStrandingForWallet would
	// always skip it. drainStrandingRechecks, called at the top of
	// every pollPendingActions tick, gives a wallet notified last tick
	// roughly one full poll interval to mine before its state is
	// re-checked. Multiple Reanchor-timeout notifications against the
	// same wallet within one tick naturally dedupe here (map key),
	// so a single stranding check runs once per wallet per drain
	// regardless of how many of that wallet's reservations timed out
	// in the same tick.
	strandingRecheckWallets map[[20]byte]struct{}

	// operatorAddress identifies this process for
	// reservationOperatorStaggerOffset (see reservation_wiring.go), used
	// to stagger the FIRST notify attempt for a given action generation
	// so multiple operators' simultaneous first attempts do not collide
	// on-chain (see pollPendingActions). It is a required
	// NewReservationActionTimeoutWatcher parameter, not a
	// post-construction setter, so a watcher can never be constructed in
	// a partially-initialized state that would silently skip staggering.
	operatorAddress common.Address
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

// actionEventKey identifies one reservation action generation. It
// delegates to reservationEventKey (see reservation_proof_loop.go), the
// canonical (reservationKey, requestNonce) key formatter shared by
// every pending-event map in this package; this watcher previously used
// its own "%s#%d" format, now consolidated onto reservationEventKey's
// pre-existing "%s:%d" format.
func actionEventKey(reservationKey *big.Int, requestNonce uint64) string {
	return reservationEventKey(reservationKey, requestNonce)
}

// NewReservationActionTimeoutWatcher constructs a watcher bound to the
// given chain and poll interval.
//
// pollInterval must be positive whenever Run is used to drive the
// background loop. Production always uses Run, via
// WireReservationWatchers; pollInterval only needs to be non-zero for
// that path, not for tests that drive the watcher directly through
// CheckReservationActionTimeouts.
//
// operatorAddress is required (see the operatorAddress field doc).
// strandingWatcher is required but may legitimately be nil (see the
// strandingWatcher field doc for what a nil value means); both are
// constructor parameters rather than post-construction setters so a
// watcher can never be constructed in a partially-initialized state
// that would silently skip staggering or the stranding recheck.
func NewReservationActionTimeoutWatcher(
	spvChain Chain,
	pollInterval time.Duration,
	operatorAddress common.Address,
	strandingWatcher *reservationStrandingWatcher,
) *ReservationActionTimeoutWatcher {
	return &ReservationActionTimeoutWatcher{
		spvChain:                spvChain,
		nowFn:                   defaultActionTimeoutNowFn,
		interval:                pollInterval,
		pendingActions:          make(map[string]*pendingAction),
		operatorAddress:         operatorAddress,
		strandingWatcher:        strandingWatcher,
		strandingRecheckWallets: make(map[[20]byte]struct{}),
	}
}

// defaultActionTimeoutNowFn returns time.Now() as a uint32 UNIX timestamp.
// Kept separate from the struct to allow tests to swap it deterministically.
func defaultActionTimeoutNowFn() uint32 {
	return uint32(time.Now().Unix())
}

// nextScanRange calculates the start and current block numbers for the
// next event scan. It delegates to the shared reservationProofNextScanRange
// helper (see reservation_proof_loop.go): this watcher's own incremental
// scan shape (bounded catch-up window on the first pass, resume-from-cursor
// thereafter) is identical to the proof loop's, so both now share one
// implementation instead of two independently-maintained copies.
func (ratw *ReservationActionTimeoutWatcher) nextScanRange(
	lastScannedBlock uint64,
) (startBlock uint64, currentBlock uint64, err error) {
	return reservationProofNextScanRange(ratw.spvChain, lastScannedBlock)
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
// overdue actions for timeout. Before any of that, it drains
// strandingRecheckWallets (see drainStrandingRechecks), so a wallet
// deferred there by a Reanchor-timeout notification in the PREVIOUS
// tick is rechecked before this tick's own batch processing runs.
func (ratw *ReservationActionTimeoutWatcher) pollPendingActions() error {
	ratw.drainStrandingRechecks()

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
			(now < item.notifiedAt ||
				now-item.notifiedAt < uint32(actionTimeoutRenotifyInterval.Seconds())) {
			// now < item.notifiedAt is treated the same as "interval not
			// yet elapsed" rather than falling through to retry: now is a
			// caller-supplied, not-guaranteed-monotonic tick token (see
			// nowFn's doc comment), so without this guard a clock
			// adjustment or an out-of-order now would underflow the
			// subtraction below to a huge uint32 and fail OPEN -
			// resubmitting immediately instead of waiting out the
			// backoff. Staying in the skip branch is the safe direction
			// for an ambiguous time comparison; a genuine elapsed
			// interval will still be observed on a later tick with a
			// larger now.
			//
			// A timeout notification was attempted recently for this
			// action generation while it remains Pending; give it time
			// to land before resubmitting a timeout notification on
			// every poll tick. If the prior attempt's transaction was
			// dropped or reverted, the action is still Pending once
			// actionTimeoutRenotifyInterval elapses and this branch is
			// skipped, so the next tick retries below.
			continue
		}

		deadline := action.TimeoutAt
		if item.notifiedAt == 0 {
			// FIRST notify attempt for this action generation: stagger
			// it by a deterministic per-operator offset (see
			// reservationOperatorStaggerOffset) so that every
			// reservation-enabled operator's first permissionless
			// notify attempt does not collide in the same poll tick.
			// Retries (notifiedAt != 0) are unaffected by this and
			// keep using the renotify-interval backoff checked above
			// unchanged.
			deadline += reservationOperatorStaggerOffset(
				ratw.operatorAddress,
				key,
				actionTimeoutRenotifyInterval,
			)
		}

		if now > deadline {
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

		// Defer the stranding recheck to the next pollPendingActions
		// tick's drainStrandingRechecks call instead of running it
		// synchronously here: NotifyReservationActionTimeout has only
		// just been submitted, not mined, so a synchronous GetWallet
		// read at this point would read pre-mining state (see the
		// strandingRecheckWallets field doc). The map key naturally
		// dedupes multiple Reanchor timeouts against the same wallet
		// within this tick down to a single recheck.
		if ratw.strandingWatcher != nil {
			ratw.strandingRecheckWallets[walletPublicKeyHash] = struct{}{}
		}
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

// drainStrandingRechecks runs the deferred stranding recheck (see the
// strandingRecheckWallets field doc for why this is deferred rather
// than run synchronously at notify time) for every wallet recorded
// since the previous drain, then clears the set. pollPendingActions
// calls this at the very top of every tick, before processing that
// tick's own batch, so a wallet notified last tick has had roughly one
// full poll interval for its NotifyReservationActionTimeout
// transaction to mine before this recheck reads GetWallet's state -
// and so this tick's own eviction of newly-non-Pending actions (later
// in pollPendingActions) always happens after the drain for wallets
// notified in a prior tick.
//
// The wallet hash recorded in strandingRecheckWallets is the one the
// reservation already had BEFORE its NotifyReservationActionTimeout
// call: ReservationRouter.sol's notifyReservationActionTimeout restores
// the reservation to Active under its current wallet (does not
// reassign a new one), so no re-resolution via GetReservation is
// needed here.
func (ratw *ReservationActionTimeoutWatcher) drainStrandingRechecks() {
	if len(ratw.strandingRecheckWallets) == 0 {
		return
	}

	wallets := ratw.strandingRecheckWallets
	ratw.strandingRecheckWallets = make(map[[20]byte]struct{})

	for walletPublicKeyHash := range wallets {
		wallet, err := ratw.spvChain.GetWallet(walletPublicKeyHash)
		if err != nil {
			// Transient RPC failure, not a definitive "wallet is Live"
			// outcome: re-queue the wallet on the new
			// strandingRecheckWallets set (already reset above for
			// this drain, and self-dedupes) so the next tick's drain
			// retries it instead of permanently losing the deferred
			// recheck.
			logger.Errorf(
				"failed to fetch wallet [0x%x] state for deferred "+
					"stranding re-check after action timeout "+
					"notification: [%v]; re-queuing for the next drain",
				walletPublicKeyHash,
				err,
			)
			ratw.strandingRecheckWallets[walletPublicKeyHash] = struct{}{}
			continue
		}

		if wallet.State != tbtc.StateClosed && wallet.State != tbtc.StateTerminated {
			continue
		}

		if err := ratw.strandingWatcher.checkReservationStrandingForWallet(
			walletPublicKeyHash,
		); err != nil {
			// Same transient-failure reasoning as the GetWallet error
			// above: re-queue rather than silently drop the recheck.
			logger.Errorf(
				"deferred stranding re-check after action timeout "+
					"notification failed for wallet [0x%x]: [%v]; "+
					"re-queuing for the next drain",
				walletPublicKeyHash,
				err,
			)
			ratw.strandingRecheckWallets[walletPublicKeyHash] = struct{}{}
		}
	}
}
