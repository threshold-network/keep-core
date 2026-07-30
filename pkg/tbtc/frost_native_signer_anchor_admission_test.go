package tbtc

import (
	"context"
	"strings"
	"testing"
)

func TestFrostPreSignMaximumAnchorCapacityCost(t *testing.T) {
	cost, err := frostPreSignMaximumAnchorCapacityCost(
		frostPreSignAuthorizationMaximumInputs,
		1,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatal(err)
	}
	if cost.Revisions != 546 || cost.Generations != 1095 {
		t.Fatalf("unexpected single-seat maximum cost [%+v]", cost)
	}
	if cost.Revisions > FrostNativeSignerAnchorMaximumHistoryEvents ||
		cost.Generations > FrostNativeSignerAnchorMaximumHistoryProofEntries {
		t.Fatalf(
			"single-seat maximum [%+v] exceeds restart windows [%d/%d]",
			cost,
			FrostNativeSignerAnchorMaximumHistoryEvents,
			FrostNativeSignerAnchorMaximumHistoryProofEntries,
		)
	}

	multiSeatCost, err := frostPreSignMaximumAnchorCapacityCost(
		frostPreSignAuthorizationMaximumInputs,
		2,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatal(err)
	}
	if multiSeatCost.Revisions != 966 ||
		multiSeatCost.Generations != 1935 {
		t.Fatalf("unexpected multi-seat maximum cost [%+v]", multiSeatCost)
	}

	// Three and four local seats are an ordinary sortition result in a
	// hundred-seat wallet, and a twenty-one-input batch is the standard
	// deposit sweep maximum. Both must be admissible at the maximum batch
	// size: a seat excluded here is lost to the wallet's signing threshold on
	// every batch of that size, and enough excluded seats stop the group from
	// assembling a threshold at all.
	for _, seats := range []uint64{3, 4} {
		seatCost, err := frostPreSignMaximumAnchorCapacityCost(
			frostPreSignAuthorizationMaximumInputs,
			seats,
			signingAttemptsLimit,
		)
		if err != nil {
			t.Fatalf("maximum batch with [%d] local seats was rejected: %v", seats, err)
		}
		if seatCost.Revisions > FrostNativeSignerAnchorMaximumHistoryEvents ||
			seatCost.Generations >
				FrostNativeSignerAnchorMaximumHistoryProofEntries {
			t.Fatalf(
				"[%d]-seat maximum [%+v] exceeds restart windows [%d/%d]",
				seats,
				seatCost,
				FrostNativeSignerAnchorMaximumHistoryEvents,
				FrostNativeSignerAnchorMaximumHistoryProofEntries,
			)
		}
	}

	// The windows are finite, so the ceiling is real and must stay pinned
	// where the accounting actually puts it. Crossing it is reported with the
	// ceiling itself, because a node above it is otherwise silently excluded
	// from every batch of this size.
	if admissibleSeats := frostPreSignMaximumAdmissibleLocalSeatCount(
		frostPreSignAuthorizationMaximumInputs,
		signingAttemptsLimit,
	); admissibleSeats != 4 {
		t.Fatalf(
			"unexpected maximum-batch local seat ceiling [%d]",
			admissibleSeats,
		)
	}
	_, err = frostPreSignMaximumAnchorCapacityCost(
		frostPreSignAuthorizationMaximumInputs,
		5,
		signingAttemptsLimit,
	)
	if err == nil || !strings.Contains(err.Error(), "proof window") {
		t.Fatalf("oversized workflow was accepted: [%v]", err)
	}
	if !strings.Contains(err.Error(), "at most [4] local seats") ||
		!strings.Contains(err.Error(), "signing threshold") {
		t.Fatalf("oversized workflow did not report its seat ceiling: [%v]", err)
	}
}

