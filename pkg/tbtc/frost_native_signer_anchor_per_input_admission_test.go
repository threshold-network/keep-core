package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/keep-network/keep-core/pkg/bitcoin"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// testFrostAnchorAdmissionHarness is a real admission controller with a
// settable headroom, plus the bookkeeping the tests below need: how many
// per-input admissions were asked for, and the high-water mark of what the
// controller had reserved at once.
//
// The controller itself is the production one. Only the headroom read is
// injected, exactly as the other admission tests inject it, so what is under
// test is the real reserve/release accounting rather than a model of it.
type testFrostAnchorAdmissionHarness struct {
	controller *frostNativeSignerAnchorAdmissionController

	mutex     sync.Mutex
	headroom  frostNativeSignerAnchorCapacity
	admits    int
	failures  int
	peak      frostNativeSignerAnchorCapacity
	onAdmit   func(admits int)
	seatCount uint64
}

func newTestFrostAnchorAdmissionHarness(
	headroom frostNativeSignerAnchorCapacity,
	seatCount uint64,
) *testFrostAnchorAdmissionHarness {
	harness := &testFrostAnchorAdmissionHarness{
		headroom:  headroom,
		seatCount: seatCount,
	}
	harness.controller = &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			harness.mutex.Lock()
			defer harness.mutex.Unlock()
			return harness.headroom, nil
		},
	}
	return harness
}

func (harness *testFrostAnchorAdmissionHarness) setHeadroom(
	headroom frostNativeSignerAnchorCapacity,
) {
	harness.mutex.Lock()
	defer harness.mutex.Unlock()
	harness.headroom = headroom
}

// admitInput is shaped exactly like the closure walletTransactionExecutor hands
// the signing executor, and charges what the gate charges: one input, for this
// node's whole local seat set, for the full signing-attempt budget.
func (harness *testFrostAnchorAdmissionHarness) admitInput(
	inputCount uint64,
) func(context.Context) (func(), error) {
	return func(ctx context.Context) (func(), error) {
		harness.mutex.Lock()
		harness.admits++
		admits := harness.admits
		onAdmit := harness.onAdmit
		harness.mutex.Unlock()
		if onAdmit != nil {
			onAdmit(admits)
		}

		reservation, err := harness.controller.reservePreSign(
			ctx,
			testFrostAnchorAdmissionReadinessSnapshot(
				FrostNativeSignerAnchorMaximumHistoryEvents,
				FrostNativeSignerAnchorMaximumHistoryProofEntries,
			),
			inputCount,
			harness.seatCount,
			signingAttemptsLimit,
		)
		if err != nil {
			harness.mutex.Lock()
			harness.failures++
			harness.mutex.Unlock()
			return nil, err
		}
		harness.recordPeak()
		return reservation.Release, nil
	}
}

func (harness *testFrostAnchorAdmissionHarness) recordPeak() {
	reserved := harness.reserved()
	harness.mutex.Lock()
	defer harness.mutex.Unlock()
	if reserved.Revisions > harness.peak.Revisions {
		harness.peak.Revisions = reserved.Revisions
	}
	if reserved.Generations > harness.peak.Generations {
		harness.peak.Generations = reserved.Generations
	}
}

func (harness *testFrostAnchorAdmissionHarness) reserved() frostNativeSignerAnchorCapacity {
	harness.controller.mutex.Lock()
	defer harness.controller.mutex.Unlock()
	return harness.controller.reserved
}

func (harness *testFrostAnchorAdmissionHarness) counts() (int, int) {
	harness.mutex.Lock()
	defer harness.mutex.Unlock()
	return harness.admits, harness.failures
}

