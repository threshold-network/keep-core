//go:build frost_roast_retry

package signing

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/keep-network/keep-core/pkg/frost/roast"
	"github.com/keep-network/keep-core/pkg/frost/roast/attempt"
	"github.com/keep-network/keep-core/pkg/protocol/group"
)

func newOrchestrationTestContext(t *testing.T) attempt.AttemptContext {
	t.Helper()
	ctx, err := attempt.NewAttemptContext(
		"orchestration-session",
		"key-group-orchestration",
		[]byte{0x01, 0x02},
		[attempt.MessageDigestLength]byte{0x77},
		0,
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)
	if err != nil {
		t.Fatalf("ctx: %v", err)
	}
	return ctx
}

func TestBeginOrchestrationForSession_HappyPath(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	ctx := newOrchestrationTestContext(t)
	handle, cleanup, err := BeginOrchestrationForSession("session-A", ctx, []byte{0x01, 0x02})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup must not be nil")
	}

	// Binding must exist.
	gotHandle, gotCtx, ok := currentAttemptHandleForCollect("session-A")
	if !ok {
		t.Fatal("binding must exist after Begin")
	}
	if gotHandle != handle {
		t.Fatal("binding handle mismatch")
	}
	if gotCtx.Hash() != ctx.Hash() {
		t.Fatal("binding context mismatch")
	}

	cleanup()
	if _, _, ok := currentAttemptHandleForCollect("session-A"); ok {
		t.Fatal("binding must be cleared after cleanup")
	}
}

func TestBeginOrchestrationForSession_ErrorsWhenRegistryEmpty(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// Readiness env var is set; the registry is empty -- we expect
	// the registry-empty error, not the env-var error.
	_, _, err := BeginOrchestrationForSession("session-X", newOrchestrationTestContext(t), []byte{0x01, 0x02})
	if err == nil {
		t.Fatal("expected error when registry is empty")
	}
	if !strings.Contains(err.Error(), "no coordinator registered") {
		t.Fatalf("error must mention missing registration; got %v", err)
	}
}

func TestBeginOrchestrationForSession_ErrorsWhenReadinessOptInUnset(t *testing.T) {
	// Explicitly unset, in case the test runner inherits the env var
	// from outside.
	t.Setenv(RoastRetryReadinessOptInEnvVar, "")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// Even with a registered coordinator, the readiness env var
	// short-circuits orchestration. This is the load-bearing safety
	// property: production builds with the frost_roast_retry tag
	// still cannot enter the orchestration path without an explicit
	// operator decision.
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	_, _, err := BeginOrchestrationForSession("session-no-optin", newOrchestrationTestContext(t), []byte{0x01, 0x02})
	if !errors.Is(err, ErrRoastRetryReadinessOptOut) {
		t.Fatalf("expected ErrRoastRetryReadinessOptOut, got %v", err)
	}
}

func TestBeginOrchestrationForSession_ErrorsWhenCoordinatorNil(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: nil,
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	_, _, err := BeginOrchestrationForSession("session-Y", newOrchestrationTestContext(t), []byte{0x01, 0x02})
	if err == nil {
		t.Fatal("expected error when Coordinator is nil")
	}
	if !strings.Contains(err.Error(), "nil Coordinator") {
		t.Fatalf("error must mention nil coordinator; got %v", err)
	}
}

func TestBeginOrchestrationForSession_PropagatesBeginAttemptError(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// A coordinator whose BeginAttempt always fails.
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: &erroringCoordinator{err: errors.New("synthetic begin failure")},
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	_, _, err := BeginOrchestrationForSession("session-Z", newOrchestrationTestContext(t), []byte{0x01, 0x02})
	if err == nil {
		t.Fatal("expected error from coordinator")
	}
	if !strings.Contains(err.Error(), "synthetic begin failure") {
		t.Fatalf("error must wrap underlying cause; got %v", err)
	}
}

func TestEndOrchestrationForSession_RemovesBinding(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContext(t)
	SetCurrentAttemptHandleForSession("session-end", roast.AttemptHandle{}, ctx)

	if _, _, ok := currentAttemptHandleForCollect("session-end"); !ok {
		t.Fatal("setup: binding must exist")
	}
	EndOrchestrationForSession("session-end")
	if _, _, ok := currentAttemptHandleForCollect("session-end"); ok {
		t.Fatal("binding must be removed after End")
	}
}

