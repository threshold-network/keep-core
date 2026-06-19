//go:build frost_roast_retry

package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func cleanupTestContext(t *testing.T, members []group.MemberIndex) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		"orchestration-cleanup-test",
		"key-group",
		[]byte{0x01, 0x02, 0x03},
		[attempt.MessageDigestLength]byte{0xab},
		0,
		members,
		nil,
	)
	if err != nil {
		t.Fatalf("ctx: %v", err)
	}
	return ctx
}

// TestCleanup_ClearsBindingAndProducesNoTransitionRecord pins the RFC-21
// Phase 7.3 PR2b-1b producer retirement: the orchestration cleanup clears the
// per-attempt handle binding and, even on the elected coordinator with recorded
// evidence (the exact case the old producer aggregated), produces NO transition
// record -- the session-scoped transition exchange is the sole producer now, so
// a second drive-handle producer here would risk a divergent record.
func TestCleanup_ClearsBindingAndProducesNoTransitionRecord(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	ctx := cleanupTestContext(t, []group.MemberIndex{1, 2, 3, 4, 5})

	// Determine the elected coordinator and register a coordinator bound to it,
	// so the retired producer's elected==member precondition WOULD have held.
	scratch := roast.NewInMemoryCoordinator()
	scratchHandle, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(scratchHandle)
	coord := roast.NewInMemoryCoordinatorWithSigning(
		elected, roast.NoOpSigner(), roast.NoOpSignatureVerifier(),
	)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
	})

	const sessionID = "cleanup-no-record-session"
	handle, cleanup, err := BeginOrchestrationForSession(sessionID, elected, ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	// Seed the evidence the old producer would have aggregated.
	snap := roast.NewLocalEvidenceSnapshot(elected, ctx.Hash(), attempt.Evidence{})
	snap.OperatorSignature = []byte{0x01}
	if err := coord.RecordEvidence(handle, snap); err != nil {
		t.Fatalf("record evidence: %v", err)
	}

	cleanup()

	if _, ok := RoastTransitionForSession(ctx.SessionID, elected); ok {
		t.Fatal("cleanup must not produce a transition record (the exchange is the sole producer)")
	}
	if _, _, ok := currentAttemptHandleForCollect(sessionID); ok {
		t.Fatal("cleanup must clear the per-attempt handle binding")
	}
}
