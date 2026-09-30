package spv

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// StaleDepositResolution indicates the outcome of a stale deposit check to
// help callers (e.g. pollers) decide whether to keep or drop the deposit
// from tracking.
type StaleDepositResolution uint8

const (
	// StaleDepositResolutionUnknown is the zero value, returned alongside
	// a non-nil error whenever the check could not be completed (a chain
	// call failed, the deposit key was invalid, or the reveal-time refund
	// deadline could not be snapshotted - for instance because the
	// matching DepositRevealed event predates the catch-up window). The
	// deposit is never added to or evicted from the caller's tracking
	// set on this resolution - the caller must retain it and retry on
	// the next tick.
	StaleDepositResolutionUnknown StaleDepositResolution = iota
	// StaleDepositResolutionKeep indicates the deposit is still
	// pending-stale and must be retained in the caller's tracking set
	// for the next tick: its snapshotted refund deadline has not yet
	// passed - for a pollTick-discovered deposit the deadline is
	// memoized from the reveal event, so no per-deposit chain read is
	// made before it does - its current action generation is still
	// Pending (the contract would
	// revert the notification while an acceptance authorization is
	// pending), a previously submitted notification is inside its
	// renotify backoff window, or a notification was just submitted
	// and is awaiting its on-chain effect.
	StaleDepositResolutionKeep
	// StaleDepositResolutionDrop indicates the deposit is no longer a
	// staleness candidate without this watcher's notification being the
	// observed release: the reserved deposit record is not pending on-
	// chain - its wallet field is zero, whether the deposit was never
	// reserved or a release path already cleared it (a stale
	// notification or governance's force-release emits
	// ReservedDepositMarkedStale in the same transaction; the
	// owner's acceptance clears the record without emitting the
	// event). The caller should also call forgetDeposit to release any
	// per-deposit entries the watcher holds for this key.
	StaleDepositResolutionDrop
	// StaleDepositResolutionNotified indicates this watcher's release
	// was confirmed: an outstanding NotifyStaleReservedDeposit
	// submission took effect and the reserved deposit record's wallet
	// field cleared on-chain - observable, once the deadline has
	// passed, by the check's wallet-field read, with the
	// ReservedDepositMarkedStale event emitted by the very same
	// transaction that clears it. A submitted-but-not-yet-mined
	// notification resolves Keep, not Notified, until that cleared
	// field is observed. The caller must remove the deposit from its
	// tracking set and call forgetDeposit so the watcher does not
	// retain state for a deposit it will never check again.
	StaleDepositResolutionNotified
)

