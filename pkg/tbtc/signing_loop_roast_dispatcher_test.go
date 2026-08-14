package tbtc

import (
	"errors"
	"reflect"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// Note: TestDefaultSigningParticipantSelector_IsLegacy below is
// build-tag-conditional (see _default_build_test.go); under
// frost_roast_retry the default is the ROAST selector and a
// dedicated test verifies that.

// recordingSelector counts how often Select was called and returns
// a fixed result. Tests use it to assert the dispatcher routes
// participant selection through the configured selector rather
// than the legacy path.
type recordingSelector struct {
	calls  int
	result []group.MemberIndex
	err    error
}

func (r *recordingSelector) Select(
	readyMembersIndexes []group.MemberIndex,
	_ chain.Addresses,
	_ int64,
	_ uint,
	_ uint,
	_ uint,
	_ string,
	_ group.MemberIndex,
	_ string,
) (participantSelection, error) {
	r.calls++
	if r.err != nil {
		return participantSelection{}, r.err
	}
	included := r.result
	if included == nil {
		included = readyMembersIndexes
	}
	return participantSelection{includedMembersIndexes: included}, nil
}

func TestLegacySigningParticipantSelector_DelegatesToRetryShuffle(t *testing.T) {
	operators := chain.Addresses{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
		chain.Address("op-4"),
		chain.Address("op-5"),
	}
	readyMembers := []group.MemberIndex{1, 2, 3, 4, 5}
	sel := legacySigningParticipantSelector{}
	// Args: ready, operators, seed, retryCount, roastAttemptNumber, honestThreshold,
	// sessionID, memberIndex.
	selection, err := sel.Select(readyMembers, operators, 42, 0, 0, 3, "session-x", 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := selection.includedMembersIndexes
	// Five members are ready and the honest threshold is 3, so the
	// qualification shuffle leaves exactly the threshold count included.
	if len(got) != 3 {
		t.Fatalf("expected 3 included members (the honest threshold), got %d", len(got))
	}
	// Each operator holds a single seat here, so EvaluateRetryParticipantsForSigning
	// stops at exactly honestThreshold qualified operators and the surplus trim never
	// fires. The two ready members it did not qualify are blameless -- they lost a
	// seeded shuffle -- so they must be reported as transiently parked, not folded
	// into the permanent excluded set.
	parked := selection.transientlyParkedMembersIndexes
	if len(parked) != 2 {
		t.Fatalf("expected 2 transiently parked members, got %v", parked)
	}
	// Parked and included must partition the ready set exactly.
	seen := make(map[group.MemberIndex]int, len(readyMembers))
	for _, memberIndex := range got {
		seen[memberIndex]++
	}
	for _, memberIndex := range parked {
		seen[memberIndex]++
	}
	for _, memberIndex := range readyMembers {
		if seen[memberIndex] != 1 {
			t.Fatalf(
				"ready member %d appears %d times across included+parked, want 1: "+
					"included %v, parked %v",
				memberIndex,
				seen[memberIndex],
				got,
				parked,
			)
		}
	}
	for i := 1; i < len(parked); i++ {
		if parked[i] <= parked[i-1] {
			t.Fatalf("expected parked members sorted ascending, got %v", parked)
		}
	}
	// The result must be sorted ascending and contain only ready members.
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("expected included members sorted ascending, got %v", got)
		}
	}
}

func TestLegacySigningParticipantSelector_PropagatesErrors(t *testing.T) {
	sel := legacySigningParticipantSelector{}
	_, err := sel.Select(
		[]group.MemberIndex{1},
		chain.Addresses{chain.Address("op-1")},
		0, 0, 0,
		99, // honest threshold higher than member count
		"session-x",
		1,
		"",
	)
	if err == nil {
		t.Fatal("expected error from retry shuffle")
	}
}

func TestSigningRetryLoopUsesDispatcher(t *testing.T) {
	sentinel := []group.MemberIndex{1, 2, 3}
	recorder := &recordingSelector{result: sentinel}

	srl := &signingRetryLoop{
		signingGroupOperators: chain.Addresses{
			chain.Address("op-1"),
			chain.Address("op-2"),
			chain.Address("op-3"),
			chain.Address("op-4"),
			chain.Address("op-5"),
		},
		groupParameters: &GroupParameters{
			HonestThreshold: 3,
		},
		attemptCounter:      1,
		attemptSeed:         42,
		participantSelector: recorder,
	}

	included, excluded, _, err := srl.performMembersSelection(
		[]group.MemberIndex{1, 2, 3, 4, 5},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recorder.calls != 1 {
		t.Fatalf("expected dispatcher to be called once; got %d", recorder.calls)
	}
	if !reflect.DeepEqual(included, sentinel) {
		t.Fatalf(
			"expected the dispatcher's included set (the sentinel %v), got %v",
			sentinel, included,
		)
	}
	// Excluded is the complement of the sentinel over the 5-member group.
	wantExcluded := []group.MemberIndex{4, 5}
	if !reflect.DeepEqual(excluded, wantExcluded) {
		t.Fatalf("expected excluded %v, got %v", wantExcluded, excluded)
	}
}

func TestSigningRetryLoopPropagatesSelectorError(t *testing.T) {
	wantErr := errors.New("synthetic selector failure")
	srl := &signingRetryLoop{
		signingGroupOperators: chain.Addresses{
			chain.Address("op-1"),
			chain.Address("op-2"),
		},
		groupParameters:     &GroupParameters{HonestThreshold: 2},
		attemptCounter:      1,
		attemptSeed:         0,
		participantSelector: &recordingSelector{err: wantErr},
	}
	_, _, _, err := srl.performMembersSelection([]group.MemberIndex{1, 2})
	if err == nil {
		t.Fatal("expected selector error to propagate")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped sentinel; got %v", err)
	}
}
