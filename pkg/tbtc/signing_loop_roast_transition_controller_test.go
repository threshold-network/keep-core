package tbtc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type observedAttemptCall struct {
	roastAttemptNumber uint
	included           []group.MemberIndex
	excluded           []group.MemberIndex
	parked             []group.MemberIndex
}

type failedAttemptCall struct {
	attemptNumber uint
	timeoutBlock  uint64
}

// fakeTransitionController records controller calls so the loop-wiring tests can
// assert the loop observes each attempt (with the exact member sets), signals
// failed and succeeded attempts, and consults lost-sync before selection.
type fakeTransitionController struct {
	calls          []observedAttemptCall
	failedCalls    []failedAttemptCall
	succeededCalls int
	// lostSync is consumed by ConsumeLostSync; a test sets it to drive the loop's
	// skip-attempt-before-selection path.
	lostSync bool
}

func (f *fakeTransitionController) BeginObservedAttempt(
	roastAttemptNumber uint,
	includedMembersIndexes []group.MemberIndex,
	excludedMembersIndexes []group.MemberIndex,
	transientlyParkedMembersIndexes []group.MemberIndex,
) {
	f.calls = append(f.calls, observedAttemptCall{
		roastAttemptNumber: roastAttemptNumber,
		included:           includedMembersIndexes,
		excluded:           excludedMembersIndexes,
		parked:             transientlyParkedMembersIndexes,
	})
}

func (f *fakeTransitionController) OnAttemptFailed(
	attemptNumber uint,
	timeoutBlock uint64,
) {
	f.failedCalls = append(f.failedCalls, failedAttemptCall{
		attemptNumber: attemptNumber,
		timeoutBlock:  timeoutBlock,
	})
}

func (f *fakeTransitionController) OnAttemptSucceeded() {
	f.succeededCalls++
}

func (f *fakeTransitionController) ConsumeLostSync() bool {
	if !f.lostSync {
		return false
	}
	f.lostSync = false

	return true
}

func newObserveTestRetryLoop(
	announcer signingAnnouncer,
	doneCheck signingDoneCheckStrategy,
) *signingRetryLoop {
	return newSigningRetryLoop(
		&testutils.MockLogger{},
		big.NewInt(100),
		"",  // roastSessionID
		200, // initialStartBlock
		1,   // signingGroupMemberIndex
		chain.Addresses{
			"address-1",
			"address-2",
			"address-3",
			"address-4",
			"address-5",
		},
		&GroupParameters{GroupSize: 5, HonestThreshold: 3},
		announcer,
		doneCheck,
	)
}

func runOneSuccessfulAttempt(
	t *testing.T,
	retryLoop *signingRetryLoop,
) {
	t.Helper()
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return nil },
		func() (uint64, error) { return 200, nil },
		func(*signingAttemptParams) (*signing.Result, uint64, error) {
			return testResult, 215, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestSigningRetryLoop_ObservesEachAttempt asserts the loop drives the
// transition controller's observe step once per attempt, with the exact
// member-level included/excluded sets selection produced -- the binding the
// later commits' transition exchange and selection consume.
func TestSigningRetryLoop_ObservesEachAttempt(t *testing.T) {
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	announcer := &mockSigningAnnouncer{
		outgoingAnnouncements: make(map[string]group.MemberIndex),
		incomingAnnouncementsFn: func(string) ([]group.MemberIndex, error) {
			return []group.MemberIndex{1, 2, 3, 4, 5}, nil
		},
	}
	doneCheck := &mockSigningDoneCheck{
		waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
			return testResult, 215, nil
		},
	}

	retryLoop := newObserveTestRetryLoop(announcer, doneCheck)
	controller := &fakeTransitionController{}
	retryLoop.setTransitionController(controller)

	runOneSuccessfulAttempt(t, retryLoop)

	if len(controller.calls) != 1 {
		t.Fatalf("expected BeginObservedAttempt called once, got %d", len(controller.calls))
	}
	call := controller.calls[0]
	if call.roastAttemptNumber != 0 {
		t.Fatalf("expected the first committed roast attempt number 0, got %d", call.roastAttemptNumber)
	}
	// The observed sets must partition the whole group and match the honest
	// threshold count -- i.e. the loop passes selection's exact member-level
	// output, not a recomputed approximation.
	if len(call.included) != 3 {
		t.Fatalf("expected 3 included members (the honest threshold), got %v", call.included)
	}
	if len(call.included)+len(call.excluded) != 5 {
		t.Fatalf(
			"included+excluded must cover the group of 5, got %d+%d",
			len(call.included), len(call.excluded),
		)
	}
}

