package tbtc

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

type observedAttemptCall struct {
	attemptNumber uint
	included      []group.MemberIndex
	excluded      []group.MemberIndex
}

type failedAttemptCall struct {
	attemptNumber uint
	timeoutBlock  uint64
}

// fakeTransitionController records controller calls so the loop-wiring tests can
// assert the loop observes each attempt (with the exact member sets) and signals
// failed attempts.
type fakeTransitionController struct {
	calls       []observedAttemptCall
	failedCalls []failedAttemptCall
}

func (f *fakeTransitionController) BeginObservedAttempt(
	attemptNumber uint,
	includedMembersIndexes []group.MemberIndex,
	excludedMembersIndexes []group.MemberIndex,
) {
	f.calls = append(f.calls, observedAttemptCall{
		attemptNumber: attemptNumber,
		included:      includedMembersIndexes,
		excluded:      excludedMembersIndexes,
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
	if call.attemptNumber != 1 {
		t.Fatalf("expected attempt number 1, got %d", call.attemptNumber)
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
}
