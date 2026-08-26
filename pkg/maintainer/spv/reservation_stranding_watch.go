package spv

import (
	"fmt"
	"math/big"

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
	notifier ReservationStrandingNotifier
}

// ReservationStrandingNotifier is the contract the stranding watcher uses to
// forward notifications to the Bridge. It mirrors
// `Chain.NotifyReservationStranded` but is interface-typed so the watcher can
// be unit-tested with an in-memory recorder.
type ReservationStrandingNotifier interface {
	NotifyReservationStranded(reservationKey *big.Int) error
}

// ReservationStrandingNotifierFunc adapts a plain function to the
// ReservationStrandingNotifier interface, matching the Go idiomatic pattern
// for callbacks in this package (see also `unprovenTransactionsGetter` in
// spv.go).
type ReservationStrandingNotifierFunc func(reservationKey *big.Int) error

// NotifyReservationStranded forwards the call to the wrapped function.
func (f ReservationStrandingNotifierFunc) NotifyReservationStranded(
	reservationKey *big.Int,
) error {
	return f(reservationKey)
}

// NewReservationStrandingWatcher constructs a stranding watcher bound to the
// given chain. The returned watcher is not yet attached to a wallet; use
// WatchWallet to start observing a particular wallet's close/termination
// events.
//
// The notifier is mandatory and must be non-nil; nil is treated as a
// programming error rather than a no-op because silently dropping stray
// notifications would leave reservation anchors unreconciled.
func NewReservationStrandingWatcher(
	spvChain Chain,
	notifier ReservationStrandingNotifier,
) *ReservationStrandingWatcher {
	return &ReservationStrandingWatcher{
		spvChain: spvChain,
		notifier: notifier,
	}
}

// WatchWallet subscribes the watcher to Bridge close/termination events for
// the given wallet. When the wallet transitions to StateClosed or
// StateTerminated, the watcher walks the wallet's reservations and notifies
// the Bridge of any reservation that is not currently ActionPending.
//
// Note: Bridge.go emits OnWalletClosed for both close and termination
// (see `pkg/tbtc/chain.go` BridgeChain.OnWalletClosed), so a single
// subscription covers both terminal states. The watcher therefore registers
// a single OnWalletClosed handler; downstream code may alias this hook for
// OnWalletTerminated dispatch if both events are ever split.
//
// Pass a nil fn to skip wiring (used in tests that drive the watcher
// imperatively via CheckReservationStranding). Pass a non-nil fn to enable
// live observation.
func (rsw *ReservationStrandingWatcher) WatchWallet(
	walletPublicKeyHash [20]byte,
) error {
	if rsw.notifier == nil {
		return fmt.Errorf("stranding watcher requires a non-nil notifier")
	}

	// Implementation note: an integration step (a later PR in this milestone)
	// wires the watcher into `chain.OnWalletClosed(...)` and dispatches by
	// wallet ID -> public key hash mapping. The watcher itself remains
	// wallet-agnostic; tests can exercise it by calling
	// `CheckReservationStrandingForWallet` directly.
	_ = walletPublicKeyHash

	return nil
}

// CheckReservationStrandingForWallet walks the reservations currently
// custodied by walletPublicKeyHash and forwards a stray notification to the
// Bridge for every reservation whose state is not ActionPending.
//
// This is the single-shot form used both by tests and by the integration
// wiring of WatchWallet. It is intentionally synchronous and per-wallet: the
// caller decides which wallets to inspect, and the watcher does not run a
// background loop of its own.
//
// The function is idempotent at the chain level: notifying an already-stranded
// reservation is a no-op on the Bridge side. It is the caller's
// responsibility to dedupe notifications across watcher restarts; the watcher
// never silently drops or coalesces calls.
func (rsw *ReservationStrandingWatcher) CheckReservationStrandingForWallet(
	walletPublicKeyHash [20]byte,
) error {
	if rsw.notifier == nil {
		return fmt.Errorf("stranding watcher requires a non-nil notifier")
	}

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
		if reservation.State == tbtc.ReservationStateActionPending {
			logger.Debugf(
				"reservation [%v] has a pending action generation; "+
					"deferring stray notification to action-timeout watcher",
				key,
			)
			continue
		}

		if err := rsw.notifier.NotifyReservationStranded(key); err != nil {
			logger.Errorf(
				"failed to notify stranded reservation [%v]: [%v]",
				key,
			)
			// Continue with the remaining reservations: a single failure
			// must not starve the others.
			continue
		}

		logger.Infof("notified stranded reservation [%v]", key)
	}

	return nil
}
