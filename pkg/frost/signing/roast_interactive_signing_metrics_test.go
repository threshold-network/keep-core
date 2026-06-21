package signing

import (
	"context"
	"math/big"
	"testing"

	"github.com/ipfs/go-log/v2"
)

func TestInteractiveSigningMetrics_RecordFunctionsIncrement(t *testing.T) {
	resetInteractiveSigningMetricsForTest()
	t.Cleanup(resetInteractiveSigningMetricsForTest)

	recordInteractiveSigningSuccess()
	recordInteractiveSigningSuccess()
	recordInteractiveSigningFailure()
	recordCoarseFallbackRefused()
	recordCoarseFallbackRefused()
	recordCoarseFallbackRefused()

	if got := interactiveSigningSuccessEvents.Load(); got != 2 {
		t.Fatalf("success counter: got %d want 2", got)
	}
	if got := interactiveSigningFailureEvents.Load(); got != 1 {
		t.Fatalf("failure counter: got %d want 1", got)
	}
	if got := coarseFallbackRefusedEvents.Load(); got != 3 {
		t.Fatalf("coarse-refused counter: got %d want 3", got)
	}
}

func TestInteractiveSigningMetrics_CoarseRefusalAtLegacyBackendIncrements(t *testing.T) {
	// End-to-end wiring: the legacy backend's interactive-only terminal refusal bumps
	// the flip-safety counter (the signal that a node is configured interactive-only
	// but is still being asked to sign coarsely).
	resetInteractiveSigningMetricsForTest()
	t.Cleanup(resetInteractiveSigningMetricsForTest)
	t.Setenv(InteractiveSigningOnlyEnvVar, "true")

	_, err := newLegacyExecutionBackend().Execute(
		context.Background(), log.Logger("test"), &Request{Message: big.NewInt(1)},
	)
	if err == nil {
		t.Fatal("expected the interactive-only terminal refusal")
	}
	if got := coarseFallbackRefusedEvents.Load(); got != 1 {
		t.Fatalf("coarse-refused counter after a legacy refusal: got %d want 1", got)
	}
}

func TestRegisterInteractiveSigningMetrics_NilRegistryIsNoOp(t *testing.T) {
	RegisterInteractiveSigningMetrics(nil) // must not panic
}