func TestFrostNativeSignerAnchorAdmissionController_AtomicallyReservesNearWarning(
	t *testing.T,
) {
	currentHeadroom := frostNativeSignerAnchorCapacity{
		Revisions:   FrostNativeSignerAnchorRotationWarningHeadroom + 1,
		Generations: FrostNativeSignerAnchorRotationWarningHeadroom + 1,
	}
	controller := &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			return currentHeadroom, nil
		},
	}
	snapshot := testFrostAnchorAdmissionReadinessSnapshot(
		FrostNativeSignerAnchorRotationWarningHeadroom+1,
		FrostNativeSignerAnchorRotationWarningHeadroom+1,
	)

	// Four inputs and one local seat reserve 104 revisions and 211
	// generations. Admission one unit above the warning boundary is safe
	// because the full dual-dimensional cost is reserved before the workflow.
	first, err := controller.reservePreSign(
		context.Background(),
		snapshot,
		4,
		1,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	// A concurrent workflow cannot spend the revisions promised to the first,
	// even though both observed the same pre-mutation readiness snapshot.
	if _, err := controller.reservePreSign(
		context.Background(),
		snapshot,
		1,
		1,
		signingAttemptsLimit,
	); err == nil || !strings.Contains(err.Error(), "unreserved") {
		t.Fatalf("concurrent reservation exceeded headroom: [%v]", err)
	}

	// Crossing into the warning band does not revoke the admitted reservation.
	// Boundary revalidation uses ordinary readiness (which rejects only zero);
	// only a fresh admission is rejected here.
	currentHeadroom = frostNativeSignerAnchorCapacity{
		Revisions:   FrostNativeSignerAnchorRotationWarningHeadroom,
		Generations: FrostNativeSignerAnchorRotationWarningHeadroom,
	}
	warningSnapshot := testFrostAnchorAdmissionReadinessSnapshot(
		FrostNativeSignerAnchorRotationWarningHeadroom,
		FrostNativeSignerAnchorRotationWarningHeadroom,
	)
	if err := validateFrostNativeSignerAnchorReadinessHeadroom(
		warningSnapshot.Inventory,
	); err != nil {
		t.Fatalf("in-flight readiness rejected warning headroom: [%v]", err)
	}
	if _, err := controller.reservePreSign(
		context.Background(),
		warningSnapshot,
		1,
		1,
		signingAttemptsLimit,
	); err == nil || !strings.Contains(err.Error(), "rotation") {
		t.Fatalf("new work was admitted inside rotation reserve: [%v]", err)
	}

	first.Release()
	currentHeadroom = frostNativeSignerAnchorCapacity{
		Revisions:   FrostNativeSignerAnchorRotationWarningHeadroom + 1,
		Generations: FrostNativeSignerAnchorRotationWarningHeadroom + 1,
	}
	if _, err := controller.reservePreSign(
		context.Background(),
		snapshot,
		1,
		1,
		signingAttemptsLimit,
	); err != nil {
		t.Fatalf("released reservation remained charged: [%v]", err)
	}
}

func TestFrostNativeSignerAnchorAdmissionController_RejectsStaleReadinessHeadroom(
	t *testing.T,
) {
	currentHeadroom := frostNativeSignerAnchorCapacity{
		Revisions:   400,
		Generations: 500,
	}
	controller := &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			return currentHeadroom, nil
		},
	}
	staleSnapshot := testFrostAnchorAdmissionReadinessSnapshot(400, 700)

	// Twelve inputs and one seat cost 312 revisions and 627 generations. The
	// stale snapshot would admit it, but the authenticated current tip has
	// advanced after that snapshot and must be authoritative.
	if _, err := controller.reservePreSign(
		context.Background(),
		staleSnapshot,
		12,
		1,
		signingAttemptsLimit,
	); err == nil || !strings.Contains(err.Error(), "unreserved") {
		t.Fatalf("stale readiness headroom was trusted: [%v]", err)
	}
}

func testFrostAnchorAdmissionReadinessSnapshot(
	revisionHeadroom uint64,
	generationHeadroom uint64,
) *frostProductionSignerReadinessSnapshot {
	return &frostProductionSignerReadinessSnapshot{
		Inventory: &frostNativeSignerInventorySnapshot{
			CertifiedFloorRevision:   1,
			CertifiedFloorGeneration: 1,
			CurrentAnchorRevision: 1 +
				FrostNativeSignerAnchorMaximumHistoryEvents -
				revisionHeadroom,
			StateGeneration: 1 +
				FrostNativeSignerAnchorMaximumHistoryProofEntries -
				generationHeadroom,
			RestartableRevisionHeadroom:   revisionHeadroom,
			RestartableGenerationHeadroom: generationHeadroom,
			AnchorRotationWarning: frostNativeSignerAnchorRotationWarning(
				minFrostNativeSignerAnchorHeadroom(
					revisionHeadroom,
					generationHeadroom,
				),
			),
		},
		InteractiveSigningReady: true,
	}
}