// TestSigningExecutor_ReservesAnchorCapacityPerInputNotPerBatch is the
// behavioural half of the fix. The arithmetic tests prove one input's cost is
// what admission charges; this proves the batch loop actually charges it once
// per input and gives it back before the next one, so a node never holds more
// than one input's worth however large the sweep is.
//
// The executor is the ordinary in-process one the rest of signing_test.go uses.
// Its signing backend is irrelevant here - the admission discipline under test
// lives in signBatchWithTaprootPolicy's loop and is the same on every backend -
// and using it means the loop being exercised is the real one rather than a
// stand-in.
func TestSigningExecutor_ReservesAnchorCapacityPerInputNotPerBatch(
	t *testing.T,
) {
	executor := setupSigningExecutor(t)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	messages := []*big.Int{
		big.NewInt(1000),
		big.NewInt(2000),
		big.NewInt(3000),
	}

	// A twenty-seat operator - a large but entirely ordinary mainnet holder -
	// signing a full-size sweep. One input costs 406 revisions and 815
	// generations; the whole 21-input batch would have cost 8526 and 17055,
	// four times the certified windows.
	const localSeats = uint64(20)
	const sweepInputs = uint64(frostPreSignAuthorizationMaximumInputs)

	inputCost, err := frostPreSignAnchoredInputCost(
		localSeats,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatal(err)
	}
	if inputCost.Revisions != 406 || inputCost.Generations != 815 {
		t.Fatalf("unexpected twenty-seat per-input cost [%+v]", inputCost)
	}

	harness := newTestFrostAnchorAdmissionHarness(
		frostNativeSignerAnchorCapacity{
			Revisions:   FrostNativeSignerAnchorMaximumHistoryEvents,
			Generations: FrostNativeSignerAnchorMaximumHistoryProofEntries,
		},
		localSeats,
	)

	signatures, err := executor.signBatchWithTaprootPolicy(
		ctx,
		messages,
		nil,
		0,
		nil,
		nil,
		nil,
		harness.admitInput(sweepInputs),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(signatures) != len(messages) {
		t.Fatalf(
			"signed [%d] of [%d] messages",
			len(signatures),
			len(messages),
		)
	}

	admits, failures := harness.counts()
	if admits != len(messages) {
		t.Fatalf(
			"anchor admission was asked [%d] times for [%d] inputs; the "+
				"reservation is not per input",
			admits,
			len(messages),
		)
	}
	if failures != 0 {
		t.Fatalf("[%d] admissions were refused on an unspent window", failures)
	}

	// The high-water mark is the whole point: whatever the batch size, the
	// node commits one input's worth at a time.
	if harness.peak != inputCost {
		t.Fatalf(
			"peak reservation [%+v], expected exactly one input's [%+v]; a "+
				"batch that holds more than one input's capacity at once puts "+
				"the seat ceiling back",
			harness.peak,
			inputCost,
		)
	}

	// Nothing may be left charged once the batch is done. A leaked reservation
	// is capacity no other wallet on this node can ever use again, because the
	// certified windows do not refill and the controller cannot notice an
	// owner that walked away.
	if reserved := harness.reserved(); reserved !=
		(frostNativeSignerAnchorCapacity{}) {
		t.Fatalf("[%+v] stayed reserved after the batch finished", reserved)
	}
}

// TestSigningExecutor_MidBatchAnchorExhaustionRefusesAndReleases pins the
// residual the per-input reservation accepts in exchange for removing the seat
// ceiling: a batch can be admitted for its first inputs and refused part way
// through. That has to be a clean refusal - named, counted, and holding
// nothing - rather than a partially applied reservation or a silent continue.
func TestSigningExecutor_MidBatchAnchorExhaustionRefusesAndReleases(
	t *testing.T,
) {
	resetFrostNativeSignerAnchorAdmissionMetricsForTest()
	t.Cleanup(resetFrostNativeSignerAnchorAdmissionMetricsForTest)

	executor := setupSigningExecutor(t)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	messages := []*big.Int{
		big.NewInt(1000),
		big.NewInt(2000),
		big.NewInt(3000),
	}

	harness := newTestFrostAnchorAdmissionHarness(
		frostNativeSignerAnchorCapacity{
			Revisions:   FrostNativeSignerAnchorMaximumHistoryEvents,
			Generations: FrostNativeSignerAnchorMaximumHistoryProofEntries,
		},
		20,
	)
	// The window runs out under the batch after its first input, exactly as it
	// would on a node whose anchor is due a rotation.
	harness.onAdmit = func(admits int) {
		if admits == 2 {
			harness.setHeadroom(frostNativeSignerAnchorCapacity{
				Revisions:   FrostNativeSignerAnchorMaximumHistoryEvents,
				Generations: 700,
			})
		}
	}

	signatures, err := executor.signBatchWithTaprootPolicy(
		ctx,
		messages,
		nil,
		0,
		nil,
		nil,
		nil,
		harness.admitInput(uint64(frostPreSignAuthorizationMaximumInputs)),
	)
	if err == nil {
		t.Fatal("a batch was signed through an exhausted proof window")
	}
	if signatures != nil {
		t.Fatalf(
			"a refused batch returned [%d] signatures; a partial batch must "+
				"never reach the caller",
			len(signatures),
		)
	}

	// The refusal has to say which input it gave up on and why, or an operator
	// sees a wallet action fail with no cause and no remedy.
	for _, want := range []string{
		"input [1]",
		"signer generations",
		"offline anchor rotation is required",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("mid-batch refusal is missing %q: [%v]", want, err)
		}
	}

	// The first input's reservation was taken and released before the second
	// was even attempted, and the failed one took nothing, so the controller
	// must be back to zero.
	if reserved := harness.reserved(); reserved !=
		(frostNativeSignerAnchorCapacity{}) {
		t.Fatalf(
			"[%+v] stayed reserved after a mid-batch refusal",
			reserved,
		)
	}

	admits, failures := harness.counts()
	if admits != 2 || failures != 1 {
		t.Fatalf(
			"admission was asked [%d] times with [%d] refusals; the loop must "+
				"stop at the first refused input",
			admits,
			failures,
		)
	}

	// The underlying cause is counted where it always was. The per-input
	// counter is what tells an operator the difference between a batch refused
	// before its authorization was relayed and one abandoned after.
	if unreserved :=
		frostNativeSignerAnchorUnreservedHeadroomRejections.Load(); unreserved != 1 {
		t.Fatalf(
			"unreserved-headroom rejections counted [%d], expected [1]",
			unreserved,
		)
	}
}

