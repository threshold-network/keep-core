//go:build frost_native && !frost_roast_retry

package signing

import (
	"testing"
)

func TestVerifyMessageAttemptContextHash_DefaultBuildPassesEverything(t *testing.T) {
	// Without the frost_roast_retry tag, currentAttemptHandleForCollect
	// always returns ok=false, so the helper short-circuits to nil
	// for every input. This guarantees that the receive-loop wiring
	// never enforces the AttemptContextHash binding in the default
	// build, matching the rollback promise made in the rollout
	// guide (docs/development/frost-roast-retry-rollout.adoc).
	msg := stubDefaultBuildMessage{}
	if err := verifyMessageAttemptContextHash(msg, "any-session", 1); err != nil {
		t.Fatalf(
			"default build must always pass; got %v",
			err,
		)
	}
}

// stubDefaultBuildMessage is the equivalent of the tagged-build
// test's stubMessage. Kept separate to avoid the tagged-build
// definition leaking into this build's compilation unit.
type stubDefaultBuildMessage struct{}

func (stubDefaultBuildMessage) GetAttemptContextHash() (
	[AttemptContextHashFieldLength]byte, bool,
) {
	return [AttemptContextHashFieldLength]byte{}, false
}
