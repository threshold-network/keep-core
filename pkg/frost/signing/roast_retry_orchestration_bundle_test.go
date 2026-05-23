//go:build frost_roast_retry

package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// signingForBundleContext constructs an attempt context whose
// SelectCoordinator will deterministically pick member 1 (for the
// sake of this test). Real production deployments use the
// rotating selection; here we pin a stable handle for assertion.
func signingForBundleContext(t *testing.T, members []group.MemberIndex) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		"orchestration-bundle-test",
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

// realCoordinatorForBundleTest returns an in-memory coordinator
// with NoOp signer/verifier so AggregateBundle path runs end-to-
// end without crypto setup. The coordinator's selfMember is the
// elected coordinator computed from the test context, so
// maybeProduceTransitionBundle invokes AggregateBundle.
func realCoordinatorForBundleTest(
	t *testing.T,
	ctx attempt.AttemptContext,
) (roast.Coordinator, group.MemberIndex) {
	t.Helper()
	scratch := roast.NewInMemoryCoordinator()
	hScratch, _ := scratch.BeginAttempt(ctx)
	elected, _ := scratch.SelectedCoordinator(hScratch)
	coord := roast.NewInMemoryCoordinatorWithSigning(
		elected,
		roast.NoOpSigner(),
		roast.NoOpSignatureVerifier(),
	)
	return coord, elected
}

func TestCleanup_ProducesBundleWhenElectedCoordinator(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	ResetTransitionBundleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetTransitionBundleRegistryForTest)

	ctx := signingForBundleContext(t, []group.MemberIndex{1, 2, 3, 4, 5})
	coord, elected := realCoordinatorForBundleTest(t, ctx)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
	})

	const sessionID = "bundle-producer-session"
	handle, cleanup, err := BeginOrchestrationForSession(sessionID, ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	// Seed at least one snapshot so AggregateBundle's
	// non-empty-bundle validation passes.
	snap := roast.NewLocalEvidenceSnapshot(elected, ctx.Hash(), attempt.Evidence{})
	// NoOpSigner returns empty bytes but the signature-verification
	// pre-check rejects zero-length signatures. Provide a dummy
	// non-empty signature; the NoOp verifier accepts any byte
	// sequence.
	snap.OperatorSignature = []byte{0x01}
	if err := coord.RecordEvidence(handle, snap); err != nil {
		t.Fatalf("record evidence: %v", err)
	}

	// Cleanup must produce + record a bundle (we're the elected
	// coordinator and the attempt is still Collecting).
	cleanup()

	bundle, ok := TransitionBundleForSession(sessionID)
	if !ok {
		t.Fatal("elected coordinator's cleanup must produce a bundle")
	}
	if bundle == nil {
		t.Fatal("recorded bundle must not be nil")
	}
	if bundle.CoordinatorID() != elected {
		t.Fatalf(
			"bundle coordinator id %d != elected %d",
			bundle.CoordinatorID(), elected,
		)
	}
}

func TestCleanup_DoesNotProduceBundleWhenNotElectedCoordinator(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	ResetTransitionBundleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetTransitionBundleRegistryForTest)

	ctx := signingForBundleContext(t, []group.MemberIndex{1, 2, 3, 4, 5})
	_, elected := realCoordinatorForBundleTest(t, ctx)

	// Register with a SELF that is NOT the elected coordinator.
	nonElected := group.MemberIndex(elected + 10) // arbitrary non-elected
	for _, m := range ctx.IncludedSet {
		if m != elected {
			nonElected = m
			break
		}
	}

	// Use a fresh coordinator bound to the non-elected member.
	coord := roast.NewInMemoryCoordinatorWithSigning(
		nonElected,
		roast.NoOpSigner(),
		roast.NoOpSignatureVerifier(),
	)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(nonElected),
	})

	const sessionID = "non-elected-session"
	_, cleanup, err := BeginOrchestrationForSession(sessionID, ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	cleanup()

	if _, ok := TransitionBundleForSession(sessionID); ok {
		t.Fatal("non-elected coordinator must not produce a bundle")
	}
}

func TestCleanup_AggregateBundleErrorIsSwallowed(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	ResetTransitionBundleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetTransitionBundleRegistryForTest)

	// Use the standard coordinator. AggregateBundle will fail
	// because the elected coordinator was 'self' but we never
	// recorded any snapshots in the coordinator (so the bundle
	// would be empty). Actually -- empty bundle violates
	// validation. Let me set up a scenario where Aggregate fails.
	//
	// Strategy: register a coordinator whose BeginAttempt succeeds
	// but AggregateBundle returns ErrAttemptStateInvalid because
	// we manually transition the state through State. Simpler:
	// just call cleanup() twice. The second call sees the
	// already-transitioned state and bails out cleanly without
	// recording a duplicate bundle.

	ctx := signingForBundleContext(t, []group.MemberIndex{1, 2, 3, 4, 5})
	coord, elected := realCoordinatorForBundleTest(t, ctx)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
	})

	const sessionID = "double-cleanup-session"
	handle, cleanup, err := BeginOrchestrationForSession(sessionID, ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	// Seed snapshot so the first cleanup's AggregateBundle
	// succeeds.
	snap := roast.NewLocalEvidenceSnapshot(elected, ctx.Hash(), attempt.Evidence{})
	// NoOpSigner returns empty bytes but the signature-verification
	// pre-check rejects zero-length signatures. Provide a dummy
	// non-empty signature; the NoOp verifier accepts any byte
	// sequence.
	snap.OperatorSignature = []byte{0x01}
	if err := coord.RecordEvidence(handle, snap); err != nil {
		t.Fatalf("record evidence: %v", err)
	}

	// First cleanup -- bundle recorded.
	cleanup()
	if _, ok := TransitionBundleForSession(sessionID); !ok {
		t.Fatal("first cleanup must record bundle")
	}

	// Second cleanup -- state is now Transitioned. AggregateBundle
	// returns ErrAttemptStateInvalid; the helper must swallow the
	// error rather than panic.
	cleanup() // Must not panic.
}