// ReservationStaleDepositWatcher observes deposit-revealed events and
// notifies the Bridge when a reserved deposit's on-chain refund deadline
// has passed without its acceptance action generation resolving.
//
// The Bridge records each reserved deposit's assigned wallet via
// `ReservedDepositWallet` and snapshots its exact Bitcoin refund deadline
// (the reveal-time `RefundLocktime`, byte-reversed; see
// reverseUint32) at reveal. Until that deadline the deposit cannot
// become stale - the contract rejects
// NotifyStaleReservedDeposit with "Deposit refund deadline has not
// elapsed" - so for a deposit discovered by pollTick, whose deadline is
// memoized from the reveal event, the watcher makes no per-deposit
// chain read at all before the deadline (an in-memory gate only); a
// deposit checked directly without that memo first recovers its
// deadline from chain state (see getRefundDeadline). Once the deadline
// has passed, the watcher notifies whenever the current action
// generation is not Pending, regardless of the assigned wallet's
// state: a Live wallet that never accepts its deposit is exactly the
// case the notification exists to clean up. Every tracked deposit is
// kept until its reserved deposit record is observed cleared on-chain
// (wallet field zero): both the owner's acceptance and a stale
// release clear the record, and only the stale-release paths - a
// successful NotifyStaleReservedDeposit or governance's
// forceStaleReservedDeposit - emit ReservedDepositMarkedStale. The
// zero field, not the event, is the completion signal; a
// still-pending record is never dropped merely because its action
// generation is TimedOut or has not reached Settled.
type ReservationStaleDepositWatcher struct {
	spvChain Chain
	// notifiedAt is the UNIX timestamp of this watcher's last
	// successful NotifyStaleReservedDeposit submission for a given
	// deposit key, or absent if none has ever succeeded. It is NOT
	// proof the notification took effect: NotifyStaleReservedDeposit's
	// generated chain binding returns as soon as the transaction is
	// submitted, not once it mines, so a submitted-but-dropped-or-
	// reverted transaction still leaves the reserved deposit record
	// open. After the deadline has passed, CheckStaleReservedDeposit
	// re-reads the record's wallet field on every call and only
	// resolves Notified - letting the caller evict the deposit - once
	// that field is observed cleared; until then it resolves Keep
	// and, once actionTimeoutRenotifyInterval has elapsed since
	// notifiedAt without the record clearing, retries the
	// notification.
	notifiedAt map[string]uint32

	// refundDeadlineMemo caches each tracked deposit's snapshotted
	// on-chain refund deadline, derived from its DepositRevealed
	// event's RefundLocktime via reverseUint32. pollTick fills it at
	// discovery time, when the reveal event it schedules from is
	// already in hand; CheckStaleReservedDeposit fills it lazily for
	// deposits checked directly (the snapshot's one-time reads run
	// only while this memo has no entry for the deposit), so in
	// steady operation no per-deposit chain read happens before the
	// deadline.
	refundDeadlineMemo map[string]uint32

	// operatorAddress identifies this process for
	// reservationOperatorStaggerOffset (see reservation_wiring.go),
	// used to stagger a deposit's FIRST NotifyStaleReservedDeposit
	// attempt. It is a required constructor parameter, not a
	// post-construction setter, so a watcher can never be
	// constructed in a partially-initialized state that would silently
	// skip staggering.
	operatorAddress common.Address

	// attempted records every deposit key for which
	// NotifyStaleReservedDeposit has already been attempted at least
	// once (success or failure), so CheckStaleReservedDeposit knows to
	// apply the stagger offset only to the FIRST attempt (see
	// reservationOperatorStaggerOffset).
	attempted map[string]struct{}

	// pending, together with lastSeenBlock, is this watcher's own
	// poll-loop cross-tick state (see Run and pollTick): the
	// incremental DepositRevealed scan cursor and the tracked-deposit
	// set. Folding this state into the watcher itself - rather than a
	// separate type the wiring layer had to construct and thread
	// through free functions - mirrors
	// ReservationActionTimeoutWatcher's self-contained Run loop.
	//
	// The set is a single one: the old split between an actively-
	// polled set and a slower-reconciled set of Live-wallet deposits
	// is gone, because deadline-aware scheduling makes it redundant.
	// Before a deposit's refund deadline passes it costs no chain
	// reads at all (nothing to amortize away), and after the deadline
	// every tracked deposit - including ones whose wallet is Live,
	// since the notification is wallet-state-independent - must be
	// re-checked every tick until its record clears on-chain.
	pending       map[string]*big.Int
	lastSeenBlock uint64

	// activationBlock is the network's reservation activation block
	// (see tbtc.ReservationsActivationBlock): the first reveal scan
	// starts here instead of a fixed lookback window, so a process
	// restart after activation picks up every reveal that has appeared
	// since the feature went live. math.MaxUint64 is the sentinel
	// for a network without an entry (e.g. ethereum.Unknown): the
	// watcher skips the startup scan entirely - reservations are
	// inactive - and the cursor jumps to the chain head.
	activationBlock uint64
}

// errStaleDepositCleared signals that the reserved deposit record was
// already released on-chain (its wallet field is zero) when a refund
// deadline snapshot was attempted: there is nothing left to schedule or
// notify for this deposit.
var errStaleDepositCleared = errors.New("reserved deposit record already cleared")

