//go:build !frost_roast_retry

package signing

import "testing"

func TestRoastRetryRegistration_DefaultBuildIsStub(t *testing.T) {
	// Register a non-zero dependency set. Because the default build
	// is a no-op stub, the registry must remain empty.
	deps := RoastRetryDeps{SelfMember: 7}
	RegisterRoastRetryCoordinator(deps)
	got, ok := RegisteredRoastRetryCoordinator()
	if ok {
		t.Fatalf("default build must report not-registered; got ok=true, deps=%+v", got)
	}
	if got != (RoastRetryDeps{}) {
		t.Fatalf("default build must return zero value; got %+v", got)
	}
}

func TestRoastRetryRegistration_DefaultBuildResetIsNoOp(t *testing.T) {
	// Reset should not panic even though there is no real state.
	ResetRoastRetryRegistrationForTest()
	if _, ok := RegisteredRoastRetryCoordinator(); ok {
		t.Fatal("default build registry should remain empty after reset")
	}
}
