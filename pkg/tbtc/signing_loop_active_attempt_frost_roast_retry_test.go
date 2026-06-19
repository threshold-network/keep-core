//go:build frost_roast_retry

package tbtc

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/keep-network/keep-core/internal/testutils"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// TestSigningRetryLoop_ActiveAttemptUsesCommittedRoastNumber asserts that under
// active ROAST retry the active signing attempt keys its number off the COMMITTED
// roast attempt index (roastAttemptNumber+1), NOT the block-paced attemptCounter,
// so the active signing AttemptContext is byte-identical to the observe/transition
// context the loop built for the same committed attempt (Codex's PR2b-1b review
// P1: otherwise the two bind to different context hashes after a skip).
//
// A pre-selection minority-readiness skip makes attemptCounter (2) diverge from
// the committed roast number (1); the active attempt must report 1, not 2.
func TestSigningRetryLoop_ActiveAttemptUsesCommittedRoastNumber(t *testing.T) {
	t.Setenv(signing.RoastRetryReadinessOptInEnvVar, "true")
	signing.ResetRoastRetryRegistrationForTest()
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)
	signing.RegisterRoastRetryCoordinator(signing.RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})
	if !signing.RoastRetryActive() {
		t.Fatal("precondition: ROAST retry must be active for this test")
	}

	message := big.NewInt(100)
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	announcer := &mockSigningAnnouncer{
		outgoingAnnouncements: make(map[string]group.MemberIndex),
		incomingAnnouncementsFn: func(sessionID string) ([]group.MemberIndex, error) {
			// Loop attempt 1: minority readiness (< honest threshold) -> skipped
			// BEFORE selection, so attemptCounter advances but roastAttemptNumber
			// does not. Loop attempt 2: full -> committed (attemptCounter=2, committed
			// roast index 0 -> active number 1).
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

	var captured *signingAttemptParams
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return nil },
		func() (uint64, error) { return 200, nil },
		func(params *signingAttemptParams) (*signing.Result, uint64, error) {
			captured = params
			return testResult, 215, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("the committed signing attempt was never executed")
	}
	// The block-paced attemptCounter is 2 (one skip + one commit); the committed
	// roast number is 1. The active attempt must use the committed number so its
	// context matches the observe context the loop built at committed index 0.
	if captured.number != 1 {
		t.Fatalf(
			"active attempt must key off the committed roast number (1), not the "+
				"block-paced attemptCounter (2); got %d",
			captured.number,
		)
	}
}

// parkingSelector is a participant selector that returns a fixed included +
// transiently-parked set, so a loop test can assert the parking threads from
// selection through signingAttemptParams to the active attempt without standing
// up a full transition record.
type parkingSelector struct {
	included []group.MemberIndex
	parked   []group.MemberIndex
}

func (s parkingSelector) Select(
	_ []group.MemberIndex,
	_ chain.Addresses,
	_ int64,
	_ uint,
	_ uint,
	_ uint,
	_ string,
	_ group.MemberIndex,
) (participantSelection, error) {
	return participantSelection{
		includedMembersIndexes:          s.included,
		transientlyParkedMembersIndexes: s.parked,
	}, nil
}

// TestSigningRetryLoop_ActiveAttemptCarriesParking asserts the transiently-parked
// set produced by selection threads through signingAttemptParams to the active
// signing attempt (and thus onto its AttemptContext), so the active context's
// parking matches the observe context's. A regression dropping the field would
// make a one-attempt park permanent (the B1 hazard) on the active path.
func TestSigningRetryLoop_ActiveAttemptCarriesParking(t *testing.T) {
	t.Setenv(signing.RoastRetryReadinessOptInEnvVar, "true")
	signing.ResetRoastRetryRegistrationForTest()
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)
	signing.RegisterRoastRetryCoordinator(signing.RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	message := big.NewInt(100)
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
		message,
		"",
		200,
		1,
		chain.Addresses{"address-1", "address-2", "address-3"},
		&GroupParameters{GroupSize: 3, HonestThreshold: 3},
		announcer,
		doneCheck,
	)
	// Park member 3; keep the local seat (1) included so it reaches the active
	// attempt.
	retryLoop.participantSelector = parkingSelector{
		included: []group.MemberIndex{1, 2},
		parked:   []group.MemberIndex{3},
	}

	var captured *signingAttemptParams
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return nil },
		func() (uint64, error) { return 200, nil },
		func(params *signingAttemptParams) (*signing.Result, uint64, error) {
			captured = params
			return testResult, 215, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("the committed signing attempt was never executed")
	}
	if len(captured.transientlyParkedMembersIndexes) != 1 ||
		captured.transientlyParkedMembersIndexes[0] != 3 {
		t.Fatalf(
			"active attempt must carry the parked set [3]; got %v",
			captured.transientlyParkedMembersIndexes,
		)
	}
}

// TestSigningRetryLoop_ActiveAttemptUsesAttemptCounterWhenRoastInactive asserts
// the legacy invariant is preserved: with ROAST retry inactive (no coordinator
// registered), the active attempt keys off the block-paced attemptCounter
// unchanged, even after a skip.
func TestSigningRetryLoop_ActiveAttemptUsesAttemptCounterWhenRoastInactive(t *testing.T) {
	// Readiness opted in but NO coordinator registered -> RoastRetryActive is false.
	t.Setenv(signing.RoastRetryReadinessOptInEnvVar, "true")
	signing.ResetRoastRetryRegistrationForTest()
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)
	if signing.RoastRetryActive() {
		t.Fatal("precondition: ROAST retry must be inactive without a coordinator")
	}

	message := big.NewInt(100)
	testResult := &signing.Result{
		Signature: mustFrostSignatureFromBigInts(big.NewInt(300), big.NewInt(400)),
	}
	announcer := &mockSigningAnnouncer{
		outgoingAnnouncements: make(map[string]group.MemberIndex),
		incomingAnnouncementsFn: func(sessionID string) ([]group.MemberIndex, error) {
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

	var captured *signingAttemptParams
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := retryLoop.start(
		ctx,
		func(context.Context, uint64) error { return nil },
		func() (uint64, error) { return 200, nil },
		func(params *signingAttemptParams) (*signing.Result, uint64, error) {
			captured = params
			return testResult, 215, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == nil {
		t.Fatal("the committed signing attempt was never executed")
	}
	// Legacy: the active attempt keys off attemptCounter (2 after one skip).
	if captured.number != 2 {
		t.Fatalf(
			"with ROAST inactive the active attempt must use the block-paced "+
				"attemptCounter (2); got %d",
			captured.number,
		)
	}
}
