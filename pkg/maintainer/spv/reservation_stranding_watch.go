package spv

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// reservationStrandingRetryAttempts bounds the number of attempts made
// against a single WalletReservations or GetReservation chain read before
// checkReservationStrandingForWallet gives up and logs a persistent
// failure. Mirrors the fixed 3-attempt convention the stranding startup
// scan already uses for its own GetWallet/checkReservationStrandingForWallet
// retries in reservation_wiring.go.
const reservationStrandingRetryAttempts = 3

// DefaultReservationStrandingRetryDelay is the default fixed pause between retry
// attempts, giving a transient RPC failure time to clear before the next
// attempt.
const DefaultReservationStrandingRetryDelay = 2 * time.Second

// reservationStrandingMutex synchronizes access to reservationStrandingRetryDelay.
var reservationStrandingMutex sync.RWMutex

// reservationStrandingRetryDelay is the pause between retry attempts
// newReservationStrandingWatcher sets. Defaults to
// DefaultReservationStrandingRetryDelay; declared as a var so tests in this
// package can shrink it.
var reservationStrandingRetryDelay = DefaultReservationStrandingRetryDelay

func getReservationStrandingRetryDelay() time.Duration {
	reservationStrandingMutex.RLock()
	defer reservationStrandingMutex.RUnlock()
	return reservationStrandingRetryDelay
}

func setReservationStrandingRetryDelay(d time.Duration) {
	reservationStrandingMutex.Lock()
	defer reservationStrandingMutex.Unlock()
	reservationStrandingRetryDelay = d
}

// reservationStrandingWatcher observes wallet close/termination events and
// notifies the Bridge of any reservation whose anchor is now stranded.
//
// In tBTC v2 wallets, a live reservation anchor is held in a wallet-controlled
// output. When the wallet is closed or terminated the anchor is stranded:
// the keyset can no longer sign a redemption, reanchor, or dissolution
// transaction for that reservation. The Bridge must be informed so the
// reservation can transition to ReservationStateStranded and the anchor can
// be reconciled via the owner-facing late settlement path.
type reservationStrandingWatcher struct {
	spvChain Chain
	// retryDelay is the pause between WalletReservations/GetReservation
	// retry attempts. Defaults to reservationStrandingRetryDelay; tests
	// override it directly to avoid slowing down the suite.
	retryDelay time.Duration
}

// newReservationStrandingWatcher constructs a stranding watcher bound to the
// given chain.
//
// The watcher is intended to be wired to wallet-close events via a subscription
func newReservationStrandingWatcher(spvChain Chain) *reservationStrandingWatcher {
	return &reservationStrandingWatcher{
		spvChain:   spvChain,
		retryDelay: getReservationStrandingRetryDelay(),
	}
}

// withStrandingRetry retries op up to reservationStrandingRetryAttempts
// times, pausing rsw.retryDelay between attempts, so a transient
// WalletReservations/GetReservation RPC failure does not permanently miss a
// stranding notification: the triggering OnWalletClosed event is one-shot
// and never replayed, so a single failed read here would otherwise silently
// drop the wallet's reservations from stranding coverage forever.
// description names the operation being retried for the warning log
// emitted between attempts.
func (rsw *reservationStrandingWatcher) withStrandingRetry(
	description string,
	op func() error,
) error {
	var err error
	for attempt := range reservationStrandingRetryAttempts {
		if err = op(); err == nil {
			return nil
		}
		logger.Warnf(
			"stranding check attempt %d/%d failed to %s: [%v]",
			attempt+1,
			reservationStrandingRetryAttempts,
			description,
			err,
		)
		if attempt < reservationStrandingRetryAttempts-1 {
			time.Sleep(rsw.retryDelay)
		}
	}
	return err
}

// checkReservationStrandingForWallet is the Closed/Terminated form of the
// stranding check. A closed or terminated wallet strands a reservation
// outright: the keyset can no longer sign for it, so every Active
// reservation is notified.
func (rsw *reservationStrandingWatcher) checkReservationStrandingForWallet(
	walletPublicKeyHash [20]byte,
) (recheck bool, err error) {
	return rsw.checkStranding(walletPublicKeyHash, tbtc.StateClosed)
}

// checkReservationStrandingForClosingWallet is the Closing-wallet form of
// checkReservationStrandingForWallet. A Closing wallet strands a
// reservation only once the reservation's term has passed its
// DissolutionEligibleAt (Reservation.sol), so a reservation is notified
// only when the chain head time has reached that timestamp; every other
// Active reservation is left alone and the wallet is flagged for a later
// recheck.
func (rsw *reservationStrandingWatcher) checkReservationStrandingForClosingWallet(
	walletPublicKeyHash [20]byte,
) (recheck bool, err error) {
	return rsw.checkStranding(walletPublicKeyHash, tbtc.StateClosing)
}

