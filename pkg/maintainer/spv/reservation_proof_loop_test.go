package spv

import (
	"math/big"
	"testing"

	"github.com/keep-network/keep-core/pkg/tbtc"
)

// TestVerifyReservationActionStillProvable_Pending verifies the happy path:
// the action generation is still pending, still the expected type, and
// still targets the expected wallet, so submission may proceed.
func TestVerifyReservationActionStillProvable_Pending(t *testing.T) {
	spvChain := newLocalChain()

	reservationKey := big.NewInt(1)
	requestNonce := uint64(5)
	targetWalletPKH := [20]byte{0x01, 0x02, 0x03}

	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: targetWalletPKH,
	})

	stillProvable, err := verifyReservationActionStillProvable(
		spvChain,
		reservationKey,
		requestNonce,
		tbtc.ReservationActionTypeReanchor,
		targetWalletPKH,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stillProvable {
		t.Fatal("expected the still-pending action generation to be provable")
	}
}

// TestVerifyReservationActionStillProvable_StaleActionGeneration verifies
// that submission is skipped, without error, when the action generation at
// the given nonce is no longer pending (settled, timed out, or superseded)
// by the time the caller is ready to submit - mirroring the race the
// generic-loop adapter design (superseded by this dedicated loop) guarded
// against: a discovered transaction's action generation advancing between
// discovery and submission. A nil error matters here: an error would
// propagate out of proveReservationAcceptanceActions/
// proveReservationReanchorActions and abort that pass for every other
// in-flight action generation this tick, which is disproportionate for
// what is an expected, if rare, race outcome rather than an infrastructure
// failure.
func TestVerifyReservationActionStillProvable_StaleActionGeneration(t *testing.T) {
	spvChain := newLocalChain()

	reservationKey := big.NewInt(2)
	staleNonce := uint64(7)
	targetWalletPKH := [20]byte{0x01, 0x02, 0x03}

	// The action generation that produced the discovered transaction timed
	// out; the reservation may have since moved on to an unrelated action
	// generation.
	spvChain.setReservationAction(reservationKey, staleNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStateTimedOut,
		TargetWalletPublicKeyHash: targetWalletPKH,
	})

	stillProvable, err := verifyReservationActionStillProvable(
		spvChain,
		reservationKey,
		staleNonce,
		tbtc.ReservationActionTypeReanchor,
		targetWalletPKH,
	)
	if err != nil {
		t.Fatalf(
			"expected nil error for a stale action generation (the "+
				"caller must not abort the whole proving round for a "+
				"skip), got: %v",
			err,
		)
	}
	if stillProvable {
		t.Fatal("expected a timed-out action generation to be reported unprovable")
	}
}

// TestVerifyReservationActionStillProvable_WrongActionType verifies that
// submission is skipped when the action generation at the given nonce is
// pending but for a different action type than expected - e.g. the
// reservation moved on to a dissolution while the caller was still trying
// to prove a stale re-anchor transaction.
func TestVerifyReservationActionStillProvable_WrongActionType(t *testing.T) {
	spvChain := newLocalChain()

	reservationKey := big.NewInt(3)
	requestNonce := uint64(8)
	targetWalletPKH := [20]byte{0x01, 0x02, 0x03}

	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType: tbtc.ReservationActionTypeDissolution,
		State:      tbtc.ReservationActionStatePending,
	})

	stillProvable, err := verifyReservationActionStillProvable(
		spvChain,
		reservationKey,
		requestNonce,
		tbtc.ReservationActionTypeReanchor,
		targetWalletPKH,
	)
	if err != nil {
		t.Fatalf("expected nil error for a wrong action type, got: %v", err)
	}
	if stillProvable {
		t.Fatal("expected a mismatched action type to be reported unprovable")
	}
}

// TestVerifyReservationActionStillProvable_MismatchedTargetWallet verifies
// that submission is skipped when the reservation's current pending action
// generation at the given nonce targets a different wallet than the one
// the discovered transaction actually pays - evidence the transaction
// belongs to a superseded generation even though the current generation is
// also, coincidentally, pending and of the expected type.
func TestVerifyReservationActionStillProvable_MismatchedTargetWallet(t *testing.T) {
	spvChain := newLocalChain()

	reservationKey := big.NewInt(4)
	requestNonce := uint64(3)
	oldTargetWalletPKH := [20]byte{0x92, 0xa6, 0xec, 0x88, 0x9a, 0x8f, 0xa3, 0x4f, 0x73, 0x1e}
	newTargetWalletPKH := [20]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x11, 0x22, 0x33, 0x44}

	// A new re-anchor request superseded the one that produced the
	// discovered transaction, this time targeting a different wallet,
	// before the discovered transaction's proof was submitted.
	spvChain.setReservationAction(reservationKey, requestNonce, &tbtc.ReservationAction{
		ActionType:                tbtc.ReservationActionTypeReanchor,
		State:                     tbtc.ReservationActionStatePending,
		TargetWalletPublicKeyHash: newTargetWalletPKH,
	})

	stillProvable, err := verifyReservationActionStillProvable(
		spvChain,
		reservationKey,
		requestNonce,
		tbtc.ReservationActionTypeReanchor,
		oldTargetWalletPKH,
	)
	if err != nil {
		t.Fatalf(
			"expected nil error for a mismatched target wallet (the "+
				"caller must not abort the whole proving round for a "+
				"skip), got: %v",
			err,
		)
	}
	if stillProvable {
		t.Fatal("expected a mismatched target wallet to be reported unprovable")
	}
}

// TestVerifyReservationActionStillProvable_ChainError verifies that a
// chain-level error re-fetching the action generation is propagated to the
// caller, rather than silently treated as a skip - unlike a settled/
// superseded action generation, a read failure gives no evidence either
// way and must not be treated as "safe to skip".
func TestVerifyReservationActionStillProvable_ChainError(t *testing.T) {
	spvChain := newLocalChain()

	reservationKey := big.NewInt(5)
	requestNonce := uint64(1)

	// No action installed for this (reservationKey, requestNonce) pair, so
	// GetReservationAction returns an error (see localChain.GetReservationAction).
	stillProvable, err := verifyReservationActionStillProvable(
		spvChain,
		reservationKey,
		requestNonce,
		tbtc.ReservationActionTypeReanchor,
		[20]byte{},
	)
	if err == nil {
		t.Fatal("expected a chain error to be propagated, got nil")
	}
	if stillProvable {
		t.Fatal("expected a chain error to report unprovable")
	}
}
