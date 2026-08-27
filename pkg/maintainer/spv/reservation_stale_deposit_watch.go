package spv

import (
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ReservationStaleDepositWatcher observes deposit-revealed events and
// notifies the Bridge when a reserved deposit's acceptance window expired
// without the assigned wallet becoming live.
//
// A reserved deposit is a deposit that was revealed against a reservation
// vault address. The Bridge records the assigned wallet via
// `ReservedDepositWallet`. If that wallet fails to transition to StateLive
// within the reservation action timeout window, the deposit must be released
// back to the default deposit sweep path; otherwise it sits orphaned
// because the anchor can never be produced. The watcher is the backstop that
// flips the deposit's bookkeeping when the wallet never shows up.
type ReservationStaleDepositWatcher struct {
	spvChain Chain
	notifier StaleReservedDepositNotifier
}

// StaleReservedDepositNotifier is the Bridge-facing contract for releasing a
// reserved deposit back to the default sweep path. It mirrors
// `Chain.NotifyStaleReservedDeposit` but is interface-typed to enable
// in-memory recorders during tests.
type StaleReservedDepositNotifier interface {
	NotifyStaleReservedDeposit(depositKey *big.Int) error
}

// StaleReservedDepositNotifierFunc adapts a function to the
// StaleReservedDepositNotifier interface.
type StaleReservedDepositNotifierFunc func(depositKey *big.Int) error

// NotifyStaleReservedDeposit forwards the call to the wrapped function.
func (f StaleReservedDepositNotifierFunc) NotifyStaleReservedDeposit(
	depositKey *big.Int,
) error {
	return f(depositKey)
}

// NewReservationStaleDepositWatcher constructs a stale-deposit watcher
// bound to the given chain and notifier.
func NewReservationStaleDepositWatcher(
	spvChain Chain,
	notifier StaleReservedDepositNotifier,
) *ReservationStaleDepositWatcher {
	return &ReservationStaleDepositWatcher{
		spvChain: spvChain,
		notifier: notifier,
	}
}

// OnDepositRevealed is the entry-point Bridge event for the watcher. It
// inspects the revealed deposit and decides whether to register a deferred
// stale check or skip the deposit entirely.
//
// Behavior:
//
//  1. If `IsReservedDeposit(depositKey)` is false the deposit is on the
//     default sweep path; the watcher takes no action.
//  2. If the assigned wallet (`ReservedDepositWallet`) is already
//     StateLive, the deposit will anchor through the normal path; the
//     watcher takes no action.
//  3. Otherwise the watcher records the deposit as pending-stale and
//     arranges for `CheckStaleReservedDeposit` to fire once the action
//     timeout window has elapsed. The exact deferral mechanism is the
//     integration step's responsibility (sleep loop, time.AfterFunc, or
//     scheduled job); the watcher exposes the operation as a pure
//     function so the integration can pick the right primitive.
//
// Pass an explicit `now` for deterministic tests; production wires this to
// `time.Now().Unix()` in the caller.
func (rsdw *ReservationStaleDepositWatcher) OnDepositRevealed(
	depositKey *big.Int,
	now uint32,
) error {
	if rsdw.notifier == nil {
		return fmt.Errorf("stale-deposit watcher requires a non-nil notifier")
	}
	if depositKey == nil {
		return fmt.Errorf("deposit key must not be nil")
	}

	return rsdw.CheckStaleReservedDeposit(depositKey, now)
}

// CheckStaleReservedDeposit is the synchronous core of the watcher. It is
// invoked both by OnDepositRevealed (immediately after the event) and by
// the integration's deferred callback (once the action timeout window has
// elapsed).
//
// The function is intentionally pure: given the chain state and a `now`
// timestamp, it either notifies the Bridge of a stale deposit or skips
// silently. There is no internal scheduling; the caller owns the lifecycle.
//
// Conditions for notification:
//
//  1. `IsReservedDeposit(depositKey)` returns true. A non-reserved deposit
//     is the default sweep path's responsibility; the watcher must not
//     interfere with it.
//  2. The reservation's assigned wallet exists and is NOT in StateLive.
//     A live wallet is expected to anchor the deposit itself; the action
//     timeout window only applies when the wallet is missing or has not
//     progressed to live.
//  3. The action timeout has elapsed. The watcher derives the timeout
//     from the reservation action record at the current nonce. If the
//     action has already been advanced (Settled/TimedOut/Superseded/Vetoed),
//     the deposit is no longer in the pending-stale window and the watcher
//     skips it without notifying.
//
// Parameters:
//   - depositKey:  the deposit identifier reported by the Bridge.
//   - now:        the UNIX timestamp against which the action timeout is
//     compared. Tests pass an explicit value; production passes
//     time.Now().Unix() cast to uint32.
func (rsdw *ReservationStaleDepositWatcher) CheckStaleReservedDeposit(
	depositKey *big.Int,
	now uint32,
) error {
	if rsdw.notifier == nil {
		return fmt.Errorf("stale-deposit watcher requires a non-nil notifier")
	}
	if depositKey == nil {
		return fmt.Errorf("deposit key must not be nil")
	}

	isReserved, err := rsdw.spvChain.IsReservedDeposit(depositKey)
	if err != nil {
		return fmt.Errorf(
			"failed to determine if deposit [%v] is reserved: [%v]",
			depositKey,
			err,
		)
	}
	if !isReserved {
		logger.Debugf(
			"deposit [%v] is not a reserved deposit; skipping stale check",
			depositKey,
		)
		return nil
	}

	walletPublicKeyHash, err := rsdw.spvChain.ReservedDepositWallet(depositKey)
	if err != nil {
		return fmt.Errorf(
			"failed to fetch wallet for reserved deposit [%v]: [%v]",
			depositKey,
			err,
		)
	}

	// The Bridge only assigns a non-zero wallet to a reserved deposit.
	// Defensive: if the wallet is zero the deposit bookkeeping is broken;
	// rather than notify on partial information, we skip with a warning.
	if walletPublicKeyHash == ([20]byte{}) {
		logger.Warnf(
			"reserved deposit [%v] has no wallet assigned; "+
				"skipping stale notification",
			depositKey,
		)
		return nil
	}

	wallet, err := rsdw.spvChain.GetWallet(walletPublicKeyHash)
	if err != nil {
		return fmt.Errorf(
			"failed to fetch wallet [0x%x] for reserved deposit [%v]: [%v]",
			walletPublicKeyHash,
			depositKey,
			err,
		)
	}

	// Wallet live: anchor is expected on its own. The watcher does not
	// interfere.
	if wallet.State == tbtc.StateLive {
		logger.Debugf(
			"reserved deposit [%v] assigned to live wallet [0x%x]; "+
				"anchor expected; skipping stale notification",
			depositKey,
			walletPublicKeyHash,
		)
		return nil
	}

	// The action timeout is the deadline bound to the reservation action
	// generation. Reserved deposits carry exactly one action generation
	// (the acceptance) so we look it up via the reservation key.
	reservationKey := depositToReservationKey(depositKey)
	if reservationKey == nil {
		logger.Warnf(
			"could not derive reservation key from deposit key [%v]; "+
				"skipping stale notification",
			depositKey,
		)
		return nil
	}

	action, err := rsdw.spvChain.GetReservationAction(reservationKey, 0)
	if err != nil {
		return fmt.Errorf(
			"failed to load acceptance action for reservation [%v]: [%v]",
			reservationKey,
			err,
		)
	}

	// A non-pending action means the acceptance already progressed past the
	// timeout-eligible window. Skip without notifying.
	if action.State != tbtc.ReservationActionStatePending {
		logger.Debugf(
			"reservation [%v] acceptance action state=%s; "+
				"deposit [%v] is no longer pending-stale; skipping",
			reservationKey,
			action.State,
			depositKey,
		)
		return nil
	}

	if now <= action.TimeoutAt {
		logger.Debugf(
			"reserved deposit [%v] action timeout at [%d] not yet reached "+
				"(now=%d); deferring stale notification",
			depositKey,
			action.TimeoutAt,
			now,
		)
		return nil
	}

	if err := rsdw.notifier.NotifyStaleReservedDeposit(depositKey); err != nil {
		return fmt.Errorf(
			"failed to notify stale reserved deposit [%v]: [%v]",
			depositKey,
			err,
		)
	}

	logger.Infof(
		"notified stale reserved deposit [%v] "+
			"(wallet [0x%x] state=%s, action timeout %d)",
		depositKey,
		walletPublicKeyHash,
		wallet.State,
		action.TimeoutAt,
	)

	return nil
}

// depositToReservationKey maps a deposit identifier to the reservation key
// the action is filed under. In m1 the mapping is identical because
// ReservedDepositWallet and Reservation share the same identifier space
// exposed by the Bridge; future revisions of the Bridge may introduce
// disjoint identifiers, in which case the mapping is filled in by the
// integration layer.
//
// Returning nil here signals "unknown mapping"; the caller treats nil as a
// soft skip, not an error.
func depositToReservationKey(depositKey *big.Int) *big.Int {
	if depositKey == nil {
		return nil
	}
	return new(big.Int).Set(depositKey)
}
