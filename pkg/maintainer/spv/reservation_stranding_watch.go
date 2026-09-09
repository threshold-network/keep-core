package spv

import (
	"fmt"
	"math/big"
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

// reservationStrandingRetryDelay is the default fixed pause between retry
// attempts, giving a transient RPC failure time to clear before the next
// attempt. Tests override the watcher's retryDelay field directly to keep
// the suite fast.
const reservationStrandingRetryDelay = 2 * time.Second

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
		retryDelay: reservationStrandingRetryDelay,
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
// Bridge for every reservation whose state is Active.
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
func (rsw *reservationStrandingWatcher) checkReservationStrandingForWallet(
	walletPublicKeyHash [20]byte,
) error {
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
		return fmt.Errorf(
			"failed to fetch reservations for wallet [0x%x] after %d attempts: [%v]",
			walletPublicKeyHash,
			reservationStrandingRetryAttempts,
			err,
		)
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
					"skipping - its stranded state will not be reconciled "+
					"until the next triggering event",
				key,
				reservationStrandingRetryAttempts,
				err,
			)
			continue
		}

		// A reservation with a pending action generation must be left for the
		// action-timeout watcher. Marking it stranded would preempt a healthy
		// settlement path and trigger gratuitous reconciliation cost for the
		// owner.
		if reservation.State != tbtc.ReservationStateActive {
			logger.Debugf(
				"reservation [%v] is not Active (state: %v); "+
					"deferring stray notification to action-timeout watcher",
				key,
				reservation.State,
			)
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

	return nil
}
