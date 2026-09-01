package spv

import (
	"fmt"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// ReservationStrandingWatcher observes wallet close/termination events and
// notifies the Bridge of any reservation whose anchor is now stranded.
//
// In tBTC v2 wallets, a live reservation anchor is held in a wallet-controlled
// output. When the wallet is closed or terminated the anchor is stranded:
// the keyset can no longer sign a redemption, reanchor, or dissolution
// transaction for that reservation. The Bridge must be informed so the
// reservation can transition to ReservationStateStranded and the anchor can
// be reconciled via the owner-facing late settlement path.
type ReservationStrandingWatcher struct {
	spvChain Chain
}

// NewReservationStrandingWatcher constructs a stranding watcher bound to the
// given chain.
//
// The watcher is intended to be wired to wallet-close events via a subscription
func NewReservationStrandingWatcher(spvChain Chain) *ReservationStrandingWatcher {
	return &ReservationStrandingWatcher{
		spvChain: spvChain,
	}
}

// CheckReservationStrandingForWallet walks the reservations currently
// custodied by walletPublicKeyHash and forwards a stray notification to the
// Bridge for every reservation whose state is Active.
//
// This is the single-shot form used both by tests and by the integration
// wiring that subscribes to wallet close/termination events. It is
// intentionally synchronous and per-wallet: the caller decides which wallets
// to inspect, and the watcher does not run a background loop of its own.

// The function is idempotent at the chain level: notifying an already-stranded
// reservation is a no-op on the Bridge side. It is the caller's
// responsibility to dedupe notifications across watcher restarts; the watcher
// never silently drops or coalesces calls.
func (rsw *ReservationStrandingWatcher) CheckReservationStrandingForWallet(
	walletPublicKeyHash [20]byte,
) error {
	keys, err := rsw.spvChain.WalletReservations(walletPublicKeyHash)
	if err != nil {
		return fmt.Errorf(
			"failed to fetch reservations for wallet [0x%x]: [%v]",
			walletPublicKeyHash,
			err,
		)
	}

	for _, key := range keys {
		if key == nil {
			continue
		}

		reservation, err := rsw.spvChain.GetReservation(key)
		if err != nil {
			logger.Errorf(
				"failed to fetch reservation [%v]: [%v]; skipping",
				key,
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

		logger.Infof("notified stranded reservation [%v]", key)
	}

	return nil
}