// TestSigningRetryLoop_NilTransitionControllerIsSafe asserts a loop with no
// controller installed (the default build / non-ROAST deployment) runs an
// attempt without panicking.
func TestSigningRetryLoop_NilTransitionControllerIsSafe(t *testing.T) {
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	announcer := &mockSigningAnnouncer{
		outgoingAnnouncements: make(map[string]group.MemberIndex),
		incomingAnnouncementsFn: func(string) ([]group.MemberIndex, error) {
			return []group.MemberIndex{1, 2, 3, 4, 5}, nil
		},
	}
	doneCheck := &mockSigningDoneCheck{
		waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
			return testResult, 215, nil
		},
	}

	retryLoop := newObserveTestRetryLoop(announcer, doneCheck)
	// No setTransitionController -> transitionController stays nil.
	runOneSuccessfulAttempt(t, retryLoop)
}

// TestSigningRetryLoop_SignalsFailedAttempt asserts the loop signals
// OnAttemptFailed (with the attempt's timeout block) when a committed attempt
// fails, and only for the failed attempt. A 3-of-3 group keeps the local seat
// included on every attempt, so it reaches the committed-failure path.
func TestSigningRetryLoop_SignalsFailedAttempt(t *testing.T) {
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	announcer := &mockSigningAnnouncer{
		outgoingAnnouncements: make(map[string]group.MemberIndex),
		incomingAnnouncementsFn: func(string) ([]group.MemberIndex, error) {
			return []group.MemberIndex{1, 2, 3}, nil
		},
	}
	doneCheck := &mockSigningDoneCheck{
		waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
			return testResult, 215, nil
		},
	}
	retryLoop := newSigningRetryLoop(
		&testutils.MockLogger{},
		big.NewInt(100),
		"",
		200,
		1,
		chain.Addresses{"address-1", "address-2", "address-3"},
		&GroupParameters{GroupSize: 3, HonestThreshold: 3},
		announcer,
		doneCheck,
	)
	controller := &fakeTransitionController{}
	retryLoop.setTransitionController(controller)

	attempts := 0
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return nil },
		func() (uint64, error) { return 200, nil },
		func(*signingAttemptParams) (*signing.Result, uint64, error) {
			attempts++
			if attempts == 1 {
				return nil, 0, errors.New("synthetic attempt failure")
			}
			return testResult, 215, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(controller.failedCalls) != 1 {
		t.Fatalf("expected OnAttemptFailed called once, got %d", len(controller.failedCalls))
	}
	if controller.failedCalls[0].attemptNumber != 1 {
		t.Fatalf(
			"expected failed attempt number 1, got %d",
			controller.failedCalls[0].attemptNumber,
		)
	}
	// Every attempt that reaches selection is observed (the failed one and the
	// successful retry).
	if len(controller.calls) != 2 {
		t.Fatalf("expected BeginObservedAttempt called twice, got %d", len(controller.calls))
	}
	// The committed ROAST attempt number advances 0 -> 1 across the two committed
	// attempts (no pre-selection skip here), so the transition chain is consecutive.
	if controller.calls[0].roastAttemptNumber != 0 || controller.calls[1].roastAttemptNumber != 1 {
		t.Fatalf(
			"committed roast attempt numbers must advance 0,1; got %d,%d",
			controller.calls[0].roastAttemptNumber, controller.calls[1].roastAttemptNumber,
		)
	}
}

