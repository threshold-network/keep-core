package tbtc

import (
	"context"
	"errors"
	"fmt"
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

// TestFrostNativeSignerAnchorAdmissionController_RefusesWhilePoisoned pins the
// invariant that admission must not accept work this node cannot finish. The
// controller reads headroom through the anchor binding and the inventory
// bridge, neither of which takes the request-taking barrier, so a barrier that
// has latched a terminal failure leaves every headroom number here looking
// perfectly healthy. Without the poisoned check a poisoned node keeps admitting
// pre-sign and DKG workflows whose every signer call is already doomed, and the
// wallet loses this member's seats with no node reporting a cause.
func TestFrostNativeSignerAnchorAdmissionController_RefusesWhilePoisoned(
	t *testing.T,
) {
	resetFrostNativeSignerAnchorAdmissionMetricsForTest()

	poisonCause := fmt.Errorf(
		"%w: post-mutation anchor advance did not match",
		frostsigning.ErrNativeTBTCSignerStateAnchorTerminal,
	)
	poisoned := poisonCause
	controller := &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			// Both windows completely unspent: nothing but the poison can
			// refuse anything here.
			return frostNativeSignerAnchorCapacity{
				Revisions:   FrostNativeSignerAnchorMaximumHistoryEvents,
				Generations: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			}, nil
		},
		anchorPoisoned: func() error { return poisoned },
	}
	snapshot := testFrostAnchorAdmissionReadinessSnapshot(
		FrostNativeSignerAnchorMaximumHistoryEvents,
		FrostNativeSignerAnchorMaximumHistoryProofEntries,
	)

	// Every reservation path shares reserve(), so pre-sign, native DKG and DKG
	// retirement all have to refuse. DKG is not exempt: a poisoned node cannot
	// persist a key package any more than it can sign.
	for _, test := range []struct {
		name    string
		reserve func() (*frostNativeSignerAnchorRevisionReservation, error)
	}{
		{
			name: "pre-sign",
			reserve: func() (
				*frostNativeSignerAnchorRevisionReservation,
				error,
			) {
				return controller.reservePreSign(
					context.Background(),
					snapshot,
					1,
					1,
					signingAttemptsLimit,
				)
			},
		},
		{
			name: "native DKG",
			reserve: func() (
				*frostNativeSignerAnchorRevisionReservation,
				error,
			) {
				return controller.reserveDKG(context.Background(), 1)
			},
		},
		{
			name: "native DKG retirement",
			reserve: func() (
				*frostNativeSignerAnchorRevisionReservation,
				error,
			) {
				return controller.reserveDKGRetirement(context.Background(), 1)
			},
		},
	} {
		reservation, err := test.reserve()
		if err == nil {
			reservation.Release()
			t.Fatalf(
				"[%s] was admitted while the state anchor is poisoned",
				test.name,
			)
		}
		if !errors.Is(
			err,
			frostsigning.ErrNativeTBTCSignerStateAnchorTerminal,
		) {
			t.Fatalf(
				"[%s] refusal did not carry the terminal anchor cause: [%v]",
				test.name,
				err,
			)
		}
		// The refusal has to be operator-actionable on its own. Poisoning is
		// latched on a package-global barrier and nothing clears it in
		// process, so a message that does not name the restart leaves an
		// operator waiting for a recovery that cannot arrive.
		if !strings.Contains(err.Error(), "terminally poisoned") ||
			!strings.Contains(err.Error(), "restart") {
			t.Fatalf(
				"[%s] refusal did not name the cause and the remedy: [%v]",
				test.name,
				err,
			)
		}
	}

	if rejections :=
		frostNativeSignerAnchorPoisonedRejections.Load(); rejections != 3 {
		t.Fatalf(
			"poisoned rejections counted [%d], expected [3]",
			rejections,
		)
	}
	// Nothing else refused, so no other cause may have been counted. A counter
	// that fires on the wrong cause sends an operator to the wrong remedy.
	if seatCeiling := frostNativeSignerAnchorSeatCeilingRejections.Load(); seatCeiling != 0 {
		t.Fatalf("seat-ceiling rejections counted [%d]", seatCeiling)
	}
	if unreserved := frostNativeSignerAnchorUnreservedHeadroomRejections.Load(); unreserved != 0 {
		t.Fatalf("unreserved-headroom rejections counted [%d]", unreserved)
	}
	if rotationFloor := frostNativeSignerAnchorRotationFloorRejections.Load(); rotationFloor != 0 {
		t.Fatalf("rotation-floor rejections counted [%d]", rotationFloor)
	}

	// The poison was the only thing refusing: with it cleared the identical
	// reservation is admitted, so the refusals above cannot be explained by
	// headroom.
	poisoned = nil
	reservation, err := controller.reservePreSign(
		context.Background(),
		snapshot,
		1,
		1,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatalf("healthy anchor refused an admissible workflow: [%v]", err)
	}
	reservation.Release()
}

