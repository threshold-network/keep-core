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

// maxActionTimeoutLoadRetries is the maximum number of consecutive
// GetReservationAction poll-pass failures a tracked pendingAction entry
// may accumulate before it is evicted from pendingActions. Mirrors the
// maxReservationActionLoadRetries convention in reservation_proof_loop.go,
// kept as a separate local constant rather than a shared one since the two
// loops track independent pending-action sets.
const maxActionTimeoutLoadRetries = 3

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
// In m1 the operator-side penalty is a no-op; NotifyReservationActionTimeout
// is still called with the wallet member IDs as required by the Bridge
// function signature so that m2+ integrations only need to add the slashing
// logic without changing the call shape.
//
// The members resolver maps a wallet public key hash to the operator IDs
// composing that wallet's signing group. In production, node.ResolveWalletMembers
// resolves members for wallets the local operator co-signs and returns an error
// ("wallet not found") for other on-chain wallets. The watcher treats resolver
// errors for non-member wallets as an expected non-member condition and skips
// them silently at Debug log level, so the watcher only meaningfully monitors
// reservation action timeouts for wallets the local operator co-signs.
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
	// membersResolver turns a wallet public key hash into the operator IDs
	// the Bridge expects for the slashing argument. The resolver is
	// injected to keep the watcher independent of the chain interface used
	// to look up operator addresses (the SPV maintainer chain interface
	// does not expose GetOperatorID today).
	membersResolver tbtc.WalletMembersResolver

	acceptanceLastScannedBlock uint64
	reanchorLastScannedBlock   uint64

	// pendingActions tracks still-pending reservation actions discovered from
	// acceptance and re-anchor request events across successive poll passes.
	pendingActions map[string]*pendingAction
}

type pendingAction struct {
	reservationKey *big.Int
	requestNonce   uint64
	// loadFailures counts consecutive GetReservationAction failures for
	// this entry across successive poll passes. Reset to 0 on a
	// successful load; once it reaches maxActionTimeoutLoadRetries the
	// entry is evicted so a permanently unreadable action does not
	// accumulate forever in pendingActions.
	loadFailures int
	// notified is set once a timeout notification has been attempted (and
	// locally reported success) for this action generation while it is
	// still Pending, so pollPendingActions does not resubmit
	// NotifyReservationActionTimeout on every subsequent poll tick. The
	// entry is NOT deleted when this is set: eviction still happens only
	// once action.State actually leaves Pending, which is the real
	// on-chain evidence the notification took effect, so a dropped or
	// reverted notification stays visible in pendingActions instead of
	// being forgotten.
	notified bool
}

// actionEventKey identifies one reservation action generation.
func actionEventKey(reservationKey *big.Int, requestNonce uint64) string {
	return fmt.Sprintf("%s#%d", reservationKey.String(), requestNonce)
}