// TestSigningRetryLoop_SkipDoesNotAdvanceRoastAttemptNumber asserts the committed
// ROAST attempt number advances only on attempts that reach selection/observe,
// not on the pre-selection skips (here a minority-readiness skip). The committed
// attempt after a skip must still be roast attempt 0 -- otherwise the transition
// chain would expect a record for an attempt that never ran (Codex B2).
func TestSigningRetryLoop_SkipDoesNotAdvanceRoastAttemptNumber(t *testing.T) {
	message := big.NewInt(100)
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	announcer := &mockSigningAnnouncer{
		outgoingAnnouncements: make(map[string]group.MemberIndex),
		incomingAnnouncementsFn: func(sessionID string) ([]group.MemberIndex, error) {
			// Loop attempt 1: minority readiness (< honest threshold) -> skipped
			// BEFORE selection/observe. Loop attempt 2: full -> committed.
			if sessionID == fmt.Sprintf("%v-%v", message, 1) {
				return []group.MemberIndex{1, 2}, nil
			}
			return []group.MemberIndex{1, 2, 3}, nil
		},
	}
	doneCheck := &mockSigningDoneCheck{
		waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
			return testResult, 215, nil
		},
	}
	retryLoop := newSigningRetryLoop(
		&testutils.MockLogger{},
		message,
		"",
		200,
		1,
		chain.Addresses{"address-1", "address-2", "address-3"},
		&GroupParameters{GroupSize: 3, HonestThreshold: 3},
		announcer,
		doneCheck,
	)
	controller := &fakeTransitionController{}
	retryLoop.setTransitionController(controller)

	runOneSuccessfulAttempt(t, retryLoop)

	// Only the committed (loop attempt 2) attempt was observed; the minority skip
	// did not.
	if len(controller.calls) != 1 {
		t.Fatalf("expected 1 observed (committed) attempt, got %d", len(controller.calls))
	}
	if controller.calls[0].roastAttemptNumber != 0 {
		t.Fatalf(
			"the committed attempt after a skip must be roast attempt 0, got %d",
			controller.calls[0].roastAttemptNumber,
		)
	}
}

// TestSigningRetryLoop_LocalSuccessSignalsSucceeded asserts the loop signals
// OnAttemptSucceeded -- and NOT OnAttemptFailed -- when a committed attempt the
// seat participated in completes successfully. The succeeded signal clears the
// observe binding so no failure transition can be synthesized for an attempt this
// seat won (Codex B3). A 3-of-3 group keeps the local seat included.
func TestSigningRetryLoop_LocalSuccessSignalsSucceeded(t *testing.T) {
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	announcer := &mockSigningAnnouncer{
		outgoingAnnouncements: make(map[string]group.MemberIndex),
		incomingAnnouncementsFn: func(string) ([]group.MemberIndex, error) {
			return []group.MemberIndex{1, 2, 3}, nil
		},
	}
	doneCheck := &mockSigningDoneCheck{
		waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
			return testResult, 215, nil
		},
	}
	retryLoop := newSigningRetryLoop(
		&testutils.MockLogger{},
		big.NewInt(100),
		"",
		200,
		1,
		chain.Addresses{"address-1", "address-2", "address-3"},
		&GroupParameters{GroupSize: 3, HonestThreshold: 3},
		announcer,
		doneCheck,
	)
	controller := &fakeTransitionController{}
	retryLoop.setTransitionController(controller)

	runOneSuccessfulAttempt(t, retryLoop)

	if controller.succeededCalls != 1 {
		t.Fatalf("expected OnAttemptSucceeded called once, got %d", controller.succeededCalls)
	}
	if len(controller.failedCalls) != 0 {
		t.Fatalf("a successful attempt must not signal OnAttemptFailed, got %d", len(controller.failedCalls))
	}
}