// TestFrostNativeSignerAnchorWorkloadRotationWarning pins the warning against
// the workload it is supposed to warn about. The flat floor fires only once all
// new work is already refused, which is far too late to be a warning: a
// four-seat node - an ordinary sortition result, and the ceiling for the
// standard 21-input sweep - stops admitting maximum-size batches thousands of
// generations before it.
func TestFrostNativeSignerAnchorWorkloadRotationWarning(t *testing.T) {
	fourSeatCost, err := frostPreSignAnchoredWorkflowCost(
		frostPreSignAuthorizationMaximumInputs,
		4,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatal(err)
	}
	if fourSeatCost.Revisions != 1806 || fourSeatCost.Generations != 3615 {
		t.Fatalf("unexpected four-seat maximum-batch cost [%+v]", fourSeatCost)
	}
	// The gap the flat threshold leaves open, pinned as a number so that
	// shrinking it is a deliberate act.
	if gap := fourSeatCost.Generations -
		uint64(FrostNativeSignerAnchorRotationWarningHeadroom); gap != 3359 {
		t.Fatalf(
			"the flat warning now fires [%d] generations after a four-seat "+
				"node stops signing, expected [3359]",
			gap,
		)
	}

	for _, test := range []struct {
		name               string
		revisionHeadroom   uint64
		generationHeadroom uint64
		localSeatCount     uint64
		warning            bool
	}{
		// A fresh rotation warns about nothing.
		{
			name:               "four seats, both windows unspent",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     4,
			warning:            false,
		},
		// One generation above the next workflow's cost is the last moment
		// this node can still admit a maximum-size batch, so it is the last
		// moment before the warning.
		{
			name:               "four seats, one generation above the next workflow",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: 3616,
			localSeatCount:     4,
			warning:            false,
		},
		{
			name:               "four seats, exactly the next workflow's generations",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: 3615,
			localSeatCount:     4,
			warning:            true,
		},
		// Each window is measured against its own dimension's cost. The
		// revision cost is about half the generation cost, so a revision
		// shortfall has to be caught on the revision number.
		{
			name:               "four seats, revision window at the next workflow's revisions",
			revisionHeadroom:   1806,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     4,
			warning:            true,
		},
		{
			name:               "four seats, revision window one above",
			revisionHeadroom:   1807,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     4,
			warning:            false,
		},
		// A node above the seat ceiling must not be warned forever. It can
		// never be admitted for 21 inputs at all, so it is measured against
		// the 19-input batch it can actually be admitted for; warning
		// permanently would also force its activation-handshake health to
		// false for an exclusion that no rotation can fix.
		{
			name:               "five seats, both windows unspent",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     5,
			warning:            false,
		},
		{
			name:               "five seats, at its own admissible maximum",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: 4031,
			localSeatCount:     5,
			warning:            true,
		},
		// A seat count that can serve no batch at all leaves only the flat
		// floor, which still has to work.
		{
			name:               "no local seats, windows unspent",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     0,
			warning:            false,
		},
		{
			name:               "no local seats, at the flat floor",
			revisionHeadroom:   FrostNativeSignerAnchorRotationWarningHeadroom,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     0,
			warning:            true,
		},
		// The flat floor is kept, not replaced: it still fires for a seat
		// count whose workload term would not have.
		{
			name:               "one seat, at the flat floor",
			revisionHeadroom:   FrostNativeSignerAnchorRotationWarningHeadroom,
			generationHeadroom: FrostNativeSignerAnchorRotationWarningHeadroom,
			localSeatCount:     1,
			warning:            true,
		},
	} {
		if warning := frostNativeSignerAnchorWorkloadRotationWarning(
			test.revisionHeadroom,
			test.generationHeadroom,
			test.localSeatCount,
		); warning != test.warning {
			t.Fatalf(
				"[%s] warned [%t], expected [%t]",
				test.name,
				warning,
				test.warning,
			)
		}
	}

	// The whole point of the change: the flat predicate is still silent at the
	// exact headroom where a four-seat node stops being able to sign a
	// maximum-size batch.
	if frostNativeSignerAnchorRotationWarning(
		minFrostNativeSignerAnchorHeadroom(
			FrostNativeSignerAnchorMaximumHistoryEvents,
			fourSeatCost.Generations,
		),
	) {
		t.Fatal(
			"the flat rotation warning already fires at the four-seat " +
				"workload threshold, so the workload-relative term adds nothing",
		)
	}
}

