package tbtc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

// TestFrostPreSignMaximumAnchorCapacityCost pins the reservation contract:
// anchor admission charges for ONE transaction input, and the batch that input
// belongs to does not scale it. Inputs are signed strictly sequentially, so a
// batch never needs more than one input's worth of unconsumed window at any
// instant; charging the whole batch reserved 21 times the peak demand.
func TestFrostPreSignMaximumAnchorCapacityCost(t *testing.T) {
	// One input with one local seat: BuildTaprootTx, then five attempts of
	// (Open, Round1, Round2, Abort) per seat plus one memoized Aggregate, so
	// 1 + 5*(4*1+1) = 26 revisions and 2*26+3 = 55 generations.
	cost, err := frostPreSignMaximumAnchorCapacityCost(
		frostPreSignAuthorizationMaximumInputs,
		1,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatal(err)
	}
	if cost.Revisions != 26 || cost.Generations != 55 {
		t.Fatalf("unexpected single-seat per-input cost [%+v]", cost)
	}

	// The batch size is validated but must not enter the cost. A one-input
	// batch and a full sweep reserve exactly the same thing, because they
	// reserve it once per input either way.
	for _, inputCount := range []uint64{
		1,
		2,
		uint64(frostPreSignAuthorizationMaximumInputs),
	} {
		batchCost, err := frostPreSignMaximumAnchorCapacityCost(
			inputCount,
			1,
			signingAttemptsLimit,
		)
		if err != nil {
			t.Fatalf("[%d]-input batch was rejected: %v", inputCount, err)
		}
		if batchCost != cost {
			t.Fatalf(
				"[%d]-input batch reserved [%+v], not the per-input [%+v]; the "+
					"reservation is scaling with the batch again",
				inputCount,
				batchCost,
				cost,
			)
		}
	}

	// An illegal batch size is still refused here, so a batch that never
	// passed proposal validation cannot reach the native signer this way.
	for _, inputCount := range []uint64{
		0,
		uint64(frostPreSignAuthorizationMaximumInputs) + 1,
	} {
		if _, err := frostPreSignMaximumAnchorCapacityCost(
			inputCount,
			1,
			signingAttemptsLimit,
		); err == nil || !strings.Contains(err.Error(), "input count") {
			t.Fatalf(
				"[%d]-input batch was admitted: [%v]",
				inputCount,
				err,
			)
		}
	}

	// The seat counts that matter operationally. Mainnet sortition samples a
	// hundred-seat wallet with replacement across roughly twenty operators, so
	// the average holder has five seats and a large staker can hold twenty or
	// more. Every one of them must be able to sign a full 20-deposit sweep plus
	// its main UTXO; a seat excluded here is lost to the wallet's signing
	// threshold, and enough excluded seats leave a formed wallet unable to move
	// the deposits it has already received.
	for _, test := range []struct {
		seats       uint64
		revisions   uint64
		generations uint64
	}{
		{seats: 1, revisions: 26, generations: 55},
		{seats: 4, revisions: 86, generations: 175},
		{seats: 5, revisions: 106, generations: 215},
		{seats: 20, revisions: 406, generations: 815},
		// The whole wallet held by one operator - not a realistic sortition
		// result, but the protocol's own maximum, and it has to fit for the
		// ceiling to be gone rather than merely moved.
		{
			seats:       uint64(frostPreSignAuthorizationMaximumSeats),
			revisions:   2006,
			generations: 4015,
		},
	} {
		seatCost, err := frostPreSignMaximumAnchorCapacityCost(
			frostPreSignAuthorizationMaximumInputs,
			test.seats,
			signingAttemptsLimit,
		)
		if err != nil {
			t.Fatalf(
				"maximum batch with [%d] local seats was rejected: %v",
				test.seats,
				err,
			)
		}
		if seatCost.Revisions != test.revisions ||
			seatCost.Generations != test.generations {
			t.Fatalf(
				"[%d]-seat per-input cost [%+v], expected [%d/%d]",
				test.seats,
				seatCost,
				test.revisions,
				test.generations,
			)
		}
		if seatCost.Revisions > FrostNativeSignerAnchorMaximumHistoryEvents ||
			seatCost.Generations >
				FrostNativeSignerAnchorMaximumHistoryProofEntries {
			t.Fatalf(
				"[%d]-seat per-input cost [%+v] exceeds restart windows [%d/%d]",
				test.seats,
				seatCost,
				FrostNativeSignerAnchorMaximumHistoryEvents,
				FrostNativeSignerAnchorMaximumHistoryProofEntries,
			)
		}
	}

	// The ceiling is gone rather than moved: every seat count a wallet can
	// award fits. The closed form is 40*seats+15 generations, which passes the
	// 4096-entry proof window only at 103 seats, and a wallet has 100 to give.
	if admissibleSeats := frostPreSignMaximumAdmissibleLocalSeatCount(
		signingAttemptsLimit,
	); admissibleSeats != uint64(frostPreSignAuthorizationMaximumSeats) {
		t.Fatalf(
			"local seat ceiling is [%d], expected every one of the wallet's "+
				"[%d] seats to be admissible",
			admissibleSeats,
			frostPreSignAuthorizationMaximumSeats,
		)
	}
}