// checkReservationStrandingForLiveWallet is the Live-wallet form of
// checkReservationStrandingForWallet. A Live wallet still holds its keys,
// so the owner can still act on the reservation; it strands a reservation
// only once the reservation's term has passed its DissolutionEligibleAt
// (Reservation.sol). A Live wallet whose reservation is past that
// timestamp is notified, and every other Active reservation is left alone
// and the wallet is flagged for a later recheck.
//
// The Live branch is a release path, not a loss of funds: the reservation's
// anchor stays in the wallet's custody, but milestone 1 has no other way
// to release the position, so an expired Live position would pin the
// wallet's and the global reservation capacity forever.
func (rsw *reservationStrandingWatcher) checkReservationStrandingForLiveWallet(
	walletPublicKeyHash [20]byte,
) (recheck bool, err error) {
	return rsw.checkStranding(walletPublicKeyHash, tbtc.StateLive)
}

// checkStrandingForWalletState dispatches to the stranding check that
// matches the wallet's on-chain state. checked is false for states that
// cannot strand a reservation (MovingFunds and the like); nothing is read
// then.
//
// The four states the Bridge accepts in notifyReservationStranded are
// covered: Closed and Terminated strand outright, Live and Closing strand
// only once the reservation's term has passed its DissolutionEligibleAt,
// and Closing additionally waits out the wallet's closing period.
func (rsw *reservationStrandingWatcher) checkStrandingForWalletState(
	walletPublicKeyHash [20]byte,
	state tbtc.WalletState,
) (checked bool, recheck bool, err error) {
	switch state {
	case tbtc.StateClosed, tbtc.StateTerminated:
		recheck, err = rsw.checkReservationStrandingForWallet(walletPublicKeyHash)
		return true, recheck, err
	case tbtc.StateClosing:
		recheck, err = rsw.checkReservationStrandingForClosingWallet(walletPublicKeyHash)
		return true, recheck, err
	case tbtc.StateLive:
		recheck, err = rsw.checkReservationStrandingForLiveWallet(walletPublicKeyHash)
		return true, recheck, err
	default:
		return false, false, nil
	}
}