// NewReservationStaleDepositWatcher constructs a stale-deposit watcher
// bound to the given chain. operatorAddress is required: it is used by
// CheckStaleReservedDeposit to derive a deterministic per-operator
// stagger offset for a deposit's FIRST NotifyStaleReservedDeposit
// attempt (see reservationOperatorStaggerOffset). It is a constructor
// parameter, not a post-construction setter, so a watcher can never be
// constructed in a partially-initialized state that would silently skip
// staggering.
func NewReservationStaleDepositWatcher(
	spvChain Chain,
	operatorAddress common.Address,
	activationBlock uint64,
) *ReservationStaleDepositWatcher {
	return &ReservationStaleDepositWatcher{
		spvChain:           spvChain,
		operatorAddress:    operatorAddress,
		notifiedAt:         make(map[string]uint32),
		refundDeadlineMemo: make(map[string]uint32),
		attempted:          make(map[string]struct{}),
		pending:            make(map[string]*big.Int),
		activationBlock:    activationBlock,
	}
}

// reverseUint32 decodes the 4-byte little-endian Bitcoin refund locktime
// carried by a DepositRevealed event into the uint32 UNIX deadline the
// Bridge compares against block.timestamp, mirroring
// BTCUtils.reverseUint32(uint32(refundLocktime)) in tbtc-v2's
// Deposit.sol: Solidity first reads the raw bytes big-endian and then
// byte-swaps the result, which is exactly a little-endian read of the
// original byte array.
func reverseUint32(locktime [4]byte) uint32 {
	return binary.LittleEndian.Uint32(locktime[:])
}

