package tbtcpg

import (
	"testing"
)

// TestReservationReanchorTask_CooldownEndIsInclusive pins the run path's
// re-anchor cooldown gate on its own boundary: Reservation.sol refuses a
// request only while block.timestamp < reanchorCooldownUntil, so the
// reservation becomes requestable in the block where the chain head
// reaches the cooldown end. Skipping at that equality point leaves the
// reservation unrequested for one more pass after the cooldown has
// expired, and the local chain mirror accepts the request at the same
// point, so the two would disagree.
func TestReservationReanchorTask_CooldownEndIsInclusive(t *testing.T) {
	tbtcChain, _, _, task, sourceWalletPublicKeyHash, _, reservationKey :=
		newInFlightReanchorFixture(t)

	headTime, err := tbtcChain.GetLatestBlockTimestamp()
	if err != nil {
		t.Fatal(err)
	}

	reservation, err := tbtcChain.GetReservation(reservationKey)
	if err != nil {
		t.Fatal(err)
	}
	atCooldownEnd := *reservation
	// The chain head sits exactly on the cooldown end: one second later
	// the cooldown is over, and it is over now.
	atCooldownEnd.ReanchorCooldownUntil = uint32(headTime.Unix())
	tbtcChain.SetReservation(reservationKey, &atCooldownEnd)

	// The gate must not skip: with the head at the cooldown end the
	// reservation is requestable, so the pass proposes and books exactly
	// one request on the target.
	proposal, ok, err := task.Run(reanchorSourceRequest(tbtcChain, sourceWalletPublicKeyHash))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || proposal == nil {
		t.Fatalf(
			"expected a proposal once the chain head reached the cooldown "+
				"end, got ok=%v proposal=%v",
			ok,
			proposal,
		)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestAttempts()); got != 1 {
		t.Fatalf(
			"expected exactly 1 re-anchor request attempt at the cooldown "+
				"end, got %d",
			got,
		)
	}
}
