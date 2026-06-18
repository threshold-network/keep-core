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

	if _, _, err := ConsumeRoastTransitionForSelection("session", 1, 0, 3); !errors.Is(
		err, ErrRoastSelectionFallBackToLegacy,
	) {
		t.Fatalf("retry 0 must request the legacy fallback, got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_FallbackInactiveRoast(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	// No coordinator registered -> inactive -> uniform legacy fallback.
	if _, _, err := ConsumeRoastTransitionForSelection("session", 1, 1, 3); !errors.Is(
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
	_, _, err := ConsumeRoastTransitionForSelection("session", 1, 1, 3)
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

	_, _, err := ConsumeRoastTransitionForSelection("session", 1, 3, 3)
	if err == nil || errors.Is(err, ErrRoastSelectionFallBackToLegacy) {
		t.Fatalf("a stale record must fail closed, got %v", err)
	}
}

func TestConsumeRoastTransitionForSelection_CarriesParking(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	resetSelectionRegistries(t)

	roastSessionID := "consume-parking-session"
	included := []group.MemberIndex{1, 2, 3}
	dkgKey := []byte{0x0c}
	prevCtx := newExchangeTestContext(t, roastSessionID, included, dkgKey)
	hash := prevCtx.Hash()

	probe := roast.NewInMemoryCoordinatorWithSigning(0, fixedSigner{}, roast.NoOpSignatureVerifier())
	probeHandle, _ := probe.BeginAttempt(prevCtx)
	elected, _ := probe.SelectedCoordinator(probeHandle)

	// Pick a parked member that is NOT the elected coordinator, so the coordinator
	// is in the bundle and can aggregate; omit its forced snapshot so NextAttempt
	// silence-parks it (absent-from-bundle -> transient park).
	var parkedMember group.MemberIndex
	for _, m := range included {
		if m != elected {
			parkedMember = m
			break
		}
	}

	coord := roast.NewInMemoryCoordinatorWithSigning(
		elected, fixedSigner{}, roast.NoOpSignatureVerifier(),
	)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      fixedSigner{},
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
	})
	handle, _ := coord.BeginAttempt(prevCtx)
	for _, m := range included {
		if m == parkedMember {
			continue // omit -> silence-parked by NextAttempt
		}
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

	// threshold 2 so the 2-member next set stays feasible.
	includedSet, parked, err := ConsumeRoastTransitionForSelection(roastSessionID, elected, 1, 2)
	if err != nil {
		t.Fatalf("consume must succeed: %v", err)
	}
	// The absent member is carried as parked (reinstated next attempt, not
	// permanently excluded) and is not in the included set.
	if len(parked) != 1 || parked[0] != parkedMember {
		t.Fatalf("expected parked [%d], got %v", parkedMember, parked)
	}
	for _, m := range includedSet {
		if m == parkedMember {
			t.Fatalf("parked member %d must not be in the included set %v", parkedMember, includedSet)
		}
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

	// Consume for roast attempt 1 (prevCtx.AttemptNumber 0 + 1 == 1).
	includedSet, parked, err := ConsumeRoastTransitionForSelection(roastSessionID, elected, 1, 3)
	if err != nil {
		t.Fatalf("consume must succeed for a fresh record: %v", err)
	}
	// Every included member submitted a proof-of-attendance snapshot, so none is
	// silence-parked: the next included set equals the prior included set and the
	// parked set is empty.
	if len(includedSet) != len(included) {
		t.Fatalf("expected the full included set %v, got %v", included, includedSet)
	}
	if len(parked) != 0 {
		t.Fatalf("expected no parked members, got %v", parked)
	}
}
