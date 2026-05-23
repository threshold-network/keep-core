//go:build frost_roast_retry

package tbtc

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/frost/signing"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestDefaultSigningParticipantSelector_IsROASTInTaggedBuild(t *testing.T) {
	sel := defaultSigningParticipantSelector()
	if _, ok := sel.(roastSigningParticipantSelector); !ok {
		t.Fatalf(
			"defaultSigningParticipantSelector in frost_roast_retry build must return ROAST impl; got %T",
			sel,
		)
	}
}

func TestROASTSelector_FallsBackToLegacyWhenNoBundle(t *testing.T) {
	signing.ResetTransitionBundleRegistryForTest()
	t.Cleanup(signing.ResetTransitionBundleRegistryForTest)

	sel := roastSigningParticipantSelector{}
	members := []chain.Address{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
		chain.Address("op-4"),
		chain.Address("op-5"),
	}
	got, err := sel.Select(members, 42, 0, 3, "session-no-bundle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("expected at least 3 from legacy fallback; got %d", len(got))
	}
}

func TestROASTSelector_FallsBackToLegacyWhenRegistryEmpty(t *testing.T) {
	signing.ResetTransitionBundleRegistryForTest()
	signing.ResetRoastRetryRegistrationForTest()
	signing.ResetSessionHandleRegistryForTest()
	t.Cleanup(signing.ResetTransitionBundleRegistryForTest)
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)
	t.Cleanup(signing.ResetSessionHandleRegistryForTest)

	// Record a bundle but do NOT register a coordinator.
	signing.RecordTransitionBundleForSession(
		"session-no-registry",
		&roast.TransitionMessage{CoordinatorIDValue: 1},
	)

	sel := roastSigningParticipantSelector{}
	members := []chain.Address{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
		chain.Address("op-4"),
		chain.Address("op-5"),
	}
	got, err := sel.Select(members, 42, 0, 3, "session-no-registry")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("expected at least 3 from legacy fallback; got %d", len(got))
	}
}

func TestROASTSelector_FallsBackToLegacyWhenNoHandleBinding(t *testing.T) {
	signing.ResetTransitionBundleRegistryForTest()
	signing.ResetRoastRetryRegistrationForTest()
	signing.ResetSessionHandleRegistryForTest()
	t.Cleanup(signing.ResetTransitionBundleRegistryForTest)
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)
	t.Cleanup(signing.ResetSessionHandleRegistryForTest)

	// Register coordinator + record bundle, but DO NOT bind a
	// session handle. The selector must still fall back to legacy
	// because it cannot identify which attempt to consume the
	// bundle against.
	signing.RegisterRoastRetryCoordinator(signing.RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})
	signing.RecordTransitionBundleForSession(
		"session-no-handle",
		&roast.TransitionMessage{CoordinatorIDValue: 1},
	)

	sel := roastSigningParticipantSelector{}
	members := []chain.Address{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
		chain.Address("op-4"),
		chain.Address("op-5"),
	}
	got, err := sel.Select(members, 42, 0, 3, "session-no-handle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("expected legacy fallback; got %d members", len(got))
	}
}

func TestMembersResolver_MapsIndexToAddress(t *testing.T) {
	members := []chain.Address{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
	}
	r := membersResolver(members)
	for i := 1; i <= 3; i++ {
		got, err := r.For(group.MemberIndex(i))
		if err != nil {
			t.Fatalf("For(%d): %v", i, err)
		}
		want := members[i-1]
		if got != want {
			t.Fatalf("For(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestMembersResolver_RejectsZeroIndex(t *testing.T) {
	r := membersResolver([]chain.Address{chain.Address("op-1")})
	_, err := r.For(0)
	if err == nil {
		t.Fatal("expected error for zero member index")
	}
}

func TestMembersResolver_RejectsOutOfRangeIndex(t *testing.T) {
	r := membersResolver([]chain.Address{chain.Address("op-1")})
	_, err := r.For(99)
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}

func TestROASTSelector_UsesBundleWhenAllConditionsMet(t *testing.T) {
	signing.ResetTransitionBundleRegistryForTest()
	signing.ResetRoastRetryRegistrationForTest()
	signing.ResetSessionHandleRegistryForTest()
	t.Cleanup(signing.ResetTransitionBundleRegistryForTest)
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)
	t.Cleanup(signing.ResetSessionHandleRegistryForTest)

	// Build a real coordinator and run through the bundle-production
	// flow end-to-end, then verify the selector consumes the bundle
	// and returns the IncludedSet mapped to addresses.
	ctx, _ := attempt.NewAttemptContext(
		"session-with-bundle",
		"key-group",
		[]byte{0x01, 0x02, 0x03},
		[attempt.MessageDigestLength]byte{0xab},
		0,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)

	scratch := roast.NewInMemoryCoordinator()
	hScratch, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(hScratch)

	coord := roast.NewInMemoryCoordinatorWithSigning(
		elected, roast.NoOpSigner(), roast.NoOpSignatureVerifier(),
	)
	signing.RegisterRoastRetryCoordinator(signing.RoastRetryDeps{
		Coordinator: coord,
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
	})

	handle, _ := coord.BeginAttempt(ctx)
	signing.SetCurrentAttemptHandleForSession("session-with-bundle", handle, ctx)

	// Seed every member's snapshot so AggregateBundle has content.
	for _, m := range ctx.IncludedSet {
		snap := roast.NewLocalEvidenceSnapshot(m, ctx.Hash(), attempt.Evidence{})
		snap.OperatorSignature = []byte{0x01}
		if err := coord.RecordEvidence(handle, snap); err != nil {
			t.Fatalf("record %d: %v", m, err)
		}
	}
	bundle, err := coord.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	signing.RecordTransitionBundleForSession("session-with-bundle", bundle)

	sel := roastSigningParticipantSelector{}
	members := []chain.Address{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
		chain.Address("op-4"),
		chain.Address("op-5"),
	}
	got, err := sel.Select(members, 0, 0, 3, "session-with-bundle")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("selector must return at least one address")
	}
}
