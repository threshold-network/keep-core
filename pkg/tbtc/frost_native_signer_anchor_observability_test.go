package tbtc

import (
	"context"
	"errors"
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

// The warning must fire on exactly the seat counts anchor admission would
// refuse, and stay quiet on the rest. This is the coupling that matters: a
// warning that fires on an admissible seat count is noise an operator learns to
// ignore, and one that stays quiet on an inadmissible one leaves the wallet to
// discover the exclusion weeks later, when a sweep first forms.
//
// Both sides are checked at the shipped attempt limit, where nothing is
// refused, and at an attempt limit high enough to put the ceiling back in
// reach, where the two must agree on exactly where it falls.
func TestFrostPreSignLocalSeatCeilingWarning_MatchesAdmission(t *testing.T) {
	const maximumInputs = uint64(frostPreSignAuthorizationMaximumInputs)

	for _, attempts := range []uint64{signingAttemptsLimit, 20} {
		for seats := uint64(1); seats <= 30; seats++ {
			_, admissionErr := frostPreSignMaximumAnchorCapacityCost(
				maximumInputs,
				seats,
				attempts,
			)
			warning, warned := frostPreSignLocalSeatCeilingWarning(
				seats,
				attempts,
			)
			if warned != (admissionErr != nil) {
				t.Errorf(
					"[%d] attempts, [%d] local seats: warned=%v but admission "+
						"refusal=%v",
					attempts,
					seats,
					warned,
					admissionErr != nil,
				)
			}
			if !warned && warning != "" {
				t.Errorf(
					"[%d] attempts, [%d] local seats: no warning expected but "+
						"text was [%s]",
					attempts,
					seats,
					warning,
				)
			}
		}
	}
}

// Under the shipped constants there is no ceiling left to warn about: anchor
// admission reserves one transaction input at a time, one input costs
// 40*seats+15 generations, and a whole hundred-seat wallet held by a single
// node still fits the 4096-entry proof window. Pinning that here means a change
// to the certified windows or the attempt limit shows up as a failing test
// rather than as operators silently excluded from signing again.
func TestFrostPreSignLocalSeatCeilingWarning_ProductionParameters(t *testing.T) {
	if ceiling := frostPreSignMaximumAdmissibleLocalSeatCount(
		signingAttemptsLimit,
	); ceiling != uint64(frostPreSignAuthorizationMaximumSeats) {
		t.Fatalf(
			"production local seat ceiling: got %d want %d",
			ceiling,
			frostPreSignAuthorizationMaximumSeats,
		)
	}

	// Every seat count a wallet can award, including the five-seat average
	// mainnet holder that the batch-wide reservation used to exclude.
	for seats := uint64(1); seats <=
		uint64(frostPreSignAuthorizationMaximumSeats); seats++ {
		if warning, warned := frostPreSignLocalSeatCeilingWarning(
			seats,
			signingAttemptsLimit,
		); warned {
			t.Fatalf(
				"[%d] local seats are admissible but were warned: [%s]",
				seats,
				warning,
			)
		}
	}
}

// A seat count above the ceiling still has to be reported, and the report has
// to be actionable. Only a raised attempt limit can produce one now, so that is
// what drives it.
func TestFrostPreSignLocalSeatCeilingWarning_AboveTheCeiling(t *testing.T) {
	const attempts = uint64(20)

	if ceiling := frostPreSignMaximumAdmissibleLocalSeatCount(
		attempts,
	); ceiling != 25 {
		t.Fatalf("[%d]-attempt seat ceiling: got %d want 25", attempts, ceiling)
	}
	if _, warned := frostPreSignLocalSeatCeilingWarning(25, attempts); warned {
		t.Error("a node at the ceiling is admissible and must not be warned")
	}

	warning, warned := frostPreSignLocalSeatCeilingWarning(26, attempts)
	if !warned {
		t.Fatal("a node above the ceiling must be warned")
	}
	// The operator needs the numbers it can act on: how many seats it holds and
	// the ceiling it is over. The batch size it used to be told is deliberately
	// absent - admission reserves per input now, so smaller sweeps do not help.
	for _, want := range []string{"[26]", "[25]"} {
		if !strings.Contains(warning, want) {
			t.Errorf("warning is missing %s: [%s]", want, warning)
		}
	}
	if !strings.Contains(warning, "Batch size is not a lever") {
		t.Errorf(
			"warning does not rule out the lever that no longer works: [%s]",
			warning,
		)
	}
	// The exclusion is whole-node, not just the surplus seat, because
	// reservePreSign is charged for the complete local seat set at once.
	if !strings.Contains(warning, "signing threshold") {
		t.Errorf(
			"warning does not name the consequence for the wallet: [%s]",
			warning,
		)
	}
}

// An unset attempt limit must be read the way the gate reads it, otherwise the
// warning describes a ceiling admission never applies.
func TestFrostPreSignLocalSeatCeilingWarning_ZeroAttemptsUsesTheDefault(
	t *testing.T,
) {
	// A seat count above what any wallet can award is the only input that
	// warns under the shipped attempt limit, which makes it the only one that
	// can compare two non-empty warnings.
	const overCapSeats = uint64(frostPreSignAuthorizationMaximumSeats) + 1

	zeroWarning, zeroWarned := frostPreSignLocalSeatCeilingWarning(
		overCapSeats,
		0,
	)
	defaultWarning, defaultWarned := frostPreSignLocalSeatCeilingWarning(
		overCapSeats,
		signingAttemptsLimit,
	)
	if !zeroWarned {
		t.Fatal("a seat count no wallet can award must be warned")
	}
	if zeroWarned != defaultWarned || zeroWarning != defaultWarning {
		t.Fatalf(
			"zero attempts did not fall back to the default limit: [%s] vs [%s]",
			zeroWarning,
			defaultWarning,
		)
	}
}

// A configuration in which no seat count can be admitted at all must still be
// reported, and must not offer shedding seats as if it were a usable remedy.
// Under today's constants this needs an attempt limit no certified window could
// serve, because a single local seat signing a single input costs 55 of 4096
// generations.
func TestFrostPreSignLocalSeatCeilingWarning_NoAdmissibleSeatCount(t *testing.T) {
	if admissible := frostPreSignMaximumAdmissibleLocalSeatCount(
		signingAttemptsLimit,
	); admissible == 0 {
		t.Fatal(
			"the shipped attempt limit now admits no seat count at all; this " +
				"test needs a configuration admission rejects outright",
		)
	}

	warning, warned := frostPreSignLocalSeatCeilingWarning(1, 1000)
	if !warned {
		t.Fatal("a configuration that can sign nothing must be warned")
	}
	if !strings.Contains(warning, "no local seat count can sign") {
		t.Errorf(
			"warning does not say the node can sign nothing: [%s]",
			warning,
		)
	}
	if strings.Contains(warning, "shed seats") {
		t.Errorf(
			"warning offers shedding seats for a limit no seat count can "+
				"serve: [%s]",
			warning,
		)
	}
}

type stubFrostProductionSignerReadinessVerifier struct {
	snapshot        *frostProductionSignerReadinessSnapshot
	err             error
	unchangedErr    error
	unchangedCalls  int
	reconcileCalls  int
	lastFinality    FrostPreSignFinality
	lastUnchangedIn *frostProductionSignerReadinessSnapshot
}

func (stub *stubFrostProductionSignerReadinessVerifier) verifyFrostProductionSignerReadiness(
	_ context.Context,
	finality FrostPreSignFinality,
) (*frostProductionSignerReadinessSnapshot, error) {
	stub.reconcileCalls++
	stub.lastFinality = finality
	return stub.snapshot, stub.err
}

func (stub *stubFrostProductionSignerReadinessVerifier) verifyFrostProductionSignerReadinessUnchanged(
	_ context.Context,
	snapshot *frostProductionSignerReadinessSnapshot,
) error {
	stub.unchangedCalls++
	stub.lastUnchangedIn = snapshot
	return stub.unchangedErr
}

func readinessSnapshotWithHeadroom(
	revisions uint64,
	generations uint64,
	rotationWarning bool,
	largestLocalSeatCount uint64,
) *frostProductionSignerReadinessSnapshot {
	return &frostProductionSignerReadinessSnapshot{
		Inventory: &frostNativeSignerInventorySnapshot{
			RestartableRevisionHeadroom:   revisions,
			RestartableGenerationHeadroom: generations,
			AnchorRotationWarning:         rotationWarning,
			LargestLocalSeatCount:         largestLocalSeatCount,
		},
	}
}

// publishFrostNativeSignerAnchorHeadroomForTest drives one successful
// reconciliation through the real observer so the process-wide mirror holds a
// known reading. The mirror is last-write-wins and has no un-observe, so every
// test below establishes its own baseline this way rather than depending on
// the order tests run in.
func publishFrostNativeSignerAnchorHeadroomForTest(
	t *testing.T,
	revisions uint64,
	generations uint64,
) {
	t.Helper()
	if _, err := newFrostNativeSignerAnchorHeadroomObserver(
		&stubFrostProductionSignerReadinessVerifier{
			snapshot: readinessSnapshotWithHeadroom(revisions, generations, false, 0),
		},
	).verifyFrostProductionSignerReadiness(
		context.Background(),
		FrostPreSignFinality{BlockNumber: 1},
	); err != nil {
		t.Fatalf("seeding the headroom mirror failed: %v", err)
	}
}

func requireFrostNativeSignerAnchorHeadroomForTest(
	t *testing.T,
	context string,
	revisions uint64,
	generations uint64,
) {
	t.Helper()
	gotRevisions, gotGenerations, observed :=
		frostsigning.NativeTBTCSignerStateAnchorRestartableHeadroom()
	if !observed || gotRevisions != revisions ||
		gotGenerations != generations {
		t.Errorf(
			"%s: headroom mirror is (%d, %d, %v), want (%d, %d, true)",
			context,
			gotRevisions,
			gotGenerations,
			observed,
			revisions,
			generations,
		)
	}
}

// A successful reconciliation is the only place at this layer where the
// restartable headroom is both fresh and authenticated, so it is the only
// place allowed to publish it to the scrape. Without this the two numbers
// exist only on the loopback-only activation-handshake endpoint.
func TestFrostNativeSignerAnchorHeadroomObserver_PublishesOnSuccess(
	t *testing.T,
) {
	stub := &stubFrostProductionSignerReadinessVerifier{
		snapshot: readinessSnapshotWithHeadroom(3971, 3820, true, 42),
	}
	observer := newFrostNativeSignerAnchorHeadroomObserver(stub)

	snapshot, err := observer.verifyFrostProductionSignerReadiness(
		context.Background(),
		FrostPreSignFinality{BlockNumber: 7},
	)
	if err != nil || snapshot == nil {
		t.Fatalf("delegation failed: snapshot=%v err=%v", snapshot, err)
	}
	if stub.reconcileCalls != 1 || stub.lastFinality.BlockNumber != 7 {
		t.Fatalf(
			"inner verifier was not called through: calls=%d finality=%d",
			stub.reconcileCalls,
			stub.lastFinality.BlockNumber,
		)
	}

	requireFrostNativeSignerAnchorHeadroomForTest(
		t,
		"after a successful reconciliation",
		3971,
		3820,
	)
	warning, warningObserved := frostsigning.NativeTBTCSignerStateAnchorRotationWarning()
	if !warningObserved || !warning {
		t.Errorf(
			"rotation warning mirror is (%v, %v), want (true, true)",
			warning,
			warningObserved,
		)
	}
	seats, seatsObserved := frostsigning.NativeTBTCSignerStateAnchorLargestLocalSeatCount()
	if !seatsObserved || seats != 42 {
		t.Errorf(
			"largest local seat count mirror is (%d, %v), want (42, true)",
			seats,
			seatsObserved,
		)
	}
}

// Zero is the value that means "the certified windows are exhausted". A failed
// reconciliation - an unreachable anchor service, say - must not be able to
// publish it, and must leave the last good reading standing.
func TestFrostNativeSignerAnchorHeadroomObserver_FailurePublishesNothing(
	t *testing.T,
) {
	publishFrostNativeSignerAnchorHeadroomForTest(t, 2048, 1024)

	for _, testCase := range []struct {
		name string
		stub *stubFrostProductionSignerReadinessVerifier
	}{
		{
			name: "reconciliation error",
			stub: &stubFrostProductionSignerReadinessVerifier{
				snapshot: readinessSnapshotWithHeadroom(0, 0, false, 0),
				err:      errors.New("anchor service unreachable"),
			},
		},
		{
			name: "nil snapshot",
			stub: &stubFrostProductionSignerReadinessVerifier{},
		},
		{
			name: "nil inventory",
			stub: &stubFrostProductionSignerReadinessVerifier{
				snapshot: &frostProductionSignerReadinessSnapshot{},
			},
		},
	} {
		_, _ = newFrostNativeSignerAnchorHeadroomObserver(testCase.stub).
			verifyFrostProductionSignerReadiness(
				context.Background(),
				FrostPreSignFinality{BlockNumber: 2},
			)
		requireFrostNativeSignerAnchorHeadroomForTest(
			t,
			testCase.name+" overwrote the last good reading",
			2048,
			1024,
		)
	}
}

// The unchanged check proves a previously reconciled snapshot still holds; it
// produces no fresh reading, so republishing through it would only restate a
// known value while resetting the staleness signal that tells an operator how
// fresh the gauges are.
func TestFrostNativeSignerAnchorHeadroomObserver_UnchangedDelegatesOnly(
	t *testing.T,
) {
	publishFrostNativeSignerAnchorHeadroomForTest(t, 777, 888)

	stub := &stubFrostProductionSignerReadinessVerifier{
		unchangedErr: errors.New("cached readiness changed"),
	}
	observer := newFrostNativeSignerAnchorHeadroomObserver(stub)

	snapshot := readinessSnapshotWithHeadroom(4096, 4096, false, 0)
	err := observer.verifyFrostProductionSignerReadinessUnchanged(
		context.Background(),
		snapshot,
	)
	if err == nil || !strings.Contains(err.Error(), "cached readiness changed") {
		t.Fatalf("inner error was not propagated: %v", err)
	}
	if stub.unchangedCalls != 1 || stub.lastUnchangedIn != snapshot {
		t.Fatalf(
			"inner verifier was not called through: calls=%d",
			stub.unchangedCalls,
		)
	}
	requireFrostNativeSignerAnchorHeadroomForTest(
		t,
		"the unchanged check published a headroom reading",
		777,
		888,
	)
}

func TestFrostNativeSignerAnchorHeadroomObserver_NilInnerIsNil(t *testing.T) {
	if observer := newFrostNativeSignerAnchorHeadroomObserver(
		nil,
	); observer != nil {
		t.Fatalf("wrapping a nil verifier produced [%v]", observer)
	}
}
