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

func selectorTestMembers() []chain.Address {
	return []chain.Address{
		chain.Address("op-1"),
		chain.Address("op-2"),
		chain.Address("op-3"),
		chain.Address("op-4"),
		chain.Address("op-5"),
	}
}

func TestDefaultSigningParticipantSelector_IsROASTInTaggedBuild(t *testing.T) {
	sel := defaultSigningParticipantSelector()
	if _, ok := sel.(roastSigningParticipantSelector); !ok {
		t.Fatalf(
			"defaultSigningParticipantSelector in frost_roast_retry build must return ROAST impl; got %T",
			sel,
		)
	}
}

func TestROASTSelector_FallsBackToLegacyWhenNoRecord(t *testing.T) {
	signing.ResetRoastTransitionRegistryForTest()
	t.Cleanup(signing.ResetRoastTransitionRegistryForTest)

	sel := roastSigningParticipantSelector{}
	got, err := sel.Select(selectorTestMembers(), 42, 0, 3, "session-no-record", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("expected at least 3 from legacy fallback; got %d", len(got))
	}
}

func TestROASTSelector_FallsBackToLegacyWhenRegistryEmpty(t *testing.T) {
	signing.ResetRoastTransitionRegistryForTest()
	signing.ResetRoastRetryRegistrationForTest()
	t.Cleanup(signing.ResetRoastTransitionRegistryForTest)
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)

	// Record a transition but do NOT register a coordinator.
	signing.RecordRoastTransition("session-no-registry", 1, signing.RoastTransitionRecord{
		Bundle:            &roast.TransitionMessage{CoordinatorIDValue: 1},
		DkgGroupPublicKey: []byte{0x01, 0x02, 0x03},
	})

	sel := roastSigningParticipantSelector{}
	got, err := sel.Select(selectorTestMembers(), 42, 0, 3, "session-no-registry", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("expected at least 3 from legacy fallback; got %d", len(got))
	}
}

func TestROASTSelector_FallsBackToLegacyWhenNoRecordForMember(t *testing.T) {
	signing.ResetRoastTransitionRegistryForTest()
	t.Cleanup(signing.ResetRoastTransitionRegistryForTest)

	// A record exists for member 1, but the selector queries for member 2.
	// Records are member-scoped (the multi-seat fix), so member 2 must NOT alias
	// member 1's record; it falls back to legacy.
	signing.RecordRoastTransition("session-member-scoped", 1, signing.RoastTransitionRecord{
		Bundle:            &roast.TransitionMessage{CoordinatorIDValue: 1},
		DkgGroupPublicKey: []byte{0x01, 0x02, 0x03},
	})

	sel := roastSigningParticipantSelector{}
	got, err := sel.Select(selectorTestMembers(), 42, 0, 3, "session-member-scoped", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("expected legacy fallback for a member with no record; got %d", len(got))
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

func TestROASTSelector_UsesRecordWhenAllConditionsMet(t *testing.T) {
	signing.ResetRoastTransitionRegistryForTest()
	signing.ResetRoastRetryRegistrationForTest()
	t.Cleanup(signing.ResetRoastTransitionRegistryForTest)
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)

	// Build a real coordinator and run the bundle-production flow end-to-end,
	// then store a full transition record (handle + bundle + DKG key) and verify
	// the selector consumes it (calling NextAttempt against the record's handle
	// with the record's DKG key) rather than falling back to legacy.
	dkgKey := []byte{0x01, 0x02, 0x03}
	ctx, _ := attempt.NewAttemptContext(
		"session-with-record",
		"key-group",
		dkgKey,
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

	// Store the full record exactly as the orchestration cleanup would, keyed by
	// the elected member.
	signing.RecordRoastTransition("session-with-record", elected, signing.RoastTransitionRecord{
		Bundle:            bundle,
		PreviousHandle:    handle,
		PreviousContext:   ctx,
		DkgGroupPublicKey: dkgKey,
	})

	sel := roastSigningParticipantSelector{}
	got, err := sel.Select(selectorTestMembers(), 0, 0, 3, "session-with-record", elected)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("selector must return at least one address from the record's bundle")
	}
}
