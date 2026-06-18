//go:build frost_roast_retry

package tbtc

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/frost/roast"
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

// The initial attempt (retry 0) has no prior transition, so the ROAST selector
// uses the legacy retry shuffle -- a uniform decision every honest node makes.
func TestROASTSelector_InitialAttemptUsesLegacy(t *testing.T) {
	signing.ResetRoastRetryRegistrationForTest()
	signing.ResetRoastTransitionRegistryForTest()
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)
	t.Cleanup(signing.ResetRoastTransitionRegistryForTest)

	sel := roastSigningParticipantSelector{}
	// Args: ready, operators, seed, retryCount, roastAttemptNumber, honestThreshold,
	// sessionID, memberIndex. roastAttemptNumber 0 == the initial ROAST attempt.
	got, err := sel.Select(
		[]group.MemberIndex{1, 2, 3, 4, 5},
		selectorTestMembers(),
		42, 0, 0, 3, "session", 1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.includedMembersIndexes) != 3 {
		t.Fatalf(
			"expected a legacy-shaped included set (the honest threshold); got %d",
			len(got.includedMembersIndexes),
		)
	}
}

// On a retry under ACTIVE ROAST retry, a transition is expected; when none
// arrived the selector must FAIL CLOSED (surface an error that terminates the
// loop) rather than fall back to legacy -- mixed selection across honest nodes
// is the fracture class. C3 activates this consumption.
func TestROASTSelector_FailsClosedWhenTransitionMissing(t *testing.T) {
	t.Setenv(signing.RoastRetryReadinessOptInEnvVar, "true")
	signing.ResetRoastRetryRegistrationForTest()
	signing.ResetRoastTransitionRegistryForTest()
	t.Cleanup(signing.ResetRoastRetryRegistrationForTest)
	t.Cleanup(signing.ResetRoastTransitionRegistryForTest)
	signing.RegisterRoastRetryCoordinator(signing.RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	sel := roastSigningParticipantSelector{}
	// roastAttemptNumber 1 (> 0) under active ROAST expects a transition; none is
	// stored, so the selector must fail closed.
	_, err := sel.Select(
		[]group.MemberIndex{1, 2, 3, 4, 5},
		selectorTestMembers(),
		42, 1, 1, 3, "session", 1,
	)
	if err == nil {
		t.Fatal("expected a fail-closed error when an expected transition is missing")
	}
	if errors.Is(err, signing.ErrRoastSelectionFallBackToLegacy) {
		t.Fatal("a missing expected transition must fail closed, not fall back to legacy")
	}
}