// TestFrostPreSignPerInputReservationAdmitsMainnetSeatCounts is the regression
// this change exists for. Charging a whole batch up front admitted at most four
// local seats for a 21-input sweep. Mainnet runs a hundred-seat wallet across
// roughly twenty operators with replacement sortition and no per-operator cap,
// so the average operator holds five seats: wallets formed normally and then
// could not sweep, because most of the stake-weighted seats were refused every
// full-size batch and the group never reached its signing threshold.
//
// Reserving per input removes the exclusion outright rather than raising the
// bar, and this test states both halves - that the old arithmetic really did
// refuse these operators, and that the new one admits them.
func TestFrostPreSignPerInputReservationAdmitsMainnetSeatCounts(t *testing.T) {
	const maximumInputs = uint64(frostPreSignAuthorizationMaximumInputs)

	for _, seats := range []uint64{
		1, 4, 5, 6, 7, 10, 20, 50,
		uint64(frostPreSignAuthorizationMaximumSeats),
	} {
		cost, err := frostPreSignMaximumAnchorCapacityCost(
			maximumInputs,
			seats,
			signingAttemptsLimit,
		)
		if err != nil {
			t.Fatalf(
				"[%d] local seats cannot sign a [%d]-input sweep: %v",
				seats,
				maximumInputs,
				err,
			)
		}

		// The reservation an operator of this size is charged has to be
		// coverable by a freshly rotated window with room to keep working, not
		// merely to fit inside it once.
		if cost.Generations >=
			uint64(FrostNativeSignerAnchorMaximumHistoryProofEntries) &&
			seats != uint64(frostPreSignAuthorizationMaximumSeats) {
			t.Fatalf(
				"[%d] local seats reserve [%d] of the [%d]-entry proof window "+
					"for one input",
				seats,
				cost.Generations,
				FrostNativeSignerAnchorMaximumHistoryProofEntries,
			)
		}

		// What the batch-wide charge would have been for the same operator.
		// Anything over the window is an operator the old accounting excluded.
		batchGenerations := maximumInputs*cost.Revisions*
			frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall +
			frostNativeSignerTerminalGenerationAdvancesPerAdmittedInput
		excludedBefore := batchGenerations >
			uint64(FrostNativeSignerAnchorMaximumHistoryProofEntries)
		if seats >= 5 && !excludedBefore {
			t.Fatalf(
				"[%d] local seats were not excluded by the batch-wide charge "+
					"([%d] generations); this test no longer covers the defect",
				seats,
				batchGenerations,
			)
		}
	}

	// The five-seat operator is the average mainnet holder and the first one
	// the old ceiling of four excluded. Pin it as a number so a regression
	// reads as this test failing rather than as wallets quietly not sweeping.
	fiveSeatCost, err := frostPreSignAnchoredInputCost(5, signingAttemptsLimit)
	if err != nil {
		t.Fatal(err)
	}
	if fiveSeatCost.Revisions != 106 || fiveSeatCost.Generations != 215 {
		t.Fatalf("unexpected five-seat per-input cost [%+v]", fiveSeatCost)
	}
	twentySeatCost, err := frostPreSignAnchoredInputCost(
		20,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatal(err)
	}
	if twentySeatCost.Revisions != 406 || twentySeatCost.Generations != 815 {
		t.Fatalf("unexpected twenty-seat per-input cost [%+v]", twentySeatCost)
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

	for _, localSeatCount := range []uint64{
		1,
		4,
		uint64(frostPreSignAuthorizationMaximumSeats),
	} {
		cost, err := frostPreSignAnchoredInputCost(
			localSeatCount,
			signingAttemptsLimit,
		)
		if err != nil {
			t.Fatal(err)
		}
		expected := cost.Revisions*
			frostNativeSignerAmortizedGenerationAdvancesPerAnchoredCall +
			frostNativeSignerTerminalGenerationAdvancesPerAdmittedInput
		if cost.Generations != expected {
			t.Fatalf(
				"[%d] seats reserved [%d] generations, not the amortized [%d]",
				localSeatCount,
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
				"[%d] seats reserved the per-call worst case [%d] rather than "+
					"an amortized bound",
				localSeatCount,
				cost.Generations,
			)
		}
	}
}

// TestFrostPreSignSeatCeilingGuardStillFires pins that the seat-ceiling refusal
// is unreachable rather than deleted. Nothing a wallet can award trips it now -
// one input costs 40*seats+15 generations and a wallet has a hundred seats to
// give - but it is the guard that would catch a raised signing-attempt limit or
// a shrunken certified window before it silently excluded operators again, so
// it must keep working and keep naming the ceiling.
func TestFrostPreSignSeatCeilingGuardStillFires(t *testing.T) {
	resetFrostNativeSignerAnchorAdmissionMetricsForTest()
	t.Cleanup(resetFrostNativeSignerAnchorAdmissionMetricsForTest)

	// No protocol-legal seat count is refused under the shipped constants.
	for seats := uint64(1); seats <=
		uint64(frostPreSignAuthorizationMaximumSeats); seats++ {
		if _, err := frostPreSignMaximumAnchorCapacityCost(
			uint64(frostPreSignAuthorizationMaximumInputs),
			seats,
			signingAttemptsLimit,
		); err != nil {
			t.Fatalf("[%d] local seats were refused: %v", seats, err)
		}
	}
	if rejections :=
		frostNativeSignerAnchorSeatCeilingRejections.Load(); rejections != 0 {
		t.Fatalf(
			"the seat ceiling refused [%d] admissible seat counts",
			rejections,
		)
	}

	// A raised attempt limit is the change most likely to move the ceiling
	// back into reach, so it is what drives the guard here. At twenty attempts
	// one input costs 160*seats+45 generations and the proof window binds at
	// twenty-five local seats.
	const attempts = uint64(20)
	if ceiling := frostPreSignMaximumAdmissibleLocalSeatCount(
		attempts,
	); ceiling != 25 {
		t.Fatalf("unexpected [%d]-attempt seat ceiling [%d]", attempts, ceiling)
	}
	if _, err := frostPreSignMaximumAnchorCapacityCost(
		uint64(frostPreSignAuthorizationMaximumInputs),
		25,
		attempts,
	); err != nil {
		t.Fatalf("the [%d]-attempt ceiling itself was refused: %v", attempts, err)
	}
	_, err := frostPreSignMaximumAnchorCapacityCost(
		uint64(frostPreSignAuthorizationMaximumInputs),
		26,
		attempts,
	)
	if err == nil {
		t.Fatal("a seat count over the ceiling was admitted")
	}
	// The refusal has to name the ceiling, this node's own seat count, and the
	// only remedy. Batch size must NOT be offered: it is no longer a lever, and
	// an operator who shrinks sweeps because the error suggested it would lose
	// throughput and still be excluded.
	for _, want := range []string{
		"at most [25] local seats",
		"this node holds [26]",
		"shed seats down to [25]",
		"signing threshold",
		"batch size is not a lever",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("seat-ceiling refusal is missing %q: [%v]", want, err)
		}
	}
	if !strings.Contains(err.Error(), "proof window") {
		t.Errorf("seat-ceiling refusal did not name the window: [%v]", err)
	}

	// The revision window is the binding one at a high enough attempt count,
	// and it has its own message.
	_, err = frostPreSignMaximumAnchorCapacityCost(
		uint64(frostPreSignAuthorizationMaximumInputs),
		1,
		1000,
	)
	if err == nil || !strings.Contains(err.Error(), "history window") {
		t.Fatalf("the revision window did not refuse an oversized cost: [%v]", err)
	}
	if !strings.Contains(err.Error(), "no local seat count can sign") {
		t.Errorf(
			"a configuration no seat count can serve did not say so: [%v]",
			err,
		)
	}

	if rejections :=
		frostNativeSignerAnchorSeatCeilingRejections.Load(); rejections != 2 {
		t.Fatalf(
			"seat-ceiling rejections counted [%d], expected [2]",
			rejections,
		)
	}
}