// TestSigningRetryLoop_DoneCheckFailureAfterSuccessDoesNotSignalFailure asserts
// that when the protocol round succeeds locally but the done-check then fails, the
// loop signals OnAttemptSucceeded (clearing the observe binding) and NEVER
// OnAttemptFailed -- it must not synthesize a failure transition for an attempt
// whose signature aggregated (Codex B3, adjudicated to mark-succeeded + fail-closed
// over synthesizing a no-reject transition). Here the done-check fails on the first
// attempt and succeeds on the retry; both attempts succeeded locally.
func TestSigningRetryLoop_DoneCheckFailureAfterSuccessDoesNotSignalFailure(t *testing.T) {
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	announcer := &mockSigningAnnouncer{
		outgoingAnnouncements: make(map[string]group.MemberIndex),
		incomingAnnouncementsFn: func(string) ([]group.MemberIndex, error) {
			return []group.MemberIndex{1, 2, 3}, nil
		},
	}
	doneCheck := &mockSigningDoneCheck{
		waitUntilAllDoneOutcomeFn: func(attemptNumber uint64) (*signing.Result, uint64, error) {
			// The protocol round (signingAttemptFn) always succeeds; the done-check
			// coordination fails on the first attempt only.
			if attemptNumber == 1 {
				return nil, 0, errors.New("synthetic done-check failure after local success")
			}
			return testResult, 215, nil
		},
	}
	retryLoop := newSigningRetryLoop(
		&testutils.MockLogger{},
		big.NewInt(100),
		"",
		200,
		1,
		chain.Addresses{"address-1", "address-2", "address-3"},
		&GroupParameters{GroupSize: 3, HonestThreshold: 3},
		announcer,
		doneCheck,
	)
	controller := &fakeTransitionController{}
	retryLoop.setTransitionController(controller)

	runOneSuccessfulAttempt(t, retryLoop)

	// Both attempts' protocol rounds succeeded locally, so each signals succeeded.
	if controller.succeededCalls != 2 {
		t.Fatalf("expected OnAttemptSucceeded for each locally-succeeded attempt (2), got %d", controller.succeededCalls)
	}
	// A done-check failure after a local success must NOT be reported as an attempt
	// failure -- no failure transition is honest for an attempt that aggregated.
	if len(controller.failedCalls) != 0 {
		t.Fatalf(
			"a done-check failure after local success must not signal OnAttemptFailed, got %d",
			len(controller.failedCalls),
		)
	}
}

// TestSigningRetryLoop_LostSyncSkipsOneAttempt asserts the loop skips a single
// attempt -- before selection and before observing -- when the controller reports
// lost ROAST sync (its listener received a transition for an attempt this seat
// never observed), then carries on. Selecting from a stale position would diverge
// from peers (the fracture class), so the attempt must be abandoned; but the
// triggering bundle is unverifiable on arrival, so any authenticated member can
// produce one. Aborting the session here would hand a single unverifiable message
// a session kill.
func TestSigningRetryLoop_LostSyncSkipsOneAttempt(t *testing.T) {
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	announcer := &mockSigningAnnouncer{
		outgoingAnnouncements: make(map[string]group.MemberIndex),
		incomingAnnouncementsFn: func(string) ([]group.MemberIndex, error) {
			return []group.MemberIndex{1, 2, 3, 4, 5}, nil
		},
	}
	doneCheck := &mockSigningDoneCheck{
		waitUntilAllDoneOutcomeFn: func(uint64) (*signing.Result, uint64, error) {
			return testResult, 215, nil
		},
	}
	retryLoop := newObserveTestRetryLoop(announcer, doneCheck)
	controller := &fakeTransitionController{lostSync: true}
	retryLoop.setTransitionController(controller)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return nil },
		func() (uint64, error) { return 200, nil },
		func(*signingAttemptParams) (*signing.Result, uint64, error) {
			return testResult, 215, nil
		},
	)
	// The session must survive: one unobserved-attempt bundle costs one attempt.
	if err != nil {
		t.Fatalf("lost sync must not abort the signing session, got %v", err)
	}
	if result == nil {
		t.Fatal("expected the attempt after the skipped one to produce a result")
	}
	// Two announcement rounds ran (the announcement phase precedes the lost-sync
	// check, and each attempt announces under its own session id), proving the first
	// attempt was reached and then skipped rather than never attempted.
	if len(announcer.outgoingAnnouncements) != 2 {
		t.Fatalf(
			"expected 2 announcement rounds (one skipped attempt, one committed), got %d",
			len(announcer.outgoingAnnouncements),
		)
	}
	// The skip happens BEFORE selection/observe, so only the second attempt is
	// observed. Observing the skipped attempt would put a stale record on the
	// transition chain.
	if len(controller.calls) != 1 {
		t.Fatalf(
			"expected exactly 1 observed attempt (the skipped one must not be observed), got %d",
			len(controller.calls),
		)
	}
}