// CheckStaleReservedDeposit is the synchronous core of the watcher. It is
// invoked by the polling loop (or directly, in tests) for each tracked
// deposit.
//
// The function may submit a Bridge notification
// (NotifyStaleReservedDeposit) as a side effect, and it caches the
// snapshotted refund deadline and notification state on the receiver
// across calls. There is no internal scheduling; the caller owns
// invocation lifecycle and synchronization.
//
// Notification eligibility mirrors the contract's
// notifyStaleReservedDeposit guards:
//
//  1. The deposit's reserved record is still open: its wallet field is
//     non-zero. The owner's acceptance and the stale-release paths
//     (a successful NotifyStaleReservedDeposit - this watcher's or
//     another operator's - or governance's forceStaleReservedDeposit)
//     all clear the record; only the stale-release paths emit
//     ReservedDepositMarkedStale in the clearing transaction. The
//     zero field, not the event, is the completion signal.
//  2. The snapshotted refund deadline (the reveal-time RefundLocktime,
//     byte-reversed; see reverseUint32) has STRICTLY passed: the
//     contract's requirement is block.timestamp > refundDeadline. At
//     or before the deadline, a deposit discovered by pollTick
//     performs no per-deposit chain reads at all (in-memory gate
//     only); a deposit checked directly without that memo first
//     recovers its deadline from chain state (see getRefundDeadline).
//  3. The current action generation is not Pending: while an
//     acceptance authorization is pending, the contract reverts the
//     notification with "Acceptance authorization pending", so the
//     deposit stays tracked until the generation resolves - TimedOut,
//     Settled, Superseded, Vetoed, or no generation ever requested -
//     regardless of the assigned wallet's state.
//
// Submitting NotifyStaleReservedDeposit does NOT immediately resolve
// Notified: the generated chain binding returns as soon as the
// transaction is submitted, not once it mines (mining is handled
// elsewhere by a background ForceMining goroutine), so a call that
// just submitted the notification resolves Keep and records notifiedAt.
// Only a LATER call that observes the record's wallet field cleared
// retires the deposit - as Notified when this watcher's own
// submission was outstanding, otherwise as Drop. If
// actionTimeoutRenotifyInterval elapses without the record clearing,
// the notification is retried rather than the deposit being silently
// abandoned or falsely declared resolved.
//
// Parameters:
//   - depositKey: the deposit identifier reported by the Bridge.
//   - now:        the UNIX timestamp against which the refund deadline
//     and the notifiedAt renotify backoff are compared. Tests pass an
//     explicit value; production passes time.Now().Unix() cast to
//     uint32.
func (rsdw *ReservationStaleDepositWatcher) CheckStaleReservedDeposit(
	depositKey *big.Int,
	now uint32,
) (StaleDepositResolution, error) {
	if depositKey == nil {
		return StaleDepositResolutionUnknown, fmt.Errorf("deposit key must not be nil")
	}

	depositKeyStr := depositKey.String()

	deadline, err := rsdw.getRefundDeadline(depositKey)
	if errors.Is(err, errStaleDepositCleared) {
		// The record was already released on-chain before its deadline
		// could be snapshotted: there is nothing to release.
		logger.Debugf(
			"reserved deposit [%v] record already cleared; skipping "+
				"stale check",
			depositKey,
		)
		if _, outstanding := rsdw.notifiedAt[depositKeyStr]; outstanding {
			return StaleDepositResolutionNotified, nil
		}
		return StaleDepositResolutionDrop, nil
	}
	if err != nil {
		return StaleDepositResolutionUnknown, err
	}

	// Until the snapshotted refund deadline has STRICTLY passed the
	// contract rejects the notification and nothing about this deposit
	// is eligible; keep the check in-memory-only until it does.
	if now <= deadline {
		logger.Debugf(
			"reserved deposit [%v] refund deadline [%d] not yet reached "+
				"(now=%d); no chain reads until it passes",
			depositKey,
			deadline,
			now,
		)
		return StaleDepositResolutionKeep, nil
	}

	// Past the deadline: per-deposit chain reads begin with the
	// wallet-field read below. The record's wallet field is the
	// completion signal and also the pending-or-not discriminator: a
	// reveal that satisfied the vault-match filter but was not
	// actually reserved has no pendingReservedDeposit record on
	// chain, so ReservedDepositWallet reads back zero and retires
	// the deposit here without burning a NotifyStaleReservedDeposit
	// transaction.

	walletPublicKeyHash, err := rsdw.spvChain.ReservedDepositWallet(depositKey)
	if err != nil {
		return StaleDepositResolutionUnknown, fmt.Errorf(
			"failed to fetch wallet for reserved deposit [%v]: [%v]",
			depositKey,
			err,
		)
	}

	// A zero wallet field means the reserved deposit record was
	// cleared on-chain - this watcher's stale notification mined,
	// another operator's did, the owner's acceptance consumed it, or
	// governance force-cleared it - and there is nothing left to
	// release: only the stale-release paths emit
	// ReservedDepositMarkedStale, while the owner's acceptance
	// clears the record without emitting it, so the zero field - not
	// the event - is the completion signal.
	if walletPublicKeyHash == ([20]byte{}) {
		logger.Debugf(
			"reserved deposit [%v] record cleared on-chain; retiring from "+
				"tracking",
			depositKey,
		)
		if _, outstanding := rsdw.notifiedAt[depositKeyStr]; outstanding {
			// The release we submitted is now visible on-chain: that
			// is the real evidence the notification took effect.
			return StaleDepositResolutionNotified, nil
		}
		return StaleDepositResolutionDrop, nil
	}

	if notifiedAt, everNotified := rsdw.notifiedAt[depositKeyStr]; everNotified {
		if now < notifiedAt || now-notifiedAt < uint32(actionTimeoutRenotifyInterval.Seconds()) {
			// now < notifiedAt is treated the same as "interval not yet
			// elapsed" rather than falling through to retry: now is a
			// caller-supplied, not-guaranteed-monotonic tick token, so
			// without this guard a clock adjustment or an out-of-order
			// now would underflow the subtraction below to a huge
			// uint32 and fail OPEN - resubmitting immediately instead
			// of waiting out the backoff. Staying in Keep is the safe
			// direction for an ambiguous time comparison; a genuine
			// elapsed interval will still be observed on a later tick
			// with a larger now.
			//
			// NotifyStaleReservedDeposit's generated chain binding
			// returns as soon as the transaction is submitted, not once
			// it mines (mining is handled by a background ForceMining
			// goroutine elsewhere), so give a recently-submitted
			// notification time to land before resubmitting or retiring
			// the deposit on the strength of the local send alone.
			return StaleDepositResolutionKeep, nil
		}

		// actionTimeoutRenotifyInterval has elapsed without the record
		// clearing: the prior transaction may have been dropped or
		// reverted. Clear notifiedAt and fall through to retry the
		// notification below rather than leaving the deposit stuck
		// forever awaiting an effect that never comes.
		delete(rsdw.notifiedAt, depositKeyStr)
	}

	// In m1 the reservation key and deposit key share the same
	// identifier space exposed by the Bridge (ReservedDepositWallet
	// and Reservation are both keyed by the same value); future
	// revisions of the Bridge may introduce disjoint identifiers, in
	// which case this direct use of depositKey as reservationKey must
	// be replaced with a real lookup.
	reservationKey := depositKey

	reservation, err := rsdw.spvChain.GetReservation(reservationKey)
	if err != nil {
		return StaleDepositResolutionUnknown, fmt.Errorf(
			"failed to fetch reservation [%v]: [%v]",
			reservationKey,
			err,
		)
	}

	// The contract's getAction returns the zero storage record for a
	// requested nonce that has no record on-chain, which reads as
	// state Unknown. Since "not Pending" is the notification
	// eligibility gate, a deposit whose reservation has never had an
	// action generation requested is eligible once its refund deadline
	// passes.
	actionState := tbtc.ReservationActionStateUnknown
	if reservation.RequestNonce != 0 {
		action, err := rsdw.spvChain.GetReservationAction(
			reservationKey,
			reservation.RequestNonce,
		)
		if err != nil {
			return StaleDepositResolutionUnknown, fmt.Errorf(
				"failed to fetch reservation action for [%v] nonce [%d]: [%v]",
				reservationKey,
				reservation.RequestNonce,
				err,
			)
		}
		actionState = action.State
	}

	if actionState == tbtc.ReservationActionStatePending {
		// While an acceptance authorization is pending the contract
		// reverts the notification, so keep the deposit tracked until
		// the generation resolves - it must not be dropped merely
		// because the current generation has not yet reached Settled
		// - regardless of the assigned wallet's state.
		logger.Debugf(
			"reserved deposit [%v] current action state=%s; the "+
				"notification would revert while an acceptance "+
				"authorization is pending; keeping it tracked",
			depositKey,
			actionState,
		)
		return StaleDepositResolutionKeep, nil
	}

	if _, alreadyAttempted := rsdw.attempted[depositKeyStr]; !alreadyAttempted {
		// FIRST notify attempt for this deposit: stagger it by a
		// deterministic per-operator offset (see
		// reservationOperatorStaggerOffset in reservation_wiring.go) so
		// that many reservation-enabled operators independently
		// discovering the same overdue deposit do not all submit their
		// first NotifyStaleReservedDeposit attempt in the same poll
		// tick and collide on-chain. Unlike the renotify backoff above
		// (applied once a submission has already been made), only the
		// FIRST attempt is staggered; reusing
		// actionTimeoutRenotifyInterval as the stagger window avoids
		// introducing yet another near-duplicate interval constant.
		offset := reservationOperatorStaggerOffset(
			rsdw.operatorAddress,
			depositKeyStr,
			actionTimeoutRenotifyInterval,
		)
		if now <= deadline+offset {
			return StaleDepositResolutionKeep, nil
		}
		rsdw.attempted[depositKeyStr] = struct{}{}
	}

	if err := rsdw.spvChain.NotifyStaleReservedDeposit(depositKey); err != nil {
		return StaleDepositResolutionUnknown, fmt.Errorf(
			"failed to notify stale reserved deposit [%v]: [%v]",
			depositKey,
			err,
		)
	}
	rsdw.notifiedAt[depositKeyStr] = now

	logger.Infof(
		"submitted stale reserved deposit notification for [%v] "+
			"(wallet [0x%x], refund deadline %d, current action "+
			"state=%s); keeping it tracked until the reserved deposit "+
			"record clears on-chain",
		depositKey,
		walletPublicKeyHash,
		deadline,
		actionState,
	)

	return StaleDepositResolutionKeep, nil
}

