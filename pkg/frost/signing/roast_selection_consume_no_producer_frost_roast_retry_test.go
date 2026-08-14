//go:build frost_roast_retry && !frost_native

package signing

import (
	"errors"
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
)

// TestConsumeRoastTransitionForSelection_FallbackWhenNoProducer asserts that in a
// frost_roast_retry build WITHOUT frost_native -- where the participant selector
// and the coordinator registry exist but nothing PRODUCES transition records (the
// observe step, exchange, and aggregation are frost_native && frost_roast_retry) --
// a retry falls back to the uniform legacy shuffle instead of fail-closing against
// a transition record that can never be created (Codex P2-1). Without the producer
// gate in RoastRetryActive this would fail closed and stall the signing.
func TestConsumeRoastTransitionForSelection_FallbackWhenNoProducer(t *testing.T) {
	if roastTransitionProducerAvailable() {
		t.Fatal("precondition: this build (no frost_native) must have no producer")
	}

	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	// roastAttemptNumber 1 (> 0): with a producer this expects a transition and
	// fail-closes when none exists; without a producer it must fall back to legacy
	// (the deterministic, group-wide outcome every no-producer node reaches).
	_, _, err := ConsumeRoastTransitionForSelection("session", 1, 1, 3, "")
	if !errors.Is(err, ErrRoastSelectionFallBackToLegacy) {
		t.Fatalf("a no-producer build must fall back to legacy, not fail closed; got %v", err)
	}
}
