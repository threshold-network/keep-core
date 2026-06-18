package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func TestBeginOrchestrationForSession_DefaultBuildReturnsError(t *testing.T) {
	// In the default build, RegisteredRoastRetryCoordinator always
	// returns (zero, false), so the orchestration helper must
	// return an error directing the caller to fall back to legacy
	// behaviour. This guarantees no production caller can
	// accidentally "succeed" into orchestration when the build tag
	// is off.
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()

	ctx, err := attempt.NewAttemptContext(
		"session-default-build",
		"key-group",
		[]byte{0x01},
		[attempt.MessageDigestLength]byte{0x77},
		0,
		[]group.MemberIndex{1, 2, 3},
		nil,
	)
	if err != nil {
		t.Fatalf("ctx: %v", err)
	}

	_, _, err = BeginOrchestrationForSession("session-default-build", ctx, []byte{0x01, 0x02})
	if err == nil {
		t.Fatal("default build must return error from BeginOrchestrationForSession")
	}
}
