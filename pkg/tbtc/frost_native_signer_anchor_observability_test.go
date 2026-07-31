package tbtc

import (
	"context"
	"errors"
	"strings"
	"testing"

	frostsigning "github.com/keep-network/keep-core/pkg/frost/signing"
)

// The warning must fire on exactly the seat counts anchor admission would
// refuse for a maximum-size batch, and stay quiet on the rest. This is the
// coupling that matters: a warning that fires on an admissible seat count is
// noise an operator learns to ignore, and one that stays quiet on an
// inadmissible one leaves the wallet to discover the exclusion weeks later,
// when a full-size sweep first forms.
func TestFrostPreSignLocalSeatCeilingWarning_MatchesAdmission(t *testing.T) {
	const maximumInputs = uint64(frostPreSignAuthorizationMaximumInputs)

	for seats := uint64(1); seats <= 8; seats++ {
		_, admissionErr := frostPreSignMaximumAnchorCapacityCost(
			maximumInputs,
			seats,
			signingAttemptsLimit,
		)
		warning, warned := frostPreSignLocalSeatCeilingWarning(
			seats,
			maximumInputs,
			signingAttemptsLimit,
		)
		if warned != (admissionErr != nil) {
			t.Errorf(
				"[%d] local seats: warned=%v but admission refusal=%v",
				seats,
				warned,
				admissionErr != nil,
			)
		}
		if !warned && warning != "" {
			t.Errorf(
				"[%d] local seats: no warning expected but text was [%s]",
				seats,
				warning,
			)
		}
	}
}

// The production ceiling is four local seats at the 21-input maximum batch and
// five signing attempts, and a five-seat node tops out at 19 inputs. Those are
// the numbers the admission accounting documents; pinning them here means a
// change to the certified windows or the attempt limit shows up as a failing
// test rather than as a silently different operator warning.
func TestFrostPreSignLocalSeatCeilingWarning_ProductionParameters(t *testing.T) {
	const maximumInputs = uint64(frostPreSignAuthorizationMaximumInputs)

	if ceiling := frostPreSignMaximumAdmissibleLocalSeatCount(
		maximumInputs,
		signingAttemptsLimit,
	); ceiling != 4 {
		t.Fatalf("production local seat ceiling: got %d want 4", ceiling)
	}

	if _, warned := frostPreSignLocalSeatCeilingWarning(
		4,
		maximumInputs,
		signingAttemptsLimit,
	); warned {
		t.Error("a four-seat node is admissible and must not be warned")
	}

	warning, warned := frostPreSignLocalSeatCeilingWarning(
		5,
		maximumInputs,
		signingAttemptsLimit,
	)
	if !warned {
		t.Fatal("a five-seat node is above the ceiling and must be warned")
	}
	// The operator needs all four numbers to act: how many seats it holds, the
	// ceiling it is over, the batch size that produced the ceiling, and the
	// largest batch it can actually sign.
	for _, want := range []string{"[5]", "[4]", "[21]", "[19]"} {
		if !strings.Contains(warning, want) {
			t.Errorf("warning is missing %s: [%s]", want, warning)
		}
	}
	// The exclusion is whole-node, not just the surplus seat, because
	// reservePreSign is charged for the complete local seat set at once.
	if !strings.Contains(warning, "signing threshold") {
		t.Errorf(
			"warning does not name the consequence for the wallet: [%s]",
			warning,
		)
	}

	sixSeatWarning, warned := frostPreSignLocalSeatCeilingWarning(
		6,
		maximumInputs,
		signingAttemptsLimit,
	)
	if !warned || !strings.Contains(sixSeatWarning, "[16]") {
		t.Errorf(
			"a six-seat node tops out at 16 inputs; warning was [%s]",
			sixSeatWarning,
		)
	}
}

// An unset attempt limit must be read the way authorize reads it, otherwise
// the warning describes a ceiling admission never applies.
func TestFrostPreSignLocalSeatCeilingWarning_ZeroAttemptsUsesTheDefault(
	t *testing.T,
) {
	const maximumInputs = uint64(frostPreSignAuthorizationMaximumInputs)

	zeroWarning, zeroWarned := frostPreSignLocalSeatCeilingWarning(
		5,
		maximumInputs,
		0,
	)
	defaultWarning, defaultWarned := frostPreSignLocalSeatCeilingWarning(
		5,
		maximumInputs,
		signingAttemptsLimit,
	)
	if zeroWarned != defaultWarned || zeroWarning != defaultWarning {
		t.Fatalf(
			"zero attempts did not fall back to the default limit: [%s] vs [%s]",
			zeroWarning,
			defaultWarning,
		)
	}
}

// A seat count that can sign no batch size at all must still be reported, and
// must not offer a largest admissible batch of zero inputs as if it were a
// usable setting. Under today's constants this needs a seat count admission
// rejects outright - at the 100-seat maximum a node can still sign a
// single-input batch - so the case is defensive, and it is covered because the
// arithmetic that makes it unreachable is exactly the arithmetic a constant
// change would move.
func TestFrostPreSignLocalSeatCeilingWarning_NoAdmissibleBatch(t *testing.T) {
	const maximumInputs = uint64(frostPreSignAuthorizationMaximumInputs)

	if admissible := frostPreSignMaximumAdmissibleInputCount(
		frostPreSignAuthorizationMaximumSeats,
		signingAttemptsLimit,
	); admissible == 0 {
		t.Fatalf(
			"the maximum seat count now signs nothing; this test needs a seat "+
				"count admission rejects outright, not [%d]",
			frostPreSignAuthorizationMaximumSeats,
		)
	}

	warning, warned := frostPreSignLocalSeatCeilingWarning(
		frostPreSignAuthorizationMaximumSeats+1,
		maximumInputs,
		signingAttemptsLimit,
	)
	if !warned {
		t.Fatal("a seat count that can sign nothing must be warned")
	}
	if !strings.Contains(warning, "no batch size at all") {
		t.Errorf(
			"warning does not say the node can sign nothing: [%s]",
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
) *frostProductionSignerReadinessSnapshot {
	return &frostProductionSignerReadinessSnapshot{
		Inventory: &frostNativeSignerInventorySnapshot{
			RestartableRevisionHeadroom:   revisions,
			RestartableGenerationHeadroom: generations,
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
			snapshot: readinessSnapshotWithHeadroom(revisions, generations),
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
		snapshot: readinessSnapshotWithHeadroom(3971, 3820),
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
				snapshot: readinessSnapshotWithHeadroom(0, 0),
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

	snapshot := readinessSnapshotWithHeadroom(4096, 4096)
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