// TestSigningExecutor_ReleasesAnchorAdmissionOnEveryInputExitPath walks the
// ways one input can end other than by producing a signature. Each of them has
// to give the reservation back: the release is deferred precisely so that no
// future edit has to remember to add it to a new error return.
func TestSigningExecutor_ReleasesAnchorAdmissionOnEveryInputExitPath(
	t *testing.T,
) {
	executor := setupSigningExecutor(t)

	harness := newTestFrostAnchorAdmissionHarness(
		frostNativeSignerAnchorCapacity{
			Revisions:   FrostNativeSignerAnchorMaximumHistoryEvents,
			Generations: FrostNativeSignerAnchorMaximumHistoryProofEntries,
		},
		20,
	)
	admitInput := harness.admitInput(
		uint64(frostPreSignAuthorizationMaximumInputs),
	)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, test := range []struct {
		name            string
		ctx             context.Context
		roastKeyGroupID string
		unsignedTx      *bitcoin.TransactionBuilder
		wantMessage     string
	}{
		{
			// The policy binding is the first thing the reservation pays for,
			// and it can fail before any signer call happens.
			name:            "policy binding fails",
			ctx:             context.Background(),
			roastKeyGroupID: "",
			unsignedTx:      bitcoin.NewTransactionBuilder(newLocalBitcoinChain()),
			wantMessage:     "policy artifact",
		},
		{
			// A cancelled context is the ordinary way a signing window ends,
			// and it unwinds through the signing call rather than through a
			// returned error of the loop's own.
			name:            "signing context cancelled",
			ctx:             cancelledCtx,
			roastKeyGroupID: "",
			unsignedTx:      nil,
			wantMessage:     "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := harness.reserved()
			_, _, err := executor.signBatchInputUnderAnchorAdmission(
				test.ctx,
				0,
				big.NewInt(1000),
				nil,
				0,
				test.roastKeyGroupID,
				test.unsignedTx,
				nil,
				nil,
				admitInput,
			)
			if err == nil {
				t.Fatal("the input did not fail")
			}
			if test.wantMessage != "" &&
				!strings.Contains(err.Error(), test.wantMessage) {
				t.Fatalf("unexpected failure: [%v]", err)
			}
			if after := harness.reserved(); after != before {
				t.Fatalf(
					"the reservation was not released: [%+v] before, [%+v] "+
						"after",
					before,
					after,
				)
			}
		})
	}

	// A refused admission must take nothing at all, not take and then fail to
	// give back. Twenty local seats need 815 generations for one input, and
	// this leaves 500 - short of the cost but well clear of the rotation floor,
	// so the refusal is the unreserved-headroom one rather than the blanket
	// one.
	harness.setHeadroom(frostNativeSignerAnchorCapacity{
		Revisions:   FrostNativeSignerAnchorMaximumHistoryEvents,
		Generations: 500,
	})
	if _, _, err := executor.signBatchInputUnderAnchorAdmission(
		context.Background(),
		3,
		big.NewInt(1000),
		nil,
		0,
		"",
		nil,
		nil,
		nil,
		admitInput,
	); err == nil || !strings.Contains(err.Error(), "input [3]") {
		t.Fatalf("a refused admission was not reported against its input: [%v]", err)
	}
	if reserved := harness.reserved(); reserved !=
		(frostNativeSignerAnchorCapacity{}) {
		t.Fatalf("a refused admission left [%+v] reserved", reserved)
	}
}

