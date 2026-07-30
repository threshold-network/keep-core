package tbtc

import (
	"context"
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
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

// TestFrostPreSignGenerationReservationIsAmortizedNotWorstCase pins the two
// gaps the generation accounting deliberately leaves open, both of which are
// documented residuals at the constants rather than proven bounds. The
// reservation sits below the hard per-operation bound the output barrier
// enforces, and that bound itself sits below what one anchored call can
// actually reach in the signer engine. Closing either gap silently - by
// reserving the worst case again, or by widening the barrier - must be a
// deliberate decision that revisits the documentation with it.
func TestFrostPreSignGenerationReservationIsAmortizedNotWorstCase(t *testing.T) {
	if frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall >=
		frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall {
		t.Fatalf(
			"amortized reservation [%d] no longer sits below the per-operation "+
				"bound [%d]",
			frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall,
			frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall,
		)
	}
	if frostsigning.
		NativeTBTCSignerStateAnchorEngineReachableGenerationAdvancePerOperation <=
		frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall {
		t.Fatalf(
			"the engine-reachable per-call advance [%d] no longer exceeds the "+
				"per-operation bound [%d]; the four-advance residual documented "+
				"at both constants must stay stated or stay closed deliberately",
			frostsigning.
				NativeTBTCSignerStateAnchorEngineReachableGenerationAdvancePerOperation,
			frostNativeSignerMaximumGenerationAdvancesPerAnchoredCall,
		)
	}

	for _, test := range []struct {
		inputCount     uint64
		localSeatCount uint64
	}{
		{inputCount: 1, localSeatCount: 1},
		{inputCount: frostPreSignAuthorizationMaximumInputs, localSeatCount: 4},
	} {
		cost, err := frostPreSignAnchoredWorkflowCost(
			test.inputCount,
			test.localSeatCount,
			signingAttemptsLimit,
		)
		if err != nil {
			t.Fatal(err)
		}
		expected := cost.Revisions*
			frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall +
			frostNativeSignerTerminalGenerationAdvancesPerWorkflow
		if cost.Generations != expected {
			t.Fatalf(
				"[%d] inputs with [%d] seats reserved [%d] generations, not the "+
					"amortized [%d]",
				test.inputCount,
				test.localSeatCount,
				cost.Generations,
				expected,
			)
		}
		// The reservation is explicitly not the engine's per-call worst case
		// on every call. Repeated persist failures after their rename can
		// overrun it; the terminal allowance is what absorbs a few of them.
		if cost.Generations >= cost.Revisions*frostsigning.
			NativeTBTCSignerStateAnchorEngineReachableGenerationAdvancePerOperation {
			t.Fatalf(
				"[%d] inputs with [%d] seats reserved the per-call worst case "+
					"[%d] rather than an amortized bound",
				test.inputCount,
				test.localSeatCount,
				cost.Generations,
			)
		}
	}
}

// TestFrostPreSignSeatCeilingResidual pins the exclusion the amortized
// reservation still leaves in place. Four local seats is the ceiling at the
// standard maximum batch, so a five-seat holder in a hundred-seat wallet - an
// ordinary sortition result for a large staker, one seat over - is refused
// every 20-deposit sweep, and those seats are lost to the wallet's signing
// threshold. That is an accepted operational limit; moving it needs the
// certified windows or the attempt limit to change, so the numbers are pinned
// here and reported in the rejection itself.
func TestFrostPreSignSeatCeilingResidual(t *testing.T) {
	if admissibleSeats := frostPreSignMaximumAdmissibleLocalSeatCount(
		frostPreSignAuthorizationMaximumInputs,
		signingAttemptsLimit,
	); admissibleSeats != 4 {
		t.Fatalf(
			"unexpected maximum-batch local seat ceiling [%d]",
			admissibleSeats,
		)
	}
	for _, test := range []struct {
		localSeatCount      uint64
		admissibleInputs    uint64
		servesStandardSweep bool
	}{
		{localSeatCount: 4, admissibleInputs: 21, servesStandardSweep: true},
		{localSeatCount: 5, admissibleInputs: 19, servesStandardSweep: false},
		{localSeatCount: 6, admissibleInputs: 16, servesStandardSweep: false},
	} {
		admissibleInputs := frostPreSignMaximumAdmissibleInputCount(
			test.localSeatCount,
			signingAttemptsLimit,
		)
		if admissibleInputs != test.admissibleInputs {
			t.Fatalf(
				"[%d] local seats top out at [%d] inputs, expected [%d]",
				test.localSeatCount,
				admissibleInputs,
				test.admissibleInputs,
			)
		}
		if servesStandardSweep := admissibleInputs >=
			uint64(frostPreSignAuthorizationMaximumInputs); servesStandardSweep !=
			test.servesStandardSweep {
			t.Fatalf(
				"[%d] local seats serving the standard sweep is [%t], expected [%t]",
				test.localSeatCount,
				servesStandardSweep,
				test.servesStandardSweep,
			)
		}
	}

	// The rejection has to name both levers the operator holds: how many seats
	// this batch admits, and how large a batch these seats admit. Neither is
	// inferable from the other, and the exclusion is otherwise visible only as
	// a wallet that quietly stops reaching its signing threshold.
	_, err := frostPreSignMaximumAnchorCapacityCost(
		frostPreSignAuthorizationMaximumInputs,
		5,
		signingAttemptsLimit,
	)
	if err == nil {
		t.Fatal("five local seats were admitted at the maximum batch size")
	}
	if !strings.Contains(err.Error(), "at most [4] local seats") ||
		!strings.Contains(
			err.Error(),
			"[5] local seats can sign at most [19] inputs",
		) ||
		!strings.Contains(err.Error(), "signing threshold") {
		t.Fatalf("seat-ceiling rejection lost an operator lever: [%v]", err)
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
