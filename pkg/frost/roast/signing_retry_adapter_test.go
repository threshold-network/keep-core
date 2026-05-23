package roast

import (
	"errors"
	"fmt"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// addressResolverString is a deterministic resolver that maps
// member index N to the string "addr-N". Used by the adapter
// tests to verify the conversion path without needing chain types.
type addressResolverString struct{}

func (addressResolverString) For(m group.MemberIndex) (string, error) {
	if m == 0 {
		return "", fmt.Errorf("zero member index")
	}
	return fmt.Sprintf("addr-%d", m), nil
}

// failingResolver always errors. Used to verify that resolver
// failures propagate cleanly through the adapter.
type failingResolver struct{ err error }

func (f failingResolver) For(_ group.MemberIndex) (string, error) {
	return "", f.err
}

// retryAdapterFixture provides a previously-completed attempt with
// a verified bundle that NextAttempt can transition from.
type retryAdapterFixture struct {
	coord     Coordinator
	handle    AttemptHandle
	bundle    *TransitionMessage
	threshold uint
	dkgPub    []byte
}

func newRetryAdapterFixture(t *testing.T) *retryAdapterFixture {
	t.Helper()
	members := []group.MemberIndex{1, 2, 3, 4, 5}

	// Use a throwaway coordinator to discover the elected
	// coordinator, then build a real coordinator bound to that
	// member as the aggregator.
	scratch := NewInMemoryCoordinator()
	ctx := mustBuildContext(t, members, nil, nil)
	h0, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(h0)

	aggregator := newSignedCoordinatorForMember(elected)
	handle, err := aggregator.BeginAttempt(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	for _, m := range members {
		snap := signSnapshotForTest(t, NewLocalEvidenceSnapshot(m, ctx.Hash(), attempt.Evidence{}))
		if err := aggregator.RecordEvidence(handle, snap); err != nil {
			t.Fatalf("record %d: %v", m, err)
		}
	}
	bundle, err := aggregator.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	return &retryAdapterFixture{
		coord:     aggregator,
		handle:    handle,
		bundle:    bundle,
		threshold: 3,
		dkgPub:    []byte{0xab, 0xcd, 0xef},
	}
}

func mustBuildContext(
	t *testing.T,
	included, excluded, parked []group.MemberIndex,
) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContextWithParking(
		"session-test",
		"key-group-test",
		[]byte{0xab, 0xcd, 0xef},
		[attempt.MessageDigestLength]byte{0x42},
		0,
		included,
		excluded,
		parked,
	)
	if err != nil {
		t.Fatalf("build ctx: %v", err)
	}
	return ctx
}

func TestEvaluateRoastRetryForSigning_HappyPath(t *testing.T) {
	f := newRetryAdapterFixture(t)

	addresses, nextCtx, err := EvaluateRoastRetryForSigning[string](
		f.coord, f.handle, f.bundle, f.threshold, f.dkgPub,
		addressResolverString{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(addresses) != 5 {
		t.Fatalf("expected 5 addresses, got %d", len(addresses))
	}
	for i, a := range addresses {
		want := fmt.Sprintf("addr-%d", nextCtx.IncludedSet[i])
		if a != want {
			t.Fatalf(
				"address[%d]: got %q want %q",
				i, a, want,
			)
		}
	}
	if nextCtx.AttemptNumber != 1 {
		t.Fatalf("attempt number: got %d want 1", nextCtx.AttemptNumber)
	}
}

func TestEvaluateRoastRetryForSigning_PropagatesInfeasibility(t *testing.T) {
	f := newRetryAdapterFixture(t)

	_, _, err := EvaluateRoastRetryForSigning[string](
		f.coord, f.handle, f.bundle, 99, f.dkgPub,
		addressResolverString{},
	)
	if !errors.Is(err, ErrAttemptInfeasible) {
		t.Fatalf("expected ErrAttemptInfeasible, got %v", err)
	}
}

func TestEvaluateRoastRetryForSigning_PropagatesResolverError(t *testing.T) {
	f := newRetryAdapterFixture(t)

	sentinel := errors.New("resolver lookup failed")
	_, _, err := EvaluateRoastRetryForSigning[string](
		f.coord, f.handle, f.bundle, f.threshold, f.dkgPub,
		failingResolver{err: sentinel},
	)
	if err == nil {
		t.Fatal("expected resolver error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
}

func TestEvaluateRoastRetryForSigning_RejectsNilCoordinator(t *testing.T) {
	_, _, err := EvaluateRoastRetryForSigning[string](
		nil, AttemptHandle{}, &TransitionMessage{}, 3, []byte{0x01},
		addressResolverString{},
	)
	if err == nil {
		t.Fatal("expected nil-coordinator error")
	}
}

func TestEvaluateRoastRetryForSigning_RejectsNilResolver(t *testing.T) {
	_, _, err := EvaluateRoastRetryForSigning[string](
		NewInMemoryCoordinator(),
		AttemptHandle{}, &TransitionMessage{}, 3, []byte{0x01},
		nil,
	)
	if err == nil {
		t.Fatal("expected nil-resolver error")
	}
}

func TestSigningRetryAdapter_LegacyShapeMatchesPureFunction(t *testing.T) {
	f := newRetryAdapterFixture(t)
	resolver := addressResolverString{}

	adapter := SigningRetryAdapter[string]{
		Coordinator:       f.coord,
		Handle:            f.handle,
		Bundle:            f.bundle,
		Threshold:         f.threshold,
		DkgGroupPublicKey: f.dkgPub,
		Resolver:          resolver,
	}

	// Legacy parameters are ignored.
	viaAdapter, err := adapter.EvaluateRetryParticipantsForSigning(
		nil, 0, 0, 0,
	)
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}
	viaFunc, _, err := EvaluateRoastRetryForSigning[string](
		f.coord, f.handle, f.bundle, f.threshold, f.dkgPub, resolver,
	)
	if err != nil {
		t.Fatalf("function: %v", err)
	}
	if len(viaAdapter) != len(viaFunc) {
		t.Fatalf(
			"adapter and function disagree on participant count: %d vs %d",
			len(viaAdapter), len(viaFunc),
		)
	}
	for i := range viaAdapter {
		if viaAdapter[i] != viaFunc[i] {
			t.Fatalf("adapter[%d] = %q, function[%d] = %q", i, viaAdapter[i], i, viaFunc[i])
		}
	}
}

func TestSigningRetryAdapter_NextAttemptContextRoundTrip(t *testing.T) {
	f := newRetryAdapterFixture(t)
	adapter := SigningRetryAdapter[string]{
		Coordinator:       f.coord,
		Handle:            f.handle,
		Bundle:            f.bundle,
		Threshold:         f.threshold,
		DkgGroupPublicKey: f.dkgPub,
		Resolver:          addressResolverString{},
	}
	ctx1, err := adapter.NextAttemptContext()
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	ctx2, err := adapter.NextAttemptContext()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if ctx1.Hash() != ctx2.Hash() {
		t.Fatal("NextAttemptContext must be deterministic across calls")
	}
}

func TestSigningRetryAdapter_PropagatesInfeasibility(t *testing.T) {
	f := newRetryAdapterFixture(t)
	adapter := SigningRetryAdapter[string]{
		Coordinator:       f.coord,
		Handle:            f.handle,
		Bundle:            f.bundle,
		Threshold:         99,
		DkgGroupPublicKey: f.dkgPub,
		Resolver:          addressResolverString{},
	}
	_, err := adapter.EvaluateRetryParticipantsForSigning(nil, 0, 0, 0)
	if !errors.Is(err, ErrAttemptInfeasible) {
		t.Fatalf("expected ErrAttemptInfeasible, got %v", err)
	}
}
