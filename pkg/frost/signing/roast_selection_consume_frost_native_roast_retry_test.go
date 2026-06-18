//go:build frost_native && frost_roast_retry

package signing

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func signedForcedSnapshot(
	member group.MemberIndex,
	hash [attempt.MessageDigestLength]byte,
) *roast.LocalEvidenceSnapshot {
	snap := roast.NewLocalEvidenceSnapshot(member, hash, attempt.Evidence{})
	payload, _ := snap.SignableBytes()
	sig, _ := fixedSigner{}.Sign(payload)
	snap.OperatorSignature = sig
	return snap
}

func resetSelectionRegistries(t *testing.T) {
	t.Helper()
	ResetRoastRetryRegistrationForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)
}

func TestConsumeRoastTransitionForSelection_FallbackInitialAttempt(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	if _, err := ConsumeRoastTransitionForSelection("session", 1, 0, 3); !errors.Is(
		err, ErrRoastSelectionFallBackToLegacy,
	) {
		t.Fatalf("retry 0 must request the legacy fallback, got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_FallbackInactiveRoast(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	// No coordinator registered -> inactive -> uniform legacy fallback.
	if _, err := ConsumeRoastTransitionForSelection("session", 1, 1, 3); !errors.Is(
		err, ErrRoastSelectionFallBackToLegacy,
	) {
		t.Fatalf("inactive ROAST must request the legacy fallback, got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_FailsClosedNoRecord(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	// Active ROAST, a retry, but no record -> a transition was expected -> fail
	// closed (NOT the legacy-fallback sentinel).
	_, err := ConsumeRoastTransitionForSelection("session", 1, 1, 3)
	if err == nil || errors.Is(err, ErrRoastSelectionFallBackToLegacy) {
		t.Fatalf("a missing expected transition must fail closed, got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_FailsClosedStaleRecord(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	// A record fresh for retry 1 (prev attempt 0) but we select for retry 3.
	prevCtx := newExchangeTestContext(t, "session", []group.MemberIndex{1, 2, 3}, []byte{0x01})
	RecordRoastTransition("session", 1, RoastTransitionRecord{
		Bundle:          &roast.TransitionMessage{CoordinatorIDValue: 1},
		PreviousContext: prevCtx,
	})

	_, err := ConsumeRoastTransitionForSelection("session", 1, 3, 3)
	if err == nil || errors.Is(err, ErrRoastSelectionFallBackToLegacy) {
		t.Fatalf("a stale record must fail closed, got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_ConsumesFreshRecord(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	roastSessionID := "consume-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x0a, 0x0b}
	prevCtx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := prevCtx.Hash()

	// Determine the deterministically elected coordinator for prevCtx.
	probe := roast.NewInMemoryCoordinatorWithSigning(0, fixedSigner{}, roast.NoOpSignatureVerifier())
	probeHandle, err := probe.BeginAttempt(prevCtx)
	if err != nil {
		t.Fatalf("probe begin attempt: %v", err)
	}
	elected, err := probe.SelectedCoordinator(probeHandle)
	if err != nil {
		t.Fatalf("selected coordinator: %v", err)
	}

	// Register a coordinator bound to the elected member and build a real bundle.
	coord := roast.NewInMemoryCoordinatorWithSigning(
		elected, fixedSigner{}, roast.NoOpSignatureVerifier(),
	)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      fixedSigner{},
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
	})
	handle, err := coord.BeginAttempt(prevCtx)
	if err != nil {
		t.Fatalf("begin attempt: %v", err)
	}
	for _, m := range included {
		if err := coord.RecordEvidence(handle, signedForcedSnapshot(m, hash)); err != nil {
			t.Fatalf("record evidence for member %d: %v", m, err)
		}
	}
	bundle, err := coord.AggregateBundle(handle)
	if err != nil {
		t.Fatalf("aggregate bundle: %v", err)
	}
	RecordRoastTransition(roastSessionID, elected, RoastTransitionRecord{
		Bundle:            bundle,
		PreviousHandle:    handle,
		PreviousContext:   prevCtx,
		DkgGroupPublicKey: dkgKey,
	})

	// Consume for retry 1 (prevCtx.AttemptNumber 0 + 1 == 1).
	includedSet, err := ConsumeRoastTransitionForSelection(roastSessionID, elected, 1, 3)
	if err != nil {
		t.Fatalf("consume must succeed for a fresh record: %v", err)
	}
	// Every included member submitted a proof-of-attendance snapshot, so none is
	// silence-parked: the next included set equals the prior included set.
	if len(includedSet) != len(included) {
		t.Fatalf("expected the full included set %v, got %v", included, includedSet)
	}
}