// NewReservationActionTimeoutWatcher constructs a watcher bound to the
// given chain, members resolver, and poll interval.
//
// The members resolver is mandatory: the watcher will refuse to operate
// without it because emitting NotifyReservationActionTimeout with a nil
// or empty member slice would be ill-formed on the Bridge side.
//
// pollInterval must be positive whenever Run is used to drive the
// background loop. Production always uses Run, via
// WireReservationWatchers; pollInterval only needs to be non-zero for
// that path, not for tests that drive the watcher directly through
// CheckReservationActionTimeouts.
func NewReservationActionTimeoutWatcher(
	spvChain Chain,
	membersResolver tbtc.WalletMembersResolver,
	pollInterval time.Duration,
) *ReservationActionTimeoutWatcher {
	return &ReservationActionTimeoutWatcher{
		spvChain:        spvChain,
		nowFn:           defaultActionTimeoutNowFn,
		interval:        pollInterval,
		membersResolver: membersResolver,
		pendingActions:  make(map[string]*pendingAction),
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
	if ratw.membersResolver == nil {
		return fmt.Errorf(
			"action-timeout watcher requires a non-nil members resolver",
		)
	}
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
			item.loadFailures++
			logger.Errorf(
				"failed to load reservation action [%v]/%d: [%v]",
				item.reservationKey,
				item.requestNonce,
				err,
			)
			if item.loadFailures >= maxActionTimeoutLoadRetries {
				logger.Errorf(
					"evicting reservation action [%v]/%d from tracking "+
						"after %d consecutive load failures",
					item.reservationKey,
					item.requestNonce,
					item.loadFailures,
				)
				delete(ratw.pendingActions, key)
			}
			continue
		}
		item.loadFailures = 0

		if action.State != tbtc.ReservationActionStatePending {
			delete(ratw.pendingActions, key)
			continue
		}

		if item.notified {
			// A timeout notification was already attempted for this
			// action generation while it remains Pending; skip
			// resubmitting NotifyReservationActionTimeout on every poll
			// tick. The entry is not deleted here - eviction happens only
			// above, once action.State actually leaves Pending, which is
			// the real on-chain evidence the notification took effect. A
			// dropped or reverted notification therefore stays visible in
			// pendingActions instead of being silently forgotten for the
			// rest of the process lifetime.
			continue
		}

		if now > action.TimeoutAt {
			if err := ratw.checkReservationActionTimeout(
				item.reservationKey,
				now,
				action,
			); err != nil {
				logger.Errorf(
					"action-timeout watcher failed to check reservation [%v]: [%v]",
					item.reservationKey,
					err,
				)
			} else {
				item.notified = true
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
// The function resolves the custodying wallet, looks up the operator member
// IDs through the injected resolver, then inspects only the action
// generation at reservation.RequestNonce. By the Bridge invariant, only the
// most-recent action generation can be Pending - older nonces have already
// settled, timed out, or been superseded - so a single lookup suffices; no
// walk from nonce 0 is needed. A RequestNonce of 0 means no action
// generation has ever been requested against the reservation, so there is
// nothing to check.
func (ratw *ReservationActionTimeoutWatcher) CheckReservationActionTimeouts(
	reservationKey *big.Int,
	now uint32,
) error {
	return ratw.checkReservationActionTimeout(reservationKey, now, nil)
}

// checkReservationActionTimeout is the shared implementation behind
// CheckReservationActionTimeouts. When preloadedAction is non-nil, it is
// used in place of a second GetReservationAction RPC: pollPendingActions
// already loads the action for (reservationKey, requestNonce) once per
// poll pass to decide whether the entry is overdue, and by the Bridge
// invariant documented above that loaded action is the same one
// reservation.RequestNonce resolves to whenever its state is still
// Pending, so re-fetching it here would be a redundant RPC for the exact
// same value.
func (ratw *ReservationActionTimeoutWatcher) checkReservationActionTimeout(
	reservationKey *big.Int,
	now uint32,
	preloadedAction *tbtc.ReservationAction,
) error {
	if ratw.membersResolver == nil {
		return fmt.Errorf(
			"action-timeout watcher requires a non-nil members resolver",
		)
	}
	if reservationKey == nil {
		return fmt.Errorf("reservation key must not be nil")
	}

	reservation, err := ratw.spvChain.GetReservation(reservationKey)
	if err != nil {
		return fmt.Errorf(
			"failed to load reservation [%v]: [%v]",
			reservationKey,
			err,
		)
	}

	if reservation.RequestNonce == 0 {
		// No action generation has ever been requested; nothing pending.
		return nil
	}

	walletPublicKeyHash := reservation.WalletPublicKeyHash
	if walletPublicKeyHash == ([20]byte{}) {
		logger.Debugf("reservation [%v] has no wallet; skipping", reservationKey)
		return nil
	}

	nonce := reservation.RequestNonce

	action := preloadedAction
	if action == nil {
		action, err = ratw.spvChain.GetReservationAction(reservationKey, nonce)
		if err != nil {
			return fmt.Errorf(
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
		return nil
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
		return nil
	}

	memberIDs, err := ratw.membersResolver.ResolveWalletMembers(walletPublicKeyHash)
	if err != nil {
		logger.Debugf(
			"operator is not a member of wallet [0x%x]; skipping action timeout check for reservation [%v]: [%v]",
			walletPublicKeyHash,
			reservationKey,
			err,
		)
		return nil
	}

	if len(memberIDs) == 0 {
		logger.Debugf(
			"wallet [0x%x] members resolver returned an empty set; "+
				"skipping action timeout check for reservation [%v]",
			walletPublicKeyHash,
			reservationKey,
		)
		return nil
	}

	if err := ratw.spvChain.NotifyReservationActionTimeout(
		reservationKey,
		memberIDs,
	); err != nil {
		return fmt.Errorf(
			"failed to notify action timeout for "+
				"reservation [%v] nonce %d: [%v]",
			reservationKey,
			nonce,
			err,
		)
	}

	logger.Infof(
		"notified action timeout for reservation [%v] nonce %d "+
			"(timeout=%d, members=%d)",
		reservationKey,
		nonce,
		action.TimeoutAt,
		len(memberIDs),
	)

	return nil
}
