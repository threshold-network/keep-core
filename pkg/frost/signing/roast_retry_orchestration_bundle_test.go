//go:build frost_roast_retry

package signing

import (
	"testing"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

// bundleTestDkgKey is the DKG group public key signingForBundleContext builds
// the attempt context from; the orchestration cleanup stores it in the
// transition record.
var bundleTestDkgKey = []byte{0x01, 0x02, 0x03}

// signingForBundleContext constructs an attempt context whose
// SelectCoordinator will deterministically pick a member (for the
// sake of this test). Real production deployments use the
// rotating selection; here we pin a stable handle for assertion.
func signingForBundleContext(t *testing.T, members []group.MemberIndex) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		"orchestration-bundle-test",
		"key-group",
		bundleTestDkgKey,
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

// realCoordinatorForBundleTest returns an in-memory coordinator with NoOp
// signer/verifier so the AggregateBundle path runs end-to-end without crypto
// setup, plus the elected coordinator computed from the test context. The
// caller passes `elected` as the member to BeginOrchestrationForSession so
// maybeProduceTransitionRecord's elected==member check passes and it invokes
// AggregateBundle.
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

func TestCleanup_ProducesRecordWhenElectedCoordinator(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	ctx := signingForBundleContext(t, []group.MemberIndex{1, 2, 3, 4, 5})
	coord, elected := realCoordinatorForBundleTest(t, ctx)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
	})

	const sessionID = "bundle-producer-session"
	handle, cleanup, err := BeginOrchestrationForSession(sessionID, ctx, elected, bundleTestDkgKey)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	// Seed at least one snapshot so AggregateBundle's non-empty-bundle
	// validation passes. NoOpSigner returns empty bytes but the
	// signature-verification pre-check rejects zero-length signatures, so
	// provide a dummy non-empty signature (the NoOp verifier accepts any bytes).
	snap := roast.NewLocalEvidenceSnapshot(elected, ctx.Hash(), attempt.Evidence{})
	snap.OperatorSignature = []byte{0x01}
	if err := coord.RecordEvidence(handle, snap); err != nil {
		t.Fatalf("record evidence: %v", err)
	}

	// Cleanup must produce + record a transition record (we passed the elected
	// member and the attempt is still Collecting).
	cleanup()

	record, ok := RoastTransitionForSession(ctx.SessionID, elected)
	if !ok {
		t.Fatal("elected coordinator's cleanup must produce a transition record")
	}
	if record.Bundle == nil {
		t.Fatal("recorded transition must carry a non-nil bundle")
	}
	if record.Bundle.CoordinatorID() != elected {
		t.Fatalf("bundle coordinator id %d != elected %d", record.Bundle.CoordinatorID(), elected)
	}
	// The record must carry the binding the selector needs.
	if record.PreviousHandle != handle {
		t.Fatal("record must carry the failed attempt's handle (survives cleanup's handle clear)")
	}
	if string(record.DkgGroupPublicKey) != string(bundleTestDkgKey) {
		t.Fatalf("record dkg key %x != %x", record.DkgGroupPublicKey, bundleTestDkgKey)
	}
}

func TestCleanup_DoesNotProduceRecordWhenNotElectedCoordinator(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	ctx := signingForBundleContext(t, []group.MemberIndex{1, 2, 3, 4, 5})
	_, elected := realCoordinatorForBundleTest(t, ctx)

	// A member that is NOT the elected coordinator.
	nonElected := group.MemberIndex(elected + 10)
	for _, m := range ctx.IncludedSet {
		if m != elected {
			nonElected = m
			break
		}
	}

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
	_, cleanup, err := BeginOrchestrationForSession(sessionID, ctx, nonElected, bundleTestDkgKey)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	cleanup()

	if _, ok := RoastTransitionForSession(ctx.SessionID, nonElected); ok {
		t.Fatal("non-elected coordinator must not produce a transition record")
	}
}

func TestCleanup_AggregateBundleErrorIsSwallowed(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	ResetRoastTransitionRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)
	t.Cleanup(ResetRoastTransitionRegistryForTest)

	ctx := signingForBundleContext(t, []group.MemberIndex{1, 2, 3, 4, 5})
	coord, elected := realCoordinatorForBundleTest(t, ctx)
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: coord,
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  uint32(elected),
	})

	const sessionID = "double-cleanup-session"
	handle, cleanup, err := BeginOrchestrationForSession(sessionID, ctx, elected, bundleTestDkgKey)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	snap := roast.NewLocalEvidenceSnapshot(elected, ctx.Hash(), attempt.Evidence{})
	snap.OperatorSignature = []byte{0x01}
	if err := coord.RecordEvidence(handle, snap); err != nil {
		t.Fatalf("record evidence: %v", err)
	}

	// First cleanup -- record produced.
	cleanup()
	if _, ok := RoastTransitionForSession(ctx.SessionID, elected); !ok {
		t.Fatal("first cleanup must record a transition")
	}

	// Second cleanup -- state is now Transitioned. AggregateBundle returns
	// ErrAttemptStateInvalid; the helper must swallow it rather than panic.
	cleanup() // Must not panic.
}