func TestFrostNativeSignerAnchorAdmissionController_AtomicallyReservesNearWarning(
	t *testing.T,
) {
	resetFrostNativeSignerAnchorAdmissionMetricsForTest()
	t.Cleanup(resetFrostNativeSignerAnchorAdmissionMetricsForTest)

	// Six local seats cost 126 revisions and 255 generations for one input, so
	// one fits a headroom of 257 and two do not. That is the property under
	// test: the second workflow is refused on what the first was promised, not
	// on what the anchor has already spent.
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

	first, err := controller.reservePreSign(
		context.Background(),
		snapshot,
		4,
		6,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	// A concurrent workflow cannot spend the capacity promised to the first,
	// even though both observed the same pre-mutation readiness snapshot.
	if _, err := controller.reservePreSign(
		context.Background(),
		snapshot,
		1,
		6,
		signingAttemptsLimit,
	); err == nil || !strings.Contains(err.Error(), "temporary reservations") ||
		strings.Contains(err.Error(), "offline anchor rotation") {
		t.Fatalf("concurrent reservation exceeded headroom: [%v]", err)
	}
	if contention :=
		frostNativeSignerAnchorReservationContentionRejections.Load(); contention != 1 {
		t.Fatalf("reservation-contention rejections counted [%d], expected [1]", contention)
	}
	if exhausted :=
		frostNativeSignerAnchorUnreservedHeadroomRejections.Load(); exhausted != 0 {
		t.Fatalf("temporary contention counted as anchor exhaustion [%d] times", exhausted)
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
		6,
		signingAttemptsLimit,
	); err != nil {
		t.Fatalf("released reservation remained charged: [%v]", err)
	}

	// The DKG admit counter is incremented only by reserveDKG, not by the
	// controller-level reservePreSign calls above, so a successful native
	// DKG reservation must be the exact amount it moves.
	baseDKGs := frostNativeSignerAnchorAdmittedDKGs.Load()
	dkgReservation, err := controller.reserveDKG(
		context.Background(),
		1,
	)
	if err != nil {
		t.Fatalf("native DKG reservation was refused: [%v]", err)
	}
	defer dkgReservation.Release()
	if got := frostNativeSignerAnchorAdmittedDKGs.Load(); got != baseDKGs+1 {
		t.Fatalf(
			"native DKG admission counter moved by [%d], want [1]",
			got-baseDKGs,
		)
	}

	// The pre-sign admit counters are incremented by the gate (admitInput and
	// authorize), not by the controller-level reservePreSign calls in this
	// test, so they must stay at zero. A counter that bumps on the wrong call
	// site would burn a per-input or per-batch denominator the operator has
	// no way to reconstruct from the calls actually made.
	if got := frostNativeSignerAnchorAdmittedPresignInputs.Load(); got != 0 {
		t.Fatalf(
			"controller-level reservePreSign bumped the gate-level pre-sign "+
				"input counter to [%d]; only gate.admitInput must",
			got,
		)
	}
	if got := frostNativeSignerAnchorAdmittedPresignRelayGates.Load(); got != 0 {
		t.Fatalf(
			"controller-level reservePreSign bumped the gate-level pre-sign "+
				"relay-gate counter to [%d]; only gate.authorize must",
			got,
		)
	}
}

func TestFrostNativeSignerAnchorAdmissionController_ContentionSaturatesAvailableHeadroom(
	t *testing.T,
) {
	resetFrostNativeSignerAnchorAdmissionMetricsForTest()
	t.Cleanup(resetFrostNativeSignerAnchorAdmissionMetricsForTest)

	currentHeadroom := frostNativeSignerAnchorCapacity{
		Revisions:   FrostNativeSignerAnchorMaximumHistoryEvents,
		Generations: FrostNativeSignerAnchorMaximumHistoryProofEntries,
	}
	controller := &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			return currentHeadroom, nil
		},
	}
	snapshot := testFrostAnchorAdmissionReadinessSnapshot(
		FrostNativeSignerAnchorMaximumHistoryEvents,
		FrostNativeSignerAnchorMaximumHistoryProofEntries,
	)

	first, err := controller.reservePreSign(
		context.Background(),
		snapshot,
		1,
		20,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatal(err)
	}
	currentHeadroom = frostNativeSignerAnchorCapacity{
		Revisions:   300,
		Generations: FrostNativeSignerAnchorMaximumHistoryProofEntries,
	}
	_, err = controller.reservePreSign(
		context.Background(),
		snapshot,
		1,
		1,
		signingAttemptsLimit,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "only [0] are currently unreserved") ||
		!strings.Contains(err.Error(), "temporary reservations") ||
		strings.Contains(err.Error(), "offline anchor rotation") {
		t.Fatalf("wrapped or terminal reservation-contention refusal: [%v]", err)
	}
	if contention :=
		frostNativeSignerAnchorReservationContentionRejections.Load(); contention != 1 {
		t.Fatalf("reservation-contention rejections counted [%d], expected [1]", contention)
	}
	if exhausted :=
		frostNativeSignerAnchorUnreservedHeadroomRejections.Load(); exhausted != 0 {
		t.Fatalf("temporary contention counted as anchor exhaustion [%d] times", exhausted)
	}

	_, err = controller.reservePreSign(
		context.Background(),
		snapshot,
		1,
		20,
		signingAttemptsLimit,
	)
	if err == nil ||
		!strings.Contains(err.Error(), "only [0] are unreserved") ||
		!strings.Contains(err.Error(), "offline anchor rotation") ||
		strings.Contains(err.Error(), "temporary reservations") {
		t.Fatalf("wrapped or transient raw-exhaustion refusal: [%v]", err)
	}
	if contention :=
		frostNativeSignerAnchorReservationContentionRejections.Load(); contention != 1 {
		t.Fatalf("raw exhaustion changed contention count to [%d]", contention)
	}
	if exhausted :=
		frostNativeSignerAnchorUnreservedHeadroomRejections.Load(); exhausted != 1 {
		t.Fatalf("raw exhaustion rejections counted [%d], expected [1]", exhausted)
	}

	first.Release()
	retry, err := controller.reservePreSign(
		context.Background(),
		snapshot,
		1,
		1,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatalf("released temporary reservation still blocked retry: [%v]", err)
	}
	retry.Release()

	// Two controller-level reservePreSign reservations succeeded, two were
	// refused. None of them went through gate.admitInput or gate.authorize,
	// and none went through reserveDKG, so every admission counter must
	// stay at zero. A counter that bumps on the wrong call site would burn
	// a per-input or per-batch denominator the operator has no way to
	// reconstruct from the calls actually made.
	if got := frostNativeSignerAnchorAdmittedPresignInputs.Load(); got != 0 {
		t.Fatalf(
			"controller-level reservePreSign bumped the gate-level pre-sign "+
				"input counter to [%d]; only gate.admitInput must",
			got,
		)
	}
	if got := frostNativeSignerAnchorAdmittedPresignRelayGates.Load(); got != 0 {
		t.Fatalf(
			"controller-level reservePreSign bumped the gate-level pre-sign "+
				"relay-gate counter to [%d]; only gate.authorize must",
			got,
		)
	}
	if got := frostNativeSignerAnchorAdmittedDKGs.Load(); got != 0 {
		t.Fatalf(
			"controller-level reservePreSign bumped the native DKG "+
				"admission counter to [%d]; only reserveDKG must",
			got,
		)
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

	// Thirteen local seats cost 266 revisions and 535 generations for one
	// input. The stale snapshot would admit that, but the authenticated current
	// tip has advanced after that snapshot and must be authoritative.
	if _, err := controller.reservePreSign(
		context.Background(),
		staleSnapshot,
		12,
		13,
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
	if preSignInput := frostNativeSignerAnchorPreSignInputRejections.Load(); preSignInput != 0 {
		t.Fatalf("pre-sign input rejections counted [%d]", preSignInput)
	}
	// Every reservation in this test was refused, so no admission counter
	// may have moved. The DKG admission counter in particular only fires on
	// a successful reserveDKG, so a poisoned DKG reservation that bumped
	// it would make the burn-rate denominator count work that never
	// actually proceeded.
	if got := frostNativeSignerAnchorAdmittedDKGs.Load(); got != 0 {
		t.Fatalf(
			"poisoned reservations bumped the native DKG admission "+
				"counter to [%d]; only successful reserveDKG must",
			got,
		)
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

	// The recovery reservation also went through controller-level
	// reservePreSign, not through gate.admitInput, gate.authorize, or
	// reserveDKG, so the per-workflow admission counters must still be at
	// zero. A successful reservation that bumped the gate-level counters
	// would tell operators a per-input or per-batch admission happened
	// when only a controller-level reservation did.
	if got := frostNativeSignerAnchorAdmittedPresignInputs.Load(); got != 0 {
		t.Fatalf(
			"controller-level reservePreSign bumped the gate-level "+
				"pre-sign input counter to [%d]; only gate.admitInput must",
			got,
		)
	}
	if got := frostNativeSignerAnchorAdmittedPresignRelayGates.Load(); got != 0 {
		t.Fatalf(
			"controller-level reservePreSign bumped the gate-level "+
				"pre-sign relay-gate counter to [%d]; only gate.authorize must",
			got,
		)
	}

	resetFrostNativeSignerAnchorAdmissionMetricsForTest()
}

// TestFrostNativeSignerAnchorWorkloadRotationWarning pins the warning against
// the workload it is supposed to warn about: the largest single admission this
// node can be asked to make, which is one transaction input.
//
// Which of the two terms binds now depends on the seat count, and that is the
// honest answer. One input costs 40*seats+15 generations, so a small holder
// needs less than the flat floor's 256 and the flat floor genuinely is its
// warning; a large holder is warned much earlier by the workload term. While
// the reservation covered a whole batch this term carried everything - a
// four-seat node then stopped signing 3359 generations before the flat floor -
// and that gap is exactly what reserving per input removed.
func TestFrostNativeSignerAnchorWorkloadRotationWarning(t *testing.T) {
	fiftySeatCost, err := frostPreSignAnchoredInputCost(50, signingAttemptsLimit)
	if err != nil {
		t.Fatal(err)
	}
	if fiftySeatCost.Revisions != 1006 || fiftySeatCost.Generations != 2015 {
		t.Fatalf("unexpected fifty-seat per-input cost [%+v]", fiftySeatCost)
	}
	fourSeatCost, err := frostPreSignAnchoredInputCost(4, signingAttemptsLimit)
	if err != nil {
		t.Fatal(err)
	}
	if fourSeatCost.Revisions != 86 || fourSeatCost.Generations != 175 {
		t.Fatalf("unexpected four-seat per-input cost [%+v]", fourSeatCost)
	}
	// The gap the flat threshold used to leave open is gone for a small
	// holder: its next admission now costs less than the flat floor, so the
	// flat floor fires first and there is nothing left for the workload term
	// to catch. Pinning this keeps a future reservation increase honest.
	if fourSeatCost.Generations >=
		uint64(FrostNativeSignerAnchorRotationWarningHeadroom) {
		t.Fatalf(
			"a four-seat node again needs [%d] generations per admission, at "+
				"or above the flat rotation floor [%d]",
			fourSeatCost.Generations,
			FrostNativeSignerAnchorRotationWarningHeadroom,
		)
	}

	for _, test := range []struct {
		name               string
		revisionHeadroom   uint64
		generationHeadroom uint64
		localSeatCount     uint64
		warning            bool
	}{
		// A fresh rotation warns about nothing, at any seat count a wallet can
		// award.
		{
			name:               "four seats, both windows unspent",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     4,
			warning:            false,
		},
		{
			name:               "the whole wallet on one node, both windows unspent",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     uint64(frostPreSignAuthorizationMaximumSeats),
			warning:            false,
		},
		// One generation above the next admission's cost is the last moment
		// this node can still be admitted, so it is the last moment before the
		// warning.
		{
			name:               "fifty seats, one generation above the next admission",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: 2016,
			localSeatCount:     50,
			warning:            false,
		},
		{
			name:               "fifty seats, exactly the next admission's generations",
			revisionHeadroom:   FrostNativeSignerAnchorMaximumHistoryEvents,
			generationHeadroom: 2015,
			localSeatCount:     50,
			warning:            true,
		},
		// Each window is measured against its own dimension's cost. The
		// revision cost is about half the generation cost, so a revision
		// shortfall has to be caught on the revision number.
		{
			name:               "fifty seats, revision window at the next admission's revisions",
			revisionHeadroom:   1006,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     50,
			warning:            true,
		},
		{
			name:               "fifty seats, revision window one above",
			revisionHeadroom:   1007,
			generationHeadroom: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			localSeatCount:     50,
			warning:            false,
		},
		// A small holder's admission costs less than the flat floor, so the
		// flat floor is what warns it and the workload term stays quiet above
		// that floor.
		{
			name:               "four seats, one above the flat floor",
			revisionHeadroom:   FrostNativeSignerAnchorRotationWarningHeadroom + 1,
			generationHeadroom: FrostNativeSignerAnchorRotationWarningHeadroom + 1,
			localSeatCount:     4,
			warning:            false,
		},
		{
			name:               "four seats, at the flat floor",
			revisionHeadroom:   FrostNativeSignerAnchorRotationWarningHeadroom,
			generationHeadroom: FrostNativeSignerAnchorRotationWarningHeadroom,
			localSeatCount:     4,
			warning:            true,
		},
		// A seat count that can be admitted for nothing at all leaves only the
		// flat floor, which still has to work.
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

	// The workload term still earns its place for the seat counts whose
	// admission costs more than the flat floor: the flat predicate is silent at
	// the exact headroom where a fifty-seat node stops being admissible.
	if frostNativeSignerAnchorRotationWarning(
		minFrostNativeSignerAnchorHeadroom(
			FrostNativeSignerAnchorMaximumHistoryEvents,
			fiftySeatCost.Generations,
		),
	) {
		t.Fatal(
			"the flat rotation warning already fires at the fifty-seat " +
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
	// it must stay separately countable from the rotation causes. Nothing a
	// wallet can award reaches it now, so it is driven here the only way that
	// still can: a signing-attempt limit no certified window could serve.
	if _, err := frostPreSignMaximumAnchorCapacityCost(
		uint64(frostPreSignAuthorizationMaximumInputs),
		1,
		1000,
	); err == nil {
		t.Fatal("an unservable signing-attempt limit was admitted")
	}

	for _, test := range []struct {
		name     string
		counted  uint64
		expected uint64
	}{
		{
			name:     "reservation contention",
			counted:  frostNativeSignerAnchorReservationContentionRejections.Load(),
			expected: 0,
		},
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
		// Nothing here went through the per-input admission, so the counter
		// that tells an operator "an already-relayed batch was abandoned" must
		// stay clean.
		{
			name:     "pre-sign input",
			counted:  frostNativeSignerAnchorPreSignInputRejections.Load(),
			expected: 0,
		},
		// Every reserveDKG call in this test was refused on a refusal path,
		// so the native DKG admission counter must stay at zero. A counter
		// that bumps on a refused reservation would tell operators that a
		// DKG workflow was admitted when the seat count or window said it
		// could not be.
		{
			name:     "admitted DKG",
			counted:  frostNativeSignerAnchorAdmittedDKGs.Load(),
			expected: 0,
		},
		// Nothing here went through gate.admitInput or gate.authorize, so the
		// gate-level pre-sign admission counters must stay at zero. A counter
		// that bumps on a controller-level reservation would tell operators
		// a per-input or per-batch admission happened when only a raw
		// reservation did.
		{
			name:     "admitted pre-sign input",
			counted:  frostNativeSignerAnchorAdmittedPresignInputs.Load(),
			expected: 0,
		},
		{
			name:     "admitted pre-sign relay gate",
			counted:  frostNativeSignerAnchorAdmittedPresignRelayGates.Load(),
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
