package spv

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"

	"github.com/go-test/deep"
)

// reservationKey constructs an in-memory reservation key with the given low
// 16 bytes set. The watcher only consults `GetReservation` via the local
// chain's [16]byte map key, so compact test keys avoid accidental collisions
// across test cases.
func reservationKey(low uint64) *big.Int {
	return new(big.Int).SetUint64(low)
}

// walletPKH is a deterministic wallet public key hash used in the stranding
// tests. Tests that need a different wallet use walletPKHAt.
func walletPKH() [20]byte {
	var out [20]byte
	out[19] = 0x42
	return out
}

// walletPKHAt returns a wallet PKH with the trailing byte set to byte b. It
// exists to make multi-wallet tests readable.
func walletPKHAt(b byte) [20]byte {
	var out [20]byte
	out[19] = b
	return out
}

func TestReservationStrandingWatcher_NoReservations(t *testing.T) {
	spvChain := newLocalChain()

	watcher := newReservationStrandingWatcher(spvChain)
	if watcher == nil {
		t.Fatal("expected non-nil watcher")
	}

	if err := watcher.checkReservationStrandingForWallet(walletPKH()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationStrandedKeys(); len(calls) != 0 {
		t.Fatalf(
			"expected no notifications, got %d",
			len(calls),
		)
	}
}

func TestReservationStrandingWatcher_NotifiesActiveReservation(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xAA01)

	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := spvChain.getSubmittedReservationStrandedKeys()
	if len(calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(calls))
	}
	if diff := deep.Equal(key, calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

func TestReservationStrandingWatcher_SkipsClosedReservation(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xAA02)

	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateClosed,
	})

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationStrandedKeys(); len(calls) != 0 {
		t.Fatalf("expected no notifications, got %d", len(calls))
	}
}

func TestReservationStrandingWatcher_SkipsPendingReservation(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xAA03)

	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActionPending,
	})

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getSubmittedReservationStrandedKeys(); len(calls) != 0 {
		t.Fatalf(
			"expected pending reservation to defer to action-timeout, "+
				"got %d notifications",
			len(calls),
		)
	}
}

func TestReservationStrandingWatcher_MultipleReservations(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	active := reservationKey(0xAA10)
	closed := reservationKey(0xAA11)
	pending := reservationKey(0xAA12)
	stranded := reservationKey(0xAA13)

	spvChain.setWalletReservations(
		wallet,
		[]*big.Int{active, closed, pending, stranded},
	)
	spvChain.setReservation(active, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})
	spvChain.setReservation(closed, &tbtc.Reservation{
		State: tbtc.ReservationStateClosed,
	})
	spvChain.setReservation(pending, &tbtc.Reservation{
		State: tbtc.ReservationStateActionPending,
	})
	spvChain.setReservation(stranded, &tbtc.Reservation{
		State: tbtc.ReservationStateStranded,
	})

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The watcher must notify only for reservations in the Active state
	// (finding #27's allow-list fix): closed, pending, and stranded
	// reservations must all be skipped - a stranded reservation has
	// already been notified once and re-notifying it is redundant, and a
	// closed reservation was already resolved through in-kind redemption
	// or another terminal path, not stranding.
	calls := spvChain.getSubmittedReservationStrandedKeys()
	if len(calls) != 1 {
		t.Fatalf(
			"expected exactly one notification (active only), "+
				"got %d: %v",
			len(calls),
			calls,
		)
	}
	if diff := deep.Equal(active, calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

func TestReservationStrandingWatcher_UnknownReservationIsSkipped(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	staleKey := reservationKey(0xAA20)
	freshKey := reservationKey(0xAA21)

	// walletReservations references staleKey but the chain has no record of
	// it. freshKey is properly recorded.
	spvChain.setWalletReservations(
		wallet,
		[]*big.Int{staleKey, freshKey},
	)
	spvChain.setReservation(freshKey, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := spvChain.getSubmittedReservationStrandedKeys()
	if len(calls) != 1 {
		t.Fatalf("expected one notification (freshKey), got %d", len(calls))
	}
	if diff := deep.Equal(freshKey, calls[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

func TestReservationStrandingWatcher_WalletChainError(t *testing.T) {
	spvChain := newLocalChain()

	// No walletReservations entry: WalletReservations returns (nil, nil) for
	// unknown wallets; the watcher iterates over a nil slice and exits
	// cleanly, so we expect no error here. Reserve the case for wallets that
	// have an entry which the chain then refuses to enumerate.
	wallet := walletPKH()
	spvChain.setWalletReservations(wallet, nil)

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error for empty wallet: %v", err)
	}

	if calls := spvChain.getSubmittedReservationStrandedKeys(); len(calls) != 0 {
		t.Fatalf(
			"expected zero notifications on empty wallet list, got %d",
			len(calls),
		)
	}
}

// TestReservationStrandingWatcher_NotifierErrorContinuesProcessing mirrors
// TestReservationActionTimeoutWatcher_NotifiesOncePerQualifyingNonce's
// resilience property for the sibling action-timeout watcher: a single
// NotifyReservationStranded failure must not starve the remaining
// reservations in the same wallet.
func TestReservationStrandingWatcher_NotifierErrorContinuesProcessing(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	failing := reservationKey(0xAA40)
	succeeding := reservationKey(0xAA41)

	spvChain.setWalletReservations(wallet, []*big.Int{failing, succeeding})
	spvChain.setReservation(failing, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})
	spvChain.setReservation(succeeding, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})
	spvChain.notifyReservationStrandedErrByKey = map[string]error{
		failing.String(): fmt.Errorf("notifier unavailable"),
	}

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf(
			"a single notifier failure must not fail the whole check: %v",
			err,
		)
	}

	notified := spvChain.getSubmittedReservationStrandedKeys()
	if len(notified) != 1 {
		t.Fatalf(
			"expected the remaining reservation to still be notified, got %d",
			len(notified),
		)
	}
	if diff := deep.Equal(succeeding, notified[0]); diff != nil {
		t.Errorf("unexpected notified key: %v", diff)
	}
}

// TestReservationStrandingWatcher_WalletReservationsChainError covers the
// case TestReservationStrandingWatcher_WalletChainError's comment
// explicitly calls out as unreserved: WalletReservations itself failing
// (as opposed to returning an empty list for an unknown wallet).
func TestReservationStrandingWatcher_WalletReservationsChainError(t *testing.T) {
	spvChain := newLocalChain()

	spvChain.walletReservationsErr = fmt.Errorf("rpc unavailable")

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(walletPKH()); err == nil {
		t.Fatal("expected error when WalletReservations fails, got nil")
	}

	if calls := spvChain.getSubmittedReservationStrandedKeys(); len(calls) != 0 {
		t.Fatalf(
			"expected no notifications on chain error, got %d",
			len(calls),
		)
	}
}
