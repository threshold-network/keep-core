package spv

import (
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ReservationActionTimeoutWatcher observes the reservation action set and
// notifies the Bridge when a pending action's on-chain deadline has elapsed
// without an SPV proof being submitted.
//
// The Bridge uses a per-action timeout window (snaphotted in the
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
type ReservationActionTimeoutWatcher struct {
	spvChain Chain
	notifier ReservationActionTimeoutNotifier
	// nowFn returns the current UNIX timestamp the watcher treats as "now"
	// for `now > timeoutAt` comparisons. Tests override it to drive the
	// deadline forward; production wires it to time.Now in UTC.
	nowFn func() uint32
	// interval is how often the background poll loop re-checks pending
	// actions. A zero value disables the background loop; tests and the
	// synchronously driven integration code path will use a positive
	// duration.
	interval time.Duration
	// membersResolver turns a wallet public key hash into the operator IDs
	// the Bridge expects for the slashing argument. The resolver is
	// injected to keep the watcher independent of the chain interface used
	// to look up operator addresses (the SPV maintainer chain interface
	// does not expose GetOperatorID today).
	membersResolver WalletMembersResolver
}

// WalletMembersResolver maps a wallet public key hash to the operator IDs
// the wallet's signing group is composed of. The Bridge uses the IDs to
// attribute slashing; m2+ will layer the actual penalty computation on top
// of the IDs carried in NotifyReservationActionTimeout.
//
// In production the resolver must look up the wallet's signing group via
// the maintenance bridge / sortition pool and translate each operator
// address to an operator ID via chain.GetOperatorID. The watcher does not
// prescribe a particular lookup because the production wiring depends on
// the sortition backend chosen for the deployment.
type WalletMembersResolver interface {
	ResolveWalletMembers(walletPublicKeyHash [20]byte) ([]uint32, error)
}

// WalletMembersResolverFunc adapts a plain function to the
// WalletMembersResolver interface.
type WalletMembersResolverFunc func(walletPublicKeyHash [20]byte) ([]uint32, error)

// ResolveWalletMembers forwards the call to the wrapped function.
func (f WalletMembersResolverFunc) ResolveWalletMembers(
	walletPublicKeyHash [20]byte,
) ([]uint32, error) {
	return f(walletPublicKeyHash)
}

// ReservationActionTimeoutNotifier is the Bridge-facing contract for the
// action-timeout watcher. It mirrors
// `Chain.NotifyReservationActionTimeout` but is interface-typed to enable
// in-memory recorders during tests.
type ReservationActionTimeoutNotifier interface {
	NotifyReservationActionTimeout(
		reservationKey *big.Int,
		walletMembersIDs []uint32,
	) error
}

// ReservationActionTimeoutNotifierFunc adapts a function to the
// ReservationActionTimeoutNotifier interface.
type ReservationActionTimeoutNotifierFunc func(
	reservationKey *big.Int,
	walletMembersIDs []uint32,
) error

// NotifyReservationActionTimeout forwards the call to the wrapped function.
func (f ReservationActionTimeoutNotifierFunc) NotifyReservationActionTimeout(
	reservationKey *big.Int,
	walletMembersIDs []uint32,
) error {
	return f(reservationKey, walletMembersIDs)
}

// NewReservationActionTimeoutWatcher constructs a watcher bound to the
// given chain, notifier, members resolver, and poll interval.
//
// The members resolver is mandatory: the watcher will refuse to operate
// without it because emitting NotifyReservationActionTimeout with a nil
// or empty member slice would be ill-formed on the Bridge side.
//
// A zero pollInterval disables the background loop; the watcher must then
// be driven by CheckReservationActionTimeouts calls from the integration.
func NewReservationActionTimeoutWatcher(
	spvChain Chain,
	notifier ReservationActionTimeoutNotifier,
	membersResolver WalletMembersResolver,
	pollInterval time.Duration,
) *ReservationActionTimeoutWatcher {
	return &ReservationActionTimeoutWatcher{
		spvChain:        spvChain,
		notifier:        notifier,
		nowFn:           defaultActionTimeoutNowFn,
		interval:        pollInterval,
		membersResolver: membersResolver,
	}
}

// defaultActionTimeoutNowFn returns time.Now() as a uint32 UNIX timestamp.
// Kept separate from the struct to allow tests to swap it deterministically.
func defaultActionTimeoutNowFn() uint32 {
	return uint32(time.Now().Unix())
}

// Run starts the background poll loop. It returns immediately and runs
// until ctx is done.
//
// Each iteration enumerates the reservations of every wallet registered
// with the watcher (added via WatchWallet), inspects each nonce-keyed
// action, and notifies the Bridge for those whose state is Pending and
// whose TimeoutAt has elapsed.
//
// Integration code typically calls Run once at startup and WatchWallet per
// discovered wallet. The loop is best-effort: errors are logged and the
// next iteration retries.
//
// Note: Run is a placeholder for the integration wiring in this PR. The
// per-wallet reservation enumeration rides on top of the stranding watcher's
// discovery path; m1 ships the synchronous CheckReservationActionTimeouts
// for one reservation key (tests + integration) and the interface surface
// to wire the loop in a follow-up PR.
func (ratw *ReservationActionTimeoutWatcher) Run() error {
	if ratw.notifier == nil {
		return fmt.Errorf(
			"action-timeout watcher requires a non-nil notifier",
		)
	}
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
	// The loop is owned by the integration step; the watcher itself
	// exposes the synchronous CheckReservationActionTimeouts entry-point
	// for tests and one-shot invocations.
	return nil
}

// CheckReservationActionTimeouts inspects the action generations of a
// single reservation and notifies the Bridge of any pending action whose
// TimeoutAt has elapsed. The caller controls the iteration; the watcher
// does not background-loop on its own.
//
// Parameters:
//
//   - reservationKey: the reservation identifier used by the Bridge's
//     ReservationRouter.
//   - now: a UNIX timestamp used to compare against TimeoutAt. Tests pass
//     an explicit value; production passes time.Now().Unix() cast to uint32.
//
// The function resolves the custodying wallet once, looks up the operator
// member IDs through the injected resolver, then walks the nonce axis
// starting from 0 and stopping at the first non-pending action. Walking
// until the first non-pending action models the on-chain invariant that
// nonces are sequential: only the most-recent pending action can time
// out, since older nonces have already been settled or superseded.
func (ratw *ReservationActionTimeoutWatcher) CheckReservationActionTimeouts(
	reservationKey *big.Int,
	now uint32,
) error {
	if ratw.notifier == nil {
		return fmt.Errorf(
			"action-timeout watcher requires a non-nil notifier",
		)
	}
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

	walletPublicKeyHash := reservation.WalletPublicKeyHash
	if walletPublicKeyHash == ([20]byte{}) {
		// Reservation exists but has no wallet assigned (e.g. the
		// acceptance has not progressed yet). Without a wallet we cannot
		// resolve members, so the watcher skips silently: the stranding
		// watcher will eventually catch this case.
		logger.Debugf(
			"reservation [%v] has no wallet assigned; "+
				"action-timeout watcher skipping",
			reservationKey,
		)
		return nil
	}

	// Resolve the wallet members exactly once per Check call: the Bridge
	// requires the member IDs to be consistent across all notifications
	// issued in response to a single reservation.
	memberIDs, err := ratw.membersResolver.ResolveWalletMembers(
		walletPublicKeyHash,
	)
	if err != nil {
		return fmt.Errorf(
			"failed to resolve wallet member IDs for "+
				"wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	// Walk the nonce axis from 0 upward. Stop at the first non-pending
	// action: by the Bridge invariant, only the most-recent action can be
	// in Pending state; older actions are Settled/TimedOut/Superseded/Vetoed.
	for nonce := uint64(0); nonce <= reservation.RequestNonce; nonce++ {
		action, err := ratw.spvChain.GetReservationAction(reservationKey, nonce)
		if err != nil {
			// A missing action record for a nonce in [0, RequestNonce]
			// is a Bridge-data inconsistency; we log and stop walking
			// rather than notify on partial information.
			logger.Errorf(
				"failed to load action for reservation [%v] at nonce %d: [%v]; "+
					"stopping nonce walk",
				reservationKey,
				nonce,
				err,
			)
			return nil
		}

		if action.State != tbtc.ReservationActionStatePending {
			logger.Debugf(
				"reservation [%v] action nonce %d state=%s; "+
					"stopping nonce walk at first non-pending action",
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
			// Continue the walk in case multiple actions are pending past
			// their timeouts; in practice this should not happen because
			// RequestNonce points at the latest pending nonce, but the
			// walker is defensive.
			continue
		}

		if err := ratw.notifier.NotifyReservationActionTimeout(
			reservationKey,
			memberIDs,
		); err != nil {
			logger.Errorf(
				"failed to notify action timeout for "+
					"reservation [%v] nonce %d: [%v]",
				reservationKey,
				nonce,
				err,
			)
			// Continue with the next nonce despite the error: a single
			// failure must not starve subsequent notifications.
			continue
		}

		logger.Infof(
			"notified action timeout for reservation [%v] nonce %d "+
				"(timeout=%d, members=%d)",
			reservationKey,
			nonce,
			action.TimeoutAt,
			len(memberIDs),
		)
	}

	return nil
}