// getRefundDeadline returns the deposit's snapshotted on-chain refund
// deadline, memoized on the receiver (see refundDeadlineMemo):
// pollTick fills it at discovery time, and this call back-fills it
// lazily for a deposit checked directly, paying the snapshot's one-time
// reads only while no entry exists yet.
func (rsdw *ReservationStaleDepositWatcher) getRefundDeadline(
	depositKey *big.Int,
) (uint32, error) {
	if deadline, ok := rsdw.refundDeadlineMemo[depositKey.String()]; ok {
		return deadline, nil
	}

	deadline, err := rsdw.snapshotRefundDeadline(depositKey)
	if err != nil {
		return 0, err
	}
	rsdw.refundDeadlineMemo[depositKey.String()] = deadline
	return deadline, nil
}

// snapshotRefundDeadline snapshots a deposit's on-chain refund deadline
// for the first time, from the RefundLocktime field of its own
// DepositRevealed event (decoded via reverseUint32 to the exact
// deadline the Bridge snapshotted at reveal - see tbtc-v2's
// Deposit.sol reveal-time snapshot of validateDepositRefundLocktime's
// result). The deposit's assigned wallet is read first, both to key the
// reveal-event scan and as an early exit: a zero wallet field means the
// record was already released, and there is nothing to schedule.
//
// The event scan is bounded by reservationDefaultLookBackBlocks, the
// same catch-up window every reservation watcher shares; a reveal that
// predates it (and was never seen by pollTick) cannot be snapshotted
// here and surfaces as an error the caller retries.
func (rsdw *ReservationStaleDepositWatcher) snapshotRefundDeadline(
	depositKey *big.Int,
) (uint32, error) {
	walletPublicKeyHash, err := rsdw.spvChain.ReservedDepositWallet(depositKey)
	if err != nil {
		return 0, fmt.Errorf(
			"failed to fetch wallet for reserved deposit [%v]: [%w]",
			depositKey,
			err,
		)
	}
	if walletPublicKeyHash == ([20]byte{}) {
		return 0, fmt.Errorf("%w for deposit [%v]", errStaleDepositCleared, depositKey)
	}

	blockCounter, err := rsdw.spvChain.BlockCounter()
	if err != nil {
		return 0, fmt.Errorf(
			"failed to get block counter for refund deadline snapshot: [%w]",
			err,
		)
	}
	if blockCounter == nil {
		return 0, fmt.Errorf(
			"failed to get block counter for refund deadline snapshot: nil block counter",
		)
	}
	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		return 0, fmt.Errorf(
			"failed to get current block for refund deadline snapshot: [%w]",
			err,
		)
	}

	startBlock := uint64(0)
	if currentBlock > reservationDefaultLookBackBlocks {
		startBlock = currentBlock - reservationDefaultLookBackBlocks
	}

	events, eventsErr := rsdw.spvChain.PastDepositRevealedEvents(
		&tbtc.DepositRevealedEventFilter{
			StartBlock:          startBlock,
			EndBlock:            &currentBlock,
			WalletPublicKeyHash: [][20]byte{walletPublicKeyHash},
		},
	)
	if eventsErr != nil {
		return 0, fmt.Errorf(
			"failed to fetch deposit revealed events for refund deadline "+
				"snapshot: [%w]",
			eventsErr,
		)
	}

	var matchingEvent *tbtc.DepositRevealedEvent
	for _, event := range events {
		if rsdw.spvChain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		).Cmp(depositKey) == 0 {
			matchingEvent = event
			break
		}
	}
	if matchingEvent == nil {
		return 0, fmt.Errorf(
			"no matching DepositRevealed event for deposit [%v] within the "+
				"catch-up window",
			depositKey,
		)
	}

	return reverseUint32(matchingEvent.RefundLocktime), nil
}

