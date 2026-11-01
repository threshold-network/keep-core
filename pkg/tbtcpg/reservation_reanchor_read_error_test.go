package tbtcpg

import (
	"math/big"
	"strings"
	"testing"
)

// TestReservationReanchorTask_UnreadableReservationDoesNotStarveThePass
// pins that an unreadable reservation costs only itself: the pass keeps
// reading the wallet's remaining reservations and proposes for one that is
// re-anchorable. A wallet can hold several reservations, so an RPC failure
// on one candidate must not abandon the hand-off of the others - the source
// wallet cannot finish closing until every reservation has moved.
func TestReservationReanchorTask_UnreadableReservationDoesNotStarveThePass(t *testing.T) {
	tbtcChain, _, _, task, sourceWalletPublicKeyHash, _, reservationKey :=
		newInFlightReanchorFixture(t)

	// The unreadable reservation has no record on this chain, so
	// GetReservation fails for it. It is listed first, ahead of the
	// fully configured, re-anchorable reservation.
	unreadableReservationKey := new(big.Int).Sub(reservationKey, big.NewInt(1))
	tbtcChain.SetWalletReservations(
		sourceWalletPublicKeyHash,
		[]*big.Int{unreadableReservationKey, reservationKey},
	)

	proposal, shouldExecute, err := task.Run(
		reanchorSourceRequest(tbtcChain, sourceWalletPublicKeyHash),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !shouldExecute || proposal == nil {
		t.Fatalf(
			"expected a proposal for the readable reservation, got "+
				"shouldExecute=%v proposal=%v",
			shouldExecute,
			proposal,
		)
	}
}

// TestReservationReanchorTask_AllReservationsUnreadableFailsThePass pins
// the other half of the per-reservation error handling: when every
// candidate is unreadable the pass must not look like a benign "nothing to
// do yet", because the wallet's reservations were never handed off and the
// source wallet cannot close. Reporting the failure lets the coordinator
// retry the pass.
func TestReservationReanchorTask_AllReservationsUnreadableFailsThePass(t *testing.T) {
	tbtcChain, _, _, task, sourceWalletPublicKeyHash, _, reservationKey :=
		newInFlightReanchorFixture(t)

	unreadableReservationKey := new(big.Int).Sub(reservationKey, big.NewInt(1))
	tbtcChain.SetWalletReservations(
		sourceWalletPublicKeyHash,
		[]*big.Int{unreadableReservationKey},
	)

	proposal, shouldExecute, err := task.Run(
		reanchorSourceRequest(tbtcChain, sourceWalletPublicKeyHash),
	)
	if err == nil {
		t.Fatalf(
			"expected an error for a pass that could not read any "+
				"reservation, got shouldExecute=%v proposal=%v",
			shouldExecute,
			proposal,
		)
	}
	if !strings.Contains(err.Error(), "cannot read every wallet reservation") {
		t.Errorf(
			"expected the incomplete-read error, got [%v]",
			err,
		)
	}
}
