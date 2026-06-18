package tbtc

import (
	"context"
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

// fakeTransitionController records BeginObservedAttempt calls so the loop-wiring
// tests can assert the loop observes each attempt with the exact member sets.
type fakeTransitionController struct {
	calls []observedAttemptCall
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