// forgetDeposit clears any cached notification state and snapshotted
// refund deadline held for the given deposit key. The poller invokes
// this once a deposit resolves to Drop or Notified, so a resolved
// deposit's per-call cache entries do not linger in these maps for the
// remaining life of the process.
func (rsdw *ReservationStaleDepositWatcher) forgetDeposit(depositKey *big.Int) {
	key := depositKey.String()
	delete(rsdw.notifiedAt, key)
	delete(rsdw.refundDeadlineMemo, key)
	delete(rsdw.attempted, key)
}

// trackedCount returns the total number of deposits currently tracked by
// this watcher's poll loop.
func (rsdw *ReservationStaleDepositWatcher) trackedCount() int {
	return len(rsdw.pending)
}

// pollTick runs one stale-deposit poll pass: it fetches deposit-revealed
// events since rsdw.lastSeenBlock, adds every reserved deposit among
// them to the tracked set - snapshotting its refund deadline from the
// reveal event's RefundLocktime at the same moment - then re-runs
// CheckStaleReservedDeposit for every tracked deposit. A tracked
// deposit whose deadline has not yet passed performs no chain reads
// during that pass (the check's in-memory gate), so steady-state
// per-tick cost no longer grows with the number of still-open deposits.
// A deposit is removed once it resolves Drop (its record was already
// released on-chain, or it is not reserved) or Notified (its own
// notification was confirmed by the record clearing); neither needs
// further polling. A deposit that resolves Keep stays tracked -
// including ones whose wallet is Live, since a Live wallet may never
// anchor the deposit and, once the refund deadline passes, the
// notification is eligible regardless of wallet state.
//
// The poller is intentionally tolerant of chain errors: a transient RPC
// failure logs and continues rather than aborting the poll pass, but
// such a failure also makes the tick's activity count unreliable as a
// zero-activity signal (the caller cannot distinguish "genuinely no
// activity" from "the scan that would have found it failed"). Returns
// the total tracked count after the tick, and a second bool that is
// true only when every chain read this tick (block counter, current
// block, deposit-revealed event scan, reservation parameters)
// definitively succeeded - false makes the count untrustworthy.
// Callers that want a results signal (see
// WireReservationWatchers's misconfiguration self-check) must check
// both.
func (rsdw *ReservationStaleDepositWatcher) pollTick(now uint32) (int, bool) {
	blockCounter, err := rsdw.spvChain.BlockCounter()
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to get block counter: [%v]",
			err,
		)
		return rsdw.trackedCount(), false
	}
	currentBlock, err := blockCounter.CurrentBlock()
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to get current block: [%v]",
			err,
		)
		return rsdw.trackedCount(), false
	}

	// queryStartBlock is the inclusive lower bound of the next reveal
	// scan. On the very first tick (lastSeenBlock == 0) it is the
	// network's reservation activation block, so a process restart
	// after activation picks up every reveal since the feature went
	// live instead of the fixed 30-day lookback. Networks without an
	// activation entry (math.MaxUint64) skip the catch-up scan
	// entirely: reservations are inactive, so there are no reveals
	// to find, and the cursor jumps to the head. Steady-state ticks
	// resume at lastSeenBlock+1, unchanged from the original
	// incremental scan.
	var queryStartBlock uint64
	switch rsdw.lastSeenBlock {
	case 0:
		switch {
		case rsdw.activationBlock == math.MaxUint64,
			rsdw.activationBlock > currentBlock:
			// Skip the catch-up window entirely (cursor on the
			// next tick starts from this head). Returning early here
			// (before the ReservationParameters fetch) keeps the
			// tick fast on networks where reservations are not
			// active; pending is empty so the Check loop below is a
			// no-op anyway.
			rsdw.lastSeenBlock = currentBlock
			return len(rsdw.pending), true
		default:
			queryStartBlock = rsdw.activationBlock
		}
	default:
		queryStartBlock = rsdw.lastSeenBlock + 1
	}

	// Chunk the reveal-event scan so a wide catch-up window
	// (activation block to chain tip) does not become a single huge
	// PastDepositRevealedEvents query. Steady-state deltas are small
	// enough to fall into a single chunk.
	events, err := fetchPastEventsInChunks[*tbtc.DepositRevealedEvent](
		func(chunkStart, chunkEnd uint64) ([]*tbtc.DepositRevealedEvent, error) {
			return rsdw.spvChain.PastDepositRevealedEvents(
				&tbtc.DepositRevealedEventFilter{
					StartBlock: chunkStart,
					EndBlock:   &chunkEnd,
				},
			)
		},
		queryStartBlock,
		currentBlock,
	)
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to fetch deposit revealed "+
				"events: [%v]",
			err,
		)
		return rsdw.trackedCount(), false
	}

	params, err := rsdw.spvChain.ReservationParameters()
	if err != nil {
		reservationWiringLogger.Errorf(
			"stale-deposit poll failed to fetch reservation "+
				"parameters: [%v]",
			err,
		)
		return rsdw.trackedCount(), false
	}

	for _, event := range events {
		if event.Vault == nil || !strings.EqualFold(string(*event.Vault), string(params.ReservationVault)) {
			continue
		}

		depositKey := rsdw.spvChain.BuildDepositKey(
			event.FundingTxHash,
			event.FundingOutputIndex,
		)

		// Track every vault-matched reveal in memory only: the vault
		// match (ReservationParameters().ReservationVault, fetched
		// once per tick) is the discovery gate, and the refund
		// deadline is memoized from the reveal event in hand, so a
		// tracked deposit performs no per-deposit chain reads until
		// its deadline passes - by which point CheckStaleReservedDeposit
		// reads the record's wallet field first. Doing any read here
		// would re-introduce the per-deposit RPC the deadline-aware
		// scheduling was meant to eliminate.

		rsdw.pending[depositKey.String()] = depositKey
		rsdw.refundDeadlineMemo[depositKey.String()] = reverseUint32(
			event.RefundLocktime,
		)
	}

	rsdw.lastSeenBlock = currentBlock

	for key, depositKey := range rsdw.pending {
		resolution, err := rsdw.CheckStaleReservedDeposit(depositKey, now)
		if err != nil {
			reservationWiringLogger.Errorf(
				"stale-deposit poll failed to check deposit "+
					"[%v]: [%v]",
				depositKey,
				err,
			)
			continue
		}

		switch resolution {
		case StaleDepositResolutionDrop, StaleDepositResolutionNotified:
			delete(rsdw.pending, key)
			rsdw.forgetDeposit(depositKey)
		case StaleDepositResolutionKeep:
			// Stays tracked: before its refund deadline it costs no
			// chain reads at all, and after the deadline it is
			// re-checked every tick - including while its wallet is
			// Live - until its record clears on-chain.
		}
	}

	return len(rsdw.pending), true
}

// Run starts the background poll loop, mirroring
// ReservationActionTimeoutWatcher.Run. It returns when ctx is done or
// when a non-positive interval is supplied. Every interval tick runs
// one poll pass over the tracked deposit set (pollTick). No separate
// slower reconcile pass is needed: before a tracked deposit's
// snapshotted refund deadline passes it costs no chain reads at all,
// and after the deadline every tracked deposit - Live-wallet ones
// included, since the notification is wallet-state-independent - must
// be re-checked every tick until its record clears on-chain.
func (rsdw *ReservationStaleDepositWatcher) Run(
	ctx context.Context,
	interval time.Duration,
) error {
	if interval <= 0 {
		return fmt.Errorf(
			"stale-deposit watcher requires a positive poll interval",
		)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		now := uint32(time.Now().Unix())
		rsdw.pollTick(now)
	}
}