func TestEvictStaleSessionHandleBindings_RemovesOldEntries(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// Two bindings with different ages.
	ctx := newOrchestrationTestContext(t)
	SetCurrentAttemptHandleForSession("session-old", roast.AttemptHandle{}, ctx)
	// Backdate by forcing the timestamp.
	sessionAttemptBindingMu.Lock()
	b := sessionAttemptBindings["session-old"]
	b.createdAt = time.Now().Add(-10 * time.Minute)
	sessionAttemptBindings["session-old"] = b
	sessionAttemptBindingMu.Unlock()

	SetCurrentAttemptHandleForSession("session-new", roast.AttemptHandle{}, ctx)

	// Sweep with 5-minute TTL: old must be evicted, new must survive.
	evicted := evictStaleSessionHandleBindings(5 * time.Minute)
	if evicted != 1 {
		t.Fatalf("expected 1 eviction, got %d", evicted)
	}
	if _, _, ok := currentAttemptHandleForCollect("session-old"); ok {
		t.Fatal("session-old must be evicted")
	}
	if _, _, ok := currentAttemptHandleForCollect("session-new"); !ok {
		t.Fatal("session-new must survive")
	}
}

func TestEvictStaleSessionHandleBindings_LeavesFreshEntries(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContext(t)
	SetCurrentAttemptHandleForSession("session-fresh", roast.AttemptHandle{}, ctx)

	// Sweep with the default 2-hour TTL: nothing should be evicted.
	evicted := evictStaleSessionHandleBindings(SessionHandleBindingTTL)
	if evicted != 0 {
		t.Fatalf("expected 0 evictions for fresh binding, got %d", evicted)
	}
}

func TestSessionHandleBindingTTL_MatchesRFC(t *testing.T) {
	if SessionHandleBindingTTL != 2*time.Hour {
		t.Fatalf(
			"RFC-21 specifies a 2-hour default TTL; constant is %s",
			SessionHandleBindingTTL,
		)
	}
}

func TestStartSessionHandleSweeper_IsIdempotent(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	StartSessionHandleSweeper()
	StartSessionHandleSweeper()
	StartSessionHandleSweeper()
	// sync.Once means only one goroutine started; we don't have a
	// direct observable, but ResetSessionHandleRegistryForTest will
	// close the stop channel and the goroutine will exit cleanly.
	// If sync.Once were broken, double-close on the stop channel
	// would panic during cleanup.
}

func TestRegisterRoastRetryCoordinator_StartsSweeper(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  1,
	})

	// Register again to verify sync.Once prevents a second
	// sweeper.
	RegisterRoastRetryCoordinator(RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinator(),
		Signer:      roast.NoOpSigner(),
		Verifier:    roast.NoOpSignatureVerifier(),
		SelfMember:  2,
	})

	// Reset should not panic (would panic on double-close if
	// sync.Once failed).
	ResetSessionHandleRegistryForTest()
}

// erroringCoordinator returns a synthetic error from BeginAttempt.
// Other methods return zero values or nil; tests that need them
// should use a real coordinator.
type erroringCoordinator struct {
	err error
}

func (e *erroringCoordinator) BeginAttempt(_ attempt.AttemptContext) (roast.AttemptHandle, error) {
	return roast.AttemptHandle{}, e.err
}
func (e *erroringCoordinator) State(_ roast.AttemptHandle) (roast.AttemptState, error) {
	return roast.AttemptStatePending, nil
}
func (e *erroringCoordinator) SelectedCoordinator(_ roast.AttemptHandle) (group.MemberIndex, error) {
	return 0, nil
}
func (e *erroringCoordinator) RecordEvidence(_ roast.AttemptHandle, _ *roast.LocalEvidenceSnapshot) error {
	return nil
}
func (e *erroringCoordinator) AggregateBundle(_ roast.AttemptHandle) (*roast.TransitionMessage, error) {
	return nil, nil
}
func (e *erroringCoordinator) MarkSucceeded(_ roast.AttemptHandle) error {
	return nil
}
func (e *erroringCoordinator) VerifyBundle(_ roast.AttemptHandle, _ *roast.TransitionMessage) error {
	return nil
}
func (e *erroringCoordinator) NextAttempt(
	_ roast.AttemptHandle, _ *roast.TransitionMessage, _ uint, _ []byte,
) (attempt.AttemptContext, error) {
	return attempt.AttemptContext{}, nil
}
