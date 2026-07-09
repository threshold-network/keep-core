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
		// Empty key group: these tests register via the default (unscoped) helper, so
		// the context's key group must match "" for the wallet-scoped lookup/count to
		// resolve. Cross-key-group isolation is covered by the dedicated scoping test.
		"",
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
	handle, cleanup, err := BeginOrchestrationForSession("session-A", 1, ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup must not be nil")
	}

	// Binding must exist under (session, member).
	gotHandle, gotCtx, ok := currentAttemptHandleForCollect("session-A", 1)
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
	if _, _, ok := currentAttemptHandleForCollect("session-A", 1); ok {
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
	_, _, err := BeginOrchestrationForSession("session-X", 1, newOrchestrationTestContext(t))
	if err == nil {
		t.Fatal("expected error when registry is empty")
	}
	if !strings.Contains(err.Error(), "no coordinator registered") {
		t.Fatalf("error must mention missing registration; got %v", err)
	}
	// Empty registry => count==0 => the STATIC legacy-fallback sentinel, NOT
	// the terminal fail-closed (which is reserved for partial registration).
	if !errors.Is(err, ErrNoRoastRetryCoordinatorRegistered) {
		t.Fatalf("empty registry must return the static sentinel; got %v", err)
	}
	if errors.Is(err, ErrTerminalSigningFailure) {
		t.Fatalf("empty registry must NOT be terminal; got %v", err)
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

	_, _, err := BeginOrchestrationForSession("session-no-optin", 1, newOrchestrationTestContext(t))
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

	_, _, err := BeginOrchestrationForSession("session-Y", 1, newOrchestrationTestContext(t))
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

	_, _, err := BeginOrchestrationForSession("session-Z", 1, newOrchestrationTestContext(t))
	if err == nil {
		t.Fatal("expected error from coordinator")
	}
	if !strings.Contains(err.Error(), "synthetic begin failure") {
		t.Fatalf("error must wrap underlying cause; got %v", err)
	}
}

// assertOrchestrationFailedClosed asserts err is a HARD fail-closed: non-nil,
// neither static-fallback sentinel, classified terminal, and that no session
// binding leaked for (sessionID, member).
func assertOrchestrationFailedClosed(t *testing.T, sessionID string, member group.MemberIndex, cleanup func(), err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a fail-closed error, got nil")
	}
	if errors.Is(err, ErrNoRoastRetryCoordinatorRegistered) {
		t.Fatalf("must NOT return the legacy-fallback sentinel; got %v", err)
	}
	if errors.Is(err, ErrRoastRetryReadinessOptOut) {
		t.Fatalf("must NOT return the readiness sentinel; got %v", err)
	}
	// Must be classified TERMINAL so the signingRetryLoop aborts instead of
	// retrying the (static, never-resolving) fail-closed condition.
	if !errors.Is(err, ErrTerminalSigningFailure) {
		t.Fatalf("fail-closed must be classified terminal (ErrTerminalSigningFailure); got %v", err)
	}
	if cleanup != nil {
		t.Fatal("a failed begin must not return a cleanup")
	}
	if _, _, ok := currentAttemptHandleForCollect(sessionID, member); ok {
		t.Fatal("fail-closed must not create a session binding")
	}
}

// TestBeginOrchestrationForSession_FailsClosedPartialMultiSeat is the Codex
// re-review case: a multi-seat operator that has at least one seat registered but
// NOT this one. The member-aware lookup misses, and rather than returning the
// legacy-fallback sentinel (which would let this seat run coarse/legacy while the
// registered sibling drives bound ROAST -> fracture), Begin fails CLOSED. PR2b-2
// keeps this fail-closed: member-keying the handle binding does nothing for a seat
// with no coordinator at all.
func TestBeginOrchestrationForSession_FailsClosedPartialMultiSeat(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// Only seat 1 is registered; this Execute is for the unregistered seat 2.
	RegisterRoastRetryCoordinatorForMember(1, RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinatorWithSigning(1, roast.NoOpSigner(), roast.NoOpSignatureVerifier()),
		SelfMember:  1,
	})

	_, cleanup, err := BeginOrchestrationForSession("session-partial", 2, newOrchestrationTestContext(t))
	assertOrchestrationFailedClosed(t, "session-partial", 2, cleanup, err)
	if !strings.Contains(err.Error(), "fail closed") {
		t.Fatalf("error must explain the fail-closed; got %v", err)
	}
}

