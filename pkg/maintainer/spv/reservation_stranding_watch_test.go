package spv

import (
	"fmt"
	"math/big"
	"testing"
	"time"

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

	// The watcher must notify only for reservations in the Active state:
	// closed, pending, and stranded reservations must all be skipped - a
	// stranded reservation has already been notified once and
	// re-notifying it is redundant, and a closed reservation was already
	// resolved through in-kind redemption or another terminal path, not
	// stranding.
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
	watcher.retryDelay = time.Millisecond
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
	watcher.retryDelay = time.Millisecond
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

// transientErrChain wraps a Chain and fails the first N calls to a wrapped
// method before delegating to the embedded Chain, so the stranding
// watcher's retry behavior (finding P1-1: bounded retry on a transient
// WalletReservations/GetReservation chain-read failure) can be exercised
// without extending the shared localChain fake in chain_test.go.
type transientErrChain struct {
	Chain
	walletReservationsFailuresLeft int
	getReservationFailuresLeft     int
	walletReservationsCalls        int
	getReservationCalls            int
}

func (c *transientErrChain) WalletReservations(
	walletPublicKeyHash [20]byte,
) ([]*big.Int, error) {
	c.walletReservationsCalls++
	if c.walletReservationsFailuresLeft > 0 {
		c.walletReservationsFailuresLeft--
		return nil, fmt.Errorf("transient rpc failure")
	}
	return c.Chain.WalletReservations(walletPublicKeyHash)
}

func (c *transientErrChain) GetReservation(
	reservationKey *big.Int,
) (*tbtc.Reservation, error) {
	c.getReservationCalls++
	if c.getReservationFailuresLeft > 0 {
		c.getReservationFailuresLeft--
		return nil, fmt.Errorf("transient rpc failure")
	}
	return c.Chain.GetReservation(reservationKey)
}

// TestReservationStrandingWatcher_WalletReservationsTransientErrorRetries
// verifies finding P1-1: a WalletReservations failure that clears within
// reservationStrandingRetryAttempts attempts must not be treated as
// permanent - the caller-visible OnWalletClosed trigger is one-shot and
// never replayed, so giving up after a single transient RPC hiccup would
// silently and permanently drop the wallet's reservations from stranding
// coverage.
func TestReservationStrandingWatcher_WalletReservationsTransientErrorRetries(t *testing.T) {
	spvChain := newLocalChain()
	wallet := walletPKH()
	key := reservationKey(0xAA60)
	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	wrapped := &transientErrChain{
		Chain:                          spvChain,
		walletReservationsFailuresLeft: 2,
	}

	watcher := newReservationStrandingWatcher(wrapped)
	watcher.retryDelay = time.Millisecond
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("expected the retry to eventually succeed, got error: %v", err)
	}

	if wrapped.walletReservationsCalls != 3 {
		t.Fatalf(
			"expected 3 WalletReservations attempts (2 failures + 1 success), got %d",
			wrapped.walletReservationsCalls,
		)
	}

	if notified := spvChain.getSubmittedReservationStrandedKeys(); len(notified) != 1 {
		t.Fatalf(
			"expected the reservation to still be notified after the retry succeeded, got %d",
			len(notified),
		)
	}
}

// TestReservationStrandingWatcher_WalletReservationsExhaustsRetriesAndFails
// verifies the bound on finding P1-1's retry: after
// reservationStrandingRetryAttempts consecutive failures the watcher gives
// up and returns an error, rather than retrying forever.
func TestReservationStrandingWatcher_WalletReservationsExhaustsRetriesAndFails(t *testing.T) {
	spvChain := newLocalChain()
	wallet := walletPKH()

	wrapped := &transientErrChain{
		Chain:                          spvChain,
		walletReservationsFailuresLeft: reservationStrandingRetryAttempts,
	}

	watcher := newReservationStrandingWatcher(wrapped)
	watcher.retryDelay = time.Millisecond
	if err := watcher.checkReservationStrandingForWallet(wallet); err == nil {
		t.Fatal("expected an error after exhausting all retry attempts")
	}

	if wrapped.walletReservationsCalls != reservationStrandingRetryAttempts {
		t.Fatalf(
			"expected exactly %d attempts, got %d",
			reservationStrandingRetryAttempts,
			wrapped.walletReservationsCalls,
		)
	}
}

// TestReservationStrandingWatcher_GetReservationTransientErrorRetries is the
// GetReservation analog of
// TestReservationStrandingWatcher_WalletReservationsTransientErrorRetries:
// a transient per-reservation read failure must also be retried rather
// than immediately skipping the reservation.
func TestReservationStrandingWatcher_GetReservationTransientErrorRetries(t *testing.T) {
	spvChain := newLocalChain()
	wallet := walletPKH()
	key := reservationKey(0xAA61)
	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})

	wrapped := &transientErrChain{
		Chain:                      spvChain,
		getReservationFailuresLeft: 2,
	}

	watcher := newReservationStrandingWatcher(wrapped)
	watcher.retryDelay = time.Millisecond
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wrapped.getReservationCalls != 3 {
		t.Fatalf(
			"expected 3 GetReservation attempts (2 failures + 1 success), got %d",
			wrapped.getReservationCalls,
		)
	}

	if notified := spvChain.getSubmittedReservationStrandedKeys(); len(notified) != 1 {
		t.Fatalf(
			"expected the reservation to still be notified after the retry succeeded, got %d",
			len(notified),
		)
	}
}

func TestReservationStrandingWatcher_CapturesTerminationCause(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xAA10)

	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})
	spvChain.setWalletTerminationCause(wallet, tbtc.WalletTerminationCauseFraudChallengeDefeat)

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls := spvChain.getWalletTerminationCauseCallCount(); calls != 1 {
		t.Fatalf("expected exactly one WalletTerminationCause call, got %d", calls)
	}

	if notified := spvChain.getSubmittedReservationStrandedKeys(); len(notified) != 1 {
		t.Fatalf("expected one notification, got %d", len(notified))
	}
}

// TestReservationStrandingWatcher_CauseLookupErrorDoesNotBlockNotification
// proves the cause lookup is best-effort observability, not a correctness
// gate: a failure to determine the termination cause must not prevent the
// reservation from being notified stranded.
func TestReservationStrandingWatcher_CauseLookupErrorDoesNotBlockNotification(t *testing.T) {
	spvChain := newLocalChain()

	wallet := walletPKH()
	key := reservationKey(0xAA11)

	spvChain.setWalletReservations(wallet, []*big.Int{key})
	spvChain.setReservation(key, &tbtc.Reservation{
		State: tbtc.ReservationStateActive,
	})
	spvChain.walletTerminationCauseErr = fmt.Errorf("rpc unavailable")

	watcher := newReservationStrandingWatcher(spvChain)
	if err := watcher.checkReservationStrandingForWallet(wallet); err != nil {
		t.Fatalf("cause lookup failure must not fail the whole check: %v", err)
	}

	if notified := spvChain.getSubmittedReservationStrandedKeys(); len(notified) != 1 {
		t.Fatalf(
			"expected the reservation to still be notified despite the cause "+
				"lookup failure, got %d",
			len(notified),
		)
	}
}
