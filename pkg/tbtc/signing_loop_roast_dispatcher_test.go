package tbtc

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// recordingSelector counts how often Select was called and returns
// a fixed result. Tests use it to assert the dispatcher routes
// participant selection through the configured selector rather
// than the legacy path.
type recordingSelector struct {
	calls  int
	result []chain.Address
	err    error
}

func (r *recordingSelector) Select(
	members []chain.Address,
	_ int64,
	_ uint,
	_ uint,
	_ string,
) ([]chain.Address, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if r.result != nil {
		return r.result, nil
	}
	return members, nil
}

func TestDefaultSigningParticipantSelector_IsLegacy(t *testing.T) {
	sel := defaultSigningParticipantSelector()
	if _, ok := sel.(legacySigningParticipantSelector); !ok {
		t.Fatalf(
			"defaultSigningParticipantSelector must return legacy implementation; got %T",
			sel,
		)
	}
}

func TestLegacySigningParticipantSelector_DelegatesToRetryShuffle(t *testing.T) {
	members := []chain.Address{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
		chain.Address("op-4"),
		chain.Address("op-5"),
	}
	sel := legacySigningParticipantSelector{}
	got, err := sel.Select(members, 42, 0, 3, "session-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("expected at least 3 qualified operators, got %d", len(got))
	}
}

func TestLegacySigningParticipantSelector_PropagatesErrors(t *testing.T) {
	sel := legacySigningParticipantSelector{}
	_, err := sel.Select(
		[]chain.Address{chain.Address("op-1")},
		0, 0,
		99, // honest threshold higher than member count
		"session-x",
	)
	if err == nil {
		t.Fatal("expected error from retry shuffle")
	}
}

func TestSigningRetryLoopUsesDispatcher(t *testing.T) {
	sentinel := []chain.Address{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
	}
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

	set, err := srl.qualifiedOperatorsSet([]group.MemberIndex{1, 2, 3, 4, 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recorder.calls != 1 {
		t.Fatalf("expected dispatcher to be called once; got %d", recorder.calls)
	}
	if len(set) != len(sentinel) {
		t.Fatalf(
			"expected %d qualified operators (the sentinel), got %d",
			len(sentinel), len(set),
		)
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
	_, err := srl.qualifiedOperatorsSet([]group.MemberIndex{1, 2})
	if err == nil {
		t.Fatal("expected selector error to propagate")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped sentinel; got %v", err)
	}
}