// TestFrostNativeSignerAnchorAdmission_ConcurrentPerInputWorkflowsCannotOverCommit
// covers the property the whole controller exists for, under the interleaving
// that reserving per input makes possible: several wallets on one node,
// admitting and releasing inputs independently, must never between them promise
// more of a certified window than it has.
//
// Per-input reservations make the interleaving finer, not looser. Two wallets
// can now sit between each other's inputs where before one held the window for
// a whole batch, but each admission still takes the single controller mutex,
// still compares its cost against headroom minus everything currently reserved,
// and still either succeeds outright or fails outright - there is no waiting, so
// no deadlock and no livelock. What per-input reservations remove is the
// scenario where a large batch could never start at all.
func TestFrostNativeSignerAnchorAdmission_ConcurrentPerInputWorkflowsCannotOverCommit(
	t *testing.T,
) {
	// Sized so a twenty-seat and a five-seat wallet fit together (815+215) but
	// a second twenty-seat input does not.
	const headroomGenerations = uint64(1200)

	headroom := frostNativeSignerAnchorCapacity{
		Revisions:   FrostNativeSignerAnchorMaximumHistoryEvents,
		Generations: headroomGenerations,
	}

	var overCommitted string
	var overCommitMutex sync.Mutex
	controller := &frostNativeSignerAnchorAdmissionController{}
	controller.readHeadroom = func(
		context.Context,
	) (frostNativeSignerAnchorCapacity, error) {
		// readHeadroom runs while reserve() holds the admission mutex, so this
		// observes the reserved total at the one instant it is authoritative.
		if controller.reserved.Revisions > headroom.Revisions ||
			controller.reserved.Generations > headroom.Generations {
			overCommitMutex.Lock()
			overCommitted = fmt.Sprintf(
				"reserved [%+v] exceeds headroom [%+v]",
				controller.reserved,
				headroom,
			)
			overCommitMutex.Unlock()
		}
		return headroom, nil
	}

	snapshot := testFrostAnchorAdmissionReadinessSnapshot(
		FrostNativeSignerAnchorMaximumHistoryEvents,
		headroomGenerations,
	)

	// Two wallets of different sizes, plus the DKG paths that share this
	// controller, all cycling reservations against the same window.
	var waitGroup sync.WaitGroup
	var admitted, refused [3]int
	var resultMutex sync.Mutex
	for worker, seats := range []uint64{20, 5, 0} {
		waitGroup.Add(1)
		go func(worker int, seats uint64) {
			defer waitGroup.Done()
			for i := 0; i < 200; i++ {
				var reservation *frostNativeSignerAnchorRevisionReservation
				var err error
				if seats == 0 {
					// Native DKG shares the controller and must keep its own
					// semantics: two capacity units per local seat, refused by
					// the same accounting.
					reservation, err = controller.reserveDKG(
						context.Background(),
						4,
					)
				} else {
					reservation, err = controller.reservePreSign(
						context.Background(),
						snapshot,
						uint64(frostPreSignAuthorizationMaximumInputs),
						seats,
						signingAttemptsLimit,
					)
				}
				resultMutex.Lock()
				if err != nil {
					refused[worker]++
				} else {
					admitted[worker]++
				}
				resultMutex.Unlock()
				if err == nil {
					reservation.Release()
				}
			}
		}(worker, seats)
	}
	waitGroup.Wait()

	overCommitMutex.Lock()
	defer overCommitMutex.Unlock()
	if overCommitted != "" {
		t.Fatalf("concurrent admissions over-committed the window: %s", overCommitted)
	}
	if reserved := controller.reserved; reserved !=
		(frostNativeSignerAnchorCapacity{}) {
		t.Fatalf("[%+v] stayed reserved after every workflow finished", reserved)
	}
	// No worker may be shut out entirely. Per-input admissions all cost the
	// same regardless of batch size, so the only asymmetry left is between
	// wallets of different seat counts, and a window this size admits the
	// larger one too.
	for worker, count := range admitted {
		if count == 0 {
			t.Fatalf(
				"worker [%d] was never admitted in [%d] attempts; a workflow "+
					"that can never make progress is a starvation path",
				worker,
				refused[worker],
			)
		}
	}
}

