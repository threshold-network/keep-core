//go:build !frost_roast_retry

package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
)

func TestTransitionBundleRegistry_DefaultBuildIsNoOp(t *testing.T) {
	// In the default build the registry is a permanent stub:
	// RecordTransitionBundleForSession discards; TransitionBundleForSession
	// always returns (nil, false). The ROAST selector must therefore
	// always fall back to legacy retry in the default build.
	RecordTransitionBundleForSession(
		"session-default-build-test",
		&roast.TransitionMessage{},
	)
	got, ok := TransitionBundleForSession("session-default-build-test")
	if ok {
		t.Fatalf(
			"default build registry must report not-present; got bundle %v",
			got,
		)
	}
	if got != nil {
		t.Fatalf("default build must return nil bundle; got %v", got)
	}

	// Clear and reset must not panic.
	ClearTransitionBundleForSession("session-default-build-test")
	ResetTransitionBundleRegistryForTest()
}
