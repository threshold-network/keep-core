package tbtcpg

import (
	"testing"
	"time"
)

// TestReservationReanchorTask_ResumeMarginBoundary pins the exact edge of
// the resume path's timeout safety margin gate. WalletProposalValidator
// signs a pending generation only while block.timestamp is below
// TimeoutAt - REQUEST_TIMEOUT_SAFETY_MARGIN, so at
// block.timestamp == TimeoutAt - margin the generation is already outside
// its signing window and must be skipped rather than proposed. Proposing
// there burns a fee estimate and a round on a proposal the validator
// rejects.
func TestReservationReanchorTask_ResumeMarginBoundary(t *testing.T) {
	tbtcChain, _, _, task, sourceWalletPublicKeyHash, targetWalletPublicKeyHash, reservationKey :=
		newInFlightReanchorFixture(t)

	// Pin the chain head so the margin edge is exact and does not drift
	// with the wall clock between seeding and the pass.
	head := time.Unix(1700000000, 0)
	tbtcChain.SetLatestBlockTimestamp(head)
	seedResumableReanchorGeneration(
		t,
		tbtcChain,
		targetWalletPublicKeyHash,
		reservationKey,
		4,
		head.Add(time.Duration(reservationRequestTimeoutSafetyMarginSeconds)*time.Second),
	)

	proposal, ok, err := task.Run(reanchorSourceRequest(tbtcChain, sourceWalletPublicKeyHash))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || proposal != nil {
		t.Fatalf(
			"expected no proposal for a generation exactly at the safety "+
				"margin, got ok=%v proposal=%v",
			ok,
			proposal,
		)
	}
	if got := tbtcChain.GetReservationReanchorValidationCallCount(); got != 0 {
		t.Fatalf(
			"expected the margin gate to skip the generation before "+
				"reaching on-chain validation, got %d validation calls",
			got,
		)
	}
	if got := len(tbtcChain.GetReservationReanchorRequestSubmissions()); got != 0 {
		t.Fatalf(
			"expected no new re-anchor request for a pending generation, "+
				"got %d submissions",
			got,
		)
	}
}