// TestFrostPreSignAuthorizationGate_AdmitInputChargesOneInput covers the gate
// side of the split, including the thing that makes the hundred-seat case fit:
// the reservation authorize() takes to gate the on-chain relay must be released
// before the per-input admissions start, because the two are the same size and
// the window has room for one of them at that seat count.
func TestFrostPreSignAuthorizationGate_AdmitInputChargesOneInput(t *testing.T) {
	resetFrostNativeSignerAnchorAdmissionMetricsForTest()
	t.Cleanup(resetFrostNativeSignerAnchorAdmissionMetricsForTest)

	const seats = uint64(frostPreSignAuthorizationMaximumSeats)

	controller := &frostNativeSignerAnchorAdmissionController{
		readHeadroom: func(
			context.Context,
		) (frostNativeSignerAnchorCapacity, error) {
			return frostNativeSignerAnchorCapacity{
				Revisions:   FrostNativeSignerAnchorMaximumHistoryEvents,
				Generations: FrostNativeSignerAnchorMaximumHistoryProofEntries,
			}, nil
		},
	}
	localMemberIndexes := make([]group.MemberIndex, 0, seats)
	for i := uint64(1); i <= seats; i++ {
		localMemberIndexes = append(localMemberIndexes, group.MemberIndex(i))
	}
	gate := &thresholdFrostPreSignAuthorizationGate{
		anchorAdmission:    controller,
		localMemberIndexes: localMemberIndexes,
	}
	// An unset attempt limit must be read as the package default here exactly
	// as authorize reads it, or the two would charge different amounts.
	if gate.effectiveMaximumAttempts() != signingAttemptsLimit {
		t.Fatalf(
			"unset attempt limit read as [%d]",
			gate.effectiveMaximumAttempts(),
		)
	}

	signatureHashes := make([][32]byte, frostPreSignAuthorizationMaximumInputs)
	authorization := &frostPreSignAuthorization{
		proposal: &FrostPreSignAuthorizationProposal{
			Transaction: &FrostPreSignTransaction{
				SignatureHashes: signatureHashes,
			},
		},
	}

	// Admission before any readiness reconciliation must fail closed rather
	// than reserve against an unauthenticated view of the windows.
	if _, err := gate.admitInput(
		context.Background(),
		authorization,
	); err == nil || !strings.Contains(err.Error(), "reconciled signer readiness") {
		t.Fatalf("admission without reconciled readiness was allowed: [%v]", err)
	}

	authorization.cacheReadiness(
		FrostPreSignFinality{BlockNumber: 1},
		testFrostAnchorAdmissionReadinessSnapshot(
			FrostNativeSignerAnchorMaximumHistoryEvents,
			FrostNativeSignerAnchorMaximumHistoryProofEntries,
		),
	)

	// The relay gate: what authorize() holds while it relays and finalizes.
	relayReservation, err := controller.reservePreSign(
		context.Background(),
		testFrostAnchorAdmissionReadinessSnapshot(
			FrostNativeSignerAnchorMaximumHistoryEvents,
			FrostNativeSignerAnchorMaximumHistoryProofEntries,
		),
		uint64(frostPreSignAuthorizationMaximumInputs),
		seats,
		signingAttemptsLimit,
	)
	if err != nil {
		t.Fatalf("the relay gate itself was refused: [%v]", err)
	}

	// Held alongside a per-input admission, two inputs' worth is 8030 of a
	// 4096-entry window. This is exactly why walletTransactionExecutor hands
	// the reservation over instead of keeping it for the batch.
	if _, err := gate.admitInput(
		context.Background(),
		authorization,
	); err == nil || !strings.Contains(err.Error(), "temporary reservations") {
		t.Fatalf(
			"the relay reservation and a per-input admission were charged "+
				"together: [%v]",
			err,
		)
	}
	if rejections :=
		frostNativeSignerAnchorPreSignInputRejections.Load(); rejections != 1 {
		t.Fatalf(
			"per-input rejections counted [%d], expected [1]",
			rejections,
		)
	}

	// Handed over, the same node signs a full sweep at the wallet's entire seat
	// count - the case the batch-wide reservation could not admit at four
	// seats, let alone a hundred.
	relayReservation.Release()
	for input := 0; input < frostPreSignAuthorizationMaximumInputs; input++ {
		release, err := gate.admitInput(context.Background(), authorization)
		if err != nil {
			t.Fatalf("input [%d] of a full sweep was refused: [%v]", input, err)
		}
		if release == nil {
			t.Fatalf("input [%d] was admitted with no release", input)
		}
		release()
	}
	if reserved := func() frostNativeSignerAnchorCapacity {
		controller.mutex.Lock()
		defer controller.mutex.Unlock()
		return controller.reserved
	}(); reserved != (frostNativeSignerAnchorCapacity{}) {
		t.Fatalf("[%+v] stayed reserved after the sweep", reserved)
	}
}