// TestBeginOrchestrationForSession_MultiSeatProceedsPerSeat asserts that PR2b-2
// retired the fully-registered multi-seat fail-closed guard: with both local
// seats registered, EACH seat begins its own attempt and binds its OWN handle
// under (sessionID, member), so sibling seats stay isolated. The load-bearing
// isolation proof is that one seat's cleanup does NOT tear down the sibling's
// binding -- under the old sessionID-only key, seat 1's cleanup deleted the
// shared binding and seat 2 lost its hash enforcement. (The two handles are equal
// here because both coordinators mint id=1 for the same ctx; equality is expected,
// so survival-after-sibling-cleanup -- not handle distinctness -- is the proof.
// TestSessionHandleBinding_IsolatesByMember exercises distinct handles directly.)
func TestBeginOrchestrationForSession_MultiSeatProceedsPerSeat(t *testing.T) {
	t.Setenv(RoastRetryReadinessOptInEnvVar, "true")
	ResetRoastRetryRegistrationForTest()
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetRoastRetryRegistrationForTest)
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// Both local seats registered -> multi-seat.
	RegisterRoastRetryCoordinatorForMember(1, RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinatorWithSigning(1, roast.NoOpSigner(), roast.NoOpSignatureVerifier()),
		SelfMember:  1,
	})
	RegisterRoastRetryCoordinatorForMember(2, RoastRetryDeps{
		Coordinator: roast.NewInMemoryCoordinatorWithSigning(2, roast.NoOpSigner(), roast.NoOpSignatureVerifier()),
		SelfMember:  2,
	})

	ctx := newOrchestrationTestContext(t)

	handle1, cleanup1, err := BeginOrchestrationForSession("session-multiseat", 1, ctx)
	if err != nil {
		t.Fatalf("seat 1 begin must succeed (no longer fail-closed): %v", err)
	}
	if cleanup1 == nil {
		t.Fatal("seat 1 cleanup must not be nil")
	}
	handle2, cleanup2, err := BeginOrchestrationForSession("session-multiseat", 2, ctx)
	if err != nil {
		t.Fatalf("seat 2 begin must succeed (no longer fail-closed): %v", err)
	}
	if cleanup2 == nil {
		t.Fatal("seat 2 cleanup must not be nil")
	}

	// Each seat reads back its own binding.
	got1, _, ok := currentAttemptHandleForCollect("session-multiseat", 1)
	if !ok {
		t.Fatal("seat 1 binding must exist")
	}
	if got1 != handle1 {
		t.Fatal("seat 1 must read back its own handle")
	}
	got2, _, ok := currentAttemptHandleForCollect("session-multiseat", 2)
	if !ok {
		t.Fatal("seat 2 binding must exist")
	}
	if got2 != handle2 {
		t.Fatal("seat 2 must read back its own handle")
	}

	// ISOLATION: seat 1's cleanup must clear ONLY seat 1's binding; seat 2's
	// binding must survive (this is exactly the multi-seat bug being fixed).
	cleanup1()
	if _, _, ok := currentAttemptHandleForCollect("session-multiseat", 1); ok {
		t.Fatal("seat 1 binding must be cleared after its own cleanup")
	}
	if _, _, ok := currentAttemptHandleForCollect("session-multiseat", 2); !ok {
		t.Fatal("seat 2 binding must SURVIVE seat 1's cleanup (member isolation)")
	}
	cleanup2()
	if _, _, ok := currentAttemptHandleForCollect("session-multiseat", 2); ok {
		t.Fatal("seat 2 binding must be cleared after its own cleanup")
	}
}