// checkStranding walks the reservations currently custodied by
// walletPublicKeyHash and notifies the Bridge of every reservation the
// wallet's state and the reservation's term allow to be stranded.
//
// walletState is the custodying wallet's on-chain state. It selects the
// gate the reservation must pass before the client notifies the Bridge:
//
//   - Closed and Terminated: no gate. The keyset is gone.
//   - Live: the chain head time must have reached the reservation's
//     DissolutionEligibleAt.
//   - Closing: the chain head time must have reached the reservation's
//     DissolutionEligibleAt AND the wallet's closing period must have
//     elapsed since closingStartedAt.
//
// The gates mirror the Bridge's own require in notifyReservationStranded
// (Reservation.sol). A reservation that fails its gate is left alone and
// the wallet is flagged for a later recheck, so the client never submits
// a notification the Bridge would revert.
func (rsw *reservationStrandingWatcher) checkStranding(
	walletPublicKeyHash [20]byte,
	walletState tbtc.WalletState,
) (bool, error) {
	var keys []*big.Int
	err := rsw.withStrandingRetry(
		fmt.Sprintf("fetch reservations for wallet [0x%x]", walletPublicKeyHash),
		func() error {
			var opErr error
			keys, opErr = rsw.spvChain.WalletReservations(walletPublicKeyHash)
			return opErr
		},
	)
	if err != nil {
		return false, fmt.Errorf(
			"failed to fetch reservations for wallet [0x%x] after %d attempts: [%v]",
			walletPublicKeyHash,
			reservationStrandingRetryAttempts,
			err,
		)
	}

	// A wallet with zero reservations has nothing for the loop below to
	// notify, so skip WalletTerminationCause entirely: it costs 3
	// genesis-to-tip eth_getLogs scans on the underlying chain and its
	// result is only ever read inside that loop. Without this guard,
	// every wallet-close event pays that cost even when the wallet
	// never held a reservation.
	if len(keys) == 0 {
		return false, nil
	}

	var headTime time.Time
	// headNow mirrors headTime as the uint32 UNIX timestamp the on-chain
	// deadlines use, bounded so it serves the comparisons below in the same
	// form as DissolutionEligibleAt (see tbtc.UnixSecondsToUint32).
	var headNow uint32
	// requireDissolutionEligible is true for the states whose gate the
	// Bridge gates on the reservation's DissolutionEligibleAt: Live and
	// Closing. Closed and Terminated need no chain head read at all.
	requireDissolutionEligible := walletState == tbtc.StateLive ||
		walletState == tbtc.StateClosing
	if requireDissolutionEligible {
		headTime, err = rsw.spvChain.GetLatestBlockTimestamp()
		if err != nil {
			return false, fmt.Errorf(
				"failed to read the chain head time for wallet [0x%x]: [%v]",
				walletPublicKeyHash,
				err,
			)
		}
		headNow = tbtc.UnixSecondsToUint32(headTime.Unix())
	}

	// closingPeriodEnd is the timestamp at which the wallet's closing
	// period ends: closingStartedAt + the Bridge's wallet closing period.
	// The Bridge's Closing branch requires the chain head time to have
	// reached it, so the client must not notify before it either.
	//
	// The sum is computed in uint64 because the Bridge computes it in
	// uint256: a uint32 sum would wrap and produce a closing-period end
	// in the past, which would let the client notify early.
	var closingPeriodEnd uint64
	if walletState == tbtc.StateClosing {
		wallet, walletErr := rsw.spvChain.GetWallet(walletPublicKeyHash)
		if walletErr != nil {
			return false, fmt.Errorf(
				"failed to fetch wallet [0x%x] for its closing period: [%v]",
				walletPublicKeyHash,
				walletErr,
			)
		}
		walletParameters, parametersErr := rsw.spvChain.GetWalletParameters()
		if parametersErr != nil {
			return false, fmt.Errorf(
				"failed to fetch the wallet parameters for wallet [0x%x]'s "+
					"closing period: [%v]",
				walletPublicKeyHash,
				parametersErr,
			)
		}
		closingPeriodEnd = uint64(
			tbtc.UnixSecondsToUint32(wallet.ClosingStartedAt.Unix()),
		) + uint64(walletParameters.ClosingPeriod)
	}

	// Best-effort: the termination cause is operational context for the
	// notifications below, not a correctness gate. A failure here must not
	// block stranding notifications, so it is logged and treated as
	// Unknown rather than propagated.
	//
	// A Live wallet has no termination cause: the lookup costs three
	// genesis-to-tip eth_getLogs scans and can only ever return Unknown.
	// It is skipped there so the Live branch stays affordable.
	var cause tbtc.WalletTerminationCause
	if walletState != tbtc.StateLive {
		cause, err = rsw.spvChain.WalletTerminationCause(walletPublicKeyHash)
		if err != nil {
			logger.Warnf(
				"failed to determine termination cause for wallet [0x%x]: [%v]; "+
					"proceeding with cause Unknown",
				walletPublicKeyHash,
				err,
			)
			cause = tbtc.WalletTerminationCauseUnknown
		}
	} else {
		cause = tbtc.WalletTerminationCauseUnknown
	}

	recheck := false

	for _, key := range keys {
		if key == nil {
			continue
		}

		var reservation *tbtc.Reservation
		err := rsw.withStrandingRetry(
			fmt.Sprintf("fetch reservation [%v]", key),
			func() error {
				var opErr error
				reservation, opErr = rsw.spvChain.GetReservation(key)
				return opErr
			},
		)
		if err != nil {
			logger.Errorf(
				"failed to fetch reservation [%v] after %d attempts: [%v]; "+
					"skipping it and flagging wallet [0x%x] for a later "+
					"stranding pass",
				key,
				reservationStrandingRetryAttempts,
				err,
				walletPublicKeyHash,
			)
			recheck = true
			continue
		}

		// A reservation with a pending action generation must be left for the
		// action-timeout watcher. Marking it stranded would preempt a healthy
		// settlement path and trigger gratuitous reconciliation cost for the
		// owner. It can return to Active when the action times out, so the
		// wallet is flagged for another pass.
		if reservation.State == tbtc.ReservationStateActionPending {
			logger.Debugf(
				"reservation [%v] has a pending action; "+
					"deferring stray notification to action-timeout watcher",
				key,
			)
			recheck = true
			continue
		}
		if reservation.State != tbtc.ReservationStateActive {
			logger.Debugf(
				"reservation [%v] is not Active (state: %v); "+
					"nothing to notify",
				key,
				reservation.State,
			)
			continue
		}

		if requireDissolutionEligible &&
			(reservation.DissolutionEligibleAt == 0 ||
				uint64(headNow) < uint64(reservation.DissolutionEligibleAt)) {
			logger.Debugf(
				"reservation [%v] on %v wallet [0x%x] is not yet "+
					"dissolution-eligible; flagging the wallet for a later pass",
				key,
				walletState,
				walletPublicKeyHash,
			)
			recheck = true
			continue
		}

		// The Bridge's Closing branch additionally requires the wallet's
		// closing period to have elapsed since closingStartedAt. Without
		// this term the client would notify a Closing wallet's
		// dissolution-eligible reservation before the wallet's closing
		// period ended, and the Bridge would revert the notification.
		// The closing period is an exclusive re-anchor window: a wallet
		// pushed Live -> MovingFunds -> Closing by permissionless calls
		// must get one full closing period to move its position before
		// anyone may strand it.
		if walletState == tbtc.StateClosing &&
			uint64(headNow) < closingPeriodEnd {
			logger.Debugf(
				"reservation [%v] on Closing wallet [0x%x] is not yet past "+
					"the wallet's closing period (ends at %d, head time %d); "+
					"flagging the wallet for a later pass",
				key,
				walletPublicKeyHash,
				closingPeriodEnd,
				headNow,
			)
			recheck = true
			continue
		}

		if err := rsw.spvChain.NotifyReservationStranded(key); err != nil {
			logger.Errorf(
				"failed to notify stranded reservation [%v]: [%v]",
				key,
				err,
			)
			// Continue with the remaining reservations: a single failure
			// must not starve the others.
			continue
		}

		logger.Infof(
			"notified stranded reservation [%v] (wallet [0x%x] termination "+
				"cause: %v)",
			key,
			walletPublicKeyHash,
			cause,
		)
	}

	return recheck, nil
}
