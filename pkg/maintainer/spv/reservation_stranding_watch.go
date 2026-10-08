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

// checkReservationStrandingForWallet walks the reservations currently
// custodied by walletPublicKeyHash and forwards a stranding notification to the
// Bridge for every reservation whose state is Active. It is meant for wallets
// that are already Closed or Terminated.
//
// This is the single-shot form used both by tests and by the integration
// wiring that subscribes to wallet close/termination events. It is
// intentionally synchronous and per-wallet: the caller decides which wallets
// to inspect, and the watcher does not run a background loop of its own.
//
// The function is idempotent at the chain level: notifying an already-stranded
// reservation is a no-op on the Bridge side. It is the caller's
// responsibility to dedupe notifications across watcher restarts; the watcher
// never silently drops or coalesces calls.
//
// The returned recheck flag is true when this pass could not settle the
// wallet's outcome and a later pass may still find work: a reservation
// with a pending action generation was skipped (it may return to Active
// once the action times out), or a reservation read failed after retries.
// The error is non-nil only when the wallet's reservations could not be
// listed.
func (rsw *reservationStrandingWatcher) checkReservationStrandingForWallet(
	walletPublicKeyHash [20]byte,
) (recheck bool, err error) {
	return rsw.checkStranding(walletPublicKeyHash, false)
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
	return rsw.checkStranding(walletPublicKeyHash, true)
}

// checkStrandingForWalletState dispatches to the stranding check that
// matches the wallet's on-chain state. checked is false for states that
// cannot strand a reservation (Live and the like); nothing is read then.
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
	default:
		return false, false, nil
	}
}

func (rsw *reservationStrandingWatcher) checkStranding(
	walletPublicKeyHash [20]byte,
	requireDissolutionEligible bool,
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
	if requireDissolutionEligible {
		headTime, err = rsw.spvChain.GetLatestBlockTimestamp()
		if err != nil {
			return false, fmt.Errorf(
				"failed to read the chain head time for wallet [0x%x]: [%v]",
				walletPublicKeyHash,
				err,
			)
		}
	}

	// Best-effort: the termination cause is operational context for the
	// notifications below, not a correctness gate. A failure here must not
	// block stranding notifications, so it is logged and treated as
	// Unknown rather than propagated.
	cause, causeErr := rsw.spvChain.WalletTerminationCause(walletPublicKeyHash)
	if causeErr != nil {
		logger.Warnf(
			"failed to determine termination cause for wallet [0x%x]: [%v]; "+
				"proceeding with cause Unknown",
			walletPublicKeyHash,
			causeErr,
		)
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
				uint64(headTime.Unix()) < uint64(reservation.DissolutionEligibleAt)) {
			logger.Debugf(
				"reservation [%v] on Closing wallet [0x%x] is not yet "+
					"dissolution-eligible; flagging the wallet for a later pass",
				key,
				walletPublicKeyHash,
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