// TestSessionHandleBinding_IsolatesByMember exercises the (sessionID, member)
// re-keying directly with DISTINCT handles: two contexts (differing only in
// attempt number) minted by one coordinator yield distinct handles, bound under
// the same session for two different members. Each member must read back its own
// handle, and clearing one member must leave the other intact.
func TestSessionHandleBinding_IsolatesByMember(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	coord := roast.NewInMemoryCoordinator()
	ctxA := newOrchestrationTestContext(t)
	ctxB, err := attempt.NewAttemptContext(
		"orchestration-session",
		"key-group-orchestration",
		[]byte{0x01, 0x02},
		[attempt.MessageDigestLength]byte{0x77},
		1, // distinct attempt number -> distinct context hash -> distinct handle
		[]group.MemberIndex{1, 2, 3, 4, 5},
		nil,
	)
	if err != nil {
		t.Fatalf("ctxB: %v", err)
	}

	handleA, err := coord.BeginAttempt(ctxA)
	if err != nil {
		t.Fatalf("beginA: %v", err)
	}
	handleB, err := coord.BeginAttempt(ctxB)
	if err != nil {
		t.Fatalf("beginB: %v", err)
	}
	if handleA == handleB {
		t.Fatal("setup: the two handles must be distinct for a meaningful isolation test")
	}

	SetCurrentAttemptHandleForSession("shared-session", 1, handleA, ctxA)
	SetCurrentAttemptHandleForSession("shared-session", 2, handleB, ctxB)

	got1, gotCtx1, ok := currentAttemptHandleForCollect("shared-session", 1)
	if !ok || got1 != handleA || gotCtx1.Hash() != ctxA.Hash() {
		t.Fatalf("member 1 must read back its own (handle, ctx); ok=%v", ok)
	}
	got2, gotCtx2, ok := currentAttemptHandleForCollect("shared-session", 2)
	if !ok || got2 != handleB || gotCtx2.Hash() != ctxB.Hash() {
		t.Fatalf("member 2 must read back its own (handle, ctx); ok=%v", ok)
	}

	// Clearing member 1 must not disturb member 2.
	ClearCurrentAttemptHandleForSession("shared-session", 1)
	if _, _, ok := currentAttemptHandleForCollect("shared-session", 1); ok {
		t.Fatal("member 1 binding must be gone after clear")
	}
	if got, _, ok := currentAttemptHandleForCollect("shared-session", 2); !ok || got != handleB {
		t.Fatal("member 2 binding must survive member 1's clear")
	}
}

func TestEndOrchestrationForSession_RemovesBinding(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContext(t)
	SetCurrentAttemptHandleForSession("session-end", 3, roast.AttemptHandle{}, ctx)

	if _, _, ok := currentAttemptHandleForCollect("session-end", 3); !ok {
		t.Fatal("setup: binding must exist")
	}
	EndOrchestrationForSession("session-end", 3)
	if _, _, ok := currentAttemptHandleForCollect("session-end", 3); ok {
		t.Fatal("binding must be removed after End")
	}
}

func TestEvictStaleSessionHandleBindings_RemovesOldEntries(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	// Two bindings with different ages.
	ctx := newOrchestrationTestContext(t)
	SetCurrentAttemptHandleForSession("session-old", 1, roast.AttemptHandle{}, ctx)
	// Backdate by forcing the timestamp.
	sessionAttemptBindingMu.Lock()
	oldKey := sessionMemberKey{"session-old", 1}
	b := sessionAttemptBindings[oldKey]
	b.createdAt = time.Now().Add(-10 * time.Minute)
	sessionAttemptBindings[oldKey] = b
	sessionAttemptBindingMu.Unlock()

	SetCurrentAttemptHandleForSession("session-new", 1, roast.AttemptHandle{}, ctx)

	// Sweep with 5-minute TTL: old must be evicted, new must survive.
	evicted := evictStaleSessionHandleBindings(5 * time.Minute)
	if evicted != 1 {
		t.Fatalf("expected 1 eviction, got %d", evicted)
	}
	if _, _, ok := currentAttemptHandleForCollect("session-old", 1); ok {
		t.Fatal("session-old must be evicted")
	}
	if _, _, ok := currentAttemptHandleForCollect("session-new", 1); !ok {
		t.Fatal("session-new must survive")
	}
}

func TestEvictStaleSessionHandleBindings_LeavesFreshEntries(t *testing.T) {
	ResetSessionHandleRegistryForTest()
	t.Cleanup(ResetSessionHandleRegistryForTest)

	ctx := newOrchestrationTestContext(t)
	SetCurrentAttemptHandleForSession("session-fresh", 1, roast.AttemptHandle{}, ctx)

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