// TestFrostNativeSignerAnchorAdmissionRefusalsNameARemedy pins that every
// admission refusal an operator can hit says what to do about it, and that each
// one is countable on its own. The unreserved-headroom refusal is the one a
// healthy, correctly configured node produces first, and it arrives while most
// of both windows are still unspent - so a message that only reports the
// numbers reads as transient when it is not.
func TestFrostNativeSignerAnchorAdmissionRefusalsNameARemedy(t *testing.T) {
	resetFrostNativeSignerAnchorAdmissionMetricsForTest()

	currentHeadroom := frostNativeSignerAnchorCapacity{}
	controller := &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			return currentHeadroom, nil
		},
	}

	// Native DKG reserves one revision and one generation per persistence
	// call, so 300 local seats cost exactly 600 of each and each dimension can
	// be starved on its own while the other stays healthy.
	currentHeadroom = frostNativeSignerAnchorCapacity{
		Revisions:   300,
		Generations: 1000,
	}
	if _, err := controller.reserveDKG(
		context.Background(),
		300,
	); err == nil || !strings.Contains(err.Error(), "anchor revisions") ||
		!strings.Contains(err.Error(), "offline anchor rotation is required") {
		t.Fatalf("revision-dimension refusal did not name the remedy: [%v]", err)
	}

	currentHeadroom = frostNativeSignerAnchorCapacity{
		Revisions:   1000,
		Generations: 300,
	}
	if _, err := controller.reserveDKG(
		context.Background(),
		300,
	); err == nil || !strings.Contains(err.Error(), "signer generations") ||
		!strings.Contains(err.Error(), "offline anchor rotation is required") {
		t.Fatalf("generation-dimension refusal did not name the remedy: [%v]", err)
	}

	// The sibling rotation-floor refusal keeps its own wording and its own
	// counter, so a monitoring system can tell "rotation is due" from
	// "rotation is overdue and everything is already refused".
	currentHeadroom = frostNativeSignerAnchorCapacity{
		Revisions:   FrostNativeSignerAnchorRotationWarningHeadroom,
		Generations: 1000,
	}
	if _, err := controller.reserveDKG(
		context.Background(),
		1,
	); err == nil || !strings.Contains(err.Error(), "is blocked with") ||
		!strings.Contains(err.Error(), "offline anchor rotation is required") {
		t.Fatalf("rotation-floor refusal changed shape: [%v]", err)
	}

	// The seat-ceiling refusal is a configuration signal no rotation fixes, so
	// it must stay separately countable from the rotation causes.
	if _, err := frostPreSignMaximumAnchorCapacityCost(
		frostPreSignAuthorizationMaximumInputs,
		5,
		signingAttemptsLimit,
	); err == nil {
		t.Fatal("five local seats were admitted at the maximum batch size")
	}

	for _, test := range []struct {
		name     string
		counted  uint64
		expected uint64
	}{
		{
			name:     "unreserved headroom",
			counted:  frostNativeSignerAnchorUnreservedHeadroomRejections.Load(),
			expected: 2,
		},
		{
			name:     "rotation floor",
			counted:  frostNativeSignerAnchorRotationFloorRejections.Load(),
			expected: 1,
		},
		{
			name:     "seat ceiling",
			counted:  frostNativeSignerAnchorSeatCeilingRejections.Load(),
			expected: 1,
		},
		{
			name:     "poisoned",
			counted:  frostNativeSignerAnchorPoisonedRejections.Load(),
			expected: 0,
		},
	} {
		if test.counted != test.expected {
			t.Fatalf(
				"[%s] rejections counted [%d], expected [%d]",
				test.name,
				test.counted,
				test.expected,
			)
		}
	}

	// Registration must never be what breaks a node that has no metrics
	// endpoint configured.
	RegisterFrostNativeSignerAnchorAdmissionMetrics(nil)

	resetFrostNativeSignerAnchorAdmissionMetricsForTest()
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
