//go:build !frost_roast_retry

package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
)

func TestRoastTransitionRegistry_DefaultBuildIsNoOp(t *testing.T) {
	// In the default build the registry is a permanent stub:
	// RecordRoastTransition discards; RoastTransitionForSession always returns
	// (zero, false). The ROAST selector must therefore always fall back to
	// legacy retry in the default build.
	RecordRoastTransition(
		"session-default-build-test",
		1,
		RoastTransitionRecord{Bundle: &roast.TransitionMessage{}},
	)
	got, ok := RoastTransitionForSession("session-default-build-test", 1)
	if ok {
		t.Fatalf("default build registry must report not-present; got record %v", got)
	}
	if got.Bundle != nil {
		t.Fatalf("default build must return a zero record; got bundle %v", got.Bundle)
	}

	// Clear and reset must not panic.
	ClearRoastTransitionForSession("session-default-build-test", 1)
	ResetRoastTransitionRegistryForTest()
}
